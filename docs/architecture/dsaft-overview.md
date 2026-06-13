# DSAFT 架构总览

**Capability:** architecture-layering（可读版）  
**Canonical:** `openspec/specs/architecture/layering.md` v3.1.0  
**Last Updated:** 2026-06-13

---

## 1. 什么是 DSAFT

Devrix 使用 **DSAFT** 五层编号描述业务架构，从稳定到易变：

| 层 | 名称 | 编号格式 | 稳定性 | 含义 |
|----|------|----------|--------|------|
| **D** | Domain 领域 | D1–D6 + ORCH | 极高 | 顶层限界上下文，对应 `internal/layers/{domain}/` |
| **S** | Scenario 场景 | D{N}-S{M} | 高 | 域内模块/二级包 |
| **A** | Activity 活动 | D{N}-S{M}-A{XX} | 中 | 可发起的业务动作（输入→输出→状态变更） |
| **F** | Function Point 功能点 | …-F{NN} | 低 | A 编排的最小可测逻辑单元 |
| **T** | Test Point 测试点 | …-T{NN} | 最高 | 确定性验收锚点，注册于 `openspec/t-registry.md` |

**注册表规模（2026-06-12）：** 77 Activities · 98 Function Points · 130+ Test Points

---

## 2. 七域 + ORCH

```
                    ┌─────────────────────────────────────┐
                    │  D1 Communication (COMM)            │
                    │  Gateway · Adapters · EventBus …    │
                    └──────────────┬──────────────────────┘
                                   │ EngineEvent / Permission
                    ┌──────────────▼──────────────────────┐
                    │  D2 Context Engine (CTX)            │
                    │  QueryLoop · Compression · Memory … │
                    └───┬──────────────────────┬──────────┘
                        │ ILLMGateway          │ Fork/Delegate
            ┌───────────▼──────────┐    ┌────▼─────────────────┐
            │ D3 LLM Gateway (LLM) │    │ D4 Multi-Agent       │
            │ Adapter · Breaker …  │    │ Agent · Delegate …   │
            └──────────────────────┘    └──────────┬───────────┘
                                                     │ FlowEvent
                    ┌────────────────────────────────▼──────────┐
                    │ ORCH Orchestration (读模型，非 D7)          │
                    │ WorkPlan · ExecutionFlowHub · WaveScheduler │
                    └───────────────────────────────────────────┘

     D5 Observability ──span/metric/log──▶ 全域
     D6 Evolution     ──eval/probe──────▶ D2/D4/ORCH 质量门禁
```

| 域 | 目录 | 核心职责 |
|----|------|----------|
| **D1** | `internal/layers/communication/` | IM 入站/出站、会话、权限 UI、EventBus |
| **D2** | `internal/layers/contextengine/` | 上下文、压缩、QueryLoop、工具执行 |
| **D3** | `internal/layers/llmgateway/` | 模型适配、熔断、重试、Token 计数 |
| **D4** | `internal/layers/multiagent/` | Agent 生命周期、Fork/Join、Delegate Worker |
| **D5** | `internal/layers/observability/` | Tracing、Metrics、Logging、Coverage |
| **D6** | `internal/layers/evolution/` | EvalRun、探针、Delta、Layer lint |
| **ORCH** | `internal/layers/orchestration/` | WorkPlan 读模型、Flow 双通道、Wave 调度 |

---

## 3. D2 上下文域（现行重点）

> **2026-06-13：** D2-S1 PEV 已 **RETIRED**。生产路径为 **D2-S10 QueryLoop**（`query_loop.enabled=true` 默认）。

| S ID | Scenario | 包路径 | 状态 |
|------|----------|--------|------|
| D2-S1 | PEV | — | **RETIRED** |
| D2-S2 | Compression | `compression/` | IMPLEMENTED |
| D2-S3 | Memory | `memory/` | IMPLEMENTED |
| D2-S4 | Token | `token/` + shared counter | IMPLEMENTED |
| D2-S5 | Registry | `registry/` | IMPLEMENTED |
| D2-S6 | Snapshot | `snapshot/` | IMPLEMENTED |
| D2-S7 | Prompt | `prompt/` | IMPLEMENTED |
| D2-S8 | Sandbox | `toolrunner/`（命令策略） | IMPLEMENTED |
| D2-S9 | Harness | `harness/` + 根目录装配 | IMPLEMENTED（legacy 分支 `#deprecated`） |
| **D2-S10** | **QueryLoop** | `query/` · `usercontext/` · `attachments/` · `permission/` · `tasks/` · `transcript/` | **生产主路径** |
| D2-S11 | Queue | `queue/` | IMPLEMENTED |
| D2-S12 | Worktree | `worktree/` | IMPLEMENTED |
| D2-S13 | Conversation | `conversation/` | IMPLEMENTED |
| D2-S14 | Mock | `mock/` | 测试辅助 |

**D2 根包关键类型：**

- `ContextEngine`（`engine.go`）— 实现 `contracts.IEngine`，编排 Process 管线
- `toolrunner/` — `IToolRunner` / `IToolRegistry` 实现与 bash 沙箱
- `query/adapters.go` — D2→D3 LLM 调用适配

---

## 4. 生产路径 vs Legacy

| 配置 | 行为 |
|------|------|
| `query_loop.enabled: true`（**默认**） | Process → `query.Loop.Run`；Harness 仅首次 Bootstrap（若 `harness.enabled`） |
| `query_loop.enabled: false` | 触发 legacy Harness 压缩/装配分支（`#deprecated`，D6 PathRegressionProbe 监控） |
| `harness.enabled: false` | 跳过 Bootstrap/Preflight/Router/SystemPromptBuild 重路径 |

D6 **PathRegressionProbe**：生产环境 legacy 路径计数必须为 0。

---

## 5. 与其他文档的关系

| 需求 | 去哪里 |
|------|--------|
| 改代码放哪 | [code-map.md](./code-map.md) |
| 一次请求的时序 | [request-flow.md](./request-flow.md) |
| 跨层接口在哪定义 | [contracts-and-boundaries.md](./contracts-and-boundaries.md) |
| 新增 T 测试点 | `openspec/t-registry.md` |
| 新增 Activity | `openspec/a-registry.md` |
| CI 层违规检测 | `internal/lint/layer/` + D6 LayerViolationProbe |
