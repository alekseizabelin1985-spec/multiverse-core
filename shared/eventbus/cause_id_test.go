package eventbus

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// cause is the event every test of this file derives from. Its id is fixed
// rather than taken from a generator, so that the expected ids below do not
// depend on how many events a test happened to build before.
func cause() Event {
	ev := NewRoot("player.attacked", "gateway", "dark-forest-world",
		&ScopeRef{ID: "solo:player-A", Type: "solo"}, ActorHuman, map[string]any{"target": "wolf"})
	ev.ID = "ev-1"
	ev.Meta.CorrelationID = "ev-1"
	return ev
}

func TestCauseIDNamespaceIsReproducible(t *testing.T) {
	want := uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://multiverse-core.io/eventbus/cause-id")).String()
	if CauseIDNamespace != want {
		t.Errorf("CauseIDNamespace = %s, want %s — the UUIDv5 its comment says it is", CauseIDNamespace, want)
	}
}

func TestWithCauseIDIsTheSameWhateverTheGenerator(t *testing.T) {
	deterministicSources(t)
	parent := cause()

	// The default generator, the sequence of tests and replay, and a second
	// sequence that has already moved on: a derived id must not see any of
	// them (C-01 v1.4, NFR-061; the finding of T-255 — the constructors read
	// the process-wide generator, not Deps.IDs).
	sources := map[string]func() string{
		"uuid v4":        nil,
		"sequence ev":    SequenceIDs("ev"),
		"sequence other": SequenceIDs("other"),
	}
	ids := make(map[string]string, len(sources))
	for name, gen := range sources {
		SetIDSource(gen)
		ids[name] = Derive(parent, "narrative.output", "core/swarm", nil, WithCauseID("turn")).ID
	}

	first := ids["uuid v4"]
	for name, id := range ids {
		if id != first {
			t.Errorf("with %s the id is %s, with uuid v4 it is %s: the derived id depends on the generator", name, id, first)
		}
	}
}

// TestWithCauseIDKeepsItsEncoding pins ids computed outside Go, over the same
// name: every component prefixed with its length in bytes, UTF-8. A journal
// recorded today must replay to the same bytes tomorrow, so the encoding is
// part of the contract even though no caller sees it (ADR-027, NFR-061).
//
// Every row goes through the option, not causeID, so that the option losing a
// part is caught as well as the encoding changing. The rows with several
// parts, an empty part and none at all hold the parts; the non-ASCII rows hold
// the length in bytes, which a length in runes would give differently. The
// values were computed twice, independently: by review #1 of T-417 and by the
// author (raw SHA-1 with the version bits set by hand, checked against
// Python's uuid.uuid5).
func TestWithCauseIDKeepsItsEncoding(t *testing.T) {
	deterministicSources(t)

	cases := []struct {
		cause, typ string
		parts      []string
		want       string
	}{
		{"ev-1", "narrative.output", []string{"turn"}, "bb274b03-fde8-5a4e-b1e1-c229f628425b"},
		{"ev-1", "encounter.started", []string{"enc-wolf-1"}, "854c166e-8870-5a29-8347-47d6d4f789ba"},
		{"ev-1", "t", []string{"12:ab", "3:x"}, "7d98e0bb-e5f9-56ca-97eb-3d4c1fe8c862"},
		{"ev-1", "t", []string{"a", ""}, "ac52f5dc-7bd0-5e72-8c5d-3b38556ce5e2"},
		{"ev-1", "t", nil, "056337ca-da42-50be-8cf1-f2e138401948"},
		{"ev-1", "narrative.output", []string{"смерть"}, "41f7097b-ef70-520d-a05d-53a503efcdc9"},
		{"ев-1", "тип", []string{"ё", "0"}, "e970d6c2-cf82-584a-a3d4-b4780581db6e"},
	}
	for _, c := range cases {
		parent := cause()
		parent.ID = c.cause
		if got := Derive(parent, c.typ, "core/swarm", nil, WithCauseID(c.parts...)).ID; got != c.want {
			t.Errorf("%s under %s with %q = %s, want %s", c.typ, c.cause, c.parts, got, c.want)
		}
	}
}

