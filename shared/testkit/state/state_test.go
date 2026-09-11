package state_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/state"
)

const (
	worldID  = "dark-forest-world"
	regionID = "dark-forest-01"
	npcID    = "wolf-alpha"
	playerA  = "player-A"
)

func fixturesDir() string { return filepath.Join("..", "..", "..", "testdata", "fixtures") }

// world is the stub with the six entities of the dark forest in it, on an
// in-process bus that validates everything against the registry — the same
// checks the broker applies (C-01).
//
// It does not Start: a test that wants the subscription says so, and a test
// that only wants a decision calls Apply.
func world(t *testing.T) (*state.FakeState, *membus.Bus, *objstore.Memory) {
	t.Helper()
	bus := newBus(t)
	store := objstore.NewMemoryWithClock(clock.NewManual(testkit.Epoch))
	return seeded(t, bus, store), bus, store
}

// newBus is the in-process bus of a test: every topic of the registry, no
// backoff between the delivery attempts of a failing handler.
func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	testkit.Deterministic(t, "ev")

	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(),
		Topics:   topics,
		Backoff:  []time.Duration{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

// seeded is the stub on the given bus and store, with the six entities of the
// dark forest in it. A test that has to watch what the stub asks of the bus or
// of the store passes its own decorator here.
func seeded(t *testing.T, bus eventbus.Bus, store objstore.Client) *state.FakeState {
	t.Helper()
	fake, err := state.New(state.Config{
		Bus: bus, Store: store, WorldID: worldID,
		RulesVersion: "0.1", Clock: clock.NewManual(testkit.Epoch),
	})
	if err != nil {
		t.Fatalf("new fake state: %v", err)
	}
	entities, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	if err := fake.Seed(entities); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return fake
}

// events decodes a topic. The bodies are read raw rather than through a
// subscription so that an assertion sees everything that was published, in
// order, without competing with the subscription of the stub for a cursor.
func events(t *testing.T, bus busReader, topic string) []eventbus.Event {
	t.Helper()
	records, err := bus.Records(topic)
	if err != nil {
		t.Fatalf("read %s: %v", topic, err)
	}
	out := make([]eventbus.Event, 0, len(records))
	for i, body := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode %s[%d]: %v", topic, i, err)
		}
		out = append(out, ev)
	}
	return out
}

func ofType(evs []eventbus.Event, typ string) []eventbus.Event {
	out := make([]eventbus.Event, 0, len(evs))
	for _, ev := range evs {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

// waitFor polls until the condition holds. The deadline is wall time from
// shared/clock: the stub runs its subscription in a goroutine and a manual
// clock cannot tell a test how long that goroutine has actually had.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.After(3 * time.Second)
	for {
		if cond() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-clock.RealTimers{}.After(time.Millisecond).C():
		}
	}
}

// proposal builds an entity.update.proposed the way the gateway builds one,
// for the world the stub owns.
func proposal(id, cause string, atomic bool, sets ...entity.ChangeSet) eventbus.Event {
	return proposalIn(worldID, id, cause, atomic, sets...)
}

// proposalIn is the same proposal addressed to a world of its own: what a
// second world on the same bus publishes.
func proposalIn(world, id, cause string, atomic bool, sets ...entity.ChangeSet) eventbus.Event {
	return eventbus.NewRoot(state.TypeUpdateProposed, contracts.SourceGateway, world, nil,
		entity.ActorKindCI, map[string]any{
			"proposal_id": id,
			"changes":     sets,
			"atomic":      atomic,
			"cause":       cause,
		})
}

func changeSet(ref entity.Ref, name string, version *int64, ops ...entity.Op) entity.ChangeSet {
	return entity.ChangeSet{
		Entity:          eventbus.Entity{Entity: ref.EventRef(), Name: name},
		ExpectedVersion: version,
		Ops:             ops,
	}
}

func playerRef(id string) entity.Ref { return entity.Ref{ID: id, Type: entity.TypePlayer} }

func versionOf(t *testing.T, s *state.FakeState, id string) *int64 {
	t.Helper()
	e, ok := s.Get(id)
	if !ok {
		t.Fatalf("%s is not in the world", id)
	}
	v := e.Version
	return &v
}

// --- the start protocol ---

