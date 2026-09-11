package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/logging"
)

// botToken and apiKey are fabricated values of the shape a real credential
// has; they exist so that the redaction has something to fail on.
const (
	botToken = "123456789:AAHfake-token-value-that-is-long-enough-x"
	apiKey   = "sk-fakefakefakefake"
)

func newLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	return logging.New(logging.Options{
		Service: "core", Version: "test", Level: slog.LevelDebug, Output: &buf,
	}), &buf
}

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not JSON: %q (%v)", line, err)
		}
		out = append(out, record)
	}
	return out
}

func TestLoggerCarriesTheRequiredFields(t *testing.T) {
	logger, buf := newLogger(t)
	ctx := logging.ContextWith(context.Background(), logging.Fields{
		Context:       "state",
		CorrelationID: "corr-1",
		EventID:       "ev-1",
		EventType:     "entity.updated",
		AgentLevel:    "encounter",
	})

	logging.With(ctx, logger).InfoContext(ctx, "entity applied")

	records := lines(t, buf)
	if len(records) != 1 {
		t.Fatalf("wrote %d lines, want 1", len(records))
	}
	want := map[string]string{
		"service":        "core",
		"version":        "test",
		"level":          "INFO",
		"msg":            "entity applied",
		"context":        "state",
		"correlation_id": "corr-1",
		"event_id":       "ev-1",
		"event_type":     "entity.updated",
		"agent.level":    "encounter",
	}
	for key, value := range want {
		if got, _ := records[0][key].(string); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
	if _, ok := records[0][logging.TimeKey]; !ok {
		t.Errorf("the line carries no %s field: %v", logging.TimeKey, records[0])
	}
	if _, ok := records[0]["time"]; ok {
		t.Error("the line still carries the slog time key; infrastructure.md §7.1 names ts")
	}
}

func TestWithLeavesUnsetFieldsOut(t *testing.T) {
	logger, buf := newLogger(t)
	ctx := context.Background()

	logging.With(ctx, logger).InfoContext(ctx, "no fields yet")
	ctx = logging.WithContextName(ctx, "gateway")
	logging.With(ctx, logger).InfoContext(ctx, "named context")

	records := lines(t, buf)
	if _, ok := records[0]["correlation_id"]; ok {
		t.Error("an empty correlation_id is logged, want it left out")
	}
	if got, _ := records[1]["context"].(string); got != "gateway" {
		t.Errorf("context = %q, want gateway", got)
	}
}

// NFR-041 and SEC-02/28: an external identifier and a credential must not
// reach the log — neither under their own key, nor buried in a message or in
// an error somebody wrapped.
func TestExternalIDAndTokenNeverReachTheLog(t *testing.T) {
	logger, buf := newLogger(t)
	ctx := context.Background()

	logger.InfoContext(ctx, "link created",
		slog.String("external_id", "tg-987654321"),
		slog.String("username", "vasya"),
		slog.String("token", botToken),
		slog.String("text", "what the player actually said"),
		slog.String("api_key", apiKey))
	logger.ErrorContext(ctx, "calling the bot API with "+botToken,
		slog.Any("error", errors.New("unauthorized for key "+apiKey)))

	out := buf.String()
	for _, leaked := range []string{"tg-987654321", "vasya", botToken, apiKey, "what the player actually said"} {
		if strings.Contains(out, leaked) {
			t.Errorf("the log carries %q:\n%s", leaked, out)
		}
	}
	records := lines(t, buf)
	if got, _ := records[0]["external_id"].(string); got != logging.Redacted {
		t.Errorf("external_id = %q, want %q", got, logging.Redacted)
	}
	if msg, _ := records[1]["msg"].(string); !strings.Contains(msg, logging.Redacted) {
		t.Errorf("msg = %q, want the token replaced", msg)
	}
	if got, _ := records[1]["error"].(string); !strings.Contains(got, logging.Redacted) {
		t.Errorf("error = %q, want the key replaced", got)
	}
}

// threat-model T-06 and T-08 name the fields a bot library writes when it logs
// a raw Telegram update; each of them is personal data and none may reach the
// output under its own key.
func TestTelegramUpdateFieldsNeverReachTheLog(t *testing.T) {
	logger, buf := newLogger(t)
	ctx := context.Background()

	logger.InfoContext(ctx, "update received",
		slog.Int64("chat_id", 987654321),
		slog.Int64("update_id", 42),
		slog.String("first_name", "Вася"),
		slog.String("last_name", "Пупкин"),
		slog.String("username", "vasya"))

	out := buf.String()
	for _, leaked := range []string{"987654321", "Вася", "Пупкин", "vasya"} {
		if strings.Contains(out, leaked) {
			t.Errorf("the log carries %q:\n%s", leaked, out)
		}
	}
	record := lines(t, buf)[0]
	for _, key := range []string{"chat_id", "update_id", "first_name", "last_name", "username"} {
		if got, _ := record[key].(string); got != logging.Redacted {
			t.Errorf("%s = %v, want %q", key, record[key], logging.Redacted)
		}
	}
}

func TestRedact(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		clean bool
	}{
		{"bot token", "sent bot" + botToken + " upstream", false},
		{"bare token", botToken, false},
		{"api key", "Authorization: Bearer " + apiKey, false},
		{"ordinary text", "the player entered the dark forest", true},
		{"short colon pair", "12:30", true},
		{"empty", "", true},
	}
	for _, c := range cases {
		got := logging.Redact(c.in)
		if c.clean && got != c.in {
			t.Errorf("%s: Redact(%q) = %q, want it unchanged", c.name, c.in, got)
		}
		if !c.clean && !strings.Contains(got, logging.Redacted) {
			t.Errorf("%s: Redact(%q) = %q, want the credential replaced", c.name, c.in, got)
		}
	}
}

