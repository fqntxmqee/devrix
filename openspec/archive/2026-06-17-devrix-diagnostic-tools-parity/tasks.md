# Tasks: 诊断工具对标 clawcode

**Change ID:** devrix-diagnostic-tools-parity
**Demand ID:** DM-20260616-003
**Status:** S4_Implementation
**Last updated:** 2026-06-17

---

> **能力别名前缀 (Capability Aliases)**
>
> 本 change 遵循 DSAFT 域-场景-活动-功能-任务五层命名作为权威 ID。G1-G6 / A1-A7 是 S2 阶段为方便对照 `docs/reference/clawcode-diagnostic-tools-analysis.md` 而保留的需求侧别名前缀。W{N} 任务行项沿用 capability-DSAFT Activity 双标（`DSAFT Activity (alias Gx/Ax)`）。

---

## W1 — D2-S6-A02 共享错误栈截断 (alias A7)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W1.1 | 实现 `ShortStack` / `WithShortStack` / `FormatStack` API | `internal/shared/errors/shortstack.go` | done |
| W1.2 | 过滤 runtime/testing/reflect 噪声 | `internal/shared/errors/shortstack.go` | done |
| W1.3 | 单元测试（深度 ≥ 3、并发安全、nil-safe） | `internal/shared/errors/shortstack_test.go` | done |

## W2 — D3-S3-A02 LLM 错误分类 (alias A6)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W2.1 | 实现 21 个 Class 常量 | `internal/layers/llmgateway/protect/errorclass/classifier.go` | done |
| W2.2 | 三层匹配:LlmError.Code + errors.Is + HTTP status + regex | `internal/layers/llmgateway/protect/errorclass/classifier.go` | done |
| W2.3 | ctx propagation:`InjectClassification` / `FromContext` | `internal/layers/llmgateway/protect/errorclass/classifier.go` | done |
| W2.4 | 20+ 内置 regex 规则（rate_limit / quota / auth / network / stream_error 等） | `internal/layers/llmgateway/protect/errorclass/classifier.go` | done |
| W2.5 | 单元测试（每个 Class 至少一个 case） | `internal/layers/llmgateway/protect/errorclass/classifier_test.go` | done |

## W3 — D2-S4-A01 LSP 代码智能工具 (alias G1)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W3.1 | 定义 `Position/Range/Location/SymbolKind/CallHierarchyItem/CallHierarchyIncomingCall` 类型 | `internal/shared/lsp/types.go` | done |
| W3.2 | `Client` interface（Initialize / DidOpen / Definition / References / PrepareCallHierarchy / IncomingCalls / Close） | `internal/shared/lsp/types.go` | done |
| W3.3 | `SandboxLauncher` interface + `ExecLauncher` | `internal/shared/lsp/types.go` | done |
| W3.4 | `rpcClient`:Content-Length 帧协议 + request/notify | `internal/shared/lsp/types.go` | done |
| W3.5 | `lspClient` 实现 Client 接口 | `internal/shared/lsp/types.go` | done |
| W3.6 | `Manager`:500-file LRU + per-language routing + 并发上限 | `internal/shared/lsp/manager.go` | done |
| W3.7 | `lsp_tool` runner:definition / references / incoming_calls 三种 operation | `internal/layers/contextengine/enforce/toolrunner/lsp_tool.go` | done |
| W3.8 | 单元测试（LRU eviction / Acquire 同 rootURI 复用 / Client 错误处理） | `internal/shared/lsp/*_test.go`、`internal/layers/contextengine/enforce/toolrunner/lsp_tool_test.go` | done |
| W3.9 | `RegisterLSPTool` bootstrap 入口（默认 disabled） | `internal/layers/contextengine/enforce/toolrunner/lsp_register.go` | done |
| W3.10 | 接入 `context_engine_builder.go` 工具注册流程 | `internal/bootstrap/context_engine_builder.go` | done |

## W4 — TOOL-SEC-2-A02 Bash AST 安全分析器 (alias G2)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W4.1 | 添加 mvdan.cc/sh/v3 依赖 | `go.mod` | done |
| W4.2 | 实现 9 个 FindingKind 常量 | `internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer.go` | done |
| W4.3 | `Analyzer.Analyze`:string 预检 + AST walk + heredoc body 检查 | `internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer.go` | done |
| W4.4 | mvdan.cc/sh 类型适配(`*syntax.File` / `Pos.Line()` uint / `*syntax.Word` Parts) | `internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer.go` | done |
| W4.5 | panic 兜底 (`defer recover()` → Allow=true fallback regex) | `internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer.go` | done |
| W4.6 | `PolicyAnalyzer` 适配器(sandboxast.Analyzer → toolrunner.ASTAnalyzer) | `internal/layers/contextengine/enforce/toolrunner/sandboxast/policy_adapter.go` | done |
| W4.7 | `CommandPolicy.ASTAnalyzer` 字段 + `Validate()` 前置调用 | `internal/layers/contextengine/enforce/toolrunner/sandbox.go` | done |
| W4.8 | 单元测试(heredoc_injection / zsh_attack / command_subst / process_subst / dangerous_redirect / eval_call / nested_escape / compound) | `internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer_test.go` | done |

