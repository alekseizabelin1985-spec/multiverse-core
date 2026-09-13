package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/config"
	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/logging"
)

// assertNoSecret fails when the token, its secret half or a Telegram id of the
// tests reached what the process wrote.
func assertNoSecret(t *testing.T, where, out string) {
	t.Helper()
	_, secret, _ := strings.Cut(testToken, ":")
	for _, leak := range []string{testToken, secret, strconv.FormatInt(stranger, 10), strconv.FormatInt(playerA, 10), strconv.FormatInt(playerB, 10), strconv.FormatInt(deliveryChat, 10)} {
		if strings.Contains(out, leak) {
			t.Errorf("%s carries %q:\n%s", where, leak, out)
		}
	}
}

// SEC-08: without a token the bot does not start, says why, and prints no
// secret.
func TestWithoutATokenTheBotStopsWithAClearError(t *testing.T) {
	bot := newBotAPI(t)
	gw := newGatewayMock(t)
	source := vars(bot, gw)
	source["MV_TELEGRAM_BOT_TOKEN"] = ""
	e, p := testEnvironment(bot, source)

	if code := run(context.Background(), nil, e); code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}
	if !strings.Contains(p.stderr.String(), config.ErrTokenMissing.Error()) {
		t.Errorf("stderr does not say the token is missing:\n%s", p.stderr)
	}
	if bot.callsOf("getMe") != 0 || len(gw.requestsSeen()) != 0 {
		t.Errorf("a bot without a token called Telegram (%d) or the gateway (%v)", bot.callsOf("getMe"), gw.requestsSeen())
	}
}

func TestAMalformedTokenIsRefusedWithoutItsValue(t *testing.T) {
	bot := newBotAPI(t)
	gw := newGatewayMock(t)
	source := vars(bot, gw)
	source["MV_TELEGRAM_BOT_TOKEN"] = "not-a-token-ABCdefGHIjklMNOpqrSTUvwxYZ"
	e, p := testEnvironment(bot, source)

	if code := run(context.Background(), nil, e); code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}
	if !strings.Contains(p.stderr.String(), "MV_TELEGRAM_BOT_TOKEN") || strings.Contains(p.stderr.String(), "ABCdefGHI") {
		t.Errorf("stderr:\n%s", p.stderr)
	}
}

// Review #1 of T-310, Mi-1: a token Telegram rejects at getMe stops the bot
// with code 1 and a clear line, without the token.
//
// Acceptance of T-312, N-3 of review #1: the delivery loop starts only after
// getMe, so a rejected token takes no delivery into a lease. getMe answers
// late here, which gives a loop started before it the time to poll.
func TestATokenRejectedAtStartExitsWithOne(t *testing.T) {
	bot := newBotAPI(t)
	bot.getMe = unauthorized
	bot.getMeDelay = 300 * time.Millisecond
	gw := newGatewayMock(t)
	e, p := testEnvironment(bot, vars(bot, gw))
	start(t, e, p)

	if code := p.exitCode(t); code != 1 {
		t.Fatalf("exit code %d, want 1\nstderr:\n%s", code, p.stderr)
	}
	if !strings.Contains(p.stderr.String(), updates.ErrUnauthorized.Error()) {
		t.Errorf("stderr does not say the token was rejected:\n%s", p.stderr)
	}
	if n := bot.callsOf("getUpdates"); n != 0 {
		t.Errorf("getUpdates called %d times after getMe answered 401", n)
	}
	if seen := gw.requestsSeen(); len(seen) != 0 {
		t.Errorf("the gateway was called although getMe answered 401: %v", seen)
	}
	assertNoSecret(t, "stderr", p.stderr.String())
	assertNoSecret(t, "the log", p.stdout.String())
}

