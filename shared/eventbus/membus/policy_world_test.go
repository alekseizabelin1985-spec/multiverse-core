package membus_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
)

// worldRequired is the registry of the platform with one type given the rule
// C-01 v1.10 adds. No row of the registry carries it yet — the first ones are
// the proposals of State, set by their owner (T-056) — so the test sets it on
// a type of its own choosing without touching the table.
type worldRequired struct {
	*contracts.Registry
	eventType string
}

func (r worldRequired) Lookup(eventType string) (eventbus.TypeSpec, bool) {
	spec, ok := r.Registry.Lookup(eventType)
	if ok && eventType == r.eventType {
		spec.Policy.World = eventbus.WorldRequired
	}
	return spec, ok
}

func newWorldBus(t *testing.T) *membus.Bus {
	t.Helper()
	testkit.Deterministic(t, t.Name())
	eventbus.SetRegistry(contracts.Default())
	bus, err := membus.New(membus.Config{
		Registry: worldRequired{Registry: contracts.Default(), eventType: "player.looked"},
		Topics:   platformTopics(),
		Backoff:  noPause,
	})
	if err != nil {
		t.Fatalf("new membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func TestPublishRefusesAnEventWithoutTheWorldItsTypeRequires(t *testing.T) {
	bus := newWorldBus(t)

	if err := bus.Publish(t.Context(), looked("world-1", "with a world")); err != nil {
		t.Fatalf("publish with a world = %v, want nil", err)
	}

	noWorld := looked("world-1", "without a world")
	noWorld.World = nil
	if err := bus.Publish(t.Context(), noWorld); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("publish without a world = %v, want ErrPolicyViolation", err)
	}

	emptyID := looked("world-1", "with an empty world id")
	emptyID.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{Type: "world"}}
	if err := bus.Publish(t.Context(), emptyID); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("publish with an empty world.entity.id = %v, want ErrPolicyViolation", err)
	}

	end, err := bus.End(t.Context(), eventbus.TopicPlayerEvents)
	if err != nil || end != 1 {
		t.Errorf("End(player_events) = (%d, %v), want (1, nil): only the event with a world was written", end, err)
	}
}

// On read the rule parks the event before the handler, like the rest of the
// policy: attempts 0.
func TestReadParksAnEventWithoutTheWorldItsTypeRequires(t *testing.T) {
	bus := newWorldBus(t)

	noWorld := looked("world-1", "without a world")
	noWorld.World = nil
	body, err := json.Marshal(noWorld)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := bus.Append(eventbus.TopicPlayerEvents, body); err != nil {
		t.Fatalf("append: %v", err)
	}

	var calls atomic.Int64
	next, err := bus.ReadRange(t.Context(), eventbus.TopicPlayerEvents, 0, 1, func(context.Context, eventbus.Event) error {
		calls.Add(1)
		return nil
	})
	if err != nil || next != 1 {
		t.Fatalf("ReadRange = (%d, %v), want (1, nil)", next, err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("the handler was called %d times for an event the world rule rejects, want 0", got)
	}
	letters, err := bus.DeadLetters()
	if err != nil {
		t.Fatalf("dead letters: %v", err)
	}
	if len(letters) != 1 || letters[0].Original.ID != noWorld.ID || letters[0].Attempts != 0 {
		t.Fatalf("dead letters = %+v, want the event without a world with attempts 0", letters)
	}

	// Validation on read switched off lets it through: the rule is part of the
	// policy, not a check of its own.
	var lenientCalls atomic.Int64
	if _, err := bus.Lenient().ReadRange(t.Context(), eventbus.TopicPlayerEvents, 0, 1, func(context.Context, eventbus.Event) error {
		lenientCalls.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("lenient ReadRange: %v", err)
	}
	if got := lenientCalls.Load(); got != 1 {
		t.Errorf("with validation on read off the handler was called %d times, want 1", got)
	}
}
