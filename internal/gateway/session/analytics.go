package session

import (
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// The analytics types of a session (C-10).
const (
	TypeStarted = "analytics.session.started"
	TypeEnded   = "analytics.session.ended"
)

// Started is analytics.session.started of s (C-10, metrics.md §4.2).
//
// The payload carries what the schema requires and nothing it leaves optional:
// no copy of world or scope — the world is in the envelope, which keys the
// message — no scope in the envelope either, and no names. Analytics carry no
// free text of a player, and a character name is one.
func Started(s Session) eventbus.Event {
	return root(TypeStarted, s, map[string]any{
		"session": map[string]any{
			"id":            s.ID,
			"kind":          s.Kind,
			"actor_kind":    s.ActorKind,
			"started_at":    Timestamp(s.StartedAt),
			"players_count": len(s.Participants),
		},
		"participants": participants(s.Participants),
	})
}

// Ended is analytics.session.ended of s, which must be ended.
func Ended(s Session) eventbus.Event {
	session := map[string]any{
		"id":             s.ID,
		"kind":           s.Kind,
		"actor_kind":     s.ActorKind,
		"started_at":     Timestamp(s.StartedAt),
		"end_reason":     s.EndReason,
		"players_count":  len(s.Participants),
		"turns_count":    s.TurnsCount,
		"turns_degraded": s.TurnsDegraded,
		"turns_failed":   s.TurnsFailed,
	}
	if s.EndedAt != nil {
		session["ended_at"] = Timestamp(*s.EndedAt)
	}
	return root(TypeEnded, s, map[string]any{"session": session, "participants": participants(s.Participants)})
}

func root(typ string, s Session, payload map[string]any) eventbus.Event {
	return eventbus.NewRoot(typ, contracts.SourceGateway, s.WorldID, nil, s.ActorKind, payload)
}

func participants(ids []string) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}})
	}
	return out
}

// Timestamp is the wire form of a time of analytics: RFC 3339 in UTC.
func Timestamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
