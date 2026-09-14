package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	"multiverse-core.io/shared/entity"
)

// ErrPersistFailed is why a world stops when the object store refused an
// entity of an answer after every attempt: the entity was not written, no fact
// of the answer is published, and nothing more of the world is decided until a
// restart (state-and-mechanics.md §4.5 p. 11, §9).
var ErrPersistFailed = errors.New("state: persist_failed")

// persistPauses are the pauses between the attempts of one write: three more
// attempts after the first (§4.5 p. 11).
var persistPauses = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond, 900 * time.Millisecond}

// written is one entity of an answer on its way to the object store, with the
// version it had before the proposal.
type written struct {
	entity      *entity.Entity
	fromVersion int64
}

// persist writes the entities of an answer before any of its facts goes out
// (C-02 "Гарантии", ADR-011 p. 1): the intent first when the package is atomic
// and changes more than one entity (ADR-013 p. 4), then every entity in
// ascending identifier order, then the intent is removed. The entities are
// written with an empty fact_event_id — the fact is not out yet — and learn
// it in memory after the publication (§4.5 p. 12).
//
// The writes do not see the cancellation of ctx: a Stop that comes while an
// entity is being written lets the write finish (C-01 v1.7, ADR-023 p. 4). ctx
// ends the pauses between attempts only.
func (a *Applier) persist(ctx context.Context, p *Proposal, entities []written) error {
	if a.objects == nil || len(entities) == 0 {
		return nil
	}
	var intent *Intent
	if p.Atomic && len(entities) > 1 {
		intent = newIntent(p, a.worldID, entities)
		if err := a.withAttempts(ctx, p, "put the intent", func(out context.Context) error {
			return a.objects.PutIntent(out, a.worldID, intent)
		}); err != nil {
			return a.persistFailed(p, "the intent", err)
		}
	}
	for _, w := range entities {
		e := w.entity
		if err := a.withAttempts(ctx, p, "put "+e.ID, func(out context.Context) error {
			return a.objects.PutEntity(out, a.worldID, e)
		}); err != nil {
			return a.persistFailed(p, e.Ref().String(), err)
		}
	}
	if intent != nil {
		// Every entity is written, so a stale intent is harmless: the roll
		// forward of a restart skips the entities already at their to_version
		// (ADR-013 p. 4). The answer goes out either way.
		if err := a.withAttempts(ctx, p, "delete the intent", func(out context.Context) error {
			return a.objects.DeleteIntent(out, a.worldID, p.ID)
		}); err != nil {
			a.log.Error("the intent of a written package was not removed; a restart rolls it forward as done",
				"proposal_id", p.ID, "err", err, "handled", true)
		}
	}
	return nil
}

// withAttempts runs one write until it succeeds or its attempts are spent. It
// returns the last error, or the error of ctx when ctx ended a pause.
func (a *Applier) withAttempts(ctx context.Context, p *Proposal, what string, write func(context.Context) error) error {
	out := context.WithoutCancel(ctx)
	for attempt := 0; ; attempt++ {
		err := write(out)
		if err == nil {
			return nil
		}
		if attempt == len(persistPauses) {
			return err
		}
		pause := persistPauses[attempt]
		a.log.Warn("a write to the object store failed; it is tried again",
			"proposal_id", p.ID, "write", what, "attempt", attempt+1, "pause", pause, "err", err, "handled", true)
		if waitErr := a.wait(ctx, pause); waitErr != nil {
			return errors.Join(err, waitErr)
		}
	}
}

// persistFailed stops the world whose answer could not be written.
func (a *Applier) persistFailed(p *Proposal, what string, cause error) error {
	failure := fmt.Errorf("%w: %w: %s of proposal %s was not written: %w",
		ErrWorldStopped, ErrPersistFailed, what, p.ID, cause)
	a.halt(failure)
	a.log.Error("the object store refused an answer; the world is stopped",
		"reason", "persist_failed", "proposal_id", p.ID, "event_id", p.Event.ID, "write", what,
		"err", cause, "handled", false,
		"remedy", "bring the object store back and restart the process")
	return failure
}

// newIntent is the intent of a package: every entity with the version it
// leaves and the one it reaches (ADR-013 p. 4, §4.7).
func newIntent(p *Proposal, worldID string, entities []written) *Intent {
	in := &Intent{
		ProposalID:      p.ID,
		ProposalEventID: p.Event.ID,
		World:           worldID,
		Cause:           p.Cause,
		Changes:         make([]IntentChange, 0, len(entities)),
	}
	for _, w := range entities {
		e := w.entity
		var changed []entity.Change
		if e.LastChange != nil {
			changed = e.LastChange.Changed
		}
		in.Changes = append(in.Changes, IntentChange{
			Ref:             e.Ref(),
			FromVersion:     w.fromVersion,
			ToVersion:       e.Version,
			AttributesAfter: e.Attributes,
			Changed:         changed,
		})
	}
	return in
}
