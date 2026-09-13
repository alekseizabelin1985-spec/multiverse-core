package characters_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/characters"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

const (
	world  = "dark-forest-world"
	forest = "dark-forest-01"
	// externalID is shaped like a Telegram user id; it is not a real account.
	externalID = "7391846205"
	otherID    = "5550001111"
)

var t0 = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// State is how the State of the tests answers a proposal of a character.
type State int

const (
	// Creates applies the proposal: entity.created goes into the projection
	// right after the publication.
	Creates State = iota
	// Silent never answers.
	Silent
	// Refuses answers entity.update.rejected.
	Refuses
	// Down fails the publication.
	Down
)

// bus is membus with a State in front of it.
type bus struct {
	*membus.Bus
	f     *fixture
	mu    sync.Mutex
	state State
}

func (b *bus) set(s State) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = s
}

func (b *bus) Publish(ctx context.Context, ev eventbus.Event) error {
	b.mu.Lock()
	state := b.state
	b.mu.Unlock()
	if state == Down {
		return errors.New("broker down")
	}
	if err := b.Bus.Publish(ctx, ev); err != nil {
		return err
	}
	if ev.Type != characters.TypeCreateProposed {
		return nil
	}
	switch state {
	case Creates:
		b.f.create(ev)
	case Refuses:
		pa := ev.Path()
		proposalID, _ := pa.GetString("proposal_id")
		rejected := eventbus.Derive(ev, readmodel.TypeUpdateRejected, contracts.SourceState, map[string]any{
			"proposal_id": proposalID, "reason": "duplicate_entity",
		}, eventbus.WithCauseID("rejected"))
		if _, err := b.f.model.Apply(rejected); err != nil {
			b.f.t.Error(err)
		}
	}
	return nil
}

// signal is the timers of the projection, telling the test when a wait of
// Wait is armed.
type signal struct {
	*clock.ManualTimers
	wait  time.Duration
	armed chan struct{}
}

func (s signal) After(d time.Duration) clock.Timer {
	timer := s.ManualTimers.After(d)
	if d == s.wait {
		select {
		case s.armed <- struct{}{}:
		default:
		}
	}
	return timer
}

// replacing is an input filter of the operator: it blocks, fails or replaces
// the whole text.
type replacing struct {
	mu      sync.Mutex
	status  string
	replace string
	fail    bool
}

func (r *replacing) set(status, replace string, fail bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status, r.replace, r.fail = status, replace, fail
}

func (r *replacing) Check(_ context.Context, _, text string) (actions.FilterResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.fail:
		return actions.FilterResult{}, errors.New("filter down")
	case r.status == actions.FilterBlocked:
		return actions.FilterResult{Status: actions.FilterBlocked}, nil
	case r.replace != "":
		return actions.FilterResult{Status: actions.FilterReplaced, Text: r.replace}, nil
	}
	return actions.FilterResult{Status: actions.FilterPass, Text: text}, nil
}

// hookedLinks is the links store with a hook in front of DetachPlayer, which
// also counts its calls.
type hookedLinks struct {
	*links.SQLite
	mu       sync.Mutex
	detaches int
	before   func(playerID string)
}

func (h *hookedLinks) setBeforeDetach(f func(playerID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.before = f
}

func (h *hookedLinks) detachCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.detaches
}

func (h *hookedLinks) DetachPlayer(ctx context.Context, playerID string) (bool, error) {
	h.mu.Lock()
	h.detaches++
	before := h.before
	h.mu.Unlock()
	if before != nil {
		before(playerID)
	}
	return h.SQLite.DetachPlayer(ctx, playerID)
}

type noTurns struct{}

func (noTurns) Begin(context.Context, actions.Turn) (api.TurnRef, error) { return api.TurnRef{}, nil }
func (noTurns) Accepted(context.Context, actions.Turn, api.TurnRef, string) error {
	return nil
}
func (noTurns) Rejected(context.Context, actions.Turn, string) error { return nil }

