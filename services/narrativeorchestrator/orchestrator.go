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
	reqBody, _ := json.Marshal(ContextWithEventsRequest{
		EntityIDs:  entityIDs,
		EventTypes: eventTypes,
		Depth:      depth,
	})

	resp, err := http.Post(c.BaseURL+"/v1/context-with-events", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ContextWithEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
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
	minioClient *minio.Client
	configStore *config.Store
	geoProvider spatial.GeometryProvider
}

func NewNarrativeOrchestrator(bus *eventbus.EventBus) *NarrativeOrchestrator {
	semanticURL := os.Getenv("SEMANTIC_MEMORY_URL")
	if semanticURL == "" {
		semanticURL = "http://semantic-memory:8080"
	}

	minioCfg := minio.Config{
		Endpoint:        "http://"+os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"),
		UseSSL:          false,
		Region:          "us-east-1",
	}
	minioClient, _ := minio.NewClient(minioCfg)
	geoProvider := spatial.NewSemanticMemoryProvider(semanticURL)
	configStore := config.NewStore(minioClient, "gnue-configs")

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
	if parts := strings.SplitN(scopeID, ":", 2); len(parts) == 2 {
		return parts[1]
	}
	return scopeID
}

func getDefaultProfile() *config.Profile {
	return &config.Profile{
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

	profile, _ := no.configStore.GetProfile(scopeType)
	log.Println(profile)
	if profile == nil {
		profile = getDefaultProfile()
	}
	log.Println(profile)
	override, _ := no.configStore.GetOverride(scopeID)
	
	if override != nil {
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

	geometry, _ := no.geoProvider.GetGeometry(context.Background(), ev.WorldID, scopeID)
	if geometry == nil {
		geometry = &spatial.Geometry{Point: &spatial.Point{X: 0, Y: 0}}
	}
	gm.VisibilityScope = spatial.DefaultScope(scopeType, geometry, gm.Config)
	gm.UpdateVisibilityScope(no.geoProvider)

	if savedGM, _ := no.loadSnapshot(scopeID); savedGM != nil {
		gm = savedGM
		gm.ScopeID = scopeID
		gm.ScopeType = scopeType
		gm.WorldID = ev.WorldID
		gm.FocusEntities = focusEntities
		gm.Config = profile.ToMap()
		gm.UpdateVisibilityScope(no.geoProvider)
		log.Printf("GM rehydrated: %s", scopeID)
	}

	timeoutMin := 30.0
	if cfgTriggers, ok := gm.Config["triggers"].(map[string]interface{}); ok {
		if intervalMs, ok := cfgTriggers["time_interval_ms"].(float64); ok {
			timeoutMin = intervalMs / 60000.0 * 5
		}
	}
	timeoutMin = 30.0
	time.AfterFunc(time.Duration(timeoutMin)*time.Minute, func() {
		no.DeleteGMByScope(scopeID)
	})

	no.mu.Lock()
	no.gms[scopeID] = gm
	no.mu.Unlock()

	log.Println(gm.Config)

	log.Printf("GM created: %s (type: %s)", scopeID, scopeType)
}

func (no *NarrativeOrchestrator) DeleteGMByScope(scopeID string) {
	no.mu.Lock()
	defer no.mu.Unlock()
	delete(no.gms, scopeID)
}

func (no *NarrativeOrchestrator) DeleteGM(ev eventbus.Event) {
	scopeID, _ := ev.Payload["scope_id"].(string)
	no.DeleteGMByScope(scopeID)
	log.Printf("GM deleted: %s", scopeID)
}

func (no *NarrativeOrchestrator) MergeGM(ev eventbus.Event) {
	log.Printf("gm.merged: %v", ev.Payload)
}

func (no *NarrativeOrchestrator) SplitGM(ev eventbus.Event) {
	log.Printf("gm.split: %v", ev.Payload)
}

func (no *NarrativeOrchestrator) HandleTimerEvent(ev eventbus.Event) {
	currentTimeMsRaw, ok := ev.Payload["current_time_unix_ms"]
	if !ok {
		log.Printf("time.syncTime без current_time_unix_ms: %s", ev.EventID)
		return
	}
	currentTimeMs := int64(currentTimeMsRaw.(float64))

	no.mu.RLock()
	gms := make([]*GMInstance, 0, len(no.gms))
	for _, gm := range no.gms {
		gms = append(gms, gm)
	}
	no.mu.RUnlock()

	for _, gm := range gms {
		intervalMs := int64(10000)
		if triggers, ok := gm.Config["triggers"].(map[string]interface{}); ok {
			if ms, ok := triggers["time_interval_ms"].(float64); ok {
				intervalMs = int64(ms)
			}
		}

		nextProcessTime := gm.LastProcessTime + intervalMs
		if gm.LastProcessTime == 0 || currentTimeMs >= nextProcessTime {
			no.mu.Lock()
			gm.LastProcessTime = currentTimeMs
			no.mu.Unlock()
			go no.processBatchForGM(gm)
		}
	}
}

func (no *NarrativeOrchestrator) processEntityUpdate(gm *GMInstance, changes []map[string]interface{}) {
	no.mu.Lock()
	defer no.mu.Unlock()

	if gm.Config == nil {
		gm.Config = make(map[string]interface{})
	}

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
						}
					case "add_to_slice":
						if value, exists := op["value"].(string); exists {
							key := prefix + path
							slice, ok := gm.Config[key].([]interface{})
							if !ok {
								slice = []interface{}{}
							}
							gm.Config[key] = append(slice, value)
						}
					case "remove_from_slice":
						if value, exists := op["value"].(string); exists {
							key := prefix + path
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
						delete(gm.Config, prefix+path)
					}
				}
			}
		}
	}

	gm.UpdateVisibilityScope(no.geoProvider)
	log.Printf("GM %s updated from state_changes", gm.ScopeID)
}

