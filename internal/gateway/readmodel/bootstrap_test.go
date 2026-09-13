package readmodel_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
)

// fixtureDir is the snapshot of State the fixtures of the world carry
// (EPIC-001 F-10; C-14 v1.1).
var fixtureDir = filepath.Join("..", "..", "..", "testdata", "fixtures", "snapshots", "state")

// storeWithFixture puts the pointer and the object of the fixture where State
// writes them, and returns the pointer as it reads.
func storeWithFixture(t *testing.T, edit func(object []byte) []byte) (*objstore.Memory, map[string]any) {
	t.Helper()
	ctx := context.Background()
	store := objstore.NewMemory()
	bucket := objstore.SnapshotsBucket(world)
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatal(err)
	}
	pointer, err := os.ReadFile(filepath.Join(fixtureDir, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(pointer, &parsed); err != nil {
		t.Fatal(err)
	}
	key := parsed["snapshot"].(map[string]any)["key"].(string)
	object, err := os.ReadFile(filepath.Join(fixtureDir, strings.TrimPrefix(key, "state/")))
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		object = edit(object)
	}
	for k, body := range map[string][]byte{readmodel.StatePointerKey: pointer, key: object} {
		if _, err := store.Put(ctx, bucket, k, body, objstore.PutOptions{ContentType: "application/json"}); err != nil {
			t.Fatal(err)
		}
	}
	return store, parsed
}

func TestTheProjectionLoadsFromTheSnapshotOfState(t *testing.T) {
	store, pointer := storeWithFixture(t, nil)
	m, _ := newModel(t)
	cursor, err := m.LoadFromStateSnapshot(context.Background(), store, world)
	if err != nil {
		t.Fatalf("LoadFromStateSnapshot: %v", err)
	}
	if offset, ok := cursor["system_events"]; !ok || offset != 0 {
		t.Errorf("cursor = %v, want system_events 0 from the pointer", cursor)
	}
	if p, reason := m.Status(); p != readmodel.ProjectionOK || reason != "" {
		t.Errorf("Status = %q %q, want ok", p, reason)
	}
	if got, want := m.Hash(), pointer["snapshot"].(map[string]any)["state_hash"]; got != want {
		t.Errorf("Hash = %s, the snapshot says %v", got, want)
	}
	c, ok := m.Character("player-A")
	if !ok || c.Name != "Вася" || c.HP != 10 || c.Position != "outside:"+world || c.Scope.Type != "solo" || c.ActorKind != "ci" {
		t.Errorf("player-A = %+v %v", c, ok)
	}
	if n, ok := m.NPC("wolf-alpha"); !ok || n.RegionID != "dark-forest-01" || n.HP != 10 {
		t.Errorf("wolf-alpha = %+v %v", n, ok)
	}
	if w := m.Worlds(); len(w) != 1 || w[0].LawsVersion != "v1" {
		t.Errorf("Worlds = %+v", w)
	}
	if got := m.Cursor()["system_events"]; got != 0 {
		t.Errorf("Cursor after the load = %d, want the cursor of the snapshot", got)
	}

	// The facts after the snapshot move the projection on from there.
	mustApply(t, m, updated(t, "u-2", "player-A", entity.TypePlayer, 2, "c", set("hp", 10, 6)))
	if c, _ := m.Character("player-A"); c.HP != 6 {
		t.Errorf("hp after a fact = %d, want 6", c.HP)
	}
	if p, _ := m.Status(); p != readmodel.ProjectionOK {
		t.Errorf("projection %q after the next version, want ok", p)
	}
}

// No snapshot yet is the start of a new world: the projection stays empty,
// says it is missing, and says so without an error reason (US-011).
func TestWithoutASnapshotTheProjectionIsMissing(t *testing.T) {
	for name, store := range map[string]objstore.Client{
		"no bucket": objstore.NewMemory(),
		"no store":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := newModel(t)
			cursor, err := m.LoadFromStateSnapshot(context.Background(), store, world)
			if !errors.Is(err, readmodel.ErrNoSnapshot) || cursor != nil {
				t.Errorf("LoadFromStateSnapshot = %v %v, want ErrNoSnapshot", cursor, err)
			}
			if p, reason := m.Status(); p != readmodel.ProjectionMissing || reason != "" {
				t.Errorf("Status = %q %q, want missing without a reason", p, reason)
			}
			if len(m.Worlds()) != 0 {
				t.Error("the projection is not empty")
			}
		})
	}
}

