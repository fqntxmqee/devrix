# Tasks: Agent Tool 系统

## T1: 定义 AgentTool 接口和核心类型

**文件:** `internal/layers/multiagent/tool/contracts.go`
**内容:**
- `Info` 结构体（Name, DisplayName, Description, Capabilities）
- `Request` 结构体（Task, WorkDir）
- `Event` 结构体（Type, Content）
- `AgentTool` 接口（Info(), Execute()）

## T2: 实现线程安全 Registry

**文件:** `internal/layers/multiagent/tool/registry.go`
**内容:**
- `Registry` 结构体（sync.RWMutex + map）
- `Register()`, `Get()`, `List()`, `FindByCapability()` 方法

## T3: 实现 CLI 子进程适配器（含 Session 管理）

**文件:** `internal/layers/multiagent/tool/cli_adapter.go`
**内容:**
- `CLIConfig` + `CLIAgentTool` 结构体
- `CLISession` 结构体：封装长驻子进程（cmd, stdin pipe, stdout scanner, timestamps, sync.Mutex）
- `EnsureSession(sessionID string)` 方法：按 `(sessionID, toolName)` 查找或创建子进程
  - 首次：`exec.Command` 启动，建立 pipe，启动 stdout scanner goroutine
  - 复用：直接返回已有 session，更新 `lastUsedAt`
- `Execute()` 方法：通过 stdin 发送 `{"type":"user","message":...}` JSON，收集事件到 complete
- `CloseSession(sessionID string)` 方法：三阶段关闭（close stdin → SIGTERM → SIGKILL）
- `CleanupBySessionID(sessionID string)` 方法：清理某 D1 Session 下所有子进程
- 空闲超时 goroutine：定时扫描，回收超时 session
- `parseStreamJSON()` 函数：scanner 逐行解析，JSON 降级为 text
- 参数模板替换（{prompt}/{task}）
- context timeout 控制

## T4: 定义配置类型和加载函数

**文件:**
- `internal/shared/config/multiagent.go` — 新增 AgentToolFileConfig / AgentToolsFileConfig / AgentToolConfig / AgentToolsConfig + Builder
- `internal/shared/config/loader.go` — ConfigFile 新增 AgentTools 字段 + LoadAgentToolsConfig()

## T5: 创建 bootstrap bridge plugin

**文件:** `internal/bootstrap/agent_tool.go`
**内容:**
- `agentToolPlugin` 结构体（持有 *tool.Registry）
- `Name()`, `Schema()`, `RiskLevel()`, `Execute()` 方法
- Schema 动态枚举已注册工具
- Execute 委托 Registry 查找并调用 AgentTool
- `CleanupSession(sessionID string)` 方法：供 Gateway 在 D1 Session 销毁时回调
- Execute 方法增加 session_id 参数传递支持

## T6: 修改 bootstrap 注入 agentToolReg

**文件:**
- `internal/bootstrap/context_engine.go` — 新增参数，启用时注册 call_agent
- `internal/bootstrap/context_engine_builder.go` — 同上

## T7: 修改 main.go 加载配置并构建 Registry

**文件:**
- `cmd/devrix/main.go` — 加载 agent tools 配置，构建 Registry，注入引擎，注册 Session 清理回调
- `cmd/llm-smoke/main.go` — 传 nil 适配新签名
- `cmd/devrix-feishu/main.go` — 同上
- `cmd/devrix-dingtalk/main.go` — 同上
- Gateway 创建 Session 时注册清理：`gateway.OnSessionDestroy(sessionID, func() { agentToolPlugin.CleanupSession(sessionID) })`

## T8: 更新 devrix.yaml

**文件:** `devrix.yaml`
**内容:** 新增 agent_tools 配置段（默认 enabled: false，含三个示例工具）

## T9: 测试

**内容:**
- Registry 单元测试：注册/查找/能力查询/并发安全
- CLI 适配器测试：正常执行/超时/stream-json 解析/模板替换
- **Session 管理测试：**
  - 首次 `Execute(sessionID)` 创建子进程，再次调用复用（L5-4-6-04）
  - 不同 D1 SessionID 使用独立子进程（L5-4-6-07）
  - 空闲超时后子进程自动回收（L5-4-6-05）
  - `CleanupBySessionID()` 清理指定 Session 的子进程（L5-4-6-06）
  - session map 并发安全（-race 检测）
- 配置加载测试

**L5 验收覆盖:**
- P0: L5-4-6-01, L5-4-6-02, L5-4-6-04, L5-4-6-07
- P1: L5-4-6-03, L5-4-6-05, L5-4-6-06

## T10: 验证

**内容:**
- `go build ./...` 编译通过
- `go test ./...` 全部通过
- `go test -race ./internal/layers/multiagent/tool/...` 无 data race

---

## 依赖顺序

```
T1 → T2 → T3
  ↘              ↘
   T4 → T5 → T6 → T7 → T8 → T9 → T10
```
