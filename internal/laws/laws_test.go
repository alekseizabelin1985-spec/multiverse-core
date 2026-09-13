package laws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
)

const world = "dark-forest-world"

// fakeVersions is the WorldView of the swarm as the laws see it.
type fakeVersions map[string]string

func (f fakeVersions) LawsVersion(worldID string) (string, bool) {
	v, ok := f[worldID]
	return v, ok
}

// spec describes a document the tests write.
type spec struct {
	version, status, createdBy, basedOn string
	laws                                []string
}

// render writes the YAML of a spec. A law is "inv-NN" (an invariant checked
// under its own id) or anything else (a declarative law).
func render(worldID string, s spec) string {
	if s.status == "" {
		s.status = StatusApproved
	}
	if s.createdBy == "" {
		s.createdBy = CreatedByAuthor
	}
	var b strings.Builder
	fmt.Fprintf(&b, "version: %s\nworld_id: %s\nstatus: %s\ncreated_by: %s\n", s.version, worldID, s.status, s.createdBy)
	if s.basedOn != "" {
		fmt.Fprintf(&b, "based_on: %s\n", s.basedOn)
	}
	b.WriteString("created_at: 2026-09-13T00:00:00Z\nlaws:\n")
	for _, id := range s.laws {
		if strings.HasPrefix(id, "inv-") {
			fmt.Fprintf(&b, "  - { id: %s, kind: invariant, check: %s, text: \"text of %s\" }\n", id, id, id)
		} else {
			fmt.Fprintf(&b, "  - { id: %s, kind: declarative, source: author, text: \"text of %s\" }\n", id, id)
		}
	}
	return b.String()
}

// lawsDir writes the specs into a fresh directory and returns it.
func lawsDir(t *testing.T, specs ...spec) string {
	t.Helper()
	dir := t.TempDir()
	for _, s := range specs {
		writeSpec(t, dir, s)
	}
	return dir
}

func writeSpec(t *testing.T, dir string, s spec) string {
	t.Helper()
	path := filepath.Join(dir, FileName(world, s.version))
	writeFile(t, path, render(world, s))
	return path
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newKeeper(t *testing.T, cfg Config) *Keeper {
	t.Helper()
	k, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

var (
	v1 = spec{version: "v1", laws: []string{"inv-01", "law-1", "law-2"}}
	v2 = spec{version: "v2", basedOn: "v1", laws: []string{"inv-01", "inv-02", "law-1", "law-3"}}
)

func TestCurrentComesFromTheStateThroughWorldVersions(t *testing.T) {
	dir := lawsDir(t, v1, v2)
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}, Versions: fakeVersions{world: "v1"}})
	doc, origin, err := k.CurrentFrom(world)
	if err != nil || doc.Version != "v1" || origin != OriginState {
		t.Errorf("CurrentFrom = %s from %s, %v; want v1 from the state although v2 is newer", doc.Version, origin, err)
	}
	if cur, err := k.Current(world); err != nil || cur.Version != "v1" {
		t.Errorf("Current = %s, %v; want v1", cur.Version, err)
	}
}

func TestCurrentWithoutStateIsTheNewestApprovedFile(t *testing.T) {
	v3 := spec{version: "v3", basedOn: "v2", status: StatusPendingReview, laws: []string{"inv-01"}}
	dir := lawsDir(t, v1, v2, v3)
	for name, versions := range map[string]WorldVersions{
		"nil":             nil,
		"world not known": fakeVersions{"other-world": "v1"},
	} {
		t.Run(name, func(t *testing.T) {
			k := newKeeper(t, Config{Source: FileSource{Dir: dir}, Versions: versions})
			doc, origin, err := k.CurrentFrom(world)
			if err != nil || doc.Version != "v2" || origin != OriginFiles {
				t.Errorf("CurrentFrom = %s from %s, %v; want v2 from the files (v3 is pending review)",
					doc.Version, origin, err)
			}
		})
	}
}

// The world runs under the version the state names; answering with another
// one would hand the guardian the wrong checks.
func TestCurrentOfAVersionTheSourceDoesNotHoldIsAnError(t *testing.T) {
	k := newKeeper(t, Config{Source: FileSource{Dir: lawsDir(t, v1)}, Versions: fakeVersions{world: "v7"}})
	if _, err := k.Current(world); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("Current = %v, want ErrUnknownVersion", err)
	}
}

