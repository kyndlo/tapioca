package control

import (
	"context"
	"testing"
)

func TestCancellationRegistry(t *testing.T) {
	registry := NewCancellationRegistry()
	ctx, err := registry.Register(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if !registry.Contains("job-1") {
		t.Fatal("registry does not contain registered job")
	}
	if _, duplicate := registry.Register(context.Background(), "job-1"); duplicate == nil ||
		duplicate.Code != "job_conflict" {
		t.Fatalf("duplicate Register() error = %#v", duplicate)
	}
	if !registry.Cancel("job-1") {
		t.Fatal("Cancel() = false, want true")
	}
	<-ctx.Done()
	if ctx.Err() != context.Canceled {
		t.Fatalf("context error = %v, want context.Canceled", ctx.Err())
	}
	registry.Complete("job-1")
	if registry.Contains("job-1") {
		t.Fatal("registry still contains completed job")
	}
	if registry.Cancel("missing") {
		t.Fatal("Cancel(missing) = true, want false")
	}
}
