package readmodel_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

func TestApplyBuildsTheCharacterFromItsFacts(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, created(t, "player-A", entity.TypePlayer, "create-A", playerAttrs("player-A")))

	c, ok := m.Character("player-A")
	if !ok || c.HP != 10 || c.HPMax != 10 || c.Status != entity.StatusAlive || c.Position != "outside:"+world ||
		c.Scope != (eventbus.ScopeRef{ID: "player-A", Type: "solo"}) || len(c.Inventory) != 0 || c.Version != 1 || c.WorldID != world {
		t.Fatalf("after entity.created: %+v %v", c, ok)
	}

	pelt := map[string]any{"item_id": "pelt-1", "kind": "wolf-pelt", "name": "волчья шкура",
		"source":      map[string]any{"entity": map[string]any{"id": "wolf-alpha", "type": "npc"}, "event_id": "cd-1"},
		"acquired_at": "2026-09-13T12:00:00Z"}
	mustApply(t, m, updated(t, "u-2", "player-A", entity.TypePlayer, 2, "move-1",
		set("position", "outside:"+world, "dark-forest-01")))
	mustApply(t, m, updated(t, "u-3", "player-A", entity.TypePlayer, 3, "attack-1",
		set("hp", 10, 4), set("encounter_id", nil, "encounter-1")))
	// The element of an append in the form of C-02 v1.5: old is null.
	mustApply(t, m, updated(t, "u-4", "player-A", entity.TypePlayer, 4, "loot-1",
		set("inventory[0]", nil, pelt)))
	mustApply(t, m, updated(t, "u-5", "player-A", entity.TypePlayer, 5, "attack-2",
		set("hp", 4, 0), set("status", "alive", "dead")))

	c, _ = m.Character("player-A")
	if c.HP != 0 || c.Status != entity.StatusDead || c.Position != "dark-forest-01" || c.EncounterID != "encounter-1" || c.Version != 5 {
		t.Errorf("after the facts: %+v", c)
	}
	if len(c.Inventory) != 1 || c.Inventory[0].ItemID != "pelt-1" || c.Inventory[0].Source.Entity.ID != "wolf-alpha" {
		t.Errorf("inventory = %+v, want the pelt", c.Inventory)
	}
	if _, ok := m.NPC("player-A"); ok {
		t.Error("a player is returned as an NPC")
	}
}

