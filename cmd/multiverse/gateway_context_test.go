package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/runtime"
)

// T-303: the name gateway is registered once, from contexts.go, and builds the
// real context instead of the stub of wave 0.
func TestGatewayIsTheRealContext(t *testing.T) {
	if n := countOf(runtime.Names(), gateway.Name); n != 1 {
		t.Fatalf("%s is registered %d times, want once", gateway.Name, n)
	}
	contexts, err := runtime.New([]string{gateway.Name})
	if err != nil {
		t.Fatalf("New(gateway): %v", err)
	}
	if _, ok := contexts[0].(*gateway.Context); !ok || len(contexts) != 1 {
		t.Fatalf("New(gateway) = %T, want *gateway.Context", contexts[0])
	}
	if _, isStub := factoryOf(gateway.Name)().(stub); isStub {
		t.Error("the factory of gateway still builds the stub")
	}
}

func countOf(list []string, name string) int {
	n := 0
	for _, s := range list {
		if s == name {
			n++
		}
	}
	return n
}

// lineWriter is the stdout of a process under test: it hands over the address
// of the start line once the process prints it.
type lineWriter struct {
	mu   sync.Mutex
	addr chan string
	sent bool
}

var listening = regexp.MustCompile(`listening on (\S+),`)

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if m := listening.FindSubmatch(p); m != nil && !w.sent {
		w.sent = true
		w.addr <- string(m[1])
	}
	return len(p), nil
}

// The DoD of T-303: a process with the context gateway serves the links routes
// and /health on MV_CORE_ADDR, and /health reports the Health of the gateway.
func TestTheProcessServesTheGatewayOnItsAddress(t *testing.T) {
	onLoopback(t)
	contexts, err := runtime.New([]string{gateway.Name})
	if err != nil {
		t.Fatal(err)
	}
	out := &lineWriter{addr: make(chan string, 1)}
	p := newProcess(contexts, openBus)
	p.stdout = out

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.run(ctx, cancel) }()

	var addr string
	wait, stopWaiting := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopWaiting()
	select {
	case addr = <-out.addr:
	case err := <-done:
		t.Fatalf("the process ended before it listened: %v", err)
	case <-wait.Done():
		t.Fatal("the process did not listen within 30s")
	}

	c := client.New("http://"+addr, "telegram-bot")
	c.Backoff = client.NoRetry
	res, err := c.Resolve(wait, "telegram", "7391846205")
	if err != nil || res.LinkStatus != "pending_consent" {
		t.Errorf("Resolve on the process address = %+v %v, want pending_consent", res, err)
	}

	req, _ := http.NewRequestWithContext(wait, http.MethodGet, "http://"+addr+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var health struct {
		Status  string `json:"status"`
		Details struct {
			Contexts map[string]runtime.Status `json:"contexts"`
		} `json:"details"`
	}
	err = json.NewDecoder(resp.Body).Decode(&health)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	gw, ok := health.Details.Contexts[gateway.Name]
	if health.Status != runtime.StatusOK || !ok || gw.Status != runtime.StatusOK || gw.Details["links_store"] != runtime.StatusOK {
		t.Errorf("/health = %+v, want ok with the health of the gateway", health)
	}
	names := make([]string, 0, len(health.Details.Contexts))
	for name := range health.Details.Contexts {
		names = append(names, name)
	}
	if !slices.Equal(names, []string{gateway.Name}) {
		t.Errorf("/health reports %v, want the gateway only", names)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("run: %v", err)
		}
	case <-wait.Done():
		t.Fatal("the process did not stop within 30s")
	}
}
