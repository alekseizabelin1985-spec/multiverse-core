// Package access is the first step of handling every update (component §10.2,
// ADR-018 addendum p. 1–2): it lets through only messages of allow-listed
// users (SEC-06) in private chats (SEC-07), at most N commands per minute per
// user (SEC-11). A refusal is decided here, before the command parser, the
// onboarding flow and any call to the gateway.
//
// A refusal never names the user: the log says why an update was refused and
// the counters say how often, nothing else. A flood is answered and logged
// once per series, so that a stranger cannot hold up the players behind a
// stream of replies.
package access

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/shared/clock"
)

// Replies of the gate, plain text (SEC-10). A group chat gets none (US-008,
// FR-131, NFR-044): the bot does not answer where it cannot serve.
const (
	TextInviteOnly = "Доступ по приглашению."
	tooOftenFormat = "Слишком часто, подождите %d с."
)

// MaxRefusedChats bounds how many chats the gate remembers a refusal of. A chat
// beyond it is refused without a reply and without a log line: a flood from
// many accounts costs at most this many log lines within a Window.
const MaxRefusedChats = 1024

// MaxInviteRepliesPerWindow bounds the replies «Доступ по приглашению.» of the
// whole bot within any Window, whatever the number of strangers. A reply is
// sent in the one handler of all updates and may last up to
// sender.BestEffortHTTPTimeout, so the bound is what a flood from many accounts
// can take from the players: a few seconds a minute when Telegram answers as
// usual. A stranger past the bound is refused in silence; the refusal is still
// counted and its series still logged once, with answered=false.
//
// The replies «Слишком часто» to the players of the allow-list are not bound by
// it: there are as many series of them as players, and a flood of strangers
// must not take the hint away from a player.
const MaxInviteRepliesPerWindow = 10

// TooOftenText is the reply to a command over the limit.
func TooOftenText(wait time.Duration) string {
	return fmt.Sprintf(tooOftenFormat, int(math.Ceil(max(wait, time.Second).Seconds())))
}

// Outcome is the decision on one update.
type Outcome int

const (
	// Admit: the update goes on to the parser and the flow.
	Admit Outcome = iota
	// Ignore: nothing to handle — no message, no sender, or a private chat
	// that is not the sender's own.
	Ignore
	// NotPrivate: a group, supergroup or channel (SEC-07); never answered.
	NotPrivate
	// NotAllowed: the sender is not on the allow-list (SEC-06).
	NotAllowed
	// TooOften: the sender is over the command limit (SEC-11).
	TooOften
)

var outcomeNames = map[Outcome]string{
	Admit: "admit", Ignore: "ignore", NotPrivate: "not_private", NotAllowed: "not_allowed", TooOften: "too_often",
}

func (o Outcome) String() string { return outcomeNames[o] }

// Verdict is the decision with what the reply needs.
type Verdict struct {
	Outcome Outcome
	// Wait is how long until the sender may send a command again (TooOften).
	Wait time.Duration
	// Notify says whether the refusal is answered: NotAllowed and TooOften
	// once per series of refusals, NotAllowed only within
	// MaxInviteRepliesPerWindow, NotPrivate never.
	Notify bool
	// Report says whether the refusal is logged: once per series, so that a
	// flood writes one line rather than one per message.
	Report bool
}

// Counters are the refusals since start; Denied is bot_denied_total.
type Counters struct {
	Denied     uint64
	NotPrivate uint64
	TooOften   uint64
}

// Options configures a Gate.
type Options struct {
	// AllowedUserIDs is the allow-list; empty admits nobody.
	AllowedUserIDs []int64
	// CommandsPerMinute is the limit per user; below 1 is treated as 1.
	CommandsPerMinute int
	Clock             clock.Clock
	// Sender answers a refusal. The gate runs in the one handler of all
	// updates, so the sender must give up rather than wait, and give up soon:
	// sender.NewBestEffortTelegram — one try, a client Timeout of at most
	// sender.BestEffortHTTPTimeout. WithPolicy(BestEffortPolicy) of the sender
	// of the flow keeps its 30 s client and is not enough.
	Sender sender.Sender
	// Log receives one line per series of refusals, without ids; nil discards.
	Log *slog.Logger
}

