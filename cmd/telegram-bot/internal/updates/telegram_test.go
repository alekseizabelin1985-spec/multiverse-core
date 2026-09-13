package updates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/shared/logging"
)

const testToken = "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"

// fakeTelegram is the Bot API on httptest: getMe answers with a bot, every
// getUpdates takes the next reply of its script, and a script that ran out
// holds the request until the client gives up or the test releases it — a
// long poll with nothing to say.
type fakeTelegram struct {
	t       *testing.T
	getMe   reply
	mu      sync.Mutex
	polls   []reply
	forms   []map[string]string
	polled  atomic.Int32
	methods []string
	// onPoll, when set, sees the form of every getUpdates as it arrives.
	onPoll func(form map[string]string)
	// held is closed by releaseHeld: a test that may leave a poll held ends it
	// before closing the server, which waits for every request.
	held        chan struct{}
	releaseOnce sync.Once
}

func (f *fakeTelegram) releaseHeld() { f.releaseOnce.Do(func() { close(f.held) }) }

type reply struct {
	status int
	body   string
	// after holds the answer until the channel is closed.
	after <-chan struct{}
}

func okResult(result string) reply {
	return reply{status: http.StatusOK, body: `{"ok":true,"result":` + result + `}`}
}

func newFakeTelegram(t *testing.T, polls ...reply) *fakeTelegram {
	return &fakeTelegram{t: t, getMe: okResult(`{"id":123456,"is_bot":true,"first_name":"bot"}`), polls: polls, held: make(chan struct{})}
}

func (f *fakeTelegram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prefix := "/bot" + testToken + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	method := strings.TrimPrefix(r.URL.Path, prefix)
	form := map[string]string{}
	if err := r.ParseMultipartForm(1 << 20); err == nil {
		for k, v := range r.MultipartForm.Value {
			form[k] = v[0]
		}
	}
	f.mu.Lock()
	f.methods = append(f.methods, method)
	var next *reply
	switch method {
	case "getMe":
		rep := f.getMe
		next = &rep
	case "getUpdates":
		f.forms = append(f.forms, form)
		f.polled.Add(1)
		if len(f.polls) > 0 {
			rep := f.polls[0]
			f.polls = f.polls[1:]
			next = &rep
		}
	}
	f.mu.Unlock()
	if method == "getUpdates" && f.onPoll != nil {
		f.onPoll(form)
	}
	if next == nil {
		select {
		case <-r.Context().Done():
		case <-f.held:
		}
		return
	}
	if next.after != nil {
		select {
		case <-next.after:
		case <-r.Context().Done():
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(next.status)
	_, _ = w.Write([]byte(next.body))
}

func (f *fakeTelegram) pollForms() []map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]string(nil), f.forms...)
}

func privateMessage(updateID, userID int64, text string) string {
	return fmt.Sprintf(`{"update_id":%d,"message":{"message_id":1,"date":1,"from":{"id":%d,"is_bot":false,"first_name":"Vasya","username":"vasya_nick"},"chat":{"id":%d,"type":"private"},"text":%q}}`,
		updateID, userID, userID, text)
}

func groupMessage(updateID, userID, chatID int64, text string) string {
	return fmt.Sprintf(`{"update_id":%d,"message":{"message_id":2,"date":1,"from":{"id":%d,"is_bot":false,"first_name":"Lena"},"chat":{"id":%d,"type":"group","title":"friends"},"text":%q}}`,
		updateID, userID, chatID, text)
}

func testLogger(buf *bytes.Buffer) (*slog.Logger, *privacy.Redactor) {
	redactor := privacy.NewRedactor(testToken)
	inner := logging.New(logging.Options{Service: "telegram-bot", Level: slog.LevelDebug, Output: buf}).Handler()
	return slog.New(privacy.NewHandler(inner, redactor)), redactor
}

