# Living Worlds Integration Guide
**Version**: 1.0  
**Date**: 2026-02-22  
**Status**: Ready for Implementation

---

## 🎯 Overview

This guide provides step-by-step instructions for integrating the Living Worlds Entity-Actor architecture with the existing **multiverse-core** system.

### Integration Philosophy

**Living Worlds does NOT replace existing components** — it **enhances** them:

| Existing Component | Role | Living Worlds Enhancement |
|-------------------|------|---------------------------|
| **SemanticMemory** | Context storage & retrieval | Add entity-specific context endpoints |
| **NarrativeOrchestrator (GM)** | Narrative generation | Add mechanical results processing |
| **EntityManager** | Entity lifecycle management | Add Entity-Actor lifecycle |
| **EventBus (Kafka)** | Event routing | Add actor-specific topics |

---

## 📋 Integration Checklist

### Phase 1: Foundation Setup (Week 1)

#### ✅ 1.1 Create New Services

**Create directory structure:**
```bash
mkdir -p services/entityactor
mkdir -p services/evolutionwatcher
mkdir -p services/ruleengine
mkdir -p internal/tinyml
mkdir -p internal/rules
mkdir -p internal/intent
```

**Create service files:**
```bash
touch services/entityactor/{actor.go,model.go,service.go,api/handlers.go}
touch services/evolutionwatcher/{watcher.go,anomaly.go,service.go}
touch services/ruleengine/{engine.go,rule.go,validator.go}
```

#### ✅ 1.2 Update Docker Compose

Add new services to `docker-compose.yml`:

```yaml
services:
  # ... existing services ...
  
  entity-actor:
    build:
      context: .
      dockerfile: ./services/entityactor/Dockerfile
    ports:
      - "8081:8080"
    environment:
      - MINIO_ENDPOINT=minio:9090
      - REDIS_ENDPOINT=redis:6379
      - KAFKA_BROKERS=kafka:9092
      - ORACLE_ENDPOINT=http://oracle:8000
    depends_on:
      - minio
      - redis
      - kafka
      - oracle
  
  evolution-watcher:
    build:
      context: .
      dockerfile: ./services/evolutionwatcher/Dockerfile
    environment:
      - MINIO_ENDPOINT=minio:9090
      - KAFKA_BROKERS=kafka:9092
      - ORACLE_ENDPOINT=http://oracle:8000
    depends_on:
      - minio
      - kafka
      - oracle
  
  rule-engine:
    build:
      context: .
      dockerfile: ./services/ruleengine/Dockerfile
    ports:
      - "8082:8080"
    environment:
      - MINIO_ENDPOINT=minio:9090
    depends_on:
      - minio
```

#### ✅ 1.3 Create New Kafka Topics

Add to `kafka-topics.sh`:

```bash
#!/bin/bash

# New Living Worlds topics
kafka-topics --create --topic actor_events --partitions 10 --replication-factor 1 --bootstrap-server kafka:9092
kafka-topics --create --topic intent_requests --partitions 5 --replication-factor 1 --bootstrap-server kafka:9092
kafka-topics --create --topic evolution_proposals --partitions 3 --replication-factor 1 --bootstrap-server kafka:9092
kafka-topics --create --topic rule_updates --partitions 3 --replication-factor 1 --bootstrap-server kafka:9092
kafka-topics --create --topic evolution_analysis --partitions 5 --replication-factor 1 --bootstrap-server kafka:9092
```

---

### Phase 2: SemanticMemory Integration (Week 2)

#### ✅ 2.1 Add Entity Context Endpoint

**Update `services/semanticmemory/api_structured.go`:**

```go
// Add new endpoint for entity-specific context
func (s *Service) HandleEntityContext(w http.ResponseWriter, r *http.Request) {
    var req struct {
        EntityID string `json:"entity_id"`
        TimeRange string `json:"time_range"` // "last_1h", "last_24h", etc.
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Get entity context from ChromaDB + Neo4j
    context, err := s.storage.GetEntityContext(req.EntityID, req.TimeRange)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(context)
}
```

