//go:build e2e

package e2e_test

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/commands"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
)

// scenarioRuns is how many times the scenario runs from nothing: the record of
// every run must be the one of the first (DoD of T-315).
const scenarioRuns = 3

// quietWindow is how long, on the wall clock, the scenario waits after its
// last step for anything that should not come.
const quietWindow = 200 * time.Millisecond

// scriptStep is one message of the player and how the bot answers it: the
// answers of the update handler, in order, and the deliveries it brings, in
// any order within the step.
type scriptStep struct {
	text       string
	replies    []string
	deliveries []string
}

// The golden record of the scenario. A line is "<stream> <chat> <text> <keyboard>",
// the text quoted, the keyboard as rows in brackets ("-" none, "[remove]" hides
// the one shown). The texts of the rules and of the templates are the ones of
// the gateway and of testkit/swarm; the dice are those of the table of
// FixedMechanics for the identifiers of this run.
var script = []scriptStep{
	{text: "/start", replies: []string{reply(render.NoticeText, "[Мне есть 18, принимаю|Отказаться]")}},
	{text: render.ConsentButton, replies: []string{reply("Введите имя персонажа: от 2 до 32 символов — буквы, цифры, пробел, дефис. Имя будет видно другим игрокам.", "[remove]")}},
	{text: "В!", replies: []string{reply("Такое имя не подходит: нужно от 2 до 32 символов — буквы, цифры, пробел, дефис. Введите другое.", "-")}},
	// The name repeats the username of the profile, ignoring case (US-009).
	{text: "vasya", replies: []string{reply("Это имя совпадает с вашим ником в Telegram и будет видно другим игрокам. Оставить?", "[Да|Ввести другое]")}},
	{text: render.NameKeep, replies: []string{reply("Выберите мир.", "[Мир Тёмного леса][Мир Туманных холмов]")}},
	{text: "Мир Тёмного леса", replies: []string{reply("Персонаж «vasya» создан.\n«vasya»: жив, HP 10/10\nГде: Мир Тёмного леса\nКоманда: /enter dark-forest-01", "[/enter dark-forest-01]")}},
	{text: "/enter dark-forest-01", replies: []string{reply("Принято.", "-")}, deliveries: []string{
		delivered("Вы вошли в Тёмный лес.", "-"),
		delivered("Ельник смыкается за спиной. vasya оглядывается: между стволами ничего не движется. Здоровье — 10/10.\n— упрощённый режим (шаблон): unavailable", "-"),
		delivered("Из тени выходит Альфа-волк. Действия: /attack wolf-alpha, /flee", "[/attack][/flee|/defend]"),
		delivered("Из подлеска выходит Альфа-волк. Отступать поздно — схватка начинается: vasya.\n— упрощённый режим (шаблон): unavailable", "-"),
	}},
	// Without a target the flow takes the only opponent standing.
	{text: "/attack", replies: []string{reply("Принято.", "-")}, deliveries: []string{
		delivered("Попадание! Урон 1. Альфа-волк: 9/10.", "-"),
		delivered("Альфа-волк атакует — попадание! Урон 3. vasya: 7/10.", "-"),
		delivered("vasya достаёт Альфа-волк: удар на 1. В ответ — 3. Сил 7 из 10.\n— упрощённый режим (шаблон): unavailable", "-"),
	}},
	{text: "/say привет", replies: []string{reply("Принято.", "-")}},
	{text: "/forget", replies: []string{reply("Удалить вашу связку с игрой? Персонаж останется в мире без владельца, вернуть его будет нельзя. Что удаляется и что остаётся, написано в сообщении /help. Чтобы подтвердить, отправьте /forget confirm", "-")}},
	{text: "/forget confirm", replies: []string{reply("Связка удалена. /start — начать заново", "[/start]")}},
	// A barrier (N-1 of review #1): one answer and no delivery. A late answer
	// or delivery of the steps before it would land here and fail the record.
	// After /forget the link is gone, so /help shows the notice with the
	// consent keyboard.
	{text: "/help", replies: []string{reply(render.NoticeText, "[Мне есть 18, принимаю|Отказаться]")}},
}

func reply(text, kb string) string {
	return line{Stream: streamReply, Chat: player, Text: text, Keyboard: kb}.String()
}

func delivered(text, kb string) string {
	return line{Stream: streamDelivery, Chat: player, Text: text, Keyboard: kb}.String()
}

// sayStep is the index of /say in the script; its update is handed to the bot
// a second time, as Telegram does when the offset of the bot did not move.
var sayStep = slices.IndexFunc(script, func(s scriptStep) bool { return strings.HasPrefix(s.text, "/say") })

// record is what one run of the scenario left.
type record struct {
	replies    []string
	deliveries [][]string
}

