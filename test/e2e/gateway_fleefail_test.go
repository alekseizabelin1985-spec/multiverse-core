//go:build e2e

// flee-fail through the HTTP API of the gateway (T-313, US-008, UC-009): a
// character in a fight tries to get away, fails, and the creature strikes out
// of turn. The player hears both decisions as the text of the rules and one
// narrative of the turn about both, and stays in the fight.

package e2e_test

import (
	"strconv"
	"testing"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	tkswarm "multiverse-core.io/shared/testkit/swarm"
)

// gatewayFleeBudget bounds the attempts to fail a flight. Whether a flight
// fails is decided by FixedMechanics from the identifier of the action (a
// failure is three rows of ten), so the scenario flees, walks back in when the
// flight succeeded, and flees again until one fails.
const gatewayFleeBudget = 20

func TestGatewayFleeFail(t *testing.T) {
	var first string
	if !t.Run("run", func(t *testing.T) { first = gatewayPlayFleeFail(t, gatewayOptions{}) }) {
		return
	}
	t.Run("the second run gives the same journal", func(t *testing.T) {
		gatewayAssertSameRun(t, "flee-fail", first, gatewayPlayFleeFail(t, gatewayOptions{}))
	})
}

func gatewayPlayFleeFail(t *testing.T, opts gatewayOptions) string {
	w := newGatewayWorld(t, opts)
	w.enter()

	caught := ""
	for range gatewayFleeBudget {
		if w.character().Encounter == nil {
			target := gatewayRegion
			w.act(api.ActionRequest{Type: api.ActionEnter, Target: &target})
		}
		before := w.character()
		correlation := w.act(api.ActionRequest{Type: api.ActionFlee})
		if len(gatewayDecisions(w, correlation, "free_attack")) > 0 {
			caught = correlation
			gatewayAssertCaught(t, w, before, correlation)
			break
		}
	}
	if caught == "" {
		t.Fatalf("no flight of %d failed", gatewayFleeBudget)
	}
	w.finish()
	return w.canonical()
}

// gatewayAssertCaught checks the step of the failed flight.
func gatewayAssertCaught(t *testing.T, w *gatewayWorld, before api.CharacterState, correlation string) {
	t.Helper()
	flight := gatewayDecisions(w, correlation, "flee")
	strike := gatewayDecisions(w, correlation, "free_attack")
	if len(flight) != 1 || len(strike) != 1 {
		t.Fatalf("decisions of the failed flight: %d flee, %d free_attack, want one of each", len(flight), len(strike))
	}
	fp, sp := flight[0].Path(), strike[0].Path()
	if success, ok := fp.GetBool("outcome.success"); !ok || success {
		t.Errorf("flee decided success=%v (present %v), want false", success, ok)
	}
	if last, _ := fp.GetBool("exchange.last"); last {
		t.Error("the failed flight is the last decision of its exchange; the strike out of turn must follow it")
	}
	attacker, _ := sp.GetString("attacker.entity.id")
	defender, _ := sp.GetString("defender.entity.id")
	free, _ := sp.GetBool("free_attack")
	last, _ := sp.GetBool("exchange.last")
	if attacker != gatewayWolf || defender != w.player || !free || !last {
		t.Errorf("strike out of turn: %s → %s, free_attack %v, last %v; want %s → %s, true, true",
			attacker, defender, free, last, gatewayWolf, w.player)
	}

	step := w.steps[len(w.steps)-1]
	var mechanics, narratives []api.Delivery
	for _, d := range step.deliveries {
		switch {
		case d.CorrelationID != correlation:
		case d.Kind == outbox.KindMechanics:
			mechanics = append(mechanics, d)
		case d.Kind == outbox.KindNarrative:
			narratives = append(narratives, d)
		}
	}
	if len(mechanics) != 2 || len(narratives) != 1 {
		t.Fatalf("deliveries of the failed flight: %d mechanics, %d narrative; want 2 and 1", len(mechanics), len(narratives))
	}
	told := w.journal.correlated(tkswarm.TypeNarrativeOutput, correlation)
	if len(told) != 1 {
		t.Fatalf("narratives of the failed flight: %d, want one", len(told))
	}
	np := told[0].Path()
	if kind, _ := np.GetString("kind"); kind != "turn" {
		t.Errorf("the failed flight is told as %q, want turn", kind)
	}
	refs, _ := np.GetSlice("based_on")
	about := map[string]bool{}
	for i := range refs {
		id, _ := np.GetString("based_on[" + strconv.Itoa(i) + "].event.id")
		about[id] = true
	}
	if len(refs) != 2 || !about[flight[0].ID] || !about[strike[0].ID] {
		t.Errorf("the narrative of the failed flight is based on %v, want exactly the two decisions", about)
	}

	after := w.character()
	if after.Encounter == nil || after.Position == nil || after.Position.ID != gatewayRegion {
		t.Errorf("after the failed flight the character is at %+v in %+v, want still in the fight in %s",
			after.Position, after.Encounter, gatewayRegion)
	}
	// A strike that missed costs nothing; one that landed costs its damage.
	damage := 0
	if hit, _ := sp.GetBool("outcome.hit"); hit {
		damage, _ = sp.GetInt("outcome.damage")
	}
	if after.HP == nil || before.HP == nil || *after.HP != max(*before.HP-damage, 0) {
		t.Errorf("hp %v → %v after a strike that cost %d", gatewayHP(before), gatewayHP(after), damage)
	}
	if after.Status != entity.StatusAlive {
		t.Errorf("the character is %s after one strike from full health", after.Status)
	}

	completed := w.journal.completed(correlation)
	if len(completed) != 1 {
		t.Fatalf("analytics.turn.completed of the failed flight: %d, want one", len(completed))
	}
	cp := completed[0].Path()
	status, _ := cp.GetString("turn.status")
	action, _ := cp.GetString("turn.action_type")
	if status != turns.OutcomeDegraded || action != api.ActionFlee {
		t.Errorf("turn.completed of the failed flight: status %q, action %q; want a degraded flee", status, action)
	}
}

// gatewayHP is the health a character reports, for a message.
func gatewayHP(c api.CharacterState) string {
	if c.HP == nil {
		return "none"
	}
	return strconv.Itoa(*c.HP)
}

// gatewayDecisions are the combat.decided of one action with the given action
// kind.
func gatewayDecisions(w *gatewayWorld, correlation, action string) []eventbus.Event {
	w.journal.refresh()
	var out []eventbus.Event
	for _, ev := range w.journal.correlated(tkswarm.TypeCombatDecided, correlation) {
		if a, _ := ev.Path().GetString("action"); a == action {
			out = append(out, ev)
		}
	}
	return out
}