**Register in `services/semanticmemory/service.go`:**

```go
func (s *Service) RegisterRoutes(r *mux.Router) {
    // ... existing routes ...
    
    // New entity context endpoint
    r.HandleFunc("/v1/entity-context/{entity_id}", s.HandleEntityContext).Methods("GET")
}
```

#### ✅ 2.2 Update MinIO Buckets

Add new bucket for entity actors:

```bash
# Add to minio-setup.sh
mc mb myminio/gnue-entity-actors
mc policy set public myminio/gnue-entity-actors
```

---

### Phase 3: NarrativeOrchestrator Integration (Week 3)

#### ✅ 3.1 Add Mechanical Results Handler

**Update `services/narrativeorchestrator/orchestrator.go`:**

```go
// Add new method to handle mechanical results from Entity-Actors
func (no *NarrativeOrchestrator) HandleMechanicalResult(ev eventbus.Event) {
    // Extract mechanical result
    result := ev.Payload["mechanical_result"].(map[string]interface{})
    
    // Get GM for this scope
    gm, exists := no.gms[*ev.ScopeID]
    if !exists {
        log.Warn("No GM found for mechanical result", "scope_id", *ev.ScopeID)
        return
    }
    
    // Build narrative from mechanical result
    narrative := no.buildNarrativeFromMechanics(result, gm)
    
    // Publish narrative output
    narrativeEvent := eventbus.Event{
        Type: "narrative.generate",
        ScopeID: ev.ScopeID,
        Payload: map[string]interface{}{
            "narrative": narrative,
            "mood": result["mood"],
            "entity_id": result["entity_id"],
        },
    }
    
    no.eventBus.Produce("narrative_output", narrativeEvent)
}

// buildNarrativeFromMechanics converts mechanical results to narrative
func (no *NarrativeOrchestrator) buildNarrativeFromMechanics(
    result map[string]interface{},
    gm *GMInstance,
) string {
    // Use semantic layer from rule for narrative
    ruleID := result["rule_id"].(string)
    rule, err := no.ruleEngine.GetRule(ruleID)
    if err != nil {
        return "Something happened..."
    }
    
    // Get poetic description
    narrativeHook := rule.SemanticLayer.Descriptions["poetic"]
    
    // Add context from GM
    context := no.buildContext(gm)
    
    // Combine into final narrative
    return fmt.Sprintf("%s\n\n%s", narrativeHook, context)
}
```

#### ✅ 3.2 Subscribe to Mechanical Results

**Update `services/narrativeorchestrator/service.go`:**

```go
func (s *Service) Start() error {
    // ... existing subscriptions ...
    
    // Subscribe to mechanical results from Entity-Actors
    s.eventBus.Subscribe("mechanical_results", func(ev eventbus.Event) {
        s.orchestrator.HandleMechanicalResult(ev)
    })
    
    return nil
}
```

---

### Phase 4: EntityManager Integration (Week 4)

#### ✅ 4.1 Add Entity-Actor Lifecycle

**Update `services/entitymanager/manager.go`:**

```go
// Add Entity-Actor management
type EntityManager struct {
    // ... existing fields ...
    actorRegistry map[string]*entityactor.EntityActor
}

// CreateEntityActor creates a new Entity-Actor for an entity
func (em *EntityManager) CreateEntityActor(entityID string, entityType string) error {
    // Create actor
    actor, err := entityactor.NewEntityActor(
        entityID,
        entityType,
        em.ruleEngine,
        em.eventBus,
        em.minioClient,
        em.redisClient,
    )
    if err != nil {
        return fmt.Errorf("create actor: %w", err)
    }
    
    // Store in registry
    em.actorRegistry[entityID] = actor
    
    log.Info("Entity-Actor created", "entity_id", entityID)
    return nil
}

// DestroyEntityActor cleans up an Entity-Actor
func (em *EntityManager) DestroyEntityActor(entityID string) error {
    actor, exists := em.actorRegistry[entityID]
    if !exists {
        return fmt.Errorf("actor not found: %s", entityID)
    }
    
    // Save final state
    actor.saveState()
    
    // Cleanup
    delete(em.actorRegistry, entityID)
    actor.cancel()
    
    log.Info("Entity-Actor destroyed", "entity_id", entityID)
    return nil
}
```

