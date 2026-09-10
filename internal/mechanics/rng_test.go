package mechanics

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"multiverse-core.io/shared/entity"
)

// TestSeedVectors pins the seed function to numbers computed outside Go, from
// the definition alone: the first eight bytes of SHA-256 over "eventID:index",
// big endian (ADR-003 p. 5). A refactor that changes any of them changes every
// recorded run in the project, so the test is here to make that impossible to
// do by accident.
func TestSeedVectors(t *testing.T) {
	cases := []struct {
		eventID string
		index   int
		want    uint64
	}{
		{"", 0, 1866903987063215603},
		{"01JC0000000000000000000000", 0, 5566502161584001210},
		{"01JC0000000000000000000000", 1, 14095709468665957854},
		{"01JC0000000000000000000000", 3, 1256483486913892533},
		{"player.attacked", 0, 11954706955398637122},
		{"01JCZZZZZZZZZZZZZZZZZZZZZZ", 255, 17918835045097094771},
	}
	for _, c := range cases {
		if got := Seed(c.eventID, c.index); got != c.want {
			t.Errorf("Seed(%q, %d) = %d, want %d", c.eventID, c.index, got, c.want)
		}
	}
}

// TestSeedSeparatesIndices: the index is part of the address, and neighbouring
// indices of one event must not give neighbouring seeds — otherwise the four
// rolls of one turn would be correlated.
func TestSeedSeparatesIndices(t *testing.T) {
	const cause = "01JCTESTEVENT0000000000000"
	seen := map[uint64]int{}
	for i := range 1000 {
		seed := Seed(cause, i)
		if first, ok := seen[seed]; ok {
			t.Fatalf("indices %d and %d of one event share the seed %d", first, i, seed)
		}
		seen[seed] = i
	}
	// Adjacent indices differ in far more than the low bits: a hash, not a
	// counter.
	a, b := Seed(cause, 0), Seed(cause, 1)
	if bits := popcount(a ^ b); bits < 16 {
		t.Errorf("seeds of index 0 and 1 differ in %d bits, want a hash apart", bits)
	}
}

func popcount(v uint64) int {
	n := 0
	for ; v != 0; v &= v - 1 {
		n++
	}
	return n
}

// TestSeedSeparatesEvents: two events rolling their first die must not agree,
// or every turn of a session would play out the same way.
func TestSeedSeparatesEvents(t *testing.T) {
	seen := map[uint64]string{}
	for i := range 1000 {
		id := fmt.Sprintf("01JCEVENT%017d", i)
		seed := Seed(id, 0)
		if first, ok := seen[seed]; ok {
			t.Fatalf("events %s and %s share the seed %d", first, id, seed)
		}
		seen[seed] = id
	}
}

// TestRollIsReproducible is the promise a replay rests on: the same cause and
// the same index give the same dice, however many times and in whatever order
// they are asked for (NFR-060).
func TestRollIsReproducible(t *testing.T) {
	r := load(t)
	const cause = "01JCATTACK00000000000000000"

	first := make([]Roll, 4)
	for i := range first {
		roll, err := r.Roll(cause, i, "d20", PurposeHit)
		if err != nil {
			t.Fatalf("roll %d: %v", i, err)
		}
		first[i] = roll
	}

	// Backwards, on a second rule set, after a thousand unrelated rolls.
	other := load(t)
	for i := range 1000 {
		if _, err := other.Roll("01JCNOISE0000000000000000", i, "3d6+2", PurposeBackground); err != nil {
			t.Fatalf("noise: %v", err)
		}
	}
	for i := len(first) - 1; i >= 0; i-- {
		again, err := other.Roll(cause, i, "d20", PurposeHit)
		if err != nil {
			t.Fatalf("roll %d again: %v", i, err)
		}
		if again != first[i] {
			t.Errorf("roll %d = %+v, first time %+v", i, again, first[i])
		}
	}
}

// TestRollIndicesDiffer: four rolls of one turn must not be four copies of one
// number. The check is over a thousand causes, because any single turn may
// legitimately roll the same face twice.
func TestRollIndicesDiffer(t *testing.T) {
	r := load(t)

	identical := 0
	for i := range 1000 {
		cause := fmt.Sprintf("01JCTURN%018d", i)
		var results [4]int
		for idx := range results {
			roll, err := r.Roll(cause, idx, "d20", PurposeHit)
			if err != nil {
				t.Fatalf("roll: %v", err)
			}
			results[idx] = roll.Result
		}
		if results[0] == results[1] && results[1] == results[2] && results[2] == results[3] {
			identical++
		}
	}
	// Four d20 agreeing by chance happens about once in 8000 turns; a broken
	// index would make it happen every time.
	if identical > 5 {
		t.Errorf("%d turns of 1000 rolled the same number four times: the index is not reaching the seed", identical)
	}
}

