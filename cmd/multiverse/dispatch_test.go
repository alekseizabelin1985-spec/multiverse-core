package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
)

// startBriefly is the start of the process with the wait taken out: the real
// contexts, the real bus and the real start line, under a context that is done
// before the run begins, so the process stops as soon as it has said what it
// runs. The options it was handed are kept for the caller.
func startBriefly(got *serveOptions, calls *int) startFunc {
	return func(opts serveOptions, stdout, _ io.Writer) error {
		*got = opts
		*calls++
		contexts, err := runtime.New(opts.contexts)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return process{
			opts:     opts,
			contexts: contexts,
			openBus:  openBus,
			stdout:   stdout,
			log:      slog.New(slog.DiscardHandler),
		}.run(ctx, func() {})
	}
}

// T-414: "multiverse serve <flags>" and "multiverse <flags>" are one command.
// Each case goes through the real dispatch and the real parsing twice, once per
// spelling, and the two runs must agree on everything the flags decide — the
// source of the mode and of the bus included (T-408), both in the options and in
// the start line the process prints.
func TestServeSubcommandAndTheBareFormAreOneCommand(t *testing.T) {
	tests := map[string]struct {
		env   map[string]string
		flags []string
		want  string
	}{
		"shipped manifest, bus from the flag": {
			flags: []string{"--contexts=all", "--bus=memory"},
			want:  "mode: live (MV_MODE), bus: memory (--bus)",
		},
		"the manifest decides": {
			env:   map[string]string{env.Mode.Name(): "replay", env.Bus.Name(): "memory"},
			flags: []string{"--contexts=all"},
			want:  "mode: replay (MV_MODE), bus: memory (MV_BUS)",
		},
		// The flag package takes one dash as well as two; a bare form spelled so
		// is still serve and not an unknown subcommand.
		"single-dash flags": {
			flags: []string{"-contexts=all", "-bus=memory"},
			want:  "mode: live (MV_MODE), bus: memory (--bus)",
		},
		"the flag overrides the manifest": {
			env:   map[string]string{env.Mode.Name(): "replay", env.Bus.Name(): "memory"},
			flags: []string{"--contexts=all", "--mode=live", "--bus=kafka"},
			want:  "mode: live (--mode), bus: kafka (--bus)",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			clearModeAndBus(t)
			clearVar(t, env.SwarmFake.Name())
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			spellings := map[string][]string{
				"serve": append([]string{"serve"}, tc.flags...),
				"bare":  tc.flags,
			}
			opts := map[string]serveOptions{}
			for spelling, args := range spellings {
				var got serveOptions
				var calls int
				var stdout, stderr bytes.Buffer
				if code := dispatch(args, &stdout, &stderr, startBriefly(&got, &calls)); code != 0 || calls != 1 {
					t.Fatalf("%s %v: exit %d, started %d times, want 0 and once (stderr: %s)",
						spelling, args, code, calls, stderr.String())
				}
				if !strings.Contains(stdout.String(), tc.want) {
					t.Errorf("%s %v printed %q, want it to say %q", spelling, args, stdout.String(), tc.want)
				}
				opts[spelling] = got
			}
			if !reflect.DeepEqual(opts["serve"], opts["bare"]) {
				t.Fatalf("the two spellings resolved differently:\nserve: %+v\nbare:  %+v", opts["serve"], opts["bare"])
			}
		})
	}
}

// A refusal of the flags is the same refusal under either spelling, and it
// still names the variable the operator set rather than a flag he did not pass.
func TestServeSubcommandAndTheBareFormRefuseAlike(t *testing.T) {
	clearModeAndBus(t)
	t.Setenv(env.Mode.Name(), "dry-run")
	flags := []string{"--contexts=all"}

	stderrs := map[string]string{}
	for spelling, args := range map[string][]string{"serve": append([]string{"serve"}, flags...), "bare": flags} {
		var got serveOptions
		var calls int
		var stdout, stderr bytes.Buffer
		if code := dispatch(args, &stdout, &stderr, startBriefly(&got, &calls)); code != 2 || calls != 0 {
			t.Fatalf("%s %v: exit %d, started %d times, want 2 and never", spelling, args, code, calls)
		}
		stderrs[spelling] = stderr.String()
	}
	if stderrs["serve"] != stderrs["bare"] {
		t.Fatalf("the two spellings refuse differently:\nserve: %q\nbare:  %q", stderrs["serve"], stderrs["bare"])
	}
	if !strings.Contains(stderrs["serve"], env.Mode.Name()) {
		t.Fatalf("stderr = %q, want it to name %s", stderrs["serve"], env.Mode.Name())
	}
}