func (no *NarrativeOrchestrator) loadSnapshot(scopeID string) (*GMInstance, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(scopeID)))
	prefix := path.Join("gnue", "gm-snapshots", "v1", hash)
	objects, _ := no.minioClient.ListObjects("gnue-snapshots", prefix)
	if len(objects) == 0 {
		return nil, fmt.Errorf("no snapshots")
	}
	data, _ := no.minioClient.GetObject("gnue-snapshots", objects[0].Key)
	var gm GMInstance
	json.Unmarshal(data, &gm)
	return &gm, nil
}

func (no *NarrativeOrchestrator) saveSnapshot(scopeID string, gm *GMInstance) error {
	data, _ := json.Marshal(gm)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(scopeID)))
	timestamp := time.Now().Unix()
	key := path.Join("gnue", "gm-snapshots", "v1", hash, fmt.Sprintf("%d_001.json", timestamp))
	return no.minioClient.PutObject("gnue-snapshots", key, bytes.NewReader(data), int64(len(data)))
}

func (no *NarrativeOrchestrator) HandleGameEvent(ev eventbus.Event) {
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
	fmt.Println(ev)
	// 2. Для обычных событий — находим GM по scope_id
	if ev.ScopeID == nil {
		println("no scope ID")
		return
	}
	scopeID := *ev.ScopeID
	fmt.Println(scopeID)

	no.mu.RLock()
	gm, exists := no.gms[scopeID]
	no.mu.RUnlock()

	fmt.Println(gm)

	if !exists {
		fmt.Println("no gm found")
		return
	}

	// 3. Мгновенная реакция: проверяем, есть ли ev.EventType в narrative_triggers GM
	if triggersRaw, ok := gm.Config["triggers"].(map[string]interface{}); ok {
		if triggersList, ok := triggersRaw["narrative_triggers"].([]interface{}); ok {
			fmt.Println("Trigers list")
			fmt.Println(triggersList)
			for _, t := range triggersList {
				fmt.Println(t.(string))
				if tStr, ok := t.(string); ok && tStr == ev.EventType {
					fmt.Println("process event")
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
			gm.History = gm.History[len(gm.History)-maxSize:]
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
	if len(gm.History) >= maxEvents {
		go no.processBatchForGM(gm)
	}
}

func (no *NarrativeOrchestrator) processBatchForGM(gm *GMInstance) {
	dummyEvent := eventbus.Event{
		EventID:   "batch-" + time.Now().Format("20060102-150405"),
		EventType: "batch.process",
		Source:    "narrative-orchestrator",
		WorldID:   gm.WorldID,
		ScopeID:   &gm.ScopeID,
		Timestamp: time.Now(),
	}
	no.processEventForGM(dummyEvent, gm)
}

// processEventForGM — основной метод обработки.
func (no *NarrativeOrchestrator) processEventForGM(ev eventbus.Event, gm *GMInstance) {
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
	} else {
		eventsToProcess = []HistoryEntry{{
			EventID:   ev.EventID,
			Timestamp: ev.Timestamp,
		}}
	}

	// Получаем контекст с событиями из Semantic Memory
	// 🔑 ДОБАВИТЬ: защита от пустых сущностей
	entityIDs := append([]string{}, gm.FocusEntities...)
	if len(entityIDs) == 0 {
		log.Printf("No entity IDs for GM %s, skipping", gm.ScopeID)
		return
	}

	// 🔑 ДОБАВИТЬ: защита от нулевого клиента
	if no.semantic == nil {
		log.Printf("SemanticMemoryClient is nil for GM %s", gm.ScopeID)
		return
	}

	

	eventTypes := []string{} // или ["player.*", "combat.*"] из конфига
	contexts, err := no.semantic.GetContextWithEvents(context.Background(), entityIDs, eventTypes, 2)
	if err != nil {
		log.Printf("Failed to get context with events: %v", err)
		//return
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
	log.Println("------------------")
	log.Println(systemPrompt)
	log.Println("------------------")
	log.Println(userPrompt)
	log.Println("------------------")
	oracleResp, err := CallOracle(ctx, systemPrompt, userPrompt)

	log.Println(oracleResp)
	// Публикация
	if len(oracleResp.Mood) > 0 {
		no.mu.Lock()
		if gm.State == nil {
			gm.State = make(map[string]interface{})
		}
		gm.State["last_mood"] = oracleResp.Mood
		no.mu.Unlock()
	}

	for _, evMap := range oracleResp.NewEvents {
		eventType, _ := evMap["event_type"].(string)
		payload, _ := evMap["payload"].(map[string]interface{})

		log.Println(evMap)
		log.Println("------------------")
		log.Println(payload)
		outputEvent := eventbus.Event{
			EventID:   "evt-" + uuid.New().String()[:8],
			EventType: eventType,
			Source:    "narrative-orchestrator",
			WorldID:   gm.WorldID,
			ScopeID:   &gm.ScopeID,
			Payload:   payload,
			Timestamp: time.Now(),
		}

		 error1 := no.bus.Publish(context.Background(), eventbus.TopicWorldEvents, outputEvent)
		if error1 != nil {
			log.Println(error1)

		}
	}
	if oracleResp.Narrative != "" {
		narative :=map[string]interface{}{}
		narative["narrative"] = oracleResp.Narrative
		outputEvent := eventbus.Event{
			EventID:   "evt-" + uuid.New().String()[:8],
			EventType: "narrative.generate",
			Source:    "narrative-orchestrator",
			WorldID:   gm.WorldID,
			ScopeID:   &gm.ScopeID,
			Payload:   narative,
			Timestamp: time.Now(),
		}

		 error1 := no.bus.Publish(context.Background(), eventbus.TopicNarrativeOutput, outputEvent)
		if error1 != nil {
			log.Println(error1)

		}
	}

		
	

	err1 := no.saveSnapshot(gm.ScopeID, gm)
	if err1 != nil {
		log.Println(err1)
	}
	log.Printf("GM %s generated narrative", gm.ScopeID)
}

// clusterEvents — группировка по времени.
func clusterEvents(events []HistoryEntry) []EventCluster {
	if len(events) == 0 {
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
	return clusters
}

func (no *NarrativeOrchestrator) extractEventPoint(ev eventbus.Event) (spatial.Point, bool) {
	if loc, ok := ev.Payload["location"].(map[string]interface{}); ok {
		x := loc["x"].(float64)
		y := loc["y"].(float64)
		return spatial.Point{X: x, Y: y}, true
	}
	if to, ok := ev.Payload["to"].(map[string]interface{}); ok {
		x := to["x"].(float64)
		y := to["y"].(float64)
		return spatial.Point{X: x, Y: y}, true
	}
	return spatial.Point{}, false
}

func extractEntityIDs(payload map[string]interface{}) []string {
	var ids []string
	if mentions, ok := payload["mentions"].([]interface{}); ok {
		for _, m := range mentions {
			if id, ok := m.(string); ok {
				ids = append(ids, id)
			}
		}
	}
	if entityID, ok := payload["entity_id"].(string); ok {
		ids = append(ids, entityID)
	}
	if target, ok := payload["target"].(string); ok {
		ids = append(ids, target)
	}
	if source, ok := payload["source"].(string); ok {
		ids = append(ids, source)
	}
	return ids
}

func toStringSlice(v []interface{}) []string {
	res := make([]string, len(v))
	for i, val := range v {
		if s, ok := val.(string); ok {
			res[i] = s
		}
	}
	return res
}
