# 代码图谱（DSAFT ↔ 包路径）

**Canonical machine index:** `openspec/specs/architecture/code-atlas.md`  
**Last Updated:** 2026-06-13

新建或移动文件前，先确定 **D{N}-S{M}** 归属，再落盘到对应目录。Layer lint（`internal/lint/layer/`）与 D6 探针会在 CI 中校验 import 方向。

---

## 1. 顶层目录

```
internal/
├── layers/           # D1–D6 + ORCH 实现
├── bridges/          # 跨域装配（如 llm bridge）
├── bootstrap/        # 进程启动 wiring
├── shared/           # config · types · contracts · errors
└── cmd/devrix/       # CLI 入口
```

---

## 2. D1 Communication

| S ID | 包 | 入口/关键类型 |
|------|-----|----------------|
| D1-S13 | `communication/capture/` | `CommunicationGateway`, `RouteInbound`, session store |
| D1-S14 | `communication/thinking/` | Thinking 信号 emit |
| D1-S15 | `communication/taskprogress/` | Task/Tool/Worker 展示 |
| D1-S16 | `communication/conclusion/` | Conclusion 流式/终态/摘要 |
| D1-S17 | `communication/channel/` | adapters / connection / renderers / ratelimit |
| D1-S18 | `communication/delivery/eventbus/` | 背压、Drain、Reconnect |
| — | `communication/kernel/` | Card、OutboundMetadata |

**Legacy 映射：** 原 `gateway/`→`capture/`、`adapters/`→`channel/adapters/`、`eventbus/`→`delivery/eventbus/`、`core/`→`kernel/`。详见 `openspec/specs/architecture/code-layout.md`。

---

## 3. D2 Context Engine

| S ID | 包 | 关键类型 / 文件 |
|------|-----|-----------------|
| — | `contextengine/`（根） | `engine.go` · `contracts.go` · `tool_context.go` |
| D2-S2 | `compression/` | 七步管道、`AsyncAutocompacter` |
| D2-S3 | `memory/` | `Manager`, LongTerm SQLite |
| D2-S4 | `token/` | Token 预算（与 D3 counter 对齐） |
| D2-S5 | `registry/` | `builtin.go` 注册表 |
| D2-S5/D2-S8 | **`toolrunner/`** | `ToolRunner`, `Sandbox`, `DefaultCommandPolicy`, 内置工具 |
| D2-S6 | `snapshot/` | `Store` 快照读写 |
| D2-S7 | `prompt/` | `Loader`, `SystemPromptAssembler` |
| D2-S9 | `harness/` | `Bootstrap`, `ToolPool`, `Preflight`, `PromptRouter` |
| **D2-S10** | **`query/`** | **`Loop.Run`** — 多轮 LLM↔Tool 主循环 |
| D2-S10 | `usercontext/` | API 边界 prepend |
| D2-S10 | `attachments/` | Plan mode 附件 |
| D2-S10 | `permission/` | Plan 写过滤 |
| D2-S10 | `tasks/` | Task 磁盘持久化 |
| D2-S10 | `transcript/` | Sidechain JSONL |
| D2-S10 | `query/adapters.go` | `NewLLMCaller` — D2→D3 边界 |
| D2-S11 | `queue/` | `SessionQueue`, background 通知 |
| D2-S12 | `worktree/` | Delegate 沙箱目录 |
| D2-S13 | `conversation/` | 消息链修复、system 剥离 |
| D2-S14 | `mock/` | 测试用 LLM mock |

---

## 4. D3 LLM Gateway

| S ID | 包 | 关键类型 |
|------|-----|----------|
| — | `llmgateway/contracts.go` | **`ILLMGateway`**, **`ITierResolver`**（契约归属 D3） |
| D3-S1 | `llmgateway/route/` | RouteModel — 路由解析 |
| D3-S2 | `llmgateway/stream/`（含 `stream/adapter/`） | StreamChat — 流式调用 + DeepSeek/MiniMax 适配器 |
| D3-S3 | `llmgateway/protect/` | ProtectCall — 熔断 + 重试 + Full Jitter |
| D3-S4 | `llmgateway/budget/` | BudgetTokens — cl100k_base 计数 |
| D3-S5 | `llmgateway/guard/` | GuardContent — 内容安全过滤 |
| D3-S6 | `llmgateway/configure/` | ConfigureGateway — Provider 配置 |

**Bridge：** `internal/bridges/llm/` — 将 D3 实现注入 D2 `EngineDeps`。

---

## 5. D4 Multi-Agent

| S ID | 包 | 关键类型 |
|------|-----|----------|
| D4-S1 | `multiagent/factory/` | Agent 工厂 |
| D4-S2 | `multiagent/agent/` | `Impl`, `WorkerEngine`, `PermissionGate` |
| D4-S4 | `multiagent/collaboration/` | CoT / Iterative-Refinement |
| D4-S5 | `multiagent/observer/` | 事件桥接 |
| D4-S6 | `multiagent/tool/` | Agent 工具注册 |
| D4-S7 | `multiagent/builtin/` | 内建 Agent |
| D4-S8 | `multiagent/observability/` | Agent 指标 |
| D4-S9 | `multiagent/sessionview/` | COW 会话视图 |
| **D4-S10** | **`multiagent/delegate/`** | Hub-Spoke、`FlowBridge`、Worker |

Fork/Join 逻辑主要在 `agent/forkjoin*.go`（映射 D4-S3）。

---

## 6. ORCH Orchestration

| S ID | 包 | 职责 |
|------|-----|------|
| ORCH-S1 | `orchestration/workplan/` | Task + Flow 读模型 |
| ORCH-S2 | `orchestration/flow/` | `Hub`, FlowEvent 发布 |
| ORCH-S2 | `orchestration/imsink/` | Flow → D1 Gateway 卡片 |
| ORCH-S3 | `orchestration/wave/` | DAG 调度、ConflictGuard |

**Bootstrap：** `internal/bootstrap/execution_flow.go`, `delegate.go`

---

## 7. D5 / D6

| 域 | 包 | 说明 |
|----|-----|------|
| D5 | `observability/` | `Bridge`, `telemetry/names.go`（Operation 常量）, `coverage/` |
| D6-S3 | `evolution/eval/` | EvalRun、探针（含 `tool_accuracy`） |
| D6-S4 | `evolution/orchestration/` | 运行时判官 / 干预 |
| — | `internal/lint/layer/` | DSAFT import 规则 lint |

---

## 8. 测试落点

| T 域 | 目录 |
|------|------|
| D2-S10 QueryLoop | `contextengine/query/*_test.go`, `tests/integration/query_loop_*` |
| D2-S8 Sandbox | `toolrunner/sandbox_test.go`, `tests/security/shell_injection_test.go` |
| D2-S12 Worktree | `contextengine/worktree/manager_test.go` |
| D4-S10 Delegate | `multiagent/delegate/*_test.go` |
| ORCH | `orchestration/flow/hub_test.go` |
| 跨域集成 | `tests/integration/`, `tests/acceptance/` |

Build tags 见 `openspec/specs/testing-framework/spec.md`。
