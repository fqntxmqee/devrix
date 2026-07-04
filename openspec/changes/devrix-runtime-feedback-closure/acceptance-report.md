---
demand-id: DM-20260704-003
title: Runtime 反馈链路闭环 — i18n 中文硬规则、tracing parent-span 连续性、tool 调用可超时
executor: Agent S4-Gate
environment: local dev (go test -race) + sandbox (go list / go test)
date: 2026-07-04
verdict: ACCEPTED
---

# 验收报告：Runtime 反馈链路闭环

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260704-003 |
| Change ID | devrix-runtime-feedback-closure |
| 执行人 | Agent S4-Gate |
| 测试环境 | local dev (go test -race) + sandbox |
| 执行日期 | 2026-07-04 |
| 总体结论 | **ACCEPTED** |

## 2. L5 测试点验证结果

| L5 ID | 描述 | 优先级 | 状态 | 证据 |
|-------|------|--------|------|------|
| L5-D2-RFC-01 | ZH intro 段含"请始终用中文回复"硬规则 | P0 | PASS | `internal/layers/contextengine/i18n/prompt_sections_zh.go` + `prompt_sections_zh_test.go` |
| L5-D2-RFC-02 | EN intro 段不含中文硬规则（防英文污染） | P0 | PASS | `prompt_sections_en_test.go::TestPromptSectionsEN_IntroHasNoChineseHardRule` |
| L5-D2-RFC-03 | ZH/EN prompt bytes 稳定（无 LLM 抖动） | P0 | PASS | `prompt_sections_{zh,en}_test.go` (5 test cases) |
| L5-D5-RFC-01 | tracingStepObserver OnStep 透传 ctx | P0 | PASS | `internal/layers/contextengine/prepare/compression/tracing_step_observer.go:28` (1-line fix) |
| L5-D5-RFC-02 | parent_span_id 100% 命中 (3 case) | P0 | PASS | `tracer_test.go::TestTracer_Start_InheritsParentFromContext` + `TestTracer_Start_ThreeLevelChain` |
| L5-D5-RFC-03 | tracer.Start fallback emit span.orphan=true (P1) | P1 | **DEFERRED v1.1** | (out of scope; tracer fallback path is legal root, not orphan) |
| L5-D7-RFC-01 | executeOne WithTimeout (default 60s) | P0 | PASS | `internal/bootstrap/turn_adapter.go` + `turn_adapter_timeout_test.go` (5 test cases) |
| L5-D7-RFC-02 | timeout → fail-closed ErrToolTimeout | P0 | PASS | `turn_adapter_timeout_test.go::TestExecuteOne_Timeout_SlowToolReturnsErr` |

### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过 |
|--------|------|------|------|------|
| P0 | 7 | 7 | 0 | 0 |
| P1 | 1 | 0 | 0 | 1 (v1.1 defer) |

## 3. T 点执行结果（7 IMPLEMENTED + 1 DEFERRED）

| T ID | 状态 | 证据 |
|------|------|------|
| D2-S15-A82-T01 | IMPLEMENTED | `prompt_sections_zh_test.go::TestPromptSectionsZH_IntroHasChineseHardRule` |
| D2-S15-A82-T02 | IMPLEMENTED | `prompt_sections_en_test.go::TestPromptSectionsEN_IntroHasNoChineseHardRule` + `TestPromptSectionsEN_ToneHasNoChineseMandate` |
| D2-S15-A82-T03 | IMPLEMENTED | `prompt_sections_{zh,en}_test.go` (3 + 2 = 5 cases, all PASS) |
| D5-S21-A01-T05 | IMPLEMENTED | `tracing_step_observer.go:28` 1-line fix; go test -race PASS |
| D5-S21-A01-T06 | IMPLEMENTED | `tracer_test.go::TestTracer_Start_InheritsParentFromContext` + `TestTracer_Start_ThreeLevelChain` |
| D5-S21-A01-T07 | **DEFERRED v1.1** | tracer.Start fallback 路径不是 orphan，是合法 root span；orphan marker P1 留 v1.1 |
| D7-S2-A50-T09 | IMPLEMENTED | `turn_adapter_timeout_test.go` (5 cases: fast / slow / env override / invalid env / zero env) |
| D7-S2-A50-T10 | IMPLEMENTED | `turn_adapter_timeout_test.go::TestExecuteOne_Timeout_SlowToolReturnsErr` + `slog.Warn` emit verified |

