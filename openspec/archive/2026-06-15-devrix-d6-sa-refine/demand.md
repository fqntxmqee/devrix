---
demand-id: DM-20260615-002
title: D6 Evolution — 消除占位 S + 解决 D7 命名冲突
source: 架构审计（D6 S1/S2 为 PLANNED 占位符；S4 "Orchestration" 与 D7 同名冲突）
priority: P1
status: S3_Design
dsaft_domain: evolution
created: 2026-06-15
parent: dsaft-refactoring-playbook
related:
  - DM-20260614-016  # D3 SA Refine（首案）
  - DM-20260614-018  # D4 SA Refine
  - DM-20260615-001  # D5 SA Refine（并行）
---

# D6 Evolution — 消除占位 S + 解决 D7 命名冲突

## 1. 背景

### 1.1 D6 根本目标

**作为支撑域，提供自演化评测引擎（RunEvaluation）+ 运行时守护（GuardRuntime），验证系统行为质量。**

消费者可验证承诺：

| # | 承诺 | 验收主体 |
|---|------|---------|
| C1 | **RunEvaluation**：给定 dataset + probes，返回 eval_report + delta + tune | CLI / CI |
| C2 | **GuardRuntime**：Agent 异常被 Observer 捕获 → Validator 判定 → Intervention 执行 | D7 Orchestration |
| C3 | **TrackVersion**：构建信息 → 版本报告（PLANNED） | 运维 |
| C4 | **ReloadConfig**：监控配置变更 → 热加载（PLANNED） | 运维 |

### 1.2 现状问题

| 问题 | 根因 |
|------|------|
| D6-S1/S2 为 PLANNED 占位符，无代码但占独立 S 位 | 早期架构预留 |
| D6-S4 名为 "Orchestration"，与 D7 Orchestration Domain 同名冲突 | `code-layout.md §2` 要求跨域唯一语义化 |
| D6-S3 Eval 承载 89% T 点，但 S 名不反映评测语义 | 技术模块切法 |

---

## 2. 方案概要

**S 层重切为 4 价值流（S11–S14）：**

| Canonical S | Scenario | 旧 S | 关键变更 |
|-------------|----------|------|---------|
| D6-S11 | RunEvaluation | S3 Eval | 动词+名词语义化 |
| D6-S12 | GuardRuntime | S4 Orchestration | **消除 D7 命名冲突** |
| D6-S13 | TrackVersion | S1 Version | 保留 PLANNED，语义化重命名 |
| D6-S14 | ReloadConfig | S2 Config | 保留 PLANNED，语义化重命名 |

**v1.0 范围：** 纯文档 — 重排 a/t-registry、更新 layering/code-layout/cross-domain-boundaries。零 Go 代码变更。

---

## 3. 验收标准

- [x] `a-registry.md` Canonical v3.0（4 S + Legacy 列）
- [x] `t-registry.md` Canonical v3.0（canonical_s 列 + Legacy T ID 列）
- [x] `layering.md` §D6 Canonical + Legacy 双轨
- [x] `code-layout.md §4.7` D6 scenario-slug 注册表
- [ ] 24 T 追溯表 100% 覆盖验证
- [ ] S3-Gate review 通过