func TestTelegramHandsUpdatesOverInOrderAndStopsWithTheContext(t *testing.T) {
	tg := newFakeTelegram(t, okResult("["+privateMessage(10, 987654321, "/look")+","+groupMessage(11, 555, -100200, "/start")+"]"))
	srv := httptest.NewServer(tg)
	defer srv.Close()

	var buf bytes.Buffer
	log, redactor := testLogger(&buf)
	src := NewTelegram(TelegramOptions{Token: testToken, Log: log, Redactor: redactor, ServerURL: srv.URL})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var (
		got    []Update
		inside atomic.Int32
	)
	err := src.Start(ctx, func(_ context.Context, u Update) {
		if inside.Add(1) != 1 {
			t.Error("handlers ran concurrently: updates must be handled one at a time")
		}
		got = append(got, u)
		if len(got) == 2 {
			cancel()
		}
		inside.Add(-1)
	})
	if err != nil {
		t.Fatalf("Start after cancel = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("handled %d updates, want 2", len(got))
	}
	first, second := got[0], got[1]
	if first.ID != 10 || first.Message == nil || first.Message.Chat.Type != ChatPrivate || first.Message.Chat.ID != 987654321 ||
		first.Message.From == nil || first.Message.From.ID != 987654321 || first.Message.Text != "/look" ||
		first.Message.From.Username != "vasya_nick" || first.Message.From.FirstName != "Vasya" {
		t.Fatalf("first update mapped as %#v / %#v / %#v", first, *first.Message, *first.Message.From)
	}
	if second.ID != 11 || second.Message.Chat.Type != "group" || second.Message.Chat.ID != -100200 || second.Message.From.ID != 555 {
		t.Fatal("second update mapped wrongly")
	}

	form := tg.pollForms()[0]
	if form["timeout"] != "25" {
		t.Fatalf("getUpdates timeout = %q, want 25 (ADR-006)", form["timeout"])
	}
	var allowed []string
	if err := json.Unmarshal([]byte(form["allowed_updates"]), &allowed); err != nil || len(allowed) != 1 || allowed[0] != "message" {
		t.Fatalf("allowed_updates = %q, want [\"message\"] (ADR-006)", form["allowed_updates"])
	}
	if strings.Contains(buf.String(), "ABCdefGHI") {
		t.Fatalf("token in the log:\n%s", buf.String())
	}
}

func TestTelegramStopsWithErrConflictOn409(t *testing.T) {
	conflict := reply{status: http.StatusConflict,
		body: `{"ok":false,"error_code":409,"description":"Conflict: terminated by other getUpdates request; see https://api.telegram.org/bot` + testToken + `/getUpdates"}`}
	// The 409 waits until update 1 is handled. Answered at once, it could
	// cancel the context between the hand-over of update 1 and its handler,
	// and an update handed over with an ended context is not handled.
	handledFirst := make(chan struct{})
	conflict.after = handledFirst
	tg := newFakeTelegram(t, okResult("["+privateMessage(1, 987654321, "/start")+"]"), conflict, conflict)
	srv := httptest.NewServer(tg)
	defer srv.Close()

	var buf bytes.Buffer
	log, redactor := testLogger(&buf)
	src := NewTelegram(TelegramOptions{Token: testToken, Log: log, Redactor: redactor, ServerURL: srv.URL})

	handled := 0
	err := src.Start(context.Background(), func(context.Context, Update) {
		handled++
		if handled == 1 {
			close(handledFirst)
		}
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Start = %v, want ErrConflict", err)
	}
	if code := ExitCode(err); code != 3 {
		t.Fatalf("ExitCode(conflict) = %d, want 3", code)
	}
	if handled != 1 {
		t.Fatalf("handled %d updates before the conflict, want 1", handled)
	}
	if n := tg.polled.Load(); n != 2 {
		t.Fatalf("getUpdates called %d times: polling must stop at the first 409", n)
	}
	out := buf.String()
	if strings.Contains(out, "ABCdefGHI") || strings.Contains(out, testToken) || strings.Contains(err.Error(), "ABCdefGHI") {
		t.Fatalf("token reached the log or the error:\n%s\n%v", out, err)
	}
	if !strings.Contains(out, "409 Conflict") || !strings.Contains(out, `"handled":true`) {
		t.Fatalf("the conflict must be logged once as handled:\n%s", out)
	}
}

func TestTelegramRedactsTheErrorsOfTheLibrary(t *testing.T) {
	serverError := reply{status: http.StatusBadGateway,
		body: `{"ok":false,"error_code":502,"description":"Bad Gateway at /bot` + testToken + `/getUpdates"}`}
	stop := reply{status: http.StatusConflict, body: `{"ok":false,"error_code":409,"description":"Conflict"}`}
	tg := newFakeTelegram(t, serverError, stop)
	srv := httptest.NewServer(tg)
	defer srv.Close()

	var buf bytes.Buffer
	log, redactor := testLogger(&buf)
	err := NewTelegram(TelegramOptions{Token: testToken, Log: log, Redactor: redactor, ServerURL: srv.URL}).
		Start(context.Background(), func(context.Context, Update) {})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Start = %v, want ErrConflict", err)
	}
	out := buf.String()
	if !strings.Contains(out, "telegram client error") || !strings.Contains(out, "502") {
		t.Fatalf("the 502 must be logged:\n%s", out)
	}
	if strings.Contains(out, "ABCdefGHI") {
		t.Fatalf("token reached the log:\n%s", out)
	}
}

func TestTelegramStartReportsARejectedTokenWithoutIt(t *testing.T) {
	tg := newFakeTelegram(t)
	tg.getMe = reply{status: http.StatusUnauthorized,
		body: `{"ok":false,"error_code":401,"description":"Unauthorized: https://api.telegram.org/bot` + testToken + `/getMe"}`}
	srv := httptest.NewServer(tg)
	defer srv.Close()

	err := NewTelegram(TelegramOptions{Token: testToken, Redactor: privacy.NewRedactor(testToken), ServerURL: srv.URL}).
		Start(context.Background(), func(context.Context, Update) { t.Error("no update expected") })
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Start = %v, want ErrUnauthorized", err)
	}
	if strings.Contains(err.Error(), "ABCdefGHI") {
		t.Fatalf("token in the error: %v", err)
	}
	if ExitCode(err) != 1 {
		t.Fatalf("ExitCode(unauthorized) = %d, want 1", ExitCode(err))
	}
	if tg.polled.Load() != 0 {
		t.Fatal("getUpdates must not be called with a rejected token")
	}

	srv.Close()
	err = NewTelegram(TelegramOptions{Token: testToken, ServerURL: srv.URL}).Start(context.Background(), func(context.Context, Update) {})
	if err == nil || errors.Is(err, ErrUnauthorized) || strings.Contains(err.Error(), "ABCdefGHI") {
		t.Fatalf("Start against a closed server = %v, want a redacted start error", err)
	}
	if err := NewTelegram(TelegramOptions{Token: testToken}).Start(context.Background(), nil); err == nil {
		t.Fatal("Start without a handler must fail")
	}
}

