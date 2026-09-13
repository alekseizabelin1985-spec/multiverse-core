package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
)

var budgetEpoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

const (
	worldA = "dark-forest-world"
	worldB = "second-world"
)

var (
	globalGM = eventbus.AgentRef{ID: "global-gm:" + worldA, Level: "global", Blueprint: "global-gm"}
	regionGM = eventbus.AgentRef{ID: "region-gm:dark-forest", Level: "domain", Blueprint: "region-gm"}
	playerGM = eventbus.AgentRef{ID: "player-gm:solo:player-A", Level: "task", Blueprint: "player-gm"}
)

// manualBudget is a budget on a manual clock that also stamps the events the
// test builds, so a record carries the instant the budget reads.
func manualBudget(t *testing.T, limits BudgetLimits) (*Budget, *clock.Manual) {
	t.Helper()
	c := clock.NewManual(budgetEpoch)
	eventbus.SetClock(c)
	t.Cleanup(func() { eventbus.SetClock(nil) })
	b, err := NewBudget(c, limits)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	return b, c
}

func tickKey(world, level string) BudgetKey {
	return BudgetKey{World: world, Level: level, Phase: PhaseTick, Provider: ProviderOpenAICompat}
}

func turnKey(world string) BudgetKey {
	return BudgetKey{World: world, Level: "task", Phase: PhaseNarrative, Provider: ProviderOpenAICompat}
}

// cause is the event a call answers: a tick of the scheduler (system) or an
// action of a player (human), stamped now by the clock of eventbus.
func cause(world string, phase Phase) eventbus.Event {
	if phase == PhaseTick {
		return eventbus.NewRoot("tick.fired", contracts.SourceSwarm, world, nil, eventbus.ActorSystem, nil)
	}
	return eventbus.NewRoot("player.looked", contracts.SourceGateway, world,
		&eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"}, eventbus.ActorHuman, nil)
}

// record is the llm.output of one call as the recorder of the gateway builds
// it: derived from its cause, a payload the schema accepts.
func record(from eventbus.Event, agent eventbus.AgentRef, phase Phase, provider, status string) eventbus.Event {
	payload := map[string]any{
		"phase":             string(phase),
		"attempt":           1,
		"provider":          provider,
		"model":             "Qwen3.8-27B-UD-Q3_K_XL",
		"params":            map[string]any{"temperature": 0.7},
		"prompt_hash":       "sha256:" + fmt.Sprintf("%064x", 1),
		"response_hash":     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"response_len":      0,
		"validation_status": status,
		"laws_version":      "v1",
		"latency_ms":        1200,
		"tokens":            map[string]any{"prompt": 100, "completion": 0},
		"cost_usd":          0,
	}
	switch status {
	case "error":
		payload["error"] = map[string]any{"code": "timeout"}
	case "invalid":
		payload["response_raw"] = "{}"
	}
	return eventbus.Derive(from, typeLLMOutput, contracts.SourceLLM, payload, eventbus.WithAgent(agent))
}

func call(world string, agent eventbus.AgentRef, phase Phase) eventbus.Event {
	return record(cause(world, phase), agent, phase, ProviderOpenAICompat, "invalid")
}

func mustObserve(t *testing.T, b *Budget, ev eventbus.Event) {
	t.Helper()
	counted, err := b.Observe(ev)
	if err != nil || !counted {
		t.Fatalf("Observe(%s) = %v, %v; want counted", ev.ID, counted, err)
	}
}

func refusal(t *testing.T, err error) *BudgetExceededError {
	t.Helper()
	if !errors.Is(err, ErrBudget) {
		t.Fatalf("err = %v, want ErrBudget", err)
	}
	var exceeded *BudgetExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("err = %T, want *BudgetExceededError", err)
	}
	return exceeded
}

func mustRefuse(t *testing.T, err error) {
	t.Helper()
	_ = refusal(t, err)
}

// countingProvider stands for FakeProvider.Calls() of T-207 until that task
// is merged.
type countingProvider struct{ calls int }

func (p *countingProvider) Generate(context.Context, Request) (Response, error) {
	p.calls++
	return Response{Content: "{}", Provider: ProviderFake}, nil
}
func (p *countingProvider) Embed(context.Context, string, []string) ([][]float32, error) {
	return nil, nil
}
func (p *countingProvider) Health(context.Context) Status            { return StatusOK }
func (p *countingProvider) Models(context.Context) ([]string, error) { return nil, nil }

