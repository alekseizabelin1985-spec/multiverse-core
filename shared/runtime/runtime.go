// Package runtime holds the process skeleton shared by every deployment of
// cmd/multiverse: the context interface, the registry that orders contexts by
// their declared dependencies, and the HTTP server the process owns.
//
// See foundation.md §3 and ADR-001 addendum p. 3.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

// Mode selects how the process consumes time and events.
type Mode string

const (
	// ModeLive runs against the real bus and the wall clock.
	ModeLive Mode = "live"
	// ModeReplay drives the process from a recorded journal.
	ModeReplay Mode = "replay"
)

// All is the reserved context name that expands to every registered context.
const All = "all"

// Health status values reported by a context.
const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
	StatusFail     = "fail"
)

// Status is the health of one context or of the whole process.
type Status struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
}

// OK returns a healthy Status without details.
func OK() Status { return Status{Status: StatusOK} }

// Deps is the single dependency bundle handed to every context; a context
// takes what it needs and ignores the rest. A context that requires a
// dependency the process did not build fails in Start.
//
// Fields are added by the tasks that create the packages behind them:
// Store and Env in F-5 (T-007).
type Deps struct {
	// Bus publishes events and opens live subscriptions; Journal reads a
	// topic by offset for the catch-up after a snapshot (C-01).
	Bus     eventbus.Bus
	Journal eventbus.Journal
	// Contracts is the event registry the contexts validate against
	// (foundation.md §3, contracts.md C-01). The process passes
	// contracts.Default(); a test passes contracts.New over a fixture tree.
	Contracts *contracts.Registry
	Clock     clock.Clock
	Timers    clock.Timers
	Mode      Mode
	// IDs generates event identifiers (uuid or a deterministic sequence).
	IDs func() string
	Log *slog.Logger
	// Mux is the HTTP mux of the process; a context implementing Routes gets
	// it before Start.
	Mux *http.ServeMux
}

// Context is one bounded context of the platform, compiled into the process
// and selected at start time by --contexts.
type Context interface {
	Name() string
	// DependsOn names the contexts whose Start must complete first. Names
	// outside the selected set are ignored: those contexts run in another
	// process and are reached over the bus.
	DependsOn() []string
	Start(ctx context.Context, deps Deps) error
	Stop(ctx context.Context) error
	Health() Status
}

// Routes is implemented by a context that mounts HTTP handlers on the process
// mux (/v1/admin/*, /v1/context/*). Routes is called before Start.
type Routes interface {
	Routes(mux *http.ServeMux)
}

// ErrUnknownContext is returned by New for a name nobody registered.
var ErrUnknownContext = errors.New("unknown context")

// Registry maps context names to their factories.
type Registry struct {
	mu        sync.Mutex
	order     []string
	factories map[string]func() Context
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]func() Context)}
}

// Register adds a factory under name. Registering the same name twice panics:
// it is a programming error visible at process start, not at runtime.
func (r *Registry) Register(name string, factory func() Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic(fmt.Sprintf("runtime: context %q registered twice", name))
	}
	r.factories[name] = factory
	r.order = append(r.order, name)
}

// Names returns the registered context names in registration order.
func (r *Registry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

// New builds the named contexts and returns them in start order: a context
// comes after every context it depends on, ties broken by registration order
// so the sequence is the same on every run. The single name "all" expands to
// every registered context.
func (r *Registry) New(names []string) ([]Context, error) {
	selected, err := r.resolve(names)
	if err != nil {
		return nil, err
	}
	built := make(map[string]Context, len(selected))
	for _, name := range selected {
		r.mu.Lock()
		factory := r.factories[name]
		r.mu.Unlock()
		built[name] = factory()
	}
	return sortByDeps(selected, built)
}

func (r *Registry) resolve(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, errors.New("runtime: no contexts requested")
	}
	if len(names) == 1 && names[0] == All {
		return r.Names(), nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := make(map[string]bool, len(names))
	selected := make([]string, 0, len(names))
	for _, name := range names {
		if name == All {
			return nil, errors.New("runtime: context \"all\" cannot be combined with other names")
		}
		if _, ok := r.factories[name]; !ok {
			return nil, fmt.Errorf("%w: %q (registered: %s)", ErrUnknownContext, name, strings.Join(r.order, ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		selected = append(selected, name)
	}
	// Keep registration order regardless of the order given on the command line.
	rank := make(map[string]int, len(r.order))
	for i, name := range r.order {
		rank[name] = i
	}
	sort.SliceStable(selected, func(i, j int) bool { return rank[selected[i]] < rank[selected[j]] })
	return selected, nil
}

func sortByDeps(selected []string, built map[string]Context) ([]Context, error) {
	inSet := make(map[string]bool, len(selected))
	for _, name := range selected {
		inSet[name] = true
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := make(map[string]int, len(selected))
	ordered := make([]Context, 0, len(selected))
	var visit func(name string, path []string) error
	visit = func(name string, path []string) error {
		switch color[name] {
		case black:
			return nil
		case grey:
			return fmt.Errorf("runtime: dependency cycle %s", strings.Join(append(path, name), " -> "))
		}
		color[name] = grey
		for _, dep := range built[name].DependsOn() {
			if !inSet[dep] {
				continue
			}
			if err := visit(dep, append(path, name)); err != nil {
				return err
			}
		}
		color[name] = black
		ordered = append(ordered, built[name])
		return nil
	}
	for _, name := range selected {
		if err := visit(name, nil); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

var defaultRegistry = NewRegistry()

// Register adds a context factory to the default registry. Packages call it
// from an init function; cmd/multiverse imports them for the side effect.
func Register(name string, factory func() Context) { defaultRegistry.Register(name, factory) }

// Names returns the context names of the default registry.
func Names() []string { return defaultRegistry.Names() }

// New builds the named contexts from the default registry in start order.
func New(names []string) ([]Context, error) { return defaultRegistry.New(names) }
