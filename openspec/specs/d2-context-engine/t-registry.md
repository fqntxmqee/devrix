# D2 Context Engine Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.11.0
**Last Updated:** 2026-06-29
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `openspec/specs/d2-context-engine/d2-domain.md`
**Change:** devrix-tool-surface-contract (DM-20260617-007) — W1-W9 阶段 1 落地：7 surface + 3 filter + turn_adapter dispatch 路径
**Change:** devrix-tool-surface-phase2-full (DM-20260617-008) — W1-W5 阶段 2 落地：5 剩余 global singleton 全删
**Change:** devrix-tool-spec-enrichment (DM-20260618-001) — v2: ToolSpec 4 bool + InterruptBehavior (5th method) + BuildSurfaces sort + parallel dispatch (T22-T25)
**Change:** devrix-surface-permission-extension (DM-20260618-002) — v3: CheckPermission (6th method) + Decision enum + BashAST + IPermissionGate + turn_adapter 2-phase (T26-T29, PERMISSION-GATE-1-T01/T02)
**Change:** devrix-surface-lazy-loading (DM-20260618-003) — DeferLoading + ShouldDefer + ToolSearchSurface + zodgen (T30-T34)
**Change:** devrix-ask-user-question (DM-20260618-006) — AskUserQuestionSurface (9th) + IM 推送 sender 桥接 (T35-T38)
**Change:** devrix-tools-terminal-architecture (DM-20260618-007) — LSP 5 typed method spec (T02-T04) + BashAST fail-closed + zsh 22+ rules (T05-T06) + cross-cutting LTL-Lite framework
**Change:** 2026-06-20-devrix-context-budget-and-isolation (devrix-context-budget-and-isolation / DM-20260620-001) — Phase A: AC1 tool result size cap (D2-S17-A05 T01-T05) + AC2 assistant fold (D2-S17-A06 T01-T03) + AC4+AC13 per-iter token audit + proactive fold (D2-S15-A08 T01-T05); TruncateToTokens dead-code upgraded to required.
**Change:** 2026-06-20-devrix-context-budget-and-isolation-phase-b (devrix-context-budget-and-isolation / DM-20260620-001-B) — Phase B: AC11a fork prefix byte-level stability via `BuildForkedMessages` (D2-S15-A08 T06-T08); IMPLEMENTED 109→112, P0 56→59.
**Change:** 2026-06-20-devrix-context-budget-phase-c-nested (devrix-context-budget-phase-c-nested / DM-20260620-002) — Phase C: nested-branch budget injection via `SubTurnRequest.MaxContextTokens` → `TurnRequest.MaxContextTokens` (D2-S15-A08 T09-T10); IMPLEMENTED 112→114, P0 59→61.

---

## Canonical T 映射（DM-20260614-009）

v1.0：**不修改**现有测试 `// T:` 注释。下表供追溯与新测试登记。

> **ValueFlow Alias (用户感知, DM-20260629-002 PR-5):**
> - D2-S15 (PrepareExecutionContext) → `D2_Context_Loading_Compression`
> - D2-S17 (PersistSessionState) → `D2_Session_State_Persistence`
> - D2-S18 (EnforceExecutionPolicy) → `D2_Tool_Permission_Sandbox`
> - D2-S16 (RunQueryLoop) → (归 D7 ValueFlow, REMOVED DM-20260618-010)

| Canonical T ID | Legacy T ID | Canonical S | 描述 | Status |
|----------------|-------------|-------------|------|--------|
| D2-S15-A01-T01 | D2-S3-A01-T01 | S15 | 新会话历史正确追加 | IMPLEMENTED |
| D2-S15-A02-T01 | D2-S13-A01-T01 | S15 | RepairToolChain 修复 orphan | IMPLEMENTED | `prepare/conversation/repair_test.go` |
| D2-S15-A03-T01 | D2-S2-A01-T01 | S15 | 超阈值触发压缩 | IMPLEMENTED |
| D2-S16-A01-T01 | D2-S10-A01-T34 | S16 | Multi-turn tool loop | IMPLEMENTED |
| D2-S16-A01-T02 | D2-CTX-T01 | S16 | Process cancel 无 panic | IMPLEMENTED |
| D2-S16-A01-T03 | — | S16 | query 包无 D4/D7 import | IMPLEMENTED | `internal/lint/layer/d2_thin_test.go` |
| D2-S17-A01-T01 | D2-S3-A01-T02 | S17 | Deferred complete 后快照 | IMPLEMENTED |
| D2-S17-A02-T01 | D2-S6-A02-T01 | S17 | Main transcript append | IMPLEMENTED |
| D2-S18-A01-T01 | D2-CTX-T36 | S18 | Plan mode write deny | IMPLEMENTED |
| D2-S18-A02-T01 | D2-S8-A01-T01 | S18 | Bash sandbox workdir | IMPLEMENTED |
| D2-S19-A01-T01 | D2-CTX-T40 | ~~S19~~ → S18 | Explore read-only | IMPLEMENTED → `enforce/subquery_test.go` |
| D2-S19-A02-T01 | D2-CTX-T41 | ~~S19~~ → S18 | Fork identical prefix | IMPLEMENTED → `prepare/conversation/fork_test.go` |
| D2-S20-A01-T01 | D2-S11-A01-T02 | ~~S20~~ | ~~默认跳过 harness~~ | **REMOVED（v6.5.0）** |
| D2-S20-A02-T01 | D2-S9-A01-T01 | ~~S20~~ | ~~Legacy bootstrap 一次~~ | **REMOVED（v6.5.0）** |

---

## Legacy Module T（冻结追溯）

## D2-S3: Memory Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S3-A01-T01 | 新会话历史正确追加 | Memory | `internal/layers/contextengine/prepare/memory/manager_test.go` | IMPLEMENTED |
| D2-S3-A01-T02 | ContextSnapshot 备份 | Memory | `internal/layers/contextengine/persist/snapshot/store_test.go` | IMPLEMENTED |
| D2-S3-A01-T03 | LongTerm Recall 注入上下文 | Memory | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| D2-S3-A01-T04 | LongTerm Store 持久化写入 | Memory | `internal/layers/contextengine/prepare/memory/longterm_test.go` | IMPLEMENTED |
| D2-S3-A01-T05 | L3 长期记忆返回 NotImplemented | Memory | `internal/layers/contextengine/prepare/memory/longterm_test.go` | IMPLEMENTED |
| D2-S3-A01-T06 | 快照使用 snappy 压缩体积显著缩减 | Memory | `internal/layers/contextengine/persist/snapshot/store_test.go` | IMPLEMENTED |
| **D2-S3-A02-T01** | **ContextEngine.AppendAndTrimMessages 写入 D2 内存并按 budget 裁剪（DM-20260617-003 D7 turn bridge）** | **Memory** | **`internal/layers/contextengine/engine_persist_bridge_test.go::Test{AppendAndTrimMessages_EmptyMessages,FreshSession,ExistingSession,TrimTriggered,RaceSafety}`** | **IMPLEMENTED** | **P0** |
| **D2-S3-A02-T02** | **AppendAndTrimMessages lazy-init 不存在的 sid** | **Memory** | **`internal/layers/contextengine/engine_persist_bridge_test.go::TestAppendAndTrimMessages_FreshSession`** | **IMPLEMENTED** | **P0** |