func TestCurrentDoesNotStepOverANewerVersionThatDoesNotLoad(t *testing.T) {
	dir := lawsDir(t, v1)
	writeFile(t, filepath.Join(dir, FileName(world, "v2")), strings.Replace(render(world, v2), "check: inv-02", "check: nope", 1))
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})
	if _, err := k.Current(world); !errors.Is(err, ErrUnknownCheck) {
		t.Errorf("Current = %v, want the error of v2 (ErrUnknownCheck)", err)
	}
}

func TestCurrentOfAWorldWithoutAnApprovedVersion(t *testing.T) {
	dir := lawsDir(t, spec{version: "v1", status: StatusRolledBack, laws: []string{"law-1"}})
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})
	for _, w := range []string{world, "no-such-world"} {
		if _, err := k.Current(w); !errors.Is(err, ErrNoApprovedVersion) {
			t.Errorf("Current(%s) = %v, want ErrNoApprovedVersion", w, err)
		}
	}
}

func TestGet(t *testing.T) {
	k := newKeeper(t, Config{Source: FileSource{Dir: lawsDir(t, v1, v2)}})
	doc, err := k.Get(world, "v2")
	if err != nil || doc.BasedOn != "v1" || !slices.Equal(doc.IDs(), v2.laws) {
		t.Fatalf("Get(v2) = %+v, %v", doc, err)
	}
	doc.Laws[0].Text = "changed by the caller"
	if again, _ := k.Get(world, "v2"); again.Laws[0].Text == "changed by the caller" {
		t.Error("a caller changed what the keeper holds through the slice of laws")
	}
	for _, version := range []string{"v3", "latest", ""} {
		if _, err := k.Get(world, version); !errors.Is(err, ErrUnknownVersion) {
			t.Errorf("Get(%q) = %v, want ErrUnknownVersion", version, err)
		}
	}
	if _, err := k.Get("other-world", "v1"); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("Get of another world = %v, want ErrUnknownVersion", err)
	}
	if got := k.Worlds(); !slices.Equal(got, []string{world}) {
		t.Errorf("Worlds = %v", got)
	}
	if got := k.Versions(world); !slices.Equal(got, []string{"v1", "v2"}) {
		t.Errorf("Versions = %v", got)
	}
}

// DoD: an unknown check fails the load of its version and gives /health the
// mark unknown_check; the other versions still load.
func TestUnknownCheckFailsItsVersionAndDegradesHealth(t *testing.T) {
	dir := lawsDir(t, v1)
	bad := strings.Replace(render(world, v2), "check: inv-02", "check: no_such_check", 1)
	writeFile(t, filepath.Join(dir, FileName(world, "v2")), bad)
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})

	if _, err := k.Get(world, "v2"); !errors.Is(err, ErrUnknownCheck) {
		t.Errorf("Get(v2) = %v, want ErrUnknownCheck", err)
	}
	if _, err := k.Get(world, "v1"); err != nil {
		t.Errorf("Get(v1) = %v; a bad v2 must not take v1 down", err)
	}
	problems := k.Problems()
	if len(problems) != 1 || problems[0].Document != "dark-forest-world.v2.yaml" ||
		problems[0].World != world || problems[0].Version != "v2" {
		t.Fatalf("Problems = %+v", problems)
	}
	h := k.Health()
	if h.Status != runtime.StatusDegraded || h.Details["laws"] != HealthUnknownCheck {
		t.Errorf("Health = %+v, want degraded with laws=unknown_check", h)
	}
	if msgs, _ := h.Details["problems"].([]string); len(msgs) != 1 || !strings.Contains(msgs[0], "no_such_check") {
		t.Errorf("Health problems = %v", h.Details["problems"])
	}
}

func TestHealth(t *testing.T) {
	if h := newKeeper(t, Config{Source: FileSource{Dir: lawsDir(t, v1)}}).Health(); h.Status != runtime.StatusOK {
		t.Errorf("Health of good laws = %+v", h)
	}
	dir := lawsDir(t, v1)
	writeFile(t, filepath.Join(dir, FileName(world, "v2")), "version: [")
	h := newKeeper(t, Config{Source: FileSource{Dir: dir}}).Health()
	if h.Status != runtime.StatusDegraded || h.Details["laws"] != HealthInvalidDocument {
		t.Errorf("Health with a broken document = %+v, want degraded with laws=invalid_document", h)
	}
}

