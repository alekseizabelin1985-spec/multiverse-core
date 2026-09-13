// Package providers is the registry of LLM provider implementations: the name
// in MV_LLM_PROVIDER selects a factory, and the factory builds a llm.Provider
// from the configuration of the block (C-15, ADR-005 add. 2 p. 2).
//
// Implementations live in subpackages (openai_compat, fake, recorded, ollama)
// and register themselves; this package compiles and works without any of
// them, which is what lets internal/memory import the registry without
// pulling every provider in (ADR-001 addendum 2026-09-13 p. 3).
package providers

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"multiverse-core.io/internal/llm"
)

// Factory builds a provider from the configuration. It is called after the
// cloud gate, so a factory never sees a non-local endpoint the operator did
// not allow, nor a missing address or one LoadConfig would have refused —
// whether the Config came from LoadConfig or was assembled by hand.
type Factory func(cfg llm.Config) (llm.Provider, error)

// ErrUnknownProvider is a name that is not a value of MV_LLM_PROVIDER.
var ErrUnknownProvider = errors.New("llm/providers: unknown provider")

// ErrNotBuiltIn is a valid value of MV_LLM_PROVIDER whose implementation is
// not registered in this binary (ollama before T-254, anthropic before E-H).
var ErrNotBuiltIn = errors.New("llm/providers: provider not built into this binary")

// Registry maps provider names to factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds the factory of provider name. A name outside MV_LLM_PROVIDER,
// a nil factory or a second registration of a name is a programming error of
// the provider package and panics at init, the way database/sql.Register does:
// a registry that silently kept the first or the last one would make the
// provider depend on the order of imports.
func (r *Registry) Register(name string, f Factory) {
	if err := llm.CheckProvider(name); err != nil {
		panic(fmt.Sprintf("llm/providers: Register %q: %v", name, err))
	}
	if f == nil {
		panic(fmt.Sprintf("llm/providers: Register %q: nil factory", name))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic(fmt.Sprintf("llm/providers: Register %q: registered twice", name))
	}
	r.factories[name] = f
}

// New builds provider name from cfg. It refuses a name that is not a value of
// MV_LLM_PROVIDER, a value this binary has no implementation of, a cfg that
// names a different provider and an endpoint the cloud gate refuses — in that
// order, and before the factory runs.
func (r *Registry) New(name string, cfg llm.Config) (llm.Provider, error) {
	if err := llm.CheckProvider(name); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnknownProvider, err)
	}
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		registered := r.Names()
		list := "none"
		if len(registered) > 0 {
			list = strings.Join(registered, ", ")
		}
		return nil, fmt.Errorf("%w: %q is a value of MV_LLM_PROVIDER, but this binary has no implementation of it (built in: %s)",
			ErrNotBuiltIn, name, list)
	}
	switch cfg.Provider {
	case "":
		cfg.Provider = name
	case name:
	default:
		return nil, fmt.Errorf("llm/providers: New %q with a configuration of provider %q", name, cfg.Provider)
	}
	if err := cfg.CheckCloudGate(); err != nil {
		return nil, fmt.Errorf("llm/providers: provider %s does not start: %w", name, err)
	}
	p, err := f(cfg)
	if err != nil {
		return nil, fmt.Errorf("llm/providers: provider %s: %w", name, err)
	}
	if p == nil {
		return nil, fmt.Errorf("llm/providers: provider %s: the factory returned no provider", name)
	}
	return p, nil
}

// Names lists the registered providers in lexical order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var defaultRegistry = NewRegistry()

// Register adds a factory to the registry of the process.
func Register(name string, f Factory) { defaultRegistry.Register(name, f) }

// New builds a provider from the registry of the process.
func New(name string, cfg llm.Config) (llm.Provider, error) { return defaultRegistry.New(name, cfg) }

// Names lists the providers registered in the process.
func Names() []string { return defaultRegistry.Names() }
