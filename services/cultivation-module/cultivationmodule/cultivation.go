// Package cultivationmodule manages cultivation systems and dao interactions.
package cultivationmodule

import (
	"context"
	"log"
	"time"

	"multiverse-core.io/shared/eventbus"

	"github.com/google/uuid"
)

// CultivationModule manages cultivation systems across plans.
type CultivationModule struct {
	bus *eventbus.EventBus
}

// NewCultivationModule creates a new CultivationModule.
func NewCultivationModule(bus *eventbus.EventBus) *CultivationModule {
	return &CultivationModule{bus: bus}
}

// HandleEvent processes events for cultivation management.
func (cm *CultivationModule) HandleEvent(ev eventbus.Event) {
	switch ev.Type {
	case "player.used_skill":
		cm.processSkillUsage(ev)
	case "ascension.completed":
		cm.handleAscension(ev)
	case "dao.interaction.attempt":
		cm.handleDaoInteraction(ev)
	case "cultivation.form.created":
		cm.handleCultivationForm(ev)
	}
}

// processSkillUsage processes skill usage for cultivation progression — с универсальным доступом через jsonpath
func (cm *CultivationModule) processSkillUsage(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение skill с поддержкой вложенной структуры и fallback
	skill, _ := pa.GetString("skill")
	if skill == "" {
		skill, _ = pa.GetString("action.skill") // fallback на вложенную структуру
	}

	// Извлечение playerID: новая структура entity.id → старая player_id
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = pa.GetString("player_id") // fallback на плоский ключ
	}

	if playerID == "" {
		log.Printf("Skill usage missing player_id")
		return
	}

	// Извлечение world_id с поддержкой обеих структур:
	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Update cultivation progress based on skill usage — с иерархической структурой событий:
	payload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithWorld(worldID)

	// Добавляем данные через dot-notation для гибкости:
	eventbus.SetNested(payload.GetCustom(), "skill_used", skill)
	eventbus.SetNested(payload.GetCustom(), "progress_gained", cm.calculateProgress(skill, worldID))

	// Сохраняем также иерархические пути для совместимости с LLM:
	eventbus.SetNested(payload.GetCustom(), "entity.id", playerID)
	eventbus.SetNested(payload.GetCustom(), "world.id", worldID)

	progressEvent := eventbus.NewStructuredEvent("cultivation.progress.updated", "cultivation-module", worldID, payload)
	progressEvent.ID = "cult-progress-" + uuid.New().String()[:8]
	progressEvent.Timestamp = time.Now()

	cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, progressEvent)
}

// handleAscension handles post-ascension cultivation changes — с универсальным доступом и иерархическими событиями:
func (cm *CultivationModule) handleAscension(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение playerID: новая структура entity.id → старая player_id
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = pa.GetString("player_id")
	}

	// Извлечение числовых параметров с типобезопасным преобразованием:
	fromPlan, _ := pa.GetFloat("from_plan")
	toPlan, _ := pa.GetFloat("to_plan")

	if playerID == "" {
		log.Printf("Ascension missing player_id")
		return
	}

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Handle dao merging on higher plans
	if toPlan > fromPlan && toPlan >= 1 {
		cm.mergeDaoPaths(ev, int(toPlan))
	}

	// Update cultivation system for new plan — с иерархической структурой:
	payload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithWorld(worldID)

	eventbus.SetNested(payload.GetCustom(), "new_plan", toPlan)
	eventbus.SetNested(payload.GetCustom(), "system_type", cm.getSystemTypeForPlan(int(toPlan)))

	// Иерархические пути для LLM-совместимости:
	eventbus.SetNested(payload.GetCustom(), "entity.id", playerID)
	eventbus.SetNested(payload.GetCustom(), "world.id", worldID)
	eventbus.SetNested(payload.GetCustom(), "ascension.from_plan", fromPlan)
	eventbus.SetNested(payload.GetCustom(), "ascension.to_plan", toPlan)

	updateEvent := eventbus.NewStructuredEvent("cultivation.system.updated", "cultivation-module", worldID, payload)
	updateEvent.ID = "cult-update-" + uuid.New().String()[:8]
	updateEvent.Timestamp = time.Now()

	cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, updateEvent)

	log.Printf("Cultivation system updated for %s at Plan %d", playerID, int(toPlan))
}

