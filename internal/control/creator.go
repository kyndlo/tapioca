package control

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/carlos/tapioca/internal/adapter"
	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/imageruntime"
	"github.com/carlos/tapioca/internal/speechruntime"
	"github.com/carlos/tapioca/internal/videoruntime"
)

type ImageRunFunc func(context.Context, string, imageruntime.Request, io.Writer, io.Writer) error
type VideoRunFunc func(context.Context, string, videoruntime.Request, io.Writer, io.Writer) error
type SpeechRunFunc func(context.Context, string, speechruntime.Request, io.Writer, io.Writer) error

type LoRASelection struct {
	Reference string   `json:"reference"`
	File      string   `json:"file,omitempty"`
	Scale     *float64 `json:"scale,omitempty"`
}

type CreatorOutput struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	MIME      string `json:"mime"`
	Bytes     int64  `json:"bytes"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
}

type CreatorCatalogModel struct {
	CatalogModel
	Operation          string `json:"operation"`
	SupportsInputImage bool   `json:"supports_input_image"`
	RequiresInputImage bool   `json:"requires_input_image"`
	SupportsLoRA       bool   `json:"supports_lora"`
	Available          bool   `json:"available"`
	UnavailableReason  string `json:"unavailable_reason,omitempty"`
}

type ImageGenerateParams struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	NegativePrompt string          `json:"negative_prompt,omitempty"`
	OutputName     string          `json:"output_name,omitempty"`
	Width          int             `json:"width,omitempty"`
	Height         int             `json:"height,omitempty"`
	Steps          int             `json:"steps,omitempty"`
	Seed           uint64          `json:"seed,omitempty"`
	InputImages    []string        `json:"input_images,omitempty"`
	LoRAs          []LoRASelection `json:"loras,omitempty"`
}

type VideoGenerateParams struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	NegativePrompt string          `json:"negative_prompt,omitempty"`
	InputImage     string          `json:"input_image,omitempty"`
	OutputName     string          `json:"output_name,omitempty"`
	Width          int             `json:"width,omitempty"`
	Height         int             `json:"height,omitempty"`
	Frames         int             `json:"frames,omitempty"`
	Steps          int             `json:"steps,omitempty"`
	FPS            int             `json:"fps,omitempty"`
	Seed           uint64          `json:"seed,omitempty"`
	LoRAs          []LoRASelection `json:"loras,omitempty"`
}

type SpeechGenerateParams struct {
	Model       string `json:"model"`
	Text        string `json:"text"`
	VoiceSample string `json:"voice_sample,omitempty"`
	Transcript  string `json:"transcript,omitempty"`
	Language    string `json:"language,omitempty"`
	OutputName  string `json:"output_name,omitempty"`
}

func (h *Handler) handleCreator(
	ctx context.Context,
	request Request,
) (any, *ProtocolError, bool) {
	switch request.Method {
	case "creator.capabilities":
		if err := decodeNoParams(request.Params); err != nil {
			return nil, err, true
		}
		return creatorCapabilities(), nil, true
	case "creator.catalog":
		if err := decodeNoParams(request.Params); err != nil {
			return nil, err, true
		}
		models, err := loadCatalog(ctx)
		if err != nil {
			return nil, operationError(ctx, "catalog_failed", err), true
		}
		filtered := make([]CreatorCatalogModel, 0, len(models))
		for _, model := range models {
			if model.Kind == "image" || model.Kind == "video" || model.Kind == "speech" {
				item := CreatorCatalogModel{
					CatalogModel: model,
					Operation:    model.Kind + ".generate",
					Available:    true,
				}
				switch model.Kind {
				case "image":
					item.SupportsInputImage = model.Backend == "mflux" ||
						model.Backend == "diffusers"
					item.SupportsLoRA = model.Backend == "mflux" ||
						model.Backend == "diffusers"
				case "video":
					item.SupportsInputImage = true
					item.RequiresInputImage = strings.Contains(model.Name, "stable-video-diffusion")
					item.SupportsLoRA = !strings.Contains(model.Name, "stable-video-diffusion")
				case "speech":
					item.Operation = "speech.generate"
				}
				filtered = append(filtered, item)
			}
		}
		return filtered, nil, true
	case "image.generate":
		var params ImageGenerateParams
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err, true
		}
		result, err := h.generateImage(ctx, request.ID, params)
		return result, err, true
	case "video.generate":
		var params VideoGenerateParams
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err, true
		}
		result, err := h.generateVideo(ctx, request.ID, params)
		return result, err, true
	case "speech.generate", "voice.clone":
		var params SpeechGenerateParams
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err, true
		}
		result, err := h.generateSpeech(ctx, request.ID, params, request.Method == "voice.clone")
		return result, err, true
	case "lora.list":
		if err := decodeNoParams(request.Params); err != nil {
			return nil, err, true
		}
		result, err := listLoRAs()
		return result, err, true
	case "lora.inspect":
		var params struct {
			Reference string `json:"reference"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err, true
		}
		result, err := inspectLoRA(ctx, params.Reference)
		return result, err, true
	case "lora.pull":
		var params struct {
			Reference string `json:"reference"`
			File      string `json:"file,omitempty"`
			Force     bool   `json:"force,omitempty"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err, true
		}
		result, err := pullLoRA(params.Reference, params.File, params.Force)
		return result, err, true
	case "lora.import":
		var params struct {
			Path  string `json:"path"`
			Name  string `json:"name,omitempty"`
			Base  string `json:"base"`
			Force bool   `json:"force,omitempty"`
		}
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err, true
		}
		result, err := importLoRA(params.Path, params.Name, params.Base, params.Force)
		return result, err, true
	default:
		return nil, nil, false
	}
}

func creatorCapabilities() map[string]any {
	return map[string]any{
		"image": map[string]any{
			"available": true, "method": "image.generate",
			"supports_input_images": true, "supports_loras": true,
			"parameters": map[string]any{
				"width_height": "multiples of 8 from 64 through 4096",
				"steps":        "1 through 200; omit for the catalog default",
				"input_images": "regular PNG, JPEG, or WebP files up to 100 MiB each",
				"output_name":  "optional .png filename without directories",
			},
		},
		"video": map[string]any{
			"available": true, "method": "video.generate",
			"supports_input_image": true, "supports_loras": true,
			"parameters": map[string]any{
				"width_height": "multiples of 8 from 64 through 4096",
				"steps":        "1 through 200; omit for the catalog default",
				"frames": fmt.Sprintf(
					"1 through %d; omit for the catalog default", videoruntime.MaxVideoFrames,
				),
				"fps":         "1 through 60; omit for the catalog default",
				"input_image": "optional regular PNG, JPEG, or WebP file up to 100 MiB",
				"output_name": "optional .mp4 filename without directories",
			},
		},
		"speech": map[string]any{
			"available": true, "method": "speech.generate",
			"supports_voice_reference": true,
		},
		"voice_clone": map[string]any{
			"available": true, "method": "voice.clone",
			"requires_voice_reference": true,
		},
		"outputs": map[string]any{
			"binary_in_protocol":  false,
			"managed_local_paths": true,
		},
		"progress": map[string]any{
			"mode":                   "indeterminate",
			"numeric_when_available": false,
			"reason":                 "current image and video runtimes expose log streams but no numeric progress callback",
		},
	}
}

func (h *Handler) generateImage(
	ctx context.Context,
	requestID string,
	params ImageGenerateParams,
) (any, *ProtocolError) {
	model, resolved, protocolError := resolveCreatorModel(params.Model, "image")
	if protocolError != nil {
		return nil, protocolError
	}
	if strings.TrimSpace(params.Prompt) == "" {
		return nil, invalidParams("params.prompt is required", nil)
	}
	if params.Width == 0 {
		params.Width = resolved.Width
	}
	if params.Height == 0 {
		params.Height = resolved.Height
	}
	if params.Steps == 0 {
		params.Steps = resolved.Steps
	}
	if err := validateDimensions(params.Width, params.Height, params.Steps); err != nil {
		return nil, err
	}
	inputs, err := validateInputFiles(params.InputImages, imageExtensions, 100<<20)
	if err != nil {
		return nil, err
	}
	loras, err := resolveLoRAs(params.LoRAs, model.Name, model.Backend)
	if err != nil {
		return nil, err
	}
	output, err := managedOutputPath("images", requestID, params.OutputName, ".png")
	if err != nil {
		return nil, err
	}
	reportProgress(ctx, map[string]any{
		"stage": "starting", "determinate": false, "output": output,
	})
	logs := creatorLogWriter{ctx: ctx}
	cacheDir, cacheError := runtimeCacheDir()
	if cacheError != nil {
		return nil, cacheError
	}
	var guidanceScale *float64
	if resolved.GuidanceScaleSet {
		guidanceScale = &resolved.GuidanceScale
	}
	runError := h.dependencies.Image(ctx, cacheDir, imageruntime.Request{
		ModelPath: model.Path, Prompt: params.Prompt,
		NegativePrompt: params.NegativePrompt, Output: output,
		Width: params.Width, Height: params.Height, Steps: params.Steps,
		Seed: params.Seed, Backend: model.Backend, GuidanceScale: guidanceScale,
		InputImages: inputs, Adapters: loras,
	}, logs, logs)
	if runError != nil {
		_ = os.Remove(output)
		return nil, operationError(ctx, "image_generation_failed", runError)
	}
	return creatorOutput(output, "image", model.Name, h.dependencies.Now())
}

func (h *Handler) generateVideo(
	ctx context.Context,
	requestID string,
	params VideoGenerateParams,
) (any, *ProtocolError) {
	model, resolved, protocolError := resolveCreatorModel(params.Model, "video")
	if protocolError != nil {
		return nil, protocolError
	}
	if strings.TrimSpace(params.Prompt) == "" {
		return nil, invalidParams("params.prompt is required", nil)
	}
	if params.Width == 0 {
		params.Width = resolved.Width
	}
	if params.Height == 0 {
		params.Height = resolved.Height
	}
	if params.Steps == 0 {
		params.Steps = resolved.Steps
	}
	if params.Frames == 0 {
		params.Frames = resolved.Frames
	}
	if params.FPS == 0 {
		params.FPS = resolved.FPS
	}
	if err := validateVideo(params); err != nil {
		return nil, err
	}
	if (model.Backend == "comfy-h3-mps" || model.Backend == "comfy-h3-cuda") &&
		(params.Frames < 5 || (params.Frames-5)%17 != 0) {
		return nil, invalidParams("MiniMax-H3 frames must have the form 17n+5", nil)
	}
	var input string
	if params.InputImage != "" {
		inputs, err := validateInputFiles([]string{params.InputImage}, imageExtensions, 100<<20)
		if err != nil {
			return nil, err
		}
		input = inputs[0]
	}
	loras, err := resolveLoRAs(params.LoRAs, model.Name, model.Backend)
	if err != nil {
		return nil, err
	}
	output, err := managedOutputPath("videos", requestID, params.OutputName, ".mp4")
	if err != nil {
		return nil, err
	}
	reportProgress(ctx, map[string]any{
		"stage": "starting", "determinate": false, "output": output,
	})
	logs := creatorLogWriter{ctx: ctx}
	cacheDir, cacheError := runtimeCacheDir()
	if cacheError != nil {
		return nil, cacheError
	}
	runError := h.dependencies.Video(ctx, cacheDir, videoruntime.Request{
		ModelPath: model.Path, Prompt: params.Prompt,
		NegativePrompt: params.NegativePrompt, InputImage: input, Output: output,
		Width: params.Width, Height: params.Height, Frames: params.Frames,
		Steps: params.Steps, FPS: params.FPS, Seed: params.Seed,
		Backend: model.Backend, Adapters: loras,
	}, logs, logs)
	if runError != nil {
		_ = os.Remove(output)
		return nil, operationError(ctx, "video_generation_failed", runError)
	}
	return creatorOutput(output, "video", model.Name, h.dependencies.Now())
}

func runImage(
	ctx context.Context,
	cacheDir string,
	request imageruntime.Request,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return imageruntime.RunWithWriters(ctx, cacheDir, request, stdout, stderr)
}

func runVideo(
	ctx context.Context,
	cacheDir string,
	request videoruntime.Request,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return videoruntime.RunWithWriters(ctx, cacheDir, request, stdout, stderr)
}

func runSpeech(
	ctx context.Context,
	cacheDir string,
	request speechruntime.Request,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return speechruntime.RunWithWriters(ctx, cacheDir, request, stdout, stderr)
}

func (h *Handler) generateSpeech(
	ctx context.Context,
	requestID string,
	params SpeechGenerateParams,
	clone bool,
) (any, *ProtocolError) {
	model, _, protocolError := resolveCreatorModel(params.Model, "speech")
	if protocolError != nil {
		return nil, protocolError
	}
	params.Text = strings.TrimSpace(params.Text)
	if params.Text == "" {
		return nil, invalidParams("params.text is required", nil)
	}
	if len(params.Text) > 20_000 {
		return nil, invalidParams("params.text exceeds the 20,000 character limit", nil)
	}
	var voiceSample string
	if params.VoiceSample != "" {
		files, err := validateInputFiles(
			[]string{params.VoiceSample},
			map[string]bool{".wav": true, ".mp3": true, ".flac": true, ".m4a": true, ".ogg": true},
			100<<20,
		)
		if err != nil {
			return nil, err
		}
		voiceSample = files[0]
	}
	if clone && voiceSample == "" {
		return nil, invalidParams("params.voice_sample is required for voice cloning", nil)
	}
	if (model.Backend == "speech-qwen" || model.Backend == "speech-qwen-mlx" ||
		strings.Contains(model.Name, "chatterbox:nano")) && voiceSample == "" {
		return nil, invalidParams(model.Name+" requires a voice reference", nil)
	}
	output, err := managedOutputPath("audio", requestID, params.OutputName, ".wav")
	if err != nil {
		return nil, err
	}
	reportProgress(ctx, map[string]any{
		"stage": "starting", "determinate": false, "output": output,
	})
	logs := creatorLogWriter{ctx: ctx}
	cacheDir, cacheError := runtimeCacheDir()
	if cacheError != nil {
		return nil, cacheError
	}
	runError := h.dependencies.Speech(ctx, cacheDir, speechruntime.Request{
		ModelPath: model.Path, ModelName: model.Name, Text: params.Text,
		Output: output, VoiceSample: voiceSample, Transcript: params.Transcript,
		Language: params.Language, Backend: model.Backend,
	}, logs, logs)
	if runError != nil {
		_ = os.Remove(output)
		return nil, operationError(ctx, "speech_generation_failed", runError)
	}
	return creatorOutput(output, "audio", model.Name, h.dependencies.Now())
}

func resolveCreatorModel(
	name string,
	kind string,
) (config.Model, catalog.Resolved, *ProtocolError) {
	registry, err := config.Load()
	if err != nil {
		return config.Model{}, catalog.Resolved{}, operationError(context.Background(), "registry_failed", err)
	}
	model, ok := registry.Find(name)
	if !ok {
		return config.Model{}, catalog.Resolved{}, &ProtocolError{
			Code: "model_not_installed", Message: "model is not installed", Retryable: false,
		}
	}
	resolved, err := catalog.Resolve(model.Name)
	if err != nil {
		return config.Model{}, catalog.Resolved{}, &ProtocolError{
			Code: "model_not_found", Message: err.Error(), Retryable: false,
		}
	}
	if resolved.Kind != kind {
		return config.Model{}, catalog.Resolved{}, &ProtocolError{
			Code:      "incompatible_model",
			Message:   fmt.Sprintf("%s is a %s model, not a %s model", model.Name, resolved.Kind, kind),
			Retryable: false,
		}
	}
	if model.Kind == "" {
		model.Kind = resolved.Kind
	}
	if model.Backend == "" {
		model.Backend = resolved.Backend
	}
	home, homeError := config.Home()
	if homeError != nil {
		return config.Model{}, catalog.Resolved{}, operationError(
			context.Background(), "storage_failed", homeError,
		)
	}
	modelPath, pathError := filepath.Abs(model.Path)
	if pathError != nil || !withinRoot(filepath.Join(home, "models"), modelPath) {
		return config.Model{}, catalog.Resolved{}, &ProtocolError{
			Code:      "unsafe_model_path",
			Message:   "registered model path is outside the managed model store",
			Retryable: false,
		}
	}
	modelInfo, statError := os.Lstat(modelPath)
	if statError != nil || modelInfo.Mode()&os.ModeSymlink != 0 ||
		(!modelInfo.Mode().IsRegular() && !modelInfo.IsDir()) {
		return config.Model{}, catalog.Resolved{}, &ProtocolError{
			Code:      "unsafe_model_path",
			Message:   "registered model path must be a regular file or directory and not a symlink",
			Retryable: false,
		}
	}
	model.Path = modelPath
	return model, resolved, nil
}

func validateDimensions(width, height, steps int) *ProtocolError {
	if width < 64 || width > 4096 || height < 64 || height > 4096 ||
		width%8 != 0 || height%8 != 0 {
		return invalidParams("width and height must be multiples of 8 between 64 and 4096", nil)
	}
	if steps < 1 || steps > 200 {
		return invalidParams("steps must be between 1 and 200", nil)
	}
	return nil
}

func validateVideo(params VideoGenerateParams) *ProtocolError {
	if err := validateDimensions(params.Width, params.Height, params.Steps); err != nil {
		return err
	}
	if params.Frames < 1 || params.Frames > videoruntime.MaxVideoFrames {
		return invalidParams(fmt.Sprintf(
			"frames must be between 1 and %d", videoruntime.MaxVideoFrames,
		), nil)
	}
	if params.FPS < 1 || params.FPS > 60 {
		return invalidParams("fps must be between 1 and 60", nil)
	}
	return nil
}

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true,
}

func validateInputFiles(
	values []string,
	extensions map[string]bool,
	maxBytes int64,
) ([]string, *ProtocolError) {
	results := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			return nil, invalidParams("input path must not be empty", nil)
		}
		path, err := filepath.Abs(value)
		if err != nil {
			return nil, invalidParams("input path is invalid", err.Error())
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, invalidParams("input file is not accessible", err.Error())
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, invalidParams("input must be a regular file and not a symlink", map[string]any{"path": path})
		}
		if info.Size() > maxBytes {
			return nil, invalidParams("input file is too large", map[string]any{"path": path, "max_bytes": maxBytes})
		}
		if !extensions[strings.ToLower(filepath.Ext(path))] {
			return nil, invalidParams("input file extension is not supported", map[string]any{"path": path})
		}
		results = append(results, path)
	}
	return results, nil
}

func managedOutputPath(
	kind string,
	requestID string,
	outputName string,
	extension string,
) (string, *ProtocolError) {
	if outputName == "" {
		digest := sha256.Sum256([]byte(requestID))
		outputName = fmt.Sprintf("%x", digest[:8]) + extension
	}
	if filepath.Base(outputName) != outputName || outputName == "." ||
		strings.ContainsAny(outputName, `/\`) {
		return "", invalidParams("output_name must be a filename without directories", nil)
	}
	if !strings.EqualFold(filepath.Ext(outputName), extension) {
		return "", invalidParams("output_name must use "+extension, nil)
	}
	home, err := config.Home()
	if err != nil {
		return "", operationError(context.Background(), "storage_failed", err)
	}
	root := filepath.Join(home, "outputs", kind)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", operationError(context.Background(), "storage_failed", err)
	}
	output := filepath.Join(root, outputName)
	if !withinRoot(root, output) {
		return "", invalidParams("output path escapes the managed output directory", nil)
	}
	if _, err := os.Lstat(output); err == nil {
		return "", &ProtocolError{
			Code: "output_exists", Message: "output file already exists", Retryable: false,
			Details: map[string]any{"path": output},
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", operationError(context.Background(), "storage_failed", err)
	}
	return output, nil
}

func creatorOutput(
	path string,
	kind string,
	model string,
	now time.Time,
) (any, *ProtocolError) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		if err == nil {
			err = errors.New("runtime output is not a regular file")
		}
		return nil, operationError(context.Background(), "output_missing", err)
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return CreatorOutput{
		Path: path, Kind: kind, MIME: mimeType, Bytes: info.Size(),
		Model: model, CreatedAt: now.UTC().Format(time.RFC3339Nano),
	}, nil
}

func resolveLoRAs(
	selections []LoRASelection,
	baseName string,
	backend string,
) ([]adapter.Local, *ProtocolError) {
	if len(selections) > 8 {
		return nil, invalidParams("at most 8 LoRAs may be selected", nil)
	}
	home, err := config.Home()
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	locals := make([]adapter.Local, 0, len(selections))
	for _, selection := range selections {
		if selection.Scale != nil && (*selection.Scale < -4 || *selection.Scale > 4) {
			return nil, invalidParams("LoRA scale must be between -4 and 4", nil)
		}
		reference, err := adapter.Parse(selection.Reference)
		if err != nil {
			return nil, invalidParams("invalid LoRA reference", err.Error())
		}
		local, err := adapter.Resolve(http.DefaultClient, home, reference, selection.File, selection.Scale)
		if err != nil {
			return nil, &ProtocolError{
				Code: "lora_not_found", Message: err.Error(), Retryable: false,
			}
		}
		if err := adapter.ValidateCompatibility(baseName, backend, local); err != nil {
			return nil, &ProtocolError{
				Code: "incompatible_lora", Message: err.Error(), Retryable: false,
			}
		}
		info, err := os.Lstat(local.Path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, &ProtocolError{
				Code:      "lora_not_installed",
				Message:   "LoRA must be installed as a regular local file before generation",
				Retryable: false, Details: map[string]any{"path": local.Path},
			}
		}
		locals = append(locals, local)
	}
	return locals, nil
}

func listLoRAs() (any, *ProtocolError) {
	home, err := config.Home()
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	results, err := adapter.List(home)
	if err != nil {
		return nil, operationError(context.Background(), "lora_discovery_failed", err)
	}
	return results, nil
}

func inspectLoRA(ctx context.Context, value string) (any, *ProtocolError) {
	reference, err := adapter.Parse(value)
	if err != nil {
		return nil, invalidParams("invalid LoRA reference", err.Error())
	}
	client := &http.Client{Timeout: 30 * time.Second}
	metadata, err := adapter.Inspect(client, reference)
	if err != nil {
		return nil, operationError(ctx, "lora_inspection_failed", err)
	}
	return metadata, nil
}

func importLoRA(path, name, base string, force bool) (any, *ProtocolError) {
	home, err := config.Home()
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	local, err := adapter.Import(home, path, name, base, force)
	if err != nil {
		return nil, operationError(context.Background(), "lora_import_failed", err)
	}
	return local, nil
}

func pullLoRA(value, file string, force bool) (any, *ProtocolError) {
	reference, err := adapter.Parse(value)
	if err != nil {
		return nil, invalidParams("invalid LoRA reference", err.Error())
	}
	home, err := config.Home()
	if err != nil {
		return nil, operationError(context.Background(), "storage_failed", err)
	}
	local, err := adapter.Resolve(http.DefaultClient, home, reference, file, nil)
	if err != nil {
		return nil, operationError(context.Background(), "lora_pull_failed", err)
	}
	if err := adapter.Pull(http.DefaultClient, local, force); err != nil {
		return nil, operationError(context.Background(), "lora_pull_failed", err)
	}
	return local, nil
}

type creatorLogWriter struct {
	ctx context.Context
}

func (w creatorLogWriter) Write(data []byte) (int, error) {
	message := strings.TrimSpace(string(data))
	if message != "" {
		reportLog(w.ctx, message)
	}
	return len(data), nil
}

func runtimeCacheDir() (string, *ProtocolError) {
	home, err := config.Home()
	if err != nil {
		return "", operationError(context.Background(), "storage_failed", err)
	}
	return filepath.Join(home, "runtime"), nil
}
