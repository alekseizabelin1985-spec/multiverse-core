package mechanics

import (
	"strings"
	"testing"

	"multiverse-core.io/shared/entity"
)

func TestParseDice(t *testing.T) {
	cases := map[string]struct {
		want   DiceExpr
		render string
	}{
		"d20":   {DiceExpr{Count: 1, Sides: 20}, "d20"},
		"d6":    {DiceExpr{Count: 1, Sides: 6}, "d6"},
		"2d6":   {DiceExpr{Count: 2, Sides: 6}, "2d6"},
		"d6+1":  {DiceExpr{Count: 1, Sides: 6, Modifier: 1}, "d6+1"},
		"3d6+2": {DiceExpr{Count: 3, Sides: 6, Modifier: 2}, "3d6+2"},
		"2d4-1": {DiceExpr{Count: 2, Sides: 4, Modifier: -1}, "2d4-1"},
		" d8 ":  {DiceExpr{Count: 1, Sides: 8}, "d8"},
	}
	for text, c := range cases {
		t.Run(text, func(t *testing.T) {
			got, err := ParseDice(text)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got != c.want {
				t.Errorf("parsed %+v, want %+v", got, c.want)
			}
			if got.String() != c.render {
				t.Errorf("rendered %q, want %q", got.String(), c.render)
			}
			// What is rendered parses back into the same expression: the string
			// travels in dice.rolled.formula and has to survive the round trip.
			again, err := ParseDice(got.String())
			if err != nil || again != got {
				t.Errorf("round trip of %q gave %+v, %v", got, again, err)
			}
		})
	}
}

func TestParseDiceRejects(t *testing.T) {
	for _, text := range []string{
		"", "   ", "20", "d", "dd6", "d0", "d1", "0d6", "-d6", "-2d6",
		"2d6+", "2d6+x", "x d6", "d6+1+2", "d 6", "２d６",
		// Nothing without an upper end: 200000000d6 used to load in silence
		// and then spend 466 ms inside one turn.
		"1001d6", "200000000d6", "d1001", "d6+1001", "d6-1001",
	} {
		t.Run(text, func(t *testing.T) {
			if got, err := ParseDice(text); err == nil {
				t.Fatalf("parsed %q as %+v", text, got)
			}
		})
	}
}

func TestDiceRange(t *testing.T) {
	cases := map[string]struct{ min, max int }{
		"d20":   {1, 20},
		"3d6+2": {5, 20},
		"2d4-1": {1, 7},
		"d6-10": {-9, -4},
	}
	for text, c := range cases {
		d, err := ParseDice(text)
		if err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		if d.Min() != c.min || d.Max() != c.max {
			t.Errorf("%s ranges [%d, %d], want [%d, %d]", text, d.Min(), d.Max(), c.min, c.max)
		}
	}
}

// TestDiceRollStaysInRange rolls every expression of the rule set a thousand
// times over addressed seeds and checks it never leaves its own range.
func TestDiceRollStaysInRange(t *testing.T) {
	for _, text := range []string{"d20", "d6", "d4", "3d6+2", "2d4-1"} {
		d, err := ParseDice(text)
		if err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		t.Run(text, func(t *testing.T) {
			for i := range 1000 {
				result, natural := d.Roll(NewRNG(Seed("01JCRANGE", i)))
				if result < d.Min() || result > d.Max() {
					t.Fatalf("%s rolled %d, outside [%d, %d]", text, result, d.Min(), d.Max())
				}
				if natural < 1 || natural > d.Sides {
					t.Fatalf("%s gave a natural of %d", text, natural)
				}
			}
		})
	}
}

func TestParseCheck(t *testing.T) {
	c, err := ParseCheck("d20 + atk >= def")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Dice != (DiceExpr{Count: 1, Sides: 20}) {
		t.Errorf("dice %+v", c.Dice)
	}
	if len(c.Left) != 1 || c.Left[0].Ident != IdentAtk {
		t.Errorf("left %+v, want [atk]", c.Left)
	}
	if len(c.Right) != 1 || c.Right[0].Ident != IdentDef {
		t.Errorf("right %+v, want [def]", c.Right)
	}

	c, err = ParseCheck("d20 + flee >= 10 + living_enemies")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Right) != 2 || c.Right[0].Const != 10 || c.Right[1].Ident != IdentLivingEnemies {
		t.Errorf("right %+v, want [10 living_enemies]", c.Right)
	}
	if c.String() != "d20 + flee >= 10 + living_enemies" {
		t.Errorf("rendered %q", c.String())
	}
}

