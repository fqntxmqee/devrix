# D6 Evolution — Changelog

> **时间线列表（Lite-Mode）**。每个 change 一行 + 一句话摘要 + 链接到 `archive/`。
>
> - **spec.md 详细 Scenario 演进** = 在 `archive/<change>/specs/` 各 change 目录
> - **当前符合代码的设计契约** = [spec.md](spec.md)（v2.5.0，≤ 200 行）
> - **域 SoT** = [d6-domain.md](d6-domain.md)（v1.0.0，North Star + 3 子系统 + DSAFT 资产 + 边界 SoT）
> - **变更类型说明** = IMPLEMENTED / PARTIAL / SUPERSEDED / OBSOLETE
> - **最近 30 天** = 2026-05-31 ~ 2026-06-30，共 4 条 d6 change（含本 change）

---

## 时间线（最近 30 天）

| Date | Change ID | 摘要 | 状态 | 归档 |
|------|-----------|------|------|------|
| 2026-06-30 | devrix-d6-spec-lite | d6 spec.md 604→151 lite-mode (12 AC) | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d6-spec-lite/) |
| 2026-06-21 | devrix-d6-evolution-review-fixes | d6 v2.4.0 deep review fixes（DM-20260621-011）：C-1 bridge 清债 + H-1 Orchestration*→Guard* rename + 6 metric orch_*→guard_* + H-2 panic→log.Fatalf + H-3 三联固化（metric+slog+errors.Join）+ 6 P0 T 点 + scripts/check-orch-rename.sh CI guard; PR-A #156 + PR-B #157 + PR-C #158 + S6 #159 | IMPLEMENTED | [archive](../../archive/2026-06-21-devrix-d6-evolution-review-fixes/) |
| 2026-06-19 | devrix-spec-sync-d6-evolution-registration | d6 物理路径同步 + d6-domain.md 新建（DM-20260619-003）：对齐 D2/D4/D5/D7 结构 | IMPLEMENTED | [archive](../../archive/2026-06-19-devrix-spec-sync-d6-evolution-registration/) |
| 2026-06-15 | devrix-d6-sa-refine | D6 SA Refine（DM-20260614-021）：S3-S5 canonical Scenario 收口 + 18 条 Requirements 详细 Gherkin 沉淀 | IMPLEMENTED | [archive](../../archive/2026-06-15-devrix-d6-sa-refine/) |

---

## 历史归档（早于 30 天）

如需查阅 30 天前的 d6 历史，访问 `openspec/archive/`，命名格式 `YYYY-MM-DD-devrix-{name}`。主要历史里程碑：
- 2026-06-08 ~ 06-10：V1.0 评测引擎基线（d6-eval / d6-eval-cli / d6-eval-phase2/3/4）
- 2026-06-14：validation-metric + d3-sa-refine-v1.1 D6 探针 #1/#2/#4 落地 + probe #3 推迟
- 2026-06-15：d5-d6-sa-refine-v2.0 物理路径迁移 + d6-sa-refine
- 2026-06-19：spec-sync-d6-evolution-registration

---

## 状态映射（spec.md 索引）

| spec.md 段 | 描述 | 对应 archive 历史 |
|-----------|------|-----------------|
| Overview | D6 演化域 + 3 子系统（Eval/Guard/Verify）+ 2 PLANNED (S1/S2) | devrix-d6-sa-refine + devrix-spec-sync-d6-evolution-registration + devrix-d6-evolution-review-fixes |
| 核心设计原则 | 8 条（Judge 双模型 / Delta 3 档 / CI 门禁 fail-closed / Guard 跨模型 / Verify fail-closed / Bridge 清债 / Guard 命名空间收敛 / 三联固化错误处理） | devrix-d6-evolution-review-fixes (v2.4.0) + devrix-d6-sa-refine + devrix-d5-d6-sa-refine-v2.0 |
| S 层职责 | canonical D6-S3+S4+S5 + v2.4 韧性 D6-S11+S12 + 2 PLANNED | devrix-d6-sa-refine + devrix-d6-evolution-review-fixes + devrix-spec-sync-d6-evolution-registration |
| DSAFT 结构 | 1 D + 5 S + 20 A + 15 F + 22 T + 10 Probe (v2.2.0: 7 + 3) | devrix-spec-sync-d6-evolution-registration + devrix-d6-evolution-review-fixes |
| Scenarios | 5 canonical S 状态表 | devrix-d6-sa-refine（canonical Gherkin 详细文本）+ devrix-d6-evolution-review-fixes |
| Architecture | 3 子系统图（Eval/Guard/Verify）+ JudgeManager + InterventionExecutor + InvariantRegistry | devrix-spec-sync-d6-evolution-registration + devrix-d6-evolution-review-fixes |
| 关键 Scenario 范式 | 1 canonical：D6-S3 Tier Resolution Probe ≥ 99% | devrix-d6-sa-refine + devrix-d3-sa-refine-v1.1 (D6 探针 #1 落地) |
| 关键链路口 | 6 端到端路径（Eval / Guard / Verify / OTel 指标 / 韧性修复 / 跨域锚点） | 全部 archive change 累积 |

---

## 维护规则

- **新增 change 时**：归档时（`changes/<id>/` → `archive/<date>-<id>/`）追加一行，按 `Date | Change ID | 摘要 | 状态 | 归档` 格式
- **架构级变更时**：修订 [spec.md](spec.md) 主体段 + [d6-domain.md](d6-domain.md)（North Star + 3 子系统）+ Boundary Debt 决议登记
- **超 300 行时**：精简为一行摘要 + 归档链接；超期条目（> 30 天）折叠到「历史归档」段
- **禁止**：复制 Requirement/Scenario 详细文本到本文件；创建子文件（lite-mode 不需要）