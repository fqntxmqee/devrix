# D3 LLM Gateway Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.1.0
**Last Updated:** 2026-06-29
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d3-sa-refine（R1+Q1-Q7 决议 + R2 命题 A/OQ-4 决议 + R3 命题 A~D 自裁决）
**Domain SoT:** `d3-domain.md`
**Companion Docs:** `terminal-state-guide.md` · `observability-guide.md` · `f-registry.md` · `t-registry.md` · `span-registry.md` · `spec.md` · `design.md`

---

## 0. 变更摘要（v2.1.0 → v3.0.0）

| 维度 | v2.1.0 | v3.0.0 |
|------|--------|--------|
| 场景数 | 7（技术角色词） | **6（5+1 价值流承诺）** |
| 切法 | Adapter / Gateway / Breaker / Retry / Token / Config / Safety | RouteModel / StreamChat / ProtectCall / BudgetTokens / GuardContent / ConfigureGateway |
| A 数 | 7 | 6 |
| F 数 | 22 | 22（保留；F02 拆 F02a/F02b，ProtectCall 合并 Breaker+Retry 后总 F 数不变） |
| Bridge / Bootstrap | 混入 D3-S2 内部 A 末尾 | **移至 §CROSS** 段（跨域锚点 `internal/bridges/llm/`，R1 D2 决议） |
| T 追溯 | 直读 | t-registry.md 末尾 `<!-- Mechanism: -->` 注释（R2 命题 A 决议） |
| Tier 解析 | 单一 F02 ResolveTier | F02a ResolveTierAlias + F02b ResolveDefault（R2 OQ-4 决议） |

> **Backward compatibility**：v3.0.0 不改运行时行为、不删旧 ID；旧 ID 通过 `t-registry.md §Legacy Archive` 100% alias 追溯。`scripts/check_t_aliases.py` 校验覆盖。

---

## 1. S 切法总览（5+1 价值流承诺）

> S = 价值流承诺（playbook 原则 1「S 是可被独立验证的承诺」）；scenario-slug 全部语义化、无技术角色词、Go 合法目录名（`code-layout.md §2`）。

| S ID | S Name | scenario-slug | 承诺（North Star 5 承诺映射） | 北极星承诺归属 | ValueFlow Alias (用户感知) |
|------|--------|---------------|------------------------------|----------------|----------------------------|
| **D3-S1** | **RouteModel** | `route/` | C1：用户给出 model 名（含 tier alias），D3 必须返回正确 provider + 实际 model | C1 模型路由 | `D3_Model_Routing` |
| **D3-S2** | **StreamChat** | `stream/` | C2：用户发起流式聊天，D3 必须返回符合 OpenAI SSE 协议的 chunk 流 | C2 流式调用 | `D3_Stream_Chat_Completion` |
| **D3-S3** | **ProtectCall** | `protect/` | C3：Provider 故障（5xx / 网络错误 / 限流），D3 必须不阻塞用户（Breaker + Retry + Fallback） | C3 韧性保护 | `D3_Circuit_Breaker_And_Retry` |
| **D3-S4** | **BudgetTokens** | `budget/` | C4：Token 超预算，D3 必须截断或报错，不超额调用 | C4 预算控制 | `D3_Token_Budget_Control` |
| **D3-S5** | **GuardContent** | `guard/` | C5：用户 prompt 命中危险模式，D3 必须拒绝（critical）或告警（warning） | C5 内容守卫 | `D3_Content_Safety_Filter` |
| **D3-S6** | **ConfigureGateway** | `configure/` | （横切支撑）配置加载与验证；不挂承诺 | 横切 | `D3_Gateway_Configuration` |

**S 与承诺 1:1 对应**（R1 D1 决议）：5 承诺 → 5 S；Config 横切 → +1 S。

---

## 2. A 层编排

> 每个 S 下当前为单一 A（合并型 S 仅 1 个 A 编排全部 F；拆分型 S 可扩展为多 A 取决于未来 A 边界细化）。

### 2.1 D3-S1 RouteModel — 路由解析

