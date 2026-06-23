# Demand: D7 MUPS v4.3 Phase 7 — Verify→Learn Auto-Close + Operator TrackMode + D5 增强

**Change ID:** `devrix-d7-mups-v4-phase7-verify-auto-close`
**Demand ID:** DM-20260625-001
**Status:** S1_Demand → S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Priority:** P0 (PR-7.1 + PR-7.2) / P1 (PR-7.3)
**Created:** 2026-06-25

---

## 1. 业务需求（BR）

### BR-1: 5 节点管道运行时闭环

**需求**：当用户在飞书发消息时（d7_enabled=true），5 节点管道（Observe → Plan → Execute → Verify → Learn）必须在生产运行时完整跑通, Learn 节点自动接收 Verify 的 Verdict 并触发 BayesianUpdate + ReputationStore 更新。

**当前状态**：
- ✅ Phase 2 (Observe + Plan) 升格完成, IMPLEMENTED
- ✅ Phase 3 (Execute) 升格完成, IMPLEMENTED
- ✅ Phase 4 (Verify) 升格完成, IMPLEMENTED
- ✅ Phase 5 (Learn) 升格完成, IMPLEMENTED
- ✅ Phase 6 (Observe-Learner 跨域闭环) 升格完成, **LP-1 在 in-package 测试 + E2E 测试中验证, 但生产 ProcessMessage 不触发 Learn**
- ❌ **PR-7.1 缺口**: SessionOrchestrator.ProcessMessage 不调用 learner.Learn

**验收**：
- `TestProcessMessage_Verify2Learn_AutoClose_PassAlpha` 通过
- 飞书端到端测试: 同一 session 发 3 条 Pass 消息, 第 4 条注入 prior Beta(8,3) (Alpha=3 累积)

### BR-2: Operator 角色 TrackMode 切换

**需求**：当用户被识别为 Operator 角色时（运维类会话），AdaptivePrior 应该使用 `DefaultOperatorPrior = Beta(8,1)` (Mean=0.889) 而非 `DefaultDeveloperPrior = Beta(5,3)` (Mean=0.625)，门槛更高，更信任用户。

**当前状态**：
- ✅ Phase 5 PR-E5 E5.3 (D7-S11-A38-T07) 定义 `DefaultOperatorPrior Beta(8,1)` + `BuildAdaptivePrior` 接受 trackMode 参数
- ❌ **PR-7.2 缺口**: `ProcessRequest` 没有 `TrackMode` 字段, Orchestrator 只能 hardcode `TrackModeDeveloper`

**验收**：
- `TestProcessRequest_TrackMode_Operator` 通过
- 飞书端到端测试: 配置 Operator 角色 → AdaptivePrior.Mean ≥ 0.85

### BR-3: D5 Trace 完整 prior 语义

**需求**：当 Phase 6 PR-F2 注入 prior 时, Jaeger sessionSpan 必须显示完整 6 字段（alpha/beta/mean/track_mode/classifier_source/injected_at），运维同学一眼看清 prior 是真实注入还是冷启动兜底。

**当前状态**：
- ✅ Phase 6 PR-F2 加了 `learn.prior.alpha/beta` (orchestrator.go:307-310)
- ❌ **PR-7.3 缺口**: 缺 mean / track_mode / classifier_source / injected_at

**验收**：
- `TestSessionSpan_Attributes_AllPriorFields` 通过
- 飞书端到端测试: Jaeger UI 看到完整 6 字段

## 2. 功能需求（FR）

### FR-7.1.1 processAutoClose 包装函数

- `SessionOrchestrator.processAutoClose(ch, sessionCtx, sessionID, intent) <-chan *contracts.EngineEvent` 包装 channel
- 监听最后一个 `EngineEvent` 的 Type, 按规则合成 `workmodel.Verdict`:
  - `complete` → `VerdictPass`
  - `error` → `VerdictFail` (Reason = event.Content)
  - `tombstone` → `VerdictIndeterminate` (IndeterminateReason = "interrupt")
  - skip 路径 (IntentSkip) → 不触发 Learn, 透传 channel
- 内部异步 goroutine 调用 `o.learner.Learn(ctx, req)` 后返回
- 替换 `endSpanWhenChannelClosed` 调用

### FR-7.1.2 3 层 fail-safe

| 失败模式 | 行为 | 日志 |
|---------|------|------|
| `o.learner == nil` | skip, 透传 channel | none (预期情况) |
| `o.learner != nil`, Learn error | log warning, 不阻塞 | slog.Warn |
| channel 提前关闭（context cancellation）| log warning, 不阻塞 | slog.Warn |
| 整段 path context 取消 | skip Learn (无 Verdict) | none |

### FR-7.1.3 LearnRequest 构造

- `LearnRequest{Verdict, Plan, Artifact, Observations, SessionID}`
- Plan/Artifact/Observations 暂时为 nil (Phase 7 v1.0 不做反向追溯, 由 PR-7.4+ 后续补)
- 关键: SessionID 必填, 触发 BayesianUpdate 必须有 session

