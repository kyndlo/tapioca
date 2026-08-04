# Tapioca HTTP API

Default base URL: `http://127.0.0.1:11435`.

Endpoints:

- `GET /health`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`

Wait for `/health` before inference. Use `tapioca-local` as a compatibility API
key when a client requires a non-empty value.

## Tool loop

1. Send JSON function definitions with the request.
2. Validate returned tool names and argument objects.
3. Execute only permitted host tools.
4. Return the result with the matching call ID.
5. Stop at a final answer or the host's iteration limit.

Tapioca transports tool calls but never executes them.

## Minimal request

```json
{
  "model": "glm-4.7-flash:q8_0",
  "messages": [
    {"role": "user", "content": "Say hello in one sentence."}
  ],
  "stream": false
}
```
