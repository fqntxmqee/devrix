# Design: Agent Tool 系统

## 1. Root Cause Analysis

当前 devrix 无法注册外部 Agent 工具的根本原因：

1. **Agent 实现单一**：`agent.Impl` 是所有子 Agent 的唯一实现，没有插件化接口
2. **工具系统封闭**：Context Engine 的 `ToolRegistry` 只注册了 bash/read_file/write_file 三个内置工具，没有扩展点用于注册外部 Agent 调用工具
3. **配置无感知**：`devrix.yaml` 没有外部 Agent 的配置段
4. **层间隔离**：Context Engine (D2) 不能直接依赖 Multi-Agent (D4)，缺少桥接层

## 2. Solution Design

### 2.1 总体架构

在 D4 层新增 `multiagent/tool/` 子包，定义 Agent Tool 的统一接口和注册表。在 bootstrap（组合根）编写适配器，将 D4 的 Registry 桥接到 D2 的 `PluginRunner` 接口。LLM 通过 tool calling 调用 `call_agent` 工具，由 Registry 路由到具体的 CLI 适配器执行。

```
┌─────────────────────────────────────────────────┐
│  Context Engine (D2)                            │
│  ┌──────────────────────────────┐               │
│  │  ToolRegistry                │               │
│  │  ├─ bash                     │               │
│  │  ├─ read_file                │               │
│  │  ├─ write_file               │               │
│  │  └─ call_agent (PluginRunner)│◄── bridge     │
│  └──────────────────────────────┘               │
├─────────────────────────────────────────────────┤
│  Bootstrap (组合根)                              │
│  ┌──────────────────────────────────────────┐   │
│  │  agentToolPlugin: PluginRunner            │   │
│  │  → 持有 *tool.Registry 引用              │   │
│  │  → Schema() 动态枚举已注册工具             │   │
│  │  → Execute() 委托给 Registry.Get().Execute│   │
│  └──────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  Multi-Agent (D4)                               │
│  ┌──────────────────────────────┐               │
│  │  tool.Registry               │               │
│  │  ├─ "claude-code": CLIAdapter│               │
│  │  ├─ "gemini": CLIAdapter     │               │
│  │  └─ "cursor": CLIAdapter     │               │
│  └──────────────────────────────┘               │
└─────────────────────────────────────────────────┘
       │
       ▼ 子进程 (exec.CommandContext)
  Claude Code / Gemini CLI / Cursor
       │
       ▼ stream-json (newline-delimited JSON)
  {"type":"text","content":"..."}
  {"type":"tool_use","content":"..."}
  {"type":"complete","content":""}
```

### 2.2 组件设计

#### AgentTool 接口（D4 - `multiagent/tool/contracts.go`）

```go
type Info struct {
    Name         string
    DisplayName  string
    Description  string
    Capabilities []string
}

type Request struct {
    Task    string
    WorkDir string
}

type Event struct {
    Type    string // "text", "tool_use", "error", "complete"
    Content string
}

type AgentTool interface {
    Info() Info
    Execute(ctx context.Context, req Request) (<-chan Event, error)
}
```

#### Registry（D4 - `multiagent/tool/registry.go`）

线程安全注册表，基于 `sync.RWMutex`：
- `Register(tool)` — 注册，重名返回 error
- `Get(name)` — 按名称查找
- `List()` — 返回所有工具 Info，按名排序
- `FindByCapability(cap)` — 按能力查找

#### CLI 适配器（D4 - `multiagent/tool/cli_adapter.go`）

- **Session 管理**：按 `(sessionID, toolName)` 维护长驻子进程映射
  - 首次 `Execute()` → 启动子进程，建立 stdin/stdout/stderr pipe
  - 后续 `Execute()` → 复用已有进程，通过 stdin 发送任务
  - 关闭：关闭 stdin → SIGTERM → SIGKILL 三阶段终止
- **输入协议**：通过 stdin pipe 写入 `{"type":"user","message":{"role":"user","content":"..."}}`
- **输出解析**：stdout scanner 逐行解析 stream-json 事件（text / tool_use / error / complete）
- **空闲超时**：可配置 `idle_timeout`（默认 5 分钟），超时自动回收子进程
- **D1 Session 联动**：监听 Session 销毁事件，清理对应子进程
- **参数模板替换**：`{prompt}` 或 `{task}` 占位符

#### Bridge Plugin（bootstrap - `agent_tool.go`）

- 实现 `contextengine.PluginRunner` 接口
- `Schema()` 动态生成 JSON Schema，枚举已注册的工具名称
- `Execute()` 解析 LLM 传入的 `{agent_name, task, work_dir}`，委托 Registry 执行
- 收集所有 Event 到 ToolResult.Output

