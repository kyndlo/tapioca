package videoruntime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/carlos/tapioca/internal/adapter"
)

func TestH3GraphChainsAdaptersIntoSchedulerAndGuider(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for the H3 graph contract test")
	}
	root := t.TempDir()
	workflow := filepath.Join(root, "workflow.json")
	output := filepath.Join(root, "graph.json")
	graph := map[string]any{
		"105:11":  map[string]any{"inputs": map[string]any{}},
		"105:24":  map[string]any{"inputs": map[string]any{}},
		"105:104": map[string]any{"inputs": map[string]any{}},
		"105:9":   map[string]any{"inputs": map[string]any{}},
		"105:16":  map[string]any{"inputs": map[string]any{}},
		"105:15":  map[string]any{"inputs": map[string]any{}},
		"105:91":  map[string]any{"inputs": map[string]any{}},
		"92":      map[string]any{"inputs": map[string]any{}},
	}
	data, _ := json.Marshal(graph)
	if err := os.WriteFile(workflow, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "h3_video.py",
		"--server", "http://127.0.0.1:1", "--workflow", workflow,
		"--comfy-root", root, "--backend", "comfy-h3-mps", "--model", root,
		"--prompt", "test", "--output", filepath.Join(root, "out.mp4"),
		"--width", "640", "--height", "352", "--frames", "73", "--steps", "10",
		"--fps", "24", "--seed", "1", "--adapter", "first.safetensors",
		"--adapter-scale", "0.8", "--adapter", "second.safetensors",
		"--adapter-scale", "0.4", "--dump-graph", output,
	)
	if result, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("patch graph: %v: %s", err, result)
	}
	patchedData, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var patched map[string]struct {
		ClassType string         `json:"class_type"`
		Inputs    map[string]any `json:"inputs"`
	}
	if err := json.Unmarshal(patchedData, &patched); err != nil {
		t.Fatal(err)
	}
	first := patched["tapioca:lora:0"]
	second := patched["tapioca:lora:1"]
	if first.ClassType != "LoraLoaderModelOnly" || first.Inputs["lora_name"] != "first.safetensors" {
		t.Fatalf("first LoRA node = %#v", first)
	}
	if second.ClassType != "LoraLoaderModelOnly" || second.Inputs["lora_name"] != "second.safetensors" {
		t.Fatalf("second LoRA node = %#v", second)
	}
	for _, node := range []string{"105:9", "105:16"} {
		model, ok := patched[node].Inputs["model"].([]any)
		if !ok || len(model) != 2 || model[0] != "tapioca:lora:1" {
			t.Fatalf("%s model input = %#v", node, patched[node].Inputs["model"])
		}
	}
}

func TestH3CUDAInstallsMatchedWheelsAfterComfyRequirements(t *testing.T) {
	commands := h3DependencyCommands("python", "requirements.txt", "comfy-h3-cuda")
	if len(commands) != 3 {
		t.Fatalf("commands = %#v", commands)
	}
	if got := commands[1].args; len(got) < 5 || got[3] != "-r" || got[4] != "requirements.txt" {
		t.Fatalf("requirements command = %v", got)
	}
	last := commands[2].args
	for _, want := range []string{"--force-reinstall", "--no-deps", "torch==2.11.0+cu128", "torchvision==0.26.0+cu128", "torchaudio==2.11.0+cu128"} {
		found := false
		for _, value := range last {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CUDA command missing %q: %v", want, last)
		}
	}
}

func TestH3CUDAServerAggressivelyOffloadsBetweenNodes(t *testing.T) {
	args := h3ServerArgs("C:/ComfyUI", "C:/extra.yaml", 8188, "comfy-h3-cuda")
	foundDisableSmartMemory := false
	for _, value := range args {
		if value == "--lowvram" {
			t.Fatalf("CUDA server args enable pathological low-VRAM mode: %v", args)
		}
		if value == "--disable-smart-memory" {
			foundDisableSmartMemory = true
		}
	}
	if !foundDisableSmartMemory {
		t.Fatalf("CUDA server args do not force inter-node offload: %v", args)
	}
	foundReserve := false
	for index, value := range args {
		if value == "--reserve-vram" && index+1 < len(args) && args[index+1] == "1" {
			foundReserve = true
		}
	}
	if !foundReserve {
		t.Fatalf("CUDA server args do not reserve 1 GiB: %v", args)
	}
}

func TestWindowsPythonCandidatesAvoidStoreAliasFirst(t *testing.T) {
	candidates := pythonCandidates("windows")
	if len(candidates) != 3 || candidates[0].name != "py" || candidates[1].name != "python" || candidates[2].name != "python3" {
		t.Fatalf("Windows candidates = %#v", candidates)
	}
}

func TestEngineForKeepsBackendDetailsBehindStableBoundary(t *testing.T) {
	tests := map[string]string{
		"mlx-video": "mlx", "diffusers-video": "diffusers",
		"comfy-h3-mps": "comfy-h3", "comfy-h3-cuda": "comfy-h3",
	}
	for backend, want := range tests {
		engine, err := engineFor(backend)
		if err != nil {
			t.Fatalf("engineFor(%q): %v", backend, err)
		}
		if engine.Name() != want {
			t.Errorf("engineFor(%q).Name() = %q, want %q", backend, engine.Name(), want)
		}
	}
}

func TestStageH3AdaptersUsesPrivateDeterministicNames(t *testing.T) {
	root := t.TempDir()
	adapterPath := filepath.Join(root, "source", "motion.safetensors")
	if err := os.MkdirAll(filepath.Dir(adapterPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(adapterPath, []byte("adapter"), 0o644); err != nil {
		t.Fatal(err)
	}
	items := []adapter.Local{{Reference: "hf://owner/motion", Path: adapterPath, Scale: 0.8}}
	first, err := stageH3Adapters(root, items)
	if err != nil {
		t.Fatal(err)
	}
	second, err := stageH3Adapters(root, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0] != second[0] || first[0] == filepath.Base(adapterPath) {
		t.Fatalf("unexpected staged names: first=%v second=%v", first, second)
	}
	staged := filepath.Join(root, "loras", first[0])
	data, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "adapter" {
		t.Fatalf("staged contents = %q", data)
	}
	if runtime.GOOS != "windows" {
		if sourceInfo, _ := os.Stat(adapterPath); sourceInfo == nil {
			t.Fatal("source adapter was changed")
		}
	}
}

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

func TestH3RuntimeSetupIntegration(t *testing.T) {
	cache := os.Getenv("TAPIOCA_H3_INTEGRATION_CACHE")
	if cache == "" {
		t.Skip("set TAPIOCA_H3_INTEGRATION_CACHE to install and verify the managed H3 runtime")
	}
	root := filepath.Join(cache, "video-runtime", "0.2.0-h3")
	comfy := filepath.Join(root, "ComfyUI")
	venv := filepath.Join(root, "venv")
	if err := ensureH3Runtime(
		context.Background(), root, comfy, venv, venvPython(venv), "comfy-h3-mps",
	); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(comfy, "main.py"),
		filepath.Join(comfy, "custom_nodes", "ComfyUI-GGUF"),
		filepath.Join(comfy, "custom_nodes", "ComfyUI-AppleSilicon-FP8"),
		filepath.Join(root, "h3_api.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("managed runtime component missing: %s: %v", path, err)
		}
	}
}
