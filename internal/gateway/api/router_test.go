package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/api"
)

func handlerSaying(s string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, s+" "+r.PathValue("player_id"))
	})
}

func TestRouterMountsRoutesAsMethodPatterns(t *testing.T) {
	r := api.NewRouter()
	r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", handlerSaying("get"))
	r.Handle("postAction", http.MethodPost, "/v1/players/{player_id}/actions", handlerSaying("act"))
	mux := http.NewServeMux()
	r.Mount(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, tc := range []struct{ method, path, want string }{
		{http.MethodGet, "/v1/players/player-A", "get player-A"},
		{http.MethodPost, "/v1/players/player-A/actions", "act player-A"},
	} {
		req, _ := http.NewRequest(tc.method, srv.URL+tc.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || string(body) != tc.want {
			t.Errorf("%s %s = %d %q, want 200 %q", tc.method, tc.path, resp.StatusCode, body, tc.want)
		}
	}
	resp, err := http.Post(srv.URL+"/v1/players/player-A", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST on a GET route = %d, want 405", resp.StatusCode)
	}

	routes := r.Routes()
	if len(routes) != 2 || routes[0].OperationID != "getPlayer" || routes[1].OperationID != "postAction" {
		t.Errorf("Routes() = %+v, want registration order", routes)
	}
	routes[0].OperationID = "changed"
	if r.Routes()[0].OperationID != "getPlayer" {
		t.Error("Routes exposes the table itself")
	}
}

func TestRouterRejectsConflictingRoutes(t *testing.T) {
	h := handlerSaying("x")
	for _, tc := range []struct {
		name  string
		setup func(r *api.Router)
		want  string
	}{
		{"repeated operationId", func(r *api.Router) {
			r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", h)
			r.Handle("getPlayer", http.MethodGet, "/v1/other", h)
		}, "operationId"},
		{"repeated method and path", func(r *api.Router) {
			r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", h)
			r.Handle("getPlayer2", http.MethodGet, "/v1/players/{player_id}", h)
		}, "registered twice"},
		{"missing handler", func(r *api.Router) {
			r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", nil)
		}, "required"},
		{"missing operationId", func(r *api.Router) {
			r.Handle("", http.MethodGet, "/v1/players/{player_id}", h)
		}, "required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, tc.want) {
					t.Errorf("panic %q, want it to contain %q", msg, tc.want)
				}
			}()
			tc.setup(api.NewRouter())
		})
	}
}

func TestSameMethodDifferentPathIsNotAConflict(t *testing.T) {
	r := api.NewRouter()
	r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", handlerSaying("a"))
	r.Handle("getGroup", http.MethodGet, "/v1/groups/{group_id}", handlerSaying("b"))
	if len(r.Routes()) != 2 {
		t.Errorf("Routes() = %d, want 2", len(r.Routes()))
	}
}
