package env_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	envcmd "multiverse-core.io/cmd/mvctl/internal/env"
	sharedenv "multiverse-core.io/shared/env"
)

// dotenv writes a dotenv file and returns its path.
func dotenv(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	return path
}

// runCheck runs `mvctl env check` and returns the code and both streams.
func runCheck(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := envcmd.Run(append([]string{"check"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestCheckOfTheShippedExample is the criterion of the task: the file the
// repository ships agrees with the manifest, in both directions.
func TestCheckOfTheShippedExample(t *testing.T) {
	code, stdout, stderr := runCheck(t, "--file=../../../../.env.example")
	if code != cli.ExitOK {
		t.Fatalf("exit code %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "variables declared") {
		t.Errorf("the summary does not say what was compared: %q", stdout)
	}
	if stderr != "" {
		t.Errorf("a clean run wrote to stderr: %q", stderr)
	}
}

// TestVariableMissingFromTheExample is one half of NFR-074: a variable the code
// declares and the example does not mention is a variable an operator will not
// know to set.
func TestVariableMissingFromTheExample(t *testing.T) {
	code, _, stderr := runCheck(t, "--file="+dotenv(t, "MV_ENV=dev\n"))
	if code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(stderr, "MV_KAFKA_BROKERS") ||
		!strings.Contains(stderr, "missing from .env.example") {
		t.Errorf("the missing variable was not named: %q", stderr)
	}
}

// TestVariableNotDeclared is the other half: a variable in the file that no
// package reads is either a typo or a leftover, and both are worth a line.
func TestVariableNotDeclared(t *testing.T) {
	code, _, stderr := runCheck(t, "--file="+dotenv(t, "MV_KAFKA_BROKER=redpanda:9092\n"))
	if code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(stderr, "MV_KAFKA_BROKER") ||
		!strings.Contains(stderr, "not declared through env.Declare") {
		t.Errorf("the undeclared variable was not named: %q", stderr)
	}
}

// TestSecretValueIsNotPrinted is the rule of ADR-009 p. 4 held against the one
// command whose whole job is to print what is wrong with the environment.
func TestSecretValueIsNotPrinted(t *testing.T) {
	const password = "hunter2-hunter2-hunter2"
	code, stdout, stderr := runCheck(t,
		"--file="+dotenv(t, "MV_MINIO_SECRET_KEY="+password+"\n"))

	if code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d", code, cli.ExitFindings)
	}
	if strings.Contains(stdout+stderr, password) {
		t.Fatalf("the value of a secret was printed:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if !strings.Contains(stderr, "MV_MINIO_SECRET_KEY") ||
		!strings.Contains(stderr, "must be empty") {
		t.Errorf("the finding does not say what is wrong: %q", stderr)
	}
}

// TestFindingNamesTheLine is what makes the report usable: an operator gets the
// file and the line, not just a name.
func TestFindingNamesTheLine(t *testing.T) {
	body := "# ===== platform =====\nMV_ENV=dev\nMV_LOG_LEVEL=verbose\n"
	_, _, stderr := runCheck(t, "--file="+dotenv(t, body))
	if !strings.Contains(stderr, ".env.example:3 MV_LOG_LEVEL") {
		t.Errorf("the finding does not point at the line: %q", stderr)
	}
	if !strings.Contains(stderr, "outside the declared set") {
		t.Errorf("the finding does not say why: %q", stderr)
	}
}

func TestMissingFile(t *testing.T) {
	code, _, stderr := runCheck(t, "--file="+filepath.Join(t.TempDir(), "absent"))
	if code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(stderr, "cannot be read") {
		t.Errorf("stderr %q", stderr)
	}
}

// TestCheckOfTheEnvironment covers --env: the manifest is validated whole, not
// only the part the contexts of a running process would need (ОВ-27).
func TestCheckOfTheEnvironment(t *testing.T) {
	// MV_LOG_LEVEL carries a closed set; an unlisted value must be reported
	// here rather than at the first read inside a process.
	t.Setenv(sharedenv.LogLevel.Name(), "verbose")

	code, _, stderr := runCheck(t, "--file=../../../../.env.example", "--env")
	if code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d (stderr: %s)", code, cli.ExitFindings, stderr)
	}
	if !strings.Contains(stderr, "MV_LOG_LEVEL") || !strings.Contains(stderr, "expected one of") {
		t.Errorf("the environment was not validated: %q", stderr)
	}
}

func TestJSONForm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := envcmd.Run([]string{"check", "--file=" + dotenv(t, "MV_ENV=dev\n"), "--json"},
		&stdout, &stderr)
	if code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d", code, cli.ExitFindings)
	}
	var report struct {
		Command  string        `json:"command"`
		Status   string        `json:"status"`
		Findings []cli.Finding `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if report.Command != "env check" || report.Status != cli.StatusFindings {
		t.Errorf("report header %+v", report)
	}
	if len(report.Findings) == 0 {
		t.Error("the JSON form carries no findings")
	}
}

func TestUsageErrors(t *testing.T) {
	cases := map[string][]string{
		"no subcommand":      nil,
		"unknown subcommand": {"dump"},
		"unknown flag":       {"check", "--strict"},
		"stray argument":     {"check", ".env"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := envcmd.Run(args, &stdout, &stderr); code != cli.ExitUsage {
				t.Errorf("exit code %d, want %d (stderr: %s)", code, cli.ExitUsage, stderr.String())
			}
		})
	}
}
