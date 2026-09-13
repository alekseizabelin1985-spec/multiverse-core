package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

// --- learning without a process (acceptance of T-454, review #1, Mi-1) ---
//
// The fights through the process pin that the stand calls ready before the
// entry, not what ready does: a ready that let the held facts through and
// returned at once passes them all. These tests pin the wait itself on the
// wrapper alone, with the handler of the fake played by the test.

// capturing is a transport that keeps the handler it was given instead of
// running a subscription, so that the test delivers the facts itself.
type capturing struct {
	transport
	h eventbus.Handler
}

func (c *capturing) Subscribe(_ context.Context, _, _ string, h eventbus.Handler) error {
	c.h = h
	return nil
}

// watchedByLearning wraps fake the way the stand wraps the subscription of the
// encounter to system_events and returns the handler the bus would call.
func watchedByLearning(t *testing.T, created []string, hold bool, fake eventbus.Handler) (*learning, eventbus.Handler) {
	t.Helper()
	c := &capturing{}
	l := newLearning(c, created, hold)
	group := swarm.EncounterGroup + "-" + eventbus.TopicSystemEvents
	if err := l.Subscribe(context.Background(), eventbus.TopicSystemEvents, group, fake); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if c.h == nil {
		t.Fatalf("learning did not subscribe the encounter group %s", group)
	}
	return l, c.h
}

func createdFact(id string) eventbus.Event {
	return eventbus.NewRoot(state.TypeCreated, state.Source, "", nil, eventbus.ActorSystem,
		map[string]any{"entity": map[string]any{"entity": map[string]any{"id": id}}})
}

func isLearnt(l *learning) bool {
	select {
	case <-l.learnt:
		return true
	default:
		return false
	}
}

// ready must not let a character in while the fake is still folding a fact:
// the second fact takes a while, as on a slow runner, and ready returns only
// once the handler of the fake has returned on both. A ready that stops
// waiting returns while the fake has folded neither.
func TestReadyReturnsOnlyOnceTheFakeHasLearntEveryEntity(t *testing.T) {
	var (
		mu      sync.Mutex
		handled []string
	)
	l, h := watchedByLearning(t, []string{"dark-forest-01", "wolf-alpha"}, true,
		func(_ context.Context, ev eventbus.Event) error {
			id, _ := ev.Path().GetString("entity.entity.id")
			if id == "wolf-alpha" {
				<-clock.RealTimers{}.After(200 * time.Millisecond).C()
			}
			mu.Lock()
			defer mu.Unlock()
			handled = append(handled, id)
			return nil
		})

	// One subscription delivers in order on a goroutine of its own.
	delivered := make(chan error, 1)
	go func() {
		for _, id := range []string{"dark-forest-01", "wolf-alpha"} {
			if err := h(context.Background(), createdFact(id)); err != nil {
				delivered <- err
				return
			}
		}
		delivered <- nil
	}()
	t.Cleanup(func() { <-delivered })

	mu.Lock()
	early := len(handled)
	mu.Unlock()
	if early != 0 || isLearnt(l) {
		t.Fatalf("the fake handled %d facts before the stand began to wait, want them held", early)
	}

	l.ready(t, &running{done: make(chan error, 1)})

	mu.Lock()
	got := append([]string(nil), handled...)
	mu.Unlock()
	if len(got) != 2 {
		t.Errorf("ready returned when the fake had handled %v, want both dark-forest-01 and wolf-alpha", got)
	}
	if missing := l.missing(); len(missing) != 0 {
		t.Errorf("ready returned with %v still unlearnt", missing)
	}
}

// An entity is learnt when the handler of the fake has returned, not when the
// fact reached it: while Observe is still running the entity is not in the view
// the entry is answered from.
func TestLearningMarksAnEntityOnlyAfterTheFakeReturned(t *testing.T) {
	inside, release := make(chan struct{}), make(chan struct{})
	l, h := watchedByLearning(t, []string{"wolf-alpha"}, false, func(context.Context, eventbus.Event) error {
		close(inside)
		<-release
		return nil
	})
	returned := make(chan error, 1)
	go func() { returned <- h(context.Background(), createdFact("wolf-alpha")) }()

	<-inside
	if isLearnt(l) {
		close(release)
		<-returned
		t.Fatal("wolf-alpha was learnt while the handler of the fake was still running")
	}
	close(release)
	if err := <-returned; err != nil {
		t.Fatalf("the watched handler returned %v, want nil", err)
	}
	if !isLearnt(l) {
		t.Errorf("the handler of the fake returned on wolf-alpha, but %v is still unlearnt", l.missing())
	}
}

// A fact the fake refused is not learnt: the bus delivers it again, and only
// the delivery the fake accepts counts. Until then ready names the refusal.
func TestAFactTheFakeRefusedIsNotLearnt(t *testing.T) {
	errFake := errors.New("the fake could not fold the wolf")
	refuse := true
	l, h := watchedByLearning(t, []string{"wolf-alpha"}, false, func(context.Context, eventbus.Event) error {
		if refuse {
			return errFake
		}
		return nil
	})

	if err := h(context.Background(), createdFact("wolf-alpha")); !errors.Is(err, errFake) {
		t.Fatalf("the watched handler returned %v, want the refusal of the fake passed to the bus", err)
	}
	if isLearnt(l) {
		t.Fatal("wolf-alpha was learnt although the fake refused it")
	}
	if got := l.unlearnt(); !strings.Contains(got, "wolf-alpha") || !strings.Contains(got, errFake.Error()) {
		t.Errorf("unlearnt() = %q, want wolf-alpha and the refusal of the fake", got)
	}

	refuse = false
	if err := h(context.Background(), createdFact("wolf-alpha")); err != nil {
		t.Fatalf("the redelivery returned %v, want nil", err)
	}
	if !isLearnt(l) {
		t.Errorf("the fake accepted the redelivery of wolf-alpha, but %v is still unlearnt", l.missing())
	}
}
