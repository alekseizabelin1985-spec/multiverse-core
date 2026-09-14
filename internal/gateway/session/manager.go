// Package session keeps the play sessions of the scopes in gateway.db and
// publishes their boundaries as analytics (component gateway-and-bot.md §7.6,
// C-10, BR-17).
//
// A session of a scope opens with the first action of the scope that finds no
// active session, or finds one idle for MV_GATEWAY_SESSION_IDLE: the idle one
// ends with end_reason=idle and a new one opens. The sweeper ends idle sessions
// without waiting for the next action, so that every analytics.session.started
// has its analytics.session.ended (NFR-036).
//
// The table sessions is the truth; the manager keeps nothing in memory, so a
// restarted process continues the sessions it finds there.
package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"multiverse-core.io/shared/eventbus"
)

// DefaultIdle is MV_GATEWAY_SESSION_IDLE by default (BR-17).
const DefaultIdle = 30 * time.Minute

// Kinds and states of a session, and the reasons it ends (C-10 v1.1).
const (
	KindSolo  = "solo"
	KindGroup = "group"

	StateActive = "active"
	StateEnded  = "ended"

	EndLeave  = "leave"
	EndIdle   = "idle"
	EndDeath  = "death"
	EndError  = "error"
	EndForget = "forget"
)

// ErrPublish wraps a failure to publish the analytics of a session. The
// change of the table is undone, so the caller may repeat.
var ErrPublish = errors.New("session: analytics not published")

// DB is what the manager writes through: the database or a transaction of the
// caller, which holds the only connection of gateway.db.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Publisher publishes analytics; eventbus.Bus is one.
type Publisher interface {
	Publish(ctx context.Context, ev eventbus.Event) error
}

// Config builds a manager. DB is required.
type Config struct {
	DB *sql.DB
	// Bus receives analytics.session.*; nil publishes nothing, which is the
	// replay mode: analytics are read there, not published (C-10).
	Bus Publisher
	// Idle is MV_GATEWAY_SESSION_IDLE; zero is DefaultIdle.
	Idle time.Duration
	Log  *slog.Logger
	// OnEnded, when set, is told of every session that ended, after its end is
	// recorded and published — the trigger of the snapshot of the gateway
	// (component §11.2). It runs under the lock of the manager and, for an end
	// inside a transaction of the caller, while that transaction holds the
	// only connection of gateway.db: it must return at once and must not read
	// gateway.db.
	OnEnded func(Session)
}

// Session is one row of sessions.
type Session struct {
	ID            string
	WorldID       string
	Scope         eventbus.ScopeRef
	Kind          string
	ActorKind     string
	Participants  []string
	StartedAt     time.Time
	LastActionAt  time.Time
	EndedAt       *time.Time
	EndReason     string
	TurnsCount    int
	TurnsDegraded int
	TurnsFailed   int
	State         string
}

// Manager is the sessions of gateway.db. It is safe for concurrent use: the
// check for an active session and the opening of a new one are one step.
type Manager struct {
	cfg Config
	mu  sync.Mutex
}

// New returns the manager of cfg.
func New(cfg Config) (*Manager, error) {
	if cfg.DB == nil {
		return nil, errors.New("session: DB is required")
	}
	if cfg.Idle < 0 {
		return nil, errors.New("session: Idle is not negative")
	}
	if cfg.Idle == 0 {
		cfg.Idle = DefaultIdle
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Manager{cfg: cfg}, nil
}

// Idle is how long a session waits for an action before it ends.
func (m *Manager) Idle() time.Duration { return m.cfg.Idle }

// Touch records an action of playerID in scope at now and returns the session
// it belongs to. A scope without an active session, or with one idle for Idle,
// gets a new session (opened is true), the idle one ending with end_reason=idle
// first. The actor kind of a new session is the actor kind of this action.
func (m *Manager) Touch(ctx context.Context, scope eventbus.ScopeRef, worldID, playerID, actorKind string, now time.Time) (Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, found, err := active(ctx, m.cfg.DB, scope.ID)
	if err != nil {
		return Session{}, false, err
	}
	if found && now.Sub(s.LastActionAt) < m.cfg.Idle {
		if now.After(s.LastActionAt) {
			if _, err := m.cfg.DB.ExecContext(ctx, `UPDATE sessions SET last_action_at = ? WHERE id = ?`, formatTime(now), s.ID); err != nil {
				return Session{}, false, fmt.Errorf("session: touch %s: %w", s.ID, err)
			}
			s.LastActionAt = now.UTC()
		}
		return s, false, nil
	}
	if found {
		if err := m.end(ctx, s, EndIdle, s.LastActionAt.Add(m.cfg.Idle)); err != nil {
			return Session{}, false, err
		}
	}
	s, err = m.open(ctx, scope, worldID, []string{playerID}, actorKind, now)
	return s, err == nil, err
}

// Open opens a session of a scope with its participants, the way a group opens
// one when it is created (component §6). A scope with an active session keeps
// it.
func (m *Manager) Open(ctx context.Context, scope eventbus.ScopeRef, worldID string, participants []string, actorKind string, now time.Time) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, found, err := active(ctx, m.cfg.DB, scope.ID); err != nil || found {
		return s, err
	}
	return m.open(ctx, scope, worldID, participants, actorKind, now)
}

