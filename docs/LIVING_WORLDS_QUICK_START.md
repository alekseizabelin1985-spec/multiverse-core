# Living Worlds Quick Start Guide
**Version**: 1.0  
**Date**: 2026-02-22  
**Status**: Ready to Use

---

## 🚀 Quick Start (15 Minutes)

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Existing multiverse-core setup
- MinIO, Redis, Kafka running

### Step 1: Checkout the Branch

```bash
git clone https://github.com/alekseizabelin1985-spec/multiverse-core.git
cd multiverse-core
git checkout feat/living-worlds-entity-actor
```

### Step 2: Build & Deploy

```bash
# Build services
docker-compose build entity-actor evolution-watcher rule-engine

# Deploy
docker-compose up -d entity-actor evolution-watcher rule-engine

# Verify
docker ps | grep -E "entity-actor|evolution-watcher|rule-engine"
```

### Step 3: Create Test Entity

```bash
# Send entity creation event
cat > create_entity.json <<EOF
{
  "event_type": "entity.created",
  "payload": {
    "entity_id": "player-test-001",
    "entity_type": "player",
    "payload": {
      "name": "Test Player",
      "stats": {
        "charisma": 0.8,
        "strength": 0.6,
        "intelligence": 0.7
      }
    }
  }
}
EOF

# Send to Kafka
cat create_entity.json | kafkacat -P -b localhost:9092 -t world_events
```

### Step 4: Test Intent Recognition

```bash
# Send player action
cat > player_action.json <<EOF
{
  "event_type": "player.action",
  "scope_id": "player-test-001",
  "payload": {
    "action_text": "I try to flirt with the bartender, giving her a charming smile",
    "target": "npc-bartender-001"
  }
}
EOF

cat player_action.json | kafkacat -P -b localhost:9092 -t world_events
```

### Step 5: Check Results

```bash
# Monitor narrative output
kafkacat -C -b localhost:9092 -t narrative_output -o end

# Check entity state
curl http://localhost:8081/v1/entity/player-test-001/state

# Check health
curl http://localhost:8081/health
```

---

## 📝 Example Scenarios

### Scenario 1: Player Flirting with NPC

**Event Flow:**
```
1. Player sends text: "I try to flirt with the bartender..."
2. Intent Recognition Layer detects "social_connection_attempt"
3. Rule Engine applies: d10 + charisma
4. Entity-Actor calculates result
5. GM generates narrative
6. Player sees: "The bartender smiles warmly..."
```

**Code Example:**
```go
// Send player action
event := eventbus.Event{
    Type: "player.action",
    ScopeID: strPtr("player-test-001"),
    Payload: map[string]interface{}{
        "action_text": "I try to flirt with the bartender...",
        "target": "npc-bartender-001",
    },
}
kafka.Produce("world_events", event)
```

### Scenario 2: Rabbit Evolving Perfect Stillness

**Event Flow:**
```
1. Rabbit successfully hides 100 times from predators
2. Evolution Watcher detects anomaly (deviation: 3.2σ)
3. Oracle generates "perfect_stillness" rule
4. Rule Validator approves
5. Rule Engine adds new rule
6. Rabbit now has automatic success when still near predators
```

**Monitoring:**
```bash
# Check evolution proposals
kafkacat -C -b localhost:9092 -t evolution_proposals -o end

# Check rule updates
curl http://localhost:8082/v1/rules?entity_type=rabbit
```

### Scenario 3: Weather Affecting Entity State

**Event Flow:**
```
1. Weather event: "rain started in forest_north"
2. Entity-Actor receives event
3. Model updates state: wetness += 0.15, temperature -= 2°C
4. If player has fire nearby: apply heat gradient
5. State saved to Redis + MinIO
```

**Event Example:**
```json
{
  "event_type": "weather.rain",
  "region": "forest_north",
  "payload": {
    "intensity": 0.8,
    "duration_minutes": 30
  }
}
```

---

## 🔧 Configuration

### Environment Variables

```bash
# Entity-Actor
export ENTITY_ACTOR_PORT=8081
export MINIO_ENDPOINT=minio:9090
export REDIS_ENDPOINT=redis:6379
export KAFKA_BROKERS=kafka:9092
export ORACLE_ENDPOINT=http://oracle:8000

# Evolution Watcher
export EVOLUTION_WATCHER_PORT=8083
export ANOMALY_THRESHOLD=0.85
export ANALYSIS_BATCH_SIZE=50

# Rule Engine
export RULE_ENGINE_PORT=8082
export RULE_CACHE_SIZE=1000
export RULE_VALIDATION_REQUIRED=true
```

