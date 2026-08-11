# Import existing LoRAs and transfer model files

You do not need to download large weights again just because they were
downloaded outside Tapioca or because you are moving to another computer.
Use the workflow that matches what you already have.

## Import one LoRA file

Tapioca imports LoRAs in the `.safetensors` format. The `--base` value is
required because the filename and extension do not prove which model family
the LoRA supports.

macOS or Linux:

```bash
tapioca adapter import ~/Downloads/cinematic-motion.safetensors \
  --base minimax-h3 \
  --name cinematic-motion
```

Windows PowerShell:

```powershell
tapioca adapter import "C:\Users\Carlos\Downloads\cinematic-motion.safetensors" `
  --base minimax-h3 `
  --name cinematic-motion
```

Tapioca validates the safetensors header, calculates a SHA-256 hash, copies the
file into its managed adapter library, and returns a reusable `local://`
reference. The original file is not changed or removed.

Confirm the import:

```bash
tapioca adapter list
```

Then use the exact reference from the first column:

```bash
tapioca video minimax-h3 \
  --adapter 'local://cinematic-motion#cinematic-motion.safetensors@0.8' \
  --prompt "A cinematic tracking shot" \
  --output adapted.mp4
```

In Tapioca Desktop, open **Images** or **Video**, find **LoRA styles**, and
choose **Import from computer**. Select the `.safetensors` file, enter the base
model family, and choose **Import LoRA**. It will then appear under **Installed
LoRA**.

`--file` is not a local-file option. It selects one weight file inside a
Hugging Face, Civitai, or ModelScope repository. Use `adapter import` for a file
that already exists on the computer.

## Move all LoRAs to another computer

Tapioca stores adapters under:

```text
macOS/Linux: ~/.tapioca/adapters
Windows:     %USERPROFILE%\.tapioca\adapters
```

1. Quit Tapioca on both computers.
2. Copy the complete `adapters` directory from the old computer.
3. Place it inside the new computer's Tapioca home.
4. Keep every `snapshot.json` file next to its weights.
5. Run `tapioca adapter list` on the new computer.

The snapshot files preserve the canonical reference, source, checksum,
revision, and declared base-model families. Do not copy only the weight files
from Tapioca's managed directory. If all you have is a raw `.safetensors` file,
import it again with `adapter import`.

## Transfer an installed base model

Base models are platform and backend specific. Only transfer a variant that is
compatible with the destination computer. For example, an Apple Silicon MLX
variant is not a Windows CUDA model.

1. On the source computer, run `tapioca list` and note the exact model ID.
2. Quit Tapioca on both computers.
3. Copy that model's complete directory from `TAPIOCA_HOME/models`.
4. Place it in the destination computer's `TAPIOCA_HOME/models` directory with
   the same directory name.
5. On the destination computer, run:

```bash
tapioca pull MODEL
```

Do not add `--force`. Tapioca reuses files that are already present, downloads
only required files that are missing, and registers the model using paths that
are correct for the new computer.

For example, after copying the `minimax-h3` directory:

```bash
tapioca pull minimax-h3
tapioca list
```

Do not copy `registry.json` by itself between computers. It contains absolute
paths from the old machine. Let `tapioca pull MODEL` rebuild the registration.

## Move Tapioca to an external drive

Set `TAPIOCA_HOME` before importing or pulling models so future weights and
runtimes use the external disk.

macOS or Linux:

```bash
export TAPIOCA_HOME="/Volumes/ExternalSSD/tapioca"
tapioca adapter list
tapioca list
```

Windows PowerShell:

```powershell
$env:TAPIOCA_HOME = "D:\Tapioca"
tapioca adapter list
tapioca list
```

Set the environment variable permanently in the operating system if every
Tapioca session should use that location. Keep enough free space for model
weights, managed runtimes, partial downloads, and generated media.

## Verify before deleting the old copy

Run these checks on the destination computer:

```bash
tapioca adapter list
tapioca list
tapioca run MODEL
```

For image or video models, make one small smoke-test output. Delete the old
copy only after the destination can load the model and the generated output is
valid.
