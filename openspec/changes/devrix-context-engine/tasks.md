# Tasks: devrix-context-engine

**Change ID:** devrix-context-engine
**Status:** Draft (Design Phase — 无代码任务)
**Based on:** design.md, specs/context-engine/spec.md

---

## Milestone 1: 领域与配置基础

### Definition of Done
- [ ] 类型与配置编译通过
- [ ] L5-CTX-01/02 测试点已登记

### Tasks

- [ ] **T1**: 新增 `types/context.go`（SessionContext, PEVState, TokenBudget）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-01, L5-CTX-02
  - Estimate: 4h

- [ ] **T2**: 新增 `errors/context.go`（CTX_* 错误码）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-04
  - Estimate: 2h

- [ ] **T3**: 新增 `config/contextengine.go` + `devrix.yaml` 配置块
  - L4: L4-CTX-STATE
  - L5: —
  - Estimate: 3h

---

## Milestone 2: 分层记忆与快照

- [ ] **T4**: 实现 `memory/manager.go` + working/shortterm
  - L4: L4-CTX-MEMORY
  - L5: L5-CTX-01, L5-CTX-02
  - Estimate: 6h

- [ ] **T5**: 实现 `snapshot/store.go`（ContextSnapshot v1 JSON）
  - L4: L4-CTX-MEMORY
  - L5: L5-CTX-05
  - Estimate: 4h

- [ ] **T6**: `longterm_stub.go` 返回 FeatureNotImplemented
  - L4: L4-CTX-MEMORY
  - L5: L5-CTX-10
  - Estimate: 1h

---

## Milestone 3: 七步压缩管道

- [ ] **T7**: 实现 `compression/pipeline.go` + 步骤 1-5
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03
  - Estimate: 8h

- [ ] **T8**: 实现 `token_block.go` + `autocompact_stub.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-04, L5-CTX-08
  - Estimate: 4h

- [ ] **T9**: 实现 `token/counter.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03
  - Estimate: 3h

- [ ] **T10**: 压缩单元测试 + golden cases
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03, L5-CTX-04
  - Estimate: 6h

---

## Milestone 4: PEV Engine

- [ ] **T11**: 定义 `contracts.go`（ILLMGateway, IToolRunner, IObserver）
  - L4: L4-CTX-PEV
  - L5: —
  - Estimate: 2h

- [ ] **T12**: 实现 `pev/execute.go`（LLM 流式 + tool_call 事件）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-06
  - Estimate: 8h

- [ ] **T13**: 实现 `pev/verify.go`（basic 模式）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-07
  - Estimate: 4h

- [ ] **T14**: 实现 `pev/engine.go`（迭代控制）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-06, L5-CTX-07
  - Estimate: 4h

---

## Milestone 5: ContextEngine 集成

- [ ] **T15**: 实现 `engine.go`（Process 主流程）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-09
  - Estimate: 8h

- [ ] **T16**: 实现 `prompt/loader.go`（AGENTS.md）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-01
  - Estimate: 3h

- [ ] **T17**: `main.go` 注入 ContextEngine，移除 Stub 默认路径
  - L4: L4-CTX-STATE
  - L5: L5-CTX-09
  - Estimate: 2h

- [ ] **T18**: Mock LLM + Mock ToolRunner 用于测试
  - L4: L4-CTX-PEV
  - L5: L5-CTX-06
  - Estimate: 4h

---

## Milestone 6: 测试与验收

- [ ] **T19**: 集成测试 `tests/integration/context_gateway_flow_test.go`
  - L4: L4-CTX-STATE
  - L5: L5-CTX-05, L5-CTX-09
  - Estimate: 4h

- [ ] **T20**: 验收测试 `tests/acceptance/p0/ctx_compression_test.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03, L5-CTX-04
  - Estimate: 4h

- [ ] **T21**: 更新 `openspec/l5-registry.md` IMPLEMENTED 状态
  - L5: L5-CTX-01 ~ L5-CTX-10
  - Estimate: 1h

- [ ] **T22**: S5 `./scripts/gen-acceptance-report.sh --change devrix-context-engine`
  - L5: 全部 P0
  - Estimate: 1h

---

## 任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 基础 | 3 | 9h |
| M2 记忆 | 3 | 11h |
| M3 压缩 | 4 | 21h |
| M4 PEV | 4 | 18h |
| M5 集成 | 4 | 17h |
| M6 测试 | 4 | 10h |
| **合计** | **22** | **~86h** |

---

## V2/V3  backlog（本变更不实施）

- [ ] V2: `autocompact.go` LLM 摘要（步骤 6）
- [ ] V2: PEV verify `commands` 模式
- [ ] V3: `pev/plan.go` + Milestone DAG 对接
- [ ] V3: `memory/longterm.go` SQLite
