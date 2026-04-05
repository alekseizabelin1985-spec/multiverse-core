// Package citygovernor manages city life, NPCs, quests, and economy.
package citygovernor

import (
	"context"
	"log"
	"time"

	"multiverse-core.io/shared/eventbus"

	"github.com/google/uuid"
)

// CityGovernor manages city-related logic.
type CityGovernor struct {
	bus *eventbus.EventBus
}

// NewCityGovernor creates a new CityGovernor.
func NewCityGovernor(bus *eventbus.EventBus) *CityGovernor {
	return &CityGovernor{bus: bus}
}

// HandleEvent processes events for city management.
func (cg *CityGovernor) HandleEvent(ev eventbus.Event) {
	if eventbus.GetScopeFromEvent(ev) == nil {
		return // Not a city-scoped event
	}

	switch ev.Type {
	case "player.entered":
		cg.handlePlayerEntry(ev)
	case "violation.detected":
		cg.handleViolation(ev)
	case "quest.completed":
		cg.handleQuestCompletion(ev)
	case "city.reputation.changed":
		cg.handleReputationChange(ev)
	case "npc.interaction":
		cg.handleNPCInteraction(ev)
	}
}

// handlePlayerEntry handles player entry into a city — с универсальным доступом и иерархическими событиями:
func (cg *CityGovernor) handlePlayerEntry(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение playerID: новая структура entity.id → старая player_id
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = pa.GetString("player_id")
	}

	// Извлечение city_id: новая структура scope.id → старая scope_id
	var cityID string
	if scope := eventbus.GetScopeFromEvent(ev); scope != nil && scope.Type == "city" {
		cityID = scope.ID
	} else {
		cityID, _ = pa.GetString("city_id")
	}

	if playerID == "" || cityID == "" {
		log.Printf("Player entry missing player_id or city_id")
		return
	}

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Generate welcome quest if player is new
	if cg.isNewPlayerInCity(playerID, cityID) {
		cg.generateWelcomeQuest(ev)
	}

	// Update city population
	cg.updateCityPopulation(cityID, 1)

	// Notify NPCs of new arrival — с иерархической структурой событий:
	npcPayload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithScope(cityID, "city").
		WithWorld(worldID)

	eventbus.SetNested(npcPayload.GetCustom(), "event_type", "new_arrival")
	eventbus.SetNested(npcPayload.GetCustom(), "city.id", cityID)

	// Иерархические пути для LLM:
	eventbus.SetNested(npcPayload.GetCustom(), "entity.id", playerID)
	eventbus.SetNested(npcPayload.GetCustom(), "world.entity.id", worldID)
	eventbus.SetNested(npcPayload.GetCustom(), "scope.id", cityID)
	eventbus.SetNested(npcPayload.GetCustom(), "scope.type", "city")

	npcEvent := eventbus.NewStructuredEvent("citizen.event", "city-governor", worldID, npcPayload)
	npcEvent.ID = "npc-notify-" + uuid.New().String()[:8]
	npcEvent.Timestamp = time.Now()

	// ✨ Этап 6: Явные связи — игрок вошёл в город
	npcEvent.Relations = []eventbus.Relation{
		{
			From:     playerID,
			To:       cityID,
			Type:     eventbus.RelLocatedIn,
			Directed: true,
		},
	}

	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, npcEvent)

	log.Printf("Player %s entered city %s", playerID, cityID)
}

