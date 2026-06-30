# Proposal: d6 演化域 spec 精简

**Change ID:** devrix-d6-spec-lite
**Demand ID:** DM-20260630-009
**Status:** S2_Proposal
**Date:** 2026-06-30

---

## 1. 方案对比

### 方案 A：复用 lite-mode 模式（推荐）

复用 d7/d2/d1/d3/d4/d5 lite-mode 模式（DM-20260630-003/004/005/006/007/008）：
- spec.md 顶部 SoT 引用 + 8 段结构
- CHANGELOG.md 4 列表格
- 18 条 Requirements 详细 Gherkin 迁 archive
- 0 Go 代码 diff + 0 其他域 diff + 0 d6 子文档 diff
- 1 canonical Gherkin 范式（候选：D6-S3 Tier Resolution ≥ 99%）

**优势**：模式已 6 站验证，最小 diff，跨域一致性，0 行为变更

**劣势**：18 条 Requirements 详细文本不再在 spec.md 中（trade-off）

### 方案 B：物理分片

按 S 层分片 spec-s3.md / spec-s4.md / spec-s5.md / spec-s11.md / spec-s12.md。

**劣势**：d7 spec-split 已被 lite-mode 替代（s1_cancelled）；增加 5 个子文件

### 方案 C：仅做 CHANGELOG.md

**劣势**：spec.md 604 行仍在，不符合 lite-mode 核心规范

## 2. 决策

**方案 A**：复用 lite-mode 模式。lite-mode 推广第 6 站。

## 3. 工作量

2 PR（main + archive），~7 文件改动。

## 4. 风险

| 风险 | 缓解 |
|------|------|
| 18 条 Requirements 详细文本丢失 | 1 行 reference + CHANGELOG.md 时间线 |
| canonical Scenario 选错 | 候选 D6-S3 Tier Resolution ≥ 99%（v2.2.0 新增探针，跨 D3-D6 锚点） |
| 604→200 压缩比 67% 是 6 站最大 | 重点削 Requirements + 探针详细表 + Revision History + D6-S11/S12 |

## 5. 验收

12 AC，详见 demand.md

## 6. 复用参考

- devrix-spec-lite-mode (DM-20260630-003) PR #333/#334
- devrix-d2/d1/d3/d4/d5-spec-lite PR #336+#337/#338+#339/#340+#341/#342+#343/#344+#345