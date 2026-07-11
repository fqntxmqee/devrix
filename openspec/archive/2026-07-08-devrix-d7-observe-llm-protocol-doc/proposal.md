# Proposal: D7 Observe ↔ LLM 5 场景输入输出协议沉淀 (DM-20260708-003)

**Change ID:** `devrix-d7-observe-llm-protocol-doc`
**Demand ID:** DM-20260708-003
**Priority:** P1
**Status:** S2_Proposal
**PR Strategy:** 追加到 PR #472 (trace test 16 cases) 作为 spec doc 沉淀

---

## 1. Background

D7 Observe 节点是 MUPS 5 节点流水线的入口，负责把用户 directive + 结构化 signal
转成类型化的 `ObservationProposal[]` (4 种 ObsKind + 1 混合场景)。下游 Plan /
fast-path / Learn 都消费这份产出。

`devrix-d7-observational-fastpath` (DM-20260706-011, S7_Archived 2026-07-07) 沉淀
了 fast-path 4 闸门契约，但**未显式定义 Observe↔LLM 的帧级 I/O 协议**：
- 11→6 字段过滤规则 (5 字段为什么被过滤)
- 4 kind 的 JSON schema 与 payload 字段约束
- Go-side 兜底规则 (strength cap / 零保护 / evidence 注入)
- 混合场景 (fact+uncertainty) 的 hasObsUncertainty 阻断机制

这些契约散落在 6 个 Go 文件 + 1 个 i18n 文件里，future maintainer / code
reviewer / 用户验证时需要 grep 跳读。PR #472 用 15 个 trace test 把契约**运行化**，
但没有 spec doc 描述**契约本身**。

## 2. Goal Shape

产出 1 份 spec doc (`d7-observe-llm-io-protocol-spec.md`)，显式回答 5 个维度：

| 维度 | 回答 |
|---|---|
| 输入 | LLM 看到哪 6/11 字段？5 个被过滤字段理由？ |
| 输出 | 4 种 kind 的 JSON schema？每种 kind 的 payload 字段约束？ |
| 兜底 | Go-side `validateOneProposal` 做了什么？strength cap / 零保护 / evidence 注入？ |
| 路由 | LLM 输出 → `[]Observation` → `UncertaintyReport.Partition()` 怎么分 Business/Anomalies？ |
| 5 场景 | 纯确定性 / 纯不确定性 / 结构化信号 / 异常检测 / fact+uncertainty 混合 |

**不变性承诺**：本 Change **不修改任何源代码**（除了 +1 test 注释修正），
纯 spec 沉淀 + 1 test 补充。

## 3. Deliverable

| 路径 | 类型 | 描述 |
|---|---|---|
| `openspec/changes/devrix-d7-observe-llm-protocol-doc/specs/d7-orchestration/d7-observe-llm-io-protocol-spec.md` | NEW | 主 spec 文档 (5 场景 I/O 协议，~500 行) |
| `internal/layers/orchestration/sessionorchestrator/observe_trace_e2e_test.go` | MODIFIED | +1 test (TestObserveTraceE2E_FactPlusUncertainty_FastPathBlocked) + 注释重编号 |

## 4. Acceptance Criteria

| ID | 标准 | 验证方式 |
|---|---|---|
| AC1 | spec 覆盖 5 场景，每场景含 ① 输入 ② 期望输出 ③ Go 处理 ④ 最终路由 | review |
| AC2 | 11→6 字段过滤 + 5 字段理由明确 | review |
| AC3 | 4 种 kind 的 Payload 字段约束文档化 | review |
| AC4 | 混合场景显式标注 fast-path 阻断，引用 `item_pipeline.go:301` 闸门 | review |
| AC5 | 16 个 trace test 全部 PASS（含 TestObserveTraceE2E_FactPlusUncertainty_FastPathBlocked） | `go test -race` |
| AC6 | trace test stdout 是 spec 的"活验证" | manual review |

## 5. Non-Goals

- ❌ 修改任何源代码（仅 spec + 1 test）
- ❌ 合并 spec 到主 `openspec/specs/d7-orchestration/spec.md`（S6 归档时再决策）
- ❌ 新增 ObservationKind（type system 已 sealed）
- ❌ 改 i18n prompt（i18n 防呆已存在，混合场景由 Gate 3 兜底）
- ❌ 加 EN 版本的 mixed-scenario 引导（后续工作）

## 6. Risks

| 风险 | 缓解 |
|---|---|
| spec 与实现漂移 | trace test stdout 是"活验证"，任何字段/scenario 不一致都被 test 暴露 |
| 字段过滤规则变更 (11→6 变化) | Test #1 (OnlyFieldsVisibleToLLM) 锁死 |
| fast-path 闸门调整 | Test #15 (FactPlusUncertainty_FastPathBlocked) 锁死 `item_pipeline.go:301` 行为 |
| 4-kind 枚举值变更 | Test #11 (KindAliasCaseInsensitive) 锁死 `mapRawObsKind` 4 值 |

## 7. 关联

### 父 Change
- `devrix-d7-observational-fastpath` (DM-20260706-011, S7_Archived 2026-07-07) — fast-path 闸门契约源头

### 关联 PR
- #472 (Trace validation 16 tests) — 本 spec 的运行验证
- #470 (D7 fast-path task_incomplete bypass) — 上一阶段 hotfix
- #471 (D1 fast-path task_incomplete bypass) — 上一阶段 hotfix
