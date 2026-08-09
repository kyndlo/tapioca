package control

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/imageruntime"
	"github.com/carlos/tapioca/internal/speechruntime"
	"github.com/carlos/tapioca/internal/videoruntime"
)

func TestCreatorCapabilitiesReportProtocolSafeAvailability(t *testing.T) {
	capabilities := creatorCapabilities()
	if capabilities["image"].(map[string]any)["available"] != true {
		t.Fatal("image capability is not available")
	}
	if capabilities["video"].(map[string]any)["available"] != true {
		t.Fatal("video capability is not available")
	}
	speech := capabilities["speech"].(map[string]any)
	if speech["available"] != true || speech["supports_voice_reference"] != true {
		t.Fatalf("speech capability = %#v", speech)
	}
	if capabilities["outputs"].(map[string]any)["binary_in_protocol"] != false {
		t.Fatal("creator protocol must not transport binary output")
	}
	progress := capabilities["progress"].(map[string]any)
	if progress["mode"] != "indeterminate" || progress["numeric_when_available"] != false {
		t.Fatalf("creator progress contract = %#v", progress)
	}
}

func TestCreatorCatalogContainsCompatibilityMetadata(t *testing.T) {
	handler := NewHandler(Dependencies{})
	result, err := handler.Handle(context.Background(), Request{
		Method: "creator.catalog",
	})
	if err != nil {
		t.Fatalf("creator.catalog error = %v", err)
	}
	models := result.([]CreatorCatalogModel)
	if len(models) == 0 {
		t.Fatal("creator catalog is empty")
	}
	var foundSpeech bool
	for _, model := range models {
		if model.Kind == "speech" {
			foundSpeech = true
			if !model.Available || model.Operation != "speech.generate" {
				t.Fatalf("speech compatibility = %#v", model)
			}
		}
	}
	if !foundSpeech {
		t.Fatal("creator catalog did not include speech compatibility records")
	}
}

func TestImageGenerateUsesRuntimeAdapterAndManagedOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	modelPath := filepath.Join(home, "models", "flux2-klein-4b-q4-mlx")
	if err := os.MkdirAll(modelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	saveCreatorModel(t, config.Model{
		Name: "flux2-klein:4b-q4-mlx", Path: modelPath,
		Kind: "image", Backend: "mflux",
	})
	var received imageruntime.Request
	handler := NewHandler(Dependencies{
		Image: func(
			_ context.Context,
			cacheDir string,
			request imageruntime.Request,
			stdout io.Writer,
			stderr io.Writer,
		) error {
			received = request
			if !withinRoot(home, cacheDir) {
				t.Fatalf("cache directory is not managed: %s", cacheDir)
			}
			_, _ = stdout.Write([]byte("generating"))
			_, _ = stderr.Write([]byte("progress"))
			return os.WriteFile(request.Output, []byte("png"), 0o644)
		},
	})
	result, err := handler.Handle(context.Background(), Request{
		ID: "image-request", Method: "image.generate",
		Params: []byte(`{"model":"flux2-klein:4b-q4-mlx","prompt":"A fox","width":512,"height":512,"steps":4}`),
	})
	if err != nil {
		t.Fatalf("image.generate error = %v", err)
	}
	output := result.(CreatorOutput)
	if output.Kind != "image" || output.MIME != "image/png" || output.Bytes != 3 {
		t.Fatalf("image output = %#v", output)
	}
	if !withinRoot(filepath.Join(home, "outputs", "images"), output.Path) {
		t.Fatalf("output escaped managed root: %s", output.Path)
	}
	if received.ModelPath != modelPath || received.Prompt != "A fox" {
		t.Fatalf("runtime request = %#v", received)
	}
}

func TestCreatorProgressIsExplicitlyIndeterminate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	modelPath := filepath.Join(home, "models", "flux2-klein-4b-q4-mlx")
	if err := os.MkdirAll(modelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	saveCreatorModel(t, config.Model{
		Name: "flux2-klein:4b-q4-mlx", Path: modelPath,
		Kind: "image", Backend: "mflux",
	})
	handler := NewHandler(Dependencies{
		Image: func(
			_ context.Context,
			_ string,
			request imageruntime.Request,
			_ io.Writer,
			_ io.Writer,
		) error {
			return os.WriteFile(request.Output, []byte("png"), 0o644)
		},
	})
	input := strings.NewReader(
		`{"version":1,"type":"request","id":"creator-progress","method":"image.generate","params":{"model":"flux2-klein:4b-q4-mlx","prompt":"fox","width":512,"height":512,"steps":4},"job_id":"job-creator"}` + "\n",
	)
	var output bytes.Buffer
	if err := NewServer(input, &output, handler).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil ||
			event.Event != string(EventJobProgress) {
			continue
		}
		data := event.Data.(map[string]any)
		if data["determinate"] != false {
			t.Fatalf("progress event = %#v", event)
		}
		if _, fabricated := data["percent"]; fabricated {
			t.Fatalf("progress event fabricated a percentage: %#v", event)
		}
		found = true
	}
	if !found {
		t.Fatalf("no creator progress event in:\n%s", output.String())
	}
}

