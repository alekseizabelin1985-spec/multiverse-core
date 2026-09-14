package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"multiverse-core.io/internal/gateway/store"
)

// Config builds a store. DB is required.
type Config struct {
	DB *sql.DB
	// Lease is MV_GATEWAY_DELIVERY_LEASE; zero is DefaultLease.
	Lease time.Duration
	// TTL is MV_GATEWAY_DELIVERY_TTL; zero is DefaultTTL.
	TTL time.Duration
}

// Store is the outbox of gateway.db. It expects the database opened by
// store.OpenGateway, with its single connection: a method that takes a DB
// writes through the transaction it is given, every other method uses the
// database and never runs inside a transaction of its caller.
type Store struct {
	db       *sql.DB
	lease    time.Duration
	ttl      time.Duration
	notifier *Notifier
}

// New returns the store of cfg.
func New(cfg Config) (*Store, error) {
	if cfg.DB == nil {
		return nil, errors.New("outbox: DB is required")
	}
	if cfg.Lease < 0 || cfg.TTL < 0 {
		return nil, errors.New("outbox: Lease and TTL are not negative")
	}
	if cfg.Lease == 0 {
		cfg.Lease = DefaultLease
	}
	if cfg.TTL == 0 {
		cfg.TTL = DefaultTTL
	}
	return &Store{db: cfg.DB, lease: cfg.Lease, ttl: cfg.TTL, notifier: NewNotifier()}, nil
}

// Notifier is the bell the store rings after it enqueues or releases
// deliveries.
func (s *Store) Notifier() *Notifier { return s.notifier }

