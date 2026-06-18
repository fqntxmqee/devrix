# Tasks: devrix-tools-terminal-architecture

**Change ID:** devrix-tools-terminal-architecture
**Demand ID:** DM-20260618-007
**Status:** S4_Implementation
**估算参考（仅供参考，非承诺）:** 7 Phase × 16 W, ~+2800 LOC (含测试)

---

> **DSAFT Activity 一览**
>
> 本 change 涉及 7 个 Activity（D2-S4-A01 LSP / TOOL-SEC-2-A02 BashAST /
> D5-S23-A02 文件诊断追踪 / D4-S11-A02 自由分叉 / D4-S13-A02 Worktree /
> D6-S11-A02 实现后验证 / D4-S12-A03 后台任务事件推送）+ 1 个跨切面
> LTL-Lite 框架（PERMISSION-GATE-1）。W 编号按 Activity 组织，每个 W 标注
> 关联 Activity / F / T。
>
> **S3-Gate 吸收的建议（SUG-1/2/5/6 必收，其他 SUG 可后置）**：
> - **SUG-1** WorkerContext 子代理 budget 共享 → 合并到 W8 FreeFork F01
> - **SUG-2** LSP SLO 监控 → 合并到 W1 LSP W3
> - **SUG-5** devrix.yaml 配置项 → 贯穿 W1/W4/W6/W8/W11
> - **SUG-6** mvdan.cc/sh v3.x 锁定 → W4 + go.mod

## Phase 0: 共享基础（W0，最先做）

### W0 — 工具层依赖与配置基础

- **文件 1:** `internal/shared/config/contextengine.go` (修改, +40 行)
  - 增 `ToolsConfig.Surfaces` 子结构（已有部分，按 surface 扩展字段）
  - LSP 池大小：`Surfaces.LSP.MaxServers int`（默认 4，硬上限 4 由 invariant 强约束）
  - Bash AST 启用：`Surfaces.Bash.ASTEnabled bool`（默认 true）
  - FreeFork：`Surfaces.FreeFork.MaxConcurrent int`（默认 8）
  - Tracker LRU：`Surfaces.Tracker.LRUSize int`（默认 1000）
  - Verify timeout：`Surfaces.Verify.TimeoutSec int`（默认 300，per-verify-type 可覆盖）
  - DiagnosticTracker 采样阈值：`Surfaces.Tracker.SamplingThresholdHz int`（默认 10）
- **文件 2:** `internal/shared/config/contextengine_test.go` (修改, +40 行)
  - 缺省 + 显式配置 case + 字段边界值校验
- **文件 3:** `go.mod` (修改)
  - 锁定 `mvdan.cc/sh v3.x`（SUG-6 吸收，避免 v4 重大变更）
- **文件 4:** `.github/workflows/ci.yml` (修改)
  - 新增 `go list -m all` 检查步骤，确保锁定版本不被无意升级
- **依赖:** 无
- **AC:** AC7（不破坏既有 P0 T 点）
- **T:** 无（基础步骤）
- **估时参考:** 45 min

---

## Phase 1: LSP Tool Surface（W1-W3，6 F + 6 T）

### W1 — D2-S4-A01-F01/F02 LSP goToDefinition + findReferences

- **文件 1:** `internal/layers/contextengine/lsp/server.go` (新建, ~120 行)
  - 定义 `Server` struct（cmd *exec.Cmd, stdin/stdout, language string）
  - `Start(ctx) error` / `Stop() error` / `Call(method string, params any) (json.RawMessage, error)`
  - JSON-RPC 2.0 协议封装（Content-Length header）
- **文件 2:** `internal/layers/contextengine/lsp/pool.go` (新建, ~100 行)
  - LRU 池（默认 4，硬上限由 invariant 强约束）
  - `Get(language string) (*Server, error)` / `Release(server *Server)` / `Shutdown(ctx)`
  - 淘汰策略：超过池大小时关闭最久未用 server
- **文件 3:** `internal/layers/contextengine/lsp/pool_test.go` (新建, ~120 行)
  - `TestPool_LRUEviction` — 验证第 5 个请求触发最旧 server 关闭
  - `TestPool_ConcurrencySafe` — 8 并发请求不 panic
  - `TestPool_Shutdown` — 所有 server 干净退出
- **依赖:** W0
- **AC:** AC1（4 P0 操作之一 goToDefinition/findReferences）
- **T:** D2-S4-A01-F01-T01（goToDefinition 基本返回）, D2-S4-A01-F02-T01（findReferences 基本返回）
- **估时参考:** 180 min

### W2 — D2-S4-A01-F03/F04/F05 LSP incomingCalls / hover / workspaceSymbol

- **文件 1:** `internal/layers/contextengine/lsp/adapter.go` (新建, ~150 行)
  - `Adapter` struct：持有 `*Pool`，提供 6 个 typed method
  - `GoToDefinition(ctx, file, line, col) ([]Location, error)`
  - `FindReferences(ctx, file, line, col, includeDecl bool) ([]Location, error)`
  - `IncomingCalls(ctx, file, line, col) ([]CallHierarchyItem, error)`
  - `Hover(ctx, file, line, col) (*HoverInfo, error)`
  - `WorkspaceSymbol(ctx, query string) ([]SymbolInformation, error)`
  - 通用 2s timeout（SUG-2 关联）+ 失败时 fallback 到 grep（设计文档 §3.1.1）
