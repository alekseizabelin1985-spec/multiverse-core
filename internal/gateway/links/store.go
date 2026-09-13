package links

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"multiverse-core.io/internal/gateway/store"
)

// Store is the links of component §6. The gateway has one implementation,
// SQLite; the interface is what the handlers and the later tasks (characters,
// outbox routes) depend on.
type Store interface {
	// Resolve returns the link of an external account, inserting a
	// pending_consent link with a new link_id when there is none, and moves
	// last_seen_at to now.
	Resolve(ctx context.Context, platform, externalID string, now time.Time) (Resolution, error)
	// Consent records a complete consent form, creating the link when the
	// account has none. An incomplete form returns ErrConsentIncomplete: it
	// leaves an existing link pending_consent and touches last_seen_at, and it
	// creates nothing for an account without a link. A repeated consent keeps
	// the first timestamps: they are the evidence of the consent.
	Consent(ctx context.Context, platform, externalID string, form ConsentForm, now time.Time) (Link, error)
	// AttachPlayer binds playerID and worldID to the link, replacing a
	// previous (dead) character; link_id does not change.
	AttachPlayer(ctx context.Context, linkID, playerID, worldID string) error
	ByExternal(ctx context.Context, platform, externalID string) (Link, bool, error)
	ByPlayer(ctx context.Context, playerID string) (Link, bool, error)
	// RouteFor is the external address of a player, taken at the moment of a
	// delivery and never stored elsewhere (component §8.2).
	RouteFor(ctx context.Context, playerID string) (platform, externalID string, ok bool, err error)
	// Forget runs the /forget cascade for an external account (forget.go).
	Forget(ctx context.Context, platform, externalID string) (ForgetResult, error)
	// ForgetByPlayer is Forget addressed by player_id (operator route).
	ForgetByPlayer(ctx context.Context, playerID string) (ForgetResult, error)
	// CharacterRequest returns the stored answer of (linkID, actionKey) that
	// has not expired at now.
	CharacterRequest(ctx context.Context, linkID, actionKey string, now time.Time) (StoredResponse, bool, error)
	// SaveCharacterRequest stores the answer for store.KeyTTL. A live answer
	// under the same key is kept: the first answer is the one repeated.
	SaveCharacterRequest(ctx context.Context, linkID, actionKey string, r StoredResponse, now time.Time) error
	// Compact wipes what deletions left in links.db (store.CompactLinks).
	Compact(ctx context.Context) error
	// Sweep deletes expired character requests and finishes a pending
	// compaction (store.Sweeper).
	Sweep(ctx context.Context, now time.Time) error
}

// SQLite is the Store over links.db. It expects the database opened by
// store.OpenLinks, with its single connection: a statement issued on db inside
// a transaction of the same store would wait for itself, so every method runs
// either on db or inside one transaction, never both.
type SQLite struct {
	db    *sql.DB
	ids   IDSource
	hooks []ForgetHooks
	// pending is set when a compaction failed after a deletion and cleared by
	// the next successful one.
	pending atomic.Bool
}

// NewSQLite returns the store over db with ids as the source of link_id, and
// the hooks Forget runs, in order, before it deletes a link with a character.
func NewSQLite(db *sql.DB, ids IDSource, hooks ...ForgetHooks) (*SQLite, error) {
	if db == nil {
		return nil, errors.New("links: nil database")
	}
	if ids == nil {
		return nil, errors.New("links: nil id source")
	}
	for i, h := range hooks {
		if h == nil {
			return nil, fmt.Errorf("links: forget hook %d is nil", i)
		}
	}
	return &SQLite{db: db, ids: ids, hooks: hooks}, nil
}

var _ Store = (*SQLite)(nil)

// timeLayout is fixed width, so that expires_at compares as text in SQL the
// way it compares as time; RFC3339Nano drops trailing zeros and would not.
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

