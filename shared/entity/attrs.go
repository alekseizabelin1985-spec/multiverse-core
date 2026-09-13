package entity

import (
	"bytes"
	"encoding/json"
	"fmt"
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
// their positions, checked by inv-10.
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
