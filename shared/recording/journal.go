package recording

import (
	"context"
	"fmt"

	"multiverse-core.io/shared/eventbus"
)

// ReadJournal reads the events of topic from offset from up to the end the
// journal reports when the call starts, and returns them as a recording
// (C-01 v1.9, "Чтение журнала"). The end is taken once, before the read: an
// event published during the read is not part of the recording, otherwise a
// busy topic would never let the read finish. With the end at or before from
// the recording is empty and there is no error.
//
// The handler handed to the journal only keeps the event and never fails. A
// consumer parses the recording after the read: a handler that refused a
// record would send it into the retries and dead_letters of the bus, and the
// record would silently drop out of whatever the consumer indexes.
//
// A read that ends short of the end without an error — the way membus answers
// a Close — or under a cancelled ctx is an error naming where it stopped: a
// partial recording passed off as a whole one would miss the records a replay
// looks up.
//
// A read of ReadJournal is a read of history, not a delivery (C-01 v1.11): the
// events come out as the journal holds them in every mode of the process. The
// journal is read under a context marked for InReadJournal, and the replay
// middleware hands a handler under that mark on unchanged — it neither moves
// the replay clock nor sets meta.replay. Validation on read, the retries and
// dead_letters of the bus stay: the mark does not switch the delivery off.
func ReadJournal(ctx context.Context, j eventbus.Journal, topic string, from int64) (*Recording, error) {
	end, err := j.End(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("recording: journal %s: end: %w", topic, err)
	}
	rec := &Recording{}
	if end <= from {
		return rec, nil
	}
	next, err := j.ReadRange(context.WithValue(ctx, inReadJournal{}, true), topic, from, end, func(_ context.Context, ev eventbus.Event) error {
		rec.events = append(rec.events, ev)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("recording: journal %s read stopped at %d of %d: %w", topic, next, end, err)
	}
	if cerr := ctx.Err(); cerr != nil {
		return nil, fmt.Errorf("recording: journal %s read stopped at %d of %d: %w", topic, next, end, cerr)
	}
	if next < end {
		return nil, fmt.Errorf("recording: journal %s read stopped at %d of %d", topic, next, end)
	}
	return rec, nil
}

// inReadJournal is the key of the mark ReadJournal puts on the context of its
// read. It is unexported, so nothing outside the package can forge the mark.
type inReadJournal struct{}

// InReadJournal reports whether ctx is the context of a handler that
// ReadJournal handed to the journal, or one derived from it (C-01 v1.11). The
// replay middleware asks it to tell a read of history from a delivery: every
// other read with a handler — Subscribe, Tail, ReadRange with a handler of the
// caller, replay.CatchUp — is a delivery and runs under the replay clock. The
// context passed to ReadJournal itself is not marked, before or after the call.
func InReadJournal(ctx context.Context) bool {
	marked, _ := ctx.Value(inReadJournal{}).(bool)
	return marked
}
