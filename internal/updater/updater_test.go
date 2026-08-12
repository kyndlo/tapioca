package updater

import (
	"archive/zip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCheckSelectsCurrentPlatformAsset(t *testing.T) {
	name, err := cliAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releaseResponse{
			TagName: "v9.2.1", URL: "https://example.test/release",
			Assets: []Asset{{Name: name, URL: "https://example.test/archive"}, {Name: name + ".sha256", URL: "https://example.test/checksum"}},
		})
	}))
	defer server.Close()
	t.Setenv("TAPIOCA_RELEASE_API", server.URL)
	info, err := Check(context.Background(), "0.8.0")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "9.2.1" || info.Asset.Name != name {
		t.Fatalf("unexpected update: %#v", info)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escape")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("bad"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(archivePath, t.TempDir()); err == nil {
		t.Fatal("zip traversal was accepted")
	}
}

func TestReplaceInstallMovesRuntimeAndBinaryTogether(t *testing.T) {
	root, staging := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(staging, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tapioca"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "tapioca"), []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceInstall(staging, root, []string{"runtime", "tapioca"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "tapioca"))
	if err != nil || string(data) != "new" {
		t.Fatalf("installed binary = %q, %v", data, err)
	}
}

func TestCheckReportsCurrentVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releaseResponse{TagName: "v0.8.0"})
	}))
	defer server.Close()
	t.Setenv("TAPIOCA_RELEASE_API", server.URL)
	info, err := Check(context.Background(), "0.8.0")
	if err != nil {
		t.Fatal(err)
	}
	if info.Available {
		t.Fatalf("unexpected update: %#v", info)
	}
}

func TestSafeTargetRejectsTraversal(t *testing.T) {
	if _, err := safeTarget(t.TempDir(), "../escape"); err == nil {
		t.Fatal("archive traversal was accepted")
	}
}

func TestWindowsInstallScriptQuotesPaths(t *testing.T) {
	script := windowsInstallScript(`C:\Program Files\Tapioca\tapioca.exe`, `C:\stage dir`)
	if len(script) == 0 || script[0] != '@' {
		t.Fatalf("unexpected script: %q", script)
	}
}
