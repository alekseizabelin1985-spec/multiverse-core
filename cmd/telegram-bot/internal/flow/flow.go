// Package flow is the dialog of the bot with a player (component §10.1,
// §10.3): the onboarding from /start through the notice of FR-009, the consent,
// the name and the world to a character; the game commands of a player with a
// character; and /forget with its confirmation.
//
// A player is identified by from.id only; the chat of a private message is the
// same id (SEC-07). The flow keeps, per chat and in memory only, the step of
// the onboarding for DialogTTL and the player_id for PlayerTTL. The username
// and the first name of an update are read for one comparison with the name
// the player types and are never stored or logged (US-009).
//
// Consent is given only by pressing ConsentButton while the flow waits for it,
// right after the notice was sent: every other way to the consent — a button
// pressed after the TTL, /help, a game command — shows the notice again and
// does not call links/consent (FR-009, BR-09, NFR-045; decisions Р-3 A, Mi-5).
package flow

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/commands"
	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
)

// Platform is the external platform of every link the bot resolves.
const Platform = "telegram"

// Statuses of links/resolve (api-contracts.md §1.2).
const (
	linkConsented     = "consented"
	characterAlive    = "alive"
	characterCreating = "creating"
	characterDead     = "dead"
)

// CreatingPolls and CreatingPollInterval bound the wait for a character
// answered 202 creating: GET /v1/players/{id} once a second, at most ten
// times (component §7.1).
const (
	CreatingPolls        = 10
	CreatingPollInterval = time.Second
)

// Gateway is what the flow calls of the gateway; *client.Client implements it.
type Gateway interface {
	Resolve(ctx context.Context, platform, externalID string) (api.ResolveResponse, error)
	Consent(ctx context.Context, req api.ConsentRequest) (api.ConsentResponse, error)
	Forget(ctx context.Context, platform, externalID string) (client.ForgetResult, error)
	Worlds(ctx context.Context) (api.WorldsResponse, error)
	CreateCharacter(ctx context.Context, req api.CreateCharacterRequest) (api.CreateCharacterResponse, int, error)
	Player(ctx context.Context, playerID string) (api.CharacterState, error)
	Action(ctx context.Context, playerID string, req api.ActionRequest) (client.ActionResult, error)
}

var _ Gateway = (*client.Client)(nil)

// Options configures a Flow.
type Options struct {
	Gateway Gateway
	// Sender sends the answers. It runs in the one handler of all updates, so
	// it repeats by sender.ReplyPolicy, not by DefaultPolicy (review #1 of
	// T-310, M-2).
	Sender sender.Sender
	Clock  clock.Clock
	// Timers drive the wait for a character being created; nil means the
	// wall clock.
	Timers clock.Timers
	// ActionKeySalt is the key of action_key (config.Config.ActionKeySalt).
	ActionKeySalt []byte
	// Log receives the lines of the flow, with the log keys below only; nil
	// discards them. It must be built on privacy.NewHandler, as every logger
	// of the bot: the flow redacts the error texts it writes itself, but the
	// handler is what drops an identity under any other key (SEC-01/02).
	Log *slog.Logger
}

// Flow handles the admitted updates of all chats, one at a time.
type Flow struct {
	gw     Gateway
	send   sender.Sender
	clock  clock.Clock
	timers clock.Timers
	salt   []byte
	log    *slog.Logger

	mu         sync.Mutex
	dialogs    map[int64]dialog
	players    map[int64]player
	forgetting map[int64]time.Time
}

// New builds a flow. Gateway, Sender, Clock and a non-empty ActionKeySalt are
// required.
func New(opts Options) (*Flow, error) {
	if opts.Gateway == nil || opts.Sender == nil || opts.Clock == nil || len(opts.ActionKeySalt) == 0 {
		return nil, errors.New("flow: Gateway, Sender, Clock and ActionKeySalt are required")
	}
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	timers := opts.Timers
	if timers == nil {
		timers = clock.RealTimers{}
	}
	return &Flow{
		gw:         opts.Gateway,
		send:       opts.Sender,
		clock:      opts.Clock,
		timers:     timers,
		salt:       append([]byte(nil), opts.ActionKeySalt...),
		log:        log,
		dialogs:    make(map[int64]dialog),
		players:    make(map[int64]player),
		forgetting: make(map[int64]time.Time),
	}, nil
}

