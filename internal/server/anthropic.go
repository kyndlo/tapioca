package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type anthropicRequest struct {
	Model       string          `json:"model"`
	System      any             `json:"system,omitempty"`
	Messages    []ChatMessage   `json:"messages"`
	Tools       json.RawMessage `json:"tools,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	var in anthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	messages := anthropicMessages(in.Messages)
	if in.System != nil {
		messages = append([]ChatMessage{{Role: "system", Content: textContent(in.System)}}, messages...)
	}
	tools := anthropicTools(in.Tools)
	result, err := s.complete(r.Context(), ChatRequest{
		Messages: messages, Tools: tools, Temperature: in.Temperature, MaxTokens: in.MaxTokens,
	})
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	response := anthropicResponse(s.opts.Model.Name, result)
	if in.Stream {
		streamAnthropic(w, response)
		return
	}
	writeJSON(w, 200, response)
}

func anthropicMessages(input []ChatMessage) []ChatMessage {
	var output []ChatMessage
	for _, message := range input {
		blocks, ok := message.Content.([]any)
		if !ok {
			output = append(output, message)
			continue
		}
		var text string
		var calls []any
		for _, raw := range blocks {
			block, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			switch block["type"] {
			case "text":
				text += fmt.Sprint(block["text"])
			case "tool_use":
				args, _ := json.Marshal(block["input"])
				calls = append(calls, map[string]any{
					"id": block["id"], "type": "function",
					"function": map[string]any{"name": block["name"], "arguments": string(args)},
				})
			case "tool_result":
				output = append(output, ChatMessage{
					Role: "tool", ToolCallID: fmt.Sprint(block["tool_use_id"]), Content: textContent(block["content"]),
				})
			}
		}
		if text != "" || len(calls) > 0 {
			converted := ChatMessage{Role: message.Role, Content: text}
			if len(calls) > 0 {
				converted.ToolCalls, _ = json.Marshal(calls)
			}
			output = append(output, converted)
		}
	}
	return output
}

func anthropicTools(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if json.Unmarshal(raw, &tools) != nil {
		return raw
	}
	var converted []any
	for _, tool := range tools {
		converted = append(converted, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": tool["name"], "description": tool["description"], "parameters": tool["input_schema"],
			},
		})
	}
	out, _ := json.Marshal(converted)
	return out
}

func anthropicResponse(model string, chat ChatResponse) map[string]any {
	var content []any
	stop := "end_turn"
	if len(chat.Choices) > 0 {
		message := chat.Choices[0].Message
		if text := textContent(message.Content); text != "" {
			content = append(content, map[string]any{"type": "text", "text": text})
		}
		if len(message.ToolCalls) > 0 {
			var calls []map[string]any
			if json.Unmarshal(message.ToolCalls, &calls) == nil {
				for _, call := range calls {
					fn, _ := call["function"].(map[string]any)
					var args any
					_ = json.Unmarshal([]byte(fmt.Sprint(fn["arguments"])), &args)
					content = append(content, map[string]any{
						"type": "tool_use", "id": call["id"], "name": fn["name"], "input": args,
					})
				}
				stop = "tool_use"
			}
		}
	}
	return map[string]any{
		"id": "msg_" + fmt.Sprint(time.Now().UnixNano()), "type": "message", "role": "assistant",
		"model": model, "content": content, "stop_reason": stop, "stop_sequence": nil,
		"usage": map[string]int{"input_tokens": 0, "output_tokens": 0},
	}
}

func streamAnthropic(w http.ResponseWriter, response map[string]any) {
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
	base := map[string]any{}
	for k, v := range response {
		base[k] = v
	}
	base["content"] = []any{}
	base["stop_reason"] = nil
	send("message_start", map[string]any{"type": "message_start", "message": base})
	for i, block := range response["content"].([]any) {
		blockMap := block.(map[string]any)
		startBlock := block
		if blockMap["type"] == "text" {
			startBlock = map[string]any{"type": "text", "text": ""}
		} else if blockMap["type"] == "tool_use" {
			startBlock = map[string]any{"type": "tool_use", "id": blockMap["id"], "name": blockMap["name"], "input": map[string]any{}}
		}
		send("content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": startBlock})
		if blockMap["type"] == "text" {
			send("content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": map[string]any{"type": "text_delta", "text": blockMap["text"]}})
		} else if blockMap["type"] == "tool_use" {
			input, _ := json.Marshal(blockMap["input"])
			send("content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(input)}})
		}
		send("content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
	}
	send("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": response["stop_reason"], "stop_sequence": nil}, "usage": map[string]int{"output_tokens": 0}})
	send("message_stop", map[string]any{"type": "message_stop"})
}
