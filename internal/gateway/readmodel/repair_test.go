package readmodel_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

// countingStore counts the reads of the store.
type countingStore struct {
	objstore.Client
	gets atomic.Int32
}

func (c *countingStore) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	c.gets.Add(1)
	return c.Client.Get(ctx, bucket, key)
}

// fixtureEntities are the entities of the snapshot of State of the fixtures.
func fixtureEntities(t *testing.T) []*entity.Entity {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "20260101T000000Z-000000.json"))
	if err != nil {
		t.Fatal(err)
	}
	var object struct {
		Entities []*entity.Entity `json:"entities"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	return object.Entities
}

// laterKey is the object of a later snapshot of State; latest.json still names
// the fixture, so a repair that read the pointer would find the old versions.
const laterKey = "state/20260913T120000Z-000007.json"

// putStateObject writes a snapshot object of State with entities at key and
// returns its state_hash.
func putStateObject(t *testing.T, store objstore.Client, key string, entities []*entity.Entity) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"schema_version": 1, "component": "state", "world_id": world, "entities": entities})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), objstore.SnapshotsBucket(world), key, body, objstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	return entity.StateHash(entities)
}

func snapshotCreated(t *testing.T, component, key, hash string) eventbus.Event {
	t.Helper()
	return event(t, "snap-"+component, readmodel.TypeSnapshotCreated, "snap", map[string]any{
		"component": component,
		"snapshot": map[string]any{"id": component + ":" + world + ":000007", "seq": 7, "taken_at": t0.Format("2006-01-02T15:04:05Z"),
			"cursor": map[string]any{"system_events": 9}, "laws_version": "v1", "state_hash": hash, "size_bytes": 1, "key": key},
	})
}

// staleFixture is the projection of the fixtures with player-A stale: a fact
// of version 3 came after version 1.
type staleFixture struct {
	model *readmodel.Model
	store *countingStore
}

func newStaleFixture(t *testing.T) staleFixture {
	t.Helper()
	store, _ := storeWithFixture(t, nil)
	m, _ := newModel(t)
	if _, err := m.LoadFromStateSnapshot(context.Background(), store, world); err != nil {
		t.Fatal(err)
	}
	mustApply(t, m, updated(t, "u-3", "player-A", entity.TypePlayer, 3, "c3", set("hp", 10, 7)))
	if p, _ := m.Status(); p != readmodel.ProjectionStale || !m.HasStale() {
		t.Fatalf("control: projection %s after a gap of versions, want stale", p)
	}
	return staleFixture{model: m, store: &countingStore{Client: store}}
}

// laterEntities are the fixtures with player-A at version with hp.
func laterEntities(t *testing.T, version int64, hp int) []*entity.Entity {
	t.Helper()
	entities := fixtureEntities(t)
	for _, e := range entities {
		if e.ID == "player-A" {
			e.Version = version
			e.Attributes["hp"] = float64(hp)
		}
	}
	return entities
}

// (а, з) A snapshot of State holding the stale entity at a version not below
// the projection replaces it and the projection is ok again. Only stale
// entities are taken: one that is not stale and is ahead of the snapshot is not
// rolled back, and one the snapshot holds at a later version is left to its
// facts.
func TestASnapshotOfStateRepairsTheStaleEntity(t *testing.T) {
	for _, version := range []int64{3, 5} {
		f := newStaleFixture(t)
		mustApply(t, f.model, updated(t, "wolf-2", "wolf-alpha", entity.TypeNPC, 2, "w2", set("hp", 10, 6)))
		entities := laterEntities(t, version, 4)
		for _, e := range entities {
			if e.ID == "player-B" {
				e.Version, e.Attributes["hp"] = 2, float64(1)
			}
		}
		hash := putStateObject(t, f.store, laterKey, entities)
		repaired, left, err := f.model.RepairFromStateSnapshot(context.Background(), f.store, world, snapshotCreated(t, "state", laterKey, hash))
		if err != nil || repaired != 1 || left != 0 {
			t.Fatalf("version %d: Repair = %d %d %v, want 1 repaired, none left", version, repaired, left, err)
		}
		if p, _ := f.model.Status(); p != readmodel.ProjectionOK || f.model.HasStale() {
			t.Errorf("version %d: projection %s after the repair, want ok", version, p)
		}
		if c, _ := f.model.Character("player-A"); c.HP != 4 || c.Version != version {
			t.Errorf("version %d: player-A = hp %d v%d, want the copy of the snapshot", version, c.HP, c.Version)
		}
		if n, _ := f.model.NPC("wolf-alpha"); n.HP != 6 || n.Version != 2 {
			t.Errorf("version %d: wolf-alpha = hp %d v%d, the entity that is not stale was rolled back", version, n.HP, n.Version)
		}
		if c, _ := f.model.Character("player-B"); c.HP != 10 || c.Version != 1 {
			t.Errorf("version %d: player-B = hp %d v%d, an entity that is not stale was taken from the snapshot", version, c.HP, c.Version)
		}
	}
}

// (б, в) A snapshot that holds the stale entity at a lower version, or not at
// all, leaves it stale.
func TestAStaleEntityBehindOrAbsentInTheSnapshotStaysStale(t *testing.T) {
	cases := map[string][]*entity.Entity{
		"lower version": laterEntities(t, 2, 4),
		"absent": func() []*entity.Entity {
			var out []*entity.Entity
			for _, e := range fixtureEntities(t) {
				if e.ID != "player-A" {
					out = append(out, e)
				}
			}
			return out
		}(),
	}
	for name, entities := range cases {
		t.Run(name, func(t *testing.T) {
			f := newStaleFixture(t)
			hash := putStateObject(t, f.store, laterKey, entities)
			repaired, left, err := f.model.RepairFromStateSnapshot(context.Background(), f.store, world, snapshotCreated(t, "state", laterKey, hash))
			if err != nil || repaired != 0 || left != 1 {
				t.Errorf("Repair = %d %d %v, want nothing repaired and one left", repaired, left, err)
			}
			if c, _ := f.model.Character("player-A"); c.HP != 7 || c.Version != 3 {
				t.Errorf("player-A = hp %d v%d, want the projection untouched", c.HP, c.Version)
			}
			if p, _ := f.model.Status(); p != readmodel.ProjectionStale {
				t.Errorf("projection %s, want stale", p)
			}
		})
	}
}

// (г, д) The store is read only for a snapshot of State of the world of the
// context, and only while something is stale.
func TestTheStoreIsReadOnlyForASnapshotOfStateOfTheWorldWhileStale(t *testing.T) {
	f := newStaleFixture(t)
	hash := putStateObject(t, f.store, laterKey, laterEntities(t, 3, 4))
	foreign := snapshotCreated(t, "state", laterKey, hash)
	foreign.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: "another-world", Type: entity.TypeWorld}}
	other := updated(t, "u-9", "wolf-alpha", entity.TypeNPC, 2, "w", set("hp", 10, 9))
	for name, ev := range map[string]eventbus.Event{
		"gateway":        snapshotCreated(t, "gateway", laterKey, hash),
		"swarm":          snapshotCreated(t, "swarm", laterKey, hash),
		"another world":  foreign,
		"not a snapshot": other,
	} {
		if _, left, err := f.model.RepairFromStateSnapshot(context.Background(), f.store, world, ev); err != nil || left != 1 {
			t.Errorf("%s: left %d, %v", name, left, err)
		}
	}
	if _, _, err := f.model.RepairFromStateSnapshot(context.Background(), nil, world, snapshotCreated(t, "state", laterKey, hash)); err != nil {
		t.Errorf("without a store: %v", err)
	}
	if n := f.store.gets.Load(); n != 0 {
		t.Errorf("the store was read %d times for snapshots it must not repair from", n)
	}

	clean, _ := newModel(t)
	if _, _, err := clean.RepairFromStateSnapshot(context.Background(), f.store, world, snapshotCreated(t, "state", laterKey, hash)); err != nil {
		t.Fatal(err)
	}
	if n := f.store.gets.Load(); n != 0 {
		t.Errorf("a projection without stale entities read the store %d times", n)
	}
}

// (е) A snapshot that does not check — another hash, the object gone — is a
// RepairError with its code and leaves the stale entity as it was.
func TestASnapshotThatDoesNotCheckRepairsNothing(t *testing.T) {
	cases := map[string]struct {
		prepare func(t *testing.T, f staleFixture) eventbus.Event
		reason  string
	}{
		"another hash": {func(t *testing.T, f staleFixture) eventbus.Event {
			putStateObject(t, f.store, laterKey, laterEntities(t, 3, 4))
			return snapshotCreated(t, "state", laterKey, entity.StateHash(nil))
		}, readmodel.ReasonHashMismatch},
		"object gone": {func(t *testing.T, _ staleFixture) eventbus.Event {
			return snapshotCreated(t, "state", laterKey, entity.StateHash(nil))
		}, readmodel.ReasonSnapshotUnreadable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newStaleFixture(t)
			_, left, err := f.model.RepairFromStateSnapshot(context.Background(), f.store, world, tc.prepare(t, f))
			var repairErr *readmodel.RepairError
			if !errors.As(err, &repairErr) || repairErr.Reason != tc.reason || left != 1 {
				t.Errorf("Repair = left %d, %v; want a RepairError %s and the entity left", left, err, tc.reason)
			}
			if c, _ := f.model.Character("player-A"); c.HP != 7 {
				t.Errorf("player-A hp %d, want the projection untouched", c.HP)
			}
		})
	}
}

// (ж) After the repair the rule of versions goes on from the copy: a fact of
// the next version applies without making anything stale, an older one does
// not apply.
func TestFactsAfterTheRepairFollowTheCopy(t *testing.T) {
	f := newStaleFixture(t)
	hash := putStateObject(t, f.store, laterKey, laterEntities(t, 4, 4))
	if _, left, err := f.model.RepairFromStateSnapshot(context.Background(), f.store, world, snapshotCreated(t, "state", laterKey, hash)); err != nil || left != 0 {
		t.Fatalf("Repair: left %d, %v", left, err)
	}
	if res := mustApply(t, f.model, updated(t, "u-4", "player-A", entity.TypePlayer, 4, "c4", set("hp", 4, 1))); res.Applied {
		t.Error("a fact of the version of the copy was applied again")
	}
	if res := mustApply(t, f.model, updated(t, "u-5", "player-A", entity.TypePlayer, 5, "c5", set("hp", 4, 2))); !res.Applied {
		t.Error("the fact after the copy was not applied")
	}
	if c, _ := f.model.Character("player-A"); c.HP != 2 || c.Version != 5 {
		t.Errorf("player-A = hp %d v%d, want 2 v5", c.HP, c.Version)
	}
	if p, _ := f.model.Status(); p != readmodel.ProjectionOK {
		t.Errorf("projection %s after the next fact, want ok", p)
	}
}
