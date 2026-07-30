package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLauncherConfigurations(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("TAPIOCA_HOME", filepath.Join(temp, "tapioca"))
	bin := filepath.Join(temp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codex", "claude", "opencode", "openclaw", "hermes"} {
		path := filepath.Join(bin, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)

	codex, err := launcher("codex", "glm-4.7-flash:q8_0", 11435, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(codex.Env, "CODEX_HOME=") {
		t.Fatal("CODEX_HOME not configured")
	}
	configBytes, err := os.ReadFile(filepath.Join(temp, "tapioca", "launch", "codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configBytes), `wire_api = "responses"`) {
		t.Fatalf("unexpected Codex config: %s", configBytes)
	}

	claude, err := launcher("claude", "glm-4.7-flash:q8_0", 11435, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(claude.Env, "ANTHROPIC_BASE_URL=http://127.0.0.1:11435") {
		t.Fatal("Anthropic base URL not configured")
	}

	opencode, err := launcher("opencode", "glm-4.7-flash:q8_0", 11435, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(opencode.Env, "OPENCODE_CONFIG=") {
		t.Fatal("OPENCODE_CONFIG not configured")
	}

	openclaw, err := launcher("openclaw", "glm-4.7-flash:q8_0", 11435, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(openclaw.Env, "OPENCLAW_CONFIG_PATH=") {
		t.Fatal("OPENCLAW_CONFIG_PATH not configured")
	}
	openclawConfig, err := os.ReadFile(filepath.Join(temp, "tapioca", "launch", "openclaw", "state", "openclaw.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(openclawConfig), `"primary": "tapioca/glm-4.7-flash:q8_0"`) {
		t.Fatalf("unexpected OpenClaw config: %s", openclawConfig)
	}

	hermes, err := launcher("hermes", "glm-4.7-flash:q8_0", 11435, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(hermes.Env, "HERMES_HOME=") {
		t.Fatal("HERMES_HOME not configured")
	}
	hermesConfig, err := os.ReadFile(filepath.Join(temp, "tapioca", "launch", "hermes", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hermesConfig), `provider: custom`) {
		t.Fatalf("unexpected Hermes config: %s", hermesConfig)
	}
}

func TestSplitClientArgs(t *testing.T) {
	options, client := splitClientArgs([]string{"--port", "12000", "--", "--help"})
	if len(options) != 2 || len(client) != 1 || client[0] != "--help" {
		t.Fatalf("unexpected split: %#v %#v", options, client)
	}
}

func containsEnv(env []string, prefix string) bool {
	for _, value := range env {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