## D2-S2: Compression Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S2-A01-T01 | 超 Token 阈值触发七步压缩 | Compression | `tests/acceptance/p0/ctx_compression_test.go` | IMPLEMENTED |
| D2-S2-A01-T02 | TokenBlock 超限返回 ContextExceeded | Compression | `internal/layers/contextengine/prepare/compression/pipeline_test.go` | IMPLEMENTED |
| D2-S2-A01-T03 | Autocompact 触发并降低 token | Compression | `internal/layers/contextengine/prepare/compression/autocompact_test.go` | IMPLEMENTED |
| D2-S2-A01-T04 | Autocompact LLM 失败降级跳过 | Compression | `internal/layers/contextengine/prepare/compression/autocompact_test.go` | IMPLEMENTED |
| D2-S2-A01-T05 | Autocompact 禁用时跳过步骤 6 | Compression | `internal/layers/contextengine/prepare/compression/pipeline_test.go` | IMPLEMENTED |
| D2-S2-A01-T06 | 异步压缩占位不阻塞主路径 | Compression | `internal/layers/contextengine/prepare/compression/async_compact_test.go` | IMPLEMENTED |
| D2-S2-A01-T07 | 异步压缩失败降级不丢失数据 | Compression | `internal/layers/contextengine/prepare/compression/async_compact_test.go` | IMPLEMENTED |
| D2-S2-A01-T08 | Autocompact timeout fallback | Compression | `internal/layers/contextengine/prepare/compression/autocompact_test.go` | IMPLEMENTED |

## D2-S1: PEV Module (RETIRED)

> **2026-06-13**：PEV 引擎已下线，主路径由 **D2-S10 QueryLoop** 承接。

| T ID | 描述 | 迁移 / 备注 | Status |
|-------|------|-------------|--------|
| D2-S1-A01-T01–T04 | PEV Execute / Gateway 四握 | → `D2-S10-A01-T34` | RETIRED |
| D2-S1-A02-T02,T05,T06,T09 | Verify commands | PEV Verify 已移除 | RETIRED |
| D2-S1-A02-T10 | Shell injection | → `D2-S8-A01-T01` | MIGRATED |
| D2-S1-A03-T07,T08 | Milestone Plan | PEV Plan 已移除 | RETIRED |
| D2-S1-A01-T11–T14 | PEV 并发/span/complete | QueryLoop 路径替代 | RETIRED |

## D2-S4: Token Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S4-A01-T01 | Token 计数共享约定与 Gateway 对齐 | Token | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| D2-S4-A01-T02 | lsp_go_to_definition spec + Execute 路径 | LSP | `tests/integration/tools_terminal_test.go` (TestLSP_End2End) | IMPLEMENTED | P0 |
| D2-S4-A01-T03 | lsp_find_references spec + Execute 路径 | LSP | `tests/integration/tools_terminal_test.go` (TestLSP_End2End) | IMPLEMENTED | P0 |
| D2-S4-A01-T04 | lsp_incoming_calls / hover / workspace_symbol spec 暴露 | LSP | `tests/integration/tools_terminal_test.go` (TestLSP_End2End) | IMPLEMENTED | P0 |
| D2-S4-A01-T05 | bash audit + policy decision (fail-closed) | Bash | `tests/integration/tools_terminal_test.go` (TestBashAST_DenyAttack) | IMPLEMENTED | P0 |
| D2-S4-A01-T06 | zsh attack pattern deny (22+ rules) | Bash | `internal/layers/contextengine/enforce/tools/bash/` | IMPLEMENTED | P0 |

## D2-S8: Sandbox Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S8-A01-T01 | bash 困居工作目录 + 命令白名单 | Sandbox | `internal/layers/contextengine/enforce/tools/sandbox_test.go` | IMPLEMENTED |
| D2-S8-A01-T02 | Shell injection attack prevention | Sandbox | `tests/security/shell_injection_test.go` | IMPLEMENTED |

