package recording_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/recording"
	"multiverse-core.io/shared/testkit"
)

// session builds a small recorded session: a player action, the dice and the
// decision derived from it, and the llm.output records of two agents, two
// phases and a retry.
func session(t *testing.T) []eventbus.Event {
	t.Helper()
	src := testkit.Deterministic(t, "rec")
	scope := &eventbus.ScopeRef{ID: "enc-7", Type: "encounter"}
	root := eventbus.NewRoot("player.attacked", contracts.SourceTestkitGateway, "dark-forest-world", scope,
		eventbus.ActorCI, map[string]any{"target": map[string]any{"id": "wolf-alpha"}, "weapon": "sword"})
	src.Clock.Advance(time.Second)
	enc := eventbus.AgentRef{ID: "encounter-wolf:solo:A", Level: "task", Blueprint: "encounter-wolf"}
	gm := eventbus.AgentRef{ID: "region-gm:dark-forest", Level: "region", Blueprint: "region-gm"}
	dice := eventbus.Derive(root, "dice.rolled", contracts.SourceSwarm,
		map[string]any{"roll_index": 0, "sides": 20, "value": 17, "nested": map[string]any{"list": []any{1, "two", 3.5, nil, true}}},
		eventbus.WithAgent(enc))
	llm := func(agent eventbus.AgentRef, phase string, attempt int, answer string) eventbus.Event {
		return eventbus.Derive(root, recording.TypeLLMOutput, contracts.SourceSwarm,
			map[string]any{"phase": phase, "attempt": attempt, "response_raw": answer}, eventbus.WithAgent(agent))
	}
	return []eventbus.Event{
		root,
		dice,
		llm(enc, "decision", 1, "enc decision 1"),
		llm(enc, "decision", 2, "enc decision 2"),
		llm(enc, "narrative", 1, "enc narrative 1"),
		llm(gm, "decision", 1, "gm decision 1"),
		eventbus.NewRoot("player.said", contracts.SourceTestkitGateway, "dark-forest-world", nil,
			eventbus.ActorCI, map[string]any{"text": "строка с \"кавычками\" и\nпереводом строки"}),
	}
}

