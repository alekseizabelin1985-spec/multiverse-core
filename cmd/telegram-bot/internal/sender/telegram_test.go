package sender_test

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
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/shared/clock"
)

const (
	testToken = "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"
	chatID    = int64(987654321)
)

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

func (it *instantTimers) Every(time.Duration) clock.Timer { panic("sender uses no periodic timer") }

func (it *instantTimers) Pauses() []time.Duration {
	it.mu.Lock()
	defer it.mu.Unlock()
	return slices.Clone(it.pauses)
}

// silentTimers never fire; armed tells the test a pause started.
type silentTimers struct{ armed chan struct{} }

func (s silentTimers) After(time.Duration) clock.Timer {
	s.armed <- struct{}{}
	return chanTimer{make(chan time.Time)}
}

func (silentTimers) Every(time.Duration) clock.Timer { panic("sender uses no periodic timer") }

type chanTimer struct{ ch chan time.Time }

func (c chanTimer) C() <-chan time.Time { return c.ch }
func (c chanTimer) Stop() bool          { return false }

// sentForm is one sendMessage as the fake Bot API received it.
type sentForm map[string]string

type answer struct {
	status int
	body   string
}

var okAnswer = answer{http.StatusOK, `{"ok":true,"result":{"message_id":1,"date":1,"chat":{"id":987654321,"type":"private"}}}`}

// fakeBotAPI answers sendMessage with its script; the last answer repeats.
type fakeBotAPI struct {
	t       *testing.T
	mu      sync.Mutex
	answers []answer
	forms   []sentForm
}

