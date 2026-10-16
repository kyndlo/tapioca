package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/carlos/tapioca/internal/config"
)

func fixtureURL(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/vision/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}
func imagePart(url string) map[string]any {
	return map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}}
}
func visionPayload(t *testing.T) map[string]any {
	return map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "describe"}, imagePart(fixtureURL(t, "shapes.png")), imagePart(fixtureURL(t, "ocr.png"))}}}}
}
func chatRecorder(t *testing.T, s *Server, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)))
	return w
}

func TestVisionForwardsOrderedImagesAndReasoning(t *testing.T) {
	payload := visionPayload(t)
	upstream, port := fakeUpstream(t, func(p map[string]any) {
		if p["max_tokens"] != float64(512) {
			t.Errorf("unbounded default: %v", p["max_tokens"])
		}
		kwargs := p["chat_template_kwargs"].(map[string]any)
		expected := false
		if value, ok := payload["chat_template_kwargs"].(map[string]any); ok {
			expected, _ = value["enable_thinking"].(bool)
		}
		if kwargs["enable_thinking"] != expected {
			t.Errorf("thinking changed: %v", kwargs)
		}
		got, _ := json.Marshal(p["messages"])
		want, _ := json.Marshal(payload["messages"])
		if !bytes.Equal(got, want) {
			t.Error("image contents/history/order changed")
		}
	})
	defer upstream.Close()
	s := New(Options{Model: config.Model{Backend: "llama-vision"}, UpstreamPort: port})
	for _, thinking := range []any{nil, map[string]any{}, map[string]any{"enable_thinking": true}} {
		if thinking != nil {
			payload["chat_template_kwargs"] = thinking
		}
		if w := chatRecorder(t, s, payload); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
}

func TestVisionInputRejections(t *testing.T) {
	var oversized bytes.Buffer
	if err := png.Encode(&oversized, image.NewRGBA(image.Rect(0, 0, 1025, 1))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		part any
	}{
		{"remote", imagePart("https://example.com/image.png")},
		{"file", imagePart("file:///tmp/image.png")},
		{"base64", imagePart("data:image/png;base64,!!!")},
		{"truncated", imagePart("data:image/png;base64,aGVsbG8=")},
		{"oversized", imagePart("data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxImageBytes)+4))},
		{"dimensions", imagePart("data:image/png;base64," + base64.StdEncoding.EncodeToString(oversized.Bytes()))},
		{"mime", imagePart(strings.Replace(fixtureURL(t, "ocr.png"), "image/png", "image/jpeg", 1))},
		{"video", map[string]any{"type": "video", "video": "x"}},
		{"audio", map[string]any{"type": "input_audio", "input_audio": map[string]any{"data": "x"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(Options{Model: config.Model{Backend: "llama-vision"}})
			p := map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{tc.part}}}}
			if w := chatRecorder(t, s, p); w.Code != 400 {
				t.Fatalf("accepted invalid input: %d %s", w.Code, w.Body.String())
			}
		})
	}
	if w := chatRecorder(t, New(Options{}), visionPayload(t)); w.Code != 400 {
		t.Fatal("text model accepted images")
	}
	for _, p := range []map[string]any{{"max_tokens": -1}, {"max_tokens": 1025}, {"max_completion_tokens": 2000}, {"n_predict": -1}, {"n": 2}, {"chat_template_kwargs": map[string]any{"enable_thinking": "yes"}}} {
		p["messages"] = visionPayload(t)["messages"]
		if w := chatRecorder(t, New(Options{Model: config.Model{Backend: "llama-vision"}}), p); w.Code != 400 {
			t.Fatalf("accepted unbounded controls %v", p)
		}
	}
}

func TestAdaptersRejectMediaBeforeTextConversion(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/v1/messages"} {
		p := map[string]any{"input": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_image", "image_url": "x"}}}}, "messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "x"}}}}}}
		b, _ := json.Marshal(p)
		w := httptest.NewRecorder()
		New(Options{}).Handler().ServeHTTP(w, httptest.NewRequest("POST", path, bytes.NewReader(b)))
		if w.Code != 400 {
			t.Fatalf("%s discarded media: %d", path, w.Code)
		}
	}
}

func TestVisionLaunchArguments(t *testing.T) {
	s := New(Options{Model: config.Model{Path: "model.gguf", ProjectorPath: "projector.gguf", Backend: "llama-vision"}, Context: 4096, CPU: true})
	args := strings.Join(s.llamaArgs(), " ")
	for _, want := range []string{"--mmproj projector.gguf", "--n-gpu-layers 0", "--no-mmproj-offload", "--parallel 1", `--chat-template-kwargs {"enable_thinking":false}`, "--n-predict 512", "--jinja"} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %s in %s", want, args)
		}
	}
	s.opts.Model.Backend = ""
	if strings.Contains(strings.Join(s.llamaArgs(), " "), "mmproj") {
		t.Fatal("text model got projector flags")
	}
	if err := verifyVisionRuntime(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("accepted missing runtime")
	}
}

func TestVisionStreamingCancellation(t *testing.T) {
	cancelled := make(chan struct{})
	received := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
			return
		}
		if p["stream"] != true {
			t.Error("lost streaming flag")
		}
		close(received)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(cancelled)
	}))
	defer upstream.Close()
	parsed, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(parsed.Port())
	proxy := httptest.NewServer(New(Options{Model: config.Model{Backend: "llama-vision"}, UpstreamPort: port}).Handler())
	defer proxy.Close()
	payload := visionPayload(t)
	payload["stream"] = true
	b, _ := json.Marshal(payload)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", proxy.URL+"/v1/chat/completions", bytes.NewReader(b))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 8)
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("upstream not called")
	}
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not reach backend")
	}
}

func TestVisionRejectsStaleRuntimeOnUpgrade(t *testing.T) {
	for _, output := range []string{"version: b10603", "version: b11476", "build 11696, commit wrong", "build 11697, commit ceefd16da"} {
		if validVisionRuntimeVersion(output) {
			t.Fatalf("accepted stale/unqualified runtime %q", output)
		}
	}
	if !validVisionRuntimeVersion("version: 0.7.0-dev (build 11696, commit ceefd16da)") {
		t.Fatal("rejected pinned runtime")
	}
}

func TestVisionImageCountAndRequestBodyBounds(t *testing.T) {
	image := imagePart(fixtureURL(t, "shapes.png"))
	var parts []any
	for range 5 {
		parts = append(parts, image)
	}
	s := New(Options{Model: config.Model{Backend: "llama-vision"}})
	payload := map[string]any{"messages": []any{map[string]any{"role": "user", "content": parts}}}
	if w := chatRecorder(t, s, payload); w.Code != 400 {
		t.Fatal("accepted more than four images")
	}
	payload = map[string]any{"messages": []any{map[string]any{"role": "assistant", "content": []any{image}}}}
	if w := chatRecorder(t, s, payload); w.Code != 400 {
		t.Fatal("accepted assistant image")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"messages":[],"padding":"`+strings.Repeat("x", MaxChatBodyBytes)+`"}`)))
	if w.Code != 400 {
		t.Fatal("accepted oversized request")
	}
}
