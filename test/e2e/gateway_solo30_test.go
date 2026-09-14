//go:build e2e

// solo-30 through the HTTP API of the gateway (T-313, S1, US-001, UC-001…005):
// a player registers, consents, gets a character, walks into the dark forest,
// meets the wolf and plays thirty turns — fighting, looking around, speaking and
// resting — and leaves. Every turn is completed in analytics, the session is
// opened once and closed once, and a run with every record of the bus carried
// twice (--chaos=duplicate, NFR-013) is the same run.

package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/jsonpath"
	tkswarm "multiverse-core.io/shared/testkit/swarm"
)

const (
	// gatewaySolo30Turns is the length of the scenario, the entry and the
	// leave included.
	gatewaySolo30Turns = 30
	// gatewayWounded is the health at which the character stops trading
	// blows and tries to get away: the wolf bites for a d4.
	gatewayWounded = 4
	// gatewaySolo30Say is what the character says. It must not reach
	// analytics (C-10).
	gatewaySolo30Say = "Кто здесь?"
)

func TestGatewaySolo30(t *testing.T) {
	var first string
	if !t.Run("run", func(t *testing.T) { first = gatewayPlaySolo30(t, gatewayOptions{}) }) {
		return
	}
	t.Run("the second run gives the same journal", func(t *testing.T) {
		gatewayAssertSameRun(t, "solo-30", first, gatewayPlaySolo30(t, gatewayOptions{}))
	})
	t.Run("every record carried twice gives the same journal", func(t *testing.T) {
		gatewayAssertSameRun(t, "solo-30 --chaos=duplicate", first, gatewayPlaySolo30(t, gatewayOptions{duplicate: true}))
	})
}

func gatewayPlaySolo30(t *testing.T, opts gatewayOptions) string {
	w := newGatewayWorld(t, opts)
	w.enter()
	rested := false
	for turn := 2; turn < gatewaySolo30Turns; turn++ {
		c := w.character()
		if c.Status != entity.StatusAlive {
			t.Fatalf("turn %d: the character is %s", turn, c.Status)
		}
		req := gatewaySolo30Choice(turn, c, w.journal.alive(gatewayWolf), rested)
		rested = rested || req.Type == api.ActionRest
		w.act(req)
	}
	c := w.character()
	if c.Status != entity.StatusAlive || c.Encounter != nil || c.Position == nil || c.Position.ID != gatewayRegion {
		t.Fatalf("before the last turn the character is %s at %+v in %+v, want alive in %s out of a fight",
			c.Status, c.Position, c.Encounter, gatewayRegion)
	}
	w.act(api.ActionRequest{Type: api.ActionLeave})
	if c := w.character(); c.Position == nil || c.Position.ID == gatewayRegion {
		t.Errorf("after the leave the character is at %+v, want out of %s", c.Position, gatewayRegion)
	}

	// The session ends by the sweeper once it has been idle long enough
	// (BR-17): a solo session has no other end in I1.
	w.clock.Advance(session.DefaultIdle + time.Minute)
	waitFor(t, "analytics.session.ended", func() bool {
		w.journal.refresh()
		_, ended := gatewaySessions(w.journal)
		return len(ended) > 0
	})
	w.settle("session end", "")

	if opts.duplicate {
		gatewayAssertCarriedTwice(t, w)
	}
	gatewayAssertSolo30Turns(t, w)
	gatewayAssertSolo30Analytics(t, w)
	gatewayAssertAccepted(t, w.accepted)
	w.finish()
	return w.canonical()
}

// gatewaySolo30Choice is what the character does on a turn: it fights the wolf,
// looks around and speaks between blows, flees when wounded, rests once out of
// the fight and walks back in while the wolf lives, and passes the time with
// the forest once the wolf is dead.
func gatewaySolo30Choice(turn int, c api.CharacterState, wolfAlive, rested bool) api.ActionRequest {
	hp, hpMax := 0, 0
	if c.HP != nil && c.HPMax != nil {
		hp, hpMax = *c.HP, *c.HPMax
	}
	say := gatewaySolo30Say
	inRegion := c.Position != nil && c.Position.ID == gatewayRegion
	switch {
	case c.Encounter != nil && hp <= gatewayWounded:
		return api.ActionRequest{Type: api.ActionFlee}
	case c.Encounter != nil && turn%5 == 3:
		return api.ActionRequest{Type: api.ActionLook}
	case c.Encounter != nil && turn%5 == 4:
		return api.ActionRequest{Type: api.ActionSay, Text: &say}
	case c.Encounter != nil:
		return api.ActionRequest{Type: api.ActionAttack, Target: gatewayPtr(gatewayWolf)}
	case hp < hpMax || !rested:
		return api.ActionRequest{Type: api.ActionRest}
	case !inRegion:
		return api.ActionRequest{Type: api.ActionEnter, Target: gatewayPtr(gatewayRegion)}
	case wolfAlive:
		return api.ActionRequest{Type: api.ActionEnter, Target: gatewayPtr(gatewayRegion)}
	case turn%2 == 0:
		return api.ActionRequest{Type: api.ActionLook}
	default:
		return api.ActionRequest{Type: api.ActionSay, Text: &say}
	}
}

