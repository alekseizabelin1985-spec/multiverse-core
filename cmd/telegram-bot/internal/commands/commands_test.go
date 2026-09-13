package commands_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"multiverse-core.io/cmd/telegram-bot/internal/commands"
)

func TestParseTheDictionaryOfFR002(t *testing.T) {
	cases := []struct {
		in   string
		want commands.Command
	}{
		{"/start", commands.Command{Type: commands.Start, Slash: true}},
		{"/start deep-link-payload", commands.Command{Type: commands.Start, Slash: true}},
		{"/help", commands.Command{Type: commands.Help, Slash: true}},
		{"/forget", commands.Command{Type: commands.Forget, Slash: true}},
		{"/forget confirm", commands.Command{Type: commands.Forget, Slash: true, Confirm: true}},
		{"/forget  CONFIRM ", commands.Command{Type: commands.Forget, Slash: true, Confirm: true}},
		{"/forget please", commands.Command{Type: commands.Forget, Slash: true}},

		{"/enter dark-forest-01", commands.Command{Type: commands.Enter, Target: "dark-forest-01", Slash: true}},
		{"/enter", commands.Command{Type: commands.Enter, Slash: true}},
		{"/leave", commands.Command{Type: commands.Leave, Slash: true}},
		{"/look", commands.Command{Type: commands.Look, Slash: true}},
		{"/attack wolf-alpha", commands.Command{Type: commands.Attack, Target: "wolf-alpha", Slash: true}},
		{"/attack", commands.Command{Type: commands.Attack, Slash: true}},
		{"/flee", commands.Command{Type: commands.Flee, Slash: true}},
		{"/say привет, лес", commands.Command{Type: commands.Say, Text: "привет, лес", Slash: true}},
		{"/rest", commands.Command{Type: commands.Rest, Slash: true}},
		{"/status", commands.Command{Type: commands.Status, Slash: true}},
		{"/defend", commands.Command{Type: commands.Defend, Slash: true}},
		{"/group create", commands.Command{Type: commands.GroupCreate, Slash: true}},
		{"/group join grp-42", commands.Command{Type: commands.GroupJoin, Target: "grp-42", Slash: true}},
		{"/group leave", commands.Command{Type: commands.GroupLeave, Slash: true}},

		// Telegram appends the bot name to a command picked from the menu.
		{"/look@multiverse_bot", commands.Command{Type: commands.Look, Slash: true}},
		{"/ATTACK Wolf-Alpha", commands.Command{Type: commands.Attack, Target: "Wolf-Alpha", Slash: true}},
		{"  /rest  ", commands.Command{Type: commands.Rest, Slash: true}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := commands.Parse(c.in); got != c.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestParseSynonymsWithoutSlash(t *testing.T) {
	cases := []struct {
		in   string
		want commands.Command
	}{
		{"look", commands.Command{Type: commands.Look}},
		{"Look", commands.Command{Type: commands.Look}},
		{"leave", commands.Command{Type: commands.Leave}},
		{"flee", commands.Command{Type: commands.Flee}},
		{"rest", commands.Command{Type: commands.Rest}},
		{"defend", commands.Command{Type: commands.Defend}},
		{"status", commands.Command{Type: commands.Status}},
		{"enter dark-forest-01", commands.Command{Type: commands.Enter, Target: "dark-forest-01"}},
		{"attack wolf-alpha", commands.Command{Type: commands.Attack, Target: "wolf-alpha"}},
		{"attack", commands.Command{Type: commands.Attack}},
		{"say hello there", commands.Command{Type: commands.Say, Text: "hello there"}},
		{"group create", commands.Command{Type: commands.GroupCreate}},
		{"group join grp-42", commands.Command{Type: commands.GroupJoin, Target: "grp-42"}},
		{"group leave", commands.Command{Type: commands.GroupLeave}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := commands.Parse(c.in); got != c.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestParseLeavesOrdinaryTextAndServiceWordsUnknown(t *testing.T) {
	for _, in := range []string{
		"leave me alone",
		"look at that tree",
		"attack the wolf now",
		"enter the dark forest",
		"group join",
		"group create now please",
		"start",
		"help",
		"forget",
		"forget confirm",
		"Подтверждаю: мне 18+ и я согласен",
		"Вася",
		"/dance",
		"/",
	} {
		got := commands.Parse(in)
		if got.Type != commands.Unknown {
			t.Fatalf("Parse(%q) = %+v, want unknown", in, got)
		}
		if got.Text != strings.TrimSpace(in) || got.Slash != strings.HasPrefix(in, "/") {
			t.Fatalf("Parse(%q) must keep the text and the slash for the flow: %+v", in, got)
		}
	}
	if got := commands.Parse("   "); got != (commands.Command{Type: commands.Unknown}) {
		t.Fatalf("Parse(blank) = %+v", got)
	}
	got := commands.Parse("/group dance")
	if got.Type != commands.Unknown || got.Problem != commands.GroupSubcommand || got.Hint == "" {
		t.Fatalf("Parse(/group dance) = %+v, want unknown with a hint on the group commands", got)
	}
}

func TestParseLimitsSay(t *testing.T) {
	exact := strings.Repeat("ё", commands.MaxSayRunes)
	if got := commands.Parse("/say " + exact + "   "); got.Type != commands.Say || got.Problem != commands.NoProblem || got.Text != exact {
		t.Fatalf("say of exactly %d characters (after trim) must pass: problem %q, %d runes", commands.MaxSayRunes, got.Problem, len([]rune(got.Text)))
	}
	long := commands.Parse("/say " + exact + "ж")
	if long.Type != commands.Say || long.Problem != commands.TextTooLong {
		t.Fatalf("say of %d characters: %+v", commands.MaxSayRunes+1, long.Problem)
	}
	if !strings.Contains(long.Hint, "501") || !strings.Contains(long.Hint, "500") {
		t.Fatalf("the hint must name the length and the limit: %q", long.Hint)
	}
	empty := commands.Parse("/say   ")
	if empty.Type != commands.Say || empty.Problem != commands.TextRequired || empty.Hint == "" {
		t.Fatalf("empty say: %+v", empty)
	}
	join := commands.Parse("/group join")
	if join.Type != commands.GroupJoin || join.Problem != commands.TargetRequired || join.Hint == "" {
		t.Fatalf("group join without id: %+v", join)
	}
	multiline := commands.Parse("/say первая строка\nвторая")
	if multiline.Text != "первая строка\nвторая" {
		t.Fatalf("say keeps inner line breaks: %q", multiline.Text)
	}
}

func TestActionKeyIsStableIrreversibleAndKeyed(t *testing.T) {
	salt := []byte("salt-of-the-test-process")
	key := commands.ActionKey(salt, 700123)
	if key != commands.ActionKey(salt, 700123) {
		t.Fatal("the same update must yield the same key on a repeat")
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(key) {
		t.Fatalf("key %q, want 32 lowercase hex characters", key)
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte("700123"))
	if want := hex.EncodeToString(mac.Sum(nil))[:32]; key != want {
		t.Fatalf("key %q, want hex(HMAC-SHA256(salt, update_id))[:32] = %q", key, want)
	}
	if commands.ActionKey(salt, 700124) == key {
		t.Fatal("different updates must yield different keys")
	}
	if commands.ActionKey([]byte("another-salt-of-the-test"), 700123) == key {
		t.Fatal("the key must depend on the salt")
	}
	plain := sha256.Sum256([]byte("700123"))
	if strings.Contains(key, "700123") || key == hex.EncodeToString(plain[:])[:32] {
		t.Fatal("the key must not be the update id or its unkeyed hash")
	}
}
