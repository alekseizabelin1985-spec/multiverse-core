package flow_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/commands"
	"multiverse-core.io/cmd/telegram-bot/internal/flow"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/internal/gateway/api"
)

// The onboarding driven the way the bot runs it: updates from a source, one at
// a time, into Handle (FakeUpdateSource and FakeSender of component §15).
func TestTheOnboardingThroughTheUpdateSource(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone, resolvedConsented)
	f.gw.on(routeConsent, answer{status: http.StatusOK, body: api.ConsentResponse{LinkStatus: "consented"}})
	f.gw.on(routeWorlds, oneWorld)
	st := aliveCharacter("Вася")
	f.gw.on(routeCreate, answer{status: http.StatusCreated, body: api.CreateCharacterResponse{PlayerID: "player-A", Character: &st, Created: ptr(true)}})
	f.gw.on(routeActions, accepted)

	src := updates.NewFake(f.update("/start"), f.update(render.DeclineButton), f.update("/look"), f.update(render.ConsentButton),
		f.update("Вася"), f.update("/enter dark-forest-01"))
	src.Close()
	if err := src.Start(t.Context(), f.flow.Handle); err != nil {
		t.Fatal(err)
	}
	got := f.texts(f.sent.Sent())
	want := []string{render.NoticeText, render.DeclineReply, render.NoticeText, render.NamePrompt}
	if len(got) != 6 || !reflect.DeepEqual(got[:4], want) || !strings.HasPrefix(got[4], "Персонаж «Вася» создан.") || got[5] != render.Accepted {
		t.Errorf("messages = %q", got)
	}
	if f.gw.count(routeConsent) != 1 || f.gw.count(routeCreate) != 1 || f.gw.count(routeActions) != 1 {
		t.Errorf("consent %d, create %d, actions %d; want one each", f.gw.count(routeConsent), f.gw.count(routeCreate), f.gw.count(routeActions))
	}
}

func TestStartShowsTheNoticeWithTheConsentKeyboard(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	got := f.send("/start")
	if len(got) != 1 || got[0].Text != render.NoticeText || !reflect.DeepEqual(got[0].Keyboard, render.ConsentKeyboard()) {
		t.Fatalf("answer = %q, want the notice with the consent keyboard", f.texts(got))
	}
	if got[0].ChatID != playerUser {
		t.Errorf("answered chat %d, want the sender", got[0].ChatID)
	}
	var req api.ResolveRequest
	if err := json.Unmarshal(f.gw.last(routeResolve).Body, &req); err != nil || req.ExternalPlatform != "telegram" || req.ExternalID != externalID() {
		t.Errorf("resolve body = %+v (%v)", req, err)
	}
	if step := f.flow.StepOf(playerUser); step != flow.AwaitingConsent {
		t.Errorf("step = %s, want awaiting_consent", step)
	}
}

func TestConsentButtonGivesTheConsentWithTheTimeOfTheNotice(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	f.send("/start")
	shownAt := f.clock.Now()
	f.clock.Advance(3 * time.Minute)

	f.gw.on(routeConsent, answer{status: http.StatusOK, body: api.ConsentResponse{LinkStatus: "consented"}})
	f.gw.on(routeResolve, resolvedConsented)
	got := f.send(render.ConsentButton)
	if n := f.gw.count(routeConsent); n != 1 {
		t.Fatalf("consent calls = %d, want 1", n)
	}
	var req api.ConsentRequest
	if err := json.Unmarshal(f.gw.last(routeConsent).Body, &req); err != nil {
		t.Fatal(err)
	}
	if !req.NoticeShown || !req.Consent || !req.AgeConfirmed || !req.ShownAt.Equal(shownAt) || req.ExternalID != externalID() || req.ExternalPlatform != "telegram" {
		t.Errorf("consent body = %+v, want all three flags and shown_at %v", req, shownAt)
	}
	if len(got) != 1 || got[0].Text != render.NamePrompt || got[0].Keyboard == nil || !got[0].Keyboard.Remove {
		t.Errorf("after the consent = %q, want the name prompt removing the consent keyboard", f.texts(got))
	}
	if step := f.flow.StepOf(playerUser); step != flow.AwaitingName {
		t.Errorf("step = %s, want awaiting_name", step)
	}
}

