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
	if errs := runtime.StopAll(context.Background(), contexts, nil); len(errs) != 0 {
		t.Fatalf("StopAll errors = %v", errs)
	}
	if strings.Join(stopped, ",") != "swarm,mechanics,state" {
		t.Fatalf("stopped = %v, want reverse start order", stopped)
	}
}

// stopPanicking is a context whose Stop panics with value once it has put
// itself on the list of the stopped.
type stopPanicking struct {
	fake
	value any
}

func (p *stopPanicking) Stop(ctx context.Context) error {
	_ = p.fake.Stop(ctx)
	panic(p.value)
}

// failingStop is a context whose Stop returns err.
type failingStop struct {
	fake
	err error
}

func (f *failingStop) Stop(ctx context.Context) error {
	_ = f.fake.Stop(ctx)
	return f.err
}

// records keeps every record logged through it, so a test reads an attribute
// as it was logged, not as a text handler quotes it.
type records struct{ list []slog.Record }

func (h *records) Enabled(context.Context, slog.Level) bool { return true }
func (h *records) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *records) WithGroup(string) slog.Handler            { return h }

func (h *records) Handle(_ context.Context, r slog.Record) error {
	h.list = append(h.list, r.Clone())
	return nil
}

func attr(r slog.Record, key string) string {
	var value string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			value = a.Value.String()
			return false
		}
		return true
	})
	return value
}

// noPanicOut runs f and fails the test when a panic leaves it.
func noPanicOut(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s let the panic out: %v", what, r)
		}
	}()
	f()
}

// T-430, C-01 v1.6: a panic in Stop is an error of that context, not an
// unwinding that cuts the stop of the others short and closes the bus under
// them (ADR-023 p. 4). The error is one line; the log gets the value as it is
// and the stack.
func TestStopAllTurnsAPanicIntoAnError(t *testing.T) {
	tests := map[string]struct {
		value  any
		want   string
		logged string
	}{
		"a string":                 {value: "boom", want: "stop context mechanics: panic: boom", logged: "boom"},
		"an error":                 {value: errors.New("broken wiring"), want: "stop context mechanics: panic: broken wiring"},
		"nil":                      {value: nil, want: "stop context mechanics: panic: panic called with nil argument"},
		"an unprintable value":     {value: unprintable{}, want: "stop context mechanics: panic: runtime_test.unprintable (printing the value panicked)"},
		"a value of several lines": {value: "a\nb", want: `stop context mechanics: panic: a\nb`, logged: "a\nb"},
		"a value with CRLF":        {value: "a\r\nb", want: `stop context mechanics: panic: a\r\nb`, logged: "a\r\nb"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var stopped []string
			log := &records{}
			contexts := []runtime.Context{
				&fake{name: "state", stopped: &stopped},
				&stopPanicking{fake: fake{name: "mechanics", stopped: &stopped}, value: tc.value},
				&fake{name: "swarm", stopped: &stopped},
			}

			var errs []error
			noPanicOut(t, "StopAll", func() {
				errs = runtime.StopAll(context.Background(), contexts, slog.New(log))
			})

			if strings.Join(stopped, ",") != "swarm,mechanics,state" {
				t.Errorf("stopped = %v, want swarm,mechanics,state: the panic cut the stop of the rest short", stopped)
			}
			if len(errs) != 1 {
				t.Fatalf("StopAll errors = %q, want the one of mechanics", errs)
			}
			got := errs[0].Error()
			if !strings.HasPrefix(got, tc.want) {
				t.Errorf("error = %q, want %q", got, tc.want)
			}
			if strings.ContainsAny(got, "\r\n") || strings.Contains(got, "goroutine") {
				t.Errorf("error = %q, want one line without the stack", got)
			}
			if len(log.list) != 1 || log.list[0].Level != slog.LevelError {
				t.Fatalf("log = %d records, want one Error for the panic", len(log.list))
			}
			rec := log.list[0]
			if attr(rec, "context") != "mechanics" || !strings.Contains(attr(rec, "stack"), "goroutine") {
				t.Errorf("log: context = %q, stack = %q; want mechanics and the stack", attr(rec, "context"), attr(rec, "stack"))
			}
			if tc.logged != "" && attr(rec, "panic") != tc.logged {
				t.Errorf("log: panic = %q, want the value as it is, %q", attr(rec, "panic"), tc.logged)
			}
		})
	}
}

