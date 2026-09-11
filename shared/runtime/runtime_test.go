package runtime_test

import (
	"context"
	"errors"
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
