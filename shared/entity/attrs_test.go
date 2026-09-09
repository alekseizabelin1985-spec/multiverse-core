package entity_test

import (
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

func TestCharacterAttributes(t *testing.T) {
	e := player(t)

	if hp, ok := e.HP(); !ok || hp != 10 {
		t.Fatalf("HP = %d, %v; want 10", hp, ok)
	}
	if hpMax, ok := e.HPMax(); !ok || hpMax != 10 {
		t.Fatalf("HPMax = %d, %v; want 10", hpMax, ok)
	}
	if atk, ok := e.Atk(); !ok || atk != 2 {
		t.Fatalf("Atk = %d, %v; want 2", atk, ok)
	}
	if def, ok := e.Def(); !ok || def != 12 {
		t.Fatalf("Def = %d, %v; want 12", def, ok)
	}
	if dmg, ok := e.Dmg(); !ok || dmg != "d6" {
		t.Fatalf("Dmg = %q, %v; want d6", dmg, ok)
	}
	if flee, ok := e.Flee(); !ok || flee != "+2" {
		t.Fatalf("Flee = %q, %v; want +2", flee, ok)
	}
	if status, ok := e.Status(); !ok || status != entity.StatusAlive {
		t.Fatalf("Status = %q, %v; want alive", status, ok)
	}
	if position, ok := e.Position(); !ok || position != "outside:dark-forest-world" {
		t.Fatalf("Position = %q, %v", position, ok)
	}
	if kind, ok := e.ActorKind(); !ok || kind != entity.ActorKindCI {
		t.Fatalf("ActorKind = %q, %v; want ci", kind, ok)
	}
}

func TestAttributesThatAreNotThere(t *testing.T) {
	empty := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", nil, proposedAt)

	if _, ok := empty.HP(); ok {
		t.Fatal("HP reports a value on an entity without one")
	}
	if _, ok := empty.Status(); ok {
		t.Fatal("Status reports a value on an entity without one")
	}
	if _, ok := empty.Scope(); ok {
		t.Fatal("Scope reports a value on an entity without one")
	}
	if _, ok := empty.RespawnTTL(); ok {
		t.Fatal("RespawnTTL reports a value on an entity without one")
	}
	if _, ok := empty.DiedAt(); ok {
		t.Fatal("DiedAt reports a value on an entity without one")
	}
	if _, ok := empty.NPCIDs(); ok {
		t.Fatal("NPCIDs reports a value on an entity without one")
	}
	if empty.HasAttr(entity.AttrHP) {
		t.Fatal("HasAttr(hp) = true on an entity without one")
	}
}

func TestAttributesOfTheWrongShape(t *testing.T) {
	e := player(t)
	e.Attributes[entity.AttrHP] = "quite a lot"
	e.Attributes[entity.AttrStatus] = 7
	e.Attributes[entity.AttrScope] = 42
	e.Attributes[entity.AttrNPCIDs] = []any{"npc-1", 7}
	e.Attributes[entity.AttrRespawnTTL] = "a day or so"
	e.Attributes[entity.AttrDiedAt] = "yesterday"

	if _, ok := e.HP(); ok {
		t.Fatal("HP read a string as a number")
	}
	if _, ok := e.Status(); ok {
		t.Fatal("Status read a number as a string")
	}
	if _, ok := e.Scope(); ok {
		t.Fatal("Scope read a number as a scope")
	}
	if _, ok := e.NPCIDs(); ok {
		t.Fatal("NPCIDs read a list with a number in it")
	}
	if _, ok := e.RespawnTTL(); ok {
		t.Fatal("RespawnTTL read a duration out of prose")
	}
	if _, ok := e.DiedAt(); ok {
		t.Fatal("DiedAt read a timestamp out of prose")
	}
}

func TestScopeReadsBothShapes(t *testing.T) {
	e := player(t)
	want := eventbus.ScopeRef{ID: "player-A", Type: "solo"}
	if scope, ok := e.Scope(); !ok || scope != want {
		t.Fatalf("Scope = %+v, %v; want %+v from the solo:{id} shorthand", scope, ok, want)
	}

	e.Attributes[entity.AttrScope] = map[string]any{"id": "group-1", "type": "group"}
	want = eventbus.ScopeRef{ID: "group-1", Type: "group"}
	if scope, ok := e.Scope(); !ok || scope != want {
		t.Fatalf("Scope = %+v, %v; want %+v from the object shape", scope, ok, want)
	}

	e.Attributes[entity.AttrScope] = "group-1"
	if _, ok := e.Scope(); ok {
		t.Fatal("Scope accepted a string with no type in it")
	}
}

func TestInventoryDecodes(t *testing.T) {
	e := player(t)
	e.Attributes[entity.AttrInventory] = []any{
		map[string]any{
			"item_id":     "item-1",
			"kind":        "wolf-pelt",
			"name":        "волчья шкура",
			"source":      map[string]any{"entity": map[string]any{"id": "wolf-alpha", "type": "npc"}, "event_id": "ev-combat-7"},
			"acquired_at": "2026-09-09T10:15:00Z",
		},
	}

	items, err := e.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Inventory = %+v, want one item", items)
	}
	item := items[0]
	if item.ItemID != "item-1" || item.Kind != "wolf-pelt" || item.Name != "волчья шкура" {
		t.Fatalf("item = %+v", item)
	}
	if item.Source.Entity.ID != "wolf-alpha" || item.Source.EventID != "ev-combat-7" {
		t.Fatalf("item.Source = %+v, want the NPC and the combat that produced it", item.Source)
	}
	if !item.AcquiredAt.Equal(proposedAt) {
		t.Fatalf("item.AcquiredAt = %v, want %v", item.AcquiredAt, proposedAt)
	}

	e.Attributes[entity.AttrInventory] = "a sack"
	if _, err := e.Inventory(); err == nil {
		t.Fatal("Inventory decoded a string as a list of items")
	}
}

