# Communication Layer Design (Layer 1)

**Change ID:** `devrix-foundation`
**Layer:** 1 - Communication
**Status:** Draft
**Version:** 1.0
**Based on:** Obsidian `01知识探索/项目/devrix/docs/architecture/通信层设计.md`

---

## 一、架构目标

### 1.1 业务目标

| 业务目标 | 量化指标 | V1 实现 |
|---------|---------|---------|
| **IM 远程控制** | 支持飞书/钉钉/Telegram/CLI 多端接入 | CLI |
| **流式响应** | LLM 输出实时推送，端到端延迟 < 500ms | ✅ |
| **权限审批** | 敏感操作需用户确认，支持 60s 超时自动拒绝 | ✅ |
| **多会话隔离** | 不同 chatId 独立会话，支持 30min 空闲超时 | ✅ |
| **开发助手能力** | 对标 Claude Code，工具调用、子 Agent、上下文压缩 | ✅ |

### 1.2 技术约束

| 约束类型 | 指标要求 | V1 实现方案 |
|---------|---------|------------|
| **可用性** | 99.9% (月停机 < 44min) | 消息确认 + 重试 |
| **延迟** | P99 < 2s（不含 LLM 推理） | 心跳保活 + 消息确认 |
| **并发** | 单 Gateway 支持 1000 并发会话 | 水平扩展 Adapter |
| **消息可靠性** | 0 消息丢失，99.99% 送达 | ack + 重试 + 去重 |
| **数据持久性** | 会话状态持久化，重启不丢失 | FileSessionStore |
| **安全性** | Adapter 共享密钥认证，消息加密传输 | V2 实现 |

---

## 二、架构原则

| 原则 | 说明 | 实施方式 |
|-----|------|---------|
| **高内聚低耦合** | Adapter 只负责平台适配，Gateway 只负责路由 | 清晰接口边界 |
| **面向失败设计** | 熔断、降级、重试、幂等 | 心跳检测 + 消息确认 |
| **数据所有权** | 会话状态由 SessionStore 统一管理 | 持久化存储 |
| **单向依赖** | 通信层 → 引擎层，Gateway 不可反向依赖 Adapter | 接口抽象 |
| **最小暴露** | 仅通信层对外暴露 WebSocket 接口 | Gateway 集中入口 |

### 2.1 命名规范

| 类别 | 命名规范 | 示例 |
|-----|---------|------|
| **模块前缀** | `layer-name/` | `communication/gateway/` |
| **类名** | PascalCase，核心类以功能命名 | `IMGateway`, `SessionStore` |
| **接口类型** | `I` 前缀 + PascalCase | `IContextEngine` |
| **消息类型** | `SnakeCase` + `_message` 后缀 | `im_message`, `text_message` |
| **事件类型** | `on_event` 或 `handle_event` | `onConnectionLost`, `handlePong` |
| **错误码** | `UPPER_SNAKE_CASE` | `SESSION_NOT_FOUND` |

---

## 三、领域模型

### 3.1 核心实体关系图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            核心实体关系图                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐         ┌─────────────────┐                          │
│  │    Platform     │         │    Adapter      │                          │
│  │    (IM平台)     │────────▶│   (适配器)      │                          │
│  │                 │  1:N   │                 │                          │
│  └─────────────────┘         └────────┬────────┘                          │
│                                        │                                   │
│                                        │ 维护                              │
│                                        ▼                                   │
│  ┌─────────────────┐         ┌─────────────────┐                          │
│  │  Connection     │◀────────│    Gateway      │                          │
│  │  (WebSocket连接) │  N:1   │   (网关)        │                          │
│  └────────┬────────┘         └────────┬────────┘                          │
│           │                           │                                   │
│           │ 属于                     │ 路由                              │
│           ▼                           ▼                                   │
│  ┌─────────────────┐         ┌─────────────────┐                          │
│  │    Session      │────────▶│  ContextEngine  │                          │
│  │   (会话-聚合根)  │  1:1   │   (引擎层)      │                          │
│  └────────┬────────┘         └─────────────────┘                          │
│           │                                                             │
│           │ 1:N                                                        │
│           ├──────────────────────────────────┐                           │
│           │                                  │                           │
│           ▼                                  ▼                           │
│  ┌─────────────────┐               ┌─────────────────┐                  │
│  │PermissionRequest │               │    Message      │                  │
│  │  (权限请求)      │               │    (消息)        │                  │
│  └─────────────────┘               └─────────────────┘                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Session（会话 - 聚合根）

