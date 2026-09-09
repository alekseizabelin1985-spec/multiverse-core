package contracts

import (
	"encoding/json"
	"errors"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/schemas"
	"multiverse-core.io/shared/eventbus"
)

// TestSchemasValid compiles every embedded schema as JSON Schema 2020-12
// (ADR-007). A schema that does not compile fails the build of the registry,
// so this is the test that keeps Default() from panicking in production.
func TestSchemasValid(t *testing.T) {
	reg, err := New(schemas.FS)
	if err != nil {
		t.Fatalf("the embedded schemas do not compile: %v", err)
	}
	for _, spec := range reg.All() {
		if spec.Deprecated {
			if spec.Schema != nil {
				t.Errorf("%s: a deprecated type must carry no payload schema", spec.Type)
			}
			continue
		}
		if spec.Schema == nil {
			t.Errorf("%s: no compiled schema", spec.Type)
		}
	}
}

// TestNoPhantoms checks the registry against the schema directory in both
// directions: a type without a file cannot be validated, a file without a type
// cannot be published (foundation.md §6).
func TestNoPhantoms(t *testing.T) {
	files, err := fs.Glob(schemas.FS, "events/*.json")
	if err != nil {
		t.Fatalf("list schemas: %v", err)
	}

	fileOf := map[string]string{}
	versioned := regexp.MustCompile(`^(.+)\.v(\d+)\.json$`)
	for _, file := range files {
		name := strings.TrimPrefix(file, "events/")
		if strings.HasPrefix(name, "_") {
			continue // _common.json and _envelope.json belong to no type
		}
		match := versioned.FindStringSubmatch(name)
		if match == nil {
			t.Errorf("%s: a schema file is named <type>.v<n>.json", file)
			continue
		}
		fileOf[match[1]] = name
	}

	reg := Default()
	for _, spec := range reg.All() {
		if spec.Deprecated {
			continue
		}
		want := SchemaFile(spec.Type, spec.SchemaVersion)
		if fileOf[spec.Type] != want {
			t.Errorf("%s: registered without the schema file %s", spec.Type, want)
		}
		delete(fileOf, spec.Type)
	}
	for typ, file := range fileOf {
		t.Errorf("%s: schema file of a type nobody registered (%s)", file, typ)
	}
}

// TestEveryTypeHasPublisherAndConsumer is the registry half of the contracts
// job (contracts.md §16 p. 4, p. 7): a type nobody may publish is dead, a type
// nobody reads is noise.
func TestEveryTypeHasPublisherAndConsumer(t *testing.T) {
	for _, spec := range All() {
		if len(spec.Publishers) == 0 {
			t.Errorf("%s: empty Publishers", spec.Type)
		}
		if len(spec.Consumers) == 0 {
			t.Errorf("%s: empty Consumers", spec.Type)
		}
		if spec.Owner == "" {
			t.Errorf("%s: no schema owner", spec.Type)
		}
		if spec.Topic == "" || spec.SchemaVersion < 1 {
			t.Errorf("%s: incomplete TypeSpec %+v", spec.Type, spec.TypeSpec)
		}
	}
}

// TestPublishersOfSharedTypes pins the types of MVP-1 that have more than one
// publisher (contracts.md §0 v0.4). The owner of the type is not necessarily
// among them: the encounter agent of EPIC-003 publishes dice.rolled, which
// belongs to EPIC-002.
func TestPublishersOfSharedTypes(t *testing.T) {
	cases := map[string]struct {
		owner      string
		publishers []string
	}{
		"dice.rolled": {OwnerState, []string{SourceSwarm, SourceTestkitSwarm}},
		"entity.create.proposed": {OwnerState, []string{
			SourceGateway, SourceSwarm, SourceMvctl, SourceTestkitSwarm, SourceTestkitGateway,
		}},
		"entity.update.proposed": {OwnerState, []string{
			SourceGateway, SourceSwarm, SourceMvctl, SourceTestkitSwarm, SourceTestkitGateway,
		}},
		"analytics.replay.completed": {OwnerState, []string{SourceState, SourceMvctl, SourceTestkitState}},
		"snapshot.created":           {OwnerState, []string{SourceState, SourceSwarm, SourceGateway}},
	}
	for typ, want := range cases {
		spec, ok := Lookup(typ)
		if !ok {
			t.Errorf("%s is not registered", typ)
			continue
		}
		if spec.Owner != want.owner {
			t.Errorf("%s: owner %s, want %s", typ, spec.Owner, want.owner)
		}
		if !slices.Equal(spec.Publishers, want.publishers) {
			t.Errorf("%s: publishers %v, want %v", typ, spec.Publishers, want.publishers)
		}
	}
	// encounter.started, the fifth type of the list, arrives with block "в"
	// (T-009) together with the rest of the swarm types.
	if _, ok := Lookup("encounter.started"); ok {
		t.Error("encounter.started is registered by T-009, not here: remove the note above")
	}
}

