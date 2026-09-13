// Package privacy keeps the token of the bot and the Telegram identities of
// players out of everything the bot writes: log lines and error texts
// (SEC-01/02, SEC-08, ADR-018 p. 7 and addendum p. 3).
//
// Two mechanisms work together. Handler is the slog.Handler of the bot: it
// drops the attributes that carry an identity whatever the level, and runs
// every string it lets through, the message included, through a Redactor.
// Redactor removes the token wherever it turns up — the URL of the Bot API is
// https://api.telegram.org/bot<token>/<method>, so any error of the HTTP
// client that quotes the URL quotes the token.
//
// The shapes of a credential are those of shared/logging; this package adds
// the exact secrets of this process and the bot<digits>:<token> form of the
// URL, which it rewrites as bot<redacted> so that a line still says what failed.
package privacy

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"multiverse-core.io/shared/logging"
)

// RedactedToken replaces the token inside a Bot API URL: bot<digits>:<token>
// becomes bot<redacted>.
const RedactedToken = "bot<redacted>"

// redactedSecret replaces a registered secret found anywhere else.
const redactedSecret = "<redacted>"

// botTokenInURL is the path segment of the Bot API. The tail is matched at any
// length: a test token or a truncated one is still a token.
var botTokenInURL = regexp.MustCompile(`bot\d+:[A-Za-z0-9_-]+`)

// responseBody is how the library reports an answer it could not decode: the
// whole body follows the method name, and a body of getUpdates or sendMessage
// carries the ids, names and texts of players. Everything from the body on is
// cut, the decoding error included: it comes after the body and cannot be told
// apart from a body that imitates it.
var responseBody = regexp.MustCompile(`(?s)(error decode response body for method \w+,).*`)

// RedactedBody replaces the body quoted by the library.
const RedactedBody = "[body redacted]"

// minSecretLen keeps a short secret from being registered: replacing every
// occurrence of a three-letter value would mangle ordinary words of a line,
// and such a value is not a secret worth the name anyway.
const minSecretLen = 8

// Redact removes the shapes of a credential from s: the bot<digits>:<token>
// segment of a Bot API URL and whatever shared/logging recognises. It also cuts
// the response body the library quotes when it cannot decode an answer.
func Redact(s string) string {
	if s == "" {
		return s
	}
	if strings.Contains(s, "error decode response body") {
		s = responseBody.ReplaceAllString(s, "${1} "+RedactedBody)
	}
	if strings.Contains(s, "bot") {
		s = botTokenInURL.ReplaceAllString(s, RedactedToken)
	}
	return logging.Redact(s)
}

// Redactor removes the exact secrets of the process on top of the shapes
// Redact knows. A token in an unexpected form — without the bot prefix,
// URL-escaped, or only its secret half after the colon — is still removed.
type Redactor struct {
	secrets []string
}

// NewRedactor registers the secrets of the process. Empty and short values are
// ignored (minSecretLen).
func NewRedactor(secrets ...string) *Redactor {
	r := &Redactor{}
	for _, s := range secrets {
		r.add(s)
		if i := strings.IndexByte(s, ':'); i >= 0 {
			r.add(s[i+1:])
		}
	}
	return r
}

func (r *Redactor) add(s string) {
	for _, form := range []string{s, url.QueryEscape(s)} {
		if len(form) < minSecretLen {
			continue
		}
		dup := false
		for _, known := range r.secrets {
			dup = dup || known == form
		}
		if !dup {
			r.secrets = append(r.secrets, form)
		}
	}
}

// Redact removes the registered secrets and then the shapes of Redact. A nil
// Redactor still applies the shapes.
func (r *Redactor) Redact(s string) string {
	if r != nil {
		for _, secret := range r.secrets {
			if strings.Contains(s, "bot"+secret) {
				s = strings.ReplaceAll(s, "bot"+secret, RedactedToken)
			}
			s = strings.ReplaceAll(s, secret, redactedSecret)
		}
	}
	return Redact(s)
}

