package deliver_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/deliver"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
)

func mustOnce(t *testing.T, l *deliver.Loop) {
	t.Helper()
	if err := l.Once(context.Background()); err != nil {
		t.Fatalf("Once: %v", err)
	}
}

// The messages of a chat go out in the order of the gateway, and one ack for
// the whole answer follows the last message of it (component §10.4).
func TestMessagesKeepTheOrderOfTheGatewayAndOneAckFollowsTheAnswer(t *testing.T) {
	j := &journal{}
	ob := newOutbox(j)
	ob.enqueue(
		delivery("a1", "player-A", chatA, render.KindMechanics, "A: первое"),
		delivery("b1", "player-B", chatB, render.KindMechanics, "B: первое"),
		delivery("a2", "player-A", chatA, render.KindMechanics, "A: второе"),
		delivery("c1", "player-C", chatC, render.KindMechanics, "C: первое"),
		delivery("a3", "player-A", chatA, render.KindMechanics, "A: третье"),
	)
	s := &recordingSender{j: j}
	l := newLoop(t, ob, s, &instantTimers{}, nil)

	for range 3 {
		mustOnce(t, l)
	}

	a, b, c := strconv.FormatInt(chatA, 10), strconv.FormatInt(chatB, 10), strconv.FormatInt(chatC, 10)
	want := []string{
		"send " + a + " A: первое", "send " + b + " B: первое", "send " + c + " C: первое", "ack a1,b1,c1",
		"send " + a + " A: второе", "ack a2",
		"send " + a + " A: третье", "ack a3",
	}
	if got := j.all(); !reflect.DeepEqual(got, want) {
		t.Fatalf("journal:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A text longer than one message goes out as consecutive messages, all before
// the ack and before the next delivery.
func TestTheMessagesOfOneDeliveryGoOutTogetherBeforeTheAck(t *testing.T) {
	j := &journal{}
	ob := newOutbox(j)
	long := strings.Repeat("я", render.MaxMessageLen) + "\n\n" + "конец"
	ob.enqueue(
		delivery("a1", "player-A", chatA, render.KindMechanics, long),
		delivery("b1", "player-B", chatB, render.KindMechanics, "B"),
	)
	s := &recordingSender{j: j}
	mustOnce(t, newLoop(t, ob, s, &instantTimers{}, nil))

	got := j.all()
	if len(got) != 4 || !strings.HasPrefix(got[0], "send "+strconv.FormatInt(chatA, 10)+" яя") ||
		got[1] != "send "+strconv.FormatInt(chatA, 10)+" конец" || got[3] != "ack a1,b1" {
		t.Fatalf("journal: %q", got)
	}
}

// US-008, FR-003: the mechanics and the narrative of a turn reach the player as
// two messages, the narrative with the mark of its source.
func TestMechanicsAndNarrativeAreSeparateMessages(t *testing.T) {
	ob := newOutbox(nil)
	mech := delivery("m1", "player-A", chatA, render.KindMechanics, "Вы атакуете волка: попадание, урон 4.")
	narr := delivery("n1", "player-A", chatA, render.KindNarrative, "Клинок входит под рёбра зверя.")
	narr.GeneratedBy = render.GeneratedByLLM
	ob.enqueue(mech, narr)
	s := &sender.Fake{}
	l := newLoop(t, ob, s, &instantTimers{}, nil)

	mustOnce(t, l)
	mustOnce(t, l)

	sent := s.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2: %+v", len(sent), sent)
	}
	if sent[0].Text != mech.Text {
		t.Errorf("first message %q, want the mechanics alone", sent[0].Text)
	}
	if want := narr.Text + "\n" + render.MarkLLM; sent[1].Text != want {
		t.Errorf("second message %q, want the narrative with its mark %q", sent[1].Text, want)
	}
	if strings.Contains(sent[0].Text, narr.Text) || strings.Contains(sent[1].Text, mech.Text) {
		t.Errorf("mechanics and narrative share a message: %+v", sent)
	}
	if got := ob.ackCalls(); !reflect.DeepEqual(got, [][]string{{"m1"}, {"n1"}}) {
		t.Errorf("acks %v", got)
	}
}

// 429: the loop's sender waits out retry_after, repeats and the delivery is
// acknowledged.
func TestRateLimitIsWaitedOutThenTheMessageIsRepeated(t *testing.T) {
	bot := newBotAPI(t)
	bot.on(chatA,
		botAnswer{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 7","parameters":{"retry_after":7}}`},
		botOK)
	timers := &instantTimers{}
	ob := newOutbox(nil)
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "удар"))
	mustOnce(t, newLoop(t, ob, telegramSender(t, bot, timers), timers, nil))

	if got := bot.triesOf(chatA); got != 2 {
		t.Errorf("sendMessage tries %d, want 2", got)
	}
	if got := timers.all(); !reflect.DeepEqual(got, []time.Duration{7 * time.Second}) {
		t.Errorf("pauses %v, want [7s] of retry_after", got)
	}
	if got := ob.ackCalls(); !reflect.DeepEqual(got, [][]string{{"a1"}}) {
		t.Errorf("acks %v, want [[a1]]", got)
	}
}

// 5xx: three tries, no ack; the gateway hands the delivery out again once its
// lease of 30 s is over, and then it is sent and acknowledged.
func TestServerErrorsAreTriedThreeTimesAndLeftToTheGateway(t *testing.T) {
	bot := newBotAPI(t)
	bot.on(chatA, botAnswer{http.StatusInternalServerError, `{"ok":false,"error_code":500,"description":"Internal Server Error"}`})
	timers := &instantTimers{}
	ob := newOutbox(nil)
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "удар"))
	l := newLoop(t, ob, telegramSender(t, bot, timers), timers, nil)

	mustOnce(t, l)
	if got := bot.triesOf(chatA); got != 3 {
		t.Fatalf("sendMessage tries %d, want 3", got)
	}
	if got := ob.ackCalls(); len(got) != 0 {
		t.Fatalf("acks %v after a 5xx, want none", got)
	}

	ob.clock.Advance(lease - time.Second)
	mustOnce(t, l)
	if got := bot.triesOf(chatA); got != 3 {
		t.Fatalf("the delivery came again within its lease: %d tries", got)
	}

	bot.on(chatA, botOK)
	ob.clock.Advance(time.Second)
	mustOnce(t, l)
	if got := bot.triesOf(chatA); got != 4 {
		t.Fatalf("sendMessage tries %d after the lease, want 4", got)
	}
	if got := ob.ackCalls(); !reflect.DeepEqual(got, [][]string{{"a1"}}) {
		t.Errorf("acks %v, want [[a1]]", got)
	}
}

