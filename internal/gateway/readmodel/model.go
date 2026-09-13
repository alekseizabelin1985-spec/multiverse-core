// Package readmodel is the projection of the world the gateway validates
// actions against and answers GET requests from (component gateway-and-bot.md
// §2.1, §6). It holds the entities of State as the facts describe them —
// entity.created and entity.updated applied in the order of their versions —
// and the life cycle of the encounters announced on world_events.
//
// The projection lives in memory only (component §16 p. 2): at start it is
// loaded from snapshots-{world}/state/latest.json and caught up from the
// journal, and it is never the truth for anything the gateway writes.
//
// Every read goes through the typed getters of shared/entity; the projection
// never indexes the attribute maps itself (design §8, risk of shared/entity v2
// changing under it).
package readmodel

import (
	"cmp"
	"log/slog"
	"slices"
	"sync"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// The event types the projection reads (C-02, C-05).
const (
	TypeEntityCreated    = "entity.created"
	TypeEntityUpdated    = "entity.updated"
	TypeUpdateRejected   = "entity.update.rejected"
	TypeEncounterStarted = "encounter.started"
	TypeEncounterEnded   = "encounter.ended"
)

// Projection is the state of the projection reported by /health as
// projection (component §11.4).
type Projection string

const (
	// ProjectionOK: loaded from the snapshot of State and kept up by the facts.
	ProjectionOK Projection = "ok"
	// ProjectionMissing: there was no snapshot to load, or it could not be
	// read; the projection holds only what the journal told it.
	ProjectionMissing Projection = "missing"
	// ProjectionStale: a fact arrived whose version does not follow the one
	// the projection holds, so at least one entity is known to be behind State.
	ProjectionStale Projection = "stale"
)

// EncounterAnnounced is the state of an encounter whose encounter.started has
// arrived and whose entity.created has not. It is always temporary: the start
// is published only after the fact (C-05 v1.4 p. 4), and only the order in
// which two topics are read puts it first (C-04 v1.3).
const EncounterAnnounced = "announced"

// Config builds a model.
type Config struct {
	// Timers arms the deadlines of AwaitFact. Required.
	Timers clock.Timers
	// Log receives the anomalies of the stream; nil discards them.
	Log *slog.Logger
}

// Model is the projection. It is safe for concurrent use: the consumer applies
// events from one goroutine per topic while HTTP handlers read.
type Model struct {
	timers clock.Timers
	log    *slog.Logger

	mu         sync.Mutex
	entities   map[string]*entity.Entity
	encounters map[string]*lifecycle
	cursor     map[string]int64
	loaded     bool
	loadErr    string // a Reason code
	stale      map[string]struct{}
	waiters    map[string][]*Waiter
	// open indexes the open encounters by participant, and listed remembers
	// under which participants each encounter is indexed, so that EncounterOf
	// looks at the encounters of one player rather than at every encounter
	// the process has seen.
	open   map[string]map[string]struct{}
	listed map[string][]string
}

// New returns an empty projection: missing until a snapshot is loaded.
func New(cfg Config) *Model {
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Model{
		timers:     cfg.Timers,
		log:        log,
		entities:   make(map[string]*entity.Entity),
		encounters: make(map[string]*lifecycle),
		cursor:     make(map[string]int64),
		stale:      make(map[string]struct{}),
		waiters:    make(map[string][]*Waiter),
		open:       make(map[string]map[string]struct{}),
		listed:     make(map[string][]string),
	}
}

// World is the projection of a world entity (data-model.md §3.1).
type World struct {
	ID          string
	Name        string
	Version     int64
	LawsVersion string
	Locale      string
}

// Region is the projection of a region (data-model.md §3.2).
type Region struct {
	ID             string
	WorldID        string
	Name           string
	Version        int64
	Description    string
	NPCIDs         []string
	PlayersPresent []string
}

// NPC is the projection of a non-player character (data-model.md §3.4).
type NPC struct {
	ID       string
	WorldID  string
	Name     string
	Version  int64
	Kind     string
	RegionID string
	Position string
	Status   string
	HP       int
	HPMax    int
}

// CharacterState is the projection of a player character (data-model.md §3.3).
type CharacterState struct {
	ID          string
	WorldID     string
	Name        string
	Version     int64
	HP          int
	HPMax       int
	Status      string
	Position    string
	Scope       eventbus.ScopeRef
	GroupID     string
	EncounterID string
	ActorKind   string
	Inventory   []entity.Item
}

// Group is the projection of a group (data-model.md §3.6). LeaderID is nil
// for a group that has no living leader (BR-13 v0.4).
type Group struct {
	ID       string
	WorldID  string
	Name     string
	Version  int64
	LeaderID *string
	Members  []entity.Member
	State    string
	Position string
	Scope    eventbus.ScopeRef
}

// RoundParams are the round parameters an encounter.started carries for a
// group encounter (C-05 v1.1).
type RoundParams struct {
	Timeout         string
	IdleAfterMissed int
}

// Encounter is an encounter as the gateway sees it: the entity of State when
// its fact has arrived, merged with what encounter.started and encounter.ended
// said. It is computed on every read, so the order in which the two topics
// delivered the events cannot show in it.
type Encounter struct {
	ID      string
	WorldID string
	// Version is the version of the entity; 0 while only the start is known.
	Version int64
	// State is announced, active or resolved.
	State        string
	RegionID     string
	Scope        eventbus.ScopeRef
	Participants []string
	NPCIDs       []string
	Resolution   string
	RoundSeq     int
	Round        *RoundParams
}

// Open says whether the encounter still takes actions: announced or active.
func (e Encounter) Open() bool { return e.State != entity.EncounterStateResolved }

// Character returns a player character.
func (m *Model) Character(id string) (CharacterState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entityOf(id, entity.TypePlayer)
	if !ok {
		return CharacterState{}, false
	}
	c := CharacterState{ID: e.ID, WorldID: e.WorldID, Name: e.Name, Version: e.Version}
	c.HP, _ = e.HP()
	c.HPMax, _ = e.HPMax()
	c.Status, _ = e.Status()
	c.Position, _ = e.Position()
	c.Scope, _ = e.Scope()
	c.GroupID, _ = e.GroupID()
	c.EncounterID, _ = e.EncounterID()
	c.ActorKind, _ = e.ActorKind()
	items, err := e.Inventory()
	if err != nil {
		m.log.Warn("readmodel: inventory does not decode", slog.String("entity_id", e.ID), slog.String("error", err.Error()))
	}
	c.Inventory = items
	return c, true
}

// NPC returns a non-player character.
func (m *Model) NPC(id string) (NPC, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entityOf(id, entity.TypeNPC)
	if !ok {
		return NPC{}, false
	}
	n := NPC{ID: e.ID, WorldID: e.WorldID, Name: e.Name, Version: e.Version}
	n.Kind, _ = e.Kind()
	n.RegionID, _ = e.RegionID()
	n.Position, _ = e.Position()
	n.Status, _ = e.Status()
	n.HP, _ = e.HP()
	n.HPMax, _ = e.HPMax()
	return n, true
}

// Region returns a region of a world.
func (m *Model) Region(worldID, regionID string) (Region, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entityOf(regionID, entity.TypeRegion)
	if !ok || e.WorldID != worldID {
		return Region{}, false
	}
	r := Region{ID: e.ID, WorldID: e.WorldID, Name: e.Name, Version: e.Version}
	r.Description, _ = e.Description()
	r.NPCIDs, _ = e.NPCIDs()
	r.PlayersPresent, _ = e.PlayersPresent()
	return r, true
}

// Worlds returns every world the projection knows, by id.
func (m *Model) Worlds() []World {
	m.mu.Lock()
	defer m.mu.Unlock()
	var worlds []World
	for _, e := range m.entities {
		if e.Type != entity.TypeWorld {
			continue
		}
		w := World{ID: e.ID, Name: e.Name, Version: e.Version}
		w.LawsVersion, _ = e.LawsVersion()
		w.Locale, _ = e.Locale()
		worlds = append(worlds, w)
	}
	slices.SortFunc(worlds, func(a, b World) int { return cmp.Compare(a.ID, b.ID) })
	return worlds
}

// Group returns a group.
func (m *Model) Group(id string) (Group, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entityOf(id, entity.TypeGroup)
	if !ok {
		return Group{}, false
	}
	g := Group{ID: e.ID, WorldID: e.WorldID, Name: e.Name, Version: e.Version}
	if leader, ok := e.LeaderID(); ok {
		g.LeaderID = &leader
	}
	members, err := e.Members()
	if err != nil {
		m.log.Warn("readmodel: members do not decode", slog.String("entity_id", e.ID), slog.String("error", err.Error()))
	}
	g.Members = members
	g.State, _ = e.State()
	g.Position, _ = e.Position()
	g.Scope, _ = e.Scope()
	return g, true
}

// Encounter returns an encounter known from its entity, from its start, or
// from both.
func (m *Model) Encounter(id string) (Encounter, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.encounterOf(id)
}

// EncounterOf returns the open encounter a character takes part in. An
// encounter that ended by either of its two ends is not returned, which is
// what makes an action after the end fail its precondition (C-05 p. 3,
// not_in_encounter).
func (m *Model) EncounterOf(playerID string) (Encounter, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.open[playerID]))
	for id := range m.open[playerID] {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		enc, ok := m.encounterOf(id)
		if ok && enc.Open() && slices.Contains(enc.Participants, playerID) {
			return enc, true
		}
	}
	return Encounter{}, false
}

