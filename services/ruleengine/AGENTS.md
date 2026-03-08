# AGENTS.md for RuleEngine

This file provides guidance to AI assistants when working with the RuleEngine service.

## Service Overview

RuleEngine is a specialized service that applies mechanical rules to entity behavior in the multiverse. It serves as the core engine for executing game mechanics and behavioral rules that govern how entities interact within the virtual worlds.

### Key Responsibilities
- Apply mechanical rules to entity actions and behaviors
- Validate rule safety before application
- Support 50 base mechanical systems as defined in Living Worlds architecture
- Integrate with Entity-Actor service for rule application
- Provide rule storage and retrieval capabilities

### Event Integration
- Subscribes to: `system_events` topic with type `rule.apply`
- Publishes to: `entity_events` topic with type `entity.rule_applied`
- Event types: rule application requests, rule validation results

## Build/Run Commands

- Build: `make build-service SERVICE=ruleengine`
- Run: `make run SERVICE=ruleengine`
- Logs: `make logs-service SERVICE=ruleengine`

## Code Style Guidelines

- All services written in Go 1.25
- Use JSON Schema Draft 7 for payload validation
- Follow event-driven architecture with Kafka/Redpanda
- Use entity paths with dot notation for nested access (e.g., `payload.health.current`)
- Build with CGO disabled (`CGO_ENABLED=0`)
- Use UUIDs for entity IDs
- Containerize with Docker multi-stage builds

## Key Patterns and Utilities

- Event bus integration via `internal/eventbus`
- Rule caching for performance optimization
- Integration with EntityManager for entity state management
- Validation layer for rule safety checks
- Support for contextual modifiers in rule application

## Directory Structure

- Service implementation: `services/ruleengine/`
- Command entry point: `cmd/ruleengine/`
- Internal packages: `internal/eventbus/`, `internal/minio/`

## Service-Specific Conventions

- Supports 50 base mechanical systems as defined in Living Worlds architecture
- All rules are validated through RuleValidator before application
- Rules are stored in MinIO with bucket naming pattern: `rules-{world_id}`
- Integrates with Entity-Actor service for rule execution
- Uses contextual modifiers to adjust rule outcomes
- Implements caching for frequently used rules