## W5 — D4-S12-A03 后台任务完成通知总线 (alias G3)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W5.1 | `Bus` interface + `InMemoryBus` 实现(每 session buffered channel + pending list) | `internal/layers/orchestration/workmodel/notify/bus.go` | done |
| W5.2 | `CompletionEvent` struct(TaskID / Kind / ExitCode / Duration / TailLines / Error / Summary / Time) | `internal/layers/orchestration/workmodel/notify/bus.go` | done |
| W5.3 | `FormatReminder` 渲染 `<task_notifications>` 块 | `internal/layers/orchestration/workmodel/notify/bus.go` | done |
| W5.4 | `GlobalBus()` 进程级单例 + `SetGlobalBus` 注入 | `internal/layers/orchestration/workmodel/notify/wire.go` | done |
| W5.5 | `TaskManager.UpdateStatus` 进入终态时 `go publishCompletion` 钩子 | `internal/layers/orchestration/workmodel/task_manager.go` | done |
| W5.6 | 单元测试(Publish/Drain/Subscribe/channel 满溢出/session 隔离/concurrent) | `internal/layers/orchestration/workmodel/notify/bus_test.go` | done |

## W6 — D6-S11-A02 实现后自动验证 (alias G4)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W6.1 | `PlanItem` / `Evidence` / `Report` / `Verifier` interface | `internal/layers/evolution/verify/plan.go` | done |
| W6.2 | `FileVerifier.LoadPlan`:解析 `| W{N}.{M} | desc | file | done|pending |` 行 | `internal/layers/evolution/verify/plan.go` | done |
| W6.3 | `FileVerifier.Verify`:file 存在性 + `_test.go` 含 `func TestXxx(` 检查 | `internal/layers/evolution/verify/plan.go` | done |
| W6.4 | `FormatJSON` 报告序列化 | `internal/layers/evolution/verify/plan.go` | done |
| W6.5 | 单元测试(行解析 / 缺文件 / 测试函数缺失 / ctx 取消) | `internal/layers/evolution/verify/plan_test.go` | done |

## W7 — D4-S11-A02 + D4-S13-A02 自由分叉子代理 (alias G5)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W7.1 | `ForkRequest` / `Handle` / `Forker` interface | `internal/layers/multiagent/provision/freefork/forker.go` | done |
| W7.2 | `DefaultForker.Fork`:批量 + 并行 + worktree 隔离 (D4-S13-A02) | `internal/layers/multiagent/provision/freefork/forker.go` | done |
| W7.3 | 失败回滚(已启动子 agent Terminate + worktree Exit) | `internal/layers/multiagent/provision/freefork/forker.go` | done |
| W7.4 | `slugify` 规整 Name → worktree slug | `internal/layers/multiagent/provision/freefork/forker.go` | done |
| W7.5 | 单元测试(批量 / 失败回滚 / prompt 传递 / slug) | `internal/layers/multiagent/provision/freefork/forker_test.go` | done |

## W8 — D5-S23-A03 /doctor 自检命令 (alias A1)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W8.1 | `Check` / `Doctor` interface + `DefaultDoctor` 实现 | `internal/layers/observability/diagnose/doctor/doctor.go` | done |
| W8.2 | 7 项内置 check(install_paths / config_yaml_valid / lsp_servers_reachable / workdir_writable / observability_ready / tool_count / transcript_dir_ok) | `internal/layers/observability/diagnose/doctor/doctor.go` | done |
| W8.3 | `FormatJSON` / `FormatTable` / `Summary` 渲染 | `internal/layers/observability/diagnose/doctor/doctor.go` | done |
| W8.4 | 单元测试(7 项 check / 状态聚合 / JSON 序列化) | `internal/layers/observability/diagnose/doctor/doctor_test.go` | done |

## W9 — D5-S24-A02 Debug 日志分类过滤 (alias A2)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W9.1 | `Filter` 包装 `logger.Handler`,按 Component 字段过滤 debug 级别 | `internal/layers/observability/instrument/logger/debugfilter/filter.go` | done |
| W9.2 | `WithPassthroughNonDebug` 切换非 debug 级别放行(默认 true) | `internal/layers/observability/instrument/logger/debugfilter/filter.go` | done |
| W9.3 | 单元测试(categories 过滤 / 非 debug 放行 / 未知 Component 拦截 / nil-safe) | `internal/layers/observability/instrument/logger/debugfilter/filter_test.go` | done |

