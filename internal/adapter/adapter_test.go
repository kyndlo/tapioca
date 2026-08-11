package adapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
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
		kind  Provider
	}{
		{"hf://creator/cinematic-motion", "creator/cinematic-motion", "", 1, ProviderHuggingFace},
		{"hf://creator/cinematic-motion@0.8", "creator/cinematic-motion", "", 0.8, ProviderHuggingFace},
		{"hf://owner/repo#weights/model.safetensors", "owner/repo", "weights/model.safetensors", 1, ProviderHuggingFace},
		{"hf://owner/repo#model.safetensors@0.65", "owner/repo", "model.safetensors", 0.65, ProviderHuggingFace},
		{"civitai://2830065/3193337#model.safetensors@0.5", "2830065/3193337", "model.safetensors", 0.5, ProviderCivitai},
		{"ms://owner/repo#adapter.safetensors", "owner/repo", "adapter.safetensors", 1, ProviderModelScope},
		{"modelscope://owner/repo", "owner/repo", "", 1, ProviderModelScope},
		{"local://cinematic@0.7", "cinematic", "", 0.7, ProviderLocal},
	}
	for _, test := range tests {
		got, err := Parse(test.value)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.value, err)
		}
		if got.Repo != test.repo || got.File != test.file || got.Scale != test.scale ||
			got.Provider != test.kind {
			t.Fatalf("Parse(%q) = %#v", test.value, got)
		}
	}
}

func TestParseCivitaiWebURL(t *testing.T) {
	got, err := Parse("https://civitai.red/models/2830065/name?modelVersionId=3193337")
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != ProviderCivitai || got.Repo != "2830065/3193337" {
		t.Fatalf("Parse() = %#v", got)
	}
}

func TestParseRejectsUnsafeOrInvalidReferences(t *testing.T) {
	for _, value := range []string{
		"creator/repo",
		"hf://creator",
		"hf://creator/repo#../secret.safetensors",
		`hf://creator/repo#..\secret.safetensors`,
		"hf://creator/repo@strong",
		"civitai://model/version",
		"local://../unsafe",
		"ms://owner/repo#adapter.ckpt",
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
	ref, err := Parse("hf://owner/adapter")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := Inspect(client, ref)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Revision != "revision" || metadata.Files[0].Size != 1048576 ||
		len(metadata.Bases) != 1 {
		t.Fatalf("Inspect() = %#v", metadata)
	}
}

func TestInspectCivitaiSelectsVersionAndChecksum(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v1/models/12" {
			t.Fatalf("unexpected Civitai URL %s", request.URL)
		}
		body := `{"id":12,"type":"LORA","baseModels":["MiniMax H3"],"modelVersions":[{"id":34,"baseModel":"MiniMax H3","files":[{"name":"h3.safetensors","sizeKB":2,"hashes":{"SHA256":"ABCDEF"},"downloadUrl":"https://example.test/h3"}]}]}`
		return response(body), nil
	})}
	ref, _ := Parse("civitai://12/34")
	metadata, err := Inspect(client, ref)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Type != "LORA" || metadata.Files[0].Size != 2048 ||
		metadata.Files[0].SHA256 != "abcdef" || metadata.Files[0].DownloadURL == "" {
		t.Fatalf("Inspect() = %#v", metadata)
	}
}

func TestCivitaiRejectsCheckpointModelsAndUntrustedDownloadHosts(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(`{"id":12,"type":"Checkpoint","modelVersions":[{"id":34,"files":[{"name":"model.safetensors","downloadUrl":"https://civitai.com/api/download/models/34"}]}]}`), nil
	})}
	ref, _ := Parse("civitai://12/34")
	if _, err := Resolve(client, t.TempDir(), ref, "", nil); err == nil ||
		!strings.Contains(err.Error(), "not a LoRA") {
		t.Fatalf("checkpoint Resolve() error = %v", err)
	}
	if _, err := adapterDownloadURL(Local{
		Provider: ProviderCivitai, Repo: "12/34",
		DownloadURL: "https://example.test/weights.safetensors",
	}); err == nil {
		t.Fatal("untrusted Civitai download host was accepted")
	}
}

