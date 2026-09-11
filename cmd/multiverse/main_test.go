package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
)

// clearVar removes a manifest variable for the length of a test. The default of
// a declaration applies only while the variable is unset, and a developer who
// exports MV_MODE in their own shell must not be able to change what a test
// proves. t.Setenv first, so that the value the machine had is restored.
func clearVar(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	_ = os.Unsetenv(name)
}

// clearModeAndBus puts the process back on the shipped defaults of the manifest.
func clearModeAndBus(t *testing.T) {
	t.Helper()
	clearVar(t, env.Mode.Name())
	clearVar(t, env.Bus.Name())
}

func TestParseServeAccepts(t *testing.T) {
	tests := map[string]struct {
		args     []string
		contexts []string
		mode     runtime.Mode
		bus      string
	}{
		"single context":  {[]string{"--contexts=state"}, []string{"state"}, runtime.ModeLive, "kafka"},
		"context list":    {[]string{"--contexts=state,gateway"}, []string{"state", "gateway"}, runtime.ModeLive, "kafka"},
		"spaces trimmed":  {[]string{"--contexts=state, gateway"}, []string{"state", "gateway"}, runtime.ModeLive, "kafka"},
		"all with memory": {[]string{"--contexts=all", "--bus=memory"}, []string{"all"}, runtime.ModeLive, "memory"},
		"replay": {
			[]string{"--contexts=all", "--bus=memory", "--mode=replay", "--recording=r.jsonl"},
			[]string{"all"}, runtime.ModeReplay, "memory",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			clearModeAndBus(t)
			got, err := parseServe(tc.args, &bytes.Buffer{})
			if err != nil {
				t.Fatalf("parseServe(%v): %v", tc.args, err)
			}
			if strings.Join(got.contexts, ",") != strings.Join(tc.contexts, ",") {
				t.Errorf("contexts = %v, want %v", got.contexts, tc.contexts)
			}
			if got.mode != tc.mode {
				t.Errorf("mode = %q, want %q", got.mode, tc.mode)
			}
			if got.bus != tc.bus {
				t.Errorf("bus = %q, want %q", got.bus, tc.bus)
			}
		})
	}
}

