# Design: d4 多智能体域 spec 精简

**Change ID:** devrix-d4-spec-lite
**Demand ID:** DM-20260630-007
**Status:** S3_Design
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-06-30

---

## ① 架构目标

- d4 spec.md 222 → ≤ 200（Sub-Agent Mode Field process requirement 54 行 → archive/）
- d4 CHANGELOG.md 0 → ≤ 300
- 0 Go 代码 diff
- 11 个 d4 子文档 0 diff
- 复用 d7/d2/d1/d3 lite-mode 模式

## ② 架构原则

1. **复用 d7/d2/d1/d3 lite-mode 模式**（5 站验证）
2. **d4-domain.md v2.2.0 是 SoT**（双层文档清晰分工）
3. **canonical S = 6 + Legacy S1-S10 双轨**
4. **不创建子文件**（lite-mode 硬约束）
5. **检索路径固定**：spec.md → CHANGELOG.md → archive/<change>/
6. **d4 11 子文档**全部不动
7. **跨域一致性**：规范升级对所有域立即生效
8. **0 行为变更**（纯文档规范升级）

## ③ 业务流程

S6-归档触发 → 评估 d4 域 → lite-mode 评估 → git mv changes → archive → 更新 demand-archive-index.md → verify-archive.sh PASS。

## ④ 领域模型

聚合根：spec.md（主契约）/ CHANGELOG.md（时间线）/ d4-domain.md v2.2.0（D4 SoT）/ archive/<change>/specs/（过程需求）。

d4 12 文件白名单：本 change 仅 spec.md + CHANGELOG.md，其他 11 个不动。

## ⑤ 核心链路图

读路径：spec.md → d4-domain.md v2.2.0 → d7-boundary.md → CHANGELOG.md → archive/。SLA ≤ 6 跳。

## ⑥ 接口 / API 设计

- spec.md 顶部契约段（8 段结构）
- CHANGELOG.md 4 列表格
- Sub-Agent Mode Field reference 单行（指向 archive）

---

## 附录 A：File Manifest

| 文件 | 类型 | 行数 | 说明 |
|------|------|------|------|
| `openspec/specs/d4-multi-agent/spec.md` | REWRITE | 222 → ≤ 200 | 重写为精简设计契约 |
| `openspec/specs/d4-multi-agent/CHANGELOG.md` | NEW | 0 → ≤ 300 | d4 域时间线 |
| 6 change docs | NEW | — | S1-S5 |

## 附录 B-D：略（同 d1/d2/d3 lite-mode pattern）

## 附录 E：下一步

1. S4 实现（cut branch + spec.md 重写 + CHANGELOG.md NEW + 验证）
2. S5 验收（acceptance-report.md）
3. S6-交付（push + PR + auto-merge）
4. S6-归档（独立 PR）
