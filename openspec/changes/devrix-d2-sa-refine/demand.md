---
demand-id: DM-20260614-009
title: D2 Context Engine — 执行原语价值流重构与 D7 边界收口
priority: P0
status: S5_Accepted
dsaft_domain: context-engine
created: 2026-06-14
gaming_analysis: gaming-analysis.md
---

# D2 Context Engine — 执行原语价值流重构与 D7 边界收口

## 1. 背景

### 1.1 D2 根本目标（领域 North Star）

**在会话边界内，可靠地准备上下文、执行 LLM↔Tool 多轮循环，并持久化会话状态——作为被 D7 调度的纯执行原语（Follower），不承担编排决策。**

用户 / 系统侧可验证承诺：

1. **准备**：会话可加载/恢复；工具链可修复；超长对话可压缩；System Prompt 可组装
2. **执行**：QueryLoop 多轮 tool_use 有序、可取消、可观测
3. **持久化**：快照与 transcript 在 turn 结束后 durable；`complete` 延迟到写入完成
4. **约束**：权限/沙箱/Plan 写限制在工具执行前生效
5. **边界**：D2 **不**决定「做什么、谁来做、按什么顺序做」（归 D7）；**不**拥有 IM 信号语义（归 D1）

### 1.2 现状问题

| 问题 | 根因 |
|------|------|
| D2-S1–S14 按 **Go 包/module** 切 S，非用户/系统价值流 | S 被目录结构绑架（Playbook 原则 1 违反） |
| D2-S10 **膨胀**：QueryLoop + TaskTools + Permission + SubQuery + Attachments 同 S | 单 S 承载编排 + 执行混合语义 |
| Task 写模型、PlanMode/PlanAgent 在 `contextengine/tasks/` | 跨域边界漂移（应归 D7-S1/S5） |
| `delegate_tools.go`、`queue/` delegate-progress 在 D2 | 编排关注点渗入执行域 |
| D2 Loop 仍含 Hooks/SessionQueue 等 D7 编排字段 | 「D2 Thin」目标未在规格层闭合 |
| Legacy Harness（S9）与 Canonical 主路径（S10）同级展示 | 稳定性梯度未在 S 层表达 |

### 1.3 D2 博弈定位

> **D2 = Execution Follower（Stackelberg Follower）**：在 D7 Leader 给定执行参数后，保证 LLM↔Tool 回合**机制正确**（顺序、持久化、权限）；**不**保证任务结构正确或结论质量（归 D6 Judge）。

| D2 承诺（v1.0 可验证） | 博弈含义 |
|----------------------|----------|
| QueryLoop turn 有序 | 执行路径 commitment device |
| `complete` 延迟到持久化后 | costly signal（durable 才宣告完成） |
| Permission 先于 tool execute | 机制约束，非质量评判 |
| SubQuery depth / read-only 约束 | 嵌套执行边界 |

| 非 D2 职责 | 归属 |
|-----------|------|
| 意图分类 / 任务图合成 | D7-S5 |
| Task 写模型 / WorkPlan 读模型 | D7-S1/S4 |
| Wave 并行调度 | D7-S3 |
| delegate_* 路由到 D4 | D7-S2/S5 |
| IM 进度展示 | D1-S15 + D7-S4 |

完整分析见 [`gaming-analysis.md`](gaming-analysis.md)。

## 2. 问题陈述

