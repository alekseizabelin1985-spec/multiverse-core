// services/narrativeorchestrator/narrative_agent_factory.go

package narrativeorchestrator

import (
	"context"
	"fmt"
	"log"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/eventbus"
	"time"
)

// NarrativeAgentFactory реализует agent.AgentFactory
// Создает NarrativeAgent instances через lifecycle
type NarrativeAgentFactory struct {
	pipeline *agent.TwoPhasePipeline
	bus      *eventbus.EventBus
	logger   *log.Logger
}

// NewNarrativeAgentFactory создает новую фабрику
func NewNarrativeAgentFactory(pipeline *agent.TwoPhasePipeline, bus *eventbus.EventBus, logger *log.Logger) *NarrativeAgentFactory {
	return &NarrativeAgentFactory{
		pipeline: pipeline,
		bus:      bus,
		logger:   logger,
	}
}

// CreateAgent создает нового NarrativeAgent
func (f *NarrativeAgentFactory) CreateAgent(ctx context.Context, blueprint *agent.AgentBlueprint, context *agent.AgentContext) (agent.Agent, error) {
	if context == nil {
		return nil, fmt.Errorf("agent context is required")
	}
	if context.ScopeID == "" {
		return nil, fmt.Errorf("scope_id is required in agent context")
	}

	agentLogger := log.New(f.logger.Writer(), fmt.Sprintf("[NarrativeAgent %s] ", context.ScopeID), log.LstdFlags|log.Lshortfile)

	// Создаем NarrativeAgent
	agent := &NarrativeAgent{
		id:            context.AgentID,
		typeName:      blueprint.Name,
		level:         context.Level,
		state:         agent.LifecycleRunning,
		context:       context,
		pipeline:      f.pipeline,
		bus:           f.bus,
		worldID:       "", // Будет установлено из события
		scopeID:       context.ScopeID,
		scopeType:     "", // Будет установлено из блупринта
		focusEntities: []string{},
		history:       []HistoryEntry{},
		stateData:     make(map[string]interface{}),
		config:        make(map[string]interface{}),
		logger:        agentLogger,
	}

	// Устанавливаем blueprint в контекст
	context.Blueprint = blueprint

	return agent, nil
}

// CreateGMAgent создает NarrativeAgent из GMInstance
// Это мост между старой GMInstance архитектурой и новой Agent GM Core
func CreateGMAgent(
	gmScopeID string,
	gmScopeType string,
	gmWorldID string,
	focusEntities []string,
	config map[string]interface{},
	pipeline *agent.TwoPhasePipeline,
	bus *eventbus.EventBus,
	logger *log.Logger,
) *NarrativeAgent {
	agentLogger := log.New(logger.Writer(), fmt.Sprintf("[NarrativeAgent %s] ", gmScopeID), log.LstdFlags|log.Lshortfile)

	return &NarrativeAgent{
		id:            gmScopeID,
		typeName:      "narrative-gm-" + gmScopeType,
		level:         agent.LevelDomain,
		state:         agent.LifecycleRunning,
		context: &agent.AgentContext{
			AgentID:   gmScopeID,
			AgentType: "narrative-gm-" + gmScopeType,
			Level:     agent.LevelDomain,
			ScopeID:   gmScopeID,
			Entities:  toEntityRefs(focusEntities),
			CreatedAt: time.Now().Format(time.RFC3339),
		},
		pipeline:      pipeline,
		bus:           bus,
		worldID:       gmWorldID,
		scopeID:       gmScopeID,
		scopeType:     gmScopeType,
		focusEntities: focusEntities,
		history:       []HistoryEntry{},
		stateData:     make(map[string]interface{}),
		config:        config,
		logger:        agentLogger,
	}
}