func TestWorldAndRegionAttributes(t *testing.T) {
	world := entity.New(entity.Ref{ID: "dark-forest-world", Type: entity.TypeWorld}, "", "Тёмный лес", map[string]any{
		entity.AttrLawsVersion:  "v1",
		entity.AttrWeather:      "cloudy",
		entity.AttrTimeOfDay:    "dusk",
		entity.AttrDay:          float64(3),
		entity.AttrSeason:       "autumn",
		entity.AttrLocale:       "ru",
		entity.AttrBlueprintRef: "world-dark-forest@1.0",
	}, proposedAt)

	if v, ok := world.LawsVersion(); !ok || v != "v1" {
		t.Fatalf("LawsVersion = %q, %v", v, ok)
	}
	if v, ok := world.Weather(); !ok || v != "cloudy" {
		t.Fatalf("Weather = %q, %v", v, ok)
	}
	if v, ok := world.TimeOfDay(); !ok || v != "dusk" {
		t.Fatalf("TimeOfDay = %q, %v", v, ok)
	}
	if v, ok := world.Day(); !ok || v != 3 {
		t.Fatalf("Day = %d, %v", v, ok)
	}
	if v, ok := world.Season(); !ok || v != "autumn" {
		t.Fatalf("Season = %q, %v", v, ok)
	}
	if v, ok := world.Locale(); !ok || v != "ru" {
		t.Fatalf("Locale = %q, %v", v, ok)
	}
	if v, ok := world.BlueprintRef(); !ok || v != "world-dark-forest@1.0" {
		t.Fatalf("BlueprintRef = %q, %v", v, ok)
	}

	region := entity.New(entity.Ref{ID: "dark-forest-01", Type: entity.TypeRegion}, "dark-forest-world", "Тёмный лес", map[string]any{
		entity.AttrDescription:      "Ели смыкаются над тропой.",
		entity.AttrNPCIDs:           []any{"wolf-alpha"},
		entity.AttrRespawnTTL:       "24h",
		entity.AttrPerceptionRadius: float64(50),
		entity.AttrPlayersPresent:   []any{"player-A", "player-B"},
		entity.AttrEncounterChance:  0.35,
		entity.AttrCanon: []any{
			map[string]any{"fact": "Волки не выходят к опушке днём.", "source": "authored", "version": "1"},
		},
	}, proposedAt)

	if v, ok := region.Description(); !ok || v == "" {
		t.Fatalf("Description = %q, %v", v, ok)
	}
	if ids, ok := region.NPCIDs(); !ok || len(ids) != 1 || ids[0] != "wolf-alpha" {
		t.Fatalf("NPCIDs = %v, %v", ids, ok)
	}
	if ttl, ok := region.RespawnTTL(); !ok || ttl != 24*time.Hour {
		t.Fatalf("RespawnTTL = %v, %v; want 24h", ttl, ok)
	}
	if radius, ok := region.PerceptionRadius(); !ok || radius != 50 {
		t.Fatalf("PerceptionRadius = %v, %v", radius, ok)
	}
	if present, ok := region.PlayersPresent(); !ok || len(present) != 2 {
		t.Fatalf("PlayersPresent = %v, %v", present, ok)
	}
	if chance, ok := region.EncounterChance(); !ok || chance != 0.35 {
		t.Fatalf("EncounterChance = %v, %v", chance, ok)
	}
	canon, err := region.Canon()
	if err != nil || len(canon) != 1 || canon[0].Source != "authored" {
		t.Fatalf("Canon = %+v, %v", canon, err)
	}
}

