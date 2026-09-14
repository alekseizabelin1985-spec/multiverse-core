package gateway

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/snapshot"
)

// The states of the snapshot of the gateway /health reports as snapshot.
const (
	// snapshotCorrupted: the pointer of the gateway is there and it, or the
	// object it names, does not check (US-011).
	snapshotCorrupted = "corrupted"
	// snapshotUnreadable: the store could not be read at the start.
	snapshotUnreadable = "unreadable"
	// snapshotWriteFailed: the last snapshot could not be written.
	snapshotWriteFailed = "write_failed"
	// snapshotNoLawsVersion: the projection knows the world, and the world has
	// no laws_version — a defect of the data, not a new world (component §11.2).
	snapshotNoLawsVersion = "no_laws_version"
)

// snapshots is the snapshot writer of a started context and, in live mode, the
// goroutine that writes a snapshot after a session ended. It holds no timer.
//
// A replay takes no snapshot of the gateway and announces none, neither after
// a session nor at Stop (component §11.2): the place of such a publication
// among the events is asynchronous, which a byte-for-byte replay (NFR-061)
// cannot have. The check of the last snapshot at the start reads only and
// runs in both modes.
type snapshots struct {
	writer   *snapshot.Writer
	worldID  string
	model    *readmodel.Model
	db       *sql.DB
	sessions *session.Manager
	log      *slog.Logger
	// setState reports the state of the snapshot to /health.
	setState func(string)
	// budget is SnapshotWriteBudget: the deadline of one write.
	budget time.Duration

	// live says the snapshots are taken: false in replay, where only the
	// check at the start runs.
	live    bool
	trigger chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}
}

// request asks for a snapshot after a session ended. It never blocks — it is
// session.Config.OnEnded — and requests that arrive while one is pending are
// one snapshot: it is taken after them and records all of them.
func (s *snapshots) request(session.Session) {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// run writes a snapshot for every request until ctx ends.
func (s *snapshots) run(ctx context.Context) {
	defer close(s.done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.trigger:
			if err := s.write(ctx, snapshot.ReasonSessionEnded); err != nil && ctx.Err() == nil {
				// A write past its budget is a failure too: ctx is the life of
				// the goroutine, not the deadline of the write.
				s.log.Error("snapshot after a session ended", slog.String("error", err.Error()))
				s.setState(snapshotWriteFailed)
			}
		}
	}
}

// stop ends the goroutine and waits for it, or for ctx.
func (s *snapshots) stop(ctx context.Context) error {
	if !s.live {
		return nil
	}
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("gateway: snapshot writer did not stop: %w", ctx.Err())
	}
}

// write takes one snapshot of what the gateway holds now; a written snapshot
// clears the state /health reports.
//
// laws_version is an attribute of the world in the projection, and nothing
// else is a source of it (component §11.2). A world the projection does not
// know yet is a new world without sessions: the snapshot is skipped with a
// warning. A world it knows without laws_version is a defect of the data: the
// snapshot is skipped with an error and /health reports no_laws_version, or
// the active sessions of the world would go without snapshots in silence.
func (s *snapshots) write(ctx context.Context, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, s.budget)
	defer cancel()
	st, known, err := s.state(ctx)
	if err != nil {
		return err
	}
	switch {
	case !known:
		s.log.Warn("snapshot not taken: the projection does not know the world yet",
			slog.String("world_id", s.worldID), slog.String("reason", reason))
		return nil
	case st.LawsVersion == "":
		s.log.Error("snapshot not taken: the world of the projection has no laws_version",
			slog.String("world_id", s.worldID), slog.String("reason", reason))
		s.setState(snapshotNoLawsVersion)
		return nil
	}
	if _, err := s.writer.Write(ctx, st, reason); err != nil {
		return err
	}
	s.setState("")
	return nil
}

// state reads what the snapshot records: the cursors, the hash and the laws of
// the projection, the active sessions and the open rounds.
func (s *snapshots) state(ctx context.Context) (st snapshot.State, worldKnown bool, err error) {
	active, err := s.sessions.Active(ctx)
	if err != nil {
		return snapshot.State{}, false, err
	}
	effects, err := consumer.Cursors(ctx, s.db)
	if err != nil {
		return snapshot.State{}, false, fmt.Errorf("gateway: cursors: %w", err)
	}
	for topic, last := range effects {
		effects[topic] = last + 1
	}
	rounds, err := openRounds(ctx, s.db)
	if err != nil {
		return snapshot.State{}, false, err
	}
	st = snapshot.State{
		Cursors:        snapshot.Cursors{Projection: s.model.Cursor(), Effects: effects},
		ProjectionHash: s.model.Hash(),
		OpenRounds:     rounds,
	}
	var w readmodel.World
	if w, worldKnown = s.model.World(s.worldID); worldKnown {
		st.LawsVersion = w.LawsVersion
	}
	for _, a := range active {
		st.ActiveSessions = append(st.ActiveSessions, snapshot.Session{
			ID: a.ID, WorldID: a.WorldID, Scope: a.Scope, Kind: a.Kind, ActorKind: a.ActorKind,
			Participants: a.Participants, StartedAt: a.StartedAt, LastActionAt: a.LastActionAt, TurnsCount: a.TurnsCount,
		})
	}
	return st, worldKnown, nil
}

// openRounds reads the open and closing rounds of the table rounds.
func openRounds(ctx context.Context, db *sql.DB) ([]snapshot.Round, error) {
	rows, err := db.QueryContext(ctx, `SELECT scope_id, seq, encounter_id, state, deadline_at FROM rounds
		WHERE state IN ('open', 'closing') ORDER BY scope_id, seq`)
	if err != nil {
		return nil, fmt.Errorf("gateway: open rounds: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []snapshot.Round
	for rows.Next() {
		var (
			r        snapshot.Round
			deadline string
		)
		if err := rows.Scan(&r.ScopeID, &r.Seq, &r.EncounterID, &r.State, &deadline); err != nil {
			return nil, fmt.Errorf("gateway: open rounds: %w", err)
		}
		if r.DeadlineAt, err = time.Parse(time.RFC3339Nano, deadline); err != nil {
			return nil, fmt.Errorf("gateway: deadline of round %s/%d: %w", r.ScopeID, r.Seq, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