// The projection applies changed[] by the rule of catching up and ends where
// State ended. In the form of C-02 v1.6 (T-448) the hash of the entity after
// every fact is the hash of the entity State committed. The form State writes
// today (C-02 v1.5) reaches the same hash on every step but the removal of a
// key: it travels as new: null, the same as a set to null, and the projection
// keeps the null State deleted. That step is checked for exactly this
// divergence — the hash of a projection built from v1.5 facts is not the hash
// of State once a key was removed — and the getters still read the key as
// absent. The removal is the last step, so the divergence carries no further.
func TestChangedOfBothFormsReproducesTheStateOfState(t *testing.T) {
	fang := map[string]any{"item_id": "fang-1", "kind": "wolf-fang", "name": "клык"}
	pelt := map[string]any{"item_id": "pelt-1", "kind": "wolf-pelt", "name": "шкура"}
	steps := []entity.Op{
		{Op: entity.OpInc, Path: "hp", Value: -3},
		{Op: entity.OpSet, Path: "position", Value: "dark-forest-01"},
		{Op: entity.OpAppend, Path: "inventory", Value: pelt},
		{Op: entity.OpAppend, Path: "inventory", Value: fang},
		{Op: entity.OpRemove, Path: "inventory", Value: map[string]any{"item_id": "pelt-1"}},
		{Op: entity.OpSet, Path: "stats.deep.x", Value: 1},
		// A text replaced by an object. position is a text of data-model.md
		// §3.3, and since T-472 no path goes below it; banner is untyped.
		{Op: entity.OpSet, Path: "banner", Value: "green"},
		{Op: entity.OpSet, Path: "banner.region", Value: "dark-forest-01"},
		{Op: entity.OpSet, Path: "encounter_id", Value: nil},
		{Op: entity.OpAppend, Path: "inventory", Value: pelt},
		{Op: entity.OpRemove, Path: "inventory[0]"},
		{Op: entity.OpRemove, Path: "encounter_id"},
	}
	for _, form := range []string{"v1.5", "v1.6"} {
		t.Run(form, func(t *testing.T) {
			m, _ := newModel(t)
			truth := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, world, "player-A", playerAttrs("player-A"), t0)
			mustApply(t, m, created(t, "player-A", entity.TypePlayer, "create-A", playerAttrs("player-A")))
			for i, op := range steps {
				before := entity.Clone(truth)
				attrs, changed, err := entity.ApplyOps(truth, []entity.Op{op})
				if err != nil {
					t.Fatalf("step %d: %v", i, err)
				}
				truth.Commit(attrs, changed, entity.LastChange{ProposalID: fmt.Sprint(i), AppliedAt: t0})
				wire := make([]map[string]any, 0, len(changed))
				for _, c := range changed {
					if form == "v1.5" {
						wire = append(wire, map[string]any{"path": c.Path, "old": c.Old, "new": c.New})
						continue
					}
					element := map[string]any{"path": c.Path}
					if before.HasAttr(c.Path) {
						element["old"] = c.Old
					}
					if truth.HasAttr(c.Path) {
						element["new"] = c.New
					}
					wire = append(wire, element)
				}
				mustApply(t, m, updated(t, fmt.Sprintf("u-%d", i), "player-A", entity.TypePlayer, truth.Version, "c", wire...))
				got, state := m.Hash(), entity.StateHash([]*entity.Entity{truth})
				if form == "v1.5" && removesKey(op) {
					if want := entity.StateHash([]*entity.Entity{withNull(t, truth, op.Path)}); got != want || got == state {
						t.Fatalf("step %d (%s %s): projection hash %s, want %s — the key kept as null, not %s of State",
							i, op.Op, op.Path, got, want, state)
					}
					if c, _ := m.Character("player-A"); c.EncounterID != "" {
						t.Errorf("step %d: the null left by v1.5 reads as encounter_id %q", i, c.EncounterID)
					}
					continue
				}
				if got != state {
					t.Fatalf("step %d (%s %s): projection hash %s, State %s", i, op.Op, op.Path, got, state)
				}
			}
			if v, _ := m.Version("player-A"); v != truth.Version {
				t.Errorf("version %d, State has %d", v, truth.Version)
			}
			if p, _ := m.Status(); p != readmodel.ProjectionMissing {
				t.Errorf("projection %q: consecutive versions made it stale", p)
			}
		})
	}
}

// removesKey says whether an operation deletes a key of a map rather than an
// element of a list, which v1.5 writes as a whole list.
func removesKey(op entity.Op) bool {
	return op.Op == entity.OpRemove && op.Value == nil && !strings.HasSuffix(op.Path, "]")
}

// withNull is e with path set to null, as State would have it after a set to
// null: what a projection built from v1.5 facts holds after a removal.
func withNull(t *testing.T, e *entity.Entity, path string) *entity.Entity {
	t.Helper()
	out := entity.Clone(e)
	attrs, _, err := entity.ApplyOps(out, []entity.Op{{Op: entity.OpSet, Path: path, Value: nil}})
	if err != nil {
		t.Fatal(err)
	}
	out.Attributes = attrs
	return out
}

// Under C-02 v1.5 the removal of a key and a set to null are one and the same
// element, new: null, so the projection keeps the key as null: it hashes as
// State after a set to null, not as State after the removal, and the getter
// reads the null as absent. Under v1.6 the element has no new, the key goes,
// and the projection hashes as State after the removal.
func TestARemovedKeyInEitherForm(t *testing.T) {
	stateAfter := func(op entity.Op) string {
		attrs := playerAttrs("player-A")
		attrs["encounter_id"] = "encounter-1"
		e := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, world, "", attrs, t0)
		next, _, err := entity.ApplyOps(e, []entity.Op{op})
		if err != nil {
			t.Fatal(err)
		}
		e.Attributes, e.Version = next, 2
		return entity.StateHash([]*entity.Entity{e})
	}
	removed := stateAfter(entity.Op{Op: entity.OpRemove, Path: "encounter_id"})
	setNull := stateAfter(entity.Op{Op: entity.OpSet, Path: "encounter_id", Value: nil})
	if removed == setNull {
		t.Fatal("State hashes a removed key and a null key alike: the divergence below cannot show")
	}
	for _, tc := range []struct {
		form    string
		element map[string]any
		want    string
	}{
		{"v1.5", map[string]any{"path": "encounter_id", "old": "encounter-1", "new": nil}, setNull},
		{"v1.6", map[string]any{"path": "encounter_id", "old": "encounter-1"}, removed},
	} {
		t.Run(tc.form, func(t *testing.T) {
			m, _ := newModel(t)
			attrs := playerAttrs("player-A")
			attrs["encounter_id"] = "encounter-1"
			mustApply(t, m, created(t, "player-A", entity.TypePlayer, "c", attrs))
			mustApply(t, m, updated(t, "u-2", "player-A", entity.TypePlayer, 2, "c", tc.element))

			if got := m.Hash(); got != tc.want {
				t.Errorf("projection hash %s, want %s (removed %s, set to null %s)", got, tc.want, removed, setNull)
			}
			if c, _ := m.Character("player-A"); c.EncounterID != "" {
				t.Errorf("encounter_id = %q after it was removed", c.EncounterID)
			}
		})
	}
}

