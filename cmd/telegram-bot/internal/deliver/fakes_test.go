package deliver_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/deliver"
	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/logging"
)

const testToken = "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"

// Chats of three players; each id is long enough that a leak into a log line
// cannot be a coincidence.
const (
	chatA = int64(700000000101)
	chatB = int64(700000000202)
	chatC = int64(700000000303)
)

var epoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// lease is MV_GATEWAY_DELIVERY_LEASE of the gateway.
const lease = 30 * time.Second

// journal is the one ordered record of what the loop did: the messages it
// sent and the acks it made, so that a test sees their order.
type journal struct {
	mu      sync.Mutex
	entries []string
}

func (j *journal) add(entry string) {
	j.mu.Lock()
	j.entries = append(j.entries, entry)
	j.mu.Unlock()
}

func (j *journal) all() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.entries)
}

// outbox is the gateway of C-08 as the bot sees it, over a manual clock: a
// long-poll leases the head delivery of every player that has none leased
// (component §8.2), ack confirms by id, and a delivery not acknowledged within
// the lease is handed out again. It keeps nothing about the client between
// calls but the leases, as the gateway does.
type outbox struct {
	mu      sync.Mutex
	clock   *clock.Manual
	items   []*item
	j       *journal
	afters  []string
	acks    [][]string
	pollErr []error
	ackErr  []error
	// block makes an empty long-poll wait for ctx instead of answering at once.
	block bool
	// stopping makes every long-poll answer an empty list at once, as the
	// gateway does while its process stops.
	stopping bool
	// emptyTook is how long a nil entry of pollErr takes; zero is the wait.
	emptyTook time.Duration
	seq       int
}

type item struct {
	d           api.Delivery
	leasedUntil time.Time
	acked       bool
}

func newOutbox(j *journal) *outbox {
	return &outbox{clock: clock.NewManual(epoch), j: j}
}

func (o *outbox) enqueue(ds ...api.Delivery) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, d := range ds {
		o.items = append(o.items, &item{d: d})
	}
}

func (o *outbox) Deliveries(ctx context.Context, after string, limit int, wait time.Duration) (api.DeliveriesResponse, error) {
	o.mu.Lock()
	o.afters = append(o.afters, after)
	if len(o.pollErr) > 0 {
		err := o.pollErr[0]
		o.pollErr = o.pollErr[1:]
		o.mu.Unlock()
		if err == nil {
			// A nil entry is a long-poll that took its whole wait, or
			// emptyTook, and found nothing: the clock moves by that much.
			took := wait
			if o.emptyTook > 0 {
				took = o.emptyTook
			}
			o.clock.Advance(took)
		}
		return api.DeliveriesResponse{}, err
	}
	if o.stopping {
		o.mu.Unlock()
		return api.DeliveriesResponse{Deliveries: []api.Delivery{}}, nil
	}
	if limit != deliver.Limit || wait != deliver.Wait {
		o.mu.Unlock()
		return api.DeliveriesResponse{}, errors.New("unexpected limit or wait")
	}
	now := o.clock.Now()
	busy := map[string]bool{}
	out := api.DeliveriesResponse{Deliveries: []api.Delivery{}}
	for _, it := range o.items {
		if it.acked || busy[it.d.PlayerID] {
			continue
		}
		busy[it.d.PlayerID] = true
		if now.Before(it.leasedUntil) {
			continue
		}
		it.leasedUntil = now.Add(lease)
		out.Deliveries = append(out.Deliveries, it.d)
		o.seq++
		out.Cursor = strconv.Itoa(o.seq)
	}
	block := o.block && len(out.Deliveries) == 0
	o.mu.Unlock()
	if block {
		<-ctx.Done()
		return api.DeliveriesResponse{}, ctx.Err()
	}
	return out, nil
}

func (o *outbox) Ack(ctx context.Context, ids []string) (api.AckResponse, error) {
	if err := ctx.Err(); err != nil {
		return api.AckResponse{}, err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.acks = append(o.acks, slices.Clone(ids))
	if len(o.ackErr) > 0 {
		err := o.ackErr[0]
		o.ackErr = o.ackErr[1:]
		return api.AckResponse{}, err
	}
	if o.j != nil {
		o.j.add("ack " + strings.Join(ids, ","))
	}
	res := api.AckResponse{Unknown: []string{}}
	for _, id := range ids {
		found := false
		for _, it := range o.items {
			if it.d.ID == id && !it.acked {
				it.acked, found = true, true
				res.Acked++
			}
		}
		if !found {
			res.Unknown = append(res.Unknown, id)
		}
	}
	return res, nil
}

func (o *outbox) ackCalls() [][]string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.acks)
}

