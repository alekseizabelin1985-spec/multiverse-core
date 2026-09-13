package state_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/jsonpath"
	"multiverse-core.io/shared/testkit"
)

// --- the cycle ---

// A create becomes entity.created: version 1, the attributes, the proposal_id
// the schema requires since C-02 v1.6, the trace of the proposal and its
// timestamp — and the entity in the world carries the same.
func TestACreateBecomesEntityCreated(t *testing.T) {
	f := newFixture(t)
	ev := create("prop-create-A", ref("player-A", entity.TypePlayer), "Вася",
		map[string]any{"hp": 10, "status": "alive"})
	ev.Timestamp = testkit.Epoch.Add(42)

	f.apply(t, ev)

	facts := f.journal.all()
	if len(facts) != 1 || facts[0].Type != state.TypeCreated {
		t.Fatalf("published %v, want one %s", types(facts), state.TypeCreated)
	}
	fact := facts[0]
	assertDerivedFrom(t, fact, ev)
	p := fact.Path()
	if v, _ := p.GetInt("version"); v != 1 {
		t.Errorf("version %d, want 1", v)
	}
	if id, _ := p.GetString("proposal_id"); id != "prop-create-A" {
		t.Errorf("proposal_id %q, want the proposal's", id)
	}
	if hp, _ := p.GetInt("attributes.hp"); hp != 10 {
		t.Errorf("attributes.hp %d, want 10", hp)
	}
	if name, _ := p.GetString("entity.name"); name != "Вася" {
		t.Errorf("entity.name %q, want the name proposed", name)
	}

	e := f.get(t, "player-A")
	if e.Version != 1 || !e.CreatedAt.Equal(ev.Timestamp) || !e.UpdatedAt.Equal(ev.Timestamp) {
		t.Errorf("entity v%d created %v updated %v, want v1 at the proposal time %v",
			e.Version, e.CreatedAt, e.UpdatedAt, ev.Timestamp)
	}
	if e.LastEventID != fact.ID || e.LastChange == nil || e.LastChange.FactEventID != fact.ID ||
		e.LastChange.ProposalID != "prop-create-A" || e.LastChange.ProposalEventID != ev.ID {
		t.Errorf("commit record %+v last event %q, want the fact %s of prop-create-A", e.LastChange, e.LastEventID, fact.ID)
	}
}

// An update becomes one entity.updated per entity: the version moves by one,
// changed[] is what ApplyOps found, cause and proposal_id are the proposal's,
// applied_at and timestamp are the time of the proposal.
func TestAnUpdateBecomesEntityUpdated(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 4, map[string]any{"hp": 10, "position": "outside", "inventory": []any{}})
	ev := update(t, "prop-move", "move", true, set(ref("player-A", entity.TypePlayer), version(4),
		op(entity.OpSet, "position", "dark-forest-01"),
		op(entity.OpInc, "hp", -3),
		op(entity.OpAppend, "inventory", map[string]any{"item_id": "fang"})))
	ev.Timestamp = testkit.Epoch.Add(7)

	f.apply(t, ev)

	facts := f.journal.ofType(state.TypeUpdated)
	if len(facts) != 1 {
		t.Fatalf("published %v, want one %s", types(f.journal.all()), state.TypeUpdated)
	}
	fact := facts[0]
	assertDerivedFrom(t, fact, ev)
	p := fact.Path()
	if v, _ := p.GetInt("version"); v != 5 {
		t.Errorf("version %d, want 5", v)
	}
	if cause, _ := p.GetString("cause"); cause != "move" {
		t.Errorf("cause %q, want move", cause)
	}
	if applied, _ := p.GetString("applied_at"); !sameInstant(applied, ev.Timestamp) {
		t.Errorf("applied_at %q, want the proposal time %v", applied, ev.Timestamp)
	}
	var changed []string
	for i := range 3 {
		path, _ := p.GetString("changed[" + itoa(i) + "].path")
		changed = append(changed, path)
	}
	if strings.Join(changed, ",") != "position,hp,inventory[0]" {
		t.Errorf("changed paths %v, want position,hp,inventory[0]", changed)
	}

	e := f.get(t, "player-A")
	if e.Version != 5 || e.Attributes["position"] != "dark-forest-01" || e.LastEventID != fact.ID {
		t.Errorf("entity v%d at %v last %q, want v5 in dark-forest-01 announced by %s",
			e.Version, e.Attributes["position"], e.LastEventID, fact.ID)
	}
	if !e.UpdatedAt.Equal(ev.Timestamp) || e.LastChange.AppliedAt != ev.Timestamp || e.LastChange.FactEventID != fact.ID {
		t.Errorf("updated %v, commit record %+v; want the proposal time and the fact", e.UpdatedAt, e.LastChange)
	}
}

