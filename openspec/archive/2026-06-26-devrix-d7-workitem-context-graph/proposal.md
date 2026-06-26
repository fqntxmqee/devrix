# Proposal: WorkItem × ContextGraph 分层透传

**Change ID:** `devrix-d7-workitem-context-graph`
**Demand ID:** DM-20260626-020
**Priority:** P1
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**SoT:** `openspec/specs/d7-orchestration/workitem-context-graph-design.md`

---

## 1. Problem Statement

WorkItem Pipeline 解决了 **spawn 规则化**，但未解决 **上下文分区与透传规则化**。多假设并行探索时，session 级 history 导致假收敛；依赖任务看不到 upstream Artifact。

## 2. Proposed Solution

两维模型：**WorkTree（做什么）× ContextGraph（谁知道什么）**。

- `ContextScope` 1:1 绑定 WorkItem
- 垂直：`ContextBubbleKind`（structured 强制 + 叙事门控）
- 水平：`ContextLinkKind` + Sibling taxonomy R1–R6
- `ContextLinkEvaluator` / `ContextBubbleEvaluator` 对称 `SpawnPolicyEvaluator`

## 3. Scope

### In Scope (this change)

- Phase F1–F6 per design §11
- Feature flag `D7_WORKITEM_CONTEXT_GRAPH=1`（F3+）

### Out of Scope

- D2 存储引擎重写
- 跨 Session Feishu UI（TD-WT-04）
- 保证 LLM 结论正确（D6 advisory）

## 4. Review 决议（2026-06-26）

| 项 | 结论 |
|----|------|
| OQ-CG-1 存储 | WorkTree 字段 + D2 sidechain `wi_<id>`（F5） |
| OQ-CG-2 sibling share | **仅 completed → pending 单向**（防环） |
| OQ-CG-4 ContextPolicy SoT | WorkItem.ContextPolicy，Plan 仅提案 |
| 与 Pipeline 关系 | Context 不驱动 spawn；仅 Observe/Execute 输入 |
| 审查结论 | **通过**，无阻塞性问题；分 Phase 落地 |

## 5. Risks

| Risk | Mitigation |
|------|------------|
| Context 环 | CL8 拒绝 + OQ-CG-2 单向 share |
| Token 爆炸 | CB3 降级链 full_tail → summary → structured |
| 与 legacy Wave 双轨 | WorkItem.ContextPolicy 为 SoT，Wave 投影只读 |

## 6. Success Criteria

- [x] F1: `go test ./internal/layers/orchestration/workmodel/... -run Context` PASS
- [x] F3: BlockedBy 集成测试 Wave upstream
- [x] F6: ResolveHint 展示 link/bubble 审计信息

---

## Archive Information

**Archived:** 2026-06-26
**Duration:** 1 day
**Outcome:** Successfully implemented
**PR:** #244 → master (`6aec4c6`)

### Specs Updated

- `openspec/specs/d7-orchestration/workitem-context-graph-design.md` v0.3.0
- `openspec/specs/d7-orchestration/d7-domain.md` v2.5.0
- `openspec/archive/2026-06-26-devrix-d7-workitem-context-graph/specs/d7-orchestration/spec.md`

### Code (primary)

- `internal/layers/orchestration/workmodel/context_*.go`
- `internal/layers/orchestration/sessionorchestrator/item_observe.go`
- `internal/layers/orchestration/sessionorchestrator/item_pipeline.go`
