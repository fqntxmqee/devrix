# D1 Communication Spec Delta — feishu TurnInProgressError 友好文案 (DM-20260628-003)

**Change ID:** devrix-d7-multiturn-session-state
**Demand ID:** DM-20260628-003
**Delta Type:** ADDED (v3.5.0 → v3.6.0)
**SOT:** `internal/layers/communication/feishu/feishu.go`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. feishu adapter 检测 `sessionorchestrator.TurnInProgressError` | `feishu.go` | MODIFIED | IM 卡片显示"⏳ 上一条还在处理中" |
| 2. CLI 适配器不动 | `cli.go` | UNCHANGED | 保持兼容（errors.Is 不识别） |

---

## 2. 错误识别契约（D1 → D7）

```go
// feishu.go 错误处理分支（伪代码）
err := sessionorchestrator.Entry.ProcessMessage(ctx, req)
if err != nil {
    var tipErr sessionorchestrator.TurnInProgressError
    if errors.As(err, &tipErr) {
        // 发送 IM 卡片：⏳ 上一条消息还在处理中，请稍候...
        return nil  // 不返回 error 事件，避免 Done 卡片被覆盖
    }
    // 原有 error 处理路径不变
    return err
}
```

---

## 3. T 点增量（v3.5.0 → v3.6.0）

| T ID | 描述 | Status |
|------|------|--------|
| D1-S2-A13-T08 | feishu adapter 识别 `TurnInProgressError` 错误 + 发送"⏳ 上一条还在处理中" IM 卡片 | PENDING (S4) |

1 个 T 点 P0。

---

## 4. 行为不变保证

- CLI 适配器（`cli.go`）保持原有 error 处理路径（不识别 TurnInProgressError，走通用文案）
- 现有 error event 行为不变（除新增 TurnInProgressError 分支外）
- feishu 现有所有事件类型（text/tool_call/tool_result/complete/error）行为不变
- DM-20260628-001（API 错误分类）已暴露的 `error_code` 字段处理逻辑不变

---

## 5. 文案模板

| 错误类型 | IM 卡片文案 |
|----------|------------|
| TurnInProgressError | ⏳ 上一条消息还在处理中，请稍候... |
| 其他 error（fallback） | 现有通用文案（DM-20260628-001 引入的 code 差异化文案） |

---

## 6. Out of Scope

- CLI 适配器 TurnInProgressError 友好提示（保持兼容，不在本 change scope）
- IM 卡片显示历史 turn 列表（单独 follow-up）
- 飞书 card action 按钮（用户主动取消 turn）（不引入新交互）

---

## 7. 关联变更

- **DM-20260628-002 (PR #271)**：panic hotfix
- **DM-20260628-003**：本 change，定义 `TurnInProgressError` 类型，D1 在本 delta 中识别
- **DM-20260628-001**：API 错误分类，已暴露 `error_code` 字段供 IM 端差异化消费