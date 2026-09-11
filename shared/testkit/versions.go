package testkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"

	"multiverse-core.io/shared/env"
)

// VersionsFile is the single source of the pinned versions, relative to the
// repository root (NFR-071). compose interpolates it, the Makefile includes it,
// CI appends it to the job environment, and the container helpers below read it
// through Versions() — so that an image is bumped in one place.
const VersionsFile = "build/versions.env"

// ErrNoVersion marks a pin the file does not define, or defines empty.
var ErrNoVersion = errors.New("testkit: no such pin in " + VersionsFile)

var (
	versionsOnce sync.Once
	versionsMap  map[string]string
	versionsErr  error
)

// Versions returns the pins of build/versions.env as a map of name to value.
// Names whose value is empty on purpose (CHROMA_IMAGE) are present with an
// empty value: the file says "undecided", and hiding that behind a missing key
// would let a caller silently fall back to a guess.
//
// The file is parsed once per process; the returned map is a copy.
func Versions() (map[string]string, error) {
	versionsOnce.Do(func() {
		root, err := RepoRoot()
		if err != nil {
			versionsErr = err
			return
		}
		versionsMap, versionsErr = readVersions(filepath.Join(root, filepath.FromSlash(VersionsFile)))
	})
	if versionsErr != nil {
		return nil, versionsErr
	}
	out := make(map[string]string, len(versionsMap))
	for k, v := range versionsMap {
		out[k] = v
	}
	return out, nil
}

// Version returns one pin. It fails on an unknown or empty name rather than
// returning "": an image name that is silently empty starts a container from
// whatever the daemon has cached under that repository.
func Version(name string) (string, error) {
	all, err := Versions()
	if err != nil {
		return "", err
	}
	value, ok := all[name]
	if !ok || value == "" {
		return "", fmt.Errorf("%w: %s", ErrNoVersion, name)
	}
	return value, nil
}

// MustVersion returns one pin or panics. It is meant for the container helpers,
// where a missing pin is a defect of the repository and not a runtime
// condition.
func MustVersion(name string) string {
	value, err := Version(name)
	if err != nil {
		panic(err)
	}
	return value
}

// readVersions parses the dotenv file with the parser of shared/env, the same
// one that reads .env.example: the two files share a format, and a second
// parser would be a second set of rules about comments and quoting.
func readVersions(path string) (map[string]string, error) {
	f, err := os.Open(path) //nolint:gosec // the path is derived from the repository root, not from input
	if err != nil {
		return nil, fmt.Errorf("testkit: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	entries, err := env.ParseExample(f)
	if err != nil {
		return nil, fmt.Errorf("testkit: parse %s: %w", path, err)
	}
	pins := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.Commented {
			continue
		}
		pins[e.Name] = e.Value
	}
	if len(pins) == 0 {
		return nil, fmt.Errorf("testkit: %s defines no pins", path)
	}
	return pins, nil
}

var (
	rootOnce sync.Once
	rootDir  string
	rootErr  error
)

// RepoRoot returns the directory holding go.mod. It walks up from the source
// file of this package rather than from the working directory: a test runs in
// the directory of its own package, and every caller would otherwise carry its
// own count of "../".
func RepoRoot() (string, error) {
	rootOnce.Do(func() {
		_, self, _, ok := goruntime.Caller(0)
		if !ok {
			rootErr = errors.New("testkit: cannot locate the source of the package")
			return
		}
		dir := filepath.Dir(self)
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				rootDir = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				rootErr = fmt.Errorf("testkit: no go.mod above %s", filepath.Dir(self))
				return
			}
			dir = parent
		}
	})
	return rootDir, rootErr
}

// Path joins a path relative to the repository root, for a test that needs a
// fixture or a schema by name.
func Path(parts ...string) (string, error) {
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{root}, parts...)...), nil
}
