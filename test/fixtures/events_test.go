// This file guards testdata/fixtures/events — one valid and one invalid
// payload per event type (T-214, api-contracts.md §2.3.6–§2.3.9, §2.3.13).
//
// A payload fixture is not documentation. It is the shape a publisher of the
// type has to produce and the shape a consumer may assume, written down where
// both can read it before either exists: the schemas of EPIC-003 land in wave
// 1 and the agents that fill them arrive later.
//
// So every pair is checked in the two directions that matter. The valid
// payload must survive the whole read path — envelope, topic policy, payload
// schema — and reach a handler. The invalid one must not: it is parked in
// dead_letters and the handler is never called. The same invalid payload is
// then read again through the lenient view of the bus, where it does reach the
// handler; that second read is what proves the rejection came from validation
// on read (MV_BUS_VALIDATE_ON_READ, SEC-16, ADR-007 add. 4) rather than from
// something else on the way.
//
// The envelope around a fixture is built from the registry rather than from a
// table here. The topic of a type, who may publish it and what its policy
// demands are contract, and a test that copied them would go on passing after
// the contract moved.
package fixtures_test

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
)

// The two halves of every pair, as they appear in a file name.
const (
	fixtureValid   = "valid"
	fixtureInvalid = "invalid"
)

func eventFixtureDir() string { return repoPath("testdata", "fixtures", "events") }

// eventFixture is one file: the type it belongs to, the schema version it was
// written against, and whether it is meant to pass.
type eventFixture struct {
	typ     string
	version int
	kind    string // valid | invalid
	name    string
}

func (fx eventFixture) path() string { return filepath.Join(eventFixtureDir(), fx.name) }

// parseEventFixtureName splits <type>.v<n>.<valid|invalid>.json. The type
// itself contains dots (world.law_breach.applied), so the name is taken apart
// from the right: the last two segments before .json are the version and the
// half of the pair, and whatever precedes them is the type.
func parseEventFixtureName(name string) (eventFixture, bool) {
	base, ok := strings.CutSuffix(name, ".json")
	if !ok {
		return eventFixture{}, false
	}
	rest, kind, ok := cutLast(base)
	if !ok || (kind != fixtureValid && kind != fixtureInvalid) {
		return eventFixture{}, false
	}
	typ, versionPart, ok := cutLast(rest)
	if !ok || typ == "" {
		return eventFixture{}, false
	}
	digits, ok := strings.CutPrefix(versionPart, "v")
	if !ok {
		return eventFixture{}, false
	}
	version, err := strconv.Atoi(digits)
	if err != nil || version < 1 {
		return eventFixture{}, false
	}
	return eventFixture{typ: typ, version: version, kind: kind, name: name}, true
}

// cutLast splits a dotted name around its last dot.
func cutLast(s string) (before, after string, ok bool) {
	i := strings.LastIndexByte(s, '.')
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// loadEventFixtures reads the directory and refuses anything it cannot name: a
// stray file in this tree is a fixture nobody checks, which is worse than no
// fixture at all.
func loadEventFixtures(t *testing.T) []eventFixture {
	t.Helper()
	entries, err := os.ReadDir(eventFixtureDir())
	if err != nil {
		t.Fatalf("read %s: %v", eventFixtureDir(), err)
	}
	fixtures := make([]eventFixture, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case entry.IsDir():
			t.Errorf("%s: the payload fixture tree is flat, it holds no directories", name)
		case name == "README.md":
		default:
			fx, ok := parseEventFixtureName(name)
			if !ok {
				t.Errorf("%s: a payload fixture is named <type>.v<n>.valid.json or <type>.v<n>.invalid.json", name)
				continue
			}
			fixtures = append(fixtures, fx)
		}
	}
	if len(fixtures) == 0 {
		t.Fatalf("%s holds no payload fixtures", eventFixtureDir())
	}
	return fixtures
}

// TestEventFixturesComeInPairs is the rule that keeps the tree honest. A type
// with only a valid payload proves that the happy path parses and nothing
// more; the invalid half is the one that shows the schema actually refuses
// something (T-214 DoD).
func TestEventFixturesComeInPairs(t *testing.T) {
	halves := map[string][]string{}
	for _, fx := range loadEventFixtures(t) {
		halves[fx.typ] = append(halves[fx.typ], fx.kind)
	}
	for _, typ := range slices.Sorted(maps.Keys(halves)) {
		for _, want := range []string{fixtureValid, fixtureInvalid} {
			if !slices.Contains(halves[typ], want) {
				t.Errorf("%s: no %s payload fixture", typ, want)
			}
		}
	}
}

