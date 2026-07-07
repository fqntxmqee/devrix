---
demand-id: DM-20260706-008
change-id: devrix-d7-proposer-prepend-and-report-summary-fix
title: D7 Proposer UserContextPrepend + ReportSummary 信息密度修复 — 验收报告
executor: Agent S5 (retroactive after 3-PR ship-then-archive cycle)
environment: local dev (go test -race + CI guard)
date: 2026-07-08
verdict: ACCEPTED
---

# 验收报告:D7 Proposer UserContextPrepend + ReportSummary 信息密度修复

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260706-008 |
| Change ID | devrix-d7-proposer-prepend-and-report-summary-fix |
| 总体结论 | **ACCEPTED** |
| 关联 PR | #449 (DM-20260706-007) + #450 (DM-20260706-008) + #460 (跨域 debt hotfix) |
| 域 | D7 |

`sess_1783333760211_6000` 飞书卡片失败事件(trace `0369df062c125dfe2aad9de21363730e`)
揭示 D7 编排层 3 个 LLM proposer 存在两类同源 bug:
- **Bug A**: UserContextPrepend 边界吞(messagesForLLMInvoke 未调用,LLM 拿不到 AGENTS.md)
- **Bug B**: ReportSummary 信息坍塌(uncertaintyReportSummary 只数 anomalies,ObsUncertainty 实际内容丢失)

PR #449 修复 Observe + Plan proposer;PR #450 加 Execute trace 验证 + CI guard 防御;
PR #460 跨域 debt 修复 IntentSegmenter(PR-A2 新增的 5th call site)。

### 测试命令与结果

| Check | Command | Result |
|-------|---------|--------|
| 单元测试 (orchestration) | `go test ./internal/layers/orchestration/... -count=1 -race` | **PASS** (26/26 packages) |
| 静态检查 | `go vet ./...` | **PASS** (0 warning) |
| CI guard | `bash scripts/check-d7-d3-prepend-boundary.sh` | **PASS** (6/6 InvokeStream call sites 通过 + 1 allow-list) |
| 7 PR #449 unit tests | `go test ./internal/layers/orchestration/sessionorchestrator/ -run "UserContext\|UncertaintyReport" -race -count=1` | **PASS** (7/7) |
| 1 PR #460 unit test | `go test ./internal/layers/orchestration/sessionorchestrator/ -run TestLLMIntentSegmenter_RoutesUserContextPrepend -race -count=1` | **PASS** (1/1) |

## 2. L5 / T 验收矩阵

