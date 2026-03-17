# Entity-Actor Technical Specification
**Version**: 1.0  
**Date**: 2026-02-22  
**Status**: Implementation Ready

---

## 📦 Package Structure

```
services/entityactor/
├── actor.go                    # EntityActor core implementation
├── model.go                    # TinyModel neural network
├── rule_engine.go              # Rule Engine integration
├── intent_recognition.go       # Intent Recognition Layer
├── evolution_watcher.go        # Evolution Watcher (separate service)
├── state_persistence.go        # State persistence logic
├── service.go                  # HTTP service and Kafka consumers
└── api/
    ├── handlers.go             # HTTP endpoints
    └── types.go                # Request/Response types

internal/
├── tinyml/                     # TinyML model utilities
│   ├── tflite_wrapper.go
│   ├── model_loader.go
│   └── inference.go
├── rules/                      # Rule Engine
│   ├── rule.go
│   ├── engine.go
│   └── validator.go
└── intent/                     # Intent Recognition
    ├── oracle_client.go
    ├── cache.go
    └── prompt_builder.go
```

---

## 🔧 Core Implementation

### EntityActor Structure

```go
package entityactor

import (
    "context"
    "time"
    
    "github.com/alekseizabelin1985-spec/multiverse-core/internal/tinyml"
    "github.com/alekseizabelin1985-spec/multiverse-core/internal/rules"
    eventbus "github.com/alekseizabelin1985-spec/multiverse-core/internal/eventbus"
)

type EntityActor struct {
    // Identity
    entityID   string
    entityType string
    
    // State management
    state      map[string]float32
    model      *tinyml.TinyModel
    ruleEngine *rules.Engine
    
    // Event processing
    eventBuffer []eventbus.Event
    lastProcess time.Time
    
    // Infrastructure
    kafkaProducer eventbus.Producer
    minioClient   *minio.Client
    redisClient   *redis.Client
    
    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
}

// NewEntityActor creates a new entity actor instance
func NewEntityActor(
    entityID string,
    entityType string,
    ruleEngine *rules.Engine,
    kafkaProducer eventbus.Producer,
    minioClient *minio.Client,
    redisClient *redis.Client,
) (*EntityActor, error) {
    ctx, cancel := context.WithCancel(context.Background())
    
    actor := &EntityActor{
        entityID:      entityID,
        entityType:    entityType,
        state:         make(map[string]float32),
        ruleEngine:    ruleEngine,
        kafkaProducer: kafkaProducer,
        minioClient:   minioClient,
        redisClient:   redisClient,
        eventBuffer:   make([]eventbus.Event, 0, 10),
        ctx:           ctx,
        cancel:        cancel,
    }
    
    // Restore state from persistence
    if err := actor.restoreState(); err != nil {
        return nil, err
    }
    
    return actor, nil
}
```

### Event Processing

