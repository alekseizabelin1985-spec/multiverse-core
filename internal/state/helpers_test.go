package state_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
)

const world = "dark-forest-world"

// --- a publisher that is the wire ---

// journal is the Publisher of the Applier tests: it validates every event
// against the registry, as the bus does on Publish, and keeps what went out as
// a subscriber would read it — the raw bytes and the event decoded from them.
// fail, when set, decides the fate of the n-th call before anything is kept.
type journal struct {
	mu     sync.Mutex
	calls  int
	fail   func(n int, ev eventbus.Event) error
	raw    [][]byte
	events []eventbus.Event
}

func (j *journal) Publish(_ context.Context, ev eventbus.Event) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls++
	if j.fail != nil {
		if err := j.fail(j.calls, ev); err != nil {
			return err
		}
	}
	if err := contracts.Validate(ev); err != nil {
		return fmt.Errorf("the registry refuses %s: %w", ev.Type, err)
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	var decoded eventbus.Event
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	j.raw = append(j.raw, raw)
	j.events = append(j.events, decoded)
	return nil
}

func (j *journal) ofType(typ string) []eventbus.Event {
	j.mu.Lock()
	defer j.mu.Unlock()
	var out []eventbus.Event
	for _, ev := range j.events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func (j *journal) all() []eventbus.Event {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]eventbus.Event(nil), j.events...)
}

// --- an applier over a world ---

type fixture struct {
	applier *state.Applier
	store   *memstore.Store
	journal *journal
	log     *lockedBuffer
	// clock paces the attempts to publish an answer again: they wait until a
	// test advances it.
	clock *clock.Manual
}

// newFixture builds the Applier of the world with deterministic sources, so
// that ids and timestamps of the proposals are the same on every run.
//
// The pipeline tests of T-055 run it without the ownership check: what they
// pin — versions, atomic packages, the attempts to publish — does not depend on
// who proposes. The checks of T-056 run on newOwnedFixture, and the context
// always enforces ownership.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	return fixtureWith(t, state.ApplierConfig{WithoutOwnership: true})
}

// newOwnedFixture is the Applier as State runs it: the ownership table and the
// norms over it in force, and the laws given.
func newOwnedFixture(t *testing.T, laws ...mechanics.Invariant) *fixture {
	t.Helper()
	return fixtureWith(t, state.ApplierConfig{Invariants: laws})
}

func fixtureWith(t *testing.T, cfg state.ApplierConfig) *fixture {
	t.Helper()
	sources := testkit.Deterministic(t, "t055")
	eventbus.SetRegistry(contracts.Default())
	f := &fixture{store: memstore.New(), journal: &journal{}, log: &lockedBuffer{}, clock: sources.Clock}
	cfg.WorldID, cfg.Store, cfg.Publisher, cfg.Timers = world, f.store, f.journal, sources.Timers
	cfg.Log = slog.New(slog.NewJSONHandler(f.log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	applier, err := state.NewApplier(cfg)
	if err != nil {
		t.Fatalf("NewApplier: %v", err)
	}
	f.applier = applier
	return f
}

func (f *fixture) apply(t *testing.T, ev eventbus.Event) {
	t.Helper()
	if err := f.applier.Apply(context.Background(), ev); err != nil {
		t.Fatalf("apply %s %s: %v", ev.Type, ev.ID, err)
	}
}

// seed puts an entity into the world at a version, without a proposal.
func (f *fixture) seed(t *testing.T, id, typ string, version int64, attrs map[string]any) {
	t.Helper()
	e := entity.New(entity.Ref{ID: id, Type: typ}, world, id, attrs, testkit.Epoch)
	e.Version = version
	if err := f.store.Put(world, e); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func (f *fixture) get(t *testing.T, id string) *entity.Entity {
	t.Helper()
	e, ok := f.store.Get(world, id)
	if !ok {
		t.Fatalf("%s is not in the world", id)
	}
	return e
}

// logged is every record of the log whose message is msg.
func (f *fixture) logged(t *testing.T, msg string) []map[string]any {
	t.Helper()
	return recordsOf(t, f.log.String(), msg)
}

// applyAdvancing runs Apply on its own goroutine, as the worker does, and moves
// the manual clock on until it returns: every pause between two attempts to
// publish ends at the next move. Real time only bounds a defect.
func (f *fixture) applyAdvancing(t *testing.T, ctx context.Context, ev eventbus.Event) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- f.applier.Apply(ctx, ev) }()
	var err error
	waitFor(t, "Apply to return", func() bool {
		select {
		case err = <-done:
			return true
		default:
			f.clock.Advance(time.Minute)
			return false
		}
	})
	return err
}

func recordsOf(t *testing.T, text, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.Lines(text) {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("a log line is not JSON: %q", line)
		}
		if rec["msg"] == msg {
			out = append(out, rec)
		}
	}
	return out
}

// --- proposals ---

func ref(id, typ string) entity.Ref { return entity.Ref{ID: id, Type: typ} }

func version(v int64) *int64 { return &v }

