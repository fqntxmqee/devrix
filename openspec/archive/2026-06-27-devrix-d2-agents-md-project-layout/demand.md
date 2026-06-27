# Demand: .devrix/AGENTS.md project layout map

**Demand ID:** DM-20260627-002
**Created:** 2026-06-27
**Reporter:** 用户飞书反馈（PR #257 衍生）
**Priority:** P0

## 现象

PR #257 修好 "tools 不显示" 后，飞书用户反馈 "调用大模型返回的信息没有展示出来"。查 transcript jsonl 发现 LLM 真返回的就是 152 字节 meta-comment：

> The user is asking me to review the 'd2' domain code. Let me start by exploring the workspace to understand the structure and find the d2 domain code.

LLM 没找到 d2 域代码就放弃了。

## 触发链

1. 飞书消息 "review d2领域代码" → gateway.RouteInbound
2. SessionOrchestrator → ItemPipelineRunner（PR #257 emit hook 修复后）
3. LLM 收到 system prompt，包含 `.devrix/AGENTS.md` 内容（runtime-loaded）
4. LLM 看到 "d2" 但 .devrix/AGENTS.md 没有 D{N}→path 映射
5. LLM 盲搜 `**/d2/**/*.go`（空）→ 盲搜 `**/domain/d2*`（空）→ 放弃
6. 回 152 字节 meta-comment → D1 gateway 显示给飞书用户

## 根因

devrix 项目的 runtime-loaded `.devrix/AGENTS.md` 没暴露项目结构布局：
- CLAUDE.md（人读）有 D{N}→path 表 ✓
- .devrix/AGENTS.md（runtime-loaded）没有 ✗

## 验收

- 飞书发 "review d2领域代码" 后，LLM 应：
  - 直接定位 `internal/layers/contextengine/`
  - 产出真正的代码 review（数百/数千字节）
  - 不再回 152 字节 meta-comment
- 等待用户飞书实测验收

## 关联

- DM-20260627-001（PR #257）— emit hook 修 "tools 不显示"，本 PR 治同 bug 的另一半