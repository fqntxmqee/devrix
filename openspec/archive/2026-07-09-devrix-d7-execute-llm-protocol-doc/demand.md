---
demand-id: DM-20260708-005
title: D7 Execute-ToolRunner 4 Channel 5 场景输入输出协议沉淀
priority: P1
status: S2_Proposal
dsaft_domain: orchestration
created: 2026-07-09
parent_change: devrix-d7-plan-llm-protocol-doc
parent_demand: DM-20260708-004
origin: |
  从 devrix-d7-plan-llm-protocol-doc (DM-20260708-004) S5_Accepted 后的
  对称需求。Plan 节点已沉淀帧级 I/O 协议,Execute 节点作为 4 Channel
  路由器 (Commit / Protocol / Scenario / Exploration) 也需要把
  ChannelRouter → Channel → ToolRunner 三层协议契约显式文档化。
  Execute 节点与 Observe/Plan 的根本区别是**不进 LLM**,通过 4 Channel
  派发 plan.Step 到 pluggable ToolRunner。本 spec 覆盖 4 Channel
  I/O + 1 混合场景 (timeout + ctx cancel)。
---

# D7 Execute-ToolRunner 4 Channel 5 场景输入输出协议沉淀

## 1. 背景

D7 Execute 节点是 MUPS 5 节点流水线的第 3 节点,负责把 `*plan.Plan` 路由到 4 种
Channel,每种 Channel 调用 pluggable ToolRunner 执行 `plan.Step[]`,返回
`*wavescheduler.Artifact` 给 Phase 4 Verify。

现有契约散落在:
- `internal/layers/orchestration/mups/execute/channel.go:69-82` — PlanChannel interface
- `internal/layers/orchestration/mups/execute/channel.go:206-258` — ChannelRouter.Route
- `internal/layers/orchestration/mups/execute/commit.go:39-149` — CommitChannel
- `internal/layers/orchestration/mups/execute/protocol.go:45-208` — ProtocolChannel
- `internal/layers/orchestration/mups/execute/scenario.go:37-167` — ScenarioChannel
- `internal/layers/orchestration/mups/execute/exploration.go:43-243` — ExplorationChannel
- `internal/layers/orchestration/mups/execute/errors.go:30-154` — 7 SentinelError + helpers
- `internal/shared/types/execute.go:19-147` — ArtifactKind 4 + SideEffectStatus 5
- `internal/layers/orchestration/plan/plan.go:30-140` — 4 PlanKind enum
- `internal/layers/orchestration/plan/blast_radius.go:13-72` — BlastRadius + PersistScope + Step

**契约** = 1 份协议规范(给三方 reviewer / future maintainer / 用户验证用)+ 5 个 trace test(运行验证)。

## 2. 沉淀目标

产出 1 份 spec doc,**显式**回答以下问题:

| 维度 | 问题 |
|---|---|
| 输入 | ChannelRequest 3 字段 + ToolRequest 5 字段 + PlanKind 4 enum 是什么? |
| 输出 | ArtifactKind 4 + SideEffectStatus 5 + WorkerType 3 是什么? |
| 4 Channel 差异 | Commit/Protocol/Scenario/Exploration 在 step 数 / 并发 / 副作用 / rollback 行为如何? |
| 路由 | ChannelRouter 1:1 映射 (PlanKind ↔ Channel) 怎么保证? |
| 场景 1 | CommitChannel 1 step 成功 → ArtifactStateChangeCert + SideEffectCommitted |
| 场景 2 | ProtocolChannel 多步 + step 2 失败 → rollback → SideEffectRolledBack |
| 场景 3 | ScenarioChannel 5 并行探测 + 3 通过 → majority vote → SideEffectNone |
| 场景 4 | ExplorationChannel 3 实验 + 优先级排序 → sideEffectForScope 决定 SideEffectStatus |
| 场景 5 (混合) | Commit timeout + Scenario ctx cancel — 区分 EXEC_CHANNEL_9006 vs 9007 |

## 3. 验收标准

| ID | 标准 | 优先级 | 验证方式 |
|----|------|--------|---------|
| AC1 | spec 文档覆盖 5 场景, 每场景含 ① 输入协议 ② 期望输出 ③ Go 侧处理 ④ 测试 | P0 | review |
| AC2 | 4 Channel I/O 契约 (ChannelRequest 3 字段 / ToolRequest 5 字段 / Artifact 13 字段) 文档化 | P0 | review |
| AC3 | 4 Channel 差异表 (step 数 / tool / side-effect / rollback / worker) 明确 | P0 | review |
| AC4 | 7 SentinelError + EXEC_CHANNEL_9001-9007 错误码表文档化 | P0 | review |
| AC5 | sideEffectForScope 映射 (PersistTransient/Session/Permanent → SideEffect) 明确 | P0 | review |
| AC6 | 5 个 trace test 全部 PASS, 模拟 observe/plan 节点的 printBanner 模式 | P0 | go test |
| AC7 | 混合场景 (timeout + ctx cancel) 显式标注 EXEC_CHANNEL_9006 vs 9007 不混淆 | P0 | review |
| AC8 | spec 与现有 23 个 execute_test.go 测试 (797 lines) 互补, 不重复 | P1 | grep |
| AC9 | 集成 spec 引用到主 spec.md (lite-mode 兼容) | P1 | review |

