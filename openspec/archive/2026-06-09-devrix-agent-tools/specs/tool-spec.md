# Agent Tool System — Specification

<!-- L5: L5-4-6-01, L5-4-6-02, L5-4-6-03 -->

## ADDED

### Requirement: Registry — 注册与查询

<!-- L5: L5-4-6-01 -->

#### Scenario: 注册一个 Agent 工具
- GIVEN 一个空的 Registry
- WHEN 注册一个名为 "claude-code" 的有效 AgentTool
- THEN 注册成功，无 error
- AND List() 返回包含该工具的列表

#### Scenario: 重复名称注册被拒绝
- GIVEN 已注册 "claude-code" 的 Registry
- WHEN 再次注册名为 "claude-code" 的另一个工具
- THEN 返回 error "agent tool already registered: claude-code"

#### Scenario: 按名称查找存在的工具
- GIVEN 已注册 "claude-code" 和 "gemini" 的 Registry
- WHEN Get("claude-code") 调用
- THEN 返回 "claude-code" 工具
- AND error 为 nil

#### Scenario: 按名称查找不存在的工具
- GIVEN 空的 Registry
- WHEN Get("unknown") 调用
- THEN 返回 error "agent tool not found: unknown"

#### Scenario: 按能力查询
- GIVEN 已注册 claude-code (capabilities: ["coding", "review"]) 和 gemini (capabilities: ["research"]) 的 Registry
- WHEN FindByCapability("coding") 调用
- THEN 返回包含 "claude-code" 的列表
- AND 不包含 "gemini"

#### Scenario: Registry 并发安全
- GIVEN 一个 Registry
- WHEN 并发的 10 个 goroutine 同时注册和查询
- THEN 无 data race
- AND 所有操作正确完成

### Requirement: CLI 适配器 — 子进程执行

<!-- L5: L5-4-6-02 -->

#### Scenario: 正常执行并解析 stream-json
- GIVEN 配置为执行 `echo` 命令，输出 `{"type":"text","content":"hello"}`
- WHEN Execute() 调用
- THEN Event channel 收到一个 text 事件，Content="hello"
- AND 收到一个 complete 事件

#### Scenario: 非 JSON 行降级为 text 事件
- GIVEN 配置为执行 `echo` 命令，输出 "plain text"
- WHEN Execute() 调用
- THEN Event channel 收到一个 text 事件，Content="plain text"

#### Scenario: 参数模板替换
- GIVEN Args 包含 `{"command", "{prompt}"}`
- WHEN Execute(Request{Task: "hello"}) 调用
- THEN 子进程实际参数为 `["command", "hello"]`

#### Scenario: 无占位符时追加到末尾
- GIVEN Args 为 `["--print"]`
- WHEN Execute(Request{Task: "hello"}) 调用
- THEN 子进程实际参数为 `["--print", "hello"]`

#### Scenario: 子进程超时终止
- GIVEN 配置 timeout=100ms，命令为 `sleep 10`
- WHEN Execute() 调用
- THEN 收到一个 error 事件
- AND 子进程在 200ms 内终止
- AND 不返回 complete 事件

#### Scenario: 首次调用创建长驻 Session
- GIVEN 一个未初始化的 CLIAgentTool
- WHEN 首次 Execute(sessionID="sess_1", ...) 调用
- THEN 启动一个子进程
- AND 存入 sessions 映射
- AND 返回正常结果
- AND 子进程保持运行

#### Scenario: 复用已有 Session
- GIVEN 已为 sess_1 启动子进程的 CLIAgentTool
- WHEN 再次 Execute(sessionID="sess_1", ...) 调用
- THEN 不创建新子进程
- AND 复用已有进程的 stdin 发送消息
- AND lastUsedAt 被更新

#### Scenario: 不同 D1 Session 使用不同子进程
- GIVEN CLIAgentTool 已有 sess_1 的子进程
- WHEN Execute(sessionID="sess_2", ...) 调用
- THEN 创建第二个子进程
- AND 两个子进程独立运行，互不干扰

#### Scenario: 空闲超时自动回收
- GIVEN idle_timeout=100ms
- WHEN 超过 100ms 无 Execute() 调用
- THEN 子进程被三阶段关闭
- AND 从 sessions 映射中移除

#### Scenario: D1 Session 销毁清理关联子进程
- GIVEN CLIAgentTool 有 sess_1 和 sess_2 的子进程
- WHEN CleanupBySessionID("sess_1") 调用
- THEN sess_1 的子进程被关闭
- AND sess_2 的子进程不受影响

### Requirement: Config — YAML 配置加载

#### Scenario: 默认配置为 disabled
- GIVEN 配置文件不包含 `agent_tools` 段
- WHEN LoadAgentToolsConfig("") 调用
- THEN 返回默认配置，Enabled=false
- AND Tools 为 nil

#### Scenario: 完整配置解析正确
- GIVEN 包含完整 agent_tools 段的 YAML
- WHEN LoadAgentToolsConfig 加载
- THEN 返回的配置包含所有注册的工具
- AND 每个工具的字段正确映射

### Requirement: call_agent — LLM 工具调用

<!-- L5: L5-4-6-03 -->

#### Scenario: LLM 调用存在的 Agent
- GIVEN Registry 中有 "claude-code"
- WHEN PluginRunner.Execute() 传入 `{"agent_name":"claude-code","task":"hello"}`
- THEN ToolResult 包含子进程的输出

#### Scenario: LLM 调用不存在的 Agent
- GIVEN Registry 为空
- WHEN PluginRunner.Execute() 传入 `{"agent_name":"unknown","task":"hello"}`
- THEN ToolResult.Error 包含 "unknown agent tool"

#### Scenario: 未启用时 LLM 看不到 call_agent
- GIVEN Registry 为空或 AgentTools.Enabled=false
- WHEN ToolRegistry.ListTools() 调用
- THEN 返回的 schema 中不包含 "call_agent"

## MODIFIED
(None)

## REMOVED
(None)