```go
// internal/shared/types/session.go

type Session struct {
    // 标识
    SessionID    string      // 内部会话 ID（sess_时间戳_随机）
    RequestID    string      // 请求关联 ID
    AdapterID    string      // 所属 Adapter

    // 参与者
    UserID       string      // 用户 ID（CLI 无用户概念）
    UserName     string      // 用户名

    // 环境
    WorkDir      string      // 工作目录
    Model        string      // 指定模型

    // 状态
    State        SessionState // idle | thinking | streaming | tool_executing | waiting_permission | completed | failed
    CurrentAgentID string    // 当前 Agent ID

    // 生命周期
    CreatedAt    time.Time   // 创建时间
    UpdatedAt    time.Time   // 更新时间
    LastMessageAt time.Time  // 最后消息时间

    // 上下文快照（可选持久化）
    ContextSnapshot []byte
}

type SessionState string

const (
    SessionStateIdle             SessionState = "idle"
    SessionStateThinking        SessionState = "thinking"
    SessionStateStreaming       SessionState = "streaming"
    SessionStateToolExecuting   SessionState = "tool_executing"
    SessionStateWaitingPermission SessionState = "waiting_permission"
    SessionStateCompleted       SessionState = "completed"
    SessionStateFailed         SessionState = "failed"
)
```

### 3.3 Message（消息 - 实体）

```go
// internal/shared/types/message.go

type Message struct {
    ID         string       // 消息 ID
    SessionID  string       // 所属会话
    Role       MessageRole  // user | assistant | system
    Content    string       // 内容
    Attachments []Attachment // 附件
    Metadata   map[string]string // 元数据
    Timestamp  time.Time    // 时间戳
}

type MessageRole string

const (
    MessageRoleUser      MessageRole = "user"
    MessageRoleAssistant MessageRole = "assistant"
    MessageRoleSystem    MessageRole = "system"
)

type Attachment struct {
    Type    AttachmentType // file | image | code
    Name    string
    Path    string
    Content string
}

type AttachmentType string

const (
    AttachmentTypeFile  AttachmentType = "file"
    AttachmentTypeImage AttachmentType = "image"
    AttachmentTypeCode  AttachmentType = "code"
)
```

### 3.4 PermissionRequest（权限请求 - 实体）

```go
// internal/shared/types/permission.go

type PermissionRequest struct {
    ID          string           // 内部 ID (UUID)
    SessionID   string           // 所属会话
    ToolName    string           // 工具名称
    Description string           // 工具描述
    InputPreview string          // 输入预览（截断）
    RiskLevel   RiskLevel       // LOW | MEDIUM | HIGH | CRITICAL

    // 生命周期
    CreatedAt   time.Time       // 创建时间
    ExpiresAt   time.Time       // 过期时间（创建 + 60s）
    Status      PermissionStatus // pending | approved | denied | expired
    RespondedAt time.Time       // 响应时间
    Response    *bool           // 响应结果（nil=未响应）
}

type RiskLevel string

const (
    RiskLevelLow      RiskLevel = "LOW"
    RiskLevelMedium   RiskLevel = "MEDIUM"
    RiskLevelHigh     RiskLevel = "HIGH"
    RiskLevelCritical RiskLevel = "CRITICAL"
)

type PermissionStatus string

const (
    PermissionStatusPending   PermissionStatus = "pending"
    PermissionStatusApproved PermissionStatus = "approved"
    PermissionStatusDenied   PermissionStatus = "denied"
    PermissionStatusExpired  PermissionStatus = "expired"
)
```

### 3.5 V1/V2/V3 实体差异

| 实体 | V1 | V2 | V3 |
|------|-----|-----|-----|
| Session | ✅ 核心字段 | ✅ | ✅ 完整 |
| Message | ✅ | ✅ | ✅ |
| PermissionRequest | ✅ | ✅ | ✅ |
| ShortId (5位) | ❌ requestId 直接使用 | ✅ | ✅ |
| Milestone | ✅ DAG + 进度追踪 | ✅ | ✅ 完整 |

---

## 四、四流设计

