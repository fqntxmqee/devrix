---
demand-id: DM-20260618-007
title: Devrix Tools 终态架构 — 五层正交工具系统的能力蓝图与落地路线图
priority: P0
status: S1_Resolved（S1 checklist 12/12 完成，proposal S2 R2 100% 共识，2026-06-18 进入 S3 Design）
dsaft_domain: multi-domain
created: 2026-06-18
parent_docs:
  - openspec/archive/2026-06-17-devrix-tool-surface-contract/
  - openspec/archive/2026-06-18-devrix-tool-spec-enrichment/
  - openspec/archive/2026-06-18-devrix-surface-permission-extension/
  - openspec/archive/2026-06-18-devrix-surface-lazy-loading/
  - openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/
  - docs/reference/clawcode-diagnostic-tools-analysis.md
  - openspec/archive/2026-06-14-devrix-d7-sa-refine/gaming-analysis.md
  - openspec/archive/2026-06-15-devrix-d7-turn-orchestration/
---

# Demand: Devrix Tools 终态架构

> 本文档从博弈论视角、第一性原理、DSAFT 拆面方法论三个维度，定义 Devrix 工具系统的终态架构蓝图、能力建设路线图、以及与业界领先设计的差距分析。

---

## 1. 背景

### 1.1 当前状态

Devrix 工具系统经过 5 个 OpenSpec change 的迭代（DM-007 工具面契约化 → DM-008 phase-2 global 清理 → DM-20260618-001 ToolSpec 正交标志 → DM-20260618-002 CheckPermission 三态 → DM-20260618-003 DeferLoading 懒加载），架构契约层已基本达到终态：

| 能力 | 状态 |
|------|------|
| ToolSurface 6 方法契约 + ToolSpec 4 正交标志 + DeferLoading | ✅ IMPLEMENTED |
| ToolFilter 组合管道 (PerAgent + PerRisk + PlanMode) | ✅ IMPLEMENTED |
| InterruptBehavior (cancel/block) | ✅ IMPLEMENTED |
| CheckPermission 三态 (Allow/Deny/Ask) + BashASTPolicy | ✅ IMPLEMENTED |
| ToolSearchSurface 按需发现 | ✅ IMPLEMENTED |
| BuildSurfaces 单入口、0 global singleton | ✅ IMPLEMENTED |
| 9 个 ToolSurface (Builtin/LSP/FreeFork/Tracker/Verify/Delegate/BGTask/ToolSearch/AskUserQuestion) | ✅ IMPLEMENTED |

**但工具能力层和执行引擎层存在显著差距：**

| 差距 | 严重度 | 说明 |
|------|--------|------|
| **无 LSP 代码智能** | P0 致命 | goToDefinition / findReferences / incomingCalls 全部缺失 |
| **Bash 仅正则匹配** | P0 安全 | 无可验证的 AST 级分析，heredoc/zsh 攻击面未覆盖 |
| **无文件诊断追踪** | P0 重要 | 编辑后不知是否引入新错误，无反馈闭环 |
| **无自由分叉探索** | P0 重要 | Agent 不能并行探索多个方向 |
| **无实现后验证** | P0 重要 | S4-Gate 靠人工 Review，无可重复的自动化验证 |
| **StreamingToolExecutor 仅全量并行** | P1 | 不支持混合批次调度、sibling abort、fallback discard |
| **无 PerSessionFilter** | P1 | 同一 agent 类型无法按 session context 动态调整工具集 |
| **无 MCP 协议接入** | P1 | 工具生态封闭，无法接入第三方工具 |

### 1.2 为什么需要终态架构蓝图

DM-007~DM-20260618-003 完成了"工具的架构应该长什么样"的契约化定义。但每个新工具能力的引入（LSP、诊断追踪、自由分叉等）如果缺乏统一的终态视图，会面临三个风险：

1. **Surface 膨胀失控**：每加一个能力就新建一个 Surface，没有合并/复用准则
2. **Filter 链优先级混乱**：静态 filter 和动态 filter 的组合顺序缺乏原则
3. **跨域边界漂移**：新能力可能模糊 D2↔D3↔D4↔D7 的 Mechanism Design 边界

本需求文档定义工具系统的**终态架构蓝图**，作为后续所有工具能力建设的**架构对齐基准**。

---

## 2. 问题陈述

### 2.1 核心命题：工具系统的三个不可调和矛盾

回到第一性原理，任何 AI Agent 工具系统面对三个结构性矛盾：

| 矛盾 | 张力 A | 张力 B |
|------|--------|--------|
| **能力 vs 安全** | 工具越多、越开放，Agent 能解决的问题越大 | 工具越多、越开放，破坏半径越大 |
| **可见性 vs 认知负荷** | LLM 需要看到所有可用工具才能正确决策 | prompt 长度有限，工具描述占用 context window |
| **灵活性 vs 可验证性** | Agent 需要动态选择工具路径应对不确定性 | 外部系统需要确定性地验证"Agent 没做坏事" |

当前 Devrix 工具系统在架构契约层已经为这三个矛盾提供了机制设计解（ToolFilter 链 = Screening、4 正交标志 = 类型信号、CheckPermission = Commitment Device），但在工具能力层和执行引擎层，这些机制的**能力覆盖范围**和**执行质量**还有显著不足。

### 2.2 现状的核心均衡失灵

当前工具系统的博弈论均衡存在三个结构性缺陷：

| 缺陷 | 博弈论描述 | 用户痛感 |
|------|-----------|---------|
| **信息获取瓶颈** | Agent 只能用 grep + 多轮 read_file 模拟代码理解 | 排错靠 grep + 大脑模拟调用栈，效率极低 |
| **无反馈开环** | 编辑后无即时诊断反馈，Agent 不知道是否引入新错误 | 编辑后只能靠后续 test 或告警发现错误 |
| **执行引擎粗糙** | 混合批次工具全部串行执行，浪费并行能力 | 多工具调用轮次延迟高 |

### 2.3 架构产权问题：为什么 D2 不能持有 D3 引用