func TestWithCauseIDIsAValidEnvelopeID(t *testing.T) {
	deterministicSources(t)

	ev := Derive(cause(), "narrative.output", "core/swarm", nil, WithCauseID("turn"))

	parsed, err := uuid.Parse(ev.ID)
	if err != nil {
		t.Fatalf("id %q is not a UUID: %v", ev.ID, err)
	}
	if parsed.Version() != 5 {
		t.Errorf("id %s is a version %d UUID, want 5", ev.ID, parsed.Version())
	}
	// _envelope.json: id is a string of length at least 1.
	if err := ev.ValidateEnvelope(); err != nil {
		t.Errorf("an event with a derived id fails the envelope check: %v", err)
	}
}

// TestWithCauseIDApplicability walks the table of ADR-027 p. 2. Every "same"
// row is a repeat that consumers must drop; every "different" row is two
// events that the right parts keep apart. The last row is the misuse the ADR
// forbids, pinned so that the rule has a reason anyone can run.
func TestWithCauseIDApplicability(t *testing.T) {
	deterministicSources(t)

	type side struct {
		cause string
		typ   string
		parts []string
	}
	cases := []struct {
		name string
		a, b side
		same bool
	}{
		{
			name: "narrative.output: a retried publish of one kind is one event",
			a:    side{"ev-1", "narrative.output", []string{"turn"}},
			b:    side{"ev-1", "narrative.output", []string{"turn"}},
			same: true,
		},
		{
			name: "narrative.output: two kinds for one cause are two events",
			a:    side{"ev-1", "narrative.output", []string{"turn"}},
			b:    side{"ev-1", "narrative.output", []string{"death"}},
		},
		{
			name: "encounter.started and encounter.ended of one encounter are two events",
			a:    side{"ev-1", "encounter.started", []string{"enc-wolf-1"}},
			b:    side{"ev-1", "encounter.ended", []string{"enc-wolf-1"}},
		},
		{
			name: "encounter.started: two encounters opened by one fact are two events",
			a:    side{"ev-1", "encounter.started", []string{"enc-wolf-1"}},
			b:    side{"ev-1", "encounter.started", []string{"enc-wolf-2"}},
		},
		{
			name: "encounter.ended: the same id after a restart as before it",
			a:    side{"ev-7", "encounter.ended", []string{"enc-wolf-1"}},
			b:    side{"ev-7", "encounter.ended", []string{"enc-wolf-1"}},
			same: true,
		},
		{
			name: "entity facts: two entities of one proposal are two facts",
			a:    side{"ev-1", "entity.created", []string{"npc-wolf-1"}},
			b:    side{"ev-1", "entity.created", []string{"npc-wolf-2"}},
		},
		{
			name: "one type and parts under two causes are two events",
			a:    side{"ev-1", "narrative.output", []string{"turn"}},
			b:    side{"ev-2", "narrative.output", []string{"turn"}},
		},
		{
			name: "dice.rolled with the index of the roll: two rolls are two events",
			a:    side{"ev-1", "dice.rolled", []string{"0"}},
			b:    side{"ev-1", "dice.rolled", []string{"1"}},
		},
		{
			name: "dice.rolled without the index glues two rolls into one — why ADR-027 forbids it",
			a:    side{"ev-1", "dice.rolled", nil},
			b:    side{"ev-1", "dice.rolled", nil},
			same: true,
		},
	}
	derive := func(s side) string {
		parent := cause()
		parent.ID = s.cause
		return Derive(parent, s.typ, "core/swarm", nil, WithCauseID(s.parts...)).ID
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, b := derive(c.a), derive(c.b)
			if c.same && a != b {
				t.Errorf("ids differ (%s, %s): a repeat would reach consumers as a new event", a, b)
			}
			if !c.same && a == b {
				t.Errorf("both events got id %s: consumers would drop the second one as a duplicate", a)
			}
		})
	}
}

// TestWithCauseIDDoesNotGlueNeighbouringComponents is the encoding seen from
// the other side: components that a plain join would run together must still
// give different ids.
func TestWithCauseIDDoesNotGlueNeighbouringComponents(t *testing.T) {
	cases := []struct {
		name string
		a, b [3]any // cause, type, parts
	}{
		{"one part against two", [3]any{"ev-1", "t", []string{"ab"}}, [3]any{"ev-1", "t", []string{"a", "b"}}},
		{"a separator inside a part", [3]any{"ev-1", "t", []string{"a|b"}}, [3]any{"ev-1", "t", []string{"a", "b"}}},
		{"an empty part", [3]any{"ev-1", "t", []string{"a", ""}}, [3]any{"ev-1", "t", []string{"a"}}},
		{"the order of the parts", [3]any{"ev-1", "t", []string{"a", "b"}}, [3]any{"ev-1", "t", []string{"b", "a"}}},
		{"cause against type", [3]any{"ev-1x", "t", []string(nil)}, [3]any{"ev-1", "xt", []string(nil)}},
		{"type against parts", [3]any{"ev-1", "tx", []string(nil)}, [3]any{"ev-1", "t", []string{"x"}}},
	}
	id := func(c [3]any) string { return causeID(c[0].(string), c[1].(string), c[2].([]string)) }
	for _, c := range cases {
		if a, b := id(c.a), id(c.b); a == b {
			t.Errorf("%s: %v and %v both give %s", c.name, c.a, c.b, a)
		}
	}
}

