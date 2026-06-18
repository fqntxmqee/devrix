# S3 Design: Devrix Tools 终态架构

**Change ID:** devrix-tools-terminal-architecture
**Demand ID:** DM-20260618-007
**Status:** S3_Design
**Source Documents:** demand.md §3（终态架构蓝图）+ proposal.md §3.1（五层正交模型）

> 本设计文档是 **需求→架构的契约**。5 个 Phase 全部规划在设计层，但实现按 Phase 1 → 1.5 → 2 → 3 顺序推进。每个 Phase 内的子 change 可独立 PR、独立回滚。

---

## 0. DSAFT 资产登记（DSAFT §九 Checklist）

本节按 DSAFT 方法论 §九要求，列出本 change 涉及的所有领域、场景、活动、功能点、测试点，作为 S3→S4→S5 的可执行契约。

### 0.1 D 领域速览（6 个）

| ID | 名称 | 类型 | 领域职责（与本 change 相关）|
|----|------|------|----------------------------|
| **D2** | 上下文引擎 | 公共 | ToolSurface 拆面契约 + 工具执行 + WorkerContext 消费方 |
| **D3** | LLM 网关 | 公共 | LLM 调用权 + 内容安全 + 错误分类（不变：物理层 D2 不持有 D3 引用）|
| **D4** | 多智能体 | 核心 | FreeFork 自由分叉 + Worktree 隔离 + 后台任务事件 |
| **D5** | 可观测性 | 公共 | Span 锚点 + DiagnosticTracker 文件诊断 + chain_consistency 交叉验证 |
| **D6** | 演化层 | 支撑 | VerifyPlanExecution 自动验证 + Causal Audit Trail 4-tuple + Reputation 衰减 |
| **D7** | 编排层 | 核心 | ToolFilter 链 FIFO + WorkerContext + turn_adapter 调度 |

### 0.2 S 场景注册表（9 个）

| S ID | 名称 | 触发条件 | 用户目标 | 涉及 A |
|------|------|---------|---------|--------|
| **D2-S16** | RunQueryLoop | D7 turn_adapter 进入 query 循环轮次 | 高效完成多轮 LLM 工具调用编排 | D2-S18-A01, A05, A06 |
| **D2-S18** | EnforceExecutionPolicy | LLM 返回 tool_use 需要执行 | 工具执行前的安全/权限/可见性校验 | D2-S18-A01, A05, A06 |
| **D2-S4** | LSP Tool Surface | LLM 调用 lsp_* 类工具 | 为 Agent 提供 LSP 级代码理解能力 | D2-S4-A01 |
| **D3-S5** | GuardContent | D3 LLM Gateway 接收 LLM 响应 | 内容安全过滤 + 错误分类 + 工具调用解析 | （不新增 A，D3 内部活动）|
| **D4-S11** | ForkAgent | LLM 调用 free_fork | 脱离 DAG 拓扑的自由分叉探索 | D4-S11-A02, D4-S13-A02 |
| **D4-S13** | IsolateWorktree | ForkAgent 创建子代理 | 子代理工作目录隔离防止污染 | D4-S13-A02 |
| **D5-S23** | Diagnose | 编辑文件后异步触发 | 收集编辑前后 diff + LRU 去重 + 异步 linter | D5-S23-A02 |
| **D6-S11** | Verify | S4 任务完成触发 | 自动化 S4-Gate，对照 tasks.md 验证 | D6-S11-A02 |
| **D7-S5** | ClassifyIntent | 用户输入进入 turn loop | 识别意图 + 构造 WorkerContext | （不新增 A，D7 内部活动）|

### 0.3 A 活动注册表（10 个，含输入/输出/状态变更）

| A ID | 名称 | Type | 输入 | 输出 | 状态变更 | 归属 Surface |
|------|------|------|------|------|---------|------------|
| D2-S4-A01 | LSP Tool 注册与调度 | A-FE | `name (string)`, `input (json.RawMessage)`, `wc (WorkerContext)` | `*ToolResult`, `error` | LSP server 进程池 LRU 状态 + 进程健康状态 | LSPSurface |
| TOOL-SEC-2-A02 | Bash AST 安全引擎 | A-BE | `command (string)`, `wc (WorkerContext)` | `*AuditDecision{Allow, Reason}` | 无（纯函数） | BashASTPolicy |
| D2-S18-A01 | CheckPermission 三态门控 | A-FE | `tool (ToolSpec)`, `action (string)`, `wc (WorkerContext)` | `Permission{Allow/Deny/Ask, Reason, ExpiresAtTurn}` | D6 授权历史追加 + 撤销协议 | 跨 Surface（所有工具）|
| D2-S18-A05 | BuildSurfaces 单入口 | A-FE | `wc (WorkerContext)`, `feature_flags` | `[]ToolSurface` | Surface 注册表查询 | ToolSurface 集合 |
| D2-S18-A06 | ExecuteToolRound 两阶段分派 | A-FE | `surfaces []ToolSurface`, `tool_uses []ToolUse`, `wc` | `[]ToolResult`, `[]error` | mixed_batch 调度状态 + sibling abort 标记 | StreamingToolExecutor |
| D5-S23-A02 | 文件诊断追踪 | A-FE | `workDir (string)`, `toolName (string)`, `input (json.RawMessage)` | `*DiffRecord` (异步) | LRU 缓存 + async channel | DiagnosticTracker |
| D4-S11-A02 | 自由分叉探索 | A-FE | `n (int)`, `directions ([]string)`, `wc` | `[]*SubAgent`, `error` | Fork 计数 + SendMessage 通道 + Resource 仲裁器状态 | FreeForkSurface |
| D4-S13-A02 | Worktree 隔离 | A-BE | `workspaceRoot (string)`, `forkID (string)` | `worktreePath (string)`, `error` | Worktree 池 LRU 状态 | FreeFork 内部 |
| D6-S11-A02 | 实现后自动验证 | A-FE | `tasksMdPath (string)`, `wc` | `*VerifyResult{Pass, Tasks[], SpanReport}` | D6 Reputation 调整 + Causal Audit Trail 写入 | VerifySurface |
| D4-S12-A03 | 后台任务事件推送 | A-FE | `taskID (string)`, `event (BGEvent)` | `error` | BGTask 状态机 + 通知队列 | BGTaskSurface |

### 0.4 F 功能点注册表（23 个，含类型/输入/输出）

| F ID | 名称 | Type | 归属 A | 输入 | 输出 |
|------|------|------|--------|------|------|
| D2-S4-A01-F01 | goToDefinition | F-BE | D2-S4-A01 | `{uri, line, character}` | `[]Location` (1-2s 超时) |
| D2-S4-A01-F02 | findReferences | F-BE | D2-S4-A01 | `{uri, line, character, includeDeclaration}` | `[]Location` |
| D2-S4-A01-F03 | incomingCalls | F-BE | D2-S4-A01 | `{uri, line, character}` | `[]CallHierarchyItem` |
| D2-S4-A01-F04 | hover | F-BE | D2-S4-A01 | `{uri, line, character}` | `{contents: {kind: "markdown", value: string}}` |
| D2-S4-A01-F05 | workspaceSymbol | F-BE | D2-S4-A01 | `{query}` | `[]SymbolInformation` |
| D2-S4-A01-F06 | LSP server 进程池（LRU） | F-BE | D2-S4-A01 | `(workspaceRoot)` | `*lsp.Server, error` |
| TOOL-SEC-2-A02-F01 | AST 解析（mvdan.cc/sh） | F-BE | TOOL-SEC-2-A02 | `(command string)` | `*ast.File, error` |
| TOOL-SEC-2-A02-F02 | heredoc 审计 | F-BE | TOOL-SEC-2-A02 | `(ast *ast.File)` | `*HeredocAuditResult` |
| TOOL-SEC-2-A02-F03 | zsh 攻击面规则集 | F-BE | TOOL-SEC-2-A02 | `(ast *ast.File)` | `*RuleMatch{Allow, RuleName, Reason}` |
| D5-S23-A02-F01 | 编辑前后 diff 收集 | F-BE | D5-S23-A02 | `(workDir, toolName, input)` | `*DiffRecord` |
| D5-S23-A02-F02 | LRU 去重 | F-BE | D5-S23-A02 | `(*DiffRecord)` | `bool (duplicate?)` |
| D5-S23-A02-F03 | 异步触发器 | F-BE | D5-S23-A02 | `(editEvent)` | `(fire-and-forget)` |
| D4-S11-A02-F01 | ForkAgent 创建 | F-BE | D4-S11-A02 | `(n, directions[], wc)` | `[]*SubAgent` |
| D4-S11-A02-F02 | SendMessage 通道 | F-BE | D4-S11-A02 | `(from, to, content)` | `error` |
| D4-S11-A02-F03 | fork 资源争抢仲裁 | F-BE | D4-S11-A02 | `(resource Resource, requester SubAgentID)` | `Decision{Wait/Release/Abort}` |
| D4-S13-A02-F01 | worktree 隔离 | F-BE | D4-S13-A02 | `(workspaceRoot, forkID)` | `(worktreePath string, cleanup func(), error)` |
| D6-S11-A02-F01 | tasks.md 解析 | F-BE | D6-S11-A02 | `(path string)` | `[]Task{name, criteria[]}` |
| D6-S11-A02-F02 | 验证项执行编排 | F-BE | D6-S11-A02 | `([]Task)` | `[]VerifyItemResult` |
| D6-S11-A02-F03 | 结果聚合 + pass/fail 报告 | F-BE | D6-S11-A02 | `([]VerifyItemResult)` | `*VerifyResult{Pass, Report, SpanReport}` |
| D4-S12-A03-F01 | 事件流推送 | F-BE | D4-S12-A03 | `(taskID, event)` | `error` |
| D2-S18-A06-F01 | 混合批次调度 | F-BE | D2-S18-A06 | `([]ToolUse)` | `([]BatchGroup)` |
| D2-S18-A06-F02 | sibling abort | F-BE | D2-S18-A06 | `(triggeringResult, siblingBatch)` | `([]AbortedResults)` |
| D2-S18-A06-F03 | fallback discard | F-BE | D2-S18-A06 | `(failedResult, fallbackPlan)` | `(*ToolResult, error)` |

