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

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/characters"
	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/snapshot"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
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
	// StaleRepairBudget bounds the read of the snapshot of State that repairs
	// the stale entities of the projection (component §11.2). The repair runs
	// in the handler of system_events: past the budget the entities stay stale
	// and the event goes on.
	StaleRepairBudget = 5 * time.Second
	// SnapshotWriteBudget bounds one snapshot of the gateway: the object, the
	// pointer, the rotation and snapshot.created. A store that hangs costs
	// write_failed in /health, not the goroutine of the snapshots until Stop
	// (N-4 of review #1 of T-309). Like the other budgets it is a deadline of
	// the context: the clients of the store and of the bus take their
	// deadlines from it.
	SnapshotWriteBudget = 30 * time.Second
)

// Context is the gateway context. It is built by New and started once.
type Context struct {
	src env.Source
	// objects replaces the object store built from MV_MINIO_* (tests).
	objects objstore.Client
	// probeInterval is HealthProbeInterval unless a test shortened it.
	probeInterval time.Duration
	// repairBudget and snapshotBudget are StaleRepairBudget and
	// SnapshotWriteBudget unless a test shortened them.
	repairBudget, snapshotBudget time.Duration
	// loadBudget and catchUpBudget are SnapshotLoadBudget and CatchUpBudget
	// unless a test shortened them.
	loadBudget, catchUpBudget time.Duration

	// Built in New and mounted in Routes; their dependencies are set in Start,
	// before the process server serves (api.Config).
	httpCfg    *api.Config
	links      *handlers.Links
	actions    *handlers.Actions
	characters *handlers.Characters
	deliveries *handlers.Deliveries

	mu        sync.Mutex
	started   bool
	mode      runtime.Mode
	linksDB   *sql.DB
	gatewayDB *sql.DB
	store     *links.SQLite
	model     *readmodel.Model
	consumer  *consumer.Dispatcher
	keys      *actions.Keys
	service   *actions.Service
	limiter   *actions.Limiter
	sessions  *session.Manager
	turns     *turns.Tracker
	chars     *characters.Service
	outbox    *outbox.Store
	log       *slog.Logger
	stopLoop  context.CancelFunc
	loopDone  chan struct{}
	// snaps is nil without an object store: there is nowhere to write to.
	snaps *snapshots
	probe *probe
	// stopping is set while Stop runs without the lock: a Start meanwhile
	// would open the files Stop is about to close.
	stopping bool

	// stateMu guards snapshotState, which the writer goroutine sets while
	// Health reads it.
	stateMu       sync.Mutex
	snapshotState string
}

// New returns the context reading its variables from src; nil is the process
// environment. The variables are read in Start, not here: a factory runs when
// serve builds the contexts, and an error belongs to the start of the process.
func New(src env.Source) *Context {
	return &Context{src: src, httpCfg: &api.Config{}, links: &handlers.Links{}, actions: &handlers.Actions{},
		characters: &handlers.Characters{}, deliveries: &handlers.Deliveries{},
		loadBudget: SnapshotLoadBudget, catchUpBudget: CatchUpBudget, probeInterval: HealthProbeInterval,
		repairBudget: StaleRepairBudget, snapshotBudget: SnapshotWriteBudget}
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
		ResolveLink:      http.HandlerFunc(c.links.Resolve),
		ConsentLink:      http.HandlerFunc(c.links.Consent),
		ForgetLink:       http.HandlerFunc(c.links.Forget),
		ListWorlds:       http.HandlerFunc(c.characters.ListWorlds),
		CreateCharacter:  http.HandlerFunc(c.characters.Create),
		GetPlayer:        http.HandlerFunc(c.characters.Player),
		PostAction:       http.HandlerFunc(c.actions.Post),
		PollDeliveries:   http.HandlerFunc(c.deliveries.Poll),
		AckDeliveries:    http.HandlerFunc(c.deliveries.Ack),
		StreamDeliveries: http.HandlerFunc(c.deliveries.Stream),
	})
	router.Mount(mux, api.Chain(c.httpCfg)...)
}

