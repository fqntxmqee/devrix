# LLM Gateway Specification

**Capability:** llm-gateway
**Change ID:** devrix-llm-gateway (V1, archived), devrix-llm-gateway-v2 (V2, archived), devrix-d3-sa-refine (V3 S/A 重切, archived), **devrix-d3-sa-refine-v1.1 (V3.1 韧性可见性 + 评测探针 + 适配扩展, current)**
**Demand:** DM-20260607-004 (V1), DM-20260608-002 (V2), DM-20260614-016 (V3), **DM-20260614-017 (V3.1)**
**Domain:** D3
**DSAFT Type:** 公共域 (Common Domain)
**Version:** 3.2.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-16
**Domain SoT:** `d3-domain.md` · **Guides:** `terminal-state-guide.md` · `observability-guide.md`

---

## 0. 变更摘要（V2.1.0 → V3.0.0）

| 维度 | V2.1.0 | V3.0.0 |
|------|--------|--------|
| S 切法 | 7 S 技术角色词 | **6 S（5+1）价值流承诺**：RouteModel / StreamChat / ProtectCall / BudgetTokens / GuardContent / ConfigureGateway |
| Spec 编排 | 按 Adapter / Gateway / Breaker / Retry / Token / Config / Safety 7 段 | **按 5+1 S 价值流段**；每段以"承诺 + Feature + Scenario"组织 |
| North Star 5 承诺 | 散落 7 段 | **C1~C5 显式声明** + 1 横切 S6（Config） |
| Bridge | 混入 §L2 Bridge 段 | **归属跨域锚点 `internal/bridges/llm/`**，本 spec 仅 §CROSS 占位 |
| Tier 解析 | 单一 ResolveTier F | **F02a ResolveTierAlias + F02b ResolveDefault**（R2 OQ-4 决议） |
| 灰区声明 | 无 | **D3-S5 GuardContent vs D2-S18 PermissionMode 灰区**（R2 命题 E 决议） |
| T 编号 | 旧 S/A/T | **新 S/A/T + §Legacy Archive 100% alias 追溯**（R1 Q4 + 脚本 `check_t_aliases.py` 校验） |
| Breaker/Retry | 两个独立 S | **合并到 D3-S3 ProtectCall**（R1 D1 决议），T 末尾加 `<!-- Mechanism: -->` 注释（R2 命题 A） |
| 运行时 span 名 | `llm.stream` 等 5 个 | **保持不变**（R1 Q3 + playbook 原则 3） |
| 启动顺序 silent failure | silent fallback | **fail-fast**：`NewFromConfig` obs nil → `ErrObservabilityRequired`（R3 P0 #8） |

## 0.1 变更摘要（V3.0.0 → V3.1.0，v1.1 子 change）

| 维度 | V3.0.0 | V3.1.0 |
|------|--------|--------|
| F 总数（域内） | 24 | **30**（v1.1 增 6 F 域内）+ CROSS 段 3（+1 FailFastOnObsNil） |
| F 新增 | F02a/F02b/F06（F02 拆 + Breaker Cross） | + F07/F08/F09（emit metric / state hook / engine event）+ F04（AdapterProtocol BREAKING）+ F04（EmitSafetyLatencyEvent）+ F05（FeatureFlagDefaults） |
| T 总数 | 26 | **35**（v1.1 增 9 T：6 P0 + 3 P1） |
| Spec 新增段 | §12 V3 韧性可见性 — v1.1 计划（P1 placeholder） | **§13 V3.1 韧性可见性 + 评测探针 + 适配扩展**（9 个 FR 全部落地为 P0/P1） |
| Feature flag | 3 flag 默认 `false` | **D4-B 决议**：emit flag 默认 ON + warn flag 默认 OFF |
| 运行时新增 | — | 3 metric（`llm_breaker_state` / `llm_breaker_transitions_total` / `llm_tier_resolve_total`）+ 1 span event（`safety.check.duration_ms`）+ 3 EngineEvent（`flow.breaker.*`） |
| 跨域契约 | 1（v1.0 D3→D5 metric 命名） | 3（+D3→D6 探针接入 / +D3→D7 EngineEvent 复用） |
| BREAKING 变更 | 0 | 1（`IAdapter.Protocol() string` 接口扩展；3 处实现同步修） |

---

## 1. North Star 5 承诺

> D3 作为公共域，向所有消费者域（D1/D2/D4/D5/D6/D7）承诺以下 5 项可独立验证的能力；每项承诺 1:1 对应一个 S。

