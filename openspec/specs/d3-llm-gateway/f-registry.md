# D3 LLM Gateway Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.1.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `a-registry.md` · `t-registry.md`
**Change:** devrix-d3-sa-refine（R1+R2+R3）+ devrix-d3-sa-refine-v1.1（D1-D7 R1 决议固化；6 F 新增/调整）

---

## 0. 变更摘要（v3.0.0 → v3.1.0）

| 维度 | v3.0.0 | v3.1.0 |
|------|--------|--------|
| F 总数（域内） | 24 | **29**（v1.1 新增 5 F 域内） |
| F 编排粒度 | 6 S × 1 A | 6 S × 1 A（5+1，新增 F 挂现有 A） |
| D3-S3 新增 F | F01~F06 | F01~F06 + **F07 EmitBreakerStateMetric** + **F08 OnStateTransitionEmit** + **F09 ReuseEngineEvent**（D1-A / D6-A） |
| D3-S2 新增 F | F01~F03 | F01~F03 + **F04 AdapterProtocolMethod**（D3-A，BREAKING 接口扩展） |
| D3-S5 新增 F | F01~F03 | F01~F03 + **F04 EmitSafetyLatencyEvent**（D5-A） |
| D3-S6 新增 F | F01~F04 | F01~F04 + **F05 FeatureFlagDefaults**（D4-B） |
| D3-X 新增 F | F01（F04 Bridge / F01 Wire） | F01 + **F02 FailFastOnObsNil**（R3 P0 #8 实施） |
| 运行时字面量 | 5 span + 3 metric | 5 span + 3 metric（保持）+ 1 新 metric `llm_breaker_state` + 1 新 span event `safety.check.duration_ms` + 3 新 event `flow.breaker.*` |
| 跨域契约 | 1（v1.0 D3→D5 metric 命名） | 3（+D3→D6 探针接入 / +D3→D7 EngineEvent 复用） |

> **v1.1 F 命名规则**：新 F 沿用 `D3-S{N}-A{XX}-F{NN}` 编号；D3-S3-A01 F07-F09 紧接 F06 编号（D3-S3 核心 F 编排保持），D3-S2-A01 F04 / D3-S5-A01 F04 / D3-S6-A01 F05 / D3-X-A02 F02 紧随 F03 / F03 / F04 / F01。

> **F 编排哲学**：F 是可被 A 编排的最小业务/技术逻辑单元（playbook 原则 6）。F 不跨 A；F 跨 S 时（如 D3-S3-A01 F05 StreamWithFallback 内部调 D3-S2-A01 F01 OpenAIStreamClientStream）通过 A 间协作完成，不出现"跨 S 边界 F"。

---

## 1. S → A → F 索引

