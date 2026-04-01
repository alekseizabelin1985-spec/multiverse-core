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
	switch ev.EventType {
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

// processSkillUsage processes skill usage for cultivation progression.
func (cm *CultivationModule) processSkillUsage(ev eventbus.Event) {
	skill, _ := ev.Payload["skill"].(string)

	// Извлекаем playerID с поддержкой новой структуры (entity.id) и fallback (player_id)
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = ev.Payload["player_id"].(string)
	}

	if playerID == "" {
		log.Printf("Skill usage missing player_id")
		return
	}

	// Update cultivation progress based on skill usage
	payload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "")

	eventbus.SetNested(payload.GetCustom(), "skill_used", skill)
	eventbus.SetNested(payload.GetCustom(), "progress_gained", cm.calculateProgress(skill, ev.WorldID))

	progressEvent := eventbus.NewStructuredEvent("cultivation.progress.updated", "cultivation-module", ev.WorldID, payload)
	progressEvent.EventID = "cult-progress-" + uuid.New().String()[:8]
	progressEvent.Timestamp = time.Now()

	cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, progressEvent)
}

// handleAscension handles post-ascension cultivation changes.
func (cm *CultivationModule) handleAscension(ev eventbus.Event) {
	// Извлекаем playerID с поддержкой новой структуры (entity.id) и fallback (player_id)
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = ev.Payload["player_id"].(string)
	}

	fromPlan, _ := ev.Payload["from_plan"].(float64)
	toPlan, _ := ev.Payload["to_plan"].(float64)

	if playerID == "" {
		log.Printf("Ascension missing player_id")
		return
	}

	// Handle dao merging on higher plans
	if toPlan > fromPlan && toPlan >= 1 {
		cm.mergeDaoPaths(ev, int(toPlan))
	}

	// Update cultivation system for new plan
	payload := eventbus.NewEventPayload().
		WithEntity(playerID, "player", "")

	eventbus.SetNested(payload.GetCustom(), "new_plan", toPlan)
	eventbus.SetNested(payload.GetCustom(), "system_type", cm.getSystemTypeForPlan(int(toPlan)))

	updateEvent := eventbus.NewStructuredEvent("cultivation.system.updated", "cultivation-module", ev.WorldID, payload)
	updateEvent.EventID = "cult-update-" + uuid.New().String()[:8]
	updateEvent.Timestamp = time.Now()

	cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, updateEvent)

	log.Printf("Cultivation system updated for %s at Plan %d", playerID, int(toPlan))
}

// handleDaoInteraction handles attempts to interact with other daos.
func (cm *CultivationModule) handleDaoInteraction(ev eventbus.Event) {
	// Извлекаем playerID с поддержкой новой структуры (entity.id) и fallback (player_id)
	entity := eventbus.ExtractEntityID(ev.Payload)
	var playerID string
	if entity != nil && entity.ID != "" {
		playerID = entity.ID
	} else {
		playerID, _ = ev.Payload["player_id"].(string)
	}

	targetDao, _ := ev.Payload["target_dao"].(string)
	interactionType, _ := ev.Payload["interaction_type"].(string)

	if playerID == "" || targetDao == "" {
		log.Printf("Dao interaction missing required fields")
		return
	}

	// Check if interaction is allowed
	if cm.isDaoInteractionAllowed(playerID, targetDao, interactionType, ev.WorldID) {
		successPayload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "")

		eventbus.SetNested(successPayload.GetCustom(), "target_dao", targetDao)
		eventbus.SetNested(successPayload.GetCustom(), "interaction_type", interactionType)
		eventbus.SetNested(successPayload.GetCustom(), "result", "harmony_achieved")

		successEvent := eventbus.NewStructuredEvent("dao.interaction.success", "cultivation-module", ev.WorldID, successPayload)
		successEvent.EventID = "dao-success-" + uuid.New().String()[:8]
		successEvent.Timestamp = time.Now()

		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, successEvent)
	} else {
		conflictPayload := eventbus.NewEventPayload().
			WithEntity(playerID, "player", "")

		eventbus.SetNested(conflictPayload.GetCustom(), "target_dao", targetDao)
		eventbus.SetNested(conflictPayload.GetCustom(), "interaction_type", interactionType)
		eventbus.SetNested(conflictPayload.GetCustom(), "result", "dao_conflict")

		conflictEvent := eventbus.NewStructuredEvent("dao.interaction.conflict", "cultivation-module", ev.WorldID, conflictPayload)
		conflictEvent.EventID = "dao-conflict-" + uuid.New().String()[:8]
		conflictEvent.Timestamp = time.Now()
		eventbus.SetNested(conflictPayload.GetCustom(), "consequences", []string{"spiritual_damage", "path_instability"})
		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, conflictEvent)
	}
}

// handleCultivationForm handles creation of new cultivation forms.
func (cm *CultivationModule) handleCultivationForm(ev eventbus.Event) {
	formID, _ := ev.Payload["form_id"].(string)
	playerID, _ := ev.Payload["player_id"].(string)
	formType, _ := ev.Payload["form_type"].(string)

	if formID == "" || playerID == "" {
		log.Printf("Cultivation form missing required fields")
		return
	}

	// Validate form against world ontology
	if cm.isFormValid(formType, ev.WorldID) {
		validateEvent := eventbus.Event{
			EventID:   "form-valid-" + uuid.New().String()[:8],
			EventType: "cultivation.form.validated",
			Source:    "cultivation-module",
			WorldID:   ev.WorldID,
			Payload: map[string]interface{}{
				"form_id":   formID,
				"player_id": playerID,
				"form_type": formType,
				"status":    "approved",
			},
			Timestamp: time.Now(),
		}
		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, validateEvent)
	} else {
		rejectEvent := eventbus.Event{
			EventID:   "form-reject-" + uuid.New().String()[:8],
			EventType: "cultivation.form.rejected",
			Source:    "cultivation-module",
			WorldID:   ev.WorldID,
			Payload: map[string]interface{}{
				"form_id":   formID,
				"player_id": playerID,
				"form_type": formType,
				"status":    "rejected",
				"reason":    "ontology_violation",
			},
			Timestamp: time.Now(),
		}
		cm.bus.Publish(context.Background(), eventbus.TopicWorldEvents, rejectEvent)
	}
}

// mergeDaoPaths handles dao merging on higher plans.
func (cm *CultivationModule) mergeDaoPaths(ev eventbus.Event, targetPlan int) {
	playerID, _ := ev.Payload["player_id"].(string)

	mergeEvent := eventbus.Event{
		EventID:   "dao-merge-" + uuid.New().String()[:8],
		EventType: "dao.hybrid_formed",
		Source:    "cultivation-module",
		WorldID:   ev.WorldID,
		Payload: map[string]interface{}{
			"player_id":      playerID,
			"plan_level":     targetPlan,
			"original_paths": ev.Payload["original_paths"],
			"hybrid_path":    cm.generateHybridPath(ev.Payload["original_paths"], targetPlan),
			"stability":      "unstable", // Requires further cultivation
		},
		Timestamp: time.Now(),
	}
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
