// Package storage implements `mvctl storage init`: the buckets the platform
// needs before anything writes to the object store (design.md §4.1,
// infrastructure.md v0.3 §5.2).
//
// It exists so that the platform does not depend on `minio/mc` being available
// to create a bucket (ADR-021 p. 1): the same objstore.EnsureBucket a context
// calls at run time is what an operator calls here, which is also why the
// bucket the init container creates and the one the platform creates cannot
// carry different rules.
//
// The buckets of a world — entities-{world}, snapshots-{world}, prompts-{world}
// — belong to `mvctl world init` (EPIC-002): a world is created with its
// storage, not before it.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/shared/objstore"
)

// The rules a finding is filed under.
const (
	// CheckBucket names a bucket that could not be brought into shape.
	CheckBucket = "bucket"
	// CheckStore names a problem with the store itself: no endpoint, no
	// credentials, no answer.
	CheckStore = "store"
)

// The values of --store.
const (
	// StoreMinIO is the object store of a deployment.
	StoreMinIO = "minio"
	// StoreMemory is the in-process store: it makes the command runnable in a
	// test and in CI, where there is no server.
	StoreMemory = "memory"
)

// initTimeout bounds the whole run: an object store that does not answer must
// fail the command rather than hang a pipeline.
const initTimeout = 30 * time.Second

// Summary is the line `mvctl help` shows for the command.
const Summary = "create the buckets of the platform in the object store"

// BucketResult is what became of one bucket.
type BucketResult struct {
	Name       string `json:"name"`
	Versioned  bool   `json:"versioned"`
	ExpireDays int    `json:"expire_days,omitempty"`
	// NoncurrentExpireDays applies to a versioned bucket only.
	NoncurrentExpireDays int `json:"noncurrent_expire_days,omitempty"`
	// Ready is false when EnsureBucket refused.
	Ready bool `json:"ready"`
}

// Run dispatches `mvctl storage <subcommand>`.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return cli.UnknownSubcommand(stderr, "mvctl storage", "", "init")
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr)
	default:
		return cli.UnknownSubcommand(stderr, "mvctl storage", args[0], "init")
	}
}

// runInit implements `mvctl storage init`.
func runInit(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl storage init", stderr)
	store := flags.String("store", StoreMinIO,
		"object store to initialise: minio or memory")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "mvctl storage init: unexpected argument %q\n", flags.Arg(0))
		return cli.ExitUsage
	}

	report := cli.NewReport("storage init")
	client, err := open(*store)
	if err != nil {
		var wrongArgument usageError
		if errors.As(err, &wrongArgument) {
			_, _ = fmt.Fprintln(stderr, "mvctl storage init:", err)
			return cli.ExitUsage
		}
		report.Add(CheckStore, *store, err.Error())
		return report.Write(stdout, stderr, *asJSON)
	}

	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()
	Init(ctx, client, report)
	return report.Write(stdout, stderr, *asJSON)
}

// Init brings the buckets of the platform into shape and records what it did in
// the report. It is idempotent: EnsureBucket creates what is missing and
// applies the rules to what is already there, so a second run changes nothing
// and still reports the same state.
func Init(ctx context.Context, client objstore.Client, report *cli.Report) {
	caps := client.Capabilities()
	results := make([]BucketResult, 0, len(PlatformBuckets))
	// Whether this run had a rule to apply at all. A bucket without rules —
	// ops-artifacts is the only one of the platform (infrastructure.md §5.2) —
	// loses nothing on a store that cannot version or expire, and saying it was
	// skipped reads to an operator, and to a CI log, as breakage.
	var wantsVersioning, wantsLifecycle bool

	for _, bucket := range PlatformBuckets {
		opts := objstore.BucketOptionsFor(bucket)
		wantsVersioning = wantsVersioning || opts.Versioned
		wantsLifecycle = wantsLifecycle ||
			opts.ExpireDays > 0 || opts.NoncurrentExpireDays > 0
		result := BucketResult{
			Name:                 bucket,
			Versioned:            opts.Versioned,
			ExpireDays:           opts.ExpireDays,
			NoncurrentExpireDays: opts.NoncurrentExpireDays,
		}
		if err := client.EnsureBucket(ctx, bucket, opts); err != nil {
			report.Addf(CheckBucket, bucket, "cannot be created: %v", err)
		} else {
			result.Ready = true
			report.Linef("bucket %s ready (%s)", bucket, describe(opts))
		}
		results = append(results, result)
	}

	// A store that cannot version or expire is a degradation, not a failure
	// (ADR-021 p. 2, D-6): the platform runs on it, and /health.store says so.
	// The command prints the same thing rather than pretending the rules were
	// applied — but only about a rule a bucket of this run actually asked for.
	if wantsVersioning && !caps.Versioning {
		report.Line("note: the store does not support versioning; the rule was skipped")
	}
	if wantsLifecycle && !caps.Lifecycle {
		report.Line("note: the store does not support lifecycle rules; expiry was skipped")
	}

	report.Details = struct {
		Buckets      []BucketResult        `json:"buckets"`
		Capabilities objstore.Capabilities `json:"capabilities"`
	}{results, caps}
	report.Summary = fmt.Sprintf("%s ready", cli.Plural(readyCount(results), "bucket"))
}

// PlatformBuckets are the buckets that belong to the installation rather than
// to a world. ops-artifacts holds what the operator and CI produce and carries
// no rule: an artifact is deleted when somebody decides to, not on a schedule.
var PlatformBuckets = []string{objstore.OpsArtifacts}

// usageError marks an argument the caller got wrong, as opposed to a store
// that is unreachable: the first is exit code 2, the second is 1.
type usageError struct{ error }

// open builds the client named by --store.
func open(store string) (objstore.Client, error) {
	switch store {
	case StoreMemory:
		return objstore.NewMemory(), nil
	case StoreMinIO:
		cfg, err := objstore.ConfigFromEnv()
		if err != nil {
			return nil, err
		}
		return objstore.New(cfg)
	default:
		return nil, usageError{fmt.Errorf(
			"unknown store %q, expected %s or %s", store, StoreMinIO, StoreMemory)}
	}
}

// describe renders the rules of a bucket the way the table of §5.2 reads.
func describe(opts objstore.BucketOptions) string {
	switch {
	case opts.Versioned && opts.NoncurrentExpireDays > 0:
		return fmt.Sprintf("versioning on, non-current versions expire after %d days",
			opts.NoncurrentExpireDays)
	case opts.Versioned:
		return "versioning on"
	case opts.ExpireDays > 0:
		return fmt.Sprintf("versioning off, objects expire after %d days", opts.ExpireDays)
	default:
		return "no rules"
	}
}

func readyCount(results []BucketResult) int {
	n := 0
	for _, result := range results {
		if result.Ready {
			n++
		}
	}
	return n
}
