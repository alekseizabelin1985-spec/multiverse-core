package sender

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/shared/clock"
)

// Policy is how Send repeats a message Telegram did not take.
type Policy struct {
	// Attempts bounds the tries on a network error or a 5xx, the first one
	// included (component §10.4: three).
	Attempts int
	// Pause separates those tries.
	Pause time.Duration
	// RateLimitWaits bounds how many 429 answers Send waits out.
	RateLimitWaits int
	// MaxRetryAfter is the longest retry_after Send waits for; a longer one
	// means a flood ban, and the message is left for a later delivery.
	MaxRetryAfter time.Duration
}

// DefaultPolicy is the policy of component §10.4.
var DefaultPolicy = Policy{Attempts: 3, Pause: time.Second, RateLimitWaits: 5, MaxRetryAfter: time.Minute}

// BestEffortPolicy tries a message once: no 429 is waited out and no failure
// is repeated. It arms no pause, but the one try still lasts as long as the
// HTTP client lets it — up to the Timeout of that client. The replies of the
// access gate run in the one handler of all updates, so they take a sender of
// NewBestEffortTelegram, whose client gives up after BestEffortHTTPTimeout:
// a pause or a slow answer there would hold the commands of every player
// behind a refusal to a stranger.
var BestEffortPolicy = Policy{Attempts: 1}

// ReplyPolicy is the policy of the answers of the flow, which runs in the one
// handler of all updates (review #1 of T-310, M-2): a network error or a 5xx
// is tried once more after a second, and a 429 is waited out once, only when
// retry_after is at most 3 s. A longer ban is not waited for — the answer is
// lost, and the commands of the other players go on. The delivery loop, which
// runs in a goroutine of its own, keeps DefaultPolicy. The one try still lasts
// up to the Timeout of the HTTP client of the sender.
var ReplyPolicy = Policy{Attempts: 2, Pause: time.Second, RateLimitWaits: 1, MaxRetryAfter: 3 * time.Second}

// DefaultHTTPTimeout bounds one sendMessage.
const DefaultHTTPTimeout = 30 * time.Second

// BestEffortHTTPTimeout bounds one sendMessage of a best-effort sender.
const BestEffortHTTPTimeout = 5 * time.Second

// TelegramOptions configures Telegram.
type TelegramOptions struct {
	Token string
	// Redactor removes the secrets of the process from error texts; nil still
	// removes the shapes of a token.
	Redactor *privacy.Redactor
	// Timers drive the pauses between tries; nil means the wall clock.
	Timers clock.Timers
	// Policy is the repeat policy; the zero value means DefaultPolicy.
	Policy Policy
	// ErrorLog receives the errors the library reports outside a call; nil
	// discards them.
	ErrorLog func(error)
	// ServerURL replaces https://api.telegram.org; tests point it at httptest.
	ServerURL string
	// HTTPClient replaces the client with DefaultHTTPTimeout.
	HTTPClient *http.Client
}

// Telegram sends messages with sendMessage.
type Telegram struct {
	bot      *bot.Bot
	redactor *privacy.Redactor
	timers   clock.Timers
	policy   Policy
}

// NewTelegram builds the sender. It sends nothing: the token is checked by the
// update source at start.
func NewTelegram(opts TelegramOptions) (*Telegram, error) {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	onError := opts.ErrorLog
	if onError == nil {
		onError = func(error) {}
	}
	botOpts := []bot.Option{
		bot.WithSkipGetMe(),
		bot.WithHTTPClient(client.Timeout, statusRecorder{inner: client}),
		bot.WithErrorsHandler(onError),
		bot.WithDefaultHandler(func(context.Context, *bot.Bot, *models.Update) {}),
		bot.WithDebugHandler(func(string, ...any) {}),
	}
	if opts.ServerURL != "" {
		botOpts = append(botOpts, bot.WithServerURL(opts.ServerURL))
	}
	b, err := bot.New(opts.Token, botOpts...)
	if err != nil {
		return nil, fmt.Errorf("telegram sender: %s", opts.Redactor.Redact(err.Error()))
	}
	t := &Telegram{bot: b, redactor: opts.Redactor, timers: opts.Timers, policy: opts.Policy}
	if t.timers == nil {
		t.timers = clock.RealTimers{}
	}
	if t.policy == (Policy{}) {
		t.policy = DefaultPolicy
	}
	return t, nil
}

// NewBestEffortTelegram builds a sender of its own for messages that must not
// hold anybody up, the replies of the access gate: BestEffortPolicy over a
// client whose Timeout is at most BestEffortHTTPTimeout. opts.Policy is
// ignored; opts.HTTPClient, when set, is copied, not changed, and its Timeout
// is cut down to BestEffortHTTPTimeout when it is longer or unset.
func NewBestEffortTelegram(opts TelegramOptions) (*Telegram, error) {
	opts.HTTPClient = bestEffortClient(opts.HTTPClient)
	opts.Policy = BestEffortPolicy
	return NewTelegram(opts)
}

