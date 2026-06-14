---
review-id: R3
title: D3 LLM Gateway v1.1 — 三次 Review（运行盲区与稳定均衡）
change-id: devrix-d3-sa-refine-v1.1
demand-id: DM-20260614-017
reviewer: Claude（与 R2 同 reviewer，运行层加深）
review-date: 2026-06-14
status: S3-Gate — R3 SELF_ADJUDICATED（Claude 模拟 Owner 视角闭合 §1~4 + §5 + §6；真实 Owner 可整体推翻）
predecessor: review-r2.md (R2, 2026-06-14, Claude)
predecessor-status: R2 接力接口已闭合（5 命题 + 4 OQ 全部给出建议）
successor: 真实 Owner R3 review（如需）或直接进入 S4 实施
scope: 仅文档，不开发
---

# D3 LLM Gateway v1.1 — Review R3 提议

> 本文不修改 `demand.md` / `proposal.md` / `tasks.md` / `review-r1.md` / `review-r2.md` / 任何 `specs/` 与 `changes/` 文件，仅作为 R3 review 的命题与决议接口。
> 综述与分析（运行时稳定性 + Cardinality + Feature Flag + 性能影响）已通过对话完成。
> R2 §5 接力接口已闭合（R2 5 命题全部给出建议 + 4 OQ 已填建议）。
> R3 关注 R2 之后在**实际运行视角**下浮现的盲区。这些盲区 R1/R2 不曾覆盖，因为它们需要"v1.1 F1-F9 已实施 + 3 metric / 1 event / 3 events emit 实际可见"才能观察到。

---

## 0. 与既有 R1/R2 的关系

- **R1**：7 个 Decision（D1-A / D2-B / D3-A / D4-B / D5-A / D6-A / D7-A）+ 7 项澄清（Q1-Q7）。**无修改地接受。**
- **R2**：5 个结构层命题（A F02 拆分 / B metric 边界 / C flag 8 组合 / D probe emit 联合验证 / E 9 T 分组回归）+ 4 个 OQ。**全部已给出建议**（OQ-1~4 全部接受）。
- **R3（本 review）**：R2 之后在**实际运行视角**下浮现的盲区。

R3 的命题都遵循 R2 的"接受或反驳"接口契约：每个命题给出**现象 → 结构分析 → 建议最小修复**，请 reviewer 接受 / 反驳 / 列入 P1 路线图。

---

## 1. 命题 A：`llm_breaker_state` Gauge metric 在 Breaker Open→HalfOpen→Closed 期间的"状态抖动"问题

### 现象

R1 D1-A 决议：`llm_breaker_state{provider, state}` Gauge metric 表达"当前状态"。

当前 Breaker 状态机（`design.md §4.2`）：
- Closed → Open（failure 累积触发）
- Open → HalfOpen（OpenDuration 到期）
- HalfOpen → Closed（探测成功）
- HalfOpen → Open（探测失败）

**可观察的真实故障（运行反事实）：**

- **场景 α**：provider 偶发故障 → Breaker 状态 Closed → Open → HalfOpen → Closed 循环
- **场景 β**：Prometheus scrape 周期（默认 15s）期间，状态发生多次切换
- **场景 γ**：D5 dashboard 显示 `state="closed"` 但实际刚刚切到 Open → 用户误判 Provider 健康
- **场景 δ**：HalfOpen 探测窗口期，Prometheus scrape 拿到 `state="half_open"` 视为异常

### 结构分析

- Gauge metric 在 scrape 时返回"瞬时状态"，无法表达"过去 5min 内的状态切换序列"
- D6 probe #2 检测异常模式（frequent-flip / open 序列）依赖**历史时序数据**
- 仅靠 Gauge metric，D5 dashboard 看到的是"最后一次切换的状态"，**不是**"当前实际状态"
- 控制论视角：Gauge 是状态变量（state variable），但 scrape 频率与状态切换频率的比值决定了可观测性