// handleViolation handles world integrity violations in the city — с универсальным доступом и иерархическими событиями:
func (cg *CityGovernor) handleViolation(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение city_id: новая структура scope.id → старая scope_id
	var cityID string
	if scope := eventbus.GetScopeFromEvent(ev); scope != nil && scope.Type == "city" {
		cityID = scope.ID
	} else {
		cityID, _ = pa.GetString("city_id")
	}

	// Извлечение playerID: новая структура entity.id → старая player_id
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = pa.GetString("player_id")
	}

	violationType, _ := pa.GetString("violation_type")
	if violationType == "" {
		violationType, _ = pa.GetString("violation.type") // fallback на вложенную структуру
	}

	if playerID == "" || cityID == "" {
		return
	}

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Apply city-specific consequences
	consequence := cg.getCityConsequence(violationType, cityID)

	// Создаём событие с иерархической структурой:
	consequencePayload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithScope(cityID, "city").
		WithWorld(worldID)

	eventbus.SetNested(consequencePayload.GetCustom(), "violation_type", violationType)
	eventbus.SetNested(consequencePayload.GetCustom(), "consequence", consequence)
	eventbus.SetNested(consequencePayload.GetCustom(), "city.id", cityID)

	// Иерархические пути для LLM:
	eventbus.SetNested(consequencePayload.GetCustom(), "entity.id", playerID)
	eventbus.SetNested(consequencePayload.GetCustom(), "world.entity.id", worldID)
	eventbus.SetNested(consequencePayload.GetCustom(), "scope.id", cityID)
	eventbus.SetNested(consequencePayload.GetCustom(), "scope.type", "city")
	eventbus.SetNested(consequencePayload.GetCustom(), "violation.type", violationType)

	consequenceEvent := eventbus.NewStructuredEvent("city.violation.consequence", "city-governor", worldID, consequencePayload)
	consequenceEvent.ID = "city-conseq-" + uuid.New().String()[:8]
	consequenceEvent.Timestamp = time.Now()

	// ✨ Этап 6: Явные связи — город наказал игрока за нарушение
	consequenceEvent.Relations = []eventbus.Relation{
		{
			From:     cityID,
			To:       playerID,
			Type:     eventbus.RelActedOn,
			Directed: true,
			Metadata: map[string]any{
				"violation_type": violationType,
				"consequence":    consequence,
			},
		},
	}

	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, consequenceEvent)

	// Update city reputation
	cg.updateCityReputation(cityID, -10) // Reputation decreases on violations

	log.Printf("Applied consequence %s for violation %s in city %s", consequence, violationType, cityID)
}

// handleQuestCompletion handles quest completion events.
func (cg *CityGovernor) handleQuestCompletion(ev eventbus.Event) {
	// Извлекаем playerID с поддержкой новой структуры (entity.id) и fallback (player_id)
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = ev.Payload["player_id"].(string)
	}
	questID, _ := ev.Payload["quest_id"].(string)
	scope := eventbus.GetScopeFromEvent(ev)
	if scope == nil {
		return
	}
	cityID := scope.ID

	if playerID == "" || questID == "" {
		return
	}

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Generate reward
	reward := cg.generateQuestReward(questID, cityID)

	rewardPayload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithScope(cityID, "city").
		WithWorld(worldID)

	eventbus.SetNested(rewardPayload.GetCustom(), "quest_id", questID)
	eventbus.SetNested(rewardPayload.GetCustom(), "reward", reward)
	eventbus.SetNested(rewardPayload.GetCustom(), "city.id", cityID)

	rewardEvent := eventbus.NewStructuredEvent("quest.reward.granted", "city-governor", worldID, rewardPayload)
	rewardEvent.ID = "quest-reward-" + uuid.New().String()[:8]
	rewardEvent.Timestamp = time.Now()
	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, rewardEvent)

	// Update reputation based on quest type
	questType, _ := ev.Payload["quest_type"].(string)
	reputationChange := cg.getReputationChangeForQuest(questType)
	cg.updateCityReputation(cityID, reputationChange)

	// Generate new quest
	cg.generateNewQuest(ev)

	log.Printf("Granted reward for quest %s to player %s in city %s", questID, playerID, cityID)
}

// handleReputationChange handles reputation changes.
func (cg *CityGovernor) handleReputationChange(ev eventbus.Event) {
	scope := eventbus.GetScopeFromEvent(ev)
	if scope == nil {
		return
	}
	cityID := scope.ID
	change, _ := ev.Payload["change"].(float64)

	newReputation := cg.getCurrentReputation(cityID) + int(change)

	// Apply reputation effects
	cg.applyReputationEffects(cityID, newReputation)

	log.Printf("City %s reputation changed to %d", cityID, newReputation)
}

