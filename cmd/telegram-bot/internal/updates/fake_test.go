package updates

import (
	"context"
	"errors"
	"testing"
)

func TestFakeHandsOverQueuedUpdatesInOrderThenStopsOnClose(t *testing.T) {
	f := NewFake(Update{ID: 1}, Update{ID: 2})
	var got []int64
	done := make(chan error, 1)
	go func() {
		done <- f.Start(context.Background(), func(_ context.Context, u Update) {
			got = append(got, u.ID)
			if u.ID == 2 {
				f.Push(Update{ID: 3})
				f.Close()
			}
		})
	}()
	if err := <-done; err != nil {
		t.Fatalf("Start = %v, want nil after Close", err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("handled %v, want [1 2 3]: an update pushed before Close is still handled", got)
	}
	if err := f.Start(context.Background(), func(context.Context, Update) {}); err == nil {
		t.Fatal("a fake source starts once")
	}
}

func TestFakeFailsWithTheGivenErrorAfterTheQueueBeforeIt(t *testing.T) {
	f := NewFake(Update{ID: 1})
	f.Fail(ErrConflict)
	f.Push(Update{ID: 2})
	var got []int64
	err := f.Start(context.Background(), func(_ context.Context, u Update) { got = append(got, u.ID) })
	if !errors.Is(err, ErrConflict) || ExitCode(err) != ExitConflict {
		t.Fatalf("Start = %v, want ErrConflict", err)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("handled %v, want only the update queued before the failure", got)
	}
}

func TestFakeWaitsForUpdatesAndStopsWithTheContext(t *testing.T) {
	f := NewFake()
	ctx, cancel := context.WithCancel(context.Background())
	handled := make(chan int64)
	done := make(chan error, 1)
	go func() {
		done <- f.Start(ctx, func(_ context.Context, u Update) { handled <- u.ID })
	}()
	f.Push(Update{ID: 7})
	if id := <-handled; id != 7 {
		t.Fatalf("handled %d, want 7", id)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start after cancel = %v, want nil", err)
	}

	cancelled, stop := context.WithCancel(context.Background())
	stop()
	g := NewFake(Update{ID: 1})
	if err := g.Start(cancelled, func(context.Context, Update) { t.Error("no update after the context ended") }); err != nil {
		t.Fatal(err)
	}
	if err := g.Start(context.Background(), nil); err == nil {
		t.Fatal("Start without a handler must fail")
	}
}
