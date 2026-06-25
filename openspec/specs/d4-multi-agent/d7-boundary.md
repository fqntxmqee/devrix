# D4 ↔ D7 跨域边界规范

**Capability:** d4-d7-boundary
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-14
**Change ID:** devrix-d4-sa-refine
**Demand ID:** DM-20260614-018
**Parent (D4):** `openspec/specs/d4-multi-agent/d4-domain.md`
**Parent (D7):** `openspec/specs/d7-orchestration/d7-domain.md`
**Related (D2):** `openspec/specs/d2-context-engine/d7-boundary.md`

---

## 1. 关系摘要

| 域 | 角色 | 一句话 |
|----|------|--------|
| **D7** | Orchestration Leader + **Hub-Spoke SoT** | 决定派哪个 Spoke、发布 FlowEvent、聚合 WorkPlan |
| **D4** | Agent Execution Follower | 在 WorkerSpec 下 fork→run→join，不 Publish |

**ingress：** D1 → D7 `ProcessMessage` only。D4 **不被** D1 直接调用。

---

## 2. Hub-Spoke 归属（R1 D7-1）

| 组件 | v1.0 代码位置 | Canonical 归属 | v2.0 目标 |
|------|--------------|---------------|----------|
| delegate_* 工具 | D7 `delegatetools/` | D7-S2 | 保持 |
| Spoke 派发 / fallback | D4 `delegate/service.go` | **D7-S2** | `hubspoke/dispatch.go` |
| Agent FlowBridge | D4 `delegate/bridge.go` | **D7-S4** | `hubspoke/agent_bridge.go` |
| SubQuery Flow 发布 | D2 `nested/flow_report.go` | **D7-S4** | `hubspoke/subquery_bridge.go` |
| WorkPlan | D7 `workplan/` | D7-S4 | 保持 |
| sessionqueue drain | D7 `executionflow/` (formerly `sessionqueue/`) | D7-S4 | DM-20260625-018 PR-3b |

**唯一 `hub.Publish` 出口：** D7 `SpokeBridge`（v2.0 目标态）。

---

## 3. 调用链 SoT

```text
D1.Gateway.RouteInbound
    └── D7.IOrchestrationEntry.ProcessMessage
            ├── D7-S2 delegatetools / DispatchWorker
            │       ├── Spoke=D4Worker → D4.WorkerExecutor.Execute
            │       └── Spoke=D2SubQuery → D2.NestedExecutor.Run
            ├── D7-S4 SpokeBridge.Publish(FlowEvent)
            │       └── WorkPlan + executionflow + imsink → D1
            └── [optional] D4-S12 RunAgentLoop（Leader Agent 本体）
```

**代码锚点（v1.0）：** `internal/layers/orchestration/delegatetools/delegate_tools.go` → `multiagent/delegate/service.go`

---

## 4. 职责矩阵

| 能力 | D7 | D4 | D2 | D1 | D5 |
|------|----|----|----|----|-----|
| delegate_* 路由 | ✅ S2 | ❌ | ❌ | — | — |
| Spoke 选择 / fallback | ✅ S2 | ❌ | ❌ | — | — |
| Worker fork/run/join | 派发 | ✅ S14 | ❌ | — | — |
| Agent 生命周期 | 可选 Leader | ✅ S12 | ❌ | — | — |
| PermissionGate 机制 | — | ✅ S12 | ❌ | ✅ UI | — |
| SessionView COW | — | ✅ S13 | ❌ | — | — |
| FlowEvent 发布 | ✅ S4 | 🔶 v1.0 临时 bridge | 🔶 v1.0 flow_report | 展示 | 观测 |
| SubQuery 嵌套执行 | 派发 | ❌ | ✅ S19 | — | — |
| WorkPlan 读模型 | ✅ S4 | ❌ | ❌ | 展示 | — |
| Agent metric 定义 | — | 🔶 emit | — | — | ✅ SoT |

图例：✅ SoT · 🔶 代码暂存、规格已迁出 · ❌ Out of Scope

---

## 5. 契约接口

| 接口 | 定义位置 | 实现 | 消费 |
|------|----------|------|------|
| `WorkerExecutor` | `shared/contracts`（v2.0） | D4 `execute/worker.go` | D7 Dispatcher |
| `NestedExecutor` | `shared/contracts`（v2.0） | D2 `nested/subquery.go` | D7 Dispatcher |
| `SpokeDispatcher` | D7 `hubspoke` | D7 bootstrap | delegatetools |
| `ExecutionFlowHub` | `shared/contracts` | D7 `flow` | D7 内部 |
| `AgentObserver` | D4 `contracts` | D7 Bridge（v2.0） | 进度映射 |

### 5.1 依赖规则

```text
✅ D7 → D4（WorkerExecutor）
✅ D7 → D2（NestedExecutor）
✅ D4 → D2（IEngine.Process）
✅ D4 → shared/contracts, D1 PermissionManager
❌ D4 → orchestration/flow（v2.0-b 后禁止）
❌ D2 nested → hub.Publish 直连（v2.0-c 后禁止）
```

---

## 6. Canonical S 对照

| D7 Canonical S | 与 D4 关系 | D4 Canonical |
|----------------|-----------|--------------|
| D7-S2 Session Orchestrator | 派发 Worker | S14 ExecuteWorker |
| D7-S2 DispatchWorker | 选 Spoke | S14 或 D2-S19 |
| D7-S3 Wave Scheduler | 可调度 D4 Worker | S14 per worker |
| D7-S4 Execution Flow | 聚合 D4 AgentEvent | S12 产出事件经 D7 Bridge |
| D7-S5 Decision & Planning | 不下发 Hub-Spoke 实现 | Out of Scope |

