package audioruntime

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestPythonArguments(t *testing.T) {
	args := pythonArguments("/runtime", Request{
		ModelPath: "/models/stable-audio", Prompt: "bright synth loop",
		NegativePrompt: "vocals", Output: "/out.wav", Duration: 12, Steps: 8, Seed: 42,
	})
	for _, value := range []string{
		"--model", "/models/stable-audio", "--prompt", "bright synth loop",
		"--negative-prompt", "vocals", "--duration", "12", "--steps", "8", "--seed", "42",
	} {
		if !slices.Contains(args, value) {
			t.Fatalf("arguments missing %q: %v", value, args)
		}
	}
	if filepath.Base(args[0]) != "stable_audio3.py" {
		t.Fatalf("unexpected runtime script: %v", args)
	}
}