// TestStartAnnouncesRecovery is the addendum of сведение 3 (C-14 v0.4, TL2-6):
// the stub keeps the start protocol instead of making every consumer grow a
// branch for the stub. The signal has to be a valid analytics.replay.completed
// — the schema of T-006 decides that, not this test — published by a source
// the registry allows to publish it.
func TestStartAnnouncesRecovery(t *testing.T) {
	fake, bus, _ := world(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); _ = fake.Wait() }()

	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	signals := ofType(events(t, bus, eventbus.TopicAnalyticsEvents), state.TypeReplayDone)
	if len(signals) != 1 {
		t.Fatalf("%d recovery signals, want exactly one", len(signals))
	}
	ev := signals[0]

	if ev.Source != state.Source {
		t.Errorf("source %q, want %q", ev.Source, state.Source)
	}
	spec, ok := contracts.Lookup(state.TypeReplayDone)
	if !ok {
		t.Fatalf("%s is not in the registry", state.TypeReplayDone)
	}
	if !containsString(spec.Publishers, ev.Source) {
		t.Errorf("source %q is not among the publishers %v of %s",
			ev.Source, spec.Publishers, state.TypeReplayDone)
	}
	if err := contracts.Validate(ev); err != nil {
		t.Fatalf("the recovery signal is not valid: %v", err)
	}
	if ev.Meta.CausationID != "" {
		t.Errorf("causation %q: nothing caused the start of a process", ev.Meta.CausationID)
	}

	payload := ev.Path()
	// The literal is the contract value (C-14): the schema of the type admits
	// mode=test as well — that is the harness of EPIC-005 — so a comparison
	// with state.ReplayModeRecovery would only prove that the stub agrees with
	// itself, whichever value the constant had come to hold.
	if mode, _ := payload.GetString("mode"); mode != "recovery" {
		t.Errorf("mode %q, want %q", mode, "recovery")
	}
	if state.ReplayModeRecovery != "recovery" {
		t.Errorf("state.ReplayModeRecovery is %q: consumers wait for the contract value %q",
			state.ReplayModeRecovery, "recovery")
	}
	replay, ok := payload.GetMap("replay")
	if !ok {
		t.Fatal("the signal carries no replay block")
	}
	if snapshot, present := replay["snapshot_id"]; !present || snapshot != nil {
		t.Errorf("snapshot_id %v, want null: the stub caught up from nothing", snapshot)
	}
	for _, counter := range []string{"events_replayed", "llm_calls", "dice_rolled_new"} {
		if value, _ := payload.GetInt("replay." + counter); value != 0 {
			t.Errorf("%s = %d: the stub replayed no journal", counter, value)
		}
	}
	if incomplete, _ := payload.GetBool("replay.incomplete_record"); incomplete {
		t.Error("incomplete_record is true on a catch-up that never happened")
	}
	if hash, _ := payload.GetString("replay.state_hash_after"); hash != fake.StateHash() {
		t.Errorf("state_hash_after %q, the world hashes to %q", hash, fake.StateHash())
	}
}

// TestNothingPublishedBeforeStartIsLost is what the stub relies on instead of
// a readiness handshake: a new consumer group starts at the first offset, so a
// proposal already in the topic is applied once the stub gets there (C-01
// v1.2, ADR-022; the bus side is pinned by the contract case
// ANewGroupStartsAtTheFirstOffset).
func TestNothingPublishedBeforeStartIsLost(t *testing.T) {
	fake, bus, _ := world(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); _ = fake.Wait() }()

	before := versionOf(t, fake, playerA)
	if err := bus.Publish(ctx, proposal("prop-early", "combat", true,
		changeSet(playerRef(playerA), "Вася", before,
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -3}))); err != nil {
		t.Fatalf("publish before start: %v", err)
	}

	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, "the fact of the proposal published before Start", func() bool {
		for _, ev := range ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeUpdated) {
			if id, _ := ev.Path().GetString("proposal_id"); id == "prop-early" {
				return true
			}
		}
		return false
	})
}

func TestStartTwiceIsRefused(t *testing.T) {
	fake, _, _ := world(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); _ = fake.Wait() }()

	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := fake.Start(ctx); err == nil {
		t.Fatal("started twice: two subscriptions of one group would split the proposals")
	}
}

// --- the cycle a consumer sees: proposal in, fact out ---

