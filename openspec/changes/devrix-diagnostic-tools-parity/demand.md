---
demand-id: DM-20260616-003
title: 诊断工具能力差距闭环 — 对齐 clawcode (Claude Code v2.1.88)
priority: P0
status: S1_Proposal
dsaft_domain: multi-domain
created: 2026-06-16
parent_doc: docs/reference/clawcode-diagnostic-tools-analysis.md
---

# Demand: 诊断工具能力差距闭环 — 对齐 clawcode (Claude Code v2.1.88)

## 1. 背景

clawcode 作为 Claude Code v2.1.88 的运行时，提供 40+ 内置工具让 LLM 自主排查问题。
其诊断工具体系分三层：**感知层**（LSP、文件诊断追踪）、**执行层**（Bash 安全引擎、后台任务、并行子代理）、**验证层**（计划执行验证、工作流验证）。

devrix 当前已具备基础能力（沙箱 allowlist/deny 正则、`task_output`、`delegate_explore`），但与 clawcode 相比存在 **6 项核心能力差距** 和 **7 项附加诊断特性缺口**（详见 `docs/reference/clawcode-diagnostic-tools-analysis.md`）。
差距最大的是 **LSP Tool** — devrix 完全无 LSP 能力，模型只能依赖文本搜索与正则匹配；其次是 **文件诊断追踪** — 改完代码不知是否引入新错误，只能依赖后续告警或测试。

## 2. 问题陈述

### 2.1 核心差距（6 项，按优先级降序）

| ID | 能力 | clawcode 来源 | devrix 现状 |
|----|------|--------------|-------------|
| **G1** | LSP Tool — goToDefinition / findReferences / incomingCalls | `src/tools/LSPTool/LSPTool.ts` | **无** |
| **G2** | Bash AST 级别安全分析（Tree-sitter/heredoc/zsh 攻击面） | `src/tools/BashTool/bashSecurity.ts`（2592 行） | 仅 allowlist + deny 正则 |
| **G3** | 后台任务统一接口 + 完成通知（事件推送） | `src/tools/TaskOutputTool/TaskOutputTool.tsx` | 有 `task_output`，仅轮询 |
| **G4** | 实现后自动验证（VerifyPlanExecutionTool） | `src/tools/VerifyPlanExecutionTool/` | **无** |
| **G5** | 自由分叉子代理调查（不受 DAG 限制） | `src/tools/AgentTool/ForkSubagent` | `delegate_explore`，DAG 拓扑 |
| **G6** | 文件诊断追踪（编辑前后快照对比 + 去重） | `src/services/diagnosticTracking.ts` | **无** |

### 2.2 附加诊断特性（7 项）

| ID | 特性 | clawcode 来源 | devrix 现状 |
|----|------|--------------|-------------|
| **A1** | `/doctor` 自诊断命令（安装/配置/上下文健康） | 内置 slash command | **无** |
| **A2** | Debug 日志分类过滤（`--debug=api,hooks`） | CLI flag | **无** |
| **A3** | 会话转录持久化（JSONL + `--continue`） | session tracing | 部分（仅 incident export） |
| **A4** | 故障注入（模拟 bridge API 故障） | test harness | **无** |
| **A5** | 上下文窗口分析（逐类别 token 分解 + 可视化） | `/context` | **无** |
| **A6** | 错误分类引擎（20+ 标准 API 错误类型） | error classifier | 部分（sentinel error） |
| **A7** | 堆栈截断（`shortErrorStack(e, 5)`） | utils | **无** |

### 2.3 用户/Agent 痛感

- **改完代码不知是否引入新错**：每次编辑都是赌博，必须依赖后续 test/告警发现。
- **排错靠 grep + 大脑模拟调用栈**：LSP 的 `incomingCalls` 一秒能列出所有调用方，devrix 要靠多轮 Read + 记忆。
- **复杂 bash 命令沙箱误杀**：`zmodload`、`sysopen` 等 zsh 特有攻击面正则覆盖不全；heredoc 内容没单独审计。
- **S4-Gate 靠人工 Review**：实现完成没自动对照计划逐项验证，审查负担全部压在 reviewer。
- **轮询后台任务**：浪费 token，无完成通知，模型被卡住。

## 3. 验收标准

### 3.1 P0（必须达成，否则不交付）

