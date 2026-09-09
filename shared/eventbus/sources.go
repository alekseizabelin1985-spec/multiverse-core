package eventbus

import (
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"multiverse-core.io/shared/clock"
)

// The identifier generator, the clock and the contract registry are process
// wide: the constructors are plain functions called from every publisher, and
// threading three dependencies through each of them would buy nothing. The
// process installs them once at start (cmd/multiverse); a test installs a
// sequence generator and a manual clock and gets byte-identical events on
// every run (foundation.md §5.2).
var (
	sourcesMu  sync.RWMutex
	idSource   = uuid.NewString
	eventClock clock.Clock
	registry   Registry
)

// SetIDSource installs the generator of event identifiers. Passing nil
// restores the default (UUID v4).
func SetIDSource(gen func() string) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	if gen == nil {
		gen = uuid.NewString
	}
	idSource = gen
}

// SetClock installs the clock the constructors stamp root events with. Passing
// nil restores the wall clock.
func SetClock(c clock.Clock) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	eventClock = c
}

// SetRegistry installs the contract registry the constructors read the schema
// version from and the bus routes by. Passing nil clears it.
func SetRegistry(r Registry) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	registry = r
}

// PackageRegistry returns the registry installed with SetRegistry, or nil.
func PackageRegistry() Registry {
	sourcesMu.RLock()
	defer sourcesMu.RUnlock()
	return registry
}

// SequenceIDs returns a deterministic generator producing prefix-1, prefix-2
// and so on. It is the generator of tests and of replay runs.
func SequenceIDs(prefix string) func() string {
	var mu sync.Mutex
	n := 0
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return prefix + "-" + strconv.Itoa(n)
	}
}

func nextID() string {
	sourcesMu.RLock()
	gen := idSource
	sourcesMu.RUnlock()
	return gen()
}

func nowUTC() time.Time {
	sourcesMu.RLock()
	c := eventClock
	sourcesMu.RUnlock()
	if c == nil {
		c = clock.Real{}
	}
	return c.Now().UTC()
}

// schemaVersionOf reads the payload schema version of a type from the
// registry. An unregistered type gets 1: publishing it fails later in Route,
// which reports the unknown type instead of a confusing schema mismatch.
func schemaVersionOf(typ string) int {
	reg := PackageRegistry()
	if reg == nil {
		return 1
	}
	spec, ok := reg.Lookup(typ)
	if !ok || spec.SchemaVersion == 0 {
		return 1
	}
	return spec.SchemaVersion
}
