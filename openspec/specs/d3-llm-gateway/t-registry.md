# D3 LLM Gateway Domain — T 层测试点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.1.1
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d3-sa-refine（R1+R2+R3）+ devrix-d3-sa-refine-v1.1（D1-D7 R1 决议；9 新 T 增；S5 验收后 v1.1 PLANNED→IMPLEMENTED）
**Companion Docs:** `a-registry.md` · `f-registry.md` · `span-registry.md` · `spec.md` · `design.md`

---

## 0. 变更摘要（v3.0.0 → v3.1.0 → v3.1.1）

| 维度 | v3.0.0 | v3.1.0（spec 设计） | **v3.1.1（S5 验收后）** |
|------|--------|--------|--------|
| T 总数 | 26 | 35 | **35**（不变） |
| T 编号规则 | 新 S/A 顺序 + `<!-- Mechanism: -->` | 继承 v3.0.0 + v1.1 新 T 标注 `<!-- v1.1 Fx -->` | **继承** v3.1.0 |
| Legacy 兼容 | §Legacy Archive 26 条 100% 覆盖 | 继承 100%（v1.1 不增不删旧 T） | **继承** 100% |
| IMPLEMENTED T | 25 | 26 | **34**（v1.1 9 T PLANNED→IMPLEMENTED；与 §1 S 段合计 34 一致） |
| PLANNED T | 1 | 9 | **1**（仅 T08 持久化仍 PLANNED） |
| v1.1 新 T 编号 | — | D3-S1-A01-T03 / D3-S2-A01-T06 / D3-S3-A01-T13~T15 / D3-S5-A01-T03 / D3-S6-A01-T02 / D3-X-A02-T01 | **继承** |

> **v1.1 T 编号连续性**：v1.1 新增 T 紧接 v3.0.0 末尾 T 编号；不重启 S 编号池；不破坏 `§Legacy Archive` 100% 覆盖。

---

## 1. T 总览

| S 段 | A | T 总数（v3.0.0） | T 总数（v3.1.0） | P0（v3.1.0） | P1（v3.1.0） | P2 (PLANNED) | IMPLEMENTED |
|------|---|------------------|------------------|--------------|--------------|--------------|-------------|
| D3-S1 RouteModel | 1 | 2 | **3**（+1） | 1（+1） | 2 | 0 | 3 |
| D3-S2 StreamChat | 1 | 5 | **6**（+1） | 3（+1） | 3 | 0 | 6 |
| D3-S3 ProtectCall | 1 | 12 | **15**（+3） | 8（+2） | 6（+1） | 1 | 14 |
| D3-S4 BudgetTokens | 1 | 3 | 3 | 2 | 1 | 0 | 3 |
| D3-S5 GuardContent | 1 | 2 | **3**（+1） | 2（+1） | 1 | 0 | 3 |
| D3-S6 ConfigureGateway | 1 | 1 | **2**（+1） | 2（+1） | 0 | 0 | 2 |
| D3-X CROSS (Bridge + Bootstrap) | 2 | 1 | **2**（+1） | 1（+1） | 1 | 0 | 2 |
| **合计** | **8** | **26** | **35**（+9） | **19** | **14** | **1** | **34** |

> **v1.1 净增 9 T**：6 P0（F1/F2/F4/F5/F8/F9）+ 3 P1（F3/F6/F7）。v1.0 26 T 全量保留 + IMPLEMENTED 状态不变。

---

## 2. D3-S1 RouteModel — 测试点

**A**: `D3-S1-A01 ResolveModelRoute`
**承诺 C1**：用户给出 model 名（含 tier alias），D3 必须返回正确 provider + 实际 model。

