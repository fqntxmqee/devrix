# LLM Gateway Layer Design (Layer 3)

> **Source of Truth:** `openspec/specs/d3-llm-gateway/spec.md`（设计历史见 `openspec/archive/2026-06-07-devrix-llm-gateway/design.md`）

**Change ID:** devrix-llm-gateway
**Demand:** DM-20260607-004
**Layer:** 3 - LLM Gateway
**Status:** S7 Archived (2026-06-07) — 层边界仍有效；Process 调用方现为 **QueryLoop**（非 PEVEngine）  
**架构入口:** [architecture/request-flow.md](./architecture/request-flow.md)

---

## 一、架构目标

### 1.1 业务目标

| 痛点 | 目标能力 | 用户可感知结果 |
|------|----------|----------------|
| LLM 调用无熔断保护 | Circuit Breaker 保护 | 模型故障时降级，不阻塞用户 |
| 多模型切换耦合业务 | 统一模型适配器接口 | 支持 DeepSeek / MiniMax |
| Token 预算失控 | Token 计数与预算检查 | 上下文压缩触发准确 |
| 模型调用无观测 | Tracing + Metrics 内建 | 问题可诊断 |

### 1.2 技术指标

| 指标 | V1 目标 | 测量方式 |
|------|---------|----------|
| LLM 调用延迟 | P99 < 5s | span duration |
| 熔断器切换延迟 | < 10ms | circuit state event |
| Token 计数准确性 | cl100k_base ± 5% | 单元测试 |
| 并发模型调用 | 单进程 100 实例 | 压力测试 |
| Provider 故障恢复 | 半开→关闭 30s | 集成测试 |

### 1.3 层间边界

```
Layer 2 (Context Engine)        Layer 3 (LLM Gateway)
─────────────────────────      ──────────────────────
QueryLoop.Run() / adapters
    │
    └──▶ ILLMGateway.ChatStream() ──▶ LLMGateway
                                      ├─ AdapterRegistry
                                      │   ├─ DeepSeekAdapter
                                      │   └─ MiniMaxAdapter
                                      ├─ CircuitBreaker
                                      ├─ TokenCounter
                                      └─ ConfigLoader
```

**禁止：LLM Gateway 不得 import contextengine/ 或 communication/ 包**

---

## 二、领域模型

### 2.1 核心类型

```go
// contracts.go

type LLMRequest struct {
    Model        string            // 用户配置的模型名
    Provider     string            // deepseek | minimax
    SystemPrompt string
    Messages     []types.Message
    Tools        []ToolSchema
    MaxTokens    int
    Temperature  float64
    Stream       bool
}

type LLMChunk struct {
    Content   string
    Thinking  string
    ToolCalls []ToolCall
    Done      bool
    Usage     TokenUsage
}

type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}

type ToolSchema struct {
    Name        string
    Description string
    Parameters  string // JSON Schema
}

type ToolCall struct {
    ID    string
    Name  string
    Input string // JSON string
}

type CircuitState string

const (
    CircuitClosed   CircuitState = "closed"
    CircuitOpen    CircuitState = "open"
    CircuitHalfOpen CircuitState = "half-open"
)

type CircuitBreakerConfig struct {
    FailureThreshold  int
    SuccessThreshold  int
    OpenDuration     time.Duration
}

type RetryConfig struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Backoff      float64
}
```

---

## 三、Provider 配置

### 3.1 支持的 Provider

| Provider | Adapter | API 类型 | Base URL |
|----------|---------|----------|----------|
| `deepseek` | DeepSeekAdapter | OpenAI-compatible | `https://api.deepseek.com/v1` |
| `minimax` | MiniMaxAdapter | OpenAI-compatible | `https://api.minimax.io/v1` |

**注意**: MiniMax 国内站可配置为 `https://api.minimaxi.chat/v1`

### 3.2 配置契约

```yaml
llm_gateway:
  default_provider: "minimax"
  default_model: ""  # 用户配置
  
  circuit_breaker:
    failure_threshold: 5
    success_threshold: 2
    open_duration: "30s"
  
  providers:
    deepseek:
      type: "deepseek"
      base_url: "https://api.deepseek.com/v1"
      api_key_env: "DEEPSEEK_API_KEY"
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
      base_url: "https://api.minimax.io/v1"
      api_key_env: "MINIMAX_API_KEY"
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

## 四、接口设计

### 4.1 核心接口

```go
// ILLMGateway LLM 网关接口 (被 L2 依赖)
type ILLMGateway interface {
    ChatStream(ctx context.Context, req *LLMRequest) (<-chan LLMChunk, error)
    ChatComplete(ctx context.Context, req *LLMRequest) (*LLMChunk, error)
    Close() error
}

// IAdapter 模型适配器接口
type IAdapter interface {
    Stream(ctx context.Context, req *LLMRequest) (<-chan *AdapterChunk, error)
    Provider() string
    Supports(provider string) bool
}

