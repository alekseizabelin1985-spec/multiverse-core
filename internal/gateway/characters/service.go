// Package characters creates the characters of the players and answers where
// a character stands (component gateway-and-bot.md §5.2, §7.1; C-08, US-001).
//
// A character is created by a proposal to State, never by the gateway itself
// (C-02): POST /v1/characters publishes entity.create.proposed and waits for
// its fact up to MV_GATEWAY_CHARACTER_WAIT. The fact in time answers 201; a
// fact that is late answers 202 creating, and the character stays in
// pending_characters until its fact comes or MV_GATEWAY_CHARACTER_DEADLINE
// passes. A character past its deadline, or refused by State, is taken off its
// link, and the next request creates a new player_id.
//
// The answer is kept under (link_id, action_key) in links.db, which /forget
// deletes with the link. gateway.db holds the player_id of a pending character
// and never the link or the external account behind it (SEC-03).
package characters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Defaults of MV_GATEWAY_CHARACTER_WAIT and MV_GATEWAY_CHARACTER_DEADLINE
// (component §11.3).
const (
	DefaultWait     = readmodel.DefaultFactWait
	DefaultDeadline = 60 * time.Second
)

// StatusReadTimeout bounds the reading of pending_characters in Status: the
// deadline of an ordinary request (api.RequestTimeout).
const StatusReadTimeout = api.RequestTimeout

// TypeCreateProposed is the proposal a character is created by (C-02).
const TypeCreateProposed = "entity.create.proposed"

// CauseCreate is the cause of the proposal (C-02, contracts.OwnershipRules).
const CauseCreate = "create"

// Statuses of a character in the answers: the statuses of State and creating,
// the state between the proposal and its fact (C-08 v1.1).
const (
	StatusNone     = "none"
	StatusCreating = "creating"
	StatusAlive    = entity.StatusAlive
	StatusDead     = entity.StatusDead
)

// Links is what the service uses of the links store.
type Links interface {
	ByExternal(ctx context.Context, platform, externalID string) (links.Link, bool, error)
	ByPlayer(ctx context.Context, playerID string) (links.Link, bool, error)
	AttachPlayer(ctx context.Context, linkID, playerID, worldID string) error
	DetachPlayer(ctx context.Context, playerID string) (bool, error)
	CharacterRequest(ctx context.Context, linkID, actionKey string, now time.Time) (links.StoredResponse, bool, error)
	SaveCharacterRequest(ctx context.Context, linkID, actionKey string, r links.StoredResponse, now time.Time) error
}

// Projection is what the service reads of the read model: the world of a view
// and the wait for the fact of a proposal.
type Projection interface {
	World
	Expect(correlationID string, timeout time.Duration, proposalIDs ...string) *readmodel.Waiter
}

// NameFilter passes a name through the input filter (actions.Service).
type NameFilter interface {
	FilterText(ctx context.Context, kind, text string) (string, *api.Error)
}

// Config builds the service. Every field but Sessions and Log is required.
type Config struct {
	Links Links
	Model Projection
	// Sessions gives a view its session; nil leaves the session out.
	Sessions Sessions
	DB       *sql.DB
	Bus      eventbus.Bus
	Filter   NameFilter
	IDs      links.IDSource
	Clock    clock.Clock
	// Wait is MV_GATEWAY_CHARACTER_WAIT, Deadline MV_GATEWAY_CHARACTER_DEADLINE.
	Wait, Deadline time.Duration
	Log            *slog.Logger
}

// Request is a request to create a character. ActorKind is X-Actor-Kind.
type Request struct {
	Platform, ExternalID, WorldID, Name, ActionKey, ActorKind string
}

// Answer is what the handler writes: Body with Status, or Err.
type Answer struct {
	Status int
	Body   *api.CreateCharacterResponse
	Err    *api.Error
}

// Service is the creation of characters. It is safe for concurrent use: the
// requests of one link are taken one at a time, so that a repeat of a key sent
// while the first request still waits for its fact gets the first answer.
type Service struct {
	cfg    Config
	viewer Viewer
	// statusTimeout is StatusReadTimeout unless a test shortened it.
	statusTimeout time.Duration

	mu    sync.Mutex
	locks map[string]*linkLock
}

type linkLock struct {
	ch    chan struct{}
	users int
}