// handleNPCInteraction handles NPC interaction events.
func (cg *CityGovernor) handleNPCInteraction(ev eventbus.Event) {
	// Извлекаем playerID с поддержкой новой структуры (entity.id) и fallback (player_id)
	playerEntity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if playerEntity != nil && playerEntity.ID != "" {
		playerID = playerEntity.ID
	} else {
		playerID, _ = ev.Payload["player_id"].(string)
	}

	// Извлекаем npcID с поддержкой новой структуры (target.entity.id) и fallback (npc_id)
	npcEntity := eventbus.ExtractTargetEntityID(ev.Payload)
	var npcID string
	if npcEntity != nil && npcEntity.ID != "" {
		npcID = npcEntity.ID
	} else {
		npcID, _ = ev.Payload["npc_id"].(string)
	}

	interactionType, _ := ev.Payload["interaction_type"].(string)
	scope := eventbus.GetScopeFromEvent(ev)
	if scope == nil {
		return
	}
	cityID := scope.ID

	if playerID == "" || npcID == "" {
		return
	}

	// Generate interaction response
	response := cg.generateNPCResponse(npcID, interactionType, cityID)

	responsePayload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithScope(cityID, "city").
		WithWorld(eventbus.GetWorldIDFromEvent(ev))

	eventbus.SetNested(responsePayload.GetCustom(), "npc_id", npcID)
	eventbus.SetNested(responsePayload.GetCustom(), "interaction_type", interactionType)
	eventbus.SetNested(responsePayload.GetCustom(), "response", response)
	eventbus.SetNested(responsePayload.GetCustom(), "city.id", cityID)

	responseEvent := eventbus.NewStructuredEvent("npc.response.generated", "city-governor", eventbus.GetWorldIDFromEvent(ev), responsePayload)
	responseEvent.ID = "npc-response-" + uuid.New().String()[:8]
	responseEvent.Timestamp = time.Now()
	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, responseEvent)

	log.Printf("Generated NPC response for %s -> %s in city %s", playerID, npcID, cityID)
}

// generateWelcomeQuest generates a welcome quest for new players.
func (cg *CityGovernor) generateWelcomeQuest(ev eventbus.Event) {
	playerID, _ := ev.Payload["player_id"].(string)
	scope := eventbus.GetScopeFromEvent(ev)
	if scope == nil {
		return
	}
	cityID := scope.ID
	worldID := eventbus.GetWorldIDFromEvent(ev)

	questID := "welcome-" + uuid.New().String()[:8]

	questPayload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithScope(cityID, "city").
		WithWorld(worldID)

	eventbus.SetNested(questPayload.GetCustom(), "quest_id", questID)
	eventbus.SetNested(questPayload.GetCustom(), "title", "Добро пожаловать в "+cg.getCityName(cityID))
	eventbus.SetNested(questPayload.GetCustom(), "description", "Старейшина просит вас принести ему Слёзы Памяти из ближайшего леса.")
	eventbus.SetNested(questPayload.GetCustom(), "reward", "50 золотых и репутация +10")
	eventbus.SetNested(questPayload.GetCustom(), "quest_type", "welcome")
	eventbus.SetNested(questPayload.GetCustom(), "city.id", cityID)

	questEvent := eventbus.NewStructuredEvent("quest.assigned", "city-governor", worldID, questPayload)
	questEvent.ID = "quest-welcome-" + uuid.New().String()[:8]
	questEvent.Timestamp = time.Now()
	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, questEvent)
}

// generateNewQuest generates a new quest after completion.
func (cg *CityGovernor) generateNewQuest(ev eventbus.Event) {
	playerID, _ := ev.Payload["player_id"].(string)
	scope := eventbus.GetScopeFromEvent(ev)
	if scope == nil {
		return
	}
	cityID := scope.ID
	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Determine quest type based on player history and city state
	questType := cg.determineNextQuestType(playerID, cityID)

	questID := "quest-" + uuid.New().String()[:8]

	questPayload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithScope(cityID, "city").
		WithWorld(worldID)

	eventbus.SetNested(questPayload.GetCustom(), "quest_id", questID)
	eventbus.SetNested(questPayload.GetCustom(), "title", cg.generateQuestTitle(questType, cityID))
	eventbus.SetNested(questPayload.GetCustom(), "description", cg.generateQuestDescription(questType, cityID))
	eventbus.SetNested(questPayload.GetCustom(), "reward", cg.generateQuestReward(questID, cityID))
	eventbus.SetNested(questPayload.GetCustom(), "quest_type", questType)
	eventbus.SetNested(questPayload.GetCustom(), "city.id", cityID)

	questEvent := eventbus.NewStructuredEvent("quest.assigned", "city-governor", worldID, questPayload)
	questEvent.ID = "quest-new-" + uuid.New().String()[:8]
	questEvent.Timestamp = time.Now()
	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, questEvent)
}

