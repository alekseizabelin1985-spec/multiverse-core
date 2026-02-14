// services/narrativeorchestrator/orchestrator.go

package narrativeorchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"multiverse-core/internal/config"
	"multiverse-core/internal/eventbus"
	"multiverse-core/internal/minio"
	"multiverse-core/internal/spatial"

	"github.com/google/uuid"
)

// Log levels
const (
	DEBUG = "DEBUG"
	INFO  = "INFO"
	WARN  = "WARN"
	ERROR = "ERROR"
)

// Structured logging function
func structuredLog(level, scopeID, worldID, msg string, fields map[string]interface{}) {
	fieldStr := ""
	for k, v := range fields {
		fieldStr += fmt.Sprintf(" %s=%v", k, v)
	}

	if scopeID != "" {
		fieldStr = fmt.Sprintf(" scope_id=%s%s", scopeID, fieldStr)
	}
	if worldID != "" {
		fieldStr = fmt.Sprintf(" world_id=%s%s", worldID, fieldStr)
	}

	log.Printf("[%s] %s%s", level, msg, fieldStr)
}

// Helper functions for different log levels
func debugLog(scopeID, worldID, msg string, fields map[string]interface{}) {
	structuredLog(DEBUG, scopeID, worldID, msg, fields)
}

func infoLog(scopeID, worldID, msg string, fields map[string]interface{}) {
	structuredLog(INFO, scopeID, worldID, msg, fields)
}

func warnLog(scopeID, worldID, msg string, fields map[string]interface{}) {
	structuredLog(WARN, scopeID, worldID, msg, fields)
}

func errorLog(scopeID, worldID, msg string, fields map[string]interface{}) {
	structuredLog(ERROR, scopeID, worldID, msg, fields)
}

type SemanticMemoryClient struct {
	BaseURL string
}

type GetContextResponse struct {
	Contexts map[string]string `json:"contexts"`
}

// ContextWithEventsRequest для /v1/context-with-events.
type ContextWithEventsRequest struct {
	EntityIDs  []string `json:"entity_ids"`
	EventTypes []string `json:"event_types,omitempty"`
	Depth      int      `json:"depth,omitempty"`
}

type ContextWithEventsResponse struct {
	Contexts map[string]interface{} `json:"contexts"`
}

