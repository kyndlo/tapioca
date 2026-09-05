package catalog

import (
	"os"
	"strings"
	"testing"
)

func TestGraniteCandidateManifest(t *testing.T) {
	data, err := os.ReadFile("../../catalog/candidates/granite-4.2.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := decodeManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	model := manifest.Models["granite-4.2"]
	pin := model.Downloads[model.Default]
	if model.Repo != "ibm-granite/granite-4.2-3b-GGUF" || pin.Revision != "47a3d9699d7539606c83943d717fcea7bd9f6a19" || pin.SizeBytes != 2244012160 || pin.SHA256 != "20e436143017578687f7f848225cc6c6038126c84149192229c7dff6e4e0f427" {
		t.Fatalf("incorrect Granite candidate: %#v", model)
	}
	if DefaultContext("granite-4.2:3b-q4_k_m") != 8192 {
		t.Fatal("Granite must default to its qualified 8K context")
	}
}

func TestPinnedDownloadResolution(t *testing.T) {
	model := Model{Name: "test-pinned", Repo: "owner/model", Default: "q4", Files: map[string]string{"q4": "model.gguf"}, Downloads: map[string]Download{"q4": {Revision: strings.Repeat("a", 40), SHA256: strings.Repeat("b", 64), SizeBytes: 42}}}
	if err := validateModel(model.Name, model); err != nil {
		t.Fatal(err)
	}
	builtInModels[model.Name] = model
	t.Cleanup(func() { delete(builtInModels, model.Name) })
	result, err := ResolveForPlatform("test-pinned", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://huggingface.co/owner/model/resolve/"+strings.Repeat("a", 40)+"/model.gguf" || result.SizeBytes != 42 || result.SHA256 != strings.Repeat("b", 64) {
		t.Fatalf("lost pin metadata: %#v", result)
	}
}

func TestDownloadValidation(t *testing.T) {
	for _, invalid := range []Download{{Revision: "main"}, {Revision: "../main"}, {SHA256: "abc"}, {SizeBytes: -1}} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("accepted %#v", invalid)
		}
	}
	if (Download{}).Ref() != "main" {
		t.Fatal("legacy reference changed")
	}
}
