package modellicense

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/carlos/tapioca/internal/config"
)

type Acceptance struct {
	Model      string    `json:"model"`
	License    string    `json:"license"`
	LicenseURL string    `json:"license_url"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type store struct {
	Acceptances map[string]Acceptance `json:"acceptances"`
}

func Accepted(model string) (bool, error) {
	items, err := load()
	if err != nil {
		return false, err
	}
	_, ok := items.Acceptances[strings.ToLower(model)]
	return ok, nil
}

func Accept(model, license, licenseURL string) error {
	items, err := load()
	if err != nil {
		return err
	}
	if items.Acceptances == nil {
		items.Acceptances = map[string]Acceptance{}
	}
	items.Acceptances[strings.ToLower(model)] = Acceptance{
		Model: model, License: license, LicenseURL: licenseURL, AcceptedAt: time.Now().UTC(),
	}
	return save(items)
}

func Require(model, license, licenseURL string) error {
	accepted, err := Accepted(model)
	if err != nil {
		return err
	}
	if accepted {
		return nil
	}
	return fmt.Errorf(
		"%s is gated by the %s; review and accept the terms at %s, then run `tapioca pull %s --accept-license` (and set HF_TOKEN to a Hugging Face read token)",
		model, license, licenseURL, model,
	)
}

func load() (store, error) {
	home, err := config.Home()
	if err != nil {
		return store{}, err
	}
	data, err := os.ReadFile(filepath.Join(home, "licenses.json"))
	if errors.Is(err, os.ErrNotExist) {
		return store{Acceptances: map[string]Acceptance{}}, nil
	}
	if err != nil {
		return store{}, err
	}
	var value store
	if err := json.Unmarshal(data, &value); err != nil {
		return store{}, fmt.Errorf("read model license acceptances: %w", err)
	}
	if value.Acceptances == nil {
		value.Acceptances = map[string]Acceptance{}
	}
	return value, nil
}

func save(value store) error {
	home, err := config.Home()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary := filepath.Join(home, "licenses.json.tmp")
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(home, "licenses.json"))
}