// New returns the service of cfg.
func New(cfg Config) (*Service, error) {
	if cfg.Links == nil || cfg.Model == nil || cfg.DB == nil || cfg.Bus == nil || cfg.Filter == nil ||
		cfg.IDs == nil || cfg.Clock == nil {
		return nil, errors.New("characters: Links, Model, DB, Bus, Filter, IDs and Clock are required")
	}
	if cfg.Wait <= 0 || cfg.Deadline <= 0 {
		return nil, errors.New("characters: Wait and Deadline are positive")
	}
	if cfg.Wait >= cfg.Deadline {
		return nil, errors.New("characters: Wait is less than Deadline")
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{cfg: cfg, viewer: Viewer{World: cfg.Model, Sessions: cfg.Sessions}, statusTimeout: StatusReadTimeout,
		locks: make(map[string]*linkLock)}, nil
}

// Create serves POST /v1/characters (component §7.1). The checks, in order:
// the request (invalid_request), the consent of the link (consent_required),
// the answer kept under the key, the character the link already has (200 for a
// living one, the proposal again for one still creating), the world
// (world_not_found), the name (name_required, name_invalid, filter_error).
// Only a created, existing or creating character is kept under the key: a
// refusal publishes nothing and a repeat checks again.
func (s *Service) Create(ctx context.Context, r Request) Answer {
	if e := checkRequest(r); e != nil {
		return refused(e)
	}
	link, found, err := s.cfg.Links.ByExternal(ctx, r.Platform, r.ExternalID)
	if err != nil {
		return s.internal(ctx, "link of a character request not read", err)
	}
	if !found || link.Status != links.StatusConsented {
		return refused(api.NewError(api.CodeConsentRequired, nil))
	}
	unlock, err := s.lock(ctx, link.LinkID)
	if err != nil {
		return refused(api.NewError(api.CodeBusUnavailable, nil))
	}
	defer unlock()

	now := s.cfg.Clock.Now()
	stored, found, err := s.cfg.Links.CharacterRequest(ctx, link.LinkID, r.ActionKey, now)
	if err != nil {
		return s.internal(ctx, "kept answer of a character request not read", err)
	}
	if found {
		return replay(stored)
	}
	// The link as it is now: an earlier request of the same account may have
	// attached a character while this one waited for the lock.
	if link, found, err = s.cfg.Links.ByExternal(ctx, r.Platform, r.ExternalID); err != nil || !found {
		if err == nil {
			return refused(api.NewError(api.CodeConsentRequired, nil))
		}
		return s.internal(ctx, "link of a character request not read", err)
	}
	if link.PlayerID != nil {
		if a, done := s.existing(ctx, link, r, *link.PlayerID, now); done {
			return a
		}
	}

	world, ok := s.cfg.Model.World(r.WorldID)
	if !ok {
		return refused(api.NewError(api.CodeWorldNotFound, nil))
	}
	name, e := s.name(ctx, r.Name)
	if e != nil {
		return refused(e)
	}
	playerID, err := links.NewPlayerID(s.cfg.IDs)
	if err != nil {
		return s.internal(ctx, "player_id not assigned", err)
	}
	p := Pending{PlayerID: playerID, WorldID: world.ID, Name: name, ActorKind: actorKind(r.ActorKind),
		CreatedAt: now, DeadlineAt: now.Add(s.cfg.Deadline)}
	ev := Proposal(p, "")
	p.ProposalID = ev.ID
	if err := insertPending(ctx, s.cfg.DB, p); err != nil {
		return s.internal(ctx, "pending character not recorded", err)
	}
	if err := s.cfg.Links.AttachPlayer(ctx, link.LinkID, playerID, world.ID); err != nil {
		_ = deletePending(context.WithoutCancel(ctx), s.cfg.DB, p.ProposalID)
		return s.internal(ctx, "character not attached to its link", err)
	}
	return s.propose(ctx, link, r.ActionKey, p, ev)
}

// existing answers a request of a link that already has a character: 200 for
// a living one, the proposal again for one still creating. A dead, abandoned
// or forgotten character is no answer (done is false): the request creates a
// new one (UC-002 A2).
func (s *Service) existing(ctx context.Context, link links.Link, r Request, playerID string, now time.Time) (Answer, bool) {
	if ch, ok := s.cfg.Model.Character(playerID); ok {
		if entity.IsTerminalStatus(ch.Status) {
			return Answer{}, false
		}
		created := false
		body := &api.CreateCharacterResponse{PlayerID: playerID, Character: s.viewer.View(ctx, ch), Created: &created}
		return s.keep(ctx, link, r.ActionKey, http.StatusOK, body, now), true
	}
	p, found, err := pendingOf(ctx, s.cfg.DB, playerID)
	if err != nil {
		return s.internal(ctx, "pending character not read", err), true
	}
	if !found || !p.DeadlineAt.After(now) {
		return Answer{}, false
	}
	// The proposal is published again under its proposal_id, which State
	// applies once (C-02 v1.6): the first publication may have failed, or its
	// fact may be about to come.
	return s.propose(ctx, link, r.ActionKey, p, Proposal(p, p.ProposalID)), true
}

// propose publishes the proposal of p and waits for its fact.
func (s *Service) propose(ctx context.Context, link links.Link, actionKey string, p Pending, ev eventbus.Event) Answer {
	w := s.cfg.Model.Expect(ev.ID, s.cfg.Wait, p.ProposalID)
	if err := s.cfg.Bus.Publish(ctx, ev); err != nil {
		w.Cancel()
		s.logFailure(ctx, "character proposal not published", err)
		return refused(api.NewError(api.CodeBusUnavailable, nil))
	}
	_, err := w.Wait(ctx)
	out := context.WithoutCancel(ctx)
	now := s.cfg.Clock.Now()
	if errors.Is(err, readmodel.ErrRejected) {
		// A clearing that fails is logged by clear and left to the sweeper:
		// the refusal expired nothing in gateway.db, so the deadline does it.
		_ = s.clear(out, p, "character proposal rejected by state")
		return refused(api.NewError(api.CodeInternal, nil))
	}
	// Whatever ended the wait, the projection decides: the fact of an earlier
	// publication of the same proposal carries its own correlation and does
	// not end this wait, and a wait past its time may meet a fact just applied.
	if ch, ok := s.cfg.Model.Character(p.PlayerID); ok {
		if err := deletePending(out, s.cfg.DB, p.ProposalID); err != nil {
			s.logFailure(ctx, "pending character not removed", err)
		}
		created := true
		return s.keep(out, link, actionKey, http.StatusCreated,
			&api.CreateCharacterResponse{PlayerID: p.PlayerID, Character: s.viewer.View(out, ch), Created: &created}, now)
	}
	return s.keep(out, link, actionKey, http.StatusAccepted,
		&api.CreateCharacterResponse{PlayerID: p.PlayerID, Status: StatusCreating}, now)
}

// keep keeps the answer under the key and returns it. An answer that cannot be
// kept is still the answer: a repeat of the key finds the character through
// the link.
func (s *Service) keep(ctx context.Context, link links.Link, actionKey string, status int, body *api.CreateCharacterResponse, now time.Time) Answer {
	raw, err := json.Marshal(body)
	if err == nil {
		err = s.cfg.Links.SaveCharacterRequest(ctx, link.LinkID, actionKey,
			links.StoredResponse{PlayerID: body.PlayerID, StatusCode: status, Body: raw}, now)
	}
	if err != nil {
		s.logFailure(ctx, "answer to a character request not kept", err)
	}
	return Answer{Status: status, Body: body}
}

// name checks the name of a new character, passes it through the input filter
// and checks the result again by the rules of a name (N-1 of T-305).
func (s *Service) name(ctx context.Context, raw string) (string, *api.Error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", api.NewError(api.CodeNameRequired, nil)
	}
	if !api.ValidCharacterName(name) {
		return "", api.NewError(api.CodeNameInvalid, nil)
	}
	filtered, e := s.cfg.Filter.FilterText(ctx, actions.KindName, name)
	switch {
	case e != nil && e.Code == api.CodeFilterError:
		return "", e
	case e != nil:
		return "", api.NewError(api.CodeNameInvalid, e.Details)
	}
	filtered = strings.TrimSpace(filtered)
	if filtered == "" {
		return "", api.NewError(api.CodeNameRequired, nil)
	}
	if !api.ValidCharacterName(filtered) {
		return "", api.NewError(api.CodeNameInvalid, nil)
	}
	return filtered, nil
}

