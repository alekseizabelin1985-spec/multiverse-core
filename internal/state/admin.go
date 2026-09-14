package state

import (
	"encoding/json"
	"errors"
	"net/http"

	"multiverse-core.io/shared/runtime"
)

// SnapshotRoute is the admin route that writes a snapshot of a world now
// (state-and-mechanics.md §4.9, §4.10): the harness of S3 and mvctl world init
// with --bus kafka ask for one. The method is part of the pattern: the mux of
// the process is shared, and a pattern without a method would overlap with
// "POST /v1/admin/{path...}" of another context (C-01 v1.11).
const SnapshotRoute = http.MethodPost + " /v1/admin/state/{world}/snapshot"

// Routes mounts the admin route of State on the mux of the process, behind
// runtime.AdminOnly (ADR-009 p. 9, C-06). The process calls it once, before
// Start.
func (c *Context) Routes(mux *http.ServeMux) {
	mux.Handle(SnapshotRoute, runtime.AdminOnly(http.HandlerFunc(c.serveSnapshot)))
}

// serveSnapshot writes the snapshot with reason admin on the worker of the
// world and answers 200 with latest.json as written. The errors have the form
// of the HTTP API of the platform, {"error": {"code", "message"}}:
//
//   - 404 unknown_world — not a world this State serves;
//   - 409 no_object_store — a State that keeps its worlds in memory only;
//   - 503 not_running — the context is not running, or the world is stopped
//     (world_stopped) and writes no snapshot (§4.9);
//   - 500 snapshot_failed — the store refused the snapshot; the pointer stays
//     at the snapshot before.
//
// A snapshot written whose snapshot.created did not go out is 200: the
// snapshot is there and its readers read the pointer (§9); /health says
// snapshot_event_failed.
func (c *Context) serveSnapshot(w http.ResponseWriter, r *http.Request) {
	pointer, err := c.Snapshot(r.Context(), r.PathValue("world"), SnapshotAdmin)
	switch {
	case pointer != nil:
		writeJSON(w, http.StatusOK, pointer)
	case errors.Is(err, ErrUnknownWorld):
		writeError(w, http.StatusNotFound, "unknown_world", err)
	case errors.Is(err, ErrNoObjectStore):
		writeError(w, http.StatusConflict, "no_object_store", err)
	case errors.Is(err, ErrNotRunning):
		writeError(w, http.StatusServiceUnavailable, "not_running", err)
	case errors.Is(err, ErrWorldStopped):
		writeError(w, http.StatusServiceUnavailable, "world_stopped", err)
	default:
		writeError(w, http.StatusInternalServerError, "snapshot_failed", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": err.Error()}})
}

var _ runtime.Routes = (*Context)(nil)
