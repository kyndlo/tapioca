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
