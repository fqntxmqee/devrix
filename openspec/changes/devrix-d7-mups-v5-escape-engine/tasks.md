# Tasks — devrix-d7-mups-v5-escape-engine (DM-20260625-003)

**Change ID:** `devrix-d7-mups-v5-escape-engine`
**Demand ID:** DM-20260625-003
**Sprint:** mups-v5
**Total Effort:** 6.5 天
**PR Count:** 5

---

## 任务总览

| Task ID | 内容 | 工作量 | 依赖 | PR | AC | L1 测试 |
|---------|------|--------|------|-----|-----|--------|
| T-01 | LoopDepthTracker v2 | 1 天 | doc 38 §19.2 | V5.1 | AC1 | 10 + 1 gap = 11 |
| T-02 | PlanKindSwitchPolicy 3 档 | 0.5 天 | 无 | V5.2 | AC2 | 15 |
| T-03 | ChainedArbitrator 3 层 | 2 天 | T-01 | V5.3 | AC3 | 35 + 1 gap = 36 |
| T-04 | EscapeEngine 整合 + CircuitBreaker 5 层 | 1 天 | T-03 | V5.4 | AC4, AC7 | 16 + 6 gap = 22 |
| T-05 | 5 节点接线 + 集成测试 + 文档同步 | 2 天 | 全部 | V5.5 | AC5, AC6, AC8 | 14 + 5 gap = 19 |
| **总计** | | **6.5 天 + 2.2 gap = 8.7 天** | | | 8 AC | **90 L1 + 13 gap = 103 L1** |

> **测试用例全面性**：详见下文 `## 测试用例全面性设计（4 层金字塔）` 章节，共 121 个测试（L4 4 + L3 7 + L2 7 + L1 103），每条含 4 必填字段（业务目标/删除后果/攻击者视角/金字塔层）。

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
  - [ ] `TestPlanKindSwitchCount_ZeroStart` 0 次切换 → OK（首次切换合法）
  - [ ] `TestPlanKindSwitchPolicy_Forbidden_NoSwitch` CommitmentPlan 0 次切换 → OK
  - [ ] `TestPlanKindSwitchPolicy_Forbidden_OneSwitch` CommitmentPlan 1 次切换 → ForceExit
  - [ ] `TestPlanKindSwitchPolicy_Constrained_Boundary` Constrained 4 次切换 → OK, 5 → ForceExit
  - [ ] `TestPlanKindSwitchPolicy_Allowed_NoLimit` Allowed 100 次切换 → OK（无上限）
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
  - [ ] `EscapeAction` enum（**6 类** 含新增 `EscapePendingHuman` 中间态）
  - [ ] `EscapeDecision` struct（**5 字段** 含 `PendingID`）
  - [ ] `EscapeArbitrator` interface
  - [ ] `LLMArbitrator`：
    - [ ] 调 LLM（注入 prompt + PlanKindSwitchCount）
    - [ ] 5s timeout（context.WithTimeout）
    - [ ] Continue / Exit 二选一
  - [ ] `RuleArbitrator`：
    - [ ] 检查 `hasUnrecoverableFailure(ctx)`
    - [ ] 不可恢复 → AbortWithAudit (AuditLevel=2)
    - [ ] 可恢复 → EscalateToHuman
  - [ ] `HumanArbitrator`（**异步化，详见 T-03.1**）：
    - [ ] 注册 pendingID + 异步通知 user
    - [ ] 启动 10s timer goroutine
    - [ ] 立即返回 `EscapePendingHuman`（不阻塞）
    - [ ] user 提前响应 → 应用 choice
    - [ ] 10s timeout → ForceExit 兜底
    - [ ] ctx 取消 → ForceExit 兜底
  - [ ] `ChainedArbitrator`（3 层链式）
- [ ] 创建 `notifier.go`（**新增 T-03.1 任务**）：
  - [ ] `Notifier` interface
  - [ ] `FeishuCardNotifier`（dev 默认，3 按钮 A/B/C + ExpiresAt 10s）
  - [ ] `CLINotifier`（terminal 降级 fallback）
  - [ ] `EmailNotifier`（可选，备用）
  - [ ] `ChainedNotifier`（Feishu → CLI → Email 链式 fallback）
- [ ] 创建 `pending_resolution.go`（**新增 T-03.1 任务**）：
  - [ ] `PendingResolutionStore` interface
  - [ ] `InMemoryPendingResolutionStore`（dev 默认）
  - [ ] `DBPendingResolutionStore`（生产可换 DB/Redis）
  - [ ] `Save / Load / Delete` 3 方法
- [ ] 创建 `arbitrator_test.go`：
  - [ ] `TestLLMArbitrator_Continue` mock LLM 选 Continue
  - [ ] `TestLLMArbitrator_Exit` mock LLM 选 Exit
  - [ ] `TestLLMArbitrator_Timeout` 5s 超时 → ForceExit
  - [ ] `TestRuleArbitrator_Unrecoverable` → AbortWithAudit
  - [ ] `TestRuleArbitrator_Recoverable` → EscalateToHuman
  - [ ] `TestHumanArbitrator_ChoiceA` SubmitUserChoice("A") → Continue
  - [ ] `TestHumanArbitrator_ChoiceB` SubmitUserChoice("B") → ForceExit
  - [ ] `TestHumanArbitrator_ChoiceC` SubmitUserChoice("C") → AbortWithAudit
  - [ ] `TestHumanArbitrator_Timeout` 10s 不响应 → ForceExit
  - [ ] `TestHumanArbitrator_CtxCancel` ctx 取消 → ForceExit
  - [ ] `TestHumanArbitrator_PendingResolution_Save` Save → Load 命中
  - [ ] `TestHumanArbitrator_PendingResolution_Load_NotFound` 首次 ProcessMessage
  - [ ] `TestHumanArbitrator_SubmitUserChoice_Expired` pendingID 已 cleanup → 丢弃
  - [ ] `TestHumanArbitrator_NotBlockProcessMessage` Arbitrate 立即返回 <100ms
  - [ ] `TestChainedArbitrator_LLMContinue` LLM 选 Continue → 立即返回
  - [ ] `TestChainedArbitrator_LLMExit_Rule` LLM Exit → Rule
  - [ ] `TestChainedArbitrator_RuleHuman` Rule Human → Human
  - [ ] `TestChainedArbitrator_HumanPending` → 返回 EscapePendingHuman
