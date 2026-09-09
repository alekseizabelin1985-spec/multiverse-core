// Package logging builds the logger of every process: JSON on stdout, the
// fields the platform expects on every line, and a redaction pass that keeps
// personal data and credentials out of the log by construction
// (infrastructure.md §7.1, ADR-009, NFR-041).
//
// Two rules make the redaction hold instead of relying on care at each call
// site. First, a value under a sensitive key never reaches the output — the
// key is what marks it, so a caller does not have to remember. Second, every
// string that does reach the output, the message included, is scanned for the
// shapes of a credential: a Telegram bot token and an API key look the same
// wherever they turn up, including inside an error somebody wrapped.
//
// What the platform must not log at all — request bodies, Telegram updates,
// the text of what a player said — is not a matter of redaction: those never
// become attributes. The logger takes explicit fields, never a whole event.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
)

// Redacted replaces a value that must not appear in the log.
const Redacted = "[redacted]"

// TimeKey is the name of the timestamp field of a log line. slog writes time;
// infrastructure.md §7.1 lists ts, and the jq recipes there select on it.
const TimeKey = "ts"

// sensitiveKeys are the attribute keys whose value never reaches the output
// (infrastructure.md §7.1: external_id, token, authorization, api_key, text;
// the rest are the obvious neighbours of the same kind).
//
// chat_id, first_name, last_name, update_id and username are here because
// threat-model T-06 and T-08 name them one by one: a bot library that logs a
// raw update writes exactly those fields, and both threats are rated Major.
var sensitiveKeys = map[string]bool{
	"api_key":       true,
	"apikey":        true,
	"authorization": true,
	"bot_token":     true,
	"chat_id":       true,
	"external_id":   true,
	"first_name":    true,
	"last_name":     true,
	"password":      true,
	"secret":        true,
	"text":          true,
	"token":         true,
	"update_id":     true,
	"username":      true,
}

// credentialPattern is one shape a credential has, with the literal every
// string matching it must contain. The literal is the fast path: redaction
// runs over every string of every line, and a plain substring scan is an order
// of magnitude cheaper than the regexp. Adding a pattern means stating its
// literal, and the literal must be implied by the expression — otherwise the
// fast path would skip a credential.
type credentialPattern struct {
	must string
	re   *regexp.Regexp
}

