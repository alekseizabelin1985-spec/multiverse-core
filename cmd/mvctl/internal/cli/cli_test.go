package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
)

// okCommand is a subcommand that records it was called and succeeds.
func okCommand(name string, called *bool) cli.Command {
	return cli.Command{
		Name:    name,
		Summary: name + " summary",
		Run: func(_ []string, stdout, _ io.Writer) int {
			*called = true
			_, _ = io.WriteString(stdout, name+" ran\n")
			return cli.ExitOK
		},
	}
}

func TestDispatchRunsTheCommand(t *testing.T) {
	called := false
	registry := cli.NewRegistry(okCommand("check", &called))

	var stdout, stderr bytes.Buffer
	code := registry.Dispatch("mvctl", []string{"check", "--flag"}, &stdout, &stderr)

	if code != cli.ExitOK {
		t.Errorf("exit code %d, want %d", code, cli.ExitOK)
	}
	if !called {
		t.Error("the command was not called")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr is not empty: %q", stderr.String())
	}
}

// TestDispatchWithoutArguments pins the two ways of asking for the command
// list apart: `mvctl` alone is a call that went wrong and exits 2, `mvctl help`
// is a question that was answered and exits 0.
func TestDispatchWithoutArguments(t *testing.T) {
	called := false
	registry := cli.NewRegistry(okCommand("check", &called))

	var stdout, stderr bytes.Buffer
	if code := registry.Dispatch("mvctl", nil, &stdout, &stderr); code != cli.ExitUsage {
		t.Errorf("bare mvctl exits %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "usage: mvctl") {
		t.Errorf("the usage did not reach stderr: %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := registry.Dispatch("mvctl", []string{"help"}, &stdout, &stderr); code != cli.ExitOK {
		t.Errorf("mvctl help exits %d, want %d", code, cli.ExitOK)
	}
	if !strings.Contains(stdout.String(), "check summary") {
		t.Errorf("help did not list the command: %q", stdout.String())
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	called := false
	registry := cli.NewRegistry(okCommand("check", &called))

	var stdout, stderr bytes.Buffer
	code := registry.Dispatch("mvctl", []string{"nope"}, &stdout, &stderr)

	if code != cli.ExitUsage {
		t.Errorf("exit code %d, want %d", code, cli.ExitUsage)
	}
	if called {
		t.Error("an unknown command reached a registered one")
	}
	if !strings.Contains(stderr.String(), `unknown command "nope"`) {
		t.Errorf("stderr does not name the command: %q", stderr.String())
	}
}

// TestReservedIsNotSuccess is the point of reserving a name: a build where the
// command does not exist must not look to a script like a check that passed.
func TestReservedIsNotSuccess(t *testing.T) {
	registry := cli.NewRegistry(cli.Reserved("report", "reports as CSV", "EPIC-005"))

	var stdout, stderr bytes.Buffer
	code := registry.Dispatch("mvctl", []string{"report", "--audit"}, &stdout, &stderr)

	if code != cli.ExitUsage {
		t.Errorf("exit code %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "not implemented") ||
		!strings.Contains(stderr.String(), "EPIC-005") {
		t.Errorf("stderr does not say who owns the command: %q", stderr.String())
	}

	stdout.Reset()
	registry.Usage("mvctl", &stdout)
	if !strings.Contains(stdout.String(), "[reserved, EPIC-005]") {
		t.Errorf("the command list does not mark the name reserved: %q", stdout.String())
	}
}

func TestDuplicateNamePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("two commands under one name were accepted")
		}
	}()
	called := false
	cli.NewRegistry(okCommand("check", &called), okCommand("check", &called))
}

// TestReportExitCode is the contract CI reads: findings mean 1, nothing means 0.
func TestReportExitCode(t *testing.T) {
	report := cli.NewReport("contracts check")
	report.Summary = "3 types checked"

	var stdout, stderr bytes.Buffer
	if code := report.Write(&stdout, &stderr, false); code != cli.ExitOK {
		t.Errorf("a clean report exits %d, want %d", code, cli.ExitOK)
	}
	if stderr.Len() != 0 {
		t.Errorf("a clean report wrote to stderr: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "contracts check: 3 types checked") {
		t.Errorf("the summary is missing: %q", stdout.String())
	}

	report.Addf("registry", "player.attacked", "phantom %s", "type")
	if report.OK() {
		t.Error("a report with a finding calls itself clean")
	}
	stdout.Reset()
	stderr.Reset()
	if code := report.Write(&stdout, &stderr, false); code != cli.ExitFindings {
		t.Errorf("a report with a finding exits %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(stderr.String(), "player.attacked: phantom type") {
		t.Errorf("the finding did not reach stderr: %q", stderr.String())
	}
}

// TestReportJSON checks the machine form: the findings are in the document, not
// only on stderr, and stdout stays parseable.
func TestReportJSON(t *testing.T) {
	report := cli.NewReport("env check")
	report.Line("this line belongs to the human form only")
	report.Add("example", "MV_X", "missing from .env.example")

	var stdout, stderr bytes.Buffer
	if code := report.Write(&stdout, &stderr, true); code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d", code, cli.ExitFindings)
	}

	var got struct {
		Command  string        `json:"command"`
		Status   string        `json:"status"`
		Findings []cli.Finding `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.Command != "env check" || got.Status != cli.StatusFindings {
		t.Errorf("report header is %+v", got)
	}
	if len(got.Findings) != 1 || got.Findings[0].Subject != "MV_X" {
		t.Errorf("findings %+v", got.Findings)
	}
	if strings.Contains(stdout.String(), "human form only") {
		t.Error("a human line leaked into the JSON document")
	}
}

// TestReportJSONWithoutFindings keeps the field an array rather than null, so a
// consumer can iterate it without a nil check.
func TestReportJSONWithoutFindings(t *testing.T) {
	report := cli.NewReport("storage init")
	var stdout, stderr bytes.Buffer
	if code := report.Write(&stdout, &stderr, true); code != cli.ExitOK {
		t.Fatalf("exit code %d, want %d", code, cli.ExitOK)
	}
	if !strings.Contains(stdout.String(), `"findings": []`) {
		t.Errorf("findings is not an empty array: %s", stdout.String())
	}
}

func TestParseHelpIsNotAFailure(t *testing.T) {
	var stderr bytes.Buffer
	flags := cli.FlagSet("mvctl x", &stderr)
	flags.Bool("json", false, "print JSON")

	if code, ok := cli.Parse(flags, []string{"-h"}); ok || code != cli.ExitOK {
		t.Errorf("-h gave (%d, %t), want (%d, false)", code, ok, cli.ExitOK)
	}
	if code, ok := cli.Parse(flags, []string{"--nope"}); ok || code != cli.ExitUsage {
		t.Errorf("an unknown flag gave (%d, %t), want (%d, false)", code, ok, cli.ExitUsage)
	}
}

func TestPlural(t *testing.T) {
	cases := map[int]string{0: "0 topics", 1: "1 topic", 8: "8 topics"}
	for n, want := range cases {
		if got := cli.Plural(n, "topic"); got != want {
			t.Errorf("Plural(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFindingWithoutSubject(t *testing.T) {
	got := cli.Finding{Check: "store", Message: "no credentials"}.String()
	if got != "[store] no credentials" {
		t.Errorf("Finding.String() = %q", got)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	if code := cli.UnknownSubcommand(&stderr, "mvctl env", "", "check"); code != cli.ExitUsage {
		t.Errorf("exit code %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "expected one of check") {
		t.Errorf("stderr %q", stderr.String())
	}

	stderr.Reset()
	_ = cli.UnknownSubcommand(&stderr, "mvctl env", "dump", "check")
	if !strings.Contains(stderr.String(), `unknown subcommand "dump"`) {
		t.Errorf("stderr %q", stderr.String())
	}
}

func TestNames(t *testing.T) {
	called := false
	registry := cli.NewRegistry(okCommand("storage", &called), okCommand("contracts", &called))
	names := registry.Names()
	if len(names) != 2 || names[0] != "contracts" || names[1] != "storage" {
		t.Errorf("Names() = %v, want them sorted", names)
	}
	if _, ok := registry.Lookup("storage"); !ok {
		t.Error("Lookup does not find a registered command")
	}
	if _, ok := registry.Lookup("nope"); ok {
		t.Error("Lookup found a command that is not registered")
	}
}
