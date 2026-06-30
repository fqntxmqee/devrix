# Proposal: d3 LLM 网关 spec 精简

**Change ID:** devrix-d3-spec-lite
**Demand ID:** DM-20260630-006
**Status:** S2_Proposal
**Follows:** devrix-spec-lite-mode (DM-20260630-003, s7_archived), devrix-d2-spec-lite (DM-20260630-004, s7_archived), devrix-d1-spec-lite (DM-20260630-005, s7_archived)

---

## 1. Background

`devrix-spec-lite-mode` (DM-20260630-003) 已 S7_Archived (2026-06-30, PR #333+#334)，lite-mode 规范生效：spec.md ≤ 200 / CHANGELOG.md ≤ 300 / 其他 d{N} 子文档 ≤ 800。d7/d2/d1 已作为示范完成（d7 spec.md 2622 → 195，d2 1622 → 152，d1 577 → 175；三次 S1-S7 闭环全部 squash auto-merge，0 行为变更）。

本 change 是 **lite-mode 推广第三站**。d3 spec.md 累积 1060 行（含 90 Scenario 详细 Gherkin 文本，行 ~85-1000），从 V1 (DM-20260607-004) 到 V3.2.0 (DM-20260614-017) 演进，canonical S = 6 + 1 CROSS，90 个 Scenario 已全部 IMPLEMENTED 在代码中。d3-domain.md v1.6.0 已 S7_Archived 收口，spec.md 仅作精简契约。

## 2. Problem Statement

d3 spec.md 累积了从 V1 到 V3.1 所有 ADDED Requirements + 90 个 Scenario 详细文本：

| 维度 | 现状 | 期望 |
|------|------|------|
| d3 spec.md 行数 | **1060** | ≤ 200 |
| d3 spec.md Requirement 段 | 5（§10-§14：V2 继承 / V3 跨域灰区 / V3 韧性可见性 / V3.1 / V4 API 错误分类） | 0（合并到 §Scenarios + archive/） |
| d3 spec.md Scenario 详细文本 | **90** | 1-2 canonical 范式 |
| d3 CHANGELOG.md | **无** | ≤ 300（6+ d3 change 时间线） |
| d3 6 个 archive 目录 | 累积 90 Scenario 详细文本 | 保留（不丢失） |

**痛点**：
- 违反已生效的 lite-mode 规范（spec.md 硬上限 200 行）
- 90 Scenario 详细文本对当前开发是噪音（已合入代码）
- d3 canonical S = 6 + 1 CROSS 已稳定收敛，无需逐 S 详述
- 用户原始诉求：specs 域文档只放最新符合代码的设计，过程需求走 archive/

## 3. Proposed Solution

### 3.1 方案对比

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A. 复用 d7/d2/d1 lite-mode 模式（推荐）** | 一致性；3 站（d7/d2/d1）验证成熟；1 PR 1 站到位 | 仍需 1 跳到 archive/ |
| B. 按 S 分片（spec-s{1..6}.md + cross.md） | S 层清晰 | 子文件持续累积；与 lite-mode 反模式 |
| C. 维持 1060 行不拆分 | 0 改动 | 违反 lite-mode 硬上限 |
| D. 按承诺装置分片（C1..C5 + Config + CROSS） | 与承诺装置哲学一致 | 6 + 1 段仍超 200 行；需深度重组 |

**选择**：A（复用 d7/d2/d1 lite-mode）

**理由**：
- lite-mode 模式已 3 站验证成熟（d7 PR #333 / d2 PR #336 / d1 PR #338）
- 用户原始诉求 = specs 域文档只放最新符合代码的设计
- 90 个 Scenario 详细文本对当前开发是噪音（已合入代码，需求态已转为代码态）
- d3-domain.md v1.6.0 已提供 North Star + 5 承诺 + DSAFT 资产 SoT
- d3 设计模式（5 承诺装置 + 1 横切 Config）与 d2/d1 canonical S 切法同构

### 3.2 复用 d7/d2/d1 lite-mode 模式

| 步骤 | 输出 | 行数 |
|------|------|------|
| S1 | `demand.md` | ~80 |
| S2 | `proposal.md`（本文） | ~150 |
| S3 | `design.md`（六段式 + 5 附录） | ~350 |
| S4 | `tasks.md`（5 phases） | ~80 |
| S4 实现 | `openspec/specs/d3-llm-gateway/spec.md` REWRITE | 1060 → **≤ 200** |
| S4 实现 | `openspec/specs/d3-llm-gateway/CHANGELOG.md` NEW | 0 → **≤ 300** |
| S5 | `acceptance-report.md`（12 AC） | ~80 |
| S6 | PR + auto-merge + S6-archive PR + auto-merge | 2 PR |

### 3.3 spec.md 8 段结构（与 d2/d1 对齐）

1. **Overview** — D3 LLM 网关 公共域 + 5 承诺 C1-C5 + 1 横切 + Bridge 跨域锚点
2. **核心设计原则** — 7-8 条（承诺装置 + 跨域锚点 + Tier 解析 + 灰区声明 + 启动 fail-fast + Breaker/Retry 合并 + 运行时 span 保持 + BREAKING 显式）
3. **S 层职责**（canonical 6 + 1 CROSS）— D3-S1 RouteModel / D3-S2 StreamChat / D3-S3 ProtectCall / D3-S4 BudgetTokens / D3-S5 GuardContent / D3-S6 ConfigureGateway / D3-X CROSS
4. **DSAFT 结构** — 1 D + 7 S (6 canonical + 1 CROSS) + A + F + T + Span ops 计数
5. **Scenarios**（6 canonical S 状态表 + 90 Scenario 5 类分布）
6. **Architecture** — Adapter / Gateway / Breaker/Retry/Budget/Safety/Config + bridges/llm 引用
7. **关键 Scenario 范式** — 1 canonical: D3-S3 ProtectCall Breaker Open（最有特色场景）
8. **关键链路口** — 4-6 端到端路径

### 3.4 CHANGELOG.md 时间线（6+ d3 change）

最近 30 天 d3 change：
- 2026-06-30 devrix-d3-spec-lite（本 change）
- 2026-06-29 devrix-d3-dsaft-restructuring
- 2026-06-14 devrix-d3-sa-refine-v2.0
- 2026-06-14 devrix-d3-sa-refine-v1.1
- 2026-06-14 devrix-d3-sa-refine
- 2026-06-08 devrix-llm-gateway-v2
- 2026-06-07 devrix-llm-gateway

**总计**：7 条 d3 change

## 4. AC 总结

12 AC（详见 demand.md §3）：
- AC1 spec.md ≤ 200
- AC2 8 段契约
- AC3 1-2 canonical Gherkin
- AC4 CHANGELOG.md ≤ 300 + ≥ 6 d3 change
- AC5 0 子文件
- AC6 0 Go diff
- AC7 12 d3 子文档 0 diff
- AC8 90 Scenario 留 archive + distribution summary
- AC9 规范对其他域立即生效
- AC10 verify-archive.sh PASS
- AC11 demand-archive-index.md 追加
- AC12 verdict ACCEPTED

## 5. 不动文件验证

| 类别 | 文件数 | 验证 |
|------|--------|------|
| d3 其他 12 个子文档 | 12 | `git diff --name-only -- openspec/specs/d3-llm-gateway/` 仅 spec.md + CHANGELOG.md |
| d1/d2/d4/d5/d6/d7 spec.md | 6 | `git diff --stat openspec/specs/d{1,2,4,5,6,7}-*/` = 0 |
| Go 代码 | — | `git diff --stat internal/` = 0 |
| 项目级规范 | — | `git diff --name-only -- openspec/specs/project/` = 0 |

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 90 Scenario 详细文本丢失追溯 | 跨 archive/ 全局 grep `#### Scenario:` 总数 = 90 校验 |
| d3 canonical S = 6 + 1 CROSS 比 d2 (4) / d1 (6) 多/略多 | 表格展示；不增加 spec.md 复杂度 |
| reviewer 担心 d3 spec 引用过多（d3-domain + dsaf-architecture + observability-guide + terminal-state-guide + model-resolution-trace + span-registry）| spec.md 顶部 SoT 引用段统一声明 |
| d3-design.md 1042 行未精简 | Backlog 立项 `devrix-d3-design-split`；本 change 不动 |

## 7. Open Questions

无（复用 d7/d2/d1 lite-mode 全验证模式）

## 8. 关联引用

- devrix-spec-lite-mode (DM-20260630-003, s7_archived) — lite-mode 规范源头
- devrix-d2-spec-lite (DM-20260630-004, s7_archived) — lite-mode 推广第一站
- devrix-d1-spec-lite (DM-20260630-005, s7_archived) — lite-mode 推广第二站
- d3 canonical S = 6 (5 承诺 + 1 横切) + D3-X CROSS 跨域
- d3 6 archive 目录：v1 / v2 / sa-refine / v1.1 / v2.0 / dsaft-restructuring
- d3 域 SoT: `d3-llm-gateway/d3-domain.md` v1.6.0 (DM-20260629-003 收口，未触碰)
