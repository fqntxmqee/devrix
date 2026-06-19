# LLM Gateway Domain Design (D3)

> **Source of Truth:** `openspec/specs/d3-llm-gateway/spec.md` (v3.1.0)
> **设计历史:** `openspec/archive/2026-06-07-devrix-llm-gateway/design.md`, `openspec/archive/2026-06-08-devrix-llm-gateway-v2/`, `openspec/changes/devrix-d3-sa-refine/design.md` (v3.0.0)
> **Change:** devrix-d3-sa-refine（R1+R2+R3 全部决议）+ devrix-d3-sa-refine-v1.1（D1-D7 R1 决议）+ devrix-d3-sa-refine-v2.0（DM-019 物理路径迁移）

**Domain:** D3 - LLM Gateway
**DSAFT Type:** 公共域 (Common Domain)
**Version:** 3.3.0
**Status:** Active (2026-06-19)
**Last Updated:** 2026-06-19

---

## 0. 变更摘要（V3.1.0 → V3.2.0，v2.0 子 change）

| 维度 | V3.1.0 | **V3.2.0** |
|------|--------|--------|
| 物理目录 | 7 个技术角色词目录（`adapter/` `gateway/` `breaker/` `retry/` `token/` `safety/` `config/`） | **6 个价值流 slug 目录**（`route/` `stream/` `stream/adapter/` `protect/` `budget/` `guard/` `configure/`）+ 7 个 re-export 桥接 |
| `contracts.go` | 单文件 ~450 行（kernel 性质保留） | **按价值流拆分到子包**，根 < 200 行（仅 ILLMGateway/ITierResolver/EngineEvent/SentinelError + re-export） |
| 跨包配置 | `internal/shared/config/llmgateway.go` 与 `internal/layers/llmgateway/configure/loader.go` 分属两包 | **合并到 `configure/`**（llmgateway_features_test.go 跨包迁移） |
| Bridge 跨域锚点 | `internal/bridges/llm/` | **不变**（R1 D2 决议） |
| F 编排 / T 映射 | v1.0 + v1.1 30 F 域内 + 3 F CROSS + 26 T + 9 v1.1 T = 35 T | **不变**（F 编排 + T 映射仅 import 路径同步） |
| runtime span / metric / YAML config key 字面量 | 5 span + 5 metric + 3 YAML key | **不变**（v1.0 不变性承诺） |
| §10.2 物理路径计划 | "v2.0 启动时执行"占位 | **详细 F2–F9 步骤 + re-export 桥接契约 + contracts.go 拆分步骤 C1–C5** |

**v2.0 核心承诺（继承 + 新增）**：

| 承诺 | 状态 |
|------|------|
| 物理路径与 5+1 S 1:1 对齐 | ✅ F2-F8 |
| Bridge 跨域锚点稳定 | ✅ F10 不变 |
| 旧路径 1 发布周期兼容 | ✅ re-export 桥接 |
| v1.0 + v1.1 行为 bit-identical | ✅ 仅动路径，不动行为 |
| 11 P0 T + 26 T 回归 100% 绿 | ✅ G5 强制 |
| runtime span/metric/config key 字面量未改 | ✅ G8 强制 |

---

## 0a. 变更摘要（V3.0.0 → V3.1.0，v1.1 子 change）

---

## 一、架构目标

### 1.1 业务目标

| 痛点 | 目标能力 | 价值流 S | 用户可感知结果 |
|------|----------|----------|----------------|
| LLM 模型路由不透明 | model 名 → provider + 实际 model | **D3-S1 RouteModel** | 给 model 名就能调 |
| 流式调用协议分裂 | 统一 OpenAI-compatible SSE 协议 | **D3-S2 StreamChat** | 流式 chunk 实时回传 |
| Provider 故障阻塞用户 | Breaker + Retry + Fallback | **D3-S3 ProtectCall** | 模型故障时降级不阻塞 |
| Token 预算失控 | 预算检查 + 截断 | **D3-S4 BudgetTokens** | 上下文压缩触发准确 |
| 恶意内容注入 | Safety Filter | **D3-S5 GuardContent** | 风险内容被拦截 |
| 配置缺失静默启动 | 启动 fail-fast | **D3-S6 ConfigureGateway** | 启动错误立即可见 |

### 1.2 技术指标

| 指标 | 目标 | 测量方式 | 关联 S |
|------|------|----------|--------|
| LLM 调用延迟 | P99 < 5s | span `llm.stream` duration | D3-S2 |
| 熔断器切换延迟 | < 10ms | span `llm.circuit_breaker` | D3-S3 |
| Routing 解析延迟 | < 1ms | span `llm.provider.route` | D3-S1（v3.0.0 新增） |
| Token 计数准确性 | cl100k_base ± 5% | 单元测试 | D3-S4 |
| Provider 故障恢复 | 半开→关闭 30s | 集成测试 | D3-S3 |
| Safety 过滤延迟 | P99 < 1ms | span event `safety.check.duration_ms` | D3-S5（v1.1 监测） |
| 启动 fail-fast 延迟 | < 100ms | 启动 trace | D3-S6 |

### 1.3 层间边界

```
Layer 2 (Context Engine)              Layer 3 (LLM Gateway)              Cross-Domain
─────────────────────────            ──────────────────────────         ────────────
QueryLoop / adapters
    │                                    ┌─ D3-S1 RouteModel
    │                                    │  └─ router (Resolve, F02a/F02b)
    │                                    ├─ D3-S2 StreamChat
    │                                    │  └─ adapter (F01/F02/F03)
    │                                    ├─ D3-S3 ProtectCall
    │                                    │  └─ breaker + retry (F01~F06)
    │                                    ├─ D3-S4 BudgetTokens
    │                                    │  └─ counter (F01~F05)
    │                                    ├─ D3-S5 GuardContent
    │                                    │  └─ filter (F01~F03)
    │                                    └─ D3-S6 ConfigureGateway
    │                                       └─ loader (F01~F04)
    │
    └──▶ ILLMGateway.ChatStream() ──▶ Bridge (bridges/llm) [D3-X]
                                         │
                                         └──▶ IGateway.Stream() ──▶ 编排所有 S
```

