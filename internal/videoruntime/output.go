package videoruntime

import (
	"context"
	"io"
	"os"
)

type outputContextKey struct{}

type runtimeOutput struct {
	stdout io.Writer
	stderr io.Writer
}

func RunWithWriters(
	ctx context.Context,
	cacheDir string,
	request Request,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	ctx = context.WithValue(ctx, outputContextKey{}, runtimeOutput{
		stdout: stdout,
		stderr: stderr,
	})
	return Run(ctx, cacheDir, request)
}

func runtimeStdout(ctx context.Context) io.Writer {
	if output, ok := ctx.Value(outputContextKey{}).(runtimeOutput); ok {
		return output.stdout
	}
	return os.Stdout
}

func runtimeStderr(ctx context.Context) io.Writer {
	if output, ok := ctx.Value(outputContextKey{}).(runtimeOutput); ok {
		return output.stderr
	}
	return os.Stderr
}