| # | 问题 | 影响 |
|---|------|------|
| P1 | S 层按 module 切分，无法回答「执行前/中/后」价值流 | 可扩展性 / onboarding 困难 |
| P2 | D2-S10 混合执行 + 编排 + Task 语义 | 变更 D7 时误伤 D2 T |
| P3 | 跨域代码（tasks/, delegate_tools）无 Out of Scope 声明 | 边界漂移持续 |
| P4 | Legacy S9 Harness 与 Canonical 主路径同级 | 新人误判主路径 |
| P5 | 无 Canonical S→Legacy Module 双轨表 | IMPLEMENTED T 无法安全迁移注释 |

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | D2 价值流 S 采用 **切法 A**：S15–S20 六场景注册完整；旧 S1–S14 标记 Legacy Module Index 冻结 | P0 |
| AC2 | North Star + Out of Scope（D7/D1/D4/D6）在 proposal/design 显式声明 | P0 |
| AC3 | 每个 Canonical S 至少 1 个 Gherkin Scenario（happy + sad 合计 ≥2 跨域） | P0 |
| AC4 | IMPLEMENTED T 通过 canonical→legacy 列追溯；v1.0 不要求改测试 `// T:` 注释 | P0 |
| AC5 | 跨域漂移清单（tasks/, delegate_tools, queue delegate-progress）登记迁移目标 D7 | P0 |
| AC6 | `code-layout.md` §4.3 补充 S15–S20 scenario-slug 目标路径 | P1 |
| AC7 | S3-Gate Approved；v1.0 无 Go 代码变更 | P0 |
| AC8 | D2 Thin QueryLoop 在 design 定义 A/F 边界（Hooks/Queue 归 D7 注入） | P0 |
| AC9 | `d7-boundary.md` 跨域 SoT + D7 `d7-domain.md` 双向引用 | P0 |

### 分阶段终态

| 版本 | 范围 | 风险 |
|------|------|------|
| v1.0 Registry | S15–S20 + Legacy 双轨 + Gherkin + 跨域清单 | 低 |
| v1.1 Traceability | Span 对齐 Canonical S；D7 边界 stub 测试 | 中 |
| v2.0 Structure | tasks/ → D7；loop 瘦身；scenario 物理路径 | 高 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `docs/methodology/dsaft-methodology.md`、`dsaft-refactoring-playbook.md` |
| 依赖 | D7 v2.3.0（D1→D7 ingress、D2 Thin 目标） |
| 依赖 | DM-20260614-008 D7-SA-Refine（Follower/Leader 模型） |
| 约束 | **切法 A**；新 S 号段 **D2-S15–S20**；旧 S1–S14 不重定义语义 |
| 约束 | v1.0 registry-only；不改 Go 代码 |

## 5. 变更范围

### 新增

- D2-S15–S20 Canonical Scenario 注册表
- `openspec/specs/d2-context-engine/d2-domain.md`
- `openspec/specs/d2-context-engine/d7-boundary.md`
- `devrix-d2-sa-refine/` change 包（demand + proposal + design + gaming-analysis）

### 修改

- `openspec/specs/architecture/layering.md` D2 双轨表
- `openspec/specs/architecture/code-layout.md` §4.3
- `openspec/specs/d2-context-engine/{a,f,t}-registry.md` canonical 列

### 不变更

- v1.0 Go 代码与现有 `// T:` 测试注释
- D2-S10 QueryLoop 运行时行为
- 已 IMPLEMENTED T 的测试实现

## 6. 风险评估

| 风险 | 缓解 |
|------|------|
| 双轨 S 表认知负担 | layering 明确：SoT 价值流 = S15–S20；S1–S14 仅追溯 |
| 与 D7 Task 迁移冲突 | v1.0 仅登记 Out of Scope；v2.0 与 D7 v2.0 联动 |
| S10 大量 T 需重映射 | Legacy 双轨 + canonical 列，不强制改号 |

## 7. 关联需求

| Demand ID | 标题 | 关系 |
|-----------|------|------|
| DM-20260614-008 | D7 S 层价值流重构 | Leader/Follower 模型；D2 Thin 前提 |
| DM-20260614-007 | D1 入站仅路由 D7 | D2 不再被 D1 直接调用 |
| DM-20260611-004 | Harness 退役 / QueryLoop 主路径 | S20 LegacyHarnessFallback |
| DM-20260612-011 | Unified Task Registry | Task 写模型 v2.0 迁入 D7 |