> **注**：本 change 的所有 F 均为 F-BE（后端逻辑单元），无 F-FE（前端模块）。ToolSurface 调用层属 D7 编排层，UI 交互由 D1 通信层独立处理。

### 0.5 T 测试点注册表（25 个 Phase 1 P0 T 点）

详见 §8.x T 测试点注册表章节（在 §8 Decision 之前）。

---

## 1. 架构目标

### 1.1 业务目标

| # | 目标 | 验证指标 | 归属 Phase |
|---|------|---------|-----------|
| G1 | Agent 拥有 LSP 级代码理解能力 | LSP 4 P0 操作可用，hover/findReferences/goToDef/incomingCalls 全部 T01-T04 PASS | Phase 1 |
| G2 | Bash 执行有 AST 级安全保证 | BashAST 拦截 20+ zsh 攻击面 + heredoc 审计 | Phase 1 |
| G3 | 编辑后即时诊断反馈 | DiagnosticTracker 编辑前后 diff + LRU 去重 | Phase 1 |
| G4 | 支持自由分叉探索 | FreeFork 并发 8 + SendMessage + worktree 隔离 | Phase 1 |
| G5 | 实现后自动验证 | VerifyPlanExecution 对照 tasks.md 自动化 S4-Gate | Phase 1 |
| G6 | MCP 多中心相变有完整机制设计 | Capability Attestation + Reputation Budget + Cross-Validation + Decay | Phase 1.5 → Phase 2 |
| G7 | LTL-Lite 不变式规约框架可用 | 每个 Surface 有 `_invariant.go`，CI lint 验证 | Phase 1.5 |
| G8 | 工具可见性策略稳定 | PerAgent+PerRisk+PerSession+PlanMode+UserCustom FIFO 链 subgame-perfect | 既有 + Phase 2 强化 |
| G9 | 跨域审计可追溯 | Causal Audit Trail 4-tuple 可查询 | Phase 2 |
| G10 | 端到端延迟可控 | StreamingToolExecutor 混合批次 + sibling abort | Phase 2 |

### 1.2 技术目标

- **零架构回归**：所有现有 P0 T 点保持 PASS（AC7）
- **零 import 跨域**：D2 不持有 D3 引用，CI import lint 强制（AC6）
- **零信号伪造**：ToolSpec 4 bool 来自硬编码真值表，duration_ms 来自 D7 wall clock
- **零 hidden coupling**：所有跨层调用通过 `contracts/` 或 `bridges/`
- **零估算承诺**：design.md 不含工时；tasks.md 中估算仅作参考

### 1.3 约束

| 约束 | 来源 | 影响 |
|------|------|------|
| D2 不持有 D3 引用 | Principal-Agent 决策权与成本对齐 | 所有 LLM 调用必须经 D7 |
| Surface 数量上限 ~12 | §3 终态架构图 | 新 Surface 必须通过 AC25 异质性门槛 |
| 自由分叉并发 ≤ 8 | 单 machine 内存/CPU 限制（非博弈论最优） | FreeFork 硬上限 |
| CheckPermission 承诺仅当前 turn 有效 | §2.5 不可逆性 + 撤销协议 | Allow 跨 turn 必须重新授权 |
| Hard token budget cap | D7 防止二级代理问题 | D7 自身 LLM 调用有上限 |
| Filter 链 FIFO subgame-perfect | invariant #6 | 新 filter 插入必须证明不破坏 |

---

## 2. 架构原则

### 2.1 DSAFT 强制归属

每个新增能力必须先回答三个问题：

1. **D 归属**：核心 / 支撑 / 公共？涉及几个 D？跨 D 时必须有 `contracts/` 接口
2. **S 归属**：进入哪个现有 Scenario？是否需要新建？
3. **A 归属**：对外暴露什么 A？输入/输出/状态变更？

> **完整 A/F 字段（输入/输出/状态变更/类型）见 §0.3 / §0.4 DSAFT 资产登记**。本表只补充 **Phase 排序视角**（用于 S4 任务拆分）。

**S3 本 change 新增的 A**（按 Phase 排序，来自 `.openspec.yaml`）：

| A ID | Name | Phase | Type | 归属 Surface |
|------|------|-------|------|------------|
| D2-S4-A01 | LSP Tool 注册与调度 | Phase 1 | A-FE | LSPSurface |
| TOOL-SEC-2-A02 | Bash AST 安全引擎 | Phase 1 | A-BE | BashASTPolicy |
| D2-S18-A01 | CheckPermission 三态门控 | **既有**（DM-20260618-002） | A-FE | 跨 Surface |
| D2-S18-A05 | BuildSurfaces 单入口 | **既有** | A-FE | ToolSurface 集合 |
| D2-S18-A06 | ExecuteToolRound 两阶段分派 | Phase 2 | A-FE | StreamingToolExecutor |
| D5-S23-A02 | 文件诊断追踪 | Phase 1 | A-FE | DiagnosticTracker |
| D4-S11-A02 | 自由分叉探索 | Phase 1 | A-FE | FreeForkSurface |
| D4-S13-A02 | Worktree 隔离 | Phase 1 | A-BE | FreeFork 内部 |
| D6-S11-A02 | 实现后自动验证 | Phase 1 | A-FE | VerifySurface |
| D4-S12-A03 | 后台任务事件推送 | Phase 1 | A-FE | BGTaskSurface |

**S3 本 change 新增的 F**（按 Phase 1 关键路径，完整字段见 §0.4）：

| F ID | Name | 归属 A | Phase |
|------|------|--------|-------|
| D2-S4-A01-F01 | goToDefinition | D2-S4-A01 | Phase 1 |
| D2-S4-A01-F02 | findReferences | D2-S4-A01 | Phase 1 |
| D2-S4-A01-F03 | incomingCalls | D2-S4-A01 | Phase 1 |
| D2-S4-A01-F04 | hover | D2-S4-A01 | Phase 1 |
| D2-S4-A01-F05 | workspaceSymbol | D2-S4-A01 | Phase 1 |
| D2-S4-A01-F06 | LSP server 进程池（LRU 淘汰） | D2-S4-A01 | Phase 1 |
| TOOL-SEC-2-A02-F01 | AST 解析（mvdan.cc/sh） | TOOL-SEC-2-A02 | Phase 1 |
| TOOL-SEC-2-A02-F02 | heredoc 审计 | TOOL-SEC-2-A02 | Phase 1 |
| TOOL-SEC-2-A02-F03 | zsh 攻击面规则集 | TOOL-SEC-2-A02 | Phase 1 |
| D5-S23-A02-F01 | 编辑前后 diff 收集 | D5-S23-A02 | Phase 1 |
| D5-S23-A02-F02 | LRU 去重 | D5-S23-A02 | Phase 1 |
| D5-S23-A02-F03 | 异步触发器（不阻塞编辑） | D5-S23-A02 | Phase 1 |
| D4-S11-A02-F01 | ForkAgent 创建 | D4-S11-A02 | Phase 1 |
| D4-S11-A02-F02 | SendMessage 通道 | D4-S11-A02 | Phase 1 |
| D4-S11-A02-F03 | fork 资源争抢仲裁 | D4-S11-A02 | Phase 1 |
| D4-S13-A02-F01 | worktree 隔离 | D4-S13-A02 | Phase 1 |
| D6-S11-A02-F01 | tasks.md 解析 | D6-S11-A02 | Phase 1 |
| D6-S11-A02-F02 | 验证项执行编排 | D6-S11-A02 | Phase 1 |
| D6-S11-A02-F03 | 结果聚合 + pass/fail 报告 | D6-S11-A02 | Phase 1 |
| D4-S12-A03-F01 | 事件流推送 | D4-S12-A03 | Phase 1 |
| D2-S18-A06-F01 | 混合批次调度 | D2-S18-A06 | Phase 2 |
| D2-S18-A06-F02 | sibling abort | D2-S18-A06 | Phase 2 |
| D2-S18-A06-F03 | fallback discard | D2-S18-A06 | Phase 2 |