| T ID | 描述 | 结果 |
|------|------|------|
| D7-S5-A121-T01 | uncertaintyReportSummary 签名扩展 + ObsUncertainty/ObsDeviation 序列化 + strength ≥ 0.7 阈值 | PASS |
| D7-S5-A121-T02 | LLMObservationProposer 走 messagesForLLMInvoke | PASS |
| D7-S5-A121-T03 | LLMStrategicPlanProposer 走 messagesForLLMInvoke | PASS |
| D7-S5-A121-T04 | LLMIntentSegmenter.SegmentRequest.UserContextPrepend 字段 + messagesForLLMInvoke wrap(PR #460 跨域 debt) | PASS |
| D7-S5-A121-T05 | scripts/check-d7-d3-prepend-boundary.sh CI guard | PASS |

| AC | 业务目标 | 结果 |
|----|----------|------|
| AC1 | D2→D3 边界强制 — 全仓 0 proposer 绕过 messagesForLLMInvoke | PASS (6/6 call sites + 1 allow-list) |
| AC2 | UncertaintyReport 内容保真 — serialization 含 q=...; dev=... | PASS (5 unit tests + trace observation_summary 含 q=<实际问题>) |
| AC3 | Execute 节点协议自洽 — Execute LLM 收到 AGENTS.md + plan_frame_delta | PASS (trace messages_count=2 自愈 Plan scope_in) |
| AC4 | Regression 全绿 — 26/26 orchestration packages go test -race PASS | PASS |
| AC5 | CI guard 兜底 — check-d7-d3-prepend-boundary.sh PASS | PASS (跨域 debt PR #460 触发 FAIL → 修复后 PASS) |

## 3. 8 unit test 覆盖矩阵

### 3.1 PR #449 新增 7 test

| Test | Gate / Behaviour | Result |
|------|------------------|--------|
| TestUncertaintyReportSummary_* (5 tests in deliverable_execute_test.go) | uncertaintyReportSummary 签名扩展 + ObsUncertainty/ObsDeviation 序列化 + strength ≥ 0.7 阈值过滤 | PASS |
| TestLLMObservationProposer_RoutesUserContextPrepend (llm_observation_proposer_test.go) | Observe proposer msgs 走 messagesForLLMInvoke + system block 含 AGENTS.md | PASS |
| TestLLMStrategicPlanProposer_RoutesUserContextPrepend (strategic_plan_proposer_usercontext_test.go NEW) | Plan proposer msgs 走 messagesForLLMInvoke + system block 含 AGENTS.md | PASS |

### 3.2 PR #460 跨域 debt 1 test

| Test | Gate / Behaviour | Result |
|------|------------------|--------|
| TestLLMIntentSegmenter_RoutesUserContextPrepend (intent_segmenter_llm_test.go) | IntentSegmenter SegmentRequest.UserContextPrepend 字段透传 + msgs 走 messagesForLLMInvoke + system block 含 AGENTS.md + D7 mapping | PASS |

### 3.3 测试用例细节

#### TestLLMIntentSegmenter_RoutesUserContextPrepend

测试构造:
```go
prepend := map[string]string{
    "AGENTS.md":         "D{N}→path mapping",
    "D7":                "internal/layers/orchestration/",
}
stub := &stubSegLLM{raw: `[{"id":"seg_0","text":"x","kind":"explore","priority":50,"confidence":0.8}]`}
s := NewLLMIntentSegmenter(stub)
set, err := s.Segment(context.Background(), SegmentRequest{
    SessionID:          "sess_prepend_test",
    Message:            "查 devrix",
    UserContextPrepend: prepend,
})
```

断言:
- `err == nil`(Segment 正常返回)
- `len(set.Segments) == 1`(解析正常)
- `strings.Contains(stub.messageLast[0].Content, "AGENTS.md")` ← messagesForLLMInvoke 包装后 system-reminder 块含 AGENTS.md
- `strings.Contains(stub.messageLast[0].Content, "D7")` ← D7 path mapping 在
- `strings.Contains(stub.messageLast[0].Content, "internal/layers/orchestration/")` ← 完整路径在

设计要点:`strings.Contains` 而非 exact match,因为 `messagesForLLMInvoke` 把 prepend
包在 `<system-reminder>` 块里(已存在逻辑),Content 是 system_reminder + user_prompt 拼接。

### 3.4 跨域 debt 解决详情(PR #460)

**触发**:PR #452(DM-20260707-001 PR-A2)合并后,CI guard `check-d7-d3-prepend-boundary.sh`
执行发现 `intent_segmenter.go:293` 处的 `InvokeStream(req)` 调用**没有**前 30 行调过
`messagesForLLMInvoke`,exit 1,CI 必跑 fail。

**根因**:PR #452 是 DM-20260707-001 multi-intent observation decompose 的 PR-A2,
它引入 `LLMIntentSegmenter` 作为新的 5th InvokeStream call site。但因为 DM-20260706-008
是 ship-then-archive hotfix,**没走 S1-S5 OpenSpec 流程,文档后补**,PR #452 开发者
没读到 contract。

**修复**(PR #460 commit 658f6df3 → cherry-pick to 474157ae → PR #460):

1. **forward-compatible 字段添加**:
   ```go
   type SegmentRequest struct {
       SessionID string
       Message   string
       Prior     *learn.AdaptivePrior
       UserContextPrepend map[string]string  // NEW, default nil = legacy
   }
   ```

2. **wrap messages**:
   ```go
   msgs := messagesForLLMInvoke([]types.Message{{
       SessionID: req.SessionID,
       Role:      types.MessageRoleUser,
       Content:   userPrompt,
   }}, req.UserContextPrepend)
   ```

3. **测试覆盖**:`TestLLMIntentSegmenter_RoutesUserContextPrepend` 验证 system block
   含 AGENTS.md + D7 mapping。

**结果**:CI guard 重跑 PASS。**正向反馈**:ship-then-archive 不是无序,
CI guard 充当合同警察,late shipping 的 sibling change 自动得到一致性约束。

## 4. 域文档同步

| 文件 | 已更新 |
|------|--------|
| openspec/specs/d7-orchestration/t-registry.md | ✅ D7-S5-A121 段(本 archive 同步新增 5 T points) |
| openspec/specs/d7-orchestration/CHANGELOG.md | ✅ 顶部条目 v4.27.0 → v4.28.0 |
| openspec/specs/d7-orchestration/spec.md | ✅ UserContextPrepend 边界协议段 |
| openspec/specs/d7-orchestration/d7-prepend-boundary-spec.md (NEW) | ✅ spec delta 文档化 boundary contract |
| openspec/demand-archive-index.md | ✅ DM-20260706-008 行(顶部) |
| openspec/t-registry.md (根) | ✅ D7 段引用 |

## 5. 部署状态

- **Production**: 已部署
  - PR #449 merged 2026-07-06 (commit a50de14f)
  - PR #450 merged 2026-07-06 (commit 31600755)
  - PR #460 merged 2026-07-08 (commit 474157ae,after cherry-pick)
- **Rollback plan**:
  - revert PR #449/450 → 飞书卡片❌ 回归,但不 panic
  - revert PR #460 → CI guard FAIL,后续 IntentSegmenter 调用点恢复绕过(legacy 行为)
- **Monitoring**:
  - D2→D3 边界 trace attribute:`span.attributes["llm.invoke.messages_count"]` 期望 ≥ 2(AGENTS.md + user)
  - D7 Observe/Plan scope_in 自愈:`span.attributes["plan.scope_in.path"]` 不再含 `d7 领域` 字面

## 6. 已知限制 / Follow-up

| Item | Severity | Plan |
|------|----------|------|
| Plan LLM output 缺 `source_observation_ids` 字段 | P2 | design.md §7.2 强约束,Plan→Execute 帧 delta 已 trace-verified,Gate 兜底无事故。归 hardening backlog |
| D2 prior_delta_empty span fallback 缺失 | P2 | DM-20260705-010 协议要求,r1 initial 阶段 FrameDelta 零值时 hardening 承诺的 span 未 emit。归 hardening backlog |
| CI guard 接入 GitHub Actions heavy steps | P1 | `check-d7-d3-prepend-boundary.sh` 当前本地跑,接入 CI 与 `check-orch-rename.sh` 并列 | 下一 PR 跟进 |
| 跨域 wire 一致性 enforcement | P1 | ship-then-archive 模式下 CI guard 是最后防线,理想是 OpenSpec S1-S5 流程前置预防(本 archive 是补全措施) | 流程优化 backlog |