# Agent workflows

## Select a local model

Run `tapioca list`, then `tapioca catalog update`, then `tapioca catalog`.
Catalog refresh is a safe read of published model recipes; it does not download
model weights or replace the application. Prefer:

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

### MiniMax-H3 video and LoRAs

`minimax-h3` is a platform-resolved four-file bundle. On Apple Silicon it uses
the MPS variant; Windows and Linux NVIDIA hosts use the CUDA variant. Always
let `tapioca pull` resolve and verify the complete bundle:

```bash
tapioca pull minimax-h3
tapioca video minimax-h3 \
  --prompt 'A friendly presenter says exactly: "Hello from Tapioca."' \
  --preset low-memory \
  --output minimax-h3.mp4
```

The low-memory preset generates 640×352, 73 frames, 10 sampling steps, and
24 FPS. MiniMax-H3 frame counts must have the form `17n+5`.

Before using an adapter, inspect it and verify that its model card declares
MiniMax-H3 as the base architecture:

```bash
tapioca adapter inspect hf://OWNER/REPOSITORY
tapioca adapter pull hf://OWNER/REPOSITORY --file adapter.safetensors
tapioca video minimax-h3 \
  --prompt "A cinematic tracking shot" \
  --adapter 'hf://OWNER/REPOSITORY#adapter.safetensors@0.8' \
  --output adapted.mp4
```

Before any provider call, run `tapioca adapter list`. When a compatible item is
present, reuse the returned canonical reference instead of pulling it again.
Changing only the `@SCALE` suffix does not require another copy. Use
`adapter import --base` for files already present on disk; `--file` selects a
remote repository member and is not a local path flag.

Provider references are interchangeable at the CLI boundary:

```bash
tapioca adapter inspect civitai://MODEL_ID/VERSION_ID
tapioca adapter inspect ms://OWNER/REPOSITORY
tapioca adapter import ./adapter.safetensors --base minimax-h3 --name my-adapter
```

Use the `reference` returned by `adapter list` or the import response. Do not
invent a `local://` path or pass an arbitrary filesystem path to generation.

Repeat `--adapter` to create an ordered stack. Tapioca applies MiniMax-H3 LoRAs
to the diffusion transformer in command order. Do not assume a Flux, Wan,
Qwen, LTX, or Stable Diffusion LoRA is compatible.

Model download and first-run runtime setup can be long. A generation that only
reports elapsed time is not necessarily stalled. Wait for the command to exit,
then validate that the returned MP4 exists and contains the expected streams;
use `ffprobe` when it is installed.

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
