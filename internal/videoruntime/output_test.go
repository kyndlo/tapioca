package videoruntime

import (
	"bytes"
	"context"
	"testing"
)

func TestWriterContextDefaultsAndOverrides(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	ctx := context.WithValue(context.Background(), outputContextKey{}, runtimeOutput{
		stdout: &stdout, stderr: &stderr,
	})
	_, _ = runtimeStdout(ctx).Write([]byte("out"))
	_, _ = runtimeStderr(ctx).Write([]byte("err"))
	if stdout.String() != "out" || stderr.String() != "err" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