### 2.2 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| Surface 名 | `<Domain>Surface` | `LSPSurface`, `FreeForkSurface` |
| F 名 | 动词短语 | `goToDefinition`, `findReferences` |
| Filter 名 | `<Scope>Filter` | `PerAgentFilter`, `PerSessionFilter` |
| 不变式文件 | `<surface_dir>/_invariant.go` | `lsp/_invariant.go` |
| Spec 目录 | `specs/<surface>/spec.md` | `specs/lsp-surface/spec.md` |
| 错误 Sentinel | `Err<Module><Action>` | `ErrLSPTimeout`, `ErrBashASTDenied` |

### 2.3 不变式（继承自 demand.md §3.2）

| # | 不变式 | 验证 |
|---|--------|------|
| 1 | 拆面单一职责 | `devrix tool list` 按 surface 分组 |
| 2 | Filter 纯函数 | 单测覆盖，无 I/O |
| 3 | 可见性 ≠ 可执行性 | Filter 与 CheckPermission 独立变更 |
| 4 | 信号不可伪造 | CI 真值表 lint + T 层 spot-check |
| 5 | T 锚点全覆盖 | P0 T 100% PASS |
| 6 | Filter 链 FIFO subgame-perfect | S3-Gate 审查新 filter |

---

## 3. 业务流程

### 3.1 Phase 1 核心用例

#### 3.1.1 LSP 代码理解流

```
User: "重构 OrderService 的 calculateTotal"
  ↓
D7 Turn → ClassifyIntent → code-refactor
  ↓
D7 WorkerContext{Goal: refactor, FileHints: [OrderService.go]}
  ↓
D7 turn_adapter.Prepare → ToolFilter.Apply → 包含 LSPSurface
  ↓
D3 LLM Gateway → LLM 推理
  ↓
LLM: tool_use = {name: "lsp_findReferences", input: {uri: "OrderService.go", symbol: "calculateTotal"}}
  ↓
D7 turn_adapter.Dispatch → LSPSurface.Execute → goToDefinition
  ↓
D2 LSPAdapter.CallLSP(gopls, method=goToDefinition) → position
  ↓
result: [{uri: "OrderService.go:42", range: ...}]
  ↓
D5 span: lsp.go_to_def → D7 RoundAggregator → D3 LLM 下一轮
```

**异常路径**：
- LSP server 不可用 → fallback 到 grep + read_file（降级但可用）
- LSP 超时 (> 2s) → 异步返回 partial result，UI 显示"加载中"
- LSP 进程崩溃 → LRU 池自动重启，下次调用重试

#### 3.1.2 Bash AST 安全审计流

```
LLM: tool_use = {name: "bash", input: {command: "rm -rf /tmp/*"}}
  ↓
D7 turn_adapter.Dispatch → BashSurface.Execute
  ↓
BashASTPolicy.Audit(command)
  ├─ AST 解析（mvdan.cc/sh）
  ├─ heredoc 审计
  ├─ zsh 攻击面规则集匹配
  └─ 返回 Decision{Deny, Reason: "rm -rf 模式 + 路径通配"}
  ↓
若 Allow: bash sandbox 执行
若 Deny: 返回 error + 建议（"用更精确的路径模式"）
```

**异常路径**：
- AST 解析失败 → 拒绝执行（fail-closed）
- 规则集冲突 → 选最严格规则
- heredoc 嵌套过深 → 拒绝 + 警告

#### 3.1.3 自由分叉探索流

```
User: "为什么 Service 启动慢？"
  ↓
LLM: tool_use = {name: "free_fork", input: {n: 3, directions: ["DB query", "Goroutine leak", "Network IO"]}}
  ↓
FreeForkSurface.Execute
  ├─ 创建 3 个子代理（每个独立 worktree）
  ├─ 每个子代理 worker_context = {Goal: explore_<direction>, ...}
  ├─ 子代理间 SendMessage 通道建立
  └─ 主代理等待 N 个子代理完成（或 timeout 60s）
  ↓
子代理并行探索
  ↓
结果聚合 → 3 个子代理报告 → 主代理 D3 LLM 推理 → 总结
```

**异常路径**：
- 并发达到 8 上限 → 排队等待
- 子代理超时 → 中止 + 返回部分结果
- 子代理间文件锁冲突 → fork 资源争抢仲裁（文件锁优先级 + LSP server 公平调度）

#### 3.1.4 实现后自动验证流

```
S4 任务完成
  ↓
VerifySurface.Execute
  ├─ 读取 tasks.md 当前 change 的 task list
  ├─ 对每个 task 执行 verify 项
  │   ├─ 类型 A: go test -race ./...
  │   ├─ 类型 B: go vet + gofmt
  │   ├─ 类型 C: P0 T 点全绿
  │   └─ 类型 D: CI lint 通过
  └─ 聚合结果 → pass/fail 报告
  ↓
若全 pass: S4-Gate 自动通过
若 fail: 返回失败项列表 + 修复建议
```

### 3.2 Phase 1.5 LTL-Lite 规约流

```go
// specs/lsp-surface/_invariant.go
package lsp

// LTL-Lite 不变式规约（Go struct tag DSL）
type LSPSurfaceInvariants struct {
    // 1. ReadOnly → 不可修改文件
    ReadOnlyImpliesNoMutation string `invariant:"read_only => !modifies_files"`
    
    // 2. LSP server 数量上限
    LSPServerCountBounded string `invariant:"lsp_servers <= 4"`
    
    // 3. LSP 超时必须有 fallback
    LSPTimeoutHasFallback string `invariant:"timeout => fallback(grep)"`
    
    // 4. hover 必须返回 markdown
    HoverReturnsMarkdown string `invariant:"hover_result.format = markdown"`
    
    // 5. 并发安全
    ConcurrencySafe string `invariant:"!modifies_lsp_server_state"`
}
```

**CI lint 规则**：
- 每个 Surface 目录必须存在 `_invariant.go`
- 每个 invariant 必须是合法格式 `<precondition> => <postcondition>`
- 重复 invariant（不同 Surface 间冲突）必须在 S3-Gate 审查

### 3.3 Phase 2 MCP 多中心相变流

```
MCP server 注册
  ↓
CapabilityAttestation.Verify(signed_metadata)
  ├─ 签名验证（Ed25519 + TUF root key）
  ├─ 声明能力 vs 实际调用一致性监测
  └─ 偏离检测 → 降权触发
  ↓
MCP server 获得初始 Reputation Budget (e.g., 100)
  ↓
每次调用消耗 budget（RiskLevel 越高消耗越快）
  ↓
Reputation Decay: budget *= exp(-λ * Δt)
  ├─ λ 与 server 的 RiskLevel 成正比
  └─ λ = 0.01 * (1 + RiskLevel)
  ↓
Cross-Validation：关键操作 (Destructive || OpenWorld) → 2 个 MCP server 结果比对
  └─ 不一致 → Devrix 内置工具结果胜出
  ↓
Causal Audit Trail：每次破坏性操作 → 4-tuple 记录
  └─ (tool, filter, permission_gate, llm_tool_use_id)
```

---

## 4. 领域模型

### 4.1 核心聚合根

```go
// contracts/tool/spec.go（既有，扩展）
type ToolSpec struct {
    Name           string         // e.g., "lsp_findReferences"
    Description    string
    Parameters     string         // JSON schema
    Risk           types.RiskLevel
    ReadOnly       bool           // 4 正交标志 1/4
    Destructive    bool           // 4 正交标志 2/4
    OpenWorld      bool           // 4 正交标志 3/4
    ConcurrencySafe bool          // 4 正交标志 4/4
    DeferLoading   bool           // DM-20260618-003
}

// contracts/tool/surface.go（既有，扩展）
type ToolSurface interface {
    Name() string
    Tools(ctx, workDir) []ToolSpec
    Execute(ctx, name string, input json.RawMessage, worker *WorkerContext) (*ToolResult, error)
    OrthogonalFlags() map[string]OrthogonalFlag  // 真值表
}
```

### 4.2 新增聚合根

