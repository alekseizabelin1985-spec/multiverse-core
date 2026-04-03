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

	"multiverse-core.io/shared/config"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/minio"
	"multiverse-core.io/shared/spatial"

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
	logger  *log.Logger
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
	// Validate inputs
	if c == nil {
		return nil, fmt.Errorf("semantic memory client is nil")
	}

	if len(entityIDs) == 0 {
		return nil, fmt.Errorf("entity IDs cannot be empty")
	}

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
		return nil, fmt.Errorf("failed to marshal context request: %w", err)
	}

	resp, err := http.Post(c.BaseURL+"/v1/context-with-events", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		errorLog("", "", "Failed to call semantic memory service", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to call semantic memory service: %w", err)
	}
	defer resp.Body.Close()

	var result ContextWithEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		errorLog("", "", "Failed to decode semantic memory response", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode semantic memory response: %w", err)
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
	logger      *log.Logger
}

func NewNarrativeOrchestrator(bus *eventbus.EventBus) *NarrativeOrchestrator {
	logger := log.New(log.Writer(), "NarrativeOrchestrator: ", log.LstdFlags|log.Lshortfile)

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
		semantic:    &SemanticMemoryClient{BaseURL: semanticURL, logger: logger},
		minioClient: minioClient,
		configStore: configStore,
		geoProvider: geoProvider,
		logger:      logger,
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
	scopeID, ok := ev.Payload["scope_id"].(string)
	if !ok || scopeID == "" {
		errorLog("", ev.WorldID, "Invalid or missing scope_id in event payload", map[string]interface{}{
			"event_id": ev.EventID,
			"payload":  ev.Payload,
		})
		return
	}

	scopeType, ok := ev.Payload["scope_type"].(string)
	if !ok {
		scopeType = ""
	}
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

	ttlMinutes := 30.0
	if cfgTriggers, ok := gm.Config["triggers"].(map[string]interface{}); ok {
		if intervalMs, ok := cfgTriggers["time_interval_ms"].(float64); ok {
			// TTL = 5x the processing interval, minimum 10 minutes
			computed := intervalMs / 60000.0 * 5
			if computed > 10 {
				ttlMinutes = computed
			}
		}
	}

	ttlDur := time.Duration(ttlMinutes) * time.Minute
	infoLog(scopeID, ev.WorldID, "Setting up GM with activity-based TTL", map[string]interface{}{
		"ttl_minutes": ttlMinutes,
	})

	gm.initConcurrency(ttlDur, func() {
		infoLog(scopeID, ev.WorldID, "GM inactivity TTL expired", map[string]interface{}{})
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
	gm, exists := no.gms[scopeID]
	if exists {
		gm.stopTTL()
		delete(no.gms, scopeID)
	}
	no.mu.Unlock()

	if exists {
		infoLog(scopeID, "", "GM deleted successfully", map[string]interface{}{})
	} else {
		warnLog(scopeID, "", "GM not found for deletion", map[string]interface{}{})
	}
}

func (no *NarrativeOrchestrator) DeleteGM(ev eventbus.Event) {
	scopeID, _ := ev.Payload["scope_id"].(string)
	infoLog(scopeID, ev.WorldID, "Processing GM deletion event", map[string]interface{}{})

	no.DeleteGMByScope(scopeID)

	infoLog(scopeID, ev.WorldID, "GM deletion event processed", map[string]interface{}{})
}

func (no *NarrativeOrchestrator) MergeGM(ev eventbus.Event) {
	targetScopeID, _ := ev.Payload["scope_id"].(string)
	worldID := ev.WorldID

	// source_scope_ids — список GM, которые вливаются в target
	sourceScopeIDsRaw, _ := ev.Payload["source_scope_ids"].([]interface{})
	if targetScopeID == "" || len(sourceScopeIDsRaw) == 0 {
		warnLog(targetScopeID, worldID, "MergeGM: missing scope_id or source_scope_ids", map[string]interface{}{})
		return
	}

	infoLog(targetScopeID, worldID, "Processing GM merge event", map[string]interface{}{
		"source_count": len(sourceScopeIDsRaw),
	})

	no.mu.Lock()
	targetGM, exists := no.gms[targetScopeID]
	if !exists {
		no.mu.Unlock()
		warnLog(targetScopeID, worldID, "MergeGM: target GM not found", map[string]interface{}{})
		return
	}

	for _, raw := range sourceScopeIDsRaw {
		srcID, ok := raw.(string)
		if !ok || srcID == "" {
			continue
		}
		srcGM, srcExists := no.gms[srcID]
		if !srcExists {
			continue
		}

		// Merge focus entities (deduplicate)
		seen := make(map[string]bool, len(targetGM.FocusEntities))
		for _, id := range targetGM.FocusEntities {
			seen[id] = true
		}
		for _, id := range srcGM.FocusEntities {
			if !seen[id] {
				targetGM.FocusEntities = append(targetGM.FocusEntities, id)
			}
		}

		// Merge history
		targetGM.History = append(targetGM.History, srcGM.History...)

		// Merge canon from state
		if srcCanon, ok := srcGM.State["canon"].([]interface{}); ok {
			existingCanon, _ := targetGM.State["canon"].([]interface{})
			targetGM.State["canon"] = append(existingCanon, srcCanon...)
		}

		srcGM.stopTTL()
		delete(no.gms, srcID)

		infoLog(targetScopeID, worldID, "Merged source GM into target", map[string]interface{}{
			"source_scope_id": srcID,
		})
	}
	no.mu.Unlock()

	// Update visibility after merge (HTTP call outside lock)
	targetGM.mu.Lock()
	targetGM.UpdateVisibilityScope(no.geoProvider)
	targetGM.mu.Unlock()

	infoLog(targetScopeID, worldID, "GM merge completed", map[string]interface{}{
		"focus_entities_count": len(targetGM.FocusEntities),
	})
}

func (no *NarrativeOrchestrator) SplitGM(ev eventbus.Event) {
	sourceScopeID, _ := ev.Payload["scope_id"].(string)
	worldID := ev.WorldID

	// new_scopes — список новых скоупов для создания
	newScopesRaw, _ := ev.Payload["new_scopes"].([]interface{})
	if sourceScopeID == "" || len(newScopesRaw) == 0 {
		warnLog(sourceScopeID, worldID, "SplitGM: missing scope_id or new_scopes", map[string]interface{}{})
		return
	}

	infoLog(sourceScopeID, worldID, "Processing GM split event", map[string]interface{}{
		"new_scopes_count": len(newScopesRaw),
	})

	// Create each new scope as a separate gm.created event
	for _, raw := range newScopesRaw {
		scopeDef, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		newScopeID, _ := scopeDef["scope_id"].(string)
		if newScopeID == "" {
			continue
		}

		// Entities to move to the new scope
		focusRaw, _ := scopeDef["focus_entities"].([]interface{})

		createEv := eventbus.Event{
			EventID:   "split-" + time.Now().Format("20060102-150405") + "-" + newScopeID,
			EventType: "gm.created",
			Source:    "narrative-orchestrator",
			WorldID:   worldID,
			Payload: map[string]interface{}{
				"scope_id":       newScopeID,
				"scope_type":     scopeDef["scope_type"],
				"focus_entities": focusRaw,
			},
			Timestamp: time.Now(),
		}

		no.CreateGM(createEv)

		// Remove moved entities from source GM
		if len(focusRaw) > 0 {
			no.mu.RLock()
			srcGM, exists := no.gms[sourceScopeID]
			no.mu.RUnlock()
			if exists {
				removeSet := make(map[string]bool)
				for _, f := range focusRaw {
					if s, ok := f.(string); ok {
						removeSet[s] = true
					}
				}
				srcGM.mu.Lock()
				filtered := srcGM.FocusEntities[:0]
				for _, id := range srcGM.FocusEntities {
					if !removeSet[id] {
						filtered = append(filtered, id)
					}
				}
				srcGM.FocusEntities = filtered
				srcGM.mu.Unlock()
			}
		}

		infoLog(sourceScopeID, worldID, "Created split GM", map[string]interface{}{
			"new_scope_id": newScopeID,
		})
	}

	infoLog(sourceScopeID, worldID, "GM split completed", map[string]interface{}{})
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
		gm.mu.Lock()
		if triggers, ok := gm.Config["triggers"].(map[string]interface{}); ok {
			if ms, ok := triggers["time_interval_ms"].(float64); ok {
				intervalMs = int64(ms)
			}
		}

		nextProcessTime := gm.LastProcessTime + intervalMs
		shouldProcess := gm.LastProcessTime == 0 || currentTimeMs >= nextProcessTime
		if shouldProcess {
			gm.LastProcessTime = currentTimeMs
		}
		gm.mu.Unlock()

		if shouldProcess {
			infoLog(gm.ScopeID, gm.WorldID, "Scheduling batch processing for GM", map[string]interface{}{
				"interval_ms":       intervalMs,
				"next_process_time": nextProcessTime,
				"current_time_ms":   currentTimeMs,
			})
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

	// Use per-GM lock instead of global lock to avoid blocking other GMs
	gm.mu.Lock()

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
					opPath, _ := op["path"].(string)

					switch opType {
					case "set":
						if value, exists := op["value"]; exists {
							gm.Config[prefix+opPath] = value
						}
					case "add_to_slice":
						if value, exists := op["value"].(string); exists {
							key := prefix + opPath
							slice, ok := gm.Config[key].([]interface{})
							if !ok {
								slice = []interface{}{}
							}
							gm.Config[key] = append(slice, value)
						}
					case "remove_from_slice":
						if value, exists := op["value"].(string); exists {
							key := prefix + opPath
							if slice, ok := gm.Config[key].([]interface{}); ok {
								for i, v := range slice {
									if v == value {
										gm.Config[key] = append(slice[:i], slice[i+1:]...)
										break
									}
								}
							}
						}
					case "remove":
						delete(gm.Config, prefix+opPath)
					}
				}
			}
		}

		updatedEntities = append(updatedEntities, entityID)
	}

	// Release per-GM lock BEFORE making HTTP call to geoProvider
	gm.mu.Unlock()

	// HTTP call to update visibility — outside any lock
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

// findGMsForEvent returns all GMs that should receive this event:
// 1. Exact scope_id match
// 2. Spatial match — event point is within GM's VisibilityScope
func (no *NarrativeOrchestrator) findGMsForEvent(ev eventbus.Event) []*GMInstance {
	no.mu.RLock()
	defer no.mu.RUnlock()

	var result []*GMInstance
	matched := make(map[string]bool)

	// 1. Exact scope_id match
	if ev.ScopeID != nil {
		if gm, exists := no.gms[*ev.ScopeID]; exists {
			result = append(result, gm)
			matched[gm.ScopeID] = true
		}
	}

	// 2. Spatial routing — check if event point falls within any GM's visibility
	eventPoint, hasPoint := no.extractEventPoint(ev)
	if hasPoint {
		for _, gm := range no.gms {
			if matched[gm.ScopeID] {
				continue
			}
			if gm.VisibilityScope.IsInScope(eventPoint) {
				result = append(result, gm)
				matched[gm.ScopeID] = true
			}
		}
	}

	// 3. Entity mention match — check if any entity mentioned in event is a focus entity
	mentionedIDs := extractEntityIDs(ev.Payload)
	if len(mentionedIDs) > 0 {
		for _, gm := range no.gms {
			if matched[gm.ScopeID] {
				continue
			}
			for _, mentioned := range mentionedIDs {
				found := false
				for _, focus := range gm.FocusEntities {
					if focus == mentioned {
						result = append(result, gm)
						matched[gm.ScopeID] = true
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}

	return result
}

func (no *NarrativeOrchestrator) HandleGameEvent(ev eventbus.Event) {
	localScopeID := ""
	if ev.ScopeID != nil {
		localScopeID = *ev.ScopeID
	}

	debugLog(localScopeID, ev.WorldID, "Processing game event", map[string]interface{}{
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

			no.mu.RLock()
			for _, gm := range no.gms {
				for _, change := range changesMap {
					if entityID, ok := change["entity_id"].(string); ok {
						for _, focusID := range gm.FocusEntities {
							if focusID == entityID {
								gm.resetTTL()
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

	// 2. Find all GMs that should receive this event (spatial + exact match)
	targetGMs := no.findGMsForEvent(ev)
	if len(targetGMs) == 0 {
		debugLog(localScopeID, ev.WorldID, "No GMs matched for event", map[string]interface{}{
			"event_type": ev.EventType,
		})
		return
	}

	// 3. Dispatch event to each matched GM
	for _, gm := range targetGMs {
		gm.resetTTL()
		no.dispatchEventToGM(ev, gm)
	}
}

// dispatchEventToGM handles trigger check, buffering, and threshold for a single GM.
func (no *NarrativeOrchestrator) dispatchEventToGM(ev eventbus.Event, gm *GMInstance) {
	// Каскад-защита: ГМ не реагирует на события, которые сам опубликовал
	gm.mu.Lock()
	if gm.isOwnEvent(ev.EventID) {
		gm.mu.Unlock()
		debugLog(gm.ScopeID, gm.WorldID, "Skipping own emitted event", map[string]interface{}{
			"event_id": ev.EventID,
		})
		return
	}
	triggers := gm.Config["triggers"]
	gm.mu.Unlock()

	if triggersRaw, ok := triggers.(map[string]interface{}); ok {
		if triggersList, ok := triggersRaw["narrative_triggers"].([]interface{}); ok {
			for _, t := range triggersList {
				if tStr, ok := t.(string); ok && tStr == ev.EventType {
					infoLog(gm.ScopeID, gm.WorldID, "Event matches narrative trigger", map[string]interface{}{
						"event_type": ev.EventType,
					})
					go no.processEventForGM(ev, gm)
					return
				}
			}
		}
	}

	// Буферизация — per-GM lock
	gm.mu.Lock()

	// Extract description from payload
	description := ""
	if desc, ok := ev.Payload["description"].(string); ok {
		description = desc
	} else if detail, ok := ev.Payload["detail"].(string); ok {
		description = detail
	}

	gm.History = append(gm.History, HistoryEntry{
		EventID:     ev.EventID,
		EventType:   ev.EventType,
		Description: description,
		Timestamp:   ev.Timestamp,
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
			gm.History = gm.History[len(gm.History)-maxSize:]
		}
	}

	// Проверка порога по объёму
	maxEvents := 50
	if tr, ok := gm.Config["triggers"].(map[string]interface{}); ok {
		if me, ok := tr["max_events"].(float64); ok {
			maxEvents = int(me)
		}
	}
	historySize := len(gm.History)
	gm.mu.Unlock()

	if historySize >= maxEvents {
		infoLog(gm.ScopeID, gm.WorldID, "History threshold reached, scheduling batch", map[string]interface{}{
			"history_size": historySize,
			"threshold":    maxEvents,
		})
		go no.processBatchForGM(gm)
	}
}

func (no *NarrativeOrchestrator) processBatchForGM(gm *GMInstance) {
	batchEventID := "batch-" + time.Now().Format("20060102-150405")

	dummyEvent := eventbus.Event{
		EventID:   batchEventID,
		EventType: "batch.process",
		Source:    "narrative-orchestrator",
		WorldID:   gm.WorldID,
		ScopeID:   &gm.ScopeID,
		Timestamp: time.Now(),
	}

	no.processEventForGM(dummyEvent, gm)
}

// NEW: Handle mechanical results from Entity-Actors
func (no *NarrativeOrchestrator) HandleMechanicalResult(ev eventbus.Event) {
	debugLog("", ev.WorldID, "Processing mechanical result event", map[string]interface{}{
		"event_id": ev.EventID,
	})

	// Extract mechanical result from payload with proper validation
	mechanicalResult, exists := ev.Payload["mechanical_result"]
	if !exists {
		warnLog("", ev.WorldID, "No mechanical_result in event payload", map[string]interface{}{
			"event_id": ev.EventID,
		})
		return
	}

	// Validate that mechanical result is a map
	mechanicalResultMap, ok := mechanicalResult.(map[string]interface{})
	if !ok {
		errorLog("", ev.WorldID, "Invalid mechanical result structure", map[string]interface{}{
			"event_id": ev.EventID,
			"type":     fmt.Sprintf("%T", mechanicalResult),
		})
		return
	}

	// Validate required fields in mechanical result
	entityID, entityIDExists := mechanicalResultMap["entity_id"].(string)
	if !entityIDExists || entityID == "" {
		warnLog("", ev.WorldID, "Missing or invalid entity_id in mechanical result", map[string]interface{}{
			"event_id": ev.EventID,
			"result":   mechanicalResultMap,
		})
		return
	}

	mood, moodExists := mechanicalResultMap["mood"].(string)
	if !moodExists {
		warnLog("", ev.WorldID, "Missing or invalid mood in mechanical result", map[string]interface{}{
			"event_id": ev.EventID,
			"result":   mechanicalResultMap,
		})
		// Continue processing without mood
		mood = ""
	}

	// Get GM for this scope with proper validation
	var gm *GMInstance
	if ev.ScopeID == nil {
		warnLog("", ev.WorldID, "Event missing scope ID", map[string]interface{}{
			"event_id": ev.EventID,
		})
		return
	}

	no.mu.RLock()
	gm, exists = no.gms[*ev.ScopeID]
	no.mu.RUnlock()
	if !exists {
		warnLog("", ev.WorldID, "No GM found for mechanical result", map[string]interface{}{
			"scope_id": *ev.ScopeID,
		})
		return
	}

	// Build narrative from mechanical result with proper error handling
	narrative := no.buildNarrativeFromMechanics(mechanicalResultMap, gm)
	// if err != nil { // Removed since buildNarrativeFromMechanics doesn't return an error
	// 	errorLog(gm.ScopeID, gm.WorldID, "Failed to build narrative from mechanics", map[string]interface{}{
	// 		"error": err.Error(),
	// 	})
	// 	return
	// }

	// Publish narrative output
	narrativeEvent := eventbus.Event{
		EventID:   "narrative-" + time.Now().Format("20060102-150405"),
		EventType: "narrative.generate",
		Source:    "narrative-orchestrator",
		WorldID:   ev.WorldID,
		ScopeID:   ev.ScopeID,
		Payload: map[string]interface{}{
			"narrative": narrative,
			"mood":      mood,
			"entity_id": entityID,
		},
		Timestamp: time.Now(),
	}

	err := no.bus.Publish(context.Background(), eventbus.TopicNarrativeOutput, narrativeEvent)
	if err != nil {
		errorLog("", ev.WorldID, "Failed to publish narrative output", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	infoLog(gm.ScopeID, gm.WorldID, "Published mechanical result narrative", map[string]interface{}{
		"event_id": narrativeEvent.EventID,
	})
}

// buildNarrativeFromMechanics converts mechanical results to narrative
func (no *NarrativeOrchestrator) buildNarrativeFromMechanics(result interface{}, gm *GMInstance) string {
	// Convert result to map for processing
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return "Mechanical result could not be processed"
	}

	// Use semantic layer from rule engine if available
	ruleID, exists := resultMap["rule_id"].(string)
	if exists && ruleID != "" {
		// In a real implementation, we would get the rule and use its semantic layer
		// For now, return basic narrative
		return fmt.Sprintf("Mechanical result processed with rule: %s", ruleID)
	}

	// Fallback to basic narrative
	return "Mechanical result processed successfully"
}

// processEventForGM — основной метод обработки.
// Uses per-GM processing guard to prevent concurrent Oracle calls.
func (no *NarrativeOrchestrator) processEventForGM(ev eventbus.Event, gm *GMInstance) {
	// Per-GM processing guard: only one goroutine processes at a time
	if !gm.tryStartProcessing() {
		debugLog(gm.ScopeID, gm.WorldID, "GM already processing, skipping", map[string]interface{}{
			"event_type": ev.EventType,
		})
		return
	}
	defer gm.doneProcessing()

	infoLog(gm.ScopeID, gm.WorldID, "Starting event processing", map[string]interface{}{
		"event_type": ev.EventType,
		"event_id":   ev.EventID,
		"is_batch":   ev.EventType == "batch.process",
	})

	if ev.EventType != "batch.process" && ev.EventType != "time.syncTime" {
		gm.mu.Lock()
		gm.History = append(gm.History, HistoryEntry{
			EventID:   ev.EventID,
			Timestamp: ev.Timestamp,
		})
		gm.mu.Unlock()
	}

	// Copy data under per-GM lock
	gm.mu.Lock()
	focusEntities := make([]string, len(gm.FocusEntities))
	copy(focusEntities, gm.FocusEntities)
	historyCopy := make([]HistoryEntry, len(gm.History))
	copy(historyCopy, gm.History)
	gm.mu.Unlock()

	if no.semantic == nil {
		warnLog(gm.ScopeID, gm.WorldID, "SemanticMemoryClient is nil", map[string]interface{}{})
		return
	}

	// All network calls happen outside any lock
	eventTypes := []string{}
	// Include world entity ID in context to ensure world data is loaded
	entityIDs := append([]string{gm.WorldID}, focusEntities...)
	contexts, err := no.semantic.GetContextWithEvents(context.Background(), entityIDs, eventTypes, 2)
	if err != nil {
		warnLog(gm.ScopeID, gm.WorldID, "Failed to get context with events, continuing without", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Определяем начало окна: от LastProcessTime (скользящее окно), а не фиксированный час
	gm.mu.Lock()
	var sinceTime time.Time
	if gm.LastProcessTime > 0 {
		sinceTime = time.UnixMilli(gm.LastProcessTime)
	} else {
		sinceTime = time.Now().Add(-10 * time.Minute) // fallback для первого запуска
	}
	gm.mu.Unlock()

	var fullEvents []eventbus.Event
	if len(historyCopy) > 0 {
		fullEvents, err = no.semantic.GetEventsForEntities(entityIDs, gm.WorldID, sinceTime, 50)
		if err != nil {
			warnLog(gm.ScopeID, gm.WorldID, "Failed to get full events, using history fallback", map[string]interface{}{
				"error": err.Error(),
			})
			fullEvents = make([]eventbus.Event, len(historyCopy))
			for i, he := range historyCopy {
				fullEvents[i] = eventbus.Event{
					EventID:     he.EventID,
					EventType:   "history.fallback",
					Timestamp:   he.Timestamp,
					WorldID:     gm.WorldID,
					ScopeID:     &gm.ScopeID,
					Payload: map[string]interface{}{
						"description": he.Description,
					},
				}
			}
		}

		// Фильтруем события которые этот конкретный ГМ сам опубликовал
		// (события других ГМ с Source "narrative-orchestrator" — легитимный вход)
		gm.mu.Lock()
		filtered := fullEvents[:0]
		for _, fe := range fullEvents {
			if !gm.isOwnEvent(fe.EventID) {
				filtered = append(filtered, fe)
			}
		}
		gm.mu.Unlock()
		fullEvents = filtered
	} else {
		fullEvents = []eventbus.Event{ev}
	}

	infoLog(gm.ScopeID, gm.WorldID, "Retrieved full events for clustering", map[string]interface{}{
		"full_events_count": len(fullEvents),
	})

	// Кластеризация событий
	clusters := clusterEvents(fullEvents)

	// Триггер — описание инициирующего события
	var triggerEvent string
	switch {
	case ev.EventType == "batch.process":
		// Батч — триггером является последнее реальное событие
		if len(fullEvents) > 0 {
			lastEv := fullEvents[len(fullEvents)-1]
			triggerEvent = formatEventDescription(lastEv)
		} else {
			triggerEvent = "Прошло время. Мир продолжает жить."
		}
	case ev.EventType == "time.syncTime":
		if len(fullEvents) > 0 {
			lastEv := fullEvents[len(fullEvents)-1]
			triggerEvent = formatEventDescription(lastEv)
		} else {
			triggerEvent = "Прошло время. Мир продолжает жить."
		}
	default:
		// Прямой триггер — описание самого события
		triggerEvent = formatEventDescription(ev)
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

	// Read state under per-GM lock
	gm.mu.Lock()
	var lastEventTime *time.Time
	if len(gm.History) > 0 {
		t := gm.History[len(gm.History)-1].Timestamp
		lastEventTime = &t
	}
	lastMood := []string{}
	if mood, ok := gm.State["last_mood"].([]string); ok {
		lastMood = mood
	}
	var canon []string
	if raw, ok := gm.State["canon"].([]interface{}); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				canon = append(canon, s)
			}
		}
	}
	gm.mu.Unlock()

	timeContext := BuildTimeContext(lastEventTime, lastMood)

	// Формируем промт
	sections := PromptSections{
		WorldFacts:     worldContext,
		EntityStates:   entitiesContext,
		Canon:          canon,
		ScopeID:        gm.ScopeID,
		ScopeType:      gm.ScopeType,
		WorldID:        gm.WorldID,
		TimeContext:    timeContext,
		EventClusters:  clusters,
		TriggerEvent:   triggerEvent,
		LastMood:       lastMood,
		MaxEvents:      4,
		DefaultSource:  "narrative-orchestrator",
		DefaultWorldID: gm.WorldID,
	}

	// Вызов Oracle
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	infoLog(gm.ScopeID, gm.WorldID, "Calling Oracle (structured) for narrative generation", map[string]interface{}{
		"scope_id": gm.ScopeID,
		"world_id": gm.WorldID,
	})

	oracleResp, err := CallOracleStructured(ctx, sections)
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

	// Update mood under per-GM lock
	if len(oracleResp.Mood) > 0 {
		gm.mu.Lock()
		if gm.State == nil {
			gm.State = make(map[string]interface{})
		}
		gm.State["last_mood"] = oracleResp.Mood
		gm.mu.Unlock()
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

		// Запоминаем event_id чтобы не реагировать на своё же событие
		gm.mu.Lock()
		gm.trackEmitted(outputEvent.EventID)
		gm.mu.Unlock()

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

	// Clear history and evict old emitted IDs after successful Oracle processing
	gm.mu.Lock()
	clearedCount := len(gm.History)
	gm.History = nil
	gm.evictExpiredEmitted()
	gm.mu.Unlock()

	infoLog(gm.ScopeID, gm.WorldID, "Cleared history buffer after Oracle processing", map[string]interface{}{
		"cleared_events_count": clearedCount,
	})

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

// clusterEvents — группировка по времени для полных событий eventbus.Event.
func clusterEvents(events []eventbus.Event) []EventCluster {
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
	currentEvents := []eventbus.Event{}

	addCluster := func(first, last time.Time, events []eventbus.Event) {
		duration := last.Sub(first).Milliseconds()
		eventDetails := make([]EventDetail, 0, len(events))
		for _, ev := range events {
			eventDetails = append(eventDetails, EventDetail{
				EventID:   ev.EventID,
				EventType: ev.EventType,
				Timestamp: ev.Timestamp,
				Source:    ev.Source,
				WorldID:   ev.WorldID,
				ScopeID: func() string {
					if ev.ScopeID != nil {
						return *ev.ScopeID
					}
					return ""
				}(),
				Payload:     ev.Payload,
				Description: formatEventDescription(ev),
			})
		}
		clusters = append(clusters, EventCluster{
			RelativeTime: humanizeDuration(duration),
			Events:       eventDetails,
		})
	}

	if len(events) == 0 {
		debugLog("", "", "No events to cluster after sorting", map[string]interface{}{})
		return clusters
	}

	first := events[0].Timestamp
	currentEvents = append(currentEvents, events[0])

	for i := 1; i < len(events); i++ {
		prev := events[i-1].Timestamp
		curr := events[i].Timestamp
		gap := curr.Sub(prev).Milliseconds()

		if gap > 50 {
			addCluster(first, prev, currentEvents)
			first = curr
			currentEvents = []eventbus.Event{}
		}
		currentEvents = append(currentEvents, events[i])
	}

	addCluster(first, events[len(events)-1].Timestamp, currentEvents)

	infoLog("", "", "Event clustering completed", map[string]interface{}{
		"clusters_count": len(clusters),
		"events_count":   len(events),
	})

	return clusters
}

// capitalizeFirst делает первую букву заглавной
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

// formatEventDescription формирует человеко-читаемое описание события.
func formatEventDescription(ev eventbus.Event) string {
	// Сначала проверяем наличие explicit описания
	if desc, ok := ev.Payload["description"].(string); ok && desc != "" {
		return desc
	}
	if desc, ok := ev.Payload["detail"].(string); ok && desc != "" {
		return desc
	}

	// Извлекаем ключевые поля из payload для формирования описания
	var parts []string

	// Источник (если есть) — используем ExtractEntityID для поддержки нового формата
	if entity := eventbus.ExtractEntityID(ev.Payload); entity != nil && entity.ID != "" {
		parts = append(parts, entity.ID)
	}

	// Действие (на основе типа события или explicit action)
	action := ev.EventType
	if action == "" {
		if ev.EventID != "" {
			return fmt.Sprintf("Событие %s", ev.EventID[:min(8, len(ev.EventID))])
		}
		action = "неизвестное событие"
	}
	if explicitAction, ok := ev.Payload["action"].(string); ok && explicitAction != "" {
		action = explicitAction
	}
	if explicitType, ok := ev.Payload["type"].(string); ok && explicitType != "" {
		action = explicitType
	}
	// Преобразуем snake_case/camelCase в readable текст
	action = strings.ReplaceAll(action, ".", " ")
	action = strings.ReplaceAll(action, "_", " ")
	action = capitalizeFirst(action) // Первая буква заглавная
	parts = append(parts, action)

	// Цель (если есть)
	if targetID, ok := ev.Payload["target_id"].(string); ok && targetID != "" {
		parts = append(parts, fmt.Sprintf("к %s", targetID))
	}
	if to, ok := ev.Payload["to"].(map[string]any); ok {
		if toID, ok := to["id"].(string); ok && toID != "" {
			parts = append(parts, fmt.Sprintf("к %s", toID))
		}
	}

	result := strings.Join(parts, " ")
	if result == "" {
		result = fmt.Sprintf("Событие %s", ev.EventID[:8])
	}
	return result
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
	if to, ok := ev.Payload["to"].(map[string]any); ok {
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
	if entity := eventbus.ExtractEntityID(payload); entity != nil && entity.ID != "" {
		ids = append(ids, entity.ID)
		debugLog("", "", "Found entity ID in payload", map[string]interface{}{
			"entity_id": entity.ID,
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
