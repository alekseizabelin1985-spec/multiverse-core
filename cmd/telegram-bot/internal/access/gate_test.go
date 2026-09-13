package access_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/access"
	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/logging"
)

const (
	allowedUser  = int64(111222333)
	strangerUser = int64(987654321)
	groupChat    = int64(-100500600700)
)

var epoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// gatewayMock counts every request the bot makes to the gateway: an update
// the gate refuses must leave it at zero.
type gatewayMock struct {
	srv   *httptest.Server
	calls atomic.Int32
	paths chan string
}

func newGatewayMock(t *testing.T) *gatewayMock {
	g := &gatewayMock{paths: make(chan string, 64)}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.calls.Add(1)
		g.paths <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"link_status":"pending_consent","player_id":null,"world_id":null,"character_status":"none","notice_due":true}`))
	}))
	t.Cleanup(g.srv.Close)
	return g
}

// resolveLink is what the flow does first with an admitted update: POST
// /v1/links/resolve (component §7.1). The flow itself is T-311; the gate is
// only allowed to reach it for an admitted update.
func (g *gatewayMock) resolveLink(t *testing.T) updates.Handler {
	return func(ctx context.Context, u updates.Update) {
		body := `{"external_platform":"telegram","external_id":"` + strconv.FormatInt(u.Message.From.ID, 10) + `"}`
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.srv.URL+"/v1/links/resolve", strings.NewReader(body))
		if err != nil {
			t.Error(err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		_ = resp.Body.Close()
	}
}

type fixture struct {
	gate    *access.Gate
	clock   *clock.Manual
	sent    *sender.Fake
	gateway *gatewayMock
	logs    *bytes.Buffer
	handler updates.Handler
}

func newFixture(t *testing.T, allowed []int64, perMinute int) *fixture {
	t.Helper()
	f := &fixture{clock: clock.NewManual(epoch), sent: &sender.Fake{}, gateway: newGatewayMock(t), logs: &bytes.Buffer{}}
	inner := logging.New(logging.Options{Service: "telegram-bot", Level: slog.LevelDebug, Output: f.logs}).Handler()
	gate, err := access.New(access.Options{
		AllowedUserIDs:    allowed,
		CommandsPerMinute: perMinute,
		Clock:             f.clock,
		Sender:            f.sent,
		Log:               slog.New(privacy.NewHandler(inner, nil)),
	})
	if err != nil {
		t.Fatalf("access.New: %v", err)
	}
	f.gate = gate
	f.handler = gate.Wrap(f.gateway.resolveLink(t))
	return f
}

func private(updateID, user int64, text string) updates.Update {
	return updates.Update{ID: updateID, Message: &updates.Message{
		Chat: updates.Chat{ID: user, Type: updates.ChatPrivate},
		From: &updates.User{ID: user, Username: "someone"},
		Text: text,
	}}
}

func (f *fixture) texts() []string {
	var out []string
	for _, m := range f.sent.Sent() {
		out = append(out, m.Text)
	}
	return out
}

// The reply is pinned as a literal: US-008 names it, and a test against the
// constant would pass whatever the constant said.
func TestRepliesSayWhatUS008Names(t *testing.T) {
	if access.TextInviteOnly != "Доступ по приглашению." {
		t.Fatalf("reply %q", access.TextInviteOnly)
	}
}

func TestSEC06AStrangerIsRefusedBeforeTheGateway(t *testing.T) {
	f := newFixture(t, []int64{allowedUser}, 20)
	f.handler(context.Background(), private(1, strangerUser, "/start"))

	if n := f.gateway.calls.Load(); n != 0 {
		t.Fatalf("links/resolve called %d times for a stranger: the gate runs before any gateway call", n)
	}
	sent := f.sent.Sent()
	if len(sent) != 1 || sent[0].Text != access.TextInviteOnly || sent[0].ChatID != strangerUser || sent[0].Keyboard != nil {
		t.Fatalf("stranger got %d messages, want exactly one «%s»", len(sent), access.TextInviteOnly)
	}
	if c := f.gate.Counters(); c.Denied != 1 {
		t.Fatalf("bot_denied_total = %d, want 1", c.Denied)
	}
	if out := f.logs.String(); strings.Contains(out, strconv.FormatInt(strangerUser, 10)) || !strings.Contains(out, "not_allowed") {
		t.Fatalf("the refusal must be logged by reason and without the id:\n%s", out)
	}

	f.handler(context.Background(), private(2, allowedUser, "/start"))
	if n := f.gateway.calls.Load(); n != 1 {
		t.Fatalf("an allowed user must reach the gateway: %d calls", n)
	}
	if path := <-f.gateway.paths; path != "/v1/links/resolve" {
		t.Fatalf("gateway called at %s", path)
	}
}

func TestSEC06AnEmptyAllowListRefusesEveryone(t *testing.T) {
	f := newFixture(t, nil, 20)
	for i, user := range []int64{allowedUser, strangerUser, 1} {
		f.handler(context.Background(), private(int64(i), user, "/start"))
	}
	if n := f.gateway.calls.Load(); n != 0 {
		t.Fatalf("gateway called %d times with an empty allow-list", n)
	}
	if got := f.texts(); len(got) != 3 || got[0] != access.TextInviteOnly || got[2] != access.TextInviteOnly {
		t.Fatalf("replies %q, want three refusals", got)
	}
	if c := f.gate.Counters(); c.Denied != 3 {
		t.Fatalf("bot_denied_total = %d, want 3", c.Denied)
	}
}

// TestSEC07AGroupChatGetsSilence: in a group the bot neither answers nor calls
// the gateway, for a player of the allow-list and for a stranger alike
// (US-008, FR-131, NFR-044), and logs one line without the chat id.
func TestSEC07AGroupChatGetsSilence(t *testing.T) {
	f := newFixture(t, []int64{allowedUser}, 20)
	groupMessage := func(chat int64, chatType string, user int64) {
		f.handler(context.Background(), updates.Update{ID: 5, Message: &updates.Message{
			Chat: updates.Chat{ID: chat, Type: chatType},
			From: &updates.User{ID: user},
			Text: "/attack wolf-alpha",
		}})
	}
	// Each sender opens a series of a group of its own first, the stranger
	// before the player (review #2 of T-310, N-8, R8b): the opener of a series
	// gets no reply either. Then both write again into the same groups.
	series := 0
	for _, chatType := range []string{"group", "supergroup", "channel"} {
		for _, user := range []int64{strangerUser, allowedUser} {
			groupMessage(groupChat-int64(series), chatType, user)
			series++
		}
	}
	for i, chatType := range []string{"group", "supergroup", "channel"} {
		for _, user := range []int64{strangerUser, allowedUser} {
			groupMessage(groupChat-int64(2*i), chatType, user)
		}
	}
	if n := f.gateway.calls.Load(); n != 0 {
		t.Fatalf("gateway called %d times for group messages", n)
	}
	if sent := f.sent.Sent(); len(sent) != 0 {
		t.Fatalf("%d replies to group messages, want none", len(sent))
	}
	if c := f.gate.Counters(); c.NotPrivate != 12 || c.Denied != 0 || c.TooOften != 0 {
		t.Fatalf("counters %+v", c)
	}
	out := f.logs.String()
	if strings.Contains(out, "10050060") || strings.Contains(out, strconv.FormatInt(strangerUser, 10)) || strings.Contains(out, strconv.FormatInt(allowedUser, 10)) {
		t.Fatalf("an id in the log:\n%s", out)
	}
	if n := strings.Count(out, "update refused"); n != series || strings.Count(out, "not_private") != series {
		t.Fatalf("%d log lines for %d groups, want one per group:\n%s", n, series, out)
	}
}

func TestSEC07IdentityIsTheSenderNotTheChat(t *testing.T) {
	f := newFixture(t, []int64{allowedUser}, 20)
	// A private chat whose id is allowed but whose sender is not: nothing may
	// be attributed to the chat.
	f.handler(context.Background(), updates.Update{ID: 1, Message: &updates.Message{
		Chat: updates.Chat{ID: allowedUser, Type: updates.ChatPrivate},
		From: &updates.User{ID: strangerUser},
		Text: "/start",
	}})
	f.handler(context.Background(), updates.Update{ID: 2})
	f.handler(context.Background(), updates.Update{ID: 3, Message: &updates.Message{Chat: updates.Chat{ID: allowedUser, Type: updates.ChatPrivate}}})
	if n := f.gateway.calls.Load(); n != 0 || len(f.sent.Sent()) != 0 {
		t.Fatalf("%d gateway calls and %d replies for updates without a matching sender", n, len(f.sent.Sent()))
	}
}

func TestSEC11The21stCommandWaitsUntilTheWindowMoves(t *testing.T) {
	f := newFixture(t, []int64{allowedUser, strangerUser + 1}, 20)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		f.handler(ctx, private(int64(i), allowedUser, "/look"))
		f.clock.Advance(time.Second)
	}
	if n := f.gateway.calls.Load(); n != 20 {
		t.Fatalf("20 commands within a minute: %d reached the gateway", n)
	}

	// t = 20 s: the 21st command within the minute.
	f.handler(ctx, private(21, allowedUser, "/look"))
	if n := f.gateway.calls.Load(); n != 20 {
		t.Fatal("the 21st command within a minute reached the gateway")
	}
	sent := f.sent.Sent()
	if len(sent) != 1 || sent[0].Text != "Слишком часто, подождите 40 с." || sent[0].ChatID != allowedUser {
		t.Fatalf("replies %q, want «Слишком часто, подождите 40 с.»", f.texts())
	}

	// A flood is answered once, and another user is not affected.
	f.clock.Set(epoch.Add(59 * time.Second))
	f.handler(ctx, private(22, allowedUser, "/look"))
	f.handler(ctx, private(23, strangerUser+1, "/look"))
	if n := f.gateway.calls.Load(); n != 21 || len(f.sent.Sent()) != 1 {
		t.Fatalf("at 59 s: %d gateway calls and %d replies, want 21 (the other user) and 1", n, len(f.sent.Sent()))
	}
	// And logged once (review #2 of T-310, N-8, X11).
	if n := strings.Count(f.logs.String(), "update refused"); n != 1 || !strings.Contains(f.logs.String(), "too_often") {
		t.Fatalf("%d log lines for one series of «too often», want 1:\n%s", n, f.logs.String())
	}

	// At the 61st second the first command left the window.
	f.clock.Set(epoch.Add(60*time.Second + 500*time.Millisecond))
	f.handler(ctx, private(24, allowedUser, "/look"))
	if n := f.gateway.calls.Load(); n != 22 {
		t.Fatalf("at the 61st second the command must be accepted: %d gateway calls", n)
	}
	if c := f.gate.Counters(); c.TooOften != 2 {
		t.Fatalf("too_often counter = %d, want 2", c.TooOften)
	}

	// After an admitted command the next refusal is answered again.
	f.handler(ctx, private(25, allowedUser, "/look"))
	if got := f.texts(); len(got) != 2 || got[1] != "Слишком часто, подождите 1 с." {
		t.Fatalf("replies %q, want a second warning with 1 s", got)
	}
	if n := strings.Count(f.logs.String(), "update refused"); n != 2 {
		t.Fatalf("%d log lines after a second series, want 2", n)
	}
}

func TestSEC11ARefusedStrangerDoesNotSpendTheLimit(t *testing.T) {
	f := newFixture(t, nil, 1)
	for i := 0; i < 5; i++ {
		f.handler(context.Background(), private(int64(i), strangerUser, "/look"))
	}
	if c := f.gate.Counters(); c.TooOften != 0 || c.Denied != 5 {
		t.Fatalf("counters %+v: the limit applies after the allow-list", c)
	}
}

func TestTooOftenTextRoundsUp(t *testing.T) {
	for wait, want := range map[time.Duration]string{
		60 * time.Second:        "Слишком часто, подождите 60 с.",
		1500 * time.Millisecond: "Слишком часто, подождите 2 с.",
		0:                       "Слишком часто, подождите 1 с.",
	} {
		if got := access.TooOftenText(wait); got != want {
			t.Errorf("TooOftenText(%s) = %q, want %q", wait, got, want)
		}
	}
}

func TestAFailedRefusalIsLoggedWithoutTheID(t *testing.T) {
	f := newFixture(t, nil, 20)
	f.sent.FailNext(errors.New("telegram: the chat refuses messages from the bot (403): Forbidden"))
	f.handler(context.Background(), private(1, strangerUser, "/start"))
	out := f.logs.String()
	if !strings.Contains(out, "refusal not delivered") || strings.Contains(out, strconv.FormatInt(strangerUser, 10)) {
		t.Fatalf("log:\n%s", out)
	}
}

func TestNewNeedsClockAndSender(t *testing.T) {
	if _, err := access.New(access.Options{Sender: &sender.Fake{}}); err == nil {
		t.Fatal("New without a clock must fail")
	}
	if _, err := access.New(access.Options{Clock: clock.NewManual(epoch)}); err == nil {
		t.Fatal("New without a sender must fail")
	}
	gate, err := access.New(access.Options{AllowedUserIDs: []int64{allowedUser}, Clock: clock.NewManual(epoch), Sender: &sender.Fake{}})
	if err != nil {
		t.Fatal(err)
	}
	if v := gate.Check(private(1, allowedUser, "/look")); v.Outcome != access.Admit {
		t.Fatalf("a limit below 1 means 1: first command %v", v.Outcome)
	}
	if v := gate.Check(private(2, allowedUser, "/look")); v.Outcome != access.TooOften || !v.Notify || v.Wait != access.Window {
		t.Fatalf("second command %+v", v)
	}
	if access.Admit.String() != "admit" || access.TooOften.String() != "too_often" {
		t.Fatal("outcome names are what the log says")
	}
}

// TestAFloodFromAStrangerIsAnsweredAndLoggedOncePerWindow: 30 messages of one
// stranger within a second get one reply and one log line, and the command of
// an invited player right behind them reaches the gateway. A new series opens
// exactly one Window after the first.
func TestAFloodFromAStrangerIsAnsweredAndLoggedOncePerWindow(t *testing.T) {
	f := newFixture(t, []int64{allowedUser}, 20)
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		f.handler(ctx, private(int64(i), strangerUser, "/start"))
		f.clock.Advance(30 * time.Millisecond)
	}
	f.handler(ctx, private(100, allowedUser, "/look"))

	if got := f.texts(); len(got) != 1 || got[0] != access.TextInviteOnly {
		t.Fatalf("replies %q, want one «%s» for the whole flood", got, access.TextInviteOnly)
	}
	if n := f.gateway.calls.Load(); n != 1 {
		t.Fatalf("the invited player reached the gateway %d times, want 1", n)
	}
	if n := strings.Count(f.logs.String(), "update refused"); n != 1 {
		t.Fatalf("%d log lines for one flood, want 1:\n%s", n, f.logs.String())
	}
	if c := f.gate.Counters(); c.Denied != 30 {
		t.Fatalf("bot_denied_total = %d, want every refused update counted (30)", c.Denied)
	}

	f.clock.Set(epoch.Add(access.Window - time.Nanosecond))
	f.handler(ctx, private(31, strangerUser, "/start"))
	if n := len(f.sent.Sent()); n != 1 {
		t.Fatalf("%d replies just before the Window ends, want still 1", n)
	}
	f.clock.Set(epoch.Add(access.Window))
	f.handler(ctx, private(32, strangerUser, "/start"))
	if n := len(f.sent.Sent()); n != 2 {
		t.Fatalf("%d replies once the Window ended, want a second one", n)
	}
}

// TestTheRefusalsOfManyChatsAreBounded: the gate remembers at most
// MaxRefusedChats chats; one more within the Window is refused in silence and
// without a log line, and is answered once the Window of the others is over.
// Replies stay within MaxInviteRepliesPerWindow all along.
func TestTheRefusalsOfManyChatsAreBounded(t *testing.T) {
	f := newFixture(t, nil, 20)
	ctx := context.Background()
	for i := 0; i < access.MaxRefusedChats; i++ {
		f.handler(ctx, private(int64(i), strangerUser+int64(i), "/start"))
	}
	one := strangerUser + int64(access.MaxRefusedChats)
	f.handler(ctx, private(9000, one, "/start"))
	if n := len(f.sent.Sent()); n != access.MaxInviteRepliesPerWindow {
		t.Fatalf("%d replies, want %d: the budget of the bot", n, access.MaxInviteRepliesPerWindow)
	}
	out := f.logs.String()
	if n := strings.Count(out, "update refused"); n != access.MaxRefusedChats {
		t.Fatalf("%d log lines, want %d: a chat beyond the table gets none", n, access.MaxRefusedChats)
	}
	if n := strings.Count(out, `"answered":false`); n != access.MaxRefusedChats-access.MaxInviteRepliesPerWindow {
		t.Fatalf("%d series logged as unanswered, want %d", n, access.MaxRefusedChats-access.MaxInviteRepliesPerWindow)
	}
	f.clock.Advance(access.Window)
	f.handler(ctx, private(9001, one, "/start"))
	sent := f.sent.Sent()
	if len(sent) != access.MaxInviteRepliesPerWindow+1 || sent[len(sent)-1].ChatID != one {
		t.Fatalf("%d replies: the table must make room once the Window of the others is over", len(sent))
	}
}

// TestInviteRepliesOfTheWholeBotAreBudgeted (review #2 of T-310, Mi-4): many
// strangers, one message each, get at most MaxInviteRepliesPerWindow replies
// within a Window; every refusal is still counted; the hint «Слишком часто» to
// a player is not taken away by them; the budget is back one Window after the
// first reply.
func TestInviteRepliesOfTheWholeBotAreBudgeted(t *testing.T) {
	f := newFixture(t, []int64{allowedUser}, 1)
	ctx := context.Background()
	strangers := access.MaxInviteRepliesPerWindow + 5
	for i := 0; i < strangers; i++ {
		f.handler(ctx, private(int64(i), strangerUser+int64(i), "/start"))
		f.clock.Advance(time.Second)
	}
	if n := len(f.sent.Sent()); n != access.MaxInviteRepliesPerWindow {
		t.Fatalf("%d replies to %d strangers, want %d", n, strangers, access.MaxInviteRepliesPerWindow)
	}
	if c := f.gate.Counters(); c.Denied != uint64(strangers) {
		t.Fatalf("bot_denied_total = %d, want %d", c.Denied, strangers)
	}
	f.handler(ctx, private(100, allowedUser, "/look"))
	f.handler(ctx, private(101, allowedUser, "/look"))
	sent := f.sent.Sent()
	if len(sent) != access.MaxInviteRepliesPerWindow+1 || sent[len(sent)-1].ChatID != allowedUser || !strings.HasPrefix(sent[len(sent)-1].Text, "Слишком часто") {
		t.Fatalf("replies %q: the player must still be told to wait", f.texts())
	}

	// The first reply went out at epoch: a Window later there is room again.
	f.clock.Set(epoch.Add(access.Window - time.Nanosecond))
	f.handler(ctx, private(200, strangerUser+1000, "/start"))
	if n := len(f.sent.Sent()); n != access.MaxInviteRepliesPerWindow+1 {
		t.Fatalf("%d replies a nanosecond before the budget frees, want %d", n, access.MaxInviteRepliesPerWindow+1)
	}
	f.clock.Set(epoch.Add(access.Window))
	f.handler(ctx, private(201, strangerUser+1001, "/start"))
	if sent := f.sent.Sent(); len(sent) != access.MaxInviteRepliesPerWindow+2 || sent[len(sent)-1].ChatID != strangerUser+1001 {
		t.Fatalf("%d replies once the first left the Window, want %d", len(sent), access.MaxInviteRepliesPerWindow+2)
	}
}

// hangingBotAPI takes a sendMessage and never answers it until the test ends.
func hangingBotAPI(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	release := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(api.Close)
	t.Cleanup(func() { close(release) })
	return api, &requests
}

// TestASlowTelegramHoldsTheGateNoLongerThanTheClientTimeout (review #2 of
// T-310, Mi-4): Telegram takes a refusal and does not answer. The sender of
// NewBestEffortTelegram gives up after the Timeout of its own client — here
// cut to 100 ms, in the process at most sender.BestEffortHTTPTimeout — and the
// command of an invited player right behind reaches the gateway.
func TestASlowTelegramHoldsTheGateNoLongerThanTheClientTimeout(t *testing.T) {
	api, requests := hangingBotAPI(t)
	tg, err := sender.NewBestEffortTelegram(sender.TelegramOptions{
		Token:      testToken,
		Timers:     &countingTimers{},
		ServerURL:  api.URL,
		HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway := newGatewayMock(t)
	gate, err := access.New(access.Options{AllowedUserIDs: []int64{allowedUser}, Clock: clock.NewManual(epoch), Sender: tg})
	if err != nil {
		t.Fatal(err)
	}
	handler := gate.Wrap(gateway.resolveLink(t))

	done := make(chan struct{})
	go func() {
		defer close(done)
		handler(context.Background(), private(1, strangerUser, "/start"))
		handler(context.Background(), private(2, allowedUser, "/look"))
	}()
	deadline, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-deadline.Done():
		t.Fatal("a refusal Telegram does not answer held the gate past the timeout of the client")
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("%d sendMessage for one refusal, want 1", n)
	}
	if n := gateway.calls.Load(); n != 1 {
		t.Fatalf("the invited player reached the gateway %d times, want 1", n)
	}
}

// TestTheLimitWindowEndsExactly: with a limit of 1 the second command waits
// until exactly one Window after the first.
func TestTheLimitWindowEndsExactly(t *testing.T) {
	f := newFixture(t, []int64{allowedUser}, 1)
	if v := f.gate.Check(private(1, allowedUser, "/look")); v.Outcome != access.Admit {
		t.Fatalf("first command %v", v.Outcome)
	}
	f.clock.Set(epoch.Add(access.Window - time.Nanosecond))
	if v := f.gate.Check(private(2, allowedUser, "/look")); v.Outcome != access.TooOften || v.Wait != time.Nanosecond {
		t.Fatalf("a nanosecond before the Window ends: %+v", v)
	}
	f.clock.Set(epoch.Add(access.Window))
	if v := f.gate.Check(private(3, allowedUser, "/look")); v.Outcome != access.Admit {
		t.Fatalf("exactly one Window later: %v, want admit", v.Outcome)
	}
}

// countingTimers fire at once and count: a refusal must arm none.
type countingTimers struct{ armed atomic.Int32 }

func (c *countingTimers) After(time.Duration) clock.Timer {
	c.armed.Add(1)
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return firedTimer{ch}
}

func (c *countingTimers) Every(time.Duration) clock.Timer { panic("no periodic timer expected") }

type firedTimer struct{ ch chan time.Time }

func (f firedTimer) C() <-chan time.Time { return f.ch }
func (f firedTimer) Stop() bool          { return false }

const testToken = "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"

// TestARateLimitedRefusalHoldsNobodyUp: Telegram answers the refusals with 429
// retry_after=30. With the sender the gate is given, the refusal is not waited
// out — no timer is armed — and the next command of an invited player reaches
// the gateway at once.
func TestARateLimitedRefusalHoldsNobodyUp(t *testing.T) {
	var sendMessages atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendMessages.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 30","parameters":{"retry_after":30}}`))
	}))
	t.Cleanup(api.Close)
	timers := &countingTimers{}
	tg, err := sender.NewBestEffortTelegram(sender.TelegramOptions{Token: testToken, Timers: timers, ServerURL: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	gateway := newGatewayMock(t)
	var logs bytes.Buffer
	gate, err := access.New(access.Options{
		AllowedUserIDs:    []int64{allowedUser},
		CommandsPerMinute: 20,
		Clock:             clock.NewManual(epoch),
		Sender:            tg,
		Log:               slog.New(privacy.NewHandler(slog.NewJSONHandler(&logs, nil), privacy.NewRedactor(testToken))),
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := gate.Wrap(gateway.resolveLink(t))
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		handler(ctx, private(int64(i), strangerUser, "/start"))
	}
	handler(ctx, private(100, allowedUser, "/look"))

	if n := timers.armed.Load(); n != 0 {
		t.Fatalf("%d pauses armed for refusals: a 429 on a refusal must not be waited out", n)
	}
	if n := sendMessages.Load(); n != 1 {
		t.Fatalf("%d sendMessage for a flood of 30, want 1", n)
	}
	if n := gateway.calls.Load(); n != 1 {
		t.Fatalf("the invited player reached the gateway %d times, want 1", n)
	}
	if out := logs.String(); !strings.Contains(out, "refusal not delivered") || strings.Contains(out, strconv.FormatInt(strangerUser, 10)) {
		t.Fatalf("the undelivered refusal must be logged without the id:\n%s", out)
	}
}
