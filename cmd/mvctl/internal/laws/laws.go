// Package laws implements `mvctl laws show` and `mvctl laws bump`: the author's
// view of the laws of a world and the announcement of a new version
// (swarm-llm-laws.md §12.2, ADR-008 p. 1, contracts.md C-12).
//
// The command works without the swarm. show reads the laws directory and
// prints the version the files make current — the state of a running world is
// not asked, and the output says so. bump publishes world.laws.changed and the
// proposal of World.laws_version on the bus of the installation; State applies
// the proposal, and core reloads its laws from the same directory.
package laws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	lawspkg "multiverse-core.io/internal/laws"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
)

// Summary is the line `mvctl help` shows for the command.
const Summary = "read and bump the laws of a world"

// The rules a finding is filed under.
const (
	// CheckDocument names a laws document that does not load.
	CheckDocument = "document"
	// CheckWorld names a world the laws directory has no current version of.
	CheckWorld = "world"
	// CheckBump names a bump that was refused or could not be published.
	CheckBump = "bump"
	// CheckSource names a laws directory that cannot be read.
	CheckSource = "source"
	// CheckBus names a bus that could not be opened.
	CheckBus = "bus"
)

// The values of --bus: the values of MV_BUS.
const (
	BusKafka  = "kafka"
	BusMemory = "memory"
)

// bumpTimeout bounds the publication: a broker that does not answer must fail
// the command rather than hang the author's terminal or a pipeline.
const bumpTimeout = 30 * time.Second

// Run dispatches `mvctl laws <subcommand>`.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return cli.UnknownSubcommand(stderr, "mvctl laws", "", "show", "bump")
	}
	switch args[0] {
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "bump":
		return runBump(args[1:], stdout, stderr)
	default:
		return cli.UnknownSubcommand(stderr, "mvctl laws", args[0], "show", "bump")
	}
}

// WorldLaws is what show reports about one world.
type WorldLaws struct {
	World string `json:"world"`
	// Origin is where the current version came from: always files here, the
	// command does not ask the state of a running world.
	Origin   string               `json:"origin"`
	Current  *lawspkg.LawsVersion `json:"current,omitempty"`
	Versions []string             `json:"versions"`
	Strain   map[string]int64     `json:"strain"`
}

func runShow(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl laws show", stderr)
	world := flags.String("world", env.WorldID.String(),
		"world to show; empty shows every world of the laws directory")
	dir := flags.String("dir", env.LawsDir.String(), "laws directory")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "mvctl laws show: unexpected argument %q\n", flags.Arg(0))
		return cli.ExitUsage
	}

	report := cli.NewReport("laws show")
	keeper, err := lawspkg.New(context.Background(), lawspkg.Config{Source: lawspkg.FileSource{Dir: *dir}})
	if err != nil {
		sourceFinding(report, *dir, err)
		return report.Write(stdout, stderr, *asJSON)
	}
	Show(keeper, *world, report)
	return report.Write(stdout, stderr, *asJSON)
}

// Show records the current laws of world — of every world of the keeper when
// world is empty — and every document that does not load.
func Show(keeper *lawspkg.Keeper, world string, report *cli.Report) {
	worlds := []string{world}
	if world == "" {
		worlds = keeper.Worlds()
	}
	strain := keeper.Strain().Snapshot()
	shown := make([]WorldLaws, 0, len(worlds))
	for _, w := range worlds {
		current, origin, err := keeper.CurrentFrom(w)
		entry := WorldLaws{World: w, Origin: origin, Versions: keeper.Versions(w), Strain: map[string]int64{}}
		if err != nil {
			report.Add(CheckWorld, w, err.Error())
			shown = append(shown, entry)
			continue
		}
		entry.Current = &current
		for _, id := range current.IDs() {
			entry.Strain[id] = strain[id]
		}
		shown = append(shown, entry)
		describe(report, entry)
	}
	for _, p := range keeper.Problems() {
		if world != "" && p.World != "" && p.World != world {
			continue
		}
		report.Add(CheckDocument, p.Document, p.Err.Error())
	}
	report.Details = shown
	report.Summary = fmt.Sprintf("%s shown", cli.Plural(len(shown)-countFailed(shown), "world"))
}

// describe prints one world the way an author reads it: the version line,
// then one line per law.
func describe(report *cli.Report, w WorldLaws) {
	c := w.Current
	basedOn := ""
	if c.BasedOn != "" {
		basedOn = ", based on " + c.BasedOn
	}
	report.Linef("world %s: laws %s (%s, created by %s%s), current according to the %s; versions: %s",
		w.World, c.Version, c.Status, c.CreatedBy, basedOn, w.Origin, strings.Join(w.Versions, ", "))
	width := 0
	for _, law := range c.Laws {
		width = max(width, len(law.ID))
	}
	for _, law := range c.Laws {
		kind := law.Kind
		if law.Check != "" {
			kind += " " + law.Check
		}
		report.Linef("  %-*s  %-34s  strain %d  %s", width, law.ID, kind, w.Strain[law.ID], law.Text)
	}
	// The strain lives in the laws of core: this process counted nothing, and
	// a column of zeros must not read as "no law was ever pressed".
	report.Line("  strain is counted by core; this process has counted none")
}

func countFailed(shown []WorldLaws) int {
	n := 0
	for _, w := range shown {
		if w.Current == nil {
			n++
		}
	}
	return n
}

