// services/entityactor/api/handlers.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"multiverse-core/internal/intent"
	"multiverse-core/internal/rules"
	"multiverse-core/internal/tinyml"
)

// Service HTTP сервис для EntityActor API
type Service struct {
	ModelLoader  *tinyml.ModelLoader
	RuleEngine   *rules.Engine
	IntentCache  *intent.IntentCache
	OracleClient *intent.OracleClient
	startTime    time.Time
}

// NewService создает новый API сервис
func NewService(
	modelLoader *tinyml.ModelLoader,
	ruleEngine *rules.Engine,
	intentCache *intent.IntentCache,
	oracleClient *intent.OracleClient,
) *Service {
	return &Service{
		ModelLoader:  modelLoader,
		RuleEngine:   ruleEngine,
		IntentCache:  intentCache,
		OracleClient: oracleClient,
		startTime:    time.Now(),
	}
}

// HandleEntityCreation обрабатывает создание новой сущности
func (s *Service) HandleEntityCreation(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			s.handleError(w, rec, "HandleEntityCreation")
		}
	}()

	var req EntityActorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", "INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.EntityID == "" {
		s.sendError(w, "entity_id is required", "MISSING_ENTITY_ID", http.StatusBadRequest)
		return
	}

	if req.EntityType == "" {
		s.sendError(w, "entity_type is required", "MISSING_ENTITY_TYPE", http.StatusBadRequest)
		return
	}

	// Создаем модель для сущности
	model, err := tinyml.NewTinyModel(tinyml.ModelConfig{
		Version:      "v1.0",
		InputSize:    10,
		OutputSize:   5,
		HiddenLayers: []int{8},
		ActivationFn: "relu",
	})
	if err != nil {
		s.sendError(w, "Failed to create model", "MODEL_CREATION_FAILED", http.StatusInternalServerError)
		return
	}

	// Сохраняем модель
	modelID := req.EntityID
	if err := s.ModelLoader.SaveToStorage(modelID, model); err != nil {
		s.sendError(w, "Failed to save model", "MODEL_SAVE_FAILED", http.StatusInternalServerError)
		return
	}

	resp := EntityActorResponse{
		EntityID:     req.EntityID,
		ActorID:      req.EntityID,
		State:        req.State,
		Success:      true,
		Message:      "Entity actor created successfully",
		Timestamp:    time.Now(),
		ProcessingMs: time.Since(startTime).Milliseconds(),
	}

	s.sendJSON(w, resp, http.StatusCreated)
}

// HandleEntityUpdate обрабатывает обновление сущности
func (s *Service) HandleEntityUpdate(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			s.handleError(w, rec, "HandleEntityUpdate")
		}
	}()

	var req EntityActorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", "INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	// Загружаем модель
	model, err := s.ModelLoader.LoadFromStorage(req.EntityID)
	if err != nil {
		s.sendError(w, "Entity not found", "ENTITY_NOT_FOUND", http.StatusNotFound)
		return
	}

	// Выполняем inference
	features := make([]float32, 0, len(req.State))
	for _, v := range req.State {
		features = append(features, v)
	}

	modelOutput, err := model.Run(features)
	if err != nil {
		s.sendError(w, "Model inference failed", "INFERENCE_FAILED", http.StatusInternalServerError)
		return
	}

	// Обновляем состояние
	updatedState := make(map[string]float32)
	for k, v := range req.State {
		updatedState[k] = v
	}
	for k, v := range modelOutput {
		updatedState[k] = v
	}

	resp := EntityActorResponse{
		EntityID:     req.EntityID,
		State:        updatedState,
		Success:      true,
		Message:      "Entity updated successfully",
		Timestamp:    time.Now(),
		ProcessingMs: time.Since(startTime).Milliseconds(),
	}

	s.sendJSON(w, resp, http.StatusOK)
}

// HandleIntentRecognition обрабатывает распознавание намерения
func (s *Service) HandleIntentRecognition(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			s.handleError(w, rec, "HandleIntentRecognition")
		}
	}()

	var req IntentRecognitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", "INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	if req.PlayerText == "" {
		s.sendError(w, "player_text is required", "MISSING_PLAYER_TEXT", http.StatusBadRequest)
		return
	}

	// Проверяем кэш
	cacheHit := false
	var intentResp *IntentRecognitionResponse

	if cached, found := s.IntentCache.Get(req.PlayerText, req.EntityID, req.WorldContext); found {
		intentResp = &IntentRecognitionResponse{
			Intent:        cached.Intent,
			Confidence:    cached.Confidence,
			BaseAction:    cached.BaseAction,
			TargetEntity:  cached.TargetEntity,
			Parameters:    cached.Parameters,
			RequiresRoll:  cached.RequiresRoll,
			SuggestedRule: cached.SuggestedRule,
			Reasoning:     cached.Reasoning,
			CacheHit:      true,
			ProcessingMs:  time.Since(startTime).Milliseconds(),
		}
		cacheHit = true
	}

	// Если нет в кэше - вызываем Oracle
	if !cacheHit {
		oracleReq := intent.IntentRequest{
			PlayerText:   req.PlayerText,
			EntityID:     req.EntityID,
			EntityType:   req.EntityType,
			WorldContext: req.WorldContext,
			State:        req.State,
			History:      req.History,
			Metadata:     req.Metadata,
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		recognized, err := s.OracleClient.RecognizeIntent(ctx, oracleReq)
		if err != nil {
			s.sendError(w, "Intent recognition failed", "ORACLE_ERROR", http.StatusInternalServerError)
			return
		}

		intentResp = &IntentRecognitionResponse{
			Intent:        recognized.Intent,
			Confidence:    recognized.Confidence,
			BaseAction:    recognized.BaseAction,
			Modifiers:     s.convertModifiers(recognized.Modifiers),
			TargetEntity:  recognized.TargetEntity,
			Parameters:    recognized.Parameters,
			RequiresRoll:  recognized.RequiresRoll,
			SuggestedRule: recognized.SuggestedRule,
			Reasoning:     recognized.Reasoning,
			CacheHit:      false,
			ProcessingMs:  time.Since(startTime).Milliseconds(),
		}

		// Кэшируем результат
		s.IntentCache.Put(req.PlayerText, req.EntityID, req.WorldContext, &intent.IntentResponse{
			Intent:        recognized.Intent,
			Confidence:    recognized.Confidence,
			BaseAction:    recognized.BaseAction,
			TargetEntity:  recognized.TargetEntity,
			Parameters:    recognized.Parameters,
			RequiresRoll:  recognized.RequiresRoll,
			SuggestedRule: recognized.SuggestedRule,
			Reasoning:     recognized.Reasoning,
		})
	}

	s.sendJSON(w, intentResp, http.StatusOK)
}