**承诺 C1**：用户给出 model 名（含 tier alias 如 `fast` / `pro`），D3 必须返回正确 provider + 实际 model 名。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D3-S1-A01** | **ResolveModelRoute** | A-BE | model_name (string) | (provider, resolved_model, error) | — | `internal/layers/llmgateway/route/router.go`（`Router.Resolve`） |

**F 编排**（详见 `f-registry.md` §3.1）：

```
D3-S1-A01 ResolveModelRoute
  ├─ F01 MatchRouting         (model → provider)
  ├─ F02a ResolveTierAlias    (tier alias → concrete model)  <!-- Tier -->
  └─ F02b ResolveDefault      (empty → default model)        <!-- Default -->
```

> **F02 拆分依据**：R2 命题 D / OQ-4（playbook 原则 6「F 是可被 A 编排的最小业务/技术逻辑单元」）；Tier alias 解析与 Empty model 默认是两种不同的最小单元，错误码不同（`ErrUnknownTier` vs `ErrNoRoute` / `ErrUnsupportedModel`）。

---

### 2.2 D3-S2 StreamChat — 适配器流式调用

**承诺 C2**：用户发起流式聊天，D3 必须返回符合 OpenAI SSE 协议的 chunk 流。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D3-S2-A01** | **StreamChatCompletion** | A-BE | ctx, llmgateway.Request | <-chan *AdapterChunk | — | `internal/layers/llmgateway/stream/adapter/openai_stream.go`（`OpenAIStreamClient.Stream`） |

**F 编排**（详见 `f-registry.md` §3.2）：

```
D3-S2-A01 StreamChatCompletion
  ├─ F01 OpenAIStreamClientStream  (provider-agnostic stream)
  ├─ F02 ParseSSE                  (SSE → AdapterChunk)
  └─ F03 BuildOpenAIRequest        (Request → OpenAI JSON body)
```

> **Provider-specific 适配**（DeepSeekAdapter / MiniMaxAdapter）通过 `IAdapter` 接口复用 F01+F03，Provider 路由在 `D3-S1` 完成（参见 `design.md §3.2 A 编排`）。F03 归属本 A（StreamChat）而非 D3-S1（RouteModel），理由是请求体构造是流式调用的"前置生产"动作，与路由解析（model 名 → provider）无业务耦合。
> **V3 扩展点**：R3 命题 C 决议，v1.0 release 后第一个 issue 增 `IAdapter.Protocol() string` 方法；本 A 不动。

---

### 2.3 D3-S3 ProtectCall — 韧性保护（Breaker + Retry + Fallback 合并）

**承诺 C3**：Provider 故障（5xx / 网络错误 / 限流），D3 必须不阻塞用户（Breaker + Retry + Fallback）。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D3-S3-A01** | **ShieldAndRetry** | A-BE | ctx, call, primary_model, fallback_model, retry_config | <-chan *AdapterChunk | circuit.{closed, open, half_open} | `internal/layers/llmgateway/protect/circuit_breaker.go` + `internal/layers/llmgateway/protect/retry.go` |

**F 编排**（详见 `f-registry.md` §3.3）：

```
D3-S3-A01 ShieldAndRetry
  ├─ F01 AllowCircuit              (Breaker.Allow)             <!-- Breaker -->
  ├─ F02 RecordOutcome             (RecordSuccess/Failure)     <!-- Breaker -->
  ├─ F03 ManageCircuitState        (Closed/Open/HalfOpen)      <!-- Breaker -->
  ├─ F04 ComputeBackoff            (Full Jitter)               <!-- Retry -->
  ├─ F05 StreamWithFallback        (Retry.Executor.Stream)     <!-- Retry -->
  └─ F06 ShouldRecordBreakerFailure (Cancel/Deadline 不触发)    <!-- Cross -->
```

