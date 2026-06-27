# Acceptance Report: devrix-d2-agents-md-project-layout

**Change ID:** `devrix-d2-agents-md-project-layout`
**Demand ID:** DM-20260627-002
**Acceptance Date:** 2026-06-27
**Result:** ACCEPTED (hotfix path, pending user verification)

---

## 1. Acceptance Criteria

| AC | Description | Result | Evidence |
|----|-------------|--------|----------|
| AC1 | .devrix/AGENTS.md 含 D{N}→path 映射表 | ✅ PASS | openspec/archive/.../proposal.md §3 含完整表 |
| AC2 | Layer-lint pass on PR | ✅ PASS | gh pr checks 258 layer-lint (warn) pass |
| AC3 | Build + restart devrix 让 cache 清空 | ✅ PASS | pid=85935 @ 09:45:39 |
| AC4 | 用户飞书实测 "review d2 域代码" 后 LLM 产出真实 review | ⏳ PENDING | 等待用户飞书指令验收 |
| AC5 | AGENTS.md change 不破坏其他 devrix 用户（项目特定修复） | ✅ PASS | 只改 devrix 项目的 .devrix/AGENTS.md，未动 workspace_guidance.zh.md 通用模板 |

## 2. Test Results

- Layer-lint pass
- Markdown 文件修改无编译影响
- Cache invalidation 通过 process restart 完成

## 3. Risk Assessment

极低风险：
- 纯 markdown 文档变更，0 行 Go 代码变更
- 不影响其他 devrix 用户（项目特定修复）
- Cache 清理依赖 process restart，下次 restart 自动清空

## 4. Follow-up

- 长期方案：考虑 `globalAgentsContextCache` 加 mtime invalidation（避免 hotfix path 必须 restart）
- 通用方案：考虑把"项目布局探测"提示加到 `workspace_guidance.zh.md`（如"先 ls / glob 主目录再深入"）

## 5. Archive Status

**S7_Archived** — 按 hotfix path 跳过 S1-S6 完整流程；code+tests+commit → build+restart → 用户验收（pending）→ S7 归档。