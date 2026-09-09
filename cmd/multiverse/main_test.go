package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"multiverse-core.io/shared/runtime"
)

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
		"unknown mode":           {[]string{"--contexts=all", "--mode=dry-run"}, "--mode must be live or replay"},
		"unknown bus":            {[]string{"--contexts=all", "--bus=nats"}, "--bus must be kafka or memory"},
		"memory without all":     {[]string{"--contexts=state", "--bus=memory"}, "--bus=memory requires --contexts=all"},
		"memory with all listed": {[]string{"--contexts=all,state", "--bus=memory"}, "--bus=memory requires --contexts=all"},
		"unknown id source":      {[]string{"--contexts=all", "--id-source=random"}, "--id-source must be uuid or sequence"},
		"recording without replay": {
			[]string{"--contexts=all", "--bus=memory", "--recording=r.jsonl"},
			"--recording is only used with --mode=replay",
		},
		"stray argument": {[]string{"--contexts=all", "extra"}, "unexpected argument"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
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

func TestParseServeReportsUnknownFlag(t *testing.T) {
	var stderr bytes.Buffer
	if _, err := parseServe([]string{"--contexts=all", "--admin-addr=:9000"}, &stderr); err == nil {
		t.Fatal("parseServe accepted an unknown flag")
	}
	if !strings.Contains(stderr.String(), "admin-addr") {
		t.Fatalf("stderr = %q, want it to name the unknown flag", stderr.String())
	}
}

// Wave 0 registers a stub for every context of the platform so that
// --contexts=all and the compose profiles work before internal/* exists.
func TestStubContextsAreRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, name := range runtime.Names() {
		registered[name] = true
	}
	for _, want := range stubContexts {
		if !registered[want] {
			t.Errorf("context %q is not registered (registered: %v)", want, runtime.Names())
		}
	}
	if len(stubContexts) != 7 {
		t.Fatalf("stubContexts has %d entries, want the 7 contexts of foundation.md §1", len(stubContexts))
	}
}

func TestStubIsHealthyAndDoesNothing(t *testing.T) {
	contexts, err := runtime.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New(all): %v", err)
	}
	if len(contexts) != len(stubContexts) {
		t.Fatalf("New(all) built %d contexts, want %d", len(contexts), len(stubContexts))
	}
	for _, c := range contexts {
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