// TestTheLibraryNeverRunsInDebugMode checks the configuration itself: the
// options of Start plus a debug handler that records. In debug mode the library
// reports every non-empty response through that handler; the update below is
// such a response.
func TestTheLibraryNeverRunsInDebugMode(t *testing.T) {
	tg := newFakeTelegram(t, okResult("["+privateMessage(7, 987654321, "/look")+"]"))
	srv := httptest.NewServer(tg)
	defer srv.Close()

	var debugCalls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src := NewTelegram(TelegramOptions{Token: testToken, ServerURL: srv.URL})
	opts := append(src.options(func(context.Context, Update) { cancel() }, func(error) {}),
		bot.WithDebugHandler(func(string, ...any) { debugCalls.Add(1) }))
	b, err := bot.New(testToken, opts...)
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}
	b.Start(ctx)
	if n := debugCalls.Load(); n != 0 {
		t.Fatalf("the library ran in debug mode (%d debug lines): WithDebug must not be configured", n)
	}
}

// TestNoFileOfTheBotEnablesDebug is the static half of the same rule: no
// shipped file of cmd/telegram-bot calls bot.WithDebug.
func TestNoFileOfTheBotEnablesDebug(t *testing.T) {
	root := filepath.Join("..", "..")
	found := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		found++
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithDebug" {
				t.Errorf("%s uses WithDebug: the library would log payloads with ids and texts (ADR-018 p. 1)", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("no source file found under cmd/telegram-bot: the check proves nothing")
	}
}

func TestUpdatesPrintRedacted(t *testing.T) {
	// The update id is long and distinctive: a short one would match the
	// digits of a timestamp.
	u := Update{ID: 700012345, Message: &Message{Chat: Chat{ID: 987654321, Type: ChatPrivate}, From: &User{ID: 987654321, Username: "vasya_nick"}, Text: "secret words"}}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", slog.Any("u", u), slog.Any("m", *u.Message), slog.Any("c", u.Message.Chat), slog.Any("f", *u.Message.From))
	for _, out := range []string{buf.String(), fmt.Sprintf("%v %+v", u, u), fmt.Sprint(*u.Message, u.Message.Chat, *u.Message.From)} {
		for _, leaked := range []string{"987654321", "vasya_nick", "secret words", "700012345"} {
			if strings.Contains(out, leaked) {
				t.Fatalf("%q printed: %s", leaked, out)
			}
		}
	}
}

func TestExitCode(t *testing.T) {
	for err, want := range map[error]int{nil: 0, ErrConflict: 3, fmt.Errorf("wrapped: %w", ErrConflict): 3, ErrUnauthorized: 1, errors.New("other"): 1} {
		if got := ExitCode(err); got != want {
			t.Errorf("ExitCode(%v) = %d, want %d", err, got, want)
		}
	}
}

// lateForConfirmation bounds how long the handler of update 1 watches for a
// getUpdates that would confirm the updates behind it. Without the unbuffered
// hand-over that request leaves within microseconds of the batch.
const lateForConfirmation = 300 * time.Millisecond

