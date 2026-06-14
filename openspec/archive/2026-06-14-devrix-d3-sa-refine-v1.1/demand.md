---
demand-id: DM-20260614-017
title: D3 LLM Gateway v1.1 — 韧性可见性 + 评测探针 + 适配扩展
source: 父需求 DM-20260614-016 v1.0 ACCEPTED，按 R1 Q5「v1.0 与 v1.1 合并发布」承诺接续；触发：R1 Q6/Q7 + R3 P1 #15/#16 落地为可执行代码与测试
priority: P0
status: S2_Clarified
dsaft_domain: D3
created: 2026-06-14
last-updated: 2026-06-14
review-round: R1（pending Owner）
parent: DM-20260614-016
playbook: dsaft-refactoring-playbook
---

# D3 LLM Gateway v1.1 — 韧性可见性 + 评测探针 + 适配扩展

> **本子 change 性质**：v1.0 ACCEPTED 的紧凑跟进；接续 R1 Q5「v1.0 与 v1.1 合并发布，避免注册表已价值流化但代码目录仍叫 adapter/ 的中间态」。本期落地 v1.0 决议中"占位"的运行时代码与 D5/D6 接力。

---

## 0. 父需求落地状态

| 父 Change | DM ID | 状态 | 触发关系 |
|----------|-------|------|---------|
| `devrix-d3-sa-refine` | DM-20260614-016 | **S5_Accepted (v1.0)** | 本期 v1.1 接续；v1.0 acceptance-report.md §6 已明确触发 |

**v1.0 已 ACCEPTED 但留作 v1.1 占位的项**（来自 v1.0 acceptance-report.md §4 决议清单）：

| 来源 | 占位项 | v1.1 落地形式 |
|------|--------|--------------|
| R1 Q6 | D3 → D5 韧性状态 metric `llm_breaker_state{provider,state}` | 设计 + 实现 + emit + D5 持久化 |
| R1 Q6 | D3 → D7 复用 EngineEvent，**不新增直接契约** | `FlowStarted` / `FlowFailed` 在 Breaker open/close 时 emit；D7 订阅验证 |
| R1 Q7 | D6 3 probe：Tier 解析正确性 / Breaker 状态切换次数 / Token 预算触发率 | D6 evolution spec 补丁 + 探针实施 + 阈值告警 |
| R3 P0 #8 | obs nil fail-fast `ErrObservabilityRequired` | v1.0 design.md 已声明；本期实施代码改造（`WireContextLLM` + `NewFromConfig`） |
| R3 P1 #11 | `Scope` 字段扩展（携带跨域元数据） | 类型定义 + 兼容性约束（旧 Scope 调用方零改动） |
| R3 P1 #15 | `IAdapter.Protocol() string` 接口扩展 | 接口新增方法 + 现有 2 adapter (DeepSeek / MiniMax) 实现 |
| R3 P1 #16 | Safety filter latency P99 告警 | span event `safety.check.duration_ms` + D6 probe #4 |
| R3 NQ-5 | Breaker 事件命名 | `flow.breaker.opened` / `closed` / `halfopened` 候选，本期第一个 issue 决议 |

---

## 1. v1.1 范围

### 1.1 In Scope（v1.1 必做）

| # | 项 | 责任域 | 输出 |
|---|----|--------|------|
| F1 | Metric `llm_breaker_state{provider, state}` 命名、维度、Cardinality 控制 | D3 → D5 | `span-registry.md` §metrics + provider 实施 |
| F2 | D3-S3 ProtectCall 状态切换时 emit `llm_breaker_state`（Feature flag `d3_resilience_emit_enabled`） | D3 | `breaker/circuit_breaker.go` 改造 |
| F3 | EngineEvent 复用：Breaker `Open`/`Closed`/`HalfOpen` 切换 emit `FlowStarted`/`FlowFailed` | D3 → D7（D7 不动） | `D3` 内部 wiring |
| F4 | obs nil fail-fast：`WireContextLLM` 缺 obs 时返回 `ErrObservabilityRequired` | D3 → bootstrap | `internal/bridges/llm/wire.go` 改造 |
| F5 | `IAdapter.Protocol() string` 接口扩展 + DeepSeek/MiniMax 实现 | D3 | `adapter/iadapter.go` + 2 impl |
| F6 | D6 probe #1：Tier 解析正确性（覆盖率 ≥ 99%） | D6 ← D3 | D6 evolution spec 补丁 |
| F7 | D6 probe #2：Breaker 状态切换次数（异常切换告警阈值） | D6 ← D3 | D6 evolution spec 补丁 + 阈值 |
| F8 | D6 probe #4：Safety filter latency P99 < 1ms span event + D6 阈值 | D3 → D6 | `safety/filter.go` 计时 + D6 probe |
| F9 | Feature flags：`d3_resilience_emit_enabled` / `d3_safety_latency_event_enabled` / `d3_metric_emit_warn` 缺省值与回滚开关 | D3 + bootstrap | config schema + 默认值 |