// credentialPatterns are the shapes a credential has wherever it appears: a
// Telegram bot token (digits, a colon, a long opaque tail) and an API key of
// the sk- family.
var credentialPatterns = []credentialPattern{
	{must: ":", re: regexp.MustCompile(`(?i)\b(?:bot)?\d{6,}:[A-Za-z0-9_-]{30,}`)},
	{must: "sk-", re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`)},
}

// Version is the build of the process, stamped by the linker:
//
//	-ldflags "-X multiverse-core.io/shared/logging.Version=<git sha>"
//
// It is what Init puts in the version field infrastructure.md §7.1 requires on
// every line. A build that stamps nothing logs dev, which is what a local run
// is; New still takes an explicit Options.Version for a process that knows its
// own.
var Version = "dev"

// Options describes the logger of a process.
type Options struct {
	// Service is gateway, core, memory or telegram-bot.
	Service string
	// Version is the build of the process, stamped by the linker.
	Version string
	Level   slog.Level
	// Format is json or text; anything else is json, because the platform
	// ships JSON and a typo must not silently change the format of the logs.
	Format string
	// Output defaults to stdout.
	Output io.Writer
}

// New builds the logger described by opts.
func New(opts Options) *slog.Logger {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	handlerOpts := &slog.HandlerOptions{Level: opts.Level, ReplaceAttr: redactAttr}
	var handler slog.Handler
	if opts.Format == "text" {
		handler = slog.NewTextHandler(out, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(out, handlerOpts)
	}
	attrs := []slog.Attr{slog.String("service", opts.Service)}
	if opts.Version != "" {
		attrs = append(attrs, slog.String("version", opts.Version))
	}
	return slog.New(handler.WithAttrs(attrs))
}

// Init builds the JSON logger of a service on stdout at the given level. It is
// the short form of New for a process that has nothing to override; the
// version comes from the linker through Version.
func Init(service string, level slog.Level) *slog.Logger {
	return New(Options{Service: service, Version: Version, Level: level})
}

// LevelFromEnv reads MV_LOG_LEVEL.
func LevelFromEnv() (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(env.LogLevel.String())); err != nil {
		return 0, err
	}
	return level, nil
}

// FormatFromEnv reads MV_LOG_FORMAT.
func FormatFromEnv() string { return env.LogFormat.String() }

// Fields are the attributes that identify what a line is about. They travel in
// the context so that a handler deep inside a context logs them without being
// handed the event.
type Fields struct {
	// Context is the bounded context that is running (state, swarm, gateway).
	Context       string
	CorrelationID string
	EventID       string
	EventType     string
	// AgentLevel is set for a line produced while handling an event of the
	// swarm (global, domain, encounter, personal).
	AgentLevel string
}

type fieldsKey struct{}

// ContextWith attaches the fields to ctx, replacing whatever was there.
func ContextWith(ctx context.Context, f Fields) context.Context {
	return context.WithValue(ctx, fieldsKey{}, f)
}

// WithContextName attaches the name of the bounded context, keeping the event
// fields already in ctx.
func WithContextName(ctx context.Context, name string) context.Context {
	f, _ := FieldsFrom(ctx)
	f.Context = name
	return ContextWith(ctx, f)
}

// FieldsFrom returns the fields attached to ctx.
func FieldsFrom(ctx context.Context) (Fields, bool) {
	f, ok := ctx.Value(fieldsKey{}).(Fields)
	return f, ok
}

// EventFields reads the fields of an event envelope (C-01).
func EventFields(ev eventbus.Event) Fields {
	f := Fields{
		CorrelationID: ev.CorrelationID(),
		EventID:       ev.ID,
		EventType:     ev.Type,
	}
	if ev.Meta.Agent != nil {
		f.AgentLevel = ev.Meta.Agent.Level
	}
	return f
}

// With returns the logger carrying the fields of ctx. A field that is not set
// is left out rather than logged empty.
func With(ctx context.Context, l *slog.Logger) *slog.Logger {
	f, ok := FieldsFrom(ctx)
	if !ok {
		return l
	}
	attrs := make([]any, 0, 8)
	for _, pair := range []struct{ key, value string }{
		{"context", f.Context},
		{"correlation_id", f.CorrelationID},
		{"event_id", f.EventID},
		{"event_type", f.EventType},
		{"agent.level", f.AgentLevel},
	} {
		if pair.value != "" {
			attrs = append(attrs, slog.String(pair.key, pair.value))
		}
	}
	if len(attrs) == 0 {
		return l
	}
	return l.With(attrs...)
}

// BusMiddleware puts the fields of the event in the context of the handler and
// reports a handler that failed. The line says handled=false: the delivery
// will retry, and only the dead letter that follows the last attempt is the
// end of the story (LogDeadLetter).
func BusMiddleware(l *slog.Logger) eventbus.Middleware {
	return func(next eventbus.Handler) eventbus.Handler {
		return func(ctx context.Context, ev eventbus.Event) error {
			f, _ := FieldsFrom(ctx)
			event := EventFields(ev)
			event.Context = f.Context
			ctx = ContextWith(ctx, event)

			err := next(ctx, ev)
			if err != nil {
				With(ctx, l).ErrorContext(ctx, "event handler failed",
					slog.Bool("handled", false),
					slog.String("error", Redact(err.Error())))
			}
			return err
		}
	}
}

// LogDeadLetter reports an event parked in dead_letters: handled=true, because
// nothing else will be tried.
func LogDeadLetter(ctx context.Context, l *slog.Logger, dl eventbus.DeadLetter) {
	ctx = ContextWith(ctx, EventFields(dl.Original))
	With(ctx, l).ErrorContext(ctx, "event parked in dead letters",
		slog.Bool("handled", true),
		slog.String("consumer", dl.Consumer),
		slog.Int("attempts", dl.Attempts),
		slog.String("error", Redact(dl.Error)))
}

// Redact removes from s anything shaped like a credential. It is exported
// because the bot has to run it over a message it did not build (ADR-006
// add. 3).
func Redact(s string) string {
	if s == "" {
		return s
	}
	for _, p := range credentialPatterns {
		// ReplaceAllString copies the string even when nothing matches, and
		// redaction runs over every string of every line, msg included. The
		// typical line carries no credential, so it is now decided by a
		// substring scan and pays neither the regexp nor an allocation.
		if !strings.Contains(s, p.must) || !p.re.MatchString(s) {
			continue
		}
		s = p.re.ReplaceAllString(s, Redacted)
	}
	return s
}

// redactAttr is the ReplaceAttr of every handler of the platform: a sensitive
// key loses its value whatever it holds, and every other string is scanned for
// a credential. It also renames the built-in time key to ts, which is what
// infrastructure.md §7.1 names and what the jq recipes there select on. Only
// the top-level attribute is renamed: a group is free to carry its own time.
func redactAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
		a.Key = TimeKey
		return a
	}
	if sensitiveKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, Redacted)
	}
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Redact(a.Value.String()))
	case slog.KindAny:
		if err, ok := a.Value.Any().(error); ok {
			return slog.String(a.Key, Redact(err.Error()))
		}
	}
	return a
}
