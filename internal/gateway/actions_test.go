package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// created is entity.created of State as the consumer would apply it.
func created(t *testing.T, id, typ, name string, attrs map[string]any) eventbus.Event {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"entity":  map[string]any{"entity": map[string]any{"id": id, "type": typ}, "name": name},
		"version": 1, "attributes": attrs, "proposal_id": "create-" + id,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return eventbus.Event{ID: "created-" + id, Type: readmodel.TypeEntityCreated, Timestamp: t0, Source: contracts.SourceState,
		World:   &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: world, Type: entity.TypeWorld}},
		Meta:    eventbus.Meta{SchemaVersion: 1, CorrelationID: "create-" + id, ActorKind: eventbus.ActorSystem, Locale: "ru", GMPath: "agent"},
		Payload: payload}
}

// withCharacter puts a region and a character outside it into the projection
// of a started context.
func withCharacter(t *testing.T, r running) {
	t.Helper()
	m := gateway.ReadModel(r.ctx)
	for _, ev := range []eventbus.Event{
		created(t, "dark-forest-01", entity.TypeRegion, "Опушка", map[string]any{"description": "лес"}),
		created(t, "player-A", entity.TypePlayer, "Вася", map[string]any{
			"hp": 10, "hp_max": 10, "status": "alive", "position": actions.OutsidePosition(world),
			"scope": map[string]any{"id": "player-A", "type": "solo"}, "actor_kind": "human",
		}),
	} {
		if _, err := m.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
}

// An action goes through the real middleware, handler, store of keys and bus:
// 202 with the correlation of the published player.* event, the same answer
// to a repeat, and a refusal with its code (C-08).
func TestTheContextAcceptsActions(t *testing.T) {
	r := start(t, runtime.ModeLive)
	withCharacter(t, r)
	ctx := context.Background()

	target := "dark-forest-01"
	req := api.ActionRequest{ActionKey: "update-1", Type: api.ActionEnter, Target: &target}
	first, err := r.client.Action(ctx, "player-A", req)
	if err != nil || first.Status != http.StatusAccepted || first.Accepted == nil {
		t.Fatalf("Action = %+v %v", first, err)
	}
	again, err := r.client.Action(ctx, "player-A", req)
	if err != nil || again.Accepted == nil || again.Accepted.CorrelationID != first.Accepted.CorrelationID {
		t.Fatalf("repeat = %+v %v", again, err)
	}
	records, err := r.bus.Records(eventbus.TopicPlayerEvents)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || !strings.Contains(string(records[0]), first.Accepted.CorrelationID) ||
		!strings.Contains(string(records[0]), actions.TypeEnteredRegion) {
		t.Fatalf("player_events = %d records, want the one action", len(records))
	}

	_, err = r.client.Action(ctx, "player-A", api.ActionRequest{ActionKey: "update-2", Type: api.ActionFlee})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != api.CodeNotInEncounter {
		t.Errorf("flee outside an encounter = %v, want 409 not_in_encounter", err)
	}
}

// The rate limit answers 429 with Retry-After in live mode, before the
// handler, and does not exist in replay (SEC-11, component §5.1 p. 6).
func TestTheRateLimitIsALimitOfLiveMode(t *testing.T) {
	for _, mode := range []runtime.Mode{runtime.ModeLive, runtime.ModeReplay} {
		t.Run(string(mode), func(t *testing.T) {
			r := start(t, mode)
			var limited int
			for i := range 7 {
				res, err := r.client.Action(context.Background(), "player-Z", api.ActionRequest{ActionKey: "k" + string(rune('a'+i)), Type: api.ActionLook})
				var apiErr *client.APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("Action = %+v %v", res, err)
				}
				switch apiErr.Code {
				case api.CodeRateLimited:
					limited++
				case api.CodePlayerNotFound:
				default:
					t.Fatalf("Action %d = %v", i, err)
				}
			}
			want := 2
			if mode == runtime.ModeReplay {
				want = 0
			}
			if limited != want {
				t.Errorf("%d of 7 actions limited, want %d", limited, want)
			}
		})
	}
}

