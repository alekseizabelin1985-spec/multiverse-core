package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// recorder is the one timeline of a run: what the contexts did and when the
// bus closed, in the order it happened.
type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// witness is a context that keeps what serve handed it and, when it stops, still
// reads the journal — the case ADR-023 p. 4 is about: a context stopped by the
// runtime may need the bus to finish, so the bus must outlive it.
type witness struct {
	name       string
	rec        *recorder
	startErr   error
	startPanic any
	stopPanic  any
	started    chan runtime.Deps
	deps       runtime.Deps
}

func (p *witness) Name() string        { return p.name }
func (p *witness) DependsOn() []string { return nil }
func (p *witness) Health() runtime.Status {
	return runtime.OK()
}

func (p *witness) Start(_ context.Context, deps runtime.Deps) error {
	if p.startErr != nil {
		return p.startErr
	}
	if p.startPanic != nil {
		panic(p.startPanic)
	}
	p.deps = deps
	p.rec.add("start " + p.name)
	if p.started != nil {
		p.started <- deps
	}
	return nil
}

func (p *witness) Stop(ctx context.Context) error {
	// A witness is stopped without having started only when the process stops a
	// context whose Start failed — a defect of run. It goes on the timeline, so
	// the test reports it, instead of a nil registry panicking the whole package
	// (review #1 of T-255, mutant O3).
	if p.deps.Contracts == nil {
		p.rec.add("stop " + p.name + " (never started)")
		return nil
	}
	topic := p.deps.Contracts.Topics()[0].Name
	if _, err := p.deps.Journal.End(ctx, topic); err != nil {
		p.rec.add("stop " + p.name + ": journal: " + err.Error())
		return nil
	}
	p.rec.add("stop " + p.name)
	if p.stopPanic != nil {
		panic(p.stopPanic)
	}
	return nil
}

// recordingBus is the real transport of the process with its Close put on the
// timeline.
type recordingBus struct {
	transport
	rec *recorder
}

func (b recordingBus) Close() error {
	b.rec.add("bus closed")
	return b.transport.Close()
}

func recordingOpen(rec *recorder) openBusFunc {
	return func(bus string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error) {
		t, err := openBus(bus, reg, timers, log)
		if err != nil {
			return nil, err
		}
		return recordingBus{transport: t, rec: rec}, nil
	}
}

// memoryOptions is what parseServe resolves for "--contexts=all --bus=memory".
func memoryOptions() serveOptions {
	return serveOptions{
		contexts: []string{runtime.All},
		mode:     runtime.ModeLive,
		bus:      busMemory,
		modeFrom: "--mode",
		busFrom:  "--bus",
		idSource: "uuid",
	}
}

// onLoopback makes the process listen on a port of its own choosing and puts
// validation on read back on the shipped default.
//
// It also gives the gateway context a data directory of the test: since T-303
// the gateway of --contexts=all is real and opens its SQLite files at start,
// and the default /data of the manifest is no place for a test.
func onLoopback(t *testing.T) {
	t.Helper()
	t.Setenv(env.CoreAddr.Name(), "127.0.0.1:0")
	t.Setenv(env.GatewayDataDir.Name(), sqlitedir.Temp(t))
	clearVar(t, env.BusValidateOnRead.Name())
}

func newProcess(contexts []runtime.Context, open openBusFunc) process {
	return process{
		opts:     memoryOptions(),
		contexts: contexts,
		openBus:  open,
		stdout:   io.Discard,
		log:      slog.New(slog.DiscardHandler),
	}
}

// runUntilStarted runs the process, waits for the probe to report its start,
// hands the Deps it got to inspect while everything is still up, and then
// stops the process the way a signal does.
func runUntilStarted(t *testing.T, p process, started <-chan runtime.Deps, inspect func(runtime.Deps)) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.run(ctx, cancel) }()

	wait, stopWaiting := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopWaiting()
	select {
	case deps := <-started:
		if inspect != nil {
			inspect(deps)
		}
	case err := <-done:
		t.Fatalf("the process ended before the probe started: %v", err)
	case <-wait.Done():
		t.Fatal("the probe did not start within 30s")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-wait.Done():
		t.Fatal("the process did not stop within 30s")
	}
}