### 2.3 配置格式

```yaml
agent_tools:
  enabled: false
  tools:
    - name: claude-code
      display_name: "Claude Code"
      description: "通用编码、代码审查、重构、调试"
      capabilities: ["coding", "code-review", "debug", "refactor"]
      command: "claude"
      args: ["--print", "--output-format", "stream-json"]
      work_dir: "."
      timeout: "5m"
      idle_timeout: "5m"
```

### 2.4 Session 关系

Agent Tool 系统引入了一种新的 Session 类型——**Agent Tool Session**，它位于现有 D1 Session 和 D4 Agent 之间：

```
┌──────────────────────────────────────────────────────┐
│  D1 Session (聊天对话)                                │
│  SessionID: sess_20260609_abc123                     │
│  ChatID: xxx, UserID: yyy                           │
│  ┌────────────────────────────────────────────────┐  │
│  │  Agent Tool Session Map                        │  │
│  │                                                │  │
│  │  (sess_xxx, "claude-code") → CLISession{       │  │
│  │    cmd: *exec.Cmd,                             │  │
│  │    stdin: io.WriteCloser,                      │  │
│  │    stdout: *bufio.Scanner,                     │  │
│  │    createdAt: time.Time,                       │  │
│  │    lastUsedAt: time.Time,                      │  │
│  │  }                                             │  │
│  │                                                │  │
│  │  (sess_xxx, "gemini") → CLISession{ ... }      │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

#### 生命周期规则

| 阶段 | 事件 | 行为 |
|------|------|------|
| 创建 | 首次 `call_agent({agent_name})` | `exec.Command` 启动子进程，建立 pipe |
| 复用 | 同 Session 再次调用同 Agent | stdin 写入新任务，等待 complete 事件 |
| 空闲超时 | 超过 `idle_timeout` 无调用 | 三阶段关闭子进程，从映射中移除 |
| Session 销毁 | D1 Session 过期/删除 | 清理该 Session 下所有 Agent Tool Session |
| 显式关闭 | `/tool-close` 命令 | 关闭指定 Agent Tool Session |

#### 与 cc-connect 的对比

| 维度 | cc-connect | devrix Agent Tool |
|------|-----------|-------------------|
| Session 粒度 | 全局唯一，一个 Agent 一个进程 | 按 `(D1 Session, Agent Tool)` 组合 |
| 通信方式 | stdin/stdout stream-json | 相同协议 |
| 生命周期 | 跟随 cc-connect 进程 | 跟随 D1 Session，支持空闲回收 |
| 复用策略 | 始终复用 | 同 Session 内复用，跨 Session 独立 |
| 清理机制 | 进程退出时自然清理 | 三阶段关闭 + 空闲超时 + Session 联动 |

#### 设计决策

**Decision: Agent Tool Session 为什么按 `(D1 Session, Tool)` 组合而不是全局单例**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 按 `(Session, Tool)` 组合（选择） | 隔离性好，不同会话不互相干扰；可独立控制空闲超时 | 每个会话多一个进程，资源占用略高 |
| B: 全局单例 | 资源占用最少 | 并发冲突（两个用户同时调 claude-code），上下文混淆 |

**选择:** A
**理由:** 隔离性优先。Agent Tool 的 session 携带对话上下文，不同用户/不同会话的上下文必须隔离。全局单例会导致上下文污染和安全问题。

## 3. Key Interfaces / Types

### 新增类型

| 包 | 类型 | 说明 |
|------|------|------|
| `multiagent/tool` | `Info` | Agent 工具元数据 |
| `multiagent/tool` | `Request` | 执行请求 |
| `multiagent/tool` | `Event` | 流式事件 |
| `multiagent/tool` | `AgentTool` | 核心接口 |
| `multiagent/tool` | `CLIConfig` | CLI 适配器配置 |
| `multiagent/tool` | `CLIAgentTool` | CLI 适配器实现（含 session 管理） |
| `multiagent/tool` | `CLISession` | 长驻子进程封装（cmd, stdin, stdout, timestamps） |
| `multiagent/tool` | `Registry` | 线程安全注册表 |
| `config` | `AgentToolFileConfig` | YAML 配置结构 |
| `config` | `AgentToolsFileConfig` | YAML agent_tools 段 |
| `config` | `AgentToolConfig` | 运行时配置 |
| `config` | `AgentToolsConfig` | 运行时配置容器 |
| bootstrap | `agentToolPlugin` | PluginRunner 桥接实现 |

### 修改类型

| 包 | 类型 | 变更 |
|------|------|------|
| `config` | `ConfigFile` | 新增 `AgentTools` 字段 |
| bootstrap | `ContextEngineBuilder` | 新增 `agentToolReg` 字段 |

## 4. Data Flow

### 正常路径

#### 首次调用（创建 Session）

```
1. User: "帮我重构这个模块"
2. Context Engine: 发送消息 + tool schemas 给 LLM
3. LLM: 决定调用 call_agent({agent_name:"claude-code", task:"重构模块..."})
4. Context Engine: 通过 ToolRegistry 找到 call_agent 工具
5. agentToolPlugin.Execute():
   a. 解析 JSON input → {agent_name, task, session_id}
   b. tool.Registry.Get("claude-code") → CLIAgentTool
   c. CLIAgentTool.Execute(ctx, sessionID, Request{Task: "重构模块..."})
   d. EnsureSession(sessionID, "claude-code"):
      - 映射中不存在 → exec.Command 启动子进程
      - 建立 stdin/stdout/stderr pipe
      - 启动 stdout scanner goroutine
      - 存入 sessions 映射
   e. 通过 stdin 写入: {"type":"user","message":{"role":"user","content":"重构模块..."}}
   f. stdout scanner 逐行读取 stream-json，收集到 complete 事件
   g. 返回 ToolResult{Output: "..."}（子进程保持运行）
