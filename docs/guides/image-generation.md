# Image generation

## Apple Silicon

For a 16–32 GiB Mac, start with FLUX.2 Klein 4B Q4:

```bash
tapioca image flux2-klein:4b-q4-mlx \
  --prompt "A red fox in a snowy pine forest at golden hour" \
  --output fox.png
```

The model is approximately 4.6 GiB and uses MFLUX. The first run creates an
isolated runtime under `~/.tapioca/runtime`; later generations reuse it.

For a larger Mac:

```bash
tapioca image qwen-image-flash:int8 \
  --prompt "A red fox in a snowy pine forest at golden hour" \
  --output fox.png
```

Qwen Image Flash is approximately 28 GiB and requires Apple Silicon and
macOS 26 in the current release.

### Krea 2 Turbo on a large Mac

Krea 2 Turbo is an experimental Apple Metal option for Macs with at least
64 GiB unified memory; 96 GiB is recommended. The curated snapshot is about
34 GiB. It is gated, so sign in to Hugging Face, accept the terms on the
[Krea 2 Turbo model page](https://huggingface.co/krea/Krea-2-Turbo), and create
a read token. Then:

```bash
export HF_TOKEN=hf_your_read_token
tapioca pull krea-2-turbo --accept-license
tapioca image krea-2-turbo \
  --prompt "Editorial photograph of a glass sculpture, studio lighting" \
  --width 1024 --height 1024 --steps 8 --output sculpture.png
```

The acceptance is stored locally in `~/.tapioca/licenses.json`; the CLI reads
the token from the environment and does not write it there. Tapioca Desktop can
accept a token for one pull and discards it afterward. The Krea license includes
commercial-use limits and an acceptable-use policy. Review every output before
sharing it and review the current terms before using the model.

## Windows x64 with AMD or Intel graphics

Use the ONNX DirectML variant:

```powershell
tapioca image sd-turbo:onnx-directml `
  --prompt "A red fox in snow" `
  --output fox.png
```

This supports DirectX 12 GPUs from AMD, Intel, and NVIDIA. The first run creates
an isolated Python environment under `%USERPROFILE%\.tapioca\runtime` and
installs ONNX Runtime DirectML. Python 3.11–3.14 is required. Integrated GPUs
share system memory, so close memory-heavy applications before generating.

## Windows ARM64

On Snapdragon and other Windows ARM64 devices:

```powershell
tapioca image sd-turbo `
  --prompt "A red fox in snow" `
  --output fox.png
```

The untagged `sd-turbo` name automatically selects `onnx-arm64`. This uses
native ARM64 ONNX Runtime on the CPU. It works without CUDA or x64 emulation,
but generation is slower than GPU-backed DirectML. Python 3.11–3.14 ARM64 is
required; verify it with `python -c "import platform; print(platform.machine())"`.

## Windows x64 with NVIDIA CUDA

Start with SD Turbo:

```powershell
tapioca image sd-turbo:fp16 `
  --prompt "A red fox in snow" `
  --output fox.png
```

| Model | Download | Default output | Notes |
| --- | ---: | ---: | --- |
| `sd-turbo:onnx-directml` | ~4.8 GiB | 512×512 | AMD/Intel/NVIDIA DirectX 12 |
| `sd-turbo:onnx-arm64` | ~4.8 GiB | 512×512 | Windows ARM64 CPU; slower |
| `sd-turbo:fp16` | ~3 GiB | 512×512 | Best first model |
| `sdxl-turbo:fp16` | ~7 GiB | 1024×1024 | Better detail |
| `qwen-image-flash:bf16` | ~58 GiB | 1024×1024 | Ampere+; very high memory |
| `krea-2-turbo:bf16-cuda` | ~34 GiB | 1024×1024 | NVIDIA 16 GiB minimum with CPU offload; 24 GiB recommended |

The first run creates a Python environment under
`%USERPROFILE%\.tapioca\runtime` and installs CUDA PyTorch and Diffusers.

Krea 2 Turbo also requires a one-time provider and license setup:

```powershell
$env:HF_TOKEN = "hf_your_read_token"
tapioca pull krea-2-turbo --accept-license
tapioca image krea-2-turbo --prompt "A cinematic mountain observatory at dawn" `
  --width 1024 --height 1024 --steps 8 --output observatory.png
```

Accept access on the [Hugging Face model page](https://huggingface.co/krea/Krea-2-Turbo)
before running the command. A 16 GiB NVIDIA GPU can use sequential CPU offload,
but generation is slower; 24 GiB VRAM is recommended. Krea 2 is not currently
available through Tapioca on AMD, Intel, or Windows ARM64 hardware.

## Gated models and licenses

`--accept-license` is an explicit acknowledgement, not a provider bypass. For
a gated model you must complete all three steps:

1. Accept access in your own provider account.
2. Set `HF_TOKEN` or `HUGGING_FACE_HUB_TOKEN` to a read token.
3. Run `tapioca pull MODEL --accept-license` after reviewing the named license.

Tapioca stores only the model, license name, license URL, and acceptance time.
It does not store the token. Desktop passes a pasted token only to its local
downloader for that pull. Agents must not accept model terms for a user.

## Useful flags

```text
--prompt TEXT
--negative-prompt TEXT
--output FILE
--width PIXELS
--height PIXELS
--steps NUMBER
--seed NUMBER
```

Width and height must be positive and divisible by 16.

## Edit one or more images

`edit` requires at least one input image. Repeat `--image` when the model
supports multiple references:

```bash
tapioca edit flux2-klein:4b-q4-mlx \
  --image room.png \
  --image chair.png \
  --prompt "Place the chair from Image 2 in the room from Image 1" \
  --output furnished-room.png
```

Input order matters and is passed to the model in command-line order.

## Use a LoRA

```bash
tapioca edit flux2-klein:4b-q4-mlx \
  --adapter hf://Alissonerdx/BFS-Best-Face-Swap \
  --adapter-file bfs_head_v1_flux-klein_4b.safetensors \
  --adapter-scale 1.0 \
  --image body.png \
  --image face.png \
  --prompt "head swap: use Image 1 as the body and Image 2 as the face" \
  --output result.png
```

Tapioca downloads the adapter automatically. Only use a person's identity
with permission. The [LoRA guide](../concepts/lora-adapters.md) explains how
to inspect a repository, select a weight file, and understand the local files.

## Use a direct Hugging Face base model

Catalog entries are the easiest path because their defaults have been curated.
Advanced users can select another compatible base repository:

```bash
tapioca image hf://OWNER/REPOSITORY \
  --prompt "A red fox in snow" \
  --output fox.png
```

On Apple Silicon, the repository must be supported by MFLUX. Direct Hugging
Face models on Windows still use CUDA Diffusers; DirectML and ARM64 require a
pre-exported ONNX model and currently use the curated `sd-turbo` entries. A
model being hosted on Hugging Face does not by itself guarantee runtime
compatibility.

ONNX models use static computation graphs, so Tapioca cannot attach arbitrary
LoRAs to the DirectML or ARM64 variants at runtime. Use CUDA Diffusers or
macOS MFLUX when dynamic LoRA loading is required.

Krea 2 Turbo uses Diffusers' Krea pipeline and supports compatible Krea 2 LoRA
weights. Inspect the adapter's declared base model before use; Flux, SDXL,
Qwen, and MiniMax-H3 LoRAs are not interchangeable with Krea 2 LoRAs.
