# clawcode 问题排查定位工具分析

**日期:** 2026-06-16
**目的:** 对照 clawcode (Claude Code v2.1.88) 的诊断工具能力，识别 devrix 待补齐的差距

---

## 1. 概述

clawcode 作为编程助手运行时，通过 40+ 内置工具让 LLM 自主排查问题。其诊断工具体系分为三层：

- **感知层**: LSP 代码智能、文件诊断追踪
- **执行层**: Bash 安全引擎、后台任务、并行子代理
- **验证层**: 计划执行验证、工作流验证

---

## 2. 核心诊断工具详解

### 2.1 LSP Tool（最大差距）

**devrix 状态：无**

9 种 Language Server Protocol 操作，让模型获得 IDE 级别的代码理解：

| 操作 | 排查场景 | 优先级 |
|------|---------|--------|
| `goToDefinition` | 跟踪变量/函数来源，理解数据流 | P0 |
| `findReferences` | 评估改动影响范围，找到所有引用点 | P0 |
| `hover` | 获取类型信息、文档注释 | P1 |
| `incomingCalls` | 找到所有调用方 — 改函数前知道谁依赖它 | P0 |
| `outgoingCalls` | 理解函数的依赖链 | P1 |
| `goToImplementation` | 找到接口的所有实现 | P0 |
| `prepareCallHierarchy` | 构建完整调用层级 | P1 |
| `documentSymbol` | 快速了解文件结构（函数/类/变量列表） | P2 |
| `workspaceSymbol` | 跨文件搜索符号 | P1 |

**技术要点：**
- 基于 `vscode-languageserver-types` 协议
- 支持多 LSP server 管理器（按文件类型路由）
- 10MB 文件大小上限
- 结果格式化输出（行号+上下文）

**devrix 实现建议：**
- 复用 `gopls`（Go）、`tsserver`（TypeScript）等现有 LSP server
- 作为 D2 Context Engine 的新 tool 插件注册
- 初期支持 `goToDefinition` + `findReferences` + `incomingCalls` 三个 P0 操作

### 2.2 Bash 安全引擎

**devrix 状态：有基础 sandbox（allowlist + deny 正则），无 AST 级别分析**

clawcode 的 `bashSecurity.ts`（2592 行）实现：

| 能力 | 说明 | devrix 对应 |
|------|------|------------|
| Tree-sitter AST 解析 | 将 bash 命令解析为 AST，结构化分析 | 无 |
| Heredoc 提取分析 | 识别 heredoc 内容并单独检查 | 无 |
| Shell 引号解析 | 检测引号嵌套 bug、特殊字符转义 | 无 |
| Zsh 专项检测 | `zmodload`、`sysopen`/`syswrite`、`=cmd` 等 20+ 种攻击面 | 无 |
| 进程替换检测 | `<()`、`>()`、`=()`、`$()`、反引号 | 部分（正则） |
| 读/写分类 | 区分只读命令和副作用命令 | 有（Risk Level） |
| 沙箱隔离 | 可选的进程沙箱 | 有（`sandbox.go`） |
| 审计日志 | 每条 bash 命令记录到 slog | 有 |
| 错误提示重定向 | 检测用户误用 bash 做 grep/glob，提示正确工具 | 有（`bashWrongToolHint`） |

**devrix 改进方向：**
- 引入 shell AST 解析（可用 `mvdan.cc/sh` Go 库），替代纯正则匹配
- 补充 Zsh 特有攻击面检测
- Heredoc 内容单独审计

### 2.3 后台任务 + 异步输出

**devrix 状态：有基础实现（`task_output`），接口较简单**

clawcode 的 `TaskOutputTool`：

- 阻塞等待（`block=true`，默认）或即时查询（`block=false`）
- 可配置超时（最长 600s）
- 跨任务类型统一接口（bash / agent / remote）
- 支持后台运行通知（任务完成时自动通知模型）

**devrix 改进方向：**
- 统一 `task_output` 接口，支持所有后台任务类型
- 增加任务完成通知机制（事件推送而非轮询）

### 2.4 实现后验证

**devrix 状态：无**

clawcode 的 `VerifyPlanExecutionTool`：
- 实现完成后自动对照计划逐项验证
- 验证失败时生成差异报告
- 与 Plan Mode 深度集成

