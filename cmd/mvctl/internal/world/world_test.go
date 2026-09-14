package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

const world = "dark-forest-world"

// testEpoch is an instant of the tests, for the key of a snapshot object.
var testEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// The fixtures and the rule book of the tree, relative to this package.
var (
	fixturesDir = filepath.Join("..", "..", "..", "..", "testdata", "fixtures")
	rulesBook   = filepath.Join("..", "..", "..", "..", "rules", "dark-forest.yaml")
)

// stand is `mvctl world` over one memory object store that outlives a run, so
// that a second run sees what the first one wrote, and counts how often a store
// was opened.
type stand struct {
	objects *objstore.Memory
	opened  int
	// wrap, when set, puts a store of the test in front of the memory store.
	wrap func(objstore.Client) objstore.Client
}

func newStand() *stand { return &stand{objects: objstore.NewMemory()} }

func (s *stand) command() Command {
	return Command{OpenStore: func(kind string) (objstore.Client, error) {
		s.opened++
		if kind != StoreMemory {
			return nil, fmt.Errorf("the test opens the memory store only, not %q", kind)
		}
		if s.wrap != nil {
			return s.wrap(s.objects), nil
		}
		return s.objects, nil
	}}
}

type run struct {
	code           int
	stdout, stderr string
}

func (s *stand) run(args ...string) run {
	var stdout, stderr bytes.Buffer
	code := s.command().Run(args, &stdout, &stderr)
	return run{code, stdout.String(), stderr.String()}
}

func (s *stand) init(extra ...string) run {
	args := append([]string{"init", "--world", world, "--fixtures", fixturesDir, "--bus", BusMemory,
		"--store", StoreMemory, "--rules", rulesBook}, extra...)
	return s.run(args...)
}

func (s *stand) snapshots(t *testing.T) []state.SnapshotRef {
	t.Helper()
	refs, err := state.NewObjectStore(s.objects).ListSnapshots(context.Background(), world)
	if err != nil {
		t.Fatalf("snapshots: %v", err)
	}
	return refs
}

func (s *stand) latest(t *testing.T) *state.LatestPointer {
	t.Helper()
	pointer, err := state.NewObjectStore(s.objects).ReadLatest(context.Background(), world)
	if err != nil {
		t.Fatalf("latest.json: %v", err)
	}
	return pointer
}

// fixtureHash is the state_hash of the fixture pointer of seq 0: the world of
// the fixtures, however it came to be.
func fixtureHash(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fixturesDir, "snapshots", "state", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pointer state.LatestPointer
	if err := json.Unmarshal(body, &pointer); err != nil {
		t.Fatal(err)
	}
	return pointer.Snapshot.StateHash
}

// The DoD line of T-058: `mvctl world init --bus memory` creates the six
// entities and the snapshot seq 0 of reason bootstrap, and prints entities_count
// and state_hash.
func TestInitCreatesTheWorldAndItsSnapshot(t *testing.T) {
	s := newStand()
	r := s.init()
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	for _, want := range []string{"entities_count: 6", "state_hash: " + fixtureHash(t), "rules_version: 0.1",
		"seq 0, reason bootstrap", "store: memory (its objects are gone when the command ends)"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout does not say %q:\n%s", want, r.stdout)
		}
	}

	// One snapshot and no other: the in-process State wrote no shutdown
	// snapshot over the bootstrap one.
	refs := s.snapshots(t)
	if len(refs) != 1 || refs[0].Seq != 0 {
		t.Fatalf("snapshot objects %v, want the one of seq 0", refs)
	}
	meta := s.latest(t).Snapshot
	if meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap || meta.EntitiesCount != 6 || meta.Key != refs[0].Key {
		t.Errorf("latest.json points at seq %d %s (%d entities, %s), want seq 0 bootstrap of 6 entities at %s",
			meta.Seq, meta.Reason, meta.EntitiesCount, meta.Key, refs[0].Key)
	}
	// Six proposals, each followed by its fact: the cursor stands past the last
	// proposal, on its fact, and not at 0 as in the fixture (acceptance of
	// T-057). The exact offset is checked against the journal in
	// internal/state (TestTheSnapshotOfABootstrapIsTheWorldOfTheFixtures).
	if got := meta.Cursor[eventbus.TopicSystemEvents]; got != 2*6-1 {
		t.Errorf("cursor.system_events %d, want %d", got, 2*6-1)
	}

	entities, err := state.NewObjectStore(s.objects).ListEntities(context.Background(), world)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 6 {
		t.Fatalf("%d entity objects, want 6", len(entities))
	}
	for _, e := range entities {
		if e.Version != 1 || e.LastChange == nil || e.LastChange.ProposalID != state.BootstrapProposalID(world, e.Ref()) {
			t.Errorf("%s: version %d, commit record %+v; want version 1 of its bootstrap proposal",
				e.ID, e.Version, e.LastChange)
		}
	}
	if got := entity.StateHash(entities); got != meta.StateHash {
		t.Errorf("the objects hash to %s, the snapshot says %s", got, meta.StateHash)
	}
	// The object of an entity is written before its fact goes out (T-057), so
	// only the snapshot, taken after, knows the id of every fact.
	snap, err := state.NewObjectStore(s.objects).ReadSnapshot(context.Background(), world, meta.Key)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range snap.Entities {
		if e.LastChange == nil || e.LastChange.FactEventID == "" {
			t.Errorf("%s in the snapshot: commit record %+v, want the id of its fact", e.ID, e.LastChange)
		}
	}
}