```go
// internal/layers/contextengine/lsp/aggregate.go (Phase 1, NEW)
type LSPAggregate struct {
    workspaceRoot string
    serverPool    *LRUServerPool  // D2-S4-A01-F06
    fallback      *GrepFallback   // 降级
}

// internal/layers/contextengine/enforce/bash/ast_policy.go (Phase 1, NEW)
type BashASTPolicy struct {
    ruleSet    *ZshRuleSet           // TOOL-SEC-2-A02-F03
    astParser  *mvdanParser          // TOOL-SEC-2-A02-F01
    heredocAud *HeredocAuditor       // TOOL-SEC-2-A02-F02
}

// internal/layers/observability/diagnose/tracker/aggregate.go (Phase 1, NEW)
type DiagnosticTracker struct {
    preEditSnapshots  *LRU  // 编辑前状态
    postEditDiffs     *LRU  // 编辑后 diff
    asyncTrigger      chan<- EditEvent
}

// internal/layers/multiagent/provision/freefork/aggregate.go (Phase 1, NEW)
type FreeForkAggregate struct {
    maxConcurrent int               // 8
    worktreePool  *WorktreePool     // D4-S13-A02-F01
    messageBus    *SendMessageBus   // D4-S11-A02-F02
    arbiter       *ResourceArbiter  // D4-S11-A02-F03
}

// internal/layers/evolution/guard/verify_plan_execution/aggregate.go (Phase 1, NEW)
type VerifyAggregate struct {
    tasksParser  *TasksParser          // D6-S11-A02-F01
    executor     *VerifyExecutor       // D6-S11-A02-F02
    reporter     *VerifyReporter       // D6-S11-A02-F03
}
```

### 4.3 限界上下文

| 限界上下文 | 归属 | 职责 | 不应跨入 |
|-----------|------|------|---------|
| Tool Spec Registry | D2 | ToolSpec 注册/查询 | 不持有 LLM 调用权 |
| Tool Surface Pool | D2 | Surface 集合 + BuildSurfaces | 不执行工具 |
| Tool Execution Engine | D2 | StreamingToolExecutor 混合批次 | 不解析 LLM tool_use |
| CheckPermission Gate | D2 | 三态门控 | 不持久化承诺 |
| WorkerContext | D7 | Goal + FileHints + Constraints | 不修改工具列表 |
| ToolFilter Chain | D7 | 5 层 filter 组合 | 不执行工具 |
| D3 LLM Gateway | D3 | 内容安全 + 错误分类 | 不持有工具引用 |
| D5 Span | D5 | 跨域观测锚点 | 不参与业务逻辑 |
| D6 Reputation | D6 | 信誉存储 + 保守路由 | 不参与 LLM 决策 |
| MCP Capability Attestation | Phase 2 | 签名验证 | 不执行 MCP 工具 |
| LTL-Lite Monitor | Phase 1.5 | 不变式运行时校验 | 不做 model checking |

---

## 5. 核心链路

### 5.1 端到端：LSP 代码理解 + 重构

```
User Input
  ↓
D1 Inbound (IM/CLI/API) → D1 Bus
  ↓
D7 Turn Loop
  ├─ ClassifyIntent → code-refactor
  ├─ BuildWorkerContext{Goal, FileHints, Constraints}
  ├─ ToolFilter.Apply → LSPSurface + EditSurface 可见
  ├─ D3 LLM Gateway.CallLLM
  └─ Parse tool_use = lsp_findReferences
  ↓
D7 turn_adapter.Dispatch
  ├─ CheckPermission (read_only=true → Allow)
  ├─ LSPSurface.Execute → goToDefinition (D2-S4-A01-F01)
  └─ ToolResult → D7 RoundAggregator
  ↓
D5 span: d7.turn → d7.route.decision → d2.lsp.go_to_def → llm.next_round
  ↓
LLM 下一轮: 基于 LSP 结果生成 edit_file
  ↓
D7 turn_adapter.Dispatch
  ├─ CheckPermission (destructive=true → Ask → Allow)
  ├─ EditSurface.Execute → edit_file
  ├─ DiagnosticTracker.OnEdit (D5-S23-A02-F01) ← 异步触发
  └─ ToolResult
  ↓
D5 span: d2.tool.edit → d5.diagnostic.diff (async)
  ↓
D5 chain_consistency: d7.turn ↔ d2.tool.edit ↔ d5.diagnostic.diff
  ↓
D6 Reputation: 检测 LSP server 稳定性 → 触发 L3 保守路由（如需要）
```

### 5.2 关键时序约束

| 约束 | 时间 | 触发 |
|------|------|------|
| LSP 查询超时 | 2s | → fallback grep |
| Bash AST 解析超时 | 100ms | → 拒绝（fail-closed） |
| FreeFork 并发上限 | 8 | → 排队 |
| ToolFilter.Apply 累计 | < 50ms | → LLM round-trip 不阻塞 |
| CheckPermission 查询 | < 10ms | → 不可阻塞 turn |
| DiagnosticTracker 异步 | 不阻塞编辑 | → fire-and-forget |
| LTL-Lite 运行时 assert | < 5ms/turn | → S3-Gate 验证 |
| MCP Attestation 验证 | < 50ms | → 启动时一次性 |

---

## 6. 接口 / API 设计

### 6.1 新增 Surface 接口（Phase 1）

```go
// internal/layers/contextengine/lsp/surface.go
type LSPSurface struct {
    aggregate *LSPAggregate
}

func (s *LSPSurface) Name() string { return "lsp" }

func (s *LSPSurface) Tools(ctx context.Context, workDir string) []contracts.ToolSpec {
    return []contracts.ToolSpec{
        {Name: "lsp_goToDefinition", ReadOnly: true, ConcurrencySafe: true, Risk: types.Low, ...},
        {Name: "lsp_findReferences", ReadOnly: true, ConcurrencySafe: true, Risk: types.Low, ...},
        {Name: "lsp_incomingCalls", ReadOnly: true, ConcurrencySafe: true, Risk: types.Low, ...},
        {Name: "lsp_hover", ReadOnly: true, ConcurrencySafe: true, Risk: types.Low, ...},
        {Name: "lsp_workspaceSymbol", ReadOnly: true, ConcurrencySafe: true, Risk: types.Low, ...},
    }
}

func (s *LSPSurface) Execute(ctx context.Context, name string, input json.RawMessage, wc *contracts.WorkerContext) (*contracts.ToolResult, error) {
    switch name {
    case "lsp_goToDefinition":
        return s.aggregate.goToDefinition(ctx, input, wc)
    // ...
    }
}

// internal/layers/contextengine/enforce/bash/ast_policy.go
type BashASTPolicy struct{ /* ... */ }

func (p *BashASTPolicy) Audit(ctx context.Context, command string) (*AuditDecision, error) {
    ast, err := p.astParser.Parse(command)
    if err != nil {
        return &AuditDecision{Allow: false, Reason: "AST parse failed: " + err.Error()}, nil  // fail-closed
    }
    if p.heredocAud.HasNestedHeredoc(ast) {
        return &AuditDecision{Allow: false, Reason: "nested heredoc denied"}, nil
    }
    if p.ruleSet.Match(ast) {
        return &AuditDecision{Allow: false, Reason: p.ruleSet.MatchedRule()}, nil
    }
    return &AuditDecision{Allow: true}, nil
}
```

### 6.2 既有 Surface 扩展

```go
// internal/layers/contextengine/enforce/toolrunner/surface/builtin.go
// EditSurface 增加 DiagnosticTracker 集成
type EditSurface struct {
    tracker *diagnose.DiagnosticTracker  // NEW dependency
}

func (s *EditSurface) Execute(ctx, name, input, wc) (*ToolResult, error) {
    result, err := s.legacyExecute(ctx, name, input, wc)
    if err == nil && (name == "edit_file" || name == "write_file") {
        s.tracker.OnEdit(wc.WorkDir, name, input)  // 异步触发
    }
    return result, err
}
```

### 6.3 D6 信誉闭环接口（既有 + Phase 2 强化）

```go
// internal/layers/evolution/guard/reputation/aggregate.go
// 既有：Reputation → L3 conservative routing
// Phase 2: + Reputation Decay + Causal Audit Trail
type ReputationAggregate struct {
    store   *LRUStore
    decayFn func(server string, dt time.Duration) float64
    audit   *CausalAuditTrail  // Phase 2
}

func (r *ReputationAggregate) Apply(ctx, action Action) Decision {
    score := r.store.Get(action.ServerID)
    decayed := r.decayFn(action.ServerID, time.Since(action.LastUpdate))
    // ...
}

type CausalAuditTrail struct {
    store *AppendOnlyLog  // (tool, filter, gate, llm_id, ts)
}

func (c *CausalAuditTrail) Record(action *Action) {
    c.store.Append(Tuple4{
        Tool: action.ToolName,
        Filter: action.FilterName,
        Gate: action.GateResult,
        LLMID: action.LLMCallID,
    })
}
```

### 6.4 LTL-Lite 不变式规约接口（Phase 1.5）

```go
// internal/layers/contextengine/enforce/ltllite/spec.go (NEW)
package ltllite

import "reflect"

// Invariant 表示一条 LTL-Lite 规约
type Invariant struct {
    Name        string
    Pre         string  // "read_only"
    Operator    string  // "=>"
    Post        string  // "!modifies_files"
    Validate    func(state SystemState) bool
}

type InvariantSet struct {
    invariants []Invariant
}

func ParseFromStructTag(s interface{}) (*InvariantSet, error) {
    t := reflect.TypeOf(s)
    set := &InvariantSet{}
    for i := 0; i < t.NumField(); i++ {
        f := t.Field(i)
        tag := f.Tag.Get("invariant")
        if tag == "" { continue }
        inv, err := parseInvariant(f.Name, tag)
        if err != nil { return nil, err }
        set.invariants = append(set.invariants, inv)
    }
    return set, nil
}

func (s *InvariantSet) Check(state SystemState) []Violation {
    var vs []Violation
    for _, inv := range s.invariants {
        if !inv.Validate(state) {
            vs = append(vs, Violation{Invariant: inv.Name, State: state})
        }
    }
    return vs
}
```