| T ID | 描述 | 优先级 | F 编排 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S1-A01-T01 | 多 Provider 并发调用（MatchRouting 路径） | P1 | F01 MatchRouting | `internal/layers/llmgateway/route/router_test.go` | IMPLEMENTED |
| D3-S1-A01-T02 | 未知 Provider/Model 报错（ResolveDefault F02a+F02b 联动） | P1 | F02a ResolveTierAlias + F02b ResolveDefault | `internal/layers/llmgateway/route/router_test.go` | IMPLEMENTED |
| **D3-S1-A01-T03** | **Tier 解析正确性 ≥ 99%（D6 probe #1）** `<!-- v1.1 F6 -->` | **P1** | F01 + F02a（routing 暴露调用次数 + 错误率） | `internal/layers/llmgateway/route/router_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，D6-S3-A01-T20 配套） |

**F 覆盖**：

```
D3-S1-A01 ResolveModelRoute
  ├─ F01 MatchRouting         → T01 (P1) 验证 routing 矩阵
  ├─ F02a ResolveTierAlias    → T02 (P1) 验证 tier alias 透传
  └─ F02b ResolveDefault      → T02 (P1) 验证 default 回填
  + T03 (v1.1) → D6 probe #1 配合：routing 正确性 ≥ 99%
```

> **R2 命题 D 衍生**：T02 同时覆盖 F02a + F02b（V2.1 旧 D3-S2-A01-T02 已经是合并测试，F02 拆 F02a/F02b 后测试点未拆），error 签名由 `ErrUnknownTier` / `ErrNoRoute` / `ErrUnsupportedModel` 区分。

---

## 3. D3-S2 StreamChat — 测试点

**A**: `D3-S2-A01 StreamChatCompletion`
**承诺 C2**：用户发起流式聊天，D3 必须返回符合 OpenAI SSE 协议的 chunk 流。

| T ID | 描述 | 优先级 | F 编排 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S2-A01-T01 | DeepSeek 适配器流式响应 | P0 | F01 OpenAIStreamClientStream | `internal/layers/llmgateway/stream/adapter/deepseek_test.go` | IMPLEMENTED |
| D3-S2-A01-T02 | MiniMax 适配器流式响应 | P0 | F01 OpenAIStreamClientStream | `internal/layers/llmgateway/stream/adapter/minimax_test.go` | IMPLEMENTED |
| D3-S2-A01-T03 | SSE parse error handling | P1 | F02 ParseSSE | `internal/layers/llmgateway/stream/adapter/sse_parser_test.go` | IMPLEMENTED |
| D3-S2-A01-T04 | OpenAI request body construction | P1 | F03 BuildOpenAIRequest | `internal/layers/llmgateway/stream/adapter/openai_request_test.go` | IMPLEMENTED |
| D3-S2-A01-T05 | LLM 调用可观测事件 (spans + metrics) | P1 | F01 + F02（跨 A 验证 emit） | `tests/integration/llm_observer_test.go` | IMPLEMENTED |
| **D3-S2-A01-T06** | **`IAdapter.Protocol() string` 接口扩展 + 3 实现** `<!-- v1.1 F5 -->` | **P0** | F04 AdapterProtocolMethod | `internal/layers/llmgateway/stream/adapter/iadapter_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，BREAKING 接口，DeepSeek + MiniMax + Mock 全部实施） |

**F 覆盖**：

```
D3-S2-A01 StreamChatCompletion
  ├─ F01 OpenAIStreamClientStream  → T01 (P0) + T02 (P0) DeepSeek/MiniMax 流式
  ├─ F02 ParseSSE                  → T03 (P1) SSE parse error
  ├─ F03 BuildOpenAIRequest        → T04 (P1) OpenAI request body
  ├─ F04 AdapterProtocolMethod     → T06 (v1.1 P0) Protocol() 返回非空字符串
  └─ (跨 A 验证)                   → T05 (P1) spans + metrics emit
