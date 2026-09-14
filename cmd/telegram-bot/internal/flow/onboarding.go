package flow

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
)

// start is /start: resolve, then the notice, the name or the character.
func (t *turn) start() {
	res, ok := t.resolve()
	if !ok {
		return
	}
	if res.LinkStatus == linkConsented && (res.CharacterStatus == characterAlive || res.CharacterStatus == characterCreating) && res.PlayerID != nil {
		t.f.dropDialog(t.chat)
		t.f.setPlayer(t.chat, *res.PlayerID, deref(res.WorldID), t.now)
		if res.NoticeDue && !t.reply(render.NoticeText, nil) {
			return
		}
		st, err := t.f.gw.Player(t.ctx, *res.PlayerID)
		if err != nil {
			t.fail("status", err)
			return
		}
		t.replyWith(render.StatusReply(st))
		return
	}
	t.route(res)
}

// help is /help. A player who has not consented gets the notice with the
// consent keyboard and nothing else: /help never gives the consent (Mi-5 of
// T-318). A player who has gets the commands and the notice again, and the
// resolve behind it refreshes last_seen_at.
func (t *turn) help() {
	res, ok := t.resolve()
	if !ok {
		return
	}
	if res.LinkStatus != linkConsented {
		t.showNotice()
		return
	}
	if res.CharacterStatus == characterAlive && res.PlayerID != nil {
		t.f.setPlayer(t.chat, *res.PlayerID, deref(res.WorldID), t.now)
	}
	if t.reply(render.HelpText, nil) {
		t.reply(render.NoticeText, nil)
	}
}

// idle handles a message of a chat outside the onboarding: a game command of a
// player with a character, or the way into the onboarding.
func (t *turn) idle() {
	p, ok := t.f.playerOf(t.chat, t.now)
	if t.u.Message.Text == render.ConsentButton {
		if ok {
			// A player with a character already consented: the press shows the
			// notice and keeps the chat where it is, so that the next game
			// command is not answered with the notice for 15 minutes (N-1 of
			// review #1 of T-311, decision of tech-lead#3).
			t.reply(render.NoticeText, nil)
			return
		}
		// A consent button without the notice just before it — after the TTL,
		// after a restart, typed by hand — shows the notice again (Р-3 A).
		t.showNotice()
		return
	}
	if !ok {
		res, resolved := t.resolve()
		if !resolved {
			return
		}
		if res.LinkStatus != linkConsented || res.PlayerID == nil ||
			(res.CharacterStatus != characterAlive && res.CharacterStatus != characterCreating) {
			t.route(res)
			return
		}
		if res.NoticeDue && !t.reply(render.NoticeText, nil) {
			return
		}
		t.f.setPlayer(t.chat, *res.PlayerID, deref(res.WorldID), t.now)
		p = player{id: *res.PlayerID, worldID: deref(res.WorldID)}
	}
	t.game(p)
}

// route leads a resolved chat without a character to its step: the notice
// when the link is not consented, the name prompt otherwise.
func (t *turn) route(res api.ResolveResponse) {
	if res.LinkStatus != linkConsented {
		t.showNotice()
		return
	}
	prompt := render.NamePrompt
	if res.CharacterStatus == characterDead {
		prompt = render.DeadPrompt
	}
	if t.reply(prompt, render.RemoveKeyboard()) {
		t.f.setDialog(t.chat, &dialog{step: AwaitingName}, t.now)
	}
}

func (t *turn) resolve() (api.ResolveResponse, bool) {
	res, err := t.f.gw.Resolve(t.ctx, Platform, t.ext)
	if err != nil {
		t.fail("resolve", err)
		return api.ResolveResponse{}, false
	}
	return res, true
}

// awaitingConsent waits for ConsentButton, exactly as the keyboard sends it.
// DeclineButton gets the short DeclineReply; anything else the notice again
// (Р-3 A, US-008, UC-001 E1). Only ConsentButton calls the gateway.
func (t *turn) awaitingConsent(d *dialog) {
	switch t.u.Message.Text {
	case render.ConsentButton:
		t.consent(d)
	case render.DeclineButton:
		if t.reply(render.DeclineReply, render.ConsentKeyboard()) {
			t.f.setDialog(t.chat, d, t.now)
		}
	default:
		t.showNotice()
	}
}

func (t *turn) consent(d *dialog) {
	_, err := t.f.gw.Consent(t.ctx, api.ConsentRequest{
		ExternalPlatform: Platform,
		ExternalID:       t.ext,
		NoticeShown:      true,
		Consent:          true,
		AgeConfirmed:     true,
		ShownAt:          d.noticeShownAt,
	})
	if err != nil {
		t.fail("consent", err)
		return
	}
	t.logStep("consent given", AwaitingConsent)
	t.f.dropDialog(t.chat)
	if t.ctx.Err() != nil {
		// Stopping between two calls: the next call would be lost with its
		// answer (N-6 of review #1 of T-311).
		return
	}
	// The consent may be given again by a player who already has a character:
	// the link tells which step comes next.
	t.start()
}

