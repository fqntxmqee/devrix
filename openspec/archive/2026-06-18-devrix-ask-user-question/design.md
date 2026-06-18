# S3 设计：ask_user_question ToolSurface

## 1. 组件拓扑

```
┌────────────────────────────────────────────────────────────────────┐
│  LLM Turn (D7)                                                      │
│  ┌──────────────────────────────────────────────────────────┐     │
│  │  turn_adapter.ExecuteRound                                │     │
│  │  → findSurface("ask_user_question") → AskUserQuestionSurf│     │
│  └──────────────────────────────────────────────────────────┘     │
│                              │                                      │
│                              ▼                                      │
│  ┌──────────────────────────────────────────────────────────┐     │
│  │  AskUserQuestionSurface (stateless)                      │     │
│  │   • Tools() → ToolSpec with 4 bool flags                 │     │
│  │   • Execute(ctx, _, input, _) → JSON result              │     │
│  │   • InterruptBehavior() → InterruptCancel               │     │
│  └──────────────────────────────────────────────────────────┘     │
│                              │                                      │
│                              ▼ currentAskSender()                   │
│  ┌──────────────────────────────────────────────────────────┐     │
│  │  globalAskUserQuestionSender (process-global hook)       │     │
│  │   (SetAskUserQuestionSender called once in main.go)      │     │
│  └──────────────────────────────────────────────────────────┘     │
│                              │                                      │
│                              ▼                                      │
│  ┌──────────────────────────────────────────────────────────┐     │
│  │  main.go wiring closure:                                  │     │
│  │  asksurface.SetAskUserQuestionSender(func(ctx, sid, txt) │     │
│  │    → gw.RouteOutbound(&types.OutboundMessage{...}))      │     │
│  └──────────────────────────────────────────────────────────┘     │
│                              │                                      │
│                              ▼                                      │
│  CommunicationGateway.RouteOutbound                                 │
│  → IM adapter (Feishu) → 飞书 chat                                  │
└────────────────────────────────────────────────────────────────────┘
```

## 2. 数据模型

### 2.1 ToolSpec（dm-007 + dm-001 已定型）

```go
contracts.ToolSpec{
    Name: "ask_user_question",
    Description: "Ask the user 1-4 multiple-choice questions ...",
    Parameters: `<JSON schema>`,
    Risk: types.RiskLevelLow,
    ReadOnly: true,         // 不修改状态
    Destructive: false,
    OpenWorld: true,        // 发 IM 消息
    ConcurrencySafe: false, // 长 run，可能等 IM gateway write
}
```

### 2.2 input / output

input (`askUserQuestionInput`):
```go
type Question struct {
    Question    string           `json:"question"`
    Header      string           `json:"header,omitempty"`     // ≤ 12 chars
    Options     []QuestionOption `json:"options"`               // 2-4 entries
    MultiSelect bool             `json:"multi_select,omitempty"`
}
type QuestionOption struct {
    Label       string `json:"label"`         // 1-5 word, unique per question
    Description string `json:"description"`
}
type askUserQuestionInput struct {
    Questions []Question `json:"questions"`    // 1-4 entries
}
```

output (`askUserQuestionOutput`):
```go
type askUserQuestionOutput struct {
    Delivered    bool       `json:"delivered"`        // sender 成功推送为 true
    SentAt       string     `json:"sent_at"`
    QuestionText string     `json:"question_text"`    // 实际推送的纯文本（让 LLM 看到最终用户看到的）
    Questions    []Question `json:"questions"`        // 回显已校验的入参
    Hint         string     `json:"hint"`             // 提示 LLM "下个 user message 里会有回复"
}
```

## 3. IM 渲染格式

`renderQuestionsForIM` 输出结构（devrix-specific）：

```
【Header A】
question text (可多选)
  1. label A — description A
  2. label B — description B
  其他. 直接回复你的想法

【Header B】
question text
  1. label A — description A
  其他. 直接回复你的想法

回复序号 (例如 1) 或选项文字即可。
```

要点：
- `【Header X】` chip 来自 `header` 字段（≤ 12 chars）
- `(可多选)` 仅在 `multi_select=true` 时追加
- `其他. 直接回复你的想法` 在每个问题末尾显式追加（clawcode UI 自动加，IM 通道没有）
- 多问题之间空行分隔，最后追加使用说明

## 4. sender 桥接

```go
// main.go (在 gw 装配后)
asksurface.SetAskUserQuestionSender(func(ctx context.Context, sessionID, text string) error {
    return gw.RouteOutbound(&types.OutboundMessage{
        MessageID: "ask_" + sessionID + "_" + time.Now().UTC().Format("20060102T150405.000"),
        SessionID: sessionID,
        Content:   text,
        Role:      types.MessageRoleAssistant,
        Metadata: map[string]string{
            "source":   "ask_user_question",
            "blocking": "false",
        },
        SentAt: time.Now().UTC(),
    })
})
```