**禁止**：LLM Gateway 不得 import contextengine/ 或 communication/ 包。**Bridge 是 D3 → D2 的契约实现，归属跨域锚点 `internal/bridges/llm/`**（R1 D2 决议）。

---

## 二、领域模型

### 2.1 核心类型（contracts.go 拆分粒度 — R2 §4.3 决策占位）

> **R2 §4.3 决策**：v1.0 收尾时 contracts.go 整体保留（**0 行为变更**）；v2.0 Phase F 启动时**实际拆分**到各子包。
> **v2.0 拆分原则**（R2 §4.3 + R3 NQ-6 决策占位）：
> - **Kernel 性质**（跨 S 共享 / 跨域锚点依赖）：`Request` / `Chunk` / `TokenUsage` / `ToolCall` 留根包（`internal/layers/llmgateway/` 根）
> - **S 内部私有**：`CircuitState` / `CircuitBreakerConfig` 移入 `protect/` 子包；`RetryConfig` 移入 `protect/` 子包
> - **跨域共享**：`Request` / `Chunk` 与 D2 types.Message / types.ToolCall 共享，故留根包（不引入 `kernel/` 子包，R3 NQ-6 决议 v2.0 不引入 `kernel/`）

```go
// === 根包 contracts.go（v1.0 不动；v2.0 拆分计划见上） ===

// Request is the D3 internal chat completion input.
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
    CacheReadTokens  int
    ReasoningTokens  int
}

// ToolCall is an LLM-requested tool invocation (no RiskLevel; D2 fills it).
type ToolCall struct {
    ID    string
    Name  string
    Input string
}

type AdapterChunk struct {
    Raw    []byte
    Parsed *Chunk
    Error  error
}

// === v2.0 Phase F 拆分到 protect/ 子包（占位） ===

// type CircuitState string
// const (
//     CircuitClosed   CircuitState = "closed"
//     CircuitOpen     CircuitState = "open"
//     CircuitHalfOpen CircuitState = "half-open"
// )
//
// type CircuitBreakerConfig struct {
//     FailureThreshold  int
//     SuccessThreshold  int
//     OpenDuration      time.Duration
//     HalfOpenMaxProbes int
//     Scope             Scope  // v1.1 扩展枚举：provider / provider_model / model
// }
//
// type RetryConfig struct {
//     MaxAttempts  int
//     InitialDelay time.Duration
//     MaxDelay     time.Duration
//     Backoff      float64
// }
```

> **v2.0 实施时**：`Request` / `Chunk` / `TokenUsage` / `ToolCall` / `AdapterChunk` 留根（`llmgateway/`）；`CircuitState` / `CircuitBreakerConfig` / `RetryConfig` 移 `protect/`。**v1.0 阶段完全不拆**，所有 import 路径不变。

### 2.2 V3.0.0 新增类型

```go
// === R3 P0 #8 — Fail-Fast 启动 ===

// ErrObservabilityRequired is returned by NewFromConfig when obs == nil.
// RATIONALE: silent fallback 导致 v1.1 翻 d3_resilience_emit_enabled 时用户感知不到。
var ErrObservabilityRequired = errors.New("llmgateway: observability bridge is required (fail-fast, no silent fallback)")

// === R3 P1 #11 — Breaker Scope 扩展（v1.1 实施） ===

type Scope string

const (
    ScopeProvider      Scope = "provider"       // V2.1 默认
    ScopeProviderModel Scope = "provider_model" // v1.1 候选
    ScopeModel         Scope = "model"          // 未来
)
```

---

## 三、A + F 编排时序

> 完整 spec 见 `spec.md` §3-9；本节展示 A 编排的运行时序。

### 3.1 D3-S1 RouteModel 编排

```
A: D3-S1-A01 ResolveModelRoute
  Input: model_name (含 tier alias)
  Output: (provider, resolved_model, error)

  Sequence:
    [span: llm.provider.route]
      1. F02a ResolveTierAlias(model)
         ├─ hit (tier ∈ ModelTiers) → resolved_model
         └─ miss → ErrUnknownTier
      2. F02b ResolveDefault(model)  [if F02a returns unknown/empty]
         ├─ empty model → DefaultProvider.DefaultModel
         └─ unknown model → ErrUnsupportedModel
      3. F01 MatchRouting(resolved_model)
         └─ pattern match (deepseek-* → deepseek) → provider
      [span end]
```

**错误码签名**：

| 错误 | 抛出 F | 含义 |
|------|--------|------|
| `ErrUnknownTier` | F02a | tier alias 不在 `ModelTiers` 配置表 |
| `ErrNoRoute` | F02b | 空 model + 无 `DefaultProvider` |
| `ErrUnsupportedModel` | F02b | model 名非空但非任何 provider 支持 |
| `LLM_UNSUPPORTED_1007` | F01 | model → provider 路由失败 |

### 3.2 D3-S2 StreamChat 编排

```
A: D3-S2-A01 StreamChatCompletion
  Input: ctx, llmgateway.Request
  Output: <-chan *AdapterChunk

  Sequence:
    [span: llm.stream]  ← CLIENT
      1. F03 BuildOpenAIRequest(req)
         └─ llmgateway.Request → OpenAI JSON body
      2. F01 OpenAIStreamClientStream(ctx, body, adapter_cfg)
         └─ HTTP POST /chat/completions (SSE) → raw stream
      3. F02 ParseSSE(raw_stream)
         └─ SSE events → AdapterChunk
    [span end]
```

**Provider 适配**（R1+Q5 + R3 命题 C 衍生）：

- DeepSeekAdapter / MiniMaxAdapter 复用 F01 + F03
- Provider 路由在 D3-S1 完成（`Router.Resolve` 返回 provider）
- V3 扩展点：`IAdapter` 接口增 `Protocol() string` 方法（v1.0 release 后 P1 实施）

### 3.3 D3-S3 ProtectCall 编排

