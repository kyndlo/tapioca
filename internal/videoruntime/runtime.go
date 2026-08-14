package videoruntime

import (
	"archive/zip"
	"bufio"
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
	"strings"
	"time"

	"github.com/carlos/tapioca/internal/adapter"
	"github.com/carlos/tapioca/internal/pythonruntime"
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

// MaxVideoFrames bounds a single generation across CLI and control surfaces.
const MaxVideoFrames = 513

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
	comfyTag        = "v0.30.0"
	comfyCommit     = "b1693ecba9f5b65f8c80ab36b195ab963ec92413"
	comfyArchiveURL = "https://github.com/comfyanonymous/ComfyUI/archive/" + comfyCommit + ".zip"
	comfyArchiveSHA = "912365439272d6c7cd9428f897b23be747e27a6a56dfa7f3d3132e72fb564699"
	h3WorkflowURL   = "https://raw.githubusercontent.com/Bambushu/minimax-h3-mac/6959ac3d986909183e2a0bc9c06c1a13e2746ebf/h3_api.json"
	h3WorkflowSHA   = "26291d8f7ac3aaecca9738f066925169f5f31524eebd7c5bfcfee6ac658322a9"
	uvWindowsURL    = "https://github.com/astral-sh/uv/releases/download/0.12.3/uv-x86_64-pc-windows-msvc.zip"
	uvWindowsSHA    = "b23350c79e8ad0192b8124af13a0f17e8d4e4549524785e1aef389ae5a06990e"
)

func runH3(ctx context.Context, cacheDir string, request Request) error {
	var cudaGPU *nvidiaGPU
	if request.Backend == "comfy-h3-cuda" {
		gpu, err := inspectNVIDIA(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(runtimeStderr(ctx), "using %s (%d MiB VRAM, driver %s)\n", gpu.Name, gpu.MemoryMiB, gpu.Driver)
		if gpu.MemoryMiB < 16*1024 {
			fmt.Fprintf(runtimeStderr(ctx), "warning: MiniMax-H3 is tested with 16 GiB VRAM; lower resolutions may work, but generation can be slow or run out of memory\n")
		}
		cudaGPU = &gpu
	}
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
	args := h3ServerArgs(comfy, filepath.Join(root, "extra_model_paths.yaml"), port, request.Backend)
	server := exec.CommandContext(ctx, python, args...)
	server.Dir = comfy
	server.Stdout, server.Stderr = log, log
	server.Env = append(os.Environ(), "ASFP8_INT8_EXT=1")
	if cudaGPU != nil {
		server.Env = append(server.Env, "CUDA_VISIBLE_DEVICES="+strconv.Itoa(cudaGPU.Index))
	}
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

type nvidiaGPU struct {
	Index     int
	Name      string
	MemoryMiB int
	Driver    string
}

func inspectNVIDIA(ctx context.Context) (nvidiaGPU, error) {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nvidiaGPU{}, errors.New("MiniMax-H3 requires an NVIDIA GPU and current NVIDIA driver; nvidia-smi was not found")
	}
	cmd := exec.CommandContext(ctx, path, "--query-gpu=index,name,memory.total,driver_version", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return nvidiaGPU{}, fmt.Errorf("inspect NVIDIA GPU with nvidia-smi: %w", err)
	}
	return parseNVIDIASMI(string(output))
}

func parseNVIDIASMI(output string) (nvidiaGPU, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var selected nvidiaGPU
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			return nvidiaGPU{}, fmt.Errorf("unexpected nvidia-smi output %q", line)
		}
		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nvidiaGPU{}, fmt.Errorf("parse NVIDIA GPU index from %q: %w", parts[0], err)
		}
		memory, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return nvidiaGPU{}, fmt.Errorf("parse NVIDIA VRAM from %q: %w", parts[2], err)
		}
		if memory > selected.MemoryMiB {
			selected = nvidiaGPU{
				Index: index, Name: strings.TrimSpace(parts[1]), MemoryMiB: memory, Driver: strings.TrimSpace(parts[3]),
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nvidiaGPU{}, err
	}
	if selected.MemoryMiB == 0 {
		return nvidiaGPU{}, errors.New("nvidia-smi reported no NVIDIA GPUs")
	}
	return selected, nil
}

