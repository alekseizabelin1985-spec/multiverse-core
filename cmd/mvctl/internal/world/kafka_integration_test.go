//go:build integration

// mvctl world init --bus kafka against the servers of the platform: Redpanda
// of the pin and the MinIO image built by build/minio.Dockerfile. The unit
// tests pin the path on membus and the memory store (kafka_test.go); this file
// is what keeps the two from forking — the offsets of a real journal, the
// answers read by Tail from End, and the objects State writes into MinIO.
package world

import (
	"context"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

const (
	minioUser     = "multiverse-test"
	minioPassword = "multiverse-test-password"
)

func TestInitOverKafkaOnRedpandaAndMinIO(t *testing.T) {
	ctx := context.Background()
	reg := contracts.Default()
	topics := make([]string, 0, len(reg.Topics()))
	for _, topic := range reg.Topics() {
		topics = append(topics, topic.Name)
	}
	rp, err := testkit.StartRedpanda(ctx, topics...)
	if rp != nil {
		testcontainers.CleanupContainer(t, rp.Container())
	}
	if err != nil {
		t.Fatalf("start redpanda: %v", err)
	}
	image, err := testkit.Version("MINIO_IMAGE")
	if err != nil {
		t.Fatalf("MINIO_IMAGE: %v", err)
	}
	minio, err := tcminio.Run(ctx, image, tcminio.WithUsername(minioUser), tcminio.WithPassword(minioPassword))
	testcontainers.CleanupContainer(t, minio)
	if err != nil {
		t.Fatalf("start %s: %v", image, err)
	}
	endpoint, err := minio.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	openStore := func(kind string) (objstore.Client, error) {
		if kind != StoreMinIO {
			t.Errorf("the path of kafka opened the store %q", kind)
		}
		return objstore.New(objstore.Config{Endpoint: endpoint, AccessKey: minioUser, SecretKey: minioPassword})
	}
	openBus := func(r *contracts.Registry) (Transport, error) {
		return eventbus.NewKafka(eventbus.KafkaConfig{Brokers: rp.Brokers(), Registry: r, Timers: clock.RealTimers{}, Log: discard()})
	}

	// The core: State over its own bus and its own client of MinIO, as the
	// process runs it, with /health and the admin route on its HTTP server.
	coreStore, err := openStore(StoreMinIO)
	if err != nil {
		t.Fatal(err)
	}
	coreBus, err := openBus(reg)
	if err != nil {
		t.Fatal(err)
	}
	book, err := mechanics.Load(rulesBook)
	if err != nil {
		t.Fatal(err)
	}
	c := state.New(state.Config{Worlds: []string{world}, Log: discard(), Invariants: book.Invariants(),
		Objects: coreStore, SnapshotEvery: -1, RulesVersion: book.Version})
	deps := runtime.Deps{Bus: coreBus, Journal: coreBus, Contracts: reg, Clock: clock.Real{},
		Timers: clock.RealTimers{}, Mode: runtime.ModeLive, Log: discard()}
	if err := c.Start(ctx, deps); err != nil {
		t.Fatalf("Start of core: %v", err)
	}
	t.Setenv(env.CoreAdminClients.Name(), AdminClientID)
	server := runtime.NewHTTP("127.0.0.1:0", runtime.Aggregate([]runtime.Context{c}))
	c.Routes(server.Mux)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		_ = server.Stop(stopCtx)
		_ = c.Stop(stopCtx)
		_ = coreBus.Close()
	})
	if status, _, _ := worldOfHealth(toJSON(t, runtime.Aggregate([]runtime.Context{c})()), world); status["world"] != "uninitialized" {
		t.Errorf("the world before init: %v, want uninitialized", status)
	}

	cmd := Command{OpenStore: openStore, OpenBus: openBus, CoreURL: "http://" + server.Addr}
	var stdout, stderr strings.Builder
	code := cmd.Run([]string{"init", "--world", world, "--fixtures", fixturesDir, "--bus", BusKafka}, &stdout, &stderr)
	t.Logf("world init --bus kafka: exit %d\n%s%s", code, stdout.String(), stderr.String())
	if code != cli.ExitOK {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"entities created 6, skipped 0", "seq 0, reason bootstrap", "state_hash: " + fixtureHash(t)} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout does not say %q", want)
		}
	}
	pointer, err := state.NewObjectStore(coreStore).ReadLatest(ctx, world)
	if err != nil {
		t.Fatalf("latest.json in MinIO: %v", err)
	}
	section, _, _ := worldOfHealth(toJSON(t, runtime.Aggregate([]runtime.Context{c})()), world)
	if cursor, _ := section["cursor"].(float64); pointer.Snapshot.Cursor[eventbus.TopicSystemEvents] != int64(cursor) {
		t.Errorf("cursor.system_events %d, the State of core stands at %v", pointer.Snapshot.Cursor[eventbus.TopicSystemEvents], cursor)
	}

	stdout.Reset()
	stderr.Reset()
	code = cmd.Run([]string{"status", "--world", world, "--store", StoreMinIO}, &stdout, &stderr)
	t.Logf("world status --store minio: exit %d\n%s%s", code, stdout.String(), stderr.String())
	if code != cli.ExitOK || !strings.Contains(stdout.String(), "state_hash: "+pointer.Snapshot.StateHash) {
		t.Errorf("world status: exit %d", code)
	}

	stdout.Reset()
	stderr.Reset()
	if code := cmd.Run([]string{"init", "--world", world, "--fixtures", fixturesDir, "--bus", BusKafka}, &stdout, &stderr); code != cli.ExitUsage {
		t.Errorf("a second init: exit %d, want %d refused as initialized: %s", code, cli.ExitUsage, stderr.String())
	}
}
