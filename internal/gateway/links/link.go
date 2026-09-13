// Package links keeps the only copy of the external messenger IDs of the
// players: the link between an external account and its player_id, the
// consent given through it and the idempotency of character creation
// (component gateway-and-bot.md §4.1, §6; ADR-009 p. 2, ADR-019).
//
// Nothing outside links.db learns an external ID or a link_id from this
// package: a Link prints as "redacted" in a log, in JSON and in fmt, and the HTTP layer
// returns neither link_id nor anything derived from it (SEC-01, SEC-03).
package links

import (
	"errors"
	"log/slog"
	"time"
)

// Statuses of a link (links.status).
const (
	StatusPendingConsent = "pending_consent"
	StatusConsented      = "consented"
)

// PlatformTelegram is the external platform of the Telegram bot. links.db also
// admits ci and sim for fixtures, the HTTP API of MVP-1 only telegram.
const PlatformTelegram = "telegram"

// Errors of the store. A caller matches them with errors.Is.
var (
	// ErrNotFound: no link for the key given.
	ErrNotFound = errors.New("links: link not found")
	// ErrConsentIncomplete: the notice, the consent or the age was not
	// confirmed; an existing link stays pending_consent, and no link is
	// created for an account without one.
	ErrConsentIncomplete = errors.New("links: consent incomplete")
	// ErrPlayerIDTaken: the player_id is already bound to another link.
	ErrPlayerIDTaken = errors.New("links: player_id is bound to another link")
	// ErrCompactionPending: /forget deleted the link, but links.db could not be
	// compacted, so the bytes of the external ID may still be in the WAL or in
	// free pages. The data must not be reported as gone; the next Forget,
	// Compact or Sweep finishes the wipe (SEC-04, ADR-019 addendum p. 1).
	ErrCompactionPending = errors.New("links: link deleted, compaction of links.db pending")
)

// Link is one row of links. Its fields are personal data: LinkID and
// ExternalID never leave the gateway, and the type redacts itself wherever it
// could be printed.
type Link struct {
	LinkID         string
	Platform       string
	ExternalID     string
	PlayerID       *string
	WorldID        *string
	Status         string
	NoticeShownAt  *time.Time
	ConsentAt      *time.Time
	AgeConfirmedAt *time.Time
	LastSeenAt     time.Time
	CreatedAt      time.Time
}

const redacted = "redacted"

// LogValue keeps the link out of slog whatever the handler: slog.Any("link",
// l) writes "redacted" (SEC-01, SEC-02).
func (Link) LogValue() slog.Value { return slog.StringValue(redacted) }

// String keeps the link out of %v and %s, including inside a wrapped error.
func (Link) String() string { return redacted }

// GoString keeps the link out of %#v.
func (Link) GoString() string { return redacted }

// MarshalJSON keeps the link out of JSON. LogValue is consulted only for the
// value of a top-level attribute: a Link inside a struct or a slice, such as
// slog.Any("res", Resolution{...}), reaches the JSON handler of the platform
// through encoding/json. No wire type embeds a Link, so nothing depends on
// its JSON form.
func (Link) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// Resolution is the result of Resolve.
type Resolution struct {
	Link Link
	// Created is true when this call inserted the link.
	Created bool
	// PreviousSeenAt is last_seen_at before this call; for a created link it
	// equals the creation time.
	PreviousSeenAt time.Time
}

// ConsentForm is what the player confirmed in the notice (UC-001).
type ConsentForm struct {
	NoticeShown  bool
	Consent      bool
	AgeConfirmed bool
	// ShownAt is when the client showed the notice; it becomes notice_shown_at.
	ShownAt time.Time
}

// Complete reports whether all three confirmations are given.
func (f ConsentForm) Complete() bool { return f.NoticeShown && f.Consent && f.AgeConfirmed }

// StoredResponse is the first answer to POST /v1/characters under one
// (link_id, action_key), returned again on a repeat.
type StoredResponse struct {
	PlayerID   string
	StatusCode int
	Body       []byte
}