- [ ] 创建 `notifier_test.go`：
  - [ ] `TestFeishuCardNotifier_BuildCard` 验证卡片结构（3 按钮 + ExpiresAt）
  - [ ] `TestChainedNotifier_FeishuSuccess` Feishu 成功 → 不 fallback
  - [ ] `TestChainedNotifier_FeishuFail_CLISuccess` Feishu 失败 → CLI 成功
  - [ ] `TestChainedNotifier_AllFail` 全部失败 → 返回 error
- [ ] 单元测试 100% PASS
- [ ] 提交 + 开 PR

### 验收
- **AC3**：3 层仲裁（LLM/Rule/Human）+ 6 类 EscapeAction（含 EscapePendingHuman 中间态）
- **依赖**：T-01
- **风险**：中，timeout 兜底 + Human 异步化 + Notifier 链式 fallback

### T-03.1: HumanArbitrator 异步化实现要点（PR-V5.3 子任务）

**核心约束**（与 Phase 7 Auto-Close 协同）：
- `ProcessMessage` 是**同步接口**（飞书卡片立即显示）
- HumanArbitrator **不能同步等待 user 响应**（10s 阻塞会破坏飞书卡片体验）
- 必须**异步注册 + 立即返回 + goroutine 兜底**

**3 个关键设计决策**：
- **D1**：Evaluate 同步 vs 异步？ → **同步返回 + 内部异步**（`EscapePendingHuman` 中间态）
- **D2**：user 响应后 decision 怎么应用？ → **audit 持久化 + 下次 ProcessMessage 续跑**
- **D3**：notifyUser 通道？ → **可插拔 `Notifier` interface**（FeishuCard 默认 + CLI fallback）

**4 类边界场景兜底**：
| 场景 | 行为 | 兜底机制 |
|------|------|---------|
| 10s 内 user 点 A/B/C | 应用 user choice | 正常路径 |
| 10s 内 user 不响应 | EscapeForceExit + AuditLevel=2 | 10s timer 兜底 |
| ProcessMessage 客户端断开 | EscapeForceExit + AuditLevel=2 | ctx.Done() 兜底 |
| user 响应但 pendingID 已 timeout/cleanup | SubmitUserChoice 丢弃 | map cleanup 兜底 |
| Notifier 发送失败 | 降级为 CLI prompt | Notifier 链式 fallback |
| LLMArbitrator 自身 panic | recover + ForceExit | 失败降级（同 Phase 7 模式）|

**详细 Go 骨架**：见 `design.md` §5.3.1
**完整决策流程**：见 `design.md` §5.3.2

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

## 测试用例全面性设计（4 层金字塔）

### 设计原则

**核心立场**：测试用例本质上从"v5 定位和目标"出发，**不能仅限于代码覆盖**。v5 的核心防御价值（防 LLM 操纵 PlanKind 绕过回路深度、防 Human 异步化破坏飞书卡片体验、防 CB 5 层漏接）只有用业务/攻击者视角才能设计出守护承诺的测试。

#### 4 层金字塔（business-driven，非 code-driven）

| 层 | 数量 | 守护对象 | 漏测影响 | 覆盖 |
|----|------|---------|---------|------|
| **L4 业务验收** | 4 | v5 立项的 4 大业务承诺 | doc 38 §21.2 漏洞重现、Phase 1-7 兼容破坏、飞书卡片体验崩溃、性能退化 | proposal.md §3 + design.md §3 |
| **L3 端到端场景** | 7 | 7 个关键故障链路（PlanKind 切换/同模式/异常检测/性能降级/Human 异步/策略约束/CB 独立）| 7 类具体 P0 bug | 5 节点 + 5 接线点 + 跨模块 |
| **L2 集成** | 6 | 4 类深度限制 + 3 层仲裁 + 5 类兜底动作 + 5 节点 + 5 接线点 | 跨 PR 集成回归 | 跨 PR 协同 |
| **L1 单元** | 89 | 状态机/分支/边界/错误/并发/降级 | 单点 bug | 单模块函数/方法 |

#### 4 必填字段（每条测试都必须有）

- **业务目标**：这个测试守护什么业务承诺？（一句话锚定到 v5 §21.x 需求）
- **删除后果**：如果删了这条测试，会漏掉哪个具体 bug？（回归时无感知）
- **攻击者视角**：LLM/Rule/Human/CB 异常/外部输入下被绕过？（v5 防御价值）
- **金字塔层**：L1/L2/L3/L4（决定测试位置 + 维护优先级）

#### 设计原则（避免常见误区）

- **业务驱动 > 代码驱动**：不是"测了什么函数"而是"守护了什么承诺"
- **守护承诺 > 验证实现**：删了不破坏 = 没守护（重构时也得跑）
- **跨层引用 > 单点测试**：L3 引用 L2/L1（避免重复 + 加快定位）
- **攻击者视角 > happy path**：v5 核心防御价值（防 LLM 操纵、防逃逸）
- **零破坏性兼容 > 增强功能**：L4-02 守护 v5 不破坏 Phase 1-7

---

### L4 业务验收（4 个）

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L4-01 | `TestL4_v5_Solves_Sec21_2_Vulnerability` | 守护 v5 立项"最核心承诺"——不再让 LLM 通过 PlanKind 切换绕过回路深度（doc 38 §21.2 关键漏洞）| 如果删了，doc 38 §21.2 漏洞（回路深度计数可被 LLM 操纵）就无人守护，回归时无法发现；sess_xxx 单 ProcessMessage 消耗 token 100k+、飞书卡片超时 | 恶意 LLM 切换 4 次 PlanKind 后能否继续？4 次以上能否继续？边界 4/5 次呢？ | L4 |
| L4-02 | `TestL4_v5_Compatible_With_Phase1_7` | 守护 v5 "叠加而非取代" Phase 1-7 的核心承诺（design §8 + proposal §3.5）| 如果删了，v5 落地可能误覆盖 Phase 4 14 ExitReason、Phase 6 buildObserveRequest 3 层 fail-safe、Phase 7 Auto-Close 等关键能力 | v5 接线是否误吞了 Phase 1-7 的 ExitReason？Evaluate error 降级是否破坏了 Auto-Close 同步返回？ | L4 |
| L4-03 | `TestL4_v5_PerformanceOverhead_Under5Percent` | 守护 v5 性能承诺——不破坏飞书卡片体验（每 ProcessMessage 增加 < 5% 延迟）| 如果删了，性能回归时无感知，3 层仲裁 + CB 5 层可能拖慢主链路 | Evaluate 调用 5 次/ProcessMessage + 5 个 CB 状态查询 = 总开销？LLM 仲裁 5s timeout 兜底对正常路径影响？ | L4 |
| L4-04 | `TestL4_FeishuCard_NotBlocked_ByHuman10s` | 守护 v5 HumanArbitrator 异步化承诺（design §5.3.1 + tasks §T-03.1）——飞书卡片立即显示，不被 10s 等待阻塞 | 如果删了，HumanArbitrator 可能误改回同步等待，ProcessMessage 阻塞 10s，飞书卡片体验崩溃 | 用户点 A/B/C 前飞书卡片是否已显示？10s 不响应是否兜底 ForceExit？客户端断开是否同步覆盖卡片？ | L4 |

