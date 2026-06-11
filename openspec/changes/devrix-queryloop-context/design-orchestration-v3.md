# Design Draft: Work Orchestration（D7 升格草案，v3.0）

**状态:** Draft — v2.0 不实施，仅指导 v3 领域拆分  
**前置:** v2.0 `ExecutionFlowHub` + `WorkPlan` 读模型（见 `design-d4-v2.md` §6.5–§6.7）

---

## 1. 升格动机

v2.0 在 `internal/layers/orchestration/` 引入 **WorkPlan 读模型**，已验证：

- Plan 产物（D2 plan_mode）
- Task 图（D2 TaskManager）
- Milestone 轨道（D1 milestone + D2 PEV）
- ExecutionFlow（SubQuery + D4 Worker）

若 v3 需要 **跨 Session 持久化**、**Milestone↔Task 双轨互操作**、**复杂编排规则**，应将写模型从 D1/D2 收拢为独立域。

---

## 2. 提议：D7 Work Orchestration Domain

| Domain ID | 名称 | 职责 |
|-----------|------|------|
| **D7** | Work Orchestration | **做什么、做到哪了** 的 SoT（规划 + 进度读模型） |

### D7 Scenarios（草案）

| Module ID | 场景 | 职责 | 现归属 |
|-----------|------|------|--------|
| D7-S1 | Plan | Plan 产物、plan 策略、enter/exit 规则 | D2 permission/plan_mode |
| D7-S2 | TaskGraph | 会话/跨会话任务分解、依赖、owner | D2 tasks |
| D7-S3 | Milestone | 宏观阶段 DAG、TaskFlow | D1 milestone（写模型迁出） |
| D7-S4 | ExecutionFlow | 子 Agent 运行时进度 SoT | v2 orchestration |

### 保留在其他域

| 域 | 保留 |
|----|------|
| D2 | QueryLoop、SubQuery、压缩、Tool 执行 **运行时** |
| D4 | Agent 生命周期、Fork/Join、Delegate **执行** |
| D1 | IM 呈现、Renderer、Gateway **出站**（只读 WorkPlan 投影） |

---

## 3. 依赖方向

```
D1 ──query──► D7 WorkPlan（读）
D2 ──events──► D7 ExecutionFlow / TaskGraph（写）
D4 ──events──► D7 ExecutionFlow（写）
D7 ──assign──► D4 Delegate（大脑决策后调用，D7 不替大脑决策）
```

**硬约束：** D7 **不**调用 LLM；**不**替代中控大脑编排权。

---

## 4. v3 交付物（占位）

| 任务 | 说明 |
|------|------|
| T23 | Milestone ↔ Task 图投影与互操作 |
| T27 | D7 包结构迁移 + layering 登记 |
| T28 | WorkPlan 跨 Session 持久化 |

---

## 5. 升格条件（满足 ≥2 再启动）

1. WorkPlan API 在 v2 稳定运行 ≥1 个 release  
2. Milestone 写逻辑从 D1 迁出成本可估且 < 400 行/PR  
3. 产品确认需要跨 Session 工作流或独立「任务看板」  
