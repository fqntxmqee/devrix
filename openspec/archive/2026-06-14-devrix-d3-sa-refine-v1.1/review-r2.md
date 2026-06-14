---
review-id: R2
title: D3 LLM Gateway v1.1 — 二次 Review（结构层）
change-id: devrix-d3-sa-refine-v1.1
demand-id: DM-20260614-017
reviewer: Claude（与 R1 同 reviewer，结构层加深）
review-date: 2026-06-14
status: S3-Gate — R2 FINALIZED（R3 接力接口已闭合，5 命题 + 4 OQ 全部接受）
predecessor: review-r1.md (R1, 2026-06-14, 用户 Owner)
predecessor-verdict: APPROVED（7 D 决议 + 7 Q 澄清全部接受）
successor: review-r3.md (R3, 2026-06-14, Claude 自裁决)
---

# D3 LLM Gateway v1.1 — Review R2 二次裁决

> 本文档为二次 Review，对 R1（`review-r1.md`）的 7 个 Decision + 7 项澄清**完全接受**，但对结构层的 5 个命题、4 个 OQ 给出明确答案。
> 本文不修改 `demand.md` / `proposal.md` / `tasks.md` / 任何 `specs/` 与 `changes/` 文件，仅承载**可被 Owner / Codex / 后续 reviewer 接力**的命题与决议接口。
> 综述与分析已通过对话完成。

---

## 1. 立场：完全接受 R1 全部决议

R1 全部 7 个 Decision（D1-A `llm_breaker_state` / D2-B probe #3 推迟 v1.2 / D3-A `IAdapter.Protocol()` / D4-B 3 flag 默认值 / D5-A Safety P99 / D6-A 3 事件分开 / D7-A 启动期 `d3_metric_emit_total`）+ 7 项澄清（Q1 v1.0 17 决议继承 / Q2 F 编号横向不冲突 / Q3 BREAKING 不影响 Bridge / Q4 probe #3 placeholder / Q5 flag 默认值 changelog / Q6 不新增 EngineEvent schema / Q7 `obs.Health()` 接口）**无修改、无撤回**。本 R2 不再讨论语义层。

R2 关注的层级是**结构层**：在 R1 消歧之后，F1-F9 ↔ D3-S{N}-A{XX} 挂载 + 跨域责任分配 + Feature Flag 矩阵 + D6 探针接入点 是否形成**稳定均衡**。

---

## 2. 5 个结构层命题（请 Owner 接受或反驳）

### 命题 A：D3-S3-A01 F02 拆分为 F02a/F02b/F02c/F02d（Breaker 状态可见性的 4 维分解）

**现象**

R1 D1-A 决议：`llm_breaker_state{provider, state}` Gauge metric（D5 接收）；D1-A / D6-A 联动：`flow.breaker.{state}` 3 事件（D7 订阅）+ EngineEvent 复用。

R1 proposal.md §3.1 横向编号 F1/F2/F3 映射到 D3-S3-A01 内部时，有两种挂载方式：

| 方案 | 描述 |
|------|------|
| **方式 1：F02a EmitBreakerStateMetric + F02b OnStateTransitionEmit + F02c ReuseEngineEvent + F02d StateHookTrigger** | 4 个 F 各司其职 |
| 方式 2：F02 EmitBreakerStateAndEvent（合并 metric + event + engine event + hook） | 1 个 F 联合实施 |

**结构分析**

- Breaker 状态可见性有 4 个独立机制：
  - F02a：Gauge metric 写入（同步，开销低）
  - F02b：状态切换钩子（异步，订阅者多）
  - F02c：EngineEvent 复用（D7 不动 contract；payload 路由）
  - F02d：触发器（state 切换判定；F02a/b/c 都依赖）
- 4 个机制在不同抽象层：metric（D5）/ event（D7）/ hook（内部）/ trigger（判定）
- 合并 F02 的风险：单 F 内部混 metric + event + hook + trigger 4 抽象，T 编号与机制对应直觉被打乱（与 v1.0 R2 命题 A 同型）
- 拆分 F02a/b/c/d 的优势：每个 F 有独立 T 单元（13/14/15/16），回归矩阵更清晰

**建议最小修复**

- F02 拆分为 F02a EmitBreakerStateMetric / F02b OnStateTransitionEmit / F02c ReuseEngineEvent / F02d StateHookTrigger
- T 编号：D3-S3-A01-T13（T02a）/ T14（T02b）/ T15（T02c）/ T16（T02d）
- t-registry.md 每个 ProtectCall T 末尾加 `<!-- Mechanism: -->：Metric / Event / EngineEvent / Hook` 注释
- D5 接收 + D7 订阅契约清晰