```
D3-S1 RouteModel
  └─ D3-S1-A01 ResolveModelRoute
       ├─ F01 MatchRouting
       ├─ F02a ResolveTierAlias       <!-- Tier -->
       └─ F02b ResolveDefault         <!-- Default -->

D3-S2 StreamChat
  └─ D3-S2-A01 StreamChatCompletion
       ├─ F01 OpenAIStreamClientStream
       ├─ F02 ParseSSE
       ├─ F03 BuildOpenAIRequest
       └─ F04 AdapterProtocolMethod       <!-- v1.1 F5: BREAKING 接口扩展 -->

D3-S3 ProtectCall
  └─ D3-S3-A01 ShieldAndRetry
       ├─ F01 AllowCircuit                <!-- Breaker -->
       ├─ F02 RecordOutcome               <!-- Breaker -->
       ├─ F03 ManageCircuitState          <!-- Breaker -->
       ├─ F04 ComputeBackoff              <!-- Retry -->
       ├─ F05 StreamWithFallback          <!-- Retry -->
       ├─ F06 ShouldRecordBreakerFailure  <!-- Cross -->
       ├─ F07 EmitBreakerStateMetric      <!-- v1.1 F1: D3→D5 -->
       ├─ F08 OnStateTransitionEmit       <!-- v1.1 F2: Breaker 钩子 -->
       └─ F09 ReuseEngineEvent            <!-- v1.1 F3: D3→D7 复用 -->

D3-S4 BudgetTokens
  └─ D3-S4-A01 CountAndCheckLLMTokens
       ├─ F01 CountText
       ├─ F02 CountMessages
       ├─ F03 CheckBudget
       ├─ F04 TruncateToTokens
       └─ F05 LoadBPE

D3-S5 GuardContent
  └─ D3-S5-A01 FilterAndMatchContent
       ├─ F01 CheckContent
       ├─ F02 LoadPatterns
       ├─ F03 MatchPattern
       └─ F04 EmitSafetyLatencyEvent      <!-- v1.1 F8: D3→D6 probe #4 -->

D3-S6 ConfigureGateway
  └─ D3-S6-A01 LoadAndValidateLLMConfig
       ├─ F01 LoadConfig
       ├─ F02 BuildConfig
       ├─ F03 ValidateProviders
       ├─ F04 LoadAPIKey
       └─ F05 FeatureFlagDefaults         <!-- v1.1 F9: 3 flag schema + 默认值 -->

CROSS (D3 → D2 Bridge)
  ├─ D3-X-A01 AdaptToContextEngine
  │    └─ F01 BridgeChatStream
  └─ D3-X-A02 WireLLMStack
       ├─ F01 WireContextLLM
       └─ F02 FailFastOnObsNil            <!-- v1.1 F4: R3 P0 #8 实施 -->
```

---

## 2. F 编号规约

| 规则 | 说明 |
|------|------|
| F ID 格式 | `D3-S{N}-A{XX}-F{NN}`（域内 6 S）或 `D3-X-A{XX}-F{NN}`（CROSS 段，X = Cross） |
| F 唯一性 | 跨 A 不重号；同 A 内顺序编号（执行序为先） |
| 拆分 F | 单 F 拆分为多 F 时使用 `F{NN}a` / `F{NN}b` 后缀（仅 D3-S1-A01-F02 拆分；其他 F 不主动拆） |
| 跨 A 协作 | F05 StreamWithFallback 内部可调 D3-S2-A01 F01（不出现"跨 S F"） |

---

## 3. F 详细注册

### 3.1 D3-S1-A01 ResolveModelRoute

| F ID | Name | Type | Input | Output | Mechanism | Code Location | Legacy Alias |
|------|------|------|-------|--------|-----------|---------------|--------------|
| D3-S1-A01-F01 | MatchRouting | F-BE | model_name (concrete) | provider | — | `gateway/router.go` (`Router.matchRouting`) | 旧 D3-S2-A01-F01 ResolveModel |
| **D3-S1-A01-F02a** | **ResolveTierAlias** | F-BE | tier_alias (e.g. "fast", "pro") | concrete_model | Tier | `gateway/router.go` (`Router.resolveTier`) | 旧 D3-S2-A01-F03 ResolveTier（部分） |
| **D3-S1-A01-F02b** | **ResolveDefault** | F-BE | empty / unknown | default_model | Default | `gateway/router.go` (`Router.resolveDefault` — v1.0 实施) | — |

**F02 拆分说明**（R2 命题 D / OQ-4）：

- **F02a ResolveTierAlias**：将 tier alias 解析为具体 model；`ErrUnknownTier` 在此抛出
- **F02b ResolveDefault**：将 empty model 或 unknown model 默认回填；`ErrNoRoute` / `ErrUnsupportedModel` 在此抛出
- 运行时顺序：`Router.Resolve` 先 F02a → 再 F02b → 再 F01
- 旧 `D3-S2-A01-F03 ResolveTier` 拆为 F02a + F02b（V2.1 T「Unknown tier passes through」挂 F02a；新 T「Empty model defaults to DefaultProvider」挂 F02b）

**错误码签名**：