| 承诺 | S | 验证入口 | 失败语义 |
|------|---|----------|----------|
| **C1 模型路由**：用户给出 model 名（含 tier alias），D3 必须返回正确 provider + 实际 model | **D3-S1 RouteModel** | `D3-S1-A01-T01/T02` | `ErrUnknownTier` / `ErrNoRoute` / `ErrUnsupportedModel` |
| **C2 流式调用**：用户发起流式聊天，D3 必须返回符合 OpenAI SSE 协议的 chunk 流 | **D3-S2 StreamChat** | `D3-S2-A01-T01~T05` | provider-specific 错误（5xx/4xx） |
| **C3 韧性保护**：Provider 故障（5xx / 网络错误 / 限流），D3 必须不阻塞用户 | **D3-S3 ProtectCall** | `D3-S3-A01-T01~T12` | `CircuitOpenError` / `RetryExhaustedError` |
| **C4 预算控制**：Token 超预算，D3 必须截断或报错，不超额调用 | **D3-S4 BudgetTokens** | `D3-S4-A01-T01~T03` | `TokenBudgetExceededError` |
| **C5 内容守卫**：用户 prompt 命中危险模式，D3 必须拒绝（critical）或告警（warning） | **D3-S5 GuardContent** | `D3-S5-A01-T01/T02` | `safety.Reject`（critical）/ 警告（warning） |
| **横切支撑**：配置加载与验证 | **D3-S6 ConfigureGateway** | `D3-S6-A01-T01` | 启动期 fail（不静默） |

> **承诺装置哲学**（R1 D1）：每个 S 都可以独立验证、独立替换。S 内部的 F 编排可以调整，但 S 的承诺对外是稳定的契约。

---

## 2. DSAFT 结构

| 层级 | ID | 名称 | 说明 |
|------|-----|------|------|
| D | D3 | LLM Gateway | 公共域，提供 LLM 调用横向共享能力 |
| S | **D3-S1** | **RouteModel** | 承诺 C1：模型路由解析 |
| S | **D3-S2** | **StreamChat** | 承诺 C2：流式调用 |
| S | **D3-S3** | **ProtectCall** | 承诺 C3：韧性保护（Breaker + Retry + Fallback 合并） |
| S | **D3-S4** | **BudgetTokens** | 承诺 C4：预算控制 |
| S | **D3-S5** | **GuardContent** | 承诺 C5：内容守卫 |
| S | **D3-S6** | **ConfigureGateway** | 横切：配置加载与验证 |
| S (跨域) | D3-X | CROSS | Bridge / Bootstrap（归属 `internal/bridges/llm/`，R1 D2 决议） |

> **S 与承诺 1:1 对应**：5 承诺 → 5 S；Config 横切 → +1 S；Bridge 跨域 → +1 CROSS S。

---

## 3. D3-S1 RouteModel — 承诺 C1 模型路由

### Feature: Model Routing

#### Scenario: Resolve tier alias to concrete model

- **Given** `ModelTiers` config maps `fast` → `MiniMax-M2.7-highspeed`
- **When** `Router.Resolve("fast")` is called
- **Then** returns provider `minimax`, model `MiniMax-M2.7-highspeed`

#### Scenario: Unknown tier passes through

- **Given** tier name is not in `ModelTiers` config
- **When** `Router.Resolve("unknown")` is called
- **Then** tier is passed through unchanged (router attempts Default fallback in F02b)

#### Scenario: Empty model defaults to DefaultProvider

- **Given** empty model name (zero-value)
- **When** `Router.Resolve("")` is called
- **Then** returns `LLMGatewayConfig.DefaultProvider` + provider's `DefaultModel`

#### Scenario: Model routing by pattern

- **Given** `ModelRouting` config has `deepseek-*` → `deepseek`
- **When** model `deepseek-v4-flash` is resolved
- **Then** provider is `deepseek`, model stays `deepseek-v4-flash`

### Feature: Multi-Provider Concurrency

#### Scenario: Concurrent calls to multiple providers

- **Given** multiple providers configured
- **When** concurrent `Router.Resolve(model)` calls for different models
- **Then** each call returns correct provider+model mapping without cross-talk

### T Reference

- `D3-S1-A01-T01` (P1) — 多 Provider 并发调用（MatchRouting F01）
- `D3-S1-A01-T02` (P1) — 未知 Provider/Model 报错（ResolveTierAlias F02a + ResolveDefault F02b 联动）

---

## 4. D3-S2 StreamChat — 承诺 C2 流式调用

### Feature: DeepSeek Adapter

#### Scenario: Call DeepSeek

- **Given** provider is `deepseek` and model is configured
- **When** `Gateway.Stream()` is called with messages
- **Then** `DeepSeekAdapter` formats request per OpenAI-compatible API spec via `OpenAIStreamClient`
- **And** response is streamed back via channel with SSE parsing

#### Scenario: DeepSeek fallback

- **Given** DeepSeek primary model fails (full retry chain exhausted)
- **When** fallback model is configured in `ProviderConfig`
- **Then** `RetryExecutor` (D3-S3-A01 F05 StreamWithFallback) switches to fallback model

### Feature: MiniMax Adapter

#### Scenario: Call MiniMax

- **Given** provider is `minimax` and model is configured
- **When** `Gateway.Stream()` is called with messages
- **Then** `MiniMaxAdapter` formats request per OpenAI-compatible API spec via `OpenAIStreamClient`
- **And** response is streamed back via channel with SSE parsing

#### Scenario: MiniMax fallback

- **Given** MiniMax primary model fails (full retry chain exhausted)
- **When** fallback model is configured
- **Then** `RetryExecutor` switches to fallback model

### Feature: SSE Parsing & Request Construction

