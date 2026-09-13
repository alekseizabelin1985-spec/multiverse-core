package actions_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/clock"
)

func newLimiter(t *testing.T) (*actions.Limiter, *clock.Manual) {
	t.Helper()
	manual := clock.NewManual(t0)
	l, err := actions.NewLimiter(manual, 30, 5)
	if err != nil {
		t.Fatal(err)
	}
	return l, manual
}

// Five actions in a row pass, the sixth waits for the token of the next two
// seconds (30 per minute).
func TestTheBurstIsFiveInARow(t *testing.T) {
	l, manual := newLimiter(t)
	for i := range 5 {
		if ok, _ := l.Take(playerA); !ok {
			t.Fatalf("action %d of the burst refused", i+1)
		}
	}
	if ok, retry := l.Take(playerA); ok || retry != 2 {
		t.Fatalf("sixth action = %v, Retry-After %d; want refused with 2", ok, retry)
	}
	if ok, _ := l.Take(playerB); !ok {
		t.Error("the burst of player-A limits player-B")
	}
	manual.Advance(2 * time.Second)
	if ok, _ := l.Take(playerA); !ok {
		t.Error("the token of the next two seconds did not come back")
	}
}

// The 31st action within a minute is refused whatever the bucket says, and in
// the 61st second, when the first action has left the minute, one is accepted
// again (SEC-11, NFR-049).
func TestThe31stActionOfAMinuteIsRefused(t *testing.T) {
	l, manual := newLimiter(t)
	// 1.9 s apart the bucket never runs dry: it refills 0.95 token per step.
	for i := range 30 {
		if ok, _ := l.Take(playerA); !ok {
			t.Fatalf("action %d at %s refused", i+1, manual.Now().Sub(t0))
		}
		manual.Advance(1900 * time.Millisecond)
	}
	at := manual.Now().Sub(t0) // 57 s
	ok, retry := l.Take(playerA)
	if ok || retry != 3 {
		t.Fatalf("31st action at %s = %v, Retry-After %d; want refused with 3", at, ok, retry)
	}
	manual.Set(t0.Add(time.Minute - time.Millisecond))
	if ok, _ := l.Take(playerA); ok {
		t.Fatal("an action just before the first one leaves the minute is accepted")
	}
	manual.Set(t0.Add(time.Minute))
	if ok, _ := l.Take(playerA); !ok {
		t.Fatal("in the 61st second the action is refused")
	}
}

// Retry-After is the wait rounded up to a whole second: rounded down, the
// client would come back before the limit allows it and get 429 again
// (review #1, Mi-4).
func TestRetryAfterRoundsTheWaitUp(t *testing.T) {
	l, manual := newLimiter(t)
	for range 5 {
		l.Take(playerA)
	}
	manual.Advance(500 * time.Millisecond)
	if ok, retry := l.Take(playerA); ok || retry != 2 {
		t.Errorf("a wait of 1.5 s = %v, Retry-After %d; want refused with 2", ok, retry)
	}

	m, clk := newLimiter(t)
	for i := range 30 {
		if ok, _ := m.Take(playerB); !ok {
			t.Fatalf("action %d refused", i+1)
		}
		clk.Advance(1900 * time.Millisecond)
	}
	clk.Advance(300 * time.Millisecond) // 57.3 s: the first action leaves the minute in 2.7 s
	if ok, retry := m.Take(playerB); ok || retry != 3 {
		t.Errorf("the 31st action with a wait of 2.7 s = %v, Retry-After %d; want refused with 3", ok, retry)
	}
}

// The limiter applies to postAction only, by the player of the path, and a
// refused action is not counted.
func TestTheLimiterReadsThePlayerOfTheRoute(t *testing.T) {
	l, _ := newLimiter(t)
	mux := http.NewServeMux()
	var allowed []bool
	var retries []int
	mux.HandleFunc("POST /v1/players/{player_id}/actions", func(_ http.ResponseWriter, r *http.Request) {
		ok, retry := l.Allow(r, api.Route{OperationID: actions.OperationPostAction})
		allowed, retries = append(allowed, ok), append(retries, retry)
	})
	for range 6 {
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/players/player-A/actions", nil))
	}
	if allowed[4] != true || allowed[5] != false || retries[5] != 2 {
		t.Errorf("allowed %v, retries %v; want the sixth refused with 2", allowed, retries)
	}
	for range 10 {
		if ok, _ := l.Allow(httptest.NewRequest(http.MethodGet, "/v1/worlds", nil), api.Route{OperationID: "listWorlds"}); !ok {
			t.Fatal("an operation other than postAction is limited")
		}
	}
	if ok, _ := l.Take("player-B"); !ok {
		t.Error("another player is limited")
	}
}

// Sweep forgets a player only when the player is indistinguishable from one
// who never acted: the bucket full and the minute empty.
func TestSweepForgetsTheIdlePlayers(t *testing.T) {
	l, manual := newLimiter(t)
	l.Take(playerA)
	l.Take(playerB)
	manual.Advance(30 * time.Second)
	l.Take(playerB)
	manual.Advance(31 * time.Second)
	l.Sweep(manual.Now())
	if got := l.Players(); got != 1 {
		t.Fatalf("players after the sweep = %d, want 1 (player-B acted within the minute)", got)
	}
	manual.Advance(actions.LimiterSweepInterval)
	l.Sweep(manual.Now())
	if got := l.Players(); got != 0 {
		t.Errorf("players after the idle interval = %d, want 0", got)
	}
}

func TestNewLimiterRefusesALimitBelowOne(t *testing.T) {
	for _, c := range [][2]int{{0, 5}, {30, 0}, {-1, -1}} {
		if _, err := actions.NewLimiter(clock.NewManual(t0), c[0], c[1]); err == nil {
			t.Errorf("NewLimiter(%d, %d) accepted", c[0], c[1])
		}
	}
}