type AdapterChunk struct {
    Raw    []byte
    Parsed *LLMChunk
    Error  error
}

// ICircuitBreaker 熔断器接口
type ICircuitBreaker interface {
    Allow(circuitKey string) (bool, error)
    RecordSuccess(circuitKey string)
    RecordFailure(circuitKey string)
    State(circuitKey string) CircuitState
    Reset(circuitKey string)
}

// ITokenCounter Token 计数接口
type ITokenCounter interface {
    Count(messages []types.Message) int
    CountWithPrompt(prompt string, messages []types.Message) int
    EstimateRemaining(current, max int) int
}

// ILLMObserver LLM 可观测接口
type ILLMObserver interface {
    EmitLLMCall(provider, model string, duration time.Duration, success bool)
    EmitTokenUsage(provider, model string, usage TokenUsage)
    EmitCircuitState(provider string, state CircuitState)
}

// IHealthCheck 健康检查接口
type IHealthCheck interface {
    Check(ctx context.Context, provider string) error
    CheckAll(ctx context.Context) map[string]error
}
```

### 4.2 配置接口

```go
type LLMConfig struct {
    DefaultProvider string
    Providers       map[string]ProviderConfig
    TokenLimit     int
    CircuitBreaker CircuitBreakerConfig
}

type ProviderConfig struct {
    Type        string
    BaseURL     string
    APIKeyEnv   string
    Timeout     time.Duration
    MaxTokens   int
    Temperature float64
    Retry       RetryConfig
    Headers     map[string]string
}
```

---

## 五、业务流程

### 5.1 流式对话时序

```
ContextEngine → LLMGateway.ChatStream()
    → TokenCounter.Count() [检查预算]
    → CircuitBreaker.Allow(provider) [检查熔断]
    → AdapterRegistry.GetAdapter(provider)
    → Adapter.Stream()
        loop: chunk → emit → yield
    → CircuitBreaker.RecordResult()
    → TokenCounter.UpdateUsage()
```

### 5.2 熔断器状态机

```
[Initial] → Closed (正常)
    ↓ failure >= threshold
    Open (熔断开启，30s 后)
    ↓
    HalfOpen (探测)
    ↓ success >= 2 → Closed
    ↓ failure → Open
```

---

## 六、目录结构

```
internal/layers/llmgateway/
├── contracts.go              # 接口与类型定义
├── gateway/
│   ├── gateway.go           # LLMGateway 主实现
│   ├── options.go            # Gateway 选项模式
│   └── gateway_test.go
├── adapter/
│   ├── registry.go           # AdapterRegistry
│   ├── adapter.go            # IAdapter 接口
│   ├── deepseek.go           # DeepSeek 适配器
│   ├── deepseek_test.go
│   ├── minimax.go            # MiniMax 适配器
│   └── minimax_test.go
├── breaker/
│   ├── circuit_breaker.go    # CircuitBreaker 实现
│   ├── circuit_breaker_test.go
│   └── state.go              # 状态机
├── token/
│   ├── counter.go           # Token 计数器
│   ├── counter_test.go
│   └── estimator.go         # Token 估算器
├── config/
│   ├── loader.go            # 配置加载器
│   ├── loader_test.go
│   └── provider.go          # Provider 配置
├── retry/
│   ├── retry.go             # 重试策略
│   └── retry_test.go
└── observer/
    ├── observer.go          # ILLMObserver 接口
    └── noop.go              # NoOp 实现
```

---

## 七、错误处理

| 错误码 | 类型 | 说明 | 可恢复 |
|--------|------|------|--------|
| LLM_PROVIDER_1001 | ProviderUnavailable | Provider 不可用 | ✅ |
| LLM_CIRCUIT_1002 | CircuitOpen | 熔断器开启 | ✅ |
| LLM_TIMEOUT_1003 | Timeout | 请求超时 | ✅ |
| LLM_AUTH_1004 | AuthFailed | 认证失败 | ❌ |
| LLM_TOKEN_1005 | TokenBudgetExceeded | Token 超限 | ❌ |
| LLM_PARSE_1006 | ParseError | 响应解析错误 | ✅ |
| LLM_UNSUPPORTED_1007 | UnsupportedProvider | 不支持的 Provider | ❌ |

### 7.1 错误可重试性

```go
func IsRetryable(err error) bool {
    switch e := err.(type) {
    case *LLMError:
        switch e.Code {
        case "LLM_TIMEOUT_1003", "LLM_PROVIDER_1001", "LLM_PARSE_1006":
            return true
        default:
            return false
        }
    }
    return true // 网络错误默认可重试
}
```

---

## 八、测试策略

| T 层 ID | 描述 | 优先级 |
|-------|------|--------|
| D3-LLM-T01 | DeepSeek 适配器流式响应 | P0 |
| D3-LLM-T02 | MiniMax 适配器流式响应 | P0 |
| D3-LLM-T03 | Circuit breaker 正常关闭 | P0 |
| D3-LLM-T04 | Circuit breaker 触发开启 | P0 |
| D3-LLM-T05 | Circuit breaker 半开→关闭 | P0 |
| D3-LLM-T06 | Circuit breaker 半开→开启 | P0 |
| D3-LLM-T07 | Token 计数准确性 | P0 |
| D3-LLM-T08 | Token 预算检查 | P0 |
| D3-LLM-T09 | Provider 配置加载 | P0 |
| D3-LLM-T10 | 未知 Provider 报错 | P1 |
| D3-LLM-T11 | 重试策略执行 | P1 |
| D3-LLM-T12 | Fallback 模型切换 | P1 |
| D3-LLM-T13 | LLM 调用可观测事件 | P1 |

---

## 九、版本分期

| 版本 | 能力 |
|------|------|
| V1 | DeepSeek + MiniMax 适配器 + Circuit Breaker + Token Counter |
| V2 | Anthropic/OpenAI 适配器 + Rate Limiter |
| V3 | 多模型负载均衡 + A/B Testing |

---

## 十、L2-L3 接口协议（关键）

### 10.1 接口契约

Context Engine 定义的接口：

```go
// internal/layers/contextengine/contracts.go