// droppedKeys are the attributes the bot never logs, at any level (component
// §11.1). The first four are named by ADR-018 p. 7; the rest are the same
// identity under the names a Telegram update gives it. The last four are the
// parts of an update: under them a plain key such as id is an identity, so a
// group of that name is muted whole, and a value under that key is dropped.
var droppedKeys = map[string]bool{
	"from":        true,
	"chat":        true,
	"message":     true,
	"update":      true,
	"external_id": true,
	"chat_id":     true,
	"username":    true,
	"text":        true,
	"user_id":     true,
	"from_id":     true,
	"first_name":  true,
	"last_name":   true,
	"token":       true,
	"bot_token":   true,
}

// telegramModels is the package of the Telegram types. A value of it logged
// whole would print the ids, names and text of an update, so it never reaches
// the output, whatever key it is logged under.
const telegramModels = "github.com/go-telegram/bot/models"

// Handler is the slog.Handler of the bot (component §3, internal/privacy).
type Handler struct {
	inner    slog.Handler
	redactor *Redactor
	// muted is set under a group whose name is itself a dropped key: nothing
	// logged inside it can be told apart from the identity it names.
	muted bool
}

// NewHandler wraps inner. The redactor may be nil; the shapes of Redact still
// apply.
func NewHandler(inner slog.Handler, redactor *Redactor) *Handler {
	return &Handler{inner: inner, redactor: redactor}
}

// Enabled reports what the inner handler reports: dropping an attribute does
// not depend on the level.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle writes the record with the message redacted and the identities
// dropped.
func (h *Handler) Handle(ctx context.Context, rec slog.Record) error {
	out := slog.NewRecord(rec.Time, rec.Level, h.redactor.Redact(rec.Message), rec.PC)
	if !h.muted {
		rec.Attrs(func(a slog.Attr) bool {
			if clean, ok := h.clean(a); ok {
				out.AddAttrs(clean)
			}
			return true
		})
	}
	return h.inner.Handle(ctx, out)
}

// WithAttrs cleans the attributes once, when they are attached.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h.muted {
		return h
	}
	kept := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if clean, ok := h.clean(a); ok {
			kept = append(kept, clean)
		}
	}
	return &Handler{inner: h.inner.WithAttrs(kept), redactor: h.redactor}
}

// WithGroup opens a group; a group named after a dropped key mutes everything
// logged inside it.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &Handler{
		inner:    h.inner.WithGroup(h.redactor.Redact(name)),
		redactor: h.redactor,
		muted:    h.muted || droppedKeys[strings.ToLower(name)],
	}
}

func (h *Handler) clean(a slog.Attr) (slog.Attr, bool) {
	if droppedKeys[strings.ToLower(a.Key)] {
		return slog.Attr{}, false
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.redactor.Redact(v.String())), true
	case slog.KindGroup:
		var kept []any
		for _, sub := range v.Group() {
			if clean, ok := h.clean(sub); ok {
				kept = append(kept, clean)
			}
		}
		if len(kept) == 0 {
			return slog.Attr{}, false
		}
		return slog.Group(a.Key, kept...), true
	case slog.KindAny:
		return h.cleanAny(a.Key, v.Any())
	default:
		return slog.Attr{Key: a.Key, Value: v}, true
	}
}

func (h *Handler) cleanAny(key string, value any) (slog.Attr, bool) {
	switch x := value.(type) {
	case nil:
		return slog.Any(key, nil), true
	case error:
		return slog.String(key, h.redactor.Redact(x.Error())), true
	}
	if fromTelegramModels(value) {
		return slog.String(key, logging.Redacted), true
	}
	return slog.String(key, h.redactor.Redact(fmtValue(value))), true
}

// fmtValue renders a value the JSON handler would otherwise marshal field by
// field, so that its text passes through the redactor like any other string.
func fmtValue(value any) string { return fmt.Sprintf("%v", value) }

func fromTelegramModels(value any) bool {
	t := reflect.TypeOf(value)
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	return t.PkgPath() == telegramModels
}

// ErrorLog is the errors handler of the Telegram library
// (bot.WithErrorsHandler): every error is logged with its text redacted
// (ADR-018 addendum p. 3). The logger should itself be built on Handler; the
// redaction here does not rely on it.
func ErrorLog(log *slog.Logger, redactor *Redactor) func(error) {
	return func(err error) {
		if err == nil {
			return
		}
		log.Error("telegram client error", slog.String("error", redactor.Redact(err.Error())))
	}
}