// The log of StopAll is optional, like Deps.Log of StartAll.
func TestStopAllTurnsAPanicIntoAnErrorWithoutALogger(t *testing.T) {
	p := &stopPanicking{fake: fake{name: "gateway"}, value: "boom"}
	var errs []error
	noPanicOut(t, "StopAll", func() { errs = runtime.StopAll(context.Background(), []runtime.Context{p}, nil) })
	if len(errs) != 1 || errs[0].Error() != "stop context gateway: panic: boom" {
		t.Fatalf("StopAll = %q, want the panic as an error", errs)
	}
}

// T-430, C-01 v1.6: the errors of the rollback of StartAll are no longer
// dropped. They follow the start error on the same line, in the reverse order
// of the rollback; the chain stays the one of the start error; a panic in
// Stop on the way back is one of them.
func TestStartAllReportsTheRollbackOnTheSameLine(t *testing.T) {
	errStart := errors.New("boom")
	errFlush := errors.New("flush failed")
	var stopped []string
	log := &records{}
	contexts := []runtime.Context{
		&stopPanicking{fake: fake{name: "state", stopped: &stopped}, value: "stuck"},
		&failingStop{fake: fake{name: "mechanics", stopped: &stopped}, err: errFlush},
		&fake{name: "swarm", startErr: errStart, stopped: &stopped},
	}

	var err error
	noPanicOut(t, "StartAll", func() {
		err = runtime.StartAll(context.Background(), contexts, runtime.Deps{Log: slog.New(log)})
	})

	want := "start context swarm: boom; stop context mechanics: flush failed; stop context state: panic: stuck"
	if err == nil || err.Error() != want {
		t.Fatalf("StartAll = %v, want %q", err, want)
	}
	if !errors.Is(err, errStart) {
		t.Error("errors.Is(err, start error) = false: the chain must stay the one of the start error")
	}
	if errors.Is(err, errFlush) {
		t.Error("errors.Is(err, stop error) = true: the rollback errors go in as text, not into the chain")
	}
	if strings.Join(stopped, ",") != "mechanics,state" {
		t.Errorf("stopped = %v, want mechanics,state", stopped)
	}
	if len(log.list) != 1 || attr(log.list[0], "context") != "state" ||
		!strings.Contains(attr(log.list[0], "stack"), "goroutine") {
		t.Errorf("log = %d records, want one Error with the stack of the panic of state", len(log.list))
	}
}

// T-430, C-01 v1.6: whatever the value of a panic, in Start or in Stop, the
// error of StartAll stays one line of stderr; the log keeps the value as it is.
func TestStartAllKeepsAPanicOfSeveralLinesOnOneLine(t *testing.T) {
	tests := map[string]struct{ value, want string }{
		"LF":   {value: "a\nb", want: `start context mechanics: panic: a\nb; stop context state: panic: a\nb`},
		"CRLF": {value: "a\r\nb", want: `start context mechanics: panic: a\r\nb; stop context state: panic: a\r\nb`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			log := &records{}
			contexts := []runtime.Context{
				&stopPanicking{fake: fake{name: "state"}, value: tc.value},
				&panicking{fake: fake{name: "mechanics"}, value: tc.value},
			}

			var err error
			noPanicOut(t, "StartAll", func() {
				err = runtime.StartAll(context.Background(), contexts, runtime.Deps{Log: slog.New(log)})
			})

			if err == nil || err.Error() != tc.want {
				t.Fatalf("StartAll = %q, want %q", err, tc.want)
			}
			if len(log.list) != 2 {
				t.Fatalf("log = %d records, want one per panic", len(log.list))
			}
			for i, owner := range []string{"mechanics", "state"} {
				rec := log.list[i]
				if attr(rec, "context") != owner || attr(rec, "panic") != tc.value {
					t.Errorf("log[%d]: context = %q, panic = %q; want %s and the value as it is, %q",
						i, attr(rec, "context"), attr(rec, "panic"), owner, tc.value)
				}
			}
		})
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
