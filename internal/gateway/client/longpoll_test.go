package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
)

const deliveriesBody = `{"deliveries":[{"id":"d-1","player_id":"player-B","route":{"external_platform":"telegram","external_id":"42"},"kind":"mechanics","correlation_id":"ev-1","event_id":"ev-3","generated_by":"rules","text":"Попадание!","created_at":"2026-09-13T10:00:00Z"}],"cursor":"c-2"}`

func TestDeliveriesAddressesThisClientAndClampsTheLimits(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusOK, body: deliveriesBody})
	c, _ := newClient(g)

	out, err := c.Deliveries(context.Background(), "c-1", 500, time.Minute)
	if err != nil {
		t.Fatalf("Deliveries: %v", err)
	}
	if out.Cursor != "c-2" || len(out.Deliveries) != 1 || out.Deliveries[0].Route == nil || out.Deliveries[0].Route.ExternalID != "42" {
		t.Errorf("answer = %+v", out)
	}

	req := g.Seen()[0]
	if req.Method != http.MethodGet || req.Path != "/v1/clients/telegram-bot/deliveries" {
		t.Errorf("request = %s %s", req.Method, req.Path)
	}
	if got := req.Header.Get(api.HeaderClientID); got != "telegram-bot" {
		t.Errorf("X-Client-Id = %q, must equal {client_id} of the path (SEC-12)", got)
	}
	q, _ := url.ParseQuery(req.Query)
	want := url.Values{"after": {"c-1"}, "limit": {"100"}, "wait_ms": {"25000"}}
	if q.Encode() != want.Encode() {
		t.Errorf("query = %q, want %q", q.Encode(), want.Encode())
	}
}

func TestDeliveriesLeavesDefaultsToTheGateway(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusOK, body: `{"deliveries":[],"cursor":""}`})
	c, _ := newClient(g)
	out, err := c.Deliveries(context.Background(), "", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Deliveries == nil || len(out.Deliveries) != 0 {
		t.Errorf("deliveries = %#v, want an empty list", out.Deliveries)
	}
	if q := g.Seen()[0].Query; q != "" {
		t.Errorf("query = %q, want none", q)
	}

	if _, err := c.Deliveries(context.Background(), "", 20, 0); err != nil {
		t.Fatal(err)
	}
	if q := g.Seen()[1].Query; q != "limit=20&wait_ms=0" {
		t.Errorf("query = %q, want limit=20&wait_ms=0 (wait 0 is an immediate answer)", q)
	}
}

// The long-poll is not repeated inside the client: a repeat could meet the
// abandoned poll of the same client and get 409 poll_in_progress.
func TestDeliveriesIsNotRepeated(t *testing.T) {
	for _, rep := range []reply{
		{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")},
		{},
	} {
		g := newFakeGateway(t, rep)
		c, timers := newClient(g)
		if _, err := c.Deliveries(context.Background(), "", 0, time.Second); err == nil {
			t.Errorf("status %d: want an error", rep.status)
		}
		if n := len(g.Seen()); n != 1 || len(timers.Pauses()) != 0 {
			t.Errorf("status %d: requests %d, pauses %v; want 1 and none", rep.status, n, timers.Pauses())
		}
	}
}

// A long-poll ends when its context ends, not when the gateway answers: the
// delivery loop of the bot stops within its shutdown deadline.
func TestDeliveriesStopsWhenTheContextEnds(t *testing.T) {
	g := newFakeGateway(t, reply{status: holdReply})
	c, timers := newClient(g)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := c.Deliveries(ctx, "", 0, 25*time.Second)
		done <- err
	}()
	select {
	case <-g.hits:
	case err := <-done:
		t.Fatalf("Deliveries returned before the gateway saw the poll: %v", err)
	}
	cancel()
	// The gateway holds the poll for good, so only the cancel can end it; if it
	// did not, the 5 s timeout of the test HTTP client would, with another error.
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if n := len(g.Seen()); n != 1 || len(timers.Pauses()) != 0 {
		t.Errorf("requests %d, pauses %v; want 1 and none", n, timers.Pauses())
	}
}
func TestDeliveriesPollInProgressIsAPIError(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusConflict, body: errorBody(api.CodePollInProgress, "")})
	c, _ := newClient(g)
	_, err := c.Deliveries(context.Background(), "", 0, time.Second)
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != api.CodePollInProgress {
		t.Errorf("err = %v, want APIError poll_in_progress", err)
	}
}

func TestAckIsRepeatedWithTheSameIDs(t *testing.T) {
	g := newFakeGateway(t,
		reply{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")},
		reply{status: http.StatusOK, body: `{"acked":1,"unknown":["d-9"]}`})
	c, timers := newClient(g)

	out, err := c.Ack(context.Background(), []string{"d-1", "d-9"})
	if err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if out.Acked != 1 || !slices.Equal(out.Unknown, []string{"d-9"}) {
		t.Errorf("answer = %+v", out)
	}
	seen := g.Seen()
	if len(seen) != 2 || len(timers.Pauses()) != 1 {
		t.Fatalf("requests %d, pauses %v; want 2 and 1", len(seen), timers.Pauses())
	}
	for i, req := range seen {
		var body api.AckRequest
		if err := json.Unmarshal(req.Body, &body); err != nil || !slices.Equal(body.IDs, []string{"d-1", "d-9"}) {
			t.Errorf("request %d: body %s (%v)", i, req.Body, err)
		}
		if req.Method != http.MethodPost || req.Path != "/v1/clients/telegram-bot/deliveries/ack" {
			t.Errorf("request %d = %s %s", i, req.Method, req.Path)
		}
	}
}
