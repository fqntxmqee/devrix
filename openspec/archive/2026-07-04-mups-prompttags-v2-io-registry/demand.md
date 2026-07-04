---
demand-id: DM-20260704-005
title: "MUPS prompttags v2 — unified IO registry + uncertainty→determinism convergence"
priority: P0
status: S4_Implementation
dsaft_domain: shared, orchestration
created: 2026-07-04
related:
  - internal/shared/prompttags/
  - internal/layers/orchestration/sessionorchestrator/observation_proposer.go
  - internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go
  - openspec/specs/shared/prompttags.md
---

# MUPS prompttags v2 IO registry

## 1. 背景

DM-20260704-004 建立了 `prompttags` 包（envelope / wholebody / linefield user frames），但 I/O 形状仍分散在注释与 i18n 中，缺少统一注册表。Observe appendix 声明「最多 3 条提案」但 Go 侧未强制；Plan user prompt 遗漏 `uncertainty_mean` 注入；Observe 多轮可能重复提案。

## 2. 目标

推进 MUPS tag/IO 系统，使 LLM I/O 可靠支持 **uncertainty→determinism 收敛**：

1. 统一 IO Registry 文档化全部 MUPS I/O 形状（envelope / lineframe / wholebody）
2. Go 侧强制 Observe 提案上限 3（与 prompt 一致）
3. 登记 L5/T 测试点覆盖新不变量

## 3. 验收标准

| ID | 标准 | 优先级 | 状态 |
|----|------|--------|------|
| AC1 | `MUPSIOCatalog` / `LineFrameRegistry` 注册 envelope + ObserveUserFrame + PlanUserFrame + wholebody shapes | P0 | — |
| AC2 | `ValidateObservationProposals` 最多保留 3 条**首个有效**提案（与 i18n「最多 3 条」一致） | P0 | — |
| AC3 | `buildStrategicPlanUserPrompt` 在 `UncertaintyMean > 0` 时注入 `uncertainty_mean` | P1 | — |
| AC4 | Observe user frame 在存在 `LastRound.ObservationIDs` 时注入 `prior_observation_ids` + `incremental_only: true` | P1 | — |
| AC5 | `go test ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/...` PASS | P0 | — |

## 4. 不变量（L5）

| # | 不变量 | 实现 |
|---|--------|------|
| 1 | Parseability — 每种 output shape 对应唯一 profile | IO Registry |
| 2 | Monotonic scope tightening Observe→Plan→Execute | 引用现有 scope validate / Plan budget |
| 3 | Observe max 3 proposals enforced in Go | ValidateObservationProposals |
| 4 | Plan budget gates | 引用 applyBudgetCap（不重复实现） |
| 5 | Reject feedback loops | P2 defer |

## 5. 范围

### P0
- OpenSpec change 包
- `internal/shared/prompttags/` IO registry 扩展
- Observe max-3 cap + test
- t-registry 登记

### P1
- Plan `uncertainty_mean` 注入
- Observe incremental-only frame 字段

### P2（defer）
- Reject feedback loops

## 6. 约束

- 保持 `MUPSRegistry` / `Wrap` / `ExtractOne` API 向后兼容
- 不引入新依赖
- orchestration Go 不含战术散文
