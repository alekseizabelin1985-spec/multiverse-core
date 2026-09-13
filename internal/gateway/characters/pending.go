package characters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/shared/entity"
)

// Pending is a row of pending_characters: a character proposed and not created
// yet (component §4.2). It names the player, never the link.
type Pending struct {
	ProposalID string
	PlayerID   string
	WorldID    string
	Name       string
	ActorKind  string
	CreatedAt  time.Time
	DeadlineAt time.Time
}

// The stats of a new character: the player row of the rules of the world
// (rules/dark-forest.yaml, entities.player), which data-model.md §3.3 has
// copied into the character when it is created. The gateway may not read the
// rules through internal/mechanics (component §3); characters_test.go keeps
// these numbers equal to the file.
const (
	StartHPMax = 10
	StartAtk   = 2
	StartDef   = 12
	StartDmg   = "d6"
	StartFlee  = "2"
)

// Attributes are the attributes of a new character (data-model.md §3.3): full
// hit points, the stats of the rules, alive, outside every region of its
// world, alone, empty-handed, with the actor kind of the request that created
// it.
func Attributes(p Pending) map[string]any {
	return map[string]any{
		entity.AttrHP:        StartHPMax,
		entity.AttrHPMax:     StartHPMax,
		entity.AttrAtk:       StartAtk,
		entity.AttrDef:       StartDef,
		entity.AttrDmg:       StartDmg,
		entity.AttrFlee:      StartFlee,
		entity.AttrStatus:    entity.StatusAlive,
		entity.AttrPosition:  "outside:" + p.WorldID,
		entity.AttrScope:     map[string]any{"id": p.PlayerID, "type": session.KindSolo},
		entity.AttrActorKind: p.ActorKind,
		entity.AttrInventory: []any{},
	}
}

// DB is what the rows of pending characters are written through.
type DB = session.DB

func insertPending(ctx context.Context, q DB, p Pending) error {
	if _, err := q.ExecContext(ctx, `INSERT INTO pending_characters (proposal_id, player_id, world_id, name, actor_kind,
		created_at, deadline_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ProposalID, p.PlayerID, p.WorldID, p.Name, p.ActorKind, formatTime(p.CreatedAt), formatTime(p.DeadlineAt)); err != nil {
		return fmt.Errorf("characters: record the pending character: %w", err)
	}
	return nil
}

func deletePending(ctx context.Context, q DB, proposalID string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM pending_characters WHERE proposal_id = ?`, proposalID); err != nil {
		return fmt.Errorf("characters: remove the pending character: %w", err)
	}
	return nil
}

const pendingColumns = `proposal_id, player_id, world_id, name, actor_kind, created_at, deadline_at`

func pendingOf(ctx context.Context, q DB, playerID string) (Pending, bool, error) {
	p, err := scanPending(q.QueryRowContext(ctx, `SELECT `+pendingColumns+` FROM pending_characters WHERE player_id = ?`, playerID))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Pending{}, false, nil
	case err != nil:
		return Pending{}, false, fmt.Errorf("characters: pending character: %w", err)
	}
	return p, true, nil
}

func expired(ctx context.Context, q DB, now time.Time) ([]Pending, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+pendingColumns+` FROM pending_characters WHERE deadline_at <= ?
		ORDER BY deadline_at, proposal_id`, formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("characters: expired pending characters: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Pending
	for rows.Next() {
		p, err := scanPending(rows)
		if err != nil {
			return nil, fmt.Errorf("characters: expired pending characters: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanPending(row scanner) (Pending, error) {
	var p Pending
	var created, deadline string
	if err := row.Scan(&p.ProposalID, &p.PlayerID, &p.WorldID, &p.Name, &p.ActorKind, &created, &deadline); err != nil {
		return Pending{}, err
	}
	var err error
	if p.CreatedAt, err = time.Parse(time.RFC3339Nano, created); err != nil {
		return Pending{}, err
	}
	if p.DeadlineAt, err = time.Parse(time.RFC3339Nano, deadline); err != nil {
		return Pending{}, err
	}
	return p, nil
}

// timeLayout keeps a fixed width, so that two times compare as text the way
// they compare as times (as in the other tables of gateway.db).
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }
