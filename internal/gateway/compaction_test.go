package gateway_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// holdReader opens a read transaction on the links.db of dir that has read a
// snapshot: while it stays, a checkpoint cannot truncate the WAL. The returned
// function ends it.
func holdReader(t *testing.T, dir string) func() {
	t.Helper()
	ctx := context.Background()
	db, err := store.OpenLinks(ctx, store.LinksPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return func() { _ = tx.Rollback() }
}

// occurrences counts id in links.db of dir and in its WAL.
func occurrences(t *testing.T, dir, id string) int {
	t.Helper()
	n := 0
	for _, name := range []string{store.LinksPath(dir), store.LinksPath(dir) + "-wal"} {
		data, err := os.ReadFile(name)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		n += strings.Count(string(data), id)
	}
	return n
}

func consent(t *testing.T, cl *client.Client, id string) {
	t.Helper()
	if _, err := cl.Consent(context.Background(), api.ConsentRequest{ExternalPlatform: links.PlatformTelegram, ExternalID: id,
		NoticeShown: true, Consent: true, AgeConfirmed: true, ShownAt: t0}); err != nil {
		t.Fatalf("Consent: %v", err)
	}
}

// forgetRaw sends /forget without the client, to see the headers of the answer.
func forgetRaw(t *testing.T, baseURL, id string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/v1/links",
		strings.NewReader(`{"external_platform":"telegram","external_id":"`+id+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", api.ContentTypeJSON)
	req.Header.Set(api.HeaderClientID, "telegram-bot")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp
}

// The decision on 503 forget_incomplete, seen from outside: a /forget whose
// wipe is blocked answers 503 with Retry-After and the gateway is degraded;
// while the compaction is pending every /forget answers 503, that of an
// account without a link included; once the reader is gone the repeat answers
// {deleted: false} and the gateway is ok again. Stop under a reader reports
// the wipe it could not finish.
func TestAPendingCompactionIsVisibleUntilItIsFinished(t *testing.T) {
	r := start(t, runtime.ModeLive)
	ctx := context.Background()
	consent(t, r.client, externalID)
	if err := gateway.SetLinksBusyTimeout(r.ctx, 0); err != nil {
		t.Fatal(err)
	}
	leave := holdReader(t, r.dir)

	resp := forgetRaw(t, r.client.BaseURL, externalID)
	wantRetry := strconv.Itoa(int(handlers.ForgetRetryAfter / time.Second))
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get(api.HeaderRetryAfter) != wantRetry {
		t.Fatalf("/forget under a reader = %d Retry-After %q, want 503 and %s", resp.StatusCode, resp.Header.Get(api.HeaderRetryAfter), wantRetry)
	}
	if occurrences(t, r.dir, externalID) == 0 {
		t.Fatal("control: the blocked wipe left no trace, the scans below prove nothing")
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusDegraded || h.Details["links_compaction"] != "pending" {
		t.Errorf("Health while the compaction is pending = %+v, want degraded with links_compaction pending", h)
	}
	var apiErr *client.APIError
	if _, err := r.client.Forget(ctx, links.PlatformTelegram, "5550001111"); !errors.As(err, &apiErr) || apiErr.Code != api.CodeForgetIncomplete {
		t.Errorf("/forget of an account without a link while pending = %v, want 503 forget_incomplete", err)
	}

	leave()
	again, err := r.client.Forget(ctx, links.PlatformTelegram, externalID)
	if err != nil || again.Deleted {
		t.Fatalf("repeat after the reader left = %+v %v, want {deleted:false}", again, err)
	}
	if n := occurrences(t, r.dir, externalID); n != 0 {
		t.Errorf("the forgotten ID occurs %d times after the repeat", n)
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusOK || h.Details["links_compaction"] != nil {
		t.Errorf("Health after the wipe = %+v, want ok", h)
	}

	consent(t, r.client, "5550001111")
	leave = holdReader(t, r.dir)
	defer leave()
	if _, err := r.client.Forget(ctx, links.PlatformTelegram, "5550001111"); !errors.As(err, &apiErr) || apiErr.Code != api.CodeForgetIncomplete {
		t.Fatalf("second /forget under a reader = %v, want 503 forget_incomplete", err)
	}
	if err := r.ctx.Stop(ctx); err == nil || !strings.Contains(err.Error(), "uncompacted") {
		t.Errorf("Stop under the reader = %v, want the wipe it could not finish reported", err)
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusFail {
		t.Errorf("Health after Stop = %+v, want fail", h)
	}
}

// Stop is the last chance of the process to wipe what a blocked /forget left:
// with the reader gone, it finishes the compaction before it closes links.db
// (review #1, Mi-3, mutant K2).
func TestStopFinishesAPendingCompaction(t *testing.T) {
	r := start(t, runtime.ModeReplay)
	consent(t, r.client, externalID)
	if err := gateway.SetLinksBusyTimeout(r.ctx, 0); err != nil {
		t.Fatal(err)
	}
	leave := holdReader(t, r.dir)
	if resp := forgetRaw(t, r.client.BaseURL, externalID); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("/forget under a reader = %d, want 503", resp.StatusCode)
	}
	if occurrences(t, r.dir, externalID) == 0 {
		t.Fatal("control: the blocked wipe left no trace, the scan after Stop proves nothing")
	}
	leave()

	// Replay runs no sweeper: only Stop can have finished the wipe.
	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := occurrences(t, r.dir, externalID); n != 0 {
		t.Errorf("the forgotten ID occurs %d times after Stop", n)
	}
}

// The mark "compaction pending" lives in memory, so the start compacts
// links.db whatever it finds (review #1, Mi-2). The files are those a process
// leaves when it stops between the DELETE of /forget and the checkpoint: the
// row is deleted in the WAL, its bytes are still in the database file.
func TestTheStartWipesWhatAStoppedProcessLeft(t *testing.T) {
	ctx := context.Background()
	src := sqlitedir.Temp(t)
	db, err := store.OpenLinks(ctx, store.LinksPath(src))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.MigrateLinks(ctx, db); err != nil {
		t.Fatal(err)
	}
	s, err := links.NewSQLite(db, sequence())
	if err != nil {
		t.Fatal(err)
	}
	form := links.ConsentForm{NoticeShown: true, Consent: true, AgeConfirmed: true, ShownAt: t0}
	for _, id := range []string{externalID, "5550001111"} {
		if _, err := s.Consent(ctx, links.PlatformTelegram, id, form, t0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Compact(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM links WHERE external_id = ?", externalID); err != nil {
		t.Fatal(err)
	}
	dir := sqlitedir.Temp(t)
	for _, name := range []string{store.LinksFile, store.LinksFile + "-wal"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(store.LinksPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), externalID) {
		t.Fatal("control: the deleted ID is not in the file left behind, the scan below proves nothing")
	}

	// Replay: the wipe is not the work of the sweeper, which does not run there.
	r := startIn(t, runtime.ModeReplay, dir)
	if n := occurrences(t, dir, externalID); n != 0 {
		t.Errorf("the deleted ID occurs %d times after the start", n)
	}
	if occurrences(t, dir, "5550001111") == 0 {
		t.Error("the scan cannot see an ID that stays: the file was not read")
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusOK {
		t.Errorf("Health = %+v, want ok", h)
	}
}

// A compaction the start cannot finish does not stop the start: the gateway
// starts degraded, and the sweeper finishes the wipe once the reader is gone.
func TestABlockedCompactionAtTheStartIsFinishedByTheSweeper(t *testing.T) {
	dir := sqlitedir.Temp(t)
	first := startIn(t, runtime.ModeLive, dir)
	consent(t, first.client, externalID)
	if err := first.ctx.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	leave := holdReader(t, dir)
	// The start waits busy_timeout, five seconds, for the reader.
	r := startIn(t, runtime.ModeLive, dir)
	if h := r.ctx.Health(); h.Status != runtime.StatusDegraded || h.Details["links_compaction"] != "pending" {
		t.Fatalf("Health after a start under a reader = %+v, want degraded with links_compaction pending", h)
	}
	leave()

	// The sweeper registers its tickers in its own goroutine: advance until
	// the tick lands, within a bound.
	deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for r.ctx.Health().Status != runtime.StatusOK && deadline.Err() == nil {
		r.clock.Advance(store.SweepInterval)
		time.Sleep(20 * time.Millisecond)
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusOK {
		t.Errorf("Health after the sweeps = %+v, want ok", h)
	}
}

// The hourly compaction of the sweeper is the safety net of ADR-019 addendum
// p. 1 for a deletion that left its bytes with no mark set, such as a mark
// lost in the window between a sweep and a parallel /forget: only the tick of
// store.LinksCompactInterval wipes them (acceptance of T-303, review #2,
// R2-N-1). Sweep alone does not compact without the mark.
func TestTheHourlyCompactionWipesADeletionThatHasNoMark(t *testing.T) {
	ctx := context.Background()
	dir := sqlitedir.Temp(t)
	first := startIn(t, runtime.ModeLive, dir)
	for _, id := range []string{externalID, "5550001111"} {
		consent(t, first.client, id)
	}
	if err := first.ctx.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	// The start compacts: the rows are in the database file and the WAL is empty.
	r := startIn(t, runtime.ModeLive, dir)
	other, err := store.OpenLinks(ctx, store.LinksPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	if _, err := other.ExecContext(ctx, "DELETE FROM links WHERE external_id = ?", externalID); err != nil {
		t.Fatal(err)
	}
	if occurrences(t, dir, externalID) == 0 {
		t.Fatal("control: the deleted ID is not in links.db, the scan below proves nothing")
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusOK {
		t.Fatalf("control: Health = %+v, want ok: the deletion must carry no mark", h)
	}

	// A sweep alone leaves the bytes: the mark is not set.
	r.clock.Advance(store.SweepInterval)
	time.Sleep(50 * time.Millisecond)

	// The sweeper registers its tickers in its own goroutine: advance until
	// the hourly tick lands, within a bound.
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for occurrences(t, dir, externalID) != 0 && deadline.Err() == nil {
		r.clock.Advance(store.LinksCompactInterval)
		time.Sleep(20 * time.Millisecond)
	}
	if n := occurrences(t, dir, externalID); n != 0 {
		t.Errorf("the deleted ID occurs %d times after the hourly compaction", n)
	}
	if occurrences(t, dir, "5550001111") == 0 {
		t.Error("the scan cannot see an ID that stays: the file was not read")
	}
}

// Outside compose the default /data of the manifest is the root of the file
// system; the error of the start names the variable to set (review #1, Mi-5).
func TestStartNamesTheDataDirectoryVariableWhenItCannotOpen(t *testing.T) {
	file := filepath.Join(sqlitedir.Temp(t), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	manual := clock.NewManual(t0)
	c := gateway.New(env.MapSource(map[string]string{env.GatewayDataDir.Name(): file}))
	err := c.Start(context.Background(), runtime.Deps{Clock: manual, Timers: manual.Timers(), IDs: sequence(),
		Log: slog.New(slog.DiscardHandler)})
	if err == nil {
		_ = c.Stop(context.Background())
		t.Fatal("Start succeeded on a data directory that is a file")
	}
	if !strings.Contains(err.Error(), env.GatewayDataDir.Name()) || !strings.Contains(err.Error(), file) {
		t.Errorf("Start = %v, want the error to name %s and the directory", err, env.GatewayDataDir.Name())
	}
}
