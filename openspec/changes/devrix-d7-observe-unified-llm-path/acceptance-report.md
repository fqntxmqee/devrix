---
demand-id: DM-20260630-001
change-id: devrix-d7-observe-unified-llm-path
title: D7 Observe 统一 LLM 入口 — 验收报告
executor: Agent S5
environment: local dev (go test)
date: 2026-06-30
verdict: ACCEPTED
---

# 验收报告：D7 Observe 统一 LLM 入口

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260630-001 |
| Change ID | devrix-d7-observe-unified-llm-path |
| 退役来源 | DM-20260628-002 T35 裸 D3 路径 / D7-S16-A74 |
| 总体结论 | **ACCEPTED** |

Observe 阶段如需调 D3，必须先经 D2 `ContextPreparer.Prepare` 获取 i18n system prompt，再调 D3 生成 Obs 提案。T35 裸 D3 + 英文写死 prompt 已废弃。

### 测试命令与结果

| Check | Command | Result |
|-------|---------|--------|
| Bootstrap | `go test ./internal/bootstrap/... -count=1` | **PASS** |
| SessionOrch | `go test ./internal/layers/orchestration/sessionorchestrator/... -count=1` | **PASS** |

## 2. L5 / T 验收矩阵

| T ID | 描述 | 结果 |
|------|------|------|
| D7-S16-A75-T01 | WireItemPipeline wires LLMObservationProposer | PASS |
| D7-S16-A75-T02 | Observe D2 Prepare before D3 | PASS |
| D7-S16-A75-T03 | zh-CN appendix + ValidateObservationProposals | PASS |
| D7-S16-A75-T04 | spec + t-registry 同步 | PASS |

| OU | 业务目标 | 结果 |
|----|----------|------|
| OU-1 | Observe D2 先于 D3 | PASS（单测 + 代码审查） |
| OU-2 | D2 i18n + Obs 附录 | PASS |
| OU-3 | 禁止裸调 D3 | PASS |
| OU-4 | R-OBS + fail-safe 保留 | PASS |

## 3. 领域文档同步

| 文件 | 已更新 |
|------|--------|
| `openspec/specs/d7-orchestration/spec.md` v4.19.0 | ✅ |
| `openspec/specs/d7-orchestration/t-registry.md` | ✅ |
| `openspec/changes/devrix-d7-observe-unified-llm-path/` | ✅ |

## 4. Jaeger 手工验收指引（合入后）

1. 发送首条消息「你好」
2. 展开 `D7_MUPS_Pipeline` → Observe 子树：**应**有 `D2_Context_Process` → `D3_LLM_Stream`
3. Observe 的 system_prompt 含 D2 中文基座 + Obs 附录（非 T35 纯英文 prompt）
4. Execute 子树独立：`D2_Context_Process` → `D3_LLM_Stream`（主 ReAct）

## 5. 归档待办（S7）

- [ ] PR 合入 main
- [ ] 移包至 `openspec/archive/2026-06-30-devrix-d7-observe-unified-llm-path/`
- [ ] 更新 `openspec/demand-archive-index.md`
