# Design: d5 可观测性域 spec 精简

**Change ID:** devrix-d5-spec-lite
**Demand ID:** DM-20260630-008
**Status:** S3_Design
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-06-30

---

## ① 架构目标

- d5 spec.md 376 → ≤ 200（13 条 Requirements 详细 Gherkin → archive）
- d5 CHANGELOG.md 0 → ≤ 300
- 0 Go 代码 diff
- 12 个 d5 子文档 0 diff
- 复用 d7/d2/d1/d3/d4 lite-mode 模式（5 站验证）

## ② 架构原则

1. **复用 d7/d2/d1/d3/d4 lite-mode 模式**（5 站验证）
2. **d5-domain.md v3.0.0 是 SoT**（双层文档清晰分工）
3. **canonical S = 5 (S21-S24 + S0)**（v2.1 Terminal 收口）
4. **不创建子文件**（lite-mode 硬约束）
5. **检索路径固定**：spec.md → CHANGELOG.md → archive/<change>/
6. **d5 12 子文档**全部不动
7. **跨域一致性**：规范升级对所有域立即生效
8. **0 行为变更**（纯文档规范升级）

## ③ 业务流程

S6-归档触发 → 评估 d5 域 → lite-mode 评估 → git mv changes → archive → 更新 demand-archive-index.md → verify-archive.sh PASS。

## ④ 领域模型

聚合根：spec.md（主契约）/ CHANGELOG.md（时间线）/ d5-domain.md v3.0.0（D5 SoT）/ archive/<change>/specs/（过程需求）。

d5 13 文件白名单：本 change 仅 spec.md + CHANGELOG.md，其他 12 个不动。

## ⑤ 核心链路图

读路径：spec.md → d5-domain.md v3.0.0 → d5-boundary.md → CHANGELOG.md → archive/。SLA ≤ 6 跳。

D7 Turn 主路径（v2.1 Terminal canonical）：
```
D1 Gateway → gateway.message.receive → D7 Orchestration → orchestration.turn.run → orchestration.turn.iteration → orchestration.llm.invoke → llm.stream (D3) + context.process (D2, caller=d7) → tool.execute.single
```

## ⑥ 接口 / API 设计

- spec.md 顶部契约段（8 段结构）
- CHANGELOG.md 4 列表格
- 13 条 Requirements 1 行 reference（指向 archive）
- 1 canonical Gherkin 范式（候选：D5-S23 Coverage HealthCheck / D5-S21 Tracer Start Operation Registry）

---

## 附录 A：File Manifest

| 文件 | 类型 | 行数 | 说明 |
|------|------|------|------|
| `openspec/specs/d5-observability/spec.md` | REWRITE | 376 → ≤ 200 | 重写为精简设计契约 |
| `openspec/specs/d5-observability/CHANGELOG.md` | NEW | 0 → ≤ 300 | d5 域时间线 |
| 6 change docs | NEW | — | S1-S5 |

## 附录 B-D：略（同 d1/d2/d3/d4 lite-mode pattern）

## 附录 E：下一步

1. S4 实现（cut branch + spec.md 重写 + CHANGELOG.md NEW + 验证）
2. S5 验收（acceptance-report.md）
3. S6-交付（push + PR + auto-merge）
4. S6-归档（独立 PR）