// C-02 v1.6, rule of catching up: an element without new names a path that is
// gone after the change, and a path that is not there is left alone. A
// proposal that wrote fresh.deep and removed it again names the ancestor it
// created, fresh, and fresh.deep, which nothing holds any more; in either
// order the fact applies and the projection hashes as State.
func TestARemovalOfAPathThatIsNotThereChangesNothing(t *testing.T) {
	truth := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, world, "", playerAttrs("player-A"), t0)
	attrs, _, err := entity.ApplyOps(truth, []entity.Op{
		{Op: entity.OpSet, Path: "fresh.deep", Value: 1},
		{Op: entity.OpRemove, Path: "fresh.deep"},
	})
	if err != nil {
		t.Fatal(err)
	}
	truth.Attributes, truth.Version = attrs, 2
	if truth.HasAttr("fresh.deep") || !truth.HasAttr("fresh") {
		t.Fatalf("State after the proposal = %v, want fresh without deep", truth.Attributes)
	}
	gone := map[string]any{"path": "fresh.deep"}
	ancestor := map[string]any{"path": "fresh", "new": map[string]any{}}
	for name, changed := range map[string][]map[string]any{
		"ancestor first":  {ancestor, gone},
		"gone path first": {gone, ancestor},
		"under a number":  {ancestor, {"path": "hp.deep"}},
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := newModel(t)
			mustApply(t, m, created(t, "player-A", entity.TypePlayer, "c", playerAttrs("player-A")))
			mustApply(t, m, updated(t, "u-2", "player-A", entity.TypePlayer, 2, "c", changed...))
			if got, want := m.Hash(), entity.StateHash([]*entity.Entity{truth}); got != want {
				t.Errorf("projection hash %s, State %s", got, want)
			}
		})
	}
}

// A v1.6 fact names the ancestor a change created: it is written as it is.
func TestACreatedAncestorIsWritten(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, created(t, "player-A", entity.TypePlayer, "c", playerAttrs("player-A")))
	mustApply(t, m, updated(t, "u-2", "player-A", entity.TypePlayer, 2, "c",
		map[string]any{"path": "fresh", "new": map[string]any{}}))
	want := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, world, "", playerAttrs("player-A"), t0)
	want.Attributes["fresh"] = map[string]any{}
	want.Version = 2
	if got := m.Hash(); got != entity.StateHash([]*entity.Entity{want}) {
		t.Error("the created ancestor is not in the projection")
	}
}

func TestFactsApplyInTheOrderOfTheirVersions(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, created(t, "player-A", entity.TypePlayer, "c", playerAttrs("player-A")))
	two := updated(t, "u-2", "player-A", entity.TypePlayer, 2, "c", set("hp", 10, 7))
	three := updated(t, "u-3", "player-A", entity.TypePlayer, 3, "c", set("hp", 7, 5))

	if res := mustApply(t, m, two); !res.Applied {
		t.Error("the fact of version 2 was not applied")
	}
	mustApply(t, m, three)
	for _, again := range []eventbus.Event{two, three, created(t, "player-A", entity.TypePlayer, "c", playerAttrs("player-A"))} {
		if res := mustApply(t, m, again); res.Applied {
			t.Errorf("%s %s applied again", again.Type, again.ID)
		}
	}
	if c, _ := m.Character("player-A"); c.HP != 5 || c.Version != 3 {
		t.Errorf("hp %d version %d after a repeat and an old fact, want 5 and 3", c.HP, c.Version)
	}
	// A fact without changes keeps the version State had (C-02).
	if res := mustApply(t, m, updated(t, "u-3b", "player-A", entity.TypePlayer, 3, "c")); res.Applied {
		t.Error("a fact without changes at the same version changed the projection")
	}
	if p, _ := m.Status(); p != readmodel.ProjectionMissing {
		t.Errorf("projection %q, want missing: nothing was skipped", p)
	}

	// Version 4 never arrived: the fact of 5 is applied for the paths it
	// names, and the projection says it is behind.
	mustApply(t, m, updated(t, "u-5", "player-A", entity.TypePlayer, 5, "c", set("position", "outside:"+world, "dark-forest-01")))
	if c, _ := m.Character("player-A"); c.Position != "dark-forest-01" || c.Version != 5 {
		t.Errorf("after a gap: %+v", c)
	}
	if p, _ := m.Status(); p != readmodel.ProjectionStale {
		t.Errorf("projection %q after a gap of versions, want stale", p)
	}
}