| ID | 标准 | 验证方式 |
|----|------|---------|
| AC1 | LSP Tool 提供 `goToDefinition` / `findReferences` / `incomingCalls` 三个 P0 操作，可在 QueryLoop ToolPool 注册并在端到端 prompt 中被模型调用 | 端到端 prompt 含调用指令 → 工具被自动调度 → 结果格式化输出（行号+上下文） |
| AC2 | 文件诊断追踪支持 `edit_file` / `write_file` 前拍快照、编辑后跑 linter、diff 输出"本次改动引入的新错误"，跨轮次 500 文件 LRU 去重 | 单测 + 集成测：编辑故意引入语法错误 → diff 出现新错误；编辑无关行 → diff 为空 |
| AC3 | D2 ToolPool 注册表支持新插件即插即用（LSP、DiagnosticTracker 注册后无需修改 QueryLoop 主路径） | 单测：注册 → QueryLoop 自动发现 → 调用 |

### 3.2 P1（本期不交付但需求锁定）

| ID | 标准 |
|----|------|
| AC4 | Bash 安全引擎支持 Tree-sitter AST 解析、heredoc 内容单独审计、zsh 攻击面检测（≥20 种模式）；现有 allowlist + deny 正则作为 fallback 保留 |
| AC5 | D6 Evolution S4-Gate 增加自动化验证步骤：实现完成后对照 plan.md 逐项验证，输出结构化差异报告（JSON 格式） |
| AC6 | API 错误分类引擎映射 ≥20 种 LLM Gateway 错误码到 sentinel error 类型，附用户可操作提示 |

### 3.3 P2（本期不交付但需求锁定）

| ID | 标准 |
|----|------|
| AC7 | 后台任务（bash/agent/remote）统一 `task_output` 接口，支持 `block=true/false` + 超时 + 完成事件推送 |
| AC8 | 自由分叉模式：AgentTool 可脱离 DAG 拓扑即兴分叉 N 个子代理调查不同方向，子代理间通过 `SendMessage` 直接通信 |
| AC9 | `/doctor` 自诊断命令检查安装/配置/上下文健康，输出 JSON 报告 |
| AC10 | Debug 日志分类过滤支持 `--debug=api,hooks,telemetry` 按类别开关 |
| AC11 | 上下文窗口分析按类别分解 token 使用（system/tools/messages/thinking），可视化 |
| AC12 | 堆栈截断 `shortErrorStack(err, 5)` 在 sentinel error 链路中默认应用 |
| AC13 | 会话转录 JSONL 持久化 + `--continue` 恢复 |

### 3.4 范围/质量基线

| ID | 标准 |
|----|------|
| AC14 | 不改变 P0 T 层的 bit-identical 行为（sub-change 各自的 `legacy.enabled=false` 路径保持） |
| AC15 | 跨域新增 import 不得引入新的依赖环（layering 规则） |
| AC16 | 涉及 auth / 网关 / 沙箱的子能力必须经 `verify-security` 闸门（见 CCG 规则） |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 上游 | DM-20260608-001 devrix-tool-security（已合入，提供 ToolPool + bash sandbox 基础） |
| 上游 | DM-20260610-001..007 observability-enhancement 系列（已合入，提供 span/trace/incident 基础） |
| 上游 | DM-20260610-009..011 devrix-d6-eval-phase2/3/4（D6 评测框架就绪，可承载 AC5 的"差异报告"格式） |
| 上游 | devrix-d7-loop-first-routing（DM-20260616-002）— LSP Tool 注册需要 D7 ingress 已统一 |
| 上游 | devrix-d7-uncertainty-gaps（DM-20260616-001）— 错误分类与冲突仲裁接口已具备 |
| 约束 | 不得在 `internal/layers/<domain>/` 跨层直接调用，必须走 `internal/shared/` 横切接口 |
| 约束 | LSP server 进程管理不得绕过现有 sandbox — 必须复用 D1 sandbox |
| 约束 | 文件诊断追踪的 linter 调用走 `lifecycle/<lifecycle-id>/<sub>.go` 的 lifecycle 钩子（参见 d7-boundary） |
| 约束 | 不得修改 D2 QueryLoop / D7 Turn 编排主路径（只新增 tool 插件） |

## 5. 变更范围

### 5.1 新增

- `internal/layers/contextengine/tool/lsp/` — LSP Tool 插件（gopls/tsserver adapter）
- `internal/layers/observability/diagnose/tracker/` — 文件诊断追踪服务（500 文件 LRU）
- `internal/layers/observability/diagnose/doctor/` — `/doctor` 自诊断命令
- `internal/shared/window/` — 上下文窗口分析（按类别 token 分解）
- `internal/layers/multiagent/fork/freefork/` — 自由分叉子代理（脱离 DAG）
- `internal/layers/llmgateway/errors/` — API 错误分类引擎（20+ 映射）
- `internal/layers/communication/transcript/` — JSONL 会话转录持久化
- `openspec/specs/<domain>/spec.md` 的 ADDED Requirements（每 sub-change 一份）

### 5.2 修改

