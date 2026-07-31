package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/server"
)

const version = "0.1.0"

func Run(args []string) error {
	if len(args) == 0 {
		usage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "pull":
		return pull(args[1:])
	case "serve":
		return serve(args[1:])
	case "run":
		return run(args[1:])
	case "launch":
		return launch(args[1:])
	case "image":
		return image(args[1:])
	case "video":
		return video(args[1:])
	case "list":
		return list()
	case "catalog":
		return showCatalog()
	case "version", "--version", "-v":
		fmt.Println("tapioca", version)
		return nil
	case "help", "--help", "-h":
		usage(os.Stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q; run `tapioca help`", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `Tapioca runs local language and diffusion models.

Usage:
  tapioca pull MODEL[:QUANT]
  tapioca serve MODEL [--port 11435] [--context 65536]
  tapioca run MODEL
  tapioca image MODEL --prompt TEXT [--output image.png]
  tapioca video MODEL --prompt TEXT [--image start.png] [--output video.mp4]
  tapioca launch (codex|claude|opencode|openclaw|hermes) MODEL [-- CLIENT_ARGS...]
  tapioca catalog
  tapioca list

Examples:
  tapioca pull glm-4.7-flash:q8_0
  tapioca serve glm-4.7-flash:q8_0
  tapioca run glm-4.7-flash:q8_0
  tapioca pull qwen-image-flash:int8
  tapioca image qwen-image-flash:int8 --prompt "A red fox in snow"
  tapioca video wan2.2-video:5b-q8-mlx --prompt "A red fox running in snow"
  tapioca launch codex glm-4.7-flash:q8_0
  tapioca launch openclaw glm-4.7-flash:q8_0
  tapioca launch hermes glm-4.7-flash:q8_0`)
}

func pull(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca pull MODEL[:QUANT]")
	}
	ref := args[0]
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	force := fs.Bool("force", false, "download again")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: tapioca pull MODEL[:QUANT]")
	}
	resolved, err := catalog.Resolve(ref)
	if err != nil {
		return err
	}
	_, err = pullResolved(resolved, *force)
	return err
}

func showCatalog() error {
	fmt.Println("MODEL\tKIND\tBACKEND\tDOWNLOAD\tMEMORY\tGPU\tPLATFORM")
	for _, ref := range catalog.Refs() {
		model, err := catalog.Resolve(ref)
		if err != nil {
			return err
		}
		kind := model.Kind
		if kind == "" {
			kind = "text"
		}
		backend := model.Backend
		if backend == "" {
			backend = "llama.cpp"
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			model.Name, kind, backend, model.Size, model.Memory, model.GPU, model.Platform)
	}
	return nil
}

func pullResolved(resolved catalog.Resolved, force bool) (config.Model, error) {
	home, err := config.Home()
	if err != nil {
		return config.Model{}, err
	}
	dir := filepath.Join(home, "models", strings.ReplaceAll(resolved.Name, ":", "-"))
	if resolved.Kind == "image" || resolved.Kind == "video" {
		if err := pullSnapshot(resolved, dir, force); err != nil {
			return config.Model{}, err
		}
		if err := register(resolved, dir); err != nil {
			return config.Model{}, err
		}
		return modelFromResolved(resolved, dir), nil
	}
	if resolved.Backend == "mlx-vlm" {
		if err := pullTextSnapshot(resolved, dir, force); err != nil {
			return config.Model{}, err
		}
		if err := register(resolved, dir); err != nil {
			return config.Model{}, err
		}
		return modelFromResolved(resolved, dir), nil
	}
	path := filepath.Join(dir, resolved.Filename)
	if _, err := os.Stat(path); err == nil && !force {
		fmt.Printf("%s already exists at %s\n", resolved.Name, path)
		if err := register(resolved, path); err != nil {
			return config.Model{}, err
		}
		return modelFromResolved(resolved, path), nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return config.Model{}, err
	}
	partial := path + ".partial"
	fmt.Printf("pulling %s from %s\n", resolved.Name, resolved.Repo)
	if err := download(resolved.URL, partial); err != nil {
		return config.Model{}, err
	}
	if err := os.Rename(partial, path); err != nil {
		return config.Model{}, err
	}
	if err := register(resolved, path); err != nil {
		return config.Model{}, err
	}
	fmt.Printf("saved %s\n", path)
	return modelFromResolved(resolved, path), nil
}

func modelFromResolved(resolved catalog.Resolved, path string) config.Model {
	return config.Model{
		Name: resolved.Name, Repo: resolved.Repo, Filename: resolved.Filename, Path: path,
		Kind: resolved.Kind, Backend: resolved.Backend,
	}
}

func register(resolved catalog.Resolved, path string) error {
	registry, err := config.Load()
	if err != nil {
		return err
	}
	registry.Models[strings.ToLower(resolved.Name)] = config.Model{
		Name: resolved.Name, Repo: resolved.Repo, Filename: resolved.Filename, Path: path,
		Kind: resolved.Kind, Backend: resolved.Backend,
	}
	return registry.Save()
}

func download(url, destination string) error {
	var offset int64
	if info, err := os.Stat(destination); err == nil {
		offset = info.Size()
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		offset = 0
	}
	file, err := os.OpenFile(destination, flags, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	total := resp.ContentLength
	if total > 0 {
		total += offset
	}
	progress := &progressWriter{w: file, written: offset, total: total, last: time.Now()}
	_, err = io.Copy(progress, resp.Body)
	fmt.Fprintln(os.Stderr)
	return err
}

type progressWriter struct {
	w       io.Writer
	written int64
	total   int64
	last    time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.written += int64(n)
	if time.Since(p.last) > time.Second {
		if p.total > 0 {
			fmt.Fprintf(os.Stderr, "\r%.1f%%  %.1f / %.1f GB", float64(p.written)*100/float64(p.total), float64(p.written)/1e9, float64(p.total)/1e9)
		} else {
			fmt.Fprintf(os.Stderr, "\r%.1f GB", float64(p.written)/1e9)
		}
		p.last = time.Now()
	}
	return n, err
}

type serveOptions struct {
	host         string
	port         int
	upstreamPort int
	context      int
	llamaServer  string
	verbose      bool
}

func serve(args []string) error {
	opts, model, err := parseServe(args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	return startServer(ctx, model, opts)
}

func parseServe(args []string) (serveOptions, config.Model, error) {
	if len(args) == 0 {
		return serveOptions{}, config.Model{}, errors.New("usage: tapioca serve MODEL [flags]")
	}
	ref := args[0]
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	opts := serveOptions{}
	fs.StringVar(&opts.host, "host", "127.0.0.1", "listen host")
	fs.IntVar(&opts.port, "port", 11435, "Tapioca API port")
	fs.IntVar(&opts.upstreamPort, "upstream-port", 11436, "private llama-server port")
	fs.IntVar(&opts.context, "context", 65536, "context window")
	fs.StringVar(&opts.llamaServer, "llama-server", "", "path to llama-server")
	fs.BoolVar(&opts.verbose, "verbose", false, "show llama.cpp and HTTP request logs")
	if err := fs.Parse(args[1:]); err != nil {
		return opts, config.Model{}, err
	}
	if fs.NArg() != 0 {
		return opts, config.Model{}, errors.New("usage: tapioca serve MODEL [flags]")
	}
	model, err := ensureModel(ref)
	return opts, model, err
}

func startServer(ctx context.Context, model config.Model, opts serveOptions) error {
	if model.Kind == "image" {
		return fmt.Errorf("%s is an image model; use `tapioca image %s --prompt TEXT`", model.Name, model.Name)
	}
	s := server.New(server.Options{
		Model: model, Host: opts.host, Port: opts.port, UpstreamPort: opts.upstreamPort,
		Context: opts.context, LlamaServer: opts.llamaServer, Verbose: opts.verbose,
	})
	return s.Start(ctx)
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca run MODEL")
	}
	ref := args[0]
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	port := fs.Int("port", 11435, "Tapioca API port")
	contextSize := fs.Int("context", 65536, "context window")
	llamaServer := fs.String("llama-server", "", "path to llama-server")
	verbose := fs.Bool("verbose", false, "show llama.cpp and HTTP request logs")
	showThinking := fs.Bool("show-thinking", true, "show the model's reasoning before its answer")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: tapioca run MODEL")
	}
	model, err := ensureModel(ref)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	errs := make(chan error, 1)
	fmt.Println("Loading", model.Name+"...")
	go func() {
		errs <- startServer(ctx, model, serveOptions{
			host: "127.0.0.1", port: *port, upstreamPort: *port + 1,
			context: *contextSize, llamaServer: *llamaServer, verbose: *verbose,
		})
	}()
	if err := waitForHealth(ctx, *port); err != nil {
		return err
	}
	fmt.Println("Chat with", model.Name, "(/bye or Ctrl-D to exit)")
	scanner := bufio.NewScanner(os.Stdin)
	messages := []server.ChatMessage{{
		Role:    "system",
		Content: "The current local date is " + time.Now().Format("Monday, January 2, 2006") + ".",
	}}
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		prompt := strings.TrimSpace(scanner.Text())
		if prompt == "" {
			continue
		}
		if isChatExit(prompt) {
			fmt.Println("Bye!")
			break
		}
		messages = append(messages, server.ChatMessage{Role: "user", Content: prompt})
		reply, thinking, err := chat(*port, model.Name, messages, *showThinking)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			continue
		}
		messages = append(messages, server.ChatMessage{
			Role: "assistant", Content: reply, ReasoningContent: thinking,
		})
	}
	stop()
	select {
	case err := <-errs:
		return err
	case <-time.After(3 * time.Second):
		return nil
	}
}

func isChatExit(prompt string) bool {
	return strings.EqualFold(strings.TrimSpace(prompt), "/bye")
}

func chat(port int, model string, messages []server.ChatMessage, showThinking bool) (string, string, error) {
	request := server.ChatRequest{Model: model, Messages: messages, Stream: true}
	if showThinking {
		request.ReasoningFormat = "deepseek"
	}
	body, _ := json.Marshal(request)
	resp, err := http.Post("http://127.0.0.1:"+strconv.Itoa(port)+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("%s: %s", resp.Status, b)
	}
	var content, thinking strings.Builder
	var sawThinking, sawContent bool
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta server.ChatMessage `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil || len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.ReasoningContent != "" {
			if !sawThinking {
				sawThinking = true
				if showThinking {
					fmt.Print("Thinking:\n")
				} else {
					fmt.Print("Thinking...")
				}
			}
			thinking.WriteString(delta.ReasoningContent)
			if showThinking {
				fmt.Print(delta.ReasoningContent)
			}
		}
		text := textDelta(delta.Content)
		if text != "" {
			if !sawContent {
				sawContent = true
				if sawThinking {
					if showThinking {
						fmt.Print("\n\nAnswer:\n")
					} else {
						fmt.Print(" done.\n")
					}
				}
			}
			content.WriteString(text)
			fmt.Print(text)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	if sawThinking || sawContent {
		fmt.Println()
	}
	if !sawContent {
		return "", thinking.String(), errors.New("model returned no answer")
	}
	return content.String(), thinking.String(), nil
}

func textDelta(content any) string {
	if content == nil {
		return ""
	}
	if text, ok := content.(string); ok {
		return text
	}
	return serverTextContent(content)
}

func serverTextContent(content any) string {
	switch parts := content.(type) {
	case []any:
		var result strings.Builder
		for _, raw := range parts {
			if part, ok := raw.(map[string]any); ok {
				if text, ok := part["text"].(string); ok {
					result.WriteString(text)
				}
			}
		}
		return result.String()
	default:
		return fmt.Sprint(content)
	}
}

func waitForHealth(ctx context.Context, port int) error {
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/health"
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			resp, err := http.Get(url)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == 200 {
					return nil
				}
			}
		}
	}
}

