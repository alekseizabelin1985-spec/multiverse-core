package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
)

// ClientID is the X-Client-Id of the HTTP harness: the CI client of the
// gateway, listed in MV_GATEWAY_CLIENT_IDS and MV_GATEWAY_ACTOR_KIND_CLIENTS
// of dev and absent from both in production (ADR-009 p. 9, C-08 v1.3).
const ClientID = "ci-harness"

// Platform is the external platform the HTTP harness links its fixtures on.
// The HTTP API of MVP-1 creates links of telegram only, and the gateway gives
// ci-harness the deliveries of telegram — provisionally, until system-architect
// decides the platform of the CI client (handlers.ClientPlatforms, decision 1
// of the orchestrator on T-307). The consequence is in README.md: the harness
// must not run against a gateway a bot is polling.
const Platform = "telegram"

// DefaultHTTPTimeout is how long RegisterAndEnter waits for State to answer a
// character and a movement. It is wall time, for the reason DefaultTimeout is.
const DefaultHTTPTimeout = DefaultTimeout

// pollPause is the pause between two reads of a character that has not
// arrived where it was sent yet.
const pollPause = 20 * time.Millisecond

// ErrNoDelivery is what AwaitDelivery reports when no delivery of the kind
// arrived within its timeout.
var ErrNoDelivery = errors.New("no delivery arrived")

// ErrRoundsNotMounted is what CloseRound reports while the gateway does not
// serve closeRound (I1; the route comes with T-354). The error CloseRound
// returns matches it with errors.Is, and errors.As reads it as a fresh
// *client.APIError 501 not_implemented, the answer C-08 gives to an operation
// that is not there.
var ErrRoundsNotMounted = errors.New("testkit/gateway: closeRound is not mounted by the gateway yet (T-354)")

// roundsNotMounted is the answer of CloseRound to a bare 404. It keeps that
// answer, so that a 404 of something else — a wrong base URL, another server —
// still shows in the message: the harness cannot tell the two apart.
type roundsNotMounted struct{ answer *client.APIError }

func (e roundsNotMounted) Error() string {
	return ErrRoundsNotMounted.Error() + ": 501 " + api.CodeNotImplemented + " (the server answered " + e.answer.Error() + ")"
}

func (e roundsNotMounted) Is(target error) bool { return target == ErrRoundsNotMounted }

// As gives every caller its own copy: an error value shared by the package
// could be changed by one test under another.
func (e roundsNotMounted) As(target any) bool {
	p, ok := target.(**client.APIError)
	if ok {
		*p = &client.APIError{Status: http.StatusNotImplemented, Code: api.CodeNotImplemented, Message: ErrRoundsNotMounted.Error()}
	}
	return ok
}

// HTTPHarness plays the fixture characters through the HTTP API of the
// gateway, as a client of its own (C-08): it links them, creates them, sends
// their actions and takes their deliveries by long-poll. It stands next to
// Harness, which publishes the same actions straight to the bus, and changes
// nothing of it.
//
// It uses internal/gateway/client, the client the bot uses (NFR-092).
//
// One HTTPHarness per gateway. It plays every fixture character through one
// client, ci-harness, and every client of that id shares one queue of
// deliveries: a second harness on the same gateway would answer 409
// poll_in_progress or take and acknowledge the deliveries of the first. Calls
// of AwaitDelivery on one harness are serialised; each counts its timeout from
// its own call, not from the moment it gets its turn to poll.
type HTTPHarness struct {
	client  *client.Client
	worldID string
	// clock stamps what the client itself reports: the moment the notice was
	// shown. wall measures the waits.
	clock   clock.Clock
	wall    clock.Clock
	timers  clock.Timers
	timeout time.Duration
	// players is the fixture characters the harness can register, by id.
	players map[string]*entity.Entity

	mu sync.Mutex
	// registered maps a fixture id to the player_id the gateway gave it.
	registered map[string]string
	keys       int

	// polling serialises the long-poll: the gateway allows one poll per
	// client, and a second one answers 409 poll_in_progress.
	polling sync.Mutex
	// cursor is the cursor of the last answer of the long-poll.
	cursor string
	// inbox is what the long-poll gave and no AwaitDelivery took yet, in the
	// order of the answers. Every delivery in it is acknowledged.
	inbox []api.Delivery
}

// NewCIClient is the client of the harness: ClientID and X-Actor-Kind ci on
// baseURL, without repeats, so that a test sees the first answer.
func NewCIClient(baseURL string) *client.Client {
	c := client.New(baseURL, ClientID)
	c.ActorKind = api.ActorCI
	c.Backoff = client.NoRetry
	return c
}