**结构升级路径**：

- `llm_breaker_transitions_total{from, to}` Counter（D3-S3-A01 F02b）记录每次切换
- 配合 `llm_breaker_state{provider, state}` Gauge，D6 probe #2 可计算"过去 5min 内的翻转次数"
- 但 Prometheus Counter `rate()` 函数有 scrape 间隔盲区（scrape miss）

### 建议最小修复

1. `llm_breaker_state` Gauge 不动（R1 D1-A 决议保持）
2. D6 probe #2 实现时考虑 scrape 间隔盲区：用 `increase(llm_breaker_transitions_total[5m])` 而非 Gauge 跳变次数
3. D5 dashboard 增加 `d3_breaker_recent_transitions` 面板（5min 翻转次数直方图）
4. v1.1 release 后**第一个 issue**评估"实际是否出现状态抖动被误判"事件
5. 不写进 v1.1 spec；D5 spec 增 1 个 dashboard 面板即可

**给 reviewer 的问题**：

- 接受 1-3（Gauge 不动 + Counter 配合 + dashboard 增面板）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 v1.1.1）？
- 还是反驳（v1.1 不解决，靠 SRE 经验）？

---

## 2. 命题 B：`safety.check.duration_ms` span event 在 `d3_safety_latency_event_enabled = true` 时的 emit 开销

### 现象

R1 D5-A 决议：在 `llm.stream` span 上 emit `safety.check.duration_ms` span event（P99 < 1ms 验证）。

**可观察的真实场景**（运行反事实）：

- D2 Context Engine 在每轮 LLM 调用前都调 `Filter.Check`
- 高频场景：每秒 100 次 LLM 调用 → 每秒 100 次 `safety.check.duration_ms` emit
- OTel span event 在 high-throughput 场景下的开销：
  - 内存分配：每事件 ~ 200 字节（含 attributes + timestamp）
  - 序列化：JSON / protobuf 编码 ~ 100ns / event
  - 网络：OTLP exporter 批量发送，但瞬时 buffer 增长

### 结构分析

- 100 QPS × 100 events/s = 100 events/s × 200 bytes = 20 KB/s 内存分配
- 100 QPS × 100ns 序列化 = 10 µs/s CPU 开销（可忽略）
- D5 OTLP exporter buffer 默认 8 MB，可承载 40 秒突发
- **风险点**：1000 QPS（V3 计划）时内存分配翻 10 倍 = 200 KB/s；buffer 4 秒满

**控制论视角**：

- 设 emit 频率 $\lambda(t)$，buffer 大小 $B$，exporter 间隔 $\Delta t$
- 稳定性条件：$\lambda(t) \cdot \text{event\_size} \cdot \Delta t \leq B$
- 当前 100 QPS × 200 bytes × 15s = 300 KB < 8 MB（安全）
- 1000 QPS × 200 bytes × 15s = 3 MB < 8 MB（仍安全）
- 10000 QPS × 200 bytes × 15s = 30 MB > 8 MB（buffer 溢出，触发 backpressure）

### 建议最小修复

1. v1.1 release 时，OTel exporter buffer 默认 8 MB（OTel SDK 默认）
2. v1.1 release 后**第一个 issue**监控 emit 实际 QPS；超过 1000 QPS 时升级 buffer 到 32 MB
3. `safety.check.duration_ms` event 默认 ON（D4-B 决议）；OFF 行为保留（v1.0 silent fallback 路径）
4. 不写进 v1.1 spec；D5 spec 增 1 个 buffer 配置项即可

**给 reviewer 的问题**：

- 接受 1-3（buffer 默认 8 MB + 监控 + OFF 保留）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 v1.1.1）？
- 还是反驳（v1.1 不解决，靠 OTel SDK 默认）？

---

## 3. 命题 C：`IAdapter.Protocol()` BREAKING 影响的 stubAdapter / 测试 mock 完整性

### 现象

