package entity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/jsonpath"
)

// Typed access to the attributes of data-model.md §3. Two shapes of getter:
// a scalar answers (value, ok), where ok is false when the attribute is absent
// or holds something else; a structured one answers (value, error), because a
// members list that does not decode is a defect worth a message rather than a
// silent empty list.
//
// A getter never guesses. It reads the attribute the model defines and nothing
// near it: an entity that lacks hp is an entity that lacks hp, and whether that
// is allowed is a question for the create path of State.

// Attr returns the raw value at a dot path.
func (e *Entity) Attr(path string) (any, bool) {
	return jsonpath.New(e.Attributes).GetAny(path)
}

// HasAttr says whether anything is at a dot path.
func (e *Entity) HasAttr(path string) bool {
	return jsonpath.New(e.Attributes).Has(path)
}

// AttrString reads a string attribute.
func (e *Entity) AttrString(path string) (string, bool) {
	return jsonpath.New(e.Attributes).GetString(path)
}

// AttrBool reads a boolean attribute.
func (e *Entity) AttrBool(path string) (bool, bool) {
	return jsonpath.New(e.Attributes).GetBool(path)
}

// AttrFloat reads a numeric attribute that need not be whole.
func (e *Entity) AttrFloat(path string) (float64, bool) {
	return jsonpath.New(e.Attributes).GetFloat(path)
}

// AttrInt reads a whole number, including the signed modifier form the rules
// document uses for the combat stats: atk of "+2" and atk of 2 are the same
// two (data-model.md §3.3).
func (e *Entity) AttrInt(path string) (int, bool) {
	raw, ok := e.Attr(path)
	if !ok {
		return 0, false
	}
	if n, ok := asInt64(raw); ok {
		return int(n), true
	}
	text, ok := raw.(string)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(text, "+"))
	if err != nil {
		return 0, false
	}
	return n, true
}

// AttrTime reads an RFC 3339 timestamp attribute.
func (e *Entity) AttrTime(path string) (time.Time, bool) {
	text, ok := e.AttrString(path)
	if !ok {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, false
	}
	return at.UTC(), true
}

// AttrDuration reads a duration attribute written the Go way ("24h").
func (e *Entity) AttrDuration(path string) (time.Duration, bool) {
	text, ok := e.AttrString(path)
	if !ok {
		return 0, false
	}
	d, err := time.ParseDuration(text)
	if err != nil {
		return 0, false
	}
	return d, true
}

