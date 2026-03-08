// services/entityactor/api/types.go
package api

import (
	"time"

	"multiverse-core/internal/rules"
	"multiverse-core/internal/tinyml"
)

// EntityActorRequest запрос на создание/обновление сущности
type EntityActorRequest struct {
	EntityID   string                 `json:"entity_id"`
	EntityType string                 `json:"entity_type"`
	WorldID    string                 `json:"world_id"`
	State      map[string]float32     `json:"state,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	PlayerText string                 `json:"player_text,omitempty"` // Для intent recognition
}

// EntityActorResponse ответ от EntityActor
type EntityActorResponse struct {
	EntityID     string             `json:"entity_id"`
	ActorID      string             `json:"actor_id"`
	State        map[string]float32 `json:"state"`
	Result       *rules.RuleResult  `json:"result,omitempty"`
	Intent       *IntentInfo        `json:"intent,omitempty"`
	Success      bool               `json:"success"`
	Message      string             `json:"message"`
	Timestamp    time.Time          `json:"timestamp"`
	ProcessingMs int64              `json:"processing_ms"`
}

// IntentInfo информация о распознанном намерении
type IntentInfo struct {
	Intent        string                 `json:"intent"`
	Confidence    float32                `json:"confidence"`
	BaseAction    string                 `json:"base_action"`
	TargetEntity  string                 `json:"target_entity,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	RequiresRoll  bool                   `json:"requires_roll"`
	SuggestedRule string                 `json:"suggested_rule,omitempty"`
	Reasoning     string                 `json:"reasoning"`
}

// ActorStateResponse ответ с состоянием актора
type ActorStateResponse struct {
	ActorID      string             `json:"actor_id"`
	EntityID     string             `json:"entity_id"`
	EntityType   string             `json:"entity_type"`
	WorldID      string             `json:"world_id"`
	State        map[string]float32 `json:"state"`
	ModelVersion string             `json:"model_version"`
	LastUpdated  time.Time          `json:"last_updated"`
	BufferSize   int                `json:"buffer_size"`
	LastProcess  time.Time          `json:"last_process"`
	LastSnapshot time.Time          `json:"last_snapshot"`
}

// ActorStatsResponse статистика актора
type ActorStatsResponse struct {
	ActorID       string    `json:"actor_id"`
	EntityID      string    `json:"entity_id"`
	StateSize     int       `json:"state_size"`
	BufferSize    int       `json:"buffer_size"`
	MaxBufferSize int       `json:"max_buffer_size"`
	LastProcess   time.Time `json:"last_process"`
	LastSnapshot  time.Time `json:"last_snapshot"`
	Uptime        string    `json:"uptime"`
	UptimeMs      int64     `json:"uptime_ms"`
}

// BatchRequest запрос на пакетную обработку
type BatchRequest struct {
	EntityIDs  []string               `json:"entity_ids"`
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// BatchResponse ответ на пакетный запрос
type BatchResponse struct {
	Results      []EntityActorResponse `json:"results"`
	SuccessCount int                   `json:"success_count"`
	FailedCount  int                   `json:"failed_count"`
	TotalMs      int64                 `json:"total_ms"`
}

// ModelInfo информация о модели
type ModelInfo struct {
	ModelID      string              `json:"model_id"`
	Version      string              `json:"version"`
	Stats        tinyml.ModelStats   `json:"stats"`
	Architecture tinyml.Architecture `json:"architecture"`
}

// RuleInfo информация о правиле
type RuleInfo struct {
	RuleID         string               `json:"rule_id"`
	Version        string               `json:"version"`
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	EntityType     string               `json:"entity_type"`
	MechanicalCore rules.MechanicalCore `json:"mechanical_core"`
	SemanticLayer  rules.SemanticLayer  `json:"semantic_layer"`
	BalanceScore   float32              `json:"balance_score"`
	SafetyLevel    string               `json:"safety_level"`
}

// HealthResponse ответ health check
type HealthResponse struct {
	Status     string            `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Components map[string]string `json:"components"`
	Version    string            `json:"version"`
	Uptime     string            `json:"uptime"`
}

// ErrorResponse ответ с ошибкой
type ErrorResponse struct {
	Error     string            `json:"error"`
	Code      string            `json:"code"`
	Details   map[string]string `json:"details,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// IntentRecognitionRequest запрос на распознавание намерения
type IntentRecognitionRequest struct {
	PlayerText   string                 `json:"player_text"`
	EntityID     string                 `json:"entity_id"`
	EntityType   string                 `json:"entity_type"`
	WorldContext string                 `json:"world_context"`
	State        map[string]float32     `json:"state,omitempty"`
	History      []string               `json:"history,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// IntentRecognitionResponse ответ распознавания намерения
type IntentRecognitionResponse struct {
	Intent        string                 `json:"intent"`
	Confidence    float32                `json:"confidence"`
	BaseAction    string                 `json:"base_action"`
	Modifiers     []IntentModifier       `json:"modifiers,omitempty"`
	TargetEntity  string                 `json:"target_entity,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	RequiresRoll  bool                   `json:"requires_roll"`
	SuggestedRule string                 `json:"suggested_rule,omitempty"`
	Reasoning     string                 `json:"reasoning"`
	CacheHit      bool                   `json:"cache_hit"`
	ProcessingMs  int64                  `json:"processing_ms"`
}

// IntentModifier модификатор намерения
type IntentModifier struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// RuleApplicationRequest запрос на применение правила
type RuleApplicationRequest struct {
	RuleID    string                   `json:"rule_id"`
	EntityID  string                   `json:"entity_id"`
	State     map[string]float32       `json:"state"`
	Modifiers []map[string]interface{} `json:"modifiers,omitempty"`
}

// RuleApplicationResponse ответ применения правила
type RuleApplicationResponse struct {
	RuleID         string             `json:"rule_id"`
	RuleVersion    string             `json:"rule_version"`
	DiceRoll       int                `json:"dice_roll"`
	DiceFormula    string             `json:"dice_formula"`
	Total          int                `json:"total"`
	Success        bool               `json:"success"`
	Critical       bool               `json:"critical"`
	Modifiers      []AppliedModifier  `json:"modifiers"`
	SensoryEffects map[string]float32 `json:"sensory_effects"`
	StateChanges   []StateChange      `json:"state_changes"`
	AppliedAt      time.Time          `json:"applied_at"`
	ProcessingMs   int64              `json:"processing_ms"`
}

// AppliedModifier примененный модификатор
type AppliedModifier struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Condition  string `json:"condition"`
	WasApplied bool   `json:"was_applied"`
}

// StateChange изменение состояния
type StateChange struct {
	Path      string  `json:"path"`
	Operation string  `json:"operation"`
	Value     float32 `json:"value"`
	Duration  int     `json:"duration"`
}

// EvolutionStatsResponse статистика EvolutionWatcher
type EvolutionStatsResponse struct {
	ShortTermCount  int        `json:"short_term_count"`
	ShortTermLimit  int        `json:"short_term_limit"`
	MediumTermCount int        `json:"medium_term_count"`
	MediumTermLimit int        `json:"medium_term_limit"`
	AnomalyModel    ModelStats `json:"anomaly_model"`
}

// ModelStats статистика модели аномалий
type ModelStats struct {
	ChecksCount     int64   `json:"checks_count"`
	AnomaliesFound  int64   `json:"anomalies_found"`
	AverageSeverity float32 `json:"average_severity"`
}
