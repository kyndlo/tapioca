package videoruntime

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

//go:embed mlx_video.py diffusers_video.py requirements-mlx.txt requirements-diffusers.txt
var source embed.FS

type Request struct {
	ModelPath      string
	Prompt         string
	NegativePrompt string
	InputImage     string
	Output         string
	Width          int
	Height         int
	Frames         int
	Steps          int
	FPS            int
	Seed           uint64
	Backend        string
}

func Run(ctx context.Context, cacheDir string, request Request) error {
	switch request.Backend {
	case "mlx-video":
		if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
			return errors.New("the MLX video backend requires macOS on Apple Silicon")
		}
		return runPython(ctx, cacheDir, request, "mlx")
	case "diffusers-video":
		if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
			return errors.New("the CUDA video backend currently requires Windows x64 with an NVIDIA GPU")
		}
		return runPython(ctx, cacheDir, request, "diffusers")
	default:
		return fmt.Errorf("unsupported video backend %q", request.Backend)
	}
}

func runPython(ctx context.Context, cacheDir string, request Request, flavor string) error {
	root := filepath.Join(cacheDir, "video-runtime", "0.1.1-"+flavor)
	script := flavor + "_video.py"
	requirements := "requirements-" + flavor + ".txt"
	for _, name := range []string{script, requirements} {
		data, err := source.ReadFile(name)
		if err != nil {
			return err
		}
		target := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}

	venv := filepath.Join(root, "venv")
	python := filepath.Join(venv, "bin", "python")
	if runtime.GOOS == "windows" {
		python = filepath.Join(venv, "Scripts", "python.exe")
	}
	ready := filepath.Join(venv, ".tapioca-ready")
	if _, err := os.Stat(ready); err != nil {
		system, prefix, err := systemPython()
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "creating the video runtime (first run only)...")
		cmd := exec.CommandContext(ctx, system, append(prefix, "-m", "venv", venv)...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create Python environment: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "--upgrade", "pip")
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("upgrade pip: %w", err)
		}
		if flavor == "diffusers" {
			cmd = exec.CommandContext(ctx, python, "-m", "pip", "install",
				"torch>=2.7", "--index-url", "https://download.pytorch.org/whl/cu128")
			cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install CUDA-enabled PyTorch: %w", err)
			}
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", filepath.Join(root, requirements))
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install video dependencies: %w", err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}

	args := []string{
		filepath.Join(root, script), "--model", request.ModelPath,
		"--prompt", request.Prompt, "--output", request.Output,
		"--width", fmt.Sprint(request.Width), "--height", fmt.Sprint(request.Height),
		"--frames", fmt.Sprint(request.Frames), "--steps", fmt.Sprint(request.Steps),
		"--fps", fmt.Sprint(request.FPS), "--seed", fmt.Sprint(request.Seed),
	}
	if request.NegativePrompt != "" {
		args = append(args, "--negative-prompt", request.NegativePrompt)
	}
	if request.InputImage != "" {
		args = append(args, "--image", request.InputImage)
	}
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s video generation failed: %w", flavor, err)
	}
	return nil
}

func systemPython() (string, []string, error) {
	for _, candidate := range []struct {
		name   string
		prefix []string
	}{{"python3", nil}, {"python", nil}, {"py", []string{"-3"}}} {
		if path, err := exec.LookPath(candidate.name); err == nil {
			return path, candidate.prefix, nil
		}
	}
	return "", nil, errors.New("Python 3.10 or newer is required for video generation")
}
