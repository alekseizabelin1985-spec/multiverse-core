package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

// The fixtures and the rule book of the tree, relative to this package.
var (
	fixturesDir = filepath.Join("..", "..", "testdata", "fixtures")
	rulesBook   = filepath.Join("..", "..", "rules", "dark-forest.yaml")
)

// bootstrapOrder is the order §4.10 creates the world of the fixtures in.
var bootstrapOrder = []entity.Ref{
	{ID: world, Type: entity.TypeWorld},
	{ID: "dark-forest-01", Type: entity.TypeRegion},
	{ID: "wolf-alpha", Type: entity.TypeNPC},
	{ID: "player-A", Type: entity.TypePlayer},
	{ID: "player-B", Type: entity.TypePlayer},
	{ID: "player-C", Type: entity.TypePlayer},
}

// bootstrapStand is State as the process runs it — held to the laws of the rule
// book, writing through to a memory object store — over the in-process bus.
type bootstrapStand struct {
	bus     *membus.Bus
	objects *objstore.Memory
	state   *state.Context
	deps    runtime.Deps
}

func newBootstrapStand(t *testing.T) *bootstrapStand {
	t.Helper()
	sources := testkit.Deterministic(t, "t058")
	eventbus.SetRegistry(contracts.Default())
	rules, err := mechanics.Load(rulesBook)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	bus := newBus(t)
	objects := objstore.NewMemoryWithClock(testkit.Wall())
	if err := state.EnsureWorldBuckets(context.Background(), objects, world); err != nil {
		t.Fatalf("buckets: %v", err)
	}
	c := state.New(state.Config{
		Worlds: []string{world}, Timers: sources.Timers, Invariants: rules.Invariants(),
		Objects: objects, SnapshotEvery: -1, RulesVersion: rules.Version,
	})
	deps := runtime.Deps{Bus: bus, Journal: bus, Contracts: contracts.Default(), Clock: sources.Clock}
	if err := c.Start(context.Background(), deps); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		_ = c.Stop(ctx)
	})
	return &bootstrapStand{bus: bus, objects: objects, state: c, deps: deps}
}

func (s *bootstrapStand) bootstrap(t *testing.T) state.BootstrapResult {
	t.Helper()
	result, err := state.Bootstrap(context.Background(), s.deps, world, fixturesDir)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return result
}

// records are the events of system_events in offset order.
func records(t *testing.T, bus *membus.Bus) []eventbus.Event {
	t.Helper()
	raw, err := bus.Records(eventbus.TopicSystemEvents)
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	out := make([]eventbus.Event, 0, len(raw))
	for _, body := range raw {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out = append(out, ev)
	}
	return out
}

func bootstrapIDs() []string {
	ids := make([]string, 0, len(bootstrapOrder))
	for _, ref := range bootstrapOrder {
		ids = append(ids, state.BootstrapProposalID(world, ref))
	}
	return ids
}