func ensureModel(ref string) (config.Model, error) {
	registry, err := config.Load()
	if err != nil {
		return config.Model{}, err
	}
	registered := false
	if model, ok := registry.Find(ref); ok {
		registered = true
		if _, err := os.Stat(model.Path); err == nil {
			return model, nil
		}
		fmt.Printf("%s is registered but unavailable; pulling it again\n", model.Name)
	}
	resolved, err := catalog.Resolve(ref)
	if err != nil {
		return config.Model{}, err
	}
	if !registered {
		fmt.Printf("%s is not installed; pulling it now\n", resolved.Name)
	}
	return pullResolved(resolved, false)
}

func list() error {
	registry, err := config.Load()
	if err != nil {
		return err
	}
	if len(registry.Models) == 0 {
		fmt.Println("No models installed.")
		return nil
	}
	for _, model := range registry.Models {
		size, _ := pathSize(model.Path)
		kind := model.Kind
		if kind == "" {
			kind = "text"
		}
		backend := model.Backend
		if backend == "" {
			backend = "llama.cpp"
		}
		fmt.Printf("%-28s %-6s %-10s %6.1f GB  %s\n", model.Name, kind, backend, float64(size)/1e9, model.Path)
	}
	return nil
}

func launch(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: tapioca launch CLIENT MODEL [flags] [-- CLIENT_ARGS...]")
	}
	clientName, ref := args[0], args[1]
	options, clientArgs := splitClientArgs(args[2:])
	fs := flag.NewFlagSet("launch", flag.ContinueOnError)
	port := fs.Int("port", 11435, "Tapioca API port")
	contextSize := fs.Int("context", 65536, "context window")
	llamaServer := fs.String("llama-server", "", "path to llama-server")
	verbose := fs.Bool("verbose", false, "show llama.cpp and HTTP request logs")
	if err := fs.Parse(options); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: tapioca launch CLIENT MODEL [flags] [-- CLIENT_ARGS...]")
	}
	model, err := ensureModel(ref)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	errs := make(chan error, 1)
	fmt.Println("Loading", model.Name+"...")
	go func() {
		errs <- startServer(ctx, model, serveOptions{
			host: "127.0.0.1", port: *port, upstreamPort: *port + 1,
			context: *contextSize, llamaServer: *llamaServer, verbose: *verbose,
		})
	}()
	if err := waitForHealth(ctx, *port); err != nil {
		return err
	}
	cmd, err := launcher(clientName, model.Name, *port, clientArgs)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	stop()
	select {
	case serverErr := <-errs:
		if err == nil && serverErr != nil {
			return serverErr
		}
	case <-time.After(3 * time.Second):
	}
	return err
}

