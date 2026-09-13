package gateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/characters"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// state stands for State on the bus of a started context: every proposal of a
// character becomes its entity.created, which the consumer of the context
// applies to the projection.
func state(t *testing.T, r running) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		<-done
	})
	go func() {
		defer close(done)
		_ = r.bus.Subscribe(ctx, eventbus.TopicSystemEvents, "test-state", func(ctx context.Context, ev eventbus.Event) error {
			if ev.Type != characters.TypeCreateProposed {
				return nil
			}
			pa := ev.Path()
			id, _ := pa.GetString("entity.entity.id")
			name, _ := pa.GetString("entity.name")
			proposalID, _ := pa.GetString("proposal_id")
			raw, _ := json.Marshal(map[string]any{
				"entity":  map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}, "name": name},
				"version": 1, "attributes": ev.Payload["attributes"], "proposal_id": proposalID,
			})
			var payload map[string]any
			_ = json.Unmarshal(raw, &payload)
			return r.bus.Publish(ctx, eventbus.Derive(ev, "entity.created", contracts.SourceState, payload, eventbus.WithCauseID(id)))
		})
	}()
}

// The routes of characters through the real middleware, handlers, stores and
// bus: the world list, a character created by its fact and read back, the
// status of the link, and the analytics of the first action of its scope.
func TestTheContextServesCharactersWorldsAndPlayers(t *testing.T) {
	r := start(t, runtime.ModeLive)
	state(t, r)
	ctx := context.Background()
	m := gateway.ReadModel(r.ctx)
	for _, ev := range []eventbus.Event{
		created(t, world, entity.TypeWorld, "Тёмный лес", map[string]any{"laws_version": "1", "locale": "ru"}),
		created(t, "dark-forest-01", entity.TypeRegion, "Опушка", map[string]any{"description": "лес"}),
	} {
		if _, err := m.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}

	worlds, err := r.client.Worlds(ctx)
	if err != nil || len(worlds.Worlds) != 1 {
		t.Fatalf("Worlds = %+v %v", worlds, err)
	}
	w := worlds.Worlds[0]
	if w.WorldID != world || w.Name != "Тёмный лес" || w.LawsVersion != "1" || w.LLM.CloudEnabled ||
		len(w.Regions) != 1 || w.Regions[0].RegionID != "dark-forest-01" || w.Regions[0].Name != "Опушка" {
		t.Errorf("world = %+v", w)
	}

	consent(t, r.client, externalID)
	req := api.CreateCharacterRequest{ExternalPlatform: links.PlatformTelegram, ExternalID: externalID, WorldID: world,
		CharacterName: "Вася", ActionKey: "create-1"}
	res, status, err := r.client.CreateCharacter(ctx, req)
	if err != nil || status != http.StatusCreated || res.Character == nil || res.Character.Status != entity.StatusAlive {
		t.Fatalf("CreateCharacter = %d %+v %v", status, res, err)
	}
	again, status, err := r.client.CreateCharacter(ctx, req)
	if err != nil || status != http.StatusCreated || again.PlayerID != res.PlayerID {
		t.Errorf("repeat = %d %+v %v", status, again, err)
	}
	player, err := r.client.Player(ctx, res.PlayerID)
	if err != nil || player.Name != "Вася" || player.HP == nil || *player.HP != characters.StartHPMax || player.Session != nil {
		t.Errorf("Player = %+v %v", player, err)
	}
	resolved, err := r.client.Resolve(ctx, links.PlatformTelegram, externalID)
	if err != nil || resolved.CharacterStatus != characters.StatusAlive || resolved.PlayerID == nil || *resolved.PlayerID != res.PlayerID {
		t.Errorf("Resolve = %+v %v", resolved, err)
	}

	// The first action of the character opens its session; GET shows it.
	a, err := r.client.Action(ctx, res.PlayerID, api.ActionRequest{ActionKey: "look-1", Type: api.ActionLook})
	if err != nil || a.Accepted == nil || a.Accepted.Turn.Seq != 1 {
		t.Fatalf("Action = %+v %v", a, err)
	}
	player, err = r.client.Player(ctx, res.PlayerID)
	if err != nil || player.Session == nil || player.Session.SessionID != a.Accepted.Turn.SessionID || player.Session.TurnsCount != 1 {
		t.Errorf("Player after an action = %+v %v", player.Session, err)
	}
	records, err := r.bus.Records(eventbus.TopicAnalyticsEvents)
	if err != nil || len(records) != 1 || !strings.Contains(string(records[0]), session.TypeStarted) {
		t.Fatalf("analytics_events = %d %v, want session.started", len(records), err)
	}
	if strings.Contains(string(records[0]), externalID) || strings.Contains(string(records[0]), "Вася") {
		t.Errorf("session.started carries the account or the name: %s", records[0])
	}

	if _, err := r.client.Player(ctx, "player-nobody"); err == nil || !strings.Contains(err.Error(), api.CodePlayerNotFound) {
		t.Errorf("Player of nobody = %v", err)
	}
}