// generate is the order of the gateway (КД §9.2): the budget first, the
// provider only past it.
func generate(ctx context.Context, b *Budget, p Provider, key BudgetKey) error {
	if err := b.Allow(key); err != nil {
		return err
	}
	_, err := p.Generate(ctx, Request{Phase: key.Phase})
	return err
}

func TestBudgetBackgroundWindowOnTheManualClock(t *testing.T) {
	const capB = 4
	b, c := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: capB})
	key := tickKey(worldA, "domain")

	for i := range capB {
		if err := b.Allow(key); err != nil {
			t.Fatalf("call %d of %d refused: %v", i+1, capB, err)
		}
		mustObserve(t, b, call(worldA, regionGM, PhaseTick))
	}

	exceeded := refusal(t, b.Allow(key))
	want := BudgetLimit{Kind: BudgetKindBackground, Limit: capB, Window: "1h"}
	if exceeded.Limit != want || exceeded.Calls != capB || exceeded.Key != key {
		t.Errorf("refusal = %+v, want limit %+v with %d calls for %+v", exceeded, want, capB, key)
	}

	c.Advance(BackgroundWindow - time.Nanosecond)
	mustRefuse(t, b.Allow(key))

	c.Advance(time.Nanosecond)
	for i := range capB {
		if err := b.Allow(key); err != nil {
			t.Fatalf("an hour later, call %d refused: %v", i+1, err)
		}
		mustObserve(t, b, call(worldA, regionGM, PhaseTick))
	}
	mustRefuse(t, b.Allow(key))
}

// The window slides: each call leaves it an hour after its own instant, not
// when the whole batch is an hour old.
func TestBudgetBackgroundWindowSlides(t *testing.T) {
	b, c := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 2})
	key := tickKey(worldA, "domain")

	mustObserve(t, b, call(worldA, regionGM, PhaseTick)) // 12:00
	c.Advance(40 * time.Minute)
	mustObserve(t, b, call(worldA, regionGM, PhaseTick)) // 12:40
	mustRefuse(t, b.Allow(key))

	c.Advance(20 * time.Minute) // 13:00, the call of 12:00 is out
	if err := b.Allow(key); err != nil {
		t.Fatalf("at 13:00: %v", err)
	}
	mustObserve(t, b, call(worldA, regionGM, PhaseTick))
	mustRefuse(t, b.Allow(key))
	if got := len(b.Calls()); got != 2 {
		t.Errorf("Calls() holds %d calls at 13:00, want 2", got)
	}
}

// The background cap is per world and sums the global and the domain level
// and every provider (КД §7.3: BackgroundBudget of a world).
func TestBudgetBackgroundCapCoversTheLevelsAndProvidersOfAWorld(t *testing.T) {
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 2})

	mustObserve(t, b, call(worldA, globalGM, PhaseTick))
	mustObserve(t, b, record(cause(worldA, PhaseTick), regionGM, PhaseTick, ProviderOllama, "error"))

	for _, key := range []BudgetKey{
		tickKey(worldA, "global"),
		tickKey(worldA, "domain"),
		{World: worldA, Level: "domain", Phase: PhaseTick, Provider: ProviderAnthropic},
	} {
		mustRefuse(t, b.Allow(key))
	}
	for _, key := range []BudgetKey{
		tickKey(worldB, "domain"),
		turnKey(worldA),
		{World: worldA, Level: "domain", Phase: PhaseNarrative, Provider: ProviderOpenAICompat},
		{World: worldA, Level: "task", Phase: PhaseTick, Provider: ProviderOpenAICompat},
	} {
		if err := b.Allow(key); err != nil {
			t.Errorf("Allow(%+v) = %v, want no limit to apply", key, err)
		}
	}
}

// B is a Must cap: zero means no background call, not "no limit".
func TestBudgetZeroBackgroundCapRefusesEveryTick(t *testing.T) {
	b, _ := manualBudget(t, BudgetLimits{})
	exceeded := refusal(t, b.Allow(tickKey(worldA, "global")))
	if exceeded.Limit.Limit != 0 || exceeded.Calls != 0 {
		t.Errorf("refusal = %+v, want limit 0 with 0 calls", exceeded)
	}
}

