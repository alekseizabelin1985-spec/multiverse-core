package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
)

// wolfNowhere is a copy of the fixtures whose wolf stands in a region that does
// not exist: State creates the world and the region and refuses the wolf
// law_violation.
func wolfNowhere(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, file := range state.FixtureFiles {
		body, err := os.ReadFile(filepath.Join(fixturesDir, file.Name))
		if err != nil {
			t.Fatal(err)
		}
		if file.Type == entity.TypeNPC {
			moved := bytes.Replace(body, []byte(`"position": "dark-forest-01"`), []byte(`"position": "nowhere"`), 1)
			if bytes.Equal(moved, body) {
				t.Fatal("the wolf of the fixtures has no position to move")
			}
			body = moved
		}
		if err := os.WriteFile(filepath.Join(dir, file.Name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// initReport is the JSON report of world init.
type initReport struct {
	Status   string        `json:"status"`
	Findings []cli.Finding `json:"findings"`
	Details  InitResult    `json:"details"`
}

func decodeInit(t *testing.T, stdout string) initReport {
	t.Helper()
	var report initReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("JSON report: %v\n%s", err, stdout)
	}
	return report
}

// N-3 of review #2 of T-058: what a snapshot asked of State means is decided
// by the pointer. A pointer is a written latest.json, whatever came with it.
func TestSnapshotOutcomeDecidesByThePointer(t *testing.T) {
	pointer := &state.LatestPointer{Snapshot: state.SnapshotMeta{ID: "state:w:000000", Reason: state.SnapshotBootstrap}}
	lost := errors.New("snapshot.created did not go out")
	cases := map[string]struct {
		pointer                *state.LatestPointer
		err                    error
		snapshot, warn, failed bool
	}{
		"a pointer and an error": {pointer, lost, true, true, false},
		"a pointer alone":        {pointer, nil, true, false, false},
		"an error alone":         {nil, lost, false, false, true},
		"nothing at all":         {nil, nil, false, false, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snapshot, warning, failure := snapshotOutcome(tc.pointer, tc.err)
			if (snapshot != nil) != tc.snapshot || (warning != nil) != tc.warn || (failure != nil) != tc.failed {
				t.Fatalf("snapshot %v, warning %v, failure %v", snapshot, warning, failure)
			}
			if snapshot != nil && snapshot.ID != pointer.Snapshot.ID {
				t.Errorf("snapshot %s, want the one of the pointer", snapshot.ID)
			}
			if warning != nil && !errors.Is(warning, lost) {
				t.Errorf("warning %v, want the error that came with the pointer", warning)
			}
			if failure != nil && (!errors.Is(failure, errSnapshot) || (tc.err != nil && !errors.Is(failure, tc.err))) {
				t.Errorf("failure %v, want errSnapshot with the error of State", failure)
			}
		})
	}
}

// N-4 of review #2 of T-058: a State that refused the bootstrap and then failed
// to stop says both. Here its shutdown snapshot cannot list the snapshots once
// the world exists — something other than the sealed store.
func TestInitReportsAStopThatFailsAfterARefusedBootstrap(t *testing.T) {
	s := newStand()
	refused := false
	s.wrap = func(client objstore.Client) objstore.Client {
		return &refusingStore{Client: client, listRefused: func(bucket string) bool {
			if bucket != objstore.SnapshotsBucket(world) {
				return false
			}
			if _, err := client.Stat(context.Background(), objstore.EntitiesBucket(world), "world/"+world+".json"); err != nil {
				return false
			}
			refused = true
			return true
		}}
	}
	r := s.run("init", "--world", world, "--fixtures", wolfNowhere(t), "--bus", BusMemory, "--store", StoreMemory,
		"--rules", rulesBook)
	if r.code != cli.ExitFindings {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, cli.ExitFindings, r.stdout, r.stderr)
	}
	if !refused {
		t.Fatal("the store never refused a list: the test did not reach the failure it is about")
	}
	for _, want := range []string{"[bootstrap]", "law_violation", "stop the in-process state", "list refused"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not say %q: %s", want, r.stderr)
		}
	}
	if strings.Contains(r.stderr, "[snapshot]") {
		t.Errorf("the refusal of the bootstrap is reported as a snapshot: %s", r.stderr)
	}
}