// A package that changes nothing still counts: one fact with an empty
// changed[] and the version where it was (C-02).
func TestAnUpdateWithoutAChangeKeepsTheVersion(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 3, map[string]any{"hp": 10})

	f.apply(t, update(t, "prop-same", "rest", true,
		set(ref("player-A", entity.TypePlayer), version(3), op(entity.OpSet, "hp", 10))))

	facts := f.journal.ofType(state.TypeUpdated)
	if len(facts) != 1 {
		t.Fatalf("%d facts, want one", len(facts))
	}
	if v, _ := facts[0].Path().GetInt("version"); v != 3 {
		t.Errorf("version %d, want 3", v)
	}
	if changed, ok := facts[0].Payload["changed"].([]any); !ok || len(changed) != 0 {
		t.Errorf("changed %#v, want an empty list", facts[0].Payload["changed"])
	}
	if e := f.get(t, "player-A"); e.Version != 3 {
		t.Errorf("entity at v%d, want v3", e.Version)
	}
}

// atomic=true: the first problem refuses the whole package, nothing is
// applied, and the refusal names the entity and the versions of the conflict.
func TestAnAtomicPackageIsAllOrNothing(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 2, map[string]any{"hp": 10})
	f.seed(t, "wolf-alpha", entity.TypeNPC, 6, map[string]any{"hp": 8})

	f.apply(t, update(t, "prop-round", "combat", true,
		set(ref("player-A", entity.TypePlayer), version(2), op(entity.OpInc, "hp", -2)),
		set(ref("wolf-alpha", entity.TypeNPC), version(5), op(entity.OpInc, "hp", -3))))

	all := f.journal.all()
	if len(all) != 1 {
		t.Fatalf("published %v, want one refusal", types(all))
	}
	assertRejected(t, all[0], "prop-round", state.ReasonVersionConflict, "wolf-alpha")
	if exp, _ := all[0].Path().GetInt("details.expected_version"); exp != 5 {
		t.Errorf("details.expected_version %d, want 5", exp)
	}
	if act, _ := all[0].Path().GetInt("details.actual_version"); act != 6 {
		t.Errorf("details.actual_version %d, want 6", act)
	}
	if a := f.get(t, "player-A"); a.Version != 2 || intAt(a, "hp") != 10 {
		t.Errorf("player-A v%d hp %v: the refused package changed it", a.Version, a.Attributes["hp"])
	}

	// The package comes again under the same identifier and is decided again,
	// against the world as it is now: a refusal is not remembered.
	f.apply(t, update(t, "prop-round", "combat", true,
		set(ref("player-A", entity.TypePlayer), version(2), op(entity.OpInc, "hp", -2)),
		set(ref("unknown-npc", entity.TypeNPC), nil, op(entity.OpInc, "hp", -3))))
	last := f.journal.all()[len(f.journal.all())-1]
	assertRejected(t, last, "prop-round", state.ReasonUnknownEntity, "unknown-npc")
}