### FR-7.2.1 ProcessRequest.TrackMode 字段

- `ProcessRequest` 新增 `TrackMode string` 字段 (`""` 表示 developer 兜底)
- `ProcessRequestContract(ctx, sessionID, message string)` 默认 `TrackMode: ""` (兼容 D1 gateway 调用)
- 飞书适配器层 (D1) 可以根据用户角色配置注入 TrackMode

### FR-7.2.2 buildObserveRequest 透传

- `buildObserveRequest` 把 `req.TrackMode` 解析为 `learn.TrackModeDeveloper` / `learn.TrackModeOperator`
- 空字符串 → `learn.TrackModeDeveloper` 兜底
- 非法值 → `learn.TrackModeDeveloper` 兜底 + log warning

### FR-7.3.1 sessionSpan 4 新增属性

- `learn.prior.mean` (float64, prior.PriorBeta.Mean())
- `learn.prior.track_mode` (string, "developer" / "operator")
- `learn.classifier_source` (string, "rule" / "shadow")
- `learn.prior.injected_at` (string, "phase6_lp1" / "cold_start_failsafe")

## 3. 非功能需求（NFR）

### NFR-7.1 性能

- Auto-Close goroutine 必须在 ProcessMessage 同步返回后**异步执行**, 不能阻塞 caller
- Learn 调用超时 5s (与 AssetBuilder.Build 默认超时对齐)
- sessionSpan attribute 写入 4 个新字段开销 < 100µs (4 次 fmt.Sprintf)

### NFR-7.2 可靠性

- Auto-Close 失败不能影响 ProcessMessage 同步路径 (3 层 fail-safe)
- Race-free: 所有 sessionSpan attribute 写入必须 `-race` 通过
- Channel 关闭语义不变: endSpanWhenChannelClosed 内嵌在 processAutoClose 内, span 关闭时机不变

### NFR-7.3 可观测性

- D5 span 包含完整 6 字段 (alpha/beta/mean/track_mode/classifier_source/injected_at)
- slog 输出 Learn 调用的 (sessionID, verdict_kind, error) 便于排查

### NFR-7.4 兼容性

- ProcessRequest 新字段用 omitempty 风格, 旧调用方 ProcessRequestContract 行为不变
- baseline 路径 (nil learner) 行为完全不变
- ProcessRequestContract(ctx, sessionID, message) 默认 TrackMode="", 兜底 developer

## 4. 约束条件

- **DSAFT 层归属**: D7 (orchestration)
- **包依赖**: sessionorchestrator 可 import learn (Phase 5 precedent), learn 不能 import sessionorchestrator
- **可证伪沉淀**: prior.Mean 作为 confidence 乘数 (Phase 6 PR-F1 既定), 不可改
- **3 层 fail-safe**: nil learner / Inject error / Learn error, 3 种全部统一为 DefaultDeveloperPrior 兜底 (Phase 6 既定)
- **冷启动延迟**: prior == nil → Beta(5,3) Mean=0.625 (Phase 5 既定)

## 5. 验收测试（AC）

| AC | T 点 | 描述 |
|----|------|------|
| **AC1** | D7-S13-A47-T01 | processAutoClose 包装 channel + 异步触发 learner.Learn |
| **AC2** | D7-S13-A47-T02 | Verdict 合成规则 (complete→Pass / error→Fail / tombstone→Indeterminate) + 3 层 fail-safe |
| **AC3** | D7-S13-A47-T03 | 集成测试 ProcessMessage 完整跑 → Alpha++ + 下一轮 prior 更新 |
| **AC4** | D7-S13-A48-T04 | ProcessRequest.TrackMode 字段 + 验证 |
| **AC5** | D7-S13-A48-T05 | buildObserveRequest 透传 + Operator track → Beta(8,1) 测试 |
| **AC6** | D7-S13-A49-T06 | 4 个 sessionSpan attribute (mean/track_mode/classifier_source/injected_at) + 测试验证 |

## 6. 依赖关系

### 上游依赖（已完成）

- **Phase 5 PR-E** (D7-S11-A40-T10) — Learner interface + DefaultLearner + BayesianUpdate
- **Phase 6 PR-F2** (D7-S12-A42-T04) — WithLearner option + buildObserveRequest
- **Phase 5 PR-E5** (D7-S11-A38-T07) — DefaultDeveloperPrior / DefaultOperatorPrior + BuildAdaptivePrior

### 下游影响

- **D1 通信层 (D7 邻居)**: ProcessRequest 新字段 TrackMode, D1 gateway 适配器层 (Feishu adapter) 未来版本可注入
- **D2 上下文引擎 (D7 邻居)**: 无影响 (Learn 节点跨域, 不动 D2)
- **D4 多智能体 (D7 邻居)**: 无影响 (Execute 节点内部)
- **D5 可观测性**: 4 新增 span attribute, Jaeger UI 自然支持
