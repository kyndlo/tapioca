# Tapioca control protocol v1

The control process reads one UTF-8 JSON request per line from standard input
and writes one JSON response or event per line to standard output. Standard
output is protocol-only. Diagnostics go to standard error.

The canonical envelope shapes are frozen in `envelope.schema.json`.
`requests.ndjson`, `responses.ndjson`, and `events.ndjson` are golden examples
for Go and TypeScript consumers.

Request IDs are unique for the lifetime of a process. Requests, methods, and job
IDs are bounded as described by the schema, and a complete line may not exceed
4 MiB. Events for one job start at sequence 1 and increase monotonically.
Progress and log event payloads are capped at 16 KiB; oversized data is replaced
with a bounded truncation marker.

A response normally echoes the nonempty request `id`. If a malformed line
cannot be decoded or validated far enough to recover a trustworthy ID, the
control process emits an uncorrelated error response with `"id":""`. This empty
ID is valid only for malformed-input errors; clients should report it as a
protocol-level error rather than associating it with an in-flight request.

Core read-only methods are `handshake`, `capabilities.get`, `health.get`,
`system.info`, `storage.info`, `catalog.list`, `catalog.get`, `installed.list`,
`server.status`, `chat.describe`, and `agent.describe`.

`health.get` reports the control-plane version, Go runtime and module build
version, process start timestamp, measured uptime in milliseconds, platform,
architecture, current timestamp, and protocol version. Development binaries
honestly report the Go build-info value `(devel)` rather than inventing a
release version.

Asynchronous operations are `model.pull`, `model.remove`, `server.start`,
`server.stop`, and `chat.request`. Supply a `job_id` to receive lifecycle,
progress, and bounded log events and to enable `job.cancel`. Only one mutating
operation runs at a time.

`model.remove` defaults to a dry run. A real removal requires
`"dry_run":false` and a `confirm` value exactly matching the installed model
name. Removed data is moved under Tapioca's recoverable trash directory before
the registry is updated.

## Feature methods

| Method | Required parameters | Result |
| --- | --- | --- |
| `system.info` | none | OS, architecture, CPU count, protocol version |
| `storage.info` | none | Tapioca home, model path, model bytes |
| `catalog.get` | `name` | one resolved catalog variant |
| `model.pull` | `name`; optional `force` | installed model record |
| `model.remove` | `name`; optional `dry_run`, `confirm` | removal plan/result |
| `server.start` | `model`; optional loopback host and ports | starting server status |
| `server.stop` | `id` | stopping server status |
| `server.status` | optional `id` | sorted server status list |
| `chat.request` | `messages`; optional model, port, sampling | typed chat completion |
| `chat.describe` | `model`; optional port | local chat endpoint descriptor |
| `agent.describe` | `agent`, `model`; optional port and args | non-executing launch descriptor |
| `creator.capabilities` | none | protocol-safe creator feature availability |
| `creator.catalog` | none | image, video, and speech-compatible catalog variants |
| `image.generate` | `model`, `prompt`; optional dimensions, inputs, LoRAs | local image file metadata |
| `video.generate` | `model`, `prompt`; optional image, dimensions, LoRAs | local MP4 file metadata |
| `speech.generate` | reserved typed request | `runtime_adapter_required` until writer-safe runtime is available |
| `voice.clone` | reserved typed request | `runtime_adapter_required` until writer-safe runtime is available |
| `lora.list` | none | installed `.safetensors` metadata |
| `lora.inspect` | `reference` | provider-neutral LoRA metadata |
| `lora.pull` | `reference`; optional `file`, `force` | installed provider LoRA record |
| `lora.import` | `path`, `base`; optional `name`, `force` | managed `local://` LoRA record |

Supported agent descriptor names are `codex`, `claude-code`, `opencode`,
`openclaw`, and `hermes`. Descriptors contain explicit executable, arguments,
environment, configuration format, endpoint, and wire protocol. The control
API never accepts a shell command.

`chat.request` also accepts bounded `tools`, `tool_choice`, and
`reasoning_format`. Responses preserve `content`, `reasoning_content`, and
structured `tool_calls`. Message content and reasoning are limited to 1 MiB,
tool arrays to 512 KiB, the result to 64 choices and 2 MiB overall. Malformed
backend tool calls return `invalid_backend_response`; oversized responses
return `backend_response_too_large`. Data is never silently dropped.

Creator jobs never place binary data in NDJSON. Image outputs are written below
`$TAPIOCA_HOME/outputs/images`; video outputs are written below
`$TAPIOCA_HOME/outputs/videos`. `output_name` must be a single filename with
the required extension. Existing outputs are never overwritten. Input images
must be regular, non-symlink files with supported extensions and bounded size.

Generation accepts at most eight LoRAs. Each reference is parsed by Tapioca's
typed adapter package, must already resolve to a local regular
`.safetensors` file, must pass base-model/backend compatibility checks, and may
use a scale from -4 through 4. The API does not auto-download a LoRA during a
generation job.

Speech and voice cloning are intentionally reported unavailable in this
protocol revision. The existing speech runtime currently binds its child
process directly to process stdout; invoking it would corrupt NDJSON. Both
methods return stable `runtime_adapter_required` until that runtime exposes the
same writer-aware adapter used by image and video.

Stable feature error codes include `invalid_params`, `model_not_found`,
`model_not_installed`, `confirmation_required`, `unsafe_model_path`,
`pull_failed`, `remove_failed`, `registry_failed`, `storage_failed`,
`server_conflict`, `server_not_found`, `chat_failed`, and
`unsupported_agent`. Creator errors add `incompatible_model`,
`image_generation_failed`, `video_generation_failed`, `output_exists`,
`output_missing`, `lora_not_found`, `lora_not_installed`,
`incompatible_lora`, `lora_discovery_failed`, `lora_inspection_failed`,
`lora_pull_failed`, `lora_import_failed`, and
`runtime_adapter_required`. Cancellation always returns `job_cancelled`.