// Р-3 A: the refusal gets the short reply; the notice comes again with the
// next command or text; nothing creates a character or gives the consent.
func TestDeclineAnswersShortlyAndTheNoticeReturnsWithTheNextMessage(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	f.send("/start")

	got := f.send(render.DeclineButton)
	if len(got) != 1 || got[0].Text != render.DeclineReply || !reflect.DeepEqual(got[0].Keyboard, render.ConsentKeyboard()) {
		t.Fatalf("decline = %q, want only DeclineReply with the consent keyboard", f.texts(got))
	}
	for _, text := range []string{"/look", "/attack wolf-alpha", "привет", "Вася"} {
		got = f.send(text)
		if len(got) != 1 || got[0].Text != render.NoticeText || !reflect.DeepEqual(got[0].Keyboard, render.ConsentKeyboard()) {
			t.Errorf("%q while awaiting the consent = %q, want the notice with the keyboard", text, f.texts(got))
		}
	}
	for _, route := range []string{routeConsent, routeCreate, routeActions, routeWorlds} {
		if n := f.gw.count(route); n != 0 {
			t.Errorf("%s called %d times before the consent", route, n)
		}
	}
	if n := f.gw.count(routeResolve); n != 1 {
		t.Errorf("resolve calls = %d, want only that of /start", n)
	}
}

// Р-3 A: the consent is given only by the button pressed while the flow waits
// for it. A press without the notice right before — after the TTL, in a fresh
// chat — shows the notice again and does not give the consent.
func TestConsentButtonOutsideTheWaitShowsTheNotice(t *testing.T) {
	t.Run("fresh chat", func(t *testing.T) {
		f := newFixture(t)
		got := f.send(render.ConsentButton)
		if len(got) != 1 || got[0].Text != render.NoticeText || !reflect.DeepEqual(got[0].Keyboard, render.ConsentKeyboard()) {
			t.Fatalf("press in a fresh chat = %q, want the notice", f.texts(got))
		}
		if n := f.gw.total(); n != 0 {
			t.Errorf("gateway calls = %d, want none", n)
		}
	})
	t.Run("after the TTL", func(t *testing.T) {
		f := newFixture(t)
		f.gw.on(routeResolve, resolvedNone)
		f.send("/start")
		f.clock.Advance(flow.DialogTTL)
		got := f.send(render.ConsentButton)
		if len(got) != 1 || got[0].Text != render.NoticeText {
			t.Fatalf("press after the TTL = %q, want the notice", f.texts(got))
		}
		if n := f.gw.count(routeConsent); n != 0 {
			t.Fatalf("consent calls after the TTL = %d, want 0", n)
		}
		f.gw.on(routeConsent, answer{status: http.StatusOK, body: api.ConsentResponse{LinkStatus: "consented"}})
		f.gw.on(routeResolve, resolvedConsented)
		f.send(render.ConsentButton)
		if n := f.gw.count(routeConsent); n != 1 {
			t.Errorf("consent calls after the notice was shown again = %d, want 1", n)
		}
	})
	t.Run("a player with a character", func(t *testing.T) {
		f := newFixture(t)
		f.onboard()
		consents := f.gw.count(routeConsent)
		got := f.send(render.ConsentButton)
		if len(got) != 1 || got[0].Text != render.NoticeText || f.gw.count(routeConsent) != consents {
			t.Errorf("press while ready = %q, consent calls %d → %d; want the notice and no consent", f.texts(got), consents, f.gw.count(routeConsent))
		}
		// N-1 of review #1 of T-311 (decision of tech-lead#3): the press keeps
		// the chat ready — no consent keyboard, and the next game command acts
		// instead of being answered with the notice for 15 minutes.
		if len(got) == 1 && got[0].Keyboard != nil {
			t.Errorf("press while ready offers a keyboard %+v, want none: the player already consented", got[0].Keyboard)
		}
		if step := f.flow.StepOf(playerUser); step != flow.Ready {
			t.Errorf("step after the press = %s, want ready", step)
		}
		f.gw.on(routeActions, accepted)
		if got := f.send("/look"); len(got) != 1 || got[0].Text != render.Accepted || f.gw.count(routeActions) != 1 {
			t.Errorf("/look after the press = %q, actions %d; want «Принято.» and one action", f.texts(got), f.gw.count(routeActions))
		}
	})
	t.Run("a button of another text", func(t *testing.T) {
		f := newFixture(t)
		f.gw.on(routeResolve, resolvedNone)
		f.send("/start")
		for _, near := range []string{" " + render.ConsentButton, render.ConsentButton + " ", strings.ToLower(render.ConsentButton), "Мне есть 18,\u00a0принимаю"} {
			f.send(near)
		}
		if n := f.gw.count(routeConsent); n != 0 {
			t.Errorf("consent calls on labels that only look like the button = %d, want 0", n)
		}
	})
}

