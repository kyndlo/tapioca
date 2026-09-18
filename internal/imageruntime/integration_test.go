package imageruntime

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Set TAPIOCA_DIFFUSERS_SMOKE_MODEL to a locally downloaded Diffusers model
// directory to exercise the pinned runtime on real hardware. This is opt-in
// because it installs PyTorch and generates an image.
func TestDiffusersHardwareSmoke(t *testing.T) {
	modelPath := os.Getenv("TAPIOCA_DIFFUSERS_SMOKE_MODEL")
	if modelPath == "" {
		t.Skip("set TAPIOCA_DIFFUSERS_SMOKE_MODEL for a hardware smoke test")
	}
	backend := "diffusers"
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		backend = "diffusers-mps"
	}
	output := filepath.Join(t.TempDir(), "smoke.png")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := Run(ctx, filepath.Join(t.TempDir(), "runtime"), Request{
		ModelPath: modelPath, Prompt: "A red sphere on a white table",
		Output: output, Width: 512, Height: 512, Steps: 2, Seed: 1,
		Backend: backend,
	}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	image, err := png.DecodeConfig(file)
	if err != nil || image.Width != 512 || image.Height != 512 {
		t.Fatalf("invalid generated PNG: dimensions=%dx%d, error=%v", image.Width, image.Height, err)
	}
}
