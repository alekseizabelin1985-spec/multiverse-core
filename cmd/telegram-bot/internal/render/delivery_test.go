package render_test

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
)

func delivery(kind, generatedBy, text string) api.Delivery {
	return api.Delivery{ID: "d-1", PlayerID: "player-A", Kind: kind, GeneratedBy: generatedBy, Text: text}
}

func TestMechanicsGroupAndAckAreSentAsTheyAre(t *testing.T) {
	for _, kind := range []string{render.KindMechanics, render.KindGroup, render.KindAck} {
		got := render.Delivery(delivery(kind, render.GeneratedByRules, "Попадание! Урон 3. [x](http://evil) <a href=x>*b*</a>"))
		want := []render.Reply{{Text: "Попадание! Урон 3. [x](http://evil) <a href=x>*b*</a>"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %+v, want %+v", kind, got, want)
		}
	}
}

// Every narrative carries the mark of its source (NFR-045, FR-052).
func TestEveryNarrativeIsMarked(t *testing.T) {
	reason := "llm_unavailable"
	cases := []struct {
		d    api.Delivery
		want string
	}{
		{delivery(render.KindNarrative, render.GeneratedByLLM, "Волк рычит."), "Волк рычит.\n— текст создан ИИ"},
		{func() api.Delivery {
			d := delivery(render.KindNarrative, render.GeneratedByTemplate, "Волк рычит.")
			d.FallbackReason = &reason
			return d
		}(), "Волк рычит.\n— упрощённый режим (шаблон): llm_unavailable"},
		{delivery(render.KindNarrative, render.GeneratedByTemplate, "Волк рычит."), "Волк рычит.\n— упрощённый режим (шаблон)"},
		{delivery(render.KindNarrative, "", "Волк рычит."), "Волк рычит.\n— текст создан ИИ"},
		{delivery(render.KindNarrative, "something_new", "Волк рычит."), "Волк рычит.\n— текст создан ИИ"},
	}
	for _, c := range cases {
		got := render.Delivery(c.d)
		if len(got) != 1 || got[0].Text != c.want {
			t.Errorf("generated_by %q: %+v, want %q", c.d.GeneratedBy, got, c.want)
		}
	}
}

func TestWorldEventAndSystemCarryTheirKeyboards(t *testing.T) {
	ev := render.Delivery(delivery(render.KindWorldEvent, render.GeneratedByRules, "Из тени выходит волк."))
	if len(ev) != 1 || !reflect.DeepEqual(ev[0].Keyboard, &sender.Keyboard{Rows: [][]string{{"/attack"}, {"/flee", "/defend"}}}) {
		t.Errorf("world_event = %+v, want the combat keyboard", ev)
	}
	sys := render.Delivery(delivery(render.KindSystem, render.GeneratedByRules, "Ваш персонаж погиб. /start — создать нового"))
	if len(sys) != 1 || !reflect.DeepEqual(sys[0].Keyboard, &sender.Keyboard{Rows: [][]string{{"/start"}}}) {
		t.Errorf("system = %+v, want [/start]", sys)
	}
}

func TestADeliveryWithoutTextGivesNoMessage(t *testing.T) {
	if got := render.Delivery(delivery(render.KindMechanics, render.GeneratedByRules, "  \n")); got != nil {
		t.Errorf("empty mechanics = %+v, want no message", got)
	}
	// N-4 of review #1 of T-311: a narrative without text is not a message of
	// its mark alone.
	for _, text := range []string{"", " \n\n "} {
		for _, by := range []string{render.GeneratedByLLM, render.GeneratedByTemplate} {
			if got := render.Delivery(delivery(render.KindNarrative, by, text)); got != nil {
				t.Errorf("narrative %q by %s = %+v, want no message", text, by, got)
			}
		}
	}
}

func units(s string) int { return len(utf16.Encode([]rune(s))) }

func TestLongTextsAreCutByParagraphs(t *testing.T) {
	para := strings.Repeat("Волк бежит по лесу. ", 100) // 2000 characters
	text := strings.Join([]string{para, para, para, para}, "\n\n")
	parts := render.Split(text)
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2 (two paragraphs of 2000 fit one message of 4096)", len(parts))
	}
	for i, p := range parts {
		if units(p) > render.MaxMessageLen {
			t.Errorf("part %d is %d units long", i, units(p))
		}
		if p != para+"\n\n"+para {
			t.Errorf("part %d is not two whole paragraphs", i)
		}
	}
}

func TestAParagraphTooLongIsCutByLinesThenWords(t *testing.T) {
	line := strings.Repeat("слово ", 500) // 3000 characters
	paragraph := line + "\n" + line
	parts := render.Split(paragraph)
	if len(parts) != 2 || parts[0] != line || parts[1] != line {
		t.Errorf("a paragraph of two long lines is cut into %d parts, want the two lines", len(parts))
	}

	long := strings.Repeat("слово ", 1500) // one line of 9000 characters
	parts = render.Split(long)
	if len(parts) < 3 {
		t.Fatalf("parts = %d, want at least 3", len(parts))
	}
	for i, p := range parts {
		if units(p) > render.MaxMessageLen {
			t.Errorf("part %d is %d units long", i, units(p))
		}
		if strings.HasPrefix(p, "ово") || strings.HasSuffix(p, "сл") {
			t.Errorf("part %d is cut inside a word", i)
		}
	}
	if strings.Join(strings.Fields(strings.Join(parts, " ")), " ") != strings.Join(strings.Fields(long), " ") {
		t.Error("the parts lost or reordered words")
	}
}

