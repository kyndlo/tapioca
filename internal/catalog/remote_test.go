package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefreshVerifiesAndLoadsRemoteCatalog(t *testing.T) {
	manifest := Manifest{
		Schema: manifestSchemaVersion, GeneratedAt: time.Now().UTC(),
		Models: map[string]Model{
			"remote-test": {
				Name: "remote-test", Repo: "owner/model", Kind: "image",
				Default: "bf16", Files: map[string]string{"bf16": ""},
				Backends: map[string]string{"bf16": "diffusers"},
				Sizes:    map[string]string{"bf16": "~1 GiB"},
			},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/catalog.json.sha256" {
			fmt.Fprint(w, hex.EncodeToString(digest[:])+"  catalog.json\n")
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "catalog.json")
	restore := SetOverridePathForTest(path)
	defer restore()
	t.Setenv("TAPIOCA_CATALOG_URL", server.URL+"/catalog.json")
	t.Setenv("TAPIOCA_CATALOG_CHECKSUM_URL", server.URL+"/catalog.json.sha256")
	result, err := Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Models != 1 || result.Path != path {
		t.Fatalf("unexpected result: %#v", result)
	}
	resolved, err := ResolveForPlatform("remote-test", "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Repo != "owner/model" || resolved.Backend != "diffusers" {
		t.Fatalf("unexpected remote model: %#v", resolved)
	}
}

func TestInvalidCachedCatalogFallsBackToBuiltIns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(`{"schema":99}`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := SetOverridePathForTest(path)
	defer restore()
	if _, err := Resolve("glm-4.7-flash:q4_k_m"); err != nil {
		t.Fatalf("built-in catalog was unavailable: %v", err)
	}
}

func TestRemoteCatalogRejectsUnsafePaths(t *testing.T) {
	manifest := Manifest{Schema: manifestSchemaVersion, GeneratedAt: time.Now(), Models: map[string]Model{
		"unsafe": {Name: "unsafe", Repo: "owner/model", Default: "bad", Files: map[string]string{"bad": "../escape.gguf"}, Sizes: map[string]string{"bad": "1"}},
	}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeManifest(data); err == nil {
		t.Fatal("unsafe manifest was accepted")
	}
}

func TestRemoteCatalogRejectsUnknownPlatformDefault(t *testing.T) {
	manifest := Manifest{Schema: manifestSchemaVersion, GeneratedAt: time.Now(), Models: map[string]Model{
		"unsafe": {Name: "unsafe", Repo: "owner/model", Default: "good", Files: map[string]string{"good": "model.gguf"}, Sizes: map[string]string{"good": "1"}, PlatformDefaults: map[string]string{"windows/amd64": "missing"}},
	}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeManifest(data); err == nil {
		t.Fatal("unknown platform default was accepted")
	}
}
