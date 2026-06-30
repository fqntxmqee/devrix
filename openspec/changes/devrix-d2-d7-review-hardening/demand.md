---
demand-id: DM-20260630-013
title: D2+D7 代码审查硬化 — 安全 / 并发 / 压缩闭环 / 错误可观测
priority: P0
status: S2_Clarified
dsaft_domain: [context-engine, orchestration]
created: 2026-06-30
reporter: AI 架构审查（D7 orchestration ~33.6k LOC + D2 contextengine ~15.8k LOC）
related:
  - DM-20260630-012 (MUPS 交付收敛 — 战术硬编码 partial cleanup)
  - DM-20260630-011 (Session 结论完整性 — TurnState)
  - DM-20260621-010 (D7 错误聚合 — silent swallow 先例)
  - DM-20260620-001 (Context Budget — compression 域)
---

# Demand: D2+D7 代码审查硬化

## 1. 原始诉求

2026-06-30 对 devrix **D7 编排域**（`internal/layers/orchestration/`，210 非测试 Go 文件）与 **D2 上下文引擎域**（`internal/layers/contextengine/`，143 非测试 Go 文件）进行只读架构审查。审查结论经源码级核验校准后，需全部转化为 **OpenSpec 规范需求**（delta specs + T 测试点），按 P0→P1→P2 分阶段交付。

**核验说明**：子代理原始报告 4+5 条 Critical 中，经复核 **否定 2 条**（D7 worker panic double-completeTask；D2 sandboxast recover fail-open），**下调 1 条**（D7 TurnState TOCTOU → Warning）。本 demand 仅登记 **核验后** 条目。

## 2. 问题陈述（按域）

### D7 Orchestration

| ID | 现象 | 根因（架构层） | 核验严重度 |
|----|------|----------------|-----------|
| RH-D7-01 | 多 session 并发时飞书/tool 事件串 session | 全局单例 `ItemPipelineRunner` + `WorkItemExecutor`，每 Turn 无锁写 `Emit` / `userContextPrepend` | **P0 Critical** |
| RH-D7-02 | 长跑进程 slot release 触发 O(n) goroutine | 每次 `WaveScheduler.Start` / `dispatchLoop` 追加 `OnRelease` hook 从不移除 | **P0 Critical** |
| RH-D7-03 | WorkTree 种子/enrichment 失败仍进 pipeline | `EnsureGoal` 错误 `_, _` 丢弃 | P1 Warning |
| RH-D7-04 | 子 WorkItem 未 terminal 时 loop 空转 | `AwaitRunningChildren` 返回值忽略 | P1 Warning |
| RH-D7-05 | parent rollup 状态可能静默失败 | `resolve.go` 多处 `SetUncertainty` / `UpdateStatus` `_ =` | P1 Warning |
| RH-D7-06 | ItemPipeline phase 与 WorkTree 不一致 | `SetRoundPhase` 错误全部忽略 | P1 Warning |
| RH-D7-07 | TurnState handles map 无界增长 | `EndTurn` 不 delete `handles[sessionID]` | P1 Warning |
| RH-D7-08 | ExpectedReturn 散文 fallback | `child_downlink.go` 自然语言拼接 directive | P1 Warning（规约） |
| RH-D7-09 | Escape LLM 超时后 goroutine/timer 泄漏 | `arbitrator.go` `time.After` + 未 drain 后台 Generate | P1 Warning |
| RH-D7-10 | 自定义 WorkItemExecutor 丢失 Emit | 类型断言仅 `*DefaultWorkItemExecutor` | P2 Info |
| RH-D7-11 | WorkTree.SetStore 无锁 | 热路径与 `mu` 保护读写并发 | P2 Warning |
| RH-D7-12 | MUPS 并行 probe ctx 取消不早退 | `wg.Wait()` 无 `ctx.Done()` 分支 | P2 Warning |
| RH-D7-13 | 战术硬编码残留 | `arbitrator.go` 中文 JSON prompt；`strategic_plan_proposer.go` appendix | P2 规约 |
| RH-D7-14 | Turn 排队语义缺口 | `WaitTurn` 无 handle 时 no-op；`BeginTurn` 在异步 goroutine | P2 Warning（非 TOCTOU） |

### D2 Context Engine