func (c *SemanticMemoryClient) GetContextWithEvents(ctx context.Context, entityIDs []string, eventTypes []string, depth int) (map[string]interface{}, error) {
	debugLog("", "", "Getting context with events from semantic memory", map[string]interface{}{
		"entity_ids":  entityIDs,
		"event_types": eventTypes,
		"depth":       depth,
	})

	reqBody, err := json.Marshal(ContextWithEventsRequest{
		EntityIDs:  entityIDs,
		EventTypes: eventTypes,
		Depth:      depth,
	})
	if err != nil {
		errorLog("", "", "Failed to marshal context request", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	resp, err := http.Post(c.BaseURL+"/v1/context-with-events", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		errorLog("", "", "Failed to call semantic memory service", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	var result ContextWithEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		errorLog("", "", "Failed to decode semantic memory response", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	infoLog("", "", "Successfully retrieved context with events from semantic memory", map[string]interface{}{
		"context_count": len(result.Contexts),
	})
	return result.Contexts, nil
}

type StateSnapshot struct {
	Entities map[string]interface{} `json:"entities"`
	Canon    []string               `json:"canon"`
	LastMood []string               `json:"last_mood,omitempty"`
}

type NarrativeOrchestrator struct {
	gms         map[string]*GMInstance
	mu          sync.RWMutex
	bus         *eventbus.EventBus
	semantic    *SemanticMemoryClient
	minioClient minio.ClientInterface
	configStore *config.Store
	geoProvider spatial.GeometryProvider
}

func NewNarrativeOrchestrator(bus *eventbus.EventBus) *NarrativeOrchestrator {
	debugLog("", "", "Initializing Narrative Orchestrator", map[string]interface{}{})

	semanticURL := os.Getenv("SEMANTIC_MEMORY_URL")
	if semanticURL == "" {
		semanticURL = "http://semantic-memory:8080"
	}

	minioCfg := minio.Config{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"),
		UseSSL:          false,
		Region:          "us-east-1",
	}

	minioClient, err := minio.NewMinIOOfficialClient(minioCfg)
	if err != nil {
		errorLog("", "", "Failed to initialize MinIO client", map[string]interface{}{
			"error": err.Error(),
		})
		// Continue with nil client to maintain backward compatibility
		minioClient = nil
	}

	geoProvider := spatial.NewSemanticMemoryProvider(semanticURL)
	configStore := config.NewStore(minioClient, "gnue-configs")

	infoLog("", "", "Successfully initialized Narrative Orchestrator", map[string]interface{}{
		"semantic_url": semanticURL,
		"minio_ready":  minioClient != nil,
	})

	return &NarrativeOrchestrator{
		gms:         make(map[string]*GMInstance),
		bus:         bus,
		semantic:    &SemanticMemoryClient{BaseURL: semanticURL},
		minioClient: minioClient,
		configStore: configStore,
		geoProvider: geoProvider,
	}
}

func extractIDFromScope(scopeID string) string {
	debugLog("", "", "Extracting ID from scope", map[string]interface{}{
		"original_scope_id": scopeID,
	})

	if parts := strings.SplitN(scopeID, ":", 2); len(parts) == 2 {
		extractedID := parts[1]
		debugLog("", "", "Successfully extracted ID from scope", map[string]interface{}{
			"original_scope_id": scopeID,
			"extracted_id":      extractedID,
		})
		return extractedID
	}

	debugLog("", "", "No colon separator found in scope ID, returning original", map[string]interface{}{
		"scope_id": scopeID,
	})
	return scopeID
}

func getDefaultProfile() *config.Profile {
	debugLog("", "", "Getting default profile", map[string]interface{}{})

	profile := &config.Profile{
		TimeWindow: "10m",
		Triggers: struct {
			TimeIntervalMs    int      `yaml:"time_interval_ms,omitempty" json:"time_interval_ms,omitempty"`
			MaxEvents         int      `yaml:"max_events,omitempty" json:"max_events,omitempty"`
			NarrativeTriggers []string `yaml:"narrative_triggers,omitempty" json:"narrative_triggers,omitempty"`
		}{
			TimeIntervalMs: 10000,
			MaxEvents:      50,
		},
	}

	debugLog("", "", "Default profile created", map[string]interface{}{
		"time_window":      profile.TimeWindow,
		"time_interval_ms": profile.Triggers.TimeIntervalMs,
		"max_events":       profile.Triggers.MaxEvents,
	})

	return profile
}

func (no *NarrativeOrchestrator) CreateGM(ev eventbus.Event) {
	scopeID, _ := ev.Payload["scope_id"].(string)
	scopeType, _ := ev.Payload["scope_type"].(string)
	if scopeType == "" {
		if parts := strings.SplitN(scopeID, ":", 2); len(parts) == 2 {
			scopeType = parts[0]
		} else {
			scopeType = "unknown"
		}
	}

	// Check if GM already exists for this scopeID
	no.mu.RLock()
	if _, exists := no.gms[scopeID]; exists {
		infoLog(scopeID, ev.WorldID, "GM already exists, returning existing instance", map[string]interface{}{
			"scope_type": scopeType,
		})
		no.mu.RUnlock()
		return
	}
	no.mu.RUnlock()

	infoLog(scopeID, ev.WorldID, "Creating GM instance", map[string]interface{}{
		"scope_type": scopeType,
	})

	profile, err := no.configStore.GetProfile(scopeType)
	if err != nil {
		warnLog(scopeID, ev.WorldID, "Failed to get profile for scope type", map[string]interface{}{
			"scope_type": scopeType,
			"error":      err.Error(),
		})
	}

	if profile == nil {
		infoLog(scopeID, ev.WorldID, "Using default profile for scope type", map[string]interface{}{
			"scope_type": scopeType,
		})
		profile = getDefaultProfile()
	}

	override, err := no.configStore.GetOverride(scopeID)
	if err != nil {
		warnLog(scopeID, ev.WorldID, "Failed to get override for scope", map[string]interface{}{
			"scope_id": scopeID,
			"error":    err.Error(),
		})
	}

	if override != nil {
		infoLog(scopeID, ev.WorldID, "Merging profile with override", map[string]interface{}{})
		profile = config.MergeProfiles(profile, override)
	}

	focusEntities := []string{scopeID}
	for _, tpl := range profile.FocusEntities {
		id := strings.ReplaceAll(tpl, "{{.player_id}}", extractIDFromScope(scopeID))
		id = strings.ReplaceAll(id, "{{.group_id}}", extractIDFromScope(scopeID))
		focusEntities = append(focusEntities, id)
	}

	gm := &GMInstance{
		ScopeID:         scopeID,
		ScopeType:       scopeType,
		WorldID:         ev.WorldID,
		FocusEntities:   focusEntities,
		State:           make(map[string]interface{}),
		History:         []HistoryEntry{},
		Config:          profile.ToMap(),
		LastProcessTime: 0,
		CreatedAt:       time.Now(),
	}

	geometry, err := no.geoProvider.GetGeometry(context.Background(), ev.WorldID, scopeID)
	if err != nil {
		warnLog(scopeID, ev.WorldID, "Failed to get geometry for scope", map[string]interface{}{
			"error": err.Error(),
		})
	}
	if geometry == nil {
		geometry = &spatial.Geometry{Point: &spatial.Point{X: 0, Y: 0}}
	}
	gm.VisibilityScope = spatial.DefaultScope(scopeType, geometry, gm.Config)
	gm.UpdateVisibilityScope(no.geoProvider)

	if savedGM, err := no.loadSnapshot(scopeID); err == nil && savedGM != nil {
		infoLog(scopeID, ev.WorldID, "Rehydrating GM from snapshot", map[string]interface{}{})
		gm = savedGM
		gm.ScopeID = scopeID
		gm.ScopeType = scopeType
		gm.WorldID = ev.WorldID
		gm.FocusEntities = focusEntities
		gm.Config = profile.ToMap()
		gm.UpdateVisibilityScope(no.geoProvider)
	} else if err != nil {
		warnLog(scopeID, ev.WorldID, "Failed to load GM snapshot", map[string]interface{}{
			"error": err.Error(),
		})
		if strings.Contains(err.Error(), "no snapshots") {
			err := no.saveSnapshot(scopeID, gm)
			if err != nil {
				warnLog(scopeID, ev.WorldID, "Failed to save GM snapshot", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}

	}

	timeoutMin := 30.0
	if cfgTriggers, ok := gm.Config["triggers"].(map[string]interface{}); ok {
		if intervalMs, ok := cfgTriggers["time_interval_ms"].(float64); ok {
			timeoutMin = intervalMs / 60000.0 * 5
		}
	}
	timeoutMin = 30.0

	infoLog(scopeID, ev.WorldID, "Setting up GM cleanup timer", map[string]interface{}{
		"timeout_minutes": timeoutMin,
	})

	time.AfterFunc(time.Duration(timeoutMin)*time.Minute, func() {
		infoLog(scopeID, ev.WorldID, "GM cleanup timer triggered", map[string]interface{}{})
		no.DeleteGMByScope(scopeID)
	})

	no.mu.Lock()
	no.gms[scopeID] = gm
	no.mu.Unlock()

	infoLog(scopeID, ev.WorldID, "GM created successfully", map[string]interface{}{
		"focus_entities_count": len(gm.FocusEntities),
		"config_keys_count":    len(gm.Config),
	})
}

func (no *NarrativeOrchestrator) DeleteGMByScope(scopeID string) {
	infoLog(scopeID, "", "Deleting GM by scope", map[string]interface{}{})

	no.mu.Lock()
	defer no.mu.Unlock()
	delete(no.gms, scopeID)

	infoLog(scopeID, "", "GM deleted successfully", map[string]interface{}{})
}

func (no *NarrativeOrchestrator) DeleteGM(ev eventbus.Event) {
	scopeID, _ := ev.Payload["scope_id"].(string)
	infoLog(scopeID, ev.WorldID, "Processing GM deletion event", map[string]interface{}{})

	no.DeleteGMByScope(scopeID)

	infoLog(scopeID, ev.WorldID, "GM deletion event processed", map[string]interface{}{})
}

func (no *NarrativeOrchestrator) MergeGM(ev eventbus.Event) {
	scopeID := ""
	if sid, ok := ev.Payload["scope_id"].(string); ok {
		scopeID = sid
	}
	worldID := ev.WorldID

	infoLog(scopeID, worldID, "Processing GM merge event", map[string]interface{}{
		"payload_keys": len(ev.Payload),
	})
}

func (no *NarrativeOrchestrator) SplitGM(ev eventbus.Event) {
	scopeID := ""
	if sid, ok := ev.Payload["scope_id"].(string); ok {
		scopeID = sid
	}
	worldID := ev.WorldID

	infoLog(scopeID, worldID, "Processing GM split event", map[string]interface{}{
		"payload_keys": len(ev.Payload),
	})
}

func (no *NarrativeOrchestrator) HandleTimerEvent(ev eventbus.Event) {
	infoLog("", ev.WorldID, "Processing timer event", map[string]interface{}{
		"event_id": ev.EventID,
	})

	currentTimeMsRaw, ok := ev.Payload["current_time_unix_ms"]
	if !ok {
		warnLog("", ev.WorldID, "Timer event missing current_time_unix_ms", map[string]interface{}{
			"event_id": ev.EventID,
		})
		return
	}
	currentTimeMs := int64(currentTimeMsRaw.(float64))

	no.mu.RLock()
	gms := make([]*GMInstance, 0, len(no.gms))
	for _, gm := range no.gms {
		gms = append(gms, gm)
	}
	no.mu.RUnlock()

	infoLog("", ev.WorldID, "Processing timer event for GMs", map[string]interface{}{
		"gms_count":       len(gms),
		"current_time_ms": currentTimeMs,
	})

	for _, gm := range gms {
		intervalMs := int64(10000)
		if triggers, ok := gm.Config["triggers"].(map[string]interface{}); ok {
			if ms, ok := triggers["time_interval_ms"].(float64); ok {
				intervalMs = int64(ms)
			}
		}

		nextProcessTime := gm.LastProcessTime + intervalMs
		if gm.LastProcessTime == 0 || currentTimeMs >= nextProcessTime {
			infoLog(gm.ScopeID, gm.WorldID, "Scheduling batch processing for GM", map[string]interface{}{
				"interval_ms":       intervalMs,
				"next_process_time": nextProcessTime,
				"current_time_ms":   currentTimeMs,
			})

			no.mu.Lock()
			gm.LastProcessTime = currentTimeMs
			no.mu.Unlock()
			go no.processBatchForGM(gm)
		}
	}

	infoLog("", ev.WorldID, "Timer event processing completed", map[string]interface{}{
		"event_id": ev.EventID,
	})
}

func (no *NarrativeOrchestrator) processEntityUpdate(gm *GMInstance, changes []map[string]interface{}) {
	infoLog(gm.ScopeID, gm.WorldID, "Starting entity update processing", map[string]interface{}{
		"changes_count": len(changes),
	})

	no.mu.Lock()
	defer no.mu.Unlock()

	if gm.Config == nil {
		gm.Config = make(map[string]interface{})
		infoLog(gm.ScopeID, gm.WorldID, "Initialized empty config for GM", map[string]interface{}{})
	}

	updatedEntities := make([]string, 0)
	for _, change := range changes {
		entityID, _ := change["entity_id"].(string)
		if entityID == "" {
			continue
		}

		prefix := "entity_" + strings.Replace(entityID, ":", "_", -1) + "_"

		if opsRaw, ok := change["operations"].([]interface{}); ok {
			for _, opRaw := range opsRaw {
				if op, ok := opRaw.(map[string]interface{}); ok {
					opType, _ := op["op"].(string)
					path, _ := op["path"].(string)

					switch opType {
					case "set":
						if value, exists := op["value"]; exists {
							gm.Config[prefix+path] = value
							infoLog(gm.ScopeID, gm.WorldID, "Set config value for entity", map[string]interface{}{
								"entity_id": entityID,
								"key":       prefix + path,
							})
						}
					case "add_to_slice":
						if value, exists := op["value"].(string); exists {
							key := prefix + path
							slice, ok := gm.Config[key].([]interface{})
							if !ok {
								slice = []interface{}{}
							}
							gm.Config[key] = append(slice, value)
							infoLog(gm.ScopeID, gm.WorldID, "Added value to slice for entity", map[string]interface{}{
								"entity_id": entityID,
								"key":       key,
								"value":     value,
							})
						}
					case "remove_from_slice":
						if value, exists := op["value"].(string); exists {
							key := prefix + path
							if slice, ok := gm.Config[key].([]interface{}); ok {
								for i, v := range slice {
									if v == value {
										gm.Config[key] = append(slice[:i], slice[i+1:]...)
										infoLog(gm.ScopeID, gm.WorldID, "Removed value from slice for entity", map[string]interface{}{
											"entity_id": entityID,
											"key":       key,
											"value":     value,
										})
										break
									}
								}
							}
						}
					case "remove":
						delete(gm.Config, prefix+path)
						infoLog(gm.ScopeID, gm.WorldID, "Removed config key for entity", map[string]interface{}{
							"entity_id": entityID,
							"key":       prefix + path,
						})
					}
				}
			}
		}

		updatedEntities = append(updatedEntities, entityID)
	}

	gm.UpdateVisibilityScope(no.geoProvider)

	infoLog(gm.ScopeID, gm.WorldID, "Completed entity update processing", map[string]interface{}{
		"updated_entities":  updatedEntities,
		"total_config_keys": len(gm.Config),
	})
}

func (no *NarrativeOrchestrator) loadSnapshot(scopeID string) (*GMInstance, error) {
	debugLog(scopeID, "", "Loading GM snapshot", map[string]interface{}{
		"scope_id": scopeID,
	})

	if no.minioClient == nil {
		warnLog(scopeID, "", "MinIO client not available, cannot load snapshot", map[string]interface{}{})
		return nil, fmt.Errorf("minio client not available")
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(scopeID)))
	prefix := path.Join("gnue", "gm-snapshots", "v1", hash)

	objects, err := no.minioClient.ListObjects("gnue-snapshots", prefix)
	if err != nil {
		errorLog(scopeID, "", "Failed to list snapshot objects from MinIO", map[string]interface{}{
			"error":  err.Error(),
			"prefix": prefix,
		})
		return nil, err
	}

	if len(objects) == 0 {
		infoLog(scopeID, "", "No snapshots found for GM", map[string]interface{}{
			"prefix": prefix,
		})
		return nil, fmt.Errorf("no snapshots")
	}

	objectKey := objects[0].Key
	data, err := no.minioClient.GetObject("gnue-snapshots", objectKey)
	if err != nil {
		errorLog(scopeID, "", "Failed to get snapshot object from MinIO", map[string]interface{}{
			"error":      err.Error(),
			"object_key": objectKey,
		})
		return nil, err
	}

	var gm GMInstance
	if err := json.Unmarshal(data, &gm); err != nil {
		errorLog(scopeID, "", "Failed to unmarshal GM snapshot", map[string]interface{}{
			"error":      err.Error(),
			"object_key": objectKey,
		})
		return nil, err
	}

	infoLog(scopeID, "", "Successfully loaded GM snapshot", map[string]interface{}{
		"object_key":    objectKey,
		"snapshot_size": len(data),
	})

	return &gm, nil
}

func (no *NarrativeOrchestrator) saveSnapshot(scopeID string, gm *GMInstance) error {
	debugLog(scopeID, gm.WorldID, "Saving GM snapshot", map[string]interface{}{
		"scope_id": scopeID,
	})

	if no.minioClient == nil {
		warnLog(scopeID, gm.WorldID, "MinIO client not available, cannot save snapshot", map[string]interface{}{})
		return fmt.Errorf("minio client not available")
	}

	data, err := json.Marshal(gm)
	if err != nil {
		errorLog(scopeID, gm.WorldID, "Failed to marshal GM for snapshot", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(scopeID)))
	timestamp := time.Now().Unix()
	key := path.Join("gnue", "gm-snapshots", "v1", hash, fmt.Sprintf("%d_001.json", timestamp))

	err = no.minioClient.PutObject("gnue-snapshots", key, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		errorLog(scopeID, gm.WorldID, "Failed to save GM snapshot to MinIO", map[string]interface{}{
			"error":      err.Error(),
			"object_key": key,
		})
		return err
	}

	infoLog(scopeID, gm.WorldID, "Successfully saved GM snapshot", map[string]interface{}{
		"object_key":    key,
		"snapshot_size": len(data),
	})

	return nil
}

func (no *NarrativeOrchestrator) HandleGameEvent(ev eventbus.Event) {
	localScopeID := ""
	if ev.ScopeID != nil {
		localScopeID = *ev.ScopeID
	}

	infoLog(localScopeID, ev.WorldID, "Processing game event", map[string]interface{}{
		"event_type": ev.EventType,
		"event_id":   ev.EventID,
	})

	// 1. Обработка state_changes (приоритетно)
	if changesRaw, exists := ev.Payload["state_changes"]; exists {
		if changes, ok := changesRaw.([]interface{}); ok {
			var changesMap []map[string]interface{}
			for _, c := range changes {
				if cm, ok := c.(map[string]interface{}); ok {
					changesMap = append(changesMap, cm)
				}
			}

			infoLog(localScopeID, ev.WorldID, "Processing state changes", map[string]interface{}{
				"changes_count": len(changesMap),
			})

			no.mu.RLock()
			for _, gm := range no.gms {
				for _, change := range changesMap {
					if entityID, ok := change["entity_id"].(string); ok {
						for _, focusID := range gm.FocusEntities {
							if focusID == entityID {
								infoLog(gm.ScopeID, gm.WorldID, "Scheduling entity update for GM", map[string]interface{}{
									"entity_id": entityID,
								})
								go no.processEntityUpdate(gm, changesMap)
								break
							}
						}
					}
				}
			}
			no.mu.RUnlock()
			return
		}
	}

	// 2. Для обычных событий — находим GM по scope_id
	if ev.ScopeID == nil {
		warnLog("", ev.WorldID, "Game event missing scope ID", map[string]interface{}{
			"event_type": ev.EventType,
			"event_id":   ev.EventID,
		})
		return
	}
	scopeID := localScopeID

	no.mu.RLock()
	gm, exists := no.gms[scopeID]
	no.mu.RUnlock()

	if !exists {
		warnLog(scopeID, ev.WorldID, "No GM found for scope ID", map[string]interface{}{
			"scope_id": scopeID,
		})
		return
	}

	// 3. Мгновенная реакция: проверяем, есть ли ev.EventType в narrative_triggers GM
	if triggersRaw, ok := gm.Config["triggers"].(map[string]interface{}); ok {
		if triggersList, ok := triggersRaw["narrative_triggers"].([]interface{}); ok {
			infoLog(gm.ScopeID, gm.WorldID, "Checking narrative triggers", map[string]interface{}{
				"triggers_count": len(triggersList),
			})

			for _, t := range triggersList {
				if tStr, ok := t.(string); ok && tStr == ev.EventType {
					infoLog(gm.ScopeID, gm.WorldID, "Event matches narrative trigger, scheduling processing", map[string]interface{}{
						"event_type": ev.EventType,
					})
					go no.processEventForGM(ev, gm)
					return
				}
			}
		}
	}

	// 4. Буферизация
	no.mu.Lock()
	gm.History = append(gm.History, HistoryEntry{
		EventID:   ev.EventID,
		Timestamp: ev.Timestamp,
	})

	// Проверка переполнения
	maxSize := 100
	if m, ok := gm.Config["buffer.max_size"].(float64); ok {
		maxSize = int(m)
	}
	if len(gm.History) > maxSize {
		dropLow := true
		if d, ok := gm.Config["buffer.drop_low_priority"].(bool); ok {
			dropLow = d
		}
		if dropLow {
			oldSize := len(gm.History)
			gm.History = gm.History[len(gm.History)-maxSize:]
			infoLog(gm.ScopeID, gm.WorldID, "History buffer trimmed", map[string]interface{}{
				"old_size": oldSize,
				"new_size": len(gm.History),
			})
		}
	}
	no.mu.Unlock()

	// Проверка порога по объёму
	maxEvents := 50
	if triggers, ok := gm.Config["triggers"].(map[string]interface{}); ok {
		if me, ok := triggers["max_events"].(float64); ok {
			maxEvents = int(me)
		}
	}

	infoLog(gm.ScopeID, gm.WorldID, "Event added to history buffer", map[string]interface{}{
		"history_size":         len(gm.History),
		"max_events_threshold": maxEvents,
	})

	if len(gm.History) >= maxEvents {
		infoLog(gm.ScopeID, gm.WorldID, "History threshold reached, scheduling batch processing", map[string]interface{}{
			"history_size": len(gm.History),
			"threshold":    maxEvents,
		})
		go no.processBatchForGM(gm)
	}

	infoLog(gm.ScopeID, gm.WorldID, "Game event processing completed", map[string]interface{}{
		"event_type": ev.EventType,
		"event_id":   ev.EventID,
	})
}

func (no *NarrativeOrchestrator) processBatchForGM(gm *GMInstance) {
	batchEventID := "batch-" + time.Now().Format("20060102-150405")
	infoLog(gm.ScopeID, gm.WorldID, "Starting batch processing", map[string]interface{}{
		"batch_event_id": batchEventID,
		"history_size":   len(gm.History),
	})

	dummyEvent := eventbus.Event{
		EventID:   batchEventID,
		EventType: "batch.process",
		Source:    "narrative-orchestrator",
		WorldID:   gm.WorldID,
		ScopeID:   &gm.ScopeID,
		Timestamp: time.Now(),
	}

	no.processEventForGM(dummyEvent, gm)

	infoLog(gm.ScopeID, gm.WorldID, "Batch processing completed", map[string]interface{}{
		"batch_event_id": batchEventID,
	})
}

// processEventForGM — основной метод обработки.
func (no *NarrativeOrchestrator) processEventForGM(ev eventbus.Event, gm *GMInstance) {
	infoLog(gm.ScopeID, gm.WorldID, "Starting event processing", map[string]interface{}{
		"event_type": ev.EventType,
		"event_id":   ev.EventID,
		"is_batch":   ev.EventType == "batch.process",
	})

	if ev.EventType != "batch.process" && ev.EventType != "time.syncTime" {
		no.mu.Lock()
		gm.History = append(gm.History, HistoryEntry{
			EventID:   ev.EventID,
			Timestamp: ev.Timestamp,
		})
		no.mu.Unlock()
	}

	// Определяем события для обработки
	var eventsToProcess []HistoryEntry
	if ev.EventType == "batch.process" || ev.EventType == "time.syncTime" {
		no.mu.RLock()
		eventsToProcess = make([]HistoryEntry, len(gm.History))
		copy(eventsToProcess, gm.History)
		no.mu.RUnlock()

		infoLog(gm.ScopeID, gm.WorldID, "Processing batch of events", map[string]interface{}{
			"events_count": len(eventsToProcess),
		})
	} else {
		eventsToProcess = []HistoryEntry{{
			EventID:   ev.EventID,
			Timestamp: ev.Timestamp,
		}}

		infoLog(gm.ScopeID, gm.WorldID, "Processing single event", map[string]interface{}{
			"event_id": ev.EventID,
		})
	}

	// Получаем контекст с событиями из Semantic Memory
	// 🔑 ДОБАВИТЬ: защита от пустых сущностей
	entityIDs := append([]string{}, gm.FocusEntities...)
	if len(entityIDs) == 0 {
		warnLog(gm.ScopeID, gm.WorldID, "No entity IDs for GM, skipping context retrieval", map[string]interface{}{
			"focus_entities_count": len(gm.FocusEntities),
		})
		return
	}

	// 🔑 ДОБАВИТЬ: защита от нулевого клиента
	if no.semantic == nil {
		warnLog(gm.ScopeID, gm.WorldID, "SemanticMemoryClient is nil, cannot retrieve context", map[string]interface{}{})
		return
	}

	eventTypes := []string{} // или ["player.*", "combat.*"] из конфига
	infoLog(gm.ScopeID, gm.WorldID, "Retrieving context with events from semantic memory", map[string]interface{}{
		"entity_ids_count":  len(entityIDs),
		"event_types_count": len(eventTypes),
		"depth":             2,
	})

	contexts, err := no.semantic.GetContextWithEvents(context.Background(), entityIDs, eventTypes, 2)
	if err != nil {
		warnLog(gm.ScopeID, gm.WorldID, "Failed to get context with events, continuing without context", map[string]interface{}{
			"error": err.Error(),
		})
		// Продолжаем выполнение без контекста
	}

	// 🔑 ИЗМЕНИТЬ: защита при извлечении данных
	worldContext := "Нет данных о мире"
	entitiesContext := "Нет данных"

	// Защита для мира
	// БЫЛО:
	// if wc, ok := contexts[gm.WorldID].(map[string]interface{})["context"].(string); ok {
	// СТАЛО:
	// ✅ Добавляем проверку на nil contexts
	if contexts != nil {
		if worldRaw, ok := contexts[gm.WorldID]; ok {
			if worldMap, ok := worldRaw.(map[string]interface{}); ok {
				if wc, ok := worldMap["context"].(string); ok {
					worldContext = wc
				}
			}
		}
	}

	// Защита для сущностей
	entitiesLines := []string{}
	if contexts != nil {
		for _, id := range gm.FocusEntities {
			if ctxRaw, ok := contexts[id]; ok {
				if ctxMap, ok := ctxRaw.(map[string]interface{}); ok {
					if ctxStr, ok := ctxMap["context"].(string); ok {
						entitiesLines = append(entitiesLines, ctxStr)
					}
				}
			}
		}
	}
	if len(entitiesLines) > 0 {
		entitiesContext = strings.Join(entitiesLines, "\n")
	}

	// Кластеризация событий
	clusters := clusterEvents(eventsToProcess)

	// Временной контекст
	var lastEventTime *time.Time
	if len(gm.History) > 0 {
		t := gm.History[len(gm.History)-1].Timestamp
		lastEventTime = &t
	}
	lastMood := []string{}
	if mood, ok := gm.State["last_mood"].([]string); ok {
		lastMood = mood
	}
	timeContext := BuildTimeContext(lastEventTime, lastMood)

	// Триггер
	triggerEvent := "Прошло время. Мир продолжает жить."
	if len(eventsToProcess) > 0 {
		triggerEvent = "Накопленные события за период"
	}

	// Формируем промт
	input := PromptInput{
		WorldContext:    worldContext,
		ScopeID:         gm.ScopeID,
		ScopeType:       gm.ScopeType,
		EntitiesContext: entitiesContext,
		EventClusters:   clusters,
		TimeContext:     timeContext,
		TriggerEvent:    triggerEvent,
	}
	systemPrompt, userPrompt := BuildPrompt(input)

	// Вызов Oracle
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	infoLog(gm.ScopeID, gm.WorldID, "Calling Oracle for narrative generation", map[string]interface{}{
		"system_prompt_length": len(systemPrompt),
		"user_prompt_length":   len(userPrompt),
	})

	oracleResp, err := CallOracle(ctx, systemPrompt, userPrompt)
	if err != nil {
		errorLog(gm.ScopeID, gm.WorldID, "Oracle call failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	infoLog(gm.ScopeID, gm.WorldID, "Oracle response received", map[string]interface{}{
		"new_events_count": len(oracleResp.NewEvents),
		"has_narrative":    oracleResp.Narrative != "",
		"mood_length":      len(oracleResp.Mood),
	})

	// Публикация
	if len(oracleResp.Mood) > 0 {
		no.mu.Lock()
		if gm.State == nil {
			gm.State = make(map[string]interface{})
		}
		gm.State["last_mood"] = oracleResp.Mood
		no.mu.Unlock()

		infoLog(gm.ScopeID, gm.WorldID, "Updated GM mood state", map[string]interface{}{
			"mood_length": len(oracleResp.Mood),
		})
	}

	for i, evMap := range oracleResp.NewEvents {
		eventType, _ := evMap["event_type"].(string)
		payload, _ := evMap["payload"].(map[string]interface{})

		outputEvent := eventbus.Event{
			EventID:   "evt-" + uuid.New().String()[:8],
			EventType: eventType,
			Source:    "narrative-orchestrator",
			WorldID:   gm.WorldID,
			ScopeID:   nil, // Will be set conditionally below
			Payload:   payload,
			Timestamp: time.Now(),
		}

		// Handle the scope_id from evMap which is of type interface{}
		if scopeIDVal, ok := evMap["scope_id"]; ok && scopeIDVal != nil {
			if scopeIDStr, ok := scopeIDVal.(string); ok {
				outputEvent.ScopeID = &scopeIDStr
			}
		} //else {
		// 	// Fallback to gm.ScopeID if no scope_id in evMap
		// 	outputEvent.ScopeID = &gm.ScopeID
		// }

		err := no.bus.Publish(context.Background(), eventbus.TopicWorldEvents, outputEvent)
		if err != nil {
			errorLog(gm.ScopeID, gm.WorldID, "Failed to publish generated event", map[string]interface{}{
				"error":       err.Error(),
				"event_type":  eventType,
				"event_index": i,
			})
		} else {
			infoLog(gm.ScopeID, gm.WorldID, "Published generated event", map[string]interface{}{
				"event_type":  eventType,
				"event_id":    outputEvent.EventID,
				"event_index": i,
			})
		}
	}

	if oracleResp.Narrative != "" {
		narrativePayload := map[string]interface{}{}
		narrativePayload["narrative"] = oracleResp.Narrative
		outputEvent := eventbus.Event{
			EventID:   "evt-" + uuid.New().String()[:8],
			EventType: "narrative.generate",
			Source:    "narrative-orchestrator",
			WorldID:   gm.WorldID,
			ScopeID:   nil,
			Payload:   narrativePayload,
			Timestamp: time.Now(),
		}

		err := no.bus.Publish(context.Background(), eventbus.TopicNarrativeOutput, outputEvent)
		if err != nil {
			errorLog(gm.ScopeID, gm.WorldID, "Failed to publish narrative event", map[string]interface{}{
				"error":    err.Error(),
				"event_id": outputEvent.EventID,
			})
		} else {
			infoLog(gm.ScopeID, gm.WorldID, "Published narrative event", map[string]interface{}{
				"event_id":         outputEvent.EventID,
				"narrative_length": len(oracleResp.Narrative),
			})
		}
	}

	saveErr := no.saveSnapshot(gm.ScopeID, gm)
	if saveErr != nil {
		errorLog(gm.ScopeID, gm.WorldID, "Failed to save GM snapshot after processing", map[string]interface{}{
			"error": saveErr.Error(),
		})
	} else {
		infoLog(gm.ScopeID, gm.WorldID, "GM snapshot saved after processing", map[string]interface{}{
			"scope_id": gm.ScopeID,
		})
	}

	infoLog(gm.ScopeID, gm.WorldID, "Event processing completed", map[string]interface{}{
		"event_type": ev.EventType,
		"event_id":   ev.EventID,
	})
}

// clusterEvents — группировка по времени.
func clusterEvents(events []HistoryEntry) []EventCluster {
	debugLog("", "", "Starting event clustering", map[string]interface{}{
		"events_count": len(events),
	})

	if len(events) == 0 {
		debugLog("", "", "No events to cluster", map[string]interface{}{})
		return nil
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	var clusters []EventCluster
	currentEvents := []string{}

	addCluster := func(first, last time.Time, events []string) {
		duration := last.Sub(first).Milliseconds()
		clusters = append(clusters, EventCluster{
			RelativeTime: humanizeDuration(duration),
			Description:  strings.Join(events, "; "),
		})
	}

	if len(events) == 0 {
		debugLog("", "", "No events to cluster after sorting", map[string]interface{}{})
		return clusters
	}

	first := events[0].Timestamp
	currentEvents = append(currentEvents, fmt.Sprintf("Событие: %s", events[0].EventID))

	for i := 1; i < len(events); i++ {
		prev := events[i-1].Timestamp
		curr := events[i].Timestamp
		gap := curr.Sub(prev).Milliseconds()

		if gap > 50 {
			addCluster(first, prev, currentEvents)
			first = curr
			currentEvents = []string{}
		}
		currentEvents = append(currentEvents, fmt.Sprintf("Событие: %s", events[i].EventID))
	}

	addCluster(first, events[len(events)-1].Timestamp, currentEvents)

	infoLog("", "", "Event clustering completed", map[string]interface{}{
		"clusters_count": len(clusters),
		"events_count":   len(events),
	})

	return clusters
}

func (no *NarrativeOrchestrator) extractEventPoint(ev eventbus.Event) (spatial.Point, bool) {
	scopeID := ""
	if ev.ScopeID != nil {
		scopeID = *ev.ScopeID
	}

	debugLog(scopeID, ev.WorldID, "Extracting event point", map[string]interface{}{
		"event_type":   ev.EventType,
		"payload_keys": len(ev.Payload),
	})

	if loc, ok := ev.Payload["location"].(map[string]interface{}); ok {
		if x, ok := loc["x"].(float64); ok {
			if y, ok := loc["y"].(float64); ok {
				point := spatial.Point{X: x, Y: y}
				infoLog(scopeID, ev.WorldID, "Extracted point from location", map[string]interface{}{
					"x": x,
					"y": y,
				})
				return point, true
			}
		}
	}
	if to, ok := ev.Payload["to"].(map[string]interface{}); ok {
		if x, ok := to["x"].(float64); ok {
			if y, ok := to["y"].(float64); ok {
				point := spatial.Point{X: x, Y: y}
				infoLog(scopeID, ev.WorldID, "Extracted point from destination", map[string]interface{}{
					"x": x,
					"y": y,
				})
				return point, true
			}
		}
	}

	infoLog(scopeID, ev.WorldID, "Could not extract point from event", map[string]interface{}{
		"event_type": ev.EventType,
	})

	return spatial.Point{}, false
}

func extractEntityIDs(payload map[string]interface{}) []string {
	debugLog("", "", "Extracting entity IDs from payload", map[string]interface{}{
		"payload_keys_count": len(payload),
	})

	var ids []string
	if mentions, ok := payload["mentions"].([]interface{}); ok {
		for i, m := range mentions {
			if id, ok := m.(string); ok {
				ids = append(ids, id)
				debugLog("", "", "Found entity ID in mentions", map[string]interface{}{
					"index":     i,
					"entity_id": id,
				})
			}
		}
	}
	if entityID, ok := payload["entity_id"].(string); ok {
		ids = append(ids, entityID)
		debugLog("", "", "Found entity ID in payload", map[string]interface{}{
			"entity_id": entityID,
		})
	}
	if target, ok := payload["target"].(string); ok {
		ids = append(ids, target)
		debugLog("", "", "Found target entity ID in payload", map[string]interface{}{
			"target": target,
		})
	}
	if source, ok := payload["source"].(string); ok {
		ids = append(ids, source)
		debugLog("", "", "Found source entity ID in payload", map[string]interface{}{
			"source": source,
		})
	}

	infoLog("", "", "Entity ID extraction completed", map[string]interface{}{
		"extracted_ids_count": len(ids),
		"extracted_ids":       ids,
	})

	return ids
}

func toStringSlice(v []interface{}) []string {
	debugLog("", "", "Converting interface slice to string slice", map[string]interface{}{
		"input_length": len(v),
	})

	res := make([]string, len(v))
	count := 0
	for i, val := range v {
		if s, ok := val.(string); ok {
			res[i] = s
			count++
		}
	}

	infoLog("", "", "Interface slice conversion completed", map[string]interface{}{
		"input_length":    len(v),
		"output_length":   len(res),
		"converted_count": count,
	})

	return res
}
