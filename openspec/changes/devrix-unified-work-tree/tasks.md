# Tasks: Unified Work Tree

**Demand ID:** DM-20260617-009  
**Status:** S3_Gate_Passed

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
- [ ] **AC23 (P0):** CI static analysis 检测 D2 直写 `sc.Todos` + 新增 `*Registry / *Manager` 类（v1.0 立即上线）
- [ ] **AC37 (P0):** WorkItem 状态机强制合法转换；terminal→back to in_progress 拒绝
- [ ] **AC38 (P1):** v1→v2 迁移边界——损坏/空文件/版本冲突处理
- [ ] **AC39 (P1):** WorkTree.Remove 级联删除子孙节点
- [ ] **AC40 (P0):** 循环依赖检测——AddDependency 成环拒绝；GetReadyItems 全量检测
- [ ] **AC53 (P1):** 磁盘写入原子性——临时文件 + 原子重命名

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
- [ ] **AC46 (P1):** WaveScheduler 读 WorkTree——subtree filter + dep satisfaction + TaskNode 投影往返一致
- [ ] **AC47 (P1):** PlanMode approve checklist→implement promote（ephemeral→persistent + 继承依赖）

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
- [ ] **AC41 (P1):** Terminal callback 失败重试——指数退避 3 次，均失败 → error log + notified=failed
- [ ] **AC42 (P1):** 并发 terminal 通知幂等——多个子任务同时 terminal 时 parent 只 re-resolve 一次

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
- [ ] **AC48 (P1):** Alias 行为等价性——task_write(mode=checklist) === todo_write；task_spawn(kind=explore) === delegate_explore

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
| T5.7 | Uncertainty Anchor 机制（historical + structural + evidence） | `workmodel/uncertainty.go` | ~100 |
| T5.8 | 递归深度/宽度约束（max_depth=3, max_children=7, daily_limit=5） | `workmodel/work_tree.go` | ~60 |

**Quality Gate:**
- [ ] AC17–AC19 满足
- [ ] **AC20 (P2):** 单 WorkItem 递归 decompose 深度 ≤ 3（可配置 `work_tree.max_decompose_depth`）
- [ ] **AC21 (P2):** 深度超限 fallback inline execute（保留 LLM 对 leaf task 的直接责任）
- [ ] **AC22 (P2):** 同 Session 24h 内同 Kind decompose > 5 → 触发 `task_await` 人工 review
- [ ] **AC27 (P0):** Uncertainty Anchor 集成测试——LLM 空 evidence 时 uncertainty 回退到 historical+structural
- [ ] **AC45 (P1):** ResolveFocus dispatch 路由——8 Kind 各路由正确；goal spawn 返回 ErrInvalidDispatch
- [ ] 集成：uncertainty 高 → 自动子项 → spawn → terminal → 父继续

**建议 PR:** `feat/runturn-recursive-resolve`

---

## Phase 1.5 — 最小递归引擎 (v1.5) ⭐ 新增

> **设计理由:** v2.0 完整递归改动量太大（~500+ 行，5 个组件）。v1.5 用最小改动让 WorkTree "活起来"——单层递归，验证 decompose→spawn→await→continue 循环。详见 `version-roadmap.md`。

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T1.5.1 | `RunTurn.resolve(focus)` hook — 单层 focus → decompose? → spawn → await children | `turn/orchestrator.go` | ~80 |
| T1.5.2 | 基础 uncertainty（LLM claim only, 无 anchor） | `workmodel/work_tree.go` | ~30 |
| T1.5.3 | 单层 decompose（depth=1 硬编码，不递归） | `coordinator/decomposer.go` | ~50 |
| T1.5.4 | `task_await` 基础实现（基于 RunRegistry terminal callback） | `enforce/background_task_tools.go` | ~60 |
| T1.5.5 | 集成测试：单 focus → 2-3 子任务 → spawn → wait → parent continue | `turn/orchestrator_test.go` | ~100 |

**Quality Gate:**
- [ ] **AC28 (P1):** RunTurn 单层递归——LLM decompose 创建子项 → spawn → await → parent 在 children terminal 后继续
- [ ] **AC29 (P1):** 子任务 terminal 后 parent WorkItem uncertainty 自动重评估
- [ ] **AC43 (P0):** 子任务部分失败——success+failed 混合 → 父节点标记 failed + 失败摘要
- [ ] **AC44 (P1):** GetFocus 确定性 tiebreak——同 kind+同 uncertainty → CreatedAt 升序 → ID 字典序
- [ ] 集成测试通过：手动/自动创建 focus → LLM decompose → 子任务执行 → parent 自动继续

**软依赖:** DM-011 Phase 1 (Register + SetTerminal)。如果 DM-011 未就绪，降级使用 Legacy BackgroundRegistry。

**建议 PR:** `feat/minimal-recursion`

---

