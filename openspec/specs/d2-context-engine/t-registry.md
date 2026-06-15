# D2 Context Engine Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-15
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `openspec/specs/d2-context-engine/d2-domain.md`

---

## Canonical T 映射（DM-20260614-009）

v1.0：**不修改**现有测试 `// T:` 注释。下表供追溯与新测试登记。

| Canonical T ID | Legacy T ID | Canonical S | 描述 | Status |
|----------------|-------------|-------------|------|--------|
| D2-S15-A01-T01 | D2-S3-A01-T01 | S15 | 新会话历史正确追加 | IMPLEMENTED |
| D2-S15-A02-T01 | D2-S13-A01-T01 | S15 | RepairToolChain 修复 orphan | PLANNED |
| D2-S15-A03-T01 | D2-S2-A01-T01 | S15 | 超阈值触发压缩 | IMPLEMENTED |
| D2-S16-A01-T01 | D2-S10-A01-T34 | S16 | Multi-turn tool loop | IMPLEMENTED |
| D2-S16-A01-T02 | D2-CTX-T01 | S16 | Process cancel 无 panic | IMPLEMENTED |
| D2-S16-A01-T03 | — | S16 | query 包无 D4/D7 import | IMPLEMENTED | `internal/lint/layer/d2_thin_test.go` |
| D2-S17-A01-T01 | D2-S3-A01-T02 | S17 | Deferred complete 后快照 | IMPLEMENTED |
| D2-S17-A02-T01 | D2-S6-A02-T01 | S17 | Main transcript append | IMPLEMENTED |
| D2-S18-A01-T01 | D2-CTX-T36 | S18 | Plan mode write deny | IMPLEMENTED |
| D2-S18-A02-T01 | D2-S8-A01-T01 | S18 | Bash sandbox workdir | IMPLEMENTED |
| D2-S19-A01-T01 | D2-CTX-T40 | S19 | Explore read-only | IMPLEMENTED |
| D2-S19-A02-T01 | D2-CTX-T41 | S19 | Fork identical prefix | IMPLEMENTED |
| D2-S20-A01-T01 | D2-S11-A01-T02 | S20 | 默认跳过 harness | IMPLEMENTED |
| D2-S20-A02-T01 | D2-S9-A01-T01 | S20 | Legacy bootstrap 一次 | IMPLEMENTED |

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

## D2-S8: Sandbox Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S8-A01-T01 | bash 困居工作目录 + 命令白名单 | Sandbox | `internal/layers/contextengine/policy/toolrunner/sandbox_test.go` | IMPLEMENTED |
| D2-S8-A01-T02 | Shell injection attack prevention | Sandbox | `tests/security/shell_injection_test.go` | IMPLEMENTED |

## D2-S9: Harness Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S9-A01-T01 | harness.enabled 首次 Process 触发 Bootstrap | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T02 | WorkspaceContext 扫描 Go 文件与 AGENTS.md | Harness | `internal/layers/contextengine/harness/workspace_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T03 | Bootstrap 幂等（同 Session 不重复） | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T04 | trusted=false 时 deferred_init 标志全 false | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A03-T05 | ToolPool simple_mode / MCP / deny 过滤 | Harness | `internal/layers/contextengine/harness/toolpool_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T06 | PromptRouter advisory 关键词计分 | Harness | `internal/layers/contextengine/harness/router_test.go` | IMPLEMENTED | P2 |
| D2-S9-A01-T07 | Transcript 内存分离与 compact | Harness | `internal/layers/contextengine/harness/transcript_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T08 | harness.enabled=false V4 回归 + 无 bootstrap info | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T09 | Preflight warn-only 规则评分与 tool filter | Harness | `internal/layers/contextengine/harness/preflight_test.go` | IMPLEMENTED | P1 |
| D2-S9-A02-T10 | System Prompt Assembly §十 XML 块 | Harness | `internal/layers/contextengine/prepare/prompt/assembler_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T11 | Jaeger span 树（enabled/disabled） | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |
| D2-S9-A02-T12 | disabled 与 BuildLegacy 字节级一致 | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A02-T13 | CompressedView system = Build 输出 | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A03-T14 | QueryLoop 可见工具 ⊆ VisibleTools | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T15 | bootstrap.stage parent = bootstrap.run | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |

