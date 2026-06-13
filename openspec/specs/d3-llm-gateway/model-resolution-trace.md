# LLM Gateway：模型名从 yaml 到 HTTP body 全链路

> 补充文档，与 [design.md](./design.md) 配套阅读。

本文追踪 Devrix 中 **模型名** 及 **provider 运行时参数**（`max_tokens`、`temperature` 等）从配置文件到出站 HTTP JSON 的完整路径。

**Last Updated:** 2026-06-14

---

## 一、链路总览

```
flowchart LR
    subgraph Config
        Y1["devrix.yaml<br/>llm_gateway"]
        Y2["devrix.yaml<br/>context_engine.compression"]
        Y3["session.Model<br/>通常为空"]
    end

    subgraph Load
        L1["config.LoadLLMGatewayConfig"]
        L2["BuildLLMGatewayConfig → DefaultLLMGatewayConfig merge"]
        L3["llmbridge.WireContextLLM"]
    end

    subgraph L2Build
        B1["sc.Model 或 autocompact.model"]
        B2["LLMRequest.Model"]
    end

    subgraph Bridge
        BR["bridge.ChatStream<br/>→ 强制 Stream=true → gw.Stream"]
    end

    subgraph Gateway
        T1["Router.ResolveTier(tier) → ModelTiers lookup"]
        R["Router.Resolve(model)"]
        G["callReq.Model = resolved<br/>callReq.MaxTokens = providerCfg.MaxTokens<br/>callReq.Temperature = providerCfg.Temperature<br/>callReq.Stream = true"]
        O["providerCfg 参数注入"]
    end

    subgraph HTTP
        H["buildOpenAIChatRequest"]
        J["JSON: model, max_tokens, temperature, stream, stream_options.include_usage, messages, tools"]
        P["POST /chat/completions"]
    end

    Y1 --> L1 --> L2 --> L3
    Y2 --> B1
    Y3 --> B1
    B1 --> B2 --> BR --> T1 --> R --> G --> O --> H --> J --> P
```

### 核心文件

| 环节 | 文件 |
|------|------|
| 配置定义 | `devrix.yaml` → `llm_gateway` |
| 配置加载 | `internal/shared/config/llmgateway.go` → `BuildLLMGatewayConfig`, `DefaultLLMGatewayConfig` |
| 配置验证 | `internal/layers/llmgateway/config/loader.go` → `Loader.validate` |
| 启动装配 | `internal/bridges/llm/context_wiring.go` → `WireContextLLM` |
| D3 装配 | `internal/bridges/llm/wire.go` → `WireFromConfig` |
| L3 类型 | `internal/layers/llmgateway/contracts.go` → `Request`, `Chunk`, `IGateway` |
| L2→L3 桥 | `internal/bridges/llm/bridge.go` → `Bridge.ChatStream` |
| Tier 解析 | `internal/layers/llmgateway/gateway/router.go` → `Router.ResolveTier` |
| 路由 | `internal/layers/llmgateway/gateway/router.go` → `Router.Resolve`, `matchRouting` |
| 编排 | `internal/layers/llmgateway/gateway/gateway.go` → `Gateway.Stream` |
| HTTP 体 | `internal/layers/llmgateway/adapter/openai_request.go` → `buildOpenAIChatRequest` |
| HTTP 出站 | `internal/layers/llmgateway/adapter/openai_stream.go` → `OpenAIStreamClient.Stream` |
| SSE 解析 | `internal/layers/llmgateway/adapter/sse_parser.go` → `streamOpenAISSE`, `streamAccumulator` |

---

## 二、L3 Request 字段与来源

`internal/layers/llmgateway/contracts.go`：

```go
type Request struct {
    Provider     string   // Gateway 路由后写入
    Model        string   // L2 传入 + Tier 解析 + 路由
    SystemPrompt string   // L2 LLMRequest.SystemPrompt
    Messages     []types.Message
    Tools        []ToolSchema
    MaxTokens    int      // Gateway 从 provider 配置覆盖
    Temperature  float64  // Gateway 从 provider 配置覆盖
    Stream       bool     // Bridge 强制 true
}
```