### 4.1 四流概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            四流模型                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  ① 指令流（Command Flow）— 用户主动发起                             │   │
│  │     用户消息 → Gateway → Engine → 工具执行                            │   │
│  │     特征：用户可见意图，可取消/停止                                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  ② 事件流（Event Flow）— 系统状态变化通知                            │   │
│  │     EngineEvent → Gateway → Adapter → UI                              │   │
│  │     特征：实时推送、用户可感知Thinking/工具调用等内部过程               │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  ③ 任务流（Task Flow）— 长期任务进度追踪                             │   │
│  │     Milestone DAG → 进度更新 → 状态同步                               │   │
│  │     ✅ V1 已实现 Milestone DAG + 进度追踪                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  ④ 信息流（Information Flow）— 辅助信息展示                            │   │
│  │     帮助信息 / 错误提示 / 使用技巧 / 系统状态                          │   │
│  │     特征：用户可查阅、不阻塞主流程                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 ① 指令流详细时序（V1 实现）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              指令流时序图                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  用户输入 ─────────────────────────────────────────────────────────────▶    │
│    │                                                                       │
│    │ "帮我重构 UserService，去掉上帝类"                                   │
│    │                                                                       │
│    ▼                                                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  CLI Adapter                                                         │   │
│  │    - 解析消息，提取意图                                               │   │
│  │    - 生成 messageId（ClientToken）                                   │   │
│  │    - 添加 sender、chatId 等元数据                                      │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│    │                                                                       │
│    │ im_message { content, messageId, chatId, userId, ... }             │
│    ▼                                                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Gateway                                                             │   │
│  │    - 路由到对应 Session                                               │   │
│  │    - 发送 ack 确认                                                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│    │                                                                       │
│    │ sendMessage(sessionId, userMessage)                                 │
│    ▼                                                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Engine Layer                                                        │   │
│  │    - 处理用户消息                                                     │   │
│  │    - 工具调用（可能触发权限请求）                                      │   │
│  │    - 返回 AsyncIterable<EngineEvent>                                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  指令类型（V1 实现）:                                                      │
│    - /new         新建会话                                               │
│    - /stop        停止当前生成                                           │
│    - /help        显示帮助                                               │
│                                                                             │
│  V2/V3 扩展指令:                                                          │
│    - /retry       重试上次失败（V2）                                      │
│    - /context     显示上下文摘要（V2）                                     │
│    - /plan        显示任务计划（V3，需要 Milestone）                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 ② 事件流详细映射（V1 实现）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          EngineEvent → Adapter 映射                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Engine Layer ──────────────────────────────────────────────────────────▶    │
│    │                                                                       │
│    │ AsyncIterable<EngineEvent>                                            │
│    │                                                                       │
│    ▼                                                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │ thinking    │  │ text        │  │ tool_call   │  │ permission  │       │
│  │ (思考中)   │  │ (流式文本)  │  │ (工具调用)  │  │ (权限请求)  │       │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘       │
│         │                 │                 │                 │             │
│         ▼                 ▼                 ▼                 ▼             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │ status     │  │ text        │  │ tool_block  │  │ permission  │       │
│  │ event      │  │ event       │  │ message     │  │ request     │       │
│  │ (状态变更) │  │ (流式)     │  │ (折叠块)   │  │ (卡片)     │       │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘       │
│                                                                             │
│  事件类型 → UI 映射:                                                       │
│                                                                             │
│  ┌───────────────┬─────────────────────────────────────────────────────┐   │
│  │ EngineEvent   │ UI 渲染                                             │   │
│  ├───────────────┼─────────────────────────────────────────────────────┤   │
│  │ thinking     │ 加载动画 / 省略号 / "思考中..."                    │   │
│  │ text         │ 流式文本追加                                        │   │
│  │ tool_call    │ 显示工具调用卡片（V1 简化，不展示参数详情）         │   │
│  │ tool_result  │ 展开工具结果                                        │   │
│  │ permission   │ 显示权限请求卡片                                    │   │
│  │ status       │ 状态徽章更新                                        │   │
│  │ complete     │ 完成摘要 + Token 统计                               │   │
│  │ error        │ 错误提示 + 恢复建议                                  │   │
│  └───────────────┴─────────────────────────────────────────────────────┘   │
│                                                                             │
│  V2/V3 扩展事件:                                                          │
│    - tool.called      → 工具开始调用（V2）                                 │
│    - milestone.created → 任务创建（V1）                                     │
│    - milestone.updated → 进度更新（V1）                                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.4 ③ 任务流（V1 实现）