// The scenario of T-315 from /start to /forget confirm, three times from
// nothing: the texts and the keyboards of the golden record, the order of the
// answers, every delivery acknowledged and none sent twice, the order of the
// deliveries the one the gateway gave, and the same action_key for the same
// update handed twice.
func TestBotScenarioFromStartToForget(t *testing.T) {
	var first record
	for run := range scenarioRuns {
		got := runScenario(t)
		if run == 0 {
			first = got
			continue
		}
		if !reflect.DeepEqual(got, first) {
			t.Errorf("run %d left another record than run 1:\n%s\nrun 1:\n%s", run+1, got, first)
		}
	}
}

func (r record) String() string {
	var b strings.Builder
	for _, l := range r.replies {
		b.WriteString(l + "\n")
	}
	for i, step := range r.deliveries {
		for _, l := range step {
			b.WriteString("step " + strconv.Itoa(i+1) + ": " + l + "\n")
		}
	}
	return b.String()
}

func runScenario(t *testing.T) record {
	t.Helper()
	w := newWorld(t, worldOptions{prefix: "bot"})
	b := startBot(t, w, botOptions{})
	defer b.stop()

	var wantReplies []string
	var wantDeliveries [][]string
	for i, s := range script {
		b.step(t, message(int64(1000+i), player, s.text), len(s.replies), len(s.deliveries))
		wantReplies = append(wantReplies, s.replies...)
		want := slices.Clone(s.deliveries)
		slices.Sort(want)
		wantDeliveries = append(wantDeliveries, want)
		if i == sayStep {
			// The same update again: the same action_key, one player.said.
			b.step(t, message(int64(1000+i), player, s.text), len(s.replies), 0)
			wantReplies = append(wantReplies, s.replies...)
			wantDeliveries = append(wantDeliveries, nil)
		}
	}
	// And after the barrier, a short quiet window: nothing more arrives.
	settledReplies, settledDeliveries := len(b.tr.all(streamReply)), len(b.tr.all(streamDelivery))
	<-clock.RealTimers{}.After(quietWindow).C()
	if r, d := len(b.tr.all(streamReply)), len(b.tr.all(streamDelivery)); r != settledReplies || d != settledDeliveries || !b.loopGW.settled() {
		t.Errorf("after the last step: %d answers and %d deliveries arrived, and every delivery acknowledged %t", r-settledReplies, d-settledDeliveries, b.loopGW.settled())
	}
	got := record{replies: lines(b.tr.all(streamReply)), deliveries: b.deliveriesByStep()}
	for i := range got.deliveries {
		if len(got.deliveries[i]) == 0 {
			got.deliveries[i] = nil
		}
	}

	if !reflect.DeepEqual(got.replies, wantReplies) {
		t.Errorf("answers:\n%s\nwant:\n%s", strings.Join(got.replies, "\n"), strings.Join(wantReplies, "\n"))
	}
	if !reflect.DeepEqual(got.deliveries, wantDeliveries) {
		t.Errorf("deliveries by step:\n%q\nwant:\n%q", got.deliveries, wantDeliveries)
	}

	// The bot reorders nothing: it sends the deliveries in the order the
	// gateway gave them, one message each here.
	if sent, given := lines(b.tr.all(streamDelivery)), lines(b.loopGW.givenLines()); !reflect.DeepEqual(sent, given) {
		t.Errorf("sent deliveries:\n%s\nin the order given:\n%s", strings.Join(sent, "\n"), strings.Join(given, "\n"))
	}
	given, acked := b.loopGW.ids()
	for _, id := range given {
		if n := count(acked, id); n != 1 {
			t.Errorf("delivery %s acknowledged %d times, want once", id, n)
		}
	}
	if len(slices.Compact(slices.Sorted(slices.Values(given)))) != len(given) {
		t.Errorf("a delivery was given twice: %v", given)
	}

	checkActionKeys(t, w, b)
	return got
}

// checkActionKeys: every action carries the action_key of its update, and the
// update handed twice sent its key twice and made one event.
func checkActionKeys(t *testing.T, w *world, b *bot) {
	t.Helper()
	var says []call
	for _, c := range b.flowGW.all() {
		if c.Err != nil {
			t.Errorf("the flow met an error of the gateway: %+v", c)
		}
		if c.Op == "action say" {
			says = append(says, c)
		}
	}
	key := commands.ActionKey(actionKeySalt, int64(1000+sayStep))
	if len(says) != 2 || says[0].ActionKey != key || says[1].ActionKey != key {
		t.Errorf("actions of /say = %+v, want two with the action_key %s of update %d", says, key, 1000+sayStep)
	}
	if n := w.countOf(t, eventbus.TopicPlayerEvents, "player.said"); n != 1 {
		t.Errorf("player.said on the bus: %d, want 1 for the update handed twice", n)
	}
}

func lines(ls []line) []string {
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.String())
	}
	return out
}

func count(ids []string, id string) int {
	n := 0
	for _, x := range ids {
		if x == id {
			n++
		}
	}
	return n
}