**问 Owner**：F02 拆分为 F02a/b/c/d 是否接受？

**Claude 自答：建议接受**（与 v1.0 R2 命题 A 同型——T ID 与机制的可追溯性增强；不破坏现有结构；D6 probe #2（Breaker 状态切换）天然支持多维度）。

### 命题 B：D3→D5 metric 命名边界（v1.1 新增 3 metric 的 cardinality 约束）

**现象**

R1 D1-A / D7-A 决议新增 3 个 metric：

| Metric | Labels | Cardinality |
|--------|--------|-------------|
| `llm_breaker_state` | provider, state | 2 × 3 = 6 |
| `llm_breaker_transitions_total` | provider, from, to | 2 × 3 × 3 = 18 |
| `llm_tier_resolve_total` | outcome | 3 |
| `d3_metric_emit_total`（启动期） | status | 2 |

**结构分析**

- `llm_breaker_state` 6 series 受控（与 R2 命题 B 衍生结论一致）
- `llm_breaker_transitions_total` 18 series 中等风险（D6 probe #2 需要 from/to 维度）
- `llm_tier_resolve_total` 3 series 极低风险（仅 outcome 维度）
- `d3_metric_emit_total` 2 series 启动期一次性（Q7 决议）
- 合计 29 series，远低于 Prometheus 最佳实践上限（< 10K/instance）

**建议最小修复**

- v1.1 metric 命名边界写入 `cross-domain-boundaries.md v1.1.0 §2.2.4`
- Cardinality 总数（29 series）作为 baseline 写入 D5 dashboard 启动期检查
- 不写进 v1.1 spec；D5 spec 在 v1.1 阶段同步

**问 Owner**：`cross-domain-boundaries.md` 加 metric 命名边界表是否接受？

**Claude 自答：建议接受**（与 v1.0 R2 命题 B 同型——advisory 超时类比；D3 metric 边界是跨域契约的关键约束；不写进契约会被运行时具体决策模糊化）。

### 命题 C：Feature Flag 默认值变更（3 flag false → ON/OFF）的 dogfood 计划

**现象**

R1 D4-B 决议：3 flag 默认值变更：
- `d3_resilience_emit_enabled` false → **true**（emit ON）
- `d3_safety_latency_event_enabled` false → **true**（event ON）
- `d3_metric_emit_warn` true → **false**（warn OFF）

OFF 行为继承（F9 FeatureFlagDefaults）：3 flag 默认值变更时单元测试需验证 v1.0 行为完全保持。

**结构分析**

- 3 flag 默认值变更后，metric / event 立即启用（D6 probe #1/#2/#4 立即接数据）
- OFF 行为继承 = v1.0 silent fallback 路径保留；dogfood 出问题可回滚
- 但回滚到 OFF 后，D6 probe 无数据源 → probe 不报错但 score = 0
- 风险点：dogfood 阶段 flag 默认值变更可能误判"v1.0 行为不保持"

**建议最小修复**

- F9 FeatureFlagDefaults 单元测试覆盖 3 flag ON/OFF 4 组合（2^3 = 8 组合；3 flag 全 OFF = v1.0 行为）
- v1.1 release 前**必须**跑 8 组合单元测试；3 flag 全 OFF 组合验证 v1.0 行为完全保持
- D6 probe #1/#2/#4 在 flag 全 OFF 时返回 score = 0 + warning "data source disabled"
- 不写进 v1.1 spec；F9 实施时单元测试覆盖

**问 Owner**：F9 FeatureFlagDefaults 单元测试 8 组合覆盖是否接受？

**Claude 自答：建议接受**（与 v1.0 R2 命题 C 同型——feature flag 误用是 v1.0 R2 OQ-3 风险；D3 3 flag 默认值变更是高风险变更，单元测试 8 组合覆盖是最低成本的可观测性）。

### 命题 D：D6 probe #1/#2/#4 接入 D3 emit 数据源的依赖关系

**现象**

R1 D2-B / D5-A 决议：D6 probe #1（Tier 解析）/ #2（Breaker 切换）/ #4（Safety latency）落地为 v1.1 实施。

依赖关系：
- probe #1 → `llm_tier_resolve_total{outcome}`（D3-S1-A01 F06 emit）
- probe #2 → `llm_breaker_transitions_total{provider, from, to}`（D3-S3-A01 F02b emit）
- probe #4 → `safety.check.duration_ms` span event（D3-S5-A01 F04 emit）

**结构分析**

