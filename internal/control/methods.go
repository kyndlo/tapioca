package control

import (
	"context"
	"encoding/json"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
)

type CatalogModel struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Backend    string   `json:"backend"`
	Repo       string   `json:"repo"`
	Size       string   `json:"size,omitempty"`
	Memory     string   `json:"memory,omitempty"`
	GPU        string   `json:"gpu,omitempty"`
	Platforms  []string `json:"platforms"`
	Languages  string   `json:"languages,omitempty"`
	Features   string   `json:"features,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	Steps      int      `json:"steps,omitempty"`
	Frames     int      `json:"frames,omitempty"`
	FPS        int      `json:"fps,omitempty"`
	Gated      bool     `json:"gated,omitempty"`
	License    string   `json:"license,omitempty"`
	LicenseURL string   `json:"license_url,omitempty"`
}

type InstalledModel struct {
	Name     string `json:"name"`
	Repo     string `json:"repo"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Backend  string `json:"backend"`
}

type Dependencies struct {
	Catalog        func(context.Context) ([]CatalogModel, error)
	RefreshCatalog func(context.Context) (catalog.RefreshResult, error)
	Installed      func(context.Context) ([]InstalledModel, error)
	Pull           PullModelFunc
	Servers        *ModelServerManager
	Chat           ChatFunc
	Image          ImageRunFunc
	Video          VideoRunFunc
	Speech         SpeechRunFunc
	Now            func() time.Time
}

type Handler struct {
	dependencies  Dependencies
	cancellations *CancellationRegistry
	startedAt     time.Time
}

const ControlVersion = "0.9.0"

func NewHandler(dependencies Dependencies) *Handler {
	if dependencies.Catalog == nil {
		dependencies.Catalog = loadCatalog
	}
	if dependencies.RefreshCatalog == nil {
		dependencies.RefreshCatalog = catalog.Refresh
	}
	if dependencies.Installed == nil {
		dependencies.Installed = loadInstalled
	}
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	if dependencies.Pull == nil {
		dependencies.Pull = pullModel
	}
	if dependencies.Servers == nil {
		dependencies.Servers = NewModelServerManager()
	}
	if dependencies.Chat == nil {
		dependencies.Chat = sendChat
	}
	if dependencies.Image == nil {
		dependencies.Image = runImage
	}
	if dependencies.Video == nil {
		dependencies.Video = runVideo
	}
	if dependencies.Speech == nil {
		dependencies.Speech = runSpeech
	}
	return &Handler{
		dependencies:  dependencies,
		cancellations: NewCancellationRegistry(),
		startedAt:     dependencies.Now(),
	}
}

