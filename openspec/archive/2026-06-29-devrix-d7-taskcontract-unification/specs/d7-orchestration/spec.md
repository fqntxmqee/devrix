# D7 Orchestration Spec Delta — TaskContract 统一 (v6.0.x → v7.0.0)

**Change ID:** devrix-d7-taskcontract-unification
**Demand ID:** DM-20260629-006
**Delta Type:** ADDED（v6.0.0 → v7.0.0 演进起点）
**SOT:** `internal/layers/orchestration/interfaces/`（NEW） + `mups/{execute,learn}` + `workmodel/` + `decisionplanning/` + `escape/` + `hardening/`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. `interfaces.TaskSpec` struct + `interfaces.TaskReport` struct（双契约主类型）| `interfaces/{task_spec,task_report}.go` (NEW) | NEW | L1 接口层：Plan/Channel/WorkItem 三处创建点统一通过该 type 构造 |
| 2. `interfaces.DissentEntry` + `interfaces.Blockage` + `interfaces.CostActual` 子类型 | `interfaces/{dissent,blockage,cost}.go` (NEW) | NEW | L2 字段语义层：5+2 字段运行时语义 |
| 3. `interfaces.MVPArtifact` + `interfaces.HardEvidence` + `interfaces.Hash` 子类型 | `interfaces/{mvp_artifact,hard_evidence,version_chain}.go` (NEW) | NEW | L3 防御运行时层：Pessimistic Commit + Hard Evidence + CoW |
| 4. `interfaces.NewTaskSpec` / `NewTaskReport` 构造器 + `With*` 不可变 API | `interfaces/{task_spec,task_report}.go` (NEW) | NEW | 遵循 `coding.md §9` 不可变约定 |
| 5. `ChannelRequest` 升级为 `TaskSpec` | `mups/execute/channel.go` (MODIFIED) | L1 | Channel.Execute 入参强类型化 |
| 6. `LearnRequest` 升级为 `TaskReport` | `mups/learn/learner.go` (MODIFIED) | L1 | Learn 节点入参强类型化 |
| 7. 全量结果 → `TaskReport.Dissent` 字段填充（ExplorationChannel） | `mups/execute/exploration.go` (MODIFIED) | L2 | 少数派方案 + 否决理由沉淀 |
| 8. `WorkItem` 创建路径统一返回 `interfaces.TaskSpec` | `workmodel/workitem.go` (MODIFIED) | L1 | 三处创建点收敛 |
| 9. `Decomposer` 分解产出 `interfaces.TaskSpec` | `decisionplanning/decomposer.go` (MODIFIED) | L1 | 分解器出参强类型化 |
| 10. `CircuitBreaker` 5 层接入 `TaskReport.Blockage` 作为升级信号 | `escape/circuit_breaker.go` (MODIFIED) | L2 | 阻塞信号结构化 |
| 11. Pessimistic Commit 触发逻辑（资源耗尽 / EscapeForceExit）| `escape/pessimistic_commit.go` (NEW) | L3 | MVP + 风险警告不无限期挂起 |
| 12. Rule-based Fallback 候选规则（单测最多 / 编译通过 / 最小代价 / 最低不确定性）| `escape/rule_fallback.go` (NEW) | L3 | VERDICT 多轮 INDETERMINATE 降级 |
| 13. `WorkItem.VersionChain []Hash` 字段 + 父 Archive snapshot 只读 + Delta 追加 | `workmodel/version_chain.go` (NEW) + `workitem.go` (MODIFIED) | L3 | CoW 物化 + 可回滚 |
| 14. `ChildDownlink` 接收时相似度校验（embedding 哈希 + cosine 阈值）| `mups/execute/child_downlink.go` (MODIFIED) | L3 | 父 directive 与子 directive 相似度 > 80% 拦截 |
| 15. Verifier 拒绝 `Kind=Pass` 但 `TestCoveragePct == 0 && LogExcerpt == "" && ArtifactHash == ""` | `executionflow/verify/verifier.go` (MODIFIED) | L3 | 防"空证 Pass"共谋 |
| 16. span `convergence.feasible_space_width`（每次聚合后采样 W_up/W_down 比值）| `mups/observe/convergence.go` (NEW) | L4 | 发散-收敛单调性可观测化 |
| 17. `AdaptiveThreshold` 接入 `RunTurn`（解 TD-WT-01）| `sessionorchestrator/run_turn.go` (MODIFIED) | L4 | 不再需要三处 `map[string]interface{}` 推断 |
| 18. Layout guard `interfaces` 包 + TaskSpec/TaskReport 创建点合规检查 | `hardening/layout_guard.go` (NEW) | L4 | 跨包 import 合规 |
| 19. v6.0.x 类型别名（`type TaskSpec = interfaces.TaskSpecV1`）+ Deprecation warning | `mups/execute/{plan,channel}.go` (MODIFIED) | L4 | 1 minor 版本保留期 |
| 20. `openspec/specs/d7-orchestration/d7-domain.md` v2.6.0 → v3.0.0 同步 + `spec.md` 同步 | `d7-domain.md` + `spec.md` (MODIFIED) | L4 | spec 同步是 PR 合并前置 |
| 21. Feature Flag env-gated（AC11 Pessimistic + AC13 CoW 默认 `disabled`）| `hardening/feature_flag.go` (NEW) | L4 | 灰度 1% → 10% → 50% → 100% |
| 22. ORCH_* SentinelError 闭合（`ErrInterfacesTaskSpecInvalid` / `ErrPessimisticCommitTriggered` / `ErrSimilarityCollapseDetected` / `ErrHardEvidenceInsufficient`）| `internal/shared/errors/orch_*.go` (NEW) | L4 | Code + Message + Remediation 三元组 |
| 23. Cross-Domain Boundary tests（D2/D4/D6 三处消费点）| `interfaces/boundary_test.go` (NEW) | L4 | 每个跨域消费点必带 boundary test |

