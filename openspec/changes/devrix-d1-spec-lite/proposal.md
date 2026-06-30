# Proposal: d1 通信层 spec 精简

**Change ID:** devrix-d1-spec-lite
**Demand ID:** DM-20260630-005
**Status:** S2_Design
**Follows:** devrix-d2-spec-lite (DM-20260630-004, s7_archived)

## 1. Background

lite-mode 模式（DM-20260630-003）已 S7_Archived 规范化，并在 d7（PR #333+#334）+ d2（PR #336+#337）两站验证完成。d1 spec.md 当前 577 行，90 个 Scenario 详细文本累积（DM-20260629-005 PR-6 gherkin-restructuring 18 缩写 bullet → 90 展开）。

本 change 是 **lite-mode 推广第二站**。d1 是 backlog 4 个候选中**最小目标**（仅次于 d4 spec.md 222 行），收益高 + 风险低。

## 2. Problem Statement

d1 spec.md 累积了从 V1 到 V6 (DM-20260630-005) 完整设计 + 90 Scenario 详细文本：

| 维度 | 现状 | 期望 |
|------|------|------|
| d1 spec.md 行数 | 577 | ≤ 200 |
| Scenario 数 | 90（PR-6 gherkin-restructuring 落地） | spec.md 中 1-2 canonical 范式 |
| 切法 A 信号分层 | 4 概念（Separating/Costly/Commitment/Screening） | 1 段引用 d1-domain.md §North Star |
| Boundary Debt Decisions | 3 row（DM-20260629-005 PR-7） | 引用 d7-boundary.md（v1.2.0 NEW） |
| 跨域漂移 | D1-D7 Ingress + D1-D4 Permission Gate + D1-D5 Observability | 1 段"跨域消费模型" |

## 3. Proposed Solution

### 3.1 总体策略

**复用 lite-mode 模式**（与 d7 / d2 spec-lite 同形）：

```
openspec/specs/d1-communication/
  spec.md         ≤ 200 行   当前符合代码的设计契约（v6.0.0）
  CHANGELOG.md    ≤ 300 行   时间线列表（6+ d1 change，最近 30 天）
  d1-domain.md    不变       v1.2.0 D1-Domain SoT（DM-20260629-005 收口）
  其他 11 个子文档 不变       a/t/f-registry / design / d7-boundary / span-registry / dsaf-architecture / observability-guide / terminal-state-guide / layer-delta / feishu-task-planning-verification
```

**过程需求**（90 Scenario 详细文本）**留在 archive/ 各 change 目录**，不进入 specs/。

### 3.2 拆分粒度决策

**方案对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 复用 d2 lite-mode（推荐） | 一致性；复用已验证模式；d2 1 站到位 | 仍需 1 跳到 archive/ |
| B. 按 S 分片（spec-s{13..18}.md） | S 层清晰 | 子文件持续累积；与 lite-mode 反模式 |
| C. 维持 577 行不拆分 | 0 改动 | 违反 lite-mode 硬上限（200） |

**选择:** A（复用 d2 lite-mode）

**理由：**
- d2 spec-lite (DM-20260630-004) 1 PR 1 站到位验证（PR #336 + #337）
- 用户原始诉求 = specs 域文档只放最新符合代码的设计
- 90 个 Scenario 详细文本对当前开发是噪音（已合入代码，需求态已转为代码态）
- d1-domain.md v1.2.0 已提供 North Star / DSAFT 资产 / 边界 SoT，spec.md 引用即可

### 3.3 d1 spec.md 新结构

```markdown
# D1 Communication Domain Specification

> 当前符合代码的设计契约（v6.0.0）。详细 Scenario 历史见 [CHANGELOG.md](CHANGELOG.md) 与 [archive/](../../archive/)。

## Overview
（3-5 段：d1 域职责 / Trusted Intermediary / 入站+3 类出站信号+多通道+弱网必达 / 上下游接口 / D7 唯一编排入口）

## 核心设计原则
（5-8 条 bullet：Trusted Intermediary / 信号分层博弈论 / 3 类出站信号 / 弱网必达 / Permission + YOLO / Card 系统 / EventBus 5 态 / Hard Ban D1→D2）

## S 层职责（canonical S13-S18）
（表格：D1-S13 CaptureUserIntent / S14 PresentThinking / S15 PresentTaskProgress / S16 DeliverConclusion / S17 ConnectChannel / S18 GuaranteeDelivery；S1-S12 RETIRED）

## DSAFT 结构
（表格：D=1, S=6, A=16, F=18, T=74 IMPLEMENTED；Span 22 ops）

## Scenarios
（表格：6 canonical S 状态）

## Architecture
（Gateway-Adapter + EventBus 5 态 + Permission YOLO + CardKit 流式 + 跨域边界 d7-boundary.md 引用）

## 关键 Scenario 范式
（1-2 canonical：S13 CaptureUserIntent 入站飞书消息持久化）

## 关键链路口
（4-6 端到端：User → D1 IM Adapter → CommunicationGateway → D7 ProcessMessage / D1 → D5 Observability / D1 → D4 Permission / D1 Critical 必达）
```

