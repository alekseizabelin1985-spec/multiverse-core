package api

import (
	"fmt"
	"net/http"
	"slices"
)

// Route is one operation of api/gateway.openapi.yaml served by the gateway.
// Path uses the ServeMux wildcard syntax of Go 1.22, which is the same {name}
// syntax as OpenAPI, so a route and its spec entry compare as plain strings.
type Route struct {
	OperationID string
	Method      string
	Path        string
	Handler     http.Handler
}

// Router is the route table of the gateway. It owns no server: the process
// HTTP server of shared/runtime serves the mux the routes are mounted on
// (C-01, component §5.1). openapi_test.go compares Routes with the spec.
type Router struct {
	routes []Route
}

// NewRouter returns an empty route table.
func NewRouter() *Router { return &Router{} }

// Handle adds a route. Like ServeMux registration, a malformed or conflicting
// route is a programming error found at start, so it panics: an empty field, a
// repeated operationId or a repeated method and path.
func (r *Router) Handle(operationID, method, path string, h http.Handler) {
	if operationID == "" || method == "" || path == "" || h == nil {
		panic(fmt.Sprintf("api: route %q %s %s: operationId, method, path and handler are required", operationID, method, path))
	}
	for _, rt := range r.routes {
		if rt.OperationID == operationID {
			panic(fmt.Sprintf("api: operationId %q registered twice", operationID))
		}
		if rt.Method == method && rt.Path == path {
			panic(fmt.Sprintf("api: route %s %s registered twice", method, path))
		}
	}
	r.routes = append(r.routes, Route{OperationID: operationID, Method: method, Path: path, Handler: h})
}

// Handlers are the handlers of the gateway operations, one field per
// operationId: the field is the operationId with its first letter in upper
// case (ResolveLink serves resolveLink). Later tasks add their fields (T-305
// actions, T-306 characters, T-307 deliveries, T-352 groups, T-354 rounds,
// T-356 service routes and the admin proxy). openapi_test.go fills every field
// and checks that GatewayRouter mounts each of them under its operationId, as
// the spec says.
type Handlers struct {
	ResolveLink http.Handler
	ConsentLink http.Handler
	ForgetLink  http.Handler
	PostAction  http.Handler
}

// GatewayRouter is the route table of the gateway context: the one
// constructor openapi_test.go compares with api/gateway.openapi.yaml. A task
// that adds a handler adds its field to Handlers, its Handle call here and
// removes its operation from notYetMounted in the test.
func GatewayRouter(h Handlers) *Router {
	r := NewRouter()
	r.Handle("resolveLink", http.MethodPost, "/v1/links/resolve", h.ResolveLink)
	r.Handle("consentLink", http.MethodPost, "/v1/links/consent", h.ConsentLink)
	r.Handle("forgetLink", http.MethodDelete, "/v1/links", h.ForgetLink)
	r.Handle("postAction", http.MethodPost, "/v1/players/{player_id}/actions", h.PostAction)
	return r
}

// Routes returns the routes in registration order.
func (r *Router) Routes() []Route { return slices.Clone(r.routes) }

// Mount registers every route on mux as a "METHOD path" pattern, its handler
// wrapped in mw; the first middleware is the outermost.
func (r *Router) Mount(mux *http.ServeMux, mw ...Middleware) {
	for _, rt := range r.routes {
		h := rt.Handler
		for i := len(mw) - 1; i >= 0; i-- {
			h = mw[i](rt, h)
		}
		mux.Handle(rt.Method+" "+rt.Path, h)
	}
}