### 6.5 MCP Surface 接口（Phase 2）

```go
// internal/layers/contextengine/mcp/surface.go (Phase 2, NEW)
type MCPSurface struct {
    attestation *AttestationRegistry  // AC22
    budget      *ReputationBudget     // AC23
    crossVal    *CrossValidator       // AC24
    decayFn     DecayFunction         // AC29
}

func (s *MCPSurface) Execute(ctx, name, input, wc) (*ToolResult, error) {
    server, tool := s.parseMCPName(name)
    
    // AC22: Attestation check
    if err := s.attestation.Verify(server); err != nil {
        return nil, ErrMCPServerUntrusted
    }
    
    // AC23: Budget check
    if !s.budget.HasBudget(server) {
        return nil, ErrMCPBudgetExhausted
    }
    
    // AC24: Cross-Validation for critical ops
    if tool.IsCritical() {
        return s.crossVal.Execute(ctx, server, tool, input, wc)
    }
    
    // Normal execution
    result, err := s.executeRaw(ctx, server, tool, input, wc)
    s.budget.Consume(server, tool.RiskLevel)
    return result, err
}
```

---

## 7. 文件清单（按 Phase 分组）

### 7.1 Phase 1 新增（5 个独立子 change）

| 子 change | 新增文件 | 修改文件 |
|-----------|---------|---------|
| `feat/lsp-tool-surface` | `internal/layers/contextengine/lsp/{surface,aggregate,lsp_adapter,server_pool,fallback}.go`<br>`internal/layers/contextengine/lsp/_invariant.go`<br>`specs/lsp-surface/spec.md`<br>`internal/layers/contextengine/lsp/lsp_test.go` | `internal/layers/contextengine/enforce/toolrunner/build_surfaces.go`<br>`openspec/specs/d2-context-engine/a-registry.md` (注册 D2-S4-A01) |
| `feat/bash-ast-security` | `internal/layers/contextengine/enforce/bash/{ast_policy,rule_set,heredoc_auditor}.go`<br>`internal/layers/contextengine/enforce/bash/_invariant.go`<br>`specs/bash-ast-policy/spec.md`<br>`internal/layers/contextengine/enforce/bash/ast_policy_test.go` | `internal/layers/contextengine/enforce/bash/surface.go` (集成 ASTPolicy) |
| `feat/diagnostic-tracker` | `internal/layers/observability/diagnose/tracker/{aggregate,diff_collector,lru_cache,async_trigger}.go`<br>`internal/layers/observability/diagnose/tracker/_invariant.go`<br>`specs/diagnostic-tracker/spec.md`<br>`internal/layers/observability/diagnose/tracker/tracker_test.go` | `internal/layers/contextengine/enforce/toolrunner/surface/builtin.go` (EditSurface) |
| `feat/free-fork-explore` | `internal/layers/multiagent/provision/freefork/{aggregate,worktree_pool,message_bus,resource_arbiter}.go`<br>`internal/layers/multiagent/provision/freefork/_invariant.go`<br>`internal/layers/multiagent/provision/isolateworktree/{pool,isolate}.go`<br>`specs/free-fork/spec.md`<br>`specs/worktree-isolation/spec.md`<br>`internal/layers/multiagent/provision/freefork/freefork_test.go` | `internal/layers/multiagent/provision/provision.go` (注册 FreeForkSurface)<br>`internal/layers/orchestration/turn_adapter.go` (添加 fork 路由) |
| `feat/verify-plan-execution` | `internal/layers/evolution/guard/verify_plan_execution/{aggregate,tasks_parser,executor,reporter}.go`<br>`internal/layers/evolution/guard/verify_plan_execution/_invariant.go`<br>`specs/verify-plan-execution/spec.md`<br>`internal/layers/evolution/guard/verify_plan_execution/verify_test.go` | `internal/layers/evolution/guard/verify/runner.go` |

### 7.2 Phase 1.5 新增（2 个子 change）

| 子 change | 新增文件 | 修改文件 |
|-----------|---------|---------|
| `feat/ltl-lite-invariant` | `internal/layers/contextengine/enforce/ltllite/{spec,parser,monitor}.go`<br>`specs/ltl-lite-invariant/spec.md`<br>`tools/ci-lint-invariant/main.go`<br>`internal/layers/contextengine/enforce/ltllite/ltllite_test.go` | `.github/workflows/ci.yml` (新增 lint 步骤)<br>`internal/layers/contextengine/enforce/toolrunner/build_surfaces.go` (启动时验证 invariants) |
| `feat/mcp-mechanism-design` | `docs/methodology/mcp-mechanism-design.md` (研究文档)<br>`openspec/specs/architecture/mcp-attestation-spec.md` (设计草图) | （仅设计研究，不实现） |

### 7.3 Phase 2 新增（3 个子 change）

| 子 change | 新增文件 | 修改文件 |
|-----------|---------|---------|
| `feat/streaming-executor-v2` | `internal/layers/contextengine/enforce/toolrunner/executor/{streaming,mixed_batch,sibling_abort,fallback_discard}.go`<br>`specs/streaming-executor-v2/spec.md`<br>`internal/layers/contextengine/enforce/toolrunner/executor/streaming_test.go` | `internal/layers/contextengine/enforce/toolrunner/executor/executor.go` |
| `feat/per-session-filter` | `internal/layers/orchestration/filter_chain/persession/{filter,state_store}.go`<br>`specs/per-session-filter/spec.md`<br>`internal/layers/orchestration/filter_chain/persession/persession_test.go` | `internal/layers/orchestration/filter_chain/chain.go` |
| `feat/mcp-surface` | `internal/layers/contextengine/mcp/{surface,attestation,budget,cross_val,decay}.go`<br>`internal/layers/contextengine/mcp/_invariant.go`<br>`internal/layers/evolution/guard/causal_audit/{trail,store}.go`<br>`specs/mcp-surface/spec.md`<br>`specs/causal-audit-trail/spec.md`<br>`internal/layers/contextengine/mcp/mcp_test.go` | `internal/layers/contextengine/enforce/toolrunner/build_surfaces.go`<br>`internal/layers/evolution/guard/reputation/aggregate.go` (集成 Decay) |

### 7.4 Phase 3 新增（远期，预留接口）

- `feat/web-tools-surface`（web_fetch / web_search）
- `feat/context-window-analysis`（prompt 组成分析）
- `feat/doctor-self-diagnose`（/doctor 命令）
- `feat/session-transcript`（会话转录 + --continue）
- `feat/fault-injection`（故障注入 + 错误分类引擎）
- `feat/tool-attention-heatmap`（工具注意力热力图）

文件清单在各自子 change 中详细定义。

### 7.5 .openspec.yaml 更新

```yaml
change_id: devrix-tools-terminal-architecture
priority: P0
demand_id: DM-20260618-007
status: s3_design  # ✓ 已更新
domains: [D2, D3, D4, D5, D6, D7]
dsaft_scenarios:
  - D2-S16
  - D2-S18
  - D2-S4
  - D3-S5
  - D4-S11
  - D4-S13
  - D5-S23
  - D6-S11
  - D7-S5
dsaft_activities:
  - D2-S4-A01       # LSP Tool 注册与调度
  - TOOL-SEC-2-A02  # Bash AST 安全引擎
  - D2-S18-A01      # CheckPermission 三态门控
  - D2-S18-A05      # BuildSurfaces 单入口
  - D2-S18-A06      # ExecuteToolRound 两阶段分派
  - D5-S23-A02      # 文件诊断追踪
  - D4-S11-A02      # 自由分叉探索
  - D4-S13-A02      # Worktree 隔离
  - D6-S11-A02      # 实现后自动验证
  - D4-S12-A03      # 后台任务事件推送通知
dsaft_functions:  # NEW: S3 阶段必须列出 F
  - D2-S4-A01-F01  # goToDefinition
  - D2-S4-A01-F02  # findReferences
  - D2-S4-A01-F03  # incomingCalls
  - D2-S4-A01-F04  # hover
  - D2-S4-A01-F05  # workspaceSymbol
  - D2-S4-A01-F06  # LSP server 进程池
  - TOOL-SEC-2-A02-F01  # AST 解析
  - TOOL-SEC-2-A02-F02  # heredoc 审计
  - TOOL-SEC-2-A02-F03  # zsh 攻击面规则集
  - D5-S23-A02-F01  # 编辑前后 diff 收集
  - D5-S23-A02-F02  # LRU 去重
  - D5-S23-A02-F03  # 异步触发器
  - D4-S11-A02-F01  # ForkAgent 创建
  - D4-S11-A02-F02  # SendMessage 通道
  - D4-S11-A02-F03  # fork 资源争抢仲裁
  - D4-S13-A02-F01  # worktree 隔离
  - D6-S11-A02-F01  # tasks.md 解析
  - D6-S11-A02-F02  # 验证项执行编排
  - D6-S11-A02-F03  # 结果聚合
  - D4-S12-A03-F01  # 事件流推送
  - D2-S18-A06-F01  # 混合批次调度
  - D2-S18-A06-F02  # sibling abort
  - D2-S18-A06-F03  # fallback discard
```

