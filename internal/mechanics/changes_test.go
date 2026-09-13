package mechanics

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

const factID = "01JCFACT"

// actionAt is the moment of every action of these tests: the timestamp of its
// cause, which is where died_at and acquired_at come from (C-03 v1.3).
var actionAt = time.Date(2026, 9, 13, 18, 30, 5, 0, time.UTC)

func version(v int64) *int64 { return &v }

func setOp(path string, value any) entity.Op {
	return entity.Op{Op: entity.OpSet, Path: path, Value: value}
}

// TestChangesForTable is what each decision writes into the world, one row per
// shape of an outcome (§7.1, ADR-012 p. 4).
func TestChangesForTable(t *testing.T) {
	r := load(t)
	pelt := r.Loot("wolf")
	if len(pelt) == 0 {
		t.Fatal("the rules give a wolf no loot: the trophy rows prove nothing")
	}
	trophy := func(item Item) entity.Op {
		return entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: entity.Item{
			ItemID: factID + ":" + item.Kind, Kind: item.Kind, Name: item.Name,
			Source: entity.ItemSource{
				Entity:  eventbus.EntityRef{ID: "wolf-alpha", Type: entity.TypeNPC},
				EventID: factID,
			},
			AcquiredAt: actionAt,
		}}
	}
	playerRef := entity.Ref{ID: "player-A", Type: entity.TypePlayer}
	wolfRef := entity.Ref{ID: "wolf-alpha", Type: entity.TypeNPC}

	cases := []struct {
		name     string
		kind     string
		outcome  Outcome
		playerHP int
		wolfHP   int
		want     []ProposedChange
	}{
		{
			name:    "a miss changes nothing",
			kind:    ActionAttack,
			outcome: Outcome{Natural: 4, HPBefore: 10, HPAfter: 10},
		},
		{
			name:    "a hit takes the damage off",
			kind:    ActionAttack,
			outcome: Outcome{Hit: true, Natural: 14, Damage: 4, HPBefore: 10, HPAfter: 6},
			want: []ProposedChange{{Entity: wolfRef, ExpectedVersion: version(9), Cause: causeCombat,
				Ops: []entity.Op{setOp(entity.AttrHP, 6)}}},
		},
		{
			name:    "a killing blow on an NPC records the death and hands out the trophy",
			kind:    ActionAttack,
			wolfHP:  3,
			outcome: Outcome{Hit: true, Critical: true, Natural: 20, Damage: 8, HPBefore: 3, HPAfter: 0, TargetDead: true, Loot: pelt},
			want: []ProposedChange{
				{Entity: wolfRef, ExpectedVersion: version(9), Cause: causeCombat, Ops: []entity.Op{
					setOp(entity.AttrHP, 0),
					setOp(entity.AttrStatus, entity.StatusDead),
					setOp(entity.AttrDiedAt, "2026-09-13T18:30:05Z"),
					setOp(entity.AttrKilledBy, "player-A"),
					setOp(entity.AttrLootClaimedBy, "player-A"),
				}},
				{Entity: playerRef, ExpectedVersion: version(4), Cause: causeCombat, Ops: []entity.Op{trophy(pelt[0])}},
			},
		},
		{
			name:    "an NPC with no loot falls without a trophy",
			kind:    ActionAttack,
			wolfHP:  2,
			outcome: Outcome{Hit: true, Natural: 17, Damage: 5, HPBefore: 2, HPAfter: 0, TargetDead: true},
			want: []ProposedChange{{Entity: wolfRef, ExpectedVersion: version(9), Cause: causeCombat, Ops: []entity.Op{
				setOp(entity.AttrHP, 0),
				setOp(entity.AttrStatus, entity.StatusDead),
				setOp(entity.AttrDiedAt, "2026-09-13T18:30:05Z"),
				setOp(entity.AttrKilledBy, "player-A"),
			}}},
		},
		{
			name:    "a blow that costs nothing changes nothing",
			kind:    ActionAttack,
			outcome: Outcome{Hit: true, Natural: 12, Damage: 0, HPBefore: 10, HPAfter: 10},
		},
		{
			name:    "a bite that lands",
			kind:    ActionNPCAttack,
			outcome: Outcome{Hit: true, Natural: 11, Damage: 3, HPBefore: 10, HPAfter: 7},
			want: []ProposedChange{{Entity: playerRef, ExpectedVersion: version(4), Cause: causeCombat,
				Ops: []entity.Op{setOp(entity.AttrHP, 7)}}},
		},
		{
			name:     "a character who falls has no death record of an NPC",
			kind:     ActionNPCAttack,
			playerHP: 2,
			outcome:  Outcome{Hit: true, Natural: 19, Damage: 4, HPBefore: 2, HPAfter: 0, TargetDead: true},
			want: []ProposedChange{{Entity: playerRef, ExpectedVersion: version(4), Cause: causeCombat, Ops: []entity.Op{
				setOp(entity.AttrHP, 0),
				setOp(entity.AttrStatus, entity.StatusDead),
			}}},
		},
		{
			name:    "a free attack is part of the flight it punishes",
			kind:    ActionFreeAttack,
			outcome: Outcome{Hit: true, Natural: 13, Damage: 2, HPBefore: 10, HPAfter: 8, FreeAttack: true},
			want: []ProposedChange{{Entity: playerRef, ExpectedVersion: version(4), Cause: causeFlee,
				Ops: []entity.Op{setOp(entity.AttrHP, 8)}}},
		},
		{
			name:    "a missed free attack changes nothing",
			kind:    ActionFreeAttack,
			outcome: Outcome{Natural: 3, HPBefore: 10, HPAfter: 10, FreeAttack: true},
		},
		{
			name:    "a flight changes nothing ChangesFor can place",
			kind:    ActionFlee,
			outcome: Outcome{Success: new(true), Natural: 15, Threshold: 11, HPBefore: 10, HPAfter: 10},
		},
		{
			name:    "a failed flight changes nothing by itself",
			kind:    ActionFlee,
			outcome: Outcome{Success: new(false), Natural: 3, Threshold: 11, HPBefore: 10, HPAfter: 10, FreeAttack: true},
		},
		{
			name:     "a rest restores",
			kind:     ActionRest,
			playerHP: 4,
			outcome:  Outcome{Hit: true, HPBefore: 4, HPAfter: 10},
			want: []ProposedChange{{Entity: playerRef, ExpectedVersion: version(4), Cause: causeRest,
				Ops: []entity.Op{setOp(entity.AttrHP, 10)}}},
		},
		{
			name:    "a rest at full health changes nothing",
			kind:    ActionRest,
			outcome: Outcome{Hit: true, HPBefore: 10, HPAfter: 10},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			player, wolf := fighters(t, r)
			if c.playerHP != 0 {
				player.HP = c.playerHP
			}
			if c.wolfHP != 0 {
				wolf.HP = c.wolfHP
			}
			action, attacker, target := actionFor(c.kind, player, wolf)

			got, err := ChangesFor(action, c.outcome, attacker, target, factID)
			if err != nil {
				t.Fatalf("changes: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("changes\n  got:  %+v\n  want: %+v", got, c.want)
			}
		})
	}
}

