# Feishu 用户交互动线 (修正版)

**基于 cc-connect 源码分析**

---

## 1. OK/Done 双状态机制

这是 cc-connect 最重要的用户体验设计：**即时确认 + 完成通知**

### 1.1 状态定义

| 状态 | 含义 | 触发时机 | 表现形式 |
|------|------|----------|----------|
| **OK** | 收到指令，agent 开始处理 | 飞书消息到达，**立即返回** | 简短文字消息 |
| **Done** | 处理完成 | agent 响应发送完毕后 | 在用户消息上添加 `done_emoji` |

### 1.2 设计价值

| 问题 | 没有 OK | 有 OK |
|------|---------|-------|
| 用户等待时 | 不确定网络是否正常，可能重试或多次发送 | 立即知道"agent 正在处理" |
| 等待焦虑 | 长时间等待不确定是否卡住 | 耐心等待 |
| 网络问题 | 用户不知道消息是否送达 | 立即知道 |

### 1.3 done_emoji 配置

```toml
# config.toml
[projects.platforms.options]
done_emoji = "Done"  # 完成时添加 "Done" 表情，设为 "none" 可禁用
```

**作用**：飞书卡片原地更新不触发推送，done 表情可以通知用户 agent 已完成。

### 1.2 progress_style 三种模式

| 模式 | 说明 | 特点 |
|------|------|------|
| `legacy` | 逐条发送思考/工具进度 | 消息多、容易刷屏 |
| `compact` | 合并到一条可更新消息 | 减少刷屏 |
| `card` | 结构化卡片持续更新 | 观感最清晰 |

### 1.3 RichCard 结构

cc-connect 使用 `BuildRichCard` 构建包含：
- **Header**: 标题 + 颜色
- **ToolSteps**: 思考/工具进度列表
- **Markdown**: 主文本内容
- **StatusFooter**: 状态行 (用时、token、模型信息)

---

## 2. 正确的用户交互流程

```
用户发送消息 ─────────────────────────────────────────────────────▶
                                                              │
                                                              ▼
                                                   ┌─────────────────────────┐
                                                   │ 👍 OK                   │
                                                   │ (收到指令，开始处理)   │
                                                   └─────────────────────────┘
                                                              │
                                           ┌───────────────────┼───────────────────┐
                                           │                   │                   │
                                           ▼                   ▼                   ▼
                                  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐
                                  │ 📊 进度 20%   │  │ 📊 进度 40%   │  │ 📊 进度 100%  │
                                  │ 🟣 (milestone)│  │ 🟣 (milestone)│  │ 🟣 (milestone)│
                                  └────────────────┘  └────────────────┘  └────────────────┘
                                                              │
                                                              ▼
                                                   ┌─────────────────────────┐
                                                   │ 📝 Devrix 响应         │
                                                   │ (流式输出中...)        │
                                                   └─────────────────────────┘
                                                              │
                                                              ▼
                                                   ┌─────────────────────────┐
                                                   │ 🟢 ✅ 完成!            │
                                                   │ 用时: 4.6s | 消耗: 127 │
                                                   └─────────────────────────┘
                                                              │
                                                              ▼
                                                   ┌─────────────────────────┐
                                                   │ 😄 [Done] 表情反应     │
                                                   │ (在用户消息上添加)     │
                                                   └─────────────────────────┘
```

### 2.1 状态说明

| 步骤 | 状态 | 卡片颜色 | 内容 |
|------|------|----------|------|
| 1 | OK | 无卡片 | 简短文字 "OK" 或 "👍" |
| 2 | 进度 | 🟣 紫色 | 任务里程碑进度更新 |
| 3 | 响应 | 🔵 蓝色 | 流式输出响应内容 |
| 4 | 完成 | 🟢 绿色 | 完成摘要 + 统计 |
| 5 | Done | 表情 | 在用户消息上添加 `done_emoji` |

---

