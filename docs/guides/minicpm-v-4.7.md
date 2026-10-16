# MiniCPM-V 4.7 1B still-image candidate

Research and local smoke testing: **2026-10-16**, issue [#57](https://github.com/kyndlo/tapioca/issues/57).
This is an optional qualification candidate, outside the published catalog and
its checksum. Existing model defaults and the shared llama.cpp b11476 bundles
are unchanged. Do not describe this as general cross-platform vision support.

The candidate is `minicpm-v-4.7:1b-q4-k-m`: OpenBMB's Q4_K_M language model and
matching **1B, 16x** F16 projector, both at
`6c2f4cb8bde51a00247e11003bbf66ca7c4f9249`. The exact filenames, full SHA-256
hashes and byte counts are in `catalog/candidates/minicpm-v-4.7.json`.
Weights total 1,782,121,664 bytes (1.66 GiB); this is not a memory requirement.
The pinned Apache-2.0 model card is downloaded as `MODEL_CARD.md` with its own
integrity metadata. See `THIRD_PARTY_NOTICES.md` for attribution.

## Try the candidate on macOS Apple Silicon

Use a separate data directory so the qualification manifest does not replace
an existing personal catalog. From the repository root:

```sh
go build -o bin/tapioca ./cmd/tapioca
sh scripts/prepare-vision-runtime.sh bin/runtime
export TAPIOCA_HOME="$PWD/.minicpm-v47"
mkdir -p "$TAPIOCA_HOME"
cp catalog/candidates/minicpm-v-4.7.json "$TAPIOCA_HOME/catalog.json"
bin/tapioca pull minicpm-v-4.7
bin/tapioca serve minicpm-v-4.7 --context 4096
```

Add `--cpu` to disable both language-model and projector GPU offload. Context
is limited to 8192; begin at 4096. `catalog update` replaces the local candidate
manifest. Desktop qualification uses the same data directory and the packaged
isolated runtime; restart Desktop after selecting `TAPIOCA_HOME`.

The separate runtime is llama.cpp **b11696**, commit
`ceefd16daa2093730bf3340315fa2b84b286ddbc`, including upstream #29416 (145
commits ahead of its merge). The macOS archive SHA-256 is
`6aac007fbe153efefe06250d2cf05d0487583aa0706379082ebef2b05579f649`.
The installer checks that hash, archive layout and version before installing
`runtime/llama-vision-b11696/`. Fresh installs and upgrades use this versioned
path; they cannot fall back to the older shared runtime or an arbitrary PATH
binary. An explicit `--llama-server` override must identify that exact build.
The archive retains upstream licenses and notices.

Both weights are verified before registry registration, on offline pull reuse,
and again before server launch. A corrupt or mismatched projector fails with
an error; `pull --force` repairs the pair. Interrupted downloads retain partial
files for range resumption. Failed validation deletes the corrupt partial,
keeps any previously verified destination and never registers an incomplete pair.

## Supported request scope

Use `POST /v1/chat/completions` with OpenAI `text` and `image_url` content parts.
Images must be inline `data:image/png;base64,...` or
`data:image/jpeg;base64,...` URLs, each at most **512 KiB** and **1024 pixels
per side**. At most **four images across the full conversation history** are
accepted, in their supplied order. Keep the earlier user image parts in
multi-turn requests. Requests are limited to 4 MiB and 256 messages.

The Desktop file picker accepts up to four PNG/JPEG attachments per
conversation, at most **128 KiB each**, within the existing sidecar message
budget. Previews and session history retain attachments. The Desktop adapter
continues to return a completed answer; token streaming is qualified through
the HTTP chat endpoint, not claimed for Desktop.

Remote/file URLs, malformed images, oversized inputs, audio, video and other
content-part modalities fail explicitly. The Responses and Anthropic adapters
remain text/tool adapters and reject media before conversion, rather than
silently dropping it. Text-only model requests retain their existing runtime.

The embedded official GGUF chat template is used with `--jinja`, without a
replacement template. Thinking defaults to off, including when callers supply
an empty `chat_template_kwargs` object. Explicit reasoning is available through
`"chat_template_kwargs":{"enable_thinking":true}`. Output defaults to 512
tokens and is capped at 1024; `max_tokens` is the supported limit field.
Streaming forwards SSE, and client cancellation reaches the upstream request.

## Acceptance and remaining qualification

Run automated Go, desktop and freshness checks, then opt into real-weight tests:

```sh
python3 scripts/qualify-minicpm-vision.py \
  --binary "$PWD/bin/tapioca" \
  --runtime "$PWD/bin/runtime/llama-vision-b11696/llama-server" \
  --work /tmp/tapioca-minicpm-acceptance \
  --reference /path/to/MiniCPM-V-4_7-1B-F16.gguf
```

