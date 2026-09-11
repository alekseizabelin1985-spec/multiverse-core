package swarm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"multiverse-core.io/shared/runtime"
	tkmech "multiverse-core.io/shared/testkit/mechanics"
	tkstate "multiverse-core.io/shared/testkit/state"
)

// ContextName is the name FakeContext registers under. It is the name of the
// real context it stands in for, because that is what makes the substitution
// invisible to everything around it: --contexts=swarm selects the fight of
// EPIC-003 when internal/swarm is there and this stub when it is not
// (design.md §4.1 p. 1, ADR-001 addendum p. 8).
const ContextName = "swarm"

// DefaultRulesPath is the rule book the stub loads when the caller names none.
const DefaultRulesPath = "rules/dark-forest.yaml"

// DefaultWorldID is the world of testdata/fixtures — the only world of MVP-1
// (state-and-mechanics.md §4.10). Nothing in the code reads the fixture file,
// so the two are held together by a test instead:
// TestTheDefaultWorldIsTheOneTheFixturesDescribe.
const DefaultWorldID = "dark-forest-world"

// ContextConfig builds a FakeContext. Every field has a default, so that the
// hook of T-255 can mount the stub with nothing but a name.
type ContextConfig struct {
	// WorldID is the world the stub serves; empty selects DefaultWorldID.
	WorldID string
	// RulesPath is the rule book; empty selects DefaultRulesPath. The path is
	// resolved against the working directory of the process, like every other
	// data file of the platform.
	RulesPath string
	// FixturesDir is the entity tree the world starts from (testdata/fixtures).
	// Empty means "seed nothing": the stub then learns the world from
	// entity.created alone and opens no encounter until it has heard of a
	// living NPC. Start says so in the log rather than failing, because a
	// process driven entirely from a journal is a legitimate way to run it.
	FixturesDir string
	// Log receives what the stub decided; it defaults to the logger of Deps,
	// and to a logger that discards everything when there is none.
	Log *slog.Logger
}

// FakeContext is the bounded context of the swarm as I1-α runs it: a process
// context that mounts FakeEncounter and FakeNarrator and nothing else
// (design.md §4.1 p. 1, tasks.md T-219).
//
// It exists so that cmd/multiverse can run a playable world before
// internal/swarm exists — the hook of T-255 registers it under the name swarm
// when MV_SWARM_FAKE is set. The flag is read there and not here: at I1-α there
// is no swarm context in integration/mvp-1 to read it in, and a package that
// reached for the flag itself could not be compiled into the production binary
// (ADR-001 addendum p. 8). The hook, and with it this type, go away in T-256.
//
// It starts and stops without any context of internal/**: what it needs is the
// bus of Deps, the rule book (a pure library) and the fixtures. There is no
// model, no scheduler, no agent lifecycle and no admin route: a /v1/admin/agents
// that answered about agents nobody spawned would be worse than a 404.
type FakeContext struct {
	cfg ContextConfig

	mu        sync.Mutex
	encounter *FakeEncounter
	narrator  *FakeNarrator
	cancel    context.CancelFunc
	running   bool
	log       *slog.Logger
}

