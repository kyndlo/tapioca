package imageruntime

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/carlos/tapioca/internal/adapter"
	"github.com/carlos/tapioca/internal/pythonruntime"
)

//go:embed Package.swift Package.resolved Sources/tapioca-image-runtime/main.swift image_diffusion.py onnx_diffusion.py requirements.txt requirements-mflux.txt requirements-onnx.txt
var source embed.FS

type Request struct {
	ModelPath      string
	Prompt         string
	NegativePrompt string
	Output         string
	Width          int
	Height         int
	Steps          int
	Seed           uint64
	GuidanceScale  *float64
	Backend        string
	InputImages    []string
	Adapters       []adapter.Local
}

func Run(ctx context.Context, cacheDir string, request Request) error {
	if request.Backend == "" {
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			request.Backend = "mlx"
		} else {
			request.Backend = "diffusers"
		}
	}
	switch request.Backend {
	case "mlx":
		return runMLX(ctx, cacheDir, request)
	case "diffusers", "diffusers-mps":
		return runDiffusers(ctx, cacheDir, request)
	case "mflux":
		return runMFlux(ctx, cacheDir, request)
	case "onnx-directml", "onnx-cpu":
		return runONNX(ctx, cacheDir, request)
	default:
		return fmt.Errorf("unsupported image backend %q", request.Backend)
	}
}

func runONNX(ctx context.Context, cacheDir string, request Request) error {
	if runtime.GOOS != "windows" {
		return errors.New("the ONNX diffusion backends currently require Windows")
	}
	if request.Backend == "onnx-directml" && runtime.GOARCH != "amd64" {
		return errors.New("DirectML requires Windows x64; use sd-turbo:onnx-arm64 on Windows ARM64")
	}
	if request.Backend == "onnx-cpu" && runtime.GOARCH != "arm64" {
		return errors.New("the curated ONNX CPU backend is intended for Windows ARM64")
	}
	root := filepath.Join(cacheDir, "onnx-image-runtime", "0.1.0-"+request.Backend)
	for _, name := range []string{"onnx_diffusion.py", "requirements-onnx.txt"} {
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
	python := filepath.Join(venv, "Scripts", "python.exe")
	ready := filepath.Join(venv, ".tapioca-ready")
	if _, err := os.Stat(ready); err != nil {
		system, prefix, err := pythonruntime.Find("ONNX image generation")
		if err != nil {
			return err
		}
		fmt.Fprintf(runtimeStderr(ctx), "creating the %s image runtime (first run only)...\n", request.Backend)
		cmd := exec.CommandContext(ctx, system, append(prefix, "-m", "venv", venv)...)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create Python environment: %w", err)
		}
		for _, args := range [][]string{
			{"-m", "pip", "install", "--upgrade", "pip"},
			{"-m", "pip", "install", "-r", filepath.Join(root, "requirements-onnx.txt")},
		} {
			cmd = exec.CommandContext(ctx, python, args...)
			cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install ONNX image dependencies: %w", err)
			}
		}
		providerPackage := "onnxruntime"
		if request.Backend == "onnx-directml" {
			providerPackage = "onnxruntime-directml"
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", providerPackage)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install %s: %w", providerPackage, err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}
	args := onnxArguments(root, request)
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Stdout, cmd.Stderr = runtimeStdout(ctx), runtimeStderr(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ONNX image generation failed: %w", err)
	}
	return nil
}

func onnxArguments(root string, request Request) []string {
	provider := "CPUExecutionProvider"
	if request.Backend == "onnx-directml" {
		provider = "DmlExecutionProvider"
	}
	args := []string{
		filepath.Join(root, "onnx_diffusion.py"),
		"--model", request.ModelPath, "--prompt", request.Prompt,
		"--output", request.Output, "--width", fmt.Sprint(request.Width),
		"--height", fmt.Sprint(request.Height), "--steps", fmt.Sprint(request.Steps),
		"--seed", fmt.Sprint(request.Seed), "--provider", provider,
	}
	if request.NegativePrompt != "" {
		args = append(args, "--negative-prompt", request.NegativePrompt)
	}
	return args
}

func runMFlux(ctx context.Context, cacheDir string, request Request) error {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return errors.New("the MFLUX image backend requires macOS on Apple Silicon")
	}
	root := filepath.Join(cacheDir, "mflux-runtime", "0.1.0")
	data, err := source.ReadFile("requirements-mflux.txt")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	requirements := filepath.Join(root, "requirements.txt")
	if err := os.WriteFile(requirements, data, 0o644); err != nil {
		return err
	}
	venv := filepath.Join(root, "venv")
	python := filepath.Join(venv, "bin", "python")
	ready := filepath.Join(venv, ".tapioca-ready")
	if _, err := os.Stat(ready); err != nil {
		system, prefix, err := pythonruntime.Find("MFLUX image generation")
		if err != nil {
			return err
		}
		fmt.Fprintln(runtimeStderr(ctx), "creating the MFLUX image runtime (first run only)...")
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
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", requirements)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install MFLUX: %w", err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}
	commandName, args := mfluxArguments(request)
	command := filepath.Join(venv, "bin", commandName)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout, cmd.Stderr = runtimeStdout(ctx), runtimeStderr(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("MFLUX image generation failed: %w", err)
	}
	return nil
}

