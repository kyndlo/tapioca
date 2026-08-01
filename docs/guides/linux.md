# Run Tapioca on Linux

Tapioca supports Linux x64 and ARM64 for local workstations, headless machines,
and GPU servers. The release bundle includes Tapioca and the matching
llama.cpp runtime.

## 1. Install

```bash
curl -fsSL https://raw.githubusercontent.com/kyndlo/tapioca/main/scripts/install.sh | sh
```

Open a new shell, then verify:

```bash
tapioca version
tapioca catalog
```

The installer selects x64 or ARM64 automatically. To install a specific
release, set `TAPIOCA_VERSION`:

```bash
TAPIOCA_VERSION=v0.4.0 \
  curl -fsSL https://raw.githubusercontent.com/kyndlo/tapioca/main/scripts/install.sh | sh
```

## 2. Check the hardware backend

GGUF text models use Vulkan. Install your distribution's current GPU driver,
Vulkan loader, and Vulkan utilities, then check:

```bash
vulkaninfo --summary
```

Image generation, video generation, and GPU-accelerated speech use NVIDIA
CUDA on Linux. Check the driver:

```bash
nvidia-smi
```

No NVIDIA GPU is required for text models. Speech backends can fall back to
CPU, but generation will take longer.

## 3. Run a first text model

For a machine with approximately 12 GiB of memory:

```bash
tapioca run qwen3:4b-q4_k_m --context 8192
```

The first run pulls the model automatically. Type `/bye` to stop the chat.

To serve the OpenAI-compatible API instead:

```bash
tapioca serve qwen3:4b-q4_k_m
```

By default, Tapioca listens on localhost. Keep it that way unless you add
authentication and a firewall or trusted private network in front of it.

## 4. Generate media on NVIDIA

Use the catalog to see CUDA requirements and memory guidance:

```bash
tapioca catalog
```

Then try:

```bash
tapioca image sd-turbo:fp16 \
  --prompt "A red fox in soft snow" \
  --output fox.png
```

For video, start with the catalog's lightest CUDA model and a low-memory preset:

```bash
tapioca video ltx-video:2b-fp16 \
  --prompt "Clouds moving over a quiet mountain" \
  --preset low-memory \
  --output clouds.mp4
```

## 5. Generate speech

Chatterbox Nano is a practical first test:

```bash
tapioca tts chatterbox:nano \
  --text "This Linux server can speak locally." \
  --output hello.wav
```

For Qwen3-TTS acceleration, use a Linux NVIDIA machine and
`qwen3-tts:0.6b`. See [Text to speech and voice cloning](speech-and-voices.md)
for reusable voices and recording advice.

## 6. Use a coding agent

Tapioca can launch a supported coding agent against its local API:

```bash
tapioca launch opencode qwen3-coder:30b-q4_k_m
```

Confirm the selected model fits your RAM or VRAM before pulling a large
variant. [Choosing models](choosing-models.md) explains the catalog columns.

## Files and automation

Linux state lives under `~/.tapioca`:

```text
~/.tapioca/
├── models/
├── runtimes/
├── voices/
└── recipes/
```

For unattended provisioning, pin `TAPIOCA_VERSION`, run `tapioca pull MODEL`
during image creation, and keep the models directory on persistent storage.
Use `tapioca serve` under your process supervisor only after the model has been
pulled and a manual health check succeeds.
