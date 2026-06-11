# Tasks: devrix-queryloop-context

**Demand ID:** DM-20260610-012  
**Design:** [design.md](./design.md)

---

## v1.0 — CC s01–s07 对齐

| # | 任务 | L4 | L5 | PR 估算 |
|---|------|----|----|---------|
| T1 | 新增 `conversation/` adapter（Message ↔ User/Assistant/Attachment） | conversation | L5-CTX-34 | ~200 |
| T2 | 实现 `query/loop.go` 主循环（compress→attach→prepend UC→LLM→tools→append） | query_loop | L5-CTX-34 | ~350 |
| T3 | 实现 `usercontext/` Provider + PrependForAPI | user_context | L5-CTX-35 | ~150 |
| T4 | Assembler 支持 `user_context.mode`；L3 剥离 AGENTS.md | user_context | L5-CTX-35 | ~120 |
| T5 | `attachments/registry` + plan_mode 5-phase 模板 | attachments | L5-CTX-36 | ~280 |
| T6 | `permission/mode` + enter/exit_plan_mode 工具 | permission_mode | L5-CTX-37 | ~250 |
| T7 | ToolPool 按 PermissionMode 过滤 | permission_mode | L5-CTX-37 | ~100 |
| T8 | TaskManager 磁盘 store + task_* 工具注册 | task_tools | L5-CTX-38 | ~200 |
| T9 | PEVEngine 委托 Loop + Verify Hook | query_loop | L5-CTX-34 | ~300 |
| T10 | engine.go Process 改造（压缩下沉、删入口 assemble） | query_loop | L5-CTX-34 | ~200 |
| T11 | `multiagent/builtin` Explore + Plan + SubQuery.Run | subquery | L5-CTX-40 | ~350 |
| T12 | `transcript/sidechain` 基础持久化 | sidechain_transcript | L5-CTX-42 | ~150 |
| T13 | devrix.yaml 配置 + 集成测试 L5-CTX-34~39 | — | L5-CTX-39 | ~200 |
| T14 | 删除 context_assembler.go + 文档更新 context-engine spec V6 | — | — | ~100 |

## v1.1 — s04/s08 深化

| # | 任务 | L4 |
|---|------|-----|
| T15 | forkSubagent buildForkedMessages 移植 | subquery |
| T16 | StreamingToolExecutor | query_loop |
| T17 | BackgroundTask + task-notification queue drain | background_tasks |
| T18 | L5-CTX-41 fork cache 测试 | subquery |

## v2.0 — Hub-Spoke + ExecutionFlow + WorkPlan

> 设计见 `design-d4-v2.md`、`design-orchestration-v3.md`（v3 草案）。

| # | 任务 | L4 | L5 |
|---|------|-----|-----|
| T19 | ExecutionFlowHub + WorkPlan 骨架 + SubQuery FlowTap | ORCH-S1/S2 | L5-ORCH-01, L5-4-10-06 |
| T20 | D1 worker_progress Gateway + WorkerProgressRenderer | D1-S8 | L5-4-10-05 |
| T21 | D4 DelegateService + delegate_* + Bridge→Hub | D4-S10 | L5-4-10-01~03 |
| T22 | Task 绑定（task_id/owner）+ 双通道联调 | ORCH + D2 tasks | L5-4-10-04,07,09 |
| T23 | Async + D4 降级 SubQuery + Worktree | D4-S10 + D2-S12 | L5-4-10-08, L5-4-12-01 |

## v3.0 — Devrix 超越层

| # | 任务 | 说明 |
|---|------|------|
| T24 | D7 Work Orchestration 升格 + Milestone↔Task | `design-orchestration-v3.md` |
| T25 | PEV Verify semantic 模式 | Hook 增强 |
| T26 | Feishu worker_card / 任务树卡片 | D1 adapter |
| T27 | L5 E2E Plan Mode + WorkPlan 场景 | 测试网 |

---

## 依赖顺序

```
T1 → T2 → T3,T4 → T5 → T6,T7 → T8
              ↘ T9,T10 → T11,T12 → T13 → T14
```

## 验收

- [x] v1.0 全部 P0 L5 绿
- [x] v2.0 Hub-Spoke + ExecutionFlow + Delegate P0 L5 绿
- [x] `harness.enabled=false` 回归绿
- [x] context-engine spec V6 delta 合并至 `openspec/specs/context-engine/spec.md`
- [x] orchestration / multi-agent spec 与 architecture 注册表同步
- [ ] v3.0 D7 Work Orchestration（T24–T27，见 `design-orchestration-v3.md`）
