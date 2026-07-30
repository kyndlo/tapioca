package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Model struct {
	Name     string `json:"name"`
	Repo     string `json:"repo"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

type Registry struct {
	Models map[string]Model `json:"models"`
}

func Home() (string, error) {
	if v := os.Getenv("TAPIOCA_HOME"); v != "" {
		return filepath.Abs(v)
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".tapioca"), nil
}

func Load() (Registry, error) {
	home, err := Home()
	if err != nil {
		return Registry{}, err
	}
	b, err := os.ReadFile(filepath.Join(home, "registry.json"))
	if errors.Is(err, os.ErrNotExist) {
		return Registry{Models: map[string]Model{}}, nil
	}
	if err != nil {
		return Registry{}, err
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return Registry{}, err
	}
	if r.Models == nil {
		r.Models = map[string]Model{}
	}
	return r, nil
}

func (r Registry) Save() error {
	home, err := Home()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, "registry.json"), append(b, '\n'), 0o644)
}

func (r Registry) Find(ref string) (Model, bool) {
	if m, ok := r.Models[strings.ToLower(ref)]; ok {
		return m, true
	}
	if !strings.Contains(ref, ":") {
		for name, m := range r.Models {
			if strings.HasPrefix(name, strings.ToLower(ref)+":") {
				return m, true
			}
		}
	}
	return Model{}, false
}
