// Command telegram-bot is the Telegram client of the platform (ADR-018,
// component gateway-and-bot §10): it polls Telegram for the messages of the
// players, answers them through the gateway, and polls the gateway for the
// deliveries of the players.
//
// Without arguments it runs the bot; that is how compose starts it
// (entrypoint /telegram-bot). "telegram-bot health [--url <url>]" is the
// healthcheck of the image, which has neither a shell nor curl.
//
// Exit codes: 0 — stopped by a signal; 1 — the configuration is invalid, the
// token was rejected (at start or while polling) or the bot could not start;
// 2 — a wrong command line; 3 — another instance polls the same token, or a
// webhook is set for it (updates.ExitConflict).
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
)

// ClientID is the X-Client-Id of the bot at the gateway: the first entry of
// the default MV_GATEWAY_CLIENT_IDS (component §11.1, SEC-12).
const ClientID = "telegram-bot"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], environment{
		source: env.OS(),
		stdout: os.Stdout,
		stderr: os.Stderr,
		clock:  clock.Real{},
		timers: clock.RealTimers{},
		limits: productionLimits,
	})
	stop()
	os.Exit(code)
}

// environment is everything run takes from outside the process: the
// variables, the streams, the time, and — for tests only — the address of the
// Bot API and a hook that learns the address /health was bound to.
type environment struct {
	source env.Source
	stdout io.Writer
	stderr io.Writer
	clock  clock.Clock
	timers clock.Timers
	limits limits
	// telegramURL replaces https://api.telegram.org when not empty.
	telegramURL string
	// listening, when set, is called with the bound address of /health once
	// the listener is open.
	listening func(addr string)
}

// run dispatches the command line and returns the exit code.
func run(ctx context.Context, args []string, e environment) int {
	if len(args) == 0 {
		return serve(ctx, e)
	}
	if args[0] == "health" {
		return runHealth(args[1:], e)
	}
	_, _ = fmt.Fprintf(e.stderr, "telegram-bot: unknown command %q: expected no arguments to run the bot, or health [--url <url>]\n", args[0])
	return 2
}
