"""Experimental, offline-only CPU qualification for Sopro V2 Turbo.

This runner requires a consented reference recording. It is not a public
catalog backend until voice quality and additional platforms are qualified.
"""

import json
import os
from pathlib import Path
import tempfile
import time


ARTIFACTS = (
    "config.json",
    "model.safetensors",
    "semantic_encoder.safetensors",
    "speaker_encoder.safetensors",
    "tokenizer.model",
    "vocoder.safetensors",
)
LANGUAGES = {"", "en", "pt", "fr", "de"}


def validate_inputs(args):
    if not args.text.strip():
        raise ValueError("Sopro text must not be empty")
    if args.language not in LANGUAGES:
        raise ValueError("Sopro language must be en, pt, fr, or de")
    if not args.voice_sample or not args.voice_consent:
        raise ValueError("Sopro requires a reference recording and explicit voice-use permission")
    root = Path(args.model).resolve()
    for name in ARTIFACTS:
        item = root / name
        if not item.is_file() or item.stat().st_size == 0:
            raise ValueError(f"Missing Sopro artifact: {item}")
    sample = Path(args.voice_sample).resolve()
    if not sample.is_file() or sample.stat().st_size == 0:
        raise ValueError(f"Missing or empty Sopro reference recording: {sample}")
    if sample.stat().st_size > 50 * 1024 * 1024:
        raise ValueError("Sopro reference recording exceeds 50 MiB")
    output = Path(args.output).resolve()
    if output.suffix.lower() != ".wav":
        raise ValueError("Sopro output must be a WAV file")
    return root, sample, output


def run(args):
    root, sample, output = validate_inputs(args)
    os.environ["HF_HUB_OFFLINE"] = "1"
    os.environ["TRANSFORMERS_OFFLINE"] = "1"
    import torch
    import soundfile
    from sopro import SoproTTS
    from pocket_qualification import write_pcm_stream

    info = soundfile.info(str(sample))
    seconds = info.frames / info.samplerate
    if not 5 <= seconds <= 20:
        raise ValueError("Sopro reference recording must be 5–20 seconds")
    torch.manual_seed(args.seed)
    torch.set_num_threads(4)
    started = time.monotonic()
    model = SoproTTS.from_pretrained(root, device="cpu")
    chunks = (
        chunk.detach().cpu().clamp(-1, 1).mul(32767)
        .to(torch.int16).numpy().astype("<i2").tobytes()
        for chunk in model.stream(args.text, ref_audio_path=sample, lang=args.language or None)
    )
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=output.parent, suffix=".wav", delete=False) as file:
        partial = Path(file.name)
    try:
        metrics = write_pcm_stream(chunks, partial, model.sample_rate, started)
        partial.replace(output)
    finally:
        partial.unlink(missing_ok=True)
    print(json.dumps(metrics))