#### Scenario: SSE parse error handling

- **Given** SSE stream contains malformed chunk
- **When** `ParseSSE` processes the stream
- **Then** returns parse error wrapped in stream chunk error metadata

#### Scenario: OpenAI request body construction

- **Given** `llmgateway.Request` with messages and system prompt
- **When** `BuildOpenAIRequest` is called
- **Then** produces valid OpenAI-compatible JSON body

### Feature: Observability (cross-A emit)

#### Scenario: Emit LLM call metrics

- **Given** LLM call is made via `Gateway.Stream`
- **When** call completes (success or error)
- **Then** metrics are emitted via `observability.Bridge.Meter()`:
  - `llm_requests_total` counter (success)
  - `llm_errors_total` counter (error)
  - `llm_latency_seconds` histogram

#### Scenario: Emit OpenTelemetry spans

- **Given** LLM call is in progress
- **When** Gateway processes the request
- **Then** child spans are created for: `llm.stream`, `llm.provider.route`, `llm.circuit_breaker`, `llm.retry`, `llm.adapter.stream`
- **And** span attributes include provider, model, token counts, usage_received flag
- **And** span 名保持不变（R1 Q3 — 运行时字符串不随架构改）

### T Reference

- `D3-S2-A01-T01` (P0) — DeepSeek 适配器流式响应（F01 OpenAIStreamClientStream）
- `D3-S2-A01-T02` (P0) — MiniMax 适配器流式响应（F01）
- `D3-S2-A01-T03` (P1) — SSE parse error handling（F02 ParseSSE）
- `D3-S2-A01-T04` (P1) — OpenAI request body construction（F03 BuildOpenAIRequest）
- `D3-S2-A01-T05` (P1) — LLM 调用可观测事件（spans + metrics emit 跨 A 验证）

---

## 5. D3-S3 ProtectCall — 承诺 C3 韧性保护

> **R1 D1 决议**：Breaker + Retry 合并到 D3-S3 ProtectCall；F 编排 1:1 反映"承诺装置"；T 编号按 F 编排顺序，每个 T 末尾加 `<!-- Mechanism: Breaker / Retry / Cross -->` 注释保留机制可追溯性。

### Feature: Circuit Breaker (Mechanism: Breaker)

#### Scenario: Circuit closed (normal operation)

- **Given** failure count is 0
- **When** LLM call succeeds
- **Then** failure count resets to 0
- **And** circuit state is `closed`

#### Scenario: Circuit opens after threshold

- **Given** failure count exceeds `FailureThreshold` (default: 5)
- **When** LLM call fails
- **Then** circuit state changes to `open`
- **And** subsequent calls are rejected immediately with `CircuitOpenError`
- **And** timer starts for half-open attempt

#### Scenario: Circuit half-open (probe)

- **Given** circuit has been open for `OpenDuration` (default: 30s)
- **When** next LLM call is attempted
- **Then** circuit state changes to `half-open`
- **And** a limited number of probe requests are allowed (`HalfOpenMaxProbes`, default: 1)

#### Scenario: Circuit closes after successful probes

- **Given** circuit is `half-open`
- **When** `SuccessThreshold` consecutive probe requests succeed (default: 2)
- **Then** circuit state returns to `closed`
- **And** failure count resets to 0

#### Scenario: Circuit reopens after failed probe

- **Given** circuit is `half-open`
- **When** probe request fails
- **Then** circuit state returns to `open`
- **And** `OpenDuration` timer restarts

### Feature: Retry Strategy (Mechanism: Retry)

#### Scenario: Retry on transient failure with Full Jitter

- **Given** LLM call fails with retryable error (timeout / provider_unavailable / parse_error)
- **When** retry is enabled
- **Then** request is retried with Full Jitter exponential backoff
- **And** delay is randomized between 0 and cap (capped at `MaxDelay`)

#### Scenario: Fail after max retries

- **Given** LLM call fails on all attempts (primary + fallback)
- **When** max attempts is exhausted
- **Then** error is returned to caller
- **And** Breaker records failure ONCE (not once per attempt)

#### Scenario: Do not retry on auth failure

- **Given** LLM call fails with auth error (401/403)
- **When** request is made
- **Then** error is returned immediately without retry

#### Scenario: Retry delays are randomized

- **Given** a retry config with `InitialDelay=1s`, `Backoff=2.0`
- **When** multiple retry delays are computed
- **Then** the delays are randomized with Full Jitter
- **And** all delays are between 0 and cap (max `MaxDelay`)

### Feature: Breaker + Retry Coordination (Mechanism: Cross)

#### Scenario: Single retry success does not open circuit

- **Given** a provider with CB `failureThreshold=3` and Retry `maxAttempts=3`
- **When** the first 2 attempts fail and the 3rd succeeds
- **Then** the CB remains closed

#### Scenario: All retry attempts fail opens circuit

- **Given** a provider with CB `failureThreshold=3` and Retry `maxAttempts=3`
- **When** all 3 attempts fail with retryable errors
- **Then** Gateway calls `RecordFailure` once (not 3 times)
- **And** the CB opens after the failure

