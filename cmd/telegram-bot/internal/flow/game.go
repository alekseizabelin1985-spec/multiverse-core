package flow

import (
	"multiverse-core.io/cmd/telegram-bot/internal/commands"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/internal/gateway/api"
)

// game handles a command of a player with a character.
func (t *turn) game(p player) {
	cmd := t.cmd
	if cmd.Problem != commands.NoProblem {
		t.reply(cmd.Hint, nil)
		return
	}
	switch cmd.Type {
	case commands.Unknown:
		// Free text is not speech in MVP-1: US-008 answers it with the list of
		// commands and creates no action (A-1).
		t.reply(render.HelpText, nil)
	case commands.Status:
		t.status(p)
	case commands.Enter:
		if cmd.Target == "" {
			t.askRegion(p)
			return
		}
		t.act(p, cmd.Target, "")
	case commands.Attack:
		if cmd.Target == "" {
			t.attackWithoutTarget(p)
			return
		}
		t.act(p, cmd.Target, "")
	case commands.Say:
		t.act(p, "", cmd.Text)
	default:
		t.act(p, cmd.Target, "")
	}
}

func (t *turn) status(p player) {
	st, err := t.f.gw.Player(t.ctx, p.id)
	if err != nil {
		t.fail("status", err)
		return
	}
	t.replyWith(render.StatusReply(st))
}

// askRegion answers /enter without a region with the regions of the world of
// the character (component §10.3).
func (t *turn) askRegion(p player) {
	w, err := t.f.gw.Worlds(t.ctx)
	if err != nil {
		t.fail("worlds", err)
		return
	}
	for _, world := range w.Worlds {
		if world.WorldID == p.worldID && len(world.Regions) > 0 {
			t.reply(render.EnterPrompt, render.RegionsKeyboard(world.Regions))
			return
		}
	}
	t.reply(render.NoRegions, nil)
}

// attackWithoutTarget picks the only opponent standing, or asks which one
// (component §10.3). Without an opponent there is nothing to send.
func (t *turn) attackWithoutTarget(p player) {
	st, err := t.f.gw.Player(t.ctx, p.id)
	if err != nil {
		t.fail("status", err)
		return
	}
	var alive []api.NPCView
	if st.Encounter != nil {
		alive = render.AliveNPCs(st.Encounter.NPCs)
	}
	switch len(alive) {
	case 0:
		t.reply(render.ErrorText(api.CodeNotInEncounter, "", 0), render.BaseKeyboard())
	case 1:
		t.act(p, alive[0].NPCID, "")
	default:
		t.reply(render.AttackPrompt, render.TargetsKeyboard(alive))
	}
}

// act sends the command as an action with the action_key of this update: the
// same update handled twice is one action (ADR-006 p. 2).
func (t *turn) act(p player, target, text string) {
	req := api.ActionRequest{ActionKey: t.actionKey(), Type: string(t.cmd.Type)}
	if target != "" {
		req.Target = ptr(target)
	}
	if t.cmd.Type == commands.Say {
		req.Text = ptr(text)
	}
	if _, err := t.f.gw.Action(t.ctx, p.id, req); err != nil {
		t.fail(string(t.cmd.Type), err)
		return
	}
	t.reply(render.Accepted, nil)
}