```

---

## 4. D3-S3 ProtectCall — 测试点

**A**: `D3-S3-A01 ShieldAndRetry`
**承诺 C3**：Provider 故障（5xx / 网络错误 / 限流），D3 必须不阻塞用户。

| T ID | 描述 | 优先级 | F 编排 | Mechanism | Test 位置 | Status |
|-------|------|--------|--------|-----------|-----------|--------|
| D3-S3-A01-T01 | Circuit breaker 正常关闭 (Closed) | P0 | F01 AllowCircuit + F03 ManageCircuitState | Breaker | `internal/layers/llmgateway/protect/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T02 | Circuit breaker 触发开启 (Open) | P0 | F01 + F02 RecordOutcome + F03 | Breaker | `internal/layers/llmgateway/protect/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T03 | Circuit breaker 半开→关闭 (HalfOpen→Closed) | P0 | F01 + F02 + F03 | Breaker | `internal/layers/llmgateway/protect/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T04 | Circuit breaker 半开→开启 (HalfOpen→Open) | P0 | F01 + F02 + F03 | Breaker | `internal/layers/llmgateway/protect/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T05 | Retry 与 CB 联动（Cancel/Deadline 不触发 CB） | P0 | F01 + F05 StreamWithFallback + F06 ShouldRecordBreakerFailure | Cross | `internal/layers/llmgateway/stream/gateway_test.go` | IMPLEMENTED |
| D3-S3-A01-T06 | Half-Open 并发探测限制 | P0 | F01 + F03 ManageCircuitState | Breaker | `internal/layers/llmgateway/stream/gateway_test.go` | IMPLEMENTED |
| D3-S3-A01-T07 | LLM 429 rate limit handling | P1 | F05 StreamWithFallback | Retry | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |
| D3-S3-A01-T08 | 熔断器状态持久化（重启后 Closed 状态恢复） | P2 | F03 + 持久化层（v1.1 候选） | Breaker | — | PLANNED |
| D3-S3-A01-T09 | 重试策略执行（Full Jitter 退避） | P0 | F04 ComputeBackoff + F05 | Retry | `internal/layers/llmgateway/protect/retry_test.go` | IMPLEMENTED |
| D3-S3-A01-T10 | Full Jitter 随机化验证 | P1 | F04 ComputeBackoff | Retry | `internal/layers/llmgateway/protect/retry_jitter_test.go` | IMPLEMENTED |
| D3-S3-A01-T11 | DeepSeek Fallback 模型切换 | P1 | F05 StreamWithFallback | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| D3-S3-A01-T12 | MiniMax Fallback 模型切换 | P1 | F05 StreamWithFallback | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| **D3-S3-A01-T13** | **Breaker 状态切换 emit `llm_breaker_state{provider,state}`** `<!-- v1.1 F1 + F2 -->` | **P0** | F07 EmitBreakerStateMetric + F08 OnStateTransitionEmit | Metric | `internal/layers/llmgateway/protect/circuit_breaker_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，2 provider × 3 state = 6 series） |
| **D3-S3-A01-T14** | **Breaker 状态切换 emit `flow.breaker.opened` / `closed` / `halfopened` EngineEvent** `<!-- v1.1 F3 -->` | **P1** | F09 ReuseEngineEvent | Event | `internal/layers/llmgateway/protect/events_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，3 事件分开，fakePublisher 验证） |
| **D3-S3-A01-T15** | **D6 probe #2 Breaker 异常切换告警** `<!-- v1.1 F7 -->` | **P1** | F07 + F08（D6 探针统计 `llm_breaker_transitions_total`） | Probe | `tests/integration/d6_breaker_probe_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，D6-S3-A01-T21 配套） |

**F 覆盖**：

```
D3-S3-A01 ShieldAndRetry
  ├─ F01 AllowCircuit                → T01-T04, T06  (Breaker 状态机)
  ├─ F02 RecordOutcome               → T01-T04, T05  (成功/失败记录)
  ├─ F03 ManageCircuitState          → T01-T04, T06, T08  (状态机切换)
  ├─ F04 ComputeBackoff              → T09, T10  (退避计算)
  ├─ F05 StreamWithFallback          → T05, T07, T09, T11, T12  (重试 + fallback)
  ├─ F06 ShouldRecordBreakerFailure  → T05  (Cancel/Deadline 不触发 CB)
  ├─ F07 EmitBreakerStateMetric      → T13 (v1.1 P0) 状态切换 emit llm_breaker_state
  ├─ F08 OnStateTransitionEmit       → T13 (v1.1 P0) 状态机钩子
  ├─ F09 ReuseEngineEvent            → T14 (v1.1 P1) emit flow.breaker.*
  └─ (D6 probe #2)                   → T15 (v1.1 P1) 异常切换告警
```

