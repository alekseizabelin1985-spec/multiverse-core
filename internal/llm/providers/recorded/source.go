package recorded

import (
	"context"
	"iter"

	"multiverse-core.io/shared/eventbus"
)

// Source hands the events of a recording to h, in recording order. The
// provider does not read a store itself: the format, the reading and the index
// of a recording belong to one package outside the providers (T-457), and the
// process wiring passes a Source built over it.
//
// The contract of a Source is stricter than that of a subscription, and it is
// what New relies on:
//   - every event of the recording is handed over, or the Source fails;
//     a read that stops short — at the end of a range it did not reach, on a
//     shutdown — is an error, not a success;
//   - the first error of h is returned as it is and ends the reading: it is
//     not retried, not backed off and not parked in dead_letters;
//   - the error of the reading itself is returned too.
//
// eventbus.Journal.ReadRange has the same signature and does not keep that
// contract: an error of its handler goes the way of a delivery (retries, then
// dead_letters, then nil to the caller), and membus returns a short read on a
// shutdown without an error. Passed to New directly it would leave a partial
// index without an error. An adapter over a journal therefore collects the
// events of the range first, checks that the whole range was read, and only
// then hands them over — with Events or Slice.
type Source func(ctx context.Context, h eventbus.Handler) error

// Events is a source over a sequence of events — for instance the events of a
// recording another package has already read. The source pulls seq on every
// reading: over a one-shot sequence it can be read once, which is what New and
// Factory do.
func Events(seq iter.Seq[eventbus.Event]) Source {
	return func(ctx context.Context, h eventbus.Handler) error {
		for ev := range seq {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := h(ctx, ev); err != nil {
				return err
			}
		}
		return nil
	}
}

// Slice is a source over events held in memory.
func Slice(events ...eventbus.Event) Source {
	return Events(func(yield func(eventbus.Event) bool) {
		for _, ev := range events {
			if !yield(ev) {
				return
			}
		}
	})
}