## D2-S9.BG: Background SubQuery Task Tools

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S9-A01-T16 | stop running task → cancelled (idempotent) | BGTask | `internal/layers/contextengine/nested/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T17 | output block=false 返回 running 状态 + partial result | BGTask | `internal/layers/contextengine/nested/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T18 | output block=true 阻塞至 terminal 或 timeout（max 600s） | BGTask | `internal/layers/contextengine/nested/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T19 | cancel 后 SessionQueue 不发 completed notification（tombstone 协议） | BGTask | `internal/layers/contextengine/nested/background_cancel_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T20 | IsTerminal 对 running/cancelled/completed/failed 正确报告（Phase 3 Wave WorkerCancelRegistry） | BGTask | `internal/layers/contextengine/nested/background_cancel_test.go` | IMPLEMENTED | P1 |

## D2-S10: QueryLoop Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S10-A01-T34 | 多轮 tool_use 直至无 tool | QueryLoop | `internal/layers/contextengine/query/loop_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T35 | UserContext prepend 不在 snapshot | QueryLoop | `internal/layers/contextengine/usercontext/` | IMPLEMENTED | P0 |
| D2-S10-A01-T36 | plan_mode attachment full/sparse throttle | QueryLoop | `internal/layers/contextengine/attachments/registry.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T37 | plan mode 拒绝 Write 非 plan 文件 | QueryLoop | `internal/layers/contextengine/policy/permission/mode_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T38 | task_create 磁盘持久 + list 一致 | QueryLoop | `internal/layers/contextengine/tasks/disk_store_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T39 | query_loop.enabled=false V4 回归 | QueryLoop | `tests/integration/query_loop_integration_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T40 | SubQuery Explore omitClaudeMd + read-only | QueryLoop | `internal/layers/contextengine/nested/subquery_test.go` | IMPLEMENTED | P1 |
| D2-S10-A01-T41 | Fork subagent placeholder tool_results 一致 | QueryLoop | `internal/layers/contextengine/nested/fork_test.go` | IMPLEMENTED | P1 |
| D2-S10-A01-T42 | sidechain transcript resume 重建 messages | QueryLoop | `internal/layers/contextengine/persist/transcript/sidechain_test.go` | IMPLEMENTED | P1 |

## D2-S11: Harness Unification

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S11-A01-T01 | `query_loop.enabled` 默认 true | HarnessUnification | `internal/shared/config/queryloop_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T02 | harnessEnabled 分支不再被生产路径触发 | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T03 | 旧路径调用计数基线=0 | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T04 | 压缩入口统一：QueryLoop 走 messages-only 七步管道 | HarnessUnification | `internal/layers/contextengine/compression_unified_test.go` | IMPLEMENTED | P1 |
| D2-S11-A01-TD01 | TD-QL-01: 413 → 一轮 messages-only 压缩 → 重试 | HarnessUnification | `internal/layers/contextengine/query/loop_recovery_test.go` | IMPLEMENTED | P1 |
| D2-S11-A01-TD03 | TD-QL-03: overload/5xx → 切换 fallback model (生产未接线) | HarnessUnification | `internal/layers/contextengine/query/loop_fallback_test.go` | PARTIAL | P1 |
| D2-S11-A01-D6PR | D6 PathRegressionProbe: legacy_harness > 0 ⇒ score 0 | HarnessUnification | `internal/layers/evolution/eval/path_regression_probe_test.go` | IMPLEMENTED | P0 |

## D2-S12: Worktree Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S12-A01-T01 | Worktree enter 后 write 不污染主 WorkDir | Worktree | `internal/layers/contextengine/worktree/manager_test.go` | IMPLEMENTED | P0 |

## D2-S6: Snapshot & Main Transcript

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S6-A02-T01 | Main transcript append-only JSONL 读写 | Transcript | `internal/layers/contextengine/persist/transcript/main_thread_test.go` | IMPLEMENTED | P1 |

## D2-S13: Conversation Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S13-A01-T01 | RepairToolMessageChain 剔除 orphan tool results | Conversation | `internal/layers/contextengine/prepare/conversation/repair_test.go` | IMPLEMENTED | P0 |
| D2-S13-A02-T01 | MessagesAfterCompactBoundary 仅保留尾部 | Conversation | `internal/layers/contextengine/prepare/conversation/boundary_test.go` | IMPLEMENTED | P1 |

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
| D2-S10-A01-T40 | D2-S19-A01-T* | S19 Nested | D2 | SubQuery Explore read-only |
| D2-S10-A01-T41 | D2-S19-A01-T* | S19 Nested | D2 | Fork subagent placeholder |
| D2-S10-A01-T42 | D2-S19-A02-T* | S19 Nested | D2 | Sidechain transcript resume |
| （新增） | D2-S15-A01-T10 | S15 Prepare | D2 | CompressHint no LLM（D2 不调 D3 摘要） |

---

## Statistics

| Total | IMPLEMENTED | PARTIAL | P0 |
|-------|-------------|---------|-----|
| 60 | 58 | 1 | 18 |
