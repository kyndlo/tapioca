package control

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/carlos/tapioca/internal/app"
	"github.com/carlos/tapioca/internal/config"
	modelserver "github.com/carlos/tapioca/internal/server"
)

func TestFeatureReadMethodsAndAgentDescriptors(t *testing.T) {
	t.Setenv("TAPIOCA_HOME", t.TempDir())
	handler := NewHandler(Dependencies{})

	for _, method := range []string{"system.info", "storage.info"} {
		result, err := handler.Handle(context.Background(), Request{Method: method})
		if err != nil {
			t.Fatalf("%s error = %v", method, err)
		}
		if result == nil {
			t.Fatalf("%s result is nil", method)
		}
	}

	result, err := handler.Handle(context.Background(), Request{
		Method: "catalog.get", Params: []byte(`{"name":"gemma3:12b-mlx"}`),
	})
	if err != nil {
		t.Fatalf("catalog.get error = %v", err)
	}
	if result.(CatalogModel).Name != "gemma3:12b-mlx" {
		t.Fatalf("catalog.get = %#v", result)
	}

	for _, agent := range []string{"codex", "claude-code", "opencode", "openclaw", "hermes"} {
		params, _ := json.Marshal(map[string]any{
			"agent": agent, "model": "gemma3:12b-mlx", "port": 11435,
		})
		result, err := handler.Handle(context.Background(), Request{
			Method: "agent.describe", Params: params,
		})
		if err != nil {
			t.Fatalf("agent.describe(%s) error = %v", agent, err)
		}
		descriptor := result.(AgentDescriptor)
		if descriptor.Executable == "" || descriptor.Endpoint == "" {
			t.Fatalf("agent.describe(%s) = %#v", agent, descriptor)
		}
	}
}

func TestLookPathAgentFindsInstalledExecutable(t *testing.T) {
	bin := t.TempDir()
	name := "codex"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(bin, name)
	if err := os.WriteFile(executable, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	resolved, err := lookPathAgent("codex")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != executable {
		t.Fatalf("lookPathAgent = %q, want %q", resolved, executable)
	}
}

func TestModelPullUsesTypedDependencyAndProgressEvents(t *testing.T) {
	pulled := make(chan string, 1)
	handler := NewHandler(Dependencies{
		Pull: func(
			_ context.Context,
			name string,
			_ bool,
			report app.PullReporter,
		) (config.Model, error) {
			pulled <- name
			report(app.PullProgress{Stage: "progress", Bytes: 5, TotalBytes: 10})
			return config.Model{Name: name, Path: "/models/test.gguf"}, nil
		},
	})
	input := strings.NewReader(
		`{"version":1,"type":"request","id":"pull-1","method":"model.pull","params":{"name":"gemma3:12b-mlx"},"job_id":"job-pull"}` + "\n",
	)
	var output bytes.Buffer
	if err := NewServer(input, &output, handler).Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if name := <-pulled; name != "gemma3:12b-mlx" {
		t.Fatalf("pulled name = %q", name)
	}
	if !strings.Contains(output.String(), `"event":"job.progress"`) {
		t.Fatalf("output has no progress event:\n%s", output.String())
	}
	assertJobEventSequences(t, output.String())
}

func TestMutatingPullJobsAreSerialized(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	handler := NewHandler(Dependencies{
		Pull: func(
			_ context.Context,
			name string,
			_ bool,
			_ app.PullReporter,
		) (config.Model, error) {
			current := active.Add(1)
			defer active.Add(-1)
			for {
				previous := maximum.Load()
				if current <= previous || maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			return config.Model{Name: name, Path: "/models/" + name}, nil
		},
	})
	input := strings.NewReader(
		`{"version":1,"type":"request","id":"one","method":"model.pull","params":{"name":"gemma3:12b-mlx"},"job_id":"job-one"}` + "\n" +
			`{"version":1,"type":"request","id":"two","method":"model.pull","params":{"name":"qwen3:8b-q4_k_m"},"job_id":"job-two"}` + "\n",
	)
	var output bytes.Buffer
	server := NewServerWithOptions(input, &output, handler, ServerOptions{MaxConcurrency: 2})
	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if maximum.Load() != 1 {
		t.Fatalf("maximum mutator concurrency = %d, want 1", maximum.Load())
	}
}

func TestJobCancelBypassesMutatorLock(t *testing.T) {
	started := make(chan struct{})
	handler := NewHandler(Dependencies{
		Pull: func(
			ctx context.Context,
			name string,
			_ bool,
			_ app.PullReporter,
		) (config.Model, error) {
			close(started)
			<-ctx.Done()
			return config.Model{}, ctx.Err()
		},
	})
	reader, writer := io.Pipe()
	var output bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- NewServer(reader, &output, handler).Run(context.Background())
	}()
	_, _ = writer.Write([]byte(
		`{"version":1,"type":"request","id":"pull","method":"model.pull","params":{"name":"gemma3:12b-mlx"},"job_id":"job-pull"}` + "\n",
	))
	<-started
	_, _ = writer.Write([]byte(
		`{"version":1,"type":"request","id":"cancel","method":"job.cancel","params":{"job_id":"job-pull"}}` + "\n",
	))
	_ = writer.Close()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), `"cancelled":true`) {
		t.Fatalf("cancel did not reach running mutator:\n%s", output.String())
	}
	if !strings.Contains(output.String(), `"code":"job_cancelled"`) {
		t.Fatalf("pull did not report cancellation:\n%s", output.String())
	}
}

