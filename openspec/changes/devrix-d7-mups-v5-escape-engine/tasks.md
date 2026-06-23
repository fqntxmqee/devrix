# Tasks — devrix-d7-mups-v5-escape-engine (DM-20260625-003)

**Change ID:** `devrix-d7-mups-v5-escape-engine`
**Demand ID:** DM-20260625-003
**Sprint:** mups-v5
**Total Effort:** 6.5 天
**PR Count:** 5

---

## 任务总览

| Task ID | 内容 | 工作量 | 依赖 | PR | AC |
|---------|------|--------|------|-----|-----|
| T-01 | LoopDepthTracker v2 | 1 天 | doc 38 §19.2 | V5.1 | AC1 |
| T-02 | PlanKindSwitchPolicy 3 档 | 0.5 天 | 无 | V5.2 | AC2 |
| T-03 | ChainedArbitrator 3 层 | 2 天 | T-01 | V5.3 | AC3 |
| T-04 | EscapeEngine 整合 + CircuitBreaker 5 层 | 1 天 | T-03 | V5.4 | AC4, AC7 |
| T-05 | 5 节点接线 + 集成测试 + 文档同步 | 2 天 | 全部 | V5.5 | AC5, AC6, AC8 |
| **总计** | | **6.5 天** | | | 8 AC |

---

## T-01: LoopDepthTracker v2 (PR-V5.1, 1 天)

### 任务
- [ ] 创建 `internal/layers/orchestration/escape/` 目录
- [ ] 创建 `loop_depth_tracker.go`：
  - [ ] `LoopContext` struct（7 字段）
  - [ ] `LoopDepthTracker` struct（MaxDepth=3 + History map + LoopBudget + SessionID）
  - [ ] `hashLoopContext(ctx LoopContext) string` SHA-256
  - [ ] `ShouldContinue(ctx LoopContext) EscapeDecision`（占位 EscapeAction，V5.3 完善）
  - [ ] `Reset()` 清空 History
- [ ] 创建 `loop_depth_tracker_test.go`：
  - [ ] `TestLoopDepthTracker_FirstCall` depth=1 → Continue
  - [ ] `TestLoopDepthTracker_SameMode` depth++ → depth=2 → Continue
  - [ ] `TestLoopDepthTracker_DifferentMode` depth=1（新回路）
  - [ ] `TestLoopDepthTracker_ExceedMax` depth=4 → ForceExit
  - [ ] `TestHashLoopContext_Deterministic` 同输入 → 同 hash
  - [ ] `TestHashLoopContext_DifferentInput` 不同输入 → 不同 hash
- [ ] 单元测试 100% PASS
- [ ] 提交 + 开 PR

### 验收
- **AC1**：按模式 hash 计数，同模式重复 depth++，不同模式 reset
- **依赖**：无
- **风险**：低

---

## T-02: PlanKindSwitchPolicy 3 档 (PR-V5.2, 0.5 天)

### 任务
- [ ] 创建 `plan_kind_switch_policy.go`：
  - [ ] `PlanKindSwitchPolicy` enum（3 档）
  - [ ] `determineSwitchPolicy(planKind PlanKind) PlanKindSwitchPolicy` 决策函数
- [ ] 集成到 `internal/layers/orchestration/plan/planner.go`：
  - [ ] `MatchKind` 之后调用 `determineSwitchPolicy` 输出 policy
  - [ ] `LoopContext.PlanKindSwitchCount` 累加（基于 PrevPlanKind 检测）
  - [ ] 超过 4 → 返回 `ErrPlanKindSwitchExceeded`
- [ ] 创建 `plan_kind_switch_policy_test.go`：
  - [ ] `TestDetermineSwitchPolicy_Exploration` → Constrained
  - [ ] `TestDetermineSwitchPolicy_Scenario` → Allowed
  - [ ] `TestDetermineSwitchPolicy_Protocol` → Constrained
  - [ ] `TestDetermineSwitchPolicy_Commitment` → Forbidden
  - [ ] `TestPlanKindSwitchCount_ExceedLimit` 4 → OK, 5 → error
- [ ] 单元测试 100% PASS
- [ ] 提交 + 开 PR

### 验收
- **AC2**：3 档策略 + 切换计数 ≤4
- **依赖**：无
- **风险**：低

---

## T-03: ChainedArbitrator 3 层 (PR-V5.3, 2 天)