// One unknown check among several broken documents is enough for
// laws=unknown_check, wherever it stands among them; problems keeps them all.
func TestHealthSaysUnknownCheckWhenAnyDocumentNamesOne(t *testing.T) {
	unknown := strings.Replace(render(world, v2), "check: inv-02", "check: no_such_check", 1)
	for name, docs := range map[string][2]string{
		"unknown check first": {unknown, "version: ["},
		"unknown check last":  {"version: [", strings.Replace(unknown, "version: v2", "version: v3", 1)},
	} {
		t.Run(name, func(t *testing.T) {
			dir := lawsDir(t, v1)
			writeFile(t, filepath.Join(dir, FileName(world, "v2")), docs[0])
			writeFile(t, filepath.Join(dir, FileName(world, "v3")), docs[1])
			h := newKeeper(t, Config{Source: FileSource{Dir: dir}}).Health()
			if h.Status != runtime.StatusDegraded || h.Details["laws"] != HealthUnknownCheck {
				t.Errorf("Health = %+v, want degraded with laws=unknown_check", h)
			}
			if msgs, _ := h.Details["problems"].([]string); len(msgs) != 2 {
				t.Errorf("Health problems = %v, want both documents", h.Details["problems"])
			}
		})
	}
}

// countingSource counts the reads of the source it wraps.
type countingSource struct {
	Source
	reads int
}

func (s *countingSource) Documents(ctx context.Context) ([]Document, error) {
	s.reads++
	return s.Source.Documents(ctx)
}

// The state may name vN+1 before world.laws.changed has reached Handle: the
// two events travel in different topics. The document is on disk, so Current
// reads the source again instead of failing (review T-205 #1, Mi-1).
func TestCurrentReloadsForAVersionTheStateNamesFirst(t *testing.T) {
	dir := lawsDir(t, v1)
	versions := fakeVersions{world: "v1"}
	src := &countingSource{Source: FileSource{Dir: dir}}
	k := newKeeper(t, Config{Source: src, Versions: versions})

	writeSpec(t, dir, v2)
	versions[world] = "v2"
	doc, origin, err := k.CurrentFrom(world)
	if err != nil || doc.Version != "v2" || origin != OriginState {
		t.Fatalf("CurrentFrom = %s from %s, %v; want v2 from the state without Handle", doc.Version, origin, err)
	}
	if src.reads != 2 {
		t.Errorf("reads of the source = %d, want 2 (New and one reload)", src.reads)
	}
	if _, err := k.Current(world); err != nil || src.reads != 2 {
		t.Errorf("Current again = %v after %d reads; a held version needs no reload", err, src.reads)
	}
}

// A version the installation does not hold costs one reload, not one per call.
func TestCurrentReloadsOncePerMissingVersion(t *testing.T) {
	versions := fakeVersions{world: "v7"}
	src := &countingSource{Source: FileSource{Dir: lawsDir(t, v1)}}
	k := newKeeper(t, Config{Source: src, Versions: versions})
	for range 3 {
		if _, err := k.Current(world); !errors.Is(err, ErrUnknownVersion) {
			t.Fatalf("Current = %v, want ErrUnknownVersion", err)
		}
	}
	if src.reads != 2 {
		t.Errorf("reads of the source = %d, want 2 (New and one reload for v7)", src.reads)
	}
	versions[world] = "v8"
	if _, err := k.Current(world); !errors.Is(err, ErrUnknownVersion) || src.reads != 3 {
		t.Errorf("Current(v8) = %v after %d reads, want ErrUnknownVersion after 3", err, src.reads)
	}
	// The same version missing in another world is a miss of its own.
	versions["other-world"] = "v7"
	if _, err := k.Current("other-world"); !errors.Is(err, ErrUnknownVersion) || src.reads != 4 {
		t.Errorf("Current(other-world v7) = %v after %d reads, want ErrUnknownVersion after 4", err, src.reads)
	}
}

// A version the keeper holds but cannot load is not a miss: reading the same
// directory again would not fix the document.
func TestCurrentDoesNotReloadForAVersionThatDoesNotLoad(t *testing.T) {
	dir := lawsDir(t, v1)
	writeFile(t, filepath.Join(dir, FileName(world, "v2")), "version: [")
	src := &countingSource{Source: FileSource{Dir: dir}}
	k := newKeeper(t, Config{Source: src, Versions: fakeVersions{world: "v2"}})
	if _, err := k.Current(world); !errors.Is(err, ErrInvalidDocument) || src.reads != 1 {
		t.Errorf("Current = %v after %d reads, want ErrInvalidDocument after 1", err, src.reads)
	}
}

