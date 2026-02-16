# Multiverse-Core Development Guide

## Project Overview

Multiverse-Core is a sophisticated distributed system designed for managing complex virtual worlds and narratives. This platform combines event-driven architecture, vector databases, graph databases, and AI-powered orchestration to create dynamic, evolving virtual environments. The system implements a philosophy where worlds are not programmed but born, evolving organically through player actions while maintaining internal consistency and narrative depth.

## Architecture

The system follows an event-driven architecture with multiple interconnected services that communicate through a unified event bus (Redpanda). Key architectural principles include:

- **Event-Driven Architecture (EDA)**: All interactions occur through events in a unified bus (Redpanda)
- **Weak Coupling**: Services only know about events, not each other
- **Stateful Services with Recovery**: Each service maintains its state and recovers through snapshot + replay
- **Generativity over Scripting**: Qwen3 creates unique outcomes instead of choosing from presets
- **Ontological Awareness**: Knowledge about the world affects logic through ontological profiles

### Core Services

- **Entity Manager**: Manages hierarchical entities with history and references
- **Narrative Orchestrator (GM)**: Generates living, context-dependent narrative based on events
- **World Generator**: Generates new worlds, regions, and ontologies based on seed or AI
- **Ban Of World (Запрет Мира)**: Serves as guardian of reality integrity, detecting and neutralizing threats
- **City Governor**: Manages urban life: economy, NPCs, quests, mood
- **Cultivation Module**: Implements cultivation system: skills, dao, ascension
- **Reality Monitor**: Aggregates metrics from all worlds and publishes anomalies
- **Plan Manager**: Manages transitions between planes, fusion zones, availability of ascension
- **Ascension Oracle**: Generative AI oracle based on Qwen3 for unique ascension outcomes
- **Semantic Memory Builder**: Builds context for AI from events: vectors + knowledge graph
- **Ontological Archivist**: Stores and evolves world ontological schemas
- **Universe Genesis Oracle**: Generates fundamental plane hierarchy of the universe

## Technology Stack

- **Language**: Go 1.25
- **Event Streaming**: Redpanda (Kafka-compatible)
- **Object Storage**: MinIO
- **Vector Database**: ChromaDB
- **Graph Database**: Neo4j
- **Time-Series Database**: TimescaleDB
- **AI Model Serving**: Ollama with Qwen3
- **Containerization**: Docker & Docker Compose

## Building and Running

### Prerequisites

- Docker and Docker Compose
- Go 1.25+
- Git

### Quick Start

1. **Clone the repository**:
   ```bash
   git clone https://github.com/your-repo/multiverse-core.git
   cd multiverse-core
   ```

2. **Copy the environment template**:
   ```bash
   cp .env.example .env
   # Edit .env with your specific configurations
   ```

3. **Start infrastructure services**:
   ```bash
   docker-compose up -d
   ```

4. **Build and run individual services**:
   ```bash
   # Build a specific service
   go build -o bin/service-name ./cmd/service-name

   # Or use the Dockerfile directly
   docker build --build-arg SERVICE=service-name -t multiverse-core:service-name .
   ```

### Using Make Commands

The project includes a comprehensive Makefile with the following commands:

- `make build` - Build all services
- `make build-service SERVICE=<name>` - Build specific service
- `make up` - Start all services
- `make run SERVICE=<name>` - Start specific service
- `make down` - Stop all services
- `make logs` - Show logs for all services
- `make logs-service SERVICE=<name>` - Show logs for specific service
- `make clean` - Clean build artifacts
- `make test` - Run tests

### Local Development

For local development, you can run services individually while keeping infrastructure in Docker:

```bash
# Start infrastructure
docker-compose up redpanda minio chromadb neo4j

# Run a service locally
KAFKA_BROKERS=localhost:9092 MINIO_ENDPOINT=localhost:9000 go run cmd/entity-manager/main.go
```

## Testing

Run all tests with:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

## Configuration

Configuration is handled through:
- Environment variables (`.env` file)
- Service-specific configuration files
- Docker environment variables

Key configuration points:
- Kafka brokers endpoint
- MinIO credentials and endpoints
- Database connection strings
- AI model endpoints
- Service-specific settings

## Development Conventions

- All services are written in Go 1.25
- Use JSON Schema Draft 7 for entity payload validation
- Follow event-driven architecture with Kafka/Redpanda as message broker
- Services communicate through events in the event bus (topics: player_events, world_events, game_events, system_events, scope_management, narrative_output)
- Entity management uses MinIO for storage with bucket naming pattern: `entities-{world_id}`
- All services are stateful and support recovery via snapshots and event replay
- Use entity paths with dot notation for nested payload access (e.g., `payload.health.current`)
- All services must be built with CGO disabled for cross-platform compatibility
- Use UUIDs for event IDs and entity IDs
- All services must be containerized with Docker using multi-stage builds

## Directory Structure

- `cmd/` - Main application entry points for each service
- `services/` - Service implementations
- `configs/` - Configuration files
- `Docs/` - Documentation
- `internal/` - Internal packages
- `plans/` - Planning documents
- `reports/` - Reports and analytics
- `build/` - Build configuration
- `fake_deps/` - Fake dependencies for CGO issues

## Monitoring

The system includes:
- Event stream monitoring through Redpanda Console
- Database health checks
- Service logs aggregation
- Performance metrics in TimescaleDB

## Troubleshooting

Common issues and solutions:
- If services fail to connect to infrastructure, ensure all required containers are running with `docker-compose ps`
- Check logs with `make logs` or `docker-compose logs -f <service-name>`
- Verify environment variables in `.env` file match your setup
- Ensure sufficient system resources (RAM, disk space) for all services