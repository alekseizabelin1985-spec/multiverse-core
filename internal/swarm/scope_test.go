package swarm_test

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"multiverse-core.io/internal/swarm"
	"multiverse-core.io/shared/eventbus"
)

const (
	worldID  = "dark-forest-world"
	regionA  = "dark-forest-01"
	regionB  = "dark-swamp-02"
	playerA  = "player-A"
	playerB  = "player-B"
	groupID  = "group-1"
	encID    = "enc-1"
	outsideW = "outside:" + worldID
)

func solo(id string) eventbus.ScopeRef   { return eventbus.ScopeRef{ID: id, Type: "solo"} }
func group(id string) eventbus.ScopeRef  { return eventbus.ScopeRef{ID: id, Type: "group"} }
func region(id string) eventbus.ScopeRef { return eventbus.ScopeRef{ID: id, Type: "region"} }

func event(typ string, scope *eventbus.ScopeRef, payload map[string]any) eventbus.Event {
	return eventbus.Event{
		ID: typ + "-" + fmt.Sprint(len(payload)), Type: typ, Source: "test",
		World: &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: worldID, Type: "world"}},
		Scope: scope, Payload: payload,
	}
}

func ref(id, typ string) map[string]any {
	return map[string]any{"entity": map[string]any{"id": id, "type": typ}}
}

func created(id, typ string, attributes map[string]any) eventbus.Event {
	return event("entity.created", nil, map[string]any{"entity": ref(id, typ), "version": 1, "attributes": attributes})
}

// updated writes changed as a slice of maps, the way a publisher in the
// process builds it; the bus decodes it as []any.
func updated(id, typ string, changes ...[2]any) eventbus.Event {
	changed := make([]map[string]any, 0, len(changes))
	for _, c := range changes {
		changed = append(changed, map[string]any{"path": c[0], "new": c[1]})
	}
	return event("entity.updated", nil, map[string]any{
		"entity": ref(id, typ), "version": 2, "changed": changed, "cause": "move", "proposal_id": "p", "applied_at": "2026-01-01T00:00:00Z",
	})
}

func groupEvent(typ string, members ...string) eventbus.Event {
	list := make([]any, 0, len(members))
	for _, m := range members {
		list = append(list, map[string]any{"entity": map[string]any{"id": m, "type": "player"}})
	}
	return event(typ, &eventbus.ScopeRef{ID: groupID, Type: "group"}, map[string]any{"group": ref(groupID, "group"), "members": list, "cause": "join"})
}

func encounterStarted(scope eventbus.ScopeRef) eventbus.Event {
	return event("encounter.started", &scope, map[string]any{
		"encounter": ref(encID, "encounter"), "region": ref(regionA, "region"),
		"participants": []any{ref(playerA, "player")}, "npcs": []any{ref("wolf-alpha", "npc")},
	})
}

func encounterEnded(scope eventbus.ScopeRef) eventbus.Event {
	return event("encounter.ended", &scope, map[string]any{"encounter": ref(encID, "encounter"), "reason": "npc_dead", "rounds": 2})
}

// view is every answer of the index about the entities of the table.
type view struct {
	RegionOfA, RegionOfB, RegionOfGroup string
	WorldOfA, WorldOfGroup, WorldOfRA   string
	GroupOfA, GroupOfB                  string
	InA, InB                            string
	EncounterOfA, EncounterOfGroup      string
	EncounterScope                      string
}