// A snapshot that exists but does not check is not loaded: the projection
// stays empty and missing, and the reason is reported.
func TestASnapshotThatDoesNotCheckIsNotLoaded(t *testing.T) {
	cases := map[string]struct {
		edit   func([]byte) []byte
		reason string
	}{
		"entities do not hash to the pointer": {func(b []byte) []byte {
			return []byte(strings.Replace(string(b), `"hp": 10`, `"hp": 9`, 1))
		}, readmodel.ReasonHashMismatch},
		"another world": {func(b []byte) []byte {
			return []byte(strings.Replace(string(b), `"world_id": "dark-forest-world",
  "entities"`, `"world_id": "another-world",
  "entities"`, 1))
		}, readmodel.ReasonWorldMismatch},
		"not json": {func([]byte) []byte { return []byte("{") }, readmodel.ReasonSnapshotUnreadable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store, _ := storeWithFixture(t, tc.edit)
			m, _ := newModel(t)
			_, err := m.LoadFromStateSnapshot(context.Background(), store, world)
			if err == nil || errors.Is(err, readmodel.ErrNoSnapshot) {
				t.Fatalf("LoadFromStateSnapshot = %v, want a failure that is not ErrNoSnapshot", err)
			}
			if p, reason := m.Status(); p != readmodel.ProjectionMissing || reason != tc.reason {
				t.Errorf("Status = %q %q, want missing with %s", p, reason, tc.reason)
			}
			if _, ok := m.Character("player-A"); ok {
				t.Error("an entity of the refused snapshot is in the projection")
			}
		})
	}

	t.Run("pointer without its object", func(t *testing.T) {
		store, pointer := storeWithFixture(t, nil)
		key := pointer["snapshot"].(map[string]any)["key"].(string)
		if err := store.Delete(context.Background(), objstore.SnapshotsBucket(world), key); err != nil {
			t.Fatal(err)
		}
		m, _ := newModel(t)
		if _, err := m.LoadFromStateSnapshot(context.Background(), store, world); err == nil || errors.Is(err, readmodel.ErrNoSnapshot) {
			t.Errorf("LoadFromStateSnapshot = %v, want damage rather than a new world", err)
		}
		if _, reason := m.Status(); reason != readmodel.ReasonSnapshotUnreadable {
			t.Errorf("reason %q, want %s", reason, readmodel.ReasonSnapshotUnreadable)
		}
	})

	t.Run("past its deadline", func(t *testing.T) {
		store, _ := storeWithFixture(t, nil)
		m, _ := newModel(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if _, err := m.LoadFromStateSnapshot(ctx, silentStore{store}, world); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("LoadFromStateSnapshot = %v, want the deadline", err)
		}
		if p, reason := m.Status(); p != readmodel.ProjectionMissing || reason != readmodel.ReasonSnapshotTimeout {
			t.Errorf("Status = %q %q, want missing with %s", p, reason, readmodel.ReasonSnapshotTimeout)
		}
	})

	t.Run("store configured wrong", func(t *testing.T) {
		m, _ := newModel(t)
		m.MarkLoadFailed(readmodel.ReasonStoreMisconfigured)
		if p, reason := m.Status(); p != readmodel.ProjectionMissing || reason != readmodel.ReasonStoreMisconfigured {
			t.Errorf("Status = %q %q, want missing with %s", p, reason, readmodel.ReasonStoreMisconfigured)
		}
	})
}

// silentStore answers no read before the deadline of its context.
type silentStore struct{ objstore.Client }

func (silentStore) Get(ctx context.Context, _, _ string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