---

### L3 端到端场景（7 个）

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 覆盖 PR |
|---|---------|---------|---------|-----------|---------|---------|
| L3-01 | `TestL3_LLM_SwitchesPlanKind_5Times_ForcesExit` | 守护"PlanKind 切换累计 ≤ 4"硬约束（doc 38 §21.4.2）——防止 LLM "试探模式"无限循环 | 如果删了，PlanKindSwitchPolicy Constrained 4 次边界无人守护，doc 38 §21.4.2 漏洞重现 | 4 次切换 → OK，5 次切换 → ForceExit，混合 Constrained/Allowed/Frobidden policy？ | L3 | V5.2 + V5.3 + V5.4 + V5.5 |
| L3-02 | `TestL3_SameMode_4Times_ForcesExit` | 守护"回路深度 v2 按模式 hash 计数"承诺（doc 38 §21.3.2）——同模式不增长不逃避 | 如果删了，LoopDepthTracker v2 计数器失灵，资源耗尽/计费爆炸、用户体验卡死 | MaxDepth=3 语义：depth=1/2 → Continue，depth=3 → ForceExit（采纳 design §5.1 SoT）；跨 SessionID 重置？同 SessionID 跨 ProcessMessage 续跑？ | L3 | V5.1 + V5.4 |
| L3-03 | `TestL3_AnomalyDetector_5Nil_OpensL0` | 守护"CircuitBreaker 5 层接线"承诺（design §7）——异常被默默吞掉的兜底 | 如果删了，L0 AnomalyDetectorCB 不触发，下游节点持续异常输入无人告警 | 5 次连续 nil → open？open 后 LLM 仲裁？close 后恢复？ | L3 | V5.4 + V5.5 |
| L3-04 | `TestL3_Verifier_3Times2s_OpensL2` | 守护"Verifier 性能降级"承诺（design §7）——不拖慢主链路 | 如果删了，L2 VerifierCB 不触发，用户等待超时，飞书卡片无反馈 | 3 次连续 > 2s → open？降级策略？close 阈值？ | L3 | V5.4 |
| L3-05 | `TestL3_Human10s_Async_FeishuNotBlocked` | 守护"HumanArbitrator 异步化"承诺（design §5.3.1）——同步返回 + 内部异步 | 如果删了，可能误改回同步等待，ProcessMessage 阻塞 10s | A/B/C 选项响应？10s timeout 兜底？ctx 取消语义？ | L3 | V5.3 + V5.5 |
| L3-06 | `TestL3_PlanKindSwitch_Constrained_4Limit` | 守护"3 档策略 + 累计 ≤ 4"承诺（design §5.2）——同策略内累计计数 | 如果删了，Constrained policy 失效，LLM 可在 Constrained 内无限切换 | Constrained 4 次 → OK，5 次 → ForceExit？多次 ProcessMessage 累计？ | L3 | V5.2 + V5.5 |
| L3-07 | `TestL3_CB5Layers_Open_Independently` | 守护"5 个 CB 互不干扰"承诺（design §7）——单层 open 不应影响其他层 | 如果删了，5 个 CB 共用状态可能误触发连锁反应 | L0 open 时 L1/L2/L3/L4/L5 状态？reset 时序？ | L3 | V5.4 |

---

### L2 集成测试（6 个）

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 覆盖 PR |
|---|---------|---------|---------|-----------|---------|---------|
| L2-01 | `TestIntegration_4DepthLimits` | 守护 3 类深度限制（回路深度 + LoopBudget + CircuitBreaker）协同（doc 38 §21.1）| 如果删了，3 类深度限制各自独立，无法验证串联决策 | 回路深度超限 + LoopBudget 耗尽 + CB open 同时发生？决策优先级？ | L2 | V5.4 + V5.5 |
| L2-02 | `TestIntegration_3LayerArbitration` | 守护 ChainedArbitrator 链式调用契约（design §5.3.2）| 如果删了，链式调用语义可能错（Rule 在 LLM 之前？Human 在 Rule 之前？）| LLM 选 Continue → 立即返回？LLM 选 Exit → Rule？Rule Human → Human？ | L2 | V5.3 |
| L2-03 | `TestIntegration_5EscapeActions` | 守护 6 类 EscapeAction 决策路径完整（含 EscapePendingHuman 中间态）| 如果删了，6 类动作覆盖不全，新增 EscapePendingHuman 中间态可能误用 | Continue / EscalateToRule / EscalateToHuman / ForceExit / AbortWithAudit / PendingHuman 各 1 个 case？ | L2 | V5.3 + V5.4 |
| L2-04 | `TestIntegration_PlanKindSwitchLimit` | 守护 PlanKindSwitchPolicy 累计约束（L3-06 已含，此处独立集成）| 如果删了，跨 PR 集成时累计逻辑可能丢失 | 单 PR 内累计？跨 PR？ | L2 | V5.2 + V5.5 |
| L2-05 | `TestIntegration_5NodePipeline_End2End` | 守护 v5 5 节点完整接线（Observe→Plan→Execute→Verify→[Compensation]→EscapeEngine）| 如果删了，5 节点接线可能漏接 | 每个节点前 Evaluate 调用？失败降级？ | L2 | V5.5 |
| L2-06 | `TestIntegration_5WiringPoints` ⭐NEW | 守护 5 个接线点（Observe 失败/Plan 失败/Plan 前/Execute 失败/Verify 失败）独立工作（design §6）| 如果删了，5 个接线点可能互相覆盖或漏接 | 1a 短路后 1b 不调？其他 4 个独立触发？ | L2 | V5.5 |

