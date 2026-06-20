# Tasks: Context Budget & Isolation — Phase B

**Change ID:** 2026-06-20-devrix-context-budget-and-isolation-phase-b
**Status:** S4_Implementation

---

## B.1: AC6 + AC9 — Mode + Depth + default brief

**PR:** feat/context-budget-phase-b → B.1 PR (squash auto-merge)
**依赖**: —
**风险**: Low (新增字段,默认 brief,旧调用方无感)

### 任务列表

- [ ] **B.1.1** `internal/shared/contracts/subturn.go` — `SubAgentMode` 类型 + 3 mode 常量 + `SubTurnRequest.Mode`/`Depth` 字段
- [ ] **B.1.2** `internal/shared/errors/subturn.go` (新) — `ErrSubagentDepthExceeded` SentinelError
- [ ] **B.1.3** `internal/shared/config/orchestration.go` — `ContextSubagentConfig` (DefaultMode / LegacyMode / MaxDepth)
- [ ] **B.1.4** `internal/layers/orchestration/turn/subturn.go` — `SubTurnConfig` + 3-mode dispatch + depth check
- [ ] **B.1.5** `internal/layers/orchestration/turn/subturn_test.go` — 3-mode × depth 边界测试
  - `TestSubTurnRunner_BriefMode_PreloadedMessagesNil` (D7-S2-A06-T14)
  - `TestSubTurnRunner_ForkMode_DispatchesAsFork` (D7-S2-A06-T14 variant)
  - `TestSubTurnRunner_FullMode_BackwardCompat` (D7-S2-A06-T15)
  - `TestSubTurnRunner_DefaultModeFromConfig` (D7-S2-A06-T17)
  - `TestSubTurnRunner_DepthLimit_Equals` (D7-S2-A06-T16)
  - `TestSubTurnRunner_DepthLimit_Exceeds` (D7-S2-A06-T16)
  - `TestSubTurnRunner_DepthLimit_BoundaryAtMaxMinus1` (D7-S2-A06-T16)
- [ ] **B.1.6** `internal/layers/contextengine/enforce/subquery.go` — `SubQueryParams.Mode` + 透传 Depth
- [ ] **B.1.7** `internal/bootstrap/wire_coordinator.go` — `NewSubTurnRunner(orch, cfg)` 配置注入
- [ ] **B.1.8** `devrix.yaml` — `context.subagent.*` schema 段
- [ ] **B.1.9** unit test 全绿 + go vet + layer-lint

---

## B.2: AC10 — delegate/free_fork schema mode 字段

**PR:** feat/context-budget-phase-b → B.2 PR (squash auto-merge)
**依赖**: B.1 (Mode 字段已存在)
**风险**: Low (schema 扩展,缺省 brief)

### 任务列表

- [ ] **B.2.1** `internal/layers/orchestration/delegatetools/freefork.go` — `free_fork` tool schema 加 `mode?: "brief"|"fork"|"full"` 字段
- [ ] **B.2.2** `internal/layers/orchestration/delegatetools/delegate_tools.go` — `delegate` tool schema 加 `mode?` 字段
- [ ] **B.2.3** `internal/layers/orchestration/delegatetools/freefork_schema_test.go` (新) — json schema 解析验证 (D4-S4-A07-T02)
- [ ] **B.2.4** `internal/layers/orchestration/delegatetools/delegate_schema_test.go` (新) — json schema 解析验证 (D4-S4-A07-T01)
- [ ] **B.2.5** integration test — D4 surface delegate 时显式传 mode=full, SubQueryParams.Mode 透传到 SubTurnRequest.Mode
- [ ] **B.2.6** unit test 全绿 + go vet + layer-lint

---

## B.3: AC8 + AC11a — full 模式 + fork 模式 prefix 稳定

**PR:** feat/context-budget-phase-b → B.3 PR (squash auto-merge)
**依赖**: B.1 (SubTurnRunner 接受 Mode)
**风险**: Med (fork prefix 字节级稳定需严格测试)

### 任务列表

