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
)

//go:embed Package.swift Package.resolved Sources/tapioca-image-runtime/main.swift diffusers.py requirements.txt requirements-mflux.txt
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
	Backend        string
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
	case "diffusers":
		return runDiffusers(ctx, cacheDir, request)
	case "mflux":
		return runMFlux(ctx, cacheDir, request)
	default:
		return fmt.Errorf("unsupported image backend %q", request.Backend)
	}
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
		system, prefix, err := systemPython()
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "creating the MFLUX image runtime (first run only)...")
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
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", requirements)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install MFLUX: %w", err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}
	command := filepath.Join(venv, "bin", "mflux-generate-flux2")
	args := []string{
		"--model", request.ModelPath, "--prompt", request.Prompt,
		"--width", fmt.Sprint(request.Width), "--height", fmt.Sprint(request.Height),
		"--steps", fmt.Sprint(request.Steps), "--seed", fmt.Sprint(request.Seed),
		"--output", request.Output,
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("MFLUX image generation failed: %w", err)
	}
	return nil
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
		fmt.Fprintln(os.Stderr, "building the MLX image runtime (first run only)...")
		cmd := exec.CommandContext(ctx, "swift", "build", "-c", "release", "--package-path", root)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
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
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func buildMLXMetallib(ctx context.Context, root, binaryDir string) error {
	metallib := filepath.Join(binaryDir, "mlx.metallib")
	if _, err := os.Stat(metallib); err == nil {
		return nil
	}

	fmt.Fprintln(os.Stderr, "building MLX Metal shaders (first run only)...")
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
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("compile MLX Metal shader %s: %w", shader, err)
		}
		airFiles = append(airFiles, airPath)
	}

	args := append([]string{"-sdk", "macosx", "metallib"}, airFiles...)
	args = append(args, "-o", metallib)
	cmd := exec.CommandContext(ctx, "xcrun", args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
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
	if runtime.GOOS == "darwin" {
		return errors.New("the Diffusers backend in this release targets NVIDIA CUDA; use qwen-image-flash:int8 on Apple Silicon")
	}
	if runtime.GOOS == "windows" && runtime.GOARCH != "amd64" {
		return errors.New("the CUDA image backend currently requires Windows x64")
	}
	root := filepath.Join(cacheDir, "diffusers-runtime", "0.1.0")
	for _, name := range []string{"diffusers.py", "requirements.txt"} {
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
		name, prefix, err := systemPython()
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "creating the CUDA image runtime (first run only)...")
		venvArgs := append(prefix, "-m", "venv", venv)
		cmd := exec.CommandContext(ctx, name, venvArgs...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("create Python environment: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "--upgrade", "pip")
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("upgrade pip: %w", err)
		}
		cmd = exec.CommandContext(
			ctx, python, "-m", "pip", "install",
			"torch>=2.7", "--index-url", "https://download.pytorch.org/whl/cu128",
		)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install CUDA-enabled PyTorch: %w", err)
		}
		cmd = exec.CommandContext(ctx, python, "-m", "pip", "install", "-r", filepath.Join(root, "requirements.txt"))
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install CUDA image dependencies: %w", err)
		}
		if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
			return err
		}
	}
	args := []string{
		filepath.Join(root, "diffusers.py"),
		"--model", request.ModelPath, "--prompt", request.Prompt,
		"--output", request.Output, "--width", fmt.Sprint(request.Width),
		"--height", fmt.Sprint(request.Height), "--steps", fmt.Sprint(request.Steps),
		"--seed", fmt.Sprint(request.Seed),
	}
	if request.NegativePrompt != "" {
		args = append(args, "--negative-prompt", request.NegativePrompt)
	}
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Diffusers image generation failed: %w", err)
	}
	return nil
}

func systemPython() (string, []string, error) {
	candidates := []struct {
		name   string
		prefix []string
	}{
		{"python3", nil},
		{"python", nil},
		{"py", []string{"-3"}},
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate.name); err == nil {
			return path, candidate.prefix, nil
		}
	}
	return "", nil, errors.New("Python 3.10 or newer is required for this image backend")
}