| 错误 | 抛出 F | 含义 |
|------|--------|------|
| `ErrUnknownTier` | F02a | tier alias 不在配置表中 |
| `ErrNoRoute` | F02b | 空 model + 无 default |
| `ErrUnsupportedModel` | F02b | model 名非空但非任何 provider 支持 |

---

### 3.2 D3-S2-A01 StreamChatCompletion

| F ID | Name | Type | Input | Output | Mechanism | Code Location | Legacy Alias |
|------|------|------|-------|--------|-----------|---------------|--------------|
| D3-S2-A01-F01 | OpenAIStreamClientStream | F-BE | ctx, llmgateway.Request, adapter_cfg | <-chan raw.SSE | — | `adapter/openai_stream.go` (`OpenAIStreamClient.Stream`) | 旧 D3-S1-A01-F01 StreamChat |
| D3-S2-A01-F02 | ParseSSE | F-BE | io.Reader (SSE stream) | <-chan *AdapterChunk | — | `adapter/sse_parser.go` (`streamOpenAISSE`, `streamAccumulator`) | 旧 D3-S1-A01-F02 ParseSSE |
| D3-S2-A01-F03 | BuildOpenAIRequest | F-BE | llmgateway.Request | openAI JSON body | — | `adapter/openai_request.go` (`buildOpenAIChatRequest`, `mapOpenAIMessage`) | 旧 D3-S1-A01-F03 BuildOpenAIRequest |
| **D3-S2-A01-F04** | **AdapterProtocolMethod** | F-BE | — | `string` (protocol 标识) | IAdapter interface | `adapter/iadapter.go` (`Protocol() string` 新增方法) | — (v1.1 增) |

**F04 AdapterProtocolMethod 实施说明**（R3 P1 #15 + D3-A 决议）：

- `IAdapter` 接口新增 `Protocol() string` 方法；返回值为 `string` 类型（**BREAKING** 变更，所有实现必须同步补）
- 现有 3 个实现（`DeepSeekAdapter` / `MiniMaxAdapter` / `stubAdapter` test fixture）均返回 `"openai-compat"`（v1.1 实施）
- 路由层 v2.0 启用：protocol-aware fallback（plan A → 协议 A → 协议 B fallback）
- 错误码：`ErrAdapterProtocolNotImplemented`（编译期阻断；v1.1 release 前必须修）

**Provider 适配**（DeepSeek / MiniMax）：通过 `IAdapter` 接口复用 F01+F03；Provider-specific 请求体差异由 `D3-S1-A01` 在 routing 阶段决定 provider 选取（详见 `design.md §3.2`）。

**V3 扩展点**（R3 命题 C 决议 #6.3 #1）：v1.0 release 后第一个 issue 增 `IAdapter.Protocol() string` 方法；v1.1 落地为 F04。

---

### 3.3 D3-S3-A01 ShieldAndRetry

> **合并依据**（R1 D1 + R2 命题 A）：承诺 C3「Provider 故障不阻塞我」是同一承诺的两个机制（Breaker + Retry）；F 编排 1:1 反映"承诺装置"，T 编号按 F 编排顺序，每个 T 末尾加 `<!-- Mechanism: -->` 注释保留机制可追溯性。

