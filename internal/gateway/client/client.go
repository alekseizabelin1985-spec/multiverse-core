// Package client is the Go client of the gateway HTTP API (C-08 v1.5), shared
// by the Telegram bot and the CI harness. The wire types are those of
// internal/gateway/api.
//
// Every operation of C-08 is idempotent by construction or by action_key, so a
// network error or 503 is repeated with the same body, and therefore with the
// same action_key (component §6). A 4xx is never repeated, and neither is
// 503 forget_incomplete (C-08 v1.5).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/clock"
)

// DefaultHTTPTimeout bounds one attempt. It is above the longest answer the
// gateway may take: a long-poll of wait_ms 25 s plus its 5 s handler margin
// (component §5.1).
const DefaultHTTPTimeout = 35 * time.Second

// maxResponseBytes bounds a response body; 100 deliveries of plain text fit
// with a wide margin.
const maxResponseBytes = 4 << 20

// Backoff is the repeat policy for network errors and 503: Retries repeats
// after the first attempt, the pause doubling from Initial and capped at Max.
type Backoff struct {
	Retries int
	Initial time.Duration
	Max     time.Duration
}

// DefaultBackoff repeats three times after 200, 400 and 800 ms; Max 1.6 s is
// the ceiling of component §6 and matters only for a policy with more repeats.
// It is also what the zero Backoff means, so a Client written as a literal
// repeats like one made by New.
var DefaultBackoff = Backoff{Retries: 3, Initial: 200 * time.Millisecond, Max: 1600 * time.Millisecond}

// NoRetry is the explicit policy without repeats.
var NoRetry = Backoff{Retries: -1}

// Pause returns the pause before repeat n, counted from 0. Max 0 means no
// ceiling.
func (b Backoff) Pause(n int) time.Duration {
	d := b.Initial
	for i := 0; i < n; i++ {
		d *= 2
		if b.Max > 0 && d >= b.Max {
			return b.Max
		}
	}
	if b.Max > 0 && d > b.Max {
		return b.Max
	}
	return d
}

// Client calls the gateway on behalf of one client instance.
type Client struct {
	// BaseURL is the address of the gateway, e.g. http://127.0.0.1:8088.
	BaseURL string
	// ClientID is sent as X-Client-Id and used as {client_id} of deliveries.
	ClientID string
	// ActorKind is sent as X-Actor-Kind when not empty; the gateway defaults
	// to human.
	ActorKind string
	// HTTP performs the requests; nil means a client with DefaultHTTPTimeout.
	HTTP *http.Client
	// Timers drives the pauses between repeats; nil means the wall clock.
	Timers clock.Timers
	// Backoff is the repeat policy; the zero value means DefaultBackoff and
	// NoRetry switches repeats off. A policy set by hand sets all its fields.
	Backoff Backoff
}

// New returns a client with DefaultBackoff, the wall clock and an HTTP client
// with DefaultHTTPTimeout. A literal Client{BaseURL, ClientID} behaves the same:
// every nil or zero field falls back to these defaults.
func New(baseURL, clientID string) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		ClientID: clientID,
		HTTP:     &http.Client{Timeout: DefaultHTTPTimeout},
		Timers:   clock.RealTimers{},
		Backoff:  DefaultBackoff,
	}
}

// APIError is an error answer of the gateway. Callers match on Code (the bot
// maps it to a hint for the player, component §10.5).
type APIError struct {
	Status    int
	Code      string
	Message   string
	Details   map[string]any
	RequestID string
	// RetryAfter is the Retry-After header in whole seconds, 0 when the
	// answer has none (rate_limited, forget_incomplete).
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("gateway: %d %s", e.Status, e.Message)
	}
	return fmt.Sprintf("gateway: %d %s: %s", e.Status, e.Code, e.Message)
}

// ErrUnexpectedStatus reports a successful status the operation does not
// define, such as 204 where the contract answers 200.
var ErrUnexpectedStatus = errors.New("gateway: unexpected status")

