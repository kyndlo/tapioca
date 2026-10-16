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
	"github.com/carlos/tapioca/internal/config"
)

func TestVisionPairPullResumeOfflineAndCorruption(t *testing.T) {
	t.Setenv("TAPIOCA_HOME", t.TempDir())
	old := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = old })
	revision := strings.Repeat("a", 40)
	metadata := func(data string) catalog.Download {
		return catalog.Download{Revision: revision, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(data))), SizeBytes: int64(len(data))}
	}
	language, projector := "language fixture", "projector fixture"
	resolved := catalog.Resolved{Name: "vision:test", Repo: "owner/model", Filename: "model.gguf", Backend: "llama-vision", Download: metadata(language), Projector: &catalog.Artifact{Repo: "owner/model", Filename: "projector.gguf", Target: "projector.gguf", Download: metadata(projector)}}
	requests := 0
	interrupted := true
	http.DefaultClient = &http.Client{Transport: artifactTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if !strings.Contains(r.URL.Path, "/resolve/"+revision+"/") {
			t.Errorf("unpinned URL: %s", r.URL)
		}
		data := language
		status := 200
		if strings.HasSuffix(r.URL.Path, "projector.gguf") {
			if interrupted {
				interrupted = false
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &interruptedBody{}, ContentLength: int64(len(projector))}, nil
			}
			data = projector
			if value := r.Header.Get("Range"); value != "" {
				if value != "bytes=4-" {
					t.Errorf("incorrect resume range %s", value)
				}
				status = 206
				data = data[4:]
			}
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(data)), ContentLength: int64(len(data))}, nil
	})}
	if _, err := pullResolvedWithContext(context.Background(), resolved, false, nil); err == nil {
		t.Fatal("interrupted projector pull succeeded")
	}
	registry, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Models) != 0 {
		t.Fatal("registered incomplete pair")
	}
	model, err := pullResolvedWithContext(context.Background(), resolved, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if model.ProjectorPath == "" || model.ProjectorIntegrity != resolved.Projector.Download {
		t.Fatalf("projector metadata not persisted: %#v", model)
	}
	registry, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if registry.Models[model.Name].ProjectorPath != model.ProjectorPath {
		t.Fatal("lost projector in registry")
	}
	if requests != 3 {
		t.Fatalf("resume redownloaded a verified language model: %d requests", requests)
	}
	before := requests
	http.DefaultClient = &http.Client{Transport: artifactTransport(func(*http.Request) (*http.Response, error) {
		t.Error("offline reuse contacted network")
		return nil, fmt.Errorf("offline")
	})}
	if _, err := pullResolvedWithContext(context.Background(), resolved, false, nil); err != nil {
		t.Fatal(err)
	}
	if requests != before {
		t.Fatal("offline reuse downloaded")
	}
	if err := os.WriteFile(model.ProjectorPath, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := pullResolvedWithContext(context.Background(), resolved, false, nil); err == nil {
		t.Fatal("accepted corrupt projector")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(model.Path), "projector.gguf.partial")); !os.IsNotExist(err) {
		t.Fatal("successful resume left partial")
	}
}

type interruptedBody struct{ read bool }

func (b *interruptedBody) Read(p []byte) (int, error) {
	if b.read {
		return 0, io.ErrUnexpectedEOF
	}
	b.read = true
	return copy(p, "proj"), nil
}
func (*interruptedBody) Close() error { return nil }

func TestInstalledVisionContextSurvivesCatalogRefresh(t *testing.T) {
	t.Setenv("TAPIOCA_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stored := range []int{0, 4096} {
		model := config.Model{Name: "installed-vision:test", Path: path, Backend: "llama-vision", Context: stored}
		if err := (config.Registry{Models: map[string]config.Model{model.Name: model}}).Save(); err != nil {
			t.Fatal(err)
		}
		opts, _, err := parseServe([]string{model.Name})
		if err != nil {
			t.Fatal(err)
		}
		if opts.context != 4096 {
			t.Fatalf("installed vision got text default %d", opts.context)
		}
		opts, _, err = parseServe([]string{model.Name, "--context", "8192"})
		if err != nil {
			t.Fatal(err)
		}
		if opts.context != 8192 {
			t.Fatal("ignored explicit context")
		}
	}
}