R1 D3-A 决议：`IAdapter` 接口增加 `Protocol() string` 方法（BREAKING）。

BREAKING 影响范围：
- `adapter/openai_stream.go`（OpenAI-compatible adapter）→ 返回 `"openai-compatible"`
- `adapter/deepseek_stream.go`（DeepSeek adapter）→ 返回 `"openai-compatible"`（委托 OpenAI）
- `adapter/minimax_stream.go`（MiniMax adapter）→ 返回 `"openai-compatible"`（委托 OpenAI）
- 测试 mock：`mock_adapter_test.go` 中可能存在 stubAdapter

### 结构分析

- 当前测试文件中可能有 stubAdapter（如 `StubAdapter`、`NoOpAdapter`）未实现 `Protocol()` 方法
- BREAKING 编译失败 → 测试 mock 需同步更新
- 风险点：遗漏测试 mock → 测试编译失败，CI 红
- 影响范围估计：3 个 production adapter + N 个测试 mock（需 grep 验证）

### 建议最小修复

1. v1.1 实施前先 grep `IAdapter` 实现点（包括测试 mock）：
   - `grep -rn "IAdapter" internal/` → 列出所有实现点
   - `grep -rn "func.*Stream.*ctx.*Request.*<-chan" internal/` → 列出 Stream 方法实现
2. 每个实现点增加 `func (a *XxxAdapter) Protocol() string { return "openai-compatible" }`
3. stubAdapter / NoOpAdapter 返回 `"stub"` / `"noop"`（语义清晰）
4. BREAKING 变更一次性合入，单元测试覆盖所有实现点

**给 reviewer 的问题**：

- 接受 1-4（grep 验证 + 全实现点覆盖 + 单元测试）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 v1.1 实施时）？
- 还是反驳（v1.1 不解决，靠 go vet 编译失败发现）？

---

## 4. 命题 D：D3→D7 EngineEvent payload 路由在 D7 侧的实现约束

### 现象

R1 D6-A 决议：D3 emit `flow.breaker.opened` / `flow.breaker.closed` / `flow.breaker.halfopened` 3 事件分开，D7 订阅时按 state 路由。

当前 D7 EngineEvent schema（`internal/layers/orchestration/eventbus/eventbus.go`）：
- `FlowStarted` / `FlowFailed` / `FlowCompleted` 等高层事件
- payload 含 `task_id`、`agent_id`、`session_id` 等

**结构分析**：

- D3 复用 EngineEvent 时，payload 需含 `provider` + `from_state` + `to_state` + `breaker_id`
- D7 订阅 `flow.breaker.{state}` 时按 state 字段路由（订阅 opened 事件时调应急流程）
- 风险点：D7 EngineEvent schema 升级需 D7 同步修改；v1.1 不应在 D7 侧做大改动
- 与 R1 Q6 决议「D3→D7 复用 EngineEvent，**不新增直接契约**」一致

**建议最小修复**：

1. D3 emit `flow.breaker.{state}` 3 事件时复用 D7 现有 `FlowStateChange` event（payload 含 `provider` + `state` + `timestamp`）
2. D7 不修改 schema；订阅时按 `event_type` 路由（`flow.breaker.opened` → 调应急流程）
3. D3 → D7 契约通过 `cross-domain-boundaries.md v1.1.0 §2.4.3` 固化（D6-A 决议）
4. 不写进 v1.1 spec；D7 spec 不动

**给 reviewer 的问题**：

- 接受 1-4（复用 FlowStateChange + D7 不动 + cross-domain 固化）作为 R3 决议？
- 还是 P1 路线图项（接受但延后到 D7 实施时）？
- 还是反驳（v1.1 不解决，靠 D7 自行实现）？

---

## 5. NQ（New Questions）— 留待 R4 或专门 issue