行数目标：170-200 行。

### 3.4 d1 CHANGELOG.md 新结构

```markdown
# D1 Communication — Changelog

> 时间线列表。每个 change 一行 + 一句话摘要 + 链接到 archive/。

| Date | Change ID | 摘要 | 归档 |
|------|-----------|------|------|
| 2026-06-30 | devrix-d1-spec-lite | d1 spec.md 577→200 lite-mode (12 AC) | [archive](../archive/2026-06-30-devrix-d1-spec-lite/) |
| 2026-06-30 | devrix-d1-ac-restructuring | 18 缩写→90 Scenario + d7-boundary.md NEW | [archive](../archive/2026-06-30-devrix-d1-ac-restructuring/) |
| ... | ... | ... | ... |
```

行数目标：30-50 行（d1 历史 6 个 archive 目录，比 d2/d7 短）。

## 4. Success Metrics

| 指标 | 目标 | 测量 |
|------|------|------|
| d1 spec.md 行数 | ≤ 200 | `wc -l` |
| d1 CHANGELOG.md 行数 | ≤ 300 | `wc -l` |
| spec.md Scenario 范式数 | 1-2 个 | `grep -c "^#### Scenario:"` |
| CHANGELOG.md change 条目数 | ≥ 6（最近 30 天 d1 changes） | `grep -c "^|" CHANGELOG.md` |
| 90 个原 Scenario 全部留 archive | 90 = 90 | 跨 archive/ 全局 grep |
| 不动 Go 代码 | 0 diff | `git diff --stat internal/` |
| 不动 d1 其他 12 个子文档 | 0 diff | `git diff --stat openspec/specs/d1-communication/`（除 spec.md + 新增 CHANGELOG.md） |
| S5 验收 verdict | ACCEPTED | acceptance-report.md |

## 5. Implementation Plan

| 阶段 | 产出 | 门禁 |
|------|------|------|
| S3 设计 | design.md（六段式 + 新旧方案对比 + rollback） | S3-Gate 通过 |
| S4 实现 | d1 spec.md 精简（577 → ≤ 200）+ d1 CHANGELOG.md + .openspec.yaml + demand/proposal/design/tasks.md | go vet / wc -l 检查 |
| S5 验收 | acceptance-report.md (verdict: ACCEPTED) + 90 Scenario 全部留 archive 验证 | AC1-AC12 全过 |
| S6-交付 | PR 合入 master (squash + auto-merge) | CI 全绿 |
| S6-归档 | mv changes/devrix-d1-spec-lite/ → archive/2026-06-30-devrix-d1-spec-lite/ + demand-archive-index.md 追加 + .openspec.yaml s7_archived + 新 PR | verify-archive.sh PASS |

**单 PR 一次到位**：本次 PR 包含 S1+S2+S3+S4+S5 全部 commit，最后 S6-归档独立 PR。

## 6. Risks & Mitigations

详见 `demand.md` §6。补充：

- **风险**: d1 spec.md 删 377 行（90 Scenario 详细文本），Reviewer 担心历史追溯
- **缓解**: d1 历史 6 个 archive 目录已覆盖（d1-ac-restructuring 含 gherkin-restructuring 完整 90），CHANGELOG.md 通过 change-id 链接引用
- **风险**: 切法 A 信号分层博弈论（Separating/Costly/Commitment/Screening）丢失
- **缓解**: spec.md 1 段"信号分层" + 引用 d1-domain.md §North Star 的 6 个 ValueFlow Alias
- **风险**: 90 个 Scenario 中 5 类平衡（happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）丢失
- **缓解**: CHANGELOG.md 1 行说明分布 + spec.md canonical 选 happy 路径

## 7. Out of Scope

- d1 design.md (527 行) 拆分 — Backlog 单独立项
- d1 t-registry.md (200 行) / a-registry.md (180 行) 拆分 — Backlog
- d3/d4 spec.md 同步精简 — 视需求后续，每个域独立 change 立项
- CI 工具 `verify-spec-links.sh` — 后续单独立项
- d1-domain.md 内容修订 — v1.2.0 已收口，本 change 不动

## 8. Reference

- `openspec/specs/d1-communication/spec.md`（577 行原版，重写为精简版）
- `openspec/specs/d1-communication/d1-domain.md` v1.2.0（D1-Domain SoT，不变）
- `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/`（d1 v6.0.0 收口 + 90 Scenario 详细）
- `openspec/archive/2026-06-30-devrix-spec-lite-mode/`（lite-mode 规范归档）
- `openspec/archive/2026-06-30-devrix-d2-spec-lite/`（d2 推广验证，1 PR 1 站到位）