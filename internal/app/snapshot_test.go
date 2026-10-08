package app

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos/tapioca/internal/catalog"
)

func TestPinnedSnapshotUsesImmutableMetadataAndFileURLs(t *testing.T) {
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	revision := strings.Repeat("d", 40)
	requests := []string{}
	http.DefaultClient = &http.Client{Transport: artifactTransport(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.String())
		body := "pinned model"
		if strings.Contains(request.URL.Path, "/api/models/") {
			body = `{"siblings":[{"rfilename":"model_index.json"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)),
		}, nil
	})}
	model := catalog.Resolved{
		Download: catalog.Download{Revision: revision},
		Name:     "qwen-image-2.1:bf16-cuda", Repo: "Qwen/Qwen-Image-2.1", Kind: "image",
	}
	root := t.TempDir()
	if err := pullHubSnapshotWithContext(context.Background(), model, root, false, imageSnapshotFile, nil); err != nil {
		t.Fatal(err)
	}
	wantMetadata := "https://huggingface.co/api/models/Qwen/Qwen-Image-2.1/revision/" + revision
	wantFile := "https://huggingface.co/Qwen/Qwen-Image-2.1/resolve/" + revision + "/model_index.json"
	if len(requests) != 2 || requests[0] != wantMetadata || requests[1] != wantFile {
		t.Fatalf("unpinned snapshot requests: %#v", requests)
	}
	marker, err := os.ReadFile(filepath.Join(root, ".tapioca-snapshot-revision"))
	if err != nil || strings.TrimSpace(string(marker)) != revision {
		t.Fatalf("snapshot revision marker = %q, %v", marker, err)
	}
	if err := pullHubSnapshotWithContext(context.Background(), model, root, false, imageSnapshotFile, nil); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 || requests[2] != wantMetadata {
		t.Fatalf("matching snapshot pin unexpectedly downloaded weights: %#v", requests)
	}
}

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
	if licensedImageSnapshotFile("raw.safetensors") {
		t.Fatal("top-level Raw duplicate checkpoint should not be downloaded")
	}
}

func TestGatedAudioSnapshotKeepsWeightsAndLocalTextEncoder(t *testing.T) {
	filter := snapshotFileFilter(catalog.Resolved{Kind: "audio", Gated: true})
	for _, name := range []string{
		"model_config.json",
		"model.safetensors",
		"t5gemma-b-b-ul2/model-00001-of-00002.safetensors",
		"t5gemma-b-b-ul2/tokenizer.json",
	} {
		if !filter(name) {
			t.Errorf("gated audio filter excluded %q", name)
		}
	}
	if filter("Stable_Audio_3.0_Thumbnail_1x1.png") {
		t.Fatal("gated audio filter included a preview image")
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