func look(x *swarm.ScopeIndex) view {
	opt := func(s string, ok bool) string {
		if !ok {
			return "-"
		}
		return s
	}
	encScope := "-"
	if s, ok := x.EncounterScope(encID); ok {
		encScope = s.Type + ":" + s.ID
	}
	return view{
		RegionOfA:        opt(x.RegionOf(solo(playerA))),
		RegionOfB:        opt(x.RegionOf(solo(playerB))),
		RegionOfGroup:    opt(x.RegionOf(group(groupID))),
		WorldOfA:         opt(x.WorldOf(solo(playerA))),
		WorldOfGroup:     opt(x.WorldOf(group(groupID))),
		WorldOfRA:        opt(x.WorldOf(region(regionA))),
		GroupOfA:         opt(x.GroupOf(playerA)),
		GroupOfB:         opt(x.GroupOf(playerB)),
		InA:              strings.Join(x.PlayersIn(regionA), ","),
		InB:              strings.Join(x.PlayersIn(regionB), ","),
		EncounterOfA:     opt(x.EncounterOf(solo(playerA))),
		EncounterOfGroup: opt(x.EncounterOf(group(groupID))),
		EncounterScope:   encScope,
	}
}

// DoD of T-222: the transitions of a scope — entering a region, a group, an
// encounter, leaving — step by step, each step applied twice: Apply sets
// relations as a set, so a repeated event changes nothing (§5.1 step 3).
func TestScopeIndexTransitions(t *testing.T) {
	empty := view{RegionOfA: "-", RegionOfB: "-", RegionOfGroup: "-", WorldOfA: "-", WorldOfGroup: "-", WorldOfRA: "-",
		GroupOfA: "-", GroupOfB: "-", EncounterOfA: "-", EncounterOfGroup: "-", EncounterScope: "-"}
	steps := []struct {
		name   string
		events []eventbus.Event
		change func(v *view)
	}{
		{"characters are created outside the regions", []eventbus.Event{
			created(playerA, "player", map[string]any{"position": outsideW, "scope": map[string]any{"id": playerA, "type": "solo"}}),
			created(playerB, "player", map[string]any{"position": outsideW}),
		}, func(v *view) { v.WorldOfA = worldID }},
		{"A enters a region", []eventbus.Event{updated(playerA, "player", [2]any{"position", regionA})},
			func(v *view) { v.RegionOfA, v.InA, v.WorldOfRA = regionA, playerA, worldID }},
		{"B enters the same region", []eventbus.Event{updated(playerB, "player", [2]any{"position", regionA})},
			func(v *view) { v.RegionOfB, v.InA = regionA, playerA+","+playerB }},
		{"an unrelated change of a character moves nobody", []eventbus.Event{updated(playerA, "player", [2]any{"hp", 7})}, nil},
		{"A solo encounter opens", []eventbus.Event{encounterStarted(solo(playerA))},
			func(v *view) { v.EncounterOfA, v.EncounterScope = encID, "solo:"+playerA }},
		{"the solo encounter ends", []eventbus.Event{encounterEnded(solo(playerA))},
			func(v *view) { v.EncounterOfA, v.EncounterScope = "-", "-" }},
		{"A creates a group", []eventbus.Event{groupEvent("group.created", playerA)},
			func(v *view) { v.GroupOfA, v.WorldOfGroup = groupID, worldID }},
		{"the group entity stands in the region", []eventbus.Event{created(groupID, "group", map[string]any{"position": regionA, "members": []any{}})},
			func(v *view) { v.RegionOfGroup = regionA }},
		{"B joins the group", []eventbus.Event{groupEvent("group.joined", playerA, playerB)},
			func(v *view) { v.GroupOfB = groupID }},
		// The scope of an encounter is written with the prefix of its type by
		// some publishers; the index reads both forms as one scope.
		{"a group encounter opens", []eventbus.Event{encounterStarted(eventbus.ScopeRef{ID: "group:" + groupID, Type: "group"})},
			func(v *view) { v.EncounterOfGroup, v.EncounterScope = encID, "group:"+groupID }},
		{"the group encounter ends", []eventbus.Event{encounterEnded(group(groupID))},
			func(v *view) { v.EncounterOfGroup, v.EncounterScope = "-", "-" }},
		{"the group moves to another region", []eventbus.Event{
			updated(groupID, "group", [2]any{"position", regionB}),
			updated(playerA, "player", [2]any{"position", regionB}),
			updated(playerB, "player", [2]any{"position", regionB}),
		}, func(v *view) {
			v.RegionOfGroup, v.RegionOfA, v.RegionOfB = regionB, regionB, regionB
			v.InA, v.InB = "", playerA+","+playerB
		}},
		{"B leaves the group", []eventbus.Event{groupEvent("group.left", playerA)},
			func(v *view) { v.GroupOfB = "-" }},
		{"B leaves the region", []eventbus.Event{updated(playerB, "player", [2]any{"position", outsideW})},
			func(v *view) { v.RegionOfB, v.InB = "-", playerA }},
		{"the group is disbanded", []eventbus.Event{groupEvent("group.disbanded", playerA)},
			func(v *view) { v.GroupOfA, v.RegionOfGroup, v.WorldOfGroup = "-", "-", "-" }},
		{"an event of another kind changes nothing", []eventbus.Event{
			event("player.attacked", &eventbus.ScopeRef{ID: playerA, Type: "solo"}, map[string]any{"entity": ref(playerA, "player"), "target": ref(regionA, "region")}),
			event("entity.updated", nil, map[string]any{"changed": []any{}}),
			event("group.joined", nil, map[string]any{"members": []any{ref(playerB, "player")}}),
			event("encounter.started", nil, map[string]any{"encounter": ref("enc-2", "encounter")}),
		}, nil},
	}

	x := swarm.NewScopeIndex()
	want := empty
	if got := look(x); got != want {
		t.Fatalf("empty index: %+v", got)
	}
	for _, step := range steps {
		if step.change != nil {
			step.change(&want)
		}
		for round := 1; round <= 2; round++ {
			for _, ev := range step.events {
				x.Apply(ev)
			}
			if got := look(x); got != want {
				t.Fatalf("%s (applied %d×):\n  got  %+v\n  want %+v", step.name, round, got, want)
			}
		}
	}
}

