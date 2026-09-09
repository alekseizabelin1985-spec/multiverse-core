// Package contracts implements `mvctl contracts`: the command the contracts
// job of CI runs to prove that the registry, the schema files and the topic map
// still describe one platform (design.md §4.1, foundation.md §6).
//
// It is deliberately the only place that knows how to answer "is the contract
// consistent"; the tests of shared/contracts check the same rules from inside
// the package, and a rule that holds in one and not the other is a defect of
// whichever was forgotten.
package contracts

import (
	"fmt"
	"io"
	"io/fs"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/schemas"
	sharedcontracts "multiverse-core.io/shared/contracts"
)

// Summary is the line `mvctl help` shows for the command.
const Summary = "check the event contract and print the topic configuration"

// Run dispatches `mvctl contracts <subcommand>`.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return cli.UnknownSubcommand(stderr, "mvctl contracts", "", "check", "topics")
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "topics":
		return runTopics(args[1:], stdout, stderr)
	default:
		return cli.UnknownSubcommand(stderr, "mvctl contracts", args[0], "check", "topics")
	}
}

// runCheck implements `mvctl contracts check`.
func runCheck(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl contracts check", stderr)
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "mvctl contracts check: unexpected argument %q\n", flags.Arg(0))
		return cli.ExitUsage
	}

	report := cli.NewReport("contracts check")

	// Rule (а): every schema compiles. Building the registry is what compiles
	// them, so a compile error is reported here instead of panicking the way
	// contracts.Default() would — a broken schema is exactly what this command
	// exists to tell an operator about.
	registry, err := sharedcontracts.New(schemas.FS)
	if err != nil {
		report.Add(CheckSchemas, "schemas/events", err.Error())
		report.Summary = "the embedded schemas do not compile"
		return report.Write(stdout, stderr, *asJSON)
	}

	files, err := schemaFiles()
	if err != nil {
		report.Add(CheckSchemas, "schemas/events", err.Error())
		return report.Write(stdout, stderr, *asJSON)
	}

	in := Input{
		Specs:        registry.All(),
		Topics:       registry.Topics(),
		SchemaFiles:  files,
		KnownSources: sharedcontracts.KnownSources(),
	}
	for _, finding := range Check(in) {
		report.Add(finding.Check, finding.Subject, finding.Message)
	}
	report.Details = struct {
		Types  int `json:"types"`
		Topics int `json:"topics"`
		Files  int `json:"schema_files"`
	}{len(in.Specs), len(in.Topics), len(in.SchemaFiles)}
	report.Summary = fmt.Sprintf("%s, %s, %s checked",
		cli.Plural(len(in.Specs), "type"),
		cli.Plural(len(in.Topics), "topic"),
		cli.Plural(len(in.SchemaFiles), "schema file"))
	return report.Write(stdout, stderr, *asJSON)
}

// schemaFiles lists the schema files the binary carries.
func schemaFiles() ([]string, error) {
	files, err := fs.Glob(schemas.FS, "events/*.json")
	if err != nil {
		return nil, fmt.Errorf("list the embedded schemas: %w", err)
	}
	return files, nil
}
