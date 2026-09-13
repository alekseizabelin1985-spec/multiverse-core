package replay

import (
	"sort"

	"multiverse-core.io/shared/eventbus"
)

// Cursor is where a reader stands in the journal: topic → offset of the next
// unread event (C-14, snapshot.cursor). A cursor names only the topics its
// reader reads. It marshals as a JSON object, whose keys encoding/json sorts,
// so the same cursor gives the same bytes in a snapshot.
type Cursor map[string]int64

// Advance records that the event at pos has been handled: the topic moves to
// pos.Offset+1 unless it already stands further. A redelivered older event
// does not move the cursor back.
func (c Cursor) Advance(pos eventbus.Position) {
	if next := pos.Offset + 1; next > c[pos.Topic] {
		c[pos.Topic] = next
	}
}

// Clone returns an independent copy.
func (c Cursor) Clone() Cursor {
	out := make(Cursor, len(c))
	for topic, offset := range c {
		out[topic] = offset
	}
	return out
}

// Merge returns the cursor that has read what either of the two has read: the
// union of the topics, the further offset of each.
func (c Cursor) Merge(o Cursor) Cursor {
	out := c.Clone()
	for topic, offset := range o {
		if cur, ok := out[topic]; !ok || offset > cur {
			out[topic] = offset
		}
	}
	return out
}

// Min returns the cursor a reader resuming for both of them starts from: the
// union of the topics, the nearer offset of each. A topic only one of them
// names is taken from that one — the other does not read it.
func (c Cursor) Min(o Cursor) Cursor {
	out := c.Clone()
	for topic, offset := range o {
		if cur, ok := out[topic]; !ok || offset < cur {
			out[topic] = offset
		}
	}
	return out
}

// Topics returns the topics of the cursor in sorted order, the order a catch-up
// reads them in, so that it is the same on every run.
func (c Cursor) Topics() []string {
	topics := make([]string, 0, len(c))
	for topic := range c {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	return topics
}
