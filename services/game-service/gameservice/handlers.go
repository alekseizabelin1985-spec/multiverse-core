package gameservice

import (
	"encoding/json"
	"fmt"
	"log"
	"multiverse-core.io/shared/eventbus"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// EntityStreamHandler обрабатывает поток обновлений сущностей
type EntityStreamHandler struct {
	entityCache *EntityCache
	broadcast   chan []byte
	minioClient *MinioClient
}

func NewEntityStreamHandler(entityCache *EntityCache, broadcast chan []byte, minioClient *MinioClient) *EntityStreamHandler {
	return &EntityStreamHandler{
		entityCache: entityCache,
		broadcast:   broadcast,
		minioClient: minioClient,
	}
}

func (h *EntityStreamHandler) HandleEntityEvent(event eventbus.Event) {
	// Обработка событий, связанных с сущностями
	log.Printf("Handling entity event: %s", event.EventType)

	// TODO: Реализовать логику обработки событий сущностей
	// Это может включать:
	// 1. Получение обновленной сущности из EntityManager (если необходимо)
	// 2. Обновление кэша сущностей
	// 3. Подготовку сообщения для отправки клиентам
	// 4. Отправку сообщения через broadcast канал

	// Пример отправки сообщения клиентам
	// message, _ := json.Marshal(map[string]interface{}{
	// 	"type":  "entity_update",
	// 	"event": event,
	// })
	message, _ := json.Marshal(event)
	h.broadcast <- message
}

// EventStreamHandler обрабатывает поток игровых событий
type EventStreamHandler struct {
	eventBuffer []eventbus.Event
	broadcast   chan []byte
}

func NewEventStreamHandler(broadcast chan []byte) *EventStreamHandler {
	return &EventStreamHandler{
		eventBuffer: make([]eventbus.Event, 0),
		broadcast:   broadcast,
	}
}

func (h *EventStreamHandler) HandleGameEvent(event eventbus.Event) {
	// Обработка игровых событий
	log.Printf("Handling game event: %s", event.EventType)

	// Добавляем событие в буфер
	h.eventBuffer = append(h.eventBuffer, event)

	// Ограничиваем размер буфера (например, 100 последних событий)
	if len(h.eventBuffer) > 100 {
		h.eventBuffer = h.eventBuffer[1:]
	}

	// Подготавливаем сообщение для отправки клиентам
	// message, _ := json.Marshal(map[string]interface{}{
	// 	"type":  "game_event",
	// 	"event": event,
	// })
	message, _ := json.Marshal(event)
	h.broadcast <- message
}

// REST Handlers
func (s *Service) GetEntityHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	entityID := vars["entity_id"]

	// Получаем worldID из query параметров или заголовков
	worldID := r.URL.Query().Get("world_id")
	if worldID == "" {
		worldID = r.Header.Get("X-World-ID")
	}

	// Если worldID не указан, возвращаем ошибку
	if worldID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("world_id is required"))
		return
	}

	// Пытаемся получить сущность из кэша
	if entity, found := s.entityCache.Get(entityID, worldID); found {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entity)
		return
	}

	// Если сущности нет в кэше и доступен MinIO клиент, загружаем из MinIO
	if s.minioClient != nil {
		entity, err := s.minioClient.LoadEntity(r.Context(), entityID, worldID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Failed to load entity: %v", err)))
			return
		}

		// Сохраняем в кэш
		s.entityCache.Set(entityID, worldID, entity)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entity)
		return
	}

	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Entity not found"))
}