type fixture struct {
	t         *testing.T
	linksDB   *sql.DB
	gatewayDB *sql.DB
	links     *links.SQLite
	hooks     *hookedLinks
	model     *readmodel.Model
	clock     *clock.Manual
	armed     chan struct{}
	bus       *bus
	filter    *replacing
	sessions  *session.Manager
	svc       *characters.Service
	ids       links.IDSource
	version   map[string]int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	eventbus.SetIDSource(eventbus.SequenceIDs("ev"))
	t.Cleanup(func() { eventbus.SetIDSource(nil) })
	ctx := context.Background()
	dir := sqlitedir.Temp(t)
	f := &fixture{t: t, clock: clock.NewManual(t0), armed: make(chan struct{}, 8), filter: &replacing{}, version: map[string]int64{}}
	var err error
	if f.linksDB, err = store.OpenLinks(ctx, store.LinksPath(dir)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.linksDB.Close() })
	if _, err := store.MigrateLinks(ctx, f.linksDB); err != nil {
		t.Fatal(err)
	}
	if f.gatewayDB, err = store.OpenGateway(ctx, store.GatewayPath(dir)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.gatewayDB.Close() })
	if _, err := store.MigrateGateway(ctx, f.gatewayDB); err != nil {
		t.Fatal(err)
	}
	n := 0
	if f.links, err = links.NewSQLite(f.linksDB, func() string { n++; return "id-" + strconv.Itoa(n) }); err != nil {
		t.Fatal(err)
	}
	f.hooks = &hookedLinks{SQLite: f.links}
	f.model = readmodel.New(readmodel.Config{Timers: signal{ManualTimers: f.clock.Timers(), wait: characters.DefaultWait, armed: f.armed}})
	f.apply(created(world, entity.TypeWorld, "Тёмный лес", "create-world", map[string]any{"laws_version": "1", "locale": "ru"}))
	f.apply(created(forest, entity.TypeRegion, "Опушка", "create-forest", map[string]any{"description": "лес"}))

	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	raw, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	f.bus = &bus{Bus: raw, f: f}
	if f.sessions, err = session.New(session.Config{DB: f.gatewayDB}); err != nil {
		t.Fatal(err)
	}
	filter, err := actions.New(actions.Config{Bus: f.bus, Model: f.model, Keys: actions.NewKeys(f.gatewayDB, store.KeyTTL),
		Filter: f.filter, FilterName: "test", Turns: noTurns{}, Clock: f.clock, Timers: f.clock.Timers(),
		GMPath: eventbus.GMPathAgent, KeyTTL: store.KeyTTL})
	if err != nil {
		t.Fatal(err)
	}
	f.ids = func() string { n++; return "id-" + strconv.Itoa(n) }
	f.svc = f.service(filter)
	return f
}

// service is a service of characters over the fixture with the name filter
// given.
func (f *fixture) service(filter characters.NameFilter) *characters.Service {
	f.t.Helper()
	svc, err := characters.New(characters.Config{Links: f.hooks, Model: f.model, Sessions: f.sessions, DB: f.gatewayDB,
		Bus: f.bus, Filter: filter, IDs: f.ids, Clock: f.clock,
		Wait: characters.DefaultWait, Deadline: characters.DefaultDeadline})
	if err != nil {
		f.t.Fatal(err)
	}
	return svc
}

func created(id, typ, name, proposalID string, attrs map[string]any) eventbus.Event {
	raw, _ := json.Marshal(map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": id, "type": typ}, "name": name},
		"version":     1,
		"attributes":  attrs,
		"proposal_id": proposalID,
	})
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	return eventbus.Event{
		ID: "created-" + id, Type: readmodel.TypeEntityCreated, Timestamp: t0, Source: contracts.SourceState,
		World: &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: world, Type: entity.TypeWorld}},
		Meta: eventbus.Meta{SchemaVersion: 1, CorrelationID: proposalID, ActorKind: eventbus.ActorSystem,
			Locale: "ru", GMPath: eventbus.GMPathAgent},
		Payload: payload,
	}
}

func (f *fixture) apply(ev eventbus.Event) {
	f.t.Helper()
	if _, err := f.model.Apply(ev); err != nil {
		f.t.Fatal(err)
	}
}

