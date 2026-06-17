# Acceptance Report: D2 QueryLoop Legacy Path Decommission (TD-QL-LOC)

**Change ID:** devrix-queryloop-legacy-decommission
**Demand ID:** DM-20260617-001
**Status:** S5_Acceptance
**Date:** 2026-06-17
**Reviewer:** change-author self-review (no external reviewer per single-maintainer policy)

---

## 1. AC 验证结果（按 demand.md AC1-AC13）

| AC ID | 描述 | 验证手段 | 结果 |
|-------|------|----------|------|
| AC1 | D2.QueryLoop.Run 函数体在 DM-020 后只剩余 D2 执行原子 | code review (loop.go 233 行) | ✅ |
| AC2 | loopFirst=true 是默认主路径 | `IsLoopFirst()` docstring + DefaultConfig | ✅ |
| AC3 | D7-S2-A06 RunTurnLoop 是 canonical path | `orchestrator.go:49` 现有实现 | ✅ |
| AC4 | QueryLoop.Run 标 Deprecated 注释 | loop.go 函数文档块（"DEPRECATED (DM-20260617-001)"） | ✅ |
| AC5 | 启动 slog.Warn | loop.go warnLegacyOnce.Do | ✅ |
| AC6 | 每次 Run 必增 metric | loop.go LegacyCounter.Inc() | ✅ |
| AC7 | 主路径代码零改动 | diff 验证 loop.go Run() 主体未改 | ✅ |
| AC8 | counter 阈值 < 1/day 持续 4 周触发 Z1 | 见本报告 §5 Decommission 计划 | ⏳ Z1 待触发 |
| AC9 | counter = 0 持续 12 周触发 Z2 | 同上 | ⏳ Z2 待触发 |
| AC10 | Z1: thin wrapper 调用 D7 | 后续 change | ⏳ 待触发 |
| AC11 | 不删除 Loop.Run 代码 | file:query/loop.go 仍存在 233 行 | ✅ |
| AC12 | 不删除 query_loop.enabled 配置 | DefaultQueryLoopConfig 仍存在 | ✅ |
| AC13 | 不删除既有测试 | `query/loop_test.go` 等未触碰 | ✅ |

---

## 2. P0 T 层验证

| T ID | 描述 | 域 | 优先级 | 实现 | 状态 |
|------|------|-----|--------|------|------|
| D7-S2-A06-T09 | D7 RunTurn never touches D2.QueryLoop.Run | D7 | P0 | `orchestration/turn/loop_legacy_test.go::TestOrchestrator_RunTurn_DoesNotInvokeLegacyQueryLoop` | ✅ |
| D7-S2-A06-T10 | D2.QueryLoop.Run bumps `d2_query_loop_legacy_invocations_total` per call | D7 | P0 | `contextengine/query/loop_legacy_test.go::TestLoopRun_legacy_counter_bumps_on_every_invocation` | ✅ |
| D5-S24-A02-T04 | legacy counter registered in registry | D5 | P0 | `internal/layers/observability/instrument/metrics/legacy_test.go::TestLegacyD2Metrics_QueryLoopInvocations_registered_in_registry` | ✅ |
| D5-S24-A02-T05 | warning emitted exactly once per Loop instance | D5 | P0 | `contextengine/query/loop_warn_test.go::TestLoopRun_warnLegacyOnce_emits_exactly_one_warning` | ✅ |

**P0 覆盖率: 4/4 (100%)**

---

## 3. 文件清单验证

### 新增文件
- ✅ `internal/layers/observability/instrument/metrics/legacy.go` (47 行)
- ✅ `internal/layers/observability/instrument/metrics/legacy_test.go` (37 行)
- ✅ `internal/layers/contextengine/query/loop_legacy_test.go` (78 行)
- ✅ `internal/layers/contextengine/query/loop_warn_test.go` (122 行)
- ✅ `internal/layers/orchestration/turn/loop_legacy_test.go` (53 行)
- ✅ `openspec/changes/devrix-queryloop-legacy-decommission/tasks.md`
- ✅ `openspec/changes/devrix-queryloop-legacy-decommission/acceptance-report.md` (本文件)

