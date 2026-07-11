# Tasks: D7 Observe 节点全协议修订 + 实现债闭环

**Change ID:** `devrix-d7-observe-node-spec`
**Demand ID:** DM-20260711-001
**Activity ID:** D7-S5-A122（消歧：A121 已属 DM-20260706-008）
**T-layer registration:** `openspec/specs/d7-orchestration/t-registry.md` §D7-S5-A122
**Status legend:** PLANNED / IN_PROGRESS / IMPLEMENTED / DEFERRED

---

## Phase A — S3 方案设计（P0）✅

| T ID | L4 | Status | Description | Evidence |
|------|-----|--------|-------------|----------|
| D7-S5-A122-T01 | `observe_node_protocol` | IMPLEMENTED | 全节点 spec `observe-node-spec.md` §1–§12 | `openspec/specs/d7-orchestration/observe-node-spec.md` |
| D7-S5-A122-T02 | `observe_node_protocol` | IMPLEMENTED | LLM I/O Review 版 `d7-observe-llm-io-protocol-spec.md` | 同目录 |
| D7-S5-A122-T03 | `observe_node_protocol` | IMPLEMENTED | change 包 demand/proposal/design/delta spec | archive 包 |

## Phase B — 文档收尾 Wave 1（P0）✅

| T ID | L4 | Status | Description | Evidence |
|------|-----|--------|-------------|----------|
| D7-S5-A122-T04 | `observe_node_protocol` | IMPLEMENTED | `spec.md` lite §12 + CHANGELOG + t-registry A122 | `spec.md`, `CHANGELOG.md`, `t-registry.md` |

## Phase C — P1 fast-path 选题 Wave 2（P0）✅

| T ID | L4 | Status | Description | Evidence |
|------|-----|--------|-------------|----------|
| D7-S5-A122-T05 | `observe_fastpath_pick` | IMPLEMENTED | `pickHighStrengthBusinessFact` 两遍扫描 + echo 排除 | `deliverable_execute.go`, `item_pipeline.go` |
| D7-S5-A122-T06 | `observe_fastpath_pick` | IMPLEMENTED | 单元测试 LLM 优先于 echo | `deliverable_execute_test.go` |
| D7-S5-A122-T07 | `observe_fastpath_pick` | IMPLEMENTED | 集成 prior≥0.85 回归 | `item_pipeline_fastpath_test.go` |
| D7-S5-A122-T08 | `observe_node_merge` | IMPLEMENTED | U08 deliverable incomplete trace | `observe_trace_e2e_test.go` |

## Phase D — P2 CatSystem + P3 scope（P1，Wave 3）✅

| T ID | L4 | Status | Description | Evidence |
|------|-----|--------|-------------|----------|
| D7-S5-A122-T09 | `observe_category_promote` | IMPLEMENTED | `observe_category_promote.go` + merge wiring | `observation_proposer.go` |
| D7-S5-A122-T10 | `observe_llm_classifier` | IMPLEMENTED | LLM frame 省略 `scope_open_question` | `llm_observation_proposer.go` |

## Phase E — P4 signal + P5 SoT（P1，Wave 4）

| T ID | L4 | Status | Description | Evidence |
|------|-----|--------|-------------|----------|
| D7-S5-A122-T11 | `observe_signal_registry` | IMPLEMENTED | registry + `buildObserveSignalInput` | `observe_signal_registry.go` |
| D7-S5-A122-T12 | `observe_llm_classifier` | DEFERRED | `observeLLMFieldMap` → pt-tag 派生（P1 例外） | follow-up change |

## Phase F — 验证与归档（P0）✅

| T ID | L4 | Status | Description | Evidence |
|------|-----|--------|-------------|----------|
| D7-S5-A122-T13 | — | IMPLEMENTED | 全量 orchestration test PASS | 26/26 `-race` |
| D7-S5-A122-T14 | — | IMPLEMENTED | acceptance-report.md + S5 验收 | `acceptance-report.md` |

---

**S7 归档**：2026-07-11 — T01–T11、T13–T14 IMPLEMENTED；T12 DEFERRED（P1 例外已记入 acceptance-report）