// A second run over an initialized world refuses with exit 2 and changes
// nothing (DoD of T-058, §4.10 p. 2).
func TestInitRefusesAWorldAlreadyInitialized(t *testing.T) {
	s := newStand()
	if r := s.init(); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	before := s.latest(t)

	r := s.init()
	if r.code != cli.ExitUsage {
		t.Fatalf("second run: exit %d, want %d\nstdout: %s\nstderr: %s", r.code, cli.ExitUsage, r.stdout, r.stderr)
	}
	for _, want := range []string{"is initialized", "--force", "latest.json"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not say %q: %s", want, r.stderr)
		}
	}
	if after := s.latest(t); after.Snapshot.ID != before.Snapshot.ID || len(s.snapshots(t)) != 1 {
		t.Errorf("the refused run wrote a snapshot: %s, %d objects", after.Snapshot.ID, len(s.snapshots(t)))
	}
}

// --force creates the world again (§4.10, "--force"): the objects of State of
// the world go first — an entity the fixtures do not describe, an intent, every
// snapshot of state/ — so the bootstrap snapshot is seq 0 again, while the
// snapshots of swarm/ are not State's and stay.
func TestInitWithForceOverAWorldWithSnapshots(t *testing.T) {
	s := newStand()
	if r := s.init(); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	ctx := context.Background()
	entities, snapshots := objstore.EntitiesBucket(world), objstore.SnapshotsBucket(world)
	for _, obj := range []struct{ bucket, key, body string }{
		{entities, "player/player-Z.json", `{}`},
		{entities, "_intents/bootstrap%3Adark-forest-world%3Anpc%2Fwolf-alpha.json", `{}`},
		{snapshots, state.SnapshotKey(testEpoch, 7), `{}`},
		{snapshots, "swarm/latest.json", `{"component":"swarm"}`},
	} {
		if _, err := s.objects.Put(ctx, obj.bucket, obj.key, []byte(obj.body), objstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	r := s.init("--force")
	if r.code != cli.ExitOK {
		t.Fatalf("--force: exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	refs := s.snapshots(t)
	if len(refs) != 1 || refs[0].Seq != 0 {
		t.Fatalf("snapshot objects %v, want the one of seq 0", refs)
	}
	meta := s.latest(t).Snapshot
	if meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap || meta.Key != refs[0].Key || meta.StateHash != fixtureHash(t) {
		t.Errorf("latest.json: seq %d, reason %s, key %s, state_hash %s; want seq 0 bootstrap of the fixture world at %s",
			meta.Seq, meta.Reason, meta.Key, meta.StateHash, refs[0].Key)
	}
	if !strings.Contains(r.stdout, "seq 0, reason bootstrap") {
		t.Errorf("stdout does not name seq 0: %s", r.stdout)
	}
	left, err := s.objects.List(ctx, entities, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 6 {
		t.Errorf("%d objects in %s, want the 6 entities of the fixtures: %v", len(left), entities, left)
	}
	for _, info := range left {
		if strings.HasPrefix(info.Key, "_intents/") || info.Key == "player/player-Z.json" {
			t.Errorf("%s/%s survived --force", entities, info.Key)
		}
	}
	if body, err := s.objects.Get(ctx, snapshots, "swarm/latest.json"); err != nil || string(body) != `{"component":"swarm"}` {
		t.Errorf("swarm/latest.json is %q (%v), want it untouched", body, err)
	}
}

// A clean-up cut off halfway leaves the pointer, removed last: the command
// reports the store and starts no State, and the next init without --force is
// refused as initialized rather than creating a world over what is left.
func TestInitWithForceCutOffKeepsThePointer(t *testing.T) {
	s := newStand()
	if r := s.init(); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	s.wrap = func(client objstore.Client) objstore.Client {
		return &refusingStore{Client: client, deleteRefused: func(bucket, key string) bool {
			return bucket == objstore.SnapshotsBucket(world) && strings.HasPrefix(key, state.Component+"/2")
		}}
	}
	r := s.init("--force")
	if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "[store]") {
		t.Fatalf("exit %d, stderr %q; want %d with a store finding", r.code, r.stderr, cli.ExitFindings)
	}
	if _, err := state.NewObjectStore(s.objects).ReadLatest(context.Background(), world); err != nil {
		t.Fatalf("latest.json after a clean-up cut off: %v", err)
	}
	s.wrap = nil
	if r := s.init(); r.code != cli.ExitUsage || !strings.Contains(r.stderr, "is initialized") {
		t.Errorf("the next init: exit %d, stderr %q; want %d refused as initialized", r.code, r.stderr, cli.ExitUsage)
	}
}

// A failure after latest.json is written is not the failure of a bootstrap
// (§4.10, exit codes: 1 is "the snapshot is not written"): the world is
// initialized, the command exits 0 and says what went wrong as a warning. Here
// the Stop of the in-process State fails with something other than the sealed
// store — its shutdown snapshot cannot list the snapshots once the pointer is
// there.
func TestInitWarnsOfAFailureAfterTheSnapshotIsWritten(t *testing.T) {
	s := newStand()
	refused := false
	s.wrap = func(client objstore.Client) objstore.Client {
		return &refusingStore{Client: client, listRefused: func(bucket string) bool {
			if bucket != objstore.SnapshotsBucket(world) {
				return false
			}
			if _, err := client.Stat(context.Background(), bucket, state.PointerKey); err != nil {
				return false
			}
			refused = true
			return true
		}}
	}
	r := s.init()
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, cli.ExitOK, r.stdout, r.stderr)
	}
	if !refused {
		t.Fatal("the store never refused a list: the test did not reach the failure it is about")
	}
	for _, want := range []string{"warning: the world is initialized, but", "stop the in-process state", "list refused"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not say %q: %s", want, r.stderr)
		}
	}
	if strings.Contains(r.stderr, "[bootstrap]") || strings.Contains(r.stderr, "[snapshot]") {
		t.Errorf("the warning is reported as a finding: %s", r.stderr)
	}
	if meta := s.latest(t).Snapshot; meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap {
		t.Errorf("latest.json: seq %d %s, want seq 0 bootstrap", meta.Seq, meta.Reason)
	}

	// The JSON report carries the warning too. A fresh store, with the same
	// refusal in front of it, so that the run is not refused as initialized.
	s.objects = objstore.NewMemory()
	r = s.init("--json")
	var report struct {
		Status  string     `json:"status"`
		Details InitResult `json:"details"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &report); err != nil {
		t.Fatalf("JSON report: %v\n%s", err, r.stdout)
	}
	if r.code != cli.ExitOK || report.Status != cli.StatusOK || len(report.Details.Warnings) == 0 || report.Details.Snapshot == nil {
		t.Errorf("--json: exit %d, report %+v; want ok with the warning and the snapshot", r.code, report)
	}
}

// refusingStore is an object store that refuses the calls its predicates name.
type refusingStore struct {
	objstore.Client
	deleteRefused func(bucket, key string) bool
	listRefused   func(bucket string) bool
}

func (s *refusingStore) Delete(ctx context.Context, bucket, key string) error {
	if s.deleteRefused != nil && s.deleteRefused(bucket, key) {
		return fmt.Errorf("delete refused: %s/%s", bucket, key)
	}
	return s.Client.Delete(ctx, bucket, key)
}

func (s *refusingStore) List(ctx context.Context, bucket, prefix string) ([]objstore.ObjectInfo, error) {
	if s.listRefused != nil && s.listRefused(bucket) {
		return nil, fmt.Errorf("list refused: %s", bucket)
	}
	return s.Client.List(ctx, bucket, prefix)
}

// --bus kafka waits for the admin route of T-059 (acceptance of T-057): it says
// so, and does nothing — no store opened, no proposal published.
func TestInitOverKafkaSaysThereIsNoAdminRoute(t *testing.T) {
	s := newStand()
	r := s.run("init", "--world", world, "--fixtures", fixturesDir, "--bus", BusKafka, "--store", StoreMemory)
	if r.code != cli.ExitUsage {
		t.Fatalf("exit %d, want %d\nstderr: %s", r.code, cli.ExitUsage, r.stderr)
	}
	for _, want := range []string{"admin route", "/v1/admin/state/{world}/snapshot", "T-059", "nothing was done"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not say %q: %s", want, r.stderr)
		}
	}
	if s.opened != 0 {
		t.Errorf("the store was opened %d times", s.opened)
	}
}

// A bootstrap State refuses leaves no latest.json behind — neither the
// bootstrap snapshot nor a shutdown snapshot of half a world — so the next init
// is not refused as initialized.
func TestInitOfFixturesThatBreakALawLeavesNoSnapshot(t *testing.T) {
	dir := t.TempDir()
	for _, file := range state.FixtureFiles {
		body, err := os.ReadFile(filepath.Join(fixturesDir, file.Name))
		if err != nil {
			t.Fatal(err)
		}
		if file.Type == entity.TypeNPC {
			body = bytes.Replace(body, []byte(`"position": "dark-forest-01"`), []byte(`"position": "nowhere"`), 1)
		}
		if err := os.WriteFile(filepath.Join(dir, file.Name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := newStand()
	r := s.run("init", "--world", world, "--fixtures", dir, "--bus", BusMemory, "--store", StoreMemory, "--rules", rulesBook)
	if r.code != cli.ExitFindings {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, cli.ExitFindings, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stderr, "[bootstrap]") || !strings.Contains(r.stderr, "law_violation") {
		t.Errorf("stderr does not report the refusal: %s", r.stderr)
	}
	if refs := s.snapshots(t); len(refs) != 0 {
		t.Errorf("snapshot objects %v after a failed bootstrap", refs)
	}
	if _, err := state.NewObjectStore(s.objects).ReadLatest(context.Background(), world); !errors.Is(err, state.ErrNoSnapshot) {
		t.Errorf("latest.json after a failed bootstrap: %v", err)
	}
}

func TestInitReportsWhatItCannotStartFrom(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nothing")
	cases := map[string]struct {
		args  []string
		check string
	}{
		"a rule book that is not there": {[]string{"--fixtures", fixturesDir, "--rules", missing}, "[rules]"},
		"fixtures that are not there":   {[]string{"--fixtures", missing, "--rules", rulesBook}, "[fixtures]"},
		"fixtures of another world": {[]string{"--world", "another-world", "--fixtures", fixturesDir,
			"--rules", rulesBook}, "[fixtures]"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStand()
			args := append([]string{"init", "--bus", BusMemory, "--store", StoreMemory}, tc.args...)
			r := s.run(args...)
			if r.code != cli.ExitFindings || !strings.Contains(r.stderr, tc.check) {
				t.Errorf("exit %d, stderr %q; want %d with %s", r.code, r.stderr, cli.ExitFindings, tc.check)
			}
			if s.opened != 0 {
				t.Errorf("the store was opened before the arguments were checked")
			}
		})
	}
	t.Run("a store that does not open", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cmd := Command{OpenStore: func(string) (objstore.Client, error) { return nil, errors.New("no credentials") }}
		code := cmd.Run([]string{"init", "--bus", BusMemory, "--fixtures", fixturesDir, "--rules", rulesBook}, &stdout, &stderr)
		if code != cli.ExitFindings || !strings.Contains(stderr.String(), "no credentials") {
			t.Errorf("exit %d, stderr %q", code, stderr.String())
		}
	})
}

func TestWorldRefusesAWrongCall(t *testing.T) {
	cases := map[string][]string{
		"no subcommand":      nil,
		"unknown subcommand": {"create"},
		"unknown bus":        {"init", "--bus", "nats"},
		"unknown store":      {"init", "--bus", BusMemory, "--store", "s3"},
		"memory bus, minio":  {"init", "--bus", BusMemory, "--store", StoreMinIO},
		"no world":           {"init", "--world", "", "--bus", BusMemory},
		"an argument":        {"init", "--bus", BusMemory, "extra"},
		"unknown flag":       {"init", "--nope"},
		"status: store":      {"status", "--store", "s3"},
		"status: no world":   {"status", "--world", ""},
		"status: argument":   {"status", "extra"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStand()
			if r := s.run(args...); r.code != cli.ExitUsage {
				t.Errorf("exit %d, want %d\nstderr: %s", r.code, cli.ExitUsage, r.stderr)
			}
			if s.opened != 0 {
				t.Errorf("the store was opened on a wrong call")
			}
		})
	}
	t.Run("help", func(t *testing.T) {
		if r := newStand().run("init", "-h"); r.code != cli.ExitOK {
			t.Errorf("exit %d, want %d", r.code, cli.ExitOK)
		}
	})
}

// The DoD line of T-058: `mvctl world status` prints state_hash, entities_count
// and rules_version of the snapshot the world stands on.
func TestStatusPrintsTheSnapshotOfTheWorld(t *testing.T) {
	s := newStand()
	if r := s.init(); r.code != cli.ExitOK {
		t.Fatalf("init: exit %d: %s", r.code, r.stderr)
	}
	r := s.run("status", "--world", world, "--store", StoreMemory)
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	for _, want := range []string{"state_hash: " + fixtureHash(t), "entities_count: 6", "rules_version: 0.1",
		"laws_version: v1", "reason bootstrap"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout does not say %q:\n%s", want, r.stdout)
		}
	}

	r = s.run("status", "--world", world, "--store", StoreMemory, "--json")
	var report struct {
		Status  string       `json:"status"`
		Details StatusResult `json:"details"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &report); err != nil {
		t.Fatalf("JSON report: %v\n%s", err, r.stdout)
	}
	if report.Status != cli.StatusOK || report.Details.Snapshot == nil ||
		report.Details.Snapshot.StateHash != fixtureHash(t) || report.Details.Snapshot.EntitiesCount != 6 {
		t.Errorf("JSON report %+v", report)
	}
}

func TestStatusOfAWorldNobodyInitialized(t *testing.T) {
	s := newStand()
	r := s.run("status", "--world", world, "--store", StoreMemory)
	if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "not initialized") ||
		!strings.Contains(r.stderr, "holds only what this command wrote") {
		t.Errorf("exit %d, stderr %q; want %d saying the world is not initialized", r.code, r.stderr, cli.ExitFindings)
	}
}

