package testkit_test

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"multiverse-core.io/shared/testkit"
)

// TestVersionsMatchesTheFile reads build/versions.env with a parser of its own
// and compares the result with Versions(). The point is the divergence, not the
// values: a pin that the tests read differently from the way compose and the
// Makefile read it is worse than no pin at all, because every environment then
// runs a slightly different image (NFR-071).
func TestVersionsMatchesTheFile(t *testing.T) {
	pins, err := testkit.Versions()
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}

	path, err := testkit.Path(filepath.FromSlash(testkit.VersionsFile))
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	want := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("%s: line %q is neither a comment nor an assignment", testkit.VersionsFile, line)
		}
		want[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if len(want) == 0 {
		t.Fatalf("%s defines no pins", testkit.VersionsFile)
	}
	for name, value := range want {
		got, ok := pins[name]
		if !ok {
			t.Errorf("Versions() does not carry %s", name)
			continue
		}
		if got != value {
			t.Errorf("Versions()[%s] = %q, the file says %q", name, got, value)
		}
	}
	for name := range pins {
		if _, ok := want[name]; !ok {
			t.Errorf("Versions() carries %s, which the file does not define", name)
		}
	}
}

// The pins the container helpers below need must exist and must name a tag: a
// floating tag is what versions.env exists to forbid, and an image the helpers
// cannot resolve fails a whole integration job with a message about Docker
// rather than about the pin.
func TestTheImagePinsAreUsable(t *testing.T) {
	for _, name := range []string{
		"REDPANDA_IMAGE", "MINIO_IMAGE", "QDRANT_IMAGE", "NEO4J_IMAGE", "GO_VERSION",
	} {
		value, err := testkit.Version(name)
		if err != nil {
			t.Errorf("Version(%s): %v", name, err)
			continue
		}
		if strings.HasSuffix(value, ":latest") || value == "latest" {
			t.Errorf("%s = %q: versions.env forbids a floating tag (NFR-071)", name, value)
		}
		if name != "GO_VERSION" && !strings.Contains(value, ":") {
			t.Errorf("%s = %q: an image pin must carry a tag or a digest", name, value)
		}
	}
}

// A pin the file leaves empty on purpose (CHROMA_IMAGE) is present in the map
// and refused by Version: an empty image name silently starts whatever the
// daemon has cached under that repository.
func TestAnEmptyPinIsReportedRatherThanReturned(t *testing.T) {
	pins, err := testkit.Versions()
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if _, ok := pins["CHROMA_IMAGE"]; !ok {
		t.Skip("CHROMA_IMAGE is no longer declared in versions.env")
	}
	if _, err := testkit.Version("CHROMA_IMAGE"); !errors.Is(err, testkit.ErrNoVersion) {
		t.Errorf("Version of an empty pin = %v, want ErrNoVersion", err)
	}
	if _, err := testkit.Version("NO_SUCH_PIN"); !errors.Is(err, testkit.ErrNoVersion) {
		t.Errorf("Version of an unknown pin = %v, want ErrNoVersion", err)
	}
}

// Versions returns a copy: a test that edits what it got must not change what
// the next one sees.
func TestVersionsReturnsACopy(t *testing.T) {
	first, err := testkit.Versions()
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	first["REDPANDA_IMAGE"] = "tampered"
	second, err := testkit.Versions()
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if second["REDPANDA_IMAGE"] == "tampered" {
		t.Error("Versions() handed out the map it caches")
	}
}

func TestRepoRootHoldsTheModule(t *testing.T) {
	root, err := testkit.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Errorf("RepoRoot returned %s, which holds no go.mod: %v", root, err)
	}
	if _, err := testkit.Path("schemas", "events", "_envelope.json"); err != nil {
		t.Errorf("Path: %v", err)
	}
}
