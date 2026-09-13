package api_test

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/schemas"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/runtime"
)

// specPath is api/gateway.openapi.yaml seen from this package directory.
var specPath = filepath.Join("..", "..", "..", "api", "gateway.openapi.yaml")

func loadSpec(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", specPath, err)
	}
	return doc
}

// at walks nested mappings and fails the test when a key is missing.
func at(t *testing.T, node any, keys ...string) any {
	t.Helper()
	cur := node
	for i, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%s: not a mapping", strings.Join(keys[:i], "."))
		}
		if cur, ok = m[k]; !ok {
			t.Fatalf("%s: missing", strings.Join(keys[:i+1], "."))
		}
	}
	return cur
}

func mapping(t *testing.T, node any, keys ...string) map[string]any {
	t.Helper()
	m, ok := at(t, node, keys...).(map[string]any)
	if !ok {
		t.Fatalf("%s: not a mapping", strings.Join(keys, "."))
	}
	return m
}

func stringList(t *testing.T, node any, keys ...string) []string {
	t.Helper()
	list, ok := at(t, node, keys...).([]any)
	if !ok {
		t.Fatalf("%s: not a list", strings.Join(keys, "."))
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("%s: %v is not a string", strings.Join(keys, "."), v)
		}
		out = append(out, s)
	}
	return out
}

var httpMethods = []string{"get", "put", "post", "delete", "patch", "head", "options", "trace"}

type operation struct {
	Method, Path, OperationID string
	Node                      map[string]any
}

func operations(t *testing.T, doc map[string]any) []operation {
	t.Helper()
	var ops []operation
	for path, item := range mapping(t, doc, "paths") {
		for _, m := range httpMethods {
			node, ok := item.(map[string]any)[m]
			if !ok {
				continue
			}
			op := node.(map[string]any)
			id, _ := op["operationId"].(string)
			ops = append(ops, operation{Method: strings.ToUpper(m), Path: path, OperationID: id, Node: op})
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].OperationID < ops[j].OperationID })
	return ops
}

func TestOpenAPIHeaderIsTheSkeletonOfC08(t *testing.T) {
	doc := loadSpec(t)
	if got := at(t, doc, "openapi"); got != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", got)
	}
	if got := at(t, doc, "info", "version"); got != "1.0.0" {
		t.Errorf("info.version = %v, want 1.0.0", got)
	}
	scheme := mapping(t, doc, "components", "securitySchemes", "ClientId")
	if scheme["type"] != "apiKey" || scheme["in"] != "header" || scheme["name"] != api.HeaderClientID {
		t.Errorf("securitySchemes.ClientId = %v, want apiKey in header %s", scheme, api.HeaderClientID)
	}
	if got := at(t, doc, "components", "parameters", "ActorKind", "name"); got != api.HeaderActorKind {
		t.Errorf("parameters.ActorKind.name = %v, want %s", got, api.HeaderActorKind)
	}
}

// The operations of component §5.6 (operationId = handler name) with the
// admin proxy of C-06, the gateway's own service routes and /health of the
// process.
var wantOperations = []operation{
	{Method: "POST", Path: "/v1/links/resolve", OperationID: "resolveLink"},
	{Method: "POST", Path: "/v1/links/consent", OperationID: "consentLink"},
	{Method: "DELETE", Path: "/v1/links", OperationID: "forgetLink"},
	{Method: "DELETE", Path: "/v1/admin/links/{player_id}", OperationID: "adminForgetLink"},
	{Method: "GET", Path: "/v1/worlds", OperationID: "listWorlds"},
	{Method: "POST", Path: "/v1/characters", OperationID: "createCharacter"},
	{Method: "GET", Path: "/v1/players/{player_id}", OperationID: "getPlayer"},
	{Method: "POST", Path: "/v1/players/{player_id}/actions", OperationID: "postAction"},
	{Method: "GET", Path: "/v1/groups/{group_id}", OperationID: "getGroup"},
	{Method: "GET", Path: "/v1/clients/{client_id}/deliveries", OperationID: "pollDeliveries"},
	{Method: "POST", Path: "/v1/clients/{client_id}/deliveries/ack", OperationID: "ackDeliveries"},
	{Method: "GET", Path: "/v1/clients/{client_id}/stream", OperationID: "streamDeliveries"},
	{Method: "POST", Path: "/v1/scopes/{scope_id}/rounds/close", OperationID: "closeRound"},
	{Method: "POST", Path: "/v1/admin/agents/{agent_id}/tick", OperationID: "adminTick"},
	{Method: "GET", Path: "/v1/admin/agents", OperationID: "adminAgents"},
	{Method: "GET", Path: "/v1/admin/llm/usage", OperationID: "adminLlmUsage"},
	{Method: "GET", Path: "/v1/admin/sessions", OperationID: "adminSessions"},
	{Method: "GET", Path: "/health", OperationID: "health"},
}

