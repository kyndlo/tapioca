"""Experimental Pocket TTS qualification runner; not a public catalog backend.

Use an explicitly downloaded, gated English model bundle. All model and voice
paths are local, and Hugging Face offline mode is set before runtime imports.
"""

import argparse
import json
import os
from pathlib import Path
import tempfile
import time
import wave


def local_inputs(root, reference=None, consent=False):
    root = Path(root).resolve()
    paths = {
        "weights": root / "languages/english/model.safetensors",
        "tokenizer": root / "languages/english/tokenizer.model",
        "voice": Path(reference).resolve() if reference else root / "languages/english/embeddings/alba.safetensors",
    }
    if reference and not consent:
        raise ValueError("Confirm permission to use this voice with --voice-consent.")
    for name, path in paths.items():
        if not path.is_file() or path.stat().st_size == 0:
            raise ValueError(f"Missing or empty {name}: {path}. Download the pinned bundle after accepting its Hugging Face terms.")
    return paths


def write_pcm_stream(chunks, output, sample_rate, started=None, clock=time.monotonic):
    """Consume one PCM16 chunk at a time; never buffer the whole utterance."""
    started = clock() if started is None else started
    first_audio = None
    samples = 0
    with wave.open(str(output), "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        for pcm in chunks:
            if len(pcm) % 2:
                raise ValueError("PCM16 chunks must contain complete samples")
            if not pcm:
                continue
            if first_audio is None:
                first_audio = clock() - started
            wav.writeframes(pcm)
            samples += len(pcm) // 2
    if not samples:
        raise ValueError("Pocket TTS produced no audio")
    elapsed = clock() - started
    duration = samples / sample_rate
    return {"first_audio_seconds": first_audio, "elapsed_seconds": elapsed,
            "audio_seconds": duration, "realtime_factor": elapsed / duration,
            "sample_rate": sample_rate, "samples": samples}


def run(args):
    if args.language.lower() not in ("en", "english"):
        raise ValueError("This qualification runner supports English only")
    if not args.text.strip():
        raise ValueError("Text must not be empty")
    output = Path(args.output).resolve()
    if output.suffix.lower() != ".wav":
        raise ValueError("Use a .wav output filename")
    inputs = local_inputs(args.model, args.voice_sample, args.voice_consent)
    os.environ["HF_HUB_OFFLINE"] = "1"
    os.environ["TRANSFORMERS_OFFLINE"] = "1"
    import yaml
    import torch
    from pocket_tts import TTSModel
    from pocket_tts.utils.config import CONFIGS_DIR

    torch.manual_seed(args.seed)
    torch.set_num_threads(2)
    with open(CONFIGS_DIR / "english.yaml", encoding="utf-8") as file:
        config = yaml.safe_load(file)
    config["weights_path"] = str(inputs["weights"])
    # Upstream's fallback must never substitute a different checkpoint.
    config["weights_path_without_voice_cloning"] = str(inputs["weights"])
    config["flow_lm"]["lookup_table"]["tokenizer_path"] = str(inputs["tokenizer"])
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="tapioca-pocket-") as temporary:
        config_path = Path(temporary) / "english.yaml"
        with config_path.open("w", encoding="utf-8") as file:
            yaml.safe_dump(config, file)
        started = time.monotonic()
        model = TTSModel.load_model(config=config_path)
        voice = model.get_state_for_audio_prompt(inputs["voice"])
        chunks = model.generate_audio_stream(voice, args.text)
        pcm = (chunk.detach().cpu().clamp(-1, 1).mul(32767).to(torch.int16).numpy().astype("<i2").tobytes() for chunk in chunks)
        # Publish the final file only after generation and WAV finalization succeed.
        with tempfile.NamedTemporaryFile(dir=output.parent, suffix=".wav", delete=False) as file:
            partial = Path(file.name)
        try:
            metrics = write_pcm_stream(pcm, partial, model.sample_rate, started)
            partial.replace(output)
        finally:
            partial.unlink(missing_ok=True)
    print(json.dumps(metrics))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model", required=True)
    parser.add_argument("--text", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--language", default="en")
    parser.add_argument("--voice-sample")
    parser.add_argument("--voice-consent", action="store_true")
    parser.add_argument("--seed", type=int, default=0)
    try:
        run(parser.parse_args())
    except (ValueError, OSError) as error:
        parser.exit(1, f"{error}\n")


if __name__ == "__main__":
    main()
