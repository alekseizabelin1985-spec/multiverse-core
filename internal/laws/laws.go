package laws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"sync"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
)

// The event types the package publishes and reads.
const (
	// TypeChanged announces a new version of the laws of a world (C-12).
	TypeChanged = "world.laws.changed"
	// typeUpdateProposed carries World.laws_version to State (C-02).
	typeUpdateProposed = "entity.update.proposed"
)

// causeAuthor is the cause of the proposal of world.laws_version: the author
// changed the laws. There is no cause "laws" in the schema of
// entity.update.proposed nor in the row author of contracts.OwnershipRules
// (C-02 v1.5, T-444).
const causeAuthor = "author"

// bumpSource is the envelope source of the events of Bump. The bump is the
// author's path and in MVP-1 only mvctl laws bump takes it; with --bus=kafka it
// is the mvctl process that publishes, not core (C-02 v1.5: source names the
// component that publishes). contracts.SourceMvctl is in the publishers of both
// types.
const bumpSource = contracts.SourceMvctl

// Where CurrentFrom took the version from.
const (
	// OriginState is World.laws_version, read through WorldVersions.
	OriginState = "state"
	// OriginFiles is the newest approved version of the source, used when
	// there is no state to ask.
	OriginFiles = "files"
)

// The values of Details["laws"] in Health.
const (
	HealthUnknownCheck    = "unknown_check"
	HealthInvalidDocument = "invalid_document"
)

// watchBuffer is how many notifications a watcher may leave unread before the
// next one is dropped. A dropped notification loses nothing but the hint:
// agents take Current when they build each prompt (swarm-llm-laws.md §12.2).
const watchBuffer = 16

var (
	// ErrUnknownVersion is a version the source does not hold.
	ErrUnknownVersion = errors.New("laws: unknown version")
	// ErrNoApprovedVersion is a world without a single approved version.
	ErrNoApprovedVersion = errors.New("laws: no approved version")
	// ErrBumpRefused is what every refusal of Bump matches; the sentence says
	// which rule the document broke.
	ErrBumpRefused = errors.New("laws: bump refused")
	// ErrNoBus is Bump on a keeper built without a bus.
	ErrNoBus = errors.New("laws: no bus to publish the bump on")
)

// WorldVersions is where the current version of a world comes from: its State
// (World.laws_version). The interface belongs to the consumer — the WorldView
// of the swarm implements it, and the laws do not import the swarm, which
// would be a cycle (C-12 v1.1, ADR-001 addendum 2026-09-13 p. 6).
type WorldVersions interface {
	// LawsVersion returns the version the world runs under, and false when the
	// world entity is not known.
	LawsVersion(worldID string) (version string, ok bool)
}

// Changed is the notification Watch delivers after world.laws.changed.
type Changed struct {
	World     string
	From      string
	To        string
	Status    string
	CreatedBy string
	// EventID is the id of the world.laws.changed that caused the reload.
	EventID string
}

// Service is the laws as the swarm sees them (swarm-llm-laws.md §3).
type Service interface {
	Current(world string) (LawsVersion, error)
	Get(world, version string) (LawsVersion, error)
	// Watch returns a channel notified after every world.laws.changed.
	Watch() <-chan Changed
	Strain() *Strain
	// Bump announces the version in the file at path as the next version of
	// the world: world.laws.changed and entity.update.proposed of
	// world.laws_version.
	Bump(ctx context.Context, world, path string) (LawsVersion, error)
}

// Problem is a document the keeper could not load.
type Problem struct {
	// Document is the name the source gave it.
	Document string
	// World and Version are empty when the name did not say them.
	World   string
	Version string
	Err     error
}

// Config builds a Keeper. Only Source is mandatory.
type Config struct {
	Source Source
	// Versions answers the current version of a world; nil means the newest
	// approved version of the source (mvctl, tests).
	Versions WorldVersions
	// Checks are the check keys an invariant may name; nil is KnownChecks.
	Checks []string
	// Bus is what Bump publishes on; Current, Get and Watch do not need it.
	Bus eventbus.Bus
	Log *slog.Logger
}

