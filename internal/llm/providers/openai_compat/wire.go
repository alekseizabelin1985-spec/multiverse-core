package openaicompat

import "encoding/json"

// The JSON of the OpenAI chat completions API as llama-server speaks it. Only
// the fields the provider sends or reads are here.

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest has no omitempty on the sampling: a field left out is decided by
// the server, and the server defaults of llama-server are the thinking profile.
// max_tokens is the exception — zero means "no limit set", and a literal 0
// would ask for an empty answer.
type chatRequest struct {
	Model              string             `json:"model"`
	Messages           []wireMessage      `json:"messages"`
	Stream             bool               `json:"stream"`
	ResponseFormat     *responseFormat    `json:"response_format,omitempty"`
	ChatTemplateKwargs chatTemplateKwargs `json:"chat_template_kwargs"`
	Temperature        float64            `json:"temperature"`
	TopP               float64            `json:"top_p"`
	TopK               int                `json:"top_k"`
	MinP               float64            `json:"min_p"`
	PresencePenalty    float64            `json:"presence_penalty"`
	MaxTokens          int                `json:"max_tokens,omitempty"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
}

type chatTemplateKwargs struct {
	EnableThinking bool `json:"enable_thinking"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          *string `json:"content"`
			ReasoningContent *string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Usage   *usage   `json:"usage"`
	Timings *timings `json:"timings"`
}

type usage struct {
	PromptTokens        *int `json:"prompt_tokens"`
	CompletionTokens    *int `json:"completion_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

// timings is the extension of llama-server; a vendor does not send it.
type timings struct {
	PromptMS    *float64 `json:"prompt_ms"`
	PredictedMS *float64 `json:"predicted_ms"`
}

type embeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
}

type embeddingResponse struct {
	Data []struct {
		Index     *int      `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

type modelList struct {
	Data *[]struct {
		ID string `json:"id"`
	} `json:"data"`
}