// Start opens and migrates links.db and gateway.db in MV_GATEWAY_DATA_DIR,
// builds the links store, compacts links.db once, loads the projection from
// the snapshot of State of MV_WORLD_ID, restores the sessions and the turns
// (component §7.6), catches the projection and the effects up from the journal
// to its end and subscribes to the bus, checks the last snapshot of the
// gateway and starts its writer, and, in live mode, starts the sweeper.
//
// The restoration runs before the catch-up, the subscriptions and the first
// request: the sessions idle for MV_GATEWAY_SESSION_IDLE end idle at the
// moment they became idle, and the turns past their deadline time out. In
// replay their analytics are not published (component §11.2).
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
	switch {
	case c.started:
		return errors.New("gateway: started twice")
	case c.stopping:
		return errors.New("gateway: start while stopping")
	}
	if deps.Clock == nil || deps.Timers == nil || deps.IDs == nil || deps.Log == nil || deps.Bus == nil || deps.Journal == nil {
		return errors.New("gateway: Deps.Clock, Timers, IDs, Log, Bus and Journal are required")
	}
	dir, err := store.DataDir(c.src)
	if err != nil {
		return err
	}
	// The settings of actions are read before a file is opened: a start that
	// fails on one of them must not leave the databases half open.
	settings, err := c.actionSettings()
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
	// The outbox is the first step of the cascade of /forget (C-04 v1.1): the
	// pending deliveries of the player are dropped before the link goes.
	queue, err := outbox.New(outbox.Config{DB: gatewayDB, Lease: settings.deliveryLease, TTL: settings.deliveryTTL})
	if err != nil {
		return errors.Join(fmt.Errorf("gateway: %w", err), gatewayDB.Close(), linksDB.Close())
	}
	linkStore, err := links.NewSQLite(linksDB, deps.IDs, links.ForgetFunc(func(ctx context.Context, playerID string) error {
		_, err := queue.DropForPlayer(ctx, playerID)
		return err
	}))
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
	objects, objectsErr := c.objectStore()
	loadCtx, cancelLoad := context.WithTimeout(context.WithoutCancel(ctx), c.loadBudget)
	from := c.loadProjection(loadCtx, objects, objectsErr, model, log)
	cancelLoad()
	// Snapshots need a store and a world to write to; a process on the memory
	// bus has neither store nor snapshots. A replay only checks the last one.
	world := env.WorldID.StringFrom(c.src)
	var snaps *snapshots
	if objects != nil && world != "" {
		snaps = &snapshots{live: deps.Mode != runtime.ModeReplay, trigger: make(chan struct{}, 1), done: make(chan struct{}),
			setState: c.setSnapshotState, budget: c.snapshotBudget}
	}
	svc, err := c.services(deps, settings, gatewayDB, linkStore, model, snaps, log)
	if err != nil {
		return errors.Join(fmt.Errorf("gateway: %w", err), gatewayDB.Close(), linksDB.Close())
	}
	restore(context.WithoutCancel(ctx), deps.Clock.Now(), svc, log)
	// The effects of the facts of a character proposal: its row goes with its
	// fact, and a refusal of State expires it (component §7.1). Then the
	// deliveries and the steps of the turns (component §8.1, §7.6).
	feed := &consumer.Deliveries{Model: model, Outbox: queue, Links: linkStore, Turns: svc.turns, Clock: deps.Clock, Log: log}
	fed, err := feed.Effects()
	if err != nil {
		return errors.Join(fmt.Errorf("gateway: %w", err), gatewayDB.Close(), linksDB.Close())
	}
	dispatcher, err := consumer.New(consumer.Config{
		Bus: deps.Bus, Journal: deps.Journal, DB: gatewayDB, Model: model, Clock: deps.Clock, Log: log,
		Effects: consumer.MergeEffects(map[string][]consumer.Effect{
			readmodel.TypeEntityCreated:  {svc.chars.OnCreated},
			readmodel.TypeUpdateRejected: {svc.chars.OnRejected},
		}, fed),
		Repair:  repairStale(model, objects, world, c.repairBudget, log),
		Repairs: func(ev eventbus.Event) bool { return readmodel.IsStateSnapshotOf(ev, world) },
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
		RequestIDs:   uuid.NewString,
		Clock:        deps.Clock,
		Log:          log,
		SetDeadlines: runtime.SetDeadlines,
	}
	// The wait of a long-poll is a property of the connection, like its
	// deadlines, and runs on the wall clock in every mode (C-01 v1.8): the null
	// timers of a replay would never end it.
	polls, err := outbox.NewService(outbox.ServiceConfig{Store: queue, Routes: linkStore, Clock: deps.Clock,
		Timers: clock.RealTimers{}, Log: log})
	if err != nil {
		return errors.Join(fmt.Errorf("gateway: %w", err), dispatcher.Stop(ctx), gatewayDB.Close(), linksDB.Close())
	}
	*c.links = handlers.Links{Store: linkStore, Clock: deps.Clock, CharacterStatus: svc.chars.Status}
	*c.actions = handlers.Actions{Service: svc.actions}
	*c.characters = handlers.Characters{Service: svc.chars, Worlds: model}
	*c.deliveries = handlers.Deliveries{Service: polls, Turns: svc.turns, Platforms: handlers.ClientPlatforms, Log: log}
	c.keys, c.service, c.sessions, c.turns, c.chars = svc.keys, svc.actions, svc.sessions, svc.turns, svc.chars
	// The rate limit is a limit of live players; a replay feeds recorded
	// actions as fast as the harness sends them (component §5.1 p. 6).
	if deps.Mode != runtime.ModeReplay {
		limiter, err := actions.NewLimiter(deps.Clock, settings.perMinute, settings.burst)
		if err != nil {
			return errors.Join(fmt.Errorf("gateway: %w", err), dispatcher.Stop(ctx), gatewayDB.Close(), linksDB.Close())
		}
		c.httpCfg.Limiter, c.limiter = limiter, limiter
	}

	c.linksDB, c.gatewayDB, c.store, c.log, c.mode = linksDB, gatewayDB, linkStore, log, deps.Mode
	c.model, c.consumer, c.outbox = model, dispatcher, queue
	// The cadence of the checks of /health is a property of the process, not of
	// the domain: in replay the clock of the events stands between events and
	// would hide a database that fails after the last one (component §11.4,
	// NFR-016). Only Now is taken from the wall clock, no timer.
	probeClock := deps.Clock
	if deps.Mode == runtime.ModeReplay {
		probeClock = clock.Real{}
	}
	// The age of the oldest delivery is a difference of two times of the
	// process: created_at is written by Deps.Clock, so the age is taken by it
	// too, in replay the clock of the events (Mi-4 of review #1 of T-309).
	c.probe = &probe{clock: probeClock, ageClock: deps.Clock, interval: c.probeInterval, linksDB: linksDB, gateway: gatewayDB,
		outbox: queue, sessions: svc.sessions}
	c.probe.read(context.WithoutCancel(ctx))
	c.setSnapshotState("")
	c.snaps = nil
	if snaps != nil {
		if err := c.startSnapshots(ctx, deps, snaps, objects, model, gatewayDB, svc.sessions, log); err != nil {
			return errors.Join(fmt.Errorf("gateway: %w", err), dispatcher.Stop(ctx), gatewayDB.Close(), linksDB.Close())
		}
		c.snaps = snaps
	}
	c.started = true
	// Replay drives no timers of its own (component §11.2).
	if deps.Mode != runtime.ModeReplay {
		loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		c.stopLoop, c.loopDone = cancel, make(chan struct{})
		go c.sweep(loopCtx, deps.Clock, deps.Timers, c.loopDone)
	}
	return nil
}

