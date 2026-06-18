# Tasks: Unified Work Tree

**Demand ID:** DM-20260617-009  
**Status:** S2_Clarified

---

## Phase 0 — WorkItem 基础模型 (v1.0)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T0.1 | 定义 `WorkItem`, `WorkKind`, `ExecPolicy`, `CreateWorkItemInput` | `workmodel/workitem.go` | ~120 |
| T0.2 | 实现 `WorkTree` CRUD + 树遍历 + GetReadyItems | `workmodel/work_tree.go` | ~280 |
| T0.3 | `DiskWorkItemStore` v2 + v1 Task 迁移 | `workmodel/workitem_store.go` | ~120 |
| T0.4 | `TaskManager` 委托 WorkTree + `Tree()` 暴露 | `workmodel/task_manager.go` | ~80 |
| T0.5 | 单元测试：树操作、迁移、legacy 兼容 | `workmodel/work_tree_test.go` | ~180 |

**Quality Gate:**
- [ ] `go test ./internal/layers/orchestration/workmodel/...` 全绿
- [ ] 现有 task_manager_test / disk_store_test 不退化
- [ ] AC1–AC5 满足

**建议 PR:** `feat/workitem-foundation`

---

## Phase 1 — 写入路径挂树 (v1.1)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T1.1 | Session 首消息 `EnsureGoal` | `coordinator/orchestrator.go` 或 ingress | ~40 |
| T1.2 | delegate_* spawn 写 WorkItem（parent + kind） | `delegatetools/delegate_tools.go` | ~80 |
| T1.3 | PlanMode approve 批量创建 implement 子项 | `workmodel/plan_mode.go` | ~60 |
| T1.4 | FlowHub linkTask 更新 WorkItem status | `flow/hub.go` | ~40 |
| T1.5 | flat `task_create` 默认挂 goal 下 | `workmodel/task_manager.go` | ~30 |
| T1.6 | `task_list` 可选 `format=tree` | `workmodel/tool_suite.go` | ~50 |

**Quality Gate:**
- [ ] delegate 集成测试：spawn 后 WorkTree 有 parent
- [ ] AC6–AC8 满足

**建议 PR:** `feat/worktree-write-paths`

---

## Phase 2 — todo_write + Wave 吸收 (v1.1)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T2.1 | `WorkTree.UpsertChecklist` | `workmodel/work_tree.go` | ~60 |
| T2.2 | todo_write 改底层 + sc.Todos 投影 | `toolrunner/todo_tool.go` | ~80 |
| T2.3 | Plan approve promote checklist→implement | `workmodel/plan_mode.go` | ~40 |
| T2.4 | Decomposer 写 WorkTree 子树 | `coordinator/decomposer.go` | ~100 |
| T2.5 | WaveScheduler 读 WorkTree ready 子项 | `wave/scheduler.go` | ~120 |
| T2.6 | TaskNode ← WorkItem 投影 adapter | `wave/types.go` 或 `wave/adapter.go` | ~60 |

**Quality Gate:**
- [ ] todo_write 后 WorkTree 有 checklist 子项
- [ ] delegate_wave 后 WorkTree 有 implement 子树
- [ ] AC9–AC10 满足

**建议 PR:** `feat/worktree-todo-wave`

---

## Phase 3 — RunRegistry 挂接 (v1.2，依赖 DM-011)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T3.1 | spawn 时写 `WorkItem.RunRef` | delegatetools + wave runners | ~60 |
| T3.2 | RunRegistry terminal → WorkItem.UpdateStatus | `runregistry/callback.go` | ~50 |
| T3.3 | terminal bubble notify 父 WorkItem | `workmodel/notify` + turn prepare | ~80 |
| T3.4 | QueryWorkPlan 树形 + RunRegistry background | `coordinator/workmodel.go` | ~60 |
| T3.5 | `GetByRunRef` 索引 | `workmodel/work_tree.go` | ~30 |

**Quality Gate:**
- [ ] DM-011 AC1–AC5 + 本 change AC11–AC13
- [ ] Wave worker output 经 RunRegistry 可读

**建议 PR:** `feat/worktree-runref`（需 DM-011 PR-1 合并）

---

## Phase 4 — Tool 面简化 (v2.0)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T4.1 | `task_write` runner（merge create/update/checklist） | `workmodel/register_tools.go` | ~120 |
| T4.2 | `task_spawn` runner（merge delegate_*） | `delegatetools/` 或新 package | ~100 |
| T4.3 | `task_await` runner（merge task_output） | `enforce/background_task_tools.go` | ~80 |
| T4.4 | `task_list` 增强（subtree/filter） | `workmodel/tool_suite.go` | ~60 |
| T4.5 | 旧 tool 名 alias + deprecation warn | bootstrap 注册 | ~40 |
| T4.6 | toolpolicy / surface 白名单更新 | `toolpolicy/filter_adapter.go` | ~30 |

**Quality Gate:**
- [ ] alias 期旧 prompt 仍可用
- [ ] AC14–AC16 满足

**建议 PR:** `feat/task-tool-simplify`

---

## Phase 5 — 递归求解引擎 (v2.0)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T5.1 | `GetFocus` + kind/uncertainty 优先级 | `workmodel/work_tree.go` | ~80 |
| T5.2 | Uncertainty threshold config | `shared/config/coordinator.go` | ~30 |
| T5.3 | Decomposer.Decompose(focus) → 子 WorkItem | `coordinator/decomposer.go` | ~100 |
| T5.4 | RunTurn resolve hook（focus → decompose/spawn） | `turn/orchestrator.go` | ~120 |
| T5.5 | 删 legacy TaskNode 持久化 / flat Task 直写 | 多文件 | ~100 |
| T5.6 | 登记 T 层 → `t-registry.md` | openspec | ~40 |

**Quality Gate:**
- [ ] AC17–AC19 满足
- [ ] 集成：uncertainty 高 → 自动子项 → spawn → terminal → 父继续

**建议 PR:** `feat/runturn-recursive-resolve`

---

## 依赖顺序

```text
Phase 0 (T0.*)
    ↓
Phase 1 (T1.*) ──→ Phase 2 (T2.*)
    ↓                    ↓
    └──────→ Phase 3 (T3.*) ←── DM-011 T1–T3
                    ↓
              Phase 4 (T4.*)
                    ↓
              Phase 5 (T5.*)
```

## 与 DM-011 并行策略

| 本 change Phase | DM-011 Phase | 联调点 |
|-----------------|--------------|--------|
| 0–2 | T1–T3（可并行） | 无硬依赖 |
| 3 | T4–T6 | RunRef 挂接 |
| 4 | T7–T8 | task_await 读 delta |
| 5 | — | wave_completed 附件 |

## 完成清单

- [ ] Phase 0–5 全部 PR 合并
- [ ] demand.md AC1–AC19 逐条验收
- [ ] T 层登记 `specs/d7-orchestration/t-registry.md`
- [ ] `task-planning-design.md` 更新 WorkItem 终态
- [ ] Ready for `/openspec-archive devrix-unified-work-tree`