func TestParseCheckRejects(t *testing.T) {
	for _, text := range []string{
		"",
		"d20 + atk > def",
		"d20 + atk = def",
		"atk >= def",
		"d20 + atk >=",
		">= def",
		"d20 + >= def",
		"d20 + Atk >= def",
		"d20 + atk-1 >= def",
		"d20 + atk >= def >= 2",
		// A constant no turn can reach. Unbounded, it wrapped the threshold
		// negative and the check passed always — the opposite of what the
		// rule says, accepted in silence.
		"d20 >= 9223372036854775807 + living_enemies",
		"d20 + atk >= def + 9223372036854775807",
		"d20 + atk >= 1001",
		"d20 + atk >= -1001 + def",
	} {
		t.Run(text, func(t *testing.T) {
			if got, err := ParseCheck(text); err == nil {
				t.Fatalf("parsed %q as %+v", text, got)
			}
		})
	}
}

// TestCheckConstBounds: the constant addend of a check is bounded like every
// other number of the grammar, and the bound is applied by Load, not left to
// the caller — an expression may arrive from a blueprint (ADR-012 p. 2).
func TestCheckConstBounds(t *testing.T) {
	for _, text := range []string{"d20 + atk >= 1000", "d20 + atk >= -1000 + def"} {
		if _, err := ParseCheck(text); err != nil {
			t.Errorf("ParseCheck(%q) = %v, want a check", text, err)
		}
	}

	// Through Load, where the rules file is read: the same refusal, named.
	text, err := readRules()
	if err != nil {
		t.Fatalf("read %s: %v", rulesPath, err)
	}
	broken := strings.Replace(text, "d20 + atk >= def", "d20 + atk >= def + 9223372036854775807", 1)
	if broken == text {
		t.Fatal("the attack check is no longer written as the test expects: fix the test, not the rules")
	}
	if _, err := LoadBytes([]byte(broken)); err == nil {
		t.Fatal("Load accepted a check whose threshold no turn can reach")
	} else if !strings.Contains(err.Error(), "at most 1000") {
		t.Errorf("Load error %v, want the bound named", err)
	}
}

// TestHitTable is the attack rule of appendix A read off a fixed roll: a
// character with atk 2 against a wolf with def 11 hits on a natural 9 and
// misses on an 8, and the two natural faces override the sum in both
// directions (domain-review §3.3).
func TestHitTable(t *testing.T) {
	r := load(t)
	hit, _ := r.Check(CheckAttackHit)
	player, _ := r.Stats("player")
	wolf, _ := r.Stats("wolf")

	cases := []struct {
		natural int
		total   int
		sum     bool // the sum alone makes the threshold
		crit    bool
		fumble  bool
	}{
		{natural: 1, total: 3, sum: false, fumble: true},
		{natural: 8, total: 10, sum: false},
		{natural: 9, total: 11, sum: true},
		{natural: 10, total: 12, sum: true},
		{natural: 19, total: 21, sum: true},
		{natural: 20, total: 22, sum: true, crit: true},
	}
	for _, c := range cases {
		total := c.natural + player.Atk
		if total != c.total {
			t.Errorf("natural %d plus atk %d is %d, want %d", c.natural, player.Atk, total, c.total)
		}
		if got := total >= wolf.Def; got != c.sum {
			t.Errorf("natural %d: sum %d against def %d says %v, want %v", c.natural, total, wolf.Def, got, c.sum)
		}
		if got := r.Critical(c.natural); got != c.crit {
			t.Errorf("natural %d: critical %v, want %v", c.natural, got, c.crit)
		}
		if got := r.Fumble(c.natural); got != c.fumble {
			t.Errorf("natural %d: fumble %v, want %v", c.natural, got, c.fumble)
		}
	}

	// The same numbers through the compiled formula, on a real roll.
	res, err := hit.Eval(NewRNG(Seed("01JCHIT", 0)), player, wolf, nil)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if res.Total != res.Natural+player.Atk || res.Threshold != wolf.Def {
		t.Errorf("eval gave %+v for atk %d against def %d", res, player.Atk, wolf.Def)
	}
	if res.OK != (res.Total >= res.Threshold) {
		t.Errorf("eval says %v for %d >= %d", res.OK, res.Total, res.Threshold)
	}
}