这是 Devrix 工具系统最关键的架构产权决策。当前架构中，LLM 调用链路为：

```
D7 → 从 D2 获取上下文（工具列表、文件状态、会话历史）
   → D7 调用 D3 LLM Gateway
   → D7 解析 tool_use
   → D7 通过 D2 ToolSurface 执行工具
```

**D2 不持有 D3 引用。** 这个约束不是因为"D2 不需要 LLM"，而是基于以下三个博弈论原因：

1. **Principal-Agent 框架下的决策权与成本对齐**：LLM 调用权是系统中最稀缺的资源。如果 D2 拥有它，D2 可以自主决定"多调一次模型"——D2 不承担 token 消耗成本，但享受额外推理的收益（道德风险，Jensen & Meckling 1976）。决策权归 D7 (Principal) 后，D7 承担全局成本，有动机最小化不必要的 LLM 调用。此产权配置必须配套**硬性 token 预算上限 (hard budget cap)**，防止 D7 自身的二级代理问题——D7 也可能在压力下过度调用 LLM。

2. **Mechanism Design / Direct Revelation (Myerson 1979)**：D7 作为机制设计者 (Mechanism Designer)，设计激励相容 (incentive-compatible) 的工具筛选机制，使得 D2/Workspace 选择"暴露真实能力需求"是占优策略。D7 通过独占 LLM 调用权来保证机制的单中心一致性——D2 是被动执行者而非策略主体，不存在"D2 破坏 D7 承诺"的博弈场景（D2 无独立收益函数）。真正的风险是 D7 内部子模块之间的策略冲突（如某 Worker 想偷懒少调 LLM 以省 token，但全局最优需要多调）。

3. **可审计性**：D7 的每次 LLM 调用暴露在 D5 span 中，可被 D6 事后 Judge 审计。D2 自主调用无法被独立审计——"工具调用慢"和"LLM 重试多"的 span 混在同一个域的内部。

**这个约束不是暂时的实现细节，而是终态架构的不变式。**

### 2.4 MCP 引入后的多中心相变 — 单中心机制设计的失效边界

当前 Devrix 的整个博弈论架构建立在**单中心假设**上：所有工具都是 Devrix 内部代码，D7 对工具的能力、风险和行为了如指掌。但 Phase 2 引入 MCP 协议后，博弈结构发生**相变 (phase transition)**：

| 维度 | 单中心 (当前) | 多中心 (MCP 后) |
|------|-------------|----------------|
| **信息结构** | D7 完全信息（硬编码真值表） | MCP server 有私有信息（私有工具能力） |
| **激励兼容** | D7 单方面设计机制 | MCP server 有自己的收益函数（想被更多调用以证明自身价值） |
| **信任模型** | Devrix 内部代码 = 完全信任 | 第三方代码 = 不完全信任 |
| **失败模式** | 内部 bug（可修复） | 恶意 MCP server、提权攻击、行为偏离声明 |
| **均衡类型** | 单设计者 → 多响应者 | 多设计者博弈（每个 MCP server 也是隐式机制设计者） |

**MCP 引入同时触发两类信息不对称**：

1. **Adverse Selection (逆向选择)**：MCP server 在注册时有动机夸大能力、低报风险（"我是安全的，多调用我"）。当前文档假设的"RiskLevel 由 Devrix 评估"在静态审查时有效，但无法防止 server 在运行时行为偏离声明。

2. **Moral Hazard (道德风险)**：MCP server 注册通过后，有动机在执行中做未声明的事（网络请求、文件读写、调用其他工具）。单次审查无法覆盖所有运行时路径。

**当前文档的缓解措施不足**：

- "RiskLevel 由 Devrix 评估" → 仅解决静态 Adverse Selection，不解决运行时 Moral Hazard
- "MCP 工具作为 MCPSurface 注册" → 仅解决契约层统一，不解决激励兼容

**必须在 Phase 2 启动前补全的机制设计**：

| 机制 | 博弈论原理 | 解决的问题 |
|------|-----------|-----------|
| **Capability Attestation** (signed metadata) | 可验证信息披露 (Verifiable Disclosure) | Adverse Selection |
| **Costly Sandboxing + Reputation Budget** | 重复博弈信誉 (Repeated Game Reputation) | Moral Hazard |
| **Cross-Validation** (≥2 server 结果比对) | 冗余验证 (Redundancy-based Verification) | 单点恶意 |
| **Reputation Decay Function** (信誉随时间指数衰减) | 持续激励兼容 (Dynamic Incentive Compatibility) | 长期行为维持 |

**关键架构决策**：在 Phase 2 启动前插入 **Phase 1.5: MCP 机制设计预研 (P0)**，否则 Phase 1 的成果在 Phase 2 会被部分推翻，产生 6-12 个月的技术债。

### 2.5 CheckPermission 承诺的可信性 — 制裁机制补全

§2.1 将 CheckPermission 三态 (Allow/Deny/Ask) 描述为 Commitment Device (Schelling 1960)。Commitment Device 的关键不仅是"承诺"本身，更是**承诺的不可逆性 + 违约可观测性**。

当前设计中缺失的部分：

1. **承诺的不可逆性**：用户点了 Allow 之后能否撤销？当前设计中 Allow 是瞬时的——本次授权后，同一 session 内是否还需要再次授权？如果 Allow 是永久的，则承诺没有时间边界，等同于无限授权。

2. **违约可观测性**：D2 是否能在执行前验证"这个权限是否还有效"？如果权限状态变更了（用户中途撤销），D2 能否感知？

3. **制裁机制**：如果只有承诺没有制裁 = **空头承诺 (cheap talk)**，博弈论上等同于无承诺。

**补全方案**：