### 1.2 Out of Scope（保留至 v2.0 / 未来）

| 项 | 理由 |
|----|------|
| 物理目录迁移（`adapter/` → `stream/` 等） | v2.0 Phase F |
| `contracts.go` 拆分到子包 | v2.0 Phase F9 |
| D6 probe #3 Token 预算触发率 | **R1 议题 D2 待决议**（详见 §3）；若 Owner 接受可纳入 v1.1，否则推迟至 v1.2 |
| Breaker 状态持久化（`D3-S3-A01-T08` PLANNED） | v1.1 之后单独 issue；不阻断 v1.1 |
| 跨 provider 的 model context length 自适应 | 与 D2 Token 协调，非本期 |

---

## 2. v1.1 决议引用（v1.0 已闭合，本期直接消费）

| 决议 | 出处 | 本期消费方式 |
|------|------|--------------|
| 5+1 S 切法 + 6 A × 24 F | v1.0 R1 D1 | 本期所有改动落到 `D3-S3` ProtectCall / `D3-S5` GuardContent / `D3-S2` StreamChat / `D3-X` Bootstrap |
| Bridge 留 `internal/bridges/llm/` | v1.0 R1 D2 | F4 fail-fast 改造在跨域锚点，不入 D3 域内 |
| 运行时字面量不变（5 span 名 + 3 metric 名 + YAML key） | v1.0 R1 Q3 | 本期新增 metric `llm_breaker_state` 与 span event `safety.check.duration_ms` 在 v1.0 未使用过的命名空间，不冲突 |
| Legacy double-track | v1.0 R1 Q4 | 本期不新增 Legacy alias（无 T ID 重命名） |
| D3 → D7 通过 EngineEvent 复用 | v1.0 R1 Q6 | F3 落地；不写 D3→D7 直接契约 |
| D6 3 probe + Safety latency 阈值 | v1.0 R1 Q7 + R3 P1 #16 | F6/F7/F8 落地 |

---

## 3. R1 议题清单（待 Owner 评审）

### D1 Metric 命名：纯 metric vs span event

> **背景**：F1/F2 的核心是 Breaker 状态变化的可观测性。当前两个候选：

| 候选 | 形式 | 优点 | 缺点 |
|------|------|------|------|
| **D1-A** | `llm_breaker_state{provider, state}` Gauge metric | Prometheus 直查、易做 dashboard | 状态机变化需要轮询采集；不携带 trace context |
| **D1-B** | span event `breaker.state_transition{from, to}` + counter `llm_breaker_transitions_total{provider, from, to}` | 携带 trace context；可关联具体调用 | 命名更长；dashboard 需联表 |

> **倾向**：**D1-A**（Q6 决议已指明 metric 命名 `llm_breaker_state{provider,state}`，复用倾向；D1-B 是后续 v1.2 增强）。但 Owner 可推翻。

### D2 D6 probe #3 Token 预算触发率纳入 v1.1？

> **背景**：v1.0 R1 Q7 决议列了 3 个 probe；本期 In Scope 已含 #1 Tier + #2 Breaker + #4 Safety latency。probe #3 Token 预算触发率（截断/报错次数）是否同期落地？

| 候选 | 范围 |
|------|------|
| **D2-A** | v1.1 含 probe #3（D6 增 4 个 probe） |
| **D2-B** | v1.1 只含 1/2/4（D6 增 3 个 probe）；probe #3 推 v1.2 |

> **倾向**：**D2-B**（Token 预算触发逻辑与 D2-S4 Token 跨域协同强，单独 issue 更易管控；R3 NQ-6 v1.1 路线图已列）。

### D3 `IAdapter.Protocol()` 返回值类型

> **背景**：F5 接口扩展需要确定返回值类型。