func mfluxArguments(request Request) (string, []string) {
	commandName := "mflux-generate-flux2"
	if len(request.InputImages) > 0 {
		commandName = "mflux-generate-flux2-edit"
	}
	args := []string{
		"--model", request.ModelPath, "--prompt", request.Prompt,
		"--width", fmt.Sprint(request.Width), "--height", fmt.Sprint(request.Height),
		"--steps", fmt.Sprint(request.Steps), "--seed", fmt.Sprint(request.Seed),
		"--output", request.Output,
	}
	if len(request.InputImages) > 0 {
		args = append(args, "--image-paths")
		args = append(args, request.InputImages...)
	}
	if len(request.Adapters) > 0 {
		args = append(args, "--lora-paths")
		for _, item := range request.Adapters {
			args = append(args, item.Path)
		}
		args = append(args, "--lora-scales")
		for _, item := range request.Adapters {
			args = append(args, fmt.Sprint(item.Scale))
		}
	}
	return commandName, args
}

func runMLX(ctx context.Context, cacheDir string, request Request) error {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return fmt.Errorf("the MLX image backend requires macOS on Apple Silicon; pull the bf16 variant for CUDA")
	}
	if binary := bundledMLXRuntime(); binary != "" {
		return runMLXBinary(ctx, binary, request)
	}
	if _, err := exec.LookPath("swift"); err != nil {
		return fmt.Errorf("this development build does not include the MLX image runtime; Swift 6.2 or newer is required: %w", err)
	}
	if err := requireMetalToolchain(ctx); err != nil {
		return err
	}
	root := filepath.Join(cacheDir, "image-runtime", "0.1.1")
	for _, name := range []string{"Package.swift", "Package.resolved", "Sources/tapioca-image-runtime/main.swift"} {
		data, err := source.ReadFile(name)
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	binary := filepath.Join(root, ".build", "release", "tapioca-image-runtime")
	if _, err := os.Stat(binary); err != nil {
		fmt.Fprintln(runtimeStderr(ctx), "building the MLX image runtime (first run only)...")
		cmd := exec.CommandContext(ctx, "swift", "build", "-c", "release", "--package-path", root)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build MLX image runtime: %w", err)
		}
	}
	if err := buildMLXMetallib(ctx, root, filepath.Dir(binary)); err != nil {
		return err
	}
	return runMLXBinary(ctx, binary, request)
}

func bundledMLXRuntime() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	root := filepath.Join(filepath.Dir(executable), "runtime", "image")
	binary := filepath.Join(root, "tapioca-image-runtime")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	for _, path := range []string{binary, filepath.Join(root, "mlx.metallib")} {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return ""
		}
	}
	return binary
}

func runMLXBinary(ctx context.Context, binary string, request Request) error {
	args := []string{
		"--model", request.ModelPath, "--prompt", request.Prompt,
		"--output", request.Output, "--width", fmt.Sprint(request.Width),
		"--height", fmt.Sprint(request.Height), "--steps", fmt.Sprint(request.Steps),
		"--seed", fmt.Sprint(request.Seed),
	}
	if request.NegativePrompt != "" {
		args = append(args, "--negative-prompt", request.NegativePrompt)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout, cmd.Stderr = runtimeStdout(ctx), runtimeStderr(ctx)
	return cmd.Run()
}

func buildMLXMetallib(ctx context.Context, root, binaryDir string) error {
	metallib := filepath.Join(binaryDir, "mlx.metallib")
	if _, err := os.Stat(metallib); err == nil {
		return nil
	}

	fmt.Fprintln(runtimeStderr(ctx), "building MLX Metal shaders (first run only)...")
	cmlxRoot := filepath.Join(root, ".build", "checkouts", "mlx-swift", "Source", "Cmlx")
	shaderRoot := filepath.Join(cmlxRoot, "mlx-generated", "metal")
	airRoot := filepath.Join(root, ".build", "tapioca-metal-air")
	if err := os.MkdirAll(airRoot, 0o755); err != nil {
		return fmt.Errorf("create MLX shader build directory: %w", err)
	}

	shaders := []string{
		"arg_reduce.metal",
		"conv.metal",
		"gemv.metal",
		"layer_norm.metal",
		"random.metal",
		"rms_norm.metal",
		"rope.metal",
		"scaled_dot_product_attention.metal",
		filepath.Join("steel", "attn", "kernels", "steel_attention.metal"),
	}
	airFiles := make([]string, 0, len(shaders))
	for index, shader := range shaders {
		sourcePath := filepath.Join(shaderRoot, shader)
		airPath := filepath.Join(airRoot, fmt.Sprintf("%02d-%s.air", index, filepath.Base(shader[:len(shader)-len(filepath.Ext(shader))])))
		args := []string{
			"-sdk", "macosx", "metal", "-x", "metal",
			"-Wall", "-Wextra", "-fno-fast-math",
			"-Wno-c++17-extensions", "-Wno-c++20-extensions",
			"-mmacosx-version-min=14.0", "-c", sourcePath,
			"-I" + cmlxRoot, "-o", airPath,
		}
		cmd := exec.CommandContext(ctx, "xcrun", args...)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("compile MLX Metal shader %s: %w", shader, err)
		}
		airFiles = append(airFiles, airPath)
	}

	args := append([]string{"-sdk", "macosx", "metallib"}, airFiles...)
	args = append(args, "-o", metallib)
	cmd := exec.CommandContext(ctx, "xcrun", args...)
	cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("link MLX Metal shader library: %w", err)
	}
	return nil
}

