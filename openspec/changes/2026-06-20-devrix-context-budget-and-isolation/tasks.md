# Tasks: Context Budget & Isolation

**Change ID:** `2026-06-20-devrix-context-budget-and-isolation`
**Demand ID:** DM-20260620-001
**Status:** S5_Accepted (Phase A)

---

## Phase A.1 — Tool result size cap + 落盘 (AC1)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| A1.1 | `ToolResultStore` struct + `Persist` / `List` / `GC` | `contextengine/prepare/persist/tool_result_store.go` | ~140 |
| A1.2 | `ShouldCap(toolName)` 白名单 + 单测 | 同上 | ~30 |
| A1.3 | turn loop 工具结果构造点接入 size cap | `orchestration/turn/llm.go` | ~40 |
| A1.4 | 单元测试：白名单/非白名单、超限/未超限、Persist 失败 fallback | `persist/tool_result_store_test.go` | ~200 |

**Quality Gate:**
- [ ] `go test ./internal/layers/contextengine/prepare/persist/...` 全绿
- [ ] `go test ./internal/layers/orchestration/turn/...` 全绿
- [ ] AC1 满足（tool audit 日志 100% 命中白名单）
- [ ] 落盘文件可读、内容完整

**建议 PR:** `feat/context-budget-tool-result-cap`

---

## Phase A.2 — Assistant output fold (AC2)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| A2.1 | `FoldAssistantOutput` 函数（head + tail + 中段占位） | `contextengine/prepare/persist/turn_output_store.go` | ~120 |
| A2.2 | turn loop iteration 末尾接入 fold | `orchestration/turn/orchestrator.go` | ~50 |
| A2.3 | 单元测试：超限/未超限、unicode 边界、persist 失败 fallback | `persist/turn_output_store_test.go` | ~180 |

**Quality Gate:**
- [ ] `go test ./internal/layers/contextengine/prepare/persist/...` 全绿
- [ ] AC2 满足（per-iteration token count log）

**建议 PR:** `feat/context-budget-assistant-fold`

---

## Phase A.3 — Per-iter Prepare + token audit (AC3 + AC4)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| A3.1 | `AuditMessages` + `ShouldFoldProactively` | `contextengine/prepare/audit/token_audit.go` | ~120 |
| A3.2 | turn loop `for { ... }` 内每轮调 Prepare | `orchestration/turn/orchestrator.go` | ~30 |
| A3.3 | turn loop 接入 proactive fold（基于 audit） | 同上 | ~30 |
| A3.4 | `Counter.TruncateToTokens` 从 dead-code 升级为引用 | `contextengine/prepare/token/counter.go` | ~10 |
| A3.5 | 单元测试：audit 边界（60% / 80% / 100%）、Prepare 失败 fallback | `audit/token_audit_test.go` | ~180 |
| A3.6 | 集成测试：22 步任务 P95 < 50ms 增加 | `tests/integration/turn_loop_budget_test.go` | ~120 |

**Quality Gate:**
- [ ] `go test ./...` 全绿
- [ ] `go vet ./...` 无错
- [ ] AC3 + AC4 + AC13 满足
- [ ] 22 步任务 benchmark P95 < 50ms 增加

**建议 PR:** `feat/context-budget-per-iter-prepare`

---

## Phase A.4 — Feishu card precheck (AC5)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| A4.1 | `CardContentPrecheck` interface + `ErrTooManyTables` | `communication/sender/card_precheck.go` | ~60 |
| A4.2 | `FeishuTableCountPrecheck` 实现 | `communication/feishu/feishu_table_precheck.go` | ~50 |
| A4.3 | feishu sendCard 接入 precheck + 降级路径 | `communication/feishu/send.go` | ~30 |
| A4.4 | 单元测试：<table> 计数、单/多表、超限/未超限 | `card_precheck_test.go` + `feishu_table_precheck_test.go` | ~150 |
| A4.5 | 集成测试：D5 spans 原 prompt 触发降级 | `tests/integration/feishu_card_precheck_test.go` | ~80 |

