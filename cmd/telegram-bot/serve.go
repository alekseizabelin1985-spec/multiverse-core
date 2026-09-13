package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/access"
	"multiverse-core.io/cmd/telegram-bot/internal/config"
	"multiverse-core.io/cmd/telegram-bot/internal/deliver"
	"multiverse-core.io/cmd/telegram-bot/internal/flow"
	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/logging"
)

// limits are the timeouts of the clients of the bot. Each of the three places
// that talk to the outside waits in a different way, so each has a client of
// its own:
//
//   - the flow runs in the one handler of all updates: a gateway that does not
//     answer must hold the commands of the other players for seconds, not for
//     the 35 s × 4 attempts of the default client (review #1 of T-311, risk 6);
//     the same holds for its answers to Telegram, which have a client of their
//     own (acceptance of T-312, Mi-4 of review #1);
//   - the access gate answers strangers in the same handler: its sender gives
//     up after sender.BestEffortHTTPTimeout (review #2 of T-310, Mi-4);
//   - the delivery loop runs in its own goroutine and holds a long-poll of 25 s:
//     its gateway client must outlast it.
type limits struct {
	// TelegramTimeout bounds one sendMessage of the loop.
	TelegramTimeout time.Duration
	// FlowTelegramTimeout bounds one sendMessage of an answer of the flow.
	FlowTelegramTimeout time.Duration
	// GateTimeout bounds one sendMessage of a refusal.
	GateTimeout time.Duration
	// FlowGatewayTimeout and FlowGatewayBackoff are the client of the flow.
	FlowGatewayTimeout time.Duration
	FlowGatewayBackoff client.Backoff
	// DeliverGatewayTimeout is the client of the delivery loop.
	DeliverGatewayTimeout time.Duration
}

// productionLimits: a request of the gateway other than a long-poll is cut at
// api.RequestTimeout (5 s), so the flow waits a little longer than that, and
// tries once more after a short pause — at most about 12 s for one command. An
// answer to Telegram waits up to 10 s a try, two tries of sender.ReplyPolicy
// with a pause of 1 s — about 21 s, not the 61 s of the client of the loop.
var productionLimits = limits{
	TelegramTimeout:       sender.DefaultHTTPTimeout,
	FlowTelegramTimeout:   10 * time.Second,
	GateTimeout:           sender.BestEffortHTTPTimeout,
	FlowGatewayTimeout:    6 * time.Second,
	FlowGatewayBackoff:    client.Backoff{Retries: 1, Initial: 200 * time.Millisecond, Max: 200 * time.Millisecond},
	DeliverGatewayTimeout: client.DefaultHTTPTimeout,
}

// healthReadHeaderTimeout bounds the header of a request to /health.
const healthReadHeaderTimeout = 5 * time.Second

// stopTimeout bounds the shutdown of /health once the bot has stopped.
const stopTimeout = 5 * time.Second

// bot is the assembled process. The options each component was built with are
// kept, so that a test sees what the wiring handed out.
type bot struct {
	log      *slog.Logger
	redactor *privacy.Redactor
	source   updates.Source
	gate     *access.Gate
	flow     *flow.Flow
	loop     *deliver.Loop

	flowOpts flow.Options
	accOpts  access.Options
	loopOpts deliver.Options
	srcOpts  updates.TelegramOptions

	// ready is closed by the update source once Telegram took the token; the
	// delivery loop waits for it (acceptance of T-312, N-3 of review #1).
	ready     chan struct{}
	readyOnce sync.Once

	healthAddr string
}

// serve runs the bot until ctx ends or the update source stops.
func serve(ctx context.Context, e environment) int {
	base, err := newLogger(e)
	if err != nil {
		_, _ = fmt.Fprintln(e.stderr, "telegram-bot:", err)
		return 1
	}
	cfg, err := config.Load(e.source)
	if err != nil {
		// The errors of config withhold every value; Redact is the second line.
		_, _ = fmt.Fprintln(e.stderr, "telegram-bot:", privacy.Redact(err.Error()))
		return 1
	}
	b, err := build(cfg, e, base)
	if err != nil {
		_, _ = fmt.Fprintln(e.stderr, "telegram-bot:", privacy.Redact(err.Error()))
		return 1
	}
	return b.run(ctx, e)
}

// newLogger is the JSON logger of shared/logging with the level and format of
// the manifest. The privacy handler is put over it in build, once the secrets
// are known.
func newLogger(e environment) (*slog.Logger, error) {
	var level slog.Level
	raw := env.LogLevel.StringFrom(e.source)
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return nil, fmt.Errorf("MV_LOG_LEVEL=%q: expected debug, info, warn or error", raw)
	}
	return logging.New(logging.Options{
		Service: ClientID,
		Version: logging.Version,
		Level:   level,
		Format:  env.LogFormat.StringFrom(e.source),
		Output:  e.stdout,
	}), nil
}