## 3. cc-connect 消息卡片格式

### 3.1 完成卡片 (Done Card)

```
┌────────────────────────────────────────────────────┐
│ 🟢 ✅ 完成!                                 [绿色] │
├────────────────────────────────────────────────────┤
│                                                    │
│  📝 Devrix 响应                                    │
│  ────────────────────────                          │
│  [完整响应内容]                                     │
│                                                    │
│  ────────────────────────                          │
│  ⏱ 用时: 4.6s                                     │
│  🔢 消耗: 127 tokens                               │
│  💾 上下文: ~60%                                   │
│                                                    │
└────────────────────────────────────────────────────┘
```

### 3.2 进度卡片 (Progress Card)

```
┌────────────────────────────────────────────────────┐
│ 🟣 📊 任务进度                              [紫色] │
├────────────────────────────────────────────────────┤
│                                                    │
│  🔵 Thinking...                                   │
│  ════════════════════════                         │
│                                                    │
│  🔧 Read (reading file)                          │
│  ░░░░░░░░░░░░░░░░░░░░░░░░░░░  50%             │
│                                                    │
└────────────────────────────────────────────────────┘
```

---

## 4. devrix 需要对齐的设计

### 4.1 卡片结构

| 组件 | cc-connect | devrix 现状 | 需要改进 |
|------|-----------|-------------|----------|
| Header | ✅ Title + Color | ✅ 已支持 | 对齐颜色 |
| ToolSteps | ✅ thinking/tool 列表 | ❌ 无 | 新增 |
| Markdown Body | ✅ 支持 | ✅ 已支持 | - |
| StatusFooter | ✅ 用时/token/模型 | ⚠️ 简化 | 增强 |
| Done Emoji | ✅ 支持 | ❌ 无 | 新增 |

### 4.2 事件类型对齐

| cc-connect Event | Content 示例 | devrix 对应 |
|------------------|--------------|-------------|
| EventThinking | "🤔 思考中..." | `thinking` |
| EventToolUse | "🔧 调用工具: read" | `tool_call` |
| EventToolResult | "✅ [Tool: read]\n```\n{result}\n```" | `tool_result` |
| EventResult | 完整响应 + Done | `text` + `complete` |
| milestone_progress | "📊 任务进度: 50%" | `milestone_progress` |

---

## 5. 需要确认的问题

**截图中的 "Dove Claude Code" 消息：**

根据 cc-connect 代码分析：
- cc-connect **不会**自动发送 "Done Claude Code" 消息
- `done_emoji` 只是在用户消息上添加表情反应
- "Dove Claude Code" 可能是：
  1. 用户发送的测试命令
  2. 或者是截图中被截断的其他内容

**请确认这张截图的完整上下文？**

---

## 6. 参考 cc-connect 实现

### 6.1 关键文件

| 文件 | 功能 |
|------|------|
| `core/streaming.go` | 流式预览、RichCardSupporter 接口 |
| `core/progress_compact.go` | 进度卡片 Payload 构建 |
| `platform/feishu/card.go` | Feishu 卡片渲染 |
| `platform/feishu/feishu.go` | done_emoji、BuildRichCard 实现 |

### 6.2 核心接口

```go
// RichCardSupporter 接口
type RichCardSupporter interface {
    BuildRichCard(status string, title string, steps []ToolStep, markdown string, streaming bool, footer string) string
}

// ToolStep 结构
type ToolStep struct {
    Kind    ToolStepKind  // thinking / tool
    Name    string         // 工具名称
    Summary string         // 摘要
    Status  string         // 状态
    Done    bool           // 是否完成
}
```

---

## 7. 下一步行动

1. **确认截图消息**：请提供 "Dove Claude Code" 的完整上下文
2. **确定 progress_style**：devrix 应该支持哪种模式 (legacy/compact/card)
3. **确定 done_emoji**：是否需要在 devrix 中实现这个机制