## D2-S9: Harness Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S9-A01-T01 | ~~harness.enabled 首次 Process 触发 Bootstrap~~ | ~~Harness~~ | ~~`tests/integration/context_harness_bootstrap_test.go`~~ | **REMOVED（v6.5.0）** | P0 |
| D2-S9-A01-T02 | ~~WorkspaceContext 扫描 Go 文件与 AGENTS.md~~ | ~~Harness~~ | ~~`internal/layers/contextengine/fallback/workspace_test.go`~~ | **REMOVED（v6.5.0）** | P1 |
| D2-S9-A01-T03 | ~~Bootstrap 幂等（同 Session 不重复）~~ | ~~Harness~~ | ~~`tests/integration/context_harness_bootstrap_test.go`~~ | **REMOVED（v6.5.0）** | P1 |
| D2-S9-A01-T04 | ~~trusted=false 时 deferred_init 标志全 false~~ | ~~Harness~~ | ~~`tests/acceptance/p0/ctx_harness_bootstrap_test.go`~~ | **REMOVED（v6.5.0）** | P0 |
| D2-S9-A03-T05 | ~~ToolPool simple_mode / MCP / deny 过滤~~ | ~~Harness~~ | ~~`internal/layers/contextengine/fallback/toolpool_test.go`~~ | **REMOVED（v6.5.0）** | P0 |
| D2-S9-A01-T06 | ~~PromptRouter advisory 关键词计分~~ | ~~Harness~~ | ~~`internal/layers/contextengine/fallback/router_test.go`~~ | **REMOVED（v6.5.0）** | P2 |
| D2-S9-A01-T07 | ~~Transcript 内存分离与 compact~~ | ~~Harness~~ | ~~`internal/layers/contextengine/fallback/transcript_test.go`~~ | **REMOVED（v6.5.0）** | P1 |
| D2-S9-A01-T08 | ~~harness.enabled=false V4 回归 + 无 bootstrap info~~ | ~~Harness~~ | ~~`tests/integration/context_harness_bootstrap_test.go`~~ | **REMOVED（v6.5.0）** | P0 |
| D2-S9-A01-T09 | ~~Preflight warn-only 规则评分与 tool filter~~ | ~~Harness~~ | ~~`internal/layers/contextengine/fallback/preflight_test.go`~~ | **REMOVED（v6.5.0）** | P1 |
| D2-S9-A02-T10 | System Prompt Assembly §十 XML 块 | S15 | `internal/layers/contextengine/prepare/prompt/assembler_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T11 | Jaeger span 树（enabled/disabled） | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |
| D2-S9-A02-T12 | disabled 与 BuildLegacy 字节级一致 | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A02-T13 | CompressedView system = Build 输出 | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A03-T14 | QueryLoop 可见工具 ⊆ VisibleTools | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T15 | bootstrap.stage parent = bootstrap.run | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |

## D2-S9.BG: Background SubQuery Task Tools

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S9-A01-T16 | stop running task → cancelled (idempotent) | BGTask | `internal/layers/contextengine/enforce/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T17 | output block=false 返回 running 状态 + partial result | BGTask | `internal/layers/contextengine/enforce/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T18 | output block=true 阻塞至 terminal 或 timeout（max 600s） | BGTask | `internal/layers/contextengine/enforce/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T19 | cancel 后 SessionQueue 不发 completed notification（tombstone 协议） | BGTask | `internal/layers/contextengine/enforce/background_cancel_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T20 | IsTerminal 对 running/cancelled/completed/failed 正确报告（Phase 3 Wave WorkerCancelRegistry） | BGTask | `internal/layers/contextengine/enforce/background_cancel_test.go` | IMPLEMENTED | P1 |

## D2-S10: QueryLoop Module — **REMOVED (DM-20260618-010)**

> 多轮 LLM↔Tool 循环已迁移至 D7 `RunTurn` / `SubTurn`。下列 T 点中机制类测试仍有效，loop 专属测试见 D7-S2-A06。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S10-A01-T34 | 多轮 tool_use 直至无 tool | D7-S2-A06 | `internal/layers/orchestration/turn/orchestrator_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T35 | UserContext prepend 不在 snapshot | S15 | `internal/layers/contextengine/prepare/usercontext/provider_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T36 | plan_mode attachment full/sparse throttle | S15 | `internal/layers/contextengine/prepare/attachments/registry.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T37 | plan mode 拒绝 Write 非 plan 文件 | S18 | `internal/layers/contextengine/plan_mode_tools_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T38 | task_create 磁盘持久 + list 一致 | D7-S1 | `internal/layers/orchestration/workmodel/disk_store_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T39 | PreparedTurnRunner multi-turn | D7 | `internal/layers/contextengine/prepared_turn_integration_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T40 | SubQuery Explore omitClaudeMd + read-only | S18 | `internal/layers/contextengine/enforce/subquery_test.go` | IMPLEMENTED | P1 |
| D2-S10-A01-T41 | Fork subagent placeholder tool_results 一致 | S15 | `internal/layers/contextengine/prepare/conversation/fork_test.go` | IMPLEMENTED | P1 |
| D2-S10-A01-T42 | sidechain transcript resume 重建 messages | S17 | `internal/layers/contextengine/persist/transcript/sidechain_test.go` | IMPLEMENTED | P1 |

## D2-S17: Context Budget Persistence (DM-20260620-001, Phase A)

> **devrix-context-budget-and-isolation (DM-20260620-001) — Phase A 落地。**
> 两条新价值流子能力：(1) S17-A05 `ToolResultStore` 把 oversized 工具结果
> 落盘并返回 `<persisted-output>` 预览标记；(2) S17-A06 `FoldAssistantOutput`
> 把 assistant 长输出 head/tail 折叠并返回 `<prior-output-summary>` 标记。
> 两条均服务于 turn loop (D7-S2-A06) 防 51K-token 失控。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| **D2-S17-A05-T01** | **AC1 below-limit passthrough: `ToolResultStore.Persist` 短结果原样返回 + 不落盘** | **S17-A05 ToolResultStore (new)** | **`internal/layers/contextengine/prepare/persist/tool_result_store_test.go::TestToolResultStore_Persist_BelowLimit`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S17-A05-T02** | **AC1 above-limit persists with preview: 超 `MaxToolResultChars` 落 `~/.devrix/tool-results/<sessionID>/<stamp>-<id>.txt` + 返回 `<persisted-output>` 标记（含 size + 路径 + preview head）** | **S17-A05 ToolResultStore (new)** | **`tool_result_store_test.go::TestToolResultStore_Persist_AboveLimit_WhitelistedTool` + `TestToolResultStore_Persist_GeneratesPersistedOutputMarker`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S17-A05-T03** | **AC1 allowlist enforced: 非白名单 tool (task_create / delegate_worker 等) 不 cap, 原内容保留** | **S17-A05 ShouldCap (new)** | **`tool_result_store_test.go::TestToolResultStore_ShouldCap_AllowlistEnforced` + `TestToolResultStore_Persist_NonWhitelistedTool_NoCap`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S17-A05-T04** | **AC1 session ID sanitised: sessionID 含 `/` `\` `..` 时替换为 `_`，路径不能逃逸 root** | **S17-A05 sanitizeSegment (new)** | **`tool_result_store_test.go::TestToolResultStore_Persist_SessionIDSanitised` + `TestToolResultStore_Persist_PathTraversalBlocked`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S17-A05-T05** | **AC1 persist failure falls back to head truncation: I/O error → head truncate + "[truncated, persist failed]" trailer; turn loop 不中止** | **S17-A05 Persist (failure path)** | **`tool_result_store_test.go::TestToolResultStore_Persist_IOError_FallsBackToHeadTruncate` + `internal/layers/orchestration/turn/orchestrator_toolcap_test.go::TestOrchestrator_BuildToolResultMsgWithCap_PersistFailure`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S17-A06-T01** | **AC2 below-limit passthrough: `FoldAssistantOutput` 短结果原样返回 + 不落盘** | **S17-A06 FoldAssistantOutput (new)** | **`internal/layers/contextengine/prepare/persist/turn_output_store_test.go::TestFoldAssistantOutput_BelowLimit`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S17-A06-T02** | **AC2 above-limit folds head + tail: 超 `MaxAssistantChars` 落 `~/.devrix/tool-results/<sessionID>/turn-outputs/t<n>-<stamp>-<id>.txt` + 返回 `<prior-output-summary>` (前 800 + middle marker + 后 200 runes)** | **S17-A06 FoldAssistantOutput (new)** | **`turn_output_store_test.go::TestFoldAssistantOutput_AboveLimit_HeadTail` + `TestFoldAssistantOutput_GeneratesPriorOutputSummaryMarker`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S17-A06-T03** | **AC2 tool_calls metadata preserved: `buildAssistantToolCallMsgFolded` 保留 `Metadata["tool_calls"]`，仅替换 Content** | **S17-A06 buildAssistantToolCallMsgFolded (new)** | **`orchestrator_toolcap_test.go::TestOrchestrator_BuildAssistantToolCallMsgFolded_PreservesToolCalls`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |

## D2-S15: Prepare — Token Audit (DM-20260620-001, Phase A)

> **devrix-context-budget-and-isolation (DM-20260620-001) — Phase A 落地。**
> AC4 + AC13：turn loop 每 iteration 开头调 `AuditMessages` +
> `ShouldFoldProactively`；Proactive 阈值 60% (DefaultProactiveFoldPercent)；
> TruncateToTokens 从 dead-code 升级为 turn loop 必调。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| **D2-S15-A08-T01** | **AC4 audit fires per iteration: `runTokenAudit` 计算 TotalTokens / SystemTokens / MessagesTokens / LargestMsgTokens / LargestMsgIdx / BudgetPercent / OverBudget + 挂 `audit.*` span attrs + 结构化 slog `orchestrator: token audit`** | **S15-A08 TokenAudit (new)** | **`internal/layers/contextengine/prepare/audit/token_audit_test.go::TestAuditMessages_*` + `internal/layers/orchestration/turn/orchestrator_toolcap_test.go::TestOrchestrator_RunTokenAudit_*`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S15-A08-T02** | **AC4 proactive fold fires at 60% threshold: `TotalTokens / Budget >= 0.6` 且最大消息 > `MaxAssistantChars` → fold in-place via `FoldAssistantOutput` BEFORE LLM invoke** | **S15-A08 ShouldFoldProactively (new)** | **`token_audit_test.go::TestShouldFoldProactively_AboveProactiveThreshold` + `TestShouldFoldProactively_BelowThreshold` + `TestShouldFoldProactively_LargestUnderCap_NoFold`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S15-A08-T03** | **AC4 proactive fold fires on over-budget: `TotalTokens > Budget` → `true` 无视百分比阈值** | **S15-A08 ShouldFoldProactively (new)** | **`token_audit_test.go::TestShouldFoldProactively_OverBudget`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S15-A08-T04** | **AC4 proactive fold no-op when below threshold: `TotalTokens / Budget < 0.6` 且 `<= Budget` → `false` + buffer 留空** | **S15-A08 ShouldFoldProactively (new)** | **`token_audit_test.go::TestShouldFoldProactively_BelowThreshold`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D2-S15-A08-T05** | **AC4 largest message under cap → no fold: 最大消息短于 `MaxAssistantChars` 即使 budget 60% 也返 `false`（避免无意义折叠）** | **S15-A08 ShouldFoldProactively (new)** | **`token_audit_test.go::TestShouldFoldProactively_LargestUnderCap_NoFold`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |

### D2-S15: BuildForkedMessages — Fork Mode Helpers (DM-20260620-001-B, Phase B)

> **devrix-context-budget-and-isolation (DM-20260620-001-B) — Phase B 落地（待 B.5 验证 + S6 归档）。**
> AC11a fork mode prefix byte-level stability：`BuildForkedMessages` 输出
> `[cloned_assistant, directive_user_with_placeholder]`；`ForkPrefixFingerprint`
> 在 sibling 间 byte-identical；placeholder 文本 `"Fork started — processing in background"`
> 必须 byte-exact 不变（未来 Anthropic `cache_control` 锚点）。
> 无 tool_calls fallback 文档化（SubTurnRunner 上层拒绝非 fork 候选请求）。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| **D2-S15-A08-T06** | **AC11a fork prefix stable: sibling siblings 走同一 parent → `BuildForkedMessages` 输出 + `ForkPrefixFingerprint` byte-identical** | **S15-A08 BuildForkedMessages** | **`internal/layers/contextengine/prepare/conversation/fork_test.go::TestBuildForkedMessages_should_use_identical_placeholder_results`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |
| **D2-S15-A08-T07** | **fork mode boundary: parent 无 tool_calls → `BuildForkedMessages` 返回 1 条 directive-only user message，无 placeholder（SubTurnRunner 上层拒绝非 fork 候选）** | **S15-A08 BuildForkedMessages (fallback)** | **`internal/layers/contextengine/prepare/conversation/fork_test.go::TestBuildForkedMessages_NoToolCallsFallback`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |
| **D2-S15-A08-T08** | **AC11a multi-call placeholder: parent 含 N 个 tool_calls → directive user 含 N 个 `[tool_result id=X]\nForkPlaceholderResult` 块（每个 ID 一份）** | **S15-A08 BuildForkedMessages (multi-call)** | **`internal/layers/contextengine/prepare/conversation/fork_test.go::TestBuildForkedMessages_MultipleToolCallPlaceholders`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** |

### D2-S15: Nested-Branch Budget Propagation (DM-20260620-002, Phase C)

> **devrix-context-budget-phase-c-nested (DM-20260620-002) — Phase C 落地。**
> AC1 nested 分支 maxContextTokens 注入：SubTurnRequest → TurnRequest → nested
> branch 读 `req.MaxContextTokens` + fallback `o.maxContextTokens`；enforce.Run
> 把 SubQueryParams.MaxContextTokens 透传到 SubTurnRequest。无 nested-branch
> fallback 时 `runTokenAudit` / `ShouldFoldProactively` / tool result cap 全部
> no-op（maxContextTokens=0 触发三守卫短路）。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| **D2-S15-A08-T09** | **AC1 SubTurnRequest→TurnRequest 透传: `SubTurnRequest.MaxContextTokens` 经 `SubTurnRunner.RunSubTurn` 注入 `TurnRequest.MaxContextTokens`（`req.MaxContextTokens` 优先，否则 `Cfg.MaxContextTokens`，否则 0）** | **S15-A08 SubTurnRequest.MaxContextTokens (new)** | **`internal/layers/orchestration/turn/subturn_test.go::TestSubTurnRunner_MaxContextTokens_Propagated_DM_20260620_002`** | **IMPLEMENTED (DM-20260620-002)** | **P0** |
| **D2-S15-A08-T10** | **AC1 enforce.Run 透传: `SubQueryParams.MaxContextTokens` 在 `enforce.Run` 中写入 `SubTurnRequest.MaxContextTokens`（无 `MaxContextTokens` 时 0，让 SubTurnRunner 走 `Cfg.MaxContextTokens` fallback）** | **S15-A08 SubQueryParams.MaxContextTokens (new)** | **`internal/layers/contextengine/enforce/subquery_test.go::TestSubQuery_MaxContextTokens_PassedToSubTurn_DM_20260620_002`** | **IMPLEMENTED (DM-20260620-002)** | **P0** |

## D2-S11: Harness Unification

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S11-A01-T01 | `turn_runtime` turn defaults (max_turns, compress_per_turn) | HarnessUnification | `internal/shared/config/turn_runtime_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T02 | harnessEnabled 分支不再被生产路径触发 | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T03 | 旧路径调用计数基线=0 | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T04 | 压缩入口统一：D7 turn 走 messages-only 七步管道 | HarnessUnification | `internal/layers/contextengine/compression_unified_test.go` | IMPLEMENTED | P1 |
| D2-S11-A01-TD01 | TD-QL-01: 413 → compress → 重试 | D7 | `internal/layers/orchestration/turn/recovery_test.go` | IMPLEMENTED | P1 |
| D2-S11-A01-TD03 | TD-QL-03: overload/5xx → gateway retry | D3 | `internal/layers/orchestration/turn/llm.go` (GatewayInvoker) | IMPLEMENTED | P1 |
| D2-S11-A01-D6PR | D6 PathRegressionProbe: legacy_harness > 0 ⇒ score 0 | HarnessUnification | `internal/layers/evolution/evaluate/path_regression_probe_test.go` | IMPLEMENTED | P0 |

