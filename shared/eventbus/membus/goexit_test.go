package membus_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit"
)

// A handler that ends its goroutine with runtime.Goexit — t.FailNow in the
// handler of a consumer test is the usual way — is not a panic, and Delivery
// has nothing to park. What must not happen is that the group stays locked for
// good behind it: the next subscription of the group gets the same event, at
// least once, because the cursor never moved (review #1 of T-426; T-415).
func TestAGoexitInAHandlerLeavesTheGroupUsable(t *testing.T) {
	bus := newBus(t)
	ev := looked("w", "goexit")
	if err := bus.Publish(t.Context(), ev); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// The first subscription never returns: its goroutine ends inside the
	// handler. Its context is cancelled at the end all the same.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		_ = bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "g", func(context.Context, eventbus.Event) error {
			runtime.Goexit()
			return nil
		})
	}()
	select {
	case <-gone:
	case <-testkit.After(5 * time.Second):
		t.Fatal("the handler that calls Goexit was never called")
	}

	handled := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "g", func(_ context.Context, got eventbus.Event) error {
			handled <- got.ID
			return nil
		})
	}()
	select {
	case id := <-handled:
		if id != ev.ID {
			t.Errorf("the next subscription of the group got %s, want %s again", id, ev.ID)
		}
	case <-testkit.After(5 * time.Second):
		t.Fatal("the group is stuck: the lock of its cursor was left taken by the goroutine that called Goexit")
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("the second subscription returned %v, want nil on cancel", err)
	}
}
