---
demand-id: DM-20260626-002
title: D7 编排层 MUPS 包路径迁移 — execute/ + learn/ → mups/ (v6.0.0 Step 2 落地)
priority: P1
status: S1_Proposal
dsaft_domain: architecture
created: 2026-06-26
---

# D7 MUPS 包路径迁移

## 1. 背景

`devrix-d7-six-s-simplification` (DM-20260626-001) 在 v6.0.0 域升级中，把 D7 编排层的 14 S 博弈角色精简为 6 S + 1 横切。spec 文档层（9 个文档）已全部重写为 6 S，但代码包路径迁移（Step 2: `execute/` + `learn/` → `mups/`）作为 follow-up 留待本 change 处理。

当前 D7 物理目录仍是 14 个独立子包，与 6 S + 1 横切博弈角色目标存在显著差距：
- 14 S → 6 S 文档 ✓
- 14 个 Go 包 → 8 个 Go 包 ✗ (本次 change 目标)
- 22 orchestration packages `go test -race` 100% PASS ✓

## 2. 问题陈述

虽然 spec/code 语义层已对齐 6 S（5 个新 Span 在 S6 MUPS Pipeline 角色下落地，A 编号重映射），但 Go 包路径仍按 14 S 时期的方式散落：

| 当前包路径 | 应归 S 层 | 6 S 对齐目标 |
| ---------- | --------- | ------------ |
| `orchestration/execute/` (7 .go) | S6 MUPS Pipeline | `orchestration/mups/execute/` |
| `orchestration/learn/` (17 .go) | S6 MUPS Pipeline | `orchestration/mups/learn/` |

**具体后果：**
1. **目录命名误导**：`execute/` 和 `multiagent/execute/` 同名不同域，新人需要靠完整 import path 区分
2. **物理结构未对齐博弈角色**：6 S 文档说"S6 MUPS Pipeline = Execute 4 Channel + Learn 3 通道"，但代码层仍是两个独立顶层包
3. **bootstrap 接线复杂度**：14 wire 节点的现状与 6 S 角色不对应，未来 6 S 全部落地后还需要再做一次 wire 收敛

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `internal/layers/orchestration/mups/execute/` 目录创建，原 `orchestration/execute/` 全部 7 个 .go 文件迁移完成，`package execute` 保持不变 | P0 |
| AC2 | `internal/layers/orchestration/mups/learn/` 目录创建，原 `orchestration/learn/` 全部 17 个 .go 文件（含 8 个 _test.go）迁移完成，`package learn` 保持不变 | P0 |
| AC3 | 全仓 `grep -rl "orchestration/execute\""` 0 命中（已迁移） | P0 |
| AC4 | 全仓 `grep -rl "orchestration/learn\""` 0 命中（已迁移），对应 import 全部更新为 `orchestration/mups/learn` | P0 |
| AC5 | `go build ./...` PASS（0 错误） | P0 |
| AC6 | `go vet ./...` PASS（0 警告） | P0 |
| AC7 | `go test ./internal/layers/orchestration/... -race -count=1` 全部 PASS（22/22 包，与 baseline 持平） | P0 |
| AC8 | `internal/bootstrap/wire_coordinator.go` 中若涉及 execute/learn 引用同步更新 | P1 |
| AC9 | 新增 `D7-S6-A50-T01`（mups/execute package exists）+ `D7-S6-A50-T02`（mups/learn package exists）+ `D7-S6-A50-T03`（零残留 import）+ `D7-S6-A50-T04`（build/test/vet 全绿）共 4 个 P0 T 点全部 IMPLEMENTED | P1 |
| AC10 | 6 个 follow-up PR 列表（devrix-d7-hardening-cross-cutting 等）README 同步说明本次为 follow-up #1 | P2 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | devrix-d7-six-s-simplification (DM-20260626-001) 已 S7_Archived，spec 层已对齐 6 S |
| 依赖 | 22 个 orchestration packages `go test -race` baseline 已稳定（PR #215 验证） |
| 约束 | **不允许** 改任何函数签名、行为、对外接口 — 纯目录迁移 + import path 替换 |
| 约束 | **不允许** 改动 `package execute` / `package learn` 声明 — 保持包名兼容 |
| 约束 | LP-1（Bayesian reputation）/ LP-2（Memory 3 通道）/ LP-5（Cross-session traceability）路径 0 变化 |
| 约束 | `multiagent/execute/` 不在本 change 范围内（不同域不同职责） |

