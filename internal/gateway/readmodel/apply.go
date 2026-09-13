package readmodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// ErrCorruptFact is a fact the projection cannot apply without guessing: an
// entity without an identity, or an element of a list past its end, which
// State never writes (C-02 v1.6, rule of catching up).
var ErrCorruptFact = errors.New("readmodel: corrupt fact")

// Result is what an event meant beyond the projection itself, for the side
// effects of the consumer.
//
// EncounterOpened and EncounterEnded name the encounter this very event opened
// or ended. Each encounter has two events that can open it (encounter.started,
// entity.created) and two that can end it (the fact with state=resolved,
// encounter.ended); whichever the projection sees first claims the transition,
// and the second one of the pair reports nothing (C-04 v1.3). The claim is by
// event id, so a retry of the claiming event reports the transition again: an
// effect that failed is not lost with the retry.
//
// A transition is unique within the process only. The claim lives in memory:
// after a restart the projection comes from the snapshot of State, which does
// not say which event claimed what, and the second event of a pair that
// arrives then reports the transition once more. An effect of a transition
// must therefore be idempotent by the encounter in gateway.db itself (T-307,
// T-351).
type Result struct {
	// Applied says whether the projection changed.
	Applied         bool
	EncounterOpened string
	EncounterEnded  string
}

// lifecycle is what world_events said about an encounter.
type lifecycle struct {
	started   *startedPayload
	world     string
	scope     *eventbus.ScopeRef
	ended     bool
	endReason string
	openedBy  string
	closedBy  string
}

type startedPayload struct {
	Encounter    eventbus.Entity   `json:"encounter"`
	Region       eventbus.Entity   `json:"region"`
	Participants []eventbus.Entity `json:"participants"`
	NPCs         []eventbus.Entity `json:"npcs"`
	Round        *struct {
		Timeout         string `json:"timeout"`
		IdleAfterMissed int    `json:"idle_after_missed"`
	} `json:"round"`
}

type endedPayload struct {
	Encounter eventbus.Entity `json:"encounter"`
	Reason    string          `json:"reason"`
}

type createdPayload struct {
	Entity     eventbus.Entity `json:"entity"`
	Version    int64           `json:"version"`
	Attributes map[string]any  `json:"attributes"`
}

type updatedPayload struct {
	Entity    eventbus.Entity `json:"entity"`
	Version   int64           `json:"version"`
	Changed   []changedPath   `json:"changed"`
	AppliedAt time.Time       `json:"applied_at"`
}

type rejectedPayload struct {
	ProposalID string `json:"proposal_id"`
}

// changedPath is one element of entity.updated.changed[]. Only the presence of
// new matters for applying it: C-02 v1.5 always writes new (null for a removed
// key) and old: null for an append, C-02 v1.6 writes new only when the path
// exists after the change and old only when it existed before. Reading "new
// present → write it, absent → delete the path" applies both forms.
//
// Under v1.5 the removal of a key and a set to null travel the same way and
// cannot be told apart, so the projection keeps the key as null. Every getter
// reads that null as absent, but State deleted the key, and the canonical
// JSON of the two differs: after such a fact Hash no longer matches the hash
// of State. The hash of the projection checks against State only on facts of
// v1.6 (T-448).
type changedPath struct {
	Path string   `json:"path"`
	New  presence `json:"new"`
}

// presence records whether a JSON key was there at all. encoding/json calls
// UnmarshalJSON for a present key, null included, and never for an absent one.
type presence struct {
	set   bool
	value any
}

func (p *presence) UnmarshalJSON(b []byte) error {
	p.set = true
	return json.Unmarshal(b, &p.value)
}

