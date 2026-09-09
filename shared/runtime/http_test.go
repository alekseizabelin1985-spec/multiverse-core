package runtime_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"multiverse-core.io/shared/runtime"
)

func TestHTTPHealth(t *testing.T) {
	tests := map[string]struct {
		status   runtime.Status
		wantCode int
	}{
		"ok":       {runtime.Status{Status: runtime.StatusOK, Details: map[string]any{"contexts": map[string]any{"state": "ok"}}}, http.StatusOK},
		"degraded": {runtime.Status{Status: runtime.StatusDegraded}, http.StatusOK},
		"fail":     {runtime.Status{Status: runtime.StatusFail}, http.StatusServiceUnavailable},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			srv := runtime.NewHTTP("127.0.0.1:0", func() runtime.Status { return tc.status })
			rec := httptest.NewRecorder()
			srv.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

			if rec.Code != tc.wantCode {
				t.Fatalf("status code = %d, want %d", rec.Code, tc.wantCode)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", rec.Body.String(), err)
			}
			if body["status"] != tc.status.Status {
				t.Fatalf("body status = %v, want %q", body["status"], tc.status.Status)
			}
		})
	}
}

func TestHTTPServesOnLoopbackAndStops(t *testing.T) {
	srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	resp, err := http.Get("http://" + srv.Addr + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := http.Get("http://" + srv.Addr + "/health"); err == nil {
		t.Fatal("server still answers after Stop")
	}
}

func TestAdminOnly(t *testing.T) {
	// Admission is by client id; the actor kind is only validated against the
	// envelope enum (ADR-009 p. 9, C-01).
	tests := map[string]struct {
		clients  string // MV_CORE_ADMIN_CLIENTS, empty = unset
		clientID string
		kind     string
		wantCode int
	}{
		"operator by default":    {"", "operator", "human", http.StatusNoContent},
		"operator without kind":  {"", "operator", "", http.StatusNoContent},
		"ci client from list":    {"operator,ci-harness", "ci-harness", "ci", http.StatusNoContent},
		"foreign client":         {"", "telegram-bot", "human", http.StatusForbidden},
		"no client header":       {"", "", "human", http.StatusForbidden},
		"no headers at all":      {"", "", "", http.StatusForbidden},
		"empty client value":     {"", " ", "human", http.StatusForbidden},
		"unknown actor kind":     {"", "operator", "operator", http.StatusForbidden},
		"actor kind wrong case":  {"", "operator", "Human", http.StatusForbidden},
		"client dropped by list": {"ci-harness", "operator", "human", http.StatusForbidden},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.clients != "" {
				t.Setenv(runtime.EnvAdminClients, tc.clients)
			}
			handler := runtime.AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, "/v1/admin/agents", nil)
			if tc.clientID != "" {
				req.Header.Set(runtime.ClientIDHeader, tc.clientID)
			}
			if tc.kind != "" {
				req.Header.Set(runtime.ActorKindHeader, tc.kind)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

// Emptying the allow-list closes /v1/admin/*, it does not reopen it: the
// operator who writes MV_CORE_ADMIN_CLIENTS= means nobody, and an empty value
// therefore beats the shipped default (review T-007 Mi-6).
func TestAdminOnlyAdmitsNobodyWhenTheAllowListIsEmptied(t *testing.T) {
	t.Setenv(runtime.EnvAdminClients, "")
	handler := runtime.AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, clientID := range []string{"operator", "ci-harness", ""} {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/agents", nil)
		if clientID != "" {
			req.Header.Set(runtime.ClientIDHeader, clientID)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("client %q = %d, want %d", clientID, rec.Code, http.StatusForbidden)
		}
	}
}

func TestAdminOnlyReportsReason(t *testing.T) {
	handler := runtime.AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/admin/agents", nil))

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	if !strings.Contains(body["error"], runtime.ClientIDHeader) {
		t.Fatalf("error = %q, want it to name %s", body["error"], runtime.ClientIDHeader)
	}
}
