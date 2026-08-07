# Media, files, and safety

## Images

```bash
tapioca image MODEL --prompt "A pearl astronaut" --output image.png
```

## Video

```bash
tapioca video MODEL --prompt "A pearl astronaut waving" \
  --image image.png --output video.mp4
```

Confirm input files exist. CPU diffusion can take minutes, and a runtime that
does not report a percentage is not necessarily stalled.

### MiniMax-H3

Use the platform-resolved `minimax-h3` ID; do not assemble its four weight
files manually. The low-memory preset is 640×352, 73 frames, 10 steps, and
24 FPS. Frame counts must have the form `17n+5`.

```bash
tapioca pull minimax-h3
tapioca video minimax-h3 \
  --prompt 'A friendly presenter says exactly: "Hello from Tapioca."' \
  --preset low-memory --output minimax-h3.mp4
```

For a LoRA, first run `tapioca adapter inspect`. Proceed only when the model
card identifies MiniMax-H3 as the base architecture:

```bash
tapioca video minimax-h3 \
  --prompt "A cinematic tracking shot" \
  --adapter 'hf://OWNER/REPOSITORY#adapter.safetensors@0.8' \
  --output adapted.mp4
```

Repeat `--adapter` for an ordered transformer LoRA stack. Wait for the command
to exit, confirm the returned MP4 exists, and use `ffprobe` to verify video and
audio streams when available. ComfyUI is a private, replaceable engine detail;
do not expose its node IDs, workflow files, or directories to users.

## Speech

```bash
tapioca tts MODEL --text "Hello from Tapioca." --output hello.wav
```

Clone or imitate a voice only after confirming permission to use the reference
recording.

## Storage and privacy

Tapioca uses `~/.tapioca` by default or `TAPIOCA_HOME` when set. Keep prompts,
models, voices, and outputs local unless the user explicitly asks to send them
elsewhere. Never delete stored assets without approval.