---

## 2. 双契约核心类型定义（L1 接口层）

```go
// interfaces/task_spec.go
package interfaces

// TaskSpec 统一契约：下行传播的 4 元组 + 2 元元数据
type TaskSpec struct {
    Goal              string                // 用户目标（必填）
    HardConstraints   []Constraint          // 硬约束（必填，≥ 0）
    SoftPreferences   []Preference          // 软偏好（可空）
    ConvergenceBudget ConvergenceBudget     // 收敛预算（必填）
    TraceID           string                // 工业级监控 ID（AC2 Gemini P4）
    CostBudget        CostQuota             // 资源预算（AC2 Gemini P4）
}

func NewTaskSpec(goal string) (*TaskSpec, error)            // Fail-fast: goal 空 → ErrInterfacesTaskSpecInvalid
func (s *TaskSpec) WithConstraint(c Constraint) *TaskSpec   // 不可变：返回新副本
func (s *TaskSpec) WithTraceID(id string) *TaskSpec
func (s *TaskSpec) WithCostBudget(q CostQuota) *TaskSpec
func (s *TaskSpec) Validate() error                         // 防御性：4 字段全校验

// interfaces/task_report.go
type TaskReport struct {
    Result       Result            // 产物结果（必填）
    Evidence     Evidence          // 证据（必填）
    Dissent      []DissentEntry    // 少数派报告（AC3：INDETERMINATE 时填充）
    Blockage     Blockage          // 阻塞信号（AC4：失败时填充）
    Resource     CostActual        // 资源消耗（AC5：从 ContextBudget 抽取）
    TraceID      string            // 与 TaskSpec.TraceID 1:1 对应
    CostActual   CostActual        // 元数据：实际消耗
    MVPArtifact  *MVPArtifact      // AC11：资源耗尽时输出 MVP
    HardEvidence *HardEvidence     // AC15：至少 1 项硬证据
    FallbackUsed bool              // AC12：是否走 Rule-based Fallback
    VersionChainHash Hash          // AC13：CoW 版本链哈希
}
```

---

## 3. ADDED Requirements

### Requirement: D7-S16 L1 Interface Layer — TaskSpec + TaskReport 双契约 ✅ PLANNED (PR-A)

TaskSpec / TaskReport MUST 作为 D7 域下行传播和上行反馈的统一接口。Plan / Channel / WorkItem 三处创建点 MUST 通过 `interfaces.NewTaskSpec()` 构造，Channel.Execute 输出 + Learn 节点输入 MUST 通过 `interfaces.NewTaskReport()` 构造。`With*` API MUST 遵守 `coding.md §9` 不可变约定（返回新副本，原值不变）。

**Priority:** P0
**Package:** `internal/layers/orchestration/interfaces/{task_spec,task_report,dissent,blockage,cost,mvp_artifact,hard_evidence,version_chain}.go` (NEW)
**T:** D7-S16-A01-T01, D7-S16-A02-T01
**Design:** `openspec/changes/devrix-d7-taskcontract-unification/design.md §3`

<!-- T: D7-S16-A01-T01 -->

#### Scenario: TaskSpec 4+2 字段构造 + 不可变 With*

- GIVEN 调用 `interfaces.NewTaskSpec("完成 v7.0 演进起点")`
- WHEN 连续调用 `.WithConstraint(c1).WithTraceID("trace-001").WithCostBudget(q1)`
- THEN 每次 `With*` 返回新 `*TaskSpec` 实例
- AND 原实例 `goal` / `hard_constraints` / `trace_id` / `cost_budget` 未被修改（immutability invariant）
- AND `len(hard_constraints) == 1`（追加非覆盖）
- AND `goal == "完成 v7.0 演进起点"`（必填字段保留）

- GIVEN `NewTaskSpec("")` （空 goal）
- WHEN 构造
- THEN 返回 `ErrInterfacesTaskSpecInvalid`（AC23 SentinelError）
- AND 错误 code 为 `ORCH_TASK_SPEC_INVALID`，message 含 "goal empty"，remediation 字段 "set non-empty goal"

#### Scenario: TaskReport 5+2 字段构造 + MVPArtifact 触发

- GIVEN 调用 `interfaces.NewTaskReport(result.Pass, evidence)` 无 dissent / blockage / resource
- WHEN 构造
- THEN `len(dissent) == 0 && blockage.IsZero() && resource.IsZero()` （零值合法）
- AND `mvp_artifact == nil` （未触发 Pessimistic Commit）

