# Quickstart

## 1. View models that Tapioca knows how to run

```bash
tapioca catalog
```

The table shows the model name, task, runtime backend, approximate download,
memory guidance, GPU requirement, and platform.

## 2. Pick a small first model

On a 16 GiB or larger Apple Silicon Mac:

```bash
tapioca run gemma3:12b-mlx
```

On Windows with approximately 12 GiB system memory:

```powershell
tapioca run qwen3:4b-q4_k_m --context 8192
```

The first run downloads the model. Later runs reuse the local copy.

## 3. Chat

Type a message at the `>` prompt. Enter `/bye` or press Ctrl-D to exit.
Reasoning-capable models show their generated reasoning by default. Use
`--show-thinking=false` to hide it:

```bash
tapioca run glm-4.7-flash:q8_0 --show-thinking=false
```

## 4. See what is installed

```bash
tapioca list
```

## 5. Try another local medium

Generate speech:

```bash
tapioca tts chatterbox:nano \
  --text "Tapioca is running locally." \
  --output hello.wav
```

Or make an image:

```bash
tapioca image sd-turbo --prompt "A friendly pearl astronaut"
```

See [Choosing models](../guides/choosing-models.md) before downloading a large
model and [Storage](../reference/storage.md) to find the downloaded files.

When you are ready to customize image or video models, continue with the
[beginner LoRA guide](../concepts/lora-adapters.md). For speech and reusable
voices, follow the [voice guide](../guides/speech-and-voices.md).