// TestLookupSatisfiesTheBus checks the shape the bus depends on: Lookup gives
// it the embedded eventbus.TypeSpec, so that the topic, the schema version and
// the topic policy cannot drift between the registry and the envelope.
func TestLookupSatisfiesTheBus(t *testing.T) {
	var reg eventbus.Registry = Default()

	spec, ok := reg.Lookup("player.attacked")
	if !ok {
		t.Fatal("player.attacked is not registered")
	}
	if spec.Topic != eventbus.TopicPlayerEvents {
		t.Errorf("topic %q, want %q", spec.Topic, eventbus.TopicPlayerEvents)
	}
	if err := spec.Policy.Check(newEvent(t, "player.attacked", eventbus.ActorSystem, nil)); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("player_events accepts actor_kind=system: %v", err)
	}
	if _, ok := reg.Lookup("no.such.type"); ok {
		t.Error("an unregistered type was found")
	}
}

func TestTopics(t *testing.T) {
	got := Topics()
	if len(got) != 8 {
		t.Fatalf("%d topics, want the 8 of api-contracts §2.2", len(got))
	}
	byName := map[string]TopicSpec{}
	for _, topic := range got {
		byName[topic.Name] = topic
	}
	analytics, ok := byName[eventbus.TopicAnalyticsEvents]
	if !ok {
		t.Fatal("analytics_events is missing")
	}
	if analytics.ReplayRead {
		t.Error("analytics_events must not be read during a replay (C-10)")
	}
	if analytics.RetentionMS != retention180 {
		t.Errorf("analytics retention %d ms, want 180 days", analytics.RetentionMS)
	}
	if records := byName[eventbus.TopicLLMRecords]; records.RetentionMS != retention90d {
		t.Errorf("llm_records retention %d ms, want 90 days", records.RetentionMS)
	}
	// Every registered type goes to one of the topics of the map.
	for _, spec := range All() {
		if _, ok := byName[spec.Topic]; !ok {
			t.Errorf("%s: topic %q is not in the topic map", spec.Type, spec.Topic)
		}
	}
}

// TestLegacyTypesCarryNoSchema documents the deal with the legacy profile:
// the as-is types are routable and their envelope is checked, nothing more
// (foundation.md §6). They go away with the profile at S5.
func TestLegacyTypesCarryNoSchema(t *testing.T) {
	spec, ok := Lookup("time.syncTime")
	if !ok {
		t.Fatal("time.syncTime is not registered")
	}
	if !spec.Deprecated || spec.Schema != nil {
		t.Errorf("time.syncTime: %+v, want deprecated without a schema", spec)
	}
	// Built by hand rather than with NewRoot: a legacy producer fills no meta
	// at all, which is exactly the envelope this type must survive.
	legacy := eventbus.Event{
		ID:        "legacy-1",
		Type:      "time.syncTime",
		Timestamp: time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC),
		Source:    SourceLegacy,
		Payload:   map[string]any{"anything": 1},
	}
	if err := Validate(legacy); err != nil {
		t.Errorf("a legacy event is rejected by the registry: %v", err)
	}
}

