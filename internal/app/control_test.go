package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadWithContextReportsStructuredProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tapioca"))
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "model.partial")
	var reports []PullProgress
	if err := downloadWithContext(
		context.Background(),
		server.URL,
		destination,
		func(progress PullProgress) { reports = append(reports, progress) },
	); err != nil {
		t.Fatalf("downloadWithContext() error = %v", err)
	}
	if len(reports) == 0 {
		t.Fatal("download emitted no structured progress")
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "tapioca" {
		t.Fatalf("downloaded contents = %q", contents)
	}
}

func TestDownloadWithContextHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := downloadWithContext(ctx, server.URL, filepath.Join(t.TempDir(), "partial"), nil)
	if err == nil {
		t.Fatal("downloadWithContext() succeeded with cancelled context")
	}
}
