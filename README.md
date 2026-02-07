# Multiverse-Core

A sophisticated distributed system designed for managing complex virtual worlds and narratives. This platform combines event-driven architecture, vector databases, graph databases, and AI-powered orchestration to create dynamic, evolving virtual environments. The system implements a philosophy where worlds are not programmed but born, evolving organically through player actions while maintaining internal consistency and narrative depth.

## 🚀 Features

- **Event-Driven Architecture**: Built on Redpanda (Kafka-compatible) for scalable event streaming
- **Vector Storage**: ChromaDB for semantic memory and similarity search
- **Graph Knowledge Base**: Neo4j for relationship mapping and ontological structures
- **Object Storage**: MinIO for storing world snapshots and assets
- **AI Integration**: Qwen3 integration for narrative generation and decision-making
- **Modular Services**: Microservices architecture for scalability and maintainability
- **Time Series Metrics**: TimescaleDB for performance monitoring
- **Hierarchical Worlds**: Organized from unique base worlds → fusion zones → abstract planes → Source
- **Living Narratives**: Dynamic storylines that evolve naturally from player actions
- **Entity-Based Memory**: History stored within objects themselves rather than in separate systems

## 🏗️ Architecture

The system consists of multiple interconnected services that follow key architectural principles:

### Core Principles
- **Event-Driven Architecture (EDA)**: All interactions occur through events in a unified bus (Redpanda)
- **Weak Coupling**: Services only know about events, not each other
- **Stateful Services with Recovery**: Each service maintains its state and recovers through snapshot + replay
- **Generativity over Scripting**: Qwen3 creates unique outcomes instead of choosing from presets
- **Ontological Awareness**: Knowledge about the world affects logic through ontological profiles

### Core Services

#### Entity Manager
- **Purpose**: Manages hierarchical entities with history and references
- **Features**: 
  - Stateful: caches hot entities
  - Recoverable: snapshots in MinIO + replay from Redpanda
  - Shardable: by world_id
- **Events**: Subscribes to entity.create, entity.update, entity.link; publishes entity.created, entity.updated, entity.history.appended
- **Storage**: MinIO buckets: entities-{world_id}, snapshots: snapshots/em-{world_id}-v{N}.json

#### Narrative Orchestrator (GM)
- **Purpose**: Generates living, context-dependent narrative based on events in a given scope
- **Features**:
  - Stateful: stores semantic state of the area (fatigue, mood, etc.)
  - Dynamic: created/deleted based on scope events
  - Recoverable: aggregates state from Event Log
- **Scope Types**: solo, group, city, region, quest
- **Events**: Subscribes to entire world_events topic; publishes narrative.description, npc.action.*, weather.change.*

#### World Generator
- **Purpose**: Generates new worlds, regions, and ontologies based on seed or AI
- **Features**: Stateless, initiated manually or by event
- **Output**: World entity, ontological profile (in MinIO), world.generated event
- **Integrations**: Publishes to EntityManager, BanOfWorld, CityGovernor; uses Ascension Oracle for entity schema generation; saves schemas via HTTP to OntologicalArchivist

#### Ban Of World (Запрет Мира)
- **Purpose**: Serves as guardian of reality integrity, detecting and neutralizing threats that violate world ontology
- **Features**: Stateful, stores world health metrics; recoverable; parameterizable (one code, different ontologies)
- **Metrics**: spatial_integrity, karma_entropy, core_resonance
- **AI Integration**: Calls AscensionOracle as Oracle during anomalies; generates mythological consequences instead of penalties

#### City Governor
- **Purpose**: Manages urban life: economy, NPCs, quests, mood
- **Features**: Stateful (reputation, crime_rate, active_quests), recoverable
- **Events**: Subscribes to player.enter.city, trade.*, crime.*; publishes quest.issued, market.price_changed, festival.started
- **Features**: Reacts to group composition (rich/poor), can generate unique quests through AI

#### Cultivation Module
- **Purpose**: Implements cultivation system: skills, dao, ascension
- **Features**: Stateful (stores player profiles), hierarchical (modules at each plane level)
- **Events**: Subscribes to player.skill_use, ascension.triggered; publishes dao.portrait.updated, ascension.trial.started
- **Ascension**: Generates "Dao Portrait" from player history → passes to AscensionOracle

#### Reality Monitor
- **Purpose**: Aggregates metrics from all worlds and publishes anomalies
- **Features**: Stateful (aggregated metrics), real-time monitoring
- **Events**: Subscribes to world.metrics.*; publishes reality.anomaly.detected
- **Interaction**: Trigger for BanOfWorld and AscensionOracle

