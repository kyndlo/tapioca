package videoruntime

import (
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/carlos/tapioca/internal/adapter"
)

//go:embed mlx_video.py diffusers_video.py h3_video.py requirements-mlx.txt requirements-diffusers.txt
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
	Adapters       []adapter.Local
}

func Run(ctx context.Context, cacheDir string, request Request) error {
	switch request.Backend {
	case "mlx-video":
		if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
			return errors.New("the MLX video backend requires macOS on Apple Silicon")
		}
	case "diffusers-video":
		if (runtime.GOOS != "windows" && runtime.GOOS != "linux") || runtime.GOARCH != "amd64" {
			return errors.New("the CUDA video backend requires Windows or Linux x64 with an NVIDIA GPU")
		}
	case "comfy-h3-mps":
		if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
			return errors.New("the MiniMax-H3 MPS backend requires macOS on Apple Silicon")
		}
	case "comfy-h3-cuda":
		if (runtime.GOOS != "windows" && runtime.GOOS != "linux") || runtime.GOARCH != "amd64" {
			return errors.New("the MiniMax-H3 CUDA backend requires Windows or Linux x64 with an NVIDIA GPU")
		}
	}
	engine, err := engineFor(request.Backend)
	if err != nil {
		return err
	}
	return engine.Run(ctx, cacheDir, request)
}

const (
	comfyTag      = "v0.30.0"
	h3WorkflowURL = "https://raw.githubusercontent.com/Bambushu/minimax-h3-mac/6959ac3d986909183e2a0bc9c06c1a13e2746ebf/h3_api.json"
	h3WorkflowSHA = "26291d8f7ac3aaecca9738f066925169f5f31524eebd7c5bfcfee6ac658322a9"
)

func runH3(ctx context.Context, cacheDir string, request Request) error {
	root := filepath.Join(cacheDir, "video-runtime", "0.2.0-h3")
	comfy := filepath.Join(root, "ComfyUI")
	venv := filepath.Join(root, "venv")
	python := venvPython(venv)
	if err := ensureH3Runtime(ctx, root, comfy, venv, python, request.Backend); err != nil {
		return err
	}
	loraNames, err := stageH3Adapters(root, request.Adapters)
	if err != nil {
		return err
	}
	extraPaths := fmt.Sprintf(
		"tapioca:\n  base_path: %s\n  diffusion_models: diffusion_models\n  text_encoders: text_encoders\n  vae: vae\n  loras: %s\n",
		yamlQuote(request.ModelPath), yamlQuote(filepath.Join(root, "loras")),
	)
	if err := os.WriteFile(filepath.Join(root, "extra_model_paths.yaml"), []byte(extraPaths), 0o644); err != nil {
		return err
	}
	port, err := freePort()
	if err != nil {
		return err
	}
	logPath := filepath.Join(root, "comfyui.log")
	log, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer log.Close()
	args := []string{
		filepath.Join(comfy, "main.py"), "--listen", "127.0.0.1", "--port", fmt.Sprint(port),
		"--extra-model-paths-config", filepath.Join(root, "extra_model_paths.yaml"),
		"--disable-auto-launch",
	}
	if request.Backend == "comfy-h3-mps" {
		args = append(args, "--reserve-vram", "10", "--cache-none", "--disable-smart-memory")
	} else {
		args = append(args, "--lowvram", "--fast", "fp16_accumulation")
	}
	server := exec.CommandContext(ctx, python, args...)
	server.Dir = comfy
	server.Stdout, server.Stderr = log, log
	server.Env = append(os.Environ(), "ASFP8_INT8_EXT=1")
	if err := server.Start(); err != nil {
		return fmt.Errorf("start managed ComfyUI: %w", err)
	}
	defer func() {
		if server.Process != nil {
			_ = server.Process.Kill()
		}
		_ = server.Wait()
	}()
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForServer(ctx, base, 3*time.Minute); err != nil {
		return fmt.Errorf("%w; see %s", err, logPath)
	}
	scriptData, err := source.ReadFile("h3_video.py")
	if err != nil {
		return err
	}
	script := filepath.Join(root, "h3_video.py")
	if err := os.WriteFile(script, scriptData, 0o644); err != nil {
		return err
	}
	workflow := filepath.Join(root, "h3_api.json")
	cmdArgs := []string{
		script, "--server", base, "--workflow", workflow, "--comfy-root", comfy,
		"--backend", request.Backend, "--model", request.ModelPath,
		"--prompt", request.Prompt, "--output", request.Output,
		"--width", fmt.Sprint(request.Width), "--height", fmt.Sprint(request.Height),
		"--frames", fmt.Sprint(request.Frames), "--steps", fmt.Sprint(request.Steps),
		"--fps", fmt.Sprint(request.FPS), "--seed", fmt.Sprint(request.Seed),
	}
	for index, item := range request.Adapters {
		cmdArgs = append(cmdArgs, "--adapter", loraNames[index], "--adapter-scale", fmt.Sprint(item.Scale))
	}
	if request.InputImage != "" {
		cmdArgs = append(cmdArgs, "--image", request.InputImage)
	}
	cmd := exec.CommandContext(ctx, python, cmdArgs...)
	cmd.Stdout, cmd.Stderr = runtimeStdout(ctx), runtimeStderr(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("MiniMax-H3 generation failed: %w; see %s", err, logPath)
	}
	return nil
}