// Review #1 of T-310, Mi-1: a token revoked while the bot polls stops polling
// at the first 401 and the bot exits with code 1.
func TestATokenRevokedWhilePollingExitsWithOne(t *testing.T) {
	bot := newBotAPI(t)
	bot.polls = []apiAnswer{unauthorized}
	gw := newGatewayMock(t)
	e, p := testEnvironment(bot, vars(bot, gw))
	start(t, e, p)

	if code := p.exitCode(t); code != 1 {
		t.Fatalf("exit code %d, want 1\nstderr:\n%s", code, p.stderr)
	}
	if n := bot.callsOf("getUpdates"); n != 1 {
		t.Errorf("getUpdates called %d times, want 1: polling goes on after a 401", n)
	}
	if !strings.Contains(p.stderr.String(), updates.ErrUnauthorized.Error()) {
		t.Errorf("stderr:\n%s", p.stderr)
	}
	assertNoSecret(t, "stderr", p.stderr.String())
	assertNoSecret(t, "the log", p.stdout.String())
}

// ADR-018 p. 6: a 409 of getUpdates exits with code 3.
func TestAConflictOfGetUpdatesExitsWithThree(t *testing.T) {
	bot := newBotAPI(t)
	bot.polls = []apiAnswer{conflict}
	gw := newGatewayMock(t)
	e, p := testEnvironment(bot, vars(bot, gw))
	start(t, e, p)

	if code := p.exitCode(t); code != updates.ExitConflict {
		t.Fatalf("exit code %d, want %d\nstderr:\n%s", code, updates.ExitConflict, p.stderr)
	}
	assertNoSecret(t, "stderr", p.stderr.String())
	assertNoSecret(t, "the log", p.stdout.String())
}

// Review #1 of T-310, M-2 and Mi-4 of review #2: each sender repeats by its
// own policy — a refusal of the gate is tried once, an answer of the flow
// twice (ReplyPolicy), a delivery three times (DefaultPolicy) and is not
// acknowledged. /health counts the refusal; a signal stops the bot with 0.
func TestEachSenderRepeatsByItsOwnPolicy(t *testing.T) {
	bot := newBotAPI(t)
	bot.polls = []apiAnswer{updatesOf(msg{stranger, "/start"}, msg{playerA, "/start"})}
	for _, chat := range []int64{stranger, playerA, deliveryChat} {
		bot.sends[strconv.FormatInt(chat, 10)] = []apiAnswer{serverError}
	}
	gw := newGatewayMock(t)
	gw.deliveries = []api.Delivery{{
		ID: "d-1", PlayerID: "player-X", Kind: "mechanics", GeneratedBy: "rules", Text: "Удар.",
		Route: &api.DeliveryRoute{ExternalPlatform: "telegram", ExternalID: strconv.FormatInt(deliveryChat, 10)},
	}}
	e, p := testEnvironment(bot, vars(bot, gw))
	start(t, e, p)

	waitUntil(t, "three tries of the delivery, two of the answer, one of the refusal", func() bool {
		return bot.triesOf(deliveryChat) >= 3 && bot.triesOf(playerA) >= 2 && bot.triesOf(stranger) >= 1
	})
	time.Sleep(200 * time.Millisecond)
	if got := [3]int{bot.triesOf(stranger), bot.triesOf(playerA), bot.triesOf(deliveryChat)}; got != [3]int{1, 2, 3} {
		t.Errorf("tries (refusal, answer, delivery) = %v, want [1 2 3]", got)
	}
	for _, r := range gw.requestsSeen() {
		if strings.HasSuffix(r, "/deliveries/ack") {
			t.Errorf("a delivery that Telegram did not take was acknowledged: %v", gw.requestsSeen())
		}
	}

	addr := <-p.addr
	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatal(err)
	}
	var h health
	_ = json.NewDecoder(resp.Body).Decode(&h)
	_ = resp.Body.Close()
	if h.Status != statusOK || h.Details["bot_denied_total"] != float64(1) {
		t.Errorf("/health = %+v, want ok with bot_denied_total 1", h)
	}
	he := environment{stdout: &syncBuffer{}, stderr: &syncBuffer{}, source: env.MapSource(nil)}
	if code := run(context.Background(), []string{"health", "--url", "http://" + addr + "/health"}, he); code != 0 {
		t.Errorf("telegram-bot health against the running bot: %d, stderr %s", code, he.stderr)
	}

	p.cancel()
	if code := p.exitCode(t); code != 0 {
		t.Errorf("exit code after the signal %d, want 0\nstderr:\n%s", code, p.stderr)
	}
	assertNoSecret(t, "the log", p.stdout.String())
}