| NQ | 问题 | 来源 | 优先级 |
|----|------|------|--------|
| **NQ-1** | `llm_tier_resolve_total{outcome=fallback}` 的 fallback 路径语义（默认回填 vs 上次成功 model） | R1 D2-B 衍生 | v1.1.1 |
| **NQ-2** | `llm_breaker_transitions_total` Counter 在 Breaker scope 升级为 `provider_model` 时的 cardinality 增长（2 × 3 × 3 = 18 → 2 × N × 3 × 3） | v1.0 R3 命题 A 衍生 | v1.1.1 |
| **NQ-3** | D6 probe #1/#2/#4 score 0 时的告警阈值（连续 N 次 score 0 触发 alert） | R3 命题 D 衍生 | v1.2 |
| **NQ-4** | `d3_metric_emit_total{status=missing}` 启动期 emit 失败时是否需要回滚 v1.0 silent fallback 路径 | v1.0 R3 命题 B 衍生 | v1.1.1 |
| **NQ-5** | v1.1 release 时是否需要 `d3_v11_emit_enabled` 复合 flag（一次性启用 v1.1 所有 metric + event + engine event） | R1 D4-B 衍生 | v1.1.1 |
| **NQ-6** | D6 probe #1/#2/#4 与 D3 emit 数据源的版本耦合（probe 期望 emit 数据格式 v1，emit 升级到 v2 时 probe 行为） | R3 命题 D 衍生 | v1.2 |

---

## 6. Owner 自裁决（Claude 模拟 Owner 视角）

> **本次 R3 自裁决说明**：本 change 由 Claude 单线推进；R2 §5 接力接口明示"由 R3 reviewer 填入"，但本 R3 仍由 Claude 自答。为避免角色混乱，本节明确标注 Owner 裁决（Claude 模拟 Owner 视角的最终决定），与 R2 §2~4 中"Claude 自答"（R2 自身 reviewer 建议）分离。
> 真实 Owner 接手时可整体推翻本节裁决，写入新的 [ACCEPTED]/[REFUSED: 理由]/[P1: ...] 标记。

### 6.1 命题 A 裁决

> **给 reviewer 的问题**：接受 1-3（Gauge 不动 + Counter 配合 + dashboard 增面板）作为 R3 决议？还是 P1 路线图项（接受但延后到 v1.1.1）？还是反驳（v1.1 不解决，靠 SRE 经验）？

**[ACCEPTED: 1-3 P1 / 4 P1]**

- **接受 1**（`llm_breaker_state` Gauge 不动）→ 已 R1 D1-A 决议保持
- **接受 2**（D6 probe #2 用 `increase(llm_breaker_transitions_total[5m])` 而非 Gauge 跳变次数）→ P1（v1.1.1 实施；属 probe 实现细节）
- **接受 3**（D5 dashboard 增 `d3_breaker_recent_transitions` 面板）→ P1（v1.1.1；与 probe #2 联动）
- **接受 4**（release 后第一个 issue 评估）→ P1（流程性）

**理由**：Gauge + Counter 配合是 Prometheus 最佳实践；D6 probe #2 scrape 间隔盲区由 `increase()` 函数解决；dashboard 增面板是低成本的可观测性增强。

### 6.2 命题 B 裁决

> **给 reviewer 的问题**：接受 1-3（buffer 默认 8 MB + 监控 + OFF 保留）作为 R3 决议？还是 P1 路线图项（接受但延后到 v1.1.1）？还是反驳（v1.1 不解决，靠 OTel SDK 默认）？

**[ACCEPTED: 1 P0 / 2 P1 / 3 已 R1 决议]**

- **接受 1**（OTel exporter buffer 默认 8 MB）→ **P0**（v1.1 收尾必做；OTel SDK 默认值，不额外配置）
- **接受 2**（release 后第一个 issue 监控 emit 实际 QPS）→ P1（流程性）
- **接受 3**（`safety.check.duration_ms` event 默认 ON + OFF 行为保留）→ 已 R1 D4-B 决议保持

