package flow_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/commands"
	"multiverse-core.io/cmd/telegram-bot/internal/flow"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
)

func TestAGameCommandIsSentAsAnActionWithTheKeyOfTheUpdate(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	f.gw.on(routeActions, accepted)

	u := f.update("/attack wolf-alpha")
	f.flow.Handle(t.Context(), u)
	var req api.ActionRequest
	if err := json.Unmarshal(f.gw.last(routeActions).Body, &req); err != nil {
		t.Fatal(err)
	}
	if req.Type != api.ActionAttack || req.Target == nil || *req.Target != "wolf-alpha" || req.ActionKey != commands.ActionKey(salt, u.ID) || req.Text != nil {
		t.Errorf("action = %+v", req)
	}
	sent := f.sent.Sent()
	if last := sent[len(sent)-1]; last.Text != render.Accepted {
		t.Errorf("answer = %q, want «Принято.»", last.Text)
	}

	// The same update handled twice is the same action.
	f.flow.Handle(t.Context(), u)
	var again api.ActionRequest
	_ = json.Unmarshal(f.gw.last(routeActions).Body, &again)
	if again.ActionKey != req.ActionKey {
		t.Errorf("repeat of the update: action_key %s, want %s", again.ActionKey, req.ActionKey)
	}

	f.send("/say [x](http://evil) *привет*")
	var say api.ActionRequest
	_ = json.Unmarshal(f.gw.last(routeActions).Body, &say)
	if say.Type != api.ActionSay || say.Text == nil || *say.Text != "[x](http://evil) *привет*" || say.Target != nil {
		t.Errorf("say = %+v", say)
	}
}

func TestFreeTextOfAPlayerWithACharacterShowsTheCommands(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	for _, text := range []string{"привет всем", "/dance", "leave me alone"} {
		got := f.send(text)
		if len(got) != 1 || got[0].Text != render.HelpText {
			t.Errorf("%q = %q, want the list of commands", text, f.texts(got))
		}
	}
	if n := f.gw.count(routeActions); n != 0 {
		t.Errorf("actions = %d, want none for free text (US-008)", n)
	}
	got := f.send("/say")
	if len(got) != 1 || !strings.Contains(got[0].Text, "/say") || f.gw.count(routeActions) != 0 {
		t.Errorf("/say without text = %q, want the hint and no action", f.texts(got))
	}
}

func TestErrorsOfActionsAreAnsweredByCode(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	f.gw.on(routeActions, apiError(http.StatusConflict, api.CodeInEncounter, nil))
	if got := f.send("/rest"); len(got) != 1 || got[0].Text != "Сейчас идёт бой. Сначала /flee." {
		t.Errorf("in_encounter = %q", f.texts(got))
	}
	f.gw.on(routeActions, apiError(http.StatusTooManyRequests, api.CodeRateLimited, map[string]string{api.HeaderRetryAfter: "7"}))
	if got := f.send("/look"); len(got) != 1 || got[0].Text != "Слишком часто, подождите 7 с." {
		t.Errorf("rate_limited = %q", f.texts(got))
	}
	f.gw.on(routeActions, answer{})
	if got := f.send("/look"); len(got) != 1 || got[0].Text != render.Unavailable {
		t.Errorf("network error = %q", f.texts(got))
	}

	f.gw.on(routeActions, apiError(http.StatusConflict, api.CodeCharacterDead, nil))
	got := f.send("/look")
	if len(got) != 1 || got[0].Text != "Персонаж больше не может действовать. /start — создать нового." || !reflect.DeepEqual(got[0].Keyboard, render.StartKeyboard()) {
		t.Errorf("character_dead = %q", f.texts(got))
	}
	if step := f.flow.StepOf(playerUser); step != flow.Idle {
		t.Errorf("after character_dead the step is %s, want idle: the player is asked again", step)
	}
}

