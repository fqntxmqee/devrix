# Tasks: D7 Observe ↔ LLM 5 场景输入输出协议沉淀

**T-layer registration path**: `openspec/specs/d7-orchestration/t-registry.md`
**Status legend**: PLANNED / IMPLEMENTED

## A. Spec Doc (P0)

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A120-T01 | IMPLEMENTED | §3 输入协议：11 字段全表 + observeLLMFieldMap 6 字段 + 5 字段过滤理由 | `d7-observe-llm-io-protocol-spec.md:69-118` |
| D7-S5-A120-T02 | IMPLEMENTED | §4 输出协议：顶层 JSON schema + 4 种 kind payload 契约 + 解析容错表 | `d7-observe-llm-io-protocol-spec.md:120-219` |
| D7-S5-A120-T03 | IMPLEMENTED | §5 场景 1：纯确定性问答（ObsFact fast-path 完整链路） | `d7-observe-llm-io-protocol-spec.md:222-259` |
| D7-S5-A120-T04 | IMPLEMENTED | §5 场景 2：纯不确定性（ObsUncertainty → Plan decompose） | `d7-observe-llm-io-protocol-spec.md:261-295` |
| D7-S5-A120-T05 | IMPLEMENTED | §5 场景 3：结构化信号（ObsSignal → Plan 走指标） | `d7-observe-llm-io-protocol-spec.md:297-327` |
| D7-S5-A120-T06 | IMPLEMENTED | §5 场景 4：异常检测（ObsDeviation + CatSystem → Anomalies） | `d7-observe-llm-io-protocol-spec.md:329-360` |
| D7-S5-A120-T07 | IMPLEMENTED | §5 场景 5（混合）：fact+uncertainty → fast-path 被 Gate 3 阻断 | `d7-observe-llm-io-protocol-spec.md:362-394` |
| D7-S5-A120-T08 | IMPLEMENTED | §7 Go-side invariants 表（cap / lift / inject / max / reject / fallback） | `d7-observe-llm-io-protocol-spec.md:407-420` |
| D7-S5-A120-T09 | IMPLEMENTED | §8 Partition 路由表（5 种 cat×kind 组合） | `d7-observe-llm-io-protocol-spec.md:422-434` |
| D7-S5-A120-T10 | IMPLEMENTED | §9 Test 覆盖表（16 cases → 5 场景映射） | `d7-observe-llm-io-protocol-spec.md:436-464` |

## B. Test 补充（P0）

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A120-T11 | IMPLEMENTED | 新增 TestObserveTraceE2E_FactPlusUncertainty_FastPathBlocked（场景 5 混合） | `internal/layers/orchestration/sessionorchestrator/observe_trace_e2e_test.go:887-953` |
| D7-S5-A120-T12 | IMPLEMENTED | 注释重编号：原"测试 15: 真实 WorkItem flow" → "测试 16" | `observe_trace_e2e_test.go:960` |

## C. 验证（P0）

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A120-T13 | PLANNED | `go test -v -run TestObserveTraceE2E ./internal/layers/orchestration/sessionorchestrator/...` 16/16 PASS | TBD |
| D7-S5-A120-T14 | PLANNED | `go vet ./...` 0 warning | TBD |
| D7-S5-A120-T15 | PLANNED | PR 提交（追加到 PR #472 或新 PR） | TBD |

**Total**: 15 T-points (10 spec sections + 2 test changes + 3 verification).
