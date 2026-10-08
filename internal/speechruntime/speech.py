import argparse
import glob
import os
import shutil
import tempfile


def load_chatterbox_nano(model_path, device):
    """Load Nano with released chatterbox packages that predate its loader flag."""
    import inspect
    import torch
    from pathlib import Path
    from safetensors.torch import load_file
    from transformers import AutoTokenizer
    from chatterbox.tts_turbo import ChatterboxTurboTTS, Conditionals

    if "nano" in inspect.signature(ChatterboxTurboTTS.from_local).parameters:
        return ChatterboxTurboTTS.from_local(model_path, device=device, nano=True)

    from chatterbox.models.s3gen import S3Gen
    from chatterbox.models.t3 import T3
    from chatterbox.models.t3.llama_configs import LLAMA_CONFIGS
    from chatterbox.models.t3.modules.t3_config import T3Config
    from chatterbox.models.voice_encoder import VoiceEncoder

    LLAMA_CONFIGS.setdefault(
        "GPT2_small",
        {
            "activation_function": "gelu_new",
            "architectures": ["GPT2LMHeadModel"],
            "attn_pdrop": 0.1,
            "bos_token_id": 50256,
            "embd_pdrop": 0.1,
            "eos_token_id": 50256,
            "initializer_range": 0.02,
            "layer_norm_epsilon": 1e-05,
            "model_type": "gpt2",
            "n_ctx": 8196,
            "n_embd": 768,
            "hidden_size": 768,
            "n_head": 12,
            "n_layer": 12,
            "n_positions": 8196,
            "n_special": 0,
            "predict_special_tokens": True,
            "resid_pdrop": 0.1,
            "summary_activation": None,
            "summary_first_dropout": 0.1,
            "summary_proj_to_labels": True,
            "summary_type": "cls_index",
            "summary_use_proj": True,
            "task_specific_params": {
                "text-generation": {"do_sample": True, "max_length": 50}
            },
            "vocab_size": 50276,
        },
    )
    path = Path(model_path)
    map_location = torch.device("cpu") if device in ("cpu", "mps") else None
    voice_encoder = VoiceEncoder()
    voice_encoder.load_state_dict(load_file(path / "ve.safetensors"))
    voice_encoder.to(device).eval()

    config = T3Config(text_tokens_dict_size=50276)
    config.llama_config_name = "GPT2_small"
    config.speech_tokens_dict_size = 6563
    config.input_pos_emb = None
    config.speech_cond_prompt_len = 375
    config.use_perceiver_resampler = False
    config.emotion_adv = False
    t3 = T3(config)
    t3.load_state_dict(load_file(path / "t3_nano_v1.safetensors"))
    del t3.tfmr.wte
    t3.to(device).eval()

    s3gen = S3Gen(meanflow=True)
    s3gen.load_state_dict(
        load_file(path / "s3gen_meanflow.safetensors"), strict=True
    )
    s3gen.to(device).eval()
    tokenizer = AutoTokenizer.from_pretrained(path)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    conditionals = None
    builtin_voice = path / "conds.pt"
    if builtin_voice.exists():
        conditionals = Conditionals.load(
            builtin_voice, map_location=map_location
        ).to(device)
    return ChatterboxTurboTTS(
        t3, s3gen, voice_encoder, tokenizer, device, conds=conditionals
    )


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

        model = load_chatterbox_nano(args.model, device)
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

    device = "cuda:0" if torch.cuda.is_available() else "cpu"
    dtype = torch.bfloat16 if torch.cuda.is_available() and torch.cuda.is_bf16_supported() else torch.float32
    model = Qwen3TTSModel.from_pretrained(
        args.model,
        device_map=device,
        dtype=dtype,
        attn_implementation="sdpa",
        local_files_only=True,
    )
    variant = args.model_name.lower()
    common = {"text": args.text, "language": args.language or "Auto"}
    if "custom-voice" in variant:
        if not args.speaker:
            raise SystemExit("Qwen3-TTS CustomVoice requires --speaker")
        wavs, sample_rate = model.generate_custom_voice(
            **common, speaker=args.speaker, instruct=args.instruct or None
        )
    elif "voice-design" in variant:
        if not args.instruct:
            raise SystemExit("Qwen3-TTS VoiceDesign requires --instruct")
        wavs, sample_rate = model.generate_voice_design(
            **common, instruct=args.instruct
        )
    else:
        if not args.voice_sample:
            raise SystemExit(
                "Qwen3-TTS Base requires --voice-sample or --voice for voice cloning"
            )
        wavs, sample_rate = model.generate_voice_clone(
            **common,
            ref_audio=args.voice_sample,
            ref_text=args.transcript or None,
            x_vector_only_mode=not bool(args.transcript),
        )
    sf.write(args.output, wavs[0], sample_rate)