- **文件 2:** `internal/layers/contextengine/lsp/types.go` (新建, ~100 行)
  - LSP 协议类型映射（Location, CallHierarchyItem, HoverInfo, SymbolInformation）
- **文件 3:** `internal/layers/contextengine/lsp/adapter_test.go` (新建, ~150 行)
  - 5 个 method 各 1 happy path + 1 timeout/fallback case
  - 集成测试用 `internal/layers/contextengine/lsp/testdata/gopls_mock.go`（轻量 mock server）
- **依赖:** W1
- **AC:** AC1（剩余 incomingCalls / hover / workspaceSymbol）
- **T:** D2-S4-A01-F03-T01, D2-S4-A01-F04-T01, D2-S4-A01-F05-T01
- **估时参考:** 180 min

### W3 — D2-S4-A01 LSPSurface 实现 + LRU 池集成 + SLO 监控（SUG-2 吸收）

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/surface/lsp_surface.go` (修改, +80 行)
  - 持有 `*lsp.Adapter` 显式 dep，**不**用 global
  - `Tools() []ToolSpec` 返回 5 个 LSP spec
  - `Execute(ctx, name, input) ToolResult` 路由到对应 adapter method
  - `RiskLevel(name) RiskLevel` 全部返回 ReadOnly
- **文件 2:** `internal/layers/observability/metrics/lsp.go` (新建, ~60 行)
  - D5 metrics 注册：`d2.lsp.call.count`（按 method 标签）、`d2.lsp.latency.p99`（直方图）、`d2.lsp.timeout.count`
  - **SUG-2 吸收**：p99 LSP 延迟 ≤ 1.5s 告警阈值（默认，可在 devrix.yaml `metrics.lsp.latency_alert_ms` 覆盖）
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/surface/lsp_surface_test.go` (修改, +80 行)
  - `TestLSPSurface_Execute_GoToDefinition`
  - `TestLSPSurface_Execute_FallbackGrep`（LSP 超时时降级）
  - `TestLSPSurface_RiskLevel_AllReadOnly`
- **文件 4:** `internal/layers/observability/metrics/lsp_test.go` (新建, ~50 行)
  - 验证 metrics 注册与 threshold 触发
- **依赖:** W2
- **AC:** AC1（全部 5 method 通过 surface.Execute 暴露）+ AC6（D2 不持有 D3 引用）
- **T:** D2-S4-A01-T04（surface 集成）, D2-S4-A01-T05（LRU 池行为）, D2-S4-A01-T06（fallback 降级）
- **估时参考:** 180 min

---

## Phase 2: Bash AST 安全引擎（W4-W5，3 F + 6 T）

### W4 — TOOL-SEC-2-A02-F01 AST 解析 + mvdan.cc/sh v3.x 锁定（SUG-6 吸收）

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/bash/parser.go` (新建, ~120 行)
  - `Parse(cmd string) (*AST, error)` 使用 `mvdan.cc/sh/v3` 解析
  - 解析失败返回 `ErrParseFailed`（fail-closed）
  - **SUG-6 吸收**：`go.mod` 锁定 `mvdan.cc/sh v3.x`，CI `go list -m all` 检查
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/bash/heredoc.go` (新建, ~60 行)
  - `HeredocAudit(node *AST) []Finding` 检测 heredoc 内嵌恶意 payload
  - 嵌套 heredoc 拒绝（AC2）
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/bash/parser_test.go` (新建, ~150 行)
  - `TestParse_SimpleCommand`
  - `TestParse_PipelineChain`
  - `TestParse_ParseFailure_ReturnsErrParseFailed`
  - `TestParse_HeredocDetection`
  - `TestParse_NestedHeredoc_Rejected`
- **依赖:** W0（依赖锁定）
- **AC:** AC2（BashAST 20+ 攻击模式 — 部分）
- **T:** TOOL-SEC-2-A02-T01（AST 解析基本功能）, TOOL-SEC-2-A02-T02（解析失败 fail-closed）, TOOL-SEC-2-A02-T04（heredoc 检测）
- **估时参考:** 180 min

### W5 — TOOL-SEC-2-A02-F02/F03 zsh 攻击面规则集 + Audit 入口 + Bash Surface 集成

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/bash/zsh_rules.go` (新建, ~200 行)
  - 20+ zsh 攻击模式规则集（设计文档 §8.2 T05 列出的 20 模式）
  - `MatchRule(node *AST) (Rule, bool)` 匹配 + 规则命中
  - 规则元数据：ID、严重度（critical/high/medium）、描述、修复建议
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/bash/policy.go` (新建, ~100 行)
  - `Audit(cmd string) Decision` 整合 Parse + HeredocAudit + zsh_rules.MatchRule
  - 返回 `Decision{Allow | Deny | Ask}` + 命中的 rule IDs + 修复建议
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/bash/policy_test.go` (新建, ~200 行)
  - 20 个 zsh 攻击模式各 1 个测试 case（happy 拒绝）
  - `TestAudit_FailClosed_OnParseError`
  - `TestAudit_AllowsBenign`