func yamlQuote(value string) string {
	return strconv.Quote(value)
}

// stageH3Adapters gives the private ComfyUI process a narrow, deterministic
// view of adapters selected by Tapioca. It avoids exposing the whole adapter
// store and keeps ComfyUI paths out of the public CLI contract.
func stageH3Adapters(root string, adapters []adapter.Local) ([]string, error) {
	directory := filepath.Join(root, "loras")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(adapters))
	for _, item := range adapters {
		info, err := os.Lstat(item.Path)
		if err != nil {
			return nil, fmt.Errorf("inspect LoRA %s: %w", item.Reference, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("LoRA %s is not a regular file", item.Reference)
		}
		digest := sha256.Sum256([]byte(item.Path))
		name := fmt.Sprintf("%x-%s", digest[:6], filepath.Base(item.Path))
		target := filepath.Join(directory, name)
		if targetInfo, statErr := os.Stat(target); statErr == nil && targetInfo.Size() == info.Size() {
			names = append(names, name)
			continue
		}
		_ = os.Remove(target)
		if err := os.Link(item.Path, target); err != nil {
			if err := copyFile(item.Path, target); err != nil {
				return nil, fmt.Errorf("stage LoRA %s: %w", item.Reference, err)
			}
		}
		names = append(names, name)
	}
	return names, nil
}