// awaitingName takes the name of the character. A command is not a name: the
// prompt is repeated.
func (t *turn) awaitingName(d *dialog) {
	if t.cmd.Slash {
		if t.reply(render.NamePrompt, nil) {
			t.f.setDialog(t.chat, d, t.now)
		}
		return
	}
	// The bot trims the text of the message; the rule itself is the one of
	// POST /v1/characters, so the bot and the gateway cannot disagree on a name
	// (api-contracts.md §1.3; acceptance of T-312).
	name := strings.TrimSpace(t.u.Message.Text)
	if !api.ValidCharacterName(name) {
		if t.reply(render.NameInvalid, nil) {
			t.f.setDialog(t.chat, d, t.now)
		}
		return
	}
	d.name = name
	if t.matchesProfile(name) {
		if t.reply(render.NameMatchesProfile, render.NameConfirmKeyboard()) {
			d.step = AwaitingNameConfirm
			t.f.setDialog(t.chat, d, t.now)
		}
		return
	}
	t.chooseWorld(d)
}

// matchesProfile compares the name with the username and the first name of
// the sender, ignoring case. The profile is read here and nowhere else, and
// neither kept nor logged (US-009).
func (t *turn) matchesProfile(name string) bool {
	from := t.u.Message.From
	for _, profile := range []string{from.Username, from.FirstName} {
		if p := strings.TrimSpace(profile); p != "" && strings.EqualFold(p, name) {
			return true
		}
	}
	return false
}

func (t *turn) awaitingNameConfirm(d *dialog) {
	switch t.u.Message.Text {
	case render.NameKeep:
		t.chooseWorld(d)
	case render.NameChange:
		if t.reply(render.NamePrompt, render.RemoveKeyboard()) {
			d.step, d.name = AwaitingName, ""
			t.f.setDialog(t.chat, d, t.now)
		}
	default:
		if t.reply(render.NameMatchesProfile, render.NameConfirmKeyboard()) {
			t.f.setDialog(t.chat, d, t.now)
		}
	}
}

// chooseWorld creates the character in the only world, or asks which one.
func (t *turn) chooseWorld(d *dialog) {
	w, err := t.f.gw.Worlds(t.ctx)
	if err != nil {
		t.fail("worlds", err)
		return
	}
	switch len(w.Worlds) {
	case 0:
		t.f.dropDialog(t.chat)
		t.reply(render.NoWorlds, render.StartKeyboard())
	case 1:
		t.create(d, w.Worlds[0])
	default:
		if t.reply(render.WorldPrompt, render.WorldsKeyboard(w.Worlds)) {
			d.step, d.worlds = AwaitingWorld, w.Worlds
			t.f.setDialog(t.chat, d, t.now)
		}
	}
}

func (t *turn) awaitingWorld(d *dialog) {
	text := t.u.Message.Text
	for _, w := range d.worlds {
		if text == w.Name || text == w.WorldID {
			t.create(d, w)
			return
		}
	}
	if t.reply(render.WorldPrompt, render.WorldsKeyboard(d.worlds)) {
		t.f.setDialog(t.chat, d, t.now)
	}
}

// create sends POST /v1/characters with the action_key of this update, so a
// repeat of the update gets the same character.
func (t *turn) create(d *dialog, world api.WorldSummary) {
	res, status, err := t.f.gw.CreateCharacter(t.ctx, api.CreateCharacterRequest{
		ExternalPlatform: Platform,
		ExternalID:       t.ext,
		WorldID:          world.WorldID,
		CharacterName:    d.name,
		ActionKey:        t.actionKey(),
	})
	if err != nil {
		t.createFailed(d, err)
		return
	}
	t.f.dropDialog(t.chat)
	t.f.setPlayer(t.chat, res.PlayerID, world.WorldID, t.now)
	t.logStep("character requested", Ready)
	if t.ctx.Err() != nil {
		return
	}
	if status == http.StatusAccepted || res.Character == nil {
		t.awaitCharacter(res.PlayerID, d.name, world)
		return
	}
	created := res.Created == nil || *res.Created
	t.replyWith(render.CharacterCreated(*res.Character, created, world.Regions))
}

func (t *turn) createFailed(d *dialog, err error) {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.fail("create", err)
		return
	}
	switch apiErr.Code {
	case api.CodeNameInvalid, api.CodeNameRequired:
		if t.reply(render.ErrorText(apiErr.Code, apiErr.Message, 0)+" "+render.NamePrompt, render.RemoveKeyboard()) {
			d.step, d.name, d.worlds = AwaitingName, "", nil
			t.f.setDialog(t.chat, d, t.now)
		}
	case api.CodeWorldNotFound:
		t.f.dropDialog(t.chat)
		t.reply(render.ErrorText(apiErr.Code, apiErr.Message, 0), render.StartKeyboard())
	default:
		t.fail("create", err)
	}
}

// awaitCharacter polls a character answered 202 creating (component §7.1).
func (t *turn) awaitCharacter(playerID, name string, world api.WorldSummary) {
	for range CreatingPolls {
		if !t.pause(CreatingPollInterval) {
			return
		}
		st, err := t.f.gw.Player(t.ctx, playerID)
		if err != nil {
			t.fail("status", err)
			return
		}
		if st.Status != characterCreating {
			t.replyWith(render.CharacterCreated(st, true, world.Regions))
			return
		}
	}
	t.reply(render.CharacterCreating(name), render.BaseKeyboard())
}

func (t *turn) pause(d time.Duration) bool {
	timer := t.f.timers.After(d)
	select {
	case <-t.ctx.Done():
		timer.Stop()
		return false
	case <-timer.C():
		return true
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
