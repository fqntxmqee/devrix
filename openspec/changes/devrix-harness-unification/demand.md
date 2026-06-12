---
demand-id: DM-20260611-004
title: Legacy Harness 退役 — QueryLoop 为唯一主路径
source: devrix-harness-architecture-audit（2026-06-11 修订）
priority: P1
status: S2_Revised
revised: 2026-06-11
dsaft_domain: context-engine
created: 2026-06-11
---

# Legacy Harness 退役

## 1. 背景（修订）

2026-06-11 审计撰写时，ContextEngine 存在 harness / non-harness **等权双路径**。  
**2026-06-10 起**，DM-012 QueryLoop 已交付并在生产配置 `query_loop.enabled: true` 下成为 **LLM↔Tool 主路径**。

本需求目标从「两路径统一为第三路径」**收窄**为：

> **以 QueryLoop 为唯一主路径，逐步删除 `harnessEnabled` legacy 分支与 dead code。**

## 2. 现状（2026-06-11 代码）

| 组件 | 状态 |
|------|------|
| QueryLoop | ✅ 主路径（`runViaQueryLoop`） |
| `harnessEnabled` 分支 | ⚠️ `engine.go` 仍有 7+ 处（压缩/bootstrap/preflight） |
| Legacy PEV 固定 3 轮 | 仅 `query_loop.enabled=false` 时生效 |
| Harness Bootstrap | 可选附加层，与 QueryLoop 部分重叠 |

**根因（更新）：** Harness 在 QueryLoop 之前是「可选增强」；QueryLoop 落地后 Harness 分支成为 **legacy fallback**，而非与 QueryLoop 等权的第二主路径。

## 3. 问题陈述

### 3.1 维护成本

| 问题 | 影响 |
|------|------|
| 新特性需同步 harness 分支 | QueryLoop 已改、Harness 未改 → 行为漂移 |
| 测试覆盖分裂 | `query_loop.enabled` true/false 两套套件 |
| `harnessEnabled` 条件扩散 | 压缩、权限、事件路径不一致 |

### 3.2 非目标

- **不**再要求「HarnessBootstrap 内联为 QueryLoop 组成部分」—— Bootstrap 可保留为 Session 初始化 Hook
- **不**重复 DM-012 已交付的 QueryLoop 能力

## 4. 验收标准（修订）

### P0

- [ ] `query_loop.enabled` 默认为 `true`，文档声明为唯一支持的主路径
- [ ] 删除或 `#deprecated` 标记 `engine.go` 中 `harnessEnabled && !workerLocal` 的 **PEV/压缩双路径** 分支
- [ ] Legacy 路径删除前：PathRegression 测试覆盖原 harness 分支的关键行为（压缩触发、权限、complete 事件）

### P1

- [ ] 统一压缩入口：QueryLoop 迭代前走 messages-only 七步管道；删除 harness 专用压缩分叉
- [ ] 迁移 tech-debt `queryloop-error-recovery.md` 中 TD-QL-01~03（与 Harness 退役同 PR 系列可合并）
- [ ] `PathRegressionProbe` 注册 D6 Eval：旧路径调用计数 → 0

### P2

- [ ] 移除 `harnessEnabled` 配置项或改为 no-op + WARN 日志
- [ ] 删除 dead code 与仅 legacy 路径使用的测试

## 5. 依赖与顺序

```
DM-012 QueryLoop（已归档）→ 本需求 → DM-003 EventChannel（可选并行）
tech-debt/queryloop-error-recovery.md 可并入 Phase 2 PR
```

## 6. 回归风险

- 仍使用 `query_loop.enabled=false` 的部署需迁移窗口 + 配置 WARN
- Harness Bootstrap 删除可能影响 Session 冷启动时序，需集成测试
