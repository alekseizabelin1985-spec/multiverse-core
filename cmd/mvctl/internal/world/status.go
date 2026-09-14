package world

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

// statusTimeout bounds the reads of `mvctl world status`.
const statusTimeout = 30 * time.Second

// StatusResult is what `mvctl world status` read: the details of its JSON
// report.
type StatusResult struct {
	World    string              `json:"world"`
	Store    string              `json:"store"`
	Snapshot *state.SnapshotMeta `json:"snapshot,omitempty"`
}

// runStatus implements `mvctl world status`: the snapshot latest.json points at,
// read the way a consumer of the read-model reads it — the pointer, the object
// under the key derived from its instant and sequence, and the state_hash
// recomputed over the entities of the object (§4.4).
func (c Command) runStatus(args []string, stdout, stderr io.Writer) int {
	const command = "world status"
	flags := cli.FlagSet("mvctl "+command, stderr)
	worldID := flags.String("world", env.WorldID.String(), "world to read (default "+env.WorldID.Name()+")")
	store := flags.String("store", StoreMinIO,
		"object store: minio, or memory, which holds only what this command wrote: "+
			"a world init over the memory store is gone when that command ends")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	if code, ok := noArguments(stderr, command, flags.Args()); !ok {
		return code
	}
	if *worldID == "" {
		return usage(stderr, command, "no world: pass --world or set %s", env.WorldID.Name())
	}
	if *store != StoreMemory && *store != StoreMinIO {
		return usage(stderr, command, "unknown store %q, expected %s or %s", *store, StoreMinIO, StoreMemory)
	}

	report := cli.NewReport(command)
	result := StatusResult{World: *worldID, Store: *store}
	report.Details = &result
	client, err := c.OpenStore(*store)
	if err != nil {
		report.Add(CheckStore, *store, err.Error())
		return report.Write(stdout, stderr, *asJSON)
	}
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	objects := state.NewObjectStore(client)
	pointer, err := objects.ReadLatest(ctx, *worldID)
	// A world nobody initialized has no latest.json, and on a store nobody
	// prepared for it not even the bucket.
	if errors.Is(err, state.ErrNoSnapshot) || errors.Is(err, objstore.ErrNoBucket) {
		// A world in the store of a deployment is created only through the
		// State of the running core (§4.10): the in-process path writes the
		// memory store and nothing else.
		hint := "run mvctl world init --bus kafka while core is running"
		if *store == StoreMemory {
			// The memory store is born empty with every command: a world init
			// over it is not seen here, and the answer must not suggest it was lost.
			hint = "the memory store holds only what this command wrote, and world status writes nothing"
		}
		report.Addf(CheckWorld, *worldID, "not initialized: %s/%s is not there; %s",
			objstore.SnapshotsBucket(*worldID), state.PointerKey, hint)
		return report.Write(stdout, stderr, *asJSON)
	}
	if err != nil {
		report.Add(CheckStore, *store, err.Error())
		return report.Write(stdout, stderr, *asJSON)
	}
	meta := pointer.Snapshot
	result.Snapshot = &meta
	for _, finding := range verify(ctx, objects, *worldID, pointer) {
		report.Add(CheckSnapshot, meta.Key, finding)
	}

	report.Linef("world %s: snapshot seq %d, reason %s, taken_at %s", *worldID, meta.Seq, meta.Reason,
		meta.TakenAt.UTC().Format(time.RFC3339))
	report.Linef("state_hash: %s", meta.StateHash)
	report.Linef("entities_count: %d", meta.EntitiesCount)
	report.Linef("rules_version: %s", meta.RulesVersion)
	report.Linef("laws_version: %s", meta.LawsVersion)
	report.Linef("cursor.%s: %d", eventbus.TopicSystemEvents, meta.Cursor[eventbus.TopicSystemEvents])
	if report.OK() {
		report.Summary = "world " + *worldID + " stands on snapshot " + meta.ID
	}
	return report.Write(stdout, stderr, *asJSON)
}

// verify is what disagrees between the pointer and the object it points at.
func verify(ctx context.Context, objects state.Store, worldID string, pointer *state.LatestPointer) []string {
	meta := pointer.Snapshot
	var findings []string
	if want := state.SnapshotKey(meta.TakenAt, meta.Seq); meta.Key != want {
		findings = append(findings, fmt.Sprintf("the key does not follow from taken_at and seq: want %s", want))
	}
	snap, err := objects.ReadSnapshot(ctx, worldID, meta.Key)
	if err != nil {
		return append(findings, fmt.Sprintf("the object the pointer names cannot be read: %v", err))
	}
	if got := entity.StateHash(snap.Entities); got != meta.StateHash {
		findings = append(findings, fmt.Sprintf("state_hash of the entities is %s, the pointer says %s", got, meta.StateHash))
	}
	if len(snap.Entities) != meta.EntitiesCount {
		findings = append(findings, fmt.Sprintf("the object holds %d entities, the pointer says %d",
			len(snap.Entities), meta.EntitiesCount))
	}
	if snap.Snapshot.ID != meta.ID || snap.Snapshot.StateHash != meta.StateHash {
		findings = append(findings, fmt.Sprintf("the object is snapshot %s with state_hash %s, not the one the pointer names",
			snap.Snapshot.ID, snap.Snapshot.StateHash))
	}
	return findings
}
