# Third-party software and models

Tapioca's original source code, documentation, and project assets are licensed
under the Apache License, Version 2.0. Tapioca also interoperates with or
distributes third-party components under their own licenses. Those licenses are
not replaced by Tapioca's license.

## Distributed runtime components

The Audio8 CPU speech adapter includes Apache-2.0 source from
`Audio8-AI/Audio8_TTS`, pinned to commit
`07e40f5d0b03fc473635ef378654bfb581027ac3`. Its license and modification notice
are retained in `internal/speechruntime/arktts_runtime/` and embedded in the
managed runtime. The separate Pocket TTS runtime is MIT licensed; its gated
weights are CC-BY-4.0 and require Kyutai attribution and accepted access terms.

Release bundles include a pinned `llama.cpp` runtime for local text-model
inference. `llama.cpp` is licensed under the MIT License:

> MIT License
>
> Copyright (c) 2023-2026 The ggml authors
>
> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to deal
> in the Software without restriction, including without limitation the rights
> to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
> copies of the Software, and to permit persons to whom the Software is
> furnished to do so, subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in all
> copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
> SOFTWARE.

The Apple Silicon image runtime uses MLX, MLX Swift, and related MLX packages,
which are licensed under the MIT License. Tapioca Desktop uses Electron, React,
Vite, Zod, and other packages whose license metadata is recorded in
`desktop/package-lock.json`. The documentation website's dependency metadata is
recorded in `website/package-lock.json`. Packaged Electron distributions also
retain Electron and Chromium license files.

## Installed on demand

Some features install pinned third-party software into the user's local
Tapioca data directory on first use rather than incorporating it into Tapioca's
source or binaries:

- ComfyUI is licensed under GPL-3.0 and is downloaded as source from
  <https://github.com/Comfy-Org/ComfyUI>.
- `uv` is available under Apache-2.0 or MIT and is downloaded from
  <https://github.com/astral-sh/uv> on supported Windows systems.
- Python packages installed into managed environments retain the license and
  metadata supplied by their respective distributions.

Tapioca communicates with these programs through process and local API
boundaries. Any redistribution or modification of those programs must comply
with their respective licenses.

## Models and adapters

Models, checkpoints, tokenizers, voices, and LoRA adapters are not licensed
under the Tapioca license. They are downloaded from third-party providers and
remain subject to each provider's model card, repository license, access terms,
and acceptable-use requirements. Always review those terms before downloading,
modifying, or redistributing model files or generated outputs.

This notice is informational and is not legal advice. The license files shipped
with a particular dependency or model are authoritative.
