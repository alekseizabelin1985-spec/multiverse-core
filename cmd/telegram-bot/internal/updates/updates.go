// Package updates is where Telegram updates enter the bot (ADR-018 p. 2,
// component §10.2). Source is the only point of contact with the library for
// incoming messages; Telegram is its long polling implementation and Fake the
// one tests drive.
//
// Update carries only what the bot is allowed to read (ADR-018 p. 3): the
// update id, the chat id and type, the sender id and the text. The username
// and first name are kept for one comparison in the onboarding flow and are
// never stored or logged there. Every type of this package prints as redacted
// in a log line.
package updates

import (
	"context"
	"errors"
	"log/slog"

	"multiverse-core.io/shared/logging"
)

// ChatPrivate is the type of a one-to-one chat with the bot, the only chat the
// bot serves (SEC-07).
const ChatPrivate = "private"

// ExitConflict is the exit code of a bot that found another instance polling
// the same token (ADR-018 p. 6): compose restarts on failure, and a distinct
// code lets the operator tell a duplicate from a crash.
const ExitConflict = 3

// ErrConflict is returned by a Source when Telegram answers getUpdates with
// 409 Conflict: another instance of the bot is polling the same token, or a
// webhook is set for it.
var ErrConflict = errors.New("telegram: getUpdates conflict, another instance of the bot is polling this token, or a webhook is set")

// ErrUnauthorized is returned when Telegram rejects the token: at start, or
// while polling once the token was revoked. The source stops rather than poll
// on with a token that will not work again.
var ErrUnauthorized = errors.New("telegram: the bot token was rejected")

// ExitCode maps the error a Source stopped with to the exit code of the
// process: 0 for a normal stop, ExitConflict for a duplicate, 1 otherwise, a
// rejected token included.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrConflict):
		return ExitConflict
	default:
		return 1
	}
}

// Update is one incoming update.
type Update struct {
	// ID is update_id; action_key is derived from it.
	ID      int64
	Message *Message
}

// Message is the message of an update.
type Message struct {
	Chat Chat
	// From is nil for a message sent on behalf of a channel.
	From *User
	Text string
}

// Chat is where a message was sent.
type Chat struct {
	ID   int64
	Type string
}

// User is the sender of a message. Its ID is the external id of the player.
type User struct {
	ID        int64
	IsBot     bool
	Username  string
	FirstName string
}

// LogValue keeps the update id, the chat id and the text out of a log line.
func (Update) LogValue() slog.Value { return slog.StringValue(logging.Redacted) }

// LogValue keeps the chat id and the text out of a log line.
func (Message) LogValue() slog.Value { return slog.StringValue(logging.Redacted) }

// LogValue keeps the chat id out of a log line.
func (Chat) LogValue() slog.Value { return slog.StringValue(logging.Redacted) }

// LogValue keeps the user id and the names out of a log line.
func (User) LogValue() slog.Value { return slog.StringValue(logging.Redacted) }

// String keeps the ids and the text out of an error built with %v.
func (Update) String() string { return "update " + logging.Redacted }

// String keeps the ids and the text out of an error built with %v.
func (Message) String() string { return "message " + logging.Redacted }

// String keeps the chat id out of an error built with %v.
func (Chat) String() string { return "chat " + logging.Redacted }

// String keeps the user id and the names out of an error built with %v.
func (User) String() string { return "user " + logging.Redacted }

// Handler processes one update. Updates reach it one at a time, in the order
// Telegram delivered them.
type Handler func(ctx context.Context, u Update)

// Source delivers updates to a handler until ctx ends or the source fails.
// Start returns nil when ctx ended, ErrConflict when another instance polls
// the same token, ErrUnauthorized when the token is rejected, and another error
// when the source could not start. Once ctx ends no further update reaches the
// handler.
type Source interface {
	Start(ctx context.Context, h Handler) error
}
