package swarm

import (
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Scope types of the index (the scope types of a blueprint, C-11).
const (
	scopeWorld  = "world"
	scopeRegion = "region"
	scopeSolo   = "solo"
	scopeGroup  = "group"
)

// outsidePrefix is the position of a character standing in no region:
// outside:{world_id} (data-model.md §3.3).
const outsidePrefix = "outside:"

// ScopeIndex answers where a scope is: solo and group scopes lie in a region,
// a region lies in a world (swarm-llm-laws.md §2, §5.2). The Router feeds it
// every event (Apply, step 3 of §5.1) and the roles ask it whom an event
// concerns.
//
// Each relation has one source, so that the events of two publishers never
// argue about it:
//   - where a character or a group stands — the position of the entity in
//     entity.created and entity.updated (State; §7.2: only the position);
//   - who is in a group — the members of group.created, group.joined,
//     group.left and group.disbanded (gateway);
//   - which encounter a scope is in — encounter.started and encounter.ended.
//
// Apply sets relations, it never counts: an event applied twice leaves the
// index as it left it once (§5.1 step 3, T-431). Every other event type is
// ignored. The index is safe for concurrent use.
type ScopeIndex struct {
	mu         sync.RWMutex
	players    map[string]*placement // player id → where it stands
	groups     map[string]*groupEntry
	groupOf    map[string]string // player id → group id
	regions    map[string]string // region id → world id
	encounters map[string]encounterEntry
	engaged    map[string]string // scope key → encounter id
}

type placement struct {
	world  string
	region string // empty when the entity stands outside every region
}

type groupEntry struct {
	placement
	members []string // sorted
}

type encounterEntry struct {
	scope eventbus.ScopeRef
}

// NewScopeIndex returns an empty index.
func NewScopeIndex() *ScopeIndex {
	return &ScopeIndex{
		players:    map[string]*placement{},
		groups:     map[string]*groupEntry{},
		groupOf:    map[string]string{},
		regions:    map[string]string{},
		encounters: map[string]encounterEntry{},
		engaged:    map[string]string{},
	}
}

// The payload shapes the index reads. The payload is decoded through JSON, so
// an event built in the process (slices of maps) reads the same as one that
// came over the wire ([]any).
type (
	refPayload struct {
		Entity eventbus.EntityRef `json:"entity"`
	}
	entityPayload struct {
		Entity     refPayload     `json:"entity"`
		Attributes map[string]any `json:"attributes"`
		Changed    []struct {
			Path string `json:"path"`
			New  any    `json:"new"`
		} `json:"changed"`
	}
	groupPayload struct {
		Group   refPayload   `json:"group"`
		Members []refPayload `json:"members"`
	}
	encounterPayload struct {
		Encounter refPayload `json:"encounter"`
		Region    refPayload `json:"region"`
	}
)

func decodePayload(ev eventbus.Event, dst any) bool {
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// Apply updates the index from one event.
func (x *ScopeIndex) Apply(ev eventbus.Event) {
	world := ""
	if ev.World != nil {
		world = ev.World.Entity.ID
	}
	switch ev.Type {
	case "entity.created", "entity.updated":
		var p entityPayload
		if decodePayload(ev, &p) && p.Entity.Entity.ID != "" {
			x.applyEntity(world, p)
		}
	case "group.created", "group.joined", "group.left":
		var p groupPayload
		if decodePayload(ev, &p) && p.Group.Entity.ID != "" {
			x.setMembers(world, p)
		}
	case "group.disbanded":
		var p groupPayload
		if decodePayload(ev, &p) && p.Group.Entity.ID != "" {
			x.disband(p.Group.Entity.ID)
		}
	case "encounter.started":
		var p encounterPayload
		if ev.Scope != nil && decodePayload(ev, &p) && p.Encounter.Entity.ID != "" {
			x.startEncounter(world, normalizeScope(*ev.Scope), p)
		}
	case "encounter.ended":
		var p encounterPayload
		if decodePayload(ev, &p) && p.Encounter.Entity.ID != "" {
			x.endEncounter(p.Encounter.Entity.ID)
		}
	}
}

// applyEntity reads the attributes of a new entity, or the changed paths of
// an updated one: the position of a character or a group, the world of a
// region, the end of a group.
func (x *ScopeIndex) applyEntity(world string, p entityPayload) {
	values := p.Attributes
	if values == nil {
		values = make(map[string]any, len(p.Changed))
		for _, change := range p.Changed {
			values[change.Path] = change.New
		}
	}
	id := p.Entity.Entity.ID
	x.mu.Lock()
	defer x.mu.Unlock()
	switch p.Entity.Entity.Type {
	case entity.TypePlayer:
		if position, ok := values[entity.AttrPosition].(string); ok {
			x.place(x.player(id), world, position)
		}
	case entity.TypeGroup:
		if state, _ := values[entity.AttrState].(string); state == entity.GroupStateDisbanded {
			x.disbandLocked(id)
			return
		}
		if position, ok := values[entity.AttrPosition].(string); ok {
			x.place(&x.group(id).placement, world, position)
		}
	case entity.TypeRegion:
		if world != "" {
			x.regions[id] = world
		}
	}
}

func (x *ScopeIndex) player(id string) *placement {
	pl, ok := x.players[id]
	if !ok {
		pl = &placement{}
		x.players[id] = pl
	}
	return pl
}

func (x *ScopeIndex) group(id string) *groupEntry {
	g, ok := x.groups[id]
	if !ok {
		g = &groupEntry{}
		x.groups[id] = g
	}
	return g
}

func (x *ScopeIndex) place(pl *placement, world, position string) {
	if world != "" {
		pl.world = world
	}
	if strings.HasPrefix(position, outsidePrefix) {
		pl.region = ""
		if pl.world == "" {
			pl.world = strings.TrimPrefix(position, outsidePrefix)
		}
		return
	}
	pl.region = position
	if position != "" && pl.world != "" {
		x.regions[position] = pl.world
	}
}

// setMembers makes the member list of a group exactly the one of the event:
// a player it no longer lists leaves the group, a player it lists joins it
// and leaves any group it was in before.
func (x *ScopeIndex) setMembers(world string, p groupPayload) {
	id := p.Group.Entity.ID
	members := make([]string, 0, len(p.Members))
	for _, m := range p.Members {
		if m.Entity.ID != "" {
			members = append(members, m.Entity.ID)
		}
	}
	slices.Sort(members)
	members = slices.Compact(members)

	x.mu.Lock()
	defer x.mu.Unlock()
	g := x.group(id)
	if world != "" {
		g.world = world
	}
	for _, old := range g.members {
		if !slices.Contains(members, old) && x.groupOf[old] == id {
			delete(x.groupOf, old)
		}
	}
	for _, member := range members {
		if previous, ok := x.groupOf[member]; ok && previous != id {
			if pg, ok := x.groups[previous]; ok {
				pg.members = slices.DeleteFunc(pg.members, func(m string) bool { return m == member })
			}
		}
		x.groupOf[member] = id
	}
	g.members = members
}

func (x *ScopeIndex) disband(id string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.disbandLocked(id)
}

func (x *ScopeIndex) disbandLocked(id string) {
	if g, ok := x.groups[id]; ok {
		for _, member := range g.members {
			if x.groupOf[member] == id {
				delete(x.groupOf, member)
			}
		}
		delete(x.groups, id)
	}
}

func (x *ScopeIndex) startEncounter(world string, scope eventbus.ScopeRef, p encounterPayload) {
	id := p.Encounter.Entity.ID
	region := p.Region.Entity.ID
	x.mu.Lock()
	defer x.mu.Unlock()
	if previous, ok := x.encounters[id]; ok && x.engaged[scopeKey(previous.scope)] == id {
		delete(x.engaged, scopeKey(previous.scope))
	}
	x.encounters[id] = encounterEntry{scope: scope}
	x.engaged[scopeKey(scope)] = id
	if region != "" && world != "" {
		x.regions[region] = world
	}
}

func (x *ScopeIndex) endEncounter(id string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	enc, ok := x.encounters[id]
	if !ok {
		return
	}
	if x.engaged[scopeKey(enc.scope)] == id {
		delete(x.engaged, scopeKey(enc.scope))
	}
	delete(x.encounters, id)
}

// normalizeScope takes the id of a scope without the prefix of its type: both
// {type: solo, id: player-A} and {type: solo, id: solo:player-A} are written
// by publishers of MVP-1 (_common.json ScopeRef, shared/entity.Scope).
func normalizeScope(s eventbus.ScopeRef) eventbus.ScopeRef {
	return eventbus.ScopeRef{ID: strings.TrimPrefix(s.ID, s.Type+":"), Type: s.Type}
}

func scopeKey(s eventbus.ScopeRef) string {
	s = normalizeScope(s)
	return s.Type + ":" + s.ID
}

// RegionOf returns the region a scope lies in: a region is its own, a solo
// scope lies where its character stands, a group scope where its group
// stands. A world lies in no region, and neither does a scope standing
// outside every region or one the index has not heard of.
func (x *ScopeIndex) RegionOf(scope eventbus.ScopeRef) (string, bool) {
	s := normalizeScope(scope)
	x.mu.RLock()
	defer x.mu.RUnlock()
	var region string
	switch s.Type {
	case scopeRegion:
		region = s.ID
	case scopeSolo:
		if pl, ok := x.players[s.ID]; ok {
			region = pl.region
		}
	case scopeGroup:
		if g, ok := x.groups[s.ID]; ok {
			region = g.region
		}
	}
	return region, region != ""
}

// WorldOf returns the world a scope lies in.
func (x *ScopeIndex) WorldOf(scope eventbus.ScopeRef) (string, bool) {
	s := normalizeScope(scope)
	x.mu.RLock()
	defer x.mu.RUnlock()
	var world string
	switch s.Type {
	case scopeWorld:
		world = s.ID
	case scopeRegion:
		world = x.regions[s.ID]
	case scopeSolo:
		if pl, ok := x.players[s.ID]; ok {
			world = pl.world
		}
	case scopeGroup:
		if g, ok := x.groups[s.ID]; ok {
			world = g.world
		}
	}
	return world, world != ""
}

// GroupOf returns the group a player is a member of.
func (x *ScopeIndex) GroupOf(playerID string) (string, bool) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	group, ok := x.groupOf[playerID]
	return group, ok
}

// PlayersIn returns the players whose position is the region, sorted. Only
// the position counts, not a session or the status of the character
// (swarm-llm-laws.md §7.2).
func (x *ScopeIndex) PlayersIn(regionID string) []string {
	if regionID == "" {
		return nil
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	var out []string
	for id, pl := range x.players {
		if pl.region == regionID {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// EncounterOf returns the encounter a solo or group scope is in.
func (x *ScopeIndex) EncounterOf(scope eventbus.ScopeRef) (string, bool) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	id, ok := x.engaged[scopeKey(scope)]
	return id, ok
}

// EncounterScope returns the scope an encounter was opened for.
func (x *ScopeIndex) EncounterScope(encounterID string) (eventbus.ScopeRef, bool) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	enc, ok := x.encounters[encounterID]
	return enc.scope, ok
}
