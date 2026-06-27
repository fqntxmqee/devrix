# Demand: D7 ItemPipelineRunner emit hook (hotfix)

**Demand ID:** DM-20260627-001
**Created:** 2026-06-27
**Reporter:** 用户飞书反馈
**Priority:** P0

## 现象

飞书指令 "review d2领域代码" 触发的 LLM 调用：
- tools 调用都没有显示（飞书卡片只有 152 字节 meta-comment）
- LLM 文本响应没有显示（实际是所有 text/thinking/tool_call/tool_result 事件都被 ItemPipelineRunner 层吞掉）

## 触发链

1. 飞书消息 "review d2领域代码" → gateway.RouteInbound
2. SessionOrchestrator → ItemPipelineRunner（default-on since DM-20260626-009）
3. DefaultWorkItemExecutor.Emit = nil → 事件静默丢
4. D1 gateway 收不到事件 → 飞书卡片无内容

## 根因

D7 ItemPipelineRunner 路径的 emit hook 在 default-on 切换时没接线：
- Wave path：OrchestratePath.Run → subagent.streamEmit ✓
- ItemPipelineRunner path：DefaultWorkItemExecutor.Emit = nil ✗

3 个改动点缺失：
1. DefaultWorkItemExecutor.Emit 字段不存在
2. ItemPipelineRunner.Emit 字段 + propagation 缺失
3. session_turn_loop.go goroutine wrapper 没接 itemPipeline.Emit

## 验收

- 飞书发 "review d2领域代码" 后，飞书卡片可见：
  - tools 列表（tool_call 事件）
  - LLM thinking + text（text/thinking 事件）
  - tool_result（带 Name）
- 09:32 用户反馈："tools有了" ✓

## 关联

- DM-20260627-002（PR #258）— AGENTS.md 加 D{N}→path 映射（同一 bug 调研过程发现）