```
A: D3-S3-A01 ShieldAndRetry
  Input: ctx, call, primary, fallback, retry_config
  Output: <-chan *AdapterChunk

  Sequence:
    [span: llm.circuit_breaker]
      1. F01 AllowCircuit(provider)
         ├─ blocked → CircuitOpenError → return
         └─ allowed ↓
    [span: llm.retry]
      2. F05 StreamWithFallback(ctx, call, primary, fallback, cfg)
         for attempt := 1; attempt <= MaxAttempts; attempt++ {
             [span: llm.adapter.stream]
               call(primary) → chunk
             on success:
               F02 RecordOutcome(provider, true)
               F03 ManageCircuitState(provider, success) → 可能 Closed
               break
             on failure:
               F06 ShouldRecordBreakerFailure(err)?
                 ├─ true (非 Cancel/Deadline) → F02 RecordOutcome(false) → F03 ManageCircuitState
                 └─ false → 不记录
               F04 ComputeBackoff(cfg, attempt) → delay
               sleep(delay) → retry
         }
         on exhausted:
           switch_to(fallback) → 重复 retry 链
           全失败 → RetryExhaustedError
    [span end]
```

**Mechanism 可追溯性**（R2 命题 A）：T 编号按 F 编排顺序，每个 T 末尾加 `<!-- Mechanism: Breaker / Retry / Cross -->` 注释。

### 3.4 D3-S4 / D3-S5 / D3-S6 编排（注入式）

> D3-S4 / D3-S5 / D3-S6 **不直接 emit span**；通过 `llm.stream` 顶层 span 注入 attribute / event。

**D3-S4 BudgetTokens 注入点**：

```
[span: llm.stream]
  start → 注入 budget.checked = true / false
       → 注入 budget.remaining = N
  end → 注入 budget.exceeded (v1.1 候选)
```

**D3-S5 GuardContent 注入点**（R3 P1 #16）：

```
[span: llm.stream]
  safety.check call → 内部计时
  on done:
    span event: safety.check.duration_ms = N
  v1.0: 注入 safety.checked = true / false
```

**D3-S6 ConfigureGateway**（启动期）：

```
启动 trace (独立)
  config.load.duration_ms = N
  on missing obs:
    返回 ErrObservabilityRequired → 启动失败
```

### 3.5 v1.1 F1-F9 时序（新增段）

> v1.1 子 change 引入 9 个新 F（F1-F9 横向编号）；本节列出各 F 的运行时序与跨域 emit 路径。详细 spec 见 `spec.md` §13。

#### 3.5.1 F1/F2/F3 D3-S3 ProtectCall 状态切换 emit

```
A: D3-S3-A01 ShieldAndRetry (v1.1 增 F07/F08/F09)
  Sequence:
    1. F01 AllowCircuit(provider)
       ├─ blocked → F08 OnStateTransitionEmit (provider, from=closed, to=open)
       │           ├─ F07 EmitBreakerStateMetric(provider, "open")
       │           └─ F09 ReuseEngineEvent(provider, "flow.breaker.opened")
       └─ allowed ↓
    2. F05 StreamWithFallback(...)
       ├─ 成功 → F02 RecordOutcome(true) → F03 ManageCircuitState
       │       → F08 状态变化钩子（若 Closed 回归）→ F07 emit state="closed"
       │                                           → F09 emit "flow.breaker.closed"
       └─ 失败（且非 Cancel/Deadline）
           ├─ F06 ShouldRecordBreakerFailure(err) == true
           │   → F02 RecordOutcome(false) → F03 ManageCircuitState
           │   → F08 钩子（若 Open 触发）→ F07 emit state="open" + F09 emit "flow.breaker.opened"
           └─ F04 ComputeBackoff(...) → 退避 → 重试
```

**emit 路径**：
- F07 → D5 Prometheus exporter（`llm_breaker_state{provider, state}`）
- F09 → D7 EngineEvent 订阅（`flow.breaker.*`，3 事件分开）
- 受 `d3_resilience_emit_enabled` flag 控制（默认 ON）

#### 3.5.2 F4 D3-X-A02 WireContextLLM obs nil fail-fast

```
A: D3-X-A02 WireLLMStack (v1.1 增 F02 FailFastOnObsNil)
  Sequence:
    1. 入参检查
       ├─ obs == nil → 返回 ErrObservabilityRequired (F02)
       └─ obs != nil ↓
    2. config 加载 + validate
    3. NewFromConfig (factory.go) — 内部不重复检查（已 F02 拦截）
    4. 返回 ContextLLMStack + nil
```

**Fail-Fast 触发**：
- 测试 fixture 显式注入 mock obs（`WithMockObs()` helper）避免误传 nil
- 错误码 `ErrObservabilityRequired` 在 `internal/shared/errors/` 已注册

#### 3.5.3 F5 D3-S2-A01 IAdapter.Protocol() string

```
A: D3-S2-A01 StreamChatCompletion (v1.1 增 F04 AdapterProtocolMethod)
  编译期强制：所有 IAdapter 实现必须补 Protocol() string 方法
    - DeepSeekAdapter.Protocol() → "openai-compat"
    - MiniMaxAdapter.Protocol()  → "openai-compat"
    - stubAdapter.Protocol()     → "openai-compat" (test fixture)
  运行时：通过 interface dispatch；不引入额外 hot path
  错误码：ErrAdapterProtocolNotImplemented（编译期阻断；v1.1 release 前必须修）
```

**BREAKING 改造**：
- v1.1 release 时 Provider 列表已知（仅 DeepSeek + MiniMax + stubAdapter test fixture = 3 处），可控
- 旧 IAdapter 实现的 import 全部编译失败，强制同步修
- v2.0 启用：protocol-aware fallback（plan A → 协议 A → 协议 B fallback）

#### 3.5.4 F6 D3-S1-A01 Tier Resolution Probe

```
A: D3-S1-A01 ResolveModelRoute (v1.1 增 F06 ProbeTierResolution 配合 D6 probe #1)
  运行时：
    1. F02a/F02b/F01 编排（v1.0 不变）
    2. F06 emit llm_tier_resolve_total{outcome=hit|fallback|error}
  D6 probe #1 统计：
    coverage = hits / total
    阈值 ≥ 99%
```

#### 3.5.5 F7 D3-S3-A01 Breaker Transitions Probe

```
A: D3-S3-A01 ShieldAndRetry (v1.1 增 F07 ProbeBreakerTransitions 配合 D6 probe #2)
  运行时：
    1. F03 ManageCircuitState 状态切换
    2. F07 emit llm_breaker_transitions_total{provider, from, to}
  D6 probe #2 统计：
    anomaly = 短时间内（5min）切换次数 > 阈值
    触发 ErrBreakerAnomalyTransition 告警
```