func (f *fakeBotAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bot"+testToken+"/sendMessage" {
		f.t.Errorf("unexpected request %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		f.t.Errorf("parse form: %v", err)
	}
	form := sentForm{}
	for k, v := range r.MultipartForm.Value {
		form[k] = v[0]
	}
	f.mu.Lock()
	f.forms = append(f.forms, form)
	a := f.answers[0]
	if len(f.answers) > 1 {
		f.answers = f.answers[1:]
	}
	f.mu.Unlock()
	if a.status == 0 {
		// Drop the connection without an answer: a network error.
		hj, ok := w.(http.Hijacker)
		if !ok {
			f.t.Error("cannot hijack the connection")
			return
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(a.status)
	_, _ = w.Write([]byte(a.body))
}

func (f *fakeBotAPI) Forms() []sentForm {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.forms)
}

func newSender(t *testing.T, timers clock.Timers, answers ...answer) (*sender.Telegram, *fakeBotAPI) {
	t.Helper()
	api := &fakeBotAPI{t: t, answers: answers}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	s, err := sender.NewTelegram(sender.TelegramOptions{
		Token:     testToken,
		Redactor:  privacy.NewRedactor(testToken),
		Timers:    timers,
		ServerURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("NewTelegram: %v", err)
	}
	return s, api
}

func TestSendDeliversMarkupLiterally(t *testing.T) {
	texts := []string{
		"[x](http://evil.example/phish)",
		`<a href="http://evil.example/phish">нажми</a>`,
		"*текст* _курсив_ `код`",
		"Механика: волк атакует — 3 урона. <b>не жирный</b>",
	}
	for _, text := range texts {
		t.Run(text, func(t *testing.T) {
			s, api := newSender(t, &instantTimers{}, okAnswer)
			if err := s.Send(context.Background(), chatID, text, nil); err != nil {
				t.Fatalf("Send: %v", err)
			}
			forms := api.Forms()
			if len(forms) != 1 {
				t.Fatalf("%d requests, want 1", len(forms))
			}
			form := forms[0]
			if form["text"] != text {
				t.Fatalf("text sent as %q, want it byte for byte: %q", form["text"], text)
			}
			if _, set := form["parse_mode"]; set {
				t.Fatalf("parse_mode sent (%q): messages are plain text (SEC-10)", form["parse_mode"])
			}
			if _, set := form["entities"]; set {
				t.Fatal("entities sent: messages are plain text (SEC-10)")
			}
			if form["chat_id"] != strconv.FormatInt(chatID, 10) {
				t.Fatalf("chat_id = %q", form["chat_id"])
			}
		})
	}
}

// TestSendHasNoMarkupParameter pins the signature: the only way to ask for a
// parse mode would be a new parameter or a field of Keyboard, and both change
// what this test sees.
func TestSendHasNoMarkupParameter(t *testing.T) {
	send, ok := reflect.TypeOf((*sender.Sender)(nil)).Elem().MethodByName("Send")
	if !ok {
		t.Fatal("Sender has no Send")
	}
	want := []reflect.Type{
		reflect.TypeOf((*context.Context)(nil)).Elem(),
		reflect.TypeOf(int64(0)),
		reflect.TypeOf(""),
		reflect.TypeOf(&sender.Keyboard{}),
	}
	if send.Type.NumIn() != len(want) {
		t.Fatalf("Send takes %d parameters, want %d (ctx, chatID, text, keyboard)", send.Type.NumIn(), len(want))
	}
	for i, w := range want {
		if send.Type.In(i) != w {
			t.Fatalf("parameter %d of Send is %s, want %s", i, send.Type.In(i), w)
		}
	}
	kb := reflect.TypeOf(sender.Keyboard{})
	for i := 0; i < kb.NumField(); i++ {
		if name := strings.ToLower(kb.Field(i).Name); strings.Contains(name, "parse") || strings.Contains(name, "mode") || strings.Contains(name, "entit") {
			t.Fatalf("Keyboard has field %s: markup has no place in a keyboard", kb.Field(i).Name)
		}
	}
}

func TestSendMapsTheKeyboard(t *testing.T) {
	s, api := newSender(t, &instantTimers{}, okAnswer)
	kb := &sender.Keyboard{Rows: [][]string{{"Подтверждаю: мне 18+ и я согласен"}, {"Отказаться", "[x](http://a)"}}}
	if err := s.Send(context.Background(), chatID, "уведомление", kb); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := s.Send(context.Background(), chatID, "без клавиатуры", &sender.Keyboard{Remove: true}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	forms := api.Forms()
	var markup struct {
		Keyboard [][]struct {
			Text string `json:"text"`
		} `json:"keyboard"`
		Resize bool `json:"resize_keyboard"`
	}
	if err := json.Unmarshal([]byte(forms[0]["reply_markup"]), &markup); err != nil {
		t.Fatalf("reply_markup %q: %v", forms[0]["reply_markup"], err)
	}
	if len(markup.Keyboard) != 2 || markup.Keyboard[0][0].Text != "Подтверждаю: мне 18+ и я согласен" ||
		markup.Keyboard[1][1].Text != "[x](http://a)" || !markup.Resize {
		t.Fatalf("keyboard sent as %s", forms[0]["reply_markup"])
	}
	if forms[1]["reply_markup"] != `{"remove_keyboard":true}` {
		t.Fatalf("remove keyboard sent as %q", forms[1]["reply_markup"])
	}
}

func TestSendWaitsOutTooManyRequests(t *testing.T) {
	timers := &instantTimers{}
	s, api := newSender(t, timers,
		answer{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 7","parameters":{"retry_after":7}}`},
		answer{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":0}}`},
		okAnswer)
	if err := s.Send(context.Background(), chatID, "hello", nil); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := timers.Pauses(); !slices.Equal(got, []time.Duration{7 * time.Second, time.Second}) {
		t.Fatalf("pauses %v, want [7s 1s]: retry_after, and one second when Telegram gives none", got)
	}
	if n := len(api.Forms()); n != 3 {
		t.Fatalf("%d requests, want 3", n)
	}
}

func TestSendGivesUpOnAFloodBanOrTooMany429(t *testing.T) {
	ban := answer{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":3600}}`}
	s, api := newSender(t, &instantTimers{}, ban)
	if err := s.Send(context.Background(), chatID, "hello", nil); !errors.Is(err, sender.ErrUnavailable) {
		t.Fatalf("Send under a flood ban = %v, want ErrUnavailable", err)
	}
	if n := len(api.Forms()); n != 1 {
		t.Fatalf("%d requests: a retry_after beyond the maximum is not waited", n)
	}

	busy := answer{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`}
	timers := &instantTimers{}
	s, api = newSender(t, timers, busy)
	if err := s.Send(context.Background(), chatID, "hello", nil); !errors.Is(err, sender.ErrUnavailable) {
		t.Fatalf("Send under endless 429 = %v, want ErrUnavailable", err)
	}
	if n, p := len(api.Forms()), len(timers.Pauses()); n != sender.DefaultPolicy.RateLimitWaits+1 || p != sender.DefaultPolicy.RateLimitWaits {
		t.Fatalf("%d requests and %d pauses, want %d and %d", n, p, sender.DefaultPolicy.RateLimitWaits+1, sender.DefaultPolicy.RateLimitWaits)
	}
}

func TestSendRepeatsServerErrorsThreeTimes(t *testing.T) {
	serverError := answer{http.StatusBadGateway, `<html>502 Bad Gateway</html>`}
	internal := answer{http.StatusInternalServerError, `{"ok":false,"error_code":500,"description":"Internal Server Error"}`}

	timers := &instantTimers{}
	s, api := newSender(t, timers, serverError, internal, okAnswer)
	if err := s.Send(context.Background(), chatID, "hello", nil); err != nil {
		t.Fatalf("Send after two 5xx = %v, want success on the third try", err)
	}
	if got := timers.Pauses(); !slices.Equal(got, []time.Duration{time.Second, time.Second}) || len(api.Forms()) != 3 {
		t.Fatalf("pauses %v over %d requests, want [1s 1s] over 3", got, len(api.Forms()))
	}

	timers = &instantTimers{}
	s, api = newSender(t, timers, internal)
	err := s.Send(context.Background(), chatID, "hello", nil)
	if !errors.Is(err, sender.ErrUnavailable) {
		t.Fatalf("Send after endless 5xx = %v, want ErrUnavailable", err)
	}
	if n := len(api.Forms()); n != 3 {
		t.Fatalf("%d requests, want exactly 3 tries", n)
	}
}

func TestSendRepeatsANetworkError(t *testing.T) {
	timers := &instantTimers{}
	s, api := newSender(t, timers, answer{}, okAnswer)
	if err := s.Send(context.Background(), chatID, "hello", nil); err != nil {
		t.Fatalf("Send after a dropped connection = %v, want success", err)
	}
	if len(api.Forms()) != 2 || len(timers.Pauses()) != 1 {
		t.Fatalf("%d requests and %d pauses, want 2 and 1", len(api.Forms()), len(timers.Pauses()))
	}
}

func TestSendClassifiesRefusalsWithoutIDsOrToken(t *testing.T) {
	cases := []struct {
		name string
		a    answer
		want error
	}{
		{"blocked", answer{http.StatusForbidden, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user 987654321"}`}, sender.ErrBlocked},
		{"chat not found", answer{http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`}, sender.ErrChatNotFound},
		{"other 400", answer{http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: message text is empty at /bot` + testToken + `/sendMessage"}`}, sender.ErrRejected},
		{"unauthorized", answer{http.StatusUnauthorized, `{"ok":false,"error_code":401,"description":"Unauthorized"}`}, sender.ErrUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			timers := &instantTimers{}
			s, api := newSender(t, timers, c.a)
			err := s.Send(context.Background(), chatID, "hello", nil)
			if !errors.Is(err, c.want) {
				t.Fatalf("Send = %v, want %v", err, c.want)
			}
			for _, other := range []error{sender.ErrBlocked, sender.ErrChatNotFound, sender.ErrRejected, sender.ErrUnauthorized, sender.ErrUnavailable} {
				if other != c.want && errors.Is(err, other) {
					t.Fatalf("Send = %v is also %v: a delivery loop must tell them apart", err, other)
				}
			}
			if len(api.Forms()) != 1 || len(timers.Pauses()) != 0 {
				t.Fatal("a refusal is not repeated")
			}
			for _, leaked := range []string{"987654321", "ABCdefGHI"} {
				if strings.Contains(err.Error(), leaked) {
					t.Fatalf("error text carries %q: %v", leaked, err)
				}
			}
		})
	}
}

func TestSendStopsWithTheContext(t *testing.T) {
	timers := silentTimers{armed: make(chan struct{}, 1)}
	s, _ := newSender(t, timers, answer{http.StatusInternalServerError, `{"ok":false,"error_code":500,"description":"x"}`})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Send(ctx, chatID, "hello", nil) }()
	<-timers.armed
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Send cancelled during a pause = %v, want context.Canceled", err)
	}

	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if err := s.Send(cancelled, chatID, "hello", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Send with a cancelled context = %v", err)
	}
}

func TestFakeRecordsAndFails(t *testing.T) {
	var f sender.Fake
	boom := errors.New("boom")
	f.FailNext(boom)
	kb := &sender.Keyboard{Rows: [][]string{{"a"}}}
	if err := f.Send(context.Background(), 1, "first", kb); !errors.Is(err, boom) {
		t.Fatalf("first Send = %v, want the queued error", err)
	}
	if err := f.Send(context.Background(), 1, "second", kb); err != nil {
		t.Fatal(err)
	}
	kb.Rows[0][0] = "changed"
	sent := f.Sent()
	if len(sent) != 1 || sent[0].Text != "second" || sent[0].Keyboard.Rows[0][0] != "a" {
		t.Fatalf("sent %#v", sent)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.Send(ctx, 1, "x", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Send with a cancelled context = %v", err)
	}
	if s := sent[0].String(); strings.Contains(s, "second") {
		t.Fatalf("a message prints its text: %s", s)
	}
}

// TestSendWaitsForRetryAfterUpToTheMaximumInclusive pins the edge of the flood
// ban: retry_after equal to MaxRetryAfter is waited, one second more is not.
func TestSendWaitsForRetryAfterUpToTheMaximumInclusive(t *testing.T) {
	atMax := answer{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":60}}`}
	timers := &instantTimers{}
	s, api := newSender(t, timers, atMax, okAnswer)
	if err := s.Send(context.Background(), chatID, "hello", nil); err != nil {
		t.Fatalf("Send with retry_after = 60 s = %v, want it waited out", err)
	}
	if got := timers.Pauses(); !slices.Equal(got, []time.Duration{time.Minute}) || len(api.Forms()) != 2 {
		t.Fatalf("pauses %v over %d requests, want [1m0s] over 2", got, len(api.Forms()))
	}

	overMax := answer{http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":61}}`}
	timers = &instantTimers{}
	s, api = newSender(t, timers, overMax, okAnswer)
	if err := s.Send(context.Background(), chatID, "hello", nil); !errors.Is(err, sender.ErrUnavailable) {
		t.Fatalf("Send with retry_after = 61 s = %v, want ErrUnavailable", err)
	}
	if len(timers.Pauses()) != 0 || len(api.Forms()) != 1 {
		t.Fatalf("%d pauses over %d requests, want none over 1", len(timers.Pauses()), len(api.Forms()))
	}
}

// TestBestEffortWaitsForNothing: the policy of the gate replies gives up at
// once on a 429, a 5xx and a network error, and arms no timer.
func TestBestEffortWaitsForNothing(t *testing.T) {
	cases := map[string]answer{
		"429":     {http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":30}}`},
		"5xx":     {http.StatusInternalServerError, `{"ok":false,"error_code":500,"description":"Internal Server Error"}`},
		"network": {},
	}
	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			timers := &instantTimers{}
			s, api := newSender(t, timers, a, okAnswer)
			err := s.WithPolicy(sender.BestEffortPolicy).Send(context.Background(), chatID, "hello", nil)
			if !errors.Is(err, sender.ErrUnavailable) {
				t.Fatalf("Send = %v, want ErrUnavailable without a repeat", err)
			}
			if n := len(timers.Pauses()); n != 0 {
				t.Fatalf("%d pauses armed: a best-effort message waits for nothing", n)
			}
			if n := len(api.Forms()); n != 1 {
				t.Fatalf("%d requests, want 1", n)
			}
		})
	}
}

func TestWithPolicyKeepsTheClientAndDefaultsTheZeroPolicy(t *testing.T) {
	timers := &instantTimers{}
	s, api := newSender(t, timers, answer{http.StatusInternalServerError, `{"ok":false,"error_code":500,"description":"x"}`})
	if err := s.WithPolicy(sender.Policy{}).Send(context.Background(), chatID, "hello", nil); !errors.Is(err, sender.ErrUnavailable) {
		t.Fatalf("Send = %v", err)
	}
	if n := len(api.Forms()); n != sender.DefaultPolicy.Attempts {
		t.Fatalf("%d requests: the zero policy means DefaultPolicy (%d tries)", n, sender.DefaultPolicy.Attempts)
	}

	// WithPolicy gives a copy: the sender it came from keeps its own policy
	// (review #2 of T-310, N-8, X12). Otherwise building the sender of the gate
	// from the sender of the deliveries would make every delivery best effort.
	timers = &instantTimers{}
	s, api = newSender(t, timers, answer{http.StatusInternalServerError, `{"ok":false,"error_code":500,"description":"x"}`})
	_ = s.WithPolicy(sender.BestEffortPolicy)
	if err := s.Send(context.Background(), chatID, "hello", nil); !errors.Is(err, sender.ErrUnavailable) {
		t.Fatalf("Send = %v", err)
	}
	if n := len(api.Forms()); n != sender.DefaultPolicy.Attempts {
		t.Fatalf("%d requests by the original sender after WithPolicy(BestEffortPolicy), want %d", n, sender.DefaultPolicy.Attempts)
	}
}

// deadlineTransport answers every request with okAnswer and records the
// deadline the HTTP client put on it: http.Client.Timeout becomes a deadline
// of the request context.
type deadlineTransport struct {
	mu        sync.Mutex
	deadlines []time.Time
}

func (d *deadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}
	deadline, _ := req.Context().Deadline()
	d.mu.Lock()
	d.deadlines = append(d.deadlines, deadline)
	d.mu.Unlock()
	return &http.Response{
		StatusCode: okAnswer.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(okAnswer.body)),
		Request:    req,
	}, nil
}