func key(op operation) string { return op.Method + " " + op.Path + " " + op.OperationID }

func TestOpenAPIOperationsAreThoseOfComponent56(t *testing.T) {
	doc := loadSpec(t)
	ops := operations(t, doc)

	seen := map[string]bool{}
	var got []string
	for _, op := range ops {
		if op.OperationID == "" {
			t.Errorf("%s %s has no operationId", op.Method, op.Path)
		}
		if seen[op.OperationID] {
			t.Errorf("operationId %q is used twice", op.OperationID)
		}
		seen[op.OperationID] = true
		got = append(got, key(op))
	}
	var want []string
	for _, op := range wantOperations {
		want = append(want, key(op))
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("operations of the spec:\n got %q\nwant %q", got, want)
	}

	byID := map[string]operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}
	for _, id := range []string{"createCharacter", "postAction"} {
		if got := byID[id].Node["x-idempotency"]; got != "action_key" {
			t.Errorf("%s: x-idempotency = %v, want action_key", id, got)
		}
	}
	for _, id := range []string{"adminForgetLink", "closeRound", "adminSessions", "adminTick", "adminAgents", "adminLlmUsage"} {
		if got := stringList(t, byID[id].Node, "x-actor-kind"); !slices.Equal(got, []string{api.ActorCI}) {
			t.Errorf("%s: x-actor-kind = %v, want [ci]", id, got)
		}
	}
}

// The admin section belongs to EPIC-003 (C-06): reserved operations and only
// they carry the tag admin, and all of them name the task that writes them.
func TestOpenAPIAdminSectionIsReservedForT239(t *testing.T) {
	doc := loadSpec(t)
	var adminTag map[string]any
	for _, tag := range at(t, doc, "tags").([]any) {
		if m := tag.(map[string]any); m["name"] == "admin" {
			adminTag = m
		}
	}
	if adminTag == nil {
		t.Fatal("tag admin is missing")
	}
	if d, _ := adminTag["description"].(string); !strings.Contains(d, "Reserved") || !strings.Contains(d, "T-239") {
		t.Errorf("tag admin description = %q, want it to say Reserved and name T-239", d)
	}
	var reserved []string
	for _, op := range operations(t, doc) {
		tags := stringList(t, op.Node, "tags")
		_, isReserved := op.Node["x-reserved"]
		if slices.Contains(tags, "admin") != isReserved {
			t.Errorf("%s: tag admin %v, x-reserved %v; they go together", op.OperationID, slices.Contains(tags, "admin"), isReserved)
		}
		if !isReserved {
			continue
		}
		reserved = append(reserved, op.OperationID)
		if got := op.Node["x-reserved"]; got != "EPIC-003 T-239" {
			t.Errorf("%s: x-reserved = %v, want EPIC-003 T-239", op.OperationID, got)
		}
		if !strings.HasPrefix(op.Path, "/v1/admin/") {
			t.Errorf("%s: reserved path %s is outside /v1/admin/", op.OperationID, op.Path)
		}
	}
	slices.Sort(reserved)
	if want := []string{"adminAgents", "adminLlmUsage", "adminTick"}; !slices.Equal(reserved, want) {
		t.Errorf("reserved operations = %v, want %v", reserved, want)
	}
}

// notYetMounted names the operations the gateway does not serve yet and the
// task that mounts each of them (tasks.md). A task that mounts handlers adds
// their fields to api.Handlers, registers them in api.GatewayRouter and deletes
// its lines here; the test fails while a mounted operation is still listed, so
// the list can only shrink, and when it is empty the route table equals the
// spec.
var notYetMounted = map[string]string{
	"postAction":       "T-305",
	"listWorlds":       "T-306",
	"createCharacter":  "T-306",
	"getPlayer":        "T-306",
	"pollDeliveries":   "T-307",
	"ackDeliveries":    "T-307",
	"streamDeliveries": "T-307",
	"getGroup":         "T-352",
	"closeRound":       "T-354",
	"adminForgetLink":  "T-356",
	"adminSessions":    "T-356",
	"adminTick":        "T-356",
	"adminAgents":      "T-356",
	"adminLlmUsage":    "T-356",
}

