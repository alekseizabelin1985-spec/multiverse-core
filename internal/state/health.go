package state

import (
	"errors"
	"math"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/runtime"
)

// WorldHealth is the section of one world in /health of State
// (state-and-mechanics.md §10 with §18 p. 2, §9): details.worlds.<id>. The
// shape of runtime.Status is the process's and is not changed here; this is
// what State puts into its details.
//
//   - entities, seq and state_hash of the world in memory, rules_version, the
//     cursor of system_events and pending_intents — always;
//   - snapshot {seq, taken_at, age_s}: the last snapshot written, or the one the
//     world was rebuilt from; age_s is whole seconds on the clock of the world;
//   - fail, with reason ∈ panic, persist_failed, publish_failed,
//     snapshot_corrupted, state_divergence, when the world is stopped; a
//     corrupted snapshot also says snapshot: corrupted (§9);
//   - degraded while an answer is published again (publish_attempts_failed),
//     while the world waits for its init (world: uninitialized, §18), after a
//     snapshot attempt failed (snapshot_stale: the age of the last successful
//     snapshot in seconds, null without one — the text of the failure goes to
//     the log only), while the snapshot.created of the last snapshot did not go
//     out (snapshot_event_failed), and after a recovery without the journal
//     (log_gap) or without a snapshot (no_snapshot) until a snapshot is written.
func (a *Applier) WorldHealth() (string, map[string]any) {
	a.mu.Lock()
	last, snapshotErr, eventErr := a.lastSnapshot, a.snapshotErr, a.snapshotEventErr
	rec, sinceRecovery, failure, attempts := a.recovery, a.snapshotSinceRecovery, a.failure, a.retrying
	a.mu.Unlock()

	entities := a.store.List(a.worldID)
	section := map[string]any{
		"entities":        len(entities),
		"state_hash":      entity.StateHash(entities),
		"rules_version":   a.rulesVersion,
		"cursor":          a.cursor.Load(),
		"pending_intents": a.pendingIntents.Load(),
	}
	now := a.clock.Now()
	if last != nil {
		section["seq"] = last.Snapshot.Seq
		section["snapshot"] = map[string]any{
			"seq": last.Snapshot.Seq, "taken_at": last.Snapshot.TakenAt, "age_s": ageSeconds(now, last.Snapshot.TakenAt),
		}
	}

	if failure != nil {
		section["status"], section["err"], section["reason"] = runtime.StatusFail, failure.Error(), stopReason(failure)
		if errors.Is(failure, ErrSnapshotCorrupted) {
			section["snapshot"] = "corrupted"
		}
		return runtime.StatusFail, section
	}
	status := runtime.StatusOK
	degrade := func(key string, value any) {
		section[key] = value
		status = runtime.StatusDegraded
	}
	if attempts > 0 {
		degrade("publish_attempts_failed", attempts)
	}
	if a.uninitialized.Load() {
		degrade("world", "uninitialized")
	}
	if snapshotErr != nil {
		var age any
		if last != nil {
			age = ageSeconds(now, last.Snapshot.TakenAt)
		}
		degrade("snapshot_stale", age)
	}
	if eventErr != nil {
		degrade("snapshot_event_failed", true)
	}
	if !sinceRecovery {
		if rec.LogGap {
			degrade("log_gap", true)
		}
		if rec.NoSnapshot {
			degrade("no_snapshot", true)
		}
	}
	section["status"] = status
	return status, section
}

// ageSeconds is the whole seconds from then to now, never below zero.
func ageSeconds(now, then time.Time) int64 {
	return int64(math.Max(0, math.Floor(now.Sub(then).Seconds())))
}

// stopReason names why a world stopped, for /health.
func stopReason(err error) string {
	switch {
	case errors.Is(err, ErrPublishFailed):
		return "publish_failed"
	case errors.Is(err, ErrPersistFailed):
		return "persist_failed"
	case errors.Is(err, errPanicked):
		return "panic"
	case errors.Is(err, ErrSnapshotCorrupted):
		return "snapshot_corrupted"
	case errors.Is(err, ErrStateDivergence):
		return "state_divergence"
	default:
		return "stopped"
	}
}
