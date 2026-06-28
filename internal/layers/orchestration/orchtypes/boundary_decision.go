package orchtypes

// Boundary Debt Decision Constants (DM-20260629-001 PR-9, T47).
//
// 3 项越界能力在 v6.0.0 临时放在 D7 域（含工作树 / orchestrator 内），
// 但归属存在争议。每项分配 boundary-debt ID 以便后续 v7.0 重新评估
// 时精准迁移。

const (
	// BoundaryReputationEvidence 标记 ReputationEvidence 数据结构
	// 在 D7 域内定义但跨域使用（Learn 节点产出 → Observe 节点消费）。
	// 归属决策待 v7.0 重新评估：可能迁移到 shared/types 或独立 evolution 域。
	BoundaryReputationEvidence = "boundary-debt:reputation-evidence-v7.0"

	// BoundarySystemAnomaly 标记 SystemAnomaly 阈值触发逻辑在 D7 域内，
	// 但跨 Verify (D7-S10) + Observe (D7-S5) 双消费。归属决策待 v7.0
	// 重新评估：可能提升为 hardening/ 横切包。
	BoundarySystemAnomaly = "boundary-debt:system-anomaly-v7.0"

	// BoundaryAdaptivePrior 标记 AdaptivePrior Bayesian 状态在 D7 域
	// workmodel 内，但跨 SessionOrchestrator + Learner 双读写。
	// 归属决策待 v7.0 重新评估：可能提升为 D2 contextengine 子模块。
	BoundaryAdaptivePrior = "boundary-debt:adaptive-prior-v7.0"
)