| F ID | Name | Type | Input | Output | Mechanism | Code Location | Legacy Alias |
|------|------|------|-------|--------|-----------|---------------|--------------|
| D3-S3-A01-F01 | AllowCircuit | F-BE | provider | allowed/blocked | Breaker | `breaker/circuit_breaker.go` (`Allow`) | 旧 D3-S3-A01-F01 BeforeCall |
| D3-S3-A01-F02 | RecordOutcome | F-BE | provider, success/failure | — | Breaker | `breaker/circuit_breaker.go` (`RecordSuccess`, `RecordFailure`) | 旧 D3-S3-A01-F02 AfterCall |
| D3-S3-A01-F03 | ManageCircuitState | F-BE | provider, events | — | Breaker | `breaker/circuit_breaker.go` (`open`, `finalize`) + `state.go` (`circuitRecord`) | 旧 D3-S3-A01-F03 ManageState |
| D3-S3-A01-F04 | ComputeBackoff | F-BE | cfg, attempt | delay (Full Jitter) | Retry | `retry/retry.go` (`backoffDelay`) | 旧 D3-S4-A01-F02 ComputeBackoff |
| D3-S3-A01-F05 | StreamWithFallback | F-BE | ctx, call, primary, fallback, cfg | <-chan *AdapterChunk | Retry | `retry/retry.go` (`Executor.Stream`) | 旧 D3-S4-A01-F01 StreamWithFallback |
| D3-S3-A01-F06 | ShouldRecordBreakerFailure | F-BE | err | bool | Cross | `breaker/circuit_breaker.go` (`shouldRecord` — 避免 Cancel/Deadline 触发) | 旧 D3-S2-A01-F02 联动逻辑（部分） |
| **D3-S3-A01-F07** | **EmitBreakerStateMetric** | F-BE | provider, state | — | Metric | `breaker/circuit_breaker.go` (状态切换钩子 emit `llm_breaker_state{provider, state}`) | — (v1.1 F1 增) |
| **D3-S3-A01-F08** | **OnStateTransitionEmit** | F-BE | provider, from, to | — | Breaker hook | `breaker/state.go` (状态变化钩子，触发 F07 + F09) | — (v1.1 F2 增) |
| **D3-S3-A01-F09** | **ReuseEngineEvent** | F-BE | provider, from, to, ts | — | Event | `breaker/events.go` (emit `flow.breaker.opened` / `closed` / `halfopened`) | — (v1.1 F3 增) |

**F 编排顺序**（`ShieldAndRetry.Execute` v1.1）：

```
1. F01 AllowCircuit(provider)
   ├─ blocked → F07/F08 钩子触发（如果开启 emit）→ F09 emit flow.breaker.opened
   └─ allowed ↓
2. F05 StreamWithFallback(ctx, call, primary, fallback, cfg)
   ├─ 成功 → F02 RecordOutcome(provider, success=true) → F03 ManageCircuitState
   │       → F08 状态变化钩子（若 Closed）→ F07/F09 emit（若 closed 回归）
   └─ 失败（且非 Cancel/Deadline）
       ├─ F06 ShouldRecordBreakerFailure(err) == true → F02 RecordOutcome(provider, success=false) → F03 ManageCircuitState
       │       → F08 状态变化钩子（若 Open 触发）→ F07 emit `state=open` + F09 emit `flow.breaker.opened`
       └─ F04 ComputeBackoff(cfg, attempt) → 退避 → 重试
```

**F07 / F08 / F09 触发关系**（v1.1 D1-A + D6-A 决议）：

- F07 EmitBreakerStateMetric：受 `d3_resilience_emit_enabled` feature flag 控制；默认 ON
- F08 OnStateTransitionEmit：F03 ManageCircuitState 状态切换处调用；幂等保护（首次切换后无变化不重复）
- F09 ReuseEngineEvent：复用 D7 现有 `EngineEvent`（`FlowStarted` / `FlowFailed` 字段），3 事件分开 emit（D6-A 决议）

**Mechanism 可追溯性**（R2 命题 A 衍生）：`t-registry.md` 中每个 ProtectCall T 末尾加 `<!-- Mechanism: Breaker / Retry / Cross -->` 注释；v1.1 新 F 标注 `<!-- v1.1 F1/F2/F3 -->`。

---

### 3.4 D3-S4-A01 CountAndCheckLLMTokens