// Status is the status of the character of a link for POST /v1/links/resolve:
// alive or dead from the projection (abandoned reads as dead), creating while
// its proposal waits before its deadline, none otherwise.
//
// The reading of pending_characters has a deadline of its own, StatusReadTimeout:
// the only connection of gateway.db may be held by a transaction that waits,
// and the answer to resolve does not wait with it (acceptance of T-306, Mi-2).
// A reading that fails or runs out answers creating, as before.
func (s *Service) Status(playerID string) string {
	if ch, ok := s.cfg.Model.Character(playerID); ok {
		if entity.IsTerminalStatus(ch.Status) {
			return StatusDead
		}
		return StatusAlive
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.statusTimeout)
	defer cancel()
	p, found, err := pendingOf(ctx, s.cfg.DB, playerID)
	if err != nil {
		s.logFailure(ctx, "pending character not read", err)
		return StatusCreating
	}
	if found && p.DeadlineAt.After(s.cfg.Clock.Now()) {
		return StatusCreating
	}
	return StatusNone
}

// Player serves GET /v1/players/{player_id}: the character the projection
// holds, the minimal state of one still creating, or player_not_found.
func (s *Service) Player(ctx context.Context, playerID string) (*api.CharacterState, *api.Error) {
	if ch, ok := s.cfg.Model.Character(playerID); ok {
		return s.viewer.View(ctx, ch), nil
	}
	p, found, err := s.Creating(ctx, playerID)
	switch {
	case err != nil:
		s.logFailure(ctx, "pending character not read", err)
		return nil, api.NewError(api.CodeInternal, nil)
	case found:
		return Creating(p), nil
	}
	return nil, api.NewError(api.CodePlayerNotFound, nil)
}

