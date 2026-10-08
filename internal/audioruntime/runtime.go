package audioruntime

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/carlos/tapioca/internal/pythonruntime"
)

//go:embed stable_audio3.py requirements.txt
var source embed.FS

type Request struct {
	ModelPath      string
	Prompt         string
	NegativePrompt string
	Output         string
	Duration       int
	Steps          int
	Seed           uint64
	Backend        string
}

func Run(ctx context.Context, cacheDir string, request Request) error {
	return RunWithWriters(ctx, cacheDir, request, os.Stdout, os.Stderr)
}

func RunWithWriters(
	ctx context.Context,
	cacheDir string,
	request Request,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if request.Backend != "stable-audio3" {
		return fmt.Errorf("unsupported audio backend %q", request.Backend)
	}
	if request.Duration <= 0 || request.Duration > 120 {
		return errors.New("Stable Audio 3 duration must be between 1 and 120 seconds")
	}
	root := filepath.Join(cacheDir, "audio-runtime", "0.1.0-stable-audio3")
	for _, name := range []string{"stable_audio3.py", "requirements.txt"} {
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
		system, prefix, err := pythonruntime.Find("Stable Audio 3 generation")
		if err != nil {
			return err
		}
		fmt.Fprintln(stderr, "creating the Stable Audio 3 runtime (first run only)...")
		cmd := exec.CommandContext(ctx, system, append(prefix, "-m", "venv", venv)...)
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create Stable Audio 3 environment: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "--upgrade", "pip")
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("upgrade pip: %w", err)
		}
		if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, python, "-m", "pip", "install",
				"torch==2.7.1", "torchaudio==2.7.1",
				"--index-url", "https://download.pytorch.org/whl/cu128")
			cmd.Stdout, cmd.Stderr = stderr, stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install CUDA-enabled Stable Audio 3 PyTorch: %w", err)
			}
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", filepath.Join(root, "requirements.txt"))
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install Stable Audio 3 dependencies: %w", err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}

	cmd := exec.CommandContext(ctx, python, pythonArguments(root, request)...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Stable Audio 3 generation failed: %w", err)
	}
	return nil
}

func pythonArguments(root string, request Request) []string {
	args := []string{
		filepath.Join(root, "stable_audio3.py"),
		"--model", request.ModelPath,
		"--prompt", request.Prompt,
		"--output", request.Output,
		"--duration", fmt.Sprint(request.Duration),
		"--steps", fmt.Sprint(request.Steps),
		"--seed", fmt.Sprint(request.Seed),
	}
	if request.NegativePrompt != "" {
		args = append(args, "--negative-prompt", request.NegativePrompt)
	}
	return args
}
