# Proposal: Devrix Tools 终态架构

**Change ID:** devrix-tools-terminal-architecture
**Demand ID:** DM-20260618-007
**Status:** S2_Resolved（R2 博弈论 Review 100% 共识达成，2026-06-18）

---

## 1. 背景

Devrix 工具系统经过 5 个 OpenSpec change 的迭代，架构契约层（ToolSurface 6 方法 + ToolSpec 4 正交标志 + ToolFilter 组合链 + CheckPermission 三态 + DeferLoading 懒加载）已基本达到终态。但工具能力层和执行引擎层存在显著差距：无 LSP 代码智能、Bash 仅正则匹配、无文件诊断追踪、无自由分叉探索、无实现后验证、执行引擎不支持混合批次调度。

更重要的是，当前缺乏一个**终态架构蓝图**来指导后续所有工具能力建设——每个新能力的引入如果缺乏统一的架构对齐基准，可能导致 Surface 膨胀失控、Filter 链优先级混乱、跨域边界漂移。

## 2. 问题陈述

### 2.1 三个结构性矛盾

任何 AI Agent 工具系统面对三个不可调和的矛盾：

1. **能力 vs 安全**：工具越多越开放，Agent 能力越大，但破坏半径也越大
2. **可见性 vs 认知负荷**：LLM 需要看到所有可用工具，但 prompt 长度和注意力有限
3. **灵活性 vs 可验证性**：Agent 需要动态选择工具路径，但外部需要确定性验证

### 2.2 当前均衡失灵

- **信息获取瓶颈**：Agent 只能用 grep + 多轮 read_file 模拟代码理解，无 LSP 的结构化代码智能
- **无反馈开环**：编辑后无即时诊断反馈，Agent 不知道是否引入新错误
- **执行引擎粗糙**：混合批次工具全部串行，浪费并行能力
- **探索受限**：Agent 不能脱离 DAG 自由分叉探索

### 2.3 架构产权问题（核心决策）

**D2 物理上不持有 D3 引用。** 这是基于 Principal-Agent 决策权对齐原则 + Mechanism Design (Myerson 1979) + 可审计性原则的架构决策：

- LLM 调用权归 D7（Mechanism Designer / Controller），D2（Player / Worker）仅有工具执行权
- D7 承担 LLM 调用的全局成本（token、延迟），有动机最小化不必要的调用。配套硬性 token 预算上限防止 D7 自身过度调用
- D2 自主调用 LLM 无法被独立审计（Principal-Agent 框架下的决策权与成本对齐）

## 3. 提议方案

### 3.1 终态架构：五层正交模型

```
┌─────────────────────────────────────────────────────┐
│  Semantic Legend (语义映射):                         │
│   "Leader"   = Control-Theory Controller (D7)       │
│   "Follower" = Plant in hierarchical control (D2)   │
│   严格博弈论含义:                                     │
│     D7 = Mechanism Designer (Myerson 1979)           │
│     D2 = Player (Revelation Principle)               │
│   工程直觉含义:                                       │
│     D7 = Orchestrator, D2 = Worker                  │
│   CI lint: proposal.md 必须包含此 legend             │
└─────────────────────────────────────────────────────┘

D7 编排层 (Leader/Designer) — LLM 调用权 + ToolFilter 链 + PermissionGate + WorkerContext
    ↓ LLM 调用
D3 LLM Gateway — GuardContent + ErrorClassifier
    ↓ 解析 tool_use
D2 上下文引擎 (Follower/Player) — ToolSurface 拆面契约层
    ├── 感知层 (ReadOnly=true)  : read_file / glob / grep / lsp / web_fetch / web_search
    ├── 执行层 (Destructive)    : write_file / edit_file / bash / task_*
    ├── 编排层 (OpenWorld)      : delegate_* / free_fork / ask_user_question
    ├── 验证层 (ReadOnly=true)  : verify_plan_execution / query_diagnostics
    └── 元认知层               : tool_search
    ↓
D2 执行安全层 — BashASTPolicy (AST 级) + Sandbox + DiagnosticTracker
    ↓
D5 可观测性 — Span + Metric + chain_consistency 交叉验证
    ↓
D6 演化层 — Reputation → L3 保守路由 + VerifyPlanExecution
```

### 3.2 三个阶段交付

**Phase 1 (P0)**: LSP Tool + BashAST + 文件诊断追踪 + 自由分叉 + 自动验证 + import lint
**Phase 1.5 (P0)**: MCP 机制设计预研 + LTL-Lite 不变式规约框架（R1/R2 博弈论 Review 新增——Phase 2 启动前必须完成）
**Phase 2 (P1)**: StreamingToolExecutor 混合批次 + PerSessionFilter + MCP 接入 + Capability Attestation + Costly Sandboxing + Cross-Validation + Causal Audit Trail
**Phase 3 (P2)**: Web 工具 + 上下文分析 + /doctor + 会话转录 + 故障注入 + 错误分类 + 工具注意力热力图