// Keeper is the implementation of Service over a Source.
type Keeper struct {
	source   Source
	versions WorldVersions
	checks   []string
	bus      eventbus.Bus
	log      *slog.Logger
	strain   Strain

	mu       sync.RWMutex
	worlds   map[string]map[int]entry
	problems []Problem

	watchMu  sync.Mutex
	watchers []chan Changed

	// missed holds world@version the state named before the keeper held it
	// and the keeper has reloaded for (CurrentFrom).
	missMu sync.Mutex
	missed map[string]bool
}

// entry is one version as loaded: the document, or why it did not load.
type entry struct {
	name string
	doc  LawsVersion
	err  error
}

var _ Service = (*Keeper)(nil)

// New builds the keeper and loads every document of the source. A document
// that does not load does not fail New: it is a Problem, Get of its version
// returns the reason, and Health says degraded. New fails only when the source
// itself cannot be read.
func New(ctx context.Context, cfg Config) (*Keeper, error) {
	if cfg.Source == nil {
		return nil, errors.New("laws: no source")
	}
	checks := cfg.Checks
	if checks == nil {
		checks = KnownChecks()
	}
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	k := &Keeper{
		source:   cfg.Source,
		versions: cfg.Versions,
		checks:   slices.Clone(checks),
		bus:      cfg.Bus,
		log:      log,
	}
	if err := k.Reload(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

// Reload reads the source again and replaces what the keeper holds.
func (k *Keeper) Reload(ctx context.Context) error {
	docs, err := k.source.Documents(ctx)
	if err != nil {
		return err
	}
	worlds := make(map[string]map[int]entry)
	var problems []Problem
	for _, d := range docs {
		if d.NameErr != nil {
			problems = append(problems, Problem{Document: d.Name,
				Err: &DocumentError{Name: d.Name, Reason: d.NameErr.Error()}})
			continue
		}
		n, err := VersionNumber(d.Version)
		if err != nil {
			problems = append(problems, Problem{Document: d.Name, World: d.World, Version: d.Version,
				Err: &DocumentError{Name: d.Name, Reason: err.Error()}})
			continue
		}
		e := entry{name: d.Name}
		e.doc, e.err = Parse(d.Name, d.Data, k.checks)
		if e.err == nil && (e.doc.WorldID != d.World || e.doc.Version != d.Version) {
			e.err = &DocumentError{Name: d.Name, Reason: fmt.Sprintf(
				"the name says %s %s, the document says %s %s",
				d.World, d.Version, e.doc.WorldID, e.doc.Version)}
		}
		if worlds[d.World] == nil {
			worlds[d.World] = make(map[int]entry)
		}
		if prev, dup := worlds[d.World][n]; dup {
			e.err = &DocumentError{Name: d.Name, Reason: fmt.Sprintf(
				"%s of %s is also in %s", d.Version, d.World, prev.name)}
		}
		if e.err != nil {
			e.doc = LawsVersion{}
			problems = append(problems, Problem{Document: d.Name, World: d.World, Version: d.Version, Err: e.err})
		}
		worlds[d.World][n] = e
	}
	for _, p := range problems {
		k.log.WarnContext(ctx, "laws: a document does not load",
			"document", p.Document, "error", p.Err.Error())
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	k.worlds = worlds
	k.problems = problems
	return nil
}

// Get returns a version of the laws of a world.
func (k *Keeper) Get(world, version string) (LawsVersion, error) {
	n, err := VersionNumber(version)
	if err != nil {
		return LawsVersion{}, fmt.Errorf("%w: %w", ErrUnknownVersion, err)
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	e, ok := k.worlds[world][n]
	if !ok {
		return LawsVersion{}, fmt.Errorf("%w: %s of %s", ErrUnknownVersion, version, world)
	}
	if e.err != nil {
		return LawsVersion{}, fmt.Errorf("laws: %s of %s does not load: %w", version, world, e.err)
	}
	return clone(e.doc), nil
}

// Current returns the version the world runs under.
func (k *Keeper) Current(world string) (LawsVersion, error) {
	doc, _, err := k.CurrentFrom(world)
	return doc, err
}

// CurrentFrom is Current together with where the version came from:
// OriginState when WorldVersions knows the world, OriginFiles otherwise
// (mvctl laws show prints it, C-12 v1.1).
//
// A version the state names but the source does not hold is an error, not a
// fallback: the world runs under it, and answering with another one would
// hand the guardian the wrong set of checks.
//
// Before that error the keeper reads the source once more, once per version
// of a world. world.laws.changed travels in world_events and the entity.updated
// that moves World.laws_version in system_events, so the state may name vN+1
// before Handle has reloaded; the document is already on disk, because Bump
// refuses a version the laws directory does not hold. A version that is still
// missing after that reload stays an error without further reads, so that a
// state naming a version this installation never had does not turn every
// prompt into a read of the directory.
func (k *Keeper) CurrentFrom(world string) (LawsVersion, string, error) {
	if k.versions != nil {
		if version, ok := k.versions.LawsVersion(world); ok {
			doc, err := k.Get(world, version)
			if errors.Is(err, ErrUnknownVersion) && k.firstMiss(world, version) {
				doc, err = k.reloadFor(world, version, err)
			}
			return doc, OriginState, err
		}
	}
	doc, err := k.newestApproved(world, "")
	return doc, OriginFiles, err
}

// firstMiss reports whether the state names this version of the world for the
// first time without the keeper holding it, and remembers that it did.
func (k *Keeper) firstMiss(world, version string) bool {
	key := world + "@" + version
	k.missMu.Lock()
	defer k.missMu.Unlock()
	if k.missed[key] {
		return false
	}
	if k.missed == nil {
		k.missed = make(map[string]bool)
	}
	k.missed[key] = true
	return true
}

// reloadFor reads the source again for a version the state names and returns
// that version, or miss when the source cannot be read.
func (k *Keeper) reloadFor(world, version string, miss error) (LawsVersion, error) {
	if err := k.Reload(context.Background()); err != nil {
		k.log.Warn("laws: reload for the version the state names failed",
			"world", world, "version", version, "error", err.Error())
		return LawsVersion{}, miss
	}
	return k.Get(world, version)
}

// newestApproved is the newest approved version of a world, leaving out the
// version except. A newer version that does not load is an error rather than
// something to step over: it may be the one the author meant to be current.
// The only caller that passes except, bumpBase, has loaded that version
// already, so except is never a version that does not load.
func (k *Keeper) newestApproved(world, except string) (LawsVersion, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	numbers := make([]int, 0, len(k.worlds[world]))
	for n := range k.worlds[world] {
		numbers = append(numbers, n)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(numbers)))
	for _, n := range numbers {
		e := k.worlds[world][n]
		if e.err != nil {
			return LawsVersion{}, fmt.Errorf("laws: v%d of %s does not load: %w", n, world, e.err)
		}
		if e.doc.Version == except || e.doc.Status != StatusApproved {
			continue
		}
		return clone(e.doc), nil
	}
	return LawsVersion{}, fmt.Errorf("%w: %s", ErrNoApprovedVersion, world)
}

// Worlds returns the worlds the source holds documents of, sorted.
func (k *Keeper) Worlds() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	worlds := make([]string, 0, len(k.worlds))
	for w := range k.worlds {
		worlds = append(worlds, w)
	}
	sort.Strings(worlds)
	return worlds
}

// Versions returns the versions of a world the source holds, loaded or not,
// oldest first.
func (k *Keeper) Versions(world string) []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	numbers := make([]int, 0, len(k.worlds[world]))
	for n := range k.worlds[world] {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	out := make([]string, 0, len(numbers))
	for _, n := range numbers {
		out = append(out, fmt.Sprintf("v%d", n))
	}
	return out
}

// Problems returns the documents that did not load at the last reload.
func (k *Keeper) Problems() []Problem {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return slices.Clone(k.problems)
}

// Health is degraded while a document does not load. Details["laws"] is
// unknown_check when a law names a check nothing implements, and
// invalid_document for any other defect (tasks.md T-205; the /health of the
// swarm, T-239, carries it).
func (k *Keeper) Health() runtime.Status {
	problems := k.Problems()
	if len(problems) == 0 {
		return runtime.OK()
	}
	detail := HealthInvalidDocument
	messages := make([]string, 0, len(problems))
	for _, p := range problems {
		if errors.Is(p.Err, ErrUnknownCheck) {
			detail = HealthUnknownCheck
		}
		messages = append(messages, p.Err.Error())
	}
	return runtime.Status{Status: runtime.StatusDegraded,
		Details: map[string]any{"laws": detail, "problems": messages}}
}

// Strain returns the strain counter of the keeper.
func (k *Keeper) Strain() *Strain { return &k.strain }

// Watch returns a channel that receives a Changed after every
// world.laws.changed the keeper handles. Every call returns a channel of its
// own. A watcher that leaves watchBuffer notifications unread loses the next
// ones rather than blocking the subscription.
func (k *Keeper) Watch() <-chan Changed {
	ch := make(chan Changed, watchBuffer)
	k.watchMu.Lock()
	defer k.watchMu.Unlock()
	k.watchers = append(k.watchers, ch)
	return ch
}

// Handle is the handler of the subscription to world_events: on
// world.laws.changed it reloads the source and notifies the watchers; every
// other type is not its business. A source that cannot be read is an error, so
// that the bus retries the event instead of the keeper announcing a version it
// does not hold.
func (k *Keeper) Handle(ctx context.Context, ev eventbus.Event) error {
	if ev.Type != TypeChanged {
		return nil
	}
	if err := k.Reload(ctx); err != nil {
		return fmt.Errorf("laws: reload after %s %s: %w", ev.Type, ev.ID, err)
	}
	pa := ev.Path()
	changed := Changed{World: eventbus.GetWorldIDFromEvent(ev), EventID: ev.ID}
	changed.From, _ = pa.GetString("laws.version_from")
	changed.To, _ = pa.GetString("laws.version_to")
	changed.Status, _ = pa.GetString("laws.status")
	changed.CreatedBy, _ = pa.GetString("created_by")

	k.watchMu.Lock()
	defer k.watchMu.Unlock()
	for _, ch := range k.watchers {
		select {
		case ch <- changed:
		default:
			k.log.WarnContext(ctx, "laws: a watcher is full, notification dropped",
				"event_id", ev.ID, "world", changed.World, "version_to", changed.To)
		}
	}
	return nil
}

// Subscribe consumes world_events under group with Handle until ctx is done.
func (k *Keeper) Subscribe(ctx context.Context, bus eventbus.Bus, group string) error {
	return bus.Subscribe(ctx, eventbus.TopicWorldEvents, group, k.Handle)
}

// Bump announces the version in the file at path as the next version of the
// laws of world (swarm-llm-laws.md §12.2, ADR-008 p. 1).
//
// The file must be the document the source holds for that version — the
// author puts laws/{world}.v{N+1}.yaml in place first — so that a version is
// never announced that the processes reading the source cannot load. The
// document must be approved, created by the author, based on the current
// version and numbered one above it. Then the keeper publishes
// world.laws.changed and, derived from it, the proposal of
// world.laws_version with actor_kind=system and cause=author.
//
// A repeated bump of the same version is the way to recover from a partial
// failure (world.laws.changed published, the proposal not), and it is allowed:
// it publishes a second world.laws.changed with a new event id. A consumer
// treats world.laws.changed as idempotent by (world, laws.version_to) — the
// guarantee is proposed for C-12, and a consumer that keeps the history of the
// laws must drop the repeat by that pair itself; State drops the
// repeated proposal by its proposal_id, and the reload of the swarm does not
// depend on how many times it runs. A keeper without WorldVersions — mvctl —
// takes version_from from the files, so the repeat still says vN -> vN+1
// although the world may already run under vN+1.
func (k *Keeper) Bump(ctx context.Context, world, path string) (LawsVersion, error) {
	if k.bus == nil {
		return LawsVersion{}, ErrNoBus
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return LawsVersion{}, fmt.Errorf("laws: bump: %w", err)
	}
	doc, err := Parse(filepath.Base(path), data, k.checks)
	if err != nil {
		return LawsVersion{}, fmt.Errorf("%w: %w", ErrBumpRefused, err)
	}
	if err := k.Reload(ctx); err != nil {
		return LawsVersion{}, err
	}
	base, err := k.bumpBase(world, filepath.Base(path), doc)
	if err != nil {
		return LawsVersion{}, err
	}

	added, removed := diff(base, doc)
	changed := eventbus.NewRoot(TypeChanged, bumpSource, world, nil, eventbus.ActorSystem, map[string]any{
		"laws": map[string]any{
			"version_from": base.Version,
			"version_to":   doc.Version,
			"status":       doc.Status,
		},
		"created_by": doc.CreatedBy,
		"diff": map[string]any{
			"added":   added,
			"removed": removed,
		},
		// A version takes effect on a round boundary, never in the middle of
		// one (ADR-008 p. 6).
		"effective_from": map[string]any{"round_boundary": true},
	})
	proposal := eventbus.Derive(changed, typeUpdateProposed, bumpSource, map[string]any{
		// One proposal per version of a world: State drops a repeated bump
		// of the same version by this id (C-02).
		"proposal_id": "laws:" + world + ":" + doc.Version,
		"changes": []any{map[string]any{
			"entity": map[string]any{"entity": map[string]any{"id": world, "type": entity.TypeWorld}},
			"ops": []any{map[string]any{
				"op": "set", "path": entity.AttrLawsVersion, "value": doc.Version,
			}},
		}},
		"atomic": true,
		"cause":  causeAuthor,
	})
	if err := k.bus.Publish(ctx, changed); err != nil {
		return LawsVersion{}, fmt.Errorf("laws: publish %s: %w", TypeChanged, err)
	}
	if err := k.bus.Publish(ctx, proposal); err != nil {
		return LawsVersion{}, fmt.Errorf("laws: %s %s is published, the proposal of world.laws_version is not: %w",
			TypeChanged, changed.ID, err)
	}
	return doc, nil
}

// bumpBase checks the document of a bump and returns the version it replaces.
func (k *Keeper) bumpBase(world, name string, doc LawsVersion) (LawsVersion, error) {
	refuse := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrBumpRefused}, args...)...)
	}
	switch {
	case doc.WorldID != world:
		return LawsVersion{}, refuse("%s holds the laws of %s, not of %s", name, doc.WorldID, world)
	case doc.Status != StatusApproved:
		return LawsVersion{}, refuse("%s is %s; the author bumps to an approved version", name, doc.Status)
	case doc.CreatedBy != CreatedByAuthor:
		return LawsVersion{}, refuse("%s is created by %s; mvctl laws bump announces the author's versions", name, doc.CreatedBy)
	case doc.BasedOn == "":
		return LawsVersion{}, refuse("%s has no based_on: %s is where a world starts, not a bump", name, doc.Version)
	}

	held, err := k.Get(world, doc.Version)
	if err != nil {
		return LawsVersion{}, refuse("%s is not in the laws source as %s: %v", name, FileName(world, doc.Version), err)
	}
	if !reflect.DeepEqual(held, doc) {
		return LawsVersion{}, refuse("%s differs from %s in the laws source", name, FileName(world, doc.Version))
	}

	var base LawsVersion
	if k.versions != nil {
		if version, ok := k.versions.LawsVersion(world); ok {
			if version == doc.Version {
				return LawsVersion{}, refuse("%s already runs under %s", world, doc.Version)
			}
			if base, err = k.Get(world, version); err != nil {
				return LawsVersion{}, refuse("the current version of %s: %v", world, err)
			}
		}
	}
	if base.Version == "" {
		if base, err = k.newestApproved(world, doc.Version); err != nil {
			return LawsVersion{}, refuse("the current version of %s: %v", world, err)
		}
	}
	if doc.BasedOn != base.Version || doc.Number() != base.Number()+1 {
		return LawsVersion{}, refuse("%s is %s based on %s, and the current version of %s is %s",
			name, doc.Version, doc.BasedOn, world, base.Version)
	}
	return base, nil
}

// diff is the law identifiers the new version adds and removes, sorted. A law
// whose text changed under the same identifier is neither: the schema of
// world.laws.changed has no field for it.
func diff(from, to LawsVersion) (added, removed []string) {
	added, removed = []string{}, []string{}
	fromIDs, toIDs := from.IDs(), to.IDs()
	for _, id := range toIDs {
		if !slices.Contains(fromIDs, id) {
			added = append(added, id)
		}
	}
	for _, id := range fromIDs {
		if !slices.Contains(toIDs, id) {
			removed = append(removed, id)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	return added, removed
}

// clone copies a version so that a caller cannot change what the keeper holds
// through the slice of laws.
func clone(v LawsVersion) LawsVersion {
	v.Laws = slices.Clone(v.Laws)
	return v
}
