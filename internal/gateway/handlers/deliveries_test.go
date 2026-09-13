package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
)

func discard() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

type fakeDeliveries struct {
	poll    outbox.Poll
	polls   int
	ackIDs  []string
	ackErr  error
	answer  api.DeliveriesResponse
	deliver []outbox.Delivery
}

func (f *fakeDeliveries) Serve(_ context.Context, p outbox.Poll) (api.DeliveriesResponse, error) {
	f.poll, f.polls = p, f.polls+1
	return f.answer, nil
}

func (f *fakeDeliveries) Ack(ctx context.Context, _ string, ids []string, onDelivered outbox.OnDelivered) (api.AckResponse, error) {
	f.ackIDs = ids
	for _, d := range f.deliver {
		if err := onDelivered(ctx, nil, d, t0); err != nil {
			return api.AckResponse{}, err
		}
	}
	if f.ackErr != nil {
		return api.AckResponse{}, f.ackErr
	}
	return api.AckResponse{Acked: len(ids), Unknown: []string{}}, nil
}

type fakeTurns struct{ delivered []string }

func (f *fakeTurns) OnDelivered(_ context.Context, _ turns.DB, correlationID string, _ time.Time) error {
	f.delivered = append(f.delivered, correlationID)
	return nil
}

// serveDeliveries runs a request through the real middleware of the gateway
// for the client telegram-bot.
func serveDeliveries(t *testing.T, h *handlers.Deliveries, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveAs(t, h, "telegram-bot", []string{"telegram-bot", "ci-harness"}, method, path, body)
}

// serveAs is serveDeliveries for client with the admitted clients given.
func serveAs(t *testing.T, h *handlers.Deliveries, client string, admitted []string, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	cfg := &api.Config{ClientIDs: admitted, RequestIDs: func() string { return "r-1" },
		Clock: clock.NewManual(t0), Log: discard()}
	router := api.GatewayRouter(api.Handlers{
		ResolveLink: http.NotFoundHandler(), ConsentLink: http.NotFoundHandler(), ForgetLink: http.NotFoundHandler(),
		ListWorlds: http.NotFoundHandler(), CreateCharacter: http.NotFoundHandler(), GetPlayer: http.NotFoundHandler(),
		PostAction:     http.NotFoundHandler(),
		PollDeliveries: http.HandlerFunc(h.Poll), AckDeliveries: http.HandlerFunc(h.Ack), StreamDeliveries: http.HandlerFunc(h.Stream),
	})
	mux := http.NewServeMux()
	router.Mount(mux, api.Chain(cfg)...)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(api.HeaderClientID, client)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// wait_ms and limit above their bounds are brought down to them, the defaults
// are the bounds, and the platform of the client is the one of the table
// (C-08 v1.1, SEC-11, SEC-12).
func TestPollClampsItsParametersAndNamesThePlatform(t *testing.T) {
	for _, tc := range []struct {
		query string
		limit int
		wait  time.Duration
		after int64
	}{
		{"", outbox.MaxLimit, outbox.MaxWait, 0},
		{"?limit=1000&wait_ms=99999999999999", outbox.MaxLimit, outbox.MaxWait, 0},
		{"?limit=7&wait_ms=0&after=42", 7, 0, 42},
		{"?wait_ms=1500", outbox.MaxLimit, 1500 * time.Millisecond, 0},
	} {
		svc := &fakeDeliveries{answer: api.DeliveriesResponse{Deliveries: []api.Delivery{}, Cursor: "0"}}
		h := &handlers.Deliveries{Service: svc, Platforms: handlers.ClientPlatforms}
		rec := serveDeliveries(t, h, http.MethodGet, "/v1/clients/telegram-bot/deliveries"+tc.query, "")
		if rec.Code != http.StatusOK || svc.polls != 1 {
			t.Fatalf("%q: %d %s", tc.query, rec.Code, rec.Body)
		}
		p := svc.poll
		if p.Limit != tc.limit || p.Wait != tc.wait || p.After != tc.after || p.ClientID != "telegram-bot" || p.Platform != "telegram" {
			t.Errorf("%q: poll = %+v", tc.query, p)
		}
		if !strings.Contains(rec.Body.String(), `"deliveries":[]`) {
			t.Errorf("%q: body %s", tc.query, rec.Body)
		}
	}
}

func TestPollRefusesParametersThatAreNoNumbers(t *testing.T) {
	for _, query := range []string{"?after=abc", "?after=-1", "?limit=0", "?limit=x", "?wait_ms=-1", "?wait_ms=1.5"} {
		svc := &fakeDeliveries{}
		rec := serveDeliveries(t, &handlers.Deliveries{Service: svc}, http.MethodGet, "/v1/clients/telegram-bot/deliveries"+query, "")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), api.CodeInvalidRequest) || svc.polls != 0 {
			t.Errorf("%q = %d %s", query, rec.Code, rec.Body)
		}
	}
}