### 3.3 五个架构不变式

1. 拆面单一职责 — 每个 ToolSurface 只管一组内聚工具
2. Filter 纯函数 — 无 I/O、无副作用、可独立测试
3. 可见性 ≠ 可执行性 — ToolFilter 控制看到什么，CheckPermission 控制能否执行
4. 信号不可伪造 — 4 bool 来自硬编码真值表，duration_ms 来自 D7 wall clock。CI lint 检查真值表 + T 层运行时 spot-check 一致性
5. T 锚点全覆盖 — 每个工具的关键行为有 DSAFT T 层测试点
6. Filter 链 FIFO 可辩护 — PerAgent → PerRisk → PerSession → PlanMode → UserCustom 固定顺序，新 filter 插入必须证明不破坏 subgame perfect 性质

## 4. 成功度量

| 度量 | 当前 | 目标 |
|------|------|------|
| P0 T 点覆盖率 | 153/153 IMPLEMENTED | 保持不变（新增 T 点必须 IMPLEMENTED） |
| LSP 操作可用数 | 0 | 4 P0 (goToDefinition / findReferences / incomingCalls / hover) |
| Bash 安全分析深度 | 正则匹配 | AST 级 + heredoc + 20+ zsh 模式 |
| 工具总数 | ~20 | ~35 |
| 诊断反馈延迟 | ∞（无反馈） | 异步（不阻塞编辑主路径） |
| D2→D3 import 数 | 0（通过 lint 保证） | 0（CI import lint 强制执行） |

## 5. 实施计划

### Phase 1: 致命缺口（5 个独立子 change，每个可独立 PR）

| 子 change | 核心交付 | 依赖 |
|-----------|---------|------|
| `feat/lsp-tool-surface` | LSP Tool Surface 完整实现（gopls/tsserver adapter） | DM-007 surface 契约 |
| `feat/bash-ast-security` | Bash AST 安全引擎（mvdan.cc/sh AST） | DM-20260618-002 BashAST 基础 |
| `feat/diagnostic-tracker` | 文件诊断追踪（编辑前后 diff + LRU 去重） | DM-20260618-001 ToolSpec |
| `feat/free-fork-explore` | 自由分叉探索（脱离 DAG + SendMessage + worktree） | DM-20260616-001 不确定性缺口 |
| `feat/verify-plan-execution` | 实现后自动验证（对照 tasks.md） | D6 eval 框架 |

### Phase 1.5: MCP 机制设计预研（R1/R2 博弈论 Review 新增，Phase 2 启动前必须完成）

| 子 change | 核心交付 |
|-----------|---------|
| `feat/mcp-mechanism-design` | MCP 多中心均衡分析 + Capability Attestation 协议 + Reputation Decay 设计 |
| `feat/ltl-lite-invariant` | LTL-Lite 不变式规约框架（Go struct tag DSL + `_invariant.go` 规范） |

### Phase 2: 执行质量（3 个子 change）

| 子 change | 核心交付 |
|-----------|---------|
| `feat/streaming-executor-v2` | 混合批次 + sibling abort + discard |
| `feat/per-session-filter` | PerSessionFilter |
| `feat/mcp-surface` | MCP Surface |

## 6. 风险与缓解

| 风险 | 缓解 |
|------|------|
| Surface 数量膨胀到 12+ | PluginSurface 模式复用；同质工具合并；AC25 Surface 合并异质性门槛 |
| Surface 搭便车均衡 | 新工具必须申报"为什么不能合并到现有 Surface"，S3-Gate 审查 |
| LSP server 进程数失控 | 单 workspace 最多 4 并发，LRU 淘汰 |
| 诊断追踪放大编辑延迟 | 异步化，不阻塞编辑主路径 |
| MCP 工具提权 | RiskLevel 由 Devrix 评估 + Capability Attestation (AC22) + Costly Sandboxing (AC23) + Cross-Validation (AC24) |
| MCP 多中心均衡失稳 | Phase 1.5 机制设计预研必须在 Phase 2 启动前完成 |
| 自由分叉滥用 | 硬上限：并发 8（来源：单 machine 内存/CPU 限制），总预算可配置；fork 间文件锁优先级 + LSP server 公平调度 |
| CheckPermission 空头承诺 | 承诺有效期（Allow 仅当前 turn 有效）+ 撤销协议 + Causal Audit Trail (AC26) |

## 7. Out of Scope

- 不重构 D2 QueryLoop 主路径
- 不修改 D7 Turn 编排状态机核心逻辑
- 不覆盖 Python/Rust LSP server（初期仅 Go + TypeScript）
- 不实现 Clawcode 私有协议
- D6 EvolutionPolicy → D7 保守路由接口在独立 change 中处理