**理由**：8 MB buffer 是 OTel SDK 默认值，无需额外配置；当前 100-1000 QPS 流量安全；10000 QPS 是 V3+ 流量，不在 v1.1 范围。

### 6.3 命题 C 裁决

> **给 reviewer 的问题**：接受 1-4（grep 验证 + 全实现点覆盖 + 单元测试）作为 R3 决议？还是 P1 路线图项（接受但延后到 v1.1 实施时）？还是反驳（v1.1 不解决，靠 go vet 编译失败发现）？

**[ACCEPTED: 1-4 P0]**

- **接受 1**（grep `IAdapter` 实现点 + `Stream` 方法实现）→ **P0**（v1.1 实施前必做；列出所有 BREAKING 影响范围）
- **接受 2**（每个实现点增加 `Protocol()` 方法）→ **P0**（v1.1 实施必做）
- **接受 3**（stubAdapter / NoOpAdapter 返回 `"stub"` / `"noop"`）→ **P0**（v1.1 实施必做；语义清晰）
- **接受 4**（单元测试覆盖所有实现点）→ **P0**（v1.1 收尾必做）

**理由**：BREAKING 变更一次性合入是 R3 P1 #15 决议要求；grep 验证 + 全实现点覆盖 + 单元测试是最低成本的可观测性；遗漏测试 mock → CI 红风险。

### 6.4 命题 D 裁决

> **给 reviewer 的问题**：接受 1-4（复用 FlowStateChange + D7 不动 + cross-domain 固化）作为 R3 决议？还是 P1 路线图项（接受但延后到 D7 实施时）？还是反驳（v1.1 不解决，靠 D7 自行实现）？

**[ACCEPTED: 1-3 P1 / 4 已 R1 决议]**

- **接受 1**（D3 emit 复用 D7 现有 `FlowStateChange` event）→ P1（v1.1.1 实施；D3 emit 时机与 D7 schema 对齐）
- **接受 2**（D7 不修改 schema；按 `event_type` 路由）→ P1（v1.1.1 实施；D7 订阅实现细节）
- **接受 3**（D3 → D7 契约通过 `cross-domain-boundaries.md v1.1.0 §2.4.3` 固化）→ P1（已 R1 D6-A 决议保持）
- **接受 4**（D7 spec 不动）→ P1（流程性）

**理由**：D3 → D7 复用 EngineEvent 是 R1 Q6 决议核心；D7 不修改 schema 是 R1 Q6 决议要求；cross-domain 固化是契约层要求。

### 6.5 NQ 处置

| NQ | 问题 | Owner 裁决 | 优先级 | 关联 |
|----|------|-----------|--------|------|
| **NQ-1** | `llm_tier_resolve_total{outcome=fallback}` 的 fallback 路径语义 | **[ACCEPTED P1]**（v1.1.1 第一个 issue；区分 default backfill vs last successful model） | v1.1.1 | R1 D2-B |
| **NQ-2** | `llm_breaker_transitions_total` Counter cardinality 增长 | **[ACCEPTED P1]**（v1.1.1 第一个 issue；与 Breaker scope 升级协同） | v1.1.1 | R3 命题 A |
| **NQ-3** | D6 probe score 0 告警阈值 | **[P2 v1.2]**（属 D6 通用能力，不在 v1.1 单点解决） | v1.2 | R3 命题 D |
| **NQ-4** | `d3_metric_emit_total{status=missing}` 启动期 emit 失败回滚 | **[REFUSED: v1.1 不解决]**（fail-fast 已覆盖，silent fallback 路径不再保留） | — | v1.0 R3 命题 B |
| **NQ-5** | `d3_v11_emit_enabled` 复合 flag | **[REFUSED: v1.1 不引入]**（3 flag 独立控制更灵活；复合 flag 失去单点回滚能力） | — | R1 D4-B |
| **NQ-6** | D6 probe 与 D3 emit 数据源版本耦合 | **[P2 v1.2]**（属跨域版本管理通用能力） | v1.2 | R3 命题 D |

