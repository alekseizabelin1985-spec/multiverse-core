package flow_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/flow"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
)

var (
	forgotten = answer{status: http.StatusOK, body: api.ForgetResponse{Deleted: true, PlayerIDDetached: ptr("player-A")}}
	nothing   = answer{status: http.StatusOK, body: api.ForgetResponse{Deleted: false}}
)

func TestForgetWithoutConfirmOnlyAsks(t *testing.T) {
	f := newFixture(t)
	for _, text := range []string{"/forget", "/forget please", "/forget@multiverse_bot"} {
		got := f.send(text)
		if len(got) != 1 || got[0].Text != render.ForgetQuestion {
			t.Errorf("%q = %q, want only the question", text, f.texts(got))
		}
	}
	if n := f.gw.total(); n != 0 {
		t.Errorf("gateway calls = %d, want none before the confirmation", n)
	}
}

func TestForgetConfirmDeletesTheLinkAndResets(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	f.gw.on(routeForget, forgotten)
	got := f.send("/forget confirm")
	if n := f.gw.count(routeForget); n != 1 {
		t.Fatalf("DELETE /v1/links calls = %d, want 1", n)
	}
	if len(got) != 1 || got[0].Text != render.ForgetDone || !reflect.DeepEqual(got[0].Keyboard, render.StartKeyboard()) {
		t.Errorf("after /forget confirm = %q, want «Связка удалена. /start — начать заново»", f.texts(got))
	}
	if step := f.flow.StepOf(playerUser); step != flow.Idle {
		t.Errorf("step after /forget = %s, want idle: the player cache is reset", step)
	}
	// The next command asks the gateway again instead of acting for the
	// forgotten player_id.
	f.gw.on(routeResolve, resolvedNone)
	resolves := f.gw.count(routeResolve)
	f.send("/look")
	if f.gw.count(routeResolve) != resolves+1 || f.gw.count(routeActions) != 0 {
		t.Errorf("after /forget /look: resolve %d → %d, actions %d; want a resolve and no action", resolves, f.gw.count(routeResolve), f.gw.count(routeActions))
	}
}

// Р-2 A: /forget before the consent removes the pending_consent row.
func TestForgetWorksWhileAwaitingTheConsent(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	f.send("/start")
	f.send(render.DeclineButton)

	if got := f.send("/forget"); len(got) != 1 || got[0].Text != render.ForgetQuestion {
		t.Fatalf("/forget while awaiting the consent = %q, want the question", f.texts(got))
	}
	if f.gw.count(routeForget) != 0 {
		t.Fatal("/forget without confirm reached the gateway")
	}
	f.gw.on(routeForget, answer{status: http.StatusOK, body: api.ForgetResponse{Deleted: true}})
	if got := f.send("/forget confirm"); len(got) != 1 || got[0].Text != render.ForgetDone {
		t.Fatalf("/forget confirm while awaiting the consent = %q", f.texts(got))
	}
	if n := f.gw.count(routeForget); n != 1 {
		t.Errorf("DELETE /v1/links calls = %d, want exactly 1", n)
	}
	if f.gw.count(routeConsent) != 0 {
		t.Error("the consent was given on the way to /forget")
	}
	// The dialog is gone: the button no longer gives the consent.
	f.send(render.ConsentButton)
	if f.gw.count(routeConsent) != 0 || f.flow.StepOf(playerUser) != flow.AwaitingConsent {
		t.Errorf("press after /forget: consent calls %d, step %s; want 0 and the notice shown again", f.gw.count(routeConsent), f.flow.StepOf(playerUser))
	}
}

func TestForgetOfAnAccountWithoutALinkHasNothingToDelete(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeForget, nothing)
	got := f.send("/forget confirm")
	if len(got) != 1 || got[0].Text != render.ForgetNothing {
		t.Errorf("/forget confirm without a link = %q, want «нечего удалять»", f.texts(got))
	}
}

