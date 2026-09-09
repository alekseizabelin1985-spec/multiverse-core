// Package cli is the shape every mvctl subcommand has: the same flags, the
// same two output forms and the same three exit codes (foundation.md §12,
// design.md §4.1).
//
// It exists because the subcommands are written by five epics. A check that
// invents its own exit code or prints its findings in its own shape turns the
// contracts job of CI into a script of special cases, so the rules are here
// rather than in a convention nobody can enforce.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// The exit codes of mvctl. They are what CI reads, so they mean the same thing
// in every subcommand:
//
//	0 — the command ran and found nothing wrong;
//	1 — the command ran and found something: a phantom type, a variable
//	    missing from .env.example, a bucket it could not create;
//	2 — the command was called wrongly, or is not part of this build.
//
// The split matters in a pipeline: a job that fails with 1 has a report to
// read, a job that fails with 2 has a typo in its command line.
const (
	ExitOK       = 0
	ExitFindings = 1
	ExitUsage    = 2
)

// Command is one subcommand of mvctl.
type Command struct {
	// Name is the first word after mvctl.
	Name string
	// Summary is the one line shown by mvctl help.
	Summary string
	// Owner is the epic that fills the command; it is shown for a command
	// that is only reserved.
	Owner string
	// Run receives the arguments after the name and returns an exit code.
	Run func(args []string, stdout, stderr io.Writer) int
}

// Reserved returns a command that holds a name for another epic. The name is
// taken now so that two epics do not discover at merge time that they both
// called their command "report", and so that mvctl help lists what is coming.
//
// It exits with ExitUsage rather than ExitOK on purpose: a build where the
// command does not exist must not look to a script like a check that passed.
func Reserved(name, summary, owner string) Command {
	return Command{
		Name:    name,
		Summary: summary,
		Owner:   owner,
		Run: func(_ []string, _, stderr io.Writer) int {
			_, _ = fmt.Fprintf(stderr, "mvctl %s: not implemented in this build (%s)\n", name, owner)
			return ExitUsage
		},
	}
}

// Registry is the subcommand table of mvctl.
type Registry struct {
	commands map[string]Command
	order    []string
}

// NewRegistry builds the table. It panics on a duplicate name: two commands
// answering to one word is a defect visible when the binary starts, and the
// point of reserving the names of other epics is to make it visible early.
func NewRegistry(commands ...Command) *Registry {
	r := &Registry{commands: make(map[string]Command, len(commands))}
	for _, cmd := range commands {
		if _, dup := r.commands[cmd.Name]; dup {
			panic("cli: subcommand " + cmd.Name + " is registered twice")
		}
		r.commands[cmd.Name] = cmd
		r.order = append(r.order, cmd.Name)
	}
	sort.Strings(r.order)
	return r
}

// Lookup returns the command registered under name.
func (r *Registry) Lookup(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Names returns the registered names in alphabetical order.
func (r *Registry) Names() []string { return append([]string(nil), r.order...) }

// Dispatch runs the subcommand named by the first argument.
func (r *Registry) Dispatch(binary string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		r.Usage(binary, stderr)
		return ExitUsage
	}
	switch args[0] {
	case "help", "-h", "--help":
		r.Usage(binary, stdout)
		return ExitOK
	}
	cmd, ok := r.commands[args[0]]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "%s: unknown command %q\n", binary, args[0])
		r.Usage(binary, stderr)
		return ExitUsage
	}
	return cmd.Run(args[1:], stdout, stderr)
}

// Usage prints the command table.
func (r *Registry) Usage(binary string, w io.Writer) {
	_, _ = fmt.Fprintf(w, "usage: %s <command> [flags]\n\ncommands:\n", binary)
	width := 0
	for _, name := range r.order {
		if len(name) > width {
			width = len(name)
		}
	}
	for _, name := range r.order {
		cmd := r.commands[name]
		line := fmt.Sprintf("  %-*s  %s", width, name, cmd.Summary)
		if cmd.Owner != "" {
			line += fmt.Sprintf(" [reserved, %s]", cmd.Owner)
		}
		_, _ = fmt.Fprintln(w, line)
	}
	_, _ = fmt.Fprintf(w,
		"\nexit codes: %d ok, %d findings, %d usage error\n", ExitOK, ExitFindings, ExitUsage)
}

