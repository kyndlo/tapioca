package catalog

import (
	"fmt"
	"strings"
)

type Model struct {
	Name     string
	Repo     string
	Files    map[string]string
	Template string
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
}

type Resolved struct {
	Name     string
	Repo     string
	Filename string
	URL      string
}

func Resolve(ref string) (Resolved, error) {
	name, tag, _ := strings.Cut(ref, ":")
	if tag == "" {
		tag = "q4_k_m"
	}
	m, ok := models[strings.ToLower(name)]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown model %q; MVP catalog currently contains glm-4.7-flash", name)
	}
	filename, ok := m.Files[strings.ToLower(tag)]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown quantization %q for %s (available: q4_k_m, q8_0)", tag, name)
	}
	return Resolved{
		Name:     m.Name + ":" + strings.ToLower(tag),
		Repo:     m.Repo,
		Filename: filename,
		URL:      "https://huggingface.co/" + m.Repo + "/resolve/main/" + filename,
	}, nil
}
