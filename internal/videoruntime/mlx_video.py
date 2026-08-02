import argparse
import os
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--negative-prompt", default="")
    parser.add_argument("--image")
    parser.add_argument("--adapter", action="append", default=[])
    parser.add_argument("--adapter-scale", action="append", type=float, default=[])
    parser.add_argument("--output", required=True)
    parser.add_argument("--width", type=int, required=True)
    parser.add_argument("--height", type=int, required=True)
    parser.add_argument("--frames", type=int, required=True)
    parser.add_argument("--steps", type=int, required=True)
    parser.add_argument("--fps", type=int, required=True)
    parser.add_argument("--seed", type=int, required=True)
    args = parser.parse_args()

    command = [
        sys.executable,
        "-m",
        "mlx_video.models.wan_2.generate",
        "--model-dir",
        args.model,
        "--prompt",
        args.prompt,
        "--width",
        str(args.width),
        "--height",
        str(args.height),
        "--num-frames",
        str(args.frames),
        "--steps",
        str(args.steps),
        "--seed",
        str(args.seed),
        "--output-path",
        args.output,
    ]
    if args.image:
        command.extend(["--image", args.image])
    if args.negative_prompt:
        command.extend(["--negative-prompt", args.negative_prompt])
    if len(args.adapter) != len(args.adapter_scale):
        raise SystemExit("each --adapter requires one --adapter-scale")
    for path, scale in zip(args.adapter, args.adapter_scale):
        command.extend(["--lora", path, str(scale)])
    subprocess.run(command, check=True)
    # mlx-video currently encodes WAN outputs at the model's native 24 FPS.
    # Re-time the generated frames when the user requested a different playback
    # rate so the CLI/UI FPS control has an observable effect.
    if args.fps != 24:
        import imageio.v2 as imageio

        temporary = args.output + ".fps.mp4"
        reader = imageio.get_reader(args.output)
        writer = imageio.get_writer(temporary, fps=args.fps, codec="libx264", quality=8)
        try:
            for frame in reader:
                writer.append_data(frame)
        finally:
            reader.close()
            writer.close()
        os.replace(temporary, args.output)


if __name__ == "__main__":
    main()
