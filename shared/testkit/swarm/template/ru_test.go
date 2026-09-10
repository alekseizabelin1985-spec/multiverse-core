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
		Name: "Вася", Names: "Вася, Лена", Place: "Тёмный лес",
		HP: 7, HPMax: 10, Round: 3,
	}
}

// TestTheTableStaysUnderTheCeiling is the limit T-018 sets on the stub: at
// most ten texts. It is asserted against the table rather than against a
// number repeated in prose.
func TestTheTableStaysUnderTheCeiling(t *testing.T) {
	if got := template.Count(); got > 10 {
		t.Errorf("the table holds %d texts, the stub is allowed ten", got)
	}
	if template.Count() == 0 {
		t.Fatal("the table is empty")
	}
}

// TestEverySubstitutionIsMade walks enough seeds to reach every text of both
// tables and checks that none of them leaves a placeholder behind. A text that
// reaches a player with a brace in it is a defect the schema cannot catch: the
// contract only asks for a non-empty string.
func TestEverySubstitutionIsMade(t *testing.T) {
	seen := map[string]struct{}{}
	for i := range 200 {
		seed := "ev-" + strconv.Itoa(i)
		for _, text := range []string{
			template.Entry(seed, full()),
			template.Round(seed, full()),
		} {
			seen[text] = struct{}{}
			if strings.ContainsAny(text, "{}") {
				t.Fatalf("a placeholder was left in %q", text)
			}
			if text == "" {
				t.Fatal("an empty text")
			}
		}
	}
	if len(seen) != template.Count() {
		t.Errorf("%d distinct texts came out of %d in the table: a text no seed reaches "+
			"is a text nobody reviews", len(seen), template.Count())
	}
}

// TestTheSameEventAlwaysGetsTheSameText is what makes a replay reproducible:
// the text is a function of the event, not of a random source or of a map
// iteration (NFR-061).
func TestTheSameEventAlwaysGetsTheSameText(t *testing.T) {
	const seed = "ev-42"
	first := template.Entry(seed, full())
	for range 50 {
		if again := template.Entry(seed, full()); again != first {
			t.Fatalf("the same event produced two texts:\n  %q\n  %q", first, again)
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
	for i := range 50 {
		seed := "ev-" + strconv.Itoa(i)
		for _, text := range []string{template.Entry(seed, empty), template.Round(seed, empty)} {
			if !strings.Contains(text, "—") {
				t.Fatalf("nothing was known and nothing is marked missing: %q", text)
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