func TestNPCAttributes(t *testing.T) {
	wolf := entity.New(entity.Ref{ID: "wolf-alpha", Type: entity.TypeNPC}, "dark-forest-world", "Вожак", map[string]any{
		entity.AttrKind:     "wolf",
		entity.AttrRegionID: "dark-forest-01",
		entity.AttrStatus:   entity.StatusDead,
		entity.AttrDiedAt:   "2026-09-09T10:15:00Z",
		entity.AttrKilledBy: "player-A",
		entity.AttrLoot:     []any{map[string]any{"item_kind": "wolf-pelt", "name": "волчья шкура"}},
	}, proposedAt)

	if kind, ok := wolf.Kind(); !ok || kind != "wolf" {
		t.Fatalf("Kind = %q, %v", kind, ok)
	}
	if region, ok := wolf.RegionID(); !ok || region != "dark-forest-01" {
		t.Fatalf("RegionID = %q, %v", region, ok)
	}
	if diedAt, ok := wolf.DiedAt(); !ok || !diedAt.Equal(proposedAt) {
		t.Fatalf("DiedAt = %v, %v", diedAt, ok)
	}
	if killer, ok := wolf.KilledBy(); !ok || killer != "player-A" {
		t.Fatalf("KilledBy = %q, %v", killer, ok)
	}
	loot, err := wolf.Loot()
	if err != nil || len(loot) != 1 || loot[0].ItemKind != "wolf-pelt" {
		t.Fatalf("Loot = %+v, %v", loot, err)
	}
	if !wolf.IsTerminal() {
		t.Fatal("a dead wolf is not terminal")
	}
}