## D2-S12: Worktree Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S12-A01-T01 | Sandbox enter 后 write 不污染主 WorkDir | D2-S18 | `internal/layers/contextengine/sandbox/manager_test.go` | IMPLEMENTED | P0 |

## D2-S6: Snapshot & Main Transcript

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S6-A02-T01 | Main transcript append-only JSONL 读写 | Transcript | `internal/layers/contextengine/persist/transcript/main_thread_test.go` | IMPLEMENTED | P1 |

## D2-S13: Conversation Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S13-A01-T01 | RepairToolMessageChain 剔除 orphan tool results | Conversation | `internal/layers/contextengine/prepare/conversation/repair_test.go` | IMPLEMENTED | P0 |
| D2-S13-A02-T01 | MessagesAfterCompactBoundary 仅保留尾部 | Conversation | `internal/layers/contextengine/prepare/conversation/boundary_test.go` | IMPLEMENTED | P1 |

## TOOL-SURFACE-1: Tool Surface Contract (DM-20260617-007)

> **devrix-tool-surface-contract (DM-20260617-007) — W1-W9 阶段 1 落地。**
> 7 个 surface (Builtin / LSP / FreeFork / Tracker / Verify / Delegate /
> BackgroundTask) + 3 个 filter (PerAgent / PerRisk / decisionplanning
> adapter) + 1 个 dispatch path (turn_adapter.findSurface).
> 完整列表见 `openspec/changes/devrix-tool-surface-contract/design.md` §2.5-§2.6。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| TOOL-SURFACE-1-T01 | ToolSurface 4 方法契约 (Name/Tools/RiskLevel/Execute) 编译期断言 | Surface Contract | `internal/shared/contracts/tool_surface_test.go` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T02 | ToolFilter 1 方法契约 (Apply) + Composite/Allow/Deny/ApplyFilters | Filter Contract | `internal/shared/contracts/tool_filter_test.go` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T03 | BuiltinSurface + LSPToolSurface 可见性 (cfg nil / 无 server / 有 server) | Surface Wiring | `internal/layers/contextengine/enforce/tools/surface/{builtin,lsptool}_surface_test.go` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T04 | FreeForkSurface + TrackerSurface + VerifySurface 行为 + 接口合规 | Surface Wiring | `internal/layers/contextengine/enforce/tools/surface/{w4_surfaces,w4_surfaces}_test.go` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T05 | PluginSurface dispatch (delegate + background 共享) | Surface Wiring | `internal/layers/contextengine/enforce/tools/surface/{w5_surfaces,plugin}_test.go` | IMPLEMENTED | P0 |
| TOOL-FILTER-1-T01 | PerAgentFilter 5 agent + WithAllowlist + 不可变 + 未知 agent | Filter Impl | `internal/layers/contextengine/enforce/tools/filter/w6_filters_test.go` | IMPLEMENTED | P0 |
| TOOL-FILTER-1-T02 | PerRiskFilter 4 阈值 + 空阈值 pass-through | Filter Impl | `internal/layers/contextengine/enforce/tools/filter/w6_filters_test.go` | IMPLEMENTED | P0 |
| TOOL-FILTER-1-T03 | Composite FIFO 顺序 + 顺序敏感性 | Filter Composition | `internal/layers/contextengine/enforce/tools/filter/w6_filters_test.go` | IMPLEMENTED | P1 |
| TOOL-FILTER-1-T04 | decisionplanning.AsToolFilter 5 agent 适配 | Adapter | `internal/layers/orchestration/decisionplanning/filter_adapter_test.go` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T08 | NewContextEngine 收编 surface 列表 (BuildSurfaces + DefaultFilters) | Engine Wiring | `internal/bootstrap/surfaces_test.go` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T09 | turn_adapter.ExecuteRound 走 surface.Execute (findSurface 线性扫) | Dispatch Path | `internal/bootstrap/turn_adapter_surface_test.go` | IMPLEMENTED | P0 |

