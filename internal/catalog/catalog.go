package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Model struct {
	Context                   int                   `json:"context,omitempty"`
	Name                      string                `json:"name"`
	Repo                      string                `json:"repo,omitempty"`
	Files                     map[string]string     `json:"files"`
	Template                  string                `json:"template,omitempty"`
	Kind                      string                `json:"kind,omitempty"`
	Backends                  map[string]string     `json:"backends,omitempty"`
	Repos                     map[string]string     `json:"repos,omitempty"`
	Default                   string                `json:"default"`
	PlatformDefaults          map[string]string     `json:"platform_defaults,omitempty"`
	Width                     int                   `json:"width,omitempty"`
	Height                    int                   `json:"height,omitempty"`
	Steps                     int                   `json:"steps,omitempty"`
	Frames                    int                   `json:"frames,omitempty"`
	FPS                       int                   `json:"fps,omitempty"`
	DurationSeconds           int                   `json:"duration_seconds,omitempty"`
	MaxDuration               int                   `json:"max_duration_seconds,omitempty"`
	SampleRate                int                   `json:"sample_rate,omitempty"`
	Sizes                     map[string]string     `json:"sizes"`
	Memory                    map[string]string     `json:"memory,omitempty"`
	GPUs                      map[string]string     `json:"gpus,omitempty"`
	Languages                 map[string]string     `json:"languages,omitempty"`
	Features                  map[string]string     `json:"features,omitempty"`
	Artifacts                 map[string][]Artifact `json:"artifacts,omitempty"`
	Downloads                 map[string]Download   `json:"downloads,omitempty"`
	GuidanceScale             float64               `json:"guidance_scale,omitempty"`
	GuidanceScaleSet          bool                  `json:"guidance_scale_set,omitempty"`
	Gated                     bool                  `json:"gated,omitempty"`
	LicenseAcceptanceRequired bool                  `json:"license_acceptance_required,omitempty"`
	License                   string                `json:"license,omitempty"`
	LicenseURL                string                `json:"license_url,omitempty"`
}

// Artifact is one explicitly selected file in a multi-repository model bundle.
// Target is relative to the model directory and uses forward slashes.
type Artifact struct {
	Repo     string `json:"repo"`
	Filename string `json:"filename"`
	Target   string `json:"target"`
	Download
}

