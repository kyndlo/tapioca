package speechruntime

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

//go:embed speech.py cpu_speech.py pocket_qualification.py arktts_runtime requirements-*.txt
var source embed.FS

type Request struct {
	ModelPath    string
	ModelName    string
	Text         string
	Output       string
	VoiceSample  string
	Transcript   string
	Language     string
	Backend      string
	VoiceConsent bool
	Seed         uint64
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
	flavor := ""
	switch request.Backend {
	case "speech-audio8-onnx", "speech-pocket-tts":
		if runtime.GOARCH != "amd64" && !(runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") {
			return errors.New("this CPU speech backend requires x64 Windows/Linux or Apple Silicon macOS")
		}
		if request.VoiceSample != "" && !request.VoiceConsent {
			return errors.New("voice cloning requires explicit permission")
		}
		flavor = "audio8"
		if request.Backend == "speech-pocket-tts" {
			flavor = "pocket-qualification"
		}
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
	return runPython(ctx, cacheDir, request, flavor, stdout, stderr)
}

func runPython(
	ctx context.Context,
	cacheDir string,
	request Request,
	flavor string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	root := filepath.Join(cacheDir, "speech-runtime", "0.1.3-"+flavor)
	requirementsName := "requirements-" + flavor + ".txt"
	names := []string{"speech.py", requirementsName, "cpu_speech.py", "pocket_qualification.py"}
	if flavor == "audio8" {
		entries, err := source.ReadDir("arktts_runtime")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				names = append(names, "arktts_runtime/"+entry.Name())
			}
		}
	}
	for _, name := range names {
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
		system, prefix, err := pythonruntime.Find("speech generation")
		if err != nil {
			return err
		}
		fmt.Fprintf(stderr, "creating the %s speech runtime (first run only)...\n", flavor)
		cmd := exec.CommandContext(ctx, system, append(prefix, "-m", "venv", venv)...)
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create Python environment: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "--upgrade", "pip")
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("upgrade pip: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", filepath.Join(root, requirementsName))
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install %s speech dependencies: %w", flavor, err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}

	cmd := exec.CommandContext(ctx, python, pythonArguments(root, request)...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
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
	if request.VoiceConsent {
		args = append(args, "--voice-consent")
	}
	args = append(args, "--seed", fmt.Sprint(request.Seed))
	return args
}