## TOOL-SURFACE-1: Phase 2 Full — Global Singleton Cleanup (DM-20260617-008)

> **devrix-tool-surface-phase2-full (DM-20260617-008) — W1-W5 阶段 2 落地。**
> 删除 5 个剩余 global singleton (transcript / flow / sessionqueue [now
> in executionflow] / workmodel / freefork-in-pkg), 全部 caller 改构造期
> 显式 dep 注入。
> 父 change AC4 (6+ global 全删) + AC14 (SetGlobalXxx API 全删) 由 PARTIAL 转 PASS。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| TOOL-SURFACE-1-T15 | transcript.GlobalWriter 零引用 + Gateway.Writer 字段注入 | Global Cleanup | git grep + `internal/layers/communication/capture/gateway.go` (Writer field) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T16 | flow.GlobalHub 零引用 + delegatetools.Deps.Hub 字段注入 | Global Cleanup | git grep + `internal/layers/orchestration/delegatetools/deps.go` (Hub field) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T17 | sessionqueue.GlobalSessionQueue 零引用 + 5 caller 局部 NewSessionQueue() | Global Cleanup | git grep + `internal/layers/orchestration/executionflow/session_queue.go` (no Global var) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T18 | workmodel.GlobalTaskManager 零引用 + 6+ caller ctor 注入 (Orchestrator.tasks / CommandHandler.tasks / delegatetools.Deps.Tasks / cli.NewCLIAdapter) | Global Cleanup | git grep + `internal/layers/orchestration/workmodel/task_manager.go` (no Global var) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T19 | freefork.SetGlobalForker 零引用 + freeforkGlobalFunc 参数化 Forker | Global Cleanup | git grep + `internal/layers/multiagent/provision/freefork/freefork_injection.go` (factory pattern) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T20 | git grep 验证 5 global + 5 setter 全删 (production-code 0 命中) | Static Verify | `git grep -nE "SetGlobal\|GlobalSessionQueue\|GlobalTaskManager\|GlobalHub\|GlobalWriter\|GlobalForker" internal/` (only comment matches) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T21 | go test -race -count=1 ./... 100% 绿 (89 packages) | Dynamic Verify | `go test -race -timeout 180s -count=1 ./...` (all OK) | IMPLEMENTED | P0 |