func TestBudgetTurnLimitIsOffByDefault(t *testing.T) {
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 1})
	for i := range 50 {
		if err := b.Allow(turnKey(worldA)); err != nil {
			t.Fatalf("turn call %d refused with the limit off: %v", i+1, err)
		}
		mustObserve(t, b, call(worldA, playerGM, PhaseNarrative))
	}
	if err := b.Allow(tickKey(worldA, "domain")); err != nil {
		t.Errorf("narrative calls spent the background cap: %v", err)
	}
}

func TestBudgetTurnWindow(t *testing.T) {
	b, c := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 1, TurnCallsPerMinute: 2})
	key := turnKey(worldA)

	mustObserve(t, b, call(worldA, playerGM, PhaseNarrative))
	mustObserve(t, b, call(worldA, playerGM, PhaseNarrative))
	exceeded := refusal(t, b.Allow(key))
	if want := (BudgetLimit{Kind: BudgetKindTurn, Limit: 2, Window: "1m"}); exceeded.Limit != want {
		t.Errorf("limit = %+v, want %+v", exceeded.Limit, want)
	}
	if err := b.Allow(turnKey(worldB)); err != nil {
		t.Errorf("another world refused: %v", err)
	}
	if err := b.Allow(tickKey(worldA, "domain")); err != nil {
		t.Errorf("turn calls spent the background cap: %v", err)
	}
	// The turn limit covers narrative calls of task agents only: a narrative
	// of the global or the domain level is not a turn, whatever the window holds.
	for _, level := range []string{levelGlobal, levelDomain} {
		other := BudgetKey{World: worldA, Level: level, Phase: PhaseNarrative, Provider: ProviderOpenAICompat}
		if err := b.Allow(other); err != nil {
			t.Errorf("Allow(%+v) with the turn window full = %v, want no limit to apply", other, err)
		}
	}

	c.Advance(TurnWindow - time.Nanosecond)
	mustRefuse(t, b.Allow(key))
	c.Advance(time.Nanosecond)
	if err := b.Allow(key); err != nil {
		t.Errorf("a minute later: %v", err)
	}
}

// A narrative of the global or the domain level does not fill the turn window
// either: the rule sums the task level alone.
func TestBudgetTurnWindowIsFilledByTaskAgentsOnly(t *testing.T) {
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 1, TurnCallsPerMinute: 1})

	mustObserve(t, b, call(worldA, globalGM, PhaseNarrative))
	mustObserve(t, b, call(worldA, regionGM, PhaseNarrative))
	if err := b.Allow(turnKey(worldA)); err != nil {
		t.Fatalf("narratives of the global and domain levels spent the turn window: %v", err)
	}

	mustObserve(t, b, call(worldA, playerGM, PhaseNarrative))
	mustRefuse(t, b.Allow(turnKey(worldA)))
}

func TestBudgetRefusalDoesNotCallTheProvider(t *testing.T) {
	ctx := context.Background()
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 1, TurnCallsPerMinute: 1})
	p := &countingProvider{}

	for _, tc := range []struct {
		key   BudgetKey
		agent eventbus.AgentRef
	}{
		{tickKey(worldA, "domain"), regionGM},
		{turnKey(worldA), playerGM},
	} {
		before := p.calls
		if err := generate(ctx, b, p, tc.key); err != nil {
			t.Fatalf("first call %+v: %v", tc.key, err)
		}
		mustObserve(t, b, call(worldA, tc.agent, tc.key.Phase))
		if err := generate(ctx, b, p, tc.key); !errors.Is(err, ErrBudget) {
			t.Fatalf("second call %+v: err = %v, want ErrBudget", tc.key, err)
		}
		if got := p.calls - before; got != 1 {
			t.Errorf("%+v: the provider was called %d times, want 1", tc.key, got)
		}
	}
}

