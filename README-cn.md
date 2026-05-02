# completion-to-response

一个轻量级代理服务，在 OpenAI **Chat Completions API** 和 **Responses API** 格式之间进行转换 —— 让任何兼容 Chat Completions 的后端都能与期望 Responses API 的工具（如 [OpenAI Codex CLI](https://github.com/openai/codex)、Claude Code 等）配合使用。

[English](README.md)

---

## 为什么需要它？

许多 LLM 提供商（智谱/BigModel、DeepSeek、Ollama、vLLM 等）提供了 OpenAI 兼容的 **Chat Completions** 接口，但 Codex CLI 等新工具使用的是 **Responses API**。这个代理在两者之间架起桥梁 —— 两边都不需要改动代码。

```
Codex / Claude Code          completion-to-response              你的后端
       │                            │                                │
       │  POST /v1/responses        │                                │
       │  (Responses API 格式)      │                                │
       │ ─────────────────────────> │                                │
       │                            │  POST /v1/chat/completions     │
       │                            │  (Chat Completions 格式)       │
       │                            │ ─────────────────────────────> │
       │                            │                                │
       │                            │  200 (Chat Completions 响应)   │
       │                            │ <───────────────────────────── │
       │                            │                                │
       │  200 (Responses API 响应)  │                                │
       │ <───────────────────────── │                                │
```

## 功能

- **透明转换** — 请求和响应格式自动翻译
- **SSE 流式支持** — 实时 token 流式传输，格式正确转换
- **API Key 透传** — 直接使用客户端请求中的 key，无需重复配置
- **模型覆盖** — 将客户端的任意模型名映射为后端模型
- **零外部依赖** — 纯 Go 标准库，单二进制文件
- **函数调用** — 工具定义和调用在两种格式间正确转换

## 快速开始

```bash
# 编译
go build -o completion-to-response ./cmd/server

# 启动，指向任意 OpenAI 兼容后端
./completion-to-response \
  -url https://api.openai.com/v1/chat/completions \
  -model gpt-4o
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-url` | *(必填)* | 后端 Chat Completions 接口地址 |
| `-model` | `""` | 强制覆盖所有请求的模型名（忽略客户端指定的） |
| `-default-model` | `gpt-4o` | 客户端未指定模型时的默认值 |
| `-port` | `8080` | 服务监听端口 |
| `-key` | `""` | 后端 API Key（兜底；推荐通过 `Authorization` 请求头透传） |
| `-timeout` | `30s` | 后端请求超时时间 |
| `-log` | `""` | 日志写入文件（默认输出到 stderr） |

## 使用场景

### OpenAI Codex CLI

在 Codex 中配置代理，指向 Chat Completions 后端：

```bash
# 启动代理
./completion-to-response \
  -url https://open.bigmodel.cn/api/paas/v4/chat/completions \
  -model glm-4-plus \
  -log ./proxy.log

# Codex 中配置：
#   API Base URL: http://localhost:8080/v1
#   API Key: 你的智谱 API Key
#   Model: gpt-4o  （Codex 需要已知的模型名，代理通过 -model 映射）
```

### Ollama / 本地模型

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
  -url "https://你的资源名.openai.azure.com/openai/deployments/你的部署名/chat/completions?api-version=2024-08-01-preview" \
  -model gpt-4o
```

### 任意 OpenAI 兼容 API

```bash
./completion-to-response \
  -url https://your-llm-endpoint.com/v1/chat/completions \
  -model your-model-name
```

## API

### `POST /v1/responses`（也支持 `/responses`）

接受 Responses API 格式的请求体，转换后转发到 Chat Completions 后端，返回 Responses API 格式的响应。

**请求：**

```json
{
  "model": "gpt-4o",
  "input": "写一句关于独角兽的睡前故事。",
  "stream": true
}
```

多轮对话：

```json
{
  "model": "gpt-4o",
  "input": [
    { "role": "system", "content": "你是一个有用的助手。" },
    { "role": "user", "content": "法国的首都是哪里？" }
  ]
}
```

带函数调用：

```json
{
  "model": "gpt-4o",
  "input": "巴黎天气怎么样？",
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "获取指定地点的天气",
      "parameters": {
        "type": "object",
        "properties": { "location": { "type": "string" } },
        "required": ["location"]
      }
    }
  ]
}
```

**响应：**

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
          "text": "在星光的毯子下，一只困倦的独角兽踮着脚尖穿过月光草地。",
          "annotations": []
        }
      ]
    }
  ],
  "output_text": "在星光的毯子下，一只困倦的独角兽踮着脚尖穿过月光草地。",
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

## 转换对照

### 请求方向（Responses API → Chat Completions）

| Responses API | Chat Completions |
|---|---|
| `input`（字符串） | `messages` 中单个 user 消息 |
| `input`（对象数组） | `messages` 数组 |
| `instructions` | 系统消息，插入到 `messages` 最前面 |
| `developer` 角色 | 映射为 `system` 角色 |
| `[{type: "input_text", text: "..."}]` 内容格式 | 提取为纯字符串 |
| `tools`（内部标签格式） | `tools` 包装为 `{type: "function", function: {...}}` |
| `text.format`（json_schema） | `response_format` |
| `max_output_tokens` | `max_tokens` |

### 响应方向（Chat Completions → Responses API）

| Chat Completions | Responses API |
|---|---|
| `choices[].message.content` | `output[].content[]` 中 `output_text` 类型的项 |
| `choices[].message.tool_calls` | `output[]` 中 `type: "function_call"` 的项 |
| `finish_reason: "stop"` | `status: "completed"` |
| `finish_reason: "tool_calls"` | `status: "requires_action"` |
| `finish_reason: "length"` | `status: "incomplete"` |
| `usage.prompt_tokens` | `usage.input_tokens` |
| `usage.completion_tokens` | `usage.output_tokens` |

### 流式事件

后端的 SSE 数据块被转换为完整的 Responses API 流式事件序列：

```
response.created → response.in_progress → response.output_item.added →
response.content_part.added → response.output_text.delta (×N) →
response.output_text.done → response.content_part.done →
response.output_item.done → response.completed
```

## API Key

代理支持两种方式与后端认证：

1. **透传（推荐）** — 客户端发送 `Authorization: Bearer <key>`，代理直接转发给后端。只需在 Codex / Claude Code 中配置一次 key。
2. **静态 key** — 使用 `-key <你的key>` 作为兜底，当请求中没有 `Authorization` 头时使用。

## 项目结构

```
cmd/server/main.go               # HTTP 服务、路由、流式处理
internal/config/config.go         # 命令行参数解析
internal/transform/transform.go   # 核心转换逻辑
internal/transform/transform_test.go # 单元测试
internal/types/completion.go      # Chat Completions 类型定义
internal/types/response.go        # Responses API 类型定义
```

## 开发

```bash
go build -o completion-to-response ./cmd/server   # 编译
go test ./...                                      # 运行测试
go run ./cmd/server -url http://localhost:11434/v1/chat/completions
```

## License

MIT