// servedByProcess are operations of the process HTTP server (shared/runtime):
// the gateway must never mount them, the mux would panic on the duplicate.
var servedByProcess = []string{"health"}

// namedHandler is a stub whose identity tells which Handlers field it came from.
type namedHandler string

func (namedHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

var handlerType = reflect.TypeFor[http.Handler]()

// allHandlers fills every field of api.Handlers with a stub named after the
// field, so the test needs no change when a task adds a handler.
func allHandlers(t *testing.T) (api.Handlers, []string) {
	t.Helper()
	var h api.Handlers
	v := reflect.ValueOf(&h).Elem()
	var fields []string
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Type != handlerType {
			t.Errorf("api.Handlers.%s is %s; every field is an http.Handler named after its operation", f.Name, f.Type)
			continue
		}
		v.Field(i).Set(reflect.ValueOf(namedHandler(f.Name)))
		fields = append(fields, f.Name)
	}
	return h, fields
}

func operationOfField(field string) string {
	return strings.ToLower(field[:1]) + field[1:]
}

func TestOpenAPIRoutesMatchRouter(t *testing.T) {
	doc := loadSpec(t)
	spec := map[string]operation{}
	for _, op := range operations(t, doc) {
		spec[op.OperationID] = op
	}
	handlers, fields := allHandlers(t)

	mounted := map[string]bool{}
	for _, rt := range api.GatewayRouter(handlers).Routes() {
		mounted[rt.OperationID] = true
		op, ok := spec[rt.OperationID]
		switch {
		case !ok:
			t.Errorf("route %s %s (%s) is not in the spec", rt.Method, rt.Path, rt.OperationID)
		case op.Method != rt.Method || op.Path != rt.Path:
			t.Errorf("route %s is %s %s, the spec says %s %s", rt.OperationID, rt.Method, rt.Path, op.Method, op.Path)
		}
		if name, ok := rt.Handler.(namedHandler); !ok || operationOfField(string(name)) != rt.OperationID {
			t.Errorf("route %s serves %v, want the handler api.Handlers.%s", rt.OperationID, rt.Handler,
				strings.ToUpper(rt.OperationID[:1])+rt.OperationID[1:])
		}
		if task, listed := notYetMounted[rt.OperationID]; listed {
			t.Errorf("route %s is mounted: %s removes it from notYetMounted", rt.OperationID, task)
		}
		if slices.Contains(servedByProcess, rt.OperationID) {
			t.Errorf("route %s belongs to the process HTTP server and must not be mounted by the gateway", rt.OperationID)
		}
	}
	for _, field := range fields {
		if id := operationOfField(field); !mounted[id] {
			t.Errorf("api.Handlers.%s is not mounted by api.GatewayRouter as %s", field, id)
		}
	}
	for id := range spec {
		_, listed := notYetMounted[id]
		if !mounted[id] && !listed && !slices.Contains(servedByProcess, id) {
			t.Errorf("operation %s of the spec is neither mounted nor listed as not yet mounted", id)
		}
	}
	for id, task := range notYetMounted {
		if _, ok := spec[id]; !ok {
			t.Errorf("%s (%s) is listed in notYetMounted but is not in the spec", id, task)
		}
		if !regexp.MustCompile(`^T-3\d\d$`).MatchString(task) {
			t.Errorf("%s: %q is not a task of EPIC-004", id, task)
		}
	}
	for _, id := range servedByProcess {
		if _, ok := spec[id]; !ok {
			t.Errorf("%s is listed in servedByProcess but is not in the spec", id)
		}
	}
}

// The operations the middleware keeps out of the log are those the spec marks
// x-nolog, and the long-poll the middleware exempts from the request timeout
// is an operation of the spec.
func TestMiddlewarePoliciesNameOperationsOfTheSpec(t *testing.T) {
	doc := loadSpec(t)
	var noLog, all []string
	for _, op := range operations(t, doc) {
		all = append(all, op.OperationID)
		if op.Node["x-nolog"] == true {
			noLog = append(noLog, op.OperationID)
		}
	}
	equalSets(t, "api.NoLogOperations vs x-nolog of the spec", api.NoLogOperations(), noLog)
	for _, id := range api.LongPollOperations() {
		if !slices.Contains(all, id) {
			t.Errorf("long-poll operation %s is not in the spec", id)
		}
	}
}