## TOOL-SURFACE-1: v2 — Tool Spec Enrichment (DM-20260618-001)

> **devrix-tool-spec-enrichment (DM-20260618-001) — 4 个 P0 T 点。**
> ToolSpec 增加 4 个正交 bool 字段 (ReadOnly / Destructive / OpenWorld /
> ConcurrencySafe) + 5th method `InterruptBehavior` + BuildSurfaces
> 排序稳定 + turn_adapter 并行 dispatch。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|--------|-----------|--------|----------|
| TOOL-SURFACE-1-T22 | 7 surface Tools() 100% 填充 4 bool 字段（每 spec 至少 1 个 true） | ToolSpec v2 | `tests/integration/tool_surface_test.go::TestIntegration_AllSurfaces_HaveCompleteOrthogonalFlags` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T23 | FreeForkSurface.InterruptBehavior=InterruptCancel，6 short-run surface=InterruptBlock；free_fork cancel 200ms 内返回 | InterruptBehavior | `internal/layers/contextengine/enforce/tools/surface/{freefork,builtin,lsptool,tracker,verify,delegate,background_task}_surface_test.go` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T24 | BuildSurfaces 排序稳定 (lexicographic by Name)，3 套 opts 顺序一致 | BuildSurfaces | `internal/bootstrap/surfaces_test.go::TestBuildSurfaces_StableOrder` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T25 | turn_adapter 并行 dispatch (ConcurrencySafe=true → errgroup；=false → 顺序)；indexed write-back 保序；race 无报警 | Parallel Dispatch | `tests/integration/turn_adapter_test.go::TestTurnAdapter_ParallelDispatch` | IMPLEMENTED | P0 |

## TOOL-SURFACE-1: v3 — Surface Permission Extension (DM-20260618-002)

> **devrix-surface-permission-extension (DM-20260618-002) — 4 个 P0 T 点。**
> ToolSurface 加 6th method `CheckPermission(ctx, spec, input) Decision`，
> Decision 三态 (Allow / Deny / Ask)，BashASTPolicy 5 deny rules，
> IPermissionGate.CheckPermission 消费 ToolSpec.OpenWorld 字段，
> turn_adapter 2-phase dispatch。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|--------|-----------|--------|----------|
| TOOL-SURFACE-1-T26 | Surface.CheckPermission 5 short-run surface=DecisionAllow；Surface 返回 Ask 时 turn_adapter 调 IPermissionGate 进一步决策 | Surface Permission | `internal/bootstrap/turn_adapter_surface_test.go::TestCheckPermission_Ask_EscalatesToIPermissionGate` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T27 | BashASTPolicy 默认 deny-list (rm -rf /, dd, mkfs, sudo, chmod 777 /) → DecisionDeny；parse 错误 → Ask | BashAST | `internal/layers/contextengine/enforce/tools/surface/bash_ast_test.go` (7 cases) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T28 | IPermissionGate.CheckPermission 消费 ToolSpec.OpenWorld 字段 (4 bool orthogonal flags) | Permission Gate | `internal/layers/orchestration/decisionplanning/plan_mode_test.go` (consume spec flags) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T29 | turn_adapter.ExecuteRound dispatch 前 CheckPermission；PlanModeOpenWorldPolicy 在 plan_mode + OpenWorld + not-allowlist 时 Deny | Two-phase Dispatch | `internal/bootstrap/turn_adapter_surface_test.go` + `internal/layers/orchestration/decisionplanning/plan_mode_test.go::TestPlanModeOpenWorldPolicy` | IMPLEMENTED | P0 |

## PERMISSION-GATE-1: Permission Gate Policy (DM-20260618-002)

> **PERMISSION-GATE-1 域（P0, 2026-06-18 新建）。** 由 DM-20260618-002
> 跨切面注册。本域与 D7 orchestration permission 包共生。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|--------|-----------|--------|----------|
| PERMISSION-GATE-1-T01 | Permission policy 决策 path 单元测试 (Risk→Decision 映射 Low=Allow / Med+High=Ask / Critical=Deny) | Permission Policy | `internal/shared/contracts/permission_check_test.go` + `internal/layers/communication/capture/permission_test.go` | IMPLEMENTED | P0 |
| PERMISSION-GATE-1-T02 | Plan mode 自动 deny OpenWorld=true 的 tool（per-risk 收紧），除非 in allowlist (wildcard) | PlanMode Policy | `internal/layers/orchestration/decisionplanning/plan_mode_test.go::TestPlanMode_AllowListBypassesDeny` | IMPLEMENTED | P0 |

## TOOL-SURFACE-1: Lazy Loading (DM-20260618-003)

> **devrix-surface-lazy-loading (DM-20260618-003) — 5 个 P0 T 点。**
> ToolSpec.DeferLoading 静态字段 + ToolFilter.ShouldDefer runtime hook +
> ToolSearchSurface (8th surface) + turn_adapter.Prepare 过滤 +
> zodgen (Go struct → JSON Schema subset)。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|--------|-----------|--------|----------|
| TOOL-SURFACE-1-T30 | zodgen.Schema() Go struct → JSON Schema subset (type/properties/required/enum/description) | zodgen | `internal/layers/contextengine/enforce/tools/zodgen/zodgen_test.go` (10 cases) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T31 | ShouldDeferByDefault 返回 true for 6 hardcoded candidates (delegate_*, task_output_background) | DeferLoading Static | `internal/layers/contextengine/enforce/tools/surface/tool_search_surface_test.go::TestShouldDeferByDefault` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T32 | ToolSearchSurface.Tools() 返回 1 个 spec (DeferLoading=false 强制)；search() 匹配 exact > glob > substring，top-5 cap | ToolSearchSurface | `internal/layers/contextengine/enforce/tools/surface/tool_search_surface_test.go` (6 cases) | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T33 | turn_adapter.Prepare 过滤 DeferLoading=true 的 tools (tool_search 必须保留)；deferDecider chain 加 runtime defer | Prepare Filter | `internal/bootstrap/turn_adapter_surface_test.go::TestPrepare_FiltersDeferred` | IMPLEMENTED | P0 |
| TOOL-SURFACE-1-T34 | PlanModeOpenWorldPolicy.ShouldDefer runtime defer (mode=plan_mode + OpenWorld + !allowlist → defer) | ShouldDefer Runtime | `internal/layers/orchestration/decisionplanning/plan_mode_test.go::TestPlanModeOpenWorldPolicy_ShouldDefer` | IMPLEMENTED | P0 |