const linkColumns = `link_id, external_platform, external_id, player_id, world_id, status,
	notice_shown_at, consent_at, age_confirmed_at, last_seen_at, created_at`

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *SQLite) Resolve(ctx context.Context, platform, externalID string, now time.Time) (Resolution, error) {
	var res Resolution
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		link, found, err := selectLink(ctx, tx, "external_platform = ? AND external_id = ?", platform, externalID)
		if err != nil {
			return err
		}
		if !found {
			link, err = s.insert(ctx, tx, platform, externalID, now)
			res = Resolution{Link: link, Created: true, PreviousSeenAt: link.CreatedAt}
			return err
		}
		res.PreviousSeenAt = link.LastSeenAt
		if err := touch(ctx, tx, &link, now); err != nil {
			return err
		}
		res.Link = link
		return nil
	})
	if err != nil {
		return Resolution{}, fmt.Errorf("links: resolve: %w", err)
	}
	return res, nil
}

func (s *SQLite) Consent(ctx context.Context, platform, externalID string, form ConsentForm, now time.Time) (Link, error) {
	var out Link
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		link, found, err := selectLink(ctx, tx, "external_platform = ? AND external_id = ?", platform, externalID)
		if err != nil {
			return err
		}
		if !found {
			// A refused consent of an account the gateway does not know leaves
			// nothing behind: no row holds its external ID (api-contracts.md
			// §1.2, SEC-03).
			if !form.Complete() {
				return nil
			}
			if link, err = s.insert(ctx, tx, platform, externalID, now); err != nil {
				return err
			}
		}
		if !form.Complete() || link.Status == StatusConsented {
			out = link
			return touch(ctx, tx, &out, now)
		}
		if form.ShownAt.IsZero() {
			return errors.New("consent without the time the notice was shown")
		}
		shown, at := form.ShownAt.UTC(), now.UTC()
		if _, err := tx.ExecContext(ctx, `UPDATE links SET status = ?, notice_shown_at = ?, consent_at = ?,
			age_confirmed_at = ?, last_seen_at = ? WHERE link_id = ?`,
			StatusConsented, formatTime(shown), formatTime(at), formatTime(at), formatTime(at), link.LinkID); err != nil {
			return err
		}
		link.Status = StatusConsented
		link.NoticeShownAt, link.ConsentAt, link.AgeConfirmedAt = &shown, &at, &at
		link.LastSeenAt = at
		out = link
		return nil
	})
	if err != nil {
		return Link{}, fmt.Errorf("links: consent: %w", err)
	}
	if !form.Complete() {
		return out, ErrConsentIncomplete
	}
	return out, nil
}

func (s *SQLite) AttachPlayer(ctx context.Context, linkID, playerID, worldID string) error {
	if playerID == "" || worldID == "" {
		return errors.New("links: attach player: empty player_id or world_id")
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, found, err := selectLink(ctx, tx, "link_id = ?", linkID); err != nil {
			return fmt.Errorf("links: attach player: %w", err)
		} else if !found {
			return ErrNotFound
		}
		other, taken, err := selectLink(ctx, tx, "player_id = ?", playerID)
		if err != nil {
			return fmt.Errorf("links: attach player: %w", err)
		}
		if taken && other.LinkID != linkID {
			return ErrPlayerIDTaken
		}
		if _, err := tx.ExecContext(ctx, "UPDATE links SET player_id = ?, world_id = ? WHERE link_id = ?",
			playerID, worldID, linkID); err != nil {
			return fmt.Errorf("links: attach player: %w", err)
		}
		return nil
	})
}

func (s *SQLite) ByExternal(ctx context.Context, platform, externalID string) (Link, bool, error) {
	link, found, err := selectLink(ctx, s.db, "external_platform = ? AND external_id = ?", platform, externalID)
	if err != nil {
		return Link{}, false, fmt.Errorf("links: by external account: %w", err)
	}
	return link, found, nil
}

func (s *SQLite) ByPlayer(ctx context.Context, playerID string) (Link, bool, error) {
	link, found, err := selectLink(ctx, s.db, "player_id = ?", playerID)
	if err != nil {
		return Link{}, false, fmt.Errorf("links: by player: %w", err)
	}
	return link, found, nil
}

func (s *SQLite) RouteFor(ctx context.Context, playerID string) (string, string, bool, error) {
	link, found, err := s.ByPlayer(ctx, playerID)
	if err != nil || !found {
		return "", "", false, err
	}
	return link.Platform, link.ExternalID, true, nil
}

