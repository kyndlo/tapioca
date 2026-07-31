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

## Windows x64 with NVIDIA CUDA

Start with SD Turbo:

```powershell
tapioca image sd-turbo:fp16 `
  --prompt "A red fox in snow" `
  --output fox.png
```

| Model | Download | Default output | Notes |
| --- | ---: | ---: | --- |
| `sd-turbo:fp16` | ~3 GiB | 512×512 | Best first model |
| `sdxl-turbo:fp16` | ~7 GiB | 1024×1024 | Better detail |
| `qwen-image-flash:bf16` | ~58 GiB | 1024×1024 | Ampere+; very high memory |

The first run creates a Python environment under
`%USERPROFILE%\.tapioca\runtime` and installs CUDA PyTorch and Diffusers.

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

On Apple Silicon, the repository must be supported by MFLUX. On Windows, it
must be loadable by the installed Diffusers version and CUDA backend. A model
being hosted on Hugging Face does not by itself guarantee runtime
compatibility.
