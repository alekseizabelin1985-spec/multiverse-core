// Package api holds the wire types of the gateway HTTP API (C-08 v1.3), the
// table of its error codes and the route table the handlers are mounted by.
// The types are shared with internal/gateway/client, so the bot, the CI
// harness and the gateway speak one set of structs.
//
// The specification is api/gateway.openapi.yaml; openapi_test.go keeps these
// structs, the error table and the spec in step. JSON conventions of the
// structs, which the test relies on:
//   - a field without omitempty is required;
//   - a pointer field without omitempty is required and nullable;
//   - a field with omitempty is optional;
//   - a required slice encodes as [] when nil, never as null (dto_json.go).
package api

import "time"

// ContentTypeJSON is the content type of every request and response body.
const ContentTypeJSON = "application/json; charset=utf-8"

// Headers of the API (api-contracts.md §1.1, C-08 v1.1). The names match the
// admin envelope of the process HTTP server (shared/runtime).
const (
	HeaderClientID   = "X-Client-Id"
	HeaderActorKind  = "X-Actor-Kind"
	HeaderRequestID  = "X-Request-Id"
	HeaderRetryAfter = "Retry-After"
)

// Actor kinds a client may declare in X-Actor-Kind; human is the default.
// system is an envelope value of the platform itself, never of a client.
const (
	ActorHuman = "human"
	ActorCI    = "ci"
	ActorSim   = "sim"
)

// ActorKinds returns the values X-Actor-Kind accepts.
func ActorKinds() []string { return []string{ActorHuman, ActorCI, ActorSim} }

// Action types of POST /v1/players/{player_id}/actions (api-contracts.md §1.4).
const (
	ActionEnter       = "enter"
	ActionLeave       = "leave"
	ActionLook        = "look"
	ActionAttack      = "attack"
	ActionFlee        = "flee"
	ActionRest        = "rest"
	ActionSay         = "say"
	ActionDefend      = "defend"
	ActionGroupCreate = "group.create"
	ActionGroupJoin   = "group.join"
	ActionGroupLeave  = "group.leave"
)

// ActionTypes returns the action dictionary in the order of §1.4.
func ActionTypes() []string {
	return []string{
		ActionEnter, ActionLeave, ActionLook, ActionAttack, ActionFlee, ActionRest,
		ActionSay, ActionDefend, ActionGroupCreate, ActionGroupJoin, ActionGroupLeave,
	}
}

// ResolveRequest is the body of POST /v1/links/resolve.
type ResolveRequest struct {
	ExternalPlatform string `json:"external_platform"`
	ExternalID       string `json:"external_id"`
}

// ResolveResponse tells the client where the external account stands. The
// link_id of the link never leaves the gateway (SEC-03).
type ResolveResponse struct {
	LinkStatus      string  `json:"link_status"`
	PlayerID        *string `json:"player_id"`
	WorldID         *string `json:"world_id"`
	CharacterStatus string  `json:"character_status"`
	NoticeDue       bool    `json:"notice_due"`
}

// ConsentRequest is the body of POST /v1/links/consent.
type ConsentRequest struct {
	ExternalPlatform string    `json:"external_platform"`
	ExternalID       string    `json:"external_id"`
	NoticeShown      bool      `json:"notice_shown"`
	Consent          bool      `json:"consent"`
	AgeConfirmed     bool      `json:"age_confirmed"`
	ShownAt          time.Time `json:"shown_at"`
}

// ConsentResponse is the answer to a complete consent.
type ConsentResponse struct {
	LinkStatus     string    `json:"link_status"`
	ConsentAt      time.Time `json:"consent_at"`
	AgeConfirmedAt time.Time `json:"age_confirmed_at"`
	NoticeShownAt  time.Time `json:"notice_shown_at"`
}

// ForgetRequest is the body of DELETE /v1/links.
type ForgetRequest struct {
	ExternalPlatform string `json:"external_platform"`
	ExternalID       string `json:"external_id"`
}

// ForgetResponse reports what /forget removed; player_id_detached is null when
// there was no link or no character.
type ForgetResponse struct {
	Deleted          bool    `json:"deleted"`
	PlayerIDDetached *string `json:"player_id_detached"`
}

// WorldsResponse is the answer of GET /v1/worlds.
type WorldsResponse struct {
	Worlds []WorldSummary `json:"worlds"`
}

