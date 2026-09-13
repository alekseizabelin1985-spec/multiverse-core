package privacy_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/shared/logging"
)

// token has the shape BotFather issues; shortToken is the shape the DoD names
// (bot123456:ABC…), shorter than shared/logging recognises on its own.
const (
	token      = "7012345678:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsawQ"
	shortToken = "123456:ABCdef"
	externalID = "987654321"
)

func TestRedactRemovesTheTokenFromAnErrorOfTheHTTPClient(t *testing.T) {
	cases := map[string]error{
		"network error quoting the URL": &url.Error{
			Op:  "Post",
			URL: "https://api.telegram.org/bot" + shortToken + "/getUpdates",
			Err: errors.New("dial tcp: i/o timeout"),
		},
		"409 conflict of getUpdates": fmt.Errorf("error get updates, %w",
			fmt.Errorf("%w, Conflict: terminated by other getUpdates request (https://api.telegram.org/bot%s/getUpdates)",
				bot.ErrorConflict, shortToken)),
		"full token in a wrapped error": fmt.Errorf("start: %w",
			fmt.Errorf("error call getMe, Post \"https://api.telegram.org/bot%s/getMe\": EOF", token)),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			got := privacy.Redact(err.Error())
			tokenTail := token[strings.IndexByte(token, ':')+1:]
			for _, leak := range []string{shortToken, "ABCdef", token, tokenTail} {
				if strings.Contains(got, leak) {
					t.Fatalf("Redact left %q in %q", leak, got)
				}
			}
			if !strings.Contains(got, privacy.RedactedToken) {
				t.Fatalf("Redact(%q) = %q, want the segment rewritten as %s", err, got, privacy.RedactedToken)
			}
		})
	}
}

func TestRedactKeepsTheConflictReadable(t *testing.T) {
	err := fmt.Errorf("error get updates, %w, Conflict: terminated by other getUpdates request /bot%s/getUpdates",
		bot.ErrorConflict, shortToken)
	got := privacy.Redact(err.Error())
	if !strings.Contains(got, "conflict") || !strings.Contains(got, "/bot<redacted>/getUpdates") {
		t.Fatalf("Redact(%q) = %q: the reason and the method must survive", err, got)
	}
}

func TestRedactorRemovesTheExactSecretInAnyForm(t *testing.T) {
	r := privacy.NewRedactor(token, "", "short")
	secretHalf := token[strings.IndexByte(token, ':')+1:]
	for _, in := range []string{
		"token=" + token,
		"escaped=" + url.QueryEscape(token),
		"half=" + secretHalf,
		"url=https://api.telegram.org/bot" + token + "/sendMessage",
	} {
		got := r.Redact(in)
		if strings.Contains(got, secretHalf) || strings.Contains(got, url.QueryEscape(secretHalf)) {
			t.Fatalf("Redactor.Redact(%q) = %q, secret left", in, got)
		}
	}
	if got := r.Redact("a short word stays"); got != "a short word stays" {
		t.Fatalf("a secret shorter than the minimum must not be registered: %q", got)
	}
	var nilRedactor *privacy.Redactor
	if got := nilRedactor.Redact("bot" + shortToken); strings.Contains(got, "ABCdef") {
		t.Fatalf("nil Redactor must still apply the shapes: %q", got)
	}
}

// logger is the logger of the bot as main builds it: the platform JSON handler
// under the privacy handler.
func logger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	inner := logging.New(logging.Options{Service: "telegram-bot", Level: level, Output: buf}).Handler()
	return slog.New(privacy.NewHandler(inner, privacy.NewRedactor(token)))
}

// inners are the handlers the privacy handler is tested over: the platform one,
// and a bare JSON handler without the redaction of shared/logging, so that
// what the privacy handler does is not hidden by what the platform does again.
var inners = map[string]func(*bytes.Buffer) slog.Handler{
	"platform": func(buf *bytes.Buffer) slog.Handler {
		return logging.New(logging.Options{Service: "telegram-bot", Level: slog.LevelDebug, Output: buf}).Handler()
	},
	"bare": func(buf *bytes.Buffer) slog.Handler {
		return slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	},
}

