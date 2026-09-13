package llm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
)

// typeLLMOutput is the record of one call of the model (C-07), the event the
// budget counts.
const typeLLMOutput = "llm.output"

// The kinds of a budget_exceeded refusal this package enforces: the values of
// llm.output.rejected.budget.kind. The cloud kind, a limit in money, comes with
// T-250.
const (
	BudgetKindBackground = "background"
	BudgetKindTurn       = "turn"
)

// The windows of the limits (КД EPIC-003 §9.3) and their spelling in
// llm.output.rejected.budget.window.
const (
	BackgroundWindow = time.Hour
	TurnWindow       = time.Minute

	backgroundWindowName = "1h"
	turnWindowName       = "1m"
)

// The levels of meta.agent.level the limits select on.
const (
	levelGlobal = "global"
	levelDomain = "domain"
	levelTask   = "task"
)

// BudgetKey is the window a call is counted in: (world, level, phase,
// provider), КД EPIC-003 §9.3. A limit may cover several keys at once — the
// background limit sums the global and the domain level and every provider of
// a world — so the key is what is recorded, and the rule decides what is summed.
type BudgetKey struct {
	World string
	// Level is meta.agent.level of the calling agent: global, domain, task.
	Level    string
	Phase    Phase
	Provider string
}

func (k BudgetKey) check() error {
	var missing []string
	if k.World == "" {
		missing = append(missing, "world")
	}
	if k.Level == "" {
		missing = append(missing, "level")
	}
	if !k.Phase.Valid() {
		missing = append(missing, "phase")
	}
	if k.Provider == "" {
		missing = append(missing, "provider")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s missing or invalid", ErrBudgetKey, strings.Join(missing, ", "))
	}
	return nil
}

// ErrBudgetKey is a call or a record the budget cannot place in a window. It
// is a defect of the caller or of the publisher, not a refusal: it does not
// match ErrBudget.
var ErrBudgetKey = errors.New("llm: budget key incomplete")

// BudgetLimits are the caps of the windows.
type BudgetLimits struct {
	// BackgroundCallsPerHour is B, budget.background_calls_per_hour_world of the
	// global-gm blueprint: tick calls of the global and domain levels of one
	// world in a sliding hour. It is a Must limit, so zero is a cap of zero —
	// no background call at all — and not "off".
	BackgroundCallsPerHour int
	// TurnCallsPerMinute caps narrative calls of task agents of one world in a
	// sliding minute. It is a Should limit, off by default: zero switches it off.
	// MV_LLM_TURN_CALLS_PER_MIN is read by T-250, which wires it here.
	TurnCallsPerMinute int
}

// BudgetExceededError is a refusal of the budget: the call was not made. It
// matches ErrBudget; Limit is the budget{kind, limit, window} of
// llm.output.rejected.
type BudgetExceededError struct {
	Key   BudgetKey
	Limit BudgetLimit
	// Calls is how many calls the window held when the call was refused.
	Calls int
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("llm: budget exceeded: %s limit %d per %s reached (%d calls) for world %s, level %s, phase %s",
		e.Limit.Kind, e.Limit.Limit, e.Limit.Window, e.Calls, e.Key.World, e.Key.Level, e.Key.Phase)
}

// Is makes the refusal match ErrBudget, the sentinel the swarm falls back on.
func (e *BudgetExceededError) Is(target error) bool { return target == ErrBudget }

// BudgetCall is one call held in the windows.
type BudgetCall struct {
	EventID string
	Key     BudgetKey
	// At is the timestamp of the llm.output. The record derives it from its
	// cause (eventbus.Derive), so a catch-up reads the same instant the live
	// run counted.
	At time.Time
}

// Budget holds the sliding windows of the LLM calls of the process and says
// whether one more call fits (КД EPIC-003 §9.3, §7.3; FR-071, FR-125).
//
// The windows are filled by the llm.output records alone, live and during a
// catch-up alike, so the two cannot disagree: a record is one call, whatever
// its validation_status, because the provider was asked either way.
//
// Counting is idempotent by the id of the record. The bus may deliver the same
// llm.output twice — a redelivery after another recipient failed, or the
// catch-up reading what the live run already counted (C-01 v1.6, "посредник
// доставки") — and a second count would move the background cap in a live run.
//
// Allow and Observe are separate steps, so two callers may both pass a window
// that has room for one: the gateway is served by one LLM worker (КД §7.1).
type Budget struct {
	clock  clock.Clock
	limits BudgetLimits

	mu    sync.Mutex
	calls map[string]BudgetCall // by event id
}

// NewBudget returns empty windows read against c.
func NewBudget(c clock.Clock, limits BudgetLimits) (*Budget, error) {
	if c == nil {
		return nil, errors.New("llm: budget needs a clock")
	}
	if limits.BackgroundCallsPerHour < 0 || limits.TurnCallsPerMinute < 0 {
		return nil, fmt.Errorf("llm: budget limits must not be negative: background %d, turn %d",
			limits.BackgroundCallsPerHour, limits.TurnCallsPerMinute)
	}
	return &Budget{clock: c, limits: limits, calls: make(map[string]BudgetCall)}, nil
}

// budgetRule is the limit that covers a key: which keys it sums, its cap and
// its window.
type budgetRule struct {
	kind   string
	cap    int
	window time.Duration
	name   string
	covers func(BudgetKey) bool
}

