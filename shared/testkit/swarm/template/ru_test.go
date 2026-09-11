package template_test

import (
	"strconv"
	"strings"
	"testing"

	"multiverse-core.io/shared/testkit/swarm/template"
)

// full is a character the narrator knows everything about.
func full() template.Fields {
	return template.Fields{
		Name: "Вася", Names: "Вася, Лена", Place: "Тёмный лес", Foe: "Альфа-волк",
		HP: 7, HPMax: 10, Round: 3, Dealt: 4, Taken: 2,
	}
}

// renderers are every way a text is rendered: one per kind, and one per way a
// turn can end. A kind or an outcome added to the package and not here leaves
// its texts unreached, which TestEverySubstitutionIsMade reports.
func renderers() map[string]func(seed string, f template.Fields) string {
	out := map[string]func(string, template.Fields) string{
		"entry":       template.Entry,
		"round":       template.Round,
		"death":       template.Death,
		"world_event": template.WorldEvent,
	}
	for _, o := range template.Outcomes() {
		out["turn/"+strconv.Itoa(int(o))] = func(seed string, f template.Fields) string {
			return template.Turn(seed, o, f)
		}
	}
	return out
}

// TestNoTextNeedsAParseMode is SEC-10 checked against the table as it is
// written: the bot sends a narrative as plain text, and a character a
// messenger reads as markup either shows up as litter or gets the message
// refused. The rendered texts are checked as well, because a placeholder is
// part of the text until it is substituted.
func TestNoTextNeedsAParseMode(t *testing.T) {
	const markup = "<>*_`"
	for i, text := range template.Texts() {
		if strings.ContainsAny(text, markup) {
			t.Errorf("text %d carries markup (one of %q): %q", i, markup, text)
		}
	}
	for kind, render := range renderers() {
		for i := range 100 {
			if text := render("ev-"+strconv.Itoa(i), full()); strings.ContainsAny(text, markup) {
				t.Errorf("%s renders markup (one of %q): %q", kind, markup, text)
			}
		}
	}
}

// TestEveryKindHasItsTexts: a kind of narrative with no text would make the
// narrator publish an empty string, which the schema refuses (text minLength
// 1) — the narrative would end up in dead_letters instead of in front of a
// player.
func TestEveryKindHasItsTexts(t *testing.T) {
	for kind, render := range renderers() {
		if render("ev-0", full()) == "" {
			t.Errorf("%s renders an empty text", kind)
		}
	}
	if len(template.Outcomes()) != 6 {
		t.Errorf("%d outcomes of a turn; an exchange ends in a hit, a miss, a kill, "+
			"an escape, a caught flight or a flight nothing struck at", len(template.Outcomes()))
	}
}

// TestAFlightNothingStruckAtNamesNoBlow is Mi-2 of the review of T-220 at the
// level of the table: a flight that failed with nothing striking — the wolf may
// be lying dead — is told without a blow, a foe or a number of damage, while
// the caught flight still tells the strike it took.
func TestAFlightNothingStruckAtNamesNoBlow(t *testing.T) {
	fields := full()
	fields.Taken = 3
	for i := range 50 {
		seed := "ev-" + strconv.Itoa(i)
		stuck := template.Turn(seed, template.Stuck, fields)
		for _, blow := range []string{"бьёт", "вдогонку", fields.Foe, "3"} {
			if strings.Contains(stuck, blow) {
				t.Fatalf("a flight nothing struck at says %q: %q", blow, stuck)
			}
		}
		if !strings.Contains(stuck, fields.Name) {
			t.Fatalf("a flight nothing struck at does not say whose it was: %q", stuck)
		}
		if caught := template.Turn(seed, template.Caught, fields); !strings.Contains(caught, "вдогонку") {
			t.Fatalf("a caught flight does not tell the strike it took: %q", caught)
		}
	}
}

// TestEverySubstitutionIsMade walks enough seeds to reach every text of every
// table and checks that none of them leaves a placeholder behind. A text that
// reaches a player with a brace in it is a defect the schema cannot catch: the
// contract only asks for a non-empty string.
func TestEverySubstitutionIsMade(t *testing.T) {
	seen := map[string]struct{}{}
	for kind, render := range renderers() {
		for i := range 200 {
			text := render("ev-"+strconv.Itoa(i), full())
			seen[text] = struct{}{}
			if strings.ContainsAny(text, "{}") {
				t.Fatalf("%s left a placeholder in %q", kind, text)
			}
		}
	}
	if len(seen) != template.Count() {
		t.Errorf("%d distinct texts came out of %d in the tables: a text no seed reaches "+
			"is a text nobody reviews", len(seen), template.Count())
	}
}

// TestTheSameEventAlwaysGetsTheSameText is what makes a replay reproducible:
// the text is a function of the event, not of a random source or of a map
// iteration (NFR-061).
func TestTheSameEventAlwaysGetsTheSameText(t *testing.T) {
	const seed = "ev-42"
	for kind, render := range renderers() {
		first := render(seed, full())
		for range 20 {
			if again := render(seed, full()); again != first {
				t.Fatalf("%s: the same event produced two texts:\n  %q\n  %q", kind, first, again)
			}
		}
	}
	// And two different events do not all collapse onto one text, or the seed
	// would not be doing anything.
	distinct := map[string]struct{}{}
	for i := range 50 {
		distinct[template.Entry("ev-"+strconv.Itoa(i), full())] = struct{}{}
	}
	if len(distinct) < 2 {
		t.Error("every event got the same text: the seed is being ignored")
	}
}

// TestWhatTheNarratorDoesNotKnowIsADash covers the degradation of the table: a
// value the stub has not heard of is a visible gap, not an empty space.
func TestWhatTheNarratorDoesNotKnowIsADash(t *testing.T) {
	empty := template.Fields{HP: template.Unknown, HPMax: template.Unknown, Round: template.Unknown}
	for kind, render := range renderers() {
		for i := range 50 {
			if text := render("ev-"+strconv.Itoa(i), empty); !strings.Contains(text, "—") {
				t.Fatalf("%s: nothing was known and nothing is marked missing: %q", kind, text)
			}
		}
	}
}

// TestZeroHealthIsANumberNotAGap: a character at 0 hp is a character the text
// has something to say about, and the dash is reserved for what the narrator
// has not heard.
func TestZeroHealthIsANumberNotAGap(t *testing.T) {
	fields := full()
	fields.HP = 0
	found := false
	for i := range 200 {
		text := template.Entry("ev-"+strconv.Itoa(i), fields)
		if strings.Contains(text, "0") {
			found = true
			break
		}
	}
	if !found {
		t.Error("no text ever printed a health of zero")
	}
}

// TestATurnSaysWhatTheExchangeDealt: the numbers of a turn are the ones the
// decisions carried, both ways, and a text that dropped one of them would tell
// a player a fight they did not have.
func TestATurnSaysWhatTheExchangeDealt(t *testing.T) {
	fields := full()
	fields.Dealt, fields.Taken = 5, 3
	for i := range 50 {
		seed := "ev-" + strconv.Itoa(i)
		if text := template.Turn(seed, template.Hit, fields); !strings.Contains(text, "5") ||
			!strings.Contains(text, "3") {
			t.Fatalf("a landed blow of 5 answered by 3 reads %q", text)
		}
		if text := template.Turn(seed, template.Caught, fields); !strings.Contains(text, "3") {
			t.Fatalf("a caught flight that cost 3 reads %q", text)
		}
		if text := template.Turn(seed, template.Fled, fields); !strings.Contains(text, fields.Foe) {
			t.Fatalf("an escape does not say whom it escaped: %q", text)
		}
	}
}