func requireMetalToolchain(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "xcrun", "-sdk", "macosx", "--find", "metallib")
	if err := cmd.Run(); err != nil {
		return errors.New(
			"the Xcode Metal Toolchain is required for MLX image generation; install it with `xcodebuild -downloadComponent MetalToolchain`",
		)
	}
	return nil
}

func runDiffusers(ctx context.Context, cacheDir string, request Request) error {
	if request.Backend == "diffusers-mps" &&
		(runtime.GOOS != "darwin" || runtime.GOARCH != "arm64") {
		return errors.New("the MPS Diffusers backend requires macOS on Apple Silicon")
	}
	if request.Backend == "diffusers" && runtime.GOOS == "darwin" {
		return errors.New("this Diffusers profile targets NVIDIA CUDA; select a model variant with MPS support")
	}
	if request.Backend == "diffusers" && runtime.GOOS == "windows" && runtime.GOARCH != "amd64" {
		return errors.New("the CUDA image backend currently requires Windows x64")
	}
	if request.Backend == "diffusers" && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return errors.New("the CUDA image backend requires Windows or Linux")
	}
	root := filepath.Join(cacheDir, "diffusers-runtime", "0.2.0")
	for _, name := range []string{"image_diffusion.py", "requirements.txt"} {
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
		name, prefix, err := pythonruntime.Find("Diffusers image generation")
		if err != nil {
			return err
		}
		acceleration := "CUDA"
		if request.Backend == "diffusers-mps" {
			acceleration = "Apple MPS"
		}
		fmt.Fprintf(runtimeStderr(ctx), "creating the %s image runtime (first run only)...\n", acceleration)
		venvArgs := append(prefix, "-m", "venv", venv)
		cmd := exec.CommandContext(ctx, name, venvArgs...)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create Python environment: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "--upgrade", "pip")
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("upgrade pip: %w", err)
		}
		torchArgs := []string{"-m", "pip", "install", "torch>=2.7"}
		if request.Backend == "diffusers" {
			torchArgs = append(torchArgs, "--index-url", "https://download.pytorch.org/whl/cu128")
		}
		cmd = exec.CommandContext(ctx, python, torchArgs...)
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install %s PyTorch runtime: %w", acceleration, err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", filepath.Join(root, "requirements.txt"))
		cmd.Stdout, cmd.Stderr = runtimeStderr(ctx), runtimeStderr(ctx)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install %s image dependencies: %w", acceleration, err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}
	args := diffusersArguments(root, request)
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Stdout, cmd.Stderr = runtimeStdout(ctx), runtimeStderr(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Diffusers image generation failed: %w", err)
	}
	return nil
}

func diffusersArguments(root string, request Request) []string {
	args := []string{
		"-P", filepath.Join(root, "image_diffusion.py"),
		"--model", request.ModelPath, "--prompt", request.Prompt,
		"--output", request.Output, "--width", fmt.Sprint(request.Width),
		"--height", fmt.Sprint(request.Height), "--steps", fmt.Sprint(request.Steps),
		"--seed", fmt.Sprint(request.Seed), "--backend", request.Backend,
	}
	if request.GuidanceScale != nil {
		args = append(args, "--guidance-scale", fmt.Sprint(*request.GuidanceScale))
	}
	if request.NegativePrompt != "" {
		args = append(args, "--negative-prompt", request.NegativePrompt)
	}
	for _, image := range request.InputImages {
		args = append(args, "--image", image)
	}
	for _, item := range request.Adapters {
		args = append(args, "--adapter", item.Path, "--adapter-scale", fmt.Sprint(item.Scale))
	}
	return args
}
