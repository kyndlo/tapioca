# Tapioca commands

## Discover

```bash
tapioca version
tapioca list
tapioca catalog
```

Treat catalog output as authoritative for model names, tasks, platforms,
download size, memory, runtime, and GPU requirements.

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

## Diagnose

Retry a failing `run`, `serve`, or `launch` command with `--verbose`. Report the
selected model, backend, platform requirement, and actionable final error.
