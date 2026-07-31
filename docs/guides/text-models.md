# Text models and local APIs

## Interactive chat

```bash
tapioca run MODEL[:VARIANT]
```

Examples:

```bash
tapioca run qwen3:30b-mlx
tapioca run glm-4.7-flash:q8_0 --context 65536
```

Missing catalog models are pulled automatically. Enter `/bye` or press Ctrl-D
to stop the chat and its local server.

Reasoning is displayed separately from the final answer by default:

```bash
tapioca run glm-4.7-flash:q8_0 --show-thinking=false
```

Add `--verbose` only when diagnosing runtime startup or requests.

## Serve an API

```bash
tapioca serve glm-4.7-flash:q8_0 --port 11435 --context 65536
```

Available endpoints:

- `GET /health`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`

Tapioca suppresses llama.cpp and HTTP request logs by default. Use `--verbose`
when debugging.