func bestEffortClient(base *http.Client) *http.Client {
	var c http.Client
	if base != nil {
		c = *base
	}
	if c.Timeout <= 0 || c.Timeout > BestEffortHTTPTimeout {
		c.Timeout = BestEffortHTTPTimeout
	}
	return &c
}

// WithPolicy returns a sender that shares the client of t and repeats by p;
// the zero value means DefaultPolicy. The client, and so its Timeout, stays
// that of t: a sender that must answer quickly is NewBestEffortTelegram, not
// WithPolicy(BestEffortPolicy) of a sender with DefaultHTTPTimeout. t itself
// keeps its policy.
func (t *Telegram) WithPolicy(p Policy) *Telegram {
	if p == (Policy{}) {
		p = DefaultPolicy
	}
	out := *t
	out.policy = p
	return &out
}

// Send sends text as plain text: the request carries no parse_mode (SEC-10).
// A 429 is waited out for retry_after, a network error or a 5xx is tried again
// up to Policy.Attempts; the error returned wraps one of the Err* of this
// package, or the error of ctx.
func (t *Telegram) Send(ctx context.Context, chatID int64, text string, kb *Keyboard) error {
	params := &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup(kb)}
	tries, waits := 0, 0
	for {
		var status atomic.Int32
		_, err := t.bot.SendMessage(context.WithValue(ctx, statusKey{}, &status), params)
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		var tooMany *bot.TooManyRequestsError
		switch {
		case errors.As(err, &tooMany):
			waits++
			wait := time.Duration(tooMany.RetryAfter) * time.Second
			if wait <= 0 {
				wait = time.Second
			}
			if waits > t.policy.RateLimitWaits || wait > t.policy.MaxRetryAfter {
				return t.fail(ErrUnavailable, err, chatID)
			}
			if werr := t.pause(ctx, wait); werr != nil {
				return werr
			}
		case errors.Is(err, bot.ErrorUnauthorized):
			return t.fail(ErrUnauthorized, err, chatID)
		case errors.Is(err, bot.ErrorForbidden):
			return t.fail(ErrBlocked, err, chatID)
		case errors.Is(err, bot.ErrorBadRequest) && strings.Contains(strings.ToLower(err.Error()), "chat not found"):
			return t.fail(ErrChatNotFound, err, chatID)
		case status.Load() == 0 || status.Load() >= http.StatusInternalServerError:
			tries++
			if tries >= t.policy.Attempts {
				return t.fail(ErrUnavailable, err, chatID)
			}
			if werr := t.pause(ctx, t.policy.Pause); werr != nil {
				return werr
			}
		default:
			return t.fail(ErrRejected, err, chatID)
		}
	}
}

// fail builds the error of Send from the redacted text of the library error,
// with the chat id removed as well: the text is Telegram's and may quote it.
func (t *Telegram) fail(kind, err error, chatID int64) error {
	detail := t.redactor.Redact(err.Error())
	detail = strings.ReplaceAll(detail, strconv.FormatInt(chatID, 10), "<chat>")
	return &sendError{kind: kind, detail: detail}
}

func (t *Telegram) pause(ctx context.Context, d time.Duration) error {
	timer := t.timers.After(d)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C():
		return nil
	}
}

type sendError struct {
	kind   error
	detail string
}

func (e *sendError) Error() string { return e.kind.Error() + ": " + e.detail }
func (e *sendError) Unwrap() error { return e.kind }

// markup maps a Keyboard onto the reply markup of the Bot API: plain text
// buttons, no inline keyboard (ADR-018, allowed_updates stays [message]).
func markup(kb *Keyboard) models.ReplyMarkup {
	if kb == nil {
		return nil
	}
	if kb.Remove {
		return &models.ReplyKeyboardRemove{RemoveKeyboard: true}
	}
	rows := make([][]models.KeyboardButton, 0, len(kb.Rows))
	for _, row := range kb.Rows {
		buttons := make([]models.KeyboardButton, 0, len(row))
		for _, label := range row {
			buttons = append(buttons, models.KeyboardButton{Text: label})
		}
		rows = append(rows, buttons)
	}
	return &models.ReplyKeyboardMarkup{Keyboard: rows, ResizeKeyboard: true}
}

// statusKey carries, in the context of one call, where the HTTP status of its
// answer is recorded. The library turns every error answer into a text error;
// only a 403, 400, 404, 409 and 429 get a typed one, so a 5xx is told from a
// refusal by the status seen on the wire.
type statusKey struct{}

type statusRecorder struct {
	inner bot.HttpClient
}

func (s statusRecorder) Do(req *http.Request) (*http.Response, error) {
	resp, err := s.inner.Do(req)
	if status, ok := req.Context().Value(statusKey{}).(*atomic.Int32); ok && resp != nil {
		status.Store(int32(resp.StatusCode))
	}
	return resp, err
}
