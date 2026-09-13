package laws

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
	"multiverse-core.io/shared/eventbus"
)

// shippedDir is the laws directory of the repository, seen from the package.
const shippedDir = "../../../../laws"

const world = "dark-forest-world"

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestShowPrintsTheCurrentLawsOfTheShippedWorld(t *testing.T) {
	code, stdout, stderr := run(t, "show", "--dir="+shippedDir, "--world="+world)
	if code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, want := range []string{
		"world dark-forest-world: laws v1 (approved, created by author), current according to the files; versions: v1",
		"inv-01  invariant inv-01", "inv-11  invariant laws_version_current", "law-2   declarative",
		"strain 0", "strain is counted by core",
		"laws show: 1 world shown",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestShowAsJSON(t *testing.T) {
	code, stdout, _ := run(t, "show", "--dir="+shippedDir, "--world=", "--json")
	if code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, stdout)
	}
	var report struct {
		Status  string `json:"status"`
		Details []struct {
			World   string                   `json:"world"`
			Origin  string                   `json:"origin"`
			Current struct{ Version string } `json:"current"`
			Strain  map[string]int64         `json:"strain"`
		} `json:"details"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("%v\n%s", err, stdout)
	}
	if report.Status != cli.StatusOK || len(report.Details) != 1 {
		t.Fatalf("report %+v", report)
	}
	d := report.Details[0]
	if d.World != world || d.Origin != "files" || d.Current.Version != "v1" || len(d.Strain) != 13 {
		t.Errorf("details %+v, want v1 of %s from the files with 13 laws in strain", d, world)
	}
}

func TestShowFindings(t *testing.T) {
	broken := t.TempDir()
	writeFile(t, filepath.Join(broken, "dark-forest-world.v1.yaml"), "version: [")
	writeFile(t, filepath.Join(broken, "other-world.v1.yaml"), "version: [")
	cases := map[string]struct {
		args  []string
		check string
	}{
		"unknown world":     {[]string{"show", "--dir=" + shippedDir, "--world=no-such-world"}, CheckWorld},
		"broken document":   {[]string{"show", "--dir=" + broken, "--world=" + world}, CheckDocument},
		"missing directory": {[]string{"show", "--dir=" + filepath.Join(broken, "none")}, CheckSource},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := run(t, tc.args...)
			if code != cli.ExitFindings || !strings.Contains(stderr, "["+tc.check+"]") {
				t.Errorf("exit %d, stderr %q; want %d with a [%s] finding\nstdout: %s",
					code, stderr, cli.ExitFindings, tc.check, stdout)
			}
		})
	}
	// A world asked for by name does not report the broken documents of
	// another world.
	_, _, stderr := run(t, "show", "--dir="+broken, "--world="+world)
	if strings.Contains(stderr, "other-world") {
		t.Errorf("the documents of another world are reported:\n%s", stderr)
	}
}

// A laws directory that cannot be read is named by its absolute path, and a
// relative one says what it is relative to (review T-205 #1, N-3).
func TestSourceFindingNamesTheAbsoluteDirectory(t *testing.T) {
	const relative = "no-such-laws-dir"
	abs, err := filepath.Abs(relative)
	if err != nil {
		t.Fatal(err)
	}
	hint := "relative to the working directory"
	for name, args := range map[string][]string{
		"show": {"show", "--dir=" + relative},
		"bump": {"bump", "--world=" + world, "--from=x.yaml", "--dir=" + relative, "--bus=memory"},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := run(t, args...)
			if code != cli.ExitFindings || !strings.Contains(stderr, "[source] "+abs+": ") || !strings.Contains(stderr, hint) {
				t.Errorf("exit %d, stderr %q; want a [source] finding on %s with the hint", code, stderr, abs)
			}
		})
	}
	absent := filepath.Join(t.TempDir(), "none")
	if _, _, stderr := run(t, "show", "--dir="+absent); !strings.Contains(stderr, "[source] "+absent+": ") || strings.Contains(stderr, hint) {
		t.Errorf("an absolute directory: stderr %q; want the path as given and no hint", stderr)
	}
}

func TestUsageErrors(t *testing.T) {
	dir := t.TempDir()
	for name, args := range map[string][]string{
		"no subcommand":        nil,
		"unknown subcommand":   {"check"},
		"show extra argument":  {"show", "extra"},
		"show bad flag":        {"show", "--nope"},
		"bump without --from":  {"bump", "--dir=" + dir},
		"bump without --world": {"bump", "--world=", "--from=x.yaml"},
		"bump unknown bus":     {"bump", "--from=x.yaml", "--bus=nats"},
		"bump extra argument":  {"bump", "--from=x.yaml", "extra"},
		"bump bad flag":        {"bump", "--nope"},
	} {
		t.Run(name, func(t *testing.T) {
			if code, _, stderr := run(t, args...); code != cli.ExitUsage {
				t.Errorf("exit %d, want %d; stderr %s", code, cli.ExitUsage, stderr)
			}
		})
	}
	if code, _, _ := run(t, "show", "-h"); code != cli.ExitOK {
		t.Errorf("show -h exit %d", code)
	}
}

const v1 = `version: v1
world_id: dark-forest-world
status: approved
created_by: author
created_at: 2026-09-09T00:00:00Z
laws:
  - { id: inv-01, kind: invariant, check: inv-01, text: "a" }
  - { id: law-2, kind: declarative, source: author, text: "b" }
`

const v2 = `version: v2
world_id: dark-forest-world
status: approved
created_by: author
based_on: v1
created_at: 2026-09-13T00:00:00Z
laws:
  - { id: inv-01, kind: invariant, check: inv-01, text: "a" }
  - { id: law-3, kind: declarative, source: author, text: "c" }
`

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func bumpDir(t *testing.T) (dir, next string) {
	t.Helper()
	dir = t.TempDir()
	writeFile(t, filepath.Join(dir, "dark-forest-world.v1.yaml"), v1)
	next = filepath.Join(dir, "dark-forest-world.v2.yaml")
	writeFile(t, next, v2)
	return dir, next
}

func TestBumpOnTheMemoryBus(t *testing.T) {
	dir, next := bumpDir(t)
	code, stdout, stderr := run(t, "bump", "--world="+world, "--from="+next, "--dir="+dir, "--bus=memory")
	if code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, want := range []string{
		"world dark-forest-world: laws v1 -> v2, added [law-3], removed [law-2]",
		"published world.laws.changed ", "published entity.update.proposed ",
		"nothing outside it saw the events", "laws bump: v2 announced",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}

	code, stdout, _ = run(t, "bump", "--world="+world, "--from="+next, "--dir="+dir, "--bus=memory", "--json")
	if code != cli.ExitOK {
		t.Fatalf("json exit %d", code)
	}
	var report struct {
		Details Bumped `json:"details"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("%v\n%s", err, stdout)
	}
	d := report.Details
	if d.From != "v1" || d.To != "v2" || d.Bus != BusMemory || len(d.EventIDs) != 2 ||
		strings.Join(d.Added, ",") != "law-3" || strings.Join(d.Removed, ",") != "law-2" {
		t.Errorf("details %+v", d)
	}
}