| 要素 | 设计 |
|------|------|
| **承诺有效期** | Allow 仅当前 turn 有效，跨 turn 必须重新授权（防止"一次 Allow = 永久授权"） |
| **撤销协议** | 用户在任意时刻可撤销之前的 Allow（通过 `/revoke` 或 Ask 弹窗中的"撤销所有"按钮） |
| **违约可观测性** | CheckPermission 执行前查询当前有效授权集合，权限状态变更立即生效 |
| **事后审计** | 每条破坏性操作可追溯到 4-tuple：(a) 哪个工具 (b) 哪个 filter 放行 (c) 哪个 permission gate 通过 (d) 哪条 LLM tool_use 触发 |

---

## 3. 终态架构蓝图

### 3.1 五层正交架构

```
┌──────────────────────────────────────────────────────────────────┐
│                     D7 编排层 (Leader)                            │
│  ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────────┐  │
│  │ 路由矩阵  │ │ToolFilter │ │Permission │ │  WorkerContext   │  │
│  │ Screening│ │  组合链   │ │   Gate    │ │  自包含契约       │  │
│  │          │ │           │ │           │ │                  │  │
│  │ FastPath │ │PerAgent   │ │ Request   │ │ Goal + FileHints │  │
│  │ CmdPath  │ │PerRisk    │ │ CheckPerm │ │ + Constraints    │  │
│  │ WaveExec │ │PerSession │ │ PlanMode  │ │ 禁止模糊指代     │  │
│  │ PlanPath │ │UserCustom │ │ Policy    │ │                  │  │
│  └──────────┘ └──────────┘ └───────────┘ └──────────────────┘  │
│                                                                    │
│  D7 持有 LLM 调用权 ──→ D3 LLM Gateway ──→ LLM API               │
│  D2 物理上不持有 D3 引用（产权配置硬约束）                         │
├──────────────────────────────────────────────────────────────────┤
│               D2 上下文引擎 (Follower 执行)                       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              ToolSurface 拆面契约层 (9 → 终态 12+)        │   │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌───────┐  │   │
│  │  │Builtin │ │  LSP   │ │FreeFork│ │Verify  │ │Tool   │  │   │
│  │  │Surface │ │Surface │ │Surface │ │Surface │ │Search │  │   │
│  │  │read    │ │goToDef │ │fork    │ │verify  │ │       │  │   │
│  │  │write   │ │findRef │ │explore │ │plan    │ │catalog│  │   │
│  │  │edit    │ │inCall  │ │N方向   │ │exec    │ │       │  │   │
│  │  │bash    │ │hover   │ │        │ │        │ │       │  │   │
│  │  │grep    │ │sym     │ │        │ │        │ │       │  │   │
│  │  │glob    │ │        │ │        │ │        │ │       │  │   │
│  │  └────────┘ └────────┘ └────────┘ └────────┘ └───────┘  │   │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────────────────┐  │   │
│  │  │Tracker │ │Delegate│ │BGTask  │ │AskUserQuestion     │  │   │
│  │  │diag    │ │explore │ │stop    │ │1-4 问题, 2-4 选项  │  │   │
│  │  │track   │ │plan    │ │output  │ │                    │  │   │
│  │  │        │ │impl    │ │list    │ │                    │  │   │
│  │  │        │ │status  │ │notify  │ │                    │  │   │
│  │  └────────┘ └────────┘ └────────┘ └────────────────────┘  │   │
│  │  ┌────────┐ ┌────────┐                                     │   │
│  │  │  MCP   │ │  Web   │  ← 终态新增                         │   │
│  │  │Surface │ │Surface │                                     │   │
│  │  │mcp_*   │ │fetch   │                                     │   │
│  │  │        │ │search  │                                     │   │
│  │  └────────┘ └────────┘                                     │   │
│  └──────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              ToolFilter 组合管道                           │   │
│  │  PerAgent → PerRisk → PerSession → PlanMode → UserCustom │   │
│  └──────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │           StreamingToolExecutor (执行引擎, 终态 v2)       │   │
│  │  混合批次并发 │ sibling abort │ fallback discard │progress│   │
│  └──────────────────────────────────────────────────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│             D2 执行安全层 (Enforce)                               │
│  ┌──────────────────┐  ┌────────────────┐  ┌───────────────┐   │
│  │  BashASTPolicy   │  │   Sandbox      │  │  Diagnostic   │   │
│  │  mvdan.cc/sh AST │  │   进程隔离      │  │  Tracker      │   │
│  │  heredoc 审计     │  │                │  │  编辑前后 diff │   │
│  │  20+ zsh 攻击面   │  │                │  │  LRU 去重     │   │
│  └──────────────────┘  └────────────────┘  └───────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│          D3 LLM Gateway (内容安全 + 错误分类)                     │
│  ┌──────────────────┐  ┌────────────────────────────────────┐   │
│  │  GuardContent    │  │  ErrorClassifier (20+ 错误类型)     │   │
│  └──────────────────┘  └────────────────────────────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│       D5 可观测性 (客观锚点 + 交叉验证)                           │
│  ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────┐     │
│  │ d7.route │ │ d7.flow  │ │ chain_    │ │ threshold_   │     │
│  │ .decision│ │ .event   │ │consistency│ │breach_rate   │     │
│  └──────────┘ └──────────┘ └───────────┘ └──────────────┘     │
├──────────────────────────────────────────────────────────────────┤
│       D6 演化层 (信誉 + 事后 Judge)                               │
│  ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────┐     │
│  │Reputation│ │ L3 保守  │ │ Verify    │ │ Strategy     │     │
│  │ Store    │ │ 路由收缩  │ │ Plan Exec │ │ Evolution    │     │
│  └──────────┘ └──────────┘ └───────────┘ └──────────────┘     │
└──────────────────────────────────────────────────────────────────┘
```

### 3.2 终态架构的五个不变式