| F ID | Name | Type | Input | Output | Mechanism | Code Location | Legacy Alias |
|------|------|------|-------|--------|-----------|---------------|--------------|
| D3-S4-A01-F01 | CountText | F-BE | text | token_count | — | `token/counter.go` (`Counter.CountText`) | 旧 D3-S5-A01-F01 CountText |
| D3-S4-A01-F02 | CountMessages | F-BE | []Message, systemPrompt | total_tokens | — | `token/counter.go` (`Counter.CountMessages`, `CountWithSystemPrompt`) | 旧 D3-S5-A01-F02 CountMessages |
| D3-S4-A01-F03 | CheckBudget | F-BE | count, budget | error/nil | — | `token/counter.go` (`Counter.CheckBudget`) | 旧 D3-S5-A01-F03 CheckBudget |
| D3-S4-A01-F04 | TruncateToTokens | F-BE | text, maxTokens | truncated_text | — | `token/counter.go` (`Counter.TruncateToTokens`) | 旧 D3-S5-A01-F04 TruncateToTokens |
| D3-S4-A01-F05 | LoadBPE | F-BE | — | tiktoken.Encoding | — | `token/bpe_loader.go` (`ensureEmbeddedBPELoader`, `embeddedBpeLoader`) | 旧 D3-S5-A01-F05 LoadBPE |

> **跨域观察**（R3 NQ-6 / P2 #20）：D2-S4 Token vs D3-S4 BudgetTokens 合并评估是 v1.1 路线图项；当前 F 边界不重叠（D2-S4 关注 context window 内 token 统计；D3-S4 关注预算 + 截断）。

---

### 3.5 D3-S5-A01 FilterAndMatchContent

| F ID | Name | Type | Input | Output | Mechanism | Code Location | Legacy Alias |
|------|------|------|-------|--------|-----------|---------------|--------------|
| D3-S5-A01-F01 | CheckContent | F-BE | ctx, system_prompt, []messages | *safety.Result | — | `safety/filter.go` (`Filter.Check`) | 旧 D3-S7-A01-F01 CheckContent |
| D3-S5-A01-F02 | LoadPatterns | F-BE | — | []Pattern | — | `safety/patterns.go` (`defaultPatterns`) | 旧 D3-S7-A01-F02 LoadPatterns |
| D3-S5-A01-F03 | MatchPattern | F-BE | text, pattern | bool | — | `safety/filter.go` (`strings.Contains`, case-insensitive) | 旧 D3-S7-A01-F03 MatchPattern |
| **D3-S5-A01-F04** | **EmitSafetyLatencyEvent** | F-BE | duration_ms | — | Span event | `safety/filter.go` (F01 调用后计时写入 span event `safety.check.duration_ms`) | — (v1.1 F8 增) |

**F04 EmitSafetyLatencyEvent 实施说明**（R3 P1 #16 + D5-A 决议）：

- `Filter.Check` 内部计时（`time.Now()` / `time.Since()`），调用结束后写入当前 trace span event `safety.check.duration_ms`
- 受 `d3_safety_latency_event_enabled` feature flag 控制；默认 ON
- D6 probe #4：统计 P99；**阈值 1ms**（D5-A 决议，与 v1.0 design.md §6.4 #1 一致）
- 错误码：`ErrSafetyLatencyThreshold`（D6 触发，非 D3 抛）

**V2.1 已有 T 全部 IMPLEMENTED**：
- D3-S7-A01-T01 critical reject（malware/exploit）
- D3-S7-A01-T02 warning match（injection/credential）

**跨域灰区**（R2 命题 E / P0 #5）：详见 `cross-domain-boundaries.md` §D3-S5 —— D3 优先拒（前置过滤），D2-S18 PermissionMode 兜底。

**v1.0 性能观察**（R3 命题 D 决议 #6.4 #1）：Filter.Check 内部计时写入 span event `safety.check.duration_ms`；P99 告警阈值 1ms（v1.0 release 后第一个 issue）。v1.1 落地为 F04。

---

### 3.6 D3-S6-A01 LoadAndValidateLLMConfig