func (s *SQLite) CharacterRequest(ctx context.Context, linkID, actionKey string, now time.Time) (StoredResponse, bool, error) {
	var r StoredResponse
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT player_id, status_code, response_json FROM character_requests
		WHERE link_id = ? AND action_key = ? AND expires_at > ?`, linkID, actionKey, formatTime(now)).
		Scan(&r.PlayerID, &r.StatusCode, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredResponse{}, false, nil
	}
	if err != nil {
		return StoredResponse{}, false, fmt.Errorf("links: character request: %w", err)
	}
	r.Body = []byte(body)
	return r, true, nil
}

func (s *SQLite) SaveCharacterRequest(ctx context.Context, linkID, actionKey string, r StoredResponse, now time.Time) error {
	at := formatTime(now)
	_, err := s.db.ExecContext(ctx, `INSERT INTO character_requests
		(link_id, action_key, player_id, status_code, response_json, expires_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (link_id, action_key) DO UPDATE SET player_id = excluded.player_id,
			status_code = excluded.status_code, response_json = excluded.response_json,
			expires_at = excluded.expires_at
		WHERE character_requests.expires_at <= ?`,
		linkID, actionKey, r.PlayerID, r.StatusCode, string(r.Body), formatTime(now.Add(store.KeyTTL)), at)
	if err != nil {
		return fmt.Errorf("links: save character request: %w", err)
	}
	return nil
}

// Compact runs store.CompactLinks. A failure marks the compaction pending, a
// success clears the mark.
func (s *SQLite) Compact(ctx context.Context) error {
	if err := store.CompactLinks(ctx, s.db); err != nil {
		s.pending.Store(true)
		return err
	}
	s.pending.Store(false)
	return nil
}

// CompactionPending reports whether a deletion is not yet wiped from the file.
func (s *SQLite) CompactionPending() bool { return s.pending.Load() }

func (s *SQLite) Sweep(ctx context.Context, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM character_requests WHERE expires_at <= ?", formatTime(now)); err != nil {
		return fmt.Errorf("links: sweep character requests: %w", err)
	}
	if s.pending.Load() {
		return s.Compact(ctx)
	}
	return nil
}

func (s *SQLite) insert(ctx context.Context, tx *sql.Tx, platform, externalID string, now time.Time) (Link, error) {
	linkID, err := NewLinkID(s.ids)
	if err != nil {
		return Link{}, err
	}
	at := now.UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO links (link_id, external_platform, external_id, status,
		last_seen_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		linkID, platform, externalID, StatusPendingConsent, formatTime(at), formatTime(at)); err != nil {
		return Link{}, err
	}
	return Link{LinkID: linkID, Platform: platform, ExternalID: externalID, Status: StatusPendingConsent,
		LastSeenAt: at, CreatedAt: at}, nil
}

func touch(ctx context.Context, q queryer, link *Link, now time.Time) error {
	at := now.UTC()
	if _, err := q.ExecContext(ctx, "UPDATE links SET last_seen_at = ? WHERE link_id = ?", formatTime(at), link.LinkID); err != nil {
		return err
	}
	link.LastSeenAt = at
	return nil
}

// selectLink reads the one link matching where; the condition is a constant
// of this file, the values are bound.
func selectLink(ctx context.Context, q queryer, where string, args ...any) (Link, bool, error) {
	var (
		l                   Link
		playerID, worldID   sql.NullString
		shown, consent, age sql.NullString
		lastSeen, created   string
	)
	err := q.QueryRowContext(ctx, "SELECT "+linkColumns+" FROM links WHERE "+where, args...).
		Scan(&l.LinkID, &l.Platform, &l.ExternalID, &playerID, &worldID, &l.Status,
			&shown, &consent, &age, &lastSeen, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, false, nil
	}
	if err != nil {
		return Link{}, false, err
	}
	l.PlayerID, l.WorldID = nullString(playerID), nullString(worldID)
	for _, f := range []struct {
		src sql.NullString
		dst **time.Time
	}{{shown, &l.NoticeShownAt}, {consent, &l.ConsentAt}, {age, &l.AgeConfirmedAt}} {
		if !f.src.Valid {
			continue
		}
		t, err := parseTime(f.src.String)
		if err != nil {
			return Link{}, false, err
		}
		*f.dst = &t
	}
	if l.LastSeenAt, err = parseTime(lastSeen); err != nil {
		return Link{}, false, err
	}
	if l.CreatedAt, err = parseTime(created); err != nil {
		return Link{}, false, err
	}
	return l, true, nil
}

func nullString(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func (s *SQLite) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}
