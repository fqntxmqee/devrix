// Package escape implements MUPS v5 (DM-20260625-003) 统一逃逸机制
// (Unified Escape Mechanism). It provides:
//
//   - LoopDepthTracker v2: 按"模式 hash"计数回路深度，MaxDepth=3 兜底 ForceExit
//   - PlanKindSwitchPolicy: 3 档策略（Constrained ≤4 / Allowed / Forbidden）
//   - ChainedArbitrator: LLM/Rule/Human 3 层仲裁（5s + 10s timeout 兜底）
//   - EscapeEngine: 整合 3 类深度限制（回路深度 + LoopBudget + CircuitBreaker）
//   - CircuitBreaker 5 层: L0 AnomalyDetector / L1 DispatchLoop / L2 Verifier / L3 Hook / L4 WorkerPanic / L5 SandboxExit
//   - AuditLog: AuditLevel 0/1/2 + 14 ExitReason 映射
//   - 5 节点 EscapeEngine 接线点: Observe 失败 / Plan 失败 1a / Plan 前 1b / Execute 失败 / Verify 失败
//   - T2 ResumeSession 续跑: user_choice A/B/C → Continue/ForceExit/AbortWithAudit
//   - 13 类失败降级矩阵: Evaluate panic/error + audit fail-open + LLM timeout + ctx cancel + CB metric timeout
//
// 包与现有 Phase 1-7 数据契约零破坏性变更（叠加而非取代），通过 ProcessMessage
// 的 5 节点接线 + 失败降级矩阵与 D7 编排层集成。
//
// SoT: brain/.../core-concepts/38-mature-uncertainty-methodology.md §21
//      (line 3621-4025, 400 行 v5 完整设计)
//
// 错误码范围: 7100-7199 (orchestration escape 子域)
package escape