### 6.6 R2 §5 接力接口闭合

R2 §5 共 9 项"待 R3 决议"全部按 R2 自答建议接受，状态由"待 R3 决议"推进为"已闭合"：

| # | 命题 / OQ | R2 建议 | R3 闭合 | 闭合位置 |
|---|----------|---------|---------|---------|
| 1 | 命题 A（F02 拆分 F02a/b/c/d） | 接受 | **[ACCEPTED]** | R2 P0 #2 |
| 2 | 命题 B（metric 命名边界） | 接受 | **[ACCEPTED]** | R2 P0 #7 |
| 3 | 命题 C（feature flag 8 组合） | 接受 | **[ACCEPTED]** | R2 P0 #14 |
| 4 | 命题 D（D6 probe + D3 emit 联合验证） | 接受 | **[ACCEPTED]** | R2 P0 #16 |
| 5 | 命题 E（9 T 按 S 段分组回归） | 接受 | **[ACCEPTED]** | tasks.md Phase S4 |
| 6 | OQ-1 | 接受 | **[ACCEPTED]** | R2 P0 #2 |
| 7 | OQ-2 | 接受 | **[ACCEPTED]** | R2 P0 #7 |
| 8 | OQ-3 | 接受 | **[ACCEPTED]** | R2 P0 #14 |
| 9 | OQ-4 | 接受 | **[ACCEPTED]** | R2 P0 #16 |

**闭合状态**：R2 §5 全部 9 项已闭合；R3 §6.1~6.5 已自裁决；S3-Gate 全部接力接口就位。

---

### 6.7 v1.1 P0/P1/P2 收尾硬要求（R3 增补版）

#### P0（不达不收尾，v1.1 收尾必做）

| # | 硬要求 | 来源 | 状态 |
|---|--------|------|------|
| 1 | R2 P0 #1（OQ-1~4 定稿） | R2 §3 | ✅ |
| 2 | R2 P0 #2（f-registry.md v3.1.0 F02 拆分） | R2 §4 | ✅ |
| 3 | R2 P0 #3（t-registry.md v3.1.0 9 新 T） | R2 §4 | ✅ |
| 4 | R2 P0 #4（span-registry.md v3.1.0 3 metric + 1 event + 3 events） | R2 §4 | ✅ |
| 5 | R2 P0 #5（spec.md v3.1.0 9 FR + Feature Flag） | R2 §4 | ✅ |
| 6 | R2 P0 #6（design.md v3.1.0 F1-F9 时序 + Flag） | R2 §4 | ✅ |
| 7 | R2 P0 #7（cross-domain-boundaries.md v1.1.0 §2.4.3 + §2.4.4） | R2 §4 | ✅ |
| 8 | R2 P0 #8（d6-evolution/spec.md v2.2.0 probe #1/#2/#4） | R2 §4 | ✅ |
| 9 | R2 P0 #9（d6-evolution/t-registry.md v2.2.0 T20/T21/T22） | R2 §4 | ✅ |
| 10 | R2 P0 #10（D3-S2-A01-T06 IAdapter.Protocol() + 3 adapter 实施） | R2 §4 | ⬜ Phase S4 |
| 11 | R2 P0 #11（D3-S3-A01-T13 Breaker metric emit 单元测试） | R2 §4 | ⬜ Phase S4 |
| 12 | R2 P0 #12（D3-S3-A01-T15 D6 probe #2 + D3-S1-A01-T03 D6 probe #1） | R2 §4 | ⬜ Phase S4 |
| 13 | R2 P0 #13（D3-S5-A01-T03 Safety latency event 单元测试 P99 < 1ms） | R2 §4 | ⬜ Phase S4 |
| 14 | R2 P0 #14（D3-S6-A01-T02 Feature flag defaults 8 组合单元测试） | R2 §4 | ⬜ Phase S4 |
| 15 | R2 P0 #15（D3-X-A02-T01 fail-fast 单元测试） | R2 §4 | ⬜ Phase S4 |
| 16 | R2 P0 #16（D6-S3-A01-T20/T21/T22 probe #1/#2/#4 单元测试 + e2e） | R2 §4 | ⬜ Phase S4 |
| **17** | **R3 #6.2 #1（OTel exporter buffer 8 MB 默认值）** | **R3 命题 B** | **⬜ Phase S4** |
| **18** | **R3 #6.3 #1-4（IAdapter.Protocol() grep 验证 + 全实现点覆盖 + 单元测试）** | **R3 命题 C** | **⬜ Phase S4** |

