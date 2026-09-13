package entity_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
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

// roundtrip is an entity after a snapshot: encoded the way State writes it and
// decoded the way recovery reads it.
func roundtrip(t *testing.T, e *entity.Entity) *entity.Entity {
	t.Helper()
	encoded, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back entity.Entity
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &back
}

// textStamp encodes itself through MarshalText, the way a net.IP or a custom
// identifier type does.
type textStamp string

func (s textStamp) MarshalText() ([]byte, error) { return []byte("stamp:" + string(s)), nil }

// dangerLevel is a number in Go and a word on the wire: its kind says int, its
// MarshalJSON says otherwise, and the wire believes the method.
type dangerLevel int

func (l dangerLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"level": int(l), "name": "wolf-pack"})
}

// tagSet is a list in Go and a comma-joined string on the wire.
type tagSet []string

func (s tagSet) MarshalText() ([]byte, error) { return []byte(strings.Join(s, ",")), nil }

// T-050. A world assembled in Go — a bootstrap from fixtures, an op that
// appended an entity.Item — and the same world read back from its snapshot hash
// the same (§3.3, §4.4): StateHash(e) == StateHash(roundtrip(e)) for every Go
// shape an attribute can hold. Before, a struct kept its fields in declaration
// order, a typed nil list or map was written empty, a float32 was widened to
// the float64 nearest it, and a []byte or a type with its own encoding was
// written by its kind — each of them a divergence a recovery would report and
// nobody caused (review #1, Mi-1).
//
// Integers past ±2^53 are not here on purpose: JSON loses them, and what to do
// about it is a decision of system-architect (backlog of the review).
func TestStateHashOfAGoBuiltWorldMatchesItsWireForm(t *testing.T) {
	flee := 2
	fleePtr := &flee
	var noTarget *string
	forms := map[string]any{
		"int32":                   int32(7),
		"uint16":                  uint16(10),
		"int64":                   int64(-3),
		"pointer":                 &flee,
		"pointer to a pointer":    &fleePtr,
		"nil pointer":             noTarget,
		"float32 exact":           float32(1.5),
		"float32 inexact":         float32(0.1),
		"float32 large":           float32(3.4e38),
		"float64 inexact":         0.1,
		"struct":                  pelt(),
		"list of structs":         []entity.Item{pelt()},
		"list of strings":         []string{"wolf-alpha", "wolf-beta"},
		"nil list of strings":     []string(nil),
		"empty list":              []string{},
		"nil list of any":         []any(nil),
		"nil list of objects":     []map[string]any(nil),
		"map of ints":             map[string]int{"wolf-beta": 3, "wolf-alpha": 4},
		"nil map of ints":         map[string]int(nil),
		"nil map of any":          map[string]any(nil),
		"map with int keys":       map[int]string{2: "b", 10: "a"},
		"array":                   [2]int{1, 2},
		"bytes":                   []byte("pelt"),
		"nil bytes":               []byte(nil),
		"raw JSON":                json.RawMessage(`{"z":1,"a":[true,null]}`),
		"text marshaler":          textStamp("dawn"),
		"number with MarshalJSON": dangerLevel(3),
		"list with MarshalText":   tagSet{"hunted", "wounded"},
		"time":                    proposedAt.In(time.FixedZone("MSK", 3*60*60)),
		"scope ref":               eventbus.ScopeRef{ID: "player-A", Type: "solo"},
		"nested in a list":        []any{[]string(nil), float32(0.1), map[string]int(nil)},
		"nested in an object":     map[string]any{"bytes": []byte{0, 255}, "at": float32(0.2)},
	}

	ref := entity.Ref{ID: "player-A", Type: entity.TypePlayer}
	for name, value := range forms {
		t.Run(name, func(t *testing.T) {
			built := entity.New(ref, "dark-forest-world", "Аня", map[string]any{"value": value}, proposedAt)
			back := roundtrip(t, built)
			inMemory, onDisk := string(entity.CanonicalJSON(built)), string(entity.CanonicalJSON(back))
			if inMemory != onDisk {
				t.Fatalf("CanonicalJSON differs between the entity and its snapshot:\nin memory %s\non disk   %s", inMemory, onDisk)
			}
			if entity.StateHash([]*entity.Entity{built}) != entity.StateHash([]*entity.Entity{back}) {
				t.Fatal("StateHash differs between the entity and its snapshot")
			}
		})
	}

	// All of them at once, and the one literal that says the keys of a struct
	// are sorted like those of any other object.
	all := entity.New(ref, "dark-forest-world", "Аня", forms, proposedAt)
	if entity.StateHash([]*entity.Entity{all}) != entity.StateHash([]*entity.Entity{roundtrip(t, all)}) {
		t.Fatal("StateHash differs between the whole entity and its snapshot")
	}
	if encoded := string(entity.CanonicalJSON(all)); !strings.Contains(encoded, `{"acquired_at":"2026-09-09T10:15:00Z","item_id":`) {
		t.Fatalf("CanonicalJSON = %s, want the keys of a struct sorted like any other object", encoded)
	}
}

// The reproduction of review #1, Mi-1, on the path State actually takes: a set
// of a typed nil list, committed, snapshotted and recovered. The hash before and
// after the snapshot has to be one, and repeating the same set has to be the
// same no-op on the original and on the recovered entity — otherwise the
// recovered State moves a version the original did not (NFR-061).
//
// The nils jsonpath.Clone rebuilds as empty containers are here too: Commit
// stores them empty, so the repeat compares an empty container with the nil of
// the proposal, and the two have to read as one.
func TestASetOfANilListHashesTheSameAfterASnapshot(t *testing.T) {
	nils := map[string]any{
		"[]string":         []string(nil),
		"map[string]int":   map[string]int(nil),
		"[]any":            []any(nil),
		"map[string]any":   map[string]any(nil),
		"[]map[string]any": []map[string]any(nil),
	}
	for name, value := range nils {
		t.Run(name, func(t *testing.T) {
			original := player(t)
			set := entity.Op{Op: entity.OpSet, Path: entity.AttrPlayersPresent, Value: value}
			attrs, changed := applyOK(t, original, set)
			original.Commit(attrs, changed, entity.LastChange{ProposalID: "p-1", AppliedAt: proposedAt})

			recovered := roundtrip(t, original)
			if a, b := entity.StateHash([]*entity.Entity{original}), entity.StateHash([]*entity.Entity{recovered}); a != b {
				t.Fatalf("StateHash %s before the snapshot, %s after it", a, b)
			}

			_, again := applyOK(t, original, set)
			_, againRecovered := applyOK(t, recovered, set)
			if len(again) != 0 || len(againRecovered) != 0 {
				t.Fatalf("repeating the set changed %+v on the original and %+v on the recovered entity, want nothing on both",
					again, againRecovered)
			}
		})
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