// AttrStrings reads a list of strings, skipping nothing: an element that is not
// a string makes the whole read fail, because a half-read list of ids is worse
// than none.
func (e *Entity) AttrStrings(path string) ([]string, bool) {
	raw, ok := e.Attr(path)
	if !ok {
		return nil, false
	}
	list, ok := asAnySlice(raw)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

// DecodeAttr fills dst from the attribute at path, the way the wire would.
// An absent attribute leaves dst untouched and returns false with no error.
func (e *Entity) DecodeAttr(path string, dst any) (bool, error) {
	raw, ok := e.Attr(path)
	if !ok || raw == nil {
		return false, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return false, fmt.Errorf("entity %s: attribute %s: %w", e.Ref(), path, err)
	}
	if err := json.Unmarshal(encoded, dst); err != nil {
		return false, fmt.Errorf("entity %s: attribute %s: %w", e.Ref(), path, err)
	}
	return true, nil
}

// --- character and NPC (data-model.md §3.3, §3.4) ---

// HP is the current hit points.
func (e *Entity) HP() (int, bool) { return e.AttrInt(AttrHP) }

// HPMax is the upper bound of HP; inc on hp clamps against it.
func (e *Entity) HPMax() (int, bool) { return e.AttrInt(AttrHPMax) }

// Atk is the attack modifier.
func (e *Entity) Atk() (int, bool) { return e.AttrInt(AttrAtk) }

// Def is the defence value a roll has to beat.
func (e *Entity) Def() (int, bool) { return e.AttrInt(AttrDef) }

// Dmg is the damage formula ("d6"), not a number: the rules own the dice.
func (e *Entity) Dmg() (string, bool) { return e.AttrString(AttrDmg) }

// Flee is the flee bonus, as the text a formula reads ("+2", "2").
//
// It is a modifier like atk (data-model.md §3.3), and the two writers of it
// disagree about the shape: the fixtures write "2", the rules document writes
// flee: 2. A whole number comes back as its decimal text, so a character
// created from the rules does not silently lose the ability to run away. A null
// flee — the wolf of the rules, which does not run — and an absent one are no
// flee at all.
//
// Anything else that is there — a fraction, a boolean, a number past the range
// of int64, an object — is a defect in the data, and it comes back as its
// canonical JSON text ("2.5", "true") with true. The formula that reads it
// fails on it exactly as it fails on the text "abc", instead of taking a broken
// bonus for a character who does not run (internal/mechanics.ActorFromEntity).
func (e *Entity) Flee() (string, bool) {
	raw, ok := e.Attr(AttrFlee)
	if !ok || raw == nil {
		return "", false
	}
	if text, ok := raw.(string); ok {
		return text, true
	}
	if n, ok := asInt64(raw); ok {
		return strconv.FormatInt(n, 10), true
	}
	var text bytes.Buffer
	writeCanonical(&text, raw)
	return text.String(), true
}

// Status is alive, dead, abandoned or ascended_final.
func (e *Entity) Status() (string, bool) { return e.AttrString(AttrStatus) }

// Position is a region id, or outside:{world_id} for a character who has not
// entered one.
func (e *Entity) Position() (string, bool) { return e.AttrString(AttrPosition) }

// Scope is the solo or group scope the entity acts in. It is stored either as
// the object of _common.json (id and type) or as the shorthand solo:{id} that
// the fixtures write, and both read the same way here.
func (e *Entity) Scope() (eventbus.ScopeRef, bool) {
	raw, ok := e.Attr(AttrScope)
	if !ok {
		return eventbus.ScopeRef{}, false
	}
	switch value := raw.(type) {
	case string:
		kind, id, found := strings.Cut(value, ":")
		if !found || kind == "" || id == "" {
			return eventbus.ScopeRef{}, false
		}
		return eventbus.ScopeRef{ID: id, Type: kind}, true
	case map[string]any:
		id, idOK := value["id"].(string)
		kind, typeOK := value["type"].(string)
		if !idOK || !typeOK {
			return eventbus.ScopeRef{}, false
		}
		return eventbus.ScopeRef{ID: id, Type: kind}, true
	default:
		return eventbus.ScopeRef{}, false
	}
}

// GroupID is the group the character belongs to, if any.
func (e *Entity) GroupID() (string, bool) { return e.AttrString(AttrGroupID) }

// EncounterID is the encounter the entity is in, if any.
func (e *Entity) EncounterID() (string, bool) { return e.AttrString(AttrEncounterID) }

// ActorKind is how the session behind the entity is driven: human, ci or sim.
func (e *Entity) ActorKind() (string, bool) { return e.AttrString(AttrActorKind) }

// ItemSource is where an item came from: the NPC it was taken from and the
// combat.decided that killed it. It is the proof that a trophy is handed out
// once (data-model.md §3.5, inv-03).
type ItemSource struct {
	Entity  eventbus.EntityRef `json:"entity"`
	EventID string             `json:"event_id"`
}

// Item is an element of an inventory (data-model.md §3.5).
type Item struct {
	ItemID     string     `json:"item_id"`
	Kind       string     `json:"kind"`
	Name       string     `json:"name"`
	Source     ItemSource `json:"source"`
	AcquiredAt time.Time  `json:"acquired_at"`
}

// Inventory is what the character carries.
func (e *Entity) Inventory() ([]Item, error) {
	var items []Item
	_, err := e.DecodeAttr(AttrInventory, &items)
	return items, err
}

// Kind is the sort of NPC ("wolf"), from the NPC table of the region blueprint.
func (e *Entity) Kind() (string, bool) { return e.AttrString(AttrKind) }

// RegionID is the home region of an NPC.
func (e *Entity) RegionID() (string, bool) { return e.AttrString(AttrRegionID) }

// DiedAt is when the NPC died; the respawn cooldown counts from it.
func (e *Entity) DiedAt() (time.Time, bool) { return e.AttrTime(AttrDiedAt) }

// KilledBy is the character who landed the last hit.
func (e *Entity) KilledBy() (string, bool) { return e.AttrString(AttrKilledBy) }

// LootEntry is one line of the loot table of an NPC (data-model.md §3.4).
type LootEntry struct {
	ItemKind string `json:"item_kind"`
	Name     string `json:"name"`
}

// Loot is what the NPC drops.
func (e *Entity) Loot() ([]LootEntry, error) {
	var loot []LootEntry
	_, err := e.DecodeAttr(AttrLoot, &loot)
	return loot, err
}

// SpawnSource is who put an NPC into the world: the agent that decided it and
// the tick it decided on (data-model.md §3.4). An NPC written by a blueprint
// has none.
type SpawnSource struct {
	Agent       string `json:"agent"`
	TickEventID string `json:"tick_event_id"`
}

// SpawnedBy is what spawned the NPC; a zero value means it was authored.
func (e *Entity) SpawnedBy() (SpawnSource, error) {
	var source SpawnSource
	_, err := e.DecodeAttr(AttrSpawnedBy, &source)
	return source, err
}

// last_session_ended_at (data-model.md §3.3) has no getter on purpose: the
// model marks it as an attribute that may live in game-service instead, as a
// projection of analytics.session.ended rather than a fact about the entity.

// --- world (data-model.md §3.1) ---

// LawsVersion is the version of the laws in force; only world.laws.changed
// moves it.
func (e *Entity) LawsVersion() (string, bool) { return e.AttrString(AttrLawsVersion) }

// Weather is the current weather of the world.
func (e *Entity) Weather() (string, bool) { return e.AttrString(AttrWeather) }

// TimeOfDay is dawn, day, dusk or night.
func (e *Entity) TimeOfDay() (string, bool) { return e.AttrString(AttrTimeOfDay) }

// Day is the game day since the world was created; it only grows.
func (e *Entity) Day() (int, bool) { return e.AttrInt(AttrDay) }

// Season is the season of the world, when it has one.
func (e *Entity) Season() (string, bool) { return e.AttrString(AttrSeason) }

// Epoch is the era the world is in. Like Season it is authored and, in MVP-1,
// nothing moves it.
func (e *Entity) Epoch() (string, bool) { return e.AttrString(AttrEpoch) }

// Locale is the language of the world ("ru").
func (e *Entity) Locale() (string, bool) { return e.AttrString(AttrLocale) }

// BlueprintRef is the blueprint the entity was authored from ("domain-dark-forest@1.0").
func (e *Entity) BlueprintRef() (string, bool) { return e.AttrString(AttrBlueprintRef) }

// --- region (data-model.md §3.2) ---

// Description is the authored description of a region.
func (e *Entity) Description() (string, bool) { return e.AttrString(AttrDescription) }

// CanonFact is one fact of the canon of a region (data-model.md §3.2).
type CanonFact struct {
	Fact    string `json:"fact"`
	Source  string `json:"source"`
	Version string `json:"version"`
}

// Canon is the canon of the region; MVP-1 writes only authored facts.
func (e *Entity) Canon() ([]CanonFact, error) {
	var canon []CanonFact
	_, err := e.DecodeAttr(AttrCanon, &canon)
	return canon, err
}

// NPCIDs are the NPCs that live in the region.
func (e *Entity) NPCIDs() ([]string, bool) { return e.AttrStrings(AttrNPCIDs) }

// RespawnTTL is how long a killed NPC stays dead.
func (e *Entity) RespawnTTL() (time.Duration, bool) { return e.AttrDuration(AttrRespawnTTL) }

// PerceptionRadius is the detection radius in abstract units; MVP-1 treats the
// whole region as one.
func (e *Entity) PerceptionRadius() (float64, bool) { return e.AttrFloat(AttrPerceptionRadius) }

// PlayersPresent are the characters standing in the region — a projection of
// their positions that may lag behind them; no law checks it — read positions
// (data-model §3.2).
func (e *Entity) PlayersPresent() ([]string, bool) { return e.AttrStrings(AttrPlayersPresent) }

// EncounterChance is the probability the region rolls for an encounter.
func (e *Entity) EncounterChance() (float64, bool) { return e.AttrFloat(AttrEncounterChance) }

// LastBackgroundEventAt is when the region last had a background event; the
// summary and the metrics read it (data-model.md §3.2).
func (e *Entity) LastBackgroundEventAt() (time.Time, bool) {
	return e.AttrTime(AttrLastBackgroundEventAt)
}

// --- group (data-model.md §3.6) ---

// Member is one participant of a group.
type Member struct {
	PlayerID      string    `json:"player_id"`
	JoinedAt      time.Time `json:"joined_at"`
	Participation string    `json:"participation"`
	MissedRounds  int       `json:"missed_rounds"`
}

// Members are the participants of the group, in the order they joined. A dead
// or abandoned member stays in the list as history and is excluded from the
// round elsewhere (C-04 v1.1).
func (e *Entity) Members() ([]Member, error) {
	var members []Member
	_, err := e.DecodeAttr(AttrMembers, &members)
	return members, err
}

// LeaderID is the leader of the group. The second value is false when the
// group has none: leader_id is nullable, and a group whose living members are
// gone keeps going leaderless until it disbands (BR-13 v0.4).
func (e *Entity) LeaderID() (string, bool) {
	id, ok := e.AttrString(AttrLeaderID)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// State is the state of a group (forming, active, disbanded) or of an
// encounter (active, resolved).
func (e *Entity) State() (string, bool) { return e.AttrString(AttrState) }

// --- encounter (data-model.md §3.7) ---

// Participant is one character in an encounter.
type Participant struct {
	PlayerID    string     `json:"player_id"`
	State       string     `json:"state"`
	DamageDealt int        `json:"damage_dealt"`
	LastHitAt   *time.Time `json:"last_hit_at,omitempty"`
}

// Participants are the characters in the encounter.
func (e *Entity) Participants() ([]Participant, error) {
	var participants []Participant
	_, err := e.DecodeAttr(AttrParticipants, &participants)
	return participants, err
}

// EncounterNPC is one NPC in an encounter and who hit it last — the first
// candidate its next attack considers (C-03 NPCTarget).
type EncounterNPC struct {
	NPCID       string `json:"npc_id"`
	LastDamager string `json:"last_damager,omitempty"`
}

// NPCs are the NPCs of the encounter.
func (e *Entity) NPCs() ([]EncounterNPC, error) {
	var npcs []EncounterNPC
	_, err := e.DecodeAttr(AttrNPCs, &npcs)
	return npcs, err
}

// Resolution is how the encounter ended: npc_dead, players_out or abandoned.
func (e *Entity) Resolution() (string, bool) { return e.AttrString(AttrResolution) }

// RoundSeq is the number of the current round.
func (e *Entity) RoundSeq() (int, bool) { return e.AttrInt(AttrRoundSeq) }

// TaskAgentID is the task agent running the encounter.
func (e *Entity) TaskAgentID() (string, bool) { return e.AttrString(AttrTaskAgentID) }

// OpenedByEventID is the event that opened the encounter — the encounter.started
// a reader follows back into the journal (data-model.md §3.7).
func (e *Entity) OpenedByEventID() (string, bool) { return e.AttrString(AttrOpenedByEventID) }

// ClosedByEventID is the event that resolved it. An encounter still running has
// none.
func (e *Entity) ClosedByEventID() (string, bool) { return e.AttrString(AttrClosedByEventID) }

// --- kinds of values (data-model.md §3; C-02 v1.8 p. 2) ---

// ValueKind is the kind of value data-model.md §3 gives an attribute.
type ValueKind string

// The kinds of §3. A text, a reference and an enumeration are all strings on
// the wire; they are named apart because the model names them apart, and
// nothing here checks that a reference resolves or that an enumeration holds
// one of its values — that is a question about the world, not about the form
// of a value.
const (
	KindText     ValueKind = "text"
	KindInteger  ValueKind = "integer"
	KindNumber   ValueKind = "number"
	KindTime     ValueKind = "time"     // RFC 3339, as AttrTime reads it
	KindDuration ValueKind = "duration" // a Go duration ("24h"), as AttrDuration reads it
	KindRef      ValueKind = "ref"
	KindEnum     ValueKind = "enum"
	// KindModifier is the "int / формула" of the combat stats (§3.3): a whole
	// number or the text of a formula ("+2", "d6"). AttrInt and Flee read
	// both.
	KindModifier ValueKind = "modifier"
	// KindOpen is a container of §3 (inventory[], members[], spawned_by) or
	// the scope, which is the object {id, type} or its shorthand. It is not a
	// scalar, its value is not checked, and it is in the table only for
	// whether a create has to carry it.
	KindOpen ValueKind = "open"
)

// AttributeSpec is one row of a table of data-model.md §3: the kind of the
// value, whether an entity of the type is created with it (a required
// attribute), and whether null stands for it. null is allowed on an optional
// attribute, and on the one required attribute whose row says so: the
// leader_id of a group without living members (§3.6).
//
// flee is optional although its row says "Обяз.: да": the same row says that
// null or no flee at all is "does not run" (§3.3), so a character created
// without it is a character that does not run (decision of the orchestrator
// on review #1 of T-472, to be confirmed by system-architect).
type AttributeSpec struct {
	Kind     ValueKind
	Required bool
	Nullable bool
}

// Scalar says whether the attribute is one value, below which no path goes.
func (s AttributeSpec) Scalar() bool { return s.Kind != KindOpen }

func required(kind ValueKind) AttributeSpec { return AttributeSpec{Kind: kind, Required: true} }
func optional(kind ValueKind) AttributeSpec { return AttributeSpec{Kind: kind, Nullable: true} }

// attrName is the name every entity has (§3, EntityBase). The entity carries it
// in Entity.Name, so a create need not repeat it among the attributes; an
// attribute of that name is still a text with nothing below it.
const attrName = "name"

// attrLastSessionEndedAt has no exported constant, for the reason given above
// the getters of the world.
const attrLastSessionEndedAt = "last_session_ended_at"

// attributeTable is data-model.md §3 type by type: §3.1–§3.4, §3.6 and §3.7,
// each with the common name. Item (§3.5) is a value inside inventory[], not a
// row here. An attribute its type does not list, and a type that is not here,
// is not typed: the model is open (C-02 v1.8 p. 2).
var attributeTable = map[string]map[string]AttributeSpec{
	TypeWorld: {
		attrName:         {Kind: KindText},
		AttrLawsVersion:  required(KindText),
		AttrWeather:      required(KindEnum),
		AttrTimeOfDay:    required(KindEnum),
		AttrDay:          required(KindInteger),
		AttrSeason:       optional(KindText),
		AttrEpoch:        optional(KindText),
		AttrBlueprintRef: required(KindText),
		AttrLocale:       required(KindText),
	},
	TypeRegion: {
		attrName:                  {Kind: KindText},
		AttrDescription:           required(KindText),
		AttrCanon:                 optional(KindOpen),
		AttrNPCIDs:                required(KindOpen),
		AttrRespawnTTL:            required(KindDuration),
		AttrPerceptionRadius:      optional(KindNumber),
		AttrEncounterChance:       required(KindNumber),
		AttrPlayersPresent:        required(KindOpen),
		AttrLastBackgroundEventAt: optional(KindTime),
		AttrBlueprintRef:          required(KindText),
	},
	TypePlayer: {
		attrName:               {Kind: KindText},
		AttrHP:                 required(KindInteger),
		AttrHPMax:              required(KindInteger),
		AttrAtk:                required(KindModifier),
		AttrDef:                required(KindModifier),
		AttrDmg:                required(KindModifier),
		AttrFlee:               optional(KindModifier),
		AttrStatus:             required(KindEnum),
		AttrPosition:           required(KindText),
		AttrScope:              required(KindOpen),
		AttrGroupID:            optional(KindRef),
		AttrEncounterID:        optional(KindRef),
		AttrInventory:          required(KindOpen),
		AttrActorKind:          required(KindEnum),
		attrLastSessionEndedAt: optional(KindTime),
	},
	TypeNPC: {
		attrName:          {Kind: KindText},
		AttrKind:          required(KindText),
		AttrRegionID:      required(KindRef),
		AttrHP:            required(KindInteger),
		AttrHPMax:         required(KindInteger),
		AttrAtk:           required(KindModifier),
		AttrDef:           required(KindModifier),
		AttrDmg:           required(KindModifier),
		AttrStatus:        required(KindEnum),
		AttrPosition:      required(KindText),
		AttrDiedAt:        optional(KindTime),
		AttrKilledBy:      optional(KindRef),
		AttrLoot:          required(KindOpen),
		AttrLootClaimedBy: optional(KindRef),
		AttrSpawnedBy:     optional(KindOpen),
	},
	TypeGroup: {
		attrName:        {Kind: KindText},
		AttrLeaderID:    {Kind: KindRef, Required: true, Nullable: true},
		AttrMembers:     required(KindOpen),
		AttrPosition:    required(KindText),
		AttrScope:       required(KindOpen),
		AttrEncounterID: optional(KindRef),
		AttrState:       required(KindEnum),
	},
	TypeEncounter: {
		attrName:            {Kind: KindText},
		AttrRegionID:        required(KindRef),
		AttrScope:           required(KindOpen),
		AttrParticipants:    required(KindOpen),
		AttrNPCs:            required(KindOpen),
		AttrState:           required(KindEnum),
		AttrResolution:      optional(KindEnum),
		AttrRoundSeq:        required(KindInteger),
		AttrTaskAgentID:     optional(KindRef),
		AttrOpenedByEventID: optional(KindRef),
		AttrClosedByEventID: optional(KindRef),
	},
}

// AttributeSpecOf is the row of data-model.md §3 for an attribute of a type;
// false means the attribute is not typed for that type.
func AttributeSpecOf(entityType, name string) (AttributeSpec, bool) {
	spec, ok := attributeTable[entityType][name]
	return spec, ok
}

// Holds says whether a value is of the kind of the attribute as the wire carries
// it: an int built in Go and the float64 read off the bus are the same whole
// number, and a time.Time is the RFC 3339 text it is written as.
func (s AttributeSpec) Holds(value any) bool {
	if s.Kind == KindOpen {
		return true
	}
	wire, ok := wireValue(value)
	if !ok {
		return false
	}
	if wire == nil {
		return s.Nullable
	}
	text, isText := wire.(string)
	number, isNumber := wire.(float64)
	whole := isNumber && number == math.Trunc(number)
	switch s.Kind {
	case KindText, KindRef, KindEnum:
		return isText
	case KindInteger:
		return whole
	case KindNumber:
		return isNumber
	case KindModifier:
		return isText || whole
	case KindTime:
		_, err := time.Parse(time.RFC3339, text)
		return isText && err == nil
	case KindDuration:
		_, err := time.ParseDuration(text)
		return isText && err == nil
	default:
		return false
	}
}

// wireValue is a value as a reader of the bus decodes it.
func wireValue(value any) (any, bool) {
	switch value.(type) {
	case nil, string, bool, float64, map[string]any, []any:
		return value, true
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

// Reasons a set of attributes cannot make an entity: the message of
// ErrInvalidAttribute. On the wire every one of them is invalid_op (C-02).
const (
	ReasonRequiredMissing = "required attribute is missing"
	ReasonWrongKind       = "value is not of the kind of the attribute"
)

// ErrInvalidAttribute is an attribute an entity of its type cannot be created
// with: a required one missing, or a value of another kind. State answers it
// with entity.update.rejected reason=invalid_op (state-and-mechanics.md §4.5,
// the paragraph on create).
type ErrInvalidAttribute struct {
	Type      string
	Attribute string
	Reason    string
}

func (e ErrInvalidAttribute) Error() string {
	return fmt.Sprintf("invalid attributes of %s: %s: %s", e.Type, e.Attribute, e.Reason)
}

// CheckAttributes checks the attributes of a create against the table of its
// type: every required attribute is there, and every typed one holds a value of
// its kind; attributes the table does not list are left alone. The problem
// reported is the first in the order of the names, so the answer does not
// depend on the order a map iterates in.
func CheckAttributes(entityType string, attrs map[string]any) error {
	table := attributeTable[entityType]
	names := make([]string, 0, len(table))
	for name := range table {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec := table[name]
		value, present := attrs[name]
		switch {
		case !present && spec.Required:
			return ErrInvalidAttribute{Type: entityType, Attribute: name, Reason: ReasonRequiredMissing}
		case present && !spec.Holds(value):
			return ErrInvalidAttribute{Type: entityType, Attribute: name, Reason: ReasonWrongKind}
		}
	}
	return nil
}
