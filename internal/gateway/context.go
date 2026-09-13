// Package gateway is the context gateway of the platform: the player entry
// point over HTTP (C-08), the links of external accounts, the projection of
// the world read from the bus and, in later tasks, actions, sessions, rounds
// and the outbox (component gateway-and-bot.md).
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
	"time"

	"github.com/google/uuid"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
)

// Name is the name the context is registered and selected under.
const Name = "gateway"

// The budgets of the reads Start waits for. Both run under a context without
// the cancellation of the process, and neither the kafka adapter nor MinIO
// bounds a read by itself: the adapter takes the deadline of its connection
// from the context only, so a broker that accepts the connection and stays
// silent would hold the start forever.
const (
	// SnapshotLoadBudget bounds the load of the snapshot of State. A load
	// past it leaves the projection missing with ReasonSnapshotTimeout, and
	// the start goes on from the journal alone.
	SnapshotLoadBudget = 30 * time.Second
	// CatchUpBudget bounds the catch-up of the journal. A catch-up past it
	// fails the start: a gateway that validates actions against a projection
	// it could not bring up to date would answer from an old world.
	CatchUpBudget = 2 * time.Minute
)

// Context is the gateway context. It is built by New and started once.
type Context struct {
	src env.Source
	// objects replaces the object store built from MV_MINIO_* (tests).
	objects objstore.Client
	// loadBudget and catchUpBudget are SnapshotLoadBudget and CatchUpBudget
	// unless a test shortened them.
	loadBudget, catchUpBudget time.Duration

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
	model     *readmodel.Model
	consumer  *consumer.Dispatcher
	log       *slog.Logger
	stopLoop  context.CancelFunc
	loopDone  chan struct{}
}

// New returns the context reading its variables from src; nil is the process
// environment. The variables are read in Start, not here: a factory runs when
// serve builds the contexts, and an error belongs to the start of the process.
func New(src env.Source) *Context {
	return &Context{src: src, httpCfg: &api.Config{}, links: &handlers.Links{},
		loadBudget: SnapshotLoadBudget, catchUpBudget: CatchUpBudget}
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
// builds the links store, compacts links.db once, loads the projection from
// the snapshot of State of MV_WORLD_ID, catches it up from the journal and
// subscribes to the bus, and, in live mode, starts the sweeper.
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
	if deps.Clock == nil || deps.Timers == nil || deps.IDs == nil || deps.Log == nil || deps.Bus == nil || deps.Journal == nil {
		return errors.New("gateway: Deps.Clock, Timers, IDs, Log, Bus and Journal are required")
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

	// The load and the catch-up are reads the start of the process waits for,
	// like the opening of the files above: the projection has to be current
	// before the first request is validated against it. Unlike the files they
	// cross the network, so each has a budget of its own.
	model := readmodel.New(readmodel.Config{Timers: deps.Timers, Log: log})
	loadCtx, cancelLoad := context.WithTimeout(context.WithoutCancel(ctx), c.loadBudget)
	from := c.loadProjection(loadCtx, model, log)
	cancelLoad()
	dispatcher, err := consumer.New(consumer.Config{
		Bus: deps.Bus, Journal: deps.Journal, DB: gatewayDB, Model: model, Clock: deps.Clock, Log: log,
	})
	if err == nil {
		catchUpCtx, cancelCatchUp := context.WithTimeout(context.WithoutCancel(ctx), c.catchUpBudget)
		err = dispatcher.Start(catchUpCtx, from)
		if err != nil && errors.Is(catchUpCtx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("catch-up of the journal did not finish within %s: %w", c.catchUpBudget, err)
		}
		cancelCatchUp()
	}
	if err != nil {
		return errors.Join(fmt.Errorf("gateway: %w", err), gatewayDB.Close(), linksDB.Close())
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
	c.model, c.consumer = model, dispatcher
	c.started = true
	// Replay drives no timers of its own (component §11.2).
	if deps.Mode != runtime.ModeReplay {
		loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		c.stopLoop, c.loopDone = cancel, make(chan struct{})
		go c.sweep(loopCtx, deps.Clock, deps.Timers, c.loopDone)
	}
	return nil
}

// loadProjection loads the snapshot of State into model and returns its cursor;
// nil when there is none, and the projection is caught up from the start of
// the journal. A missing snapshot is the start of a new world and is logged as
// a warning; an object store configured wrong, a store that cannot be read or
// a snapshot that does not check is an error and leaves /health degraded
// (US-011: the gateway does not come up empty in silence).
func (c *Context) loadProjection(ctx context.Context, model *readmodel.Model, log *slog.Logger) map[string]int64 {
	world := env.WorldID.StringFrom(c.src)
	objects, err := c.objectStore()
	if err != nil {
		model.MarkLoadFailed(readmodel.ReasonStoreMisconfigured)
		log.Error("projection: object store is configured wrong, built from the journal alone",
			slog.String("world_id", world), slog.String("error", err.Error()))
		return nil
	}
	from, err := model.LoadFromStateSnapshot(ctx, objects, world)
	switch {
	case err == nil:
		log.Info("projection loaded from the snapshot of state", slog.String("world_id", world))
	case errors.Is(err, readmodel.ErrNoSnapshot):
		log.Warn("projection: no snapshot of state, built from the journal alone",
			slog.String("world_id", world), slog.String("reason", err.Error()))
	default:
		log.Error("projection: snapshot of state not loaded, built from the journal alone",
			slog.String("world_id", world), slog.String("error", err.Error()))
	}
	return from
}

// objectStore is the store the projection is loaded from: the one a test put
// in, or a client over MV_MINIO_*. Without either key there is no store — a
// process on the memory bus runs without MinIO — and the snapshot is reported
// missing. One key without the other is an error, like every other setting
// that does not make a client: a store half configured is not a new world.
// The errors name the variables, never their values.
func (c *Context) objectStore() (objstore.Client, error) {
	if c.objects != nil {
		return c.objects, nil
	}
	cfg := objstore.Config{
		Endpoint:  env.MinIOEndpoint.StringFrom(c.src),
		AccessKey: env.MinIOAccessKey.StringFrom(c.src),
		SecretKey: env.MinIOSecretKey.StringFrom(c.src),
	}
	switch {
	case cfg.AccessKey == "" && cfg.SecretKey == "":
		return nil, nil
	case cfg.AccessKey == "" || cfg.SecretKey == "":
		return nil, fmt.Errorf("gateway: %s and %s are set together or not at all",
			env.MinIOAccessKey.Name(), env.MinIOSecretKey.Name())
	}
	useSSL, err := env.MinIOUseSSL.BoolFrom(c.src)
	if err != nil {
		return nil, err
	}
	cfg.UseSSL = useSSL
	return objstore.New(cfg)
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

// sweep runs the housekeeping: every store.SweepInterval the expired character
// requests and the old marks of processed events go and a pending compaction
// is retried, and
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
			now := clk.Now()
			if err := c.store.Sweep(ctx, now); err != nil && ctx.Err() == nil {
				c.log.Error("links sweep", slog.String("error", err.Error()))
			}
			if err := c.consumer.Sweep(ctx, now); err != nil && ctx.Err() == nil {
				c.log.Error("processed events sweep", slog.String("error", err.Error()))
			}
		case <-compact.C():
			if err := c.store.Compact(ctx); err != nil && ctx.Err() == nil {
				c.log.Error("links compaction", slog.String("error", err.Error()))
			}
		}
	}
}