def qwen_mlx(args):
    from mlx_audio.tts.generate import generate_audio
    from mlx_audio.tts.utils import load_model

    model = load_model(args.model)
    with tempfile.TemporaryDirectory(prefix="tapioca-speech-") as output_dir:
        options = dict(
            model=model,
            text=args.text,
            output_path=output_dir,
            file_prefix="speech",
            audio_format="wav",
            join_audio=True,
            lang_code=args.language or "auto",
            play=False,
            verbose=False,
        )
        variant = args.model_name.lower()
        if "custom-voice" in variant:
            if not args.speaker:
                raise SystemExit("Qwen3-TTS CustomVoice requires --speaker")
            options.update(voice=args.speaker, instruct=args.instruct or None)
        elif "voice-design" in variant:
            if not args.instruct:
                raise SystemExit("Qwen3-TTS VoiceDesign requires --instruct")
            options.update(voice=None, instruct=args.instruct)
        else:
            if not args.voice_sample:
                raise SystemExit(
                    "Qwen3-TTS Base requires --voice-sample or --voice for voice cloning"
                )
            options.update(
                voice=None,
                ref_audio=args.voice_sample,
                ref_text=args.transcript or None,
            )
        generate_audio(**options)
        candidates = glob.glob(os.path.join(output_dir, "*.wav"))
        if not candidates:
            raise SystemExit("MLX Audio did not produce a WAV file")
        shutil.move(candidates[0], args.output)


def audio8(args):
    import soundfile as sf
    import torch
    from transformers import AutoModel, AutoProcessor

    if args.voice_sample and not args.transcript:
        raise SystemExit("Audio8 voice cloning requires --transcript")
    device = "cuda" if torch.cuda.is_available() else "cpu"
    dtype = torch.bfloat16 if device == "cuda" else torch.float32
    processor = AutoProcessor.from_pretrained(
        args.model, trust_remote_code=True, local_files_only=True
    )
    model = AutoModel.from_pretrained(
        args.model,
        trust_remote_code=True,
        local_files_only=True,
        dtype=dtype,
    ).eval().to(device)
    processor_options = {"text": [args.text], "return_tensors": "pt"}
    if args.voice_sample:
        processor_options.update(
            reference_audio=[args.voice_sample], reference_text=[args.transcript]
        )
    inputs = processor(**processor_options)
    inputs = {
        name: value.to(device) if hasattr(value, "to") else value
        for name, value in inputs.items()
    }
    with torch.inference_mode():
        generated = model.generate(
            **inputs,
            max_new_tokens=1024,
            temperature=0.8,
            top_p=0.95,
            top_k=50,
            do_sample=True,
            return_dict_in_generate=True,
        )
        waveforms, lengths = model.decode_audio(generated.codes)
    audio = waveforms[0, : int(lengths[0])].float().cpu().numpy()
    sf.write(args.output, audio, model.config.codec_sample_rate)


def audio8_mlx(args):
    if args.voice_sample and not args.transcript:
        raise SystemExit("Audio8 voice cloning requires --transcript")
    from mlx_audio.tts.generate import generate_audio
    from mlx_audio.tts.utils import load_model

    model = load_model(args.model)
    with tempfile.TemporaryDirectory(prefix="tapioca-speech-") as output_dir:
        generate_audio(
            model=model,
            text=args.text,
            voice=None,
            ref_audio=args.voice_sample or None,
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
    parser.add_argument("--voice-consent", action="store_true")
    parser.add_argument("--seed", type=int, default=0)
    parser.add_argument("--speaker", default="")
    parser.add_argument("--instruct", default="")
    args = parser.parse_args()

    if os.path.splitext(args.output)[1].lower() != ".wav":
        raise SystemExit("the first speech release supports WAV output; use a .wav filename")
    if args.backend == "speech-chatterbox":
        chatterbox(args)
    elif args.backend == "speech-qwen":
        qwen(args)
    elif args.backend == "speech-qwen-mlx":
        qwen_mlx(args)
    elif args.backend == "speech-audio8-onnx":
        from cpu_speech import audio8
        audio8(args)
    elif args.backend == "speech-pocket-tts":
        from pocket_qualification import run
        args.language = args.language or "en"
        run(args)
    elif args.backend == "speech-sopro":
        from sopro_qualification import run
        run(args)
    elif args.backend == "speech-audio8":
        audio8(args)
    elif args.backend == "speech-audio8-mlx":
        audio8_mlx(args)
    else:
        raise SystemExit(f"unsupported speech backend: {args.backend}")


if __name__ == "__main__":
    main()
