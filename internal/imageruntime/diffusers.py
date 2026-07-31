import argparse
import inspect
import os
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
    is_qwen_image = "qwen-image" in args.model.lower()
    if is_qwen_image and not torch.cuda.is_bf16_supported():
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
    dtype = torch.bfloat16 if is_qwen_image else torch.float16
    load_options = {
        "torch_dtype": dtype,
        "local_files_only": True,
    }
    if not is_qwen_image:
        load_options["variant"] = "fp16"
    pipe = DiffusionPipeline.from_pretrained(args.model, **load_options)
    weight_bytes = 0
    for root, _, files in os.walk(args.model):
        for name in files:
            if name.endswith(".safetensors"):
                weight_bytes += os.path.getsize(os.path.join(root, name))
    weight_gb = weight_bytes / (1024 ** 3)
    if weight_gb and weight_gb <= vram_gb * 0.65:
        pipe.to("cuda")
    else:
        print(
            f"enabling sequential CPU offload for {weight_gb:.1f} GB of weights; "
            "generation will be slower",
            file=sys.stderr,
        )
        pipe.enable_sequential_cpu_offload()
    if hasattr(pipe, "enable_vae_tiling"):
        pipe.enable_vae_tiling()
    if hasattr(pipe, "enable_vae_slicing"):
        pipe.enable_vae_slicing()

    generator = torch.Generator(device="cuda").manual_seed(args.seed)
    supported = inspect.signature(pipe.__call__).parameters
    generation = {
        "prompt": args.prompt,
        "negative_prompt": args.negative_prompt,
        "width": args.width,
        "height": args.height,
        "num_inference_steps": args.steps,
        "generator": generator,
    }
    if "true_cfg_scale" in supported:
        generation["true_cfg_scale"] = 1.0
    elif "guidance_scale" in supported:
        generation["guidance_scale"] = 0.0 if "turbo" in args.model.lower() else 7.5
    generation = {key: value for key, value in generation.items() if key in supported}
    image = pipe(**generation).images[0]
    image.save(args.output)


if __name__ == "__main__":
    main()
