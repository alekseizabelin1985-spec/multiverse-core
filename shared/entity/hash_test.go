package entity_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
)

// A map iterates in a different order every time it is built, so building the
// same attributes repeatedly is the shuffle this test needs.
func TestCanonicalJSONDoesNotDependOnMapOrder(t *testing.T) {
	build := func() *entity.Entity {
		return entity.New(entity.Ref{ID: "npc-1", Type: entity.TypeNPC}, "w", "wolf", map[string]any{
			"zulu": 1, "alpha": 2, "mike": 3, "kilo": 4, "echo": 5, "sierra": 6,
			"nested": map[string]any{"z": true, "a": false, "m": nil},
		}, proposedAt)
	}
	want := string(entity.CanonicalJSON(build()))
	for i := 0; i < 64; i++ {
		if got := string(entity.CanonicalJSON(build())); got != want {
			t.Fatalf("CanonicalJSON is not stable:\n got %s\nwant %s", got, want)
		}
	}
	if !json.Valid([]byte(want)) {
		t.Fatalf("CanonicalJSON is not valid JSON: %s", want)
	}
	if strings.Contains(want, " ") {
		t.Fatalf("CanonicalJSON has whitespace in it: %s", want)
	}
	if !strings.HasPrefix(want, `{"attributes":`) {
		t.Fatalf("CanonicalJSON = %s, want every key sorted, the top level included", want)
	}
}

func TestCanonicalJSONIgnoresHowTheEntityGotHere(t *testing.T) {
	e := player(t)
	before := string(entity.CanonicalJSON(e))

	e.UpdatedAt = e.UpdatedAt.Add(72 * time.Hour)
	e.LastEventID = "ev-9"
	e.History = append(e.History, entity.HistoryEntry{Version: 4, EventID: "ev-9"})
	e.LastChange = &entity.LastChange{ProposalID: "p-9", AppliedAt: proposedAt.Add(99 * time.Hour)}
	e.Name = "another name entirely"
	e.WorldID = "some-other-world"

	if after := string(entity.CanonicalJSON(e)); after != before {
		t.Fatalf("CanonicalJSON changed with the bookkeeping:\nbefore %s\nafter  %s", before, after)
	}
}

func TestCanonicalJSONReactsToTheGameState(t *testing.T) {
	e := player(t)
	before := string(entity.CanonicalJSON(e))

	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -4})
	e.Commit(attrs, changed, entity.LastChange{ProposalID: "p-1", AppliedAt: proposedAt})

	if after := string(entity.CanonicalJSON(e)); after == before {
		t.Fatalf("CanonicalJSON = %s for both 10 and 6 hit points", after)
	}
}

func TestCanonicalJSONWritesOneNumberFormat(t *testing.T) {
	whole := entity.New(entity.Ref{ID: "x", Type: entity.TypeNPC}, "w", "", map[string]any{"hp": 10}, proposedAt)
	fromTheWire := entity.New(entity.Ref{ID: "x", Type: entity.TypeNPC}, "w", "", map[string]any{"hp": float64(10)}, proposedAt)
	if string(entity.CanonicalJSON(whole)) != string(entity.CanonicalJSON(fromTheWire)) {
		t.Fatalf("10 and 10.0 hash differently:\n%s\n%s",
			entity.CanonicalJSON(whole), entity.CanonicalJSON(fromTheWire))
	}

	big := entity.New(entity.Ref{ID: "x", Type: entity.TypeNPC}, "w", "", map[string]any{
		"large": 1e21, "small": 0.000001,
	}, proposedAt)
	encoded := string(entity.CanonicalJSON(big))
	for _, want := range []string{`"large":1000000000000000000000`, `"small":0.000001`} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("CanonicalJSON = %s, want %s written without an exponent", encoded, want)
		}
	}
}

