# 跨层契约与依赖边界

**Registry 源码：** `internal/shared/contracts/registry.go`  
**Layer lint：** `internal/lint/layer/`  
**Last Updated:** 2026-06-13

---

## 1. 依赖方向（硬规则）

```
D1 Communication  ──►  D2 Context Engine  ──►  D3 LLM Gateway
        │                      │
        │                      ├──► D4 Multi-Agent（通过接口）
        │                      └──► D2 toolrunner（域内）
        │
        └──read── ORCH WorkPlan / imsink（读模型）

D5 Observability  ··· 观测注入（Bridge），不反向驱动业务
D6 Evolution      ··· 探针 / lint，不反向驱动业务
```

**禁止：**

- D3 import `contextengine/` 或 `communication/`
- D2 import D1 `adapters/` 实现
- ORCH import D1 adapter 实现
- D4 Worker import `delegate` 工具注册细节

Layer lint `--strict` 与 D6 **LayerViolationProbe** 在 CI 强制执行。

---

## 2. 契约注册表（DefaultCatalog）

| 契约 | 定义位置 | 主要消费方 |
|------|----------|------------|
| `IEngine` | `shared/contracts/engine.go` | D1 Gateway |
| `EngineEvent` | `shared/contracts/engine.go` | D1 ↔ D2 事件流 |
| `ITokenCounter` | `shared/contracts/tokencounter.go` | D2 压缩、D3 计数 |
| `IPermissionGate` | **`shared/contracts/permission.go`** | D2 QueryLoop、D4 Agent、D1 Gateway |
| `ILLMGateway` | **`llmgateway/contracts.go`** | D2（经 bridges/llm） |
| `ITierResolver` | **`llmgateway/contracts.go`** | D2 模型/Tier 解析 |
| `IToolRunner` | **`contextengine/toolrunner/contracts.go`** | D2 QueryLoop |
| `IToolRegistry` | **`contextengine/toolrunner/contracts.go`** | D2 registry/bootstrap |
| `ExecutionFlowHub` | `shared/contracts/execution_flow.go` | D2 SubQuery、D4 Delegate、ORCH |
| `FlowEvent` | `shared/contracts/execution_flow.go` | 进度双写 IM + Leader Queue |
| `WorkPlanSnapshot` | `shared/contracts/execution_flow.go` | ORCH、delegate_status |
| `ComputeCtxPct` | `shared/contracts/ctxutil.go` | D2 complete、D1 摘要卡片 |

新增跨层接口：**必须** `Register` 到 `DefaultCatalog()` 并补充 layer-lint 规则。

---

## 3. 为何 IPermissionGate 在 shared 而非 D1

D2 不能 import D1 `communication/`（会形成 D2→D1 违规）。Permission 契约放在 **`shared/contracts`**，D1 Gateway 与 D4 Agent 各自实现/适配，D2 仅依赖接口。

---

## 4. D2 域内接口（不跨层 export）

定义于 `contextengine/contracts.go`，**不**进入 Global registry：

| 接口 | 用途 |
|------|------|
| `IObserver` | 引擎 info/error 事件 |
| `ICompressionObserver` | 压缩步骤可观测 |
| `IToolRunner` / `IToolRegistry` | 类型别名 → `toolrunner` 包（向后兼容 re-export） |

---

## 5. Bridges 与 Bootstrap

| 文件 | 职责 |
|------|------|
| `bridges/llm/wire.go` | D3 Gateway → D2 `EngineDeps.LLM` |
| `bootstrap/context_engine_builder.go` | 组装 ContextEngine |
| `bootstrap/execution_flow.go` | ExecutionFlowHub 全局注册 |
| `bootstrap/delegate.go` | DelegateService + Worktree |

Bridges 是 **composition root**，允许跨域 import；业务包不应复制此模式。

---

## 6. EngineEvent 四握约定

D2 → D1 事件类型（字符串）：

| Type | 含义 |
|------|------|
| `text` | 流式 assistant 文本 |
| `thinking` | 推理/思考块 |
| `tool_call` | 工具调用展示 |
| `permission_required` | 等待用户授权 |
| `info` / `error` | 进度与错误 |
| `complete` | 本轮结束（含 usage/duration/ctx_pct metadata） |

Gateway 必须在收到 `complete` 后才持久化会话终态（engine 已保证 snapshot 先于 complete）。

---

## 7. 错误码（D2 现行）

定义：`internal/shared/errors/context.go`

| Code | 场景 |
|------|------|
| `CTX_EXCEEDED_4001` | Token 预算耗尽 |
| `CTX_SNAPSHOT_4002` | 快照损坏 |
| `CTX_LLM_4004` | LLM 不可用 / 超时 / 529 |
| `CTX_PERMISSION_4006` | 工具权限拒绝 |
| `CTX_AUTOCOMPACT_4010` | Autocompact 降级 |
| `CTX_MEMORY_4022` | LongTerm DB 错误 |

已移除（PEV 退役）：`CTX_PEV_*`、`CTX_VERIFY_CMD_*`、`CTX_PLAN_402*`

---

## 8. 验收与工具

```bash
# 单元 + security
./scripts/test-unit.sh

# 层 import 规则（若已安装 devrix-layer-lint）
go test ./internal/lint/layer/...

# 契约注册表自检
go test ./internal/shared/contracts/...
```

T 层映射：`openspec/t-registry.md` · CROSS-A02-T03（契约注册表）