---

### L1 单元测试（89 个，按 PR 分布）

#### T-01 LoopDepthTracker (PR-V5.1): 10 tests = 6 现有 + 4 新增

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-01 | `TestLoopDepthTracker_FirstCall` | 守护首回路计数语义（depth=1）| 首回路 depth=1 行为无守护 | 首次调用 session 状态？ | L1 | 现有 |
| L1-02 | `TestLoopDepthTracker_SameMode` | 守护同模式 depth++ | 核心计数语义失守 | 同模式连续调用 | L1 | 现有 |
| L1-03 | `TestLoopDepthTracker_DifferentMode` | 守护异模式 reset | 回路污染，跨模式误计数 | 模式切换 | L1 | 现有 |
| L1-04 | `TestLoopDepthTracker_ExceedMax` | 守护 MaxDepth=3 边界（采纳 design §5.1 SoT：`depth >= MaxDepth` 触发 ForceExit）| 超过 3 不触发 ForceExit | 断言：depth=1/2 → EscapeContinue；depth=3 → EscapeForceExit(reason=loop_depth_exceeded)；depth=4 → 同 ForceExit（兜底）| L1 | 现有 |
| L1-05 | `TestHashLoopContext_Deterministic` | 守护 hash 稳定性 | 同输入产生不同 hash，History 失效 | 同输入多次 hash | L1 | 现有 |
| L1-06 | `TestHashLoopContext_DifferentInput` | 守护 hash 区分性 | 不同输入产生相同 hash，误判同模式 | 5 字段任一不同 | L1 | 现有 |
| L1-07 | `TestLoopDepthTracker_SessionID_Isolated` ⭐NEW | 守护跨 session 隔离（codex review M4 明确）| sessionA 的 depth 污染 sessionB，"重置 depth 让回路无限续命"漏洞重现 | 跨 session 同模式调用 | L1 | 新增 |
| L1-08 | `TestLoopDepthTracker_Concurrent` ⭐NEW | 守护并发安全（race 0）| race 条件，depth 计数错误 | 100 并发同模式调用 | L1 | 新增 |
| L1-09 | `TestLoopDepthTracker_Reset` ⭐NEW | 守护 Reset 清空 History | History map 残留，session 内污染 | Reset 后再调用 | L1 | 新增 |
| L1-10 | `TestLoopDepthTracker_HashCollision_Resilience` ⭐NEW | 守护 hash 冲突时的降级行为 | hash 冲突时回路误判 | 构造 hash 冲突输入 | L1 | 新增 |

#### T-02 PlanKindSwitchPolicy (PR-V5.2): 15 tests = 10 现有 + 5 新增

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-11 | `TestDetermineSwitchPolicy_Exploration` | 守护 Exploration → Constrained 映射 | 映射关系错乱 | Exploration Plan | L1 | 现有 |
| L1-12 | `TestDetermineSwitchPolicy_Scenario` | 守护 Scenario → Allowed 映射 | 映射关系错乱 | Scenario Plan | L1 | 现有 |
| L1-13 | `TestDetermineSwitchPolicy_Protocol` | 守护 Protocol → Constrained 映射 | 映射关系错乱 | Protocol Plan | L1 | 现有 |
| L1-14 | `TestDetermineSwitchPolicy_Commitment` | 守护 Commitment → Forbidden 映射 | 映射关系错乱 | Commitment Plan | L1 | 现有 |
| L1-15 | `TestPlanKindSwitchCount_ExceedLimit` | 守护 4/5 累计边界 | 边界判断失守 | 4 → OK, 5 → error | L1 | 现有 |
| L1-16 | `TestPlanKindSwitchCount_ZeroStart` | 守护 0 次切换合法（首次）| 误判首次切换违规 | 0 次切换 | L1 | 现有 |
| L1-17 | `TestPlanKindSwitchPolicy_Forbidden_NoSwitch` | 守护 CommitmentPlan 0 次切换合法 | 误判 0 次违规 | Commitment 0 次 | L1 | 现有 |
| L1-18 | `TestPlanKindSwitchPolicy_Forbidden_OneSwitch` | 守护 CommitmentPlan 1 次切换 → ForceExit | 1 次未触发 | Commitment 1 次 | L1 | 现有 |
| L1-19 | `TestPlanKindSwitchPolicy_Constrained_Boundary` | 守护 Constrained 4/5 边界 | 4 → OK, 5 → ForceExit | Constrained 4/5 次 | L1 | 现有 |
| L1-20 | `TestDetermineSwitchPolicy_Allowed_NoLimit` | 守护 Allowed 无上限 | 误加限制 | Allowed 100 次 | L1 | 现有 |
| L1-21 | `TestPlanKindSwitchPolicy_PreReset_Boundary` ⭐NEW | 守护 Reset 后首次切换合法 | 误判首次切换违规 | Reset 后切换 | L1 | 新增 |
| L1-22 | `TestPlanKindSwitchPolicy_Concurrent` ⭐NEW | 守护并发计数 | 累计计数 race | 100 并发切换 | L1 | 新增 |
| L1-23 | `TestPlanKindSwitchPolicy_Integration_With_Planner` ⭐NEW | 守护与 planner.go MatchKind 集成 | MatchKind 之后未接 policy | planner 调用链 | L1 | 新增 |
| L1-24 | `TestPlanKindSwitchPolicy_EdgeCase_SameKindSwitch` ⭐NEW | 守护"同 Kind 重选"语义 | 重选不计数 vs 误计数 | 同一 Kind 连续选 | L1 | 新增 |
| L1-25 | `TestPlanKindSwitchPolicy_Allowed_NoUpperLimit` ⭐NEW | 守护 Allowed 无上限边界 | 误加 100/200 上限 | Allowed 1000 次 | L1 | 新增 |

#### T-03 ChainedArbitrator (PR-V5.3): 35 tests = 22 现有 + 13 新增

