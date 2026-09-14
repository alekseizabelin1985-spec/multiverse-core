// Package turns tracks the turns of the players from the acceptance of an
// action to the delivery of its narrative, and publishes each of them once as
// analytics.turn.completed (component gateway-and-bot.md §7.6, C-10, US-038).
//
// A turn is a row of turns in gateway.db. An accepted action becomes a turn
// with the deadline MV_GATEWAY_TURN_TIMEOUT; the facts of its mechanics and
// its narrative move it on, and the acknowledgement of the narrative by its
// last recipient completes it — once its mechanics is recorded too, when the
// action has Phase 1 (component §7.7). A turn past its deadline times out,
// unless its narrative reached every recipient and only its mechanics is
// missing (Sweep). An action refused by its preconditions is a turn too: it is
// published at once with status=rejected.
//
// The tracker implements actions.Turns for the service of actions. The steps
// that follow the facts — OnMechanics, OnNarrative, OnDelivered — take the
// transaction of their caller, the consumer or the outbox, because gateway.db
// has one connection and the effects of an event commit together (T-307).
package turns

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

// DefaultTimeout is MV_GATEWAY_TURN_TIMEOUT by default.
const DefaultTimeout = 60 * time.Second

// DefaultPublishTimeout bounds one publication of analytics.turn.completed. The
// steps of a turn publish inside the transaction of the consumer, of an
// acknowledgement or of the sweeper, which holds the only connection of
// gateway.db: a broker that does not answer must not hold it longer than an
// ordinary request (api.RequestTimeout). The transaction is rolled back and the
// step repeated — by the bus, by the client, by the next tick — under the same
// id of the event (review #1 of T-306, Mi-2).
const DefaultPublishTimeout = api.RequestTimeout

// ErrPublish wraps a failure to publish analytics.turn.completed; the step that
// published it did not happen and is repeated.
var ErrPublish = errors.New("turns: analytics not published")

// Statuses of a row of turns (component §4.2).
const (
	StatusAccepted         = "accepted"
	StatusMechanicsApplied = "mechanics_applied"
	StatusNarrated         = "narrated"
	StatusCompleted        = "completed"
	StatusDegraded         = "degraded"
	StatusRejected         = "rejected"
	StatusTimeout          = "timeout"
)

// Statuses of analytics.turn.completed (C-10).
const (
	OutcomeOK       = "ok"
	OutcomeDegraded = "degraded"
	OutcomeRejected = "rejected"
	OutcomeTimeout  = "timeout"
)

// TypeCompleted is the analytics type of a turn.
const TypeCompleted = "analytics.turn.completed"

// Generators of a narrative (C-05, C-10).
const (
	GeneratedByLLM      = "llm"
	GeneratedByTemplate = "template"
	GeneratedByNone     = "none"
)

// phase1 are the actions whose turn has a Phase 1 — the column "Phase 1" of
// api-contracts.md §1.4 — each with the type of the event that is its
// mechanics: the decision of a fight or a flight, the fact of State of a move
// or a rest. A turn of one of them completes only once that mechanics is
// recorded (component §7.7); look, say and defend have none and complete by
// the acknowledgement of their narrative.
//
// The type is fixed because a fight has mechanics on two topics — the decision
// on game_events, the facts of State it causes on system_events — and C-01
// orders neither against the other: the first of any of them would make
// result_event_id depend on which topic the consumer read first. The decision
// is the result the player is told (component §7.2), and the facts of a
// fight are applied without a delivery.
var phase1 = map[string]string{
	api.ActionEnter:  outbox.TypeEntityUpdated,
	api.ActionLeave:  outbox.TypeEntityUpdated,
	api.ActionAttack: outbox.TypeCombatDecided,
	api.ActionFlee:   outbox.TypeCombatDecided,
	api.ActionRest:   outbox.TypeEntityUpdated,
}

// hasPhase1 reports whether the turn of an action waits for its mechanics.
func hasPhase1(actionType string) bool {
	_, ok := phase1[actionType]
	return ok
}

