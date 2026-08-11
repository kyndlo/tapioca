# LoRA adapters and model composition

This page is deliberately detailed because LoRAs introduce several new terms
and files.

## What is a LoRA?

A normal image or video model contains the main model weights. A LoRA
(Low-Rank Adaptation) is a much smaller set of additional weights that changes
how a compatible base model behaves.

A LoRA is not usually a complete model. To generate output, Tapioca needs:

1. A **base model**, such as FLUX.2 Klein 4B.
2. A compatible **LoRA adapter**, such as a cinematic-motion style.
3. Input images when the workflow requires them.
4. A prompt and generation settings.

Think of it as:

```text
base model + LoRA adapter + inputs + prompt = output
```

## Read an adapter reference

The compact form is:

```text
hf://creator/cinematic-motion@0.8
│     │         │               │
│     │         │               └─ adapter strength
│     │         └─ Hugging Face repository name
│     └─ Hugging Face account or organization
└─ download from Hugging Face
```

This corresponds to the webpage:

```text
https://huggingface.co/creator/cinematic-motion
```

`@0.8` is not part of the Hugging Face repository name. It tells Tapioca to
apply the adapter at 80% strength. A common starting value is `1.0`; lower
values make the effect subtler.

The readable equivalent is:

```bash
--adapter hf://creator/cinematic-motion --adapter-scale 0.8
```

Beginners should use the readable form until the compact syntax feels
familiar.

Tapioca uses the same shape for every supported provider:

```text
hf://OWNER/REPOSITORY#FILE.safetensors@0.8
civitai://MODEL_ID/VERSION_ID#FILE.safetensors@0.8
ms://OWNER/REPOSITORY#FILE.safetensors@0.8
local://IMPORTED_NAME#FILE.safetensors@0.8
```

`modelscope://` is accepted as a longer alias for `ms://`. You may also paste a
full Civitai model URL when it includes `modelVersionId`; Tapioca converts it to
the stable numeric reference. Civitai checkpoint pages are not LoRAs and are
rejected instead of being treated as adapters.

## Select a file from a repository

A Hugging Face repository can contain multiple LoRA files. For example:

```text
bfs_face_v1_qwen_image_edit_2509.safetensors
bfs_head_v5_2511_original.safetensors
bfs_head_v1_flux-klein_4b.safetensors
```

These files target different base-model families. The `#` syntax selects one:

```text
hf://Alissonerdx/BFS-Best-Face-Swap#bfs_head_v1_flux-klein_4b.safetensors
                                      │
                                      └─ exact file inside the repository
```

Readable form:

```bash
--adapter hf://Alissonerdx/BFS-Best-Face-Swap \
--adapter-file bfs_head_v1_flux-klein_4b.safetensors
```

If a repository has one `.safetensors` file, Tapioca selects it automatically.
If it has several, Tapioca stops and tells you to select one instead of
guessing.

## How a beginner will find and use a LoRA

### 1. Open its provider model card

Read the description and look for:

- Base model or model family
- Required input images and their order
- Recommended prompt
- Recommended adapter strength
- License and usage restrictions
- The exact `.safetensors` weight file

### 2. Choose the matching base model

An adapter trained for FLUX.2 Klein 4B cannot be assumed to work with Qwen
Image Edit, Stable Diffusion, Wan, or another architecture.

Inspect the repository before downloading:

```bash
tapioca adapter inspect hf://Alissonerdx/BFS-Best-Face-Swap
```

It lists the provider, task, license, declared base models, revision, weight
files, checksums, and file sizes when the provider supplies them.

Equivalent provider examples are:

```bash
tapioca adapter inspect civitai://2830065/3193337
tapioca adapter inspect ms://OWNER/REPOSITORY
```

### 3. Pull explicitly or let the task pull automatically

Explicit download:

```bash
tapioca adapter pull \
  hf://Alissonerdx/BFS-Best-Face-Swap \
  --file bfs_head_v1_flux-klein_4b.safetensors
```

### 4. Run the workflow

Image-edit example:

```bash
tapioca edit flux2-klein:4b-q4-mlx \
  --adapter hf://Alissonerdx/BFS-Best-Face-Swap \
  --adapter-file bfs_head_v1_flux-klein_4b.safetensors \
  --adapter-scale 1.0 \
  --image body.png \
  --image face.png \
  --prompt "head swap: use Image 1 as the body and Image 2 as the face" \
  --output result.png
```

Video example:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --adapter hf://creator/cinematic-motion \
  --adapter-scale 0.8 \
  --prompt "A runner crossing a rainy street" \
  --output runner.mp4
```

MiniMax-H3 example with an ordered adapter stack:

```bash
tapioca video minimax-h3 \
  --adapter 'hf://creator/minimax-h3-cinematic#cinematic.safetensors@0.8' \
  --adapter 'hf://creator/minimax-h3-motion#motion.safetensors@0.4' \
  --prompt 'A presenter walks through a cinematic studio' \
  --preset low-memory --output presenter.mp4
```

MiniMax-H3 adapters are applied to its diffusion transformer, not its Qwen3-VL
text encoder. Their order is significant: the second adapter receives the
model already modified by the first.

Adapter support is runtime-specific. Tapioca checks obvious family indicators
in filenames, such as `minimax-h3`, `flux`, `qwen`, `wan`, and `ltx`, before
downloading weights. The selected Tapioca video engine, MLX, MFLUX, or
Diffusers runtime performs the final architecture validation.

## Multiple LoRAs

Repeat `--adapter` to apply more than one:

```bash
tapioca video wan2.2-video:5b-q8-mlx \
  --adapter hf://creator/cinematic-motion@0.8 \
  --adapter hf://creator/consistent-character@0.6 \
  --prompt "Walking through a neon city" \
  --output city.mp4