func (h *Handler) Handle(ctx context.Context, request Request) (any, *ProtocolError) {
	if request.Method == "job.cancel" {
		var params struct {
			JobID string `json:"job_id"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		if params.JobID == "" {
			return nil, invalidRequest("params.job_id is required", nil)
		}
		if len(params.JobID) > MaxJobIDBytes {
			return nil, invalidRequest("params.job_id exceeds the 128-byte limit", nil)
		}
		return map[string]any{
			"job_id":    params.JobID,
			"cancelled": h.cancellations.Cancel(params.JobID),
		}, nil
	}
	switch request.Method {
	case "handshake", "capabilities.get", "health.get", "catalog.list", "catalog.refresh", "installed.list":
		if err := decodeNoParams(request.Params); err != nil {
			return nil, err
		}
	}

	jobContext, err := h.cancellations.Register(ctx, request.JobID)
	if err != nil {
		return nil, err
	}
	defer h.cancellations.Complete(request.JobID)

	var result any
	switch request.Method {
	case "handshake":
		result = map[string]any{
			"name":             "tapioca-control",
			"protocol_version": ProtocolVersion,
			"capabilities":     capabilities(),
		}
	case "capabilities.get":
		result = capabilities()
	case "health.get":
		now := h.dependencies.Now().UTC()
		uptime := now.Sub(h.startedAt.UTC())
		if uptime < 0 {
			uptime = 0
		}
		result = map[string]any{
			"status":           "ok",
			"name":             "tapioca-control",
			"control_version":  ControlVersion,
			"protocol_version": ProtocolVersion,
			"goos":             runtime.GOOS,
			"goarch":           runtime.GOARCH,
			"go_version":       runtime.Version(),
			"module_version":   buildModuleVersion(),
			"started_at":       h.startedAt.UTC().Format(time.RFC3339Nano),
			"uptime_ms":        uptime.Milliseconds(),
			"time":             now.Format(time.RFC3339Nano),
		}
	case "catalog.list":
		result, err = call(jobContext, h.dependencies.Catalog)
	case "catalog.refresh":
		result, err = call(jobContext, h.dependencies.RefreshCatalog)
	case "installed.list":
		result, err = call(jobContext, h.dependencies.Installed)
	default:
		result, err = h.handleFeature(jobContext, request)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func buildModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}

func capabilities() map[string]any {
	return map[string]any{
		"methods": []string{
			"handshake",
			"capabilities.get",
			"health.get",
			"system.info",
			"storage.info",
			"catalog.list",
			"catalog.refresh",
			"catalog.get",
			"installed.list",
			"model.pull",
			"model.remove",
			"server.start",
			"server.stop",
			"server.status",
			"chat.request",
			"chat.describe",
			"agent.describe",
			"creator.capabilities",
			"creator.catalog",
			"image.generate",
			"video.generate",
			"speech.generate",
			"voice.clone",
			"lora.list",
			"lora.inspect",
			"lora.pull",
			"lora.import",
			"job.cancel",
		},
		"events": []string{
			"job.started",
			"job.progress",
			"job.log",
			"job.completed",
			"job.failed",
		},
		"max_request_bytes": MaxRequestBytes,
		"max_concurrency":   DefaultMaxConcurrency,
	}
}

func call[T any](ctx context.Context, function func(context.Context) (T, error)) (T, *ProtocolError) {
	var zero T
	value, err := function(ctx)
	if err == nil {
		return value, nil
	}
	if ctx.Err() != nil {
		return zero, &ProtocolError{
			Code:      "job_cancelled",
			Message:   "job was cancelled",
			Retryable: false,
		}
	}
	return zero, protocolError(err)
}

func decodeParams(raw json.RawMessage, destination any) *ProtocolError {
	if len(raw) == 0 || string(raw) == "null" {
		return invalidRequest("params are required", nil)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return invalidRequest("params are invalid", err.Error())
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return invalidRequest("params must contain one JSON object", err.Error())
	}
	return nil
}

func decodeNoParams(raw json.RawMessage) *ProtocolError {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var params struct{}
	return decodeParams(raw, &params)
}

func loadCatalog(ctx context.Context) ([]CatalogModel, error) {
	refs := catalog.Refs()
	models := make([]CatalogModel, 0, len(refs))
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resolved, err := catalog.Resolve(ref)
		if err != nil {
			return nil, err
		}
		kind := resolved.Kind
		if kind == "" {
			kind = "text"
		}
		backend := resolved.Backend
		if backend == "" {
			backend = "llama.cpp"
		}
		models = append(models, CatalogModel{
			Name:       resolved.Name,
			Kind:       kind,
			Backend:    backend,
			Repo:       resolved.Repo,
			Size:       resolved.Size,
			Memory:     resolved.Memory,
			GPU:        resolved.GPU,
			Platforms:  normalizePlatforms(resolved.Platform),
			Languages:  resolved.Languages,
			Features:   resolved.Features,
			Width:      resolved.Width,
			Height:     resolved.Height,
			Steps:      resolved.Steps,
			Frames:     resolved.Frames,
			FPS:        resolved.FPS,
			Gated:      resolved.Gated,
			License:    resolved.License,
			LicenseURL: resolved.LicenseURL,
		})
	}
	return models, nil
}

func normalizePlatforms(value string) []string {
	lower := strings.ToLower(value)
	platforms := make([]string, 0, 3)
	for _, candidate := range []struct {
		name   string
		marker string
	}{
		{name: "windows", marker: "windows"},
		{name: "macos", marker: "macos"},
		{name: "linux", marker: "linux"},
	} {
		if strings.Contains(lower, candidate.marker) {
			platforms = append(platforms, candidate.name)
		}
	}
	return platforms
}

func loadInstalled(ctx context.Context) ([]InstalledModel, error) {
	registry, err := config.Load()
	if err != nil {
		return nil, err
	}
	models := make([]InstalledModel, 0, len(registry.Models))
	for _, model := range registry.Models {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		kind := model.Kind
		if kind == "" {
			kind = "text"
		}
		backend := model.Backend
		if backend == "" {
			backend = "llama.cpp"
		}
		models = append(models, InstalledModel{
			Name:     model.Name,
			Repo:     model.Repo,
			Filename: model.Filename,
			Path:     model.Path,
			Kind:     kind,
			Backend:  backend,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})
	return models, nil
}
