//go:build e2e

// death through the HTTP API of the gateway (T-313, US-007, UC-012): a
// character dies in a fight. The player hears the blow, the death told by the
// narrator and the system text that offers /start; the fight ends, and every
// later action of the character is refused with character_dead.

package e2e_test

import (
	"errors"
	"net/http"
	"testing"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/entity"
	tkstate "multiverse-core.io/shared/testkit/state"
	tkswarm "multiverse-core.io/shared/testkit/swarm"
)

// gatewayDeathBudget bounds the actions of the death scenario. The character
// never strikes, so the wolf never falls: it flees, is struck out of turn when
// the flight fails, walks back in when it succeeds, and has ten hit points to
// lose to a d4.
const gatewayDeathBudget = 80

func TestGatewayDeath(t *testing.T) {
	var first string
	if !t.Run("run", func(t *testing.T) { first = gatewayPlayDeath(t, gatewayOptions{}) }) {
		return
	}
	t.Run("the second run gives the same journal", func(t *testing.T) {
		gatewayAssertSameRun(t, "death", first, gatewayPlayDeath(t, gatewayOptions{}))
	})
}

func gatewayPlayDeath(t *testing.T, opts gatewayOptions) string {
	w := newGatewayWorld(t, opts)
	w.enter()

	fatal := ""
	for range gatewayDeathBudget {
		c := w.character()
		if c.Status == entity.StatusDead {
			break
		}
		if c.Encounter == nil {
			target := gatewayRegion
			w.act(api.ActionRequest{Type: api.ActionEnter, Target: &target})
			continue
		}
		fatal = w.act(api.ActionRequest{Type: api.ActionFlee})
	}
	c := w.character()
	if c.Status != entity.StatusDead {
		t.Fatalf("the character is %s after %d actions, want dead", c.Status, gatewayDeathBudget)
	}
	t.Logf("death: the character died on turn %d", len(w.steps))
	gatewayAssertDeath(t, w, c, fatal)

	// Whatever the player tries afterwards is refused. Nothing of it reaches the
	// world: the one event of a refusal is its analytics.turn.completed with
	// status rejected (C-10), and the player hears nothing but the answer.
	target := gatewayRegion
	for _, req := range []api.ActionRequest{{Type: api.ActionLook}, {Type: api.ActionEnter, Target: &target},
		{Type: api.ActionAttack, Target: gatewayPtr(gatewayWolf)}} {
		before := w.journal.refresh()
		err := w.refused(req)
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != api.CodeCharacterDead {
			t.Errorf("%s of the dead character = %v, want 409 %s", req.Type, err, api.CodeCharacterDead)
		}
		published := w.journal.events[before:w.journal.refresh()]
		status := ""
		if len(published) == 1 && published[0].ev.Type == turns.TypeCompleted {
			status, _ = published[0].ev.Path().GetString("turn.status")
		}
		if status != turns.OutcomeRejected {
			types := make([]string, 0, len(published))
			for _, r := range published {
				types = append(types, r.ev.Type)
			}
			t.Errorf("%s of the dead character published %v, want one rejected turn.completed", req.Type, types)
		}
		if n := len(w.steps[len(w.steps)-1].deliveries); n != 0 {
			t.Errorf("%s of the dead character delivered %d texts", req.Type, n)
		}
	}
	w.finish()
	return w.canonical()
}

// gatewayAssertDeath checks the step the character died in.
func gatewayAssertDeath(t *testing.T, w *gatewayWorld, c api.CharacterState, fatal string) {
	t.Helper()
	if c.HP == nil || *c.HP != 0 || c.Encounter != nil {
		t.Errorf("the dead character has hp %v and encounter %+v, want 0 and none", c.HP, c.Encounter)
	}
	var blow, died bool
	for _, ev := range w.journal.correlated(tkswarm.TypeCombatDecided, fatal) {
		pa := ev.Path()
		defender, _ := pa.GetString("defender.entity.id")
		dead, _ := pa.GetBool("outcome.target_dead")
		blow = blow || (defender == w.player && dead)
	}
	for _, ev := range w.journal.correlated(tkstate.TypeUpdated, fatal) {
		if who, _ := ev.Path().GetString("entity.entity.id"); who == w.player &&
			gatewayChangedTo(ev, entity.AttrStatus, entity.StatusDead) {
			died = true
		}
	}
	if !blow || !died {
		t.Fatalf("the last flight %s: a killing blow %v, a fact of the death %v; want both", fatal, blow, died)
	}
	ended := w.journal.correlated(tkswarm.TypeEncounterEnded, fatal)
	if len(ended) != 1 {
		t.Fatalf("encounter.ended of the death: %d, want one", len(ended))
	}
	if reason, _ := ended[0].Path().GetString("reason"); reason != entity.ResolutionPlayersOut {
		t.Errorf("the fight ended with %q, want %q", reason, entity.ResolutionPlayersOut)
	}
	kinds := map[string]int{}
	for _, ev := range w.journal.correlated(tkswarm.TypeNarrativeOutput, fatal) {
		kind, _ := ev.Path().GetString("kind")
		kinds[kind]++
	}
	if kinds["turn"] != 1 || kinds["death"] != 1 {
		t.Errorf("narratives of the death: %v, want one turn and one death", kinds)
	}

	step := w.steps[len(w.steps)-1]
	got := map[string]int{}
	for _, d := range step.deliveries {
		if d.CorrelationID == fatal {
			got[d.Kind]++
		}
	}
	if got[outbox.KindSystem] != 1 || got[outbox.KindNarrative] != 2 || got[outbox.KindMechanics] != 2 {
		t.Errorf("deliveries of the death: %v, want one system, two narratives and two mechanics", got)
	}
	if len(w.journal.completed(fatal)) != 1 {
		t.Errorf("analytics.turn.completed of the fatal turn: %d, want one", len(w.journal.completed(fatal)))
	}
}

// gatewayPtr is a pointer to a string of a request.
func gatewayPtr(s string) *string { return &s }
