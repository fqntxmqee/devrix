# Feishu Card Adapter Redesign

**Change ID:** `devrix-feishu-cards`
**Type:** Architecture Redesign
**Status:** Delivered
**Version:** 1.0
**Based on:** cc-connect feishu platform + devrix-foundation communication layer

---

## 1. Motivation

当前 devrix 的飞书适配存在以下问题：

| 问题 | 影响 | 严重度 |
|------|------|--------|
| **无即时 OK 确认** | 用户不知道消息是否送达，agent 处理时间长时焦虑 | High |
| 无 done_emoji 完成通知 | 用户不知道 agent 是否完成 | Medium |
| Card 元素类型有限 | 无法支持 ListItem、Select 等复杂交互 | Medium |
| 无 RenderText 回退 | 不支持卡片的平台会失效 | High |
| 进度系统与 cc-connect 不对齐 | 事件类型不一致，无法互通 | Medium |
| 颜色支持不完整 | cc-connect 支持 12 种颜色，devrix 只有 7 种 | Low |

---

## 2. Design Goals

### 2.1 功能目标

| 目标 | 描述 | 优先级 |
|------|------|--------|
| **即时 OK 确认** | 收到飞书消息后立即返回 OK，告知用户网络正常 | P0 |
| **完成 done_emoji** | agent 完成后在用户消息上添加表情反应 | P0 |
| 统一 Card 模型 | 与 cc-connect core.Card 兼容 | P0 |
| 完整元素支持 | CardMarkdown, CardDivider, CardActions, CardListItem, CardSelect, CardNote | P0 |
| 平台回退机制 | 不支持卡片的平台自动降级为纯文本 | P0 |
| 进度事件对齐 | ProgressEntry 类型与 cc-connect 一致 | P1 |
| 流式预览支持 | RichCardTextStreamer 接口实现打字机效果 | P2 |

### 2.2 兼容性目标

| 方面 | 目标 |
|------|------|
| cc-connect 兼容 | Card 结构、Element 类型、渲染逻辑与 cc-connect 兼容 |
| 向后兼容 | 现有 four_flows_test.go 测试继续通过 |
| API 稳定 | FeishuAdapter 公共接口不变 |

### 2.3 设计决策汇总

| 决策 | 选择 | 理由 |
|------|------|------|
| OK 返回值 | "👍" (emoji) | 简洁友好 |
| OK 发送方式 | 独立消息 (Send) | 不产生引用链，快速 |
| done_emoji 默认 | 启用 | 用户体验更好 |
| done_emoji 默认值 | "✅" | 中文用户友好 |
| done_emoji 展示 | Reaction 在用户消息上 | 最轻量，不产生新消息 |
| 进度卡片样式 | 更新同一张卡片 | 减少刷屏，提供实时反馈 |
| 进度卡片状态值 | running，紫色 | 与 cc-connect 一致 |
| 响应卡片标题 | "Devrix 响应" | 品牌露出 |
| 完成卡片标题 | "✅ 完成!" | 简洁明确 |
| Card 元素优先级 | Markdown → Divider → Actions → Note → ListItem → Select | 按复杂度从低到高 |
| RenderText 回退 | 保留 Markdown 代码块，其他转文本 | 保持代码可读性 |
| 颜色实现 | 先 7 种（蓝绿红橙紫灰蓝），后续扩展 | 快速验证核心功能 |

---

## 3. Architecture

### 3.1 模块位置

```
internal/layers/communication/
├── adapters/
│   ├── feishu.go           # FeishuAdapter 实现
│   ├── feishu_card.go      # 飞书卡片渲染器 (新增/重写)
│   ├── feishu_api.go       # FeishuAPI 接口
│   └── mock_feishu.go      # Mock 实现
└── core/                   # 共享类型
    └── card.go             # 统一 Card 模型 (新增)
```

### 3.2 核心接口

```go
// ============================================================
// 核心类型定义 (internal/layers/communication/core/card.go)
// ============================================================

// CardElement 是所有卡片元素的公共接口
type CardElement interface {
    cardElement()
}

// CardMarkdown 渲染 markdown 格式文本
type CardMarkdown struct{ Content string }

// CardDivider 渲染水平分隔线
type CardDivider struct{}

// CardActions 渲染按钮行
type CardActions struct {
    Buttons []CardButton
    Layout  CardActionLayout
}

// CardButton 按钮
type CardButton struct {
    Text  string
    Type  string // "primary", "default", "danger"
    Value string // 回调数据
    Extra map[string]string
}

// CardListItem 左侧文字 + 右侧按钮的布局
type CardListItem struct {
    Text     string
    BtnText  string
    BtnType  string
    BtnValue string
    Extra    map[string]string
}

// CardSelect 下拉选择器
type CardSelect struct {
    Placeholder string
    Options    []CardSelectOption
    InitValue  string
}

// CardSelectOption 选择项
type CardSelectOption struct {
    Text  string
    Value string
}

// CardNote 脚注文本
type CardNote struct {
    Text string
    Tag  string // 可选标签，用于程序识别
}

// CardActionLayout 按钮布局模式
type CardActionLayout string

const (
    CardActionLayoutRow          CardActionLayout = "row"
    CardActionLayoutEqualColumns CardActionLayout = "equal_columns"
)
```

