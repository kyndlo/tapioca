package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type responsesRequest struct {
	Model        string          `json:"model"`
	Input        any             `json:"input"`
	Instructions string          `json:"instructions,omitempty"`
	Tools        json.RawMessage `json:"tools,omitempty"`
	Temperature  *float64        `json:"temperature,omitempty"`
	MaxOutput    int             `json:"max_output_tokens,omitempty"`
	Stream       bool            `json:"stream,omitempty"`
}

func (s *Server) responses(w http.ResponseWriter, r *http.Request) {
	var in responsesRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	messages := responseInputMessages(in.Input)
	if in.Instructions != "" {
		messages = append([]ChatMessage{{Role: "system", Content: in.Instructions}}, messages...)
	}
	result, err := s.complete(r.Context(), ChatRequest{
		Messages: messages, Tools: responseTools(in.Tools), Temperature: in.Temperature, MaxTokens: in.MaxOutput,
	})
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	response := openAIResponse(s.opts.Model.Name, result)
	if in.Stream {
		streamResponses(w, response)
		return
	}
	writeJSON(w, 200, response)
}

func responseInputMessages(input any) []ChatMessage {
	if text, ok := input.(string); ok {
		return []ChatMessage{{Role: "user", Content: text}}
	}
	items, ok := input.([]any)
	if !ok {
		return []ChatMessage{{Role: "user", Content: textContent(input)}}
	}
	var messages []ChatMessage
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch item["type"] {
		case "function_call_output":
			messages = append(messages, ChatMessage{
				Role: "tool", ToolCallID: fmt.Sprint(item["call_id"]), Content: fmt.Sprint(item["output"]),
			})
		case "function_call":
			call, _ := json.Marshal([]any{map[string]any{
				"id": item["call_id"], "type": "function",
				"function": map[string]any{"name": item["name"], "arguments": item["arguments"]},
			}})
			messages = append(messages, ChatMessage{Role: "assistant", ToolCalls: call})
		default:
			role := fmt.Sprint(item["role"])
			if role == "" || role == "<nil>" {
				role = "user"
			}
			messages = append(messages, ChatMessage{Role: role, Content: textContent(item["content"])})
		}
	}
	return messages
}

func responseTools(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if json.Unmarshal(raw, &tools) != nil {
		return raw
	}
	var converted []any
	for _, tool := range tools {
		if tool["type"] != "function" {
			continue
		}
		converted = append(converted, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": tool["name"], "description": tool["description"], "parameters": tool["parameters"],
			},
		})
	}
	out, _ := json.Marshal(converted)
	return out
}

func openAIResponse(model string, chat ChatResponse) map[string]any {
	id := "resp_" + fmt.Sprint(time.Now().UnixNano())
	var output []any
	if len(chat.Choices) > 0 {
		message := chat.Choices[0].Message
		text := textContent(message.Content)
		if text != "" {
			output = append(output, map[string]any{
				"id": "msg_" + id, "type": "message", "status": "completed", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}},
			})
		}
		if len(message.ToolCalls) > 0 {
			var calls []map[string]any
			if json.Unmarshal(message.ToolCalls, &calls) == nil {
				for _, call := range calls {
					fn, _ := call["function"].(map[string]any)
					output = append(output, map[string]any{
						"id": call["id"], "call_id": call["id"], "type": "function_call",
						"name": fn["name"], "arguments": fn["arguments"], "status": "completed",
					})
				}
			}
		}
	}
	return map[string]any{
		"id": id, "object": "response", "created_at": time.Now().Unix(), "status": "completed",
		"model": model, "output": output, "parallel_tool_calls": true,
		"usage": map[string]any{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0},
	}
}

func streamResponses(w http.ResponseWriter, response map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	send := func(event string, data any) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	send("response.created", map[string]any{"type": "response.created", "response": response})
	for i, raw := range response["output"].([]any) {
		item := raw.(map[string]any)
		send("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": i, "item": item})
		if item["type"] == "message" {
			content := item["content"].([]any)[0].(map[string]any)
			text := fmt.Sprint(content["text"])
			send("response.content_part.added", map[string]any{"type": "response.content_part.added", "output_index": i, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
			send("response.output_text.delta", map[string]any{"type": "response.output_text.delta", "output_index": i, "content_index": 0, "delta": text})
			send("response.output_text.done", map[string]any{"type": "response.output_text.done", "output_index": i, "content_index": 0, "text": text})
		} else if strings.Contains(fmt.Sprint(item["type"]), "function_call") {
			send("response.function_call_arguments.done", map[string]any{"type": "response.function_call_arguments.done", "output_index": i, "arguments": item["arguments"]})
		}
		send("response.output_item.done", map[string]any{"type": "response.output_item.done", "output_index": i, "item": item})
	}
	send("response.completed", map[string]any{"type": "response.completed", "response": response})
}
