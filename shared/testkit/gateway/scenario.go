package gateway

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"multiverse-core.io/shared/entity"
)

// The actions a scripted scenario is made of. They are the methods of the
// harness under the names a script uses.
const (
	ActionCreate = "create"
	ActionEnter  = "enter"
	ActionLook   = "look"
	ActionSay    = "say"
	ActionRest   = "rest"
	ActionLeave  = "leave"
	ActionAttack = "attack"
	ActionFlee   = "flee"
)

// The scenarios the harness knows.
const (
	// ScenarioVisit is one character walking through a region: create, enter,
	// look, say, rest, leave.
	ScenarioVisit = "solo-visit"
	// ScenarioParty is the same visit for the three fixture characters, one
	// after another.
	ScenarioParty = "party-visit"
	// ScenarioSkirmish is a visit that meets the wolf: the same walk with
	// blows and a flight in the middle of it.
	//
	// It runs only where something resolves a fight — FakeEncounter of
	// EPIC-003 (contracts.md C-05, tasks.md T-219) — because every combat
	// step waits for the change the fight proposes. On a bus where nothing
	// answers, it stops at the first blow and says so.
	//
	// How many blows it strikes is not written into it. How long a wolf stands
	// up to a character is decided by the table of the mechanics, so a script
	// of a fixed length promises what the rules do not: it is the fight that
	// says when the fight is over, and the script follows (journal.md
	// 2026-09-11, the decision on the second open question of the review).
	ScenarioSkirmish = "solo-skirmish"
)

// Condition is what a step of a script asks of the world before it is taken.
//
// It is always about what the harness KNOWS, never about what is true: a fight
// opens and closes on world_events while the answer to a blow arrives on
// system_events, so the two can cross. Not knowing that a fight is over is
// therefore not knowing there is one, and a step under a condition is taken
// unless the harness knows better. That is also why a step conditional on the
// fight is skipped rather than failed when the action comes back with
// ErrFightOver: the same condition, learned one moment later.
type Condition string

const (
	// Anytime is the default: nothing about the world holds the step back.
	Anytime Condition = ""
	// InFight takes the step unless the harness knows the encounter of the
	// character is over.
	InFight Condition = "in-fight"
	// Alive takes the step unless the harness knows the character is past
	// acting — dead, abandoned or ascended.
	Alive Condition = "alive"
)

// Step is one action of a scripted scenario, written out so that a test can
// read the script instead of repeating what it expects.
//
// That is the point of exporting it: the number of narratives a scenario
// produces, the number of facts it asks State for, the characters it touches —
// all of it is derived from the script by whoever asserts on it, and a script
// that grows a step does not leave a stale literal behind in a test
// (tasks.md T-018).
type Step struct {
	Action string
	// Player is the character acting.
	Player string
	// Region is where Enter walks the character. Leave takes it from where the
	// character stands and the other actions do not use it.
	Region string
	// Target is whom Attack swings at. The other actions do not use it: a
	// flight is resolved against a threshold rather than against somebody.
	Target string
	// Text is what Say says.
	Text string
	// When is what the step asks of the world before it is taken. The zero
	// value takes it whatever the harness knows.
	When Condition
	// Turns is how many times the step may be taken; zero and one mean once.
	// A step with more turns stops as soon as When no longer holds, so the
	// number is a bound and not a promise — which is the whole difference
	// between a script that survives a fight and one that describes it.
	Turns int
}

// PlayerEvent is the player_events type this step publishes, or the empty
// string for a step that only proposes a change of the world.
func (s Step) PlayerEvent() string {
	switch s.Action {
	case ActionEnter:
		return TypeEnteredRegion
	case ActionLeave:
		return TypeLeftRegion
	case ActionLook:
		return TypeLooked
	case ActionSay:
		return TypeSaid
	case ActionRest:
		return TypeRested
	case ActionAttack:
		return TypeAttacked
	case ActionFlee:
		return TypeFleeAttempted
	default:
		return ""
	}
}