// actionFor is the action of a kind between the two fighters, and who is the
// attacker and who the target of it.
func actionFor(kind string, player, wolf *Actor) (Action, *Actor, *Actor) {
	switch kind {
	case ActionNPCAttack, ActionFreeAttack:
		return Action{Kind: kind, Actor: wolf.ID, Target: player.ID, At: actionAt}, wolf, player
	case ActionFlee:
		return Action{Kind: kind, Actor: player.ID, LivingEnemies: 1, At: actionAt}, player, nil
	case ActionRest:
		return Action{Kind: kind, Actor: player.ID, At: actionAt}, player, nil
	default:
		return Action{Kind: kind, Actor: player.ID, Target: wolf.ID, At: actionAt}, player, wolf
	}
}

// TestChangesForHoldsHitPointsInRange is inv-02 on the side of the proposal,
// over every decision Resolve makes for a few hundred causes and every starting
// health: whatever the package writes into hp lies in [0, hp_max], agrees with
// the decision, and a fall and a zero always come together.
func TestChangesForHoldsHitPointsInRange(t *testing.T) {
	r := load(t)
	for i := range 400 {
		cause := fmt.Sprintf("01JCRANGE%04d", i)
		for _, kind := range []string{ActionAttack, ActionNPCAttack, ActionFreeAttack} {
			player, wolf := fighters(t, r)
			player.HP, wolf.HP = 1+i%10, 1+(i/3)%10
			action, attacker, target := actionFor(kind, player, wolf)

			out, _, err := r.Resolve(cause, i%3, action, actorsMap(player, wolf))
			if err != nil {
				t.Fatalf("%s %s: resolve: %v", cause, kind, err)
			}
			changes, err := ChangesFor(action, out, attacker, target, factID)
			if err != nil {
				t.Fatalf("%s %s: changes for %+v: %v", cause, kind, out, err)
			}
			if !out.Hit {
				if changes != nil {
					t.Fatalf("%s %s: a miss proposes %+v", cause, kind, changes)
				}
				continue
			}
			hp, status := written(t, changes, target.ID)
			if hp < 0 || hp > target.HPMax || hp != out.HPAfter {
				t.Fatalf("%s %s: hp %d written, decision %d, max %d", cause, kind, hp, out.HPAfter, target.HPMax)
			}
			if (hp == 0) != (status == entity.StatusDead) || out.TargetDead != (hp == 0) {
				t.Fatalf("%s %s: hp %d status %q target_dead %v", cause, kind, hp, status, out.TargetDead)
			}
		}
	}
}