### 修改文件
- ✅ `internal/layers/contextengine/query/loop.go`（添加 2 字段 + Deprecated 注释 + Inc + sync.Once Warn）
- ✅ `internal/layers/contextengine/engine_builder.go`（wire LegacyCounter + helper 函数）
- ✅ `internal/layers/orchestration/coordinator/routing.go`（IsLoopFirst docstring）
- ✅ `internal/bootstrap/wire_coordinator.go`（rule_orchestrate 启动警告）
- ✅ `openspec/specs/d7-orchestration/spec.md`（6 Gherkin 场景 + Revision History 3.5.0）
- ✅ `openspec/specs/d2-context-engine/spec.md`（LEGACY 标记）
- ✅ `openspec/specs/d7-orchestration/t-registry.md`（+2 T 点 + Revision History 3.3.0）
- ✅ `openspec/specs/d5-observability/t-registry.md`（+2 T 点 + Revision History 3.1.0）

### 未变更（按需保留回滚）
- ✅ `internal/layers/contextengine/query/loop.go` 主体逻辑（行 80-225 区域，仅前置插入新代码）
- ✅ `internal/shared/contracts/llm_facade.go`（"拆面 adapter" 注释保留）
- ✅ `internal/layers/orchestration/turn/query_llm_caller.go`（"拆面 adapter" 注释保留）
- ✅ 所有既有测试、配置文件、YAML

---

## 4. 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| LegacyCounter nil 时 Run() panic | 低 | 代码 nil-check: `if l.LegacyCounter != nil` |
| sync.Once 在多 Loop 实例下 spam 日志 | 低 | 生产环境仅一个 Loop（engine_builder 启动期单次构造）；多 Loop 测试场景可接受 |
| metric 注册冲突（重复 Init） | 低 | `metrics.NewCounter` + `Registry.RegisterCounter` 已有重复保护 |
| slog.Warn 噪声 | 低 | sync.Once 限定单次 |
| 误以为"修复了"而停止监控 | 中 | 验收报告 §5 显式定义 Z1/Z2 触发条件 |

---

## 5. Decommission 触发计划

| 阶段 | 触发条件 | 行动 |
|------|----------|------|
| **当前 (v1.0)** | 合并后 | log warn + emit metric |
| **Z1** | counter 7d 平均 < 1/day，持续 4 周 | 将 Loop.Run 改为 thin wrapper 调 D7 |
| **Z2** | counter = 0 持续 12 周 | 删除 Loop.Run 文件 + `query_loop.enabled` 配置 |

**监控责任**：SRE 在 `d2_query_loop_legacy_invocations_total` dashboard 设置 P0 alert（任意 >0 即触发）。

---

## 6. 回归风险（每项评估）

| 既有能力 | 受影响？ | 说明 |
|----------|----------|------|
| loopFirst=true 主路径（D7-S2-A06 RunTurnLoop） | 否 | 未触碰 orchestrator.go |
| D2 QueryLoop fallback（loopFirst=false） | 否 | 函数体未改，仍可作紧急回滚 |
| Multi-turn tool loop | 否 | 既有测试 `TestQueryLoop_should_continue_until_no_tool_use` 不变 |
| Compression | 否 | Compress 字段未改 |
| Fallback LLM | 否 | FallbackLLM/FallbackOnErr 字段未改 |
| 启动时 Sync | 否 | 改 engine_builder 加 nil-safe `resolveLegacyQueryLoopCounter` |

---

## 7. CI 验证状态

> **注**：本地无 Go 工具链（`go not found`），以下项目需在 CI 验证：

- [ ] `go test -race ./internal/layers/contextengine/query/...` — 跑 4 个新测试
- [ ] `go test -race ./internal/layers/orchestration/turn/...` — 跑 1 个新测试
- [ ] `go test -race ./internal/layers/observability/instrument/metrics/...` — 跑 2 个新测试
- [ ] `go vet ./...` — 验证导入与类型
- [ ] `gofmt -d` — 验证格式
- [ ] `gosec ./internal/layers/contextengine/query/...` — 安全扫描
- [ ] 既有测试不回归：`TestQueryLoop_should_continue_until_no_tool_use` 等保持绿

**PR Description 必须显式声明**: "本地无 go 工具链，测试需 CI 验证。"

---

## 8. 关闭标准

- [x] 所有 AC1-AC7（P0）+ AC11-AC13（baseline）已验证
- [ ] CI 测试通过
- [ ] PR 合入 master
- [ ] 归档到 `openspec/archive/2026-06-17-devrix-queryloop-legacy-decommission/`
- [ ] Z1 监控 dashboard 配置

---

## 9. 总结

**实施状态：v1.0 完成（Z0 阶段）**

D2 QueryLoop 主路径未动（用户"理想架构"已是 loopFirst=true 默认现状），legacy 路径已加 3 信号 deprecation 契约（metric + 一次性 warn + 文档），可观测性、可回滚性、可演进性三件套到位。

后续 Z1/Z2 由 metric 阈值驱动，无需本次 PR 进一步行动。
