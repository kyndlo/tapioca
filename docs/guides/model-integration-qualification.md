# Model integration qualification

Research/validation: 2026-09-05. Addresses [#30](https://github.com/kyndlo/tapioca/issues/30), [#31](https://github.com/kyndlo/tapioca/issues/31), and [#32](https://github.com/kyndlo/tapioca/issues/32). Issue #29 stays historical. Daily discovery remains local Codex scheduling, not GitHub cron.

## Release scope and downloads

Granite 4.2 3B Q4_K_M is built-in for text chat at a conservative 8K default. Audio8 and Pocket TTS adapters are experimental and **not in the public/built-in catalog**. Desktop changes improve filtering, result counts/reset, installed-model disk checks, progress, retry/removal errors, duplicate-action prevention, modal keyboard/focus handling and voice-reference consent/transcripts. Windows runtime extraction now passes paths as environment data instead of the ineffective PowerShell `-Command` trailing arguments.

Single-file `downloads` and explicit bundle artifacts support full `revision`, `size_bytes`, and `sha256` metadata. New/cached artifacts are checked before registration/replacement. Failed forced replacement preserves a good cached file; corrupt partial files are discarded for a clean retry. Legacy unpinned entries retain their previous behavior. Watcher checks follow pinned revisions.

Older clients reject unknown manifest fields. The remote production manifest is unchanged for the client release. Built-ins merge with cached catalogs, so Granite is available without a remote update. Publish new remote metadata only after a compatible client is available. Candidate manifests are not activated automatically; validation is not qualification.

## Granite — #32

Official artifact: `ibm-granite/granite-4.2-3b-GGUF`, revision `47a3d9699d7539606c83943d717fcea7bd9f6a19`, `granite-4.2-3b-Q4_K_M.gguf`, 2,244,012,160 bytes, SHA-256 `20e436143017578687f7f848225cc6c6038126c84149192229c7dff6e4e0f427`. Ungated Apache-2.0 dense Granite; released August 25, 2026. Guidance: 8 GiB minimum / 16 GiB recommended at 8K. Embedded chat template retained; no global default/runtime pin change.

The actual Tapioca downloader retrieved and verified the artifact. Bundled llama.cpp b10603 generated `Two plus two is four.` on Mac Metal and Windows CPU/Vulkan. Short-fixture generation measured about 25.6 tok/s on Mac Metal, 44.0 on the Windows Vulkan bundle and 11.7 on Windows CPU; these are smoke results, not comparative benchmarks.

Through Tapioca's OpenAI-compatible API, Spanish system/multi-turn history returned `Te llamas Ana.` with thinking disabled. Thinking enabled and low-effort prompts both terminated with answer `4` and separate reasoning content. A 128-token thinking budget hit the length limit: allow sufficient reasoning tokens. Controls use `chat_template_kwargs`; no desktop thinking-mode selector was added.

Tool calling, FIM, alternative runtimes and 32K/128K/512K context are not advertised. Linux/Windows ARM64 inference and larger-context memory remain unmeasured. #32's broader qualification checklist remains open.

Sources: [model card](https://huggingface.co/ibm-granite/granite-4.2-3b), [exact GGUF](https://huggingface.co/ibm-granite/granite-4.2-3b-GGUF/tree/47a3d9699d7539606c83943d717fcea7bd9f6a19), [IBM upstream](https://github.com/ibm-granite/granite-4.2-language-models), [runtime b10603](https://github.com/ggml-org/llama.cpp/releases/tag/b10603). Non-authoritative Reddit evidence: [release](https://www.reddit.com/r/LocalLLaMA/comments/1vy2jz7/ibmgranitegranite4230b_hugging_face/), [memory](https://www.reddit.com/r/LocalLLaMA/comments/1vyfd81/ibm_granite_42_8b_is_52_gb_but_loads_into_27_gb/).

## Audio8 — #30

Implemented the separate 0.1B INT8 DualAR adapter (not the incompatible 0.6B INT4 runtime). Vendored Apache-2.0 code is pinned to `07e40f5d0b03fc473635ef378654bfb581027ac3`; license/modification notice embedded. Ungated Apache-2.0 model revision `317c12d4e0da83847b594fcf8bd74bf2c76615ec`; all 14 candidate artifacts downloaded/hashed, including external ONNX data and voice-registration files. See `catalog/candidates/audio8.json`.

ONNX Runtime 1.23.2 CPUExecutionProvider, INT8 AR, FP16 codec, pinned Python dependencies, temporary local voices, explicit consent plus exact transcript for custom references, and atomic 44.1 kHz mono PCM16 WAV output. Chunks are written incrementally; **desktop/control streaming playback is not implemented**. Limit exhaustion errors rather than silently publishing truncated speech. No DirectML/CUDA/Metal/MLX/Vulkan speech provider claim.

Fixture: `Hello from Tapioca. This speech was generated locally.`, seed 42, five threads, Python 3.12.

| CPU host | First audio including load | Elapsed | WAV duration | RTF |
| --- | --- | --- | --- | --- |
| Mac mini M4, 24 GB | 5.70 s | 21.55 s | 4.60 s | 4.69 |
| Windows x64, 64 GB | 9.06 s | 9.64 s | 0.60 s | 15.97 |

Mac peak RSS: 1.83 GB decimal. Windows output is too short for the fixture, so complete-speech qualification **fails despite successful exit and valid WAV**. RTF above 1 is slower than real time. Keep the candidate out of the picker pending comparison with the unchanged official runtime/provider numerics, multilingual intelligibility, consented voice quality, Linux inference and cancellation cleanup. No quality/performance claims from unit tests.

Sources: [model/export](https://huggingface.co/Audio8/audio8-TTS-0.1B-ONNX-INT8), [runtime](https://github.com/Audio8-AI/Audio8_TTS/tree/07e40f5d0b03fc473635ef378654bfb581027ac3/onnx_runtime_0_1b_int8). Reddit search found no credible model-specific hands-on report; reposted speed/memory claims are not evidence.

## Pocket TTS — #31

Experimental runner/dispatch uses the official CPU TTSModel API at `7809e76aa94d5e841a006a27c1b31f07c60d22d4` in a separate dependency environment. Not in the catalog. At model revision `492522650173a0653b7575cdc25ae09810e5d741`, required paths are `languages/english/model.safetensors` (219,029,196 bytes), `languages/english/tokenizer.model` (59,339), and `languages/english/embeddings/alba.safetensors` (6,194,424).

Actual download returned **401 GatedRepoError**. User must accept [access terms](https://huggingface.co/kyutai/pocket-tts) and authenticate locally, never paste tokens into issues/chat. Exact hashes and model inference remain unverified. Weights CC-BY-4.0 (retain Kyutai attribution); runtime MIT.

Runner rewrites the pinned English configuration to local paths, sets offline flags before imports, selects local Alba and incrementally writes PCM to a temporary WAV before atomic publication. Reference consent is enforced in the Python runner and Go CPU-backend contract; desktop acknowledgement follows the reference token through IPC/control. An acknowledgement is not proof of rights. Helpers test PCM, empty/malformed chunks, missing files and consent without loading weights. Remaining: authenticated provenance, checkpoint compatibility, intelligibility, reference decoding, long text, memory/platform tests and live playback.

Sources: [runtime](https://github.com/kyutai-labs/pocket-tts/tree/7809e76aa94d5e841a006a27c1b31f07c60d22d4), [release](https://kyutai.org/blog/2026-01-13-pocket-tts), [non-authoritative CPU discussion](https://www.reddit.com/r/LocalLLaMA/comments/1vzyuyv/local_tts_models_not_sure_if_this_is_the_place/).

## Repeat validation

`powershell -NoProfile -File scripts/test-windows.ps1` records hardware/commands in a unique temporary directory and runs affected checks from a checkout. Executed successfully on the authorized Windows PC: 64 GB, RTX 4070 Ti SUPER, Python 3.12.10, Node 22.20.0, Go 1.26.1. Inference/visual QA remain separate from the script.

Run full Go tests/vet, `scripts/test-model-watch.py`, Python speech unit discovery, manifest/checksum checks and desktop tests/typecheck/build. The release qualification includes actual Electron search/filter/reset/details/Escape interaction checks, not just component tests. Local test artifacts were moved to the DATA volume to avoid filling the Mac system disk; the installed user app was not replaced.