func h3ServerArgs(comfy, extraPaths string, port int, backend string) []string {
	args := []string{
		filepath.Join(comfy, "main.py"), "--listen", "127.0.0.1", "--port", fmt.Sprint(port),
		"--extra-model-paths-config", extraPaths, "--disable-auto-launch",
	}
	if backend == "comfy-h3-mps" {
		return append(args, "--reserve-vram", "10", "--cache-none", "--disable-smart-memory")
	}
	// Aggressively unload the 20 GiB transformer when the 4.9 GiB VAE requests
	// memory. Unlike --lowvram, this still lets the VAE stay resident during
	// decode instead of swapping its layers for every frame.
	return append(args, "--reserve-vram", "1", "--disable-smart-memory", "--fast", "fp16_accumulation")
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
		if backend != "comfy-h3-cuda" || verifyCUDA(ctx, python) == nil {
			return nil
		}
		fmt.Fprintln(runtimeStderr(ctx), "repairing the managed MiniMax-H3 CUDA runtime...")
		_ = os.Remove(ready)
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
		if err := installComfyArchive(ctx, root, comfy); err != nil {
			return err
		}
	}
	if _, err := os.Stat(python); err != nil {
		if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
			if err := installManagedWindowsPython(ctx, root, venv); err != nil {
				return err
			}
		} else {
			system, prefix, err := systemPython()
			if err != nil {
				return err
			}
			if err := run(root, system, append(prefix, "-m", "venv", venv)...); err != nil {
				return err
			}
		}
	}
	for _, command := range h3DependencyCommands(python, filepath.Join(comfy, "requirements.txt"), backend) {
		if err := run(root, command.name, command.args...); err != nil {
			return err
		}
	}
	if backend == "comfy-h3-cuda" {
		if err := verifyCUDA(ctx, python); err != nil {
			return fmt.Errorf("MiniMax-H3 CUDA runtime validation failed: %w", err)
		}
	}
	if backend == "comfy-h3-mps" {
		if _, err := exec.LookPath("git"); err != nil {
			return errors.New("Git is required for the first MiniMax-H3 runtime setup on macOS")
		}
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

func installComfyArchive(ctx context.Context, root, destination string) error {
	archive := filepath.Join(root, "ComfyUI-"+strings.TrimPrefix(comfyTag, "v")+".zip")
	if err := downloadVerified(ctx, comfyArchiveURL, archive, comfyArchiveSHA); err != nil {
		return fmt.Errorf("download managed ComfyUI %s: %w", comfyTag, err)
	}
	extracted := filepath.Join(root, "ComfyUI-"+comfyCommit)
	if err := extractZip(archive, root); err != nil {
		return fmt.Errorf("extract managed ComfyUI %s: %w", comfyTag, err)
	}
	if _, err := os.Stat(filepath.Join(extracted, "main.py")); err != nil {
		return fmt.Errorf("managed ComfyUI archive did not contain main.py: %w", err)
	}
	if _, err := os.Stat(destination); err == nil {
		backup := destination + ".incomplete-" + strconv.FormatInt(time.Now().Unix(), 10)
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("preserve incomplete managed ComfyUI directory: %w", err)
		}
	}
	if err := os.Rename(extracted, destination); err != nil {
		return fmt.Errorf("activate managed ComfyUI %s: %w", comfyTag, err)
	}
	return nil
}

func installManagedWindowsPython(ctx context.Context, root, venv string) error {
	toolsDir := filepath.Join(root, "tools")
	uv := filepath.Join(toolsDir, "uv.exe")
	if _, err := os.Stat(uv); err != nil {
		if err := os.MkdirAll(toolsDir, 0o755); err != nil {
			return err
		}
		archive := filepath.Join(toolsDir, "uv-windows-amd64.zip")
		if err := downloadVerified(ctx, uvWindowsURL, archive, uvWindowsSHA); err != nil {
			return fmt.Errorf("download managed Python bootstrap: %w", err)
		}
		if err := extractZip(archive, toolsDir); err != nil {
			return fmt.Errorf("extract managed Python bootstrap: %w", err)
		}
	}
	pythonDir := filepath.Join(root, "python")
	cmd := exec.CommandContext(ctx, uv, "venv", "--python", "3.12", "--managed-python", "--seed", venv)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "UV_PYTHON_INSTALL_DIR="+pythonDir, "UV_NO_CONFIG=1")
	cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create Tapioca-managed Python 3.12 environment: %w", err)
	}
	return nil
}

func extractZip(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	cleanRoot := filepath.Clean(destination) + string(os.PathSeparator)
	for _, entry := range reader.File {
		target := filepath.Join(destination, filepath.FromSlash(entry.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanRoot) {
			return fmt.Errorf("archive contains unsafe path %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if entry.Mode()&os.ModeType != 0 {
			return fmt.Errorf("archive contains unsupported file type %q", entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		sourceFile, err := entry.Open()
		if err != nil {
			return err
		}
		targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.Mode().Perm())
		if err != nil {
			sourceFile.Close()
			return err
		}
		_, copyErr := io.Copy(targetFile, sourceFile)
		closeTargetErr := targetFile.Close()
		closeSourceErr := sourceFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeTargetErr != nil {
			return closeTargetErr
		}
		if closeSourceErr != nil {
			return closeSourceErr
		}
	}
	return nil
}

type runtimeCommand struct {
	name string
	args []string
}

func h3DependencyCommands(python, requirements, backend string) []runtimeCommand {
	commands := []runtimeCommand{
		{name: python, args: []string{"-m", "pip", "install", "--upgrade", "pip"}},
		{name: python, args: []string{"-m", "pip", "install", "-r", requirements}},
	}
	if backend == "comfy-h3-cuda" {
		// ComfyUI's unconstrained requirements can replace a CUDA wheel with the
		// CPU-only PyPI build. Install the matched CUDA trio last and without
		// dependencies so the generic requirements cannot overwrite it.
		commands = append(commands, runtimeCommand{name: python, args: []string{
			"-m", "pip", "install", "--force-reinstall", "--no-deps",
			"torch==2.11.0+cu128", "torchvision==0.26.0+cu128", "torchaudio==2.11.0+cu128",
			"--index-url", "https://download.pytorch.org/whl/cu128",
		}})
	}
	return commands
}

func verifyCUDA(ctx context.Context, python string) error {
	cmd := exec.CommandContext(ctx, python, "-c",
		"import torch; assert torch.cuda.is_available(), 'PyTorch was installed without CUDA support'; print(torch.__version__, torch.cuda.get_device_name(0))")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(output), err)
	}
	return nil
}

func downloadVerified(ctx context.Context, url, destination, wantSHA string) error {
	if got, err := fileSHA256(destination); err == nil && got == wantSHA {
		return nil
	}
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
	if _, err := os.Stat(destination); err == nil {
		backup := destination + ".invalid-" + strconv.FormatInt(time.Now().Unix(), 10)
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("preserve invalid runtime asset: %w", err)
		}
	}
	return os.Rename(partial, destination)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
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
	return pythonruntime.Find("video generation")
}
