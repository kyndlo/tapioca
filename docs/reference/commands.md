# Command reference

## `catalog`

Show supported models, variants, backends, download sizes, memory, GPU, and
platform requirements:

```bash
tapioca catalog
```

## `list`

Show locally installed models:

```bash
tapioca list
```

## `pull`

Download a catalog model explicitly:

```bash
tapioca pull MODEL[:VARIANT]
tapioca pull MODEL[:VARIANT] --force
```

Explicit pull is optional for `run`, `serve`, `launch`, `image`, `video`, and `tts`;
they download a missing catalog model automatically.

## `run`

Open an interactive chat:

```bash
tapioca run MODEL [--context TOKENS] [--show-thinking=false] [--verbose]
```

Enter `/bye` or press Ctrl-D to exit.

## `serve`

Run compatible local HTTP APIs:

```bash
tapioca serve MODEL [--port 11435] [--context TOKENS] [--verbose]
```

## `image`

```bash
tapioca image MODEL --prompt TEXT [flags]
```

Flags include `--negative-prompt`, `--output`, `--width`, `--height`,
`--steps`, `--seed`, repeated `--image`, repeated `--adapter`,
`--adapter-file`, and `--adapter-scale`. Dimensions must be divisible by 16.

## `edit`

Edit using one or more ordered input images:

```bash
tapioca edit MODEL --image FILE [--image FILE] --prompt TEXT [flags]
```

The adapter flags are the same as `image`.

## `video`

```bash
tapioca video MODEL --prompt TEXT [flags]
```

Flags include `--image`, `--negative-prompt`, `--output`, `--preset`,
`--enhance-prompt`, `--width`, `--height`, `--frames`, `--steps`, `--fps`,
and `--seed`.

Video additionally supports repeated `--adapter`, `--adapter-file`, and
`--adapter-scale` when the selected backend supports LoRAs. MiniMax-H3 uses a
managed ComfyUI engine, generates native audio, and accepts compatible
transformer LoRA stacks:

```bash
tapioca video minimax-h3 --prompt 'A musician says: "Ready."' \
  --adapter 'hf://creator/minimax-h3-style@0.7' \
  --preset low-memory --output musician.mp4
```

## `tts`

Generate a WAV file from text:

```bash
tapioca tts MODEL --text TEXT [flags]
```

Flags include `--output`, `--language`, `--voice`, `--voice-sample`,
`--transcript`, and `--transcript-file`.

## `voice`

Save and reuse reference voices:

```bash
tapioca voice create NAME --model MODEL --audio FILE \
  [--transcript TEXT | --transcript-file FILE]
tapioca voice list
tapioca voice inspect NAME
tapioca voice remove NAME
```

## `adapter inspect`

List a Hugging Face adapter repository's task, license, declared base models,
revision, and `.safetensors` files:

```bash
tapioca adapter inspect hf://OWNER/REPOSITORY
```

## `adapter pull`

Download an adapter:

```bash
tapioca adapter pull hf://OWNER/REPOSITORY --file WEIGHTS.safetensors
```

The `#FILE` compact form also works.

## `adapter list`

Show downloaded adapters and their exact local paths:

```bash
tapioca adapter list
```

## `create`

Save a base model, repeated adapters, and an optional preset as a recipe:

```bash
tapioca create NAME \
  --base MODEL \
  [--adapter REFERENCE] \
  [--preset PRESET]
```

## `launch`

```bash
tapioca launch (codex|claude|opencode|openclaw|hermes) MODEL \
  [-- CLIENT_ARGS...]
```

The client must already be installed.

Image and video base models may be catalog names or compatible direct
`hf://OWNER/REPOSITORY` references.