> **R2 命题 A 落地**：T01~T12 末尾 `<!-- Mechanism: -->` 注释保留 Breaker / Retry / Cross 机制可追溯性。v1.1 新 T13~T15 标注 `<!-- v1.1 F1~F3 + F7 -->`。

---

## 5. D3-S4 BudgetTokens — 测试点

**A**: `D3-S4-A01 CountAndCheckLLMTokens`
**承诺 C4**：Token 超预算，D3 必须截断或报错，不超额调用。

| T ID | 描述 | 优先级 | F 编排 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S4-A01-T01 | Token 计数准确性 (cl100k_base) | P0 | F01 CountText | `internal/layers/llmgateway/budget/counter_test.go` | IMPLEMENTED |
| D3-S4-A01-T02 | Token 预算检查 (CheckBudget) | P0 | F03 CheckBudget | `internal/layers/llmgateway/budget/counter_test.go` | IMPLEMENTED |
| D3-S4-A01-T03 | Token counter 中文准确性 (CJK) | P1 | F01 CountText | `internal/layers/llmgateway/budget/counter_test.go` | IMPLEMENTED |

---

## 6. D3-S5 GuardContent — 测试点

**A**: `D3-S5-A01 FilterAndMatchContent`
**承诺 C5**：用户 prompt 命中危险模式，D3 必须拒绝（critical）或告警（warning）。

| T ID | 描述 | 优先级 | F 编排 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S5-A01-T01 | Safety filter critical 拒绝 (malware/exploit) | P0 | F01 CheckContent | `internal/layers/llmgateway/guard/filter_test.go` | IMPLEMENTED |
| D3-S5-A01-T02 | Safety filter warning 匹配 (injection/credential) | P1 | F01 CheckContent | `internal/layers/llmgateway/guard/filter_test.go` | IMPLEMENTED |
| **D3-S5-A01-T03** | **Safety filter span event `safety.check.duration_ms` + D6 probe #4 P99 < 1ms** `<!-- v1.1 F8 -->` | **P0** | F04 EmitSafetyLatencyEvent | `internal/layers/llmgateway/guard/filter_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，LatencySink 接口 + WithLatencySink，D6-S3-A01-T22 配套） |

> **跨域灰区声明**（R2 命题 E / P0 #5）：D3-S5 与 D2-S18 PermissionMode 灰区 — 当 prompt 内容与 tool execution 存在交叉时，**D3 优先拒**（前置过滤），D2 兜底；详见 `openspec/specs/architecture/cross-domain-boundaries.md` §D3-S5。

---

## 7. D3-S6 ConfigureGateway — 测试点

**A**: `D3-S6-A01 LoadAndValidateLLMConfig`
**承诺**：无（横切支撑）。

