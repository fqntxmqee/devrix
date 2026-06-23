# Tasks: D7 MUPS v4.3 Phase 7 — Verify→Learn Auto-Close + Operator TrackMode + D5 增强

**Change ID:** `devrix-d7-mups-v4-phase7-verify-auto-close`
**Demand ID:** DM-20260625-001
**Status:** S3_Tasks → S4_Implemented → S7_Archived
**Created:** 2026-06-25

---

## 总览

| 阶段 | PR | T 点 | 文件 | LOC | 风险 | 状态 |
|------|----|----|------|-----|------|------|
| PR-7.1 | Verify→Learn Auto-Close | 3 T (D7-S13-A47-T01/T02/T03) | 2 MODIFIED + 2 NEW | +200/-30 | Medium | pending |
| PR-7.2 | Operator TrackMode | 2 T (D7-S13-A48-T04/T05) | 2 MODIFIED + 1 NEW | +50/-5 | Low | pending |
| PR-7.3 | D5 可观测化增强 | 1 T (D7-S13-A49-T06) | 1 MODIFIED + 1 NEW | +60/-0 | Low | pending |
| **总计** | **3 PR 联动** | **6 T 点** | **5 MODIFIED + 4 NEW** | **+310/-35** | — | **3 天** |

---

## PR-7.1: Verify→Learn Auto-Close (D7-S13-A47)

### T01: processAutoClose 包装 channel + 异步触发 learner.Learn

- [ ] **T01.1** `sessionorchestrator/orchestrator.go` 新增 `processAutoClose` 私有方法
  - 签名: `func (o *SessionOrchestrator) processAutoClose(ch <-chan *contracts.EngineEvent, sessionCtx context.Context, sessionID string, intent orchtypes.IntentClassification) <-chan *contracts.EngineEvent`
  - 内部启动 goroutine, 透传 channel events
  - channel 关闭后, 监听最后事件 Type → synthesizeVerdict → 异步调 o.learner.Learn
  - 替换 `endSpanWhenChannelClosed` 在 ProcessMessage 的调用 (orchestrator.go:401)
- [ ] **T01.2** nil learner 短路: `o.learner == nil` → 直接调 `endSpanWhenChannelClosed(ch, sessionSpan)`, 不启 goroutine
- [ ] **T01.3** IntentSkip 路径不调用 processAutoClose (skip 不学), 在 orchestrator.go:373-376 维持 close channel 直连
- [ ] **T01.4** 单元测试 `TestProcessAutoClose_NilLearner_Passthrough`
  - 验证 nil learner 走 endSpanWhenChannelClosed, 透传 channel, span 正常关闭
- [ ] **T01.5** 单元测试 `TestProcessAutoClose_LearnerError_LoggedNotBlocked`
  - 验证 Learner.Learn 返回 error → log warning, channel 透传不受影响

### T02: Verdict 合成规则 + 3 层 fail-safe

- [ ] **T02.1** `sessionorchestrator/orchestrator.go` 新增 `synthesizeVerdict` 私有函数
  - 签名: `func synthesizeVerdict(last *contracts.EngineEvent, sessionID string) *workmodel.Verdict`
  - 规则: complete → VerdictPass / error → VerdictFail (Reason=event.Content) / tombstone → VerdictIndeterminate (IndeterminateReason="interrupt")
  - 其他 Type (text / thinking / tool_call / tool_result / status / permission) → 返回 nil
  - SourceID 格式: `autoclose:{sessionID}:{nanosecond}`
- [ ] **T02.2** 3 层 fail-safe 在 processAutoClose 内部统一:
  - Layer 1: o.learner == nil → 走 endSpanWhenChannelClosed (T01.2 已覆盖)
  - Layer 2: o.learner != nil, Learn returns err → slog.Warn + 不阻塞
  - Layer 3: channel 提前关闭 (context cancel) → slog.Warn + skip Learn
- [ ] **T02.3** 单元测试 `TestSynthesizeVerdict_AllEventTypes` (table-driven):
  - `complete` → VerdictPass
  - `error` (Content="OOM") → VerdictFail (Reason="OOM")
  - `tombstone` → VerdictIndeterminate (IndeterminateReason="interrupt")
  - `text` → nil
  - `thinking` → nil
  - `tool_call` → nil
  - `tool_result` → nil
  - `status` → nil
  - `permission` → nil
  - nil lastEvent → nil (skip path / empty channel)