func TestScopeIndexScopes(t *testing.T) {
	x := swarm.NewScopeIndex()
	x.Apply(created(regionA, "region", map[string]any{"description": "лес"}))
	if w, ok := x.WorldOf(region(regionA)); !ok || w != worldID {
		t.Errorf("WorldOf(region) = %q, %v", w, ok)
	}
	if r, ok := x.RegionOf(region(regionA)); !ok || r != regionA {
		t.Errorf("RegionOf(region) = %q, %v: a region lies in itself", r, ok)
	}
	if w, ok := x.WorldOf(eventbus.ScopeRef{ID: worldID, Type: "world"}); !ok || w != worldID {
		t.Errorf("WorldOf(world) = %q, %v", w, ok)
	}
	if r, ok := x.RegionOf(eventbus.ScopeRef{ID: worldID, Type: "world"}); ok {
		t.Errorf("RegionOf(world) = %q: a world lies in no region", r)
	}
	for _, s := range []eventbus.ScopeRef{solo("nobody"), group("none"), region("unknown"), {ID: "x", Type: "planet"}} {
		if r, ok := x.RegionOf(s); ok && s.Type != "region" {
			t.Errorf("RegionOf(%+v) = %q for a scope nobody told the index of", s, r)
		}
		if w, ok := x.WorldOf(s); ok {
			t.Errorf("WorldOf(%+v) = %q for a scope nobody told the index of", s, w)
		}
	}
	if players := x.PlayersIn(""); players != nil {
		t.Errorf("PlayersIn(\"\") = %q", players)
	}

	// A character created in a region, and the world read from its position
	// when the envelope carries none.
	x.Apply(created(playerA, "player", map[string]any{"position": regionA}))
	noWorld := created(playerB, "player", map[string]any{"position": "outside:another-world"})
	noWorld.World = nil
	x.Apply(noWorld)
	if w, ok := x.WorldOf(solo(playerB)); !ok || w != "another-world" {
		t.Errorf("WorldOf(solo B) = %q, %v; want the world of outside:", w, ok)
	}
	if r, ok := x.RegionOf(eventbus.ScopeRef{ID: "solo:" + playerA, Type: "solo"}); !ok || r != regionA {
		t.Errorf("RegionOf(solo:solo:A) = %q, %v", r, ok)
	}

	// A player joining a second group leaves the first.
	x.Apply(groupEvent("group.created", playerA, playerB))
	other := groupEvent("group.created", playerB)
	other.Payload["group"] = ref("group-2", "group")
	x.Apply(other)
	if g, _ := x.GroupOf(playerB); g != "group-2" {
		t.Errorf("GroupOf(B) = %q, want group-2", g)
	}
	x.Apply(groupEvent("group.disbanded"))
	if g, ok := x.GroupOf(playerB); !ok || g != "group-2" {
		t.Errorf("disbanding group-1 took B out of group-2: %q, %v", g, ok)
	}
	if _, ok := x.GroupOf(playerA); ok {
		t.Error("GroupOf(A) after group-1 is disbanded")
	}

	// A group entity whose state becomes disbanded ends the group as well.
	x.Apply(updated("group-2", "group", [2]any{"position", regionA}))
	x.Apply(updated("group-2", "group", [2]any{"state", "disbanded"}))
	if _, ok := x.GroupOf(playerB); ok {
		t.Error("GroupOf(B) after group-2 is disbanded by its entity")
	}
	if r, ok := x.RegionOf(group("group-2")); ok {
		t.Errorf("RegionOf(group-2) = %q after it is disbanded", r)
	}

	// The same encounter reopened for another scope leaves the first scope.
	x.Apply(encounterStarted(solo(playerA)))
	x.Apply(encounterStarted(solo(playerB)))
	if _, ok := x.EncounterOf(solo(playerA)); ok {
		t.Error("EncounterOf(A) after the encounter moved to B")
	}
	if e, ok := x.EncounterOf(solo(playerB)); !ok || e != encID {
		t.Errorf("EncounterOf(B) = %q, %v", e, ok)
	}
	x.Apply(encounterEnded(solo(playerB)))
	x.Apply(encounterEnded(solo(playerB)))
	if _, ok := x.EncounterOf(solo(playerB)); ok {
		t.Error("EncounterOf(B) after the encounter ended")
	}

	// Two encounters of one scope in a row (acceptance of T-222, review #1
	// N-1): the end of the first one does not free the scope of the second.
	second := encounterStarted(solo(playerA))
	second.Payload["encounter"] = ref("enc-2", "encounter")
	x.Apply(encounterStarted(solo(playerA)))
	x.Apply(second)
	x.Apply(encounterEnded(solo(playerA)))
	if e, ok := x.EncounterOf(solo(playerA)); !ok || e != "enc-2" {
		t.Errorf("EncounterOf(A) = %q, %v after enc-1 ended; want enc-2", e, ok)
	}
	if s, ok := x.EncounterScope("enc-2"); !ok || s != solo(playerA) {
		t.Errorf("EncounterScope(enc-2) = %+v, %v", s, ok)
	}
	if _, ok := x.EncounterScope(encID); ok {
		t.Error("EncounterScope(enc-1) after it ended")
	}
}

func TestScopeIndexIsSafeForConcurrentUse(t *testing.T) {
	x := swarm.NewScopeIndex()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("player-%d", i)
			for j := 0; j < 100; j++ {
				position := []string{regionA, regionB, outsideW}[j%3]
				x.Apply(updated(id, "player", [2]any{"position", position}))
				x.Apply(groupEvent("group.joined", id))
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = x.PlayersIn(regionA)
				_, _ = x.RegionOf(solo(playerA))
				_, _ = x.GroupOf(playerA)
				_, _ = x.EncounterOf(group(groupID))
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("player-%d", i)
		// The last position of every writer is index 99 % 3 = 0.
		if r, ok := x.RegionOf(solo(id)); !ok || r != regionA {
			t.Errorf("RegionOf(%s) = %q, %v", id, r, ok)
		}
	}
	if got := x.PlayersIn(regionA); len(got) != 8 || !slices.IsSorted(got) {
		t.Errorf("PlayersIn = %q", got)
	}
}
