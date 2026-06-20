# Tasks: Context Budget & Isolation — Phase C

**Change ID:** 2026-06-20-devrix-context-budget-phase-c-nested
**Demand ID:** DM-20260620-002
**Created:** 2026-06-20

---

## C.1 — 透传 MaxContextTokens + nested 分支读取 (PR #1)

### T1.1 — TurnRequest 新增 MaxContextTokens 字段
- **文件**: `internal/layers/orchestration/turn/contracts.go`
- **改动**: TurnRequest struct +5 行 (字段 + 注释)
- **依赖**: 无
- **验收**: go vet 通过

### T1.2 — nested 分支读取 + fallback
- **文件**: `internal/layers/orchestration/turn/orchestrator.go:230`
- **改动**: nested 分支内 +4 行 (读 req.MaxContextTokens + fallback o.maxContextTokens)
- **依赖**: T1.1
- **验收**: 单测覆盖 fallback chain

### T1.3 — SubTurnRequest 新增 MaxContextTokens 字段
- **文件**: `internal/shared/contracts/subturn.go`
- **改动**: SubTurnRequest struct +5 行
- **依赖**: 无
- **验收**: go vet 通过

### T1.4 — SubTurnRunner.Cfg 新增 MaxContextTokens 字段
- **文件**: `internal/layers/orchestration/turn/subturn.go`
- **改动**: SubTurnConfig struct +1 行, RunSubTurn 注入 TurnRequest +5 行
- **依赖**: T1.1, T1.3
- **验收**: 单测覆盖 fallback

### T1.5 — SubQueryParams 新增 MaxContextTokens 字段 + 透传
- **文件**: `internal/layers/contextengine/enforce/subquery.go`
- **改动**: SubQueryParams struct +1 行, Run 中透传 +1 行
- **依赖**: T1.3
- **验收**: go vet 通过

### T1.6 — bootstrap 注入 MaxContextTokens
- **文件**: `internal/bootstrap/wire_coordinator.go`
- **改动**: `NewSubTurnRunner` 调用 +1 行 (MaxContextTokens: maxContextTokens)
- **依赖**: T1.4
- **验收**: 全量 build 通过

### T1.7 — 单测 TestNestedBranch_BudgetBypass_Reversed
- **文件**: `internal/layers/orchestration/turn/orchestrator_test.go`
- **改动**: 新增测试函数 +80 行
- **覆盖**: T1.2 fallback chain + audit + proactive fold 触发
- **依赖**: T1.2

### T1.8 — 单测 TestSubTurnRunner_MaxContextTokens_Propagated
- **文件**: `internal/layers/orchestration/turn/subturn_test.go`
- **改动**: 新增测试函数 +50 行
- **覆盖**: T1.4 fallback chain (req / Cfg)
- **依赖**: T1.4

### T1.9 — C.1 验证
- `go test -race ./internal/layers/orchestration/turn/...` PASS
- `go vet ./...` PASS
- `tools/layer-lint` PASS

---

## C.2 — 4 路并行 deep review fixture + integration test (PR #2)

### T2.1 — fixture 文件
- **文件**: `tests/fixtures/nested-4parallel-deep-review.jsonl` (新)
- **内容**: 4 worker × 10 步 = 40 事件
- **格式**: 每行 `{"worker": N, "step": M, "type": "...", ...}`
- **依赖**: 无

### T2.2 — integration test
- **文件**: `tests/integration/d7/nested_budget_test.go` (新)
- **改动**: ~150 行
- **覆盖**: 4 路并行 + audit/fold 断言 + LLM 0 reject
- **依赖**: T2.1, C.1

### T2.3 — C.2 验证
- `go test -tags integration -race ./tests/integration/d7/nested_budget_test.go` PASS

---

## C.3 — D5 spans 22 步回归 (PR #3 或同 C.2)

### T3.1 — D5 spans 回归
- **命令**: `go test -tags acceptance -race ./tests/acceptance/p0/d5_spans_replay_test.go`
- **断言**: prompt_tokens P95 ≤ 40K (Phase B baseline 不退化)
- **依赖**: C.1

---

## C.4 — docs + t-registry + S6 归档 (PR #4)

### T4.1 — docs/context-budget.md 新增 §"Nested branch budget injection (Phase C)"
- **内容**: nested 分支路径 + maxContextTokens 透传链路 + AC1/AC2 度量
- **依赖**: C.1, C.2

### T4.2 — openspec/specs/d7-orchestration/t-registry.md 新增 T18-T23
- **T18-T23**: 6 T 点 (AC1 5 T 点 + AC2 1 T 点)
- **依赖**: 无 (独立登记)

### T4.3 — openspec/specs/d2-context-engine/t-registry.md 新增 T09-T10
- **T09-T10**: 2 T 点 (SubQueryParams.MaxContextTokens 透传)
- **依赖**: 无

### T4.4 — openspec/t-registry.md 根索引加 8 T 点
- **依赖**: T4.2, T4.3

### T4.5 — S6 归档
- **目录**: `openspec/archive/2026-06-20-devrix-context-budget-phase-c-nested/`
- **命令**: `scripts/verify-archive.sh 2026-06-20-devrix-context-budget-phase-c-nested`
- **依赖**: C.1-C.3, T4.1-T4.4

---

## 验收检查清单

- [ ] C.1 全量绿 (go test -race ./internal/layers/orchestration/turn/...)
- [ ] C.1 go vet ./... 通过
- [ ] C.1 tools/layer-lint 通过
- [ ] C.2 integration test PASS (prompt_tokens ≤ 40K, 0 LLM reject)
- [ ] C.3 D5 spans 22 步 P95 ≤ 40K (不退化)
- [ ] C.4 docs 同步
- [ ] C.4 t-registry 8 T 点登记
- [ ] C.4 S6 归档 verify-archive.sh 12/12 PASS
- [ ] 全量 go test -race ./... 绿
- [ ] 全量 go vet ./... 通过
- [ ] 全量 tools/layer-lint 通过

## 工时估算 (仅参考, 不承诺)

| PR | 估算 | 说明 |
|----|------|------|
| C.1 | ~2 小时 | 透传字段 + 单测,改动量小 |
| C.2 | ~3 小时 | fixture 设计 + integration test |
| C.3 | ~0.5 小时 | 跑一遍 regression |
| C.4 | ~1 小时 | docs + t-registry + 归档 |
| **总计** | **~6.5 小时** | |

## 实施顺序

1. **C.1** (独立 PR): T1.1 → T1.2 → T1.3 → T1.4 → T1.5 → T1.6 → T1.7 → T1.8 → T1.9 验证
2. **C.2** (独立 PR): T2.1 → T2.2 → T2.3 验证
3. **C.3** (合并 C.2 PR): T3.1 验证
4. **C.4** (独立 PR): T4.1 → T4.2 → T4.3 → T4.4 → T4.5
5. **验收**: 全量 + verify-archive.sh