// handleDaoInteraction handles attempts to interact with other daos — с универсальным доступом и иерархическими событиями:
func (cm *CultivationModule) handleDaoInteraction(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение playerID: новая структура entity.id → старая player_id
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = pa.GetString("player_id")
	}

	// Извлечение параметров с поддержкой вложенной структуры:
	targetDao, _ := pa.GetString("target_dao")
	if targetDao == "" {
		targetDao, _ = pa.GetString("dao.target.id") // fallback на вложенную структуру
	}

	interactionType, _ := pa.GetString("interaction_type")
	if interactionType == "" {
		interactionType, _ = pa.GetString("action.type") // fallback
	}

	if playerID == "" || targetDao == "" {
		log.Printf("Dao interaction missing required fields")
		return
	}

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Check if interaction is allowed
	if cm.isDaoInteractionAllowed(playerID, targetDao, interactionType, worldID) {
		successPayload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "").
			WithWorld(worldID)

		eventbus.SetNested(successPayload.GetCustom(), "target_dao", targetDao)
		eventbus.SetNested(successPayload.GetCustom(), "interaction_type", interactionType)
		eventbus.SetNested(successPayload.GetCustom(), "result", "harmony_achieved")

		// Иерархические пути для LLM:
		eventbus.SetNested(successPayload.GetCustom(), "entity.id", playerID)
		eventbus.SetNested(successPayload.GetCustom(), "world.id", worldID)
		eventbus.SetNested(successPayload.GetCustom(), "dao.interaction.target", targetDao)
		eventbus.SetNested(successPayload.GetCustom(), "dao.interaction.type", interactionType)

		successEvent := eventbus.NewStructuredEvent("dao.interaction.success", "cultivation-module", worldID, successPayload)
		successEvent.ID = "dao-success-" + uuid.New().String()[:8]
		successEvent.Timestamp = time.Now()

		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, successEvent)
	} else {
		conflictPayload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "").
			WithWorld(worldID)

		eventbus.SetNested(conflictPayload.GetCustom(), "target_dao", targetDao)
		eventbus.SetNested(conflictPayload.GetCustom(), "interaction_type", interactionType)
		eventbus.SetNested(conflictPayload.GetCustom(), "result", "dao_conflict")

		// Иерархические пути для LLM:
		eventbus.SetNested(conflictPayload.GetCustom(), "entity.id", playerID)
		eventbus.SetNested(conflictPayload.GetCustom(), "world.id", worldID)
		eventbus.SetNested(conflictPayload.GetCustom(), "dao.conflict.target", targetDao)
		eventbus.SetNested(conflictPayload.GetCustom(), "dao.conflict.type", interactionType)
		eventbus.SetNested(conflictPayload.GetCustom(), "consequences", []string{"spiritual_damage", "path_instability"})

		conflictEvent := eventbus.NewStructuredEvent("dao.interaction.conflict", "cultivation-module", worldID, conflictPayload)
		conflictEvent.ID = "dao-conflict-" + uuid.New().String()[:8]
		conflictEvent.Timestamp = time.Now()

		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, conflictEvent)
	}
}

// handleCultivationForm handles creation of new cultivation forms — с иерархической структурой событий:
func (cm *CultivationModule) handleCultivationForm(ev eventbus.Event) {
	pa := ev.Path()

	// Извлечение с поддержкой вложенной структуры и fallback:
	formID, _ := pa.GetString("form_id")
	if formID == "" {
		formID, _ = pa.GetString("cultivation.form.id")
	}

	playerID, _ := pa.GetString("player_id")
	if playerID == "" {
		entity := eventbus.ExtractEntityID(ev.Payload)
		if entity != nil {
			playerID = entity.ID
		}
	}

	formType, _ := pa.GetString("form_type")
	if formType == "" {
		formType, _ = pa.GetString("cultivation.form.type")
	}

	if formID == "" || playerID == "" {
		log.Printf("Cultivation form missing required fields")
		return
	}

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Validate form against world ontology — с иерархической структурой:
	if cm.isFormValid(formType, worldID) {
		payload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "").
			WithWorld(worldID)

		eventbus.SetNested(payload.GetCustom(), "form_id", formID)
		eventbus.SetNested(payload.GetCustom(), "form_type", formType)
		eventbus.SetNested(payload.GetCustom(), "validation.status", "approved")

		// Иерархические пути для LLM:
		eventbus.SetNested(payload.GetCustom(), "entity.id", playerID)
		eventbus.SetNested(payload.GetCustom(), "world.id", worldID)
		eventbus.SetNested(payload.GetCustom(), "cultivation.form.id", formID)
		eventbus.SetNested(payload.GetCustom(), "cultivation.form.type", formType)

		validateEvent := eventbus.NewStructuredEvent("cultivation.form.validated", "cultivation-module", worldID, payload)
		validateEvent.ID = "form-valid-" + uuid.New().String()[:8]
		validateEvent.Timestamp = time.Now()

		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, validateEvent)
	} else {
		payload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "").
			WithWorld(worldID)

		eventbus.SetNested(payload.GetCustom(), "form_id", formID)
		eventbus.SetNested(payload.GetCustom(), "form_type", formType)
		eventbus.SetNested(payload.GetCustom(), "validation.status", "rejected")
		eventbus.SetNested(payload.GetCustom(), "validation.reason", "ontology_violation")

		// Иерархические пути для LLM:
		eventbus.SetNested(payload.GetCustom(), "entity.id", playerID)
		eventbus.SetNested(payload.GetCustom(), "world.id", worldID)
		eventbus.SetNested(payload.GetCustom(), "cultivation.form.id", formID)
		eventbus.SetNested(payload.GetCustom(), "cultivation.form.type", formType)
		eventbus.SetNested(payload.GetCustom(), "cultivation.form.violation", "ontology_mismatch")

		rejectEvent := eventbus.NewStructuredEvent("cultivation.form.rejected", "cultivation-module", worldID, payload)
		rejectEvent.ID = "form-reject-" + uuid.New().String()[:8]
		rejectEvent.Timestamp = time.Now()

		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, rejectEvent)
	}
}

