# LLM Gateway Domain Design (D3)

> **Source of Truth:** `openspec/specs/d3-llm-gateway/spec.md`
> **设计历史:** `openspec/archive/2026-06-07-devrix-llm-gateway/design.md`, `openspec/archive/2026-06-08-devrix-llm-gateway-v2/`

**Domain:** D3 - LLM Gateway
**DSAFT Type:** 公共域 (Common Domain)
**Version:** 2.1.0
**Status:** Active (2026-06-14)
**Last Updated:** 2026-06-14

---

## 一、架构目标

### 1.1 业务目标

| 痛点 | 目标能力 | 用户可感知结果 |
|------|----------|----------------|
| LLM 调用无熔断保护 | Circuit Breaker 保护 | 模型故障时降级，不阻塞用户 |
| 多模型切换耦合业务 | 统一模型适配器接口 | 支持 DeepSeek / MiniMax |
| Token 预算失控 | Token 计数与预算检查 | 上下文压缩触发准确 |
| 模型调用无观测 | OpenTelemetry Tracing + Metrics 内建 | 问题可诊断 |
| 恶意内容注入 LLM 请求 | Safety Filter 内容过滤 | 风险内容被拦截 |

### 1.2 技术指标

| 指标 | 目标 | 测量方式 |
|------|------|----------|
| LLM 调用延迟 | P99 < 5s | span duration (llm_latency_seconds) |
| 熔断器切换延迟 | < 10ms | circuit state event |
| Token 计数准确性 | cl100k_base ± 5% | 单元测试 |
| Provider 故障恢复 | 半开→关闭 30s | 集成测试 |
| Safety 过滤延迟 | < 1ms | Filter.Check duration |

### 1.3 层间边界

```
Layer 2 (Context Engine)              Layer 3 (LLM Gateway)
─────────────────────────            ──────────────────────
QueryLoop / adapters
    │
    └──▶ ILLMGateway.ChatStream() ──▶ Bridge (bridges/llm)
                                         │
                                         └──▶ IGateway.Stream() ──▶ Gateway
                                              ├─ Router (model→provider)
                                              ├─ ICircuitBreaker
                                              ├─ Retry.Executor
                                              ├─ adapter.Registry
                                              │   ├─ DeepSeekAdapter → OpenAIStreamClient
                                              │   └─ MiniMaxAdapter  → OpenAIStreamClient
                                              ├─ token.Counter
                                              └─ observability.Bridge (spans + metrics)
```

**禁止：LLM Gateway 不得 import contextengine/ 或 communication/ 包**

---

## 二、领域模型

### 2.1 核心类型 (contracts.go)

```go
// Request is the L3 internal chat completion input.
type Request struct {
    Provider     string
    Model        string
    SystemPrompt string
    Messages     []types.Message
    Tools        []ToolSchema
    MaxTokens    int
    Temperature  float64
    Stream       bool
}

// Chunk is a streaming LLM response fragment.
type Chunk struct {
    Content   string
    Thinking  string
    ToolCalls []ToolCall
    Done      bool
    Usage     TokenUsage
}

// TokenUsage reports token consumption from the provider.
type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    CacheReadTokens  int // prompt_tokens_details.cached_tokens
    ReasoningTokens  int // completion_tokens_details.reasoning_tokens
}

// ToolCall is an LLM-requested tool invocation (no RiskLevel; L2 fills it).
type ToolCall struct {
    ID    string
    Name  string
    Input string
}

type CircuitState string

const (
    CircuitClosed   CircuitState = "closed"
    CircuitOpen     CircuitState = "open"
    CircuitHalfOpen CircuitState = "half-open"
)

// CircuitBreakerConfig holds circuit breaker thresholds.
type CircuitBreakerConfig struct {
    FailureThreshold  int
    SuccessThreshold  int
    OpenDuration      time.Duration
    HalfOpenMaxProbes int
    Scope             string
}

// RetryConfig holds retry/backoff settings.
type RetryConfig struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Backoff      float64
}

type AdapterChunk struct {
    Raw    []byte
    Parsed *Chunk
    Error  error
}
```

---

## 二点五、Provider 配置

### 支持的 Provider

| Provider | Adapter | API 类型 | Base URL |
|----------|---------|----------|----------|
| `deepseek` | DeepSeekAdapter | OpenAI-compatible | `https://api.deepseek.com/v1` |
| `minimax` | MiniMaxAdapter | OpenAI-compatible | `https://api.minimaxi.com/v1` |

### 配置契约 (devrix.yaml)

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
```

---

## 三、接口设计

### 3.1 核心接口

```go
// IGateway streams chat completions (L3 internal API).
type IGateway interface {
    Stream(ctx context.Context, req *Request) (<-chan Chunk, error)
    ResolveTier(tier string) string
    Close() error
}