// Enqueue writes deliveries through q, the transaction of the consumer, at now,
// and returns how many new pending rows it wrote. It is idempotent: a delivery
// whose id is already in the table is not written again, so a repeat of the
// same event by the bus adds nothing (NFR-013). A delivery without an id gets
// EventDeliveryID.
//
// A delivery without a platform has no link to go to — the player ran /forget,
// or never had a link — and is written dropped, not pending, so that it does
// not pile up (component §7.5).
//
// The platforms of the new rows are notified before the transaction commits. A
// waiting long-poll leases on the same single connection, so it reads the rows
// only after the commit; a rollback wakes it for nothing, which is harmless.
func (s *Store) Enqueue(ctx context.Context, q DB, now time.Time, ds ...Delivery) (int, error) {
	written := 0
	var platforms []string
	for _, d := range ds {
		if err := check(d); err != nil {
			return written, err
		}
		if d.ID == "" {
			d.ID = EventDeliveryID(d.EventID, d.PlayerID, d.Kind)
		}
		state := StatePending
		if d.Platform == "" {
			state = StateDropped
		}
		var data any
		if len(d.Data) > 0 {
			raw, err := json.Marshal(d.Data)
			if err != nil {
				return written, fmt.Errorf("outbox: data of %s: %w", d.ID, err)
			}
			data = string(raw)
		}
		var round any
		if d.RoundSeq != nil {
			round = *d.RoundSeq
		}
		res, err := q.ExecContext(ctx, `INSERT INTO deliveries (id, world_id, player_id, platform, kind, correlation_id,
			event_id, round_seq, generated_by, fallback_reason, text, data, state, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO NOTHING`,
			d.ID, d.WorldID, d.PlayerID, d.Platform, d.Kind, d.CorrelationID, d.EventID, round, d.GeneratedBy,
			d.FallbackReason, d.Text, data, state, formatTime(now), formatTime(now.Add(s.ttl)))
		if err != nil {
			return written, fmt.Errorf("outbox: enqueue %s: %w", d.ID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return written, fmt.Errorf("outbox: enqueue %s: %w", d.ID, err)
		}
		if n > 0 && state == StatePending {
			written++
			if !slices.Contains(platforms, d.Platform) {
				platforms = append(platforms, d.Platform)
			}
		}
	}
	for _, p := range platforms {
		s.notifier.Notify(p)
	}
	return written, nil
}

func check(d Delivery) error {
	switch {
	case d.PlayerID == "", d.EventID == "", d.CorrelationID == "":
		return fmt.Errorf("outbox: a delivery of %q to %q needs player_id, event_id and correlation_id", d.EventID, d.PlayerID)
	case d.Kind == "", d.GeneratedBy == "", d.Text == "":
		return fmt.Errorf("outbox: the delivery of %s to %s needs kind, generated_by and text", d.EventID, d.PlayerID)
	}
	return nil
}

// Lease gives clientID the next deliveries of platform at now, at most limit of
// them: for every player the earliest pending delivery, when no other delivery
// of that player is in a lease that still runs, in the order of seq (component
// §8.2). The deliveries are leased to clientID for the lease of the store. The
// transaction ends before Lease returns: nothing waits on the connection.
//
// A lease that ran out does not stop another lease of the same delivery, and a
// delivery is never handed to two leases that run at the same time.
func (s *Store) Lease(ctx context.Context, clientID, platform string, limit int, now time.Time) (out []Delivery, err error) {
	if limit <= 0 {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("outbox: lease: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, ignoreDone(tx.Rollback()))
		}
	}()
	at := formatTime(now)
	rows, err := tx.QueryContext(ctx, `WITH ranked AS (
		  SELECT d.seq, ROW_NUMBER() OVER (PARTITION BY d.player_id ORDER BY d.seq) AS rn
		  FROM deliveries d
		  WHERE d.platform = ? AND d.state = 'pending'
		    AND (d.leased_until IS NULL OR d.leased_until < ?)
		    AND NOT EXISTS (SELECT 1 FROM deliveries x WHERE x.player_id = d.player_id AND x.state = 'pending'
		                    AND x.leased_until IS NOT NULL AND x.leased_until >= ? AND x.seq <> d.seq)
		)
		SELECT `+deliveryColumns+` FROM deliveries WHERE seq IN (SELECT seq FROM ranked WHERE rn = 1)
		ORDER BY seq LIMIT ?`, platform, at, at, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox: lease: %w", err)
	}
	out, err = scanDeliveries(rows)
	if err != nil {
		return nil, fmt.Errorf("outbox: lease: %w", err)
	}
	until := formatTime(now.Add(s.lease))
	for i := range out {
		if _, err = tx.ExecContext(ctx, `UPDATE deliveries SET leased_by = ?, leased_until = ?, attempts = attempts + 1
			WHERE id = ?`, clientID, until, out[i].ID); err != nil {
			return nil, fmt.Errorf("outbox: lease %s: %w", out[i].ID, err)
		}
		out[i].Attempts++
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("outbox: lease: %w", err)
	}
	return out, nil
}

// OnDelivered is called for every delivery an acknowledgement moves to
// delivered, once per delivery, through the transaction of the acknowledgement.
type OnDelivered func(ctx context.Context, q DB, d Delivery, at time.Time) error

// Ack marks delivered the deliveries of ids that clientID leased and that are
// still pending, at now, and calls onDelivered for each of them. An id the
// store does not know, a delivery leased by another client, already delivered
// or dropped comes back in unknown, and is not an error (component §8.4).
//
// The lease need not still run: a late acknowledgement of a delivery nobody
// leased again since is taken, so that a client that took longer than the lease
// to send a batch does not get its messages again. Once another client leased
// the delivery, the acknowledgement of the first one is unknown.
//
// Everything is one transaction: an error of onDelivered — analytics that could
// not be published — rolls every acknowledgement of the call back, and the
// client repeats it.
func (s *Store) Ack(ctx context.Context, clientID string, ids []string, now time.Time, onDelivered OnDelivered) (acked, unknown []string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("outbox: ack: %w", err)
	}
	defer func() {
		if err != nil {
			acked, unknown = nil, nil
			err = errors.Join(err, ignoreDone(tx.Rollback()))
		}
	}()
	acked, unknown = []string{}, []string{}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		rows, qerr := tx.QueryContext(ctx, `SELECT `+deliveryColumns+` FROM deliveries
			WHERE id = ? AND state = 'pending' AND leased_by = ?`, id, clientID)
		if qerr != nil {
			return nil, nil, fmt.Errorf("outbox: ack %s: %w", id, qerr)
		}
		found, serr := scanDeliveries(rows)
		if serr != nil {
			return nil, nil, fmt.Errorf("outbox: ack %s: %w", id, serr)
		}
		if len(found) == 0 {
			unknown = append(unknown, id)
			continue
		}
		if _, err = tx.ExecContext(ctx, `UPDATE deliveries SET state = 'delivered', delivered_at = ?, leased_until = NULL
			WHERE id = ? AND state = 'pending'`, formatTime(now), id); err != nil {
			return nil, nil, fmt.Errorf("outbox: ack %s: %w", id, err)
		}
		if onDelivered != nil {
			if err = onDelivered(ctx, tx, found[0], now); err != nil {
				return nil, nil, err
			}
		}
		acked = append(acked, id)
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("outbox: ack: %w", err)
	}
	return acked, unknown, nil
}

