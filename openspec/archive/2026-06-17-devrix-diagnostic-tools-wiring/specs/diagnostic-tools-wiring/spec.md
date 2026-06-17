# Diagnostic Tools Wiring Specification

**Change ID:** devrix-diagnostic-tools-wiring
**Demand ID:** DM-20260617-002
**Module:** diagnostic-tools-wiring
**Status:** S3_Design

---

> **能力别名前缀 (Capability Aliases)**
>
> 本 spec 复用 DM-016 已确立的 14 个 DSAFT Activity 节点。G1-G6 / A1-A7 保留为需求侧 alias，便于对照 `docs/reference/clawcode-diagnostic-tools-analysis.md`。

---

## ADDED

### Requirement: /doctor 通过 IM 触发 (alias A1, D5-S23-A03-RunDoctor wiring)

#### Scenario: /doctor 通过飞书 IM 触发后返回 7 项 check status 表
- GIVEN 用户在飞书 IM 发送 "/doctor"
- WHEN adapters/cli.go handleDoctor 路由触发 doctor.NewDefaultDoctor.Run
- THEN 返回的 IM 消息包含 7 项 check 的 status (PASS/FAIL/WARN)
- AND Summary 是 7 项中最低等级
<!-- T: D5-S23-A03-T01 -->

#### Scenario: /doctor 检测缺失 LSP server 返回 StatusFail
- GIVEN devrix.yaml 配置了 lsp server "gopls-fake" 但不在 PATH
- WHEN doctor.Run 执行 lsp_servers_reachable check
- THEN 该 check 返回 StatusFail 且 Summary 是 StatusFail
<!-- T: D5-S23-A03-T02 -->

### Requirement: /context analyze 通过 IM 触发 (alias A5, D2-S6-A03-AnalyzeWindow wiring)

#### Scenario: /context analyze 通过飞书 IM 触发后返回 5 类 token 拆分表
- GIVEN session 包含 5 类 messages (system/assistant/tool/thinking/reminder)
- WHEN adapters/cli.go handleContextAnalyze 路由触发 windowanalyzer.AnalyzeMessages
- THEN 返回的 IM 消息包含 5 行 token 拆分 (system/messages/tools/thinking/reminders)
- AND 总和行 total 显示 100%
<!-- T: D2-S6-A03-T01 -->

### Requirement: G1 LSP Tool 由 devrix.yaml 启用后可被 LLM 调用 (alias G1, D2-S4-A01-ToolRegister wiring)

#### Scenario: devrix.yaml lsp.enabled=true 后 LLM 调用 lsp tool 成功
- GIVEN devrix.yaml 配置 lsp.enabled=true 和 servers: [{name: gopls, ...}]
- WHEN LLM 在 IM 回复中调用 lsp tool (operation=definition)
- THEN tool 返回 source file + line 而非 "lsp: tool is disabled"
- AND 5s 内返回
<!-- T: D2-S4-A01-T02 -->

#### Scenario: devrix.yaml 无 lsp 配置时 LLM 调用 lsp tool 返回 disabled 提示
- GIVEN devrix.yaml 不含 lsp 节
- WHEN LLM 调用 lsp tool
- THEN tool 返回 "lsp: tool is disabled (LSPConfig.Enabled=false)"
<!-- T: D2-S4-A01-T01 -->

### Requirement: G4 verify_plan_execution tool 可被 LLM 调用 (alias G4, D6-S11-A02-VerifyPlanExec wiring)

#### Scenario: verify_plan_execution tool 扫描 done items 返回 Verified 统计
- GIVEN tasks.md 有 3 个 done items 对应文件存在且含 _test.go with func TestXxx
- WHEN LLM 调用 verify_plan_execution tool
- THEN 返回 JSON {verified: 3, unverified: 0, skipped: pending_count}
<!-- T: D6-S11-A02-T01 -->

