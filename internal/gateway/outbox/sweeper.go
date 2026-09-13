package outbox

import (
	"context"
	"errors"
	"time"

	"multiverse-core.io/internal/gateway/store"
)

var _ store.Sweeper = (*Store)(nil)

// Sweep is the housekeeping of the outbox at now (component §4.2, §8.4): the
// leases that ran out are released and their deliveries given out again, the
// pending deliveries past their TTL are dropped, and the delivered and dropped
// ones older than store.FinishedDeliveryKeep are deleted. Every step runs even
// when an earlier one failed. The gateway context calls it once per
// store.SweepInterval and never in replay mode.
func (s *Store) Sweep(ctx context.Context, now time.Time) error {
	_, released := s.ReleaseExpiredLeases(ctx, now)
	_, expired := s.Expire(ctx, now)
	_, purged := s.Purge(ctx, now)
	return errors.Join(released, expired, purged)
}
