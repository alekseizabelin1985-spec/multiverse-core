package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
)

// AdminClientID is the X-Client-Id mvctl sends to the admin routes of core. It
// is on the default list of MV_CORE_ADMIN_CLIENTS (ADR-009 p. 9); a core whose
// list leaves it out answers 403.
const AdminClientID = "mvctl"

// The bounds of the requests to core. The snapshot is bounded on the side of
// State by state.SnapshotTimeout; the request waits a little longer, so that
// State, not the client, says why a snapshot took too long.
const (
	coreHealthTimeout   = 5 * time.Second
	coreSnapshotTimeout = state.SnapshotTimeout + 10*time.Second
	// coreBodyLimit bounds what is read of an answer of core.
	coreBodyLimit = 1 << 20
)

// core is the HTTP server of the running core, as mvctl world init --bus kafka
// talks to it: GET /health and POST /v1/admin/state/{world}/snapshot. The
// requests go to url; what is printed is display, the address without the
// password it may carry.
type core struct {
	url     string
	display string
	client  *http.Client
}

func newCore(base string) core {
	base = strings.TrimRight(base, "/")
	return core{url: base, display: redacted(base), client: &http.Client{}}
}

// redacted is an address fit to print: the password of its userinfo, if any,
// is replaced (url.URL.Redacted). An address that does not parse is not
// printed at all — its text might be the secret.
func redacted(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return "(" + env.CoreURL.Name() + " does not parse as a URL)"
	}
	return u.Redacted()
}

// errNotARequest is an address no request can be built for. The error of
// net/http is not wrapped: it quotes the address as it is, password included.
func errNotARequest(display string) error {
	return fmt.Errorf("core at %s: %s is not an address a request can go to", display, env.CoreURL.Name())
}

// errCoreUnreachable is a core that gave no HTTP answer at all.
var errCoreUnreachable = errors.New("not reachable")

// coreAnswer is an HTTP answer of core that is not the one asked for.
type coreAnswer struct {
	status  int
	code    string
	message string
}

func (a *coreAnswer) Error() string {
	text := fmt.Sprintf("answered %d", a.status)
	if a.code != "" {
		text += " " + a.code
	}
	if a.message != "" {
		text += ": " + a.message
	}
	if a.status == http.StatusForbidden {
		text += fmt.Sprintf(" (mvctl sends %s %s; core admits the clients of %s)",
			runtime.ClientIDHeader, AdminClientID, env.CoreAdminClients.Name())
	}
	return text
}

// health is GET /health of core, decoded. Any HTTP answer with a JSON body is
// a health: a core that fails answers 503 with the same shape.
func (c core) health(ctx context.Context) (runtime.Status, error) {
	ctx, cancel := context.WithTimeout(ctx, coreHealthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/health", nil)
	if err != nil {
		return runtime.Status{}, errNotARequest(c.display)
	}
	status, body, err := c.do(req)
	if err != nil {
		return runtime.Status{}, err
	}
	var health runtime.Status
	if err := json.Unmarshal(body, &health); err != nil {
		return runtime.Status{}, fmt.Errorf("core at %s: /health %w", c.display,
			&coreAnswer{status: status, message: "the body is not a health: " + excerpt(body)})
	}
	return health, nil
}

// snapshot asks the State of core for a snapshot of the world with reason
// bootstrap and returns latest.json as written.
func (c core) snapshot(ctx context.Context, worldID string) (*state.LatestPointer, error) {
	ctx, cancel := context.WithTimeout(ctx, coreSnapshotTimeout)
	defer cancel()
	body, err := json.Marshal(state.SnapshotRequest{Reason: state.SnapshotBootstrap})
	if err != nil {
		return nil, err
	}
	route := c.url + "/v1/admin/state/" + url.PathEscape(worldID) + "/snapshot"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, route, bytes.NewReader(body))
	if err != nil {
		return nil, errNotARequest(c.display)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(runtime.ClientIDHeader, AdminClientID)
	req.Header.Set(runtime.ActorKindHeader, "ci")
	status, answer, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		code, message := errorOf(answer)
		return nil, fmt.Errorf("core at %s: POST %s %w", c.display, req.URL.Path,
			&coreAnswer{status: status, code: code, message: message})
	}
	var pointer state.LatestPointer
	if err := json.Unmarshal(answer, &pointer); err != nil || pointer.Snapshot.ID == "" {
		return nil, fmt.Errorf("core at %s: POST %s %w", c.display, req.URL.Path,
			&coreAnswer{status: status, message: "the body is not latest.json: " + excerpt(answer)})
	}
	return &pointer, nil
}

// do sends the request and reads the answer, whatever its status.
func (c core) do(req *http.Request) (int, []byte, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("core at %s (%s) is %w: %w", c.display, env.CoreURL.Name(), errCoreUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, coreBodyLimit))
	if err != nil {
		return 0, nil, fmt.Errorf("core at %s: read the answer to %s %s: %w", c.display, req.Method, req.URL.Path, err)
	}
	return resp.StatusCode, body, nil
}

// errorOf reads the error of an answer of the HTTP API of the platform. It is
// tolerant: {"error": {"code", "message"}} of the routes, {"error": "..."} of
// runtime.AdminOnly, or any other body, which is quoted.
func errorOf(body []byte) (code, message string) {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Error) > 0 {
		var detailed struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(envelope.Error, &detailed); err == nil && (detailed.Code != "" || detailed.Message != "") {
			return detailed.Code, detailed.Message
		}
		var text string
		if err := json.Unmarshal(envelope.Error, &text); err == nil {
			return "", text
		}
	}
	return "", excerpt(body)
}

// excerpt is the start of a body, for a message.
func excerpt(body []byte) string {
	const limit = 200
	text := strings.TrimSpace(string(body))
	if len(text) > limit {
		text = strings.ToValidUTF8(text[:limit], "") + "…"
	}
	return text
}

// worldOfHealth is the section of the world in the health of core:
// details.contexts.state.details.worlds.<world> (state-and-mechanics.md §10,
// §19). stateSeen says whether the health has a context state at all.
func worldOfHealth(health runtime.Status, worldID string) (section map[string]any, stateStatus runtime.Status, stateSeen bool) {
	contexts, _ := health.Details["contexts"].(map[string]any)
	raw, ok := contexts["state"]
	if !ok {
		return nil, runtime.Status{}, false
	}
	encoded, err := json.Marshal(raw)
	if err != nil || json.Unmarshal(encoded, &stateStatus) != nil {
		return nil, runtime.Status{}, false
	}
	worlds, _ := stateStatus.Details["worlds"].(map[string]any)
	section, _ = worlds[worldID].(map[string]any)
	return section, stateStatus, true
}
