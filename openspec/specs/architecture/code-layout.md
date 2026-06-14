# Devrix 代码路径布局规范（Domain / Scenario）

**Capability:** architecture-code-layout  
**Status:** Active  
**Version:** 1.3.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## 1. 目标

代码目录 **必须先能回答「属于哪个领域、哪个场景」**，再体现技术分层。  
路径是 DSAFT 资产（D/S）在仓库中的 **物理锚点**；`openspec/specs/{domain}/` 是同一资产的 **规格锚点**。

```
L1 领域 (D)  →  internal/layers/{domain-slug}/
L2 场景 (S)  →  …/{scenario-slug}/
L4 功能点 (F) →  …/{scenario-slug}/*.go  （或 activity 子目录）
```

> **Go 包名与目录名** 遵循 `openspec/specs/project/coding.md` §2.2：**全小写、无下划线、单数，`package` 与目录叶子名严格一致**。禁止 `d1`、`s13` 等 DSAFT 编号进入包名（见 `layering.md` §命名规约）。

---

## 2. 命名规则

| 层级 | 路径段 | 规则 | 示例 |
|------|--------|------|------|
| L1 领域 | `{domain-slug}/` | 全小写复合词，无下划线；与 `layering.md` D 表一致 | `communication`, `contextengine`, `orchestration` |
| L2 场景 | `{scenario-slug}/` | **Go 合法目录名**（同 §2.2）；在本文 §4 登记，**禁止自造** | `capture`, `thinking`, `taskprogress` |
| L3 活动 | `{activity-slug}/` | **可选**；仅当单 S 下 A 组 >1 且需物理隔离时使用；同样遵守 Go 目录名规则 | `dispatch`, `encode/feishu` |
| 域内核 | `kernel/` | 非 S 的共享模型（Card、Session 值对象等） | `communication/kernel/` |
| 跨域契约 | — | `internal/shared/contracts/` | 禁止域内 duplicated 契约 |

**禁止作为 L2 场景目录名：**

- 技术角色词：`gateway`, `adapters`, `eventbus`, `handler`, `service`（可作 **文件名** 或 L3 activity）
- DSAFT 编号：`d1`, `s13`, `d7-s2`
- 动词堆叠无场景语义：`utils`, `common`, `internal`
- **下划线**：Go 包名不允许，`scenario-slug` 目录亦不允许（如 ~~`present_thinking`~~）

---

## 3. 目录决策树（新文件放哪）

```text
1. 是否跨域共享契约/类型？
   YES → internal/shared/{contracts,types,config}/
2. 属于哪个 L1 领域 D？
   → internal/layers/{domain-slug}/
3. 属于哪个 L2 场景 S？（查 §4 注册表）
   → …/{scenario-slug}/
4. 是否域内核（无 S 归属）？
   → …/kernel/
5. 是否仅编排/启动接线？
   → internal/bootstrap/ 或 cmd/
6. 文件名体现 L4 F 语义（camelCase 导出 / snake 文件）
```

**测试文件** 与实现同目录：`{scenario-slug}/*_test.go`；E2E 在 `tests/acceptance/` 按域标签组织。

---

## 4. Scenario 路径注册表

### 4.1 D1 Communication（canonical S13–S18）

| S ID | Scenario | scenario-slug | 目标路径 | 职责摘要 |
|------|----------|---------------|----------|----------|
| D1-S13 | CaptureUserIntent | `capture` | `communication/capture/` | 入站、Persist、Dispatch、Permission、Command |
| D1-S14 | PresentThinking | `thinking` | `communication/thinking/` | Thinking 信号映射与 emit |
| D1-S15 | PresentTaskProgress | `taskprogress` | `communication/taskprogress/` | Task/Tool/Worker/Milestone **展示** |
| D1-S16 | DeliverConclusion | `conclusion` | `communication/conclusion/` | Conclusion 流式/终态/摘要 |
| D1-S17 | ConnectChannel | `channel` | `communication/channel/` | IM 适配、连接、实例、限流、Encode |
| D1-S18 | GuaranteeDelivery | `delivery` | `communication/delivery/` | EventBus、Critical、Drain |
| — | Domain Kernel | `kernel` | `communication/kernel/` | Card、平台无关消息模型 |

**横切（暂存，随迁移收敛）：**

| 组件 | 当前路径 | 收敛目标 |
|------|----------|----------|
| Turn tracker / 信号锚点 | `communication/capture/signal/` | ✅ 已收敛 |
| 契约映射 | `shared/contracts/im_outbound_signal.go` | 保持 shared |

### 4.2 D7 Orchestration

> D7 待迁移目录须遵守 §2.2（无下划线）。DSAFT 场景名与 slug 映射示例：`Wave Scheduler` → `wavescheduler/`。

| S ID | Scenario | scenario-slug | 目标路径 | 当前路径（迁移中） |
|------|----------|---------------|----------|-------------------|
| D7-S1 | Work Model | `workmodel` | `orchestration/workmodel/` | ✅ DM-012 |
| D7-S2 | Session Orchestrator | `sessionorchestrator` | `orchestration/sessionorchestrator/` | `orchestration/coordinator/` |
| D7-S3 | Wave Scheduler | `wavescheduler` | `orchestration/wavescheduler/` | `orchestration/wave/` |
| D7-S4 | Execution Flow | `executionflow` | `orchestration/executionflow/` | `flow/`, `workplan/`, `imsink/` |
| D7-S5 | Decision & Planning | `decisionplanning` | `orchestration/decisionplanning/` | `coordinator/classifier*` |
| — | Worker tool policy F | `toolpolicy` | `orchestration/toolpolicy/` | ✅ DM-015 |
| — | Delegate routing F | `delegatetools` | `orchestration/delegatetools/` | ✅ DM-011 |
| — | Session command queue F | `sessionqueue` | `orchestration/sessionqueue/` | ✅ DM-013 |
| — | Milestone DAG | `milestone` | `orchestration/milestone/` | ✅ 已迁入 |

