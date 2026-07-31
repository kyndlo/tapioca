package app

import (
	"runtime"
	"testing"
)

func TestResolveMediaModelAcceptsHuggingFaceReference(t *testing.T) {
	resolved, err := resolveMediaModel("hf://owner/custom-image-model", "image")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Repo != "owner/custom-image-model" || resolved.Kind != "image" {
		t.Fatalf("resolveMediaModel() = %#v", resolved)
	}
	if runtime.GOOS == "darwin" && resolved.Backend != "mflux" {
		t.Fatalf("Apple Silicon image backend = %q", resolved.Backend)
	}
}

func TestResolveMediaModelRejectsUnsafeReference(t *testing.T) {
	if _, err := resolveMediaModel("hf://owner/../model", "image"); err == nil {
		t.Fatal("unsafe Hugging Face reference unexpectedly succeeded")
	}
}