#### Scenario: Half-Open limits concurrent probes

- **Given** a CB in Half-Open state with `HalfOpenMaxProbes=1`
- **When** 3 concurrent requests arrive
- **Then** only 1 request is allowed
- **And** the other 2 receive `CircuitOpenError`

#### Scenario: Context cancellation does not trigger CB failure

- **Given** a provider with CB closed
- **When** a streaming request is cancelled via context
- **Then** the CB remains closed (no `RecordFailure`)

### Feature: Fallback Model

#### Scenario: DeepSeek fallback (full chain exhausted)

- **Given** DeepSeek primary model fails all retries
- **When** fallback model is configured
- **Then** `RetryExecutor` switches to fallback model

#### Scenario: MiniMax fallback (full chain exhausted)

- **Given** MiniMax primary model fails all retries
- **When** fallback model is configured
- **Then** `RetryExecutor` switches to fallback model

### Feature: 429 Rate Limit Handling (Mechanism: Retry)

#### Scenario: Provider returns 429

- **Given** provider returns 429 with `Retry-After` header
- **When** Stream is called
- **Then** retry applies Full Jitter backoff respecting `Retry-After`

### Feature: Context Timeout Propagation

#### Scenario: Gateway injects default timeout when none set

- **Given** the parent context has no deadline
- **When** `Stream` is called
- **Then** the request context has a provider-configured deadline (`ProviderConfig.Timeout` or default 30s)

#### Scenario: Stream goroutine exits on context cancellation

- **Given** an active streaming request
- **When** the context is cancelled
- **Then** the output channel is closed without goroutine leak

### Feature: Breaker State Persistence (PLANNED)

> **PLANNED** — 实施时点 v1.1（与 R3 NQ-1 同步）

#### Scenario: Breaker state restored after restart

- **Given** Breaker state is persisted to durable storage
- **When** Gateway restarts
- **Then** Breaker state is restored from storage (state + failure count + OpenDuration timer)

### T Reference

- `D3-S3-A01-T01` (P0) — Breaker Closed（F01 AllowCircuit + F03 ManageCircuitState）`<!-- Mechanism: Breaker -->`
- `D3-S3-A01-T02` (P0) — Breaker Open（F01 + F02 + F03）`<!-- Mechanism: Breaker -->`
- `D3-S3-A01-T03` (P0) — Breaker HalfOpen→Closed（F01 + F02 + F03）`<!-- Mechanism: Breaker -->`
- `D3-S3-A01-T04` (P0) — Breaker HalfOpen→Open（F01 + F02 + F03）`<!-- Mechanism: Breaker -->`
- `D3-S3-A01-T05` (P0) — Retry+CB 联动（Cancel/Deadline 不触发 CB）`<!-- Mechanism: Cross -->`
- `D3-S3-A01-T06` (P0) — Half-Open 并发探测限制（F01 + F03）`<!-- Mechanism: Breaker -->`
- `D3-S3-A01-T07` (P1) — 429 rate limit handling（F05 StreamWithFallback）`<!-- Mechanism: Retry -->`
- `D3-S3-A01-T08` (P2 PLANNED) — 熔断器状态持久化（F03 + 持久化层）`<!-- Mechanism: Breaker -->`
- `D3-S3-A01-T09` (P0) — 重试策略执行 Full Jitter 退避（F04 + F05）`<!-- Mechanism: Retry -->`
- `D3-S3-A01-T10` (P1) — Full Jitter 随机化验证（F04）`<!-- Mechanism: Retry -->`
- `D3-S3-A01-T11` (P1) — DeepSeek Fallback（F05）`<!-- Mechanism: Retry -->`
- `D3-S3-A01-T12` (P1) — MiniMax Fallback（F05）`<!-- Mechanism: Retry -->`

---

## 6. D3-S4 BudgetTokens — 承诺 C4 预算控制

### Feature: Token Counter

#### Scenario: Count tokens before call

- **Given** messages array with system prompt
- **When** `Counter.CountWithSystemPrompt()` is called
- **Then** total token count is returned (including role/message overhead)

#### Scenario: Token counter accuracy (cl100k_base)

- **Given** text input
- **When** `Counter.CountText()` is called
- **Then** token count matches cl100k_base reference within ±1% tolerance

#### Scenario: Chinese CJK accuracy (P1)

- **Given** CJK text content
- **When** token counting is performed
- **Then** CJK characters are counted per cl100k_base rules; pure ASCII text MUST NOT be affected

### Feature: Budget Check

#### Scenario: Check budget before call

- **Given** token count and budget (default: 128000)
- **When** `Counter.CheckBudget()` is called
- **Then** if within budget, proceed
- **And** if over budget, return `TokenBudgetExceededError`

#### Scenario: Truncate text to token limit

- **Given** text and max token limit
- **When** `Counter.TruncateToTokens()` is called
- **Then** text is truncated to at most `maxTokens`

### T Reference

- `D3-S4-A01-T01` (P0) — Token 计数准确性 cl100k_base（F01 CountText）
- `D3-S4-A01-T02` (P0) — Token 预算检查（F03 CheckBudget）
- `D3-S4-A01-T03` (P1) — Token CJK 准确性（F01）