// Mi-5 of T-318: /help never gives the consent.
func TestHelpBeforeTheConsentShowsOnlyTheNotice(t *testing.T) {
	for _, res := range []answer{resolved("none", "none", nil), resolvedNone} {
		f := newFixture(t)
		f.gw.on(routeResolve, res)
		got := f.send("/help")
		if len(got) != 1 || got[0].Text != render.NoticeText || !reflect.DeepEqual(got[0].Keyboard, render.ConsentKeyboard()) {
			t.Errorf("/help of %s = %q, want the notice with the consent keyboard", res.body.(api.ResolveResponse).LinkStatus, f.texts(got))
		}
		if n := f.gw.count(routeConsent); n != 0 {
			t.Errorf("/help of %s: consent calls %d, want 0", res.body.(api.ResolveResponse).LinkStatus, n)
		}
	}
}

func TestHelpOfAPlayerWhoConsentedShowsTheCommandsAndTheNotice(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedAlive)
	got := f.send("/help")
	if !reflect.DeepEqual(f.texts(got), []string{render.HelpText, render.NoticeText}) {
		t.Errorf("/help = %q, want the commands and the notice", f.texts(got))
	}
	for _, m := range got {
		if m.Keyboard != nil {
			t.Errorf("/help of a player who consented offers a keyboard %+v", m.Keyboard)
		}
	}
	if f.gw.count(routeResolve) != 1 || f.gw.count(routeConsent) != 0 {
		t.Errorf("resolve %d, consent %d; want 1 and 0: last_seen_at is refreshed by resolve", f.gw.count(routeResolve), f.gw.count(routeConsent))
	}
}

// Acceptance of T-312: the bot checks a name by api.ValidCharacterName, the
// rule of POST /v1/characters, after trimming the message. The rule is stricter
// than the copy the bot had before in two places — two spaces in a row and a
// name in NFD are refused — and both reach the player as NameInvalid without a
// call to the gateway.
func TestTheNameIsCheckedByTheRuleOfTheGateway(t *testing.T) {
	for _, name := range []string{"Вася  Пупкин", "йва", "Вася!", "В"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.gw.on(routeResolve, resolvedConsented)
			f.send("/start")
			got := f.send(name)
			if len(got) != 1 || got[0].Text != render.NameInvalid {
				t.Errorf("name %q = %q, want NameInvalid", name, f.texts(got))
			}
			if f.gw.count(routeWorlds)+f.gw.count(routeCreate) != 0 {
				t.Errorf("name %q reached the gateway", name)
			}
		})
	}

	f := newFixture(t)
	f.gw.on(routeResolve, resolvedConsented)
	f.gw.on(routeWorlds, oneWorld)
	st := aliveCharacter("Вася Пупкин")
	f.gw.on(routeCreate, answer{status: http.StatusCreated, body: api.CreateCharacterResponse{PlayerID: "player-A", Character: &st, Created: ptr(true)}})
	f.send("/start")
	f.send("  Вася Пупкин \t")
	var req api.CreateCharacterRequest
	if err := json.Unmarshal(f.gw.last(routeCreate).Body, &req); err != nil || req.CharacterName != "Вася Пупкин" {
		t.Errorf("create body = %+v (%v), want the name trimmed at both ends", req, err)
	}
}

