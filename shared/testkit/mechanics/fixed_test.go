package mechanics_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	mech "multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	fixed "multiverse-core.io/shared/testkit/mechanics"
)

// rulesPath is the one rule set of MVP-1, read from the repository so that the
// stub is tested against the numbers a run actually plays with.
func rulesPath() string { return filepath.Join("..", "..", "..", "rules", "dark-forest.yaml") }

func load(t *testing.T) *fixed.FixedMechanics {
	t.Helper()
	m, err := fixed.Load(rulesPath())
	if err != nil {
		t.Fatalf("load %s: %v", rulesPath(), err)
	}
	return m
}

// actorsOf builds the two fighters of the dark forest out of the rules, so
// that no test here repeats a number the rules already state.
func actorsOf(t *testing.T, m *fixed.FixedMechanics) (player, wolf *mech.Actor) {
	t.Helper()
	p, ok := m.Stats("player")
	if !ok {
		t.Fatal("the rules describe no player")
	}
	w, ok := m.Stats("wolf")
	if !ok {
		t.Fatal("the rules describe no wolf")
	}
	p.ID, p.Type = "player-A", entity.TypePlayer
	w.ID, w.Type = "wolf-alpha", entity.TypeNPC
	return &p, &w
}

func actorMap(as ...*mech.Actor) map[string]*mech.Actor {
	out := make(map[string]*mech.Actor, len(as))
	for _, a := range as {
		out[a.ID] = a
	}
	return out
}

// causeFor finds a cause event whose first roll lands on the wanted verdict.
// A test that wants to see a critical says so instead of hunting for a magic
// string, and the search itself proves the table is reachable in every row.
func causeFor(t *testing.T, verdict string, index int) string {
	t.Helper()
	for i := range 1000 {
		id := "ev-" + strconv.Itoa(i)
		if fixed.Verdict(id, index) == verdict {
			return id
		}
	}
	t.Fatalf("no cause event out of a thousand lands on %q at index %d", verdict, index)
	return ""
}

// --- the table itself ---

// TestOutcomeTableHasTheSharesOfTheDesign counts the rows of the table over
// the seeds of ten thousand cause events. The shares are exact rather than
// approximate: the slot is the seed modulo ten and every slot is one verdict,
// so a run that drifts from 60/10/10/20 has had its table edited, not its luck
// turn (design.md §5).
func TestOutcomeTableHasTheSharesOfTheDesign(t *testing.T) {
	const runs = 10000
	counts := map[string]int{}
	for i := range runs {
		counts[fixed.Verdict("ev-"+strconv.Itoa(i), 0)]++
	}

	// The shares hold over the seeds only if SHA-256 spreads them evenly, so
	// the assertion is the design share with the tolerance of a hash, not an
	// equality.
	want := map[string]float64{
		fixed.VerdictHit: 0.60, fixed.VerdictCritical: 0.10,
		fixed.VerdictFumble: 0.10, fixed.VerdictMiss: 0.20,
	}
	total := 0
	for verdict, share := range want {
		have := float64(counts[verdict]) / runs
		total += counts[verdict]
		if have < share-0.02 || have > share+0.02 {
			t.Errorf("%s: %.3f of %d rolls, want %.2f", verdict, have, runs, share)
		}
	}
	if total != runs {
		t.Errorf("the table answered %d of %d rolls: it has a row nothing names", total, runs)
	}
}

// TestOutcomeTableIsTheOneWrittenDown is the golden vector of the table: for
// each of its ten rows, one cause event that lands on it and the outcome it
// has to produce, written out rather than derived.
//
// The shares alone do not pin the table — reorder the rows and 60/10/10/20
// still holds — but the scenario recorded for T-018 would quietly become a
// different story, because it picks its cause events by the outcome they land
// on. These pairs are what makes such a reordering visible.
func TestOutcomeTableIsTheOneWrittenDown(t *testing.T) {
	for _, tc := range []struct {
		causeEventID string
		rollIndex    int
		want         string
	}{
		{"ev-11", 0, fixed.VerdictHit},      // the six rows of a hit
		{"ev-38", 0, fixed.VerdictHit},      //
		{"ev-5", 0, fixed.VerdictHit},       //
		{"ev-21", 0, fixed.VerdictHit},      //
		{"ev-10", 0, fixed.VerdictHit},      //
		{"ev-23", 0, fixed.VerdictHit},      //
		{"ev-1", 0, fixed.VerdictCritical},  // the one row of a critical
		{"ev-8", 0, fixed.VerdictFumble},    // the one row of a fumble
		{"ev-0", 0, fixed.VerdictMiss},      // the two rows of a miss
		{"ev-3", 0, fixed.VerdictMiss},      //
		{"ev-24", 1, fixed.VerdictCritical}, // a second swing of one event
		{"ev-27", 1, fixed.VerdictMiss},     // decides on its own row
	} {
		if have := fixed.Verdict(tc.causeEventID, tc.rollIndex); have != tc.want {
			t.Errorf("Verdict(%q, %d) = %q, want %q: the table is not the one written down",
				tc.causeEventID, tc.rollIndex, have, tc.want)
		}
	}
}