// statusOfResponse names the status each shared error response travels with.
var statusOfResponse = map[string]int{
	"BadRequest":          400,
	"Forbidden":           403,
	"NotFound":            404,
	"Conflict":            409,
	"PayloadTooLarge":     413,
	"UnprocessableEntity": 422,
	"RateLimited":         429,
	"InternalError":       500,
	"NotImplemented":      501,
	"Unavailable":         503,
}

var snakeCase = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)

func TestErrorTableIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range api.ErrorSpecs() {
		if !snakeCase.MatchString(spec.Code) {
			t.Errorf("code %q is not snake_case", spec.Code)
		}
		if seen[spec.Code] {
			t.Errorf("code %q is listed twice", spec.Code)
		}
		seen[spec.Code] = true
		if spec.Status < 400 || spec.Status > 599 {
			t.Errorf("code %q has status %d, want 4xx or 5xx", spec.Code, spec.Status)
		}
		if strings.TrimSpace(spec.Message) == "" {
			t.Errorf("code %q has no message", spec.Code)
		}
	}
}

func TestOpenAPIErrorCodesMatchErrorTable(t *testing.T) {
	doc := loadSpec(t)
	responses := mapping(t, doc, "components", "responses")

	specStatus := map[string]int{}
	for name, node := range responses {
		status, known := statusOfResponse[name]
		if !known {
			t.Errorf("components.responses.%s: unknown error response, add its status to the test", name)
			continue
		}
		codes := stringList(t, node, "x-error-codes")
		desc, _ := node.(map[string]any)["description"].(string)
		for _, code := range codes {
			if prev, dup := specStatus[code]; dup {
				t.Errorf("code %q is listed under %d and %d", code, prev, status)
			}
			specStatus[code] = status
			if !strings.Contains(desc, code) {
				t.Errorf("components.responses.%s: description does not name %q", name, code)
			}
		}
		example := at(t, node, "content", "application/json", "example", "error", "code")
		if !slices.Contains(codes, example.(string)) {
			t.Errorf("components.responses.%s: example code %v is not one of %v", name, example, codes)
		}
	}
	for name := range statusOfResponse {
		if _, ok := responses[name]; !ok {
			t.Errorf("components.responses.%s is missing", name)
		}
	}

	for _, spec := range api.ErrorSpecs() {
		got, ok := specStatus[spec.Code]
		switch {
		case !ok:
			t.Errorf("code %q of errors.go is not in the spec", spec.Code)
		case got != spec.Status:
			t.Errorf("code %q: spec %d, errors.go %d", spec.Code, got, spec.Status)
		}
	}
	for code := range specStatus {
		if _, ok := api.LookupError(code); !ok {
			t.Errorf("code %q of the spec is not in errors.go", code)
		}
	}
}

// codesOfContract is the reference list of error codes: api-contracts.md §1.6
// with the additions of C-08 v1.1–v1.3, consent_incomplete (§1.2), internal
// (component §5.1) and character_alive_exists (§1.3, reserved). It is written
// out as literals on purpose. The spec and errors.go are compared with each
// other elsewhere, so a code removed from both at once passes that comparison;
// here the removal has to show up in the diff of this list.
var codesOfContract = map[string]int{
	"invalid_request":        400,
	"unknown_action":         400,
	"text_invalid":           400,
	"name_required":          400,
	"name_invalid":           400,
	"consent_incomplete":     400,
	"consent_required":       403,
	"actor_kind_forbidden":   403,
	"client_unknown":         403,
	"client_mismatch":        403,
	"player_not_found":       404,
	"world_not_found":        404,
	"unknown_target":         404,
	"character_dead":         409,
	"in_encounter":           409,
	"not_in_encounter":       409,
	"target_dead":            409,
	"no_leader":              409,
	"not_leader":             409,
	"not_in_region":          409,
	"already_acted":          409,
	"no_open_round":          409,
	"poll_in_progress":       409,
	"already_in_group":       409,
	"not_in_group":           409,
	"group_full":             409,
	"group_in_encounter":     409,
	"encounter_unavailable":  409,
	"character_alive_exists": 409,
	"payload_too_large":      413,
	"filter_error":           422,
	"rate_limited":           429,
	"internal":               500,
	"not_implemented":        501,
	"bus_unavailable":        503,
	"state_unavailable":      503,
	"forget_incomplete":      503,
}