// actionSettings are the variables of actions, sessions, turns and characters
// (component §11.3).
type actionSettings struct {
	filter           actions.InputFilter
	filterName       string
	perMinute, burst int
	grace            time.Duration
	gmPath           string
	// MV_GATEWAY_SESSION_IDLE, _TURN_TIMEOUT, _CHARACTER_WAIT and
	// _CHARACTER_DEADLINE.
	sessionIdle, turnTimeout, characterWait, characterDeadline time.Duration
	// MV_GATEWAY_DELIVERY_LEASE and _DELIVERY_TTL.
	deliveryLease, deliveryTTL time.Duration
}

// services are the services over the databases and the projection.
type services struct {
	keys     *actions.Keys
	actions  *actions.Service
	sessions *session.Manager
	turns    *turns.Tracker
	chars    *characters.Service
}

// services builds the sessions, turns, actions and characters of a start.
// Analytics go to the bus in live mode only: a replay reads them and publishes
// none (C-10, component §11.2).
func (c *Context) services(deps runtime.Deps, s actionSettings, gatewayDB *sql.DB, linkStore *links.SQLite,
	model *readmodel.Model, snaps *snapshots, log *slog.Logger) (services, error) {
	var analytics session.Publisher
	if deps.Mode != runtime.ModeReplay {
		analytics = deps.Bus
	}
	cfg := session.Config{DB: gatewayDB, Bus: analytics, Idle: s.sessionIdle, Log: log}
	if snaps != nil && snaps.live {
		cfg.OnEnded = snaps.request
	}
	sessions, err := session.New(cfg)
	if err != nil {
		return services{}, err
	}
	tracker, err := turns.New(turns.Config{DB: gatewayDB, Sessions: sessions, Bus: analytics, Clock: deps.Clock,
		Timeout: s.turnTimeout, GMPath: s.gmPath, Log: log})
	if err != nil {
		return services{}, err
	}
	keys := actions.NewKeys(gatewayDB, store.KeyTTL)
	service, err := actions.New(actions.Config{
		Bus: deps.Bus, Model: model, Keys: keys, Filter: s.filter, FilterName: s.filterName,
		Turns: tracker, Clock: deps.Clock, Timers: deps.Timers, Log: log,
		GMPath: s.gmPath, Grace: s.grace, KeyTTL: store.KeyTTL,
	})
	if err != nil {
		return services{}, err
	}
	chars, err := characters.New(characters.Config{
		Links: linkStore, Model: model, Sessions: sessions, DB: gatewayDB, Bus: deps.Bus, Filter: service,
		IDs: deps.IDs, Clock: deps.Clock, Wait: s.characterWait, Deadline: s.characterDeadline, Log: log,
	})
	if err != nil {
		return services{}, err
	}
	return services{keys: keys, actions: service, sessions: sessions, turns: tracker, chars: chars}, nil
}

