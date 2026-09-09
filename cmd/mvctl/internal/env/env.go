// Package env implements `mvctl env check`: the full reading of the variable
// manifest of shared/env (NFR-074, infrastructure.md v0.3 §4.2).
//
// A process checks only what its own contexts need, so that a core without
// memory does not refuse to start over a Neo4j password it will never read
// (decision ОВ-27). This command is the other half of that deal: it checks the
// whole manifest, in both directions against .env.example, and it is what the
// contracts job of CI runs.
//
// No value of a variable marked secret is ever printed. The manifest says which
// ones those are, and shared/env withholds the value in the message it builds;
// this package only ever prints what shared/env hands it.
package env

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	sharedenv "multiverse-core.io/shared/env"
)

// The rules a finding is filed under.
const (
	// CheckExample compares the manifest with the dotenv example, both ways.
	CheckExample = "example"
	// CheckEnvironment validates the values the environment actually carries.
	CheckEnvironment = "environment"
	// CheckDeprecated names a retired variable that is still set.
	CheckDeprecated = "deprecated"
)

// Summary is the line `mvctl help` shows for the command.
const Summary = "check the environment manifest against .env.example"

// Run dispatches `mvctl env <subcommand>`.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return cli.UnknownSubcommand(stderr, "mvctl env", "", "check")
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	default:
		return cli.UnknownSubcommand(stderr, "mvctl env", args[0], "check")
	}
}

// runCheck implements `mvctl env check`.
//
// The default subject is .env.example, not the environment of whoever runs the
// command: the file is committed and CI has no .env, so this is the check that
// can be green in a pipeline. Checking the machine is a separate request
// (--env), because a developer machine legitimately lacks the MinIO keys a
// deployment must carry.
func runCheck(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl env check", stderr)
	file := flags.String("file", sharedenv.ExamplePath, "dotenv file compared with the manifest")
	withEnv := flags.Bool("env", false,
		"also validate the variables of the current process environment")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "mvctl env check: unexpected argument %q\n", flags.Arg(0))
		return cli.ExitUsage
	}

	report := cli.NewReport("env check")
	manifest := sharedenv.Manifest()

	problems, err := sharedenv.CheckExampleFile(*file)
	if err != nil {
		report.Addf(CheckExample, *file, "cannot be read: %v", err)
		return report.Write(stdout, stderr, *asJSON)
	}
	for _, problem := range problems {
		report.Add(CheckExample, subjectOf(*file, problem), problem.Text)
	}

	if *withEnv {
		checkEnvironment(report)
	}

	report.Details = struct {
		File      string `json:"file"`
		Variables int    `json:"variables"`
		Checked   string `json:"checked"`
	}{*file, len(manifest), checkedWhat(*withEnv)}
	report.Summary = fmt.Sprintf("%s declared, compared with %s%s",
		cli.Plural(len(manifest), "variable"), *file, environmentSuffix(*withEnv))
	return report.Write(stdout, stderr, *asJSON)
}

// checkEnvironment validates the process environment against the whole
// manifest — every variable, not only the ones the contexts of a running
// process would need (ОВ-27).
//
// The errors are unwrapped rather than split on newlines: env.Validate joins
// one *env.Error per variable, and the struct already separates the name from
// the reason and has withheld the value of a secret.
func checkEnvironment(report *cli.Report) {
	for _, err := range unwrapJoined(sharedenv.Validate(sharedenv.OS())) {
		var problem *sharedenv.Error
		if errors.As(err, &problem) {
			report.Add(CheckEnvironment, problem.Name, reasonOf(problem))
			continue
		}
		report.Add(CheckEnvironment, "", err.Error())
	}
	for _, gone := range sharedenv.Deprecated(sharedenv.OS()) {
		name, reason, _ := strings.Cut(gone, ":")
		report.Addf(CheckDeprecated, name,
			"retired and still set:%s — remove it from the environment", reason)
	}
}

// reasonOf renders a validation error without repeating the variable name,
// which the finding already carries as its subject. The value of a secret is
// never part of it: shared/env leaves the field empty and says so instead.
func reasonOf(problem *sharedenv.Error) string {
	switch {
	case problem.Secret:
		return problem.Reason + " (value withheld: the variable is a secret)"
	case problem.Value != "":
		return fmt.Sprintf("%s (value %q)", problem.Reason, problem.Value)
	default:
		return problem.Reason
	}
}

// unwrapJoined flattens what errors.Join returned; a lone error is a list of
// one, and nil is a list of none.
func unwrapJoined(err error) []error {
	if err == nil {
		return nil
	}
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		return joined.Unwrap()
	}
	return []error{err}
}

// subjectOf names the place a problem is at: the file and the line when the
// variable is in it, the file alone when the variable is missing from it.
func subjectOf(file string, problem sharedenv.Problem) string {
	if problem.Line == 0 {
		return problem.Name
	}
	return fmt.Sprintf("%s:%d %s", trimPath(file), problem.Line, problem.Name)
}

// trimPath keeps the file name readable in a report printed from any directory.
func trimPath(file string) string {
	if base := file[strings.LastIndexAny(file, `/\`)+1:]; base != "" {
		return base
	}
	return file
}

func checkedWhat(withEnv bool) string {
	if withEnv {
		return "example+environment"
	}
	return "example"
}

func environmentSuffix(withEnv bool) string {
	if withEnv {
		return " and with the process environment"
	}
	return ""
}
