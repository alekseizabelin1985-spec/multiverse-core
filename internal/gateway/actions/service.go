package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// KeyStore keeps the answers to actions under their keys (Keys).
type KeyStore interface {
	Lookup(ctx context.Context, playerID, actionKey string, now time.Time) (Stored, bool, error)
	Save(ctx context.Context, playerID, actionKey string, s Stored, now time.Time) error
}

// Config is what the service of actions needs. Every field but Log is
// required.
type Config struct {
	Bus    eventbus.Bus
	Model  Projection
	Keys   KeyStore
	Filter InputFilter
	// FilterName names Filter in the record of each check.
	FilterName string
	Turns      Turns
	Clock      clock.Clock
	// Timers bound the decision of an action by Budget.
	Timers clock.Timers
	Log    *slog.Logger
	// GMPath is MV_GM_PATH: agent, or legacy for the bridge to the legacy
	// orchestrator (gm.created).
	GMPath string
	// Grace is MV_GATEWAY_ENCOUNTER_GRACE.
	Grace time.Duration
	// KeyTTL is how long an answer is kept under its key (store.KeyTTL); a
	// batch left half published is held as long.
	KeyTTL time.Duration
	// Budget bounds the decision of one action once the lock of its player is
	// taken; zero is api.RequestTimeout.
	Budget time.Duration
	// PendingLimit bounds the batches held in memory; zero is
	// DefaultPendingLimit.
	PendingLimit int
}

// Service accepts actions (component §5.5). It is safe for concurrent use: the
// actions of one player are taken one at a time, so that a repeat of an
// action_key sent while the first request is still publishing waits for its
// answer instead of publishing a second action.
type Service struct {
	cfg       Config
	validator Validator
	pending   *pendingBatches

	mu      sync.Mutex
	players map[string]*playerLock
}

// playerLock is a mutex whose wait gives up with the request (a channel of
// one).
type playerLock struct {
	ch    chan struct{}
	users int
}

// errBudget is the cause of the end of a decision that ran out of its budget.
var errBudget = fmt.Errorf("actions: the budget of an action ran out: %w", context.DeadlineExceeded)

// Answer is what the handler writes: Accepted with 202, or Err with its status.
type Answer struct {
	Status   int
	Accepted *api.ActionAccepted
	Err      *api.Error
}