func TestGroupAttributes(t *testing.T) {
	group := entity.New(entity.Ref{ID: "group-1", Type: entity.TypeGroup}, "dark-forest-world", "", map[string]any{
		entity.AttrLeaderID: "player-A",
		entity.AttrState:    entity.GroupStateActive,
		entity.AttrPosition: "dark-forest-01",
		entity.AttrMembers: []any{
			map[string]any{
				"player_id": "player-A", "joined_at": "2026-09-09T10:15:00Z",
				"participation": entity.ParticipationActive, "missed_rounds": float64(0),
			},
			map[string]any{
				"player_id": "player-B", "joined_at": "2026-09-09T10:16:00Z",
				"participation": entity.ParticipationIdle, "missed_rounds": float64(2),
			},
		},
	}, proposedAt)

	leader, ok := group.LeaderID()
	if !ok || leader != "player-A" {
		t.Fatalf("LeaderID = %q, %v", leader, ok)
	}
	members, err := group.Members()
	if err != nil || len(members) != 2 {
		t.Fatalf("Members = %+v, %v", members, err)
	}
	if members[1].Participation != entity.ParticipationIdle || members[1].MissedRounds != 2 {
		t.Fatalf("members[1] = %+v", members[1])
	}
	if !members[0].JoinedAt.Equal(proposedAt) {
		t.Fatalf("members[0].JoinedAt = %v, want %v", members[0].JoinedAt, proposedAt)
	}
	if state, ok := group.State(); !ok || state != entity.GroupStateActive {
		t.Fatalf("State = %q, %v", state, ok)
	}

	// A group whose living members are gone keeps going without a leader
	// (BR-13 v0.4).
	group.Attributes[entity.AttrLeaderID] = nil
	if _, ok := group.LeaderID(); ok {
		t.Fatal("LeaderID reports a leader for a null leader_id")
	}
}

func TestEncounterAttributes(t *testing.T) {
	encounter := entity.New(entity.Ref{ID: "enc-1", Type: entity.TypeEncounter}, "dark-forest-world", "", map[string]any{
		entity.AttrRegionID:    "dark-forest-01",
		entity.AttrState:       entity.EncounterStateResolved,
		entity.AttrResolution:  entity.ResolutionNPCDead,
		entity.AttrRoundSeq:    float64(3),
		entity.AttrTaskAgentID: "agent-encounter-1",
		entity.AttrParticipants: []any{
			map[string]any{"player_id": "player-A", "state": "in_combat", "damage_dealt": float64(7), "last_hit_at": "2026-09-09T10:15:00Z"},
		},
		entity.AttrNPCs: []any{
			map[string]any{"npc_id": "wolf-alpha", "last_damager": "player-A"},
		},
	}, proposedAt)

	participants, err := encounter.Participants()
	if err != nil || len(participants) != 1 || participants[0].DamageDealt != 7 {
		t.Fatalf("Participants = %+v, %v", participants, err)
	}
	if participants[0].LastHitAt == nil || !participants[0].LastHitAt.Equal(proposedAt) {
		t.Fatalf("participants[0].LastHitAt = %v", participants[0].LastHitAt)
	}
	npcs, err := encounter.NPCs()
	if err != nil || len(npcs) != 1 || npcs[0].LastDamager != "player-A" {
		t.Fatalf("NPCs = %+v, %v", npcs, err)
	}
	if resolution, ok := encounter.Resolution(); !ok || resolution != entity.ResolutionNPCDead {
		t.Fatalf("Resolution = %q, %v", resolution, ok)
	}
	if seq, ok := encounter.RoundSeq(); !ok || seq != 3 {
		t.Fatalf("RoundSeq = %d, %v", seq, ok)
	}
	if agent, ok := encounter.TaskAgentID(); !ok || agent != "agent-encounter-1" {
		t.Fatalf("TaskAgentID = %q, %v", agent, ok)
	}
}

func TestAttrIntReadsTheModifierForm(t *testing.T) {
	e := player(t)
	e.Attributes[entity.AttrAtk] = "+2"
	e.Attributes[entity.AttrDef] = "12"
	e.Attributes["penalty"] = "-3"

	if atk, ok := e.Atk(); !ok || atk != 2 {
		t.Fatalf("Atk = %d, %v; want 2 from the +2 the rules write", atk, ok)
	}
	if def, ok := e.Def(); !ok || def != 12 {
		t.Fatalf("Def = %d, %v", def, ok)
	}
	if penalty, ok := e.AttrInt("penalty"); !ok || penalty != -3 {
		t.Fatalf("AttrInt(penalty) = %d, %v", penalty, ok)
	}
	if _, ok := e.AttrInt(entity.AttrDmg); ok {
		t.Fatal("AttrInt read a dice formula as a number")
	}
}

