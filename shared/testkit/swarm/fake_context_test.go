package swarm_test

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/swarm"
)

// --- the context of the process ---

// TestTheContextRunsAWholeFight is what the hook of T-255 mounts: a context of
// shared/runtime that starts, answers a player over the bus and stops — with no
// context of internal/** anywhere near it.
//
// It goes as far as a blow, and not only as far as the encounter opening,
// because FakeContext is where both halves of the stub stand together: the
// entry has to reach the narrator and the swing has to reach the fight, over
// the subscriptions the context opened rather than through a handler called by
// hand.
func TestTheContextRunsAWholeFight(t *testing.T) {
	bus := newBus(t)
	ctx := swarm.NewFakeContext(swarm.ContextConfig{
		WorldID:     worldID,
		RulesPath:   filepath.Join("..", "..", "..", "rules", "dark-forest.yaml"),
		FixturesDir: fixturesDir(t),
	})
	if ctx.Name() != swarm.ContextName {
		t.Errorf("the context is called %q, want %q", ctx.Name(), swarm.ContextName)
	}
	if deps := ctx.DependsOn(); len(deps) != 0 {
		t.Errorf("the stub waits for %v; it waits for nothing", deps)
	}
	if health := ctx.Health(); health.Status != runtime.StatusDegraded {
		t.Errorf("a context that has not started reports %q", health.Status)
	}

	if err := ctx.Start(t.Context(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = ctx.Stop(t.Context()) })

	health := ctx.Health()
	if health.Status != runtime.StatusOK {
		t.Fatalf("a running stub reports %q: %v", health.Status, health.Details)
	}
	if health.Details["removed_by"] != "T-256" {
		t.Errorf("health does not say which task removes the stub: %v", health.Details)
	}

	if err := bus.Publish(t.Context(), entered(playerA, nameA, regionID, regName)); err != nil {
		t.Fatalf("publish the entry: %v", err)
	}
	waitFor(t, "the encounter to open", func() bool {
		_, open := ctx.Encounter().ActiveEncounter(playerA)
		return open
	})
	// The narrator of the same context answers the same entry: FakeContext
	// raises both halves of the stub (design §4.1 p. 1).
	waitFor(t, "the narrative of the entry", func() bool {
		return len(eventsOf(t, bus, eventbus.TopicNarrativeOutput)) > 0
	})
	if n := ctx.Encounter().ActiveCount(); n != 1 {
		t.Errorf("%d fights in progress, want one", n)
	}

	// The swing: the wolf of the fixtures survives one blow whatever the table
	// rolls, so the fight is still open when the decisions arrive.
	if err := bus.Publish(t.Context(), attack(playerA, nameA, wolfID)); err != nil {
		t.Fatalf("publish the attack: %v", err)
	}
	waitFor(t, "the exchange to be decided", func() bool {
		return len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided)) > 0
	})
	waitFor(t, "the package of the exchange", func() bool {
		return len(ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)) > 0
	})
	if got := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeDiceRolled); len(got) == 0 {
		t.Error("the blow was decided without a die being rolled")
	}
	if _, open := ctx.Encounter().ActiveEncounter(playerA); !open {
		t.Error("one blow ended the fight; the wolf of the fixtures survives it")
	}

	if err := ctx.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if health := ctx.Health(); health.Status != runtime.StatusDegraded {
		t.Errorf("a stopped context reports %q", health.Status)
	}
	if err := ctx.Stop(t.Context()); err != nil {
		t.Errorf("stopping twice: %v", err)
	}
}

// TestTheContextRefusesToStartTwiceAndWithoutABus: a stub mounted twice, or
// mounted with nothing to talk on, is a defect of the process that has to be
// visible at start rather than as silence afterwards.
//
// Each refusal is read, not merely counted. Start has four reasons to refuse —
// no bus, already started, unreadable rules, unreadable fixtures — and a test
// content with any of them would stay green while the wrong one fired.
func TestTheContextRefusesToStartTwiceAndWithoutABus(t *testing.T) {
	bus := newBus(t)
	ctx := swarm.NewFakeContext(swarm.ContextConfig{
		WorldID:   worldID,
		RulesPath: filepath.Join("..", "..", "..", "rules", "dark-forest.yaml"),
	})
	assertRefusal(t, ctx.Start(t.Context(), runtime.Deps{}), "no bus in deps",
		"a context with no bus started")

	if err := ctx.Start(t.Context(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = ctx.Stop(t.Context()) })
	assertRefusal(t, ctx.Start(t.Context(), runtime.Deps{Bus: bus}), "already started",
		"the context started a second time")
}

// TestTheContextFailsOnRulesItCannotRead: the rule book is what every number of
// a fight comes from, and a stub without one would decide by defaults nobody
// wrote down. The refusal has to name the rules and the path it looked at,
// because "the context did not start" is the same sentence for all four
// reasons Start has.
func TestTheContextFailsOnRulesItCannotRead(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-rules.yaml")
	ctx := swarm.NewFakeContext(swarm.ContextConfig{WorldID: worldID, RulesPath: missing})

	err := ctx.Start(t.Context(), runtime.Deps{Bus: newBus(t)})
	assertRefusal(t, err, "load rules", "the context started without a rule book")
	if err != nil && !strings.Contains(err.Error(), "no-such-rules.yaml") {
		t.Errorf("the refusal is %q and names no path; the operator cannot tell "+
			"which file was missing", err)
	}
}