## Phase 7 — 跨会话 (v2.1)

> **前提:** DM-011 完整交付。WorkTree 生命周期从 per-session 扩展到跨 session。

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T7.1 | `QueryWorkPlan` 跨 session 只读查询 | `coordinator/workmodel.go` | ~60 |
| T7.2 | 历史 Session WorkItem lock 协议（terminal → immutable） | `workmodel/work_tree.go` | ~40 |
| T7.3 | 跨 Session mutable 引用：propose-modify → 新 Session → arbitration | `coordinator/orchestrator.go` | ~120 |
| T7.4 | 飞书卡片：历史 WorkItem 摘要 + "继续？" 入口 | D1 communication 层 | ~60 |
| T7.5 | 集成测试：新 Session 引用历史 WorkItem | `coordinator/workmodel_test.go` | ~80 |

**Quality Gate:**
- [ ] **AC30 (P2):** 用户在新 Session 问"昨天那个 task 完成了吗"→ 返回历史 WorkItem 状态
- [ ] **AC31 (P2):** 历史 Session WorkItem 默认不可修改（lock 协议生效）
- [ ] **AC32 (P2):** "继续昨天的第三个任务"→ 创建新 Session，继承 WorkItem 上下文
- [ ] **AC49 (P0):** Lock 运行时 enforcement——修改历史 WorkItem 返回 ErrWorkItemLocked
- [ ] **AC50 (P1):** 链式继承追溯——A→B→C 后 SourceSession 指向 A

**建议 PR:** `feat/cross-session-worktree`

---

## Phase 8 — 自演化 (v3.0)

> **前提:** v2.1 完整交付 + 至少 10 个 Session 的历史 WorkTree 数据积累。

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T8.1 | 自适应 Uncertainty 阈值（基于用户/项目历史） | `workmodel/uncertainty.go` | ~80 |
| T8.2 | WorkItem Kind 自动检测 + 提议协议（LLM → S3-Gate） | `workmodel/workitem.go` + `coordinator/decomposer.go` | ~60 |
| T8.3 | 跨项目任务模板提取（高频子树 → 可复用模板） | `workmodel/template.go` | ~120 |
| T8.4 | WorkTree 结构自优化（基于 terminal 率反馈） | `workmodel/optimizer.go` | ~100 |
| T8.5 | 集成测试：10 Session 后同场景 terminal 率提升 | `workmodel/optimizer_test.go` | ~100 |

**Quality Gate:**
- [ ] **AC33 (P3):** 同场景 10 Session 后 auto-decompose 的子任务 terminal 率 ≥ 手动 decompose
- [ ] **AC34 (P3):** LLM 提议新 Kind 时附带 evidence + historical precedent
- [ ] **AC35 (P3):** 新项目启动时系统建议任务模板（基于相似项目）
- [ ] **AC36 (P3):** Uncertainty 阈值在 10 Session 后收敛到用户最优值（不再需要手动调整）
- [ ] **AC51 (P1):** 冷启动降级——数据不足时使用全局默认值，不报错
- [ ] **AC52 (P1):** 阈值振荡防止——hysteresis（差异 > 0.1 且连续 3 Session 同方向才更新）

**建议 PR:** `feat/self-evolving-tasks`

---

## 依赖顺序（更新）

```text
Phase 0 (T0.*) — v1.0
    ↓
Phase 1 (T1.*) ──→ Phase 2 (T2.*) — v1.1
    ↓                    ↓
    └──────→ Phase 3 (T3.*) ←── DM-011 Phase 1 — v1.2
                    ↓
              Phase 1.5 (T1.5.*) ←── DM-011 Phase 1 (软依赖) — v1.5 ⭐
                    ↓
              Phase 4 (T4.*) ──→ Phase 5 (T5.*) ←── DM-011 Phase 2+ — v2.0
                                        ↓
                                  Phase 7 (T7.*) ←── DM-011 完整 — v2.1
                                        ↓
                                  Phase 8 (T8.*) ←── 10+ Session 数据 — v3.0
```

## 与 DM-011 并行策略（更新）

| 本 change Phase | DM-011 Phase | 联调点 | 备注 |
|-----------------|--------------|--------|------|
| 0–2 | T1–T3（可并行）| 无硬依赖 | v1.0–v1.1 独立交付 |
| 3 | T4–T6 | RunRef 挂接 | 硬依赖 Phase 1 |
| 1.5 | Phase 1 | Register + SetTerminal | 软依赖（可降级 Legacy） |
| 4 | T7–T8 | task_await 读 delta | 硬依赖 Phase 2+ |
| 7 | 完整交付 | 跨 session RunRef 查询 | 硬依赖 |
| 8 | — | 历史数据积累 | 依赖 10+ Session 数据 |

---

