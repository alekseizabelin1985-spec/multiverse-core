package runtime_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"multiverse-core.io/shared/runtime"
)

type fake struct {
	name     string
	deps     []string
	health   runtime.Status
	startErr error
	started  *[]string
	stopped  *[]string
}

func (f *fake) Name() string        { return f.name }
func (f *fake) DependsOn() []string { return f.deps }

func (f *fake) Start(context.Context, runtime.Deps) error {
	if f.startErr != nil {
		return f.startErr
	}
	if f.started != nil {
		*f.started = append(*f.started, f.name)
	}
	return nil
}

func (f *fake) Stop(context.Context) error {
	if f.stopped != nil {
		*f.stopped = append(*f.stopped, f.name)
	}
	return nil
}

func (f *fake) Health() runtime.Status {
	if f.health.Status == "" {
		return runtime.OK()
	}
	return f.health
}

type routed struct {
	fake
	mounted *bool
}

func (r *routed) Routes(mux *http.ServeMux) {
	*r.mounted = true
	mux.HandleFunc("GET /v1/admin/probe", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func register(r *runtime.Registry, name string, deps ...string) {
	r.Register(name, func() runtime.Context { return &fake{name: name, deps: deps} })
}

func names(contexts []runtime.Context) []string {
	out := make([]string, 0, len(contexts))
	for _, c := range contexts {
		out = append(out, c.Name())
	}
	return out
}

func TestNewOrdersByDependsOn(t *testing.T) {
	r := runtime.NewRegistry()
	register(r, "swarm", "mechanics", "laws")
	register(r, "laws")
	register(r, "mechanics", "state")
	register(r, "state")

	got, err := r.New([]string{"swarm", "state", "mechanics", "laws"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	position := map[string]int{}
	for i, name := range names(got) {
		position[name] = i
	}
	for _, pair := range [][2]string{{"state", "mechanics"}, {"mechanics", "swarm"}, {"laws", "swarm"}} {
		if position[pair[0]] > position[pair[1]] {
			t.Errorf("%s starts after %s: %v", pair[0], pair[1], names(got))
		}
	}
}

// The order on the command line must not change the start order, otherwise a
// compose file and a shell invocation would boot differently.
func TestNewOrderIsIndependentOfArgumentOrder(t *testing.T) {
	r := runtime.NewRegistry()
	register(r, "state")
	register(r, "laws")
	register(r, "gateway")

	forward, err := r.New([]string{"state", "laws", "gateway"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	backward, err := r.New([]string{"gateway", "laws", "state"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if strings.Join(names(forward), ",") != strings.Join(names(backward), ",") {
		t.Fatalf("order differs: %v vs %v", names(forward), names(backward))
	}
}

// A dependency that runs in another process is reached over the bus, so it
// must not block the contexts selected here.
func TestNewIgnoresDependenciesOutsideTheSelection(t *testing.T) {
	r := runtime.NewRegistry()
	register(r, "state")
	register(r, "gateway", "state")

	got, err := r.New([]string{"gateway"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "gateway" {
		t.Fatalf("New = %v, want [gateway]", names(got))
	}
}

func TestNewAllExpandsToEveryContext(t *testing.T) {
	r := runtime.NewRegistry()
	register(r, "state")
	register(r, "gateway")

	got, err := r.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if strings.Join(names(got), ",") != "state,gateway" {
		t.Fatalf("New(all) = %v, want [state gateway]", names(got))
	}
}

func TestNewRejects(t *testing.T) {
	r := runtime.NewRegistry()
	register(r, "state")
	r.Register("cyclic-a", func() runtime.Context { return &fake{name: "cyclic-a", deps: []string{"cyclic-b"}} })
	r.Register("cyclic-b", func() runtime.Context { return &fake{name: "cyclic-b", deps: []string{"cyclic-a"}} })

	tests := map[string]struct {
		names []string
		want  string
	}{
		"unknown name":     {[]string{"state", "nope"}, "unknown context"},
		"empty selection":  {nil, "no contexts requested"},
		"all with others":  {[]string{runtime.All, "state"}, "cannot be combined"},
		"dependency cycle": {[]string{"cyclic-a", "cyclic-b"}, "dependency cycle"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := r.New(tc.names)
			if err == nil {
				t.Fatalf("New(%v) = nil error, want %q", tc.names, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("New(%v) error = %q, want it to contain %q", tc.names, err, tc.want)
			}
		})
	}
}

func TestNewUnknownContextIsMatchable(t *testing.T) {
	r := runtime.NewRegistry()
	register(r, "state")
	if _, err := r.New([]string{"nope"}); !errors.Is(err, runtime.ErrUnknownContext) {
		t.Fatalf("error = %v, want it to wrap ErrUnknownContext", err)
	}
}

func TestRegisterTwicePanics(t *testing.T) {
	r := runtime.NewRegistry()
	register(r, "state")
	defer func() {
		if recover() == nil {
			t.Fatal("registering the same name twice did not panic")
		}
	}()
	register(r, "state")
}

func TestStartAllMountsRoutesBeforeStart(t *testing.T) {
	mounted := false
	var started []string
	r := &routed{fake: fake{name: "swarm", started: &started}, mounted: &mounted}
	mux := http.NewServeMux()

	if err := runtime.StartAll(context.Background(), []runtime.Context{r}, runtime.Deps{Mux: mux}); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	if !mounted {
		t.Fatal("Routes was not called")
	}
	if len(started) != 1 {
		t.Fatalf("started = %v, want [swarm]", started)
	}
}

func TestStartAllStopsStartedContextsOnFailure(t *testing.T) {
	var started, stopped []string
	first := &fake{name: "state", started: &started, stopped: &stopped}
	second := &fake{name: "mechanics", startErr: errors.New("boom"), stopped: &stopped}

	err := runtime.StartAll(context.Background(), []runtime.Context{first, second}, runtime.Deps{})
	if err == nil {
		t.Fatal("StartAll = nil error, want the failure of mechanics")
	}
	if !strings.Contains(err.Error(), "mechanics") {
		t.Fatalf("error = %q, want it to name the failing context", err)
	}
	if strings.Join(stopped, ",") != "state" {
		t.Fatalf("stopped = %v, want [state]", stopped)
	}
}

// panicking is a context whose Start or Routes panics with value.
type panicking struct {
	fake
	value    any
	inRoutes bool
}

func (p *panicking) Start(context.Context, runtime.Deps) error { panic(p.value) }

func (p *panicking) Routes(*http.ServeMux) {
	if p.inRoutes {
		panic(p.value)
	}
}

// unprintable panics when fmt asks it for its text, and so does the value of
// that panic: fmt gives up and re-panics, which is the second panic StartAll
// must not let out either.
type unprintable struct{}

func (unprintable) String() string { panic(unprintable{}) }

// T-415: a panic in Start is a failed start, not an unwinding that closes the
// bus under the contexts started before it (ADR-023 p. 4). The contexts
// already up are stopped, the error names the context and the value, and the
// stack goes to the log of the process rather than into the error.
func TestStartAllTurnsAPanicIntoAFailedStart(t *testing.T) {
	tests := map[string]struct {
		value    any
		inRoutes bool
		want     string
	}{
		"a string":             {value: "boom", want: "panic: boom"},
		"an error":             {value: errors.New("broken wiring"), want: "panic: broken wiring"},
		"nil":                  {value: nil, want: "panic: panic called with nil argument"},
		"in Routes":            {value: "bad pattern", inRoutes: true, want: "panic: bad pattern"},
		"an unprintable value": {value: unprintable{}, want: "runtime_test.unprintable (printing the value panicked)"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var started, stopped []string
			var logged strings.Builder
			first := &fake{name: "state", started: &started, stopped: &stopped}
			second := &panicking{fake: fake{name: "mechanics", stopped: &stopped}, value: tc.value, inRoutes: tc.inRoutes}
			third := &fake{name: "swarm", started: &started, stopped: &stopped}
			deps := runtime.Deps{Mux: http.NewServeMux(), Log: slog.New(slog.NewTextHandler(&logged, nil))}

			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("StartAll let the panic out: %v", r)
					}
				}()
				err = runtime.StartAll(context.Background(), []runtime.Context{first, second, third}, deps)
			}()

			if err == nil {
				t.Fatal("StartAll = nil error, want the panic of mechanics as a failed start")
			}
			if !strings.Contains(err.Error(), "start context mechanics") || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name mechanics and %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "goroutine") {
				t.Errorf("the stack went into the error, which ends up in one line of stderr: %q", err)
			}
			if strings.Join(started, ",") != "state" || strings.Join(stopped, ",") != "state" {
				t.Errorf("started %v, stopped %v; want state started and stopped, nothing after mechanics", started, stopped)
			}
			if !strings.Contains(logged.String(), "level=ERROR") || !strings.Contains(logged.String(), "goroutine") {
				t.Errorf("the panic left no error with its stack in the log: %q", logged.String())
			}
		})
	}
}

// A process without a logger still gets the error: Deps.Log is optional.
func TestStartAllTurnsAPanicIntoAnErrorWithoutALogger(t *testing.T) {
	p := &panicking{fake: fake{name: "gateway"}, value: "boom"}
	if err := runtime.StartAll(context.Background(), []runtime.Context{p}, runtime.Deps{}); err == nil ||
		!strings.Contains(err.Error(), "panic: boom") {
		t.Fatalf("StartAll = %v, want the panic as an error", err)
	}
}

func TestStopAllRunsInReverseOrder(t *testing.T) {
	var stopped []string
	contexts := []runtime.Context{
		&fake{name: "state", stopped: &stopped},
		&fake{name: "mechanics", stopped: &stopped},
		&fake{name: "swarm", stopped: &stopped},
	}
	if errs := runtime.StopAll(context.Background(), contexts); len(errs) != 0 {
		t.Fatalf("StopAll errors = %v", errs)
	}
	if strings.Join(stopped, ",") != "swarm,mechanics,state" {
		t.Fatalf("stopped = %v, want reverse start order", stopped)
	}
}

func TestAggregate(t *testing.T) {
	tests := map[string]struct {
		statuses []string
		want     string
	}{
		"all ok":       {[]string{runtime.StatusOK, runtime.StatusOK}, runtime.StatusOK},
		"one degraded": {[]string{runtime.StatusOK, runtime.StatusDegraded}, runtime.StatusDegraded},
		"fail wins":    {[]string{runtime.StatusDegraded, runtime.StatusFail}, runtime.StatusFail},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var contexts []runtime.Context
			for i, s := range tc.statuses {
				contexts = append(contexts, &fake{name: string(rune('a' + i)), health: runtime.Status{Status: s}})
			}
			got := runtime.Aggregate(contexts)()
			if got.Status != tc.want {
				t.Fatalf("Status = %q, want %q", got.Status, tc.want)
			}
			byName, ok := got.Details["contexts"].(map[string]runtime.Status)
			if !ok {
				t.Fatalf("details.contexts = %T, want map[string]runtime.Status", got.Details["contexts"])
			}
			if len(byName) != len(tc.statuses) {
				t.Fatalf("details.contexts has %d entries, want %d", len(byName), len(tc.statuses))
			}
		})
	}
}