// Drop marks a pending delivery dropped: it has no route to go to.
func (s *Store) Drop(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE deliveries SET state = 'dropped', leased_until = NULL
		WHERE id = ? AND state = 'pending'`, id); err != nil {
		return fmt.Errorf("outbox: drop %s: %w", id, err)
	}
	return nil
}

// ReleaseExpiredLeases ends the leases that ran out by now, so that their
// deliveries are given out again, and wakes the waiting long-polls. leased_by
// stays: a late acknowledgement of the client that leased the delivery is
// still taken until somebody leases it again (Ack).
func (s *Store) ReleaseExpiredLeases(ctx context.Context, now time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE deliveries SET leased_until = NULL
		WHERE state = 'pending' AND leased_until IS NOT NULL AND leased_until < ?`, formatTime(now))
	if err != nil {
		return 0, fmt.Errorf("outbox: release expired leases: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("outbox: release expired leases: %w", err)
	}
	if n > 0 {
		s.notifier.NotifyAll()
	}
	return int(n), nil
}

// Pending counts the pending deliveries and returns how old the oldest of
// them is at now by created_at; zero age for an empty queue, and never a
// negative one (component §11.4: outbox_pending, outbox_oldest_age_s). It is a
// read of /health: ctx bounds the wait for the only connection.
func (s *Store) Pending(ctx context.Context, now time.Time) (int, time.Duration, error) {
	var (
		count  int
		oldest sql.NullString
	)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(created_at) FROM deliveries WHERE state = 'pending'`).
		Scan(&count, &oldest); err != nil {
		return 0, 0, fmt.Errorf("outbox: pending: %w", err)
	}
	if !oldest.Valid {
		return count, 0, nil
	}
	created, err := parseTime(oldest.String)
	if err != nil {
		return 0, 0, fmt.Errorf("outbox: pending: %w", err)
	}
	return count, max(now.Sub(created), 0), nil
}

// Expire drops the pending deliveries past their expires_at.
func (s *Store) Expire(ctx context.Context, now time.Time) (int, error) {
	return s.exec(ctx, "expire", `UPDATE deliveries SET state = 'dropped', leased_until = NULL
		WHERE state = 'pending' AND expires_at < ?`, formatTime(now))
}

// DropForPlayer drops every pending delivery of a player: the first step of
// the cascade of /forget (C-04 v1.1, component §7.5). A repeat drops nothing
// more.
func (s *Store) DropForPlayer(ctx context.Context, playerID string) (int, error) {
	return s.exec(ctx, "drop for player", `UPDATE deliveries SET state = 'dropped', leased_until = NULL
		WHERE player_id = ? AND state = 'pending'`, playerID)
}

// Purge deletes the delivered and dropped deliveries created before
// store.FinishedDeliveryKeep (component §4.2).
func (s *Store) Purge(ctx context.Context, now time.Time) (int, error) {
	return s.exec(ctx, "purge", `DELETE FROM deliveries WHERE state IN ('delivered', 'dropped') AND created_at < ?`,
		formatTime(now.Add(-store.FinishedDeliveryKeep)))
}

func (s *Store) exec(ctx context.Context, what, query string, args ...any) (int, error) {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("outbox: %s: %w", what, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("outbox: %s: %w", what, err)
	}
	return int(n), nil
}

const deliveryColumns = `seq, id, world_id, player_id, platform, kind, correlation_id, event_id, round_seq,
	generated_by, fallback_reason, text, data, state, attempts, created_at, expires_at`

func scanDeliveries(rows *sql.Rows) ([]Delivery, error) {
	defer func() { _ = rows.Close() }()
	var out []Delivery
	for rows.Next() {
		var (
			d                  Delivery
			round              sql.NullInt64
			fallback, data     sql.NullString
			created, expiresAt string
		)
		if err := rows.Scan(&d.Seq, &d.ID, &d.WorldID, &d.PlayerID, &d.Platform, &d.Kind, &d.CorrelationID, &d.EventID,
			&round, &d.GeneratedBy, &fallback, &d.Text, &data, &d.State, &d.Attempts, &created, &expiresAt); err != nil {
			return nil, err
		}
		if round.Valid {
			seq := int(round.Int64)
			d.RoundSeq = &seq
		}
		if fallback.Valid {
			reason := fallback.String
			d.FallbackReason = &reason
		}
		if data.Valid {
			if err := json.Unmarshal([]byte(data.String), &d.Data); err != nil {
				return nil, fmt.Errorf("data of %s: %w", d.ID, err)
			}
		}
		var err error
		if d.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if d.ExpiresAt, err = parseTime(expiresAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ignoreDone drops the error of a rollback after the transaction already
// ended: a failed commit has rolled back by itself.
func ignoreDone(err error) error {
	if errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}
