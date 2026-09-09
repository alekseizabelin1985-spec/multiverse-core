package contracts_test

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	mvcontracts "multiverse-core.io/cmd/mvctl/internal/contracts"
	sharedcontracts "multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

func TestLinesCarryTheClusterSettings(t *testing.T) {
	lines := mvcontracts.Lines(sharedcontracts.Topics())
	if len(lines) != len(sharedcontracts.Topics()) {
		t.Fatalf("%d lines for %d topics", len(lines), len(sharedcontracts.Topics()))
	}
	for _, line := range lines {
		if line.Partitions != 1 || line.Replicas != 1 {
			t.Errorf("%s: -p %d -r %d, want one of each (§5.1)",
				line.Name, line.Partitions, line.Replicas)
		}
		if line.SegmentMS != 86400000 {
			t.Errorf("%s: segment.ms %d, want a day (D-9)", line.Name, line.SegmentMS)
		}
		if line.CleanupPolicy != "delete" {
			t.Errorf("%s: cleanup.policy %q", line.Name, line.CleanupPolicy)
		}
		if line.RetentionDays == 0 {
			t.Errorf("%s: retention rounds to zero days", line.Name)
		}
	}
}

// TestMaxMessageBytesOnlyOnLLMRecords pins decision ОВ-37: llm_records carries
// whole prompts and completions and is the one topic that raises the broker
// limit; raising it everywhere would hide a payload that has no business being
// that big.
func TestMaxMessageBytesOnlyOnLLMRecords(t *testing.T) {
	for _, line := range mvcontracts.Lines(sharedcontracts.Topics()) {
		switch line.Name {
		case eventbus.TopicLLMRecords:
			if line.MaxMessageBytes != 4*1024*1024 {
				t.Errorf("llm_records max.message.bytes %d, want 4 MiB", line.MaxMessageBytes)
			}
		default:
			if line.MaxMessageBytes != 0 {
				t.Errorf("%s: max.message.bytes %d, want the broker default",
					line.Name, line.MaxMessageBytes)
			}
		}
	}
}

// scriptTopic is what build/redpanda-init.sh says one topic must look like.
type scriptTopic struct {
	retentionMS     int64
	maxMessageBytes int64
}

// TestRPKMatchesTheInitScript is the reason the command exists: build/
// redpanda-init.sh and the registry must describe one cluster. The script is
// still hand-written, so every number it carries is read back out of it and
// held against the registry — the whole target configuration of a topic, not
// its retention alone, because a drift in segment.ms or in max.message.bytes
// is as silent and as expensive as a drift in the topic list.
func TestRPKMatchesTheInitScript(t *testing.T) {
	raw, err := os.ReadFile("../../../../build/redpanda-init.sh")
	if err != nil {
		t.Skipf("the init script is not in this tree: %v", err)
	}
	script := string(raw)

	// The numeric assignments of the script: SEGMENT_MS, the three retentions
	// and LLM_MAX_MESSAGE_BYTES.
	numbers := map[string]int64{}
	for _, m := range regexp.MustCompile(`(?m)^([A-Z_0-9]+)=([0-9]+)$`).FindAllStringSubmatch(script, -1) {
		numbers[m[1]] = mustInt(t, m[2])
	}

	// What the script applies to every topic: -p, -r, the cleanup policy and
	// the segment, all of them literals in the create arguments.
	args := regexp.MustCompile(`create_args=\(-p ([0-9]+) -r ([0-9]+)`).FindStringSubmatch(script)
	if args == nil {
		t.Fatal("build/redpanda-init.sh no longer opens create_args with -p and -r")
	}
	partitions, replicas := mustInt(t, args[1]), mustInt(t, args[2])

	policy := regexp.MustCompile(`-c cleanup\.policy=([a-z]+)`).FindStringSubmatch(script)
	if policy == nil {
		t.Fatal("build/redpanda-init.sh no longer sets cleanup.policy on create")
	}
	if !strings.Contains(script, "--set cleanup.policy="+policy[1]) {
		t.Errorf("the script creates topics with cleanup.policy=%s and alters them to something else",
			policy[1])
	}
	if !strings.Contains(script, `-c "segment.ms=${SEGMENT_MS}"`) {
		t.Error("build/redpanda-init.sh no longer sets segment.ms from SEGMENT_MS")
	}
	segment, ok := numbers["SEGMENT_MS"]
	if !ok {
		t.Fatal("build/redpanda-init.sh no longer assigns SEGMENT_MS")
	}

	// Lines of the TOPICS table: "name:${RETENTION_30D}[:extra=${VAR}]".
	entry := regexp.MustCompile(`(?m)^\s*"([a-z_]+):\$\{([A-Z_0-9]+)\}(:[^"]*)?"`)
	maxBytes := regexp.MustCompile(`max\.message\.bytes=\$\{([A-Z_0-9]+)\}`)
	inScript := map[string]scriptTopic{}
	for _, m := range entry.FindAllStringSubmatch(script, -1) {
		retention, known := numbers[m[2]]
		if !known {
			t.Errorf("%s: the script takes retention.ms from %s, which it never assigns", m[1], m[2])
			continue
		}
		topic := scriptTopic{retentionMS: retention}
		if extra := maxBytes.FindStringSubmatch(m[3]); extra != nil {
			value, known := numbers[extra[1]]
			if !known {
				t.Errorf("%s: the script takes max.message.bytes from %s, which it never assigns",
					m[1], extra[1])
				continue
			}
			topic.maxMessageBytes = value
		}
		inScript[m[1]] = topic
	}

	for _, line := range mvcontracts.Lines(sharedcontracts.Topics()) {
		want, ok := inScript[line.Name]
		if !ok {
			t.Errorf("%s is in the registry but not in build/redpanda-init.sh", line.Name)
			continue
		}
		if line.RetentionMS != want.retentionMS {
			t.Errorf("%s: retention.ms is %d in the registry and %d in the init script",
				line.Name, line.RetentionMS, want.retentionMS)
		}
		if line.MaxMessageBytes != want.maxMessageBytes {
			t.Errorf("%s: max.message.bytes is %d in the registry and %d in the init script",
				line.Name, line.MaxMessageBytes, want.maxMessageBytes)
		}
		if int64(line.Partitions) != partitions || int64(line.Replicas) != replicas {
			t.Errorf("%s: -p %d -r %d in the registry, -p %d -r %d in the init script",
				line.Name, line.Partitions, line.Replicas, partitions, replicas)
		}
		if line.SegmentMS != segment {
			t.Errorf("%s: segment.ms is %d in the registry and %d in the init script",
				line.Name, line.SegmentMS, segment)
		}
		if line.CleanupPolicy != policy[1] {
			t.Errorf("%s: cleanup.policy is %q in the registry and %q in the init script",
				line.Name, line.CleanupPolicy, policy[1])
		}
		delete(inScript, line.Name)
	}
	for name := range inScript {
		t.Errorf("%s is created by build/redpanda-init.sh but is not in the topic map", name)
	}
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("build/redpanda-init.sh carries %q where a number belongs: %v", s, err)
	}
	return n
}