| 候选 | 形式 | 优缺 |
|------|------|------|
| **D3-A** | `Protocol() string` | 灵活、易扩展、Provider 自填字符串 |
| **D3-B** | `Protocol() ProtocolKind`（const 枚举） | 类型安全、IDE 提示；新 Provider 必须改 enum 表 |

> **倾向**：**D3-A**（与 R3 P1 #15 决议原文一致 `Protocol() string`；保持开放扩展）。

### D4 Feature flags 默认值

> **背景**：F9 3 个 feature flag 在 v1.1 release 时的默认值。

| Flag | 默认 ON 风险 | 默认 OFF 风险 |
|------|--------------|---------------|
| `d3_resilience_emit_enabled` | metric cardinality 增加（按 provider×state；最坏 2×3=6 series） | dashboard 看不到 Breaker state |
| `d3_safety_latency_event_enabled` | span 体积略增 | 性能基线无法验证 |
| `d3_metric_emit_warn` | log noise（emit 失败时） | emit 失败静默 |

| 候选 | 全 ON | 仅 emit ON / warn OFF | 全 OFF |
|------|-------|----------------------|--------|
| **D4-A** | 全 ON（最大可见性） | — | — |
| **D4-B** | `d3_resilience_emit_enabled` ON + `d3_safety_latency_event_enabled` ON + `d3_metric_emit_warn` OFF | — | — |
| **D4-C** | 全 OFF（默认安全，需手动启用） | — | — |

> **倾向**：**D4-B**（emit 默认开，确保 dashboard 可见；warn 默认关，避免 log noise；与 R3 P1 #16 / Q6 决议精神一致：可见性优先，但保留回滚开关）。

### D5 Safety latency 阈值

> **背景**：F8 D6 probe #4 的 P99 告警阈值。

| 候选 | 阈值 | 依据 |
|------|------|------|
| **D5-A** | P99 < 1ms | v1.0 design.md §6.4 #1 原文阈值 |
| **D5-B** | P99 < 5ms | 留余量给 v2.0 ML 内容过滤 |

> **倾向**：**D5-A**（v1.0 已声明 1ms 阈值，保持一致；ML 内容过滤是 v3.0 范围，本期不影响）。

### D6 Breaker 事件命名（R3 NQ-5）

> **背景**：v1.0 cross-domain-boundaries.md §2.4.3 标明「Breaker 事件命名由 v1.1 第一个 issue 决定」。本期需固化。

| 候选 | 命名 |
|------|------|
| **D6-A** | `flow.breaker.opened` / `flow.breaker.closed` / `flow.breaker.halfopened` |
| **D6-B** | `breaker.state_transition` 单事件 + `from`/`to` 属性 |

> **倾向**：**D6-A**（3 事件分开易订阅、命名更具语义；与 EngineEvent 现有 `FlowStarted` / `FlowFailed` 命名风格一致 `<noun>.<action>`）。

### D7 v1.1 范围与子 change 拆分

> **背景**：v1.1 范围横跨 D3 + D5 + D6 + bootstrap，是否拆为 3 个并行子 change？

| 候选 | 拆分方式 |
|------|----------|
| **D7-A** | 单 change `devrix-d3-sa-refine-v1.1`（本 change，覆盖全部 F1-F9） |
| **D7-B** | 拆 3：`-v1.1-metric`（F1-F4）+ `-v1.1-adapter`（F5）+ `-v1.1-probes`（F6-F8） |
| **D7-C** | 拆 2：`-v1.1-d3-emit`（D3 侧 F1-F5 + F9）+ `-v1.1-d6-probes`（F6-F8） |

> **倾向**：**D7-A**（v1.0 已声明「v1.0 与 v1.1 合并发布」紧凑窗口，避免多 change 来回切换；F1-F9 强耦合于韧性可见性主题）。

---

## 4. 影响范围（v1.1 必动文件）