func written(t *testing.T, changes []ProposedChange, id string) (hp int, status string) {
	t.Helper()
	hp = -1
	for _, change := range changes {
		if change.Entity.ID != id {
			continue
		}
		for _, op := range change.Ops {
			switch op.Path {
			case entity.AttrHP:
				hp = op.Value.(int)
			case entity.AttrStatus:
				status = op.Value.(string)
			}
		}
	}
	if hp == -1 {
		t.Fatalf("no hp written for %s in %+v", id, changes)
	}
	return hp, status
}

// TestChangesForClampsIntoTheMaximum: a target that stands above its maximum
// — a defect of whoever wrote it, which inv-02 refuses — is brought back into
// range by the blow rather than kept out of it.
func TestChangesForClampsIntoTheMaximum(t *testing.T) {
	r := load(t)
	player, wolf := fighters(t, r)
	wolf.HP = 15
	action, attacker, target := actionFor(ActionAttack, player, wolf)

	got, err := ChangesFor(action, Outcome{Hit: true, Damage: 1, HPBefore: 15, HPAfter: 10}, attacker, target, factID)
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	if hp, _ := written(t, got, wolf.ID); hp != wolf.HPMax {
		t.Errorf("hp %d written, want the maximum %d", hp, wolf.HPMax)
	}
}

// TestChangesForAgainstTheWorldAsItIsNow is a package worked out again after a
// version conflict (C-05 v1.3 p. 1): the same blow comes off the hit points the
// target has now. A retry that would change who is left standing is refused —
// the decision about the fall is already published (C-05 v1.4 p. 1в).
func TestChangesForAgainstTheWorldAsItIsNow(t *testing.T) {
	r := load(t)
	decided := Outcome{Hit: true, Natural: 15, Damage: 3, HPBefore: 10, HPAfter: 7}

	player, wolf := fighters(t, r)
	wolf.HP = 8 // somebody else struck in between
	action, attacker, target := actionFor(ActionAttack, player, wolf)
	got, err := ChangesFor(action, decided, attacker, target, factID)
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	if hp, _ := written(t, got, wolf.ID); hp != 5 {
		t.Errorf("hp %d written, want the 3 damage off the 8 the wolf has now", hp)
	}

	wolf.HP = 2 // the same blow would now kill
	if _, err := ChangesFor(action, decided, attacker, target, factID); err == nil ||
		!strings.Contains(err.Error(), "target_dead false") {
		t.Errorf("a retry that would kill answered %v, want a refusal", err)
	}

	killing := Outcome{Hit: true, Natural: 15, Damage: 3, HPBefore: 3, HPAfter: 0, TargetDead: true}
	wolf.HP = 9 // healed in between: the killing blow would no longer kill
	if _, err := ChangesFor(action, killing, attacker, target, factID); err == nil ||
		!strings.Contains(err.Error(), "target_dead true") {
		t.Errorf("a retry that would not kill answered %v, want a refusal", err)
	}
}