// TestErrorTableIsTheListOfTheContract holds errors.go to the reference list
// in both directions: T-303 and later tasks extend both together.
func TestErrorTableIsTheListOfTheContract(t *testing.T) {
	table := map[string]int{}
	for _, spec := range api.ErrorSpecs() {
		table[spec.Code] = spec.Status
	}
	for code, status := range codesOfContract {
		got, ok := table[code]
		if !ok || got != status {
			t.Errorf("code %q: errors.go has status %d (present %v), the contract says %d", code, got, ok, status)
		}
	}
	for code := range table {
		if _, ok := codesOfContract[code]; !ok {
			t.Errorf("code %q of errors.go is not in the reference list of the test", code)
		}
	}
}

// Every operation response that points at a shared error response points at
// the one of its own status.
func TestOpenAPIOperationResponsesUseTheirStatus(t *testing.T) {
	doc := loadSpec(t)
	for _, op := range operations(t, doc) {
		for code, resp := range mapping(t, op.Node, "responses") {
			ref, ok := resp.(map[string]any)["$ref"].(string)
			if !ok {
				continue
			}
			name := strings.TrimPrefix(ref, "#/components/responses/")
			if want := statusOfResponse[name]; want == 0 || code != strconv.Itoa(want) {
				t.Errorf("%s: response %s refers to %s (%d)", op.OperationID, code, name, want)
			}
		}
	}
}

func TestOpenAPIReferencesResolve(t *testing.T) {
	doc := loadSpec(t)
	components := mapping(t, doc, "components")
	var walk func(path string, node any)
	walk = func(path string, node any) {
		switch v := node.(type) {
		case map[string]any:
			for k, child := range v {
				if k == "$ref" {
					ref, _ := child.(string)
					parts := strings.Split(strings.TrimPrefix(ref, "#/components/"), "/")
					if !strings.HasPrefix(ref, "#/components/") || len(parts) != 2 {
						t.Errorf("%s: $ref %q is not #/components/<kind>/<name>", path, ref)
						continue
					}
					kind, ok := components[parts[0]].(map[string]any)
					if _, found := kind[parts[1]]; !ok || !found {
						t.Errorf("%s: $ref %q does not resolve", path, ref)
					}
					continue
				}
				walk(path+"."+k, child)
			}
		case []any:
			for _, child := range v {
				walk(path+"[]", child)
			}
		}
	}
	walk("", doc)
}

// dtoOfSchema binds every components.schemas entry to its Go wire type. Health
// is the status of the process HTTP server, so it binds to runtime.Status.
var dtoOfSchema = map[string]any{
	"Error":                   api.ErrorResponse{},
	"ErrorBody":               api.ErrorBody{},
	"ResolveRequest":          api.ResolveRequest{},
	"ResolveResponse":         api.ResolveResponse{},
	"ConsentRequest":          api.ConsentRequest{},
	"ConsentResponse":         api.ConsentResponse{},
	"ForgetRequest":           api.ForgetRequest{},
	"ForgetResponse":          api.ForgetResponse{},
	"WorldsResponse":          api.WorldsResponse{},
	"WorldSummary":            api.WorldSummary{},
	"RegionSummary":           api.RegionSummary{},
	"WorldLLM":                api.WorldLLM{},
	"CreateCharacterRequest":  api.CreateCharacterRequest{},
	"CreateCharacterResponse": api.CreateCharacterResponse{},
	"CharacterState":          api.CharacterState{},
	"PositionRef":             api.PositionRef{},
	"ScopeRef":                api.ScopeRef{},
	"GroupView":               api.GroupView{},
	"GroupMember":             api.GroupMember{},
	"EncounterView":           api.EncounterView{},
	"NPCView":                 api.NPCView{},
	"Item":                    api.Item{},
	"WorldView":               api.WorldView{},
	"SessionRef":              api.SessionRef{},
	"ActionRequest":           api.ActionRequest{},
	"ActionAccepted":          api.ActionAccepted{},
	"ActionPending":           api.ActionPending{},
	"TurnRef":                 api.TurnRef{},
	"Delivery":                api.Delivery{},
	"DeliveryRoute":           api.DeliveryRoute{},
	"DeliveriesResponse":      api.DeliveriesResponse{},
	"AckRequest":              api.AckRequest{},
	"AckResponse":             api.AckResponse{},
	"RoundCloseResponse":      api.RoundCloseResponse{},
	"ClosedRound":             api.ClosedRound{},
	"AdminSessions":           api.AdminSessions{},
	"AdminSession":            api.AdminSession{},
	"Health":                  runtime.Status{},
}

