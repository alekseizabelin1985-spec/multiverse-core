package state

import (
	"testing"
	"time"
)

// The pause between two attempts to publish an answer doubles from the first
// and stays at the cap: the attempts are bounded in pace, not in number.
func TestThePauseBetweenAttemptsDoublesUpToTheCap(t *testing.T) {
	want := []time.Duration{
		100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond,
		1600 * time.Millisecond, 3200 * time.Millisecond, 5 * time.Second, 5 * time.Second,
	}
	for i, pause := range want {
		if got := retryPause(i + 1); got != pause {
			t.Errorf("pause after attempt %d = %v, want %v", i+1, got, pause)
		}
	}
	if got := retryPause(1 << 20); got != publishRetryMax {
		t.Errorf("pause after a million attempts = %v, want the cap %v", got, publishRetryMax)
	}
}
