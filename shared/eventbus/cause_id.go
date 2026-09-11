package eventbus

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// CauseIDNamespace is the UUID namespace of identifiers derived from a cause
// (C-01 v1.4, ADR-027 p. 1). It is itself the UUIDv5 of the URL
// https://multiverse-core.io/eventbus/cause-id in the standard URL namespace,
// so that it can be reproduced rather than trusted.
//
// Changing it changes the id of every derived event, and a journal recorded
// before the change would no longer replay to the same bytes (NFR-061).
const CauseIDNamespace = "46a1a952-5297-584b-9430-d9895b6db112"

var causeIDSpace = uuid.MustParse(CauseIDNamespace)

// WithCauseID derives the identifier of the event from its cause instead of
// taking it from the process-wide generator: the UUIDv5 of the causation id,
// the event type and the parts. The same cause, type and parts give the same
// id on a retried publish, after a restart and in replay, whatever generator
// the process installed (C-01 v1.4, ADR-027).
//
// The option is only for an event its publisher emits at most once per
// (cause, type, parts). The parts must tell apart every event the publisher
// may emit for one cause and type — the kind of narrative, the encounter id,
// the entity id, the index of a dice roll — or two different events get one
// id and consumers drop the second one as a duplicate (ADR-027 p. 2).
//
// The option panics when the event has no cause to derive from: on NewRoot,
// and on Derive from an envelope without an id. On NewRoot it is a programmer
// error, better seen in the first test than silently in play. An envelope
// without an id fails the envelope check, so with validation on read — the
// default — it goes to dead_letters and never reaches a handler; with
// MV_BUS_VALIDATE_ON_READ=false it does, and the panic escapes Deliver. Nothing
// recovers it, so it takes down the process with every context in it. A
// handler that may run with validation off checks ev.ID before deriving.
func WithCauseID(parts ...string) DeriveOption {
	// Cloned so that a caller reusing its slice cannot change the id of an
	// event built later with the same option value.
	parts = slices.Clone(parts)
	return func(e *Event) {
		if e.Meta.CausationID == "" {
			panic(fmt.Sprintf("eventbus: WithCauseID on %q, an event without a cause: a root event has no cause to derive its id from", e.Type))
		}
		e.ID = causeID(e.Meta.CausationID, e.Type, parts)
	}
}

// causeID encodes every component with its length in front of it. A plain
// separator would not do: the parts are arbitrary strings, and any separator
// they may contain would let ("a|b") and ("a", "b") collide.
func causeID(causationID, typ string, parts []string) string {
	var name strings.Builder
	for _, component := range append([]string{causationID, typ}, parts...) {
		name.WriteString(strconv.Itoa(len(component)))
		name.WriteByte(':')
		name.WriteString(component)
	}
	return uuid.NewSHA1(causeIDSpace, []byte(name.String())).String()
}