---

## 8. T 测试点注册表（Phase 1 P0 T 点全集）

按 DSAFT §九要求，列出本 change 在 Phase 1 涉及的 **25 个 P0 T 点**。每个 T 标注：名称、归属层级（A/F）、验收契约（Given-When-Then）、优先级、测试类型。

> T 归属规则（DSAFT §四）：
> - **T 归属 A**（`D{X}-S{X}-A{XX}-T{XX}`）：验证整个活动，如契约测试、E2E
> - **T 归属 F**（`D{X}-S{X}-A{XX}-F{XX}-T{XX}`）：验证具体功能点，如单元测试

### 8.1 LSPSurface T 点（D2-S4-A01，6 个 T 归属 A）

| T ID | 名称 | 归属层级 | 验收契约（Given-When-Then）| 优先级 | 测试类型 |
|------|------|---------|---------------------------|--------|---------|
| D2-S4-A01-T01 | goToDefinition 返回位置 | A | **GIVEN** workspace 含 Go 文件 + gopls 运行中<br>**WHEN** LLM 调用 `lsp_goToDefinition` 指向已定义符号<br>**THEN** 返回 `{uri, range}` 位置 + 在 2s 内 | P0 | 契约测试 |
| D2-S4-A01-T02 | findReferences 返回所有引用 | A | **GIVEN** 符号被引用 N 次<br>**WHEN** LLM 调用 `lsp_findReferences`<br>**THEN** 返回 N 个位置（包含声明位置）| P0 | 契约测试 |
| D2-S4-A01-T03 | incomingCalls 层级完整 | A | **GIVEN** 函数 A 调用 B，B 调用 A（递归）<br>**WHEN** LLM 调用 `lsp_incomingCalls`<br>**THEN** 返回完整调用链（含循环检测）| P0 | 契约测试 |
| D2-S4-A01-T04 | hover 返回 markdown | A | **GIVEN** Go 函数含 doc comments<br>**WHEN** LLM 调用 `lsp_hover`<br>**THEN** 返回 `{kind: "markdown", value: "..."}` 格式 | P0 | 契约测试 |
| D2-S4-A01-T05 | workspaceSymbol 按模式搜索 | A | **GIVEN** workspace 含 N 个匹配符号<br>**WHEN** LLM 调用 `lsp_workspaceSymbol` 模式 "Calcul*"<br>**THEN** 返回所有匹配符号 | P0 | 契约测试 |
| D2-S4-A01-T06 | LRU 池淘汰保上限 | A | **GIVEN** LRU 池容量 4 + 5 个 workspace<br>**WHEN** 第 5 个 workspace 打开<br>**THEN** 最久未用被淘汰 + 池总数仍 ≤ 4 | P0 | 集成测试 |
| D2-S4-A01-F*-T07 | LTL-Lite 不变式保持 | F（占位通配）| **GIVEN** LSPSurface 任意状态<br>**WHEN** InvariantSet.Check 调用<br>**THEN** 5 条不变式（read_only, lsp_servers≤4, timeout fallback, hover=markdown, !mutate server）全部通过 | P0 | 单元测试 |

### 8.2 BashASTPolicy T 点（TOOL-SEC-2-A02，6 个 T 归属 A）

| T ID | 名称 | 归属层级 | 验收契约（Given-When-Then）| 优先级 | 测试类型 |
|------|------|---------|---------------------------|--------|---------|
| TOOL-SEC-2-A02-T01 | 安全命令放行 | A | **GIVEN** `ls -la /tmp`<br>**WHEN** BashASTPolicy.Audit<br>**THEN** `Allow: true` + 审计 < 100ms | P0 | 单元测试 |
| TOOL-SEC-2-A02-T02 | rm -rf 被拒 | A | **GIVEN** `rm -rf /tmp/*`<br>**WHEN** BashASTPolicy.Audit<br>**THEN** `Allow: false` + reason 含 "rm -rf pattern" + span `d3.bash.denied` 发出 | P0 | 单元测试 |
| TOOL-SEC-2-A02-T03 | AST 解析失败 = 拒绝 | A | **GIVEN** 不可解析的 bash<br>**WHEN** BashASTPolicy.Audit<br>**THEN** `Allow: false` + reason "AST parse failed" | P0 | 单元测试 |
| TOOL-SEC-2-A02-T04 | 嵌套 heredoc 拒绝 | A | **GIVEN** heredoc 深度 > 2<br>**WHEN** BashASTPolicy.Audit<br>**THEN** `Allow: false` + reason "nested heredoc denied" | P0 | 单元测试 |
| TOOL-SEC-2-A02-T05 | 20+ zsh 攻击模式全部拦截 | A | **GIVEN** 20+ 已知攻击模式集合<br>**WHEN** 每个模式分别审计<br>**THEN** 全部 `Allow: false` + 返回匹配规则名 | P0 | 参数化测试 |
| TOOL-SEC-2-A02-T06 | Pre-Execution 审计钩子 | A | **GIVEN** BashSurface 即将执行<br>**WHEN** Execute 被调用<br>**THEN** Audit 先调用 + Deny 时返回 `ErrBashASTDenied` | P0 | 集成测试 |

### 8.3 DiagnosticTracker T 点（D5-S23-A02，5 个 T 归属 A）

| T ID | 名称 | 归属层级 | 验收契约（Given-When-Then）| 优先级 | 测试类型 |
|------|------|---------|---------------------------|--------|---------|
| D5-S23-A02-T01 | 编辑触发 tracker | A | **GIVEN** LLM 调用 `edit_file`<br>**WHEN** 编辑完成<br>**THEN** tracker 异步记录 pre/post diff + 发出 `d5.diagnostic.diff` span | P0 | 集成测试 |
| D5-S23-A02-T02 | 重复编辑去重 | A | **GIVEN** 同一编辑重复 N 次<br>**WHEN** tracker 处理<br>**THEN** 仅首次记录 + 后续丢弃 + 指标 `dedup_hit` 递增 | P0 | 单元测试 |
| D5-S23-A02-T03 | 不阻塞编辑主路径 | A | **GIVEN** tracker 慢（500ms）<br>**WHEN** edit_file 被调用<br>**THEN** edit 在 100ms 内完成 + tracker fire-and-forget | P0 | 性能测试 |
| D5-S23-A02-T04 | LRU 容量 1000 封顶 | A | **GIVEN** LRU 1000 条已满<br>**WHEN** 新编辑到达<br>**THEN** 最久未访问被淘汰 | P0 | 单元测试 |
| D5-S23-A02-T05 | EditSurface 集成 | A | **GIVEN** EditSurface 执行 `edit_file`<br>**WHEN** 成功完成<br>**THEN** tracker.OnEdit 被调用（非阻塞）| P0 | 集成测试 |

### 8.4 FreeForkSurface T 点（D4-S11-A02，4 个 T 归属 A）

| T ID | 名称 | 归属层级 | 验收契约（Given-When-Then）| 优先级 | 测试类型 |
|------|------|---------|---------------------------|--------|---------|
| D4-S11-A02-T01 | N 个子代理并行创建 | A | **GIVEN** fork 预算可用<br>**WHEN** `free_fork{n: 3, directions: [...]}`<br>**THEN** 3 个子代理并行创建 + 每个独立 worktree + 返回 3 个 sub-agent IDs | P0 | 集成测试 |
| D4-S11-A02-T02 | SendMessage 跨组拒绝 | A | **GIVEN** 子代理 A 在 group 1，B 在 group 2<br>**WHEN** A 发送消息给 B<br>**THEN** 拒绝 + span `message_cross_group_denied` | P0 | 单元测试 |
| D4-S11-A02-T03 | 文件锁仲裁 | A | **GIVEN** 2 子代理同需改 `/shared/file.go`<br>**WHEN** 并发修改请求<br>**THEN** 先到先得 + 后到收 `WaitForLock` 信号 | P0 | 集成测试 |
| D4-S11-A02-T04 | 子代理超时中止 | A | **GIVEN** 子代理运行 60s<br>**WHEN** 超时阈值到达<br>**THEN** 发送取消信号 + span `d4.fork.timeout` + 部分结果返回 | P0 | 集成测试 |

### 8.5 WorktreeIsolation T 点（D4-S13-A02，1 个 T 归属 A）

| T ID | 名称 | 归属层级 | 验收契约（Given-When-Then）| 优先级 | 测试类型 |
|------|------|---------|---------------------------|--------|---------|
| D4-S13-A02-T01 | worktree 物理隔离 | A | **GIVEN** workspace 在 `/workspace`<br>**WHEN** fork 创建<br>**THEN** 子代理工作目录在 `/workspace/.devrix-forks/<id>/` + 沙箱禁止越界 | P0 | 集成测试 |

