# Command reference

## `catalog`

Show supported models, variants, backends, download sizes, memory, GPU, and
platform requirements:

```bash
tapioca catalog
tapioca catalog update
```

`catalog update` downloads the published JSON manifest and SHA-256 checksum,
validates every repository, backend, variant, and relative artifact path, then
atomically caches it at `~/.tapioca/catalog.json` (or
`%USERPROFILE%\.tapioca\catalog.json` on Windows). A missing or invalid cache
automatically falls back to the catalog compiled into the binary.

The remote catalog can add or update recipes for runtimes the installed
version already understands. A new runtime or file format still requires a
Tapioca software update.

## `update`

Check or install the newest stable CLI bundle from GitHub Releases:

```bash
tapioca update --check
tapioca update
```

Tapioca chooses the archive for the current OS and architecture, streams it to
a temporary directory, verifies the published SHA-256 checksum, rejects unsafe
archive paths, and replaces the executable and bundled runtime. Managed data
under `~/.tapioca` is not changed. Windows completes replacement just after the
current process exits. Desktop checks separately at startup and offers an
Update button when its matching installer is available.

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
tapioca pull GATED_MODEL[:VARIANT] --accept-license
```

Explicit pull is optional for `run`, `serve`, `launch`, `image`, `video`, and `tts`;
they download a missing catalog model automatically.

Gated models do not auto-pull until their terms have been accepted explicitly.
First accept access with the provider, set its access token in the environment,
then use `--accept-license`. Tapioca remembers the acknowledgement locally; it
does not store the provider token.

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
`--steps`, `--seed`, `--random-seed`, repeated `--image`, repeated `--adapter`,
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
`--seconds`, `--seed`, and `--random-seed`.

Image, edit, and video commands use seed `0` by default. Pass `--seed NUMBER`
for an explicit reproducible seed or `--random-seed` to generate and print a
random seed. The two flags cannot be combined. Copy the printed value into
`--seed NUMBER` to reproduce the same configuration later.

`--seconds` requests an approximate duration and selects the closest valid
frame count for the model and FPS. It cannot be combined with `--frames`.
MiniMax-H3 follows `17n+5`, LTX Video follows `8n+1`, and other video models
follow `4n+1`. For example:

```bash
tapioca video minimax-h3 --prompt "A cinematic tracking shot" \
  --seconds 5 --output five-seconds.mp4
```

At 24 FPS this selects 124 MiniMax-H3 frames, approximately 5.17 seconds.
One generation is limited to 513 frames; the nearest valid maximum can be
lower for a model's frame rule (498 for MiniMax-H3). Requests beyond the limit
fail with the maximum approximate duration for the selected model and FPS.
Compose longer videos from multiple short clips.

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

List a provider adapter's task, license, declared base models, revision, and
`.safetensors` files. References may use `hf://`, `civitai://`, `ms://`, or
`local://`:

```bash
tapioca adapter inspect hf://OWNER/REPOSITORY
tapioca adapter inspect civitai://MODEL_ID/VERSION_ID
tapioca adapter inspect ms://OWNER/REPOSITORY
```

## `adapter pull`

Download an adapter:

```bash
tapioca adapter pull hf://OWNER/REPOSITORY --file WEIGHTS.safetensors
tapioca adapter pull civitai://MODEL_ID/VERSION_ID --file WEIGHTS.safetensors
tapioca adapter pull ms://OWNER/REPOSITORY --file WEIGHTS.safetensors
```

The `#FILE` compact form also works.

## `adapter import`

Copy an existing local LoRA into Tapioca's verified, managed adapter store:

```bash
tapioca adapter import ~/Downloads/style.safetensors \
  --base minimax-h3 \
  --name cinematic-style
```

`--base` is required because a `.safetensors` extension alone cannot establish
compatibility. Tapioca rejects symlinks and malformed safetensors files. Use
`--force` only to replace an imported adapter with the same name.

## `adapter list`

Show downloaded/imported adapters, their canonical reusable references,
providers, and exact local paths:

```bash
tapioca adapter list
```

Pass the reference from the first column to `--adapter`. Tapioca reuses the
managed file without another download. Changing `@SCALE` changes strength but
does not duplicate the weights.

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