// TestChangesForRefuses is every outcome that cannot belong to its action.
func TestChangesForRefuses(t *testing.T) {
	r := load(t)
	pelt := r.Loot("wolf")

	cases := []struct {
		name    string
		kind    string
		outcome Outcome
		change  func(a *Action, attacker, target **Actor, fact *string)
		says    string
	}{
		{name: "no attacker", kind: ActionAttack, change: func(_ *Action, attacker, _ **Actor, _ *string) { *attacker = nil }, says: "without an attacker"},
		{name: "the attacker is not the actor", kind: ActionAttack, change: func(a *Action, _, _ **Actor, _ *string) { a.Actor = "player-B" }, says: "player-B"},
		{name: "no fact", kind: ActionAttack, change: func(_ *Action, _, _ **Actor, fact *string) { *fact = "" }, says: "without the fact"},
		{name: "unknown kind", kind: ActionAttack, change: func(a *Action, _, _ **Actor, _ *string) { a.Kind = "meditate" }, says: "meditate"},
		{name: "no target", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: 1}, change: func(_ *Action, _, target **Actor, _ *string) { *target = nil }, says: "without a target"},
		{name: "the target is not the one aimed at", kind: ActionAttack, change: func(a *Action, _, _ **Actor, _ *string) { a.Target = "wolf-beta" }, says: "wolf-beta"},
		{name: "a flight result on a strike", kind: ActionAttack, outcome: Outcome{Success: new(true)}, says: "is a flight"},
		{name: "negative damage", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: -2}, says: "damage -2"},
		{name: "damage without a hit", kind: ActionAttack, outcome: Outcome{Damage: 3}, says: "without a hit"},
		{name: "a fall without a hit", kind: ActionAttack, outcome: Outcome{TargetDead: true}, says: "without a hit"},
		{name: "a trophy of a target still standing", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: 1, Loot: pelt}, says: "trophy"},
		{name: "a trophy of a character", kind: ActionNPCAttack, outcome: Outcome{Hit: true, Damage: 10, TargetDead: true, Loot: pelt}, says: "trophy"},
		{name: "a blow on the dead", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: 1},
			change: func(_ *Action, _, target **Actor, _ *string) { (*target).Status = entity.StatusDead }, says: "already dead"},
		{name: "a blow on the abandoned", kind: ActionNPCAttack, outcome: Outcome{Hit: true, Damage: 1},
			change: func(_ *Action, _, target **Actor, _ *string) { (*target).Status = entity.StatusAbandoned }, says: "already abandoned"},
		{name: "a fall the hit points deny", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: 1, TargetDead: true}, says: "target_dead true"},
		// C-03 v1.3: no proposal without a version, no death record without a time.
		{name: "a wounded target of no version", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: 1},
			change: func(_ *Action, _, target **Actor, _ *string) { (*target).Version = 0 }, says: "version 0"},
		{name: "a winner of no version", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: 10, TargetDead: true, Loot: pelt},
			change: func(_ *Action, attacker, _ **Actor, _ *string) { (*attacker).Version = 0 }, says: "version 0"},
		{name: "a rested character of no version", kind: ActionRest, outcome: Outcome{Hit: true, HPAfter: 10},
			change: func(_ *Action, attacker, _ **Actor, _ *string) { (*attacker).HP, (*attacker).Version = 3, 0 }, says: "version 0"},
		{name: "a fallen NPC without a time", kind: ActionAttack, outcome: Outcome{Hit: true, Damage: 10, TargetDead: true},
			change: func(a *Action, _, _ **Actor, _ *string) { a.At = time.Time{} }, says: "Action.At"},
		{name: "a flight without a flight result", kind: ActionFlee, outcome: Outcome{Hit: true}, says: "not a flight"},
		{name: "a rest with a flight result", kind: ActionRest, outcome: Outcome{Success: new(false)}, says: "is a flight"},
		{name: "a rest of the dead", kind: ActionRest, outcome: Outcome{Hit: true, HPAfter: 10},
			change: func(_ *Action, attacker, _ **Actor, _ *string) { (*attacker).Status = entity.StatusDead }, says: "dead"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			player, wolf := fighters(t, r)
			action, attacker, target := actionFor(c.kind, player, wolf)
			fact := factID
			if c.change != nil {
				c.change(&action, &attacker, &target, &fact)
			}
			got, err := ChangesFor(action, c.outcome, attacker, target, fact)
			if err == nil {
				t.Fatalf("proposed %+v", got)
			}
			if got != nil {
				t.Errorf("proposed %+v together with the error", got)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("error %q does not say %q", err, c.says)
			}
		})
	}
}

