# Use local models with coding agents

Tapioca can launch Codex, Claude Code, OpenCode, OpenClaw, or Hermes against a
local model endpoint.

```bash
tapioca launch codex glm-4.7-flash:q8_0
tapioca launch claude glm-4.7-flash:q8_0
tapioca launch opencode qwen3-coder:30b-mlx
tapioca launch openclaw qwen3-coder:30b-mlx
tapioca launch hermes qwen3-coder:30b-mlx
```

The selected client must already be installed and available on `PATH`.
Tapioca provides the model server but does not bundle these clients.

Pass client-specific arguments after `--`:

```bash
tapioca launch opencode qwen3-coder:30b-mlx -- --help
```

OpenClaw and Hermes use isolated configuration inside
`TAPIOCA_HOME/launch`; Tapioca does not overwrite their normal profiles.

If Hermes is not installed:

```bash
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
```

Tool-use quality depends on the selected model, its chat template, and the
runtime's tool-call support. Prefer a coder or tool-oriented model.