// reindex puts an encounter under its participants while it is open and takes
// it out of the index once it is closed. It must be called with m.mu held,
// after every change of the entity or the life cycle of the encounter.
func (m *Model) reindex(id string) {
	for _, player := range m.listed[id] {
		delete(m.open[player], id)
		if len(m.open[player]) == 0 {
			delete(m.open, player)
		}
	}
	delete(m.listed, id)
	enc, ok := m.encounterOf(id)
	if !ok || !enc.Open() {
		return
	}
	for _, player := range enc.Participants {
		if m.open[player] == nil {
			m.open[player] = make(map[string]struct{})
		}
		m.open[player][id] = struct{}{}
	}
	m.listed[id] = slices.Clone(enc.Participants)
}

// Version returns the version of an entity the projection holds.
func (m *Model) Version(id string) (int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entities[id]
	if !ok {
		return 0, false
	}
	return e.Version, true
}

// Hash is the state hash of the entities of the projection, in the form
// State writes into its snapshot (entity.StateHash): a projection that
// applied the facts right hashes the same as the world State holds.
func (m *Model) Hash() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := make([]*entity.Entity, 0, len(m.entities))
	for _, e := range m.entities {
		list = append(list, e)
	}
	return entity.StateHash(list)
}

// Cursor returns, per topic, the offset of the next event the projection has
// not applied: the snapshot of State sets it, every applied event moves it.
func (m *Model) Cursor() map[string]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int64, len(m.cursor))
	for topic, offset := range m.cursor {
		out[topic] = offset
	}
	return out
}

// Advance records that the event at pos is applied.
func (m *Model) Advance(pos eventbus.Position) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if next := pos.Offset + 1; next > m.cursor[pos.Topic] {
		m.cursor[pos.Topic] = next
	}
}

// Status is the projection state for /health, and the reason the snapshot
// did not load when there is one: one of the Reason codes, never the text of
// the error.
func (m *Model) Status() (Projection, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case len(m.stale) > 0:
		return ProjectionStale, m.loadErr
	case m.loaded:
		return ProjectionOK, ""
	default:
		return ProjectionMissing, m.loadErr
	}
}

// entityOf must be called with m.mu held.
func (m *Model) entityOf(id, typ string) (*entity.Entity, bool) {
	e, ok := m.entities[id]
	if !ok || e.Type != typ {
		return nil, false
	}
	return e, true
}
