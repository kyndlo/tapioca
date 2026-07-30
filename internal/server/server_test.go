package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/carlos/tapioca/internal/config"
)

func fakeUpstream(t *testing.T, inspect func(map[string]any)) (*httptest.Server, int) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if inspect != nil {
			inspect(request)
		}
		writeJSON(w, 200, map[string]any{
			"id": "chatcmpl_test", "object": "chat.completion", "created": 1, "model": "test",
			"choices": []any{map[string]any{
				"index": 0, "finish_reason": "tool_calls",
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []any{map[string]any{
						"id": "call_1", "type": "function",
						"function": map[string]any{"name": "read_file", "arguments": `{"path":"README.md"}`},
					}},
				},
			}},
		})
	}))
	parsed, _ := url.Parse(upstream.URL)
	_, portText, _ := strings.Cut(parsed.Host, ":")
	port, _ := strconv.Atoi(portText)
	return upstream, port
}

func TestResponsesConvertsTools(t *testing.T) {
	upstream, port := fakeUpstream(t, func(request map[string]any) {
		tools := request["tools"].([]any)
		tool := tools[0].(map[string]any)
		if _, ok := tool["function"]; !ok {
			t.Fatal("Responses tool was not converted to Chat Completions format")
		}
	})
	defer upstream.Close()
	s := New(Options{Model: config.Model{Name: "glm"}, UpstreamPort: port})
	body := `{"model":"glm","input":"read it","tools":[{"type":"function","name":"read_file","description":"read","parameters":{"type":"object"}}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &response)
	output := response["output"].([]any)
	found := false
	for _, raw := range output {
		if raw.(map[string]any)["type"] == "function_call" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestAnthropicConvertsToolResults(t *testing.T) {
	upstream, port := fakeUpstream(t, func(request map[string]any) {
		messages := request["messages"].([]any)
		found := false
		for _, raw := range messages {
			message := raw.(map[string]any)
			if message["role"] == "tool" && message["tool_call_id"] == "call_old" {
				found = true
			}
		}
		if !found {
			t.Fatalf("tool result missing from converted messages: %#v", messages)
		}
	})
	defer upstream.Close()
	s := New(Options{Model: config.Model{Name: "glm"}, UpstreamPort: port})
	body := `{"model":"glm","max_tokens":100,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_old","content":"file data"}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	responseBody, _ := io.ReadAll(recorder.Result().Body)
	if !strings.Contains(string(responseBody), `"type":"tool_use"`) {
		t.Fatalf("unexpected response: %s", responseBody)
	}
}

func TestModels(t *testing.T) {
	s := New(Options{Model: config.Model{Name: "glm"}})
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"id":"glm"`) {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
}

func ExampleServer_Handler() {
	s := New(Options{Model: config.Model{Name: "glm-4.7-flash"}})
	_ = s.Handler()
	fmt.Println("ready")
	// Output: ready
}