> **V1 已实现**：Milestone DAG + 进度追踪。

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              任务流（V1 实现）                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ✅ V1 实现:                                                              │
│     - Milestone DAG 支持依赖关系                                            │
│     - 进度百分比追踪 (0.0 - 1.0)                                           │
│     - 状态机：pending → in_progress → completed/failed                     │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                                                                      │   │
│  │  ┌─────────────┐                                                    │   │
│  │  │ 重构任务    │                                                    │   │
│  │  │  (root)     │                                                    │   │
│  │  └──────┬──────┘                                                    │   │
│  │         │                                                            │   │
│  │         ├──▶ ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │   │
│  │         │    │ 提取方法    │  │ 创建新类    │  │ 更新引用    │    │   │
│  │         │    │ (m1)        │  │ (m2)        │  │ (m3)        │    │   │
│  │         │    └─────────────┘  └─────────────┘  └─────────────┘    │   │
│  │         │                                                            │   │
│  │         └──▶ ┌─────────────┐                                         │   │
│  │              │ 测试验证    │                                         │   │
│  │              │ (m4)        │                                         │   │
│  │              └─────────────┘                                         │   │
│  │                                                                      │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.5 ④ 信息流（V1 实现）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              信息流                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  帮助信息 ──────────────────────────────────────────────────────────────▶    │
│                                                                             │
│  V1 实现内容:                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  信息类型                      │ 展示方式                              │   │
│  ├─────────────────────────────────┼──────────────────────────────────────┤   │
│  │ 帮助信息 (/help)               │ 折叠块                                │   │
│  │ 错误提示                       │ 错误消息                              │   │
│  │ 快捷命令                       │ 帮助面板                              │   │
│  │ 系统状态                       │ 状态徽章（连接/断开）                 │   │
│  └─────────────────────────────────┴──────────────────────────────────────┘   │
│                                                                             │
│  V2/V3 扩展内容:                                                          │
│    - 使用技巧（首回推送）                                                 │
│    - Token 统计（完成时）                                                 │
│    - 任务进度（V3）                                                        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.6 四流与组件映射

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            四流 → 组件映射                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ① 指令流                                                                 │
│     入口: CLI Adapter                                                        │
│     处理: Gateway → Engine Layer                                           │
│     反馈: status event                                                      │
│     V1: ✅ 完整实现                                                        │
│                                                                             │
│  ② 事件流                                                                 │
│     产生: Engine Layer (AsyncIterable)                                       │
│     传递: Gateway → Adapter                                                 │
│     消费: UI Renderer                                                       │
│     V1: ✅ 核心事件完整                                                    │
│                                                                             │
│  ③ 任务流                                                                 │
│     生成: PEV Engine (Plan 阶段) → V3 实现                                  │
│     更新: PEV Engine (Execute/Verify 阶段) → V3 实现                       │
│     展示: Milestone Card / Progress Bar → V3 实现                          │
│     V1: ❌ 暂不实现                                                        │
│                                                                             │
│  ④ 信息流                                                                 │
│     来源: HelpService / StatusManager                                        │
│     触发: 命令 / 完成 / 错误                                                │
│     展示: Help Card / Error Message / Status Badge                          │
│     V1: ✅ 简化实现                                                        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 五、领域事件

### 5.1 领域事件（V1 简化版）

| 事件 | 触发条件 | 消费者 | 副作用 |
|-----|---------|-------|--------|
| `session.created` | 新建会话 | Adapter（恢复 UI） | 绑定 chatId |
| `session.expired` | 30min 空闲 | Adapter（清理状态） | 解绑 chatId |
| `message.received` | 收到用户消息 | Engine（处理） | 更新 lastMessageAt |
| `message.sent` | 发送消息 | Adapter（渲染） | 更新 UI |
| `permission.requested` | 工具需审批 | Adapter（显示卡片） | 等待用户 |

### 5.2 事件扩展计划

| 事件 | V1 | V2 | V3 |
|------|-----|-----|-----|
| `session.created` | ✅ | ✅ | ✅ |
| `session.expired` | ✅ | ✅ | ✅ |
| `message.received` | ✅ | ✅ | ✅ |
| `message.sent` | ✅ | ✅ | ✅ |
| `permission.requested` | ✅ | ✅ | ✅ |
| `permission.responded` | ⚠️ 合并到 requested 响应 | ✅ | ✅ |
| `permission.expired` | ⚠️ 内部处理 | ✅ | ✅ |
| `tool.called` | ❌ | ❌ | ✅ |
| `connection.lost` | ❌ | ✅ | ✅ |
| `connection.restored` | ❌ | ✅ | ✅ |
| `milestone.created` | ✅ | ✅ | ✅ |
| `milestone.updated` | ✅ | ✅ | ✅ |

---

## 六、接口设计

### 6.1 消息契约

#### Adapter → Gateway

| 消息类型 | 路径 | 字段 | 说明 |
|----------|------|------|------|
| `register` | `POST /adapters` | platform, adapterId, secret | 适配器注册 |
| `im_message` | `WS /adapters/:id/messages` | chatId, userId, content, messageId | IM 消息 |
| `permission_reply` | `WS /adapters/:id/permissions` | sessionId, requestId, allowed | 权限回复 |
| `stop` | `WS /adapters/:id/sessions/:sid/stop` | — | 停止生成 |
| `new_session` | `WS /adapters/:id/sessions` | chatId, workDir | 新建会话 |

#### Gateway → Adapter

| 消息类型 | 字段 | 说明 |
|----------|------|------|
| `registered` | adapterId, gatewayId, token | 注册确认 |
| `text` | messageId, chatId, content, isComplete | 文本（流式） |
| `tool_block` | messageId, chatId, toolCall | 工具调用块 |
| `tool_result` | messageId, chatId, toolCallId, output, error | 工具结果 |
| `permission_request` | chatId, sessionId, requestId, toolName, expiresAt | 权限请求 |
| `permission_result` | chatId, requestId, status | 权限结果 |
| `status` | chatId, sessionId, state, message | 状态变更 |
| `complete` | chatId, sessionId, usage | 完成 |
| `error` | chatId, sessionId, code, message, recoverable | 错误 |

