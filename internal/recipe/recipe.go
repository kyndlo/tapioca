package recipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Recipe struct {
	Name     string   `json:"name"`
	Base     string   `json:"base"`
	Adapters []string `json:"adapters,omitempty"`
	Preset   string   `json:"preset,omitempty"`
}

func Save(home string, value Recipe) error {
	if !validName.MatchString(value.Name) {
		return errors.New("recipe name may contain letters, numbers, dots, underscores, and hyphens")
	}
	if value.Base == "" {
		return errors.New("recipe base model is required")
	}
	dir := filepath.Join(home, "recipes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, value.Name+".json"), append(data, '\n'), 0o644)
}

func Load(home, name string) (Recipe, error) {
	if !validName.MatchString(name) {
		return Recipe{}, fmt.Errorf("invalid recipe name %q", name)
	}
	data, err := os.ReadFile(filepath.Join(home, "recipes", name+".json"))
	if err != nil {
		return Recipe{}, err
	}
	var value Recipe
	if err := json.Unmarshal(data, &value); err != nil {
		return Recipe{}, err
	}
	if value.Name == "" {
		value.Name = name
	}
	if value.Base == "" {
		return Recipe{}, fmt.Errorf("recipe %q has no base model", name)
	}
	return value, nil
}

func Exists(home, name string) bool {
	_, err := os.Stat(filepath.Join(home, "recipes", name+".json"))
	return err == nil
}
