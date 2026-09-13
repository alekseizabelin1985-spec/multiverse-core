package flow

import (
	"errors"
	"log/slog"

	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
)

// forget is /forget, accepted at any step, before the consent included
// (decision Р-2 A, US-009): the notice promises that /forget removes the
// Telegram ID, and the pending_consent row of a player who declined holds it.
//
// Without confirm it only asks. With confirm it sends DELETE /v1/links once —
// the client repeats a lost answer itself — and never checks the result with
// links/resolve: resolve of an account without a link writes its external id
// into links.db again (T-456, SEC-04/05).
func (t *turn) forget() {
	if !t.cmd.Confirm {
		t.reply(render.ForgetQuestion, nil)
		return
	}
	res, err := t.f.gw.Forget(t.ctx, Platform, t.ext)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.Code == api.CodeForgetIncomplete && t.ctx.Err() == nil {
			// The link is deleted, its wipe is not confirmed: nothing of the
			// chat is kept but the memory that this /forget is under way.
			t.f.Reset(t.chat)
			t.f.markForgetting(t.chat, t.now)
			t.f.log.Warn("forget incomplete", slog.String(keyCode, apiErr.Code), slog.Int(keyStatus, apiErr.Status))
			t.reply(render.ForgetIncomplete(apiErr.RetryAfter), nil)
			return
		}
		t.fail("forget", err)
		return
	}
	underWay := t.f.takeForgetting(t.chat, t.now)
	t.f.Reset(t.chat)
	// {deleted: false} ends the same /forget when the client repeated a lost
	// answer or an earlier attempt answered forget_incomplete (C-08 v1.5).
	if res.Deleted || res.Repeated || underWay {
		t.logStep("link forgotten", Idle)
		t.reply(render.ForgetDone, render.StartKeyboard())
		return
	}
	t.reply(render.ForgetNothing, render.StartKeyboard())
}
