package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos/tapioca/internal/catalog"
)

type artifactTransport func(*http.Request) (*http.Response, error)

func (f artifactTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestPinnedArtifactDownloadAndCache(t *testing.T) {
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	data := "verified bundle component"
	expected := catalog.Download{Revision: strings.Repeat("a", 40), SizeBytes: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}
	model := catalog.Resolved{Name: "bundle:test", Artifacts: []catalog.Artifact{{Repo: "owner/model", Filename: "component.bin", Target: "nested/component.bin", Download: expected}}}
	count := 0
	http.DefaultClient = &http.Client{Transport: artifactTransport(func(request *http.Request) (*http.Response, error) {
		count++
		if request.URL.String() != "https://huggingface.co/owner/model/resolve/"+expected.Revision+"/component.bin" {
			t.Errorf("unpinned URL: %s", request.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(data)), ContentLength: int64(len(data))}, nil
	})}
	root := t.TempDir()
	if err := pullArtifactsWithContext(context.Background(), model, root, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := pullArtifactsWithContext(context.Background(), model, root, false, nil); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("cached pull made %d requests", count)
	}
	path := filepath.Join(root, "nested/component.bin")
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pullArtifactsWithContext(context.Background(), model, root, false, nil); err == nil {
		t.Fatal("reused corrupt cached artifact")
	}
	if err := pullArtifactsWithContext(context.Background(), model, root, true, nil); err != nil {
		t.Fatal(err)
	}
	data = "invalid upstream body"
	if err := pullArtifactsWithContext(context.Background(), model, root, true, nil); err == nil {
		t.Fatal("accepted corrupt download")
	}
	if _, err := os.Stat(path + ".partial"); !os.IsNotExist(err) {
		t.Fatal("corrupt partial remains resumable")
	}
	if err := verifyArtifact(path, expected); err != nil {
		t.Fatalf("failed download replaced good cached file: %v", err)
	}
}

func TestVerifyArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.gguf")
	data := []byte("test artifact")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	valid := catalog.Download{SizeBytes: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	for _, tc := range []struct {
		name     string
		metadata catalog.Download
		fail     bool
	}{
		{"exact", valid, false},
		{"legacy", catalog.Download{}, false},
		{"truncated", catalog.Download{SizeBytes: 100}, true},
		{"corrupted", catalog.Download{SHA256: fmt.Sprintf("%064d", 0)}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifyArtifact(path, tc.metadata); (err != nil) != tc.fail {
				t.Fatalf("verifyArtifact = %v", err)
			}
		})
	}
}