func TestConsentRequiredRestartsTheOnboarding(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	f.gw.on(routeActions, apiError(http.StatusForbidden, api.CodeConsentRequired, nil))
	got := f.send("/look")
	if len(got) != 2 || got[1].Text != render.NoticeText || !reflect.DeepEqual(got[1].Keyboard, render.ConsentKeyboard()) {
		t.Fatalf("consent_required = %q, want the hint and the notice", f.texts(got))
	}
	if step := f.flow.StepOf(playerUser); step != flow.AwaitingConsent {
		t.Errorf("step = %s, want awaiting_consent", step)
	}
}

func TestAttackWithoutATarget(t *testing.T) {
	wolf := api.NPCView{NPCID: "wolf-alpha", Name: "Альфа-волк", HP: 4, HPMax: 10, Status: "alive"}
	t.Run("one opponent is attacked", func(t *testing.T) {
		f := newFixture(t)
		f.onboard()
		f.gw.on(routePlayer, answer{status: http.StatusOK, body: aliveCharacter("Вася", wolf, api.NPCView{NPCID: "wolf-dead", Status: "dead"})})
		f.gw.on(routeActions, accepted)
		f.send("/attack")
		var req api.ActionRequest
		_ = json.Unmarshal(f.gw.last(routeActions).Body, &req)
		if req.Target == nil || *req.Target != "wolf-alpha" {
			t.Errorf("action = %+v, want the only living opponent", req)
		}
	})
	t.Run("several opponents are offered", func(t *testing.T) {
		f := newFixture(t)
		f.onboard()
		other := wolf
		other.NPCID = "wolf-beta"
		f.gw.on(routePlayer, answer{status: http.StatusOK, body: aliveCharacter("Вася", wolf, other)})
		got := f.send("/attack")
		if len(got) != 1 || got[0].Text != render.AttackPrompt || !reflect.DeepEqual(got[0].Keyboard.Rows, [][]string{{"/attack wolf-alpha"}, {"/attack wolf-beta"}}) {
			t.Errorf("answer = %q", f.texts(got))
		}
		if f.gw.count(routeActions) != 0 {
			t.Error("an attack was sent without a chosen target")
		}
	})
	t.Run("no opponent", func(t *testing.T) {
		f := newFixture(t)
		f.onboard()
		f.gw.on(routePlayer, answer{status: http.StatusOK, body: aliveCharacter("Вася")})
		got := f.send("/attack")
		if len(got) != 1 || got[0].Text != "Сейчас нет боя. Вокруг никого; /look." || f.gw.count(routeActions) != 0 {
			t.Errorf("answer = %q, actions %d", f.texts(got), f.gw.count(routeActions))
		}
	})
}

func TestEnterWithoutARegionOffersTheRegions(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	got := f.send("/enter")
	if len(got) != 1 || got[0].Text != render.EnterPrompt || !reflect.DeepEqual(got[0].Keyboard.Rows, [][]string{{"/enter dark-forest-01"}}) {
		t.Errorf("/enter = %q", f.texts(got))
	}
	if f.gw.count(routeActions) != 0 {
		t.Error("/enter without a region sent an action")
	}
}

func TestStatusShowsTheCharacter(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	f.gw.on(routePlayer, answer{status: http.StatusOK, body: aliveCharacter("Вася")})
	got := f.send("/status")
	if len(got) != 1 || !strings.HasPrefix(got[0].Text, "«Вася»: жив, HP 10/10") || !reflect.DeepEqual(got[0].Keyboard, render.BaseKeyboard()) {
		t.Errorf("/status = %q", f.texts(got))
	}
	if f.gw.count(routeActions) != 0 {
		t.Error("/status is not a turn, yet an action was sent")
	}
}

// A cache miss asks the gateway once and then acts; a hit does not ask.
func TestThePlayerIsCachedForAnHour(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedAlive)
	f.gw.on(routeActions, accepted)
	f.send("/look")
	f.send("/look")
	if f.gw.count(routeResolve) != 1 || f.gw.count(routeActions) != 2 {
		t.Fatalf("resolve %d, actions %d; want 1 and 2", f.gw.count(routeResolve), f.gw.count(routeActions))
	}
	f.clock.Advance(flow.PlayerTTL)
	f.send("/look")
	if f.gw.count(routeResolve) != 2 {
		t.Errorf("resolve after the TTL = %d, want 2", f.gw.count(routeResolve))
	}
}