## TOOL-SURFACE-1: ask_user_question (DM-20260618-006)

> **devrix-ask-user-question (DM-20260618-006) — 4 个 P1 T 点。**
> 新增 AskUserQuestionSurface（9th surface，stateless），让 LLM
> 通过 IM 通道向用户主动提 1-4 个多选问题。Sender 桥接到
> CommunicationGateway.RouteOutbound；OrthogonalFlagFor +
> InterruptBehaviorFor 登记。BuildSurfaces 装配并保持字典序稳定。
> **范围外**：task_* 任务管理工具（v2 workmodel 既有覆盖，
> 本次 hotfix 发现 → 0 新增代码达成 B 部分需求）。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|--------|-----------|--------|----------|
| TOOL-SURFACE-1-T35 | AskUserQuestionSurface.Tools() 1 spec + OrthogonalFlagFor 返回 (ReadOnly=T, OpenWorld=T, ConcurrencySafe=F) + InterruptBehavior=InterruptCancel | ask_user_question Spec | `internal/layers/contextengine/enforce/tools/surface/ask_user_question_surface_test.go::TestAskUserQuestionSurface_Tools` | IMPLEMENTED | P1 |
| TOOL-SURFACE-1-T36 | 5 项 validation: 1-4 questions / 2-4 options / header ≤ 12 / unique label / non-empty label-question | ask_user_question Validation | `ask_user_question_surface_test.go::TestAskUserQuestionSurface_Execute_{EmptyQuestions,TooManyQuestions,OptionLabelRequired,DuplicateLabels,HeaderCap}` | IMPLEMENTED | P1 |
| TOOL-SURFACE-1-T37 | sender 桥接: 装配 → gw.RouteOutbound；无 sender → Delivered=false graceful；sender 错 → ToolResult.Error | ask_user_question Sender | `ask_user_question_surface_test.go::TestAskUserQuestionSurface_Execute_{HappyPath,NoSender,SenderError,RenderMultiple}` | IMPLEMENTED | P1 |
| TOOL-SURFACE-1-T38 | BuildSurfaces 装配 ask_user_question surface + 排序稳定 (3 套 opts 一致)；main.go 装配 sender；启动 0 错误 | ask_user_question Wiring | `internal/bootstrap/surfaces_test.go::TestBuildSurfaces_OnlyStateless` + `cmd/devrix/main.go` startup log | IMPLEMENTED | P1 |

## D2: Cross-Scenario Tests

| T ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| D2-S0-A01-T01 | 压缩/Verify 步骤可观事务 | `tests/integration/context_compression_obs_test.go` | IMPLEMENTED |
| D2-S0-A01-T02 | 主路径接入真实 LLM Gateway | `tests/integration/context_llm_gateway_test.go` | IMPLEMENTED |
| D2-S0-A01-T03 | plan.enabled=false 时回退 V2 路径 | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED | (历史 PEV 回归，legacy) |

---

## DM-020 Legacy T 映射（v1.0 Registry）

> **DM-020（D7 Turn 编排上移）：** D2-S16 Legacy Freeze。v1.0 **不修改**现有测试 `// T:` 注释。下表供追溯与新测试登记。

| Legacy T ID | Canonical T ID | Canonical S | 域 | 描述 |
|-------------|----------------|-------------|-----|------|
| D2-S16-A01-T01 | D7-S2-A06-T01 | S2 Turn | D7 | FastPath turn D2 then D3 |
| D2-S16-A01-T02 | D7-S2-A06-T02 | S2 Turn cancel | D7 | Cancel propagates |
| D2-S16-A01-T03 | D2-THIN-T01 | import lint | D2 | D2→D3 import 禁止（v2.0-d CI 硬阻断） |
| D2-S10-A01-T34 | D7-S2-A06-T03 | multi-turn loop | D7 | Multi-turn tool_use |
| D2-S10-A01-T35 | D2-S15-A01-T* | S15 Prepare | D2 | UserContext prepend |
| D2-S10-A01-T36 | D2-S18-A01-T* | S18 Policy | D2 | plan_mode attachment throttle |
| D2-S10-A01-T37 | D2-S18-A01-T* | S18 Policy | D2 | plan mode write deny |
| D2-S10-A01-T40 | D2-S18-A02-T* | S18 Enforce | D2 | SubQuery Explore read-only |
| D2-S10-A01-T41 | D2-S15-A04-T* | S15 Prepare | D2 | Fork subagent placeholder |
| D2-S10-A01-T42 | D2-S18-A02-T* | S18 Enforce | D2 | Sidechain transcript resume |
| （新增） | D2-S15-A01-T10 | S15 Prepare | D2 | CompressHint no LLM（D2 不调 D3 摘要） |
| D2-DIAG-T01 | D2-S23-A01-T01 | S23 LSP Tool | D2 | LSP `definition` operation 返回 location | `internal/layers/contextengine/enforce/tools/lsp_tool_test.go` | IMPLEMENTED | P0 |
| D2-DIAG-T02 | D2-S23-A01-T02 | S23 LSP Tool | D2 | LSP `references` operation 返回引用列表 | `internal/layers/contextengine/enforce/tools/lsp_tool_test.go` | IMPLEMENTED | P0 |
| D2-DIAG-T03 | D2-S23-A01-T03 | S23 LSP Tool | D2 | LSP `incoming_calls` 返回 call hierarchy | `internal/layers/contextengine/enforce/tools/lsp_tool_test.go` | IMPLEMENTED | P1 |
| D2-DIAG-T04 | D2-S23-A02-T01 | S23 WindowAnalyzer | D2 | WindowAnalyzer 按 5 类拆分 token | `internal/layers/contextengine/prepare/token/windowanalyzer/analyzer_test.go` | IMPLEMENTED | P0 |
| D2-DIAG-T05 | D2-S23-A02-T02 | S23 WindowAnalyzer | D2 | WindowAnalyzer role 路由（system/tool/thinking/reminder） | `internal/layers/contextengine/prepare/token/windowanalyzer/analyzer_test.go` | IMPLEMENTED | P0 |
| D2-SEC-T01 | TS-AST-T01 | tool-security AST | shared | Bash AST 阻止 heredoc 注入 | `internal/layers/contextengine/enforce/tools/sandboxast/analyzer_test.go` | IMPLEMENTED | P0 |
| D2-SEC-T02 | TS-AST-T02 | tool-security AST | shared | Bash AST 阻止 zsh attack surface | `internal/layers/contextengine/enforce/tools/sandboxast/analyzer_test.go` | IMPLEMENTED | P0 |
| D2-SEC-T03 | TS-AST-T03 | tool-security AST | shared | Bash AST 阻止 process/command substitution | `internal/layers/contextengine/enforce/tools/sandboxast/analyzer_test.go` | IMPLEMENTED | P0 |
| D2-SEC-T04 | TS-AST-T04 | tool-security AST | shared | Bash AST 阻止 dangerous redirect (`>/dev/sda`) | `internal/layers/contextengine/enforce/tools/sandboxast/analyzer_test.go` | IMPLEMENTED | P0 |
| D2-SEC-T05 | TS-AST-T05 | tool-security AST | shared | Bash AST 阻止 eval/source/exec/`.` | `internal/layers/contextengine/enforce/tools/sandboxast/analyzer_test.go` | IMPLEMENTED | P0 |

