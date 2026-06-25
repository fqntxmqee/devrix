# Proposal: D7 MUPS 包路径迁移

**Change ID:** `devrix-d7-mups-package-migration`
**Demand ID:** DM-20260626-002
**Priority:** P1
**Sprint:** d7-v6 follow-up
**PR Count:** 1
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**SoT:** `devrix-d7-six-s-simplification` (DM-20260626-001) acceptance-report.md §7 后续工作

---

## 1. Background

`devrix-d7-six-s-simplification` (DM-20260626-001) 在 v6.0.0 域升级中，把 D7 编排层的 14 S 博弈角色精简为 6 S + 1 横切：
- **S1 WorkModel** (State Authority)
- **S2 SessionOrchestrator** (Mediator + Turn Leader + Error Recovery)
- **S3 WaveScheduler** (Mechanism Designer)
- **S4 ExecutionFlow + Verify** (Costly Signaler + Certifier)
- **S5 DecisionPlanning + Observe** (Information Producer + Quantizer)
- **S6 MUPS Pipeline** (Pipeline Coordinator + Memory Curator)
- **横切 Hardening** (Discipline Keeper)

9 个 spec 文档已在 PR #215 (commit 0ce5e52) 中完整重写并 S7_Archived。但 **物理代码包路径未做迁移**，Step 2 作为 follow-up 留待本 change。

## 2. Problem Statement

虽然 spec/code 语义层已对齐 6 S + 1 横切，但 Go 包路径仍按 14 S 时期散落：

| 当前包路径 | 现状 | 应归 S 层 |
| ---------- | ---- | --------- |
| `orchestration/execute/` (7 .go) | 顶层独立包 | S6 MUPS Pipeline |
| `orchestration/learn/` (17 .go) | 顶层独立包 | S6 MUPS Pipeline |

**问题：**

1. **目录命名误导**：`execute/` 和 `multiagent/execute/` 同名不同域，需要靠完整 import path 区分
2. **物理结构未对齐博弈角色**：6 S 文档说"S6 MUPS Pipeline = Execute 4 Channel + Learn 3 通道"，代码层却是两个独立顶层包
3. **bootstrap 接线复杂度**：14 wire 节点的现状与 6 S 角色不对应
4. **新人 onboard 认知负担**：看到 14 个目录会怀疑"是不是 14 个 S 角色还在？"

## 3. Proposed Solution

**把 `orchestration/execute/` 和 `orchestration/learn/` 物理迁移到 `orchestration/mups/` 子树下，保持包名不变：**

```
当前：                              目标：
orchestration/                     orchestration/
├── execute/                       ├── mups/  (NEW)
│   ├── channel.go                 │   ├── execute/   (从 orchestration/execute/ 迁移)
│   ├── channel_commit.go          │   │   ├── channel.go
│   ├── channel_exploration.go     │   │   ├── channel_commit.go
│   ├── channel_protocol.go        │   │   ├── channel_exploration.go
│   ├── channel_scenario.go        │   │   ├── channel_protocol.go
│   ├── errors.go                  │   │   ├── channel_scenario.go
│   └── execute_test.go            │   │   ├── errors.go
└── learn/                         │   │   └── execute_test.go
    ├── adaptive_prior.go          │   └── learn/     (从 orchestration/learn/ 迁移)
    ├── adaptive_prior_test.go     │       ├── adaptive_prior.go
    ├── asset_builder.go           │       ├── ...
    ├── ...                        │       └── testhelpers_test.go
    └── testhelpers_test.go        ├── decisionplanning/  (不变)
                                   ├── executionflow/     (不变)
                                   ├── sessionorchestrator/ (不变)
                                   ├── ... (其他 12 个目录)
```

**包名保持不变（`package execute` / `package learn`）** — 这样：
- 内部 cross-file 引用（`learn/learning_asset.go` 引用 `learn/asset_content.go`）无需改动
- 对外 import path 只改 `internal/layers/orchestration/learn` → `internal/layers/orchestration/mups/learn`

**Import path 替换（15 处）：**
- `orchestration/decisionplanning/classifier.go` (1)
- `orchestration/orchtypes/` 4 impl + 4 test (8)
- `orchestration/sessionorchestrator/` 3 impl + 7 test (10)
- 总计 15 处 import 替换（部分文件 0 引用本次 change 范围）

## 4. Success Metrics

| 指标 | 当前 | 目标 |
|------|------|------|
| **D7 orchestration 子包数** | 14 | 13 (本次 -1) |
| **`orchestration/mups/` 子目录** | 0 | 2 (execute + learn) |
| **orchestration/execute 直接外部 import** | 0 | 0 (位置迁移，无外部调用方需更新) |
| **orchestration/learn 直接外部 import** | 15 处 | 0 (全部更新为 mups/learn) |
| **go test -race 通过包数** | 22/22 | 22/22 (持平 baseline) |
| **`grep "orchestration/execute\""` 命中数** | 1 (execute_test.go 自身) | 0 |
| **`grep "orchestration/learn\""` 命中数** | 15 | 0 |
| **D7-S6-A51 新 T 点** | 0 | 4 PLANNED → 4 IMPLEMENTED |
| **LP-1 / LP-2 / LP-5 路径变化** | — | 0 (行为不变) |