// Download pins an artifact to immutable upstream content. Empty metadata keeps
// legacy catalogs working; new integrations should specify all three fields.
type Download struct {
	Revision  string `json:"revision,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

func (d Download) Ref() string {
	if d.Revision != "" {
		return d.Revision
	}
	return "main"
}

func (d Download) Validate() error {
	if d.Revision != "" && !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(d.Revision) {
		return errors.New("revision must be a full lowercase Git commit SHA")
	}
	if d.SHA256 != "" && !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(d.SHA256) {
		return errors.New("sha256 must contain 64 lowercase hexadecimal characters")
	}
	if d.SizeBytes < 0 {
		return errors.New("size_bytes must not be negative")
	}
	return nil
}

const (
	manifestSchemaVersion = 1
	defaultManifestURL    = "https://raw.githubusercontent.com/kyndlo/tapioca/main/catalog/catalog.json"
	defaultChecksumURL    = defaultManifestURL + ".sha256"
)

type Manifest struct {
	Schema      int              `json:"schema"`
	GeneratedAt time.Time        `json:"generated_at"`
	Models      map[string]Model `json:"models"`
}

type RefreshResult struct {
	Path   string `json:"path"`
	Models int    `json:"models"`
	SHA256 string `json:"sha256"`
}

var cachedOverridePath string

var safeName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var safeRepoPart = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func SetOverridePathForTest(path string) func() {
	previous := cachedOverridePath
	cachedOverridePath = path
	return func() { cachedOverridePath = previous }
}

func cachePath() string {
	if cachedOverridePath != "" {
		return cachedOverridePath
	}
	if home := os.Getenv("TAPIOCA_HOME"); home != "" {
		return filepath.Join(home, "catalog.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tapioca", "catalog.json")
}

func activeModels() map[string]Model {
	merged := cloneModels(builtInModels)
	path := cachePath()
	if path == "" {
		return merged
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return merged
	}
	manifest, err := decodeManifest(data)
	if err != nil {
		return merged
	}
	for name, model := range manifest.Models {
		merged[strings.ToLower(name)] = model
	}
	return merged
}

func cloneModels(source map[string]Model) map[string]Model {
	result := make(map[string]Model, len(source))
	for name, model := range source {
		result[name] = model
	}
	return result
}

func BuiltInManifest() Manifest {
	return Manifest{
		Schema: manifestSchemaVersion, GeneratedAt: time.Now().UTC(),
		Models: cloneModels(builtInModels),
	}
}

func EncodeBuiltInManifest(generatedAt time.Time) ([]byte, error) {
	manifest := BuiltInManifest()
	manifest.GeneratedAt = generatedAt.UTC()
	return json.MarshalIndent(manifest, "", "  ")
}

// ValidateManifest validates a published catalog without activating it. It is
// used by CI so the remote catalog may add recipes independently of the binary
// while remaining constrained to runtimes and paths Tapioca already supports.
func ValidateManifest(data []byte) error {
	_, err := decodeManifest(data)
	return err
}

func Refresh(ctx context.Context) (RefreshResult, error) {
	manifestURL := os.Getenv("TAPIOCA_CATALOG_URL")
	if manifestURL == "" {
		manifestURL = defaultManifestURL
	}
	checksumURL := os.Getenv("TAPIOCA_CATALOG_CHECKSUM_URL")
	if checksumURL == "" {
		checksumURL = manifestURL + ".sha256"
		if manifestURL == defaultManifestURL {
			checksumURL = defaultChecksumURL
		}
	}
	manifestData, err := fetch(ctx, manifestURL, 8<<20)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("download catalog: %w", err)
	}
	checksumData, err := fetch(ctx, checksumURL, 4096)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("download catalog checksum: %w", err)
	}
	expected := strings.Fields(string(checksumData))
	if len(expected) == 0 || len(expected[0]) != sha256.Size*2 {
		return RefreshResult{}, errors.New("catalog checksum is invalid")
	}
	digest := sha256.Sum256(manifestData)
	actual := hex.EncodeToString(digest[:])
	if !strings.EqualFold(expected[0], actual) {
		return RefreshResult{}, errors.New("catalog checksum did not match")
	}
	manifest, err := decodeManifest(manifestData)
	if err != nil {
		return RefreshResult{}, err
	}
	path := cachePath()
	if path == "" {
		return RefreshResult{}, errors.New("cannot determine Tapioca catalog path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return RefreshResult{}, err
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(path), ".catalog-*.tmp")
	if err != nil {
		return RefreshResult{}, err
	}
	temporary := temporaryFile.Name()
	defer os.Remove(temporary)
	if err := temporaryFile.Chmod(0o644); err != nil {
		temporaryFile.Close()
		return RefreshResult{}, err
	}
	if _, err := temporaryFile.Write(append(manifestData, '\n')); err != nil {
		temporaryFile.Close()
		return RefreshResult{}, err
	}
	if err := temporaryFile.Close(); err != nil {
		return RefreshResult{}, err
	}
	if err := os.Rename(temporary, path); err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{Path: path, Models: len(manifest.Models), SHA256: actual}, nil
}

func fetch(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("response exceeds size limit")
	}
	return data, nil
}

func decodeManifest(data []byte) (Manifest, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("invalid catalog manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Manifest{}, errors.New("invalid catalog manifest: trailing JSON content")
	}
	if manifest.Schema != manifestSchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported catalog schema %d", manifest.Schema)
	}
	if len(manifest.Models) == 0 || len(manifest.Models) > 1000 {
		return Manifest{}, errors.New("catalog must contain between 1 and 1000 models")
	}
	if manifest.GeneratedAt.IsZero() || manifest.GeneratedAt.After(time.Now().UTC().Add(24*time.Hour)) {
		return Manifest{}, errors.New("catalog generated_at is missing or invalid")
	}
	for name, model := range manifest.Models {
		if err := validateModel(name, model); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func validateModel(name string, model Model) error {
	if model.Context < 0 || model.Context > 1048576 {
		return fmt.Errorf("catalog model %s has invalid context", name)
	}
	if !safeName.MatchString(name) || model.Name != name {
		return fmt.Errorf("invalid catalog model name %q", name)
	}
	if len(model.Files) == 0 || model.Default == "" {
		return fmt.Errorf("catalog model %s requires files and a default variant", name)
	}
	if _, ok := model.Files[model.Default]; !ok {
		return fmt.Errorf("catalog model %s has unknown default variant %q", name, model.Default)
	}
	for platform, variant := range model.PlatformDefaults {
		if platform == "" {
			return fmt.Errorf("catalog model %s has an empty platform default", name)
		}
		if _, ok := model.Files[variant]; !ok {
			return fmt.Errorf("catalog model %s has platform default for unknown variant %q", name, variant)
		}
	}
	if model.Repo != "" && !validRepo(model.Repo) {
		return fmt.Errorf("catalog model %s has invalid repository", name)
	}
	allowedBackends := map[string]bool{
		"": true, "mlx": true, "mlx-vlm": true, "mlx-video": true,
		"mflux": true, "diffusers": true, "diffusers-mps": true,
		"diffusers-video": true, "diffusers-ltx2": true,
		"onnx-directml": true, "onnx-cpu": true,
		"comfy-h3-mps": true, "comfy-h3-cuda": true,
		"speech-chatterbox": true, "speech-qwen": true, "speech-qwen-mlx": true,
		"speech-audio8-onnx": true, "speech-pocket-tts": true, "speech-sopro": true,
		"speech-audio8": true, "speech-audio8-mlx": true,
		"stable-audio3": true,
	}
	for variant := range model.Files {
		if !safeName.MatchString(variant) {
			return fmt.Errorf("catalog model %s has invalid variant %q", name, variant)
		}
		if !allowedBackends[model.Backends[variant]] {
			return fmt.Errorf("catalog model %s has unsupported backend %q", name, model.Backends[variant])
		}
		if repo := model.Repos[variant]; repo != "" && !validRepo(repo) {
			return fmt.Errorf("catalog model %s has invalid variant repository", name)
		}
		if file := model.Files[variant]; file != "" && !validRelativePath(file) {
			return fmt.Errorf("catalog model %s has unsafe file path", name)
		}
	}
	for label, values := range map[string]map[string]string{
		"backend": model.Backends, "repository": model.Repos, "size": model.Sizes,
		"memory": model.Memory, "GPU": model.GPUs, "language": model.Languages,
		"feature": model.Features,
	} {
		for variant := range values {
			if _, ok := model.Files[variant]; !ok {
				return fmt.Errorf("catalog model %s has %s metadata for unknown variant %q", name, label, variant)
			}
		}
	}
	for variant, artifacts := range model.Artifacts {
		if _, ok := model.Files[variant]; !ok {
			return fmt.Errorf("catalog model %s has artifacts for unknown variant %q", name, variant)
		}
		for _, artifact := range artifacts {
			if err := artifact.Download.Validate(); err != nil {
				return fmt.Errorf("catalog model %s artifact: %w", name, err)
			}
			if !validRepo(artifact.Repo) || !validRelativePath(artifact.Filename) ||
				!validRelativePath(artifact.Target) {
				return fmt.Errorf("catalog model %s has an unsafe artifact", name)
			}
		}
	}
	for variant, download := range model.Downloads {
		file, ok := model.Files[variant]
		if !ok {
			return fmt.Errorf("catalog model %s download has unknown variant %q", name, variant)
		}
		if file == "" && (download.Revision == "" || download.SHA256 != "" || download.SizeBytes != 0) {
			return fmt.Errorf("catalog model %s snapshot download %q requires only a pinned revision", name, variant)
		}
		if err := download.Validate(); err != nil {
			return fmt.Errorf("catalog model %s download: %w", name, err)
		}
	}
	if (model.Gated || model.LicenseAcceptanceRequired) &&
		(model.License == "" || !strings.HasPrefix(model.LicenseURL, "https://")) {
		return fmt.Errorf("restricted catalog model %s requires license metadata", name)
	}
	return nil
}

func validRepo(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && safeRepoPart.MatchString(parts[0]) && safeRepoPart.MatchString(parts[1])
}

func validRelativePath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") ||
		strings.Contains(value, ":") {
		return false
	}
	for _, component := range strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '\\' }) {
		if component == ".." || component == "." || component == "" {
			return false
		}
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	return !filepath.IsAbs(clean) && clean != "." && clean != ".." &&
		!strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func flux2KleinFP8Artifacts() []Artifact {
	const base = "black-forest-labs/FLUX.2-klein-4B"
	artifacts := []Artifact{
		{Repo: base, Filename: "model_index.json", Target: "model_index.json"},
		{Repo: base, Filename: "scheduler/scheduler_config.json", Target: "scheduler/scheduler_config.json"},
		{Repo: base, Filename: "text_encoder/config.json", Target: "text_encoder/config.json"},
		{Repo: base, Filename: "text_encoder/generation_config.json", Target: "text_encoder/generation_config.json"},
		{Repo: base, Filename: "text_encoder/model-00001-of-00002.safetensors", Target: "text_encoder/model-00001-of-00002.safetensors"},
		{Repo: base, Filename: "text_encoder/model-00002-of-00002.safetensors", Target: "text_encoder/model-00002-of-00002.safetensors"},
		{Repo: base, Filename: "text_encoder/model.safetensors.index.json", Target: "text_encoder/model.safetensors.index.json"},
		{Repo: base, Filename: "tokenizer/added_tokens.json", Target: "tokenizer/added_tokens.json"},
		{Repo: base, Filename: "tokenizer/chat_template.jinja", Target: "tokenizer/chat_template.jinja"},
		{Repo: base, Filename: "tokenizer/merges.txt", Target: "tokenizer/merges.txt"},
		{Repo: base, Filename: "tokenizer/special_tokens_map.json", Target: "tokenizer/special_tokens_map.json"},
		{Repo: base, Filename: "tokenizer/tokenizer.json", Target: "tokenizer/tokenizer.json"},
		{Repo: base, Filename: "tokenizer/tokenizer_config.json", Target: "tokenizer/tokenizer_config.json"},
		{Repo: base, Filename: "tokenizer/vocab.json", Target: "tokenizer/vocab.json"},
		{Repo: base, Filename: "transformer/config.json", Target: "transformer/config.json"},
		{Repo: base, Filename: "vae/config.json", Target: "vae/config.json"},
		{Repo: base, Filename: "vae/diffusion_pytorch_model.safetensors", Target: "vae/diffusion_pytorch_model.safetensors"},
		{
			Repo: "black-forest-labs/FLUX.2-klein-4b-fp8", Filename: "flux-2-klein-4b-fp8.safetensors",
			Target: "transformer/diffusion_pytorch_model.safetensors",
		},
	}
	return artifacts
}

func ltx25DiffusersArtifacts() []Artifact {
	const repo = "Lightricks/LTX-2.5-Diffusers"
	files := []string{
		"model_index.json",
		"modular_model_index.json",
		"audio_vae/config.json",
		"audio_vae/diffusion_pytorch_model.safetensors",
		"connectors/config.json",
		"connectors/diffusion_pytorch_model.safetensors",
		"diffusion_decoder/config.json",
		"diffusion_decoder/diffusion_pytorch_model.safetensors",
		"duration_head/config.json",
		"duration_head/diffusion_pytorch_model.safetensors",
		"latent_upsampler/config.json",
		"latent_upsampler/diffusion_pytorch_model.safetensors",
		"scheduler/scheduler_config.json",
		"temporal_latent_upsampler/config.json",
		"temporal_latent_upsampler/diffusion_pytorch_model.safetensors",
		"text_encoder/config.json",
		"text_encoder/generation_config.json",
		"text_encoder/model-00001-of-00005.safetensors",
		"text_encoder/model-00002-of-00005.safetensors",
		"text_encoder/model-00003-of-00005.safetensors",
		"text_encoder/model-00004-of-00005.safetensors",
		"text_encoder/model-00005-of-00005.safetensors",
		"text_encoder/model.safetensors.index.json",
		"tokenizer/chat_template.jinja",
		"tokenizer/tokenizer.json",
		"tokenizer/tokenizer_config.json",
		"vae/config.json",
		"vae/diffusion_pytorch_model.safetensors",
		"vocoder/config.json",
		"vocoder/diffusion_pytorch_model.safetensors",
	}
	artifacts := make([]Artifact, 0, len(files)+6)
	for _, filename := range files {
		artifacts = append(artifacts, Artifact{Repo: repo, Filename: filename, Target: filename})
	}
	artifacts = append(artifacts,
		Artifact{Repo: repo, Filename: "transformer_full/config.json", Target: "transformer/config.json"},
		Artifact{Repo: repo, Filename: "transformer_full/diffusion_pytorch_model-00001-of-00004.safetensors", Target: "transformer/diffusion_pytorch_model-00001-of-00004.safetensors"},
		Artifact{Repo: repo, Filename: "transformer_full/diffusion_pytorch_model-00002-of-00004.safetensors", Target: "transformer/diffusion_pytorch_model-00002-of-00004.safetensors"},
		Artifact{Repo: repo, Filename: "transformer_full/diffusion_pytorch_model-00003-of-00004.safetensors", Target: "transformer/diffusion_pytorch_model-00003-of-00004.safetensors"},
		Artifact{Repo: repo, Filename: "transformer_full/diffusion_pytorch_model-00004-of-00004.safetensors", Target: "transformer/diffusion_pytorch_model-00004-of-00004.safetensors"},
		Artifact{Repo: repo, Filename: "transformer_full/diffusion_pytorch_model.safetensors.index.json", Target: "transformer/diffusion_pytorch_model.safetensors.index.json"},
	)
	return artifacts
}

var builtInModels = map[string]Model{
	"granite-4.2":   graniteModel(),
	"minicpm5-2b":   miniCPM5Model(),
	"spark-x2.5-4b": sparkX25Model(),
	"chatterbox": {
		Name:    "chatterbox",
		Repo:    "ResembleAI/chatterbox",
		Kind:    "speech",
		Default: "multilingual",
		Files: map[string]string{
			"nano":         "",
			"multilingual": "",
		},
		Backends: map[string]string{
			"nano":         "speech-chatterbox",
			"multilingual": "speech-chatterbox",
		},
		Repos: map[string]string{
			"nano":         "ResembleAI/chatterbox-nano",
			"multilingual": "ResembleAI/chatterbox",
		},
		Sizes: map[string]string{
			"nano":         "~1 GiB",
			"multilingual": "~4 GiB",
		},
		Memory: map[string]string{
			"nano":         "8 GiB min; 12 GiB recommended",
			"multilingual": "12 GiB min; 16 GiB recommended",
		},
		GPUs: map[string]string{
			"nano":         "CPU, Apple MPS, or NVIDIA CUDA",
			"multilingual": "CPU, Apple MPS, or NVIDIA CUDA",
		},
		Languages: map[string]string{
			"nano":         "English",
			"multilingual": "23+ languages",
		},
		Features: map[string]string{
			"nano":         "voice cloning, paralinguistic tags, watermarked",
			"multilingual": "voice cloning, multilingual, emotion control, watermarked",
		},
	},
	"qwen3-tts": {
		Name:    "qwen3-tts",
		Kind:    "speech",
		Default: "0.6b",
		Files: map[string]string{
			"0.6b":                  "",
			"0.6b-mlx":              "",
			"1.7b-base":             "",
			"1.7b-base-mlx":         "",
			"1.7b-custom-voice":     "",
			"1.7b-custom-voice-mlx": "",
			"1.7b-voice-design":     "",
			"1.7b-voice-design-mlx": "",
		},
		Backends: map[string]string{
			"0.6b":                  "speech-qwen",
			"0.6b-mlx":              "speech-qwen-mlx",
			"1.7b-base":             "speech-qwen",
			"1.7b-base-mlx":         "speech-qwen-mlx",
			"1.7b-custom-voice":     "speech-qwen",
			"1.7b-custom-voice-mlx": "speech-qwen-mlx",
			"1.7b-voice-design":     "speech-qwen",
			"1.7b-voice-design-mlx": "speech-qwen-mlx",
		},
		Repos: map[string]string{
			"0.6b":                  "Qwen/Qwen3-TTS-12Hz-0.6B-Base",
			"0.6b-mlx":              "mlx-community/Qwen3-TTS-12Hz-0.6B-Base-bf16",
			"1.7b-base":             "Qwen/Qwen3-TTS-12Hz-1.7B-Base",
			"1.7b-base-mlx":         "mlx-community/Qwen3-TTS-12Hz-1.7B-Base-bf16",
			"1.7b-custom-voice":     "Qwen/Qwen3-TTS-12Hz-1.7B-CustomVoice",
			"1.7b-custom-voice-mlx": "mlx-community/Qwen3-TTS-12Hz-1.7B-CustomVoice-bf16",
			"1.7b-voice-design":     "Qwen/Qwen3-TTS-12Hz-1.7B-VoiceDesign",
			"1.7b-voice-design-mlx": "mlx-community/Qwen3-TTS-12Hz-1.7B-VoiceDesign-bf16",
		},
		Sizes: map[string]string{
			"0.6b":                  "~2.5 GiB",
			"0.6b-mlx":              "~2.5 GiB",
			"1.7b-base":             "~4.3 GiB",
			"1.7b-base-mlx":         "~4.3 GiB",
			"1.7b-custom-voice":     "~4.3 GiB",
			"1.7b-custom-voice-mlx": "~4.3 GiB",
			"1.7b-voice-design":     "~4.3 GiB",
			"1.7b-voice-design-mlx": "~4.3 GiB",
		},
		Memory: map[string]string{
			"0.6b":                  "12 GiB min; 16 GiB recommended",
			"0.6b-mlx":              "12 GiB min; 16 GiB recommended",
			"1.7b-base":             "16 GiB min; 24 GiB recommended",
			"1.7b-base-mlx":         "16 GiB min; 24 GiB recommended",
			"1.7b-custom-voice":     "16 GiB min; 24 GiB recommended",
			"1.7b-custom-voice-mlx": "16 GiB min; 24 GiB recommended",
			"1.7b-voice-design":     "16 GiB min; 24 GiB recommended",
			"1.7b-voice-design-mlx": "16 GiB min; 24 GiB recommended",
		},
		GPUs: map[string]string{
			"0.6b":                  "NVIDIA CUDA recommended; CPU supported",
			"0.6b-mlx":              "Apple Silicon GPU",
			"1.7b-base":             "NVIDIA CUDA recommended; CPU supported",
			"1.7b-base-mlx":         "Apple Silicon GPU",
			"1.7b-custom-voice":     "NVIDIA CUDA recommended; CPU supported",
			"1.7b-custom-voice-mlx": "Apple Silicon GPU",
			"1.7b-voice-design":     "NVIDIA CUDA recommended; CPU supported",
			"1.7b-voice-design-mlx": "Apple Silicon GPU",
		},
		Languages: map[string]string{
			"0.6b":                  "10 languages",
			"0.6b-mlx":              "10 languages",
			"1.7b-base":             "10 languages",
			"1.7b-base-mlx":         "10 languages",
			"1.7b-custom-voice":     "10 languages",
			"1.7b-custom-voice-mlx": "10 languages",
			"1.7b-voice-design":     "10 languages",
			"1.7b-voice-design-mlx": "10 languages",
		},
		Features: map[string]string{
			"0.6b":                  "3-second voice cloning, streaming architecture",
			"0.6b-mlx":              "3-second voice cloning, Apple Silicon optimized",
			"1.7b-base":             "higher-quality 3-second voice cloning, streaming architecture",
			"1.7b-base-mlx":         "higher-quality voice cloning, Apple Silicon optimized",
			"1.7b-custom-voice":     "9 built-in speakers, instruction-based emotion and style",
			"1.7b-custom-voice-mlx": "9 built-in speakers, instruction-based emotion and style, Apple Silicon optimized",
			"1.7b-voice-design":     "natural-language voice design without a reference recording",
			"1.7b-voice-design-mlx": "natural-language voice design without a reference recording, Apple Silicon optimized",
		},
		License:    "Apache-2.0",
		LicenseURL: "https://huggingface.co/Qwen/Qwen3-TTS-12Hz-1.7B-Base",
	},
	"audio8-tts": {
		Name:    "audio8-tts",
		Kind:    "speech",
		Default: "0.6b",
		PlatformDefaults: map[string]string{
			"darwin/arm64": "0.6b-mlx",
		},
		Files: map[string]string{
			"0.6b":     "",
			"0.6b-mlx": "",
		},
		Backends: map[string]string{
			"0.6b":     "speech-audio8",
			"0.6b-mlx": "speech-audio8-mlx",
		},
		Repos: map[string]string{
			"0.6b":     "Audio8/Audio8-TTS-Preview-0.6b",
			"0.6b-mlx": "mlx-community/Audio8-TTS-Preview-0.6b-bf16",
		},
		Sizes: map[string]string{
			"0.6b":     "~2.4 GiB",
			"0.6b-mlx": "~2.4 GiB",
		},
		Memory: map[string]string{
			"0.6b":     "8 GiB min; 12 GiB recommended",
			"0.6b-mlx": "8 GiB min; 12 GiB recommended",
		},
		GPUs: map[string]string{
			"0.6b":     "NVIDIA CUDA recommended; CPU supported",
			"0.6b-mlx": "Apple Silicon GPU",
		},
		Languages: map[string]string{
			"0.6b":     "11 languages",
			"0.6b-mlx": "11 languages",
		},
		Features: map[string]string{
			"0.6b":     "preview, 44.1 kHz speech, zero-shot voice cloning, default voice",
			"0.6b-mlx": "preview, 44.1 kHz speech, zero-shot voice cloning, Apple Silicon optimized",
		},
		License:    "Apache-2.0",
		LicenseURL: "https://huggingface.co/Audio8/Audio8-TTS-Preview-0.6b",
	},
	"stable-audio-3": {
		Name:            "stable-audio-3",
		Kind:            "audio",
		Default:         "small-music",
		DurationSeconds: 30,
		MaxDuration:     120,
		SampleRate:      44100,
		Steps:           8,
		Gated:           true,
		License:         "Stability AI Community License",
		LicenseURL:      "https://huggingface.co/collections/stabilityai/stable-audio-3",
		Files: map[string]string{
			"small-music": "",
			"small-sfx":   "",
		},
		Backends: map[string]string{
			"small-music": "stable-audio3",
			"small-sfx":   "stable-audio3",
		},
		Repos: map[string]string{
			"small-music": "stabilityai/stable-audio-3-small-music",
			"small-sfx":   "stabilityai/stable-audio-3-small-sfx",
		},
		Sizes: map[string]string{
			"small-music": "~3.3 GiB",
			"small-sfx":   "~3.3 GiB",
		},
		Memory: map[string]string{
			"small-music": "16 GiB min; 24 GiB recommended",
			"small-sfx":   "16 GiB min; 24 GiB recommended",
		},
		GPUs: map[string]string{
			"small-music": "Apple Silicon MPS, NVIDIA CUDA, or CPU",
			"small-sfx":   "Apple Silicon MPS, NVIDIA CUDA, or CPU",
		},
		Languages: map[string]string{
			"small-music": "English prompts",
			"small-sfx":   "English prompts",
		},
		Features: map[string]string{
			"small-music": "text-to-music, 44.1 kHz stereo, variable duration up to 120 seconds",
			"small-sfx":   "text-to-sound-effects, 44.1 kHz stereo, variable duration up to 120 seconds",
		},
	},
	"glm-4.7-flash": {
		Name:    "glm-4.7-flash",
		Repo:    "ggml-org/GLM-4.7-Flash-GGUF",
		Default: "q4_k_m",
		Files: map[string]string{
			"q4_k_m": "GLM-4.7-Flash-Q4_K.gguf",
			"q8_0":   "GLM-4.7-Flash-Q8_0.gguf",
		},
		Sizes: map[string]string{
			"q4_k_m": "~17 GiB",
			"q8_0":   "~30 GiB",
		},
	},
	"qwen-image-flash": {
		Name:    "qwen-image-flash",
		Repo:    "mlx-community/Qwen-Image-Flash-8bit",
		Kind:    "image",
		Default: "int8",
		Width:   1024,
		Height:  1024,
		Steps:   4,
		Files: map[string]string{
			"int8": "",
			"8bit": "",
			"bf16": "",
		},
		Backends: map[string]string{
			"int8": "mlx",
			"8bit": "mlx",
			"bf16": "diffusers",
		},
		Sizes: map[string]string{
			"int8": "~28 GiB",
			"8bit": "~28 GiB",
			"bf16": "~58 GiB",
		},
		Memory: map[string]string{
			"int8": "48 GiB min; 64 GiB recommended",
			"8bit": "48 GiB min; 64 GiB recommended",
			"bf16": "64 GiB min; 96 GiB recommended",
		},
		GPUs: map[string]string{
			"int8": "Apple Silicon GPU",
			"8bit": "Apple Silicon GPU",
			"bf16": "NVIDIA Ampere+, 16 GiB VRAM with offload",
		},
	},
	"qwen-image-2.1": {
		Name:                      "qwen-image-2.1",
		Repo:                      "Qwen/Qwen-Image-2.1",
		Kind:                      "image",
		Default:                   "bf16-cuda",
		Width:                     1024,
		Height:                    1024,
		Steps:                     40,
		Files:                     map[string]string{"bf16-cuda": ""},
		Backends:                  map[string]string{"bf16-cuda": "diffusers"},
		Sizes:                     map[string]string{"bf16-cuda": "~31 GiB"},
		Memory:                    map[string]string{"bf16-cuda": "48 GiB min; 64 GiB recommended"},
		GPUs:                      map[string]string{"bf16-cuda": "NVIDIA CUDA, 24 GiB+ VRAM; CPU offload recommended"},
		Languages:                 map[string]string{"bf16-cuda": "multilingual prompts and typography"},
		Features:                  map[string]string{"bf16-cuda": "text-to-image, image editing, native RGBA transparency, up to 10 references"},
		Downloads:                 map[string]Download{"bf16-cuda": {Revision: "d26bb61231c349cf6b7896fa83353113880e1ba3"}},
		LicenseAcceptanceRequired: true,
		License:                   "Qwen Research License Agreement (non-commercial only)",
		LicenseURL:                "https://huggingface.co/Qwen/Qwen-Image-2.1/blob/main/LICENSE",
	},
	"flux2-klein": {
		Name:             "flux2-klein",
		Repo:             "mlx-community/flux2-klein-4b-4bit",
		Kind:             "image",
		Default:          "4b-q4-mlx",
		Width:            1024,
		Height:           1024,
		Steps:            4,
		GuidanceScale:    1.0,
		GuidanceScaleSet: true,
		PlatformDefaults: map[string]string{
			"windows":      "4b-bf16-cuda",
			"linux":        "4b-bf16-cuda",
			"darwin/arm64": "4b-q4-mlx",
		},
		Files: map[string]string{
			"4b-q4-mlx":    "",
			"4b-bf16-cuda": "",
			"4b-fp8-cuda":  "",
		},
		Backends: map[string]string{
			"4b-q4-mlx":    "mflux",
			"4b-bf16-cuda": "diffusers",
			"4b-fp8-cuda":  "diffusers",
		},
		Repos: map[string]string{
			"4b-q4-mlx":    "mlx-community/flux2-klein-4b-4bit",
			"4b-bf16-cuda": "black-forest-labs/FLUX.2-klein-4B",
			"4b-fp8-cuda":  "black-forest-labs/FLUX.2-klein-4b-fp8",
		},
		Sizes: map[string]string{
			"4b-q4-mlx":    "~4.6 GiB",
			"4b-bf16-cuda": "~15 GiB",
			"4b-fp8-cuda":  "~11 GiB",
		},
		Memory: map[string]string{
			"4b-q4-mlx":    "16 GiB min; 24 GiB recommended",
			"4b-bf16-cuda": "24 GiB system RAM; 13 GiB VRAM min",
			"4b-fp8-cuda":  "24 GiB system RAM; 12 GiB VRAM recommended",
		},
		GPUs: map[string]string{
			"4b-q4-mlx":    "Apple Silicon GPU",
			"4b-bf16-cuda": "NVIDIA RTX 3090/4070 or newer",
			"4b-fp8-cuda":  "NVIDIA Ada or newer recommended; Ampere supported with reduced benefit",
		},
		Features: map[string]string{
			"4b-q4-mlx":    "4-step generation, image editing, multi-reference editing",
			"4b-bf16-cuda": "4-step generation, image editing, multi-reference editing, Apache-2.0",
			"4b-fp8-cuda":  "reduced-storage CUDA bundle, image editing, multi-reference editing, Apache-2.0",
		},
		Artifacts: map[string][]Artifact{
			"4b-fp8-cuda": flux2KleinFP8Artifacts(),
		},
		License:    "Apache-2.0",
		LicenseURL: "https://huggingface.co/black-forest-labs/FLUX.2-klein-4B",
	},
	"qwen3.6": {
		Name:    "qwen3.6",
		Kind:    "text",
		Default: "35b-mlx",
		PlatformDefaults: map[string]string{
			"windows":      "27b-q4_k_m",
			"linux":        "27b-q4_k_m",
			"darwin/arm64": "35b-mlx",
		},
		Files: map[string]string{
			"27b-q4_k_m":   "Qwen3.6-27B-Q4_K_M.gguf",
			"27b-q8_0":     "Qwen3.6-27B-Q8_0.gguf",
			"35b-mlx":      "",
			"35b-mlx-4bit": "",
			"35b-mlx-6bit": "",
			"35b-mlx-8bit": "",
		},
		Backends: map[string]string{
			"35b-mlx":      "mlx-vlm",
			"35b-mlx-4bit": "mlx-vlm",
			"35b-mlx-6bit": "mlx-vlm",
			"35b-mlx-8bit": "mlx-vlm",
		},
		Repos: map[string]string{
			"27b-q4_k_m":   "ggml-org/Qwen3.6-27B-GGUF",
			"27b-q8_0":     "ggml-org/Qwen3.6-27B-GGUF",
			"35b-mlx":      "mlx-community/Qwen3.6-35B-A3B-4bit",
			"35b-mlx-4bit": "mlx-community/Qwen3.6-35B-A3B-4bit",
			"35b-mlx-6bit": "mlx-community/Qwen3.6-35B-A3B-6bit",
			"35b-mlx-8bit": "mlx-community/Qwen3.6-35B-A3B-8bit",
		},
		Sizes: map[string]string{
			"27b-q4_k_m":   "~19.1 GiB",
			"27b-q8_0":     "~28.6 GiB",
			"35b-mlx":      "~20 GiB",
			"35b-mlx-4bit": "~20 GiB",
			"35b-mlx-6bit": "~27 GiB",
			"35b-mlx-8bit": "~36 GiB",
		},
		Memory: map[string]string{
			"27b-q4_k_m": "24 GiB min; 32 GiB recommended",
			"27b-q8_0":   "40 GiB min; 48 GiB recommended",
		},
		GPUs: map[string]string{
			"27b-q4_k_m": "Optional Metal, CUDA, Vulkan, or ROCm acceleration",
			"27b-q8_0":   "Optional Metal, CUDA, Vulkan, or ROCm acceleration",
		},
		Features: map[string]string{
			"27b-q4_k_m": "reasoning, coding, tool use, image understanding with optional mmproj",
			"27b-q8_0":   "higher-fidelity reasoning, coding, tool use, image understanding with optional mmproj",
		},
	},
	"gemma4": {
		Name:    "gemma4",
		Repo:    "ggml-org/gemma-4-E2B-it-GGUF",
		Kind:    "text",
		Default: "e2b-q4_0",
		PlatformDefaults: map[string]string{
			"darwin/arm64": "e2b-mlx-4bit",
		},
		Files: map[string]string{
			"e2b-q4_0":     "gemma-4-E2B-it-Q4_0.gguf",
			"e2b-mlx-4bit": "",
		},
		Backends: map[string]string{
			"e2b-mlx-4bit": "mlx-vlm",
		},
		Repos: map[string]string{
			"e2b-q4_0":     "ggml-org/gemma-4-E2B-it-GGUF",
			"e2b-mlx-4bit": "mlx-community/gemma-4-e2b-it-4bit",
		},
		Sizes: map[string]string{
			"e2b-q4_0":     "~2.8 GiB",
			"e2b-mlx-4bit": "~3.6 GiB",
		},
		Memory: map[string]string{
			"e2b-q4_0":     "8 GiB min; 12 GiB recommended",
			"e2b-mlx-4bit": "12 GiB min; 16 GiB recommended",
		},
		GPUs: map[string]string{
			"e2b-q4_0":     "Optional Metal, CUDA, Vulkan, or ROCm acceleration",
			"e2b-mlx-4bit": "Apple Silicon GPU",
		},
		Features: map[string]string{
			"e2b-q4_0":     "reasoning, system prompts, function calling, 128K context",
			"e2b-mlx-4bit": "reasoning, system prompts, function calling, image understanding",
		},
		License:    "Apache-2.0",
		LicenseURL: "https://huggingface.co/google/gemma-4-E2B-it",
	},
	"qwen3.8": {
		Name:    "qwen3.8",
		Repo:    "unsloth/Qwen3.8-27B-GGUF",
		Kind:    "text",
		Default: "27b-q4_k_m",
		Files: map[string]string{
			"27b-q4_k_m": "Qwen3.8-27B-UD-Q4_K_M.gguf",
			"27b-q5_k_m": "Qwen3.8-27B-UD-Q5_K_M.gguf",
			"27b-mlx":    "",
		},
		Backends: map[string]string{
			"27b-mlx": "mlx-vlm",
		},
		Repos: map[string]string{
			"27b-mlx": "mlx-community/Qwen3.8-27B-4bit",
		},
		PlatformDefaults: map[string]string{
			"darwin/arm64": "27b-mlx",
		},
		Sizes: map[string]string{
			"27b-q4_k_m": "~16 GiB",
			"27b-q5_k_m": "~19 GiB",
			"27b-mlx":    "~15 GiB",
		},
		Memory: map[string]string{
			"27b-q4_k_m": "24 GiB min; 32 GiB recommended",
			"27b-q5_k_m": "32 GiB min; 40 GiB recommended",
			"27b-mlx":    "24 GiB min; 32 GiB recommended",
		},
		GPUs: map[string]string{
			"27b-q4_k_m": "Optional GPU acceleration",
			"27b-q5_k_m": "Optional GPU acceleration",
			"27b-mlx":    "Apple Silicon GPU",
		},
		Features: map[string]string{
			"27b-q4_k_m": "agentic coding, tool use, long-horizon reasoning",
			"27b-q5_k_m": "agentic coding, tool use, long-horizon reasoning",
			"27b-mlx":    "agentic coding, tool use, Apple Silicon optimized",
		},
	},
	"qwen3": {
		Name:    "qwen3",
		Repo:    "Qwen/Qwen3-8B-GGUF",
		Kind:    "text",
		Default: "8b-q4_k_m",
		Files: map[string]string{
			"4b-q4_k_m": "Qwen3-4B-Q4_K_M.gguf",
			"4b-q5_k_m": "Qwen3-4B-Q5_K_M.gguf",
			"8b-q4_k_m": "Qwen3-8B-Q4_K_M.gguf",
			"8b-q5_k_m": "Qwen3-8B-Q5_K_M.gguf",
			"30b-mlx":   "",
		},
		Backends: map[string]string{
			"30b-mlx": "mlx-vlm",
		},
		Repos: map[string]string{
			"4b-q4_k_m": "Qwen/Qwen3-4B-GGUF",
			"4b-q5_k_m": "Qwen/Qwen3-4B-GGUF",
			"8b-q4_k_m": "Qwen/Qwen3-8B-GGUF",
			"8b-q5_k_m": "Qwen/Qwen3-8B-GGUF",
			"30b-mlx":   "mlx-community/Qwen3-30B-A3B-4bit",
		},
		Sizes: map[string]string{
			"4b-q4_k_m": "~2.4 GiB",
			"4b-q5_k_m": "~2.8 GiB",
			"8b-q4_k_m": "~4.7 GiB",
			"8b-q5_k_m": "~5.5 GiB",
			"30b-mlx":   "~16 GiB",
		},
	},
	"phi4-mini": {
		Name:    "phi4-mini",
		Repo:    "unsloth/Phi-4-mini-instruct-GGUF",
		Kind:    "text",
		Default: "q4_k_m",
		Files: map[string]string{
			"q4_k_m": "Phi-4-mini-instruct-Q4_K_M.gguf",
			"q5_k_m": "Phi-4-mini-instruct-Q5_K_M.gguf",
		},
		Sizes: map[string]string{
			"q4_k_m": "~2.4 GiB",
			"q5_k_m": "~2.7 GiB",
		},
	},
	"gemma3": {
		Name:    "gemma3",
		Repo:    "ggml-org/gemma-3-4b-it-GGUF",
		Kind:    "text",
		Default: "4b-q4_k_m",
		Files: map[string]string{
			"4b-q4_k_m": "gemma-3-4b-it-Q4_K_M.gguf",
			"12b-mlx":   "",
			"27b-mlx":   "",
		},
		Backends: map[string]string{
			"12b-mlx": "mlx-vlm",
			"27b-mlx": "mlx-vlm",
		},
		Repos: map[string]string{
			"4b-q4_k_m": "ggml-org/gemma-3-4b-it-GGUF",
			"12b-mlx":   "mlx-community/gemma-3-12b-it-4bit",
			"27b-mlx":   "mlx-community/gemma-3-27b-it-4bit",
		},
		Sizes: map[string]string{
			"4b-q4_k_m": "~2.4 GiB",
			"12b-mlx":   "~7.6 GiB",
			"27b-mlx":   "~16 GiB",
		},
	},
	"qwen3-coder": {
		Name:    "qwen3-coder",
		Kind:    "text",
		Default: "30b-mlx",
		Files: map[string]string{
			"30b-mlx": "",
		},
		Backends: map[string]string{
			"30b-mlx": "mlx-vlm",
		},
		Repos: map[string]string{
			"30b-mlx": "mlx-community/Qwen3-Coder-30B-A3B-Instruct-4bit",
		},
		Sizes: map[string]string{
			"30b-mlx": "~16 GiB",
		},
	},
	"sd-turbo": {
		Name:    "sd-turbo",
		Repo:    "stabilityai/sd-turbo",
		Kind:    "image",
		Default: "fp16",
		Width:   512,
		Height:  512,
		Steps:   4,
		Files: map[string]string{
			"fp16":          "",
			"onnx-directml": "",
			"onnx-arm64":    "",
		},
		Backends: map[string]string{
			"fp16":          "diffusers",
			"onnx-directml": "onnx-directml",
			"onnx-arm64":    "onnx-cpu",
		},
		Repos: map[string]string{
			"onnx-directml": "Heliosoph/sd-turbo-onnx",
			"onnx-arm64":    "Heliosoph/sd-turbo-onnx",
		},
		Sizes: map[string]string{
			"fp16":          "~3 GiB",
			"onnx-directml": "~4.8 GiB",
			"onnx-arm64":    "~4.8 GiB",
		},
		Memory: map[string]string{
			"fp16":          "16 GiB min; 24 GiB recommended",
			"onnx-directml": "12 GiB min; 16 GiB recommended",
			"onnx-arm64":    "12 GiB min; 16 GiB recommended",
		},
		GPUs: map[string]string{
			"fp16":          "NVIDIA CUDA, 6 GiB+ VRAM",
			"onnx-directml": "AMD, Intel, or NVIDIA DirectX 12 GPU",
			"onnx-arm64":    "CPU (native Windows ARM64)",
		},
	},
	"sdxl-turbo": {
		Name:    "sdxl-turbo",
		Repo:    "stabilityai/sdxl-turbo",
		Kind:    "image",
		Default: "fp16",
		Width:   1024,
		Height:  1024,
		Steps:   4,
		Files: map[string]string{
			"fp16": "",
		},
		Backends: map[string]string{
			"fp16": "diffusers",
		},
		Sizes: map[string]string{
			"fp16": "~7 GiB",
		},
		Memory: map[string]string{"fp16": "24 GiB min; 32 GiB recommended"},
		GPUs:   map[string]string{"fp16": "NVIDIA CUDA, 8 GiB+ VRAM"},
	},
	"krea-2-turbo": {
		Name:       "krea-2-turbo",
		Repo:       "krea/Krea-2-Turbo",
		Kind:       "image",
		Default:    "bf16-cuda",
		Width:      1024,
		Height:     1024,
		Steps:      8,
		Gated:      true,
		License:    "Krea 2 Community License",
		LicenseURL: "https://huggingface.co/krea/Krea-2-Turbo",
		Files: map[string]string{
			"bf16-cuda": "",
			"bf16-mps":  "",
		},
		Backends: map[string]string{
			"bf16-cuda": "diffusers",
			"bf16-mps":  "diffusers-mps",
		},
		Sizes: map[string]string{
			"bf16-cuda": "~34 GiB",
			"bf16-mps":  "~34 GiB",
		},
		Memory: map[string]string{
			"bf16-cuda": "32 GiB min; 48 GiB recommended",
			"bf16-mps":  "64 GiB min; 96 GiB recommended",
		},
		GPUs: map[string]string{
			"bf16-cuda": "NVIDIA CUDA, 16 GiB VRAM min with CPU offload; 24 GiB recommended",
			"bf16-mps":  "Apple Silicon GPU (experimental MPS)",
		},
		Features: map[string]string{
			"bf16-cuda": "8-step text-to-image, 1K–2K output, LoRA stacks, gated license",
			"bf16-mps":  "8-step text-to-image, 1K–2K output, LoRA stacks, gated license, experimental",
		},
	},
	"krea-2-raw": {
		Name:             "krea-2-raw",
		Repo:             "krea/Krea-2-Raw",
		Kind:             "image",
		Default:          "bf16-cuda",
		Width:            1024,
		Height:           1024,
		Steps:            52,
		GuidanceScale:    3.5,
		GuidanceScaleSet: true,
		Gated:            true,
		License:          "Krea 2 Community License",
		LicenseURL:       "https://huggingface.co/krea/Krea-2-Raw",
		Files: map[string]string{
			"bf16-cuda": "",
			"bf16-mps":  "",
		},
		Backends: map[string]string{
			"bf16-cuda": "diffusers",
			"bf16-mps":  "diffusers-mps",
		},
		Sizes: map[string]string{
			"bf16-cuda": "~34 GiB",
			"bf16-mps":  "~34 GiB",
		},
		Memory: map[string]string{
			"bf16-cuda": "48 GiB min; 64 GiB recommended",
			"bf16-mps":  "64 GiB min; 96 GiB recommended",
		},
		GPUs: map[string]string{
			"bf16-cuda": "NVIDIA CUDA, 24 GiB VRAM recommended; CPU offload supported",
			"bf16-mps":  "Apple Silicon GPU (experimental MPS)",
		},
		Features: map[string]string{
			"bf16-cuda": "52-step base checkpoint for fine-tuning and post-training; LoRA stacks; gated license",
			"bf16-mps":  "52-step base checkpoint for fine-tuning and post-training; LoRA stacks; gated license; experimental",
		},
	},
	"wan2.2-video": {
		Name:    "wan2.2-video",
		Repo:    "Anes1032/Wan2.2-TI2V-5B-mlx-q8",
		Kind:    "video",
		Default: "5b-q8-mlx",
		Width:   832,
		Height:  480,
		Steps:   40,
		Frames:  41,
		FPS:     24,
		Files:   map[string]string{"5b-q8-mlx": ""},
		Backends: map[string]string{
			"5b-q8-mlx": "mlx-video",
		},
		Sizes:  map[string]string{"5b-q8-mlx": "~18 GiB"},
		Memory: map[string]string{"5b-q8-mlx": "32 GiB min; 48 GiB recommended"},
		GPUs:   map[string]string{"5b-q8-mlx": "Apple Silicon GPU"},
	},
	"yume-video": {
		Name:    "yume-video",
		Repo:    "ckurasek/Yume-1.5-5B-720P-MLX",
		Kind:    "video",
		Default: "5b-mlx",
		Width:   832,
		Height:  480,
		Steps:   30,
		Frames:  81,
		FPS:     24,
		Files:   map[string]string{"5b-mlx": ""},
		Backends: map[string]string{
			"5b-mlx": "mlx-video",
		},
		Sizes:  map[string]string{"5b-mlx": "~23 GiB"},
		Memory: map[string]string{"5b-mlx": "32 GiB min; 48 GiB recommended"},
		GPUs:   map[string]string{"5b-mlx": "Apple Silicon GPU"},
	},
	"minimax-h3": {
		Name:    "minimax-h3",
		Repo:    "Comfy-Org/MiniMax-H3",
		Kind:    "video",
		Default: "fl2va-int8-mac",
		Width:   864,
		Height:  480,
		Steps:   20,
		Frames:  73,
		FPS:     24,
		Files: map[string]string{
			"fl2va-int8-mac":  "",
			"fl2va-int8-cuda": "",
		},
		Backends: map[string]string{
			"fl2va-int8-mac":  "comfy-h3-mps",
			"fl2va-int8-cuda": "comfy-h3-cuda",
		},
		Sizes: map[string]string{
			"fl2va-int8-mac":  "~41 GiB",
			"fl2va-int8-cuda": "~41 GiB",
		},
		Memory: map[string]string{
			"fl2va-int8-mac":  "48 GiB min; 64 GiB recommended",
			"fl2va-int8-cuda": "32 GiB system RAM; 16 GiB VRAM recommended",
		},
		GPUs: map[string]string{
			"fl2va-int8-mac":  "Apple Silicon GPU",
			"fl2va-int8-cuda": "NVIDIA CUDA, 16 GiB VRAM (4070 Ti SUPER supported)",
		},
		Features: map[string]string{
			"fl2va-int8-mac":  "text/image-to-video, native stereo audio, LoRA stacks, managed ComfyUI",
			"fl2va-int8-cuda": "text/image-to-video, native stereo audio, LoRA stacks, managed ComfyUI",
		},
		Artifacts: map[string][]Artifact{
			"fl2va-int8-mac": {
				{Repo: "Comfy-Org/MiniMax-H3", Filename: "diffusion_models/minimax_h3_fl2va_pruned_int8_convrot.safetensors", Target: "diffusion_models/minimax_h3_fl2va_pruned_int8_convrot.safetensors"},
				{Repo: "realrebelai/MiniMax-H3_GGUFs", Filename: "qwen3vl-32B-MiniMax-H3-Q4_K_M.gguf", Target: "text_encoders/qwen3vl-32B-MiniMax-H3-Q4_K_M.gguf"},
				{Repo: "Comfy-Org/MiniMax-H3", Filename: "vae/minimax_h3_video_vae_fp16.safetensors", Target: "vae/minimax_h3_video_vae_fp16.safetensors"},
				{Repo: "Comfy-Org/MiniMax-H3", Filename: "vae/minimax_h3_audio_vae_fp32.safetensors", Target: "vae/minimax_h3_audio_vae_fp32.safetensors"},
			},
			"fl2va-int8-cuda": {
				{Repo: "Comfy-Org/MiniMax-H3", Filename: "diffusion_models/minimax_h3_fl2va_pruned_int8_convrot.safetensors", Target: "diffusion_models/minimax_h3_fl2va_pruned_int8_convrot.safetensors"},
				{Repo: "Comfy-Org/MiniMax-H3", Filename: "text_encoders/qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors", Target: "text_encoders/qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"},
				{Repo: "Comfy-Org/MiniMax-H3", Filename: "vae/minimax_h3_video_vae_fp16.safetensors", Target: "vae/minimax_h3_video_vae_fp16.safetensors"},
				{Repo: "Comfy-Org/MiniMax-H3", Filename: "vae/minimax_h3_audio_vae_fp32.safetensors", Target: "vae/minimax_h3_audio_vae_fp32.safetensors"},
			},
		},
	},
	"ltx-video": {
		Name:    "ltx-video",
		Repo:    "Lightricks/LTX-Video-0.9.5",
		Kind:    "video",
		Default: "2b-fp16",
		Width:   768,
		Height:  512,
		Steps:   8,
		Frames:  49,
		FPS:     24,
		Files:   map[string]string{"2b-fp16": ""},
		Backends: map[string]string{
			"2b-fp16": "diffusers-video",
		},
		Sizes:  map[string]string{"2b-fp16": "~24 GiB"},
		Memory: map[string]string{"2b-fp16": "24 GiB min; 32 GiB recommended"},
		GPUs:   map[string]string{"2b-fp16": "NVIDIA CUDA, 8 GiB+ VRAM; CPU offload"},
	},
	"ltx-2.5": {
		Name:       "ltx-2.5",
		Repo:       "Lightricks/LTX-2.5-Diffusers",
		Kind:       "video",
		Default:    "22b-bf16-cuda",
		Width:      768,
		Height:     512,
		Steps:      30,
		Frames:     121,
		FPS:        24,
		Gated:      true,
		License:    "LTX-2 Community License",
		LicenseURL: "https://huggingface.co/Lightricks/LTX-2.5-Diffusers",
		Files: map[string]string{
			"22b-bf16-cuda": "",
		},
		Backends: map[string]string{
			"22b-bf16-cuda": "diffusers-ltx2",
		},
		Sizes: map[string]string{
			"22b-bf16-cuda": "~68 GiB",
		},
		Memory: map[string]string{
			"22b-bf16-cuda": "96 GiB system RAM recommended; sequential CPU offload enabled",
		},
		GPUs: map[string]string{
			"22b-bf16-cuda": "NVIDIA CUDA, 24 GiB VRAM minimum; 48 GiB recommended",
		},
		Features: map[string]string{
			"22b-bf16-cuda": "text-to-video, synchronized stereo audio, multishot prompting, 1-20 second clips",
		},
		Artifacts: map[string][]Artifact{
			"22b-bf16-cuda": ltx25DiffusersArtifacts(),
		},
	},
	"stable-video-diffusion": {
		Name:    "stable-video-diffusion",
		Repo:    "stabilityai/stable-video-diffusion-img2vid-xt",
		Kind:    "video",
		Default: "xt-fp16",
		Width:   1024,
		Height:  576,
		Steps:   25,
		Frames:  25,
		FPS:     7,
		Files:   map[string]string{"xt-fp16": ""},
		Backends: map[string]string{
			"xt-fp16": "diffusers-video",
		},
		Sizes:  map[string]string{"xt-fp16": "~4.5 GiB"},
		Memory: map[string]string{"xt-fp16": "16 GiB min; 24 GiB recommended"},
		GPUs:   map[string]string{"xt-fp16": "NVIDIA CUDA, 8 GiB+ VRAM; CPU offload"},
	},
}

type Resolved struct {
	Download
	Name                      string
	Repo                      string
	Filename                  string
	URL                       string
	Kind                      string
	Backend                   string
	Width                     int
	Height                    int
	Steps                     int
	Frames                    int
	FPS                       int
	DurationSeconds           int
	MaxDuration               int
	SampleRate                int
	Size                      string
	Platform                  string
	Memory                    string
	GPU                       string
	Languages                 string
	Features                  string
	Artifacts                 []Artifact
	GuidanceScale             float64
	GuidanceScaleSet          bool
	Gated                     bool
	LicenseAcceptanceRequired bool
	License                   string
	LicenseURL                string
}

func Resolve(ref string) (Resolved, error) {
	return ResolveForPlatform(ref, runtime.GOOS, runtime.GOARCH)
}

// DefaultContext keeps new compact models within their qualified memory tier.
func DefaultContext(ref string) int {
	name, _, _ := strings.Cut(strings.ToLower(ref), ":")
	if model, ok := activeModels()[name]; ok && model.Context > 0 {
		return model.Context
	}
	return 65536
}

func ResolveFor(ref, goos string) (Resolved, error) {
	return ResolveForPlatform(ref, goos, runtime.GOARCH)
}

func ResolveForPlatform(ref, goos, goarch string) (Resolved, error) {
	name, tag, _ := strings.Cut(ref, ":")
	m, ok := activeModels()[strings.ToLower(name)]
	if !ok {
		return Resolved{}, fmt.Errorf(
			"unknown model %q; run `tapioca catalog` to see available models", name,
		)
	}
	if tag == "" {
		tag = m.Default
	}
	if platformDefault := m.PlatformDefaults[goos+"/"+goarch]; platformDefault != "" &&
		!strings.Contains(ref, ":") {
		tag = platformDefault
	} else if platformDefault := m.PlatformDefaults[goos]; platformDefault != "" &&
		!strings.Contains(ref, ":") {
		tag = platformDefault
	}
	if m.Name == "qwen-image-flash" && !strings.Contains(ref, ":") {
		if goos == "darwin" {
			tag = "int8"
		} else {
			tag = "bf16"
		}
	}
	if m.Name == "sd-turbo" && !strings.Contains(ref, ":") &&
		goos == "windows" && goarch == "arm64" {
		tag = "onnx-arm64"
	}
	if m.Name == "qwen3-tts" && !strings.Contains(ref, ":") &&
		goos == "darwin" && goarch == "arm64" {
		tag = "0.6b-mlx"
	}
	if m.Name == "minimax-h3" && !strings.Contains(ref, ":") {
		if goos == "darwin" && goarch == "arm64" {
			tag = "fl2va-int8-mac"
		} else {
			tag = "fl2va-int8-cuda"
		}
	}
	if (m.Name == "krea-2-turbo" || m.Name == "krea-2-raw") &&
		!strings.Contains(ref, ":") {
		if goos == "darwin" && goarch == "arm64" {
			tag = "bf16-mps"
		} else {
			tag = "bf16-cuda"
		}
	}
	filename, ok := m.Files[strings.ToLower(tag)]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown variant %q for %s", tag, name)
	}
	repo := m.Repo
	if variantRepo := m.Repos[strings.ToLower(tag)]; variantRepo != "" {
		repo = variantRepo
	}
	if m.Name == "qwen-image-flash" && strings.EqualFold(tag, "bf16") {
		repo = "nvidia/Qwen-Image-Flash"
	}
	url := ""
	download := m.Downloads[strings.ToLower(tag)]
	if filename != "" {
		url = "https://huggingface.co/" + repo + "/resolve/" + download.Ref() + "/" + filename
	}
	backend := m.Backends[strings.ToLower(tag)]
	platform := "Windows, macOS, Linux"
	switch backend {
	case "mlx", "mlx-vlm", "mlx-video", "mflux", "speech-qwen-mlx", "speech-audio8-mlx":
		platform = "macOS Apple Silicon"
	case "diffusers", "diffusers-video", "diffusers-ltx2":
		platform = "Windows/Linux NVIDIA"
	case "diffusers-mps":
		platform = "macOS Apple Silicon"
	case "comfy-h3-mps":
		platform = "macOS Apple Silicon"
	case "comfy-h3-cuda":
		platform = "Windows/Linux NVIDIA"
	case "speech-chatterbox":
		platform = "Windows, macOS, Linux"
	case "speech-audio8-onnx", "speech-pocket-tts":
		platform = "macOS Apple Silicon; Windows/Linux x64 CPU"
	case "speech-sopro":
		platform = "macOS Apple Silicon CPU (experimental)"
	case "speech-qwen":
		platform = "Windows/Linux NVIDIA or CPU"
	case "speech-audio8":
		platform = "Windows, macOS, Linux; CUDA recommended"
	case "stable-audio3":
		platform = "Windows, macOS, Linux"
	case "onnx-directml":
		platform = "Windows x64 AMD/Intel/NVIDIA"
	case "onnx-cpu":
		platform = "Windows ARM64"
	}
	memory, gpu := requirements(m, strings.ToLower(tag), backend)
	return Resolved{
		Download:        download,
		Name:            m.Name + ":" + strings.ToLower(tag),
		Repo:            repo,
		Filename:        filename,
		URL:             url,
		Kind:            m.Kind,
		Backend:         backend,
		Width:           m.Width,
		Height:          m.Height,
		Steps:           m.Steps,
		Frames:          m.Frames,
		FPS:             m.FPS,
		DurationSeconds: m.DurationSeconds,
		MaxDuration:     m.MaxDuration,
		SampleRate:      m.SampleRate,
		Size:            m.Sizes[strings.ToLower(tag)],
		Platform:        platform,
		Memory:          memory,
		GPU:             gpu,
		Languages:       m.Languages[strings.ToLower(tag)],
		Features:        m.Features[strings.ToLower(tag)],
		Artifacts:       append([]Artifact(nil), m.Artifacts[strings.ToLower(tag)]...),
		GuidanceScale:   m.GuidanceScale, GuidanceScaleSet: m.GuidanceScaleSet,
		Gated: m.Gated, LicenseAcceptanceRequired: m.LicenseAcceptanceRequired,
		License: m.License, LicenseURL: m.LicenseURL,
	}, nil
}

func requirements(model Model, variant, backend string) (string, string) {
	if memory := model.Memory[variant]; memory != "" {
		return memory, model.GPUs[variant]
	}
	size := model.Sizes[variant]
	switch backend {
	case "mlx", "mlx-vlm", "mflux", "speech-qwen-mlx", "speech-audio8-mlx":
		switch {
		case strings.Contains(size, "~36"):
			return "48 GiB min; 64 GiB recommended", "Apple Silicon GPU"
		case strings.Contains(size, "~27"), strings.Contains(size, "~28"):
			return "40 GiB min; 48 GiB recommended", "Apple Silicon GPU"
		case strings.Contains(size, "~20"), strings.Contains(size, "~16"):
			return "24 GiB min; 32 GiB recommended", "Apple Silicon GPU"
		default:
			return "16 GiB min; 24 GiB recommended", "Apple Silicon GPU"
		}
	default:
		switch {
		case strings.Contains(size, "~30"):
			return "40 GiB min; 48 GiB recommended", "Optional GPU acceleration"
		case strings.Contains(size, "~17"):
			return "24 GiB min; 32 GiB recommended", "Optional GPU acceleration"
		case strings.Contains(size, "~7"):
			return "16 GiB min; 24 GiB recommended", "Optional GPU acceleration"
		default:
			return "8 GiB min; 12 GiB recommended", "Optional GPU acceleration"
		}
	}
}

func Refs() []string {
	var refs []string
	for name, model := range activeModels() {
		for variant := range model.Files {
			refs = append(refs, name+":"+variant)
		}
	}
	sort.Strings(refs)
	return refs
}
