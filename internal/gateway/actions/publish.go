package actions

import (
	"crypto/sha256"
	"encoding/hex"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Causes of the proposals the gateway makes for an action (C-02).
const (
	CauseMove = "move"
	CauseRest = "rest"
)

// DefendCausePlayer is player.defended by the action of the player, as opposed
// to the round timeout the coordinator defends for (C-04).
const DefendCausePlayer = "player"

// OutsidePosition is the position of a character standing in no region of its
// world (shared/entity Position).
func OutsidePosition(worldID string) string { return "outside:" + worldID }

// KeyHash is action.key_hash: the SHA-256 of action_key in hex. The key itself
// stays in the gateway.
func KeyHash(actionKey string) string {
	sum := sha256.Sum256([]byte(actionKey))
	return hex.EncodeToString(sum[:])
}

// BuildPlayerEvent builds the root player.* event of a validated action
// (api-contracts.md §2.3.1): its correlation_id is its own id and it has no
// cause (C-01 v1.9, the root of an input). text is the text of say after the
// input filter; turn, when known, names the session.
func BuildPlayerEvent(v Validated, text string, turn *api.TurnRef, gmPath string) eventbus.Event {
	ch := v.Character
	payload := map[string]any{
		"entity": ref(ch.ID, entity.TypePlayer, ch.Name),
		"action": map[string]any{"type": v.Command.Type, "key_hash": KeyHash(v.Command.ActionKey)},
	}
	if turn != nil && turn.SessionID != "" {
		payload["session"] = map[string]any{"id": turn.SessionID}
	}
	switch v.Command.Type {
	case api.ActionEnter:
		payload["target"] = ref(v.Region.ID, entity.TypeRegion, v.Region.Name)
		payload["position"] = map[string]any{"from": nullable(ch.Position), "to": v.Region.ID}
	case api.ActionLeave:
		payload["target"] = ref(v.Region.ID, entity.TypeRegion, v.Region.Name)
		payload["position"] = map[string]any{"from": nullable(ch.Position), "to": OutsidePosition(ch.WorldID)}
	case api.ActionLook:
		if v.Region != nil {
			payload["target"] = ref(v.Region.ID, entity.TypeRegion, v.Region.Name)
		}
	case api.ActionAttack:
		payload["target"] = ref(v.NPC.ID, entity.TypeNPC, v.NPC.Name)
		payload["encounter"] = ref(v.Encounter.ID, entity.TypeEncounter, "")
	case api.ActionFlee:
		payload["encounter"] = ref(v.Encounter.ID, entity.TypeEncounter, "")
	case api.ActionDefend:
		payload["cause"] = DefendCausePlayer
		payload["encounter"] = ref(v.Encounter.ID, entity.TypeEncounter, "")
	case api.ActionSay:
		payload["text"] = text
	}
	return eventbus.NewRoot(v.Rule.Event, contracts.SourceGateway, ch.WorldID, scopeOf(ch), v.Command.ActorKind,
		payload, eventbus.WithGMPath(gmPath))
}

// BuildPositionProposal is the proposal that goes with enter and leave: the
// position of the character and nothing else. The scope does not travel with
// it, because moving does not change it (C-04 v1.2).
func BuildPositionProposal(action eventbus.Event, ch readmodel.CharacterState, to string) eventbus.Event {
	return proposal(action, ch, CauseMove, entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: to})
}

// BuildRestProposal is the proposal that goes with rest: hp back to hp_max
// (C-02 v1.1). State refuses it during an encounter.
func BuildRestProposal(action eventbus.Event, ch readmodel.CharacterState) eventbus.Event {
	return proposal(action, ch, CauseRest, entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: ch.HPMax})
}

// ProposalID is the proposal_id of the proposal of move or rest that the
// action event actionEventID makes for the character playerID. The consumer
// matches a refusal of State against it to find the player of the action
// (T-307): the id is derived from the action and the character alone.
func ProposalID(actionEventID, playerID string) string {
	action := eventbus.Event{ID: actionEventID, Meta: eventbus.Meta{CorrelationID: actionEventID}}
	return eventbus.Derive(action, TypeUpdateProposed, contracts.SourceGateway, nil, eventbus.WithCauseID(playerID)).ID
}

// proposal derives an atomic entity.update.proposed from the action, so that
// the fact of State shares the correlation of the action. Its id comes from
// the action and the character (C-01 v1.4), and it is the proposal_id too. The
// version is pinned: hp and position are paths ADR-013 locks.
func proposal(action eventbus.Event, ch readmodel.CharacterState, cause string, op entity.Op) eventbus.Event {
	ev := eventbus.Derive(action, TypeUpdateProposed, contracts.SourceGateway, map[string]any{
		"changes": []any{map[string]any{
			"entity":           ref(ch.ID, entity.TypePlayer, ch.Name),
			"expected_version": ch.Version,
			"ops":              []any{map[string]any{"op": string(op.Op), "path": op.Path, "value": op.Value}},
		}},
		"atomic": true,
		"cause":  cause,
	}, eventbus.WithCauseID(ch.ID))
	ev.Payload["proposal_id"] = ev.ID
	return ev
}

// BuildGMCreated is the bridge to the legacy orchestrator: with
// MV_GM_PATH=legacy an accepted action also announces the game master of its
// scope, in the payload the as-is narrative-orchestrator reads (C-04 v1.4). The
// orchestrator keeps one GM per scope, so a repeat is harmless.
//
// Removed with the legacy profile in EPIC-003 I2 (S5).
func BuildGMCreated(v Validated) eventbus.Event {
	ch := v.Character
	scope := scopeOf(ch)
	return eventbus.NewRoot(TypeGMCreated, contracts.SourceGateway, ch.WorldID, scope, v.Command.ActorKind,
		map[string]any{
			"scope_id":   scope.ID,
			"scope_type": scope.Type,
			"config":     map[string]any{"perception": 0.8, "focus_entities": []any{ch.ID}},
		}, eventbus.WithGMPath(eventbus.GMPathLegacy))
}

// scopeOf is the scope the character acts in; a character without one acts
// alone (C-04: the scope of a solo character is its own id).
func scopeOf(ch readmodel.CharacterState) *eventbus.ScopeRef {
	if ch.Scope.ID != "" {
		scope := ch.Scope
		return &scope
	}
	return &eventbus.ScopeRef{ID: ch.ID, Type: "solo"}
}

func ref(id, typ, name string) map[string]any {
	out := map[string]any{"entity": map[string]any{"id": id, "type": typ}}
	if name != "" {
		out["name"] = name
	}
	return out
}

// nullable is the JSON null position.from allows for a position the projection
// does not know.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
