package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carlos/tapioca/internal/app"
	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
	modelserver "github.com/carlos/tapioca/internal/server"
)

type PullModelFunc func(context.Context, string, bool, app.PullReporter) (config.Model, error)
type ChatFunc func(context.Context, ChatParams) (modelserver.ChatResponse, error)

type ChatParams struct {
	Port            int                       `json:"port"`
	Model           string                    `json:"model,omitempty"`
	Messages        []modelserver.ChatMessage `json:"messages"`
	Tools           json.RawMessage           `json:"tools,omitempty"`
	ToolChoice      any                       `json:"tool_choice,omitempty"`
	Temperature     *float64                  `json:"temperature,omitempty"`
	MaxTokens       int                       `json:"max_tokens,omitempty"`
	ReasoningFormat string                    `json:"reasoning_format,omitempty"`
}

type RemovePlan struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	DryRun    bool   `json:"dry_run"`
	Removed   bool   `json:"removed"`
	TrashPath string `json:"trash_path,omitempty"`
}

func (h *Handler) handleFeature(
	ctx context.Context,
	request Request,
) (any, *ProtocolError) {
	switch request.Method {
	case "system.info":
		if err := decodeNoParams(request.Params); err != nil {
			return nil, err
		}
		return systemInfo(), nil
	case "storage.info":
		if err := decodeNoParams(request.Params); err != nil {
			return nil, err
		}
		return storageInfo()
	case "catalog.get":
		var params struct {
			Name string `json:"name"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return catalogDetail(ctx, params.Name)
	case "model.pull":
		var params struct {
			Name          string `json:"name"`
			Force         bool   `json:"force,omitempty"`
			AcceptLicense bool   `json:"accept_license,omitempty"`
			HFToken       string `json:"hf_token,omitempty"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		if strings.TrimSpace(params.Name) == "" {
			return nil, invalidParams("params.name is required", nil)
		}
		resolved, resolveError := catalog.Resolve(params.Name)
		if resolveError != nil {
			return nil, &ProtocolError{
				Code: "model_not_found", Message: resolveError.Error(), Retryable: false,
			}
		}
		if params.AcceptLicense {
			if !resolved.Gated {
				return nil, invalidParams("selected model does not require license acceptance", nil)
			}
			if acceptError := app.AcceptModelLicense(params.Name); acceptError != nil {
				return nil, operationError(ctx, "license_acceptance_failed", acceptError)
			}
		}
		if len(params.HFToken) > 1024 {
			return nil, invalidParams("params.hf_token is too long", nil)
		}
		if params.HFToken != "" {
			if !resolved.Gated {
				return nil, invalidParams("params.hf_token is only accepted for gated models", nil)
			}
			ctx = app.WithHuggingFaceToken(ctx, params.HFToken)
		}
		model, err := h.dependencies.Pull(ctx, params.Name, params.Force, func(progress app.PullProgress) {
			reportProgress(ctx, progress)
		})
		if err != nil {
			return nil, operationError(ctx, "pull_failed", err)
		}
		return installedModel(model), nil
	case "model.remove":
		var params struct {
			Name    string `json:"name"`
			DryRun  *bool  `json:"dry_run,omitempty"`
			Confirm string `json:"confirm,omitempty"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		dryRun := params.DryRun == nil || *params.DryRun
		return removeModel(params.Name, dryRun, params.Confirm, h.dependencies.Now())
	case "server.start":
		var params ServerStartParams
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return h.dependencies.Servers.Start(params)
	case "server.stop":
		var params struct {
			ID string `json:"id"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return h.dependencies.Servers.Stop(params.ID)
	case "server.status":
		var params struct {
			ID string `json:"id,omitempty"`
		}
		if len(request.Params) > 0 && string(request.Params) != "null" {
			if err := decodeParams(request.Params, &params); err != nil {
				return nil, err
			}
		}
		return h.dependencies.Servers.Status(params.ID), nil
	case "chat.request":
		var params ChatParams
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		if len(params.Messages) == 0 {
			return nil, invalidParams("params.messages must not be empty", nil)
		}
		if validationError := validateChatParams(params); validationError != nil {
			return nil, validationError
		}
		response, err := h.dependencies.Chat(ctx, params)
		if err != nil {
			return nil, operationError(ctx, "chat_failed", err)
		}
		if validationError := validateChatResponse(response); validationError != nil {
			return nil, validationError
		}
		return response, nil
	case "chat.describe":
		var params struct {
			Model string `json:"model"`
			Port  int    `json:"port,omitempty"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		if params.Port == 0 {
			params.Port = 11435
		}
		return map[string]any{
			"model": params.Model,
			"endpoint": "http://127.0.0.1:" + strconv.Itoa(params.Port) +
				"/v1/chat/completions",
			"protocol": "openai-chat-completions",
		}, nil
	case "agent.describe":
		var params struct {
			Agent string   `json:"agent"`
			Model string   `json:"model"`
			Port  int      `json:"port,omitempty"`
			Args  []string `json:"args,omitempty"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return describeAgent(params.Agent, params.Model, params.Port, params.Args)
	default:
		if result, err, handled := h.handleCreator(ctx, request); handled {
			return result, err
		}
		return nil, &ProtocolError{
			Code: "method_not_found", Message: "unknown method", Retryable: false,
			Details: map[string]any{"method": request.Method},
		}
	}
}

func systemInfo() map[string]any {
	return map[string]any{
		"goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"cpu_count": runtime.NumCPU(), "protocol_version": ProtocolVersion,
		"accelerators": detectAccelerators(),
	}
}

func detectAccelerators() []string {
	accelerators := []string{"cpu"}
	seen := map[string]bool{"cpu": true}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			accelerators = append(accelerators, name)
		}
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		add("apple")
	}
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		add("nvidia")
	}
	var description string
	switch runtime.GOOS {
	case "windows":
		if output, err := exec.Command(
			"powershell", "-NoProfile", "-Command",
			"(Get-CimInstance Win32_VideoController).Name -join ';'",
		).Output(); err == nil {
			description = strings.ToLower(string(output))
		}
	case "linux":
		if output, err := exec.Command("lspci").Output(); err == nil {
			description = strings.ToLower(string(output))
		}
	}
	if strings.Contains(description, "nvidia") {
		add("nvidia")
	}
	if strings.Contains(description, "amd") || strings.Contains(description, "radeon") {
		add("amd")
	}
	if strings.Contains(description, "intel") {
		add("intel")
	}
	return accelerators
}