| # | 不变式 | 博弈论对应 | 验证方式 |
|---|--------|-----------|---------|
| 1 | **拆面单一职责** — 每个 ToolSurface 只管一组内聚工具，不跨关注点 | Facet Decomposition | `devrix tool list` 按 surface 分组 |
| 2 | **Filter 纯函数** — 每个 ToolFilter.Apply 无 I/O、无副作用、可独立测试 | Screening 可组合 | 单测覆盖每个 filter |
| 3 | **可见性 ≠ 可执行性** — ToolFilter 控制 LLM 看到什么，CheckPermission 控制能否执行 | Separating Equilibrium | 两者独立变更互不影响 |
| 4 | **信号不可伪造** — ToolSpec 的 4 bool 来自硬编码真值表，duration_ms 来自 D7 wall clock | Costly Signal 真实性 | CI 检查真值表 |
| 5 | **T 锚点全覆盖** — 每个工具的关键行为有 DSAFT T 层测试点 | Commitment Device | P0 T 100% PASS |
| 6 | **Filter 链 FIFO 可辩护** — PerAgent → PerRisk → PerSession → PlanMode → UserCustom 的固定顺序在"高优先级约束总是先过滤"假设下是 subgame perfect 的。新 filter 插入必须证明不破坏此性质 | Screening 均衡稳定性 | S3-Gate 审查 filter 插入的均衡影响 |

---

## 4. 验收标准

### 4.1 P0（Phase 1 — 致命缺口，不交付则博弈论承诺无法兑现）

| ID | 标准 | 验证方式 |
|----|------|---------|
| **AC1** | **LSP Tool Surface** 提供 `goToDefinition` / `findReferences` / `incomingCalls` 三个 P0 操作。基于 gopls/tsserver 现有 LSP server，在 ToolSurface 契约内注册，LLM 可通过 tool_use 调用。结果格式化输出（行号+上下文）。不修改 D2 主路径。 | 端到端测试：prompt 含调用指令 → 工具被自动调度 → 结果含正确行号和上下文 |
| **AC2** | **Bash AST 安全引擎** 使用 `mvdan.cc/sh`（纯 Go，无 CGO）进行 AST 级 bash 命令解析。拒绝已知危险模式（rm -rf /、dd、mkfs、sudo、chmod 777 /）。heredoc 内容独立审计。zsh 特有攻击面检测 ≥ 20 种模式（zmodload、sysopen、syswrite、=cmd 展开等）。现有正则 allowlist/deny 作为 fallback 保留。 | 单测：每个危险模式至少 1 个绕过尝试 → 被 AST 层拦截；heredoc 注入 → 被独立审计拦截 |
| **AC3** | **文件诊断追踪** 在 `edit_file` / `write_file` 前拍诊断快照（linter），编辑后异步跑 linter，diff 输出"本次改动引入的新错误"。跨轮次 500 文件 LRU 去重。编辑主路径不阻塞（异步触发）。 | 单测：编辑故意引入语法错误 → diff 出现新错误；编辑无关行 → diff 为空；去重：同一文件两次编辑 → 不重复报告 |
| **AC4** | **自由分叉探索** Agent 可以脱离 DAG 拓扑即兴分叉 N 个子代理并行探索。子代理间通过 SendMessage 直接通信。每个子代理默认 worktree 隔离。硬上限：最大并发 fork 数 8，总 session fork 预算可配置。 | 集成测试：单次探索任务 fork 3 个子代理 → 3 方向并行执行 → 结果合并 |
| **AC5** | **实现后自动验证** (`verify_plan_execution`) 实现完成后自动对照 `tasks.md` 逐项验证。验证失败生成结构化差异报告（JSON）。不依赖人工 Review。 | 单测：tasks.md 含 3 项任务 → 实现完成 2 项 → 输出差异报告含 1 项未完成 |
| **AC6** | **D2 不持有 D3 引用** 作为架构不变量进行静态验证。`internal/layers/contextengine/` 下任何 .go 文件不得 import D3 的任何 package。import lint 规则在 CI 中执行，违反视为构建失败。 | CI: `go build -tags=layerlint` 或等价的 import 检查脚本 |
| **AC7** | 所有新增能力不破坏现有 P0 T 层（TOOL-SURFACE-1 T01-T38、PERMISSION-GATE-1 T01-T02 全部保持 PASS） | `go test -race ./...` 全量回归 |
| **AC25** | **Surface 合并显式收益门槛**：新建独立 Surface 必须证明其工具的 Risk Level 异质性 ≥ 阈值（与已有 Surface 的工具 RiskLevel 分布有显著差异），或无法合并到现有 Surface 的技术理由。开发者提交新工具时必须申报"为什么不能合并到现有 Surface"，由 D7 在 S3-Gate 审查。防止搭便车均衡——每个新工具都倾向挂到大 Surface 以获得曝光，最终大 Surface 重新膨胀、filter 失效。 | S3-Gate: 新 Surface 注册时检查异质性声明 |

### 4.2 P1（Phase 2 — 执行质量提升）

