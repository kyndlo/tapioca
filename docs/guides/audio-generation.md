# Music and sound generation

Tapioca can generate standalone music, ambience, and sound effects locally as
WAV files. This is separate from text-to-speech and from video models that
produce a synchronized soundtrack.

## Choose a Stable Audio 3 profile

| Model | Best for | Default |
| --- | --- | --- |
| `stable-audio-3:small-music` | Instrumental music, loops, and ambience | 30 seconds, 8 steps |
| `stable-audio-3:small-sfx` | Foley, transitions, impacts, and environmental sounds | 30 seconds, 8 steps |

Both profiles use official Stable Audio 3 Small checkpoints, support up to 120
seconds, and save 44.1 kHz WAV output. Music and SFX are separate downloads, so
install only the profile you need.

## Accept access and download

Stable Audio 3 is gated under the Stability AI Community License. Review and
accept the current terms in your own Hugging Face account, then provide a read
token for the first pull:

```bash
export HF_TOKEN=hf_your_read_token
tapioca pull stable-audio-3:small-music --accept-license
```

On PowerShell:

```powershell
$env:HF_TOKEN = "hf_your_read_token"
tapioca pull stable-audio-3:small-music --accept-license
```

Tapioca records the local license acknowledgement but does not store the token
or accept provider terms on your behalf.

## Generate music

```bash
tapioca audio stable-audio-3:small-music \
  --prompt "Warm analog synthesizer, slow pulse, spacious, instrumental" \
  --negative-prompt "vocals, speech, clipping" \
  --seconds 30 --output ambient.wav
```

## Generate a sound effect

```bash
tapioca audio stable-audio-3:small-sfx \
  --prompt "Heavy wooden door closing in a stone hall, short natural reverb" \
  --seconds 6 --output door.wav
```

Use `--seed NUMBER` to repeat a configuration or `--random-seed` to generate
and print a new seed. The flags cannot be combined. Duration must be between 1
and 120 seconds, and `--steps` must be between 1 and 200.

## Hardware and first run

An NVIDIA CUDA machine is recommended for practical generation. Apple MPS and
CPU fallback are supported by the runtime but can be substantially slower,
especially for long output. The first command creates a private Python runtime
under Tapioca's managed runtime directory and installs the pinned upstream
Stable Audio 3 package; later runs reuse it.

The downloaded checkpoint and runtime are large. Keep ample free disk space,
close other GPU-heavy applications, and begin with a 5–10 second smoke test
before attempting the 120-second maximum.

## Desktop and control API

Tapioca Desktop exposes the same models under **Audio**, with duration, steps,
and seed controls. Control-protocol clients use `audio.generate` with
`model`, `prompt`, optional `negative_prompt`, `duration_seconds`, `steps`,
`seed`, and `output_name` parameters. Outputs are written to Tapioca's managed
audio directory rather than transported as binary protocol data.