**Quality Gate:**
- [ ] `go test ./internal/layers/communication/...` 全绿
- [ ] AC5 满足（feishu ERROR 0 命中"D5 spans 任务"）
- [ ] 降级路径 plain text 输出完整

**建议 PR:** `feat/feishu-card-precheck`（**紧急 PR**，独立合入）

---

## Phase A.5 — Counter 升级 (AC13)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| A5.1 | `Counter.TruncateToTokens` 移除 `_ = TruncateToTokens` dead-code 标记 | `contextengine/prepare/token/counter.go` | ~10 |
| A5.2 | godoc 注释引用 A1/A2 调用点 | 同上 | ~10 |

**Quality Gate:**
- [ ] `go vet ./...` 无错
- [ ] godoc 链接正确

**建议 PR:** 合并到 A1 或 A3 任意一个 PR（独立 commit）

---

## Phase B.1 — SubTurnRunner Mode + Depth (AC6 + AC8 + AC9)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| B1.1 | `SubTurnMode` type + 3 个常量 | `orchestration/turn/contracts.go` | ~20 |
| B1.2 | `SubTurnRequest` 新增 `Mode` / `Depth` 字段 | 同上 | ~10 |
| B1.3 | `ErrSubagentDepthExceeded` error | `shared/contracts/contracts.go` | ~5 |
| B1.4 | `SubTurnRunner.SetMaxDepth` + 新字段 | `orchestration/turn/subturn.go` | ~20 |
| B1.5 | `SubTurnRunner.RunSubTurn` switch 3 mode | 同上 | ~50 |
| B1.6 | 默认 mode 从 `devrix.yaml` 读取 + `legacy_mode` fallback | 同上 + 配置加载 | ~40 |
| B1.7 | 单元测试：3 mode × 3 depth 边界 | `subturn_test.go` | ~250 |

**Quality Gate:**
- [ ] `go test ./internal/layers/orchestration/turn/...` 全绿
- [ ] AC6 + AC8 + AC9 满足
- [ ] LLM 日志 sub-agent first call prompt_tokens ≤ 3K (mode=brief 默认)

**建议 PR:** `feat/subagent-mode-and-depth`（**默认 brief，加 legacy_mode 兼容开关**）

---

## Phase B.2 — Tool schema 暴露 mode (AC10)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| B2.1 | `delegate` tool schema 增加 `mode` 字段 | `multiagent/delegatetools/delegate_tools.go` | ~20 |
| B2.2 | `free_fork` tool schema 增加 `mode` 字段（如有） | `multiagent/delegatetools/free_fork_tools.go` | ~20 |
| B2.3 | 工具执行时 Mode 缺省从 `devrix.yaml` 读取 | 同上 | ~30 |
| B2.4 | 单元测试：schema dump、mode 缺省、mode 显式 | `delegate_tools_test.go` | ~120 |

**Quality Gate:**
- [ ] `go test ./internal/layers/multiagent/...` 全绿
- [ ] AC10 满足（tool schema json dump 验证）

**建议 PR:** `feat/subagent-tool-mode-schema`

---

## Phase B.3 — Fork mode + cache anchor (AC7 + AC11)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| B3.1 | `buildForkedMessages` 函数 + `ForkedMessages` struct | `orchestration/turn/fork_messages.go` | ~150 |
| B3.2 | 解析 parent 最后 assistant message 的 tool_use blocks | 同上 | ~50 |
| B3.3 | 构造占位 tool_result + directive user msg | 同上 | ~40 |
| B3.4 | `SubTurnRunner` mode=fork 分支接入 `buildForkedMessages` | `subturn.go` | ~30 |
| B3.5 | `ContentBlock.CacheControl` + `buildSystemPromptWithCacheAnchor` | `llmgateway/message.go` | ~80 |
| B3.6 | D2 `AssemblerAdapter` system prompt 注入 cache anchor | `contextengine/prepare/adapters/assembler_adapter.go` | ~30 |
| B3.7 | 单元测试：prefix 字节级稳定、cache anchor 存在、tool_result 占位 | `fork_messages_test.go` | ~250 |
| B3.8 | 集成测试：fork 兄弟子 agent 共享 prefix | `tests/integration/subagent_mode_test.go` | ~120 |