// telegram-bot and, provisionally, ci-harness take the deliveries of telegram;
// no other client has a platform (component §5.1; decision 1 of the
// orchestrator on T-307, review #1 Mi-1). ci-harness polls with its platform
// when it is admitted, and is refused before the handler when it is not, as in
// production.
func TestTheHarnessTakesTheDeliveriesOfTelegram(t *testing.T) {
	if got := handlers.ClientPlatforms; len(got) != 2 || got["telegram-bot"] != "telegram" || got["ci-harness"] != "telegram" {
		t.Errorf("ClientPlatforms = %v", got)
	}
	svc := &fakeDeliveries{answer: api.DeliveriesResponse{Deliveries: []api.Delivery{}, Cursor: "0"}}
	h := &handlers.Deliveries{Service: svc, Platforms: handlers.ClientPlatforms}
	rec := serveAs(t, h, "ci-harness", []string{"telegram-bot", "ci-harness"}, http.MethodGet, "/v1/clients/ci-harness/deliveries?wait_ms=0", "")
	if rec.Code != http.StatusOK || svc.poll.ClientID != "ci-harness" || svc.poll.Platform != "telegram" {
		t.Errorf("admitted harness = %d %s, poll %+v", rec.Code, rec.Body, svc.poll)
	}
	svc = &fakeDeliveries{}
	h = &handlers.Deliveries{Service: svc, Platforms: handlers.ClientPlatforms}
	rec = serveAs(t, h, "ci-harness", []string{"telegram-bot"}, http.MethodGet, "/v1/clients/ci-harness/deliveries?wait_ms=0", "")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), api.CodeClientUnknown) || svc.polls != 0 {
		t.Errorf("harness not admitted = %d %s, service called %d times", rec.Code, rec.Body, svc.polls)
	}
	if p := (&handlers.Deliveries{Platforms: handlers.ClientPlatforms}).Platforms["mvctl"]; p != "" {
		t.Errorf("mvctl has the platform %q", p)
	}
}

// An ack moves on the turn of a narrative only; analytics that did not go out
// answer 503 bus_unavailable, another failure 500, and an ack without ids 400.
func TestAckAnswersAndMovesTheTurnOfANarrative(t *testing.T) {
	tr := &fakeTurns{}
	svc := &fakeDeliveries{deliver: []outbox.Delivery{
		{ID: "d-1", Kind: outbox.KindNarrative, CorrelationID: "act-1"},
		{ID: "d-2", Kind: outbox.KindMechanics, CorrelationID: "act-2"},
	}}
	h := &handlers.Deliveries{Service: svc, Turns: tr, Log: discard()}
	rec := serveDeliveries(t, h, http.MethodPost, "/v1/clients/telegram-bot/deliveries/ack", `{"ids":["d-1","d-2"]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"acked":2`) {
		t.Fatalf("ack = %d %s", rec.Code, rec.Body)
	}
	if len(tr.delivered) != 1 || tr.delivered[0] != "act-1" {
		t.Errorf("turns moved on = %v, want only the narrative act-1", tr.delivered)
	}

	for err, code := range map[error]string{
		fmt.Errorf("%w: broker away", turns.ErrPublish): api.CodeBusUnavailable,
		errors.New("disk full"):                         api.CodeInternal,
	} {
		svc := &fakeDeliveries{ackErr: err}
		rec := serveDeliveries(t, &handlers.Deliveries{Service: svc, Log: discard()}, http.MethodPost,
			"/v1/clients/telegram-bot/deliveries/ack", `{"ids":["d-1"]}`)
		if !strings.Contains(rec.Body.String(), code) {
			t.Errorf("ack failing with %v = %d %s, want %s", err, rec.Code, rec.Body, code)
		}
	}
	rec = serveDeliveries(t, &handlers.Deliveries{Service: &fakeDeliveries{}}, http.MethodPost,
		"/v1/clients/telegram-bot/deliveries/ack", `{"ids":[]}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ack without ids = %d %s", rec.Code, rec.Body)
	}
}

func TestStreamIsReserved(t *testing.T) {
	rec := serveDeliveries(t, &handlers.Deliveries{}, http.MethodGet, "/v1/clients/telegram-bot/stream", "")
	if rec.Code != http.StatusNotImplemented || !strings.Contains(rec.Body.String(), api.CodeNotImplemented) {
		t.Errorf("stream = %d %s", rec.Code, rec.Body)
	}
}
