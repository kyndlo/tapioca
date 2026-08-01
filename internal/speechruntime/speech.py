import argparse
import glob
import os
import shutil
import tempfile


def device_for_torch(torch):
    if torch.cuda.is_available():
        return "cuda"
    if hasattr(torch.backends, "mps") and torch.backends.mps.is_available():
        return "mps"
    return "cpu"


def chatterbox(args):
    import torch
    import torchaudio

    device = device_for_torch(torch)
    if args.model_name.endswith(":nano"):
        if not args.voice_sample:
            raise SystemExit(
                "chatterbox:nano requires --voice-sample or --voice because it is a voice-cloning model"
            )
        from chatterbox.tts_turbo import ChatterboxTurboTTS

        model = ChatterboxTurboTTS.from_local(args.model, device=device, nano=True)
        wav = model.generate(args.text, audio_prompt_path=args.voice_sample)
    else:
        from chatterbox.mtl_tts import ChatterboxMultilingualTTS

        model = ChatterboxMultilingualTTS.from_local(
            args.model, device=device, t3_model="v3"
        )
        options = {"language_id": args.language or "en"}
        if args.voice_sample:
            options["audio_prompt_path"] = args.voice_sample
        wav = model.generate(args.text, **options)
    torchaudio.save(args.output, wav.cpu(), model.sr)


def qwen(args):
    import soundfile as sf
    import torch
    from qwen_tts import Qwen3TTSModel

    if not args.voice_sample:
        raise SystemExit(
            "qwen3-tts Base requires --voice-sample or --voice for voice cloning"
        )
    device = "cuda:0" if torch.cuda.is_available() else "cpu"
    dtype = torch.bfloat16 if torch.cuda.is_available() and torch.cuda.is_bf16_supported() else torch.float32
    model = Qwen3TTSModel.from_pretrained(
        args.model,
        device_map=device,
        dtype=dtype,
        attn_implementation="sdpa",
        local_files_only=True,
    )
    wavs, sample_rate = model.generate_voice_clone(
        text=args.text,
        language=args.language or "Auto",
        ref_audio=args.voice_sample,
        ref_text=args.transcript or None,
        x_vector_only_mode=not bool(args.transcript),
    )
    sf.write(args.output, wavs[0], sample_rate)


def qwen_mlx(args):
    if not args.voice_sample:
        raise SystemExit(
            "qwen3-tts Base requires --voice-sample or --voice for voice cloning"
        )
    from mlx_audio.tts.generate import generate_audio
    from mlx_audio.tts.utils import load_model

    model = load_model(args.model)
    with tempfile.TemporaryDirectory(prefix="tapioca-speech-") as output_dir:
        generate_audio(
            model=model,
            text=args.text,
            ref_audio=args.voice_sample,
            ref_text=args.transcript or None,
            output_path=output_dir,
            file_prefix="speech",
            audio_format="wav",
            join_audio=True,
            play=False,
            verbose=False,
        )
        candidates = glob.glob(os.path.join(output_dir, "*.wav"))
        if not candidates:
            raise SystemExit("MLX Audio did not produce a WAV file")
        shutil.move(candidates[0], args.output)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--model-name", required=True)
    parser.add_argument("--backend", required=True)
    parser.add_argument("--text", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--voice-sample")
    parser.add_argument("--transcript", default="")
    parser.add_argument("--language", default="")
    args = parser.parse_args()

    if os.path.splitext(args.output)[1].lower() != ".wav":
        raise SystemExit("the first speech release supports WAV output; use a .wav filename")
    if args.backend == "speech-chatterbox":
        chatterbox(args)
    elif args.backend == "speech-qwen":
        qwen(args)
    elif args.backend == "speech-qwen-mlx":
        qwen_mlx(args)
    else:
        raise SystemExit(f"unsupported speech backend: {args.backend}")


if __name__ == "__main__":
    main()