---

## 7. D3-S5 GuardContent — 承诺 C5 内容守卫

### Feature: Safety Filter

#### Scenario: Content filtering on system prompt and messages

- **Given** Safety Filter is configured with default patterns
- **When** `Filter.Check()` is called with system prompt and messages
- **Then** patterns are matched case-insensitively
- **And** Result includes matches with severity and action

#### Scenario: Reject on critical match

- **Given** system prompt or message matches a critical pattern (e.g., `malware_generation`)
- **When** `Filter.Check()` is called
- **Then** `Result.Allowed` is `false`
- **And** `Result.Reason` describes the rejection

#### Scenario: Warn on medium match

- **Given** message matches a medium-severity pattern (e.g., `prompt_injection`, `hardcoded_credential`)
- **When** `Filter.Check()` is called
- **Then** `Result.Allowed` is `true`
- **And** matches are logged as warnings

### Cross-Domain Boundary (R2 命题 E / P0 #5)

> **D3-S5 GuardContent vs D2-S18 PermissionMode 灰区**：
> - **D3 责任**：prompt 内容过滤（内容本身是否含危险模式）
> - **D2 责任**：tool execution 权限（允许哪些 tool 调用）
> - **灰区场景**：用户 prompt 包含「用 curl 调用内部 API 拿 token」或「用 read_file 读 ~/.ssh/id_rsa」——这既是 prompt 内容（D3 决策）也是 tool execution policy（D2 决策）
> - **灰区处理契约**（R2 命题 E 决议）：当 prompt 内容与 tool execution 存在交叉时，**D3 优先拒**（前置过滤），D2-S18 PermissionMode 仍保留「tool schema 不暴露」作为兜底
> - 详见 `openspec/specs/architecture/cross-domain-boundaries.md` §D3-S5

### T Reference

- `D3-S5-A01-T01` (P0) — critical 拒绝（F01 CheckContent）
- `D3-S5-A01-T02` (P1) — warning 匹配（F01）

---

## 8. D3-S6 ConfigureGateway — 横切配置

### Feature: Model Configuration

#### Scenario: Load provider config via Router

- **Given** provider is configured in `LLMGatewayConfig`
- **When** `Loader.Load(configFile)` is called
- **Then** returns validated `LLMGatewayConfig` and nil error
- **And** `ValidateProviders` checks each provider's required fields

#### Scenario: API key loading

- **Given** provider requires API key
- **When** `LoadAPIKey` is called
- **Then** returns API key from env / file / secret manager

### Feature: Fail-Fast Startup (R3 P0 #8)

#### Scenario: Missing observability bridge fails fast

- **Given** `WireContextLLM` is called with `obs == nil`
- **When** `NewFromConfig` initializes D3 stack
- **Then** returns `ErrObservabilityRequired` (NOT silent fallback to mock)
- **And** Gateway startup fails with clear error

> **背景**：当前实现 silent fallback 会导致 v1.1 翻 `d3_resilience_emit_enabled = true` 时用户感知不到（metric 缺失）；fail-fast 是 v1.0 收尾必做的 P0 硬要求。

### T Reference

- `D3-S6-A01-T01` (P0) — Provider 配置加载与验证（F01 + F03）

---

## 9. CROSS — 跨域锚点（占位声明）

> **R1 D2 决议**：D3 内部 A 不含 Bridge / Bootstrap；它们是 D3 → D2 的契约实现，归属跨域锚点 `internal/bridges/llm/`。本 spec 仅作占位声明。

### Feature: L2 Bridge (CROSS)

#### Scenario: Bridge adapts D3 gateway to ILLMGateway contract

- **Given** `IGateway` implementation
- **When** `Bridge.ChatStream()` is called with `llmgateway.Request`
- **Then** request.Stream is forced to true
- **And** delegates to `IGateway.Stream()`
- **And** returns `<-chan llmgateway.Chunk` for **D7 consumers** (DM-020: primary consumer migrated D2→D7)

#### Scenario: Bridge resolves tier aliases (delegation)

- **Given** Bridge wrapping an IGateway
- **When** `Bridge.ResolveTier(tier)` is called
- **Then** delegates to `IGateway.ResolveTier()`
- **And** returns error if resolved model is empty

### T Reference

- `D3-X-A01-T01` (P1) — Bridge 适配 Gateway → ILLMGateway（D3-X-A01 AdaptToContextEngine）

> **Fail-Fast 改造**（R3 P0 #8）：`internal/bridges/llm/wire.go:WireFromConfig` 在 obs == nil 时返回 `ErrObservabilityRequired`；不 silent fallback。

---

## 10. ADDED Requirements (V2 Reliability 继承 + V3 增补)

### Requirement: CircuitBreaker and Retry Coordination

> **V2 继承**：Gateway MUST 仅在 Retry 链整体失败后调用 `RecordFailure` 一次；`context.Canceled` 与 `context.DeadlineExceeded` MUST NOT 触发 CB failure。Half-Open 状态 MUST 限制并发探测数（`HalfOpenMaxProbes`，默认 1），超限请求返回 `CircuitOpenError`。