// ILLMGateway is the D2 Context Engine consumer contract.
// DSAFT: D3-S2-A01-F01 (AdaptToContextEngine)
type ILLMGateway interface {
    ChatStream(ctx context.Context, req *Request) (<-chan Chunk, error)
}

// ITierResolver resolves tier aliases to concrete model names.
// DSAFT: D3-S2-A01-F02 (ResolveTier)
type ITierResolver interface {
    ResolveTier(tier string) (string, error)
}

// IAdapter streams provider-specific responses.
type IAdapter interface {
    Stream(ctx context.Context, req *Request) (<-chan *AdapterChunk, error)
    Provider() string
}

// ICircuitBreaker protects providers from cascading failures.
type ICircuitBreaker interface {
    Allow(circuitKey string) (bool, error)
    RecordSuccess(circuitKey string)
    RecordFailure(circuitKey string)
    State(circuitKey string) CircuitState
}
```

### 3.2 配置类型 (shared/config/llmgateway.go)

```go
type LLMGatewayConfig struct {
    DefaultProvider string
    DefaultModel    string
    DefaultTier     string
    ModelTiers      map[string]string
    ModelRouting    map[string]string
    CircuitBreaker  LLMCircuitBreakerConfig
    Providers       map[string]LLMProviderRuntimeConfig
}

type LLMProviderRuntimeConfig struct {
    Type          string
    BaseURL       string
    APIKeyEnv     string
    DefaultModel  string
    FallbackModel string
    Timeout       time.Duration
    MaxTokens     int
    Temperature   float64
    Retry         LLMRetryConfig
    Headers       map[string]string
}
```

---

## 四、业务流程

### 4.1 流式对话时序 (Gateway.Stream)

```
D2 Consumer → Bridge.ChatStream()
    → IGateway.Stream()
        ├─ [span: llm.stream] 主 span
        ├─ [span: llm.provider.route] Router.Resolve(model)
        │   ├─ Tier alias → ModelTiers lookup
        │   └─ Model → model_routing pattern match → provider
        ├─ Counter.CheckBudget() [Token 预算检查]
        ├─ [span: llm.circuit_breaker] Breaker.Allow(provider)
        ├─ Registry.Get(provider) → IAdapter
        ├─ Context deadline injection (若父 ctx 无 deadline → provider.Timeout)
        ├─ [span: llm.retry] Retry.Stream()
        │   └─ For each attempt:
        │       ├─ [span: llm.adapter.stream] ad.Stream()
        │       │   ├─ buildOpenAIChatRequest → JSON body
        │       │   ├─ POST /chat/completions (SSE)
        │       │   └─ streamOpenAISSE → parse SSE → emit AdapterChunk
        │       └─ On failure: IsRetryable check → Full Jitter delay → retry/fallback
        ├─ Stream goroutine:
        │   ├─ Forward chunks to out channel
        │   ├─ Handle ctx.Done() → graceful close
        │   └─ On success: Breaker.RecordSuccess / metrics
        │   └─ On error: shouldRecordBreakerFailure? → Breaker.RecordFailure
        └─ finishStream: span end + GenAI token usage recording
```

### 4.2 熔断器状态机

```
[Initial] → Closed (正常)
    ↓ failure >= FailureThreshold (default: 5)
    Open (熔断开启，OpenDuration 后)
    ↓
    HalfOpen (探测，最多 HalfOpenMaxProbes 并发)
    ↓ success >= SuccessThreshold (default: 2) → Closed
    ↓ failure → Open
```

Key: context.Canceled 和 context.DeadlineExceeded 不触发 RecordFailure。

### 4.3 Model Tier 解析链

```
User input → Router.Resolve(model)
    ├─ model = "" → DefaultProvider + DefaultModel → ResolveTier(defaultModel)
    ├─ model = "fast" → ResolveTier("fast") → "MiniMax-M2.7-highspeed" → provider routing
    └─ model = "deepseek-v4-flash" → ResolveTier("deepseek-v4-flash") → no match → provider routing
