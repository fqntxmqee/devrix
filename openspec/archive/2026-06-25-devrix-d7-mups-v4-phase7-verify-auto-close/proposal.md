# Proposal: D7 MUPS v4.3 Phase 7 — Verify→Learn Auto-Close + Operator TrackMode + D5 增强 (运行时 5 节点闭环)

**Change ID:** `devrix-d7-mups-v4-phase7-verify-auto-close`
**Demand ID:** DM-20260625-001
**Status:** S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Priority:** P0 (PR-7.1 + PR-7.2) / P1 (PR-7.3)
**Created:** 2026-06-25
**Author:** MUPS v4.3 Phase 7 运行时 5 节点闭环

---

## 1. 背景

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）Phase 2-6 已全部 S7_Archived。Phase 6 PR-F3 (D7-S12-A43-T06) 端到端 LP-1 集成测试中，**Verify→Learn 这一步是 E2E 测试代码手工构造的**（`learn_observe_closure_test.go:204-214` 循环调用 `f.learner.Learn(req)`），并不是 `SessionOrchestrator.ProcessMessage` 运行时自动完成的。

换言之：**生产环境调用 ProcessMessage 时，Verify 节点的 Verdict 不会自动触发 Learn 节点的 BayesianUpdate + ReputationStore 更新**，LP-1 闭环在生产中是断的。

doc 35 §三.4 + doc 46 §五.2 明确 Verify→Learn 应当自动闭环：

```
Plan → Execute → Verify (产生 Verdict) → Learn (BayesianUpdate + ReputationStore)
                                                                ↓
                                                    下一轮 Observe 注入 prior
```

但当前 v1.0 实现中：

- **Verify 节点**（Phase 4 PR-D 升格，D7-S10-A32-A35）只在路径级（FastPath/CommandHandler/OrchestratePath 内部）有 Verdict 概念，**没有聚合到 SessionOrchestrator 层级**
- **Learn 节点**（Phase 5 PR-E 升格，D7-S11-A36-A40）`Learner.Learn` 接口完整，**但 Orchestrator 不调用**
- **TrackMode 字段** `ProcessRequest.TrackMode` 不存在，hardcoded `TrackModeDeveloper`（观察者"开发"）— 缺少 Operator 角色（操作员高自信）切换能力
- **D5 可观测化** `sessionSpan` 已记录 `prior.alpha/beta`（Phase 6 PR-F2），但缺 `prior.mean / track_mode / classifier_source / injected_at`，trace 中无法一眼看清 prior 注入的语义

## 2. 问题陈述（4 Problems）

### P1: Verify→Learn 运行时断链（最严重）

`SessionOrchestrator.ProcessMessage` 在路径 channel 关闭后**直接返回**，不触发 `learner.Learn`。这导致：

1. **生产 LP-1 闭环不工作**：5 节点管道在生产里只跑前 4 节点，Learn 节点需要靠用户手工调用（实际场景里没人这么做）
2. **ReputationStore 永远冷启动**：每次会话 `prior.PriorBeta` 都是 `Beta(5,3)`，`Mean=0.625` 不会跨会话累积
3. **E2E 测试通过 ≠ 运行时通过**：`learn_observe_closure_test.go:204-214` 在测试代码里手工循环调用 `Learn` 模拟了闭环，但生产代码路径上这一步是缺的

### P2: ProcessRequest 缺 TrackMode 字段

`ProcessRequest{SessionID, Message, Metadata}` 没有 `TrackMode` 字段。`buildObserveRequest` 在传给 `learn.BuildAdaptivePrior` 时只能 hardcode `learn.TrackModeDeveloper`，无法支持 Operator 角色（高自信阈值，用于运维类会话）。

Phase 5 PR-E5 E5.3（D7-S11-A38-T07）已经定义了 `DefaultOperatorPrior = Beta(8,1)` 但 Orchestrator 没用上。