- GIVEN `escape_force_exit=true` 或 `budget_exhausted=true`
- WHEN Channel.Execute 包装返回 `NewTaskReport(...)`
- THEN `mvp_artifact != nil` 包含 `code_path` + `test_status` + `risk_warning` 3 字段
- AND `taskreport.dissent_recorded` span + `pessimistic.commit.emit` span 同步触发

<!-- T: D7-S16-A02-T01 -->

#### Scenario: TaskReport 构造 fail-fast 校验（ORCH_TASK_REPORT_INVALID）

- GIVEN 调用 `interfaces.NewTaskReport(result, evidence)` 其中 `result.Kind == ""` 或 `evidence.IsZero()`
- WHEN 构造
- THEN 返回 `ErrInterfacesTaskReportInvalid`（AC23 SentinelError）
- AND 错误 code 为 `ORCH_TASK_REPORT_INVALID`，remediation 字段 "set result.kind and provide evidence"

#### Scenario: 三处创建点统一通过 NewTaskSpec 构造（Layout Guard）

- GIVEN 启动 Layout guard `interfaces` 包合规检查（AC8）
- WHEN `grep -rE "Plan\{|ChannelRequest\{|WorkItem\{" internal/layers/orchestration/{mups/execute,workmodel,decisionplanning}/` 匹配到 0 处裸 struct literal
- AND `grep -rE "interfaces\.NewTaskSpec" internal/layers/orchestration/{mups/execute,workmodel,decisionplanning}/` ≥ 3 处
- THEN Layout guard PASS

---

### Requirement: D7-S17 L2 Field Semantics — Dissent / Blockage / Resource 三元素 ✅ PLANNED (PR-A)

TaskReport MUST 携带三个字段语义：`Dissent`（少数派方案 + 否决理由 + 否决者，触发条件 = VERDICT=INDETERMINATE 或 fallback_used=true）、`Blockage`（结构化 missing info / infeasible path / required external）、`Resource`（per-Plan token / time / step 消耗）。`Dissent.Reason` 与 `LogExcerpt` MUST 打 `Classification` 标签（`internal` / `confidential` / `secret`，AC20）。

**Priority:** P0（AC3） / P1（AC4, AC5）
**Package:** `internal/layers/orchestration/interfaces/{dissent,blockage,cost}.go` (NEW) + `mups/execute/{exploration,channel}.go` (MODIFIED) + `mups/learn/learner.go` (MODIFIED) + `hardening/context_budget.go` (MODIFIED)
**T:** D7-S17-A01-T01, D7-S17-A02-T01, D7-S17-A03-T01
**Design:** `openspec/changes/devrix-d7-taskcontract-unification/design.md §3 + §4`

<!-- T: D7-S17-A01-T01 -->

#### Scenario: Dissent 字段填充 — ExplorationChannel 全量保留 + Learn 节点沉淀

- GIVEN `VERDICT=INDETERMINATE` 或 `fallback_used=true`
- WHEN `mups/execute/exploration.go` 收集 3 个候选方案 A/B/C，A 胜出
- THEN `TaskReport.Dissent` 包含 2 个 `DissentEntry`（B + C）
- AND 每个 entry 字段：`plan_id` / `plan_kind` / `reason` / `rejecter`（= "verifier" | "user" | "rule_fallback"） / `classification`（= "internal"）
- AND `taskreport.dissent_recorded` span 触发，labels = `{verdict_kind: indeterminate, session_id_hash: ...}`

- GIVEN `fallback_used=false` 且 `VERDICT=PASS`
- WHEN 构造 TaskReport
- THEN `Dissent` 字段为 nil（不浪费存储）

#### Scenario: Dissent 沉淀至 SkillMemory.SOP（Learn 节点反馈闭环）

- GIVEN `TaskReport.Dissent` 含 2 个 entry（B + C 否决理由）
- WHEN `mups/learn/learner.go` 接收 TaskReport
- THEN 仅保留 top-3 entries（按 `reason` 长度降序），超过则 summary 哈希引用（"..." + `sha256(reasons)`）
- AND 沉淀为 `LearningSOP` 类型 Asset 至 `SkillMemory`
- AND `ReputationEvidence` Bayesian 更新：否决方案的 `rejection_count++`

<!-- T: D7-S17-A02-T01 -->

#### Scenario: Blockage 字段填充 — 驱动重规划决策

- GIVEN `VERDICT=FAIL` 且 `evidence.error.kind = "missing_input"` 或 `"infeasible_path"` 或 `"external_required"`
- WHEN `mups/execute/exploration.go` 包装 TaskReport
- THEN `Blockage` 字段非零
- AND `Blockage.Kind` ∈ {"missing_info", "infeasible_path", "required_external"}
- AND `Blockage.Detail` 字段保留原始 error message（不 sanitize，保留 review 价值）
- AND `taskreport.blockage_recorded` span 触发
- AND `CircuitBreaker.L3 (Hook)` 接收 `TaskReport.Blockage` 作为升级信号（触发 5 fail 阈值）

- GIVEN `VERDICT=FAIL` 但 error.kind 不在 3 类内
- WHEN 构造 TaskReport
- THEN `Blockage.Kind` 默认为 `infeasible_path`（fail-safe 兜底）

<!-- T: D7-S17-A03-T01 -->

#### Scenario: Resource 字段填充 — 从 ContextBudget Phase B 抽取