func TestStateHashDoesNotDependOnTheOrderOfTheWorld(t *testing.T) {
	world := entity.New(entity.Ref{ID: "dark-forest-world", Type: entity.TypeWorld}, "", "", map[string]any{"day": 1}, proposedAt)
	region := entity.New(entity.Ref{ID: "dark-forest-01", Type: entity.TypeRegion}, "dark-forest-world", "", nil, proposedAt)
	wolf := entity.New(entity.Ref{ID: "wolf-alpha", Type: entity.TypeNPC}, "dark-forest-world", "", nil, proposedAt)
	me := player(t)

	forwards := entity.StateHash([]*entity.Entity{world, region, wolf, me})
	backwards := entity.StateHash([]*entity.Entity{me, wolf, region, world})
	if forwards != backwards {
		t.Fatalf("StateHash depends on the order: %s vs %s", forwards, backwards)
	}
	if withHole := entity.StateHash([]*entity.Entity{me, nil, wolf, region, world}); withHole != forwards {
		t.Fatalf("StateHash counted a nil entity: %s vs %s", withHole, forwards)
	}
}

func TestStateHashFormat(t *testing.T) {
	hash := entity.StateHash([]*entity.Entity{player(t)})
	if !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("StateHash = %q, want the sha256: prefix", hash)
	}
	if len(hash) != len("sha256:")+64 {
		t.Fatalf("StateHash = %q, want 64 hex characters", hash)
	}
	if empty := entity.StateHash(nil); !strings.HasPrefix(empty, "sha256:") {
		t.Fatalf("StateHash(nil) = %q, want the hash of nothing, not an empty string", empty)
	}
}

func TestStateHashFollowsTheState(t *testing.T) {
	before := entity.StateHash([]*entity.Entity{player(t)})

	wounded := player(t)
	attrs, changed := applyOK(t, wounded, entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1})
	wounded.Commit(attrs, changed, entity.LastChange{ProposalID: "p", AppliedAt: proposedAt})

	if after := entity.StateHash([]*entity.Entity{wounded}); after == before {
		t.Fatal("StateHash did not move when a character took damage")
	}
}

// goldenEntity is the fixture behind the literals below: every kind of value
// the canonical form has to write — a whole number, a number read off the wire,
// a fraction, a boolean, a null, a nested object, a list with a null in it and
// a non-ASCII string — in an entity whose version is pinned.
func goldenEntity() *entity.Entity {
	e := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, "dark-forest-world", "Аня", map[string]any{
		"hp":        float64(10),
		"hp_max":    10,
		"status":    entity.StatusAlive,
		"canon":     true,
		"chance":    0.25,
		"died_at":   nil,
		"scope":     map[string]any{"id": "player-A", "type": "solo"},
		"inventory": []any{map[string]any{"item_id": "item-1", "name": "волчья шкура"}},
		"tags":      []any{"wounded", nil},
	}, proposedAt)
	e.Version = 7
	return e
}

// The canonical form and the hash of one fixed entity, written out in full.
//
// The DoD asks that the hash be stable between runs, and reassembling the same
// map inside one process proves the sort but not the format: a change to the
// encoder — a field added to the canonical object, a number written another way
// — would silently invalidate every snapshot.state_hash already stored and not
// fail a single test. These two literals are what such a change has to argue
// with. Changing them is changing the contract of §3.3 and belongs to
// system-architect.
func TestCanonicalJSONAndStateHashGolden(t *testing.T) {
	const wantJSON = `{"attributes":{"canon":true,"chance":0.25,"died_at":null,"hp":10,"hp_max":10,"inventory":[{"item_id":"item-1","name":"волчья шкура"}],"scope":{"id":"player-A","type":"solo"},"status":"alive","tags":["wounded",null]},"id":"player-A","type":"player","version":7}`
	const wantHash = `sha256:9a5c1fa6917d0e8bce7de56eede3da47243a9381a6e2a78e16eb79e4a4acce72`

	if got := string(entity.CanonicalJSON(goldenEntity())); got != wantJSON {
		t.Fatalf("CanonicalJSON =\n%s\nwant\n%s", got, wantJSON)
	}
	if got := entity.StateHash([]*entity.Entity{goldenEntity()}); got != wantHash {
		t.Fatalf("StateHash = %s, want %s", got, wantHash)
	}
}
