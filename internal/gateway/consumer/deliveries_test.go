package consumer_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
)

// linkRows is the links.db of the tests: player → platform.
type linkRows map[string]string

func (l linkRows) ByPlayer(_ context.Context, playerID string) (links.Link, bool, error) {
	platform, ok := l[playerID]
	if !ok {
		return links.Link{}, false, nil
	}
	return links.Link{Platform: platform, PlayerID: &playerID}, true, nil
}

// RouteFor is the route of a linked player; the external ID is made up.
func (l linkRows) RouteFor(_ context.Context, playerID string) (string, string, bool, error) {
	platform, ok := l[playerID]
	return platform, "ext-" + playerID, ok, nil
}

// recorder keeps what the tracker publishes.
type recorder struct {
	mu     sync.Mutex
	events []eventbus.Event
}

func (r *recorder) Publish(_ context.Context, ev eventbus.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return nil
}

func (r *recorder) completed() []eventbus.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []eventbus.Event
	for _, ev := range r.events {
		if ev.Type == turns.TypeCompleted {
			out = append(out, ev)
		}
	}
	return out
}

type feed struct {
	*fixture
	links     linkRows
	queue     *outbox.Store
	tracker   *turns.Tracker
	analytics *recorder
	d         *consumer.Dispatcher
	offsets   map[string]int64
}

// newFeed builds the deliveries over the database of f and the projection m,
// with player-A and player-B on telegram and nobody else linked.
func newFeed(t *testing.T, f *fixture, m *readmodel.Model) *feed {
	t.Helper()
	fd := &feed{fixture: f, links: linkRows{"player-A": links.PlatformTelegram, "player-B": links.PlatformTelegram},
		analytics: &recorder{}, offsets: map[string]int64{}}
	var err error
	if fd.queue, err = outbox.New(outbox.Config{DB: f.db}); err != nil {
		t.Fatal(err)
	}
	sessions, err := session.New(session.Config{DB: f.db})
	if err != nil {
		t.Fatal(err)
	}
	if fd.tracker, err = turns.New(turns.Config{DB: f.db, Sessions: sessions, Bus: fd.analytics, Clock: f.clock}); err != nil {
		t.Fatal(err)
	}
	fd.rebuild(t, m)
	return fd
}

// rebuild stands for the process after a restart: a new projection and a new
// dispatcher over the same gateway.db.
func (fd *feed) rebuild(t *testing.T, m *readmodel.Model) {
	t.Helper()
	fd.model = m
	effects, err := (&consumer.Deliveries{Model: m, Outbox: fd.queue, Links: fd.links, Turns: fd.tracker, Clock: fd.clock}).Effects()
	if err != nil {
		t.Fatal(err)
	}
	fd.d, err = consumer.New(consumer.Config{Bus: fd.bus, Journal: fd.bus, DB: fd.db, Model: m, Clock: fd.clock,
		Timers: fd.clock.Timers(), Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Effects: effects})
	if err != nil {
		t.Fatal(err)
	}
}

// handle hands an event to the consumer at the next offset of its topic.
func (fd *feed) handle(t *testing.T, topic string, ev eventbus.Event) {
	t.Helper()
	pos := eventbus.Position{Topic: topic, Offset: fd.offsets[topic]}
	fd.offsets[topic]++
	if err := fd.d.Handle(eventbus.ContextWithPosition(context.Background(), pos), ev); err != nil {
		t.Fatalf("handle %s %s: %v", ev.Type, ev.ID, err)
	}
}

type row struct {
	Player, Kind, Event, Text, State, Platform, GeneratedBy string
	Data                                                    map[string]any
	RoundSeq                                                sql.NullInt64
	FallbackReason                                          sql.NullString
}