// A creation whose fact is late answers 202; the refusal of State that comes
// later reaches the character through the consumer of the context, and the
// sweeper takes it off its link (component §7.1).
func TestAStateRefusalAfterCreatingReachesTheLinkThroughTheConsumerAndTheSweeper(t *testing.T) {
	r := start(t, runtime.ModeLive)
	ctx := context.Background()
	if _, err := gateway.ReadModel(r.ctx).Apply(created(t, world, entity.TypeWorld, "Тёмный лес", map[string]any{"laws_version": "1"})); err != nil {
		t.Fatal(err)
	}
	consent(t, r.client, externalID)
	type result struct {
		res    api.CreateCharacterResponse
		status int
		err    error
	}
	answered := make(chan result, 1)
	go func() {
		res, status, err := r.client.CreateCharacter(ctx, api.CreateCharacterRequest{ExternalPlatform: links.PlatformTelegram,
			ExternalID: externalID, WorldID: world, CharacterName: "Вася", ActionKey: "create-1"})
		answered <- result{res, status, err}
	}()
	var got result
	for waited := false; !waited; {
		select {
		case got = <-answered:
			waited = true
		default:
			// The wait for the fact is armed on the manual clock at some
			// moment of the request; moving the clock past it in steps keeps
			// the deadline of the character far away.
			r.clock.Advance(characters.DefaultWait)
			runtimeYield()
		}
	}
	if got.err != nil || got.status != http.StatusAccepted || got.res.Status != characters.StatusCreating {
		t.Fatalf("CreateCharacter = %d %+v %v", got.status, got.res, got.err)
	}
	proposals, err := r.bus.Records(eventbus.TopicSystemEvents)
	if err != nil {
		t.Fatal(err)
	}
	var proposal eventbus.Event
	for _, raw := range proposals {
		var ev eventbus.Event
		if err := json.Unmarshal(raw, &ev); err == nil && ev.Type == characters.TypeCreateProposed {
			proposal = ev
		}
	}
	proposalID, _ := proposal.Path().GetString("proposal_id")
	if err := r.bus.Publish(ctx, eventbus.Derive(proposal, "entity.update.rejected", contracts.SourceState,
		map[string]any{"proposal_id": proposalID, "reason": "duplicate_entity"}, eventbus.WithCauseID("rejected"))); err != nil {
		t.Fatal(err)
	}
	eventually(t, "character_status none after the refusal", func() bool {
		res, err := r.client.Resolve(ctx, links.PlatformTelegram, externalID)
		return err == nil && res.CharacterStatus == characters.StatusNone
	})
	// One tick of the sweeper, at the next minute: the refusal expired the
	// character at once, so this very tick takes it off its link.
	r.clock.Advance(time.Minute)
	eventually(t, "the link without its character after the sweep", func() bool {
		res, err := r.client.Resolve(ctx, links.PlatformTelegram, externalID)
		return err == nil && res.PlayerID == nil
	})
}

