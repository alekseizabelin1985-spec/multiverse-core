// Package gateway is the context gateway of the platform: the player entry
// point over HTTP (C-08), the links of external accounts and, in later tasks,
// actions, sessions, rounds and the outbox (component gateway-and-bot.md).
//
// The context owns no HTTP server: it mounts its routes on the mux of the
// process server of shared/runtime, which listens on MV_CORE_ADDR and serves
// GET /health from the Health of every context (C-01 v1.3).
package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
)

// Name is the name the context is registered and selected under.
const Name = "gateway"

// Context is the gateway context. It is built by New and started once.
type Context struct {
	src env.Source

	// Built in New and mounted in Routes; their dependencies are set in Start,
	// before the process server serves (api.Config).
	httpCfg *api.Config
	links   *handlers.Links

	mu        sync.Mutex
	started   bool
	mode      runtime.Mode
	linksDB   *sql.DB
	gatewayDB *sql.DB
	store     *links.SQLite
	log       *slog.Logger
	stopLoop  context.CancelFunc
	loopDone  chan struct{}
}

// New returns the context reading its variables from src; nil is the process
// environment. The variables are read in Start, not here: a factory runs when
// serve builds the contexts, and an error belongs to the start of the process.
func New(src env.Source) *Context {
	return &Context{src: src, httpCfg: &api.Config{}, links: &handlers.Links{}}
}

var (
	_ runtime.Context = (*Context)(nil)
	_ runtime.Routes  = (*Context)(nil)
)

func (c *Context) Name() string { return Name }

// DependsOn is empty: the gateway reaches state and swarm over the bus only.
func (c *Context) DependsOn() []string { return nil }

// Routes mounts the operations of the gateway with the middleware of
// component §5.1 on the mux of the process. /health is not among them: the
// process server serves it (api servedByProcess).
func (c *Context) Routes(mux *http.ServeMux) {
	router := api.GatewayRouter(api.Handlers{
		ResolveLink: http.HandlerFunc(c.links.Resolve),
		ConsentLink: http.HandlerFunc(c.links.Consent),
		ForgetLink:  http.HandlerFunc(c.links.Forget),
	})
	router.Mount(mux, api.Chain(c.httpCfg)...)
}

// Start opens and migrates links.db and gateway.db in MV_GATEWAY_DATA_DIR,
// builds the links store, compacts links.db once and, in live mode, starts the
// sweeper of links.db.
//
// The compaction at the start is unconditional. The mark "compaction pending"
// lives in memory: a process that stopped between the DELETE of a /forget and
// its checkpoint, or while the mark was set, may have left the bytes of an
// external ID in the WAL or in free pages, and the new process knows nothing
// of it. A failure does not stop the start: it sets the mark again, /forget
// answers 503 and Health is degraded until the sweeper finishes the wipe.
func (c *Context) Start(ctx context.Context, deps runtime.Deps) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return errors.New("gateway: started twice")
	}
	if deps.Clock == nil || deps.Timers == nil || deps.IDs == nil || deps.Log == nil {
		return errors.New("gateway: Deps.Clock, Timers, IDs and Log are required")
	}
	dir, err := store.DataDir(c.src)
	if err != nil {
		return err
	}
	// Opening and migrating two local files is short and bounded by
	// busy_timeout; the context of Start is the life of the process, and a
	// signal that arrives meanwhile stops the process right after the start
	// instead of failing it halfway with the files half open.
	linksDB, gatewayDB, err := openDatabases(context.WithoutCancel(ctx), dir)
	if err != nil {
		// The default of the manifest is the volume of compose; a process
		// started on a host meets it as the root of the file system.
		return fmt.Errorf("gateway: data directory %s (%s): %w; outside compose set %s to a directory the process may create",
			dir, env.GatewayDataDir.Name(), err, env.GatewayDataDir.Name())
	}
	linkStore, err := links.NewSQLite(linksDB, deps.IDs)
	if err != nil {
		return errors.Join(err, gatewayDB.Close(), linksDB.Close())
	}
	log := deps.Log.With(slog.String("context", Name))
	if err := linkStore.Compact(context.WithoutCancel(ctx)); err != nil {
		log.Error("links compaction at start", slog.String("error", err.Error()))
	}

	*c.httpCfg = api.Config{
		ClientIDs:        env.GatewayClientIDs.ListFrom(c.src),
		ActorKindClients: env.GatewayActorKindClients.ListFrom(c.src),
		// X-Request-Id is not domain data and does not draw on Deps.IDs: with
		// --id-source=sequence a request would shift the link_id and player_id
		// of everything after it.
		RequestIDs: uuid.NewString,
		Clock:      deps.Clock,
		Log:        log,
	}
	*c.links = handlers.Links{Store: linkStore, Clock: deps.Clock}

	c.linksDB, c.gatewayDB, c.store, c.log, c.mode = linksDB, gatewayDB, linkStore, log, deps.Mode
	c.started = true
	// Replay drives no timers of its own (component §11.2).
	if deps.Mode != runtime.ModeReplay {
		loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		c.stopLoop, c.loopDone = cancel, make(chan struct{})
		go c.sweep(loopCtx, deps.Clock, deps.Timers, c.loopDone)
	}
	return nil
}