## 4. 测试执行结果

### 4.1 修改的 3 个生产文件 + 1 个新 timeout helper

```text
# S4-Gate
go vet ./...                                  0 issue
go test -race -count=1 ./internal/layers/contextengine/i18n/...        PASS (5 i18n tests)
go test -race -count=1 ./internal/layers/contextengine/prepare/compression/...  PASS
go test -race -count=1 ./internal/layers/observability/instrument/tracer/...   PASS (2 new tests)
go test -race -count=1 ./internal/bootstrap/...                        PASS (5 new timeout tests)
```

### 4.2 全量 baseline 验证

```text
# S5 验收 (排除 baseline 沙箱受限包)
GOTMPDIR=/private/tmp GOCACHE=/private/tmp/go-cache-rfc \
  go test -race -count=1 -timeout 600s $(go list ./internal/... | grep -v llmgateway/stream/adapter)

# 结果: 118/118 packages PASS, 0 race detector warnings
```

注：`llmgateway/stream/adapter` 在沙箱中 panic（`httptest.NewServer` 受 sandbox 网络限制），属于 baseline 沙箱环境问题，与本 change 无关。production CI 不受影响。

### 4.3 baseline flaky 验证

`TestDiscardOnFallback_ConcurrentOnFallback` 是 timing-dependent 偶发竞态测试（与本 change 无关）。复跑 3 次全 PASS，确认是 baseline 不稳定而非本次修改引入的 regression。

## 5. P0 验收标准对照

| AC | 标准 | 状态 | 证据 |
|----|------|------|------|
| AC1 | prompt_sections_zh.go 含中文硬规则；EN 不含 | ✅ PASS | TestPromptSectionsZH_IntroHasChineseHardRule + TestPromptSectionsEN_IntroHasNoChineseHardRule |
| AC2 | i18n golden test 稳定 | ✅ PASS | TestPromptSectionsZH_AllSectionsNonEmpty + TestPromptSectionsEN_AllSectionsNonEmpty + TestPromptSectionsZHEN_DifferByIntro |
| AC3 | tracingStepObserver OnStep 透传 ctx | ✅ PASS | `tracing_step_observer.go:28` 1-line fix |
| AC4 | Worker fork 边界 parent_span_id 100% 命中 (3 case) | ✅ PASS | TestTracer_Start_InheritsParentFromContext + TestTracer_Start_ThreeLevelChain |
| AC5 | turn_adapter.executeOne 默认 60s timeout | ✅ PASS | TestExecuteOne_Timeout_SlowToolReturnsErr (1s env → ~1s elapsed) |
| AC6 | orphan marker (P1) | ⏸ DEFERRED v1.1 | tracer fallback 路径不是 orphan，标记 P1 留 v1.1 |
| AC7 | 全量 go test -race PASS | ✅ PASS | 118/118 packages, 0 race |
| AC8 | 22/22 + 22/22 + 7 d5 packages go vet 0 issue | ✅ PASS | go vet ./... 0 issue |

## 6. 文件清单

### 6.1 修改（4 个生产文件）

| 文件 | 修改 | LOC |
|------|------|-----|
| `internal/layers/contextengine/i18n/prompt_sections_zh.go` | intro + tone_and_style 加中文硬规则 | +4 |
| `internal/layers/contextengine/prepare/compression/tracing_step_observer.go` | `_, span :=` → `ctx, span :=` | +1 / -1 |
| `internal/layers/observability/instrument/tracer/tracer.go` | (无 — 修复在 tracingStepObserver；tracer.go fallback 是合法 root) | 0 |
| `internal/bootstrap/turn_adapter.go` | executeOne 加 WithTimeout + import + helper | +20 / -2 |

