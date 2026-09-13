package mechanics

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"multiverse-core.io/shared/entity"
)

// causeWithNatural finds a cause event whose d20 at this index shows the wanted
// face. A table of outcomes is written in naturals, and a test that names the
// face it needs says more than a magic identifier would; the search over a few
// thousand identifiers also proves every face is reachable.
func causeWithNatural(t *testing.T, r *Rules, natural, index int) string {
	t.Helper()
	for i := range 5000 {
		id := fmt.Sprintf("01JCRESOLVE%06d", i)
		roll, err := r.Roll(id, index, "d20", PurposeHit)
		if err != nil {
			t.Fatalf("roll: %v", err)
		}
		if roll.Natural == natural {
			return id
		}
	}
	t.Fatalf("no cause out of 5000 shows a natural %d at index %d", natural, index)
	return ""
}

// fighters are the two actors of the dark forest as the rules create them, at
// the versions an entity read from the world would carry.
func fighters(t *testing.T, r *Rules) (player, wolf *Actor) {
	t.Helper()
	p, ok := r.Stats(entity.TypePlayer)
	if !ok {
		t.Fatal("the rules describe no player")
	}
	w, ok := r.Stats("wolf")
	if !ok {
		t.Fatal("the rules describe no wolf")
	}
	p.ID, w.ID = "player-A", "wolf-alpha"
	p.Version, w.Version = 4, 9
	return &p, &w
}

func actorsMap(as ...*Actor) map[string]*Actor {
	out := make(map[string]*Actor, len(as))
	for _, a := range as {
		out[a.ID] = a
	}
	return out
}

// rulesWith loads the shipped rules with one fragment replaced, for the rule
// sets the file of v0.1 does not describe.
func rulesWith(t *testing.T, old, replacement string) *Rules {
	t.Helper()
	body, err := readRules()
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	if !strings.Contains(body, old) {
		t.Fatalf("the rules file has no %q to replace", old)
	}
	r, err := LoadBytes([]byte(strings.Replace(body, old, replacement, 1)))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return r
}