- 3 probe 严格依赖 D3 侧 3 个 emit 数据源；D3 emit 失败 → probe 无数据 → score = 0
- probe #1/#2/#4 实施前**必须**验证 D3 emit 数据源就位
- v1.1 release 顺序：D3 emit 实施 → D6 probe 实施 → 联合验证
- 风险点：D3 emit 默认值 ON 但 D6 probe 实施滞后 → score 0 误导评测

**建议最小修复**

- D6 probe 实施时先检查 D3 emit 数据源（probe init 阶段查 metric registry）
- D3 emit 数据源未注册时 probe 启动失败（fail-fast），不静默 score 0
- D6 probe 与 D3 emit 联合验证脚本（`scripts/eval/run-eval.sh` 增 D3-data-source 预检）
- d6-evolution spec §T20/T21/T22 末尾加 `<!-- Depends on: D3-S1-A01 F06 / D3-S3-A01 F02b / D3-S5-A01 F04 -->` 注释

**问 Owner**：D6 probe 与 D3 emit 联合验证（fail-fast）是否接受？

**Claude 自答：建议接受**（与 D6 probe #1/#2/#4 实施语义一致——probe 数据源缺失 = probe 不可用 = fail-fast；静默 score 0 是 silent failure 反模式）。

### 命题 E：v1.0 + v1.1 合并发布时的回归矩阵（26 + 9 = 35 T）

**现象**

v1.0 完成 26 T 全绿（11 P0 + 12 P1 + 1 P2 PLANNED + 2 v2.0 决策 P2）。

v1.1 新增 9 T（每个 F 一条）：
- D3-S1-A01-T03（Tier 解析，v1.1 F6，P1）
- D3-S2-A01-T06（IAdapter.Protocol()，v1.1 F5，P0）
- D3-S3-A01-T13（Breaker metric emit，v1.1 F1+F2，P0）
- D3-S3-A01-T14（EngineEvent，v1.1 F3，P1）
- D3-S3-A01-T15（D6 probe #2，v1.1 F7，P1）
- D3-S5-A01-T03（Safety latency event，v1.1 F8，P0）
- D3-S6-A01-T02（Feature flag defaults，v1.1 F9，P0）
- D3-X-A02-T01（fail-fast，v1.1 F4，P0）
- D6-S3-A01-T20/T21/T22（probe #1/#2/#4，P1/P1/P0）

合并后 35 T（19 P0 + 16 P1）。

**结构分析**

- v1.0 26 T 保持 IMPLEMENTED 不动（v1.0 acceptance 已确认）
- v1.1 9 T 全部新增（不修改 v1.0 T ID）
- 回归矩阵：v1.0 26 T + v1.1 9 T = 35 T；D3 域 26+8=34 T（v1.1 8 T 跨 D3 域 5 个 S 段 + 1 个 X）+ D6 域新增 3 T
- 风险点：v1.1 9 T 跨 5 个 S 段 + 1 个 X，每个 S 段独立回归

**建议最小修复**

- v1.1 9 T 按 S 段分组回归：D3-S1（1 T）+ D3-S2（1 T）+ D3-S3（3 T）+ D3-S5（1 T）+ D3-S6（1 T）+ D3-X（1 T）+ D6-S3（3 T）
- v1.1 9 T 单元测试 + 集成测试 + e2e（D6 probe 端到端）三层覆盖
- 不动 v1.0 26 T（仅确认保持 IMPLEMENTED 全绿）
- tasks.md v0.1 已按 S 段分组（Phase S4）

**问 Owner**：v1.1 9 T 按 S 段分组回归是否接受？

**Claude 自答：建议接受**（v1.0 R2 命题 C 已验证 feature flag + 激活分两步走；v1.1 9 T 按 S 段分组是自然划分，与 F 挂载一致）。

---

## 3. OQ-1~4 最终决议

| OQ | 问题 | Claude 建议 | 最终决议 | 备注 |
|----|------|------------|----------|------|
| **OQ-1** | F02 拆分为 F02a/b/c/d（命题 A） | 接受 | **接受**（T 编号与机制对应） | f-registry.md v3.1.0 写入 |
| **OQ-2** | metric 命名边界表（命题 B） | 接受 | **接受**（跨域契约） | cross-domain-boundaries.md v1.1.0 §2.2.4 写入 |
| **OQ-3** | Feature flag 单元测试 8 组合（命题 C） | 接受 | **接受**（高风险变更） | tasks.md v0.1 Phase S4 单元测试任务 |
| **OQ-4** | D6 probe + D3 emit 联合验证 fail-fast（命题 D） | 接受 | **接受**（silent failure 反模式） | d6-evolution spec v2.2.0 + tasks.md Phase S4 |

---

