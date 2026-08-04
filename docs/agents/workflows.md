# Agent workflows

## Select a local model

Run `tapioca list` first, then `tapioca catalog`. Prefer:

- An installed model to avoid an unnecessary download.
- A coder/tool model for repository work.
- A small quantization for lower-memory machines.
- MLX variants on Apple Silicon when the catalog marks them compatible.
- GGUF/Vulkan variants for portable Windows and Linux text inference.

Do not estimate compatibility from parameter count alone. Use the catalog's
memory and platform fields.

## Text and tool use

```bash
tapioca serve glm-4.7-flash:q8_0 --host 127.0.0.1 --port 11435
```

Wait for `/health`, then use `/v1/responses`, `/v1/chat/completions`, or
`/v1/messages`.

## Coding agents

```bash
tapioca launch codex MODEL
tapioca launch claude MODEL
tapioca launch opencode MODEL
tapioca launch openclaw MODEL
tapioca launch hermes MODEL
```

The selected client must already be installed. Pass client arguments after
`--`. Tapioca creates isolated launch configuration under
`TAPIOCA_HOME/launch`.

## Images

```bash
tapioca image MODEL \
  --prompt "A friendly pearl astronaut" \
  --output image.png
```

Use a catalog model compatible with the current OS and GPU. Image generation
may take minutes on CPU. Do not treat an absent percentage as a stalled job.

## Video

```bash
tapioca video MODEL \
  --prompt "A friendly pearl astronaut waving" \
  --image image.png \
  --output video.mp4
```

When an input image is provided, preserve its path exactly and confirm it
exists before starting the job.

## Speech and voices

```bash
tapioca tts MODEL --text "Hello from Tapioca." --output hello.wav
```

For cloning, create a named voice only from consented audio, then pass that
voice to `tts`. Read the speech guide before choosing model-specific flags.

## Diagnose

1. Run `tapioca version`.
2. Confirm the model appears in `tapioca list` or `catalog`.
3. Retry the failing command with `--verbose`.
4. Check the selected backend's platform and dependency requirements.
5. Report the command, backend, and actionable final error without dumping
   unrelated runtime logs.
