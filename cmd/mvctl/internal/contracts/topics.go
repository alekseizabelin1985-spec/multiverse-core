package contracts

import (
	"fmt"
	"io"
	"strings"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	sharedcontracts "multiverse-core.io/shared/contracts"
	sharedenv "multiverse-core.io/shared/env"
)

// The output formats of `mvctl contracts topics`.
const (
	// FormatText is the table an operator reads.
	FormatText = "text"
	// FormatRPK is the script that brings a cluster to the topic map: the same
	// rpk commands build/redpanda-init.sh runs, so that the init script and the
	// registry cannot describe two different clusters (foundation.md §10).
	FormatRPK = "rpk"
)

// millisPerDay turns a retention into the number an operator thinks in.
const millisPerDay = 24 * 60 * 60 * 1000

// TopicLine is one topic in the JSON form of the command.
type TopicLine struct {
	Name            string `json:"name"`
	Partitions      int    `json:"partitions"`
	Replicas        int    `json:"replicas"`
	RetentionMS     int64  `json:"retention_ms"`
	RetentionDays   int64  `json:"retention_days"`
	SegmentMS       int64  `json:"segment_ms"`
	CleanupPolicy   string `json:"cleanup_policy"`
	MaxMessageBytes int64  `json:"max_message_bytes,omitempty"`
	ReplayRead      bool   `json:"replay_read"`
}

// Lines renders the topic map as the target configuration of the cluster.
func Lines(topics []sharedcontracts.TopicSpec) []TopicLine {
	out := make([]TopicLine, 0, len(topics))
	for _, topic := range topics {
		out = append(out, TopicLine{
			Name:            topic.Name,
			Partitions:      sharedcontracts.TopicPartitions,
			Replicas:        sharedcontracts.TopicReplicas,
			RetentionMS:     topic.RetentionMS,
			RetentionDays:   topic.RetentionMS / millisPerDay,
			SegmentMS:       sharedcontracts.TopicSegmentMS,
			CleanupPolicy:   sharedcontracts.TopicCleanupPolicy,
			MaxMessageBytes: topic.MaxMessageBytes,
			ReplayRead:      topic.ReplayRead,
		})
	}
	return out
}

// Text renders the table a person reads.
func Text(lines []TopicLine) []string {
	width := len("topic")
	for _, line := range lines {
		if len(line.Name) > width {
			width = len(line.Name)
		}
	}
	out := []string{fmt.Sprintf("%-*s  %9s  %9s  %10s  %16s  %6s",
		width, "topic", "retention", "segment", "partitions", "max.message.bytes", "replay")}
	for _, line := range lines {
		maxBytes := "-"
		if line.MaxMessageBytes > 0 {
			maxBytes = fmt.Sprintf("%d", line.MaxMessageBytes)
		}
		out = append(out, fmt.Sprintf("%-*s  %8dd  %8dd  %10d  %16s  %6t",
			width, line.Name,
			line.RetentionDays, line.SegmentMS/millisPerDay,
			line.Partitions, maxBytes, line.ReplayRead))
	}
	return out
}

// RPK renders the commands that bring a cluster to this configuration, in the
// form that survives being piped into a shell more than once. Every topic gets
// a create and an alter-config: the create is what a fresh cluster needs, the
// alter is what an existing one needs after a retention changed here.
//
// Two things make the output a script rather than a listing, and both are what
// build/redpanda-init.sh does in shell around the same pair. The create ends in
// `|| true` because rpk exits non-zero on a topic that is already there, which
// under `sh -e` would end the run on the second pass; a create that failed for
// a real reason is still caught, because the alter-config of the same topic
// runs right after it and has nothing to alter. And the broker address is
// written into every command: rpk without `-X brokers=` goes to
// localhost:9092, which is not where the broker is inside compose.
func RPK(lines []TopicLine, brokers string) []string {
	out := make([]string, 0, 2*len(lines))
	for _, line := range lines {
		configs := []string{
			"cleanup.policy=" + line.CleanupPolicy,
			fmt.Sprintf("retention.ms=%d", line.RetentionMS),
			fmt.Sprintf("segment.ms=%d", line.SegmentMS),
		}
		if line.MaxMessageBytes > 0 {
			configs = append(configs, fmt.Sprintf("max.message.bytes=%d", line.MaxMessageBytes))
		}
		create := fmt.Sprintf("rpk topic create %s -X brokers=%s -p %d -r %d",
			line.Name, brokers, line.Partitions, line.Replicas)
		alter := fmt.Sprintf("rpk topic alter-config %s -X brokers=%s", line.Name, brokers)
		for _, c := range configs {
			create += " -c " + c
			alter += " --set " + c
		}
		out = append(out, create+" || true", alter)
	}
	return out
}

// runTopics implements `mvctl contracts topics`.
func runTopics(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl contracts topics", stderr)
	format := flags.String("format", FormatText, "output format: text or rpk")
	brokers := flags.String("brokers", sharedenv.KafkaBrokers.String(),
		"broker address the generated rpk commands talk to, used by --format=rpk only "+
			"(the default is "+sharedenv.KafkaBrokers.Name()+")")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "mvctl contracts topics: unexpected argument %q\n", flags.Arg(0))
		return cli.ExitUsage
	}

	lines := Lines(sharedcontracts.Topics())
	report := cli.NewReport("contracts topics")
	report.Details = lines
	report.Summary = cli.Plural(len(lines), "topic")

	switch strings.ToLower(*format) {
	case FormatText:
		for _, line := range Text(lines) {
			report.Line(line)
		}
	case FormatRPK:
		if strings.TrimSpace(*brokers) == "" {
			_, _ = fmt.Fprintf(stderr,
				"mvctl contracts topics: --format=rpk needs a broker address; "+
					"pass --brokers or set %s\n", sharedenv.KafkaBrokers.Name())
			return cli.ExitUsage
		}
		for _, line := range RPK(lines, strings.TrimSpace(*brokers)) {
			report.Line(line)
		}
		// The rpk form is meant to be piped into a shell; a trailing summary
		// would end up in the script.
		report.Summary = ""
	default:
		_, _ = fmt.Fprintf(stderr,
			"mvctl contracts topics: unknown format %q, expected %s or %s\n",
			*format, FormatText, FormatRPK)
		return cli.ExitUsage
	}
	return report.Write(stdout, stderr, *asJSON)
}
