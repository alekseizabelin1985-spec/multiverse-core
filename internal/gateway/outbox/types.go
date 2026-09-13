// Package outbox is the queue of deliveries to the players (component
// gateway-and-bot.md §8, ADR-006, C-08): what the consumer turns events into,
// what a client takes by long-poll and acknowledges.
//
// A delivery is a row of deliveries in gateway.db. It is enqueued in the
// transaction of the consumer that handles its event, taken by one client at a
// time under a lease, one delivery per player in lease, in the order of seq,
// and done when the client that leased it acknowledges it. The route to the
// messenger is never stored: it comes from links.db when the delivery is given
// out, and only to a client of the platform of the link (SEC-12).
package outbox

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Kinds of a delivery (component §4.2, api-contracts.md §1.5).
const (
	KindAck        = "ack"
	KindMechanics  = "mechanics"
	KindNarrative  = "narrative"
	KindWorldEvent = "world_event"
	KindGroup      = "group"
	KindSystem     = "system"
)

// States of a delivery.
const (
	StatePending   = "pending"
	StateDelivered = "delivered"
	StateDropped   = "dropped"
)

// Generators of the text of a delivery.
const (
	GeneratedByRules    = "rules"
	GeneratedByLLM      = "llm"
	GeneratedByTemplate = "template"
)

// Limits and defaults of component §8 and C-08 v1.1.
const (
	// DefaultLease is MV_GATEWAY_DELIVERY_LEASE by default: a delivery given
	// out and not acknowledged is given again after it.
	DefaultLease = 30 * time.Second
	// DefaultTTL is MV_GATEWAY_DELIVERY_TTL by default: a delivery still
	// pending after it is dropped.
	DefaultTTL = 24 * time.Hour
	// MaxWait and MaxLimit bound wait_ms and limit of the long-poll.
	MaxWait  = 25 * time.Second
	MaxLimit = 100
	// WakeEvery is the periodic wake-up of a waiting long-poll, the safety net
	// for a notification that was lost (component §8.3).
	WakeEvery = time.Second
	// InflightPerPlayer is MV_GATEWAY_OUTBOX_INFLIGHT_PER_PLAYER, a constant in
	// MVP-1 (component §8.2): one delivery in lease per player.
	InflightPerPlayer = 1
)

// Delivery is one message for one player (component §6).
type Delivery struct {
	ID             string
	Seq            int64
	WorldID        string
	PlayerID       string
	Platform       string
	Kind           string
	CorrelationID  string
	EventID        string
	RoundSeq       *int
	GeneratedBy    string
	FallbackReason *string
	Text           string
	Data           map[string]any
	State          string
	Attempts       int
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// DB is what the outbox writes through: the database, or the transaction of
// the consumer, which holds the only connection of gateway.db.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// idSpace is the namespace of the ids of deliveries.
var idSpace = uuid.MustParse("5b0e7c52-7f4e-4d2b-9a44-4b1f0f3c6a70")

// EventDeliveryID is the id of the delivery of an event to a player. It is
// derived from what the delivery is, not drawn from a generator: a repeat of
// the same event gives the same id and the unique id refuses the second row
// (NFR-013), and a delivery does not move the sequence of --id-source=sequence
// the link_id and player_id are drawn from.
func EventDeliveryID(eventID, playerID, kind string) string {
	return deliveryID("event", eventID, playerID, kind)
}

// TransitionDeliveryID is the id of the delivery of a transition of an
// encounter to a player. Two events can open an encounter and two can end it,
// and after a restart the second one of a pair reports the transition again
// (readmodel.Result): the id names the encounter and the transition, not the
// event, so the second report finds the first row.
func TransitionDeliveryID(encounterID, transition, playerID string) string {
	return deliveryID("encounter", encounterID, transition, playerID)
}

// deliveryID encodes every component with its length in front of it, so that
// no separator inside a component can make two keys collide.
func deliveryID(parts ...string) string {
	var name strings.Builder
	for _, p := range parts {
		name.WriteString(strconv.Itoa(len(p)))
		name.WriteByte(':')
		name.WriteString(p)
	}
	return "d-" + uuid.NewSHA1(idSpace, []byte(name.String())).String()
}

// timeLayout keeps a fixed width, so that two times compare as text the way
// they compare as times (as in the other tables of gateway.db).
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