func TestVideoGenerateValidatesAndPassesInputImage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	modelPath := filepath.Join(home, "models", "ltx-video-2b-fp16")
	if err := os.MkdirAll(modelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	saveCreatorModel(t, config.Model{
		Name: "ltx-video:2b-fp16", Path: modelPath,
		Kind: "video", Backend: "diffusers-video",
	})
	input := filepath.Join(home, "input.png")
	if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	var received videoruntime.Request
	handler := NewHandler(Dependencies{
		Video: func(
			_ context.Context,
			_ string,
			request videoruntime.Request,
			_ io.Writer,
			_ io.Writer,
		) error {
			received = request
			return os.WriteFile(request.Output, []byte("mp4"), 0o644)
		},
	})
	result, err := handler.Handle(context.Background(), Request{
		ID: "video-request", Method: "video.generate",
		Params: []byte(`{"model":"ltx-video:2b-fp16","prompt":"A fox runs","input_image":"` +
			input + `","width":768,"height":512,"frames":25,"steps":20,"fps":8}`),
	})
	if err != nil {
		t.Fatalf("video.generate error = %v", err)
	}
	if result.(CreatorOutput).MIME != "video/mp4" || received.InputImage != input {
		t.Fatalf("video result=%#v request=%#v", result, received)
	}
}

func TestImageGenerationCancellationReturnsStableError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	modelPath := filepath.Join(home, "models", "flux2-klein-4b-q4-mlx")
	if err := os.MkdirAll(modelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	saveCreatorModel(t, config.Model{
		Name: "flux2-klein:4b-q4-mlx", Path: modelPath,
		Kind: "image", Backend: "mflux",
	})
	handler := NewHandler(Dependencies{
		Image: func(
			ctx context.Context,
			_ string,
			_ imageruntime.Request,
			_ io.Writer,
			_ io.Writer,
		) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := handler.Handle(ctx, Request{
		ID: "cancel-image", Method: "image.generate",
		Params: []byte(`{"model":"flux2-klein:4b-q4-mlx","prompt":"fox","width":512,"height":512,"steps":4}`),
	})
	if err == nil || err.Code != "job_cancelled" {
		t.Fatalf("cancelled image error = %#v", err)
	}
}

func TestCreatorRejectsUnsafePathsAndParameters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	if _, err := managedOutputPath("images", "id", "../escape.png", ".png"); err == nil ||
		err.Code != "invalid_params" {
		t.Fatalf("unsafe output error = %#v", err)
	}
	target := filepath.Join(home, "target.png")
	link := filepath.Join(home, "link.png")
	if err := os.WriteFile(target, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := validateInputFiles([]string{link}, imageExtensions, 1024); err == nil ||
		err.Code != "invalid_params" {
		t.Fatalf("symlink input error = %#v", err)
	}
	if err := validateDimensions(513, 512, 4); err == nil {
		t.Fatal("non-multiple-of-eight width was accepted")
	}
	invalidScale := 5.0
	if _, err := resolveLoRAs([]LoRASelection{{
		Reference: "hf://owner/repo", Scale: &invalidScale,
	}}, "model", "mflux"); err == nil || err.Code != "invalid_params" {
		t.Fatalf("invalid LoRA scale error = %#v", err)
	}
}

func TestSpeechGenerateUsesRuntimeAdapterAndManagedOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	modelPath := filepath.Join(home, "models", "chatterbox-multilingual")
	if err := os.MkdirAll(modelPath, 0o755); err != nil {
		t.Fatal(err)
	}
	saveCreatorModel(t, config.Model{
		Name: "chatterbox:multilingual", Path: modelPath,
		Kind: "speech", Backend: "speech-chatterbox",
	})
	var received speechruntime.Request
	handler := NewHandler(Dependencies{
		Speech: func(_ context.Context, _ string, request speechruntime.Request, _, _ io.Writer) error {
			received = request
			return os.WriteFile(request.Output, []byte("RIFF-test"), 0o600)
		},
	})
	result, err := handler.Handle(context.Background(), Request{
		ID: "speech-request", Method: "speech.generate",
		Params: []byte(`{"model":"chatterbox:multilingual","text":"hello"}`),
	})
	if err != nil {
		t.Fatalf("speech.generate error = %v", err)
	}
	output := result.(CreatorOutput)
	if received.Text != "hello" || received.Backend != "speech-chatterbox" {
		t.Fatalf("speech request = %#v", received)
	}
	if output.Kind != "audio" || !withinRoot(filepath.Join(home, "outputs", "audio"), output.Path) {
		t.Fatalf("speech output = %#v", output)
	}
}

func TestLoRAListDiscoversOnlySafetensors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	root := filepath.Join(home, "adapters", "huggingface", "owner", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "motion.safetensors"), []byte("lora"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := listLoRAs()
	if err != nil {
		t.Fatalf("listLoRAs error = %v", err)
	}
	items := result.([]map[string]any)
	if len(items) != 1 || !strings.HasSuffix(items[0]["file"].(string), "motion.safetensors") {
		t.Fatalf("LoRA list = %#v", items)
	}
}

func TestLoRAListReturnsEmptyArrayWhenNoAdaptersAreInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)

	result, err := listLoRAs()
	if err != nil {
		t.Fatalf("listLoRAs error = %v", err)
	}
	items := result.([]map[string]any)
	if items == nil || len(items) != 0 {
		t.Fatalf("LoRA list = %#v, want non-nil empty slice", items)
	}
}

func saveCreatorModel(t *testing.T, model config.Model) {
	t.Helper()
	registry := config.Registry{Models: map[string]config.Model{
		strings.ToLower(model.Name): model,
	}}
	if err := registry.Save(); err != nil {
		t.Fatal(err)
	}
}