// eventually polls cond for up to five seconds of wall time: the consumer and
// the sweeper run in goroutines of the context.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for range 500 {
		if cond() {
			return
		}
		runtimeYield()
	}
	t.Fatalf("%s: not reached", what)
}

func runtimeYield() { time.Sleep(10 * time.Millisecond) }

// In replay the sweeper does not run and no analytics are published, while
// the session of an action is still kept (C-10, component §11.2).
func TestReplayKeepsSessionsWithoutAnalytics(t *testing.T) {
	r := start(t, runtime.ModeReplay)
	withCharacter(t, r)
	ctx := context.Background()
	a, err := r.client.Action(ctx, "player-A", api.ActionRequest{ActionKey: "look-1", Type: api.ActionLook})
	if err != nil || a.Accepted == nil {
		t.Fatalf("Action = %+v %v", a, err)
	}
	r.clock.Advance(time.Hour)
	if records, _ := r.bus.Records(eventbus.TopicAnalyticsEvents); len(records) != 0 {
		t.Errorf("replay published %d analytics", len(records))
	}
	player, err := r.client.Player(ctx, "player-A")
	if err != nil || player.Session == nil || player.Session.TurnsCount != 1 {
		t.Errorf("Player = %+v %v", player.Session, err)
	}
}

// The variables of sessions, turns and characters are durations above zero; a
// start with one that is not fails before a file is opened.
func TestStartRefusesDurationsOfSessionsAndCharactersItCannotUse(t *testing.T) {
	for _, name := range []string{"MV_GATEWAY_SESSION_IDLE", "MV_GATEWAY_TURN_TIMEOUT", "MV_GATEWAY_CHARACTER_WAIT", "MV_GATEWAY_CHARACTER_DEADLINE"} {
		for _, value := range []string{"0s", "-1s", "soon"} {
			r, mux, deps := build(t, runtime.ModeLive, sqlitedir.Temp(t), options{vars: map[string]string{name: value}})
			_ = mux
			err := r.ctx.Start(context.Background(), deps)
			if err == nil {
				_ = r.ctx.Stop(context.Background())
				t.Errorf("%s=%s started", name, value)
				continue
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("%s=%s: error %v does not name the variable", name, value, err)
			}
		}
	}
}

// The wait for the fact of a character is shorter than its deadline (N-5 of
// review #1 of T-306): a start with MV_GATEWAY_CHARACTER_WAIT at or past
// MV_GATEWAY_CHARACTER_DEADLINE fails and names the variable.
func TestStartRefusesACharacterWaitNotShorterThanItsDeadline(t *testing.T) {
	for _, tc := range []struct {
		wait, deadline string
		starts         bool
	}{
		{"60s", "60s", false},
		{"2m", "60s", false},
		{"59s", "60s", true},
	} {
		r, _, deps := build(t, runtime.ModeLive, sqlitedir.Temp(t), options{vars: map[string]string{
			"MV_GATEWAY_CHARACTER_WAIT": tc.wait, "MV_GATEWAY_CHARACTER_DEADLINE": tc.deadline,
		}})
		err := r.ctx.Start(context.Background(), deps)
		if err == nil {
			_ = r.ctx.Stop(context.Background())
		}
		switch {
		case tc.starts && err != nil:
			t.Errorf("wait %s, deadline %s: %v", tc.wait, tc.deadline, err)
		case !tc.starts && err == nil:
			t.Errorf("wait %s, deadline %s started", tc.wait, tc.deadline)
		case !tc.starts && !strings.Contains(err.Error(), "MV_GATEWAY_CHARACTER_WAIT"):
			t.Errorf("wait %s, deadline %s: error %v does not name the variable", tc.wait, tc.deadline, err)
		}
	}
}
