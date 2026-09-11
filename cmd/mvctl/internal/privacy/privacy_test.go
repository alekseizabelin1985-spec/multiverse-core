package privacy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
)

// TestRunExitCodes: the exit code is what the security job of CI reads, so it
// is what the test pins (cli.ExitOK / ExitFindings / ExitUsage).
func TestRunExitCodes(t *testing.T) {
	clean := t.TempDir()
	if err := os.WriteFile(filepath.Join(clean, "solo.jsonl"),
		[]byte("{\"player_id\": \"player-A\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirty := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirty, "solo.jsonl"),
		[]byte("{\"chat_id\": 482913776}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		args []string
		want int
	}{
		"clean tree":         {[]string{"scan", clean}, cli.ExitOK},
		"leak":               {[]string{"scan", dirty}, cli.ExitFindings},
		"missing path":       {[]string{"scan", filepath.Join(clean, "absent")}, cli.ExitUsage},
		"unknown subcommand": {[]string{"inspect"}, cli.ExitUsage},
		"no subcommand":      {nil, cli.ExitUsage},
		"unknown flag":       {[]string{"scan", "--deep"}, cli.ExitUsage},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tc.args, &stdout, &stderr); code != tc.want {
				t.Errorf("exit code %d, want %d\nstdout: %s\nstderr: %s",
					code, tc.want, stdout.String(), stderr.String())
			}
		})
	}
}

// TestScanSeparatesAnErrorFromAFinding: the exit code is the only thing CI
// reads, and a walk that stopped on an unreadable path returns a partial
// result — reporting it as ExitFindings would let a leak and a broken scan
// look the same, and a clean partial scan pass as a whole one.
//
// The unreadable path is a name the operating system cannot express (a NUL
// byte): every platform answers it with an error that is not fs.ErrNotExist,
// which is what the branch under test needs. Making a directory unreadable
// with a mode bit would not work on Windows, where this repository is written.
func TestScanSeparatesAnErrorFromAFinding(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "na\x00me")

	if _, err := Scan([]string{broken}); err == nil {
		t.Fatal("an unreadable root scanned without an error")
	} else if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the fixture must not be a missing path: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"scan", broken}, &stdout, &stderr); code != cli.ExitUsage {
		t.Errorf("exit code %d, want %d (error, not findings)", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "incomplete") {
		t.Errorf("stderr does not say the scan is partial: %s", stderr.String())
	}
}

// TestScanNeverPrintsTheValue: the command runs in CI, whose transcript is a
// second place the identifier would live.
func TestScanNeverPrintsTheValue(t *testing.T) {
	dir := t.TempDir()
	const id = "482913776"
	if err := os.WriteFile(filepath.Join(dir, "solo.jsonl"),
		[]byte("{\"chat_id\": "+id+"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, asJSON := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		// The flag goes before the path: the flag package stops parsing at
		// the first positional argument.
		args := []string{"scan"}
		if asJSON {
			args = append(args, "--json")
		}
		args = append(args, dir)
		if code := Run(args, &stdout, &stderr); code != cli.ExitFindings {
			t.Fatalf("exit code %d, want %d", code, cli.ExitFindings)
		}
		printed := stdout.String() + stderr.String()
		if strings.Contains(printed, id) {
			t.Errorf("json=%v: the identifier reached the output:\n%s", asJSON, printed)
		}
		if !strings.Contains(printed, CheckExternalID) {
			t.Errorf("json=%v: the output does not name the rule:\n%s", asJSON, printed)
		}
	}
}

// TestScanJSONIsMachineReadable: the job of CI and, later, the operator report
// read the JSON form, so it has to parse and to carry the findings.
func TestScanJSONIsMachineReadable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "solo.jsonl"),
		[]byte("{\"chat_id\": 482913776}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"scan", "--json", dir}, &stdout, &stderr); code != cli.ExitFindings {
		t.Fatalf("exit code %d\nstderr: %s", code, stderr.String())
	}
	var report struct {
		Command  string        `json:"command"`
		Status   string        `json:"status"`
		Findings []cli.Finding `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, stdout.String())
	}
	if report.Status != cli.StatusFindings {
		t.Errorf("status %q, want %q", report.Status, cli.StatusFindings)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("%d findings, want 1", len(report.Findings))
	}
	if !strings.HasSuffix(report.Findings[0].Subject, "solo.jsonl:1") {
		t.Errorf("subject %q does not name the file and the line", report.Findings[0].Subject)
	}
}

// TestScanTheShippedTestdata is the line the security job runs. It is here so
// that a fixture added in another task fails the unit job of its own pull
// request instead of the security job of the next one.
func TestScanTheShippedTestdata(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"scan", filepath.Join("..", "..", "..", "..", "testdata")},
		&stdout, &stderr); code != cli.ExitOK {
		t.Fatalf("exit code %d\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
}