func TestSplitCountsWhatTelegramCounts(t *testing.T) {
	emoji := strings.Repeat("🐺", 3000) // 3000 runes, 6000 UTF-16 units, no space
	parts := render.Split(emoji)
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if strings.Join(parts, "") != emoji {
		t.Error("the parts of a word cut inside lost characters")
	}
	for i, p := range parts {
		if units(p) > render.MaxMessageLen {
			t.Errorf("part %d is %d units long", i, units(p))
		}
	}
}

func TestTheKeyboardGoesWithTheLastPart(t *testing.T) {
	text := strings.Repeat("а", 4000) + "\n\n" + strings.Repeat("б", 100)
	got := render.Delivery(delivery(render.KindSystem, render.GeneratedByRules, text))
	if len(got) != 2 || got[0].Keyboard != nil || got[1].Keyboard == nil {
		t.Errorf("parts = %+v, want two with the keyboard on the second", len(got))
	}
}

func TestKeyboardsAreThePlainCommands(t *testing.T) {
	npcs := []api.NPCView{{NPCID: "wolf-alpha", Status: "alive"}, {NPCID: "wolf-beta", Status: "dead"}, {NPCID: "wolf-gamma", Status: "alive"}}
	cases := map[string]struct {
		got  *sender.Keyboard
		want [][]string
	}{
		"consent":      {render.ConsentKeyboard(), [][]string{{render.ConsentButton, render.DeclineButton}}},
		"name confirm": {render.NameConfirmKeyboard(), [][]string{{"Да", "Ввести другое"}}},
		"base":         {render.BaseKeyboard(), [][]string{{"/look", "/status", "/rest"}}},
		"combat":       {render.CombatKeyboard(npcs), [][]string{{"/attack wolf-alpha", "/attack wolf-gamma"}, {"/flee", "/defend"}}},
		"targets":      {render.TargetsKeyboard(npcs), [][]string{{"/attack wolf-alpha"}, {"/attack wolf-gamma"}}},
		"regions":      {render.RegionsKeyboard([]api.RegionSummary{{RegionID: "dark-forest-01", Name: "Тёмный лес"}}), [][]string{{"/enter dark-forest-01"}}},
		"worlds":       {render.WorldsKeyboard([]api.WorldSummary{{WorldID: "w-1", Name: "Тёмный лес"}, {WorldID: "w-2"}}), [][]string{{"Тёмный лес"}, {"w-2"}}},
		"start":        {render.StartKeyboard(), [][]string{{"/start"}}},
	}
	for name, c := range cases {
		if c.got == nil || c.got.Remove || !reflect.DeepEqual(c.got.Rows, c.want) {
			t.Errorf("%s keyboard = %+v, want rows %q", name, c.got, c.want)
			continue
		}
		for _, row := range c.got.Rows {
			for _, label := range row {
				if strings.ContainsAny(label, "`*[<") {
					t.Errorf("%s keyboard label %q carries markup", name, label)
				}
			}
		}
	}
	if kb := render.RemoveKeyboard(); !kb.Remove || len(kb.Rows) != 0 {
		t.Errorf("RemoveKeyboard = %+v", kb)
	}
}

// Mi-5 of review #1 of T-311: no part of a cut text is empty or blank —
// Telegram refuses such a message — and the keyboard goes with the last part
// that has text.
func TestSplitGivesNoEmptyPart(t *testing.T) {
	long := strings.Repeat("а", render.MaxMessageLen+10)
	short := strings.Repeat("б", 100)
	cases := map[string]string{
		"a long line ending with a newline":         long + "\n",
		"a long paragraph ending with a blank line": long + "\n\n",
		"a blank line before a long paragraph":      "\n\n" + long,
		"blank paragraphs around a long one":        short + "\n\n  \n\n" + long + "\n\n \n\n",
		"a long word followed by spaces only":       long + "\n   ",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			parts := render.Split(text)
			if len(parts) == 0 {
				t.Fatal("no parts")
			}
			for i, p := range parts {
				if strings.TrimSpace(p) == "" {
					t.Errorf("part %d of %d is blank: %q", i, len(parts), p)
				}
				if units(p) > render.MaxMessageLen {
					t.Errorf("part %d is %d units long", i, units(p))
				}
			}
			got := render.Delivery(delivery(render.KindSystem, render.GeneratedByRules, text))
			if len(got) != len(parts) {
				t.Fatalf("delivery gives %d messages, Split %d parts", len(got), len(parts))
			}
			last := got[len(got)-1]
			if strings.TrimSpace(last.Text) == "" || !reflect.DeepEqual(last.Keyboard, render.StartKeyboard()) {
				t.Errorf("last message %q carries keyboard %+v, want a text with [/start]", last.Text, last.Keyboard)
			}
			for _, m := range got[:len(got)-1] {
				if m.Keyboard != nil {
					t.Error("a message before the last carries a keyboard")
				}
			}
		})
	}
	if got := render.Split(" \n\n "); got != nil {
		t.Errorf("Split of a blank text = %q, want no parts", got)
	}
}

// N-2 of review #1 of T-311: paragraphs that fill a message exactly go
// together.
func TestParagraphsThatFillAMessageExactlyGoTogether(t *testing.T) {
	first := strings.Repeat("а", 2000)
	second := strings.Repeat("б", render.MaxMessageLen-2000-2) // with "\n\n" the message is full
	third := strings.Repeat("в", 10)
	parts := render.Split(first + "\n\n" + second + "\n\n" + third)
	if want := []string{first + "\n\n" + second, third}; !reflect.DeepEqual(parts, want) {
		t.Errorf("parts = %d of lengths %v, want a full first message of %d units and the rest", len(parts), lengths(parts), render.MaxMessageLen)
	}
}

func lengths(parts []string) []int {
	out := make([]int, len(parts))
	for i, p := range parts {
		out[i] = units(p)
	}
	return out
}
