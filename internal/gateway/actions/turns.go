package actions

import (
	"context"
	"strconv"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/eventbus"
)

// Turn is an action as the turns of a session count it.
type Turn struct {
	Scope     eventbus.ScopeRef
	WorldID   string
	PlayerID  string
	Type      string
	ActorKind string
	At        time.Time
}

// Turns is the insertion point of the sessions and turns of T-306 (component
// §7.6). Begin names the turn an action is about to become and stores nothing,
// so an action whose publication fails leaves no turn behind (§5.5); Accepted
// records the turn of a published action and Rejected an action refused by its
// preconditions.
type Turns interface {
	Begin(ctx context.Context, t Turn) (api.TurnRef, error)
	Accepted(ctx context.Context, t Turn, ref api.TurnRef, eventID string) error
	Rejected(ctx context.Context, t Turn, code string) error
}

// MemoryTurns numbers the turns of each scope in memory: a session is opened by
// the first action of its scope in the process and never ends. It stands in
// for the session manager and the tracker of turns of T-306, which replace it.
type MemoryTurns struct {
	mu       sync.Mutex
	sessions map[string]*memorySession
}

type memorySession struct {
	id    string
	turns int
}

// NewMemoryTurns returns turns with no session.
func NewMemoryTurns() *MemoryTurns { return &MemoryTurns{sessions: make(map[string]*memorySession)} }

// Begin returns the next turn of the session of the scope; the id of a new
// session is "{scope.id}:{started_at_unix}" (component §4.2).
func (m *MemoryTurns) Begin(_ context.Context, t Turn) (api.TurnRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[t.Scope.ID]
	if s == nil {
		return api.TurnRef{Seq: 1, SessionID: t.Scope.ID + ":" + strconv.FormatInt(t.At.Unix(), 10)}, nil
	}
	return api.TurnRef{Seq: s.turns + 1, SessionID: s.id}, nil
}

// Accepted counts the turn in its session.
func (m *MemoryTurns) Accepted(_ context.Context, t Turn, ref api.TurnRef, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[t.Scope.ID]
	if s == nil {
		s = &memorySession{id: ref.SessionID}
		m.sessions[t.Scope.ID] = s
	}
	s.turns = max(s.turns, ref.Seq)
	return nil
}

// Rejected counts nothing: a refused action is no turn of the session.
func (m *MemoryTurns) Rejected(context.Context, Turn, string) error { return nil }
