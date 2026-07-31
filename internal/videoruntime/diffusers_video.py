import argparse
import inspect

import torch
from diffusers import DiffusionPipeline, StableVideoDiffusionPipeline
from diffusers.utils import export_to_video, load_image


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--negative-prompt", default="")
    parser.add_argument("--image")
    parser.add_argument("--output", required=True)
    parser.add_argument("--width", type=int, required=True)
    parser.add_argument("--height", type=int, required=True)
    parser.add_argument("--frames", type=int, required=True)
    parser.add_argument("--steps", type=int, required=True)
    parser.add_argument("--fps", type=int, required=True)
    parser.add_argument("--seed", type=int, required=True)
    args = parser.parse_args()

    if not torch.cuda.is_available():
        raise RuntimeError("a CUDA-capable NVIDIA GPU and driver are required")

    is_svd = "stable-video-diffusion" in args.model.lower()
    pipeline_class = StableVideoDiffusionPipeline if is_svd else DiffusionPipeline
    load_options = {
        "torch_dtype": torch.float16 if is_svd else torch.bfloat16,
        "local_files_only": True,
    }
    if is_svd:
        load_options["variant"] = "fp16"
    pipe = pipeline_class.from_pretrained(args.model, **load_options)
    pipe.enable_sequential_cpu_offload()
    if hasattr(pipe, "vae"):
        pipe.vae.enable_tiling()
    if hasattr(pipe, "unet") and hasattr(pipe.unet, "enable_forward_chunking"):
        pipe.unet.enable_forward_chunking()

    call_args = {
        "prompt": args.prompt,
        "negative_prompt": args.negative_prompt,
        "height": args.height,
        "width": args.width,
        "num_frames": args.frames,
        "num_inference_steps": args.steps,
        "decode_chunk_size": 2,
        "generator": torch.Generator().manual_seed(args.seed),
    }
    if args.image:
        image = load_image(args.image)
        call_args["image"] = image.resize((args.width, args.height))
    accepted = inspect.signature(pipe.__call__).parameters
    call_args = {key: value for key, value in call_args.items() if key in accepted}
    frames = pipe(**call_args).frames[0]
    export_to_video(frames, args.output, fps=args.fps)


if __name__ == "__main__":
    main()
