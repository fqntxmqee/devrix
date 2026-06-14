# Proposal: D3 LLM Gateway v1.1 — 韧性可见性 + 评测探针 + 适配扩展

**Change ID:** devrix-d3-sa-refine-v1.1
**Demand ID:** DM-20260614-017
**Status:** S2_Clarified (Review R1 incorporated)
**Phase Scope:** v1.1 Traceability（runtime + spec patch；不动物理目录）

---

## 1. Background

`devrix-d3-sa-refine`（DM-20260614-016）已于 2026-06-14 完成 **v1.0 Registry** 阶段（acceptance-report.md 状态 ACCEPTED）。v1.0 完成了：

- 5+1 S 价值流切法（RouteModel / StreamChat / ProtectCall / BudgetTokens / GuardContent + ConfigureGateway）
- 6 A × 24 F（域内）+ CROSS 2 A × 2 F 的注册表重排
- 26 条 T 全绿（11 P0 + 12 P1 + 1 P2 PLANNED）+ Legacy double-track 100% 覆盖
- `cross-domain-boundaries.md` 新建 + `code-layout.md §4` 补 D3 scenario-slug 注册表
- Bridge / Bootstrap 跨域归位（`internal/bridges/llm/` → D3-X 段）

v1.0 全部 17 项 R1/R2/R3 决议落地，**但 5 项决议**保留为 v1.1 占位（详见 `acceptance-report.md §4`）：

| 决议 | 占位内容 | v1.1 落地形式 |
|------|---------|---------------|
| R1 Q6 | D3 → D5 韧性状态 metric `llm_breaker_state{provider,state}` | 设计 + 实现 + emit + D5 持久化 |
| R1 Q6 | D3 → D7 复用 EngineEvent，**不新增直接契约** | Breaker open/close 时 emit `FlowStarted`/`FlowFailed` |
| R1 Q7 | D6 3 probe：Tier 解析 / Breaker 切换 / Token 预算 | D6 spec 补丁 + 探针实施 + 阈值告警 |
| R3 P0 #8 | obs nil fail-fast `ErrObservabilityRequired` | `WireContextLLM` + `NewFromConfig` 实施 |
| R3 P1 #15 / #16 | `IAdapter.Protocol()` + Safety P99 < 1ms | 接口扩展 + span event + D6 probe #4 |

本期 v1.1 是 **v1.0 R1 Q5「v1.0 与 v1.1 合并发布」**承诺的接续子 change；只动运行时代码与 spec 补丁，**不动物理目录**（物理迁移 → v2.0 Phase F）。

---

## 2. Problem Statement

### 2.1 韧性状态「可见性真空」

D3 ProtectCall 在 v1.0 已实现 Breaker 状态机（Closed / Open / HalfOpen），但**状态切换不对外暴露**：

- D5 Observability 看不到 Breaker 当前状态 → dashboard 无 Provider 健康视图
- D7 Orchestrator 不知道 Breaker 已 open → 仍向故障 Provider 派发 task
- 排障时只能 grep 业务日志（`circuit_breaker.go` 内的 log），无结构化 metric

**结果**：故障发生时 SRE 在 D5 dashboard 看不到任何「Breaker 已 open」信号，只能从 D4 业务指标（latency 飙升 / 错误率上升）反推；MTTR 偏高。

### 2.2 评测盲区

D6 Evolution 引擎在 v1.0 未与 D3 建立直接探针：

- **Tier 解析正确性**无自动校验（每次 release 需手动跑 fixture）
- **Breaker 状态切换**无异常告警（毛刺切换、抖动切换、误触发 open 都被忽略）
- **Safety filter latency** 无基线（v1.0 阈值 P99 < 1ms 已声明，但无 span event 落地）

**结果**：D6 自演化引擎缺少 D3 侧的「评测探针」数据，v1.0 release 后无法自动化回测。

### 2.3 适配器扩展接口受限

