# Video engines

Tapioca's public video contract is independent of any one inference project.
The CLI, desktop application, catalog, model bundles, LoRA registry, presets,
progress, and output paths belong to Tapioca. An internal video engine performs
the hardware-specific inference.

```text
CLI / Desktop / control API
            |
            v
    Tapioca video request
            |
            v
       VideoEngine
       |    |    |
       |    |    +-- Diffusers (CUDA)
       |    +------- MLX video (Apple Silicon)
       +------------ managed ComfyUI (MiniMax-H3)
```

This boundary means Tapioca can replace or add an engine without changing a
command such as:

```bash
tapioca video minimax-h3 --prompt "A presenter waves" --output result.mp4
```

## What Tapioca owns

- Platform-aware catalog resolution and multi-file model bundles.
- Adapter discovery, download, ordering, scale, and basic compatibility checks.
- Input validation, presets, output locations, and the control protocol.
- The desktop experience and stable CLI commands.
- Runtime pinning and reproducible workflow construction.

## What the managed MiniMax-H3 engine owns

The current MiniMax-H3 engine starts a private, pinned ComfyUI process. Tapioca
builds the request graph and communicates with it over localhost. ComfyUI is
not exposed as Tapioca's public API, and users do not manage its graph or model
directories.

Tapioca stages only the selected LoRA files into the private runtime. It chains
them through model-only loader nodes in command order and then connects the
resulting transformer to both the scheduler and guider. The Qwen3-VL text
encoder is deliberately left unchanged.

Keeping this implementation behind `VideoEngine` allows a future native
PyTorch or MLX MiniMax-H3 engine to reuse the same catalog entries, installed
models, commands, and desktop controls.

## Compatibility boundary

Tapioca rejects obvious cross-family adapter mistakes before generation. Exact
tensor compatibility is checked by the selected engine because converted or
quantized weights can use architecture-specific tensor names. A LoRA should
identify MiniMax-H3 as its base architecture; sharing the `.safetensors`
extension is not sufficient.