func writeRecording(t *testing.T, path string, events []eventbus.Event) {
	t.Helper()
	w, err := recording.NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	for _, ev := range events {
		if err := w.Append(ev); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// The DoD of T-060: a recording read back and written again is the same bytes.
// A second read gives the same events, so replaying a recording twice sees one
// session.
func TestRecordingRoundTripIsByteForByte(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.jsonl"), filepath.Join(dir, "second.jsonl")
	events := session(t)
	writeRecording(t, first, events)

	rec, err := recording.Open(first)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if rec.Len() != len(events) {
		t.Fatalf("Len = %d, want %d", rec.Len(), len(events))
	}
	writeRecording(t, second, slices.Collect(rec.Events()))

	a, errA := os.ReadFile(first)
	b, errB := os.ReadFile(second)
	if errA != nil || errB != nil {
		t.Fatalf("read back: %v, %v", errA, errB)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("the round trip changed the bytes:\nfirst:  %s\nsecond: %s", a, b)
	}
	if n := bytes.Count(a, []byte("\n")); n != len(events) {
		t.Errorf("%d lines for %d events: one event per line", n, len(events))
	}

	again, err := recording.Open(second)
	if err != nil {
		t.Fatalf("Open(second): %v", err)
	}
	for i, ev := range slices.Collect(again.Events()) {
		orig := events[i]
		if ev.ID != orig.ID || ev.Type != orig.Type || !ev.Timestamp.Equal(orig.Timestamp) ||
			ev.CorrelationID() != orig.CorrelationID() || ev.Meta.CausationID != orig.Meta.CausationID {
			t.Errorf("event %d = %s %s %v, want %s %s %v", i, ev.Type, ev.ID, ev.Timestamp, orig.Type, orig.ID, orig.Timestamp)
		}
	}
	if start, ok := rec.Start(); !ok || !start.Equal(events[0].Timestamp) {
		t.Errorf("Start = %v, %v; want the timestamp of the first event %v", start, ok, events[0].Timestamp)
	}
}

// The DoD of T-060: Index finds a record by (correlation_id, agent.id, phase,
// attempt) — the key RecordedProvider looks an answer up by (C-07).
func TestIndexFindsAnLLMOutputByItsKey(t *testing.T) {
	events := session(t)
	path := filepath.Join(t.TempDir(), "s.jsonl")
	// A redelivered record carries the same key and must not replace the
	// record that was used.
	redelivered := events[2]
	redelivered.Payload = map[string]any{"phase": "decision", "attempt": 1, "response_raw": "a second answer"}
	writeRecording(t, path, append(events, redelivered))

	rec, err := recording.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	index := rec.Index(recording.TypeLLMOutput, recording.LLMOutputKeyOf)
	cid := events[0].ID

	want := map[string]string{
		recording.LLMOutputKey(cid, "encounter-wolf:solo:A", "decision", 1):  "enc decision 1",
		recording.LLMOutputKey(cid, "encounter-wolf:solo:A", "decision", 2):  "enc decision 2",
		recording.LLMOutputKey(cid, "encounter-wolf:solo:A", "narrative", 1): "enc narrative 1",
		recording.LLMOutputKey(cid, "region-gm:dark-forest", "decision", 1):  "gm decision 1",
	}
	if len(index) != len(want) {
		t.Errorf("index has %d records, want %d: only llm.output, one per key", len(index), len(want))
	}
	for key, answer := range want {
		ev, ok := index[key]
		if !ok {
			t.Errorf("no record under %q", key)
			continue
		}
		if got, _ := ev.Path().GetString("response_raw"); got != answer {
			t.Errorf("record under %q answers %q, want %q", key, got, answer)
		}
	}
	for _, miss := range []string{
		recording.LLMOutputKey(cid, "encounter-wolf:solo:A", "decision", 3),
		recording.LLMOutputKey("other-chain", "encounter-wolf:solo:A", "decision", 1),
		recording.LLMOutputKey(cid, "region-gm:dark-forest", "narrative", 1),
	} {
		if _, ok := index[miss]; ok {
			t.Errorf("a record was found under %q, which was never recorded", miss)
		}
	}
}

// Each part of the key is length-prefixed, so moving a character across a
// boundary gives another key — a plain separator would not.
func TestLLMOutputKeyKeepsThePartsApart(t *testing.T) {
	pairs := [][2]string{
		{recording.LLMOutputKey("a:b", "c", "decision", 1), recording.LLMOutputKey("a", "b:c", "decision", 1)},
		{recording.LLMOutputKey("ab", "c", "decision", 1), recording.LLMOutputKey("a", "bc", "decision", 1)},
		{recording.LLMOutputKey("a", "b", "decision", 11), recording.LLMOutputKey("a", "b", "decision1", 1)},
	}
	for _, p := range pairs {
		if p[0] == p[1] {
			t.Errorf("two different tuples share the key %q", p[0])
		}
	}
}

// A record without an agent, a phase or an attempt has no key and is left out
// of the index instead of colliding under a partial one.
func TestLLMOutputKeyOfNeedsEveryPart(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	agent := eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-wolf:solo:A", Level: "task"})
	cases := map[string]eventbus.Event{
		"no agent": eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceSwarm, "w", nil, eventbus.ActorSystem,
			map[string]any{"phase": "decision", "attempt": 1}),
		"no phase": eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceSwarm, "w", nil, eventbus.ActorSystem,
			map[string]any{"attempt": 1}, agent),
		"no attempt": eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceSwarm, "w", nil, eventbus.ActorSystem,
			map[string]any{"phase": "decision"}, agent),
	}
	for name, ev := range cases {
		if key := recording.LLMOutputKeyOf(ev); key != "" {
			t.Errorf("%s: key %q, want none", name, key)
		}
	}
}

func TestEventsFiltersByType(t *testing.T) {
	events := session(t)
	path := filepath.Join(t.TempDir(), "s.jsonl")
	writeRecording(t, path, events)
	rec, err := recording.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for ev := range rec.Events("player.attacked", "player.said") {
		types = append(types, ev.Type)
	}
	if !slices.Equal(types, []string{"player.attacked", "player.said"}) {
		t.Errorf("filtered events = %v, want [player.attacked player.said] in recording order", types)
	}
	for range rec.Events() {
		break // an early stop of the iteration must not panic
	}
}

// What a hand-edited or truncated recording may contain: blank lines and a
// last line without a newline are read; a line that is not an event is an error
// naming its line.
func TestReadRecordingToleratesLayoutAndNamesBrokenLines(t *testing.T) {
	events := session(t)
	path := filepath.Join(t.TempDir(), "s.jsonl")
	writeRecording(t, path, events[:2])
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")

	ok := "\n" + lines[0] + "\r\n\n" + lines[1] // CRLF, blank lines, no final newline
	rec, err := recording.Read(strings.NewReader(ok))
	if err != nil || rec.Len() != 2 {
		t.Fatalf("Read = %v, %v; want 2 events", rec, err)
	}

	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"not JSON":   {lines[0] + "\n{broken\n", "line 2"},
		"not event":  {lines[0] + "\n" + lines[1] + "\n{\"id\":\"x\"}\n", "line 3: not an event"},
		"line limit": {"{\"type\":\"x\",\"pad\":\"" + strings.Repeat("a", 16<<20) + "\"}\n", "line 1: longer than"},
	} {
		if _, err := recording.Read(strings.NewReader(tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one containing %q", name, err, tc.want)
		}
	}

	// An llm.output carries the raw answer of a model: a line far above the
	// 64 KiB of a default bufio.Scanner is an ordinary event.
	big := events[2]
	big.Payload = map[string]any{"phase": "decision", "attempt": 1, "response_raw": strings.Repeat("я", 100_000)}
	bigPath := filepath.Join(t.TempDir(), "big.jsonl")
	writeRecording(t, bigPath, []eventbus.Event{big})
	if rec, err := recording.Open(bigPath); err != nil || rec.Len() != 1 {
		t.Errorf("a 200 KiB line: %v, %v; want it read", rec, err)
	}

	if rec, err := recording.Read(strings.NewReader("")); err != nil || rec.Len() != 0 {
		t.Errorf("empty recording = %v, %v; want no events and no error", rec, err)
	} else if _, ok := rec.Start(); ok {
		t.Error("Start of an empty recording reports a time")
	}
}

