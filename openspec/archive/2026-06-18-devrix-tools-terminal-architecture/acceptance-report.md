# Acceptance Report — DM-20260618-007 devrix-tools-terminal-architecture

**Change ID:** devrix-tools-terminal-architecture
**Change Title:** 终端侧工具链架构 — 5 Surface × 跨切面 LTL-Lite 不变式规约
**Date:** 2026-06-18
**Author:** OMC Agent (S5 验收 + S6 归档驱动)
**Branch:** `feat/devrix-tools-terminal-architecture` (PR #76)
**Status:** ✅ S5_Accepted → 进入 S6 归档阶段

---

## 1. 27 AC 状态表

| AC | 类型 | 描述 | 状态 | 证据 |
|----|------|------|------|------|
| **AC1** | P0 | LSP 5 typed method spec 暴露 + Execute 路径 | ✅ PASS | `lsptool_surface.go` + `TestLSP_End2End` |
| **AC2** | P0 | BashAST fail-closed + 22+ zsh rules + heredoc 注入检测 | ✅ PASS | `sandboxast/analyzer.go` + `bash/policy.go` + `TestBashAST_DenyAttack` |
| **AC3** | P0 | Tracker diff/LRU/async 触发器 | ✅ PASS | `diagnose/tracker/tracker.go` + `TestTracker_NonBlocking` |
| **AC4** | P0 | FreeFork Forker + Worktree 隔离 + 3+ 分叉 | ✅ PASS | `provision/freefork/` + `TestFreeFork_3Directions` |
| **AC5** | P0 | Verify parser + executor + aggregator | ✅ PASS | `evolution/verify/plan.go` + `TestVerify_AllPass` |
| **AC6** | P0 | D5 SLO 指标 (LSPMethodLatency histogram) | ✅ PASS | `lsptool_surface.go` metrics 集成 |
| **AC7** | P0 | mvdan.cc/sh v3.x 锁定 + devrix.yaml LSP 配置 | ✅ PASS | `go.mod` + `devrix.yaml` |
| **AC8** | P1 | LSP 5 method goToDefinition/findReferences/incomingCalls/hover/workspaceSymbol | ✅ PASS | W1+W2 commit |
| **AC9** | P1 | BashAST 9 dangerous words + 23 zsh patterns + 嵌套 heredoc | ✅ PASS | W4 commit |
| **AC10** | P1 | BashAST fail-closed (parse err → Deny) | ✅ PASS | W4 commit + `TestParseFailureFailClosed` |
| **AC11** | P1 | Tracker LRU 上限 + dedup 跨文件 | ✅ PASS | W6 commit |
| **AC12** | P1 | Linter 路由 (.go → go vet) + WatchFile + TickOnce | ✅ PASS | W7 commit + `TestTracker_NonBlocking` |
| **AC13** | P1 | FreeFork Forker 并发 budget (maxChildren) | ✅ PASS | W8 commit |
| **AC14** | P1 | FreeFork 资源争抢仲裁 + FreeForkSurface 集成 | ✅ PASS | W9 commit |
| **AC15** | P1 | Worktree 隔离 (per-handle wt path) | ✅ PASS | W10 commit |
| **AC16** | P1 | Verify parser 兼容 \| W{N}.{M} \| 表格 + done/pending | ✅ PASS | W11 commit |
| **AC17** | P1 | Verify executor 5 evidence kind (file/test/cmd/api/...) | ✅ PASS | W11 commit |
| **AC18** | P1 | Verify aggregator (verified/unverified/skipped/summary) | ✅ PASS | W12 commit |
| **AC19** | P1 | BackgroundTaskSurface + ToolEventStream context 推送 | ✅ PASS | W13 commit + `tool_stream_test.go` |
| **AC20** | P1 | LTL-Lite ltllite/ parser + check + violation 框架 | ✅ PASS | W14 commit |
| **AC21** | P1 | 5 surface _invariant.go (LSP/Bash/Tracker/FreeFork/Verify) | ✅ PASS | W15 partial + W15 续 commit |
| **AC22** | P1 | ci-lint-invariant CLI tool + 跨 surface 冲突检测 | ✅ PASS | W15 续 commit + `tools/ci-lint-invariant/` |
| **AC23** | P1 | turn_adapter/ltl_hook HookRegistry Prepare/BeforeExecute | ✅ PASS | W15 续 commit + `turn_adapter/ltl_hook.go` |
| **AC24** | P1 | 25 个 T 点全部 PASS (回归) | ✅ PASS | W16 commit + 全量回归 |
| **AC25** | P0 | Surface 合并异质性门槛验证 (5 surface × 4 orthogonal flags) | ✅ PASS | W3 commit + `OrthogonalFlagFor` 矩阵 |
| **AC26** | P1 | 5-step IM E2E (lsp → bash → fork → tracker → verify) | ✅ PASS | W16 commit + `TestE2E_IMToolsTerminal_5Steps` |
| **AC27** | P1 | LTL-Lite CI lint step 通过 (5 文件 / 20 invariants) | ✅ PASS | W16 commit + ci-lint 验证 |

**统计:** 7 P0 + 20 P1 = 27 AC 全 PASS (100%)。

---

## 2. T 层验证（25 个 P0 T 点）

| T 点 ID | 描述 | 实施 W | 状态 | 覆盖率 |
|---------|------|-------|------|--------|
| D2-S4-A01-T01 | lsp_go_to_definition spec + Execute | W3 | ✅ | ≥ 85% |
| D2-S4-A01-T02 | lsp_find_references spec + Execute | W3 | ✅ | ≥ 85% |
| D2-S4-A01-T03 | lsp_incoming_calls spec + Execute | W3 | ✅ | ≥ 85% |
| D2-S4-A01-T04 | lsp_hover spec + Execute | W3 | ✅ | ≥ 85% |
| D2-S4-A01-T05 | lsp_workspace_symbol spec + Execute | W3 | ✅ | ≥ 85% |
| TOOL-SEC-2-A02-T01 | bash audit + policy decision | W4+W5 | ✅ | ≥ 90% |
| TOOL-SEC-2-A02-T02 | zsh attack pattern deny (22+ rules) | W5 | ✅ | ≥ 90% |
| TOOL-SEC-2-A02-T03 | heredoc injection detection | W4 | ✅ | ≥ 85% |
| D5-S23-A02-T01 | tracker tick → query | W6 | ✅ | ≥ 80% |
| D5-S23-A02-T02 | LRU dedup cross-file | W6 | ✅ | ≥ 85% |
| D5-S23-A02-T03 | linter 路由 + WatchFile 一致性 | W7 | ✅ | ≥ 85% |
| D4-S11-A02-T01 | Forker 并发 budget + WorkerContext | W8 | ✅ | ≥ 85% |
| D4-S11-A02-T02 | SendMessage 跨代理通信 | W8 | ✅ | ≥ 80% |
| D4-S11-A02-T03 | 资源争抢仲裁 | W9 | ✅ | ≥ 85% |
| D4-S11-A02-T04 | FreeForkSurface 集成 (3 分叉) | W9 | ✅ | ≥ 90% |
| D4-S13-A02-T01 | Worktree 隔离 per-handle | W10 | ✅ | ≥ 85% |
| D6-S11-A02-T01 | tasks.md 解析 + executor | W11 | ✅ | ≥ 85% |
| D6-S11-A02-T02 | evidence kind → checker 路由 (5 kind) | W11 | ✅ | ≥ 85% |
| D6-S11-A02-T03 | aggregator + report JSON | W12 | ✅ | ≥ 85% |
| D4-S12-A03-T01 | event 流推送 + BackgroundTaskSurface | W13 | ✅ | ≥ 80% |
| PERMISSION-GATE-1-T01 | LTL-Lite runtime check (ltllite.Check + HookRegistry) | W14+W15 续 | ✅ | ≥ 85% |
| PERMISSION-GATE-1-T02 | CI lint 静态校验 (ci-lint-invariant) | W15 续 | ✅ | ≥ 90% |
| PERMISSION-GATE-1-T03 | turn_adapter HookRegistry Prepare/BeforeExecute | W15 续 | ✅ | ≥ 85% |
| D5-S23-A02-T04 | tracker 非阻塞高频 tick (≤ 50ms/tick) | W7 | ✅ | ≥ 85% |
| D2-S4-A01-T06 | LSP surface 5 typed method spec 全暴露 (≥ 5 spec) | W3+W16 | ✅ | ≥ 90% |

**统计:** 25 个 T 点全 PASS，平均覆盖率 ≥ 85%。

---

## 3. 跨域一致性

| 检查 | 工具 | 结果 |
|------|------|------|
| D2-D3 import lint | `go run ./cmd/devrix-layer-lint ./...` | ✅ 0 violation |
| D2-D7 import lint | 同上 | ✅ 0 violation |
| 11 个限界上下文边界 | layer-lint 规则 | ✅ 全通过 |
| tools/ci-lint-invariant 跨 surface 冲突 | `go run ./tools/ci-lint-invariant` | ✅ 0 conflict |

---

## 4. 风险评估 — 5 类风险实际影响 vs 设计预期

| 风险 | 设计预期 | 实际影响 | 偏差评估 |
|------|---------|---------|---------|
| R1: LSP server 缺失导致 Execute 阻塞 | cfg.Enabled=false → 早返回 disabled 错误 | 0 阻塞，0 panic（`TestLSP_End2End` 验证） | ✅ 优于预期 |
| R2: BashAST 解析失败导致误 allow | fail-closed (Allow=false + Reason="AST parse failed") | `TestParseFailureFailClosed` 通过；含 parse error 时的所有命令 deny | ✅ 完全一致 |
| R3: Tracker 高频 tick 阻塞主流程 | LRU + 异步 TickOnce | `TestTracker_NonBlocking`: 100 tick + query 串行 < 1s | ✅ 优于预期 |
| R4: FreeFork 资源争抢 | WorkerContext budget + 仲裁 | `TestW8_10_FreeForkStack_T_CrossRef` 通过 | ✅ 完全一致 |
| R5: LTL-Lite 违规漏检 | ParseStruct reflect + Check State 全量评估 | `TestLTL_Violation_AbortTurn` 验证 wrapped ErrInvariantViolation | ✅ 完全一致 |

---

## 5. 文件交付清单

### 5.1 新增 (~30 文件)

| 路径 | W | 行数 |
|------|---|------|
| `internal/shared/ltllite/parser.go` | W14 | 105 |
| `internal/shared/ltllite/check.go` | W14 | 77 |
| `internal/shared/ltllite/parser_test.go` | W14 | 156 |
| `internal/shared/ltllite/check_test.go` | W14 | 109 |
| `internal/layers/contextengine/enforce/toolrunner/surface/lsp_surface_invariant.go` | W15 | 49 |
| `internal/layers/contextengine/enforce/toolrunner/surface/lsp_surface_invariant_test.go` | W15 | 60 |
| `internal/layers/contextengine/enforce/toolrunner/surface/bash_surface_invariant.go` | W15 续 | 36 |
| `internal/layers/observability/diagnose/tracker/_invariant.go` | W15 续 | 36 |
| `internal/layers/multiagent/provision/freefork/_invariant.go` | W15 续 | 36 |
| `internal/layers/evolution/verify/_invariant.go` | W15 续 | 32 |
| `internal/layers/orchestration/turn_adapter/ltl_hook.go` | W15 续 | 135 |
| `internal/layers/orchestration/turn_adapter/ltl_hook_test.go` | W15 续 | 175 |
| `tools/ci-lint-invariant/main.go` | W15 续 | 290 |
| `tools/ci-lint-invariant/main_test.go` | W15 续 | 168 |
| `tests/integration/tools_terminal_test.go` | W16 | 250 |
| `tests/e2e/im_tools_terminal_test.go` | W16 | 145 |

### 5.2 修改 (~10 文件)

| 路径 | W | 增量 |
|------|---|------|
| `internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer.go` | W4 | +200 行 (Severity/Rule/Finding/22+ zsh patterns) |
| `internal/layers/contextengine/enforce/toolrunner/surface/lsptool_surface.go` | W3 | +400 行 (5 typed method + metrics) |
| `internal/layers/contextengine/enforce/toolrunner/surface/bash_ast.go` | W5 | +30 行 (bashPolicy 注入) |
| `internal/layers/contextengine/enforce/toolrunner/bash/parser.go` | W5 | 30 行 (新增) |
| `internal/layers/contextengine/enforce/toolrunner/bash/heredoc.go` | W5 | 70 行 (新增) |
| `internal/layers/contextengine/enforce/toolrunner/bash/zsh_rules.go` | W5 | 80 行 (新增) |
| `internal/layers/contextengine/enforce/toolrunner/bash/policy.go` | W5 | 90 行 (新增) |
| `internal/layers/orchestration/turn/tool_stream_test.go` | W13 | +150 行 (新增) |
| `internal/layers/observability/diagnose/tracker/tracker_test.go` | W6/W7 | +80 行 (cross-ref) |
| `internal/layers/multiagent/provision/freefork/forker_test.go` | W8-W10 | +60 行 (cross-ref) |
| `internal/layers/evolution/verify/plan_test.go` | W11-W12 | +50 行 (cross-ref) |

---

## 6. S5 验收结论

✅ **所有 27 个 AC 通过**（7 P0 + 20 P1）
✅ **所有 25 个 T 点 PASS**（覆盖率 ≥ 80%，平均 ≥ 85%）
✅ **跨域一致性 0 violation**（layer-lint）
✅ **5 类风险实际影响 ≤ 设计预期**
✅ **W16 全量回归：99 packages OK / 100 tagged packages OK / 0 FAIL**
✅ **ci-lint-invariant：5 文件 / 20 invariants / 0 error / 0 conflict**

**进入 S6 归档阶段：** move `openspec/changes/devrix-tools-terminal-architecture/` → `openspec/archive/2026-06-18-devrix-tools-terminal-architecture/`，PR #76 squash merge + auto-merge。