### 任务
- [ ] 创建 `arbitrator.go`：
  - [ ] `EscapeAction` enum（5 类）
  - [ ] `EscapeDecision` struct（4 字段）
  - [ ] `EscapeArbitrator` interface
  - [ ] `LLMArbitrator`：
    - [ ] 调 LLM（注入 prompt + PlanKindSwitchCount）
    - [ ] 5s timeout（context.WithTimeout）
    - [ ] Continue / Exit 二选一
  - [ ] `RuleArbitrator`：
    - [ ] 检查 `hasUnrecoverableFailure(ctx)` 
    - [ ] 不可恢复 → AbortWithAudit (AuditLevel=2)
    - [ ] 可恢复 → EscalateToHuman
  - [ ] `HumanArbitrator`：
    - [ ] 异步 prompt 用户（A/B/C）
    - [ ] 10s timeout 兜底 ForceExit
    - [ ] A → Continue, B → ForceExit, C → AbortWithAudit
  - [ ] `ChainedArbitrator`（3 层链式）
- [ ] 创建 `arbitrator_test.go`：
  - [ ] `TestLLMArbitrator_Continue` mock LLM 选 Continue
  - [ ] `TestLLMArbitrator_Exit` mock LLM 选 Exit
  - [ ] `TestLLMArbitrator_Timeout` 5s 超时 → ForceExit
  - [ ] `TestRuleArbitrator_Unrecoverable` → AbortWithAudit
  - [ ] `TestRuleArbitrator_Recoverable` → EscalateToHuman
  - [ ] `TestHumanArbitrator_ChoiceA` → Continue
  - [ ] `TestHumanArbitrator_Timeout` → ForceExit
  - [ ] `TestChainedArbitrator_LLMContinue` LLM 选 Continue → 立即返回
  - [ ] `TestChainedArbitrator_LLMExit_Rule` LLM Exit → Rule
  - [ ] `TestChainedArbitrator_RuleHuman` Rule Human → Human
- [ ] 单元测试 100% PASS
- [ ] 提交 + 开 PR

### 验收
- **AC3**：3 层仲裁（LLM/Rule/Human）+ 5 类 EscapeAction
- **依赖**：T-01
- **风险**：中，timeout 兜底 + Human 异步化

---

## T-04: EscapeEngine 整合 + CircuitBreaker 5 层 (PR-V5.4, 1 天)

### 任务
- [ ] 创建 `circuit_breaker.go`：
  - [ ] `CircuitBreaker` interface
  - [ ] `AnomalyDetectorCB`（L0）：5 次 nil → open
  - [ ] `DispatchLoopWakeupsCB`（L1）：100/min → open
  - [ ] `VerifierCB`（L2）：3 次 > 2s → open
  - [ ] `HookCB`（L3）：5 次 fail → open
  - [ ] `WorkerPanicCB`（L4）：1 次 panic → open
  - [ ] `SandboxExitCB`（L5）：5 次 fail → open
- [ ] 创建 `audit_log.go`：
  - [ ] `EscapeAuditLog` struct
  - [ ] `Record(ctx, decisions, final)` 
  - [ ] AuditLevel 0/1/2 区分
- [ ] 创建 `engine.go`：
  - [ ] `EscapeEngine` struct（tracker + chain + auditLog + circuitBreakers）
  - [ ] `Evaluate(ctx LoopContext) EscapeDecision`：
    - [ ] 收集 3 类决策（tracker + loopBudget + circuitBreaker）
    - [ ] 任一非 Continue → ChainedArbitrator.Arbitrate
    - [ ] auditLog.Record
  - [ ] 失败降级：Evaluate error → slog.Warn + Continue
- [ ] 创建 `engine_test.go` + `circuit_breaker_test.go` + `audit_log_test.go`：
  - [ ] `TestEscapeEngine_AllContinue` → Continue
  - [ ] `TestEscapeEngine_TrackerExceed` → ForceExit
  - [ ] `TestEscapeEngine_CircuitBreakerOpen` → LLM 仲裁
  - [ ] `TestEscapeEngine_AuditLog_Record` 验证 audit
  - [ ] 5 个 CircuitBreaker 各 1 个 test
- [ ] 单元测试 100% PASS
- [ ] 提交 + 开 PR

### 验收
- **AC4**：3 类深度限制整合
- **AC7**：CircuitBreaker 5 层接线
- **依赖**：T-03
- **风险**：中，与现有 5 metrics 重叠

---

## T-05: 5 节点接线 + 集成测试 + 文档同步 (PR-V5.5, 2 天)

