---
demand-id: DM-20260708-003
title: D7 Observe-LLM 5 场景输入输出协议沉淀
priority: P1
status: S2_Proposal
dsaft_domain: orchestration
created: 2026-07-08
parent_change: devrix-d7-observational-fastpath
parent_demand: DM-20260706-011
origin: |
  从 devrix-d7-observational-fastpath (DM-20260706-011) S5_Accepted 后的
  trace 验证需求。fast-path 闭环后,用户需要能"逐场景验证" Observe 节点
  与 LLM 的输入输出协议契约 — 4 种 kind + 1 种混合场景 (fact+uncertainty)。
  现有 spec/d7-observational-fastpath-spec.md 覆盖了 5 个 fast-path 场景,但
  **未显式定义 Observe↔LLM 帧级 I/O 协议**(6/11 字段过滤规则 / 4-kind JSON
  schema / Go-side 兜底规则 / 混合场景的 hasObsUncertainty 阻断机制)。
---

# D7 Observe-LLM 5 场景输入输出协议沉淀

## 1. 背景

D7 Observe 节点是 MUPS 5 节点流水线的第 1 节点,负责把用户 directive + 结构化 signal 转成
**类型化的 Obs* 提案**(ObservationProposal[])。下游 Plan / fast-path / Learn 都消费这份产出。

现有契约散落在:
- `internal/layers/contextengine/i18n/format_hints_mups.go:21-55` — i18n appendix (LLM 角色 + JSON schema)
- `internal/layers/orchestration/sessionorchestrator/observation_proposer.go:67-89` — observeLLMFieldMap (6/11 字段过滤)
- `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go:126-164` — parseObservationProposalsJSON + mapRawObsProposals
- `internal/layers/orchestration/sessionorchestrator/observation_proposer.go:178-235` — validateOneProposal (Go-side 兜底)
- `internal/layers/orchestration/sessionorchestrator/deliverable_execute.go:177-209` — fast-path 闸门
- `internal/layers/orchestration/orchtypes/observation.go` — 4 ObservationKind + Payload sealed interface

**契约** = 1 份协议规范(给三方 reviewer / future maintainer / 用户验证用)+ 16 个 trace test(运行验证)。

## 2. 沉淀目标

产出 1 份 spec doc,**显式**回答以下问题:

| 维度 | 问题 |
|---|---|
| 输入 | LLM 实际看到哪 6/11 个字段?被过滤的 5 个字段理由是什么? |
| 输出 | 4 种 kind 的 JSON schema 是什么?每种 kind 的 payload 字段约束? |
| 兜底 | Go-side validateOneProposal 做了什么?强度 cap / 零保护 / evidence 注入? |
| 路由 | LLM 输出 → []Observation → UncertaintyReport.Partition() 怎么分 Business/Anomalies? |
| 场景 1 | 纯确定性:directive 是直接 Q&A → ObsFact 路径 |
| 场景 2 | 纯不确定性:directive 模糊 → ObsUncertainty 路径 |
| 场景 3 | 结构化 signal:有量化指标 → ObsSignal 路径 |
| 场景 4 | 异常检测:有 delta → ObsDeviation + CatSystem 路径 |
| 场景 5 (混合) | fact + uncertainty 同时出现 → fast-path 被 hasObsUncertainty 阻断 |

## 3. 验收标准

| ID | 标准 | 优先级 | 验证方式 |
|----|------|--------|---------|
| AC1 | spec 文档覆盖 5 场景, 每场景含 ① 输入协议 ② 期望 LLM 输出 ③ Go 侧处理 ④ 最终路由 | P0 | review |
| AC2 | 11→6 字段过滤规则 + 5 个被过滤字段理由明确文档化 | P0 | review |
| AC3 | 4 种 kind 的 Payload 字段约束 (Statement/Question/Metric/Name+Value+Threshold) 文档化 | P0 | review |
| AC4 | 混合场景 (fact+uncertainty) 显式标注 fast-path 被阻断, 引用 item_pipeline.go:301 闸门 | P0 | review |
| AC5 | 16 个 trace test 全部 PASS (含 TestObserveTraceE2E_FactPlusUncertainty_FastPathBlocked 混合场景) | P0 | go test |
| AC6 | trace test 的 stdout 输出本身就是协议契约的"运行验证" | P1 | manual review |
| AC7 | spec 与 PR #472 trace test 一一对应, 通过文件路径引用 | P1 | grep |

## 4. 依赖与约束

| 类型 | 内容 |
|---|---|
| 依赖 | devrix-d7-observational-fastpath (DM-20260706-011, S5_Accepted) |
| 依赖 | PR #472 (16 trace test, 含混合场景) |
| 依赖 | PR #470/#471 (D7/D1 fast-path task_incomplete bypass) |
| 约束 | 不修改任何源代码 — 纯 spec 沉淀 + test 补充 |
| 约束 | spec 章节锚定 file:line (与现有 d7-observational-fastpath-spec.md 风格一致) |
| 约束 | 不重复 d7-observational-fastpath-spec.md 已覆盖的 fast-path 闸门契约 |

## 5. 变更范围

### 新增

| 路径 | 描述 |
|------|------|
| `openspec/changes/devrix-d7-observe-llm-protocol-doc/specs/d7-orchestration/d7-observe-llm-io-protocol-spec.md` | 主 spec 文档 (5 场景 I/O 协议) |
| `internal/layers/orchestration/sessionorchestrator/observe_trace_e2e_test.go` (MODIFIED) | +1 test (TestObserveTraceE2E_FactPlusUncertainty_FastPathBlocked) |

### 不变更

- `internal/layers/orchestration/sessionorchestrator/observation_proposer.go` — 0 修改
- `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go` — 0 修改
- `internal/layers/contextengine/i18n/format_hints_mups.go` — 0 修改
- `openspec/specs/d7-orchestration/spec.md` — 0 修改(spec doc 在本 change 内, 不合并到主 spec)

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| spec 与实现漂移 | High | trace test stdout 是 spec 的"活验证", 任何 field/scenario 不一致都会被 test 暴露 |
| 字段过滤规则变更 (11→6 变化) | Medium | Test #1 (OnlyFieldsVisibleToLLM) 锁死, 改 schema 必然 break test |
| fast-path 闸门调整 | Medium | Test #14 (FactPlusUncertainty_FastPathBlocked) 锁死 item_pipeline.go:301 行为 |
| 4-kind 枚举值变更 | Low | Test #11 (KindAliasCaseInsensitive) 锁死 mapRawObsKind 4 值 |

## 7. 关联

### 父 Change
- `devrix-d7-observational-fastpath` (DM-20260706-011, S7_Archived 2026-07-07) — fast-path 闸门契约源头

### 关联 PR
- #472 (Trace validation 16 tests) — 本 spec 的运行验证
- #470 (D7 fast-path task_incomplete bypass) — 上一阶段 hotfix
- #471 (D1 fast-path task_incomplete bypass) — 上一阶段 hotfix