// WorldSummary is one world a character can be created in.
type WorldSummary struct {
	WorldID     string          `json:"world_id"`
	Name        string          `json:"name"`
	Regions     []RegionSummary `json:"regions"`
	LawsVersion string          `json:"laws_version"`
	LLM         WorldLLM        `json:"llm"`
}

// RegionSummary is a region of a world in the world list.
type RegionSummary struct {
	RegionID string `json:"region_id"`
	Name     string `json:"name"`
}

// WorldLLM carries the only LLM fact a client sees: whether texts go to a cloud
// model. Provider, URL and keys are never exposed (C-08 v1.2, SEC-21).
type WorldLLM struct {
	CloudEnabled bool `json:"cloud_enabled"`
}

// CreateCharacterRequest is the body of POST /v1/characters; action_key makes
// the call idempotent within the link.
type CreateCharacterRequest struct {
	ExternalPlatform string `json:"external_platform"`
	ExternalID       string `json:"external_id"`
	WorldID          string `json:"world_id"`
	CharacterName    string `json:"character_name"`
	ActionKey        string `json:"action_key"`
}

// CreateCharacterResponse covers the three successful answers: 201 (created,
// character), 200 (a living character already exists, created=false) and 202
// (status=creating, the fact is not there yet).
type CreateCharacterResponse struct {
	PlayerID  string          `json:"player_id"`
	Character *CharacterState `json:"character,omitempty"`
	Created   *bool           `json:"created,omitempty"`
	Status    string          `json:"status,omitempty"`
}

// CharacterState is the answer of GET /v1/players/{player_id}. While the
// character is creating only player_id, name, world_id and status are present.
type CharacterState struct {
	PlayerID  string         `json:"player_id"`
	Name      string         `json:"name"`
	WorldID   string         `json:"world_id"`
	Status    string         `json:"status"`
	HP        *int           `json:"hp,omitempty"`
	HPMax     *int           `json:"hp_max,omitempty"`
	Position  *PositionRef   `json:"position,omitempty"`
	Scope     *ScopeRef      `json:"scope,omitempty"`
	Group     *GroupView     `json:"group,omitempty"`
	Encounter *EncounterView `json:"encounter,omitempty"`
	Inventory []Item         `json:"inventory,omitempty"`
	World     *WorldView     `json:"world,omitempty"`
	Session   *SessionRef    `json:"session,omitempty"`
	Version   *int64         `json:"version,omitempty"`
}

// PositionRef is where a character or a group is.
type PositionRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// ScopeRef is the scope of play: solo or group.
type ScopeRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// GroupView is a group as a client sees it; leader_id is null when no living
// member is left (C-08 v1.2, BR-13).
type GroupView struct {
	GroupID   string         `json:"group_id"`
	LeaderID  *string        `json:"leader_id"`
	Members   []GroupMember  `json:"members"`
	Position  *PositionRef   `json:"position,omitempty"`
	Encounter *EncounterView `json:"encounter,omitempty"`
	State     string         `json:"state,omitempty"`
}

// GroupMember is one member of a group; dead and abandoned members stay in the
// list as history.
type GroupMember struct {
	PlayerID      string `json:"player_id"`
	Name          string `json:"name"`
	Participation string `json:"participation"`
	Status        string `json:"status,omitempty"`
}

// EncounterView is the encounter a character or a group is in.
type EncounterView struct {
	EncounterID string    `json:"encounter_id"`
	NPCs        []NPCView `json:"npcs"`
	MyState     string    `json:"my_state,omitempty"`
	RoundSeq    *int      `json:"round_seq,omitempty"`
}

// NPCView is an opponent in an encounter.
type NPCView struct {
	NPCID  string `json:"npc_id"`
	Name   string `json:"name"`
	HP     int    `json:"hp"`
	HPMax  int    `json:"hp_max"`
	Status string `json:"status"`
}