6. Context Engine: 将 ToolResult 注入 LLM 上下文
7. LLM: 基于结果继续对话
```

#### 复用 Session

```
1. User: "继续优化，换个方式实现"
2-4. 同上
5. agentToolPlugin.Execute():
   a. 解析 JSON input → {agent_name:"claude-code", task:"继续优化..."}
   b. tool.Registry.Get("claude-code") → CLIAgentTool
   c. CLIAgentTool.Execute(ctx, sessionID, Request{Task: "继续优化..."})
   d. EnsureSession(sessionID, "claude-code"):
      - 映射中存在 → 直接返回现有 session（上下文保持）
      - 更新 lastUsedAt
   e. stdin 写入新任务
   f. 读取 stream-json 到 complete
   g. 返回 ToolResult（子进程仍保持运行）
6-7. 同上
```

### 错误路径

```
LLM 指定不存在的 Agent:
  → Registry.Get("unknown") 返回 error
  → ToolResult.Error = "unknown agent tool"

CLI 子进程超时:
  → context.WithTimeout 触发 cancel
  → 三阶段关闭子进程（close stdin → SIGTERM → SIGKILL）
  → 从 sessions 映射中移除该 session
  → 返回已收集的部分结果 + error 事件
  → 下次调用时重新创建子进程

stream-json 解析失败:
  → 非 JSON 行 → Event{Type:"text", Content: line}
  → 不阻断整体执行

Session 销毁:
  → D1 Session 过期/Gateway 通知 Agent Tool 系统
  → 遍历该 Session 下所有 Agent Tool Session
  → 三阶段关闭每个子进程
  → 从 sessions 映射中清除