- GIVEN `hardening/context_budget.go` 已埋点 `d7.context.budget.phase_b.{mode,tokens,time_ms,steps}`
- WHEN `mups/execute/channel.go::Channel.Execute` 结束
- THEN 抽取 `taskreport.resource_recorded` span labels：`{tokens_used, time_ms, steps_taken, subagent_depth}`
- AND 写入 `TaskReport.Resource` 字段（CostActual struct）
- AND `CostActual.SubagentDepth <= 3`（ContextBudget Phase B MaxSubagentDepth 约束）

---

### Requirement: D7-S18 L3 Defense Runtime — 5 大防御机制 ✅ PLANNED (PR-B 低风险 + PR-C 高风险)

D7 MUST 实施 5 大防御机制：(1) Pessimistic Commit（资源耗尽 / EscapeForceExit 触发，输出 MVP + 风险警告）；(2) Hard Evidence（防"空证 Pass"共谋，至少 1 项硬证据）；(3) CoW Persistent（子层只读父 Archive + Commit 仅追加 Delta + VersionChain 支持回滚）；(4) Rule-based Fallback（VERDICT 多轮 INDETERMINATE 强制规则降级）；(5) Similarity Check（防递归塌陷，父 directive 与子 directive 相似度 > 80% 拦截）。

**Priority:** P0（AC11, AC13, AC15）/ P1（AC14）/ P0（AC12）
**Package:** `internal/layers/orchestration/escape/{pessimistic_commit,rule_fallback,similarity_check}.go` (NEW) + `workmodel/version_chain.go` (NEW) + `executionflow/verify/verifier.go` (MODIFIED) + `mups/execute/child_downlink.go` (MODIFIED)
**T:** D7-S18-A01-T01, D7-S18-A02-T01, D7-S18-A03-T01, D7-S18-A04-T01, D7-S18-A05-T01
**Design:** `openspec/changes/devrix-d7-taskcontract-unification/design.md §3 + §4`

<!-- T: D7-S18-A01-T01 -->

#### Scenario: Pessimistic Commit 触发 — 资源耗尽 / EscapeForceExit

- GIVEN `CircuitBreaker.L1 (DispatchLoop)` 触发 `budget_exhausted=true` 或 `escape_force_exit=true`（AC11 触发条件）
- WHEN `escape/pessimistic_commit.go::Evaluate` 被调用
- THEN 返回 `MVPArtifact` 包含 `code_path`（最后一个 commit hash） + `test_status`（last_passed/failed/none） + `risk_warning`（"资源耗尽，产物可能不完整"）
- AND `TaskReport.MVPArtifact != nil` 设置
- AND `TaskReport.Result.Kind` 强制为 `INDETERMINATE`（不允 Pass/Fail）
- AND `pessimistic.commit.emit` span 触发，labels = `{trigger_reason: budget_exhausted | escape_force_exit, session_id_hash: ...}`
- AND `pessimistic_commit_trigger_count{trigger_reason}` metric 计数 +1
- AND `pessimistic_commit_mvp_artifact_size{artifact_kind}` histogram 记录 artifact 大小

- GIVEN 资源尚有富余（`tokens_remaining > cost_budget.min_reserve`）
- WHEN Evaluate 被调用
- THEN 返回 nil（**不**触发 Pessimistic Commit，避免误降级）
- AND 走正常 Channel.Execute 完整路径

#### Scenario: Pessimistic Commit 输出保留 VersionChainHash（上游追溯）

- GIVEN `escape/pessimistic_commit.go::Evaluate` 触发并生成 MVPArtifact
- WHEN 包装 TaskReport
- THEN `TaskReport.VersionChainHash` 非零（指向父 Archive snapshot 哈希）
- AND Learn 节点收到 TaskReport 时可追溯到"产物不完整的上游版本"

<!-- T: D7-S18-A02-T01 -->

#### Scenario: Hard Evidence 拒绝"空证 Pass"防共谋

- GIVEN Verifier 准备输出 `Kind=Pass, Confidence=0.9`
- WHEN `executionflow/verify/verifier.go::evaluate` 校验 evidence
- THEN 若 `TestCoveragePct == 0 && LogExcerpt == "" && ArtifactHash == ""` 三项全为零值
- AND Verifier **拒绝**输出 Kind=Pass，强制降级为 `Kind=Indeterminate, Reason="insufficient_evidence"`
- AND `hard.evidence.reject` span 触发，labels = `{verifier_kind: code | chat, missing_evidence: "no_test|no_log|no_artifact_hash"}`
- AND `hard_evidence_reject_count{verifier_kind, missing_evidence}` metric 计数 +1
- AND 返回 `ErrHardEvidenceInsufficient`（AC23 SentinelError），remediation "provide at least one: test_coverage_pct, log_excerpt, artifact_hash"

- GIVEN chat/Q&A 任务（无 test）
- WHEN Verifier 校验 evidence
- THEN Verifier kind-specific 配置生效：`code` 任务要 test/log/artifact_hash；`chat` 任务要 `entity_hash` 或 `coherence_score`
- AND `chat` 任务允许仅 `entity_hash != ""` 通过

<!-- T: D7-S18-A03-T01 -->

#### Scenario: CoW VersionChain 追加 + 父 Archive 只读 + 回滚