| 字段 | 谁决定 |
|------|--------|
| `Model`（L2 侧） | `sc.Model`（← `session.Model`）或压缩/Plan 专用 model 配置 |
| `Provider` / 最终 `Model` | `Router.Resolve`，先 `ResolveTier(tier)` 再 `matchRouting(pattern, model)` |
| `MaxTokens` / `Temperature` | `llm_gateway.providers.<provider>`，Bridge **不传**，Gateway 在 `callReq` 阶段覆盖 |
| `SystemPrompt` / `Messages` / `Tools` | Context Engine / PEVEngine |
| `API Key` / `BaseURL` | provider 配置的 `api_key_env`、`base_url`，由 `config.APIKey()` 从环境变量加载 |

Gateway 出站前补充参数（`gateway/gateway.go` `streamCall` 闭包内）：

```go
callReq := *req
callReq.Provider = provider
callReq.Model = callModel
callReq.MaxTokens = providerCfg.MaxTokens
callReq.Temperature = providerCfg.Temperature
callReq.Stream = true
```

---

## 三、路径 A：`session.Model` 为空 → 默认 MiniMax

典型生产路径：`CreateSession` 不设置 `session.Model`，`sc.Model` 为空。

| 步骤 | 位置 | 字段值 |
|------|------|--------|
| 1 | `devrix.yaml` | `default_provider: minimax`, `default_model: MiniMax-M2.7-highspeed`, `default_tier: default` |
| 2 | `BuildLLMGatewayConfig` | 合并文件配置到 `DefaultLLMGatewayConfig` |
| 3 | `WireContextLLM` | 调用 `WireFromConfig` → `gateway.NewFromConfig` |
| 4 | `memory.LoadOrInit` | `sc.Model = session.Model` → **空串** |
| 5 | PEVEngine / QueryLoop | `LLMRequest.Model = ""` |
| 6 | `Bridge.ChatStream` | `internal.Stream = true`，调用 `gw.Stream` |
| 7 | `Router.Resolve("")` | 空 model → `DefaultProvider`="minimax" → `provider.DefaultModel`="MiniMax-M2.7-highspeed" |
| 8 | `Router.ResolveTier` 对 defaultModel 调用 | `ModelTiers["MiniMax-M2.7-highspeed"]` 无匹配 → 保持原值 |
| 9 | `Gateway.Stream` `streamCall` | `callReq.Model = "MiniMax-M2.7-highspeed"` |
| 10 | provider 配置 | `max_tokens=8192`, `temperature=0.7`, `base_url=https://api.minimaxi.com/v1` |
| 11 | `buildOpenAIChatRequest` | `Stream: true`, `StreamOptions: {IncludeUsage: true}` |
| 12 | HTTP JSON | `"model":"MiniMax-M2.7-highspeed"`, `"stream":true`, `"stream_options":{"include_usage":true}` |

`Router.Resolve` 空 model 逻辑（`gateway/router.go`）：

```go
if model == "" {
    provider = r.cfg.DefaultProvider
    resolvedModel = r.defaultModelFor(provider)
    if resolvedModel == "" {
        return "", "", sharederrors.NewUnsupportedModelError(model)
    }
    resolvedModel = r.ResolveTier(resolvedModel)
    return provider, resolvedModel, nil
}
```

---

## 四、路径 B：Tier 别名 "fast" → MiniMax

| 步骤 | 位置 | 字段值 |
|------|------|--------|
| 1 | `devrix.yaml` | `model_tiers: { fast: "MiniMax-M2.7-highspeed" }` |
| 2 | L2 设置 | `LLMRequest.Model = "fast"` |
| 3 | `Router.Resolve("fast")` | `ResolveTier("fast")` → `ModelTiers["fast"]` → "MiniMax-M2.7-highspeed" |
| 4 | `matchRouting("MiniMax-M2.7-highspeed")` | 匹配 `MiniMax-*` → provider="minimax" |
| 5 | `callReq` | model="MiniMax-M2.7-highspeed" |
| 6 | HTTP JSON | `"model":"MiniMax-M2.7-highspeed"` |

---

## 五、路径 C：显式 `deepseek-v4-flash`（压缩摘要）

| 步骤 | 位置 | 字段值 |
|------|------|--------|
| 1 | `devrix.yaml` | `context_engine.compression.autocompact.model: deepseek-v4-flash` |
| 2 | `model_routing` | `"deepseek-*" → deepseek` |
| 3 | `Router.Resolve("deepseek-v4-flash")` | `ResolveTier` 无匹配 → `matchRouting` 匹配 `deepseek-*` → provider=`deepseek`, model=`deepseek-v4-flash` |
| 4 | `callReq` | `MaxTokens`, `Temperature` 从 `providers.deepseek` 取；fallback=`deepseek-v4-pro` |
| 5 | HTTP JSON | `"model":"deepseek-v4-flash"`, `"max_tokens":8192`, `"temperature":0.7` |