// atomic=false: the change sets that fail are refused one by one, the others
// are applied, and the facts come out in identifier order.
func TestANonAtomicPackageRefusesOnlyWhatFailed(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "b-player", entity.TypePlayer, 1, map[string]any{"hp": 10})
	f.seed(t, "a-player", entity.TypePlayer, 1, map[string]any{"hp": 10})
	f.seed(t, "c-player", entity.TypePlayer, 1, map[string]any{"hp": "ten"})

	f.apply(t, update(t, "prop-tick", "tick", false,
		set(ref("b-player", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)),
		set(ref("ghost", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)),
		set(ref("c-player", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)),
		set(ref("a-player", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))

	all := f.journal.all()
	if got := types(all); strings.Join(got, ",") != "entity.updated,entity.updated,entity.update.rejected,entity.update.rejected" {
		t.Fatalf("published %v, want two facts and then two refusals", got)
	}
	for i, want := range []string{"a-player", "b-player"} {
		if id, _ := all[i].Path().GetString("entity.entity.id"); id != want {
			t.Errorf("fact %d is about %q, want %q (identifier order)", i, id, want)
		}
	}
	assertRejected(t, all[2], "prop-tick", state.ReasonUnknownEntity, "ghost")
	assertRejected(t, all[3], "prop-tick", state.ReasonInvalidOp, "c-player")
	if f.get(t, "a-player").Version != 2 || f.get(t, "b-player").Version != 2 || f.get(t, "c-player").Version != 1 {
		t.Error("the versions of the package do not match what was applied")
	}
	// §4.5 p. 9: "применённый размер остаётся в last_change.batch_size".
	for _, id := range []string{"a-player", "b-player"} {
		if batch := f.get(t, id).LastChange.BatchSize; batch != 2 {
			t.Errorf("%s last_change.batch_size = %d, want 2 applied of the 4 proposed", id, batch)
		}
	}
}

// A create over an identifier already in the world is duplicate_entity.
func TestACreateOfAnExistingEntityIsDuplicate(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 3, map[string]any{"hp": 10})

	f.apply(t, create("prop-again", ref("player-A", entity.TypePlayer), "Вася", map[string]any{"hp": 1}))

	all := f.journal.all()
	if len(all) != 1 {
		t.Fatalf("published %v, want one refusal", types(all))
	}
	assertRejected(t, all[0], "prop-again", state.ReasonDuplicateEntity, "player-A")
	if e := f.get(t, "player-A"); e.Version != 3 {
		t.Errorf("the entity moved to v%d", e.Version)
	}
}

// --- what the proposal may look like ---

// C-02 v1.3: a package that names one entity twice is refused whole, with the
// entity it repeats, whatever atomic says — otherwise the two sets would be
// published as two facts under one version.
func TestOneEntityNamedTwiceIsRefused(t *testing.T) {
	for _, atomic := range []bool{true, false} {
		t.Run("atomic="+map[bool]string{true: "true", false: "false"}[atomic], func(t *testing.T) {
			f := newFixture(t)
			f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
			f.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{"hp": 10})

			f.apply(t, update(t, "prop-twice", "combat", atomic,
				set(ref("wolf-alpha", entity.TypeNPC), nil, op(entity.OpInc, "hp", -1)),
				set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)),
				set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -2))))

			all := f.journal.all()
			if len(all) != 1 {
				t.Fatalf("published %v, want one refusal", types(all))
			}
			assertRejected(t, all[0], "prop-twice", state.ReasonInvalidOp, "player-A")
			if f.get(t, "player-A").Version != 1 || f.get(t, "wolf-alpha").Version != 1 {
				t.Error("a version moved under a refused package")
			}
		})
	}
}

// A payload that does not read — changes of the wrong shape, an operation the
// model does not know — is invalid_op under its proposal_id (§4.5 p. 1).
func TestAMalformedProposalIsInvalidOp(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	shapeless := update(t, "prop-shapeless", "move", true)
	shapeless.Payload["changes"] = "not a list"
	unknownOp := update(t, "prop-unknown-op", "move", true,
		set(ref("player-A", entity.TypePlayer), nil, op("teleport", "position", "x")))
	empty := update(t, "prop-empty", "move", true)

	for _, ev := range []eventbus.Event{shapeless, unknownOp, empty} {
		f.apply(t, ev)
	}

	all := f.journal.all()
	if len(all) != 3 {
		t.Fatalf("published %v, want three refusals", types(all))
	}
	assertRejected(t, all[0], "prop-shapeless", state.ReasonInvalidOp, "")
	assertRejected(t, all[1], "prop-unknown-op", state.ReasonInvalidOp, "player-A")
	assertRejected(t, all[2], "prop-empty", state.ReasonInvalidOp, "")
}