// The reason of a refusal is the operator's to read (acceptance of T-058, 3.1):
// the entity, the reason and the details State gave, in the text and in --json.
func TestInitNamesTheDetailsOfARefusal(t *testing.T) {
	dir := wolfNowhere(t)
	args := []string{"init", "--world", world, "--fixtures", dir, "--bus", BusMemory, "--store", StoreMemory, "--rules", rulesBook}

	r := newStand().run(args...)
	if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "[bootstrap]") {
		t.Fatalf("exit %d, stderr %q; want %d with a bootstrap finding", r.code, r.stderr, cli.ExitFindings)
	}
	if !strings.Contains(r.stderr, "npc/wolf-alpha refused law_violation (invariant_id inv-") {
		t.Errorf("stderr does not name the entity, the reason and the invariant: %s", r.stderr)
	}

	r = newStand().run(append(args, "--json")...)
	report := decodeInit(t, r.stdout)
	refusal := report.Details.Refusal
	if r.code != cli.ExitFindings || refusal == nil {
		t.Fatalf("--json: exit %d, report %+v; want the refusal in the details", r.code, report)
	}
	if invariant, _ := refusal.Details["invariant_id"].(string); refusal.Entity.ID != "wolf-alpha" ||
		refusal.Reason != "law_violation" || !strings.HasPrefix(invariant, "inv-") || refusal.EventID == "" {
		t.Errorf("--json: refusal %+v, want wolf-alpha refused law_violation with its invariant_id", refusal)
	}
}

// putPointer writes body as latest.json of the world.
func putPointer(t *testing.T, s *stand, body string) {
	t.Helper()
	if _, err := s.objects.Put(context.Background(), objstore.SnapshotsBucket(world), state.PointerKey, []byte(body),
		objstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

// --force over a latest.json that does not decode creates the world again: the
// pointer is removed with the rest and never read.
func TestInitWithForceOverAPointerThatDoesNotDecode(t *testing.T) {
	s := newStand()
	if r := s.init(); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	putPointer(t, s, `{"snapshot": [`)

	r := s.init("--force")
	if r.code != cli.ExitOK {
		t.Fatalf("--force: exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	if meta := s.latest(t).Snapshot; meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap || meta.StateHash != fixtureHash(t) {
		t.Errorf("latest.json: seq %d, reason %s, state_hash %s; want seq 0 bootstrap of the fixture world",
			meta.Seq, meta.Reason, meta.StateHash)
	}
}

// Without --force a latest.json that does not decode is a finding of the store
// that names the pointer and --force, and nothing is written over it.
func TestInitWithoutForceOverAPointerThatDoesNotDecode(t *testing.T) {
	s := newStand()
	if r := s.init(); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	putPointer(t, s, `{"snapshot": [`)
	before := len(s.snapshots(t))

	r := s.init()
	if r.code != cli.ExitFindings {
		t.Fatalf("exit %d, want %d\nstderr: %s", r.code, cli.ExitFindings, r.stderr)
	}
	for _, want := range []string{"[store]", state.PointerKey, "--force"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not say %q: %s", want, r.stderr)
		}
	}
	body, err := s.objects.Get(context.Background(), objstore.SnapshotsBucket(world), state.PointerKey)
	if err != nil || string(body) != `{"snapshot": [` || len(s.snapshots(t)) != before {
		t.Errorf("the refused run wrote over the pointer: %q (%v), %d snapshot objects, had %d", body, err,
			len(s.snapshots(t)), before)
	}
}

// A store that cannot read latest.json is a finding of the store with --force
// and without: what is there cannot be told, so nothing is removed.
func TestInitOverAPointerTheStoreCannotRead(t *testing.T) {
	for _, extra := range [][]string{nil, {"--force"}} {
		t.Run(strings.Join(append([]string{"init"}, extra...), " "), func(t *testing.T) {
			s := newStand()
			if r := s.init(); r.code != cli.ExitOK {
				t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
			}
			s.wrap = func(client objstore.Client) objstore.Client {
				return &refusingStore{Client: client, getRefused: func(bucket, key string) bool {
					return bucket == objstore.SnapshotsBucket(world) && key == state.PointerKey
				}}
			}
			r := s.init(extra...)
			if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "[store]") || !strings.Contains(r.stderr, "get refused") {
				t.Fatalf("exit %d, stderr %q; want %d with the store finding", r.code, r.stderr, cli.ExitFindings)
			}
			entities, err := s.objects.List(context.Background(), objstore.EntitiesBucket(world), "")
			if err != nil || len(entities) != 6 {
				t.Errorf("%d entity objects (%v) after the run, want the 6 of the world untouched", len(entities), err)
			}
		})
	}
}

// The world of a deployment is created only through the State of the running
// core, so that is what world status over MinIO tells to do.
func TestStatusOfAWorldNobodyInitializedInTheStoreOfADeployment(t *testing.T) {
	s := newStand()
	r := s.run("status", "--world", world, "--store", StoreMinIO)
	if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "not initialized") ||
		!strings.Contains(r.stderr, "run mvctl world init --bus kafka while core is running") {
		t.Errorf("exit %d, stderr %q; want %d with the hint of --bus kafka", r.code, r.stderr, cli.ExitFindings)
	}
}
