package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClockPath is the route of the replay clock (C-01 v1.9). cmd/multiverse
// mounts ClockHandler there with the pattern "POST "+ClockPath, behind
// runtime.AdminOnly, in --mode=replay only (C-01 v1.11).
const ClockPath = "/v1/admin/replay/clock"

// Codes in error.code of the answers of ClockHandler.
const (
	CodeClockBehind      = "clock_behind"
	CodeInvalidBody      = "invalid_body"
	CodeMethodNotAllowed = "method_not_allowed"
)

// maxClockBody bounds the body of a request: {"at": "<RFC 3339>"} is a few
// dozen bytes, and the route has no reason to read a megabyte to learn that.
const maxClockBody = 1 << 10

// ClockHandler serves POST ClockPath with the body {"at": "<RFC 3339>"}: it
// moves ec to at by EventClock.Advance, so that the next root event gets the
// time of the next input of a scenario (C-01 v1.9, "Время корневых событий в
// replay"). Fractional seconds are accepted.
//
// Answers, the errors in the shape Error of the HTTP API of the platform
// (api-contracts.md §1, C-01 v1.11):
//   - 204 — at is not before the clock;
//   - 409 {"error": {"code": "clock_behind", "message": …}} — at is before the
//     clock, which stays where it was;
//   - 400 {"error": {"code": "invalid_body", "message": …}} — the body is not a
//     JSON object with a string at in RFC 3339;
//   - 405 {"error": {"code": "method_not_allowed", …}} with Allow: POST —
//     another method. Mounted with the pattern "POST "+ClockPath, the handler
//     never sees one: the mux of the process answers 405 itself. The check
//     guards a mount without a method.
//
// Admission (403) is the business of runtime.AdminOnly around the handler.
func ClockHandler(ec *EventClock) http.Handler {
	if ec == nil {
		panic("replay: ClockHandler needs an EventClock")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, r.Method+" is not allowed, use POST")
			return
		}
		at, err := readAt(http.MaxBytesReader(w, r.Body, maxClockBody))
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeInvalidBody, err.Error())
			return
		}
		// ErrClockBehind is the only error of Advance.
		if err := ec.Advance(at); err != nil {
			writeError(w, http.StatusConflict, CodeClockBehind, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func readAt(body io.Reader) (time.Time, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return time.Time{}, fmt.Errorf("read body: %w", err)
	}
	var req struct {
		At *string `json:"at"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return time.Time{}, fmt.Errorf("body is not a JSON object {\"at\": \"<RFC 3339>\"}: %w", err)
	}
	if req.At == nil {
		return time.Time{}, errors.New("body has no \"at\"")
	}
	at, err := time.Parse(time.RFC3339Nano, *req.At)
	if err != nil {
		return time.Time{}, fmt.Errorf("\"at\" is not RFC 3339: %w", err)
	}
	return at, nil
}

// errorBody is the shape Error: a machine code a client branches on, and a
// message in English for the log of the harness.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: errorDetail{Code: code, Message: message}})
}
