package mechanics

import (
	"fmt"
	"math"
	"testing"
)

// The statistical half of NFR-060 (T-051 DoD): a roll is one number per
// address, the numbers of one face are equally likely, and the rolls of one
// cause at different indices show no linear dependence (Pearson). Pearson does
// not prove independence: a roll that is a fixed permutation of the faces of
// the previous one keeps |r| small and passes (see TestRollIndicesUncorrelated).
//
// Every test here is deterministic: the causes are fixed strings, the generator
// has no state outside its seed, so a statistic that passes today passes on
// every run and every toolchain. On this sample of causes the thresholds are
// not a matter of flakiness — they are the point past which the generator is
// called broken. Change the sample (the prefix or the number of causes in
// statCause) and a sound generator turns one of these checks red with a
// probability of about 1–1.5 % (five χ² checks at p = 0.001, six |r| checks at
// p ≈ 0.0015 each): after such a change, try a neighbouring sample before
// suspecting the generator.

// turnRolls is the shape of one solo turn (state-and-mechanics.md §5.5): the
// hit and damage of the player, then the hit and damage of the NPC, at indices
// 0..3 of one cause.
var turnRolls = [4]struct {
	formula string
	purpose string
}{
	{"d20", PurposeHit},
	{"2d6+1", PurposeDamage},
	{"d20", PurposeNPCHit},
	{"d4", PurposeNPCDamage},
}

func statCause(prefix string, i int) string { return fmt.Sprintf("01JC%s%016d", prefix, i) }

// TestRollReproducibleThousandCausesTwoOrders: 1000 causes × 4 rolls, asked for
// twice — once cause by cause in index order, once index by index from the last
// cause back, on a second rule set — must agree in 100 % of the 4000 rolls.
// Order must not matter because replay asks for rolls in whatever order the
// recorded turns are re-run, and a generator that carries anything between
// rolls would answer differently.
func TestRollReproducibleThousandCausesTwoOrders(t *testing.T) {
	const causes = 1000

	forward := load(t)
	first := make([][4]Roll, causes)
	for c := range causes {
		for idx, tr := range turnRolls {
			roll, err := forward.Roll(statCause("REPRO", c), idx, tr.formula, tr.purpose)
			if err != nil {
				t.Fatalf("cause %d roll %d: %v", c, idx, err)
			}
			first[c][idx] = roll
		}
	}

	backward := load(t)
	matched, total := 0, 0
	for idx := len(turnRolls) - 1; idx >= 0; idx-- {
		tr := turnRolls[idx]
		for c := causes - 1; c >= 0; c-- {
			again, err := backward.Roll(statCause("REPRO", c), idx, tr.formula, tr.purpose)
			if err != nil {
				t.Fatalf("cause %d roll %d again: %v", c, idx, err)
			}
			total++
			if again == first[c][idx] {
				matched++
				continue
			}
			if total-matched <= 5 {
				t.Errorf("cause %d roll %d = %+v, first pass %+v", c, idx, again, first[c][idx])
			}
		}
	}
	if total != causes*len(turnRolls) || matched != total {
		t.Fatalf("%d of %d rolls matched across the two passes, want all %d", matched, total, causes*len(turnRolls))
	}
}

// chiSquareD20Critical is the upper critical value of χ² with 19 degrees of
// freedom at p = 0.001. A fair d20 exceeds it once in a thousand samples; the
// samples here are fixed, so the test is either always green or always red —
// but a new sample of causes may land in that one-in-a-thousand on a sound
// generator (see the note at the top of the file).
const chiSquareD20Critical = 43.82

// TestRollD20ChiSquare: the faces of a d20 are equally likely. The check runs on
// each index of a turn separately (2500 causes each, n ≥ 2000) and on all of
// them pooled (n = 10000), because a bias could sit on one index alone.
func TestRollD20ChiSquare(t *testing.T) {
	r := load(t)
	const causes = 2500

	pooled := make([]int, 21)
	for idx := range turnRolls {
		counts := make([]int, 21)
		for c := range causes {
			roll, err := r.Roll(statCause("CHISQ", c), idx, "d20", PurposeHit)
			if err != nil {
				t.Fatalf("cause %d roll %d: %v", c, idx, err)
			}
			if roll.Result < 1 || roll.Result > 20 {
				t.Fatalf("d20 rolled %d", roll.Result)
			}
			counts[roll.Result]++
			pooled[roll.Result]++
		}
		stat := chiSquareUniform(counts[1:])
		t.Logf("index %d: n=%d χ²=%.2f", idx, causes, stat)
		if stat >= chiSquareD20Critical {
			t.Errorf("index %d: χ² = %.2f over %d rolls, critical %.2f (df=19, p=0.001): faces %v are not uniform",
				idx, stat, causes, chiSquareD20Critical, counts[1:])
		}
	}

	stat := chiSquareUniform(pooled[1:])
	t.Logf("pooled: n=%d χ²=%.2f", causes*len(turnRolls), stat)
	if stat >= chiSquareD20Critical {
		t.Errorf("pooled: χ² = %.2f over %d rolls, critical %.2f (df=19, p=0.001): faces %v are not uniform",
			stat, causes*len(turnRolls), chiSquareD20Critical, pooled[1:])
	}
}