// TestChangesForAppliesAsOnePackage: what ChangesFor proposes is operations
// State can apply as they are — every set and append lands, the trophy reads
// back as an item of the inventory, and the NPC reads back dead with its killer.
func TestChangesForAppliesAsOnePackage(t *testing.T) {
	r := load(t)
	player, wolf := fighters(t, r)
	wolf.HP = 1
	action, attacker, target := actionFor(ActionAttack, player, wolf)

	out, _, err := r.Resolve(causeWithNatural(t, r, 16, 0), 0, action, actorsMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	changes, err := ChangesFor(action, out, attacker, target, factID)
	if err != nil {
		t.Fatalf("changes: %v", err)
	}

	world := map[string]*entity.Entity{
		player.ID: playerEntity(map[string]any{entity.AttrInventory: []any{}}),
		wolf.ID:   wolfEntity(),
	}
	world[wolf.ID].Attributes[entity.AttrHP] = 1
	for _, change := range changes {
		ent := world[change.Entity.ID]
		attrs, _, err := entity.ApplyOps(ent, change.Ops)
		if err != nil {
			t.Fatalf("apply to %s: %v", change.Entity, err)
		}
		ent.Attributes = attrs
	}

	npc := world[wolf.ID]
	if hp, _ := npc.HP(); hp != 0 {
		t.Errorf("the wolf stands at %d", hp)
	}
	if status, _ := npc.Status(); status != entity.StatusDead {
		t.Errorf("the wolf is %s", status)
	}
	if killer, _ := npc.KilledBy(); killer != player.ID {
		t.Errorf("killed by %q", killer)
	}
	if died, ok := npc.DiedAt(); !ok || !died.Equal(actionAt) {
		t.Errorf("died at %v (%v), want the time of the cause %v", died, ok, actionAt)
	}
	items, err := world[player.ID].Inventory()
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if len(items) != 1 || items[0].Kind != "wolf-pelt" || items[0].Source.EventID != factID ||
		items[0].Source.Entity.ID != wolf.ID || !items[0].AcquiredAt.Equal(actionAt) {
		t.Errorf("inventory %+v", items)
	}
	for _, change := range changes {
		who := actorsMap(player, wolf)[change.Entity.ID]
		if change.ExpectedVersion == nil || *change.ExpectedVersion != who.Version {
			t.Errorf("%s expected at %v, the actor was read at %d", change.Entity, change.ExpectedVersion, who.Version)
		}
	}
}

// TestChangesForPinsTheVersionOfEachActor: in a package of two entities each
// change carries the version of its own actor, never the other's.
func TestChangesForPinsTheVersionOfEachActor(t *testing.T) {
	r := load(t)
	player, wolf := fighters(t, r)
	player.Version, wolf.Version, wolf.HP = 17, 3, 1
	action, attacker, target := actionFor(ActionAttack, player, wolf)

	got, err := ChangesFor(action, Outcome{Hit: true, Damage: 2, TargetDead: true, Loot: r.Loot("wolf")},
		attacker, target, factID)
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	want := map[string]int64{wolf.ID: 3, player.ID: 17}
	if len(got) != len(want) {
		t.Fatalf("%d changes, want the wolf and the winner", len(got))
	}
	for _, change := range got {
		if change.ExpectedVersion == nil || *change.ExpectedVersion != want[change.Entity.ID] {
			t.Errorf("%s expected at %v, want %d", change.Entity, change.ExpectedVersion, want[change.Entity.ID])
		}
	}
}