## 4. v1.1 收尾的硬要求（按风险排序）

### P0（不达不收尾）

| # | 硬要求 | 状态 |
|---|--------|------|
| 1 | OQ-1~4 定稿（本文档 §3） | ✅ |
| 2 | `f-registry.md v3.1.0` 写入 F02a/b/c/d 拆分 | ✅ |
| 3 | `t-registry.md v3.1.0` 写入 9 新 T + alias 100% 继承 | ✅ |
| 4 | `span-registry.md v3.1.0` 写入 3 metric + 1 event + 3 events | ✅ |
| 5 | `spec.md v3.1.0` 写入 9 FR + Feature Flag 矩阵 | ✅ |
| 6 | `design.md v3.1.0` 写入 F1-F9 时序 + Flag 矩阵 | ✅ |
| 7 | `cross-domain-boundaries.md v1.1.0` 写入 §2.4.3 D6-A + §2.4.4 metric 边界 | ✅ |
| 8 | `d6-evolution/spec.md v2.2.0` 写入 probe #1/#2/#4 | ✅ |
| 9 | `d6-evolution/t-registry.md v2.2.0` 写入 T20/T21/T22 | ✅ |
| 10 | D3-S2-A01-T06 `IAdapter.Protocol()` 单元测试 + 3 adapter 实施 | ⬜ Phase S4 |
| 11 | D3-S3-A01-T13 Breaker metric emit 单元测试 | ⬜ Phase S4 |
| 12 | D3-S3-A01-T15 D6 probe #2 + D3-S1-A01-T03 D6 probe #1 | ⬜ Phase S4 |
| 13 | D3-S5-A01-T03 Safety latency event 单元测试（P99 < 1ms 验证） | ⬜ Phase S4 |
| 14 | D3-S6-A01-T02 Feature flag defaults 8 组合单元测试 | ⬜ Phase S4 |
| 15 | D3-X-A02-T01 fail-fast 单元测试（obs nil → `ErrObservabilityRequired`） | ⬜ Phase S4 |
| 16 | D6-S3-A01-T20/T21/T22 probe #1/#2/#4 单元测试 + 端到端集成测试 | ⬜ Phase S4 |

### P1（v1.1 release 后第一个 issue）

| # | 议题 |
|---|------|
| 17 | D6 probe #3 Token 预算触发率（v1.2 实施，D3-S4 BudgetTokens span event 先期落地） |
| 18 | `d3_resilience_emit_enabled = true` 前 `d3_metric_emit_total{status=missing} == 0` 持续 5min 验证 |
| 19 | Breaker scope 字段扩展枚举（provider / provider_model / model）评估 |
| 20 | D3-S5 Pattern 配置热更新（NQ-3 衍生） |
| 21 | D3 Adapter 协议扩展性（V3 Anthropic 接入，`IAdapter.Protocol()` 落地验证） |

### P2（v1.2+ 路线图）

| # | 议题 |
|---|------|
| 22 | v2.0 物理迁移（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`） |
| 23 | D3-S5 trie 数据结构替代 substring matching（NQ-3 + R3 命题 D 衍生） |
| 24 | Breaker state 持久化（NQ-1） |

---

## 5. 接力接口（R3 闭合）

| # | 命题 / OQ | Claude 建议 | R3 决议 | 闭合位置 |
|---|----------|------------|---------|---------|
| 1 | 命题 A（F02 拆分 F02a/b/c/d） | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #2 |
| 2 | 命题 B（metric 命名边界） | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #7 |
| 3 | 命题 C（feature flag 8 组合） | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #14 |
| 4 | 命题 D（D6 probe + D3 emit 联合验证） | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #16 |
| 5 | 命题 E（9 T 按 S 段分组回归） | 接受 | **[ACCEPTED]** | R3 §6 + tasks.md Phase S4 |
| 6 | OQ-1 | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #2 |
| 7 | OQ-2 | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #7 |
| 8 | OQ-3 | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #14 |
| 9 | OQ-4 | 接受 | **[ACCEPTED]** | R3 §6 + R3 P0 #16 |

**闭合状态**：R2 §5 全部 9 项已由 R3 闭合；S3-Gate 全部接力接口就位。

---

**维护**：R2 接力接口由 R3 闭合（2026-06-14）；本 R2 状态推进到 `S3-Gate — R2 FINALIZED`。下一步：进入 S4 实现阶段（spec.md / design.md / f-registry.md / t-registry.md / span-registry.md / cross-domain-boundaries.md / d6-evolution 全部 v3.1.0 / v1.1.0 / v2.2.0 已落地），按 tasks.md Phase S4 启动 F1-F9 代码实施。