| 类型 | 路径 | 改动性质 |
|------|------|---------|
| 代码 | `internal/layers/llmgateway/breaker/circuit_breaker.go` | F2 emit metric |
| 代码 | `internal/layers/llmgateway/breaker/state.go` | F2/F3 状态变化事件钩子 |
| 代码 | `internal/layers/llmgateway/adapter/iadapter.go` | F5 接口新增 `Protocol() string` |
| 代码 | `internal/layers/llmgateway/adapter/deepseek.go` | F5 实现 |
| 代码 | `internal/layers/llmgateway/adapter/minimax.go` | F5 实现 |
| 代码 | `internal/layers/llmgateway/safety/filter.go` | F8 span event 计时 |
| 代码 | `internal/bridges/llm/wire.go` | F4 fail-fast `ErrObservabilityRequired` |
| 代码 | `internal/bridges/llm/context_wiring.go` | F4 配合 |
| 代码 | `internal/shared/contracts/llm.go` 或 `gateway/contracts.go` | F5 接口签名（若 IAdapter 在此） |
| 配置 | `internal/shared/config/llmgateway.go` | F9 3 feature flag |
| Spec | `openspec/specs/d3-llm-gateway/spec.md` | v3.1.0：新增 F1-F9 Requirements |
| Spec | `openspec/specs/d3-llm-gateway/design.md` | v3.1.0：F1-F9 时序与 Feature flag |
| Registry | `openspec/specs/d3-llm-gateway/f-registry.md` | v3.1.0：F5 IAdapter.Protocol 新增 |
| Registry | `openspec/specs/d3-llm-gateway/t-registry.md` | v3.1.0：新 T 点（v1.1 覆盖测试） |
| Registry | `openspec/specs/d3-llm-gateway/span-registry.md` | v3.1.0：`llm_breaker_state` + `safety.check.duration_ms` |
| Spec | `openspec/specs/d6-evolution/spec.md` | 增 probe #1/#2/#4 + 阈值 |
| Spec | `openspec/specs/architecture/cross-domain-boundaries.md` | v1.1.0：§2.4.3 Breaker 事件命名 D6-A 决议固化 |

---

## 5. 验收预期（v1.1 AC 占位）

| AC | 准则 | 验证 |
|----|------|------|
| AC-01 | F1 metric `llm_breaker_state{provider,state}` 在 Prometheus 可查到 | `curl /metrics \| grep llm_breaker_state` |
| AC-02 | F2 Breaker `Open` 切换时 metric 值 +1（state="open"），`Closed` 时 state="closed" | integration test |
| AC-03 | F3 Breaker 状态切换 emit EngineEvent，D7 订阅可见 | D7 集成测试 |
| AC-04 | F4 obs nil 时 `WireContextLLM` 返回 `ErrObservabilityRequired`，不 silent fallback | unit test |
| AC-05 | F5 `IAdapter.Protocol()` 返回非空字符串（DeepSeek="openai-compat", MiniMax="openai-compat" 或 vendor 标识） | unit test |
| AC-06 | F6 D6 probe #1 Tier 解析覆盖率 ≥ 99% | D6 报告 |
| AC-07 | F7 D6 probe #2 Breaker 异常切换告警阈值落地 | D6 报告 |
| AC-08 | F8 Safety filter span event `safety.check.duration_ms` 在 trace 中可见 | Jaeger 查询 |
| AC-09 | F8 D6 probe #4 P99 < 1ms 告警阈值落地 | D6 报告 |
| AC-10 | F9 3 feature flag 默认值与 D4 决议一致；OFF 时 v1.0 行为完全保持 | unit test + integration test |
| AC-11 | v1.0 5 个运行时 span 名 + 3 metric 名仍字面量不变（不变性继承） | grep 校验 |
| AC-12 | v1.0 全部 26 T 仍全绿（回归） | full T 跑 |

---

## 6. 风险预登记

| # | 风险 | 缓解 |
|---|------|------|
| 1 | F1 metric cardinality 失控（`provider × state` 维度若 provider 多） | 默认仅 2 provider × 3 state = 6 series；约束 provider 字段必须来自配置文件（不可动态生成） |
| 2 | F4 fail-fast 改 bootstrap 可能影响测试 fixture | 测试 fixture 显式注入 mock obs；新增 `WithMockObs()` helper |
| 3 | F5 接口扩展是 BREAKING（旧 IAdapter 实现编译失败） | v1.1 release 时跟随；Provider 列表已知（仅 DeepSeek + MiniMax），可控 |
| 4 | F8 span event 增加 trace volume | feature flag `d3_safety_latency_event_enabled` 控制；P99 计算外移到 D6 avoiding hot path |
| 5 | D6 probe 接入需要 D6 配合 | 已与 D6 R1 Q7 对齐；本期通过 D6 spec 补丁 + 探针实施 |
| 6 | v1.1 release 节奏紧（紧跟 v1.0），多文件改动 | 已拆细 F1-F9；R1 议题清单先行决议；S3 Gate 严卡 |