```go
// ProcessEvent handles a single event for this entity
func (ea *EntityActor) ProcessEvent(event eventbus.Event) error {
    // 1. Preprocess event to feature vector
    features, err := ea.preprocessEvent(event)
    if err != nil {
        return fmt.Errorf("preprocess event: %w", err)
    }
    
    // 2. Neural inference - update internal state
    newState, err := ea.model.Run(features)
    if err != nil {
        return fmt.Errorf("model inference: %w", err)
    }
    ea.state = newState
    
    // 3. Check if event has structured action (from Intent Recognition)
    if event.Payload["structured_action"] != nil {
        action := event.Payload["structured_action"].(map[string]interface{})
        
        // Apply rule if action is known
        result, err := ea.ruleEngine.Apply(
            action["base_action"].(string),
            ea.state,
            action["modifiers"].([]map[string]interface{}),
        )
        if err != nil {
            log.Warn("Rule application failed", "error", err)
        }
        
        // Publish result
        if result != nil {
            ea.publishResult(result)
        }
    }
    
    // 4. Buffer event for batch processing
    ea.eventBuffer = append(ea.eventBuffer, event)
    
    // 5. Check if batch is ready for processing
    if len(ea.eventBuffer) >= 10 || time.Since(ea.lastProcess) > 5*time.Second {
        if err := ea.processBatch(); err != nil {
            log.Error("Batch processing failed", "error", err)
        }
    }
    
    return nil
}

// processBatch processes buffered events and sends for evolution analysis
func (ea *EntityActor) processBatch() error {
    if len(ea.eventBuffer) == 0 {
        return nil
    }
    
    // Send events to evolution watcher for analysis
    evolutionEvent := eventbus.Event{
        Type: "actor.batch_processed",
        Payload: map[string]interface{}{
            "entity_id":   ea.entityID,
            "entity_type": ea.entityType,
            "events":      ea.eventBuffer,
            "state":       ea.state,
        },
        Timestamp: time.Now(),
    }
    
    if err := ea.kafkaProducer.Produce("evolution_analysis", evolutionEvent); err != nil {
        log.Error("Failed to send to evolution analysis", "error", err)
    }
    
    // Clear buffer
    ea.eventBuffer = ea.eventBuffer[:0]
    ea.lastProcess = time.Now()
    
    return nil
}
```

### State Persistence

```go
// restoreState restores actor state from persistence
func (ea *EntityActor) restoreState() error {
    // Try Redis first (hot cache)
    redisKey := fmt.Sprintf("entity_actor:%s:state", ea.entityID)
    if cached, err := ea.redisClient.Get(redisKey).Result(); err == nil {
        var state map[string]float32
        if err := json.Unmarshal([]byte(cached), &state); err == nil {
            ea.state = state
            log.Info("State restored from Redis", "entity", ea.entityID)
            return nil
        }
    }
    
    // Try MinIO (cold storage)
    objectKey := fmt.Sprintf("entity_actors/%s/state.json", ea.entityID)
    obj, err := ea.minioClient.GetObject(context.Background(), "gnue-state", objectKey, minio.GetObjectOptions{})
    if err == nil {
        defer obj.Close()
        
        var snapshot StateSnapshot
        if err := json.NewDecoder(obj).Decode(&snapshot); err == nil {
            ea.state = snapshot.State
            ea.model = tinyml.LoadModel(snapshot.ModelVersion)
            log.Info("State restored from MinIO", "entity", ea.entityID)
            return nil
        }
    }
    
    // Create default state for new entities
    ea.state = ea.createDefaultState()
    ea.model = tinyml.LoadBaseModel(ea.entityType)
    log.Warn("Created new state for entity", "entity", ea.entityID)
    
    return nil
}

// saveState persists current state
func (ea *EntityActor) saveState() error {
    // Save to Redis (hot cache)
    redisKey := fmt.Sprintf("entity_actor:%s:state", ea.entityID)
    stateJSON, _ := json.Marshal(ea.state)
    ea.redisClient.Set(redisKey, string(stateJSON), 24*time.Hour)
    
    // Save to MinIO (cold storage)
    snapshot := StateSnapshot{
        EntityID:    ea.entityID,
        State:       ea.state,
        ModelVersion: ea.model.Version(),
        Timestamp:   time.Now(),
    }
    
    snapshotJSON, _ := json.Marshal(snapshot)
    reader := bytes.NewReader(snapshotJSON)
    
    _, err := ea.minioClient.PutObject(
        context.Background(),
        "gnue-state",
        fmt.Sprintf("entity_actors/%s/state.json", ea.entityID),
        reader,
        int64(len(snapshotJSON)),
        minio.PutObjectOptions{ContentType: "application/json"},
    )
    
    if err != nil {
        log.Error("Failed to save state to MinIO", "error", err)
        return err
    }
    
    log.Debug("State saved", "entity", ea.entityID)
    return nil
}
```

---

## 🎲 Rule Engine Implementation

### Rule Structure