// Retry-After of a limited action is the wait of the bucket in whole seconds.
func TestALimitedActionCarriesRetryAfter(t *testing.T) {
	r := start(t, runtime.ModeLive)
	var header string
	for i := range 6 {
		body := strings.NewReader(`{"action_key":"k` + string(rune('a'+i)) + `","type":"look"}`)
		req, err := http.NewRequest(http.MethodPost, r.client.BaseURL+"/v1/players/player-Z/actions", body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(api.HeaderClientID, "telegram-bot")
		req.Header.Set("Content-Type", api.ContentTypeJSON)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests {
			header = res.Header.Get(api.HeaderRetryAfter)
		}
	}
	if header != "2" {
		t.Errorf("Retry-After = %q, want 2", header)
	}
}

// A client of MV_GATEWAY_ACTOR_KIND_CLIENTS that sends X-Actor-Kind: ci gets
// its action on the bus as ci (C-10; review #1, Mi-1).
func TestTheActorKindHeaderReachesTheBus(t *testing.T) {
	r := start(t, runtime.ModeLive)
	withCharacter(t, r)
	harness := client.New(r.client.BaseURL, "ci-harness")
	harness.Backoff = client.NoRetry
	harness.ActorKind = api.ActorCI
	if _, err := harness.Action(context.Background(), "player-A", api.ActionRequest{ActionKey: "k", Type: api.ActionLook}); err != nil {
		t.Fatal(err)
	}
	records, err := r.bus.Records(eventbus.TopicPlayerEvents)
	if err != nil || len(records) != 1 {
		t.Fatalf("player_events = %d records, %v", len(records), err)
	}
	var ev eventbus.Event
	if err := json.Unmarshal(records[0], &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Type != actions.TypeLooked || ev.Meta.ActorKind != api.ActorCI {
		t.Errorf("%s with actor_kind %q, want player.looked with ci", ev.Type, ev.Meta.ActorKind)
	}
}

// failingProposals is the bus of a broker that takes player.* and refuses the
// proposals.
type failingProposals struct{ eventbus.Bus }

func (b failingProposals) Publish(ctx context.Context, ev eventbus.Event) error {
	if ev.Type == actions.TypeUpdateProposed {
		return errors.New("broker refuses")
	}
	return b.Bus.Publish(ctx, ev)
}

// The sweeper of live mode forgets the players who stopped acting and the half
// published actions whose key expired (review #1, Mi-5 and Ma-1).
func TestTheSweeperForgetsIdlePlayersAndExpiredBatches(t *testing.T) {
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		bus: func(bus *membus.Bus) eventbus.Bus { return failingProposals{Bus: bus} },
	})
	withCharacter(t, r)
	target := "dark-forest-01"
	_, err := r.client.Action(context.Background(), "player-A", api.ActionRequest{ActionKey: "k", Type: api.ActionEnter, Target: &target})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != api.CodeBusUnavailable {
		t.Fatalf("Action = %v, want 503 bus_unavailable", err)
	}
	limiter := gateway.Limiter(r.ctx)
	if limiter.Players() != 1 || gateway.PendingActions(r.ctx) != 1 {
		t.Fatalf("limiter players %d, pending %d; want 1 and 1", limiter.Players(), gateway.PendingActions(r.ctx))
	}
	// The sweeper registers its tickers in its own goroutine: advance until
	// its ticks land, within a bound.
	deadline := testkit.After(10 * time.Second)
	for limiter.Players() != 0 {
		r.clock.Advance(actions.LimiterSweepInterval)
		select {
		case <-deadline:
			t.Fatalf("limiter players %d after the sweeps", limiter.Players())
		case <-testkit.After(20 * time.Millisecond):
		}
	}
	if n := gateway.PendingActions(r.ctx); n != 1 {
		t.Fatalf("pending %d before the expiry of the key, want 1", n)
	}
	r.clock.Set(t0.Add(store.KeyTTL))
	for gateway.PendingActions(r.ctx) != 0 {
		r.clock.Advance(store.SweepInterval)
		select {
		case <-deadline:
			t.Fatalf("pending %d after the expiry of the key", gateway.PendingActions(r.ctx))
		case <-testkit.After(20 * time.Millisecond):
		}
	}
}

// A start with a filter the gateway does not have, or with a rate limit it
// cannot run, fails before a file is opened (component §5.4, §11.3).
func TestStartRefusesTheSettingsOfActionsItCannotRun(t *testing.T) {
	cases := map[string]map[string]string{
		"filter":         {env.GatewayInputFilter.Name(): "other"},
		"rate":           {env.GatewayRateActionsPerMin.Name(): "0"},
		"burst":          {env.GatewayRateActionsBurst.Name(): "many"},
		"grace":          {env.GatewayEncounterGrace.Name(): "soon"},
		"negative grace": {env.GatewayEncounterGrace.Name(): "-1s"},
		"gm path":        {env.GMPath.Name(): "both"},
	}
	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			dir := sqlitedir.Temp(t)
			vars[env.GatewayDataDir.Name()] = dir
			c := gateway.New(env.MapSource(vars))
			manual := clock.NewManual(t0)
			bus := newBus(t)
			err := c.Start(context.Background(), runtime.Deps{Clock: manual, Timers: manual.Timers(), IDs: sequence(),
				Log: slog.New(slog.DiscardHandler), Bus: bus, Journal: bus})
			if err == nil {
				_ = c.Stop(context.Background())
				t.Fatal("Start succeeded")
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
				t.Errorf("the refused start left %d files in the data directory (%v)", len(entries), err)
			}
			for key := range vars {
				if key != env.GatewayDataDir.Name() && !strings.Contains(err.Error(), key) {
					t.Errorf("error %q does not name %s", err, key)
				}
			}
			// A value that does not parse is one error (review #1, N-3).
			if name == "burst" && strings.Contains(err.Error(), "at least 1") {
				t.Errorf("error %q also calls the unparsed burst below 1", err)
			}
		})
	}
}