- **文件 4:** `internal/layers/contextengine/enforce/toolrunner/surface/bash_surface.go` (修改, +50 行)
  - 在 `Execute` 中先调 `bash.Policy.Audit` → Deny 立即返回 error + 建议
  - Ask 弹窗通过 D1 IM 集成（SUG-4 可后置，先返回 error 文本）
- **文件 5:** `internal/layers/contextengine/enforce/toolrunner/surface/bash_surface_test.go` (修改, +100 行)
  - `TestBashSurface_Deny_OnAttackPattern`
  - `TestBashSurface_Deny_OnParseFailure`
  - `TestBashSurface_Allow_OnBenign`
- **依赖:** W4
- **AC:** AC2（剩余 — 20+ zsh 攻击模式）
- **T:** TOOL-SEC-2-A02-T03（fail-closed 行为）, TOOL-SEC-2-A02-T05（zsh 规则集）, TOOL-SEC-2-A02-T06（Bash Surface 集成）
- **估时参考:** 240 min

---

## Phase 3: 文件诊断追踪（W6-W7，3 F + 5 T）

### W6 — D5-S23-A02-F01/F02 Diff 收集 + LRU 去重

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/tracker/diff.go` (新建, ~120 行)
  - `CollectDiff(before, after string) Diff` 编辑前后 diff
  - 提取 `added_lines`、`removed_lines`、`changed_files`
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/tracker/lru.go` (新建, ~80 行)
  - 通用 LRU 容器（key = file:hash, value = Diff）
  - `LRU.Put(key, value)` / `LRU.Get(key) (value, ok)` / `LRU.Len() int`
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/tracker/record.go` (新建, ~100 行)
  - `Tracker` struct：LRU 持有 + lock（concurrent-safe）
  - `Record(file string, diff Diff)` 写入 LRU（重复文件 hash 一致则去重）
  - `Query(file string) (Diff, bool)` 读取
- **文件 4:** `internal/layers/contextengine/enforce/toolrunner/tracker/record_test.go` (新建, ~150 行)
  - `TestRecord_Deduplication`（相同 diff 不重复占用 LRU slot）
  - `TestRecord_LRUEviction`（超过 1000 时淘汰最旧）
  - `TestRecord_ConcurrentSafe`
- **依赖:** W0
- **AC:** AC3（文件诊断追踪 LRU 1000）
- **T:** D5-S23-A02-T01（diff 收集）, D5-S23-A02-T02（LRU 去重）
- **估时参考:** 180 min

### W7 — D5-S23-A02-F03 异步触发器 + Linter 集成 + EditSurface 集成

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/tracker/async.go` (新建, ~80 行)
  - `AsyncTrigger` struct：channel + worker goroutine
  - `FireAndForget(file string, diff Diff)` 非阻塞写入 channel
  - 编辑频率 > 10/s 时降级为采样追踪（SUG-5 关联，阈值由 devrix.yaml 控制）
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/tracker/linter.go` (新建, ~100 行)
  - `Linter` interface：`Lint(diff Diff) []Diagnostic`
  - 默认实现：内置基础规则（过长行、潜在 TODO 残留、import 顺序）
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/surface/edit_surface.go` (修改, +60 行)
  - 在 Edit/Write spec 的 `Execute` 中调 `tracker.AsyncTrigger.FireAndForget`
  - 编辑延迟不被追踪阻塞（设计文档 §9 Decision 4）
- **文件 4:** `internal/layers/contextengine/enforce/toolrunner/tracker/async_test.go` (新建, ~120 行)
  - `TestAsync_NonBlocking`（验证 FireAndForget 不等待）
  - `TestAsync_Sampling`（高频时按比例丢弃）
- **文件 5:** `internal/layers/contextengine/enforce/toolrunner/tracker/linter_test.go` (新建, ~100 行)
  - 3 个基础规则各 1 case
- **依赖:** W6
- **AC:** AC3（剩余 — 异步不阻塞 + Linter 集成）
- **T:** D5-S23-A02-T03（异步触发）, D5-S23-A02-T04（Linter 集成）, D5-S23-A02-T05（EditSurface 集成）
- **估时参考:** 180 min

---

## Phase 4: 自由分叉 + Worktree（W8-W10，4 F + 5 T）

### W8 — D4-S11-A02-F01/F02 ForkAgent + SendMessage + WorkerContext budget（SUG-1 吸收）

- **文件 1:** `internal/layers/multiagent/freefork/forker.go` (新建, ~150 行)
  - `Forker` struct：持有 `maxConcurrent int`（默认 8，可配置 W0）+ lock
  - `Fork(ctx, request ForkRequest) ([]ForkResult, error)`
  - 并发上限 8 由 invariant 强约束，W0 配置覆盖
- **文件 2:** `internal/layers/multiagent/freefork/messaging.go` (新建, ~100 行)
  - `SendMessage(parentCtx, childID, msg Message) error`
  - 60s timeout（设计文档 §8.4 T04）
- **文件 3:** `internal/layers/multiagent/freefork/worker_context.go` (新建, ~80 行)
  - **SUG-1 吸收**：`WorkerContext` 增加 `BudgetCap int` 字段（token 上限）
  - 子代理创建时父 WorkerContext 的 budget 按方向数 n 等分（共享父 budget）
  - 任何子代理超 budget → 中止该子代理并通知父
