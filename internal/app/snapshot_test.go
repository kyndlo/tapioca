package app

import (
	"context"
	"strings"
	"testing"

	"github.com/carlos/tapioca/internal/catalog"
)

func TestImageFP16SnapshotFile(t *testing.T) {
	included := []string{
		"model_index.json",
		"scheduler/scheduler_config.json",
		"text_encoder/model.fp16.safetensors",
		"text_encoder_2/model.fp16.safetensors",
		"tokenizer_2/tokenizer_config.json",
		"unet/diffusion_pytorch_model.fp16.safetensors",
		"vae/diffusion_pytorch_model.fp16.safetensors",
	}
	for _, name := range included {
		if !imageFP16SnapshotFile(name) {
			t.Errorf("expected %s to be included", name)
		}
	}

	excluded := []string{
		"sd_xl_turbo_1.0_fp16.safetensors",
		"unet/diffusion_pytorch_model.safetensors",
		"unet/model.onnx",
		"unet/model.onnx_data",
		"README.md",
	}
	for _, name := range excluded {
		if imageFP16SnapshotFile(name) {
			t.Errorf("expected %s to be excluded", name)
		}
	}
}

func TestImageFP16SnapshotIncludesVideoFeatureExtractor(t *testing.T) {
	if !imageFP16SnapshotFile("feature_extractor/preprocessor_config.json") {
		t.Fatal("expected video feature extractor config to be included")
	}
	if !imageFP16SnapshotFile("unet/diffusion_pytorch_model.fp16.safetensors") {
		t.Fatal("expected fp16 video UNet to be included")
	}
	if imageFP16SnapshotFile("unet/diffusion_pytorch_model.safetensors") {
		t.Fatal("did not expect duplicate full-precision video UNet")
	}
}

func TestImageSnapshotIncludesSplitONNXVAE(t *testing.T) {
	for _, name := range []string{
		"vae_decoder/model.onnx",
		"vae_encoder/model.onnx",
		"unet/model.onnx_data",
	} {
		if !imageSnapshotFile(name) {
			t.Errorf("imageSnapshotFile(%q) = false", name)
		}
	}
}

func TestLicensedImageSnapshotIncludesTermsButNotDuplicateWeights(t *testing.T) {
	for _, name := range []string{"README.md", "LICENSE.pdf", "transformer/diffusion_pytorch_model-00001-of-00003.safetensors"} {
		if !licensedImageSnapshotFile(name) {
			t.Errorf("licensedImageSnapshotFile(%q) = false", name)
		}
	}
	if licensedImageSnapshotFile("turbo.safetensors") {
		t.Fatal("top-level duplicate checkpoint should not be downloaded")
	}
}

func TestPullArtifactsRejectsEscapingTarget(t *testing.T) {
	err := pullArtifactsWithContext(
		context.Background(),
		catalog.Resolved{
			Name: "unsafe:test",
			Artifacts: []catalog.Artifact{{
				Repo: "owner/repo", Filename: "model.bin", Target: "../model.bin",
			}},
		},
		t.TempDir(),
		false,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "invalid artifact target") {
		t.Fatalf("unexpected error: %v", err)
	}
}