- [ ] **T02.4** 单元测试 `TestProcessAutoClose_EmptyChannel_NoLearn`
  - 验证 channel 0 events 直接关闭 → processAutoClose 不触发 Learn (skip path 等价)

### T03: 集成测试 ProcessMessage 完整跑 → Alpha++ + 下一轮 prior 更新

- [ ] **T03.1** `sessionorchestrator/orchestrator_learner_test.go` 新增 `TestProcessMessage_Verify2Learn_AutoClose_PassAlpha`
  - 装配: 真实 DefaultLearner + InMemoryReputationStore + fakeD2 (recording executor emit "complete" event)
  - Round 1: ProcessMessage → executor emit "complete" → processAutoClose → Learn(VerdictPass) → ReputationStore.Alpha=1
  - Round 2: ProcessMessage → buildObserveRequest → learner.Inject → PriorBeta=merged Beta(5+1, 3+0)=Beta(6,3) Mean=0.667
  - 验证: Round 2 prior.PriorBeta = Beta(6,3), 真实运行时闭环
- [ ] **T03.2** 集成测试 `TestProcessMessage_AutoClose_NilLearner_NoOp`
  - 装配: nil learner (no WithLearner)
  - 验证: ProcessMessage 正常跑, channel 透传, 不 panic, 不调用 Learn
- [ ] **T03.3** 集成测试 `TestProcessMessage_AutoClose_LearnerError_LoggedNotBlocked`
  - 装配: fakeLearner with LearnErr=error
  - 验证: Learn 返回 error → log warning, ProcessMessage 同步返回 channel 不受影响
- [ ] **T03.4** 集成测试 `TestProcessMessage_AutoClose_ContextCancel_SkipLearn`
  - 装配: cancel context mid-path
  - 验证: channel 提前关闭 → processAutoClose skip Learn, slog.Warn

### T01-T03 PR 验收

- [ ] go test -race ./internal/layers/orchestration/sessionorchestrator/... 全绿
- [ ] 6 unit/integration tests 全绿
- [ ] ProcessMessage 同步返回语义不变 (channel 立即可读, 关闭时机不变)
- [ ] endSpanWhenChannelClosed 在 processAutoClose 内部仍被调用, span 关闭时机不变
- [ ] Layer lint: sessionorchestrator → learn 合法, 反向 0 import

---

## PR-7.2: Operator TrackMode 字段 (D7-S13-A48)

### T04: ProcessRequest.TrackMode 字段 + 验证

- [ ] **T04.1** `orchtypes/process.go` 新增 `TrackMode string` 字段
  - 默认 `""` → 兜底 `learn.TrackModeDeveloper`
  - 接受值: `"developer"` / `"operator"` / `""` (兜底)
  - 非法值: `"unknown"` → log warning + 兜底 developer
- [ ] **T04.2** `orchtypes/process.go` 现有调用方零修改 (字段在结构体尾部, omitempty 风格)
- [ ] **T04.3** 单元测试 `TestProcessRequest_TrackMode_Default`:
  - 空 ProcessRequest{}.TrackMode == ""
  - ProcessRequest{TrackMode: "operator"}.TrackMode == "operator"
- [ ] **T04.4** 单元测试 `TestProcessRequest_TrackMode_Contract_Default`:
  - `ProcessMessageContract(ctx, sessionID, message)` 调用 ProcessRequest 转换时, TrackMode = "" (D1 gateway 兼容)

### T05: buildObserveRequest 透传 + Operator track → Beta(8,1) 测试

- [ ] **T05.1** `sessionorchestrator/orchestrator.go` `buildObserveRequest` 解析 req.TrackMode:
  - `""` → `learn.TrackModeDeveloper`
  - `"developer"` → `learn.TrackModeDeveloper`
  - `"operator"` → `learn.TrackModeOperator`
  - 其他 → `learn.TrackModeDeveloper` + log warning
- [ ] **T05.2** `buildObserveRequest` 把 trackMode 传给 `learn.BuildAdaptivePrior(rep, trackMode)`
- [ ] **T05.3** 单元测试 `TestBuildObserveRequest_TrackMode_Operator_PriorBeta_8_1`:
  - fakeLearner returning AdaptivePrior{TrackMode: operator, PriorBeta: Beta(8,1)}
  - 验证 buildObserveRequest 返回的 ObserveRequest.Prior.PriorBeta == Beta(8,1)
  - 验证 Mean = 0.889
- [ ] **T05.4** 单元测试 `TestBuildObserveRequest_TrackMode_Default_Developer`:
  - req.TrackMode = "" → DefaultDeveloperPrior Beta(5,3) Mean=0.625