- **Priority**: P0
- **S**: D3-S3 ProtectCall
- **T**: `D3-S3-A01-T05`, `D3-S3-A01-T06`

#### Scenario: Single retry success does not open circuit

- **Given** a provider with CB `failureThreshold=3` and Retry `maxAttempts=3`
- **When** the first 2 attempts fail and the 3rd succeeds
- **Then** the CB remains closed

#### Scenario: All retry attempts fail opens circuit

- **Given** a provider with CB `failureThreshold=3` and Retry `maxAttempts=3`
- **When** all 3 attempts fail with retryable errors
- **Then** Gateway calls `RecordFailure` once (not 3 times)
- **And** the CB opens after the failure

#### Scenario: Half-Open limits concurrent probes

- **Given** a CB in Half-Open state with `HalfOpenMaxProbes=1`
- **When** 3 concurrent requests arrive
- **Then** only 1 request is allowed
- **And** the other 2 receive `CircuitOpenError`

#### Scenario: Context cancellation does not trigger CB failure

- **Given** a provider with CB closed
- **When** a streaming request is cancelled via context
- **Then** the CB remains closed (no `RecordFailure`)

### Requirement: Context Timeout Propagation

- **Priority**: P1
- **S**: D3-S3 ProtectCall
- **T**: `D3-S3-A01-T05`

#### Scenario: Gateway injects default timeout when none set

- **Given** the parent context has no deadline
- **When** `Stream` is called
- **Then** the request context has a provider-configured deadline (`ProviderConfig.Timeout` or default 30s)

#### Scenario: Stream goroutine exits on context cancellation

- **Given** an active streaming request
- **When** the context is cancelled
- **Then** the output channel is closed without goroutine leak

### Requirement: Retry Jitter

- **Priority**: P1
- **S**: D3-S3 ProtectCall
- **T**: `D3-S3-A01-T09`, `D3-S3-A01-T10`

#### Scenario: Retry delays are randomized

- **Given** a retry config with `InitialDelay=1s`, `Backoff=2.0`
- **When** multiple retry delays are computed
- **Then** the delays are randomized with Full Jitter
- **And** all delays are between 0 and cap (max `MaxDelay`)

### Requirement: Chinese Token Compensation (P2)

- **Priority**: P2
- **S**: D3-S4 BudgetTokens

#### Scenario: CJK text token counting

- **Given** CJK text content
- **When** token counting is performed
- **Then** CJK multiplier MAY be applied; pure ASCII text MUST NOT be affected

### Requirement: V3 Fail-Fast Startup (R3 P0 #8)

- **Priority**: P0
- **S**: D3-S6 ConfigureGateway + D3-X CROSS (WireFromConfig)
- **T**: （新增 T 推迟到 v1.0 实施时）

#### Scenario: NewFromConfig with nil observability bridge

- **Given** `obsBridge == nil` passed to `WireContextLLM`
- **When** `NewFromConfig` initializes D3 stack
- **Then** returns `ErrObservabilityRequired` (fail-fast, no silent fallback)

### Requirement: V3 Tier Resolution Split (R2 OQ-4)

- **Priority**: P1
- **S**: D3-S1 RouteModel
- **T**: `D3-S1-A01-T02`

#### Scenario: Tier alias resolution

- **Given** `ModelTiers` config maps tier alias to concrete model
- **When** `Router.Resolve(tierAlias)` is called
- **Then** F02a ResolveTierAlias returns concrete model; on `ErrUnknownTier`, F02b ResolveDefault falls back to `DefaultProvider.DefaultModel`

---

## 11. ADDED Requirements (V3 跨域灰区)

### Requirement: D3-S5 GuardContent vs D2-S18 PermissionMode Boundary

- **Priority**: P0
- **S**: D3-S5 GuardContent
- **T**: （灰区场景未触发 T，运行时由 D3 优先拒）

#### Scenario: Prompt content with tool execution overlap

- **Given** user prompt contains content that is BOTH a content-level danger pattern AND a tool execution policy violation
- **When** D3-S5 `Filter.Check()` is called
- **Then** D3-S5 returns `Result.Allowed = false` (D3 优先拒 — 前置过滤)
- **And** D2-S18 PermissionMode still enforces tool schema not exposed (D2 兜底)
- **And** 用户被告知拒绝原因（提示 D3 GuardContent 触发 + 灰区声明 reference）

> 详见 `openspec/specs/architecture/cross-domain-boundaries.md` §D3-S5

---

## 12. ADDED Requirements (V3 韧性可见性 — v1.1 计划)

### Requirement: v1.1 Resilience Metric Emit

- **Priority**: P1（v1.1 实施）
- **S**: D3-S3 ProtectCall
- **T**: （v1.1 实施时新增）

#### Scenario: Breaker state metric emitted

- **Given** `d3_resilience_emit_enabled = true`
- **When** Breaker state changes (Closed ↔ Open ↔ HalfOpen)
- **Then** metric `llm_breaker_state{provider, state}` is emitted
- **And** v1.1 release 后第一个 issue 评估是否升级为 `provider_model` 维度（R3 P1 #11）