- GIVEN `WorkItem.VersionChain []Hash` 初始为空，子层接收父 Archive snapshot
- WHEN 子层 Commit 一次状态变更
- THEN `WorkItem.VersionChain = append(WorkItem.VersionChain, sha256(delta))`
- AND 父 Archive snapshot 在子层视角下只读（`atomic.Pointer[Snapshot]` + 写时 COW）
- AND `worktree.versionchain.append` span 触发
- AND `worktree_versionchain_length{workitem_id_hash}` gauge 记录当前链长度

- GIVEN `worktree_versionchain_length > 10`（target ≤ 10 / session，AC13 性能目标）
- WHEN `workmodel/version_chain.go::gc` 周期任务触发（24h 一次）
- THEN 旧版本压缩归档（hash-only GC），链长度恢复 ≤ 10
- AND `worktree.versionchain.gc` span 触发

- GIVEN 子层需回滚到 VersionChain[N-3] 历史版本
- WHEN `WorkItem.RollbackTo(hash)` 被调用
- THEN O(1) hash 索引查表（map[Hash]*Snapshot）
- AND 当前 snapshot 替换为 VersionChain[N-3]
- AND `worktree_versionchain_length` 保留前 N-3 个 entry（N-2/N-1 截断）

<!-- T: D7-S18-A04-T01 -->

#### Scenario: Rule-based Fallback 候选规则可插拔 + A/B test

- GIVEN `VERDICT=INDETERMINATE` 连续 ≥ 3 轮（AC12 触发条件）
- WHEN `escape/rule_fallback.go::Evaluate` 被调用
- THEN 4 个候选规则按 `env.RuleFallbackStrategy` 选择（默认 `min_uncertainty`）：
  - `most_tests_passed`：单测通过数最多
  - `compiled_clean`：编译通过
  - `min_cost`：执行 step 最少
  - `min_uncertainty`：UncertaintyReport.Anomalies 数量最少
- AND 选中规则后强制落 `Verdict.Pass`（非 Indeterminate）+ `TaskReport.FallbackUsed=true`
- AND `taskreport.dissent_recorded` span 同步触发（即使 Pass 也记录 Dissent 决策过程）
- AND `fallback_used=true` 标记至 metrics + AuditLog

- GIVEN `env.RuleFallbackStrategy="none"`（禁用 fallback）
- WHEN VERDICT 连续 INDETERMINATE
- THEN 走 Pessimistic Commit 路径（而非 fallback）

<!-- T: D7-S18-A05-T01 -->

#### Scenario: Similarity Check 拦截递归塌陷

- GIVEN 父 directive `goal="实现 v7.0 演进起点"` 与子 directive `goal="完成 v7.0 演进起点"`
- WHEN `mups/execute/child_downlink.go::Validate` 接收子 downlink
- THEN 计算 embedding 哈希 + cosine 相似度
- AND 若相似度 > 0.80（threshold），拦截并返回 `ErrSimilarityCollapseDetected`（AC23 SentinelError）
- AND `similarity.check.intercept` span 触发，labels = `{intercept_reason: parent_child_similarity_high}`
- AND `similarity_check_intercept_count{intercept_reason}` metric 计数 +1
- AND 触发 Refine：要求子层重新拆解（`Refine()` 方法）

- GIVEN 相似度 ∈ [0.70, 0.80]（边界区）
- WHEN Validate 接收
- THEN 升级至 LLM 二次校验（避免哈希碰撞误判）
- AND 二次校验结果覆盖哈希判定

- GIVEN 相似度 ≤ 0.70
- WHEN Validate 接收
- THEN 通过（O(1) embedding 缓存命中）

---

### Requirement: D7-S19 L4 Governance — 13 横切治理项 ✅ PLANNED (PR-B + PR-C)

D7 MUST 实施 13 项治理横切：可观测化（convergence span、AdaptiveThreshold wiring、Layout guard）、验证（race test、LP 回归）、迁移（type alias lifecycle、Spec 同步、Coverage、Performance、Security Classification）、横切（Cross-Domain Boundary、Feature Flag 灰度、Error Code 闭合）。

**Priority:** P0 × 9 / P1 × 3 / P2 × 1
**Package:** `mups/observe/convergence.go` (NEW) + `sessionorchestrator/run_turn.go` (MODIFIED) + `hardening/{layout_guard,feature_flag}.go` (NEW) + `internal/shared/errors/orch_*.go` (NEW) + `interfaces/boundary_test.go` (NEW)
**T:** D7-S19-A01..A11-T01（11 个活动对应 11+ 个 T 点）
**Design:** `openspec/changes/devrix-d7-taskcontract-unification/design.md §5 + §6 + §7`

<!-- T: D7-S19-A06-T01 -->

#### Scenario: convergence.feasible_space_width span 采样 W_up/W_down 比值

- GIVEN `mups/observe/aggregate.go` 完成一轮 Observation 聚合（Observe 节点）
- WHEN 聚合结束后
- THEN 采样 `feasible_space_width_upstream`（候选方案数 W_up）与 `feasible_space_width_downstream`（收敛后方案数 W_down）
- AND 写入 span `convergence.feasible_space_width`，labels = `{ratio: W_down/W_up, round: N, session_id_hash: ...}`
- AND `ratio < 1.0` 表示发散-收敛（设计目标）
- AND `ratio > 1.0` 触发告警（异常发散）

