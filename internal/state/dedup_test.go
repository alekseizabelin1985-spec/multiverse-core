package state_test

import (
	"fmt"
	"testing"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
)

// Step 2 of §4.5: the window of proposal identifiers and, past it, the commit
// record of the entities (last_change.proposal_id).

// TestARepeatPastTheWindowIsRecognisedByTheCommitRecord: with a window of one
// identifier, a later proposal pushes the first out, and the first comes again
// under a new event — an update, a create and a package applied in part alike.
// None of them publishes a second answer: the entities still carry the
// proposal in last_change.
func TestARepeatPastTheWindowIsRecognisedByTheCommitRecord(t *testing.T) {
	for name, first := range map[string]func(t *testing.T) eventbus.Event{
		"an update": func(t *testing.T) eventbus.Event {
			return update(t, "prop-first", "author", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
		},
		"a create": func(*testing.T) eventbus.Event {
			return create("prop-first", ref("wolf-beta", entity.TypeNPC), "", map[string]any{"hp": 5})
		},
		"a package applied in part": func(t *testing.T) eventbus.Event {
			return update(t, "prop-first", "author", false,
				set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)),
				set(ref("ghost", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := fixtureWith(t, state.ApplierConfig{DedupCapacity: 1, WithoutOwnership: true})
			f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
			f.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{"hp": 10})

			f.apply(t, first(t))
			answered := len(f.journal.all())
			f.apply(t, update(t, "prop-second", "author", true, set(ref("wolf-alpha", entity.TypeNPC), nil, op(entity.OpInc, "hp", -1))))
			if window := f.applier.AppliedProposals(); len(window) != 1 || window[0] != "prop-second" {
				t.Fatalf("window %v, want only prop-second: the first has to be out of it", window)
			}
			before := len(f.journal.all())
			if before != answered+1 {
				t.Fatalf("%d events, want %d", before, answered+1)
			}

			f.apply(t, first(t))

			if all := f.journal.all(); len(all) != before {
				t.Errorf("the repeat past the window published %v", types(all[before:]))
			}
			if e := f.get(t, "player-A"); name != "a create" && e.Version != 2 {
				t.Errorf("player-A v%d, want v2: the repeat was applied again", e.Version)
			}
			if dups := f.logged(t, "proposal already applied"); len(dups) != 1 {
				t.Errorf("%d debug records of a repeat, want one", len(dups))
			}
		})
	}
}

// TestDuplicatedDeliveryMovesTheWorldOnce is NFR-013 over the bus of the
// process: with --chaos=duplicate every proposal is on system_events twice, and
// every one of them is also proposed again under a new event. Each entity moves
// by exactly one version per proposal and one fact announces it.
func TestDuplicatedDeliveryMovesTheWorldOnce(t *testing.T) {
	bus := newBus(t)
	c, _ := running(t, bus, world)
	bus.SetChaos(membus.Chaos{Duplicate: true})
	const n = 20

	publish(t, bus, create("prop-born", ref("counter", entity.TypeNPC), "", map[string]any{"turns": 0}))
	for i := range n {
		id := fmt.Sprintf("prop-%02d", i)
		publish(t, bus,
			update(t, id, "author", true, set(ref("counter", entity.TypeNPC), nil, op(entity.OpInc, "turns", 1))),
			update(t, id, "author", true, set(ref("counter", entity.TypeNPC), nil, op(entity.OpInc, "turns", 1))))
	}
	sentinel := update(t, "prop-sentinel", "author", true, set(ref("counter", entity.TypeNPC), nil, op(entity.OpInc, "turns", 0)))
	publish(t, bus, sentinel)
	waitFor(t, "the answer to the sentinel", func() bool { return len(answersTo(t, bus, "prop-sentinel")) > 0 })

	if e, _ := c.Get(world, "counter"); e.Version != n+1 || intAt(e, "turns") != n {
		t.Errorf("counter v%d turns %v, want v%d turns %d", e.Version, e.Attributes["turns"], n+1, n)
	}
	versions := map[int]string{}
	for _, fact := range onTopic(t, bus, state.TypeUpdated) {
		if id, _ := fact.Path().GetString("proposal_id"); id == "prop-sentinel" {
			continue // a turn without a change, at the version it found
		}
		version, _ := fact.Path().GetInt("version")
		if first, seen := versions[version]; seen && first != fact.ID {
			t.Errorf("v%d announced by %s and %s", version, first, fact.ID)
		}
		versions[version] = fact.ID
	}
	if len(versions) != n {
		t.Errorf("%d versions announced by entity.updated, want %d", len(versions), n)
	}
	if refusals := onTopic(t, bus, state.TypeRejected); len(refusals) != 0 {
		t.Errorf("%d refusals, want none", len(refusals))
	}
}

// TestTheWindowRecognisesARepeatTheCommitRecordNoLongerCarries: P changes an
// entity, Q changes it after P, and P comes again under a new event. The commit
// record of the entity names Q now; only the window of proposal identifiers
// knows P was applied. Nothing is answered and P is not applied twice — for an
// update, and for a create, where the world alone would answer duplicate_entity
// (§4.5, the paragraph on create; review #1 of T-056, Mi-2).
func TestTheWindowRecognisesARepeatTheCommitRecordNoLongerCarries(t *testing.T) {
	t.Run("an update", func(t *testing.T) {
		f := newFixture(t)
		f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
		hit := func(id string) eventbus.Event {
			return update(t, id, "author", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
		}
		f.apply(t, hit("prop-P"))
		f.apply(t, hit("prop-Q"))
		f.apply(t, hit("prop-P"))

		if all := f.journal.all(); len(all) != 2 {
			t.Fatalf("published %v, want the two facts of P and Q", types(all))
		}
		if e := f.get(t, "player-A"); e.Version != 3 || intAt(e, "hp") != 8 {
			t.Errorf("player-A v%d hp %v, want v3 hp 8: P was applied twice", e.Version, e.Attributes["hp"])
		}
	})
	t.Run("a create", func(t *testing.T) {
		f := newFixture(t)
		born := func() eventbus.Event {
			return create("prop-P", ref("wolf-beta", entity.TypeNPC), "", map[string]any{"hp": 5})
		}
		f.apply(t, born())
		f.apply(t, update(t, "prop-Q", "author", true, set(ref("wolf-beta", entity.TypeNPC), nil, op(entity.OpInc, "hp", -1))))
		f.apply(t, born())

		if all := f.journal.all(); len(all) != 2 || all[0].Type != state.TypeCreated || all[1].Type != state.TypeUpdated {
			t.Fatalf("published %v, want entity.created and entity.updated and nothing for the repeat", types(all))
		}
	})
}
