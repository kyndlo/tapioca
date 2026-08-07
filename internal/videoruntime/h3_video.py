"""Submit a patched MiniMax-H3 graph to Tapioca's managed ComfyUI process."""
import argparse
import json
import os
import shutil
import time
import urllib.error
import urllib.request
import uuid


def get(base, path):
    with urllib.request.urlopen(base + path, timeout=60) as response:
        return json.load(response)


def post(base, path, value):
    request = urllib.request.Request(
        base + path,
        data=json.dumps(value).encode("utf-8"),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--server", required=True)
    parser.add_argument("--workflow", required=True)
    parser.add_argument("--comfy-root", required=True)
    parser.add_argument("--backend", required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--image")
    parser.add_argument("--adapter", action="append", default=[])
    parser.add_argument("--adapter-scale", action="append", type=float, default=[])
    parser.add_argument("--width", type=int, required=True)
    parser.add_argument("--height", type=int, required=True)
    parser.add_argument("--frames", type=int, required=True)
    parser.add_argument("--steps", type=int, required=True)
    parser.add_argument("--fps", type=int, required=True)
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--dump-graph", help="write the patched graph and exit")
    args = parser.parse_args()

    if len(args.adapter) != len(args.adapter_scale):
        raise RuntimeError("each MiniMax-H3 LoRA requires one adapter scale")

    with open(args.workflow, encoding="utf-8") as handle:
        graph = json.load(handle)

    graph["105:119"] = {
        "class_type": "UNETLoader",
        "inputs": {
            "unet_name": "minimax_h3_fl2va_pruned_int8_convrot.safetensors",
            "weight_dtype": "default",
        },
        "_meta": {"title": "MiniMax H3 INT8"},
    }
    if args.backend == "comfy-h3-mps":
        graph["105:120"] = {
            "class_type": "CLIPLoaderGGUF",
            "inputs": {
                "clip_name": "qwen3vl-32B-MiniMax-H3-Q4_K_M.gguf",
                "type": "minimax",
            },
            "_meta": {"title": "Qwen3-VL GGUF"},
        }
    else:
        graph["105:120"] = {
            "class_type": "CLIPLoader",
            "inputs": {
                "clip_name": "qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors",
                "type": "minimax",
                "device": "default",
            },
            "_meta": {"title": "Qwen3-VL NVFP4"},
        }

    # H3 LoRAs target the diffusion transformer. Chain adapters in CLI order;
    # Tapioca owns this stack independently of the underlying workflow IDs.
    model_output = ["105:119", 0]
    for index, (name, scale) in enumerate(zip(args.adapter, args.adapter_scale)):
        node_id = "tapioca:lora:%d" % index
        graph[node_id] = {
            "class_type": "LoraLoaderModelOnly",
            "inputs": {
                "model": model_output,
                "lora_name": name,
                "strength_model": scale,
            },
            "_meta": {"title": "Tapioca LoRA %d" % (index + 1)},
        }
        model_output = [node_id, 0]
    graph["105:9"]["inputs"]["model"] = model_output
    graph["105:16"]["inputs"]["model"] = model_output

    graph["105:11"]["inputs"]["vae_name"] = "minimax_h3_video_vae_fp16.safetensors"
    graph["105:24"]["inputs"]["vae_name"] = "minimax_h3_audio_vae_fp32.safetensors"
    graph["105:104"]["inputs"].update({
        "prompt": args.prompt,
        "width": args.width,
        "height": args.height,
        "length": args.frames,
    })
    graph["105:9"]["inputs"]["steps"] = args.steps
    graph["105:15"]["inputs"]["noise_seed"] = args.seed
    graph["105:91"]["inputs"]["fps"] = args.fps
    graph["92"]["inputs"]["filename_prefix"] = "video/tapioca_h3"

    # Width, height, and length are supplied directly by Tapioca.
    for node in ("115", "105:107", "105:111", "123", "124"):
        graph.pop(node, None)

    if args.image:
        name = "tapioca-" + uuid.uuid4().hex + os.path.splitext(args.image)[1]
        shutil.copy2(args.image, os.path.join(args.comfy_root, "input", name))
        graph["121"]["inputs"]["image"] = name
        graph["105:104"]["inputs"]["first_frame"] = ["121", 0]
        graph["105:104"]["inputs"].pop("last_frame", None)
    else:
        graph.pop("121", None)
        graph.pop("125", None)
        graph["105:104"]["inputs"].pop("first_frame", None)
        graph["105:104"]["inputs"].pop("last_frame", None)

    if args.dump_graph:
        with open(args.dump_graph, "w", encoding="utf-8") as handle:
            json.dump(graph, handle, indent=2)
        return

    client = str(uuid.uuid4())
    try:
        result = post(args.server, "/prompt", {"prompt": graph, "client_id": client})
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", "replace")
        raise RuntimeError("ComfyUI rejected the MiniMax-H3 workflow: " + detail) from error
    prompt_id = result["prompt_id"]
    started = time.time()
    while True:
        history = get(args.server, "/history/" + prompt_id)
        if prompt_id in history:
            break
        print("MiniMax-H3 is generating locally (%dm elapsed)..." %
              ((time.time() - started) // 60), flush=True)
        time.sleep(10)

    entry = history[prompt_id]
    status = entry.get("status", {})
    if not status.get("completed"):
        raise RuntimeError("MiniMax-H3 generation failed: " + json.dumps(status))
    files = []
    for output in entry.get("outputs", {}).values():
        files.extend(output.get("video", []))
        files.extend(output.get("videos", []))
        files.extend(output.get("gifs", []))
        files.extend(output.get("images", []))
    if not files:
        raise RuntimeError("MiniMax-H3 completed but ComfyUI reported no video")
    item = files[-1]
    source = os.path.join(
        args.comfy_root, "output", item.get("subfolder", ""), item["filename"]
    )
    shutil.copy2(source, args.output)
    print("MiniMax-H3 completed in %.1f minutes" % ((time.time() - started) / 60), flush=True)


if __name__ == "__main__":
    main()