<!-- T: D7-S19-A10-T01 -->

#### Scenario: AdaptiveThreshold 接入 RunTurn（解 TD-WT-01）

- GIVEN `SessionOrchestrator.RunTurn` 当前需 3 处 `map[string]interface{}` 推断（Plan / Channel / WorkItem）
- WHEN `interfaces.NewTaskSpec` 构造后传入 RunTurn
- THEN 强类型读取：`spec.Goal` / `spec.HardConstraints` / `spec.ConvergenceBudget`
- AND `AdaptiveThreshold.Adjust(spec.ConvergenceBudget)` 类型安全（不再需要 reflection）
- AND `TD-WT-01` 标记为 RESOLVED
- AND `interfaces_task_spec_coverage{call_site: "plan.New" | "channel.New" | "workitem.New"}` gauge = 1.0（3 处全部覆盖）

<!-- T: D7-S19-A11-T01 -->

#### Scenario: Layout guard interfaces 包 + TaskSpec/TaskReport 创建点合规

- GIVEN 启动 `hardening/layout_guard.go::CheckInterfacesPackage`（CI gate）
- WHEN 扫描 `internal/layers/orchestration/` 跨包 import
- THEN `import "internal/layers/orchestration/interfaces"` 仅允许在白名单包：`mups/execute` / `mups/learn` / `workmodel` / `decisionplanning` / `escape` / `hardening` / `sessionorchestrator` / `executionflow` / `d7-bootstrap`
- AND interfaces 包自身 0 import D7 子包（Pure types 原则，AC23 防止循环依赖）
- AND TaskSpec 创建点 100% 通过 `interfaces.NewTaskSpec()`（基于 grep + go vet 规则）

#### Scenario: 22/22 orchestration packages go test -race PASS（AC9 验证）

- GIVEN 22 个 orchestration 子包（v6.0.x 已就位）
- WHEN PR-B/PR-C 合并后运行 `go test ./internal/layers/orchestration/... -race -count=1`
- THEN 22/22 PASS（0 FAIL / 0 SKIP）
- AND `d7-test-coverage.sh` CI gate 通过

#### Scenario: LP-1 / LP-2 / LP-5 100% 兼容（AC10 验证）

- GIVEN LP-1（Learn × 3 Pass → Bayesian Alpha=3）回归测试 + LP-2（PendingAsset ScheduledMemory 隔离） + LP-5（Plan.SourceObservationIDs 反向追溯）
- WHEN PR-B/PR-C 合并后
- THEN 3 个集成测试 PASS（无回归）
- AND `TestAutoClose_FullLP1Loop` + `TestIntegration_5NodePipeline_End2End` + `TestLPReverseTraceability` 全绿

<!-- T: D7-S19-A01-T01 -->

#### Scenario: Migration Plan v6.0.x 类型别名 1 minor 版本保留 + Deprecation warning

- GIVEN v6.0.x `mups/execute.Plan` / `ChannelRequest` / `LearnRequest` 三个老 struct
- WHEN PR-B 合并时
- THEN 添加类型别名：`type Plan = interfaces.TaskSpecV1`（v6.0.x 兼容）
- AND `go build` 输出 deprecation warning：`Plan is deprecated, use interfaces.TaskSpecV1 (will remove in v8.0)`，仅 1 次/包
- AND `openspec/specs/d7-orchestration/d7-domain.md` §Deprecation Lifecycle 加 v8.0 移除计划

- GIVEN v7.0 阶段新代码
- WHEN 调用老别名
- THEN `staticcheck` 规则拦截（CI gate）

<!-- T: D7-S19-A02-T01 -->

#### Scenario: Spec 文档同步（d7-domain.md v3.0.0 + spec.md v7.0.0）

- GIVEN PR-A 合并时
- THEN `openspec/specs/d7-orchestration/d7-domain.md` v2.6.0 → v3.0.0（新增 §TaskContract 章节）
- AND `openspec/specs/d7-orchestration/spec.md` v4.10.0 → v7.0.0（新增 D7-S16/S17/S18/S19 4 个 Requirement）
- AND `openspec/specs/d7-orchestration/t-registry.md` 加 D7-S16-A01-T01..S19-A11-T01 共 24+ T 行
- AND `openspec/t-registry.md` (root) 同步
- AND CI gate：`git diff --stat openspec/specs/d7-orchestration/ | grep -v "spec.md|domain.md"` 必须为 0（除非有显式 reason）

<!-- T: D7-S19-A03-T01 -->

#### Scenario: Test Coverage ≥ 80%（AC18）

- GIVEN PR-C 合并时
- WHEN 运行 `d7-coverage-report.sh`（在 `scripts/` 下新增）
- THEN `internal/layers/orchestration/interfaces/` 行覆盖率 ≥ 80%
- AND `internal/layers/orchestration/workmodel/version_chain.go` ≥ 80%
- AND `internal/layers/orchestration/escape/{pessimistic_commit,rule_fallback,similarity_check}.go` ≥ 80%
- AND 总体覆盖率不下降（delta ≥ -2% 可接受，<-2% 触发 S4-Gate 拒绝）

<!-- T: D7-S19-A04-T01 -->

