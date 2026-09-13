//go:build e2e

// The one TestMain of the package (ownership.md v0.6 §1, test/e2e/**). The
// scenarios of the epics live in files of their own, prefixed with the owner,
// and a function name can be prefixed but TestMain cannot: two epics that each
// wrote one would break the build of the package at their first merge. An epic
// that needs something prepared before the tests of the package and released
// after them registers it here from an init of its own file, under the name of
// the owner that prefixes its files — state, swarm, gateway or ops, not the
// number of the epic:
//
//	func init() { registerPackageSetup("state", stateSetup) }
//
//	func stateSetup() (teardown func(), err error) { ... }
//
// Until an epic needs that, a lazy helper with its prefix on sync.Once is
// enough (T-444, iteration 3; T-446).

package e2e_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

// packageSetup prepares something for the whole package and returns what
// releases it; a nil teardown means there is nothing to release.
type packageSetup func() (teardown func(), err error)

// setupOwners are the owners that prefix the scenario files of the package
// (ownership.md v0.6 §1, test/e2e/**): state (EPIC-002), swarm (EPIC-003),
// gateway (EPIC-004), ops (EPIC-005). The setups run sorted by these names, and
// one owner may register once, so a registration under any other
// name — the number of an epic — would both run out of place and slip past the
// check for a second registration.
var setupOwners = []string{"state", "swarm", "gateway", "ops"}

// packageSetups holds the setups registered by the owners of the scenarios.
// The init functions of the files of the package all run before TestMain, so
// every registration is in by the time run reads them.
type packageSetups struct {
	byOwner map[string]packageSetup
	errs    []error
}

func newPackageSetups() *packageSetups {
	return &packageSetups{byOwner: make(map[string]packageSetup)}
}

var setups = newPackageSetups()

// registerPackageSetup registers the setup of owner for the whole package. A
// second registration of one owner, an owner outside setupOwners or a nil setup
// fails the package before any setup or test runs.
func registerPackageSetup(owner string, setup func() (teardown func(), err error)) {
	setups.register(owner, setup)
}

func (s *packageSetups) register(owner string, setup packageSetup) {
	switch {
	case owner == "":
		s.errs = append(s.errs, errors.New("a package setup was registered without the name of its owner"))
	case !slices.Contains(setupOwners, owner):
		s.errs = append(s.errs, fmt.Errorf("%q registered a package setup, want one of the owners %s",
			owner, strings.Join(setupOwners, ", ")))
	case setup == nil:
		s.errs = append(s.errs, fmt.Errorf("%s registered a nil package setup", owner))
	default:
		if _, dup := s.byOwner[owner]; dup {
			s.errs = append(s.errs, fmt.Errorf("%s registered a package setup twice", owner))
			return
		}
		s.byOwner[owner] = setup
	}
}

// run prepares the package in the order of the names of the owners, runs the
// tests and releases what was prepared in the reverse order. A failed setup
// releases the setups before it and fails the package without running a test.
func (s *packageSetups) run(tests func() int, stderr io.Writer) int {
	if err := errors.Join(s.errs...); err != nil {
		_, _ = fmt.Fprintf(stderr, "e2e: %s\n", strings.ReplaceAll(err.Error(), "\n", "; "))
		return 1
	}
	owners := make([]string, 0, len(s.byOwner))
	for owner := range s.byOwner {
		owners = append(owners, owner)
	}
	slices.Sort(owners)

	var teardowns []func()
	release := func() {
		for i := len(teardowns) - 1; i >= 0; i-- {
			teardowns[i]()
		}
	}
	for _, owner := range owners {
		teardown, err := s.byOwner[owner]()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "e2e: package setup of %s: %v\n", owner, err)
			release()
			return 1
		}
		if teardown != nil {
			teardowns = append(teardowns, teardown)
		}
	}
	code := tests()
	release()
	return code
}

func TestMain(m *testing.M) {
	os.Exit(setups.run(m.Run, os.Stderr))
}