func TestHandlerDropsIdentitiesAtEveryLevel(t *testing.T) {
	for innerName, inner := range inners {
		for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
			t.Run(innerName+"/"+level.String(), func(t *testing.T) {
				testDropsIdentities(t, inner, level)
			})
		}
	}
}

func testDropsIdentities(t *testing.T, inner func(*bytes.Buffer) slog.Handler, level slog.Level) {
	var buf bytes.Buffer
	log := slog.New(privacy.NewHandler(inner(&buf), privacy.NewRedactor(token)))
	log.Log(context.Background(), level, "update handled",
		slog.String("external_id", externalID),
		slog.Int64("chat_id", 555000111),
		slog.String("username", "vasya_secret_nick"),
		slog.String("text", "my private words"),
		slog.String("kept", "visible"))
	log.With(slog.String("Chat_ID", "555000222")).
		WithGroup("round").
		Log(context.Background(), level, "grouped", slog.Group("actor", slog.String("username", "vasya_secret_nick"), slog.Int("n", 1)))

	out := buf.String()
	for _, leaked := range []string{externalID, "555000111", "555000222", "vasya_secret_nick", "my private words", "external_id", "chat_id", "Chat_ID", "username", `"text"`} {
		if strings.Contains(out, leaked) {
			t.Fatalf("level %s: %q reached the log:\n%s", level, leaked, out)
		}
	}
	if !strings.Contains(out, `"kept":"visible"`) || !strings.Contains(out, `"n":1`) {
		t.Fatalf("level %s: an ordinary attribute was dropped:\n%s", level, out)
	}
}

func TestHandlerRedactsTheTokenInMessagesAttributesAndErrors(t *testing.T) {
	for innerName, inner := range inners {
		t.Run(innerName, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(privacy.NewHandler(inner(&buf), privacy.NewRedactor(token, shortToken)))
			err := &url.Error{Op: "Post", URL: "https://api.telegram.org/bot" + token + "/getUpdates", Err: errors.New("EOF")}
			log.Error("poll failed for bot"+token+" and bot"+shortToken, slog.Any("error", err), slog.String("url", "/bot"+token+"/x"))
			log.With(slog.String("with", "bot"+shortToken)).WithGroup("bot"+token).Info("group name", slog.Int("n", 1))
			privacy.ErrorLog(log, privacy.NewRedactor(token))(err)
			privacy.ErrorLog(log, nil)(nil)

			out := buf.String()
			if strings.Contains(out, "AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsawQ") || strings.Contains(out, "ABCdef") {
				t.Fatalf("token reached the log:\n%s", out)
			}
			if strings.Count(out, "telegram client error") != 1 {
				t.Fatalf("ErrorLog must log one line per error and none for nil:\n%s", out)
			}
		})
	}
}

// TestErrorLogRedactsOnItsOwn: the errors handler of the library must not leak
// the token even through a logger that was not built on the privacy handler.
func TestErrorLogRedactsOnItsOwn(t *testing.T) {
	var buf bytes.Buffer
	bare := slog.New(slog.NewJSONHandler(&buf, nil))
	privacy.ErrorLog(bare, privacy.NewRedactor(shortToken))(fmt.Errorf("error get updates, %w, see /bot%s/getUpdates", bot.ErrorConflict, shortToken))
	if strings.Contains(buf.String(), "ABCdef") || !strings.Contains(buf.String(), "conflict") {
		t.Fatalf("ErrorLog wrote:\n%s", buf.String())
	}
}

func TestHandlerNeverPrintsATelegramUpdate(t *testing.T) {
	var buf bytes.Buffer
	log := logger(&buf, slog.LevelDebug)
	upd := &models.Update{ID: 42, Message: &models.Message{
		From: &models.User{ID: 987654321, Username: "vasya_secret_nick"},
		Chat: models.Chat{ID: 987654321, Type: models.ChatTypePrivate},
		Text: "my private words",
	}}
	log.Debug("raw update", slog.Any("u", upd), slog.Any("list", []*models.Update{upd}), slog.Any("nothing", nil))
	out := buf.String()
	for _, leaked := range []string{externalID, "vasya_secret_nick", "my private words"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("%q reached the log:\n%s", leaked, out)
		}
	}
	if !strings.Contains(out, `"u":"[redacted]"`) {
		t.Fatalf("the update must be replaced, not dropped silently:\n%s", out)
	}
}