// Bumped is what bump reports.
type Bumped struct {
	World    string   `json:"world"`
	From     string   `json:"version_from"`
	To       string   `json:"version_to"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Bus      string   `json:"bus"`
	EventIDs []string `json:"event_ids"`
}

func runBump(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl laws bump", stderr)
	world := flags.String("world", env.WorldID.String(), "world whose laws are bumped")
	from := flags.String("from", "", "the document of the next version, laws/<world>.v<N+1>.yaml")
	dir := flags.String("dir", env.LawsDir.String(), "laws directory the processes of the installation read")
	bus := flags.String("bus", env.Bus.String(), "bus to publish on: kafka or memory")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	switch {
	case flags.NArg() > 0:
		_, _ = fmt.Fprintf(stderr, "mvctl laws bump: unexpected argument %q\n", flags.Arg(0))
		return cli.ExitUsage
	case *from == "":
		_, _ = fmt.Fprintln(stderr, "mvctl laws bump: --from is required")
		return cli.ExitUsage
	case *world == "":
		_, _ = fmt.Fprintln(stderr, "mvctl laws bump: --world is required")
		return cli.ExitUsage
	case *bus != BusKafka && *bus != BusMemory:
		_, _ = fmt.Fprintf(stderr, "mvctl laws bump: unknown bus %q, expected %s or %s\n", *bus, BusKafka, BusMemory)
		return cli.ExitUsage
	}

	report := cli.NewReport("laws bump")
	transport, err := openBus(*bus)
	if err != nil {
		report.Add(CheckBus, *bus, err.Error())
		return report.Write(stdout, stderr, *asJSON)
	}
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), bumpTimeout)
	defer cancel()
	Bump(ctx, BumpRequest{World: *world, From: *from, Dir: *dir, Bus: transport, BusName: *bus}, report)
	return report.Write(stdout, stderr, *asJSON)
}

// BumpRequest is one run of bump.
type BumpRequest struct {
	World   string
	From    string
	Dir     string
	Bus     eventbus.Bus
	BusName string
}

// Bump announces the next version of the laws of a world and records what was
// published.
func Bump(ctx context.Context, req BumpRequest, report *cli.Report) {
	recorder := &recordingBus{Bus: req.Bus}
	keeper, err := lawspkg.New(ctx, lawspkg.Config{Source: lawspkg.FileSource{Dir: req.Dir}, Bus: recorder})
	if err != nil {
		sourceFinding(report, req.Dir, err)
		return
	}
	doc, err := keeper.Bump(ctx, req.World, req.From)
	if err != nil {
		report.Add(CheckBump, req.From, err.Error())
		if len(recorder.published) > 0 {
			report.Linef("published before the failure: %s", strings.Join(recorder.ids(), ", "))
		}
		return
	}

	result := Bumped{World: req.World, To: doc.Version, Bus: req.BusName, EventIDs: recorder.ids()}
	for _, ev := range recorder.published {
		if ev.Type != lawspkg.TypeChanged {
			continue
		}
		pa := ev.Path()
		result.From, _ = pa.GetString("laws.version_from")
		result.Added = stringsAt(ev.Payload, "added")
		result.Removed = stringsAt(ev.Payload, "removed")
	}
	report.Linef("world %s: laws %s -> %s, added [%s], removed [%s]",
		result.World, result.From, result.To, strings.Join(result.Added, ", "), strings.Join(result.Removed, ", "))
	for _, ev := range recorder.published {
		report.Linef("  published %s %s", ev.Type, ev.ID)
	}
	if req.BusName == BusMemory {
		report.Line("note: the bus is in this process; nothing outside it saw the events")
	}
	report.Details = result
	report.Summary = fmt.Sprintf("%s announced", doc.Version)
}

// sourceFinding records a laws directory that cannot be read. The subject is
// the absolute path: MV_LAWS_DIR is relative to the working directory of the
// process, and a relative subject does not say relative to what.
func sourceFinding(report *cli.Report, dir string, err error) {
	subject, message := dir, err.Error()
	if !filepath.IsAbs(dir) {
		if abs, absErr := filepath.Abs(dir); absErr == nil {
			subject = abs
		}
		message += fmt.Sprintf(" (%q is relative to the working directory; set --dir or MV_LAWS_DIR)", dir)
	}
	report.Add(CheckSource, subject, message)
}

// stringsAt reads diff.<key> of the payload Bump built.
func stringsAt(payload map[string]any, key string) []string {
	d, _ := payload["diff"].(map[string]any)
	values, _ := d[key].([]string)
	out := slices.Clone(values)
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// recordingBus remembers what the keeper published, so that the command can
// print the event ids an operator follows with mvctl trace.
type recordingBus struct {
	eventbus.Bus
	published []eventbus.Event
}

func (b *recordingBus) Publish(ctx context.Context, ev eventbus.Event) error {
	if err := b.Bus.Publish(ctx, ev); err != nil {
		return err
	}
	b.published = append(b.published, ev)
	return nil
}

func (b *recordingBus) ids() []string {
	ids := make([]string, 0, len(b.published))
	for _, ev := range b.published {
		ids = append(ids, ev.ID)
	}
	return ids
}

// openBus builds the transport named by --bus with the registry of the
// binary, the way the process does (cmd/multiverse/bus.go).
func openBus(bus string) (eventbus.Bus, error) {
	reg := contracts.Default()
	switch bus {
	case BusKafka:
		return eventbus.NewKafka(eventbus.KafkaConfig{Brokers: env.KafkaBrokers.List(), Registry: reg})
	case BusMemory:
		return membus.New(membus.Config{Registry: reg})
	default:
		return nil, errors.New("unknown bus " + bus)
	}
}