// The registrar on three owners, registered out of their order: the setups run
// in the order of the names, the teardowns in the reverse one, the tests in
// between. ops prepares nothing it has to release and returns a nil teardown,
// which is skipped rather than called (review #1 of T-446, Mi-1).
func TestPackageSetupsRunInTheOrderOfTheOwners(t *testing.T) {
	var trail []string
	s := newPackageSetups()
	for _, owner := range []string{"state", "gateway"} {
		s.register(owner, func() (func(), error) {
			trail = append(trail, "setup "+owner)
			return func() { trail = append(trail, "teardown "+owner) }, nil
		})
	}
	s.register("ops", func() (func(), error) {
		trail = append(trail, "setup ops")
		return nil, nil
	})
	var stderr strings.Builder
	code := s.run(func() int { trail = append(trail, "tests"); return 0 }, &stderr)

	want := []string{"setup gateway", "setup ops", "setup state", "tests", "teardown state", "teardown gateway"}
	if code != 0 || !slices.Equal(trail, want) {
		t.Fatalf("run = %d with %q, want 0 with %q (stderr %q)", code, trail, want, stderr.String())
	}
}

// A failed second setup releases the first, runs no test and fails the
// package with the owner and the cause.
func TestAFailedPackageSetupReleasesTheOnesBeforeIt(t *testing.T) {
	var trail []string
	s := newPackageSetups()
	s.register("gateway", func() (func(), error) {
		trail = append(trail, "setup gateway")
		return func() { trail = append(trail, "teardown gateway") }, nil
	})
	s.register("state", func() (func(), error) {
		trail = append(trail, "setup state")
		return func() { trail = append(trail, "teardown state") }, errors.New("no broker")
	})
	var stderr strings.Builder
	code := s.run(func() int { trail = append(trail, "tests"); return 0 }, &stderr)

	want := []string{"setup gateway", "setup state", "teardown gateway"}
	if code == 0 || !slices.Equal(trail, want) {
		t.Fatalf("run = %d with %q, want a failure with %q", code, trail, want)
	}
	if !strings.Contains(stderr.String(), "of state:") || !strings.Contains(stderr.String(), "no broker") {
		t.Errorf("stderr = %q, want the owner and the cause", stderr.String())
	}
}

// registerPackageSetup writes to the registry TestMain runs, not to a copy. No
// epic calls it yet, so this is also what keeps it compiled and linted.
func TestRegisterPackageSetupReachesTestMain(t *testing.T) {
	saved := setups
	setups = newPackageSetups()
	t.Cleanup(func() { setups = saved })

	registerPackageSetup("swarm", func() (func(), error) { return nil, nil })
	registerPackageSetup("swarm", func() (func(), error) { return nil, nil })

	if _, ok := setups.byOwner["swarm"]; !ok {
		t.Error("the setup of swarm is not in the registry of the package")
	}
	if len(setups.errs) != 1 {
		t.Errorf("errors = %v, want the one of the second registration", setups.errs)
	}
}

// A registration the package cannot honour fails it before anything runs.
func TestAnInvalidRegistrationFailsThePackage(t *testing.T) {
	ok := func() (func(), error) { return nil, nil }
	cases := map[string]struct {
		register func(*packageSetups)
		want     string
	}{
		"one owner twice": {func(s *packageSetups) { s.register("state", ok); s.register("state", ok) }, "state registered a package setup twice"},
		"no owner":        {func(s *packageSetups) { s.register("", ok) }, "without the name of its owner"},
		"nil setup":       {func(s *packageSetups) { s.register("gateway", nil) }, "gateway registered a nil package setup"},
		"number of an epic": {func(s *packageSetups) { s.register("EPIC-002", ok) },
			`"EPIC-002" registered a package setup, want one of the owners state, swarm, gateway, ops`},
		"not an owner of the package": {func(s *packageSetups) { s.register("memory", ok) }, `"memory" registered a package setup`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newPackageSetups()
			tc.register(s)
			ran := false
			var stderr strings.Builder
			code := s.run(func() int { ran = true; return 0 }, &stderr)
			if code == 0 || ran {
				t.Fatalf("run = %d, tests ran: %v; want a failure before the tests", code, ran)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("stderr = %q, want %q", stderr.String(), tc.want)
			}
		})
	}
}
