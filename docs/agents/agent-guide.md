# Tapioca for LLMs and agents

This is the canonical operating guide for an automated agent using Tapioca.
Tapioca runs text, image, video, and speech models locally and exposes
OpenAI- and Anthropic-compatible HTTP APIs.

Current agent contract: `v1`.

## Operating sequence

1. Confirm that Tapioca is installed:

   ```bash
   tapioca version
   ```

2. Inspect the machine instead of inventing model names:

   ```bash
   tapioca list
   tapioca catalog update
   tapioca catalog
   ```

3. Prefer an installed model that matches the requested task. Before a large
   download, tell the user the approximate size and memory requirement.

4. Pull a missing model only when the task needs it:

   ```bash
   tapioca pull MODEL
   ```

5. Use the narrowest execution surface:

   - `tapioca run` for a human-led terminal conversation.
   - `tapioca serve` for programmatic text and tool calls.
   - `tapioca image`, `video`, or `tts` for local media.
   - `tapioca launch` to run a supported coding agent with an isolated profile.

   Media models may be multi-file bundles. Treat the model ID as the contract;
   do not download individual weights or construct runtime graphs yourself.

6. Start an API server on loopback and wait for health:

   ```bash
   tapioca serve MODEL --host 127.0.0.1 --port 11435
   curl http://127.0.0.1:11435/health
   ```

7. Send requests to one of the supported endpoints:

   - `POST /v1/chat/completions`
   - `POST /v1/responses`
   - `POST /v1/messages`
   - `GET /v1/models`
   - `GET /health`

8. Stop servers and child agents started for the task when they are no longer
   needed.

## Rules

- Keep APIs bound to `127.0.0.1` unless the user explicitly approves network
  exposure.
- Treat `tapioca catalog` as the authority for model IDs, platforms, memory,
  GPU requirements, and backends.
- Refresh the verified catalog before declaring that a model is unavailable.
  `tapioca update --check` is read-only; do not install a binary update unless
  the user requested that system change.
- Do not claim that a model supports tools merely because it can chat. Prefer
  catalog entries described as coder or tool-capable.
- Do not overwrite a user's normal Codex, Claude Code, OpenCode, OpenClaw, or
  Hermes configuration. `tapioca launch` creates isolated launch state.
- Do not use voice cloning without confirming that the user has permission to
  use the reference voice.
- Put generated files in the user's requested workspace. Otherwise, report the
  exact output path returned by Tapioca.
- Add `--verbose` only for diagnosis. Normal commands intentionally suppress
  noisy backend logs.
- For LoRAs, inspect the Hugging Face, Civitai, or ModelScope reference before
  pulling and require the declared base architecture to match the selected
  model. Import existing files with `adapter import --base`; a `.safetensors`
  extension alone does not establish compatibility.
- Do not expose ComfyUI workflow IDs or directories to users. MiniMax-H3 uses
  a replaceable managed video engine behind Tapioca's stable CLI contract.

## References

- [HTTP API](api-reference.md)
- [Task workflows](workflows.md)
- [Safety and permissions](safety.md)
- [Video engine boundary](../concepts/video-engines.md)
- [LoRA adapters](../concepts/lora-adapters.md)
- [Coding-agent guide](../guides/coding-agents.md)
- [Command reference](../reference/commands.md)