#### Scenario: verify_plan_execution tool 检测缺失文件
- GIVEN tasks.md 1 个 done item 引用不存在的文件
- WHEN LLM 调用 verify_plan_execution tool
- THEN 返回 unverified 列表含该 item 且 reason="file not found"
<!-- T: D6-S11-A02-T02 -->

### Requirement: G5 free_fork tool 批量分叉子代理 (alias G5, D4-S11-A02-ForkAgent + D4-S13-A02-IsolateWorktree wiring)

#### Scenario: free_fork tool 批量分叉 3 个子代理并返回 agent_ids
- GIVEN parent session "s1" 已存在且 GlobalForker 已注入
- WHEN LLM 调用 free_fork tool with requests=[3 个 ForkRequest]
- THEN tool 返回 {spawned_count: 3, agent_ids: [...]} JSON
- AND 每个子代理默认 worktree 隔离
<!-- T: D4-S11-A02-T01 -->

#### Scenario: free_fork tool 在 factory 失败时回滚
- GIVEN factory.Create 在第 2 个请求时失败
- WHEN LLM 调用 free_fork tool
- THEN tool 返回 error
- AND 已启动的 1 个子代理被 Terminate + worktree 清理
<!-- T: D4-S11-A02-T02 -->

#### Scenario: free_fork tool 限制最大请求数为 5
- GIVEN LLM 调用 free_fork tool with requests=[6 个 ForkRequest]
- WHEN tool 执行
- THEN 返回 error "free_fork: requests count must be in [1,5]"
<!-- T: D4-S11-A02-T01 (边界) -->

### Requirement: G6 query_diagnostics tool 与异步 tracker tick (alias G6, D5-S23-A02-TrackDiagnostics wiring)

#### Scenario: edit_file 后 5s 内 tracker 注入 file_diagnostics reminder
- GIVEN file foo.go 编译干净
- WHEN LLM 调用 edit_file 引入 unused import
- AND 5s 后 LLM 发起下一个请求
- THEN system prompt 的 <reminder> 段含 <file_diagnostics> 块
- AND 块内列新错误 (line + severity)
<!-- T: D5-S23-A02-T01 -->

#### Scenario: 无 edit 时 tracker 不注入 reminder
- GIVEN file foo.go 无错误
- WHEN 1s 后 LLM 发起下一个请求
- THEN system prompt 不含 <file_diagnostics> 块
<!-- T: D5-S23-A02-T02 -->

#### Scenario: query_diagnostics tool 返回当前 diff
- GIVEN tracker 已记录 1 个新错误
- WHEN LLM 调用 query_diagnostics tool
- THEN 返回该错误的 line + severity + message
<!-- T: D5-S23-A02-T01 (tool 入口) -->

### Requirement: A6 ErrorClassifier 注入到 LLM 网关错误响应 (alias A6, D3-S3-A02-ErrorMapping wiring)

#### Scenario: LLM 网关 HTTP 401 错误响应含 [class=AuthRequired] 标签
- GIVEN LLM 网关调用 provider 返回 HTTP 401
- WHEN DispatchInvoke 捕获错误
- THEN 错误字符串以 "[class=AuthRequired]" 开头
- AND ctx 中可通过 FromContext 取出 Classification{Class: AuthRequired}
<!-- T: D3-S3-A02-T01 -->

#### Scenario: LLMError.Code="rate_limit" 优先于 HTTP status 分类
- GIVEN err 是 *LLMError with Code=rate_limit 且 HTTP 200
- WHEN DispatchInvoke 调用 errorclass.Classify
- THEN Classification.Class=RateLimit（不是 Unclassified）
<!-- T: D3-S3-A02-T01 -->

### Requirement: A7 ShortStack 包装 sandbox 拒绝错误 (alias A7, D2-S6-A02-TruncateError wiring)

