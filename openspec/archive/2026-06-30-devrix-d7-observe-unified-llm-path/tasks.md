# Implementation Tasks: D7 Observe 统一 LLM 入口

**Change ID:** `devrix-d7-observe-unified-llm-path`  
**Demand ID:** DM-20260630-001

---

## Phase 1: 退役裸 D3 路径 (S4)

- [x] 1.1 移除 T35 裸 D3 wiring
- [x] 1.2 删除旧 `llm_observation_proposer.go`（英文写死 prompt）

---

## Phase 2: D2→D3 Observe 重写 (S4)

- [x] 2.1 重写 `llm_observation_proposer.go` — `Prepare` → Obs 附录 → `InvokeStream`
- [x] 2.2 `wire_item_pipeline.go` — wired `NewLLMObservationProposer(llm, ctx, locale)`
- [x] 2.3 `llm_observation_proposer_test.go` — D2-before-D3 + zh/en appendix
- [x] 2.4 `wire_item_pipeline_test.go` — 断言 proposer non-nil

**Quality Gate:**
- [x] `go test ./internal/bootstrap/... ./internal/layers/orchestration/sessionorchestrator/... -count=1`

---

## Phase 3: 规格同步 (S5 → S6)

- [x] 3.1 change delta + design/demand/proposal 更新
- [x] 3.2 合并至 `openspec/specs/d7-orchestration/spec.md` v4.19.0 (A75 D2→D3)
- [x] 3.3 更新 `t-registry.md` A75 T 点
- [x] 3.4 `acceptance-report.md`

---

## Completion Checklist

- [x] 代码变更完成
- [x] 单测 PASS
- [x] SoT spec 已写入 `openspec/specs/`
- [x] S6 PR 合入 main ([#330](https://github.com/fqntxmqee/devrix/pull/330))
- [x] S7 归档 → `openspec/archive/2026-06-30-devrix-d7-observe-unified-llm-path/`
