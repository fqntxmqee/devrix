# Tasks: devrix-context-engine

**Change ID:** devrix-context-engine
**Status:** Implementation In Progress
**Based on:** design.md, specs/context-engine/spec.md

---

## Milestone 1: 领域与配置基础

### Definition of Done
- [x] 类型与配置编译通过
- [x] L5-CTX-01/02 测试点已登记

### Tasks

- [x] **T1**: 新增 `types/context.go`（SessionContext, PEVState, TokenBudget）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-01, L5-CTX-02
  - Estimate: 4h

- [x] **T2**: 新增 `errors/context.go`（CTX_* 错误码）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-04
  - Estimate: 2h

- [x] **T3**: 新增 `config/contextengine.go` + `devrix.yaml` 配置块
  - L4: L4-CTX-STATE
  - L5: —
  - Estimate: 3h

---

## Milestone 2: 分层记忆与快照

- [x] **T4**: 实现 `memory/manager.go` + working/shortterm
  - L4: L4-CTX-MEMORY
  - L5: L5-CTX-01, L5-CTX-02
  - Estimate: 6h

- [x] **T5**: 实现 `snapshot/store.go`（ContextSnapshot v1 JSON）
  - L4: L4-CTX-MEMORY
  - L5: L5-CTX-05
  - Estimate: 4h

- [x] **T6**: `longterm_stub.go` 返回 FeatureNotImplemented
  - L4: L4-CTX-MEMORY
  - L5: L5-CTX-10
  - Estimate: 1h

---

## Milestone 3: 七步压缩管道

- [x] **T7**: 实现 `compression/pipeline.go` + 步骤 1-5
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03
  - Estimate: 8h

- [x] **T8**: 实现 `token_block.go` + `autocompact_stub.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-04, L5-CTX-08
  - Estimate: 4h

- [x] **T9**: 实现 `token/counter.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03
  - Estimate: 3h

- [x] **T10**: 压缩单元测试 + golden cases
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03, L5-CTX-04
  - Estimate: 6h

---

## Milestone 4: PEV Engine

- [x] **T11**: 定义 `contracts.go`（ILLMGateway, IToolRunner, IToolRegistry, IPermissionGate, IObserver）
  - L4: L4-CTX-PEV
  - L5: —
  - Estimate: 2h

- [x] **T12**: 实现 `pev/execute.go`（LLM 流式 + tool_call 事件）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-06
  - Estimate: 8h

- [x] **T13**: 实现 `pev/verify.go`（basic 模式）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-07
  - Estimate: 4h

- [x] **T14**: 实现 `pev/engine.go`（迭代控制）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-06, L5-CTX-07
  - Estimate: 4h

---

## Milestone 5: ContextEngine 集成

- [x] **T15**: 实现 `engine.go`（Process 主流程 + EngineEvent 契约 emit）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-09
  - Estimate: 8h
  - Note: 流式 StreamBuffer 合并、RequestID 幂等、IPermissionGate 注入

- [x] **T16**: 实现 `prompt/loader.go`（AGENTS.md）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-01
  - Estimate: 3h

- [x] **T17**: `main.go` 注入 ContextEngine，移除 Stub 默认路径
  - L4: L4-CTX-STATE
  - L5: L5-CTX-09
  - Estimate: 2h

- [x] **T18**: Mock LLM + Mock ToolRunner 用于测试
  - L4: L4-CTX-PEV
  - L5: L5-CTX-06
  - Estimate: 4h

---

## Milestone 6: 测试与验收

- [x] **T19**: 集成测试 `tests/integration/context_gateway_flow_test.go`
  - L4: L4-CTX-STATE
  - L5: L5-CTX-05, L5-CTX-09, L5-CTX-11
  - Estimate: 4h

- [x] **T20**: 验收测试 `tests/acceptance/p0/ctx_compression_test.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-03, L5-CTX-04
  - Estimate: 4h

- [x] **T21**: 更新 `openspec/l5-registry.md` IMPLEMENTED 状态
  - L5: L5-CTX-01 ~ L5-CTX-10
  - Estimate: 1h

- [x] **T22**: S5 `./scripts/gen-acceptance-report.sh --change devrix-context-engine`
  - L5: 全部 P0
  - Estimate: 1h

---

## Milestone 7: L1-L2 集成契约

- [x] **T23**: 实现 `IPermissionGate` + `PermissionGateAdapter`（gateway 适配器）
  - L4: L4-CTX-PEV, L4-COMM-PERM
  - L5: L5-CTX-11
  - Estimate: 4h
  - Note: Gateway `tool_call` handler 改为仅展示，移除权限阻塞

- [x] **T24**: Gateway 实现 `Stopper` + `activeProcesses` 生命周期
  - L4: L4-COMM-GW, L4-COMM-CMD
  - L5: L5-CTX-09（Process cancellation）
  - Estimate: 3h
  - Note: `/stop` → context.Cancel；RouteInbound 创建 WithCancel

- [x] **T25**: Gateway EngineEvent 消费对齐（is_complete / ToolInput / complete metadata）
  - L4: L4-CTX-STATE, L4-COMM-GW
  - L5: L5-CTX-09
  - Estimate: 2h

- [x] **T26**: 实现 `IToolRegistry` 最小内置集（V1 stub）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-06
  - Estimate: 2h

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
| M7 L1-L2 | 4 | 11h |
| **合计** | **26** | **~97h** |

---

## V2/V3  backlog（本变更不实施）

- [ ] V2: `autocompact.go` LLM 摘要（步骤 6）
- [ ] V2: PEV verify `commands` 模式
- [ ] V3: `pev/plan.go` + Milestone DAG 对接
- [ ] V3: `memory/longterm.go` SQLite
