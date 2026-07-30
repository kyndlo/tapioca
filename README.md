# Tapioca

Tapioca is a small local-model runtime for coding agents. It downloads GGUF
models, starts `llama-server` with Metal acceleration, and presents compatible
APIs to Codex, Claude Code, and OpenCode.

## Requirements

- macOS on Apple Silicon
- Go 1.25 or newer (to build)
- llama.cpp: `brew install llama.cpp`
- The client CLI you want to launch (`codex`, `claude`, `opencode`,
  `openclaw`, or `hermes`)

## Build

```bash
make build
```

GitHub Actions builds downloadable artifacts for:

- macOS on Apple Silicon (`darwin/arm64`)
- Windows x64 (`windows/amd64`)
- Windows ARM64 (`windows/arm64`)

## Quick start

```bash
./bin/tapioca pull glm-4.7-flash:q8_0
./bin/tapioca run glm-4.7-flash:q8_0
```

Launch a coding agent:

```bash
./bin/tapioca launch codex glm-4.7-flash:q8_0
./bin/tapioca launch claude glm-4.7-flash:q8_0
./bin/tapioca launch opencode glm-4.7-flash:q8_0
./bin/tapioca launch openclaw glm-4.7-flash:q8_0
./bin/tapioca launch hermes glm-4.7-flash:q8_0
```

OpenClaw and Hermes run with isolated configuration under
`~/.tapioca/launch/`; Tapioca does not overwrite their normal user profiles.
If Hermes is not installed, use its official installer before launching it:

```bash
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
```

Or expose the APIs directly:

```bash
./bin/tapioca serve glm-4.7-flash:q8_0 --context 65536
```

Endpoints:

- `GET /health`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`

Tapioca stores models and generated launcher configuration in `~/.tapioca`.
Override this with `TAPIOCA_HOME`.

## MVP limitations

- The built-in catalog currently contains GLM-4.7-Flash Q4_K_M and Q8_0.
- Responses and Anthropic streaming are compatibility streams produced after
  generation completes. True token-by-token translation is planned.
- The launcher owns a server for the lifetime of its child coding-agent process.
- Tool-call quality depends on the model's embedded chat template and current
  llama.cpp Jinja support.
