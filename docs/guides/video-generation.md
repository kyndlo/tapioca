# Video generation

## Apple Silicon

For a 32 GiB Mac, start with the low-memory preset:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --prompt "A red fox running through soft falling snow, cinematic" \
  --preset low-memory \
  --output fox.mp4
```

The model is approximately 18 GiB. Close memory-heavy applications on a
32 GiB system.

Presets:

| Preset | Intended use |
| --- | --- |
| `low-memory` | Smaller resolution/frame workload |
| `balanced` | Default compromise |
| `quality` | Larger output for high-memory systems |

Macs with 64 GiB or more can try:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --prompt "A red fox running through soft falling snow" \
  --preset quality \
  --output fox-quality.mp4
```

Add `--image start.png` for image-to-video. `yume-video:5b-mlx` is another
option but its approximately 23 GiB snapshot is better suited to 48 GiB or
larger Macs.

## MiniMax-H3 with native audio

MiniMax-H3 creates video and stereo audio together. Tapioca downloads only the
four required files (about 41 GiB) and installs a private, pinned ComfyUI
runtime on the first generation:

```bash
# Apple Silicon, 48 GiB unified memory minimum
tapioca video minimax-h3 \
  --prompt 'A presenter waves and says exactly: "Hello from Tapioca."' \
  --preset low-memory \
  --output hello-h3.mp4

# Use a first frame
tapioca video minimax-h3 \
  --image presenter.png \
  --prompt 'The presenter smiles and says exactly: "Welcome."' \
  --preset low-memory \
  --output welcome-h3.mp4
```

The default `minimax-h3` variant is selected for the host:

| Host | Variant | Encoder | Practical requirement |
| --- | --- | --- | --- |
| Apple Silicon | `fl2va-int8-mac` | Q4_K_M GGUF | 48 GiB unified memory; 64 GiB recommended |
| Windows/Linux NVIDIA | `fl2va-int8-cuda` | NVFP4-AWQ | 32 GiB RAM and 16 GiB VRAM recommended |

On a 48 GiB Mac, a 3-second 864×480 clip can take roughly 15–20 minutes;
larger clips can take much longer. A 4070 Ti SUPER 16 GB is supported by the
CUDA variant. The first run is slower because Tapioca creates the managed
runtime. Windows x64 users need only a current NVIDIA driver: Tapioca manages
Python 3.12, ComfyUI, and CUDA-enabled PyTorch privately. The CUDA developer
Toolkit is not required. Fresh macOS and Linux installations still require
Python 3.10+; macOS also requires Git for its pinned custom nodes.

### NVIDIA setup on Windows

Users should not overclock the GPU, change its BIOS, or try to switch a
GeForce card to TCC mode. For a reliable run:

1. Install a current NVIDIA Studio or Game Ready driver and restart Windows.
2. Run `nvidia-smi` in PowerShell. It must show the GPU, driver, and available
   VRAM before starting Tapioca.
3. Close games, browsers using GPU acceleration, and other generators to free
   VRAM. Plug laptops into power and select the high-performance NVIDIA GPU for
   `tapioca.exe` in Windows **Settings > System > Display > Graphics**.
4. Keep at least 32 GiB of system RAM and the Windows page file enabled.

The MiniMax-H3 profile is tested with 16 GiB VRAM. The runtime warns rather
than rejecting smaller NVIDIA cards, because reduced resolution and frame
counts may work, but 12 GiB and smaller configurations are not currently a
supported performance target. Use LTX Video on 8–12 GiB cards. On systems
with multiple NVIDIA GPUs, Tapioca automatically selects the card with the
most VRAM.

MiniMax-H3 frame counts use `17n+5`, such as 5, 73, or 124. Ten or more steps
are recommended when judging speech. Compatible MiniMax-H3 transformer LoRAs
can be stacked in command order:

```bash
tapioca video minimax-h3 \
  --prompt "A cinematic tracking shot" \
  --adapter 'hf://creator/minimax-h3-cinematic#adapter.safetensors@0.8' \
  --adapter 'hf://creator/minimax-h3-motion#motion.safetensors@0.4' \
  --preset low-memory --output adapted.mp4
```

LoRAs are architecture-specific. Use adapters whose model card identifies
MiniMax-H3 as the base model. Tapioca validates obvious family mismatches and
the runtime validates tensor compatibility before sampling.

## Windows x64 with NVIDIA CUDA

```powershell
# Text to video
tapioca video ltx-video:2b-fp16 `
  --prompt "A small sailboat crossing a calm lake at sunrise" `
  --width 768 --height 512 --frames 17 --steps 8 `
  --output sailboat.mp4

# Image to video
tapioca video stable-video-diffusion:xt-fp16 `
  --image start.png `
  --prompt "Gentle camera movement" `
  --frames 25 --output animated.mp4
```

LTX-Video 2B needs approximately 24 GiB system RAM and an 8 GiB CUDA GPU with
offload; 32 GiB RAM is recommended. Stable Video Diffusion is approximately
4.5 GiB and requires a starting image.

## Rules and flags

- The default seed is `0`. Use `--random-seed` to generate and print a seed,
  then reuse that value with `--seed NUMBER` when reproducibility is needed.
  The two flags cannot be combined.
- Dimensions must be divisible by 32.
- Most frame counts have the form `4n+1`, such as 17, 41, or 81.
- LTX-Video frame counts have the form `8n+1`.
- MiniMax-H3 frame counts have the form `17n+5`.
- Prompt enhancement is enabled by default. Pass
  `--enhance-prompt=false` for exact prompt passthrough.

## Use video LoRAs

Wan models through the Apple Silicon MLX backend accept repeated LoRAs:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --adapter hf://OWNER/cinematic-motion#wan22-motion.safetensors@0.8 \
  --prompt "A runner crossing a rainy street" \
  --output runner.mp4
```

Compatible Windows Diffusers video pipelines can also load adapters. Stable
Video Diffusion does not expose LoRA loading through Tapioca.

Use repeated `--adapter` flags to combine adapters:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --adapter hf://OWNER/cinematic-motion#wan22-motion.safetensors@0.8 \
  --adapter hf://OWNER/character-motion#wan22-character.safetensors@0.6 \
  --prompt "The character walks through a neon city" \
  --output city.mp4
```

See [LoRA adapters](../concepts/lora-adapters.md) for repository inspection,
file selection, local storage, compatibility, and recipes.

## Use a direct Hugging Face base model

```bash
tapioca video hf://OWNER/REPOSITORY \
  --prompt "A sailboat at sunrise" \
  --output sailboat.mp4
```

The repository must use a model family supported by MLX-video on Apple
Silicon or Diffusers on Windows. Prefer catalog entries until you are
comfortable identifying model architectures.