#### Scenario: EngineEvent reuse for D7 notification

- **Given** D3 needs to notify D7 of resilience state changes
- **When** Breaker state changes
- **Then** D3 reuses existing `EngineEvent` (`FlowStarted` / `FlowFailed`) — NO new D3→D7 direct contract
- **And** D7 subscribes to EngineEvent for breaker state visibility

> 详细命名（`flow.breaker.opened`?）由 v1.1 第一个 issue 决定（R3 NQ-5）

---

## 13. ADDED Requirements (V3.1 韧性可见性 + 评测探针 + 适配扩展)

> **v1.1 落地**：本节 9 个 FR（FR-1 ~ FR-9）由 v1.1 子 change（DM-20260614-017）实施；F 编号对应 `f-registry.md v3.1.0` F1-F9；T 编号对应 `t-registry.md v3.1.0` 9 新 T。

### Requirement FR-1: Breaker State Metric Emit（D1-A 决议）

- **Priority**: P0
- **S**: D3-S3 ProtectCall（**新 F07 EmitBreakerStateMetric**）
- **T**: `D3-S3-A01-T13`
- **Feature flag**: `d3_resilience_emit_enabled`（**默认 ON**）

#### Scenario: Breaker 状态切换 emit metric

- **Given** Breaker 状态变化（Closed ↔ Open ↔ HalfOpen）
- **When** F03 ManageCircuitState 触发状态变化
- **Then** F08 OnStateTransitionEmit 钩子调用 F07 EmitBreakerStateMetric
- **And** F07 emit metric `llm_breaker_state{provider, state}`（state 取值：closed / half_open / open）
- **And** 受 `d3_resilience_emit_enabled` flag 控制；OFF 时 v1.0 行为完全保持

### Requirement FR-2: Breaker State Transition Hook（D1-A 决议）

- **Priority**: P0
- **S**: D3-S3 ProtectCall（**新 F08 OnStateTransitionEmit**）
- **T**: `D3-S3-A01-T13`

#### Scenario: 状态机钩子幂等保护

- **Given** F03 ManageCircuitState 已切换 Breaker 状态
- **When** 状态变化钩子 F08 触发
- **Then** F08 同步调用 F07 emit metric + F09 emit EngineEvent
- **And** 首次切换后无变化不重复触发（幂等保护）

### Requirement FR-3: EngineEvent Reuse for D7 Notification（D6-A 决议）

- **Priority**: P1
- **S**: D3-S3 ProtectCall（**新 F09 ReuseEngineEvent**）
- **T**: `D3-S3-A14`
- **D7 事件名**（D6-A 决议）：`flow.breaker.opened` / `flow.breaker.closed` / `flow.breaker.halfopened`（3 事件分开）

#### Scenario: Breaker Open 触发 flow.breaker.opened

- **Given** Breaker 状态由 Closed 切换到 Open
- **When** F08 OnStateTransitionEmit 触发
- **Then** F09 emit `flow.breaker.opened` 事件（带 SessionID / FlowID / Timestamp 字段，复用 EngineEvent）
- **And** D7 订阅可见（不新增 D3→D7 直接契约）

#### Scenario: HalfOpen 探测通过 emit flow.breaker.closed

- **Given** Breaker HalfOpen 状态探测通过
- **When** 状态切换到 Closed
- **Then** F09 emit `flow.breaker.closed` 事件

### Requirement FR-4: Bootstrap Fail-Fast on Obs Nil（R3 P0 #8 实施）

- **Priority**: P0
- **S**: D3-X CROSS（**新 F02 FailFastOnObsNil**）
- **T**: `D3-X-A02-T01`
- **错误码**: `ErrObservabilityRequired`（已在 `internal/shared/errors/` 注册）

#### Scenario: WireContextLLM obs nil 失败

- **Given** `WireContextLLM` 被调用
- **When** obs 入参 == nil
- **Then** 立即返回 `ErrObservabilityRequired`，不 silent fallback
- **And** 启动 trace emit `config.load.duration_ms` + 失败原因

#### Scenario: WireContextLLM mock obs 正常

- **Given** 测试 fixture 注入 mock obs（`WithMockObs()` helper）
- **When** `WireContextLLM` 被调用
- **Then** 正常返回 `ContextLLMStack`，无 error

### Requirement FR-5: IAdapter Protocol Method Interface Extension（D3-A 决议）

- **Priority**: P0
- **S**: D3-S2 StreamChat（**新 F04 AdapterProtocolMethod**）
- **T**: `D3-S2-A01-T06`
- **BREAKING 接口扩展**：v1.1 release 时所有 IAdapter 实现必须同步补 `Protocol() string` 方法

#### Scenario: IAdapter Protocol 返回值

- **Given** `IAdapter` 实现（DeepSeekAdapter / MiniMaxAdapter / stubAdapter）
- **When** `Protocol() string` 被调用
- **Then** 返回非空字符串（v1.1 实施时 3 个实现均返回 `"openai-compat"`）

#### Scenario: 旧 IAdapter 实现编译失败