func TestGenericAttributeAccess(t *testing.T) {
	e := player(t)
	e.Attributes["banner"] = map[string]any{"colour": "green", "torn": true, "weight": 1.5}

	if colour, ok := e.AttrString("banner.colour"); !ok || colour != "green" {
		t.Fatalf("AttrString(banner.colour) = %q, %v", colour, ok)
	}
	if torn, ok := e.AttrBool("banner.torn"); !ok || !torn {
		t.Fatalf("AttrBool(banner.torn) = %v, %v", torn, ok)
	}
	if weight, ok := e.AttrFloat("banner.weight"); !ok || weight != 1.5 {
		t.Fatalf("AttrFloat(banner.weight) = %v, %v", weight, ok)
	}
	if raw, ok := e.Attr("banner"); !ok || raw == nil {
		t.Fatalf("Attr(banner) = %v, %v", raw, ok)
	}
	if !e.HasAttr("banner.colour") {
		t.Fatal("HasAttr(banner.colour) = false")
	}

	var banner struct {
		Colour string `json:"colour"`
	}
	found, err := e.DecodeAttr("banner", &banner)
	if err != nil || !found || banner.Colour != "green" {
		t.Fatalf("DecodeAttr(banner) = %+v, %v, %v", banner, found, err)
	}
	if found, err := e.DecodeAttr("no-such-attribute", &banner); found || err != nil {
		t.Fatalf("DecodeAttr of an absent attribute = %v, %v; want false and no error", found, err)
	}
}

// Mi-8. The attributes of data-model.md §3 that had a name in the model and no
// getter here: the epoch of a world (§3.1), what spawned an NPC (§3.4) and the
// two events that bracket an encounter (§3.7).
func TestAttributesThatOnlyHadANameBefore(t *testing.T) {
	world := entity.New(entity.Ref{ID: "dark-forest-world", Type: entity.TypeWorld}, "", "", map[string]any{
		entity.AttrEpoch: "the long winter",
	}, proposedAt)
	if epoch, ok := world.Epoch(); !ok || epoch != "the long winter" {
		t.Fatalf("Epoch = %q, %v", epoch, ok)
	}

	wolf := entity.New(entity.Ref{ID: "wolf-alpha", Type: entity.TypeNPC}, "dark-forest-world", "", map[string]any{
		entity.AttrSpawnedBy: map[string]any{"agent": "region-gm", "tick_event_id": "ev-tick-7"},
	}, proposedAt)
	spawned, err := wolf.SpawnedBy()
	if err != nil || spawned.Agent != "region-gm" || spawned.TickEventID != "ev-tick-7" {
		t.Fatalf("SpawnedBy = %+v, %v", spawned, err)
	}

	encounter := entity.New(entity.Ref{ID: "enc-1", Type: entity.TypeEncounter}, "dark-forest-world", "", map[string]any{
		entity.AttrOpenedByEventID: "ev-open-1",
		entity.AttrClosedByEventID: "ev-close-1",
	}, proposedAt)
	if opened, ok := encounter.OpenedByEventID(); !ok || opened != "ev-open-1" {
		t.Fatalf("OpenedByEventID = %q, %v", opened, ok)
	}
	if closed, ok := encounter.ClosedByEventID(); !ok || closed != "ev-close-1" {
		t.Fatalf("ClosedByEventID = %q, %v", closed, ok)
	}

	// An entity that carries none of them says so rather than inventing one.
	empty := entity.New(entity.Ref{ID: "x", Type: entity.TypeNPC}, "w", "", nil, proposedAt)
	if _, ok := empty.Epoch(); ok {
		t.Fatal("Epoch reports a value on an entity without one")
	}
	if _, ok := empty.OpenedByEventID(); ok {
		t.Fatal("OpenedByEventID reports a value on an entity without one")
	}
	if _, ok := empty.ClosedByEventID(); ok {
		t.Fatal("ClosedByEventID reports a value on an entity without one")
	}
	spawned, err = empty.SpawnedBy()
	if err != nil || spawned != (entity.SpawnSource{}) {
		t.Fatalf("SpawnedBy = %+v, %v; want the zero value for an authored NPC", spawned, err)
	}
}
