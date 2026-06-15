---
change-id: devrix-d7-turn-orchestration
demand-id: DM-20260614-020
phase: v2.0 Structure (Phase D a-f)
verdict: PASS
date: 2026-06-15
---

# Acceptance Report — D7 Turn 编排上移 v2.0 Structure

## 1. 验收范围

| 维度 | 范围 |
|------|------|
| Change | `devrix-d7-turn-orchestration`（DM-20260614-020） |
| Phase | **v2.0 Structure（D a–f）** |
| 约束 | Go 代码实施；接口契约落地；import lint 守卫 |

## 2. Phase D — v2.0 Structure 产出

| Slice | Task | T | 产出文件 | 状态 |
|-------|------|---|---------|------|
| D-a | `orchestration/turn/` 骨架 | — | `turn/doc.go`, `contracts.go`, `orchestrator.go`, `llm.go` | ✅ |
| D-b | bootstrap WireContextLLM → D7 | A07 | `bootstrap/turn_wiring.go`; `wire_coordinator.go` 接受 llmStack | ✅ |
| D-c | FastPath → TurnOrchestrator | A06 P0 | `bootstrap/turn_adapter.go`; `orchestrator.go` 完整 runLoop; `contextengine/engine.go` getter | ✅ |
| D-d | D2 移除 ILLMGateway + import lint | THIN-T01 | `lint/layer/d2_d3_ban_test.go` D2→D3 导入守卫 | ✅ |
| D-e | Autocompact D7→D3 | S15-T10 | d2Adapter TokenCounter + CompressHint; runCompress D7→D3 | ✅ |
| D-f | Legacy adapter + 全量 T 绿 | P0 19+ | `d2Executor` 保留向后兼容; `turnOrchExecutor` 为主执行器 | ✅ |

## 3. 代码变更统计

| 类型 | 数量 | 文件 |
|------|------|------|
| 新增 | 7 | `turn/doc.go`, `contracts.go`, `orchestrator.go`, `llm.go`, `turn_wiring.go`, `turn_adapter.go`, `d2_d3_ban_test.go` |
| 修改 | 5 | `main.go`, `obs-verify/main.go`, `wire_coordinator.go`, `engine.go`, `tasks.md` |

## 4. 架构关键路径验证

| 路径 | 实现 | 状态 |
|------|------|------|
| D7→D3 LLM 调用 | GatewayInvoker.InvokeStream → IGateway.Stream | ✅ |
| PREPARE→LLM↔TOOLS→PERSIST | DefaultOrchestrator.runLoop 状态机 | ✅ |
| D2 拆面适配 | d2Adapter 实现 ContextPreparer/ToolRoundExecutor/SessionPersister | ✅ |
| CompressHint 回路 | Prepare 检测 → D7 runCompress → D3 摘要 | ✅ |
| D2→D3 导入守卫 | d2_d3_ban_test.go（4 项已知违规，禁止新增） | ✅ |
| Legacy 兼容 | d2Executor 保留; QueryLoopExecutor 接口不变 | ✅ |
| Layer lint | 全部 13 项测试通过（含新增 D2→D3 ban） | ✅ |

## 5. 测试结果

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./...` | 全 PASS（0 FAIL） |
| `go test ./internal/lint/layer/` | 13/13 PASS（含 D2→D3 ban） |

## 6. 已知技术债务（后续 Slice）

| 项 | 归属 | 说明 |
|----|------|------|
| D2→D3 已知违规 4 项 | D-d 后续 | engine.go, llm_logger.go, mock/llm.go, query/adapters.go, prepare/compression/（需独立 Phase E） |
| ~~旧 d2Executor 移除~~ | ~~D-f 后续~~ | ✅ 已完成（commit a6356bc） |
| D2 EngineDeps.LLM 字段移除 | D-d 后续 | 需 D2 内部重构拆分 |
| ~~runCompress 降级策略~~ | ~~D-e 后续~~ | ✅ 已完成：LLM → Truncation → Passthrough 三级降级 |

## 7. 裁决

**Verdict: PASS — v2.0 Structure 验收通过**

- ✅ 6 个 Slice（D-a 到 D-f）全部完成
- ✅ D7→D3 LLM 直接调用路径已打通
- ✅ TurnOrchestrator 完整状态机实现
- ✅ D2 拆面三接口适配器落地
- ✅ CompressHint 回路 D7→D3 实施
- ✅ D2→D3 import lint CI 守卫就位
- ✅ 全量测试通过，Legacy 兼容保留

Phase A–D 全部完成，DM-020 D7 Turn 编排上移可进入 S6 归档。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-06-15 | v1.0 Registry 验收通过 |
| 2.0 | 2026-06-15 | v2.0 Structure 验收通过 |