**arbitrator_test.go**: 18 现有 + 8 新增 = 26 tests

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-26 | `TestLLMArbitrator_Continue` | 守护 LLM 选 Continue | LLM Continue 路径失守 | mock Continue | L1 | 现有 |
| L1-27 | `TestLLMArbitrator_Exit` | 守护 LLM 选 Exit → 链式 Rule | LLM Exit 路径失守 | mock Exit | L1 | 现有 |
| L1-28 | `TestLLMArbitrator_Timeout` | 守护 5s 超时 → ForceExit 兜底 | LLM 阻塞 5s 不兜底 | mock 5s 不响应 | L1 | 现有 |
| L1-29 | `TestRuleArbitrator_Unrecoverable` | 守护不可恢复 → AbortWithAudit | 不可恢复场景失守 | mock 不可恢复 | L1 | 现有 |
| L1-30 | `TestRuleArbitrator_Recoverable` | 守护可恢复 → EscalateToHuman | 可恢复场景失守 | mock 可恢复 | L1 | 现有 |
| L1-31 | `TestHumanArbitrator_ChoiceA` | 守护 A 选项 → Continue | A 选项响应失守 | SubmitUserChoice("A") | L1 | 现有 |
| L1-32 | `TestHumanArbitrator_ChoiceB` | 守护 B 选项 → ForceExit | B 选项响应失守 | SubmitUserChoice("B") | L1 | 现有 |
| L1-33 | `TestHumanArbitrator_ChoiceC` | 守护 C 选项 → AbortWithAudit | C 选项响应失守 | SubmitUserChoice("C") | L1 | 现有 |
| L1-34 | `TestHumanArbitrator_Timeout` | 守护 10s 不响应 → ForceExit | 10s timeout 失守 | 10s 不响应 | L1 | 现有 |
| L1-35 | `TestHumanArbitrator_CtxCancel` | 守护 ctx 取消 → ForceExit | ctx 取消语义失守 | ctx.Done() | L1 | 现有 |
| L1-36 | `TestHumanArbitrator_PendingResolution_Save` | 守护 Save → Load 命中 | PendingResolution 存储失守 | Save 后 Load | L1 | 现有 |
| L1-37 | `TestHumanArbitrator_PendingResolution_Load_NotFound` | 守护首次 ProcessMessage Load 失败 | Load 失败处理失守 | 首次 Load | L1 | 现有 |
| L1-38 | `TestHumanArbitrator_SubmitUserChoice_Expired` | 守护过期 pendingID 丢弃 | 过期响应覆盖新 pendingID | cleanup 后提交 | L1 | 现有 |
| L1-39 | `TestHumanArbitrator_NotBlockProcessMessage` | 守护 Arbitrate 立即返回 <100ms | 10s 阻塞主链路 | Arbitrate 同步性 | L1 | 现有 |
| L1-40 | `TestChainedArbitrator_LLMContinue` | 守护 LLM Continue 立即返回 | 链式 LLM 路径失守 | LLM Continue | L1 | 现有 |
| L1-41 | `TestChainedArbitrator_LLMExit_Rule` | 守护 LLM Exit → Rule | 链式 LLM→Rule 失守 | LLM Exit | L1 | 现有 |
| L1-42 | `TestChainedArbitrator_RuleHuman` | 守护 Rule Human → Human | 链式 Rule→Human 失守 | Rule Human | L1 | 现有 |
| L1-43 | `TestChainedArbitrator_HumanPending` | 守护 Human → 返回 EscapePendingHuman | 中间态语义失守 | Human 调用 | L1 | 现有 |
| L1-44 | `TestLLMArbitrator_InvalidAction` ⭐NEW | 守护 LLM 返回非法 action 降级 | 非法 action 阻塞 | LLM 输出乱码 | L1 | 新增 |
| L1-45 | `TestLLMArbitrator_PanicRecovery` ⭐NEW | 守护 LLM panic 兜底 | 崩溃主链路 | LLM SDK panic | L1 | 新增 |
| L1-46 | `TestLLMArbitrator_ConcurrentCalls` ⭐NEW | 守护并发 LLM 仲裁 | race | 100 并发 | L1 | 新增 |
| L1-47 | `TestLLMArbitrator_PromptInjectingPlanKindSwitch` ⭐NEW | 守护 prompt 注入防御 | 注入攻击 | prompt 包含 SwitchCount=999 | L1 | 新增 |
| L1-48 | `TestRuleArbitrator_NilContext` ⭐NEW | 守护 nil context | panic | nil 输入 | L1 | 新增 |
| L1-49 | `TestHumanArbitrator_SubmitAfterCleanup` ⭐NEW | 守护过期提交丢弃 | 过期响应覆盖新 pendingID | cleanup 后提交 | L1 | 新增 |
| L1-50 | `TestChainedArbitrator_LLM_Panic_Recovery` ⭐NEW | 守护 LLM panic 后链式降级 | 链式调用中断 | LLM panic | L1 | 新增 |
| L1-51 | `TestChainedArbitrator_Order_Invariant` ⭐NEW | 守护链式顺序 LLM→Rule→Human | 顺序错乱 | 调用顺序 | L1 | 新增 |

**notifier_test.go**: 4 现有 + 2 新增 = 6 tests

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-52 | `TestFeishuCardNotifier_BuildCard` | 守护卡片结构（3 按钮 + ExpiresAt）| 卡片结构错乱 | 构建卡片 | L1 | 现有 |
| L1-53 | `TestChainedNotifier_FeishuSuccess` | 守护 Feishu 成功 → 不 fallback | 链式 fallback 失守 | Feishu 成功 | L1 | 现有 |
| L1-54 | `TestChainedNotifier_FeishuFail_CLISuccess` | 守护 Feishu 失败 → CLI 成功 | fallback 路径失守 | Feishu 失败 | L1 | 现有 |
| L1-55 | `TestChainedNotifier_AllFail` | 守护全部失败 → error | 错误传播失守 | 全部失败 | L1 | 现有 |
| L1-56 | `TestFeishuCardNotifier_NotImpl_DefaultsToCLI` ⭐NEW | 守护 Feishu 未配置时降级 | dev 环境无 fallback | 无 Feishu 配置 | L1 | 新增 |
| L1-57 | `TestChainedNotifier_PartialFail` ⭐NEW | 守护部分失败 fallback | 第一通道失败中断 | 第一通道失败 | L1 | 新增 |

**pending_resolution_test.go**: 0 现有 + 3 新增 = 3 tests（T-03 现有未列，补全）

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-58 | `TestInMemoryPendingResolutionStore_SaveLoad` ⭐NEW | 守护 Save → Load 命中语义 | 存储语义失守 | Save 后 Load | L1 | 新增（补全 T-03 缺失）|
| L1-59 | `TestInMemoryPendingResolutionStore_Delete` ⭐NEW | 守护 Delete 清理语义 | cleanup 不彻底 | Delete 后 Load | L1 | 新增（补全 T-03 缺失）|
| L1-60 | `TestInMemoryPendingResolutionStore_Concurrent` ⭐NEW | 守护并发读写 | race | 100 并发 | L1 | 新增（补全 T-03 缺失）|