// A source that fails the reload leaves the answer ErrUnknownVersion.
func TestCurrentReloadThatFailsKeepsTheMiss(t *testing.T) {
	src := &flakySource{Source: FileSource{Dir: lawsDir(t, v1)}}
	k := newKeeper(t, Config{Source: src, Versions: fakeVersions{world: "v2"}})
	src.fail = true
	if _, err := k.Current(world); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("Current = %v, want ErrUnknownVersion", err)
	}
	if _, err := k.Get(world, "v1"); err != nil {
		t.Errorf("Get(v1) = %v; a failed reload must keep what the keeper holds", err)
	}
}

// flakySource fails once fail is set.
type flakySource struct {
	Source
	fail bool
}

func (s *flakySource) Documents(ctx context.Context) ([]Document, error) {
	if s.fail {
		return nil, errors.New("the laws directory is gone")
	}
	return s.Source.Documents(ctx)
}

func TestADocumentMustBeWhatItsNameSays(t *testing.T) {
	dir := lawsDir(t, v1)
	writeFile(t, filepath.Join(dir, FileName(world, "v2")), render(world, spec{version: "v3", basedOn: "v1", laws: []string{"law-1"}}))
	writeFile(t, filepath.Join(dir, "notes.yaml"), render(world, v2))
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})
	if _, err := k.Get(world, "v2"); !errors.Is(err, ErrInvalidDocument) {
		t.Errorf("Get(v2) of a file carrying v3 = %v, want ErrInvalidDocument", err)
	}
	if _, err := k.Get(world, "v3"); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("Get(v3) = %v; a version is loaded from its own name only", err)
	}
	names := []string{}
	for _, p := range k.Problems() {
		names = append(names, p.Document)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"dark-forest-world.v2.yaml", "notes.yaml"}) {
		t.Errorf("Problems name %v", names)
	}
}

// sliceSource is a source that is not a directory: two documents of one
// version, and one whose name the source could not read.
type sliceSource []Document

func (s sliceSource) Documents(context.Context) ([]Document, error) { return s, nil }

func TestTwoDocumentsOfOneVersionLoadNeither(t *testing.T) {
	body := []byte(render(world, v1))
	k := newKeeper(t, Config{Source: sliceSource{
		{Name: "a", World: world, Version: "v1", Data: body},
		{Name: "b", World: world, Version: "v1", Data: body},
		{Name: "c", World: world, Version: "one", Data: body},
	}})
	if _, err := k.Get(world, "v1"); !errors.Is(err, ErrInvalidDocument) || !strings.Contains(err.Error(), "also in a") {
		t.Errorf("Get(v1) = %v, want the duplicate named", err)
	}
	if len(k.Problems()) != 2 {
		t.Errorf("Problems = %+v, want the duplicate and the bad version name", k.Problems())
	}
}

type failingSource struct{}

func (failingSource) Documents(context.Context) ([]Document, error) {
	return nil, errors.New("the object store is down")
}

func TestNewFailsOnlyWhenTheSourceCannotBeRead(t *testing.T) {
	if _, err := New(context.Background(), Config{Source: failingSource{}}); err == nil {
		t.Error("New over an unreadable source succeeded")
	}
	if _, err := New(context.Background(), Config{}); err == nil {
		t.Error("New without a source succeeded")
	}
}

func TestStrain(t *testing.T) {
	k := newKeeper(t, Config{Source: FileSource{Dir: lawsDir(t, v1)}})
	s := k.Strain()
	if s != k.Strain() {
		t.Fatal("Strain returns a different counter on each call")
	}
	if got := s.Snapshot(); len(got) != 0 {
		t.Errorf("a fresh counter holds %v", got)
	}
	s.Inc("inv-01")
	s.Inc("inv-01")
	s.Inc("law-2")
	s.Inc("")
	snap := s.Snapshot()
	if len(snap) != 2 || snap["inv-01"] != 2 || snap["law-2"] != 1 {
		t.Errorf("Snapshot = %v, want inv-01=2 law-2=1", snap)
	}
	snap["inv-01"] = 100
	s.Inc("inv-01")
	if got := s.Snapshot()["inv-01"]; got != 3 {
		t.Errorf("inv-01 = %d after changing a snapshot; the snapshot is not a copy", got)
	}
	if snap["inv-01"] != 100 {
		t.Error("a later Inc changed a snapshot already taken")
	}

	var zero Strain
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			zero.Inc("inv-02")
		}()
	}
	wg.Wait()
	if got := zero.Snapshot()["inv-02"]; got != 50 {
		t.Errorf("concurrent Inc counted %d, want 50", got)
	}
}