| ID | 标准 |
|----|------|
| **AC8** | **StreamingToolExecutor 混合批次调度**：concurrency-safe 工具可与 safe 工具并行，非 safe 工具独占执行。规则：executing=0 → 任意启动；新工具 safe && 所有 executing safe → 并行；新工具 !safe → executing=0 后执行。结果按 LLM 返回顺序 emit。Bash sibling abort：并行 Bash 任一失败取消其余。Fallback discard：模型切换时丢弃在途工具。 |
| **AC9** | **PerSessionFilter** 支持按 session context 动态调整工具集。session type "code_review" → 不需要 edit_file/bash；"debug" → 需要 LSP + diagnostic tracking；"refactor" → 需要 edit_file + bash + verify。Filter 链组合顺序：PerAgent → PerRisk → **PerSession** → PlanMode → UserCustom。 |
| **AC10** | **MCP 协议接入** 支持 MCP (Model Context Protocol) 工具服务器。MCP 工具的 RiskLevel 由 Devrix 评估（不接受 server 自报）。MCP 工具作为 `MCPSurface` 注册到 ToolSurface 契约。 |
| **AC22** | **MCP Capability Attestation**：MCP server 注册时必须提供确定性可验证的能力清单（signed metadata，类似 TUF 的签名元数据）。Devrix 运行时持续监测实际调用是否与声明偏离，偏离触发降权。解决 Adverse Selection。 | 单测：模拟 MCP server 声明"只读"但实际尝试写文件 → 被运行时监测拦截 + 降权 |
| **AC23** | **MCP Costly Sandboxing + Reputation Budget**：每个 MCP server 初始化获得信誉预算 (reputation budget)。每次调用消耗预算（调用次数越多消耗越快）。信誉低于阈值自动触发降权（RiskLevel 上升一级）。只有持续产生正确结果才能恢复信誉。解决 Moral Hazard。 | 单测：模拟 MCP server 快速消耗信誉预算 → 自动降权触发 |
| **AC24** | **MCP Cross-Validation**：对于关键操作（Destructive=true 或 OpenWorld=true 的 MCP 工具），同一任务至少 2 个独立 MCP server 结果比对。结果不一致时以 Devrix 内置工具结果为准。 | 集成测试：2 个 MCP server 对同一文件搜索返回不同结果 → Devrix 内置 glob/grep 结果胜出 |
| **AC26** | **Causal Audit Trail**：D6 Evolution 增加可审计追溯链。每条破坏性操作必须可追溯到 4-tuple：(a) 哪个工具 (b) 哪个 filter 放行 (c) 哪个 permission gate 通过 (d) 哪条 LLM tool_use 触发。形成完整的因果审计链。 | 单测：模拟破坏性操作 → 4-tuple 链完整可查询 |
| **AC29** | **MCP Reputation Decay Function**：MCP server 信誉预算随时间指数衰减 (exponential decay)。衰减速率与工具 RiskLevel 成正比——高风险工具的 server 信誉衰减更快。server 必须持续产生正确结果来维持信誉水平。 | 单测：模拟 MCP server 静默一段时间后 → 信誉自动衰减 → 下一次调用需要重新证明 |

### 4.3 P2（Phase 3 — 生态扩展与体验优化）

| ID | 标准 |
|----|------|
| **AC11** | Web 工具 (`web_fetch` / `web_search`) 注册为 WebSurface，标记 OpenWorld=true。 |
| **AC12** | 上下文窗口分析（按类别 token 分解：system/tools/messages/thinking），输出结构化报告。 |
| **AC13** | `/doctor` 自诊断命令检查安装/配置/上下文健康，输出 JSON 报告。 |
| **AC14** | Debug 日志分类过滤支持 `--debug=api,hooks,telemetry` 按类别开关。 |
| **AC15** | 会话转录 JSONL 持久化 + `--continue` 恢复。 |
| **AC16** | 故障注入 harness + 错误分类引擎（20+ 标准化错误类型映射）。 |
| **AC27** | **工具注意力热力图**：上下文窗口分析 (AC12) 扩展为工具注意力热力图。记录每轮 LLM response 中实际被引用的工具，统计"曝光但未使用"的工具占比。识别 DeferLoading 策略是否有效减少了认知负荷。 |

### 4.4 范围/质量基线

| ID | 标准 |
|----|------|
| **AC17** | 每个新增 ToolSurface 必须在 DSAFT T 层预登记测试点（S2 阶段 PLANNED，S4 阶段 IMPLEMENTED）。 |
| **AC18** | 每个新增 ToolSurface 必须填充 ToolSpec 的 4 正交标志（扩展 `orthogonal_flags.go` 真值表）。 |
| **AC19** | 跨域新增 import 不得引入新的依赖环（layering 规则：D2 不能 import D7，D3 不能 import D2）。 |
| **AC20** | 涉及安全敏感的变更（BashAST、MCP 协议接入）必须经 `verify-security` 闸门。 |
| **AC21** | 覆盖率 ≥ 80%，P0 T 点 100% PASS。 |
| **AC28** | **LTL-Lite 不变式规约** (Phase 1.5)：每个 ToolSurface 必须有一个 `_invariant.go` 文件，用 Go struct tag 声明 5-10 条不变式。仅规约语言 (spec language)，不引入 model checker。运行时通过 assert 校验。CI lint 检查 `_invariant.go` 文件存在。MCP server 的 Reputation Budget 用不变式规约。 |

---

## 5. 依赖与约束

| 类型 | 内容 |
|------|------|
| **上游已完成** | DM-20260617-007 devrix-tool-surface-contract（ToolSurface + ToolFilter 契约） |
| **上游已完成** | DM-20260618-001 devrix-tool-spec-enrichment（4 正交标志 + InterruptBehavior） |
| **上游已完成** | DM-20260618-002 devrix-surface-permission-extension（CheckPermission 三态 + BashAST 基础） |
| **上游已完成** | DM-20260618-003 devrix-surface-lazy-loading（DeferLoading + ToolSearchSurface） |
| **上游已完成** | DM-20260617-008 devrix-tool-surface-phase2-full（0 global singleton） |
| **上游已完成** | DM-20260616-003 devrix-diagnostic-tools-parity（DM-003 已定义 13 项能力需求，本 change 继承并扩展为终态架构） |
| **上游已完成** | DM-20260614-020 devrix-d7-turn-orchestration（D7→D3 LLM 直达，TurnOrchestrator 状态机，D2→D3 import lint） |
| **上游已完成** | DM-20260616-001 devrix-d7-uncertainty-gaps（PlanAgent 运行时门控 + ConflictGuard 原子化） |
| **上游已完成** | DM-20260616-002 devrix-d7-loop-first-routing（loop_first 默认 ingress） |
| **约束** | D2 物理上不得持有 D3 引用 — import lint CI 规则强制执行 |
| **约束** | LSP server 进程管理不得绕过现有 sandbox — 必须复用 D1 sandbox |
| **约束** | 文件诊断追踪的 linter 调用走异步路径（不阻塞编辑主路径） |
| **约束** | MCP 工具的 RiskLevel 必须由 Devrix 评估，不接受 MCP server 自报 |
| **约束** | 自由分叉有硬上限（最大并发 8，来源：单 machine 内存/CPU 限制，非博弈论最优解）。总 session fork 预算可配置。fork 间资源争抢协议：文件锁优先级 + LSP server 公平调度（防公地悲剧） |

