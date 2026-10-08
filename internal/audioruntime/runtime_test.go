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

func TestCUDAWheelSelectionExcludesARM64(t *testing.T) {
	if !usesCUDATorch("windows", "amd64") || !usesCUDATorch("linux", "amd64") {
		t.Fatal("x64 NVIDIA platforms must install the CUDA wheel")
	}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		if usesCUDATorch(goos, "arm64") {
			t.Fatalf("%s/arm64 must use the default CPU-compatible dependency", goos)
		}
	}
}