// 403 blocked and 400 chat not found: the player cannot be reached, the
// delivery is acknowledged, and no line of the log names the chat.
func TestUnreachablePlayersAreAcknowledgedWithoutTheirIDsInTheLog(t *testing.T) {
	bot := newBotAPI(t)
	bot.on(chatA, botAnswer{http.StatusForbidden, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user ` + strconv.FormatInt(chatA, 10) + `"}`})
	bot.on(chatB, botAnswer{http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`})
	timers := &instantTimers{}
	log, buf := logBuffer()
	ob := newOutbox(nil)
	ob.enqueue(
		delivery("a1", "player-A", chatA, render.KindMechanics, "секретный текст A"),
		delivery("b1", "player-B", chatB, render.KindNarrative, "секретный текст B"),
	)
	mustOnce(t, newLoop(t, ob, telegramSender(t, bot, timers), timers, log))

	if bot.triesOf(chatA) != 1 || bot.triesOf(chatB) != 1 {
		t.Errorf("tries A %d, B %d: a refusal for good is not repeated", bot.triesOf(chatA), bot.triesOf(chatB))
	}
	if got := ob.ackCalls(); !reflect.DeepEqual(got, [][]string{{"a1", "b1"}}) {
		t.Errorf("acks %v, want [[a1 b1]]", got)
	}
	out := buf.String()
	for _, reason := range []string{`"reason":"blocked"`, `"reason":"chat_not_found"`} {
		if !strings.Contains(out, reason) {
			t.Errorf("log has no %s:\n%s", reason, out)
		}
	}
	for _, leak := range []string{strconv.FormatInt(chatA, 10), strconv.FormatInt(chatB, 10), "секретный", testToken} {
		if strings.Contains(out, leak) {
			t.Errorf("log carries %q:\n%s", leak, out)
		}
	}
}

// Review #1 of T-310, Mi-1: the fate of a delivery follows the type of the
// error of the Sender.
func TestTheAckFollowsTheTypeOfTheSenderError(t *testing.T) {
	cases := []struct {
		err error
		ack bool
	}{
		{sender.ErrBlocked, true},
		{sender.ErrChatNotFound, true},
		{sender.ErrRejected, true},
		{sender.ErrUnavailable, false},
		{sender.ErrUnauthorized, false},
		{errors.New("something the loop does not know"), false},
	}
	for _, c := range cases {
		ob := newOutbox(nil)
		ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "удар"))
		s := &sender.Fake{}
		s.FailNext(c.err)
		mustOnce(t, newLoop(t, ob, s, &instantTimers{}, nil))
		acked := len(ob.unacked()) == 0
		if acked != c.ack {
			t.Errorf("%v: acknowledged %t, want %t", c.err, acked, c.ack)
		}
	}
}

// 401 at sendMessage: the token was revoked. Nothing of the answer is
// acknowledged, and the rest of it is not tried, since it would fail the same
// way; what was sent before stays acknowledged.
func TestARevokedTokenAcknowledgesNothingItDidNotSend(t *testing.T) {
	bot := newBotAPI(t)
	bot.on(chatB, botAnswer{http.StatusUnauthorized, `{"ok":false,"error_code":401,"description":"Unauthorized"}`})
	timers := &instantTimers{}
	log, buf := logBuffer()
	ob := newOutbox(nil)
	ob.enqueue(
		delivery("a1", "player-A", chatA, render.KindMechanics, "A"),
		delivery("b1", "player-B", chatB, render.KindMechanics, "B"),
		delivery("c1", "player-C", chatC, render.KindMechanics, "C"),
	)
	mustOnce(t, newLoop(t, ob, telegramSender(t, bot, timers), timers, log))

	if got := bot.triesOf(chatB); got != 1 {
		t.Errorf("tries on 401: %d, want 1", got)
	}
	if got := bot.triesOf(chatC); got != 0 {
		t.Errorf("the rest of the answer was tried after a 401: %d", got)
	}
	if got := ob.unacked(); !reflect.DeepEqual(got, []string{"b1", "c1"}) {
		t.Errorf("unacknowledged %v, want [b1 c1]", got)
	}
	if !strings.Contains(buf.String(), `"reason":"unauthorized"`) || strings.Contains(buf.String(), testToken) {
		t.Errorf("log:\n%s", buf.String())
	}
}

// A delivery without a Telegram route can never be sent by this bot: it is
// acknowledged rather than held for 24 h in front of the queue of its player.
func TestADeliveryWithoutATelegramRouteIsAcknowledgedUnsent(t *testing.T) {
	noRoute := delivery("r1", "player-A", chatA, render.KindMechanics, "x")
	noRoute.Route = nil
	otherPlatform := delivery("r2", "player-B", chatB, render.KindMechanics, "x")
	otherPlatform.Route.ExternalPlatform = "discord"
	badID := delivery("r3", "player-C", chatC, render.KindMechanics, "x")
	badID.Route.ExternalID = "not-a-number"

	ob := newOutbox(nil)
	ob.enqueue(noRoute, otherPlatform, badID)
	s := &sender.Fake{}
	mustOnce(t, newLoop(t, ob, s, &instantTimers{}, nil))

	if len(s.Sent()) != 0 {
		t.Errorf("sent %+v", s.Sent())
	}
	if got := ob.ackCalls(); !reflect.DeepEqual(got, [][]string{{"r1", "r2", "r3"}}) {
		t.Errorf("acks %v", got)
	}
}

// component §16 p. 5: the cursor is not kept across restarts. A new loop starts
// from an empty cursor, and a delivery the old one did not acknowledge comes
// again once its lease is over.
func TestARestartedLoopStartsWithoutACursorAndGetsTheUnacknowledgedAgain(t *testing.T) {
	ob := newOutbox(nil)
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "удар"), delivery("b1", "player-B", chatB, render.KindMechanics, "удар"))
	first := &sender.Fake{}
	first.FailNext(sender.ErrUnavailable)
	old := newLoop(t, ob, first, &instantTimers{}, nil)
	mustOnce(t, old)
	mustOnce(t, old)

	ob.clock.Advance(lease)
	second := &sender.Fake{}
	restarted := newLoop(t, ob, second, &instantTimers{}, nil)
	mustOnce(t, restarted)

	afters := ob.pollAfters()
	if len(afters) != 3 || afters[0] != "" || afters[1] == "" || afters[2] != "" {
		t.Errorf("after of the polls %q: the old loop passes its cursor on, the restarted one starts empty", afters)
	}
	sent := second.Sent()
	if len(sent) != 1 || sent[0].ChatID != chatA {
		t.Fatalf("the restarted loop sent %+v, want the unacknowledged delivery of A", sent)
	}
	if got := ob.unacked(); len(got) != 0 {
		t.Errorf("unacknowledged %v", got)
	}
}

// An ack the gateway did not take is repeated before the next long-poll, with
// the same ids. Acceptance of T-312, Mi-3 of review #1: the next long-poll
// fails here, so the repeated ack can only be the one sent before it — an ack
// left until after the poll would wait for up to 25 s and outlive the lease.
func TestAFailedAckIsRepeatedBeforeTheNextPoll(t *testing.T) {
	ob := newOutbox(nil)
	ob.ackErr = []error{errors.New("gateway POST /v1/clients/telegram-bot/deliveries/ack: attempt 4: connection refused")}
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "удар"))
	l := newLoop(t, ob, &sender.Fake{}, &instantTimers{}, nil)

	mustOnce(t, l)
	ob.pollErr = []error{errors.New("gateway GET /v1/clients/telegram-bot/deliveries: connection refused")}
	if err := l.Once(context.Background()); err == nil {
		t.Fatal("Once with a failed long-poll returned nil")
	}

	if got := ob.ackCalls(); !reflect.DeepEqual(got, [][]string{{"a1"}, {"a1"}}) {
		t.Errorf("acks %v, want the same ids twice, the second before the failed poll", got)
	}
	if got := ob.unacked(); len(got) != 0 {
		t.Errorf("unacknowledged %v", got)
	}
}

// Acceptance of T-312, N-2 of review #1: a delivery sent again after its ack
// was lost and its lease ran out is acknowledged once, not twice in one ack.
func TestADeliverySentAgainIsAcknowledgedOnce(t *testing.T) {
	refused := errors.New("gateway POST /v1/clients/telegram-bot/deliveries/ack: attempt 4: connection refused")
	ob := newOutbox(nil)
	ob.ackErr = []error{refused, refused}
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "удар"))
	s := &sender.Fake{}
	l := newLoop(t, ob, s, &instantTimers{}, nil)

	mustOnce(t, l)
	ob.clock.Advance(lease)
	mustOnce(t, l)

	if n := len(s.Sent()); n != 2 {
		t.Fatalf("sent %d messages, want the delivery twice", n)
	}
	if got := ob.ackCalls(); !reflect.DeepEqual(got, [][]string{{"a1"}, {"a1"}, {"a1"}}) {
		t.Errorf("acks %v, want [a1] three times, never [a1 a1]", got)
	}
}

// A stopping loop still acknowledges what it sent, under a context of its own.
func TestAStoppingLoopAcknowledgesWhatItSent(t *testing.T) {
	ob := newOutbox(nil)
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "A"), delivery("b1", "player-B", chatB, render.KindMechanics, "B"))
	ctx, cancel := context.WithCancel(context.Background())
	s := &cancellingSender{cancel: cancel}
	l := newLoop(t, ob, s, &instantTimers{}, nil)

	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()
	waitUntil(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	if got := ob.unacked(); !reflect.DeepEqual(got, []string{"b1"}) {
		t.Errorf("unacknowledged %v, want [b1]: a1 was sent before the stop", got)
	}
}

// Acceptance of T-312, Mi-2 of review #1: a sendMessage cut by the stop is not
// acknowledged — the message may not have reached the player, and the gateway
// hands the delivery out again after the restart.
func TestASendCutByTheStopIsNotAcknowledged(t *testing.T) {
	ob := newOutbox(nil)
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "A"), delivery("b1", "player-B", chatB, render.KindMechanics, "B"))
	ctx, cancel := context.WithCancel(context.Background())
	s := &stoppedInSendSender{cancel: cancel}
	l := newLoop(t, ob, s, &instantTimers{}, nil)

	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()
	waitUntil(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	if got := ob.unacked(); !reflect.DeepEqual(got, []string{"b1"}) {
		t.Errorf("unacknowledged %v, want [b1]: its send was cut by the stop", got)
	}
	for _, ids := range ob.ackCalls() {
		if slices.Contains(ids, "b1") {
			t.Errorf("an ack carries b1: %v", ob.ackCalls())
		}
	}
}

// stoppedInSendSender sends the first message; on the second it is stopped
// while Telegram has not answered yet: the context ends, and the send returns
// its error.
type stoppedInSendSender struct {
	sender.Fake
	cancel context.CancelFunc
	calls  int
}

func (s *stoppedInSendSender) Send(ctx context.Context, chatID int64, text string, kb *sender.Keyboard) error {
	s.calls++
	if s.calls == 1 {
		return s.Fake.Send(ctx, chatID, text, kb)
	}
	s.cancel()
	<-ctx.Done()
	return ctx.Err()
}

// cancellingSender sends the first message and ends the context of the loop
// with it, a SIGTERM that arrives right after a sendMessage.
type cancellingSender struct {
	sender.Fake
	cancel context.CancelFunc
	once   sync.Once
}

func (s *cancellingSender) Send(ctx context.Context, chatID int64, text string, kb *sender.Keyboard) error {
	if err := s.Fake.Send(ctx, chatID, text, kb); err != nil {
		return err
	}
	s.once.Do(s.cancel)
	return nil
}

// A failed long-poll is repeated after a pause that doubles up to a ceiling,
// and a context that ends in a pause ends Run.
func TestAFailedPollIsRepeatedAfterAGrowingPause(t *testing.T) {
	ob := newOutbox(nil)
	ob.block = true
	for range 7 {
		ob.pollErr = append(ob.pollErr, errors.New("gateway GET /v1/clients/telegram-bot/deliveries: connection refused"))
	}
	ob.pollErr = append(ob.pollErr, &client.APIError{Status: http.StatusConflict, Code: "poll_in_progress"})
	timers := &instantTimers{}
	log, buf := logBuffer()
	l := newLoop(t, ob, &sender.Fake{}, timers, log)

	// The ninth poll answers with a delivery, the tenth waits for the end.
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "A"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()
	waitUntil(t, func() bool { return len(ob.pollAfters()) >= 10 })
	cancel()
	<-done

	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}
	if got := timers.all(); !reflect.DeepEqual(got, want) {
		t.Errorf("pauses %v, want %v", got, want)
	}
	if n := strings.Count(buf.String(), "deliveries not polled"); n != 8 {
		t.Errorf("%d lines of a failed poll, want 8:\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), "deliveries polled again") {
		t.Errorf("no line of the recovery:\n%s", buf.String())
	}
}

// Acceptance of T-312, Mi-1 of review #1: a successful long-poll resets the
// pause — the next failure waits 1 s again, not the pause the series had
// grown to — and the recovery is logged once.
func TestASuccessfulPollResetsThePause(t *testing.T) {
	refused := errors.New("gateway GET /v1/clients/telegram-bot/deliveries: connection refused")
	ob := newOutbox(nil)
	ob.block = true
	// A nil entry is a long-poll that took its wait and answers with no
	// delivery.
	ob.pollErr = []error{refused, refused, nil, refused}
	timers := &instantTimers{}
	log, buf := logBuffer()
	l, err := deliver.New(deliver.Options{Gateway: ob, Sender: &sender.Fake{}, Timers: timers, Clock: ob.clock, Log: log})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()
	waitUntil(t, func() bool { return len(ob.pollAfters()) >= 5 })
	cancel()
	<-done

	if got, want := timers.all(), []time.Duration{time.Second, 2 * time.Second, time.Second}; !reflect.DeepEqual(got, want) {
		t.Errorf("pauses %v, want %v", got, want)
	}
	if n := strings.Count(buf.String(), "deliveries polled again"); n != 1 {
		t.Errorf("%d lines of the recovery, want 1:\n%s", n, buf.String())
	}
}

// The loop speaks C-08 through the real client: the long-poll of its own
// client id with the largest limit and wait, and one POST of the ids to ack.
func TestTheLoopSpeaksC08ThroughTheClient(t *testing.T) {
	var (
		mu       sync.Mutex
		requests []string
		ackBody  api.AckRequest
		polled   int
	)
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.Method+" "+r.URL.RequestURI()+" "+r.Header.Get(api.HeaderClientID))
		w.Header().Set("Content-Type", api.ContentTypeJSON)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/clients/telegram-bot/deliveries":
			polled++
			res := api.DeliveriesResponse{Deliveries: []api.Delivery{}, Cursor: "7"}
			if polled == 1 {
				res.Deliveries = []api.Delivery{delivery("d-1", "player-A", chatA, render.KindMechanics, "A"), delivery("d-2", "player-B", chatB, render.KindMechanics, "B")}
			}
			_ = json.NewEncoder(w).Encode(res)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/clients/telegram-bot/deliveries/ack":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &ackBody)
			_ = json.NewEncoder(w).Encode(api.AckResponse{Acked: len(ackBody.IDs), Unknown: []string{}})
		default:
			http.Error(w, "unexpected", http.StatusTeapot)
		}
	}))
	defer gw.Close()

	s := &sender.Fake{}
	l := newLoop(t, client.New(gw.URL, "telegram-bot"), s, &instantTimers{}, nil)
	mustOnce(t, l)
	mustOnce(t, l)

	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"GET /v1/clients/telegram-bot/deliveries?limit=100&wait_ms=25000 telegram-bot",
		"POST /v1/clients/telegram-bot/deliveries/ack telegram-bot",
		"GET /v1/clients/telegram-bot/deliveries?after=7&limit=100&wait_ms=25000 telegram-bot",
	}
	if !reflect.DeepEqual(requests, want) {
		t.Errorf("requests:\n%s\nwant:\n%s", strings.Join(requests, "\n"), strings.Join(want, "\n"))
	}
	if !slices.Equal(ackBody.IDs, []string{"d-1", "d-2"}) {
		t.Errorf("ack body %+v", ackBody)
	}
	if len(s.Sent()) != 2 {
		t.Errorf("sent %+v", s.Sent())
	}
}

func TestNewRequiresAGatewayAndASender(t *testing.T) {
	if _, err := deliver.New(deliver.Options{Sender: &sender.Fake{}}); err == nil {
		t.Error("New without a gateway succeeded")
	}
	if _, err := deliver.New(deliver.Options{Gateway: newOutbox(nil)}); err == nil {
		t.Error("New without a sender succeeded")
	}
}

// scriptedGateway answers each long-poll with the next result a test hands
// it, and waits for one meanwhile. nil is an empty answer at the end of the
// wait — the clock moves by the wait — and errEarlyEmpty an empty answer at
// once, the answer of a stopping gateway; any other error fails the long-poll.
type scriptedGateway struct {
	results chan error
	polls   atomic.Int32
	clock   *clock.Manual
}

// errEarlyEmpty tells scriptedGateway to answer an empty list at once.
var errEarlyEmpty = errors.New("empty at once")

func newScriptedGateway() *scriptedGateway {
	return &scriptedGateway{results: make(chan error), clock: clock.NewManual(epoch)}
}

func (g *scriptedGateway) Deliveries(ctx context.Context, _ string, _ int, wait time.Duration) (api.DeliveriesResponse, error) {
	select {
	case err := <-g.results:
		g.polls.Add(1)
		switch {
		case err == nil:
			g.clock.Advance(wait)
		case errors.Is(err, errEarlyEmpty):
			err = nil
		}
		return api.DeliveriesResponse{Deliveries: []api.Delivery{}}, err
	case <-ctx.Done():
		return api.DeliveriesResponse{}, ctx.Err()
	}
}

// runScripted runs a loop over a scriptedGateway on its clock until the test
// ends.
func runScripted(t *testing.T, gw *scriptedGateway) *deliver.Loop {
	t.Helper()
	l, err := deliver.New(deliver.Options{Gateway: gw, Sender: &sender.Fake{}, Timers: &instantTimers{}, Clock: gw.clock})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return l
}

func (g *scriptedGateway) Ack(context.Context, []string) (api.AckResponse, error) {
	return api.AckResponse{}, nil
}

// Acceptance of T-312: the loop is degraded after DegradedAfterFailedPolls
// failed long-polls in a row, not one before, and a successful long-poll makes
// it healthy again. The pauses run on clock.Timers.
func TestTheLoopIsDegradedAfterASeriesOfFailedPolls(t *testing.T) {
	gw := newScriptedGateway()
	l := runScripted(t, gw)
	if h := l.Health(); h.Degraded() {
		t.Fatalf("a new loop is degraded: %+v", h)
	}
	refused := errors.New("gateway GET /v1/clients/telegram-bot/deliveries: connection refused")
	for n := 1; n < deliver.DegradedAfterFailedPolls; n++ {
		gw.results <- refused
		waitUntil(t, func() bool { return l.Health().FailedPolls == n })
		if h := l.Health(); h.Degraded() {
			t.Fatalf("degraded after %d failed polls, want only after %d", n, deliver.DegradedAfterFailedPolls)
		}
	}
	gw.results <- refused
	waitUntil(t, func() bool { return l.Health().Degraded() })
	gw.results <- nil
	waitUntil(t, func() bool { return l.Health().FailedPolls == 0 })
	if h := l.Health(); h.Degraded() {
		t.Errorf("a successful poll left the loop degraded: %+v", h)
	}
}

// Ma-1 of review #1 of T-315: an empty answer sooner than EarlyEmpty is the
// answer of a stopping gateway, and the bot delivers nothing meanwhile. A
// series of them makes the loop degraded at the same threshold as failed
// long-polls, and an empty answer at the end of the wait makes it healthy.
func TestASeriesOfEarlyEmptyAnswersDegradesTheLoop(t *testing.T) {
	gw := newScriptedGateway()
	l := runScripted(t, gw)
	for n := 1; n < deliver.DegradedAfterFailedPolls; n++ {
		gw.results <- errEarlyEmpty
		waitUntil(t, func() bool { return l.Health().FailedPolls == n })
		if h := l.Health(); h.Degraded() {
			t.Fatalf("degraded after %d early empty answers, want only after %d", n, deliver.DegradedAfterFailedPolls)
		}
	}
	gw.results <- errEarlyEmpty
	waitUntil(t, func() bool { return l.Health().Degraded() })
	gw.results <- nil
	waitUntil(t, func() bool { return l.Health().FailedPolls == 0 })
	if h := l.Health(); h.Degraded() {
		t.Errorf("an empty answer at the end of the wait left the loop degraded: %+v", h)
	}
}

// An ordinary empty answer — one that took the wait — ends a series of failed
// long-polls and of early empty answers alike: the count starts from zero.
func TestAnEmptyAnswerAtTheEndOfTheWaitResetsTheFailedPolls(t *testing.T) {
	gw := newScriptedGateway()
	l := runScripted(t, gw)
	refused := errors.New("gateway GET /v1/clients/telegram-bot/deliveries: connection refused")
	for _, series := range [][]error{{refused, refused, errEarlyEmpty}, {errEarlyEmpty, errEarlyEmpty, refused}} {
		for n, result := range series {
			gw.results <- result
			waitUntil(t, func() bool { return l.Health().FailedPolls == n+1 })
		}
		gw.results <- nil
		waitUntil(t, func() bool { return l.Health().FailedPolls == 0 })
		// One more failure counts from one again, not from the series before:
		// the next long-poll is taken only once the count of this one is
		// stored, so the count cannot pass one on its way.
		gw.results <- errEarlyEmpty
		waitUntil(t, func() bool { return l.Health().FailedPolls == 1 })
		gw.results <- nil
		waitUntil(t, func() bool { return l.Health().FailedPolls == 0 })
	}
}

// Acceptance of T-312: a 401 on a send makes the loop degraded, whatever the
// long-poll does; a message sent later makes it healthy again.
func TestARefusedTokenOnASendDegradesTheLoop(t *testing.T) {
	ob := newOutbox(nil)
	ob.enqueue(delivery("a1", "player-A", chatA, render.KindMechanics, "A"))
	s := &sender.Fake{}
	s.FailNext(sender.ErrUnauthorized)
	l := newLoop(t, ob, s, &instantTimers{}, nil)
	mustOnce(t, l)
	if h := l.Health(); !h.Unauthorized || !h.Degraded() {
		t.Fatalf("after a 401 on a send: %+v, want degraded", h)
	}
	mustOnce(t, l)
	if h := l.Health(); !h.Unauthorized {
		t.Errorf("a successful poll without a send cleared the 401: %+v", h)
	}
	ob.clock.Advance(lease + time.Millisecond)
	mustOnce(t, l)
	if h := l.Health(); h.Unauthorized || h.Degraded() {
		t.Errorf("after a message sent: %+v, want ok", h)
	}
}
