# D1 Communication — Changelog

> **时间线列表（Lite-Mode）**。每个 change 一行 + 一句话摘要 + 链接到 `archive/`。
>
> - **spec.md 详细 Scenario 演进** = 在 `archive/<change>/specs/` 各 change 目录
> - **当前符合代码的设计契约** = [spec.md](spec.md)（v6.0.0，≤ 200 行）
> - **域 SoT** = [d1-domain.md](d1-domain.md)（v1.2.0，North Star / 6 ValueFlow / DSAFT 资产 / 边界 SoT）
> - **D7 边界契约** = [d7-boundary.md](d7-boundary.md)（v1.2.0 NEW，D1↔D7 跨域边界 + Boundary Debt Decisions）
> - **变更类型说明** = IMPLEMENTED（已合入代码）/ PARTIAL（部分合入）/ SUPERSEDED（被替代）/ OBSOLETE（已废弃）
> - **最近 30 天** = 2026-05-31 ~ 2026-06-30，共 7 条 d1 change（含本 change）

---

## 时间线（最近 30 天）

| Date | Change ID | 摘要 | 状态 | 归档 |
|------|-----------|------|------|------|
| 2026-06-30 | devrix-d1-spec-lite | d1 spec.md 577→175 lite-mode (12 AC) | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d1-spec-lite/) |
| 2026-06-30 | devrix-d1-ac-restructuring | d1 v6.0.0 收口（DM-20260629-005）：8 PR squash→3 PR，90 Scenario gherkin-restructuring + d7-boundary.md NEW | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d1-ac-restructuring/) |
| 2026-06-28 | devrix-d1-dsaft-refactor | D1 DSAFT 边界 + Gateway 拆分 + contracts DTO + `lint-d1-imports.sh` CI 守门 | IMPLEMENTED | [archive](../../archive/2026-06-28-devrix-d1-dsaft-refactor/) |
| 2026-06-14 | devrix-d1-sa-refine | 切法 A 双轨（DM-20260614-006）— 信号分层博弈论 + Canonical SoT D1-S13–S18 + Legacy S1–S12 退役 | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d1-sa-refine/) |
| 2026-06-14 | devrix-d1-d7-only-ingress | D1→D7 唯一编排入口契约（`boundary-debt:d1-to-d7-orchestration-entry-v1.0` RESOLVED） | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d1-d7-only-ingress/) |
| 2026-06-09 | devrix-d1-d5-unit-tests | D1/D5 单元测试补全 | IMPLEMENTED | [archive](../../archive/2026-06-09-devrix-d1-d5-unit-tests/) |
| 2026-06-08 | devrix-d1-d6-testing | D1 & D6 Testing Coverage | IMPLEMENTED | [archive](../../archive/2026-06-08-devrix-d1-d6-testing/) |

---

## 历史归档（早于 30 天）

如需查阅 30 天前的 d1 历史，访问 `openspec/archive/` 目录，命名格式 `YYYY-MM-DD-devrix-{name}`。`openspec/demand-archive-index.md` 包含全部归档记录的元信息（Demand ID / 标题 / 归档日期 / PR / Verdict）。

---

## 状态映射（spec.md 索引）

| spec.md 段 | 描述 | 对应 archive 历史 |
|-----------|------|-----------------|
| Overview | D1 Trusted Intermediary + 入站+3 类出站+多通道+弱网必达 + D7 唯一编排入口 | devrix-d1-sa-refine + devrix-d1-d7-only-ingress + devrix-d1-ac-restructuring |
| 信号分层博弈论 | 4 概念（Separating/Costly/Commitment/Screening）+ ValueFlow Alias | devrix-d1-sa-refine + devrix-d1-ac-restructuring PR-5 |
| 核心设计原则 | 8 条（Trusted Intermediary / 唯一编排入口 / 3 类信号 / EventBus / Permission+ YOLO / Card / Session / Hard Ban） | devrix-d1-ac-restructuring + devrix-d1-dsaft-refactor + devrix-d1-sa-refine |
| S 层职责 | canonical S13-S18（6 个）+ S1-S12 RETIRED | devrix-d1-ac-restructuring PR-4 + devrix-d1-sa-refine |
| DSAFT 结构 | 1 D + 6 S + 16 A + 18 F + 74 T + 22 Span ops | devrix-d1-ac-restructuring PR-4 + devrix-d1-dsaft-refactor |
| Architecture | Gateway-Adapter + EventBus 5 态 + d7-boundary.md 引用 | devrix-d1-ac-restructuring PR-7 + devrix-d1-dsaft-refactor |
| 关键 Scenario 范式 | 1 canonical：S13 入站飞书消息持久化 | devrix-d1-ac-restructuring PR-6（90 Scenario 详细） |
| 关键链路口 | 6 端到端路径 | 全部 archive change 累积形成 |

---

## 90 Scenario 分布（DM-20260629-005 PR-6 gherkin-restructuring）

| 类别 | 数量 | 占比 |
|------|------|------|
| happy | 30 | 33% |
| sad | 24 | 27% |
| boundary | 18 | 20% |
| concurrent | 9 | 10% |
| timeout | 9 | 10% |
| **总计** | **90** | **100%** |

**详细 90 Scenario 文本**：在 `openspec/archive/2026-06-30-devrix-d1-ac-restructuring/specs/`（PR-6 #4 落地）。

---

## 维护规则

- **新增 change 时**：归档时（`changes/<id>/` → `archive/<date>-<id>/`）追加一行，按 `Date | Change ID | 摘要 | 状态 | 归档` 格式
- **架构级变更时**：修订 [spec.md](spec.md) 主体段（Overview / 核心设计原则 / S 层职责 / 关键 Scenario 范式）+ [d1-domain.md](d1-domain.md)（North Star / DSAFT 资产）+ [d7-boundary.md](d7-boundary.md)（D1↔D7 边界）
- **超 300 行时**：精简为一行摘要 + 归档链接；超期条目（> 30 天）折叠到「历史归档」段
- **禁止**：复制 Requirement/Scenario 详细文本到本文件；创建子文件（lite-mode 不需要）