// TestTheContextFailsOnFixturesItCannotRead is the fourth reason, and it is
// here so that the three above cannot pass for it: an empty FixturesDir means
// "seed nothing", but a directory that was named and cannot be read is a
// mounting error.
func TestTheContextFailsOnFixturesItCannotRead(t *testing.T) {
	ctx := swarm.NewFakeContext(swarm.ContextConfig{
		WorldID:     worldID,
		RulesPath:   filepath.Join("..", "..", "..", "rules", "dark-forest.yaml"),
		FixturesDir: filepath.Join(t.TempDir(), "no-such-fixtures"),
	})

	assertRefusal(t, ctx.Start(t.Context(), runtime.Deps{Bus: newBus(t)}), "seed the world",
		"the context started over a fixture tree that is not there")
}

// assertRefusal is a refusal read rather than counted: the error has to be
// there and it has to be the one the test is about.
func assertRefusal(t *testing.T, err error, want, whenNil string) {
	t.Helper()
	if err == nil {
		t.Error(whenNil)
		return
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("refused with %q, want the reason %q: the reasons Start refuses for "+
			"are not interchangeable", err, want)
	}
}

// --- what the stub may depend on ---

// TestThePackageCanBeCompiledIntoTheProductionBinary is the condition ADR-001
// addendum p. 8 puts on this package: the hook of T-255 links it into
// cmd/multiverse, so nothing here may reach for a testing library, for the
// command line, or for the environment the flag lives in.
//
// It is checked by walking the imports rather than by reading the code, because
// the dependency that breaks it would arrive through a package this one merely
// uses — and it is a test rather than a promise in a comment because that is
// the only form of it a merge cannot ignore.
func TestThePackageCanBeCompiledIntoTheProductionBinary(t *testing.T) {
	const root = "multiverse-core.io/shared/testkit/swarm"
	seen := make(map[string]bool)
	var walk func(path string)
	walk = func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		for _, imp := range importsOf(t, path) {
			switch {
			case imp == "testing" || strings.HasPrefix(imp, "testing/"):
				t.Errorf("%s imports %s: the package would not compile into cmd/multiverse", path, imp)
			case strings.Contains(imp, "testify") || strings.Contains(imp, "goleak"):
				t.Errorf("%s imports %s, a testing library", path, imp)
			case strings.HasPrefix(imp, "multiverse-core.io/cmd/"):
				t.Errorf("%s imports %s: the stub knows nothing of the commands that mount it", path, imp)
			case imp == "multiverse-core.io/shared/env" && path == root:
				// Only the stub itself: shared/runtime and shared/objstore read
				// their own configuration through shared/env, which is what
				// that package is for. What must not happen is the stub
				// reaching for the flag that decides whether to mount it.
				t.Errorf("%s reads the environment: MV_SWARM_FAKE is read by the hook of T-255, "+
					"never by the stub (ADR-001 addendum p. 8)", path)
			}
			if strings.HasPrefix(imp, "multiverse-core.io/") {
				walk(imp)
			}
		}
	}
	walk(root)
	if len(seen) < 2 {
		t.Fatalf("the walk saw %d packages; it did not read the imports at all", len(seen))
	}
}

// TestTheStubNamesNoEnvironmentVariable is the other half of the same
// condition, read off the code with the comments removed: the prose of this
// package explains why the flag is not here, and only the code has to be free
// of it.
func TestTheStubNamesNoEnvironmentVariable(t *testing.T) {
	for _, name := range sourcesOf(t, ".") {
		if strings.Contains(codeOf(t, name), "MV_") {
			t.Errorf("%s names an environment variable; configuration reaches the stub "+
				"through ContextConfig, and the flag belongs to the hook of T-255", name)
		}
	}
}

// --- reading the source of the package ---

// sourcesOf lists the Go files of a directory that are compiled into the
// package itself — the test files are what talks about the stub, not what is
// linked into a binary.
func sourcesOf(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	if len(out) == 0 {
		t.Fatalf("%s holds no source files", dir)
	}
	return out
}

// codeOf is a source file without its comments: what the compiler sees.
func codeOf(t *testing.T, path string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		t.Fatalf("print %s: %v", path, err)
	}
	return buf.String()
}

// importsOf is what a package of this module imports, read from its source.
//
// The directory is derived from the import path rather than from go/build,
// because the answer must not depend on a module cache, on the network or on
// the working directory a test runner picked.
func importsOf(t *testing.T, importPath string) []string {
	t.Helper()
	const module = "multiverse-core.io/"
	if !strings.HasPrefix(importPath, module) {
		return nil
	}
	dir := filepath.Join("..", "..", "..", filepath.FromSlash(strings.TrimPrefix(importPath, module)))
	fset := token.NewFileSet()
	seen := make(map[string]bool)
	var out []string
	for _, path := range sourcesOf(t, dir) {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range file.Imports {
			name := strings.Trim(imp.Path.Value, `"`)
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}
