import argparse
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--negative-prompt", default=" ")
    parser.add_argument("--width", type=int, default=1024)
    parser.add_argument("--height", type=int, default=1024)
    parser.add_argument("--steps", type=int, default=4)
    parser.add_argument("--seed", type=int, default=0)
    args = parser.parse_args()

    import torch
    from diffusers import DiffusionPipeline

    if not torch.cuda.is_available():
        raise SystemExit(
            "CUDA is unavailable. Install a current NVIDIA driver and a CUDA-enabled "
            "PyTorch build; AMD and CPU diffusion backends are not supported yet."
        )
    if not torch.cuda.is_bf16_supported():
        raise SystemExit(
            "Qwen-Image-Flash requires a CUDA GPU with bfloat16 support "
            "(NVIDIA Ampere generation or newer)."
        )

    properties = torch.cuda.get_device_properties(0)
    vram_gb = properties.total_memory / (1024 ** 3)
    print(
        f"using CUDA device: {torch.cuda.get_device_name(0)} ({vram_gb:.1f} GB VRAM)",
        file=sys.stderr,
    )
    pipe = DiffusionPipeline.from_pretrained(
        args.model,
        torch_dtype=torch.bfloat16,
        local_files_only=True,
    )
    if vram_gb >= 64:
        pipe.to("cuda")
    else:
        print(
            "enabling sequential CPU offload; generation will be slower because this "
            "checkpoint is larger than available VRAM",
            file=sys.stderr,
        )
        pipe.enable_sequential_cpu_offload()
    if hasattr(pipe, "enable_vae_tiling"):
        pipe.enable_vae_tiling()
    if hasattr(pipe, "enable_vae_slicing"):
        pipe.enable_vae_slicing()

    generator = torch.Generator(device="cuda").manual_seed(args.seed)
    image = pipe(
        prompt=args.prompt,
        negative_prompt=args.negative_prompt,
        width=args.width,
        height=args.height,
        num_inference_steps=args.steps,
        true_cfg_scale=1.0,
        generator=generator,
    ).images[0]
    image.save(args.output)


if __name__ == "__main__":
    main()