// Gate is the access check.
type Gate struct {
	allowed    map[int64]struct{}
	limiter    *limiter
	refused    *refusals
	clock      clock.Clock
	sender     sender.Sender
	log        *slog.Logger
	denied     atomic.Uint64
	notPrivate atomic.Uint64
	tooOften   atomic.Uint64
}

// New builds a gate. Clock and Sender are required.
func New(opts Options) (*Gate, error) {
	if opts.Clock == nil || opts.Sender == nil {
		return nil, fmt.Errorf("access: Clock and Sender are required")
	}
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	allowed := make(map[int64]struct{}, len(opts.AllowedUserIDs))
	for _, id := range opts.AllowedUserIDs {
		allowed[id] = struct{}{}
	}
	return &Gate{
		allowed: allowed,
		limiter: newLimiter(max(opts.CommandsPerMinute, 1)),
		refused: newRefusals(MaxRefusedChats, MaxInviteRepliesPerWindow),
		clock:   opts.Clock,
		sender:  opts.Sender,
		log:     log,
	}, nil
}

// Check decides on one update, in the order of component §10.2: a message
// with a sender, a private chat, the allow-list, the limit. Only an admitted
// update counts against the limit.
func (g *Gate) Check(u updates.Update) Verdict {
	now := g.clock.Now()
	// Every update the gate sees lets go of the chats whose series are over,
	// so a refused id stays in memory no longer than its Window and the next
	// update after it.
	g.refused.sweep(now)
	m := u.Message
	if m == nil || m.From == nil {
		return Verdict{Outcome: Ignore}
	}
	if m.Chat.Type != updates.ChatPrivate {
		g.notPrivate.Add(1)
		return Verdict{Outcome: NotPrivate, Report: g.refused.opens(m.Chat.ID, now)}
	}
	// In a private chat the chat is the sender; a message where they differ
	// is not one the bot can attribute (SEC-07: identity is from.id only).
	if m.Chat.ID != m.From.ID {
		return Verdict{Outcome: Ignore}
	}
	if _, ok := g.allowed[m.From.ID]; !ok {
		g.denied.Add(1)
		first := g.refused.opens(m.From.ID, now)
		return Verdict{Outcome: NotAllowed, Notify: first && g.refused.reply(now), Report: first}
	}
	ok, wait, first := g.limiter.take(m.From.ID, now)
	if !ok {
		g.tooOften.Add(1)
		return Verdict{Outcome: TooOften, Wait: wait, Notify: first, Report: first}
	}
	return Verdict{Outcome: Admit}
}

// Wrap puts the gate in front of next: an admitted update reaches next, a
// refused one is answered (or not) and goes no further.
func (g *Gate) Wrap(next updates.Handler) updates.Handler {
	return func(ctx context.Context, u updates.Update) {
		v := g.Check(u)
		switch v.Outcome {
		case Admit:
			next(ctx, u)
			return
		case Ignore:
			return
		}
		if v.Report {
			g.log.Info("update refused", slog.String("reason", v.Outcome.String()), slog.Bool("answered", v.Notify))
		}
		if !v.Notify {
			return
		}
		text := TextInviteOnly
		if v.Outcome == TooOften {
			text = TooOftenText(v.Wait)
		}
		if err := g.sender.Send(ctx, u.Message.From.ID, text, nil); err != nil {
			g.log.Warn("refusal not delivered", slog.String("reason", v.Outcome.String()), slog.Any("error", err))
		}
	}
}

// Counters returns the refusals since start.
func (g *Gate) Counters() Counters {
	return Counters{Denied: g.denied.Load(), NotPrivate: g.notPrivate.Load(), TooOften: g.tooOften.Load()}
}