// NewFakeContext builds the context. Nothing is read and nothing is published
// until Start.
func NewFakeContext(cfg ContextConfig) *FakeContext {
	if cfg.WorldID == "" {
		cfg.WorldID = DefaultWorldID
	}
	if cfg.RulesPath == "" {
		cfg.RulesPath = DefaultRulesPath
	}
	return &FakeContext{cfg: cfg, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// Name is the context name of the process (--contexts).
func (c *FakeContext) Name() string { return ContextName }

// DependsOn is nothing: the stub reads the bus and answers on it, and a
// consumer group of C-01 starts at the first offset, so nothing has to be
// running before it (C-01 v1.2, ADR-022).
func (c *FakeContext) DependsOn() []string { return nil }

// Start loads the rules, seeds the world and opens the subscriptions of the two
// stubs. It returns as soon as they are running; the subscriptions live until
// Stop.
//
// The lifetime is the context's own rather than the one Start was given: a
// context of this platform stops when Stop is called (runtime.StopAll), and
// tying the subscriptions to the start context would make the shutdown depend
// on which context the process happened to pass in.
func (c *FakeContext) Start(_ context.Context, deps runtime.Deps) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return errors.New("testkit/swarm: context already started")
	}
	if deps.Bus == nil {
		return errors.New("testkit/swarm: no bus in deps")
	}
	if deps.Log != nil {
		c.log = deps.Log
	}
	if c.cfg.Log != nil {
		c.log = c.cfg.Log
	}
	log := c.log.With("context", ContextName, "stub", true)

	mechanics, err := tkmech.Load(c.cfg.RulesPath)
	if err != nil {
		return fmt.Errorf("testkit/swarm: load rules: %w", err)
	}
	encounter, err := NewFakeEncounter(EncounterConfig{
		Bus:       deps.Bus,
		WorldID:   c.cfg.WorldID,
		Rules:     mechanics.Rules(),
		Mechanics: mechanics,
		Log:       log,
	})
	if err != nil {
		return err
	}
	if c.cfg.FixturesDir == "" {
		log.Warn("no fixtures: the stub opens an encounter only once it has heard " +
			"of a living npc through entity.created")
	} else {
		fixtures, err := tkstate.LoadFixtures(c.cfg.FixturesDir)
		if err != nil {
			return fmt.Errorf("testkit/swarm: seed the world: %w", err)
		}
		if err := encounter.Seed(fixtures); err != nil {
			return fmt.Errorf("testkit/swarm: seed the world: %w", err)
		}
	}
	narrator, err := NewFakeNarrator(deps.Bus, c.cfg.WorldID)
	if err != nil {
		return err
	}
	narrator.WithLog(log)

	ctx, cancel := context.WithCancel(context.Background())
	if err := encounter.Start(ctx); err != nil {
		cancel()
		return err
	}
	if err := narrator.Start(ctx); err != nil {
		cancel()
		return err
	}

	c.encounter, c.narrator, c.cancel, c.running = encounter, narrator, cancel, true
	log.Info("phase 1 stub running", "world_id", c.cfg.WorldID,
		"rules_version", mechanics.Rules().Version, "removed_by", "T-256")
	return nil
}

// Stop cancels the subscriptions and waits for them. A subscription cancelled
// through its context is not a failure and reports nil (C-01), so a clean stop
// answers nil; anything else is what stopped a subscription early.
func (c *FakeContext) Stop(_ context.Context) error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	encounter, narrator, cancel := c.encounter, c.narrator, c.cancel
	c.running = false
	c.mu.Unlock()

	cancel()
	return errors.Join(encounter.Wait(), narrator.Wait())
}

// Health is what /health of the process reports for this context: ok while the
// subscriptions are alive, fail when one of them stopped on its own, degraded
// before Start and after Stop.
//
// The details say out loud that this is a stub and which task removes it, so
// that a run of I1-α cannot be mistaken for a run of the swarm.
func (c *FakeContext) Health() runtime.Status {
	c.mu.Lock()
	encounter, running := c.encounter, c.running
	c.mu.Unlock()

	details := map[string]any{
		"stub":       "testkit/swarm.FakeContext",
		"phase1":     Phase1Mode,
		"removed_by": "T-256",
	}
	if !running {
		details["state"] = "stopped"
		return runtime.Status{Status: runtime.StatusDegraded, Details: details}
	}
	details["encounters"] = encounter.ActiveCount()
	// Only the fight is asked. FakeNarrator reports a dead subscription through
	// its log and through Wait, which blocks until the subscription is over —
	// and a health check that blocks is worse than one that reports on the half
	// of the stub that decides anything (T-220 owns narrator.go; a non-blocking
	// Err on it is a request, see the report of T-219).
	if err := encounter.Err(); err != nil {
		details["err"] = err.Error()
		return runtime.Status{Status: runtime.StatusFail, Details: details}
	}
	return runtime.Status{Status: runtime.StatusOK, Details: details}
}

// Encounter is the fight stub the context runs, for a test that drives it
// directly instead of through the bus.
func (c *FakeContext) Encounter() *FakeEncounter {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.encounter
}

var _ runtime.Context = (*FakeContext)(nil)