type jsonField struct {
	Name      string
	Type      reflect.Type
	OmitEmpty bool
}

func jsonFields(t *testing.T, typ reflect.Type) []jsonField {
	t.Helper()
	var out []jsonField
	for i := range typ.NumField() {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			t.Errorf("%s.%s: every wire field needs an explicit json name", typ.Name(), f.Name)
			continue
		}
		out = append(out, jsonField{Name: name, Type: f.Type, OmitEmpty: strings.Contains(opts, "omitempty")})
	}
	return out
}

type shape struct {
	Types    []string
	Ref      string
	Nullable bool
	Format   string
	Items    any
}

func shapeOf(node any) shape {
	var s shape
	m, _ := node.(map[string]any)
	if ref, ok := m["$ref"].(string); ok {
		s.Ref = ref[strings.LastIndex(ref, "/")+1:]
	}
	for _, combo := range []string{"oneOf", "anyOf"} {
		branches, _ := m[combo].([]any)
		for _, b := range branches {
			bs := shapeOf(b)
			s.Types = append(s.Types, bs.Types...)
			s.Nullable = s.Nullable || bs.Nullable
			if bs.Ref != "" {
				s.Ref = bs.Ref
			}
		}
	}
	switch v := m["type"].(type) {
	case string:
		s.Types = append(s.Types, v)
	case []any:
		for _, x := range v {
			s.Types = append(s.Types, x.(string))
		}
	}
	if i := slices.Index(s.Types, "null"); i >= 0 {
		s.Types = slices.Delete(s.Types, i, i+1)
		s.Nullable = true
	}
	s.Format, _ = m["format"].(string)
	s.Items = m["items"]
	return s
}

var timeType = reflect.TypeFor[time.Time]()

func checkShape(t *testing.T, where string, gt reflect.Type, s shape, schemaOf map[reflect.Type]string) {
	t.Helper()
	for gt.Kind() == reflect.Pointer {
		gt = gt.Elem()
	}
	wantType := func(want string) {
		if !slices.Equal(s.Types, []string{want}) || s.Ref != "" {
			t.Errorf("%s: Go %s, spec type %v ref %q; want type %s", where, gt, s.Types, s.Ref, want)
		}
	}
	switch {
	case gt == timeType:
		wantType("string")
		if s.Format != "date-time" {
			t.Errorf("%s: time.Time needs format date-time, spec has %q", where, s.Format)
		}
	case gt.Kind() == reflect.String:
		wantType("string")
	case gt.Kind() == reflect.Bool:
		wantType("boolean")
	case gt.Kind() == reflect.Int || gt.Kind() == reflect.Int64:
		wantType("integer")
	case gt.Kind() == reflect.Slice:
		wantType("array")
		checkShape(t, where+"[]", gt.Elem(), shapeOf(s.Items), schemaOf)
	case gt.Kind() == reflect.Map:
		wantType("object")
	case gt.Kind() == reflect.Struct:
		if want := schemaOf[gt]; want == "" || s.Ref != want {
			t.Errorf("%s: Go %s, spec ref %q; want ref %q", where, gt, s.Ref, want)
		}
	default:
		t.Errorf("%s: Go kind %s has no rule in the test", where, gt.Kind())
	}
}

