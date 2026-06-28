# ADR-001: CG2′ — Transcript 隔离与 Cohort 域共享

**Status:** Accepted (R1 2026-06-28)  
**Change:** `devrix-d7-layer-subcontext`  
**Supersedes:** ContextGraph CG2 v0.3.0 单一「默认隔离」语义

---

## Context

原 CG2：子/sibling 上下文 **不自动** 合并；任何透传须 `ContextLinkRecord`。

WorkTree 深度增加后，「同层协作」需要共享 **ScopeContract 与 cohort 元数据**，但 **不能** 共享 ReAct 全文（公共池塘悲剧 + 并行污染）。

## Decision

修订为 **CG2′** 双轨：

1. **Transcript 隔离：** 未经批准的 Link，不得注入其他 WI 的 WorkItemPrivate 链。  
2. **Cohort 域共享：** 同 Parent sibling 共享 ScopeContract + cohort meta（非 transcript）。  
3. **Signal 透传：** Bubble / Upstream / PeerStatus / Link 均为 Signal，适用 CL/CB 规则。

## Consequences

- `workitem-context-graph-design.md` bump **0.3.0 → 0.4.0**。  
- F3 测试语义 MODIFIED：sibling 共享 scope prompt，不共享 wi jsonl。  
- Materialize 成为 D2 统一读路径（flag=on）。

## 博弈论依据

见 `game-theory-review.md` §1.5：session 单桶 → 分层产权；cohort 为新公共池塘，需 OQ-LC-9 预算 cap。
