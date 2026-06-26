# D7 Orchestration Delta — WorkItem ContextGraph

**Change ID:** `devrix-d7-workitem-context-graph`
**Base:** `openspec/specs/d7-orchestration/d7-domain.md` v2.4.0

## ADDED

### D7-S1 WorkModel — ContextGraph 契约

- `ContextScope` 1:1 绑定非 Ephemeral WorkItem
- `ContextLinkKind` / `ContextBubbleKind` 枚举
- `ContextLinkEvaluator` CL0–CL8、`ContextBubbleEvaluator` CB0–CB6
- Sibling taxonomy R1–R6 + `ClassifySiblingRelation`
- WorkItem 扩展：`ContextScopeID`, `ContextPolicy`

### Feature Flag

- `D7_WORKITEM_CONTEXT_GRAPH=1`（F3+ 运行时，F1 仅注册 env 常量）

## MODIFIED

- `workitem-pipeline-unification-design.md` — Related 指向 ContextGraph 设计
- `d7-domain.md` — 索引 ContextGraph 设计文档

## UNCHANGED

- SpawnPolicyEvaluator R0–R8
- WorkTree BlockedBy 语义（ContextGraph 只读映射为 CL0）