### P3: D5 sessionSpan prior 字段不全

Phase 6 PR-F2 (orchestrator.go:307-310) 已加 `learn.prior.alpha/beta`，但缺关键语义字段：

- `prior.mean` — Jaeger UI 一眼能看的核心指标
- `track_mode` — `developer` / `operator`，影响追溯
- `classifier_source` — `rule` / `shadow`，区分 baseline vs Phase 6 async LLM
- `injected_at` — `phase6_lp1` / `none`，证明 prior 真的被注入而非兜底

没有这 4 个字段，trace 排查时只能看到 alpha/beta 两个数字，看不出 prior 来源是真实注入还是冷启动兜底。

### P4: 缺 Auto-Close 语义层抽象

即使 PR-7.1 解决了"调用 Learn"，**如何从 FastPath/CommandHandler/OrchestratePath 的 channel 终端事件合成 Verdict** 仍需要明确约定：

- `complete` 事件 → `VerdictPass` (Success)
- `error` 事件 → `VerdictFail` (Failed)
- `tombstone` 事件 → `VerdictIndeterminate` + `IndeterminateReason="interrupt"` (Cancelled)
- skip 路径（IntentSkip）→ 不触发 Learn（无执行结果）
- 整段 path context cancellation → 同 tombstone

## 3. 解决方案（3 PR × 6 T 点）

### PR-7.1: Verify→Learn Auto-Close（核心）

- **范围**：
  - `sessionorchestrator/orchestrator.go`：新增 `processAutoClose(ch, sessionCtx, sessionID, intent)` 方法包装 channel, 监听最后一个 `EngineEvent` 的 Type 合成 `workmodel.Verdict` → 构造 `learn.LearnRequest{Verdict, Plan, Artifact, Observations, SessionID}` → 异步调用 `o.learner.Learn(ctx, req)`
  - 替换 `endSpanWhenChannelClosed` 调用为 `processAutoClose` (内部嵌套 endSpan), 保持 span 关闭语义不变
  - 3 层 fail-safe: nil learner / Learn error / channel 提前关闭 → log warning + skip, 不阻塞 caller
  - skip 路径不触发 Learn (无执行结果可学)
- **T 点**：D7-S13-A47-T01 (Auto-Close 包装函数) / T02 (Verdict 合成规则 + 3 层 fail-safe) / T03 (集成测试 ProcessMessage 完整跑 → Alpha++)
- **依赖**：Phase 5 (Learner interface 已就绪) + Phase 6 (learner 字段已注入)
- **风险**：Medium — 新增异步调用, 不能阻塞 ProcessMessage 同步返回, 失败不能影响主路径

### PR-7.2: Operator TrackMode 字段

- **范围**：
  - `orchtypes/process.go`：`ProcessRequest` 新增 `TrackMode string` 字段 (`""` 表示 `developer` 兜底)
  - `sessionorchestrator/orchestrator.go`：buildObserveRequest 把 `req.TrackMode` 透传给 `learn.BuildAdaptivePrior(rep, trackMode)`
  - `learn.BuildAdaptivePrior` 已有 `trackMode==""` 兜底逻辑, 仅需 Orchestrator 传值
- **T 点**：D7-S13-A48-T04 (ProcessRequest.TrackMode 字段 + 验证) / T05 (buildObserveRequest 透传 + Operator track → Beta(8,1) 测试)
- **依赖**：Phase 5 (BuildAdaptivePrior 已支持 trackMode 区分)
- **风险**：Low — 字段加 + 透传, 不影响既有调用方

### PR-7.3: D5 可观测化增强

- **范围**：
  - `sessionorchestrator/orchestrator.go`：sessionSpan 新增 4 个属性:
    - `learn.prior.mean` (float64, e.g. 0.625)
    - `learn.prior.track_mode` (string, e.g. "developer")
    - `learn.classifier_source` (string, "rule" / "shadow")
    - `learn.prior.injected_at` (string, "phase6_lp1" / "cold_start_failsafe")
  - 4 个 attribute 在 buildObserveRequest 返回后 + classifySpan 关闭前写入 sessionSpan