func (fd *feed) rows(t *testing.T) []row {
	t.Helper()
	rs, err := fd.db.QueryContext(context.Background(),
		`SELECT player_id, kind, event_id, text, state, platform, generated_by, COALESCE(data, '{}'), round_seq, fallback_reason
		 FROM deliveries ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rs.Close() }()
	var out []row
	for rs.Next() {
		var r row
		var data string
		if err := rs.Scan(&r.Player, &r.Kind, &r.Event, &r.Text, &r.State, &r.Platform, &r.GeneratedBy, &data, &r.RoundSeq, &r.FallbackReason); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(data), &r.Data); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func mk(id, typ, correlation string, scope *eventbus.ScopeRef, payload map[string]any) eventbus.Event {
	raw, _ := json.Marshal(payload)
	var p map[string]any
	_ = json.Unmarshal(raw, &p)
	if correlation == "" {
		correlation = id
	}
	return eventbus.Event{ID: id, Type: typ, Timestamp: t0, Source: contracts.SourceState, Scope: scope,
		World: &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: world, Type: entity.TypeWorld}},
		Meta: eventbus.Meta{SchemaVersion: 1, CorrelationID: correlation, CausationID: correlation,
			ActorKind: eventbus.ActorSystem, Locale: "ru", GMPath: "agent"},
		Payload: p}
}

func entityRef(id, typ string) map[string]any {
	return map[string]any{"entity": map[string]any{"id": id, "type": typ}}
}

func birth(id, typ, name string, attrs map[string]any) eventbus.Event {
	return mk("created-"+id, readmodel.TypeEntityCreated, "", nil, map[string]any{
		"entity":  map[string]any{"entity": map[string]any{"id": id, "type": typ}, "name": name},
		"version": 1, "attributes": attrs, "proposal_id": "create-" + id,
	})
}

func solo(player string) *eventbus.ScopeRef { return &eventbus.ScopeRef{ID: player, Type: "solo"} }

// seeded is a projection with a region, a wolf, player-A and player-B alone,
// and group-1 of player-B, player-C (unlinked) and player-D (dead).
func seeded(t *testing.T, m *readmodel.Model) *readmodel.Model {
	t.Helper()
	player := func(id, name, status string, scope map[string]any) eventbus.Event {
		return birth(id, entity.TypePlayer, name, map[string]any{"hp": 4, "hp_max": 10, "status": status,
			"position": "outside:" + world, "scope": scope})
	}
	for _, ev := range []eventbus.Event{
		birth("dark-forest-01", entity.TypeRegion, "Тёмный лес", map[string]any{"description": "лес"}),
		birth("wolf-alpha", entity.TypeNPC, "Волк", map[string]any{"hp": 10, "hp_max": 10, "status": "alive"}),
		player("player-A", "Вася", "alive", map[string]any{"id": "player-A", "type": "solo"}),
		player("player-B", "Лена", "alive", map[string]any{"id": "group-1", "type": "group"}),
		player("player-C", "Олег", "alive", map[string]any{"id": "group-1", "type": "group"}),
		player("player-D", "Петя", "dead", map[string]any{"id": "group-1", "type": "group"}),
		birth("group-1", entity.TypeGroup, "Отряд", map[string]any{"leader_id": "player-B", "state": "active",
			"members": []any{
				map[string]any{"player_id": "player-B", "joined_at": "2026-09-13T12:00:00Z", "participation": "active", "missed_rounds": 0},
				map[string]any{"player_id": "player-C", "joined_at": "2026-09-13T12:00:01Z", "participation": "active", "missed_rounds": 0},
				map[string]any{"player_id": "player-D", "joined_at": "2026-09-13T12:00:02Z", "participation": "active", "missed_rounds": 0},
			}}),
	} {
		if _, err := m.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func combat(id string, scope *eventbus.ScopeRef, attacker, defender string) eventbus.Event {
	return mk(id, outbox.TypeCombatDecided, "act-"+id, scope, map[string]any{
		"encounter": entityRef("enc-1", entity.TypeEncounter), "round": map[string]any{"seq": 2}, "action": "attack",
		"attacker": entityRef(attacker, entity.TypePlayer), "defender": entityRef(defender, entity.TypeNPC),
		"outcome": map[string]any{"hit": true, "natural": 15, "damage": 3},
		"hp":      map[string]any{"defender_before": 10, "defender_after": 7, "defender_max": 10},
		"rolls":   []any{}, "rules_version": "1", "phase1_mode": "rules",
	})
}

func fact(id, entityID, typ, cause, correlation string, changed ...map[string]any) eventbus.Event {
	list := make([]any, 0, len(changed))
	for _, c := range changed {
		list = append(list, c)
	}
	return mk(id, readmodel.TypeEntityUpdated, correlation, nil, map[string]any{
		"entity": entityRef(entityID, typ), "version": 2, "changed": list, "cause": cause,
		"proposal_id": "p-" + id, "applied_at": "2026-09-13T12:00:00Z",
	})
}

// A decision reaches the living players of its scope as the text of the rules;
// the same event delivered again by the bus adds no row (component §8.1,
// NFR-013).
func TestADecisionReachesThePlayersOfItsScope(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	fd := newFeed(t, f, seeded(t, f.model))

	hit := combat("c-1", solo("player-A"), "player-A", "wolf-alpha")
	fd.handle(t, eventbus.TopicGameEvents, hit)
	fd.handle(t, eventbus.TopicGameEvents, hit)
	got := fd.rows(t)
	if len(got) != 1 || got[0].Player != "player-A" || got[0].Kind != outbox.KindMechanics || got[0].Platform != "telegram" ||
		got[0].GeneratedBy != outbox.GeneratedByRules || got[0].Text != "Попадание! Урон 3. Волк: 7/10." {
		t.Fatalf("rows = %+v", got)
	}

	fd.handle(t, eventbus.TopicGameEvents, combat("c-2", &eventbus.ScopeRef{ID: "group-1", Type: "group"}, "player-B", "wolf-alpha"))
	var group []string
	for _, r := range fd.rows(t)[1:] {
		group = append(group, r.Player+":"+r.State)
	}
	if !slices.Equal(group, []string{"player-B:pending", "player-C:dropped"}) {
		t.Errorf("group decision = %v, want the living members, the unlinked one dropped", group)
	}
}

// The facts a player hears about (component §8.1): a rest, a move alone, loot,
// a death; the move of a group reaches its living members once, and the fact of
// a player of the group who moved with it reaches nobody.
func TestTheFactsAPlayerHearsAbout(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	fd := newFeed(t, f, seeded(t, f.model))
	for _, ev := range []eventbus.Event{
		fact("u-rest", "player-A", entity.TypePlayer, outbox.CauseRest, "", map[string]any{"path": "hp", "old": 4, "new": 10}),
		fact("u-move", "player-A", entity.TypePlayer, outbox.CauseMove, "", map[string]any{"path": "position", "new": "dark-forest-01"}),
		fact("u-loot", "player-A", entity.TypePlayer, outbox.CauseLoot, "",
			map[string]any{"path": "inventory[0]", "new": map[string]any{"item_id": "pelt", "kind": "loot", "name": "Шкура"}}),
		fact("u-hurt", "player-A", entity.TypePlayer, "combat", "", map[string]any{"path": "hp", "old": 10, "new": 2}),
		fact("u-member", "player-B", entity.TypePlayer, outbox.CauseMove, "", map[string]any{"path": "position", "new": "dark-forest-01"}),
		fact("u-group", "group-1", entity.TypeGroup, outbox.CauseMove, "", map[string]any{"path": "position", "new": "dark-forest-01"}),
		fact("u-death", "player-A", entity.TypePlayer, "combat", "", map[string]any{"path": "status", "old": "alive", "new": "dead"}),
	} {
		fd.handle(t, eventbus.TopicSystemEvents, ev)
	}
	var got []string
	for _, r := range fd.rows(t) {
		got = append(got, r.Event+" "+r.Player+" "+r.Kind+" "+r.Text)
	}
	want := []string{
		"u-rest player-A mechanics Вы отдохнули: HP 10/10.",
		"u-move player-A mechanics Вы вошли в Тёмный лес.",
		"u-loot player-A mechanics Трофей: Шкура.",
		"u-group player-B mechanics Группа вошла в Тёмный лес.",
		"u-group player-C mechanics Группа вошла в Тёмный лес.",
		"u-death player-A system Ваш персонаж погиб. /start — создать нового.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("deliveries:\n got %q\nwant %q", got, want)
	}
}

// restarted is a projection loaded from a snapshot of State holding the given
// entities, as the gateway starts after a restart.
func restarted(t *testing.T, f *fixture, entities ...*entity.Entity) *readmodel.Model {
	t.Helper()
	ctx := context.Background()
	const key = "state/20260913T120000Z-000001.json"
	pointer, _ := json.Marshal(map[string]any{"component": "state", "snapshot": map[string]any{
		"key": key, "cursor": map[string]int64{"system_events": 0}, "state_hash": entity.StateHash(entities)}})
	object, _ := json.Marshal(map[string]any{"component": "state", "world_id": world, "entities": entities})
	store := objstore.NewMemory()
	bucket := objstore.SnapshotsBucket(world)
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatal(err)
	}
	for k, body := range map[string][]byte{readmodel.StatePointerKey: pointer, key: object} {
		if _, err := store.Put(ctx, bucket, k, body, objstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	m := readmodel.New(readmodel.Config{Timers: f.clock.Timers()})
	if _, err := m.LoadFromStateSnapshot(ctx, store, world); err != nil {
		t.Fatal(err)
	}
	return m
}

// The opening of an encounter reaches its participants once, even when a
// restart between the two events of the pair makes the second one report the
// opening again: the delivery is idempotent by the encounter and the
// transition in gateway.db (review #1 of T-304, Mi-2).
func TestAnEncounterOpeningIsDeliveredOnceAcrossARestart(t *testing.T) {
	attrs := map[string]any{"region_id": "dark-forest-01", "scope": map[string]any{"id": "player-A", "type": "solo"},
		"state": "active", "round_seq": 0,
		"participants": []any{map[string]any{"player_id": "player-A", "state": "active", "damage_dealt": 0}},
		"npcs":         []any{map[string]any{"npc_id": "wolf-alpha"}}}
	created := birth("enc-1", entity.TypeEncounter, "enc-1", attrs)
	started := mk("started-1", readmodel.TypeEncounterStarted, "enter-A", solo("player-A"), map[string]any{
		"encounter": entityRef("enc-1", entity.TypeEncounter), "region": entityRef("dark-forest-01", entity.TypeRegion),
		"participants": []any{entityRef("player-A", entity.TypePlayer)}, "npcs": []any{entityRef("wolf-alpha", entity.TypeNPC)},
	})
	snapshot := func(t *testing.T) *entity.Entity {
		e := entity.New(entity.Ref{ID: "enc-1", Type: entity.TypeEncounter}, world, "enc-1", attrs, t0)
		e.Version = 1
		raw, _ := json.Marshal(e)
		var decoded entity.Entity
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		return &decoded
	}
	for name, pair := range map[string][2]struct {
		topic string
		ev    eventbus.Event
	}{
		"fact, restart, start": {{eventbus.TopicSystemEvents, created}, {eventbus.TopicWorldEvents, started}},
		"start, restart, fact": {{eventbus.TopicWorldEvents, started}, {eventbus.TopicSystemEvents, created}},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, membus.Chaos{})
			fd := newFeed(t, f, seeded(t, f.model))
			fd.handle(t, pair[0].topic, pair[0].ev)
			if got := fd.rows(t); len(got) != 1 || got[0].Kind != outbox.KindWorldEvent ||
				got[0].Text != "Из тени выходит Волк. Действия: /attack wolf-alpha, /flee" || got[0].Data["encounter_id"] != "enc-1" {
				t.Fatalf("before the restart: %+v", got)
			}
			after := restarted(t, f, snapshot(t))
			if _, err := after.Apply(birth("wolf-alpha", entity.TypeNPC, "Волк", map[string]any{"status": "alive"})); err != nil {
				t.Fatal(err)
			}
			fd.rebuild(t, after)
			res, err := after.Apply(pair[1].ev)
			if err != nil || res.EncounterOpened != "enc-1" {
				t.Fatalf("the second event after the restart reports %+v, %v; the test proves nothing", res, err)
			}
			fd.rebuild(t, restarted(t, f, snapshot(t)))
			fd.handle(t, pair[1].topic, pair[1].ev)
			if got := fd.rows(t); len(got) != 1 {
				t.Errorf("deliveries after the second opening = %d, want 1", len(got))
			}
		})
	}
}

// turnOf writes a session and an accepted turn of an action of a player, as
// the service of actions leaves them.
func (fd *feed) turnOf(t *testing.T, correlation, player, actionType string) {
	t.Helper()
	ctx := context.Background()
	at := t0.Format("2006-01-02T15:04:05.000000000Z")
	if _, err := fd.db.ExecContext(ctx, `INSERT OR IGNORE INTO sessions (id, world_id, scope_id, scope_type, kind, actor_kind,
		participants, started_at, last_action_at, state) VALUES (?, ?, ?, 'solo', 'solo', 'human', '[]', ?, ?, 'active')`,
		"s-"+player, world, player, at, at); err != nil {
		t.Fatal(err)
	}
	deadline := t0.Add(time.Minute).Format("2006-01-02T15:04:05.000000000Z")
	var seq int
	_ = fd.db.QueryRowContext(ctx, `SELECT COUNT(*) + 1 FROM turns`).Scan(&seq)
	if _, err := fd.db.ExecContext(ctx, `INSERT INTO turns (correlation_id, session_id, seq, world_id, scope_id, scope_type,
		player_id, player_name, action_type, status, received_at, acked_at, deadline_at) VALUES (?, ?, ?, ?, ?, 'solo', ?, 'n', ?, 'accepted', ?, ?, ?)`,
		correlation, "s-"+player, seq, world, player, player, actionType, at, at, deadline); err != nil {
		t.Fatal(err)
	}
}

func refusal(id, correlation, proposalID string, ent map[string]any) eventbus.Event {
	p := map[string]any{"proposal_id": proposalID, "reason": "version_conflict",
		"details": map[string]any{"expected_version": 1, "actual_version": 2}}
	if ent != nil {
		p["entity"] = ent
	}
	return mk(id, readmodel.TypeUpdateRejected, correlation, nil, p)
}

// A refusal of State of the move or the rest of the gateway reaches the player
// of the action as kind=system without the reason of State; the same refusal
// again adds nothing, and a refusal of a proposal that is not the gateway's
// reaches nobody (acceptance of T-305).
func TestARefusalOfAMoveOrARestReachesThePlayer(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	fd := newFeed(t, f, seeded(t, f.model))
	fd.turnOf(t, "act-enter", "player-A", "enter")
	fd.turnOf(t, "act-rest", "player-B", "rest")

	enter := refusal("r-1", "act-enter", actions.ProposalID("act-enter", "player-A"), nil)
	fd.handle(t, eventbus.TopicSystemEvents, enter)
	fd.handle(t, eventbus.TopicSystemEvents, refusal("r-2", "act-rest", actions.ProposalID("act-rest", "player-B"), nil))
	fd.handle(t, eventbus.TopicSystemEvents, enter)
	fd.handle(t, eventbus.TopicSystemEvents, refusal("r-3", "act-enter", "encounter-package-1", entityRef("player-A", entity.TypePlayer)))
	fd.handle(t, eventbus.TopicSystemEvents, refusal("r-4", "create-player-Z", "create-player-Z", entityRef("player-Z", entity.TypePlayer)))

	var got []string
	for _, r := range fd.rows(t) {
		got = append(got, r.Event+" "+r.Player+" "+r.Kind+" "+r.Text)
		if strings.Contains(r.Text, "version_conflict") || strings.Contains(r.Text, "version") {
			t.Errorf("the text names the reason of State: %q", r.Text)
		}
	}
	want := []string{
		"r-1 player-A system Переход не удался. Повторите команду.",
		"r-2 player-B system Отдых не удался. Повторите команду.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("deliveries:\n got %q\nwant %q", got, want)
	}

	// Before its turn is recorded, the refusal still finds the player by its
	// entity and the proposal_id of the gateway.
	fd.handle(t, eventbus.TopicSystemEvents, refusal("r-5", "act-early", actions.ProposalID("act-early", "player-A"),
		entityRef("player-A", entity.TypePlayer)))
	if rows := fd.rows(t); len(rows) != 3 || rows[2].Player != "player-A" || rows[2].Kind != outbox.KindSystem {
		t.Errorf("refusal before the turn: %+v", rows)
	}
}

func told(id, correlation, kind, generatedBy string, recipients ...string) eventbus.Event {
	list := make([]any, 0, len(recipients))
	for _, r := range recipients {
		list = append(list, entityRef(r, entity.TypePlayer))
	}
	p := map[string]any{"recipients": list, "text": "Текст " + kind, "generated_by": generatedBy, "kind": kind,
		"locale": "ru", "laws_version": "v1", "narrative_event_id": id,
		"filter": map[string]any{"applied": true, "status": "pass", "filter_version": "1"}}
	if generatedBy == outbox.GeneratedByTemplate {
		p["fallback_reason"] = "timeout"
	}
	ev := mk(id, consumer.TypeNarrativeOutput, correlation, nil, p)
	ev.Source = contracts.SourceSwarm
	return ev
}

// The narrative reaches its recipients in the order the topic delivered it: a
// death that came before the text of its turn is not moved behind it (C-05 v1.4
// p. 8). The data carries narrative_event_id, kind and filter.
func TestTheNarrativeIsDeliveredInTheOrderOfTheTopic(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	fd := newFeed(t, f, seeded(t, f.model))
	fd.handle(t, eventbus.TopicNarrativeOutput, told("n-death", "act-1", "death", outbox.GeneratedByTemplate, "player-A"))
	fd.handle(t, eventbus.TopicNarrativeOutput, told("n-turn", "act-1", "turn", outbox.GeneratedByLLM, "player-A"))
	got := fd.rows(t)
	if len(got) != 2 || got[0].Event != "n-death" || got[1].Event != "n-turn" {
		t.Fatalf("rows = %+v", got)
	}
	d := got[0]
	if d.Kind != outbox.KindNarrative || d.GeneratedBy != outbox.GeneratedByTemplate || d.Data["narrative_event_id"] != "n-death" ||
		d.Data["kind"] != "death" || d.Data["filter"] == nil {
		t.Errorf("delivery of the death = %+v", d)
	}
}

// An acknowledgement counts a delivery of a narrative once: acknowledging the
// same delivery twice with two recipients leaves the turn narrated, and the
// ack of the second recipient publishes one turn.completed (acceptance of
// T-306).
func TestADoubleAckOfOneDeliveryDoesNotCompleteTheTurn(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	fd := newFeed(t, f, seeded(t, f.model))
	fd.turnOf(t, "act-look", "player-A", "look")
	fd.handle(t, eventbus.TopicNarrativeOutput, told("n-1", "act-look", "entry", outbox.GeneratedByLLM, "player-A", "player-B"))

	ctx := context.Background()
	leased, err := fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, f.clock.Now())
	if err != nil || len(leased) != 2 {
		t.Fatalf("lease = %d, %v", len(leased), err)
	}
	onDelivered := func(ctx context.Context, q outbox.DB, d outbox.Delivery, at time.Time) error {
		return fd.tracker.OnDelivered(ctx, q, d.CorrelationID, at)
	}
	idOf := func(player string) string {
		for _, d := range leased {
			if d.PlayerID == player {
				return d.ID
			}
		}
		return ""
	}
	for range 2 {
		if _, _, err := fd.queue.Ack(ctx, "telegram-bot", []string{idOf("player-A")}, f.clock.Now(), onDelivered); err != nil {
			t.Fatal(err)
		}
	}
	row, _, err := turns.Load(ctx, fd.db, "act-look")
	if err != nil || row.Status != turns.StatusNarrated || row.DeliveredCount != 1 || len(fd.analytics.completed()) != 0 {
		t.Fatalf("after a double ack: %+v, %d completed, %v", row, len(fd.analytics.completed()), err)
	}
	if _, _, err := fd.queue.Ack(ctx, "telegram-bot", []string{idOf("player-B")}, f.clock.Now(), onDelivered); err != nil {
		t.Fatal(err)
	}
	if n := len(fd.analytics.completed()); n != 1 {
		t.Errorf("turn.completed after the last recipient = %d, want 1", n)
	}
}

// The effects fail loudly without what they need.
func TestDeliveriesNeedTheirParts(t *testing.T) {
	if _, err := (&consumer.Deliveries{}).Effects(); err == nil {
		t.Error("Effects without parts succeeded")
	}
	merged := consumer.MergeEffects(
		map[string][]consumer.Effect{"a": {nil}},
		map[string][]consumer.Effect{"a": {nil}, "b": {nil}})
	if len(merged["a"]) != 2 || len(merged["b"]) != 1 {
		t.Errorf("merged = %v", merged)
	}
}

// ack leases the deliveries of the bot and acknowledges those of player, with
// the turns moved on as the handler of acknowledgements does.
func (fd *feed) ack(t *testing.T, player string) {
	t.Helper()
	ctx := context.Background()
	leased, err := fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, fd.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, d := range leased {
		if d.PlayerID == player {
			ids = append(ids, d.ID)
		}
	}
	onDelivered := func(ctx context.Context, q outbox.DB, d outbox.Delivery, at time.Time) error {
		return fd.tracker.OnDelivered(ctx, q, d.CorrelationID, at)
	}
	if _, _, err := fd.queue.Ack(ctx, "telegram-bot", ids, fd.clock.Now(), onDelivered); err != nil {
		t.Fatal(err)
	}
}

func (fd *feed) failedTurns(t *testing.T, player string) int {
	t.Helper()
	var n int
	if err := fd.db.QueryRowContext(context.Background(), `SELECT turns_failed FROM sessions WHERE id = ?`, "s-"+player).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A turn counts as its recipients the deliveries an acknowledgement can
// complete — each linked player once — so that a recipient without a link, or
// one named twice, does not leave it waiting for its deadline (review #1 of
// T-307, Ma-1; decision 3 of the orchestrator).
func TestATurnCountsOnlyTheRecipientsItCanReach(t *testing.T) {
	cases := []struct {
		name       string
		recipients []string
		ack        string
		pending    int
		counted    int
		atOnce     bool
	}{
		{"a recipient without a link", []string{"player-A", "player-C"}, "player-A", 1, 1, false},
		{"a repeated recipient", []string{"player-A", "player-A"}, "player-A", 1, 1, false},
		{"no recipient with a link", []string{"player-C", "player-D"}, "", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, membus.Chaos{})
			fd := newFeed(t, f, seeded(t, f.model))
			fd.turnOf(t, "act-look", "player-A", "look")
			fd.handle(t, eventbus.TopicNarrativeOutput, told("n-1", "act-look", "entry", outbox.GeneratedByLLM, tc.recipients...))

			pending := 0
			for _, r := range fd.rows(t) {
				if r.State == outbox.StatePending {
					pending++
				}
			}
			row, _, err := turns.Load(context.Background(), fd.db, "act-look")
			if err != nil || pending != tc.pending || row.RecipientsCount != tc.counted {
				t.Fatalf("pending %d, recipients_count %d, %v; want %d and %d", pending, row.RecipientsCount, err, tc.pending, tc.counted)
			}
			if got := len(fd.analytics.completed()); tc.atOnce != (got == 1) {
				t.Fatalf("turn.completed before any ack = %d, at once %v", got, tc.atOnce)
			}
			if tc.ack != "" {
				fd.ack(t, tc.ack)
			}
			got := fd.analytics.completed()
			if len(got) != 1 {
				t.Fatalf("turn.completed = %d, want 1", len(got))
			}
			if status, _ := got[0].Path().GetString("turn.status"); status != "ok" {
				t.Errorf("status = %s, want ok", status)
			}
			f.clock.Advance(2 * time.Minute)
			if _, err := fd.tracker.Sweep(context.Background(), f.clock.Now()); err != nil {
				t.Fatal(err)
			}
			if n := len(fd.analytics.completed()); n != 1 || fd.failedTurns(t, "player-A") != 0 {
				t.Errorf("after the deadline: %d turn.completed, turns_failed %d; want 1 and 0", n, fd.failedTurns(t, "player-A"))
			}
		})
	}
}

// A narrative delivery carries the absence summary in its data, the number of
// the round and, for a template, the reason the template replaced the model,
// in its row and in the answer of the long-poll (C-05, api-contracts.md §1.5;
// review #1 of T-307, Mi-2).
func TestANarrativeDeliveryCarriesAbsenceRoundAndFallback(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	fd := newFeed(t, f, seeded(t, f.model))
	ev := told("n-round", "act-1", "round", outbox.GeneratedByTemplate, "player-A")
	ev.Payload["absence"] = map[string]any{"since_at": "2026-09-13T10:00:00Z", "background_events_count": 4.0}
	ev.Payload["round"] = map[string]any{"seq": 3.0}
	fd.handle(t, eventbus.TopicNarrativeOutput, ev)

	rows := fd.rows(t)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	r := rows[0]
	absence, _ := r.Data["absence"].(map[string]any)
	if absence["since_at"] != "2026-09-13T10:00:00Z" || absence["background_events_count"] != 4.0 ||
		r.RoundSeq.Int64 != 3 || r.FallbackReason.String != "timeout" {
		t.Fatalf("row = %+v", r)
	}

	svc, err := outbox.NewService(outbox.ServiceConfig{Store: fd.queue, Routes: fd.links, Clock: f.clock, Timers: f.clock.Timers()})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := svc.Serve(context.Background(), outbox.Poll{ClientID: "telegram-bot", Platform: links.PlatformTelegram, Limit: 10})
	if err != nil || len(resp.Deliveries) != 1 {
		t.Fatalf("Serve = %+v, %v", resp, err)
	}
	d := resp.Deliveries[0]
	wire, _ := d.Data["absence"].(map[string]any)
	if d.RoundSeq == nil || *d.RoundSeq != 3 || d.FallbackReason == nil || *d.FallbackReason != "timeout" ||
		d.GeneratedBy != outbox.GeneratedByTemplate || wire["background_events_count"] != 4.0 ||
		d.NarrativeEventID == nil || *d.NarrativeEventID != "n-round" {
		t.Errorf("delivery = %+v", d)
	}
}
