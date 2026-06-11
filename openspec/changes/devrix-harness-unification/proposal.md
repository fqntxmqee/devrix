# Proposal: Legacy Harness 退役 — QueryLoop 为唯一主路径

**Change ID:** devrix-harness-unification
**Demand ID:** DM-20260611-004
**Status:** S2_Proposal
**Priority:** P1

## 1. Background

DM-012 QueryLoop v6 已于 2026-06-10 交付，并在生产配置 `query_loop.enabled: true` 下成为 LLM↔Tool 主路径。但 ContextEngine `engine.go` 仍保留 `harnessEnabled` legacy 分支（7+ 处：压缩/bootstrap/preflight），与 QueryLoop 行为漂移。QueryLoop 落地后 Harness 已降级为"legacy fallback"，不再是等权第二主路径。

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| 新特性同步两路径 | QueryLoop 已改、Harness 未改 → 行为不一致 |
| 测试覆盖分裂 | `query_loop.enabled` true/false 两套套件 |
| `harnessEnabled` 条件扩散 | 压缩、权限、事件路径不一致 |
| 维护成本 | 7+ 处条件分支需同步理解 |

## 3. Proposed Solution

### 3.1 阶段 1：标记废弃 + 默认 QueryLoop

- `config.ContextEngine` 默认 `query_loop.enabled = true`（不依赖配置文件）
- `engine.go` 中 `harnessEnabled && !workerLocal` 分支加 `// #deprecated: legacy fallback, will be removed in v2.0` 注释
- 行为不变（双路径仍存在），但代码意图清晰

### 3.2 阶段 2：统一压缩入口

- QueryLoop 迭代前走 messages-only 七步管道（已存在）
- 删除 harness 专用压缩分叉（位于 `contextengine/compression/pipeline.go`）
- 验证：所有 `harnessEnabled` 触发的压缩路径改为 QueryLoop 路径

### 3.3 阶段 3：PathRegressionProbe

- D5 指标 `runtime.path_resolved_total{path="query_loop|legacy_harness"}` Counter
- D6 注册 `PathRegressionProbe`：
  - 测量 baseline（启动时 query_loop=1, legacy_harness=0）
  - CI 阻断 legacy_harness > 0
- 集成测试：运行 100 个 query 循环，确认 legacy 计数为 0

### 3.4 阶段 4：迁移 tech-debt TD-QL-01~03

- `tech-debt/queryloop-error-recovery.md` 中 TD-QL-01~03（与 Harness 退役同 PR 系列可合并）
- TD-QL-01: QueryLoop 413/fallback 错误恢复
- TD-QL-02: 取消传播一致性
- TD-QL-03: tombstone 清理

### 3.5 阶段 5（未来）：完全删除

- v1.1: `harnessEnabled` 配置项改为 no-op + WARN 日志
- v2.0: 删除 dead code + legacy-only 测试

## 4. Success Metrics

| 指标 | 基线 | 目标 |
|------|------|------|
| `query_loop.enabled` 默认值 | 取决于配置 | true |
| Harness 路径生产触发次数 | 1+ | 0 |
| `engine.go` 中 `harnessEnabled` 出现处 | 7+ | ≤ 1（仅 deprecation comment） |
| 压缩入口数 | 2 | 1 |
| PathRegressionProbe 注册 | N/A | 完成 |

## 5. Implementation Plan

| Phase | 内容 | 估时 |
|-------|------|------|
| P1 | query_loop.enabled 默认 true + harnessEnabled 注释标记 | 0.5d |
| P2 | PathRegression 集成测试 + D5 指标 + D6 探针 | 1d |
| P3 | 统一压缩入口（删除 harness 分叉） | 1d |
| P4 | tech-debt TD-QL-01~03 迁移 | 1d |
| **Total** | | **3.5d** |

**合并策略**：1 个主 PR（query_loop 默认 + 注释 + D5/D6 探针）+ 1 个清理 PR（统一压缩 + TD 迁移）。

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 仍使用 `query_loop.enabled=false` 的部署 | 配置 WARN 日志（v1.1）；文档 migration guide |
| Harness Bootstrap 删除影响 Session 冷启动 | 保留为 Session init Hook（仅 bootstrap 阶段） |
| 统一压缩入口回归 | 完整 PathRegression 测试套件先到位 |
| TD-QL 迁移 scope 蔓延 | 限定 TD-QL-01~03，其它 tech-debt 留独立 PR |

## 7. Out of Scope

- QueryLoop v3 能力新增（v6 已交付足够）
- 整体重写 harness 路径
- 删除 `harnessEnabled` 配置项（v1.1）
- 删除 legacy 路径代码（v2.0）