> **Breaker + Retry 合并依据**（R1 D1 + R2 命题 A）：承诺 C3「Provider 故障不阻塞我」是同一承诺的两个机制；F 编排 1:1 反映"承诺装置"。T 编号按 F 编排顺序排列，t-registry.md 每个 T 末尾加 `<!-- Mechanism: Breaker / Retry / Cross -->` 注释保留机制可追溯性。
> **v1.1 扩展点**（R3 命题 A）：`CircuitBreakerConfig.Scope` 字段扩展为枚举 `provider` / `provider_model` / `model`；v1.0 默认 `provider` 不变。

---

### 2.4 D3-S4 BudgetTokens — 预算控制

**承诺 C4**：Token 超预算，D3 必须截断或报错，不超额调用。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D3-S4-A01** | **CountAndCheckLLMTokens** | A-BE | text / []Message / system_prompt | token_count, error/nil | — | `internal/layers/llmgateway/budget/counter.go` + `bpe_loader.go` |

**F 编排**（详见 `f-registry.md` §3.4）：

```
D3-S4-A01 CountAndCheckLLMTokens
  ├─ F01 CountText
  ├─ F02 CountMessages
  ├─ F03 CheckBudget
  ├─ F04 TruncateToTokens
  └─ F05 LoadBPE
```

---

### 2.5 D3-S5 GuardContent — 内容守卫

**承诺 C5**：用户 prompt 命中危险模式，D3 必须拒绝（critical）或告警（warning）。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D3-S5-A01** | **FilterAndMatchContent** | A-BE | ctx, system_prompt, []messages | *safety.Result | — | `internal/layers/llmgateway/guard/filter.go` + `patterns.go` |

**F 编排**（详见 `f-registry.md` §3.5）：

```
D3-S5-A01 FilterAndMatchContent
  ├─ F01 CheckContent
  ├─ F02 LoadPatterns
  └─ F03 MatchPattern
```

> **跨域边界**：D3-S5 GuardContent 负责 prompt 内容过滤；D2-S18 PermissionMode 负责 tool execution 权限。**灰区声明**（R2 命题 E）见 `openspec/specs/architecture/cross-domain-boundaries.md` §D3-S5 —— 当 prompt 内容与 tool execution 存在交叉时，D3 优先拒（前置过滤），D2 兜底。

---

### 2.6 D3-S6 ConfigureGateway — 配置加载（横切）

**承诺**：无（横切支撑）；为 S1~S5 提供运行时配置。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| **D3-S6-A01** | **LoadAndValidateLLMConfig** | A-BE | config_file | LLMGatewayConfig | — | `internal/layers/llmgateway/configure/loader.go` + `internal/layers/llmgateway/configure/shared_config.go` |

**F 编排**（详见 `f-registry.md` §3.6）：

```
D3-S6-A01 LoadAndValidateLLMConfig
  ├─ F01 LoadConfig
  ├─ F02 BuildConfig
  ├─ F03 ValidateProviders
  └─ F04 LoadAPIKey
```

---

## 3. CROSS — 跨域锚点（Bridge / Bootstrap）

> **R1 D2 决议**：D3 内部 A 不含 Bridge / Bootstrap；它们是 D3 → D2 的契约实现，归属跨域锚点 `internal/bridges/llm/`。本段仅作"占位声明"，完整 A/F/T 注册见 `internal/bridges/llm/`。
> 
> **DM-020（D7 Turn 编排上移）修订：** ILLMGateway 主消费方从 D2 变更为 **D7**。v1.0 保留 Legacy 别名 `AdaptToContextEngine`；v2.0 增 `AdaptToOrchestrator`。

| A ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| **D3-X-A01** | **AdaptToContextEngine**（Legacy 别名） | A-BE | llmgateway.Request | <-chan Chunk (via ILLMGateway) | `internal/bridges/llm/bridge.go`（`Bridge.ChatStream`） |
| **D3-X-A02** | **WireLLMStack** | A-BE | config_file, obs | ContextLLMStack | `internal/bridges/llm/context_wiring.go`（`WireContextLLM` + `WireFromConfig`） |
| **D3-X-A03** | **AdaptToOrchestrator** | **A-BE** | **llmgateway.Request** | **<-chan Chunk (via ILLMGateway)** | **`internal/bridges/llm/bridge.go`（v2.0-b 新增，DM-020）** |