#### T-04 EscapeEngine + CB (PR-V5.4): 15 tests = 9 现有 + 6 新增

**engine_test.go**: 4 现有 + 4 新增 = 8 tests

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-61 | `TestEscapeEngine_AllContinue` | 守护全部 Continue | 整合入口决策失守 | 三类都 Continue | L1 | 现有 |
| L1-62 | `TestEscapeEngine_TrackerExceed` | 守护回路超限 → ForceExit | tracker 集成失守 | depth 超过 | L1 | 现有 |
| L1-63 | `TestEscapeEngine_CircuitBreakerOpen` | 守护 CB open → LLM 仲裁 | CB 集成失守 | CB open | L1 | 现有 |
| L1-64 | `TestEscapeEngine_AuditLog_Record` | 守护 audit 记录 | audit 持久化失守 | decision 写入 | L1 | 现有 |
| L1-65 | `TestEscapeEngine_PanicRecovery` ⭐NEW | 守护 Engine panic 降级 | 主链路崩溃 | 子模块 panic | L1 | 新增 |
| L1-66 | `TestEscapeEngine_Concurrent` ⭐NEW | 守护并发 Evaluate | race | 100 并发 | L1 | 新增 |
| L1-67 | `TestEscapeEngine_ErrorFallback_Continue` ⭐NEW | 守护 error 降级为 Continue | error 阻塞主链路 | 子模块 error | L1 | 新增 |
| L1-68 | `TestEscapeEngine_5CircuitBreakers_Independent` ⭐NEW | 守护 5 CB 独立触发 | 1 open 触发全部 | L0 open | L1 | 新增 |

**circuit_breaker_test.go**: 5 现有 + 1 新增 = 6 tests

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-69 | `TestAnomalyDetectorCB_5Nil_Open` | 守护 L0 5 次 nil → open | 异常检测 CB 失守 | 5 次 nil | L1 | 现有 |
| L1-70 | `TestDispatchLoopWakeupsCB_100PerMin_Open` | 守护 L1 100/分 → open | dispatch loop CB 失守 | 100/min | L1 | 现有 |
| L1-71 | `TestVerifierCB_3Times2s_Open` | 守护 L2 3 次 > 2s → open | verifier CB 失守 | 3 次 > 2s | L1 | 现有 |
| L1-72 | `TestHookCB_5Fail_Open` | 守护 L3 5 次 fail → open | hook CB 失守 | 5 次 fail | L1 | 现有 |
| L1-73 | `TestWorkerPanicCB_1Panic_Open` | 守护 L4 1 次 panic → open | worker panic CB 失守 | 1 次 panic | L1 | 现有 |
| L1-74 | `TestSandboxExitCB_5Fail_Open` | 守护 L5 5 次 fail → open | sandbox exit CB 失守 | 5 次 fail | L1 | 现有 |
| L1-75 | `TestCircuitBreaker_StateMachine_OpenHalfOpenClose` ⭐NEW | 守护 state machine 转换 | 状态机错乱 | open/half-open/close | L1 | 新增 |

**audit_log_test.go**: 0 现有 + 1 新增 = 1 test

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-76 | `TestEscapeAuditLog_AuditLevel_0_1_2` ⭐NEW | 守护 3 个 AuditLevel 区分 | level 区分失效 | 各类 decision 写入 | L1 | 新增（补全 T-04 缺失）|

#### T-05 Orchestrator 接线 (PR-V5.5): 14 tests = 0 现有 + 14 新增（全部 new）

**orchestrator_escape_test.go**: 10 tests

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-77 | `TestOrchestrator_WithEscapeEngine_Option` | 守护 option 模式集成 | 未启用时静默失效 | 不传 option | L1 | 新增 |
| L1-78 | `TestOrchestrator_BuildLoopContext` | 守护 LoopContext 5 hash 字段构造 | hash 字段遗漏 | 字段缺失 | L1 | 新增 |
| L1-79 | `TestOrchestrator_ProcessEscapeDecision` | 守护 6 类 action 处理 | action 处理漏 | 各类 action | L1 | 新增 |
| L1-80 | `TestOrchestrator_EvaluateError_Fallback` | 守护 Evaluate error → slog.Warn + Continue | error 阻塞主链路 | Evaluate panic | L1 | 新增 |
| L1-81 | `TestOrchestrator_5WiringPoint_Observe` | 守护接线点 0 Observe 失败（含 observe==nil 和 observe 空列表两个 sub-case）| 接线点 0 漏 | case 1：Observe 返回 err + observe=nil → Plan 立即失败（1a 触发）；case 2：Observe 返回 err + observe=空列表 → Plan 走默认分支 | L1 | 新增 |
| L1-82 | `TestOrchestrator_5WiringPoint_PlanFailure` | 守护接线点 1a Plan 失败 | 接线点 1a 漏 | Plan 失败 | L1 | 新增 |
| L1-83 | `TestOrchestrator_5WiringPoint_PlanBefore` | 守护接线点 1b Plan 前 | 接线点 1b 漏 | Plan 前 | L1 | 新增 |
| L1-84 | `TestOrchestrator_5WiringPoint_PlanFailureShortCircuit` | 守护 1a 短路不调 1b（codex review R4 修复）| 同 ProcessMessage 内重复 Evaluate | Plan 失败 + Plan 前 | L1 | 新增 |
| L1-85 | `TestOrchestrator_5WiringPoint_Execute` | 守护接线点 2 Execute 失败 | 接线点 2 漏 | Execute 失败 | L1 | 新增 |
| L1-86 | `TestOrchestrator_5WiringPoint_Verify` | 守护接线点 3 Verify 失败 | 接线点 3 漏 | Verify 失败 | L1 | 新增 |

**planner_switch_policy_test.go**: 1 test

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-87 | `TestPlanner_PlanKindSwitchPolicy_Integration` | 守护 MatchKind → PlanKindSwitchPolicy 集成 | MatchKind 之后未接 policy | planner 调用链 | L1 | 新增 |