`SetAskUserQuestionSender` 的实现：
```go
var (
    globalAskUserQuestionSenderMu sync.RWMutex
    globalAskUserQuestionSender   AskUserQuestionSender
)
func SetAskUserQuestionSender(s AskUserQuestionSender) {
    globalAskUserQuestionSenderMu.Lock()
    globalAskUserQuestionSender = s
    globalAskUserQuestionSenderMu.Unlock()
}
```

- 全局锁保证 `SetAskUserQuestionSender` / `currentAskSender` 的并发安全（单测 T12 验证 `-race` 干净）
- `nil` sender 时 `Execute` 返回 `Delivered=false`（graceful degradation）

## 5. InterruptCancel 协议

`InterruptBehaviorFor("ask_user_question")` 返回 `contracts.InterruptCancel`。D7 runLoop 在 ctx.Done() 时调用 `surface.Execute` 的 cancel 分支——

当前实现：sender 内部走 `gw.RouteOutbound`，该方法接受 `ctx` 并 select `ctx.Done()`。如果 ctx 已 cancel，sender 收到 error，工具结果返回 `ToolResult.Error`，LLM 看到错误后进入下一 turn 处理用户新消息。

> v1.1 可以做真正的同步 rendezvous（事件总线等用户回复），但本期不做。

## 6. 失败模式

| 场景 | 行为 |
|---|---|
| Sender 未装配 (SetAskUserQuestionSender 没人调) | `Delivered=false` + hint，工具仍 success |
| Sender 调 `RouteOutbound` 返回 error | `ToolResult.Error = "ask_user_question: send failed: ..."` |
| `ctx.Done()` 已 cancel | sender 内部 select cancel，返回 error；同上 |
| input JSON 解析失败 | `ToolResult.Error = "ask_user_question: invalid input JSON: ..."` |
| 0 / > 4 questions | `ToolResult.Error = "ask_user_question: at least 1 / at most 4 questions ..."` |
| 选项 < 2 / > 4 | `ToolResult.Error = "ask_user_question: questions[i].options ..."` |
| label 空 | `ToolResult.Error = "ask_user_question: questions[i].options[j].label is required"` |
| label 重复 | `ToolResult.Error = "ask_user_question: questions[i].options[j].label %q is duplicated"` |
| header > 12 | `ToolResult.Error = "ask_user_question: questions[i].header exceeds 12 chars (got N)"` |

## 7. 测试覆盖（12 单测 / T01-T12）

| ID | 测试名 | 覆盖点 |
|---|---|---|
| T01 | `TestAskUserQuestionSurface_Tools` | spec 字段（4 bool + interrupt + permission） |
| T02 | `TestAskUserQuestionSurface_Execute_EmptyQuestions` | validation: 0 questions |
| T03 | `TestAskUserQuestionSurface_Execute_TooManyQuestions` | validation: 5 questions |
| T04 | `TestAskUserQuestionSurface_Execute_OptionLabelRequired` | validation: 空 label |
| T05 | `TestAskUserQuestionSurface_Execute_DuplicateLabels` | validation: 重复 label |
| T06 | `TestAskUserQuestionSurface_Execute_HeaderCap` | validation: header > 12 |
| T07 | `TestAskUserQuestionSurface_Execute_HappyPath` | 正常 sender 装配，验证格式化文本 + 3 选项 + "其他" |
| T08 | `TestAskUserQuestionSurface_Execute_NoSender` | sender=nil，Delivered=false 但 success |
| T09 | `TestAskUserQuestionSurface_Execute_SenderError` | sender 返 error → ToolResult.Error |
| T10 | `TestAskUserQuestionSurface_RenderMultiple` | 多问题渲染（header chip + multi_select 标记） |
| T11 | `TestAskUserQuestionSurface_Execute_InvalidJSON` | input JSON 解析失败 |
| T12 | `TestAskUserQuestionSurface_ConcurrentSetGet` | 并发 SetAskUserQuestionSender / currentAskSender `-race` 干净 |

## 8. 与 clawcode 的差异（设计决策记录）

| 维度 | clawcode | devrix | 理由 |
|---|---|---|---|
| 阻塞模式 | 同步（UI 等点击） | 异步（IM 推送即返回） | devrix 是 IM bot，没有 UI 渲染线程 |
| Other 入口 | UI 自动加 | 显式追加 "其他. 直接回复你的想法" | IM 通道没有自动 affordance |
| 工具返回 | 数组 `{answer: string}` | JSON `{delivered, sent_at, hint, ...}` | devrix 强调可观测性，hint 引导 LLM 处理下个 message |
| 错误处理 | 工具异常 | sender 失败 → ToolResult.Error；LLM 可见可重试 | devrix 的 sender 失败可能是网络抖动，LLM 看到后可重发 |

## 9. 后续演进（v1.1+）

- **同步 rendezvous** —— 加 event bus，sender 等用户回复后再返回（CLI / REPL 场景）
- **配额限制** —— per-turn / per-session ask_user_question 次数上限（防 LLM 滥用拖延）
- **draft 保存** —— 草稿状态持久化，session 重启后可恢复未答问题
- **结构化回复** —— 用户回复 "1" 时直接解析为 option index，免 LLM 再解析 free text