// Log keys of the flow (review #1 of T-310, N-6): a line names the step, the
// command type of the dictionary, the code and status of a gateway answer and
// a redacted error text — never an id of Telegram, a name or a text.
const (
	keyStep    = "step"
	keyCommand = "command"
	keyCode    = "code"
	keyStatus  = "status"
	keyError   = "error"
)

// turn is the handling of one update.
type turn struct {
	f    *Flow
	ctx  context.Context
	u    updates.Update
	chat int64
	ext  string
	cmd  commands.Command
	now  time.Time
}

// Handle handles one update the access gate admitted. It calls nothing and
// answers nothing once ctx is done: the bot is stopping, and a command sent
// now would be lost with its answer (review #1 of T-310, M-1).
func (f *Flow) Handle(ctx context.Context, u updates.Update) {
	if ctx.Err() != nil || u.Message == nil || u.Message.From == nil {
		return
	}
	now := f.clock.Now()
	f.mu.Lock()
	f.sweepLocked(now)
	f.mu.Unlock()

	chat := u.Message.From.ID
	t := &turn{f: f, ctx: ctx, u: u, chat: chat, ext: strconv.FormatInt(chat, 10), cmd: commands.Parse(u.Message.Text), now: now}
	switch t.cmd.Type {
	case commands.Forget:
		t.forget()
		return
	case commands.Start:
		t.start()
		return
	case commands.Help:
		t.help()
		return
	}

	d, ok := f.dialogOf(chat, now)
	if !ok {
		t.idle()
		return
	}
	switch d.step {
	case AwaitingConsent:
		t.awaitingConsent(&d)
	case AwaitingName:
		t.awaitingName(&d)
	case AwaitingNameConfirm:
		t.awaitingNameConfirm(&d)
	case AwaitingWorld:
		t.awaitingWorld(&d)
	default:
		f.dropDialog(chat)
		t.idle()
	}
}

// reply sends one message to the chat. A failure is logged and otherwise
// dropped: the player repeats the command.
func (t *turn) reply(text string, kb *sender.Keyboard) bool {
	if t.ctx.Err() != nil {
		return false
	}
	if err := t.f.send.Send(t.ctx, t.chat, text, kb); err != nil {
		if t.ctx.Err() == nil {
			t.f.log.Warn("answer not delivered", slog.String(keyCommand, string(t.cmd.Type)), slog.String(keyError, privacy.Redact(err.Error())))
		}
		return false
	}
	return true
}

func (t *turn) replyWith(r render.Reply) bool { return t.reply(r.Text, r.Keyboard) }

// showNotice sends the notice with the consent keyboard and, once it is sent,
// waits for the consent: only then does ConsentButton give it.
func (t *turn) showNotice() {
	if !t.reply(render.NoticeText, render.ConsentKeyboard()) {
		return
	}
	t.f.setDialog(t.chat, &dialog{step: AwaitingConsent, noticeShownAt: t.now}, t.now)
}

// fail answers an error of the gateway. Nothing is answered when the error is
// the end of ctx.
func (t *turn) fail(op string, err error) {
	if t.ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.f.log.Error("gateway call failed", slog.String(keyCommand, op), slog.String(keyError, privacy.Redact(err.Error())))
		t.reply(render.Unavailable, nil)
		return
	}
	t.f.log.Warn("gateway refused", slog.String(keyCommand, op), slog.String(keyCode, apiErr.Code), slog.Int(keyStatus, apiErr.Status))
	text := render.ErrorText(apiErr.Code, apiErr.Message, apiErr.RetryAfter)
	switch apiErr.Code {
	case api.CodeConsentRequired:
		t.f.dropPlayer(t.chat)
		if t.reply(text, nil) {
			t.showNotice()
		}
	case api.CodeCharacterDead, api.CodePlayerNotFound:
		t.f.dropPlayer(t.chat)
		t.reply(text, render.StartKeyboard())
	default:
		t.reply(text, nil)
	}
}

func (t *turn) actionKey() string { return commands.ActionKey(t.f.salt, t.u.ID) }

func ptr(s string) *string { return &s }

func (t *turn) logStep(msg string, step Step) {
	t.f.log.Debug(msg, slog.String(keyStep, step.String()), slog.String(keyCommand, string(t.cmd.Type)))
}