---

## 6. 变更范围

### 6.1 新增（Phase 1 — P0）

- `internal/layers/contextengine/enforce/toolrunner/surface/lsptool_surface.go` — LSP Tool Surface 完整实现（goToDefinition / findReferences / incomingCalls / hover）
- `internal/layers/contextengine/enforce/toolrunner/lsp/` — LSP server 管理（gopls/tsserver adapter, LRU 淘汰, 多 server 路由）
- `internal/layers/contextengine/enforce/toolrunner/sandbox/bash_ast.go` — Bash AST 安全引擎（mvdan.cc/sh AST 解析 + heredoc 审计 + zsh 攻击面）
- `internal/layers/observability/diagnose/tracker/diagnostic_tracker.go` — 文件诊断追踪服务（编辑前后快照 + linter diff + 500 文件 LRU 去重）
- `internal/layers/multiagent/provision/freefork/` — 自由分叉探索（脱离 DAG 拓扑, SendMessage channel, worktree 隔离, 并发上限）
- `internal/layers/evolution/evaluate/verify/plan_verifier.go` — 实现后自动验证（对照 tasks.md 逐项验证 + JSON 差异报告）

### 6.2 新增（Phase 2 — P1）

- `internal/layers/contextengine/enforce/toolrunner/executor/streaming_executor_v2.go` — StreamingToolExecutor 混合批次调度
- `internal/layers/contextengine/enforce/toolrunner/filter/per_session.go` — PerSessionFilter
- `internal/layers/contextengine/enforce/toolrunner/surface/mcp_surface.go` — MCP Surface

### 6.3 新增（Phase 3 — P2）

- `internal/layers/contextengine/enforce/toolrunner/surface/web_surface.go` — Web Surface (fetch/search)
- `internal/layers/observability/diagnose/doctor/` — /doctor 自诊断
- `internal/layers/observability/instrument/logger/debugfilter/` — Debug 日志分类过滤
- `internal/layers/communication/capture/transcript/` — JSONL 会话转录
- `internal/layers/observability/diagnose/faultinject/` — 故障注入
- `internal/layers/llmgateway/protect/errorclass/` — 错误分类引擎

### 6.4 修改

- `internal/layers/contextengine/enforce/toolrunner/surface/orthogonal_flags.go` — 扩展真值表（新增工具条目）
- `internal/layers/contextengine/enforce/toolrunner/surface/orthogonal_flags_test.go` — T 层：D2-S8-AXX-TNN 工具 4 标志与行为一致性运行时校验（工具被调用时 spot-check 真值表与实际行为是否一致）
- `internal/layers/contextengine/enforce/toolrunner/surface/lsptool_surface.go` — 从占位实现升级为完整实现
- `internal/bootstrap/surfaces.go` — BuildSurfaces 注册新 Surface
- `openspec/specs/d2-context-engine/t-registry.md` — 新增 T 点登记
- `openspec/specs/d2-context-engine/a-registry.md` — 新增 Activity 登记

### 6.5 不变更

- D2 QueryLoop 主路径（ToolSurface 架构保证新工具即插即用）
- D7 Turn 编排主路径
- D3 LLM Gateway 现有限流/熔断/重试逻辑
- D1 Communication 协议层
- 现有 ToolSurface 接口定义（6 方法不变）
- 现有 ToolFilter 接口定义（1 方法不变）

---

## 7. 风险评估

### 7.1 架构风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| **Surface 膨胀** — 终态从 9 增长到 12+ Surface | ToolSurface 接口管理复杂度上升 | PluginSurface 模式已证明同质工具可合并；LSP 的所有 4 操作归属于同一 LSPToolSurface |
| **Filter 链性能退化** — 每轮 Apply 遍历 N 个 filter × M 个 spec | Prepare 延迟增加 | Filter 纯函数设计，单次 < 1μs；M > 50 时引入缓存 |
| **跨域边界漂移** — LSP/MCP 可能模糊 D2↔D3↔D4 边界 | 违反 layering 规则 | 每个新能力必须先登记 DSAFT 归属域并 S3-Gate 审查 |

### 7.2 安全风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| **LSP server 进程注入** | 沙箱绕过 | LSP server 复用 D1 sandbox；网络隔离；单 workspace 最多 4 个并发 server |
| **Bash AST bypass** — `mvdan.cc/sh` 的解析边界被绕过 | 危险命令执行 | 保留正则 fallback 作为 defense-in-depth 第二层 |
| **MCP 工具提权** | 数据泄露 | MCP 工具 RiskLevel 由 Devrix 评估，不接受 server 自报 |
| **自由分叉滥用** — Agent 无限 fork | DoS | 硬上限：最大并发 8，总 session fork 预算可配置 |

### 7.3 博弈风险

| 风险 | 博弈论描述 | 缓解 |
|------|-----------|------|
| **工具可见性膨胀** — 随着工具增多，PerAgentFilter "最小必须"原则被稀释 | LLM 策略空间过大 → 选择错误工具的概率上升 | 每个新工具必须回答"为什么这个 agent 类型需要它"，S3-Gate 审查 |
| **DeferLoading 过载** — 太多工具延迟加载 | Agent 频繁调 tool_search，信号获取成本超过信号价值 | tool_search 结果上限 5，分类过滤降低噪音 |
| **CheckPermission 疲劳** — Ask 频率上升 | 用户过度问询 → alarm fatigue → 用户盲批 | D5 监控 deny/ask 比率，异常告警 |
| **Surface 搭便车均衡** — 当 Surface 数量扩张到 12+ 时，开发者倾向把新工具挂到大 Surface 以获得曝光（搭便车），最终大 Surface 重新膨胀、filter 失效 | 策略空间：合并到现有 Surface vs 新建独立 Surface。收益不对称：合并收益（快速曝光）> 独立收益（精准过滤），自然均衡是所有新工具都往大 Surface 挂 | **AC25**：Surface 合并必须通过显式异质性门槛。开发者提交新工具时必须申报"为什么不能合并到现有 Surface"。PluginSurface 模式复用提供机制层面的合并路径 |