func set(r entity.Ref, expected *int64, ops ...entity.Op) entity.ChangeSet {
	return entity.ChangeSet{
		Entity:          eventbus.Entity{Entity: r.EventRef()},
		ExpectedVersion: expected,
		Ops:             ops,
	}
}

func op(kind entity.OpKind, path string, value any) entity.Op {
	return entity.Op{Op: kind, Path: path, Value: value}
}

// update is entity.update.proposed as the author publishes it through mvctl,
// with the change sets encoded the way they travel. The author may change
// anything with cause=author (§4.6), so a test that is not about ownership
// passes that cause through a context that enforces it.
func update(t *testing.T, proposalID, cause string, atomic bool, sets ...entity.ChangeSet) eventbus.Event {
	t.Helper()
	return proposed(t, author, proposalID, cause, atomic, sets...)
}

// proposed is the same proposal published by a proposer of the ownership
// table.
func proposed(t *testing.T, by proposer, proposalID, cause string, atomic bool, sets ...entity.ChangeSet) eventbus.Event {
	t.Helper()
	payload := map[string]any{
		"proposal_id": proposalID,
		"changes":     wire(t, sets),
		"atomic":      atomic,
		"cause":       cause,
	}
	var opts []eventbus.DeriveOption
	if by.level != "" {
		opts = append(opts, eventbus.WithAgent(eventbus.AgentRef{ID: "agent:" + by.level, Level: by.level, Blueprint: "test"}))
	}
	return eventbus.NewRoot(state.TypeUpdateProposed, by.source, world, nil, eventbus.ActorCI, payload, opts...)
}

// create is entity.create.proposed as the bootstrap of a world publishes it
// (source mvctl, cause init, proposer author; §4.10).
func create(proposalID string, r entity.Ref, name string, attrs map[string]any) eventbus.Event {
	return createdBy(author, proposalID, "init", r, name, attrs)
}

func createdBy(by proposer, proposalID, cause string, r entity.Ref, name string, attrs map[string]any) eventbus.Event {
	var opts []eventbus.DeriveOption
	if by.level != "" {
		opts = append(opts, eventbus.WithAgent(eventbus.AgentRef{ID: "agent:" + by.level, Level: by.level, Blueprint: "test"}))
	}
	return eventbus.NewRoot(state.TypeCreateProposed, by.source, world, nil,
		eventbus.ActorCI, map[string]any{
			"proposal_id": proposalID,
			"entity":      map[string]any{"entity": map[string]any{"id": r.ID, "type": r.Type}, "name": name},
			"attributes":  attrs,
			"cause":       cause,
		}, opts...)
}

// proposer is who publishes a proposal of a test: a source and, for an agent,
// its level.
type proposer struct {
	source string
	level  string
}

var (
	author  = proposer{source: contracts.SourceMvctl}
	gateway = proposer{source: contracts.SourceGateway}
)

func agentOf(level string) proposer { return proposer{source: contracts.SourceSwarm, level: level} }

// wire is a value as a subscriber holds it: encoded and decoded, every number a
// float64.
func wire(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// --- refusals ---

func assertRejected(t *testing.T, ev eventbus.Event, proposalID string, reason state.Reason, entityID string) {
	t.Helper()
	if ev.Type != state.TypeRejected {
		t.Fatalf("event %s, want %s", ev.Type, state.TypeRejected)
	}
	p := ev.Path()
	if got, _ := p.GetString("proposal_id"); got != proposalID {
		t.Errorf("proposal_id %q, want %q", got, proposalID)
	}
	if got, _ := p.GetString("reason"); got != string(reason) {
		t.Errorf("reason %q, want %q", got, reason)
	}
	if got, _ := p.GetString("entity.entity.id"); got != entityID {
		t.Errorf("the refusal blames %q, want %q", got, entityID)
	}
}

// --- a bus for the context ---

// newBus is the in-process bus of the process with pauses between redeliveries
// of zero: the number of attempts is what a test of the mediator counts, not
// the time between them.
func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	reg := contracts.Default()
	topics := make([]string, 0)
	for _, topic := range reg.Topics() {
		topics = append(topics, topic.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry: reg,
		Topics:   topics,
		Backoff:  []time.Duration{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

// onTopic decodes every record of system_events of a type.
func onTopic(t *testing.T, bus *membus.Bus, typ string) []eventbus.Event {
	t.Helper()
	records, err := bus.Records(eventbus.TopicSystemEvents)
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	var out []eventbus.Event
	for _, body := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

// advancing is a condition that moves the manual clock on each time it does not
// hold yet, for a waitFor over a context whose pauses run on that clock.
func advancing(c *clock.Manual, cond func() bool) func() bool {
	return func() bool {
		if cond() {
			return true
		}
		c.Advance(time.Minute)
		return false
	}
}

// waitFor polls until cond holds. The deadline is long on purpose: it bounds a
// defect, it does not time a behaviour, and CI runs this under -race.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.After(60 * time.Second)
	for !cond() {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-clock.RealTimers{}.After(time.Millisecond).C():
		}
	}
}