## 5. 变更范围

### 新增

- `internal/layers/orchestration/mups/execute/` (NEW directory) — 原 `orchestration/execute/` 7 .go 迁移
- `internal/layers/orchestration/mups/learn/` (NEW directory) — 原 `orchestration/learn/` 17 .go 迁移

### 修改

- `internal/layers/orchestration/decisionplanning/classifier.go` — import 路径替换
- `internal/layers/orchestration/orchtypes/` (4 files: anomaly_detector.go + intent_quantizer.go + observe_request.go + _test.go 等) — import 路径替换
- `internal/layers/orchestration/sessionorchestrator/` (10 files: orchestrator.go + autoclose.go + tracing.go + entry_test.go + orchestrator_autoclose_test.go + orchestrator_escape_test.go + orchestrator_learner_test.go + orchestrator_priorspan_test.go + orchestrator_trackmode_test.go 等) — import 路径替换
- `internal/bootstrap/wire_coordinator.go` — 若涉及 execute/learn import 同步替换
- `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 §MUPS 5 节点管道挂载章节，更新包路径描述
- `openspec/specs/d7-orchestration/design.md` v4.0.0 §⑦ MUPS 5-node 6 S 归类，更新包路径描述
- `openspec/specs/d7-orchestration/t-registry.md` v4.0.0 — 新增 4 个 P0 T（T01-T04）

### 删除

- `internal/layers/orchestration/execute/` (整目录删除)
- `internal/layers/orchestration/learn/` (整目录删除)

### 不变更

- D7 14 S → 6 S 文档语义保持不变
- D7 5 个新 P0/P1 Span emit (channel.route / memory.persist / system.anomaly_detect / taskgraph.synthesize / executor.select) 路径 0 变化
- `internal/shared/types/` 跨域类型不动
- `internal/layers/multiagent/execute/` 不动（不同域不同职责）
- 5 个其他 follow-up PR 范围（hardening-cross-cutting / 6s-package-merge / 6s-verify-promotion / 6s-observe-merge / 6s-bootstrap-slim）不在本次范围

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| **22 个 orchestration 包 import 关系复杂** | 高 | 用 `git mv` 移动文件（保留历史），用 `sed -i` 全仓替换 import path，goimports 自动整理；执行前后 22 包 `-race` 跑一次回归 |
| **internal/external 测试 fixture 间接引用** | 中 | 全仓 grep `orchestration/execute` + `orchestration/learn` 必须 0 命中才合入；测试文件按 package 路径同步更新 |
| **PR 合并顺序冲突** | 低 | 本次 change 独立，依赖 devrix-d7-six-s-simplification 已合入 master；其他 follow-up PR 不并行 |
| **CI 镜像缓存导致旧路径仍编译过** | 低 | 删除旧目录后强制 re-build 验证；CI 单测 100% PASS 是硬门禁 |
| **IDE/Goland 索引需要重新同步** | 极低 | 文档同步说明 + README 更新 |

## 7. 调研依据（pre-S2 调研结果）

S1 阶段已完成的 import 关系调研（2026-06-26）：

**`orchestration/execute/` 包引用情况：**
- 仅自身 `execute_test.go` 使用
- **全仓 0 个外部 import**（极简迁移）

**`orchestration/learn/` 包引用情况（共 15 处 import）：**
- `orchestration/decisionplanning/classifier.go` (1 处)
- `orchestration/orchtypes/` 4 文件 + 4 测试文件 (8 处)
- `orchestration/sessionorchestrator/` orchestrator.go + autoclose.go + tracing.go + 7 测试文件 (10 处)

**结论：** 本次迁移影响面 = 15 个 import path 替换 + 2 个目录迁移（execute 极简 / learn 中等）。

## 8. 关联

- **前置：** `devrix-d7-six-s-simplification` (DM-20260626-001) S7_Archived
- **后续（其他 5 个 follow-up）：**
  - devrix-d7-hardening-cross-cutting
  - devrix-d7-6s-package-merge (turn/ + autoclose → sessionorchestrator/)
  - devrix-d7-6s-verify-promotion (exit_reason.go + observe/verify → executionflow/verify/)
  - devrix-d7-6s-observe-merge (observe/orchtypes → decisionplanning/)
  - devrix-d7-6s-bootstrap-slim (wire_coordinator.go 14 wire → 6 wire)