#### Scenario: sandbox 拒绝 heredoc 注入命令返回的栈 ≤ 5 帧
- GIVEN bash 命令 "cat <<EOF\n$(rm -rf /)\nEOF"
- WHEN sandbox.Validate 调用 errors.WithShortStack 包装
- THEN 渲染的错误栈 ≤ 5 帧
- AND runtime/testing/reflect 帧被过滤
<!-- T: D2-S6-A02-T01 -->

### Requirement: A7 ShortStack 包装 agent lifecycle 错误 (alias A7, D2-S6-A02-TruncateError wiring)

#### Scenario: agent spawn 失败错误栈 ≤ 5 帧
- GIVEN agent factory.Create 失败
- WHEN engine.spawnChild 返回 errors.WithShortStack 包装的错误
- THEN 错误栈 ≤ 5 帧且不含 runtime.goexit
<!-- T: D2-S6-A02-T01 -->

### Requirement: G2 Bash AST 通过 bootstrap 注入 (alias G2, TOOL-SEC-2-A02-ShellASTPolicy wiring)

#### Scenario: bootstrap 启动后 Bash AST 拦 heredoc body 内 $(...)
- GIVEN devrix.yaml 配置 toolrunner.sandbox.ast_enabled=true
- WHEN bash 命令 "cat <<EOF\n$(whoami)\nEOF" 被 sandbox.Validate 评估
- THEN 错误信息含 "sandbox: ast block: FindingHeredocInjection"
- AND 命令不执行
<!-- T: TOOL-SEC-2-A02-T01 -->

#### Scenario: Bash AST 不拦合法命令
- GIVEN bash 命令 "ls -la /tmp"
- WHEN sandbox.Validate 评估
- THEN 通过（Allow=true）且命令执行
<!-- T: TOOL-SEC-2-A02-T02 -->

#### Scenario: Bash AST 拦 zmodload zsh 攻击
- GIVEN bash 命令 "zmodload zsh/sys"
- WHEN sandbox.Validate 评估
- THEN 返回 "sandbox: ast block: FindingZshAttack"
<!-- T: TOOL-SEC-2-A02-T01 (zsh 子能力) -->

### Requirement: A2 DebugFilter 通过 CLI flag 启动时过滤日志 (alias A2, D5-S24-A02-ConfigureDebugFilter wiring)

#### Scenario: --debug=api,hooks 启动后仅 api/hooks 组件的 DEBUG 通过
- GIVEN devrix 启动时 --debug=api,hooks
- AND 3 个 debug log entry (component: api/hooks/telemetry) 产生
- WHEN filter.Handle 处理
- THEN 仅 api 和 hooks 的 entry 被转发到 inner handler
- AND telemetry entry 被过滤
<!-- T: D5-S24-A02-T01 -->

#### Scenario: 非 DEBUG 级别不受 filter 影响
- GIVEN devrix 启动时 --debug=api
- WHEN 一个 INFO 级别 entry with Component=hooks 产生
- THEN entry 被转发（passthroughNonDebug=true）
<!-- T: D5-S24-A02-T02 -->

### Requirement: A3 Transcript 在 session 关闭时持久化 (alias A3, D1-S2-A02-PersistTranscript wiring)

#### Scenario: session 关闭后 transcript.jsonl 写入
- GIVEN session "s1" 已运行并产生 5 个 events
- WHEN capture.SessionStore.Close 钩子触发
- THEN <transcript_dir>/s1.jsonl 存在且含 5 行 NDJSON
<!-- T: D1-S2-A02-T01 -->

#### Scenario: ListSessions 按 mtime 倒序返回
- GIVEN transcript_dir 有 3 个 .jsonl 文件 mtime 不同
- WHEN writer.ListSessions 调用
- THEN 返回按 mtime 降序的 session id 列表
<!-- T: D1-S2-A02-T01 (ListSessions) -->

### Requirement: G3 Task Notify 通过 prompt assembler drain 注入 (alias G3, D4-S12-A03-NotifyChild wiring)