func TestBumpRefusedIsAFinding(t *testing.T) {
	dir, _ := bumpDir(t)
	code, stdout, stderr := run(t, "bump", "--world="+world,
		"--from="+filepath.Join(dir, "dark-forest-world.v1.yaml"), "--dir="+dir, "--bus=memory")
	if code != cli.ExitFindings || !strings.Contains(stderr, "[bump]") || !strings.Contains(stderr, "no based_on") {
		t.Errorf("exit %d, stderr %q, stdout %q; want a [bump] finding", code, stderr, stdout)
	}
	code, _, stderr = run(t, "bump", "--world="+world, "--from=x.yaml",
		"--dir="+filepath.Join(dir, "none"), "--bus=memory")
	if code != cli.ExitFindings || !strings.Contains(stderr, "[source]") {
		t.Errorf("a missing laws directory: exit %d, stderr %q", code, stderr)
	}
}

// failingBus accepts the first publication and refuses the second.
type failingBus struct {
	eventbus.Bus
	calls int
}

func (b *failingBus) Publish(context.Context, eventbus.Event) error {
	b.calls++
	if b.calls == 2 {
		return errors.New("broker unavailable")
	}
	return nil
}

func TestBumpNamesWhatWasPublishedBeforeAFailure(t *testing.T) {
	dir, next := bumpDir(t)
	report := cli.NewReport("laws bump")
	Bump(context.Background(), BumpRequest{World: world, From: next, Dir: dir, Bus: &failingBus{}, BusName: BusKafka}, report)
	var stdout, stderr bytes.Buffer
	if code := report.Write(&stdout, &stderr, false); code != cli.ExitFindings {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "the proposal of world.laws_version is not") {
		t.Errorf("stderr %q", stderr.String())
	}
	// Only what the bus accepted is named: the refused proposal is not.
	_, ids, found := strings.Cut(strings.TrimSpace(stdout.String()), "published before the failure: ")
	if !found || ids == "" || strings.Contains(ids, ",") {
		t.Errorf("stdout %q, want exactly the id of world.laws.changed", stdout.String())
	}
}

func TestOpenBus(t *testing.T) {
	for _, name := range []string{BusKafka, BusMemory} {
		bus, err := openBus(name)
		if err != nil {
			t.Errorf("openBus(%s): %v", name, err)
			continue
		}
		_ = bus.Close()
	}
	if _, err := openBus("nats"); err == nil {
		t.Error("openBus accepted an unknown bus")
	}
}
