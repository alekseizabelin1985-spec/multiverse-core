// Package banofworld implements the BanOfWorld (World Integrity Protection).
package banofworld

import (
	"context"
	"log"
	"time"

	"multiverse-core.io/shared/eventbus"

	"github.com/google/uuid"
)

// BanOfWorld protects world integrity through resonance with the Core.
type BanOfWorld struct {
	bus *eventbus.EventBus
}

// NewBanOfWorld creates a new BanOfWorld.
func NewBanOfWorld(bus *eventbus.EventBus) *BanOfWorld {
	return &BanOfWorld{bus: bus}
}

// HandlePlayerEvent processes player events for world integrity checks.
func (b *BanOfWorld) HandlePlayerEvent(ev eventbus.Event) {
	// Check for skill usage violations
	if ev.EventType == "player.used_skill" {
		b.checkSkillUsage(ev)
	}

	// Check for item usage violations
	if ev.EventType == "player.used_item" {
		b.checkItemUsage(ev)
	}

	// Check for movement violations
	if ev.EventType == "player.moved" {
		b.checkMovement(ev)
	}
}

// checkSkillUsage checks if a skill usage violates world integrity — с универсальным доступом и иерархическими событиями:
func (b *BanOfWorld) checkSkillUsage(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение skill с поддержкой вложенной структуры и fallback
	skill, _ := pa.GetString("skill")
	if skill == "" {
		skill, _ = pa.GetString("action.skill")
	}

	// Извлечение playerID: новая структура entity.id → старая player_id
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = pa.GetString("player_id")
	}

	if playerID == "" {
		log.Printf("Skill usage missing player_id")
		return
	}

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Get world core violations
	violationType := b.getViolationType(worldID, skill)

	if violationType != "" {
		log.Printf("Violation detected in %s: %s used %s", worldID, playerID, skill)

		// Publish violation event с иерархической структурой:
		payload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "").
			WithWorld(worldID)

		eventbus.SetNested(payload.GetCustom(), "skill", skill)
		eventbus.SetNested(payload.GetCustom(), "violation_type", violationType)
		eventbus.SetNested(payload.GetCustom(), "original_event", ev.EventID)

		// Иерархические пути для LLM:
		eventbus.SetNested(payload.GetCustom(), "entity.id", playerID)
		eventbus.SetNested(payload.GetCustom(), "world.id", worldID)
		eventbus.SetNested(payload.GetCustom(), "violation.skill", skill)
		eventbus.SetNested(payload.GetCustom(), "violation.type", violationType)

		violationEvent := eventbus.NewStructuredEvent("violation.detected", "ban-of-world", worldID, payload)
		violationEvent.EventID = "violation-" + uuid.New().String()[:8]
		violationEvent.Timestamp = ev.Timestamp
		// ScopeID is set via WithScope if needed, or use ev.ScopeID for fallback
		if scope := eventbus.GetScopeFromEvent(ev); scope != nil {
			violationEvent.ScopeID = &scope.ID
		} else {
			violationEvent.ScopeID = ev.ScopeID
		}

		// ✨ Этап 6: Явные связи — игрок нарушил правила мира
		violationEvent.Relations = []eventbus.Relation{
			{
				From:     playerID,
				To:       worldID,
				Type:     eventbus.RelActedOn,
				Directed: true,
				Metadata: map[string]any{
					"violation_type": violationType,
					"skill":          skill,
					"original_event": ev.EventID,
				},
			},
		}

		b.bus.Publish(context.Background(), eventbus.TopicWorldEvents, violationEvent)

		// Apply transformation or punishment
		b.applyConsequence(ev, violationType)
	}
}

// checkItemUsage checks if an item usage violates world integrity — с универсальным доступом:
func (b *BanOfWorld) checkItemUsage(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение item с поддержкой вложенной структуры: item или action.item
	item, _ := pa.GetString("item")
	if item == "" {
		item, _ = pa.GetString("action.item")
	}

	// Извлечение playerID: новая структура entity.id → старая player_id
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = pa.GetString("player_id")
	}

	if playerID == "" {
		return
	}

	violationType := b.getViolationType(ev.WorldID, "item:"+item)

	if violationType != "" {
		worldID := eventbus.GetWorldIDFromEvent(ev)
		log.Printf("Item violation detected in %s: %s used %s", worldID, playerID, item)

		payload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "").
			WithWorld(worldID)

		eventbus.SetNested(payload.GetCustom(), "item", item)
		eventbus.SetNested(payload.GetCustom(), "violation_type", violationType)
		eventbus.SetNested(payload.GetCustom(), "original_event", ev.EventID)

		violationEvent := eventbus.NewStructuredEvent("violation.detected", "ban-of-world", worldID, payload)
		violationEvent.EventID = "violation-" + uuid.New().String()[:8]
		violationEvent.Timestamp = ev.Timestamp
		violationEvent.ScopeID = ev.ScopeID

		// ✨ Этап 6: Явные связи — игрок нарушил правила мира через предмет
		violationEvent.Relations = []eventbus.Relation{
			{
				From:     playerID,
				To:       worldID,
				Type:     eventbus.RelActedOn,
				Directed: true,
				Metadata: map[string]any{
					"violation_type": violationType,
					"item":           item,
					"original_event": ev.EventID,
				},
			},
		}

		b.bus.Publish(context.Background(), eventbus.TopicWorldEvents, violationEvent)

		b.applyConsequence(ev, violationType)
	}
}