// TestResolveIsDeterministic is the one property the stub shares with the real
// mechanics and the reason it can stand in for them at all (ADR-012 p. 3).
func TestResolveIsDeterministic(t *testing.T) {
	m := load(t)
	player, wolf := actorsOf(t, m)
	action := mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: wolf.ID}

	first, firstRolls, err := m.Resolve("ev-42", 0, action, actorMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// A second instance, loaded again, from actors built again: nothing but
	// the cause event and the index may decide the answer.
	again := load(t)
	againPlayer, againWolf := actorsOf(t, again)
	second, secondRolls, err := again.Resolve("ev-42", 0, action, actorMap(againPlayer, againWolf))
	if err != nil {
		t.Fatalf("resolve again: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("outcome\n  first:  %+v\n  second: %+v", first, second)
	}
	if !reflect.DeepEqual(firstRolls, secondRolls) {
		t.Errorf("rolls\n  first:  %+v\n  second: %+v", firstRolls, secondRolls)
	}
}

// TestResolveDoesNotTouchTheActors covers the guarantee of C-03: the answer is
// in the Outcome, and what the fight costs is applied by State.
func TestResolveDoesNotTouchTheActors(t *testing.T) {
	m := load(t)
	player, wolf := actorsOf(t, m)
	before := *wolf

	if _, _, err := m.Resolve(causeFor(t, fixed.VerdictCritical, 0), 0,
		mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: wolf.ID},
		actorMap(player, wolf)); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if *wolf != before {
		t.Errorf("the wolf changed under Resolve\n  before: %+v\n  after:  %+v", before, *wolf)
	}
}

// --- the four verdicts of a strike ---

func TestResolveAttackFollowsTheTable(t *testing.T) {
	m := load(t)
	attack := m.Rules().Attack()

	for _, tc := range []struct {
		verdict           string
		hit, crit, fumble bool
		natural           int
		damages           bool
	}{
		{fixed.VerdictHit, true, false, false, 0, true},
		{fixed.VerdictCritical, true, true, false, attack.CritNatural, true},
		{fixed.VerdictFumble, false, false, true, attack.FumbleNatural, false},
		{fixed.VerdictMiss, false, false, false, 0, false},
	} {
		t.Run(tc.verdict, func(t *testing.T) {
			player, wolf := actorsOf(t, m)
			cause := causeFor(t, tc.verdict, 0)

			out, rolls, err := m.Resolve(cause, 0,
				mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: wolf.ID},
				actorMap(player, wolf))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			if out.Hit != tc.hit || out.Critical != tc.crit || out.Fumble != tc.fumble {
				t.Errorf("hit=%v critical=%v fumble=%v, want %v/%v/%v",
					out.Hit, out.Critical, out.Fumble, tc.hit, tc.crit, tc.fumble)
			}
			if tc.natural != 0 && out.Natural != tc.natural {
				t.Errorf("natural %d, want %d", out.Natural, tc.natural)
			}
			// The die reported has to agree with the verdict, or a consumer
			// that renders it tells the player something the outcome denies.
			switch {
			case tc.crit || tc.fumble:
			case tc.hit && out.Natural+player.Atk < wolf.Def:
				t.Errorf("a hit that shows natural %d + atk %d against def %d",
					out.Natural, player.Atk, wolf.Def)
			case !tc.hit && out.Natural+player.Atk >= wolf.Def:
				t.Errorf("a miss that shows natural %d + atk %d against def %d",
					out.Natural, player.Atk, wolf.Def)
			}

			if out.HPBefore != wolf.HP {
				t.Errorf("hp_before %d, the wolf stands at %d", out.HPBefore, wolf.HP)
			}
			if want := wolf.HP - out.Damage; out.HPAfter != max(want, 0) {
				t.Errorf("hp_after %d, want %d", out.HPAfter, max(want, 0))
			}
			if out.TargetDead != (out.HPAfter == 0) {
				t.Errorf("target_dead %v at hp %d", out.TargetDead, out.HPAfter)
			}

			if tc.damages {
				if out.Damage <= 0 {
					t.Errorf("damage %d: a hit costs something", out.Damage)
				}
				if len(rolls) != 2 {
					t.Fatalf("%d rolls, want the hit and the damage", len(rolls))
				}
				if rolls[1].Purpose != mech.PurposeDamage || rolls[1].Index != 1 {
					t.Errorf("damage roll %+v: want purpose %s at index 1", rolls[1], mech.PurposeDamage)
				}
			} else {
				if out.Damage != 0 {
					t.Errorf("damage %d without a hit", out.Damage)
				}
				if len(rolls) != 1 {
					t.Errorf("%d rolls, want the hit alone: the damage index is spent, not rolled", len(rolls))
				}
			}
			if rolls[0].Purpose != mech.PurposeHit || rolls[0].Index != 0 {
				t.Errorf("hit roll %+v: want purpose %s at index 0", rolls[0], mech.PurposeHit)
			}
			if rolls[0].Natural != out.Natural {
				t.Errorf("the roll shows %d and the outcome %d", rolls[0].Natural, out.Natural)
			}
		})
	}
}