> **F 编排**（详见 `internal/bridges/llm/` 内部）：
> ```
> D3-X-A01 AdaptToContextEngine
>   └─ F01 BridgeChatStream
> D3-X-A02 WireLLMStack
>   └─ F01 WireContextLLM
> ```

> **ID 前缀 `D3-X-`**：X = Cross，避免与 D3 内部 6 个 S 混淆；不计入 5+1 S 计数。
> **Legacy 兼容**：旧 ID `D3-S2-A01-F04` (AdaptToContextEngine) / `D3-S2-A01-F05` (WireLLMStack) 在 `t-registry.md §Legacy Archive` 100% alias 追溯到 `D3-X-A01-F01` / `D3-X-A02-F01`。

---

## 4. A 数与 S 数

| 域内 S 数 | 域内 A 数 | 域内 F 数 | CROSS A 数 | CROSS F 数 | 总 A | 总 F |
|----------|----------|----------|-----------|-----------|------|------|
| 6 | 6 | 24 | 2 | 2 | 8 | 26 |

> F 总数变化：旧 22 → 新 24（域内 22 不变；CROSS 段新增 2）。T 总数：旧 26 → 新 26（26 条 T 在 `t-registry.md` 中按新 S/A 重新编号；Legacy alias 26 条 100% 覆盖）。

---

## 5. 关联文档

| 文档 | 路径 | 关系 |
|------|------|------|
| F 层注册表 | `f-registry.md` | A 编排的具体 F 单元 |
| T 层注册表 | `t-registry.md` | A/F 的测试承诺装置 + Legacy Archive |
| Span 注册表 | `span-registry.md` | A/F 的可观测性 emit（5 ops + 1 adapter op） |
| 规范 | `spec.md` | 5+1 S 规格 + North Star 5 承诺 |
| 设计 | `design.md` | A + F 编排 + 物理映射 |
| 跨域边界 | `openspec/specs/architecture/cross-domain-boundaries.md` | D3-S5 GuardContent vs D2-S18 PermissionMode 灰区 |
| 命名规约 | `openspec/specs/architecture/code-layout.md §4` | D3 scenario-slug 注册表 |
| 分层 | `openspec/specs/architecture/layering.md §D3` | D3 在 6 域架构中的位置 |

---

## 6. 关联评审记录

| 评审 | 文档 | 关联命题 | 状态 |
|------|------|----------|------|
| R1 | `review-r1.md` | D1（5+1 S 切法）/ D2（Bridge 跨域归位）/ D3（Safety 留 D3） | ✅ APPROVED |
| R2 | `review-r2.md` | 命题 A（T ID `<!-- Mechanism: -->` 注释）/ OQ-4（F02 拆分）/ 命题 E（灰区声明） | ✅ R2 FINALIZED |
| R3 | `review-r3.md` | 命题 A（Scope 字段扩展 v1.1 P1）/ 命题 C（IAdapter.Protocol() v1.0 release 后 P1） | ✅ R3 SELF_ADJUDICATED |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.1.0 | 2026-06-14 | 7 S 技术角色词版本（D3-S1~S7） |
| 3.0.0 | 2026-06-14 | 5+1 S 价值流化：RouteModel/StreamChat/ProtectCall/BudgetTokens/GuardContent/ConfigureGateway；F02 拆 F02a/F02b；Bridge / Bootstrap 移至 CROSS 段；T ID 末尾加 `<!-- Mechanism: -->` 注释（R2 命题 A 衍生） |
| 3.1.0 | 2026-06-29 | DM-20260629-003 PR-5 (#3 value-flow-rename) §S 切法总览加 ValueFlow Alias 列（5 S + 1 横切 = 6 alias：`D3_Model_Routing` / `D3_Stream_Chat_Completion` / `D3_Circuit_Breaker_And_Retry` / `D3_Token_Budget_Control` / `D3_Content_Safety_Filter` / `D3_Gateway_Configuration`） |
