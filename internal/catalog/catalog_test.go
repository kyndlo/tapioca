package catalog

import "testing"

func TestResolveGLM(t *testing.T) {
	got, err := Resolve("glm-4.7-flash:q8_0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "GLM-4.7-Flash-Q8_0.gguf" {
		t.Fatalf("unexpected filename %q", got.Filename)
	}
	if got.Name != "glm-4.7-flash:q8_0" {
		t.Fatalf("unexpected name %q", got.Name)
	}
}

func TestResolveImageDefault(t *testing.T) {
	got, err := ResolveFor("qwen-image-flash", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "qwen-image-flash:int8" || got.Kind != "image" ||
		got.Repo != "mlx-community/Qwen-Image-Flash-8bit" || got.Filename != "" {
		t.Fatalf("unexpected image resolution: %#v", got)
	}
}

func TestResolveImageDefaultsToDiffusersOnWindows(t *testing.T) {
	got, err := ResolveFor("qwen-image-flash", "windows")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "qwen-image-flash:bf16" ||
		got.Repo != "nvidia/Qwen-Image-Flash" || got.Backend != "diffusers" {
		t.Fatalf("unexpected Windows image resolution: %#v", got)
	}
}

func TestResolveQwenMLXAlias(t *testing.T) {
	got, err := ResolveFor("qwen3.6:35b-mlx", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != "mlx-community/Qwen3.6-35B-A3B-4bit" ||
		got.Backend != "mlx-vlm" || got.Kind != "text" || got.Filename != "" {
		t.Fatalf("unexpected Qwen MLX resolution: %#v", got)
	}
}

func TestResolveCompactWindowsModels(t *testing.T) {
	tests := map[string]string{
		"qwen3:4b-q4_k_m": "Qwen3-4B-Q4_K_M.gguf",
		"qwen3:8b-q4_k_m": "Qwen3-8B-Q4_K_M.gguf",
		"phi4-mini":       "Phi-4-mini-instruct-Q4_K_M.gguf",
		"gemma3":          "gemma-3-4b-it-Q4_K_M.gguf",
	}
	for ref, filename := range tests {
		t.Run(ref, func(t *testing.T) {
			got, err := ResolveForPlatform(ref, "windows", "amd64")
			if err != nil {
				t.Fatal(err)
			}
			if got.Filename != filename || got.URL == "" || got.Backend != "" {
				t.Fatalf("unexpected resolution for %s: %#v", ref, got)
			}
		})
	}
}

func TestResolveAppleSiliconMLXModels(t *testing.T) {
	tests := map[string]string{
		"qwen3:30b-mlx":       "mlx-community/Qwen3-30B-A3B-4bit",
		"qwen3-coder:30b-mlx": "mlx-community/Qwen3-Coder-30B-A3B-Instruct-4bit",
		"gemma3:12b-mlx":      "mlx-community/gemma-3-12b-it-4bit",
		"gemma3:27b-mlx":      "mlx-community/gemma-3-27b-it-4bit",
	}
	for ref, repo := range tests {
		t.Run(ref, func(t *testing.T) {
			got, err := ResolveFor(ref, "darwin")
			if err != nil {
				t.Fatal(err)
			}
			if got.Repo != repo || got.Backend != "mlx-vlm" || got.Filename != "" {
				t.Fatalf("unexpected resolution for %s: %#v", ref, got)
			}
		})
	}
}

func TestResolveTurboImageProfiles(t *testing.T) {
	tests := map[string]struct {
		width  int
		height int
	}{
		"sd-turbo":   {width: 512, height: 512},
		"sdxl-turbo": {width: 1024, height: 1024},
	}
	for ref, expected := range tests {
		t.Run(ref, func(t *testing.T) {
			got, err := ResolveForPlatform(ref, "windows", "amd64")
			if err != nil {
				t.Fatal(err)
			}
			if got.Backend != "diffusers" || got.Width != expected.width ||
				got.Height != expected.height || got.Steps != 4 {
				t.Fatalf("unexpected image profile for %s: %#v", ref, got)
			}
		})
	}
}

func TestResolveKrea2TurboProfiles(t *testing.T) {
	mac, err := ResolveForPlatform("krea-2-turbo", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if mac.Name != "krea-2-turbo:bf16-mps" || mac.Backend != "diffusers-mps" ||
		!mac.Gated || mac.Steps != 8 || mac.LicenseURL == "" {
		t.Fatalf("unexpected macOS Krea profile: %#v", mac)
	}
	windows, err := ResolveForPlatform("krea-2-turbo", "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if windows.Name != "krea-2-turbo:bf16-cuda" || windows.Backend != "diffusers" ||
		windows.Platform != "Windows/Linux NVIDIA" {
		t.Fatalf("unexpected Windows Krea profile: %#v", windows)
	}
}

func TestResolveWindowsDiffusionProfiles(t *testing.T) {
	directml, err := ResolveForPlatform("sd-turbo:onnx-directml", "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if directml.Backend != "onnx-directml" ||
		directml.Platform != "Windows x64 AMD/Intel/NVIDIA" ||
		directml.Repo != "Heliosoph/sd-turbo-onnx" {
		t.Fatalf("unexpected DirectML profile: %#v", directml)
	}

	arm, err := ResolveForPlatform("sd-turbo", "windows", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if arm.Name != "sd-turbo:onnx-arm64" || arm.Backend != "onnx-cpu" ||
		arm.Platform != "Windows ARM64" {
		t.Fatalf("unexpected ARM64 profile: %#v", arm)
	}
}

func TestAllCatalogRefsResolve(t *testing.T) {
	for _, ref := range Refs() {
		if _, err := ResolveFor(ref, "darwin"); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
}

func TestCatalogRequirementsArePresent(t *testing.T) {
	for _, ref := range Refs() {
		model, err := ResolveFor(ref, "darwin")
		if err != nil {
			t.Fatalf("ResolveFor(%q): %v", ref, err)
		}
		if model.Size == "" || model.Memory == "" || model.GPU == "" || model.Platform == "" {
			t.Errorf("%s has incomplete requirements: %#v", ref, model)
		}
	}
}

func TestLowMemoryVideoProfiles(t *testing.T) {
	tests := []struct {
		ref      string
		backend  string
		platform string
	}{
		{"wan2.2-video:5b-q8-mlx", "mlx-video", "macOS Apple Silicon"},
		{"yume-video:5b-mlx", "mlx-video", "macOS Apple Silicon"},
		{"ltx-video:2b-fp16", "diffusers-video", "Windows/Linux NVIDIA"},
		{"stable-video-diffusion:xt-fp16", "diffusers-video", "Windows/Linux NVIDIA"},
	}
	for _, test := range tests {
		model, err := ResolveFor(test.ref, "darwin")
		if err != nil {
			t.Fatalf("ResolveFor(%q): %v", test.ref, err)
		}
		if model.Kind != "video" || model.Backend != test.backend || model.Platform != test.platform {
			t.Errorf("%s resolved to %#v", test.ref, model)
		}
		if model.Width == 0 || model.Height == 0 || model.Frames == 0 || model.FPS == 0 {
			t.Errorf("%s lacks generation defaults: %#v", test.ref, model)
		}
	}
}

func TestResolveMiniMaxH3PlatformBundles(t *testing.T) {
	mac, err := ResolveForPlatform("minimax-h3", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if mac.Name != "minimax-h3:fl2va-int8-mac" ||
		mac.Backend != "comfy-h3-mps" || len(mac.Artifacts) != 4 {
		t.Fatalf("unexpected macOS MiniMax-H3 bundle: %#v", mac)
	}
	if mac.Artifacts[1].Repo != "realrebelai/MiniMax-H3_GGUFs" ||
		mac.Artifacts[1].Target != "text_encoders/qwen3vl-32B-MiniMax-H3-Q4_K_M.gguf" {
		t.Fatalf("unexpected macOS text encoder: %#v", mac.Artifacts[1])
	}

	windows, err := ResolveForPlatform("minimax-h3", "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if windows.Name != "minimax-h3:fl2va-int8-cuda" ||
		windows.Backend != "comfy-h3-cuda" || len(windows.Artifacts) != 4 {
		t.Fatalf("unexpected Windows MiniMax-H3 bundle: %#v", windows)
	}
	if windows.Artifacts[1].Filename !=
		"text_encoders/qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors" {
		t.Fatalf("unexpected CUDA text encoder: %#v", windows.Artifacts[1])
	}
}

func TestSpeechProfiles(t *testing.T) {
	mac, err := ResolveForPlatform("qwen3-tts", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if mac.Name != "qwen3-tts:0.6b-mlx" || mac.Backend != "speech-qwen-mlx" ||
		mac.Kind != "speech" {
		t.Fatalf("unexpected macOS speech profile: %#v", mac)
	}
	linux, err := ResolveForPlatform("qwen3-tts", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if linux.Name != "qwen3-tts:0.6b" || linux.Backend != "speech-qwen" ||
		linux.Platform != "Windows/Linux NVIDIA or CPU" {
		t.Fatalf("unexpected Linux speech profile: %#v", linux)
	}
	portable, err := ResolveForPlatform("chatterbox:nano", "linux", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if portable.Backend != "speech-chatterbox" || portable.Languages != "English" ||
		portable.Features == "" {
		t.Fatalf("unexpected portable speech profile: %#v", portable)
	}
}

func TestLowMemoryMacImageProfile(t *testing.T) {
	model, err := ResolveFor("flux2-klein:4b-q4-mlx", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if model.Kind != "image" || model.Backend != "mflux" {
		t.Fatalf("unexpected profile: %#v", model)
	}
	if model.Memory != "16 GiB min; 24 GiB recommended" ||
		model.GPU != "Apple Silicon GPU" {
		t.Fatalf("unexpected requirements: %#v", model)
	}
}