func TestDTOsMatchOpenAPISchemas(t *testing.T) {
	doc := loadSpec(t)
	schemasNode := mapping(t, doc, "components", "schemas")

	schemaOf := map[reflect.Type]string{}
	for name, dto := range dtoOfSchema {
		schemaOf[reflect.TypeOf(dto)] = name
	}
	for name := range schemasNode {
		if _, ok := dtoOfSchema[name]; !ok {
			t.Errorf("components.schemas.%s has no Go type in dtoOfSchema", name)
		}
	}

	for name, dto := range dtoOfSchema {
		node, ok := schemasNode[name].(map[string]any)
		if !ok {
			t.Errorf("components.schemas.%s is missing", name)
			continue
		}
		props, _ := node["properties"].(map[string]any)
		var required []string
		if _, has := node["required"]; has {
			required = stringList(t, node, "required")
		}
		typ := reflect.TypeOf(dto)

		var goNames, goRequired []string
		for _, f := range jsonFields(t, typ) {
			goNames = append(goNames, f.Name)
			where := name + "." + f.Name
			if !f.OmitEmpty {
				goRequired = append(goRequired, f.Name)
			}
			prop, ok := props[f.Name]
			if !ok {
				continue
			}
			s := shapeOf(prop)
			isPtr := f.Type.Kind() == reflect.Pointer
			switch {
			case isPtr && !f.OmitEmpty && !s.Nullable:
				t.Errorf("%s: pointer without omitempty encodes null, the spec must allow null", where)
			case !isPtr && s.Nullable:
				t.Errorf("%s: the spec allows null, the Go field %s never encodes it", where, f.Type)
			}
			checkShape(t, where, f.Type, s, schemaOf)
		}
		var specNames []string
		for p := range props {
			specNames = append(specNames, p)
		}
		for _, list := range [][]string{goNames, goRequired, specNames, required} {
			slices.Sort(list)
		}
		if !slices.Equal(goNames, specNames) {
			t.Errorf("%s: properties spec %v, Go %v", name, specNames, goNames)
		}
		if !slices.Equal(goRequired, required) {
			t.Errorf("%s: required spec %v, Go (fields without omitempty) %v", name, required, goRequired)
		}
	}
}

// TestRequiredArraysAreNeverNull encodes the zero value of every DTO, by value
// and by pointer, and requires [] for each array the spec declares required and
// not nullable: encoding/json writes a nil slice as null.
func TestRequiredArraysAreNeverNull(t *testing.T) {
	doc := loadSpec(t)
	schemasNode := mapping(t, doc, "components", "schemas")
	var checked []string
	for name, dto := range dtoOfSchema {
		node := mapping(t, schemasNode, name)
		props, _ := node["properties"].(map[string]any)
		var arrays []string
		if _, has := node["required"]; has {
			for _, p := range stringList(t, node, "required") {
				if s := shapeOf(props[p]); slices.Equal(s.Types, []string{"array"}) && !s.Nullable {
					arrays = append(arrays, p)
				}
			}
		}
		if len(arrays) == 0 {
			continue
		}
		zero := reflect.New(reflect.TypeOf(dto))
		for _, v := range []any{zero.Elem().Interface(), zero.Interface()} {
			data, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("%s: marshal zero value: %v", name, err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("%s: %s: %v", name, data, err)
			}
			for _, p := range arrays {
				if list, ok := m[p].([]any); !ok || len(list) != 0 {
					t.Errorf("%s (%T): required array %s encodes as %v, want []; body %s", name, v, p, m[p], data)
				}
			}
		}
		checked = append(checked, name)
	}
	want := []string{"AckRequest", "AckResponse", "AdminSessions", "DeliveriesResponse", "EncounterView", "GroupView", "WorldSummary", "WorldsResponse"}
	equalSets(t, "DTOs with a required array", checked, want)
}

