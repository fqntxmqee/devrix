# Tasks: Observe 节点封闭式分类器定位强化

**Change ID:** `d7-observe-closed-classifier-prompt`
**Demand:** DM-20260705-009
**Status:** S4_Implementation

## Task Breakdown

| # | Task | File | T ID | 估算 | 依赖 |
|---|------|------|------|------|------|
| T1 | 强化 `observationTaskAppendixZHIntro` 措辞 (封闭式分类器定位) | `internal/layers/contextengine/i18n/format_hints_mups.go` | D7-S5-A99-T10 | 5 min | — |
| T2 | 强化 `observationTaskAppendixZHSuffix` 措辞 (signal 不足 → obs_uncertainty 引导) | 同上 | D7-S5-A99-T11 | 5 min | T1 |
| T3 | 同步 `observationTaskAppendixENIntro/ENSuffix` 英文版 | 同上 | D7-S5-A99-T12 | 5 min | T1+T2 |
| T4 | 同步 `prompttags_semantics_zh.go::observe.node_role` | `internal/layers/contextengine/i18n/prompttags_semantics_zh.go` | D7-S5-A99-T14 | 3 min | T1 |
| T5 | 同步 `prompttags_semantics_en.go::observe.node_role` | `internal/layers/contextengine/i18n/prompttags_semantics_en.go` | D7-S5-A99-T15 | 3 min | T4 |
| T6 | 新增 golden snapshot 测试 (format_hints_mups_observer_test.go) | `internal/layers/contextengine/i18n/format_hints_mups_observer_test.go` | D7-S5-A99-T16 | 15 min | T1-T5 |
| T7 | 新增集成测试 (observation_closed_classifier_test.go) | `internal/layers/orchestration/sessionorchestrator/observation_closed_classifier_test.go` | D7-S5-A99-T17 | 15 min | T1-T5 |
| T8 | 跑 `go test ./internal/layers/contextengine/i18n/... ./internal/layers/orchestration/sessionorchestrator/... -count=1` | — | AC5 | 5 min | T1-T7 |
| T9 | 跑 `go vet ./...` | — | AC5 | 2 min | T8 |
| T10 | 跑 `gofmt -l` | — | (style) | 1 min | T8 |
| T11 | 更新 `openspec/t-registry.md` (T10-T17) | `openspec/t-registry.md` | — | 5 min | T8 |
| T12 | 更新 `openspec/specs/d7-orchestration/spec.md` v4.26.0 → v4.26.1 (Last Updated 段) | `openspec/specs/d7-orchestration/spec.md` | — | 3 min | T8 |
| T13 | 写 `acceptance-report.md` (S5 验收) | `openspec/changes/d7-observe-closed-classifier-prompt/acceptance-report.md` | AC1-AC8 | 10 min | T8-T10 |
| T14 | commit + push branch | — | — | 2 min | T1-T13 |
| T15 | `gh pr create` + auto-merge | — | — | 2 min | T14 |
| T16 | S6 归档 (archive/ 复制 + 索引 + CHANGELOG) | `openspec/archive/2026-07-05-d7-observe-closed-classifier-prompt/` | — | 10 min | T15 |

**总估算**: ~85 min (实际不含 PR CI 等待)

## T Layer Registration

新增 8 T 点 (D7-S5-A99-T10~T17) 需在 `openspec/t-registry.md` 注册,标记 PLANNED → IMPLEMENTED 在 S4 PASS 后。

## Verification

- AC1: T1+T6 验证 (golden snapshot 检查"封闭式分类器" marker)
- AC2: T2+T6 验证 (golden snapshot 检查"signal 不足 → obs_uncertainty" marker)
- AC3: T3+T6 验证 (英文版 marker 检查)
- AC4: T4+T5 验证
- AC5: T8 验证 (8 现有测试 0 修改 PASS)
- AC6: T7 验证 (集成测试覆盖"开放式 directive + 无 signal")
- AC7: T8 验证 (现有链路不动)
- AC8: T8+T9+T10 验证 (P0 T 100% + 覆盖率 ≥ 80% + go vet 0 warning)