#### 3.5.6 F8 D3-S5-A01 Safety Latency Event

```
A: D3-S5-A01 FilterAndMatchContent (v1.1 增 F04 EmitSafetyLatencyEvent)
  运行时：
    1. F01 CheckContent 调用
    2. start = time.Now()
    3. 执行 F01 内容检查
    4. duration_ms = time.Since(start).Milliseconds()
    5. 当前 trace span 写入 event: safety.check.duration_ms = N
  D6 probe #4 统计：
    P99 = percentile(durations, 99)
    阈值 P99 < 1ms；持续 5min > 1ms 触发 ErrSafetyLatencyThreshold
```

**Feature flag 控制**：受 `d3_safety_latency_event_enabled` 控制（默认 ON）

#### 3.5.7 F9 D3-S6-A01 Feature Flag Defaults

```
A: D3-S6-A01 LoadAndValidateLLMConfig (v1.1 增 F05 FeatureFlagDefaults)
  运行时：
    1. config.Load
    2. config.Build (应用默认值)
       - d3_resilience_emit_enabled = true (D4-B 决议)
       - d3_safety_latency_event_enabled = true (D4-B 决议)
       - d3_metric_emit_warn = false (D4-B 决议)
    3. config.Validate (校验 + 类型转换)
  OFF 行为继承：
    F05 单元测试覆盖：flag 显式设为 false 时 v1.0 行为完全保持
```

---

## 四、Provider 配置

### 4.1 支持的 Provider

| Provider | Adapter | API 类型 | Base URL |
|----------|---------|----------|----------|
| `deepseek` | DeepSeekAdapter | OpenAI-compatible | `https://api.deepseek.com/v1` |
| `minimax` | MiniMaxAdapter | OpenAI-compatible | `https://api.minimaxi.com/v1` |

### 4.2 配置契约（devrix.yaml）

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
    scope: "provider"  # v1.0 默认；v1.1 候选 provider_model / model

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

  # V3.0.0 新增（v1.0 P0 收尾）
  observability_required: true  # fail-fast 默认开启
  d3_resilience_emit_enabled: false  # v1.1 翻 true
```

---

## 五、接口设计

### 5.1 核心接口

```go
// IGateway streams chat completions (D3 internal API).
type IGateway interface {
    Stream(ctx context.Context, req *Request) (<-chan Chunk, error)
    ResolveTier(tier string) string
    Close() error
}

// ILLMGateway is the primary LLM consumer contract.
// DSAFT: D3-X-A01 AdaptToOrchestrator (CROSS 段)
//
// DM-020 (D7 Turn 编排上移): Primary consumer migrated from D2 to D7.
// D7-S2-A07 InvokeLLM is the canonical consumer; D2→D3 is banned (import lint).
// Legacy alias AdaptToContextEngine kept for backward compat, 1 release cycle.
type ILLMGateway interface {
    ChatStream(ctx context.Context, req *Request) (<-chan Chunk, error)
}

// ITierResolver resolves tier aliases to concrete model names.
// DSAFT: D3-S1-A01-F02a ResolveTierAlias
type ITierResolver interface {
    ResolveTier(tier string) (string, error)
}

// IAdapter streams provider-specific responses.
// DSAFT: D3-S2-A01-F01 OpenAIStreamClientStream
// V3.0.0 增量（v1.0 release 后 P1 实施）：
type IAdapter interface {
    Stream(ctx context.Context, req *Request) (<-chan *AdapterChunk, error)
    Provider() string
    Protocol() string  // v1.0 release 后 P1 增；当前 V2.1 适配器返回 "openai-compatible"
}

// ICircuitBreaker protects providers from cascading failures.
// DSAFT: D3-S3-A01 F01/F02/F03
type ICircuitBreaker interface {
    Allow(circuitKey string) (bool, error)
    RecordSuccess(circuitKey string)
    RecordFailure(circuitKey string)
    State(circuitKey string) CircuitState
}

