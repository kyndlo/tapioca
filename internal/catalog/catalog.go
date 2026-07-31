package catalog

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

type Model struct {
	Name     string
	Repo     string
	Files    map[string]string
	Template string
	Kind     string
	Backends map[string]string
	Repos    map[string]string
	Default  string
	Width    int
	Height   int
	Steps    int
	Frames   int
	FPS      int
	Sizes    map[string]string
	Memory   map[string]string
	GPUs     map[string]string
}

var models = map[string]Model{
	"glm-4.7-flash": {
		Name:    "glm-4.7-flash",
		Repo:    "ggml-org/GLM-4.7-Flash-GGUF",
		Default: "q4_k_m",
		Files: map[string]string{
			"q4_k_m": "GLM-4.7-Flash-Q4_K_M.gguf",
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
	"flux2-klein": {
		Name:    "flux2-klein",
		Repo:    "mlx-community/flux2-klein-4b-4bit",
		Kind:    "image",
		Default: "4b-q4-mlx",
		Width:   1024,
		Height:  1024,
		Steps:   4,
		Files:   map[string]string{"4b-q4-mlx": ""},
		Backends: map[string]string{
			"4b-q4-mlx": "mflux",
		},
		Sizes:  map[string]string{"4b-q4-mlx": "~4.6 GiB"},
		Memory: map[string]string{"4b-q4-mlx": "16 GiB min; 24 GiB recommended"},
		GPUs:   map[string]string{"4b-q4-mlx": "Apple Silicon GPU"},
	},
	"qwen3.6": {
		Name:    "qwen3.6",
		Kind:    "text",
		Default: "35b-mlx",
		Files: map[string]string{
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
			"35b-mlx":      "mlx-community/Qwen3.6-35B-A3B-4bit",
			"35b-mlx-4bit": "mlx-community/Qwen3.6-35B-A3B-4bit",
			"35b-mlx-6bit": "mlx-community/Qwen3.6-35B-A3B-6bit",
			"35b-mlx-8bit": "mlx-community/Qwen3.6-35B-A3B-8bit",
		},
		Sizes: map[string]string{
			"35b-mlx":      "~20 GiB",
			"35b-mlx-4bit": "~20 GiB",
			"35b-mlx-6bit": "~27 GiB",
			"35b-mlx-8bit": "~36 GiB",
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
	Name     string
	Repo     string
	Filename string
	URL      string
	Kind     string
	Backend  string
	Width    int
	Height   int
	Steps    int
	Frames   int
	FPS      int
	Size     string
	Platform string
	Memory   string
	GPU      string
}

func Resolve(ref string) (Resolved, error) {
	return ResolveForPlatform(ref, runtime.GOOS, runtime.GOARCH)
}

func ResolveFor(ref, goos string) (Resolved, error) {
	return ResolveForPlatform(ref, goos, runtime.GOARCH)
}

func ResolveForPlatform(ref, goos, goarch string) (Resolved, error) {
	name, tag, _ := strings.Cut(ref, ":")
	m, ok := models[strings.ToLower(name)]
	if !ok {
		return Resolved{}, fmt.Errorf(
			"unknown model %q; run `tapioca catalog` to see available models", name,
		)
	}
	if tag == "" {
		tag = m.Default
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
	filename, ok := m.Files[strings.ToLower(tag)]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown variant %q for %s", tag, name)
	}
	repo := m.Repo
	if variantRepo := m.Repos[strings.ToLower(tag)]; variantRepo != "" {
		repo = variantRepo
	}
	if m.Kind == "image" && strings.EqualFold(tag, "bf16") {
		repo = "nvidia/Qwen-Image-Flash"
	}
	url := ""
	if filename != "" {
		url = "https://huggingface.co/" + repo + "/resolve/main/" + filename
	}
	backend := m.Backends[strings.ToLower(tag)]
	platform := "Windows, macOS, Linux"
	switch backend {
	case "mlx", "mlx-vlm", "mlx-video", "mflux":
		platform = "macOS Apple Silicon"
	case "diffusers", "diffusers-video":
		platform = "Windows/Linux NVIDIA"
		if backend == "diffusers-video" {
			platform = "Windows x64 NVIDIA"
		}
	case "onnx-directml":
		platform = "Windows x64 AMD/Intel/NVIDIA"
	case "onnx-cpu":
		platform = "Windows ARM64"
	}
	memory, gpu := requirements(m, strings.ToLower(tag), backend)
	return Resolved{
		Name:     m.Name + ":" + strings.ToLower(tag),
		Repo:     repo,
		Filename: filename,
		URL:      url,
		Kind:     m.Kind,
		Backend:  backend,
		Width:    m.Width,
		Height:   m.Height,
		Steps:    m.Steps,
		Frames:   m.Frames,
		FPS:      m.FPS,
		Size:     m.Sizes[strings.ToLower(tag)],
		Platform: platform,
		Memory:   memory,
		GPU:      gpu,
	}, nil
}

func requirements(model Model, variant, backend string) (string, string) {
	if memory := model.Memory[variant]; memory != "" {
		return memory, model.GPUs[variant]
	}
	size := model.Sizes[variant]
	switch backend {
	case "mlx", "mlx-vlm", "mflux":
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
	for name, model := range models {
		for variant := range model.Files {
			refs = append(refs, name+":"+variant)
		}
	}
	sort.Strings(refs)
	return refs
}
