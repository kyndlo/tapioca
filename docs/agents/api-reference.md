# Agent API reference

Default base URL:

```text
http://127.0.0.1:11435
```

Start a server:

```bash
tapioca serve MODEL --host 127.0.0.1 --port 11435
```

Local compatibility clients may send any non-empty API key such as
`tapioca-local`. Tapioca does not require a secret for a loopback-only server.

## Health

```http
GET /health
```

Wait for a successful response before sending inference requests. A TCP
connection alone does not mean the model has finished loading.

## Models

```http
GET /v1/models
```

Use this endpoint to confirm the active served model. Refresh with
`tapioca catalog update`, then use `tapioca catalog` to discover models that
can be installed.

## OpenAI Chat Completions

```http
POST /v1/chat/completions
Content-Type: application/json
```

Supports messages, streaming, `tools`, `tool_choice`, assistant tool calls,
tool-result messages, and reasoning-capable model output.

See [chat-completions.json](examples/chat-completions.json) and
[tool-calling.json](examples/tool-calling.json).

## OpenAI Responses

```http
POST /v1/responses
Content-Type: application/json
```

Use `input` for a string or Responses-style input items. Tapioca translates
function tools and tool results to the active model server.

See [responses-api.json](examples/responses-api.json).

## Anthropic Messages

```http
POST /v1/messages
Content-Type: application/json
```

This compatibility route supports Claude Code launch flows and Anthropic-style
messages and tool-use blocks.

## Tool loop

1. Send the available function definitions with the user request.
2. If the assistant returns tool calls, validate each function name and JSON
   argument object.
3. Execute only tools allowed by the host agent's permission policy.
4. Return a tool-result message with the matching call ID.
5. Repeat until the model returns a final answer or the host's iteration limit
   is reached.

Tapioca transports tool calls; it does not execute them.
