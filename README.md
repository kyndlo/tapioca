# Tapioca

Tapioca is a small local-model runtime for language, image, and video models.
It downloads supported models on demand, chooses a native backend for the
computer, and can expose local models to coding agents.

## Start here

1. [Install Tapioca on macOS or Windows](docs/getting-started/installation.md)
2. [Run your first local model](docs/getting-started/quickstart.md)
3. [Choose a model for your computer](docs/guides/choosing-models.md)
4. Explore the task-specific guides below.

```bash
tapioca catalog
tapioca run gemma3:12b-mlx
```

Tapioca pulls a catalog model automatically when it is first used. Enter
`/bye` or press Ctrl-D to leave an interactive chat and stop its server.

## Documentation

### Getting started

- [Installation](docs/getting-started/installation.md)
- [Quickstart](docs/getting-started/quickstart.md)
- [Choosing models and understanding memory](docs/guides/choosing-models.md)

### Use Tapioca

- [Text chat and local APIs](docs/guides/text-models.md)
- [Coding agents: Codex, Claude Code, OpenCode, OpenClaw, and Hermes](docs/guides/coding-agents.md)
- [Image generation](docs/guides/image-generation.md)
- [Video generation](docs/guides/video-generation.md)
- [LoRA adapters and model composition](docs/concepts/lora-adapters.md)

### Reference and help

- [Command reference](docs/reference/commands.md)
- [Files, models, and disk storage](docs/reference/storage.md)
- [Troubleshooting](docs/reference/troubleshooting.md)
- [Build from source](docs/reference/building.md)

## Platform summary

| Capability | macOS Apple Silicon | Windows x64 | Windows ARM64 |
| --- | --- | --- | --- |
| GGUF text models | Metal llama.cpp | Vulkan llama.cpp | CPU llama.cpp |
| Native MLX text | Yes | No | No |
| Image generation | MLX/MFLUX | CUDA or ONNX DirectML | ONNX Runtime CPU |
| Video generation | MLX | NVIDIA CUDA/Diffusers | Not yet |
| Coding-agent APIs | Yes | Yes | Yes |

Run `tapioca catalog` for the exact models, download sizes, memory guidance,
GPU requirements, and supported platforms in the installed release.

## Everyday commands

```text
tapioca catalog
tapioca list
tapioca pull MODEL[:VARIANT]
tapioca run MODEL [--context TOKENS]
tapioca serve MODEL [--port 11435] [--context TOKENS]
tapioca image MODEL --prompt TEXT [--output image.png]
tapioca edit MODEL --image FILE [--image FILE] --prompt TEXT
tapioca video MODEL --prompt TEXT [--image start.png] [--output video.mp4]
tapioca adapter (inspect|pull|list) [hf://OWNER/REPOSITORY]
tapioca create NAME --base MODEL [--adapter REFERENCE]
tapioca launch CLIENT MODEL [-- CLIENT_ARGS...]
```

See the [command reference](docs/reference/commands.md) for flags and examples.

## Current scope

The built-in catalog provides tested defaults, while image and video commands
can also accept direct `hf://OWNER/REPOSITORY` base-model references. LoRA
support is backend-specific: MFLUX image models, CUDA Diffusers pipelines, and
Wan models through MLX-video can load compatible adapters. Tapioca validates
obvious model-family mismatches, but the upstream runtime remains the final
authority for less common adapters. Windows x64 AMD/Intel GPUs use ONNX
DirectML; Windows ARM64 uses native ONNX Runtime CPU inference.