// NewHTTPHarness builds the harness over a client of the gateway and the
// entities of a fixture world (state.LoadFixtures). The world is the one the
// fixture world entity names; the characters are its players, and the
// external ID of each is its fixture id — player-A, player-B, player-C are not
// personal data (design.md §7).
func NewHTTPHarness(c *client.Client, fixtures []*entity.Entity) (*HTTPHarness, error) {
	if c == nil {
		return nil, errors.New("testkit/gateway: no client")
	}
	worldID := ""
	players := make(map[string]*entity.Entity)
	for _, e := range fixtures {
		if e == nil || e.ID == "" {
			return nil, errors.New("testkit/gateway: a fixture without an id")
		}
		switch e.Type {
		case entity.TypeWorld:
			if worldID != "" {
				return nil, errors.New("testkit/gateway: the fixtures hold more than one world")
			}
			worldID = e.ID
		case entity.TypePlayer:
			if _, seen := players[e.ID]; seen {
				return nil, fmt.Errorf("testkit/gateway: %s appears twice in the fixtures", e.ID)
			}
			players[e.ID] = entity.Clone(e)
		}
	}
	if worldID == "" {
		return nil, errors.New("testkit/gateway: the fixtures hold no world")
	}
	return &HTTPHarness{client: c, worldID: worldID, clock: clock.Real{}, wall: clock.Real{}, timers: clock.RealTimers{},
		timeout: DefaultHTTPTimeout, players: players, registered: make(map[string]string)}, nil
}

// WithTimeout changes how long RegisterAndEnter waits for State. It is called
// before the harness acts, like Harness.WithTimeout.
func (h *HTTPHarness) WithTimeout(d time.Duration) *HTTPHarness {
	if d > 0 {
		h.timeout = d
	}
	return h
}

// WithClock gives the harness the clock shown_at of a consent is taken from,
// so that links.db of a test holds the same moment on every run. The waits of
// the harness stay on the wall clock.
func (h *HTTPHarness) WithClock(c clock.Clock) *HTTPHarness {
	if c != nil {
		h.clock = c
	}
	return h
}

// Client is the client of the gateway the harness sends through.
func (h *HTTPHarness) Client() *client.Client { return h.client }

// WorldID is the world the harness registers its characters in.
func (h *HTTPHarness) WorldID() string { return h.worldID }

// PlayerID is the player_id the gateway gave a registered fixture character.
func (h *HTTPHarness) PlayerID(fixtureID string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id, ok := h.registered[fixtureID]
	return id, ok
}

// Entered is a character the harness registered and walked into a region.
type Entered struct {
	PlayerID string
	// CorrelationID is the correlation of the enter action: the mechanics of
	// the move and the narrative of the entry are delivered under it.
	CorrelationID string
}

// RegisterAndEnter takes a fixture character from nothing to alive in a
// region: resolve and consent of its external account, the character created in
// the world of the fixtures, the enter action, and a wait until GET
// /v1/players/{id} places it in the region. State has to be on the bus: the
// gateway creates and moves nothing without its facts.
//
// A character that is already registered is not created again; the enter is
// sent all the same.
func (h *HTTPHarness) RegisterAndEnter(ctx context.Context, fixtureID, regionID string) (Entered, error) {
	fixture, ok := h.players[fixtureID]
	if !ok {
		return Entered{}, fmt.Errorf("testkit/gateway: %s is not a player of the fixtures", fixtureID)
	}
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	playerID, err := h.register(ctx, fixture)
	if err != nil {
		return Entered{}, err
	}
	target := regionID
	correlation, err := h.Act(ctx, playerID, api.ActionRequest{Type: api.ActionEnter, Target: &target})
	if err != nil {
		return Entered{}, fmt.Errorf("testkit/gateway: %s enters %s: %w", fixtureID, regionID, err)
	}
	err = h.until(ctx, func() (bool, error) {
		p, err := h.client.Player(ctx, playerID)
		if err != nil {
			return false, err
		}
		return p.Status == entity.StatusAlive && p.Position != nil && p.Position.Kind == entity.TypeRegion &&
			p.Position.ID == regionID, nil
	})
	if err != nil {
		return Entered{}, fmt.Errorf("testkit/gateway: %s did not arrive in %s: %w", fixtureID, regionID, err)
	}
	return Entered{PlayerID: playerID, CorrelationID: correlation}, nil
}

