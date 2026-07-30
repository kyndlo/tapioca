package catalog

import (
	"fmt"
	"runtime"
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
	},
	"qwen-image-flash": {
		Name:    "qwen-image-flash",
		Repo:    "mlx-community/Qwen-Image-Flash-8bit",
		Kind:    "image",
		Default: "int8",
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
	},
}

type Resolved struct {
	Name     string
	Repo     string
	Filename string
	URL      string
	Kind     string
	Backend  string
}

func Resolve(ref string) (Resolved, error) {
	return ResolveFor(ref, runtime.GOOS)
}

func ResolveFor(ref, goos string) (Resolved, error) {
	name, tag, _ := strings.Cut(ref, ":")
	m, ok := models[strings.ToLower(name)]
	if !ok {
		return Resolved{}, fmt.Errorf(
			"unknown model %q; available: glm-4.7-flash, qwen3.6, qwen-image-flash",
			name,
		)
	}
	if tag == "" {
		tag = m.Default
	}
	if m.Kind == "image" && !strings.Contains(ref, ":") {
		if goos == "darwin" {
			tag = "int8"
		} else {
			tag = "bf16"
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
	if m.Kind == "image" && strings.EqualFold(tag, "bf16") {
		repo = "nvidia/Qwen-Image-Flash"
	}
	url := ""
	if filename != "" {
		url = "https://huggingface.co/" + repo + "/resolve/main/" + filename
	}
	return Resolved{
		Name:     m.Name + ":" + strings.ToLower(tag),
		Repo:     repo,
		Filename: filename,
		URL:      url,
		Kind:     m.Kind,
		Backend:  m.Backends[strings.ToLower(tag)],
	}, nil
}