// Review #2 of T-310, Mi-4: a refusal Telegram does not answer holds the next
// player no longer than the timeout of the sender of the gate. The sender of
// the flow and of the loop has an hour here: had the gate taken it, the player
// would wait an hour.
func TestARefusalTelegramDoesNotAnswerDoesNotHoldThePlayers(t *testing.T) {
	bot := newBotAPI(t)
	bot.polls = []apiAnswer{updatesOf(msg{stranger, "/start"}, msg{playerA, "/start"})}
	bot.hang[strconv.FormatInt(stranger, 10)] = true
	gw := newGatewayMock(t)
	e, p := testEnvironment(bot, vars(bot, gw))
	start(t, e, p)

	wait, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case <-gw.resolvedCh(playerA):
	case <-wait.Done():
		t.Fatalf("the command of the player did not reach the gateway within 3 s behind a refusal Telegram does not answer (gate timeout %s)", testLimits.GateTimeout)
	}
	if bot.triesOf(stranger) != 1 {
		t.Errorf("refusal tries %d, want 1", bot.triesOf(stranger))
	}
}

// Acceptance of T-312, Mi-4 of review #1: an answer of the flow Telegram does
// not take holds the next player no longer than the client of the flow allows —
// its timeout times the attempts of sender.ReplyPolicy. The client of the loop
// has an hour here: had the flow taken it, the player would wait an hour.
func TestAnAnswerTelegramDoesNotTakeHoldsTheNextPlayerOnlyForTheFlowClient(t *testing.T) {
	bot := newBotAPI(t)
	bot.polls = []apiAnswer{updatesOf(msg{playerA, "/start"}, msg{playerB, "/start"})}
	bot.hang[strconv.FormatInt(playerA, 10)] = true
	gw := newGatewayMock(t)
	e, p := testEnvironment(bot, vars(bot, gw))
	start(t, e, p)

	wait, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case <-gw.resolvedCh(playerB):
	case <-wait.Done():
		t.Fatalf("the command of the next player did not reach the gateway within 3 s behind an answer Telegram does not take (flow timeout %s)", testLimits.FlowTelegramTimeout)
	}
	if n := bot.triesOf(playerA); n != sender.ReplyPolicy.Attempts {
		t.Errorf("answer tries %d, want %d of sender.ReplyPolicy", n, sender.ReplyPolicy.Attempts)
	}
}

// Review #1 of T-311, risk 6: a gateway that does not answer one player holds
// the next one no longer than the client of the flow allows — its timeout
// times its attempts. The client of the loop has an hour here.
func TestAGatewayThatDoesNotAnswerHoldsTheNextPlayerOnlyForTheFlowClient(t *testing.T) {
	bot := newBotAPI(t)
	bot.polls = []apiAnswer{updatesOf(msg{playerA, "/start"}, msg{playerB, "/start"})}
	gw := newGatewayMock(t)
	gw.hangFor[strconv.FormatInt(playerA, 10)] = true
	e, p := testEnvironment(bot, vars(bot, gw))
	start(t, e, p)

	bound := testLimits.FlowGatewayTimeout * time.Duration(testLimits.FlowGatewayBackoff.Retries+1)
	wait, cancel := context.WithTimeout(context.Background(), bound+2*time.Second)
	defer cancel()
	select {
	case <-gw.resolvedCh(playerB):
	case <-wait.Done():
		t.Fatalf("the command of the next player did not reach the gateway within %s", bound+2*time.Second)
	}
}

