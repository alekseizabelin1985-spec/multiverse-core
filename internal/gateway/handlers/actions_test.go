package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/handlers"
)

type fakeActions struct {
	got    actions.Command
	calls  int
	answer actions.Answer
}

func (f *fakeActions) Submit(_ context.Context, c actions.Command) actions.Answer {
	f.got, f.calls = c, f.calls+1
	return f.answer
}

func postAction(t *testing.T, svc *fakeActions, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := &handlers.Actions{Service: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/players/{player_id}/actions", h.Post)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/players/player-A/actions", strings.NewReader(body)))
	return rec
}

// The handler hands the service the player of the path and the fields of the
// body, and writes what the service answered.
func TestPostActionPassesTheCommandAndWritesTheAnswer(t *testing.T) {
	accepted := api.ActionAccepted{CorrelationID: "ev-1", Turn: api.TurnRef{Seq: 1, SessionID: "player-A:1"}, Status: api.ActionStatusAccepted, AckedAt: t0}
	svc := &fakeActions{answer: actions.Answer{Status: http.StatusAccepted, Accepted: &accepted}}
	rec := postAction(t, svc, `{"action_key":"k","type":"say","text":"привет","target":null}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got api.ActionAccepted
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.CorrelationID != "ev-1" || got.Turn.Seq != 1 {
		t.Errorf("body %s: %v", rec.Body, err)
	}
	c := svc.got
	if c.PlayerID != "player-A" || c.ActionKey != "k" || c.Type != api.ActionSay || c.Target != "" ||
		c.Text == nil || *c.Text != "привет" || c.ActorKind != "" {
		t.Errorf("command = %+v", c)
	}

	svc = &fakeActions{answer: actions.Answer{Status: http.StatusConflict, Err: api.NewError(api.CodeInEncounter, nil)}}
	rec = postAction(t, svc, `{"action_key":"k","type":"enter","target":"dark-forest-01"}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), api.CodeInEncounter) || svc.got.Target != "dark-forest-01" {
		t.Errorf("refusal = %d %s, target %q", rec.Code, rec.Body, svc.got.Target)
	}
}

func TestPostActionRefusesABodyThatIsNoRequest(t *testing.T) {
	svc := &fakeActions{}
	rec := postAction(t, svc, `{"action_key":`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), api.CodeInvalidRequest) || svc.calls != 0 {
		t.Errorf("broken body = %d %s, service called %d times", rec.Code, rec.Body, svc.calls)
	}
}