| F ID | Name | Type | Input | Output | Mechanism | Code Location | Legacy Alias |
|------|------|------|-------|--------|-----------|---------------|--------------|
| D3-S6-A01-F01 | LoadConfig | F-BE | config_file | LLMGatewayConfig | — | `config/loader.go` (`Loader.Load`, `LoadFromFileConfig`) | 旧 D3-S6-A01-F01 LoadConfig |
| D3-S6-A01-F02 | BuildConfig | F-BE | LLMGatewayFileConfig | LLMGatewayConfig | — | `shared/config/llmgateway.go` (`BuildLLMGatewayConfig`, `DefaultLLMGatewayConfig`) | 旧 D3-S6-A01-F02 BuildConfig |
| D3-S6-A01-F03 | ValidateProviders | F-BE | LLMGatewayConfig | error/nil | — | `config/loader.go` (`validate`) | 旧 D3-S6-A01-F03 ValidateProviders |
| D3-S6-A01-F04 | LoadAPIKey | F-BE | LLMProviderRuntimeConfig | api_key, ok | — | `config/loader.go` (`APIKey`) | 旧 D3-S6-A01-F04 LoadAPIKey |
| **D3-S6-A01-F05** | **FeatureFlagDefaults** | F-BE | — | feature flag schema | — | `shared/config/llmgateway.go` + `wire.go` (3 flag schema + 默认值 + 读取) | — (v1.1 F9 增) |

**F05 FeatureFlagDefaults 实施说明**（D4-B 决议）：

| Flag | 类型 | 默认 | Schema key | 读取位置 |
|------|------|------|------------|----------|
| `d3_resilience_emit_enabled` | bool | **ON** | `llm_gateway.d3_resilience_emit_enabled` | F07 EmitBreakerStateMetric |
| `d3_safety_latency_event_enabled` | bool | **ON** | `llm_gateway.d3_safety_latency_event_enabled` | F04 EmitSafetyLatencyEvent |
| `d3_metric_emit_warn` | bool | **OFF** | `llm_gateway.d3_metric_emit_warn` | (emit 失败 log warn 控制) |

**OFF 行为继承**（D4-B 决议）：F05 默认值与 v1.0 行为保持（OFF 时 v1.0 行为完全保持；无回归）。

**v1.0 扩展点**（R3 命题 B 决议 #6.2 #1）：`factory.go:NewFromConfig` 增加 obs nil 检查（**v1.0 P0 必做**），返回 `ErrObservabilityRequired`；不在本 F 中处理，但在 S6 边界上；v1.1 实施在 D3-X-A02-F02。

---

## 4. CROSS — 跨域锚点（Bridge / Bootstrap）

> **R1 D2 决议**：Bridge / Bootstrap 归属跨域锚点 `internal/bridges/llm/`，不计入 D3 域内 F 数。

### 4.1 D3-X-A01 AdaptToContextEngine

| F ID | Name | Type | Input | Output | Code Location | Legacy Alias |
|------|------|------|-------|--------|---------------|--------------|
| D3-X-A01-F01 | BridgeChatStream | F-BE | llmgateway.Request | <-chan Chunk (via ILLMGateway) | `internal/bridges/llm/bridge.go` (`Bridge.ChatStream`) | 旧 D3-S2-A01-F04 AdaptToContextEngine |

### 4.2 D3-X-A02 WireLLMStack

| F ID | Name | Type | Input | Output | Code Location | Legacy Alias |
|------|------|------|-------|--------|---------------|--------------|
| D3-X-A02-F01 | WireContextLLM | F-BE | config_file, obs | ContextLLMStack | `internal/bridges/llm/context_wiring.go` (`WireContextLLM`) + `internal/bridges/llm/wire.go` (`WireFromConfig`) | 旧 D3-S2-A01-F05 WireLLMStack |
| **D3-X-A02-F02** | **FailFastOnObsNil** | F-BE | obs | error/nil | `internal/bridges/llm/wire.go` (`WireFromConfig` 内 obs nil 检查 + `context_wiring.go` 配合) | — (v1.1 F4 增) |

**F02 FailFastOnObsNil 实施说明**（R3 P0 #8 实施 + D7-A 决议）：

