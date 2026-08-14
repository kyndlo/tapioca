# Media, files, and safety

## Images

```bash
tapioca image MODEL --prompt "A pearl astronaut" \
  --random-seed --output image.png
```

Image, edit, and video use seed `0` by default. Use `--random-seed` for a new
variation, capture the printed seed, and report it with the output path. Reuse
that value with `--seed NUMBER` for the same generation settings. The two seed
flags are mutually exclusive.

## Video

```bash
tapioca video MODEL --prompt "A pearl astronaut waving" \
  --image image.png --seconds 5 --output video.mp4
```

Confirm input files exist. CPU diffusion can take minutes, and a runtime that
does not report a percentage is not necessarily stalled.
`--seconds` requests an approximate duration and prints the selected valid frame
count. Never combine it with `--frames`.

### MiniMax-H3

Use the platform-resolved `minimax-h3` ID; do not assemble its four weight
files manually. The low-memory preset is 640×352, 73 frames, 10 steps, and
24 FPS. Frame counts must have the form `17n+5`.

```bash
tapioca pull minimax-h3
tapioca video minimax-h3 \
  --prompt 'A friendly presenter says exactly: "Hello from Tapioca."' \
  --preset low-memory --seconds 5 --output minimax-h3.mp4
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