---

## 七、项目结构

```
devrix/
├── cmd/
│   └── devrix/
│       └── main.go                    # Entry point
│
├── internal/
│   ├── layers/
│   │   └── communication/
│   │       ├── gateway/
│   │       │   ├── gateway.go         # CommunicationGateway
│   │       │   ├── api.go             # GatewayAPI 接口
│   │       │   ├── store.go          # FileSessionStore
│   │       │   ├── permission.go     # PermissionManager
│   │       │   ├── engine_stub.go    # StubContextEngine
│   │       │   ├── ack.go            # ACK 处理
│   │       │   ├── validate.go       # InputValidator
│   │       │   ├── mock_gateway.go   # MockGatewayAPI (测试)
│   │       │   └── gateway_test.go   # Gateway 测试
│   │       ├── adapters/
│   │       │   ├── cli.go            # CLI Adapter
│   │       │   ├── feishu.go         # Feishu Adapter
│   │       │   ├── feishu_api.go     # FeishuAPI 接口
│   │       │   ├── mock_feishu.go    # MockFeishuAPI (测试)
│   │       │   └── feishu_test.go     # Feishu 测试
│   │       ├── auth/
│   │       │   ├── service.go        # AuthService
│   │       │   └── middleware.go     # AuthMiddleware
│   │       ├── commands/
│   │       │   ├── help.go           # /help command
│   │       │   ├── new.go            # /new command
│   │       │   └── stop.go           # /stop command
│   │       ├── connection/
│   │       │   └── manager.go        # ConnectionManager
│   │       ├── instance/
│   │       │   └── registry.go       # InstanceRegistry
│   │       ├── metrics/
│   │       │   └── collector.go       # MetricsCollector
│   │       ├── milestone/
│   │       │   ├── service.go        # MilestoneService
│   │       │   ├── taskflow.go        # TaskFlow
│   │       │   └── service_test.go    # Milestone 测试
│   │       ├── ratelimit/
│   │       │   ├── limiter.go         # RateLimiter
│   │       │   └── limiter_test.go    # RateLimiter 测试
│   │       ├── renderers/
│   │       │   ├── message.go        # Message Renderer
│   │       │   ├── status.go         # Status Renderer
│   │       │   ├── permission.go     # Permission Renderer
│   │       │   └── components.go     # UI Components
│   │       ├── shared/
│   │       │   ├── config/           # 配置
│   │       │   ├── errors/           # 错误定义
│   │       │   └── types/            # 共享类型
│   │       └── task_flow.go           # TaskFlow
│   └── shared/
│       ├── config/                     # 用户配置
│       ├── errors/                     # 通用错误
│       └── types/                      # 通用类型
│
├── pkg/
│   └── i18n/
│       └── i18n.go                    # Internationalization
│
└── openspec/
    ├── specs/                          # 架构设计规格
    └── changes/                       # 变更记录
        ├── devrix-foundation/          # 基础架构
        ├── devrix-v2/                 # V2 变更
        └── devrix-v3/                 # V3 变更
```

### 7.1 测试覆盖率

| 模块 | 覆盖率 | 备注 |
|------|--------|------|
| FeishuAdapter | 24.8% | 接口抽象 + Mock |
| Gateway | 53.8% | RouteInbound 核心路径 |
| Ratelimit | **95.2%** | Token Bucket 实现 |
| Milestone | 42.4% | 服务层测试 |
| **总计** | ~30% | 持续提升中 |

### 7.2 核心模块设计

#### CommunicationGateway

```go
// internal/layers/communication/gateway/gateway.go

// EventHandler 定义网关事件处理接口
type EventHandler interface {
    OnMessage(msg *OutboundMessage)
    OnPermissionRequest(req *PermissionRequest) bool
    OnError(err error, sessionID string)
    OnStatus(sessionID string, state SessionState)
}

// IContextEngine 定义上下文引擎接口
type IContextEngine interface {
    Process(ctx context.Context, session *Session, message string) <-chan *EngineEvent
}

// GatewayAPI 定义网关核心接口（用于测试 Mock）
type GatewayAPI interface {
    GetSession(sessionID string) (*Session, error)
    CreateSession(chatID, workDir string) (*Session, error)
    RouteInbound(ctx context.Context, msg *InboundMessage) error
    RouteOutbound(msg *OutboundMessage) error
}

type CommunicationGateway struct {
    sessionStore     SessionStore
    eventHandler    EventHandler
    contextEngine   IContextEngine
    permissionMgr   *PermissionManager
    config          *CommunicationConfig
}

func NewCommunicationGateway(
    sessionStore SessionStore,
    eventHandler EventHandler,
    contextEngine IContextEngine,
    permissionMgr *PermissionManager,
    cfg *CommunicationConfig,
) *CommunicationGateway

// RouteInbound 处理入站消息
func (g *CommunicationGateway) RouteInbound(ctx context.Context, msg *InboundMessage) error

// RouteOutbound 发送出站消息
func (g *CommunicationGateway) RouteOutbound(msg *OutboundMessage) error

// Session management
func (g *CommunicationGateway) CreateSession(chatID, workDir string) (*Session, error)
func (g *CommunicationGateway) GetSession(sessionID string) (*Session, error)
func (g *CommunicationGateway) ExpireSession(sessionID string) error
```