- `WireFromConfig(ctx, config_file, obs)` 入参 obs == nil 时**立即**返回 `ErrObservabilityRequired`（不 silent fallback）
- 测试 fixture 显式注入 mock obs；新增 `WithMockObs()` helper 避免 fixture 误传 nil
- 错误码 `ErrObservabilityRequired` 已在 `internal/shared/errors/` 注册（v1.0 R3 P0 #8 占位）
- 启动 trace 单独 emit `config.load.duration_ms` + 失败原因 `ErrObservabilityRequired`

---

## 5. F 总数统计

| S 段 | A 数 | F 数（v3.0.0） | F 数（v3.1.0） | Δ | 说明 |
|------|------|----------------|----------------|---|------|
| D3-S1 RouteModel | 1 | 3 | 3 | 0 | 不变（v1.1 不动 routing） |
| D3-S2 StreamChat | 1 | 3 | 4 | **+1** | v1.1 增 F04 AdapterProtocolMethod（BREAKING） |
| D3-S3 ProtectCall | 1 | 6 | 9 | **+3** | v1.1 增 F07/F08/F09（F1/F2/F3 emit） |
| D3-S4 BudgetTokens | 1 | 5 | 5 | 0 | 不变（v1.1 不动 budget） |
| D3-S5 GuardContent | 1 | 3 | 4 | **+1** | v1.1 增 F04 EmitSafetyLatencyEvent |
| D3-S6 ConfigureGateway | 1 | 4 | 5 | **+1** | v1.1 增 F05 FeatureFlagDefaults |
| **域内合计** | **6** | **24** | **30** | **+6** | v1.1 净增 6 F 域内 |
| CROSS (D3-X) | 2 | 2 | 3 | **+1** | v1.1 增 D3-X-A02-F02 FailFastOnObsNil |
| **总计** | **8** | **26** | **33** | **+7** | v1.1 净增 7 F（6 域内 + 1 CROSS） |

> **v3.0.0 → v3.1.0 F 数计算**：v3.0.0 域内 24 + v1.1 新增 6（v3.1.0 域内 30）+ CROSS v3.0.0 2 + v1.1 新增 1（v3.1.0 CROSS 3）= **v3.1.0 总计 33 F**。
>
> v1.1 新增 F 全部沿用 `D3-S{N}-A{XX}-F{NN}` 编号；不跨 A；F 跨 S 时通过 A 间协作完成（与 v3.0.0 哲学一致）。

---

## 6. 关联文档

| 文档 | 路径 | 关系 |
|------|------|------|
| A 层注册表 | `a-registry.md` | F 的编排容器 |
| T 层注册表 | `t-registry.md` | F 的测试承诺装置 |
| 设计 | `design.md §3.2` | A + F 编排与时序图 |
| 跨域边界 | `openspec/specs/architecture/cross-domain-boundaries.md` | D3-S5 / D2-S18 灰区 |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.1.0 | 2026-06-14 | 7 S × 1 A × 22 F |
| 3.0.0 | 2026-06-14 | 5+1 S × 1 A × 24 F（域内）+ CROSS 2 A × 2 F；F02 拆 F02a/F02b（+1）；ProtectCall 合并 Breaker+Retry 引入 F06 ShouldRecordBreakerFailure（+1）；Bridge / Bootstrap 移至 CROSS 段 |
| 3.1.0 | 2026-06-14 | 5+1 S × 1 A × 30 F（域内）+ CROSS 3 F（净增 6 域内 + 1 CROSS = 7 F）；D3-S3 增 F07/F08/F09（emit metric / state hook / engine event）；D3-S2 增 F04 AdapterProtocolMethod（BREAKING 接口扩展）；D3-S5 增 F04 EmitSafetyLatencyEvent；D3-S6 增 F05 FeatureFlagDefaults；D3-X-A02 增 F02 FailFastOnObsNil（R3 P0 #8 实施） |
