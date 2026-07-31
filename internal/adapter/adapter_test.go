package adapter

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		value string
		repo  string
		file  string
		scale float64
	}{
		{"hf://creator/cinematic-motion", "creator/cinematic-motion", "", 1},
		{"hf://creator/cinematic-motion@0.8", "creator/cinematic-motion", "", 0.8},
		{"hf://owner/repo#weights/model.safetensors", "owner/repo", "weights/model.safetensors", 1},
		{"hf://owner/repo#model.safetensors@0.65", "owner/repo", "model.safetensors", 0.65},
	}
	for _, test := range tests {
		got, err := Parse(test.value)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.value, err)
		}
		if got.Repo != test.repo || got.File != test.file || got.Scale != test.scale {
			t.Fatalf("Parse(%q) = %#v", test.value, got)
		}
	}
}

func TestParseRejectsUnsafeOrInvalidReferences(t *testing.T) {
	for _, value := range []string{
		"creator/repo",
		"hf://creator",
		"hf://creator/repo#../secret.safetensors",
		"hf://creator/repo@strong",
	} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("Parse(%q) unexpectedly succeeded", value)
		}
	}
}

func TestValidateCompatibility(t *testing.T) {
	item := Local{File: "cinematic_wan22.safetensors"}
	if err := ValidateCompatibility("wan2.2-video:5b-q8-mlx", "mlx-video", item); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCompatibility("ltx-video:2b-fp16", "diffusers-video", item); err == nil {
		t.Fatal("expected a Wan adapter to be rejected for an LTX base")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestInspectReadsBaseModelsAndSizes(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("blobs") != "true" {
			t.Fatalf("Inspect did not request blob sizes: %s", request.URL)
		}
		body := `{
			"sha":"revision",
			"pipeline_tag":"image-to-image",
			"cardData":{"base_model":["owner/base"],"license":"mit"},
			"siblings":[{"rfilename":"adapter.safetensors","size":1048576}]
		}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	metadata, err := Inspect(client, "owner/adapter")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Revision != "revision" || metadata.Files[0].Size != 1048576 ||
		len(metadata.Bases) != 1 {
		t.Fatalf("Inspect() = %#v", metadata)
	}
}

func TestResolveUsesExplicitCachedFileOffline(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(
		home, "adapters", "huggingface", "owner", "adapter", "weights.safetensors",
	)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("weights"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network should not be used")
	})}
	ref, err := Parse("hf://owner/adapter#weights.safetensors@0.8")
	if err != nil {
		t.Fatal(err)
	}
	local, err := Resolve(client, home, ref, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if local.Path != path || local.Scale != 0.8 {
		t.Fatalf("Resolve() = %#v", local)
	}
}
