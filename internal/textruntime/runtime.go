package textruntime

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

//go:embed requirements.txt
var source embed.FS

func Command(
	ctx context.Context,
	cacheDir string,
	modelPath string,
	host string,
	port int,
	contextSize int,
) (*exec.Cmd, error) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return nil, errors.New("the MLX text backend requires macOS on Apple Silicon")
	}
	root := filepath.Join(cacheDir, "mlx-vlm", "cb2ca446")
	requirements, err := source.ReadFile("requirements.txt")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	requirementsPath := filepath.Join(root, "requirements.txt")
	if err := os.WriteFile(requirementsPath, requirements, 0o644); err != nil {
		return nil, err
	}
	venv := filepath.Join(root, "venv")
	python := filepath.Join(venv, "bin", "python")
	ready := filepath.Join(venv, ".tapioca-ready")
	if _, err := os.Stat(ready); err != nil {
		systemPython, err := findPython()
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(os.Stderr, "Setting up the MLX text runtime (first run only)...")
		cmd := exec.CommandContext(ctx, systemPython, "-m", "venv", venv)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("create MLX Python environment: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "--upgrade", "pip")
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("upgrade pip: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", requirementsPath)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("install mlx-vlm: %w", err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return nil, err
		}
	}
	args := []string{
		"-m", "mlx_vlm.server",
		"--model", modelPath,
		"--host", host,
		"--port", fmt.Sprint(port),
		"--max-kv-size", fmt.Sprint(contextSize),
		"--enable-thinking",
		"--log-progress-interval", "0",
	}
	return exec.CommandContext(ctx, python, args...), nil
}

func findPython() (string, error) {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("Python 3.10 or newer is required for the MLX text backend")
}
