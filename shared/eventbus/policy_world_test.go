package eventbus

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWorldRequiredRejectsAnEnvelopeWithoutAWorld(t *testing.T) {
	deterministicSources(t)
	required := Policy{World: WorldRequired}

	withWorld := NewRoot("entity.update.proposed", "core/mechanics", "world-1", nil, ActorSystem, map[string]any{})
	if err := required.Check(withWorld); err != nil {
		t.Errorf("Check of an envelope with a world = %v, want nil", err)
	}

	noWorld := withWorld
	noWorld.World = nil
	if err := required.Check(noWorld); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("Check without a world = %v, want ErrPolicyViolation", err)
	}

	emptyID := withWorld
	emptyID.World = &WorldRef{Entity: EntityRef{Type: "world"}}
	if err := required.Check(emptyID); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("Check with an empty world.entity.id = %v, want ErrPolicyViolation", err)
	}

	// The zero value keeps the behaviour every policy had before the rule.
	if err := (Policy{}).Check(noWorld); err != nil {
		t.Errorf("Check of WorldOptional without a world = %v, want nil", err)
	}
	if WorldOptional != 0 {
		t.Error("WorldOptional is not the zero value of WorldRule")
	}
}

func worldRegistry() fakeRegistry {
	reg := testRegistry()
	reg.specs["entity.update.proposed"] = TypeSpec{Topic: TopicSystemEvents, SchemaVersion: 1, Policy: Policy{World: WorldRequired}}
	return reg
}

func TestRouteRefusesAnEventTheWorldRuleRejects(t *testing.T) {
	deterministicSources(t)
	ev := NewRoot("entity.update.proposed", "core/mechanics", "world-1", nil, ActorSystem, map[string]any{})

	if topic, err := Route(worldRegistry(), ev); err != nil || topic != TopicSystemEvents {
		t.Errorf("Route with a world = (%q, %v), want (%q, nil)", topic, err, TopicSystemEvents)
	}
	ev.World = nil
	if _, err := Route(worldRegistry(), ev); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("Route without a world = %v, want ErrPolicyViolation", err)
	}
}

func TestDeliverParksAnEventTheWorldRuleRejectsWithoutCallingTheHandler(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	d := testDelivery(sink)
	d.Registry = worldRegistry()
	ev := NewRoot("entity.update.proposed", "core/mechanics", "world-1", nil, ActorSystem, map[string]any{})
	ev.World = &WorldRef{Entity: EntityRef{ID: "", Type: "world"}}

	called := false
	if err := d.Deliver(t.Context(), Position{Topic: TopicSystemEvents}, ev,
		func(context.Context, Event) error { called = true; return nil }); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	if called {
		t.Error("the handler was called for an event the world rule rejects")
	}
	letters := sink.all()
	if len(letters) != 1 || letters[0].Attempts != 0 {
		t.Fatalf("dead letters = %+v, want one with attempts 0", letters)
	}
	if want := "world.entity.id is empty"; !strings.Contains(letters[0].Error, want) {
		t.Errorf("dead letter says %q, want it to name %q", letters[0].Error, want)
	}
}
