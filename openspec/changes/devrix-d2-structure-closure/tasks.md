# Implementation Tasks: D2 v2.2 Structure 终态

**Demand ID:** DM-20260619-007  
**Change ID:** devrix-d2-structure-closure

---

## 阶段状态

| 阶段 | 状态 | 产物 |
|------|------|------|
| S1 需求 | ✅ | `demand.md` |
| S2 澄清 | ✅ | 终态目录树 + 根文件映射表 |
| S3 设计 | ✅ | `design.md` |
| **S3-Gate** | ✅ P1-a Approved | `design.md` §7 |
| S4 开发 | 🔄 P1-a 进行中 | `persist/commit.go` |
| S5 验收 | — | `acceptance-report.md` |
| S7 归档 | — | `openspec/archive/` + specs 回写 |

**S4 前置：** S3-Gate Approved（Owner 确认 `demand.md` §4 终态树）

---

## P1 — 编排收敛（AC-P1-*）

| ID | 任务 | 目标路径 | L4 |
|----|------|----------|-----|
| P1-T1 | `engine_prepare.go` 改调 `prepare.Orchestrator.Prepare` | `legacy/` or `facade/` | S15 orchestrator |
| P1-T2 | 实现/补齐 PrepareDeps 适配（SessionLoader 等） | `prepare/contracts.go` | S15 ports |
| P1-T3 | `engine_persist.go` 改调 `persist.Orchestrator` | `legacy/` | S17 orchestrator |
| P1-T4 | 新建 `persist/commit.go`（AppendAndTrim / CommitWindow） | `persist/commit.go` | S17-A04 |
| P1-T5 | `turn_adapter.Prepare/PersistTurn` 对齐 orchestrator | `bootstrap/turn_adapter.go` | D7 wiring |
| P1-T6 | 删除 facade 内 duplicate inline 逻辑 | `facade/` | — |
| P1-T7 | 跑通：`prepare/*_test` + `persist/*_test` + turn 集成 | — | T |

---

## P2 — 根目录清零（AC-P2-*）

| ID | 任务 | 终态 |
|----|------|------|
| P2-T1 | 迁移 10 个根 `*_test.go` | `tests/integration/d2/` 或 scenario 包 |
| P2-T2 | `tool_context.go` → `enforce/tools/context.go` | 根留 alias |
| P2-T3 | `mock/` → `tests/testutil/contextengine/` | 域外 |
| P2-T4 | 升级 `TestD2_RootProductionFiles`（仅 2 生产文件） | `internal/lint/layer/` |
| P2-T5 | 合并 queryloop 布局守卫 | `d2_layout_test.go` |

---

## P3 — enforce 归位（AC-P3-*）

| ID | 任务 | 说明 |
|----|------|------|
| P3-T1 | `git mv sandbox/` → `enforce/sandbox/` | S18-A03 |
| P3-T2 | `git mv toolrunner/` → `enforce/tools/` | 包名同步 |
| P3-T3 | 更新 bootstrap / multiagent / delegatetools import | 机械替换 |
| P3-T4 | `enforce/orchestrator.go` 与 ExecuteRound 语义文档化 | 删 stub 或 wired |
| P3-T5 | `code-layout.md` §4.3 深度规则 + 终态树 | 文档 |

---

## P4 — Memory 读写分离（AC-P4-*）

| ID | 任务 | 说明 |
|----|------|------|
| P4-T1 | 拆 `prepare/memory/longterm.go` → `recall.go` | S15 只读 |
| P4-T2 | 新建 `persist/memory/store.go` | S17-A03 |
| P4-T3 | 更新 `a-registry` S17-A03/A04 Code Location | 双锚点 |

---

## P5 — Legacy 退役（AC-P5-*）

| ID | 任务 | 说明 |
|----|------|------|
| P5-T1 | `facade/` → `legacy/` | git mv |
| P5-T2 | `Process()` 加 Deprecated 注释 + slog warn | — |
| P5-T3 | layer-lint：禁止新增 `Process()` 生产引用 | 可选 P1 |

---

## P6 — 规格双锚点（AC-P6-*）

| ID | 任务 | 文件 |
|----|------|------|
| P6-T1 | 终态路径表 v8.2 | `d2-domain.md` |
| P6-T2 | Code Location 全量同步 | `a-registry.md`, `f-registry.md` |
| P6-T3 | 目录树更新 | `layering.md`, `design.md` |
| P6-T4 | S5 后 delta 合入 `openspec/specs/d2-context-engine/` | S7 门禁 |

---

## 测试门禁（每 Phase）

```bash
go test -short ./internal/layers/contextengine/...
go test -short ./internal/bootstrap/...
go test -short ./internal/lint/layer/...
```

P0 T 点：关联 `t-registry.md` D2-S15/S17/S18 条目；新增 layout 守卫 T（待登记）。
