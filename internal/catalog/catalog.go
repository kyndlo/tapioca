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
}

var models = map[string]Model{
	"glm-4.7-flash": {
		Name: "glm-4.7-flash",
		Repo: "ggml-org/GLM-4.7-Flash-GGUF",
		Files: map[string]string{
			"q4_k_m": "GLM-4.7-Flash-Q4_K_M.gguf",
			"q8_0":   "GLM-4.7-Flash-Q8_0.gguf",
		},
	},
	"qwen-image-flash": {
		Name: "qwen-image-flash",
		Repo: "mlx-community/Qwen-Image-Flash-8bit",
		Kind: "image",
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
	if tag == "" {
		tag = "q4_k_m"
	}
	m, ok := models[strings.ToLower(name)]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown model %q; available: glm-4.7-flash, qwen-image-flash", name)
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