// The DoD of T-058: the world of the fixtures comes into being through State, in
// the order of §4.10, as the fixtures describe it.
func TestBootstrapCreatesTheWorldOfTheFixtures(t *testing.T) {
	s := newBootstrapStand(t)
	result := s.bootstrap(t)

	if !reflect.DeepEqual(result.Created, bootstrapOrder) || len(result.Skipped) != 0 {
		t.Fatalf("created %v, skipped %v; want %v created in that order", result.Created, result.Skipped, bootstrapOrder)
	}
	var proposed, created []string
	for _, ev := range records(t, s.bus) {
		id, _ := ev.Path().GetString("proposal_id")
		switch ev.Type {
		case state.TypeCreateProposed:
			proposed = append(proposed, id)
		case state.TypeCreated:
			created = append(created, id)
		}
	}
	if !slices.Equal(proposed, bootstrapIDs()) || !slices.Equal(created, bootstrapIDs()) {
		t.Fatalf("proposed %v and created %v, want both %v", proposed, created, bootstrapIDs())
	}

	fixtures, err := state.LoadFixtures(world, fixturesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range fixtures {
		got, ok := s.state.Get(world, want.ID)
		if !ok {
			t.Errorf("%s is not in the world", want.ID)
			continue
		}
		if got.Version != 1 || got.Name != want.Name || got.WorldID != world {
			t.Errorf("%s: version %d, name %q, world %q; want 1, %q, %s", want.ID, got.Version, got.Name, got.WorldID, want.Name, world)
		}
		if !reflect.DeepEqual(got.Attributes, want.Attributes) {
			t.Errorf("%s: attributes %v, want the fixture's %v", want.ID, got.Attributes, want.Attributes)
		}
		if got.LastChange == nil || got.LastChange.Cause != state.CauseInit || got.LastChange.FactEventID == "" {
			t.Errorf("%s: commit record %+v, want cause init with its fact", want.ID, got.LastChange)
		}
	}
}

// The snapshot of a bootstrapped world is the world of the fixtures: the
// state_hash the fixture pointer of seq 0 carries, the window of the six
// bootstrap proposals, and the cursor past the last of them — not 0, the value
// of the fixture (acceptance of T-057, KD §4.10 v0.4).
func TestTheSnapshotOfABootstrapIsTheWorldOfTheFixtures(t *testing.T) {
	s := newBootstrapStand(t)
	s.bootstrap(t)
	pointer, err := s.state.Snapshot(context.Background(), world, state.SnapshotBootstrap)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	var fixture state.LatestPointer
	body, err := os.ReadFile(filepath.Join(fixturesDir, "snapshots", "state", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	meta := pointer.Snapshot
	if meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap || meta.EntitiesCount != 6 {
		t.Errorf("snapshot seq %d, reason %s, %d entities; want seq 0, bootstrap, 6", meta.Seq, meta.Reason, meta.EntitiesCount)
	}
	if meta.StateHash != fixture.Snapshot.StateHash {
		t.Errorf("state_hash %s, want the fixture's %s", meta.StateHash, fixture.Snapshot.StateHash)
	}
	if meta.RulesVersion != fixture.Snapshot.RulesVersion || meta.LawsVersion != fixture.Snapshot.LawsVersion {
		t.Errorf("rules %s, laws %s; want the fixture's %s, %s", meta.RulesVersion, meta.LawsVersion,
			fixture.Snapshot.RulesVersion, fixture.Snapshot.LawsVersion)
	}

	events := records(t, s.bus)
	lastProposal := -1
	for offset, ev := range events {
		if ev.Type == state.TypeCreateProposed {
			lastProposal = offset
		}
	}
	cursor := meta.Cursor[eventbus.TopicSystemEvents]
	if want := int64(lastProposal + 1); cursor != want {
		t.Errorf("cursor %d, want %d: past the last proposal of the bootstrap", cursor, want)
	}
	end, err := s.bus.End(context.Background(), eventbus.TopicSystemEvents)
	if err != nil {
		t.Fatal(err)
	}
	if cursor <= 0 || cursor > end {
		t.Errorf("cursor %d outside (0, End %d]", cursor, end)
	}
	for _, ev := range events[cursor:] {
		if state.IsProposal(ev.Type) {
			t.Errorf("proposal %s %s past the cursor: a recovery would not answer it", ev.Type, ev.ID)
		}
	}

	snap, err := state.NewObjectStore(s.objects).ReadSnapshot(context.Background(), world, meta.Key)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(snap.AppliedProposals, bootstrapIDs()) {
		t.Errorf("applied_proposals %v, want %v", snap.AppliedProposals, bootstrapIDs())
	}
}

// A second bootstrap of the same world creates nothing and publishes no fact
// (DoD of T-058): the proposals already applied are recognised by their
// proposal_id.
func TestASecondBootstrapCreatesNothing(t *testing.T) {
	s := newBootstrapStand(t)
	s.bootstrap(t)
	before := len(records(t, s.bus))

	again := s.bootstrap(t)
	if len(again.Created) != 0 || !reflect.DeepEqual(again.Skipped, bootstrapOrder) {
		t.Fatalf("second bootstrap created %v, skipped %v; want nothing created and %v skipped",
			again.Created, again.Skipped, bootstrapOrder)
	}
	if got := len(onTopic(t, s.bus, state.TypeCreated)); got != len(bootstrapOrder) {
		t.Errorf("%d entity.created on the journal, want %d", got, len(bootstrapOrder))
	}
	if after := len(records(t, s.bus)); after != before {
		t.Errorf("the second bootstrap published %d events", after-before)
	}
}

// The proposals of a bootstrap are the operator's (C-02 v1.5): the registry
// takes them and lists their source among the publishers of the type, and State
// reads their proposer as the author.
func TestBootstrapProposalsAreValidAgainstTheRegistry(t *testing.T) {
	testkit.Deterministic(t, "t058")
	eventbus.SetRegistry(contracts.Default())
	fixtures, err := state.LoadFixtures(world, fixturesDir)
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := contracts.Lookup(state.TypeCreateProposed)
	if !ok {
		t.Fatalf("%s is not registered", state.TypeCreateProposed)
	}
	for _, e := range fixtures {
		ev := state.BootstrapProposal(world, e)
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("%s: the registry refuses the proposal: %v", e.ID, err)
		}
		if !slices.Contains(spec.Publishers, ev.Source) || ev.Source != contracts.SourceMvctl {
			t.Errorf("%s: source %q, want %s, which is among the publishers %v", e.ID, ev.Source, contracts.SourceMvctl, spec.Publishers)
		}
		if ev.Meta.ActorKind != eventbus.ActorSystem || ev.World == nil || ev.World.Entity.ID != world || ev.Meta.Agent != nil {
			t.Errorf("%s: actor %q, world %v, agent %v; want system, %s, none", e.ID, ev.Meta.ActorKind, ev.World, ev.Meta.Agent, world)
		}
		p, err := state.ParseProposal(ev)
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		if p.ID != state.BootstrapProposalID(world, e.Ref()) || p.Cause != state.CauseInit || p.Proposer.Kind != state.ProposerAuthor {
			t.Errorf("%s: proposal %q, cause %q, proposer %q; want bootstrap:%s:%s/%s, init, author",
				e.ID, p.ID, p.Cause, p.Proposer.Kind, world, e.Type, e.ID)
		}
	}
}

// The numbers of a fighter live twice on purpose — in the entity and in the
// rules — and a fixture that drifts from rules/dark-forest.yaml is a defect
// (state-and-mechanics.md §4.10).
func TestFixtureStatsAreTheStatsOfTheRules(t *testing.T) {
	rules, err := mechanics.Load(rulesBook)
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := state.LoadFixtures(world, fixturesDir)
	if err != nil {
		t.Fatal(err)
	}
	fighters := 0
	for _, e := range fixtures {
		if e.Type != entity.TypePlayer && e.Type != entity.TypeNPC {
			continue
		}
		fighters++
		actor, err := mechanics.ActorFromEntity(e, nil)
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		kind := e.Type
		if e.Type == entity.TypeNPC {
			kind = actor.Kind
		}
		want, ok := rules.Stats(kind)
		if !ok {
			t.Errorf("%s: the rules have no stats of %q", e.ID, kind)
			continue
		}
		dmg, err := mechanics.ParseDice(actor.Dmg)
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		got := [...]any{actor.HPMax, actor.HP, actor.Atk, actor.Def, dmg.String(), actor.Flee, actor.Status}
		stats := [...]any{want.HPMax, want.HPMax, want.Atk, want.Def, want.Dmg, want.Flee, want.Status}
		if got != stats {
			t.Errorf("%s (%s): hp_max, hp, atk, def, dmg, flee, status = %v; the rules say %v", e.ID, kind, got, stats)
		}
	}
	if fighters != 4 {
		t.Errorf("%d fighters in the fixtures, want the wolf and three characters", fighters)
	}
}

// respond answers every proposal of the world on the bus with a refusal of the
// reason reasonOf gives it, as State would publish one.
func respond(t *testing.T, bus *membus.Bus, reasonOf func(ref entity.Ref) state.Reason) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = bus.Subscribe(ctx, eventbus.TopicSystemEvents, "t058-responder", func(ctx context.Context, ev eventbus.Event) error {
			if ev.Type != state.TypeCreateProposed {
				return nil
			}
			p, err := state.ParseProposal(ev)
			if err != nil {
				return err
			}
			ref := p.Create.Ref
			reason := reasonOf(ref)
			if reason == "" {
				return nil
			}
			return bus.Publish(ctx, eventbus.Derive(ev, state.TypeRejected, contracts.SourceState, map[string]any{
				"proposal_id": p.ID,
				"reason":      string(reason),
				"entity":      map[string]any{"entity": map[string]any{"id": ref.ID, "type": ref.Type}},
			}, eventbus.WithCauseID(ref.ID)))
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func bareDeps(t *testing.T) (*membus.Bus, runtime.Deps) {
	t.Helper()
	testkit.Deterministic(t, "t058")
	eventbus.SetRegistry(contracts.Default())
	bus := newBus(t)
	return bus, runtime.Deps{Bus: bus, Journal: bus}
}

// An entity State already holds under another commit record is refused
// duplicate_entity (§4.10): the bootstrap skips it and goes on.
func TestBootstrapSkipsWhatStateRefusesAsDuplicate(t *testing.T) {
	bus, deps := bareDeps(t)
	respond(t, bus, func(entity.Ref) state.Reason { return state.ReasonDuplicateEntity })

	result, err := state.Bootstrap(context.Background(), deps, world, fixturesDir)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if len(result.Created) != 0 || !reflect.DeepEqual(result.Skipped, bootstrapOrder) {
		t.Errorf("created %v, skipped %v; want every entity skipped", result.Created, result.Skipped)
	}
}

// Any other refusal ends the bootstrap: the next entity may stand in the one
// refused, and a world created in part is no world the fixtures describe.
func TestBootstrapStopsOnARefusal(t *testing.T) {
	bus, deps := bareDeps(t)
	respond(t, bus, func(ref entity.Ref) state.Reason {
		if ref.Type == entity.TypeRegion {
			return state.ReasonLevelViolation
		}
		return state.ReasonDuplicateEntity
	})

	result, err := state.Bootstrap(context.Background(), deps, world, fixturesDir)
	if !errors.Is(err, state.ErrBootstrap) || !strings.Contains(err.Error(), string(state.ReasonLevelViolation)) {
		t.Fatalf("err %v, want ErrBootstrap naming level_violation", err)
	}
	if len(onTopic(t, bus, state.TypeCreateProposed)) != 2 {
		t.Errorf("proposed %d entities, want the world and the refused region only", len(onTopic(t, bus, state.TypeCreateProposed)))
	}
	if len(result.Skipped) != 1 || result.Skipped[0].ID != world {
		t.Errorf("skipped %v, want the world", result.Skipped)
	}
}

// Nobody answers: the bootstrap gives up after BootstrapTimeout on its timers
// and says why.
func TestBootstrapGivesUpWithoutAnAnswer(t *testing.T) {
	_, deps := bareDeps(t)
	manual := clock.NewManual(testkit.Epoch)
	type outcome struct {
		result state.BootstrapResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := state.Bootstrap(context.Background(), deps, world, fixturesDir,
			state.WithAnswerTimers(clock.NewManualTimers(manual)))
		done <- outcome{result, err}
	}()

	var got outcome
	waitFor(t, "the bootstrap to give up", func() bool {
		select {
		case got = <-done:
			return true
		default:
			manual.Advance(state.BootstrapTimeout - time.Millisecond)
			return false
		}
	})
	if !errors.Is(got.err, state.ErrBootstrap) || !strings.Contains(got.err.Error(), "no answer") {
		t.Fatalf("err %v, want ErrBootstrap saying there was no answer", got.err)
	}
	if len(got.result.Created) != 0 {
		t.Errorf("created %v without an answer", got.result.Created)
	}
}

// A cancelled bootstrap returns the cancellation, not a timeout.
func TestBootstrapEndsWithItsContext(t *testing.T) {
	_, deps := bareDeps(t)
	ctx, cancel := context.WithCancel(context.Background())
	manual := clock.NewManual(testkit.Epoch)
	done := make(chan error, 1)
	go func() {
		_, err := state.Bootstrap(ctx, deps, world, fixturesDir, state.WithAnswerTimers(clock.NewManualTimers(manual)))
		done <- err
	}()
	cancel()
	var err error
	waitFor(t, "the bootstrap to end", func() bool {
		select {
		case err = <-done:
			return true
		default:
			return false
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err %v, want context.Canceled", err)
	}
}

// The answers a bootstrap waits for are the ones after End: on a journal that
// holds the refusal of a run before, the next run with fixed fixtures is answered
// by State, not by that old refusal (review #1 of T-058, Mi-2).
func TestASecondBootstrapIsNotAnsweredByTheRefusalOfTheFirst(t *testing.T) {
	s := newBootstrapStand(t)
	broken := copyFixtures(t)
	path := filepath.Join(broken, "npc.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	moved := strings.Replace(string(body), `"position": "dark-forest-01"`, `"position": "nowhere"`, 1)
	if moved == string(body) {
		t.Fatal("the wolf of the fixtures has no position to move")
	}
	if err := os.WriteFile(path, []byte(moved), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := state.Bootstrap(context.Background(), s.deps, world, broken)
	if !errors.Is(err, state.ErrBootstrap) || !strings.Contains(err.Error(), string(state.ReasonLawViolation)) {
		t.Fatalf("first bootstrap: %v, want ErrBootstrap refused law_violation", err)
	}
	if !reflect.DeepEqual(first.Created, bootstrapOrder[:2]) {
		t.Fatalf("first bootstrap created %v, want the world and the region", first.Created)
	}

	second, err := state.Bootstrap(context.Background(), s.deps, world, fixturesDir)
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if !reflect.DeepEqual(second.Skipped, bootstrapOrder[:2]) || !reflect.DeepEqual(second.Created, bootstrapOrder[2:]) {
		t.Errorf("second bootstrap created %v, skipped %v; want %v created and %v skipped",
			second.Created, second.Skipped, bootstrapOrder[2:], bootstrapOrder[:2])
	}
	if got := len(onTopic(t, s.bus, state.TypeCreated)); got != len(bootstrapOrder) {
		t.Errorf("%d entity.created on the journal, want one per entity", got)
	}
}

// brokenTail is a journal whose Tail fails at once.
type brokenTail struct{ *membus.Bus }

func (brokenTail) Tail(context.Context, string, int64, eventbus.Handler) error {
	return errors.New("the journal refused to tail")
}

// A journal that stops is the answer, not a timeout: the bootstrap says what the
// journal said without waiting BootstrapTimeout (review #1 of T-058, N-2).
func TestBootstrapEndsWhenItsJournalStops(t *testing.T) {
	bus, _ := bareDeps(t)
	manual := clock.NewManual(testkit.Epoch)
	done := make(chan error, 1)
	go func() {
		_, err := state.Bootstrap(context.Background(), runtime.Deps{Bus: bus, Journal: brokenTail{bus}}, world, fixturesDir,
			state.WithAnswerTimers(clock.NewManualTimers(manual)))
		done <- err
	}()
	var err error
	waitFor(t, "the bootstrap to end", func() bool {
		select {
		case err = <-done:
			return true
		default:
			return false
		}
	})
	if !errors.Is(err, state.ErrBootstrap) || !strings.Contains(err.Error(), "the journal refused to tail") {
		t.Errorf("err %v, want ErrBootstrap with the error of the journal", err)
	}
}

func TestBootstrapNeedsTheJournal(t *testing.T) {
	bus, _ := bareDeps(t)
	_, err := state.Bootstrap(context.Background(), runtime.Deps{Bus: bus}, world, fixturesDir)
	if !errors.Is(err, state.ErrBootstrap) {
		t.Errorf("err %v, want ErrBootstrap", err)
	}
	if got := len(onTopic(t, bus, state.TypeCreateProposed)); got != 0 {
		t.Errorf("%d proposals published without a journal to read the answers from", got)
	}
}

// A fixtures directory that does not describe the world is refused before
// anything is proposed.
func TestLoadFixturesRefusesFixturesOfAnotherShape(t *testing.T) {
	cases := map[string]struct {
		file   string
		mutate func(entities []map[string]any) []map[string]any
		want   string
	}{
		"an entity of another world": {"npc.json", func(es []map[string]any) []map[string]any {
			es[0]["world_id"] = "another-world"
			return es
		}, "another-world"},
		"a type in the file of another": {"region.json", func(es []map[string]any) []map[string]any {
			es[0]["type"] = entity.TypeNPC
			return es
		}, "want region"},
		"an id used twice": {"players.json", func(es []map[string]any) []map[string]any {
			es[1]["id"] = "player-A"
			return es
		}, "already used"},
		"an unknown field": {"npc.json", func(es []map[string]any) []map[string]any {
			es[0]["atributes"] = map[string]any{}
			return es
		}, "unknown field"},
		"an empty file": {"region.json", func([]map[string]any) []map[string]any {
			return []map[string]any{}
		}, "no entities"},
		"a second world": {"world.json", func(es []map[string]any) []map[string]any {
			second := map[string]any{}
			for k, v := range es[0] {
				second[k] = v
			}
			second["id"] = "dark-forest-world-2"
			second["world_id"] = "dark-forest-world-2"
			return append(es, second)
		}, "dark-forest-world-2"},
		"an entity without an id": {"npc.json", func(es []map[string]any) []map[string]any {
			delete(es[0], "id")
			return es
		}, "without an id"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := copyFixtures(t)
			path := filepath.Join(dir, tc.file)
			var entities []map[string]any
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &entities); err != nil {
				t.Fatal(err)
			}
			changed, err := json.Marshal(tc.mutate(entities))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = state.LoadFixtures(world, dir)
			if !errors.Is(err, state.ErrFixtures) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err %v, want ErrFixtures mentioning %q", err, tc.want)
			}
		})
	}

	t.Run("another world asked for", func(t *testing.T) {
		if _, err := state.LoadFixtures("another-world", fixturesDir); !errors.Is(err, state.ErrFixtures) {
			t.Errorf("err %v, want ErrFixtures", err)
		}
	})
	t.Run("no world asked for", func(t *testing.T) {
		if _, err := state.LoadFixtures("", fixturesDir); !errors.Is(err, state.ErrFixtures) {
			t.Errorf("err %v, want ErrFixtures", err)
		}
	})
	t.Run("a file that is not there", func(t *testing.T) {
		dir := copyFixtures(t)
		if err := os.Remove(filepath.Join(dir, "players.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := state.LoadFixtures(world, dir); !errors.Is(err, state.ErrFixtures) {
			t.Errorf("err %v, want ErrFixtures", err)
		}
	})
	t.Run("two documents in a file", func(t *testing.T) {
		dir := copyFixtures(t)
		path := filepath.Join(dir, "npc.json")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(body, []byte("\n[]\n")...), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := state.LoadFixtures(world, dir); !errors.Is(err, state.ErrFixtures) {
			t.Errorf("err %v, want ErrFixtures", err)
		}
	})
}

// copyFixtures copies the entity files of the fixtures into a directory of the
// test.
func copyFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, file := range state.FixtureFiles {
		body, err := os.ReadFile(filepath.Join(fixturesDir, file.Name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file.Name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