func TestBusMiddlewarePutsTheEventFieldsInTheContext(t *testing.T) {
	logger, buf := newLogger(t)
	event := eventbus.NewRoot("player.said", "gateway", "dark-forest", nil, "human", nil,
		eventbus.WithAgent(eventbus.AgentRef{ID: "a-1", Level: "encounter", Blueprint: "bp"}))

	var seen logging.Fields
	handler := logging.BusMiddleware(logger)(func(ctx context.Context, _ eventbus.Event) error {
		seen, _ = logging.FieldsFrom(ctx)
		logging.With(ctx, logger).InfoContext(ctx, "handled")
		return nil
	})
	if err := handler(logging.WithContextName(context.Background(), "swarm"), event); err != nil {
		t.Fatalf("handler: %v", err)
	}

	if seen.EventID != event.ID || seen.EventType != "player.said" || seen.CorrelationID != event.ID {
		t.Fatalf("fields = %+v, want the envelope of the event", seen)
	}
	if seen.AgentLevel != "encounter" || seen.Context != "swarm" {
		t.Fatalf("fields = %+v, want the agent level and the context kept", seen)
	}
	records := lines(t, buf)
	if len(records) != 1 {
		t.Fatalf("wrote %d lines, want only the one the handler logged", len(records))
	}
	if got, _ := records[0]["event_type"].(string); got != "player.said" {
		t.Errorf("event_type = %q, want player.said", got)
	}
}

func TestBusMiddlewareReportsAFailingHandlerAsUnhandled(t *testing.T) {
	logger, buf := newLogger(t)
	event := eventbus.NewRoot("player.said", "gateway", "dark-forest", nil, "human", nil)
	failure := errors.New("store unreachable with key " + apiKey)

	handler := logging.BusMiddleware(logger)(func(context.Context, eventbus.Event) error { return failure })
	if err := handler(context.Background(), event); !errors.Is(err, failure) {
		t.Fatalf("handler = %v, want the error passed through", err)
	}

	records := lines(t, buf)
	if len(records) != 1 {
		t.Fatalf("wrote %d lines, want 1", len(records))
	}
	if handled, _ := records[0]["handled"].(bool); handled {
		t.Error("handled = true before the retries are over, want false")
	}
	if got, _ := records[0]["error"].(string); strings.Contains(got, apiKey) {
		t.Errorf("error = %q, want the key replaced", got)
	}
}

func TestLogDeadLetterIsTheEndOfTheStory(t *testing.T) {
	logger, buf := newLogger(t)
	event := eventbus.NewRoot("player.said", "gateway", "dark-forest", nil, "human", nil)

	logging.LogDeadLetter(context.Background(), logger, eventbus.DeadLetter{
		Original: event, Error: "store unreachable", Consumer: "state", Attempts: 4,
	})

	records := lines(t, buf)
	if len(records) != 1 {
		t.Fatalf("wrote %d lines, want 1", len(records))
	}
	if handled, _ := records[0]["handled"].(bool); !handled {
		t.Error("handled = false for a dead letter, want true")
	}
	if got, _ := records[0]["consumer"].(string); got != "state" {
		t.Errorf("consumer = %q, want state", got)
	}
	if got, _ := records[0]["event_id"].(string); got != event.ID {
		t.Errorf("event_id = %q, want the identifier of the parked event", got)
	}
}

func TestLevelAndFormatComeFromTheManifest(t *testing.T) {
	t.Setenv("MV_LOG_LEVEL", "debug")
	level, err := logging.LevelFromEnv()
	if err != nil || level != slog.LevelDebug {
		t.Fatalf("LevelFromEnv = %v, %v; want debug, nil", level, err)
	}
	t.Setenv("MV_LOG_LEVEL", "shout")
	if _, err := logging.LevelFromEnv(); err == nil {
		t.Fatal("LevelFromEnv accepts a level that does not exist")
	}
	if got := logging.FormatFromEnv(); got != "json" {
		t.Fatalf("FormatFromEnv = %q, want the json default", got)
	}
}

func TestInitAndTextFormat(t *testing.T) {
	if logger := logging.Init("core", slog.LevelInfo); logger == nil {
		t.Fatal("Init returned no logger")
	}
	// version is required on every line (infrastructure.md §7.1); Init takes
	// it from the linker, and an unstamped build says dev rather than nothing.
	if logging.Version == "" {
		t.Fatal("logging.Version is empty; Init would drop the version field")
	}

	var buf bytes.Buffer
	logger := logging.New(logging.Options{
		Service: "telegram-bot", Format: "text", Level: slog.LevelInfo, Output: &buf,
	})
	logger.Info("update received", slog.String("token", botToken))

	out := buf.String()
	if !strings.Contains(out, "service=telegram-bot") {
		t.Errorf("text output = %q, want the service field", out)
	}
	if strings.Contains(out, botToken) {
		t.Errorf("text output carries the token: %q", out)
	}
}

// The redaction runs over every string of every line, so the case that must be
// cheap is the one where nothing matches. Compared with the same handler
// without ReplaceAttr, the difference should be time, not allocations.
func BenchmarkLogLineWithoutACredential(b *testing.B) {
	logger := logging.New(logging.Options{
		Service: "core", Version: "bench", Level: slog.LevelInfo, Output: io.Discard,
	})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Info("entity applied",
			slog.String("event_type", "entity.updated"),
			slog.String("event_id", "ev-000000000000000000000001"))
	}
}

func BenchmarkRedactWithoutAMatch(b *testing.B) {
	const line = "the player entered the dark forest and looked around carefully"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := logging.Redact(line); got != line {
			b.Fatalf("Redact changed a clean line: %q", got)
		}
	}
}
