# Choose a model

A model name such as `qwen3:4b-q4_k_m` has two parts:

```text
qwen3 : 4b-q4_k_m
  │          │
  │          └─ variant: parameter size and quantization
  └─ model family
```

The variant affects download size, memory use, speed, and output quality.
Start with a smaller quantization and move up only when you need more quality.

## Understand the catalog columns

- `DOWNLOAD` is the approximate snapshot size on disk.
- `MEMORY` is conservative minimum and recommended total system or unified
  memory.
- `GPU` identifies required acceleration and approximate VRAM when applicable.
- `PLATFORM` describes the platforms supported by Tapioca's current backend.

Download size is not peak runtime memory. Context/KV cache, image resolution,
video frames, runtime libraries, and other applications consume more memory.

## Good starting points

### Windows with 12 GiB system memory

| Model | Download | Best for |
| --- | ---: | --- |
| `qwen3:4b-q4_k_m` | ~2.4 GiB | Fast chat and lightweight tool use |
| `phi4-mini:q4_k_m` | ~2.4 GiB | Instructions and function calling |
| `gemma3:4b-q4_k_m` | ~2.4 GiB | General conversation |
| `qwen3:8b-q4_k_m` | ~4.7 GiB | Better quality if memory permits |

Use an 8K–16K context instead of the model's maximum advertised context.

### Compact GGUF models under qualification

`granite-4.2-3b:q4_k_m` (~2.09 GiB), `minicpm5-2b:q4_k_m`
(~1.45 GiB), and `spark-x2.5-4b:q4_k_m` (~2.42 GiB) use pinned,
SHA-256-checked official GGUF artifacts. All three default
to an 8K context to avoid allocating their much larger advertised context
windows on a small machine. The 8 GiB system-memory figure is a conservative
test target, **not a measured minimum**; 16 GiB leaves more room for the OS and
other apps. CPU and Apple Silicon Metal chat have been smoke-tested with the
bundled llama.cpp b10964. Windows/Linux Vulkan, long-context behavior, and
structured tool use are not yet qualified, so these are not replacements for
the established starter models above.

To try one on a supported test machine, run
`tapioca pull granite-4.2-3b`, `tapioca pull minicpm5-2b`, or
`tapioca pull spark-x2.5-4b`, then
`tapioca run MODEL`.
Keep `--context 8192` when comparing memory use. The Q4 downloads are pinned
to specific Hugging Face revisions; an invalid cached file is rejected rather
than run.

### Apple Silicon

| Model | Download | Best for |
| --- | ---: | --- |
| `gemma3:12b-mlx` | ~7.6 GiB | Fast conversation |
| `gemma3:27b-mlx` | ~16 GiB | Higher-quality conversation |
| `qwen3:30b-mlx` | ~16 GiB | General use and tools |
| `qwen3-coder:30b-mlx` | ~16 GiB | Coding agents |
| `qwen3.6:35b-mlx` | ~20 GiB | Larger Qwen MLX option |
| `qwen3.8:27b-mlx` | ~15 GiB | Newest Qwen agentic coding and tool use |

On Windows or Linux, `qwen3.8:27b-q4_k_m` is the portable GGUF option and
needs at least 24 GiB of system memory. Qwen3.8 is a reasoning-heavy model;
start with a moderate context size and allow extra time for complex tasks.

For image and video recommendations, use the
[image](image-generation.md) and [video](video-generation.md) guides.