### 8.6 VerifySurface T 点（D6-S11-A02，3 个 T 归属 A）

| T ID | 名称 | 归属层级 | 验收契约（Given-When-Then）| 优先级 | 测试类型 |
|------|------|---------|---------------------------|--------|---------|
| D6-S11-A02-T01 | tasks.md 标准解析 | A | **GIVEN** tasks.md 含标准格式<br>**WHEN** VerifyAggregate 解析<br>**THEN** 返回 `[]Task{name, criteria[]}` 列表 | P0 | 单元测试 |
| D6-S11-A02-T02 | 4 类验证项执行 | A | **GIVEN** task 含 Type A/B/C/D 验证项<br>**WHEN** 执行<br>**THEN** 4 类分别正确执行 + 返回正确 pass/fail | P0 | 参数化测试 |
| D6-S11-A02-T03 | 失败聚合阻止 S4-Gate | A | **GIVEN** 5 个 task 中 1 个失败<br>**WHEN** 聚合<br>**THEN** 整体 verdict `Fail` + 失败 task 名 + S4-Gate held（人工 review 必要）| P0 | 集成测试 |

### 8.7 T 点统计

| 归属 | 数量 | 占比 | 测试类型分布 |
|------|------|------|------------|
| 归属 A（P0） | 25 | 100% | 单元 11 + 集成 10 + 契约 4 |
| 归属 F（P0） | 0（仅通配占位 D2-S4-A01-F*-T07）| — | 通配占位用于 LTL-Lite 不变式 |
| **P0 总计** | **25** | — | **Phase 1 全部 P0** |

**Phase 1.5 / 2 / 3 增量 T 点（规划，不在本注册表）**：

- Phase 1.5 LTL-Lite：PERMISSION-GATE-1-T01（runtime check）+ T02（CI lint）
- Phase 1.5 MCP 预研：~6 个 T（MCP Attestation/Costly Sandboxing/Cross-Validation/Decay）
- Phase 2 Streaming Executor：~5 个 T（混合批次 + sibling abort + fallback discard）
- Phase 2 MCP Surface：~8 个 T（MCP server 生命周期 + 信誉衰减 + Causal Audit）
- Phase 2 PerSessionFilter：~3 个 T
- Phase 3：~10+ 个 T（Web 工具、doctor、故障注入、热力图）

**累计估算**：本 change 全生命周期 T 点 ~60 个，分 5 批实现。

---

## 9. 设计决策记录

### Decision 1: LSP server 进程池 LRU 淘汰 vs 固定 1 个

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 固定 1 个 server | 简单、内存可控 | 单语言支持、跨项目隔离差 |
| B: 1 个 server per workspace | 隔离性好 | 内存膨胀（8+ workspaces 时）|
| **C: LRU 池（默认 4）** | **平衡** | **稍复杂** |

**选择:** C
**理由:** 单 workspace 1 个 + LRU 淘汰多余 = 内存可控 + 跨项目隔离。`lsp_servers <= 4` 作为 LTL-Lite 不变式。

### Decision 2: BashAST 解析失败 = 拒绝 vs 允许 + 警告

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 失败 = 拒绝 | fail-closed 安全 | 误判多（特殊语法） |
| B: 失败 = 允许 + 警告 | 用户体验好 | 危险（绕过安全） |
| **C: 失败 = 拒绝 + 提供 rewrite 建议** | **安全 + 可用** | **需 LLM 配合** |

**选择:** C
**理由:** AST 解析失败意味着我们**无法证明**该命令是安全的，必须拒绝。但提供"是否能简化命令"的建议给 LLM，让 LLM 重试。

### Decision 3: FreeFork 并发上限 8 来源

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 4 | 资源节约 | 探索效率低 |
| B: 16 | 探索激进 | 内存压力 |
| **C: 8** | **平衡（2x macOS CPU 典型）** | **需明确来源** |

**选择:** C
**理由:** 8 = 2 × 典型 4-core machine CPU 数 + LSP server slot。来源写明"单 machine 内存/CPU 限制，非博弈论最优解"。硬上限是工程约束不是策略选择。

### Decision 4: DiagnosticTracker 同步 vs 异步

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 同步 | 实现简单 | 阻塞编辑主路径 |
| **B: 异步 fire-and-forget** | **不阻塞** | **可能丢失** |

**选择:** B
**理由:** 编辑延迟是 P0 用户体验指标。丢失误接受（下一轮编辑会重新触发 diff 收集）。LRU 容量 1000 防内存膨胀。

### Decision 5: LTL-Lite DSL 用 Go struct tag vs YAML

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: YAML | 人类可读 | 需独立解析器、易漂移 |
| **B: Go struct tag** | **零额外解析器、随代码编译** | **学习曲线** |

**选择:** B
**理由:** R2 共识。零解析器 + 编译时验证 + CI `go vet` 检查存在性。YAML 易漂移问题在快速迭代期致命。

### Decision 6: MCP Capability Attestation 用 TUF vs 简单签名

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 简单 Ed25519 签名 | 简单 | 单 key compromise = 全局 compromise |
| **B: TUF 框架** | **key rotation + threshold trust** | **复杂** |

**选择:** B
**理由:** MCP 引入后第三方代码进入系统，签名 key compromise 是 P0 风险。TUF 提供多 key + 阈值信任 + 定期 rotation，是业界标准。Phase 1.5 预研期间可先实现 Ed25519 单签名，Phase 2 升级到 TUF。

### Decision 7: CheckPermission 承诺有效期 = 当前 turn vs 跨 turn

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 永久 | 体验好 | 等同无限授权 |
| B: Session 范围 | 平衡 | 用户感知不到 |
| **C: 当前 turn** | **最小权限原则** | **需频繁授权** |

**选择:** C
**理由:** Commitment Device 博弈论要求"承诺可观测且可撤销"。当前 turn 是最严格的承诺，跨 turn 必须重新授权。Ask 弹窗中提供"本 turn + 跨 turn 同一类型"快捷选项缓解频繁授权问题。

### Decision 8: Surface 数量上限 ~12

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 无上限 | 灵活 | 失控 |
| B: 严格 12 | 可控 | 偶发不够 |
| **C: 软上限 12 + AC25 异质性门槛** | **可解释** | **需审查** |

**选择:** C
**理由:** 12 是经验值（现有 7 + Phase 1+2 新增 5）。AC25 要求新 Surface 必须证明工具 RiskLevel 异质性 ≥ 阈值，防止搭便车均衡。PluginSurface 模式复用作为合并机制。

---

## 10. 回归风险评估

| 风险 | 影响域 | 缓解 | 检测方式 |
|------|--------|------|---------|
| LSP server 进程泄漏 | D2 资源 | LRU 池 + 进程健康检查 | `pgrep -f lsp` 监控 |
| BashAST 误判正常命令 | D2 工具调用 | 灰度发布 + 用户反馈通道 | Ask 弹窗"报告误判" |
| FreeFork 子代理僵尸 | D4 资源 | timeout + 子代理心跳 | runtime metrics |
| DiagnosticTracker 内存膨胀 | D5 资源 | LRU 1000 上限 | runtime metrics |
| VerifySurface 误报 fail | D6 流程 | 与 tasks.md 强一致 + 人工 override | S4-Gate 二次确认 |
| LTL-Lite 误报 violation | D7 流程 | invariant 启动时 dry-run | 启动日志 |
| MCP Attestation 误拒 | Phase 2 工具 | 降级到 PerRisk 严格模式 | 监控 rate |
| Phase 1.5 预研投入失控 | 工程资源 | 单独子 change、独立预算 | 按时关闭 |
| Causal Audit Trail 存储膨胀 | D6 存储 | append-only + 周期压缩 | 容量监控 |
| 跨 Phase 边界接口漂移 | 跨域 | design.md 锁版本 | S3-Gate review |

### 9.1 P0 T 点保护清单

S3 完成后必须保持 PASS 的 P0 T 点（来自 t-registry.md）：

- **TOOL-SURFACE-1-T01 ~ T38**：所有 ToolSurface 行为一致性
- **PERMISSION-GATE-1-T01, T02**：CheckPermission 三态门控
- **D2-S18-A01-T01 ~ T08**：CheckPermission 状态机
- **D2-S4-A01-T01 ~ T05**（新增）：LSP 4 操作 + 进程池
- **TOOL-SEC-2-A02-T01 ~ T06**（新增）：BashAST 规则
- **D5-S23-A02-T01 ~ T05**（新增）：Tracker diff + LRU
- **D4-S11-A02-T01 ~ T04**（新增）：FreeFork 生命周期
- **D6-S11-A02-T01 ~ T03**（新增）：Verify 流程

总计新增 P0 T 点 ~25 个，跨 Phase 累计 ~50 个（Phase 1.5 + 2 另增 ~25 个）。

### 9.2 性能影响预估