// TestProposalBecomesFactForEveryOp runs the four operations of C-02 through
// the bus, one at a time, and reads the facts back off the topic. It is the
// cycle every consumer of the stub depends on, and running it through membus
// rather than through Apply is what makes it cover the wiring as well as the
// decision.
func TestProposalBecomesFactForEveryOp(t *testing.T) {
	fake, bus, _ := world(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); _ = fake.Wait() }()
	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	pelt := map[string]any{
		"item_id": "pelt-1", "kind": "wolf-pelt", "name": "волчья шкура",
		"source":      map[string]any{"entity": map[string]any{"id": npcID, "type": entity.TypeNPC}, "event_id": "ev-x"},
		"acquired_at": testkit.Epoch.Format(time.RFC3339),
	}

	for _, tc := range []struct {
		name  string
		op    entity.Op
		cause string
		path  string
	}{
		{"set", entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}, "move", entity.AttrPosition},
		{"inc", entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -3}, "combat", entity.AttrHP},
		{"append", entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt}, "loot", "inventory[0]"},
		{"remove", entity.Op{Op: entity.OpRemove, Path: entity.AttrInventory, Value: pelt}, "loot", entity.AttrInventory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := versionOf(t, fake, playerA)
			id := "prop-" + tc.name

			if err := bus.Publish(ctx, proposal(id, tc.cause, true,
				changeSet(playerRef(playerA), "Вася", before, tc.op))); err != nil {
				t.Fatalf("publish the proposal: %v", err)
			}

			var fact eventbus.Event
			waitFor(t, "the fact of "+id, func() bool {
				for _, ev := range ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeUpdated) {
					if pid, _ := ev.Path().GetString("proposal_id"); pid == id {
						fact = ev
						return true
					}
				}
				return false
			})

			if err := contracts.Validate(fact); err != nil {
				t.Fatalf("the fact is not a valid %s: %v", state.TypeUpdated, err)
			}
			if fact.Source != state.Source {
				t.Errorf("source %q, want %q", fact.Source, state.Source)
			}
			// A fact is derived from the proposal: same chain, same instant,
			// the proposal named as the cause (C-01, C-02).
			if fact.Meta.CausationType != state.TypeUpdateProposed {
				t.Errorf("causation_type %q, want %q", fact.Meta.CausationType, state.TypeUpdateProposed)
			}
			if !fact.Timestamp.Equal(testkit.Epoch) {
				t.Errorf("timestamp %s, want the instant of the proposal %s", fact.Timestamp, testkit.Epoch)
			}
			if applied, _ := fact.Path().GetString("applied_at"); applied != testkit.Epoch.Format(time.RFC3339Nano) {
				t.Errorf("applied_at %q, want the timestamp of the proposal", applied)
			}
			if cause, _ := fact.Path().GetString("cause"); cause != tc.cause {
				t.Errorf("cause %q, want %q", cause, tc.cause)
			}

			version, _ := fact.Path().GetInt("version")
			if int64(version) != *before+1 {
				t.Errorf("version %d, want %d", version, *before+1)
			}
			changed, ok := fact.Path().GetSlice("changed")
			if !ok || len(changed) != 1 {
				t.Fatalf("changed %v, want one path", changed)
			}
			first, _ := changed[0].(map[string]any)
			if first["path"] != tc.path {
				t.Errorf("changed path %v, want %q", first["path"], tc.path)
			}
		})
	}
}

// TestCreateProposalBecomesEntityCreated covers the other half of C-02: a
// character that did not exist a moment ago, at version 1, with the attributes
// the gateway sent.
func TestCreateProposalBecomesEntityCreated(t *testing.T) {
	fake, bus, _ := world(t)
	ctx := context.Background()

	attrs := map[string]any{
		entity.AttrHP: 10, entity.AttrHPMax: 10, entity.AttrAtk: 2, entity.AttrDef: 12,
		entity.AttrDmg: "d6", entity.AttrFlee: "2", entity.AttrStatus: entity.StatusAlive,
		entity.AttrPosition:  "outside:" + worldID,
		entity.AttrScope:     map[string]any{"id": "player-D", "type": "solo"},
		entity.AttrActorKind: entity.ActorKindCI,
		entity.AttrInventory: []any{},
	}
	ev := eventbus.NewRoot(state.TypeCreateProposed, contracts.SourceGateway, worldID, nil,
		entity.ActorKindCI, map[string]any{
			"proposal_id": "prop-create",
			"entity": map[string]any{
				"entity": map[string]any{"id": "player-D", "type": entity.TypePlayer},
				"name":   "Дима",
			},
			"attributes": attrs,
			"cause":      "create",
		})
	if err := fake.Apply(ctx, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}

	facts := ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeCreated)
	if len(facts) != 1 {
		t.Fatalf("%d %s, want one", len(facts), state.TypeCreated)
	}
	if err := contracts.Validate(facts[0]); err != nil {
		t.Fatalf("the fact is not valid: %v", err)
	}
	if version, _ := facts[0].Path().GetInt("version"); version != 1 {
		t.Errorf("version %d, want 1", version)
	}

	created, ok := fake.Get("player-D")
	if !ok {
		t.Fatal("the character is not in the world")
	}
	if created.Name != "Дима" || created.WorldID != worldID || created.Version != 1 {
		t.Errorf("created %+v", created)
	}
	if !created.CreatedAt.Equal(testkit.Epoch) || !created.UpdatedAt.Equal(testkit.Epoch) {
		t.Errorf("created_at %s / updated_at %s, want the instant of the proposal",
			created.CreatedAt, created.UpdatedAt)
	}
	if created.LastEventID != facts[0].ID {
		t.Errorf("last_event_id %q, the fact is %q", created.LastEventID, facts[0].ID)
	}
}

func containsString(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}