// TestRollDistribution is the sanity check on the generator itself: a d20 over
// a thousand causes covers every face, and the mean sits near 10.5. It is not a
// statistics suite — it is the assertion that would fail if the roll silently
// stopped using the sides of the die.
func TestRollDistribution(t *testing.T) {
	r := load(t)

	counts := make([]int, 21)
	sum := 0
	const n = 2000
	for i := range n {
		roll, err := r.Roll(fmt.Sprintf("01JCDIST%018d", i), 0, "d20", PurposeHit)
		if err != nil {
			t.Fatalf("roll: %v", err)
		}
		if roll.Result < 1 || roll.Result > 20 {
			t.Fatalf("d20 rolled %d", roll.Result)
		}
		counts[roll.Result]++
		sum += roll.Result
	}
	for face := 1; face <= 20; face++ {
		if counts[face] == 0 {
			t.Errorf("face %d never came up in %d rolls", face, n)
		}
	}
	if mean := float64(sum) / n; math.Abs(mean-10.5) > 1 {
		t.Errorf("mean of a d20 is %.2f, want about 10.5", mean)
	}
}

// TestRollNdMPlusK: several dice of one roll are drawn from one generator, in
// order, and the natural is the first of them — the rule the whole audit trail
// of a fight rests on (§5.5).
func TestRollNdMPlusK(t *testing.T) {
	r := load(t)
	const cause = "01JCMULTI0000000000000000"

	roll, err := r.Roll(cause, 0, "3d6+2", PurposeDamage)
	if err != nil {
		t.Fatalf("roll: %v", err)
	}
	if roll.Formula != "3d6+2" {
		t.Errorf("formula %q, want 3d6+2", roll.Formula)
	}
	if roll.Result < 5 || roll.Result > 20 {
		t.Errorf("3d6+2 rolled %d, outside [5, 20]", roll.Result)
	}
	if roll.Natural < 1 || roll.Natural > 6 {
		t.Errorf("natural %d is not a face of a d6", roll.Natural)
	}

	// The same generator, drawn by hand: the sum is the three dice in order
	// plus the modifier, and nothing is drawn before them.
	rng := NewRNG(Seed(cause, 0))
	want := 2
	for i := range 3 {
		die := 1 + rng.IntN(6)
		if i == 0 && die != roll.Natural {
			t.Errorf("natural %d, the first die of the generator is %d", roll.Natural, die)
		}
		want += die
	}
	if roll.Result != want {
		t.Errorf("result %d, the generator gives %d", roll.Result, want)
	}
}