// Review #1 of T-311, Mi-6: the flow, the gate and the loop log through
// privacy.Handler; the loop and the gate get the senders meant for them.
func TestTheWiringHandsOutThePrivacyLoggerAndTheRightClients(t *testing.T) {
	bot := newBotAPI(t)
	gw := newGatewayMock(t)
	e, p := testEnvironment(bot, vars(bot, gw))
	cfg, err := config.Load(e.source)
	if err != nil {
		t.Fatal(err)
	}
	b, err := build(cfg, e, logging.New(logging.Options{Service: ClientID, Output: p.stdout}))
	if err != nil {
		t.Fatal(err)
	}
	for name, h := range map[string]any{
		"flow": b.flowOpts.Log.Handler(), "access": b.accOpts.Log.Handler(), "deliver": b.loopOpts.Log.Handler(),
		"updates": b.srcOpts.Log.Handler(),
	} {
		if _, isPrivacy := h.(*privacy.Handler); !isPrivacy {
			t.Errorf("the logger of %s is built on %T, want *privacy.Handler", name, h)
		}
	}
	flowGW, _ := b.flowOpts.Gateway.(*client.Client)
	loopGW, _ := b.loopOpts.Gateway.(*client.Client)
	if flowGW == nil || loopGW == nil || flowGW == loopGW || flowGW.HTTP == loopGW.HTTP {
		t.Fatalf("the flow and the loop share a gateway client: %p %p", flowGW, loopGW)
	}
	if flowGW.HTTP.Timeout != testLimits.FlowGatewayTimeout || flowGW.Backoff != testLimits.FlowGatewayBackoff {
		t.Errorf("client of the flow: timeout %s, backoff %+v", flowGW.HTTP.Timeout, flowGW.Backoff)
	}
	if loopGW.HTTP.Timeout != testLimits.DeliverGatewayTimeout || flowGW.ClientID != ClientID || loopGW.ClientID != ClientID {
		t.Errorf("client of the loop: timeout %s, client ids %q %q", loopGW.HTTP.Timeout, flowGW.ClientID, loopGW.ClientID)
	}
	if b.accOpts.Sender == b.loopOpts.Sender || b.accOpts.Sender == b.flowOpts.Sender || b.flowOpts.Sender == b.loopOpts.Sender {
		t.Error("the gate, the flow and the loop do not each have a sender of their own")
	}
	for name, s := range map[string]any{"loop": b.loopOpts.Sender, "flow": b.flowOpts.Sender} {
		if _, isTelegram := s.(*sender.Telegram); !isTelegram {
			t.Errorf("the sender of the %s is %T", name, s)
		}
	}
	if b.srcOpts.OnReady == nil {
		t.Error("the update source has no OnReady: the delivery loop would never start")
	}
}

// The limits the bot ships with keep the promises of the reviews: a refusal
// gives up within BestEffortHTTPTimeout; the flow waits a little longer than
// the gateway lets a request run, at most 10 s a try; the loop outlasts its
// long-poll.
func TestTheProductionLimits(t *testing.T) {
	l := productionLimits
	if l.GateTimeout <= 0 || l.GateTimeout > sender.BestEffortHTTPTimeout {
		t.Errorf("GateTimeout %s, want (0, %s]", l.GateTimeout, sender.BestEffortHTTPTimeout)
	}
	if l.FlowGatewayTimeout <= api.RequestTimeout || l.FlowGatewayTimeout > 10*time.Second {
		t.Errorf("FlowGatewayTimeout %s, want (%s, 10s]", l.FlowGatewayTimeout, api.RequestTimeout)
	}
	if l.FlowGatewayBackoff.Retries < 0 || l.FlowGatewayBackoff.Retries > 1 || l.FlowGatewayBackoff == (client.Backoff{}) {
		t.Errorf("FlowGatewayBackoff %+v: at most one repeat, set explicitly", l.FlowGatewayBackoff)
	}
	if l.DeliverGatewayTimeout <= client.MaxPollWait {
		t.Errorf("DeliverGatewayTimeout %s cuts the long-poll of %s", l.DeliverGatewayTimeout, client.MaxPollWait)
	}
	if l.TelegramTimeout != sender.DefaultHTTPTimeout {
		t.Errorf("TelegramTimeout %s, want %s", l.TelegramTimeout, sender.DefaultHTTPTimeout)
	}
	if l.FlowTelegramTimeout <= 0 || l.FlowTelegramTimeout > 10*time.Second {
		t.Errorf("FlowTelegramTimeout %s, want (0, 10s]", l.FlowTelegramTimeout)
	}
}