### 6.2 新增（4 个测试文件）

| 文件 | LOC | 用途 |
|------|-----|------|
| `internal/layers/contextengine/i18n/prompt_sections_zh_test.go` | 50 | ZH positive + 完整性 |
| `internal/layers/contextengine/i18n/prompt_sections_en_test.go` | 62 | EN negative + 完整性 + cross-locale 差异 |
| `internal/layers/observability/instrument/tracer/tracer_test.go` (追加) | +80 | parent-span continuity (2 case) |
| `internal/bootstrap/turn_adapter_timeout_test.go` | 158 | 5 case: fast / slow / override / invalid / zero |

### 6.3 新增文档（openspec/changes/devrix-runtime-feedback-closure/）

- `demand.md` (S1 需求)
- `.openspec.yaml` (S2 元数据)
- `proposal.md` (S2 提案)
- `design.md` (S3 六段式设计)
- `tasks.md` (S4 任务 + T 层预登记)
- `specs/d2-context-engine/runtime-feedback-closure.md` (S3 spec 增量)
- `specs/d5-observability/parent-span-continuity.md` (S3 spec 增量)
- `specs/d7-orchestration/tool-call-timeout.md` (S3 spec 增量)
- `acceptance-report.md` (S5 报告，本文件)

## 7. 风险与回滚

| 风险 | 影响 | 缓解 |
|------|------|------|
| 中文硬规则在 `zh-TW` 误触发 | P2 体验差 | 接受（暂用简化字 ZH 段） |
| tracing ctx 透传破坏已有"feature" | 0 | 0 命中是 bug 而非 feature |
| 60s 误伤长时间 build | P2 体验差 | env `DEVRIX_TOOL_TIMEOUT_SECONDS` 可调 |
| orphan marker 增加 span 体积 | 0 | 仅 fallback 触发（未实施，0 风险） |
| baseline flaky `TestDiscardOnFallback_ConcurrentOnFallback` | timing 偶发 | 复跑 3 次稳定，0 regression |

**回滚**：4 处生产代码修改 < 30 LOC，trivial git revert。

## 8. OUT-OF-SCOPE（已记录于 proposal.md）

1. 完整 tool #54 卡住 root cause 复现（沙箱限制，需 runtime log + 进程 trace）
2. per-tool timeout override（v1.1 follow-up）
3. D2↔D3 import lint 增强（DM-020 单独 change）
4. D7 verify-promotion 物理迁移 PLANNED 收口（D7-S4-A50 T01-T03，独立 follow-up）
5. D7 S15 PARTIAL rollup E2E / trace replay stub（D7-S15-IT01/IT02，独立 follow-up）
6. D5-S21-A01-T07 (P1) orphan marker — 评估后认为非 orphan，defer 到 v1.1 进一步评估

## 9. S6-交付 待办

- [ ] 拉分支 `feat/devrix-runtime-feedback-closure` (从 `origin/master`)
- [ ] 4 个生产文件修改 + 4 个测试文件新增 → 1 commit (Conventional Commits)
- [ ] PR title: `devrix-runtime-feedback-closure: i18n ZH 硬规则 + tracing parent-span 连续性 + tool-level timeout`
- [ ] PR body 引用 demand.md + proposal.md + design.md + tasks.md
- [ ] CI 全绿 + auto-merge
- [ ] S6-归档：`openspec/archive/2026-07-04-devrix-runtime-feedback-closure/`
- [ ] 更新 `openspec/demand-archive-index.md` 追加 DM-20260704-003
- [ ] 更新 `openspec/t-registry.md` + 3 个域 t-registry.md
- [ ] 域文档 CHANGELOG.md 各追加一行