// TestCriticalDoublesTheDamage checks the one number the verdict changes
// beyond the flag: the multiplier comes from the rules, not from the stub.
func TestCriticalDoublesTheDamage(t *testing.T) {
	m := load(t)
	attack := m.Rules().Attack()
	critCause := causeFor(t, fixed.VerdictCritical, 0)
	player, wolf := actorsOf(t, m)

	out, rolls, err := m.Resolve(critCause, 0,
		mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: wolf.ID},
		actorMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := rolls[1].Result * attack.CritMultiplier; out.Damage != want {
		t.Errorf("damage %d, want the roll %d times %d", out.Damage, rolls[1].Result, attack.CritMultiplier)
	}
}

// TestNPCAnswerCarriesTheNPCPurposes covers the difference between a strike of
// a character and the answer of a wolf: the same decision, other purposes on
// the rolls, which is what dice.rolled reports.
func TestNPCAnswerCarriesTheNPCPurposes(t *testing.T) {
	m := load(t)
	for _, kind := range []string{mech.ActionNPCAttack, mech.ActionFreeAttack} {
		t.Run(kind, func(t *testing.T) {
			player, wolf := actorsOf(t, m)
			cause := causeFor(t, fixed.VerdictHit, 0)

			out, rolls, err := m.Resolve(cause, 0,
				mech.Action{Kind: kind, Actor: wolf.ID, Target: player.ID},
				actorMap(player, wolf))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if rolls[0].Purpose != mech.PurposeNPCHit || rolls[1].Purpose != mech.PurposeNPCDamage {
				t.Errorf("purposes %s/%s, want %s/%s",
					rolls[0].Purpose, rolls[1].Purpose, mech.PurposeNPCHit, mech.PurposeNPCDamage)
			}
			if want := kind == mech.ActionFreeAttack; out.FreeAttack != want {
				t.Errorf("free_attack %v for %s", out.FreeAttack, kind)
			}
			// The wolf bites with d4 and the character with d6: the damage
			// dice belong to the attacker, not to the table.
			if want, _ := m.Rules().DamageDice(*wolf); rolls[1].Formula != want.String() {
				t.Errorf("damage formula %q, the wolf deals %q", rolls[1].Formula, want.String())
			}
		})
	}
}

// --- flight and rest ---

func TestResolveFlee(t *testing.T) {
	m := load(t)
	flee := m.Rules().Flee()

	for _, tc := range []struct {
		verdict string
		success bool
	}{
		{fixed.VerdictHit, true},
		{fixed.VerdictCritical, true},
		{fixed.VerdictFumble, false},
		{fixed.VerdictMiss, false},
	} {
		t.Run(tc.verdict, func(t *testing.T) {
			player, wolf := actorsOf(t, m)
			out, rolls, err := m.Resolve(causeFor(t, tc.verdict, 0), 0,
				mech.Action{Kind: mech.ActionFlee, Actor: player.ID, LivingEnemies: 1},
				actorMap(player, wolf))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if out.Success == nil {
				t.Fatal("a flight attempt with no success: Success is what tells it from a strike")
			}
			if *out.Success != tc.success {
				t.Errorf("success %v, want %v", *out.Success, tc.success)
			}
			if want := !tc.success && flee.OnFail == mech.OnFailFreeAttack; out.FreeAttack != want {
				t.Errorf("free_attack %v, on_fail is %q", out.FreeAttack, flee.OnFail)
			}
			want, err := m.Rules().FleeThreshold(*player, 1)
			if err != nil {
				t.Fatalf("threshold: %v", err)
			}
			if out.Threshold != want {
				t.Errorf("threshold %d, the rules say %d", out.Threshold, want)
			}
			if out.HPAfter != player.HP {
				t.Errorf("hp %d after a flight attempt, was %d", out.HPAfter, player.HP)
			}
			if len(rolls) != 1 || rolls[0].Purpose != mech.PurposeFlee {
				t.Errorf("rolls %+v, want one of purpose %s", rolls, mech.PurposeFlee)
			}
		})
	}
}

