package store

import (
	"io/fs"
	"testing"
)

// The comparison behind "refuse a data directory or file wider than 0700/0600"
// (ADR-019 p. 1). The tests on real files skip on Windows; this one does not,
// so the rule is exercised on the machine the code is written on.
func TestModeTooWide(t *testing.T) {
	cases := []struct {
		got, limit fs.FileMode
		want       bool
	}{
		{0o700, 0o700, false},
		{0o500, 0o700, false},
		{0o000, 0o700, false},
		{0o750, 0o700, true},
		{0o755, 0o700, true},
		{0o701, 0o700, true},
		{0o600, 0o600, false},
		{0o400, 0o600, false},
		{0o644, 0o600, true},
		{0o660, 0o600, true},
		{0o700, 0o600, true},
		{0o602, 0o600, true},
		// Type bits and special bits are not permissions: a directory with 0700
		// is still 0700.
		{fs.ModeDir | 0o700, 0o700, false},
		{fs.ModeSetuid | 0o600, 0o600, false},
	}
	for _, tc := range cases {
		if got := modeTooWide(tc.got, tc.limit); got != tc.want {
			t.Errorf("modeTooWide(%v, %04o) = %v, want %v", tc.got, tc.limit, got, tc.want)
		}
	}
}