#### Scenario: task 完成事件被 drain 到下一个 prompt 的 task_notifications 块
- GIVEN session "s1" 通过 /task create 添加任务后 /task update status=completed
- AND notify.GlobalBus() 持有 1 个未消费 CompletionEvent
- WHEN LLM 发起下一个请求
- THEN system prompt 的 <reminder> 段含 <task_notifications>...</task_notifications> 块
- AND 块内容为 notify.FormatReminder([event])
<!-- T: D4-S12-A03-T01 -->

#### Scenario: notify bus 为空时不注入 task_notifications 块
- GIVEN notify.GlobalBus() 为空或 session 无 pending event
- WHEN prompt assembler drain
- THEN system prompt 不含 <task_notifications> 块
<!-- T: D4-S12-A03-T02 -->

### Requirement: A4 FaultInject 仍仅在 testbuild 生效 (alias A4, D5-S23-A04-FaultInject P2 锁定)

#### Scenario: 生产 binary 启动 DEVRIX_FAULT_INJECT 不影响行为
- GIVEN devrix 二进制未带 -tags testbuild 编译
- AND 环境变量 DEVRIX_FAULT_INJECT="svc=error:test"
- WHEN 任何代码调用 injector.Hook("svc")
- THEN Hook 返回 nil（no-op stub）
- AND 不影响业务逻辑
<!-- T: D5-S23-A04-T01 (本 change 仅验证 build-tag 隔离生效, 不引入 IM 注入) -->

---

## MODIFIED

(None — DM-016 spec scenario 保持原状, 本 change 仅追加 ADDED 子节)

## REMOVED

(None)

---

## 关联 T 层映射

| Scenario | T 编号 | Activity |
|----------|--------|----------|
| doctor IM 触发 | D5-S23-A03-T01 | A1 |
| doctor 缺失 LSP 检测 | D5-S23-A03-T02 | A1 |
| context analyze 5 类 | D2-S6-A03-T01 | A5 |
| LSP enabled 调用 | D2-S4-A01-T02 | G1 |
| LSP disabled 默认 | D2-S4-A01-T01 | G1 |
| verify_plan 成功 | D6-S11-A02-T01 | G4 |
| verify_plan 缺文件 | D6-S11-A02-T02 | G4 |
| free_fork 批量 | D4-S11-A02-T01 | G5 |
| free_fork 回滚 | D4-S11-A02-T02 | G5 |
| tracker 5s 内 | D5-S23-A02-T01 | G6 |
| tracker 无 edit | D5-S23-A02-T02 | G6 |
| ErrorClassify 401 | D3-S3-A02-T01 | A6 |
| ShortStack sandbox | D2-S6-A02-T01 | A7 |
| AST heredoc | TOOL-SEC-2-A02-T01 | G2 |
| AST 合法命令 | TOOL-SEC-2-A02-T02 | G2 |
| DebugFilter whitelist | D5-S24-A02-T01 | A2 |
| DebugFilter non-debug | D5-S24-A02-T02 | A2 |
| Transcript 写 | D1-S2-A02-T01 | A3 |
| Notify consume | D4-S12-A03-T01 | G3 |
| Notify 空 bus | D4-S12-A03-T02 | G3 |
| FaultInject build-tag | D5-S23-A04-T01 | A4 |

---

## 验收对照

| AC | 覆盖 Scenario |
|----|--------------|
| AC1 | /doctor IM 触发 |
| AC2 | /context analyze IM 触发 |
| AC3 | LSP enabled 调用 |
| AC4 | verify_plan_execution 成功 / 缺文件 |
| AC5 | free_fork 批量 / 回滚 |
| AC6 | tracker 5s 内 / 无 edit |
| AC7 | ErrorClassify 401 |
| AC8 | ShortStack sandbox / agent |
| AC9 | Transcript 写 / ListSessions |
| AC10 | AST heredoc / 合法命令 |
| AC11 | Notify consume / 空 bus |
| AC12 | DebugFilter whitelist / non-debug |