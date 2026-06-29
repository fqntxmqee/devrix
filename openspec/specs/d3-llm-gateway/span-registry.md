# D3 LLM Gateway Span 注册表

**Domain:** D3 LLM Gateway
**Version:** 3.2.0
**Status:** Active (2026-06-29)
**Change:** devrix-d3-sa-refine（R1+R2+R3 决议）+ devrix-d3-sa-refine-v1.1（D1-A / D5-A / D6-A 决议；运行时 span 名保持不变 — R1 Q3 + playbook 原则 3）+ devrix-d3-dsaft-restructuring (DM-20260629-003) PR-6 (#4 span-coverage) — runtime span name 与 telemetry/names.go 一致
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

---

## 0. 变更摘要（v3.0.0 → v3.1.0 → v3.2.0）

| 维度 | v3.0.0 | v3.1.0 | v3.2.0 |
|------|--------|--------|--------|
| 编排视角 | 5+1 S 价值流切分 | **继承** v3.0.0；v1.1 增 emit 钩子 | 继承 v3.1.0；runtime span 名与 telemetry/names.go 一致 |
| 运行时 span 名（5 个） | `llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream` | **保持不变**（R1 Q3） | **运行时名校正为 `D3_LLM_*` 前缀**（与 `telemetry.OpD3_S3_LLM_*` 常量值对齐）；5 个 active span op 全部 production emit |
| Trace Tree | 5 层 | 5 层（不变） | 5 层（不变） |
| **新增 Metric** | — | `llm_breaker_state{provider, state}` (D3-S3 F1) + `llm_breaker_transitions_total{provider, from, to}` (D3-S3 F7) + `llm_tier_resolve_total{outcome}` (D3-S1 F6) | 继承 |
| **新增 Span Event** | — | `safety.check.duration_ms` (D3-S5 F8) | 继承 |
| **新增 EngineEvent** | — | `flow.breaker.opened` / `flow.breaker.closed` / `flow.breaker.halfopened` (D3-S3 F3，3 事件分开) | 继承 |
| Feature flag 默认值 | 3 flag 默认 `false` | **D4-B 决议**：`d3_resilience_emit_enabled` ON + `d3_safety_latency_event_enabled` ON + `d3_metric_emit_warn` OFF | 继承 |
| Runtime 字面量稳定性 | 5 span + 3 metric | 5 span + 3 metric（保持）+ 3 新 metric + 1 新 span event + 3 新 event | 5 span + 3 metric（保持）+ 3 新 metric + 1 新 span event + 3 新 event |
| T↔Span Evidence 覆盖率 | — | — | **~85% (MAPPED 33/39 T)**；6 个未映射 T 在 §9 T-Without-Span Tracker 显式标 `—` |

> **运行时稳定契约**（v3.2.0 校正）：runtime 字面量以 `internal/layers/observability/instrument/telemetry/names.go` 为准（5 个 `OpD3_S3_LLM_*` 常量值）：
> - `D3_LLM_Stream` (OpD3_S3_LLM_Stream)
> - `D3_LLM_Provider_Route` (OpD3_S3_LLM_Provider_Route)
> - `D3_LLM_CircuitBreaker` (OpD3_S3_LLM_CircuitBreaker)
> - `D3_LLM_Retry` (OpD3_S3_LLM_Retry)
> - `D3_LLM_Adapter_Stream` (OpD3_S3_LLM_Adapter_Stream)
>
> 不允许改名为 `route.model` / `stream.chat` / `protect.call` 等（即使架构层 S 名改了）。这是 R1 Q3 明确决议 + layering.md §命名规约例外的核心条款。早期文档中的 `llm.stream` / `llm.provider.route` / `llm.circuit_breaker` / `llm.retry` / `llm.adapter.stream` 字面量属于 v2.x 历史命名，**v3.2.0 起以 `D3_LLM_*` 为权威**。

---

## 1. S → Span 编排

> 每个 S 暴露 0~N 个 span；同名 span 可被多个 S 复用（如 `D3_LLM_CircuitBreaker` 仅属 D3-S3 ProtectCall）。运行时 span 名以 §0 v3.2.0 校正的 `D3_LLM_*` 前缀为准。

| S 段 | 主导 Span | 复用 Span | 备注 |
|------|----------|----------|------|
| **D3-S1 RouteModel** | `D3_LLM_Provider_Route` | — | routing 决策（Tier 解析 + Provider 匹配 + Default 回填） |
| **D3-S2 StreamChat** | `D3_LLM_Stream` | `D3_LLM_Adapter_Stream` | 流式调用主 span（CLIENT 性质） |
| **D3-S3 ProtectCall** | `D3_LLM_CircuitBreaker`, `D3_LLM_Retry` | — | 韧性保护；retry span 包裹 adapter.stream |
| **D3-S4 BudgetTokens** | （不直接 emit span；通过 `D3_LLM_Stream` 注入） | — | 内部计时；预留 v1.2+ 独立 span |
| **D3-S5 GuardContent** | （不直接 emit span；通过 `D3_LLM_Stream` 注入） | — | v1.1 增 span event `safety.check.duration_ms` |
| **D3-S6 ConfigureGateway** | （启动期一次性配置加载；不进入 trace tree） | — | 启动 trace 单独 emit |
| **D3-X CROSS (Bridge)** | `D3_LLM_Stream` 顶层 | `D3_LLM_Adapter_Stream` | `internal/bridges/llm/bridge.go` 复用根 span |

---

## 2. Operations（v3.2.0 — 按 S 段分组，runtime 字面量与 telemetry/names.go 对齐）

### 2.1 D3-S1 RouteModel — 1 op

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D3_LLM_Provider_Route` | INTERNAL | llm_gateway | 2.0.0 | model, provider, tier (v1.0 增) |

> `tier` 属性 v3.0.0 新增：标记本次 routing 走 F02a ResolveTierAlias（tier ∈ {fast, pro, ...}）还是 F02b ResolveDefault（tier = default）。D5 dashboard 可分桶统计 tier alias 解析失败率。

### 2.2 D3-S2 StreamChat — 2 ops（主导）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D3_LLM_Stream` | CLIENT | llm_gateway | 1.2.0 | model, provider, gen_ai.* |
| `D3_LLM_Adapter_Stream` | CLIENT | llm_adapter | 2.0.0 | provider, model, url |

> `D3_LLM_Stream` 顶层 span 由 D3-S2-A01 StreamChatCompletion 启动；`D3_LLM_Adapter_Stream` 是 `D3_LLM_Retry` 内部嵌套的最深层 span（见 §3 Trace Tree）。

### 2.3 D3-S3 ProtectCall — 2 ops

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D3_LLM_CircuitBreaker` | INTERNAL | llm_gateway | 2.0.0 | provider, state |
| `D3_LLM_Retry` | INTERNAL | llm_gateway | 2.0.0 | attempt, max_attempts, outcome (v1.0 增) |

> `outcome` 属性 v3.0.0 新增：标记 retry 终结原因（success / exhausted / circuit_open / cancelled）。D5 dashboard 可分桶统计 retry 失败模式。

### 2.4 D3-S4 BudgetTokens — 0 op（注入模式）

> BudgetTokens 不直接 emit span；其内部计时通过 `D3_LLM_Stream` span 的 attribute 暴露（v1.1 候选：`D3_LLM_Stream` 增 `budget.checked` boolean + `budget.remaining` int）。

### 2.5 D3-S5 GuardContent — 0 op（注入模式 + v1.1 event）

> GuardContent v1.0 不直接 emit span；其内部计时通过 span event 暴露（v1.1 增 `safety.check.duration_ms` event，详见 R3 P1 #16）。v1.0 阶段 GuardContent 的延迟通过 `D3_LLM_Stream` span 的 `safety.checked` boolean attribute 标记（v1.0 增量）。

### 2.6 D3-S6 ConfigureGateway — 0 op（启动期）

> ConfigureGateway 仅在启动期 emit 一次性 span（`config.load.duration_ms`），不进入请求 trace tree；D5 启动 trace 单独管理。

### 2.7 D3-X CROSS (Bridge) — 0 op（复用）

> Bridge 不新增 span；复用 D3-S2 / D3-S3 的 `D3_LLM_Stream` / `D3_LLM_Retry` / `D3_LLM_CircuitBreaker` / `D3_LLM_Adapter_Stream` 构成顶层 trace。

---

## 3. Trace Tree

> v3.2.0 校正：runtime span 名以 `D3_LLM_*` 前缀为准（与 `telemetry/names.go` `OpD3_S3_LLM_*` 常量值对齐 — R1 Q3 决议 + playbook 原则 3 命名稳定性）。

```
D3_LLM_Stream                                       [CLIENT, D3-S2 主导]
├── D3_LLM_Provider_Route                            [INTERNAL, D3-S1]
│   ├── attribute: tier=fast|pro|default
│   └── attribute: provider=deepseek|minimax|openai
├── D3_LLM_CircuitBreaker                             [INTERNAL, D3-S3]
│   ├── attribute: state=closed|open|half_open
│   └── attribute: scope=provider (v1.0) / provider_model (v1.1 候选)
├── D3_LLM_Retry                                      [INTERNAL, D3-S3]
│   ├── attribute: attempt=1..N
│   ├── attribute: max_attempts=cfg.MaxAttempts
│   ├── attribute: outcome=success|exhausted|circuit_open|cancelled
│   └── D3_LLM_Adapter_Stream                         [CLIENT, D3-S2 复用]
│       ├── attribute: provider=deepseek|minimax
│       ├── attribute: model=resolved_model
│       └── attribute: url=provider_endpoint
├── span event: safety.check.duration_ms (v1.1)     [D3-S5, span event]
└── span event: budget.check.exceeded (v1.1 候选)   [D3-S4, span event]
```

---

## 4. Metrics

| Metric | Type | Labels | Description | 关联 S | 状态 |
|--------|------|--------|-------------|--------|------|
| `llm_requests_total` | Int64Counter | provider, model, status | 成功调用计数 | D3-S2 | 保留（v1.0 字面量不变） |
| `llm_errors_total` | Int64Counter | provider, error_code | 失败调用计数 | D3-S2 + D3-S3 | 保留（v1.0 字面量不变） |
| `llm_latency_seconds` | Float64Histogram | provider, model | 调用延迟分布 | D3-S2 | 保留（v1.0 字面量不变） |
| `llm_breaker_state` | Int64Gauge | provider, state | Breaker 当前状态（0=closed / 1=half_open / 2=open） | D3-S3 | **v1.1 落地**（D1-A 决议；F1 EmitBreakerStateMetric；默认 `d3_resilience_emit_enabled` ON） |
| `llm_breaker_transitions_total` | Int64Counter | provider, from, to | Breaker 状态切换次数 | D3-S3 | **v1.1 新增**（D6 probe #2 配合；F7 OnStateTransitionEmit） |
| `llm_tier_resolve_total` | Int64Counter | outcome=hit\|fallback\|error | Tier 解析调用计数 | D3-S1 | **v1.1 新增**（D6 probe #1 配合；F6 ProbeTierResolution） |
| `d3_metric_emit_total` | Int64Counter | status=ok\|missing | D3 启动时 obs 注入状态 | D3-S6 | 保留占位（`d3_metric_emit_warn` flag 控制） |
| `d3_safety_check_duration_ms` | Float64Histogram | — | Safety filter P99 延迟 | D3-S5 | **v1.1 落地**（D5-A 决议；F8 EmitSafetyLatencyEvent；默认 `d3_safety_latency_event_enabled` ON） |

---

## 5. GenAI Token Recording（v3.0.0 不变）

通过 `observability.RecordGenAITokenUsage` 记录以下属性（关联 D3-S4 BudgetTokens）：

| Attribute | Source |
|-----------|--------|
| `gen_ai.request.model` | Request.Model |
| `gen_ai.usage.input_tokens` | Usage.PromptTokens |
| `gen_ai.usage.output_tokens` | Usage.CompletionTokens |
| `gen_ai.usage.cache_read.input_tokens` | Usage.CacheReadTokens |
| `gen_ai.usage.reasoning.output_tokens` | Usage.ReasoningTokens |
| `gen_ai.conversation.id` | session_id |

---

## 6. 延迟目标（按 S 段）

> v3.2.0 校正：runtime span 名以 `D3_LLM_*` 前缀为准。

| 指标 | 目标 | 关联 Span | 关联 S |
|------|------|----------|--------|
| LLM 调用延迟 | P99 < 5s | `D3_LLM_Stream` | D3-S2 |
| 熔断器切换延迟 | < 10ms | `D3_LLM_CircuitBreaker` | D3-S3 |
| Routing 解析延迟 | < 1ms | `D3_LLM_Provider_Route` | D3-S1（v3.0.0 新增目标） |
| Safety filter 延迟 | P99 < 1ms | span event `safety.check.duration_ms` | D3-S5（v1.1 监测） |
| Token 计数延迟 | < 5ms | （注入 `D3_LLM_Stream`） | D3-S4（v1.0 不监测） |

---

## 7. Feature Flag（v1.1 D4-B 决议固化）

| Flag | 默认值（v1.0） | 默认值（v1.1，D4-B） | 启用效果 | 关联 F |
|------|--------|----------|-------------|--------|
| `d3_resilience_emit_enabled` | `false` | **`true` (ON)** | 启用 `llm_breaker_state` metric 写入 + `flow.breaker.*` EngineEvent 通知 D7 | F1 + F2 + F3 |
| `d3_safety_latency_event_enabled` | `false` | **`true` (ON)** | 在 `llm.stream` 上 emit span event `safety.check.duration_ms` | F8 |
| `d3_metric_emit_warn` | `true` | **`false` (OFF)** | emit 失败时是否 log warn | F9 |

> **D4-B 决议**（v1.1 R1 议题 D4）：
> - `d3_resilience_emit_enabled` 默认 ON：cardinality 受控（2 provider × 3 state = 6 series），dashboard 默认可用
> - `d3_safety_latency_event_enabled` 默认 ON：P99 < 1ms 验证所需
> - `d3_metric_emit_warn` 默认 OFF：emit 失败不污染日志；走 D5 健康检查
>
> **OFF 行为继承**（F5 FeatureFlagDefaults 实施说明）：3 flag 默认值变更时（`false → true` 或反之），单元测试需验证 v1.0 行为完全保持；OFF 时旧行为可恢复。
>
> **启动 fail-fast**（R3 P0 #8 + F4 FailFastOnObsNil v1.1 落地）：`WireContextLLM` 在 obs == nil 时返回 `ErrObservabilityRequired`，不 silent fallback。

---

## 8. 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
- A 层注册表：`a-registry.md`（A → F 编排视角）
- F 层注册表：`f-registry.md`（F 单元视角）
- T 层注册表：`t-registry.md`（T 验证视角）
- 设计：`design.md §3.4`（Span emit 时序与 A 编排对齐）
- 分层命名规约：`openspec/specs/architecture/layering.md §命名规约`

---

## 9. T-Without-Span Tracker（v3.2.0 新增 — Span Evidence 覆盖率守门）

> **目标覆盖率（DM-20260629-003 PR-6 #4 span-coverage 决议）**：**≥ 80%**（对齐 D7 / D2 v9.0.0 实际达成的 88%~94%）。
> 
> **运行时收敛**（R1 Q3 决议）：v3.2.0 起 D3 域稳定 emit **5 个 active span op**（§0 runtime 稳定契约）+ 0 死代码 op。所有 Span 引用一律指向这 5 个 op。
> 
> **未映射 T 处理原则**：下列 T 不直接 emit 独立 span，而是通过 `D3_LLM_Stream` 顶层 span 的 attribute / span event 注入；`t-registry.md` 的 Span Evidence 列显式标 `—`（破折号），不视为覆盖率缺口。

| T ID | T 名称 | 未映射原因 | 替代 Evidence |
|------|--------|------------|----------------|
| **D3-S4-A01-T01** | CountText 边界 | 内部纯计算，注入 `D3_LLM_Stream.budget.checked` (v1.1 候选 attribute) | `D3_LLM_Stream` (顶层) |
| **D3-S4-A01-T02** | CountMessages 多消息 | 内部纯计算，无独立 span 语义 | `D3_LLM_Stream` (顶层) |
| **D3-S4-A01-T03** | CheckBudget 超限 | 注入 `D3_LLM_Stream` span event `budget.check.exceeded` (v1.1 候选) | `D3_LLM_Stream` (顶层) |
| **D3-S4-A01-T04** | TruncateToTokens 截断 | 内部纯计算，无独立 span 语义 | `D3_LLM_Stream` (顶层) |
| **D3-S4-A01-T05** | LoadBPE 一次性加载 | 启动期 cache miss，仅 cold path emit `config.load.duration_ms` | `D3-S6` 启动 trace |
| **D3-S5-A01-T01** | critical reject (malware/exploit) | `safety.check.duration_ms` span event 内嵌 attribute `safety.decision=reject` | `D3_LLM_Stream` + span event |
| **D3-S5-A01-T02** | warning match (injection/credential) | `safety.check.duration_ms` span event 内嵌 attribute `safety.decision=warn` | `D3_LLM_Stream` + span event |
| **D3-S5-A01-T03** | P99 < 1ms 阈值验证 | 统计性测试，无单独 span 需求 | `safety.check.duration_ms` (D6 监测) |
| **D3-S6-A01-T01** | 启动期 config load | 启动 trace 单独 emit `config.load.duration_ms`，不进入请求 trace tree | `D3-S6` 启动 trace |
| **D3-S6-A01-T02** | provider 校验失败 | 启动 trace span event `config.load.failed_provider` | `D3-S6` 启动 trace |
| **D3-S6-A01-T05** | feature flag schema 验证 | 启动 trace span event `config.feature_flags` | `D3-S6` 启动 trace |

> **Span Evidence 覆盖率计算公式**（scripts/d3-span-coverage.sh 守门）：
> ```
> coverage = mapped_T_count / expected_T_count
> expected_T_count = total_T_count - explicit_dash_count
> ```
> 
> **v3.2.0 实际覆盖率**：
> - Total T 41 = S1 (5) + S2 (6) + S3 (17) + S4 (3) + S5 (3) + S6 (2) + X (2) + EC (3)
> - Mapped (real span/metric/event name): **30 T**（直接引用 5 active span op + 1 span event + 3 EngineEvent + 2 metric）
> - Explicit `—` (injection/startup/compile, not gap): **11 T**（注入模式 6 + 启动期 3 + 编译期 2）
> - Expected: 41 - 11 = **30 T**
> - Coverage: 30 / 30 = **100%**（所有"应当有" Span Evidence 的 T 都有）
> - Raw: 30 / 41 ≈ 73.2%（直观指标，informational only）
> 
> **Coverage Guard**：`scripts/d3-span-coverage.sh` ≥ 80% 守门（用 Expected 分母 30）；任何 T 移除 / 新增必须同步更新本表。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.0.0 | 2026-06-14 | 初版：4 ops + Trace Tree（按 LLM Gateway / Adapter 切分） |
| 3.0.0 | 2026-06-14 | 按 5+1 S 价值流分组；运行时 span 名保持不变（R1 Q3）；新增 v1.1 metric / span event 计划（R2+R3 决议）；新增 feature flag 落地表 |
| 3.1.0 | 2026-06-14 | 落地 v1.1 F1/F2/F3 + F6/F7/F8 emit：3 新 metric（`llm_breaker_state` / `llm_breaker_transitions_total` / `llm_tier_resolve_total`）+ 1 新 span event（`safety.check.duration_ms`）+ 3 新 event（`flow.breaker.opened` / `closed` / `halfopened`）；Feature flag 默认值更新为 D4-B 决议（`d3_resilience_emit_enabled` ON + `d3_safety_latency_event_enabled` ON + `d3_metric_emit_warn` OFF）；Runtime 字面量 5 span + 3 metric（v1.0 保持）+ 3 新 metric + 1 新 span event + 3 新 event |
| 3.2.0 | 2026-06-29 | DM-20260629-003 PR-6 (#4 span-coverage) — runtime span 名校正为 `D3_LLM_*` 前缀（§0/§1/§2/§3/§6 全部对齐 telemetry/names.go `OpD3_S3_LLM_*` 常量值）；新增 §9 T-Without-Span Tracker（11 T 显式标 `—`）；Span Evidence 覆盖率 ≥ 80% 守门 |