```

Each adapter remains independently downloadable and independently weighted.

## Import a LoRA already on your computer

Do not point a generation runtime directly at an arbitrary filesystem path.
Import the file once so Tapioca can validate it, hash it, record its declared
base, and give it a portable `local://` reference:

```bash
tapioca adapter import ~/Downloads/cinematic-motion.safetensors \
  --base minimax-h3 \
  --name cinematic-motion
tapioca adapter list
```

PowerShell:

```powershell
tapioca adapter import "$HOME\Downloads\cinematic-motion.safetensors" `
  --base minimax-h3 `
  --name cinematic-motion
```

The resulting reference looks like
`local://cinematic-motion#cinematic-motion.safetensors`. The desktop app's
**Import from computer** button performs this same managed import before the
first generation. Original source files are not modified.

## Reuse an adapter without downloading it again

List the managed library first:

```bash
tapioca adapter list
```

The first column is the canonical reusable reference. Use it unchanged in any
compatible image, edit, or video command:

```bash
tapioca video minimax-h3 \
  --adapter 'local://cinematic-motion#cinematic-motion.safetensors@0.8' \
  --prompt "A cinematic tracking shot" \
  --output reused.mp4
```

This also works for installed `hf://`, `civitai://`, and `ms://` references.
Tapioca resolves the existing managed file before contacting the provider, so
the weights are not downloaded again. The `@0.8` suffix is only the application
strength and can change between runs without creating another copy.

In the desktop app, open **Images** or **Video**, find **LoRA styles**, choose
the adapter from **Installed LoRA**, and select **Assign LoRA**. Pulled provider
adapters and imported local adapters appear in the same list. Their recorded
base families drive the compatibility hint shown next to the selected model.

If a `.safetensors` file was downloaded by a browser or another application,
use `adapter import`; `--file` is only for selecting a file *inside a provider
repository*. To move the library between computers, copy the complete
`TAPIOCA_HOME/adapters` directory, including `snapshot.json` files. Alternatively,
import each raw file again with the correct `--base` value.

## Where files will be stored

By default, Tapioca data lives under:

```text
macOS:  ~/.tapioca
Windows: %USERPROFILE%\.tapioca
```

The adapter layout is:

```text
.tapioca/
├── models/
│   └── flux2-klein-4b-q4-mlx/       base model snapshot
├── adapters/
│   ├── huggingface/Alissonerdx/BFS-Best-Face-Swap/
│   │   ├── snapshot.json             source revision and metadata
│   │   └── bfs_head_v1_flux-klein_4b.safetensors
│   ├── civitai/2830065/3193337/      Civitai model/version IDs
│   ├── modelscope/OWNER/REPOSITORY/  ModelScope repository
│   └── local/cinematic-motion/       verified local imports
├── recipes/                          saved base + adapter combinations
└── runtime/                          Managed video, MLX, MFLUX, or Diffusers environments
```

Setting `TAPIOCA_HOME` moves all of these directories together:

```bash
export TAPIOCA_HOME="/Volumes/ExternalSSD/tapioca"
```

PowerShell:

```powershell
$env:TAPIOCA_HOME = "D:\Tapioca"
```

## Saved recipes

Long commands are optional. `create` saves a reusable composition:

```bash
tapioca create my-face-swap \
  --base flux2-klein:4b-q4-mlx \
  --adapter hf://Alissonerdx/BFS-Best-Face-Swap#bfs_head_v1_flux-klein_4b.safetensors
```

Then:

```bash
tapioca edit my-face-swap \
  --image body.png \
  --image face.png \
  --prompt "head swap" \
  --output result.png
```

Recipes should store references and settings, not duplicate model weights.

## Private or gated providers

Set a Hugging Face access token before inspecting or pulling a private or
gated base-model or adapter repository:

```bash
export HF_TOKEN="your-token"
```

PowerShell:

```powershell
$env:HF_TOKEN = "your-token"
```

Tapioca also recognizes `HUGGING_FACE_HUB_TOKEN`. Do not put a token in a
recipe, adapter reference, shell history, or Git repository.

For private Civitai or ModelScope content, set `CIVITAI_TOKEN` or
`MODELSCOPE_API_TOKEN` respectively. Tokens are read from the environment and
are never written into references or snapshot metadata.

## What happens during automatic download?

When an image, edit, or video command includes `--adapter`, Tapioca:

1. Reads metadata from Hugging Face, Civitai, ModelScope, or the managed local store.
2. Selects the requested `.safetensors` file.
3. Checks obvious compatibility indicators against the base model.
4. Reuses the local file when it is already cached.
5. Otherwise downloads it into `TAPIOCA_HOME/adapters`.
6. Passes the local path and scale to the selected runtime.

Use `tapioca adapter pull` when you want to download before going offline.

See downloaded adapters and their exact filesystem paths:

```bash
tapioca adapter list
```

## Safety and consent

Only use a person's face or identity with their permission. Clearly label
synthetic or altered media when it could be mistaken for an authentic image
or recording. Review the base model and adapter licenses before distributing
outputs commercially.
