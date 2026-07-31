# Tapioca

Tapioca is a small local-model runtime for language and diffusion models. It
downloads models on demand, runs GGUF models through llama.cpp, supports native
MLX models on Apple Silicon, and presents compatible APIs to coding agents.

## Install on macOS

Tapioca's macOS release is for Apple Silicon. Download
`tapioca-darwin-arm64.tar.gz` from
[GitHub Releases](https://github.com/kyndlo/tapioca/releases), then:

```bash
mkdir -p "$HOME/.local/tapioca"
tar -xzf tapioca-darwin-arm64.tar.gz -C "$HOME/.local/tapioca"
export PATH="$HOME/.local/tapioca:$PATH"
tapioca version
```

Add the `export PATH=...` line to `~/.zshrc` to make the command available in
new terminals. Keep the bundled `runtime` directory beside the executable.
The archive includes the Metal-enabled llama.cpp server, MLX image runtime,
and Metal shaders.

Start with:

```bash
tapioca catalog
tapioca run gemma3:12b-mlx
```

The model is pulled automatically if it is not already installed. Use `/bye`
or Ctrl-D to leave the chat and stop its server.

### macOS video generation

For a 32 GiB Apple Silicon Mac, use the native MLX Wan2.2 Q8 model and begin
with its low-memory preset:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --prompt "A red fox running through soft falling snow, cinematic" \
  --preset low-memory \
  --output fox.mp4
```

The model is approximately 18 GiB. Its catalog requirement is 32 GiB minimum
and 48 GiB recommended. At 32 GiB, close memory-heavy applications. The
low-memory preset uses 640×352, 41 frames, and 30 steps.

Without `--preset`, Tapioca uses the balanced 832×480, 41-frame, 40-step
profile. Macs with 64 GiB or more can request the model's native
1280×704, 81-frame, 40-step recipe:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --prompt "A red fox running through soft falling snow" \
  --preset quality \
  --output fox-quality.mp4
```

`yume-video:5b-mlx` is another text/image-to-video option, but its approximately
23 GiB snapshot is better suited to 48 GiB or larger Macs. Add
`--image start.png` to either model for image-to-video generation.

### macOS image generation

For 16–32 GiB Macs, FLUX.2 Klein 4B Q4 is the recommended native MLX model:

```bash
tapioca image flux2-klein:4b-q4-mlx \
  --prompt "A red fox in a snowy pine forest at golden hour" \
  --output fox.png
```

It downloads approximately 4.6 GiB, requires about 16 GiB unified memory, and
is comfortable on a 24 GiB Mac. The first run creates an isolated MFLUX
runtime that later generations reuse.

On larger Macs, Qwen Image Flash uses the native Swift/MLX backend:

```bash
tapioca image qwen-image-flash:int8 \
  --prompt "A red fox in a snowy pine forest at golden hour" \
  --output fox.png
```

Its model download is approximately 28 GiB. The first generation prepares the
MLX runtime; subsequent generations reuse it. MLX image generation currently
requires Apple Silicon and macOS 26.

## Install on Windows

Download `tapioca-windows-amd64.zip` from
[GitHub Releases](https://github.com/kyndlo/tapioca/releases). In PowerShell:

```powershell
New-Item -ItemType Directory -Force "$HOME\Apps\tapioca"
Expand-Archive .\tapioca-windows-amd64.zip "$HOME\Apps\tapioca" -Force
$env:Path = "$HOME\Apps\tapioca;$env:Path"
tapioca version
```

Keep the bundled `runtime` directory and DLLs beside `tapioca.exe`. Add the
directory to the Windows user `Path` environment variable to use Tapioca from
future terminals.

For a computer with 12 GiB of system memory, start with:

```powershell
tapioca catalog
tapioca run qwen3:4b-q4_k_m --context 8192
```

The Windows bundle uses Vulkan for GGUF text models. A discrete NVIDIA GPU is
not required for text models, although available GPU acceleration improves
performance.

### Windows image generation

Image generation currently requires Windows x64, Python 3.10 or newer, a
current NVIDIA driver, and a CUDA-capable NVIDIA GPU. Start with the compact
SD Turbo model:

```powershell
tapioca image sd-turbo:fp16 `
  --prompt "A red fox in snow" `
  --output fox.png
```

The first image run creates an isolated Python environment under
`%USERPROFILE%\.tapioca\runtime` and installs CUDA-enabled PyTorch and
Diffusers. This can take several minutes and requires additional disk space.
Tapioca loads the pipeline on the GPU when it fits and uses sequential CPU
offload otherwise.

Available Windows image choices:

| Model | Download | Default size | Notes |
| --- | ---: | ---: | --- |
| `sd-turbo:fp16` | ~3 GiB | 512×512 | Best starting point |
| `sdxl-turbo:fp16` | ~7 GiB | 1024×1024 | Better detail; needs more memory |
| `qwen-image-flash:bf16` | ~58 GiB | 1024×1024 | Ampere or newer; very high RAM/VRAM needs |

Windows ARM64 image generation and AMD/Intel image acceleration are not
currently supported.

### Windows video generation

Video generation requires Windows x64, Python 3.10 or newer, and an NVIDIA
CUDA GPU. For machines with 24–32 GiB of system memory:

```powershell
# Text-to-video; approximately 24 GiB on disk
tapioca video ltx-video:2b-fp16 `
  --prompt "A small sailboat crossing a calm lake at sunrise" `
  --width 768 --height 512 --frames 17 --steps 8 `
  --output sailboat.mp4

# Lighter image-to-video; approximately 4.5 GiB on disk
tapioca video stable-video-diffusion:xt-fp16 `
  --image start.png `
  --prompt "Gentle camera movement" `
  --frames 25 --output animated.mp4
```

LTX-Video 2B requires approximately 24 GiB of system RAM and an 8 GiB CUDA GPU
when sequential CPU offload is used; 32 GiB RAM is recommended. Stable Video
Diffusion requires a starting image and is the safer choice for a 24 GiB
computer. The first run builds an isolated CUDA/Python environment, which is
then reused.

## Everyday commands

```text
tapioca catalog
tapioca list
tapioca pull MODEL[:VARIANT]
tapioca run MODEL [--context TOKENS]
tapioca serve MODEL [--port 11435] [--context TOKENS]
tapioca image MODEL --prompt TEXT [--output image.png]
tapioca video MODEL --prompt TEXT [--image start.png] [--output video.mp4]
tapioca launch CLIENT MODEL [-- CLIENT_ARGS...]
```

- `catalog` shows available models, backends, download sizes, and platforms.
- `list` shows models already installed locally.
- `pull` downloads a model explicitly. It is optional because `run`, `serve`,
  `launch`, and `image` pull missing catalog models automatically.
- `run` opens an interactive chat. Reasoning is visible by default; pass
  `--show-thinking=false` to hide it.
- `video` generates MP4 video. Dimensions must be divisible by 32 and frame
  counts use the form `4n+1`, such as 17, 41, or 81. It enhances short prompts
  with temporal-consistency guidance by default; use `--enhance-prompt=false`
  for exact prompt passthrough. The presets are `low-memory`, `balanced`
  (default), and `quality`.
- `serve` exposes OpenAI Responses/chat and Anthropic Messages-compatible APIs.
- `launch` supports `codex`, `claude`, `opencode`, `openclaw`, and `hermes`.
- Add `--verbose` to `run`, `serve`, or `launch` when diagnosing startup.

The coding-agent client must already be installed and available on `PATH`;
Tapioca supplies its local model endpoint but does not bundle Codex, Claude
Code, OpenCode, OpenClaw, or Hermes. Arguments after `--` are passed to the
client:

```bash
tapioca launch opencode qwen3-coder:30b-mlx -- --help
```

## Storage

Tapioca stores models, runtime environments, and launcher configuration in:

- macOS and Linux: `~/.tapioca`
- Windows: `%USERPROFILE%\.tapioca`

Set `TAPIOCA_HOME` to use another location. Model weights are not included in
release archives and can require significant disk space.

The `tapioca catalog` columns are:

- `DOWNLOAD`: approximate model snapshot size, not runtime memory.
- `MEMORY`: minimum and recommended total system/unified memory.
- `GPU`: required accelerator and approximate VRAM where applicable.
- `PLATFORM`: operating systems supported by Tapioca's current backend.

These figures are conservative starting points. Video resolution, frame count,
context length, and other active applications can materially change peak
memory.

## Build from source

Building requires Go 1.25 or newer:

```bash
make build
```

Source builds can use a separately installed llama.cpp; on macOS:

```bash
brew install llama.cpp
```

Building the native macOS image runtime requires Xcode 26, Swift 6.2 or newer,
and the Xcode Metal Toolchain:

```bash
xcodebuild -downloadComponent MetalToolchain
```

## Recommended text models

Run Qwen3.6 through the native MLX text backend on Apple Silicon:

```bash
tapioca run qwen3.6:35b-mlx
```

The Ollama-compatible `35b-mlx` alias maps to the approximately 20 GB
`mlx-community/Qwen3.6-35B-A3B-4bit` snapshot on Hugging Face. Explicit
`35b-mlx-4bit`, `35b-mlx-6bit`, and `35b-mlx-8bit` variants are also available.
The first run creates an isolated `mlx-vlm` runtime under
`~/.tapioca/runtime/mlx-vlm`; subsequent runs reuse it.

For a Windows machine with 12 GiB of system memory, start with one of these
GGUF models:

| Model | Download | Good for |
| --- | ---: | --- |
| `qwen3:4b-q4_k_m` | ~2.3 GiB | Fast chat and lightweight tool use |
| `phi4-mini:q4_k_m` | ~2.3 GiB | Instruction following and function calling |
| `gemma3:4b-q4_k_m` | ~2.3 GiB | General conversation |
| `qwen3:8b-q4_k_m` | ~4.7 GiB | Better quality while remaining practical in 12 GiB |

The download size is only the model weights. Runtime memory also includes the
context/KV cache and the application itself. On a 12 GiB computer, use an
8K–16K context rather than the maximum context advertised by the model.

Apple Silicon users can additionally use native MLX models:

| Model | Download | Good for |
| --- | ---: | --- |
| `gemma3:12b-mlx` | ~7.5 GiB | Fast general conversation |
| `gemma3:27b-mlx` | ~16 GiB | Higher-quality conversation |
| `qwen3:30b-mlx` | ~16 GiB | Strong general use and tool calling |
| `qwen3-coder:30b-mlx` | ~16 GiB | Coding agents and tool-heavy workflows |
| `qwen3.6:35b-mlx` | ~20 GiB | Existing Qwen3.6 MLX option |

Inside an interactive chat, enter `/bye` (or press Ctrl-D) to stop the local
server and exit.

Tapioca suppresses llama.cpp and HTTP request logs by default. Add `--verbose`
to `run`, `serve`, or `launch` when diagnosing startup or request problems.

Reasoning-capable models show their model-generated thinking separately from
the final answer by default:

```bash
tapioca run glm-4.7-flash:q8_0
```

Tapioca preserves reasoning in the conversation history so the model can
maintain continuity across turns. To hide the trace while retaining a progress
indicator, use:

```bash
tapioca run glm-4.7-flash:q8_0 --show-thinking=false
```

Interactive responses stream as they are generated.

Launch a coding agent:

```bash
tapioca launch codex glm-4.7-flash:q8_0
tapioca launch claude glm-4.7-flash:q8_0
tapioca launch opencode glm-4.7-flash:q8_0
tapioca launch openclaw glm-4.7-flash:q8_0
tapioca launch hermes glm-4.7-flash:q8_0
```

OpenClaw and Hermes run with isolated configuration under
the `launch` directory inside `TAPIOCA_HOME`; Tapioca does not overwrite their
normal user profiles.
If Hermes is not installed, use its official installer before launching it:

```bash
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
```

Or expose the APIs directly:

```bash
tapioca serve glm-4.7-flash:q8_0 --context 65536
```

Endpoints:

- `GET /health`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`

## MVP limitations

- The built-in catalog is intentionally curated rather than a general
  Hugging Face model browser. Run `tapioca catalog` for the current list.
- MLX diffusion generation requires Apple Silicon and macOS 26.
- CUDA diffusion generation requires Windows x64 and a supported NVIDIA GPU.
  Qwen-Image-Flash BF16 requires Ampere or newer and substantial combined
  system/GPU memory; SD Turbo and SDXL Turbo have much smaller FP16 weights.
  Windows ARM64 image generation is not currently supported.
- Video generation currently supports native MLX on Apple Silicon and CUDA
  Diffusers on Windows x64. Windows AMD/Intel GPUs and macOS PyTorch/MPS video
  execution are not yet exposed by Tapioca.
- Responses and Anthropic streaming are compatibility streams produced after
  generation completes. True token-by-token translation is planned.
- The launcher owns a server for the lifetime of its child coding-agent process.
- Tool-call quality depends on the model's embedded chat template and current
  llama.cpp Jinja support.