// --- C-02 v1.6 ---

// The form of changed[] is decided by the presence of old and new, and the one
// encoder is entity.Change.MarshalJSON: a set of an absent path to null carries
// only new, a set of a present path to null both, a remove of a key holding
// null only old. Every fact passes the schema.
func TestChangedCarriesOldAndNewByPresence(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"mood": "calm", "nothing": nil})

	f.apply(t, update(t, "prop-nulls", "move", true, set(ref("player-A", entity.TypePlayer), nil,
		op(entity.OpSet, "fresh", nil),
		op(entity.OpSet, "mood", nil),
		op(entity.OpRemove, "nothing", nil))))

	if len(f.journal.raw) != 1 {
		t.Fatalf("published %d events, want one fact", len(f.journal.raw))
	}
	var wireFact struct {
		Payload struct {
			Changed []map[string]json.RawMessage `json:"changed"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(f.journal.raw[0], &wireFact); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]string{"fresh": "new", "mood": "old,new", "nothing": "old"}
	if len(wireFact.Payload.Changed) != len(want) {
		t.Fatalf("changed %s, want three entries", f.journal.raw[0])
	}
	for _, entry := range wireFact.Payload.Changed {
		var path string
		_ = json.Unmarshal(entry["path"], &path)
		var keys []string
		for _, key := range []string{"old", "new"} {
			if raw, ok := entry[key]; ok {
				keys = append(keys, key)
				if string(raw) != "null" && path != "mood" {
					t.Errorf("%s.%s = %s, want null", path, key, raw)
				}
			}
		}
		if strings.Join(keys, ",") != want[path] {
			t.Errorf("%s carries %v, want %s", path, keys, want[path])
		}
	}
	if err := contracts.Validate(f.journal.all()[0]); err != nil {
		t.Errorf("the fact is not a valid %s: %v", state.TypeUpdated, err)
	}
}

// A proposal without proposal_id is neither applied nor refused: there is no
// refusal without the identifier. The log names the event and its type.
func TestAProposalWithoutProposalIDIsPassedOver(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	noIDCreate := create("", ref("player-B", entity.TypePlayer), "Петя", map[string]any{"hp": 10})
	delete(noIDCreate.Payload, "proposal_id")
	noIDUpdate := update(t, "", "move", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpSet, "position", "x")))
	delete(noIDUpdate.Payload, "proposal_id")
	shapelessNoID := update(t, "", "move", true)
	delete(shapelessNoID.Payload, "proposal_id")
	shapelessNoID.Payload["changes"] = 7

	for _, ev := range []eventbus.Event{noIDCreate, noIDUpdate, shapelessNoID} {
		f.apply(t, ev)
	}

	if all := f.journal.all(); len(all) != 0 {
		t.Fatalf("published %v for proposals without an identifier, want nothing", types(all))
	}
	if _, ok := f.store.Get(world, "player-B"); ok || f.get(t, "player-A").Version != 1 {
		t.Error("a proposal without an identifier changed the world")
	}
	warned := f.logged(t, "proposal without proposal_id passed over: nothing to refuse it by")
	if len(warned) != 3 {
		t.Fatalf("%d warnings, want one per proposal: %s", len(warned), f.log)
	}
	for i, ev := range []eventbus.Event{noIDCreate, noIDUpdate, shapelessNoID} {
		if warned[i]["level"] != "WARN" || warned[i]["event_id"] != ev.ID || warned[i]["type"] != ev.Type {
			t.Errorf("warning %d is %v, want WARN with event_id %s and type %s", i, warned[i], ev.ID, ev.Type)
		}
	}
}

// The attributes of a create are checked before the world and before the
// window of applied proposals: a number past ±(2^53-1) is invalid_op even over
// an entity already there, where the world would answer duplicate_entity, and
// even under an identifier already applied.
func TestTheAttributesOfACreateAreCheckedFirst(t *testing.T) {
	f := newFixture(t)
	f.apply(t, create("prop-born", ref("player-A", entity.TypePlayer), "Вася", map[string]any{"hp": 10}))

	for _, proposalID := range []string{"prop-seed", "prop-born"} {
		ev := create(proposalID, ref("player-A", entity.TypePlayer), "Вася",
			map[string]any{"seed": float64(-(1 << 53))})
		f.apply(t, ev)
		all := f.journal.all()
		assertRejected(t, all[len(all)-1], proposalID, state.ReasonInvalidOp, "player-A")
	}
	if _, has := f.get(t, "player-A").Attributes["seed"]; has {
		t.Error("the refused attributes reached the entity")
	}
}

// --- deduplication of proposals ---

// A proposal applied once is not applied again under its identifier, whatever
// event brings it; a refused one is not remembered.
func TestARepeatedProposalIsAppliedOnce(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	first := update(t, "prop-hit", "combat", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
	again := update(t, "prop-hit", "combat", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))

	f.apply(t, first)
	f.apply(t, first)
	f.apply(t, again)

	if facts := f.journal.all(); len(facts) != 1 {
		t.Fatalf("published %v, want one fact", types(facts))
	}
	if e := f.get(t, "player-A"); e.Version != 2 || intAt(e, "hp") != 9 {
		t.Errorf("player-A v%d hp %v, want v2 hp 9", e.Version, e.Attributes["hp"])
	}
	if dups := f.logged(t, "proposal already applied"); len(dups) != 2 {
		t.Errorf("%d debug records of a repeat, want 2", len(dups))
	}
}

// --- answers derive their ids from the proposal (ADR-027) ---

// An event of the answer that does not go out is published again, as it is —
// the same id, the same bytes — after a pause on the clock of the context,
// until it does. Nothing of the proposal is kept while it has not: the world
// does not move and the events already out are not published a second time.
// Different entities of one package get different ids (review #1 of T-055,
// Ma-1).
func TestAFailedPublicationIsPublishedAgainAsItIs(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	f.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{"hp": 10})
	var attempts [][]byte
	var keptEarly []string
	f.journal.fail = func(n int, ev eventbus.Event) error {
		for _, id := range []string{"player-A", "wolf-alpha"} {
			if e, _ := f.store.Get(world, id); e.Version != 1 {
				keptEarly = append(keptEarly, fmt.Sprintf("%s v%d at call %d", id, e.Version, n))
			}
		}
		if id, _ := ev.Path().GetString("entity.entity.id"); id != "wolf-alpha" {
			return nil
		}
		raw, err := json.Marshal(ev)
		if err != nil {
			t.Error(err)
		}
		attempts = append(attempts, raw)
		if len(attempts) <= 3 {
			return errors.New("the acknowledgement was lost")
		}
		return nil
	}
	ev := update(t, "prop-round", "combat", true,
		set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)),
		set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -3)))

	if err := f.applyAdvancing(t, context.Background(), ev); err != nil {
		t.Fatalf("Apply = %v, want the answer out after the attempts", err)
	}

	if len(keptEarly) != 0 {
		t.Errorf("the world moved before the answer was out: %v", keptEarly)
	}
	if len(attempts) != 4 {
		t.Fatalf("the fact of wolf-alpha was published in %d attempts, want 4", len(attempts))
	}
	for i, raw := range attempts[1:] {
		if !bytes.Equal(raw, attempts[0]) {
			t.Errorf("attempt %d published %s, want the bytes of the first %s", i+2, raw, attempts[0])
		}
	}
	facts := f.journal.ofType(state.TypeUpdated)
	ids := idsByEntity(facts)
	if len(facts) != 2 || len(ids["player-A"]) != 1 || len(ids["wolf-alpha"]) != 1 {
		t.Fatalf("facts by entity %v, want each entity once: what is out is not published again", ids)
	}
	if ids["player-A"][0] == ids["wolf-alpha"][0] {
		t.Error("two entities of one package got one fact id")
	}
	if f.get(t, "player-A").Version != 2 || f.get(t, "wolf-alpha").Version != 2 {
		t.Error("the answer is out and the versions did not move by exactly one")
	}
	warned := f.logged(t, "an event of the answer did not go out; it is published again as it is")
	if len(warned) != 3 || warned[0]["level"] != "WARN" || warned[0]["proposal_id"] != "prop-round" ||
		warned[0]["event_id"] != ids["wolf-alpha"][0] {
		t.Errorf("log of the attempts %v, want three WARN records naming the proposal and the event", warned)
	}
}

// The attempts end with the context of the world: the world stops with
// publish_failed, nothing of the proposal is kept — for a create as for an
// update — and the world decides nothing more, so no later proposal can
// announce another fact under a version already announced (Ma-1).
func TestAnAnswerAbandonedWithTheContextStopsTheWorld(t *testing.T) {
	for name, tc := range map[string]struct {
		proposal func(t *testing.T) eventbus.Event
		failing  string
	}{
		"update": {
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "prop-round", "combat", true,
					set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)),
					set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -3)))
			},
			failing: state.TypeUpdated,
		},
		"create": {
			proposal: func(*testing.T) eventbus.Event {
				return create("prop-round", ref("wolf-beta", entity.TypeNPC), "", map[string]any{"hp": 5})
			},
			failing: state.TypeCreated,
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
			f.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{"hp": 10})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failed := make(chan struct{})
			var once sync.Once
			f.journal.fail = func(_ int, ev eventbus.Event) error {
				id, _ := ev.Path().GetString("entity.entity.id")
				if ev.Type != tc.failing || id == "player-A" {
					return nil
				}
				once.Do(func() { close(failed) })
				return errors.New("the broker is away")
			}
			proposal := tc.proposal(t)
			done := make(chan error, 1)
			go func() { done <- f.applier.Apply(ctx, proposal) }()
			<-failed
			cancel()
			var err error
			select {
			case err = <-done:
			case <-testkit.After(30 * time.Second):
				t.Fatal("Apply did not return within 30s of the end of its context")
			}

			if !errors.Is(err, state.ErrWorldStopped) || !errors.Is(err, state.ErrPublishFailed) {
				t.Fatalf("Apply = %v, want the world stopped with publish_failed", err)
			}
			if _, ok := f.store.Get(world, "wolf-beta"); ok ||
				f.get(t, "player-A").Version != 1 || f.get(t, "wolf-alpha").Version != 1 {
				t.Error("the world kept an answer that did not get out")
			}
			if !errors.Is(f.applier.Stopped(), state.ErrPublishFailed) {
				t.Errorf("Stopped = %v, want publish_failed", f.applier.Stopped())
			}
			abandoned := f.logged(t, "the answer to a proposal was abandoned before it got out; the world is stopped")
			if len(abandoned) != 1 || abandoned[0]["level"] != "ERROR" || abandoned[0]["handled"] != false ||
				abandoned[0]["reason"] != "publish_failed" || abandoned[0]["proposal_id"] != "prop-round" {
				t.Errorf("log %v, want one ERROR with reason publish_failed, the proposal and handled=false", abandoned)
			}

			out := len(f.journal.all())
			f.journal.fail = nil
			next := update(t, "prop-next", "combat", true,
				set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
			if err := f.applier.Apply(context.Background(), next); !errors.Is(err, state.ErrWorldStopped) {
				t.Errorf("Apply after the world stopped = %v, want ErrWorldStopped", err)
			}
			if again := f.applier.Apply(context.Background(), proposal); !errors.Is(again, state.ErrWorldStopped) {
				t.Errorf("Apply of the abandoned proposal again = %v, want ErrWorldStopped", again)
			}
			if all := f.journal.all(); len(all) != out || f.get(t, "player-A").Version != 1 {
				t.Errorf("the stopped world published %v", types(all[out:]))
			}
		})
	}
}

// Mi-3 of review #1: a create whose fact does not go out at once keeps
// nothing until it does, and is answered by exactly one entity.created with
// the id of every attempt — never by a duplicate_entity of its own making.
func TestACreateKeepsNothingUntilItsFactIsOut(t *testing.T) {
	f := newFixture(t)
	var ids []string
	var keptEarly bool
	f.journal.fail = func(_ int, ev eventbus.Event) error {
		if _, ok := f.store.Get(world, "player-A"); ok {
			keptEarly = true
		}
		ids = append(ids, ev.ID)
		if len(ids) == 1 {
			return errors.New("the acknowledgement was lost")
		}
		return nil
	}

	if err := f.applyAdvancing(t, context.Background(),
		create("prop-born", ref("player-A", entity.TypePlayer), "Вася", map[string]any{"hp": 10})); err != nil {
		t.Fatalf("Apply = %v", err)
	}

	if keptEarly {
		t.Error("the entity was in the world before its entity.created was out")
	}
	all := f.journal.all()
	if len(all) != 1 || all[0].Type != state.TypeCreated || len(ids) != 2 || ids[0] != ids[1] || all[0].ID != ids[0] {
		t.Fatalf("published %v in attempts %v, want one %s under the id of both attempts", types(all), ids, state.TypeCreated)
	}
	if e := f.get(t, "player-A"); e.Version != 1 || e.LastEventID != ids[0] {
		t.Errorf("player-A v%d announced by %q, want v1 by %s", e.Version, e.LastEventID, ids[0])
	}
}

// Mi-3 of review #1, §4.5 on create: "с тем же proposal_id — дедуп". A new
// event under the proposal_id of an applied create publishes nothing, where the
// world alone would answer duplicate_entity.
func TestACreateUnderAnAppliedProposalIDIsSilent(t *testing.T) {
	f := newFixture(t)
	f.apply(t, create("prop-born", ref("player-A", entity.TypePlayer), "Вася", map[string]any{"hp": 10}))

	again := create("prop-born", ref("player-A", entity.TypePlayer), "Вася", map[string]any{"hp": 10})
	f.apply(t, again)

	if all := f.journal.all(); len(all) != 1 {
		t.Fatalf("published %v, want the one entity.created: the repeat is silent", types(all))
	}
	if dups := f.logged(t, "proposal already applied"); len(dups) != 1 || dups[0]["event_id"] != again.ID {
		t.Errorf("debug records of a repeat %v, want one for %s", dups, again.ID)
	}
}

// Mi-4 of review #1: the refusals of one package are told apart by their ids —
// a consumer that drops repeats by id would otherwise drop the second — and the
// same event, decided again, is refused under the same ids.
func TestTheRefusalsOfOnePackageHaveIDsOfTheirOwn(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "c-player", entity.TypePlayer, 1, map[string]any{"hp": "ten"})
	ev := update(t, "prop-tick", "tick", false,
		set(ref("ghost", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)),
		set(ref("c-player", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))

	f.apply(t, ev)
	f.apply(t, ev)

	all := f.journal.all()
	if len(all) != 4 {
		t.Fatalf("published %v, want two refusals each time: a refusal is not remembered", types(all))
	}
	if all[0].ID == all[1].ID {
		t.Errorf("the two refusals of one package share the id %s", all[0].ID)
	}
	if all[2].ID != all[0].ID || all[3].ID != all[1].ID {
		t.Errorf("decided again the refusals are %s, %s; want %s, %s", all[2].ID, all[3].ID, all[0].ID, all[1].ID)
	}
}

// The deterministic path of Ma-1: a change set that names an entity by id but
// not by type is refused without the entity in the payload, which the schema
// requires whole. Named with an empty type, the refusal could never be
// published, and the fact before it would be out without the world.
func TestARefusalOfAnEntityWithoutATypeCanBePublished(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})

	if err := f.applyAdvancing(t, context.Background(), update(t, "prop-typeless", "tick", false,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)),
		set(ref("ghost", ""), nil, op(entity.OpInc, "hp", -1)))); err != nil {
		t.Fatalf("Apply = %v", err)
	}

	all := f.journal.all()
	if len(all) != 2 || f.journal.calls != 2 {
		t.Fatalf("published %v in %d calls, want a fact and a refusal at the first attempt", types(all), f.journal.calls)
	}
	assertRejected(t, all[1], "prop-typeless", state.ReasonUnknownEntity, "")
	if f.get(t, "player-A").Version != 2 {
		t.Error("the fact is out and the world did not keep it")
	}
}

// --- what the Applier leaves alone ---

func TestWhatIsNotAProposalOfThisWorldIsPassedOver(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	other := update(t, "prop-other", "move", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
	other.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: "another-world", Type: "world"}}
	worldless := update(t, "prop-none", "move", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
	worldless.World = nil
	// The world of an event is its envelope: the legacy key of the payload
	// does not address a proposal (CLAUDE.md, "Конверт события").
	worldless.Payload["world_id"] = world
	fact := eventbus.NewRoot(state.TypeUpdated, contracts.SourceState, world, nil, eventbus.ActorSystem, map[string]any{})

	for _, ev := range []eventbus.Event{other, worldless, fact} {
		f.apply(t, ev)
	}
	if all := f.journal.all(); len(all) != 0 || f.get(t, "player-A").Version != 1 {
		t.Fatalf("published %v and moved the world for events not addressed to it", types(all))
	}
	// Review #1 of T-055, Mi-1: a proposal without a world is reported, one of
	// another world is not news.
	warned := f.logged(t, "proposal without world in the envelope: no worker is addressed")
	if len(warned) != 1 || warned[0]["level"] != "WARN" || warned[0]["event_id"] != worldless.ID ||
		warned[0]["type"] != state.TypeUpdateProposed || warned[0]["proposal_id"] != "prop-none" {
		t.Errorf("log of the worldless proposal %v, want one WARN with event_id, type and proposal_id", warned)
	}
	if passed := f.logged(t, "proposal of another world passed over"); len(passed) != 1 || passed[0]["level"] != "DEBUG" {
		t.Errorf("log of the proposal of another world %v, want one DEBUG record", passed)
	}
}

// An envelope without an id — past a bus that does not validate on read — is
// not answered: every answer derives its id from the proposal event, and the
// derivation panics without one (C-01 v1.5).
func TestAProposalEventWithoutAnIDIsNotAnswered(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	ev := update(t, "prop-noid", "move", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
	ev.ID = ""

	f.apply(t, ev)

	if all := f.journal.all(); len(all) != 0 {
		t.Fatalf("published %v", types(all))
	}
	if logged := f.logged(t, "proposal without an event id: not answered"); len(logged) != 1 || logged[0]["level"] != "ERROR" {
		t.Errorf("log %v, want one ERROR record", logged)
	}
}

func TestNewApplierRefusesAnIncompleteConfig(t *testing.T) {
	store, pub := memstore.New(), &journal{}
	for name, cfg := range map[string]state.ApplierConfig{
		"no world":     {Store: store, Publisher: pub},
		"no store":     {WorldID: world, Publisher: pub},
		"no publisher": {WorldID: world, Store: store},
	} {
		if _, err := state.NewApplier(cfg); err == nil {
			t.Errorf("%s: NewApplier accepted it", name)
		}
	}
}

// --- helpers ---

func assertDerivedFrom(t *testing.T, fact, cause eventbus.Event) {
	t.Helper()
	if err := contracts.Validate(fact); err != nil {
		t.Errorf("the fact is not a valid %s: %v", fact.Type, err)
	}
	if fact.Source != contracts.SourceState {
		t.Errorf("source %q, want %q", fact.Source, contracts.SourceState)
	}
	if fact.Meta.CausationID != cause.ID || fact.Meta.CorrelationID != cause.CorrelationID() {
		t.Errorf("causation %q correlation %q, want the proposal %q and its chain %q",
			fact.Meta.CausationID, fact.Meta.CorrelationID, cause.ID, cause.CorrelationID())
	}
	if !fact.Timestamp.Equal(cause.Timestamp) {
		t.Errorf("timestamp %v, want the proposal's %v", fact.Timestamp, cause.Timestamp)
	}
}

func types(events []eventbus.Event) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.Type)
	}
	return out
}

func idsByEntity(facts []eventbus.Event) map[string][]string {
	out := map[string][]string{}
	for _, fact := range facts {
		id, _ := fact.Path().GetString("entity.entity.id")
		out[id] = append(out[id], fact.ID)
	}
	return out
}

func itoa(i int) string { return strconv.Itoa(i) }

// intAt reads a whole number of the attributes, whichever Go type holds it.
func intAt(e *entity.Entity, path string) int {
	n, _ := jsonpath.New(e.Attributes).GetInt(path)
	return n
}

func sameInstant(encoded string, want time.Time) bool {
	at, err := time.Parse(time.RFC3339Nano, encoded)
	return err == nil && at.Equal(want)
}