// Helper methods (simplified implementations)

func (cg *CityGovernor) isNewPlayerInCity(playerID, cityID string) bool {
	// In a real implementation, this would check player history
	return true // Simplified for example
}

func (cg *CityGovernor) updateCityPopulation(cityID string, delta int) {
	// Publish population update event
	popPayload := eventbus.NewEventPayload().
		WithScope(cityID, "city").
		WithWorld("global")

	eventbus.SetNested(popPayload.GetCustom(), "delta", delta)
	eventbus.SetNested(popPayload.GetCustom(), "city.id", cityID)

	popEvent := eventbus.NewStructuredEvent("city.population.changed", "city-governor", "global", popPayload)
	popEvent.ID = "pop-update-" + uuid.New().String()[:8]
	popEvent.Timestamp = time.Now()
	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, popEvent)
}

func (cg *CityGovernor) getCityConsequence(violationType, cityID string) string {
	// City-specific consequences
	switch cityID {
	case "city-ashes":
		return "imprisonment" // Harsh consequences in City of Ashes
	case "city-archives":
		return "memory_wipe" // Memory-based consequences in City of Archives
	default:
		return "fine" // Default consequence
	}
}

func (cg *CityGovernor) updateCityReputation(cityID string, delta int) {
	repPayload := eventbus.NewEventPayload().
		WithScope(cityID, "city").
		WithWorld("global")

	eventbus.SetNested(repPayload.GetCustom(), "change", delta)
	eventbus.SetNested(repPayload.GetCustom(), "city.id", cityID)

	repEvent := eventbus.NewStructuredEvent("city.reputation.changed", "city-governor", "global", repPayload)
	repEvent.ID = "rep-update-" + uuid.New().String()[:8]
	repEvent.Timestamp = time.Now()
	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, repEvent)
}

func (cg *CityGovernor) generateQuestReward(questID, cityID string) string {
	// Generate reward based on quest and city
	return "100 золотых и уникальный предмет"
}

func (cg *CityGovernor) getReputationChangeForQuest(questType string) int {
	switch questType {
	case "help_citizen":
		return 5
	case "defeat_monster":
		return 10
	case "violation":
		return -15
	default:
		return 0
	}
}

func (cg *CityGovernor) getCurrentReputation(cityID string) int {
	// In real implementation, this would query city state
	return 50 // Default reputation
}

func (cg *CityGovernor) applyReputationEffects(cityID string, reputation int) {
	// Apply effects based on reputation level
	var effect string
	if reputation > 75 {
		effect = "prosperity" // City thrives
	} else if reputation < 25 {
		effect = "decay" // City deteriorates
	} else {
		effect = "stability" // Normal state
	}

	effectPayload := eventbus.NewEventPayload().
		WithScope(cityID, "city").
		WithWorld("global")

	eventbus.SetNested(effectPayload.GetCustom(), "effect", effect)
	eventbus.SetNested(effectPayload.GetCustom(), "level", reputation)
	eventbus.SetNested(effectPayload.GetCustom(), "city.id", cityID)

	effectEvent := eventbus.NewStructuredEvent("city.reputation.effect", "city-governor", "global", effectPayload)
	effectEvent.ID = "rep-effect-" + uuid.New().String()[:8]
	effectEvent.Timestamp = time.Now()
	cg.bus.Publish(context.Background(), eventbus.TopicGameEvents, effectEvent)
}

func (cg *CityGovernor) generateNPCResponse(npcID, interactionType, cityID string) string {
	// Generate context-aware NPC response
	return "Старейшина кивает вам и говорит: 'Добро пожаловать в наш город.'"
}

func (cg *CityGovernor) getCityName(cityID string) string {
	// Map city IDs to names
	switch cityID {
	case "city-ashes":
		return "Город Пепла"
	case "city-archives":
		return "Город Архивов"
	default:
		return "Неизвестный город"
	}
}

func (cg *CityGovernor) determineNextQuestType(playerID, cityID string) string {
	// Determine quest type based on player and city state
	return "help_citizen" // Simplified
}

func (cg *CityGovernor) generateQuestTitle(questType, cityID string) string {
	return "Помощь горожанину"
}

func (cg *CityGovernor) generateQuestDescription(questType, cityID string) string {
	return "Один из горожан просит вашей помощи с доставкой посылки."
}
