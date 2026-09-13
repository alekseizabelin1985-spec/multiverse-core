package turns_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

var update = flag.Bool("update", false, "write testdata/analytics/solo-30.jsonl from the run")

// solo30Path is the fixture of the analytics of a solo session of 30 turns for
// the report of EPIC-005 (mvctl report, C-10 "Заглушка"). It lives under the
// testdata of the gateway: testdata/analytics/ of the root belongs to EPIC-005.
var solo30Path = filepath.Join("..", "testdata", "analytics", "solo-30.jsonl")

// The fixture is what a solo session of 30 turns publishes on analytics_events
// through the service of actions and the tracker: the start of the session, a
// refused action, 30 turns — mostly told by the model, two by a template, one
// that timed out — and the end of the session by the sweeper. The run is
// deterministic (sequence ids, the manual clock), every line is valid by C-10,
// and no line carries a name or a text of the player. go test -run
// TestSolo30Fixture -update writes the file again.
func TestSolo30Fixture(t *testing.T) {
	f := newFixture(t, options{})
	eventbus.SetClock(f.clock)
	t.Cleanup(func() { eventbus.SetClock(nil) })
	ctx := context.Background()
	say := "Кто здесь?"

	for i := 1; i <= 30; i++ {
		key := "solo-30-" + strconv.Itoa(i)
		var a actions.Answer
		switch i % 3 {
		case 0:
			a = f.svc.Submit(ctx, actions.Command{PlayerID: playerA, ActionKey: key, Type: api.ActionSay, Text: &say, ActorKind: eventbus.ActorCI})
		case 1:
			a = f.submit(t, key, api.ActionLook, "")
		default:
			a = f.submit(t, key, api.ActionEnter, forest)
		}
		acc := accepted(t, a)
		action := f.actionEvent(t, acc.CorrelationID)
		if i == 4 {
			f.clock.Advance(300 * time.Millisecond)
			if refused := f.submit(t, "solo-30-refused", api.ActionAttack, "wolf-alpha"); refused.Err == nil {
				t.Fatalf("attack outside an encounter = %+v", refused)
			}
		}
		if i%3 == 2 {
			f.clock.Advance(120 * time.Millisecond)
			fact := eventbus.Derive(action, "entity.updated", contracts.SourceState, map[string]any{}, eventbus.WithCauseID("fact"))
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, fact, f.clock.Now()) })
		}
		if i == 17 {
			// No narrative: the turn times out below.
			f.clock.Advance(turns.DefaultTimeout)
			if _, err := f.tracker.Sweep(ctx, f.clock.Now()); err != nil {
				t.Fatal(err)
			}
			continue
		}
		f.clock.Advance(1400 * time.Millisecond)
		generatedBy := turns.GeneratedByLLM
		if i == 9 || i == 23 {
			generatedBy = turns.GeneratedByTemplate
		}
		f.inTx(t, func(tx *sql.Tx) error {
			return f.tracker.OnNarrative(ctx, tx, narrative(t, action, generatedBy, playerA))
		})
		f.clock.Advance(90 * time.Millisecond)
		f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, acc.CorrelationID, f.clock.Now()) })
		f.clock.Advance(18 * time.Second)
	}
	f.clock.Advance(session.DefaultIdle)
	if n, err := f.sessions.Sweep(ctx, f.clock.Now()); err != nil || n != 1 {
		t.Fatalf("Sweep = %d %v", n, err)
	}

	raw, err := f.bus.Records(eventbus.TopicAnalyticsEvents)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	counts := map[string]int{}
	for _, line := range raw {
		var ev eventbus.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatal(err)
		}
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("%s %s is not valid by C-10: %v", ev.Type, ev.ID, err)
		}
		status, _ := ev.Path().GetString("turn.status")
		counts[ev.Type+":"+status]++
		for _, forbidden := range []string{"Вася", say, "external"} {
			if bytes.Contains(line, []byte(forbidden)) {
				t.Errorf("%s %s carries %q", ev.Type, ev.ID, forbidden)
			}
		}
		// Decision 1 of the orchestrator (Mi-5 of review #1 of T-306): the
		// internal ids a metric needs may go, a name, a scope, a copy of the
		// world and an external id may not — neither in the envelope nor at
		// any depth of the payload.
		if ev.Scope != nil {
			t.Errorf("%s %s carries a scope in its envelope", ev.Type, ev.ID)
		}
		if key := forbiddenKey(ev.Payload); key != "" {
			t.Errorf("%s %s carries the key %q in its payload", ev.Type, ev.ID, key)
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	want := map[string]int{
		session.TypeStarted + ":": 1, session.TypeEnded + ":": 1,
		turns.TypeCompleted + ":" + turns.OutcomeOK: 27, turns.TypeCompleted + ":" + turns.OutcomeDegraded: 2,
		turns.TypeCompleted + ":" + turns.OutcomeTimeout: 1, turns.TypeCompleted + ":" + turns.OutcomeRejected: 1,
	}
	for k, n := range want {
		if counts[k] != n {
			t.Errorf("%s = %d, want %d (all: %v)", k, counts[k], n, counts)
		}
	}
	if ended := f.bus.ofType(t, eventbus.TopicAnalyticsEvents, session.TypeEnded); len(ended) == 1 {
		pa := ended[0].Path()
		turnsCount, _ := pa.GetInt("session.turns_count")
		degraded, _ := pa.GetInt("session.turns_degraded")
		failed, _ := pa.GetInt("session.turns_failed")
		if turnsCount != 30 || degraded != 2 || failed != 1 {
			t.Errorf("session.ended counters = %d %d %d, want 30 2 1", turnsCount, degraded, failed)
		}
	}
	if t.Failed() {
		return
	}

	if *update {
		if err := os.MkdirAll(filepath.Dir(solo30Path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(solo30Path, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	golden, err := os.ReadFile(solo30Path)
	if err != nil {
		t.Fatalf("read %s (go test -run TestSolo30Fixture -update writes it): %v", solo30Path, err)
	}
	if got, want := out.String(), strings.ReplaceAll(string(golden), "\r\n", "\n"); got != want {
		t.Errorf("%s differs from the run; go test -run TestSolo30Fixture -update writes it again", solo30Path)
	}
}

// forbiddenKey is the first key of v, at any depth, that analytics must not
// carry: a name, a scope, a copy of the world, an external account or a link.
func forbiddenKey(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for k, inner := range x {
			switch k {
			case "name", "player_name", "text", "scope", "world", "external_id", "external_platform", "username", "chat_id", "link_id":
				return k
			}
			if found := forbiddenKey(inner); found != "" {
				return found
			}
		}
	case []any:
		for _, inner := range x {
			if found := forbiddenKey(inner); found != "" {
				return found
			}
		}
	}
	return ""
}