// HandleRuleApplication обрабатывает применение правила
func (s *Service) HandleRuleApplication(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			s.handleError(w, rec, "HandleRuleApplication")
		}
	}()

	var req RuleApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", "INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	if req.RuleID == "" {
		s.sendError(w, "rule_id is required", "MISSING_RULE_ID", http.StatusBadRequest)
		return
	}

	// Применяем правило
	result, err := s.RuleEngine.Apply(req.RuleID, req.State, req.Modifiers)
	if err != nil {
		s.sendError(w, "Rule application failed: "+err.Error(), "RULE_APPLICATION_FAILED", http.StatusInternalServerError)
		return
	}

	resp := RuleApplicationResponse{
		RuleID:         result.RuleID,
		RuleVersion:    result.RuleVersion,
		DiceRoll:       result.DiceRoll,
		DiceFormula:    result.DiceFormula,
		Total:          result.Total,
		Success:        result.Success,
		Critical:       result.Critical,
		Modifiers:      s.convertAppliedModifiers(result.Modifiers),
		SensoryEffects: result.SensoryEffects,
		StateChanges:   s.convertStateChanges(result.StateChanges),
		AppliedAt:      result.AppliedAt,
		ProcessingMs:   time.Since(startTime).Milliseconds(),
	}

	s.sendJSON(w, resp, http.StatusOK)
}

// HandleGetActorState обрабатывает запрос состояния актора
func (s *Service) HandleGetActorState(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			s.handleError(w, rec, "HandleGetActorState")
		}
	}()

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		s.sendError(w, "entity_id query parameter is required", "MISSING_ENTITY_ID", http.StatusBadRequest)
		return
	}

	// Загружаем модель
	model, err := s.ModelLoader.LoadFromStorage(entityID)
	if err != nil {
		s.sendError(w, "Entity not found", "ENTITY_NOT_FOUND", http.StatusNotFound)
		return
	}

	stats := model.GetStats()

	resp := ActorStateResponse{
		ActorID:      entityID,
		EntityID:     entityID,
		ModelVersion: stats.Version,
		LastUpdated:  stats.LastUsed,
	}

	s.sendJSON(w, resp, http.StatusOK)
}

// HandleHealth health check endpoint
func (s *Service) HandleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Components: map[string]string{
			"model_loader":  "ok",
			"rule_engine":   "ok",
			"intent_cache":  "ok",
			"oracle_client": "ok",
		},
		Version: "1.0.0",
		Uptime:  time.Since(s.startTime).String(),
	}

	s.sendJSON(w, resp, http.StatusOK)
}

// HandleGetStats возвращает статистику сервиса
func (s *Service) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	cacheStats := s.IntentCache.GetStats()
	ruleStats := s.RuleEngine.GetCacheStats()
	modelStats := s.ModelLoader.GetCacheStats()

	resp := map[string]interface{}{
		"intent_cache":  cacheStats,
		"rule_engine":   ruleStats,
		"uptime":        time.Since(s.startTime).String(),
		"models_loaded": modelStats.CachedModels,
	}

	s.sendJSON(w, resp, http.StatusOK)
}

// Вспомогательные функции

func (s *Service) convertModifiers(mods []intent.IntentModifier) []IntentModifier {
	result := make([]IntentModifier, len(mods))
	for i, mod := range mods {
		result[i] = IntentModifier{
			Type:  mod.Type,
			Value: mod.Value,
		}
	}
	return result
}

func (s *Service) convertAppliedModifiers(mods []rules.AppliedModifier) []AppliedModifier {
	result := make([]AppliedModifier, len(mods))
	for i, mod := range mods {
		result[i] = AppliedModifier{
			ID:         mod.ID,
			Name:       mod.Name,
			Value:      mod.Value,
			Condition:  mod.Condition,
			WasApplied: mod.WasApplied,
		}
	}
	return result
}

func (s *Service) convertStateChanges(changes []rules.StateChange) []StateChange {
	result := make([]StateChange, len(changes))
	for i, change := range changes {
		result[i] = StateChange{
			Path:      change.Path,
			Operation: change.Operation,
			Value:     change.Value,
			Duration:  change.Duration,
		}
	}
	return result
}

func (s *Service) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Service) sendError(w http.ResponseWriter, message, code string, status int) {
	resp := ErrorResponse{
		Error:     message,
		Code:      code,
		Timestamp: time.Now(),
	}
	s.sendJSON(w, resp, status)
}

func (s *Service) handleError(w http.ResponseWriter, rec interface{}, handler string) {
	fmt.Printf("Panic in %s: %v\n%s", handler, rec, string(debug.Stack()))
	s.sendError(w, "Internal server error", "INTERNAL_ERROR", http.StatusInternalServerError)
}
