# D5 Observability — 终态指南

**Capability:** observability
**Status:** S3 Design Draft
**Version:** 1.0.0
**Last Updated:** 2026-06-19
**Parent:** `d5-domain.md`

> S7 归档后迁入 `openspec/specs/d5-observability/terminal-state-guide.md`。

---

## 1. 终态定义

D5 v2.1 Terminal = **规格锚点 + 物理锚点 + 语义锚点** 三锚闭合：

| 锚点 | 终态标志 |
|------|----------|
| 规格 | `spec.md` v3.0 Canonical S21–S24 主叙事 |
| 物理 | 无 bridge 包；根目录无孤儿；scenario 目录完整 |
| 语义 | S23 C3a–C3e + S21-A14 + S0-A03 写入 a-registry |

---

## 2. DSAFT 五层终态计数（目标）

| Layer | D5 Mapping |
|-------|------------|
| D | D5 Observability |
| S | S0 + S21–S24（5） |
| A | 30（v4.0：+A14, +A03, +A07–A10） |
| F | ~45（v3.0 f-registry） |
| T | 41（2 PLANNED 闭合后全 IMPLEMENTED） |

详细计数见归档后 `dsaft-architecture.md` Stub。

---

## 3. 文档 SoT 索引

| 读者问题 | 读哪份 |
|----------|--------|
| D5 是什么、承诺是什么 | `d5-domain.md` |
| Gherkin 验收 | `spec.md` |
| Span↔T、Runbook | `observability-guide.md` |
| 跨域谁创建什么 span | `d5-boundary.md` |
| 代码在哪个目录 | `d5-domain.md` 路径表 + `code-layout.md` §4.6 |
| A/F/T 明细 | `a-registry` / `f-registry` / `t-registry` |
| 56 ops Trace 树 | `span-registry.md` |
| 染色操作手册 | `coverage.md` |
| 层能力 Delta | `layer-delta.md` §v2.1-Terminal |
| 六段式设计 | `design.md` v3.0 |

---

## 4. Legacy 双轨约束

- D5-S1–S9：**冻结**，仅追溯，禁止新 T 挂 Legacy S
- `query.loop.*`：**RETIRED**，禁止文档主路径引用
- bridge 包：**REMOVED**（v2.1），禁止新 import

---

## 5. 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | S3 设计稿 |