// build assembles the components (component §10.1). Every logger is one: the
// JSON handler of the process under privacy.Handler with the secrets of the
// configuration (ADR-018 addendum p. 3, review #1 of T-311, Mi-6).
func build(cfg config.Config, e environment, base *slog.Logger) (*bot, error) {
	redactor := privacy.NewRedactor(cfg.Secrets()...)
	log := slog.New(privacy.NewHandler(base.Handler(), redactor))
	errorLog := privacy.ErrorLog(log, redactor)
	l := e.limits

	deliverSender, err := sender.NewTelegram(sender.TelegramOptions{
		Token: cfg.Token, Redactor: redactor, Timers: e.timers, Policy: sender.DefaultPolicy,
		ErrorLog: errorLog, ServerURL: e.telegramURL, HTTPClient: &http.Client{Timeout: l.TelegramTimeout},
	})
	if err != nil {
		return nil, err
	}
	flowSender, err := sender.NewTelegram(sender.TelegramOptions{
		Token: cfg.Token, Redactor: redactor, Timers: e.timers, Policy: sender.ReplyPolicy,
		ErrorLog: errorLog, ServerURL: e.telegramURL, HTTPClient: &http.Client{Timeout: l.FlowTelegramTimeout},
	})
	if err != nil {
		return nil, err
	}
	gateSender, err := sender.NewBestEffortTelegram(sender.TelegramOptions{
		Token: cfg.Token, Redactor: redactor, Timers: e.timers,
		ErrorLog: errorLog, ServerURL: e.telegramURL, HTTPClient: &http.Client{Timeout: l.GateTimeout},
	})
	if err != nil {
		return nil, err
	}

	flowGateway := client.New(cfg.GatewayURL, ClientID)
	flowGateway.HTTP = &http.Client{Timeout: l.FlowGatewayTimeout}
	flowGateway.Backoff = l.FlowGatewayBackoff
	flowGateway.Timers = e.timers

	deliverGateway := client.New(cfg.GatewayURL, ClientID)
	deliverGateway.HTTP = &http.Client{Timeout: l.DeliverGatewayTimeout}
	deliverGateway.Timers = e.timers

	b := &bot{log: log, redactor: redactor, healthAddr: cfg.HealthAddr, ready: make(chan struct{})}
	b.accOpts = access.Options{
		AllowedUserIDs:    cfg.AllowedUserIDs,
		CommandsPerMinute: cfg.CommandsPerMinute,
		Clock:             e.clock,
		Sender:            gateSender,
		Log:               log,
	}
	if b.gate, err = access.New(b.accOpts); err != nil {
		return nil, err
	}
	b.flowOpts = flow.Options{
		Gateway:       flowGateway,
		Sender:        flowSender,
		Clock:         e.clock,
		Timers:        e.timers,
		ActionKeySalt: cfg.ActionKeySalt,
		Log:           log,
	}
	if b.flow, err = flow.New(b.flowOpts); err != nil {
		return nil, err
	}
	b.loopOpts = deliver.Options{Gateway: deliverGateway, Sender: deliverSender, Timers: e.timers, Log: log}
	if b.loop, err = deliver.New(b.loopOpts); err != nil {
		return nil, err
	}
	b.srcOpts = updates.TelegramOptions{
		Token: cfg.Token, PollTimeout: cfg.PollTimeout, Log: log, Redactor: redactor, ServerURL: e.telegramURL,
		OnReady: func() { b.readyOnce.Do(func() { close(b.ready) }) },
	}
	b.source = updates.NewTelegram(b.srcOpts)
	log.Info("telegram-bot configured", slog.Any("config", cfg))
	return b, nil
}

// run serves /health, runs the delivery loop in a goroutine of its own and the
// update source in this one, and returns the exit code of the stop. The
// delivery loop starts once the source reports that Telegram took the token:
// with a revoked token it takes no delivery into a lease. It is stopped with
// the source and acknowledges what it sent before run returns.
func (b *bot) run(ctx context.Context, e environment) int {
	lis, err := net.Listen("tcp", b.healthAddr)
	if err != nil {
		b.log.Error("health listener not opened", slog.String("error", err.Error()))
		_, _ = fmt.Fprintf(e.stderr, "telegram-bot: /health on %s: %v\n", b.healthAddr, err)
		return 1
	}
	srv := &http.Server{Handler: healthHandler(b.gate.Counters), ReadHeaderTimeout: healthReadHeaderTimeout}
	served := make(chan struct{})
	go func() {
		defer close(served)
		if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			b.log.Error("health server stopped", slog.String("error", err.Error()))
		}
	}()
	if e.listening != nil {
		e.listening(lis.Addr().String())
	}

	runCtx, cancel := context.WithCancel(ctx)
	looped := make(chan struct{})
	go func() {
		defer close(looped)
		select {
		case <-b.ready:
			b.loop.Run(runCtx)
		case <-runCtx.Done():
		}
	}()
	b.log.Info("telegram-bot started", slog.String("health_addr", lis.Addr().String()))
	stopErr := b.source.Start(runCtx, b.gate.Wrap(b.flow.Handle))
	cancel()
	<-looped

	stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), stopTimeout)
	defer stopCancel()
	_ = srv.Shutdown(stopCtx)
	<-served

	code := updates.ExitCode(stopErr)
	if stopErr == nil {
		b.log.Info("telegram-bot stopped")
		return code
	}
	text := b.redactor.Redact(stopErr.Error())
	b.log.Error("telegram-bot stopped", slog.String("error", text), slog.Int("exit_code", code))
	_, _ = fmt.Fprintln(e.stderr, "telegram-bot:", text)
	return code
}