func TestAFactOfAnUnknownEntityMakesTheProjectionStale(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, updated(t, "u-7", "wolf-alpha", entity.TypeNPC, 7, "c", set("hp", 10, 3)))
	n, ok := m.NPC("wolf-alpha")
	if !ok || n.HP != 3 || n.Version != 7 {
		t.Errorf("NPC = %+v %v, want what the fact said", n, ok)
	}
	if p, _ := m.Status(); p != readmodel.ProjectionStale {
		t.Errorf("projection %q, want stale", p)
	}
}

func TestACorruptFactIsAnErrorAndChangesNothing(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, created(t, "player-A", entity.TypePlayer, "c", playerAttrs("player-A")))
	for name, ev := range map[string]eventbus.Event{
		"element past the end": updated(t, "u-2", "player-A", entity.TypePlayer, 2, "c",
			set("hp", 10, 9), set("inventory[3]", nil, "x")),
		"no entity":    event(t, "u-3", readmodel.TypeEntityUpdated, "c", map[string]any{"version": 2, "changed": []any{}}),
		"no encounter": event(t, "s-1", readmodel.TypeEncounterStarted, "c", map[string]any{"region": ref("r", "region")}),
	} {
		if _, err := m.Apply(ev); !errors.Is(err, readmodel.ErrCorruptFact) {
			t.Errorf("%s: Apply = %v, want ErrCorruptFact", name, err)
		}
	}
	if c, _ := m.Character("player-A"); c.HP != 10 || c.Version != 1 {
		t.Errorf("a refused fact changed the character: %+v", c)
	}
}

func TestTheProjectionOfEachType(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, created(t, world, entity.TypeWorld, "c", map[string]any{"laws_version": "v1", "locale": "ru"}))
	mustApply(t, m, created(t, "dark-forest-01", entity.TypeRegion, "c", map[string]any{
		"description": "лес", "npc_ids": []any{"wolf-alpha"}, "players_present": []any{}}))
	mustApply(t, m, created(t, "wolf-alpha", entity.TypeNPC, "c", map[string]any{
		"kind": "wolf", "region_id": "dark-forest-01", "position": "dark-forest-01", "hp": 10, "hp_max": 10, "status": "alive"}))
	mustApply(t, m, created(t, "g-1", entity.TypeGroup, "c", map[string]any{
		"leader_id": nil, "state": "active", "position": "dark-forest-01",
		"members": []any{map[string]any{"player_id": "player-A", "joined_at": "2026-09-13T12:00:00Z", "participation": "active", "missed_rounds": 0}}}))

	if w := m.Worlds(); len(w) != 1 || w[0].ID != world || w[0].LawsVersion != "v1" || w[0].Locale != "ru" {
		t.Errorf("Worlds = %+v", w)
	}
	if r, ok := m.Region(world, "dark-forest-01"); !ok || !slices.Equal(r.NPCIDs, []string{"wolf-alpha"}) || r.Description != "лес" {
		t.Errorf("Region = %+v %v", r, ok)
	}
	if _, ok := m.Region("another-world", "dark-forest-01"); ok {
		t.Error("a region is found in a world it is not in")
	}
	if n, ok := m.NPC("wolf-alpha"); !ok || n.Kind != "wolf" || n.HP != 10 || n.RegionID != "dark-forest-01" {
		t.Errorf("NPC = %+v %v", n, ok)
	}
	g, ok := m.Group("g-1")
	if !ok || g.LeaderID != nil || len(g.Members) != 1 || g.Members[0].PlayerID != "player-A" || g.State != "active" {
		t.Errorf("Group = %+v %v, want no leader and one member", g, ok)
	}
	mustApply(t, m, updated(t, "u-g", "g-1", entity.TypeGroup, 2, "c", set("leader_id", nil, "player-A")))
	if g, _ := m.Group("g-1"); g.LeaderID == nil || *g.LeaderID != "player-A" {
		t.Errorf("leader after the fact = %v", g.LeaderID)
	}
}