// Apply folds one event into the projection. Events of other types are
// ignored. Applying an event again is harmless: a fact whose version the
// projection already holds changes nothing, and the life cycle of an
// encounter only moves forward.
func (m *Model) Apply(ev eventbus.Event) (Result, error) {
	switch ev.Type {
	case TypeEntityCreated:
		var p createdPayload
		if err := decode(ev, &p); err != nil {
			return Result{}, err
		}
		return m.applyCreated(ev, p)
	case TypeEntityUpdated:
		var p updatedPayload
		if err := decode(ev, &p); err != nil {
			return Result{}, err
		}
		return m.applyUpdated(ev, p)
	case TypeUpdateRejected:
		var p rejectedPayload
		if err := decode(ev, &p); err != nil {
			return Result{}, err
		}
		m.mu.Lock()
		m.notify(proposalKey(p.ProposalID), ev, ErrRejected)
		m.mu.Unlock()
		return Result{}, nil
	case TypeEncounterStarted:
		var p startedPayload
		if err := decode(ev, &p); err != nil {
			return Result{}, err
		}
		return m.applyStarted(ev, p)
	case TypeEncounterEnded:
		var p endedPayload
		if err := decode(ev, &p); err != nil {
			return Result{}, err
		}
		return m.applyEnded(ev, p)
	default:
		return Result{}, nil
	}
}