// M-1 of review #1 of T-310: a cancelled context calls nothing and answers
// nothing.
func TestACancelledContextCallsNothing(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	calls, sent := f.gw.total(), len(f.sent.Sent())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, text := range []string{"/start", "/help", "/look", "/forget confirm", render.ConsentButton, "Вася"} {
		f.flow.Handle(ctx, f.update(text))
	}
	if n := f.gw.total() - calls; n != 0 {
		t.Errorf("gateway requests with a cancelled context = %d, want 0", n)
	}
	if n := len(f.sent.Sent()) - sent; n != 0 {
		t.Errorf("answers with a cancelled context = %d, want 0", n)
	}

	// The HTTP client and the fake sender refuse a cancelled context
	// themselves; a gateway and a sender that do not still see nothing.
	blindGateway, blindSender := &ctxBlindGateway{Gateway: f.client}, &ctxBlindSender{}
	fl, err := flow.New(flow.Options{Gateway: blindGateway, Sender: blindSender, Clock: f.clock, ActionKeySalt: salt})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"/start", "/help", "/look", "/forget confirm", render.ConsentButton} {
		fl.Handle(ctx, f.update(text))
	}
	if blindGateway.calls.Load() != 0 || blindSender.calls.Load() != 0 {
		t.Errorf("with a cancelled context: %d gateway calls, %d answers; want none", blindGateway.calls.Load(), blindSender.calls.Load())
	}
}

// ctxBlindGateway counts the calls the flow makes, whatever their context.
type ctxBlindGateway struct {
	flow.Gateway
	calls atomic.Int32
}

func (g *ctxBlindGateway) Resolve(context.Context, string, string) (api.ResolveResponse, error) {
	g.calls.Add(1)
	return api.ResolveResponse{LinkStatus: "consented", CharacterStatus: "alive", PlayerID: ptr("player-A")}, nil
}

func (g *ctxBlindGateway) Consent(context.Context, api.ConsentRequest) (api.ConsentResponse, error) {
	g.calls.Add(1)
	return api.ConsentResponse{}, nil
}

func (g *ctxBlindGateway) Forget(context.Context, string, string) (client.ForgetResult, error) {
	g.calls.Add(1)
	return client.ForgetResult{}, nil
}

func (g *ctxBlindGateway) Player(context.Context, string) (api.CharacterState, error) {
	g.calls.Add(1)
	return aliveCharacter("Вася"), nil
}

func (g *ctxBlindGateway) Action(context.Context, string, api.ActionRequest) (client.ActionResult, error) {
	g.calls.Add(1)
	return client.ActionResult{}, nil
}

// ctxBlindSender counts the answers, whatever their context.
type ctxBlindSender struct{ calls atomic.Int32 }

func (s *ctxBlindSender) Send(context.Context, int64, string, *sender.Keyboard) error {
	s.calls.Add(1)
	return nil
}

// A context cancelled while the gateway answers: the answer is not sent.
func TestAContextCancelledDuringTheCallAnswersNothing(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	ctx, cancel := context.WithCancel(t.Context())
	released := make(chan struct{})
	holding := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The server notices a gone client only once the body is read.
		_, _ = io.ReadAll(r.Body)
		cancel()
		<-r.Context().Done()
		close(released)
	}))
	defer holding.Close()
	f.client.BaseURL = holding.URL
	sent := len(f.sent.Sent())
	f.flow.Handle(ctx, f.update("/look"))
	<-released
	if n := len(f.sent.Sent()) - sent; n != 0 {
		t.Errorf("answers after the context ended = %d, want 0", n)
	}
}

