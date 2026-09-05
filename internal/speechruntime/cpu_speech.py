"""Local CPU speech adapters. No network access is needed during inference."""
import json
from pathlib import Path
import shutil
import tempfile
import time

from pocket_qualification import write_pcm_stream


def audio8(args):
    if args.language and args.language.lower() not in {"auto", "en", "english", "zh", "chinese", "ja", "japanese", "es", "spanish", "de", "german", "fr", "french", "it", "italian", "ko", "korean", "ru", "russian", "pt", "portuguese", "ar", "arabic"}:
        raise ValueError("Unsupported Audio8 language; language is inferred from the input text")
    if args.voice_sample and (not args.voice_consent or not args.transcript.strip()):
        raise ValueError("Audio8 voice cloning requires permission and the exact reference transcript")
    import numpy as np
    from arktts_runtime.runtime import ArkTtsRuntime
    from arktts_runtime.registration import VoiceRegistration

    root = Path(args.model).resolve()
    output = Path(args.output).resolve()
    manifest = json.loads((root / "runtime_manifest.json").read_text(encoding="utf-8"))
    if manifest["sample_rate"] != 44100:
        raise ValueError("Audio8 requires its pinned 44.1 kHz model export")
    output.parent.mkdir(parents=True, exist_ok=True)
    started = time.monotonic()
    # Temporary voices isolate simultaneous requests and never persist reference audio.
    with tempfile.TemporaryDirectory(prefix="tapioca-audio8-") as temporary:
        voices = Path(temporary) / "voices"
        voice = voices / "default"
        voice.mkdir(parents=True)
        shutil.copyfile(root / "reference_codes.npy", voice / "codes.npy")
        (voice / "meta.json").write_text(json.dumps({"reference_text": manifest["reference_text"]}), encoding="utf-8")
        selected = "default"
        if args.voice_sample:
            reference = Path(args.voice_sample)
            if reference.stat().st_size > 50 * 1024 * 1024:
                raise ValueError("Reference recording exceeds 50 MiB")
            registration = VoiceRegistration(root / "registration", voices, manifest["model_fingerprint"])
            registration.register(reference.read_bytes(), reference.name, args.transcript, "reference")
            selected = "reference"
        runtime = ArkTtsRuntime(root, voices, "int8", "fp16", 5)
        def chunks():
            for event in runtime.stream(text=args.text, voice=selected, seed=args.seed):
                if event["type"] == "audio_chunk":
                    yield np.clip(event["audio"] * 32767, -32768, 32767).astype("<i2").tobytes()
        with tempfile.NamedTemporaryFile(dir=output.parent, suffix=".wav", delete=False) as file:
            partial = Path(file.name)
        try:
            metrics = write_pcm_stream(chunks(), partial, 44100, started)
            partial.replace(output)
        finally:
            partial.unlink(missing_ok=True)
    print(json.dumps(metrics))