// FlagSet returns the flag set every subcommand parses its arguments with:
// errors go to stderr, -h is not a failure, and the caller decides the exit
// code instead of flag calling os.Exit behind its back.
func FlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// Parse parses args and reports the exit code to return when it did not work
// out: ExitOK for -h, ExitUsage for anything else.
func Parse(fs *flag.FlagSet, args []string) (code int, ok bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK, false
		}
		return ExitUsage, false
	}
	return ExitOK, true
}

// Finding is one thing a check disagrees with. Check names the rule, Subject
// the thing the rule is about — a type, a topic, a variable — and Message says
// what is wrong with it in a sentence an operator can act on.
type Finding struct {
	Check   string `json:"check"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

// String renders a finding the way the human form prints it.
func (f Finding) String() string {
	if f.Subject == "" {
		return fmt.Sprintf("[%s] %s", f.Check, f.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", f.Check, f.Subject, f.Message)
}

// Report is the result of one subcommand run: what it looked at, and what it
// disagreed with. Lines carry the human output of a command that has something
// to show beyond its findings — the topic table, the buckets it ensured — and
// are left out of the JSON, where Details carries the same information in a
// shape a script can read.
type Report struct {
	Command  string    `json:"command"`
	Status   string    `json:"status"`
	Summary  string    `json:"summary,omitempty"`
	Findings []Finding `json:"findings"`
	Details  any       `json:"details,omitempty"`

	lines []string
}

// StatusOK and StatusFindings are the two values of Report.Status.
const (
	StatusOK       = "ok"
	StatusFindings = "findings"
)

// NewReport starts a report of the named command.
func NewReport(command string) *Report {
	return &Report{Command: command, Status: StatusOK}
}

// Add records a finding.
func (r *Report) Add(check, subject, message string) {
	r.Findings = append(r.Findings, Finding{Check: check, Subject: subject, Message: message})
	r.Status = StatusFindings
}

// Addf records a finding with a formatted message.
func (r *Report) Addf(check, subject, format string, args ...any) {
	r.Add(check, subject, fmt.Sprintf(format, args...))
}

// Line adds a line to the human output, before the findings.
func (r *Report) Line(text string) { r.lines = append(r.lines, text) }

// Linef adds a formatted line to the human output.
func (r *Report) Linef(format string, args ...any) {
	r.Line(fmt.Sprintf(format, args...))
}

// OK reports whether the run found nothing.
func (r *Report) OK() bool { return len(r.Findings) == 0 }

// Write prints the report and returns the exit code of the command. Findings
// go to stderr and the rest to stdout, so that `mvctl contracts topics
// --format=rpk > init.sh` yields a file of commands and nothing else.
func (r *Report) Write(stdout, stderr io.Writer, asJSON bool) int {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if r.Findings == nil {
			r.Findings = []Finding{}
		}
		if err := enc.Encode(r); err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: write the report: %v\n", r.Command, err)
			return ExitFindings
		}
		return r.exitCode()
	}
	for _, line := range r.lines {
		_, _ = fmt.Fprintln(stdout, line)
	}
	if r.Summary != "" {
		_, _ = fmt.Fprintf(stdout, "%s: %s\n", r.Command, r.Summary)
	}
	for _, f := range r.Findings {
		_, _ = fmt.Fprintf(stderr, "%s: %s\n", r.Command, f)
	}
	if len(r.Findings) > 0 {
		_, _ = fmt.Fprintf(stderr, "%s: %d finding(s)\n", r.Command, len(r.Findings))
	}
	return r.exitCode()
}

func (r *Report) exitCode() int {
	if len(r.Findings) > 0 {
		return ExitFindings
	}
	return ExitOK
}

// Plural is the "s" of a count, so that a summary reads "1 topic" and
// "8 topics" without a format string per message.
func Plural(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

// UnknownSubcommand reports a subcommand of a subcommand that does not exist,
// naming the ones that do.
func UnknownSubcommand(stderr io.Writer, command, got string, want ...string) int {
	if got == "" {
		_, _ = fmt.Fprintf(stderr, "%s: expected one of %s\n", command, strings.Join(want, ", "))
		return ExitUsage
	}
	_, _ = fmt.Fprintf(stderr, "%s: unknown subcommand %q, expected one of %s\n",
		command, got, strings.Join(want, ", "))
	return ExitUsage
}