// TestEventFixturesBelongToRegisteredTypes ties a file name to the registry.
// A fixture of a type nobody registered is dead weight, and one written
// against a schema version the registry has moved past is worse: it keeps
// passing against a file the platform no longer uses.
func TestEventFixturesBelongToRegisteredTypes(t *testing.T) {
	for _, fx := range loadEventFixtures(t) {
		spec, ok := contracts.Lookup(fx.typ)
		switch {
		case !ok:
			t.Errorf("%s: %s is not a registered event type", fx.name, fx.typ)
		case spec.Deprecated:
			t.Errorf("%s: %s is a legacy type and carries no payload schema", fx.name, fx.typ)
		case spec.SchemaVersion != fx.version:
			t.Errorf("%s: written against schema version %d, the registry is at %d",
				fx.name, fx.version, spec.SchemaVersion)
		}
	}
}

// TestValidFixturesReachAHandler reads every valid payload the way a consumer
// does: through the bus, with validation on read in its default position. What
// the fixture proves is not that the JSON parses but that nothing between the
// publisher and the handler objects to it.
func TestValidFixturesReachAHandler(t *testing.T) {
	for _, fx := range loadEventFixtures(t) {
		if fx.kind != fixtureValid {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			spec, ev := eventForFixture(t, fx)
			if err := contracts.Validate(ev); err != nil {
				t.Fatalf("the registry refused a payload meant to be valid: %v", err)
			}
			delivered, letters := readBack(t, spec, ev, false)
			if len(letters) != 0 {
				t.Fatalf("parked in dead letters: %s", letters[0].Error)
			}
			if !slices.Contains(delivered, ev.ID) {
				t.Fatalf("the handler saw %v, want the event %q", delivered, ev.ID)
			}
		})
	}
}

// TestInvalidFixturesAreParkedOnRead is the half that carries the weight. Each
// invalid payload must be refused as a payload — not as a broken envelope, not
// as an unknown type — and must never reach a handler.
func TestInvalidFixturesAreParkedOnRead(t *testing.T) {
	for _, fx := range loadEventFixtures(t) {
		if fx.kind != fixtureInvalid {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			spec, ev := eventForFixture(t, fx)
			err := contracts.Validate(ev)
			if !errors.Is(err, contracts.ErrInvalidPayload) {
				t.Fatalf("error %v, want an invalid payload: the fixture has to fail on its payload "+
					"and on nothing else, or it stops testing the schema", err)
			}

			delivered, letters := readBack(t, spec, ev, false)
			if len(delivered) != 0 {
				t.Fatalf("the handler saw %v, want nothing: an invalid payload must not be delivered", delivered)
			}
			if len(letters) != 1 {
				t.Fatalf("%d dead letters, want exactly one", len(letters))
			}
			if letters[0].Attempts != 0 {
				t.Errorf("attempts = %d, want 0: the event was rejected before the handler, not by it",
					letters[0].Attempts)
			}
			if !strings.Contains(letters[0].Error, contracts.ErrInvalidPayload.Error()) {
				t.Errorf("dead letter reads %q, want the invalid payload of %s", letters[0].Error, fx.typ)
			}
		})
	}
}

// TestInvalidFixturesPassWhenValidationOnReadIsOff reads the same payloads
// with the flag in its other position. Without it the test above would prove
// only that the events never arrived — it could not tell a schema doing its
// job from a bus that dropped them.
func TestInvalidFixturesPassWhenValidationOnReadIsOff(t *testing.T) {
	for _, fx := range loadEventFixtures(t) {
		if fx.kind != fixtureInvalid {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			spec, ev := eventForFixture(t, fx)
			delivered, letters := readBack(t, spec, ev, true)
			if !slices.Contains(delivered, ev.ID) {
				t.Fatalf("the handler saw %v with validation on read off, want the event %q", delivered, ev.ID)
			}
			if len(letters) != 0 {
				t.Fatalf("%d dead letters with validation on read off, want none", len(letters))
			}
		})
	}
}

