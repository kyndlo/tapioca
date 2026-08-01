package speechruntime

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

//go:embed speech.py requirements-chatterbox.txt requirements-qwen.txt requirements-mlx.txt
var source embed.FS

type Request struct {
	ModelPath   string
	ModelName   string
	Text        string
	Output      string
	VoiceSample string
	Transcript  string
	Language    string
	Backend     string
}

func Run(ctx context.Context, cacheDir string, request Request) error {
	flavor := ""
	switch request.Backend {
	case "speech-chatterbox":
		flavor = "chatterbox"
	case "speech-qwen":
		if runtime.GOOS == "darwin" {
			return errors.New("the PyTorch Qwen TTS backend is not supported on macOS; use qwen3-tts:0.6b-mlx")
		}
		flavor = "qwen"
	case "speech-qwen-mlx":
		if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
			return errors.New("the MLX speech backend requires macOS on Apple Silicon")
		}
		flavor = "mlx"
	default:
		return fmt.Errorf("unsupported speech backend %q", request.Backend)
	}
	return runPython(ctx, cacheDir, request, flavor)
}

func runPython(ctx context.Context, cacheDir string, request Request, flavor string) error {
	root := filepath.Join(cacheDir, "speech-runtime", "0.1.0-"+flavor)
	requirementsName := "requirements-" + flavor + ".txt"
	for _, name := range []string{"speech.py", requirementsName} {
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
		fmt.Fprintf(os.Stderr, "creating the %s speech runtime (first run only)...\n", flavor)
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
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", filepath.Join(root, requirementsName))
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install %s speech dependencies: %w", flavor, err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}

	cmd := exec.CommandContext(ctx, python, pythonArguments(root, request)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s speech generation failed: %w", flavor, err)
	}
	return nil
}

func pythonArguments(root string, request Request) []string {
	args := []string{
		filepath.Join(root, "speech.py"),
		"--model", request.ModelPath,
		"--model-name", request.ModelName,
		"--backend", request.Backend,
		"--text", request.Text,
		"--output", request.Output,
	}
	if request.VoiceSample != "" {
		args = append(args, "--voice-sample", request.VoiceSample)
	}
	if request.Transcript != "" {
		args = append(args, "--transcript", request.Transcript)
	}
	if request.Language != "" {
		args = append(args, "--language", request.Language)
	}
	return args
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
	return "", nil, errors.New("Python 3.10 or newer is required for speech generation")
}