// changedEvent is a world.laws.changed as another process publishes it.
func changedEvent(from, to string) eventbus.Event {
	return eventbus.NewRoot(TypeChanged, contracts.SourceMvctl, world, nil, eventbus.ActorSystem, map[string]any{
		"laws":           map[string]any{"version_from": from, "version_to": to, "status": StatusApproved},
		"created_by":     CreatedByAuthor,
		"diff":           map[string]any{"added": []string{}, "removed": []string{}},
		"effective_from": map[string]any{"round_boundary": true},
	})
}

func TestHandleReloadsAndNotifiesTheWatchers(t *testing.T) {
	dir := lawsDir(t, v1)
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})
	first, second := k.Watch(), k.Watch()
	writeSpec(t, dir, v2)

	other := eventbus.NewRoot("world.weather_changed", contracts.SourceSwarm, world, nil, eventbus.ActorSystem, nil)
	if err := k.Handle(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(world, "v2"); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("another type reloaded the source: Get(v2) = %v", err)
	}
	if len(first) != 0 {
		t.Error("another type notified a watcher")
	}

	ev := changedEvent("v1", "v2")
	if err := k.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(world, "v2"); err != nil {
		t.Errorf("Get(v2) after world.laws.changed = %v", err)
	}
	want := Changed{World: world, From: "v1", To: "v2", Status: StatusApproved, CreatedBy: CreatedByAuthor, EventID: ev.ID}
	for i, ch := range []<-chan Changed{first, second} {
		select {
		case got := <-ch:
			if got != want {
				t.Errorf("watcher %d got %+v, want %+v", i, got, want)
			}
		default:
			t.Errorf("watcher %d was not notified", i)
		}
	}
}

func TestHandleFailsWhenTheSourceCannotBeRead(t *testing.T) {
	dir := lawsDir(t, v1)
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})
	watcher := k.Watch()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := k.Handle(context.Background(), changedEvent("v1", "v2")); err == nil {
		t.Error("Handle over an unreadable source returned nil; the bus would not retry")
	}
	if len(watcher) != 0 {
		t.Error("a watcher was told about a version the keeper could not read")
	}
	if _, err := k.Get(world, "v1"); err != nil {
		t.Errorf("a failed reload dropped what the keeper held: %v", err)
	}
}