func (o *outbox) pollAfters() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.afters)
}

func (o *outbox) unacked() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []string
	for _, it := range o.items {
		if !it.acked {
			out = append(out, it.d.ID)
		}
	}
	return out
}

// recordingSender is sender.Fake that also writes every message it sent into
// the journal.
type recordingSender struct {
	sender.Fake
	j *journal
}

func (s *recordingSender) Send(ctx context.Context, chatID int64, text string, kb *sender.Keyboard) error {
	err := s.Fake.Send(ctx, chatID, text, kb)
	if err == nil {
		s.j.add("send " + strconv.FormatInt(chatID, 10) + " " + text)
	}
	return err
}

func route(chat int64) *api.DeliveryRoute {
	return &api.DeliveryRoute{ExternalPlatform: "telegram", ExternalID: strconv.FormatInt(chat, 10)}
}

func delivery(id, player string, chat int64, kind, text string) api.Delivery {
	return api.Delivery{ID: id, PlayerID: player, Route: route(chat), Kind: kind, GeneratedBy: render.GeneratedByRules, Text: text}
}

// logBuffer is the logger of the bot over a buffer: JSON of shared/logging
// under privacy.Handler.
func logBuffer() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	inner := logging.New(logging.Options{Service: "telegram-bot", Level: slog.LevelDebug, Output: buf}).Handler()
	return slog.New(privacy.NewHandler(inner, privacy.NewRedactor(testToken))), buf
}

// instantTimers fires every pause at once and remembers its length.
type instantTimers struct {
	mu     sync.Mutex
	pauses []time.Duration
}

func (it *instantTimers) After(d time.Duration) clock.Timer {
	it.mu.Lock()
	it.pauses = append(it.pauses, d)
	it.mu.Unlock()
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return chanTimer{ch}
}

func (it *instantTimers) Every(time.Duration) clock.Timer { panic("no periodic timer expected") }

func (it *instantTimers) all() []time.Duration {
	it.mu.Lock()
	defer it.mu.Unlock()
	return slices.Clone(it.pauses)
}

type chanTimer struct{ ch chan time.Time }

func (c chanTimer) C() <-chan time.Time { return c.ch }
func (c chanTimer) Stop() bool          { return false }

// botAnswer is one answer of the fake Bot API.
type botAnswer struct {
	status int
	body   string
}

var botOK = botAnswer{http.StatusOK, `{"ok":true,"result":{"message_id":1,"date":1,"chat":{"id":1,"type":"private"}}}`}

// botAPI is sendMessage of the Bot API on httptest: every chat answers by its
// own script, whose last answer repeats.
type botAPI struct {
	t       *testing.T
	srv     *httptest.Server
	mu      sync.Mutex
	scripts map[string][]botAnswer
	tries   map[string]int
}

func newBotAPI(t *testing.T) *botAPI {
	b := &botAPI{t: t, scripts: map[string][]botAnswer{}, tries: map[string]int{}}
	b.srv = httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *botAPI) on(chat int64, answers ...botAnswer) {
	b.mu.Lock()
	b.scripts[strconv.FormatInt(chat, 10)] = answers
	b.mu.Unlock()
}

func (b *botAPI) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bot"+testToken+"/sendMessage" {
		b.t.Errorf("unexpected request %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		b.t.Errorf("parse form: %v", err)
	}
	chat := r.FormValue("chat_id")
	b.mu.Lock()
	b.tries[chat]++
	script := b.scripts[chat]
	a := botOK
	switch {
	case len(script) == 1:
		a = script[0]
	case len(script) > 1:
		a, b.scripts[chat] = script[0], script[1:]
	}
	b.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(a.status)
	_, _ = w.Write([]byte(a.body))
}

func (b *botAPI) triesOf(chat int64) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tries[strconv.FormatInt(chat, 10)]
}

// telegramSender is the sender of the delivery loop in production: the real
// sender.Telegram with DefaultPolicy, against the fake Bot API.
func telegramSender(t *testing.T, b *botAPI, timers clock.Timers) *sender.Telegram {
	t.Helper()
	s, err := sender.NewTelegram(sender.TelegramOptions{Token: testToken, ServerURL: b.srv.URL, Timers: timers, Redactor: privacy.NewRedactor(testToken)})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// waitUntil polls cond for up to 5 s of the wall clock.
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for !cond() {
		if deadline.Err() != nil {
			t.Fatal("condition not met within 5 s")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func newLoop(t *testing.T, gw deliver.Gateway, s sender.Sender, timers clock.Timers, log *slog.Logger) *deliver.Loop {
	t.Helper()
	l, err := deliver.New(deliver.Options{Gateway: gw, Sender: s, Timers: timers, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	return l
}
