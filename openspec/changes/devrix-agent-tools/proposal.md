# Proposal: Agent Tool 系统

**Change ID:** devrix-agent-tools
**Demand ID:** DM-20260608-012
**Status:** S2_Design

## 1. Background

devrix 目前只有一个同质的 `agent.Impl` 类型。所有子 Agent 共享相同的 prompt 和能力配置，无法为不同的任务类型选择不同的外部工具。用户希望 devrix 能像 cc-connect 一样注册多个外部 Agent 工具（Claude Code、Gemini CLI、Cursor），由 LLM 根据意图分发给最合适的工具执行。

## 2. Problem Statement

- 单一种类 Agent：无法按场景选择不同工具
- 无意图路由：LLM 不知道可以调用外部 Agent
- 不可扩展：新增工具需要改代码
- 无统一抽象：外部 Agent 调用方式、输出格式、超时控制没有标准化

## 3. Proposed Solution

### 架构设计

在 D4（Multi-Agent）层新增 **Agent Tool Registry**，每个外部 CLI Agent 包装为一个 `AgentTool` 接口实现。在 D2（Context Engine）层通过 `call_agent` 内置工具暴露给 LLM，LLM 通过 tool calling 决定何时调用哪个 Agent。

```
User Message → Context Engine → LLM (tool calling)
                                    |
                              call_agent tool
                                    |
                          Agent Tool Registry (D4)
                                    |
                          CLI Adapter (subprocess)
                                    |
                    Claude Code / Gemini / Cursor
```

### 核心组件

| 组件 | 所在层 | 职责 |
|------|--------|------|
| `AgentTool` 接口 | D4 - `multiagent/tool` | 定义 `Info()` + `Execute()` 契约 |
| `Registry` | D4 - `multiagent/tool` | 线程安全注册表，支持按能力查询 |
| `CLIAgentTool` | D4 - `multiagent/tool` | CLI 子进程适配器，解析 stream-json |
| `agentToolPlugin` | bootstrap | 桥接 D4 Registry → D2 PluginRunner |
| `call_agent` 工具 | D2 - contextengine | 向 LLM 暴露的 tool schema |

### 方案对比

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: Context Engine Tool（选择）** | 复用现有 tool calling 机制，LLM 自动路由，无需额外路由逻辑 | D2 需要通过 bootstrap 间接感知 D4 |
| B: D4 内部路由层 | 路由逻辑集中管理 | 重复 wheel，增加了 LLM 做路由决策的延迟 |
| C: 修改 LLM Gateway 做路由 | D3 统一管理分发 | 违反分层职责，D3 不应知道 Agent 工具细节 |

**选择:** 方案 A
**理由:** 复用 Context Engine 现有的 `PluginRunner` + `ToolRegistry` 机制最轻量。LLM 本身已有 tool calling 能力，`call_agent` 作为一个普通工具注册即可，由 LLM 决定何时调用。bootstrap 作为组合根负责桥接 D2 和 D4，不破坏分层。

### 关键设计决策

#### Decision: PluginRunner 放在 bootstrap 而非 contextengine

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 放在 bootstrap（选择） | 不破坏 D2→D4 的依赖方向 | 代码在组合根 |
| B: 放在 contextengine | 工具实现靠近注册处 | 引入 D2→D4 的反向依赖 |

**选择:** A
**理由:** contextengine (D2) 不应依赖 multiagent (D4)。bootstrap 是组合根，天然负责跨层装配。

#### Decision: CLI 子进程输出格式

- 主输出：stream-json（newline-delimited JSON events）
- 降级：非 JSON 行作为 text 事件
- stderr：作为诊断日志，合并到 tool_use 事件

## 4. Success Metrics

1. Agent 工具可通过 YAML 配置，无需改代码
2. LLM 成功调用 `call_agent` 工具并返回外部 Agent 结果
3. Registry 并发安全（`-race` 测试通过）
4. CLI 子进程在超时后正确终止

## 5. Implementation Plan

| 步骤 | 内容 | 文件 |
|------|------|------|
| 1 | 定义 AgentTool 接口 + Request/Event 类型 | `contracts.go` |
| 2 | 实现线程安全 Registry | `registry.go` |
| 3 | 实现 CLI 子进程适配器 | `cli_adapter.go` |
| 4 | 定义 AgentTool 配置类型 + Builder | `multiagent.go` (config) |
| 5 | 实现 `LoadAgentToolsConfig()` | `loader.go` (config) |
| 6 | 创建 bootstrap bridge plugin | `bootstrap/agent_tool.go` |
| 7 | 修改 bootstrap 接受 agentToolReg 参数 | `context_engine.go`, `context_engine_builder.go` |
| 8 | 修改 main.go 加载配置并构建 Registry | `cmd/devrix/main.go` |
| 9 | 更新 devrix.yaml 示例配置 | `devrix.yaml` |

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| CLI 子进程挂起 | context timeout + 默认 5min |
| stream-json 兼容性 | 非 JSON 降级为 text |
| 并发注册冲突 | sync.RWMutex 保护 |
| 配置了但不可执行 | 启动时 warn 而非 panic |

## 7. Out of Scope

- 意图自动路由（auto_route）：暂不实现，依赖 LLM 的 tool calling 做路由
- 外部 Agent 的结果缓存
- Agent 工具的负载均衡或健康检查
- 修改现有 Multi-Agent 的同质 Agent 协作逻辑
