// Package fixtures_test guards testdata/fixtures — the six entities of the
// dark forest and the seq 0 snapshot pointer that stand in for a real world
// until internal/state exists (state-and-mechanics.md §4.10, C-14 v1.1).
//
// A fixture is an input, not a document: bootstrap creates a world from these
// files, a recording replays against them, and every read-model consumer of
// MVP-1 reads the pointer before State can write one. So the numbers in them
// must be checked against their source rather than read as prose.
//
// The rule this file follows is that a number is asserted only where it is
// authored, and derived everywhere else. hp_max, atk, def, dmg and flee are
// derived from rules/dark-forest.yaml through mechanics.Rules.Stats — the test
// never repeats 10, 2, 12 or d6, it asks the rules; the loot table is derived
// the same way. The projections of the region (npc_ids, players_present) are
// derived from the positions of the entities. state_hash, size_bytes,
// entities_count and applied_proposals of the pointer are recomputed from the
// entities themselves. What is left — the identifiers, the weather, the
// respawn cooldown, encounter_chance and the character names — is authored
// content with no other source, and the test checks its shape, not its value.
package fixtures_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

const (
	worldID  = "dark-forest-world"
	regionID = "dark-forest-01"
	npcID    = "wolf-alpha"
)

// snapshotKey is §4.3: state/{YYYYMMDDTHHMMSSZ}-{seq:06d}.json. It is derived
// here rather than written down, because a key that is written down stops
// being tied to the moment it names: shifting taken_at while the file keeps
// its name left the pointer saying one date and the object another, and the
// test stayed green.
func snapshotKey(takenAt time.Time, seq int64) string {
	return fmt.Sprintf("state/%s-%06d.json", takenAt.UTC().Format("20060102T150405Z"), seq)
}

// fixtureFiles are the entity files in the order bootstrap reads them: a world
// before its regions, a region before what stands in it (§4.10).
var fixtureFiles = []string{"world.json", "region.json", "npc.json", "players.json"}

func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

// readLF reads a file as it is stored in Git. The repository normalises text
// to LF, but a checkout on Windows may hand it back with CRLF (core.autocrlf),
// and size_bytes must not depend on which machine cloned the fixtures.
func readLF(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
}

// decodeStrict fills dst from a fixture and refuses a key the model does not
// know: a misspelled field would otherwise decode into nothing at all and the
// fixture would silently lose an attribute.
func decodeStrict(t *testing.T, path string, dst any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(readLF(t, path)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		t.Fatalf("decode %s: want one JSON document, have more", path)
	}
}

// loadEntities reads the four entity files in bootstrap order.
func loadEntities(t *testing.T) []*entity.Entity {
	t.Helper()
	var all []*entity.Entity
	for _, name := range fixtureFiles {
		var batch []*entity.Entity
		decodeStrict(t, repoPath("testdata", "fixtures", name), &batch)
		if len(batch) == 0 {
			t.Fatalf("%s: no entities", name)
		}
		all = append(all, batch...)
	}
	return all
}

func loadRules(t *testing.T) *mechanics.Rules {
	t.Helper()
	rules, err := mechanics.Load(repoPath("rules", "dark-forest.yaml"))
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}
	return rules
}