#### FileSessionStore

```go
// internal/layers/communication/gateway/store.go

type SessionStore interface {
    Create(session *Session) error
    Get(sessionID string) (*Session, error)
    Update(session *Session) error
    Delete(sessionID string) error
    List() ([]*Session, error)
    GetIdleSessions(timeout time.Duration) ([]*Session, error)
}

type FileSessionStore struct {
    dir string  // ~/.devrix/sessions
}

func NewFileSessionStore(dir string) (*FileSessionStore, error)
```

#### CLIAdapter

```go
// internal/layers/communication/adapters/cli.go

type CLIAdapter struct {
    gateway   GatewayAPI
    renderer  *CLIRenderer
    reader    *bufio.Reader
    config    *config.CommunicationConfig
}

func NewCLIAdapter(gateway GatewayAPI, cfg *config.CommunicationConfig) *CLIAdapter

func (a *CLIAdapter) Start(ctx context.Context) error {
    // 启动交互式 CLI
    // 读取用户输入 → 发送到 Gateway → 接收响应 → 渲染输出
}
```

#### FeishuAdapter

```go
// internal/layers/communication/adapters/feishu.go

// FeishuAPI 定义飞书 API 操作接口（用于测试 Mock）
type FeishuAPI interface {
    Get(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error)
    Im() ImAPI
}

// FeishuAdapterOption 功能选项
type FeishuAdapterOption func(*FeishuAdapter)

func WithFeishuAPI(api FeishuAPI) FeishuAdapterOption
func WithGateway(gw GatewayAPI) FeishuAdapterOption

type FeishuAdapter struct {
    gateway      GatewayAPI
    cfg         *config.CommunicationConfig
    feishuCfg   *FeishuConfig
    eventHandler EventHandler
    api         FeishuAPI
    // ...
}

func NewFeishuAdapter(
    gw *gateway.CommunicationGateway,
    feishuCfg *FeishuConfig,
    cfg *config.CommunicationConfig,
    opts ...FeishuAdapterOption,
) *FeishuAdapter

func (a *FeishuAdapter) Start(ctx context.Context) error
func (a *FeishuAdapter) Stop() error
func (a *FeishuAdapter) SendMessage(ctx context.Context, chatID, content string) error
func (a *FeishuAdapter) OnMessage(msg *OutboundMessage)
```

#### RateLimiter

```go
// internal/layers/communication/ratelimit/limiter.go

type RateLimiter struct {
    tokens     map[string]*bucket
    maxTokens  float64
    rate       float64 // tokens per second
}

type RateLimitConfig struct {
    RequestsPerMinute int
    BurstSize         int
    Enabled           bool
}

func NewRateLimiter(cfg *RateLimitConfig) *RateLimiter
func (l *RateLimiter) Allow(adapterID string) bool
func (l *RateLimiter) Remaining(adapterID string) int
func (l *RateLimiter) Reset(adapterID string)
func (l *RateLimiter) ResetAll()
```

---

## 八、实施计划

### V1（核心链路）- ✅ 已完成

| 阶段 | 内容 | 状态 | 验收标准 |
|-----|------|--------|---------|
| 1 | Gateway 核心 + SessionStore + Engine 接口 | ✅ 完成 | 类型检查通过 |
| 2 | CLI Adapter（本地测试） | ✅ 完成 | `devrix` 可用 |
| 3 | 心跳 + 重连 + 消息确认 | ✅ 完成 | 断连自动恢复 |
| 4 | 权限管理 + 超时拒绝 | ✅ 完成 | 60s 超时生效 |
| 5 | 信息流（帮助/错误/状态） | ✅ 完成 | /help 可用 |
| 10 | Milestone 任务流 | ✅ 完成 | DAG + 进度追踪 |
| 13 | 限流设计 | ✅ 完成 | Token Bucket 实现 |

### V2（可靠性增强）- 🔄 进行中