#### Scenario: Performance Budget — TaskSpec/TaskReport 构造 P99 < 1ms + VersionChain O(1)

- GIVEN PR-C 合并时
- WHEN 运行 `go test -bench=BenchmarkTaskSpecNew -benchmem -count=10` + `BenchmarkVersionChainLookup -count=10`
- THEN `BenchmarkTaskSpecNew` P99 < 1ms（`go test -benchtime=10s` 后 benchstat 处理）
- AND `BenchmarkVersionChainLookup` O(1) — `ns/op` 与链长度无关
- AND `BenchmarkSimilarityCheck` embedding 命中 O(1) — 缓存命中 ns/op < 1µs

<!-- T: D7-S19-A05-T01 -->

#### Scenario: Security Classification — Dissent.Reason + LogExcerpt 标签化

- GIVEN `TaskReport.Dissent[].Reason` 字段 + `TaskReport.HardEvidence.LogExcerpt` 字段
- WHEN 构造时
- THEN 必须打 `Classification` 标签 ∈ {"internal", "confidential", "secret"}
- AND Learn 节点沉淀时按标签过滤：secret 不写入 SkillMemory（仅 ScheduledMemory 暂存）
- AND `interfaces_test.go::TestSecurityClassificationFilter` 验证 3 种标签行为
- AND Review 期抽样 100 条人工核对（手动 gate，不进 CI）

<!-- T: D7-S19-A09-T01 -->

#### Scenario: Cross-Domain Boundary — D2 / D4 / D6 消费点 boundary test

- GIVEN TaskSpec 在 3 个跨域消费点：D2（context budget 注入）/ D4（multi-agent worker consume）/ D6（evolution observer）
- WHEN PR-B 合并时
- THEN `interfaces/boundary_test.go` (NEW) 包含 3 个 boundary test：
  - `TestBoundary_D2_ConsumeTaskSpec`：D2 读取 `TaskSpec.Goal` + `ConvergenceBudget`，写入 context budget
  - `TestBoundary_D4_ConsumeTaskSpec`：D4 worker 读取 `TaskSpec.HardConstraints`，阻塞违反约束
  - `TestBoundary_D6_ConsumeTaskSpec`：D6 observer 读取 `TaskReport.Result` + `Dissent`，advisory 校验
- AND CI grep：`grep -rln "orchestration/interfaces" internal/layers/{contextengine,multiagent,evolution}/` 必须对账到 `boundary_test.go` 文件

<!-- T: D7-S19-A07-T01 -->

#### Scenario: Feature Flag env-gated 灰度 1% → 10% → 50% → 100%

- GIVEN AC11 (Pessimistic Commit) + AC13 (CoW VersionChain) 必须 env-gated
- WHEN PR-B 合并时
- THEN `hardening/feature_flag.go` (NEW) 暴露 `IsPessimisticCommitEnabled() bool` + `IsCoWVersionChainEnabled() bool`
- AND 默认 `disabled`（env: `D7_FEATURE_PESSIMISTIC_COMMIT=0`）
- AND 灰度节奏脚本：`./scripts/devrix.sh rollout-flag pessimistic_commit 1` → `10` → `50` → `100`，每步观察 24h
- AND `RolloutDisable()` 灰度失败自动回滚
- AND CI test：`D7_FEATURE_PESSIMISTIC_COMMIT=0` 与 `=1` 两套测试均 PASS（双轨测试）

<!-- T: D7-S19-A08-T01 -->

#### Scenario: Error Code 闭合 — ORCH_* SentinelError 三元组

- GIVEN AC11-AC15 新增错误（`ErrPessimisticCommitTriggered` / `ErrRuleFallbackSelected` / `ErrSimilarityCollapseDetected` / `ErrHardEvidenceInsufficient` / `ErrVersionChainBroken` / `ErrInterfacesTaskSpecInvalid` / `ErrInterfacesTaskReportInvalid`）
- WHEN PR-B 合并时
- THEN `internal/shared/errors/orch_*.go` (NEW) 7 个文件定义
- AND 每个 SentinelError 满足三元组：`Code`（`ORCH_*` 前缀） + `Message`（人类可读） + `Remediation`（修复建议）
- AND CI gate：`grep -rE "errors\.New|fmt\.Errorf" --include="*.go" internal/layers/orchestration/interfaces/ internal/layers/orchestration/escape/ | grep -v "ORCH_"` 必须 0 命中
- AND `interfaces/errors_test.go` (NEW) 验证 7 个错误类型注册

---

## 4. Span 增量（v6.0.0 → v7.0.0）

| Span | Op | 归属 | AC |
|------|----|----|-----|
| `interfaces.task_spec.created` | `d7.s16.interfaces.task_spec.created` | sessionSpan child | AC1 |
| `interfaces.task_report.created` | `d7.s16.interfaces.task_report.created` | sessionSpan child | AC2 |
| `taskreport.dissent_recorded` | `d7.s17.taskreport.dissent_recorded` | sessionSpan child | AC3 |
| `taskreport.blockage_recorded` | `d7.s17.taskreport.blockage_recorded` | sessionSpan child | AC4 |
| `taskreport.resource_recorded` | `d7.s17.taskreport.resource_recorded` | sessionSpan child | AC5 |
| `pessimistic.commit.emit` | `d7.s18.pessimistic.commit.emit` | sessionSpan child | AC11 |
| `hard.evidence.reject` | `d7.s18.hard.evidence.reject` | sessionSpan child | AC15 |
| `worktree.versionchain.append` | `d7.s18.worktree.versionchain.append` | sessionSpan child | AC13 |
| `worktree.versionchain.gc` | `d7.s18.worktree.versionchain.gc` | sessionSpan child | AC13 |
| `similarity.check.intercept` | `d7.s18.similarity.check.intercept` | sessionSpan child | AC14 |
| `convergence.feasible_space_width` | `d7.s19.convergence.feasible_space_width` | sessionSpan child | AC6 |