// status reads the snapshot the way a consumer of the read-model does, so a
// snapshot object that does not match its pointer is a finding.
func TestStatusFindsASnapshotThatDoesNotMatchItsPointer(t *testing.T) {
	cases := map[string]func(t *testing.T, s *stand, pointer *state.LatestPointer){
		"an entity changed in the object": func(t *testing.T, s *stand, pointer *state.LatestPointer) {
			bucket := objstore.SnapshotsBucket(world)
			body, err := s.objects.Get(context.Background(), bucket, pointer.Snapshot.Key)
			if err != nil {
				t.Fatal(err)
			}
			changed := bytes.Replace(body, []byte(`"hp_max": 10`), []byte(`"hp_max": 99`), 1)
			if bytes.Equal(changed, body) {
				t.Fatal("the object holds no hp_max to change")
			}
			if _, err := s.objects.Put(context.Background(), bucket, pointer.Snapshot.Key, changed, objstore.PutOptions{}); err != nil {
				t.Fatal(err)
			}
		},
		"the object gone": func(t *testing.T, s *stand, pointer *state.LatestPointer) {
			if err := s.objects.Delete(context.Background(), objstore.SnapshotsBucket(world), pointer.Snapshot.Key); err != nil {
				t.Fatal(err)
			}
		},
		"a count the object does not hold": func(t *testing.T, s *stand, pointer *state.LatestPointer) {
			pointer.Snapshot.EntitiesCount = 7
			rewritePointer(t, s, pointer)
		},
		"a key that does not follow from the instant": func(t *testing.T, s *stand, pointer *state.LatestPointer) {
			pointer.Snapshot.TakenAt = pointer.Snapshot.TakenAt.Add(3600e9)
			rewritePointer(t, s, pointer)
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStand()
			if r := s.init(); r.code != cli.ExitOK {
				t.Fatalf("init: exit %d: %s", r.code, r.stderr)
			}
			tamper(t, s, s.latest(t))
			r := s.run("status", "--world", world, "--store", StoreMemory)
			if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "[snapshot]") {
				t.Errorf("exit %d, stderr %q; want %d with a snapshot finding", r.code, r.stderr, cli.ExitFindings)
			}
		})
	}
}