---

## Statistics

| Total | IMPLEMENTED | PARTIAL | P0 |
|-------|-------------|---------|-----|
| 114 | 114 | 0 | 61 |

> TOOL-SURFACE-1 阶段 1（W1-W9）新增 11 项 P0/P1 测试点（73 - 62 = 11）。
> TOOL-SURFACE-1 阶段 2（DM-20260617-008 W1-W5）新增 7 项 P0 测试点 T15-T21（80 - 73 = 7），全部 IMPLEMENTED。
> **TOOL-SURFACE-1 v2 (DM-20260618-001) 新增 4 项 P0 T22-T25（84 - 80 = 4）**
> **TOOL-SURFACE-1 v3 (DM-20260618-002) 新增 4 项 P0 T26-T29（88 - 84 = 4）**
> **PERMISSION-GATE-1 (DM-20260618-002) 新增 2 项 P0 T01-T02（90 - 88 = 2）**
> **TOOL-SURFACE-1 Lazy Loading (DM-20260618-003) 新增 5 项 P0 T30-T34（95 - 90 = 5）**
> **D2-STRUCT-T01 (DM-20260619-007) 新增 1 项 P0 layout 守卫（96 - 95 = 1）**
> **DM-20260620-001 (devrix-context-budget-and-isolation, Phase A) 新增 13 项 P0 T 点 (109 - 96 = 13)**
> **DM-20260620-001-B (devrix-context-budget-and-isolation, Phase B) 新增 3 项 P0 T 点 (112 - 109 = 3)：D2-S15-A08-T06/T07/T08 BuildForkedMessages helper 边界**
> **DM-20260620-002 (devrix-context-budget-phase-c-nested, Phase C) 新增 2 项 P0 T 点 (114 - 112 = 2)：D2-S15-A08-T09/T10 SubTurnRequest / SubQueryParams 透传 MaxContextTokens**
> 全部 IMPLEMENTED。

---

## D2-STRUCT: Layout Guard (DM-20260619-007)

> **D2 v2.2 Structure 终态（devrix-d2-structure-closure）** — 防止目录漂移回到 Pre-v2.2 形态。

| T ID | 描述 | S 映射 | Test 位置 | Status |
|------|------|--------|-----------|--------|
| **D2-STRUCT-T01** | 根目录生产文件仅 `contracts.go` + `aliases.go`（`tool_context.go` 核心逻辑迁出后保留 type alias） | Structure | `internal/lint/layer/d2_layout_test.go` | **IMPLEMENTED** | P0 |
| **D2-STRUCT-T02** | 无 `engine_persist.go` 在根或 `facade/` 外（v2.2 后功能归属 `persist/commit.go`） | Structure | `internal/lint/layer/d2_layout_test.go` | **IMPLEMENTED** | P0 |
| **D2-STRUCT-T03** | `enforce/tools/` 包名为 `package tools`，禁止 `package toolrunner` 残留 | Structure | `internal/lint/layer/d2_layout_test.go` | **IMPLEMENTED** | P0 |
| **D2-STRUCT-T04** | `prepare/memory/` 与 `persist/memory/` 无循环依赖（Recall 与 Store 接口解耦） | Structure | `internal/lint/layer/d2_layout_test.go` | **IMPLEMENTED** | P0 |
| **D2-STRUCT-T05** | `enforce/orchestrator.go` 已删除（stub 移除，dispatch 由 `turn_adapter` 接管） | Structure | `internal/lint/layer/d2_layout_test.go` | **IMPLEMENTED** | P0 |
| **D2-STRUCT-T06** | scenario 下目录深度 ≤2 层（`enforce/tools/surface/` ✅；更深需 F-registry 登记） | Structure | `internal/lint/layer/d2_layout_test.go` | **IMPLEMENTED** | P0 |
| **D2-STRUCT-T07** | P5 legacy 退役：禁止新增 `legacy.ContextEngine.Process()` 生产引用（CI 硬阻断）；现有 8 个 caller 在 allowlist（cmd/llm-smoke + multiagent/run + tests/* + communication mocks） | Structure | `internal/lint/layer/d2_layout_test.go` | **IMPLEMENTED** | P1 |

> 全部由 `internal/lint/layer/d2_layout_test.go` 单一守卫测试驱动。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 2.7.0 | 2026-06-19 | devrix-d2-structure-closure (DM-20260619-007) 归档：D2-STRUCT-T01..T07 layout guards |
| 2.8.0 | 2026-06-20 | DM-20260620-001 Phase A 归档：D2-S17-A05 T01-T05 (tool result cap) + D2-S17-A06 T01-T03 (assistant fold) + D2-S15-A08 T01-T05 (token audit + proactive fold)。IMPLEMENTED 96→109, P0 56→56 |
| 2.8.1 | 2026-06-20 | DM-20260620-001-B Phase B 归档：D2-S15-A08 T06-T08 (BuildForkedMessages helpers)。IMPLEMENTED 109→112, P0 56→59 |
| **2.9.0** | **2026-06-20** | **DM-20260620-002 Phase C 归档**：D2-S15-A08 T09-T10 (SubTurnRequest/SubQueryParams MaxContextTokens 透传)。IMPLEMENTED 112→114, P0 59→61 |
| **2.10.0** | **2026-06-29** | **DM-20260629-002 PR-4 — registry-sync**: F ID D2-S1..S5 → D2-S15..S18 canonical 重映射 + Historical Appendix tombstone S1/S5/S9/S10/S19/S20; F path 9 修正 (engine_persist.go → kernel/context_engine_commit_window_adapter.go, enforce/background.go → enforce/registry.go, etc.) |
| **2.11.0** | **2026-06-29** | **DM-20260629-002 PR-5 — value-flow-rename**: §Canonical T 映射 加 ValueFlow Alias block（S15/S17/S18 D2_* Alias + S16 归 D7）；与 d2-domain.md / a-registry / f-registry §North Star 对齐 |