// TestResolveAttackTable is the table of outcomes of §5.4 for a strike, at the
// faces where the rules change their answer: the fumble and the crit, and the
// two faces on either side of the defence. A character (atk 2) needs 9 on the
// die against a wolf (def 11); the same faces decide the answer of the wolf
// (atk 3) against a character (def 12).
func TestResolveAttackTable(t *testing.T) {
	r := load(t)
	attack := r.Attack()

	cases := []struct {
		name      string
		kind      string
		natural   int
		targetDef int // 0 keeps the rules
		start     int
		hit, crit bool
		fumble    bool
	}{
		{name: "fumble misses", kind: ActionAttack, natural: attack.FumbleNatural, fumble: true},
		{name: "one above the fumble misses", kind: ActionAttack, natural: attack.FumbleNatural + 1},
		{name: "one below the defence misses", kind: ActionAttack, natural: 8},
		{name: "the defence exactly hits", kind: ActionAttack, natural: 9, hit: true},
		{name: "one below the crit hits plainly", kind: ActionAttack, natural: attack.CritNatural - 1, hit: true},
		{name: "crit hits and doubles", kind: ActionAttack, natural: attack.CritNatural, hit: true, crit: true},
		{name: "crit hits a defence no sum reaches", kind: ActionAttack, natural: attack.CritNatural, targetDef: 40, hit: true, crit: true},
		{name: "one below the crit misses that defence", kind: ActionAttack, natural: attack.CritNatural - 1, targetDef: 40},
		{name: "fumble misses a defence any sum reaches", kind: ActionAttack, natural: attack.FumbleNatural, targetDef: 1, fumble: true},
		{name: "one above the fumble hits that defence", kind: ActionAttack, natural: attack.FumbleNatural + 1, targetDef: 1, hit: true},
		{name: "rolls start where they are told", kind: ActionAttack, natural: 15, start: 4, hit: true},
		{name: "npc bite below the defence misses", kind: ActionNPCAttack, natural: 8, start: 2},
		{name: "npc bite at the defence hits", kind: ActionNPCAttack, natural: 9, start: 2, hit: true},
		{name: "npc crit", kind: ActionNPCAttack, natural: attack.CritNatural, start: 2, hit: true, crit: true},
		{name: "npc fumble", kind: ActionNPCAttack, natural: attack.FumbleNatural, start: 2, fumble: true},
		{name: "free attack after a failed flight", kind: ActionFreeAttack, natural: 12, start: 1, hit: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			player, wolf := fighters(t, r)
			attacker, target := player, wolf
			hitPurpose, damagePurpose := PurposeHit, PurposeDamage
			if c.kind != ActionAttack {
				attacker, target = wolf, player
				hitPurpose, damagePurpose = PurposeNPCHit, PurposeNPCDamage
			}
			if c.targetDef != 0 {
				target.Def = c.targetDef
			}
			cause := causeWithNatural(t, r, c.natural, c.start)

			out, rolls, err := r.Resolve(cause, c.start,
				Action{Kind: c.kind, Actor: attacker.ID, Target: target.ID}, actorsMap(player, wolf))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			if out.Natural != c.natural || out.Hit != c.hit || out.Critical != c.crit || out.Fumble != c.fumble {
				t.Errorf("natural %d hit %v crit %v fumble %v, want %d/%v/%v/%v",
					out.Natural, out.Hit, out.Critical, out.Fumble, c.natural, c.hit, c.crit, c.fumble)
			}
			if out.Success != nil || out.Threshold != 0 {
				t.Errorf("a strike reports a flight: success %v threshold %d", out.Success, out.Threshold)
			}
			if want := c.kind == ActionFreeAttack; out.FreeAttack != want {
				t.Errorf("free_attack %v for %s", out.FreeAttack, c.kind)
			}
			if out.HPBefore != target.HP {
				t.Errorf("hp_before %d, the target stands at %d", out.HPBefore, target.HP)
			}

			if rolls[0].Index != c.start || rolls[0].Purpose != hitPurpose || rolls[0].Natural != c.natural {
				t.Errorf("hit roll %+v, want index %d purpose %s natural %d", rolls[0], c.start, hitPurpose, c.natural)
			}
			if rolls[0].Seed != Seed(cause, c.start) || rolls[0].Formula != "d20" {
				t.Errorf("hit roll %+v is not addressed by (%s, %d)", rolls[0], cause, c.start)
			}

			if !c.hit {
				if len(rolls) != 1 || out.Damage != 0 || out.HPAfter != target.HP || out.TargetDead {
					t.Errorf("a miss: %d rolls, damage %d, hp %d -> %d, dead %v",
						len(rolls), out.Damage, out.HPBefore, out.HPAfter, out.TargetDead)
				}
				return
			}
			if len(rolls) != 2 {
				t.Fatalf("%d rolls for a hit, want the hit and the damage", len(rolls))
			}
			dice, _ := r.DamageDice(*attacker)
			if rolls[1].Index != c.start+1 || rolls[1].Purpose != damagePurpose || rolls[1].Formula != dice.String() {
				t.Errorf("damage roll %+v, want index %d purpose %s formula %s",
					rolls[1], c.start+1, damagePurpose, dice)
			}
			wantDamage := rolls[1].Result
			if c.crit {
				wantDamage *= attack.CritMultiplier
			}
			if out.Damage != wantDamage {
				t.Errorf("damage %d, the roll %d on crit %v gives %d", out.Damage, rolls[1].Result, c.crit, wantDamage)
			}
			if want := max(target.HP-wantDamage, 0); out.HPAfter != want {
				t.Errorf("hp_after %d, want %d", out.HPAfter, want)
			}
			if out.TargetDead != (out.HPAfter == 0) {
				t.Errorf("target_dead %v at hp %d", out.TargetDead, out.HPAfter)
			}
		})
	}
}

