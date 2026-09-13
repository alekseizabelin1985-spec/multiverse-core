package actions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Stored is the answer kept under an action_key: the status and body the first
// request got, 4xx included (C-08).
type Stored struct {
	// CorrelationID is the id of the published action; empty for a refusal,
	// which published nothing.
	CorrelationID string
	Status        int
	Body          []byte
}

// Keys keeps the answers to actions by (player_id, action_key) in the table
// idempotency_keys of gateway.db (component §4.2). The key of a player never
// holds an external ID: player_id is enough.
type Keys struct {
	db  *sql.DB
	ttl time.Duration
}

// NewKeys returns the keys over gateway.db; an answer is kept for ttl.
func NewKeys(db *sql.DB, ttl time.Duration) *Keys { return &Keys{db: db, ttl: ttl} }

// timeLayout keeps a fixed width, so that the TEXT of two times compares the
// way the times do (as in internal/gateway/links).
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// Lookup returns the answer kept under the key; an answer past its expiry is
// not there any more, even before the sweeper deletes its row.
func (k *Keys) Lookup(ctx context.Context, playerID, actionKey string, now time.Time) (Stored, bool, error) {
	var s Stored
	var body string
	err := k.db.QueryRowContext(ctx, `SELECT correlation_id, status_code, response_json FROM idempotency_keys
		WHERE player_id = ? AND action_key = ? AND expires_at > ?`, playerID, actionKey, formatTime(now)).
		Scan(&s.CorrelationID, &s.Status, &body)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Stored{}, false, nil
	case err != nil:
		return Stored{}, false, fmt.Errorf("actions: look up action key: %w", err)
	}
	s.Body = []byte(body)
	return s, true, nil
}

// Save keeps the answer under the key, replacing an expired one.
func (k *Keys) Save(ctx context.Context, playerID, actionKey string, s Stored, now time.Time) error {
	if _, err := k.db.ExecContext(ctx, `INSERT INTO idempotency_keys
		(player_id, action_key, correlation_id, status_code, response_json, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (player_id, action_key) DO UPDATE SET correlation_id = excluded.correlation_id,
			status_code = excluded.status_code, response_json = excluded.response_json,
			created_at = excluded.created_at, expires_at = excluded.expires_at`,
		playerID, actionKey, s.CorrelationID, s.Status, string(s.Body), formatTime(now), formatTime(now.Add(k.ttl))); err != nil {
		return fmt.Errorf("actions: save action key: %w", err)
	}
	return nil
}

// Sweep deletes the answers past their expiry (component §4.2 retention).
func (k *Keys) Sweep(ctx context.Context, now time.Time) error {
	if _, err := k.db.ExecContext(ctx, "DELETE FROM idempotency_keys WHERE expires_at <= ?", formatTime(now)); err != nil {
		return fmt.Errorf("actions: sweep action keys: %w", err)
	}
	return nil
}