## 4. 依赖与约束

| 类型 | 内容 |
|---|---|
| 依赖 | devrix-d7-plan-llm-protocol-doc (DM-20260708-004, S5_Accepted) |
| 依赖 | devrix-d7-mups-v4-phase2-prb1 (DM-20260623-001, 4 PlanKind S7_Archived) |
| 依赖 | devrix-d7-mups-v4-phase3-prc2 (DM-20260625-001, 4 Channel S7_Archived) |
| 依赖 | devrix-d7-mups-frame-delta-closure (DM-20260705-010) |
| 依赖 | RH-D7-09 (DM-20260630-013, scenario/exploration ctx-cancel fix) |
| 约束 | 不修改任何源代码 — 纯 spec 沉淀 + 5 trace test 补充 |
| 约束 | spec 章节锚定 file:line (与 d7-plan-llm-io-protocol-spec.md 风格一致) |
| 约束 | 不重复 d7-mups-v4-phase3-prc2-archived.md 已覆盖的 4 Channel router 契约 |
| 约束 | spec doc 写到 openspec/specs/d7-orchestration/ (canonical 位置), 变更目录只放流程文件 |

## 5. 变更范围

### 新增

| 路径 | 描述 |
|------|------|
| `openspec/specs/d7-orchestration/d7-execute-toolrunner-io-protocol-spec.md` | 主 spec 文档 (5 场景 I/O 协议, ~600 行) |
| `internal/layers/orchestration/mups/execute/execute_trace_e2e_test.go` | 5 NEW trace test (Commit/Protocol/Scenario/Exploration/Mixed) |
| `openspec/specs/d7-orchestration/spec.md` (MODIFIED) | +1 行 reference, 指向 d7-execute-toolrunner-io-protocol-spec.md |
| `openspec/specs/d7-orchestration/CHANGELOG.md` (MODIFIED) | +1 row: devrix-d7-execute-llm-protocol-doc (2026-07-09) |

### 不变更

- `internal/layers/orchestration/mups/execute/channel.go` — 0 修改
- `internal/layers/orchestration/mups/execute/commit.go` — 0 修改
- `internal/layers/orchestration/mups/execute/protocol.go` — 0 修改
- `internal/layers/orchestration/mups/execute/scenario.go` — 0 修改
- `internal/layers/orchestration/mups/execute/exploration.go` — 0 修改
- `internal/layers/orchestration/mups/execute/errors.go` — 0 修改
- `internal/shared/types/execute.go` — 0 修改
- `internal/layers/orchestration/plan/plan.go` — 0 修改
- `internal/layers/orchestration/plan/blast_radius.go` — 0 修改
- 现有 `execute_test.go` (797 lines, 23 tests) — 0 修改

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| spec 与实现漂移 | High | trace test stdout 是 spec 的"活验证", 任何 channel 行为不一致都会被 test 暴露 |
| Channel 接口签名变更 | Medium | 5 trace test 锁死 Execute / ChannelRequest / Artifact 字段 |
| 7 ErrorCode 增删 | Low | Test #5 (Mixed) 显式验证 EXEC_CHANNEL_9006/9007 wire format |
| sideEffectForScope 映射变更 | Medium | Test #4 (Exploration) 显式覆盖 PersistTransient/Session/Permanent |
| RH-D7-09 fix 被回滚 | Medium | Test #2 + #3 (Scenario/Exploration) 含 ctx-cancel 验证 |

## 7. 关联

### 父 Change
- `devrix-d7-plan-llm-protocol-doc` (DM-20260708-004, S5_Accepted) — 兄弟 spec, 同模板
- `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, S7_Archived) — 4 Channel router 实现 SoT
- `devrix-d7-mups-v4-phase2-prb1` (DM-20260623-001, S7_Archived) — 4 PlanKind 源头

### 关联 PR
- #474 (Plan trace validation + spec) — 兄弟 spec
- #473 (Observe trace validation 16 tests + spec) — 兄弟 spec
- 未来 PR: 5 trace test + 本 spec