// C-08 v1.5: 503 forget_incomplete is not «deleted»; the repeat that answers
// {deleted: false} ends the same /forget. No resolve on the way (T-456).
func TestForgetIncompleteAsksToRepeatAndItsRepeatEndsTheSameForget(t *testing.T) {
	f := newFixture(t)
	f.client.Backoff = client.DefaultBackoff
	f.client.Timers = instantTimers{}
	f.onboard()
	resolves := f.gw.count(routeResolve)

	f.gw.on(routeForget, apiError(http.StatusServiceUnavailable, api.CodeForgetIncomplete, map[string]string{api.HeaderRetryAfter: "5"}), nothing)
	got := f.send("/forget confirm")
	if len(got) != 1 {
		t.Fatalf("answers = %q, want one", f.texts(got))
	}
	if strings.Contains(strings.ToLower(got[0].Text), "удален") || !strings.Contains(got[0].Text, "/forget confirm через 5 с") {
		t.Errorf("forget_incomplete = %q, want a request to repeat in 5 s without «удален»", got[0].Text)
	}
	if n := f.gw.count(routeForget); n != 1 {
		t.Errorf("DELETE /v1/links calls = %d, want 1: forget_incomplete is not repeated by the client", n)
	}

	got = f.send("/forget confirm")
	if len(got) != 1 || got[0].Text != render.ForgetDone {
		t.Errorf("repeat after forget_incomplete answered {deleted:false} = %q, want «Связка удалена»", f.texts(got))
	}
	if n := f.gw.count(routeResolve); n != resolves {
		t.Errorf("resolve calls on the way through /forget = %d, want 0", n-resolves)
	}
	// Once ended, a third /forget of the same chat has nothing to delete.
	if got = f.send("/forget confirm"); len(got) != 1 || got[0].Text != render.ForgetNothing {
		t.Errorf("third /forget = %q, want «нечего удалять»", f.texts(got))
	}
}

// T-456: a lost answer repeated by the client ends with {deleted: false} and
// Repeated; the bot reports the link gone and never checks it with resolve,
// which would write the forgotten id back into links.db.
func TestForgetWithALostAnswerIsDoneWithoutResolve(t *testing.T) {
	f := newFixture(t)
	f.client.Backoff = client.DefaultBackoff
	f.client.Timers = instantTimers{}
	f.onboard()
	resolves := f.gw.count(routeResolve)

	f.gw.on(routeForget, answer{}, nothing)
	got := f.send("/forget confirm")
	if n := f.gw.count(routeForget); n != 2 {
		t.Fatalf("DELETE /v1/links calls = %d, want the attempt and its repeat", n)
	}
	if len(got) != 1 || got[0].Text != render.ForgetDone {
		t.Errorf("after a repeated /forget = %q, want «Связка удалена»", f.texts(got))
	}
	if n := f.gw.count(routeResolve) - resolves; n != 0 {
		t.Errorf("resolve calls after /forget = %d, want 0", n)
	}
}

func TestForgetThatFailsKeepsTheChat(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	f.gw.on(routeForget, apiError(http.StatusServiceUnavailable, api.CodeBusUnavailable, nil))
	got := f.send("/forget confirm")
	if len(got) != 1 || got[0].Text != render.Unavailable {
		t.Errorf("/forget under bus_unavailable = %q, want «сервис недоступен»", f.texts(got))
	}
	if step := f.flow.StepOf(playerUser); step != flow.Ready {
		t.Errorf("step = %s, want ready: the link is still there", step)
	}
}

// instantTimers fires every pause of the client at once.
type instantTimers struct{}

func (instantTimers) After(time.Duration) clock.Timer {
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return chanTimer{ch}
}

func (instantTimers) Every(time.Duration) clock.Timer { panic("no periodic timer") }

type chanTimer struct{ ch chan time.Time }

func (c chanTimer) C() <-chan time.Time { return c.ch }
func (c chanTimer) Stop() bool          { return false }

// Mi-2 of review #1 of T-311: 503 forget_incomplete resets the chat — the link
// is deleted, so the cached player_id must not act any more.
func TestForgetIncompleteResetsTheChat(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	f.gw.on(routeForget, apiError(http.StatusServiceUnavailable, api.CodeForgetIncomplete, map[string]string{api.HeaderRetryAfter: "5"}))
	f.send("/forget confirm")
	if step := f.flow.StepOf(playerUser); step != flow.Idle {
		t.Errorf("step after forget_incomplete = %s, want idle", step)
	}
	f.gw.on(routeResolve, resolvedNone)
	resolves := f.gw.count(routeResolve)
	f.send("/look")
	if f.gw.count(routeResolve) != resolves+1 || f.gw.count(routeActions) != 0 {
		t.Errorf("/look after forget_incomplete: resolve %d → %d, actions %d; want a resolve and no action",
			resolves, f.gw.count(routeResolve), f.gw.count(routeActions))
	}
}
