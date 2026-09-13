package render

import (
	"fmt"
	"strings"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
)

// Reply is one message of the bot: plain text and the keyboard to show with
// it; a nil Keyboard leaves the keyboard of the chat as it is.
type Reply struct {
	Text     string
	Keyboard *sender.Keyboard
}

// Texts of the onboarding (component §7.1, §10.3).
const (
	NamePrompt = "Введите имя персонажа: от 2 до 32 символов — буквы, цифры, пробел, дефис. Имя будет видно другим игрокам."
	// NameInvalid answers a name the rule of NamePrompt refuses.
	NameInvalid = "Такое имя не подходит: нужно от 2 до 32 символов — буквы, цифры, пробел, дефис. Введите другое."
	// DeadPrompt starts a new character after the death of the old one
	// (UC-002 A2).
	DeadPrompt = "Ваш прежний персонаж погиб. " + NamePrompt
	// NameMatchesProfile warns that the name repeats the Telegram profile of
	// the player (US-009, component §10.3).
	NameMatchesProfile = "Это имя совпадает с вашим ником в Telegram и будет видно другим игрокам. Оставить?"
	// NameKeep and NameChange are the buttons of NameMatchesProfile; the flow
	// compares an incoming message with them exactly.
	NameKeep   = "Да"
	NameChange = "Ввести другое"
	// WorldPrompt asks for a world when there is more than one.
	WorldPrompt = "Выберите мир."
	// NoWorlds answers when the gateway lists no world.
	NoWorlds = "Миров пока нет. Попробуйте позже командой /start."
)

// Texts of the game commands.
const (
	// Accepted confirms an action the gateway accepted; its results come as
	// deliveries (component §7.2, NFR-003).
	Accepted = "Принято."
	// EnterPrompt asks for a region of /enter without a target.
	EnterPrompt = "Куда идти? Выберите регион."
	// NoRegions answers /enter without a target when the world lists none.
	NoRegions = "Регионов пока нет."
	// AttackPrompt asks for a target of /attack without one.
	AttackPrompt = "Кого атаковать? Выберите цель."
	// Unavailable answers a failure of the gateway or the network (component
	// §10.5).
	Unavailable = "Сервис недоступен, повторите позже."
)

// Texts of /forget (component §7.5, US-009, C-08 v1.5).
const (
	ForgetQuestion = "Удалить вашу связку с игрой? Персонаж останется в мире без владельца, вернуть его будет нельзя. Что удаляется и что остаётся, написано в сообщении /help. Чтобы подтвердить, отправьте /forget confirm"
	ForgetDone     = "Связка удалена. /start — начать заново"
	ForgetNothing  = "Нечего удалять: связки с этим аккаунтом нет."
)

// ForgetIncomplete answers 503 forget_incomplete: the wipe is not confirmed,
// so the text never says the link or the data are deleted (C-08 v1.5, I-2 of
// T-318). retryAfter is the Retry-After of the answer, 0 when there was none.
func ForgetIncomplete(retryAfter time.Duration) string {
	const head = "Команда /forget ещё не завершена. "
	if secs := wholeSeconds(retryAfter); secs > 0 {
		return head + fmt.Sprintf("Повторите /forget confirm через %d с.", secs)
	}
	return head + "Повторите /forget confirm чуть позже."
}

// CharacterCreated answers a character created or found alive; the regions of
// its world are offered as the next step (component §7.1).
func CharacterCreated(st api.CharacterState, created bool, regions []api.RegionSummary) Reply {
	head := fmt.Sprintf("Персонаж «%s» создан.", st.Name)
	if !created {
		head = fmt.Sprintf("У вас уже есть персонаж «%s».", st.Name)
	}
	lines := []string{head, Status(st)}
	if len(regions) > 0 {
		lines = append(lines, "Команда: /enter "+regions[0].RegionID)
		return Reply{Text: strings.Join(lines, "\n"), Keyboard: RegionsKeyboard(regions)}
	}
	return Reply{Text: strings.Join(lines, "\n"), Keyboard: BaseKeyboard()}
}

// CharacterCreating answers a character whose fact has not arrived yet.
func CharacterCreating(name string) string {
	return fmt.Sprintf("Персонаж «%s» создаётся. Проверьте позже командой /status.", name)
}

var statusNames = map[string]string{
	"alive":     "жив",
	"dead":      "погиб",
	"creating":  "создаётся",
	"abandoned": "без владельца",
}

// Status is the text of /status: the character, where it is, the encounter
// and the inventory.
func Status(st api.CharacterState) string {
	state := statusNames[st.Status]
	if state == "" {
		state = st.Status
	}
	head := fmt.Sprintf("«%s»: %s", st.Name, state)
	if st.HP != nil && st.HPMax != nil {
		head += fmt.Sprintf(", HP %d/%d", *st.HP, *st.HPMax)
	}
	lines := []string{head}
	if st.Position != nil {
		lines = append(lines, "Где: "+placeName(*st.Position))
	}
	if st.Encounter != nil && len(st.Encounter.NPCs) > 0 {
		npcs := make([]string, 0, len(st.Encounter.NPCs))
		for _, n := range st.Encounter.NPCs {
			if n.Status != "alive" {
				npcs = append(npcs, fmt.Sprintf("%s (повержен)", n.Name))
				continue
			}
			npcs = append(npcs, fmt.Sprintf("%s %d/%d — /attack %s", n.Name, n.HP, n.HPMax, n.NPCID))
		}
		lines = append(lines, "Бой: "+strings.Join(npcs, "; "))
	}
	if len(st.Inventory) > 0 {
		items := make([]string, 0, len(st.Inventory))
		for _, it := range st.Inventory {
			items = append(items, it.Name)
		}
		lines = append(lines, "Инвентарь: "+strings.Join(items, ", "))
	}
	return strings.Join(lines, "\n")
}

// StatusReply is Status with the keyboard of the situation: the combat
// keyboard in an encounter with a living opponent, the base one otherwise.
func StatusReply(st api.CharacterState) Reply {
	kb := BaseKeyboard()
	if st.Encounter != nil && len(AliveNPCs(st.Encounter.NPCs)) > 0 {
		kb = CombatKeyboard(st.Encounter.NPCs)
	}
	return Reply{Text: Status(st), Keyboard: kb}
}

// AliveNPCs returns the opponents still standing.
func AliveNPCs(npcs []api.NPCView) []api.NPCView {
	var out []api.NPCView
	for _, n := range npcs {
		if n.Status == "alive" {
			out = append(out, n)
		}
	}
	return out
}

func placeName(p api.PositionRef) string {
	if p.Name != "" {
		return p.Name
	}
	return p.ID
}

func wholeSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + time.Second - 1) / time.Second)
}
