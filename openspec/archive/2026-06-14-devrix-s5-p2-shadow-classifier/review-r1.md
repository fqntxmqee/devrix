---
review-id: S3-Gate
title: S5-P2 Tail-only LLM Classify Shadow — S3-Gate Design Review
change-id: devrix-s5-p2-shadow-classifier
demand-id: DM-20260614-004
reviewer: Claude
review-date: 2026-06-14
status: APPROVED
---

# S5-P2 Tail-only LLM Classify Shadow — S3-Gate Design Review

> 按 `openspec/specs/project/review-design.md` 流程逐项执行。

---

## 1. 提案完整性 ✅

| 检查项 | 状态 | 证据 |
|--------|------|------|
| `.openspec.yaml` 存在 | ✅ | `openspec/changes/devrix-s5-p2-shadow-classifier/.openspec.yaml` |
| `proposal.md` 存在 | ✅ | 3 方案评估 + 选定 B |
| `design.md` 存在 | ✅ | 架构图 + 数据结构 + 流程 + 测试点 + 兼容性 |
| `tasks.md` 存在 | ✅ | 16 任务 + AC 映射 + 依赖关系 |
| `demand.md` 存在 | ✅ | DM-20260614-004，P1 |

**方案选定**：B — 异步 tail-only shadow。符合 R2 §5 命题 C 决议。

---

## 2. 需求覆盖 ✅

| AC | 来源 | 设计覆盖 |
|----|------|----------|
| AC1 | demand.md §2 | §2.1 `LLMIntentClassifier` 接口 |
| AC2 | demand.md §2 | §2.2 `ShadowMetrics` + §2.3 `ShadowClassifier` |
| AC3 | demand.md §2 | §3 流程 tail-only（rule 命中 fast/command/skip 不触发 LLM） |
| AC4 | demand.md §2 | §3 流程 tail path（rule orchestrate → 异步 LLM） |
| AC5 | demand.md §2 | §3 流程 LLM Error / Timeout |
| AC6 | demand.md §2 | §2.2 `Match` counter |
| AC7 | demand.md §2 | §2.2 `Mismatch` counter + §2.3 `shadowAsync` log |
| AC8 | demand.md §2 | §2.3 nil llm 路径早 return |
| AC9 | demand.md §2 | §4 测试点 9 个 |
| AC10 | demand.md §2 | §2.3 config 字段 + demand §3.2 修改 |

**覆盖度**：10/10 AC = 100%。

---

## 3. 设计质量 ✅

| 检查项 | 状态 | 证据 |
|--------|------|------|
| 架构清晰 | ✅ | ASCII 图 + 3 流程图（hot path / tail path / error） |
| 数据结构完整 | ✅ | 1 接口 + 1 metrics struct + 1 classifier struct |
| Tail-only 语义准确 | ✅ | R2 决议 §5 命题 C："仅对规则未命中 tail" |
| 异步 + 不阻塞 | ✅ | `go s.shadowAsync(...)` + `context.WithoutCancel` |
| 默认 disabled | ✅ | `ShadowLLMClassify: false` 避免对未启用部署产生 LLM 成本 |
| 接口解耦 | ✅ | `LLMIntentClassifier` 接口在 d7 包内，D3 gateway 反向依赖 |
| 兼容性好 | ✅ | §5 兼容性表 + §6 不变更清单 |
| 风险评估 | ✅ | §7 风险与缓解 5 项 |
| 估算 | ✅ | tasks.md §1 总计 4h30min |

---

## 4. 任务分解合理性 ✅

| 检查项 | 状态 | 证据 |
|--------|------|------|
| 任务粒度 | ✅ | 16 个任务，最大估算 60 分钟 |
| AC → T 映射 | ✅ | tasks.md §3 全部 10 个 AC 覆盖 |
| 依赖关系图 | ✅ | tasks.md §2 |
| 完成判定清单 | ✅ | tasks.md §5 |

---

## 5. 决议

**Severity** | **Count**
--- | ---
CRITICAL | 0
HIGH | 0
MEDIUM | 0
LOW | 0

**决议**：**APPROVED** — 无任何级别问题。可进入 S4 实现。

---

## 6. 后续动作

1. ✅ S3-Gate 通过 → 进入 S4 实现
2. S4 完成 16 个任务：1 接口 + 1 metrics + 1 classifier + 9 测试 + config 接入 + orchestrator 接入
3. S4-Gate：review-code.md
4. S5 验收：acceptance-report.md
5. S6 归档
