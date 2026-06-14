# Devrix 配置参考

> 当前版本: v2.0.0 · 配置加载优先级: YAML 文件 (`devrix.yaml`) → 环境变量 → 代码默认值

---

## 目录

1. [应用 (app)](#1-应用-app)
2. [会话 (session)](#2-会话-session)
3. [认证 (auth)](#3-认证-auth)
4. [权限 (permission)](#4-权限-permission)
5. [连接 (connection)](#5-连接-connection)
6. [限流 (rate_limit)](#6-限流-ratelimit)
7. [CLI (cli)](#7-cli-cli)
8. [命令 (commands)](#8-命令-commands)
9. [飞书 (feishu)](#9-飞书-feishu)
10. [实例 (instance)](#10-实例-instance)
11. [日志 (logging)](#11-日志-logging)
12. [Metrics (metrics)](#12-metrics-metrics)
13. [LLM 网关 (Layer 3 — llm_gateway)](#13-llm-网关-layer-3--llm-gateway)
14. [上下文引擎 (Layer 2 — context_engine)](#14-上下文引擎-layer-2--context-engine)
15. [工具执行安全 (tool)](#15-工具执行安全-tool)
16. [多智能体 (Layer 4 — multi_agent)](#16-多智能体-layer-4--multi-agent)
17. [Agent 工具 (agent_tools)](#17-agent-工具-agent-tools)
18. [编排验证 (Layer 6 — orchestration)](#18-编排验证-layer-6--orchestration)
19. [可观察性 (Layer 5 — observability)](#19-可观察性-layer-5--observability)
20. [用户配置 (user config)](#20-用户配置-user-config)
21. [环境变量索引](#21-环境变量索引)

---

## 1. 应用 (app)

系统元信息，无运行时行为影响。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `name` | string | `"devrix"` | 应用名称 |
| `version` | string | `"2.0.0"` | 版本号 |
| `mode` | string | `"cli"` | 运行模式: `cli` \| `server` \| `daemon` |

```yaml
app:
  name: "devrix"
  version: "2.0.0"
  mode: "cli"
```

**代码**: `internal/shared/config/loader.go` — `AppConfig`

---

## 2. 会话 (session)

会话生命周期管理。

| 字段 | 类型 | 默认值 | 环境变量覆盖 | 描述 |
|------|------|--------|-------------|------|
| `idle_timeout` | duration | `30m` | `DEVRIX_SESSION_TIMEOUT` | 会话空闲超时 |
| `storage_dir` | string | `~/.devrix/sessions` | `DEVRIX_SESSION_DIR` | 会话存储目录 |
| `max_sessions` | int | `1000` | — | 最大并发会话数 |

```yaml
session:
  idle_timeout: "30m"
  storage_dir: "~/.devrix/sessions"
  max_sessions: 1000
```

**代码**: `internal/shared/config/communication.go` — `SessionConfig`

---

## 3. 认证 (auth)

JWT 认证配置。

| 字段 | 类型 | 默认值 | 环境变量覆盖 | 描述 |
|------|------|--------|-------------|------|
| `secret` | string | `"change-me-in-production"` | `DEVRIX_AUTH_SECRET` | JWT 签名密钥 |
| `token_expiry` | duration | `24h` | `DEVRIX_AUTH_TOKEN_EXPIRY` | Token 过期时间 |
| `issuer` | string | `"devrix"` | `DEVRIX_AUTH_ISSUER` | JWT 签发者 |

```yaml
auth:
  secret: "change-me-in-production"
  token_expiry: "24h"
  issuer: "devrix"
```

**环境变量优先级**高于 YAML 文件。

**代码**: `internal/shared/config/auth.go` — `AuthConfigLoader` → `types.AuthConfig`

---

## 4. 权限 (permission)

工具执行权限控制。

| 字段 | 类型 | 默认值 | 环境变量覆盖 | 描述 |
|------|------|--------|-------------|------|
| `default_timeout` | duration | `60s` | `DEVRIX_PERMISSION_TIMEOUT` | 权限审批超时时间 |
| `max_retries` | int | `3` | — | 最大重试次数 |

```yaml
permission:
  default_timeout: "60s"
  max_retries: 3
```

**代码**: `internal/shared/config/communication.go` — `PermissionConfig`

---

## 5. 连接 (connection)

WebSocket/长连接心跳。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `heartbeat_interval` | duration | `10s` | 心跳发送间隔 |
| `heartbeat_timeout` | duration | `60s` | 心跳超时断开时间 |

```yaml
connection:
  heartbeat_interval: "10s"
  heartbeat_timeout: "60s"
```

**代码**: `internal/shared/config/loader.go` — `ConnectionConfig`（仅 YAML 声明，无独立 domain config）

---

## 6. 限流 (rate_limit)

请求速率限制。

| 字段 | 类型 | 默认值 | 环境变量覆盖 | 描述 |
|------|------|--------|-------------|------|
| `enabled` | bool | `true` | `DEVRIX_RATE_LIMIT_ENABLED` | 启用限流 |
| `requests_per_minute` | int | `100` | `DEVRIX_RATE_LIMIT_RPM` | 每分钟请求数 |
| `burst_size` | int | `10` | `DEVRIX_RATE_LIMIT_BURST` | 突发容量 |

```yaml
rate_limit:
  enabled: true
  requests_per_minute: 100
  burst_size: 10
```

**代码**: `internal/shared/config/ratelimit.go` — `RateLimitConfig`, `RateLimitConfigLoader`

---

## 7. CLI (cli)

命令行界面外观。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `welcome_message` | string | *Devrix 欢迎横幅* | 启动时显示的欢迎消息 |
| `prompt` | string | `"> "` | 输入提示符 |
| `ansi.user` | string | 蓝色 | 用户消息 ANSI 颜色码 |
| `ansi.assistant` | string | 绿色 | 助手消息 ANSI 颜色码 |
| `ansi.error` | string | 红色 | 错误消息 ANSI 颜色码 |
| `ansi.warning` | string | 黄色 | 警告消息 ANSI 颜色码 |
| `ansi.reset` | string | 重置 | ANSI 重置码 |

```yaml
cli:
  welcome_message: "╔════════════════════════════════╗\n║ Devrix v2.0 - 开发大脑 ║\n╚════════════════════════════════╝"
  prompt: "> "
  ansi:
    user: "\x1b[34m"
    assistant: "\x1b[32m"
    error: "\x1b[31m"
    warning: "\x1b[33m"
    reset: "\x1b[0m"
```

**代码**: `internal/shared/config/communication.go` — `CLIConfig`, `ANSIConfig`

---

## 8. 命令 (commands)

内建命令前缀和列表。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `prefix` | string | `"/"` | 命令前缀符号 |
| `list` | []string | `["new", "stop", "help"]` | 可用命令列表 |

```yaml
commands:
  prefix: "/"
  list:
    - "new"
    - "stop"
    - "help"
```

**代码**: `internal/shared/config/communication.go` — `CommandsConfig`

---

## 9. 飞书 (feishu)

飞书 IM 集成（系统配置，区别于用户配置中的飞书）。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `false` | 启用飞书集成 |
| `app_id` | string | `""` | 飞书应用 ID |
| `app_secret` | string | `""` | 飞书应用 Secret |
| `bot_name` | string | `"Devrix"` | 机器人名称 |
| `domain` | string | `""` | 自定义域名（留空用飞书默认） |
| `encrypt_key` | string | `""` | 回调加密密钥 |
| `callback_path` | string | `"/feishu/webhook"` | 回调路径 |
| `port` | string | `"8080"` | 回调服务端口 |
| `use_webhook` | bool | `false` | 使用 Webhook 模式 |

```yaml
feishu:
  enabled: false
  app_id: ""
  app_secret: ""
  bot_name: "Devrix"
  domain: ""
  encrypt_key: ""
  callback_path: "/feishu/webhook"
  port: "8080"
  use_webhook: false
```

**代码**: `internal/shared/config/loader.go` — `FeishuFileConfig`

---

## 10. 实例 (instance)

集群部署实例标识。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `id` | string | `""` | 实例 ID（留空自动生成） |
| `name` | string | `"devrix"` | 实例名称 |
| `address` | string | `""` | 实例地址 |
| `port` | int | `0` | 实例端口 |
| `cluster_enabled` | bool | `false` | 启用集群模式 |

```yaml
instance:
  id: ""
  name: "devrix"
  address: ""
  port: 0
  cluster_enabled: false
```

**代码**: `internal/shared/config/loader.go` — `InstanceConfig`

---

## 11. 日志 (logging)

系统日志输出配置。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `level` | string | `"debug"` | 日志级别: `debug` \| `info` \| `warn` \| `error` |
| `format` | string | `"json"` | 输出格式: `json` \| `text` |
| `output` | string | `"stdout"` | 输出目标: `stdout` \| `stderr` \| `file` |

```yaml
logging:
  level: "debug"
  format: "json"
  output: "stdout"
```

**代码**: `internal/shared/config/loader.go` — `LoggingConfig`

---

## 12. Metrics (metrics)

Prometheus HTTP 端点配置。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用 metrics 端点 |
| `port` | int | `9090` | HTTP 端口 |
| `path` | string | `"/metrics"` | metrics 路径 |

```yaml
metrics:
  enabled: true
  port: 9090
  path: "/metrics"
```

**代码**: `internal/shared/config/loader.go` — `MetricsConfig`

---

## 13. LLM 网关 (Layer 3 — llm_gateway)

LLM 提供商路由、熔断、重试配置。系统核心，所有 LLM 调用经此转发。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `default_provider` | string | `"minimax"` | 默认 LLM 提供商 |
| `default_model` | string | `"MiniMax-M2.7-highspeed"` | 默认模型名 |
| `model_routing` | map[string]string | *见下方* | 模型名 → 提供商路由表 |
| `circuit_breaker.failure_threshold` | int | `5` | 熔断触发失败次数 |
| `circuit_breaker.success_threshold` | int | `2` | 半开恢复成功次数 |
| `circuit_breaker.open_duration` | duration | `30s` | 熔断开启持续时间 |
| `circuit_breaker.half_open_max_probes` | int | `1` | 半开状态最大探测数 |
| `circuit_breaker.scope` | string | `"provider"` | 熔断作用域: `provider` |
| `providers.<name>.type` | string | — | 提供商类型标识 |
| `providers.<name>.base_url` | string | — | API 基础 URL |
| `providers.<name>.api_key_env` | string | — | API Key 环境变量名 |
| `providers.<name>.default_model` | string | — | 提供商默认模型 |
| `providers.<name>.fallback_model` | string | — | 降级模型 |
| `providers.<name>.timeout` | duration | `60s` | 请求超时 |
| `providers.<name>.max_tokens` | int | `8192` | 最大 Token 数 |
| `providers.<name>.temperature` | float | `0.7` | 采样温度 |
| `providers.<name>.retry.max_attempts` | int | `3` | 最大重试次数 |
| `providers.<name>.retry.initial_delay` | duration | `1s` | 初始重试延迟 |
| `providers.<name>.retry.max_delay` | duration | `10s` | 最大重试延迟 |
| `providers.<name>.retry.backoff` | float | `2.0` | 退避倍数 |
| `providers.<name>.headers` | map[string]string | — | 自定义请求头 |

默认 model_routing:
```yaml
model_routing:
  "deepseek-*": deepseek
  "minimax-*": minimax
  "MiniMax-*": minimax
```

默认内置提供商:

| 提供商 | type | base_url | api_key_env | default_model | fallback_model |
|--------|------|----------|-------------|---------------|----------------|
| deepseek | `"deepseek"` | `https://api.deepseek.com/v1` | `DEEPSEEK_API_KEY` | `deepseek-v4-flash` | `deepseek-v4-pro` |
| minimax | `"minimax"` | `https://api.minimaxi.com/v1` | `MINIMAX_API_KEY` | `MiniMax-M2.7-highspeed` | `MiniMax-M2.5-highspeed` |

```yaml
llm_gateway:
  default_provider: "minimax"
  default_model: "MiniMax-M2.7-highspeed"
  model_routing:
    "deepseek-*": deepseek
    "minimax-*": minimax
    "MiniMax-*": minimax
  circuit_breaker:
    failure_threshold: 5
    success_threshold: 2
    open_duration: "30s"
    scope: "provider"
  providers:
    deepseek:
      type: "deepseek"
      base_url: "https://api.deepseek.com/v1"
      api_key_env: "DEEPSEEK_API_KEY"
      default_model: "deepseek-v4-flash"
      fallback_model: "deepseek-v4-pro"
      timeout: "60s"
      max_tokens: 8192
      temperature: 0.7
      retry:
        max_attempts: 3
        initial_delay: "1s"
        max_delay: "10s"
        backoff: 2.0
    minimax:
      type: "minimax"
      base_url: "https://api.minimaxi.com/v1"
      api_key_env: "MINIMAX_API_KEY"
      default_model: "MiniMax-M2.7-highspeed"
      fallback_model: "MiniMax-M2.5-highspeed"
      timeout: "180s"
      max_tokens: 8192
      temperature: 0.7
      retry:
        max_attempts: 3
        initial_delay: "1s"
        max_delay: "10s"
        backoff: 2.0
```

**代码**:
- `internal/shared/config/llmgateway.go` — `LLMGatewayConfig`, `BuildLLMGatewayConfig()`, `DefaultLLMGatewayConfig()`
- `internal/layers/llmgateway/` — 运行时路由和熔断

---

## 14. 上下文引擎 (Layer 2 — context-engine)

上下文窗口管理、压缩、PEV 验证、Plan 阶段、长期记忆、Harness 引导。

### 14.1 核心参数

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `max_context_tokens` | int | `128000` | 上下文窗口最大 Token 数 |
| `reserved_output_tokens` | int | `8192` | 保留给输出的 Token 预算 |
| `tool_result_budget` | int | `800` | Tool 结果 Token 预算 |
| `compression_enabled` | bool | `true` | 启用上下文压缩 |

```yaml
context_engine:
  max_context_tokens: 128000
  reserved_output_tokens: 8192
  tool_result_budget: 800
  compression_enabled: true
```

### 14.2 Autocompact (压缩管道)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `compression.autocompact.enabled` | bool | `true` (YAML), `false` (代码默认) | 启用自动摘要压缩 |
| `compression.autocompact.model` | string | `"deepseek-v4-flash"` | 摘要生成模型 |
| `compression.autocompact.summary_max_tokens` | int | `512` | 每条摘要最大 Token |
| `compression.autocompact.min_messages_for_summary` | int | `8` | 触发压缩的最少消息数 |
| `compression.autocompact.preserve_head_turns` | int | `2` | 保留的开头轮次 |
| `compression.autocompact.preserve_tail_turns` | int | `2` | 保留的末尾轮次 |
| `compression.autocompact.timeout` | duration | `10s` | 摘要生成超时 (P99 上限) |

```yaml
  compression:
    autocompact:
      enabled: true
      model: "deepseek-v4-flash"
      summary_max_tokens: 512
      min_messages_for_summary: 8
      preserve_head_turns: 2
      preserve_tail_turns: 2
      timeout: "10s"
```

### 14.3 Token 计数源

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `token_counter.source` | string | `"gateway"` | 计数源: `gateway` \| `heuristic` |

```yaml
  token_counter:
    source: "gateway"
```

### 14.4 PEV (Plan-Execute-Verify)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `pev.max_iterations` | int | `3` | 最大 PEV 迭代次数 |
| `pev.verify_mode` | string | `"basic"` | 验证模式: `basic` \| `none` \| `commands` |
| `pev.verify_policy` | string | `"all_pass"` | 验证策略: `all_pass` \| `any_pass` |
| `pev.verify_commands` | []VerifyCommand | — | 自定义验证命令 (仅 `commands` 模式) |

```yaml
  pev:
    max_iterations: 3
    verify_mode: "basic"
    verify_policy: "all_pass"
    verify_commands:
      - name: "go-test"
        executable: "go"
        args: ["test", "./..."]
        timeout: "120s"
```

`VerifyCommand` 字段:

| 子字段 | 类型 | 约束 | 描述 |
|--------|------|------|------|
| `name` | string | 正则 `^[a-z0-9_-]+$` | 命令标识 |
| `executable` | string | 不含 shell 元字符 | 可执行文件路径 |
| `args` | []string | 每个不含 shell 元字符 | 参数列表 |
| `timeout` | duration | — | 命令超时 |

### 14.5 Snapshot

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `snapshot.enabled` | bool | `true` | 启用快照持久化 |
| `snapshot.backup_dir` | string | `"~/.devrix/context"` | 快照备份目录 |

```yaml
  snapshot:
    enabled: true
    backup_dir: "~/.devrix/context"
```

### 14.6 System Prompt

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `system_prompt.sources` | []string | `["AGENTS.md", ".devrix/AGENTS.md"]` | 系统提示来源文件 |
| `system_prompt.fallback` | string | `"You are Devrix..."` | 无源文件时的回退提示 |

```yaml
  system_prompt:
    sources:
      - "AGENTS.md"
      - ".devrix/AGENTS.md"
    fallback: "You are Devrix, a multi-agent development assistant."
```

### 14.7 Plan (V3, 默认关闭)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `plan.enabled` | bool | `false` | 启用 Plan 阶段 |
| `plan.auto_detect` | bool | `true` | 自动检测是否需要 Plan |
| `plan.min_chars_for_plan` | int | `200` | 触发 Plan 的最小字符数 |
| `plan.model` | string | `"deepseek-v4"` | Plan 阶段使用的模型 |
| `plan.max_milestones` | int | `10` | 最大里程碑数 |
| `plan.timeout` | duration | `15s` | Plan 阶段超时 |
| `plan.on_milestone_fail` | string | `"fail_fast"` | 里程碑失败行为 (仅支持 `fail_fast`) |

```yaml
  plan:
    enabled: false
    auto_detect: true
    min_chars_for_plan: 200
    model: "deepseek-v4"
    max_milestones: 10
    timeout: "15s"
    on_milestone_fail: "fail_fast"
```

### 14.8 LongTerm (V3, 长期记忆)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `longterm.enabled` | bool | `true` | 启用长期记忆 |
| `longterm.db_path` | string | `"~/.devrix/memory.db"` | SQLite 数据库路径 |
| `longterm.auto_store` | bool | `false` | 自动存储（不依赖显式指令） |
| `longterm.topics` | []string | `["architecture", "decisions", "bugs"]` | 记忆主题, 正则 `^[a-z0-9_-]+$` |
| `longterm.recall_max_entries` | int | `5` | 每次召回最大条目数 |
| `longterm.recall_max_tokens` | int | `2000` | 召回内容最大 Token 数 |

```yaml
  longterm:
    enabled: true
    db_path: "~/.devrix/memory.db"
    auto_store: false
    topics:
      - "architecture"
      - "decisions"
      - "bugs"
    recall_max_entries: 5
    recall_max_tokens: 2000
```

### 14.9 Harness Bootstrap (V5, 默认关闭)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `harness.enabled` | bool | `false` | 启用 Harness 引导 |
| `harness.trusted` | bool | `true` | 信任模式（跳过权限确认） |
| `harness.prefetch.enabled` | bool | `true` | 启用工作区预取 |
| `harness.prefetch.max_walk_depth` | int | `4` | 目录遍历最大深度 |
| `harness.tool_pool.simple_mode` | bool | `false` | 简化工具池模式 |
| `harness.tool_pool.include_mcp` | bool | `true` | 包含 MCP 工具 |
| `harness.tool_pool.deny_names` | []string | `[]` | 禁止的工具名列表 |
| `harness.tool_pool.deny_prefixes` | []string | `[]` | 禁止的工具名前缀 |
| `harness.routing.enabled` | bool | `false` | 启用提示路由 |
| `harness.routing.max_matches` | int | `5` | 最大路由匹配数 |
| `harness.deferred_init.enabled` | bool | `true` | 启用延迟初始化 |
| `harness.transcript.enabled` | bool | `true` | 启用转录 |
| `harness.transcript.compact_after_turns` | int | `20` | 多少轮后压缩转录 |
| `harness.transcript.session_log_enabled` | bool | `true` | 启用会话日志 |

```yaml
  harness:
    enabled: false
    trusted: true
    prefetch:
      enabled: true
      max_walk_depth: 4
    tool_pool:
      simple_mode: false
      include_mcp: true
      deny_names: []
      deny_prefixes: []
    routing:
      enabled: false
      max_matches: 5
    deferred_init:
      enabled: true
    transcript:
      enabled: true
      compact_after_turns: 20
      session_log_enabled: true
```

### 14.10 Preflight (V5, 默认关闭)

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `preflight.enabled` | bool | `false` | 启用 Preflight 检查 |
| `preflight.mode` | string | `"warn-only"` | 模式: `warn-only` (仅 V5a 支持) |
| `preflight.token_budget` | int | `8000` | 预检 Token 预算 |
| `preflight.warn_ratio` | float | `0.85` | 告警阈值比例 (0, 1] |
| `preflight.tool_filter.enabled` | bool | `true` | 启用工具相关性过滤 |
| `preflight.tool_filter.mode` | string | `"auto-repair"` | 过滤模式: `none` \| `auto-repair` |

```yaml
  preflight:
    enabled: false
    mode: warn-only
    token_budget: 8000
    warn_ratio: 0.85
    tool_filter:
      enabled: true
      mode: auto-repair
```

### 14.11 Workspace Prompt

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `workspace.max_context_tokens` | int | `8000` | 工作区最大 Token |
| `workspace.agent_name` | string | `"Devrix"` | Agent 名称 |
| `workspace.additional_context_files` | []string | — | 额外上下文文件 |
| `workspace.embed_core_template` | bool | `true` | 嵌入核心模板 |

```yaml
  workspace:
    max_context_tokens: 8000
    agent_name: Devrix
    embed_core_template: true
```

**代码**:
- `internal/shared/config/contextengine.go` — `ContextEngineConfig`, `DefaultContextEngineConfig()`
- `internal/shared/config/contextengine_v2.go` — `AutocompactConfig`, `CompressionConfig`, 验证逻辑
- `internal/shared/config/contextengine_v3.go` — `PlanConfig`, `LongTermConfig`, V3 验证
- `internal/shared/config/contextengine_harness.go` — `HarnessConfig`, `PreflightConfig`, `WorkspacePromptConfig`, 验证

---

## 15. 工具执行安全 (tool)

Bash 沙箱与并发控制。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `sandbox.enabled` | bool | `true` | 启用沙箱 |
| `sandbox.allowlist_extra` | []string | `[]` | 额外放行命令 |
| `sandbox.deny_patterns_extra` | []string | `[]` | 额外禁止模式 |
| `concurrent_max` | int | `10` | 最大并发工具执行数 |

```yaml
tool:
  sandbox:
    enabled: true
    allowlist_extra: []
    deny_patterns_extra: []
  concurrent_max: 10
```

**代码**: `internal/shared/config/tool_config.go` — `ToolConfig`, `ToolSandboxConfig`, `BuildToolConfig()`

---

## 16. 多智能体 (Layer 4 — multi-agent)

Agent 树结构控制。

| 字段 | 类型 | 默认值 | 约束 | 描述 |
|------|------|--------|------|------|
| `enabled` | bool | `false` | — | 启用多智能体模式 |
| `max_children` | int | `3` | ≤ 10 | 每个 Agent 的最大子 Agent 数 |
| `max_total_agents` | int | `5` | ≤ 20 | 系统总 Agent 数上限 |
| `default_timeout` | duration | `5m` | > 0 | Agent 默认超时 |
| `default_max_iter` | int | `50` | > 0 | Agent 默认最大迭代次数 |
| `permission_timeout` | duration | `60s` | > 0 | 权限等待超时 |
| `default_mode` | string | `"default"` | — | Agent 默认模式 |

```yaml
multi_agent:
  enabled: false
  max_children: 3
  max_total_agents: 5
  default_timeout: "5m"
  default_max_iter: 50
  permission_timeout: "60s"
  default_mode: "default"
```

**代码**: `internal/shared/config/multiagent.go` — `MultiAgentConfig`, `BuildMultiAgentConfig()`, `DefaultMultiAgentConfig()`

---

## 17. Agent 工具 (agent_tools)

外部 CLI Agent 工具注册，LLM 可按需调用。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `false` | 启用 Agent 工具 |
| `tools` | []AgentTool | `[]` | 工具注册列表 |

每项 `AgentTool`:

| 子字段 | 类型 | 默认值 | 描述 |
|--------|------|--------|------|
| `name` | string | — | 工具唯一标识 |
| `display_name` | string | — | 显示名称 |
| `description` | string | — | 工具描述 |
| `capabilities` | []string | — | 能力标签列表 |
| `role` | string | — | LLM 角色描述 |
| `type` | string | `"cli"` | 类型: `cli` \| `cursor` |
| `command` | string | — | 可执行文件路径 |
| `args` | []string | — | 启动参数 |
| `model` | string | — | (cursor) 模型覆盖 |
| `mode` | string | — | (cursor) 模式: `force` \| `plan` \| `ask` |
| `work_dir` | string | — | 工作目录 |
| `timeout` | duration | `5m` | 工具超时 |
| `idle_timeout` | duration | `5m` | 空闲超时 |

```yaml
agent_tools:
  enabled: true
  tools:
    - name: claude-code
      display_name: "Claude Code"
      description: "通用编码、代码审查、重构、调试"
      capabilities: ["coding", "code-review", "debug", "refactor"]
      role: |
        通用编码助手，精通代码审查、重构、调试与文档。
      command: "claude"
      args: ["--print", "--verbose"]
      timeout: "5m"
      idle_timeout: "5m"
```

**代码**: `internal/shared/config/multiagent.go` — `AgentToolConfig`, `AgentToolsConfig`, `BuildAgentToolsConfig()`

---

## 18. 编排验证 (Layer 6 — orchestration)

运行时决策验证 — 跨模型判官验证 Agent 路由决策，支持自动干预。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用编排验证 |
| `judge_provider` | string | `"minimax"` | 判官 LLM 提供商 |
| `judge_model` | string | `"MiniMax-M2.7-highspeed"` | 判官模型名 |
| `fallback_judge_provider` | string | `"deepseek"` | 判官降级提供商 |
| `fallback_judge_model` | string | `"deepseek-v4-flash"` | 判官降级模型 |
| `pre_filter_enabled` | bool | `true` | 启用前置过滤 |
| `min_interval_between_judges` | duration | `"2s"` | 判官调用最小间隔 |
| `max_judge_calls_per_minute` | int | `10` | 每分钟判官调用上限 |
| `trusted_tool_allowlist` | []string | `[]` | 受信任工具白名单（跳过判官） |
| `intervention_threshold` | float | `0.3` | 干预置信度阈值 (0.0~1.0) |
| `auto_intervene` | bool | `false` | 启用自动干预（谨慎使用） |

```yaml
orchestration:
  enabled: true
  judge_provider: "minimax"
  judge_model: "MiniMax-M2.7-highspeed"
  fallback_judge_provider: "deepseek"
  fallback_judge_model: "deepseek-v4-flash"
  pre_filter_enabled: true
  min_interval_between_judges: "2s"
  max_judge_calls_per_minute: 10
  trusted_tool_allowlist: []
  intervention_threshold: 0.3
  auto_intervene: false
```

**判官工作流**:
1. Agent 做出 tool_call/fork/permit 决策
2. `preFilter` — 白名单 + 频率限制，通过则跳过判官
3. `RuntimeJudge.ValidateDecision` — 跨模型判官 (minimax) 验证决策
4. 判官返回 `{Valid, Confidence, SuggestedAction}`
5. `Valid=false` 且 `Confidence < threshold` → 触发 Intervention
6. `auto_intervene=true` → 自动终止当前 Agent 并 reroute

**可观测指标**:
| 指标 | 类型 | 标签 | 描述 |
|------|------|------|------|
| `orch_decisions_total` | Counter | category, risk_class | 进入验证管道的决策总数 |
| `orch_decisions_by_stage` | Counter | stage | 按阶段分布：incoming, prefilter_skip, judge_pass, judge_error, intervention, intervention_error |
| `orch_validations_total` | Counter | result | 判官验证结果计数 |
| `orch_interventions_total` | Counter | action | 干预动作计数（terminate/reroute/deny） |
| `orch_judge_latency_seconds` | Histogram | provider, model | 判官 LLM 调用延迟分布 |
| `orch_observer_active` | Gauge | session_id | Observer 存活信号（=1 表示已注册） |

**Tracing span**: `D6_S4_Validation_Decision`（别名 `evolution.decision.validate`）— 每次决策验证创建一个 span，包含 preFilter/judge/intervention 阶段事件，在 Jaeger 中可追踪单次决策全链路。

**代码**:
- `internal/shared/config/orchestration.go` — `OrchestrationConfig`, `BuildOrchestrationConfig()`, `DefaultOrchestrationConfig()`
- `internal/layers/evolution/orchestration/` — 运行时：`validator.go`, `judge_adapter.go`, `intervention.go`, `observer.go`, `metrics.go`

---

## 19. 可观察性 (Layer 5 — observability)

Tracing / Metrics / Logging / Health 统一配置。

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用可观察性 |

### 19.1 Tracing

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `tracing.enabled` | bool | `true` | 启用 Tracing |
| `tracing.service_name` | string | `"devrix"` | 服务名称 |
| `tracing.service_version` | string | `"2.0.0"` | 服务版本 |
| `tracing.exporter` | string | `"otlp"` | 导出器: `console` \| `otlp` \| `null` \| `memory` |
| `tracing.sampling.type` | string | `"always_on"` | 采样类型: `always_on` \| `always_off` \| `trace_id_ratio` |
| `tracing.sampling.rate` | float | `1.0` | 采样率 (0.0~1.0, 仅 `trace_id_ratio`) |
| `tracing.otlp.endpoint` | string | `"http://localhost:4318/v1/traces"` | OTLP HTTP 端点 |
| `tracing.otlp.insecure` | bool | `true` | 允许非安全连接 |
| `tracing.otlp.timeout` | duration | `5s` | 导出超时 |

### 19.2 Metrics

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `metrics.enabled` | bool | `true` | 启用 Metrics |
| `metrics.exporter` | string | `"prometheus"` | 导出器: `prometheus` \| `otlp` \| `null` |
| `metrics.endpoint` | string | `"/metrics"` | Metrics HTTP 路径 |
| `metrics.labels.allowlist` | []string | *见下方* | 允许的标签列表 |
| `metrics.labels.blocklist` | []string | *见下方* | 禁止的标签列表 |

默认 `labels.allowlist`:
```yaml
- provider
- model
- adapter
- tool
- risk_level
- status
- direction
- decision
- error_type
```

默认 `labels.blocklist`:
```yaml
- session_id
- user_id
- api_key
```

### 19.3 Logging

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `logging.enabled` | bool | `true` | 启用日志 |
| `logging.level` | string | `"info"` | 级别: `debug` \| `info` \| `warn` \| `error` |
| `logging.format` | string | `"json"` | 格式: `json` \| `text` |
| `logging.include_trace_id` | bool | `true` | 包含 Trace ID |
| `logging.sampling.enabled` | bool | `true` | 启用日志采样 |
| `logging.sampling.max_entries_per_span` | int | `100` | 每 Span 最大日志条目 |
| `logging.redactor.enabled` | bool | `true` | 启用敏感信息脱敏 |
| `logging.redactor.patterns` | []string | `["password", "token", "secret", "api_key", "authorization", "private_key", "access_token"]` | 脱敏关键词 |

### 19.4 Health

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `health.enabled` | bool | `true` | 启用 Health 端点 |
| `health.endpoint` | string | `"/health"` | Health 端点路径 |

### 19.5 LLM 日志

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `llm.log_content` | bool | `false` | 记录 LLM 请求/响应内容 |
| `llm.log_dir` | string | `"~/.devrix/logs/llm"` | LLM 日志目录 |

```yaml
observability:
  enabled: true
  tracing:
    enabled: true
    service_name: "devrix"
    service_version: "2.0.0"
    exporter: "otlp"
    sampling:
      type: "always_on"
      rate: 1.0
    otlp:
      endpoint: "http://localhost:4318/v1/traces"
      insecure: true
      timeout: "5s"
  metrics:
    enabled: true
    exporter: "prometheus"
    endpoint: "/metrics"
    labels:
      allowlist:
        - provider
        - model
        - adapter
        - tool
        - risk_level
        - status
        - direction
        - decision
        - error_type
      blocklist:
        - session_id
        - user_id
        - api_key
  logging:
    enabled: true
    level: "debug"
    format: "json"
    include_trace_id: true
    sampling:
      enabled: true
      max_entries_per_span: 100
    redactor:
      enabled: true
      patterns:
        - password
        - token
        - secret
        - api_key
        - authorization
  health:
    enabled: true
    endpoint: "/health"
  llm:
    log_content: true
    log_dir: "~/.devrix/logs/llm"
```

**代码**:
- `internal/layers/observability/config.go` — `Config`, `DefaultConfig()`, `Validate()`
- `internal/layers/observability/settings/config.go` — `TracingConfig`, `MetricsConfig`, `SamplingConfig`, `OTLPConfig`, `LabelsConfig`

---

## 20. 用户配置 (user config)

> 路径: `~/.devrix/config.yaml` · 独立于 `devrix.yaml` 的系统配置

### 20.1 User

| 字段 | 环境变量 | 描述 |
|------|---------|------|
| `user.name` | `DEVRIX_USER_NAME` | 用户名 |
| `user.email` | `DEVRIX_USER_EMAIL` | 邮箱 |

### 20.2 UI

| 字段 | 环境变量 | 默认值 | 描述 |
|------|---------|--------|------|
| `ui.theme` | `DEVRIX_UI_THEME` | `"auto"` | 主题: `auto` \| `light` \| `dark` |
| `ui.language` | `DEVRIX_UI_LANGUAGE` | `"zh-CN"` | 语言: `zh-CN` \| `en-US` |
| `ui.emoji` | — | `true` | 是否使用 emoji |
| `ui.color_output` | — | `true` | 彩色输出 |

### 20.3 Model

| 字段 | 环境变量 | 默认值 | 描述 |
|------|---------|--------|------|
| `model.provider` | `DEVRIX_MODEL_PROVIDER` | `"openai"` | 模型提供商 |
| `model.model` | `DEVRIX_MODEL_NAME` | `"gpt-4"` | 模型名 |
| `model.api_key` | `DEVRIX_MODEL_API_KEY` | `""` | API Key |
| `model.base_url` | `DEVRIX_MODEL_BASE_URL` | `""` | 自定义 API 地址 |

### 20.4 YOLO 模式

| 字段 | 环境变量 | 默认值 | 描述 |
|------|---------|--------|------|
| `yolo.enabled` | `DEVRIX_YOLO_MODE` | `false` | 启用 YOLO 模式 |
| `yolo.auto_approve_tools` | — | `false` | 自动批准工具执行 |
| `yolo.auto_approve_files` | — | `false` | 自动批准文件操作 |
| `yolo.auto_approve_network` | — | `false` | 自动批准网络请求 |
| `yolo.confirm_before_exec` | — | `true` | 执行前确认（非 YOLO） |
| `yolo.trust_plugins` | — | `false` | 信任插件 |

`DEVRIX_YOLO_MODE=true` 会同时启用所有 `auto_approve_*`。

### 20.5 IM 平台

| 字段 | 环境变量 | 默认值 | 描述 |
|------|---------|--------|------|
| `im.enabled` | `DEVRIX_IM_ENABLED` | `false` | 启用 IM |
| `im.engine` | `DEVRIX_ENGINE` | `"context"` | 引擎: `context` (真实 LLM) \| `stub` |
| `im.platform.provider` | `DEVRIX_IM_PROVIDER` | `"none"` | 平台: `feishu` \| `dingtalk` \| `none` |
| `im.feishu.app_id` | `FEISHU_APP_ID` | `""` | 飞书 App ID |
| `im.feishu.app_secret` | `FEISHU_APP_SECRET` | `""` | 飞书 App Secret |
| `im.feishu.bot_name` | `FEISHU_BOT_NAME` | `"Devrix"` | 飞书机器人名 |
| `im.feishu.domain` | — | `""` | 自定义域名 |
| `im.feishu.encrypt_key` | — | `""` | 加密密钥 |
| `im.feishu.callback_url` | — | `""` | 回调地址 |
| `im.feishu.use_webhook` | — | `false` | Webhook 模式 |
| `im.feishu.reaction_emoji` | — | `"OnIt"` | 消息确认表情 |
| `im.feishu.done_emoji` | — | `"Done"` | 完成表情 |
| `im.feishu.reply_in_thread` | — | `false` | 话题回复 |
| `im.feishu.progress_style` | — | `"structured"` | 进度样式: `legacy` \| `compact` \| `card` \| `structured` |
| `im.dingtalk.app_key` | `DINGTALK_APP_KEY` | `""` | 钉钉 App Key |
| `im.dingtalk.app_secret` | `DINGTALK_APP_SECRET` | `""` | 钉钉 App Secret |
| `im.dingtalk.bot_code` | `DINGTALK_BOT_CODE` | `""` | 钉钉机器人 code |
| `im.dingtalk.callback_url` | — | `""` | 回调地址 |
| `im.dingtalk.encrypt_key` | — | `""` | 加密密钥 |
| `im.dingtalk.use_webhook` | — | `false` | Webhook 模式 |

> 当 `FEISHU_APP_ID` 或 `DINGTALK_APP_KEY` 设置时，`im.platform.provider` 会自动设为对应值。

### 20.6 其他

| 字段 | 默认值 | 描述 |
|------|--------|------|
| `shortcuts.new_session` | `"Ctrl+N"` | 新会话快捷键 |
| `shortcuts.stop` | `"Ctrl+C"` | 停止快捷键 |
| `shortcuts.help` | `"Ctrl+H"` | 帮助快捷键 |
| `plugins.enabled` | `true` | 启用插件 |
| `plugins.auto_update` | `true` | 自动更新插件 |
| `plugins.list` | `[]` | 插件列表 |
| `privacy.telemetry` | `false` | 发送使用统计 |
| `privacy.save_history` | `true` | 保存聊天历史 |

**代码**: `internal/shared/config/user.go` — `UserConfig`, `DefaultUserConfig()`, `LoadUserConfig()`, `SaveUserConfig()`

---

## 21. 环境变量索引

> 优先级: 环境变量 > YAML 文件 > 代码默认值

### 21.1 认证

| 变量 | 对应配置 | 描述 |
|------|---------|------|
| `DEVRIX_AUTH_SECRET` | `auth.secret` | JWT 签名密钥 |
| `DEVRIX_AUTH_TOKEN_EXPIRY` | `auth.token_expiry` | Token 过期时间 (Go duration) |
| `DEVRIX_AUTH_ISSUER` | `auth.issuer` | JWT 签发者 |

### 21.2 会话

| 变量 | 对应配置 | 描述 |
|------|---------|------|
| `DEVRIX_SESSION_DIR` | `session.storage_dir` | 会话存储目录 |
| `DEVRIX_SESSION_TIMEOUT` | `session.idle_timeout` | 空闲超时 |

### 21.3 权限

| 变量 | 对应配置 | 描述 |
|------|---------|------|
| `DEVRIX_PERMISSION_TIMEOUT` | `permission.default_timeout` | 权限审批超时 |

### 21.4 限流

| 变量 | 对应配置 | 描述 |
|------|---------|------|
| `DEVRIX_RATE_LIMIT_RPM` | `rate_limit.requests_per_minute` | 每分钟请求数 |
| `DEVRIX_RATE_LIMIT_BURST` | `rate_limit.burst_size` | 突发容量 |
| `DEVRIX_RATE_LIMIT_ENABLED` | `rate_limit.enabled` | 启用限流 (`true`/`false`) |

### 21.5 LLM 网关

| 变量 | 对应配置 | 描述 |
|------|---------|------|
| `DEEPSEEK_API_KEY` | `llm_gateway.providers.deepseek` | DeepSeek API Key |
| `MINIMAX_API_KEY` | `llm_gateway.providers.minimax` | MiniMax API Key |

### 21.6 用户配置

| 变量 | 对应配置 | 描述 |
|------|---------|------|
| `DEVRIX_USER_NAME` | `user.name` | 用户名 |
| `DEVRIX_USER_EMAIL` | `user.email` | 邮箱 |
| `DEVRIX_MODEL_PROVIDER` | `model.provider` | 模型提供商 |
| `DEVRIX_MODEL_NAME` | `model.model` | 模型名 |
| `DEVRIX_MODEL_API_KEY` | `model.api_key` | API Key |
| `DEVRIX_MODEL_BASE_URL` | `model.base_url` | 自定义 API 地址 |
| `DEVRIX_YOLO_MODE` | `yolo.*` | YOLO 模式 (启用所有自动批准) |
| `DEVRIX_UI_THEME` | `ui.theme` | 主题 |
| `DEVRIX_UI_LANGUAGE` | `ui.language` | 语言 |
| `DEVRIX_IM_ENABLED` | `im.enabled` | 启用 IM |
| `DEVRIX_IM_PROVIDER` | `im.platform.provider` | IM 提供商 |
| `DEVRIX_ENGINE` | `im.engine` | 引擎类型 |
| `FEISHU_APP_ID` | `im.feishu.app_id` | 飞书 App ID (自动设 provider) |
| `FEISHU_APP_SECRET` | `im.feishu.app_secret` | 飞书 App Secret |
| `FEISHU_BOT_NAME` | `im.feishu.bot_name` | 飞书机器人名 |
| `DINGTALK_APP_KEY` | `im.dingtalk.app_key` | 钉钉 App Key (自动设 provider) |
| `DINGTALK_APP_SECRET` | `im.dingtalk.app_secret` | 钉钉 App Secret |
| `DINGTALK_BOT_CODE` | `im.dingtalk.bot_code` | 钉钉机器人 code |

---

## 配置加载流程

```
devrix.yaml
  │
  ├── LoadConfigFile(path) → ConfigFile (YAML 结构体)
  │     │
  │     ├── BuildOrchestrationConfig(&fileCfg.Orchestration)    → *OrchestrationConfig
  │     ├── BuildLLMGatewayConfig(&fileCfg.LLMGateway)          → *LLMGatewayConfig
  │     ├── BuildMultiAgentConfig(&fileCfg.MultiAgent)          → *MultiAgentConfig
  │     ├── BuildAgentToolsConfig(&fileCfg.AgentTools)          → *AgentToolsConfig
  │     ├── BuildToolConfig(fileCfg)                            → *ToolConfig
  │     ├── buildCommunicationConfig(fileCfg)                   → *CommunicationConfig
  │     ├── buildAuthConfig(fileCfg)                            → *AuthConfig
  │     ├── buildRateLimitConfig(fileCfg)                       → *RateLimitConfig
  │     └── buildContextEngineConfig(fileCfg)                   → *ContextEngineConfig
  │
  └── 每个 BuildXXX 函数: 先取 DefaultXXXConfig() → 用 YAML 值覆盖非零字段

~/.devrix/config.yaml (用户配置)
  │
  └── LoadUserConfig() → UserConfig
        ├── DefaultUserConfig()
        ├── 环境变量覆盖
        ├── YAML 文件解析
        └── 环境变量再次覆盖 (环境变量最高优先级)
```

---

## 快速参考: 默认启用的关键功能

| 功能 | 配置 | 代码默认 | devrix.yaml |
|------|------|---------|-------------|
| 编排验证 | `orchestration.enabled` | `false` | `true` |
| Prometheus Metrics | `metrics.enabled` | — | `true` |
| Tracing | `observability.tracing.enabled` | `true` | `true` |
| 上下文压缩 | `context_engine.compression_enabled` | `true` | `true` |
| Autocompact | `context_engine.compression.autocompact.enabled` | `false` | `true` |
| 长期记忆 | `context_engine.longterm.enabled` | `true` | `true` |
| 沙箱 | `tool.sandbox.enabled` | `true` | `true` |
| 限流 | `rate_limit.enabled` | `true` | `true` |
| 多智能体 | `multi_agent.enabled` | `false` | `false` (opt-in) |
| Harness Bootstrap | `context_engine.harness.enabled` | `false` | `false` (opt-in) |
| Plan 阶段 | `context_engine.plan.enabled` | `false` | `false` (opt-in) |
| 自动干预 | `orchestration.auto_intervene` | `false` | `false` (opt-in) |