// TestRollIndicesUncorrelated: over 1000 causes the d20 at one index shows no
// linear dependence on the d20 at another. |r| of Pearson under 0.1 is about
// three standard errors (1/√1000 ≈ 0.032) away from zero correlation. The pair
// 0–1 is the one the DoD names — the hit of the player and the roll right after
// it — and the other pairs of the turn are held to the same bound.
//
// This is not a test of independence: Pearson sees only a linear link. A roll
// at index 1 computed as a fixed permutation of the faces at index 0 (reviewer
// mutant P11, r(0,1) ≈ −0.08) keeps both marginals uniform and passes here.
func TestRollIndicesUncorrelated(t *testing.T) {
	r := load(t)
	const causes = 1000
	const maxAbsR = 0.1

	results := make([][4]float64, causes)
	for c := range causes {
		for idx := range turnRolls {
			roll, err := r.Roll(statCause("CORR", c), idx, "d20", PurposeHit)
			if err != nil {
				t.Fatalf("cause %d roll %d: %v", c, idx, err)
			}
			results[c][idx] = float64(roll.Result)
		}
	}

	for a := range turnRolls {
		for b := a + 1; b < len(turnRolls); b++ {
			xs, ys := make([]float64, causes), make([]float64, causes)
			for c := range causes {
				xs[c], ys[c] = results[c][a], results[c][b]
			}
			rho := pearson(xs, ys)
			t.Logf("indices %d–%d: r=%+.4f", a, b, rho)
			if math.IsNaN(rho) || math.Abs(rho) >= maxAbsR {
				t.Errorf("indices %d and %d: Pearson r = %+.4f over %d causes, want |r| < %.1f: one roll follows the other linearly",
					a, b, rho, causes, maxAbsR)
			}
		}
	}
}

// TestStatisticsHelpers pins the two helpers against numbers worked by hand, so
// a green statistical test cannot be a broken formula saying yes.
func TestStatisticsHelpers(t *testing.T) {
	uniform := []int{5, 5, 5, 5}
	if got := chiSquareUniform(uniform); got != 0 {
		t.Errorf("χ² of a flat histogram = %v, want 0", got)
	}
	// 20 rolls all on one of four faces, 5 expected per face:
	// (20-5)²/5 + 3·(0-5)²/5 = 45 + 15 = 60.
	if got := chiSquareUniform([]int{20, 0, 0, 0}); math.Abs(got-60) > 1e-9 {
		t.Errorf("χ² of a single-face histogram = %v, want 60", got)
	}

	xs := []float64{1, 2, 3, 4, 5}
	if got := pearson(xs, []float64{2, 4, 6, 8, 10}); math.Abs(got-1) > 1e-12 {
		t.Errorf("r of a line = %v, want 1", got)
	}
	if got := pearson(xs, []float64{5, 4, 3, 2, 1}); math.Abs(got+1) > 1e-12 {
		t.Errorf("r of a falling line = %v, want -1", got)
	}
	// x = 1..5, y = 2,1,4,3,5: Σdx·dy = 8, Σdx² = 10, Σdy² = 10 → r = 0.8.
	if got := pearson(xs, []float64{2, 1, 4, 3, 5}); math.Abs(got-0.8) > 1e-12 {
		t.Errorf("r = %v, want 0.8", got)
	}
	if got := pearson(xs, []float64{3, 3, 3, 3, 3}); !math.IsNaN(got) {
		t.Errorf("r against a constant = %v, want NaN: no variance, no correlation to speak of", got)
	}
}

// chiSquareUniform is Pearson's χ² of a histogram against the uniform
// distribution over its bins.
func chiSquareUniform(counts []int) float64 {
	n := 0
	for _, c := range counts {
		n += c
	}
	expected := float64(n) / float64(len(counts))
	stat := 0.0
	for _, c := range counts {
		d := float64(c) - expected
		stat += d * d / expected
	}
	return stat
}

// pearson is the sample correlation coefficient. A constant series has no
// variance and gives NaN, which the caller treats as a failure: a roll that
// never changes is not independent of anything, it is broken.
func pearson(xs, ys []float64) float64 {
	n := float64(len(xs))
	var sx, sy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
	}
	mx, my := sx/n, sy/n
	var cov, vx, vy float64
	for i := range xs {
		dx, dy := xs[i]-mx, ys[i]-my
		cov += dx * dy
		vx += dx * dx
		vy += dy * dy
	}
	return cov / math.Sqrt(vx*vy)
}