新增 11 个 span，与 `.openspec.yaml` `span_naming` 字段一致。

---

## 5. Metrics 增量（v6.0.0 → v7.0.0）

| Metric | Type | Labels | AC |
|--------|------|-------|-----|
| `interfaces_task_spec_coverage` | gauge | `call_site` | AC1 / AC7 |
| `taskreport_dissent_recorded_count` | counter | `verdict_kind`, `session_id_hash` | AC3 |
| `pessimistic_commit_trigger_count` | counter | `trigger_reason`, `session_id_hash` | AC11 |
| `pessimistic_commit_mvp_artifact_size` | histogram | `artifact_kind` | AC11 |
| `worktree_versionchain_length` | gauge | `workitem_id_hash` | AC13 |
| `similarity_check_intercept_count` | counter | `intercept_reason` | AC14 |
| `hard_evidence_reject_count` | counter | `verifier_kind`, `missing_evidence` | AC15 |
| `cross_domain_boundary_test_count` | gauge | `domain_pair` | AC21 |

8 个新增 metrics，与 `.openspec.yaml` `metrics_definitions` 字段一致。

---

## 6. 行为不变保证

- **Plan / ChannelRequest / LearnRequest 旧调用方**：v6.0.x 类型别名保留 1 minor 版本（v7.0.x 期间）+ 编译时 deprecation warning
- **UncertaintyCoord / Verdict / ChildDownlink 现有结构**：完全不变（向后兼容）
- **4 PlanKind → 4 Channel 路由**：不变
- **5 层 CircuitBreaker 阈值**：不变（仅在 L3 Hook 层接入 TaskReport.Blockage 作为升级信号）
- **MUPS 5 节点管道**：不变（Dissent 沉淀作为 Learn 节点增强，非破坏性）
- **LP-1 / LP-2 / LP-5 闭环**：100% 兼容（AC10 验证）

---

## 7. 跨域边界影响

| 域 | 影响 | 边界测试 |
|----|------|----------|
| D1 通信层 | 0 变化 | — |
| D2 上下文引擎 | TaskSpec 在 context budget 注入点消费 | `boundary_test.go::TestBoundary_D2_ConsumeTaskSpec` |
| D3 LLM Gateway | 0 变化 | — |
| D4 多智能体 | TaskSpec.HardConstraints 在 worker consume 时校验 | `boundary_test.go::TestBoundary_D4_ConsumeTaskSpec` |
| D5 可观测性 | 11 新 span + 8 新 metrics | span-registry.md + t-registry.md 同步 |
| D6 演化层 | TaskReport.Result + Dissent advisory 校验 | `boundary_test.go::TestBoundary_D6_ConsumeTaskSpec` |
| **D7 编排层** | **6 子包（interfaces/mups/execute/mups/learn/workmodel/decisionplanning/escape/hardening）+ 3 子包（sessionorchestrator/executionflow/d7-bootstrap）修改** | 22/22 orchestration packages -race PASS（AC9） |

---

## 8. Out of Scope（本 Change 不做）

- Reference Adapter：`TaskSpec → plan.Plan` / `TaskReport → Artifact` 参考实现 — 留作 v7.0.x 维护期（Reference Adapter follow-up Change）
- Operator Runbook：fallback / collapse 触发运维手册 — 与 hardening/metrics.go 配套（DM-20260629-006 follow-up）
- Interface Semver：`interfaces/v2` 子包路径，便于未来 v2 演进 — v8.0 规划
- 多租户 TaskSpec 隔离（task_tenant_id 字段） — 与 v7.0.x 维护期补齐
- TaskSpec 加密传输（TLS 双向认证 + mTLS） — 与 D6 evolution/security Change 联动

---

## 9. 关联变更

- **devrix-d7-dsaft-restructuring (DM-20260629-001)**：v6.0.x 维护阶段收官，本 Change 是 v7.0 演进起点
- **devrix-d7-mups-v5-escape-engine (DM-20260625-003)**：EscapeEngine + CircuitBreaker L0-L5，本 Change 在 L1/L3 接入 Pessimistic Commit + Blockage
- **devrix-d7-certainty-architecture**：UncertaintyCoord [0,1] 数值契约 + VERDICT 4 态，本 Change 复用作为 Dissent 触发条件
- **Context Budget Phase A+B (DM-20260620-001)**：3-mode brief/fork/full + MaxSubagentDepth=3，本 Change 复用作为 Resource 字段基础
- **2026-06-29 Gemini 工程实践 review**：4 点补充（降级收敛/CoW 物化/防御性/接口硬化）已映射为 AC11-AC15