// eventForFixture wraps a payload in the envelope its type demands. Source,
// topic and policy come from the registry so that adding a type here needs no
// change to this file.
func eventForFixture(t *testing.T, fx eventFixture) (contracts.Spec, eventbus.Event) {
	t.Helper()
	spec, ok := contracts.Lookup(fx.typ)
	if !ok {
		t.Fatalf("%s: %s is not a registered event type", fx.name, fx.typ)
	}
	if len(spec.Publishers) == 0 {
		t.Fatalf("%s: the registry names no publisher, so no envelope can be built", fx.typ)
	}

	var payload map[string]any
	if err := json.Unmarshal(readLF(t, fx.path()), &payload); err != nil {
		t.Fatalf("decode %s: %v", fx.name, err)
	}

	actorKind := eventbus.ActorSystem
	if kinds := spec.Policy.ActorKinds; len(kinds) > 0 && !slices.Contains(kinds, actorKind) {
		actorKind = kinds[0]
	}
	scope := &eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"}
	var opts []eventbus.DeriveOption
	if spec.Policy.Agent == eventbus.AgentRequired {
		opts = append(opts, eventbus.WithAgent(eventbus.AgentRef{
			ID:               "agent-fixture-1",
			Level:            "task",
			Blueprint:        "encounter-wolf",
			BlueprintVersion: "1.0",
		}))
	}
	ev := eventbus.NewRoot(fx.typ, spec.Publishers[0], worldID, scope, actorKind, payload, opts...)
	return spec, ev
}

// readBack puts an event into its topic behind the publisher's back and reads
// it out again, returning what the handler saw and what was parked.
//
// Append rather than Publish on purpose: publication validates too, so an
// invalid payload could never be published, and the read side would then be
// tested against messages that cannot exist. A broker holds whatever was
// written to it — including what an older, laxer publisher wrote — and that is
// exactly the case validation on read defends against.
func readBack(t *testing.T, spec contracts.Spec, ev eventbus.Event, skipValidateOnRead bool) ([]string, []eventbus.DeadLetter) {
	t.Helper()
	topics := make([]string, 0, 8)
	for _, topic := range contracts.Topics() {
		topics = append(topics, topic.Name)
	}
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })

	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal %s: %v", ev.Type, err)
	}
	if err := bus.Append(spec.Topic, body); err != nil {
		t.Fatalf("append to %s: %v", spec.Topic, err)
	}

	reader := bus
	if skipValidateOnRead {
		reader = bus.Lenient()
	}
	var delivered []string
	if _, err := reader.ReadRange(t.Context(), spec.Topic, 0, 1,
		func(_ context.Context, got eventbus.Event) error {
			delivered = append(delivered, got.ID)
			return nil
		}); err != nil {
		t.Fatalf("read %s: %v", spec.Topic, err)
	}
	letters, err := bus.DeadLetters()
	if err != nil {
		t.Fatalf("dead letters: %v", err)
	}
	return delivered, letters
}

// The two dictionaries of the LLM record (T-215)
//
// A pair of payload fixtures can carry one refused document per type, and the
// decision of consolidation 3 is about two whole dictionaries: the six values
// of validation_status and the ten reasons of a refusal. So the fixtures below
// are checked against the schema files themselves rather than through a
// payload — a seventh status added to llm.output.v1.json would pass every
// fixture in the tree and still break the report of mvctl llm-usage and the
// projection of anyone who switches on the value.
//
// The lists are written out here on purpose. Reading them from the schema and
// comparing the schema to itself would prove nothing; what is under test is
// that the file says what C-07 v1.2 and ADR-017 addendum 1 decided.

// validationStatuses are the six values of llm.output.validation_status: the
// outcome of the pipeline, never the reason of a refusal (C-07 v1.2). The
// rejected_* values and budget_exceeded that data-model.md §7.2 v0.2 listed
// are gone — they were never published, and budget_exceeded is not an outcome
// at all: it is an llm.output.rejected with no llm.output beside it.
var validationStatuses = []string{
	"valid", "partially_rejected", "invalid", "error", "quarantined", "filter_error",
}