- **Given** 任何未补 `Protocol() string` 的 IAdapter 实现
- **When** 代码编译
- **Then** 编译失败，错误为 `ErrAdapterProtocolNotImplemented`（编译期阻断）

### Requirement FR-6: D6 Probe Tier Resolution Coverage（D2-B 决议）

- **Priority**: P1
- **S**: D3-S1 RouteModel
- **T**: `D3-S1-A01-T03`
- **D6 probe #1 阈值**: 覆盖率 ≥ 99%

#### Scenario: Tier 解析覆盖率

- **Given** D3-S1 `Router.Resolve` 多次调用
- **When** D6 probe 统计调用结果
- **Then** 覆盖率（成功解析 / 总调用）≥ 99%
- **And** D3 emit `llm_tier_resolve_total{outcome=hit|fallback|error}` 配合 D6 探针

### Requirement FR-7: D6 Probe Breaker Anomaly Transition Alert

- **Priority**: P1
- **S**: D3-S3 ProtectCall
- **T**: `D3-S3-A01-T15`
- **D6 probe #2 阈值**: 短时间内 Breaker 反复切换告警

#### Scenario: Breaker 异常切换告警

- **Given** D3-S3 状态机钩子 F08 触发
- **When** 短时间内（默认 5min）Breaker 切换次数 > 阈值
- **Then** D6 probe #2 触发告警 `ErrBreakerAnomalyTransition`
- **And** D3 emit `llm_breaker_transitions_total{provider, from, to}` 配合 D6 探针

### Requirement FR-8: Safety Filter Latency Span Event + D6 Probe #4（D5-A 决议）

- **Priority**: P0
- **S**: D3-S5 GuardContent（**新 F04 EmitSafetyLatencyEvent**）
- **T**: `D3-S5-A01-T03`
- **D6 probe #4 阈值**: P99 < 1ms
- **Feature flag**: `d3_safety_latency_event_enabled`（**默认 ON**）

#### Scenario: Safety filter 计时写入 span event

- **Given** `Filter.Check` 被调用
- **When** 检查结束
- **Then** 当前 trace span 写入 event `safety.check.duration_ms`（F04 EmitSafetyLatencyEvent）
- **And** D6 probe #4 统计 P99；P99 > 1ms 持续 5min 触发 `ErrSafetyLatencyThreshold` 告警

### Requirement FR-9: Feature Flag Defaults（D4-B 决议）

- **Priority**: P0
- **S**: D3-S6 ConfigureGateway（**新 F05 FeatureFlagDefaults**）
- **T**: `D3-S6-A01-T02`

#### Scenario: 3 feature flag 默认值与 D4-B 一致

- **Given** v1.1 release 时 `LLMGatewayConfig` schema
- **When** 加载配置
- **Then** 默认值：
  - `d3_resilience_emit_enabled` = **ON**（保证 Breaker metric 可见）
  - `d3_safety_latency_event_enabled` = **ON**（保证 P99 监测）
  - `d3_metric_emit_warn` = **OFF**（避免 log noise）

#### Scenario: Feature flag OFF 时 v1.0 行为完全保持

- **Given** 单元测试 fixture 显式设置 `d3_resilience_emit_enabled = false`
- **When** Breaker 状态变化
- **Then** 不 emit `llm_breaker_state` metric；v1.0 行为完全保持

### 跨域灰区契约（继承 v1.0）

- D3-S5 vs D2-S18：详见 `cross-domain-boundaries.md` §2.1.3
- D3 → D5 metric 命名边界：详见 `cross-domain-boundaries.md` §2.4.4
- D3 → D7 EngineEvent 复用：详见 `cross-domain-boundaries.md` §2.4.3（D6-A 决议固化）

---

## 13. Archive

- `openspec/archive/2026-06-07-devrix-llm-gateway/` — V1
- `openspec/archive/2026-06-08-devrix-llm-gateway-v2/` — V2
- `openspec/changes/devrix-d3-sa-refine/` — V3 (S/A 重切，**current**)

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.1.0 | 2026-06-14 | 7 S 技术角色词版（V2 Reliability 增补） |
| 3.0.0 | 2026-06-14 | 5+1 S 价值流化：RouteModel/StreamChat/ProtectCall/BudgetTokens/GuardContent/ConfigureGateway；North Star 5 承诺显式声明；R2 灰区声明（§11）；R3 fail-fast（P0 #8）；Breaker+Retry 合并 ProtectCall；T ID 重排 + Legacy Archive 100% alias 追溯 |
| 3.1.0 | 2026-06-14 | 韧性可见性 + 评测探针 + 适配扩展（v1.1 子 change 落地）：6 F 域内新增（F07/F08/F09/F04/F04/F05）+ 1 F CROSS 段（F02 FailFastOnObsNil）；9 T 新增（6 P0 + 3 P1）；§13 V3.1 Requirements 9 个 FR（FR-1 ~ FR-9）；D1-A / D2-B / D3-A / D4-B / D5-A / D6-A / D7-A R1 决议固化；3 新 metric + 1 新 span event + 3 新 event；`IAdapter.Protocol() string` BREAKING 接口扩展（3 处实现同步修） |