// gatewayAssertCarriedTwice checks that the chaos of the run was on — every
// record of the bus is there twice — and that State applied every proposal
// once all the same.
func gatewayAssertCarriedTwice(t *testing.T, w *gatewayWorld) {
	t.Helper()
	records, err := w.bus.Records(eventbus.TopicPlayerEvents)
	if err != nil {
		t.Fatal(err)
	}
	actions := 0
	for _, r := range w.journal.events {
		if r.topic == eventbus.TopicPlayerEvents {
			actions++
		}
	}
	if actions == 0 || len(records) != 2*actions {
		t.Fatalf("player_events holds %d records for %d actions, want every one twice", len(records), actions)
	}
	applied := w.state.AppliedProposals()
	seen := make(map[string]bool, len(applied))
	for _, id := range applied {
		if seen[id] {
			t.Errorf("State applied proposal %s twice", id)
		}
		seen[id] = true
	}
}

// gatewayAssertSolo30Turns checks that the thirty turns are the ones the
// scenario is about: a fight with the wolf, looks, words and a rest in it, and
// a leave at the end.
func gatewayAssertSolo30Turns(t *testing.T, w *gatewayWorld) {
	t.Helper()
	count := map[string]int{}
	actions := make([]string, 0, len(w.steps))
	for _, s := range w.steps {
		if s.correlation == "" {
			continue
		}
		count[s.action]++
		actions = append(actions, s.action)
	}
	t.Logf("solo-30: %s", strings.Join(actions, " "))
	if len(actions) != gatewaySolo30Turns {
		t.Fatalf("%d turns, want %d", len(actions), gatewaySolo30Turns)
	}
	for _, action := range []string{api.ActionAttack, api.ActionLook, api.ActionSay, api.ActionRest} {
		if count[action] == 0 {
			t.Errorf("no %s among the turns: %v", action, count)
		}
	}
	if actions[0] != api.ActionEnter || actions[len(actions)-1] != api.ActionLeave {
		t.Errorf("the turns go from %s to %s, want from enter to leave", actions[0], actions[len(actions)-1])
	}
	if len(w.journal.ofType(tkswarm.TypeEncounterStarted)) == 0 {
		t.Error("no encounter opened: the scenario met nobody")
	}
}

// gatewayAssertSolo30Analytics checks the analytics of the run: one
// analytics.turn.completed per turn, completed by its narrative when the
// narrator tells about it and by its deadline when not, one session opened and
// closed around all of them, and nothing of the player — neither the name nor
// the words — in any of it (C-10).
func gatewayAssertSolo30Analytics(t *testing.T, w *gatewayWorld) {
	t.Helper()
	degraded, timedOut := 0, 0
	for i, s := range w.steps {
		if s.correlation == "" {
			continue
		}
		completed := w.journal.completed(s.correlation)
		if len(completed) != 1 {
			t.Errorf("turn %d (%s): %d analytics.turn.completed, want one", i+1, s.action, len(completed))
			continue
		}
		pa := completed[0].Path()
		status, _ := pa.GetString("turn.status")
		action, _ := pa.GetString("turn.action_type")
		seq, _ := pa.GetInt("turn.seq")
		want := turns.OutcomeTimeout
		if gatewayTold(s.action) {
			want = turns.OutcomeDegraded
		}
		if status != want || action != s.action || seq != i+1 {
			t.Errorf("turn %d (%s): turn.completed %s #%d with status %q, want #%d with %q", i+1, s.action, action, seq, status, i+1, want)
		}
		switch status {
		case turns.OutcomeDegraded:
			degraded++
		case turns.OutcomeTimeout:
			timedOut++
		}
		// A rest and a leave are the turns whose mechanics the gateway has
		// before they complete on every run: their fact settles with the step,
		// and only then does the clock pass their deadline.
		if s.action == api.ActionRest || s.action == api.ActionLeave {
			if result, _ := pa.GetString("delivery.result_event_id"); result == "" || !pa.Has("timings.mechanics_at") {
				t.Errorf("turn %d (%s): turn.completed without its mechanics: result_event_id %q, mechanics_at %v",
					i+1, s.action, result, pa.Has("timings.mechanics_at"))
			}
		}
	}

	started, ended := gatewaySessions(w.journal)
	if len(started) != 1 || len(ended) != 1 {
		t.Fatalf("sessions: %d started, %d ended; want one of each", len(started), len(ended))
	}
	sp, ep := started[0].Path(), ended[0].Path()
	id, _ := sp.GetString("session.id")
	endedID, _ := ep.GetString("session.id")
	reason, _ := ep.GetString("session.end_reason")
	count, _ := ep.GetInt("session.turns_count")
	gotDegraded, _ := ep.GetInt("session.turns_degraded")
	failed, _ := ep.GetInt("session.turns_failed")
	if id == "" || endedID != id || reason != session.EndIdle || count != gatewaySolo30Turns ||
		gotDegraded != degraded || failed != timedOut {
		t.Errorf("session %q ended as %q with %s, %d turns, %d degraded, %d failed; want %d, %d, %d",
			id, endedID, reason, count, gotDegraded, failed, gatewaySolo30Turns, degraded, timedOut)
	}
	for _, ev := range w.journal.ofType(turns.TypeCompleted) {
		if sid, _ := ev.Path().GetString("session.id"); sid != id {
			t.Errorf("turn.completed %s is of session %q, want %q", ev.ID, sid, id)
		}
	}

	for _, r := range w.journal.events {
		if r.topic != eventbus.TopicAnalyticsEvents {
			continue
		}
		for _, forbidden := range []string{"Вася", gatewaySolo30Say, "external"} {
			if strings.Contains(r.body, forbidden) {
				t.Errorf("%s %s carries %q", r.ev.Type, r.ev.ID, forbidden)
			}
		}
	}
	gatewayAssertFixtureShape(t, w)
}

