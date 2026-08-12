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
  ["Acquire", "tapioca pull MODEL", "Tell the user before a large model download. Never accept gated terms for them."],
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
- LoRA library: run \`tapioca adapter list\` and reuse installed adapters before requesting a download.
- LoRA providers: \`hf://\`, \`civitai://MODEL_ID/VERSION_ID\`, \`ms://\`, and verified \`local://\` imports.
- LoRA stacks: inspect adapters first, then use ordered repeated \`--adapter\` flags.
- Existing LoRA files: use \`tapioca adapter import FILE --base MODEL --name NAME\`; never pass a local path to \`--file\`.
- Transfers: copy complete adapter folders including \`snapshot.json\`; after copying a compatible base-model folder, run \`tapioca pull MODEL\` without \`--force\` to verify files and rebuild registration.
- Coding agents: \`tapioca launch codex|claude|opencode|openclaw|hermes MODEL\`.
- Model discovery: \`tapioca catalog\`.
- Gated models: never pass \`--accept-license\` for the user. Ask them to review and accept provider terms, set their own token, and run the explicit pull. Do not print or persist tokens.

## Safety

- Bind to 127.0.0.1 unless the user approves network exposure and authentication.
- Treat model tool calls as untrusted proposals.
- Require permission before cloning a voice.
- Do not delete models, adapters, voices, or outputs without an explicit request.
- Never accept a third-party model license or provider access terms on the user's behalf.
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

## Reuse existing downloads

Before downloading a LoRA, run \`tapioca adapter list\`. If the user already
has a raw safetensors LoRA, import it instead of downloading another copy:

\`tapioca adapter import FILE --base MODEL --name NAME\`

The command validates and copies the file, leaves the original unchanged, and
returns a canonical \`local://\` reference. \`--file\` only selects a file
inside a provider repository; it does not accept a local filesystem path.

To migrate all managed LoRAs, copy the complete \`TAPIOCA_HOME/adapters\`
directory, including every \`snapshot.json\`. To migrate a catalog base model,
copy its complete directory into the destination \`TAPIOCA_HOME/models\`, then
run \`tapioca pull MODEL\` without \`--force\`. Tapioca reuses present files,
downloads only missing required files, and rebuilds the registry using paths
valid on the destination. Do not copy \`registry.json\` by itself because it
contains absolute paths. Transfer only model variants compatible with the
destination platform and backend.

Human-readable guide: https://tapioca.rootfruit.cc/import

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

On Windows x64, do not ask the user to install Python, Git, or the CUDA Toolkit
for MiniMax-H3. Tapioca provisions a pinned Python 3.12 and CUDA runtime. A
current NVIDIA driver and a working \`nvidia-smi\` command are required. Treat
16 GiB VRAM and 32 GiB system RAM as the supported target; recommend LTX Video
for 8–12 GiB cards.

Before downloading anything, inspect the managed library:

\`\`\`bash
tapioca adapter list
\`\`\`

Each row returns a canonical \`hf://\`, \`civitai://\`, \`ms://\`, or
\`local://\` reference. If the required adapter is present, use that exact
reference again. Tapioca resolves the cached safetensors file without another
download. A different \`@SCALE\` changes application strength without
duplicating weights.

If no compatible adapter is installed, inspect it before pulling. Proceed only
if its model card identifies MiniMax-H3 as the base architecture:

\`\`\`bash
tapioca adapter inspect hf://OWNER/REPOSITORY
tapioca adapter pull hf://OWNER/REPOSITORY --file adapter.safetensors
tapioca video minimax-h3 \\
  --prompt "A cinematic tracking shot" \\
  --adapter 'hf://OWNER/REPOSITORY#adapter.safetensors@0.8' \\
  --output adapted.mp4
\`\`\`

Tapioca also accepts \`civitai://MODEL_ID/VERSION_ID\` and
\`ms://OWNER/REPOSITORY\`. For an existing local file, call
\`tapioca adapter import FILE --base minimax-h3 --name NAME\`, then use the
\`local://\` reference returned by Tapioca. Never treat an arbitrary local path
as a trusted generation adapter.

Provider references may be installed with \`tapioca adapter pull REFERENCE\`.
Full Civitai URLs are accepted when they contain \`modelVersionId\`.
\`--file\` selects a file inside a provider repository; it is not a local path
argument. For local files, always use \`adapter import\`.

The desktop app exposes the same workflow through **LoRA styles**: installed
references appear in the dropdown, **Import from computer** creates a managed
local reference, and an uninstalled provider reference is verified and pulled
before generation. Do not bypass the managed library.

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