func newJournal(t *testing.T) *membus.Bus {
	t.Helper()
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func restore(t *testing.T, j eventbus.Journal, b *Budget) {
	t.Helper()
	ctx := context.Background()
	end, err := j.End(ctx, eventbus.TopicLLMRecords)
	if err != nil {
		t.Fatalf("End: %v", err)
	}
	if _, err := j.ReadRange(ctx, eventbus.TopicLLMRecords, 0, end, b.Handle); err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
}

func sameCalls(x, y []BudgetCall) bool {
	return slices.EqualFunc(x, y, func(a, b BudgetCall) bool {
		return a.EventID == b.EventID && a.Key == b.Key && a.At.Equal(b.At)
	})
}

func errText(err error) string {
	if err == nil {
		return "allowed"
	}
	return err.Error()
}

// The live run spends the budget through the gateway and records every call
// on the bus; a restart reads llm_records back into an empty budget. The two
// must hold the same calls and give the same answers from then on.
func TestBudgetRestoredFromTheJournalMatchesTheLiveRun(t *testing.T) {
	ctx := context.Background()
	limits := BudgetLimits{BackgroundCallsPerHour: 3, TurnCallsPerMinute: 2}
	live, c := manualBudget(t, limits)
	bus := newJournal(t)
	p := &countingProvider{}

	// One step of the live run: the gateway asks the budget, calls the
	// provider, publishes the record and counts it from the bus.
	step := func(key BudgetKey, agent eventbus.AgentRef, status string) {
		t.Helper()
		err := generate(ctx, live, p, key)
		if errors.Is(err, ErrBudget) {
			return
		}
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		ev := record(cause(key.World, key.Phase), agent, key.Phase, key.Provider, status)
		if err := bus.Publish(ctx, ev); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		mustObserve(t, live, ev)
	}

	worldBGM := eventbus.AgentRef{ID: "region-gm:plains", Level: "domain", Blueprint: "region-gm"}
	step(tickKey(worldA, "global"), globalGM, "invalid")
	c.Advance(10 * time.Minute)
	step(tickKey(worldA, "domain"), regionGM, "error")
	step(turnKey(worldA), playerGM, "invalid")
	step(turnKey(worldA), playerGM, "invalid")
	step(turnKey(worldA), playerGM, "invalid") // refused: no record
	c.Advance(25 * time.Minute)
	step(tickKey(worldB, "domain"), worldBGM, "invalid")
	step(tickKey(worldA, "domain"), regionGM, "invalid")
	step(tickKey(worldA, "domain"), regionGM, "invalid") // refused: no record
	c.Advance(30 * time.Minute)                          // the global tick of 12:00 is out
	step(tickKey(worldA, "global"), globalGM, "invalid")
	step(turnKey(worldA), playerGM, "invalid")

	// What the journal also holds and neither side may count: a refusal and a
	// record fed back in test mode.
	rejected := eventbus.Derive(cause(worldA, PhaseTick), "llm.output.rejected", contracts.SourceLLM,
		map[string]any{"reason": "budget_exceeded", "phase": "tick", "attempt": 1,
			"budget": map[string]any{"kind": BudgetKindBackground, "limit": 3, "window": "1h"}},
		eventbus.WithAgent(regionGM))
	replayed := call(worldA, regionGM, PhaseTick)
	replayed.Meta.Replay = true
	for _, ev := range []eventbus.Event{rejected, replayed} {
		if err := bus.Publish(ctx, ev); err != nil {
			t.Fatalf("Publish %s: %v", ev.Type, err)
		}
		if counted, err := live.Observe(ev); counted || err != nil {
			t.Fatalf("live Observe(%s) = %v, %v; want not counted", ev.Type, counted, err)
		}
	}

	restored, err := NewBudget(c, limits)
	if err != nil {
		t.Fatalf("NewBudget: %v", err)
	}
	restore(t, bus, restored)

	if !sameCalls(live.Calls(), restored.Calls()) {
		t.Fatalf("restored calls differ:\nlive     %+v\nrestored %+v", live.Calls(), restored.Calls())
	}
	if len(restored.Calls()) != 7 {
		t.Errorf("restored %d calls, want 7 (the global tick of 12:00 has left the window)", len(restored.Calls()))
	}
	probes := []BudgetKey{tickKey(worldA, "global"), tickKey(worldA, "domain"), tickKey(worldB, "domain"), turnKey(worldA)}
	for _, advance := range []time.Duration{0, time.Minute, 20 * time.Minute, time.Hour} {
		c.Advance(advance)
		for _, key := range probes {
			if l, r := errText(live.Allow(key)), errText(restored.Allow(key)); l != r {
				t.Errorf("+%s %+v: live %q, restored %q", advance, key, l, r)
			}
		}
	}
}

// The same llm.output reaching the budget twice — the journal holds a
// redelivery, or the catch-up reads what the live run counted — is one call.
func TestBudgetCountsARecordOnceByItsID(t *testing.T) {
	limits := BudgetLimits{BackgroundCallsPerHour: 2}
	b, c := manualBudget(t, limits)
	bus := newJournal(t)

	ev := call(worldA, regionGM, PhaseTick)
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for range 2 {
		if err := bus.Append(eventbus.TopicLLMRecords, body); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	restore(t, bus, b)
	restore(t, bus, b)
	if calls := b.Calls(); len(calls) != 1 || calls[0].EventID != ev.ID {
		t.Fatalf("Calls() = %+v, want the one record %s", calls, ev.ID)
	}
	if err := b.Allow(tickKey(worldA, "domain")); err != nil {
		t.Errorf("a record counted twice filled the cap of 2: %v", err)
	}
	if counted, err := b.Observe(ev); counted || err != nil {
		t.Errorf("live Observe of a restored record = %v, %v; want not counted", counted, err)
	}

	// Once the record has left the window its id is forgotten; a late
	// redelivery is out of the window by its own timestamp and stays uncounted.
	c.Advance(BackgroundWindow)
	if counted, err := b.Observe(ev); counted || err != nil {
		t.Errorf("Observe after the window = %v, %v; want not counted", counted, err)
	}
	if calls := b.Calls(); len(calls) != 0 {
		t.Errorf("Calls() = %+v after the window, want none", calls)
	}
}

// КД §7.3 fills the background window from llm.output with
// meta.actor_kind=system and meta.replay=false only.
func TestBudgetObserveSkipsWhatIsNotACallOfTheWorld(t *testing.T) {
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 1, TurnCallsPerMinute: 1})

	replayed := call(worldA, regionGM, PhaseTick)
	replayed.Meta.Replay = true
	adminTick := call(worldA, regionGM, PhaseTick)
	adminTick.Meta.ActorKind = eventbus.ActorCI
	// Not only ci: every background record not made by the system itself is
	// left out (КД §7.3 fills the window with actor_kind=system alone).
	humanTick := call(worldA, globalGM, PhaseTick)
	humanTick.Meta.ActorKind = eventbus.ActorHuman
	simTick := call(worldA, regionGM, PhaseTick)
	simTick.Meta.ActorKind = eventbus.ActorSim
	other := cause(worldA, PhaseTick)

	for name, ev := range map[string]eventbus.Event{
		"replay": replayed, "ci tick": adminTick, "human tick": humanTick, "sim tick": simTick, "tick.fired": other,
	} {
		if counted, err := b.Observe(ev); counted || err != nil {
			t.Errorf("%s: Observe = %v, %v; want not counted", name, counted, err)
		}
		if err := b.Handle(context.Background(), ev); err != nil {
			t.Errorf("%s: Handle = %v", name, err)
		}
	}
	if err := b.Allow(tickKey(worldA, "domain")); err != nil {
		t.Errorf("background cap spent: %v", err)
	}

	// A turn is counted whoever acted: the player is human.
	mustObserve(t, b, call(worldA, playerGM, PhaseNarrative))
	mustRefuse(t, b.Allow(turnKey(worldA)))
}