// register links and creates the character of a fixture and returns its
// player_id once the gateway reports it alive.
func (h *HTTPHarness) register(ctx context.Context, fixture *entity.Entity) (string, error) {
	if id, ok := h.PlayerID(fixture.ID); ok {
		return id, nil
	}
	if _, err := h.client.Resolve(ctx, Platform, fixture.ID); err != nil {
		return "", fmt.Errorf("testkit/gateway: resolve %s: %w", fixture.ID, err)
	}
	if _, err := h.client.Consent(ctx, api.ConsentRequest{ExternalPlatform: Platform, ExternalID: fixture.ID,
		NoticeShown: true, Consent: true, AgeConfirmed: true, ShownAt: h.clock.Now().UTC()}); err != nil {
		return "", fmt.Errorf("testkit/gateway: consent of %s: %w", fixture.ID, err)
	}
	res, _, err := h.client.CreateCharacter(ctx, api.CreateCharacterRequest{ExternalPlatform: Platform,
		ExternalID: fixture.ID, WorldID: h.worldID, CharacterName: fixture.Name, ActionKey: "create-" + fixture.ID})
	if err != nil {
		return "", fmt.Errorf("testkit/gateway: create %s: %w", fixture.ID, err)
	}
	// 202 creating: the fact of State is late, and the link says when it came.
	err = h.until(ctx, func() (bool, error) {
		r, err := h.client.Resolve(ctx, Platform, fixture.ID)
		if err != nil {
			return false, err
		}
		return r.CharacterStatus == entity.StatusAlive && r.PlayerID != nil && *r.PlayerID == res.PlayerID, nil
	})
	if err != nil {
		return "", fmt.Errorf("testkit/gateway: %s is not alive: %w", fixture.ID, err)
	}
	h.mu.Lock()
	h.registered[fixture.ID] = res.PlayerID
	h.mu.Unlock()
	return res.PlayerID, nil
}

// until repeats cond until it holds, fails or ctx ends.
func (h *HTTPHarness) until(ctx context.Context, cond func() (bool, error)) error {
	for {
		ok, err := cond()
		if err != nil || ok {
			return err
		}
		pause := h.timers.After(pollPause)
		select {
		case <-ctx.Done():
			pause.Stop()
			return ctx.Err()
		case <-pause.C():
		}
	}
}

// Act sends an action of a player and returns the correlation_id of the event
// it became: 202 accepted, or 202 pending of a group.* action. An empty
// action_key is filled with the next key of the harness, so that a repeat of
// the same request by the test is still one action.
//
// A 200 group view carries no correlation and is an error here: a group action
// whose fact came in time has nothing to await deliveries under.
func (h *HTTPHarness) Act(ctx context.Context, playerID string, req api.ActionRequest) (string, error) {
	if req.ActionKey == "" {
		h.mu.Lock()
		h.keys++
		req.ActionKey = "ci-" + strconv.Itoa(h.keys)
		h.mu.Unlock()
	}
	res, err := h.client.Action(ctx, playerID, req)
	switch {
	case err != nil:
		return "", err
	case res.Accepted != nil:
		return res.Accepted.CorrelationID, nil
	case res.Pending != nil:
		return res.Pending.CorrelationID, nil
	default:
		return "", fmt.Errorf("testkit/gateway: %s of %s answered %d without a correlation_id", req.Type, playerID, res.Status)
	}
}

// AwaitDelivery returns the first delivery of kind the harness has taken,
// long-polling GET /v1/clients/ci-harness/deliveries until one arrives or
// timeout passes. Every delivery a poll gives is acknowledged at once — the
// gateway leases one delivery per player, and the next one of the player comes
// only after the ack — and the ones of another kind wait in the inbox for a
// later call.
//
// The timeout is wall time counted from the call: the wait of a long-poll runs
// on the wall clock in every mode (C-01 v1.8). A timeout returns ErrNoDelivery
// with the kinds that did arrive.
//
// When the ack fails, the deliveries of that poll stay leased by the gateway
// until the lease runs out, and the error ends the wait; when the gateway
// reports some of them unknown, the others are kept in the inbox all the same.
func (h *HTTPHarness) AwaitDelivery(ctx context.Context, kind string, timeout time.Duration) (api.Delivery, error) {
	return h.await(ctx, kind, timeout, func(d api.Delivery) bool { return d.Kind == kind })
}

// AwaitDeliveryOf is AwaitDelivery restricted to one correlation: the
// deliveries of an action, when the inbox may still hold those of earlier
// ones.
func (h *HTTPHarness) AwaitDeliveryOf(ctx context.Context, correlationID, kind string, timeout time.Duration) (api.Delivery, error) {
	return h.await(ctx, kind+" of "+correlationID, timeout, func(d api.Delivery) bool {
		return d.Kind == kind && d.CorrelationID == correlationID
	})
}