func TestHandlerMutesAGroupNamedAfterAnIdentity(t *testing.T) {
	var buf bytes.Buffer
	log := logger(&buf, slog.LevelDebug)
	log.WithGroup("chat_id").With(slog.String("value", "555000333")).Info("muted", slog.String("more", "555000444"))
	out := buf.String()
	if strings.Contains(out, "555000333") || strings.Contains(out, "555000444") || !strings.Contains(out, "muted") {
		t.Fatalf("a group named after an identity must keep the line and lose its attributes:\n%s", out)
	}
}

func TestHandlerFollowsTheLevelOfTheInnerHandler(t *testing.T) {
	var buf bytes.Buffer
	log := logger(&buf, slog.LevelInfo)
	if log.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug must be disabled when the inner handler is at info")
	}
	log.Debug("hidden")
	if buf.Len() != 0 {
		t.Fatalf("debug line written at level info: %s", buf.String())
	}
}

// TestHandlerMutesThePartsOfAnUpdate: under from, chat, message or update an
// ordinary key such as id is an identity, whichever way the group is built.
func TestHandlerMutesThePartsOfAnUpdate(t *testing.T) {
	for innerName, inner := range inners {
		t.Run(innerName, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(privacy.NewHandler(inner(&buf), privacy.NewRedactor(token)))
			log.Info("attr groups",
				slog.Group("from", slog.Int64("id", 555000501)),
				slog.Group("Chat", slog.Int64("id", 555000502), slog.String("type", "private")),
				slog.Group("message", slog.String("body", "my private words")),
				slog.Int64("update", 555000503),
				slog.String("kept", "visible"))
			for _, name := range []string{"from", "CHAT", "message", "update"} {
				log.WithGroup(name).Info("with group "+name, slog.Int64("id", 555000504))
			}
			out := buf.String()
			for _, leaked := range []string{"555000501", "555000502", "555000503", "555000504", "my private words"} {
				if strings.Contains(out, leaked) {
					t.Fatalf("%q reached the log:\n%s", leaked, out)
				}
			}
			if !strings.Contains(out, `"kept":"visible"`) || strings.Count(out, "with group") != 4 {
				t.Fatalf("the lines and the ordinary attributes must stay:\n%s", out)
			}
		})
	}
}

// TestRedactCutsTheBodyTheLibraryQuotes: an answer the library cannot decode
// is quoted whole in its error, ids, names and texts of players included.
func TestRedactCutsTheBodyTheLibraryQuotes(t *testing.T) {
	body := `[{"update_id":700012345,"message":{"from":{"id":987654321,"username":"vasya_secret_nick"},` + "\n" +
		`"chat":{"id":987654321,"type":"private"},"text":"my private words, json: fake"}]`
	err := fmt.Errorf("error get updates, %w",
		fmt.Errorf("error decode response body for method getUpdates, %s, %w", body, errors.New("invalid character ']' after object key")))
	var buf bytes.Buffer
	privacy.ErrorLog(slog.New(slog.NewJSONHandler(&buf, nil)), nil)(err)
	for _, got := range []string{privacy.Redact(err.Error()), privacy.NewRedactor(token).Redact(err.Error()), buf.String()} {
		for _, leaked := range []string{"700012345", externalID, "vasya_secret_nick", "my private words"} {
			if strings.Contains(got, leaked) {
				t.Fatalf("%q left in %q", leaked, got)
			}
		}
		if !strings.Contains(got, "error decode response body for method getUpdates, "+privacy.RedactedBody) {
			t.Fatalf("the failed call must stay readable: %q", got)
		}
	}
	if got := privacy.Redact("error decode response result for method getMe"); got != "error decode response result for method getMe" {
		t.Fatalf("a line without a body is kept as is: %q", got)
	}
}
