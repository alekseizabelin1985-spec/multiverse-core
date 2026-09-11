package main

import (
	"fmt"
	"log/slog"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
)

// transport is what the process builds for --bus: ONE object that is both the
// live side and the journal of the stream. Deps carries it twice, as Bus and as
// Journal, so that a context catching up through the journal reads exactly the
// log it publishes into — two objects could be two logs, and on membus they
// would be.
type transport interface {
	eventbus.Bus
	eventbus.Journal
}

// openBusFunc builds the transport named by the resolved value of --bus/MV_BUS.
// serve takes it as a parameter so that a test can watch the lifetime of the
// transport the process owns (ADR-023).
type openBusFunc func(bus string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error)

// openBus builds the transport of the process. The kind comes from the one
// source T-408 settled — the manifest, overridden by an explicitly passed
// --bus — and parseServe has already refused anything outside its enum; the
// default branch is here for the day the enum grows and this switch does not.
//
// Both transports take the same registry the contexts get in Deps, so the bus
// routes and validates by the contracts the contexts check against, and both
// read MV_BUS_VALIDATE_ON_READ the same way (SEC-16: on unless switched off).
func openBus(bus string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error) {
	switch bus {
	case busKafka:
		cfg, err := kafkaConfig(reg, timers, log)
		if err != nil {
			return nil, err
		}
		return eventbus.NewKafka(cfg)
	case busMemory:
		validate, err := env.BusValidateOnRead.Bool()
		if err != nil {
			return nil, err
		}
		return newMemoryBus(reg, !validate, timers, log)
	default:
		return nil, fmt.Errorf("no transport for %s=%q", env.Bus.Name(), bus)
	}
}

// kafkaConfig is everything the process tells the kafka adapter, read from the
// manifest in one place. It is a function of its own because the adapter keeps
// what it was given in unexported fields: a test checks the configuration here
// instead of a broker or reflection, and the whole way from the variable to
// the field is inside what it checks.
func kafkaConfig(reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (eventbus.KafkaConfig, error) {
	validate, err := env.BusValidateOnRead.Bool()
	if err != nil {
		return eventbus.KafkaConfig{}, err
	}
	return eventbus.KafkaConfig{
		Brokers:            env.KafkaBrokers.List(),
		Registry:           reg,
		SkipValidateOnRead: !validate,
		Timers:             timers,
		Log:                log,
	}, nil
}

// busKafka is the transport over the Kafka API; like busMemory it is a value of
// the manifest enum, not a name of its own.
const busKafka = "kafka"