func (c *Context) actionSettings() (actionSettings, error) {
	s := actionSettings{filterName: env.GatewayInputFilter.StringFrom(c.src), gmPath: env.GMPath.StringFrom(c.src)}
	var errs []error
	var err error
	if s.filter, err = actions.FilterFor(s.filterName); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", env.GatewayInputFilter.Name(), err))
	}
	// A value that does not parse is one error, not also "below 1".
	for _, limit := range []struct {
		dst      *int
		variable env.Var
	}{{&s.perMinute, env.GatewayRateActionsPerMin}, {&s.burst, env.GatewayRateActionsBurst}} {
		switch *limit.dst, err = limit.variable.IntFrom(c.src); {
		case err != nil:
			errs = append(errs, err)
		case *limit.dst < 1:
			errs = append(errs, fmt.Errorf("%s must be at least 1", limit.variable.Name()))
		}
	}
	switch s.grace, err = env.GatewayEncounterGrace.DurationFrom(c.src); {
	case err != nil:
		errs = append(errs, err)
	case s.grace < 0:
		errs = append(errs, fmt.Errorf("%s must not be negative", env.GatewayEncounterGrace.Name()))
	}
	for _, d := range []struct {
		dst      *time.Duration
		variable env.Var
	}{
		{&s.sessionIdle, env.GatewaySessionIdle}, {&s.turnTimeout, env.GatewayTurnTimeout},
		{&s.characterWait, env.GatewayCharacterWait}, {&s.characterDeadline, env.GatewayCharacterDeadline},
		{&s.deliveryLease, env.GatewayDeliveryLease}, {&s.deliveryTTL, env.GatewayDeliveryTTL},
	} {
		switch *d.dst, err = d.variable.DurationFrom(c.src); {
		case err != nil:
			errs = append(errs, err)
		case *d.dst <= 0:
			errs = append(errs, fmt.Errorf("%s must be positive", d.variable.Name()))
		}
	}
	// The wait for the fact of a character ends before its deadline: with the
	// wait at or past the deadline, the sweeper could take a character off its
	// link while its request still waits for the fact (N-5 of review #1 of
	// T-306). Checked only when both parsed as positive durations.
	if s.characterWait > 0 && s.characterDeadline > 0 && s.characterWait >= s.characterDeadline {
		errs = append(errs, fmt.Errorf("%s must be less than %s", env.GatewayCharacterWait.Name(), env.GatewayCharacterDeadline.Name()))
	}
	if s.gmPath != eventbus.GMPathAgent && s.gmPath != eventbus.GMPathLegacy {
		errs = append(errs, fmt.Errorf("%s is %s or %s", env.GMPath.Name(), eventbus.GMPathAgent, eventbus.GMPathLegacy))
	}
	if len(errs) > 0 {
		return actionSettings{}, fmt.Errorf("gateway: %w", errors.Join(errs...))
	}
	return s, nil
}

