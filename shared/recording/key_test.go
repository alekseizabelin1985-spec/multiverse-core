package recording_test

import (
	"encoding/json"
	"math"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/recording"
	"multiverse-core.io/shared/testkit"
)

// The key is what the recorded provider of EPIC-003 builds from llm.Request
// (C-15 v1.4), so its bytes are fixed here: a change of the encoding is a change
// of the contract, not a refactoring.
func TestLLMOutputKeyGolden(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{recording.LLMOutputKey("ev-1", "encounter-wolf:solo:A", "decision", 1), "4:ev-121:encounter-wolf:solo:A8:decision1:1"},
		{recording.LLMOutputKey("", "", "", 12), "0:0:0:2:12"},
		{recording.LLMOutputKey("цепь", "a", "narrative", 3), "8:цепь1:a9:narrative1:3"},
	} {
		if tc.got != tc.want {
			t.Errorf("LLMOutputKey = %q, want %q", tc.got, tc.want)
		}
	}
}

// C-01 v1.9, the DoD of T-458 A4: an attempt that is not a whole number of at
// least 1 gives no key. jsonpath.GetInt truncated 1.5 to 1, and the record was
// indexed under the key of the first attempt.
func TestLLMOutputKeyOfNeedsAWholeAttemptOfAtLeastOne(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	agent := eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-wolf:solo:A", Level: "task"})
	build := func(payload map[string]any, opts ...eventbus.DeriveOption) eventbus.Event {
		return eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceLLM, "w", nil, eventbus.ActorSystem, payload, opts...)
	}
	withAttempt := func(attempt any) eventbus.Event {
		return build(map[string]any{"phase": "decision", "attempt": attempt}, agent)
	}
	noAttempt := build(map[string]any{"phase": "decision"}, agent)
	blankAgent := build(map[string]any{"phase": "decision", "attempt": 1},
		eventbus.WithAgent(eventbus.AgentRef{ID: "", Level: "task"}))

	for name, tc := range map[string]struct {
		ev      eventbus.Event
		attempt int // 0: no key
	}{
		"int 1":            {withAttempt(1), 1},
		"int 2":            {withAttempt(2), 2},
		"int64":            {withAttempt(int64(3)), 3},
		"int32":            {withAttempt(int32(4)), 4},
		"int8":             {withAttempt(int8(5)), 5},
		"int16":            {withAttempt(int16(6)), 6},
		"uint":             {withAttempt(uint(7)), 7},
		"uint8":            {withAttempt(uint8(8)), 8},
		"uint16":           {withAttempt(uint16(9)), 9},
		"uint32":           {withAttempt(uint32(10)), 10},
		"uint64":           {withAttempt(uint64(11)), 11},
		"float32":          {withAttempt(float32(12)), 12},
		"int8 zero":        {withAttempt(int8(0)), 0},
		"int16 negative":   {withAttempt(int16(-3)), 0},
		"uint zero":        {withAttempt(uint(0)), 0},
		"uint64 2^53":      {withAttempt(uint64(1 << 53)), 1 << 53},
		"uint64 above":     {withAttempt(uint64(1<<53) + 1), 0},
		"float32 fraction": {withAttempt(float32(2.5)), 0},
		"float32 NaN":      {withAttempt(float32(math.NaN())), 0},
		"json.Number":      {withAttempt(json.Number("2")), 0},
		"float64 1":        {withAttempt(1.0), 1},
		"float64 2^53":     {withAttempt(float64(1 << 53)), 1 << 53},
		"fraction 1.5":     {withAttempt(1.5), 0},
		"fraction 0.5":     {withAttempt(0.5), 0},
		"zero":             {withAttempt(0), 0},
		"zero float":       {withAttempt(0.0), 0},
		"negative":         {withAttempt(-1), 0},
		"negative float":   {withAttempt(-2.0), 0},
		"above 2^53":       {withAttempt(float64(1<<53) * 2), 0},
		"int above 2^53":   {withAttempt(int64(1<<53) + 1), 0},
		"NaN":              {withAttempt(math.NaN()), 0},
		"infinity":         {withAttempt(math.Inf(1)), 0},
		"string":           {withAttempt("1"), 0},
		"bool":             {withAttempt(true), 0},
		"null":             {withAttempt(nil), 0},
		"absent":           {noAttempt, 0},
		"agent without id": {blankAgent, 0},
		"empty phase":      {build(map[string]any{"phase": "", "attempt": 1}, agent), 0},
	} {
		key := recording.LLMOutputKeyOf(tc.ev)
		want := ""
		if tc.attempt > 0 {
			want = recording.LLMOutputKey(tc.ev.CorrelationID(), "encounter-wolf:solo:A", "decision", tc.attempt)
		}
		if key != want {
			t.Errorf("%s: key %q, want %q", name, key, want)
		}
	}
}

// An attempt read back from JSON is a float64 and one built in Go is an int:
// both are one attempt and give one key, or a replay would miss every record
// the process itself indexed before writing it.
//
// Every numeric type of Go the recorded provider takes through JSON gives the
// key of its JSON form (review #1 of T-458, N-5).
func TestLLMOutputKeyOfIsTheSameFromJSONAndFromGo(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	for _, attempt := range []any{2, int8(2), int16(2), int32(2), int64(2), uint(2), uint8(2), uint16(2),
		uint32(2), uint64(2), float32(2), 2.0} {
		ev := eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceLLM, "w", nil, eventbus.ActorSystem,
			map[string]any{"phase": "narrative", "attempt": attempt},
			eventbus.WithAgent(eventbus.AgentRef{ID: "region-gm:dark-forest", Level: "region"}))
		body, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		var back eventbus.Event
		if err := json.Unmarshal(body, &back); err != nil {
			t.Fatal(err)
		}
		if _, isFloat := back.Payload["attempt"].(float64); !isFloat {
			t.Fatalf("attempt read back from JSON is %T, want float64: the test no longer compares the two forms", back.Payload["attempt"])
		}
		fromGo, fromJSON := recording.LLMOutputKeyOf(ev), recording.LLMOutputKeyOf(back)
		if fromGo == "" || fromGo != fromJSON {
			t.Errorf("%T: key from Go %q, from JSON %q: want one non-empty key", attempt, fromGo, fromJSON)
		}
	}
}