// Calls made at one instant are ordered by id, whatever order they were
// observed in: two budgets fed the same records compare equal (NFR-061).
func TestBudgetCallsAtOneInstantAreOrderedByID(t *testing.T) {
	const n = 16
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: n})
	for i := n - 1; i >= 0; i-- {
		ev := call(worldA, regionGM, PhaseTick)
		ev.ID = fmt.Sprintf("evt-%02d", i)
		mustObserve(t, b, ev)
	}
	calls := b.Calls()
	if len(calls) != n {
		t.Fatalf("Calls() holds %d calls, want %d", len(calls), n)
	}
	for i, c := range calls {
		if want := fmt.Sprintf("evt-%02d", i); c.EventID != want || !c.At.Equal(budgetEpoch) {
			t.Fatalf("Calls()[%d] = %s at %s, want %s at %s", i, c.EventID, c.At, want, budgetEpoch)
		}
	}
}

func TestBudgetRefusesARecordItCannotPlace(t *testing.T) {
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 1})
	valid := call(worldA, regionGM, PhaseTick)

	noWorld := valid
	noWorld.World = nil
	noAgent := valid
	noAgent.Meta.Agent = nil
	noID := valid
	noID.ID = ""
	badPhase := call(worldA, regionGM, PhaseTick)
	badPhase.Payload["phase"] = "combat"
	noProvider := call(worldA, regionGM, PhaseTick)
	delete(noProvider.Payload, "provider")

	for name, ev := range map[string]eventbus.Event{
		"no world": noWorld, "no agent": noAgent, "no id": noID, "bad phase": badPhase, "no provider": noProvider,
	} {
		if counted, err := b.Observe(ev); counted || !errors.Is(err, ErrBudgetKey) || errors.Is(err, ErrBudget) {
			t.Errorf("%s: Observe = %v, %v; want ErrBudgetKey", name, counted, err)
		}
		if err := b.Handle(context.Background(), ev); !errors.Is(err, ErrBudgetKey) {
			t.Errorf("%s: Handle = %v, want ErrBudgetKey", name, err)
		}
	}
	if len(b.Calls()) != 0 {
		t.Errorf("Calls() = %+v, want none", b.Calls())
	}

	for _, key := range []BudgetKey{
		{Level: "domain", Phase: PhaseTick, Provider: ProviderOpenAICompat},
		{World: worldA, Phase: PhaseTick, Provider: ProviderOpenAICompat},
		{World: worldA, Level: "domain", Phase: "combat", Provider: ProviderOpenAICompat},
		{World: worldA, Level: "domain", Phase: PhaseTick},
	} {
		if err := b.Allow(key); !errors.Is(err, ErrBudgetKey) || errors.Is(err, ErrBudget) {
			t.Errorf("Allow(%+v) = %v, want ErrBudgetKey", key, err)
		}
	}
}

