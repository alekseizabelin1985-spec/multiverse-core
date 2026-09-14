package state

import (
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/eventbus"
)

// NewOver is New over a working set that already holds the worlds: a process
// that holds a world in memory without recovering it.
func NewOver(cfg Config, store *memstore.Store) *Context { return newOver(cfg, store) }

// Store is the working set of the context, for a test that restarts over it.
func (c *Context) Store() *memstore.Store { return c.store }

// CatchUpFact applies one fact of the journal to the world by the rule of
// catching up, as recovery does (§4.8).
func (a *Applier) CatchUpFact(ev eventbus.Event) (bool, error) { return a.catchUp(ev) }
