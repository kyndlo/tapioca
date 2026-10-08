import argparse
import json
import os


def load_local_model(model_path):
    import torch
    from stable_audio_3.loading_utils import load_diffusion_cond
    from stable_audio_3.model import StableAudioModel

    config_path = os.path.join(model_path, "model_config.json")
    checkpoint_path = os.path.join(model_path, "model.safetensors")
    with open(config_path, encoding="utf-8") as handle:
        model_config = json.load(handle)

    text_encoder = os.path.join(model_path, "t5gemma-b-b-ul2")
    conditioning = model_config.get("model", {}).get("conditioning", {})
    for item in conditioning.get("configs", []):
        if item.get("type") == "t5gemma":
            item.setdefault("config", {})["model_path"] = text_encoder
            item["config"].pop("repo_id", None)
            item["config"].pop("subfolder", None)

    if torch.cuda.is_available():
        device = "cuda"
    elif hasattr(torch.backends, "mps") and torch.backends.mps.is_available():
        device = "mps"
    else:
        device = "cpu"
    model_half = device == "cuda"
    loaded = load_diffusion_cond(
        model_config,
        checkpoint_path,
        device=device,
        model_half=model_half,
    )
    loaded.use_lora = False
    loaded.lora_names = []
    return StableAudioModel(loaded, model_config, device, model_half)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--negative-prompt")
    parser.add_argument("--output", required=True)
    parser.add_argument("--duration", required=True, type=int)
    parser.add_argument("--steps", required=True, type=int)
    parser.add_argument("--seed", required=True, type=int)
    args = parser.parse_args()

    if os.path.splitext(args.output)[1].lower() != ".wav":
        raise SystemExit("Stable Audio 3 output must use the .wav extension")
    import torchaudio

    model = load_local_model(args.model)
    audio = model.generate(
        prompt=args.prompt,
        negative_prompt=args.negative_prompt,
        duration=args.duration,
        steps=args.steps,
        cfg_scale=1.0,
        seed=args.seed,
        batch_size=1,
    )
    torchaudio.save(args.output, audio[0].detach().cpu(), model.model.sample_rate)


if __name__ == "__main__":
    main()