| 阶段 | 内容 | 状态 | 验收标准 |
|-----|------|--------|---------|
| 6 | ShortId 生成（5位防脏话） | ✅ 完成 | requestId 可读性提升 |
| 7 | Auth + 输入验证 | ✅ 完成 | Token 认证 |
| 8 | 领域事件完整定义 | 🔄 进行中 | 11 个事件完整 |
| 9 | 飞书 Adapter | ✅ 完成 | 飞书 WebSocket 集成 |

### V3（功能完善）- ✅ 已完成（部分）

| 阶段 | 内容 | 状态 | 验收标准 |
|-----|------|--------|---------|
| 10 | Milestone 任务流 | ✅ 完成 | DAG + 进度追踪 |
| 11 | UI 组件体系 | 🔄 进行中 | 跨平台一致性 |
| 12 | 钉钉 Adapter | ❌ 未开始 | — |
| 13 | 限流设计 | ✅ 完成 | Token Bucket 实现 |
| 14 | 多实例部署 + 监控 | 🔄 进行中 | Prometheus 接入 |

> **注意**：Milestone 模块已实现，但 DAG 实例**不可跨 Service 共享**，每个 MilestoneService 应拥有独立的 DAG 实例以避免数据竞争。

---

## 九、完整对照表

### 与 dev-brain v3.0 功能对照

| 功能 | dev-brain v3.0 | devrix v1.0 | devrix v2.0 | devrix v3.0 | 说明 |
|------|----------------|-------------|-------------|-------------|------|
| **领域模型** ||||||
| Session | ✅ | ✅ | ✅ | ✅ | 聚合根 |
| Message | ✅ | ✅ | ✅ | ✅ | 消息实体 |
| PermissionRequest | ✅ | ✅ | ✅ | ✅ | 权限请求 |
| ShortId | ✅ | ❌ | ✅ | ✅ | 5位展示ID |
| Milestone | ✅ | ❌ | ❌ | ✅ | 任务里程碑 |
| **四流设计** ||||||
| 指令流 | ✅ | ✅ | ✅ | ✅ | 用户命令 |
| 事件流 | ✅ | ✅ | ✅ | ✅ | 状态推送 |
| 任务流 | ✅ | ✅ | ✅ | ✅ | Milestone DAG |
| 信息流 | ✅ | ✅ | ✅ | ✅ | 帮助/错误 |
| **领域事件** ||||||
| session.created | ✅ | ✅ | ✅ | ✅ | |
| session.expired | ✅ | ✅ | ✅ | ✅ | |
| message.received | ✅ | ✅ | ✅ | ✅ | |
| message.sent | ✅ | ✅ | ✅ | ✅ | |
| permission.requested | ✅ | ✅ | ✅ | ✅ | |
| permission.responded | ✅ | ⚠️ 简化 | ✅ | ✅ | V1 合并到请求 |
| permission.expired | ✅ | ⚠️ 简化 | ✅ | ✅ | V1 内部处理 |
| tool.called | ✅ | ❌ | ❌ | ✅ | |
| connection.lost | ✅ | ❌ | ✅ | ✅ | |
| connection.restored | ✅ | ❌ | ✅ | ✅ | |
| milestone.created | ✅ | ✅ | ✅ | ✅ | |
| milestone.updated | ✅ | ✅ | ✅ | ✅ | |
| **接口设计** ||||||
| Adapter 注册 | ✅ | ✅ | ✅ | ✅ | |
| IM 消息收发 | ✅ | ✅ | ✅ | ✅ | |
| 权限请求/回复 | ✅ | ✅ | ✅ | ✅ | |
| 流式响应 | ✅ | ✅ | ✅ | ✅ | |
| 错误码分层 | ✅ | ✅ | ✅ | ✅ | |
| 限流设计 | ✅ | ❌ | ✅ | ✅ | Token Bucket |
| 版本管理 | ✅ | ❌ | ✅ | ✅ | |
| **部署运维** ||||||
| 部署架构图 | ✅ | ❌ | ⚠️ 简化 | ✅ | |
| 监控指标 | ✅ | ❌ | ❌ | 🔄 进行中 | |
| 日志规范 | ✅ | ⚠️ 简化 | ✅ | ✅ | |
| 多实例部署 | ✅ | ❌ | ❌ | 🔄 进行中 | |
| **UI 设计** ||||||
| 组件体系 | ✅ | ❌ | 🔄 进行中 | ✅ | |
| 状态机 | ✅ | ❌ | ❌ | ✅ | |
| 平台策略 | ✅ | ❌ | ❌ | ✅ | |

**图例**：✅ 完整实现 | ⚠️ 简化实现 | ❌ 未实现 | 🔄 进行中

---

## 十、配置