func TestAnInvalidNameIsAskedAgain(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedConsented)
	f.send("/start")
	got := f.send("В!")
	if len(got) != 1 || got[0].Text != render.NameInvalid {
		t.Errorf("invalid name = %q, want NameInvalid", f.texts(got))
	}
	got = f.send("/look")
	if len(got) != 1 || got[0].Text != render.NamePrompt {
		t.Errorf("a command instead of a name = %q, want the prompt", f.texts(got))
	}
	if f.gw.count(routeCreate)+f.gw.count(routeWorlds)+f.gw.count(routeActions) != 0 {
		t.Error("an invalid name reached the gateway")
	}
	if step := f.flow.StepOf(playerUser); step != flow.AwaitingName {
		t.Errorf("step = %s, want awaiting_name", step)
	}
}

// US-009: a name equal to the username or the first name, ignoring case, is
// confirmed first; the profile is compared and dropped.
func TestANameEqualToTheProfileIsConfirmed(t *testing.T) {
	for _, name := range []string{"vasyatg", "VASYATG", "василий"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.gw.on(routeResolve, resolvedConsented)
			f.send("/start")
			got := f.send(name)
			if len(got) != 1 || got[0].Text != render.NameMatchesProfile || !reflect.DeepEqual(got[0].Keyboard, render.NameConfirmKeyboard()) {
				t.Fatalf("name %q = %q, want the warning with [Да] [Ввести другое]", name, f.texts(got))
			}
			if step := f.flow.StepOf(playerUser); step != flow.AwaitingNameConfirm {
				t.Fatalf("step = %s, want awaiting_name_confirm", step)
			}
			if f.gw.count(routeCreate) != 0 {
				t.Fatal("the character was created before the confirmation")
			}

			got = f.send(render.NameChange)
			if len(got) != 1 || got[0].Text != render.NamePrompt {
				t.Fatalf("another name = %q, want the prompt", f.texts(got))
			}
			f.send(name)
			f.gw.on(routeWorlds, oneWorld)
			st := aliveCharacter(name)
			f.gw.on(routeCreate, answer{status: http.StatusCreated, body: api.CreateCharacterResponse{PlayerID: "player-A", Character: &st, Created: ptr(true)}})
			f.send(render.NameKeep)
			var req api.CreateCharacterRequest
			if err := json.Unmarshal(f.gw.last(routeCreate).Body, &req); err != nil || req.CharacterName != name || req.WorldID != "dark-forest-world" {
				t.Errorf("create body = %+v (%v), want the typed name in the only world", req, err)
			}
		})
	}
}

func TestTheOnlyWorldCreatesTheCharacterWithTheActionKeyOfTheUpdate(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedConsented)
	f.send("/start")
	f.gw.on(routeWorlds, oneWorld)
	st := aliveCharacter("Вася")
	f.gw.on(routeCreate, answer{status: http.StatusCreated, body: api.CreateCharacterResponse{PlayerID: "player-A", Character: &st, Created: ptr(true)}})
	u := f.update("  Вася ")
	f.flow.Handle(t.Context(), u)

	var req api.CreateCharacterRequest
	if err := json.Unmarshal(f.gw.last(routeCreate).Body, &req); err != nil {
		t.Fatal(err)
	}
	if req.CharacterName != "Вася" || req.ActionKey != commands.ActionKey(salt, u.ID) || req.ExternalID != externalID() {
		t.Errorf("create body = %+v", req)
	}
	got := f.sent.Sent()
	last := got[len(got)-1]
	if !strings.HasPrefix(last.Text, "Персонаж «Вася» создан.") || !strings.Contains(last.Text, "/enter dark-forest-01") ||
		!reflect.DeepEqual(last.Keyboard, &sender.Keyboard{Rows: [][]string{{"/enter dark-forest-01"}}}) {
		t.Errorf("after the creation = %q %+v", last.Text, last.Keyboard)
	}
	if step := f.flow.StepOf(playerUser); step != flow.Ready {
		t.Errorf("step = %s, want ready", step)
	}
}