// TestRollRejects covers the arguments a caller can get wrong. A roll with no
// cause is a roll nothing can reproduce, and it must not be quietly accepted.
func TestRollRejects(t *testing.T) {
	r := load(t)
	cases := map[string]struct {
		cause   string
		index   int
		formula string
		purpose string
	}{
		"no cause":         {"", 0, "d20", PurposeHit},
		"negative index":   {"01JC", -1, "d20", PurposeHit},
		"unknown purpose":  {"01JC", 0, "d20", "vibes"},
		"formula not dice": {"01JC", 0, "twenty", PurposeHit},
		"one-sided die":    {"01JC", 0, "d1", PurposeHit},
		"no dice at all":   {"01JC", 0, "0d6", PurposeHit},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := r.Roll(c.cause, c.index, c.formula, c.purpose); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

// TestRollCheck rolls a whole check and reports both halves: the roll as
// dice.rolled carries it and the arithmetic combat.decided rests on.
func TestRollCheck(t *testing.T) {
	r := load(t)
	hit, _ := r.Check(CheckAttackHit)
	player, _ := r.Stats("player")
	wolf, _ := r.Stats("wolf")
	player.ID, wolf.ID = "player-A", "wolf-alpha"

	roll, res, err := r.RollCheck("01JCCHECK0000000000000000", 0, hit, PurposeHit, player, wolf, nil)
	if err != nil {
		t.Fatalf("roll check: %v", err)
	}
	if roll.Result != roll.Natural {
		t.Errorf("a single d20 rolled %d with a natural of %d", roll.Result, roll.Natural)
	}
	if res.Total != res.Roll+player.Atk {
		t.Errorf("total %d, want the roll %d plus atk %d", res.Total, res.Roll, player.Atk)
	}
	if res.Threshold != wolf.Def {
		t.Errorf("threshold %d, want the def of the wolf %d", res.Threshold, wolf.Def)
	}
	if res.OK != (res.Total >= res.Threshold) {
		t.Errorf("check says %v for %d >= %d", res.OK, res.Total, res.Threshold)
	}

	// The roll of a check is addressed exactly like a plain roll: same cause,
	// same index, same dice.
	plain, err := r.Roll("01JCCHECK0000000000000000", 0, "d20", PurposeHit)
	if err != nil {
		t.Fatalf("plain roll: %v", err)
	}
	if plain.Result != roll.Result || plain.Seed != roll.Seed {
		t.Errorf("check roll %+v differs from the plain roll %+v of the same address", roll, plain)
	}
}

// TestRollCheckRejects: a formula reading something nobody can answer for is an
// error, not a zero. A flight bonus that silently reads as zero turns a rule
// into a different rule.
func TestRollCheckRejects(t *testing.T) {
	r := load(t)
	flee, _ := r.Check(CheckFlee)
	wolf, _ := r.Stats("wolf") // the wolf does not run: no flee at all
	player, _ := r.Stats("player")

	_, _, err := r.RollCheck("01JC", 0, flee, PurposeFlee, wolf, player,
		map[string]int{IdentLivingEnemies: 1})
	if err == nil {
		t.Fatal("rolled a flight check for an actor with no flee bonus")
	}
	if !strings.Contains(err.Error(), IdentFlee) {
		t.Errorf("error %v does not name the identifier it could not resolve", err)
	}

	// living_enemies has to be supplied by the turn; without it the threshold
	// is unknown rather than ten.
	if _, _, err := r.RollCheck("01JC", 0, flee, PurposeFlee, player, wolf, nil); err == nil {
		t.Fatal("rolled a flight check without living_enemies")
	}

	if _, _, err := r.RollCheck("", 0, flee, PurposeFlee, player, wolf, nil); err == nil {
		t.Fatal("rolled a check with no cause event")
	}
	if _, _, err := r.RollCheck("01JC", -1, flee, PurposeFlee, player, wolf, nil); err == nil {
		t.Fatal("rolled a check at a negative index")
	}
	if _, _, err := r.RollCheck("01JC", 0, flee, "vibes", player, wolf, nil); err == nil {
		t.Fatal("rolled a check for an unknown purpose")
	}
}

// TestGoldenRolls is the recording contract of the platform: literal numbers,
// not numbers this package recomputes for itself.
//
// These five rows are what the dice actually said on the current
// implementation, recomputed outside Go from the definitions alone — SHA-256
// over "eventID:index" big endian for the address, PCG-DXSM seeded (seed, 0)
// drawn through IntN for the stream. Changing any of them invalidates every
// run ever recorded by the project: replay compares a fresh roll with the one
// in the journal, and events_hash_match / dice_rolled_new = 0 (C-14, §5.6,
// NFR-060/061) are exactly this table holding.
//
// So: if this test fails, the generator changed. That is a decision about
// stored data, never a test to be re-baselined.
//
// The row at index 2 is here for its seed alone: 1898231145079728858 appears
// in no other test, so the row exercises Seed and the stream in one assertion.
// The wire seed is checked with them, because a roll nobody can address in the
// journal is a roll nobody can replay.
func TestGoldenRolls(t *testing.T) {
	r := load(t)
	roller := entity.Ref{ID: "player-A", Type: entity.TypePlayer}

	cases := []struct {
		cause    string
		index    int
		formula  string // as a caller writes it
		wantForm string // as Roll canonicalises it onto the wire
		wantSeed uint64
		wantRes  int
		wantNat  int
	}{
		{"01JC0000000000000000000000", 0, "d20", "d20", 5566502161584001210, 3, 3},
		{"01JC0000000000000000000000", 1, "1d6", "d6", 14095709468665957854, 4, 4},
		{"01JC0000000000000000000000", 2, "d20", "d20", 1898231145079728858, 6, 6},
		{"01JC0000000000000000000000", 3, "d4", "d4", 1256483486913892533, 2, 2},
		{"player.attacked", 0, "3d6+2", "3d6+2", 11954706955398637122, 9, 3},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s#%d %s", c.cause, c.index, c.formula), func(t *testing.T) {
			roll, err := r.Roll(c.cause, c.index, c.formula, PurposeHit)
			if err != nil {
				t.Fatalf("roll: %v", err)
			}
			if roll.Seed != c.wantSeed {
				t.Errorf("seed %d, recorded %d", roll.Seed, c.wantSeed)
			}
			if roll.Result != c.wantRes || roll.Natural != c.wantNat {
				t.Errorf("%s rolled %d (natural %d), recorded %d (natural %d): the stream of the generator changed",
					c.formula, roll.Result, roll.Natural, c.wantRes, c.wantNat)
			}
			// The formula travels in the canonical form of DiceExpr.String(),
			// not in the form the caller happened to type (Minor-6).
			if roll.Formula != c.wantForm {
				t.Errorf("formula on the wire %q, recorded %q", roll.Formula, c.wantForm)
			}
			seed, _ := DiceRolledPayload(roll, roller)["roll"].(map[string]any)["seed"].(string)
			if want := strconv.FormatUint(c.wantSeed, 10); seed != want {
				t.Errorf("dice.rolled carries the seed as %q, recorded %q", seed, want)
			}
		})
	}
}