// TestDamageTable: a character deals d6, a wolf d4, and a critical doubles
// whatever came up. Damage is never negative, whatever the multiplier does.
func TestDamageTable(t *testing.T) {
	r := load(t)
	player, _ := r.Stats("player")
	wolf, _ := r.Stats("wolf")

	for _, c := range []struct {
		actor Actor
		dice  string
	}{{player, "d6"}, {wolf, "d4"}} {
		got, err := r.DamageDice(c.actor)
		if err != nil {
			t.Fatalf("damage dice: %v", err)
		}
		if got.String() != c.dice {
			t.Errorf("%s deals %s, want %s", c.actor.Type, got, c.dice)
		}
	}

	cases := []struct {
		rolled   int
		critical bool
		want     int
	}{
		{rolled: 1, want: 1},
		{rolled: 6, want: 6},
		{rolled: 1, critical: true, want: 2},
		{rolled: 6, critical: true, want: 12},
		{rolled: 0, want: 0},
		{rolled: -3, want: 0},
		{rolled: -3, critical: true, want: 0},
	}
	for _, c := range cases {
		if got := r.Damage(c.rolled, c.critical); got != c.want {
			t.Errorf("damage of %d (critical %v) = %d, want %d", c.rolled, c.critical, got, c.want)
		}
	}

	// An attacker with no dice at all is a defect of whoever built it, not a
	// zero-damage attack.
	if _, err := r.DamageDice(Actor{ID: "ghost", Type: entity.TypeNPC}); err == nil {
		t.Error("an actor with no dmg deals damage")
	}
	if _, err := r.DamageDice(Actor{ID: "ghost", Type: entity.TypeNPC, Dmg: "fists"}); err == nil {
		t.Error("an actor with an unparsable dmg deals damage")
	}
}

// TestFleeTable is the flight rule: d20 + flee against 10 plus the enemies
// still standing. One wolf is a threshold of 11, three wolves of 13.
func TestFleeTable(t *testing.T) {
	r := load(t)
	flee, _ := r.Check(CheckFlee)
	player, _ := r.Stats("player")
	wolf, _ := r.Stats("wolf")
	player.ID = "player-A"

	for enemies, want := range map[int]int{0: 10, 1: 11, 2: 12, 3: 13} {
		got, err := r.FleeThreshold(wolf, enemies)
		if err != nil {
			t.Fatalf("threshold: %v", err)
		}
		if got != want {
			t.Errorf("%d enemies: threshold %d, want %d", enemies, got, want)
		}
	}

	// The threshold the check evaluates and the one FleeThreshold reports are
	// the same number: combat.decided shows the player what the roll was
	// against.
	res, err := flee.Eval(NewRNG(Seed("01JCFLEE", 0)), player, wolf,
		map[string]int{IdentLivingEnemies: 2})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if res.Threshold != 12 {
		t.Errorf("threshold %d, want 12", res.Threshold)
	}
	if res.Total != res.Natural+2 {
		t.Errorf("total %d, want the natural %d plus the flee bonus 2", res.Total, res.Natural)
	}

	// The wolf does not run: the rule has nothing to read for it.
	if _, err := flee.Eval(NewRNG(1), wolf, player, map[string]int{IdentLivingEnemies: 1}); err == nil {
		t.Error("the wolf made a flight check")
	}
}

// TestFleePosition: a successful escape leads out of the world the character is
// in, not to a literal template.
func TestFleePosition(t *testing.T) {
	r := load(t)
	got := r.FleePosition("dark-forest-world", "dark-forest-01")
	if got != "outside:dark-forest-world" {
		t.Errorf("position %q, want outside:dark-forest-world", got)
	}
	if strings.Contains(got, "{") {
		t.Errorf("position %q still holds a placeholder", got)
	}
}

// TestRestore: a rest brings a wounded character back to full and leaves a
// healthy one where it is — the only rest the rules admit (rest.restore).
func TestRestore(t *testing.T) {
	r := load(t)
	player, _ := r.Stats("player")
	player.ID = "player-A"

	for _, hp := range []int{0, 3, player.HPMax} {
		player.HP = hp
		if got := r.Restore(player); got != player.HPMax {
			t.Errorf("rest from %d hp left %d, want %d", hp, got, player.HPMax)
		}
	}
}

