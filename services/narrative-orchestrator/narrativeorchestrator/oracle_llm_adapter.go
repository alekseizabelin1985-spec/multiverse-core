// services/narrativeorchestrator/oracle_llm_adapter.go

package narrativeorchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/oracle"
	"net/http"
)

// OracleLLMAdapter адаптирует oracle.Client под agent.LLMClient
type OracleLLMAdapter struct {
	client *oracle.Client
}

// NewOracleLLMAdapter создает адаптер
func NewOracleLLMAdapter() *OracleLLMAdapter {
	return &OracleLLMAdapter{
		client: oracle.NewClient(),
	}
}

// Generate реализует agent.LLMClient
func (a *OracleLLMAdapter) Generate(ctx context.Context, model string, prompt string, config agent.GenerateConfig) (*agent.LLMResult, error) {
	var content string
	var err error

	if config.StrictJSON {
		// Phase 1: строгий JSON — используем CallStructuredJSON
		content, err = a.client.CallStructuredJSON(ctx, "", prompt)
	} else {
		// Phase 2: креативный текст — кастомный вызов с температурой
		content, err = a.callCreative(ctx, prompt, config.Temperature, config.MaxTokens)
	}

	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	return &agent.LLMResult{
		Content: content,
	}, nil
}

// callCreative вызывает LLM для креативного текста с кастомной температурой
func (a *OracleLLMAdapter) callCreative(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	if temperature <= 0 {
		temperature = 0.7
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"model":     a.client.Model,
		"messages":  []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
		"temperature": temperature,
		"max_tokens":  maxTokens,
		"response_format": map[string]string{
			"type": "text",
		},
		"min_p":              0.05,
		"top_p":              0.95,
		"presence_penalty":   1.5,
		"repetition_penalty": 1.0,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal creative request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.client.BaseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.client.API_KEY != "" {
		req.Header.Set("Authorization", "Bearer "+a.client.API_KEY)
	}

	resp, err := a.client.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("oracle returned status %d: %s", resp.StatusCode, string(body))
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var responseStruct struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &responseStruct); err != nil {
		return "", fmt.Errorf("failed to unmarshal oracle response: %w", err)
	}

	if len(responseStruct.Choices) == 0 {
		return "", fmt.Errorf("oracle returned no choices")
	}

	return responseStruct.Choices[0].Message.Content, nil
}
