package recording

import (
	"errors"
	"io/fs"
	"os"
	"testing"
)

// fakeFile is a file whose sync and close answer what the test says.
type fakeFile struct {
	syncErr, closeErr error
	closed            bool
}

func (f *fakeFile) Write(p []byte) (int, error) { return len(p), nil }
func (f *fakeFile) Sync() error                 { return f.syncErr }
func (f *fakeFile) Close() error                { f.closed = true; return f.closeErr }

// A failed close of the file is not dropped and carries the prefix like a
// failed sync; when both fail, the sync is the one reported, and the file is
// closed either way (N2-1 of review #2 of T-458). A closed *os.File fails on
// the sync before the close, so a close failing alone is reachable only with a
// double.
func TestCloseReportsTheFailedStepWithThePrefix(t *testing.T) {
	syncErr := &fs.PathError{Op: "sync", Path: "s.jsonl", Err: errors.New("disk gone")}
	closeErr := &fs.PathError{Op: "close", Path: "s.jsonl", Err: os.ErrClosed}

	cases := map[string]struct {
		file    *fakeFile
		want    string
		isClose bool
	}{
		"both succeed":    {file: &fakeFile{}},
		"the close fails": {file: &fakeFile{closeErr: closeErr}, want: "recording: close s.jsonl: file already closed", isClose: true},
		"the sync fails":  {file: &fakeFile{syncErr: syncErr}, want: "recording: sync s.jsonl: disk gone"},
		"both fail":       {file: &fakeFile{syncErr: syncErr, closeErr: closeErr}, want: "recording: sync s.jsonl: disk gone"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := (&Writer{f: tc.file}).Close()
			if tc.want == "" {
				if err != nil {
					t.Errorf("Close = %v, want nil", err)
				}
			} else if err == nil || err.Error() != tc.want || errors.Is(err, os.ErrClosed) != tc.isClose {
				t.Errorf("Close = %v, want %q (wrapping the close error: %v)", err, tc.want, tc.isClose)
			}
			if !tc.file.closed {
				t.Error("the file was not closed")
			}
		})
	}
}