- **T 点**：D7-S13-A49-T06 (4 个 sessionSpan attribute + 测试验证)
- **依赖**：Phase 6 (sessionSpan prior.alpha/beta 已存在)
- **风险**：Low — 仅 trace 属性扩展, 不影响功能路径

## 4. 5 节点管道运行时闭环验证

完成 PR-7.1 后, 完整运行时 5 节点管道:

```
ProcessMessage(ctx, req{SessionID: "sess_001", Message: "fix bug", TrackMode: "operator"})
  │
  ├─[1]─→ buildObserveRequest
  │       │
  │       ├─ learner.Inject → AdaptivePrior{TrackMode: "operator", PriorBeta: Beta(8,1)}
  │       │  (假设 sess_001 ReputationStore 已有 Alpha=2, Beta=0 → merged Beta(10,1) Mean=0.909)
  │       └─ return ObserveRequest{Prior: AdaptivePrior{...}}
  │
  ├─[2]─→ classifier.ClassifyWithPrior → IntentFast (Confidence = 95 × 0.909 = 86)
  │
  ├─[3]─→ FastPath.Run → channel emit "complete" event → close
  │
  └─[4]─→ processAutoClose(channel) [异步]
          │
          ├─ 监听最后事件 Type="complete" → VerdictPass
          ├─ 构造 LearnRequest{Verdict: VerdictPass, SessionID: "sess_001"}
          ├─ learner.Learn(req)
          │   │
          │   ├─ BuildAdaptivePrior(verdict) → 5 类 Content 分发
          │   └─ BayesianUpdate → Alpha = 2 + 1 = 3 → ReputationStore.Update
          │
          └─ (下一轮 ProcessMessage) Inject → AdaptivePrior{Beta(11,1) Mean=0.917}
```

**关键不变式**：所有 5 节点（Observe → Plan → Execute → Verify → Learn）在生产运行时闭环, 不需要测试代码手工调用。

## 5. 不在本次任务范围

- ❌ D2 ContextEngine-backed ReputationStore 持久化 (P2 backlog, 太大需要独立 Change)
- ❌ ShadowClassifier.ClassifyWithPrior 异步 LLM 重新启用 (Phase 6 design gap, P2 backlog)
- ❌ Phase 6 P2/P3 audit closure (D5 span attribute 测试 + ctx cancellation + error propagation + 5 SentinelError 集成 + WithLearner(nil) 显式禁用, 独立 hotfix)
- ❌ Verifier 子 agent 显式注入 (Phase 4 D7-S10-A33-T03 路径级, 不在 Orchestrator 层)
- ❌ Plan / Artifact 反向追溯 (Phase 5 LP-5 已就绪, 不需要 Phase 7 重复)

## 6. 验收标准

- **6 个 P0 T 点全部 IMPLEMENTED** (T01-T06)
- **PR-7.1 集成测试**：`TestProcessMessage_Verify2Learn_AutoClose_PassAlpha` 验证 ProcessMessage 跑完 → Alpha++ → 下一轮 prior 更新
- **PR-7.2 字段测试**：`TestProcessRequest_TrackMode_Operator` 验证 TrackMode="operator" → DefaultOperatorPrior Beta(8,1)
- **PR-7.3 trace 测试**：`TestSessionSpan_Attributes_AllPriorFields` 验证 6 个 attribute 全部写入 (alpha/beta/mean/track_mode/classifier_source/injected_at)
- **Layer lint**：0 import cycle, 既有 baseline 路径 0 修改
- **Race-free**：所有测试 `go test -race ./...` 通过
- **3 层 fail-safe**：nil learner / Inject error / Learn error → log + skip, 不阻塞 ProcessMessage 同步返回