#### Plan Manager
- **Purpose**: Manages transitions between planes, fusion zones, availability of ascension
- **Features**: Stateful (plane graph as DAG), stores connections: who can go where
- **Events**: Subscribes to ascension.completed, planar.violation; publishes planar.transition.granted

#### Ascension Oracle
- **Purpose**: Generative AI oracle based on Qwen3. Creates unique ascension outcomes, trials, interventions
- **Features**: Stateless (HTTP client), RAG: context from SemanticMemory
- **Input**: Dao Portrait, world state, player history
- **Output**: JSON with narrative and new_events; can propose new mechanics, zones, laws

#### Semantic Memory Builder
- **Purpose**: Builds context for AI from events: vectors + knowledge graph
- **Features**: Stateless, indexes events in real-time, stores all system events for context and replay
- **Storage**: ChromaDB/Qdrant: event embeddings; Neo4j: relationships between entities and events
- **Interaction**: Used by AscensionOracle and GM through RAG

#### Ontological Archivist
- **Purpose**: Stores and evolves world ontological schemas
- **Features**: Stateful (versioned schemas), storage: MinIO (ontologies/{world_id}/v{N}.json)
- **Events**: Subscribes to world.generated, ontology.evolved; publishes ontology.published
- **Validation**: Provides schemas for validating events and entities

#### Universe Genesis Oracle
- **Purpose**: Generates fundamental plane hierarchy of the universe and basic ontological profiles for each level
- **Features**: Stateless/one-time, generates only the fundamental foundation of the Universe (Core, Laws)
- **Events**: Publishes universe.genesis.completed, entity.created (for Universe Core entity)
- **Interactions**: Uses Ascension Oracle for law and profile generation; saves profile to OntologicalArchivist via HTTP; publishes event for PlanManager and CosmicBan

### Infrastructure Components
- **Redpanda**: Event streaming platform
- **MinIO**: Object storage for snapshots and assets
- **ChromaDB**: Vector database for semantic memory
- **Neo4j**: Graph database for knowledge representation
- **TimescaleDB**: Time-series database for metrics
- **Qwen3**: AI model for narrative generation (via Ollama)

## 🌌 Philosophy

The system implements:
- Hierarchy of worlds leading to the unreachable Source (Plan Ω)
- Cultivation as a path from unique form to universal essence
- Narration as a natural, continuous process, not scripted reactions
- Integrity through the Ban of World, not through rules

**Goal**: Create a world that evolves through player actions while maintaining internal integrity and narrative depth.

## 🛠️ Prerequisites

- Docker and Docker Compose
- Go 1.25+
- Git

## 📦 Installation

1. Clone the repository:
```bash
git clone https://github.com/your-repo/multiverse-core.git
cd multiverse-core
```

2. Copy the environment template and configure your settings:
```bash
cp .env.example .env
# Edit .env with your specific configurations
```

3. Start the infrastructure services:
```bash
docker-compose up -d
```

4. Build and run individual services:
```bash
# Build a specific service
go build -o bin/service-name ./cmd/service-name

# Or use the Dockerfile directly
docker build --build-arg SERVICE=service-name -t multiverse-core:service-name .
```

## 🐳 Docker Compose Services

The project includes a comprehensive `docker-compose.yml` that sets up:

- **Redpanda**: Distributed streaming platform
- **MinIO**: S3-compatible object storage
- **ChromaDB**: Vector database for embeddings
- **Neo4j**: Graph database with APOC plugin
- **TimescaleDB**: Time-series database
- **Ollama + Qwen3**: AI model serving
- Multiple microservices with proper dependency ordering

## 🔧 Configuration

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

## 🧪 Development

### Building Services

Each service in the `cmd/` directory can be built independently:

```bash
# Build entity manager
go build -o bin/entity-manager ./cmd/entity-manager

# Build narrative orchestrator
go build -o bin/narrative-orchestrator ./cmd/narrative-orchestrator
```

### Running Tests

```bash
# Run all tests
go test ./...
# Run tests with coverage
go test -cover ./...
```

### Local Development

For local development, you can run services individually while keeping infrastructure in Docker:

```bash
# Start infrastructure
docker-compose up redpanda minio chromadb neo4j

# Run a service locally
KAFKA_BROKERS=localhost:9092 MINIO_ENDPOINT=localhost:9000 go run cmd/entity-manager/main.go
```

## 📊 Monitoring

The system includes:
- Event stream monitoring through Redpanda Console
- Database health checks
- Service logs aggregation
- Performance metrics in TimescaleDB

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

For support, please open an issue in the GitHub repository or contact the maintainers.

## 🙏 Acknowledgments

- Redpanda team for the excellent streaming platform
- ChromaDB for vector database capabilities
- Neo4j for graph database technology
- Ollama and Qwen teams for AI model serving