func TestParseServeRejects(t *testing.T) {
	tests := map[string]struct {
		args []string
		want string
	}{
		"no contexts":            {[]string{}, "--contexts is required"},
		"empty contexts":         {[]string{"--contexts="}, "--contexts is required"},
		"only separators":        {[]string{"--contexts=,,"}, "--contexts is required"},
		"unknown mode":           {[]string{"--contexts=all", "--mode=dry-run"}, `--mode="dry-run": expected live or replay`},
		"unknown bus":            {[]string{"--contexts=all", "--bus=nats"}, `--bus="nats": expected kafka or memory`},
		"memory without all":     {[]string{"--contexts=state", "--bus=memory"}, "--bus=memory requires --contexts=all"},
		"memory with all listed": {[]string{"--contexts=all,state", "--bus=memory"}, "--bus=memory requires --contexts=all"},
		"unknown id source":      {[]string{"--contexts=all", "--id-source=random"}, "--id-source must be uuid or sequence"},
		"recording without replay": {
			[]string{"--contexts=all", "--bus=memory", "--recording=r.jsonl"},
			"--recording is only used with mode replay",
		},
		"stray argument": {[]string{"--contexts=all", "extra"}, "unexpected argument"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			clearModeAndBus(t)
			_, err := parseServe(tc.args, &bytes.Buffer{})
			if err == nil {
				t.Fatalf("parseServe(%v) = nil error, want %q", tc.args, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// The whole of T-408 in one test: the manifest decides, and it decides for a
// process nobody passed a flag to — which is every process compose starts, and
// the reason MV_MODE=replay used to produce a live one in silence.
func TestModeAndBusComeFromTheManifest(t *testing.T) {
	t.Setenv(env.Mode.Name(), "replay")
	t.Setenv(env.Bus.Name(), "memory")

	got, err := parseServe([]string{"--contexts=all"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseServe: %v", err)
	}
	if got.mode != runtime.ModeReplay {
		t.Errorf("mode = %q, want %q: MV_MODE decided nothing", got.mode, runtime.ModeReplay)
	}
	if got.bus != busMemory {
		t.Errorf("bus = %q, want %q: MV_BUS decided nothing", got.bus, busMemory)
	}
	if got.modeFrom != env.Mode.Name() || got.busFrom != env.Bus.Name() {
		t.Errorf("sources = %q/%q, want %q/%q", got.modeFrom, got.busFrom, env.Mode.Name(), env.Bus.Name())
	}
}

// The flag is the convenience of whoever is at a terminal, not a second source:
// it wins only when it was actually passed.
func TestFlagOverridesTheManifest(t *testing.T) {
	t.Setenv(env.Mode.Name(), "replay")
	t.Setenv(env.Bus.Name(), "memory")

	got, err := parseServe([]string{"--contexts=all", "--mode=live", "--bus=kafka"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseServe: %v", err)
	}
	if got.mode != runtime.ModeLive || got.bus != "kafka" {
		t.Fatalf("mode/bus = %q/%q, want live/kafka: the flag did not override the manifest", got.mode, got.bus)
	}
	if got.modeFrom != "--mode" || got.busFrom != "--bus" {
		t.Fatalf("sources = %q/%q, want --mode/--bus", got.modeFrom, got.busFrom)
	}
}

// A refusal has to name what the operator actually set. Telling someone to fix
// a flag he never passed is the same misdirection as ignoring his variable.
func TestRefusalNamesTheSourceOfTheValue(t *testing.T) {
	t.Setenv(env.Mode.Name(), "dry-run")
	clearVar(t, env.Bus.Name())

	_, err := parseServe([]string{"--contexts=all"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("parseServe accepted MV_MODE=dry-run")
	}
	if !strings.Contains(err.Error(), env.Mode.Name()) {
		t.Fatalf("error = %q, want it to name %s", err, env.Mode.Name())
	}
	if strings.Contains(err.Error(), "--mode") {
		t.Fatalf("error = %q, but no --mode was passed", err)
	}
}

// The value the manifest used to declare is refused by name, with the
// replacement in the sentence — never mapped onto kafka behind the operator's
// back, and never left to the bare "expected kafka or memory" of the enum.
func TestRetiredBusValueIsRefusedByName(t *testing.T) {
	t.Setenv(env.Bus.Name(), retiredBus)
	clearVar(t, env.Mode.Name())

	_, err := parseServe([]string{"--contexts=all"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("parseServe accepted %s=%s", env.Bus.Name(), retiredBus)
	}
	for _, want := range []string{env.Bus.Name(), retiredBus, "kafka", env.KafkaBrokers.Name()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

// One dictionary, held by a test rather than by a convention: the values the
// flags accept are the values the manifest declares, and the modes are the ones
// shared/runtime actually implements. This is the guard against the two lists
// drifting apart again — they did, and neither half had a name for what the
// other accepted (T-408).
func TestDictionaryIsTheManifest(t *testing.T) {
	wantModes := []string{string(runtime.ModeLive), string(runtime.ModeReplay)}
	if got := env.Mode.Enum(); strings.Join(got, ",") != strings.Join(wantModes, ",") {
		t.Errorf("%s enum = %v, want the modes of shared/runtime %v", env.Mode.Name(), got, wantModes)
	}
	if got := env.Bus.Enum(); strings.Join(got, ",") != "kafka,memory" {
		t.Errorf("%s enum = %v, want [kafka memory]", env.Bus.Name(), got)
	}
	if declares(env.Bus, retiredBus) {
		t.Errorf("%s still accepts %q: the retired value is back in the dictionary", env.Bus.Name(), retiredBus)
	}
	if !declares(env.Bus, busMemory) {
		t.Errorf("%s does not accept %q, which serve.go treats as a value of its own", env.Bus.Name(), busMemory)
	}
}

// The listen address of a process has one name. The two per-role names are
// retired, not deleted in silence: an operator whose .env still carries a line
// is told which variable took it over (mvctl env check --env).
func TestPerRoleAddressesAreRetired(t *testing.T) {
	for _, name := range []string{"MV_GATEWAY_ADDR", "MV_MEMORY_ADDR"} {
		if _, live := env.Lookup(name); live {
			t.Errorf("%s is declared again: one process serves one HTTP server, its address is %s",
				name, env.CoreAddr.Name())
		}
		reason, gone := env.DeprecatedNames()[name]
		if !gone {
			t.Errorf("%s is neither declared nor retired: an operator who still sets it hears nothing", name)
			continue
		}
		if !strings.Contains(reason, env.CoreAddr.Name()) {
			t.Errorf("the retirement of %s does not name %s: %q", name, env.CoreAddr.Name(), reason)
		}
	}
}

func TestParseServeReportsUnknownFlag(t *testing.T) {
	var stderr bytes.Buffer
	if _, err := parseServe([]string{"--contexts=all", "--admin-addr=:9000"}, &stderr); err == nil {
		t.Fatal("parseServe accepted an unknown flag")
	}
	if !strings.Contains(stderr.String(), "admin-addr") {
		t.Fatalf("stderr = %q, want it to name the unknown flag", stderr.String())
	}
}

// Wave 0 registers every context of the platform so that --contexts=all and
// the compose profiles work before internal/* exists: the seven contexts of
// foundation.md §1, each once, in the documented start order. swarm is among
// them although its factory lives in fake_contexts.go (T-255): moving its
// registration there would have moved it to the end of the start order.
func TestPlatformContextsAreRegisteredOnceInTheStartOrder(t *testing.T) {
	want := []string{"state", "laws", "mechanics", "llm", "swarm", "gateway", "memory"}
	if got := runtime.Names(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("registered contexts = %v, want %v", got, want)
	}
	if strings.Join(platformContexts, ",") != strings.Join(want, ",") {
		t.Fatalf("platformContexts = %v, want the 7 contexts of foundation.md §1 in start order %v",
			platformContexts, want)
	}
}

// Without MV_SWARM_FAKE every context of the binary is still the empty stub of
// wave 0, swarm included: the hook changes nothing unless it is asked to.
func TestStubIsHealthyAndDoesNothing(t *testing.T) {
	clearVar(t, env.SwarmFake.Name())
	contexts, err := runtime.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New(all): %v", err)
	}
	if len(contexts) != len(platformContexts) {
		t.Fatalf("New(all) built %d contexts, want %d", len(contexts), len(platformContexts))
	}
	for _, c := range contexts {
		if _, ok := c.(stub); !ok {
			t.Errorf("%s is %T without %s, want the stub of wave 0", c.Name(), c, env.SwarmFake.Name())
		}
		if err := c.Start(context.Background(), runtime.Deps{}); err != nil {
			t.Errorf("%s.Start: %v", c.Name(), err)
		}
		if got := c.Health(); got.Status != runtime.StatusOK {
			t.Errorf("%s.Health = %q, want ok", c.Name(), got.Status)
		}
		if err := c.Stop(context.Background()); err != nil {
			t.Errorf("%s.Stop: %v", c.Name(), err)
		}
	}
}

func TestIDGeneratorSequenceIsDeterministic(t *testing.T) {
	first, second := idGenerator("sequence"), idGenerator("sequence")
	for i := 0; i < 3; i++ {
		if a, b := first(), second(); a != b {
			t.Fatalf("id %d differs between runs: %q vs %q", i, a, b)
		}
	}
	if got := idGenerator("sequence")(); got != "seq-1" {
		t.Fatalf("first id = %q, want seq-1", got)
	}
}

func TestIDGeneratorUUIDIsUnique(t *testing.T) {
	gen := idGenerator("uuid")
	if a, b := gen(), gen(); a == b {
		t.Fatalf("uuid generator repeated %q", a)
	}
}

func TestRunHealth(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ok.Close()
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"fail"}`))
	}))
	defer failing.Close()
	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer garbage.Close()
	unreachable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachable.Close()

	tests := map[string]struct {
		url  string
		want int
	}{
		"healthy":     {ok.URL + "/health", 0},
		"unhealthy":   {failing.URL + "/health", 1},
		"bad body":    {garbage.URL + "/health", 1},
		"unreachable": {unreachable.URL + "/health", 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := runHealth([]string{"--url=" + tc.url}, &stdout, &stderr); got != tc.want {
				t.Fatalf("runHealth = %d, want %d (stderr: %s)", got, tc.want, stderr.String())
			}
		})
	}
}

// The default --url must be dialable, not merely a copy of the bind address:
// the shipped MV_CORE_ADDR is ":8090", and "http://:8090/health" is routed
// through HTTP_PROXY instead of to the local process (review T-007 Mi-1).
func TestDefaultHealthURLSubstitutesLoopbackForAWildcardHost(t *testing.T) {
	tests := map[string]struct {
		addr string
		want string
	}{
		"empty host":     {":8090", "http://127.0.0.1:8090/health"},
		"any ipv4":       {"0.0.0.0:8090", "http://127.0.0.1:8090/health"},
		"any ipv6":       {"[::]:8090", "http://127.0.0.1:8090/health"},
		"explicit host":  {"127.0.0.1:8090", "http://127.0.0.1:8090/health"},
		"named host":     {"core:8090", "http://core:8090/health"},
		"ipv6 host":      {"[::1]:8090", "http://[::1]:8090/health"},
		"without a port": {"core", "http://core/health"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("MV_CORE_ADDR", tc.addr)
			if got := defaultHealthURL(); got != tc.want {
				t.Fatalf("defaultHealthURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunDB(t *testing.T) {
	tests := map[string]struct {
		args []string
		want int
	}{
		"no subcommand":      {nil, 2},
		"unknown subcommand": {[]string{"migrate"}, 2},
		"backup":             {[]string{"backup"}, 1},
		"check":              {[]string{"check"}, 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			if got := runDB(tc.args, &stderr); got != tc.want {
				t.Fatalf("runDB(%v) = %d, want %d", tc.args, got, tc.want)
			}
			if stderr.Len() == 0 {
				t.Fatal("runDB said nothing on stderr")
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"version"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(version) = %d, want 0", got)
	}
	if strings.TrimSpace(stdout.String()) != version {
		t.Fatalf("stdout = %q, want %q", stdout.String(), version)
	}
}

func TestRunWithoutContextsFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("run() = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "--contexts is required") {
		t.Fatalf("stderr = %q, want the missing flag reported", stderr.String())
	}
}
