# Tapioca commands

## Discover

```bash
tapioca version
tapioca list
tapioca catalog update
tapioca catalog
```

Treat catalog output as authoritative for model names, tasks, platforms,
download size, memory, runtime, and GPU requirements. Refreshing downloads a
small checksummed catalog and safely retains the built-in catalog if validation
or the network fails.

## Update Tapioca

```bash
tapioca update --check
tapioca update
```

The check is read-only. Installing an update downloads the matching verified
GitHub Release and replaces Tapioca without deleting models, imported LoRAs,
voices, or generated media. Agents must obtain user approval before installing
an application update.

## Acquire and run

```bash
tapioca pull MODEL
tapioca run MODEL
tapioca serve MODEL --host 127.0.0.1 --port 11435
```

Missing catalog models are pulled automatically by execution commands, but
explicit `pull` is preferable when the user should see a large download before
inference starts.

## Coding agents

```bash
tapioca launch codex MODEL
tapioca launch claude MODEL
tapioca launch opencode MODEL
tapioca launch openclaw MODEL
tapioca launch hermes MODEL
```

The client must already be on `PATH`. Pass client arguments after `--`.
Tapioca stores isolated launch configuration under `TAPIOCA_HOME/launch`.

Prefer a coder or tool-oriented model. Tool quality depends on the model,
quantization, chat template, and runtime support.

## Adapters

```bash
tapioca adapter inspect hf://OWNER/REPOSITORY
tapioca adapter pull hf://OWNER/REPOSITORY --file adapter.safetensors
tapioca adapter list
```

Use `#FILE` to select a repository file and `@SCALE` to set its weight inside
an adapter reference: `hf://OWNER/REPOSITORY#adapter.safetensors@0.8`.
Repeated `--adapter` flags are ordered. Never infer compatibility solely from
the `.safetensors` extension.

## Diagnose

Retry a failing `run`, `serve`, or `launch` command with `--verbose`. Report the
selected model, backend, platform requirement, and actionable final error.