// TestResolveKillingBlow covers the end of a fight: hit points clamp at zero
// however much the blow was worth, the target is reported dead, and a fallen
// NPC leaves the trophy of its kind while a fallen character leaves nothing.
func TestResolveKillingBlow(t *testing.T) {
	r := load(t)
	attack := r.Attack()

	player, wolf := fighters(t, r)
	wolf.HP = 1
	out, _, err := r.Resolve(causeWithNatural(t, r, attack.CritNatural, 0), 0,
		Action{Kind: ActionAttack, Actor: player.ID, Target: wolf.ID}, actorsMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !out.TargetDead || out.HPAfter != 0 || out.Damage < 2 {
		t.Errorf("a crit on a wolf at 1 hp: dead %v hp %d damage %d", out.TargetDead, out.HPAfter, out.Damage)
	}
	if want := r.Loot("wolf"); len(want) == 0 || !reflect.DeepEqual(out.Loot, want) {
		t.Errorf("loot %v, the rules give a wolf %v", out.Loot, want)
	}

	player, wolf = fighters(t, r)
	player.HP = 1
	out, _, err = r.Resolve(causeWithNatural(t, r, 15, 2), 2,
		Action{Kind: ActionNPCAttack, Actor: wolf.ID, Target: player.ID}, actorsMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve the bite: %v", err)
	}
	if !out.TargetDead || out.HPAfter != 0 || out.Loot != nil {
		t.Errorf("a bite on a character at 1 hp: dead %v hp %d loot %v", out.TargetDead, out.HPAfter, out.Loot)
	}

	// An NPC of a kind the rules give no trophy — or of no kind at all —
	// falls empty-handed.
	player, wolf = fighters(t, r)
	wolf.HP, wolf.Kind = 1, ""
	out, _, err = r.Resolve(causeWithNatural(t, r, 15, 0), 0,
		Action{Kind: ActionAttack, Actor: player.ID, Target: wolf.ID}, actorsMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !out.TargetDead || len(out.Loot) != 0 {
		t.Errorf("a wolf of no kind: dead %v loot %v", out.TargetDead, out.Loot)
	}
}

// TestResolveFleeTable: the threshold is 10 plus the enemies still standing,
// read from the rules, and a character (flee +2) makes it on the face that
// brings the sum up to it. A failed flight costs a free attack when the rules
// say so, and the flight itself rolls only once.
func TestResolveFleeTable(t *testing.T) {
	r := load(t)
	player, _ := fighters(t, r)

	// The golden numbers of the file: against one wolf 11, against two 12.
	for enemies, want := range map[int]int{1: 11, 2: 12} {
		threshold, err := r.FleeThreshold(*player, enemies)
		if err != nil {
			t.Fatalf("threshold: %v", err)
		}
		if threshold != want {
			t.Errorf("threshold against %d enemies is %d, want %d", enemies, threshold, want)
		}
	}

	cases := []struct {
		name    string
		enemies int
		natural int
		success bool
	}{
		{"one enemy, one below", 1, 8, false},
		{"one enemy, exactly", 1, 9, true},
		{"two enemies, the face that was enough for one", 2, 9, false},
		{"two enemies, exactly", 2, 10, true},
		{"a natural 20 is only a number", 30, 20, false},
		{"a natural 1 is only a number", 0, 1, false},
		{"nobody standing is still a roll", 0, 8, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			player, wolf := fighters(t, r)
			cause := causeWithNatural(t, r, c.natural, 0)
			out, rolls, err := r.Resolve(cause, 0,
				Action{Kind: ActionFlee, Actor: player.ID, LivingEnemies: c.enemies}, actorsMap(player, wolf))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if out.Success == nil || *out.Success != c.success {
				t.Fatalf("success %v, want %v", out.Success, c.success)
			}
			want, _ := r.FleeThreshold(*player, c.enemies)
			if out.Threshold != want || out.Natural != c.natural {
				t.Errorf("threshold %d natural %d, want %d/%d", out.Threshold, out.Natural, want, c.natural)
			}
			if out.FreeAttack == c.success {
				t.Errorf("free_attack %v after success %v (on_fail %s)", out.FreeAttack, c.success, r.Flee().OnFail)
			}
			if out.Hit || out.Critical || out.Fumble || out.Damage != 0 || out.HPAfter != player.HP {
				t.Errorf("a flight reports a strike: %+v", out)
			}
			if len(rolls) != 1 || rolls[0].Purpose != PurposeFlee || rolls[0].Index != 0 {
				t.Errorf("rolls %+v, want one flee roll at index 0", rolls)
			}
		})
	}

	// on_fail: none — a failed flight costs nothing.
	calm := rulesWith(t, "on_fail: free_attack", "on_fail: none")
	player, wolf := fighters(t, calm)
	out, _, err := calm.Resolve(causeWithNatural(t, calm, 2, 0), 0,
		Action{Kind: ActionFlee, Actor: player.ID, LivingEnemies: 1}, actorsMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if *out.Success || out.FreeAttack {
		t.Errorf("on_fail none: success %v free_attack %v", *out.Success, out.FreeAttack)
	}
}

// TestFreeAttackAfterAFailedFlight is the whole exchange of a failed flight as
// the caller runs it (§5.4): the flight at the first index, the free attack of
// the NPC NPCTarget picks from the index after it, with the purposes of an NPC
// and the flag that tells the narrator it was out of turn. The indices are the
// ones the encounter double uses (rollFlee 0, rollFreeAttack 1).
func TestFreeAttackAfterAFailedFlight(t *testing.T) {
	r := load(t)
	const rollFlee, rollFreeAttack = 0, 1

	var cause string
	for i := range 5000 {
		id := fmt.Sprintf("01JCFLEE%06d", i)
		flight, _ := r.Roll(id, rollFlee, "d20", PurposeFlee)
		bite, _ := r.Roll(id, rollFreeAttack, "d20", PurposeNPCHit)
		if flight.Natural < 9 && bite.Natural >= 9 && bite.Natural < 20 {
			cause = id
			break
		}
	}
	if cause == "" {
		t.Fatal("no cause with a failed flight answered by a landed bite")
	}

	player, wolf := fighters(t, r)
	actors := actorsMap(player, wolf)
	flight, fleeRolls, err := r.Resolve(cause, rollFlee,
		Action{Kind: ActionFlee, Actor: player.ID, LivingEnemies: 1}, actors)
	if err != nil {
		t.Fatalf("flee: %v", err)
	}
	if *flight.Success || !flight.FreeAttack {
		t.Fatalf("the flight: success %v free_attack %v, want a failure that costs a free attack",
			*flight.Success, flight.FreeAttack)
	}

	biter, err := r.NPCTarget(wolf, []*Actor{player})
	if err != nil || biter == nil {
		t.Fatalf("npc target: (%v, %v)", biter, err)
	}
	bite, biteRolls, err := r.Resolve(cause, rollFleeNext(fleeRolls),
		Action{Kind: ActionFreeAttack, Actor: wolf.ID, Target: biter.ID}, actors)
	if err != nil {
		t.Fatalf("free attack: %v", err)
	}
	if !bite.FreeAttack || !bite.Hit || bite.Damage == 0 {
		t.Errorf("the free attack: %+v", bite)
	}
	indices := []int{fleeRolls[0].Index, biteRolls[0].Index, biteRolls[1].Index}
	purposes := []string{fleeRolls[0].Purpose, biteRolls[0].Purpose, biteRolls[1].Purpose}
	if !reflect.DeepEqual(indices, []int{rollFlee, rollFreeAttack, rollFreeAttack + 1}) {
		t.Errorf("roll indices %v, want %d, %d, %d", indices, rollFlee, rollFreeAttack, rollFreeAttack+1)
	}
	if !reflect.DeepEqual(purposes, []string{PurposeFlee, PurposeNPCHit, PurposeNPCDamage}) {
		t.Errorf("purposes %v", purposes)
	}
}

// rollFleeNext is the first index after the rolls of a flight: §5.4 has the
// free attack start there.
func rollFleeNext(rolls []Roll) int { return rolls[len(rolls)-1].Index + 1 }

// TestResolveRest: a rest restores to the maximum, rolls nothing and cannot
// fail. A rest by dice never reaches Resolve: Load refuses it (TestLoadRejects).
func TestResolveRest(t *testing.T) {
	r := load(t)
	player, _ := fighters(t, r)
	player.HP = 3

	out, rolls, err := r.Resolve("01JCREST", 0, Action{Kind: ActionRest, Actor: player.ID}, actorsMap(player))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !out.Hit || out.HPBefore != 3 || out.HPAfter != player.HPMax || len(rolls) != 0 || out.Success != nil {
		t.Errorf("rest: %+v, %d rolls", out, len(rolls))
	}
}

// TestResolveRefuses is every defect of the caller Resolve can see. None of
// them is an outcome: a miss is an outcome, a strike at a corpse is a bug.
func TestResolveRefuses(t *testing.T) {
	r := load(t)

	type mutate func(player, wolf *Actor)
	cases := []struct {
		name   string
		cause  string
		index  int
		action func(player, wolf *Actor) Action
		change mutate
		is     error
		says   string
	}{
		{name: "no cause", action: attackOf, says: "cause"},
		{name: "negative index", cause: "c", index: -1, action: attackOf, says: "negative"},
		{name: "unknown kind", cause: "c", action: func(p, _ *Actor) Action { return Action{Kind: "meditate", Actor: p.ID} }, says: "meditate"},
		{name: "no actor", cause: "c", action: func(_, w *Actor) Action { return Action{Kind: ActionAttack, Target: w.ID} }, says: "without actor"},
		{name: "actor not among the actors", cause: "c", action: func(_, w *Actor) Action { return Action{Kind: ActionAttack, Actor: "player-Z", Target: w.ID} }, says: "player-Z"},
		{name: "no target", cause: "c", action: func(p, _ *Actor) Action { return Action{Kind: ActionAttack, Actor: p.ID} }, says: "without target"},
		{name: "target not among the actors", cause: "c", action: func(p, _ *Actor) Action { return Action{Kind: ActionAttack, Actor: p.ID, Target: "wolf-Z"} }, says: "wolf-Z"},
		{name: "a map entry under another id", cause: "c", action: attackOf, change: func(_, w *Actor) { w.ID = "wolf-beta" }, says: "wolf-beta"},
		{name: "the dead do not act", cause: "c", action: attackOf, change: func(p, _ *Actor) { p.Status = entity.StatusDead }, says: "does not act"},
		{name: "the abandoned do not act", cause: "c", action: attackOf, change: func(p, _ *Actor) { p.Status = entity.StatusAbandoned }, says: "does not act"},
		{name: "a wolf does not attack as a character", cause: "c", action: func(p, w *Actor) Action { return Action{Kind: ActionAttack, Actor: w.ID, Target: p.ID} }, says: "dealt by a player"},
		{name: "a character does not bite", cause: "c", action: func(p, w *Actor) Action { return Action{Kind: ActionNPCAttack, Actor: p.ID, Target: w.ID} }, says: "dealt by a npc"},
		{name: "no strike at oneself", cause: "c", action: func(p, _ *Actor) Action { return Action{Kind: ActionAttack, Actor: p.ID, Target: p.ID} }, says: "itself"},
		{name: "no fight among characters", cause: "c", action: func(p, _ *Actor) Action { return Action{Kind: ActionAttack, Actor: p.ID, Target: "player-B"} }, says: "aimed at a npc"},
		{name: "dead target", cause: "c", action: attackOf, change: func(_, w *Actor) { w.Status = entity.StatusDead }, is: ErrInvalidTarget},
		{name: "abandoned target", cause: "c", action: biteOf, change: func(p, _ *Actor) { p.Status = entity.StatusAbandoned }, is: ErrInvalidTarget},
		{name: "ascended target", cause: "c", action: biteOf, change: func(p, _ *Actor) { p.Status = entity.StatusAscendedFinal }, is: ErrInvalidTarget},
		{name: "idle target", cause: "c", action: biteOf, change: func(p, _ *Actor) { p.Participation = entity.ParticipationIdle }, is: ErrInvalidTarget},
		{name: "target out of combat", cause: "c", action: biteOf, change: func(p, _ *Actor) { p.Participation = entity.ParticipationOutOfCombat }, is: ErrInvalidTarget},
		{name: "damage that is not dice", cause: causeWithNatural(t, r, 15, 0), action: attackOf, change: func(p, _ *Actor) { p.Dmg = "a lot" }, says: "dmg"},
		{name: "a flight against fewer than nobody", cause: "c", action: func(p, _ *Actor) Action { return Action{Kind: ActionFlee, Actor: p.ID, LivingEnemies: -1} }, says: "-1"},
		{name: "a wolf does not run", cause: "c", action: func(_, w *Actor) Action { return Action{Kind: ActionFlee, Actor: w.ID, LivingEnemies: 1} }, says: "flee"},
		{name: "the dead do not rest", cause: "c", action: func(p, _ *Actor) Action { return Action{Kind: ActionRest, Actor: p.ID} }, change: func(p, _ *Actor) { p.Status = entity.StatusDead }, says: "does not act"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			player, wolf := fighters(t, r)
			player2 := *player
			player2.ID = "player-B"
			actors := actorsMap(player, wolf, &player2)
			action := c.action(player, wolf)
			if c.change != nil {
				c.change(player, wolf)
			}
			out, rolls, err := r.Resolve(c.cause, c.index, action, actors)
			if err == nil {
				t.Fatalf("resolved %+v with rolls %v", out, rolls)
			}
			if c.is != nil && !errors.Is(err, c.is) {
				t.Errorf("error %v, want one matching %v", err, c.is)
			}
			if c.says != "" && !strings.Contains(err.Error(), c.says) {
				t.Errorf("error %q does not say %q", err, c.says)
			}
			if rolls != nil {
				t.Errorf("a refused action rolled %v", rolls)
			}
		})
	}
}

func attackOf(p, w *Actor) Action { return Action{Kind: ActionAttack, Actor: p.ID, Target: w.ID} }
func biteOf(p, w *Actor) Action   { return Action{Kind: ActionNPCAttack, Actor: w.ID, Target: p.ID} }

// TestResolveIsPureAndDeterministic is ADR-003 p. 5 for the whole decision:
// the same address answers the same thing from a fresh rule set and fresh
// actors, and the actors it was handed are left as they were.
func TestResolveIsPureAndDeterministic(t *testing.T) {
	r := load(t)
	for _, kind := range []string{ActionAttack, ActionNPCAttack, ActionFreeAttack, ActionFlee, ActionRest} {
		t.Run(kind, func(t *testing.T) {
			for i := range 200 {
				cause := fmt.Sprintf("01JCPURE%04d", i)
				player, wolf := fighters(t, r)
				player.HP, wolf.HP = 1+i%10, 1+(i/10)%10
				action := attackOf(player, wolf)
				switch kind {
				case ActionNPCAttack, ActionFreeAttack:
					action = biteOf(player, wolf)
					action.Kind = kind
				case ActionFlee:
					action = Action{Kind: kind, Actor: player.ID, LivingEnemies: i % 3}
				case ActionRest:
					action = Action{Kind: kind, Actor: player.ID}
				}
				beforeP, beforeW := *player, *wolf

				first, firstRolls, err := r.Resolve(cause, i%4, action, actorsMap(player, wolf))
				if err != nil {
					t.Fatalf("resolve: %v", err)
				}
				if !reflect.DeepEqual(*player, beforeP) || !reflect.DeepEqual(*wolf, beforeW) {
					t.Fatalf("Resolve changed an actor: %+v %+v", *player, *wolf)
				}

				again := load(t)
				p2, w2 := fighters(t, again)
				p2.HP, w2.HP = player.HP, wolf.HP
				second, secondRolls, err := again.Resolve(cause, i%4, action, actorsMap(p2, w2))
				if err != nil {
					t.Fatalf("resolve again: %v", err)
				}
				if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(firstRolls, secondRolls) {
					t.Fatalf("%s: %+v %v then %+v %v", cause, first, firstRolls, second, secondRolls)
				}
				if first.HPAfter < 0 || first.HPAfter > 10 {
					t.Fatalf("hp_after %d outside [0, hp_max] (inv-02)", first.HPAfter)
				}
			}
		})
	}
}
