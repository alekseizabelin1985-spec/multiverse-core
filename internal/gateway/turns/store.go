package turns

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Row is a turn as gateway.db holds it, with the actor kind of its session.
type Row struct {
	CorrelationID    string
	SessionID        string
	Seq              int
	WorldID          string
	PlayerID         string
	ActionType       string
	TargetID         string
	TargetType       string
	Status           string
	ReceivedAt       time.Time
	AckedAt          time.Time
	MechanicsAt      *time.Time
	NarrativeAt      *time.Time
	GMPath           string
	Phase1Mode       string
	LOD              string
	GeneratedBy      string
	AgentLevel       string
	AgentBlueprint   string
	FallbackReason   *string
	FilterApplied    bool
	RecipientsCount  int
	DeliveredCount   int
	ResultEventID    string
	NarrativeEventID string
	Absence          *Absence
	ActorKind        string
}

// Absence is the while-you-were-away summary a narrative of entry carried,
// with the ids of the background events it surfaced (metrics.md §4.2).
type Absence struct {
	SinceAt               string   `json:"since_at"`
	BackgroundEventsCount int      `json:"background_events_count"`
	SurfacedEventIDs      []string `json:"surfaced_event_ids"`
}

// Load returns the turn of a correlation id.
func Load(ctx context.Context, q DB, correlationID string) (Row, bool, error) {
	return load(ctx, q, correlationID)
}

func load(ctx context.Context, q DB, correlationID string) (Row, bool, error) {
	var (
		r                                    Row
		target, targetType, acked, mechanics sql.NullString
		narrative, phase1, lod, generatedBy  sql.NullString
		level, blueprint, fallback, gmPath   sql.NullString
		resultID, narrativeID, absence       sql.NullString
		received                             string
		filtered, recipients, delivered      sql.NullInt64
	)
	err := q.QueryRowContext(ctx, `SELECT t.correlation_id, t.session_id, t.seq, t.world_id, t.player_id, t.action_type,
		t.target_id, t.target_type, t.status, t.received_at, t.acked_at, t.mechanics_at, t.narrative_at, t.gm_path,
		t.phase1_mode, t.lod, t.generated_by, t.agent_level, t.agent_blueprint, t.fallback_reason, t.filter_applied,
		t.recipients_count, t.delivered_count, t.result_event_id, t.narrative_event_id, t.absence, s.actor_kind
		FROM turns t JOIN sessions s ON s.id = t.session_id WHERE t.correlation_id = ?`, correlationID).
		Scan(&r.CorrelationID, &r.SessionID, &r.Seq, &r.WorldID, &r.PlayerID, &r.ActionType, &target, &targetType,
			&r.Status, &received, &acked, &mechanics, &narrative, &gmPath, &phase1, &lod, &generatedBy, &level,
			&blueprint, &fallback, &filtered, &recipients, &delivered, &resultID, &narrativeID, &absence, &r.ActorKind)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Row{}, false, nil
	case err != nil:
		return Row{}, false, fmt.Errorf("turns: load %s: %w", correlationID, err)
	}
	r.TargetID, r.TargetType, r.GMPath = target.String, targetType.String, gmPath.String
	r.Phase1Mode, r.LOD, r.GeneratedBy = phase1.String, lod.String, generatedBy.String
	r.AgentLevel, r.AgentBlueprint = level.String, blueprint.String
	r.ResultEventID, r.NarrativeEventID = resultID.String, narrativeID.String
	r.FilterApplied = filtered.Int64 != 0
	r.RecipientsCount, r.DeliveredCount = int(recipients.Int64), int(delivered.Int64)
	if fallback.Valid {
		reason := fallback.String
		r.FallbackReason = &reason
	}
	if r.ReceivedAt, err = parseTime(received); err != nil {
		return Row{}, false, err
	}
	for _, f := range []struct {
		src sql.NullString
		dst **time.Time
	}{{mechanics, &r.MechanicsAt}, {narrative, &r.NarrativeAt}} {
		if !f.src.Valid {
			continue
		}
		at, err := parseTime(f.src.String)
		if err != nil {
			return Row{}, false, err
		}
		*f.dst = &at
	}
	if acked.Valid {
		if r.AckedAt, err = parseTime(acked.String); err != nil {
			return Row{}, false, err
		}
	}
	if absence.Valid {
		var a Absence
		if err := json.Unmarshal([]byte(absence.String), &a); err != nil {
			return Row{}, false, fmt.Errorf("turns: absence of %s: %w", correlationID, err)
		}
		r.Absence = &a
	}
	return r, true, nil
}