func (m *Model) applyCreated(ev eventbus.Event, p createdPayload) (Result, error) {
	ref := entity.RefFrom(p.Entity.Entity)
	if ref.ID == "" || ref.Type == "" {
		return Result{}, fmt.Errorf("%w: %s %s without an entity", ErrCorruptFact, ev.Type, ev.ID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var res Result
	if _, known := m.entities[ref.ID]; !known {
		e := entity.New(ref, worldOf(ev), p.Entity.Name, p.Attributes, ev.Timestamp)
		e.LastEventID = ev.ID
		m.entities[ref.ID] = e
		res.Applied = true
	}
	if ref.Type == entity.TypeEncounter {
		res.EncounterOpened = m.claimOpen(ref.ID, ev.ID)
		m.reindex(ref.ID)
	}
	m.notify(factKey(ev.CorrelationID()), ev, nil)
	return res, nil
}

func (m *Model) applyUpdated(ev eventbus.Event, p updatedPayload) (Result, error) {
	ref := entity.RefFrom(p.Entity.Entity)
	if ref.ID == "" || ref.Type == "" {
		return Result{}, fmt.Errorf("%w: %s %s without an entity", ErrCorruptFact, ev.Type, ev.ID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var res Result
	// A fact at a version the projection holds is already in it — a repeat of
	// the bus, the catch-up after the snapshot, or a fact without changes,
	// which keeps the version State had (C-02).
	cur, known := m.entities[ref.ID]
	if !known || p.Version > cur.Version {
		next, err := m.applyChanges(ev, p, cur)
		if err != nil {
			return Result{}, err
		}
		if !known || p.Version != cur.Version+1 {
			m.markStale(ref, ev, cur)
		}
		m.entities[ref.ID] = next
		res.Applied = true
	}
	if ref.Type == entity.TypeEncounter {
		if resolves(p.Changed) {
			res.EncounterEnded = m.claimEnd(ref.ID, ev.ID)
		}
		m.reindex(ref.ID)
	}
	m.notify(factKey(ev.CorrelationID()), ev, nil)
	return res, nil
}

// markStale records an entity the projection knows to be behind State: its
// base was missing, or facts between the two versions never arrived. The
// changes of the fact are applied anyway — they carry the values State wrote
// for the paths they name — but the other paths may be old.
func (m *Model) markStale(ref entity.Ref, ev eventbus.Event, cur *entity.Entity) {
	var had int64
	if cur != nil {
		had = cur.Version
	}
	m.stale[ref.ID] = struct{}{}
	m.log.Warn("readmodel: fact does not follow the projection",
		slog.String("entity_id", ref.ID), slog.String("event_id", ev.ID),
		slog.Int64("projection_version", had))
}

func (m *Model) applyChanges(ev eventbus.Event, p updatedPayload, cur *entity.Entity) (*entity.Entity, error) {
	var next *entity.Entity
	if cur != nil {
		next = entity.Clone(cur)
	} else {
		next = entity.New(entity.RefFrom(p.Entity.Entity), worldOf(ev), p.Entity.Name, nil, p.AppliedAt)
	}
	for _, c := range p.Changed {
		op, ok, err := opFor(next, c)
		if err != nil {
			return nil, fmt.Errorf("%w: %s %s path %q: %v", ErrCorruptFact, ev.Type, ev.ID, c.Path, err)
		}
		if !ok {
			continue
		}
		attrs, _, err := entity.ApplyOps(next, []entity.Op{op})
		if err != nil {
			return nil, fmt.Errorf("%w: %s %s: %v", ErrCorruptFact, ev.Type, ev.ID, err)
		}
		next.Attributes = attrs
	}
	next.Version = p.Version
	next.UpdatedAt = p.AppliedAt
	next.LastEventID = ev.ID
	if p.Entity.Name != "" {
		next.Name = p.Entity.Name
	}
	return next, nil
}

// opFor turns one element of changed[] into the operation that reproduces it
// (C-02 v1.6, rule of catching up): new present — write it; new absent —
// delete the path if it is there. An element a[n] with n equal to the length
// of the list is appended as it is, without the deduplication of append: State
// already decided it is a new element.
func opFor(e *entity.Entity, c changedPath) (entity.Op, bool, error) {
	if !c.New.set {
		if !e.HasAttr(c.Path) {
			return entity.Op{}, false, nil
		}
		return entity.Op{Op: entity.OpRemove, Path: c.Path}, true, nil
	}
	parent, index, isElement := elementOf(c.Path)
	if !isElement {
		return entity.Op{Op: entity.OpSet, Path: c.Path, Value: c.New.value}, true, nil
	}
	raw, _ := e.Attr(parent)
	list, isList := raw.([]any)
	if raw != nil && !isList {
		return entity.Op{Op: entity.OpSet, Path: c.Path, Value: c.New.value}, true, nil
	}
	switch {
	case index < len(list):
		return entity.Op{Op: entity.OpSet, Path: c.Path, Value: c.New.value}, true, nil
	case index == len(list):
		grown := append(slices.Clone(list), c.New.value)
		return entity.Op{Op: entity.OpSet, Path: parent, Value: grown}, true, nil
	default:
		return entity.Op{}, false, fmt.Errorf("element %d past the end of a list of %d", index, len(list))
	}
}

// elementOf splits a path that ends with an index into the path of the list
// and the index.
func elementOf(path string) (string, int, bool) {
	if !strings.HasSuffix(path, "]") {
		return "", 0, false
	}
	open := strings.LastIndexByte(path, '[')
	if open <= 0 {
		return "", 0, false
	}
	index, err := strconv.Atoi(path[open+1 : len(path)-1])
	if err != nil || index < 0 {
		return "", 0, false
	}
	return path[:open], index, true
}

// resolves says whether a fact moves an encounter to resolved.
func resolves(changed []changedPath) bool {
	for _, c := range changed {
		if c.Path == entity.AttrState && c.New.set && c.New.value == entity.EncounterStateResolved {
			return true
		}
	}
	return false
}

func (m *Model) applyStarted(ev eventbus.Event, p startedPayload) (Result, error) {
	id := p.Encounter.Entity.ID
	if id == "" {
		return Result{}, fmt.Errorf("%w: %s %s without an encounter", ErrCorruptFact, ev.Type, ev.ID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.lifecycleOf(id)
	var res Result
	if l.started == nil {
		l.started, l.world = &p, worldOf(ev)
		if ev.Scope != nil {
			scope := *ev.Scope
			l.scope = &scope
		}
		res.Applied = true
	}
	res.EncounterOpened = m.claimOpen(id, ev.ID)
	m.reindex(id)
	return res, nil
}

func (m *Model) applyEnded(ev eventbus.Event, p endedPayload) (Result, error) {
	id := p.Encounter.Entity.ID
	if id == "" {
		return Result{}, fmt.Errorf("%w: %s %s without an encounter", ErrCorruptFact, ev.Type, ev.ID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.lifecycleOf(id)
	var res Result
	if !l.ended {
		l.ended, l.endReason = true, p.Reason
		res.Applied = true
	}
	res.EncounterEnded = m.claimEnd(id, ev.ID)
	m.reindex(id)
	return res, nil
}

// claimOpen gives the opening of an encounter to the first of its two opening
// events, and only while the encounter is not over: a start that arrives
// behind the end of its own fight opens nothing. It must be called with m.mu
// held.
func (m *Model) claimOpen(id, eventID string) string {
	l := m.lifecycleOf(id)
	if l.openedBy == "" {
		if enc, ok := m.encounterOf(id); ok && !enc.Open() {
			return ""
		}
		l.openedBy = eventID
	}
	if l.openedBy != eventID {
		return ""
	}
	return id
}

// claimEnd gives the end of an encounter to the first of its two ending
// events. It must be called with m.mu held.
func (m *Model) claimEnd(id, eventID string) string {
	l := m.lifecycleOf(id)
	if l.closedBy == "" {
		l.closedBy = eventID
	}
	if l.closedBy != eventID {
		return ""
	}
	return id
}

// lifecycleOf must be called with m.mu held.
func (m *Model) lifecycleOf(id string) *lifecycle {
	l, ok := m.encounters[id]
	if !ok {
		l = &lifecycle{}
		m.encounters[id] = l
	}
	return l
}

// encounterOf merges the entity and the life cycle of an encounter. It must be
// called with m.mu held.
func (m *Model) encounterOf(id string) (Encounter, bool) {
	e, hasEntity := m.entityOf(id, entity.TypeEncounter)
	l := m.encounters[id]
	if !hasEntity && l == nil {
		return Encounter{}, false
	}
	enc := Encounter{ID: id}
	if l != nil && l.started != nil {
		enc.WorldID = l.world
		enc.RegionID = l.started.Region.Entity.ID
		if l.scope != nil {
			enc.Scope = *l.scope
		}
		enc.Participants = ids(l.started.Participants)
		enc.NPCIDs = ids(l.started.NPCs)
		if r := l.started.Round; r != nil {
			enc.Round = &RoundParams{Timeout: r.Timeout, IdleAfterMissed: r.IdleAfterMissed}
		}
	}
	closed := l != nil && l.ended
	if hasEntity {
		m.mergeEntity(&enc, e)
		if state, _ := e.State(); state == entity.EncounterStateResolved {
			closed = true
		}
	}
	if enc.Resolution == "" && l != nil {
		enc.Resolution = l.endReason
	}
	switch {
	case closed:
		enc.State = entity.EncounterStateResolved
	case hasEntity:
		enc.State = entity.EncounterStateActive
	case l.started != nil:
		enc.State = EncounterAnnounced
	default:
		return Encounter{}, false
	}
	return enc, true
}

// mergeEntity lays what the entity says over what the start said: the entity
// is the fact of State, the start only its announcement.
func (m *Model) mergeEntity(enc *Encounter, e *entity.Entity) {
	enc.WorldID, enc.Version = e.WorldID, e.Version
	if region, ok := e.RegionID(); ok {
		enc.RegionID = region
	}
	if scope, ok := e.Scope(); ok {
		enc.Scope = scope
	}
	if e.HasAttr(entity.AttrParticipants) {
		participants, err := e.Participants()
		if err != nil {
			m.log.Warn("readmodel: participants do not decode", slog.String("entity_id", e.ID), slog.String("error", err.Error()))
		}
		enc.Participants = make([]string, 0, len(participants))
		for _, p := range participants {
			enc.Participants = append(enc.Participants, p.PlayerID)
		}
	}
	if e.HasAttr(entity.AttrNPCs) {
		npcs, err := e.NPCs()
		if err != nil {
			m.log.Warn("readmodel: npcs do not decode", slog.String("entity_id", e.ID), slog.String("error", err.Error()))
		}
		enc.NPCIDs = make([]string, 0, len(npcs))
		for _, n := range npcs {
			enc.NPCIDs = append(enc.NPCIDs, n.NPCID)
		}
	}
	enc.Resolution, _ = e.Resolution()
	enc.RoundSeq, _ = e.RoundSeq()
}

func ids(list []eventbus.Entity) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.Entity.ID)
	}
	return out
}

func worldOf(ev eventbus.Event) string {
	if ev.World != nil {
		return ev.World.Entity.ID
	}
	return ""
}

// decode reads the payload of an event into its typed form, the way the wire
// carries it.
func decode(ev eventbus.Event, dst any) error {
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		return fmt.Errorf("readmodel: %s %s: %w", ev.Type, ev.ID, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrCorruptFact, ev.Type, ev.ID, err)
	}
	return nil
}