// Start is the earliest time of the recording, not the time of its first line:
// a derived event inherits the time of its cause, so lines in publication
// order are not in time order, and a clock started too late would ignore the
// earlier events for good (review #1 of T-060, N-1).
func TestStartIsTheEarliestTimestamp(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	at := func(ts time.Time) eventbus.Event {
		ev := looked("x")
		ev.Timestamp = ts
		return ev
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	writeRecording(t, path, []eventbus.Event{
		// The zero timestamp comes last, so that nothing after it can hide it.
		at(t0.Add(time.Hour)), at(t0), at(t0.Add(time.Minute)), at(time.Time{}),
	})
	rec, err := recording.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if start, ok := rec.Start(); !ok || !start.Equal(t0) {
		t.Errorf("Start = %v, %v; want the earliest non-zero timestamp %v", start, ok, t0)
	}

	zeroPath := filepath.Join(t.TempDir(), "zero.jsonl")
	writeRecording(t, zeroPath, []eventbus.Event{at(time.Time{})})
	zero, err := recording.Open(zeroPath)
	if err != nil {
		t.Fatal(err)
	}
	if start, ok := zero.Start(); !ok || !start.IsZero() {
		t.Errorf("Start of a recording without timestamps = %v, %v; want the zero time and true", start, ok)
	}
}

// A read that fails in the middle of a line reports the failure of the read,
// not the broken JSON of the fragment it left (review #1 of T-060, N-3).
func TestReadRecordingReportsTheReadErrorOfACutLine(t *testing.T) {
	boom := errors.New("disk gone")
	r := io.MultiReader(strings.NewReader("{\"type\":\"player.looked\",\"id\":\"ev-"), iotest.ErrReader(boom))
	_, err := recording.Read(r)
	if !errors.Is(err, boom) {
		t.Fatalf("Read = %v, want the read error %v", err, boom)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("error %q does not name the line", err)
	}
}

func TestOpenRecordingNamesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.jsonl")
	if _, err := recording.Open(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open(absent) = %v, want os.ErrNotExist", err)
	}
}

// C-01 v1.11 (review of T-458 by system-architect, У-4): every error of the
// package starts with its prefix, a direct Read included, and Open names the
// path once, without the word of the prefix repeated.
func TestErrorsOfReadAndOpenCarryThePrefixOnce(t *testing.T) {
	const broken = "{\"id\":\"x\"}\n"
	if _, err := recording.Read(strings.NewReader(broken)); err == nil ||
		err.Error() != "recording: line 1: not an event: no type" {
		t.Errorf("Read = %v, want \"recording: line 1: not an event: no type\"", err)
	}

	path := filepath.Join(t.TempDir(), "broken.jsonl")
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := recording.Open(path); err == nil || err.Error() != "recording: "+path+": line 1: not an event: no type" {
		t.Errorf("Open = %v, want \"recording: %s: line 1: not an event: no type\"", err, path)
	}

	absent := filepath.Join(t.TempDir(), "absent.jsonl")
	_, err := recording.Open(absent)
	if err == nil || !strings.HasPrefix(err.Error(), "recording: open ") || strings.Count(err.Error(), absent) != 1 ||
		strings.HasPrefix(err.Error(), "recording: open recording") {
		t.Errorf("Open(absent) = %v, want \"recording: open <path>: …\" naming the path once", err)
	}
}

// A recording is the evidence of a session: a writer pointed at an existing
// one refuses instead of truncating it.
func TestNewWriterRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w, err := recording.NewWriter(path); !errors.Is(err, os.ErrExist) {
		if w != nil {
			_ = w.Close()
		}
		t.Fatalf("NewWriter over an existing file = %v, want os.ErrExist", err)
	}
	if body, _ := os.ReadFile(path); string(body) != "kept\n" {
		t.Errorf("the existing recording became %q", body)
	}
}
