package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos/tapioca/internal/catalog"
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

func TestChatExitCommand(t *testing.T) {
	for _, input := range []string{"/bye", "/BYE", "  /bye  "} {
		if !isChatExit(input) {
			t.Errorf("%q should exit chat", input)
		}
	}
	for _, input := range []string{"bye", "/bye now", "hello"} {
		if isChatExit(input) {
			t.Errorf("%q should be sent to the model", input)
		}
	}
}

func TestPullGatedModelRequiresExplicitLicenseAcceptance(t *testing.T) {
	t.Setenv("TAPIOCA_HOME", t.TempDir())
	err := pull([]string{"krea-2-turbo:bf16-mps"})
	if err == nil || !strings.Contains(err.Error(), "--accept-license") ||
		!strings.Contains(err.Error(), "HF_TOKEN") {
		t.Fatalf("pull gated model error = %v", err)
	}
}

func TestHuggingFaceTokenContextOverridesEnvironmentWithoutPersistence(t *testing.T) {
	t.Setenv("HF_TOKEN", "environment-token")
	ctx := WithHuggingFaceToken(context.Background(), "one-time-token")
	if got := huggingFaceToken(ctx); got != "one-time-token" {
		t.Fatalf("huggingFaceToken() = %q", got)
	}
	if got := huggingFaceToken(context.Background()); got != "environment-token" {
		t.Fatalf("environment token = %q", got)
	}
}

func TestVideoValidationBeforeDownload(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{
			[]string{"ltx-video:2b-fp16", "--prompt", "test", "--frames", "13"},
			"8n+1",
		},
		{
			[]string{"wan2.2-video:5b-q8-mlx", "--prompt", "test", "--fps", "12"},
			"24 fps",
		},
		{
			[]string{"stable-video-diffusion:xt-fp16", "--prompt", "test"},
			"requires --image",
		},
	}
	for _, test := range tests {
		err := video(test.args)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("video(%q) error = %v, want %q", test.args, err, test.want)
		}
	}
}

func TestVideoPresets(t *testing.T) {
	model, err := catalog.ResolveFor("wan2.2-video:5b-q8-mlx", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		preset                string
		width, height, frames int
		steps                 int
	}{
		{"low-memory", 640, 352, 41, 30},
		{"balanced", 832, 480, 41, 40},
		{"quality", 1280, 704, 81, 40},
	}
	for _, test := range tests {
		got, err := videoPreset(model, test.preset)
		if err != nil {
			t.Fatal(err)
		}
		want := videoDefaults{test.width, test.height, test.frames, test.steps}
		if got != want {
			t.Errorf("%s = %#v, want %#v", test.preset, got, want)
		}
	}
	if _, err := videoPreset(model, "extreme"); err == nil {
		t.Fatal("expected invalid preset to fail")
	}
}

func TestMiniMaxH3VideoPresets(t *testing.T) {
	model, err := catalog.ResolveForPlatform("minimax-h3", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	low, err := videoPreset(model, "low-memory")
	if err != nil {
		t.Fatal(err)
	}
	if low != (videoDefaults{640, 352, 73, 10}) {
		t.Fatalf("unexpected low-memory H3 preset: %#v", low)
	}
	balanced, err := videoPreset(model, "balanced")
	if err != nil {
		t.Fatal(err)
	}
	if balanced != (videoDefaults{864, 480, 73, 20}) {
		t.Fatalf("unexpected balanced H3 preset: %#v", balanced)
	}
}

func TestEnhanceVideoPrompt(t *testing.T) {
	got := enhanceVideoPrompt("A fox runs")
	for _, phrase := range []string{"A fox runs", "coherent temporal motion", "consistent subject appearance"} {
		if !strings.Contains(got, phrase) {
			t.Errorf("enhanced prompt missing %q: %s", phrase, got)
		}
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
