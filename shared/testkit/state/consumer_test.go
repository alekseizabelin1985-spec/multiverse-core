package state_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit/state"
)

// This file is the promise of T-017 for C-02: replacing the stub with
// internal/state must not cost a consumer a single change.
//
// C-02 is a contract of events rather than of Go signatures, so the promise is
// held here in three ways. A projection is built the way EPIC-003 and EPIC-004
// build theirs — off the bus, over the types of the registry, naming no
// package of the testkit — and is driven end to end by the stub. Every type
// the stub publishes is checked to list testkit/state among its publishers,
// which is the static check the contracts job runs (contracts.md §16 p. 7).
// And every refusal the stub can produce is checked to be one of the values
// entity.update.rejected allows, so that the matrix of the stub is a subset of
// the matrix of the real State and never a dialect of it.

// projection is a read-model of the kind C-02 tells a consumer to build: the
// hit points of everyone it has heard about, assembled from entity.created and
// entity.updated alone. It knows nothing about who published them.
//
// The subscription hands it events on a goroutine of its own while the test
// reads it, so what it holds is behind a lock, as in any consumer that answers
// questions about its projection while the bus is still feeding it.
type projection struct {
	mu      sync.Mutex
	hp      map[string]int
	refused []string
}

func newProjection() *projection { return &projection{hp: map[string]int{}} }

func (p *projection) Handle(_ context.Context, ev eventbus.Event) error {
	path := ev.Path()
	p.mu.Lock()
	defer p.mu.Unlock()
	switch ev.Type {
	case "entity.created":
		id, _ := path.GetString("entity.entity.id")
		hp, _ := path.GetInt("attributes.hp")
		p.hp[id] = hp
	case "entity.updated":
		id, _ := path.GetString("entity.entity.id")
		changed, _ := path.GetSlice("changed")
		for _, raw := range changed {
			change, _ := raw.(map[string]any)
			if change["path"] != entity.AttrHP {
				continue
			}
			// A payload off the bus is generic JSON: a number in it is a
			// float64 whichever way the publisher meant it.
			if value, ok := change["new"].(float64); ok {
				p.hp[id] = int(value)
			}
		}
	case "entity.update.rejected":
		reason, _ := path.GetString("reason")
		p.refused = append(p.refused, reason)
	}
	return nil
}

// seen is the hit points the projection holds for one entity and a copy of the
// refusals it has heard, read under the lock the subscription writes under.
func (p *projection) seen(id string) (int, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hp[id], slices.Clone(p.refused)
}

// TestAConsumerBuildsItsProjectionFromTheStub drives the projection through
// the whole cycle: a character created, a wound taken, a proposal refused.
// Nothing in the consumer names the stub, and nothing it reads is invented by
// the stub — every event went through the registry on the way out.
func TestAConsumerBuildsItsProjectionFromTheStub(t *testing.T) {
	fake, bus, _ := world(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); _ = fake.Wait() }()
	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	view := newProjection()
	done := make(chan error, 1)
	go func() {
		done <- bus.Subscribe(ctx, eventbus.TopicSystemEvents, "a-consumer", view.Handle)
	}()

	if err := bus.Publish(ctx, proposal("prop-wound", "combat", true,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -4}))); err != nil {
		t.Fatalf("publish: %v", err)
	}
	stale := int64(99)
	if err := bus.Publish(ctx, proposal("prop-stale", "combat", true,
		changeSet(playerRef(playerA), "Вася", &stale,
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1}))); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var refused []string
	waitFor(t, "the projection to catch up", func() bool {
		var hp int
		hp, refused = view.seen(playerA)
		return hp == 6 && len(refused) == 1
	})
	if refused[0] != state.ReasonVersionConflict {
		t.Errorf("the consumer saw %q, want %q", refused[0], state.ReasonVersionConflict)
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("the subscription of the consumer failed: %v", err)
	}
	if letters, err := bus.DeadLetters(); err != nil || len(letters) != 0 {
		t.Errorf("%d dead letters (err %v): nothing the stub published could be handled", len(letters), err)
	}
}

