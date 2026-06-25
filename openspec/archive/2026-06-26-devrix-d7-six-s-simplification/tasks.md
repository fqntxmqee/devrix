# Tasks — devrix-d7-six-s-simplification (DM-20260626-001)

**Change ID:** `devrix-d7-six-s-simplification`
**Demand ID:** DM-20260626-001
**Status:** S3_Design → S4_Implemented → S5_Accepted → S7_Archived (2026-06-26)
**PR:** [#215](https://github.com/fqntxmqee/devrix/pull/215)
**Merge commit:** `0ce5e52` (squash `f5cd62d`)

---

## 1. Step 1 — Spec 层精简（9 文档，0.5 天）✅

| Task | 文档 | 版本变化 | 状态 |
|------|------|----------|------|
| 1.1 | `d7-domain.md` | v1.2.0 → v2.0.0（14 S → 6 S + 1 横切） | ✅ |
| 1.2 | `a-registry.md` | v4.0.0 → v5.0.0（56 → 49 A + §v6.0.0 6 S 精简映射） | ✅ |
| 1.3 | `f-registry.md` | v4.0.0 → v5.0.0（75 → 68 F） | ✅ |
| 1.4 | `span-registry.md` | v3.0.0 → v4.0.0（18 → 23 ops） | ✅ |
| 1.5 | `t-registry.md` | v3.18.0 → v4.0.0（T 180 持平） | ✅ |
| 1.6 | `terminal-state-guide.md` | v1.2.0 → v2.0.0（§3 重写 6 S + 1 横切） | ✅ |
| 1.7 | `observability-guide.md` | v1.2.0 → v2.0.0（Span↔T 表重归类） | ✅ |
| 1.8 | `layer-delta.md` | v5.0.0 → v6.0.0（§V6 IMPLEMENTED 段） | ✅ |
| 1.9 | `design.md` | v3.3.0 → v4.0.0（§⑦ MUPS 5-node 6 S 归类） | ✅ |

**diff:** +1621/-193 行

## 2. Step 2 — Code 层包路径迁移（14 → 8 包）⏳ DEFERRED

| Task | 包 | 状态 |
|------|-----|------|
| 2.1 | `orchestration/mups/` (NEW) — 合并 `execute/` + `learn/` | ⏳ follow-up PR |
| 2.2 | `orchestration/hardening/` (NEW) — 散落各包 metrics.go | ⏳ follow-up PR |
| 2.3 | `orchestration/sessionorchestrator/` 扩展吸收 turn/ + autoclose | ⏳ follow-up PR |
| 2.4 | `orchestration/executionflow/` 扩展吸收 verify/ | ⏳ follow-up PR |
| 2.5 | `orchestration/decisionplanning/` 扩展吸收 observe/orchtypes/ | ⏳ follow-up PR |
| 2.6 | `internal/bootstrap/wire_coordinator.go` 14 wire → 6 wire | ⏳ follow-up PR |

**说明：** 本 PR (#215) 仅做 spec 文档精简 + 5 Span 落地；Step 2 的代码包路径迁移（execute/ + learn/ → mups/）影响面广（22 包 import 关系），留作后续独立 PR。

## 3. Step 3a — 3 个 P0 Span emit（1.5 天）✅

| Task | Span | 文件 | 状态 |
|------|------|------|------|
| 3a.1 | `D7_Channel_Route` | `execute/channel.go::ChannelRouter.Route` | ✅ S6-A48 |
| 3a.2 | `D7_Memory_Persist` | `learn/memory.go::SkillMemory.Store` | ✅ S6-A49 |
| 3a.3 | `D7_System_Anomaly_Detect` | NEW `executionflow/verify/anomaly.go::DetectSystemAnomaly` | ✅ S4-A47 |

**配套：**
- `d7spans/emitter.go`（NEW package）：5 emit helpers + SetBridge setter
- `telemetry/names.go`：5 new OpD7_* constants + LayerAndComponent mapping
- `bootstrap/wire_coordinator.go`：`d7spans.SetBridge(obsBridge)` 接线

## 4. Step 3b — 2 个 P1 Span emit（1 天）✅

| Task | Span | 文件 | 状态 |
|------|------|------|------|
| 3b.1 | `D7_TaskGraph_Synthesize` | `decisionplanning/decomposer.go::SynthesizeTaskGraph` | ✅ S5-A33 |
| 3b.2 | `D7_Executor_Select` | `wavescheduler/scheduler.go::dispatchOne` | ✅ S5-A34 |

**配套：**
- `dagDepth` helper：最长路径 BFS，cycle 时 cap 在 len(nodes)

## 5. Step 4 — 验证（1 天）✅

| Task | 检查 | 结果 |
|------|------|------|
| 4.1 | `go build ./...` | ✅ 0 错误 |
| 4.2 | `go vet ./...` | ✅ 0 警告 |
| 4.3 | `go test ./internal/layers/orchestration/... -race -count=1` | ✅ 22/22 PASS |
| 4.4 | 20 新 T 点（10 d7spans + 6 verify + 4 decisionplanning） | ✅ IMPLEMENTED |
| 4.5 | LP-1 兼容（Phase 4 PR-D4 UncertaintyCoord Value=0.95 路径 0 变化） | ✅ 验证通过 |
| 4.6 | PR #215 CI 跑通 + auto-merge | ✅ merged at 0ce5e52 |

## 6. Step 5 — S5 验收 + S6 归档（1 天）✅

| Task | 验收项 | 结果 |
|------|--------|------|
| 5.1 | master 分支 22/22 orchestration 包 race PASS | ✅ |
| 5.2 | 9 个 spec 文档 14 S → 6 S 完整归类 | ✅ |
| 5.3 | 5 个新 Span 在 Jaeger 中可见（d7spans.SetBridge 接线生效） | ✅ |
| 5.4 | 20 个新 T 点全 IMPLEMENTED | ✅ |
| 5.5 | S6 归档目录 `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/` | ✅ |
| 5.6 | `.openspec.yaml` + `proposal.md` + `design.md` + `tasks.md` + `acceptance-report.md` | ✅ |
| 5.7 | `demand-archive-index.md` 同步 DM-20260626-001 行 | ✅ |
| 5.8 | `verify-archive.sh devrix-d7-six-s-simplification` 11/11 PASS | ✅ |

## 7. PR 拆分

| PR | 范围 | 文件 | 估算 LOC | 风险 | commits |
|----|------|------|----------|------|---------|
| #215 | 14 S → 6 S 精简（9 spec 文档）+ 5 个新 P0/P1 Span | 22 files (2 NEW dirs) | +2590/-199 | Low | f5cd62d (squash 0ce5e52) |

## 8. 后续 PR（follow-up）

1. **devrix-d7-mups-package-migration**：把 `execute/` + `learn/` 合并到 `mups/` 包（Step 2 落地）
2. **devrix-d7-hardening-cross-cutting**：把散落各包 metrics.go 提升到 `hardening/` 包
3. **devrix-d7-6s-package-merge**：把 `turn/` + `autoclose.go` 合并到 `sessionorchestrator/` 包
4. **devrix-d7-6s-verify-promotion**：把 `turn/exit_reason.go` + `observe/verify/` 提升到 `executionflow/verify/`
5. **devrix-d7-6s-observe-merge**：把 `observe/orchtypes/` 合并到 `decisionplanning/` 包
6. **devrix-d7-6s-bootstrap-slim**：把 `wire_coordinator.go` 14 wire → 6 wire

每个 follow-up PR 工作量 0.5-1 天，影响面需逐包评估 import 关系。
