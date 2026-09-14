// Package gatewaytest runs the real gateway in-process for tests, the way
// net/http/httptest runs a real HTTP server: FakeGateway is internal/gateway
// with its SQLite files, its middleware and its outbox, on the memory bus,
// behind a process HTTP server on a loopback port (C-08 "Заглушка", ADR-001
// addendum 2026-09-13, boundary 1e).
//
// It is a double about where the gateway runs, never about what it answers:
// nothing of the gateway is replaced. What it fixes is what a test needs to be
// repeatable — a data directory of its own, a manual clock, identifiers from a
// sequence and a bus in memory — and nothing else; State, the swarm and the
// world they answer with are the business of the test (testkit/state,
// testkit/swarm).
//
// The package is for tests only: production code does not import it (depguard
// no-testkit-in-production, .golangci.yml). A test that needs it lives outside
// internal/<context> other than the gateway, because the rules of the contexts
// hold in tests as well.
package gatewaytest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
)

// Epoch is where the manual clock of a FakeGateway starts when the test gives
// none.
var Epoch = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

// IDPrefix is the prefix of the identifiers a FakeGateway draws when the test
// gives no generator: player_id, link_id and the ids of the events the gateway
// publishes come out as player-gw-1, gw-2… (--id-source=sequence).
const IDPrefix = "gw"

// removeAttempts and removePause bound the removal of a data directory the
// FakeGateway made. On Windows the -wal and -shm files SQLite deletes on the
// last Close can stay "delete pending" for a moment while the indexer or an
// antivirus still holds them.
const (
	removeAttempts = 50
	removePause    = 100 * time.Millisecond
)

// Config builds a FakeGateway. Every field is optional.
type Config struct {
	// Bus is the bus and the journal of the gateway. Nil makes a membus over
	// every topic of the registry, which Close closes; a bus the test passed
	// is the test's to close.
	Bus *membus.Bus
	// Clock is the clock of the gateway. Nil is clock.NewManual(Epoch).
	Clock *clock.Manual
	// IDs draws the identifiers of the gateway. Nil is
	// eventbus.SequenceIDs(IDPrefix).
	IDs func() string
	// Mode is live when empty. In replay the gateway keeps no rate limit,
	// runs no sweeper and publishes no analytics (component §11.2).
	Mode runtime.Mode
	// Dir is the data directory of links.db and gateway.db. Empty makes a
	// temporary directory, which Close removes; a directory the test passed
	// is left where it is.
	Dir string
	// Vars are variables of the gateway on top of the defaults of the
	// manifest. The process environment is never read, and
	// MV_GATEWAY_DATA_DIR is always Dir.
	Vars map[string]string
	// Log receives the logs of the gateway. Nil discards them.
	Log *slog.Logger
}

// FakeGateway is a started gateway behind a process HTTP server.
type FakeGateway struct {
	// URL is the base address of the server, http://127.0.0.1:<port>.
	URL string
	// Dir is the data directory of the gateway.
	Dir string

	bus     *membus.Bus
	clock   *clock.Manual
	ctx     *gateway.Context
	srv     *runtime.HTTP
	ownBus  bool
	ownDir  bool
	closing sync.Once
	closed  error
}

// Start starts a gateway the way serve does: routes on the mux of the process
// server, the context, and only then the server. Whatever it made before a
// failure is released before it returns.
func Start(cfg Config) (*FakeGateway, error) {
	g := &FakeGateway{Dir: cfg.Dir, bus: cfg.Bus, clock: cfg.Clock}
	if g.clock == nil {
		g.clock = clock.NewManual(Epoch)
	}
	ids := cfg.IDs
	if ids == nil {
		ids = eventbus.SequenceIDs(IDPrefix)
	}
	mode := cfg.Mode
	if mode == "" {
		mode = runtime.ModeLive
	}
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	if g.Dir == "" {
		dir, err := os.MkdirTemp("", "gatewaytest-")
		if err != nil {
			return nil, fmt.Errorf("gatewaytest: data directory: %w", err)
		}
		g.Dir, g.ownDir = dir, true
	}
	if g.bus == nil {
		bus, err := newBus()
		if err != nil {
			return nil, errors.Join(err, g.removeDir())
		}
		g.bus, g.ownBus = bus, true
	}

	vars := make(map[string]string, len(cfg.Vars)+1)
	for name, value := range cfg.Vars {
		vars[name] = value
	}
	vars[env.GatewayDataDir.Name()] = g.Dir
	g.ctx = gateway.New(env.MapSource(vars))
	g.srv = runtime.NewHTTP("127.0.0.1:0", g.ctx.Health)
	g.ctx.Routes(g.srv.Mux)
	deps := runtime.Deps{Bus: g.bus, Journal: g.bus, Contracts: contracts.Default(), Clock: g.clock,
		Timers: g.clock.Timers(), Mode: mode, IDs: ids, Log: log, Mux: g.srv.Mux}
	if err := g.ctx.Start(context.Background(), deps); err != nil {
		return nil, errors.Join(fmt.Errorf("gatewaytest: %w", err), g.closeBus(), g.removeDir())
	}
	if err := g.srv.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("gatewaytest: server: %w", err),
			g.ctx.Stop(context.Background()), g.closeBus(), g.removeDir())
	}
	g.URL = "http://" + g.srv.Addr
	return g, nil
}

// newBus makes the bus of a FakeGateway that was given none. It is a variable
// so that a test of the package can see the bus a failed Start closed.
var newBus = memoryBus

// memoryBus is a membus over every topic of the registry, without pauses
// between the retries of a handler.
func memoryBus() (*membus.Bus, error) {
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics, Backoff: []time.Duration{0, 0, 0}})
	if err != nil {
		return nil, fmt.Errorf("gatewaytest: membus: %w", err)
	}
	return bus, nil
}

// Bus is the bus of the gateway: a test publishes the answers of State and the
// swarm on it and reads what the gateway published.
func (g *FakeGateway) Bus() *membus.Bus { return g.bus }

// Clock is the manual clock of the gateway.
func (g *FakeGateway) Clock() *clock.Manual { return g.clock }

// Health is the health of the gateway context, as /health reports it.
func (g *FakeGateway) Health() runtime.Status { return g.ctx.Health() }

// Client is a client of the gateway under clientID, the same client the bot
// uses (NFR-092). It does not repeat: a test sees the first answer.
func (g *FakeGateway) Client(clientID string) *client.Client {
	c := client.New(g.URL, clientID)
	c.Backoff = client.NoRetry
	return c
}

// Close stops the server — a waiting long-poll answers an empty list at once
// (C-01 v1.8) — then the gateway, then the bus and the data directory when the
// FakeGateway made them. It is safe to call more than once; every call returns
// what the first one did.
func (g *FakeGateway) Close() error {
	g.closing.Do(func() {
		g.closed = errors.Join(g.srv.Stop(context.Background()), g.ctx.Stop(context.Background()),
			g.closeBus(), g.removeDir())
	})
	return g.closed
}

func (g *FakeGateway) closeBus() error {
	if !g.ownBus {
		return nil
	}
	return g.bus.Close()
}

func (g *FakeGateway) removeDir() error {
	if !g.ownDir {
		return nil
	}
	var err error
	for attempt := range removeAttempts {
		if err = os.RemoveAll(g.Dir); err == nil {
			return nil
		}
		if attempt < removeAttempts-1 {
			<-clock.RealTimers{}.After(removePause).C()
		}
	}
	return fmt.Errorf("gatewaytest: remove %s: %w", g.Dir, err)
}