// checkMovement checks if movement violates world boundaries.
func (b *BanOfWorld) checkMovement(ev eventbus.Event) {
	destination, _ := ev.Payload["destination"].(string)

	// Извлекаем playerID с поддержкой новой структуры (entity.id) и fallback (player_id)
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = ev.Payload["player_id"].(string)
	}

	if playerID == "" || destination == "" {
		return
	}

	// Example: players cannot leave certain worlds
	if ev.WorldID == "prison-realm" && destination != ev.WorldID {
		log.Printf("Movement violation in %s: %s tried to leave", ev.WorldID, playerID)

		payload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "")

		eventbus.SetNested(payload.GetCustom(), "attempted_destination", destination)
		eventbus.SetNested(payload.GetCustom(), "violation_type", "forbidden_movement")
		eventbus.SetNested(payload.GetCustom(), "original_event", ev.EventID)

		violationEvent := eventbus.NewStructuredEvent("violation.detected", "ban-of-world", ev.WorldID, payload)
		violationEvent.EventID = "violation-" + uuid.New().String()[:8]
		violationEvent.ScopeID = ev.ScopeID
		violationEvent.Timestamp = ev.Timestamp

		b.bus.Publish(context.Background(), eventbus.TopicWorldEvents, violationEvent)

		// Teleport back or apply punishment
		b.applyMovementConsequence(ev)
	}
}

// getViolationType determines the type of violation based on world and action.
func (b *BanOfWorld) getViolationType(worldID, action string) string {
	// World-specific violation rules
	switch worldID {
	case "pain-realm":
		if action == "fire_breath" || action == "item:healing_potion" {
			return "elemental_conflict" // Fire and healing forbidden in World of Pain
		}
	case "memory-realm":
		if action == "memory_erase" {
			return "memory_violation" // Memory erasure forbidden in World of Memory
		}
	case "mechanism-realm":
		if action == "organic_skill" {
			return "mechanical_purity" // Organic skills forbidden in World of Mechanisms
		}
	}

	return "" // No violation
}

// applyConsequence applies the appropriate consequence for a violation.
func (b *BanOfWorld) applyConsequence(ev eventbus.Event, violationType string) {
	// Извлекаем playerID с поддержкой новой структуры (entity.id) и fallback (player_id)
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = ev.Payload["player_id"].(string)
	}

	switch violationType {
	case "elemental_conflict":
		// Transform fire breath to scream of pain
		transformPayload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "")

		eventbus.SetNested(transformPayload.GetCustom(), "original", ev.Payload["skill"])
		eventbus.SetNested(transformPayload.GetCustom(), "transformed", "scream_of_pain")
		eventbus.SetNested(transformPayload.GetCustom(), "reason", "resonance_with_core")

		transformEvent := eventbus.NewStructuredEvent("skill.transformed", "ban-of-world", ev.WorldID, transformPayload)
		transformEvent.EventID = "transform-" + uuid.New().String()[:8]
		transformEvent.Timestamp = time.Now()

		b.bus.Publish(context.Background(), eventbus.TopicWorldEvents, transformEvent)

	case "memory_violation":
		// Apply memory corruption punishment
		punishPayload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "")

		eventbus.SetNested(punishPayload.GetCustom(), "punishment", "memory_corruption")
		eventbus.SetNested(punishPayload.GetCustom(), "duration", "1h")
		eventbus.SetNested(punishPayload.GetCustom(), "reason", "violation_of_memory_laws")

		punishEvent := eventbus.NewStructuredEvent("player.punished", "ban-of-world", ev.WorldID, punishPayload)
		punishEvent.EventID = "punish-" + uuid.New().String()[:8]
		punishEvent.Timestamp = time.Now()

		b.bus.Publish(context.Background(), eventbus.TopicWorldEvents, punishEvent)

	case "mechanical_purity":
		// Transform organic skill to mechanical equivalent
		transformPayload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "")

		eventbus.SetNested(transformPayload.GetCustom(), "original", ev.Payload["skill"])
		eventbus.SetNested(transformPayload.GetCustom(), "transformed", "mechanical_equivalent")
		eventbus.SetNested(transformPayload.GetCustom(), "reason", "mechanical_world_integrity")

		transformEvent := eventbus.NewStructuredEvent("skill.transformed", "ban-of-world", ev.WorldID, transformPayload)
		transformEvent.EventID = "transform-" + uuid.New().String()[:8]
		transformEvent.Timestamp = time.Now()

		b.bus.Publish(context.Background(), eventbus.TopicWorldEvents, transformEvent)
	}
}

// applyMovementConsequence handles movement violations.
func (b *BanOfWorld) applyMovementConsequence(ev eventbus.Event) {
	playerID, _ := ev.Payload["player_id"].(string)

	// Teleport back to original location
	teleportEvent := eventbus.Event{
		EventID:   "teleport-" + uuid.New().String()[:8],
		EventType: "player.teleported",
		Source:    "ban-of-world",
		WorldID:   ev.WorldID,
		Payload: map[string]interface{}{
			"player_id":   playerID,
			"destination": ev.WorldID, // Back to original world
			"reason":      "forbidden_movement",
		},
		Timestamp: time.Now(),
	}
	b.bus.Publish(context.Background(), eventbus.TopicWorldEvents, teleportEvent)
}
