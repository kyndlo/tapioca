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

- Dimensions must be divisible by 32.
- Most frame counts have the form `4n+1`, such as 17, 41, or 81.
- LTX-Video frame counts have the form `8n+1`.
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