func byID(entities []*entity.Entity, id string) *entity.Entity {
	for _, e := range entities {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// sortedByTypeID orders entities the way StateHash and a snapshot object do.
func sortedByTypeID(entities []*entity.Entity) []*entity.Entity {
	out := slices.Clone(entities)
	slices.SortFunc(out, func(a, b *entity.Entity) int {
		if a.Type != b.Type {
			return strings.Compare(a.Type, b.Type)
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// --- the entity files ---

func TestFixtureSetIsTheWorldBootstrapCreates(t *testing.T) {
	entities := loadEntities(t)

	want := []entity.Ref{
		{ID: worldID, Type: entity.TypeWorld},
		{ID: regionID, Type: entity.TypeRegion},
		{ID: npcID, Type: entity.TypeNPC},
		{ID: "player-A", Type: entity.TypePlayer},
		{ID: "player-B", Type: entity.TypePlayer},
		{ID: "player-C", Type: entity.TypePlayer},
	}
	have := make([]entity.Ref, 0, len(entities))
	for _, e := range entities {
		have = append(have, e.Ref())
	}
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("fixture set: have %v, want %v", have, want)
	}

	for _, e := range entities {
		t.Run(e.ID, func(t *testing.T) {
			if e.SchemaVersion != entity.SchemaVersion {
				t.Errorf("schema_version %d, want %d", e.SchemaVersion, entity.SchemaVersion)
			}
			if e.WorldID != worldID {
				t.Errorf("world_id %q, want %q", e.WorldID, worldID)
			}
			if e.Version != 1 {
				t.Errorf("version %d: a fixture is an entity right after its create.proposed", e.Version)
			}
			if e.Name == "" {
				t.Error("no name")
			}
			if e.CreatedAt.IsZero() || !e.CreatedAt.Equal(e.UpdatedAt) {
				t.Errorf("created_at %s / updated_at %s: nothing has touched a fixture yet",
					e.CreatedAt, e.UpdatedAt)
			}
			// A bootstrap fixture carries no trace of a fact, because no fact
			// has been published for it yet: State fills these in when it
			// applies the proposal (state-and-mechanics.md §4.5 p. 10, 12).
			if e.LastEventID != "" || e.LastChange != nil || len(e.History) != 0 {
				t.Errorf("last_event_id=%q last_change=%v history=%d: a fixture predates its facts",
					e.LastEventID, e.LastChange, len(e.History))
			}
		})
	}
}

func TestRulesDescribeThisWorld(t *testing.T) {
	rules := loadRules(t)
	if rules.World != worldID {
		t.Fatalf("rules/dark-forest.yaml describes world %q, fixtures build %q", rules.World, worldID)
	}
}

// TestCombatStatsComeFromTheRules is the check T-016 exists for: an entity is
// the truth about state and the rules are the truth about numbers, so the two
// are written twice on purpose (§4.10) — and a fixture that drifts from
// rules/dark-forest.yaml makes every golden number of the wave wrong.
//
// The whole actor is compared rather than five fields, so a stat added to the
// rules later cannot pass unchecked.
func TestCombatStatsComeFromTheRules(t *testing.T) {
	entities := loadEntities(t)
	rules := loadRules(t)

	for _, e := range entities {
		if e.Type != entity.TypePlayer && e.Type != entity.TypeNPC {
			continue
		}
		t.Run(e.ID, func(t *testing.T) {
			kind := entity.TypePlayer
			if e.Type == entity.TypeNPC {
				var ok bool
				if kind, ok = e.Kind(); !ok {
					t.Fatal("npc without kind: nothing to look up in the rules")
				}
			}
			want, ok := rules.Stats(kind)
			if !ok {
				t.Fatalf("the rules describe no %q", kind)
			}
			// The identity and the version are the entity's, not the rules'.
			want.ID, want.Version = e.ID, e.Version

			have, err := mechanics.ActorFromEntity(e, nil)
			if err != nil {
				t.Fatalf("read the fighter out of the fixture: %v", err)
			}
			if !reflect.DeepEqual(*have, want) {
				t.Errorf("stats of %s\nfixture: %+v\nrules:   %+v", e.ID, *have, want)
			}
		})
	}
}

func TestNPCLootComesFromTheRules(t *testing.T) {
	npc := byID(loadEntities(t), npcID)
	kind, _ := npc.Kind()

	want := make([]entity.LootEntry, 0)
	for _, item := range loadRules(t).Loot(kind) {
		want = append(want, entity.LootEntry{ItemKind: item.Kind, Name: item.Name})
	}
	if len(want) == 0 {
		t.Fatalf("the rules give %q no loot: the fixture would have nothing to derive", kind)
	}

	have, err := npc.Loot()
	if err != nil {
		t.Fatalf("loot of %s: %v", npc.ID, err)
	}
	if !reflect.DeepEqual(have, want) {
		t.Errorf("loot of %s: fixture %+v, rules %+v", npc.ID, have, want)
	}
}

// TestRegionProjectionsFollowFromPositions checks the two derived lists of the
// region (data-model.md §3.2): both are projections of where the entities
// stand, and inv-10 compares them with the positions themselves.
func TestRegionProjectionsFollowFromPositions(t *testing.T) {
	entities := loadEntities(t)
	region := byID(entities, regionID)

	var wantNPCs, wantPlayers []string
	for _, e := range entities {
		position, ok := e.Position()
		if !ok || position != regionID {
			continue
		}
		switch e.Type {
		case entity.TypeNPC:
			wantNPCs = append(wantNPCs, e.ID)
		case entity.TypePlayer:
			wantPlayers = append(wantPlayers, e.ID)
		}
	}
	slices.Sort(wantNPCs)
	slices.Sort(wantPlayers)

	haveNPCs, ok := region.NPCIDs()
	if !ok {
		t.Fatal("region has no npc_ids")
	}
	slices.Sort(haveNPCs)
	if !slices.Equal(haveNPCs, wantNPCs) {
		t.Errorf("npc_ids %v, but the NPCs standing in %s are %v", haveNPCs, regionID, wantNPCs)
	}

	havePlayers, ok := region.PlayersPresent()
	if !ok {
		t.Fatal("region has no players_present")
	}
	slices.Sort(havePlayers)
	if !slices.Equal(havePlayers, wantPlayers) {
		t.Errorf("players_present %v, but the characters standing in %s are %v",
			havePlayers, regionID, wantPlayers)
	}
	if len(havePlayers) != 0 {
		t.Error("the fixture world starts with everybody outside the region")
	}
}

// TestRegionAuthoredNumbers covers what the region alone decides. respawn_ttl
// is fixed by §4.10; encounter_chance has no source in the rules yet (the
// blueprint domain-dark-forest of EPIC-003 will own the number), so only its
// shape is asserted — a probability, not a percentage and not a certainty.
func TestRegionAuthoredNumbers(t *testing.T) {
	region := byID(loadEntities(t), regionID)

	ttl, ok := region.RespawnTTL()
	if !ok {
		t.Fatal("region has no respawn_ttl")
	}
	if ttl != 24*time.Hour {
		t.Errorf("respawn_ttl %s, want 24h (state-and-mechanics.md §4.10)", ttl)
	}

	chance, ok := region.EncounterChance()
	if !ok {
		t.Fatal("region has no encounter_chance")
	}
	if chance <= 0 || chance > 1 {
		t.Errorf("encounter_chance %v: want a probability in (0, 1]", chance)
	}

	if description, ok := region.Description(); !ok || description == "" {
		t.Error("region has no authored description")
	}
	if ref, ok := region.BlueprintRef(); !ok || ref == "" {
		t.Error("region has no blueprint_ref")
	}
}

// TestPlayersStartOutsideInTheirOwnScope covers the three characters of every
// CI session: outside the region, each alone in its own solo scope, driven by
// a machine (§4.10, data-model.md §3.3).
func TestPlayersStartOutsideInTheirOwnScope(t *testing.T) {
	for _, e := range loadEntities(t) {
		if e.Type != entity.TypePlayer {
			continue
		}
		t.Run(e.ID, func(t *testing.T) {
			if kind, _ := e.ActorKind(); kind != entity.ActorKindCI {
				t.Errorf("actor_kind %q, want %q: a fixture character is never a person",
					kind, entity.ActorKindCI)
			}
			if position, _ := e.Position(); position != "outside:"+worldID {
				t.Errorf("position %q, want %q", position, "outside:"+worldID)
			}
			scope, ok := e.Scope()
			if !ok {
				t.Fatal("no scope")
			}
			if want := (eventbus.ScopeRef{ID: e.ID, Type: "solo"}); scope != want {
				t.Errorf("scope %+v, want %+v", scope, want)
			}
			// entity.Scope also reads the shorthand "solo:{id}" of the
			// blueprints, so the value has to be checked as it is stored: the
			// canonical form inside an entity is the object of _common.json,
			// and the shorthand would move state_hash without moving meaning
			// (journal 2026-09-10, decision 2 on T-011).
			if raw, _ := e.Attr(entity.AttrScope); reflect.ValueOf(raw).Kind() != reflect.Map {
				t.Errorf("scope is stored as %T: an entity carries the object {id, type}", raw)
			}
			if status, _ := e.Status(); status != entity.StatusAlive {
				t.Errorf("status %q, want %q", status, entity.StatusAlive)
			}
			inventory, err := e.Inventory()
			if err != nil {
				t.Fatalf("inventory: %v", err)
			}
			if len(inventory) != 0 {
				t.Errorf("inventory holds %d items: a character starts empty-handed", len(inventory))
			}
			if group, ok := e.GroupID(); ok && group != "" {
				t.Errorf("group_id %q: the fixture characters start solo", group)
			}
		})
	}
}

func TestWorldCarriesTheAttributesOfItsLaws(t *testing.T) {
	world := byID(loadEntities(t), worldID)

	if version, _ := world.LawsVersion(); version != "v1" {
		t.Errorf("laws_version %q, want v1", version)
	}
	if weather, _ := world.Weather(); !slices.Contains(
		[]string{"clear", "cloudy", "rain", "storm", "fog"}, weather) {
		t.Errorf("weather %q is outside the dictionary of data-model.md §3.1", weather)
	}
	if part, _ := world.TimeOfDay(); !slices.Contains(
		[]string{"dawn", "day", "dusk", "night"}, part) {
		t.Errorf("time_of_day %q is outside the dictionary of data-model.md §3.1", part)
	}
	if day, ok := world.Day(); !ok || day < 1 {
		t.Errorf("day %d: the count starts at 1", day)
	}
	if locale, _ := world.Locale(); locale != "ru" {
		t.Errorf("locale %q, want ru", locale)
	}
	if season, ok := world.Season(); !ok || season == "" {
		t.Error("world has no season")
	}
	if ref, ok := world.BlueprintRef(); !ok || ref == "" {
		t.Error("world has no blueprint_ref")
	}
}

// --- the seq 0 snapshot ---

// snapshotMeta is the block §4.4 puts both into the pointer and into the
// object it points at. size_bytes is a pointer because only latest.json can
// carry it: a snapshot cannot state its own length without changing it.
type snapshotMeta struct {
	ID            string           `json:"id"`
	Seq           int64            `json:"seq"`
	Key           string           `json:"key"`
	TakenAt       time.Time        `json:"taken_at"`
	Cursor        map[string]int64 `json:"cursor"`
	LawsVersion   string           `json:"laws_version"`
	RulesVersion  string           `json:"rules_version"`
	StateHash     string           `json:"state_hash"`
	SizeBytes     *int64           `json:"size_bytes,omitempty"`
	EntitiesCount int              `json:"entities_count"`
	Reason        string           `json:"reason"`
}

type latestPointer struct {
	SchemaVersion int    `json:"schema_version"`
	Component     string `json:"component"`
	World         struct {
		Entity entity.Ref `json:"entity"`
	} `json:"world"`
	Snapshot  snapshotMeta `json:"snapshot"`
	WrittenAt time.Time    `json:"written_at"`
	Writer    string       `json:"writer"`
}

type snapshotObject struct {
	SchemaVersion    int              `json:"schema_version"`
	Component        string           `json:"component"`
	Snapshot         snapshotMeta     `json:"snapshot"`
	WorldID          string           `json:"world_id"`
	Entities         []*entity.Entity `json:"entities"`
	AppliedProposals []string         `json:"applied_proposals"`
}

func snapshotDir() string { return repoPath("testdata", "fixtures", "snapshots", "state") }

func loadPointer(t *testing.T) latestPointer {
	t.Helper()
	var pointer latestPointer
	decodeStrict(t, filepath.Join(snapshotDir(), "latest.json"), &pointer)
	return pointer
}

func loadSnapshot(t *testing.T) ([]byte, snapshotObject) {
	t.Helper()
	// The object is reached the way a consumer reaches it: through the key the
	// pointer gives, not through a path this test knows (C-14 v1.1, §4.4).
	path := filepath.Join(snapshotDir(), filepath.Base(loadPointer(t).Snapshot.Key))
	var object snapshotObject
	decodeStrict(t, path, &object)
	return readLF(t, path), object
}

// TestLatestPointerDescribesSnapshotZero recomputes every number of the
// pointer from the fixtures it describes. Nothing here is a literal that was
// measured once by hand: state_hash comes from entity.StateHash, size_bytes
// from the object on disk, entities_count from the entity files.
func TestLatestPointerDescribesSnapshotZero(t *testing.T) {
	entities := loadEntities(t)
	pointer := loadPointer(t)
	raw, _ := loadSnapshot(t)

	if pointer.SchemaVersion != 1 {
		t.Errorf("schema_version %d, want 1", pointer.SchemaVersion)
	}
	if pointer.Component != "state" {
		t.Errorf("component %q, want state (C-14 v1.1)", pointer.Component)
	}
	if want := (entity.Ref{ID: worldID, Type: entity.TypeWorld}); pointer.World.Entity != want {
		t.Errorf("world %+v, want %+v", pointer.World.Entity, want)
	}

	s := pointer.Snapshot
	if s.Seq != 0 {
		t.Errorf("seq %d, want 0: this is the snapshot bootstrap writes", s.Seq)
	}
	if want := fmt.Sprintf("state:%s:%06d", worldID, s.Seq); s.ID != want {
		t.Errorf("id %q, want %q", s.ID, want)
	}
	if want := snapshotKey(s.TakenAt, s.Seq); s.Key != want {
		t.Errorf("key %q, want %q: the key is taken_at and seq, not a name of its own", s.Key, want)
	}
	if s.Reason != "bootstrap" {
		t.Errorf("reason %q, want bootstrap (§4.9)", s.Reason)
	}
	if want := map[string]int64{"system_events": 0}; !maps.Equal(s.Cursor, want) {
		t.Errorf("cursor %v, want %v: nothing has been read from the journal yet", s.Cursor, want)
	}
	if s.EntitiesCount != len(entities) {
		t.Errorf("entities_count %d, but the fixtures hold %d entities", s.EntitiesCount, len(entities))
	}
	if want := entity.StateHash(entities); s.StateHash != want {
		t.Errorf("state_hash\n  file:      %s\n  recomputed: %s", s.StateHash, want)
	}
	if s.SizeBytes == nil {
		t.Fatal("no size_bytes: the pointer is what tells a consumer how big the object is")
	}
	if want := int64(len(raw)); *s.SizeBytes != want {
		t.Errorf("size_bytes %d, but %s is %d bytes", *s.SizeBytes, s.Key, want)
	}

	// The two versions the pointer carries are the versions of the files in
	// Git, not values of their own.
	if world := byID(entities, worldID); world != nil {
		if version, _ := world.LawsVersion(); s.LawsVersion != version {
			t.Errorf("laws_version %q, but the world entity says %q", s.LawsVersion, version)
		}
	}
	if version := loadRules(t).Version; s.RulesVersion != version {
		t.Errorf("rules_version %q, but rules/dark-forest.yaml says %q", s.RulesVersion, version)
	}

	for _, e := range entities {
		if s.TakenAt.Before(e.UpdatedAt) {
			t.Errorf("taken_at %s is before %s was last touched (%s)", s.TakenAt, e.ID, e.UpdatedAt)
		}
	}
	if pointer.WrittenAt.Before(s.TakenAt) {
		t.Errorf("written_at %s is before taken_at %s: the pointer is written after the object (§4.4)",
			pointer.WrittenAt, s.TakenAt)
	}
	if pointer.Writer == "" {
		t.Error("no writer")
	}
}

// TestSnapshotObjectHoldsTheSameWorld checks the object the pointer points at:
// a consumer reads latest.json, then the object, then verifies the hash (§4.4),
// and every one of those three steps has to work on the fixture.
func TestSnapshotObjectHoldsTheSameWorld(t *testing.T) {
	entities := loadEntities(t)
	pointer := loadPointer(t)
	_, object := loadSnapshot(t)

	if object.SchemaVersion != 1 || object.Component != "state" {
		t.Errorf("schema_version %d component %q, want 1 / state",
			object.SchemaVersion, object.Component)
	}
	if object.WorldID != worldID {
		t.Errorf("world_id %q, want %q", object.WorldID, worldID)
	}
	if object.Snapshot.SizeBytes != nil {
		t.Error("the object states its own size_bytes: only the pointer can (see the dev-log)")
	}

	// The metadata of the object and of the pointer are the same block.
	have, want := object.Snapshot, pointer.Snapshot
	want.SizeBytes = nil
	if !reflect.DeepEqual(have, want) {
		t.Errorf("snapshot block\n  object:  %+v\n  pointer: %+v", have, want)
	}

	// The entities of the object are the entities of the four files, in the
	// (type, id) order §4.4 fixes.
	inOrder := sortedByTypeID(entities)
	if !reflect.DeepEqual(object.Entities, inOrder) {
		t.Error("the snapshot object and the entity files describe different worlds")
		for i := range max(len(object.Entities), len(inOrder)) {
			switch {
			case i >= len(object.Entities):
				t.Logf("  %d: missing from the object, files have %s", i, inOrder[i].Ref())
			case i >= len(inOrder):
				t.Logf("  %d: only in the object: %s", i, object.Entities[i].Ref())
			case !reflect.DeepEqual(object.Entities[i], inOrder[i]):
				t.Logf("  %d differs\n    object: %s\n    files:  %s", i,
					entity.CanonicalJSON(object.Entities[i]), entity.CanonicalJSON(inOrder[i]))
			}
		}
	}
	if hash := entity.StateHash(object.Entities); hash != pointer.Snapshot.StateHash {
		t.Errorf("state_hash of the object %s, pointer %s", hash, pointer.Snapshot.StateHash)
	}

	// applied_proposals is what bootstrap published, and the identifier of a
	// bootstrap proposal is fixed by §4.10.
	wantProposals := make([]string, 0, len(entities))
	for _, e := range entities {
		wantProposals = append(wantProposals,
			fmt.Sprintf("bootstrap:%s:%s/%s", worldID, e.Type, e.ID))
	}
	if !slices.Equal(object.AppliedProposals, wantProposals) {
		t.Errorf("applied_proposals\n  object: %v\n  want:   %v", object.AppliedProposals, wantProposals)
	}
}

// TestPointerYieldsAValidSnapshotCreated publishes the pointer as the event it
// announces itself with. The payload schema is a strict subset of the pointer:
// snapshot.created.v1.json sets additionalProperties false and does not know
// rules_version, entities_count or reason, which §4.4 keeps in latest.json —
// so the three are removed here rather than smuggled in (see the dev-log).
func TestPointerYieldsAValidSnapshotCreated(t *testing.T) {
	var pointer map[string]any
	decodeStrict(t, filepath.Join(snapshotDir(), "latest.json"), &pointer)

	block, ok := pointer["snapshot"].(map[string]any)
	if !ok {
		t.Fatal("latest.json has no snapshot block")
	}
	payloadOnly := maps.Clone(block)
	for _, extra := range []string{"rules_version", "entities_count", "reason"} {
		if _, ok := payloadOnly[extra]; !ok {
			t.Errorf("latest.json lost %s: §4.4 keeps it in the pointer", extra)
		}
		delete(payloadOnly, extra)
	}

	event := eventbus.NewRoot("snapshot.created", contracts.SourceState, worldID, nil,
		entity.ActorKindCI, map[string]any{
			"component": pointer["component"],
			"snapshot":  payloadOnly,
		})
	if err := contracts.Validate(event); err != nil {
		t.Fatalf("snapshot.created built from latest.json is not valid: %v", err)
	}
}