// TestRPKOutputIsRunnable checks the shape of the generated commands: a create
// a second run survives, an alter-config that converges an existing cluster,
// and a broker address on both — what makes the output a script rather than a
// listing (Mi-3 of the review of T-010).
func TestRPKOutputIsRunnable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := mvcontracts.Run(
		[]string{"topics", "--format=rpk", "--brokers=redpanda:9092"}, &stdout, &stderr)
	if code != cli.ExitOK {
		t.Fatalf("exit code %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()

	want := []string{
		"rpk topic create llm_records -X brokers=redpanda:9092 -p 1 -r 1 -c cleanup.policy=delete " +
			"-c retention.ms=7776000000 -c segment.ms=86400000 -c max.message.bytes=4194304 || true",
		"rpk topic alter-config llm_records -X brokers=redpanda:9092 --set cleanup.policy=delete " +
			"--set retention.ms=7776000000 --set segment.ms=86400000 --set max.message.bytes=4194304",
		"rpk topic create player_events -X brokers=redpanda:9092 -p 1 -r 1 -c cleanup.policy=delete " +
			"-c retention.ms=2592000000 -c segment.ms=86400000 || true",
	}
	for _, line := range want {
		if !strings.Contains(out, line+"\n") {
			t.Errorf("missing line:\n%s\ngot:\n%s", line, out)
		}
	}
	// Piped into a shell, the output must be commands and nothing else, every
	// one of them addressed at the broker the caller named.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if !strings.HasPrefix(line, "rpk topic ") {
			t.Errorf("the rpk form printed something that is not a command: %q", line)
		}
		if !strings.Contains(line, "-X brokers=redpanda:9092") {
			t.Errorf("a command without the broker address: %q", line)
		}
		// `sh -e` stops on the first non-zero exit, and creating a topic that
		// is already there is one.
		if strings.HasPrefix(line, "rpk topic create ") && !strings.HasSuffix(line, " || true") {
			t.Errorf("a create a second run would not survive: %q", line)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr is not empty: %q", stderr.String())
	}
}

// TestRPKWithoutABrokerIsAUsageError keeps the promise of the format: a script
// with an empty -X brokers= is not one, so the command says so instead of
// printing it.
func TestRPKWithoutABrokerIsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := mvcontracts.Run([]string{"topics", "--format=rpk", "--brokers= "}, &stdout, &stderr)
	if code != cli.ExitUsage {
		t.Fatalf("exit code %d, want %d (stdout: %s)", code, cli.ExitUsage, stdout.String())
	}
	if !strings.Contains(stderr.String(), "MV_KAFKA_BROKERS") {
		t.Errorf("the message does not say where the address comes from: %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("a half-written script was printed: %q", stdout.String())
	}
}

func TestTopicsTextForm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := mvcontracts.Run([]string{"topics"}, &stdout, &stderr); code != cli.ExitOK {
		t.Fatalf("exit code %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"topic", "retention", "llm_records", "4194304", "180d"} {
		if !strings.Contains(out, want) {
			t.Errorf("the table does not mention %q:\n%s", want, out)
		}
	}
}

func TestTopicsJSONForm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := mvcontracts.Run([]string{"topics", "--json"}, &stdout, &stderr); code != cli.ExitOK {
		t.Fatalf("exit code %d (stderr: %s)", code, stderr.String())
	}
	var report struct {
		Details []mvcontracts.TopicLine `json:"details"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	names := make([]string, 0, len(report.Details))
	for _, line := range report.Details {
		names = append(names, line.Name)
	}
	for _, want := range []string{eventbus.TopicPlayerEvents, eventbus.TopicDeadLetters} {
		if !slices.Contains(names, want) {
			t.Errorf("%s is missing from the JSON form: %v", want, names)
		}
	}
}