**Quality Gate:**
- [ ] `go test ./internal/layers/orchestration/turn/...` 全绿
- [ ] `go test ./internal/layers/llmgateway/...` 全绿
- [ ] `go test ./internal/layers/contextengine/prepare/...` 全绿
- [ ] AC7 满足（fork 子 agent prefix 字节级稳定）
- [ ] AC11 满足（minimax 支持）或降级（不支持则 AC11a + 移除 AC11b）

**建议 PR:** `feat/subagent-fork-mode-and-cache-anchor`

---

## Phase B.4 — mode=full 显式声明 (AC8)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| B4.1 | `legacy_mode: full` 配置文档化 | `devrix.yaml` schema + README | ~30 |
| B4.2 | release note 预告下个 minor release 移除 `legacy_mode` | `docs/release-notes/vNEXT.md` | ~10 |

**Quality Gate:**
- [ ] 文档完整

**建议 PR:** 合并到 B.1 同 PR（独立 commit）

---

## 回归验证 — D5 spans 复跑 (AC12)

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| R.1 | 保存 D5 spans 原 prompt + 用户输入为 fixture | `tests/fixtures/d5-spans-replay.jsonl` | ~80 |
| R.2 | 脚本化重跑 + 收集 `prompt_tokens` 序列 | `tests/integration/d5_replay/main.go` | ~150 |
| R.3 | 对比 P95 ≤ 40K + feishu ERROR = 0 | `tests/integration/d5_replay/assert.go` | ~60 |
| R.4 | benchmark artifact 输出（22 步 token 增长曲线） | `coverage-reports/context-budget/d5-spans-bench.json` | 自动 |

**Quality Gate:**
- [ ] 复跑结果：22 步后 `prompt_tokens` P95 ≤ 40K
- [ ] 复跑结果：feishu ERROR 计数 = 0
- [ ] AC12 满足

**建议 PR:** `test/d5-spans-context-budget-replay`（**验证 PR，依赖 Phase A+B 全部合入**）

---

## 实施顺序汇总

```
PR #1 (A1)     feat/context-budget-tool-result-cap       — AC1
PR #2 (A2)     feat/context-budget-assistant-fold        — AC2
PR #3 (A3)     feat/context-budget-per-iter-prepare      — AC3 + AC4 + AC13
PR #4 (A4)     feat/feishu-card-precheck                 — AC5（紧急）
PR #5 (B1+B4)  feat/subagent-mode-and-depth              — AC6 + AC8 + AC9
PR #6 (B2)     feat/subagent-tool-mode-schema            — AC10
PR #7 (B3)     feat/subagent-fork-mode-and-cache-anchor  — AC7 + AC11
PR #8 (R)      test/d5-spans-context-budget-replay       — AC12
```

## 跨 PR 依赖图

```
PR #1 ──┐
PR #2 ──┼──→ PR #3 ──┐
PR #4 ──┘            │
                     ├──→ PR #5 ──┐
                     │            ├──→ PR #6 ──┐
                     │            │            ├──→ PR #7 ──→ PR #8
                     └────────────┴────────────┘
```

PR #1、#2、#4 可并行；PR #3 依赖 #1 + #2；PR #5 独立；PR #6 依赖 #5；PR #7 依赖 #5 + #6；PR #8 依赖全部。

## 总估行（不含测试）

| Phase | 新增代码 | 修改代码 | 测试代码 |
|-------|---------|---------|---------|
| A.1 | 140 | 40 | 200 |
| A.2 | 120 | 50 | 180 |
| A.3 | 120 | 70 | 300 |
| A.4 | 110 | 30 | 230 |
| B.1 | 60 | 85 | 250 |
| B.2 | 0 | 70 | 120 |
| B.3 | 230 | 130 | 370 |
| B.4 | 0 | 40 | 0 |
| **合计** | **780** | **515** | **1650** |

总计约 **2945 行**（含测试）。