// TestFleeThresholdGrowsWithTheEnemies covers the one input of a flight the
// table does not decide.
func TestFleeThresholdGrowsWithTheEnemies(t *testing.T) {
	m := load(t)
	player, wolf := actorsOf(t, m)
	action := mech.Action{Kind: mech.ActionFlee, Actor: player.ID}

	action.LivingEnemies = 1
	alone, _, err := m.Resolve("ev-1", 0, action, actorMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	action.LivingEnemies = 3
	pack, _, err := m.Resolve("ev-1", 0, action, actorMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if pack.Threshold <= alone.Threshold {
		t.Errorf("threshold %d against three enemies, %d against one", pack.Threshold, alone.Threshold)
	}
}

func TestResolveRestRestoresToTheMaximum(t *testing.T) {
	m := load(t)
	player, _ := actorsOf(t, m)
	player.HP = 1

	out, rolls, err := m.Resolve("ev-1", 0,
		mech.Action{Kind: mech.ActionRest, Actor: player.ID}, actorMap(player))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !out.Hit {
		t.Error("a rest cannot fail in the rules of v0.1")
	}
	if out.HPBefore != 1 || out.HPAfter != player.HPMax {
		t.Errorf("hp %d -> %d, want 1 -> %d", out.HPBefore, out.HPAfter, player.HPMax)
	}
	if len(rolls) != 0 {
		t.Errorf("%d rolls: rest.restore is %q and rolls nothing", len(rolls), m.Rules().Rest().Restore)
	}
}

// --- what Resolve refuses ---

func TestResolveRefusesWhatItCannotDecide(t *testing.T) {
	m := load(t)
	player, wolf := actorsOf(t, m)
	dead := *wolf
	dead.ID, dead.Status = "wolf-dead", entity.StatusDead
	abandoned := *player
	abandoned.ID, abandoned.Status = "player-Z", entity.StatusAbandoned
	all := actorMap(player, wolf, &dead, &abandoned)

	for _, tc := range []struct {
		name   string
		cause  string
		index  int
		action mech.Action
		is     error
	}{
		{name: "no cause event", action: mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: wolf.ID}},
		{name: "negative index", cause: "ev-1", index: -1,
			action: mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: wolf.ID}},
		{name: "unknown action", cause: "ev-1",
			action: mech.Action{Kind: "meditate", Actor: player.ID}},
		{name: "actor not in the map", cause: "ev-1",
			action: mech.Action{Kind: mech.ActionAttack, Actor: "nobody", Target: wolf.ID}},
		{name: "target not in the map", cause: "ev-1",
			action: mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: "nobody"}},
		{name: "dead target", cause: "ev-1",
			action: mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: dead.ID},
			is:     mech.ErrInvalidTarget},
		{name: "abandoned target", cause: "ev-1",
			action: mech.Action{Kind: mech.ActionAttack, Actor: wolf.ID, Target: abandoned.ID},
			is:     mech.ErrInvalidTarget},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := m.Resolve(tc.cause, tc.index, tc.action, all)
			if err == nil {
				t.Fatal("resolved something it cannot decide")
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("error %v, want one matching %v", err, tc.is)
			}
		})
	}
}

// --- picking a target ---

