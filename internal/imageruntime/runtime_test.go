package imageruntime

import (
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