// TestTheStubIsARegisteredPublisher is the static half of the same promise and
// the check the contracts job runs: a source nobody listed is a source mvctl
// contracts check will refuse, whatever the tests here say.
func TestTheStubIsARegisteredPublisher(t *testing.T) {
	for _, typ := range []string{
		state.TypeCreated, state.TypeUpdated, state.TypeRejected, state.TypeReplayDone,
	} {
		spec, ok := contracts.Lookup(typ)
		if !ok {
			t.Errorf("%s is not in the registry", typ)
			continue
		}
		if !slices.Contains(spec.Publishers, state.Source) {
			t.Errorf("%s: publishers %v do not include %q", typ, spec.Publishers, state.Source)
		}
	}

	// The two types the stub reads, and the one it deliberately does not
	// publish (C-14: a snapshot of the stub is written to the store, never
	// announced on the bus).
	for _, typ := range []string{state.TypeCreateProposed, state.TypeUpdateProposed} {
		spec, ok := contracts.Lookup(typ)
		if !ok {
			t.Errorf("%s is not in the registry", typ)
			continue
		}
		if spec.Topic != eventbus.TopicSystemEvents {
			t.Errorf("%s is on %q, the stub subscribes to %q",
				typ, spec.Topic, eventbus.TopicSystemEvents)
		}
	}
	if spec, ok := contracts.Lookup("snapshot.created"); ok &&
		slices.Contains(spec.Publishers, state.Source) {
		t.Errorf("%q is listed as a publisher of snapshot.created: the stub does not publish it",
			state.Source)
	}
}

// TestTheRefusalMatrixIsTheContract keeps the reasons of the double the seven
// of C-02: the double is State (T-056), and its matrix is State's. A reason the
// schema does not know would be refused on publish; a reason spelled
// differently would reach a consumer that has no branch for it.
func TestTheRefusalMatrixIsTheContract(t *testing.T) {
	contractReasons := reasonsOfTheContract(t)
	stub := []string{
		state.ReasonUnknownEntity, state.ReasonVersionConflict, state.ReasonInvalidOp,
		state.ReasonDuplicateEntity, state.ReasonDeadEntity, state.ReasonLevelViolation,
		state.ReasonLawViolation,
	}
	for _, reason := range stub {
		if !slices.Contains(contractReasons, reason) {
			t.Errorf("the stub can answer %q, which C-02 does not define", reason)
		}
		// A reason has to be publishable, which is what the schema decides.
		ev := eventbus.NewRoot(state.TypeRejected, state.Source, worldID, nil,
			entity.ActorKindCI, map[string]any{"proposal_id": "p", "reason": reason})
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("reason %q is not valid in %s: %v", reason, state.TypeRejected, err)
		}
	}
	for _, reason := range contractReasons {
		if !slices.Contains(stub, reason) {
			t.Errorf("C-02 defines %q, which the double does not name", reason)
		}
	}
}

// reasonsOfTheContract are the reasons C-02 defines, read out of the schema in
// the registry rather than copied into the test: an enum the owner of the type
// extends must not leave a stale list behind here.
func reasonsOfTheContract(t *testing.T) []string {
	t.Helper()
	spec, ok := contracts.Lookup(state.TypeRejected)
	if !ok || spec.Schema == nil {
		t.Fatalf("%s has no compiled schema in the registry", state.TypeRejected)
	}
	reason, ok := spec.Schema.Properties["reason"]
	if !ok || reason.Enum == nil {
		t.Fatalf("%s: the schema puts no enum on reason", state.TypeRejected)
	}
	out := make([]string, 0, len(reason.Enum.Values))
	for _, value := range reason.Enum.Values {
		name, ok := value.(string)
		if !ok {
			t.Fatalf("the enum of reason holds %v, which is not a string", value)
		}
		out = append(out, name)
	}
	return out
}