// Completed is analytics.turn.completed of a finished turn of an accepted
// action. It is derived from the action: its causation is the action and its
// correlation the chain of the action (the schema says so), and its id comes
// from the action, so a repeat of the same completion is the same event.
func Completed(r Row) eventbus.Event {
	action := eventbus.Event{
		ID:        r.CorrelationID,
		Type:      actionEventType(r.ActionType),
		Timestamp: r.ReceivedAt.UTC(),
		World:     &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: r.WorldID, Type: entity.TypeWorld}},
		Meta: eventbus.Meta{CorrelationID: r.CorrelationID, ActorKind: r.ActorKind, Locale: eventbus.DefaultLocale,
			GMPath: gmPathOr(r.GMPath)},
	}
	return eventbus.Derive(action, TypeCompleted, contracts.SourceGateway, Payload(r), eventbus.WithCauseID())
}

// Payload is the payload of analytics.turn.completed of a turn (C-10). Like
// the analytics of a session it carries what the schema requires and what the
// turn measured, and neither the name of the character nor a copy of the
// world and the scope.
func Payload(r Row) map[string]any {
	turn := map[string]any{
		"seq":         r.Seq,
		"action_type": r.ActionType,
		"status":      outcome(r.Status),
		"gm_path":     gmPathOr(r.GMPath),
	}
	if r.TargetID != "" && r.TargetType != "" {
		turn["target"] = map[string]any{"entity": map[string]any{"id": r.TargetID, "type": r.TargetType}}
	}
	if r.Phase1Mode != "" {
		turn["phase1_mode"] = r.Phase1Mode
	}
	if r.LOD != "" {
		turn["lod"] = r.LOD
	}
	timings := map[string]any{"received_at": session.Timestamp(r.ReceivedAt), "acked_at": session.Timestamp(r.AckedAt)}
	since := r.ReceivedAt
	if r.MechanicsAt != nil {
		timings["mechanics_at"] = session.Timestamp(*r.MechanicsAt)
		timings["mechanics_ms"] = millis(r.MechanicsAt.Sub(r.ReceivedAt))
		since = *r.MechanicsAt
	}
	if r.NarrativeAt != nil {
		timings["narrative_at"] = session.Timestamp(*r.NarrativeAt)
		timings["narrative_ms"] = millis(r.NarrativeAt.Sub(since))
		timings["total_ms"] = millis(r.NarrativeAt.Sub(r.ReceivedAt))
	}
	generatedBy := r.GeneratedBy
	if generatedBy == "" {
		generatedBy = GeneratedByNone
	}
	narrative := map[string]any{"generated_by": generatedBy, "filter_applied": r.FilterApplied}
	if r.AgentLevel != "" {
		narrative["agent_level"] = r.AgentLevel
	}
	if r.AgentBlueprint != "" {
		narrative["agent_blueprint"] = r.AgentBlueprint
	}
	if r.FallbackReason != nil {
		narrative["fallback_reason"] = *r.FallbackReason
	}
	delivery := map[string]any{"recipients_count": r.RecipientsCount, "delivered_count": r.DeliveredCount}
	if r.ResultEventID != "" {
		delivery["result_event_id"] = r.ResultEventID
	}
	if r.NarrativeEventID != "" {
		delivery["narrative_event_id"] = r.NarrativeEventID
	}
	payload := map[string]any{
		"entity":    map[string]any{"entity": map[string]any{"id": r.PlayerID, "type": entity.TypePlayer}},
		"session":   map[string]any{"id": r.SessionID},
		"turn":      turn,
		"timings":   timings,
		"narrative": narrative,
		"delivery":  delivery,
	}
	if a := r.Absence; a != nil {
		surfaced := make([]any, 0, len(a.SurfacedEventIDs))
		for _, id := range a.SurfacedEventIDs {
			surfaced = append(surfaced, id)
		}
		payload["absence"] = map[string]any{"since_at": a.SinceAt, "background_events_count": a.BackgroundEventsCount,
			"surfaced_event_ids": surfaced}
	}
	return payload
}

// outcome is the status of analytics.turn.completed for the status of a row.
func outcome(status string) string {
	switch status {
	case StatusDegraded:
		return OutcomeDegraded
	case StatusRejected:
		return OutcomeRejected
	case StatusTimeout:
		return OutcomeTimeout
	default:
		return OutcomeOK
	}
}

func actionEventType(actionType string) string {
	if rule, ok := actions.Rules[actionType]; ok && rule.Event != "" {
		return rule.Event
	}
	return actionType
}

func gmPathOr(path string) string {
	if path == "" {
		return eventbus.GMPathAgent
	}
	return path
}

func millis(d time.Duration) int64 {
	if d < 0 {
		return 0
	}
	return d.Milliseconds()
}

// timeLayout keeps a fixed width, so that two times compare as text the way
// they compare as times (as in the other tables of gateway.db).
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