## W10 — D1-S2-A02 会话转录持久化 (alias A3)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W10.1 | `Event` / `Writer`:每 session 一个 .jsonl 文件 | `internal/layers/communication/capture/transcript/writer.go` | done |
| W10.2 | `Append` 串行追加(全局 mutex 序列化) | `internal/layers/communication/capture/transcript/writer.go` | done |
| W10.3 | `LoadReader` 读全部 events(`--continue` 重建 context 用) | `internal/layers/communication/capture/transcript/writer.go` | done |
| W10.4 | `ListSessions` 按 mtime 倒序 | `internal/layers/communication/capture/transcript/writer.go` | done |
| W10.5 | `sanitize` 防 path traversal | `internal/layers/communication/capture/transcript/writer.go` | done |
| W10.6 | `GlobalWriter` 进程级单例 + `Append` 快捷方法 | `internal/layers/communication/capture/transcript/wire.go` | done |
| W10.7 | 单元测试(追加 / 读回 / 并发 / 路径遍历) | `internal/layers/communication/capture/transcript/writer_test.go` | done |

## W11 — D5-S23-A04 故障注入 (alias A4)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W11.1 | `Rule` / `Injector`(testbuild tag) | `internal/layers/observability/diagnose/faultinject/injector.go` | done |
| W11.2 | `Hook` 解析 `DEVRIX_FAULT_INJECT` env 规则 | `internal/layers/observability/diagnose/faultinject/injector.go` | done |
| W11.3 | mode=error/latency/truncate 处理 | `internal/layers/observability/diagnose/faultinject/injector.go` | done |
| W11.4 | :once 后缀只触发一次 | `internal/layers/observability/diagnose/faultinject/injector.go` | done |
| W11.5 | 生产 no-op stub(`!testbuild` tag) | `internal/layers/observability/diagnose/faultinject/injector_prod.go` | done |
| W11.6 | 单元测试(env 解析 / once / reset / AddRule / latency / nil-safe) | `internal/layers/observability/diagnose/faultinject/injector_test.go` | done |

## W12 — D2-S6-A03 上下文窗口分析 (alias A5)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W12.1 | `Breakdown` / `MessageView` / `Analyzer` interface | `internal/layers/contextengine/token/windowanalyzer/analyzer.go` | done |
| W12.2 | `TokenAnalyzer.Analyze` 累计各 Category token 数 | `internal/layers/contextengine/token/windowanalyzer/analyzer.go` | done |
| W12.3 | `AnalyzeMessages` role-based 分类(system→System/tool→Tools/<thinking>→Thinking/<reminder>+<task_notifications>→Reminders) | `internal/layers/contextengine/token/windowanalyzer/analyzer.go` | done |
| W12.4 | `FormatTable` ASCII 进度条 | `internal/layers/contextengine/token/windowanalyzer/analyzer.go` | done |
| W12.5 | 单元测试(各 category 计数 / role 路由 / thinking/reminder 标记) | `internal/layers/contextengine/token/windowanalyzer/analyzer_test.go` | done |

## W13 — D5-S23-A02 诊断跟踪器 (alias G6,前序已实现)

| ID | 任务 | File | Status |
|----|------|------|--------|
| W13.1 | 500-file LRU + async SnapshotBefore + Diff | `internal/layers/observability/diagnose/tracker/tracker.go` | done |
| W13.2 | 3 内置 linter:goVetLinter / tscLinter / shellcheckLinter | `internal/layers/observability/diagnose/tracker/tracker.go` | done |
| W13.3 | R2 缓解:Diff 只报 NEW diagnostics | `internal/layers/observability/diagnose/tracker/tracker.go` | done |
| W13.4 | `AppendToReminder` 渲染 `<file_diagnostics>` 块 | `internal/layers/observability/diagnose/tracker/tracker.go` | done |

## W14 — Spec & T-registry updates

