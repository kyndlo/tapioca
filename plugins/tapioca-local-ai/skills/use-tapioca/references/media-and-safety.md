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
