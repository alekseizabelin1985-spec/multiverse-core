package replay

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"strconv"
	"strings"
	"time"

	"multiverse-core.io/shared/eventbus"
)

// TypeLLMOutput is the type of the record the LLM gateway writes before it
// uses an answer; replay looks answers up among these (C-07).
const TypeLLMOutput = "llm.output"

// maxRecordLine bounds one line of a recording. llm.output may carry the raw
// answer of the model, far above the 64 KiB a bufio.Scanner allows by
// default; a line longer than this is not an event of the platform.
const maxRecordLine = 16 << 20

// Recording is a recorded session: events in JSONL, one envelope per line, in
// the order they were written. It is read whole into memory — a scenario
// recording is thousands of events, not millions.
type Recording struct {
	events []eventbus.Event
}

// OpenRecording reads the recording at path.
func OpenRecording(path string) (*Recording, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("replay: open recording: %w", err)
	}
	defer func() { _ = f.Close() }()
	rec, err := ReadRecording(f)
	if err != nil {
		return nil, fmt.Errorf("replay: recording %s: %w", path, err)
	}
	return rec, nil
}

// ReadRecording reads a recording from r. Events are decoded the way the bus
// decodes them (encoding/json into eventbus.Event), so a context sees the same
// payload types in replay as in live mode. Blank lines are skipped; a line
// that is not an event with a type is an error naming its number.
func ReadRecording(r io.Reader) (*Recording, error) {
	br := bufio.NewReader(r)
	rec := &Recording{}
	for n := 1; ; n++ {
		line, err := readLine(br)
		if err != nil && !errors.Is(err, io.EOF) {
			// A read that failed part-way leaves a fragment of a line; decoding it
			// would report broken JSON and hide the failure that cut it short.
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		if len(bytes.TrimSpace(line)) > 0 {
			var ev eventbus.Event
			if uerr := json.Unmarshal(line, &ev); uerr != nil {
				return nil, fmt.Errorf("line %d: %w", n, uerr)
			}
			if ev.Type == "" {
				return nil, fmt.Errorf("line %d: not an event: no type", n)
			}
			rec.events = append(rec.events, ev)
		}
		if errors.Is(err, io.EOF) {
			return rec, nil
		}
	}
}

// readLine returns the next line without its terminator, and io.EOF together
// with the last line when the input ends.
func readLine(br *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, err := br.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > maxRecordLine {
			return nil, fmt.Errorf("longer than %d bytes", maxRecordLine)
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return bytes.TrimSuffix(line, []byte("\n")), err
	}
}

// Len returns the number of events.
func (r *Recording) Len() int { return len(r.events) }

// Start returns the earliest timestamp of the recording, the time the clock of
// a replay starts at, and false for an empty recording. It is the minimum and
// not the first line: lines are in publication order, and a derived event
// inherits the time of its cause, so an early timestamp can follow a later one.
// A clock started at the first line would ignore every earlier event, since it
// never goes back. Zero timestamps are skipped; a recording that has only
// those starts at the zero time.
func (r *Recording) Start() (time.Time, bool) {
	if len(r.events) == 0 {
		return time.Time{}, false
	}
	var start time.Time
	for _, ev := range r.events {
		if !ev.Timestamp.IsZero() && (start.IsZero() || ev.Timestamp.Before(start)) {
			start = ev.Timestamp
		}
	}
	return start, true
}

// Events yields the events in recording order; given types, only the events of
// those types. The payload maps are the recording's own: a consumer that needs
// to change one copies it first.
func (r *Recording) Events(types ...string) iter.Seq[eventbus.Event] {
	want := make(map[string]bool, len(types))
	for _, typ := range types {
		want[typ] = true
	}
	return func(yield func(eventbus.Event) bool) {
		for _, ev := range r.events {
			if len(want) > 0 && !want[ev.Type] {
				continue
			}
			if !yield(ev) {
				return
			}
		}
	}
}

// Index maps key(ev) to ev for the events of type typ. An event whose key is
// empty is left out. When two events share a key the first one is kept: a
// record is written before it is used (C-07), and a later duplicate is the
// redelivery of an at-least-once bus, not a second answer.
func (r *Recording) Index(typ string, key func(eventbus.Event) string) map[string]eventbus.Event {
	index := make(map[string]eventbus.Event)
	for ev := range r.Events(typ) {
		k := key(ev)
		if k == "" {
			continue
		}
		if _, seen := index[k]; !seen {
			index[k] = ev
		}
	}
	return index
}

// LLMOutputKey is the key of an llm.output record in replay: the chain, the
// agent, the phase and the attempt (C-07, ADR-003 p. 4). Each part is written
// as <length>:<part>, so that no value can shift a boundary between two parts
// — the same encoding the derived event ids use (C-01 v1.5).
func LLMOutputKey(correlationID, agentID, phase string, attempt int) string {
	var b strings.Builder
	for _, part := range []string{correlationID, agentID, phase, strconv.Itoa(attempt)} {
		b.WriteString(strconv.Itoa(len(part)))
		b.WriteByte(':')
		b.WriteString(part)
	}
	return b.String()
}

// LLMOutputKeyOf returns the key of an llm.output event for Index, or "" when
// the event lacks a part of it: an agent, a phase or an attempt.
func LLMOutputKeyOf(ev eventbus.Event) string {
	if ev.Meta.Agent == nil || ev.Meta.Agent.ID == "" {
		return ""
	}
	pa := ev.Path()
	phase, ok := pa.GetString("phase")
	if !ok || phase == "" {
		return ""
	}
	attempt, ok := pa.GetInt("attempt")
	if !ok {
		return ""
	}
	return LLMOutputKey(ev.CorrelationID(), ev.Meta.Agent.ID, phase, attempt)
}

// Writer appends events to a recording, one JSON line per event. Every Append
// is a write of its own and nothing is buffered in the process: when Append
// returns, the line has been handed to the operating system and survives a
// crash of the process, though not of the machine until Close has synced the
// file. The "written before use" guarantee of C-07 is about the llm.output
// event on the bus, not about this file.
type Writer struct {
	f *os.File
}

// NewWriter creates the recording at path. An existing file is refused rather
// than truncated: a recording is evidence of a session, and overwriting one by
// a mistyped path loses it.
func NewWriter(path string) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("replay: create recording: %w", err)
	}
	return &Writer{f: f}, nil
}

// Append writes ev as one line.
func (w *Writer) Append(ev eventbus.Event) error {
	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("replay: encode %s %s: %w", ev.Type, ev.ID, err)
	}
	if _, err := w.f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("replay: write %s %s: %w", ev.Type, ev.ID, err)
	}
	return nil
}

// Close syncs the file to disk and closes it. The file is closed even when the
// sync fails, and the error of the sync is the one returned.
func (w *Writer) Close() error {
	syncErr := w.f.Sync()
	closeErr := w.f.Close()
	if syncErr != nil {
		return fmt.Errorf("replay: sync recording: %w", syncErr)
	}
	return closeErr
}
