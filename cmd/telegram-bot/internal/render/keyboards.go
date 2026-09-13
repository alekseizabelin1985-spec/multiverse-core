package render

import (
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
)

// Keyboards of the bot (component §10.3). A button is plain text the player
// sends back by pressing it; a game button is the command itself, so that the
// parser reads a press the same way as a typed command.

// ConsentKeyboard offers the consent and the refusal of the notice.
func ConsentKeyboard() *sender.Keyboard {
	return &sender.Keyboard{Rows: [][]string{{ConsentButton, DeclineButton}}}
}

// NameConfirmKeyboard answers NameMatchesProfile.
func NameConfirmKeyboard() *sender.Keyboard {
	return &sender.Keyboard{Rows: [][]string{{NameKeep, NameChange}}}
}

// BaseKeyboard is the keyboard outside an encounter.
func BaseKeyboard() *sender.Keyboard {
	return &sender.Keyboard{Rows: [][]string{{"/look", "/status", "/rest"}}}
}

// CombatKeyboard is the keyboard of an encounter: an attack on each opponent
// still standing, then flee and defend. Without known opponents — a delivery
// of a world event names none — the attack button has no target, and the flow
// picks it or asks.
func CombatKeyboard(npcs []api.NPCView) *sender.Keyboard {
	var attacks []string
	for _, n := range AliveNPCs(npcs) {
		attacks = append(attacks, "/attack "+n.NPCID)
	}
	if len(attacks) == 0 {
		attacks = []string{"/attack"}
	}
	return &sender.Keyboard{Rows: [][]string{attacks, {"/flee", "/defend"}}}
}

// TargetsKeyboard offers one attack per opponent still standing.
func TargetsKeyboard(npcs []api.NPCView) *sender.Keyboard {
	kb := &sender.Keyboard{}
	for _, n := range AliveNPCs(npcs) {
		kb.Rows = append(kb.Rows, []string{"/attack " + n.NPCID})
	}
	return kb
}

// RegionsKeyboard offers one /enter per region of the world.
func RegionsKeyboard(regions []api.RegionSummary) *sender.Keyboard {
	kb := &sender.Keyboard{}
	for _, r := range regions {
		kb.Rows = append(kb.Rows, []string{"/enter " + r.RegionID})
	}
	return kb
}

// WorldsKeyboard offers the worlds by name; the flow takes a press of a name
// or of a world id.
func WorldsKeyboard(worlds []api.WorldSummary) *sender.Keyboard {
	kb := &sender.Keyboard{}
	for _, w := range worlds {
		label := w.Name
		if label == "" {
			label = w.WorldID
		}
		kb.Rows = append(kb.Rows, []string{label})
	}
	return kb
}

// StartKeyboard is the keyboard of a system message: begin again.
func StartKeyboard() *sender.Keyboard {
	return &sender.Keyboard{Rows: [][]string{{"/start"}}}
}

// RemoveKeyboard hides the keyboard shown before, such as the consent one
// once the consent is given.
func RemoveKeyboard() *sender.Keyboard {
	return &sender.Keyboard{Remove: true}
}
