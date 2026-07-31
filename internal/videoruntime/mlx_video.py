import argparse
import subprocess
import sys


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
    subprocess.run(command, check=True)


if __name__ == "__main__":
    main()
