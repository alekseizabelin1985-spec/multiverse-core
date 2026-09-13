package recording_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"multiverse-core.io/shared/recording"
)

// The errors of the writer carry the prefix of the package and name the path
// once, like those of Open (N2-1 of review #2 of T-458): no "create recording"
// or "sync recording" after "recording:".
func TestErrorsOfTheWriterCarryThePrefixOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The path is replaced first: the rest of the text is checked without it,
	// whatever directory the test runs in.
	onePath := func(err error, path string) string {
		if err == nil {
			return ""
		}
		msg := strings.Replace(err.Error(), path, "<path>", 1)
		if strings.Contains(msg, path) {
			return "the path twice: " + msg
		}
		return msg
	}
	_, err := recording.NewWriter(path)
	if msg := onePath(err, path); !strings.HasPrefix(msg, "recording: open <path>: ") ||
		strings.Count(msg, "recording") != 1 || !errors.Is(err, os.ErrExist) {
		t.Errorf("NewWriter over an existing file = %v, want \"recording: open <path>: …\" naming the path once", err)
	}

	fresh := filepath.Join(t.TempDir(), "fresh.jsonl")
	w, err := recording.NewWriter(fresh)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// A second Close fails on the sync of a closed file.
	err = w.Close()
	if msg := onePath(err, fresh); !strings.HasPrefix(msg, "recording: sync <path>: ") ||
		strings.Count(msg, "recording") != 1 || !errors.Is(err, os.ErrClosed) {
		t.Errorf("second Close = %v, want \"recording: sync <path>: …\" naming the path once", err)
	}
}