func splitClientArgs(args []string) ([]string, []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func launcher(clientName, model string, port int, args []string) (*exec.Cmd, error) {
	binary := clientName
	switch clientName {
	case "claude-code":
		binary = "claude"
	case "open-code":
		binary = "opencode"
	case "open-claw":
		binary = "openclaw"
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("%s is not installed or not on PATH", binary)
	}
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	cmd := exec.Command(path, args...)
	cmd.Env = os.Environ()
	switch binary {
	case "codex":
		home, err := launchDir("codex")
		if err != nil {
			return nil, err
		}
		toml := fmt.Sprintf(`model = %q
model_provider = "tapioca"

[model_providers.tapioca]
name = "Tapioca"
base_url = %q
wire_api = "responses"
`, model, baseURL+"/v1")
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(toml), 0o644); err != nil {
			return nil, err
		}
		cmd.Env = withEnv(cmd.Env, "CODEX_HOME", home)
	case "claude":
		cmd.Env = withEnv(cmd.Env, "ANTHROPIC_BASE_URL", baseURL)
		cmd.Env = withEnv(cmd.Env, "ANTHROPIC_AUTH_TOKEN", "tapioca-local")
		cmd.Env = withEnv(cmd.Env, "ANTHROPIC_API_KEY", "")
		cmd.Env = withEnv(cmd.Env, "ANTHROPIC_DEFAULT_OPUS_MODEL", model)
		cmd.Env = withEnv(cmd.Env, "ANTHROPIC_DEFAULT_SONNET_MODEL", model)
		cmd.Env = withEnv(cmd.Env, "ANTHROPIC_DEFAULT_HAIKU_MODEL", model)
	case "opencode":
		home, err := launchDir("opencode")
		if err != nil {
			return nil, err
		}
		configPath := filepath.Join(home, "opencode.json")
		payload := map[string]any{
			"$schema": "https://opencode.ai/config.json",
			"model":   "tapioca/" + model,
			"provider": map[string]any{
				"tapioca": map[string]any{
					"npm": "@ai-sdk/openai-compatible", "name": "Tapioca",
					"options": map[string]any{"baseURL": baseURL + "/v1"},
					"models": map[string]any{
						model: map[string]any{
							"name":  model,
							"limit": map[string]any{"context": 65536, "output": 16384},
						},
					},
				},
			},
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		if err := os.WriteFile(configPath, append(b, '\n'), 0o644); err != nil {
			return nil, err
		}
		cmd.Env = withEnv(cmd.Env, "OPENCODE_CONFIG", configPath)
	case "openclaw":
		home, err := launchDir("openclaw")
		if err != nil {
			return nil, err
		}
		stateDir := filepath.Join(home, "state")
		workspace := filepath.Join(stateDir, "workspace")
		if err := os.MkdirAll(workspace, 0o755); err != nil {
			return nil, err
		}
		configPath := filepath.Join(stateDir, "openclaw.json")
		payload := map[string]any{
			"models": map[string]any{
				"mode": "merge",
				"providers": map[string]any{
					"tapioca": map[string]any{
						"baseUrl": baseURL + "/v1", "apiKey": "tapioca-local", "api": "openai-completions",
						"models": []any{map[string]any{
							"id": model, "name": model, "reasoning": true, "input": []string{"text"},
							"cost":          map[string]int{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
							"contextWindow": 65536, "maxTokens": 16384,
						}},
					},
				},
			},
			"agents": map[string]any{
				"defaults": map[string]any{
					"model": map[string]any{"primary": "tapioca/" + model}, "workspace": workspace,
				},
			},
			"gateway": map[string]any{
				"port": 18795, "mode": "local", "bind": "loopback",
				"auth": map[string]any{"mode": "token", "token": "tapioca-local"},
			},
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		if err := os.WriteFile(configPath, append(b, '\n'), 0o600); err != nil {
			return nil, err
		}
		scriptPath := filepath.Join(home, "launch-openclaw.sh")
		script := `#!/bin/sh
set -eu
gateway_log="$OPENCLAW_STATE_DIR/gateway.log"
"$TAPIOCA_OPENCLAW_BIN" gateway run --port 18795 >"$gateway_log" 2>&1 &
gateway_pid=$!
trap 'kill "$gateway_pid" 2>/dev/null || true' EXIT INT TERM
sleep 2
if ! kill -0 "$gateway_pid" 2>/dev/null; then
  echo "OpenClaw gateway failed to start:" >&2
  cat "$gateway_log" >&2
  exit 1
fi
exec "$TAPIOCA_OPENCLAW_BIN" tui --url ws://127.0.0.1:18795 --token tapioca-local "$@"
`
		if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
			return nil, err
		}
		cmd = exec.Command("/bin/sh", append([]string{scriptPath}, args...)...)
		cmd.Env = os.Environ()
		cmd.Env = withEnv(cmd.Env, "OPENCLAW_STATE_DIR", stateDir)
		cmd.Env = withEnv(cmd.Env, "OPENCLAW_CONFIG_PATH", configPath)
		cmd.Env = withEnv(cmd.Env, "TAPIOCA_OPENCLAW_BIN", path)
	case "hermes":
		home, err := launchDir("hermes")
		if err != nil {
			return nil, err
		}
		yaml := fmt.Sprintf(`model:
  default: %q
  provider: custom
  base_url: %q
  api_key: "tapioca-local"
  context_length: 65536
`, model, baseURL+"/v1")
		if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(yaml), 0o600); err != nil {
			return nil, err
		}
		cmd.Env = withEnv(cmd.Env, "HERMES_HOME", home)
		cmd.Env = withEnv(cmd.Env, "OPENAI_API_KEY", "tapioca-local")
	default:
		return nil, fmt.Errorf("unknown client %q; use codex, claude, opencode, openclaw, or hermes", clientName)
	}
	return cmd, nil
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	filtered := env[:0]
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, prefix+value)
}

func launchDir(name string) (string, error) {
	home, err := config.Home()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, "launch", name)
	return path, os.MkdirAll(path, 0o755)
}