// loadProjection loads the snapshot of State into model and returns its cursor;
// nil when there is none, and the projection is caught up from the start of
// the journal. A missing snapshot is the start of a new world and is logged as
// a warning; an object store configured wrong, a store that cannot be read or
// a snapshot that does not check is an error and leaves /health degraded
// (US-011: the gateway does not come up empty in silence).
func (c *Context) loadProjection(ctx context.Context, objects objstore.Client, err error, model *readmodel.Model,
	log *slog.Logger) map[string]int64 {
	world := env.WorldID.StringFrom(c.src)
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
// requests, the old marks of processed events, the expired answers to actions
// and the half published actions whose key expired go, a pending compaction
// is retried, the characters past their deadline leave their links, the turns
// past theirs time out, the idle sessions end, and the outbox releases its
// expired leases, drops what outlived its TTL and purges old rows; every
// actions.LimiterSweepInterval the rate limit forgets the players who stopped
// acting; and every store.LinksCompactInterval links.db is compacted as the
// safety net of /forget (ADR-019 addendum p. 1). Errors are logged; the next
// tick repeats.
func (c *Context) sweep(ctx context.Context, clk clock.Clock, timers clock.Timers, done chan<- struct{}) {
	defer close(done)
	sweep, compact := timers.Every(store.SweepInterval), timers.Every(store.LinksCompactInterval)
	limits := timers.Every(actions.LimiterSweepInterval)
	defer sweep.Stop()
	defer compact.Stop()
	defer limits.Stop()
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
			if err := c.keys.Sweep(ctx, now); err != nil && ctx.Err() == nil {
				c.log.Error("action keys sweep", slog.String("error", err.Error()))
			}
			c.service.SweepPending(now)
			if _, err := c.chars.Sweep(ctx, now); err != nil && ctx.Err() == nil {
				c.log.Error("pending characters sweep", slog.String("error", err.Error()))
			}
			if _, err := c.turns.Sweep(ctx, now); err != nil && ctx.Err() == nil {
				c.log.Error("turns sweep", slog.String("error", err.Error()))
			}
			if _, err := c.sessions.Sweep(ctx, now); err != nil && ctx.Err() == nil {
				c.log.Error("sessions sweep", slog.String("error", err.Error()))
			}
			if err := c.outbox.Sweep(ctx, now); err != nil && ctx.Err() == nil {
				c.log.Error("outbox sweep", slog.String("error", err.Error()))
			}
		case <-limits.C():
			c.limiter.Sweep(clk.Now())
		case <-compact.C():
			if err := c.store.Compact(ctx); err != nil && ctx.Err() == nil {
				c.log.Error("links compaction", slog.String("error", err.Error()))
			}
		}
	}
}

