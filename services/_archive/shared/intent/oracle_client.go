// internal/intent/oracle_client.go
package intent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OracleClient клиент для взаимодействия с Oracle (Qwen3)
type OracleClient struct {
	httpClient *http.Client
	baseURL    string
	model      string
	maxRetries int
	timeout    time.Duration
}

// OracleConfig конфигурация Oracle клиента
type OracleConfig struct {
	BaseURL    string
	Model      string
	Timeout    time.Duration
	MaxRetries int
}

// NewOracleClient создает нового Oracle клиента
func NewOracleClient(config OracleConfig) *OracleClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.Model == "" {
		config.Model = "qwen3"
	}

	return &OracleClient{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL:    config.BaseURL,
		model:      config.Model,
		maxRetries: config.MaxRetries,
		timeout:    config.Timeout,
	}
}

// IntentRequest запрос на распознавание намерения
type IntentRequest struct {
	PlayerText   string                 `json:"player_text"`
	EntityID     string                 `json:"entity_id"`
	EntityType   string                 `json:"entity_type"`
	WorldContext string                 `json:"world_context"`
	State        map[string]float32     `json:"state,omitempty"`
	History      []string               `json:"history,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// IntentResponse ответ с распознанным намерением
type IntentResponse struct {
	Intent        string                 `json:"intent"`
	Confidence    float32                `json:"confidence"`
	BaseAction    string                 `json:"base_action"`
	Modifiers     []IntentModifier       `json:"modifiers,omitempty"`
	TargetEntity  string                 `json:"target_entity,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	RequiresRoll  bool                   `json:"requires_roll"`
	SuggestedRule string                 `json:"suggested_rule,omitempty"`
	Reasoning     string                 `json:"reasoning"`
}

// IntentModifier модификатор намерения
type IntentModifier struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// RecognizeIntent распознает намерение из текста игрока
func (c *OracleClient) RecognizeIntent(ctx context.Context, req IntentRequest) (*IntentResponse, error) {
	prompt := c.buildIntentPrompt(req)

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		response, err := c.callOracle(ctx, prompt)
		if err == nil {
			return response, nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	return nil, fmt.Errorf("failed after %d retries: %w", c.maxRetries, lastErr)
}

// buildIntentPrompt строит промпт для распознавания намерения
func (c *OracleClient) buildIntentPrompt(req IntentRequest) string {
	prompt := `Ты - система распознавания намерений в RPG игре. Преобразуй текст игрока в структурированное действие.

## Контекст
- Сущность: ` + req.EntityID + ` (` + req.EntityType + `)
- Мир: ` + req.WorldContext + `

## Текст игрока
"` + req.PlayerText + `"

## Текущее состояние
` + fmt.Sprintf("%v", req.State) + `

## Задача
Распознай намерение и верни JSON в формате:
{
  "intent": "attack|talk|move|use_item|cast_spell|examine|craft|other",
  "confidence": 0.0-1.0,
  "base_action": "базовое действие",
  "modifiers": [{"type": "...", "value": "..."}],
  "target_entity": "цель (если есть)",
  "parameters": {...},
  "requires_roll": true/false,
  "suggested_rule": "правило (если нужен бросок)",
  "reasoning": "краткое обоснование"
}

## Примеры
1. "Атакую гоблина мечом" -> {"intent": "attack", "base_action": "melee_attack", "target_entity": "goblin", "requires_roll": true, "suggested_rule": "melee_attack"}
2. "Хочу поговорить с торговцем" -> {"intent": "talk", "base_action": "dialogue", "target_entity": "merchant"}
3. "Осматриваю сундук" -> {"intent": "examine", "base_action": "inspect", "target_entity": "chest"}

Верни ТОЛЬКО JSON без дополнительного текста.`

	return prompt
}

// callOracle вызывает Oracle API
func (c *OracleClient) callOracle(ctx context.Context, prompt string) (*IntentResponse, error) {
	requestBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Ты - система распознавания намерений в RPG игре. Отвечай ТОЛЬКО валидным JSON.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.1, // Низкая температура для детерминированных ответов
		"max_tokens":  500,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oracle returned status %d: %s", resp.StatusCode, string(body))
	}

	var oracleResp OracleAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oracleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(oracleResp.Choices) == 0 {
		return nil, fmt.Errorf("oracle returned no choices")
	}

	// Парсим ответ Oracle как IntentResponse
	content := oracleResp.Choices[0].Message.Content
	var intent IntentResponse
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse intent JSON: %w", err)
	}

	return &intent, nil
}

// OracleAPIResponse ответ от Oracle API
type OracleAPIResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// FilterContent фильтрует токсичный контент
func (c *OracleClient) FilterContent(text string) (bool, []string) {
	// Простая фильтрация запрещенных тем
	blockedTopics := []string{
		"violence_excessive",
		"discrimination",
		"hate_speech",
		"self_harm",
	}

	var detected []string
	for _, topic := range blockedTopics {
		// В production здесь будет ML классификатор
		// Для примера - простая проверка
		if containsBlocked(text, topic) {
			detected = append(detected, topic)
		}
	}

	return len(detected) > 0, detected
}

func containsBlocked(text, topic string) bool {
	// Заглушка для реальной фильтрации
	blockedWords := map[string][]string{
		"violence_excessive": {"убить", "убийство", "кровь"},
		"hate_speech":        {"ненависть", "дискриминация"},
	}

	if words, exists := blockedWords[topic]; exists {
		for _, word := range words {
			if contains(text, word) {
				return true
			}
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