### 7.4 实施风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| **LSP server 进程数失控** — 大型 monorepo 启动多个 gopls/tsserver | OOM + 端口冲突 | 单 workspace 最多 N 个并发 server（默认 4），LRU 淘汰 |
| **文件诊断追踪放大编辑延迟** — linter 调用耗时 | 编辑 5s → 60s | 异步化（后台触发），编辑主路径不阻塞 |
| **Phase 1 范围过大** — 5 个 P0 能力并行实施 | 交付延期 | 每个能力可独立 PR，按优先级顺序合入 |

---

## 8. Out of Scope

- **不重构** D2 QueryLoop 主路径（已在独立 change 中处理）
- **不修改** D7 Turn 编排状态机核心逻辑
- **不引入** 新 LLM provider 或新通信协议
- **不覆盖** Python/Rust LSP server（初期仅 Go + TypeScript）
- **不实现** Clawcode 私有协议（如 `_claude_fs_right:` 双协议）
- **不设计** 完整的 D6 EvolutionPolicy → D7 保守路由接口（在 `devrix-reputation-feedback-loop` 独立 change 中处理）
- **不包含** 工时估算（按 requirements.md §6 禁止原则）

---

## 9. 落地路线图

```
Phase 1: 补致命缺口 (P0)
  ├── LSP Tool Surface (AC1)
  ├── Bash AST 安全引擎 (AC2)
  ├── 文件诊断追踪 (AC3)
  ├── 自由分叉探索 (AC4)
  ├── 实现后验证 (AC5)
  └── D2 不持有 D3 引用 import lint (AC6)

Phase 1.5: MCP 机制设计预研 (P0) ← R1/R2 博弈论 Review 新增
  ├── MCP 多中心均衡分析 (AC22/AC23/AC24/AC29 设计)
  ├── LTL-Lite 不变式规约框架 (AC28)
  ├── Capability Attestation 协议设计
  └── Reputation Decay Function 设计

Phase 2: 提升执行质量 (P1)
  ├── StreamingToolExecutor 混合批次 (AC8)
  ├── PerSessionFilter (AC9)
  ├── MCP 协议接入 (AC10)
  ├── MCP Capability Attestation 实现 (AC22)
  ├── MCP Costly Sandboxing 实现 (AC23)
  ├── MCP Cross-Validation 实现 (AC24)
  ├── Causal Audit Trail (AC26)
  └── MCP Reputation Decay 实现 (AC29)

Phase 3: 生态扩展 + 体验优化 (P2)
  ├── Web 工具 (AC11)
  ├── 上下文窗口分析 (AC12)
  ├── /doctor 自诊断 (AC13)
  ├── Debug 日志分类过滤 (AC14)
  ├── 会话转录 + --continue (AC15)
  └── 故障注入 + 错误分类引擎 (AC16)
```

### Phase 1 内部优先级排序

#### 依赖度矩阵

下表列出每个能力对其他能力的依赖程度：**0** = 无依赖 / **1** = 弱依赖（降级可用）/ **2** = 强依赖（前置必须）

| 能力 ↓ \ 依赖 → | LSP | BashAST | Tracker | FreeFork | Verify |
|-----------------|-----|---------|---------|----------|--------|
| **LSP** | — | 0 | 0 | 0 | 0 |
| **BashAST** | 0 | — | 0 | 0 | 0 |
| **Tracker** | 2 | 0 | — | 0 | 0 |
| **FreeFork** | 2 | 0 | 2 | — | 0 |
| **Verify** | 1 | 0 | 1 | 2 | — |

**关键观察**：
- LSP 和 BashAST 是独立基座（不依赖任何其他能力）→ 可以并行或任意顺序交付
- Tracker 强依赖 LSP（hover 才有诊断价值）→ LSP 必须前置
- FreeFork 依赖 LSP + Tracker（分叉探索需要代码理解 + 诊断反馈）→ 放第 4 位
- Verify 依赖 FreeFork（验证对象由前 4 项产生）→ 自然末端
- 所有能力间不存在循环依赖 → 拓扑排序有效

#### 排序理由

| 顺序 | 能力 | 理由 |
|------|------|------|
| 1 | **LSP Tool Surface** | 最大差距，从根本改变 Agent 的代码理解方式。grep → LSP 是信息获取效率的指数级跃迁。零外部依赖，可独立交付。 |
| 2 | **Bash AST 安全引擎** | 安全是能力的基石。没有 AST 级安全分析，增加任何新工具都会放大风险半径。与 LSP 无依赖关系，可与 LSP 并行实施，但建议 LSP 先行（LSP 覆盖面更广）。 |
| 3 | **文件诊断追踪** | 闭环反馈的最小可行实现。成本低（异步 linter diff），收益高（打破"编辑赌博"）。强依赖 LSP 的 hover 能力以获得有意义的诊断上下文。 |
| 4 | **自由分叉探索** | 解锁并行探索，直接提升复杂问题排查效率。依赖 LSP（代码理解）+ Tracker（诊断反馈）提供分叉方向的决策依据。 |
| 5 | **实现后验证** | 自动化 S4-Gate，但依赖前 4 项能力就绪后才有完整的验证对象（LSP 理解代码、BashAST 安全执行、Tracker 诊断结果、FreeFork 并行验证）。 |

#### 反驳预案

当 reviewer 对排序提出质疑时，以下预注册回答提供了可辩护性：