```

## 5. File Manifest

### 新增文件

| 文件 | 说明 |
|------|------|
| `internal/layers/multiagent/tool/contracts.go` | AgentTool 接口 + Info/Request/Event 类型 |
| `internal/layers/multiagent/tool/registry.go` | 线程安全注册表 |
| `internal/layers/multiagent/tool/cli_adapter.go` | CLI 子进程适配器 + session 管理 + stream-json 解析 |
| `internal/bootstrap/agent_tool.go` | call_agent PluginRunner 桥接实现 |
| `openspec/changes/devrix-agent-tools/demand.md` | S1 需求文档 |
| `openspec/changes/devrix-agent-tools/.openspec.yaml` | S2 元数据 |
| `openspec/changes/devrix-agent-tools/proposal.md` | S2 提案 |
| `openspec/changes/devrix-agent-tools/design.md` | S3 设计文档 |
| `openspec/changes/devrix-agent-tools/specs/tool-spec.md` | Gherkin 规格 |
| `openspec/changes/devrix-agent-tools/tasks.md` | 任务拆解 |

### 修改文件

| 文件 | 变更 |
|------|------|
| `internal/shared/config/multiagent.go` | 新增 AgentToolFileConfig / AgentToolsFileConfig / AgentToolConfig / AgentToolsConfig + Builder |
| `internal/shared/config/loader.go` | ConfigFile 新增 AgentTools 字段 + LoadAgentToolsConfig() |
| `internal/bootstrap/context_engine.go` | NewContextEngine 接受 agentToolReg 参数，注册 call_agent |
| `internal/bootstrap/context_engine_builder.go` | 同上 |
| `cmd/devrix/main.go` | 加载 agent tools 配置，构建 Registry，注入引擎 |
| `cmd/llm-smoke/main.go` | 传递 nil 参数适配新签名 |
| `cmd/devrix-feishu/main.go` | 同上 |
| `cmd/devrix-dingtalk/main.go` | 同上 |
| `devrix.yaml` | 新增 agent_tools 配置段示例 |
| `openspec/l5-registry.md` | 注册 D4-S6 L5 测试点 |

## 6. Regression Risk Assessment

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| 修改 `NewContextEngine` 签名破坏其他调用方 | 低 | 编译失败 | 一次性更新所有 4 个调用处 |
| 新增 config 字段导致旧配置解析异常 | 低 | 字段忽略 | YAML 零值安全，无 `required` 标记 |
| Registry 并发竞争 | 低 | 数据损坏 | sync.RWMutex 保护 |
| CLI 子进程资源泄露 | 中 | 进程残留 | context.WithTimeout + defer cancel + idle timeout + Session 联动清理 |
| Agent Tool Session 泄露 | 中 | D1 Session 已销毁但子进程残留 | Gateway 注册 Session 销毁回调，遍历清理所有关联子进程 |

### 6.1 性能影响评估

#### 资源模型

Agent Tool Session 是长驻子进程，资源消耗取决于 **活跃 D1 Session 数 × 每个 Session 使用的工具数**：

| 资源 | 单子进程估算 | 正常场景 (5 Sessions × 2 Tools) | 极端场景 (20 Sessions × 5 Tools) |
|------|------------|-------------------------------|--------------------------------|
| 内存 (RSS) | Claude Code ~200MB, Gemini ~100MB | ~2-3 GB | ~10-15 GB |
| 进程数 | 1 | 10 | 100 |
| 启动延迟 | 首次 2-8s | 仅在首次调用时发生 | 同上 |

#### 性能风险与控制措施

| 风险 | 触发条件 | 缓解措施 |
|------|---------|----------|
| 内存超限 | 大量并发 Session 使用同一种 Agent Tool | 配置 `max_sessions_per_tool` 上限（默认 10），超限时复用最旧 Session |
| CPU 竞争 | 多个子进程同时响应 | 子进程各自独立，受 OS 调度，不阻塞 devrix 主进程 |
| 启动风暴 | 多个 Session 同时首次调用 Agent | 启动延迟仅影响首次 tool calling 响应时间，后续复用无延迟 |
| context 膨胀 | 长对话 Session 的 Agent 进程积累过多上下文 | 依赖 Agent 自身 context 管理（Claude Code 自动 summarize） |

#### 决策

**Decision: 是否需要限制全局最大子进程数**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 不设硬上限（选择） | 使用场景灵活，不限制高级用户 | 极端场景可能资源耗尽 |
| B: 设 `max_processes` 硬上限 | 资源可控 | 超出上限时行为复杂（拒绝/排队/淘汰） |
| C: 设软上限 + 淘汰策略（LRU） | 兼顾资源控制与可用性 | 实现复杂，被淘汰的 Agent 丢失上下文 |

**选择:** A，在配置层提供 `max_sessions_per_tool` 软上限即可。
**理由:** Agent Tool 是可选的增强功能，用户通过配置自行控制资源边界。devrix 本身不承担资源管控的责任（OS 级别的 `ulimit` / cgroups 更适合）。

## 7. Rollback Plan

1. 删除 `internal/layers/multiagent/tool/` 目录
2. 删除 `internal/bootstrap/agent_tool.go`
3. 恢复 `multiagent.go` 至仅含 MultiAgent 类型
4. 恢复 `loader.go` 至无 AgentTools 字段
5. 恢复 `context_engine.go` / `context_engine_builder.go` 原始签名
6. 恢复 `cmd/` 下所有 main.go 的原始调用
7. 恢复 `devrix.yaml` 移除 agent_tools 段
8. `git revert` 或手动回退

### Decision: 为什么不在启动时校验 Agent 工具的可执行性

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 启动时不校验（选择） | 启动快，不因配置的工具不存在而崩溃 | 运行时才暴露问题 |
| B: 启动时检查 `exec.LookPath` | 尽早暴露问题 | 配置了但暂未安装的工具会导致启动失败 |

**选择:** A
**理由:** Agent 工具是可选的增强功能，不应因外部工具未安装而阻塞 devrix 启动。缺失的工具在 LLM 尝试调用时返回 error 事件。
