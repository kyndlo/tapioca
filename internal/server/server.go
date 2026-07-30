package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/textruntime"
)

type Options struct {
	Model        config.Model
	Host         string
	Port         int
	UpstreamPort int
	Context      int
	LlamaServer  string
	Verbose      bool
}

type Server struct {
	opts      Options
	client    *http.Client
	cmd       *exec.Cmd
	logBuffer bytes.Buffer
	mu        sync.Mutex
}

func New(opts Options) *Server {
	return &Server{opts: opts, client: &http.Client{Timeout: 30 * time.Minute}}
}

func (s *Server) Start(ctx context.Context) error {
	if s.opts.Model.Backend == "mlx-vlm" {
		home, err := config.Home()
		if err != nil {
			return err
		}
		s.cmd, err = textruntime.Command(
			ctx, filepath.Join(home, "runtime"), s.opts.Model.Path,
			"127.0.0.1", s.opts.UpstreamPort, s.opts.Context,
		)
		if err != nil {
			return err
		}
	} else {
		if s.opts.LlamaServer == "" {
			s.opts.LlamaServer = bundledLlamaServer()
			if s.opts.LlamaServer == "" {
				var err error
				s.opts.LlamaServer, err = exec.LookPath("llama-server")
				if err != nil {
					return fmt.Errorf("llama-server is required; use an official Tapioca bundle or install llama.cpp")
				}
			}
		}
		args := []string{
			"--model", s.opts.Model.Path,
			"--host", "127.0.0.1",
			"--port", strconv.Itoa(s.opts.UpstreamPort),
			"--ctx-size", strconv.Itoa(s.opts.Context),
			"--jinja",
			"--n-gpu-layers", "999",
		}
		if !s.opts.Verbose {
			args = append(args, "-lv", "0")
		}
		s.cmd = exec.CommandContext(ctx, s.opts.LlamaServer, args...)
	}
	if s.opts.Verbose {
		s.cmd.Stdout = os.Stderr
		s.cmd.Stderr = os.Stderr
	} else {
		s.cmd.Stdout = io.Discard
		s.cmd.Stderr = &s.logBuffer
	}
	configureProcess(s.cmd)
	if err := s.cmd.Start(); err != nil {
		return err
	}
	processDone := make(chan error, 1)
	go func() {
		processDone <- s.cmd.Wait()
	}()
	if err := s.waitReady(ctx, processDone); err != nil {
		s.Stop()
		if detail := strings.TrimSpace(s.logBuffer.String()); detail != "" {
			return fmt.Errorf("%w\n%s", err, detail)
		}
		return err
	}
	httpServer := &http.Server{Addr: net.JoinHostPort(s.opts.Host, strconv.Itoa(s.opts.Port)), Handler: s.Handler()}
	go func() {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
		s.Stop()
	}()
	if s.opts.Verbose {
		log.Printf("tapioca serving %s at http://%s", s.opts.Model.Name, httpServer.Addr)
	}
	err := httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func bundledLlamaServer() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "llama-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(filepath.Dir(executable), "runtime", "llama.cpp", name)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	return ""
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/v1/models", s.models)
	mux.HandleFunc("/v1/chat/completions", s.chat)
	mux.HandleFunc("/v1/responses", s.responses)
	mux.HandleFunc("/v1/messages", s.messages)
	if s.opts.Verbose {
		return logging(mux)
	}
	return mux
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = stopProcess(s.cmd)
	}
}

func (s *Server) waitReady(ctx context.Context, processDone <-chan error) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", s.opts.UpstreamPort)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Minute)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-processDone:
			if err == nil {
				return errors.New("model backend exited before becoming ready")
			}
			return fmt.Errorf("model backend exited before becoming ready: %w", err)
		case <-timeout.C:
			return fmt.Errorf("timed out waiting for llama-server")
		case <-ticker.C:
			resp, err := s.client.Get(url)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode < 500 {
					return nil
				}
			}
		}
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "model": s.opts.Model.Name})
}

func (s *Server) models(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"object": "list",
		"data":   []any{map[string]any{"id": s.opts.Model.Name, "object": "model", "owned_by": "tapioca"}},
	})
}

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	s.proxy(w, r, "/v1/chat/completions")
}

func (s *Server) proxy(w http.ResponseWriter, r *http.Request, path string) {
	var body io.Reader = r.Body
	if s.opts.Model.Backend == "mlx-vlm" && r.Method == http.MethodPost {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload["model"] = s.backendModelID()
		encoded, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, s.upstream(path), body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	req.Header = r.Header.Clone()
	req.Header.Del("Content-Length")
	resp, err := s.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer resp.Body.Close()
	for k, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(k, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		_, _ = io.Copy(flushWriter{w: w}, resp.Body)
		return
	}
	_, _ = io.Copy(w, resp.Body)
}

type flushWriter struct {
	w http.ResponseWriter
}

func (f flushWriter) Write(data []byte) (int, error) {
	n, err := f.w.Write(data)
	if flusher, ok := f.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}

func (s *Server) complete(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	request.Model = s.backendModelID()
	request.Stream = false
	b, _ := json.Marshal(request)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.upstream("/v1/chat/completions"), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("llama-server: %s: %s", resp.Status, body)
	}
	var result ChatResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func (s *Server) backendModelID() string {
	if s.opts.Model.Backend == "mlx-vlm" {
		return s.opts.Model.Path
	}
	return s.opts.Model.Name
}

func (s *Server) upstream(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", s.opts.UpstreamPort, path)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func textContent(v any) string {
	if v == nil {
		return ""
	}
	switch value := v.(type) {
	case string:
		return value
	case []any:
		var b strings.Builder
		for _, raw := range value {
			if part, ok := raw.(map[string]any); ok {
				if text, ok := part["text"].(string); ok {
					b.WriteString(text)
				}
			}
		}
		return b.String()
	default:
		b, _ := json.Marshal(value)
		return string(b)
	}
}
