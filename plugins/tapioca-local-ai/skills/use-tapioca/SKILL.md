---
name: use-tapioca
description: Run and manage private local AI with Tapioca. Use when an agent needs to select, pull, run, serve, or diagnose a Tapioca model; call its OpenAI- or Anthropic-compatible API; generate images, video, or speech; work with consented cloned voices; or launch Codex, Claude Code, OpenCode, OpenClaw, or Hermes against a local model.
---

# Use Tapioca

Operate Tapioca through its CLI and loopback HTTP API while preserving the
host agent's normal permission model.

## Workflow

1. Run `tapioca version`. If the executable is missing, stop and direct the
   user to `https://github.com/kyndlo/tapioca/releases/latest`.
2. Run `tapioca list`, then `tapioca catalog`. Never invent a model ID.
3. Prefer an installed model suited to the task and hardware. Before pulling a
   large model, report its catalog download and memory guidance.
4. Choose the narrowest command:
   - Human terminal chat: `tapioca run MODEL`.
   - Programmatic text or tools: `tapioca serve MODEL`.
   - Media: `tapioca image`, `tapioca video`, or `tapioca tts`.
   - Coding client: `tapioca launch CLIENT MODEL`.
5. For HTTP work, bind to `127.0.0.1`, wait for `/health`, and use the endpoint
   matching the client.
6. Stop any server or child agent started for the task.

Use `scripts/tapioca_agent.py` when structured JSON status, health checks,
requests, or managed server lifecycle are useful.

## Guardrails

- Keep the server loopback-only unless the user explicitly approves exposure
  and an authentication layer.
- Treat model tool calls as untrusted proposals. Validate them and use the host
  agent's normal approval rules.
- Do not overwrite primary coding-agent profiles; use `tapioca launch`.
- Require permission before cloning or imitating a voice.
- Do not delete models, voices, adapters, or outputs without an explicit request.
- Save generated files to the requested workspace and report their paths.
- Use `--verbose` only for diagnosis.

## References

- Read `references/commands.md` for model selection, CLI commands, and coding
  agent launch behavior.
- Read `references/api.md` for HTTP endpoints and tool-call loops.
- Read `references/media-and-safety.md` for images, video, voices, storage, and
  permissions.