### Kafka Topics

| Topic | Purpose | Partitions |
|-------|---------|------------|
| `actor_events` | Entity-Actor events | 10 |
| `intent_requests` | Intent recognition requests | 5 |
| `evolution_proposals` | New rule proposals | 3 |
| `rule_updates` | Rule updates | 3 |
| `evolution_analysis` | Batch analysis | 5 |
| `mechanical_results` | Results from Entity-Actors | 10 |

---

## 📊 Monitoring Commands

### Check Service Health

```bash
# Entity-Actor
curl http://localhost:8081/health

# Rule Engine
curl http://localhost:8082/health

# Evolution Watcher
curl http://localhost:8083/health
```

### Monitor Metrics

```bash
# Prometheus metrics
curl http://localhost:8081/metrics | grep entity_actor

# Kafka consumer lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group entity_actor_group
```

### Check Entity State

```bash
# Get entity state
curl http://localhost:8081/v1/entity/player-test-001/state

# Get entity rules
curl http://localhost:8082/v1/rules?entity_id=player-test-001
```

---

## 🐛 Troubleshooting

### Issue: Entity-Actor not starting

**Check logs:**
```bash
docker logs entity-actor-1
```

**Common causes:**
- MinIO not reachable → Check `MINIO_ENDPOINT`
- Redis not running → Start Redis
- Kafka topic missing → Run `kafka-topics.sh`

### Issue: Intent Recognition failing

**Check Oracle:**
```bash
curl http://oracle:8000/health
```

**Check cache:**
```bash
redis-cli KEYS "intent_cache:*"
```

### Issue: Evolution not triggering

**Check anomaly threshold:**
```bash
# Lower threshold for testing
export ANOMALY_THRESHOLD=0.5
```

**Check event count:**
```bash
# Evolution requires minimum events
kafkacat -C -b localhost:9092 -t actor_events -c 100
```

---

## 📚 API Reference

### Entity-Actor API

#### Get Entity State
```bash
GET /v1/entity/{entity_id}/state
```

**Response:**
```json
{
  "entity_id": "player-test-001",
  "state": {
    "wetness": 0.15,
    "temperature": 35.2,
    "charisma": 0.8
  },
  "model_version": "v1.2",
  "last_updated": "2026-02-22T10:30:00Z"
}
```

#### Process Event
```bash
POST /v1/entity/{entity_id}/process-event
Content-Type: application/json

{
  "event_type": "player.moved",
  "payload": {
    "to": {"x": 100, "y": 200}
  }
}
```

### Rule Engine API

#### Get Rules
```bash
GET /v1/rules?entity_id=player-test-001&entity_type=player
```

#### Create Rule
```bash
POST /v1/rules
Content-Type: application/json

{
  "rule_id": "custom_rule_001",
  "mechanical_core": {
    "dice_formula": "d8 + intelligence",
    "base_difficulty": 10
  },
  "semantic_layer": {
    "name": "Custom Rule",
    "descriptions": {
      "mechanical": "Custom rule for testing",
      "poetic": "A special ability"
    }
  }
}
```

---

## 🎯 Next Steps

1. **Read Architecture**: `docs/LIVING_WORLDS_ARCHITECTURE.md`
2. **Review Technical Spec**: `docs/ENTITY_ACTOR_TECHNICAL_SPEC.md`
3. **Follow Integration Guide**: `docs/LIVING_WORLDS_INTEGRATION_GUIDE.md`
4. **Run Full Test Suite**: `go test ./services/entityactor/...`
5. **Deploy to Production**: Follow rollout strategy in integration guide

---

## 📞 Support

- **Documentation**: See `/docs` directory
- **Issues**: https://github.com/alekseizabelin1985-spec/multiverse-core/issues
- **Discussions**: https://github.com/alekseizabelin1985-spec/multiverse-core/discussions

---

## ✅ Quick Checklist

Before starting:

- [ ] Docker & Docker Compose installed
- [ ] Existing multiverse-core running
- [ ] MinIO, Redis, Kafka accessible
- [ ] Go 1.22+ installed
- [ ] Branch `feat/living-worlds-entity-actor` checked out

After deployment:

- [ ] Entity-Actor service running
- [ ] Evolution Watcher running
- [ ] Rule Engine running
- [ ] Kafka topics created
- [ ] Health checks passing
- [ ] Test entity created
- [ ] Intent recognition working
- [ ] Narrative output visible

---

**"Start small, think big. Your first entity is the seed of a living world."**  
*— Quick Start Philosophy, 2026*