// mergeDaoPaths handles dao merging on higher plans — с иерархической структурой событий:
func (cm *CultivationModule) mergeDaoPaths(ev eventbus.Event, targetPlan int) {
	pa := ev.Path()

	// Извлечение playerID с fallback:
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

	worldID := eventbus.GetWorldIDFromEvent(ev)

	// Извлечение original_paths с поддержкой вложенной структуры:
	var originalPaths interface{}
	if paths, ok := pa.GetSlice("original_paths"); ok {
		originalPaths = paths
	} else if paths, ok := pa.GetSlice("dao.original_paths"); ok {
		originalPaths = paths
	} else {
		originalPaths = ev.Payload["original_paths"] // fallback на плоский доступ
	}

	hybridPath := cm.generateHybridPath(originalPaths, targetPlan)

	// Создаём событие с иерархической структурой:
	payload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "").
		WithWorld(worldID)

	eventbus.SetNested(payload.GetCustom(), "plan_level", targetPlan)
	eventbus.SetNested(payload.GetCustom(), "original_paths", originalPaths)
	eventbus.SetNested(payload.GetCustom(), "hybrid_path", hybridPath)
	eventbus.SetNested(payload.GetCustom(), "dao.merger.stability", "unstable")

	// Иерархические пути для LLM:
	eventbus.SetNested(payload.GetCustom(), "entity.id", playerID)
	eventbus.SetNested(payload.GetCustom(), "world.id", worldID)
	eventbus.SetNested(payload.GetCustom(), "dao.merger.plan", targetPlan)
	eventbus.SetNested(payload.GetCustom(), "dao.merger.result", "hybrid_formed")

	mergeEvent := eventbus.NewStructuredEvent("dao.hybrid_formed", "cultivation-module", worldID, payload)
	mergeEvent.ID = "dao-merge-" + uuid.New().String()[:8]
	mergeEvent.Timestamp = time.Now()

	cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, mergeEvent)

	log.Printf("Hybrid dao formed for %s at Plan %d", playerID, targetPlan)
}

// calculateProgress calculates cultivation progress based on skill and world.
func (cm *CultivationModule) calculateProgress(skill, worldID string) float64 {
	// World-specific progress multipliers
	switch worldID {
	case "pain-realm":
		if skill == "scream_of_pain" {
			return 1.5 // Bonus for world-aligned skills
		}
	case "memory-realm":
		if skill == "memory_whisper" {
			return 1.5
		}
	}
	return 1.0 // Default progress
}

// getSystemTypeForPlan returns the cultivation system type for a plan level.
func (cm *CultivationModule) getSystemTypeForPlan(plan int) string {
	switch plan {
	case 0:
		return "unique_per_world" // Unique system per base world
	case 1:
		return "hybrid_system" // Hybrid systems in convergence zones
	default:
		return "abstract_dao" // Abstract cultivation on higher plans
	}
}

// isDaoInteractionAllowed checks if dao interaction is allowed.
func (cm *CultivationModule) isDaoInteractionAllowed(playerID, targetDao, interactionType, worldID string) bool {
	// Example rules
	if worldID == "pain-realm" && interactionType == "harmonize" {
		return false // Harmonization forbidden in World of Pain
	}
	if interactionType == "absorb" && targetDao == "forbidden_dao" {
		return false // Cannot absorb forbidden dao
	}
	return true // Allowed by default
}

// isFormValid validates cultivation form against world ontology.
func (cm *CultivationModule) isFormValid(formType, worldID string) bool {
	// World-specific form validation
	switch worldID {
	case "mechanism-realm":
		if formType == "organic_form" {
			return false // Organic forms forbidden in Mechanism Realm
		}
	case "pain-realm":
		if formType == "healing_form" {
			return false // Healing forms forbidden in World of Pain
		}
	}
	return true // Valid by default
}

// generateHybridPath generates a hybrid dao path name.
func (cm *CultivationModule) generateHybridPath(originalPaths interface{}, plan int) string {
	// Simplified implementation
	return "Hybrid Dao of Plan " + string(rune('0'+plan))
}
