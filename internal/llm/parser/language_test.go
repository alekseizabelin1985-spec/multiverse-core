package parser_test

import (
	"errors"
	"strings"
	"testing"

	"multiverse-core.io/internal/llm/parser"
	"multiverse-core.io/shared/env"
)

// NFR-090, ADR-016 p. 2: no character of Han, Hiragana, Katakana or Hangul,
// whatever the share of Latin.
func TestAnIdeographFailsTheLanguage(t *testing.T) {
	loose := parser.LanguagePolicy{MaxLatinRatio: 1}
	for name, text := range map[string]string{
		"Han":            "Волк 退后 в чащу.",
		"one Han":        "Волк отступает в чащу" + "。狼",
		"compatibility":  "Волк 豈 рычит.",
		"Hiragana":       "Волк ひらがな рычит.",
		"Katakana":       "Волк カタカナ рычит.",
		"Hangul":         "Волк 늑대 рычит.",
		"in a later one": "",
	} {
		texts := []string{text}
		if name == "in a later one" {
			texts = []string{"Волк рычит.", "Стая уходит.", "狼"}
		}
		err := parser.CheckLanguage(texts, loose)
		if !errors.Is(err, parser.ErrLanguage) {
			t.Errorf("%s: %v, want language", name, err)
			continue
		}
		if strings.ContainsAny(err.Error(), "退狼ひカ늑豈") {
			t.Errorf("%s: the error quotes the text: %v", name, err)
		}
	}
}

// The Latin letters of identifiers are not counted: player-A and wolf-alpha
// are names the narrative may carry (DoD T-209), and so are ids with a colon.
func TestTheLatinOfIdentifiersIsNotCounted(t *testing.T) {
	strict := parser.LanguagePolicy{MaxLatinRatio: 0}
	for _, text := range []string{
		"player-A бьёт wolf-alpha у кромки леса.",
		"Стая region:dark-forest затихает.",
		"Агент encounter-wolf:solo:p1 закрывает бой.",
		"npc-999 уходит, player-B остаётся.",
		"Числа 12 и 7 — не буквы.",
		"",
		"… 123 !?",
	} {
		if err := parser.CheckLanguage([]string{text}, strict); err != nil {
			t.Errorf("%q: %v", text, err)
		}
	}
	for _, text := range []string{
		"The wolf отступает.",
		"Волк player отступает.",
		"Волк wolf-волк отступает.",
		"Волк Region:Dark отступает.",
	} {
		if err := parser.CheckLanguage([]string{text}, strict); !errors.Is(err, parser.ErrLanguage) {
			t.Errorf("%q: %v, want language at the share 0", text, err)
		}
	}
}

// The shape of an id: lower-case segments joined by - or :, the last one
// possibly a single capital, or a UUID, or a ULID — the ids of the fixtures,
// blueprints and laws of the tree. English words joined by a hyphen with a
// capital do not have it, and their letters are counted. A lower-case phrase
// such as state-of-the-art has the shape of law-wolves-never-leave-the-region
// and is left out: only the ids a call knows tell the two apart.
func TestOnlyTheShapeOfAnIdIsNotCounted(t *testing.T) {
	strict := parser.LanguagePolicy{MaxLatinRatio: 0}
	ids := []string{
		"player-A", "wolf-alpha", "npc-999", "dark-forest-world", "global-dark-forest-world",
		"region:dark-forest", "encounter-wolf:solo:p1", "solo:player-A", "state:dark-forest-world:000000",
		"law-wolves-never-leave-the-region", "agent-encounter-1", "breach-1",
		"01JG0000000000000000000011", "7ZZZZZZZZZZZZZZZZZZZZZZZZZ",
		"123e4567-e89b-12d3-a456-426614174000", "123E4567-E89B-12D3-A456-426614174000",
	}
	for _, id := range ids {
		for _, text := range []string{
			"Волк " + id + " отступает.",
			id + ": волк отступает",
			"Волк (" + id + ") отступает, а " + id + ", " + id + "-" + " и -" + id + " тоже.",
			"—" + id + "—",
		} {
			if err := parser.CheckLanguage([]string{text}, strict); err != nil {
				t.Errorf("%q: %v", text, err)
			}
		}
	}
	for _, word := range []string{
		"Wi-Fi", "Hello-World", "Dark-Forest", "T-shirt", "Player-A", "player-AB", "wolf-A-alpha",
		"wolf_alpha", "x_wolf-alpha", "wolf-alpha_x", "player", "wolf:",
		"01JG000000000000000000001", "01JI0000000000000000000011", "01JG0000000000000000000011X",
		"123e4567-e89b-12d3-a456-42661417400G", "ÉCOLE-A",
	} {
		text := "Волк " + word + " отступает."
		if err := parser.CheckLanguage([]string{text}, strict); !errors.Is(err, parser.ErrLanguage) {
			t.Errorf("%q: %v, want language at the share 0", text, err)
		}
	}
}