// ISafetyFilter filters content with severity levels.
// DSAFT: D3-S5-A01 F01/F02/F03
type ISafetyFilter interface {
    Check(ctx context.Context, systemPrompt string, messages []string) *safety.Result
    AddPattern(p safety.Pattern)
}
```

### 5.2 配置类型（`configure/shared_config.go` · v2.0 物理路径）

> **v2.0 路径迁移**（DM-20260614-019, 2026-06-14 落地）：原 `internal/shared/config/llmgateway.go` 已合并到 `internal/layers/llmgateway/configure/shared_config.go`，与 `configure/loader.go` 同包。旧路径保留 1 发布周期 re-export 桥接。

```go
type LLMGatewayConfig struct {
    DefaultProvider          string
    DefaultModel             string
    DefaultTier              string
    ModelTiers               map[string]string
    ModelRouting             map[string]string
    CircuitBreaker           LLMCircuitBreakerConfig
    Providers                map[string]LLMProviderRuntimeConfig
    ObservabilityRequired    bool  // V3.0.0 新增；默认 true
    D3ResilienceEmitEnabled  bool  // V3.0.0 新增；默认 false（v1.1 翻）
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

## 六、熔断器状态机

```
[Initial] → Closed (正常)
    ↓ failure >= FailureThreshold (default: 5)
    Open (熔断开启，OpenDuration 后)
    ↓
    HalfOpen (探测，最多 HalfOpenMaxProbes 并发)
    ↓ success >= SuccessThreshold (default: 2) → Closed
    ↓ failure → Open
```

**Key 决策**：

- `context.Canceled` 和 `context.DeadlineExceeded` 不触发 `RecordFailure`（F06 ShouldRecordBreakerFailure）
- Breaker state 持久化：PLANNED（D3-S3-A01-T08，v1.1 实施；R3 NQ-1 决议）
- Breaker scope：`ScopeProvider` 默认（V2.1 行为不变）；v1.1 评估升级到 `ScopeProviderModel`（R3 P1 #11）

---

## 七、错误处理

| 错误码 | 类型 | 说明 | 可重试 | 抛出 S |
|--------|------|------|--------|--------|
| LLM_PROVIDER_1001 | ProviderUnavailable | Provider 不可用（HTTP >=300 或非 401/403） | ✅ | D3-S2 / D3-S3 |
| LLM_CIRCUIT_1002 | CircuitOpen | 熔断器开启 | ✅ (自动恢复) | D3-S3 |
| LLM_TIMEOUT_1003 | Timeout | 请求超时 | ✅ | D3-S2 / D3-S3 |
| LLM_AUTH_1004 | AuthFailed | 认证失败 (401/403) | ❌ | D3-S2 |
| LLM_TOKEN_1005 | TokenBudgetExceeded | Token 超限 | ❌ | D3-S4 |
| LLM_PARSE_1006 | ParseError | SSE 响应解析错误 | ✅ | D3-S2 |
| LLM_UNSUPPORTED_1007 | UnsupportedProvider | 不支持的 Provider | ❌ | D3-S1 (F01) |
| LLM_UNSUPPORTED_1008 | UnsupportedModel | 不支持的 Model | ❌ | D3-S1 (F02b) |
| LLM_TIER_1009 | UnknownTier | tier alias 不在配置表 | ❌ | D3-S1 (F02a) |
| LLM_OBS_1010 | ObservabilityRequired | obs bridge 缺失（fail-fast） | ❌ | D3-S6 / D3-X |
| LLM_SAFETY_1011 | ContentRejected | 内容 critical 命中 | ❌ | D3-S5 |

### 可重试性判断

```go
func IsRetryable(err error) bool {
    switch {
    case errors.Is(err, ErrProviderUnavailable): return true
    case errors.Is(err, ErrTimeout): return true
    case errors.Is(err, ErrParseError): return true
    case errors.Is(err, ErrCircuitOpen): return true  // 自动恢复
    case errors.Is(err, ErrAuthFailed): return false
    case errors.Is(err, ErrTokenBudgetExceeded): return false
    default: return false
    }
}
```

---

## 八、Safety Filter 设计

### 8.1 默认模式

| 模式 | 严重级别 | 动作 | 位置 |
|------|---------|------|------|
| malware_generation | critical | reject | all |
| exploit_generation | critical | reject | all |
| unauthorized_access | high | reject | all |
| hardcoded_credential | medium | warn | message |
| prompt_injection | medium | warn | message |
| data_exfiltration | medium | warn | message |

### 8.2 v1.1 增量（R3 P1 #16）

- `Filter.Check` 内部计时，写入 span event `safety.check.duration_ms`
- D6 probe #4 **Safety filter latency P99**（P99 > 1ms 持续 5min 告警）
- 长期：V1.2 路线图引入 `trie` 数据结构替代 substring matching（R3 P2 #23）

### 8.4 v1.1 F04 EmitSafetyLatencyEvent 实施

| 维度 | v1.0 占位 | v1.1 实施 |
|------|----------|----------|
| 计时位置 | — | `Filter.Check` 入口 `start := time.Now()`；出口 `duration_ms := time.Since(start).Milliseconds()` |
| Span event 写入 | — | 当前 trace span 写入 `safety.check.duration_ms = N` |
| Feature flag | — | `d3_safety_latency_event_enabled`（默认 ON） |
| D6 probe #4 阈值 | — | P99 < 1ms；持续 5min > 1ms 触发 `ErrSafetyLatencyThreshold` |
| Hot path 影响 | — | 计时 O(1) 无锁；span event 写入受 flag 控制 |

### 8.3 跨域灰区契约（R2 命题 E / P0 #5）

> D3-S5 GuardContent vs D2-S18 PermissionMode 灰区：
> - D3 责任：prompt 内容过滤（内容本身是否含危险模式）
> - D2 责任：tool execution 权限（允许哪些 tool 调用）
> - 灰区处理：内容与 tool 交叉时**D3 优先拒**（前置过滤），D2 兜底
>
> 详见 `openspec/specs/architecture/cross-domain-boundaries.md` §D3-S5

---

## 九、可观测性

完整 Span 注册表、Trace Tree、Metrics 定义见 `span-registry.md`（v3.0.0）。

**V3.0.0 关键变化**：
- 运行时 span 名 5 个保持不变（`llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream`）
- v1.1 新增 metric：`llm_breaker_state`（R1 Q6 + R2 命题 B + R3 命题 A P1 #11）
- v1.1 新增 span event：`safety.check.duration_ms`（R3 P1 #16）
- v1.1 新增 metric：`d3_metric_emit_total{status=ok|missing}`（R3 P1 #14）

**Feature flag 落地**（v1.1 D4-B 决议固化）：

| Flag | 默认值（v1.0） | 默认值（v1.1，D4-B） | 启用效果 | 关联 F |
|------|---------------|----------------------|----------|--------|
| `d3_resilience_emit_enabled` | `false` | **`true` (ON)** | 启用 `llm_breaker_state` metric + `flow.breaker.*` EngineEvent 通知 D7 | F1 + F2 + F3 |
| `d3_safety_latency_event_enabled` | `false` | **`true` (ON)** | 在 `llm.stream` 上 emit span event `safety.check.duration_ms` | F8 |
| `d3_metric_emit_warn` | `true` | **`false` (OFF)** | emit 失败时是否 log warn | F9 |

**D4-B 决议理由**：
- emit flag 默认 ON：cardinality 受控（2 provider × 3 state = 6 series），dashboard 默认可用
- warn flag 默认 OFF：emit 失败不污染日志；走 D5 健康检查（emit 失败率作为 D5 内部 metric）

**OFF 行为继承**（F5 FeatureFlagDefaults 实施说明）：3 flag 默认值变更时（`false → true` 或反之），单元测试需验证 v1.0 行为完全保持；OFF 时旧行为可恢复。

---

## 十、目录结构

### 10.1 当前物理路径（v1.0 保留 — 0 物理路径变更）

```
internal/layers/llmgateway/
├── contracts.go                 # 接口与核心类型定义（v1.0 不拆；v2.0 拆 protect/）
├── gateway/
│   ├── gateway.go              # Gateway 主实现 (Stream, startSpan, metrics)
│   ├── router.go               # Router (Resolve, F02a ResolveTierAlias + F02b ResolveDefault)
│   ├── router_test.go
│   ├── factory.go              # NewFromConfig — ★V3.0.0 fail-fast (R3 P0 #8)
│   └── gateway_test.go
├── adapter/
│   ├── registry.go             # AdapterRegistry
│   ├── deepseek.go / deepseek_test.go
│   ├── minimax.go / minimax_test.go
│   ├── openai_stream.go        # F01 OpenAIStreamClientStream
│   ├── openai_request.go       # F03 BuildOpenAIRequest
│   ├── openai_types.go
│   ├── sse_parser.go           # F02 ParseSSE
│   └── sse_parser_test.go
├── breaker/
│   ├── circuit_breaker.go      # F01/F02/F03/F06
│   ├── circuit_breaker_test.go
│   └── state.go                # circuitRecord
├── retry/
│   ├── retry.go                # F04/F05
│   ├── retry_test.go
│   └── retry_jitter_test.go
├── token/
│   ├── counter.go              # F01/F02/F03/F04
│   ├── counter_test.go
│   └── bpe_loader.go           # F05 LoadBPE
├── config/
│   ├── loader.go               # F01/F03/F04
│   └── loader_test.go
└── safety/
    ├── filter.go               # F01/F03
    ├── filter_test.go
    └── patterns.go             # F02 LoadPatterns

internal/bridges/llm/
├── bridge.go                   # D3-X-A01 AdaptToContextEngine
├── bridge_test.go
├── context_wiring.go           # D3-X-A02 WireLLMStack
├── wire.go                     # ★V3.0.0 fail-fast (R3 P0 #8): obs nil → ErrObservabilityRequired
└── readiness.go                # Readiness probe
```

### 10.2 v2.0 物理路径（✅ 已完成，DM-20260614-019, 2026-06-14）

> **v2.0 状态**：v2.0 物理路径迁移已 2026-06-14 落地（DM-20260614-019 `devrix-d3-sa-refine-v2.0`）。11 P0 T + 26 T 回归 100% 绿；re-export 桥接保留 1 发布周期（计划 v2.1 物理清理）。当前实际代码已全部位于 v2.0 物理路径，本节为权威目录树。

```
internal/layers/llmgateway/
├── contracts.go                 # < 200 行；仅 ILLMGateway/ITierResolver/EngineEvent/SentinelError + re-export
├── route/                       # F3：旧 gateway/router.go
│   └── router.go
├── stream/                      # F4：旧 gateway/gateway.go Stream 主实现
│   ├── gateway.go
│   └── adapter/                 # F2：旧 adapter/ 全部
│       ├── deepseek.go
│       ├── minimax.go
│       ├── openai_stream.go
│       ├── openai_request.go
│       ├── openai_types.go
│       ├── protocol.go          # v1.1 F5 IAdapter.Protocol()
│       ├── registry.go
│       ├── sse_parser.go
│       └── *_test.go (3 test files)
├── protect/                     # F5：旧 breaker/ + retry/ 合并（独立 .go）
│   ├── circuit_breaker.go
│   ├── state.go
│   ├── observer.go              # v1.1 F07/F08 metric + counter
│   ├── retry.go
│   ├── retry_jitter.go
│   ├── breaker_observer.go      # v1.1 wiring（从 gateway/breaker_observer.go 迁入）
│   └── *_test.go
├── budget/                      # F6：旧 token/
│   ├── counter.go
│   ├── bpe_loader.go
│   └── counter_test.go
├── guard/                       # F7：旧 safety/
│   ├── filter.go
│   ├── patterns.go
│   └── filter_test.go
└── configure/                   # F8：旧 config/ + shared/config/llmgateway.go
    ├── loader.go
    ├── loader_test.go
    ├── shared_config.go         # 旧 shared/config/llmgateway.go
    └── llmgateway_features_test.go  # 旧 shared/config/llmgateway_features_test.go
```

**re-export 桥接（1 发布周期）**：

```
internal/layers/llmgateway/
├── adapter/                     # F2 桥接：re-export → stream/adapter
│   └── (桥接文件，// Deprecated)
├── gateway/                     # F3/F4 桥接：re-export → route/ + stream/
│   ├── router.go                # F3 桥接
│   ├── gateway.go               # F4 桥接
│   └── breaker_observer.go      # v1.1 observer 桥接 → protect/breaker_observer
├── breaker/                     # F5 桥接：re-export → protect/circuit_breaker 等
│   └── (桥接文件)
├── retry/                       # F5 桥接：re-export → protect/retry
│   └── (桥接文件)
├── token/                       # F6 桥接：re-export → budget/
│   └── (桥接文件)
├── safety/                      # F7 桥接：re-export → guard/
│   └── (桥接文件)
└── config/                      # F8 桥接：re-export → configure/
    └── (桥接文件)

internal/shared/config/
├── llmgateway.go                # F8 桥接：re-export → llmgateway/configure/shared_config
└── llmgateway_features_test.go  # F8 桥接：re-export → llmgateway/configure/llmgateway_features_test
```

**根 `contracts.go` re-export 模板**：

```go
// contracts.go（v2.0 拆分后 < 200 行）
package llmgateway

import (
    "github.com/devrix/devrix/internal/shared/errors"
    stream_adapter "github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
    route_pkg "github.com/devrix/devrix/internal/layers/llmgateway/route"
    protect_pkg "github.com/devrix/devrix/internal/layers/llmgateway/protect"
    budget_pkg "github.com/devrix/devrix/internal/layers/llmgateway/budget"
    guard_pkg "github.com/devrix/devrix/internal/layers/llmgateway/guard"
    configure_pkg "github.com/devrix/devrix/internal/layers/llmgateway/configure"
)

// Deprecated: 将在 v2.1 物理清理时删除；请迁移至子包。
type (
    Adapter             = stream_adapter.Adapter
    IAdapter            = stream_adapter.IAdapter
    Request             = stream_adapter.Request
    Chunk               = stream_adapter.Chunk
    Protocol            = stream_adapter.Protocol

    Tier                = route_pkg.Tier
    TierAlias           = route_pkg.TierAlias
    RoutingTable        = route_pkg.RoutingTable

    CircuitState        = protect_pkg.CircuitState
    BreakerConfig       = protect_pkg.BreakerConfig
    BreakerObserver     = protect_pkg.BreakerObserver
    RetryPolicy         = protect_pkg.RetryPolicy

    TokenUsage          = budget_pkg.TokenUsage
    BudgetCheckResult   = budget_pkg.BudgetCheckResult

    SafetyCheckResult   = guard_pkg.SafetyCheckResult
    SafetyLevel         = guard_pkg.SafetyLevel

    LLMConfig           = configure_pkg.LLMConfig
    LLMFeatureFlags     = configure_pkg.LLMFeatureFlags
    ProviderConfig      = configure_pkg.ProviderConfig
)

// 跨域契约（根保留）
type ILLMGateway interface { /* ... */ }
type ITierResolver interface { /* ... */ }
type IEngineEvent = eventbus.EngineEvent

// SentinelError（根保留）
var (
    ErrObservabilityRequired = errors.ErrObservabilityRequired
)
```

> **R2 §4.3 决议回顾**：`CircuitState` / `CircuitBreakerConfig` / `RetryConfig` 拆到 `protect/`；`Request` / `Chunk` / `TokenUsage` / `ToolCall` 通过 type alias 仍可从根访问（kernel 性质，跨域共享）。**R3 NQ-6 决议**：v2.0 不引入 `kernel/` 子包。
> **re-export 桥接**（v2.0 实施细节）：根包保留 `contracts.go` 中对 `protect.CircuitState` 的 type alias，确保旧 import 路径不破坏。

---

## 十三、v2.0 实施步骤（Phase F 详细）

### 13.1 F2 — `stream/adapter/` 迁移步骤

| 步骤 | 操作 | 验证 |
|------|------|------|
| F2.1 | `git mv internal/layers/llmgateway/stream/adapter/ internal/layers/llmgateway/stream/adapter/` | 物理移动完成 |
| F2.2 | 更新 `stream/adapter/*.go` 的内部 import（如 `gateway` → `stream`） | `go build ./internal/layers/llmgateway/stream/adapter/` |
| F2.3 | 创建 `internal/layers/llmgateway/stream/adapter/` 桥接文件（re-export） | `go build ./internal/layers/llmgateway/stream/adapter/` |
| F2.4 | 更新所有外部 import：`grep -r "llmgateway/adapter" --include="*.go"` | 替换为 `stream/adapter` |
| F2.5 | 验证 v1.1 `Protocol()` 在新路径仍工作（`adapter/protocol_test.go` 迁移到 `stream/adapter/protocol_test.go`） | `go test ./internal/layers/llmgateway/stream/adapter/` |
| F2.6 | Bridge 调用方：`internal/bridges/llm/bridge.go` 同步更新 | 11 P0 T + v1.1 9 T 全绿 |

### 13.2 F3 — `route/` 迁移步骤

| 步骤 | 操作 | 验证 |
|------|------|------|
| F3.1 | 创建 `internal/layers/llmgateway/route/` 目录 | 目录 |
| F3.2 | `git mv internal/layers/llmgateway/route/router.go internal/layers/llmgateway/route/router.go` | 物理移动 |
| F3.3 | 创建 `gateway/router.go` 桥接文件（re-export） | 旧路径编译通过 |
| F3.4 | 同步 `router_test.go` | 测试绿 |
| F3.5 | 验证 v1.1 T03 (D3-S1-A01 Tier resolution probe) 仍工作 | D6 probe #1 绿 |

### 13.4 F5 — `protect/` 合并迁移步骤（高风险）

| 步骤 | 操作 | 验证 |
|------|------|------|
| F5.1 | `git mv internal/layers/llmgateway/protect/ internal/layers/llmgateway/protect/_breaker_legacy/`（临时） | 占位 |
| F5.2 | `git mv internal/layers/llmgateway/protect/ internal/layers/llmgateway/protect/_retry_legacy/`（临时） | 占位 |
| F5.3 | 在 `protect/` 下重命名：`_breaker_legacy/*.go → protect/circuit_breaker.go / state.go / observer.go` | 重命名 |
| F5.4 | 在 `protect/` 下重命名：`_retry_legacy/*.go → protect/retry.go / retry_jitter.go` | 重命名 |
| F5.5 | `git mv internal/layers/llmgateway/stream/breaker_observer.go internal/layers/llmgateway/protect/breaker_observer.go` | v1.1 observer 物理归位 |
| F5.6 | 创建 `breaker/` 桥接 + `retry/` 桥接 | 旧路径编译通过 |
| F5.7 | 完整 P0 回归：D3-S3 12 T（Breaker 4 + Retry 2 + 5 Cross + 1 PLANNED）+ v1.1 3 T（T13/T14/T15） | 14/14 绿（除 T08 PLANNED） |

### 13.5 F8 — `configure/` 跨包迁移步骤

| 步骤 | 操作 | 验证 |
|------|------|------|
| F8.1 | 创建 `internal/layers/llmgateway/configure/` 目录 | 目录 |
| F8.2 | `git mv internal/layers/llmgateway/configure/loader.go internal/layers/llmgateway/configure/loader.go` | 物理移动 |
| F8.3 | `git mv internal/layers/llmgateway/configure/loader_test.go internal/layers/llmgateway/configure/loader_test.go` | 测试迁移 |
| F8.4 | `git mv internal/shared/config/llmgateway.go internal/layers/llmgateway/configure/shared_config.go` | 跨包合并 |
| F8.5 | `git mv internal/shared/config/llmgateway_features_test.go internal/layers/llmgateway/configure/llmgateway_features_test.go` | 跨包测试合并 |
| F8.6 | 创建 `internal/layers/llmgateway/configure/` 桥接 + `internal/shared/config/llmgateway*.go` 桥接 | 旧路径编译 |
| F8.7 | 更新所有 `import "internal/shared/config"` 引用方（仅 D3 范围内；其它域如有引用保持） | `goimports` 通过 |
| F8.8 | F9 v1.1 T02 回归（feature flag 8 组合） | T02 绿 |

### 13.6 F9 — `contracts.go` 拆分步骤（必须 F2-F8 完成）

| 步骤 | 操作 | 验证 |
|------|------|------|
| F9.1 | 在各子包创建 `contracts.go`（子包内）：stream/adapter、route、protect、budget、guard、configure | 子包 contracts |
| F9.2 | 根 `contracts.go` 移除已迁类型，添加 `// Deprecated:` 类型别名指向新位置 | 根 < 200 行；`go build` 全绿 |
| F9.3 | 同步更新所有 `import "internal/layers/llmgateway"` 引用方（自底向上：子包 → 子包） | `goimports` 通过 |
| F9.4 | 旧 import 路径兼容（re-export 类型别名） | G2 测试全绿 |
| F9.5 | v1.0 + v1.1 26 T + 9 T 测试 import 同步 | G5 11 P0 T + 26 T 回归 |
| F9.6 | `go build ./...` 全绿 | 整体编译 |

### 13.7 实施顺序

```
F2 stream/adapter ──┐
F3 route             │
F4 stream/gateway    ├── 并行
F6 budget            │
F7 guard             │
F8 configure         │
                     │
F5 protect (高风险) ── 需 F2-F4 完成
                     │
                     ▼
F9 contracts.go 拆分 ── 需 F2-F8 全完成
                     │
                     ▼
F11 layering.md + code-layout.md 同步
```

---

## 十四、v2.0 验收（Phase G 13 项）

| ID | 检查 | 验证方式 |
|----|------|----------|
| G1 | 6 个新价值流 slug 目录存在 | `ls internal/layers/llmgateway/{route,stream,protect,budget,guard,configure}/` |
| G2 | 7 个旧技术角色词目录仅含桥接 | `ls internal/layers/llmgateway/{adapter,gateway,breaker,retry,token,safety,config}/` |
| G3 | `internal/bridges/llm/` 路径未变 | `ls internal/bridges/llm/` |
| G4 | 根 `contracts.go` < 200 行 | `wc -l internal/layers/llmgateway/contracts.go` |
| G5 | `go build ./...` 全绿 | `go build ./...` 退出码 0 |
| G6 | `go test -race ./internal/layers/llmgateway/... ./internal/bridges/llm/...` 全绿 | 测试 100% 绿 |
| G7 | 11 P0 T 100% IMPLEMENTED 状态保持 | `t-registry.md` IMPLEMENTED 列 |
| G8 | 26 T + 9 v1.1 T 全量绿 | 测试报告 |
| G9 | `scripts/check_t_aliases.py` 退出码 0 | 26 alias 100% 覆盖 |
| G10 | `grep -r "D3-S[1-7]" openspec/specs/` 无新增失同步 | 一致性扫描 |
| G11 | runtime span / metric / YAML config key 字面量未改 | `grep` 验证 5 span + 5 metric + 3 YAML key |
| G12 | `layering.md` v3.8.0 D3 章节更新 | 版本号 |
| G13 | `code-layout.md §4.4` 7 个新 slug 注册 | slug 列表 |

---

## 十一、测试策略

完整测试点清单见 `t-registry.md` v3.0.0（26 条：25 IMPLEMENTED + 1 PLANNED，P0 12 条）。本文档不重复列出。

**V3.0.0 关键变化**：
- T ID 按 S/A 编排顺序重排
- T 末尾 `<!-- Mechanism: -->` 注释（R2 命题 A）
- 26 条 Legacy alias 100% 覆盖（`scripts/check_t_aliases.py` 校验）
- D3-S3 ProtectCall 合并 Breaker+Retry 后 T 数 12（4 Breaker + 5 Cross/Retry + 1 PLANNED + 2 Retry）

---

## 十二、版本分期

| 版本 | 能力 | 状态 |
|------|------|------|
| V1 | DeepSeek + MiniMax 适配器 + Circuit Breaker + Token Counter + Retry | Archive |
| V2 | CB+Retry 协调 + Half-Open 并发限制 + Context 超时传播 + Full Jitter + CJK 补偿 | Archive |
| V2.1 | Safety Filter + ModelTier + CacheRead/Reasoning Token 分解 | Archive |
| V3.0.0 | 5+1 S 价值流化 + Legacy Archive + Breaker/Retry 合并 + Fail-Fast 启动 + 灰区声明 | Archive (commit 199ad18 之前) |
| V3.1.0 (v1.1) | `llm_breaker_state` metric + D6 3 probe + EngineEvent 命名 + IAdapter.Protocol() BREAKING + obs nil fail-fast | Archive (commit 3a6970b) |
| **V3.2.0 (v2.0)** | **物理路径迁移** (`route/` `stream/` `stream/adapter/` `protect/` `budget/` `guard/` `configure/`) + **contracts.go 拆分** + re-export 桥接 1 发布周期 | **In Progress** (DM-019) |
| V3.3+ (planned) | Anthropic native API + Rate Limiter + 多模型负载均衡 | P2 路线图 |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.1.0 | 2026-06-14 | 7 S 技术角色词版 |
| 3.0.0 | 2026-06-14 | 5+1 S 价值流化；A + F 编排时序图（§3）；R2 §4.3 contracts.go 拆分粒度占位（§2.1）；R3 P0 #8 fail-fast（§2.2 + §10.1）；R3 P1 #11 Breaker scope 扩展（§6）；R3 P1 #15 IAdapter.Protocol()（§5.1）；R3 P1 #16 Safety 时延（§8.2）；v2.0 物理路径计划（§10.2） |
| 3.1.0 | 2026-06-14 | v1.1 子 change 落地：§3.5 新增 F1-F9 时序图；§9 Feature flag 表更新为 D4-B 决议；§8.4 Safety F04 实施细节；R3 P0 #8 fail-fast 占位 → 实施；R3 P1 #15/#16 占位 → 实施 |
| **3.2.0** | **2026-06-14** | **v2.0 子 change 落地**：§0 V3.1.0→V3.2.0 变更摘要 + 不变性承诺表；§10.2 物理路径 1:1 对齐 5+1 S 详细目录树 + 7 桥接目录 + 根 contracts.go re-export 模板；§13 v2.0 实施步骤（F2-F8 + F9 + F11 详细步骤表）；§14 Phase G 13 项验收 |
| **3.3.0** | **2026-06-19** | **v2.0 状态刷新**（DM-20260619-002）：§5.2 路径 `shared/config/llmgateway.go` → `configure/shared_config.go`（v2.0 已落地）；§10.2 状态从"Phase F 实施中"改为"已完成（DM-20260614-019, 2026-06-14）"；Last Updated 同步至 2026-06-19 |
