---
demand-id: DM-20260617-001
title: D2 QueryLoop 位置错位债务显式记录与 Legacy 路径退役信号
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-17
parent_doc: openspec/tech-debt/queryloop-location.md (TD-QL-LOC)
---

# Demand: D2 QueryLoop 位置错位债务显式记录与 Legacy 路径退役信号

## 1. 背景

DM-020 (D7 Turn 编排上移) 已把编排回调、per-turn 上下文注入、Hub-Spoke drain 上移 D7；DM-20260616-002（`devrix-d7-loop-first-routing`）将 `loopFirst=true` 设为默认 ingress。

经核实，`loopFirst=true` 主路径已就绪 — `internal/layers/orchestration/turn/orchestrator.go:49` 实现 `prepare→llm→tools→persist` 状态机，`llm.go:50` 的 `GatewayInvoker.Stream` 直接调 D3 `llmgateway`。**用户提的"D7 调用 D2 获取上下文组装结果 + D7 直调 D3" 已是现状。**

唯一遗留矛盾：`loopFirst=false` legacy 路径下，`internal/layers/contextengine/query/loop.go` 的 `Loop.Run()` 仍持有"while 有 tool_use 就再来一次"的循环主逻辑。这是 PEV 时代（D2-S1）的历史产物。

`internal/shared/contracts/llm_facade.go:11` 与 `internal/layers/orchestration/turn/query_llm_caller.go:21` 两处注释自证"拆面 adapter / 拆面出口" — 现状是绕道。

完整债务分析见 `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)。

## 2. 问题陈述

| ID | 问题 | 触发频次 | 用户/Agent 痛感 |
|----|------|---------|----------------|
| P1 | `loopFirst=true` 下 D2→D3 拆面 adapter **理论上不应被调用**，但缺测试护栏，未来重构可能回归 | 长期债 | 没人能断言现状是真的 |
| P2 | `loopFirst=false` legacy 路径下 D2.QueryLoop.Run 仍跑循环，违反"D7 = 编排 Leader / D2 = 执行 Follower"边界 | 短期回滚兜底 | 边界文档 §3 第 11 行形同虚设 |
| P3 | 配置项 `query_loop.enabled=false` 无文档警告，使用者不知道这是"临时回滚" | 当前 | 用户可能误用 |
| P4 | 缺监控指标，无法量化 legacy 路径使用率 → 无法决策何时彻底删除 | 当前 | 演进时机不可见 |

## 3. 验收标准

### 3.1 P0（必须达成，否则不交付）

| ID | 标准 | 验证方式 |
|----|------|---------|
| AC1 | `loopFirst=true`（默认）下，集成测试断言 D2→D3 拆面 adapter 零调用 | 注入调用计数器；100 session × 完整 turn；counter == 0 |
| AC2 | `loopFirst=true`（默认）下，集成测试断言 `D2.QueryLoop.Run` 零调用 | 同上路径，counter == 0 |
| AC3 | `loopFirst=false` 触发 `Loop.Run()` 时输出 `// Deprecated:` slog.Warn 一次/进程 | slog capture；regex 匹配 |
| AC4 | metric `d2_query_loop_legacy_invocations_total` 注册到 observability，启动后 `/metrics` 端点可见 | 端点扫描；counter 类型无 label |
| AC5 | `devrix.yaml` / `devrix --help` 在 `loopFirst=false` 配置处含"WARNING: 仅作临时回滚使用"文本 | 集成测试调 help 输出，断言包含警告 |
| AC6 | `openspec/specs/d2-context-engine/spec.md` D2-S10 章节加 LEGACY 标记 + 链接 TD-QL-LOC | spec.md diff 审查 |
| AC7 | 主路径代码（`internal/layers/orchestration/turn/orchestrator.go` / `llm.go`）**零改动** | git diff 范围限制 |

### 3.2 P1（本期不交付但锁定演进路径）