// Creating returns the pending character of a player that is still before its
// deadline.
func (s *Service) Creating(ctx context.Context, playerID string) (Pending, bool, error) {
	p, found, err := pendingOf(ctx, s.cfg.DB, playerID)
	if err != nil || !found || !p.DeadlineAt.After(s.cfg.Clock.Now()) {
		return Pending{}, false, err
	}
	return p, true, nil
}

// Sweep takes off their links the characters whose deadline passed by now
// without a fact, and forgets their proposals; a character whose fact did come
// only loses its row. It returns how many rows it removed.
func (s *Service) Sweep(ctx context.Context, now time.Time) (int, error) {
	list, err := expired(ctx, s.cfg.DB, now)
	if err != nil {
		return 0, err
	}
	var errs []error
	removed := 0
	for _, p := range list {
		if err := s.expire(ctx, p); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}

// expire removes the row of a proposal past its deadline. A character the
// projection holds only loses its row, and its link is not touched.
//
// A character without a fact is taken off its link under the lock of the link,
// so that no request of the link attaches another character in between, and
// the projection is read again after the detachment (Mi-3 of review #1 of
// T-306). The fact is applied to the projection by the consumer, outside any
// lock the sweeper can take: a fact applied between the first reading and the
// detachment is seen by the second reading, and the character goes back on its
// link. That second reading is the moment that decides: a fact at the deadline
// never leaves a living character off its link (A-7), and a fact applied after
// it is a fact past the deadline.
func (s *Service) expire(ctx context.Context, p Pending) error {
	if _, ok := s.cfg.Model.Character(p.PlayerID); ok {
		return deletePending(ctx, s.cfg.DB, p.ProposalID)
	}
	link, found, err := s.cfg.Links.ByPlayer(ctx, p.PlayerID)
	if err != nil {
		s.logFailure(ctx, "link of an expired character not read", err)
		return err
	}
	if !found {
		// No link names the character any longer: /forget deleted it, or the
		// link has a new character. Only the row is left.
		return s.forget(ctx, p, "character creation expired")
	}
	unlock, err := s.lock(ctx, link.LinkID)
	if err != nil {
		return err
	}
	defer unlock()
	detached, err := s.cfg.Links.DetachPlayer(ctx, p.PlayerID)
	if err != nil {
		s.logFailure(ctx, "character not detached from its link", err)
		return err
	}
	if _, ok := s.cfg.Model.Character(p.PlayerID); ok {
		if detached {
			if err := s.cfg.Links.AttachPlayer(ctx, link.LinkID, p.PlayerID, p.WorldID); err != nil && !errors.Is(err, links.ErrNotFound) {
				s.logFailure(ctx, "character created at its deadline not attached back to its link", err)
				return err
			}
		}
		s.cfg.Log.LogAttrs(ctx, slog.LevelInfo, "character created at its deadline kept on its link",
			slog.String("player_id", p.PlayerID), slog.String("proposal_id", p.ProposalID), slog.Bool("handled", true))
		return deletePending(ctx, s.cfg.DB, p.ProposalID)
	}
	return s.forget(ctx, p, "character creation expired")
}

// clear takes a character that will not be created off its link and forgets
// its proposal. The link goes first: a pending row without a link is swept
// again, a link without a pending row would point at nothing for good.
func (s *Service) clear(ctx context.Context, p Pending, msg string) error {
	if _, err := s.cfg.Links.DetachPlayer(ctx, p.PlayerID); err != nil {
		s.logFailure(ctx, "character not detached from its link", err)
		return err
	}
	return s.forget(ctx, p, msg)
}

// forget removes the row of a character that is off its link and logs why.
func (s *Service) forget(ctx context.Context, p Pending, msg string) error {
	if err := deletePending(ctx, s.cfg.DB, p.ProposalID); err != nil {
		s.logFailure(ctx, "pending character not removed", err)
		return err
	}
	s.cfg.Log.LogAttrs(ctx, slog.LevelWarn, msg, slog.String("player_id", p.PlayerID),
		slog.String("proposal_id", p.ProposalID), slog.Bool("handled", true))
	return nil
}

// OnCreated is the effect of entity.created on the consumer: the proposal of a
// character that got its fact is forgotten.
func (s *Service) OnCreated(ctx context.Context, tx *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
	proposalID, _ := ev.Path().GetString("proposal_id")
	if proposalID == "" {
		return nil
	}
	return deletePending(ctx, tx, proposalID)
}

// OnRejected is the effect of entity.update.rejected on the consumer: the
// proposal of a character State refused reaches its deadline at once, so that
// it reads as none right away and the sweeper takes it off its link. The
// effect writes gateway.db alone, through the transaction of the consumer.
func (s *Service) OnRejected(ctx context.Context, tx *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
	proposalID, _ := ev.Path().GetString("proposal_id")
	if proposalID == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pending_characters SET deadline_at = ? WHERE proposal_id = ? AND deadline_at > ?`,
		formatTime(s.cfg.Clock.Now()), proposalID, formatTime(s.cfg.Clock.Now())); err != nil {
		return fmt.Errorf("characters: expire the rejected proposal: %w", err)
	}
	return nil
}

// Proposal is entity.create.proposed of a pending character (C-02, component
// §7.1): a root event whose id is the proposal_id of a first publication, or
// the proposal_id given for a repeat. The attributes are those of a new
// character of data-model.md §3.3.
func Proposal(p Pending, proposalID string) eventbus.Event {
	ev := eventbus.NewRoot(TypeCreateProposed, contracts.SourceGateway, p.WorldID,
		&eventbus.ScopeRef{ID: p.PlayerID, Type: "solo"}, p.ActorKind, map[string]any{
			"entity":     map[string]any{"entity": map[string]any{"id": p.PlayerID, "type": entity.TypePlayer}, "name": p.Name},
			"attributes": Attributes(p),
			"cause":      CauseCreate,
		})
	if proposalID == "" {
		proposalID = ev.ID
	}
	ev.Payload["proposal_id"] = proposalID
	return ev
}

func checkRequest(r Request) *api.Error {
	switch {
	case r.Platform != links.PlatformTelegram:
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "external_platform"})
	case r.ExternalID == "":
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "external_id"})
	case r.ActionKey == "" || len(r.ActionKey) > actions.MaxActionKey:
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "action_key"})
	}
	return nil
}

func actorKind(k string) string {
	if k == "" {
		return eventbus.ActorHuman
	}
	return k
}

func refused(e *api.Error) Answer { return Answer{Status: e.Status, Err: e} }

func (s *Service) internal(ctx context.Context, msg string, err error) Answer {
	s.logFailure(ctx, msg, err)
	return refused(api.NewError(api.CodeInternal, nil))
}

// logFailure writes request_id and the error; the operation is nolog, so the
// log never gets the body, the external account or the name.
func (s *Service) logFailure(ctx context.Context, msg string, err error) {
	s.cfg.Log.LogAttrs(ctx, slog.LevelError, msg, slog.String("request_id", api.RequestIDFrom(ctx)),
		slog.Bool("handled", true), slog.String("error", err.Error()))
}

// replay turns a kept answer back into the answer.
func replay(stored links.StoredResponse) Answer {
	var body api.CreateCharacterResponse
	if err := json.Unmarshal(stored.Body, &body); err != nil {
		return refused(api.NewError(api.CodeInternal, nil))
	}
	return Answer{Status: stored.StatusCode, Body: &body}
}

// lock takes the lock of one link; a request that ends while it waits holds
// nothing.
func (s *Service) lock(ctx context.Context, linkID string) (func(), error) {
	s.mu.Lock()
	l := s.locks[linkID]
	if l == nil {
		l = &linkLock{ch: make(chan struct{}, 1)}
		s.locks[linkID] = l
	}
	l.users++
	s.mu.Unlock()
	leave := func() {
		s.mu.Lock()
		if l.users--; l.users == 0 {
			delete(s.locks, linkID)
		}
		s.mu.Unlock()
	}
	select {
	case l.ch <- struct{}{}:
	default:
		select {
		case l.ch <- struct{}{}:
		case <-ctx.Done():
			leave()
			return nil, fmt.Errorf("characters: wait for the lock of the link: %w", ctx.Err())
		}
	}
	return func() {
		<-l.ch
		leave()
	}, nil
}