func TestModelRemoveDryRunConfirmationAndRecoverableMove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	modelPath := filepath.Join(home, "models", "test-q4", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modelPath, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := config.Registry{Models: map[string]config.Model{
		"test:q4": {Name: "test:q4", Path: modelPath},
	}}
	if err := registry.Save(); err != nil {
		t.Fatal(err)
	}

	plan, protocolError := removeModel("test:q4", true, "", time.Now())
	if protocolError != nil {
		t.Fatalf("dry run error = %v", protocolError)
	}
	if plan.(RemovePlan).Removed {
		t.Fatal("dry run removed model")
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("model missing after dry run: %v", err)
	}

	if _, protocolError := removeModel("test:q4", false, "wrong", time.Now()); protocolError == nil ||
		protocolError.Code != "confirmation_required" {
		t.Fatalf("confirmation error = %#v", protocolError)
	}
	removed, protocolError := removeModel(
		"test:q4", false, "test:q4",
		time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
	)
	if protocolError != nil {
		t.Fatalf("remove error = %v", protocolError)
	}
	result := removed.(RemovePlan)
	if !result.Removed || result.TrashPath == "" {
		t.Fatalf("remove result = %#v", result)
	}
	if _, err := os.Stat(result.TrashPath); err != nil {
		t.Fatalf("recoverable trash path missing: %v", err)
	}
}

func TestChatRequestUsesTypedDependency(t *testing.T) {
	var backendResponse modelserver.ChatResponse
	if err := json.Unmarshal([]byte(`{
		"id":"chat-1","object":"chat.completion","created":1,"model":"test",
		"choices":[{"index":0,"message":{"role":"assistant","content":"final answer","reasoning_content":"bounded reasoning","tool_calls":[{"id":"call-1","type":"function","function":{"name":"weather","arguments":"{}"}}]},"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":10,"completion_tokens":20}
	}`), &backendResponse); err != nil {
		t.Fatal(err)
	}
	var received ChatParams
	handler := NewHandler(Dependencies{
		Chat: func(_ context.Context, params ChatParams) (modelserver.ChatResponse, error) {
			received = params
			return backendResponse, nil
		},
	})
	result, err := handler.Handle(context.Background(), Request{
		Method: "chat.request",
		Params: []byte(`{"port":11435,"model":"test","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}],"tool_choice":"auto","reasoning_format":"deepseek"}`),
	})
	if err != nil {
		t.Fatalf("chat.request error = %v", err)
	}
	response := result.(modelserver.ChatResponse)
	if response.ID != "chat-1" || len(response.Choices) != 1 ||
		response.Choices[0].Message.ReasoningContent != "bounded reasoning" ||
		!strings.Contains(string(response.Choices[0].Message.ToolCalls), `"weather"`) ||
		!strings.Contains(string(received.Tools), `"weather"`) ||
		received.ReasoningFormat != "deepseek" {
		t.Fatalf("chat.request result = %#v", result)
	}
}

func TestChatResponseRejectsMalformedOrOversizedToolCalls(t *testing.T) {
	var response modelserver.ChatResponse
	if err := json.Unmarshal([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok","tool_calls":{"not":"an array"}}}]}`), &response); err != nil {
		t.Fatal(err)
	}
	err := validateChatResponse(response)
	if err == nil || err.Code != "invalid_backend_response" {
		t.Fatalf("malformed backend response error = %#v", err)
	}

	if err := json.Unmarshal([]byte(`{"choices":[{"message":{"role":"assistant","content":"`+
		strings.Repeat("x", MaxChatMessageBytes+1)+`"}}]}`), &response); err != nil {
		t.Fatal(err)
	}
	err = validateChatResponse(response)
	if err == nil || err.Code != "invalid_backend_response" {
		t.Fatalf("oversized backend response error = %#v", err)
	}
}

func TestServerStartValidationIsSafe(t *testing.T) {
	manager := NewModelServerManager()
	if _, err := manager.Start(ServerStartParams{
		Model: "test", Host: "0.0.0.0", Port: 11435,
	}); err == nil || err.Code != "invalid_params" {
		t.Fatalf("non-loopback error = %#v", err)
	}
}

func TestEventPayloadIsBounded(t *testing.T) {
	data := boundEventData(map[string]any{"message": strings.Repeat("x", MaxEventDataBytes)})
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > MaxEventDataBytes || !strings.Contains(string(encoded), `"truncated":true`) {
		t.Fatalf("bounded event = %s", encoded)
	}
}

func assertJobEventSequences(t *testing.T, output string) {
	t.Helper()
	var previous uint64
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Type != "event" {
			continue
		}
		if event.Sequence <= previous {
			t.Fatalf("event sequence %d follows %d", event.Sequence, previous)
		}
		previous = event.Sequence
	}
}