```

---

## 五、目录结构

```
internal/layers/llmgateway/
├── contracts.go                 # 接口与核心类型定义
├── gateway/
│   ├── gateway.go              # Gateway 主实现 (Stream, startSpan, metrics)
│   ├── router.go               # Router (Resolve, ResolveTier, model_routing)
│   ├── router_test.go
│   ├── factory.go              # NewFromConfig (装配完整 stack)
│   └── gateway_test.go
├── adapter/
│   ├── registry.go             # AdapterRegistry (Register/Get)
│   ├── registry_test.go
│   ├── errors.go               # Sentinel errors
│   ├── deepseek.go             # DeepSeekAdapter
│   ├── deepseek_test.go
│   ├── minimax.go              # MiniMaxAdapter
│   ├── minimax_test.go
│   ├── openai_stream.go        # OpenAIStreamClient (HTTP POST + SSE)
│   ├── openai_request.go       # buildOpenAIChatRequest (消息映射)
│   ├── openai_request_test.go
│   ├── openai_types.go         # OpenAI API 类型定义
│   ├── sse_parser.go           # SSE 流解析 + streamAccumulator
│   └── sse_parser_test.go
├── breaker/
│   ├── circuit_breaker.go      # CircuitBreaker 实现
│   ├── circuit_breaker_test.go
│   └── state.go                # circuitRecord 状态结构
├── token/
│   ├── counter.go              # Counter (CountText, CountMessages, CountWithSystemPrompt, TruncateToTokens, CheckBudget)
│   ├── counter_test.go
│   └── bpe_loader.go           # Embedded cl100k_base BPE 加载器
├── config/
│   ├── loader.go               # Config Loader (validate + APIKey)
│   └── loader_test.go
├── retry/
│   ├── retry.go                # Retry Executor (Full Jitter + fallback)
│   ├── retry_test.go
│   └── retry_jitter_test.go
└── safety/
    ├── filter.go               # Safety Filter (Check, AddPattern)
    ├── filter_test.go
    └── patterns.go             # Default safety patterns (malware, exploit, injection, etc.)

internal/bridges/llm/
├── bridge.go                   # Bridge (IGateway → ILLMGateway + ITierResolver)
├── bridge_test.go
├── context_wiring.go           # WireContextLLM (从 yaml 到 ContextLLMStack)
├── wire.go                     # WireFromConfig (gateway.NewFromConfig + Bridge)
└── readiness.go                # Readiness probe
```

---

## 六、错误处理

| 错误码 | 类型 | 说明 | 可重试 |
|--------|------|------|--------|
| LLM_PROVIDER_1001 | ProviderUnavailable | Provider 不可用（HTTP >=300 或非 401/403） | ✅ |
| LLM_CIRCUIT_1002 | CircuitOpen | 熔断器开启 | ✅ (自动恢复) |
| LLM_TIMEOUT_1003 | Timeout | 请求超时 | ✅ |
| LLM_AUTH_1004 | AuthFailed | 认证失败 (401/403) | ❌ |
| LLM_TOKEN_1005 | TokenBudgetExceeded | Token 超限 | ❌ |
| LLM_PARSE_1006 | ParseError | SSE 响应解析错误 | ✅ |
| LLM_UNSUPPORTED_1007 | UnsupportedProvider | 不支持的 Provider | ❌ |
| LLM_UNSUPPORTED_1008 | UnsupportedModel | 不支持的 Model | ❌ |

### 可重试性判断

```go
func IsRetryable(err error) bool - 通过 SentinelError 类型判断：
  - ProviderUnavailable → true
  - Timeout → true
  - ParseError → true
  - AuthFailed → false
  - 其余 → false
```

---

## 七、Safety Filter 设计

### 7.1 默认模式

安全过滤器基于模式匹配（大小写不敏感）检查 system prompt 和 messages：

| 模式 | 严重级别 | 动作 | 位置 |
|------|---------|------|------|
| malware_generation | critical | reject | all |
| exploit_generation | critical | reject | all |
| unauthorized_access | high | reject | all |
| hardcoded_credential | medium | warn | message |
| prompt_injection | medium | warn | message |
| data_exfiltration | medium | warn | message |

### 7.2 Filter 接口

```go
type Filter struct { ... }
func NewFilter() *Filter
func (f *Filter) Check(ctx context.Context, systemPrompt string, messages []string) *Result
func (f *Filter) AddPattern(p Pattern)
```

---

## 八、可观测性

完整的 Span 注册表、Trace Tree、Metrics 定义见独立文件：
`openspec/specs/d3-llm-gateway/span-registry.md`

Gateway 通过 `observability.Bridge` 集成 OpenTelemetry，包含 5 个 Operation（llm_gateway 4 + llm_adapter 1）、3 个 Metrics 及 GenAI Token Recording。

---

## 九、测试策略

完整的测试点清单见 `openspec/specs/d3-llm-gateway/t-registry.md`（26 条：25 IMPLEMENTED + 1 PLANNED，P0 11 条）。本文档不重复列出。

---

## 十、版本分期

| 版本 | 能力 |
|------|------|
| V1 | DeepSeek + MiniMax 适配器 + Circuit Breaker + Token Counter + Retry |
| V2 | CB+Retry 协调 + Half-Open 并发限制 + Context 超时传播 + Full Jitter + CJK 补偿 |
| V2.1 | Safety Filter 内容安全 + ModelTier 层级别名 + CacheRead/Reasoning Token 分解 |
| V3 (planned) | Anthropic/OpenAI 适配器 + Rate Limiter + 多模型负载均衡 |