func TestSeveralWorldsAreChosenByTheKeyboard(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedConsented)
	f.send("/start")
	f.gw.on(routeWorlds, answer{status: http.StatusOK, body: api.WorldsResponse{Worlds: []api.WorldSummary{
		{WorldID: "w-1", Name: "Тёмный лес"}, {WorldID: "w-2", Name: "Пустошь"},
	}}})
	got := f.send("Вася")
	if len(got) != 1 || got[0].Text != render.WorldPrompt || !reflect.DeepEqual(got[0].Keyboard.Rows, [][]string{{"Тёмный лес"}, {"Пустошь"}}) {
		t.Fatalf("several worlds = %q %+v", f.texts(got), got[0].Keyboard)
	}
	f.send("Марс")
	if f.gw.count(routeCreate) != 0 {
		t.Fatal("an unknown world created a character")
	}
	st := aliveCharacter("Вася")
	f.gw.on(routeCreate, answer{status: http.StatusCreated, body: api.CreateCharacterResponse{PlayerID: "player-A", Character: &st, Created: ptr(true)}})
	f.send("Пустошь")
	var req api.CreateCharacterRequest
	if err := json.Unmarshal(f.gw.last(routeCreate).Body, &req); err != nil || req.WorldID != "w-2" {
		t.Errorf("create body = %+v (%v), want w-2", req, err)
	}
}

func TestACharacterBeingCreatedIsPolled(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedConsented)
	f.send("/start")
	f.gw.on(routeWorlds, oneWorld)
	f.gw.on(routeCreate, answer{status: http.StatusAccepted, body: api.CreateCharacterResponse{PlayerID: "player-A", Status: "creating"}})
	creating := api.CharacterState{PlayerID: "player-A", Name: "Вася", WorldID: "dark-forest-world", Status: "creating"}
	f.gw.on(routePlayer, answer{status: http.StatusOK, body: creating}, answer{status: http.StatusOK, body: creating}, answer{status: http.StatusOK, body: aliveCharacter("Вася")})

	done := make(chan struct{})
	go func() {
		defer close(done)
		f.flow.Handle(t.Context(), f.update("Вася"))
	}()
	for range 3 {
		if d := <-f.armed; d != flow.CreatingPollInterval {
			t.Errorf("pause %v, want %v", d, flow.CreatingPollInterval)
		}
		f.clock.Advance(flow.CreatingPollInterval)
	}
	<-done
	if n := f.gw.count(routePlayer); n != 3 {
		t.Errorf("status polls = %d, want 3", n)
	}
	got := f.sent.Sent()
	if last := got[len(got)-1].Text; !strings.HasPrefix(last, "Персонаж «Вася» создан.") {
		t.Errorf("after the polls = %q", last)
	}
}

