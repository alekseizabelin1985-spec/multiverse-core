package store

import (
	"context"
	"time"
)

// Retention of the gateway tables (component gateway-and-bot.md section 4.2,
// ADR-019 p. 7), and the compaction safety net of links.db. sessions, turns and
// rounds are not cleaned in MVP-1.
const (
	// SweepInterval is how often the gateway runs its sweepers.
	SweepInterval = time.Minute
	// KeyTTL bounds idempotency_keys and character_requests by expires_at.
	KeyTTL = 24 * time.Hour
	// FinishedDeliveryKeep is how long delivered and dropped deliveries stay;
	// a pending delivery past expires_at is dropped first.
	FinishedDeliveryKeep = 7 * 24 * time.Hour
	// ProcessedEventsKeep and ProcessedEventsMax bound the deduplication
	// window of the consumer: older rows or rows beyond the count go.
	ProcessedEventsKeep = 24 * time.Hour
	ProcessedEventsMax  = 50_000
	// LinksCompactInterval is the safety net after /forget: CompactLinks runs
	// again in case the process stopped between the DELETE and the compaction
	// (ADR-019 p. 5 and addendum p. 1).
	LinksCompactInterval = time.Hour
)

// Sweeper is the housekeeping of one owner of tables. The packages that own
// the tables implement it (links, actions, outbox, consumer); the gateway
// context calls every Sweeper once per SweepInterval with the time of
// shared/clock, and never in replay mode (component section 11.2). Nothing
// runs a Sweeper yet.
type Sweeper interface {
	// Sweep deletes or drops what is past retention at now.
	Sweep(ctx context.Context, now time.Time) error
}
