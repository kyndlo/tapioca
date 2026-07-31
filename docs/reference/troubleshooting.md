# Troubleshooting

## `tapioca` is not found

Confirm the installation directory is on `PATH`. See
[Installation](../getting-started/installation.md).

## A model is unknown

Run:

```bash
tapioca catalog
```

Text commands use curated catalog references. Image and video commands also
accept `hf://OWNER/REPOSITORY`, but the repository must be compatible with the
selected MFLUX, MLX-video, or Diffusers runtime. Ollama library names are not
general Hugging Face references.

## A download was interrupted

Run the same command again. Direct model downloads use a partial file and
resume when the server supports byte ranges. Use `tapioca pull MODEL --force`
only when a completed local copy must be replaced.

## macOS image generation reports a Metal error

Use the self-contained macOS release and keep its `runtime` directory beside
the executable. Source builds require Xcode 26, Swift 6.2 or newer, and:

```bash
xcodebuild -downloadComponent MetalToolchain
```

## Windows image or video generation cannot find CUDA

The current diffusion backend requires Windows x64, Python 3.10 or newer, a
current NVIDIA driver, and a CUDA-capable NVIDIA GPU. AMD/Intel diffusion and
Windows ARM64 diffusion are not supported yet.

## The computer runs out of memory

- Choose a smaller model or quantization.
- Reduce text context.
- Use the video's `low-memory` preset.
- Reduce image/video resolution, frames, or steps.
- Close memory-heavy applications.
- Confirm that `DOWNLOAD` was not mistaken for peak runtime memory.

## Runtime logs are hidden

Add `--verbose` to `run`, `serve`, or `launch` when diagnosing text-model
startup and requests.

## A LoRA is incompatible

LoRAs are architecture-specific. Check the adapter model card and select the
weight file trained for the exact base-model family. `flux`, `qwen`, `wan`,
and `ltx` in filenames are useful indicators but not complete guarantees.
Use `tapioca adapter inspect REFERENCE` before pulling.

The native Qwen Image Flash Swift/MLX backend does not load LoRAs. On macOS,
use a compatible MFLUX model such as FLUX.2 Klein. Stable Video Diffusion also
does not expose LoRA loading through Tapioca.
