# Proposal: d5 可观测性域 spec 精简

**Change ID:** devrix-d5-spec-lite
**Demand ID:** DM-20260630-008
**Status:** S2_Proposal
**Date:** 2026-06-30

---

## 1. 方案对比

### 方案 A：复用 lite-mode 模式（推荐）

复用 d7/d2/d1/d3/d4 lite-mode 模式（DM-20260630-003/004/005/006/007）：
- spec.md 顶部 SoT 引用 + 8 段结构（Overview / 核心设计原则 / S 层职责 / DSAFT / Scenarios / Architecture / 关键 Scenario 范式 / 关键链路口）
- CHANGELOG.md 4 列表格（Date | Change ID | 摘要 | 状态 | 归档）
- 13 条 Requirements 详细 Gherkin 迁 archive（1 行 reference）
- 0 Go 代码 diff + 0 其他域 diff + 0 d5 子文档 diff
- 1 canonical Gherkin 范式（候选：D5-S23 Coverage HealthCheck）

**优势**：
- 模式已 5 站验证（d7/d2/d1/d3/d4）
- 最小 diff 范围（spec.md + CHANGELOG.md）
- 跨域一致性（同一规范）
- 0 行为变更

**劣势**：
- 13 条 Requirements 详细文本不再在 spec.md 中（trade-off）

### 方案 B：物理分片（spec-s*.md）

按 S 层分片 spec.md → spec-s21.md / spec-s22.md / spec-s23.md / spec-s24.md / spec-s0.md。

**优势**：
- 按 S 分片利于跨域对齐

**劣势**：
- d7 spec-split DM-20260630-002 已被 lite-mode 替代（s1_cancelled）
- 增加 4-5 个子文件，违反 lite-mode 不创建子文件硬约束

### 方案 C：仅做 CHANGELOG.md

只新增 CHANGELOG.md 不精简 spec.md。

**优势**：
- 改动最小

**劣势**：
- spec.md 376 行仍在（远超 200 行上限）
- 不符合 lite-mode 核心规范

## 2. 决策

**方案 A**：复用 lite-mode 模式。lite-mode 推广第 5 站。

## 3. 工作量估算

- S2 proposal: 1 PR
- S3 design: 1 PR
- S4 实现: 1 PR（含 spec.md REWRITE + CHANGELOG.md NEW）
- S5 验收: 1 acceptance-report.md
- S6 交付: 1 PR（主 change，含 0 Go diff）
- S6 归档: 1 PR（git mv + index 更新）

**总计**：2 PR（main + archive），~7-10 文件改动（不含 git rename）。

## 4. 风险

| 风险 | 缓解 |
|------|------|
| 13 条 Requirements 详细文本丢失 | 1 行 reference 指向 archive/ + CHANGELOG.md 时间线可追 |
| canonical Scenario 选错 | 候选 D5-S23 Coverage HealthCheck（覆盖核心：Operation Registry + Runtime Hit + zero_hit），同时考虑 D5-S21 Tracer Start Operation Registry |
| d5 域独有特性（GenAI 双写、Coverage 独立于采样、slog+OTLP+LLM JSONL 三联）丢失 | 核心设计原则 8 条覆盖 + Architecture 段引用 D5/D7 Span Hierarchy 图 |
| 跨域一致性（D7 Turn Span 主路径） | 关键链路口段引用 D7 主路径 |

## 5. 验收

12 AC（详见 demand.md），含：
- AC1 spec.md ≤ 200 行
- AC4 CHANGELOG.md ≤ 300 行 + ≥ 3 d5 change
- AC6 0 Go diff
- AC7 0 d5 其他 12 子文档 diff
- AC12 verdict: ACCEPTED

## 6. 复用参考

- devrix-spec-lite-mode (DM-20260630-003) PR #333/#334
- devrix-d2-spec-lite (DM-20260630-004) PR #336/#337
- devrix-d1-spec-lite (DM-20260630-005) PR #338/#339
- devrix-d3-spec-lite (DM-20260630-006) PR #340/#341
- devrix-d4-spec-lite (DM-20260630-007) PR #342/#343