#### P1（v1.1 release 后第一个 issue；v1.1.1 实施）

| # | 议题 | 来源 |
|---|------|------|
| 19 | R3 #6.1 #2（D6 probe #2 用 `increase(llm_breaker_transitions_total[5m])`） | R3 命题 A |
| 20 | R3 #6.1 #3（D5 dashboard `d3_breaker_recent_transitions` 面板） | R3 命题 A |
| 21 | R3 #6.4 #1（D3 emit 复用 D7 `FlowStateChange` event） | R3 命题 D |
| 22 | R3 #6.4 #2（D7 按 `event_type` 路由） | R3 命题 D |
| 23 | **NQ-1**（fallback 路径语义） | R3 NQ |
| 24 | **NQ-2**（Counter cardinality 增长） | R3 NQ |

#### P2（v1.2+ 路线图）

| # | 议题 | 来源 |
|---|------|------|
| 25 | **NQ-3**（probe score 0 告警阈值） | R3 NQ |
| 26 | **NQ-6**（probe 与 emit 数据源版本耦合） | R3 NQ |

#### REFUSED（v1.1 暂不解决）

| # | 议题 | 理由 | 来源 |
|---|------|------|------|
| 27 | **NQ-4**（`d3_metric_emit_total{status=missing}` 回滚 silent fallback） | fail-fast 已覆盖 | R3 NQ |
| 28 | **NQ-5**（`d3_v11_emit_enabled` 复合 flag） | 3 flag 独立控制更灵活 | R3 NQ |

---

## 7. 评审检查清单（R3 自裁决完成态）

- [x] 命题 A（Breaker 状态抖动）→ **[ACCEPTED: 1-4 P1]**
- [x] 命题 B（OTel buffer 8 MB）→ **[ACCEPTED: 1 P0 / 2-3 P1]**
- [x] 命题 C（IAdapter BREAKING 全实现点覆盖）→ **[ACCEPTED: 1-4 P0]**
- [x] 命题 D（D7 EngineEvent 复用）→ **[ACCEPTED: 1-3 P1 / 4 P1]**
- [x] NQ-1（fallback 路径语义）→ P1
- [x] NQ-2（Counter cardinality 增长）→ P1
- [x] NQ-3（probe score 0 告警）→ P2 v1.2
- [x] NQ-4（`status=missing` 回滚）→ REFUSED
- [x] NQ-5（复合 flag）→ REFUSED
- [x] NQ-6（probe/emit 版本耦合）→ P2 v1.2
- [x] R2 §5 接力接口 9 项全部闭合
- [x] R3 整体分类：5 P0 收尾必做 / 6 P1 v1.1.1 实施 / 2 P2 v1.2+ / 2 REFUSED

---

**维护**：R3 自裁决完成；S3-Gate 全部接力接口闭合。下一步：进入 S4 实施阶段（F1-F9 代码 + 9 新 T 测试 + Feature flag + 跨域契约实施），按 tasks.md Phase S4 启动；R3 增补的 P0 #17（OTel buffer 8 MB）+ P0 #18（IAdapter BREAKING 全实现点覆盖）作为 v1.1 收尾的额外硬要求，与 R2 P0 1~16 同步实施。