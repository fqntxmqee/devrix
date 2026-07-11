# Proposal: D7 Plan↔LLM 5 场景输入输出协议沉淀 (DM-20260708-004)

**Change ID:** `devrix-d7-plan-llm-protocol-doc`
**Demand ID:** DM-20260708-004
**Priority:** P1
**Status:** S2_Proposal
**PR Strategy**: 新 PR, 沿用 devrix-d7-observe-llm-protocol-doc 同模板

---

## 1. Background

`devrix-d7-observe-llm-protocol-doc` (DM-20260708-003, S5_Accepted) 沉淀了
Observe↔LLM 4 kind + 1 混合场景的 I/O 协议 (PR #473)。但 Plan 节点同样存在
契约散落问题：

- `StrategicPlanFrame` 18 字段 frame 结构 (`pt` struct tag + 反射)
- `rawStrategicPlan` JSON wire format (8 字段)
- `validateStrategicPlan` Go 兜底 (3 选 1 enum + 强制清空)
- 4 PlanKind 路由 (`MatchKind` 4 Rules)
- `applySingleModeUncertaintyGate` fast-path bypass (DM-20260706-009)

散落在 `strategic_plan_proposer.go` (524 行) + `plan/` 子包 (12 文件) +
`prompt_dynamic.go:79-104`。Plan 节点**零** trace-level e2e bannered test
(对比 observe 16 个是显著 gap)。

## 2. Goal Shape

产出 1 份 spec doc (`d7-plan-llm-io-protocol-spec.md`)，显式回答 5 个维度：

| 维度 | 回答 |
|---|---|
| 输入 | LLM 看到哪 18 字段？字段过滤规则 (区别于 observe 11→6 显式过滤)？ |
| 输出 | `rawStrategicPlan` JSON schema？与 `plan.Plan` (4 PlanKind) 的关系？ |
| 兜底 | Go-side `validateStrategicPlan` 做了什么？ |
| 路由 | LLM `execution_mode` → `QuantizedKind` → `MatchKind` → 4 PlanKind 怎么映射？ |
| 5 场景 | single→commitment / command+multi→protocol / parallel_probe→scenario / decompose+anomaly→exploration / single+高 uncertainty+高 strength fact → fast-path bypass |

**不变性承诺**：本 Change **不修改任何源代码**，纯 spec 沉淀 + 5 test 补充
(对比 observe 同模板)。

## 3. Deliverable

| 路径 | 类型 | 描述 |
|---|---|---|
| `openspec/changes/devrix-d7-plan-llm-protocol-doc/specs/d7-orchestration/d7-plan-llm-io-protocol-spec.md` | NEW | 主 spec 文档 (5 场景 I/O 协议) |
| `internal/layers/orchestration/sessionorchestrator/plan_trace_e2e_test.go` | NEW | 5 trace test (含混合场景) |

## 4. Acceptance Criteria

| ID | 标准 | 验证方式 |
|---|---|---|
| AC1 | spec 覆盖 5 场景, 每场景含 ① 输入 ② 期望输出 ③ Go 处理 ④ 最终路由 | review |
| AC2 | 18 字段 frame 结构 + 字段过滤规则 (data/control plane + omit 条件) 文档化 | review |
| AC3 | `rawStrategicPlan` JSON schema + execution_mode 3 选 1 enum 文档化 | review |
| AC4 | LLM `execution_mode` → `QuantizedKind` → `MatchKind` → 4 PlanKind 映射表明确 | review |
| AC5 | 混合场景显式标注 fast-path bypass, 引用 `strategic_plan_proposer.go:723` | review |
| AC6 | 5 个 trace test 全部 PASS | `go test -race` |
| AC7 | trace test stdout 是 spec 的"活验证" | manual review |

## 5. Non-Goals

- ❌ 修改任何源代码（仅 spec + 5 test）
- ❌ 合并 spec 到主 `openspec/specs/d7-orchestration/spec.md`（S6 归档时再决策）
- ❌ 新增 PlanKind（type system 已 sealed 4 类）
- ❌ 改 i18n prompt（`StrategicPlanAppendix` 已被 `prompt_dynamic.go:79-104` 固化）
- ❌ 改 `MatchKind` 4 Rules 优先级（现有 priority 是 Phase 2 PR-B1 决策）

## 6. Risks

| 风险 | 缓解 |
|---|---|
| spec 与实现漂移 | trace test stdout 是"活验证"，任何字段/scenario 不一致都被 test 暴露 |
| `StrategicPlanFrame` 字段增减 | Test #1 (FrameStructure_18Fields) 锁死 18 字段 |
| `rawStrategicPlan` JSON schema 变更 | Test #2 (JSONSchema_ExecutionModeEnum) 锁死 3 选 1 enum |
| `MatchKind` 4 Rules 调整 | Test #3-4 (MatchKind_All4Kinds) 锁死 routing |
| fast-path 闸门调整 | Test #5 (FastPathBypass) 锁死 `applySingleModeUncertaintyGate` 行为 |

## 7. 关联

### 父 Change
- `devrix-d7-observe-llm-protocol-doc` (DM-20260708-003) — 同模板沉淀的兄弟 spec, PR #473

### 关联 Change
- `devrix-d7-mups-v4-phase2-prb1` (DM-20260623-001, S7_Archived) — `plan.Plan` 4 PlanKind 源头
- `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, S7_Archived) — 4 Channel router
- `devrix-d7-observational-fastpath` (DM-20260706-011, S7_Archived) — Observe fast-path
- DM-20260706-009 — Plan single-mode fast-path bypass
- DM-20260704-006 — RC-1 resolution_strategies[] contract