### 3.3 CardBuilder API

```go
// CardBuilder 链式构建器
type CardBuilder struct {
    card Card
}

func NewCard() *CardBuilder

// Header 设置
func (b *CardBuilder) Title(title, color string) *CardBuilder

// Elements
func (b *CardBuilder) Markdown(content string) *CardBuilder
func (b *CardBuilder) Markdownf(format string, args ...any) *CardBuilder
func (b *CardBuilder) Divider() *CardBuilder
func (b *CardBuilder) Buttons(buttons ...CardButton) *CardBuilder
func (b *CardBuilder) ButtonsEqual(buttons ...CardButton) *CardBuilder
func (b *CardBuilder) ListItem(desc, btnText, btnValue string) *CardBuilder
func (b *CardBuilder) ListItemBtn(desc, btnText, btnType, btnValue string) *CardBuilder
func (b *CardBuilder) ListItemBtnExtra(desc, btnText, btnType, btnValue string, extra map[string]string) *CardBuilder
func (b *CardBuilder) Select(placeholder string, options []CardSelectOption, initValue string) *CardBuilder
func (b *CardBuilder) Note(text string) *CardBuilder
func (b *CardBuilder) TaggedNote(tag, text string) *CardBuilder

func (b *CardBuilder) Build() *Card
```

### 3.4 RenderText 回退机制

```go
// Card.RenderText 将卡片转换为纯文本，用于不支持卡片的平台
func (c *Card) RenderText() string

// 示例输出:
// **标题**
//
// 内容
//
// [按钮1]  [按钮2]
//
// 脚注
```

---

## 4. Feishu Card Renderer

### 4.1 卡片颜色支持

| 颜色 | cc-connect | devrix (现状) | 差异 |
|------|-----------|---------------|------|
| blue | ✅ | ✅ | 一致 |
| green | ✅ | ✅ | 一致 |
| red | ✅ | ✅ | 一致 |
| orange | ✅ | ✅ | 一致 |
| purple | ✅ | ✅ | 一致 |
| grey | ✅ | ✅ | 一致 |
| turquoise | ✅ | ❌ | 需新增 |
| violet | ✅ | ❌ | 需新增 |
| indigo | ✅ | ❌ | 需新增 |
| wathet | ✅ | ❌ | 需新增 |
| yellow | ✅ | ❌ | 需新增 |
| carmine | ✅ | ❌ | 需新增 |

### 4.2 元素渲染映射

| devrix Element | Feishu Tag | 备注 |
|----------------|-----------|------|
| CardMarkdown | `markdown` | 直接渲染 |
| CardDivider | `hr` | 水平线 |
| CardActions (row) | `action` + `actions` | 按钮数组 |
| CardActions (equal_columns) | `column_set` + `column` | 等宽列布局 |
| CardListItem | `column_set` | 左侧文字 + 右侧按钮 |
| CardSelect | `action` + `select_static` | 下拉选择 |
| CardNote | `note` | 脚注 |

### 4.3 卡片渲染函数

```go
// RenderCard 将 Card 结构渲染为 Feishu Interactive Card JSON
func RenderCard(card *Card, sessionKey string) string

// RenderCardMap 返回 map 结构，便于测试
func RenderCardMap(card *Card, sessionKey string) map[string]any
```

---

## 5. 进度事件对齐

### 5.1 ProgressEntry 类型

参考 cc-connect `core/progress_compact.go`：

```go
// ProgressEntryKind 进度条目类型
type ProgressEntryKind string

const (
    ProgressEntryThinking   ProgressEntryKind = "thinking"
    ProgressEntryToolUse   ProgressEntryKind = "tool_use"
    ProgressEntryToolResult ProgressEntryKind = "tool_result"
    ProgressEntryError     ProgressEntryKind = "error"
    ProgressEntryInfo      ProgressEntryKind = "info"
)

// ProgressEntry 进度条目
type ProgressEntry struct {
    Kind     ProgressEntryKind
    Text     string
    Tool     string // 工具名称
    Status   string
    ExitCode *int
    Success  *bool
}
```

### 5.2 事件类型映射