// Proposes is the entity.*.proposed type this step publishes alongside its
// action, or the empty string when the harness proposes nothing for it.
//
// A combat step proposes nothing: the blow is decided and the change is
// proposed by whoever runs the fight (C-05), so the step is answered without
// the harness ever having asked. Resolves is what tells the two apart.
func (s Step) Proposes() string {
	switch s.Action {
	case ActionCreate:
		return TypeCreateProposed
	case ActionEnter, ActionLeave, ActionRest:
		return TypeUpdateProposed
	default:
		return ""
	}
}

// Resolves reports whether the step hands the world to whoever runs the fight.
//
// It is the second thing a test derives from a script rather than writes down:
// how many facts a visit leaves behind is known from Fact, how many a fight
// leaves behind is not — that depends on who hit whom — so a test counts the
// steps of the first kind and, for the second, checks that each of them was
// answered at all.
func (s Step) Resolves() bool {
	return s.Action == ActionAttack || s.Action == ActionFlee
}

// Fact is the type State answers this step with when it accepts the proposal,
// or the empty string when the step proposes nothing.
func (s Step) Fact() string {
	switch s.Proposes() {
	case TypeCreateProposed:
		return TypeCreated
	case TypeUpdateProposed:
		return TypeUpdated
	default:
		return ""
	}
}

// visit is the six steps of one character passing through a region. It is
// written once and instantiated per character, so that the party scenario
// cannot drift from the solo one.
func visit(player, region, text string) []Step {
	return []Step{
		{Action: ActionCreate, Player: player},
		{Action: ActionEnter, Player: player, Region: region},
		{Action: ActionLook, Player: player},
		{Action: ActionSay, Player: player, Text: text},
		{Action: ActionRest, Player: player},
		{Action: ActionLeave, Player: player},
	}
}

// maxBlows bounds the blows of ScenarioSkirmish. It is a stop, not a plan: the
// wolf of the fixtures has 10 hp against a d6, and the table of the mechanics
// lands seven swings out of ten, so a fight is normally over well inside it —
// and where it is not, the character stops swinging and tries to run instead
// of hitting a wolf for ever.
const maxBlows = 8

// skirmish is the visit of one character with a fight in the middle of it: it
// is the same walk, so that what combat adds to a scenario is exactly the two
// steps of the fight and nothing else.
//
// Every step of it is conditional, and each condition answers something the
// fight can do to a script. The blows stop when the encounter closes, because
// swinging at a fight that is over is answered by nobody. The flight is taken
// only while there is something to flee from. And what follows the fight is
// taken only while the character is still there to take it: a rest and a walk
// out are refused over a corpse (dead_entity, C-02 v1.2), so a script that led
// one there would fail on the world rather than on itself.
func skirmish(player, region, npc, text string) []Step {
	steps := visit(player, region, text)
	fight := []Step{
		{Action: ActionAttack, Player: player, Target: npc, When: InFight, Turns: maxBlows},
		{Action: ActionFlee, Player: player, When: InFight},
	}
	// After the look and before the rest: a character that has met the wolf
	// has something to rest from.
	after := slices.Clone(steps[3:])
	for i := range after {
		after[i].When = Alive
	}
	return slices.Concat(steps[:3], fight, after)
}

// The characters, the region and the NPC of the fixture world the scripts are
// written against (testdata/fixtures). A script naming something else would
// fail in the harness, not here.
const (
	fixtureRegion  = "dark-forest-01"
	fixtureNPC     = "wolf-alpha"
	fixturePlayerA = "player-A"
	fixturePlayerB = "player-B"
	fixturePlayerC = "player-C"
)

var scripts = map[string][]Step{
	ScenarioVisit: visit(fixturePlayerA, fixtureRegion, "Здесь кто-то был до нас."),
	ScenarioParty: slices.Concat(
		visit(fixturePlayerA, fixtureRegion, "Здесь кто-то был до нас."),
		visit(fixturePlayerB, fixtureRegion, "Туман гуще, чем вчера."),
		visit(fixturePlayerC, fixtureRegion, "Идём, пока светло."),
	),
	ScenarioSkirmish: skirmish(fixturePlayerA, fixtureRegion, fixtureNPC, "Здесь кто-то был до нас."),
}