### 4.3 D2 Context Engine

> **Canonical S15–S20**（DM-20260614-009）。Legacy module 路径仍有效；v2.0 按 scenario-slug 收敛。

| S ID | Scenario | scenario-slug | 当前路径 | v2.0 目标 |
|------|----------|---------------|----------|-----------|
| D2-S15 | PrepareExecutionContext | `prepare` | `prepare/memory/` `prepare/compression/` `prepare/prompt/` `prepare/conversation/` | ✅ DM-014 |
| D2-S16 | RunQueryLoop | `query` | `contextengine/query/` | 保持（loop 瘦身） |
| D2-S17 | PersistSessionState | `persist` | `persist/snapshot/`, `persist/transcript/` | ✅ DM-014 |
| D2-S18 | EnforceExecutionPolicy | `policy` | `policy/permission/`, `policy/toolrunner/` | ✅ DM-014 |
| D2-S19 | NestedExecution | `nested` | `nested/subquery.go`, `nested/background.go`, `nested/fork.go` | ✅ DM-014 |
| D2-S20 | LegacyHarnessFallback | `legacyharness` | `harness/` | 保持或 `legacy/` |

**Legacy module 路径（冻结追溯）：**

| Legacy S | scenario-slug | 路径 |
|----------|---------------|------|
| D2-S2 | `compression` | ~~`contextengine/compression/`~~ → `contextengine/prepare/compression/` | ✅ DM-014 |
| D2-S3 | `memory` | ~~`contextengine/memory/`~~ → `contextengine/prepare/memory/` | ✅ DM-014 |
| D2-S11 | `queue` | ~~`contextengine/queue/`~~ → `orchestration/sessionqueue/` (D7-S4) | ✅ DM-013 |
| D2-S12 | `worktree` | `contextengine/worktree/` |

**跨域漂移（v2.0 迁出 D2）：**

| 组件 | 当前路径 | 目标 | 状态 |
|------|----------|------|------|
| ~~delegate_tools~~ | ~~`contextengine/delegate_tools.go`~~ | `orchestration/delegatetools/` | ✅ DM-011 |
| TaskManager | ~~`contextengine/tasks/`~~ | `orchestration/workmodel/` (D7-S1) | ✅ DM-012 |
| queue delegate-progress | ~~`contextengine/queue/`~~ | D7-S4 `sessionqueue/` | ✅ DM-013 |

---

## 5. 目标目录树（D1 终态示例）

```text
internal/layers/communication/
├── kernel/                      # Card、Builder（原 core/）
├── capture/                     # S13
│   ├── gateway.go               # CommunicationGateway 入口
│   ├── session_store.go
│   ├── dispatch.go
│   ├── permission.go
│   └── signal/                  # turn tracker
├── thinking/                    # S14
├── taskprogress/                # S15
├── conclusion/                  # S16
├── channel/                     # S17
│   ├── adapters/
│   ├── connection/
│   ├── instance/
│   ├── ratelimit/
│   └── renderers/
└── delivery/                    # S18
    └── eventbus/
```

---

## 6. 当前 → 目标迁移（D1）

| 当前路径 | 目标 scenario-slug | 迁移状态 | 关联 Change |
|----------|-------------------|----------|-------------|
| `gateway/` | `capture/` + 部分 `conclusion/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `present/` | `thinking/` `taskprogress/` `conclusion/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `signal/` | `capture/signal/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `adapters/` … `renderers/` | `channel/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `eventbus/` | `delivery/eventbus/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `core/` | `kernel/` | ✅ IMPLEMENTED | DM-20260614-006 |
| `communication/milestone/` | — | ✅ 已迁至 `orchestration/milestone/` | DM-20260614-006 |

**迁移原则：**

1. **新代码** 必须写入 §4 登记的 **目标 scenario-slug**；禁止在新 `gateway/` 下追加 F。
2. **旧路径** 允许 `re-export` / type alias 一个发布周期，PR 标注 `BREAKING` 与迁移表。
3. 每个迁移 PR 更新：`code-layout.md` 迁移状态、`layering.md` Package Map、`d1-communication/spec.md`。
4. 单 PR 仅迁 **一个 scenario-slug**（或一组强耦合 F），并跑关联 L5/T。

---

## 7. 与 OpenSpec 的对应

| 代码 | 规格 |
|------|------|
| `internal/layers/{domain}/` | `openspec/specs/{domain}-*/spec.md` |
| scenario-slug | `layering.md` S 表 + 域 `a-registry.md` / `f-registry.md` |
| `*_test.go` 内 `// T:` | `openspec/specs/{domain}/t-registry.md` |
| span 名 | `{domain}/span-registry.md` |

验收/归档时 **代码路径与 spec 必须同步更新**（见 `specs/05-delivery-process.md` §6.3、§8.2）。

---

## 8. 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0.0 | 2026-06-14 | 初版：D1/D7 scenario-slug 注册表 + D1 迁移矩阵 |
| 1.1.0 | 2026-06-14 | D1 物理路径迁移完成（capture/channel/delivery/kernel） |
| 1.3.0 | 2026-06-14 | D2 S15–S20 Canonical；delegate_tools → delegatetools (DM-011) |