func TestHealthCommand(t *testing.T) {
	answer := func(status int, body string) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(srv.Close)
		return srv.URL + "/health"
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL + "/health"
	closed.Close()

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"ok", []string{"--url", answer(http.StatusOK, `{"status":"ok","details":{"bot_denied_total":0}}`)}, 0},
		{"degraded", []string{"--url", answer(http.StatusOK, `{"status":"degraded"}`)}, 1},
		{"fail", []string{"--url", answer(http.StatusServiceUnavailable, `{"status":"fail"}`)}, 1},
		{"not json", []string{"--url", answer(http.StatusOK, `<html>`)}, 1},
		{"unreachable", []string{"--url", closedURL}, 1},
		{"extra argument", []string{"--url", closedURL, "more"}, 2},
		{"unknown flag", []string{"--nope"}, 2},
	}
	for _, c := range cases {
		e := environment{stdout: &syncBuffer{}, stderr: &syncBuffer{}, source: env.MapSource(nil)}
		if got := run(context.Background(), append([]string{"health"}, c.args...), e); got != c.want {
			t.Errorf("%s: exit code %d, want %d (stderr %s)", c.name, got, c.want, e.stderr)
		}
	}
}

// docker-compose.bot.yml: the default --url is built from
// MV_TELEGRAM_HEALTH_ADDR, a wildcard host dialled on loopback.
func TestTheDefaultHealthURLFollowsTheHealthAddress(t *testing.T) {
	cases := map[string]string{
		":8089":          "http://127.0.0.1:8089/health",
		"0.0.0.0:8089":   "http://127.0.0.1:8089/health",
		"[::]:8089":      "http://127.0.0.1:8089/health",
		"127.0.0.1:9000": "http://127.0.0.1:9000/health",
		"bot.local:8089": "http://bot.local:8089/health",
		"[::1]:8089":     "http://[::1]:8089/health",
	}
	for addr, want := range cases {
		if got := defaultHealthURL(addr); got != want {
			t.Errorf("defaultHealthURL(%q) = %q, want %q", addr, got, want)
		}
	}
	if got := defaultHealthURL(env.TelegramHealthAddr.Default()); got != "http://127.0.0.1:8089/health" {
		t.Errorf("the default of the manifest gives %q", got)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()
	e := environment{stdout: &syncBuffer{}, stderr: &syncBuffer{}, source: env.MapSource(map[string]string{
		"MV_TELEGRAM_HEALTH_ADDR": strings.TrimPrefix(srv.URL, "http://"),
	})}
	if code := run(context.Background(), []string{"health"}, e); code != 0 {
		t.Errorf("health without --url against MV_TELEGRAM_HEALTH_ADDR: %d, stderr %s", code, e.stderr)
	}
}

func TestAnUnknownCommandExitsWithTwo(t *testing.T) {
	e := environment{stdout: &syncBuffer{}, stderr: &syncBuffer{}, source: env.MapSource(nil)}
	if code := run(context.Background(), []string{"helth"}, e); code != 2 {
		t.Errorf("exit code %d, want 2", code)
	}
}