// The ids a call knows are not counted whatever their shape; the words around
// them still are, and without the ids the same text fails.
func TestTheKnownIdsOfTheCallAreNotCounted(t *testing.T) {
	strict := parser.LanguagePolicy{MaxLatinRatio: 0}
	known := strict
	known.Identifiers = []string{"Alpha-Wolf", "wolf", "Wi-Fi"}
	for _, text := range []string{"Alpha-Wolf рычит.", "Стая wolf, wolf: и (Wi-Fi)."} {
		if err := parser.CheckLanguage([]string{text}, known); err != nil {
			t.Errorf("%q with known ids: %v", text, err)
		}
		if err := parser.CheckLanguage([]string{text}, strict); !errors.Is(err, parser.ErrLanguage) {
			t.Errorf("%q without known ids: %v, want language", text, err)
		}
	}
	for _, text := range []string{"Alpha-Wolfy рычит.", "Стая wolves уходит.", "Alpha-Wolf рычит, the pack."} {
		if err := parser.CheckLanguage([]string{text}, known); !errors.Is(err, parser.ErrLanguage) {
			t.Errorf("%q: %v, want language: only the known ids are left out", text, err)
		}
	}
}

// The share is Latin letters among all letters of all texts, compared with ≤:
// exactly the limit passes, one letter more fails.
func TestTheShareOfLatinIsComparedWithTheLimit(t *testing.T) {
	policy := parser.LanguagePolicy{MaxLatinRatio: 0.10}
	cyr := func(n int) string { return strings.Repeat("я", n) }
	lat := func(n int) string { return strings.Repeat("q", n) }
	cases := []struct {
		name  string
		texts []string
		ok    bool
	}{
		{"1 of 10", []string{lat(1) + " " + cyr(9)}, true},
		{"2 of 10", []string{lat(2) + " " + cyr(8)}, false},
		{"10 of 100", []string{lat(10) + " " + cyr(90)}, true},
		{"11 of 100", []string{lat(11) + " " + cyr(89)}, false},
		{"across texts", []string{lat(1), cyr(9)}, true},
		{"across texts, over", []string{lat(2), cyr(8)}, false},
		{"only Latin", []string{"The wolf steps back."}, false},
		{"accented Latin", []string{"é" + " " + cyr(1)}, false},
		{"an id is left out by its letters, not its digits", []string{lat(2) + " " + cyr(8) + " npc-999"}, false},
		{"an id next to the limit", []string{lat(1) + " " + cyr(9) + " npc-999"}, true},
		{"a word with _ is not an id, all its letters count", []string{cyr(80) + " x_wolf-alpha"}, false},
		{"no texts", nil, true},
	}
	for _, c := range cases {
		err := parser.CheckLanguage(c.texts, policy)
		if (err == nil) != c.ok {
			t.Errorf("%s: %v, want ok=%v", c.name, err, c.ok)
		}
		if err != nil && strings.Contains(err.Error(), "wolf") {
			t.Errorf("%s: the error quotes the text: %v", c.name, err)
		}
	}
}

func TestThePolicyIsReadFromTheManifest(t *testing.T) {
	p, err := parser.LanguagePolicyFrom(env.MapSource(nil))
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxLatinRatio != 0.10 {
		t.Errorf("default %v, want 0.10 (КД §14)", p.MaxLatinRatio)
	}
	for raw, want := range map[string]float64{"0": 0, "1": 1, "0.25": 0.25} {
		p, err := parser.LanguagePolicyFrom(env.MapSource(map[string]string{"MV_LLM_LATIN_MAX_RATIO": raw}))
		if err != nil || p.MaxLatinRatio != want {
			t.Errorf("%q: %v, %v", raw, p, err)
		}
	}
	for _, raw := range []string{"", "ten", "-0.1", "1.01", "NaN", "10%", "+Inf"} {
		_, err := parser.LanguagePolicyFrom(env.MapSource(map[string]string{"MV_LLM_LATIN_MAX_RATIO": raw}))
		if !errors.Is(err, parser.ErrLanguagePolicy) || !strings.Contains(err.Error(), "MV_LLM_LATIN_MAX_RATIO") {
			t.Errorf("%q: %v, want an error naming the variable", raw, err)
		}
	}
}