func rewritePointer(t *testing.T, s *stand, pointer *state.LatestPointer) {
	t.Helper()
	body, err := json.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.objects.Put(context.Background(), objstore.SnapshotsBucket(world), state.PointerKey, body, objstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

// Only the refusals of the sealed store are the expected errors of the Stop of
// the in-process State; anything else it reports stays.
func TestUnexpectedKeepsWhatTheSealDidNotCause(t *testing.T) {
	sealed := fmt.Errorf("state: shutdown snapshot of %s: %w", world, errSealed)
	other := errors.New("state: stop the worker: deadline exceeded")
	manyWraps := fmt.Errorf("%w: %w: event e-1 did not go out after %d attempts", other, errors.ErrUnsupported, 3)
	cases := map[string]struct {
		err  error
		want error
	}{
		"nothing":                     {nil, nil},
		"the seal alone":              {sealed, nil},
		"the seal in a join":          {errors.Join(nil, sealed), nil},
		"another error":               {other, other},
		"another next to the seal":    {errors.Join(sealed, other), other},
		"several wraps in one format": {manyWraps, manyWraps},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := unexpected(tc.err)
			if (got == nil) != (tc.want == nil) || (got != nil && !errors.Is(got, tc.want)) {
				t.Errorf("unexpected(%v) = %v, want %v", tc.err, got, tc.want)
			}
			if tc.want != nil && !errors.Is(tc.err, errSealed) && got != nil && got.Error() != tc.err.Error() {
				t.Errorf("unexpected(%v) rewrote the text: %q", tc.err, got.Error())
			}
			if got != nil && errors.Is(got, errSealed) {
				t.Errorf("unexpected(%v) still holds the seal", tc.err)
			}
		})
	}
}