func (h *HTTPHarness) await(ctx context.Context, what string, timeout time.Duration, match func(api.Delivery) bool) (api.Delivery, error) {
	deadline := h.wall.Now().Add(timeout)
	h.polling.Lock()
	defer h.polling.Unlock()
	for {
		if i := slices.IndexFunc(h.inbox, match); i >= 0 {
			d := h.inbox[i]
			h.inbox = slices.Delete(h.inbox, i, i+1)
			return d, nil
		}
		remaining := deadline.Sub(h.wall.Now())
		if remaining <= 0 {
			return api.Delivery{}, fmt.Errorf("testkit/gateway: %w: %s within %s (in the inbox: %v)", ErrNoDelivery, what, timeout, h.inboxKinds())
		}
		if _, err := h.take(ctx, remaining); err != nil {
			return api.Delivery{}, err
		}
	}
}

// Drain takes every delivery the harness has: what waits in the inbox, and
// whatever the long-poll gives until a poll that waited quiet of wall time
// gives nothing. Every delivery is acknowledged, as AwaitDelivery does. The
// deliveries come in the order the gateway gave them.
//
// A quiet of zero or less is a poll with wait_ms=0: Drain returns at the first
// poll that finds nothing ready, without waiting for anything to come.
//
// On success the inbox is empty afterwards. On an error nothing is returned
// and nothing is lost: the deliveries of the earlier polls of the call, and
// those of the failing poll the gateway did acknowledge, stay in the inbox for
// the next Drain or AwaitDelivery; the ones of a poll whose ack failed stay
// leased by the gateway until the lease runs out.
//
// It is how a scenario collects what one step of it produced without naming
// every kind in advance: the quiet wait is the moment the harness believes the
// gateway has nothing more for it, and a test that needs certainty waits for
// the fact behind a delivery on the bus first.
func (h *HTTPHarness) Drain(ctx context.Context, quiet time.Duration) ([]api.Delivery, error) {
	// A negative wait would leave wait_ms out of the request, and the gateway
	// would wait its default instead of not at all.
	quiet = max(quiet, 0)
	h.polling.Lock()
	defer h.polling.Unlock()
	for {
		n, err := h.take(ctx, quiet)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			out := h.inbox
			h.inbox = nil
			return out, nil
		}
	}
}

// take runs one long-poll of at most wait, acknowledges what it gave and puts
// it into the inbox; it returns how many deliveries the poll gave. The caller
// holds polling.
//
// When the ack fails, the deliveries of that poll stay leased by the gateway
// until the lease runs out; when the gateway reports some of them unknown, the
// others are kept in the inbox all the same.
func (h *HTTPHarness) take(ctx context.Context, wait time.Duration) (int, error) {
	// The poll runs under ctx, not under a deadline: a request cancelled
	// halfway may leave a delivery leased to nobody for the lease of the
	// gateway. The gateway answers within the wait instead.
	res, err := h.client.Deliveries(ctx, h.cursor, client.MaxPollLimit, min(wait, client.MaxPollWait))
	if err != nil {
		return 0, fmt.Errorf("testkit/gateway: poll deliveries: %w", err)
	}
	if res.Cursor != "" {
		h.cursor = res.Cursor
	}
	if len(res.Deliveries) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(res.Deliveries))
	for _, d := range res.Deliveries {
		ids = append(ids, d.ID)
	}
	acked, err := h.client.Ack(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("testkit/gateway: ack deliveries: %w", err)
	}
	for _, d := range res.Deliveries {
		if !slices.Contains(acked.Unknown, d.ID) {
			h.inbox = append(h.inbox, d)
		}
	}
	if len(acked.Unknown) > 0 {
		return 0, fmt.Errorf("testkit/gateway: the gateway does not know the deliveries it just gave: %v", acked.Unknown)
	}
	return len(res.Deliveries), nil
}

func (h *HTTPHarness) inboxKinds() []string {
	kinds := make([]string, 0, len(h.inbox))
	for _, d := range h.inbox {
		kinds = append(kinds, d.Kind+"/"+d.CorrelationID)
	}
	return kinds
}

// CloseRound closes the open round of a scope (POST
// /v1/scopes/{scope_id}/rounds/close). In I1 the gateway does not mount the
// route and its mux answers a bare 404; the harness reports that as
// ErrRoundsNotMounted, 501 not_implemented. Every other answer — 409
// no_open_round of the route T-354 mounts included — is returned as the
// gateway gave it, so the stub retires itself with that task.
//
// A bare 404 cannot be told from one of a wrong base URL: the mux of the
// gateway answers an unknown path the way any Go server does. The message of
// the error keeps the answer for that reason; T-354 removes the translation.
func (h *HTTPHarness) CloseRound(ctx context.Context, scopeID string) (api.RoundCloseResponse, error) {
	res, err := h.client.CloseRound(ctx, scopeID)
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound && apiErr.Code == "" {
		return api.RoundCloseResponse{}, roundsNotMounted{answer: apiErr}
	}
	return res, err
}