| 操作 | 当前 | 设计后 | 影响 |
|------|------|--------|------|
| ToolFilter.Apply 累计 | ~20ms | ~50ms（含 LTL-Lite assert） | 仍在 <100ms 预算 |
| Turn round-trip | 2-5s | 2-5s（LSP + Tracker 不阻塞） | 无变化 |
| Bash 审计 | ~5ms（正则） | ~50ms（AST） | +45ms 一次调用 |
| LLM 调用 | 1× | 1×（D2 不持有 D3） | 无变化 |
| 内存（多 LSP） | 0 | +200MB/workspace | LRU 4 限制 |

---

## 11. 回滚方案

### 10.1 单个 Phase 内子 change 回滚

每个子 change 是独立 PR + 独立 commit，回滚 = revert PR。

**回滚前必检**：
- 子 change 无数据库 schema 变更 ✓（DSAFT 不持久化）
- 子 change 无外部 API 契约变更（仅新增，不修改既有）✓
- 子 change 无删除既有文件 ✓

### 10.2 整个 Phase 回滚

按 Phase 1 → 1.5 → 2 → 3 顺序回滚（不要逆向）：
- Phase 1 回滚 = revert 5 个 PR
- Phase 1.5 回滚 = revert 2 个 PR
- Phase 2 回滚 = revert 3 个 PR
- Phase 3 单个 feature 可独立回滚

### 10.3 紧急回滚（Feature Flag）

每个子 change 部署时使用 feature flag（`devrix.yaml` 的 `feature_flags` 段）：

```yaml
feature_flags:
  lsp_surface_enabled: true       # feat/lsp-tool-surface
  bash_ast_policy_enabled: true   # feat/bash-ast-security
  diagnostic_tracker_enabled: true # feat/diagnostic-tracker
  free_fork_enabled: true         # feat/free-fork-explore
  verify_plan_exec_enabled: true  # feat/verify-plan-execution
  ltl_lite_enabled: true          # feat/ltl-lite-invariant
  mcp_surface_enabled: false      # feat/mcp-surface
```

紧急回滚 = `kubectl set env` 或 `devrix config set feature_flags.<flag>=false`，无需重启 D2。

### 10.4 数据回滚

**无数据回滚需求**：
- 所有 change 不修改数据库 schema
- 所有 span/metric 是 additive（D5 观测层）
- 所有 D6 reputation 调整是 additive（无删除路径）

**唯一需要清理的**：
- Causal Audit Trail 累积的 append-only log（容量监控触发清理）
- LRU 缓存重启时自动清空

---

## 12. 关联与依赖

### 11.1 依赖的既有 change

- **DM-007**（devrix-tool-surface-contract）：ToolSurface 6 方法契约
- **DM-008**（devrix-tool-surface-phase2-full）：12→0 global loop 清理
- **DM-20260618-001**（ToolSpec v2）：4 正交标志
- **DM-20260618-002**（CheckPermission 三态）：Allow/Deny/Ask
- **DM-20260618-003**（DeferLoading）：tool_search 懒加载
- **devrix-diagnostic-tools-parity**（DM-20260616-003）：诊断工具基线

### 11.2 关联的未来 change

- `feat/web-tools-surface`（Phase 3）依赖 Phase 1 LSPSurface 的 ToolFilter 模式
- `feat/doctor-self-diagnose`（Phase 3）依赖 DiagnosticTracker 数据
- `feat/tool-attention-heatmap`（Phase 3）依赖 D5 chain_consistency span
- `feat/mcp-mechanism-design-research`（Phase 1.5）是 Phase 2 MCP Surface 的前置研究

### 11.3 风险传导

| 风险 | 上游 | 下游 | 缓冲 |
|------|------|------|------|
| LSP server 崩溃 | gopls bug | 所有 LSP 操作 | fallback grep |
| mvdan.cc/sh API 变更 | 库升级 | BashAST 解析 | 锁定版本 + T 测试 |
| CheckPermission 误判 | 用户配置 | 工具执行 | 用户 override + 日志 |
| FreeFork 子代理泄漏 | OS 进程管理 | D4 资源 | timeout + 心跳 + arbiter |

---

## 13. S3 完成检查清单

### 13.1 architecture-design.md §8 基础检查

- [x] `design.md` 包含根因、方案、文件清单、回归风险
- [x] `dsaft_activities` 已标注（见 §7.5）
- [x] `dsaft_functions` 已标注（见 §7.5，新增字段）
- [x] `design.md` 明确每个 A 的 F 编排关系（见 §2.1）
- [x] `specs/*/spec.md` 包含所有 Gherkin Scenario（见 specs/ 目录）
- [x] 每个 Requirement 有对应的 T 层注释
- [x] 重大决策已记录（见 §9，共 8 个 Decision）
- [x] Phase 1 5 个子 change 描述清晰（见 §7.1）
- [x] Phase 1.5 LTL-Lite 框架设计完整（见 §3.2 + §6.4）
- [x] Phase 2 MCP 多中心机制设计完整（见 §3.3 + §6.5）
- [x] 风险评估 + 回滚方案完备（见 §10 + §11）
- [x] 不含工时估算（§5 §10 仅复杂度分析）

### 13.2 DSAFT §九 资产登记 Checklist 全绿

| 层级 | 必填项 | design.md 位置 | 状态 |
|------|--------|---------------|------|
| **D** | ID、名称、类型、领域职责 | §0.1 | ✅ |
| **S** | ID、名称、触发条件、用户目标、涉及 A | §0.2 | ✅ |
| **A** | ID、名称、类型、输入、输出、状态变更 | §0.3 | ✅ |
| **F** | ID、名称、类型、输入、输出 | §0.4 | ✅ |
| **T** | ID、名称、归属层级、归属 ID、验收契约、优先级 | §8（25 个 T 全列）| ✅ |

**DSAFT 方法论 §10 OpenSpec 映射**：

| 阶段 | DSAFT 产出 | 本 design 满足 |
|------|-----------|--------------|
| S2 proposal | D + S | ✅ demand.md §2 标识 7 个 D + 9 个 S |
| S3 design | F + A↔F | ✅ §0.3 A 编排 + §0.4 F 23 个 + §2.1 A↔F 映射 |
| S4 tasks | F 实现任务 | （下一步：tasks.md 拆分）|
| S5 verify | T | ✅ §8 列出 25 个 T（归属 A）+ Phase 1.5/2/3 增量 ~35 个 |

### 13.3 review-design.md §2 Review 维度预检

| 维度 | 检查项 | 状态 |
|------|--------|------|
| **2.1 架构决策** | 层归属正确 | ✅ §0.1 D 归属 + §4.3 限界上下文 |
| | 接口方向正确 | ✅ §6 接口设计 D7→D2→LSP Adapter |
| | 不重复造轮子 | ✅ 引用 DM-007/008/DM-20260618-001/002/003 |
| | 跨层依赖最小 | ✅ §4.3 "不应跨入" 边界 + D2-D3 import lint |
| | 设计决策有记录 | ✅ §9 八个 Decision |
| **2.2 需求完整性** | 需求可追溯 | ✅ demand.md → proposal.md → design.md → specs |
| | 验收标准覆盖 | ✅ §0.2 S 表 9 个 → §8 T 表 25 个全覆盖 |
| | Out of Scope 明确 | ✅ proposal.md §7 |
| | DM ID 无冲突 | ✅ DM-20260618-007（独立 ID）|
| **2.3 规格质量** | Gherkin 格式正确 | ✅ 6 个 spec.md 文件 |
| | Happy + sad path | ✅ spec.md 中两类 Scenario 均覆盖 |
| | 并发场景覆盖 | ✅ spec/free-fork/spec.md T03 (文件锁仲裁) |
| | 错误路径覆盖 | ✅ spec/bash-ast-policy/spec.md T03 (fail-closed) |
| | T 层映射完整 | ✅ §8 全部 25 个 T 标注 |
| **2.4 风险审查** | 回归风险已评估 | ✅ §10 风险表 + 性能影响 |
| | 回滚方案可行 | ✅ §11 单 phase + 整个 phase + feature flag |
| | 性能影响已评估 | ✅ §10.2 性能表 |

---

## 14. 附录：Phase 1 优先级排序（继承自 demand.md §9）

| 顺序 | 能力 | 子 change | 依赖 |
|------|------|-----------|------|
| 1 | **LSP Tool Surface** | `feat/lsp-tool-surface` | DM-007 |
| 2 | **Bash AST 安全** | `feat/bash-ast-security` | DM-20260618-002 |
| 3 | **文件诊断追踪** | `feat/diagnostic-tracker` | DM-20260618-001 |
| 4 | **自由分叉探索** | `feat/free-fork-explore` | devrix-diagnostic-tools-parity |
| 5 | **实现后验证** | `feat/verify-plan-execution` | D6 eval 框架 |

**反驳预案**（6 条，继承自 demand.md §9）：见 demand.md 反驳预案表。

---

**Status:** S3_Design 完成。准备进入 S3-Gate（review-design.md 审查）。