### 任务
- [ ] 5 节点接线（核心工作）：
  - [ ] `sessionorchestrator/orchestrator.go`：
    - [ ] `ProcessMessage` 加 `escapeEngine` 字段
    - [ ] `WithEscapeEngine` option
    - [ ] 3 个接线点：
      - [ ] Plan 前：buildLoopContext + Evaluate
      - [ ] Execute 失败：updatePrevPlanKind + Evaluate
      - [ ] Verify 失败：updateFailureCriterion + Evaluate
  - [ ] `plan/planner.go`：MatchKind 后接 PlanKindSwitchPolicy
  - [ ] `workmodel/aggregate_verdicts.go`：AggregateVerdicts 后接 EscapeEngine
  - [ ] `learn/learner.go`：Learn 后接 EscapeEngine
- [ ] 单元测试（保持 PR 独立）：
  - [ ] `orchestrator_escape_test.go`：3 个接线点
  - [ ] `planner_switch_policy_test.go`：集成测试
- [ ] 集成测试（核心）：
  - [ ] `escape_integration_test.go`：
    - [ ] `TestIntegration_4DepthLimits` 4 scenarios
    - [ ] `TestIntegration_3LayerArbitration` 3 scenarios（mock LLM/Rule/Human）
    - [ ] `TestIntegration_5EscapeActions` 5 scenarios
    - [ ] `TestIntegration_PlanKindSwitchLimit` 累计 4 → OK, 5 → ForceExit
    - [ ] `TestIntegration_5NodePipeline_End2End` 完整 5 节点跑通
- [ ] 文档同步：
  - [ ] `openspec/specs/d7-orchestration/spec.md` v4.7.0 → v4.8.0
    - [ ] 新增 §5.x EscapeEngine 章节
    - [ ] 更新 Archived Changes
  - [ ] `openspec/specs/d7-orchestration/t-registry.md` v3.15.0 → v3.16.0
    - [ ] 新增 V5.1-V5.5 IMPLEMENTED T 点
  - [ ] `openspec/demand-archive-index.md` 加 DM-20260625-003 行
- [ ] 验证：
  - [ ] `go vet ./...` exit 0
  - [ ] `go test -race -count=1 ./internal/layers/orchestration/...` 100% PASS 0 race
  - [ ] `bash scripts/verify-archive.sh devrix-d7-mups-v5-escape-engine` 通过
  - [ ] 覆盖率 ≥ 80%
- [ ] 提交 + 开 PR

### 验收
- **AC5**：5 节点完整接线
- **AC6**：PlanKind 切换累计 ≤4 强制 ForceExit
- **AC8**：单元测试 100% PASS + 集成测试覆盖 + 0 race
- **依赖**：T-01 ~ T-04
- **风险**：中，5 节点接线改动面大

---

## 实施时间线

```
Day 1:     T-01 (LoopDepthTracker)                    ─┐
Day 1.5:   T-02 (PlanKindSwitchPolicy)                ─┼─→ Day 2 末 T-01 + T-02 完成
Day 2-3:   T-03 (ChainedArbitrator)                   ─┘   Day 3 末 T-03 完成
Day 4:     T-04 (EscapeEngine + CircuitBreaker)       ─┐
                                                        ├─→ Day 4 末 T-04 完成
Day 5-6:   T-05 (5 节点接线 + 集成测试 + 文档)         ─┘   Day 6 末 T-05 完成

Day 6.5:   S4-Gate review + CI + squash merge
```

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| R1：PlanKindSwitchPolicy 阈值（>4）估计值 | V5.5 加可配置常量 + 集成测试覆盖边界 |
| R2：3 层仲裁响应延迟 | LLM 5s timeout + Human 10s timeout 兜底 |
| R3：CircuitBreaker 与 5 metrics 重叠 | 显式选择 5 个升级为 circuit breaker，保留 state.cancels/handles 为纯 metric |
| R4：5 节点接线改动面大 | Evaluate error → slog.Warn + Continue 降级 |
| R5：Human 仲裁等待 | 不阻塞 ProcessMessage 同步返回（Phase 7 模式） |

## 提交规范

- 每个 PR 独立分支：`feat/devrix-d7-mups-v5-escape-engine-v5-1` 等
- 提交后立即本地验证：`go vet` + `go test -race` + `scripts/verify-archive.sh`
- 全部走 squash auto-merge
- 5 个 PR + 1 个 S6 archive PR（共 6 PR）

## References

- `proposal.md` §4 5 PR 拆分
- `design.md` §3-§9 devrix 落地映射
- doc 38 §21（400 行 v5 完整设计）
- 9 个 MUPS v4 归档