- [ ] **T05.5** 单元测试 `TestBuildObserveRequest_TrackMode_Invalid_FallsBackDeveloper`:
  - req.TrackMode = "garbage" → log warning + Beta(5,3)

### T04-T05 PR 验收

- [ ] go test -race ./internal/layers/orchestration/orchtypes/... 全绿
- [ ] go test -race ./internal/layers/orchestration/sessionorchestrator/... 全绿
- [ ] 5 unit tests 全绿
- [ ] ProcessRequestContract 默认 TrackMode="" (D1 gateway 兼容)
- [ ] Operator 角色真实注入 Beta(8,1) (Mean=0.889)

---

## PR-7.3: D5 可观测化增强 (D7-S13-A49)

### T06: 4 个 sessionSpan attribute (mean/track_mode/classifier_source/injected_at) + 测试验证

- [ ] **T06.1** `sessionorchestrator/orchestrator.go` sessionSpan 新增 4 个 attribute (在 buildObserveRequest 返回后):
  - `learn.prior.mean` (float, prior.PriorBeta.Mean())
  - `learn.prior.track_mode` (string, prior.TrackMode)
  - `learn.prior.injected_at` (string, "phase6_lp1" if prior != nil else "cold_start_failsafe")
  - `learn.classifier_source` (string, "rule" / "shadow", 在 classifySpan 关闭前写入, 见 orchestrator.go:348)
- [ ] **T06.2** 兼容 Phase 6 既有 alpha/beta attribute, 6 字段一气呵成
- [ ] **T06.3** 单元测试 `TestSessionSpan_Attributes_AllPriorFields`:
  - 装配: 真实 sessionSpan (tracer.NewSpan) + fakeLearner returning AdaptivePrior{TrackMode: operator, PriorBeta: Beta(8,1)}
  - 验证 span 6 attribute 全部写入: alpha=8 / beta=1 / mean=0.889 / track_mode=operator / injected_at=phase6_lp1 / classifier_source=rule
- [ ] **T06.4** 单元测试 `TestSessionSpan_Attributes_ColdStartFailsafe`:
  - 装配: nil learner (no WithLearner)
  - 验证 span 6 attribute: alpha=5 / beta=3 / mean=0.625 / track_mode=developer / injected_at=cold_start_failsafe / classifier_source=rule
- [ ] **T06.5** 单元测试 `TestSessionSpan_Attributes_ShadowSource`:
  - 装配: WithShadowClassifier + fakeLearner
  - 验证 classifier_source=shadow

### T06 PR 验收

- [ ] go test -race ./internal/layers/orchestration/sessionorchestrator/... 全绿
- [ ] 3 unit tests 全绿
- [ ] 6 span attribute 全部写入, Jaeger UI 可见
- [ ] injected_at 字段明确标识 prior 来源 (phase6_lp1 vs cold_start_failsafe), 排查一目了然

---

## 3 PR 联动验证

完成 3 PR 后, 端到端验证:

```bash
# Unit + integration tests
go test -race ./internal/layers/orchestration/...

# Layer lint
go vet ./...

# Build smoke
go build -o bin/devrix ./cmd/devrix

# Manual e2e: 飞书端到端
# 1. 启动 devrix
# 2. 发 3 条 Pass 消息 (greeting / simple question / simple command)
# 3. 观察 Jaeger sessionSpan:
#    - Round 1: injected_at=phase6_lp1 / mean=0.625 / track_mode=developer / classifier_source=rule
#    - Round 2: injected_at=phase6_lp1 / mean=0.667 / track_mode=developer / classifier_source=rule (Alpha=1 累积)
#    - Round 3: injected_at=phase6_lp1 / mean=0.700 / track_mode=developer / classifier_source=rule (Alpha=2 累积)
#    - Round 4: injected_at=phase6_lp1 / mean=0.727 / track_mode=developer / classifier_source=rule (Alpha=3 累积)
```

## 实施顺序（避免循环依赖）

1. **第 1 步**：PR-7.1 (Verify→Learn Auto-Close) — 核心闭环, 3 T 点
2. **第 2 步**：PR-7.2 (Operator TrackMode) — 字段 + 透传, 2 T 点
3. **第 3 步**：PR-7.3 (D5 可观测化) — 4 attribute 增量, 1 T 点
4. **第 4 步**：S4-Gate (code review) + S5 验收 + S6 归档