func TestAWatcherThatDoesNotReadDoesNotBlockTheSubscription(t *testing.T) {
	k := newKeeper(t, Config{Source: FileSource{Dir: lawsDir(t, v1)}})
	stuck := k.Watch()
	deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range watchBuffer + 5 {
			if err := k.Handle(context.Background(), changedEvent("v1", "v1")); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	select {
	case <-done:
	case <-deadline.Done():
		t.Fatal("Handle blocked on a watcher that does not read")
	}
	if len(stuck) != watchBuffer {
		t.Errorf("the stuck watcher holds %d notifications, want %d", len(stuck), watchBuffer)
	}
}

// newBus is the bus of the tests: membus with the registry of the binary, so
// that publication validates the envelope, the topic policy and the schema.
func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	bus, err := membus.New(membus.Config{Registry: contracts.Default()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

// published reads a topic of the bus from the start.
func published(t *testing.T, bus *membus.Bus, topic string) []eventbus.Event {
	t.Helper()
	ctx := context.Background()
	end, err := bus.End(ctx, topic)
	if errors.Is(err, membus.ErrNoTopic) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []eventbus.Event
	if _, err := bus.ReadRange(ctx, topic, 0, end, func(_ context.Context, ev eventbus.Event) error {
		out = append(out, ev)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSubscribeDeliversWorldLawsChangedToHandle(t *testing.T) {
	dir := lawsDir(t, v1)
	bus := newBus(t)
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})
	watcher := k.Watch()

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- k.Subscribe(ctx, bus, "core.laws") }()

	deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	bumper := newKeeper(t, Config{Source: FileSource{Dir: dir}, Bus: bus})
	path := writeSpec(t, dir, v2)
	if _, err := bumper.Bump(ctx, world, path); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-watcher:
		if got.To != "v2" || got.World != world {
			t.Errorf("notified %+v", got)
		}
	case <-deadline.Done():
		t.Fatal("the subscription did not deliver world.laws.changed")
	}
	cancel()
	if err := <-stopped; err != nil {
		t.Errorf("Subscribe stopped with %v", err)
	}
	if doc, err := k.Current(world); err != nil || doc.Version != "v2" {
		t.Errorf("Current after the notification = %s, %v", doc.Version, err)
	}
}

func TestBumpPublishesTheChangeAndTheProposal(t *testing.T) {
	for name, versions := range map[string]WorldVersions{
		"current from the files": nil,
		"current from the state": fakeVersions{world: "v1"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := lawsDir(t, v1)
			path := writeSpec(t, dir, v2)
			bus := newBus(t)
			k := newKeeper(t, Config{Source: FileSource{Dir: dir}, Bus: bus, Versions: versions})

			doc, err := k.Bump(context.Background(), world, path)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Version != "v2" {
				t.Errorf("Bump returned %s", doc.Version)
			}

			worldEvents := published(t, bus, eventbus.TopicWorldEvents)
			systemEvents := published(t, bus, eventbus.TopicSystemEvents)
			if len(worldEvents) != 1 || len(systemEvents) != 1 {
				t.Fatalf("published %d on world_events and %d on system_events, want one each",
					len(worldEvents), len(systemEvents))
			}
			changed, proposal := worldEvents[0], systemEvents[0]
			checkChanged(t, changed, []string{"inv-02", "law-3"}, []string{"law-2"})
			checkProposal(t, proposal, changed, "v2")
		})
	}
}

func checkChanged(t *testing.T, ev eventbus.Event, added, removed []string) {
	t.Helper()
	if ev.Type != TypeChanged || ev.Source != contracts.SourceMvctl ||
		ev.Meta.ActorKind != eventbus.ActorSystem || ev.Meta.Agent != nil ||
		eventbus.GetWorldIDFromEvent(ev) != world || ev.Meta.CausationID != "" {
		t.Errorf("world.laws.changed envelope: %+v", ev)
	}
	pa := ev.Path()
	for path, want := range map[string]string{
		"laws.version_from": "v1", "laws.version_to": "v2", "laws.status": StatusApproved,
		"created_by": CreatedByAuthor,
	} {
		if got, _ := pa.GetString(path); got != want {
			t.Errorf("payload.%s = %q, want %q", path, got, want)
		}
	}
	if got, _ := pa.GetBool("effective_from.round_boundary"); !got {
		t.Error("payload.effective_from.round_boundary is not true")
	}
	if got := anyStrings(pa.GetAny("diff.added")); !slices.Equal(got, added) {
		t.Errorf("diff.added = %v, want %v", got, added)
	}
	if got := anyStrings(pa.GetAny("diff.removed")); !slices.Equal(got, removed) {
		t.Errorf("diff.removed = %v, want %v", got, removed)
	}
	checkPublishable(t, ev)
}

func checkProposal(t *testing.T, ev, cause eventbus.Event, version string) {
	t.Helper()
	if ev.Type != "entity.update.proposed" || ev.Source != contracts.SourceMvctl ||
		ev.Meta.ActorKind != eventbus.ActorSystem || ev.Meta.Agent != nil ||
		ev.Meta.CausationID != cause.ID || ev.Meta.CorrelationID != cause.ID ||
		eventbus.GetWorldIDFromEvent(ev) != world {
		t.Errorf("entity.update.proposed envelope: %+v", ev)
	}
	pa := ev.Path()
	for path, want := range map[string]string{
		"proposal_id": "laws:" + world + ":" + version,
		"cause":       "author",
	} {
		if got, _ := pa.GetString(path); got != want {
			t.Errorf("payload.%s = %q, want %q", path, got, want)
		}
	}
	if atomic, _ := pa.GetBool("atomic"); !atomic {
		t.Error("payload.atomic is not true")
	}
	changes, _ := pa.GetAny("changes")
	list, _ := changes.([]any)
	if len(list) != 1 {
		t.Fatalf("changes = %v, want one change of the world", changes)
	}
	change := list[0].(map[string]any)
	ref := change["entity"].(map[string]any)["entity"].(map[string]any)
	if ref["id"] != world || ref["type"] != "world" {
		t.Errorf("changed entity %v, want the world", ref)
	}
	ops := change["ops"].([]any)
	if len(ops) != 1 {
		t.Fatalf("ops = %v", ops)
	}
	op := ops[0].(map[string]any)
	if op["op"] != "set" || op["path"] != "laws_version" || op["value"] != version {
		t.Errorf("op = %v, want set laws_version %s", op, version)
	}
	checkPublishable(t, ev)

	// The proposer without an agent may change the world with cause author:
	// State would otherwise refuse the proposal with level_violation (C-02).
	allowed := false
	for _, rule := range contracts.OwnershipRules() {
		if (rule.Proposer == contracts.ProposerSystem || rule.Proposer == contracts.ProposerAuthor) &&
			slices.Contains(rule.Causes, "author") {
			allowed = true
		}
	}
	if !allowed {
		t.Error("no row of contracts.OwnershipRules lets a proposal without an agent carry cause author")
	}
}

// checkPublishable is what the bus already checked on publish and on read,
// said again so that the test fails on the rule, not on a count of events: the
// schema, and the source among the publishers of the type.
func checkPublishable(t *testing.T, ev eventbus.Event) {
	t.Helper()
	if err := contracts.Validate(ev); err != nil {
		t.Errorf("%s does not validate: %v", ev.Type, err)
	}
	spec, ok := contracts.Lookup(ev.Type)
	if !ok || !slices.Contains(spec.Publishers, ev.Source) {
		t.Errorf("%s: source %s is not among the publishers %v", ev.Type, ev.Source, spec.Publishers)
	}
}

func anyStrings(v any, _ bool) []string {
	var out []string
	switch values := v.(type) {
	case []any:
		for _, x := range values {
			s, _ := x.(string)
			out = append(out, s)
		}
	case []string:
		out = values
	}
	return out
}

// A version that changes only the text of a law adds and removes nothing; the
// schema still wants both arrays, empty rather than null.
func TestBumpOfTextOnlyPublishesEmptyDiffArrays(t *testing.T) {
	dir := lawsDir(t, v1)
	path := filepath.Join(dir, FileName(world, "v2"))
	writeFile(t, path, strings.Replace(render(world, spec{version: "v2", basedOn: "v1", laws: v1.laws}),
		"text of law-1", "new text of law-1", 1))
	bus := newBus(t)
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}, Bus: bus})
	if _, err := k.Bump(context.Background(), world, path); err != nil {
		t.Fatal(err)
	}
	events := published(t, bus, eventbus.TopicWorldEvents)
	if len(events) != 1 {
		t.Fatalf("published %d", len(events))
	}
	for _, key := range []string{"diff.added", "diff.removed"} {
		v, _ := events[0].Path().GetAny(key)
		if list, ok := v.([]any); !ok || len(list) != 0 {
			t.Errorf("%s = %#v, want an empty array", key, v)
		}
	}
}