// Stop cancels the subscriptions and waits for their handlers, stops the
// sweeper and waits for it, finishes a pending compaction, stops the snapshot
// writer, writes the snapshot of the shutdown and announces it, then closes
// gateway.db and links.db.
//
// The subscriptions go first: the process closes the bus after the last Stop,
// and Close does not cancel a handler (C-01 v1.7, ADR-023 p. 4). The snapshot
// comes after everything that changes what it records has stopped, while the
// bus is still open for its snapshot.created; it is skipped when the sweeper
// or the writer did not stop, because either may still hold the only
// connection of gateway.db. The databases are closed even when something does
// not stop before the deadline of ctx: a context that is not started again
// must not keep its files open (Windows locks them).
//
// The lock of the context is held only to take the context out of service:
// Health answers fail at once while Stop waits (R2-N-3 of review #2 of T-303).
func (c *Context) Stop(ctx context.Context) error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	c.started, c.stopping = false, true
	dispatcher, stopLoop, loopDone, snaps := c.consumer, c.stopLoop, c.loopDone, c.snaps
	linkStore, gatewayDB, linksDB, log := c.store, c.gatewayDB, c.linksDB, c.log
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.stopping = false
		c.mu.Unlock()
	}()

	var errs []error
	if err := dispatcher.Stop(ctx); err != nil {
		errs = append(errs, fmt.Errorf("gateway: %w", err))
	}
	stopped := true
	if stopLoop != nil {
		stopLoop()
		select {
		case <-loopDone:
		case <-ctx.Done():
			stopped = false
			errs = append(errs, fmt.Errorf("gateway: sweeper did not stop: %w", ctx.Err()))
		}
	}
	// The wipe of /forget comes before the snapshot: it needs the local file
	// alone, and a store that hangs must not spend the deadline it has (Mi-2
	// of review #1 of T-309, ADR-019 addendum p. 1).
	if linkStore.CompactionPending() {
		// A sweeper that did not stop may still hold the only connection.
		err := errors.New("the sweeper did not stop")
		if stopped {
			err = linkStore.Compact(ctx)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("gateway: links.db left uncompacted after /forget: %w", err))
		}
	}
	if snaps != nil && snaps.live {
		if err := snaps.stop(ctx); err != nil {
			stopped = false
			errs = append(errs, err)
		}
		if stopped {
			if err := snaps.write(ctx, snapshot.ReasonShutdown); err != nil {
				log.Error("snapshot at the shutdown", slog.String("error", err.Error()))
				errs = append(errs, fmt.Errorf("gateway: snapshot at the shutdown: %w", err))
			}
		}
	}
	if err := gatewayDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("gateway: close gateway.db: %w", err))
	}
	if err := linksDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("gateway: close links.db: %w", err))
	}
	return errors.Join(errs...)
}

// Health is the /health of the gateway (component §11.4): fail before Start,
// after Stop and when either database fails its check; degraded while a
// /forget waits for its compaction, while a subscription is down, while the
// projection is stale, when the snapshot of State could not be loaded — the
// store configured wrong, unreadable, the snapshot not checking — and when the
// snapshot of the gateway is corrupted, could not be written, or cannot be
// written because the world has no laws_version. A world
// without a snapshot yet is reported as projection missing and does not
// degrade the gateway: it is how every new world starts, the empty one of the
// memory bus included. projection_error is a short code (readmodel.Reason*);
// the error itself is in the log, because it names the bucket, the key and
// the address of the store.
//
// The databases are read by probe, at most every HealthProbeInterval and never
// longer than HealthProbeDeadline, outside the lock of the context.
func (c *Context) Health() runtime.Status {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return runtime.Status{Status: runtime.StatusFail, Details: map[string]any{"started": false}}
	}
	model, dispatcher, linkStore, mode, pr := c.model, c.consumer, c.store, c.mode, c.probe
	c.mu.Unlock()

	v := pr.read(context.Background())
	projection, loadErr := model.Status()
	details := map[string]any{
		"links_store":         okOrFail(v.linksOK),
		"gateway_store":       okOrFail(v.gatewayOK),
		"bus":                 runtime.StatusOK,
		"projection":          string(projection),
		"mode":                string(mode),
		"sessions_active":     v.sessionsActive,
		"rounds_open":         v.roundsOpen,
		"outbox_pending":      v.outboxPending,
		"outbox_oldest_age_s": int64(v.outboxOldestAge / time.Second),
	}
	status := runtime.StatusOK
	if linkStore.CompactionPending() {
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
	if err := dispatcher.Err(); err != nil {
		details["bus"] = runtime.StatusFail
		status = runtime.StatusDegraded
	}
	if state := c.snapshotStateNow(); state != "" {
		details["snapshot"] = state
		status = runtime.StatusDegraded
	}
	if !v.linksOK || !v.gatewayOK {
		status = runtime.StatusFail
	}
	return runtime.Status{Status: status, Details: details}
}

