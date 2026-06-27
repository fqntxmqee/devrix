---
demand-id: DM-20260627-001
change-id: devrix-d7-workitem-rollup-pipeline
title: D7 WorkItem Rollup 闭环 — 验收报告
executor: Agent S5 (Cursor)
environment: local dev (go test)
date: 2026-06-27
verdict: PARTIAL
---

# 验收报告：D7 WorkItem Rollup 闭环

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260627-001 |
| Change ID | devrix-d7-workitem-rollup-pipeline |
| 执行人 | Agent S5 |
| 测试环境 | local dev / `go test` |
| 执行日期 | 2026-06-27 |
| 总体结论 | **PARTIAL** |

Phase 1 单元与 pipeline 级 P0 全部通过；全 TurnLoop trace 重放 E2E（A54-T04 / IT01 / IT02）以 stub IT 覆盖，待 CI stub LLM 后补全。**本 change 合入 ≠ WorkTree v2 完成。**

### 测试命令与结果

| Check | Command | Result |
|-------|---------|--------|
| 单元 | `go test ./internal/layers/orchestration/workmodel/... ./internal/layers/orchestration/sessionorchestrator/... -run 'Rollup\|SummaryBubble\|SessionDeliverable\|ChecklistFocus\|ObserveWorkItem' -count=1` | **PASS** |
| vet | `go vet ./internal/layers/orchestration/...` | **PASS** (0 error) |
| IT stub | `go test -tags 'integration d7' ./tests/integration/d7/... -run RollupTraceReplay -count=1` | **PASS** (Path B gate stub) |

> **Git：** 未 commit / 未 push（用户规则：commit 需显式请求）。合入前请用户创建功能分支 + PR。

## 2. L5 / T 测试点验证结果

| T ID | 描述 | 优先级 | 状态 | 证据 |
|------|------|--------|------|------|
| D7-S15-A50-T01 | NeedsRollup 向后兼容 | P0 | PASS | `workmodel/workitem_store_test.go` |
| D7-S15-A50-T02 | ReevaluateParent rollup gate | P0 | PASS | `rollup_gate_test.go` |
| D7-S15-A50-T03 | GetPipelineFocus rollup 优先 | P0 | PASS | `rollup_gate_test.go` |
| D7-S15-A55-T01 | all_pass 遇 fail 不 rollup | P0 | PASS | `TestShouldRollupAfterChildren_AllPassBlocksOnFail` |
| D7-S15-A55-T02 | min_coverage 阈值 | P0 | SKIP | Phase 2 冻结默认 |
| D7-S15-A55-T03 | best_effort 默认 | P0 | PASS | `rollup_gate_test.go` + `RollupGatePolicyFor` |
| D7-S15-A51-T01 | Summary CB3 截断 | P0 | PASS | `context_bubble_apply_test.go` |
| D7-S15-A51-T02 | Observe 双 bubble (T05) | P0 | PASS | `TestObserveWorkItem_RollupDualBubbles` |
| D7-S15-A51-T03 | Rollup directive 含 summary | P0 | PASS | `item_pipeline_rollup_test.go` |
| D7-S15-A60-T01..T03 | Rollup MUPS R2+ | P0 | PASS | `item_pipeline_rollup_test.go` |
| D7-S15-A61-T01..T02 | Session deliverable | P0 | PASS | `ExtractSessionDeliverable` + turn loop |
| D7-S15-A53-T01..T03 | Ephemeral checklist gate | P0 | PASS | workmodel 单测 |
| D7-S15-A54-T01..T03 | Root fallback + checklist bubble | P0 | PASS | 单测 + `item_observe.go` |
| D7-S15-A54-T04 | trace 重放 2× MUPS + complete | P0 | **PARTIAL** | stub IT only |
| D7-S15-IT01 | decompose + rollup E2E | P0 | **PARTIAL** | pipeline 单测替代 |
| D7-S15-IT02 | 无 checklist MUPS span | P0 | **PARTIAL** | stub IT only |

### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过/部分 |
|--------|------|------|------|-----------|
| P0 | 21 | 17 | 0 | 4 (1 SKIP + 3 PARTIAL) |
| P1 (Phase 2) | 6 | — | — | 登记不编码 |

## 3. Phase 2  deferred 项

| 项 | Phase 2 动作 |
|----|-------------|
| `RollupGateMinCoverage` | 持久化 `MinChildCoverageRatio` + 门控实现 |
| `RollupGatePolicy` 持久化 | WorkItem / SpawnMetadata 字段 |
| `ExpectedReturn` 文本匹配 | DecomposeProposer + rollup Verify |
| `FailureCriteria` 向下契约 | 子 Plan Verify 对齐父模板 |
| DecomposeProposer / ParallelExplore | T20–T22, T18–T19 独立 PR |
| 全 TurnLoop trace E2E | IT01/IT02 + A54-T04 补全 |

## 4. 领域文档同步（S5 门禁）

| 文件路径 | 变更摘要 | 已更新 |
|----------|----------|--------|
| `openspec/specs/d7-orchestration/spec.md` | v4.13.0 + D7-S15 Scenario | ✅ |
| `openspec/specs/d7-orchestration/t-registry.md` | v4.8.0 + D7-S15 T 点 | ✅ |
| `openspec/t-registry.md` | D7 计数更新 | ✅ |
| `openspec/specs/architecture/code-layout.md` | 无新 scenario 目录 | N/A |
| `openspec/specs/architecture/layering.md` | 无新 D/S | N/A |

## 5. Phase 1 冻结默认（§12.1）

见 `design.md` §12.1：best_effort only、min_coverage Phase 2、ExpectedReturn Phase 2、FailureCriteria 可选 footer、Verify heuristic。

## 6. 遗留风险

| 风险 | 影响 | 规避方案 |
|------|------|---------|
| 全 trace E2E 未跑通 | Jaeger 2× MUPS 未 CI 门禁 | stub IT + 后续 stub LLM PR |
| free_fork 侧路未汇总 | deliverable 缺 fork 产出 | Phase 2 / 独立 change（OQ-3） |
| PR 表述 | 误导 WorkTree v2 完成 | PR 标题/描述禁止该表述 |

## 7. 结论

**PARTIAL ACCEPTED** — Phase 1 结构闭环（gate、bubble、rollup MUPS、deliverable、checklist gate）单元验证通过；trace 级 E2E 以 stub 登记例外。P0 核心逻辑可合入；IT 补全不阻塞 Phase 1 归档。

**用户下一步：** `git checkout -b feat/d7-workitem-rollup-pipeline` → commit → PR（勿写「WorkTree v2 完成」）→ CI 绿后 squash merge。