## Phase 6 — 持续防御 (v1.1+, 持续)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T6.1 | Code Owner Bot — 新增 task-related 实体自动 @ D7 架构师 | CI config / `.github/CODEOWNERS` | ~30 |
| T6.2 | 季度 Property Rights Audit 脚本 — 扫描游离 WorkTree 外的 task 实体 | `scripts/audit-property-rights.sh` | ~60 |

**Quality Gate:**
- [ ] **AC24 (P1):** Code Owner Bot 就绪，新增 task 实体时自动 @ D7 架构师
- [ ] **AC25 (P1):** Audit 脚本可运行，首次运行输出基准报告

---

## 验收标准汇总（AC1–AC53）

| AC | 内容 | Phase | 优先级 |
|----|------|-------|--------|
| AC1–AC5 | WorkItem 基础模型 + legacy 兼容 | Phase 0 | P0 |
| AC6–AC10 | 各写入路径挂树 | Phase 1–2 | P1 |
| AC11–AC13 | RunRegistry 挂接 | Phase 3 | P2 |
| AC14–AC19 | tool 简化 + 递归求解 | Phase 4–5 | P3 |
| **AC20** | 递归深度 ≤ 3 | Phase 5 | P2 |
| **AC21** | 深度超限 fallback inline | Phase 5 | P2 |
| **AC22** | 同 Kind 24h decompose 上限 | Phase 5 | P2 |
| **AC23** | CI static analysis 防产权侵蚀 | Phase 0 | P0 |
| **AC24** | Code Owner Bot | Phase 6 | P1 |
| **AC25** | 季度 Property Rights Audit | Phase 6 | P1 |
| **AC27** | Uncertainty Anchor 机制 | Phase 5 | P0 |
| **AC28** | RunTurn 单层递归（MVP） | Phase 1.5 | P1 |
| **AC29** | Child terminal → parent uncertainty 重评估 | Phase 1.5 | P1 |
| **AC30** | 跨 Session 只读查询历史 WorkItem | Phase 7 | P2 |
| **AC31** | 历史 WorkItem lock 协议 | Phase 7 | P2 |
| **AC32** | 跨 Session 继承 WorkItem 上下文 | Phase 7 | P2 |
| **AC33** | 自演化 terminal 率 ≥ 手动 | Phase 8 | P3 |
| **AC34** | LLM 提议新 Kind 附带 evidence | Phase 8 | P3 |
| **AC35** | 新项目启动建议任务模板 | Phase 8 | P3 |
| **AC36** | Uncertainty 阈值自适应收敛 | Phase 8 | P3 |
| **AC37** | WorkItem 状态机强制合法转换 | Phase 0 | P0 |
| **AC38** | v1→v2 迁移边界处理 | Phase 0 | P1 |
| **AC39** | WorkTree.Remove 级联删除 | Phase 0 | P1 |
| **AC40** | 循环依赖检测与报错 | Phase 0 | P0 |
| **AC41** | Terminal callback 失败重试 | Phase 3 | P1 |
| **AC42** | 并发 terminal 通知幂等 | Phase 3 | P1 |
| **AC43** | 子任务部分失败处理 | Phase 1.5 | P0 |
| **AC44** | GetFocus 确定性 tiebreak | Phase 1.5 | P1 |
| **AC45** | ResolveFocus dispatch 路由 | Phase 5 | P1 |
| **AC46** | WaveScheduler 读 WorkTree 正确性 | Phase 2 | P1 |
| **AC47** | Checklist promote ephemeral→persistent | Phase 2 | P1 |
| **AC48** | Alias 行为等价性 | Phase 4 | P1 |
| **AC49** | Lock 运行时 enforcement | Phase 7 | P0 |
| **AC50** | 链式继承追溯 | Phase 7 | P1 |
| **AC51** | 冷启动降级（数据不足） | Phase 8 | P1 |
| **AC52** | 阈值振荡防止（hysteresis） | Phase 8 | P1 |
| **AC53** | 磁盘写入原子性 | Phase 0 | P1 |

> **AC26 (Codex 提议, Claude 保留异议):** v1.1 empty RunRef → block spawn。替代方案：rate-limited warn + dashboard 计数器，v1.2 hard dependency。不在当前 AC 列表中。

---

## 完成清单

- [ ] Phase 0–6 + Phase 1.5 + Phase 7–8 全部 PR 合并
- [ ] demand.md AC1–AC53 逐条验收
- [ ] `version-roadmap.md` 作为演进路线 SoT
- [ ] `gaming-analysis.md` v2 修正完成
- [ ] `gaming-analysis-bilateral-consensus.md` 标为 FINAL
- [ ] T 层登记 `specs/d7-orchestration/t-registry.md`
- [ ] `task-planning-design.md` 更新 WorkItem 终态
- [ ] v1.5 最小递归用户验收
- [ ] v3.0 自演化数据基线建立（10+ Session）
- [ ] Ready for `/openspec-archive devrix-unified-work-tree`