// M-2 of review #1 of T-310: a 429 with retry_after 30 on an answer of the
// flow is not waited out, so the next update of another player reaches the
// gateway without delay.
func TestAFloodBanOnAnAnswerDoesNotHoldTheNextUpdate(t *testing.T) {
	const token = "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"
	var mu sync.Mutex
	answered := 0
	botAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		answered++
		first := answered == 1
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if first {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 30","parameters":{"retry_after":30}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":1,"date":1,"chat":{"id":1,"type":"private"}}}`)
	}))
	defer botAPI.Close()

	f := newFixture(t)
	pauses := &recordingTimers{}
	tg, err := sender.NewTelegram(sender.TelegramOptions{Token: token, ServerURL: botAPI.URL, Timers: pauses, Policy: sender.ReplyPolicy,
		HTTPClient: &http.Client{Timeout: 5 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	fl, err := flow.New(flow.Options{Gateway: f.client, Sender: tg, Clock: f.clock, ActionKeySalt: salt})
	if err != nil {
		t.Fatal(err)
	}
	f.gw.on(routeResolve, resolvedNone)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	fl.Handle(ctx, f.update("/start"))
	if ctx.Err() != nil {
		t.Fatal("the answer under a flood ban held the handler")
	}
	other := updates.Update{ID: 9, Message: &updates.Message{Chat: updates.Chat{ID: 77, Type: updates.ChatPrivate}, From: &updates.User{ID: 77}, Text: "/start"}}
	fl.Handle(ctx, other)
	if n := f.gw.count(routeResolve); n != 2 {
		t.Errorf("resolve calls = %d, want the next update to reach the gateway", n)
	}
	for _, d := range pauses.all() {
		if d > sender.ReplyPolicy.MaxRetryAfter {
			t.Errorf("the sender armed a pause of %v, above the %v of ReplyPolicy", d, sender.ReplyPolicy.MaxRetryAfter)
		}
	}
	if sender.ReplyPolicy.MaxRetryAfter >= 30*time.Second || sender.ReplyPolicy.RateLimitWaits > 1 {
		t.Errorf("ReplyPolicy = %+v waits out a flood ban", sender.ReplyPolicy)
	}
}

type recordingTimers struct {
	mu     sync.Mutex
	pauses []time.Duration
}

func (r *recordingTimers) After(d time.Duration) clock.Timer {
	r.mu.Lock()
	r.pauses = append(r.pauses, d)
	r.mu.Unlock()
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return chanTimer{ch}
}

func (r *recordingTimers) Every(time.Duration) clock.Timer { panic("no periodic timer") }

func (r *recordingTimers) all() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.pauses...)
}

// N-6 of review #1 of T-310: the flow logs its own keys only, and no id of
// Telegram, name or text reaches the log under any key.
func TestTheLogCarriesNoIdentityNameOrText(t *testing.T) {
	f := newFixture(t)
	f.gw.on(routeResolve, resolvedNone)
	f.send("/start")
	f.send(render.DeclineButton)
	f.gw.on(routeConsent, answer{status: http.StatusOK, body: api.ConsentResponse{LinkStatus: "consented"}})
	f.gw.on(routeResolve, resolvedConsented)
	f.send(render.ConsentButton)
	f.send("vasyatg")
	f.gw.on(routeWorlds, oneWorld)
	f.gw.on(routeCreate, apiError(http.StatusServiceUnavailable, api.CodeBusUnavailable, nil))
	f.send(render.NameKeep)
	st := aliveCharacter("vasyatg")
	f.gw.on(routeCreate, answer{status: http.StatusCreated, body: api.CreateCharacterResponse{PlayerID: "player-A", Character: &st, Created: ptr(true)}})
	f.send(render.NameKeep)
	f.gw.on(routeActions, apiError(http.StatusConflict, api.CodeInEncounter, nil))
	f.send("/say секретная реплика игрока")
	f.gw.on(routeActions, answer{})
	f.send("/look")
	f.gw.on(routeForget, apiError(http.StatusServiceUnavailable, api.CodeForgetIncomplete, map[string]string{api.HeaderRetryAfter: "5"}))
	f.send("/forget confirm")
	f.sent.FailNext(sender.ErrBlocked)
	f.send("/forget")

	logs := f.logs.String()
	if strings.Count(logs, "\n") < 5 {
		t.Fatalf("control: only %d log lines, the check proves little:\n%s", strings.Count(logs, "\n"), logs)
	}
	for _, secret := range []string{externalID(), username, strings.ToLower(username), firstName, "секретная реплика", render.ConsentButton, render.DeclineButton} {
		if strings.Contains(logs, secret) {
			t.Errorf("the log carries %q:\n%s", secret, logs)
		}
	}
	allowed := map[string]bool{"ts": true, "level": true, "msg": true, "service": true, "step": true, "command": true, "code": true, "status": true, "error": true}
	sc := bufio.NewScanner(strings.NewReader(logs))
	for sc.Scan() {
		var line map[string]any
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			t.Fatalf("log line is not JSON: %s", sc.Text())
		}
		for k := range line {
			if !allowed[k] {
				t.Errorf("log key %q is not one of the flow: %s", k, sc.Text())
			}
		}
	}
}

// Mi-4 of review #1 of T-311: an answer that is not of the contract — a proxy
// page with 502 — is shown to the player as the service being unavailable,
// without the status text or code.
func TestAnAnswerNotOfTheContractReadsAsUnavailable(t *testing.T) {
	f := newFixture(t)
	f.onboard()
	for _, a := range []answer{
		{status: http.StatusBadGateway, body: rawBody("<html><body><h1>502 Bad Gateway</h1></body></html>")},
		{status: http.StatusGatewayTimeout, body: rawBody("")},
		{status: http.StatusNotFound, body: rawBody("404 page not found")},
	} {
		f.gw.on(routeActions, a)
		got := f.send("/look")
		if len(got) != 1 || got[0].Text != render.Unavailable {
			t.Errorf("%d not of the contract = %q, want %q", a.status, f.texts(got), render.Unavailable)
		}
		for _, m := range got {
			if strings.Contains(m.Text, "Gateway") || strings.Contains(m.Text, "Not Found") || strings.Contains(m.Text, strconv.Itoa(a.status)) {
				t.Errorf("the player sees the status: %q", m.Text)
			}
		}
	}
	if step := f.flow.StepOf(playerUser); step != flow.Ready {
		t.Errorf("step = %s, want ready: nothing of the link is known", step)
	}
}

// Mi-6 of review #1 of T-311: the flow redacts the error texts it logs itself,
// even under a logger that is not built on privacy.NewHandler.
func TestErrorTextsAreRedactedByTheFlow(t *testing.T) {
	const token = "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"
	var logs strings.Builder
	log := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	failing := &failingGateway{err: errors.New("gateway POST https://api.telegram.org/bot" + token + "/x: connection reset")}
	sent := &sender.Fake{}
	fl, err := flow.New(flow.Options{Gateway: failing, Sender: sent, Clock: clock.NewManual(epoch), ActionKeySalt: salt, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	u := func(text string) updates.Update {
		return updates.Update{ID: 5, Message: &updates.Message{Chat: updates.Chat{ID: 77, Type: updates.ChatPrivate}, From: &updates.User{ID: 77}, Text: text}}
	}
	fl.Handle(t.Context(), u("/start"))
	sent.FailNext(errors.New("telegram sendMessage https://api.telegram.org/bot" + token + "/sendMessage: EOF"))
	fl.Handle(t.Context(), u("/forget"))

	if strings.Count(logs.String(), "\n") < 2 {
		t.Fatalf("control: want the two error lines, got:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), token) || strings.Contains(logs.String(), "ABCdefGHI") {
		t.Errorf("the log carries the token:\n%s", logs.String())
	}
}

// failingGateway fails links/resolve with its error.
type failingGateway struct {
	flow.Gateway
	err error
}

func (g *failingGateway) Resolve(context.Context, string, string) (api.ResolveResponse, error) {
	return api.ResolveResponse{}, g.err
}
