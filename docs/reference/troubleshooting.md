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

Use `sd-turbo:fp16` only with Windows x64, a current NVIDIA driver, and CUDA.
For an AMD or Intel DirectX 12 GPU, use:

```powershell
tapioca image sd-turbo:onnx-directml --prompt "A red fox in snow"
```

DirectML requires x64 Python 3.11–3.14. On Windows ARM64, use untagged
`sd-turbo` (or `sd-turbo:onnx-arm64`) with native ARM64 Python 3.11–3.14.
Check the interpreter architecture:

```powershell
python -c "import platform; print(platform.machine())"
```

The ARM64 backend runs on CPU and will be slower. Video generation still
requires Windows x64 with NVIDIA CUDA.

MiniMax-H3 manages its own Python 3.12 and CUDA-enabled PyTorch on Windows x64;
do not install the CUDA Toolkit just for Tapioca. Run `nvidia-smi` to verify the
driver is visible. Tapioca reports the selected GPU, VRAM, and driver before it
installs or starts the runtime. A missing `nvidia-smi` means the NVIDIA driver
is absent, broken, or unavailable to the process.

## Linux cannot use the GPU

Text models use the bundled Vulkan runtime. Confirm the GPU and Vulkan loader
are visible:

```bash
vulkaninfo --summary
```

Image, video, and GPU-accelerated speech use NVIDIA CUDA. Confirm the host can
see the driver before running Tapioca:

```bash
nvidia-smi
```

If either command is missing, install the current driver and the distribution's
Vulkan utilities or NVIDIA CUDA packages. See the [Linux guide](../guides/linux.md).

## Speech generation is slow on the first run

The first `tapioca tts` command creates an isolated Python environment, installs
the selected backend, and downloads model weights. Later runs reuse all three.
CPU generation works but can be much slower than Apple MPS, MLX, or NVIDIA
CUDA. Use `chatterbox:multilingual` for a text-only test. Chatterbox Nano is
lighter but requires `--voice-sample` or a saved `--voice`.

## A cloned voice does not sound like the sample

- Use a clean 3–10 second recording with one speaker and no music.
- Remove long silence at the beginning and end.
- Supply the exact transcript spoken in the sample for Qwen3-TTS.
- Avoid switching languages or speaking styles between sample and output.
- Try WAV input and normal conversational volume.

Run `tapioca voice inspect NAME` to confirm the saved model family, transcript,
and reference file before generating again.

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
weight file trained for the exact base-model family. `minimax-h3`, `flux`,
`qwen`, `wan`, and `ltx` in filenames are useful indicators but not complete guarantees.
Use `tapioca adapter inspect REFERENCE` before pulling.

The native Qwen Image Flash Swift/MLX backend does not load LoRAs. On macOS,
use a compatible MFLUX model such as FLUX.2 Klein. Stable Video Diffusion also
does not expose LoRA loading through Tapioca.
