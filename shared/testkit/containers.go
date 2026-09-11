//go:build integration

// Containers for the integration tests. They are behind the integration tag
// because starting one costs seconds and a Docker daemon, which the unit job
// has neither of (ADR-010: the levels of the test pyramid are separated by
// build tags, not by naming conventions).
//
// Every image comes from build/versions.env through Versions(): the pins of
// the platform are one file, and a test that pulled its own tag would run
// against a broker nothing else in the repository uses (NFR-071).

package testkit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/segmentio/kafka-go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RedpandaImagePin is the name of the pin in build/versions.env.
const RedpandaImagePin = "REDPANDA_IMAGE"

// redpandaKafkaPort is the port redpanda serves the Kafka API on inside the
// container.
const redpandaKafkaPort = "9092/tcp"

// redpandaReady is how long StartRedpanda waits for the broker to answer a
// metadata request after the port opens. The port is listening well before the
// controller has elected itself, and a client that connects in between gets a
// confusing "no leader" instead of a timeout.
const redpandaReady = 90 * time.Second

// Redpanda is a running broker.
type Redpanda struct {
	// Broker is the address a client on this machine connects to.
	Broker    string
	container testcontainers.Container
}

// Brokers returns the address list a bus configuration expects.
func (r *Redpanda) Brokers() []string { return []string{r.Broker} }

// StartRedpanda runs the pinned Redpanda image in dev-container mode and waits
// until it answers a metadata request.
//
// Automatic topic creation is switched off, as it is in compose: the topics of
// the platform carry a retention that redpanda-init gives them, and a topic
// born from a stray produce request would keep its data forever
// (infrastructure.md §5.1). The topics the caller names are created here
// instead.
//
// The container publishes its Kafka port on a host port chosen in advance and
// advertises that address, because a Kafka client reaches the broker at the
// address the broker names in its metadata, not at the one it dialled.
func StartRedpanda(ctx context.Context, topics ...string) (*Redpanda, error) {
	image, err := Version(RedpandaImagePin)
	if err != nil {
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	broker := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	kafkaPort, err := network.ParsePort(redpandaKafkaPort)
	if err != nil {
		return nil, fmt.Errorf("testkit: parse %s: %w", redpandaKafkaPort, err)
	}
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{redpandaKafkaPort},
		// The host port is chosen before the container starts and bound
		// explicitly, because the broker has to advertise the address clients
		// will reach it at, and Docker only reveals a dynamic port afterwards.
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{
				kafkaPort: []network.PortBinding{{
					HostIP:   netip.MustParseAddr("127.0.0.1"),
					HostPort: strconv.Itoa(port),
				}},
			}
		},
		Cmd: []string{
			"redpanda", "start",
			// dev-container is the single-node profile: one core, one gibibyte,
			// no fsync checks. It is what the Redpanda documentation prescribes
			// for a throwaway broker and what keeps the start under ten seconds.
			"--mode", "dev-container",
			"--kafka-addr", "PLAINTEXT://0.0.0.0:9092",
			"--advertise-kafka-addr", "PLAINTEXT://" + broker,
			"--set", "redpanda.auto_create_topics_enabled=false",
		},
		WaitingFor: wait.ForListeningPort(redpandaKafkaPort).WithStartupTimeout(redpandaReady),
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("testkit: start %s: %w", image, err)
	}

	rp := &Redpanda{Broker: broker, container: ctr}
	if err := rp.awaitMetadata(ctx); err != nil {
		return nil, errors.Join(err, rp.Terminate(ctx))
	}
	if err := rp.CreateTopics(ctx, topics...); err != nil {
		return nil, errors.Join(err, rp.Terminate(ctx))
	}
	return rp, nil
}

// Container exposes the container so that a caller can hand it to
// testcontainers.CleanupContainer.
func (r *Redpanda) Container() testcontainers.Container { return r.container }

// Terminate stops the broker.
func (r *Redpanda) Terminate(ctx context.Context) error {
	if r == nil || r.container == nil {
		return nil
	}
	return r.container.Terminate(ctx)
}

// CreateTopics creates the named topics with the single partition the strict
// order of the platform relies on (ADR-007 p. 4). Creating a topic that
// already exists is not an error: a test may ask twice.
func (r *Redpanda) CreateTopics(ctx context.Context, names ...string) error {
	if len(names) == 0 {
		return nil
	}
	conn, err := r.controller(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	configs := make([]kafka.TopicConfig, 0, len(names))
	for _, name := range names {
		configs = append(configs, kafka.TopicConfig{
			Topic:             name,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}
	if err := conn.CreateTopics(configs...); err != nil {
		return fmt.Errorf("testkit: create topics on %s: %w", r.Broker, err)
	}
	return nil
}

// controller dials the broker that may create topics.
func (r *Redpanda) controller(ctx context.Context) (*kafka.Conn, error) {
	conn, err := (&kafka.Dialer{}).DialContext(ctx, "tcp", r.Broker)
	if err != nil {
		return nil, fmt.Errorf("testkit: dial %s: %w", r.Broker, err)
	}
	controller, err := conn.Controller()
	_ = conn.Close()
	if err != nil {
		return nil, fmt.Errorf("testkit: read the controller of %s: %w", r.Broker, err)
	}
	addr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	leader, err := (&kafka.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("testkit: dial the controller %s: %w", addr, err)
	}
	return leader, nil
}

// awaitMetadata polls until the broker answers, or gives up.
func (r *Redpanda) awaitMetadata(ctx context.Context) error {
	wall := Wall()
	deadline := wall.Now().Add(redpandaReady)
	var last error
	for wall.Now().Before(deadline) {
		conn, err := (&kafka.Dialer{}).DialContext(ctx, "tcp", r.Broker)
		if err == nil {
			_, err = conn.Brokers()
			_ = conn.Close()
			if err == nil {
				return nil
			}
		}
		last = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("testkit: %s did not answer within %s: %w", r.Broker, redpandaReady, last)
}

// freePort reserves a port by binding it and letting go. There is a window in
// which something else could take it, but the container is started immediately
// afterwards and the alternative — letting Docker choose the port — cannot
// work: the advertised address has to be known before the broker starts.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("testkit: reserve a port: %w", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, fmt.Errorf("testkit: release the reserved port: %w", err)
	}
	return port, nil
}