| ID | 标准 |
|----|------|
| AC8 | legacy metric 连续 4 周 < 1 invocations/day 后，开 Z1 sub-change：把 `Loop.Run` 改为 thin wrapper 直接调 D7.RunTurnLoop |
| AC9 | legacy metric 连续 12 周 = 0 后，开 Z2 sub-change：删除 `D2.QueryLoop` + 拆面 adapter + `query_loop.enabled` 配置项 |
| AC10 | Z2 完成后，`openspec/specs/d2-context-engine/spec.md` 中 D2-S10 场景从 spec 移除或显式标 LEGACY |

### 3.3 范围/质量基线

| ID | 标准 |
|----|------|
| AC11 | 不删除任何 Go 源文件、不删除配置项、不删除 T 层测试（仅加 LEGACY 标记） |
| AC12 | 跨域新增 import 不得引入新的依赖环（layering 规则） |
| AC13 | 所有 P0 T 层 100% PASS；新 T 点（`D7-S2-A06-T09`/`T10`、`D5-S24-A02-T04`/`T05`）覆盖 AC1/AC2/AC4/AC5 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 上游 | DM-20260616-002（`devrix-d7-loop-first-routing`，PR #46 merged `866506f`）— `loopFirst=true` 默认主路径已就绪 |
| 上游 | DM-20260616-001（`devrix-d7-uncertainty-gaps`）— D7 Turn 接口稳定 |
| 上游 | DM-020（D7 Turn 编排上移）— D7 拥有 `LoopHooks` / `InvokeLLM` 等编排回调 |
| 约束 | **不删除** `internal/layers/contextengine/query/loop.go` 的 `Loop.Run()`（保留回滚能力） |
| 约束 | **不删除** `internal/shared/contracts/llm_facade.go` 与 `internal/layers/orchestration/turn/query_llm_caller.go`（legacy 仍依赖） |
| 约束 | **不删除** `query_loop.enabled` 配置项（仅加文档警告） |
| 约束 | **不改** `internal/layers/orchestration/turn/orchestrator.go` / `llm.go` / `bootstrap/wire_coordinator.go` 主路径代码（AC7） |
| 约束 | legacy metric 必须走 `d5_observability/instrument/metrics/` 现有注册机制，不引入新 metric 包 |

## 5. 变更范围

### 5.1 新增

- `openspec/tech-debt/queryloop-location.md` — TD-QL-LOC 完整债务分析（已写）
- 新 T 测试点（PLANNED，待 S3 阶段在 `openspec/specs/d7-orchestration/t-registry.md` 注册）：
  - `D7-S2-A06-T09` loopFirst=true 时 D2→D3 拆面 adapter 零调用
  - `D7-S2-A06-T10` loopFirst=true 时 D2.QueryLoop.Run 零调用
  - `D5-S24-A02-T04` `d2_query_loop_legacy_invocations_total` 注册到 `/metrics`
  - `D5-S24-A02-T05` legacy 路径 slog.Warn 输出（仅一次/进程）

### 5.2 修改

| 文件 | 改动 |
|------|------|
| `internal/layers/contextengine/query/loop.go` | `Loop.Run()` 顶部加 `// Deprecated: ...` 注释；进入函数时递增 legacy metric + 首次 slog.Warn |
| `internal/layers/observability/instrument/metrics/` | 注册 `d2_query_loop_legacy_invocations_total` counter |
| `internal/layers/orchestration/coordinator/routing.go` | `IsLoopFirst()` 文档警告"false 仅作临时回滚" |
| `internal/layers/orchestration/coordinator/config_help.go`（或 CLI 帮助模块） | `devrix --help` 输出 loopFirst 警告文本 |
| `openspec/specs/d2-context-engine/spec.md` | D2-S10 章节加 LEGACY 标记 + 链接 TD-QL-LOC |
| `openspec/specs/d7-orchestration/t-registry.md` | 新增 T09/T10 条目 |

### 5.3 不变更（明确锁定）