func TestNPCTargetIsTheFirstLivingByID(t *testing.T) {
	m := load(t)
	player, wolf := actorsOf(t, m)

	build := func(id, status, participation string) *mech.Actor {
		a := *player
		a.ID, a.Status, a.Participation = id, status, participation
		return &a
	}
	alive := build("player-B", entity.StatusAlive, "")
	earlier := build("player-A", entity.StatusDead, "")
	idle := build("player-0", entity.StatusAlive, entity.ParticipationIdle)
	out := build("player-1", entity.StatusAlive, entity.ParticipationOutOfCombat)
	abandoned := build("player-2", entity.StatusAbandoned, "")
	later := build("player-C", entity.StatusAlive, "")

	// The order the candidates arrive in must not matter, and everyone the
	// rules exclude has to be skipped even though they sort first.
	target := m.NPCTarget(wolf, []*mech.Actor{later, abandoned, out, idle, earlier, alive})
	if target == nil {
		t.Fatal("nobody was picked out of two living candidates")
	}
	if target.ID != alive.ID {
		t.Errorf("picked %s, want the first living by id (%s)", target.ID, alive.ID)
	}

	if m.NPCTarget(wolf, []*mech.Actor{earlier, idle, abandoned, nil}) != nil {
		t.Error("picked somebody out of a list with nobody left to bite")
	}
	if m.NPCTarget(wolf, nil) != nil {
		t.Error("picked somebody out of an empty list")
	}
}

// --- what is not faked at all ---

// TestRulesAreForwardedUnchanged covers the three methods the stub does not
// decide: they have to answer exactly what the loaded rule set answers, or a
// consumer tested against the stub is tested against different numbers than it
// will run with.
func TestRulesAreForwardedUnchanged(t *testing.T) {
	m := load(t)
	rules := m.Rules()

	for _, kind := range rules.Kinds() {
		want, _ := rules.Stats(kind)
		have, ok := m.Stats(kind)
		if !ok || have != want {
			t.Errorf("stats of %s: %+v, rules say %+v", kind, have, want)
		}
	}
	if _, ok := m.Stats("dragon"); ok {
		t.Error("the stub knows a kind the rules do not")
	}

	roll, err := m.Roll("ev-1", 0, "d20", mech.PurposeEncounterChance)
	if err != nil {
		t.Fatalf("roll: %v", err)
	}
	want, err := rules.Roll("ev-1", 0, "d20", mech.PurposeEncounterChance)
	if err != nil {
		t.Fatalf("roll through the rules: %v", err)
	}
	if roll != want {
		t.Errorf("roll %+v, the rules roll %+v", roll, want)
	}
	if _, err := m.Roll("ev-1", 0, "d20", "guessing"); err == nil {
		t.Error("rolled for a purpose the schema of dice.rolled does not know")
	}

	if have, want := len(m.Invariants()), len(rules.Invariants()); have != want {
		t.Errorf("%d invariants, the rules have %d", have, want)
	}
}

// TestRollsBecomeValidDiceRolled publishes what Resolve returned the way C-03
// says a caller must: one dice.rolled per roll, before the decision that rests
// on it. It is the check that a consumer can use the stub without inventing
// anything of its own.
func TestRollsBecomeValidDiceRolled(t *testing.T) {
	m := load(t)
	player, wolf := actorsOf(t, m)
	cause := causeFor(t, fixed.VerdictHit, 0)

	_, rolls, err := m.Resolve(cause, 0,
		mech.Action{Kind: mech.ActionAttack, Actor: player.ID, Target: wolf.ID},
		actorMap(player, wolf))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(rolls) == 0 {
		t.Fatal("a hit rolled nothing")
	}
	for _, roll := range rolls {
		ev := eventbus.NewRoot("dice.rolled", contracts.SourceTestkitSwarm, "dark-forest-world", nil,
			entity.ActorKindCI,
			mech.DiceRolledPayload(roll, entity.Ref{ID: player.ID, Type: player.Type}),
			eventbus.WithAgent(eventbus.AgentRef{
				ID: "fake-encounter", Level: "task", Blueprint: "fake-encounter",
			}))
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("roll %d is not a valid dice.rolled: %v", roll.Index, err)
		}
	}
}

// TestVerdictNamesEveryRow guards the exported view of the table: a row it
// cannot name would make causeFor loop forever and a scenario unable to ask
// for the outcome it needs.
func TestVerdictNamesEveryRow(t *testing.T) {
	seen := map[string]bool{}
	for i := range 1000 {
		seen[fixed.Verdict("ev-"+strconv.Itoa(i), 0)] = true
	}
	for _, name := range []string{
		fixed.VerdictHit, fixed.VerdictCritical, fixed.VerdictFumble, fixed.VerdictMiss,
	} {
		if !seen[name] {
			t.Errorf("no seed out of a thousand lands on %q", name)
		}
	}
	if len(seen) != 4 {
		t.Errorf("the table answers %v, want exactly the four verdicts", seen)
	}
}

func TestLoadRejectsAMissingRulesFile(t *testing.T) {
	if _, err := fixed.Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("loaded a rule set that is not there")
	}
}