// Stop cancels the subscriptions and waits for their handlers, stops the
// sweeper and waits for it, finishes a pending compaction, then closes
// gateway.db and links.db. The subscriptions go first: the process closes the
// bus after the last Stop, and Close does not cancel a handler (C-01 v1.7,
// ADR-023 p. 4). The databases are closed even when a handler or the sweeper
// does not stop before the deadline of ctx: a context that is not started
// again must not keep its files open (Windows locks them).
func (c *Context) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return nil
	}
	c.started = false
	var errs []error
	if err := c.consumer.Stop(ctx); err != nil {
		errs = append(errs, fmt.Errorf("gateway: %w", err))
	}
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

// Health is ok while the context runs; fail before Start and after Stop.
// It is degraded while a /forget waits for its compaction, while a
// subscription is down, while the projection is stale, and when the snapshot
// of State could not be loaded — the store configured wrong, unreadable, the
// snapshot not checking. A world without a snapshot yet is reported as
// projection missing and does not degrade the gateway: it is how every new
// world starts, the empty one of the memory bus included. projection_error is
// a short code (readmodel.Reason*); the error itself is in the log, because it
// names the bucket, the key and the address of the store.
func (c *Context) Health() runtime.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return runtime.Status{Status: runtime.StatusFail, Details: map[string]any{"started": false}}
	}
	projection, loadErr := c.model.Status()
	details := map[string]any{
		"links_store":   runtime.StatusOK,
		"gateway_store": runtime.StatusOK,
		"projection":    string(projection),
		"mode":          string(c.mode),
	}
	status := runtime.StatusOK
	if c.store.CompactionPending() {
		details["links_compaction"] = "pending"
		status = runtime.StatusDegraded
	}
	if loadErr != "" {
		details["projection_error"] = loadErr
		status = runtime.StatusDegraded
	}
	if projection == readmodel.ProjectionStale {
		status = runtime.StatusDegraded
	}
	if err := c.consumer.Err(); err != nil {
		details["bus"] = runtime.StatusFail
		status = runtime.StatusDegraded
	}
	return runtime.Status{Status: status, Details: details}
}
