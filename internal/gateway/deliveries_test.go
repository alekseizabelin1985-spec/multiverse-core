package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// refuser stands for State refusing every move and rest the gateway proposes,
// the way a projection behind State meets version_conflict.
func refuser(t *testing.T, r running) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		<-done
	})
	go func() {
		defer close(done)
		_ = r.bus.Subscribe(ctx, eventbus.TopicSystemEvents, "test-refuser", func(ctx context.Context, ev eventbus.Event) error {
			if ev.Type != actions.TypeUpdateProposed {
				return nil
			}
			pa := ev.Path()
			proposalID, _ := pa.GetString("proposal_id")
			player, _ := pa.GetString("changes[0].entity.entity.id")
			payload := map[string]any{"proposal_id": proposalID, "reason": "version_conflict",
				"entity": map[string]any{"entity": map[string]any{"id": player, "type": entity.TypePlayer}}}
			return r.bus.Publish(ctx, eventbus.Derive(ev, "entity.update.rejected", contracts.SourceState, payload))
		})
	}()
}

// character creates a consented character of externalID in a world with a
// region, through the routes of the context.
func character(t *testing.T, r running) string {
	t.Helper()
	state(t, r)
	m := gateway.ReadModel(r.ctx)
	for _, ev := range []eventbus.Event{
		created(t, world, entity.TypeWorld, "Тёмный лес", map[string]any{"laws_version": "1", "locale": "ru"}),
		created(t, "dark-forest-01", entity.TypeRegion, "Опушка", map[string]any{"description": "лес"}),
	} {
		if _, err := m.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	consent(t, r.client, externalID)
	res, _, err := r.client.CreateCharacter(context.Background(), api.CreateCharacterRequest{ExternalPlatform: links.PlatformTelegram,
		ExternalID: externalID, WorldID: world, CharacterName: "Вася", ActionKey: "create-1"})
	if err != nil || res.PlayerID == "" {
		t.Fatalf("CreateCharacter = %+v %v", res, err)
	}
	return res.PlayerID
}

func raw(t *testing.T, method, url, clientID string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(api.HeaderClientID, clientID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for range 500 {
		if cond() {
			return
		}
		pause := clock.RealTimers{}.After(10 * time.Millisecond)
		<-pause.C()
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The whole path of a delivery through the context: a refusal of State of a
// move reaches the consumer, the long-poll of the bot gives it with the route
// of the link, the ack takes it; /forget drops what is still pending; another
// client sees neither the queue nor the route (C-08, SEC-12, component §8).
func TestTheContextDeliversThroughTheLongPoll(t *testing.T) {
	r := start(t, runtime.ModeLive)
	player := character(t, r)
	refuser(t, r)
	ctx := context.Background()

	target := "dark-forest-01"
	if _, err := r.client.Action(ctx, player, api.ActionRequest{ActionKey: "enter-1", Type: api.ActionEnter, Target: &target}); err != nil {
		t.Fatalf("Action: %v", err)
	}
	got, err := r.client.Deliveries(ctx, "", 10, 5*time.Second)
	if err != nil || len(got.Deliveries) != 1 {
		t.Fatalf("Deliveries = %+v %v", got, err)
	}
	d := got.Deliveries[0]
	if d.PlayerID != player || d.Kind != outbox.KindSystem || d.Route == nil || d.Route.ExternalPlatform != links.PlatformTelegram ||
		d.Route.ExternalID != externalID || strings.Contains(d.Text, "version") {
		t.Errorf("delivery = %+v", d)
	}
	acked, err := r.client.Ack(ctx, []string{d.ID, "d-unknown"})
	if err != nil || acked.Acked != 1 || len(acked.Unknown) != 1 || acked.Unknown[0] != "d-unknown" {
		t.Errorf("Ack = %+v %v", acked, err)
	}

	status, body := raw(t, http.MethodGet, r.client.BaseURL+"/v1/clients/telegram-bot/deliveries?wait_ms=0", "ci-harness")
	if status != http.StatusForbidden || !strings.Contains(body, api.CodeClientMismatch) {
		t.Errorf("a client polling another client = %d %s, want 403 client_mismatch", status, body)
	}
	status, body = raw(t, http.MethodGet, r.client.BaseURL+"/v1/clients/telegram-bot/stream", "telegram-bot")
	if status != http.StatusNotImplemented || !strings.Contains(body, api.CodeNotImplemented) {
		t.Errorf("stream = %d %s, want 501", status, body)
	}

	if _, err := r.client.Action(ctx, player, api.ActionRequest{ActionKey: "rest-1", Type: api.ActionRest}); err != nil {
		t.Fatalf("rest: %v", err)
	}
	waitFor(t, "the refusal of the rest in the outbox", func() bool {
		states, err := gateway.DeliveryStates(r.ctx)
		return err == nil && states[outbox.StatePending] == 1
	})
	status, body = raw(t, http.MethodGet, r.client.BaseURL+"/v1/clients/mvctl/deliveries?wait_ms=0", "mvctl")
	var empty api.DeliveriesResponse
	if status != http.StatusOK || json.Unmarshal([]byte(body), &empty) != nil || len(empty.Deliveries) != 0 || strings.Contains(body, externalID) {
		t.Errorf("a client without a platform = %d %s, want an empty list", status, body)
	}
	if _, err := r.client.Forget(ctx, links.PlatformTelegram, externalID); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	states, err := gateway.DeliveryStates(r.ctx)
	if err != nil || states[outbox.StatePending] != 0 || states[outbox.StateDropped] != 1 || states[outbox.StateDelivered] != 1 {
		t.Errorf("deliveries after /forget = %v %v, want the pending one dropped", states, err)
	}
}

// The long-poll does not hold the stop of the process: the process server
// begins to stop, and the waiting long-poll of 25 s answers an empty list at
// once, so that Stop returns without an error (C-01 v1.8, component §5.1 p. 8).
func TestTheLongPollAnswersWhenTheProcessStops(t *testing.T) {
	r, _, deps := build(t, runtime.ModeLive, sqlitedir.Temp(t), options{})
	srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
	routes := http.NewServeMux()
	r.ctx.Routes(routes)
	entered := make(chan struct{}, 1)
	srv.Mux.Handle("/v1/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/deliveries") {
			entered <- struct{}{}
		}
		routes.ServeHTTP(w, req)
	}))
	if err := r.ctx.Start(context.Background(), deps); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.ctx.Stop(context.Background()) })
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	bot := client.New("http://"+srv.Addr, "telegram-bot")
	bot.Backoff = client.NoRetry

	type answer struct {
		resp api.DeliveriesResponse
		err  error
	}
	out := make(chan answer, 1)
	go func() {
		resp, err := bot.Deliveries(context.Background(), "", 10, outbox.MaxWait)
		out <- answer{resp, err}
	}()
	<-entered
	began := clock.Real{}.Now()
	if err := srv.Stop(context.Background()); err != nil {
		t.Errorf("Stop = %v, want the long-poll released before ShutdownTimeout", err)
	}
	if took := (clock.Real{}).Now().Sub(began); took >= runtime.ShutdownTimeout {
		t.Errorf("Stop took %s", took)
	}
	limit := clock.RealTimers{}.After(2 * time.Second)
	defer limit.Stop()
	select {
	case a := <-out:
		var apiErr *client.APIError
		if a.err != nil && !errors.As(a.err, &apiErr) {
			t.Fatalf("long-poll = %v", a.err)
		}
		if a.err != nil || len(a.resp.Deliveries) != 0 {
			t.Errorf("long-poll at the stop = %+v %v, want an empty list", a.resp, a.err)
		}
	case <-limit.C():
		t.Fatal("the long-poll did not answer at the stop")
	}
}
