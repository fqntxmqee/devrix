---
demand-id: DM-20260706-001
change-id: devrix-d7-frame-delta-phase1-2-span-trigger
title: D7 MUPS Frame Delta Phase 1+2 spans 端到端触发 — testutil callback + seed helper 覆盖 e2e gap (scope 收窄)
executor: Agent S5
environment: local dev (go test) + CI
date: 2026-07-08
verdict: ACCEPTED
---

# 验收报告：D7 MUPS Frame Delta Phase 1+2 spans 端到端触发 (testutil_only)

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260706-001 |
| Change ID | devrix-d7-frame-delta-phase1-2-span-trigger |
| Sibling Change | devrix-d7-frame-delta-phase2-production-wiring (DM-20260706-004) |
| Parent Change | devrix-d7-mups-frame-delta-closure (DM-20260705-010, S7_Archived) |
| 总体结论 | **ACCEPTED** |
| 实现 PR | PR #466 squash merged 2026-07-08 |

D7 MUPS 5 节点 (Observe→Plan→Execute) FrameDelta I/O 协议 Phase 1+2+3 在 `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, PR #434) Phase 4 e2e 重放 (PR #437) 闭环后,Phase 4 §8.3 文档化 1 个 follow-up gap:Phase 1+2 span 计数在 memory exporter 中为 0。本 change 是该 gap 的 **testutil_only scope 修复**:为 testutil 增加 FrameDeltaInject 回调 + LastFrameDelta atomic.Pointer + SeedPriorExecContext helper + 5-cycle e2e 测试,严格 scope 隔离不修改 production code。

### 1.1 S3-Rewrite split 决定 (2026-07-08)

S3-Gate codex CLI review (2026-07-08) 判定 BLOCKED + 3 P0 issue。**Split 决议**:Phase 2 production wiring 缺失 (production code change) 拆分至独立 sibling change DM-20260706-004 处理,本 change scope 收窄至 testutil_only + e2e baseline 提升。

| P0 issue | 归属 |
|----------|------|
| Phase 1+2 e2e baseline 不触发 | **本 change 处理** (testutil callback + seed helper) |
| Phase 2 production caller 硬编码 nil | **sibling DM-20260706-004 处理** (production-side 修复) |
| SeedPriorExecContext 设计字段错位 | **本 change 处理** (代码微调对齐) |

## 2. 测试命令与结果

| Check | Command | Result |
|-------|---------|--------|
| 单元测试 (orchestration) | `go test -race -count=1 ./internal/layers/orchestration/...` | **PASS** (26/26 packages) |
| 单元测试 (testutil) | `go test -race -count=1 ./tests/testutil/...` | **PASS** |
| 集成测试 (D7 e2e) | `go test -tags 'integration d7' -count=1 -run 'TestIntegration_D7FrameDelta' ./tests/integration/d7/...` | **PASS** (5 子测试含新增 5-cycle) |
| 静态检查 | `go vet ./...` | **PASS** (0 warning) |
| CI (PR #466) | `gh pr checks 466` | unit tests + layer-lint PASS |

## 3. L5 / T 验收矩阵

| T ID | 描述 | 结果 |
|------|------|------|
| L5-MUPS-FD-6 / T20 | testutil Phase 1+2+3 span 触发链 (callback + atomic + SeedPriorExecContext + 5-cycle e2e) | PASS |
| AC4 callback invariance | FrameDeltaInject per Stream-call + LastFrameDelta atomic most-recent-wins + "testutil only" docstring | PASS |

| AC | 描述 | 结果 |
|----|------|------|
| AC1 | Phase 1 (PlanFrameDeltaInject) span ≥ 1 in 5-cycle e2e | PASS (≥1 baseline, 待 sibling PR #467 落地后 ≥5) |
| AC2 | Phase 2 (ObservePriorDelta) baseline ≥ 2 (zero-value FrameDelta via hardening prior_delta_empty) | PASS (sibling PR #467 落地后 ≥2 non-zero) |
| AC3 | Phase 3 (ConvergenceMetric) span ≥ 1 | PASS |
| AC4 | FrameDeltaInject testutil_only 文档化 + callback invariance | PASS |
| AC5 | d7.s5.observe.prior_delta.span emit 链路 | PASS (span 通过 hardening nil-bridge, zero-value branch emit "prior_delta_empty" 待 sibling PR #467 切换 non-zero) |
| AC6 | M1/M2 frame 契约 0 修改 | PASS (testutil_only 无 production code 改动) |
| AC7 | 跨链 LLM prompt size monotonic | PASS (5-cycle e2e + monotonicity test) |
| AC8 | T19 三方 review follow-up | DEFERRED (codex + cursor quota 待恢复) |

## 4. 文件改动清单

| 文件 | 改动类型 | 行数 |
|------|---------|------|
| `tests/testutil/d7_llm_stub.go` | MODIFIED (FrameDeltaInject callback + LastFrameDelta atomic.Pointer + interfaces import) | +24 |
| `tests/testutil/d7_frame_delta_helpers.go` | NEW (SeedPriorExecContext + FormatConvergenceRate) | +113 |
| `tests/testutil/d7_frame_delta_helpers_test.go` | NEW (AC4 unit tests + util test) | +98 |
| `tests/integration/d7/d7_frame_delta_e2e_test.go` | MODIFIED (5-cycle e2e TestIntegration_D7FrameDelta_Phase1And2SpanTrigger) | +145 |
| **Total** | | **+380** |

## 5. 域文档同步

| 文件 | 改动 |
|------|------|
| `openspec/specs/d7-orchestration/mups-frame-delta-spec.md` | NEW §3.4 "Phase 1+2 e2e span 触发条件 (testutil — DM-20260706-001)" + NEW §3.5 "Phase 2 production wiring (production-side — DM-20260706-004)" |
| `openspec/specs/d7-orchestration/t-registry.md` | NEW L5-MUPS-FD-6 (T20) entry; D7-FD Total 19→21 T, 18→20 IMPLEMENTED |
| `openspec/specs/d7-orchestration/CHANGELOG.md` | NEW 2026-07-08 entry |

## 6. SPEC 实施矩阵

| spec 段 | AC 描述 | 实施状态 | 实施路径 |
|---------|--------|---------|----------|
| §3.2 FrameDelta 5 字段 + ConvergenceMetric 3 字段 | 协议字段契约 | NOT_MODIFIED | 父 change DM-20260705-010 已落地 |
| §3.1 Observe→Plan 注入点 | 首轮零值边界 | NOT_MODIFIED | 父 change DM-20260705-010 + PR #437 |
| §3.2 Plan→Execute 注入点 | ≤200 字符 budget 防御 | NOT_MODIFIED | 父 change DM-20260705-010 + PR #443 |
| §3.3 Execute→Observe 回写 | deterministic 0 LLM | NOT_MODIFIED | 父 change DM-20260705-010 + PR #444 |
| **§3.4 Phase 1+2 e2e span 触发 (NEW)** | SequenceLLMStub FrameDeltaInject callback + SeedPriorExecContext helper | **IMPLEMENTED** | **本 change PR #466** |
| §3.5 Phase 2 production wiring | observation_proposer.go:257 nil → prevExecCtx 上游传参 | DEFERRED_TO_SIBLING | sibling DM-20260706-004 PR #467 |
| §4 Span 契约 | 3 span ops | PARTIAL (Phase 1+2 production emit 仅 zero-value branch) | sibling DM-20260706-004 PR #467 切换 non-zero |
| §5 AC1-AC8 | 8 AC | 7 PASS + AC8 DEFERRED (T19 follow-up) |  |

## 7. 验收决策

**ACCEPT** — DM-20260706-001 S4 实施完整闭环:
- ✅ PR #466 squash merged 2026-07-08
- ✅ 26/26 orchestration packages -race PASS
- ✅ tests/integration/d7/... PASS (5 子测试)
- ✅ go vet ./... PASS
- ✅ t-registry + CHANGELOG + spec delta §3.4 同步就位

**Sibling follow-up:**
- DM-20260706-004 PR #467 等待 CI merge (production-side wire-up)
- AC2 baseline 2 → 目标 ≥2 non-zero spans after sibling PR
- T19 三方 review follow-up

## 8. 已知限制 / Future Work

1. **Phase 2 e2e span 数 ≥ 5 目标** 需 sibling DM-20260706-004 production wiring + multi-session harness (本期 out of scope, design.md §1.3 已文档化)。
2. **T19 S3-Gate 三方 review** (codex + cursor quota 待恢复 + claude 内部 review) — follow-up change。
3. **running system 真实飞书 session Jaeger trace 重放** — out of scope,需 user action。

## 9. 签名

- S4 实现 / 单元测试: Agent executor via Claude Code (`feat/dm-20260706-001-s4-testutil-frame-delta` branch)
- S4-Gate: reviewer 待 S5 验收后指派
- S5 验收: 本报告 (Agent S5 self-verifier, local dev env)
- S6 归档: 本 change + sibling DM-20260706-004 同期归档