`WorkflowTool`：
- 多步骤工作流执行
- 每步结果检验 + 失败回退

**devrix 改进方向：**
- 在 S4-Gate 阶段增加自动化验证步骤
- 将验证结果结构化输出（JSON 格式，方便 CI 集成）

### 2.5 并行子代理调查

**devrix 状态：有 `delegate_explore` 等，但基于 DAG 调度**

clawcode 的 `AgentTool` + `ForkSubagent`：

- 模型可自由分叉多个子代理同时调查不同方向
- 子代理间通过 `SendMessageTool` 通信
- 支持 worktree 隔离（`EnterWorktreeTool`）

**devrix 改进方向：**
- 增加自由分叉模式（不受 DAG 拓扑限制），用于探索性排查
- 子代理间增加直接通信 channel

### 2.6 文件诊断追踪

**devrix 状态：无**

clawcode 的 `DiagnosticTrackingService`：
- 编辑文件前拍诊断快照
- 编辑后对比，自动发现"本次改动引入的新错误"
- 跨轮次去重（500 文件 LRU 缓存）
- 支持 `file://` 和 `_claude_fs_right:` 双协议

**devrix 改进方向：**
- 在 `edit_file` / `write_file` tool 中埋点
- 编辑前后各跑一次 linter，diff 输出

---

## 3. 优先级排序

| 优先级 | 能力 | 理由 |
|--------|------|------|
| **P0** | LSP Tool（goToDefinition + findReferences + incomingCalls） | 最大差距，从根本上改变排查方式 |
| **P0** | 文件诊断追踪（编辑前后对比） | 实现成本低，即时收益高 |
| **P1** | Bash AST 级别安全分析 | 提升沙箱安全性的同时不牺牲灵活性 |
| **P1** | 实现后自动验证 | 减少 S4-Gate 人工审查负担 |
| **P2** | 后台任务通知机制 | 优化异步工作流体验 |
| **P2** | 自由分叉调查模式 | 改善复杂问题的并行排查 |

---

## 4. 参考文件

clawcode 关键源文件（解包自 `@anthropic-ai/claude-code` v2.1.88 npm 包）：

| 文件 | 内容 |
|------|------|
| `src/tools/LSPTool/LSPTool.ts` | LSP 工具实现 |
| `src/tools/LSPTool/prompt.ts` | LSP 工具 prompt 描述 |
| `src/tools/LSPTool/formatters.ts` | LSP 结果格式化 |
| `src/tools/LSPTool/schemas.ts` | LSP 输入 schema |
| `src/tools/BashTool/bashSecurity.ts` | Bash 安全引擎（2592 行） |
| `src/tools/BashTool/bashCommandHelpers.ts` | Bash 命令辅助函数 |
| `src/tools/BashTool/sedEditParser.ts` | sed 命令解析 |
| `src/tools/TaskOutputTool/TaskOutputTool.tsx` | 后台任务输出获取 |
| `src/tools/testing/TestingPermissionTool.tsx` | 测试权限工具 |
| `src/services/diagnosticTracking.ts` | 文件诊断追踪 |
| `src/services/lsp/LSPDiagnosticRegistry.ts` | LSP 诊断注册与去重 |
| `src/utils/telemetry/sessionTracing.ts` | OTEL 会话追踪 |
| `src/utils/telemetry/perfettoTracing.ts` | Perfetto 性能追踪 |
| `src/utils/telemetry/betaSessionTracing.ts` | Beta 会话追踪（内容哈希去重） |

---

## 5. 其他值得关注的 clawcode 诊断特性

| 特性 | 说明 | devrix 状态 |
|------|------|------------|
| `/doctor` 自诊断命令 | 检查安装、配置、上下文健康 | 无 |
| debug 日志分类过滤 | `--debug=api,hooks` 按类别开关 | 无 |
| 会话转录持久化 | JSONL 完整会话记录，支持 `--continue` 恢复 | 部分（incident export） |
| 故障注入 | 模拟 bridge API 故障，测试恢复路径 | 无 |
| 上下文窗口分析 | 逐类别 token 使用分解+可视化 | 无 |
| 错误分类引擎 | 20+ 标准化 API 错误类型 + 用户可操作提示 | 部分（sentinel error） |
| 堆栈截断 | `shortErrorStack(e, 5)` 截断后喂模型 | 无 |