// notMechanicsOf are the actions with Phase 1 whose mechanics is not an event
// of type typ, sorted: the turns of those actions do not record it.
func notMechanicsOf(typ string) []any {
	out := make([]string, 0, len(phase1))
	for action, mechanics := range phase1 {
		if mechanics != typ {
			out = append(out, action)
		}
	}
	slices.Sort(out)
	args := make([]any, 0, len(out))
	for _, action := range out {
		args = append(args, action)
	}
	return args
}

// DB is what a step of a turn writes through: the database or the transaction
// of the caller.
type DB = session.DB

// Config builds a tracker. DB, Sessions and Clock are required.
type Config struct {
	DB       *sql.DB
	Sessions *session.Manager
	// Bus receives analytics.turn.completed; nil publishes nothing, which is
	// the replay mode (C-10).
	Bus     session.Publisher
	Clock   clock.Clock
	Timeout time.Duration
	// PublishTimeout bounds a publication; zero is DefaultPublishTimeout.
	PublishTimeout time.Duration
	// GMPath is MV_GM_PATH, the path every turn of the process takes.
	GMPath string
	Log    *slog.Logger
}

// Tracker is the turns of gateway.db. It is safe for concurrent use.
type Tracker struct {
	cfg Config

	mu sync.Mutex
	// next is the last number reserved in each session the process has seen.
	next map[string]int
	// listed runs in forgetEnded between the reading of the active sessions
	// and the cleanup of the reserves; nil outside the tests.
	listed func()
}

var _ actions.Turns = (*Tracker)(nil)

// New returns the tracker of cfg.
func New(cfg Config) (*Tracker, error) {
	if cfg.DB == nil || cfg.Sessions == nil || cfg.Clock == nil {
		return nil, errors.New("turns: DB, Sessions and Clock are required")
	}
	if cfg.Timeout < 0 || cfg.PublishTimeout < 0 {
		return nil, errors.New("turns: Timeout and PublishTimeout are not negative")
	}
	if cfg.PublishTimeout == 0 {
		cfg.PublishTimeout = DefaultPublishTimeout
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.GMPath == "" {
		cfg.GMPath = eventbus.GMPathAgent
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Tracker{cfg: cfg, next: make(map[string]int)}, nil
}

// Begin opens or continues the session of the scope of the action and reserves
// the next number of a turn in it. The number is taken whatever becomes of the
// action: a batch that fails to publish keeps it for its repeat, and another
// action of the scope gets the next one.
func (t *Tracker) Begin(ctx context.Context, turn actions.Turn) (api.TurnRef, error) {
	if turn.Scope.ID == "" {
		return api.TurnRef{}, errors.New("turns: an action without a scope")
	}
	s, _, err := t.cfg.Sessions.Touch(ctx, turn.Scope, turn.WorldID, turn.PlayerID, turn.ActorKind, turn.At)
	if err != nil {
		if errors.Is(err, session.ErrPublish) {
			return api.TurnRef{}, fmt.Errorf("%w: %w", actions.ErrBusUnavailable, err)
		}
		return api.TurnRef{}, err
	}
	seq, err := t.reserve(ctx, s.ID)
	if err != nil {
		return api.TurnRef{}, err
	}
	return api.TurnRef{Seq: seq, SessionID: s.ID}, nil
}

// Accepted records the turn of an action whose player.* event is about to be
// published and counts it in its session; acked_at is the moment of the record
// until Acked moves it. A repeat for the same event records nothing more.
//
// The row is written as accepted, not received (component §4.2): the steps of
// the facts, the narrative and the deadline read accepted, and a turn whose
// event did not go out is taken back by Withdrawn instead of waiting in a
// state of its own.
func (t *Tracker) Accepted(ctx context.Context, turn actions.Turn, ref api.TurnRef, eventID string) error {
	now := t.cfg.Clock.Now()
	return t.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO turns (correlation_id, session_id, seq, world_id, scope_id, scope_type,
			player_id, player_name, action_type, target_id, target_type, text_len, status, received_at, acked_at,
			deadline_at, gm_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (correlation_id) DO NOTHING`,
			eventID, ref.SessionID, ref.Seq, turn.WorldID, turn.Scope.ID, scopeType(turn.Scope), turn.PlayerID, turn.Name,
			turn.Type, nullable(turn.TargetID), nullable(turn.TargetType), textLen(turn), StatusAccepted,
			formatTime(turn.At), formatTime(now), formatTime(turn.At.Add(t.cfg.Timeout)), t.cfg.GMPath)
		if err != nil {
			return fmt.Errorf("turns: record the turn %s: %w", eventID, err)
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return err
		}
		return session.CountTurn(ctx, tx, ref.SessionID, session.Counts{Turns: 1})
	})
}

// Withdrawn takes back the turn of an action whose player.* event the bus did
// not acknowledge: the row goes, and the count of its session with it. A turn
// that already has its mechanics or its narrative stays — the event reached
// the broker after all — and a turn that is not there is not an error.
func (t *Tracker) Withdrawn(ctx context.Context, ref api.TurnRef, eventID string) error {
	return t.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM turns WHERE correlation_id = ? AND session_id = ? AND status = ?
			AND mechanics_at IS NULL`, eventID, ref.SessionID, StatusAccepted)
		if err != nil {
			return fmt.Errorf("turns: take back the turn %s: %w", eventID, err)
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return err
		}
		return session.CountTurn(ctx, tx, ref.SessionID, session.Counts{Turns: -1})
	})
}

