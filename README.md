# Tapioca

Tapioca is a small local-model runtime for language and diffusion models. It
downloads GGUF models, starts `llama-server` with Metal acceleration, and
presents compatible APIs to coding agents. On Apple Silicon it can also run
native MLX image models.

## Requirements

- Go 1.25 or newer (to build)
- The official macOS and Windows bundles include `llama-server`; source builds
  can use a separately installed llama.cpp (`brew install llama.cpp` on macOS).
- Official macOS Apple Silicon bundles include the compiled MLX image runtime
  and Metal shaders. Building that runtime from source requires Xcode 26 /
  Swift 6.2 or newer and the Xcode Metal Toolchain. Install the Metal component
  once with `xcodebuild -downloadComponent MetalToolchain`.
- Windows x64 plus Python 3.10+, a current NVIDIA driver, and an
  Ampere-or-newer NVIDIA GPU for CUDA image generation
- The client CLI you want to launch (`codex`, `claude`, `opencode`,
  `openclaw`, or `hermes`)

## Release bundles

[GitHub Releases](https://github.com/kyndlo/tapioca/releases) provide
platform-native archives:

- `tapioca-darwin-arm64.tar.gz` includes Tapioca, the Metal-enabled
  `llama-server`, and the compiled Swift/MLX image runtime with
  `mlx.metallib`.
- `tapioca-windows-amd64.zip` includes Tapioca and the Vulkan-enabled
  `llama-server` with its DLLs.

Extract the complete archive and run `tapioca` from its root. Keep the
`runtime` directory beside the executable.

Version tags such as `v0.2.0` automatically build and publish a GitHub Release.
Maintainers can also run the workflow manually with a release tag to publish
the current commit.

Model weights are never included. The Windows diffusion backend still creates
its isolated CUDA/Python environment on first use because PyTorch/CUDA packages
are several gigabytes and must match the installed NVIDIA driver. Coding-agent
clients such as Codex and Claude Code remain separate applications.

## Build

```bash
make build
```

GitHub Actions builds downloadable artifacts for:

- macOS on Apple Silicon (`darwin/arm64`)
- Windows x64 (`windows/amd64`)
- Windows ARM64 (`windows/arm64`)

## Quick start

```bash
./bin/tapioca pull glm-4.7-flash:q8_0
./bin/tapioca run glm-4.7-flash:q8_0
```

`run`, `serve`, `launch`, and `image` automatically pull catalog models that
are not installed yet, so the explicit `pull` step is optional:

```bash
./bin/tapioca run glm-4.7-flash:q8_0
```

Run Qwen3.6 through the native MLX text backend on Apple Silicon:

```bash
./bin/tapioca run qwen3.6:35b-mlx
```

The Ollama-compatible `35b-mlx` alias maps to the approximately 20 GB
`mlx-community/Qwen3.6-35B-A3B-4bit` snapshot on Hugging Face. Explicit
`35b-mlx-4bit`, `35b-mlx-6bit`, and `35b-mlx-8bit` variants are also available.
The first run creates an isolated `mlx-vlm` runtime under
`~/.tapioca/runtime/mlx-vlm`; subsequent runs reuse it.

Inside an interactive chat, enter `/bye` (or press Ctrl-D) to stop the local
server and exit.

Tapioca suppresses llama.cpp and HTTP request logs by default. Add `--verbose`
to `run`, `serve`, or `launch` when diagnosing startup or request problems.

Reasoning-capable models show their model-generated thinking separately from
the final answer by default:

```bash
./bin/tapioca run glm-4.7-flash:q8_0
```

Tapioca preserves reasoning in the conversation history so the model can
maintain continuity across turns. To hide the trace while retaining a progress
indicator, use:

```bash
./bin/tapioca run glm-4.7-flash:q8_0 --show-thinking=false
```

Interactive responses stream as they are generated.

Launch a coding agent:

```bash
./bin/tapioca launch codex glm-4.7-flash:q8_0
./bin/tapioca launch claude glm-4.7-flash:q8_0
./bin/tapioca launch opencode glm-4.7-flash:q8_0
./bin/tapioca launch openclaw glm-4.7-flash:q8_0
./bin/tapioca launch hermes glm-4.7-flash:q8_0
```

OpenClaw and Hermes run with isolated configuration under
`~/.tapioca/launch/`; Tapioca does not overwrite their normal user profiles.
If Hermes is not installed, use its official installer before launching it:

```bash
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
```

Or expose the APIs directly:

```bash
./bin/tapioca serve glm-4.7-flash:q8_0 --context 65536
```

Generate an image with Qwen-Image-Flash:

```bash
./bin/tapioca pull qwen-image-flash:int8
./bin/tapioca image qwen-image-flash:int8 \
  --prompt "A red fox in a snowy pine forest at golden hour" \
  --output fox.png \
  --seed 42
```

On Windows x64, the unqualified model name automatically selects the original
BF16 Diffusers checkpoint and the CUDA backend:

```powershell
tapioca pull qwen-image-flash
tapioca image qwen-image-flash --prompt "A glass city floating above the ocean" --output city.png
```

The first Windows run creates an isolated Python environment under
`%USERPROFILE%\.tapioca\runtime` and installs CUDA-enabled PyTorch and
Diffusers. GPUs below 64 GB VRAM use sequential CPU offload automatically, so
Qwen-Image-Flash also needs substantial system RAM and will generate much more
slowly on common 16–24 GB cards.

The image snapshot is approximately 30 GB. The first generation compiles and
caches the native Swift/MLX runtime under `~/.tapioca/runtime`; later runs reuse
it. Qwen-Image-Flash defaults to its intended 1024×1024, four-step, CFG 1.0
configuration. Width and height may be changed but must be divisible by 16.

Endpoints:

- `GET /health`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`

Tapioca stores models, runtime caches, and generated launcher configuration in `~/.tapioca`.
Override this with `TAPIOCA_HOME`.

## MVP limitations

- The built-in catalog currently contains GLM-4.7-Flash Q4_K_M/Q8_0,
  Qwen3.6 35B-A3B MLX 4/6/8-bit, Qwen-Image-Flash MLX int8, and
  Qwen-Image-Flash Diffusers BF16.
- MLX diffusion generation requires Apple Silicon and macOS 26.
- CUDA diffusion generation requires Windows x64, an NVIDIA Ampere-or-newer
  GPU, and enough combined system/GPU memory for the approximately 58 GB BF16
  pipeline. Windows ARM64 image generation is not currently supported.
- Responses and Anthropic streaming are compatibility streams produced after
  generation completes. True token-by-token translation is planned.
- The launcher owns a server for the lifetime of its child coding-agent process.
- Tool-call quality depends on the model's embedded chat template and current
  llama.cpp Jinja support.
