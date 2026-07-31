import argparse
import json
import os

import numpy as np
import onnxruntime as ort
from PIL import Image
from transformers import CLIPTokenizer


def session(path, provider):
    options = ort.SessionOptions()
    providers = [provider]
    if provider == "DmlExecutionProvider":
        options.enable_mem_pattern = False
        options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
    return ort.InferenceSession(path, sess_options=options, providers=providers)


def load_json(path):
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def scheduler_sigmas(model, steps):
    config = load_json(os.path.join(model, "scheduler", "scheduler_config.json"))
    count = int(config.get("num_train_timesteps", 1000))
    start = float(config.get("beta_start", 0.00085))
    end = float(config.get("beta_end", 0.012))
    schedule = config.get("beta_schedule", "scaled_linear")
    if schedule == "scaled_linear":
        betas = np.linspace(start**0.5, end**0.5, count, dtype=np.float64) ** 2
    elif schedule == "linear":
        betas = np.linspace(start, end, count, dtype=np.float64)
    else:
        raise RuntimeError(f"unsupported scheduler beta schedule: {schedule}")
    alphas = 1.0 - betas
    cumulative = np.cumprod(alphas)
    training_sigmas = np.sqrt((1.0 - cumulative) / cumulative)
    spacing = config.get("timestep_spacing", "linspace")
    if spacing == "trailing":
        ratio = count / steps
        timesteps = np.arange(count, 0, -ratio, dtype=np.float64).round() - 1
        timesteps = timesteps.astype(np.float32)
    elif spacing == "linspace":
        timesteps = np.linspace(count - 1, 0, steps, dtype=np.float32)
    else:
        raise RuntimeError(f"unsupported scheduler timestep spacing: {spacing}")
    sigmas = np.interp(timesteps, np.arange(count), training_sigmas)
    return timesteps, np.append(sigmas, 0.0).astype(np.float32)


def output_by_name(runtime, values, preferred):
    names = [item.name for item in runtime.get_outputs()]
    index = names.index(preferred) if preferred in names else 0
    return values[index]


def input_name(runtime, preferred):
    names = [item.name for item in runtime.get_inputs()]
    for name in preferred:
        if name in names:
            return name
    raise RuntimeError(f"expected one of {preferred}; model inputs are {names}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--negative-prompt", default="")
    parser.add_argument("--output", required=True)
    parser.add_argument("--width", type=int, required=True)
    parser.add_argument("--height", type=int, required=True)
    parser.add_argument("--steps", type=int, required=True)
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--provider", required=True)
    args = parser.parse_args()

    available = ort.get_available_providers()
    if args.provider not in available:
        raise RuntimeError(
            f"{args.provider} is unavailable; ONNX Runtime reported: {available}"
        )
    tokenizer = CLIPTokenizer.from_pretrained(
        os.path.join(args.model, "tokenizer"), local_files_only=True
    )
    text_encoder = session(
        os.path.join(args.model, "text_encoder", "model.onnx"), args.provider
    )
    unet = session(os.path.join(args.model, "unet", "model.onnx"), args.provider)
    decoder = session(
        os.path.join(args.model, "vae_decoder", "model.onnx"), args.provider
    )

    tokens = tokenizer(
        args.prompt,
        padding="max_length",
        max_length=tokenizer.model_max_length,
        truncation=True,
        return_tensors="np",
    ).input_ids.astype(np.int32)
    token_input = input_name(text_encoder, ("input_ids",))
    embeddings = output_by_name(
        text_encoder, text_encoder.run(None, {token_input: tokens}), "last_hidden_state"
    )

    timesteps, sigmas = scheduler_sigmas(args.model, args.steps)
    random = np.random.RandomState(args.seed)
    latents = random.randn(1, 4, args.height // 8, args.width // 8).astype(np.float32)
    latents *= sigmas[0]
    sample_input = input_name(unet, ("sample", "latent_model_input"))
    timestep_input = input_name(unet, ("timestep", "timesteps"))
    encoder_input = input_name(unet, ("encoder_hidden_states", "prompt_embeds"))

    for index, timestep in enumerate(timesteps):
        sigma = sigmas[index]
        scaled = latents / np.sqrt(sigma * sigma + 1.0)
        values = {
            sample_input: scaled.astype(np.float32),
            timestep_input: np.asarray(timestep, dtype=np.float32),
            encoder_input: embeddings.astype(np.float32),
        }
        noise = output_by_name(unet, unet.run(None, values), "out_sample")
        latents = latents + noise * (sigmas[index + 1] - sigma)

    decoder_input = input_name(decoder, ("latent_sample", "sample"))
    decoded = output_by_name(
        decoder,
        decoder.run(None, {decoder_input: (latents / 0.18215).astype(np.float32)}),
        "sample",
    )
    pixels = np.clip(decoded / 2.0 + 0.5, 0.0, 1.0)
    pixels = np.transpose(pixels[0], (1, 2, 0))
    image = Image.fromarray((pixels * 255).round().astype(np.uint8))
    os.makedirs(os.path.dirname(os.path.abspath(args.output)), exist_ok=True)
    image.save(args.output)


if __name__ == "__main__":
    main()