// rejectionReasons are the ten values of llm.output.rejected.reason
// (ADR-017 p. 3). llm.output.reasons[] denormalises the same dictionary so
// that a usage report needs no join over llm_output.event.id.
var rejectionReasons = []string{
	"unknown_entity", "player_agency", "level_violation", "schema_invalid", "language",
	"filter_blocked", "filter_error", "budget_exceeded", "law_violation", "other",
}

// guardianReasonsFile is where T-217 puts the same dictionary in Go. Until it
// exists the test below says what it will be held to.
func guardianReasonsFile() string { return repoPath("internal", "llm", "guardian", "reasons.go") }

// TestValidationStatusIsTheSixValuesOfTheDecision holds llm.output.v1.json to
// the enum consolidation 3 settled on.
func TestValidationStatusIsTheSixValuesOfTheDecision(t *testing.T) {
	got := schemaEnum(t, "llm.output.v1.json", "properties", "validation_status", "enum")
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(validationStatuses))) {
		t.Errorf("validation_status = %v, want the six values of C-07 v1.2 %v", got, validationStatuses)
	}
}

// TestRejectionReasonsAreOneDictionary checks that the reason of a refusal and
// its denormalised copy in the record cannot drift apart. They are two files,
// filled by the same gateway, and a report that groups by reasons[] would
// quietly lose a value the other side still publishes.
func TestRejectionReasonsAreOneDictionary(t *testing.T) {
	reason := schemaEnum(t, "llm.output.rejected.v1.json", "properties", "reason", "enum")
	if !slices.Equal(slices.Sorted(slices.Values(reason)), slices.Sorted(slices.Values(rejectionReasons))) {
		t.Errorf("reason = %v, want the ten values of ADR-017 p. 3 %v", reason, rejectionReasons)
	}
	reasons := schemaEnum(t, "llm.output.v1.json", "properties", "reasons", "items", "enum")
	if !slices.Equal(reason, reasons) {
		t.Errorf("llm.output.reasons[] = %v, llm.output.rejected.reason = %v: one dictionary, two files",
			reasons, reason)
	}
}

// TestRejectionReasonsMatchTheGuardianPackage is the equality the DoD of T-215
// asks for and the package it compares against does not exist yet (T-217, unit
// C2). It is written now rather than left as a comment because the moment
// internal/llm/guardian/reasons.go appears the two dictionaries can differ
// silently: the guardian decides the reason, the schema decides whether the
// event carrying it is publishable, and nothing else connects them.
//
// The file is read as source rather than imported, so that this test needs no
// change when the package lands. Every string literal in it is taken for a
// reason, which holds as long as reasons.go holds the dictionary and nothing
// else; if that stops being true, this test fails and whoever widened the file
// decides where the comparison belongs — most likely inside the package.
func TestRejectionReasonsMatchTheGuardianPackage(t *testing.T) {
	path := guardianReasonsFile()
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("%s does not exist yet (T-217). When it does, the reasons it declares must be "+
			"exactly the enum of llm.output.rejected.reason: %s",
			filepath.ToSlash(path), strings.Join(rejectionReasons, ", "))
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var declared []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err == nil && !slices.Contains(declared, value) {
			declared = append(declared, value)
		}
		return true
	})
	want := schemaEnum(t, "llm.output.rejected.v1.json", "properties", "reason", "enum")
	if !slices.Equal(slices.Sorted(slices.Values(declared)), slices.Sorted(slices.Values(want))) {
		t.Errorf("guardian declares %v, the schema of llm.output.rejected accepts %v", declared, want)
	}
}

// schemaEnum reads one enum out of a payload schema by the path of its keys.
func schemaEnum(t *testing.T, file string, path ...string) []string {
	t.Helper()
	var node any
	if err := json.Unmarshal(readLF(t, repoPath("schemas", "events", file)), &node); err != nil {
		t.Fatalf("decode %s: %v", file, err)
	}
	for i, key := range path {
		object, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s: %s is not an object", file, strings.Join(path[:i], "."))
		}
		if node, ok = object[key]; !ok {
			t.Fatalf("%s: no %s", file, strings.Join(path[:i+1], "."))
		}
	}
	values, ok := node.([]any)
	if !ok {
		t.Fatalf("%s: %s is not a list", file, strings.Join(path, "."))
	}
	enum := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("%s: %s holds %v, want a string", file, strings.Join(path, "."), value)
		}
		enum = append(enum, text)
	}
	return enum
}