// Scenarios are the names Script and Scenario accept, sorted.
func Scenarios() []string {
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Script returns the steps of a named scenario without running any of them.
func Script(name string) ([]Step, error) {
	steps, ok := scripts[name]
	if !ok {
		return nil, fmt.Errorf("testkit/gateway: no scenario %q; the harness knows %v, "+
			"and a script of your own is a []Step the harness runs step by step", name, Scenarios())
	}
	return slices.Clone(steps), nil
}

// Scenario runs a named script, stopping at the first step that fails.
func (h *Harness) Scenario(ctx context.Context, name string) error {
	steps, err := Script(name)
	if err != nil {
		return err
	}
	if _, err := h.Run(ctx, steps); err != nil {
		return fmt.Errorf("testkit/gateway: scenario %s, %w", name, err)
	}
	return nil
}

// Run takes the steps of a script in order, stopping at the first one that
// fails, and reports the turns it actually took. Every step that proposes a
// change waits for the answer of State before the next one starts, so the
// world a step acts on is the world the previous step left.
//
// The turns are the point of the return value. A script is a plan — a step can
// be skipped by its condition and a repeated step stops when the fight does —
// so how many actions a run is worth is known from what it took, not from what
// it planned. Whoever asserts on a run reads this, exactly as the derivations
// of a fixed script read Script.
func (h *Harness) Run(ctx context.Context, steps []Step) ([]Step, error) {
	taken := make([]Step, 0, len(steps))
	for i, step := range steps {
		turns, err := h.take(ctx, step)
		taken = append(taken, turns...)
		if err != nil {
			return taken, fmt.Errorf("step %d (%s %s): %w", i+1, step.Action, step.Player, err)
		}
	}
	return taken, nil
}

// take runs one step for as many turns as it asks for, and reports the turns
// it took.
func (h *Harness) take(ctx context.Context, step Step) ([]Step, error) {
	turns := max(step.Turns, 1)
	taken := make([]Step, 0, turns)
	for range turns {
		if !h.holds(step.When, step.Player) {
			return taken, nil
		}
		if err := h.Step(ctx, step); err != nil {
			if step.When == InFight && errors.Is(err, ErrFightOver) {
				// The condition of the step, learned one moment later than the
				// check above: the fight closed on world_events while the step
				// was being answered on system_events.
				return taken, nil
			}
			return taken, err
		}
		taken = append(taken, step)
	}
	return taken, nil
}

// holds reports whether the world, as far as the harness has been told, still
// allows a step under this condition.
func (h *Harness) holds(when Condition, playerID string) bool {
	switch when {
	case InFight:
		_, ended, known := h.Fight(playerID)
		return !known || ended == ""
	case Alive:
		status, known := h.Status(playerID)
		return !known || !entity.IsTerminalStatus(status)
	default:
		return true
	}
}

// Step runs one step of a script. It is exported so that a test can drive a
// scenario of its own without the script table.
func (h *Harness) Step(ctx context.Context, step Step) error {
	switch step.Action {
	case ActionCreate:
		return h.CreatePlayer(ctx, step.Player)
	case ActionEnter:
		return h.Enter(ctx, step.Player, step.Region)
	case ActionLook:
		return h.Look(ctx, step.Player)
	case ActionSay:
		return h.Say(ctx, step.Player, step.Text)
	case ActionRest:
		return h.Rest(ctx, step.Player)
	case ActionLeave:
		return h.Leave(ctx, step.Player)
	case ActionAttack:
		return h.Attack(ctx, step.Player, step.Target)
	case ActionFlee:
		return h.Flee(ctx, step.Player)
	default:
		return fmt.Errorf("testkit/gateway: unknown action %q", step.Action)
	}
}
