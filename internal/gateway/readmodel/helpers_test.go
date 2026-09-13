package readmodel_test

import (
	"encoding/json"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

const world = "dark-forest-world"

var t0 = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func newModel(t *testing.T) (*readmodel.Model, *clock.Manual) {
	t.Helper()
	manual := clock.NewManual(t0)
	return readmodel.New(readmodel.Config{Timers: manual.Timers()}), manual
}

// event builds an envelope as the wire delivers it: the payload goes through
// JSON, so that numbers are float64 and lists []any, exactly what a consumer
// of the bus holds.
func event(t *testing.T, id, typ, correlation string, payload map[string]any) eventbus.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return eventbus.Event{
		ID: id, Type: typ, Timestamp: t0, Source: "core/state",
		World:   &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: world, Type: entity.TypeWorld}},
		Meta:    eventbus.Meta{SchemaVersion: 1, CorrelationID: correlation, ActorKind: eventbus.ActorHuman, Locale: "ru", GMPath: "agent"},
		Payload: decoded,
	}
}

func ref(id, typ string) map[string]any {
	return map[string]any{"entity": map[string]any{"id": id, "type": typ}, "name": id}
}

func created(t *testing.T, id, typ, correlation string, attrs map[string]any) eventbus.Event {
	t.Helper()
	return event(t, "created-"+id, readmodel.TypeEntityCreated, correlation, map[string]any{
		"entity": ref(id, typ), "version": 1, "attributes": attrs, "proposal_id": correlation,
	})
}

// updated builds entity.updated with changed[] written as given: a change
// without the key "new" is the v1.6 form of a path that is gone.
func updated(t *testing.T, eventID, id, typ string, version int64, correlation string, changed ...map[string]any) eventbus.Event {
	t.Helper()
	if changed == nil {
		changed = []map[string]any{}
	}
	return event(t, eventID, readmodel.TypeEntityUpdated, correlation, map[string]any{
		"entity": ref(id, typ), "version": version, "changed": changed, "cause": "combat",
		"proposal_id": correlation, "applied_at": t0.Format(time.RFC3339),
	})
}

func set(path string, old, value any) map[string]any {
	return map[string]any{"path": path, "old": old, "new": value}
}

func playerAttrs(id string) map[string]any {
	return map[string]any{
		"hp": 10, "hp_max": 10, "atk": 2, "def": 12, "dmg": "d6", "flee": "2",
		"status": "alive", "position": "outside:" + world,
		"scope":      map[string]any{"id": id, "type": "solo"},
		"actor_kind": "ci", "inventory": []any{},
	}
}

func mustApply(t *testing.T, m *readmodel.Model, ev eventbus.Event) readmodel.Result {
	t.Helper()
	res, err := m.Apply(ev)
	if err != nil {
		t.Fatalf("Apply %s %s: %v", ev.Type, ev.ID, err)
	}
	return res
}