#### ✅ 4.2 Handle Entity Events

**Add event handlers:**

```go
func (em *EntityManager) SubscribeToEvents() {
    // ... existing subscriptions ...
    
    // Create Entity-Actor when entity is created
    em.eventBus.Subscribe("entity.created", func(ev eventbus.Event) {
        entityID := ev.Payload["entity_id"].(string)
        entityType := ev.Payload["entity_type"].(string)
        em.CreateEntityActor(entityID, entityType)
    })
    
    // Destroy Entity-Actor when entity is deleted
    em.eventBus.Subscribe("entity.deleted", func(ev eventbus.Event) {
        entityID := ev.Payload["entity_id"].(string)
        em.DestroyEntityActor(entityID)
    })
}
```

---

### Phase 5: Testing & Validation (Week 5)

#### ✅ 5.1 Unit Tests

Create test files:

```bash
touch services/entityactor/actor_test.go
touch services/entityactor/model_test.go
touch services/ruleengine/engine_test.go
touch services/evolutionwatcher/watcher_test.go
```

**Example test:**

```go
// services/entityactor/actor_test.go
func TestEntityActor_ProcessEvent(t *testing.T) {
    // Setup
    actor, err := NewEntityActor(
        "test-player",
        "player",
        mockRuleEngine,
        mockKafka,
        mockMinio,
        mockRedis,
    )
    require.NoError(t, err)
    
    // Test event
    event := eventbus.Event{
        Type: "player.moved",
        Payload: map[string]interface{}{
            "to": map[string]float64{"x": 100, "y": 200},
        },
    }
    
    // Process
    err = actor.ProcessEvent(event)
    require.NoError(t, err)
    
    // Verify state updated
    assert.Greater(t, actor.state["position_x"], 0.0)
}
```

#### ✅ 5.2 Integration Tests

Create integration test:

```bash
touch tests/integration/living_worlds_test.go
```

**Example integration test:**

```go
func TestLivingWorlds_FullFlow(t *testing.T) {
    // 1. Create entity
    createEvent := eventbus.Event{
        Type: "entity.created",
        Payload: map[string]interface{}{
            "entity_id": "player-test",
            "entity_type": "player",
        },
    }
    kafka.Produce("world_events", createEvent)
    
    // 2. Wait for Entity-Actor creation
    time.Sleep(100 * time.Millisecond)
    
    // 3. Send player action
    actionEvent := eventbus.Event{
        Type: "player.action",
        Payload: map[string]interface{}{
            "action": "flirt",
            "target": "npc-bartender",
        },
    }
    kafka.Produce("world_events", actionEvent)
    
    // 4. Verify narrative output
    narrativeChan := make(chan eventbus.Event, 1)
    kafka.Subscribe("narrative_output", func(ev eventbus.Event) {
        narrativeChan <- ev
    })
    
    select {
    case ev := <-narrativeChan:
        assert.Contains(t, ev.Payload["narrative"].(string), "flirt")
    case <-time.After(5 * time.Second):
        t.Fatal("No narrative output received")
    }
}
```

---

### Phase 6: Monitoring & Observability (Week 6)

#### ✅ 6.1 Add Prometheus Metrics

**Update `services/entityactor/service.go`:**

```go
func init() {
    prometheus.MustRegister(prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "entity_actor_events_total",
            Help: "Total events processed by entity actors",
        },
        []string{"entity_type", "event_type"},
    ))
    
    prometheus.MustRegister(prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "entity_actor_inference_latency_ms",
            Help:    "Latency of neural inference",
            Buckets: prometheus.ExponentialBuckets(10, 1.5, 10),
        },
    ))
}
```