// gatewayFixtureGaps are the fields the gateway publishes and
// testdata/analytics/solo-30.jsonl has no example of yet, each with the reason
// it is not closed here. The check fails on a gap that is not listed, and on a
// listed gap the fixture has closed, so the list cannot go stale.
var gatewayFixtureGaps = map[string]string{
	// The decision of a combat turn names its phase1_mode (C-10); the fixture
	// was written by TestSolo30Fixture from facts without it. The fixture is
	// regenerated only by that test (conditions of tech-lead#1, card of T-306):
	// backlog of T-313.
	turns.TypeCompleted + " payload.turn.phase1_mode": "TestSolo30Fixture writes no phase1_mode",
}

// gatewayAssertFixtureShape holds testdata/analytics/solo-30.jsonl to the
// analytics the gateway publishes in a real run. The fixture of EPIC-005 is
// written by one test only, TestSolo30Fixture of internal/gateway/turns
// (conditions of its owner, card of T-306), so this run does not rewrite it;
// instead every field the run publishes in an analytics event of the gateway
// must be a field the fixture has in an event of the same type, save the gaps
// listed in gatewayFixtureGaps. A field the gateway starts to publish and the
// fixture lacks fails here, before the report of EPIC-005 meets it on the
// stand.
func gatewayAssertFixtureShape(t *testing.T, w *gatewayWorld) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "analytics", "solo-30.jsonl"))
	if err != nil {
		t.Fatalf("read the fixture of EPIC-005: %v", err)
	}
	fixture := map[string]map[string]bool{}
	for i, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		doc := map[string]any{}
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("fixture line %d: %v", i+1, err)
		}
		typ, _ := doc["type"].(string)
		if fixture[typ] == nil {
			fixture[typ] = map[string]bool{}
		}
		for _, p := range gatewayShape(doc) {
			fixture[typ][p] = true
		}
	}
	for gap := range gatewayFixtureGaps {
		typ, path, _ := strings.Cut(gap, " ")
		if fixture[typ][path] {
			t.Errorf("the fixture has %s now: remove it from gatewayFixtureGaps", gap)
		}
	}
	missing := map[string]bool{}
	for _, r := range w.journal.events {
		if r.topic != eventbus.TopicAnalyticsEvents || r.ev.Source != contracts.SourceGateway {
			continue
		}
		doc := map[string]any{}
		if err := json.Unmarshal([]byte(r.body), &doc); err != nil {
			t.Fatal(err)
		}
		if fixture[r.ev.Type] == nil {
			missing[r.ev.Type] = true
			continue
		}
		for _, p := range gatewayShape(doc) {
			if gap := r.ev.Type + " " + p; !fixture[r.ev.Type][p] && gatewayFixtureGaps[gap] == "" {
				missing[gap] = true
			}
		}
	}
	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		t.Errorf("the gateway publishes analytics fields testdata/analytics/solo-30.jsonl has no example of: %s", strings.Join(keys, ", "))
	}
}

// gatewayShape is every path of a document with the indices of arrays taken
// out: items[0].id and items[3].id are one field.
func gatewayShape(doc map[string]any) []string {
	paths := jsonpath.New(doc).GetAllPaths()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, gatewayIndex.ReplaceAllString(p, "[]"))
	}
	return out
}

var gatewayIndex = regexp.MustCompile(`\[\d+\]`)