func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(targetFile, sourceFile)
	closeErr := targetFile.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func ensureH3Runtime(
	ctx context.Context,
	root, comfy, venv, python, backend string,
) error {
	ready := filepath.Join(root, ".tapioca-"+backend+"-ready")
	if _, err := os.Stat(ready); err == nil {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("Git is required for the first MiniMax-H3 runtime setup")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	stderr := runtimeStderr(ctx)
	run := func(dir string, name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s %v: %w", name, args, err)
		}
		return nil
	}
	if _, err := os.Stat(filepath.Join(comfy, "main.py")); err != nil {
		fmt.Fprintln(stderr, "installing the managed MiniMax-H3 runtime (first run only)...")
		if err := run(root, "git", "clone", "--depth", "1", "--branch", comfyTag,
			"https://github.com/comfyanonymous/ComfyUI.git", comfy); err != nil {
			return err
		}
	}
	if _, err := os.Stat(python); err != nil {
		system, prefix, err := systemPython()
		if err != nil {
			return err
		}
		if err := run(root, system, append(prefix, "-m", "venv", venv)...); err != nil {
			return err
		}
	}
	if err := run(root, python, "-m", "pip", "install", "--upgrade", "pip"); err != nil {
		return err
	}
	if backend == "comfy-h3-cuda" {
		if err := run(root, python, "-m", "pip", "install", "torch>=2.7",
			"--index-url", "https://download.pytorch.org/whl/cu128"); err != nil {
			return err
		}
	}
	if err := run(root, python, "-m", "pip", "install", "-r",
		filepath.Join(comfy, "requirements.txt")); err != nil {
		return err
	}
	if backend == "comfy-h3-mps" {
		nodes := filepath.Join(comfy, "custom_nodes")
		packs := []struct {
			name   string
			url    string
			commit string
		}{
			{"ComfyUI-GGUF", "https://github.com/city96/ComfyUI-GGUF.git", "6ea2651e7df66d7585f6ffee804b20e92fb38b8a"},
			{"ComfyUI-AppleSilicon-FP8", "https://github.com/pawel-mazurkiewicz/ComfyUI-AppleSilicon-FP8.git", "3cc65dd8d8b98f4ab69cf48b8912a831dc8ccff3"},
		}
		for _, pack := range packs {
			target := filepath.Join(nodes, pack.name)
			if _, err := os.Stat(target); err != nil {
				if err := run(nodes, "git", "clone", "--depth", "1", pack.url, target); err != nil {
					return err
				}
			}
			if err := run(target, "git", "fetch", "--depth", "1", "origin", pack.commit); err != nil {
				return err
			}
			if err := run(target, "git", "checkout", "--detach", pack.commit); err != nil {
				return err
			}
			requirements := filepath.Join(target, "requirements.txt")
			if _, err := os.Stat(requirements); err == nil {
				if err := run(root, python, "-m", "pip", "install", "-r", requirements); err != nil {
					return err
				}
			}
		}
		if err := run(root, python, "-m", "pip", "install", "gguf==0.18.0"); err != nil {
			return err
		}
	}
	workflow := filepath.Join(root, "h3_api.json")
	if err := downloadVerified(ctx, h3WorkflowURL, workflow, h3WorkflowSHA); err != nil {
		return err
	}
	return os.WriteFile(ready, []byte("ready\n"), 0o644)
}

func downloadVerified(ctx context.Context, url, destination, wantSHA string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("download runtime asset: %s", response.Status)
	}
	partial := destination + ".partial"
	file, err := os.Create(partial)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(file, hash), response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != wantSHA {
		return fmt.Errorf("runtime asset checksum mismatch: got %s", got)
	}
	return os.Rename(partial, destination)
}

func venvPython(venv string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venv, "Scripts", "python.exe")
	}
	return filepath.Join(venv, "bin", "python")
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForServer(ctx context.Context, base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/system_stats", nil)
		if err == nil {
			response, doErr := http.DefaultClient.Do(request)
			if doErr == nil {
				response.Body.Close()
				if response.StatusCode < 300 {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return errors.New("managed ComfyUI did not become ready")
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
		fmt.Fprintln(runtimeStderr(ctx), "creating the video runtime (first run only)...")
		cmd := exec.CommandContext(ctx, system, append(prefix, "-m", "venv", venv)...)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create Python environment: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "--upgrade", "pip")
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("upgrade pip: %w", err)
		}
		if flavor == "diffusers" {
			cmd = exec.CommandContext(ctx, python, "-m", "pip", "install",
				"torch>=2.7", "--index-url", "https://download.pytorch.org/whl/cu128")
			cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install CUDA-enabled PyTorch: %w", err)
			}
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", filepath.Join(root, requirements))
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install video dependencies: %w", err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}

	args := pythonArguments(root, script, request)
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Stdout, cmd.Stderr = runtimeStdout(ctx), runtimeStderr(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s video generation failed: %w", flavor, err)
	}
	return nil
}

func pythonArguments(root, script string, request Request) []string {
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
	for _, item := range request.Adapters {
		args = append(args, "--adapter", item.Path, "--adapter-scale", fmt.Sprint(item.Scale))
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
	return "", nil, errors.New("Python 3.10 or newer is required for video generation")
}