func openDatabases(ctx context.Context, dir string) (*sql.DB, *sql.DB, error) {
	linksDB, err := store.OpenLinks(ctx, store.LinksPath(dir))
	if err != nil {
		return nil, nil, err
	}
	if _, err := store.MigrateLinks(ctx, linksDB); err != nil {
		return nil, nil, errors.Join(err, linksDB.Close())
	}
	gatewayDB, err := store.OpenGateway(ctx, store.GatewayPath(dir))
	if err != nil {
		return nil, nil, errors.Join(err, linksDB.Close())
	}
	if _, err := store.MigrateGateway(ctx, gatewayDB); err != nil {
		return nil, nil, errors.Join(err, gatewayDB.Close(), linksDB.Close())
	}
	return linksDB, gatewayDB, nil
}

// sweep runs the housekeeping of links.db: every store.SweepInterval the
// expired character requests go and a pending compaction is retried, and
// every store.LinksCompactInterval links.db is compacted as the safety net of
// /forget (ADR-019 addendum p. 1). Errors are logged; the next tick repeats.
func (c *Context) sweep(ctx context.Context, clk clock.Clock, timers clock.Timers, done chan<- struct{}) {
	defer close(done)
	sweep, compact := timers.Every(store.SweepInterval), timers.Every(store.LinksCompactInterval)
	defer sweep.Stop()
	defer compact.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sweep.C():
			if err := c.store.Sweep(ctx, clk.Now()); err != nil && ctx.Err() == nil {
				c.log.Error("links sweep", slog.String("error", err.Error()))
			}
		case <-compact.C():
			if err := c.store.Compact(ctx); err != nil && ctx.Err() == nil {
				c.log.Error("links compaction", slog.String("error", err.Error()))
			}
		}
	}
}

// Stop stops the sweeper and waits for it, finishes a pending compaction, then
// closes gateway.db and links.db. The databases are closed even when the
// sweeper does not stop before the deadline of ctx: a context that is not
// started again must not keep its files open (Windows locks them). The context
// holds no subscription yet; when the consumer arrives (T-304) it is cancelled
// here first, before the process closes the bus (C-01 v1.7, ADR-023 p. 4).
func (c *Context) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return nil
	}
	c.started = false
	var errs []error
	stopped := true
	if c.stopLoop != nil {
		c.stopLoop()
		select {
		case <-c.loopDone:
		case <-ctx.Done():
			stopped = false
			errs = append(errs, fmt.Errorf("gateway: sweeper did not stop: %w", ctx.Err()))
		}
	}
	if c.store.CompactionPending() {
		// A sweeper that did not stop may still hold the only connection.
		err := errors.New("the sweeper did not stop")
		if stopped {
			err = c.store.Compact(ctx)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("gateway: links.db left uncompacted after /forget: %w", err))
		}
	}
	if err := c.gatewayDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("gateway: close gateway.db: %w", err))
	}
	if err := c.linksDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("gateway: close links.db: %w", err))
	}
	return errors.Join(errs...)
}

// Health is ok while the context runs, degraded while a /forget waits for its
// compaction, fail before Start and after Stop.
func (c *Context) Health() runtime.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return runtime.Status{Status: runtime.StatusFail, Details: map[string]any{"started": false}}
	}
	details := map[string]any{
		"links_store":   runtime.StatusOK,
		"gateway_store": runtime.StatusOK,
		"mode":          string(c.mode),
	}
	if c.store.CompactionPending() {
		details["links_compaction"] = "pending"
		return runtime.Status{Status: runtime.StatusDegraded, Details: details}
	}
	return runtime.Status{Status: runtime.StatusOK, Details: details}
}