| T ID | 描述 | 优先级 | F 编排 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S6-A01-T01 | Provider 配置加载与验证 | P0 | F01 LoadConfig + F03 ValidateProviders | `internal/layers/llmgateway/configure/loader_test.go` | IMPLEMENTED |
| **D3-S6-A01-T02** | **3 feature flag schema + 默认值；OFF 时 v1.0 行为保持** `<!-- v1.1 F9 -->` | **P0** | F05 FeatureFlagDefaults | `internal/layers/llmgateway/configure/loader_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，resilience+latency ON / warn OFF 默认；8 组合单测） |

---

## 8. D3-X CROSS (Bridge + Bootstrap) — 测试点

> **R1 D2 决议**：Bridge / Bootstrap 归属跨域锚点 `internal/bridges/llm/`，T 编号前缀 `D3-X-`（X = Cross）。

| T ID | 描述 | 优先级 | A 编排 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-X-A01-T01 | Bridge 适配 Gateway → ILLMGateway | P1 | D3-X-A01 AdaptToContextEngine | `internal/bridges/llm/bridge_test.go` | IMPLEMENTED |
| **D3-X-A02-T01** | **`WireContextLLM` obs nil fail-fast** `<!-- v1.1 F4 -->` | **P0** | D3-X-A02 WireLLMStack F02 FailFastOnObsNil | `internal/bridges/llm/wire_test.go`（v1.1 增） | **IMPLEMENTED**（v1.1 S5 验收通过，BREAKING 签名 `WireContextLLM(...) (ContextLLMStack, error)`，ErrObservabilityRequired sentinel） |

---

## 9. Legacy Archive（v2.1.0 → v3.0.0 alias 追溯）

> **R1 Q4 决议**：所有旧 T ID 写入 `§Legacy Archive`；`scripts/check_t_aliases.py` 校验 100% 覆盖。
> 旧 T ID 格式：`D3-S[1-7]-A\d{2}-T\d{2}` 或 `D3-S[1-7]-A\d{2}-T\d{2}`（Bridge T07 例外，旧 S2-T07 属 Bridge）

| 旧 T ID | 新 T ID | 备注 |
|---------|---------|------|
| D3-S1-A01-T01 | D3-S2-A01-T01 | DeepSeek 适配器流式响应（旧 Adapter → 新 StreamChat F01） |
| D3-S1-A01-T02 | D3-S2-A01-T02 | MiniMax 适配器流式响应（旧 Adapter → 新 StreamChat F01） |
| D3-S1-A01-T03 | D3-S2-A01-T03 | SSE parse error handling（旧 Adapter → 新 StreamChat F02） |
| D3-S1-A01-T04 | D3-S2-A01-T04 | OpenAI request body construction（旧 Adapter → 新 StreamChat F03） |
| D3-S2-A01-T01 | D3-S2-A01-T05 | LLM 调用可观测事件（旧 Gateway → 新 StreamChat 跨 A 验证） |
| D3-S2-A01-T02 | D3-S1-A01-T02 | 未知 Provider/Model 报错（旧 Gateway → 新 RouteModel F02a+F02b） |
| D3-S2-A01-T03 | D3-S1-A01-T01 | 多 Provider 并发调用（旧 Gateway → 新 RouteModel F01） |
| D3-S2-A01-T04 | D3-S3-A01-T05 | Retry 与 CB 联动（旧 Gateway → 新 ProtectCall Cross） |
| D3-S2-A01-T05 | D3-S3-A01-T06 | Half-Open 并发探测限制（旧 Gateway → 新 ProtectCall Breaker） |
| D3-S2-A01-T06 | D3-S3-A01-T07 | LLM 429 rate limit handling（旧 Gateway → 新 ProtectCall Retry） |
| D3-S2-A01-T07 | D3-X-A01-T01 | Bridge 适配（旧 Gateway 末尾 Bridge → 新 CROSS） |
| D3-S3-A01-T01 | D3-S3-A01-T01 | Circuit breaker Closed（旧 Breaker → 新 ProtectCall Breaker，ID 不变） |
| D3-S3-A01-T02 | D3-S3-A01-T02 | Circuit breaker Open（同上） |
| D3-S3-A01-T03 | D3-S3-A01-T03 | Circuit breaker HalfOpen→Closed（同上） |
| D3-S3-A01-T04 | D3-S3-A01-T04 | Circuit breaker HalfOpen→Open（同上） |
| D3-S3-A01-T05 | D3-S3-A01-T08 | 熔断器状态持久化（旧 Breaker PLANNED → 新 ProtectCall PLANNED） |
| D3-S4-A01-T01 | D3-S3-A01-T09 | 重试策略执行 Full Jitter 退避（旧 Retry → 新 ProtectCall Retry） |
| D3-S4-A01-T02 | D3-S3-A01-T10 | Full Jitter 随机化验证（旧 Retry → 新 ProtectCall Retry） |
| D3-S4-A01-T03 | D3-S3-A01-T11 | DeepSeek Fallback 模型切换（旧 Retry → 新 ProtectCall Retry） |
| D3-S4-A01-T04 | D3-S3-A01-T12 | MiniMax Fallback 模型切换（旧 Retry → 新 ProtectCall Retry） |
| D3-S5-A01-T01 | D3-S4-A01-T01 | Token 计数准确性（旧 Token → 新 BudgetTokens F01） |
| D3-S5-A01-T02 | D3-S4-A01-T02 | Token 预算检查（旧 Token → 新 BudgetTokens F03） |
| D3-S5-A01-T03 | D3-S4-A01-T03 | Token CJK 准确性（旧 Token → 新 BudgetTokens F01） |
| D3-S6-A01-T01 | D3-S6-A01-T01 | Provider 配置加载与验证（旧 Config → 新 ConfigureGateway，ID 不变） |
| D3-S7-A01-T01 | D3-S5-A01-T01 | Safety filter critical 拒绝（旧 Safety → 新 GuardContent F01） |
| D3-S7-A01-T02 | D3-S5-A01-T02 | Safety filter warning 匹配（旧 Safety → 新 GuardContent F01） |

**覆盖率**：26 / 26 = 100%

---

## 10. Statistics

| 维度 | v3.0.0 | v3.1.0 |
|------|--------|--------|
| Total T | 26 | **35**（+9 v1.1） |
| IMPLEMENTED | 25 | **34**（25 v1.0 保留 + 9 v1.1 全部实施） |
| PLANNED | 1 | **1**（仅 T08 持久化仍 PLANNED） |
| P0 | 12 | **19**（+7 v1.1：F1/F2/F4/F5/F8/F9 + 1 F6） |
| P1 | 13 | **15**（+2 v1.1：F3/F7） |
| P2 (PLANNED) | 1 | 1（不变；T08 持久化仍 PLANNED） |
| Legacy Archive 覆盖 | 26 / 26 (100%) | **26 / 26 (100%)**（v1.1 不破坏追溯） |
| v1.1 新 T PLANNED → IMPLEMENTED | — | 9 / 9（v1.1 S5 验收通过） |

---

## 11. 关联文档

| 文档 | 路径 | 关系 |
|------|------|------|
| A 层注册表 | `a-registry.md` | T 的编排容器 A |
| F 层注册表 | `f-registry.md` | T 的编排单元 F（含 F02a/F02b 拆分） |
| 设计 | `design.md §3.3` | T 的运行时验证逻辑 |
| 跨域边界 | `openspec/specs/architecture/cross-domain-boundaries.md` | D3-S5 灰区声明 |
| alias 校验脚本 | `scripts/check_t_aliases.py` | §Legacy Archive 100% 覆盖校验 |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 2.1.0 | 2026-06-14 | 7 S × 1 A × 26 T（无 Legacy Archive） |
| 3.0.0 | 2026-06-14 | 5+1 S × 1 A × 26 T；T ID 按 F 编排顺序重排；§Legacy Archive 26 条 100% 覆盖；T 末尾 `<!-- Mechanism: -->` 注释（R2 命题 A）；Bridge T07 移至 D3-X-A01-T01 |
| 3.1.0 | 2026-06-14 | 5+1 S × 1 A × 35 T（v1.1 净增 9）；新增 T03 (D3-S1 F6 probe) + T06 (D3-S2 F5 Protocol) + T13/T14/T15 (D3-S3 F1/F2/F3 + F7) + T03 (D3-S5 F8 latency) + T02 (D3-S6 F9 flag) + T01 (D3-X-A02 F4 fail-fast)；§Legacy Archive 26/26 (100%) 继承；8 v1.1 新 T 标注 `<!-- v1.1 Fx -->` |
| **3.1.1** | 2026-06-14 | **v1.1 S5 验收后**：9 v1.1 T PLANNED→IMPLEMENTED；IMPLEMENTED 25 → 34，PLANNED 9 → 1（仅 T08 持久化）；§0/§1/§10 统计与详细条目对齐 |