func TestBumpRefuses(t *testing.T) {
	v3 := spec{version: "v3", basedOn: "v2", laws: []string{"law-1"}}
	cases := []struct {
		name     string
		prepare  func(t *testing.T, dir string) string // returns the path to bump
		versions WorldVersions
		world    string
		want     error
		text     string
	}{
		{
			name:    "another world",
			prepare: func(t *testing.T, dir string) string { return writeSpec(t, dir, v2) },
			world:   "other-world",
			want:    ErrBumpRefused, text: "not of other-world",
		},
		{
			name: "not approved",
			prepare: func(t *testing.T, dir string) string {
				return writeSpec(t, dir, spec{version: "v2", basedOn: "v1", status: StatusPendingReview, laws: v2.laws})
			},
			want: ErrBumpRefused, text: "pending_review",
		},
		{
			name: "created by the breach",
			prepare: func(t *testing.T, dir string) string {
				return writeSpec(t, dir, spec{version: "v2", basedOn: "v1", createdBy: CreatedByBreach, laws: v2.laws})
			},
			want: ErrBumpRefused, text: "created by breach",
		},
		{
			name:    "the first version",
			prepare: func(t *testing.T, dir string) string { return filepath.Join(dir, FileName(world, "v1")) },
			want:    ErrBumpRefused, text: "no based_on",
		},
		{
			name: "based on an old version",
			prepare: func(t *testing.T, dir string) string {
				writeSpec(t, dir, v2)
				return writeSpec(t, dir, spec{version: "v3", basedOn: "v1", laws: v2.laws})
			},
			want: ErrBumpRefused, text: "based on v1, and the current version of dark-forest-world is v2",
		},
		{
			name: "skipping a number",
			prepare: func(t *testing.T, dir string) string {
				return writeSpec(t, dir, spec{version: "v3", basedOn: "v1", laws: v2.laws})
			},
			want: ErrBumpRefused, text: "the current version of dark-forest-world is v1",
		},
		{
			name: "a newer version is already in the directory",
			prepare: func(t *testing.T, dir string) string {
				path := writeSpec(t, dir, v2)
				writeSpec(t, dir, v3)
				return path
			},
			want: ErrBumpRefused, text: "the current version of dark-forest-world is v3",
		},
		{
			name:     "the world already runs under it",
			prepare:  func(t *testing.T, dir string) string { return writeSpec(t, dir, v2) },
			versions: fakeVersions{world: "v2"},
			want:     ErrBumpRefused, text: "already runs under v2",
		},
		{
			name:     "the state names a version the source does not hold",
			prepare:  func(t *testing.T, dir string) string { return writeSpec(t, dir, v2) },
			versions: fakeVersions{world: "v9"},
			want:     ErrBumpRefused, text: "the current version",
		},
		{
			name: "a file outside the laws directory",
			prepare: func(t *testing.T, _ string) string {
				return writeSpec(t, t.TempDir(), v2)
			},
			want: ErrBumpRefused, text: "not in the laws source",
		},
		{
			name: "a file that differs from the one in the directory",
			prepare: func(t *testing.T, dir string) string {
				writeSpec(t, dir, v2)
				return writeSpec(t, t.TempDir(), spec{version: "v2", basedOn: "v1", laws: []string{"law-9"}})
			},
			want: ErrBumpRefused, text: "differs from dark-forest-world.v2.yaml",
		},
		{
			name: "an unknown check",
			prepare: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, FileName(world, "v2"))
				writeFile(t, path, strings.Replace(render(world, v2), "check: inv-02", "check: nope", 1))
				return path
			},
			want: ErrUnknownCheck,
		},
		{
			name:    "a missing file",
			prepare: func(t *testing.T, dir string) string { return filepath.Join(dir, FileName(world, "v2")) },
			want:    os.ErrNotExist,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := lawsDir(t, v1)
			path := tc.prepare(t, dir)
			bus := newBus(t)
			k := newKeeper(t, Config{Source: FileSource{Dir: dir}, Bus: bus, Versions: tc.versions})
			w := tc.world
			if w == "" {
				w = world
			}
			_, err := k.Bump(context.Background(), w, path)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Bump = %v, want %v", err, tc.want)
			}
			if tc.text != "" && !strings.Contains(err.Error(), tc.text) {
				t.Errorf("Bump = %q, want it to say %q", err, tc.text)
			}
			if n := len(published(t, bus, eventbus.TopicWorldEvents)) + len(published(t, bus, eventbus.TopicSystemEvents)); n != 0 {
				t.Errorf("a refused bump published %d events", n)
			}
		})
	}
}

