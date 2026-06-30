# D3 LLM Gateway — Changelog

> **时间线列表（Lite-Mode）**。每个 change 一行 + 一句话摘要 + 链接到 `archive/`。
>
> - **spec.md 详细 Scenario 演进** = 在 `archive/<change>/specs/` 各 change 目录
> - **当前符合代码的设计契约** = [spec.md](spec.md)（v3.2.0，≤ 200 行）
> - **域 SoT** = [d3-domain.md](d3-domain.md)（v1.6.0，North Star + 5 承诺 + DSAFT 资产 + 边界 SoT）
> - **变更类型说明** = IMPLEMENTED（已合入代码）/ PARTIAL（部分合入）/ SUPERSEDED（被替代）/ OBSOLETE（已废弃）
> - **最近 30 天** = 2026-05-31 ~ 2026-06-30，共 7 条 d3 change（含本 change）

---

## 时间线（最近 30 天）

| Date | Change ID | 摘要 | 状态 | 归档 |
|------|-----------|------|------|------|
| 2026-06-30 | devrix-d3-spec-lite | d3 spec.md 1060→149 lite-mode (12 AC) | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d3-spec-lite/) |
| 2026-06-29 | devrix-d3-dsaft-restructuring | d3 v3.2.0 收口（DM-20260629-003）：8 PR squash→3 PR，90 Scenario gherkin-restructuring + 4 boundary debt RESOLVED + d3-domain.md v1.6.0 NEW | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d3-dsaft-restructuring/) |
| 2026-06-14 | devrix-d3-sa-refine-v2.0 | D3 v3.0.0 S/A 重切（DM-20260614-016）— 7 技术角色词 → 5+1 价值流承诺（S1-S6）+ §Legacy Archive 100% alias 追溯 | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d3-sa-refine-v2.0/) |
| 2026-06-14 | devrix-d3-sa-refine-v1.1 | D3 v3.1 韧性可见性 + 评测探针 + 适配扩展（DM-20260614-017）— 9 个新增 F (30 域内) + BREAKING `IAdapter.Protocol() string` + 3 metric + 1 span event + 3 EngineEvent | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d3-sa-refine-v1.1/) |
| 2026-06-14 | devrix-d3-sa-refine | D3 V3 切法 5+1 价值流承诺装置（DM-20260614-016）— North Star 5 承诺 + 1 横切 + Breaker/Retry 合并到 S3 ProtectCall | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d3-sa-refine/) |
| 2026-06-08 | devrix-llm-gateway-v2 | D3 V2 Reliability 继承（DM-20260608-002）— Provider 模型路由 + Retry/Fallback + Token 预算 + Safety 前置过滤 | IMPLEMENTED | [archive](../../archive/2026-06-08-devrix-llm-gateway-v2/) |
| 2026-06-07 | devrix-llm-gateway | D3 V1 初版（DM-20260607-004）— LLM Gateway 基础架构 + Adapter pattern + OpenAI SSE chunk | IMPLEMENTED | [archive](../../archive/2026-06-07-devrix-llm-gateway/) |

---

## 历史归档（早于 30 天）

如需查阅 30 天前的 d3 历史，访问 `openspec/archive/` 目录，命名格式 `YYYY-MM-DD-devrix-{name}`。`openspec/demand-archive-index.md` 包含全部归档记录的元信息（Demand ID / 标题 / 归档日期 / PR / Verdict）。

---

## 状态映射（spec.md 索引）

| spec.md 段 | 描述 | 对应 archive 历史 |
|-----------|------|-----------------|
| Overview | D3 公共域 + 5+1 价值流承诺 + D7 主消费方 + D2→D3 ban | devrix-d3-dsaft-restructuring + devrix-d3-sa-refine |
| 核心设计原则 | 8 条（承诺装置 / D7 caller / D2 ban / Tier 二阶段 / S5 vs S18 灰区 / Breaker 合并 / fail-fast / span 稳定） | devrix-d3-sa-refine + devrix-d3-dsaft-restructuring |
| S 层职责 | canonical D3-S1..S6 + D3-X CROSS | devrix-d3-sa-refine + devrix-d3-dsaft-restructuring |
| DSAFT 结构 | 1 D + 7 S + 6 A + 30 F + 35 T + 5 Span ops | devrix-d3-dsaft-restructuring + devrix-d3-sa-refine-v1.1 |
| Scenarios | 6 canonical S + 1 CROSS 状态表 + 90 Scenario 5 类分布 | devrix-d3-dsaft-restructuring（90 Scenario 详细） |
| Architecture | Adapter / StreamChat / Breaker / Budget / Safety / Config + bridges/llm 引用 | devrix-d3-dsaft-restructuring + devrix-d3-sa-refine-v1.1 |
| 关键 Scenario 范式 | 1 canonical：D3-S3 ProtectCall Breaker Open 路径 | devrix-d3-dsaft-restructuring PR-7（90 Scenario 详细） |
| 关键链路口 | 6 端到端路径（含 Hard Ban 链 + 内容守卫灰区链） | 全部 archive change 累积形成 |

---

## 90 Scenario 分布（DM-20260629-003 PR-7+#8 gherkin-restructuring）

| 类别 | 数量 | 占比 |
|------|------|------|
| happy | 30 | 33% |
| sad | 24 | 27% |
| boundary | 18 | 20% |
| concurrent | 9 | 10% |
| timeout | 9 | 10% |
| **总计** | **90** | **100%** |

**详细 90 Scenario 文本**：在 `openspec/archive/2026-06-29-devrix-d3-dsaft-restructuring/specs/`（PR-7+#8 落地）。

---

## 维护规则

- **新增 change 时**：归档时（`changes/<id>/` → `archive/<date>-<id>/`）追加一行，按 `Date | Change ID | 摘要 | 状态 | 归档` 格式
- **架构级变更时**：修订 [spec.md](spec.md) 主体段（Overview / 核心原则 / S 层职责 / 关键 Scenario 范式）+ [d3-domain.md](d3-domain.md)（North Star / 5 承诺 / DSAFT 资产）+ 4 boundary debt 决议登记
- **超 300 行时**：精简为一行摘要 + 归档链接；超期条目（> 30 天）折叠到「历史归档」段
- **禁止**：复制 Requirement/Scenario 详细文本到本文件；创建子文件（lite-mode 不需要）