// create is State applying a proposal of a character: the fact is derived
// from the proposal, as State derives it.
func (f *fixture) create(proposal eventbus.Event) {
	pa := proposal.Path()
	id, _ := pa.GetString("entity.entity.id")
	name, _ := pa.GetString("entity.name")
	proposalID, _ := pa.GetString("proposal_id")
	attrs, _ := proposal.Payload["attributes"].(map[string]any)
	raw, _ := json.Marshal(map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}, "name": name},
		"version":     1,
		"attributes":  attrs,
		"proposal_id": proposalID,
	})
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	f.apply(eventbus.Derive(proposal, readmodel.TypeEntityCreated, contracts.SourceState, payload, eventbus.WithCauseID(id)))
	f.version[id] = 1
}

// die is the fact of the death of a character.
func (f *fixture) die(id string) {
	f.version[id]++
	f.apply(eventbus.Event{
		ID: "died-" + id, Type: readmodel.TypeEntityUpdated, Timestamp: t0, Source: contracts.SourceState,
		World: &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: world, Type: entity.TypeWorld}},
		Meta:  eventbus.Meta{SchemaVersion: 1, CorrelationID: "died-" + id, ActorKind: eventbus.ActorSystem, Locale: "ru", GMPath: "agent"},
		Payload: map[string]any{
			"entity":  map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}},
			"version": float64(f.version[id]), "cause": "death", "proposal_id": "death-" + id,
			"changed": []any{map[string]any{"path": "status", "old": "alive", "new": "dead"}},
		},
	})
}

// consent gives the account a consented link.
func (f *fixture) consent(id string) links.Link {
	f.t.Helper()
	ctx := context.Background()
	if _, err := f.links.Resolve(ctx, links.PlatformTelegram, id, t0); err != nil {
		f.t.Fatal(err)
	}
	l, err := f.links.Consent(ctx, links.PlatformTelegram, id,
		links.ConsentForm{NoticeShown: true, Consent: true, AgeConfirmed: true, ShownAt: t0}, t0)
	if err != nil {
		f.t.Fatal(err)
	}
	return l
}

func (f *fixture) link(id string) links.Link {
	f.t.Helper()
	l, found, err := f.links.ByExternal(context.Background(), links.PlatformTelegram, id)
	if err != nil || !found {
		f.t.Fatalf("link: %v %v", found, err)
	}
	return l
}

func request(id, key, name string) characters.Request {
	return characters.Request{Platform: links.PlatformTelegram, ExternalID: id, WorldID: world, Name: name,
		ActionKey: key, ActorKind: eventbus.ActorHuman}
}

// createAsync runs Create in a goroutine and returns its answer channel.
func (f *fixture) createAsync(r characters.Request) <-chan characters.Answer {
	out := make(chan characters.Answer, 1)
	go func() { out <- f.svc.Create(context.Background(), r) }()
	return out
}

// createPastTheWait runs Create against a silent State: once the wait for the
// fact is armed the clock moves past it.
func (f *fixture) createPastTheWait(r characters.Request) characters.Answer {
	f.t.Helper()
	for len(f.armed) > 0 {
		<-f.armed
	}
	answer := f.createAsync(r)
	select {
	case <-f.armed:
	case a := <-answer:
		f.t.Fatalf("Create answered before it waited: %+v", a)
	}
	f.clock.Advance(characters.DefaultWait)
	return <-answer
}

// proposals are the proposals of characters published so far.
func (f *fixture) proposals() []eventbus.Event {
	f.t.Helper()
	records, err := f.bus.Records(eventbus.TopicSystemEvents)
	if err != nil {
		f.t.Fatal(err)
	}
	var out []eventbus.Event
	for _, raw := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			f.t.Fatal(err)
		}
		if ev.Type == characters.TypeCreateProposed {
			out = append(out, ev)
		}
	}
	return out
}

func (f *fixture) pendingRows() int {
	f.t.Helper()
	var n int
	if err := f.gatewayDB.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM pending_characters`).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func code(a characters.Answer) string {
	if a.Err == nil {
		return ""
	}
	return a.Err.Code
}