func (d *deadlineTransport) last() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.deadlines[len(d.deadlines)-1]
}

// TestNewBestEffortTelegramCutsALongClientTimeout (review #2 of T-310, Mi-4):
// given the client of the deliveries, with an hour-long Timeout, the
// best-effort sender still gives a sendMessage at most BestEffortHTTPTimeout,
// while the ordinary sender over the same client keeps the hour.
func TestNewBestEffortTelegramCutsALongClientTimeout(t *testing.T) {
	transport := &deadlineTransport{}
	shared := &http.Client{Timeout: time.Hour, Transport: transport}
	opts := sender.TelegramOptions{Token: testToken, Timers: &instantTimers{}, ServerURL: "http://bot-api.invalid", HTTPClient: shared}

	quick, err := sender.NewBestEffortTelegram(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := quick.Send(context.Background(), chatID, "hello", nil); err != nil {
		t.Fatalf("Send = %v", err)
	}
	// The deadline was set before Send returned, so measured from a moment
	// after it the time left can only be shorter than the Timeout.
	after := clock.Real{}.Now()
	if left := transport.last().Sub(after); left <= 0 || left > sender.BestEffortHTTPTimeout {
		t.Fatalf("a best-effort sendMessage may last %s, want at most %s", left, sender.BestEffortHTTPTimeout)
	}

	ordinary, err := sender.NewTelegram(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := ordinary.Send(context.Background(), chatID, "hello", nil); err != nil {
		t.Fatalf("Send = %v", err)
	}
	if left := transport.last().Sub(clock.Real{}.Now()); left < 30*time.Minute {
		t.Fatalf("the ordinary sender over the same client may last %s: the best-effort one changed the shared client", left)
	}
}

// TestNewBestEffortTelegramGivesUpOnASlowAnswer (review #2 of T-310, Mi-4):
// Telegram takes the message and does not answer. The best-effort sender makes
// one request, arms no pause and returns by the Timeout of its client, whatever
// Policy the options carried.
func TestNewBestEffortTelegramGivesUpOnASlowAnswer(t *testing.T) {
	release := make(chan struct{})
	requests := make(chan struct{}, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	timers := &instantTimers{}
	s, err := sender.NewBestEffortTelegram(sender.TelegramOptions{
		Token:      testToken,
		Redactor:   privacy.NewRedactor(testToken),
		Timers:     timers,
		Policy:     sender.DefaultPolicy,
		ServerURL:  srv.URL,
		HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Send(context.Background(), chatID, "hello", nil) }()
	deadline, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case err := <-done:
		if !errors.Is(err, sender.ErrUnavailable) {
			t.Fatalf("Send = %v, want ErrUnavailable", err)
		}
		for _, leaked := range []string{"987654321", "ABCdefGHI"} {
			if strings.Contains(err.Error(), leaked) {
				t.Fatalf("error text carries %q: %v", leaked, err)
			}
		}
	case <-deadline.Done():
		t.Fatal("a best-effort Send outlived the Timeout of its client")
	}
	if n := len(requests); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}
	if n := len(timers.Pauses()); n != 0 {
		t.Fatalf("%d pauses armed, want none", n)
	}
}

// TestSendKeepsAnUnreadableAnswerOutOfTheError: on a body it cannot decode the
// library quotes the whole body in its error, and a sendMessage answer carries
// the chat and the text of the message.
func TestSendKeepsAnUnreadableAnswerOutOfTheError(t *testing.T) {
	broken := answer{http.StatusOK, `{"ok":true,"result":{"chat":{"id":555000111,"username":"vasya_secret_nick"},"text":"my private words"}`}
	s, _ := newSender(t, &instantTimers{}, broken)
	err := s.Send(context.Background(), chatID, "my private words", nil)
	if err == nil {
		t.Fatal("Send accepted an answer it could not read")
	}
	for _, leaked := range []string{"555000111", "vasya_secret_nick", "my private words", "ABCdefGHI"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("error text carries %q: %v", leaked, err)
		}
	}
	if !strings.Contains(err.Error(), "sendMessage") {
		t.Fatalf("the error must still say which call failed: %v", err)
	}
}