func TestTheOnboardingExpiresAndStartRestoresItByResolve(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedConsented)
	f.send("/start")
	f.clock.Advance(flow.DialogTTL - 1)
	if step := f.flow.StepOf(playerUser); step != flow.AwaitingName {
		t.Fatalf("step a moment before the TTL = %s, want awaiting_name", step)
	}
	f.clock.Advance(1)
	if step := f.flow.StepOf(playerUser); step != flow.Idle {
		t.Fatalf("step at the TTL = %s, want idle", step)
	}
	got := f.send("Вася")
	if f.gw.count(routeCreate) != 0 || f.gw.count(routeWorlds) != 0 {
		t.Fatal("a name after the TTL created a character without the dialog")
	}
	if f.gw.count(routeResolve) != 2 || len(got) != 1 || got[0].Text != render.NamePrompt {
		t.Errorf("after the TTL: resolve %d, answer %q; want the step restored by resolve", f.gw.count(routeResolve), f.texts(got))
	}
}

func TestADeadCharacterStartsANewOne(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolved("consented", "dead", ptr("player-A")))
	got := f.send("/start")
	if len(got) != 1 || got[0].Text != render.DeadPrompt {
		t.Errorf("/start of a dead character = %q, want the prompt of a new one", f.texts(got))
	}
}

func TestStartOfAPlayerWithACharacterShowsTheStatus(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedAlive)
	f.gw.on(routePlayer, answer{status: http.StatusOK, body: aliveCharacter("Вася")})
	got := f.send("/start")
	if len(got) != 1 || !strings.HasPrefix(got[0].Text, "«Вася»: жив") || f.flow.StepOf(playerUser) != flow.Ready {
		t.Errorf("/start with a character = %q, step %s", f.texts(got), f.flow.StepOf(playerUser))
	}
}

// Mi-1 of review #1 of T-311: the flow waits for the consent only once the
// notice is sent. After a notice that did not reach the player, the button
// shows the notice and does not give the consent.
func TestTheConsentWaitsForTheNoticeToBeSent(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	f.sent.FailNext(sender.ErrUnavailable)
	if got := f.send("/start"); len(got) != 0 {
		t.Fatalf("answers = %q, want none: the notice was not sent", f.texts(got))
	}
	if step := f.flow.StepOf(playerUser); step != flow.Idle {
		t.Fatalf("step after a notice not sent = %s, want idle", step)
	}
	f.gw.on(routeConsent, answer{status: http.StatusOK, body: api.ConsentResponse{LinkStatus: "consented"}})
	got := f.send(render.ConsentButton)
	if n := f.gw.count(routeConsent); n != 0 {
		t.Errorf("consent calls after a notice not sent = %d, want 0", n)
	}
	if len(got) != 1 || got[0].Text != render.NoticeText || !reflect.DeepEqual(got[0].Keyboard, render.ConsentKeyboard()) {
		t.Errorf("press after a notice not sent = %q, want the notice with the consent keyboard", f.texts(got))
	}
}

// Mi-3 (б) of review #1 of T-311: notice_due of a player who consented shows
// the notice before the answer, on /start and on a game command after a miss
// of the player cache.
func TestNoticeDueOfAPlayerWhoConsentedShowsTheNoticeFirst(t *testing.T) {
	due := resolvedAlive
	res := due.body.(api.ResolveResponse)
	res.NoticeDue = true
	due.body = res

	t.Run("start", func(t *testing.T) {
		f := newFixture(t)
		f.gw.on(routeResolve, due)
		f.gw.on(routePlayer, answer{status: http.StatusOK, body: aliveCharacter("Вася")})
		got := f.send("/start")
		if len(got) != 2 || got[0].Text != render.NoticeText || got[0].Keyboard != nil || !strings.HasPrefix(got[1].Text, "«Вася»: жив") {
			t.Errorf("/start with notice_due = %q, want the notice without a keyboard, then the status", f.texts(got))
		}
		if f.gw.count(routeConsent) != 0 {
			t.Error("notice_due gave the consent")
		}
	})
	t.Run("a game command after a cache miss", func(t *testing.T) {
		f := newFixture(t)
		f.gw.on(routeResolve, due)
		f.gw.on(routeActions, accepted)
		got := f.send("/look")
		if len(got) != 2 || got[0].Text != render.NoticeText || got[0].Keyboard != nil || got[1].Text != render.Accepted {
			t.Errorf("/look with notice_due = %q, want the notice without a keyboard, then «Принято.»", f.texts(got))
		}
		if f.gw.count(routeActions) != 1 || f.gw.count(routeConsent) != 0 {
			t.Errorf("actions %d, consent %d; want 1 and 0", f.gw.count(routeActions), f.gw.count(routeConsent))
		}
	})
}