// TestLegacyEnvelopeIsStillChecked pins what "only the envelope is validated"
// means for a legacy type (foundation.md §6): the envelope rules of eventbus
// apply, they are not skipped. Validate is exported and reached directly by
// mvctl contracts check and by fixtures, without the bus in front of it.
func TestLegacyEnvelopeIsStillChecked(t *testing.T) {
	valid := eventbus.Event{
		ID:        "legacy-1",
		Type:      "time.syncTime",
		Timestamp: time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC),
		Source:    SourceLegacy,
		Payload:   map[string]any{"anything": 1},
	}
	cases := map[string]func(ev *eventbus.Event){
		"no id":        func(ev *eventbus.Event) { ev.ID = "" },
		"no source":    func(ev *eventbus.Event) { ev.Source = "" },
		"no timestamp": func(ev *eventbus.Event) { ev.Timestamp = time.Time{} },
		"half-filled meta": func(ev *eventbus.Event) {
			ev.Meta = eventbus.Meta{ActorKind: eventbus.ActorHuman, GMPath: eventbus.GMPathLegacy}
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			ev := valid
			breakIt(&ev)
			if err := Validate(ev); !errors.Is(err, eventbus.ErrInvalidEnvelope) {
				t.Fatalf("error %v, want an invalid envelope", err)
			}
		})
	}
}

// TestSpecsAreCopies is the sentinel of the rule the registry rests on: it is
// immutable after New. A caller that rewrites what All or Spec returned must
// not reach the package table, which several goroutines read at once.
func TestSpecsAreCopies(t *testing.T) {
	const typ = "entity.update.proposed"

	first, ok := Lookup(typ)
	if !ok {
		t.Fatalf("%s is not registered", typ)
	}
	wantPublishers := slices.Clone(first.Publishers)
	wantConsumers := slices.Clone(first.Consumers)
	first.Publishers[0] = "attacker"
	first.Consumers[0] = "attacker"

	second, _ := Lookup(typ)
	if !slices.Equal(second.Publishers, wantPublishers) {
		t.Errorf("Spec: publishers rewritten through the returned slice: %v", second.Publishers)
	}
	if !slices.Equal(second.Consumers, wantConsumers) {
		t.Errorf("Spec: consumers rewritten through the returned slice: %v", second.Consumers)
	}

	for _, spec := range All() {
		if spec.Type != typ {
			continue
		}
		spec.Publishers[0] = "attacker"
		if len(spec.Policy.ActorKinds) > 0 {
			spec.Policy.ActorKinds[0] = "attacker"
		}
	}
	third, _ := Lookup(typ)
	if !slices.Equal(third.Publishers, wantPublishers) {
		t.Errorf("All: publishers rewritten through the returned slice: %v", third.Publishers)
	}
	if len(third.Policy.ActorKinds) > 0 && third.Policy.ActorKinds[0] == "attacker" {
		t.Error("All: the topic policy was rewritten through the returned slice")
	}
}

// TestPlayerFamilySharesTheActionField guards the common shape of §2.3.1: one
// payload form for the whole player.* family. The gateway builds every one of
// them from POST /v1/players/{id}/actions and puts the hash of the
// idempotency key in action.key_hash; with additionalProperties false a schema
// that omits the field refuses its own publisher.
func TestPlayerFamilySharesTheActionField(t *testing.T) {
	seen := 0
	for _, spec := range All() {
		if spec.Deprecated || !strings.HasPrefix(spec.Type, "player.") {
			continue
		}
		seen++
		doc := readSchemaDoc(t, SchemaFile(spec.Type, spec.SchemaVersion))
		properties, _ := doc["properties"].(map[string]any)
		action, ok := properties["action"].(map[string]any)
		if !ok {
			t.Errorf("%s: no action field, api-contracts §2.3.1 gives it to every player.*", spec.Type)
			continue
		}
		fields, _ := action["properties"].(map[string]any)
		for _, name := range []string{"type", "key_hash"} {
			if _, ok := fields[name]; !ok {
				t.Errorf("%s: action.%s is missing", spec.Type, name)
			}
		}
	}
	if seen != 8 {
		t.Errorf("%d player.* types, want the 8 of api-contracts §2.3.1", seen)
	}
}

// readSchemaDoc decodes one embedded schema file as generic JSON, so that a
// test can assert on the schema itself and not only on what it accepts.
func readSchemaDoc(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := fs.ReadFile(schemas.FS, "events/"+name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc
}

func TestValidateUnknownType(t *testing.T) {
	ev := newEvent(t, "player.attacked", eventbus.ActorHuman, nil)
	ev.Type = "no.such.type"
	if err := Validate(ev); !errors.Is(err, eventbus.ErrUnknownType) {
		t.Errorf("error %v, want an unknown type", err)
	}
}
