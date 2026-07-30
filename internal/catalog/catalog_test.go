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
