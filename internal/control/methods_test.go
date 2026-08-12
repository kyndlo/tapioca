package control

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/carlos/tapioca/internal/catalog"
)

func TestHandlerDispatchesReadOnlyMethods(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(Dependencies{
		Now: func() time.Time { return now },
		Catalog: func(context.Context) ([]CatalogModel, error) {
			return []CatalogModel{{Name: "gemma3:12b-mlx", Kind: "text"}}, nil
		},
		Installed: func(context.Context) ([]InstalledModel, error) {
			return []InstalledModel{{Name: "qwen3:8b-q4_k_m", Kind: "text"}}, nil
		},
	})

	tests := []struct {
		method string
		check  func(t *testing.T, result any)
	}{
		{
			method: "handshake",
			check: func(t *testing.T, result any) {
				handshake := result.(map[string]any)
				if handshake["name"] != "tapioca-control" ||
					handshake["protocol_version"] != ProtocolVersion {
					t.Fatalf("handshake result = %#v", handshake)
				}
			},
		},
		{
			method: "capabilities.get",
			check: func(t *testing.T, result any) {
				value := result.(map[string]any)
				methods := value["methods"].([]string)
				if len(methods) == 0 || methods[0] != "handshake" {
					t.Fatalf("capabilities result = %#v", value)
				}
			},
		},
		{
			method: "health.get",
			check: func(t *testing.T, result any) {
				health := result.(map[string]any)
				if health["status"] != "ok" || health["time"] != "2026-08-01T12:00:00Z" {
					t.Fatalf("health result = %#v", health)
				}
			},
		},
		{
			method: "catalog.list",
			check: func(t *testing.T, result any) {
				models := result.([]CatalogModel)
				if len(models) != 1 || models[0].Name != "gemma3:12b-mlx" {
					t.Fatalf("catalog result = %#v", models)
				}
			},
		},
		{
			method: "installed.list",
			check: func(t *testing.T, result any) {
				models := result.([]InstalledModel)
				if len(models) != 1 || models[0].Name != "qwen3:8b-q4_k_m" {
					t.Fatalf("installed result = %#v", models)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			result, err := handler.Handle(context.Background(), Request{
				Version: ProtocolVersion,
				Type:    "request",
				ID:      "req-1",
				Method:  test.method,
			})
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			test.check(t, result)
		})
	}
}

func TestHealthReportsMeasuredBuildAndUptimeFacts(t *testing.T) {
	started := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	current := started
	handler := NewHandler(Dependencies{Now: func() time.Time { return current }})
	current = started.Add(2500 * time.Millisecond)
	result, err := handler.Handle(context.Background(), Request{Method: "health.get"})
	if err != nil {
		t.Fatalf("health.get error = %v", err)
	}
	health := result.(map[string]any)
	if health["name"] != "tapioca-control" ||
		health["control_version"] != ControlVersion ||
		health["uptime_ms"] != int64(2500) ||
		health["started_at"] != "2026-08-01T12:00:00Z" ||
		health["go_version"] == "" || health["module_version"] == "" {
		t.Fatalf("health result = %#v", health)
	}
}

func TestHandlerReturnsStructuredMethodAndDependencyErrors(t *testing.T) {
	handler := NewHandler(Dependencies{
		Catalog: func(context.Context) ([]CatalogModel, error) {
			return nil, errors.New("catalog unavailable")
		},
	})

	_, err := handler.Handle(context.Background(), Request{Method: "missing.method"})
	if err == nil || err.Code != "method_not_found" {
		t.Fatalf("unknown method error = %#v", err)
	}

	_, err = handler.Handle(context.Background(), Request{Method: "catalog.list"})
	if err == nil || err.Code != "internal_error" || err.Message != "catalog unavailable" {
		t.Fatalf("catalog error = %#v", err)
	}
}

func TestHandlerRefreshesCatalog(t *testing.T) {
	handler := NewHandler(Dependencies{
		RefreshCatalog: func(context.Context) (catalog.RefreshResult, error) {
			return catalog.RefreshResult{Path: "/catalog.json", Models: 12, SHA256: strings.Repeat("a", 64)}, nil
		},
	})
	result, err := handler.Handle(context.Background(), Request{Method: "catalog.refresh"})
	if err != nil {
		t.Fatal(err)
	}
	refresh := result.(catalog.RefreshResult)
	if refresh.Models != 12 || refresh.Path != "/catalog.json" {
		t.Fatalf("result = %#v", refresh)
	}
}

func TestHandlerRejectsUnknownParams(t *testing.T) {
	handler := NewHandler(Dependencies{})
	_, err := handler.Handle(context.Background(), Request{
		Method: "health.get",
		Params: []byte(`{"unexpected":true}`),
	})
	if err == nil || err.Code != "invalid_request" {
		t.Fatalf("unknown params error = %#v", err)
	}

	_, err = handler.Handle(context.Background(), Request{
		Method: "job.cancel",
		Params: []byte(`{"job_id":"job-1","unexpected":true}`),
	})
	if err == nil || err.Code != "invalid_request" {
		t.Fatalf("unknown cancel params error = %#v", err)
	}
}

func TestHandlerCancelsRunningJob(t *testing.T) {
	started := make(chan struct{})
	handler := NewHandler(Dependencies{
		Catalog: func(ctx context.Context) ([]CatalogModel, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})

	result := make(chan *ProtocolError, 1)
	go func() {
		_, err := handler.Handle(context.Background(), Request{
			ID: "request-work", JobID: "job-work", Method: "catalog.list",
		})
		result <- err
	}()
	<-started

	cancelled, err := handler.Handle(context.Background(), Request{
		ID:     "request-cancel",
		Method: "job.cancel",
		Params: []byte(`{"job_id":"job-work"}`),
	})
	if err != nil {
		t.Fatalf("job.cancel error = %v", err)
	}
	if cancelled.(map[string]any)["cancelled"] != true {
		t.Fatalf("job.cancel result = %#v", cancelled)
	}
	if workError := <-result; workError == nil || workError.Code != "job_cancelled" {
		t.Fatalf("running job error = %#v", workError)
	}
}
