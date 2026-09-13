package replay_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/replay"
)

// apiError is the shape Error of api-contracts.md §1 without details, which
// the route does not send (C-01 v1.11).
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type clockAnswer struct {
	status int
	// body is nil for an answer without a body.
	body   *apiError
	header http.Header
}

// postClock decodes an error strictly: a field outside the shape Error — the
// flat {"error": code, "message": …} of the first iteration included — fails
// the test.
func postClock(t *testing.T, h http.Handler, method, body string) clockAnswer {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, replay.ClockPath, strings.NewReader(body)))
	answer := clockAnswer{status: w.Code, header: w.Header()}
	if w.Body.Len() > 0 {
		var decoded struct {
			Error *apiError `json:"error"`
		}
		dec := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&decoded); err != nil || decoded.Error == nil {
			t.Fatalf("%s %q: body %q is not {\"error\": {\"code\", \"message\"}}: %v", method, body, w.Body.String(), err)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
			t.Errorf("%s %q: Content-Type %q, want application/json; charset=utf-8", method, body, ct)
		}
		answer.body = decoded.Error
	}
	return answer
}

// C-01 v1.9, the DoD of T-458 B2: 204 moves the clock, fractional seconds and
// an offset included; a time equal to the clock is 204 as well.
func TestClockHandlerMovesTheClock(t *testing.T) {
	ec := replay.NewEventClock(t0)
	h := replay.ClockHandler(ec)

	for _, tc := range []struct {
		at   string
		want time.Time
	}{
		{"2026-09-13T10:00:00Z", t0},
		{"2026-09-13T10:00:01.25Z", t0.Add(1250 * time.Millisecond)},
		{"2026-09-13T13:05:00+03:00", t0.Add(5 * time.Minute)},
	} {
		answer := postClock(t, h, http.MethodPost, `{"at":"`+tc.at+`"}`)
		if answer.status != http.StatusNoContent || answer.body != nil {
			t.Errorf("at %s: %d %v, want 204 without a body", tc.at, answer.status, answer.body)
		}
		if got := ec.Now(); !got.Equal(tc.want) {
			t.Errorf("at %s: the clock is at %v, want %v", tc.at, got, tc.want)
		}
	}
}

// A time before the clock is 409 clock_behind and the clock stays.
func TestClockHandlerRefusesTimeGoingBack(t *testing.T) {
	ec := replay.NewEventClock(t0.Add(time.Hour))
	answer := postClock(t, replay.ClockHandler(ec), http.MethodPost, `{"at":"2026-09-13T10:00:00Z"}`)
	if answer.status != http.StatusConflict {
		t.Fatalf("status %d, want 409", answer.status)
	}
	if answer.body == nil || answer.body.Code != replay.CodeClockBehind ||
		!strings.Contains(answer.body.Message, "2026-09-13T11:00:00Z") {
		t.Errorf("body %+v, want error.code %q and a message naming the time of the clock", answer.body, replay.CodeClockBehind)
	}
	if got := ec.Now(); !got.Equal(t0.Add(time.Hour)) {
		t.Errorf("the clock moved to %v on a refused request", got)
	}
}

func TestClockHandlerRefusesABadBody(t *testing.T) {
	ec := replay.NewEventClock(t0)
	h := replay.ClockHandler(ec)
	for name, body := range map[string]string{
		"empty":           ``,
		"not JSON":        `at=2026-09-13T10:00:00Z`,
		"array":           `["2026-09-13T10:00:00Z"]`,
		"null":            `null`,
		"no at":           `{"time":"2026-09-13T11:00:00Z"}`,
		"at null":         `{"at":null}`,
		"at a number":     `{"at":1789286400}`,
		"at not RFC 3339": `{"at":"13.09.2026 11:00"}`,
		"date only":       `{"at":"2026-09-13"}`,
		"trailing data":   `{"at":"2026-09-13T11:00:00Z"} {}`,
		"too long":        `{"at":"2026-09-13T11:00:00Z","pad":"` + strings.Repeat("a", 2<<10) + `"}`,
	} {
		answer := postClock(t, h, http.MethodPost, body)
		if answer.status != http.StatusBadRequest || answer.body == nil || answer.body.Code != replay.CodeInvalidBody ||
			answer.body.Message == "" {
			t.Errorf("%s: %d %+v, want 400 with error.code %q and a message", name, answer.status, answer.body, replay.CodeInvalidBody)
		}
	}
	if got := ec.Now(); !got.Equal(t0) {
		t.Errorf("the clock moved to %v on bad requests", got)
	}
}

// The process mounts the handler with the pattern "POST "+ClockPath and the mux
// answers another method before the handler; the check of the handler guards a
// mount without a method, and answers in the same shape.
func TestClockHandlerAcceptsOnlyPOST(t *testing.T) {
	ec := replay.NewEventClock(t0)
	h := replay.ClockHandler(ec)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		answer := postClock(t, h, method, `{"at":"2026-09-13T11:00:00Z"}`)
		if answer.status != http.StatusMethodNotAllowed || answer.header.Get("Allow") != http.MethodPost ||
			answer.body == nil || answer.body.Code != replay.CodeMethodNotAllowed || answer.body.Message == "" {
			t.Errorf("%s: %d, Allow %q, body %+v; want 405 with Allow: POST and error.code %q", method, answer.status,
				answer.header.Get("Allow"), answer.body, replay.CodeMethodNotAllowed)
		}
	}
	if got := ec.Now(); !got.Equal(t0) {
		t.Errorf("the clock moved to %v on another method", got)
	}
}

func TestClockHandlerNeedsAClock(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ClockHandler(nil) did not panic: the defect would surface on the first request, not at start")
		}
	}()
	replay.ClockHandler(nil)
}
