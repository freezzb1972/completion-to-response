# completion-to-response

[中文文档](README-cn.md)

A lightweight proxy server that translates between OpenAI **Chat Completions API** and **Responses API** formats — enabling any Chat Completions-compatible backend to work with tools built for the Responses API (e.g. [OpenAI Codex CLI](https://github.com/openai/codex), Claude Code, and others).

---

## Why?

Many LLM providers (Zhipu/BigModel, DeepSeek, Ollama, vLLM, etc.) expose an OpenAI-compatible **Chat Completions** endpoint, but newer tools like Codex CLI expect the **Responses API**. This proxy bridges that gap — no code changes needed on either side.

```
Codex / Claude Code          completion-to-response              Your Backend
       │                            │                                │
       │  POST /v1/responses        │                                │
       │  (Responses API format)    │                                │
       │ ─────────────────────────> │                                │
       │                            │  POST /v1/chat/completions     │
       │                            │  (Chat Completions format)     │
       │                            │ ─────────────────────────────> │
       │                            │                                │
       │                            │  200 (Chat Completions resp)   │
       │                            │ <───────────────────────────── │
       │                            │                                │
       │  200 (Responses API resp)  │                                │
       │ <───────────────────────── │                                │
```

## Features

- **Transparent conversion** — request and response formats are translated automatically
- **Streaming (SSE) support** — real-time token streaming, properly converted
- **API key passthrough** — use the key from the client request, no duplicate config
- **Model override** — map any client model name to your backend model
- **Zero dependencies** — pure Go stdlib, single binary
- **Function calling** — tool definitions and tool calls are converted between formats

## Quick Start

### Download Binary

Grab the latest release for your platform from [GitHub Releases](https://github.com/NoahStepheno/completion-to-response/releases/latest):

```bash
# macOS (Apple Silicon)
curl -L -o completion-to-response https://github.com/NoahStepheno/completion-to-response/releases/latest/download/completion-to-response-darwin-arm64

# macOS (Intel)
curl -L -o completion-to-response https://github.com/NoahStepheno/completion-to-response/releases/latest/download/completion-to-response-darwin-amd64

# Linux (x86_64)
curl -L -o completion-to-response https://github.com/NoahStepheno/completion-to-response/releases/latest/download/completion-to-response-linux-amd64

# Linux (ARM64)
curl -L -o completion-to-response https://github.com/NoahStepheno/completion-to-response/releases/latest/download/completion-to-response-linux-arm64

# Windows (x86_64)
curl -L -o completion-to-response.exe https://github.com/NoahStepheno/completion-to-response/releases/latest/download/completion-to-response-windows-amd64.exe

chmod +x completion-to-response
```

### Or Build from Source

```bash
git clone https://github.com/NoahStepheno/completion-to-response.git
cd completion-to-response
go build -o completion-to-response ./cmd/server
```

### Run

```bash
./completion-to-response \
  -url https://api.openai.com/v1/chat/completions \
  -model gpt-4o
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | *(required)* | Backend Chat Completions endpoint URL |
| `-model` | `""` | Override model name for all requests (ignores client) |
| `-default-model` | `gpt-4o` | Fallback model when client doesn't specify one |
| `-port` | `8080` | Server listen port |
| `-key` | `""` | Backend API key (fallback; prefer passthrough via `Authorization` header) |
| `-timeout` | `30s` | Backend request timeout |
| `-log` | `""` | Write logs to file instead of stderr |

## Use Cases

### OpenAI Codex CLI

Configure Codex to point at the proxy with a Chat Completions backend:

```bash
# Start the proxy
./completion-to-response \
  -url https://open.bigmodel.cn/api/paas/v4/chat/completions \
  -model glm-4-plus \
  -log ./proxy.log

# In Codex, set:
#   API Base URL: http://localhost:8080/v1
#   API Key: your-backend-api-key
#   Model: gpt-4o  (Codex needs a known name, proxy maps it via -model)
```

### Ollama / Local Models

```bash
./completion-to-response \
  -url http://localhost:11434/v1/chat/completions \
  -model llama3 \
  -port 9090
```

### DeepSeek

```bash
./completion-to-response \
  -url https://api.deepseek.com/v1/chat/completions \
  -model deepseek-chat
```

### Azure OpenAI

```bash
./completion-to-response \
  -url "https://YOUR_RESOURCE.openai.azure.com/openai/deployments/YOUR_DEPLOY/chat/completions?api-version=2024-08-01-preview" \
  -model gpt-4o
```

### Any OpenAI-Compatible API

```bash
./completion-to-response \
  -url https://your-llm-endpoint.com/v1/chat/completions \
  -model your-model-name
```

## API

### `POST /v1/responses` (also `/responses`)

Accepts a Responses API request body, converts and forwards to the Chat Completions backend, then returns the response in Responses API format.

**Request:**

```json
{
  "model": "gpt-4o",
  "input": "Write a one-sentence bedtime story about a unicorn.",
  "stream": true
}
```

Multi-turn:

```json
{
  "model": "gpt-4o",
  "input": [
    { "role": "system", "content": "You are a helpful assistant." },
    { "role": "user", "content": "What is the capital of France?" }
  ]
}
```

With function calling:

```json
{
  "model": "gpt-4o",
  "input": "What's the weather in Paris?",
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get weather for a location",
      "parameters": {
        "type": "object",
        "properties": { "location": { "type": "string" } },
        "required": ["location"]
      }
    }
  ]
}
```

**Response:**

```json
{
  "id": "resp_a1b2c3d4...",
  "object": "response",
  "created_at": 1234567890,
  "model": "glm-4-plus",
  "status": "completed",
  "output": [
    {
      "id": "msg_e5f6a7b8...",
      "type": "message",
      "status": "completed",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Under a blanket of starlight, a sleepy unicorn tiptoed through moonlit meadows.",
          "annotations": []
        }
      ]
    }
  ],
  "output_text": "Under a blanket of starlight, a sleepy unicorn tiptoed through moonlit meadows.",
  "usage": {
    "input_tokens": 15,
    "output_tokens": 20,
    "total_tokens": 35
  }
}
```

### `GET /health`

```json
{ "status": "ok" }
```

## How Conversion Works

### Request (Responses API → Chat Completions)

| Responses API | Chat Completions |
|---|---|
| `input` (string) | `messages` with single user message |
| `input` (array of items) | `messages` array |
| `instructions` | System message prepended to `messages` |
| `developer` role | Mapped to `system` role |
| `[{type: "input_text", text: "..."}]` content | Extracted to plain string |
| `tools` (internally tagged) | `tools` with `{type: "function", function: {...}}` wrapper |
| `text.format` (json_schema) | `response_format` |
| `max_output_tokens` | `max_tokens` |

### Response (Chat Completions → Responses API)

| Chat Completions | Responses API |
|---|---|
| `choices[].message.content` | `output[].content[]` with `output_text` items |
| `choices[].message.tool_calls` | `output[]` items with `type: "function_call"` |
| `finish_reason: "stop"` | `status: "completed"` |
| `finish_reason: "tool_calls"` | `status: "requires_action"` |
| `finish_reason: "length"` | `status: "incomplete"` |
| `usage.prompt_tokens` | `usage.input_tokens` |
| `usage.completion_tokens` | `usage.output_tokens` |

### Streaming Events

Backend SSE chunks are converted to the full Responses API streaming event sequence:

```
response.created → response.in_progress → response.output_item.added →
response.content_part.added → response.output_text.delta (×N) →
response.output_text.done → response.content_part.done →
response.output_item.done → response.completed
```

## API Key

The proxy supports two ways to authenticate with the backend:

1. **Passthrough (recommended)** — The client sends `Authorization: Bearer <key>`, the proxy forwards it to the backend. Configure your key once in Codex / Claude Code.
2. **Static key** — Use `-key <your-key>` as a fallback when no `Authorization` header is present.

## Project Structure

```
cmd/server/main.go               # HTTP server, routing, streaming
internal/config/config.go         # CLI flag parsing
internal/transform/transform.go   # Core conversion logic
internal/transform/transform_test.go # Unit tests
internal/types/completion.go      # Chat Completions types
internal/types/response.go        # Responses API types
```

## Development

```bash
go build -o completion-to-response ./cmd/server   # Build
go test ./...                                      # Run tests
go run ./cmd/server -url http://localhost:11434/v1/chat/completions
```

## License

MIT