Plan 阶段类似，Model 来自 `context_engine.plan.model`（如 `deepseek-v4`），经 Bridge → Gateway.Stream。

---

## 六、最终 HTTP body 结构

由 `buildOpenAIChatRequest`（`adapter/openai_request.go`）生成：

```json
{
  "model": "deepseek-v4-flash",
  "max_tokens": 8192,
  "temperature": 0.7,
  "stream": true,
  "stream_options": { "include_usage": true },
  "messages": [
    { "role": "system", "content": "<LLMRequest.SystemPrompt>" },
    { "role": "user", "content": "用户消息…" },
    { "role": "assistant", "content": "…", "tool_calls": [...] },
    { "role": "tool", "tool_call_id": "call_…", "content": "…" }
  ],
  "tools": [
    { "type": "function", "function": { "name": "read_file", "parameters": {...} } }
  ]
}
```

- `messages` / `tools`：来自 L2 的 `Messages` / `Tools`（经 `mapOpenAIMessage` 映射）。
  - `tool_calls` 和 `tool_call_id` 从 `Message.Metadata` 中提取
  - 空 `tool_call_id` 的 tool 消息会被跳过（避免 MiniMax 400 错误）
  - 空 `tool_call.id` 会被 `sanitizeOpenAIToolCalls` 自动填充
- `model` / `max_tokens` / `temperature`：Gateway 路由 + provider 配置最终决定。
- `stream_options.include_usage`：强制 true，确保 provider 在 [DONE] 前发送 usage 帧。
- 请求 URL：`{base_url}/chat/completions`，鉴权：`Authorization: Bearer {api_key_env 环境变量}`。

---

## 七、失败重试时的 model 变化

`Gateway.Stream` 内 `retry.Stream` 使用 primary model 与 `providerCfg.FallbackModel`：

| Provider | 主模型 (DefaultModel) | Fallback |
|----------|----------------------|----------|
| minimax | `MiniMax-M2.7-highspeed` | `MiniMax-M2.5-highspeed` |
| deepseek | `deepseek-v4-flash` | `deepseek-v4-pro` |

重试时 `callReq.Model` 换成 fallback，HTTP body 中的 `"model"` 随之改变。
退避延迟 = `rand.Int63n(min(initialDelay * backoff^attempt, maxDelay))`（Full Jitter）。

---

## 八、配置参考（devrix.yaml 片段）

```yaml
llm_gateway:
  default_provider: "minimax"
  default_model: "MiniMax-M2.7-highspeed"
  default_tier: "default"

  model_tiers:
    fast: "MiniMax-M2.7-highspeed"
    default: "MiniMax-M2.7-highspeed"
    powerful: "deepseek-v4-latest"

  model_routing:
    "deepseek-*": deepseek
    "minimax-*": minimax
    "MiniMax-*": minimax

  circuit_breaker:
    failure_threshold: 5
    success_threshold: 2
    open_duration: "30s"
    half_open_max_probes: 1
    scope: "provider"

  providers:
    deepseek:
      type: "deepseek"
      base_url: "https://api.deepseek.com/v1"
      api_key_env: "DEEPSEEK_API_KEY"
      default_model: "deepseek-v4-flash"
      fallback_model: "deepseek-v4-pro"
      timeout: "60s"
      max_tokens: 8192
      temperature: 0.7
      retry:
        max_attempts: 3
        initial_delay: "1s"
        max_delay: "10s"
        backoff: 2.0

    minimax:
      type: "minimax"
      base_url: "https://api.minimaxi.com/v1"
      api_key_env: "MINIMAX_API_KEY"
      default_model: "MiniMax-M2.7-highspeed"
      fallback_model: "MiniMax-M2.5-highspeed"
      timeout: "60s"
      max_tokens: 8192
      temperature: 0.7
      retry:
        max_attempts: 3
        initial_delay: "1s"
        max_delay: "10s"
        backoff: 2.0

context_engine:
  compression:
    autocompact:
      model: "deepseek-v4-flash"
  plan:
    model: "deepseek-v4"
```

---

## 九、相关文档

- [design.md](./design.md) — D3 LLM Gateway 架构设计（SoT 见 `spec.md`）
- [spec.md](./spec.md) — D3 域规范
- [../d2-context-engine/design.md](../d2-context-engine/design.md) — D2 上下文引擎设计