// The DoD of T-410: a context started by serve gets a bus, a journal and the
// registry — not nil, the transport --bus named, one object in both fields so
// that the journal reads the log the bus writes, and a bus that knows every
// topic of the registry.
func TestServeHandsTheBusJournalAndRegistryToContexts(t *testing.T) {
	onLoopback(t)
	started := make(chan runtime.Deps, 1)
	p := &witness{name: "probe", rec: &recorder{}, started: started}

	runUntilStarted(t, newProcess([]runtime.Context{p}, openBus), started, func(deps runtime.Deps) {
		if deps.Bus == nil || deps.Journal == nil || deps.Contracts == nil {
			t.Fatalf("Deps.Bus/Journal/Contracts = %v/%v/%v, want all three set", deps.Bus, deps.Journal, deps.Contracts)
		}
		if _, ok := deps.Bus.(*membus.Bus); !ok {
			t.Errorf("Deps.Bus is %T, want the in-process bus --bus=memory asked for", deps.Bus)
		}
		if any(deps.Bus) != any(deps.Journal) {
			t.Error("Deps.Bus and Deps.Journal are two transports: the journal would not read what the bus publishes")
		}
		if deps.Contracts != contracts.Default() {
			t.Error("Deps.Contracts is not contracts.Default(): the contexts would check against another registry than the bus routes by")
		}
		for _, topic := range deps.Contracts.Topics() {
			if _, err := deps.Journal.End(context.Background(), topic.Name); err != nil {
				t.Errorf("Journal.End(%s): %v", topic.Name, err)
			}
		}
		// The broker never creates a topic on first use; the memory bus of the
		// process must not either, or the two modes disagree on a typo.
		if _, err := deps.Journal.End(context.Background(), "no_such_topic"); !errors.Is(err, membus.ErrNoTopic) {
			t.Errorf("Journal.End(no_such_topic) = %v, want membus.ErrNoTopic: the bus creates topics on first use", err)
		}
	})
}

// ADR-023 p. 4: the bus closes after every context has stopped, never under
// one. Each probe reads the journal in its Stop, so a bus closed first shows
// up twice — out of order on the timeline and as an error of the journal.
func TestServeClosesTheBusAfterEveryContextStopped(t *testing.T) {
	onLoopback(t)
	rec := &recorder{}
	started := make(chan runtime.Deps, 1)
	first := &witness{name: "first", rec: rec}
	second := &witness{name: "second", rec: rec, started: started}

	runUntilStarted(t, newProcess([]runtime.Context{first, second}, recordingOpen(rec)), started, nil)

	want := []string{"start first", "start second", "stop second", "stop first", "bus closed"}
	if got := rec.list(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lifetime = %q, want %q", got, want)
	}
}

// T-430: a panic in the Stop of a context does not cut the shutdown short. The
// contexts before it in the start order still stop, reading the journal on a
// bus that is still open, the bus closes last (ADR-023 p. 4), and the panic
// reaches the log of the process with its stack. Before StopAll recovered it,
// the panic left run, the deferred Close ran during the unwinding and first was
// never stopped — had the test process survived the panic at all.
func TestServeClosesTheBusLastWhenAContextPanicsInStop(t *testing.T) {
	onLoopback(t)
	rec := &recorder{}
	started := make(chan runtime.Deps, 1)
	first := &witness{name: "first", rec: rec}
	second := &witness{name: "second", rec: rec, started: started, stopPanic: "boom"}
	p := newProcess([]runtime.Context{first, second}, recordingOpen(rec))
	// Written by run only; read after runUntilStarted has received its end.
	var logged strings.Builder
	p.log = slog.New(slog.NewTextHandler(&logged, nil))

	runUntilStarted(t, p, started, nil)

	want := []string{"start first", "start second", "stop second", "stop first", "bus closed"}
	if got := rec.list(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lifetime = %q, want %q", got, want)
	}
	if !strings.Contains(logged.String(), "stop context second: panic: boom") || !strings.Contains(logged.String(), "goroutine") {
		t.Errorf("the log of the process has no error with the panic of second and its stack: %q", logged.String())
	}
}