// TestClampHP is inv-02 as the mechanics enforce it: hit points never leave
// [0, hp_max], whichever end the arithmetic overshot.
func TestClampHP(t *testing.T) {
	r := load(t)
	cases := []struct{ hp, hpMax, want int }{
		{hp: 7, hpMax: 10, want: 7},
		{hp: 0, hpMax: 10, want: 0},
		{hp: -4, hpMax: 10, want: 0},
		{hp: 10, hpMax: 10, want: 10},
		{hp: 14, hpMax: 10, want: 10},
	}
	for _, c := range cases {
		if got := r.ClampHP(c.hp, c.hpMax); got != c.want {
			t.Errorf("clamp(%d, %d) = %d, want %d", c.hp, c.hpMax, got, c.want)
		}
	}
}

// TestExcluded is inv-01 as the rules state it: the dead do not act and are not
// targets, and an abandoned character is out of the fight in exactly the same
// way (C-02 v1.2).
func TestExcluded(t *testing.T) {
	r := load(t)
	cases := map[string]struct {
		status        string
		participation string
		want          bool
	}{
		"alive and fighting":  {entity.StatusAlive, entity.ParticipationActive, false},
		"alive, no state set": {entity.StatusAlive, "", false},
		"idle":                {entity.StatusAlive, entity.ParticipationIdle, true},
		"out of combat":       {entity.StatusAlive, entity.ParticipationOutOfCombat, true},
		"dead":                {entity.StatusDead, entity.ParticipationActive, true},
		"abandoned":           {entity.StatusAbandoned, entity.ParticipationActive, true},
		"ascended":            {entity.StatusAscendedFinal, entity.ParticipationActive, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := Actor{ID: "player-A", Type: entity.TypePlayer, Status: c.status, Participation: c.participation}
			if got := r.Excluded(a); got != c.want {
				t.Errorf("excluded %v, want %v", got, c.want)
			}
			if got := a.Alive(); got != (c.status == entity.StatusAlive) {
				t.Errorf("alive %v for status %q", got, c.status)
			}
		})
	}
}

// TestActorAttr covers the identifier resolution a formula rests on, including
// the flee bonus written with a sign and the one that is not there at all.
func TestActorAttr(t *testing.T) {
	a := Actor{ID: "player-A", HP: 4, HPMax: 10, Atk: 2, Def: 12, Flee: "+2"}
	for name, want := range map[string]int{
		IdentHP: 4, IdentHPMax: 10, IdentAtk: 2, IdentDef: 12, IdentFlee: 2,
	} {
		got, ok := a.Attr(name)
		if !ok || got != want {
			t.Errorf("%s = %d (%v), want %d", name, got, ok, want)
		}
	}
	if _, ok := a.Attr("luck"); ok {
		t.Error("an actor answered for an identifier the grammar does not have")
	}
	if _, ok := (Actor{}).Attr(IdentFlee); ok {
		t.Error("an actor with no flee bonus answered for flee")
	}
	if _, ok := (Actor{Flee: "swiftly"}).Attr(IdentFlee); ok {
		t.Error("an unreadable flee bonus resolved")
	}
}

// TestDiceBounds pins where the grammar stops. A formula reaches the parser
// from a blueprint as readily as from a file under review, so the limit is part
// of what Load checks and not something a caller is trusted to keep: the
// expression that parses is the expression that can be rolled inside a turn and
// whose range still fits the arithmetic of a fight.
func TestDiceBounds(t *testing.T) {
	widest, err := ParseDice("1000d1000+1000")
	if err != nil {
		t.Fatalf("1000d1000+1000 is the widest expression allowed: %v", err)
	}
	if widest.Max() != 1000*1000+1000 {
		t.Errorf("the widest expression tops out at %d", widest.Max())
	}
	for _, text := range []string{"1001d1000", "1000d1001", "1000d1000+1001", "1000d1000-1001"} {
		if _, err := ParseDice(text); err == nil {
			t.Errorf("parsed %q: one step past the bound is still past it", text)
		}
	}
}
