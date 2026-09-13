package replay

import (
	"context"
	"fmt"

	"multiverse-core.io/shared/eventbus"
)

// ReadToEnd delivers the events of topic from offset from up to the end the
// journal reports at the moment of the call, and returns the offset to
// continue from. Events published during the read are left to the next call
// or to a Tail: "the end of the journal" is the End taken once (C-01 v1.1),
// otherwise a busy topic would never let a catch-up finish.
func ReadToEnd(ctx context.Context, j eventbus.Journal, topic string, from int64, h eventbus.Handler) (int64, error) {
	end, err := j.End(ctx, topic)
	if err != nil {
		return from, fmt.Errorf("replay: end of %s: %w", topic, err)
	}
	if end <= from {
		return from, nil
	}
	next, err := j.ReadRange(ctx, topic, from, end, h)
	if err != nil {
		return next, fmt.Errorf("replay: read %s [%d, %d): %w", topic, from, end, err)
	}
	return next, nil
}

// CatchUp reads every topic of the cursor to its end, in the sorted order of
// Cursor.Topics, and returns the cursor after the read. The cursor passed in is
// not changed. On an error the returned cursor keeps the progress made so far,
// so that a caller may record it and resume from there.
func CatchUp(ctx context.Context, j eventbus.Journal, c Cursor, h eventbus.Handler) (Cursor, error) {
	out := c.Clone()
	for _, topic := range c.Topics() {
		next, err := ReadToEnd(ctx, j, topic, c[topic], h)
		if next > out[topic] {
			out[topic] = next
		}
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