- [ ] **B.3.1** `internal/layers/orchestration/turn/subturn_fork_test.go` (新) — fork prefix sibling 字节级稳定测试
  - `TestSubTurnRunner_ForkSiblingPrefixStable` (D2-S15-A08-T06)
  - `TestSubTurnRunner_ForkPrefix_ContainsPlaceholder` (D2-S15-A08-T08)
- [ ] **B.3.2** `internal/layers/orchestration/turn/subturn_test.go` — 补 `mode=full` 行为等价 Phase A 测试 (D2-S15-A08-T07)
- [ ] **B.3.3** integration test — 跑 10 个 sibling fork sub-agent, prefix fingerprint 100% 一致
- [ ] **B.3.4** unit test 全绿 + go vet + layer-lint

---

## B.4: docs + legacy mode tests + 切换链路 verify

**PR:** feat/context-budget-phase-b → B.4 PR (squash auto-merge)
**依赖**: B.1-B.3
**风险**: Low (docs + tests)

### 任务列表

- [ ] **B.4.1** `openspec/specs/d7-orchestration/spec.md` — 3-mode + depth Gherkin scenarios
- [ ] **B.4.2** `openspec/specs/d4-multi-agent/spec.md` — mode 字段 + Gherkin scenarios
- [ ] **B.4.3** `openspec/specs/d7-orchestration/t-registry.md` — +4 P0 T 点 (D7-S2-A06-T14-T17)
- [ ] **B.4.4** `openspec/specs/d4-multi-agent/t-registry.md` — +2 P0 T 点 (D4-S4-A07-T01-T02)
- [ ] **B.4.5** `openspec/specs/d2-context-engine/t-registry.md` — +3 P0 T 点 (D2-S15-A08-T06-T08)
- [ ] **B.4.6** `docs/context-budget.md` (新) — mode 选型指南 + legacy_mode 切换说明
- [ ] **B.4.7** integration test — 完整链路 `delegate mode=full` → D2 subquery → D7 SubTurnRunner depth=2 → LLM call
- [ ] **B.4.8** unit test 全绿 + go vet + layer-lint

---

## B.5: AC12 — D5 spans 22 步复跑回归

**PR:** feat/context-budget-phase-b → B.5 PR (squash auto-merge)
**依赖**: B.1-B.3
**风险**: Med (依赖所有 sub-agent mode 落地)

### 任务列表

- [ ] **B.5.1** `tests/fixtures/d5-spans-replay.jsonl` (新) — D5 spans 设计任务原 prompt + 22 步用户输入
- [ ] **B.5.2** `tests/acceptance/p0/d5_spans_replay_test.go` (新) — 22 步 prompt_tokens P95 ≤ 40K 验证 (D5-DIAG-T06)
- [ ] **B.5.3** `tests/fixtures/d5-spans-replay-bench.json` (新) — 22 步 token 增长曲线 benchmark
- [ ] **B.5.4** `openspec/specs/d5-observability/t-registry.md` — D5-DIAG-T06 登记
- [ ] **B.5.5** integration test 跑通; benchmark artifact 保存
- [ ] **B.5.6** 若 P95 > 40K → 调整 default_mode 或 depth, 重跑

---

## 总览

| Sub-PR | AC | T 点 (P0) | 文件数 | 风险 | 估时 |
|--------|-----|-----------|--------|------|------|
| B.1 | AC6+AC9 | 4 (D7-S2-A06-T14-T17) | 8 | Low | 1 天 |
| B.2 | AC10 | 2 (D4-S4-A07-T01-T02) | 4 | Low | 0.5 天 |
| B.3 | AC8+AC11a | 3 (D2-S15-A08-T06-T08) | 3 | Med | 1 天 |
| B.4 | docs+tests | 0 (registry sync) | 6 | Low | 0.5 天 |
| B.5 | AC12 | 1 (D5-DIAG-T06) | 4 | Med | 1 天 |
| **合计** | **8 AC** | **10 P0 T** | **25** | — | **4 天** |