func TestNewBudgetRefusesBadArguments(t *testing.T) {
	if _, err := NewBudget(nil, BudgetLimits{}); err == nil {
		t.Error("nil clock accepted")
	}
	c := clock.NewManual(budgetEpoch)
	for _, limits := range []BudgetLimits{{BackgroundCallsPerHour: -1}, {TurnCallsPerMinute: -1}} {
		if _, err := NewBudget(c, limits); err == nil {
			t.Errorf("limits %+v accepted", limits)
		}
	}
}

// A refusal becomes llm.output.rejected budget{kind, limit, window}: the values
// the budget names must be the ones the schema accepts.
func TestBudgetRefusalFitsTheRejectedSchema(t *testing.T) {
	b, _ := manualBudget(t, BudgetLimits{BackgroundCallsPerHour: 1, TurnCallsPerMinute: 1})
	mustObserve(t, b, call(worldA, regionGM, PhaseTick))
	mustObserve(t, b, call(worldA, playerGM, PhaseNarrative))

	kinds := schemaEnum(t, "llm.output.rejected.v1.json", "properties", "budget", "properties", "kind", "enum")
	levels := schemaEnum(t, "_common.json", "$defs", "AgentRef", "properties", "level", "enum")
	for _, l := range []string{levelGlobal, levelDomain, levelTask} {
		if !slices.Contains(levels, l) {
			t.Errorf("level %q is not in the schema enum %v", l, levels)
		}
	}

	for _, tc := range []struct {
		key   BudgetKey
		agent eventbus.AgentRef
	}{{tickKey(worldA, "domain"), regionGM}, {turnKey(worldA), playerGM}} {
		exceeded := refusal(t, b.Allow(tc.key))
		if !slices.Contains(kinds, exceeded.Limit.Kind) {
			t.Errorf("kind %q is not in the schema enum %v", exceeded.Limit.Kind, kinds)
		}
		ev := eventbus.Derive(cause(worldA, tc.key.Phase), "llm.output.rejected", contracts.SourceLLM,
			map[string]any{
				"reason": "budget_exceeded", "phase": string(tc.key.Phase), "attempt": 1,
				"budget": map[string]any{
					"kind": exceeded.Limit.Kind, "limit": exceeded.Limit.Limit, "window": exceeded.Limit.Window,
				},
			}, eventbus.WithAgent(tc.agent))
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("%s refusal does not fit the schema: %v", exceeded.Limit.Kind, err)
		}
	}
}