#### ✅ 6.2 Add Health Checks

**Add health endpoint:**

```go
func (s *Service) HandleHealth(w http.ResponseWriter, r *http.Request) {
    // Check dependencies
    minioHealthy := s.minioClient.IsOnline()
    redisHealthy := s.redisClient.Ping(context.Background()).Err() == nil
    
    if !minioHealthy || !redisHealthy {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]bool{
            "healthy": false,
            "minio": minioHealthy,
            "redis": redisHealthy,
        })
        return
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]bool{
        "healthy": true,
        "minio": true,
        "redis": true,
    })
}
```

---

## 🚀 Deployment Strategy

### Step 1: Deploy to Test Environment

```bash
# Build and deploy
docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d

# Verify services are running
docker ps | grep entity-actor
docker ps | grep evolution-watcher
docker ps | grep rule-engine
```

### Step 2: Load Test

```bash
# Install vegeta for load testing
go install github.com/tsenart/vegeta/v12@latest

# Create test scenario
cat > test-scenario.json <<EOF
{
  "target": "http://localhost:8081/v1/process-event",
  "method": "POST",
  "body": "{\"entity_id\":\"test-player\",\"event_type\":\"player.moved\"}"
}
EOF

# Run load test
vegeta attack -duration=5m -rate=100 -targets=test-scenario.json | vegeta report
```

### Step 3: Gradual Rollout

**Week 1**: 1% of entities  
**Week 2**: 10% of entities  
**Week 3**: 50% of entities  
**Week 4**: 100% of entities

---

## 📊 Monitoring Dashboard

Create Grafana dashboard:

```json
{
  "dashboard": {
    "title": "Living Worlds Metrics",
    "panels": [
      {
        "title": "Entity-Actor Events/sec",
        "targets": [
          {
            "expr": "rate(entity_actor_events_total[5m])",
            "legendFormat": "{{entity_type}} {{event_type}}"
          }
        ]
      },
      {
        "title": "Inference Latency (P99)",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, sum(rate(entity_actor_inference_latency_ms_bucket[5m])) by (le))"
          }
        ]
      },
      {
        "title": "Active Entity-Actors",
        "targets": [
          {
            "expr": "count(entity_actor_events_total)"
          }
        ]
      }
    ]
  }
}
```

---

## 🔧 Troubleshooting

### Issue: Entity-Actor not receiving events

**Solution:**
```bash
# Check Kafka consumer groups
kafka-consumer-groups --bootstrap-server kafka:9092 --describe --group entity_actor_group

# Verify topic exists
kafka-topics --bootstrap-server kafka:9092 --list | grep actor_events
```

### Issue: State restoration failing

**Solution:**
```bash
# Check MinIO bucket
mc ls myminio/gnue-entity-actors/

# Check Redis keys
redis-cli KEYS "entity_actor:*"
```

### Issue: Oracle timeout

**Solution:**
```bash
# Increase timeout in configuration
export ORACLE_TIMEOUT=120s

# Check Oracle health
curl http://oracle:8000/health
```

---

## 📚 Additional Resources

- **Architecture Docs**: `docs/LIVING_WORLDS_ARCHITECTURE.md`
- **Technical Spec**: `docs/ENTITY_ACTOR_TECHNICAL_SPEC.md`
- **SemanticMemory v2**: `docs/SEMANTIC_MEMORY_V2.md`
- **GM Documentation**: `docs/GM_COMPLETE_DOCUMENTATION.md`

---

## ✅ Final Checklist

Before production deployment:

- [ ] All unit tests passing
- [ ] Integration tests passing
- [ ] Load testing completed (1000+ entities)
- [ ] Monitoring dashboard configured
- [ ] Alerting rules set up
- [ ] Backup/restore tested
- [ ] Documentation complete
- [ ] Team training completed

---

**"Integration is not about replacing — it's about enhancing. Living Worlds breathes life into the existing multiverse."**  
*— Integration Philosophy, 2026*