// A first argument that is neither a flag nor a subcommand is refused out loud,
// with the subcommands the binary does know, and nothing is started.
func TestUnknownSubcommandIsRefusedWithTheKnownOnes(t *testing.T) {
	for _, args := range [][]string{
		{"helth", "--url=http://127.0.0.1:8090/health"},
		{"serves", "--contexts=all", "--bus=memory"},
		{"start"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var got serveOptions
			var calls int
			var stdout, stderr bytes.Buffer
			if code := dispatch(args, &stdout, &stderr, startBriefly(&got, &calls)); code != 2 {
				t.Fatalf("dispatch(%v) = %d, want 2", args, code)
			}
			if calls != 0 {
				t.Fatalf("dispatch(%v) started the process", args)
			}
			msg := stderr.String()
			for _, want := range []string{"unknown subcommand", `"` + args[0] + `"`, "serve", "health", "db", "version"} {
				if !strings.Contains(msg, want) {
					t.Errorf("stderr = %q, want it to contain %q", msg, want)
				}
			}
		})
	}
}

// The list the refusal prints and the words dispatch accepts are one set: every
// listed word reaches its own command (-h makes each of them end at once), and
// the list is the four subcommands the binary has.
func TestEveryListedSubcommandIsDispatched(t *testing.T) {
	if got := strings.Join(subcommands, ","); got != "serve,health,db,version" {
		t.Fatalf("subcommands = %v, want [serve health db version]", subcommands)
	}
	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			var got serveOptions
			var calls int
			var stdout, stderr bytes.Buffer
			code := dispatch([]string{name, "-h"}, &stdout, &stderr, startBriefly(&got, &calls))
			if strings.Contains(stderr.String(), "unknown subcommand") {
				t.Fatalf("%s is listed but refused: %s", name, stderr.String())
			}
			if code != 0 || calls != 0 {
				t.Fatalf("%s -h: exit %d, started %d times, want 0 and never (stderr: %s)",
					name, code, calls, stderr.String())
			}
		})
	}
}

// Flag parsing stops at the first word that is not a flag, so a subcommand
// written after the flags reaches serve as a stray argument. The refusal says
// where the subcommand belongs; a stray word that is no subcommand gets no such
// hint (review #1 of T-414, N-1).
func TestSubcommandAfterTheFlagsIsToldToGoFirst(t *testing.T) {
	tests := map[string]struct {
		args []string
		hint string
	}{
		"serve after the flags":        {[]string{"--contexts=all", "--bus=memory", "serve"}, "multiverse serve"},
		"health after the flags":       {[]string{"--contexts=all", "health"}, "multiverse health"},
		"a word that is no subcommand": {[]string{"--contexts=all", "extra"}, ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			clearModeAndBus(t)
			var got serveOptions
			var calls int
			var stdout, stderr bytes.Buffer
			if code := dispatch(tc.args, &stdout, &stderr, startBriefly(&got, &calls)); code != 2 || calls != 0 {
				t.Fatalf("dispatch(%v): exit %d, started %d times, want 2 and never", tc.args, code, calls)
			}
			msg := stderr.String()
			if !strings.Contains(msg, "unexpected argument") {
				t.Fatalf("stderr = %q, want the stray argument named", msg)
			}
			hinted := strings.Contains(msg, "the subcommand goes first")
			if tc.hint == "" {
				if hinted {
					t.Fatalf("stderr = %q: %v is no subcommand and must not be told to go first", msg, tc.args[len(tc.args)-1])
				}
				return
			}
			if !hinted || !strings.Contains(msg, tc.hint) {
				t.Fatalf("stderr = %q, want it to say the subcommand goes first and show %q", msg, tc.hint)
			}
		})
	}
}

// "multiverse -h" is where an operator looks first, and it has to name every
// subcommand, not only the flags of serve — under both spellings of serve
// (review #1 of T-414, N-2).
func TestHelpNamesEverySubcommand(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"serve", "-h"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			clearModeAndBus(t)
			var got serveOptions
			var calls int
			var stdout, stderr bytes.Buffer
			if code := dispatch(args, &stdout, &stderr, startBriefly(&got, &calls)); code != 0 || calls != 0 {
				t.Fatalf("dispatch(%v): exit %d, started %d times, want 0 and never", args, code, calls)
			}
			help := stderr.String()
			if !strings.Contains(help, "usage: multiverse [serve] [flags]") {
				t.Errorf("help = %q, want the usage line", help)
			}
			for _, name := range subcommands {
				if !strings.Contains(help, name) {
					t.Errorf("help does not name the subcommand %q:\n%s", name, help)
				}
			}
			if !strings.Contains(help, "| health | db | version") {
				t.Errorf("help = %q, want the other subcommands listed after serve", help)
			}
			if !strings.Contains(help, "-contexts") {
				t.Errorf("help = %q, want the flags of serve as well", help)
			}
		})
	}
}
