# Multiverse-Core

A sophisticated distributed system designed for managing complex virtual worlds and narratives. This platform combines event-driven architecture, vector databases, graph databases, and AI-powered orchestration to create dynamic, evolving virtual environments.

## 🚀 Features

- **Event-Driven Architecture**: Built on Redpanda (Kafka-compatible) for scalable event streaming
- **Vector Storage**: ChromaDB for semantic memory and similarity search
- **Graph Knowledge Base**: Neo4j for relationship mapping and ontological structures
- **Object Storage**: MinIO for storing world snapshots and assets
- **AI Integration**: Qwen3 integration for narrative generation and decision-making
- **Modular Services**: Microservices architecture for scalability and maintainability
- **Time Series Metrics**: TimescaleDB for performance monitoring

## 🏗️ Architecture

The system consists of multiple interconnected services:

### Core Services
- **Entity Manager**: Manages virtual entities and their states
- **Narrative Orchestrator**: Coordinates storylines and narrative events
- **Semantic Memory**: Handles vector embeddings and semantic search
- **Ontological Archivist**: Maintains knowledge graphs and relationships
- **Plan Manager**: Orchestrates complex multi-step plans
- **World Generator**: Creates and manages virtual world environments
- **Universe Genesis Oracle**: Makes high-level decisions about world evolution
- **Cultivation Module**: Manages character progression and skill systems
- **Game Service**: Provides game-specific APIs and interfaces

### Infrastructure Components
- **Redpanda**: Event streaming platform
- **MinIO**: Object storage for snapshots and assets
- **ChromaDB**: Vector database for semantic memory
- **Neo4j**: Graph database for knowledge representation
- **TimescaleDB**: Time-series database for metrics
- **Qwen3**: AI model for narrative generation (via Ollama)

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