## 5. Implementation Plan

### Step 1: 物理目录迁移（0.3 天）

```bash
# 在 internal/layers/orchestration/ 下创建 mups/ 子树
mkdir -p internal/layers/orchestration/mups/execute
mkdir -p internal/layers/orchestration/mups/learn

# 用 git mv 迁移文件（保留历史）
git mv internal/layers/orchestration/execute/*.go internal/layers/orchestration/mups/execute/
git mv internal/layers/orchestration/learn/*.go internal/layers/orchestration/mups/learn/

# 验证文件全部就位
ls internal/layers/orchestration/mups/execute/  # 应有 7 .go files
ls internal/layers/orchestration/mups/learn/    # 应有 17 .go files
```

### Step 2: Import Path 全仓替换（0.3 天）

```bash
# 全仓替换 import path
grep -rl "internal/layers/orchestration/learn\"" internal/ cmd/ | xargs sed -i \
  's|internal/layers/orchestration/learn"|internal/layers/orchestration/mups/learn"|g'

# execute 包 0 外部 import，跳过替换

# 验证 0 残留
grep -rln "internal/layers/orchestration/learn\"" internal/ cmd/  # 必须 0 命中
```

### Step 3: 编译 + 测试回归（0.2 天）

```bash
go build ./...          # 0 错误
go vet ./...            # 0 警告
go test -race -count=1 ./internal/layers/orchestration/...  # 22/22 PASS
```

### Step 4: 文档同步（0.2 天）

- `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 §MUPS 5 节点管道挂载：更新包路径
- `openspec/specs/d7-orchestration/design.md` v4.0.0 §⑦ MUPS 5-node 6 S 归类：更新包路径
- `openspec/specs/d7-orchestration/t-registry.md` v4.0.0 → v4.1.0：新增 4 P0 T
- `openspec/t-registry.md` (root) v5.0.0 → v5.1.0：新增 DM-20260626-002 增量条目

## 6. Risks & Mitigations

| 风险 | 等级 | 缓解 |
|------|------|------|
| 22 包 import 关系复杂导致回归失败 | 中 | Step 3 跑全量 `-race` 回归；失败立即 revert |
| `multiagent/execute/` 名称冲突 | 低 | 全仓 grep 排除，`multiagent/execute/` 不在范围 |
| 测试文件跨包 fixture 引用 | 中 | orchtypes 4 测试文件 + sessionorchestrator 7 测试文件同步替换 import；grep 0 残留是硬门禁 |
| LP-1/LP-2/LP-5 行为漂移 | 极低 | 包名不变 + 行为不变；Phase 4/5/6/7 集成测试覆盖回归 |
| CI 镜像缓存导致旧路径编译过 | 低 | 删除旧目录后强制 re-build 验证 |
| 与其他 follow-up PR 并行冲突 | 低 | 其他 5 个 follow-up 尚未启动，本 change 独占 |

## 7. Out of Scope

- ❌ 任何函数签名、行为、对外接口改动 — 纯目录迁移 + import path 替换
- ❌ `package execute` / `package learn` 声明改动 — 保持兼容
- ❌ `internal/layers/multiagent/execute/` 不动 — 不同域不同职责
- ❌ `internal/shared/types/` 不动 — 跨域类型上提已在 PR-C1 落地
- ❌ 5 个其他 follow-up PR 范围 — 各自独立 PR 处理：
  - devrix-d7-hardening-cross-cutting
  - devrix-d7-6s-package-merge (turn/ + autoclose → sessionorchestrator/)
  - devrix-d7-6s-verify-promotion (exit_reason.go + observe/verify → executionflow/verify/)
  - devrix-d7-6s-observe-merge (observe/orchtypes → decisionplanning/)
  - devrix-d7-6s-bootstrap-slim (wire_coordinator.go 14 wire → 6 wire)
- ❌ 14 S → 6 S 文档语义改动 — devrix-d7-six-s-simplification 已 S7_Archived
- ❌ 5 个新 P0/P1 Span emit 路径 — 已在 devrix-d7-six-s-simplification 落地，本 change 不动

## 8. 关联

- **前置：** `devrix-d7-six-s-simplification` (DM-20260626-001) S7_Archived
- **Plan：** `/Users/fukai/.claude/plans/iterative-dazzling-locket.md` (Step 2 路径)
- **Archive Source：** `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/tasks.md` §8 follow-up PR 列表
