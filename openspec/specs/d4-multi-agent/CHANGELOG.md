# D4 Multi-Agent — Changelog

> **时间线列表（Lite-Mode）**。每个 change 一行 + 一句话摘要 + 链接到 `archive/`。
>
> - **spec.md 详细 Scenario 演进** = 在 `archive/<change>/specs/` 各 change 目录
> - **当前符合代码的设计契约** = [spec.md](spec.md)（v3.1.0，≤ 200 行）
> - **域 SoT** = [d4-domain.md](d4-domain.md)（v2.2.0，North Star + 6 ValueFlow + 边界 SoT）
> - **D7 边界契约** = [d7-boundary.md](d7-boundary.md)
> - **变更类型说明** = IMPLEMENTED / PARTIAL / SUPERSEDED / OBSOLETE
> - **最近 30 天** = 2026-05-31 ~ 2026-06-30，共 4 条 d4 change（含本 change）

---

## 时间线（最近 30 天）

| Date | Change ID | 摘要 | 状态 | 归档 |
|------|-----------|------|------|------|
| 2026-06-30 | devrix-d4-spec-lite | d4 spec.md 222→155 lite-mode (12 AC) | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d4-spec-lite/) |
| 2026-06-29 | devrix-d4-dsaft-restructuring | d4 v3.1.0 收口（DM-20260629-004）：8 PR squash→3 PR，ValueFlow rename + 3 boundary debt RESOLVED + d4-domain.md v2.2.0 + d7-boundary.md 升级 | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d4-dsaft-restructuring/) |
| 2026-06-15 | devrix-d4-sa-refine | D4 V3 S11-S16 价值流 + Hub-Spoke 迁 D7（DM-20260614-018）— Legacy S1-S10 冻结双轨 | IMPLEMENTED | [archive](../../archive/2026-06-15-devrix-d4-sa-refine/) |
| 2026-06-08 | devrix-multi-agent | D4 V1 初版（DM-20260608-005）— Multi-Agent Layer 基础 + Fork/Join + 协作模式 | IMPLEMENTED | [archive](../../archive/2026-06-08-devrix-multi-agent/) |

---

## 历史归档（早于 30 天）

如需查阅 30 天前的 d4 历史，访问 `openspec/archive/`，命名格式 `YYYY-MM-DD-devrix-{name}`。

---

## 状态映射（spec.md 索引）

| spec.md 段 | 描述 | 对应 archive 历史 |
|-----------|------|-----------------|
| Overview | D4 Delegation Execution Follower + Hub-Spoke 归 D7 | devrix-d4-sa-refine + devrix-d4-dsaft-restructuring |
| 核心设计原则 | 8 条（Follower / 6+10 双轨 / COW / Permission Gate / Worker no-delegate / const switch emit / fail-fast / Sub-Agent Mode 3-mode） | devrix-d4-sa-refine + devrix-d4-dsaft-restructuring + devrix-context-budget-phase-b |
| S 层职责 | canonical D4-S11..S16 + Legacy D4-S1..S10 | devrix-d4-sa-refine |
| DSAFT 结构 | 1 D + 6 S (canonical) + 10 S (Legacy) + 6 A + F + 38 T + 5 Span ops | devrix-d4-dsaft-restructuring + devrix-d4-sa-refine |
| Scenarios | 6 canonical S 状态表 | devrix-d4-dsaft-restructuring（canonical Gherkin 详细文本） |
| Architecture | Hub-Spoke 编排归 D7 + AgentEvent const switch | devrix-d4-dsaft-restructuring + devrix-d4-sa-refine |
| 关键 Scenario 范式 | 1 canonical：D4-S14 ExecuteWorker fork→run→join | devrix-d4-dsaft-restructuring |
| 关键链路口 | 6 端到端路径（含 Hard Ban 链 + Worker no-delegate 链） | 全部 archive change 累积 |

---

## 维护规则

- **新增 change 时**：归档时（`changes/<id>/` → `archive/<date>-<id>/`）追加一行，按 `Date | Change ID | 摘要 | 状态 | 归档` 格式
- **架构级变更时**：修订 [spec.md](spec.md) 主体段 + [d4-domain.md](d4-domain.md)（North Star + ValueFlow） + [d7-boundary.md](d7-boundary.md)（D4↔D7 边界）+ 3 boundary debt 决议登记
- **超 300 行时**：精简为一行摘要 + 归档链接；超期条目（> 30 天）折叠到「历史归档」段
- **禁止**：复制 Requirement/Scenario 详细文本到本文件；创建子文件（lite-mode 不需要）