// End ends the active session of scope with reason at now; a scope without one
// is left as it is. It is idempotent, as a hook of /forget must be.
func (m *Manager) End(ctx context.Context, scope eventbus.ScopeRef, reason string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, found, err := active(ctx, m.cfg.DB, scope.ID)
	if err != nil || !found {
		return err
	}
	return m.end(ctx, s, reason, now)
}

// UpdateParticipants replaces the participants of the active session of scope.
func (m *Manager) UpdateParticipants(ctx context.Context, scope eventbus.ScopeRef, participants []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, err := json.Marshal(participants)
	if err != nil {
		return fmt.Errorf("session: participants: %w", err)
	}
	if _, err := m.cfg.DB.ExecContext(ctx, `UPDATE sessions SET participants = ? WHERE scope_id = ? AND state = ?`,
		string(raw), scope.ID, StateActive); err != nil {
		return fmt.Errorf("session: update participants of %s: %w", scope.ID, err)
	}
	return nil
}

// Current is the active session of a scope.
func (m *Manager) Current(ctx context.Context, scopeID string) (Session, bool, error) {
	return active(ctx, m.cfg.DB, scopeID)
}

// Active lists the active sessions, the oldest first.
func (m *Manager) Active(ctx context.Context) ([]Session, error) {
	rows, err := m.cfg.DB.QueryContext(ctx, `SELECT `+columns+` FROM sessions WHERE state = ? ORDER BY started_at, id`, StateActive)
	if err != nil {
		return nil, fmt.Errorf("session: active: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Session
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("session: active: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Sweep ends with end_reason=idle every active session whose last action is
// Idle or more before now, and returns how many it ended. The time a session
// ends is the moment it became idle, not the tick of the sweeper, so the end
// does not depend on how often the sweeper runs.
func (m *Manager) Sweep(ctx context.Context, now time.Time) (int, error) {
	list, err := m.Active(ctx)
	if err != nil {
		return 0, err
	}
	ended := 0
	var errs []error
	for _, s := range list {
		if now.Sub(s.LastActionAt) < m.cfg.Idle {
			continue
		}
		done, err := m.endIdle(ctx, s.Scope.ID, now)
		if err != nil {
			errs = append(errs, err)
		}
		if done {
			ended++
		}
	}
	return ended, errors.Join(errs...)
}

// endIdle ends the active session of a scope if it is still idle at now: an
// action may have touched it since the sweeper listed it.
func (m *Manager) endIdle(ctx context.Context, scopeID string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, found, err := active(ctx, m.cfg.DB, scopeID)
	if err != nil || !found || now.Sub(s.LastActionAt) < m.cfg.Idle {
		return false, err
	}
	if err := m.end(ctx, s, EndIdle, s.LastActionAt.Add(m.cfg.Idle)); err != nil {
		return false, err
	}
	return true, nil
}

// CountTurn adds a turn to the counters of a session: turns_count for an
// accepted turn, turns_degraded for a turn narrated by a template and
// turns_failed for one that timed out. q is the transaction of the caller.
func CountTurn(ctx context.Context, q DB, sessionID string, c Counts) error {
	if _, err := q.ExecContext(ctx, `UPDATE sessions SET turns_count = turns_count + ?, turns_degraded = turns_degraded + ?,
		turns_failed = turns_failed + ? WHERE id = ?`, c.Turns, c.Degraded, c.Failed, sessionID); err != nil {
		return fmt.Errorf("session: count a turn of %s: %w", sessionID, err)
	}
	return nil
}

// Counts are the increments of the counters of a session.
type Counts struct {
	Turns, Degraded, Failed int
}

// open inserts a session and publishes its start; a start that is not
// published takes the row back. It must be called with m.mu held.
func (m *Manager) open(ctx context.Context, scope eventbus.ScopeRef, worldID string, participants []string, actorKind string, now time.Time) (Session, error) {
	if actorKind == "" {
		actorKind = eventbus.ActorHuman
	}
	kind := KindSolo
	if scope.Type == KindGroup {
		kind = KindGroup
	}
	s := Session{
		ID: ID(scope.ID, now), WorldID: worldID, Scope: scope, Kind: kind, ActorKind: actorKind,
		Participants: append([]string(nil), participants...), StartedAt: now.UTC(), LastActionAt: now.UTC(), State: StateActive,
	}
	raw, err := json.Marshal(s.Participants)
	if err != nil {
		return Session{}, fmt.Errorf("session: participants: %w", err)
	}
	if _, err := m.cfg.DB.ExecContext(ctx, `INSERT INTO sessions (id, world_id, scope_id, scope_type, kind, actor_kind,
		participants, started_at, last_action_at, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.WorldID, scope.ID, kind, kind, actorKind, string(raw), formatTime(now), formatTime(now), StateActive); err != nil {
		return Session{}, fmt.Errorf("session: open %s: %w", s.ID, err)
	}
	if err := m.publish(ctx, Started(s)); err != nil {
		_, undo := m.cfg.DB.ExecContext(context.WithoutCancel(ctx), `DELETE FROM sessions WHERE id = ?`, s.ID)
		return Session{}, errors.Join(err, undo)
	}
	return s, nil
}

// end marks a session ended and publishes its end; an end that is not
// published is taken back, so the next sweep or action repeats it. It must be
// called with m.mu held.
func (m *Manager) end(ctx context.Context, s Session, reason string, at time.Time) error {
	if at.Before(s.StartedAt) {
		at = s.StartedAt
	}
	if _, err := m.cfg.DB.ExecContext(ctx, `UPDATE sessions SET state = ?, ended_at = ?, end_reason = ? WHERE id = ? AND state = ?`,
		StateEnded, formatTime(at), reason, s.ID, StateActive); err != nil {
		return fmt.Errorf("session: end %s: %w", s.ID, err)
	}
	at = at.UTC()
	s.State, s.EndedAt, s.EndReason = StateEnded, &at, reason
	if err := m.publish(ctx, Ended(s)); err != nil {
		_, undo := m.cfg.DB.ExecContext(context.WithoutCancel(ctx),
			`UPDATE sessions SET state = ?, ended_at = NULL, end_reason = NULL WHERE id = ?`, StateActive, s.ID)
		return errors.Join(err, undo)
	}
	if m.cfg.OnEnded != nil {
		m.cfg.OnEnded(s)
	}
	return nil
}

func (m *Manager) publish(ctx context.Context, ev eventbus.Event) error {
	if m.cfg.Bus == nil {
		return nil
	}
	if err := m.cfg.Bus.Publish(ctx, ev); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrPublish, ev.Type, err)
	}
	return nil
}

// ID is the id of a session of scopeID started at startedAt:
// "{scope.id}:{started_at_unix}" (component §4.2).
func ID(scopeID string, startedAt time.Time) string {
	return scopeID + ":" + strconv.FormatInt(startedAt.Unix(), 10)
}

const columns = `id, world_id, scope_id, scope_type, kind, actor_kind, participants, started_at, last_action_at,
	ended_at, end_reason, turns_count, turns_degraded, turns_failed, state`

type scanner interface{ Scan(dest ...any) error }

func active(ctx context.Context, q DB, scopeID string) (Session, bool, error) {
	s, err := scan(q.QueryRowContext(ctx, `SELECT `+columns+` FROM sessions WHERE scope_id = ? AND state = ?`, scopeID, StateActive))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Session{}, false, nil
	case err != nil:
		return Session{}, false, fmt.Errorf("session: active session of a scope: %w", err)
	}
	return s, true, nil
}

func scan(row scanner) (Session, error) {
	var (
		s                  Session
		participants       string
		started, last      string
		endedAt, endReason sql.NullString
	)
	if err := row.Scan(&s.ID, &s.WorldID, &s.Scope.ID, &s.Scope.Type, &s.Kind, &s.ActorKind, &participants,
		&started, &last, &endedAt, &endReason, &s.TurnsCount, &s.TurnsDegraded, &s.TurnsFailed, &s.State); err != nil {
		return Session{}, err
	}
	if err := json.Unmarshal([]byte(participants), &s.Participants); err != nil {
		return Session{}, fmt.Errorf("participants of %s: %w", s.ID, err)
	}
	var err error
	if s.StartedAt, err = parseTime(started); err != nil {
		return Session{}, err
	}
	if s.LastActionAt, err = parseTime(last); err != nil {
		return Session{}, err
	}
	if endedAt.Valid {
		t, err := parseTime(endedAt.String)
		if err != nil {
			return Session{}, err
		}
		s.EndedAt = &t
	}
	s.EndReason = endReason.String
	return s, nil
}

// timeLayout keeps a fixed width, so that two times compare as text the way
// they compare as times (as in the other tables of gateway.db).
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