// Resolve resolves an external account (POST /v1/links/resolve).
func (c *Client) Resolve(ctx context.Context, platform, externalID string) (api.ResolveResponse, error) {
	var out api.ResolveResponse
	req := api.ResolveRequest{ExternalPlatform: platform, ExternalID: externalID}
	_, _, err := c.call(ctx, request{method: http.MethodPost, path: "/v1/links/resolve", body: req, retry: true},
		decodeOn(&out, http.StatusOK))
	return out, err
}

// Consent records the notice, the consent and the age (POST /v1/links/consent).
func (c *Client) Consent(ctx context.Context, req api.ConsentRequest) (api.ConsentResponse, error) {
	var out api.ConsentResponse
	_, _, err := c.call(ctx, request{method: http.MethodPost, path: "/v1/links/consent", body: req, retry: true},
		decodeOn(&out, http.StatusOK))
	return out, err
}

// ForgetResult is the answer to /forget and whether it came to a repeat.
//
// The end state of /forget does not depend on how often it is sent: no link,
// no character bound to the account. The answer does. When an attempt deleted
// the link but its answer was lost, the repeat finds nothing and says
// {deleted: false}. So Deleted=false with Repeated=true is the end of the same
// /forget: the caller tells the player the link is gone, not "nothing to
// delete", and does not check it with Resolve. Resolve of an account without a
// link inserts a pending_consent row with the external id, so such a check
// would write the forgotten id back into links.db (T-311, SEC-04/05, US-009).
type ForgetResult struct {
	api.ForgetResponse
	Repeated bool
}

// Forget deletes the link of an external account (DELETE /v1/links). It is
// repeated like the other calls, because a /forget that silently did not
// happen is worse than an ambiguous answer; the ambiguity is reported in
// ForgetResult.Repeated.
//
// 503 forget_incomplete is the exception: the link is deleted, its wipe from
// links.db is not confirmed, and every attempt under the reader that blocks it
// holds the only connection of links.db for up to 5 s. The error is returned
// at once with APIError.RetryAfter, and the caller asks the player to repeat
// later (C-08 v1.5).
func (c *Client) Forget(ctx context.Context, platform, externalID string) (ForgetResult, error) {
	var out ForgetResult
	req := api.ForgetRequest{ExternalPlatform: platform, ExternalID: externalID}
	_, attempts, err := c.call(ctx, request{method: http.MethodDelete, path: "/v1/links", body: req, retry: true},
		decodeOn(&out.ForgetResponse, http.StatusOK))
	out.Repeated = attempts > 1
	return out, err
}

// Worlds lists the worlds (GET /v1/worlds).
func (c *Client) Worlds(ctx context.Context) (api.WorldsResponse, error) {
	var out api.WorldsResponse
	_, _, err := c.call(ctx, request{method: http.MethodGet, path: "/v1/worlds", retry: true},
		decodeOn(&out, http.StatusOK))
	return out, err
}

// CreateCharacter creates a character (POST /v1/characters). The status tells
// 201 created, 200 already alive and 202 creating apart.
func (c *Client) CreateCharacter(ctx context.Context, req api.CreateCharacterRequest) (api.CreateCharacterResponse, int, error) {
	var out api.CreateCharacterResponse
	status, _, err := c.call(ctx, request{method: http.MethodPost, path: "/v1/characters", body: req, retry: true},
		decodeOn(&out, http.StatusOK, http.StatusCreated, http.StatusAccepted))
	return out, status, err
}

// Player returns the state of a character (GET /v1/players/{player_id}).
func (c *Client) Player(ctx context.Context, playerID string) (api.CharacterState, error) {
	var out api.CharacterState
	_, _, err := c.call(ctx, request{method: http.MethodGet, path: "/v1/players/" + url.PathEscape(playerID), retry: true},
		decodeOn(&out, http.StatusOK))
	return out, err
}

// ActionResult is the successful answer to an action: exactly one of Accepted
// (202 accepted), Pending (202 pending of group.*) and Group (200 of group.*)
// is set.
//
// Component §6 declares Action as returning (ActionAccepted, *GroupView,
// error); that signature predates the 202 pending answer of C-08 v1.1, which
// carries group_id and fits neither.
type ActionResult struct {
	Status   int
	Accepted *api.ActionAccepted
	Pending  *api.ActionPending
	Group    *api.GroupView
}

