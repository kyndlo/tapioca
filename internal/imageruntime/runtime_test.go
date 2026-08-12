package imageruntime

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/carlos/tapioca/internal/adapter"
)

func TestMFluxArgumentsIncludeEditingAndAdapters(t *testing.T) {
	command, args := mfluxArguments(Request{
		ModelPath: "/models/flux", Prompt: "edit", Output: "/out.png",
		Width: 1024, Height: 1024, Steps: 4, Seed: 7,
		InputImages: []string{"/one.png", "/two.png"},
		Adapters: []adapter.Local{
			{Path: "/a.safetensors", Scale: 0.8},
			{Path: "/b.safetensors", Scale: 0.6},
		},
	})
	if command != "mflux-generate-flux2-edit" {
		t.Fatalf("command = %q", command)
	}
	for _, value := range []string{
		"--image-paths", "/one.png", "/two.png",
		"--lora-paths", "/a.safetensors", "/b.safetensors",
		"--lora-scales", "0.8", "0.6",
	} {
		if !slices.Contains(args, value) {
			t.Fatalf("arguments do not contain %q: %v", value, args)
		}
	}
}

func TestDiffusersArgumentsIncludeRepeatedInputsAndAdapters(t *testing.T) {
	args := diffusersArguments("/runtime", Request{
		ModelPath: "/models/qwen", Prompt: "edit", Output: "/out.png",
		Width: 1024, Height: 1024, Steps: 20,
		InputImages: []string{"/one.png", "/two.png"},
		Adapters:    []adapter.Local{{Path: "/a.safetensors", Scale: 1}},
	})
	if count(args, "--image") != 2 || count(args, "--adapter") != 1 {
		t.Fatalf("unexpected repeated arguments: %v", args)
	}
	if len(args) < 2 || args[0] != "-P" || filepath.Base(args[1]) != "image_diffusion.py" {
		t.Fatalf("Diffusers script is not isolated from site imports: %v", args)
	}
	if filepath.Base(args[1]) == "diffusers.py" {
		t.Fatalf("runtime script shadows the diffusers package: %v", args)
	}
}

func TestONNXArgumentsSelectProvider(t *testing.T) {
	args := onnxArguments("/runtime", Request{
		ModelPath: "/models/sd-turbo", Prompt: "fox", Output: "/out.png",
		Width: 512, Height: 512, Steps: 4, Seed: 42, Backend: "onnx-directml",
	})
	provider := slices.Index(args, "--provider")
	if provider < 0 || provider+1 >= len(args) ||
		args[provider+1] != "DmlExecutionProvider" {
		t.Fatalf("unexpected ONNX arguments: %v", args)
	}
	if slices.Contains(args, "--image") || slices.Contains(args, "--adapter") {
		t.Fatalf("static ONNX arguments include unsupported options: %v", args)
	}
}

func TestONNXArgumentsSelectARMCPU(t *testing.T) {
	args := onnxArguments("/runtime", Request{Backend: "onnx-cpu"})
	provider := slices.Index(args, "--provider")
	if provider < 0 || provider+1 >= len(args) ||
		args[provider+1] != "CPUExecutionProvider" {
		t.Fatalf("unexpected ONNX arguments: %v", args)
	}
}

func count(values []string, target string) int {
	total := 0
	for _, value := range values {
		if value == target {
			total++
		}
	}
	return total
}
