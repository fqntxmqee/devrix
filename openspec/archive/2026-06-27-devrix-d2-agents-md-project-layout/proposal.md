# Proposal: .devrix/AGENTS.md expose D{N}→path map

**Change ID:** `devrix-d2-agents-md-project-layout`
**Demand ID:** DM-20260627-002
**Priority:** P0
**Sprint:** d2-hotfix-2026-06-27
**PR Count:** 1
**Status:** S7_Archived
**SoT:** 用户飞书指令 "review d2领域代码" 触发的 hotfix investigation（PR #257 衍生）

---

## 1. Background

PR #257 修好了 "tools 不显示"，但飞书用户反馈 "调用大模型返回的信息没有展示出来"。实际查 transcript jsonl 发现 LLM 真返回的就是 152 字节 meta-comment：

> The user is asking me to review the 'd2' domain code. Let me start by exploring the workspace to understand the structure and find the d2 domain code.

LLM 盲搜 `**/d2/**/*.go` 和 `**/domain/d2*` 返回空，就放弃给 meta-comment。

## 2. Problem Statement

**根因：** devrix 项目的运行时 AGENTS.md (.devrix/AGENTS.md) 没暴露项目结构布局：

- CLAUDE.md（人读）有 D{N}→path 表 ✓
- .devrix/AGENTS.md（runtime-loaded）没有 ✗

LLM 收 "review d2 域代码" 指令时，不知道 d2 = contextengine = `internal/layers/contextengine/`。

## 3. Proposed Solution

在 `.devrix/AGENTS.md` 加 "## 项目结构（D{N} → 路径）" 章节：

1. 7 行 D{N} → 目录 映射表（D1-D7 → internal/layers/<domain>/）
2. 3 条补充：
   - 跨域共享 `internal/shared/`
   - 域归档 `openspec/specs/d{N}-*/`
   - 不确定时先 `ls internal/layers/` 探测

**项目特定修复**（不动 workspace_guidance.zh.md 通用模板），避免影响其他 devrix 用户。

## 4. Scope

- **改了**：1 file +20（仅 `.devrix/AGENTS.md` markdown）
- **没改**：runtime 逻辑、API 契约、Go 代码、t-registry

## 5. Risk

极低：纯 markdown 文档变更，不影响运行时。

## 6. Cache 注意点

`internal/layers/contextengine/prepare/prompt/agents_discovery.go:19` 全局缓存 `globalAgentsContextCache`：

- 只在 snapshot corrupt 时清（`session_loader.go:73`）
- 改 `.devrix/AGENTS.md` 后**必须 restart devrix** 才生效
- 长期方案考虑 mtime invalidation，目前 hotfix path 走 process restart

## 7. Hotfix Path Rationale

按 `feedback-devrix-bugfix-skip-openspec.md`：小 bug 跳过 S1-S6 完整流程。本次：
1. 09:15 — commit (ffe3e5f) → PR #258 创建 + auto-merge
2. 09:34 — PR #258 merge
3. 09:45 — build + restart devrix（pid=85935），cache 清空，下次会话生效
4. 09:46 — S7_Archived 补归档

## 8. Related Work

- PR #257（DM-20260627-001）：emit hook 修 "tools 不显示"，本 PR 治同 bug 的另一半 — LLM 内容质量