```go
package rules

type Rule struct {
    ID             string                 `json:"rule_id"`
    MechanicalCore MechanicalCore         `json:"mechanical_core"`
    SemanticLayer  SemanticLayer          `json:"semantic_layer"`
    Version        int                    `json:"version"`
    CreatedAt      time.Time              `json:"created_at"`
}

type MechanicalCore struct {
    DiceFormula        string              `json:"dice_formula"`        // "d10 + charisma"
    BaseDifficulty     int                 `json:"base_difficulty"`     // 12
    ContextualModifiers []Modifier         `json:"contextual_modifiers"`
    SuccessThreshold   string              `json:"success_threshold"`   // "total >= difficulty"
    SensoryEffects     map[string]float32  `json:"sensory_effects"`     // visibility, sound, etc.
}

type Modifier struct {
    Condition string `json:"condition"` // "environment == 'intimate'"
    Modifier  int    `json:"modifier"`  // +3
}

type SemanticLayer struct {
    Name        string            `json:"name"`
    Descriptions map[string]string `json:"descriptions"` // mechanical, poetic
}
```

### Rule Engine Core

```go
type Engine struct {
    rules      sync.Map // rule_id -> *Rule
    cache      *lru.Cache
    minio      *minio.Client
}

// Apply executes a rule with given context
func (e *Engine) Apply(
    ruleID string,
    state map[string]float32,
    modifiers []map[string]interface{},
) (*Result, error) {
    // Get rule from cache or load
    rule, err := e.getRule(ruleID)
    if err != nil {
        return nil, fmt.Errorf("get rule: %w", err)
    }
    
    // Calculate base roll
    diceResult, err := e.rollDice(rule.MechanicalCore.DiceFormula, state)
    if err != nil {
        return nil, fmt.Errorf("roll dice: %w", err)
    }
    
    // Apply contextual modifiers
    total := diceResult
    for _, mod := range rule.MechanicalCore.ContextualModifiers {
        if e.evaluateCondition(mod.Condition, state, modifiers) {
            total += mod.Modifier
        }
    }
    
    // Determine success
    success := total >= rule.MechanicalCore.BaseDifficulty
    
    return &Result{
        RuleID:      ruleID,
        DiceRoll:    diceResult,
        Total:       total,
        Success:     success,
        SensoryEffects: rule.MechanicalCore.SensoryEffects,
    }, nil
}

// rollDice parses and executes dice formula
func (e *Engine) rollDice(formula string, state map[string]float32) (int, error) {
    // Parse: "d10 + charisma"
    parts := strings.Split(formula, "+")
    
    // Roll dice
    dicePart := strings.TrimSpace(parts[0])
    diceType := strings.TrimPrefix(dicePart, "d")
    maxRoll, _ := strconv.Atoi(diceType)
    diceRoll := rand.Intn(maxRoll) + 1
    
    // Add stat modifier
    if len(parts) > 1 {
        statName := strings.TrimSpace(parts[1])
        statValue := int(state[statName] * 10) // Scale 0.0-1.0 to 0-10
        diceRoll += statValue
    }
    
    return diceRoll, nil
}
```

---

## 🔍 Intent Recognition Layer

### Oracle Client

