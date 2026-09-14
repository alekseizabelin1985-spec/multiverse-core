//go:build e2e

package e2e_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/access"
	"multiverse-core.io/cmd/telegram-bot/internal/commands"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/shared/eventbus"
)

// onboard takes the player from /start to a character in the fixture world;
// the first update id is from.
func onboard(t *testing.T, b *bot, from int64) {
	t.Helper()
	for i, text := range []string{"/start", render.ConsentButton, "Лена", "Мир Тёмного леса"} {
		b.step(t, message(from+int64(i), player, text), 1, 0)
	}
}

// SEC-07: a message of a group chat, even from a player of the allow-list, is
// not answered and does not reach the gateway (US-008, FR-131, NFR-044).
// SEC-06: a stranger gets one refusal for a series of messages, and the gateway
// is not called for any of them.
func TestBotIsSilentInAGroupAndRefusesAStrangerOnce(t *testing.T) {
	w := newWorld(t, worldOptions{prefix: "bot-access"})
	b := startBot(t, w, botOptions{})

	b.source.Push(groupMessage(1, player, "/start"), groupMessage(2, player, "/attack"))
	b.source.Push(message(3, stranger, "/start"), message(4, stranger, "/start"), message(5, stranger, "/look"))
	// A player of the allow-list after them: once they are answered, every
	// update before them was handled (updates.Fake hands them over in order).
	b.source.Push(message(6, otherPlayer, "/start"))
	waitFor(t, "the answer to the player of the allow-list", func() bool {
		for _, l := range b.tr.all(streamReply) {
			if l.Chat == otherPlayer {
				return true
			}
		}
		return false
	})

	got := lines(b.tr.all(streamReply))
	want := []string{
		line{Stream: streamReply, Chat: stranger, Text: access.TextInviteOnly, Keyboard: "-"}.String(),
		line{Stream: streamReply, Chat: otherPlayer, Text: render.NoticeText, Keyboard: "[Мне есть 18, принимаю|Отказаться]"}.String(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("answers:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, l := range b.tr.all(streamReply) {
		if l.Chat == groupChat || l.Chat == player {
			t.Errorf("the bot answered in the group: %s", l)
		}
	}
	// The only call to the gateway is the resolve of the player after them.
	if calls := b.flowGW.all(); len(calls) != 1 || calls[0].Op != "resolve" {
		t.Errorf("calls to the gateway = %+v, want the one resolve of the player of the allow-list", calls)
	}
	if c := b.gate.Counters(); c.Denied != 3 || c.NotPrivate != 2 {
		t.Errorf("counters = %+v, want 3 refused to the stranger and 2 from the group", c)
	}
}

// markup is a narrative a model could write: Markdown and HTML links and
// emphasis, which a parse mode would turn into links.
const markup = `Волк скалится. [нажми](http://evil.example/a) <a href="http://evil.example/b">сюда</a> *жирно* <b>громко</b>`

// SEC-10: a narrative with [x](http://…) and <a> reaches the player character
// for character, as plain text.
func TestBotDeliversTheMarkupOfANarrativeAsText(t *testing.T) {
	w := newWorld(t, worldOptions{prefix: "bot-markup", narrative: markup})
	b := startBot(t, w, botOptions{})
	onboard(t, b, 1)
	b.step(t, message(10, player, "/enter dark-forest-01"), 1, 4)

	want := markup + "\n" + render.MarkTemplate + ": unavailable"
	narratives := 0
	for _, l := range b.tr.all(streamDelivery) {
		if strings.Contains(l.Text, "evil.example") {
			narratives++
			if l.Text != want {
				t.Errorf("narrative delivered as %q, want %q", l.Text, want)
			}
		}
	}
	if narratives != 2 {
		t.Errorf("narratives with markup delivered: %d, want the entry and the start of the encounter", narratives)
	}
}

// 429 rate_limited and 503 bus_unavailable of the gateway: the player reads
// what happened, and the command is not lost — its update, handed again, goes
// out with the same action_key and becomes one action.
func TestBotKeepsTheCommandOnRateLimitAndAnUnavailableBus(t *testing.T) {
	w := newWorld(t, worldOptions{prefix: "bot-faults", vars: map[string]string{
		"MV_GATEWAY_RATE_ACTIONS_PER_MIN": "1",
		"MV_GATEWAY_RATE_ACTIONS_BURST":   "1",
	}})
	f, proxyURL := newFaults(t, w.g.URL)
	b := startBot(t, w, botOptions{gatewayURL: proxyURL})
	onboard(t, b, 1)
	said := func() int { return w.countOf(t, eventbus.TopicPlayerEvents, "player.said") }
	accepted := line{Stream: streamReply, Chat: player, Text: render.Accepted, Keyboard: "-"}

	// 429: the second action within a minute is refused with its wait.
	b.step(t, message(20, player, "/say раз"), 1, 0)
	b.step(t, message(21, player, "/say два"), 1, 0)
	if got := last(b); got != (line{Stream: streamReply, Chat: player, Text: "Слишком часто, подождите 60 с.", Keyboard: "-"}) {
		t.Errorf("answer to an action over the limit = %s", got)
	}
	if n := said(); n != 1 {
		t.Fatalf("player.said after the refusal: %d, want 1", n)
	}
	w.g.Clock().Advance(time.Minute)
	b.step(t, message(21, player, "/say два"), 1, 0)
	if got := last(b); got != accepted {
		t.Errorf("answer to the update handed again after the wait = %s", got)
	}
	if n := said(); n != 2 {
		t.Errorf("player.said after the update handed again: %d, want 2", n)
	}
	key21 := commands.ActionKey(actionKeySalt, 21)
	if keys := actionKeys(b, "action say"); !reflect.DeepEqual(keys[1:], []string{key21, key21}) {
		t.Errorf("action keys of /say = %v, want the key of update 21 twice after the first", keys)
	}

	// 503 once: the client of the flow repeats with the same action_key, and
	// the player reads that the action was accepted.
	w.g.Clock().Advance(time.Minute)
	f.refuse(1)
	b.step(t, message(30, player, "/say три"), 1, 0)
	if got := last(b); got != accepted {
		t.Errorf("answer after one 503 = %s", got)
	}
	key30 := commands.ActionKey(actionKeySalt, 30)
	if keys := f.actionKeys(); !reflect.DeepEqual(keys[len(keys)-2:], []string{key30, key30}) {
		t.Errorf("action requests after one 503 = %v, want the key of update 30 twice", keys)
	}

	// 503 on the repeat too: the player is told the service is unavailable;
	// the update handed again goes out with the same key and becomes one
	// action.
	w.g.Clock().Advance(time.Minute)
	f.refuse(2)
	b.step(t, message(31, player, "/say четыре"), 1, 0)
	if got := last(b); got != (line{Stream: streamReply, Chat: player, Text: render.Unavailable, Keyboard: "-"}) {
		t.Errorf("answer after two 503 = %s", got)
	}
	before := said()
	b.step(t, message(31, player, "/say четыре"), 1, 0)
	if got := last(b); got != accepted {
		t.Errorf("answer to the update handed again after two 503 = %s", got)
	}
	key31 := commands.ActionKey(actionKeySalt, 31)
	if keys := f.actionKeys(); !reflect.DeepEqual(keys[len(keys)-3:], []string{key31, key31, key31}) {
		t.Errorf("action requests of update 31 = %v, want its key three times", keys)
	}
	if n := said() - before; n != 1 {
		t.Errorf("player.said of update 31: %d, want 1", n)
	}
	if n := said(); n != 4 {
		t.Errorf("player.said in all: %d, want 4 — one per update", n)
	}
}

// last is the last answer of the update handler.
func last(b *bot) line {
	all := b.tr.all(streamReply)
	if len(all) == 0 {
		return line{}
	}
	return all[len(all)-1]
}

// actionKeys are the action_key of the calls of an operation, in order.
func actionKeys(b *bot, op string) []string {
	var out []string
	for _, c := range b.flowGW.all() {
		if c.Op == op {
			out = append(out, c.ActionKey)
		}
	}
	return out
}