| ID | 任务 | File | Status |
|----|------|------|--------|
| W14.1 | 在 `d1/spec.md` 加 transcript (D1-S2-A02) Gherkin scenario | `openspec/specs/d1-communication/spec.md` | pending |
| W14.2 | 在 `d2/spec.md` 加 LSP (D2-S4-A01) / windowanalyzer (D2-S6-A03) scenarios | `openspec/specs/d2-context-engine/spec.md` | pending |
| W14.3 | 在 `d3/spec.md` 加 ErrorClassifier (D3-S3-A02) scenarios | `openspec/specs/d3-llm-gateway/spec.md` | pending |
| W14.4 | 在 `d4/spec.md` 加 Free Fork (D4-S11-A02) / Task Notify (D4-S12-A03) scenarios | `openspec/specs/d4-multi-agent/spec.md` | pending |
| W14.5 | 在 `d5/spec.md` 加 Diagnostic Tracker (D5-S23-A02) / doctor (D5-S23-A03) / debugfilter (D5-S24-A02) / fault inject (D5-S23-A04) scenarios | `openspec/specs/d5-observability/spec.md` | pending |
| W14.6 | 在 `d6/spec.md` 加 Verifier (D6-S11-A02) scenarios | `openspec/specs/d6-evolution/spec.md` | pending |
| W14.7 | 在 `tool-security/spec.md` 加 Bash AST (TOOL-SEC-2-A02) scenarios | `openspec/specs/tool-security/spec.md` | pending |
| W14.8 | 更新 7 个域的 t-registry.md | `openspec/specs/d{1..6}-*/t-registry.md` + `openspec/specs/tool-security/t-registry.md` | pending |

## W15 — Documentation

| ID | 任务 | File | Status |
|----|------|------|--------|
| W15.1 | 写 `tasks.md`(本文件) | `openspec/changes/devrix-diagnostic-tools-parity/tasks.md` | done |
| W15.2 | 写 `acceptance-report.md`(S5) | `openspec/changes/devrix-diagnostic-tools-parity/acceptance-report.md` | pending |
| W15.3 | 归档 change directory(S6) | `openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/` | pending |
| W15.4 | docs 重构 — 以 DSAFT Activity 为权威 ID,G1-G6 / A1-A7 降级为 alias | 本 archive 目录 6 个 .md | done (PR #TBD) |

---

## File Manifest

### New files
- `internal/shared/errors/shortstack.go` + test — D2-S6-A02 (alias A7)
- `internal/layers/llmgateway/protect/errorclass/classifier.go` + test — D3-S3-A02 (alias A6)
- `internal/layers/observability/diagnose/tracker/tracker.go` + test — D5-S23-A02 (alias G6)
- `internal/shared/lsp/types.go` + test — D2-S4-A01 (alias G1)
- `internal/shared/lsp/manager.go` + test — D2-S4-A01 (alias G1)
- `internal/layers/contextengine/enforce/toolrunner/lsp_tool.go` + test — D2-S4-A01 (alias G1)
- `internal/layers/contextengine/enforce/toolrunner/lsp_register.go` — D2-S4-A01 (alias G1)
- `internal/layers/contextengine/enforce/toolrunner/sandboxast/analyzer.go` + test — TOOL-SEC-2-A02 (alias G2)
- `internal/layers/contextengine/enforce/toolrunner/sandboxast/policy_adapter.go` — TOOL-SEC-2-A02 (alias G2)
- `internal/layers/orchestration/workmodel/notify/bus.go` + test — D4-S12-A03 (alias G3)
- `internal/layers/orchestration/workmodel/notify/wire.go` — D4-S12-A03 (alias G3)
- `internal/layers/evolution/verify/plan.go` + test — D6-S11-A02 (alias G4)
- `internal/layers/multiagent/provision/freefork/forker.go` + test — D4-S11-A02 + D4-S13-A02 (alias G5)
- `internal/layers/observability/diagnose/doctor/doctor.go` + test — D5-S23-A03 (alias A1)
- `internal/layers/observability/instrument/logger/debugfilter/filter.go` + test — D5-S24-A02 (alias A2)
- `internal/layers/communication/capture/transcript/writer.go` + test — D1-S2-A02 (alias A3)
- `internal/layers/communication/capture/transcript/wire.go` — D1-S2-A02 (alias A3)
- `internal/layers/observability/diagnose/faultinject/injector.go` (testbuild) — D5-S23-A04 (alias A4)
- `internal/layers/observability/diagnose/faultinject/injector_prod.go` (!testbuild) — D5-S23-A04 (alias A4)
- `internal/layers/observability/diagnose/faultinject/injector_test.go` (testbuild) — D5-S23-A04 (alias A4)
- `internal/layers/observability/diagnose/faultinject/sleep.go` (testbuild) — D5-S23-A04 (alias A4)
- `internal/layers/contextengine/token/windowanalyzer/analyzer.go` + test — D2-S6-A03 (alias A5)

### Modified files
- `internal/layers/contextengine/enforce/toolrunner/sandbox.go` — TOOL-SEC-2-A02 `ASTAnalyzer` 字段 + `Validate()` 前置调用
- `internal/layers/orchestration/workmodel/task_manager.go` — D4-S12-A03 `UpdateStatus` 进入终态时 publish notify event
- `internal/bootstrap/context_engine_builder.go` — D2-S4-A01 `RegisterLSPTool` 接入
- `go.mod` — 添加 `mvdan.cc/sh/v3` 依赖