// TestTelegramConfirmsNoUpdateBeforeItIsInHand pins M-1 of review #1: the
// library confirms a batch with the next getUpdates. While update 1 of a batch
// of 3 is handled no getUpdates may leave with an offset past 2, and once the
// context ends during update 1, updates 2 and 3 are not handled: Telegram,
// which saw neither confirmed, delivers them again.
func TestTelegramConfirmsNoUpdateBeforeItIsInHand(t *testing.T) {
	batch := "[" + privateMessage(1, 987654321, "/look") + "," + privateMessage(2, 987654321, "/look") + "," + privateMessage(3, 987654321, "/look") + "]"
	tg := newFakeTelegram(t, okResult(batch))
	var handling atomic.Bool
	confirmedEarly := make(chan string, 16)
	tg.onPoll = func(form map[string]string) {
		if offset, _ := strconv.ParseInt(form["offset"], 10, 64); offset > 2 && handling.Load() {
			confirmedEarly <- form["offset"]
		}
	}
	srv := httptest.NewServer(tg)
	defer srv.Close()
	defer tg.releaseHeld()

	var buf bytes.Buffer
	log, redactor := testLogger(&buf)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var handled []int64
	err := NewTelegram(TelegramOptions{Token: testToken, Log: log, Redactor: redactor, ServerURL: srv.URL}).
		Start(ctx, func(hctx context.Context, u Update) {
			handled = append(handled, u.ID)
			if u.ID != 1 {
				return
			}
			handling.Store(true)
			wait, stopWaiting := context.WithTimeout(context.Background(), lateForConfirmation)
			defer stopWaiting()
			select {
			case offset := <-confirmedEarly:
				t.Errorf("getUpdates left with offset %s while update 1 was handled: updates 2 and 3 were confirmed before they were in hand", offset)
			case <-wait.Done():
			}
			cancel()
			handling.Store(false)
			if hctx.Err() == nil {
				t.Error("the handler context must end with the context of Start")
			}
		})
	if err != nil {
		t.Fatalf("Start after cancel = %v, want nil", err)
	}
	if len(handled) != 1 || handled[0] != 1 {
		t.Fatalf("handled %v, want [1]: no update is handled once the context ended", handled)
	}
	if n := tg.polled.Load(); n != 1 {
		t.Fatalf("getUpdates called %d times, want 1: nothing past update 1 was confirmed", n)
	}
	if out := buf.String(); strings.Contains(out, "telegram client error") {
		t.Fatalf("a stop with updates left to Telegram is not an error:\n%s", out)
	}
}

func TestAdaptSkipsAnUpdateHandedOverAfterTheContextEnded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	adapt(func(context.Context, Update) { called = true })(ctx, nil, &models.Update{ID: 5})
	if called {
		t.Fatal("an update handed over after the context ended reached the handler")
	}
	adapt(func(context.Context, Update) { called = true })(context.Background(), nil, nil)
	if called {
		t.Fatal("a nil update reached the handler")
	}
	adapt(func(context.Context, Update) { called = true })(context.Background(), nil, &models.Update{ID: 6})
	if !called {
		t.Fatal("an update with a live context must reach the handler")
	}
}

// TestTelegramStopsWithErrUnauthorizedWhenTheTokenIsRevoked: a 401 of getUpdates
// is not waited out and polled again forever; the source stops.
func TestTelegramStopsWithErrUnauthorizedWhenTheTokenIsRevoked(t *testing.T) {
	revoked := reply{status: http.StatusUnauthorized,
		body: `{"ok":false,"error_code":401,"description":"Unauthorized: see /bot` + testToken + `/getUpdates"}`}
	tg := newFakeTelegram(t, revoked, revoked)
	srv := httptest.NewServer(tg)
	defer srv.Close()
	defer tg.releaseHeld()

	var buf bytes.Buffer
	log, redactor := testLogger(&buf)
	// The bound only keeps a source that polls on from hanging the test; a
	// source that stops returns long before it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := NewTelegram(TelegramOptions{Token: testToken, Log: log, Redactor: redactor, ServerURL: srv.URL}).
		Start(ctx, func(context.Context, Update) { t.Error("no update expected") })
	if !errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrConflict) {
		t.Fatalf("Start = %v, want ErrUnauthorized", err)
	}
	if code := ExitCode(err); code != 1 {
		t.Fatalf("ExitCode(revoked token) = %d, want 1", code)
	}
	if n := tg.polled.Load(); n != 1 {
		t.Fatalf("getUpdates called %d times: polling must stop at the first 401", n)
	}
	out := buf.String()
	if strings.Contains(out, "ABCdefGHI") || strings.Contains(err.Error(), "ABCdefGHI") {
		t.Fatalf("token reached the log or the error:\n%s\n%v", out, err)
	}
	if strings.Count(out, "401 Unauthorized") != 1 || !strings.Contains(out, `"handled":true`) {
		t.Fatalf("the revoked token must be logged once as handled:\n%s", out)
	}
}
