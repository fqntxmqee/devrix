# Proposal: D7 Execute ↔ ToolRunner 4 Channel 5 场景输入输出协议沉淀 (DM-20260708-005)

**Change ID:** `devrix-d7-execute-llm-protocol-doc`
**Demand ID:** DM-20260708-005
**Priority:** P1
**Status:** S2_Proposal
**PR Strategy:** 独立 PR (1 spec + 5 trace test + spec.md/CHANGELOG.md 集成)

---

## 1. Background

D7 Execute 节点是 MUPS 5 节点流水线的第 3 节点,与 Observe/Plan 节点有根本不同:
**不直接调用 LLM**。它通过 `ChannelRouter` 把 `*plan.Plan` 路由到 4 种 Channel
(Commit / Protocol / Scenario / Exploration),每种 Channel 调用一个**可插拔
ToolRunner** 执行 `plan.Step` 序列,返回 `*wavescheduler.Artifact`。

`devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, S7_Archived) 落地了 4 Channel
router,但**未显式定义 Execute↔ToolRunner 的 4 Channel I/O 协议**:
- ChannelRequest 3 字段 / ToolRequest 5 字段契约
- 4 Channel 输入输出差异 (step 数 / 并发 / 副作用 / rollback / worker)
- ChannelRouter 1:1 路由保证 (PlanKind ↔ Channel)
- ArtifactKind 4 态 + SideEffectStatus 5 态 + WorkerType 3 态矩阵
- 7 SentinelError (EXEC_CHANNEL_9001-9007) wire format
- sideEffectForScope 映射 (Exploration 唯一消费 PersistScope)
- 混合场景:Commit timeout (9006) vs Scenario ctx cancel (9007) 不混淆

这些契约散落在 6 个 Go 文件 + 1 个 shared/types 文件里,future maintainer / code
reviewer / 用户验证时需要 grep 跳读。Execute 节点作为 4 Channel router 的总入口,
缺少 spec 文档是个显著的 onboarding 障碍。

PR #473 (Observe) 和 PR #474 (Plan) 已经用 16/5 个 trace test 把契约**运行化**,
Execute 节点也需要同等的 5 个 trace test 沉淀。

## 2. Goal Shape

产出 1 份 spec doc (`d7-execute-toolrunner-io-protocol-spec.md`),显式回答 6 个维度:

| 维度 | 回答 |
|---|---|
| 输入 | ChannelRequest 3 字段 / ToolRequest 5 字段 / PlanKind 4 enum |
| 输出 | ArtifactKind 4 + SideEffectStatus 5 + WorkerType 3 矩阵 |
| 4 Channel 差异 | step 数 / 并发 / 副作用 / rollback / worker / 配置默认值 |
| 路由 | ChannelRouter 1:1 映射 + 3 个 defensive checks (nil / unknown / not found) |
| 5 场景 | Commit success / Protocol rollback / Scenario majority / Exploration partial / Mixed (timeout + ctx cancel) |
| 错误 | 7 SentinelError + EXEC_CHANNEL_9001-9007 闭集 |

**不变性承诺**: 本 Change **不修改任何源代码** (除了主 spec.md 加 1 行 reference),
纯 spec 沉淀 + 5 trace test 补充 + 1 行 CHANGELOG。

## 3. Deliverable

| 路径 | 类型 | 描述 |
|---|---|---|
| `openspec/specs/d7-orchestration/d7-execute-toolrunner-io-protocol-spec.md` | NEW | 主 spec 文档 (5 场景 I/O 协议, ~600 行) |
| `internal/layers/orchestration/mups/execute/execute_trace_e2e_test.go` | NEW | 5 NEW trace test (Commit/Protocol/Scenario/Exploration/Mixed) |
| `openspec/specs/d7-orchestration/spec.md` | MODIFIED | +1 行 reference (§11 NEW) |
| `openspec/specs/d7-orchestration/CHANGELOG.md` | MODIFIED | +1 row: devrix-d7-execute-llm-protocol-doc (2026-07-09) |

## 4. Acceptance Criteria

| ID | 标准 | 验证方式 |
|---|---|---|
| AC1 | spec 覆盖 5 场景,每场景含 ① 输入 ② 期望输出 ③ Go 处理 ④ 测试 | review |
| AC2 | ChannelRequest 3 字段 + ToolRequest 5 字段 + PlanKind 4 enum 文档化 | review |
| AC3 | 4 Channel 差异表 (step 数 / tool / side-effect / rollback / worker) 明确 | review |
| AC4 | 7 SentinelError + EXEC_CHANNEL_9001-9007 错误码表文档化 | review |
| AC5 | sideEffectForScope 映射 (3 PersistScope → 3 SideEffect) 明确 | review |
| AC6 | 5 个 trace test 全部 PASS (含 Mixed 场景 timeout+ctx cancel) | `go test -race` |
| AC7 | 混合场景显式标注 EXEC_CHANNEL_9006 vs 9007 不混淆 | review |
| AC8 | spec 与现有 23 个 execute_test.go 测试 (797 lines) 互补, 不重复 | grep |
| AC9 | spec 引用集成到主 spec.md (§11 NEW) | review |

## 5. Non-Goals

- ❌ 修改任何源代码 (除了主 spec.md +1 行)
- ❌ 新增 ChannelKind (type system 已 sealed 4)
- ❌ 合并 spec 到主 `openspec/specs/d7-orchestration/spec.md` 全文 (lite-mode 兼容, 仅 reference)
- ❌ 改 ChannelRouter 签名 (冻结)
- ❌ 加 Channel 性能 profile span attributes (后续工作)
- ❌ 单元化 Channel 到子包 (类似 D5 sub-package pattern, 后续工作)

## 6. Risks

| 风险 | 缓解 |
|---|---|
| spec 与实现漂移 | trace test stdout 是"活验证",任何 channel 行为不一致都被 test 暴露 |
| Channel 接口签名变更 | 5 trace test 锁死 Execute / ChannelRequest / Artifact 字段 |
| 7 ErrorCode 增删 | Test #5 (Mixed) 显式验证 EXEC_CHANNEL_9006/9007 wire format |
| sideEffectForScope 映射变更 | Test #4 (Exploration) 显式覆盖 PersistTransient/Session/Permanent |
| RH-D7-09 fix 被回滚 | Test #2 + #3 (Scenario/Exploration) 含 ctx-cancel 验证 |
| 现有 23 个 test 漂移 | 0 修改 execute_test.go, 仅在 execute_trace_e2e_test.go 加 5 NEW |

## 7. 关联

### 父 Change
- `devrix-d7-plan-llm-protocol-doc` (DM-20260708-004, S5_Accepted) — 兄弟 spec, 同模板
- `devrix-d7-observe-llm-protocol-doc` (DM-20260708-003, S5_Accepted) — 兄弟 spec
- `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, S7_Archived) — 4 Channel router 实现 SoT

### 关联 PR
- #474 (Plan trace validation + spec) — 兄弟 spec
- #473 (Observe trace validation 16 tests + spec) — 兄弟 spec
- 未来 PR: 5 trace test + 本 spec
