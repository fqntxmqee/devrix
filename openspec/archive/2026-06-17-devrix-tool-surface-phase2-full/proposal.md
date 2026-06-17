# Proposal: devrix-tool-surface-phase2-full

**Change ID:** devrix-tool-surface-phase2-full
**Demand ID:** DM-20260617-008
**Parent Demand:** DM-20260617-007 (devrix-tool-surface-contract, S7_archived 2026-06-17)
**Status:** S7_Archived
**Priority:** P0

---

## 1. Background

父 change `devrix-tool-surface-contract` 在 PR #63 阶段 2c 完成了:
- 7 surface + 3 filter + 1 dispatch path 全部走 `surface.Execute`
- 3 入口 (NewContextEngine / buildWithGate / WireDelegate) 收编为 1 入口 (BuildSurfaces)
- toolrunner 层的 3 个 global singleton (freefork_runner / lsp_register / verify_runner) 删除
- `devrix tool list` CLI + `ToolsConfig` config 节
- S6 归档 (verify-archive.sh 12/12 pass)

剩余 5 个 global singleton 待删, 父 design §2.8 阶段 2 已 lock 范围。本 change
是父 change 的执行 followup, 范围严格限定在父 design §2.8 阶段 2 描述的工作。

## 2. 范围

### 2.1 In-Scope (5 个 global 全部删除)

1. **transcript.SetGlobalWriter / GlobalWriter** — `gateway.ExpireSession` 改用 Gateway 字段
2. **flow.SetGlobalHub / GlobalHub** — 4 个 caller 改构造期注入
3. **workmodel.SetGlobalTaskManager / GlobalTaskManager** — 6+ caller 改构造期注入
4. **sessionqueue.GlobalSessionQueue** — 5 caller 改构造期注入
5. **freefork.SetGlobalForker** (在 freefork 包) — `freeforkGlobalFunc` 改接受 Forker 参数

### 2.2 Out-of-Scope (明确不做的)

- 不引入新 surface / filter / ToolSpec 字段
- 不修改父 change 的 surface 行为
- 不修改 D2/D3/D4/D5/D6 library 对外 API
- 不重命名任何 6+ global (已经命名为 "SetGlobal*" 模式的, 仅删 var + setter, 不重命名)
- 不引入新的 dependency injection framework (用显式 ctor 参数即可)

## 3. 实现策略

按 global 分 5 个 sub-commit, 每个 commit 独立可编译 + 独立测试可绿:

| Sub-commit | Global | 关键改动 | 估时 |
|-----------|--------|----------|------|
| 1 | `transcript.GlobalWriter` | `Gateway.Writer *Writer` 字段 + `WireGateway(opts, writer)` 注入 | 0.5 天 |
| 2 | `flow.GlobalHub` | `flow.NewHub(contracts.SessionCommandQueue)` 显式 ctor; 3 caller 接受 `*Hub` 参数 | 1.0 天 |
| 3 | `sessionqueue.GlobalSessionQueue` | `EngineDeps.SessionCommandQueue` 已是字段; 仅删 global var + 5 caller 改用 NewSessionQueue() 局部实例 | 0.5 天 |
| 4 | `workmodel.GlobalTaskManager` | `TaskManager` 改 non-global; 6+ caller 改构造期注入; `InitGlobalTaskManager` 函数保留作为工厂 | 1.0 天 |
| 5 | `freefork.SetGlobalForker` | `freeforkGlobalFunc` 改接受 Forker 参数; `WireMultiAgent` 显式返回 Forker | 0.5 天 |

## 4. 验收 (per 父 change AC 重新计数)

父 change 的 AC4 (6+ global 全删) 和 AC14 (SetGlobalXxx API 全删) 当前 PARTIAL,
本 change 完成后两者转 PASS。

其他 AC 状态保持 (W11 phase 2c + W12 + W13 + W14 已固化)。

## 5. 风险与回滚

- **回滚**: 父 design §2.8 明确 "阶段 2 后不推荐回滚, 需用 `git revert` 恢复 global var"。本 change 不增加新的回滚路径。
- **灰度**: 阶段 2 是最终态, 无灰度期。devrix binary 新版 100% 替换。
- **测试**: `TestAppendAndTrimMessages_ExistingSession` 父 change 已知 flaky, 本 change 不修复, 仅在 verify 时记录。