```go
package intent

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type OracleClient struct {
    endpoint string
    apiKey   string
    httpClient *http.Client
    cache      *Cache
}

type IntentRequest struct {
    PlayerInput string                 `json:"player_input"`
    Context     map[string]interface{} `json:"context"`
    ExistingRules []string             `json:"existing_rules"`
}

type IntentResponse struct {
    Interpretation string                 `json:"interpretation"`
    BaseAction     string                 `json:"base_action"`
    PrimaryStat    string                 `json:"primary_stat"`
    Modifiers      []map[string]interface{} `json:"contextual_modifiers"`
    NarrativeHook  string                 `json:"narrative_hook"`
    NewMechanic    *NewMechanicProposal   `json:"new_mechanic_proposal,omitempty"`
}

func (oc *OracleClient) RecognizeIntent(input string, context map[string]interface{}) (*IntentResponse, error) {
    // Check cache first
    cacheKey := generateCacheKey(input, context)
    if cached, ok := oc.cache.Get(cacheKey); ok {
        return cached.(*IntentResponse), nil
    }
    
    // Build prompt
    prompt := oc.buildPrompt(input, context)
    
    // Call Oracle
    reqBody := map[string]interface{}{
        "model": "qwen3",
        "prompt": prompt,
        "temperature": 0.7,
        "max_tokens": 500,
    }
    
    reqJSON, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", oc.endpoint+"/v1/chat/completions", bytes.NewReader(reqJSON))
    req.Header.Set("Authorization", "Bearer "+oc.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    req = req.WithContext(ctx)
    
    resp, err := oc.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("oracle request: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("oracle returned %d", resp.StatusCode)
    }
    
    // Parse response
    var oracleResp OracleResponse
    if err := json.NewDecoder(resp.Body).Decode(&oracleResp); err != nil {
        return nil, fmt.Errorf("parse oracle response: %w", err)
    }
    
    // Extract intent
    var intent IntentResponse
    if err := json.Unmarshal([]byte(oracleResp.Choices[0].Message.Content), &intent); err != nil {
        return nil, fmt.Errorf("parse intent: %w", err)
    }
    
    // Cache result
    oc.cache.Set(cacheKey, &intent, 24*time.Hour)
    
    return &intent, nil
}
```

---

## 🌱 Evolution Watcher (Separate Service)

```go
package evolutionwatcher

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/alekseizabelin1985-spec/multiverse-core/internal/eventbus"
)

type EvolutionWatcher struct {
    kafkaConsumer eventbus.Consumer
    anomalyModel  *AnomalyDetector
    oracleClient  *OracleClient
    minioClient   *minio.Client
}

func (ew *EvolutionWatcher) Run(ctx context.Context) {
    for {
        select {
        case event := <-ew.kafkaConsumer.Events():
            if event.Type == "actor.batch_processed" {
                go ew.analyzeBatch(event)
            }
        case <-ctx.Done():
            return
        }
    }
}

func (ew *EvolutionWatcher) analyzeBatch(event eventbus.Event) {
    // Extract entity events
    events := event.Payload["events"].([]eventbus.Event)
    entityID := event.Payload["entity_id"].(string)
    
    // Detect anomaly
    anomalyScore, err := ew.anomalyModel.Detect(events)
    if err != nil {
        log.Error("Anomaly detection failed", "error", err)
        return
    }
    
    // If anomaly is significant, request Oracle generation
    if anomalyScore > 0.85 {
        context := ew.buildContext(entityID, events)
        
        proposal, err := ew.oracleClient.GenerateRule(context)
        if err != nil {
            log.Error("Oracle rule generation failed", "error", err)
            return
        }
        
        // Send proposal to validator
        validationEvent := eventbus.Event{
            Type: "evolution.proposal",
            Payload: map[string]interface{}{
                "entity_id": entityID,
                "proposal":  proposal,
                "context":   context,
            },
        }
        
        ew.kafkaConsumer.Produce("rule_validation", validationEvent)
    }
}
```

---

## 📊 Performance Optimizations

### Model Quantization

```go
// QuantizeModel reduces model precision for faster inference
func QuantizeModel(model *tflite.Interpreter) error {
    // INT8 quantization
    quantizer := tflite.NewQuantizer(model)
    quantized, err := quantizer.Quantize(
        tflite.QuantizationConfig{
            Precision: tflite.INT8,
            CalibrationData: loadCalibrationData(),
        },
    )
    if err != nil {
        return err
    }
    
    // Replace model with quantized version
    *model = *quantized
    return nil
}
```

### Batch Processing