| 可能的质疑 | 回答 | 引用位置 |
|------------|------|----------|
| "为什么 LSP 和 BashAST 不一起做？" | BashAST 是安全基座，LSP 是能力扩展；先安全后能力是分层原则。两者无依赖，可并行实施，但串行交付降低集成风险 | §5 约束 "LSP server 不得绕过现有 sandbox" |
| "为什么 Tracker 不和 LSP 一起做？" | Tracker 强依赖 LSP 的 hover 才能产生有意义的诊断 diff。无 LSP 时，Tracker 降级为 grep-based 诊断（可用但价值低）。LSP 前置是收益最大化选择 | 依赖度矩阵 LSP=2 |
| "为什么 FreeFork 不放第 1 位？" | FreeFork 的探索收益依赖 LSP（代码理解）+ Tracker（诊断反馈）提供分叉决策依据。没有诊断能力的 fork 等于并行 grep——收益有限 | 依赖度矩阵 LSP=2, Tracker=2 |
| "为什么 Verify 放最后？" | Verify 的验证对象必须由前 4 项产生。没有验证对象的验证是空验证——没有 LSP 理解的代码、没有 Tracker 诊断的结果、没有 FreeFork 的并行探索，Verify 无对象可验 | 依赖度矩阵 FreeFork=2 |
| "为什么不 5 个并行一次性交付？" | 每个子 change 可独立 PR、独立测试、独立回滚。5 个并行交付的集成风险过高——一个能力出问题阻塞整体，且排错时需要排查 5 个变量 | §6.1 "每个能力可独立 PR" |
| "如果 LSP 延期，Tracker 是否被阻塞？" | 否。Tracker 在无 LSP 时可降级为 grep-based 诊断。依赖是"强依赖高价值"而非"硬阻塞"。这也是为什么 LSP 放第 1 位——降低 Tracker 被迫降级的概率 | AC3 诊断追踪不依赖 LSP 协议 |

---

## 10. 与业界领先设计的差距总结

### 10.1 vs Clawcode (Claude Code v2.1.88)

| 维度 | Clawcode | Devrix 当前 | Devrix 终态 |
|------|----------|------------|------------|
| **工具数量** | 40+ 内置 | ~20 | ~35 |
| **LSP 代码智能** | 9 操作全覆盖 | 占位（返回空） | 4 P0 操作 + hover + workspaceSymbol |
| **Bash 安全** | Tree-sitter AST 2592 行 | 正则 allowlist/deny | mvdan.cc/sh AST + heredoc + zsh |
| **文件诊断追踪** | 编辑前后 diff + LRU 去重 | 无 | 编辑前后 diff + LRU 去重 |
| **执行引擎** | 混合批次并发 + sibling abort | 全 safe 才并行 | 混合批次 + sibling abort + discard |
| **MCP 协议** | 完整支持 | 无 | MCP Surface |
| **架构方法论** | 工程直觉驱动 | **DSAFT 方法论驱动** | DSAFT + 博弈论机制设计 |
| **契约化** | Tool 对象 30+ 方法 | **ToolSurface 6 方法 + ToolFilter 链** | 同当前 + PerSession + UserCustom |
| **跨域编排** | 单进程 Coordinator/Worker | **7 域 Mechanism Design (D7 = Designer, D2 = Player)** | 同当前 + D6 信誉闭环 |
| **LLM 调用权** | query.ts 一体 | **D7 (Principal) 独有，D2 (Agent) 不持有 D3 引用** | 同当前 + CI import lint 强制执行 + hard token budget cap |
| **T 层验证** | 无 | **130+ T 点全量 IMPLEMENTED** | 持续扩展 |
| **可观测性** | 部分 | **D5 span + metric 完整体系** | chain_consistency 交叉验证 |

### 10.2 Devrix 独特优势（业界无对标）

1. **架构可验证性**：DSAFT T 层测试点 + 4 正交标志真值表 → 每个工具的安全属性可被 CI 验证。业界其他框架（LangChain/CrewAI/AutoGen）都依赖 code review 和运行时行为。

2. **机制设计完备性**：Principal-Agent 决策权对齐 / Mechanism Design (Myerson 1979) / Commitment Device (Schelling 1960) / Costly Signal (Spence 1973) / Screening Mechanism (Rothschild-Stiglitz 1976) / Separating Equilibrium 显式写入架构 spec。业界其他框架隐式依赖工程直觉。

3. **产权明晰**：LLM 调用权归 D7 (Principal)、工具执行权归 D2 (Agent)、安全审计权归 D3——每个关注点有明确的 Canonical Owner，形成完整的 Principal-Agent 三层代理链。业界其他框架无此概念。

---

## 11. 关联参考

- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §12（Facet Decomposition 案例）
- DSAFT 重构 Playbook：`docs/methodology/dsaft-refactoring-playbook.md`（博弈论/机制设计轴）
- 工具面契约化基线：`openspec/archive/2026-06-17-devrix-tool-surface-contract/`
- ToolSpec 正交标志设计：`openspec/archive/2026-06-18-devrix-tool-spec-enrichment/design.md`
- 诊断工具差距分析：`docs/reference/clawcode-diagnostic-tools-analysis.md`
- D7 SA Refine 博弈分析：`openspec/archive/2026-06-14-devrix-d7-sa-refine/gaming-analysis.md`
- D7 Turn 编排产权配置：`openspec/archive/2026-06-15-devrix-d7-turn-orchestration/`
- 跨域边界 SoT：`openspec/specs/architecture/cross-domain-boundaries.md`
- 架构分层 SoT：`openspec/specs/architecture/layering.md`
- 执行引擎技术债务：`openspec/tech-debt/streaming-tool-executor-v2.md`
- DM-20260616-003 诊断工具能力差距：`openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/demand.md`

---

## 12. 检查清单（S1 完成确认）

- [x] DM ID 已分配（DM-20260618-007）且无冲突
- [x] demand.md 包含背景、问题陈述、验收标准、变更范围
- [x] 至少 1 个 P0 验收标准（AC1-AC7 共 7 个 P0）
- [x] Out of Scope 已明确声明（§8）
- [x] DSAFT 域标注正确（multi-domain: D2/D3/D4/D5/D6/D7）
- [x] 跨域边界与约束已声明（§5）
- [x] 风险评估含影响与缓解（§7）
- [x] 不包含工时估算（遵循 requirements.md §6 禁止原则）
- [x] 终态架构蓝图提供了后续所有工具变更的架构对齐基准（§3）