// eventSchema reads a payload schema of the event contract from the embedded
// tree that shared/contracts compiles.
func eventSchema(t *testing.T, typ string) map[string]any {
	t.Helper()
	data, err := fs.ReadFile(schemas.FS, "events/"+contracts.SchemaFile(typ, 1))
	if err != nil {
		t.Fatalf("read schema of %s: %v", typ, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse schema of %s: %v", typ, err)
	}
	return m
}

func sorted(list []string) []string {
	out := slices.Clone(list)
	slices.Sort(out)
	return out
}

func equalSets(t *testing.T, what string, a, b []string) {
	t.Helper()
	if !slices.Equal(sorted(a), sorted(b)) {
		t.Errorf("%s: %v != %v", what, a, b)
	}
}

// Values the HTTP API shares with the event contract and with the process
// server are the same values: an action type always has its event type, the
// kinds of actor and scope are those of the analytics of C-10, and so on.
func TestOpenAPIEnumsMatchGoAndEventSchemas(t *testing.T) {
	doc := loadSpec(t)
	schema := func(keys ...string) any {
		return at(t, doc, append([]string{"components", "schemas"}, keys...)...)
	}

	started := eventSchema(t, "analytics.session.started")
	actorKinds := stringList(t, doc, "components", "parameters", "ActorKind", "schema", "enum")
	equalSets(t, "X-Actor-Kind enum vs api.ActorKinds", actorKinds, api.ActorKinds())
	equalSets(t, "X-Actor-Kind enum vs analytics.session.started session.actor_kind",
		actorKinds, stringList(t, started, "properties", "session", "properties", "actor_kind", "enum"))
	equalSets(t, "AdminSession.actor_kind vs X-Actor-Kind",
		stringList(t, schema("AdminSession", "properties", "actor_kind"), "enum"), actorKinds)

	scopeKinds := stringList(t, started, "properties", "session", "properties", "kind", "enum")
	equalSets(t, "ScopeRef.type vs analytics.session.started session.kind",
		stringList(t, schema("ScopeRef", "properties", "type"), "enum"), scopeKinds)
	equalSets(t, "AdminSession.kind vs analytics.session.started session.kind",
		stringList(t, schema("AdminSession", "properties", "kind"), "enum"), scopeKinds)

	actionTypes := stringList(t, schema("ActionRequest", "properties", "type"), "enum")
	equalSets(t, "ActionRequest.type enum vs api.ActionTypes", actionTypes, api.ActionTypes())
	equalSets(t, "ActionAccepted.status vs api.ActionStatusAccepted",
		stringList(t, schema("ActionAccepted", "properties", "status"), "enum"), []string{api.ActionStatusAccepted})
	equalSets(t, "ActionPending.status vs api.ActionStatusPending",
		stringList(t, schema("ActionPending", "properties", "status"), "enum"), []string{api.ActionStatusPending})

	said := eventSchema(t, "player.said")
	if got, want := at(t, schema("ActionRequest", "properties", "text"), "maxLength"), at(t, said, "properties", "text", "maxLength"); toInt(got) < 0 || toInt(got) != toInt(want) {
		t.Errorf("ActionRequest.text maxLength %v, player.said text maxLength %v", got, want)
	}

	group := eventSchema(t, "group.created")
	equalSets(t, "GroupMember.participation vs group.created members[].participation",
		stringList(t, schema("GroupMember", "properties", "participation"), "enum"),
		stringList(t, group, "properties", "members", "items", "properties", "participation", "enum"))

	closed := eventSchema(t, "round.closed")
	equalSets(t, "ClosedRound.close_reason vs round.closed round.close_reason",
		stringList(t, schema("ClosedRound", "properties", "close_reason"), "enum"),
		stringList(t, closed, "properties", "round", "properties", "close_reason", "enum"))

	equalSets(t, "Health.status vs runtime statuses",
		stringList(t, schema("Health", "properties", "status"), "enum"),
		[]string{runtime.StatusOK, runtime.StatusDegraded, runtime.StatusFail})

	if api.HeaderClientID != runtime.ClientIDHeader || api.HeaderActorKind != runtime.ActorKindHeader {
		t.Errorf("headers %s/%s differ from shared/runtime %s/%s",
			api.HeaderClientID, api.HeaderActorKind, runtime.ClientIDHeader, runtime.ActorKindHeader)
	}
}

// eventOfAction is the event each action becomes (api-contracts.md §1.4, §2.3.1,
// §2.3.2). A solo move is published as player.*; the group move of a leader is
// group.entered_region, which the actions task adds next to it.
var eventOfAction = map[string]string{
	api.ActionEnter:       "player.entered_region",
	api.ActionLeave:       "player.left_region",
	api.ActionLook:        "player.looked",
	api.ActionAttack:      "player.attacked",
	api.ActionFlee:        "player.flee_attempted",
	api.ActionRest:        "player.rested",
	api.ActionSay:         "player.said",
	api.ActionDefend:      "player.defended",
	api.ActionGroupCreate: "group.created",
	api.ActionGroupJoin:   "group.joined",
	api.ActionGroupLeave:  "group.left",
}

func TestEveryActionTypeHasARegisteredEvent(t *testing.T) {
	for _, action := range api.ActionTypes() {
		typ, ok := eventOfAction[action]
		if !ok {
			t.Errorf("action %q has no event in the test table", action)
			continue
		}
		spec, ok := contracts.Lookup(typ)
		if !ok {
			t.Errorf("action %q: event %s is not registered", action, typ)
			continue
		}
		if spec.Schema == nil {
			t.Errorf("action %q: event %s has no schema", action, typ)
		}
		if spec.Owner != contracts.OwnerGateway {
			t.Errorf("action %q: event %s is owned by %s, want %s", action, typ, spec.Owner, contracts.OwnerGateway)
		}
	}
	if len(eventOfAction) != len(api.ActionTypes()) {
		t.Errorf("the test table has %d actions, the dictionary %d", len(eventOfAction), len(api.ActionTypes()))
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return -1
	}
}
