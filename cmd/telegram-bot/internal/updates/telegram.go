package updates

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
)

// DefaultPollTimeout is the timeout of getUpdates (ADR-006, component §10.2).
const DefaultPollTimeout = 25 * time.Second

// httpMargin is added to the poll timeout for the HTTP client: a long poll
// that Telegram holds for the whole timeout must not be cut by the client.
const httpMargin = 10 * time.Second

// TelegramOptions configures Telegram.
type TelegramOptions struct {
	// Token is the bot token.
	Token string
	// PollTimeout is the long polling timeout; zero means DefaultPollTimeout.
	PollTimeout time.Duration
	// Log receives the errors of the library, redacted. Nil discards them.
	Log *slog.Logger
	// Redactor knows the secrets of the process; nil still removes the shapes
	// of a token.
	Redactor *privacy.Redactor
	// ServerURL replaces https://api.telegram.org; tests point it at
	// httptest.
	ServerURL string
	// HTTPClient replaces the client built from PollTimeout.
	HTTPClient *http.Client
	// OnReady, when set, is called once Telegram took the token at getMe and
	// before the first getUpdates. It is not called when Start fails before
	// polling, a rejected token included.
	OnReady func()
}

// Telegram is the long polling Source over github.com/go-telegram/bot.
type Telegram struct {
	opts TelegramOptions
}

// NewTelegram returns the polling source. Nothing is sent to Telegram until
// Start.
func NewTelegram(opts TelegramOptions) *Telegram {
	if opts.PollTimeout <= 0 {
		opts.PollTimeout = DefaultPollTimeout
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}
	return &Telegram{opts: opts}
}

// Start checks the token with getMe, calls OnReady, then polls getUpdates and
// hands every update to h, one at a time, until ctx ends. A 409 Conflict of getUpdates
// stops polling and returns ErrConflict (ADR-018 p. 6); a 401 stops it and
// returns ErrUnauthorized.
func (t *Telegram) Start(ctx context.Context, h Handler) error {
	if h == nil {
		return errors.New("telegram: Start needs a handler")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var stopped atomic.Pointer[error]
	stop := func(reason error, line string) {
		if stopped.CompareAndSwap(nil, &reason) {
			t.opts.Log.Error(line, slog.Bool("handled", true))
		}
		cancel()
	}
	logError := privacy.ErrorLog(t.opts.Log, t.opts.Redactor)
	onError := func(err error) {
		switch {
		case errors.Is(err, bot.ErrorConflict):
			stop(ErrConflict, "getUpdates answered 409 Conflict: another instance polls this token or a webhook is set, stopping")
		case errors.Is(err, bot.ErrorUnauthorized):
			stop(fmt.Errorf("%w: getUpdates answered 401, polling stopped", ErrUnauthorized),
				"getUpdates answered 401 Unauthorized: the token was revoked, stopping")
		case runCtx.Err() != nil && strings.Contains(err.Error(), libraryLostUpdates):
			// The library reports the rest of a batch it did not hand over as
			// lost. With the unbuffered hand-over of options no getUpdates
			// followed it, so Telegram has not seen those updates confirmed and
			// delivers them again.
			t.opts.Log.Info("polling stopped before the rest of a batch was handed over; Telegram delivers it again")
		default:
			logError(err)
		}
	}

	b, err := bot.New(t.opts.Token, t.options(h, onError)...)
	if err != nil {
		return t.startError(err)
	}
	if t.opts.OnReady != nil {
		t.opts.OnReady()
	}
	b.Start(runCtx)
	if reason := stopped.Load(); reason != nil {
		return *reason
	}
	return nil
}

// libraryLostUpdates is the text of the error the library reports when its
// context ends while it still holds updates of a batch.
const libraryLostUpdates = "some updates lost, ctx done"

// options is the whole configuration of the library (ADR-018 p. 1). WithDebug
// is not among them and must never be: in debug mode the library logs request
// and response payloads, ids and texts of players included.
func (t *Telegram) options(h Handler, onError func(error)) []bot.Option {
	client := t.opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: t.opts.PollTimeout + httpMargin}
	}
	opts := []bot.Option{
		bot.WithAllowedUpdates(bot.AllowedUpdates{"message"}),
		bot.WithNotAsyncHandlers(),
		// The library confirms a batch to Telegram with the next getUpdates,
		// sent as soon as the last update of the batch is queued. With its
		// default queue of 1024 every queued update is confirmed before it is
		// handled and lost on any stop. Unbuffered, an update is handed over
		// only to the handler that takes it: the next getUpdates leaves once
		// the last update of the batch is in hand, and a stop loses at most
		// that one (component §10.2).
		bot.WithUpdatesChannelCap(0),
		bot.WithDefaultHandler(adapt(h)),
		bot.WithErrorsHandler(onError),
		// The library asks getUpdates for its poll timeout minus one second.
		bot.WithHTTPClient(t.opts.PollTimeout+time.Second, client),
		// Belt and braces for the rule above: were debug switched on, its
		// output would go nowhere.
		bot.WithDebugHandler(func(string, ...any) {}),
	}
	if t.opts.ServerURL != "" {
		opts = append(opts, bot.WithServerURL(t.opts.ServerURL))
	}
	return opts
}

// startError rebuilds the error of bot.New from its redacted text: wrapping
// the original would carry its text, and with it a URL, into every %v.
func (t *Telegram) startError(err error) error {
	text := t.opts.Redactor.Redact(err.Error())
	if errors.Is(err, bot.ErrorUnauthorized) {
		return fmt.Errorf("%w: %s", ErrUnauthorized, text)
	}
	return fmt.Errorf("telegram: start: %s", text)
}

// adapt turns an update of the library into an Update, reading only the
// fields ADR-018 p. 3 names. An update the library hands over after ctx ended
// is not handled: the worker of the library picks between the end of ctx and
// the next update at random, and a handler started with an ended ctx would
// only fail halfway. Telegram delivers such an update again, since no
// getUpdates confirmed it — unless it was the last of its batch and the next
// getUpdates had already left: that is the one update a stop may lose.
func adapt(h Handler) bot.HandlerFunc {
	return func(ctx context.Context, _ *bot.Bot, u *models.Update) {
		if u == nil || ctx.Err() != nil {
			return
		}
		h(ctx, fromModel(u))
	}
}

func fromModel(u *models.Update) Update {
	out := Update{ID: u.ID}
	m := u.Message
	if m == nil {
		return out
	}
	msg := &Message{Chat: Chat{ID: m.Chat.ID, Type: string(m.Chat.Type)}, Text: m.Text}
	if m.From != nil {
		msg.From = &User{ID: m.From.ID, IsBot: m.From.IsBot, Username: m.From.Username, FirstName: m.From.FirstName}
	}
	out.Message = msg
	return out
}