// The failed paths out of run close the bus too, and still only after the
// contexts that did start have been stopped.
func TestServeClosesTheBusLastWhenTheStartFails(t *testing.T) {
	tests := map[string]struct {
		failStart  bool
		panicStart bool
		stopPanic  bool
		occupy     bool
		want       string
	}{
		"a context fails to start": {failStart: true, want: "second"},
		// T-430: a panic in the Stop of the rollback is one more cause on the
		// same line, not an unwinding past the bus.
		"a context panics in Stop on the rollback": {
			failStart: true, stopPanic: true,
			want: "start context second: boom; stop context first: panic: stuck",
		},
		// T-415: a panic in Start takes the same way out as an error. Before
		// StartAll recovered it, the deferred Close ran during the unwinding
		// with first still up, and the timeline read "bus closed" before
		// "stop first" — had the test process survived the panic at all.
		"a context panics in Start": {panicStart: true, want: "start context second: panic: boom"},
		"the port is taken":         {occupy: true, want: "listen"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			if tc.occupy {
				lis, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("listen: %v", err)
				}
				defer func() { _ = lis.Close() }()
				t.Setenv(env.CoreAddr.Name(), lis.Addr().String())
			}
			rec := &recorder{}
			first := &witness{name: "first", rec: rec}
			if tc.stopPanic {
				first.stopPanic = "stuck"
			}
			contexts := []runtime.Context{first}
			if tc.failStart {
				contexts = append(contexts, &witness{name: "second", rec: rec, startErr: errors.New("boom")})
			}
			if tc.panicStart {
				contexts = append(contexts, &witness{name: "second", rec: rec, startPanic: "boom"})
			}

			// A start that fails returns at once. The deadline only bounds a run
			// that came up anyway, so that such a regression is a red test within
			// seconds instead of a package hung until the timeout of go test
			// (review #1 of T-255, mutant O3).
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := newProcess(contexts, recordingOpen(rec)).run(ctx, cancel)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("run error = %v, want one naming %q", err, tc.want)
			}
			want := []string{"start first", "stop first", "bus closed"}
			if got := rec.list(); strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("lifetime = %q, want %q", got, want)
			}
		})
	}
}

// --bus=kafka builds the adapter without touching a broker: kafka-go dials on
// the first publish or read, so a process whose contexts are stubs starts
// even when nothing listens on MV_KAFKA_BROKERS.
func TestOpenBusBuildsTheTransportTheValueNames(t *testing.T) {
	clearVar(t, env.BusValidateOnRead.Name())
	reg := contracts.Default()

	kafka, err := openBus(busKafka, reg, clock.RealTimers{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("openBus(kafka): %v", err)
	}
	defer func() { _ = kafka.Close() }()
	if _, ok := kafka.(*eventbus.Kafka); !ok {
		t.Errorf("openBus(kafka) = %T, want *eventbus.Kafka", kafka)
	}

	if _, err := openBus("nats", reg, clock.RealTimers{}, slog.New(slog.DiscardHandler)); err == nil {
		t.Error("openBus(nats) = nil error: a value outside the enum got a transport")
	}
}

// The last step of SEC-16 on the kafka side: TestKafkaConfigCarriesTheManifest
// checks the configuration up to the call, and this checks that the call hands
// it on — the adapter holds the flag in an unexported field, so it is read by
// reflection. Without it a line that changes cfg between kafkaConfig and
// eventbus.NewKafka survives every test that runs without a broker (mutant
// M7c of review #2 of T-410, N-3; T-415). The broker-backed check is T-394.
func TestOpenBusHandsTheKafkaAdapterTheValidationFlag(t *testing.T) {
	for _, tc := range []struct {
		value string
		skip  bool
	}{{"", false}, {"false", true}, {"true", false}} {
		t.Run("value "+strconv.Quote(tc.value), func(t *testing.T) {
			if tc.value == "" {
				clearVar(t, env.BusValidateOnRead.Name())
			} else {
				t.Setenv(env.BusValidateOnRead.Name(), tc.value)
			}
			bus, err := openBus(busKafka, contracts.Default(), clock.RealTimers{}, slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatalf("openBus(kafka): %v", err)
			}
			defer func() { _ = bus.Close() }()
			adapter, ok := bus.(*eventbus.Kafka)
			if !ok {
				t.Fatalf("openBus(kafka) = %T, want *eventbus.Kafka", bus)
			}
			field := reflect.ValueOf(adapter).Elem().FieldByName("skipValidateOnRead")
			if !field.IsValid() {
				t.Fatal("eventbus.Kafka has no field skipValidateOnRead any more: follow the rename here")
			}
			if field.Kind() != reflect.Bool {
				t.Fatalf("eventbus.Kafka.skipValidateOnRead is a %s now, not a bool: follow the change here", field.Kind())
			}
			if got := field.Bool(); got != tc.skip {
				t.Errorf("the adapter skips validation on read = %v, want %v for %s=%q",
					got, tc.skip, env.BusValidateOnRead.Name(), tc.value)
			}
		})
	}
}

// SEC-16: MV_BUS_VALIDATE_ON_READ reaches the transport. An envelope the
// schema rejects is parked by default and handed over once the check is off.
func TestOpenBusHonoursValidateOnRead(t *testing.T) {
	tests := map[string]struct {
		value   string
		handled int
		parked  int
	}{
		"shipped default": {value: "", handled: 0, parked: 1},
		"switched off":    {value: "false", handled: 1, parked: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.value == "" {
				clearVar(t, env.BusValidateOnRead.Name())
			} else {
				t.Setenv(env.BusValidateOnRead.Name(), tc.value)
			}
			reg := contracts.Default()
			bus, err := openBus(busMemory, reg, clock.RealTimers{}, slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatalf("openBus(memory): %v", err)
			}
			defer func() { _ = bus.Close() }()
			mem := bus.(*membus.Bus)

			topic := reg.Topics()[0].Name
			if err := mem.Append(topic, []byte(`{"type":"no.such.type"}`)); err != nil {
				t.Fatalf("append: %v", err)
			}
			handled := 0
			if _, err := bus.ReadRange(context.Background(), topic, 0, 1, func(context.Context, eventbus.Event) error {
				handled++
				return nil
			}); err != nil {
				t.Fatalf("read: %v", err)
			}
			parked, err := mem.DeadLetters()
			if err != nil {
				t.Fatalf("dead letters: %v", err)
			}
			if handled != tc.handled || len(parked) != tc.parked {
				t.Fatalf("handled %d, parked %d; want %d and %d", handled, len(parked), tc.handled, tc.parked)
			}
		})
	}
}

