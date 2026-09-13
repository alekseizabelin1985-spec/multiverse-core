package outbox

import (
	"cmp"
	"encoding/json"
	"fmt"
	"strings"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// The event types the outbox renders (C-02, C-05).
const (
	TypeCombatDecided = "combat.decided"
	TypeEntityUpdated = readmodel.TypeEntityUpdated
)

// Causes of entity.updated the player hears about (component §8.1).
const (
	CauseMove = "move"
	CauseRest = "rest"
	CauseLoot = "loot"
)

// Keys of Delivery.Data the outbox writes.
const (
	DataNarrativeEventID = "narrative_event_id"
	DataKind             = "kind"
	DataAbsence          = "absence"
	DataFilter           = "filter"
	DataEncounterID      = "encounter_id"
	DataTransition       = "transition"
)

// TransitionOpened is the transition of an encounter a world_event tells.
const TransitionOpened = "opened"

// Projection is what the texts read of the read model: the names of those they
// are about and where they stand.
type Projection interface {
	Character(id string) (readmodel.CharacterState, bool)
	NPC(id string) (readmodel.NPC, bool)
	Region(worldID, regionID string) (readmodel.Region, bool)
	Encounter(id string) (readmodel.Encounter, bool)
}

// The texts of the rules (component §7.2): Russian, plain text without markup
// (SEC-10), no model behind them (generated_by=rules).
const (
	textFled       = "Вы вырвались из боя."
	textNotFled    = "Бегство не удалось."
	textDied       = "Ваш персонаж погиб. /start — создать нового."
	textOutside    = "Вы вышли на опушку."
	textGroupOut   = "Группа вышла на опушку."
	textMoveFailed = "Переход не удался. Повторите команду."
	textRestFailed = "Отдых не удался. Повторите команду."
	textActFailed  = "Действие не выполнено. Повторите команду."
	unknownFoe     = "противник"
)

// Mechanics is the text of a result of the mechanics for the players: a
// combat.decided, or an entity.updated of a player or a group that moved, of a
// player who rested or took loot. ok is false for an event it has no text for.
// The text is the same for every recipient; data carries the ids and numbers
// behind it.
func Mechanics(ev eventbus.Event, st Projection) (text string, data map[string]any, ok bool) {
	switch ev.Type {
	case TypeCombatDecided:
		return combat(ev, st)
	case TypeEntityUpdated:
		return updated(ev, st)
	}
	return "", nil, false
}

// Died is the text of kind=system to a player whose character died.
func Died() string { return textDied }

// Refused is the text of kind=system to a player whose move or rest State
// refused (entity.update.rejected). It names neither the reason of State nor
// anything of the proposal.
func Refused(actionType string) string {
	switch actionType {
	case "enter", "leave":
		return textMoveFailed
	case "rest":
		return textRestFailed
	}
	return textActFailed
}

// EncounterOpened is the world_event of an encounter that opened: who comes out
// of the shadows and what the player can do about it.
func EncounterOpened(enc readmodel.Encounter, st Projection) (string, map[string]any) {
	names := make([]string, 0, len(enc.NPCIDs))
	commands := make([]string, 0, len(enc.NPCIDs)+1)
	for _, id := range enc.NPCIDs {
		names = append(names, npcName(st, id))
		commands = append(commands, "/attack "+id)
	}
	commands = append(commands, "/flee")
	verb := "выходит"
	if len(names) > 1 {
		verb = "выходят"
	}
	who := cmp.Or(strings.Join(names, ", "), unknownFoe)
	text := fmt.Sprintf("Из тени %s %s. Действия: %s", verb, who, strings.Join(commands, ", "))
	npcs := make([]any, 0, len(enc.NPCIDs))
	for _, id := range enc.NPCIDs {
		npcs = append(npcs, id)
	}
	return text, map[string]any{DataEncounterID: enc.ID, DataTransition: TransitionOpened, "npcs": npcs}
}

type named struct {
	Entity eventbus.EntityRef `json:"entity"`
	Name   string             `json:"name"`
}

type combatPayload struct {
	Encounter named  `json:"encounter"`
	Action    string `json:"action"`
	Attacker  named  `json:"attacker"`
	Defender  *named `json:"defender"`
	Outcome   struct {
		Hit        bool  `json:"hit"`
		Damage     int   `json:"damage"`
		Critical   bool  `json:"critical"`
		TargetDead bool  `json:"target_dead"`
		Success    *bool `json:"success"`
	} `json:"outcome"`
	HP *struct {
		After int `json:"defender_after"`
		Max   int `json:"defender_max"`
	} `json:"hp"`
	FreeAttack bool `json:"free_attack"`
}

func combat(ev eventbus.Event, st Projection) (string, map[string]any, bool) {
	var p combatPayload
	if decode(ev, &p) != nil || p.Action == "" {
		return "", nil, false
	}
	facts := map[string]any{"action": p.Action, "attacker": p.Attacker.Entity.ID, "hit": p.Outcome.Hit,
		"damage": p.Outcome.Damage, "critical": p.Outcome.Critical}
	if p.Defender != nil {
		facts["defender"] = p.Defender.Entity.ID
	}
	if p.HP != nil {
		facts["hp_after"], facts["hp_max"] = p.HP.After, p.HP.Max
	}
	if p.Outcome.TargetDead {
		facts["target_dead"] = true
	}
	data := map[string]any{"combat": facts}

	switch p.Action {
	case "flee":
		fled := p.Outcome.Success != nil && *p.Outcome.Success
		facts["success"] = fled
		switch {
		case fled:
			return textFled, data, true
		case p.FreeAttack:
			return fmt.Sprintf("Бегство не удалось; %s бьёт вслед.", foesOf(p.Encounter.Entity.ID, st)), data, true
		default:
			return textNotFled, data, true
		}
	case "attack":
		return strike("", p, st), data, true
	case "npc_attack":
		return strike(nameOf(p.Attacker, st)+" атакует — ", p, st), data, true
	case "free_attack":
		return strike(nameOf(p.Attacker, st)+" бьёт вслед — ", p, st), data, true
	}
	return "", nil, false
}

// strike is the text of one blow: its outcome, and the health the defender is
// left with when the decision says it.
func strike(lead string, p combatPayload, st Projection) string {
	outcome := "Промах."
	switch {
	case p.Outcome.Hit && p.Outcome.Critical:
		outcome = fmt.Sprintf("Критическое попадание! Урон %d (крит ×2).", p.Outcome.Damage)
	case p.Outcome.Hit:
		outcome = fmt.Sprintf("Попадание! Урон %d.", p.Outcome.Damage)
	}
	text := outcome
	if lead != "" {
		text = lead + strings.ToLower(outcome[:len("П")]) + outcome[len("П"):]
	}
	if p.Defender != nil && p.HP != nil {
		text += fmt.Sprintf(" %s: %d/%d.", nameOf(*p.Defender, st), p.HP.After, p.HP.Max)
	}
	return text
}

type updatedPayload struct {
	Entity  named  `json:"entity"`
	Cause   string `json:"cause"`
	Changed []struct {
		Path string          `json:"path"`
		New  json.RawMessage `json:"new"`
	} `json:"changed"`
}

func updated(ev eventbus.Event, st Projection) (string, map[string]any, bool) {
	var p updatedPayload
	if decode(ev, &p) != nil {
		return "", nil, false
	}
	world := ""
	if ev.World != nil {
		world = ev.World.Entity.ID
	}
	switch {
	case p.Cause == CauseMove && (p.Entity.Entity.Type == entity.TypePlayer || p.Entity.Entity.Type == entity.TypeGroup):
		var to string
		for _, c := range p.Changed {
			if c.Path == entity.AttrPosition && json.Unmarshal(c.New, &to) == nil && to != "" {
				return moved(p.Entity.Entity.Type == entity.TypeGroup, world, to, st), map[string]any{"position": to}, true
			}
		}
	case p.Cause == CauseRest && p.Entity.Entity.Type == entity.TypePlayer:
		ch, ok := st.Character(p.Entity.Entity.ID)
		if !ok {
			return "", nil, false
		}
		return fmt.Sprintf("Вы отдохнули: HP %d/%d.", ch.HP, ch.HPMax), map[string]any{"hp": ch.HP, "hp_max": ch.HPMax}, true
	case p.Cause == CauseLoot && p.Entity.Entity.Type == entity.TypePlayer:
		for _, c := range p.Changed {
			if !strings.HasPrefix(c.Path, entity.AttrInventory+"[") {
				continue
			}
			var item entity.Item
			if json.Unmarshal(c.New, &item) == nil && (item.Name != "" || item.ItemID != "") {
				return "Трофей: " + cmp.Or(item.Name, item.ItemID) + ".", map[string]any{"item_id": item.ItemID}, true
			}
		}
	}
	return "", nil, false
}

func moved(group bool, world, to string, st Projection) string {
	if strings.HasPrefix(to, "outside:") {
		if group {
			return textGroupOut
		}
		return textOutside
	}
	name := to
	if r, ok := st.Region(world, to); ok && r.Name != "" {
		name = r.Name
	}
	if group {
		return fmt.Sprintf("Группа вошла в %s.", name)
	}
	return fmt.Sprintf("Вы вошли в %s.", name)
}

// nameOf is the name a payload gives, or the one the projection knows, or the
// id.
func nameOf(ref named, st Projection) string {
	if ref.Name != "" {
		return ref.Name
	}
	switch ref.Entity.Type {
	case entity.TypeNPC:
		return npcName(st, ref.Entity.ID)
	case entity.TypePlayer:
		if ch, ok := st.Character(ref.Entity.ID); ok && ch.Name != "" {
			return ch.Name
		}
	}
	return ref.Entity.ID
}

func npcName(st Projection, id string) string {
	if n, ok := st.NPC(id); ok && n.Name != "" {
		return n.Name
	}
	return id
}

// foesOf names the opponents of an encounter; a flight names nobody it flees
// from, and its text still should.
func foesOf(encounterID string, st Projection) string {
	enc, ok := st.Encounter(encounterID)
	if !ok || len(enc.NPCIDs) == 0 {
		return unknownFoe
	}
	names := make([]string, 0, len(enc.NPCIDs))
	for _, id := range enc.NPCIDs {
		if n, ok := st.NPC(id); ok && entity.IsTerminalStatus(n.Status) {
			continue
		}
		names = append(names, npcName(st, id))
	}
	return cmp.Or(strings.Join(names, ", "), unknownFoe)
}

func decode(ev eventbus.Event, dst any) error {
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}
