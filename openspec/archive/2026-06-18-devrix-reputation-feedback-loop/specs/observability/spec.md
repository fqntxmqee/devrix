# Spec: D5/D6 — 信誉、置信度与惩罚闭环

**Change ID:** devrix-reputation-feedback-loop
**Demand ID:** DM-20260614-008
**Status:** S7_Archived (2026-06-18; S1_Cancelled)

## 1. 变更性质

本 change 期望建立"Agent 重复博弈"场景下的信誉/置信度/惩罚闭环。变更在 S1 阶段取消，未进入实施。

## 2. 涉及域

- D5 Observability（信誉 + 置信度）
- D6 Evolution（惩罚）
- D1 Communication（信号元数据）
- D2 Context Engine（上下文聚合）
- D4 Multi-Agent（Agent 选择）

## 3. 接口契约（草案，未实施）

```go
// D5 Observability
type ReputationStore interface {
    Get(agentID string) (AgentReputation, error)
    Update(agentID string, event ReputationEvent) error
}

type ConfidenceScorer interface {
    Score(ctx context.Context, output AgentOutput) (float64, error)
}

// D6 Evolution
type PenaltyPolicy interface {
    Decide(agentID string, history []ReputationEvent) PenaltyDecision
}

type PenaltyEnforcer interface {
    Enforce(ctx context.Context, decision PenaltyDecision) error
}
```

## 4. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S1_Cancelled → Archived；接口契约草案保留作为后续重开参考。