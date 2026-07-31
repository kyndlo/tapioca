package videoruntime

import (
	"testing"

	"github.com/carlos/tapioca/internal/adapter"
)

func TestPythonArgumentsIncludeRepeatedAdapters(t *testing.T) {
	args := pythonArguments("/runtime", "mlx_video.py", Request{
		ModelPath: "/models/wan", Prompt: "move", Output: "/out.mp4",
		Width: 640, Height: 352, Frames: 41, Steps: 30, FPS: 24,
		Adapters: []adapter.Local{
			{Path: "/motion.safetensors", Scale: 0.8},
			{Path: "/character.safetensors", Scale: 0.6},
		},
	})
	adapterFlags := 0
	for _, value := range args {
		if value == "--adapter" {
			adapterFlags++
		}
	}
	if adapterFlags != 2 {
		t.Fatalf("expected 2 adapter flags, got %d: %v", adapterFlags, args)
	}
}