- **文件 4:** `internal/layers/multiagent/freefork/forker_test.go` (新建, ~150 行)
  - `TestFork_MaxConcurrent_8Limit`
  - `TestFork_ConcurrentSafe`
  - `TestWorkerContext_BudgetShared_AcrossChildren`（SUG-1 验证）
- **依赖:** W0
- **AC:** AC4（自由分叉 8 上限）+ SUG-1 吸收
- **T:** D4-S11-A02-T01（ForkAgent 创建）, D4-S11-A02-T02（SendMessage）, D4-S11-A02-T04（timeout 中止）
- **估时参考:** 180 min

### W9 — D4-S11-A02-F03 资源争抢仲裁 + FreeForkSurface 集成

- **文件 1:** `internal/layers/multiagent/freefork/arbitration.go` (新建, ~100 行)
  - `Arbitrator` struct：文件锁 + 优先级队列
  - `AcquireFile(path string, owner string, timeout time.Duration) error`
  - 死锁检测（DAG 等待图 + 超时）
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/surface/freefork_surface.go` (修改, +50 行)
  - 在 `Execute` 中调 `forker.Fork` + `arbitrator.AcquireFile`
  - 通过 WorkerContext 传递 budget_cap
- **文件 3:** `internal/layers/multiagent/freefork/arbitration_test.go` (新建, ~120 行)
  - `TestArbitrator_FileLock`
  - `TestArbitrator_DeadlockDetection`（A→B→A 循环等待）
- **文件 4:** `internal/layers/contextengine/enforce/toolrunner/surface/freefork_surface_test.go` (修改, +60 行)
  - `TestFreeForkSurface_BudgetCap_Propagated`
- **依赖:** W8
- **AC:** AC4（剩余 — 资源争抢仲裁）
- **T:** D4-S11-A02-T03（资源争抢仲裁）
- **估时参考:** 120 min

### W10 — D4-S13-A02-F01 Worktree 隔离

- **文件 1:** `internal/layers/multiagent/freefork/worktree.go` (新建, ~120 行)
  - `Worktree` struct：`path`, `branch`, `cleanup func()`
  - `Create(ctx, baseBranch string) (*Worktree, error)` — 调 `git worktree add`
  - `Remove(ctx)` — 调 `git worktree remove --force`
- **文件 2:** `internal/layers/multiagent/freefork/worktree_test.go` (新建, ~150 行)
  - `TestWorktree_Create_AndCleanup`
  - `TestWorktree_Concurrent`（多个 fork 各自独立 worktree）
  - `TestWorktree_Cleanup_OnContextCancel`
- **文件 3:** `internal/layers/multiagent/freefork/forker.go` (修改, +30 行)
  - Fork 时为每个子代理创建独立 worktree
  - ForkResult 包含 worktree path
- **依赖:** W8, W9
- **AC:** AC4（worktree 隔离）
- **T:** D4-S13-A02-T01（worktree 隔离）
- **估时参考:** 120 min

---

## Phase 5: 实现后自动验证（W11-W12，3 F + 3 T）

### W11 — D6-S11-A02-F01 tasks.md 解析 + F02 验证项执行

- **文件 1:** `internal/layers/evolution/guard/verify_plan_execution/parser.go` (新建, ~150 行)
  - `ParseTasksFile(path string) ([]Task, error)`
  - 解析 `## Tasks` + `### Task <name>` + acceptance criteria
- **文件 2:** `internal/layers/evolution/guard/verify_plan_execution/executor.go` (新建, ~200 行)
  - `Executor` struct：持有 timeout（默认 300s，per-verify-type 可覆盖）
  - 4 类验证 Type A/B/C/D：
    - Type A: Go Tests（`go test -race ./...`）
    - Type B: go vet + gofmt
    - Type C: P0 T 点覆盖率（查 `openspec/t-registry.md`）
    - Type D: CI Lint
- **文件 3:** `internal/layers/evolution/guard/verify_plan_execution/parser_test.go` (新建, ~150 行)
  - `TestParse_StandardTasks`
  - `TestParse_EmptyTasks_Warning`
  - `TestParse_Malformed_ReturnsErrVerifyParseFailed`
- **文件 4:** `internal/layers/evolution/guard/verify_plan_execution/executor_test.go` (新建, ~200 行)
  - 4 类验证各 1 happy + 1 fail case
  - `TestExecutor_TimeoutEnforced`
- **依赖:** W0（config）
- **AC:** AC5（实现后自动验证）
- **T:** D6-S11-A02-T01（tasks.md 解析）, D6-S11-A02-T02（验证项执行）
- **估时参考:** 240 min

### W12 — D6-S11-A02-F03 结果聚合 + D6 reputation + Verify Surface

- **文件 1:** `internal/layers/evolution/guard/verify_plan_execution/aggregator.go` (新建, ~120 行)
  - `Aggregate(results []VerifyResult) Verdict` — 全部 Pass → `Pass`，否则 `Fail`
  - `Verdict.PassCount` / `Verdict.FailCount` / `Verdict.FailedTasks`
  - timeout 处理为 Fail
- **文件 2:** `internal/layers/evolution/reputation/store.go` (修改, +40 行)
  - 增 `OnVerifyResult(verdict Verdict) error` 方法
  - Fail → 减 LLM/tool reputation（设计文档 §3.1.4）
  - Pass → 恢复 reputation