func isBackground(k BudgetKey) bool {
	return k.Phase == PhaseTick && (k.Level == levelGlobal || k.Level == levelDomain)
}

func isTurn(k BudgetKey) bool {
	return k.Phase == PhaseNarrative && k.Level == levelTask
}

// ruleFor returns the limit that applies to key, false when none does.
func (b *Budget) ruleFor(key BudgetKey) (budgetRule, bool) {
	switch {
	case isBackground(key):
		return budgetRule{
			kind: BudgetKindBackground, cap: b.limits.BackgroundCallsPerHour,
			window: BackgroundWindow, name: backgroundWindowName, covers: isBackground,
		}, true
	case isTurn(key) && b.limits.TurnCallsPerMinute > 0:
		return budgetRule{
			kind: BudgetKindTurn, cap: b.limits.TurnCallsPerMinute,
			window: TurnWindow, name: turnWindowName, covers: isTurn,
		}, true
	}
	return budgetRule{}, false
}

// retention is how long a call can still matter to a window: the longest one.
const retention = BackgroundWindow

// Allow says whether one more call with key fits its window now. A refusal is
// a *BudgetExceededError, which matches ErrBudget; the caller must not call the
// provider then. A call no limit covers is always allowed. The cap applies to
// whoever calls with the key: actor_kind only decides what fills a window
// (Observe), so an admin tick of ci is refused once B is spent.
func (b *Budget) Allow(key BudgetKey) error {
	if err := key.check(); err != nil {
		return err
	}
	rule, ok := b.ruleFor(key)
	if !ok {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock.Now()
	b.prune(now)

	since := now.Add(-rule.window)
	calls := 0
	for _, c := range b.calls {
		if c.Key.World == key.World && rule.covers(c.Key) && c.At.After(since) {
			calls++
		}
	}
	if calls < rule.cap {
		return nil
	}
	return &BudgetExceededError{
		Key:   key,
		Limit: BudgetLimit{Kind: rule.kind, Limit: rule.cap, Window: rule.name},
		Calls: calls,
	}
}

// Observe counts the call an llm.output records and reports whether it was
// counted now. It is not counted when the event is not an llm.output, is
// marked replay (fed back in test mode, no call was made), is a background
// call not made by the system itself (КД §7.3: an admin tick of ci does not
// spend the budget of the world), is already out of every window, or was
// counted before under the same id.
//
// An llm.output that cannot be placed in a window — no world, no agent level,
// no phase or provider — is ErrBudgetKey: skipping it would hide a call from
// the budget.
func (b *Budget) Observe(ev eventbus.Event) (bool, error) {
	if ev.Type != typeLLMOutput || ev.Meta.Replay {
		return false, nil
	}
	key, err := BudgetKeyOf(ev)
	if err != nil {
		return false, err
	}
	if ev.ID == "" {
		return false, fmt.Errorf("%w: llm.output without an id", ErrBudgetKey)
	}
	if isBackground(key) && ev.Meta.ActorKind != eventbus.ActorSystem {
		return false, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock.Now()
	b.prune(now)
	if _, seen := b.calls[ev.ID]; seen {
		return false, nil
	}
	if !ev.Timestamp.After(now.Add(-retention)) {
		return false, nil
	}
	b.calls[ev.ID] = BudgetCall{EventID: ev.ID, Key: key, At: ev.Timestamp}
	return true, nil
}

// Handle is Observe as an eventbus.Handler, for the catch-up of the swarm to
// read llm_records through Journal.ReadRange (T-237). Other types of the topic
// pass through.
func (b *Budget) Handle(_ context.Context, ev eventbus.Event) error {
	if _, err := b.Observe(ev); err != nil {
		return fmt.Errorf("llm: budget: event %s: %w", ev.ID, err)
	}
	return nil
}

// Calls returns the calls the windows hold now, ordered by time and id, so two
// budgets fed the same records compare equal. It is every counted call of the
// last hour, including those no limit covers (a decision, a narrative with
// the turn limit off).
func (b *Budget) Calls() []BudgetCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.prune(b.clock.Now())
	out := make([]BudgetCall, 0, len(b.calls))
	for _, c := range b.calls {
		out = append(out, c)
	}
	slices.SortFunc(out, func(x, y BudgetCall) int {
		if c := x.At.Compare(y.At); c != 0 {
			return c
		}
		return strings.Compare(x.EventID, y.EventID)
	})
	return out
}

// prune forgets the calls no window can count any more. A record that arrives
// again after that is out of every window by its own timestamp, so forgetting
// its id does not let it be counted twice.
func (b *Budget) prune(now time.Time) {
	horizon := now.Add(-retention)
	for id, c := range b.calls {
		if !c.At.After(horizon) {
			delete(b.calls, id)
		}
	}
}

// BudgetKeyOf reads the window of an llm.output: the world from the envelope,
// the level from meta.agent, the phase and the provider from the payload.
func BudgetKeyOf(ev eventbus.Event) (BudgetKey, error) {
	var key BudgetKey
	if ev.World != nil {
		key.World = ev.World.Entity.ID
	}
	if ev.Meta.Agent != nil {
		key.Level = ev.Meta.Agent.Level
	}
	pa := ev.Path()
	phase, _ := pa.GetString("phase")
	key.Phase = Phase(phase)
	key.Provider, _ = pa.GetString("provider")
	if err := key.check(); err != nil {
		return BudgetKey{}, fmt.Errorf("llm.output %s: %w", ev.ID, err)
	}
	return key, nil
}