```go
// ProcessBatch handles multiple entities in parallel
func (service *EntityActorService) ProcessBatch(events []eventbus.Event) {
    // Group events by entity
    entityEvents := make(map[string][]eventbus.Event)
    for _, event := range events {
        entityID := event.Payload["entity_id"].(string)
        entityEvents[entityID] = append(entityEvents[entityID], event)
    }
    
    // Process in parallel
    var wg sync.WaitGroup
    for entityID, events := range entityEvents {
        wg.Add(1)
        go func(id string, evts []eventbus.Event) {
            defer wg.Done()
            
            if actor, exists := service.actors.Load(id); exists {
                for _, event := range evts {
                    actor.(*EntityActor).ProcessEvent(event)
                }
            }
        }(entityID, events)
    }
    wg.Wait()
}
```

---

## 🔒 Safety & Validation

### Rule Validator

```go
func (rv *RuleValidator) Validate(proposal *RuleProposal) (*ValidationResult, error) {
    // 1. Safety check
    if err := rv.checkSafety(proposal); err != nil {
        return &ValidationResult{
            Approved: false,
            Reason:   "safety_violation",
            Details:  err.Error(),
        }, nil
    }
    
    // 2. Balance simulation
    balanceScore, err := rv.simulateBalance(proposal)
    if err != nil || balanceScore < 0.7 {
        return &ValidationResult{
            Approved: false,
            Reason:   "balance_issue",
            Details:  fmt.Sprintf("score: %.2f", balanceScore),
        }, nil
    }
    
    // 3. Cultural sensitivity
    if err := rv.checkCulturalSensitivity(proposal); err != nil {
        return &ValidationResult{
            Approved: false,
            Reason:   "cultural_issue",
            Details:  err.Error(),
        }, nil
    }
    
    return &ValidationResult{
        Approved: true,
        BalanceScore: balanceScore,
    }, nil
}
```

---

## 🚀 Deployment Configuration

### Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev linux-headers
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-extldflags "-static"' \
    -o /bin/entity-actor ./services/entityactor

FROM alpine:3.18
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /bin/entity-actor .
COPY configs/ /app/configs/

EXPOSE 8080
ENV MINIO_ENDPOINT=minio:9090
ENV REDIS_ENDPOINT=redis:6379
ENV KAFKA_BROKERS=kafka:9092
ENV ORACLE_ENDPOINT=http://oracle:8000

CMD ["/app/entity-actor"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: entity-actors
spec:
  replicas: 10
  selector:
    matchLabels:
      app: entity-actors
  template:
    metadata:
      labels:
        app: entity-actors
    spec:
      containers:
      - name: entity-actor
        image: entity-actor:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "200m"
        env:
        - name: MINIO_ENDPOINT
          value: "minio:9090"
        - name: REDIS_ENDPOINT
          value: "redis-cluster:6379"
        - name: KAFKA_BROKERS
          value: "kafka-broker-1:9092,kafka-broker-2:9092"
```

---

## 📈 Monitoring & Metrics

```go
// Metrics exported to Prometheus
var (
    actorInferenceLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "actor_inference_latency_ms",
        Help:    "Time taken for neural inference",
        Buckets: prometheus.ExponentialBuckets(10, 1.5, 10),
    })
    
    actorStateRestoreTime = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "actor_state_restore_ms",
        Help:    "Time taken to restore actor state",
        Buckets: prometheus.ExponentialBuckets(10, 2, 10),
    })
    
    ruleApplicationCount = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "rule_application_total",
        Help: "Total rule applications by type",
    }, []string{"rule_type", "success"})
)

func init() {
    prometheus.MustRegister(actorInferenceLatency)
    prometheus.MustRegister(actorStateRestoreTime)
    prometheus.MustRegister(ruleApplicationCount)
}
```

---

## 📚 References

- **Living Worlds Architecture**: See `docs/LIVING_WORLDS_ARCHITECTURE.md`
- **SemanticMemory v2**: See `docs/SEMANTIC_MEMORY_V2.md`
- **GM Architecture**: See `docs/GM_COMPLETE_DOCUMENTATION.md`
- **Event System**: See `internal/eventbus/README.md`

---

**"The entity is not a puppet of rules — it is a living neural network that breathes through experience."**  
*— Entity-Actor Philosophy, 2026*