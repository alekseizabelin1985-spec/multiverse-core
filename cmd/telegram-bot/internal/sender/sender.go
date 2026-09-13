// Package sender is where messages leave the bot (ADR-018 p. 2, component
// §10.4). Sender is the only point of contact with the library for outgoing
// messages; Telegram is its implementation over sendMessage and Fake the one
// tests read.
//
// Every message is plain text. Send has no parameter for a parse mode, and the
// implementation never sets one (SEC-10, ADR-006 addendum p. 4): what a player
// said and what the narrator wrote reach the chat character for character, so
// [x](http://…), <a href=…> and *text* stay text.
package sender

import (
	"context"
	"errors"
	"log/slog"

	"multiverse-core.io/shared/logging"
)

// Errors of Send. They say what happened to the message, never to whom it was
// sent: the delivery loop decides on them whether to acknowledge a delivery
// (component §10.4).
var (
	// ErrBlocked: Telegram answered 403 — the player blocked the bot or the
	// account is gone. Repeating will not help.
	ErrBlocked = errors.New("telegram: the chat refuses messages from the bot (403)")
	// ErrChatNotFound: Telegram answered 400 chat not found.
	ErrChatNotFound = errors.New("telegram: chat not found (400)")
	// ErrRejected: Telegram refused the message itself (another 4xx, such as
	// an empty text). Repeating the same message will not help.
	ErrRejected = errors.New("telegram: message rejected")
	// ErrUnauthorized: Telegram answered 401 — the token was revoked while the
	// bot runs. The message was not refused and must not count as delivered;
	// no message leaves the bot until it restarts with a valid token.
	ErrUnauthorized = errors.New("telegram: the bot token was rejected (401)")
	// ErrUnavailable: the network, a 5xx or 429 outlasted the retries. The
	// message may be sent again later.
	ErrUnavailable = errors.New("telegram: unavailable, retries exhausted")
)

// Keyboard is a reply keyboard: rows of buttons, each button a plain text the
// player sends back by pressing it (ADR-018, consent is a button of this kind).
// Remove hides the keyboard shown before instead.
type Keyboard struct {
	Rows   [][]string
	Remove bool
}

// Sender sends one plain-text message to a chat, with an optional keyboard.
type Sender interface {
	Send(ctx context.Context, chatID int64, text string, kb *Keyboard) error
}

// Message is one message as a Fake recorded it.
type Message struct {
	ChatID   int64
	Text     string
	Keyboard *Keyboard
}

// LogValue keeps the chat id and the text out of a log line.
func (Message) LogValue() slog.Value { return slog.StringValue(logging.Redacted) }

// String keeps the chat id and the text out of an error built with %v.
func (Message) String() string { return "message " + logging.Redacted }

func (kb *Keyboard) clone() *Keyboard {
	if kb == nil {
		return nil
	}
	out := &Keyboard{Remove: kb.Remove, Rows: make([][]string, len(kb.Rows))}
	for i, row := range kb.Rows {
		out.Rows[i] = append([]string(nil), row...)
	}
	return out
}
