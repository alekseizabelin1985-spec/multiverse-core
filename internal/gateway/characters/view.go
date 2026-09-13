package characters

import (
	"context"
	"strings"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/session"
)

// Position kinds of api.PositionRef.
const (
	PositionRegion  = "region"
	PositionOutside = "outside"
)

// World is what a view reads of the read model besides the character.
type World interface {
	Character(id string) (readmodel.CharacterState, bool)
	World(id string) (readmodel.World, bool)
	Region(worldID, regionID string) (readmodel.Region, bool)
	NPC(id string) (readmodel.NPC, bool)
	Group(id string) (readmodel.Group, bool)
	EncounterOf(playerID string) (readmodel.Encounter, bool)
}

// Sessions is what a view reads of the sessions.
type Sessions interface {
	Current(ctx context.Context, scopeID string) (session.Session, bool, error)
}

// Viewer builds CharacterState, the answer of GET /v1/players/{player_id} and
// the character of 200 and 201 of POST /v1/characters (api-contracts.md §1.3).
type Viewer struct {
	World    World
	Sessions Sessions
}

// View is the state of a character the projection holds. The session is left
// out when it cannot be read: the state of the character is the answer, the
// session a detail of it.
func (v Viewer) View(ctx context.Context, ch readmodel.CharacterState) *api.CharacterState {
	hp, hpMax, version := ch.HP, ch.HPMax, ch.Version
	out := &api.CharacterState{
		PlayerID: ch.ID, Name: ch.Name, WorldID: ch.WorldID, Status: ch.Status,
		HP: &hp, HPMax: &hpMax, Version: &version, Inventory: []api.Item{},
	}
	out.Position = v.position(ch)
	if ch.Scope.ID != "" {
		out.Scope = &api.ScopeRef{ID: ch.Scope.ID, Type: ch.Scope.Type}
	}
	for _, it := range ch.Inventory {
		out.Inventory = append(out.Inventory, api.Item{ItemID: it.ItemID, Kind: it.Kind, Name: it.Name})
	}
	if w, ok := v.World.World(ch.WorldID); ok && (w.Weather != "" || w.TimeOfDay != "" || w.Day != nil) {
		out.World = &api.WorldView{Weather: w.Weather, TimeOfDay: w.TimeOfDay, Day: w.Day}
	}
	if enc, ok := v.World.EncounterOf(ch.ID); ok {
		out.Encounter = v.encounter(enc)
	}
	if ch.GroupID != "" {
		out.Group = v.group(ch.GroupID)
	}
	if v.Sessions != nil && ch.Scope.ID != "" {
		if s, found, err := v.Sessions.Current(ctx, ch.Scope.ID); err == nil && found {
			out.Session = &api.SessionRef{SessionID: s.ID, TurnsCount: s.TurnsCount}
		}
	}
	return out
}

// Creating is the state of a character whose fact has not come: player_id,
// name, world_id and status only (api-contracts.md §1.3).
func Creating(p Pending) *api.CharacterState {
	return &api.CharacterState{PlayerID: p.PlayerID, Name: p.Name, WorldID: p.WorldID, Status: StatusCreating}
}

func (v Viewer) position(ch readmodel.CharacterState) *api.PositionRef {
	switch {
	case ch.Position == "":
		return nil
	case strings.HasPrefix(ch.Position, PositionOutside+":"):
		ref := &api.PositionRef{Kind: PositionOutside, ID: strings.TrimPrefix(ch.Position, PositionOutside+":")}
		if w, ok := v.World.World(ref.ID); ok {
			ref.Name = w.Name
		}
		return ref
	default:
		ref := &api.PositionRef{Kind: PositionRegion, ID: ch.Position}
		if r, ok := v.World.Region(ch.WorldID, ch.Position); ok {
			ref.Name = r.Name
		}
		return ref
	}
}

func (v Viewer) encounter(enc readmodel.Encounter) *api.EncounterView {
	view := &api.EncounterView{EncounterID: enc.ID, NPCs: []api.NPCView{}}
	for _, id := range enc.NPCIDs {
		if n, ok := v.World.NPC(id); ok {
			view.NPCs = append(view.NPCs, api.NPCView{NPCID: n.ID, Name: n.Name, HP: n.HP, HPMax: n.HPMax, Status: n.Status})
		}
	}
	if enc.RoundSeq > 0 {
		seq := enc.RoundSeq
		view.RoundSeq = &seq
	}
	return view
}

func (v Viewer) group(id string) *api.GroupView {
	g, ok := v.World.Group(id)
	if !ok {
		return nil
	}
	view := &api.GroupView{GroupID: g.ID, LeaderID: g.LeaderID, Members: []api.GroupMember{}, State: g.State}
	for _, m := range g.Members {
		member := api.GroupMember{PlayerID: m.PlayerID, Participation: m.Participation}
		if ch, ok := v.World.Character(m.PlayerID); ok {
			member.Name, member.Status = ch.Name, ch.Status
		}
		view.Members = append(view.Members, member)
	}
	view.Position = v.position(readmodel.CharacterState{WorldID: g.WorldID, Position: g.Position})
	return view
}
