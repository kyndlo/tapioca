# Files and storage

## Default Tapioca home

- macOS and Linux: `~/.tapioca`
- Windows: `%USERPROFILE%\.tapioca`

The current layout contains:

```text
.tapioca/
├── models/       downloaded model weights and snapshots
├── adapters/     Hugging Face LoRA weights and source metadata
├── recipes/      saved base-model and adapter combinations
├── voices/       reusable voice samples, transcripts, and metadata
├── runtime/      generated Python/MLX runtime environments
├── launch/       isolated coding-agent configuration
└── registry.json installed-model registry
```

Model directories use the resolved name and variant. For example:

```text
~/.tapioca/models/glm-4.7-flash-q8_0/
~/.tapioca/models/flux2-klein-4b-q4-mlx/
```

Snapshot-based models contain the Hugging Face repository files. GGUF models
usually contain one `.gguf` weight file.

Video models with LoRA support can also load manually managed `.safetensors`
files from a model-local directory:

```text
TAPIOCA_HOME/models/RESOLVED-MODEL/loras/
```

Select these files with `tapioca video MODEL --lora FILE[@SCALE]`. The scale
defaults to `1.0`. Managed Hugging Face downloads continue to live under
`TAPIOCA_HOME/adapters` and use `--adapter`.

## Use another disk

Set `TAPIOCA_HOME` before running Tapioca:

```bash
export TAPIOCA_HOME="/Volumes/ExternalSSD/tapioca"
tapioca list
```

PowerShell:

```powershell
$env:TAPIOCA_HOME = "D:\Tapioca"
tapioca list
```

Add the environment variable permanently using the operating system's normal
environment settings if desired.

Release archives do not contain model weights. Leave enough free space for
the download, runtime dependencies, temporary partial downloads, and outputs.

The detailed LoRA storage layout is documented in
[LoRA adapters](../concepts/lora-adapters.md).