- **文件 3:** `internal/layers/contextengine/enforce/toolrunner/surface/verify_surface.go` (修改, +50 行)
  - `Execute` 调 `parser.ParseTasksFile` + `executor.Execute` + `aggregator.Aggregate`
  - 返回 `ToolResult{Verdict, Details}` 给 LLM
- **文件 4:** `internal/layers/evolution/guard/verify_plan_execution/aggregator_test.go` (新建, ~120 行)
  - `TestAggregate_AllPass` → S4-Gate auto-approve
  - `TestAggregate_OneFail` → S4-Gate hold + failed task name
  - `TestAggregate_TimeoutAsFail`
- **依赖:** W11
- **AC:** AC5（剩余 — 聚合 + reputation + surface 集成）
- **T:** D6-S11-A02-T03（结果聚合）
- **估时参考:** 180 min

---

## Phase 6: 后台任务事件推送（W13，1 F）

### W13 — D4-S12-A03-F01 事件流推送 + BackgroundTaskSurface 集成

- **文件 1:** `internal/layers/multiagent/runner/events.go` (新建, ~100 行)
  - `EventStream` struct：buffered channel + 订阅者列表
  - `Publish(event Event)` 非阻塞（buffer 满则丢弃并告警）
  - `Subscribe() (<-chan Event, func())` 返回 unsubscribe
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/surface/background_task_surface.go` (修改, +50 行)
  - `task_output` spec 的 `Execute` 调 `eventstream.Subscribe` 等待指定 task_id 事件
  - `task_list_background` spec 返回活跃 task 列表
- **文件 3:** `internal/layers/multiagent/runner/events_test.go` (新建, ~100 行)
  - `TestEventStream_PublishSubscribe`
  - `TestEventStream_BufferOverflow_NotBlocking`
  - `TestEventStream_Unsubscribe_NoLeak`
- **依赖:** W0
- **AC:** AC4（部分 — 后台任务事件推送）
- **T:** D4-S12-A03-T01（事件流推送）
- **估时参考:** 120 min

---

## Phase 7: LTL-Lite 跨切面框架（W14-W15，PERMISSION-GATE-1-T01/T02）

### W14 — PERMISSION-GATE-1 LTL-Lite 框架核心（Go struct tag + Runtime Check）

- **文件 1:** `internal/shared/ltllite/parser.go` (新建, ~150 行)
  - `ParseStruct(s any) (InvariantSet, error)` 通过 reflect 解析 `invariant:"pre => post"` tag
  - `Invariant{Name, Pre, Post, OwnerSurface}`
  - 格式校验：必须含 `=>`，否则返回 `ErrInvalidInvariant`
- **文件 2:** `internal/shared/ltllite/check.go` (新建, ~100 行)
  - `Check(state State) []Violation` 评估 invariant set
  - 性能 bound：`check_latency <= 5ms_per_turn`（spec.md §LTL-Lite Self-Invariants）
- **文件 3:** `internal/shared/ltllite/parser_test.go` (新建, ~120 行)
  - `TestParse_ValidInvariant`
  - `TestParse_Invalid_NoOperator`
  - `TestParse_MultipleFields`
- **文件 4:** `internal/shared/ltllite/check_test.go` (新建, ~120 行)
  - `TestCheck_AllHold_NoViolation`
  - `TestCheck_OneViolated_ReturnsViolation`
  - `TestCheck_LatencyBound_5ms`
- **依赖:** 无
- **AC:** Phase 1.5 不变式规约框架（前置）
- **T:** PERMISSION-GATE-1-T01（LTL-Lite runtime check）
- **估时参考:** 180 min

### W15 — PERMISSION-GATE-1 CI Lint + 每个 Surface `_invariant.go`

- **文件 1:** `tools/ci-lint-invariant/main.go` (新建, ~150 行)
  - 扫描 `internal/layers/**/*surface*` 目录，验证 `_invariant.go` 存在 + 非空
  - 解析所有 invariant tag，报告 malformed
  - 跨 Surface 冲突检测
- **文件 2:** `.github/workflows/ci.yml` (修改)
  - 新增 LTL-Lite Invariant Lint step（spec.md §Requirement: CI Workflow Adds Invariant Lint Step）
- **文件 3:** 各 Surface `_invariant.go`（5 个，新建, 每个 ~40 行）
  - `internal/layers/contextengine/enforce/toolrunner/surface/lsp_surface_invariant.go`
  - `internal/layers/contextengine/enforce/toolrunner/surface/bash_surface_invariant.go`
  - `internal/layers/contextengine/enforce/toolrunner/tracker/_invariant.go`
  - `internal/layers/multiagent/freefork/_invariant.go`
  - `internal/layers/evolution/guard/verify_plan_execution/_invariant.go`
- **文件 4:** `internal/layers/orchestration/turn_adapter/ltl_hook.go` (新建, ~80 行)
  - `turn_adapter.Prepare` 调 `ltllite.Check(allSurfaceState)`
  - 任何 violation → `ErrInvariantViolation` 中止 turn
  - 任何 surface.Execute 前重检相关 invariant
- **文件 5:** `tools/ci-lint-invariant/main_test.go` (新建, ~120 行)
  - `TestLint_MissingInvariantFile`
  - `TestLint_MalformedTag`
  - `TestLint_CrossSurfaceConflict`
- **依赖:** W14
- **AC:** Phase 1.5 LTL-Lite 框架集成
- **T:** PERMISSION-GATE-1-T02（CI lint invariant existence）
- **估时参考:** 240 min

---

## Phase 8: 全量回归 + E2E + 验收 + 归档（W16-W17）

### W16 — Phase 1 全量回归 + E2E IM 验证

- **文件 1:** `tests/integration/tools_terminal_test.go` (新建, ~250 行)
  - `TestLSP_End2End`（飞书发 lsp_go_to_definition → 返回 Location）
  - `TestBashAST_DenyAttack`（飞书发 `rm -rf /tmp/*` → Deny）
  - `TestFreeFork_3Directions`（飞书发 free_fork n=3 → 3 个子代理 + worktree）
  - `TestVerify_AllPass`（跑 tasks.md → Pass）
  - `TestTracker_NonBlocking`（高频编辑不阻塞）
- **文件 2:** `tests/e2e/im_tools_terminal_test.go` (新建, ~200 行)
  - 5 步骤 E2E：lsp → bash → fork → edit → verify（飞书 IM 交互）
- **文件 3:** `tests/integration/ltl_lite_test.go` (新建, ~150 行)
  - `TestLTL_AllSurfaces_ParseSuccess`
  - `TestLTL_Violation_AbortTurn`
  - `TestLTL_CrossSurface_ConflictDetected`
- **验证:**
  - `go test -race ./...` 100% 绿
  - `go vet ./...` + `staticcheck ./...` 无新增 warning
  - LTL-Lite CI lint step 通过
  - 飞书 IM E2E 5 步全部通过
  - 25 个 T 点全部 PASS
- **依赖:** W1-W15
- **AC:** AC1-AC7 + AC25（Surface 合并异质性门槛验证）
- **T:** 全部 25 个 T 点最终验证
- **估时参考:** 240 min

### W17 — S5 验收 + S6 归档

- **文件 1:** `openspec/changes/devrix-tools-terminal-architecture/acceptance-report.md` (新建, ~120 行)
  - 27 个 AC（7 P0 + 20 P1）状态表
  - T 层验证：25 个 P0 全绿（覆盖率 ≥ 80%）
  - 跨域一致性：D2-D3 import lint 通过 + 11 个限界上下文边界验证
  - 风险评估：5 类风险实际影响 vs 设计预期对比
- **文件 2:** `openspec/t-registry.md` (修改, +30 行)
  - 登记 25 个 T 点（PLANNED → ACTIVE）
- **文件 3:** `openspec/specs/tool-surface/t-registry.md` (修改, +20 行)
  - 同步新增 T 点到域注册表
- **文件 4:** `docs/methodology/dsaft-methodology.md` (修改, +40 行)
  - 补充"LTL-Lite 不变式规约"案例：Go struct tag DSL + `_invariant.go`
  - 引用本 change 的 `internal/shared/ltllite/`
- **操作 1:** `git add` + `git commit`（独立 commit，按 Phase 组织）
- **操作 2:** `bash scripts/verify-archive.sh` 全部通过
- **操作 3:** 归档到 `openspec/archive/2026-06-18-devrix-tools-terminal-architecture/`
- **操作 4:** 开 PR（squash merge + auto-merge，单人 0 approval）
- **操作 5:** S7_Archived 状态
- **依赖:** W16
- **AC:** 全部 AC 通过
- **T:** 全量回归报告
- **估时参考:** 90 min

---

## 总览

| Phase | W 编号 | 主题 | 估时参考 | AC | 吸收 SUG |
|-------|--------|------|---------|----|---------|
| P0 共享 | W0 | 依赖锁定 + devrix.yaml 配置 | 45 min | AC7 | SUG-5/6 |
| P1 LSP | W1-W3 | 6 F + 6 T | 540 min | AC1 + AC6 | SUG-2 |
| P2 BashAST | W4-W5 | 3 F + 6 T | 420 min | AC2 | SUG-6 |
| P3 Tracker | W6-W7 | 3 F + 5 T | 360 min | AC3 | SUG-5 |
| P4 FreeFork | W8-W10 | 4 F + 5 T | 420 min | AC4 | SUG-1 |
| P5 Verify | W11-W12 | 3 F + 3 T | 420 min | AC5 | SUG-5 |
| P6 Event | W13 | 1 F | 120 min | AC4（部分）| — |
| P7 LTL-Lite | W14-W15 | 跨切面 2 T | 420 min | Phase 1.5 前置 | — |
| P8 回归+归档 | W16-W17 | E2E + verify-archive | 330 min | 全 AC | — |
| **合计** | **17** | **23 F + 25 T + 2 跨切面 T** | **~51 h** | **27 AC** | **4 必收 SUG** |

> **注意：** 估时仅供参考，非承诺。实际进度按 Phase 推进。

## 执行顺序

1. **W0**（Phase 0 — 共享基础）
2. **W1 → W2 → W3**（Phase 1 — LSP 串行，依赖 W0）
3. **W4 → W5**（Phase 2 — BashAST 串行，依赖 W0）
4. **W6 → W7**（Phase 3 — Tracker 串行，依赖 W0）
5. **W8 → W9 → W10**（Phase 4 — FreeFork 串行，依赖 W0）
6. **W11 → W12**（Phase 5 — Verify 串行，依赖 W0）
7. **W13**（Phase 6 — Event，可与 P4/P5 并行）
8. **W14 → W15**（Phase 7 — LTL-Lite，可与 P1-P6 并行）
9. **W16**（Phase 8 — 全量回归，依赖 W1-W15）
10. **W17**（Phase 8 — 归档，依赖 W16）

**关键并行机会**：W3（lsp surface 集成）、W5（bash surface 集成）、W7（edit surface 集成）、W9（freefork surface 集成）、W12（verify surface 集成）、W13、W14 — 都依赖各自的 W 前置，但相互之间可并行（不同 surface 互不依赖）。

每个 W 完成后立即 `git add` + `git commit`（独立 commit），便于回滚
与 review。

## 风险与缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| LSP gopls 进程池 4 上限在大型项目不够 | M | W16 全量回归时 benchmark；如超限，调整为 8 但保留 invariant 软警告 |
| Bash AST 规则集（20+）误判率高 | M | W16 收集飞书 IM 真实流量做误判率统计；SUG-4 留通道 |
| mvdan.cc/sh v3 → v4 重大变更 | L | W0 + SUG-6 已锁定 v3.x + CI 检查 |
| FreeFork 8 并发在 4-core 机器 OOM | L | W0 默认 8 + 可配置；W10 worktree 隔离避免污染 |
| LTL-Lite 5ms bound 在 12+ Surface 不达标 | M | SUG-8 lazy check on surface load；W14 实现 + benchmark |
| Phase 1 改动大、回归面广 | H | W16 全量 E2E 验证；任一 case 失败可独立 revert 对应 Phase |
| 25 个 T 点分散在多个 PR | L | S3-Gate 已通过 review-design 验证覆盖；S5 一次性验收 |

## 文件交付清单（按 W 汇总）

详见 design.md §7 文件清单 + §10 测试矩阵。

### 新增文件 (~30 个)

**`internal/shared/ltllite/` (+4):**
- `parser.go` (W14)
- `check.go` (W14)
- `parser_test.go` (W14)
- `check_test.go` (W14)

**`internal/shared/config/` (+1):**
- `contextengine_test.go` (W0)

**`internal/layers/contextengine/lsp/` (+7):**
- `server.go` (W1)
- `pool.go` (W1)
- `pool_test.go` (W1)
- `adapter.go` (W2)
- `types.go` (W2)
- `adapter_test.go` (W2)
- `testdata/gopls_mock.go` (W2)

**`internal/layers/contextengine/enforce/toolrunner/bash/` (+5):**
- `parser.go` (W4)
- `heredoc.go` (W4)
- `parser_test.go` (W4)
- `zsh_rules.go` (W5)
- `policy.go` (W5)
- `policy_test.go` (W5)

**`internal/layers/contextengine/enforce/toolrunner/tracker/` (+8):**
- `diff.go` (W6)
- `lru.go` (W6)
- `record.go` (W6)
- `record_test.go` (W6)
- `async.go` (W7)
- `linter.go` (W7)
- `async_test.go` (W7)
- `linter_test.go` (W7)

**`internal/layers/multiagent/freefork/` (+9):**
- `forker.go` (W8, W10 修改)
- `messaging.go` (W8)
- `worker_context.go` (W8)
- `forker_test.go` (W8)
- `arbitration.go` (W9)
- `arbitration_test.go` (W9)
- `worktree.go` (W10)
- `worktree_test.go` (W10)

**`internal/layers/evolution/guard/verify_plan_execution/` (+5):**
- `parser.go` (W11)
- `executor.go` (W11)
- `aggregator.go` (W12)
- `parser_test.go` (W11)
- `executor_test.go` (W11)
- `aggregator_test.go` (W12)

**`internal/layers/multiagent/runner/` (+2):**
- `events.go` (W13)
- `events_test.go` (W13)

**`internal/layers/observability/metrics/` (+2):**
- `lsp.go` (W3)
- `lsp_test.go` (W3)

**`internal/layers/orchestration/turn_adapter/` (+1):**
- `ltl_hook.go` (W15)

**`tools/ci-lint-invariant/` (+2):**
- `main.go` (W15)
- `main_test.go` (W15)

**`tests/integration/` (+2):**
- `tools_terminal_test.go` (W16)
- `ltl_lite_test.go` (W16)

**`tests/e2e/` (+1):**
- `im_tools_terminal_test.go` (W16)

**`openspec/changes/devrix-tools-terminal-architecture/` (+1):**
- `acceptance-report.md` (W17)

**各 Surface `_invariant.go`（5 个）** (W15)

### 修改文件 (~10 个)

- `go.mod` (W0, 锁定 mvdan.cc/sh v3.x)
- `.github/workflows/ci.yml` (W0 + W15)
- `internal/shared/config/contextengine.go` (W0, +40 行)
- `internal/layers/contextengine/enforce/toolrunner/surface/lsp_surface.go` (W3, +80 行)
- `internal/layers/contextengine/enforce/toolrunner/surface/bash_surface.go` (W5, +50 行)
- `internal/layers/contextengine/enforce/toolrunner/surface/edit_surface.go` (W7, +60 行)
- `internal/layers/contextengine/enforce/toolrunner/surface/freefork_surface.go` (W9, +50 行)
- `internal/layers/contextengine/enforce/toolrunner/surface/verify_surface.go` (W12, +50 行)
- `internal/layers/contextengine/enforce/toolrunner/surface/background_task_surface.go` (W13, +50 行)
- `internal/layers/evolution/reputation/store.go` (W12, +40 行)
- `internal/layers/multiagent/freefork/forker.go` (W10, +30 行)
- `openspec/t-registry.md` (W17, +30 行)
- `openspec/specs/tool-surface/t-registry.md` (W17, +20 行)
- `docs/methodology/dsaft-methodology.md` (W17, +40 行)

## T 层测试点登记

| T ID | 描述 | 阶段 | W |
|------|------|------|---|
| D2-S4-A01-T01 | goToDefinition 基本返回 | P0 | W1 |
| D2-S4-A01-T02 | findReferences 基本返回 | P0 | W2 |
| D2-S4-A01-T03 | incomingCalls/hover/workspaceSymbol | P0 | W2 |
| D2-S4-A01-T04 | LSPSurface.Execute 暴露 5 method | P0 | W3 |
| D2-S4-A01-T05 | LRU 池 ≤ 4（硬上限）| P0 | W1, W3 |
| D2-S4-A01-T06 | LSP 超时 fallback grep | P0 | W3 |
| TOOL-SEC-2-A02-T01 | AST 解析基本功能 | P0 | W4 |
| TOOL-SEC-2-A02-T02 | 解析失败 fail-closed | P0 | W4 |
| TOOL-SEC-2-A02-T03 | heredoc 检测 | P0 | W4 |
| TOOL-SEC-2-A02-T04 | 嵌套 heredoc 拒绝 | P0 | W4 |
| TOOL-SEC-2-A02-T05 | 20+ zsh 攻击模式 | P0 | W5 |
| TOOL-SEC-2-A02-T06 | Bash Surface 集成 | P0 | W5 |
| D5-S23-A02-T01 | diff 收集 | P0 | W6 |
| D5-S23-A02-T02 | LRU 去重 | P0 | W6 |
| D5-S23-A02-T03 | 异步触发不阻塞 | P0 | W7 |
| D5-S23-A02-T04 | Linter 集成 | P0 | W7 |
| D5-S23-A02-T05 | EditSurface 集成 | P0 | W7 |
| D4-S11-A02-T01 | ForkAgent 创建（≤ 8 并发）| P0 | W8 |
| D4-S11-A02-T02 | SendMessage 通道 | P0 | W8 |
| D4-S11-A02-T03 | 资源争抢仲裁 | P0 | W9 |
| D4-S11-A02-T04 | 60s timeout 中止 | P0 | W8 |
| D4-S13-A02-T01 | Worktree 隔离 | P0 | W10 |
| D6-S11-A02-T01 | tasks.md 解析 | P0 | W11 |
| D6-S11-A02-T02 | 4 类验证执行 | P0 | W11 |
| D6-S11-A02-T03 | 结果聚合 + D6 reputation | P0 | W12 |
| D4-S12-A03-T01 | 后台任务事件流推送 | P0 | W13 |
| PERMISSION-GATE-1-T01 | LTL-Lite runtime check（≤ 5ms）| P0（跨切面）| W14 |
| PERMISSION-GATE-1-T02 | CI lint invariant existence | P0（跨切面）| W15 |

---

> **S4 → S5 接力**: 17 个 W 全部 PASS 后，进入 S5 验收（按
> `openspec/specs/project/testing.md` + `acceptance-report.md` 27 AC）。
> S5 通过后 S6 归档（PR + `verify-archive.sh`）。

---

## 附录: SUG 吸收追踪

| SUG | 严重度 | 吸收 W | 状态 |
|-----|------|--------|------|
| SUG-1 WorkerContext budget 共享 | Medium | W8（WorkerContext + Forker 测试）| ✅ 已吸收 |
| SUG-2 LSP SLO 监控 | Medium | W3（lsp.go metrics + threshold）| ✅ 已吸收 |
| SUG-3 Surface 数量季度评估 | Low | — | ⏸️ 工程流程级，文档化 |
| SUG-4 BashAST 误判反馈通道 | Low | — | ⏸️ Phase 2 backlog（W16 可加 IM 集成）|
| SUG-5 devrix.yaml 配置项 | Low | W0（config 扩展）+ W1/W4/W6/W8/W11 引用 | ✅ 已吸收 |
| SUG-6 库版本锁定 | Low | W0（go.mod + CI）| ✅ 已吸收 |
| SUG-7 AC6 跨 spec 引用 | Low | — | ⏸️ docs 级，S6 归档时同步 |
| SUG-8 LTL-Lite lazy check | Low | — | ⏸️ Phase 1.5 实施细节，W14 优化空间 |

**4 个必收 SUG 全部吸收**。4 个 Low SUG 中 SUG-3/SUG-7 文档级，SUG-4/SUG-8 留待 Phase 1.5 / Phase 2 实施。
