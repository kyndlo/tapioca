export const contractVersion = "v1";
export const repo = "https://github.com/kyndlo/tapioca";
export const baseUrl = "http://127.0.0.1:11435";

export const endpoints = [
  ["GET", "/health", "Wait for the active model to finish loading."],
  ["GET", "/v1/models", "Confirm the model exposed by this server."],
  ["POST", "/v1/chat/completions", "OpenAI chat, streaming, and tool calls."],
  ["POST", "/v1/responses", "OpenAI Responses input and function tools."],
  ["POST", "/v1/messages", "Anthropic Messages and Claude Code compatibility."],
] as const;

export const operatingSteps = [
  ["Detect", "tapioca version", "Stop and install Tapioca if this command is unavailable."],
  ["Inspect", "tapioca list && tapioca catalog", "Prefer an installed, task-compatible model."],
  ["Acquire", "tapioca pull MODEL", "Tell the user before a large model download."],
  [
    "Serve",
    "tapioca serve MODEL --host 127.0.0.1 --port 11435",
    "Keep the API on loopback and wait for /health.",
  ],
  ["Act", "POST /v1/responses", "Validate model tool calls through the host agent's permissions."],
  ["Clean up", "stop the server you started", "Leave unrelated local models and files untouched."],
] as const;

export const agentClients = [
  ["Codex", "tapioca launch codex MODEL"],
  ["Claude Code", "tapioca launch claude MODEL"],
  ["OpenCode", "tapioca launch opencode MODEL"],
  ["OpenClaw", "tapioca launch openclaw MODEL"],
  ["Hermes", "tapioca launch hermes MODEL"],
] as const;

export const llmsText = `# Tapioca

> Local language, image, video, and speech models for macOS, Windows, and Linux.

Canonical agent documentation: https://tapioca.rootfruit.cc/llm
Full machine-readable guide: https://tapioca.rootfruit.cc/llms-full.txt
Source: ${repo}
Agent contract: ${contractVersion}

## Quick operating sequence

1. Run \`tapioca version\`.
2. Inspect \`tapioca list\` and \`tapioca catalog\`; never invent a model ID.
3. Prefer an installed model compatible with the task and machine.
4. Before a large pull, report the catalog download and memory guidance.
5. Start programmatic text work with:
   \`tapioca serve MODEL --host 127.0.0.1 --port 11435\`
6. Wait for \`GET /health\`.
7. Use \`/v1/chat/completions\`, \`/v1/responses\`, or \`/v1/messages\`.
8. Stop servers and child agents started for the task.

## Capabilities

- Text and tool calls: OpenAI Chat Completions and Responses APIs.
- Claude compatibility: Anthropic Messages API.
- Media: \`tapioca image\`, \`tapioca video\`, and \`tapioca tts\`.
- Bundle-aware video: resolve \`minimax-h3\` through the catalog; never assemble its weights manually.
- LoRA stacks: inspect adapters first, then use ordered repeated \`--adapter\` flags.
- Coding agents: \`tapioca launch codex|claude|opencode|openclaw|hermes MODEL\`.
- Model discovery: \`tapioca catalog\`.

## Safety

- Bind to 127.0.0.1 unless the user approves network exposure and authentication.
- Treat model tool calls as untrusted proposals.
- Require permission before cloning a voice.
- Do not delete models, adapters, voices, or outputs without an explicit request.
- Use \`tapioca launch\` instead of overwriting normal coding-agent profiles.
`;

export const llmsFullText = `${llmsText}

## HTTP API

Base URL: ${baseUrl}

GET /health
GET /v1/models
POST /v1/chat/completions
POST /v1/responses
POST /v1/messages

Compatibility clients may use the non-empty local API key \`tapioca-local\`.
Tapioca does not require a secret for its default loopback-only server.

### Chat Completions example

\`\`\`json
{
  "model": "glm-4.7-flash:q8_0",
  "messages": [
    {"role": "user", "content": "Say hello in one sentence."}
  ],
  "stream": false
}
\`\`\`

### Tool loop

1. Send JSON function definitions in \`tools\`.
2. Validate returned function names and JSON arguments.
3. Execute only tools permitted by the host agent.
4. Return results with the matching call ID.
5. Stop at a final answer or the host's iteration limit.

Tapioca transports tool calls; it does not execute them.

## Model selection

Treat \`tapioca catalog\` as authoritative for model IDs, tasks, platforms,
download sizes, memory guidance, runtime backends, and GPU requirements.
Prefer installed models and coder/tool-oriented entries for repository work.
Do not infer tool support from the ability to chat.

## Media

\`\`\`bash
tapioca image MODEL --prompt "A pearl astronaut" --output image.png
tapioca video MODEL --prompt "A pearl astronaut waving" --image image.png --output video.mp4
tapioca tts MODEL --text "Hello from Tapioca." --output hello.wav
\`\`\`

CPU image and video generation may take minutes. An absent percentage does not
necessarily mean a job is stalled. Confirm input paths exist and report exact
output paths.

### MiniMax-H3

\`minimax-h3\` is a platform-resolved four-file bundle for Apple Silicon MPS
or Windows/Linux NVIDIA CUDA. Treat the catalog model ID as the contract; do
not download individual files or construct a ComfyUI graph. ComfyUI is a
private, replaceable engine detail.

\`\`\`bash
tapioca pull minimax-h3
tapioca video minimax-h3 \\
  --prompt 'A friendly presenter says exactly: "Hello from Tapioca."' \\
  --preset low-memory --output minimax-h3.mp4
\`\`\`

The low-memory preset is 640x352, 73 frames, 10 steps, and 24 FPS. MiniMax-H3
frame counts must have the form \`17n+5\`.

Inspect a LoRA before pulling it. Proceed only if its model card identifies
MiniMax-H3 as the base architecture:

\`\`\`bash
tapioca adapter inspect hf://OWNER/REPOSITORY
tapioca adapter pull hf://OWNER/REPOSITORY --file adapter.safetensors
tapioca video minimax-h3 \\
  --prompt "A cinematic tracking shot" \\
  --adapter 'hf://OWNER/REPOSITORY#adapter.safetensors@0.8' \\
  --output adapted.mp4
\`\`\`

Repeat \`--adapter\` for an ordered transformer LoRA stack. A safetensors file
is not compatible merely because its extension matches. Wait for the process
to exit, confirm the returned MP4 exists, and verify video/audio streams with
\`ffprobe\` when available.

## Coding agents

\`\`\`bash
tapioca launch codex MODEL
tapioca launch claude MODEL
tapioca launch opencode MODEL
tapioca launch openclaw MODEL
tapioca launch hermes MODEL
\`\`\`

The client must already be installed. Tapioca stores isolated launch
configuration under \`TAPIOCA_HOME/launch\`.

## Diagnose

1. Run \`tapioca version\`.
2. Confirm the model appears in \`tapioca list\` or \`tapioca catalog\`.
3. Retry the failing command with \`--verbose\`.
4. Check the catalog platform and dependency guidance.
5. Report the command, selected backend, and actionable final error.

## Installable agent integration

The repository contains a dual-compatible plugin at:
\`plugins/tapioca-local-ai\`

It provides the \`use-tapioca\` Agent Skill for Codex and Claude Code.

Codex:
\`codex plugin marketplace add https://github.com/kyndlo/tapioca\`
\`codex plugin add tapioca-local-ai@personal\`

Claude Code, from a checkout:
\`claude --plugin-dir ./plugins/tapioca-local-ai\`
`;