// Item is an inventory item.
type Item struct {
	ItemID string `json:"item_id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
}

// WorldView is the state of the world around a character.
type WorldView struct {
	Weather   string `json:"weather,omitempty"`
	TimeOfDay string `json:"time_of_day,omitempty"`
	Day       *int   `json:"day,omitempty"`
}

// SessionRef is the current session of the character's scope.
type SessionRef struct {
	SessionID  string `json:"session_id"`
	TurnsCount int    `json:"turns_count"`
}

// ActionRequest is the body of POST /v1/players/{player_id}/actions. A repeat
// with the same action_key returns the first answer (C-08).
type ActionRequest struct {
	ActionKey string     `json:"action_key"`
	Type      string     `json:"type"`
	Target    *string    `json:"target,omitempty"`
	Text      *string    `json:"text,omitempty"`
	ClientTS  *time.Time `json:"client_ts,omitempty"`
}

// ActionAccepted is the 202 answer to an accepted action.
type ActionAccepted struct {
	CorrelationID string    `json:"correlation_id"`
	Turn          TurnRef   `json:"turn"`
	Status        string    `json:"status"`
	AckedAt       time.Time `json:"acked_at"`
}

// ActionPending is the 202 answer to a group.* action whose fact did not arrive
// within MV_GATEWAY_FACT_WAIT; the client polls GET /v1/groups/{group_id}
// (C-08 v1.1).
type ActionPending struct {
	Status        string `json:"status"`
	GroupID       string `json:"group_id"`
	CorrelationID string `json:"correlation_id"`
}

// Statuses of the 202 answers to an action.
const (
	ActionStatusAccepted = "accepted"
	ActionStatusPending  = "pending"
)

// TurnRef identifies the turn an action became.
type TurnRef struct {
	Seq       int    `json:"seq"`
	SessionID string `json:"session_id"`
	RoundSeq  *int   `json:"round_seq,omitempty"`
}

// Delivery is one message for a player, taken from the outbox by long-poll
// (api-contracts.md §1.5). route is present only for a client of the platform
// of the link (SEC-12).
type Delivery struct {
	ID               string         `json:"id"`
	PlayerID         string         `json:"player_id"`
	Route            *DeliveryRoute `json:"route,omitempty"`
	Kind             string         `json:"kind"`
	CorrelationID    string         `json:"correlation_id"`
	EventID          string         `json:"event_id"`
	RoundSeq         *int           `json:"round_seq,omitempty"`
	GeneratedBy      string         `json:"generated_by"`
	FallbackReason   *string        `json:"fallback_reason,omitempty"`
	NarrativeEventID *string        `json:"narrative_event_id,omitempty"`
	Text             string         `json:"text"`
	Data             map[string]any `json:"data,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

// DeliveryRoute is the external address of a delivery.
type DeliveryRoute struct {
	ExternalPlatform string `json:"external_platform"`
	ExternalID       string `json:"external_id"`
}

// DeliveriesResponse is the answer of the long-poll; deliveries is an empty
// array, not null, when nothing arrived within wait_ms.
type DeliveriesResponse struct {
	Deliveries []Delivery `json:"deliveries"`
	Cursor     string     `json:"cursor"`
}

// AckRequest confirms deliveries by id.
type AckRequest struct {
	IDs []string `json:"ids"`
}

// AckResponse reports the confirmed deliveries and the ids the gateway does not
// know (expired, dropped or of another client).
type AckResponse struct {
	Acked   int      `json:"acked"`
	Unknown []string `json:"unknown"`
}

// RoundCloseResponse is the answer of POST /v1/scopes/{scope_id}/rounds/close.
type RoundCloseResponse struct {
	Round ClosedRound `json:"round"`
}

// ClosedRound is the round a close request closed.
type ClosedRound struct {
	Seq         int    `json:"seq"`
	CloseReason string `json:"close_reason"`
}

// AdminSessions is the answer of GET /v1/admin/sessions; it carries no external
// identifiers.
type AdminSessions struct {
	Sessions []AdminSession `json:"sessions"`
}

// AdminSession is one active session.
type AdminSession struct {
	SessionID  string    `json:"session_id"`
	Kind       string    `json:"kind"`
	Scope      ScopeRef  `json:"scope"`
	WorldID    string    `json:"world_id"`
	ActorKind  string    `json:"actor_kind"`
	StartedAt  time.Time `json:"started_at"`
	TurnsCount int       `json:"turns_count"`
}

// ErrorResponse is the body of every 4xx and 5xx answer: the schema Error of
// the spec. The handler-side error value is api.Error, whose Body returns this.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries the code a client matches on and a message for the player.
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}
