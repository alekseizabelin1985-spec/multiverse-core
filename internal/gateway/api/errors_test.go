package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/api"
)

func TestNewErrorTakesStatusAndMessageFromTheTable(t *testing.T) {
	e := api.NewError(api.CodeNoLeader, map[string]any{"group_id": "g-1"})
	spec, _ := api.LookupError(api.CodeNoLeader)
	if e.Status != http.StatusConflict || e.Code != api.CodeNoLeader || e.Message != spec.Message || e.Details["group_id"] != "g-1" {
		t.Errorf("NewError(no_leader) = %+v", e)
	}
	if got := e.Error(); !strings.Contains(got, "409") || !strings.Contains(got, api.CodeNoLeader) {
		t.Errorf("Error() = %q, want status and code", got)
	}
}

func TestNewErrorPanicsOnACodeOutsideTheTable(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), "no_such_code") {
			t.Errorf("recover() = %v, want a panic naming the code", r)
		}
	}()
	_ = api.NewError("no_such_code", nil)
}

func TestLookupErrorOfUnknownCode(t *testing.T) {
	if spec, ok := api.LookupError("no_such_code"); ok {
		t.Errorf("LookupError(no_such_code) = %+v, true", spec)
	}
}

func TestErrorSpecsReturnsACopy(t *testing.T) {
	specs := api.ErrorSpecs()
	specs[0].Status = 999
	if api.ErrorSpecs()[0].Status == 999 {
		t.Error("ErrorSpecs exposes the table itself")
	}
}

func TestWriteErrorWritesTheContractBody(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set(api.HeaderRetryAfter, "7")
	if err := api.WriteError(rec, api.NewError(api.CodeRateLimited, nil)); err != nil {
		t.Fatalf("WriteError: %v", err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != api.ContentTypeJSON {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get(api.HeaderRetryAfter); got != "7" {
		t.Errorf("Retry-After set by the caller = %q, want 7", got)
	}
	var raw map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	body := raw["error"]
	if body["code"] != api.CodeRateLimited || body["message"] == "" {
		t.Errorf("body = %v", raw)
	}
	if _, ok := body["details"]; ok {
		t.Errorf("details without values must be omitted, body = %v", raw)
	}
}

// A group without a living member has leader_id null on the wire, not a
// missing field and not an empty string (C-08 v1.2).
func TestGroupWithoutLeaderEncodesLeaderIDAsNull(t *testing.T) {
	data, err := json.Marshal(api.GroupView{GroupID: "g-1", Members: []api.GroupMember{}})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if v, ok := m["leader_id"]; !ok || v != nil {
		t.Errorf("leader_id = %v (present %v), want null; body %s", v, ok, data)
	}
}