**aggregate_verdicts_escape_test.go**: 1 test

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-88 | `TestAggregateVerdicts_EscapeEngine_Integration` | 守护 AggregateVerdicts → EscapeEngine 集成 | Verify 失败未触发 Evaluate | Verdict FAIL | L1 | 新增 |

**learner_escape_test.go**: 1 test

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-89 | `TestLearner_EscapeEngine_Integration` | 守护 Learn → EscapeEngine 集成 | Learn 后未触发 Evaluate | Learn 异常 | L1 | 新增 |

**orchestrator_e2e_test.go**: 1 test

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 | 状态 |
|---|---------|---------|---------|-----------|---------|------|
| L1-90 | `TestOrchestrator_End2End_5NodePipeline` | 守护 5 节点完整 e2e（含 EscapeEngine 接线）| 接线回归无感知 | 完整 5 节点调用 | L1 | 新增 |

> **注**：L1-90 即是 L1-89 之上的端到端封装，等价于 L2-05 的单元化版本；为保持"5 节点接线"单测完整性而保留。

---

### 统计校验

| 层级 | 数量 | 计算 |
|------|------|------|
| L4 业务验收 | 4 | L4-01..L4-04 |
| L3 端到端场景 | 7 | L3-01..L3-07 |
| L2 集成 | 7 | L2-01..L2-07（含 L2-07 gap 补测）|
| L1 单元 | 103 | L1-01..L1-90 + L1-91..L1-103（13 gap 补测）|
| **总计** | **121** | 4+7+7+103 |

#### L1 按 PR 分布

| PR | 现有 | 新增（4-layer）| gap 补测 | 小计 |
|----|------|------|------|------|
| V5.1 (T-01) | 6 | 4 | +1 (L1-91) | 11 |
| V5.2 (T-02) | 10 | 5 | 0 | 15 |
| V5.3 (T-03) | 22 | 13 | +1 (L1-92) | 36 |
| V5.4 (T-04) | 9 | 7 | +6 (L1-93..98) | 22 |
| V5.5 (T-05) | 0 | 14 | +5 (L1-99..103) | 19 |
| **L1 合计** | **47** | **43** | **+13** | **103** |

> **L1-90 vs L2-05 互补不重复**：
> - **L1-90** `TestOrchestrator_End2End_5NodePipeline`：在 `orchestrator_e2e_test.go` 内的"单测 e2e"（mock LLM/Rule/Human，无 DB 无飞书），跑得快、CI 必过
> - **L2-05** `TestIntegration_5NodePipeline_End2End`：在 `escape_integration_test.go` 内的"集成 e2e"（真实 LLM 网关 + DB + 飞书），跑得慢、S5 验收
>
> 两者**不重复**：覆盖不同 fixture 层级 + 不同执行时间窗。L1-90 已计入 T-05 的 14 新增中（最后一行）。

#### 新增测试统计（基线 54 + gap 补测 14 = 68）

**基线 54**（之前已加）：
- L1 新增 42 个（含 3 个 T-03 补全 + 1 个 T-04 补全 + 1 个 L1-90 单元 e2e）
- L2 新增 1 个（L2-06 5WiringPoints）
- L3 新增 7 个（端到端故障链路）
- L4 新增 4 个（业务验收）

**gap 补测 14**（本次追加，详见 `## 覆盖 gap 补测设计` 章节）：
- L1 补测 13 个（L1-91..L1-103，3 P0 + 8 P1 + 2 P2）
- L2 补测 1 个（L2-07 4 IntentKind × 5 节点）

#### 与 codex review §3 担心点的对应

| Codex 担心 | 守护测试 |
|-----------|---------|
| §3.1 不实施 v5 的备选对比缺失 | L4-01..L4-04 业务验收 |
| §3.2 6 类 EscapeAction 设计冗余 | L2-03 5 类兜底动作 + L1-79 6 类 action 处理 |
| §3.3 LoopContext 11 字段冗余 | L1-78 BuildLoopContext 5 字段构造 |
| §3.4 CB 5 层阈值不严谨 | L3-03/L3-04 + L1-69..L1-75 7 个 CB 单测 |
| §3.5 5 接线点重复 Evaluate | L1-84 1a 短路不调 1b + L2-06 5WiringPoints |

---

## 覆盖 gap 补测设计（追加 14 tests, 2.2 天）

### 背景

经过 `## 测试用例全面性设计` 章节 107 个测试对 76 个设计元素的覆盖分析，识别出 **8 个 gap**（3 个 P0 + 4 个 P1 + 1 个 P2），整体覆盖率 **85%**。本节追加 **14 个测试**将覆盖率提升至 **97%**。

### 8 gap → 14 tests 映射

| Gap | 严重性 | 设计点 | 补测数 | 归属 PR |
|-----|--------|--------|--------|---------|
| G1 LoopBudget 无 explicit 测试 | 🔴 P0 | design §3 3 类决策源之一 | 2 | V5.4 |
| G2 ResumeSession / applyResumeDecision | 🔴 P0 | design §6 T2 续跑入口 | 5 | V5.5 |
| G3 AuditLog 持久化 | 🔴 P0 | design §5.3.2 T2 续跑前提 | 2 | V5.4 |
| G4 14 ExitReason 映射 | 🟡 P1 | design §4 ExitReason 字段 | 1 | V5.4 |
| G5 LoopDepthTracker panic | 🟡 P1 | design §9 失败降级矩阵 | 1 | V5.1 |
| G6 CircuitBreaker panic | 🟡 P1 | design §9 失败降级矩阵 | 1 | V5.4 |
| G7 PendingResolutionStore TTL | 🟡 P1 | design §5.3.1 10s 过期 | 1 | V5.3 |
| G8 4-way IntentKind × 5 节点 | 🟢 P2 | design §3.5 4 IntentKind 协同 | 1 | V5.5 |
| **合计** | | | **14** | |

### 补测清单（L1-91..L1-103 + L2-07）

#### G1 LoopBudget（2 tests, V5.4）— 解决 3 类决策源之一的覆盖盲点

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L1-96 | `TestLoopBudget_ConsecutiveExceeded` | 守护 LoopBudget 连续 3 次超限触发（design §3 doc 38 §19.2）| 3 类决策源中 1 类无守护，恶意回路 3 次内不被拦截 | 同 PlanKind 失败 3 次？跨 PlanKind 失败 3 次？混合 1+1+1？ | L1 |
| L1-97 | `TestLoopBudget_TotalExceeded` | 守护 LoopBudget 累计 20 次触发（设计冗余保护）| 累计超限不兜底，资源耗尽 | 累计 19 → OK, 20 → 触发？跨 SessionID 重置？ | L1 |