func TestWithCauseIDPanicsOnARootEvent(t *testing.T) {
	deterministicSources(t)

	for name, build := range map[string]func(){
		"NewRoot": func() {
			NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil, WithCauseID("x"))
		},
		"Derive from an envelope without an id": func() {
			Derive(Event{Type: "player.attacked"}, "narrative.output", "core/swarm", nil, WithCauseID("x"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("no panic: an event without a cause got an id derived from nothing")
				}
				if msg, _ := r.(string); !strings.Contains(msg, "WithCauseID") {
					t.Errorf("panic %v does not name the option", r)
				}
			}()
			build()
		})
	}
}

func TestWithCauseIDChangesOnlyTheID(t *testing.T) {
	deterministicSources(t)
	parent := cause()
	agent := WithAgent(AgentRef{ID: "narrator", Level: "task", Blueprint: "narrator"})
	payload := map[string]any{"text": "the wolf falls"}

	plain := Derive(parent, "narrative.output", "core/swarm", payload, agent)
	derived := Derive(parent, "narrative.output", "core/swarm", payload, WithCauseID("turn"), agent)

	if derived.ID == plain.ID {
		t.Fatalf("the option did not change the id %s", plain.ID)
	}
	if derived.Meta.CausationID != parent.ID || derived.Meta.CorrelationID != parent.ID {
		t.Errorf("trace = %+v, want the cause %s as causation and correlation", derived.Meta, parent.ID)
	}
	plain.ID, derived.ID = "", ""
	a, errA := json.Marshal(plain)
	b, errB := json.Marshal(derived)
	if errA != nil || errB != nil {
		t.Fatalf("marshal: %v, %v", errA, errB)
	}
	if string(a) != string(b) {
		t.Errorf("the option changed more than the id:\nwithout %s\nwith    %s", a, b)
	}
}

// TestWithCauseIDDoesNotConsumeTheGenerator: a derived event leaves the
// sequence alone, so adding the option to one publisher — or re-publishing an
// encounter.ended after a restart (ADR-026) — does not renumber every event
// built after it.
func TestWithCauseIDDoesNotConsumeTheGenerator(t *testing.T) {
	deterministicSources(t)

	root := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	Derive(root, "narrative.output", "core/swarm", nil, WithCauseID("turn"))
	next := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)

	if root.ID != "ev-1" || next.ID != "ev-2" {
		t.Errorf("ids = %s, %s, want ev-1, ev-2: the derived event took a number from the sequence", root.ID, next.ID)
	}
}

// TestDeriveWithoutTheOptionTakesTheNextGeneratorID pins the other side of
// the option: without it Derive takes exactly one id from the generator, as it
// did before C-01 v1.4. No test said so while the id was assigned in the
// struct literal; now that it is assigned after the options, one must.
func TestDeriveWithoutTheOptionTakesTheNextGeneratorID(t *testing.T) {
	deterministicSources(t)

	root := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	child := Derive(root, "combat.decided", "core/swarm", nil,
		WithAgent(AgentRef{ID: "encounter-wolf", Level: "task", Blueprint: "encounter-wolf"}))
	next := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)

	if got := []string{root.ID, child.ID, next.ID}; strings.Join(got, ",") != "ev-1,ev-2,ev-3" {
		t.Errorf("ids = %v, want ev-1, ev-2, ev-3: Derive without WithCauseID must take one id from the generator", got)
	}
}

func TestWithCauseIDKeepsThePartsItWasGiven(t *testing.T) {
	deterministicSources(t)

	parts := []string{"turn"}
	opt := WithCauseID(parts...)
	parts[0] = "death"

	got := Derive(cause(), "narrative.output", "core/swarm", nil, opt).ID
	want := Derive(cause(), "narrative.output", "core/swarm", nil, WithCauseID("turn")).ID
	if got != want {
		t.Errorf("id = %s, want %s: the caller's slice changed an option built before", got, want)
	}
}