// Acked records at as the moment the action of a turn was acknowledged and
// answered 202. A turn already finished keeps the moment it was published with.
func (t *Tracker) Acked(ctx context.Context, eventID string, at time.Time) error {
	if _, err := t.cfg.DB.ExecContext(ctx, `UPDATE turns SET acked_at = ? WHERE correlation_id = ? AND status IN (?, ?, ?)`,
		formatTime(at), eventID, StatusAccepted, StatusMechanicsApplied, StatusNarrated); err != nil {
		return fmt.Errorf("turns: acknowledgement of %s: %w", eventID, err)
	}
	return nil
}

// Rejected publishes analytics.turn.completed status=rejected for an action
// refused by its preconditions, with the time it was received and the time it
// was answered, and records it as a turn of the active session of its scope.
//
// An action without a session to count it in is not published: a refused
// action does not open a session (metrics.md §4.2, "the first accepted
// action"), and the schema of the turn requires one. That covers an action of a
// player the projection does not know, which has no scope at all.
func (t *Tracker) Rejected(ctx context.Context, turn actions.Turn, code string) error {
	if turn.Scope.ID == "" {
		return nil
	}
	s, found, err := t.cfg.Sessions.Current(ctx, turn.Scope.ID)
	if err != nil || !found {
		return err
	}
	seq, err := t.reserve(ctx, s.ID)
	if err != nil {
		return err
	}
	now := t.cfg.Clock.Now()
	row := Row{
		SessionID: s.ID, Seq: seq, WorldID: turn.WorldID, PlayerID: turn.PlayerID, ActionType: turn.Type,
		Status: StatusRejected, ReceivedAt: turn.At, AckedAt: now, GMPath: t.cfg.GMPath, ActorKind: s.ActorKind,
	}
	ev := eventbus.NewRoot(TypeCompleted, contracts.SourceGateway, turn.WorldID, nil, s.ActorKind, Payload(row))
	if err := t.publish(ctx, ev); err != nil {
		return err
	}
	if _, err := t.cfg.DB.ExecContext(ctx, `INSERT INTO turns (correlation_id, session_id, seq, world_id, scope_id, scope_type,
		player_id, player_name, action_type, status, received_at, acked_at, gm_path, reject_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, s.ID, seq, turn.WorldID, turn.Scope.ID, scopeType(turn.Scope), turn.PlayerID, turn.Name, turn.Type,
		StatusRejected, formatTime(turn.At), formatTime(now), t.cfg.GMPath, code); err != nil {
		return fmt.Errorf("turns: record the refused turn: %w", err)
	}
	return nil
}

// OnMechanics records the fact of Phase 1 of a turn: the first combat.decided
// or entity.updated of its correlation — for an action with Phase 1, the first
// of the type of its mechanics (phase1). A turn whose narrative every
// recipient has already acknowledged was waiting for it and completes here,
// with the narrative_at of that acknowledgement (component §7.7). q is the
// transaction of the consumer.
//
// The completion does not fail the effect of the mechanics: when
// analytics.turn.completed is not published, the completion alone is rolled
// back, the mechanics stays recorded, and the next Sweep publishes the turn
// under the same id. Failing the effect would repeat it on the bus and, after
// the repeats, send it to dead letters with the text of the mechanics the
// player is owed — a text lost for a counter (component §5.5, §7.7).
func (t *Tracker) OnMechanics(ctx context.Context, q DB, ev eventbus.Event, at time.Time) error {
	pa := ev.Path()
	mode, _ := pa.GetString("phase1_mode")
	lod, _ := pa.GetString("lod")
	others := notMechanicsOf(ev.Type)
	args := append([]any{formatTime(at), ev.ID, nullable(mode), nullable(lod), StatusAccepted, StatusMechanicsApplied,
		ev.CorrelationID(), StatusAccepted, StatusMechanicsApplied, StatusNarrated}, others...)
	res, err := q.ExecContext(ctx, `UPDATE turns SET mechanics_at = COALESCE(mechanics_at, ?),
		result_event_id = COALESCE(result_event_id, ?), phase1_mode = COALESCE(phase1_mode, ?), lod = COALESCE(lod, ?),
		status = CASE WHEN status = ? THEN ? ELSE status END
		WHERE correlation_id = ? AND status IN (?, ?, ?)`+notIn("action_type", len(others)), args...)
	if err != nil {
		return fmt.Errorf("turns: mechanics of %s: %w", ev.CorrelationID(), err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return err
	}
	return t.completeByMechanics(ctx, q, ev.CorrelationID())
}

// completeByMechanics completes a turn that waited for its mechanics inside a
// savepoint of q, and rolls the savepoint back when the publication fails:
// what q wrote before stays, and so does the mechanics of the turn.
func (t *Tracker) completeByMechanics(ctx context.Context, q DB, correlationID string) error {
	if _, err := q.ExecContext(ctx, `SAVEPOINT turn_completion`); err != nil {
		return fmt.Errorf("turns: completion of %s: %w", correlationID, err)
	}
	err := t.completeDelivered(ctx, q, correlationID, nil)
	if errors.Is(err, ErrPublish) {
		if _, rollback := q.ExecContext(ctx, `ROLLBACK TO turn_completion`); rollback != nil {
			return errors.Join(err, rollback)
		}
		t.cfg.Log.LogAttrs(ctx, slog.LevelWarn, "turns: a turn is left to the sweeper: its completion was not published",
			slog.String("correlation_id", correlationID), slog.String("error", err.Error()), slog.Bool("handled", true))
		err = nil
	}
	if _, release := q.ExecContext(ctx, `RELEASE turn_completion`); release != nil {
		return errors.Join(err, fmt.Errorf("turns: completion of %s: %w", correlationID, release))
	}
	return err
}

// OnNarrative records the narrative.output of a turn: what generated it, the
// absence summary it carries, and recipients — the number of deliveries of the
// narrative an acknowledgement can complete, which the consumer counts: the
// distinct players of recipients[] that have a link. A recipient without a
// link, a repeated one or one that is not a player gets no delivery to
// acknowledge, and counting it would leave the turn waiting for its deadline
// (review #1 of T-307, Ma-1). q is the transaction of the consumer.
//
// A narrative with nobody to deliver it to completes its turn here, ok or
// degraded for a template, with narrative_at the moment the narrative is
// applied, instead of timing out at its deadline (acceptance of T-306).
func (t *Tracker) OnNarrative(ctx context.Context, q DB, ev eventbus.Event, recipients int) error {
	var p narrativePayload
	raw, err := json.Marshal(ev.Payload)
	if err == nil {
		err = json.Unmarshal(raw, &p)
	}
	if err != nil {
		return fmt.Errorf("turns: narrative %s: %w", ev.ID, err)
	}
	narrativeID := p.NarrativeEventID
	if narrativeID == "" {
		narrativeID = ev.ID
	}
	var level, blueprint any
	if ev.Meta.Agent != nil {
		level, blueprint = nullable(ev.Meta.Agent.Level), nullable(ev.Meta.Agent.Blueprint)
	}
	filtered := 0
	if p.Filter != nil && p.Filter.Applied {
		filtered = 1
	}
	var absence any
	if p.Absence != nil {
		surfaced := make([]string, 0, len(p.BackgroundRefs))
		for _, r := range p.BackgroundRefs {
			surfaced = append(surfaced, r.Event.ID)
		}
		a, err := json.Marshal(Absence{SinceAt: p.Absence.SinceAt, BackgroundEventsCount: p.Absence.BackgroundEventsCount, SurfacedEventIDs: surfaced})
		if err != nil {
			return fmt.Errorf("turns: absence of %s: %w", ev.ID, err)
		}
		absence = string(a)
	}
	if _, err := q.ExecContext(ctx, `UPDATE turns SET status = ?, generated_by = ?, fallback_reason = ?, agent_level = ?,
		agent_blueprint = ?, filter_applied = ?, recipients_count = ?, delivered_count = COALESCE(delivered_count, 0),
		narrative_event_id = ?, absence = ? WHERE correlation_id = ? AND status IN (?, ?)`,
		StatusNarrated, p.GeneratedBy, p.FallbackReason, level, blueprint, filtered, recipients, narrativeID,
		absence, ev.CorrelationID(), StatusAccepted, StatusMechanicsApplied); err != nil {
		return fmt.Errorf("turns: narrative of %s: %w", ev.CorrelationID(), err)
	}
	if recipients > 0 {
		return nil
	}
	now := t.cfg.Clock.Now()
	return t.completeDelivered(ctx, q, ev.CorrelationID(), &now)
}

// OnDelivered counts one delivery of the narrative of a turn acknowledged at
// at. The acknowledgement of the last recipient records narrative_at, that
// moment, and completes the turn: analytics.turn.completed goes out with
// status ok, or degraded when a template told the narrative. A turn with
// Phase 1 whose mechanics is not recorded yet stays narrated, and OnMechanics
// or Sweep completes it (component §7.7). q is the transaction of the caller;
// a failure to publish fails it, so the acknowledgement is repeated and the
// completion with it.
//
// Deliveries are counted up to the recipients: a turn waiting for its
// mechanics does not count the delivery of a second narrative of its
// correlation, which a turn completed by the acknowledgement before it would
// not have counted either.
func (t *Tracker) OnDelivered(ctx context.Context, q DB, correlationID string, at time.Time) error {
	if _, err := q.ExecContext(ctx, `UPDATE turns SET delivered_count = COALESCE(delivered_count, 0) + 1
		WHERE correlation_id = ? AND status = ? AND COALESCE(delivered_count, 0) < COALESCE(recipients_count, 0)`,
		correlationID, StatusNarrated); err != nil {
		return fmt.Errorf("turns: delivery of %s: %w", correlationID, err)
	}
	return t.completeDelivered(ctx, q, correlationID, &at)
}

// completeDelivered completes a narrated turn whose every recipient has its
// narrative. acked is the moment the last of them acknowledged it, recorded as
// narrative_at when the turn has none yet; nil is a step that is not an
// acknowledgement — the mechanics of a turn waiting for it.
func (t *Tracker) completeDelivered(ctx context.Context, q DB, correlationID string, acked *time.Time) error {
	row, found, err := load(ctx, q, correlationID)
	if err != nil || !found || row.Status != StatusNarrated || row.DeliveredCount < row.RecipientsCount {
		return err
	}
	if row.NarrativeAt == nil && acked != nil {
		if _, err := q.ExecContext(ctx, `UPDATE turns SET narrative_at = ? WHERE correlation_id = ?`,
			formatTime(*acked), correlationID); err != nil {
			return fmt.Errorf("turns: narrative of %s delivered: %w", correlationID, err)
		}
		row.NarrativeAt = acked
	}
	if row.NarrativeAt == nil || (hasPhase1(row.ActionType) && row.MechanicsAt == nil) {
		return nil
	}
	status, counts := narratedOutcome(row)
	return t.finish(ctx, q, row, status, counts)
}

// Sweep closes the open turns whose deadline passed by now and publishes each
// of them; it returns how many it closed. A turn whose narrative every
// recipient has acknowledged, and which waited for its mechanics in vain — a
// proposal State refused, a lost event — completes with the status of its
// narrative, without the fields of the mechanics, and is logged: the player has
// had it all (component §7.7). Every other turn past its deadline times out.
//
// A turn that has all it waited for — its narrative delivered, its mechanics
// recorded — and is still open, because OnMechanics could not publish its
// completion, is completed whatever its deadline, with its mechanics. Sweep
// also forgets the reserved numbers of the sessions that are no longer active.
func (t *Tracker) Sweep(ctx context.Context, now time.Time) (int, error) {
	ids, err := t.due(ctx, now)
	if err != nil {
		return 0, err
	}
	closed := 0
	var errs []error
	for _, id := range ids {
		var finished *Row
		err := t.inTx(ctx, func(tx *sql.Tx) error {
			row, found, err := load(ctx, tx, id)
			if err != nil || !found || !open(row.Status) {
				return err
			}
			finished = &row
			if !delivered(row) {
				return t.finish(ctx, tx, row, StatusTimeout, session.Counts{Failed: 1})
			}
			status, counts := narratedOutcome(row)
			return t.finish(ctx, tx, row, status, counts)
		})
		switch {
		case err != nil:
			errs = append(errs, err)
		case finished != nil:
			closed++
			if delivered(*finished) && finished.MechanicsAt == nil {
				t.cfg.Log.LogAttrs(ctx, slog.LevelWarn, "turns: a delivered turn completed without its mechanics",
					slog.String("correlation_id", finished.CorrelationID), slog.String("action_type", finished.ActionType),
					slog.Bool("handled", true))
			}
		}
	}
	return closed, errors.Join(append(errs, t.forgetEnded(ctx))...)
}

// notIn is the condition "column is none of n parameters", empty for none.
func notIn(column string, n int) string {
	if n == 0 {
		return ""
	}
	return " AND " + column + " NOT IN (?" + strings.Repeat(", ?", n-1) + ")"
}

// delivered reports whether every recipient of the narrative of an open turn
// has acknowledged it: only a turn waiting for its mechanics stays open so.
func delivered(row Row) bool {
	return row.Status == StatusNarrated && row.DeliveredCount >= row.RecipientsCount && row.NarrativeAt != nil
}

// narratedOutcome is the final status of a turn whose narrative reached its
// recipients, and what it adds to the counters of its session: degraded when a
// template told it.
func narratedOutcome(row Row) (string, session.Counts) {
	if row.GeneratedBy == GeneratedByTemplate {
		return StatusDegraded, session.Counts{Degraded: 1}
	}
	return StatusCompleted, session.Counts{}
}

// finish moves a turn to its final status, counts it in its session and
// publishes it, all within q. The turn keeps the narrative_at it has recorded.
func (t *Tracker) finish(ctx context.Context, q DB, row Row, status string, counts session.Counts) error {
	if _, err := q.ExecContext(ctx, `UPDATE turns SET status = ? WHERE correlation_id = ?`,
		status, row.CorrelationID); err != nil {
		return fmt.Errorf("turns: finish %s: %w", row.CorrelationID, err)
	}
	if err := session.CountTurn(ctx, q, row.SessionID, counts); err != nil {
		return err
	}
	row.Status = status
	return t.publish(ctx, Completed(row))
}

// due are the open turns Sweep closes at now: those past their deadline, and
// the narrated ones that have their narrative delivered and their mechanics
// recorded — left open by a completion that was not published.
func (t *Tracker) due(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := t.cfg.DB.QueryContext(ctx, `SELECT correlation_id FROM turns WHERE status IN (?, ?, ?) AND (deadline_at <= ?
		OR (status = ? AND mechanics_at IS NOT NULL AND narrative_at IS NOT NULL
			AND COALESCE(delivered_count, 0) >= COALESCE(recipients_count, 0)))
		ORDER BY deadline_at, correlation_id`, StatusAccepted, StatusMechanicsApplied, StatusNarrated, formatTime(now),
		StatusNarrated)
	if err != nil {
		return nil, fmt.Errorf("turns: turns due: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("turns: turns due: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// reserve takes the next number of a turn in a session. The first number a
// process takes in a session follows the highest number gateway.db holds for
// it.
func (t *Tracker) reserve(ctx context.Context, sessionID string) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, known := t.next[sessionID]
	if !known {
		if err := t.cfg.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM turns WHERE session_id = ?`, sessionID).Scan(&n); err != nil {
			return 0, fmt.Errorf("turns: number of the next turn of %s: %w", sessionID, err)
		}
	}
	n++
	t.next[sessionID] = n
	return n, nil
}