func (s *Service) RegisterPlayerHandler(w http.ResponseWriter, r *http.Request) {
	// Парсим JSON тело запроса
	var req struct {
		PlayerID   string `json:"player_id"`
		PlayerName string `json:"player_name"`
		WorldID    string `json:"world_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	// Проверяем обязательные поля
	if req.PlayerID == "" || req.PlayerName == "" || req.WorldID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("player_id, player_name and world_id are required"))
		return
	}

	// Регистрируем игрока
	entity, err := s.RegisterPlayer(r.Context(), req.PlayerID, req.PlayerName, req.WorldID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to register player: %v", err)))
		return
	}

	// Возвращаем успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Player registered successfully",
		"player":  entity,
	})
}

func (s *Service) LoginPlayerHandler(w http.ResponseWriter, r *http.Request) {
	// Парсим JSON тело запроса
	var req struct {
		PlayerID string `json:"player_id"`
		WorldID  string `json:"world_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	// Проверяем обязательные поля
	if req.PlayerID == "" || req.WorldID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("player_id and world_id are required"))
		return
	}

	// Входим как игрок
	entity, err := s.LoginPlayer(r.Context(), req.PlayerID, req.WorldID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Failed to login player: %v", err)))
		return
	}

	// Возвращаем успешный ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Player logged in successfully",
		"player":  entity,
	})
}

func (s *Service) GetEntityHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	entityID := vars["entity_id"]

	// TODO: Реализовать получение истории сущности по ID
	// Это может включать запрос к EntityManager или другому сервису
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entity_id": entityID,
		"history":   []interface{}{}, // Заглушка
	})
}

func (s *Service) GetRecentEventsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать получение последних событий
	// Это может включать запрос к Event Bus или другому сервису
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": []interface{}{}, // Заглушка
	})
}