// Sealed, the store of the in-process State takes no write and removes nothing.
func TestASealedStoreTakesNoWrite(t *testing.T) {
	memory := objstore.NewMemory()
	ctx := context.Background()
	if err := memory.EnsureBucket(ctx, "b", objstore.BucketOptions{}); err != nil {
		t.Fatal(err)
	}
	store := &sealable{Client: memory}
	if _, err := store.Put(ctx, "b", "k", []byte("1"), objstore.PutOptions{}); err != nil {
		t.Fatalf("put before the seal: %v", err)
	}
	store.seal()
	if _, err := store.Put(ctx, "b", "k", []byte("2"), objstore.PutOptions{}); !errors.Is(err, errSealed) {
		t.Errorf("put after the seal: %v, want errSealed", err)
	}
	if err := store.Delete(ctx, "b", "k"); !errors.Is(err, errSealed) {
		t.Errorf("delete after the seal: %v, want errSealed", err)
	}
	if body, err := memory.Get(ctx, "b", "k"); err != nil || string(body) != "1" {
		t.Errorf("the object is %q (%v), want the one written before the seal", body, err)
	}
}

func TestOpenStoreRefusesAnUnknownKind(t *testing.T) {
	if _, err := OpenStore("s3"); err == nil {
		t.Error("an unknown store opened")
	}
	client, err := OpenStore(StoreMemory)
	if err != nil || client == nil {
		t.Errorf("memory store: %v", err)
	}
}
