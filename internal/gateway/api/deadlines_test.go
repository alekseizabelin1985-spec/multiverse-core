package api_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/clock"
)

type deadline struct{ read, write time.Duration }

// An ordinary operation gets the deadlines of its connection from the timeout
// middleware: the request timeout plus the margin, for reading and for
// writing. A long-poll gets none there: its handler sets its own from wait_ms
// (component §5.1 p. 8, C-01 v1.8).
func TestTheTimeoutSetsTheDeadlinesOfAnOrdinaryOperationOnly(t *testing.T) {
	var got []deadline
	cfg := &api.Config{ClientIDs: []string{"telegram-bot"}, RequestIDs: func() string { return "r" },
		Clock: clock.NewManual(time.Time{}), Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		SetDeadlines: func(_ http.ResponseWriter, read, write time.Duration) error {
			got = append(got, deadline{read, write})
			return http.ErrNotSupported
		}}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r := api.NewRouter()
	r.Handle("listWorlds", http.MethodGet, "/v1/worlds", ok)
	r.Handle("pollDeliveries", http.MethodGet, "/v1/clients/{client_id}/deliveries", ok)
	mux := http.NewServeMux()
	r.Mount(mux, api.Chain(cfg)...)

	for _, path := range []string{"/v1/worlds", "/v1/clients/telegram-bot/deliveries"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(api.HeaderClientID, "telegram-bot")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s = %d: a writer without deadlines must not fail the request", path, rec.Code)
		}
	}
	want := api.RequestTimeout + api.DeadlineMargin
	if len(got) != 1 || got[0] != (deadline{want, want}) {
		t.Errorf("deadlines set = %v, want one pair of %s for listWorlds and none for the long-poll", got, want)
	}
}