`IAdapter` 当前仅暴露 `Stream(ctx, req, cfg) <-chan Chunk` + `Name() string`：

- 新 Provider 想声明「协议类型」(OpenAI-compat / Anthropic-native / 自研) 需新增方法
- 路由层无法基于 `Protocol()` 做 protocol-aware fallback（v2.0 计划）
- 跨域工具（Sidecar / Proxy）无法通过 `Protocol()` 标识自己

**结果**：v1.0 设计中已为 `IAdapter.Protocol()` 留位（R3 P1 #15），但接口未扩；新 Provider 接入时无统一契约。

### 2.4 Bootstrap fail-silent 风险

`WireContextLLM` 在 v1.0 实现允许 obs 为 nil 时 silent fallback（使用 noop obs），这导致：

- 测试环境误传 nil obs，prod 环境真实使用 noop → metric 全丢
- v1.0 acceptance §5 风险项 R-2 已登记：`obs nil fail-fast` 是 v1.1 必做

---

## 3. Proposed Solution

### 3.1 F 拆解（v1.1 范围）

F 编号沿用 v1.0 体系 `D3-S{N}-A{XX}-F{NN}`；v1.1 新增 9 个 F（F1-F9 横向编号对应需求项，便于跨域协调追踪）：

| F ID | Name | 挂载 A | Mechanism | 跨域方 | Owner 决议 |
|------|------|--------|-----------|--------|-----------|
| F1 | EmitBreakerStateMetric | D3-S3-A01 ShieldAndRetry | Breaker → Metric | D5 接收 | D1-A |
| F2 | OnStateTransitionEmit | D3-S3-A01 ShieldAndRetry | Breaker → Event | D7 订阅 | D1-A / D6-A |
| F3 | ReuseEngineEvent | D3-S3-A01 ShieldAndRetry | Breaker → EngineEvent | D7 不动 | D6-A |
| F4 | FailFastOnObsNil | D3-X-A02 WireLLMStack | Bootstrap fail-fast | — | (R3 P0 #8) |
| F5 | AdapterProtocolMethod | D3-S2-A01 StreamChatCompletion | IAdapter 扩展 | — | D3-A |
| F6 | ProbeTierResolution | D3-S1-A01 ResolveModelRoute | D6 探针 #1 | D6 实施 | D2-B (与 #3 区分) |
| F7 | ProbeBreakerTransitions | D3-S3-A01 ShieldAndRetry | D6 探针 #2 | D6 实施 | D2-B |
| F8 | EmitSafetyLatencyEvent | D3-S5-A01 FilterAndMatchContent | Span event + D6 探针 #4 | D6 实施 | D5-A |
| F9 | FeatureFlagDefaults | D3-S6-A01 LoadAndValidateLLMConfig | Config schema | bootstrap | D4-B |

**F ↔ 注册表影响**：

| 注册表 | v1.1 改动 |
|--------|----------|
| `f-registry.md` | v3.1.0：D3-S3-A01 新增 F02b EmitBreakerStateMetric、F02c OnStateTransitionEmit、F02d ReuseEngineEvent；D3-S2-A01 新增 F04 AdapterProtocolMethod；D3-S5-A01 新增 F04 EmitSafetyLatencyEvent；D3-X-A02 新增 F02 FailFastOnObsNil；D3-S6-A01 新增 F05 FeatureFlagDefaults |
| `t-registry.md` | v3.1.0：新增 9 条 T（每个 F 一条 P0/P1） + alias 100% 继承（不增不删） |
| `span-registry.md` | v3.1.0：§Metrics 新增 `llm_breaker_state{provider,state}` Gauge；§Span events 新增 `safety.check.duration_ms` |
| `spec.md` | v3.1.0：5 章 North Star 承诺保持，新增 §6 v1.1 Requirements（9 项 FR） |
| `design.md` | v3.1.0：§3 时序图新增 F1-F9 子图；§6 配置矩阵补 3 feature flag |
| `cross-domain-boundaries.md` | v1.1.0：§2.4.3 Breaker 事件命名 D6-A 决议固化 |

### 3.2 跨域责任分配

| F | D3 自留 | D5 接收 | D6 探针 | D7 订阅 | Bootstrap |
|---|---------|---------|---------|---------|----------|
| F1 EmitBreakerStateMetric | `breaker/circuit_breaker.go` emit | `d5-observability` 持久化 + dashboard | — | — | — |
| F2 OnStateTransitionEmit | `breaker/state.go` 状态变化钩子 | — | — | — | — |
| F3 ReuseEngineEvent | `d3` 内部 emit `flow.breaker.*` 事件 | — | — | 通过现有 `EngineEvent` 订阅 | — |
| F4 FailFastOnObsNil | — | — | — | — | `bridges/llm/wire.go` 返回 `ErrObservabilityRequired` |
| F5 AdapterProtocolMethod | `adapter/iadapter.go` + 2 impl | — | — | — | — |
| F6 ProbeTierResolution | D3 提供 `Resolve()` 调用次数 + 错误率 | D5 metric 持久化 | D6 探针 #1 校验覆盖率 ≥ 99% | — | — |
| F7 ProbeBreakerTransitions | D3 emit 状态切换次数 | D5 metric 持久化 | D6 探针 #2 异常切换告警 | — | — |
| F8 EmitSafetyLatencyEvent | `safety/filter.go` 计时写入 span event | D5 trace 持久化 | D6 探针 #4 P99 < 1ms 告警 | — | — |
| F9 FeatureFlagDefaults | `shared/config/llmgateway.go` schema + 默认值 | — | — | — | `WireContextLLM` 读取 flag |

**跨域契约边界**（D3 主权原则）：

| 项 | D3 SoT | 邻域 SoT | 引用 |
|----|--------|---------|------|
| Breaker metric 命名 | D3 定义 `llm_breaker_state` | D5 仅持久化不重命名 | `span-registry.md §Metrics` v3.1.0 |
| EngineEvent 事件名 | D3 决定 `flow.breaker.*` | D7 订阅方式不变 | `cross-domain-boundaries.md §2.4.3` v1.1.0 |
| D6 探针 schema | D3 定义探针元数据 | D6 负责执行 | `d6-evolution/spec.md` 补丁 |
| obs fail-fast 错误码 | D3 定义 `ErrObservabilityRequired` | bootstrap 抛 | `internal/shared/errors/` 复用 |

### 3.3 运行时不变性继承

v1.0 R1 Q3 决议：「5 span 名 + 3 metric 名 + YAML key 字面量不变」。v1.1 新增命名空间：

| 命名空间 | 类型 | 是否新 |
|---------|------|--------|
| `llm_breaker_state{provider, state}` | metric | **新**（v1.0 未使用） |
| `safety.check.duration_ms` | span event | **新**（v1.0 未使用） |
| `flow.breaker.opened` / `closed` / `halfopened` | event | **新**（v1.0 未使用） |
| `d3_resilience_emit_enabled` / `d3_safety_latency_event_enabled` / `d3_metric_emit_warn` | config key | **新** |

**v1.0 既有命名空间字面量继承**（AC-11）：

- 5 span 名（`llm.chat.stream` 等）+ 3 metric 名（`llm.call.duration_ms` 等）+ 26 YAML key
- 全部 T ID 沿用（v1.1 不改 T 编号）
- Legacy double-track 100% 覆盖（v1.1 不破坏追溯）

### 3.4 Feature flag 矩阵（D4-B 决议）

| Flag | 类型 | 默认 | 作用 | 回滚影响 |
|------|------|------|------|---------|
| `d3_resilience_emit_enabled` | bool | **ON** | 控制 F1/F2 状态 emit | OFF → Breaker 状态变化仅业务日志，无 metric |
| `d3_safety_latency_event_enabled` | bool | **ON** | 控制 F8 span event 写入 | OFF → Safety check 无计时；D6 probe #4 退化 |
| `d3_metric_emit_warn` | bool | **OFF** | emit 失败时是否 log warn | ON → log noise；OFF → silent skip |

**默认值理由**：
- `d3_resilience_emit_enabled` ON：v1.0 决议要求 dashboard 可见性，cardinality 受控（2 provider × 3 state = 6 series）
- `d3_safety_latency_event_enabled` ON：v1.0 阈值 P99 < 1ms 需 span event 才能验证
- `d3_metric_emit_warn` OFF：emit 失败不应当污染日志；走 D5 健康检查（emit 失败率作为 D5 内部 metric）

### 3.5 错误码签名

| 错误 | 抛出 F | 含义 |
|------|--------|------|
| `ErrObservabilityRequired` | F4 | bootstrap 时 obs = nil |
| `ErrAdapterProtocolNotImplemented` | F5 | IAdapter 旧实现未补 `Protocol()`（编译期阻断；v1.1 release 前必须修） |
| `ErrSafetyLatencyThreshold` | F8 | Safety check > 1ms（P99 告警；D6 触发） |
| `ErrBreakerAnomalyTransition` | F7 | 短时间内 Breaker 反复切换（D6 探针 #2 触发） |

### 3.6 三段终态（v1.0 / v1.1 / v2.0）— 进度更新

| 版本 | 范围 | 关键产出 | 状态 |
|------|------|---------|------|
| **v1.0 Registry** | 4 注册表重排 + Legacy alias + layering.md 同步 + code-layout.md §4 补 D3 + cross-domain-boundaries.md 新建 + Bridge 跨域归位 | 5+1 S 价值流化 + 26 条 T 全绿 | ✅ **ACCEPTED** |
| **v1.1 Traceability** | F1-F9 运行时代码 + 3 feature flag + 4 spec 补丁 + D6 3 probe | 跨域可观测 + 评测闭环 + IAdapter 扩展 | ⏳ **本 change** |
| **v2.0 Structure** | 物理路径迁移到 scenario-slug + contracts.go 拆分到子包 + re-export 桥接 1 周期 + 旧路径 dead code 清理 | 物理目录与价值流 S 1:1 对齐 | 📅 下一子 change（DM 待申请） |

---

## 4. Success Metrics

| 指标 | v1.0 基线 | v1.1 目标 | 验证 |
|------|----------|----------|------|
| D3 价值流 S 数 | 5 + 1 | 5 + 1（保持） | layering.md |
| P0 T 全绿数 | 11 | 11 + v1.1 新增（预估 +4） | t-registry v3.1.0 |
| T 总数 | 26 | 26 + v1.1 新增（预估 +9） | t-registry v3.1.0 |
| D3 韧性状态对 D5 可见性 | ❌ | ✅ `llm_breaker_state` dashboard 可查 | AC-01 / AC-02 |
| D3 韧性状态对 D7 可见性 | ❌ | ✅ EngineEvent 订阅 | AC-03 |
| D6 Safety 评测点 | 0 | 3（Tier / Breaker / Safety latency） | AC-06/07/09 |
| D6 接入 probe 数 | 0 | 3 | d6-evolution spec 补丁 |
| IAdapter 协议标识 | ❌ | ✅ `Protocol() string` | AC-05 |
| Bootstrap fail-fast | ❌ silent | ✅ `ErrObservabilityRequired` | AC-04 |
| v1.0 运行时字面量继承 | 5 span + 3 metric | 5 span + 3 metric + 1 new + 1 new event + 3 flag | AC-11 |
| v1.0 26 T 回归 | 全绿 | 全绿（保持） | AC-12 |
| 跨域 import 违规（D3 import D2/D4） | 0 | 0（保持） | grep 校验 |

---

## 5. Implementation Plan（OpenSpec S2→S6 阶段，无估时）

> **不估时**（playbook 原则 + OpenSpec S2 阶段约束）。各 Phase 内容描述而非时间盒。

### Phase S2 — 提案固化（本文档 + 5 review 入口）

- ✅ `demand.md` v0.2（R1 7 项决议已闭合）
- ⏳ `proposal.md`（本文件，F1-F9 拆解 + 跨域责任分配）
- ⏳ `tasks.md` v0.1（v1.1 阶段任务分解骨架）

### Phase S3 — 设计

- `openspec/specs/d3-llm-gateway/spec.md` v3.1.0：新增 §6 v1.1 Requirements（9 项 FR）+ §7 Feature Flag 矩阵
- `openspec/specs/d3-llm-gateway/design.md` v3.1.0：§3 时序图新增 F1-F9 子图；§6 Feature flag 配置矩阵
- `openspec/specs/d3-llm-gateway/f-registry.md` v3.1.0：6 F 新增/调整
- `openspec/specs/d3-llm-gateway/t-registry.md` v3.1.0：9 新 T + alias 100% 继承校验
- `openspec/specs/d3-llm-gateway/span-registry.md` v3.1.0：1 新 metric + 1 新 span event
- `openspec/specs/architecture/cross-domain-boundaries.md` v1.1.0：§2.4.3 Breaker 事件命名 D6-A 决议固化
- `openspec/specs/d6-evolution/spec.md` 补丁：probe #1 / #2 / #4 实施细节 + 阈值

### Phase S3-Gate — 设计评审

- `review-r1.md`（Owner）：5 项 v1.1 决议确认（继承 v1.0）
- `review-r2.md`（Claude 结构层）：F 编排与注册表一致性
- `review-r3.md`（Claude 运行层）：运行时字面量不变性 + feature flag 矩阵

### Phase S4 — 实现

| F | 代码改动 | 测试 |
|---|---------|------|
| F1 | `breaker/circuit_breaker.go`：状态切换处 emit `llm_breaker_state` metric | T + integration test + Prometheus 抓取验证 |
| F2 | `breaker/state.go`：状态变化钩子；`Open`/`Closed`/`HalfOpen` 三事件 | unit test 状态机 + integration test 事件流 |
| F3 | `breaker/` → `events/`：emit `flow.breaker.opened` / `closed` / `halfopened` | unit test 事件命名 + D7 集成测试 |
| F4 | `internal/bridges/llm/wire.go` + `context_wiring.go`：obs nil 检查 → `ErrObservabilityRequired` | unit test（nil obs 报错）/ fixture test（mock obs 注入） |
| F5 | `adapter/iadapter.go` 接口新增 `Protocol() string`；`deepseek.go` + `minimax.go` 实现 | unit test 3 个 impl 返回值；编译期阻断测试 |
| F6 | D3-S1 `router.go`：调用次数 + 错误率 metric（`llm_tier_resolve_total`） | unit test 路由正确性 ≥ 99% |
| F7 | D3-S3：emit `llm_breaker_transitions_total{provider, from, to}` | unit test 状态机 + D6 probe 集成 |
| F8 | `safety/filter.go`：计时写入 span event `safety.check.duration_ms` | unit test 计时精度 + Jaeger 查询验证 |
| F9 | `shared/config/llmgateway.go` + `wire.go`：3 feature flag schema + 默认值 + 读取 | unit test flag 读取；integration test OFF 行为继承 v1.0 |

**关键依赖**：
- F4 依赖 `internal/shared/errors/ErrObservabilityRequired` 已存在（v1.0 R3 P0 #8 占位）
- F5 依赖 v1.0 已存在的 `adapter/iadapter.go` 路径
- F6/F7/F8 依赖 D6 spec 补丁完成（顺序：先 D6 spec → 后 D3 实施）

### Phase S4-Gate — 代码评审

- `code review`（综合 Codex + Gemini）：CRITICAL / HIGH 全部闭合
- `go build ./...` + `go test ./internal/layers/llmgateway/... ./internal/bridges/llm/...` 全绿
- v1.0 26 T 回归全绿

### Phase S5 — 验收

- 跑通 AC-01 ~ AC-12（详见 `demand.md §5`）
- D5 dashboard 抓取 `llm_breaker_state` 验证
- D7 集成测试验证 `flow.breaker.*` 订阅
- D6 探针 #1 / #2 / #4 接入 + 阈值告警验证
- 产出 `acceptance-report.md（v1.1）` 状态 = ACCEPTED

### Phase S6 — 归档

- change 包归档到 `openspec/archive/2026-06-14-devrix-d3-sa-refine-v1.1/`
- `demand-archive-index.md` 标记 v1.1 ACCEPTED
- 反向链接更新（D6 spec 引用 v1.1 决议）

---

## 6. Risks & Mitigations

| # | 风险 | 可能性 | 影响 | 缓解 |
|---|------|--------|------|------|
| R-1 | F1 metric cardinality 失控（provider 多时） | 低 | 中 | provider 字段必须来自配置文件（不可动态生成）；v1.1 已知 2 provider |
| R-2 | F4 fail-fast 改 bootstrap 影响测试 fixture | 中 | 中 | 测试 fixture 显式注入 mock obs；新增 `WithMockObs()` helper |
| R-3 | F5 接口扩展是 BREAKING（旧 IAdapter 编译失败） | 中 | 中 | Provider 列表已知（仅 DeepSeek + MiniMax + stubAdapter test fixture = 3 处），可控；v1.1 release 时同步改 |
| R-4 | F8 span event 增加 trace volume | 低 | 低 | feature flag `d3_safety_latency_event_enabled` 控制；P99 计算外移到 D6 避免 hot path |
| R-5 | D6 探针接入需 D6 配合 | 中 | 中 | 已与 D6 R1 Q7 对齐；本期通过 D6 spec 补丁 + 探针实施 |
| R-6 | F2/F3 状态变化事件可能与现有 `Open` 字段重复 emit | 低 | 低 | 状态变化钩子处加 idempotent 保护；首次 emit 后无变化不重复 |
| R-7 | Feature flag 默认值与 v1.0 行为不一致 | 低 | 中 | D4-B 决议保留 v1.0 行为作为 OFF fallback；unit test 验证 OFF 时 v1.0 行为完全保持 |
| R-8 | v1.1 release 节奏紧（紧跟 v1.0），多文件改动 | 中 | 中 | 已拆细 F1-F9；R1 议题清单先行决议；S3-Gate 严卡 |
| R-9 | `flow.breaker.*` 事件名与 D7 既有事件命名风格冲突 | 低 | 低 | D6-A 决议采用 `<noun>.<action>` 风格与 `FlowStarted`/`FlowFailed` 一致；D7 评审确认 |
| R-10 | v2.0 物理迁移时 v1.1 新增文件路径可能变 | 中 | 低 | v1.1 仅动运行时代码，文件路径不变；v2.0 物理迁移时再处理 |

---

## 7. Out of Scope（明确不属于 v1.1）

| 项 | 理由 / 归属 |
|---|-----------|
| 物理目录迁移（`adapter/` → `stream/` 等 6 处） | v2.0 Phase F |
| `contracts.go` 拆分到子包 | v2.0 Phase F9 |
| D6 probe #3 Token 预算触发率 | **D2-B 决议**推迟至 v1.2 单独 issue（与 D2-S4 Token 跨域协同） |
| Breaker 状态持久化（`D3-S3-A01-T08` PLANNED） | v1.1 之后单独 issue；不阻断 v1.1 |
| 跨 provider 的 model context length 自适应 | 与 D2 Token 协调，非本期 |
| Provider 适配器重写（DeepSeekAdapter / MiniMaxAdapter 行为不变） | 行为冻结，v1.1 仅 `Protocol()` 扩展 |
| Safety filter 与 D2-S18 PermissionMode 合并 | 需另立 D2 change；v1.0 已声明边界 |
| V3 计划（Anthropic Adapter / Rate Limiter / 负载均衡） | 属未来 change |
| D3 公共域对**新消费者**（非 D2/D4）的开放 | 属未来 change；ILLMGateway 契约已具备扩展性 |
| Audit 日志（合规审计） | 属 D5 Observability；D3 仅 emit metric |
| 跨域 metric 命名空间统一 | D5 内部议题；D3 仅提供 metric 名称 |

---

## 8. 与 v1.0 提案的对照

| 维度 | v1.0 Registry | v1.1 Traceability（本 change） |
|------|---------------|--------------------------------|
| 核心矛盾 | S 切法为技术角色词 + 公共域身份未贯彻 | 韧性状态不可见 + 评测盲区 + 适配扩展受限 |
| 关键 Decision | 5+1 S 切法 + Bridge 跨域归位 + Safety 归属 | D1-D7（metric 命名 / probe #3 / Protocol 类型 / flag 默认值 / 阈值 / 事件命名 / 拆分） |
| 迁移期 | T ID alias 表 + metric 名不变 | **继承** v1.0 不变性 + 新增命名空间不与 v1.0 冲突 |
| 范围 | 4 注册表 + spec + design + layering + code-layout + cross-domain-boundaries | **继承** v1.0 spec/design + 6 F 新增/调整 + 9 T + D6 spec 补丁 |
| 跨域耦合 | D3 内部混搭 bridge/bootstrap → CROSS 段 | D3 → D5 (metric) + D3 → D6 (probe) + D3 → D7 (event 复用) |
| 文件改动 | 0 代码 + 7 文档 | **0 物理迁移** + 8+ 代码 + 5 spec/registry 补丁 + D6 spec 补丁 |
| 风险等级 | 低（纯文档 + 注释） | 中（新增 metric / BREAKING 接口 / feature flag） |
| 父-子关系 | — | 父 DM-016 S5_Accepted → 子 DM-017 S2_Clarified |

---

## 9. 评审入口（供 S2 → S3 推进）

| 文档 | 用途 | 状态 |
|------|------|------|
| `demand.md` v0.2 | 需求澄清 SoT + Review R1 7 项决议 | ✅ S2_Clarified |
| `proposal.md` v0.1（本文件） | F1-F9 拆解 + 跨域责任分配 | ✅ S2 产物 |
| `tasks.md` v0.1 | v1.1 任务分解（无代码） | ⏳ Phase S2 末尾 |
| `openspec/specs/d3-llm-gateway/spec.md` v3.1.0 | 9 项 FR + Feature Flag 矩阵 | ⏳ S3 |
| `openspec/specs/d3-llm-gateway/design.md` v3.1.0 | F1-F9 时序 + Flag 配置 | ⏳ S3 |
| `openspec/specs/d3-llm-gateway/f-registry.md` v3.1.0 | 6 F 新增/调整 | ⏳ S3 |
| `openspec/specs/d3-llm-gateway/t-registry.md` v3.1.0 | 9 新 T | ⏳ S3 |
| `openspec/specs/d3-llm-gateway/span-registry.md` v3.1.0 | 1 metric + 1 span event | ⏳ S3 |
| `openspec/specs/architecture/cross-domain-boundaries.md` v1.1.0 | §2.4.3 D6-A 决议固化 | ⏳ S3 |
| `openspec/specs/d6-evolution/spec.md` 补丁 | probe #1 / #2 / #4 | ⏳ S3 |
| `review-r1.md` / `review-r2.md` / `review-r3.md` | S3-Gate 评审 | ⏳ S3-Gate |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：F1-F9 拆解 + 跨域责任分配 + D1-D7 决议引用 + 12 AC 验证 + 8 项 Out of Scope |