// Action sends an action (POST /v1/players/{player_id}/actions).
func (c *Client) Action(ctx context.Context, playerID string, req api.ActionRequest) (ActionResult, error) {
	var res ActionResult
	status, _, err := c.call(ctx, request{method: http.MethodPost, path: "/v1/players/" + url.PathEscape(playerID) + "/actions", body: req, retry: true},
		func(status int, data []byte) error {
			switch status {
			case http.StatusOK:
				res.Group = new(api.GroupView)
				return decode(data, res.Group)
			case http.StatusAccepted:
				return decodeAccepted(data, &res)
			default:
				return fmt.Errorf("%w %d", ErrUnexpectedStatus, status)
			}
		})
	res.Status = status
	return res, err
}

func decodeAccepted(data []byte, res *ActionResult) error {
	var probe struct {
		Status string `json:"status"`
	}
	if err := decode(data, &probe); err != nil {
		return err
	}
	switch probe.Status {
	case api.ActionStatusPending:
		res.Pending = new(api.ActionPending)
		return decode(data, res.Pending)
	case api.ActionStatusAccepted:
		res.Accepted = new(api.ActionAccepted)
		return decode(data, res.Accepted)
	default:
		return fmt.Errorf("%w: 202 with status %q", ErrUnexpectedStatus, probe.Status)
	}
}

// Group returns the state of a group (GET /v1/groups/{group_id}); a client
// polls it after a pending group.* action.
func (c *Client) Group(ctx context.Context, groupID string) (api.GroupView, error) {
	var out api.GroupView
	_, _, err := c.call(ctx, request{method: http.MethodGet, path: "/v1/groups/" + url.PathEscape(groupID), retry: true},
		decodeOn(&out, http.StatusOK))
	return out, err
}

// CloseRound closes the open round of a scope (POST
// /v1/scopes/{scope_id}/rounds/close, ci only). It is never repeated: a repeat
// after a lost answer would meet the round it closed and turn a success into
// 409 no_open_round. The harness that calls it decides whether to try again.
func (c *Client) CloseRound(ctx context.Context, scopeID string) (api.RoundCloseResponse, error) {
	var out api.RoundCloseResponse
	_, _, err := c.call(ctx, request{method: http.MethodPost, path: "/v1/scopes/" + url.PathEscape(scopeID) + "/rounds/close"},
		decodeOn(&out, http.StatusOK))
	return out, err
}

type request struct {
	method string
	path   string
	query  url.Values
	body   any
	retry  bool
}

// call performs a request with the repeat policy and hands a successful answer
// to onSuccess. It returns the status of the last attempt and the number of
// attempts made. Error texts name the method and the path, never the body:
// bodies of links and characters carry the external id (SEC-01/02).
func (c *Client) call(ctx context.Context, req request, onSuccess func(status int, data []byte) error) (int, int, error) {
	var payload []byte
	if req.body != nil {
		var err error
		if payload, err = json.Marshal(req.body); err != nil {
			return 0, 0, fmt.Errorf("gateway %s %s: encode body: %w", req.method, req.path, err)
		}
	}
	// A malformed BaseURL fails the same way on every attempt: report it once
	// instead of repeating it as a network error.
	if _, err := http.NewRequestWithContext(ctx, req.method, c.url(req), nil); err != nil {
		return 0, 0, fmt.Errorf("gateway %s %s: build request: %w", req.method, req.path, err)
	}
	policy := c.policy()
	for attempt := 0; ; attempt++ {
		status, header, data, err := c.once(ctx, req, payload)
		if c.repeat(ctx, req, policy, attempt, status, err) && !forgetIncomplete(status, data) {
			if werr := c.pause(ctx, policy.Pause(attempt)); werr != nil {
				return status, attempt + 1, fmt.Errorf("gateway %s %s: stopped before repeat %d (last attempt: %s): %w",
					req.method, req.path, attempt+1, outcome(status, err), werr)
			}
			continue
		}
		if err != nil {
			return 0, attempt + 1, fmt.Errorf("gateway %s %s: attempt %d: %w", req.method, req.path, attempt+1, err)
		}
		if status < 200 || status > 299 {
			return status, attempt + 1, apiError(status, header, data)
		}
		if err := onSuccess(status, data); err != nil {
			return status, attempt + 1, fmt.Errorf("gateway %s %s: %w", req.method, req.path, err)
		}
		return status, attempt + 1, nil
	}
}