func TestBumpWithoutABus(t *testing.T) {
	dir := lawsDir(t, v1)
	k := newKeeper(t, Config{Source: FileSource{Dir: dir}})
	if _, err := k.Bump(context.Background(), world, writeSpec(t, dir, v2)); !errors.Is(err, ErrNoBus) {
		t.Errorf("Bump = %v, want ErrNoBus", err)
	}
}

// failingBus fails the publication number failOn (1-based).
type failingBus struct {
	eventbus.Bus
	failOn int
	calls  int
	sent   []eventbus.Event
}

func (b *failingBus) Publish(ctx context.Context, ev eventbus.Event) error {
	b.calls++
	if b.calls == b.failOn {
		return errors.New("broker unavailable")
	}
	b.sent = append(b.sent, ev)
	return nil
}

func TestBumpSaysWhatWasPublishedWhenThePublicationFails(t *testing.T) {
	for failOn, want := range map[int]string{
		1: "publish world.laws.changed",
		2: "is published, the proposal of world.laws_version is not",
	} {
		t.Run(want, func(t *testing.T) {
			dir := lawsDir(t, v1)
			bus := &failingBus{failOn: failOn}
			k := newKeeper(t, Config{Source: FileSource{Dir: dir}, Bus: bus})
			_, err := k.Bump(context.Background(), world, writeSpec(t, dir, v2))
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Bump = %v, want it to say %q", err, want)
			}
			if failOn == 2 && !strings.Contains(err.Error(), bus.sent[0].ID) {
				t.Errorf("Bump = %v, want the id of the published world.laws.changed", err)
			}
		})
	}
}
