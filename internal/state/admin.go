package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"multiverse-core.io/shared/runtime"
)

// SnapshotRoute is the admin route that writes a snapshot of a world now
// (state-and-mechanics.md §4.9, §4.10): the harness of S3 and mvctl world init
// with --bus kafka ask for one. The method is part of the pattern: the mux of
// the process is shared, and a pattern without a method would overlap with
// "POST /v1/admin/{path...}" of another context (C-01 v1.11).
const SnapshotRoute = http.MethodPost + " /v1/admin/state/{world}/snapshot"

// SnapshotRequest is the body of the snapshot route. It is optional: no body,
// or no reason, is a snapshot with reason admin. mvctl world init asks for
// bootstrap, the reason of the snapshot seq 0 of a world (§4.4, §4.10 p. 4);
// the reasons of State's own triggers are not asked for from outside.
type SnapshotRequest struct {
	Reason string `json:"reason,omitempty"`
}

// snapshotRequestLimit bounds the body the route reads: the request is a
// reason, not a document.
const snapshotRequestLimit = 1 << 12

// Routes mounts the admin route of State on the mux of the process, behind
// runtime.AdminOnly (ADR-009 p. 9, C-06). The process calls it once, before
// Start.
func (c *Context) Routes(mux *http.ServeMux) {
	mux.Handle(SnapshotRoute, runtime.AdminOnly(http.HandlerFunc(c.serveSnapshot)))
}

// serveSnapshot writes the snapshot with the reason of the request (admin
// unless it asks for bootstrap) on the worker of the world and answers 200 with
// latest.json as written. The errors have the form of the HTTP API of the
// platform, {"error": {"code", "message"}}:
//
//   - 400 invalid_body — a body that is not a SnapshotRequest; 400
//     invalid_reason — a reason other than admin and bootstrap. Nothing is
//     written;
//   - 404 unknown_world — not a world this State serves;
//   - 409 no_object_store — a State that keeps its worlds in memory only;
//   - 409 world_initialized — reason bootstrap for a world that has
//     latest.json, readable or not (ErrWorldInitialized). Nothing is written;
//   - 503 not_running — the context is not running, or the world is stopped
//     (world_stopped) and writes no snapshot (§4.9);
//   - 500 snapshot_failed — the store refused the snapshot; the pointer stays
//     at the snapshot before.
//
// A snapshot written whose snapshot.created did not go out is 200: the
// snapshot is there and its readers read the pointer (§9); /health says
// snapshot_event_failed.
func (c *Context) serveSnapshot(w http.ResponseWriter, r *http.Request) {
	reason, code, err := snapshotReason(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, code, err)
		return
	}
	pointer, err := c.Snapshot(r.Context(), r.PathValue("world"), reason)
	switch {
	case pointer != nil:
		writeJSON(w, http.StatusOK, pointer)
	case errors.Is(err, ErrUnknownWorld):
		writeError(w, http.StatusNotFound, "unknown_world", err)
	case errors.Is(err, ErrNoObjectStore):
		writeError(w, http.StatusConflict, "no_object_store", err)
	case errors.Is(err, ErrWorldInitialized):
		writeError(w, http.StatusConflict, "world_initialized", err)
	case errors.Is(err, ErrNotRunning):
		writeError(w, http.StatusServiceUnavailable, "not_running", err)
	case errors.Is(err, ErrWorldStopped):
		writeError(w, http.StatusServiceUnavailable, "world_stopped", err)
	default:
		writeError(w, http.StatusInternalServerError, "snapshot_failed", err)
	}
}

// snapshotReason is the reason the request asks for, or the code and the
// error of a request that is refused. An unknown field is refused rather than
// ignored: a misspelled reason would otherwise write a snapshot with reason
// admin where bootstrap was meant.
func snapshotReason(r *http.Request) (reason, code string, err error) {
	dec := json.NewDecoder(io.LimitReader(r.Body, snapshotRequestLimit))
	dec.DisallowUnknownFields()
	var req SnapshotRequest
	switch err := dec.Decode(&req); {
	case errors.Is(err, io.EOF):
		return SnapshotAdmin, "", nil
	case err != nil:
		return "", "invalid_body", fmt.Errorf("state: the body is not {\"reason\": \"admin|bootstrap\"}: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return "", "invalid_body", errors.New("state: the body holds more than one JSON document")
	}
	switch req.Reason {
	case "":
		return SnapshotAdmin, "", nil
	case SnapshotAdmin, SnapshotBootstrap:
		return req.Reason, "", nil
	default:
		return "", "invalid_reason", fmt.Errorf("state: reason %q, expected %s or %s", req.Reason, SnapshotAdmin, SnapshotBootstrap)
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