```go
// internal/shared/config/communication.go

type CommunicationConfig struct {
    Session    SessionConfig
    Permission PermissionConfig
    CLI        CLIConfig
    Commands   CommandsConfig
}

type SessionConfig struct {
    IdleTimeout time.Duration // 30 分钟空闲超时
    StorageDir  string       // ~/.devrix/sessions
    MaxSessions int          // 最大并发会话数
}

type PermissionConfig struct {
    DefaultTimeout time.Duration // 60 秒权限超时
    MaxRetries     int          // 最大重试次数
}

type CLIConfig struct {
    WelcomeMessage string
    Prompt         string
    ANSI           ANSIConfig
}

type ANSIConfig struct {
    User      string // 蓝色
    Assistant string // 绿色
    Error     string // 红色
    Warning   string // 黄色
    Reset     string
}

type CommandsConfig struct {
    Prefix string
    List   []string // ["new", "stop", "help"]
}

// DefaultConfig 返回默认配置
func DefaultConfig() *CommunicationConfig {
    home, _ := os.UserHomeDir()
    return &CommunicationConfig{
        Session: SessionConfig{
            IdleTimeout: 30 * time.Minute,
            StorageDir:  filepath.Join(home, ".devrix", "sessions"),
            MaxSessions: 1000,
        },
        Permission: PermissionConfig{
            DefaultTimeout: 60 * time.Second,
            MaxRetries:     3,
        },
        CLI: CLIConfig{
            WelcomeMessage: `
╔═══════════════════════════════════════════════════════════════╗
║                    Devrix v1.0 - 开发大脑                    ║
║            多智能协同开发助手 (Multi-Agent CLI)               ║
╚═══════════════════════════════════════════════════════════════╝`,
            Prompt: "> ",
            ANSI: ANSIConfig{
                User:      "\x1b[34m",
                Assistant: "\x1b[32m",
                Error:     "\x1b[31m",
                Warning:   "\x1b[33m",
                Reset:     "\x1b[0m",
            },
        },
        Commands: CommandsConfig{
            Prefix: "/",
            List:   []string{"new", "stop", "help"},
        },
    }
}
```

---

## 十一、错误处理

```go
// internal/shared/errors/communication.go

import "errors"

// Sentinel errors (可使用 errors.Is/ errors.As 检查)
var (
    // 会话错误 (1000-1999)
    ErrSessionNotFound    = errors.New("session not found")
    ErrSessionExpired     = errors.New("session expired")
    ErrSessionCreateFailed = errors.New("failed to create session")
    ErrSessionStore       = errors.New("session store error")

    // 消息错误 (2000-2999)
    ErrMessageEmpty         = errors.New("message is empty")
    ErrMessageTooLong        = errors.New("message too long")
    ErrMessageInvalidFormat = errors.New("invalid message format")

    // 权限错误 (3000-3999)
    ErrPermissionDenied  = errors.New("permission denied")
    ErrPermissionTimeout = errors.New("permission timeout")
    ErrPermissionInvalid = errors.New("invalid permission response")

    // 网关错误 (4000-4999)
    ErrGatewayRoute  = errors.New("gateway route error")
    ErrGatewayAdapt = errors.New("gateway adapter error")
)

// SentinelError 包装错误码和消息
type SentinelError struct {
    Code    string
    Message string
    Err     error
}

func (e *SentinelError) Error() string {
    return e.Message
}

func (e *SentinelError) Unwrap() error {
    return e.Err
}

// ErrorCode 返回错误码
func ErrorCode(err error) string {
    var se *SentinelError
    if errors.As(err, &se) {
        return se.Code
    }
    return ""
}
```

### 错误码对照表

| 错误码 | Go 错误 | 说明 |
|---------|---------|------|
| COMM_SESSION_NOT_FOUND | ErrSessionNotFound | Session 不存在 |
| COMM_SESSION_EXPIRED | ErrSessionExpired | Session 已过期 |
| COMM_SESSION_CREATE_FAILED | ErrSessionCreateFailed | 创建 Session 失败 |
| COMM_SESSION_STORE_ERROR | ErrSessionStore | Session 存储错误 |
| COMM_MESSAGE_EMPTY | ErrMessageEmpty | 消息为空 |
| COMM_MESSAGE_TOO_LONG | ErrMessageTooLong | 消息超长 |
| COMM_MESSAGE_INVALID_FORMAT | ErrMessageInvalidFormat | 消息格式错误 |
| COMM_PERMISSION_DENIED | ErrPermissionDenied | 权限被拒绝 |
| COMM_PERMISSION_TIMEOUT | ErrPermissionTimeout | 权限请求超时 |
| COMM_PERMISSION_INVALID_RESPONSE | ErrPermissionInvalid | 无效的权限响应 |
| COMM_GATEWAY_ROUTE_FAILED | ErrGatewayRoute | 网关路由错误 |
| COMM_GATEWAY_ADAPTER_ERROR | ErrGatewayAdapt | 网关适配器错误 |
