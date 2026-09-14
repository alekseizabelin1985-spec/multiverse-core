package state_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/runtime"
)

// The body of the snapshot route names the reason (T-475): bootstrap for the
// snapshot seq 0 mvctl world init asks for, admin without a body or a reason.
// A body that is not a SnapshotRequest, or a reason State does not take from
// outside, is refused 400 and writes nothing.
func TestTheAdminRouteTakesTheReasonOfTheRequest(t *testing.T) {
	s := newStored(t)
	c, _ := s.start(t)
	s.seededWorld(t)
	mux := http.NewServeMux()
	c.Routes(mux)
	path := "/v1/admin/state/" + world + "/snapshot"
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set(runtime.ClientIDHeader, "ci-harness")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	for name, tc := range map[string]struct {
		body, code string
	}{
		"a reason of State's own trigger": {`{"reason": "shutdown"}`, "invalid_reason"},
		"an unknown reason":               {`{"reason": "whim"}`, "invalid_reason"},
		"a misspelled field":              {`{"reson": "bootstrap"}`, "invalid_body"},
		"not JSON":                        {`reason=bootstrap`, "invalid_body"},
		"two documents":                   {`{"reason": "bootstrap"} {}`, "invalid_body"},
		"a reason of another type":        {`{"reason": 1}`, "invalid_body"},
		// The body is read up to 4 KB: a longer one is cut and does not decode.
		"a body past its limit": {`{"reason": "admin"` + strings.Repeat(" ", 5<<10) + `}`, "invalid_body"},
	} {
		got := post(tc.body)
		if got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%s: %d %s, want 400 %s", name, got.Code, got.Body, tc.code)
		}
	}
	if steps := s.objects.timeline.matching("put state/"); len(steps) != 0 {
		t.Fatalf("snapshot writes %q of refused requests", steps)
	}

	// The world has no latest.json yet: the bootstrap is the first snapshot.
	got := post(`{"reason": "bootstrap"}`)
	var first state.LatestPointer
	if err := json.Unmarshal(got.Body.Bytes(), &first); got.Code != http.StatusOK || err != nil ||
		first.Snapshot.Reason != state.SnapshotBootstrap || first.Snapshot.Seq != 0 {
		t.Fatalf("bootstrap: %d %s, want 200 with the snapshot seq 0 of reason bootstrap", got.Code, got.Body)
	}

	for _, body := range []string{`{"reason": "admin"}`, `{"reason": ""}`, `{}`, ``} {
		got := post(body)
		var pointer state.LatestPointer
		if err := json.Unmarshal(got.Body.Bytes(), &pointer); got.Code != http.StatusOK || err != nil ||
			pointer.Snapshot.Reason != state.SnapshotAdmin {
			t.Errorf("body %q: %d %s, want 200 with a snapshot of reason admin", body, got.Code, got.Body)
		}
	}

	// (e) Now the world has latest.json: a second bootstrap is 409
	// world_initialized and writes nothing.
	writes := len(s.objects.timeline.matching("put state/"))
	got = post(`{"reason": "bootstrap"}`)
	if got.Code != http.StatusConflict || !strings.Contains(got.Body.String(), `"code":"world_initialized"`) {
		t.Errorf("bootstrap of an initialized world: %d %s, want 409 world_initialized", got.Code, got.Body)
	}
	if after := len(s.objects.timeline.matching("put state/")); after != writes {
		t.Errorf("the refused bootstrap wrote %d objects of state/", after-writes)
	}
}