#### G2 ResumeSession / applyResumeDecision（5 tests, V5.5）— T2 续跑机制核心

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L1-99 | `TestResumeSession_Hit` | 守护 ResumeSession 命中续跑（design §6 T2 续跑入口）| T2 续跑入口失守，doc 38 §21.5 关键创新（Human 异步化）无守护 | 上次 ProcessMessage 升级到 Human，本次进入 → 命中？ | L1 |
| L1-100 | `TestResumeSession_Miss` | 守护 ResumeSession 未命中走完整 5 节点 | 误命中导致状态错乱 | 首次 ProcessMessage → 未命中？ | L1 |
| L1-101 | `TestApplyResumeDecision_Continue` | 守护 user 选 A → Continue 续跑 | A 选项响应失守（异步路径下）| SubmitUserChoice("A") 命中 pendingID | L1 |
| L1-102 | `TestApplyResumeDecision_ForceExit` | 守护 user 选 B → ForceExit 续跑 | B 选项响应失守 | SubmitUserChoice("B") 命中 pendingID | L1 |
| L1-103 | `TestApplyResumeDecision_AbortWithAudit` | 守护 user 选 C → AbortWithAudit 续跑 | C 选项响应失守 | SubmitUserChoice("C") 命中 pendingID | L1 |

#### G3 AuditLog 持久化（2 tests, V5.4）— T2 续跑前提

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L1-94 | `TestAuditLog_Persistence_RoundTrip` | 守护 audit 写入后能读出（设计 §5.3.2）| T2 续跑前提失守，pending 状态丢失 | Save → Load 立即命中？ | L1 |
| L1-95 | `TestAuditLog_Persistence_AfterRestart` | 守护重启后 audit 仍可读（DBPendingResolutionStore）| 重启后 T2 续跑失败 | Save → 模拟重启 → Load 命中？ | L1 |

#### G4 14 ExitReason 映射（1 test, V5.4）— Phase 4 兼容

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L1-93 | `TestEscapeDecision_ExitReason_Mapping_14` | 守护 14 类 Phase 4 ExitReason 全映射到 EscapeDecision.ExitReason（design §4 + L4-02 兼容承诺）| 14 类 ExitReason 部分失映射，Phase 4 兼容破坏 | 14 类逐一映射？新增 ExitReason 行为？ | L1 |

#### G5 LoopDepthTracker panic（1 test, V5.1）— 子模块 panic 降级

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L1-91 | `TestLoopDepthTracker_PanicRecovery` | 守护 LoopDepthTracker panic 降级为 Continue（design §9）| 子模块 panic 阻塞主链路 | 构造 tracker.ShouldContinue panic？ | L1 |

#### G6 CircuitBreaker panic（1 test, V5.4）— 子模块 panic 降级

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L1-98 | `TestCircuitBreaker_PanicRecovery` | 守护 CircuitBreaker panic 降级为 Continue（design §9）| 子模块 panic 阻塞主链路 | 构造 CB 状态查询 panic？ | L1 |

#### G7 PendingResolutionStore TTL（1 test, V5.3）— 过期清理

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L1-92 | `TestPendingResolutionStore_TTL_Expired` | 守护 TTL=10s 过期清理（design §5.3.1 HumanArbitrator 异步化）| 过期 pending 占用内存，过期后误用 | Save → 等 11s → Load 失败？ | L1 |

#### G8 4-way IntentKind × 5 节点（1 test, V5.5）— 协同覆盖

| # | Test ID | 业务目标 | 删除后果 | 攻击者视角 | 金字塔层 |
|---|---------|---------|---------|-----------|---------|
| L2-07 | `TestIntegration_4IntentKind_5NodePaths` | 守护 4 IntentKind 都能正确触发 EscapeEngine 接线点（采纳 review-r3 ISSUE-6 建议：表驱动 4×关键节点 = 12 case，非 4×5=20 矩阵）| 某 IntentKind 路径下 EscapeEngine 触发不一致 | 表驱动：4 IntentKind × {Observe, Plan, Execute, Verify} 关键节点；Skip IntentKind 应跳过 Plan/Execute/Verify 仅 1 次 Evaluate（Observe 边界）；Orchestrate IntentKind 应完整触发 5 节点 Evaluate | L2 |

### 补测后覆盖率

| 类别 | 补测前 | 补测后 | 变化 |
|------|--------|--------|------|
| A. 数据契约 | 62% | 75% | +13%（ExitReason 映射）|
| F. AuditLog | 67% | 100% | +33%（持久化）|
| G. EscapeEngine 整合 | 92% | 100% | +8%（子模块 panic）|
| I. T2 续跑机制 | 60% | 100% | +40%（ResumeSession + ApplyDecision）|
| J. 失败降级矩阵 | 69% | 85% | +16%（子模块 panic + TTL）|
| K. IntentKind 协同 | 75% | 100% | +25%（4-way × 5 节点）|
| M. 兼容 Phase 1-7 | 50% | 75% | +25%（14 ExitReason 映射）|
| **总计** | **85%** | **97%** | **+12%** |

### 新增文件

| 文件 | 归属 | 用途 |
|------|------|------|
| `loop_budget.go` | V5.4 | LoopBudget struct（consec=3, total=20）|
| `loop_budget_test.go` | V5.4 | L1-96/97 |
| `orchestrator_resume_test.go` | V5.5 | L1-99..103 |

### 工作量分布（追加 2.2 天）

| PR | 现有 | gap 补测 | 合计 | 说明 |
|----|------|---------|------|------|
| V5.1 | 1.0 天 | +0.2 天 | 1.2 天 | L1-91 LoopDepthTracker panic |
| V5.3 | 2.0 天 | +0.2 天 | 2.2 天 | L1-92 PendingResolution TTL |
| V5.4 | 1.0 天 | +0.8 天 | 1.8 天 | L1-93..98（ExitReason 映射 + AuditLog 持久化 + LoopBudget + CB panic）|
| V5.5 | 2.0 天 | +1.0 天 | 3.0 天 | L1-99..103（ResumeSession）+ L2-07（4 IntentKind × 5 节点）|
| **合计** | **6.5 天** | **+2.2 天** | **8.7 天** | |

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