type ILLMGateway interface {
    ChatStream(ctx context.Context, req *LLMRequest) (<-chan LLMChunk, error)
}

type LLMRequest struct {
    Model        string
    SystemPrompt string
    Messages     []types.Message
    Tools        []ToolSchema
}

type LLMChunk struct {
    Content   string
    Thinking  string
    ToolCalls []ToolCall
    Done      bool
    Usage     TokenUsage
}

type ToolCall struct {
    ID       string
    Name     string
    Input    string
    RiskLevel types.RiskLevel  // ⚠️ 必需
}
```

### 10.2 LLM Gateway 实现要点

```go
// LLM Gateway 实现时，必须适配 Context Engine 的接口

func (g *LLMGateway) ChatStream(ctx context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
    out := make(chan contextengine.LLMChunk, 32)
    
    go func() {
        defer close(out)
        
        // 1. 获取适配器（根据配置的 provider）
        adapter := g.registry.Get(req.Provider)
        
        // 2. 构建 Provider 特定的请求
        providerReq := g.buildProviderRequest(req)
        
        // 3. 流式调用
        for chunk := range adapter.Stream(ctx, providerReq) {
            // 4. 转换 ToolCall，填充 RiskLevel
            for i := range chunk.ToolCalls {
                chunk.ToolCalls[i].RiskLevel = g.toolsReg.RiskLevel(chunk.ToolCalls[i].Name)
            }
            
            // 5. 转换为 Context Engine 类型
            out <- contextengine.LLMChunk{
                Content:   chunk.Content,
                Thinking:  chunk.Thinking,
                ToolCalls: chunk.ToolCalls,
                Done:      chunk.Done,
                Usage: contextengine.TokenUsage{
                    PromptTokens:     chunk.Usage.PromptTokens,
                    CompletionTokens: chunk.Usage.CompletionTokens,
                },
            }
        }
    }()
    
    return out, nil
}
```

### 10.3 RiskLevel 填充逻辑

```go
// ToolCall 的 RiskLevel 不来自 LLM API，而是从 IToolRegistry 获取
// 这在 Context Engine 层通过 IToolRegistry 接口实现

// LLM Gateway 需要注入 IToolRegistry
type LLMGateway struct {
    registry IToolRegistry  // 从 Context Engine 传入
    // ...
}
```

### 10.4 Provider 信息传递

由于 Context Engine 的 `LLMRequest` 没有 `Provider` 字段，Provider 信息需要通过其他方式传递：

**方案 A**: 通过 `EngineDeps` 配置默认 Provider
```go
type EngineDeps struct {
    LLM        ILLMGateway
    DefaultProvider string  // 新增
    // ...
}
```

**方案 B**: 在 `LLMRequest` 中扩展（需同步修改 Context Engine）

**建议采用方案 A**，保持 Context Engine 不变。

### 10.5 类型转换表

| Context Engine 类型 | LLM Gateway 内部类型 | 说明 |
|-------------------|---------------------|------|
| `contextengine.LLMRequest` | `llmgateway.LLMRequest` | LLM Gateway 添加 Provider/MaxTokens 等 |
| `contextengine.LLMChunk` | `llmgateway.LLMChunk` | 直接复用 |
| `contextengine.TokenUsage` | `llmgateway.TokenUsage` | LLM Gateway 多 `TotalTokens` |
| `contextengine.ToolCall` | `llmgateway.ToolCall` | LLM Gateway 多 `RiskLevel` 填充 |

### 10.6 配置传递链

```
devrix.yaml
    ↓
LLMGateway 配置 (Provider/BaseURL/APIKey)
    ↓
Adapter 构建 HTTP 请求
    ↓
Context Engine 传入 LLMRequest (Model/Messages/Tools)
    ↓
LLM Gateway 组合: Provider 配置 + LLMRequest
    ↓
调用 Adapter.Stream()
```