func okOrFail(ok bool) string {
	if ok {
		return runtime.StatusOK
	}
	return runtime.StatusFail
}

func (c *Context) setSnapshotState(state string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.snapshotState = state
}

func (c *Context) snapshotStateNow() string {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.snapshotState
}

// restore closes what a stopped process left open (component §7.6): the turns
// past their deadline time out, then the sessions idle for
// MV_GATEWAY_SESSION_IDLE end idle. A failure is logged and does not stop the
// start: in live mode the sweeper repeats both a minute later.
func restore(ctx context.Context, now time.Time, svc services, log *slog.Logger) {
	if n, err := svc.turns.Sweep(ctx, now); err != nil {
		log.Error("restore: turns past their deadline", slog.String("error", err.Error()))
	} else if n > 0 {
		log.Info("restore: turns timed out", slog.Int("count", n))
	}
	if n, err := svc.sessions.Sweep(ctx, now); err != nil {
		log.Error("restore: idle sessions", slog.String("error", err.Error()))
	} else if n > 0 {
		log.Info("restore: idle sessions ended", slog.Int("count", n))
	}
}

// startSnapshots builds the writer of the snapshots of the gateway, checks the
// last snapshot it wrote and, in live mode, starts the goroutine that writes a
// snapshot after a session ended. A snapshot that does not check is reported in /health and
// in the log: the gateway comes up from gateway.db, which is its truth, but
// not in silence (US-011).
func (c *Context) startSnapshots(ctx context.Context, deps runtime.Deps, snaps *snapshots, objects objstore.Client,
	model *readmodel.Model, gatewayDB *sql.DB, sessions *session.Manager, log *slog.Logger) error {
	world := env.WorldID.StringFrom(c.src)
	writer, err := snapshot.New(snapshot.Config{Objects: objects, Bus: deps.Bus, WorldID: world, Clock: deps.Clock, Log: log})
	if err != nil {
		return err
	}
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.loadBudget)
	_, err = writer.Latest(readCtx)
	cancel()
	switch {
	case err == nil, errors.Is(err, snapshot.ErrNoSnapshot):
	case errors.Is(err, snapshot.ErrCorrupted):
		log.Error("snapshot of the gateway does not check", slog.String("world_id", world), slog.String("error", err.Error()))
		c.setSnapshotState(snapshotCorrupted)
	default:
		log.Error("snapshot of the gateway not read", slog.String("world_id", world), slog.String("error", err.Error()))
		c.setSnapshotState(snapshotUnreadable)
	}
	snaps.writer, snaps.worldID, snaps.model, snaps.db, snaps.sessions, snaps.log = writer, world, model, gatewayDB, sessions, log
	if !snaps.live {
		return nil
	}
	runCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	snaps.cancel = stop
	go snaps.run(runCtx)
	return nil
}

// repairStale is the repair of the consumer: the stale entities of the
// projection taken from the snapshot of State that snapshot.created announces
// (component §11.2). Without a store or a world there is nothing to read from.
// A repair that fails is a warning with its code: the entities stay stale and
// the event is not retried for it.
func repairStale(model *readmodel.Model, objects objstore.Client, world string, budget time.Duration,
	log *slog.Logger) func(context.Context, eventbus.Event) {
	if objects == nil || world == "" {
		return nil
	}
	return func(ctx context.Context, ev eventbus.Event) {
		repairCtx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		repaired, left, err := model.RepairFromStateSnapshot(repairCtx, objects, world, ev)
		var repairErr *readmodel.RepairError
		switch {
		case errors.As(err, &repairErr):
			log.Warn("projection: stale entities not repaired from the snapshot of state",
				slog.String("event_id", ev.ID), slog.String("reason", repairErr.Reason), slog.Int("stale", left))
		case err != nil:
			log.Warn("projection: stale entities not repaired from the snapshot of state",
				slog.String("event_id", ev.ID), slog.Int("stale", left))
		case repaired > 0:
			log.Info("projection: stale entities repaired from the snapshot of state",
				slog.String("event_id", ev.ID), slog.Int("repaired", repaired), slog.Int("stale", left))
		}
	}
}
