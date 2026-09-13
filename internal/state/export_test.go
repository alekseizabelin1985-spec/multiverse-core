package state

import "multiverse-core.io/internal/state/memstore"

// NewOver is New over a working set that already holds the worlds: a process
// restarted over the world its store kept. Until recovery exists (T-059) it is
// how a test stands for the world a restart would find.
func NewOver(cfg Config, store *memstore.Store) *Context { return newOver(cfg, store) }

// Store is the working set of the context, for a test that restarts over it.
func (c *Context) Store() *memstore.Store { return c.store }
