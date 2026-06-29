// Package orchtypes holds D2-level cross-cutting governance constants.
//
// Boundary Debt Decisions (DM-20260629-002 PR-8, T44):
//
// 2 项越界能力在 v8.x 临时放在 D2 域（enforce/ + 根 fixture），
// 但归属存在争议。每项分配 boundary-debt ID 以便后续 v9.0 重新评估
// 时精准迁移。
package orchtypes

const (
	// BoundaryDM018SliceC 标记 FlowEvent / WorkPlan 数据流在 v8.0.0
	// 之前由 D2 nested/flow_report.go 持有，DM-018 slice-c 之后迁至
	// D7 executionflow/bridge/flow_reporter.go。D2 通过
	// contracts.SubQueryFlowReporter 端口消费，归属决策 RESOLVED。
	// ID 保留以追溯历史。
	BoundaryDM018SliceC = "boundary-debt:dm-018-slice-c-v7.0"

	// BoundaryCrossDomainFixtures 标记 2 个跨域 fixture (summarizer_fixture.go +
	// prepared_turn_fixture.go) 在 D2 根目录保留 (无 import cycle 风险)，
	// 但语义上属 D7 域 (StaticSummarizer / StaticPreparedTurnRunner 是
	// PreparedTurnRunner 测试替身)。归属决策待 v9.0 重新评估：可能
	// 迁移到 internal/layers/orchestration/testutil/ 或独立 fixture 包。
	BoundaryCrossDomainFixtures = "boundary-debt:cross-domain-fixtures-v9.0"
)