The optional F16 reference is pinned to the same revision, 2,024,372,320 bytes,
SHA-256 `bc5a85f8973c3cbb29b3bd195cce5279361221a8628bd50a2387261289ea2fbc`.
The script downloads through Tapioca, verifies full hashes, offline reuse and
corrupt/missing artifact rejection, then tests CPU/Metal at 4K/8K and the F16
reference on Metal at 4K. Fixed fixtures
cover shape/color description, OCR, multi-turn reference, ordered multiple
images, reasoning off/on, streaming EOS, cancellation and subsequent reuse.
It writes answers, correctness checks, latency and sampled peak host RSS to the
chosen work directory. Unit tests cover interrupted downloads, corrupt-cache
and corrupt-projector rejection, installed context after catalog refresh,
stale runtime versions, image/body bounds, unsupported modalities, sidecar
forwarding and desktop image persistence/retry without duplicated prompts.
`python3 scripts/test-minicpm-qualification.py` checks normal shutdown and
forced cleanup of a separately grouped backend on timeout.

Real-weight smoke evidence is from an **Apple M2 Max with 96 GiB unified
memory**. All five cases passed:

| Case | Description / OCR latency | Sampled peak host RSS | Fixed checks |
|---|---|---|---|
| q4-cpu-4k | 26.82s / 29.44s | 2.52 GiB | 6/6, streaming EOS |
| q4-cpu-8k | 26.53s / 25.52s | 2.56 GiB | 6/6, streaming EOS |
| q4-metal-4k | 0.67s / 0.25s | 2.34 GiB | 6/6, streaming EOS |
| q4-metal-8k | 0.66s / 0.25s | 2.38 GiB | 6/6, streaming EOS |
| f16-metal-4k | 0.77s / 0.28s | 3.60 GiB | 6/6, streaming EOS |

These short-fixture timings include image preprocessing and generation and
may reuse prompt/image caches; they are not comparative benchmarks. Both Q4
and F16 read `TAPIOCA 57 / TOTAL 12.50`; default reasoning was absent and
explicit thinking produced reasoning content. Closing a stream was followed
by a nonempty EOS-terminated answer; the test does not require the model to
echo a particular cancellation-probe word. Full-file byte corruption was also
rejected on both pull and launch for each weight file, and a missing projector
failed before launch. Fixture images are committed under
`internal/server/testdata/vision/`; they were generated locally from geometric
shapes and the fixed OCR strings, without external-image licensing dependencies.

Desktop browser checks cover 1440×1100 and 768×1000 with a mocked IPC response.
An additional **Electron 43.2.0 source-build smoke** used the built renderer,
real preload/IPC, Go sidecar, and the pinned Metal runtime (repeated after
rebasing onto current main): an attached image
returned the correct red-circle/blue-square description, and the follow-up
returned `Red`. It also exercised catalog refresh before server startup with
an installed candidate. Neither check qualifies signed/packaged release
installers. The Browser plugin was absent; existing Playwright was used.
Browser page identity, nonblank rendering, no framework overlay, attachment
visibility/previews, history preservation and narrow layout passed. No relevant
app console errors occurred; browser-only QA observed the existing ignored
`frame-ancestors` meta-CSP warning.

No 8/16 GiB hardware minimum or separate peak GPU/unified-memory measurement
is established. Those machines remain qualification targets. Windows/Linux
Vulkan, Windows ARM64 CPU, CUDA, MLX/MLX-VLM, mobile, speculative decoding,
35B, 4x projectors, long-context quality and video-input UX are unqualified or
out of scope. Keep the manifest unpublished until the remaining shipping and
memory qualification is complete; do not update the public catalog checksum.

## Research sources

The same-day model watch queried current/trending Hugging Face language,
image, audio/speech and video models and the LocalLLaMA, LocalLLM,
StableDiffusion, comfyui, AudioAI and accelerate Reddit feeds. Its existing
freshness checks passed; the suite now also covers vision discovery/projector
inventory while retaining the read-only, manual workflow policy.
Exact Hub metadata confirmed public/ungated access, Apache-2.0 and the pinned
artifact sizes/hashes. Upstream ancestry, archive layout and local loading were
checked separately from community discovery.

- [Official source model](https://huggingface.co/openbmb/MiniCPM-V-4.7-1B)
- [Pinned GGUF repository and model card](https://huggingface.co/openbmb/MiniCPM-V-4.7-1B-gguf/tree/6c2f4cb8bde51a00247e11003bbf66ca7c4f9249)
- [Pinned llama.cpp MiniCPM documentation](https://github.com/ggml-org/llama.cpp/blob/ceefd16daa2093730bf3340315fa2b84b286ddbc/docs/multimodal/minicpmv4.7.md)
- [Runtime release](https://github.com/ggml-org/llama.cpp/releases/tag/b11696), [upstream integration #29416](https://github.com/ggml-org/llama.cpp/pull/29416)
- [Official deployment cookbook](https://opensqz.github.io/MiniCPM-V-CookBook/site/en/v4.7/deployment/llamacpp.html)
- [Reddit release discussion](https://www.reddit.com/r/LocalLLaMA/comments/1x492f9/openbmbminicpmv471b/), [LocalLLM field report](https://www.reddit.com/r/LocalLLM/comments/1x6oxwq/running_minicpmv_1b_fully_offline_on_iphone_to/): discovery evidence, not Tapioca hardware qualification.

The issue's newer MLX-VLM alternative is intentionally outside this GGUF
model/projector integration; no MLX runtime pin was changed.
