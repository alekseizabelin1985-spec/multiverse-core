package gateway

import (
	"context"
	"fmt"
	"slices"
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
)

// The scenarios the harness of v0 can run.
//
// Neither of them fights: the only stub of Phase 1 that resolves a blow is
// FakeEncounter of EPIC-003, and until it lands there is nothing to attack
// (contracts.md C-05, tasks.md T-018). ScenarioSolo30 of design.md §5 — thirty
// turns of combat — arrives with it, in I1-α.
const (
	// ScenarioVisit is one character walking through a region: create, enter,
	// look, say, rest, leave.
	ScenarioVisit = "solo-visit"
	// ScenarioParty is the same visit for the three fixture characters, one
	// after another.
	ScenarioParty = "party-visit"
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
	// Text is what Say says.
	Text string
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
	default:
		return ""
	}
}

// Proposes is the entity.*.proposed type this step publishes alongside its
// action, or the empty string when the step changes nothing State records.
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

// The characters and the region of the fixture world the scripts are written
// against (testdata/fixtures). A script naming something else would fail in
// the harness, not here.
const (
	fixtureRegion  = "dark-forest-01"
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
		return nil, fmt.Errorf("testkit/gateway: no scenario %q; the harness of v0 knows %v "+
			"and fights in none of them — combat arrives with testkit/swarm.FakeEncounter (I1-α)",
			name, Scenarios())
	}
	return slices.Clone(steps), nil
}

// Scenario runs a named script step by step, stopping at the first step that
// fails. Every step that proposes a change waits for the answer of State
// before the next one starts, so the world a step acts on is the world the
// previous step left.
func (h *Harness) Scenario(ctx context.Context, name string) error {
	steps, err := Script(name)
	if err != nil {
		return err
	}
	for i, step := range steps {
		if err := h.Step(ctx, step); err != nil {
			return fmt.Errorf("testkit/gateway: scenario %s, step %d (%s %s): %w",
				name, i+1, step.Action, step.Player, err)
		}
	}
	return nil
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
	default:
		return fmt.Errorf("testkit/gateway: unknown action %q", step.Action)
	}
}