func TestOpenBusRefusesAMalformedValidateOnRead(t *testing.T) {
	t.Setenv(env.BusValidateOnRead.Name(), "maybe")
	for _, bus := range []string{busKafka, busMemory} {
		if b, err := openBus(bus, contracts.Default(), clock.RealTimers{}, slog.New(slog.DiscardHandler)); err == nil {
			_ = b.Close()
			t.Errorf("openBus(%s) accepted %s=maybe", bus, env.BusValidateOnRead.Name())
		}
	}
}

// SEC-16 on the kafka side, without a broker: the adapter keeps its settings in
// unexported fields, so the configuration the process builds is checked
// instead — from the variable to the field (review #1 of T-410, Mi-2).
func TestKafkaConfigCarriesTheManifest(t *testing.T) {
	tests := map[string]struct {
		value string
		skip  bool
	}{
		"shipped default": {value: "", skip: false},
		"switched off":    {value: "false", skip: true},
		"switched on":     {value: "true", skip: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.value == "" {
				clearVar(t, env.BusValidateOnRead.Name())
			} else {
				t.Setenv(env.BusValidateOnRead.Name(), tc.value)
			}
			t.Setenv(env.KafkaBrokers.Name(), "broker-a:9092,broker-b:9092")
			reg := contracts.Default()
			timers := clock.RealTimers{}
			log := slog.New(slog.DiscardHandler)

			cfg, err := kafkaConfig(reg, timers, log)
			if err != nil {
				t.Fatalf("kafkaConfig: %v", err)
			}
			if cfg.SkipValidateOnRead != tc.skip {
				t.Errorf("SkipValidateOnRead = %v, want %v for %s=%q", cfg.SkipValidateOnRead, tc.skip, env.BusValidateOnRead.Name(), tc.value)
			}
			if got := strings.Join(cfg.Brokers, ","); got != "broker-a:9092,broker-b:9092" {
				t.Errorf("Brokers = %q, want the list of %s", got, env.KafkaBrokers.Name())
			}
			if cfg.Registry != reg {
				t.Error("Registry is not the registry of the contexts")
			}
			if cfg.Timers != timers || cfg.Log != log {
				t.Error("Timers or Log is not the one of the process")
			}
		})
	}
}

// Mi-1 of review #1: release comes right after the wait and before anything
// stops, so a second signal during the shutdown is not swallowed.
func TestServeReleasesTheSignalsBeforeStopping(t *testing.T) {
	onLoopback(t)
	rec := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := newProcess([]runtime.Context{&witness{name: "first", rec: rec}}, recordingOpen(rec))
	if err := p.run(ctx, func() { rec.add("release") }); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"start first", "release", "stop first", "bus closed"}
	if got := rec.list(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lifetime = %q, want %q", got, want)
	}
}

// The other half of Mi-1: serve hands run the real unregistering function of
// signal.NotifyContext, not a no-op. Calling it cancels the context, which a
// test can see without sending a signal.
func TestWithSignalsHandsOverTheRealRelease(t *testing.T) {
	boom := errors.New("boom")
	err := withSignals(context.Background(), func(ctx context.Context, release func()) error {
		if ctx.Err() != nil {
			t.Fatal("the signal context is done before anything happened")
		}
		release()
		if ctx.Err() == nil {
			t.Error("release did not unregister the signals: a second signal during the shutdown would be swallowed")
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("withSignals = %v, want the error of fn", err)
	}
}