- `internal/layers/contextengine/tool/pool.go` — ToolPool 支持动态插件注册（LSP/Tracker 入口）
- `internal/shared/config/tool_config.go` — LSP server 路径、tracker LRU 容量、debug filter 配置
- `internal/layers/observability/runtime/` — debug 日志分类开关（tracer/logger 子句）
- `internal/layers/multiagent/run/agent.go` — Fork 自由模式开关 + SendMessage channel
- `internal/layers/llmgateway/dispatch/error_mapping.go` — 错误分类器接入
- `internal/shared/errors/` — `shortErrorStack` helper（AC12）
- `openspec/specs/d*-*/spec.md` — 在现有 Scenario 末尾追加 ADDED 子节（不破坏原 Scenario）

### 5.3 不变更

- **D2 QueryLoop 主路径**（详见 `devrix-queryloop-legacy-decommission` 独立 change）
- **D7 Turn 编排主路径**
- **D3 LLM Gateway 现有熔断/重试/超时逻辑**
- **D1 Communication 现有协议层**
- **D4 Fork/Join 当前 DAG 编排**（只新增 freefork 模式，DAG 路径保留 legacy.enabled）
- **现有 spec.md 的 Scenario 文本**（仅追加，不删改）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| **LSP server 进程数失控** — 用户大型 monorepo 启动多个 gopls/tsserver | OOM + 端口冲突 | 单 workspace 最多 N 个并发 server（默认 4），LRU 淘汰 |
| **文件诊断追踪的 linter 调用放大编辑延迟** | 编辑 5s → 60s | 异步化（`OnEditComplete` 后台触发），编辑主路径不阻塞 |
| **Tree-sitter AST 解析引入新依赖** | 二进制大小 +200KB，CI 编译时间上升 | 评估 `mvdan.cc/sh` 纯 Go 实现（无 CGO），作为 v1.2 sub-change 的备选 |
| **错误分类引擎映射不全导致漏分类** | 用户看到 raw error 字符串 | sentinel error fallback + 监控"未分类错误率"指标，超阈值告警 |
| **自由分叉模式破坏 D4 DAG 隔离保证** | 并发 session 串扰 | freefork 子代理默认 worktree 隔离；非 worktree 模式需 explicit opt-in + warning |
| **G1/G2 触及安全敏感代码（bash sandbox）** | 漏判 → 沙箱绕过 | AC16：必须经 verify-security 闸门，CRITICAL 级必须人工对账 |

## 7. Out of Scope

- **不实现** — 本 change 仅产出 S1 需求 + S2 提案；sub-change v1.1/v1.2/v1.3 各自分配 DM ID 走 S3-S6
- **不引入** 新 LLM provider / 新通信协议 / 新持久化后端
- **不动** D1 现有 D-S2 会话录制格式（仅在 sub-change-A3 新增 JSONL 并行轨道）
- **不覆盖** Python/Rust LSP server（仅 Go + TypeScript，符合 devrix 主流项目类型）
- **不覆盖** 非 devrix 内部使用场景（如 IDE 嵌入式使用）
- **不实现** clawcode 私有协议（如 `_claude_fs_right:` 双协议）

## 8. 关联参考

- 源分析文档：`docs/reference/clawcode-diagnostic-tools-analysis.md`
- DSAFT 方法论：`openspec/specs/project/master.md` §1.2 + `docs/methodology/dsaft-methodology.md`
- 已有诊断基线：`openspec/archive/2026-06-10-devrix-observability-enhancement/`
- 工具安全基线：`openspec/archive/2026-06-08-devrix-tool-security/`
- D6 Eval 基线（AC5 验证报告载体）：`openspec/archive/2026-06-10-devrix-d6-eval-phase4/`
- D7 编排主路径：`openspec/changes/devrix-d7-loop-first-routing/`（DM-20260616-002）+ `openspec/changes/devrix-d7-uncertainty-gaps/`（DM-20260616-001）
- D2 QueryLoop 债务（独立 change）：`openspec/changes/devrix-queryloop-legacy-decommission/`（DM-20260617-001）+ `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)
- 域归属规则：`openspec/specs/architecture/layering.md`

## 9. 检查清单（S1 完成确认）

- [x] DM ID 已分配（DM-20260616-003）
- [x] demand.md 包含背景、问题、验收标准、范围
- [x] 至少 3 个 P0 验收标准（AC1/AC2/AC3）
- [x] Out of Scope 已明确声明（§7）
- [x] DSAFT 域标注正确（multi-domain，含 D1/D2/D3/D4/D5/D6 + tool-security 横切）
- [x] 跨域边界已声明（§4 约束 + §5.3 不变更）
- [x] 风险评估含影响与缓解（§6）
- [x] D2 QueryLoop 债务独立为 `devrix-queryloop-legacy-decommission`（DM-20260617-001），不混入本 change