// N-6 of review #1 of T-311: a context cancelled during links/consent stops
// the turn before the next call of the gateway.
func TestAContextCancelledDuringTheConsentCallsNothingMore(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	f.send("/start")

	ctx, cancel := context.WithCancel(t.Context())
	gw := &cancellingGateway{Gateway: f.client, cancel: cancel}
	fl, err := flow.New(flow.Options{Gateway: gw, Sender: f.sent, Clock: f.clock, ActionKeySalt: salt})
	if err != nil {
		t.Fatal(err)
	}
	fl.Handle(t.Context(), f.update("/start"))
	sent := len(f.sent.Sent())
	fl.Handle(ctx, f.update(render.ConsentButton))
	if gw.consents != 1 || gw.after != 0 {
		t.Errorf("consent calls %d, calls after it %d; want 1 and 0", gw.consents, gw.after)
	}
	if n := len(f.sent.Sent()) - sent; n != 0 {
		t.Errorf("answers after the context ended = %d, want 0", n)
	}
}

// cancellingGateway ends the context of the turn inside Consent and counts
// every call after it, whatever its context.
type cancellingGateway struct {
	flow.Gateway
	cancel          context.CancelFunc
	consents, after int
}

func (g *cancellingGateway) Resolve(ctx context.Context, platform, externalID string) (api.ResolveResponse, error) {
	if g.consents > 0 {
		g.after++
		return api.ResolveResponse{LinkStatus: "consented", CharacterStatus: "none"}, nil
	}
	return g.Gateway.Resolve(ctx, platform, externalID)
}

func (g *cancellingGateway) Consent(context.Context, api.ConsentRequest) (api.ConsentResponse, error) {
	g.consents++
	g.cancel()
	return api.ConsentResponse{LinkStatus: "consented"}, nil
}

// Mi-7 of review #1 of T-311: StepOf from another goroutine while Handle runs.
// On Windows without cgo it only runs; CI on Linux runs it under -race.
func TestStepOfIsSafeWhileHandleRuns(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedConsented)
	f.gw.on(routeWorlds, answer{status: http.StatusOK, body: api.WorldsResponse{Worlds: []api.WorldSummary{
		{WorldID: "w-1", Name: "Тёмный лес"}, {WorldID: "w-2", Name: "Пустошь"},
	}}})
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				_ = f.flow.StepOf(playerUser)
			}
		}
	}()
	for range 20 {
		f.send("/start")
		f.send("vasyatg")
		f.send(render.NameKeep)
		f.send("Марс")
	}
	close(done)
	wg.Wait()
	if step := f.flow.StepOf(playerUser); step != flow.AwaitingWorld {
		t.Errorf("step = %s, want awaiting_world", step)
	}
}

// N-2 of review #1 of T-311: the sweep on every update lets go of what expired
// in other chats, so an id stays no longer than its TTL and the next update.
func TestEveryUpdateSweepsTheExpiredChats(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	f.send("/start")
	if n := flow.HeldChats(f.flow); n != 1 {
		t.Fatalf("held chats after /start = %d, want 1", n)
	}
	f.clock.Advance(flow.DialogTTL)
	other := updates.Update{ID: 9, Message: &updates.Message{Chat: updates.Chat{ID: 77, Type: updates.ChatPrivate}, From: &updates.User{ID: 77}, Text: "/forget"}}
	f.flow.Handle(t.Context(), other)
	if n := flow.HeldChats(f.flow); n != 0 {
		t.Errorf("held chats after an update of another chat past the TTL = %d, want 0", n)
	}
}