- `internal/layers/contextengine/query/loop.go` 的 `Loop.Run()` 函数体与 `Loop` 结构体字段
- `internal/shared/contracts/llm_facade.go` 全文
- `internal/layers/orchestration/turn/query_llm_caller.go` 全文
- `internal/layers/orchestration/turn/orchestrator.go` 全文（主路径）
- `internal/layers/orchestration/turn/llm.go` 全文（主路径）
- `internal/bootstrap/wire_coordinator.go` 全文
- `devrix.yaml` 的 `query_loop.enabled` 配置项定义
- 现有 D2-S10 T 层测试（保留 IMPLEMENTED 状态，仅在 spec.md 加 LEGACY 标记）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| **现有用户依赖 `loopFirst=false`** | 强制切换破坏其工作流 | AC11：Z0 不删除任何代码，仅警告 + 监控 |
| **`Loop.Run()` 仍被外部测试或脚本直接引用** | 删除会破坏 CI | AC11：仅注释 + metric，不改函数体 |
| **legacy metric 注册路径与现有 metric 体系不一致** | 监控采集失败 | AC13：T04 测试覆盖注册到 `/metrics` 端点 |
| **D2-S10 T 层测试在 spec.md 标 LEGACY 后被 CI 当作失败** | CI 红 | spec.md LEGACY 标记是文档标记，不影响测试状态；D2-S10 测试保持 IMPLEMENTED 状态 |
| **AC1/AC2 集成测试需要 mock 全链路** | 测试不稳定 | 用现有 `tests/integration/d7_turn_loop_*` 基础设施（参考 `path_regression_integration_test.go`） |
| **slog.Warn 在高频调用时性能开销** | legacy 路径变慢 | AC3：每个进程仅输出一次（用 `sync.Once` 或 atomic flag） |

## 7. Out of Scope

- **不实现** Z1/Z2（仅在 AC8/AC9 锁定触发条件与下一步工作内容）
- **不删除** 任何 Go 源文件
- **不删除** 任何配置项
- **不删除** 任何 T 层测试
- **不重构** 主路径代码
- **不迁移** D2-S10 测试到 D7-S2-A06（保留双轨）
- **不处理** 其他 D2/D7 边界问题（仅聚焦 QueryLoop 位置）
- **不覆盖** D4 / D5 / D6 / D1 / D3 域内的 QueryLoop 相关代码（仅在 D2 / D5 / D7 域内工作）

## 8. 关联参考

- **架构债主文档：`openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)**
- D2/D7 边界规范：`openspec/specs/d2-context-engine/d7-boundary.md` §3 职责矩阵第 11 行
- 主路径实现：
  - `internal/layers/orchestration/turn/orchestrator.go:49` — `RunTurn executes prepare→llm→tools→persist (D7-S2-A06)`
  - `internal/layers/orchestration/turn/llm.go:50` — `GatewayInvoker.Stream` 直连 D3
- Legacy 路径证据：
  - `internal/shared/contracts/llm_facade.go:11` — "D2 query loop 拆面出口"
  - `internal/layers/orchestration/turn/query_llm_caller.go:21` — "D2→D3 拆面 adapter"
  - `internal/bootstrap/wire_coordinator.go:75` — "replaces the legacy executor"
- 配置开关：`internal/layers/orchestration/coordinator/routing.go:24` `IsLoopFirst()`
- 历史变更：DM-020 (D7 Turn 编排上移), DM-20260616-001 (D7 Uncertainty Gaps), DM-20260616-002 (D7 Loop-First Routing, PR #46)

## 9. 检查清单（S1 完成确认）

- [x] DM ID 已分配（DM-20260617-001，今日首个）
- [x] demand.md 包含背景、问题、验收标准、范围
- [x] 至少 7 个 P0 验收标准（AC1-AC7）+ 3 个演进锁定（AC8-AC10）+ 3 个基线（AC11-AC13）
- [x] Out of Scope 已明确声明（§7）
- [x] DSAFT 域标注正确（orchestration，含 D2/D5/D7）
- [x] 不变更范围明确锁定（§5.3 列出 7 项不动的文件）
- [x] 风险评估含影响与缓解（§6）
- [x] 关联 tech-debt 文档（§8）