// forgetEnded drops the reserved numbers of the sessions that ended.
//
// The active sessions are read under the lock of the reserves (Mi-1 of review
// #1 of T-306). Begin opens a session before it reserves a number, and reserves
// it under this lock: a session that is not in the list read here has reserved
// nothing yet, and one opened after the reading waits for the lock and reserves
// after the cleanup. Read before the lock, the list would miss a session that
// opened and reserved in between, and its reserve would be dropped. The order
// "lock of the reserves, then gateway.db" is the one of reserve.
func (t *Tracker) forgetEnded(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	list, err := t.cfg.Sessions.Active(ctx)
	if err != nil {
		return err
	}
	if t.listed != nil {
		t.listed()
	}
	active := make(map[string]struct{}, len(list))
	for _, s := range list {
		active[s.ID] = struct{}{}
	}
	for id := range t.next {
		if _, ok := active[id]; !ok {
			delete(t.next, id)
		}
	}
	return nil
}

func (t *Tracker) publish(ctx context.Context, ev eventbus.Event) error {
	if t.cfg.Bus == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, t.cfg.PublishTimeout)
	defer cancel()
	if err := t.cfg.Bus.Publish(ctx, ev); err != nil {
		return fmt.Errorf("%w: %s %s: %w", ErrPublish, ev.Type, ev.ID, err)
	}
	return nil
}

func (t *Tracker) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := t.cfg.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("turns: begin: %w", err)
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

func open(status string) bool {
	return status == StatusAccepted || status == StatusMechanicsApplied || status == StatusNarrated
}

func scopeType(s eventbus.ScopeRef) string {
	if s.Type == session.KindGroup {
		return session.KindGroup
	}
	return session.KindSolo
}

func textLen(turn actions.Turn) any {
	if turn.Type != api.ActionSay {
		return nil
	}
	return turn.TextLen
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// narrativePayload is what a turn takes of narrative.output (C-05).
type narrativePayload struct {
	GeneratedBy      string  `json:"generated_by"`
	FallbackReason   *string `json:"fallback_reason"`
	NarrativeEventID string  `json:"narrative_event_id"`
	Filter           *struct {
		Applied bool `json:"applied"`
	} `json:"filter"`
	Absence *struct {
		SinceAt               string `json:"since_at"`
		BackgroundEventsCount int    `json:"background_events_count"`
	} `json:"absence"`
	BackgroundRefs []struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	} `json:"background_refs"`
}