| ID | 现象 | 根因（架构层） | 核验严重度 |
|----|------|----------------|-----------|
| RH-D2-01 | Plan 模式下 `edit_file` 可改任意文件 | 未调用 `EnforcePlanModeWrite`（`write_file` 有） | **P0 Critical** |
| RH-D2-02 | workspace 内 symlink 可穿越到外部 | `resolveWorkspacePath` 无 `EvalSymlinks` | **P0 Critical**（有界） |
| RH-D2-03 | async autocompact 摘要永不写回 SessionContext | 默认 `NoOpCompressionEventSink`；无生产 writeback | **P0 Critical**（async 启用时） |
| RH-D2-04 | 压缩失败 middle 永久丢失 | 失败路径保留 pending placeholder | P0（与 RH-D2-03 联动） |
| RH-D2-05 | sandbox 关闭时 bash 无 AST/regex | `CommandPolicy.Validate` `!Enabled` → nil | P1 Warning |
| RH-D2-06 | bash 审计日志含完整命令 | `slog.Info("tool.bash.audit", "command", command)` | P1 Warning（合规） |
| RH-D2-07 | nil bashAST surface 直接 Allow | `builtin_surface.go` 测试降级路径 | P1 Warning |
| RH-D2-08 | CompressedView 与 Messages 并发写 | `manager.go` 无 `messagesMu` | P1 Warning |
| RH-D2-09 | async summarizer 脱离 session ctx | `context.Background()` + timeout | P1 Warning |
| RH-D2-10 | microcompact 破坏 tool call 链 | 相邻同 role 合并不校验 tool_call_id | P1 Warning |
| RH-D2-11 | JSONL Load 损坏行静默跳过 | `materialize/store.go` continue | P2 Warning |
| RH-D2-12 | God 文件维护成本高 | `context_engine.go` 514 行；`runProcess` ~348 行 | P2 Backlog |

## 3. 业务目标

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| **RH-DC-1** | **工具执行 fail-closed** | Plan 模式 + 路径穿越 + sandbox 降级路径统一守门；无静默放行 |
| **RH-DC-2** | **多 session 编排隔离** | 并发 session 下 Emit / prepend / executor 状态不串线；`-race` 无竞争 |
| **RH-DC-3** | **压缩闭环可验证** | async autocompact 启用时摘要必须写回或显式 degraded；禁止永久 placeholder |
| **RH-DC-4** | **错误可观测不吞没** | 生产路径 `_ = err` 清零；关键失败 emit 结构化日志或 terminal event |
| **RH-DC-5** | **规约一致性** | ExpectedReturn 机器 schema tag；战术 prompt 不进 Go 源码 |

## 4. L1–L5 映射（草案）

| 层级 | 映射 |
|------|------|
| **L1** | D2 Context Engine + D7 Orchestration |
| **L2** | 多 session 安全编排 + 工具沙箱硬化 + 上下文压缩可靠性 |
| **L3-BE** | ProcessMessage / RunSessionTurnLoop / WaveScheduler / ToolRound / Autocompact |
| **L4** | PerInvocationEmit、PlanModeWriteParity、SymlinkContainment、AutocompactWriteback、OnReleaseOnce、FailClosedSandbox |
| **L5** | 见 `specs/*/spec.md` 与 `tasks.md` T 点（24 条 PLANNED） |

## 5. In Scope / Out of Scope

### In Scope

- P0：RH-D7-01/02、RH-D2-01/02/03/04
- P1：RH-D7-03~09、RH-D2-05~10、RH-D7-14
- P2：RH-D7-10~13、RH-D2-11/12（God 文件仅登记 backlog，本 change 不强制拆分）
- delta specs + t-registry PLANNED 条目 + 对应单测/集成测

### Out of Scope

- D7→D3 具体包依赖改接口注入（单独 ADR/change）
- 全量 God 文件拆分（`turn_invoke.go` 816 行等）
- LLM SpawnPolicy 全量 LLM 化（DM-20260630-012 P2 backlog）
- 否定项：worker panic double-completeTask 修复（路径不存在）

## 6. Demand 级验收标准

- [ ] **P0** Plan 模式：`edit_file` 写非 plan 文件 → Deny（与 `write_file` 一致）
- [ ] **P0** workspace 内 symlink 指向外部 → `resolveWorkspacePath` Deny
- [ ] **P0** 两 session 并发 ItemPipeline → `-race` PASS；Emit 不串 session
- [ ] **P0** WaveScheduler 连续 100 次 Start → OnRelease hook 数量不增长
- [ ] **P0** async autocompact 启用 → 完成后 `SessionContext` 含真实摘要（非永久 pending）
- [ ] **P1** `EnsureGoal` 失败 → 结构化 warn 或短路，无 `_, _`
- [ ] **P1** sandbox `enabled=false` 生产配置 → 启动告警或 fail-closed 文档化
- [ ] **P1** t-registry 登记 24 条 T 点（PLANNED → IMPLEMENTED 随 PR）