| EngineEvent.Type | ProgressEntryKind | 卡片标题颜色 |
|------------------|-------------------|--------------|
| thinking | ProgressEntryThinking | blue |
| tool_call | ProgressEntryToolUse | orange |
| tool_result | ProgressEntryToolResult | green |
| error | ProgressEntryError | red |
| info | ProgressEntryInfo | blue |
| milestone_progress | ProgressEntryInfo | purple |

---

## 6. 接口变更

### 6.1 新增接口

```go
// FeishuAdapter 新增方法

// SendImmediateACK 立即发送 ACK 确认收到消息（不等待 agent 处理）
// 这是用户体验的关键：让用户知道网络正常，agent 正在处理
func (a *FeishuAdapter) SendImmediateACK(ctx context.Context, chatID, messageID string) error

// AddReaction 在用户消息上添加完成表情（done_emoji 机制）
func (a *FeishuAdapter) AddReaction(ctx context.Context, messageID, emoji string) error
```

### 6.2 OK/Done 流程

```go
// FeishuAdapter 消息处理流程
func (a *FeishuAdapter) HandleMessage(ctx context.Context, msg *InboundMessage) error {
    // 1. 立即发送 OK 确认（不阻塞）
    go func() {
        a.SendImmediateACK(ctx, msg.ChatID, msg.MessageID)
    }()

    // 2. 异步转发给 Gateway 处理（可能很慢）
    go func() {
        a.gateway.RouteInbound(ctx, msg)
    }()

    return nil
}

// agent 处理完成后添加 done_emoji
func (a *FeishuAdapter) OnAgentComplete(ctx context.Context, sessionID string) {
    // 在用户消息上添加完成表情
    if a.doneEmoji != "" {
        a.AddReaction(ctx, a.userMessageID, a.doneEmoji)
    }
}
```

### 6.2 FeishuAdapter 变更

```go
// 新增方法
func (a *FeishuAdapter) RefreshCard(ctx context.Context, sessionKey string, card *Card) error
func (a *FeishuAdapter) BuildRichCard(status, title string, steps []ToolStep, markdown string, streaming bool, footer string) string
```

---

## 7. 文件变更清单

| 操作 | 文件 | 说明 |
|------|------|------|
| 新增 | `internal/layers/communication/core/card.go` | 统一 Card 模型 |
| 重写 | `internal/layers/communication/adapters/feishu_card.go` | 完整卡片渲染 |
| 修改 | `internal/layers/communication/adapters/feishu.go` | 适配新接口 |
| 新增 | `internal/layers/communication/core/progress.go` | 进度类型定义 |
| 新增 | `internal/layers/communication/adapters/feishu_card_test.go` | 卡片测试 |

---

## 8. 测试策略

### 8.1 单元测试

```go
func TestRenderCard_Basic(t *testing.T)       // 基础卡片渲染
func TestRenderCard_WithHeader(t *testing.T)   // 带标题头
func TestRenderCard_Markdown(t *testing.T)     // Markdown 内容
func TestRenderCard_Divider(t *testing.T)      // 分隔线
func TestRenderCard_Actions(t *testing.T)      // 按钮
func TestRenderCard_ListItem(t *testing.T)     // 列表项
func TestRenderCard_Select(t *testing.T)       // 下拉选择
func TestRenderCard_Note(t *testing.T)         // 脚注
func TestRenderCard_Colors(t *testing.T)       // 所有颜色
func TestCard_RenderText(t *testing.T)          // 文本回退
```

### 8.2 集成测试

```go
func TestFeishuCard_E2E(t *testing.T)           // 端到端渲染测试
func TestFourFlow_Cards(t *testing.T)          // 四流卡片验证
```

---

## 9. 实施计划

| 阶段 | 内容 | 工作量 |
|------|------|--------|
| 1 | 创建 `core/card.go` 统一模型 | 0.5d |
| 2 | 实现 OK 即时确认 + done_emoji 机制 | 0.5d |
| 3 | 重写 `feishu_card.go` 完整渲染器 | 1d |
| 4 | 添加 7 种常用颜色支持 | 0.25d |
| 5 | 实现 `RenderText` 回退 | 0.25d |
| 6 | 创建 `feishu_card_test.go` | 0.5d |
| 7 | 更新 `feishu.go` 适配新接口 | 0.5d |
| 8 | 验证 four_flows_test.go 通过 | 0.25d |

**总计**: ~3.75d

---

## 10. 参考实现

- cc-connect `core/card.go` - Card 模型定义
- cc-connect `platform/feishu/card.go` - Feishu 渲染器
- cc-connect `core/progress_compact.go` - 进度类型
- cc-connect `core/streaming.go` - 流式预览接口
