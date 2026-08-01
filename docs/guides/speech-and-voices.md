# Text to speech and voice cloning

Tapioca can generate local speech on macOS, Windows, and Linux. Model weights,
reference recordings, and generated audio stay on your computer.

Only clone a voice you own or have permission to use. Chatterbox outputs
contain the model's built-in imperceptible audio watermark.

## Choose a model

| Model | Best for | Hardware |
| --- | --- | --- |
| `chatterbox:nano` | Fast English cloning with a short reference clip | CPU, Apple MPS, or NVIDIA CUDA |
| `chatterbox:multilingual` | Default voices and cloning across 23+ languages | CPU, Apple MPS, or NVIDIA CUDA |
| `qwen3-tts:0.6b-mlx` | High-quality 3-second cloning on Apple Silicon | Mac with 12 GiB+ unified memory |
| `qwen3-tts:0.6b` | High-quality cloning on GPU servers | Windows/Linux NVIDIA; CPU fallback |

Running a missing model automatically downloads it. To download first:

```bash
tapioca pull chatterbox:multilingual
```

Approximate runtime needs vary with text length and backend. Start with at
least 8 GiB free memory for Chatterbox Nano and 12 GiB for the 0.6B Qwen
variant. Close memory-heavy applications for long passages. The catalog is the
source of truth for each downloadable variant:

```bash
tapioca catalog
```

## Generate speech with a built-in voice

```bash
tapioca tts chatterbox:multilingual \
  --language en \
  --text "Hello from Tapioca." \
  --output hello.wav
```

Use language codes such as `en`, `es`, `fr`, `de`, `pt`, `ja`, `ko`, or `zh`.

## Record a useful voice sample

For cloning, record 3–10 seconds in a quiet room:

1. Use one speaker.
2. Avoid music, reverb, and background noise.
3. Speak naturally at a steady volume.
4. Prefer WAV, although common audio formats also work.
5. Write the exact words spoken in the recording.

An accurate transcript improves Qwen voice similarity. Chatterbox does not
require the transcript, but Tapioca saves it so the voice can move between
compatible backends.

## Save a reusable voice

```bash
tapioca voice create carlos \
  --model qwen3-tts \
  --audio ./carlos-reference.wav \
  --transcript "Hello, this is Carlos recording a short voice sample."
```

For a longer transcript:

```bash
tapioca voice create narrator \
  --model chatterbox:nano \
  --audio ./narrator.wav \
  --transcript-file ./narrator.txt
```

Inspect saved voices:

```bash
tapioca voice list
tapioca voice inspect carlos
```

## Speak with the saved voice

Apple Silicon automatically selects Qwen's MLX variant:

```bash
tapioca tts qwen3-tts \
  --voice carlos \
  --language English \
  --text "This voice was generated locally on my Mac." \
  --output carlos.wav
```

Portable Chatterbox example:

```bash
tapioca tts chatterbox:nano \
  --voice narrator \
  --text "The local runtime is ready." \
  --output narrator.wav
```

You can also clone without saving a voice:

```bash
tapioca tts qwen3-tts:0.6b \
  --voice-sample ./reference.wav \
  --transcript-file ./reference.txt \
  --text "One-off cloning works too." \
  --output one-off.wav
```

## Files on disk

Voices are stored under `~/.tapioca/voices` on macOS/Linux and
`%USERPROFILE%\.tapioca\voices` on Windows:

```text
voices/
└── carlos/
    ├── reference.wav
    └── voice.json
```

Remove a saved voice and its copied reference recording with:

```bash
tapioca voice remove carlos
```

## First-run expectations

The first speech command creates an isolated Python environment and installs
the selected backend. This is separate from the model download and can take a
few minutes. Later runs reuse both the runtime and model.

## Platform and acceleration matrix

| Platform | Chatterbox | Qwen3-TTS |
| --- | --- | --- |
| Apple Silicon | PyTorch MPS or CPU | MLX recommended |
| Windows x64 NVIDIA | CUDA or CPU | CUDA or CPU |
| Windows x64 AMD/Intel | CPU | CPU |
| Windows ARM64 | CPU | CPU |
| Linux NVIDIA | CUDA or CPU | CUDA or CPU |
| Linux without NVIDIA | CPU | CPU |

MLX is selected automatically when you request `qwen3-tts` on Apple Silicon.
On Windows or Linux NVIDIA, use `qwen3-tts:0.6b`.

## Troubleshooting quality

If similarity is weak, use a clean WAV recording, remove leading and trailing
silence, and provide the exact transcript for Qwen3-TTS. A natural 3–10 second
sample usually works better than a dramatic or whispered recording.

If generation is unexpectedly slow, remember that the first run also prepares
the runtime. Run a second short sentence before judging steady-state speed.
See [Troubleshooting](../reference/troubleshooting.md) for GPU checks and
memory advice.
