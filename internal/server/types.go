package server

import "encoding/json"

type ChatMessage struct {
	Role             string          `json:"role"`
	Content          any             `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
}

type ChatRequest struct {
	Model           string          `json:"model"`
	Messages        []ChatMessage   `json:"messages"`
	Tools           json.RawMessage `json:"tools,omitempty"`
	ToolChoice      any             `json:"tool_choice,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	MaxTokens       int             `json:"max_tokens,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
	ReasoningFormat string          `json:"reasoning_format,omitempty"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int         `json:"index"`
		Message ChatMessage `json:"message"`
		Reason  string      `json:"finish_reason"`
	} `json:"choices"`
	// llama-server includes nested timing and token-detail objects in usage.
	Usage map[string]any `json:"usage,omitempty"`
}