func outcome(status int, err error) string {
	if err != nil {
		return err.Error()
	}
	return strconv.Itoa(status)
}

// repeat says whether an attempt is repeated: only a network error or 503, and
// never a 4xx (429 included: a repeat would spend the rate limit, SEC-11).
func (c *Client) repeat(ctx context.Context, req request, policy Backoff, attempt, status int, err error) bool {
	if !req.retry || attempt >= policy.Retries || ctx.Err() != nil {
		return false
	}
	return err != nil || status == http.StatusServiceUnavailable
}

// forgetIncomplete says whether an answer is 503 forget_incomplete, which is
// not repeated (C-08 v1.5).
func forgetIncomplete(status int, data []byte) bool {
	if status != http.StatusServiceUnavailable {
		return false
	}
	var body api.ErrorResponse
	return json.Unmarshal(data, &body) == nil && body.Error.Code == api.CodeForgetIncomplete
}

func (c *Client) policy() Backoff {
	if c.Backoff == (Backoff{}) {
		return DefaultBackoff
	}
	return c.Backoff
}

func (c *Client) url(req request) string {
	u := strings.TrimRight(c.BaseURL, "/") + req.path
	if len(req.query) > 0 {
		u += "?" + req.query.Encode()
	}
	return u
}

func (c *Client) once(ctx context.Context, req request, payload []byte) (int, http.Header, []byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	hr, err := http.NewRequestWithContext(ctx, req.method, c.url(req), body)
	if err != nil {
		return 0, nil, nil, err
	}
	hr.Header.Set("Accept", "application/json")
	if payload != nil {
		hr.Header.Set("Content-Type", api.ContentTypeJSON)
	}
	hr.Header.Set(api.HeaderClientID, c.ClientID)
	if c.ActorKind != "" {
		hr.Header.Set(api.HeaderActorKind, c.ActorKind)
	}
	resp, err := c.httpClient().Do(hr)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, nil, fmt.Errorf("read body of %d: %w", resp.StatusCode, err)
	}
	return resp.StatusCode, resp.Header, data, nil
}

func (c *Client) pause(ctx context.Context, d time.Duration) error {
	t := c.timers().After(d)
	select {
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: DefaultHTTPTimeout}
}

func (c *Client) timers() clock.Timers {
	if c.Timers != nil {
		return c.Timers
	}
	return clock.RealTimers{}
}

// apiError reads the error body. A body that is not the contract error (a
// proxy page, an empty 502) still yields an APIError with the status, so the
// caller branches on one type.
func apiError(status int, header http.Header, data []byte) *APIError {
	e := &APIError{Status: status, RequestID: header.Get(api.HeaderRequestID), RetryAfter: retryAfter(header)}
	var body api.ErrorResponse
	if err := json.Unmarshal(data, &body); err == nil && body.Error.Code != "" {
		e.Code, e.Message, e.Details = body.Error.Code, body.Error.Message, body.Error.Details
		return e
	}
	e.Message = http.StatusText(status)
	return e
}

// MaxRetryAfter bounds a Retry-After read from an answer, so that a huge
// value does not overflow time.Duration (N-3 of review #1 of T-311).
const MaxRetryAfter = 24 * time.Hour

// retryAfter reads Retry-After as the gateway writes it, in whole seconds
// (C-08 v1.5); the HTTP-date form and a value that is not a positive number
// read as 0, and a value above MaxRetryAfter reads as MaxRetryAfter.
func retryAfter(header http.Header) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(header.Get(api.HeaderRetryAfter)))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(min(secs, int(MaxRetryAfter/time.Second))) * time.Second
}

func decodeOn(out any, statuses ...int) func(int, []byte) error {
	return func(status int, data []byte) error {
		for _, s := range statuses {
			if s == status {
				return decode(data, out)
			}
		}
		return fmt.Errorf("%w %d", ErrUnexpectedStatus, status)
	}
}

func decode(data []byte, out any) error {
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %T: %w", out, err)
	}
	return nil
}