func storageInfo() (any, *ProtocolError) {
	home, err := config.Home()
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	modelRoot := filepath.Join(home, "models")
	bytes, err := directorySize(modelRoot)
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	return map[string]any{
		"home": home, "models_path": modelRoot, "models_bytes": bytes,
	}, nil
}

func catalogDetail(ctx context.Context, name string) (any, *ProtocolError) {
	if strings.TrimSpace(name) == "" {
		return nil, invalidParams("params.name is required", nil)
	}
	resolved, err := catalog.Resolve(name)
	if err != nil {
		return nil, &ProtocolError{
			Code: "model_not_found", Message: err.Error(), Retryable: false,
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, operationError(ctx, "catalog_failed", err)
	}
	kind := resolved.Kind
	if kind == "" {
		kind = "text"
	}
	backend := resolved.Backend
	if backend == "" {
		backend = "llama.cpp"
	}
	return CatalogModel{
		Name: resolved.Name, Kind: kind, Backend: backend, Repo: resolved.Repo,
		Size: resolved.Size, Memory: resolved.Memory, GPU: resolved.GPU,
		Platforms: normalizePlatforms(resolved.Platform),
		Languages: resolved.Languages, Features: resolved.Features,
		Width: resolved.Width, Height: resolved.Height, Steps: resolved.Steps,
		Frames: resolved.Frames, FPS: resolved.FPS,
		Gated: resolved.Gated, License: resolved.License, LicenseURL: resolved.LicenseURL,
	}, nil
}

func pullModel(
	ctx context.Context,
	name string,
	force bool,
	report app.PullReporter,
) (config.Model, error) {
	return app.PullModel(ctx, name, force, report)
}

func removeModel(name string, dryRun bool, confirm string, now time.Time) (any, *ProtocolError) {
	registry, err := config.Load()
	if err != nil {
		return nil, operationError(context.Background(), "registry_failed", err)
	}
	model, ok := registry.Find(name)
	if !ok {
		return nil, &ProtocolError{
			Code: "model_not_installed", Message: "model is not installed", Retryable: false,
			Details: map[string]any{"name": name},
		}
	}
	home, err := config.Home()
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	modelRoot := filepath.Join(home, "models")
	path, err := filepath.Abs(model.Path)
	if err != nil || !withinRoot(modelRoot, path) {
		return nil, &ProtocolError{
			Code: "unsafe_model_path", Message: "registered model path is outside the model store",
			Retryable: false, Details: map[string]any{"path": model.Path},
		}
	}
	size, err := directorySize(path)
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	plan := RemovePlan{Name: model.Name, Path: path, Bytes: size, DryRun: dryRun}
	if dryRun {
		return plan, nil
	}
	if confirm != model.Name {
		return nil, &ProtocolError{
			Code:      "confirmation_required",
			Message:   "params.confirm must exactly match the installed model name",
			Retryable: false, Details: map[string]any{"expected": model.Name},
		}
	}
	trashRoot := filepath.Join(home, "trash", "models")
	if err := os.MkdirAll(trashRoot, 0o755); err != nil {
		return nil, operationError(context.Background(), "remove_failed", err)
	}
	trashPath := filepath.Join(
		trashRoot,
		strings.ReplaceAll(model.Name, ":", "-")+"-"+now.UTC().Format("20060102T150405.000000000Z"),
	)
	if err := os.Rename(path, trashPath); err != nil {
		return nil, operationError(context.Background(), "remove_failed", err)
	}
	delete(registry.Models, strings.ToLower(model.Name))
	if err := registry.Save(); err != nil {
		_ = os.Rename(trashPath, path)
		return nil, operationError(context.Background(), "registry_failed", err)
	}
	plan.DryRun = false
	plan.Removed = true
	plan.TrashPath = trashPath
	return plan, nil
}

func withinRoot(root, path string) bool {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, path)
	return err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func directorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func installedModel(model config.Model) InstalledModel {
	kind := model.Kind
	if kind == "" {
		kind = "text"
	}
	backend := model.Backend
	if backend == "" {
		backend = "llama.cpp"
	}
	return InstalledModel{
		Name: model.Name, Repo: model.Repo, Filename: model.Filename,
		Path: model.Path, Kind: kind, Backend: backend,
	}
}

type ServerStartParams struct {
	ID           string `json:"id,omitempty"`
	Model        string `json:"model"`
	Host         string `json:"host,omitempty"`
	Port         int    `json:"port,omitempty"`
	UpstreamPort int    `json:"upstream_port,omitempty"`
	Context      int    `json:"context,omitempty"`
}

type ModelServerStatus struct {
	ID       string `json:"id"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	State    string `json:"state"`
	Error    string `json:"error,omitempty"`
}

type managedServer struct {
	status ModelServerStatus
	cancel context.CancelFunc
}

type ModelServerManager struct {
	mu      sync.Mutex
	servers map[string]*managedServer
}

func NewModelServerManager() *ModelServerManager {
	return &ModelServerManager{servers: make(map[string]*managedServer)}
}

func (m *ModelServerManager) Start(params ServerStartParams) (any, *ProtocolError) {
	if params.Port == 0 {
		params.Port = 11435
	}
	if params.UpstreamPort == 0 {
		params.UpstreamPort = params.Port + 1
	}
	if params.Context == 0 {
		params.Context = catalog.DefaultContext(params.Model)
	}
	if params.Host == "" {
		params.Host = "127.0.0.1"
	}
	if ip := net.ParseIP(params.Host); ip == nil || !ip.IsLoopback() {
		return nil, invalidParams("server host must be a loopback IP address", nil)
	}
	if params.Port < 1 || params.Port > 65535 ||
		params.UpstreamPort < 1 || params.UpstreamPort > 65535 {
		return nil, invalidParams("ports must be between 1 and 65535", nil)
	}
	registry, err := config.Load()
	if err != nil {
		return nil, operationError(context.Background(), "registry_failed", err)
	}
	model, ok := registry.Find(params.Model)
	if !ok {
		return nil, &ProtocolError{
			Code: "model_not_installed", Message: "model is not installed", Retryable: false,
		}
	}
	if params.ID == "" {
		params.ID = "server-" + strconv.Itoa(params.Port)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.servers[params.ID]; exists {
		return nil, &ProtocolError{
			Code: "server_conflict", Message: "server ID is already in use", Retryable: false,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	entry := &managedServer{
		status: ModelServerStatus{
			ID: params.ID, Model: model.Name,
			Endpoint: "http://" + net.JoinHostPort(params.Host, strconv.Itoa(params.Port)),
			State:    "starting",
		},
		cancel: cancel,
	}
	m.servers[params.ID] = entry
	instance := modelserver.New(modelserver.Options{
		Model: model, Host: params.Host, Port: params.Port,
		UpstreamPort: params.UpstreamPort, Context: params.Context,
		Verbose: false,
	})
	go func() {
		err := instance.Start(ctx)
		m.mu.Lock()
		defer m.mu.Unlock()
		if ctx.Err() != nil {
			entry.status.State = "stopped"
			return
		}
		entry.status.State = "failed"
		if err != nil {
			entry.status.Error = err.Error()
		}
	}()
	return entry.status, nil
}

func (m *ModelServerManager) Stop(id string) (any, *ProtocolError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.servers[id]
	if !ok {
		return nil, &ProtocolError{
			Code: "server_not_found", Message: "server was not found", Retryable: false,
		}
	}
	entry.cancel()
	entry.status.State = "stopping"
	return entry.status, nil
}

func (m *ModelServerManager) Status(id string) []ModelServerStatus {
	m.mu.Lock()
	var statuses []ModelServerStatus
	for serverID, entry := range m.servers {
		if id == "" || id == serverID {
			statuses = append(statuses, entry.status)
		}
	}
	m.mu.Unlock()
	client := &http.Client{Timeout: 250 * time.Millisecond}
	for index := range statuses {
		if statuses[index].State != "starting" {
			continue
		}
		response, err := client.Get(statuses[index].Endpoint + "/health")
		if err == nil {
			_ = response.Body.Close()
		}
		if err == nil && response.StatusCode < 500 {
			statuses[index].State = "running"
			m.mu.Lock()
			if entry := m.servers[statuses[index].ID]; entry != nil {
				entry.status.State = "running"
			}
			m.mu.Unlock()
		}
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].ID < statuses[j].ID })
	return statuses
}

func (m *ModelServerManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.servers {
		entry.cancel()
	}
}

func sendChat(ctx context.Context, params ChatParams) (modelserver.ChatResponse, error) {
	if params.Port == 0 {
		params.Port = 11435
	}
	if params.Port < 1 || params.Port > 65535 {
		return modelserver.ChatResponse{}, errors.New("port must be between 1 and 65535")
	}
	request := modelserver.ChatRequest{
		Model: params.Model, Messages: params.Messages, Stream: false,
		Tools: params.Tools, ToolChoice: params.ToolChoice,
		Temperature: params.Temperature, MaxTokens: params.MaxTokens,
		ReasoningFormat: params.ReasoningFormat,
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return modelserver.ChatResponse{}, err
	}
	url := "http://127.0.0.1:" + strconv.Itoa(params.Port) + "/v1/chat/completions"
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return modelserver.ChatResponse{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Minute}).Do(httpRequest)
	if err != nil {
		return modelserver.ChatResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return modelserver.ChatResponse{}, fmt.Errorf("local server returned %s", response.Status)
	}
	var result modelserver.ChatResponse
	err = json.NewDecoder(response.Body).Decode(&result)
	return result, err
}

const (
	MaxChatMessageBytes  = 1024 * 1024
	MaxChatToolBytes     = 512 * 1024
	MaxChatResponseBytes = 2 * 1024 * 1024
	MaxChatMessages      = 256
	MaxChatChoices       = 64
)

func validateChatParams(params ChatParams) *ProtocolError {
	if len(params.Messages) > MaxChatMessages {
		return invalidParams("params.messages exceeds the 256-message limit", nil)
	}
	for index, message := range params.Messages {
		if err := validateChatMessage(message, "params.messages["+strconv.Itoa(index)+"]", true); err != nil {
			return err
		}
	}
	if len(params.Tools) > 0 {
		if len(params.Tools) > MaxChatToolBytes || !json.Valid(params.Tools) ||
			!jsonArray(params.Tools) {
			return invalidParams("params.tools must be a JSON array no larger than 512 KiB", nil)
		}
	}
	if params.Temperature != nil && (*params.Temperature < 0 || *params.Temperature > 2) {
		return invalidParams("params.temperature must be between 0 and 2", nil)
	}
	if params.MaxTokens < 0 || params.MaxTokens > 1_000_000 {
		return invalidParams("params.max_tokens must be between 0 and 1000000", nil)
	}
	if len(params.ReasoningFormat) > 64 {
		return invalidParams("params.reasoning_format exceeds 64 bytes", nil)
	}
	if params.ToolChoice != nil {
		encoded, err := json.Marshal(params.ToolChoice)
		if err != nil || len(encoded) > 64*1024 {
			return invalidParams("params.tool_choice is invalid or exceeds 64 KiB", nil)
		}
	}
	return nil
}

func validateChatResponse(response modelserver.ChatResponse) *ProtocolError {
	if len(response.Choices) > MaxChatChoices {
		return backendChatError("backend_response_too_large", "chat response exceeds the 64-choice limit")
	}
	for index, choice := range response.Choices {
		if err := validateChatMessage(
			choice.Message,
			"result.choices["+strconv.Itoa(index)+"].message",
			false,
		); err != nil {
			if err.Code == "invalid_params" {
				return backendChatError("invalid_backend_response", err.Message)
			}
			return err
		}
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return backendChatError("invalid_backend_response", "chat response cannot be encoded as JSON")
	}
	if len(encoded) > MaxChatResponseBytes {
		return backendChatError("backend_response_too_large", "chat response exceeds the 2 MiB limit")
	}
	return nil
}

func validateChatMessage(
	message modelserver.ChatMessage,
	path string,
	request bool,
) *ProtocolError {
	validRole := false
	for _, role := range []string{"system", "developer", "user", "assistant", "tool"} {
		if message.Role == role {
			validRole = true
			break
		}
	}
	if !validRole {
		return invalidParams(path+".role is invalid", nil)
	}
	content, err := json.Marshal(message.Content)
	if err != nil || len(content) > MaxChatMessageBytes {
		return invalidParams(path+".content is invalid or exceeds 1 MiB", nil)
	}
	if len(message.ReasoningContent) > MaxChatMessageBytes {
		return invalidParams(path+".reasoning_content exceeds 1 MiB", nil)
	}
	if len(message.ToolCalls) > 0 {
		if len(message.ToolCalls) > MaxChatToolBytes || !json.Valid(message.ToolCalls) ||
			!jsonArray(message.ToolCalls) {
			return invalidParams(path+".tool_calls must be a JSON array no larger than 512 KiB", nil)
		}
	}
	if request && message.Role == "tool" && strings.TrimSpace(message.ToolCallID) == "" {
		return invalidParams(path+".tool_call_id is required for tool messages", nil)
	}
	if len(message.ToolCallID) > 256 {
		return invalidParams(path+".tool_call_id exceeds 256 bytes", nil)
	}
	return nil
}

func jsonArray(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']'
}

func backendChatError(code, message string) *ProtocolError {
	return &ProtocolError{Code: code, Message: message, Retryable: false}
}

type AgentDescriptor struct {
	Agent         string            `json:"agent"`
	Executable    string            `json:"executable"`
	Args          []string          `json:"args"`
	Environment   map[string]string `json:"environment"`
	Configuration map[string]any    `json:"configuration,omitempty"`
	Endpoint      string            `json:"endpoint"`
	Protocol      string            `json:"protocol"`
	Installed     bool              `json:"installed"`
}

func describeAgent(agent, model string, port int, args []string) (any, *ProtocolError) {
	if port == 0 {
		port = 11435
	}
	if port < 1 || port > 65535 {
		return nil, invalidParams("params.port must be between 1 and 65535", nil)
	}
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	descriptor := AgentDescriptor{
		Agent: strings.ToLower(agent), Args: append([]string{}, args...),
		Environment: map[string]string{}, Endpoint: base + "/v1",
		Protocol: "openai-compatible",
	}
	switch descriptor.Agent {
	case "codex":
		descriptor.Executable = "codex"
		descriptor.Protocol = "openai-responses"
		descriptor.Configuration = map[string]any{
			"format": "codex-toml", "model": model,
			"provider": "tapioca", "base_url": descriptor.Endpoint,
			"wire_api": "responses",
		}
	case "claude", "claude-code":
		descriptor.Agent = "claude-code"
		descriptor.Executable = "claude"
		descriptor.Environment["ANTHROPIC_BASE_URL"] = base
		descriptor.Environment["ANTHROPIC_AUTH_TOKEN"] = "tapioca-local"
		descriptor.Environment["ANTHROPIC_DEFAULT_SONNET_MODEL"] = model
		descriptor.Protocol = "anthropic-messages"
	case "opencode", "open-code":
		descriptor.Agent = "opencode"
		descriptor.Executable = "opencode"
		descriptor.Configuration = map[string]any{
			"format": "opencode-json", "provider": "tapioca",
			"base_url": descriptor.Endpoint, "model": model,
		}
	case "openclaw", "open-claw":
		descriptor.Agent = "openclaw"
		descriptor.Executable = "openclaw"
		descriptor.Configuration = map[string]any{
			"format": "openclaw-json", "provider": "tapioca",
			"base_url": descriptor.Endpoint, "model": model,
			"gateway_bind": "loopback",
		}
	case "hermes":
		descriptor.Executable = "hermes"
		descriptor.Environment["OPENAI_API_KEY"] = "tapioca-local"
		descriptor.Configuration = map[string]any{
			"format": "hermes-yaml", "provider": "custom",
			"base_url": descriptor.Endpoint, "model": model,
		}
	default:
		return nil, &ProtocolError{
			Code:      "unsupported_agent",
			Message:   "agent must be codex, claude-code, opencode, openclaw, or hermes",
			Retryable: false,
		}
	}
	if resolved, err := lookPathAgent(descriptor.Executable); err == nil {
		descriptor.Executable = resolved
		descriptor.Installed = true
	}
	return descriptor, nil
}

func lookPathAgent(name string) (string, error) {
	if resolved, err := exec.LookPath(name); err == nil {
		return resolved, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(userHome, ".local", "bin", name),
		filepath.Join(userHome, ".opencode", "bin", name),
		filepath.Join(userHome, "bin", name),
	}
	nvmCandidates, _ := filepath.Glob(filepath.Join(
		userHome, ".nvm", "versions", "node", "*", "bin", name,
	))
	candidates = append(candidates, nvmCandidates...)
	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func invalidParams(message string, details any) *ProtocolError {
	return &ProtocolError{
		Code: "invalid_params", Message: message, Retryable: false, Details: details,
	}
}

func operationError(ctx context.Context, code string, err error) *ProtocolError {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return &ProtocolError{
			Code: "job_cancelled", Message: "job was cancelled", Retryable: false,
		}
	}
	reportLog(ctx, err.Error())
	return &ProtocolError{Code: code, Message: err.Error(), Retryable: true}
}
