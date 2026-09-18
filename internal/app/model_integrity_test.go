package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/carlos/tapioca/internal/catalog"
)

func TestVerifyModelArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.gguf")
	data := []byte("small test artifact")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	model := catalog.Resolved{ByteSize: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
	if err := verifyModelArtifact(path, model); err != nil {
		t.Fatalf("valid artifact: %v", err)
	}
	model.ByteSize++
	if err := verifyModelArtifact(path, model); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("expected size rejection, got %v", err)
	}
	model.ByteSize--
	model.SHA256 = strings.Repeat("0", 64)
	if err := verifyModelArtifact(path, model); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("expected checksum rejection, got %v", err)
	}
}

func TestCompactModelsDefaultToBoundedContext(t *testing.T) {
	for _, ref := range []string{"granite-4.2-3b", "minicpm5-2b"} {
		if got := defaultContextSize(ref); got != 8192 {
			t.Fatalf("%s context = %d, want 8192", ref, got)
		}
	}
	if got := defaultContextSize("glm-4.7-flash"); got != 65536 {
		t.Fatalf("existing default context = %d, want 65536", got)
	}
}

func TestPullPinnedArtifactAndOfflineCache(t *testing.T) {
	t.Setenv("TAPIOCA_HOME", t.TempDir())
	data := []byte("small pinned model")
	digest := sha256.Sum256(data)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(data)
	}))
	defer server.Close()
	model := catalog.Resolved{
		Name: "pinned-test:q4", Repo: "test/repo", Filename: "test.gguf",
		URL: server.URL, SHA256: hex.EncodeToString(digest[:]), ByteSize: int64(len(data)),
	}
	got, err := pullResolvedWithContext(context.Background(), model, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path == "" || requests.Load() != 1 {
		t.Fatalf("fresh download: path=%q requests=%d", got.Path, requests.Load())
	}
	server.Close()
	if _, err := pullResolvedWithContext(context.Background(), model, false, nil); err != nil {
		t.Fatalf("offline cache reuse: %v", err)
	}
}

func TestPullRejectsCorruptPinnedArtifact(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("wrong bytes"))
	}))
	defer server.Close()
	model := catalog.Resolved{
		Name: "pinned-test:q4", Repo: "test/repo", Filename: "test.gguf",
		URL: server.URL, SHA256: strings.Repeat("0", 64), ByteSize: int64(len("wrong bytes")),
	}
	if _, err := pullResolvedWithContext(context.Background(), model, false, nil); err == nil ||
		!strings.Contains(err.Error(), "integrity verification") {
		t.Fatalf("expected integrity error, got %v", err)
	}
	path := filepath.Join(home, "models", "pinned-test-q4", "test.gguf")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("corrupt download was installed: %v", err)
	}
	if _, err := os.Stat(path + ".partial"); !os.IsNotExist(err) {
		t.Fatalf("corrupt partial was retained: %v", err)
	}
}