func TestInspectModelScopeReadsOpenAPIAndFileTree(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/openapi/v1/models/owner/repo":
			return response(`{"success":true,"data":{"id":"owner/repo","license":"apache-2.0","tasks":["text-to-video"],"tags":["library:lora","base_model:MiniMaxAI/MiniMax-H3"]}}`), nil
		case "/api/v1/models/owner/repo/repo/files":
			return response(`{"Code":200,"Data":{"LatestCommitter":{"Id":"revision"},"Files":[{"Path":"weights/h3.safetensors","Size":4096,"Sha256":"AABB","Revision":"revision","Type":"blob"}]}}`), nil
		default:
			t.Fatalf("unexpected ModelScope URL %s", request.URL)
			return nil, nil
		}
	})}
	ref, _ := Parse("ms://owner/repo")
	metadata, err := Inspect(client, ref)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Type != "LORA" || metadata.Revision != "revision" ||
		metadata.Files[0].Name != "weights/h3.safetensors" || len(metadata.Bases) != 1 {
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

func TestImportAndListManagedLocalAdapter(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(t.TempDir(), "Cinematic Motion.safetensors")
	if err := os.WriteFile(source, safeTensorBytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	local, err := Import(home, source, "", "minimax-h3", false)
	if err != nil {
		t.Fatal(err)
	}
	if local.Provider != ProviderLocal || local.Reference !=
		"local://cinematic-motion#Cinematic Motion.safetensors" {
		t.Fatalf("Import() = %#v", local)
	}
	items, err := List(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Reference != local.Reference ||
		items[0].SHA256 == "" || len(items[0].Bases) != 1 {
		t.Fatalf("List() = %#v", items)
	}
	ref, _ := Parse(local.Reference)
	resolved, err := Resolve(nil, home, ref, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Bases) != 1 || resolved.Bases[0] != "minimax-h3" {
		t.Fatalf("Resolve() = %#v", resolved)
	}
}

func TestImportRejectsSymlinkAndNonSafeTensors(t *testing.T) {
	home := t.TempDir()
	plain := filepath.Join(t.TempDir(), "bad.safetensors")
	if err := os.WriteFile(plain, []byte("not safetensors"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(home, plain, "bad", "minimax-h3", false); err == nil {
		t.Fatal("expected invalid safetensors to be rejected")
	}
	valid := filepath.Join(t.TempDir(), "valid.safetensors")
	if err := os.WriteFile(valid, safeTensorBytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link.safetensors")
	if err := os.Symlink(valid, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(home, link, "link", "minimax-h3", false); err == nil {
		t.Fatal("expected symlink to be rejected")
	}
}

func TestImportForceReplacesManagedFile(t *testing.T) {
	home := t.TempDir()
	first := filepath.Join(t.TempDir(), "motion.safetensors")
	second := filepath.Join(t.TempDir(), "motion.safetensors")
	firstPayload := safeTensorBytes()
	secondPayload := append(safeTensorBytes(), byte(7))
	if err := os.WriteFile(first, firstPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, secondPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(home, first, "motion", "minimax-h3", false); err != nil {
		t.Fatal(err)
	}
	local, err := Import(home, second, "motion", "minimax-h3", true)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(local.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, secondPayload) {
		t.Fatal("--force did not replace the managed adapter")
	}
}

func TestPullVerifiesChecksumAndFormat(t *testing.T) {
	home := t.TempDir()
	payload := safeTensorBytes()
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "huggingface.co" {
			t.Fatalf("unexpected download URL %s", request.URL)
		}
		result := responseBytes(payload)
		result.ContentLength = int64(len(payload))
		return result, nil
	})}
	ref, _ := Parse("hf://owner/repo#adapter.safetensors")
	local, err := localReference(home, ref, ref.File, 1)
	if err != nil {
		t.Fatal(err)
	}
	local.SHA256 = hash
	if err := Pull(client, local, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(local.Path); err != nil {
		t.Fatal(err)
	}
}

func response(body string) *http.Response {
	return responseBytes([]byte(body))
}

func responseBytes(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

func safeTensorBytes() []byte {
	header := []byte(`{"__metadata__":{"format":"test"}}`)
	result := make([]byte, 8+len(header))
	binary.LittleEndian.PutUint64(result[:8], uint64(len(header)))
	copy(result[8:], header)
	return result
}
