# Design: d6 演化域 spec 精简

**Change ID:** devrix-d6-spec-lite
**Demand ID:** DM-20260630-009
**Status:** S3_Design
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-06-30

---

## ① 架构目标

- d6 spec.md 604 → ≤ 200（18 条 Requirements 详细 Gherkin → archive）
- d6 CHANGELOG.md 0 → ≤ 300
- 0 Go 代码 diff
- 7 个 d6 子文档 0 diff
- 复用 d7/d2/d1/d3/d4/d5 lite-mode 模式（6 站验证）

## ② 架构原则

1. **复用 d7/d2/d1/d3/d4/d5 lite-mode 模式**（6 站验证）
2. **d6-domain.md v1.0.0 是 SoT**
3. **canonical S = 3 (S3+S4+S5) + v2.4 韧性 2 (S11+S12)** 
4. **不创建子文件**
5. **检索路径固定**：spec.md → CHANGELOG.md → archive/<change>/
6. **d6 7 子文档**全部不动
7. **跨域一致性**
8. **0 行为变更**

## ③ 业务流程

S6-归档触发 → 评估 d6 域 → lite-mode 评估 → git mv changes → archive → 更新 demand-archive-index.md → verify-archive.sh PASS。

## ④ 领域模型

聚合根：spec.md（主契约）/ CHANGELOG.md（时间线）/ d6-domain.md v1.0.0（D6 SoT）/ archive/<change>/specs/（过程需求）。

d6 8 文件白名单：本 change 仅 spec.md + CHANGELOG.md，其他 7 个不动。

## ⑤ 核心链路图

读路径：spec.md → d6-domain.md v1.0.0 → CHANGELOG.md → archive/。SLA ≤ 6 跳。

D6 跨域评估链：
```
D6-S3 Eval Engine ← ProbeRegistry (10 类探针) ← LLM-as-Judge (D3 Gateway)
D6-S4 GuardRuntime ← D4 AgentObserver → Intervention (terminate/reroute)
D6-S5 VerifyInvariant ← init() fail-closed log.Fatalf → Plan 验证 (D7 PlanMode)
```

## ⑥ 接口 / API 设计

- spec.md 顶部契约段（8 段结构）
- CHANGELOG.md 4 列表格
- 18 条 Requirements 1 行 reference
- 1 canonical Gherkin 范式（候选：D6-S3 Tier Resolution ≥ 99%）

---

## 附录 A：File Manifest

| 文件 | 类型 | 行数 | 说明 |
|------|------|------|------|
| `openspec/specs/d6-evolution/spec.md` | REWRITE | 604 → ≤ 200 | 重写为精简设计契约 |
| `openspec/specs/d6-evolution/CHANGELOG.md` | NEW | 0 → ≤ 300 | d6 域时间线 |
| 6 change docs | NEW | — | S1-S5 |