| D4 Canonical S | 与 D7 关系 |
|----------------|-----------|
| S11 ProvisionAgent | bootstrap / Factory |
| S12 RunAgentLoop | Leader Agent 或 Worker 内循环 |
| S13 IsolateAndMerge | Worker 隔离；D7 不替代 |
| S14 ExecuteWorker | **D7 主要调用点** |
| S15 InvokeExternalAgent | 独立 Tool 面，不经 Hub-Spoke |
| S16 ConfigureAgents | D7 读 `multi_agent.delegate` 配置 |

---

## 7. T 层跨域重归属（规格层）

| Legacy T ID | Canonical 归属 | 域 |
|-------------|---------------|-----|
| D4-S10-A01-T01~T06, T12 | D4-S14-A01-T01~T07 | D4 执行面 |
| D4-S10-A01-T07 | D7-S2-A04-T01 | fallback 路由 |
| D4-S10-A02-T08~T11 | D7-S4-A04-T01~T04 | Flow / IM |
| D4-S8-A01-T01~T02 | D5-AGENT-METRIC-T01~T02 | D5 |

> v1.0：**不修改**测试文件 `// T:` 注释。

---

## 8. 跨域迁移表（v2.0，并入 DM-20260614-018）

| # | 路径 | 行为 | 目标 | Slice |
|---|------|------|------|-------|
| 1 | `multiagent/delegate/bridge.go` | Agent→FlowEvent | D7 `hubspoke/agent_bridge.go` | b |
| 2 | `delegate/service.go` Dispatch 逻辑 | Spoke 选择 | D7 `hubspoke/dispatch.go` | b |
| 3 | `delegate/service.go` 执行逻辑 | fork/run/join | D4 `execute/worker.go` | b |
| 4 | `nested/flow_report.go` | SubQuery Publish | D7 `hubspoke/subquery_bridge.go` | c |
| 5 | `SubQueryParams.FlowHub` | 直连 Hub | 删除；D7 包装 | c |

---

## 9. 影子编排风险（双边共识 G-03）

> **风险声明：** 即使 Hub-Spoke 代码和规格归 D7，D4 仍可通过间接方式影响编排。以下路径不需"违反契约"——它们在契约范围内，但累积效应可能侵蚀 D7 的 Leader 地位。

| 风险路径 | 影子编排方式 | 检测方式 | 缓解 |
|---------|------------|---------|------|
| **Prompt 注入** | D4-S11 ProvisionAgent 在 Worker prompt 中嵌入 Spoke 偏好或路由暗示 | D5 Worker prompt 审计 span（v1.1） | Worker prompt 模板必须在 D7 可审查的 registry 中登记 |
| **Builtin 选择性注册** | D4 通过选择性注册/隐藏 Builtin Agent 影响 D7 的 Spoke 可用选项 | D7 可 override Builtin 注册（显式 Power） | D7-S2 DispatchWorker 维护自己的 Spoke 注册表，不盲从 D4 的 Builtin 列表 |
| **错误吞掉** | D4-S14 ExecuteWorker 吞掉 D7 期望的某些错误码，使 D7 fallback 机制失效 | D4-S14 sad path T 覆盖所有错误类型的透传 | 错误类型白名单：只有显式标记为 "D4-internal" 的错误可吞 |
| **配置漂移** | D4-S16 ConfigureAgents 的默认值随时间偏离 D7 编排假设 | D5 配置一致性 metric | D7 bootstrap 时校验 `multi_agent` 配置与 `orchestration` 期望的对齐 |
| **Worker 生命周期劫持** | D4-S12 RunAgentLoop 的 PermissionGate 超时/拒绝策略影响 D7 的 Spoke 完成时间预期 | D7-S2 DispatchWorker 超时独立于 D4 内部超时 | D7 设置 Worker 级 deadline，不依赖 D4 内部超时 |

**设计原则：** 影子编排不是"恶意行为"——它是复杂系统中不可避免的间接影响。目标是**可观测**（D5 span 登记），而非**杜绝**（不可能）。当影子编排从"意外副作用"变为"开发者知道的捷径"时，它就成了真正的架构退化。

---

## 10. Follower 对称性声明（双边共识 G-02）

> D2 和 D4 作为 Stackelberg Follower，享有对等的角色约束。该声明写入 `cross-domain-boundaries.md` §3，本文档交叉引用。

| 对称轴 | D2 Context Follower | D4 Execution Follower |
|--------|-------------------|---------------------|
| 不拥有编排决策权 | 不选 LLM 路径 | 不选 Spoke 路径 |
| 不直接 Publish FlowEvent | 经 D7 持久化后 emit | 经 D7 SpokeBridge |
| 保留域内执行比较优势 | Prepare/Tool/Persist | Provision/Run/Isolate/Execute |
| 硬约束 | import lint D2→D3 | import lint D4→orchestration/flow |
| Follower Veto | Tool Permission Gate (D2-S18) | PermissionGate (D4-S12) |

---

## 11. Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-14 | 初版：Hub-Spoke 全归 D7 + D4 Follower 契约 + v2.0 迁移表 |
| 1.1.0 | 2026-06-15 | 双边共识落盘：影子编排风险表（§9）+ Follower 对称性声明（§10）+ 反僭越契约交叉引用 |