// RunTestHandler запускает полный тестовый сценарий для проверки narrative-orchestrator.
// Тестирует: создание GM, моментальные триггеры, пакетную обработку,
// spatial routing, state_changes, и TTL-сброс.
//
// Query params:
//
//	?scenario=all|instant|batch|spatial|state_changes|merge_split (default: all)
func (s *Service) RunTestHandler(w http.ResponseWriter, r *http.Request) {
	scenario := r.URL.Query().Get("scenario")
	if scenario == "" {
		scenario = "all"
	}

	ctx := r.Context()
	worldID := "pain-realm"
	scopePlayer := "player:kain-777"
	scopeLocation := "location:dark_alley"
	results := make([]map[string]interface{}, 0)

	publish := func(topic string, ev eventbus.Event) error {
		var err error
		switch topic {
		case "system":
			err = s.bus.PublishSystemEvent(ctx, ev)
		case "world":
			err = s.bus.PublishWorldEvent(ctx, ev)
		case "game":
			err = s.bus.PublishGameEvent(ctx, ev)
		}
		if err != nil {
			log.Printf("Failed to publish %s event %s: %v", topic, ev.EventType, err)
		} else {
			results = append(results, map[string]interface{}{
				"step":       len(results) + 1,
				"topic":      topic,
				"event_type": ev.EventType,
				"event_id":   ev.EventID,
			})
		}
		return err
	}

	scopePtr := func(s string) *string { return &s }

	// ===== STEP 1: Создание GM для player =====
	if scenario == "all" || scenario == "instant" || scenario == "batch" || scenario == "spatial" || scenario == "state_changes" {
		err := publish("system", eventbus.Event{
			EventID:   "evt-gm-create-" + uuid.NewString()[:8],
			EventType: "gm.created",
			Source:    "test-harness",
			WorldID:   worldID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"scope_id":   scopePlayer,
				"scope_type": "player",
				"config": map[string]interface{}{
					"perception": 0.8,
					"focus_entities": []string{
						scopePlayer,
					},
				},
			},
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Step 1 failed: %v", err), http.StatusInternalServerError)
			return
		}
		time.Sleep(2 * time.Second)
	}

	// ===== STEP 2: Моментальный триггер (player.used_skill) =====
	if scenario == "all" || scenario == "instant" {
		err := publish("world", eventbus.Event{
			EventID:   "evt-skill-" + uuid.NewString()[:8],
			EventType: "player.used_skill",
			Source:    "test-harness",
			WorldID:   worldID,
			ScopeID:   scopePtr(scopePlayer),
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"entity_id":  scopePlayer,
				"skill_id":   "sky_rend",
				"skill_name": "Разрыв небес",
				"target":     "npc:wolf-5",
				"location": map[string]interface{}{
					"x": 123.4, "y": 56.7,
					"location": scopeLocation,
				},
				"description": "Каин применил умение 'Разрыв небес' на белого волка.",
			},
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Step 2 failed: %v", err), http.StatusInternalServerError)
			return
		}
		time.Sleep(2 * time.Second)

		// Ещё один моментальный: player.died
		_ = publish("world", eventbus.Event{
			EventID:   "evt-died-" + uuid.NewString()[:8],
			EventType: "player.died",
			Source:    "test-harness",
			WorldID:   worldID,
			ScopeID:   scopePtr(scopePlayer),
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"entity_id":   scopePlayer,
				"killer":      "npc:wolf-5",
				"cause":       "damage",
				"description": "Белый волк нанёс смертельный удар Каину.",
			},
		})
		time.Sleep(2 * time.Second)

		// И ещё: player.got_item
		_ = publish("world", eventbus.Event{
			EventID:   "evt-item-" + uuid.NewString()[:8],
			EventType: "player.got_item",
			Source:    "test-harness",
			WorldID:   worldID,
			ScopeID:   scopePtr(scopePlayer),
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"entity_id":   scopePlayer,
				"item_id":     "item:wolf-fang",
				"item_name":   "Клык белого волка",
				"description": "После победы Каин подобрал клык поверженного волка.",
			},
		})
		time.Sleep(1 * time.Second)
	}

	// ===== STEP 3: Пакетная обработка — серия мелких событий без триггера =====
	if scenario == "all" || scenario == "batch" {
		batchEvents := []struct {
			eventType   string
			description string
		}{
			{"player.moved", "Каин пошёл на восток по тёмной аллее."},
			{"player.looked_around", "Каин осмотрелся, заметив странные тени."},
			{"npc.said", "Торговец крикнул: 'Сюда, путник!'"},
			{"player.moved", "Каин подошёл к торговцу."},
			{"ambient.sound", "Где-то в переулке послышался рык."},
			{"player.emote", "Каин нахмурился и положил руку на меч."},
			{"weather.changed", "Начался мелкий дождь."},
			{"npc.moved", "Бродячий кот прошмыгнул мимо."},
		}

		for i, be := range batchEvents {
			_ = publish("world", eventbus.Event{
				EventID:   fmt.Sprintf("evt-batch-%d-%s", i, uuid.NewString()[:8]),
				EventType: be.eventType,
				Source:    "test-harness",
				WorldID:   worldID,
				ScopeID:   scopePtr(scopePlayer),
				Timestamp: time.Now().UTC(),
				Payload: map[string]interface{}{
					"entity_id":   scopePlayer,
					"description": be.description,
					"location": map[string]interface{}{
						"x": 125.0 + float64(i), "y": 58.0,
					},
				},
			})
			// Небольшая задержка между событиями чтобы имитировать реальность
			time.Sleep(200 * time.Millisecond)
		}
		// Ждём пока таймер GM соберёт и обработает пакет (time_interval_ms=5000 для player)
		time.Sleep(6 * time.Second)
	}

	// ===== STEP 4: Spatial routing — событие рядом с GM, но с другим scope_id =====
	if scenario == "all" || scenario == "spatial" {
		// Создаём GM для локации
		_ = publish("system", eventbus.Event{
			EventID:   "evt-gm-loc-" + uuid.NewString()[:8],
			EventType: "gm.created",
			Source:    "test-harness",
			WorldID:   worldID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"scope_id":   scopeLocation,
				"scope_type": "location",
			},
		})
		time.Sleep(2 * time.Second)

		// Событие с scope_id=location, но с координатами внутри видимости player GM
		// Player GM с perception=0.8 имеет радиус 0.8*200=160м
		// Координаты (123.4, 56.7) — центр player, (130, 60) — в радиусе
		_ = publish("world", eventbus.Event{
			EventID:   "evt-spatial-" + uuid.NewString()[:8],
			EventType: "npc.attacked_player",
			Source:    "test-harness",
			WorldID:   worldID,
			ScopeID:   scopePtr(scopeLocation), // Другой scope!
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"entity_id":   "npc:bandit-3",
				"target":      scopePlayer,
				"description": "Бандит из тени напал на Каина!",
				"location": map[string]interface{}{
					"x": 130.0, "y": 60.0, // В радиусе видимости player GM
				},
			},
		})
		time.Sleep(2 * time.Second)

		// Событие далеко — player GM НЕ должен получить, только location GM
		_ = publish("world", eventbus.Event{
			EventID:   "evt-far-" + uuid.NewString()[:8],
			EventType: "npc.spawned",
			Source:    "test-harness",
			WorldID:   worldID,
			ScopeID:   scopePtr(scopeLocation),
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"entity_id":   "npc:merchant-99",
				"description": "Новый торговец появился на рыночной площади.",
				"location": map[string]interface{}{
					"x": 9999.0, "y": 9999.0, // Далеко от player
				},
			},
		})
		time.Sleep(1 * time.Second)
	}

	// ===== STEP 5: state_changes — обновление entity через state_changes =====
	if scenario == "all" || scenario == "state_changes" {
		_ = publish("world", eventbus.Event{
			EventID:   "evt-state-" + uuid.NewString()[:8],
			EventType: "entity.updated",
			Source:    "entity-manager",
			WorldID:   worldID,
			ScopeID:   scopePtr(scopePlayer),
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"state_changes": []interface{}{
					map[string]interface{}{
						"entity_id": scopePlayer,
						"operations": []interface{}{
							map[string]interface{}{
								"op":    "set",
								"path":  "health.current",
								"value": 42.0,
							},
							map[string]interface{}{
								"op":    "set",
								"path":  "status",
								"value": "wounded",
							},
						},
					},
				},
			},
		})
		time.Sleep(1 * time.Second)
	}

	// ===== STEP 6: Merge/Split GM =====
	if scenario == "all" || scenario == "merge_split" {
		// Создаём два отдельных GM для merge
		_ = publish("system", eventbus.Event{
			EventID:   "evt-gm-a-" + uuid.NewString()[:8],
			EventType: "gm.created",
			Source:    "test-harness",
			WorldID:   worldID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"scope_id":   "group:party-alpha",
				"scope_type": "group",
			},
		})
		_ = publish("system", eventbus.Event{
			EventID:   "evt-gm-b-" + uuid.NewString()[:8],
			EventType: "gm.created",
			Source:    "test-harness",
			WorldID:   worldID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"scope_id":   "group:party-beta",
				"scope_type": "group",
			},
		})
		time.Sleep(2 * time.Second)

		// Merge: beta вливается в alpha
		_ = publish("system", eventbus.Event{
			EventID:   "evt-merge-" + uuid.NewString()[:8],
			EventType: "gm.merged",
			Source:    "test-harness",
			WorldID:   worldID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"scope_id":         "group:party-alpha",
				"source_scope_ids": []interface{}{"group:party-beta"},
			},
		})
		time.Sleep(1 * time.Second)

		// Split: из alpha выделяем scout
		_ = publish("system", eventbus.Event{
			EventID:   "evt-split-" + uuid.NewString()[:8],
			EventType: "gm.split",
			Source:    "test-harness",
			WorldID:   worldID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"scope_id": "group:party-alpha",
				"new_scopes": []interface{}{
					map[string]interface{}{
						"scope_id":       "group:scout-team",
						"scope_type":     "group",
						"focus_entities": []interface{}{"player:scout-1", "player:scout-2"},
					},
				},
			},
		})
		time.Sleep(1 * time.Second)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      fmt.Sprintf("Test scenario '%s' completed", scenario),
		"events_sent":  len(results),
		"steps":        results,
	})
}