// New returns the service of cfg.
func New(cfg Config) (*Service, error) {
	if cfg.Bus == nil || cfg.Model == nil || cfg.Keys == nil || cfg.Filter == nil || cfg.Turns == nil ||
		cfg.Clock == nil || cfg.Timers == nil {
		return nil, errors.New("actions: Bus, Model, Keys, Filter, Turns, Clock and Timers are required")
	}
	if cfg.GMPath != eventbus.GMPathAgent && cfg.GMPath != eventbus.GMPathLegacy {
		return nil, errors.New("actions: GMPath is agent or legacy")
	}
	if cfg.KeyTTL <= 0 || cfg.Budget < 0 || cfg.PendingLimit < 0 {
		return nil, errors.New("actions: KeyTTL is positive, Budget and PendingLimit are not negative")
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	if cfg.Budget == 0 {
		cfg.Budget = api.RequestTimeout
	}
	if cfg.PendingLimit == 0 {
		cfg.PendingLimit = DefaultPendingLimit
	}
	return &Service{cfg: cfg, validator: Validator{Model: cfg.Model, Grace: cfg.Grace},
		pending: newPendingBatches(cfg.PendingLimit), players: make(map[string]*playerLock)}, nil
}

// Submit takes one action: the answer kept under its key when there is one;
// otherwise validation, the input filter, publication and the answer kept
// under the key. A refusal (4xx) is kept like an acceptance. Nothing is kept
// under the key when the action could not be decided — the bus or a store
// failed — so the client repeats with the same key (C-08).
//
// An action whose publication failed partway is not built again: its batch is
// held in memory, and a repeat of the key publishes the rest of the same
// events (batch). Once the lock of the player is taken, the action is decided
// on a context without the cancellation of the request and with a budget of
// its own: a client that hangs up, or the timeout of the request, does not cut
// a batch in two or leave a published action without its key.
//
// The log of an action never carries its text or its key.
func (s *Service) Submit(ctx context.Context, c Command) Answer {
	if c.ActionKey == "" || len(c.ActionKey) > MaxActionKey {
		return refused(api.NewError(api.CodeInvalidRequest, map[string]any{"field": "action_key"}))
	}
	unlock, err := s.lock(ctx, c.PlayerID)
	if err != nil {
		// The request gave up waiting for an earlier one of its player and
		// decided nothing; a repeat of the key gets the answer the earlier
		// request kept.
		return s.failed(ctx, c, api.CodeBusUnavailable, err)
	}
	defer unlock()
	ctx, release := s.bounded(ctx)
	defer release()

	now := s.cfg.Clock.Now()
	stored, found, err := s.cfg.Keys.Lookup(ctx, c.PlayerID, c.ActionKey, now)
	if err != nil {
		return s.failed(ctx, c, api.CodeInternal, err)
	}
	if found {
		return replay(stored)
	}
	key := batchKey{playerID: c.PlayerID, actionKey: c.ActionKey}
	if b := s.pending.take(key, now); b != nil {
		return s.publish(ctx, c, key, b)
	}

	v, e := s.validator.Validate(c, now)
	if e != nil {
		return s.refuse(ctx, c, e, now)
	}
	text := v.Text
	if v.Rule.Text {
		var fe *api.Error
		if text, fe = s.FilterText(ctx, KindSay, v.Text); fe != nil {
			return s.refuse(ctx, c, fe, now)
		}
	}

	turn := turnOf(c, v.Character, now)
	turn.TargetID, turn.TargetType = targetOf(v)
	if v.Rule.Text {
		turn.TextLen = utf8.RuneCountInString(text)
	}
	ref, err := s.cfg.Turns.Begin(ctx, turn)
	if err != nil {
		code := api.CodeInternal
		if errors.Is(err, ErrBusUnavailable) {
			code = api.CodeBusUnavailable
		}
		return s.failed(ctx, c, code, err)
	}
	b := &batch{turn: turn, ref: ref, expires: now.Add(s.cfg.KeyTTL)}
	if s.cfg.GMPath == eventbus.GMPathLegacy {
		b.events = append(b.events, BuildGMCreated(v))
	}
	action := BuildPlayerEvent(v, text, &ref, s.cfg.GMPath)
	b.action = len(b.events)
	b.events = append(b.events, action)
	switch c.Type {
	case api.ActionEnter:
		b.events = append(b.events, BuildPositionProposal(action, v.Character, v.Region.ID))
	case api.ActionLeave:
		b.events = append(b.events, BuildPositionProposal(action, v.Character, OutsidePosition(v.Character.WorldID)))
	case api.ActionRest:
		b.events = append(b.events, BuildRestProposal(action, v.Character))
	}
	return s.publish(ctx, c, key, b)
}

// publish publishes the events of b the bus has not acknowledged yet and keeps
// 202 under the key. A publication that fails holds the batch from the event
// that failed: that event may have reached the broker without its
// acknowledgement, and its repeat carries the same id.
//
// The turn is recorded before the player.* event goes out, not after the
// batch (component §5.5, Turn before Bus.Publish). The bus delivers
// asynchronously, and the mechanics and the narrative of the action can reach
// the consumer while the last event of the batch is still being published: a
// turn written after them would never see its narrative and would wait for its
// deadline (review #1 of T-308, Mi-1). An action event the bus did not
// acknowledge takes its turn back, so a client told to repeat leaves no turn
// behind; an event after it that fails leaves the turn, because the action is
// out and its narrative is coming.
func (s *Service) publish(ctx context.Context, c Command, key batchKey, b *batch) Answer {
	action := b.events[b.action]
	for ; b.sent < len(b.events); b.sent++ {
		if b.sent == b.action {
			if err := s.cfg.Turns.Accepted(ctx, b.turn, b.ref, action.ID); err != nil {
				s.hold(ctx, key, b)
				return s.failed(ctx, c, api.CodeInternal, err)
			}
		}
		if err := s.cfg.Bus.Publish(ctx, b.events[b.sent]); err != nil {
			if b.sent == b.action {
				// The budget of the decision may be what failed the publication.
				if werr := s.cfg.Turns.Withdrawn(context.WithoutCancel(ctx), b.ref, action.ID); werr != nil {
					s.logFailure(ctx, c, "turn of an unpublished action not taken back", werr)
				}
			}
			s.hold(ctx, key, b)
			return s.failed(ctx, c, api.CodeBusUnavailable, err)
		}
	}

	acked := s.cfg.Clock.Now()
	accepted := api.ActionAccepted{CorrelationID: action.ID, Turn: b.ref, Status: api.ActionStatusAccepted, AckedAt: acked}
	// The action is out: the moment of its acknowledgement or a key that fails
	// to record now is logged, not answered, because a client told to repeat
	// would publish it twice. Both are recorded without the cancellation and
	// the budget of the decision: a budget that runs out after the last
	// publication must not leave 202 without its key, or the repeat of the key
	// would build the action again. The store bounds the write by its busy
	// timeout.
	out := context.WithoutCancel(ctx)
	if err := s.cfg.Turns.Acked(out, action.ID, acked); err != nil {
		s.logFailure(ctx, c, "acknowledgement of an accepted action not recorded", err)
	}
	s.keep(out, c, action.ID, http.StatusAccepted, accepted, acked)
	return Answer{Status: http.StatusAccepted, Accepted: &accepted}
}

// hold keeps a batch that is not fully published under its key.
func (s *Service) hold(ctx context.Context, key batchKey, b *batch) {
	if evicted, ok := s.pending.hold(key, b); ok {
		s.cfg.Log.LogAttrs(ctx, slog.LevelWarn, "half published action dropped from memory at the limit",
			slog.String("player_id", evicted.playerID), slog.Int("limit", s.cfg.PendingLimit))
	}
}

// SweepPending drops the half published batches whose key expired by now; the
// sweeper of the context calls it.
func (s *Service) SweepPending(now time.Time) int { return s.pending.sweep(now) }

// Pending is the number of half published batches held in memory.
func (s *Service) Pending() int { return s.pending.len() }

// bounded is ctx without its cancellation and with the budget of a decision,
// counted on the timers of the service. release frees the timer.
func (s *Service) bounded(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	timer := s.cfg.Timers.After(s.cfg.Budget)
	done := make(chan struct{})
	go func() {
		select {
		case <-timer.C():
			cancel(errBudget)
		case <-done:
		}
	}()
	return ctx, func() {
		close(done)
		timer.Stop()
		cancel(nil)
	}
}

// FilterText passes the text of a player through the input filter: the text of
// say and the name of a new character (kind KindSay or KindName). It returns
// the text that goes to the bus. A failure of the filter is 422 filter_error
// (fail-closed, FR-056); a text the filter blocked, or replaced by one that is
// no longer a valid text of say, is 400 text_invalid.
//
// The result is checked by the rules of say (1 to MaxTextRunes characters)
// whatever the kind. A caller with a name checks the result again by the rules
// of a name (api-contracts.md §1.6: 2 to 32 characters) and maps a failure to
// name_invalid or name_required itself.
//
// Every check is recorded as input_filter{applied, status, filter, text_len},
// without the text.
func (s *Service) FilterText(ctx context.Context, kind, text string) (string, *api.Error) {
	res, err := s.cfg.Filter.Check(ctx, kind, text)
	attrs := []slog.Attr{
		slog.Bool("applied", err == nil),
		slog.String("filter", s.cfg.FilterName),
		slog.String("kind", kind),
		slog.Int("text_len", utf8.RuneCountInString(text)),
	}
	if err != nil {
		// The error of a filter is not written: a filter of the operator may
		// quote the text it failed on.
		s.cfg.Log.LogAttrs(ctx, slog.LevelError, "input_filter", append(attrs, slog.String("status", "error"))...)
		return "", api.NewError(api.CodeFilterError, nil)
	}
	s.cfg.Log.LogAttrs(ctx, slog.LevelInfo, "input_filter", append(attrs, slog.String("status", res.Status))...)
	if res.Status == FilterBlocked {
		return "", api.NewError(api.CodeTextInvalid, map[string]any{"filter": FilterBlocked})
	}
	out, ok := CheckText(&res.Text)
	if !ok {
		return "", api.NewError(api.CodeTextInvalid, nil)
	}
	return out, nil
}

// refuse answers a refused action, keeping a 4xx under its key.
func (s *Service) refuse(ctx context.Context, c Command, e *api.Error, now time.Time) Answer {
	if e.Status >= http.StatusBadRequest && e.Status < http.StatusInternalServerError {
		ch, _ := s.cfg.Model.Character(c.PlayerID)
		if err := s.cfg.Turns.Rejected(ctx, turnOf(c, ch, now), e.Code); err != nil {
			s.logFailure(ctx, c, "turn of a refused action not recorded", err)
		}
		s.keep(ctx, c, "", e.Status, e.Body(), now)
	}
	return refused(e)
}

func (s *Service) keep(ctx context.Context, c Command, correlationID string, status int, body any, now time.Time) {
	raw, err := json.Marshal(body)
	if err == nil {
		err = s.cfg.Keys.Save(ctx, c.PlayerID, c.ActionKey, Stored{CorrelationID: correlationID, Status: status, Body: raw}, now)
	}
	if err != nil {
		s.logFailure(ctx, c, "answer to an action not kept under its key", err)
	}
}

// failed answers an action that was not decided, with nothing kept.
func (s *Service) failed(ctx context.Context, c Command, code string, err error) Answer {
	s.logFailure(ctx, c, "action not accepted", err)
	return refused(api.NewError(code, nil))
}

func (s *Service) logFailure(ctx context.Context, c Command, msg string, err error) {
	s.cfg.Log.LogAttrs(ctx, slog.LevelError, msg,
		slog.String("request_id", api.RequestIDFrom(ctx)),
		slog.String("player_id", c.PlayerID),
		slog.String("action", actionName(c.Type)),
		slog.Bool("handled", true),
		slog.String("error", err.Error()))
}

// actionName is the type of an action for the log: a type outside the
// dictionary is whatever the client sent, and is not written.
func actionName(typ string) string {
	if _, ok := Rules[typ]; ok {
		return typ
	}
	return "unknown"
}

func refused(e *api.Error) Answer { return Answer{Status: e.Status, Err: e} }

// replay turns a kept answer back into the answer: the same status and body.
// A body that does not decode any more is a defect of the store.
func replay(s Stored) Answer {
	if s.Status == http.StatusAccepted {
		var accepted api.ActionAccepted
		if err := json.Unmarshal(s.Body, &accepted); err == nil {
			return Answer{Status: s.Status, Accepted: &accepted}
		}
		return refused(api.NewError(api.CodeInternal, nil))
	}
	var body api.ErrorResponse
	if err := json.Unmarshal(s.Body, &body); err != nil {
		return refused(api.NewError(api.CodeInternal, nil))
	}
	return refused(&api.Error{Status: s.Status, Code: body.Error.Code, Message: body.Error.Message, Details: body.Error.Details})
}

// turnOf is the turn of an action of a character; the character is the zero
// value when the projection does not know it, and the turn has no scope.
func turnOf(c Command, ch readmodel.CharacterState, now time.Time) Turn {
	t := Turn{WorldID: ch.WorldID, PlayerID: c.PlayerID, Name: ch.Name, Type: c.Type, ActorKind: c.ActorKind, At: now}
	if ch.ID != "" {
		t.Scope = *scopeOf(ch)
	}
	return t
}

// targetOf is the entity a validated action aims at, as its event names it.
func targetOf(v Validated) (id, typ string) {
	switch {
	case v.NPC != nil && v.Command.Type == api.ActionAttack:
		return v.NPC.ID, entity.TypeNPC
	case v.Region != nil && (v.Command.Type == api.ActionEnter || v.Command.Type == api.ActionLeave):
		return v.Region.ID, entity.TypeRegion
	}
	return "", ""
}

// lock takes the lock of one player and returns its release; the entry goes
// with its last user. A request that ends while it waits gets the error of its
// context and holds nothing.
func (s *Service) lock(ctx context.Context, playerID string) (func(), error) {
	s.mu.Lock()
	l := s.players[playerID]
	if l == nil {
		l = &playerLock{ch: make(chan struct{}, 1)}
		s.players[playerID] = l
	}
	l.users++
	s.mu.Unlock()
	leave := func() {
		s.mu.Lock()
		if l.users--; l.users == 0 {
			delete(s.players, playerID)
		}
		s.mu.Unlock()
	}
	// A free lock is taken even by a request that has just ended: the choice
	// of select between two ready cases is random.
	select {
	case l.ch <- struct{}{}:
	default:
		select {
		case l.ch <- struct{}{}:
		case <-ctx.Done():
			leave()
			return nil, fmt.Errorf("actions: wait for the lock of the player: %w", context.Cause(ctx))
		}
	}
	return func() {
		<-l.ch
		leave()
	}, nil
}