---

## 7. 后续阶段计划

| 阶段 | 交付物 | 触发条件 |
|------|--------|---------|
| S1（本文档） | demand.md v0.1（含 R1 议题清单） | 本期已交付 |
| S2 Clarified | demand.md v0.2（R1 决议落定）+ proposal.md（F1-F9 拆解） | Owner R1 评审通过 |
| S3 Design | spec.md v3.1.0 + design.md v3.1.0 + 注册表 v3.1.0 补丁 + cross-domain-boundaries.md v1.1.0 | S2 完成 |
| S3-Gate | review-r1.md（Owner）+ review-r2.md（Claude 结构层）+ review-r3.md（Claude 运行层） | S3 完成 |
| S4 实现 | F1-F9 代码 + 新 T 测试 + Feature flag | S3-Gate 通过 |
| S4-Gate | code review（综合 Codex/Gemini） | S4 完成 |
| S5 验收 | acceptance-report.md（v1.1） | S4-Gate 通过 + 全量 T 回归绿 |
| S6 归档 | archive 路径 + demand-archive-index.md ACCEPTED 状态 | S5 通过 |

---

## 8. 反向链接

| 文档 | 路径 | 关系 |
|------|------|------|
| 父 Change v1.0 demand | `openspec/changes/devrix-d3-sa-refine/demand.md` | DM-016 |
| 父 Change v1.0 acceptance | `openspec/changes/devrix-d3-sa-refine/acceptance-report.md` | v1.0 ACCEPTED 触发本期 |
| 父 Change v1.0 R1 | `openspec/changes/devrix-d3-sa-refine/review-r1.md` | Q5/Q6/Q7 决议 |
| 父 Change v1.0 R3 | `openspec/changes/devrix-d3-sa-refine/review-r3.md` | P0 #8 / P1 #11 #15 #16 / NQ-5 |
| D3 spec | `openspec/specs/d3-llm-gateway/spec.md` v3.0.0 | 本期补丁基线 |
| 跨域边界 | `openspec/specs/architecture/cross-domain-boundaries.md` v1.0.0 | §2.4.3 Breaker 事件命名待 v1.1 决议 |
| D6 spec | `openspec/specs/d6-evolution/spec.md` | F6/F7/F8 落地点 |

---

## 9. R1 评审请求（Owner）— 已闭合 ✅

Owner 评审结果（2026-06-14）：**全部接受倾向选项**。

| # | 议题 | 决议 |
|---|------|------|
| D1 | Metric 命名 | **D1-A** `llm_breaker_state{provider, state}` Gauge metric（与 R1 Q6 决议一致） |
| D2 | probe #3 Token 预算触发率 | **D2-B** 不纳入 v1.1；推迟至 v1.2 单独 issue（与 R3 NQ-6 路线图一致） |
| D3 | `IAdapter.Protocol()` 返回值类型 | **D3-A** `Protocol() string`（与 R3 P1 #15 原文一致） |
| D4 | Feature flag 默认值 | **D4-B** `d3_resilience_emit_enabled` ON + `d3_safety_latency_event_enabled` ON + `d3_metric_emit_warn` OFF |
| D5 | Safety latency 阈值 | **D5-A** P99 < 1ms（与 v1.0 design.md §6.4 #1 一致） |
| D6 | Breaker 事件命名 | **D6-A** `flow.breaker.opened` / `closed` / `halfopened` 三事件分开（与 EngineEvent 现有 `FlowStarted`/`FlowFailed` 风格一致） |
| D7 | 子 change 拆分 | **D7-A** 单 change（v1.0 R1 Q5「v1.0 与 v1.1 合并发布」紧凑窗口） |

**BREAKING 风险确认**：grep IAdapter 实现共 3 处（`DeepSeekAdapter`、`MiniMaxAdapter`、`stubAdapter` test fixture）；无外部 plugin，BREAKING 影响可控（v1.1 release 时同步改）。

→ 状态从 `S1_Open` → **`S2_Clarified`**，启动 S2 阶段 `proposal.md` 撰写。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初版：S1 阶段 demand.md；R1 议题清单 7 项；范围 F1-F9；与父 Change v1.0 R1/R3 决议交叉引用 |
| 0.2 | 2026-06-14 | R1 评审闭合：D1-D7 全部接受倾向选项；IAdapter 影响范围 3 处确认；状态 `S1_Open` → `S2_Clarified` |
