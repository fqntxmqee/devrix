# Proposal: D7 hardening/ 横切包迁移

**Change ID:** `devrix-d7-hardening-cross-cutting`
**Demand ID:** DM-20260626-003
**Priority:** P1
**Sprint:** d7-v6 follow-up
**PR Count:** 1
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**SoT:** `devrix-d7-six-s-simplification` (DM-20260626-001) acceptance-report.md §7 后续工作 + §3 Cross-cutting Hardening 章节

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

9 个 spec 文档已在 PR #215 (commit 0ce5e52) + execute/learn → mups/ 物理迁移已在 PR #216 (commit 64ec427) 落地。但 **横切 Hardening 仍只有文档声明**，代码侧 `orchestration/hardening/` 目录不存在，对应的横切关注点散落在各处：

| 散落位置 | 内容 | 行数 |
|---------|------|------|
| `sessionorchestrator/metrics.go` | InterruptMetrics 跨 canceller 失败计数器 | 61 |
| `turn/recovery.go` | LLM 错误恢复 helpers + stream recovery tombstone | 133 |
| `escape/circuit_breaker.go` | 5-Layer CircuitBreaker (L0-L5) | 420 (本次**不动**) |

## 2. Problem Statement

虽然 6 S + 1 横切文档已显式声明"横切 Discipline Keeper → `orchestration/hardening/`"，但代码侧散落模式与文档承诺不符：

**问题：**

1. **横切观测散落**：`metrics.go` (InterruptMetrics) 是跨 canceller 计数器，纯横切性质，却放在 `sessionorchestrator/` 包内
2. **横切错误恢复散落**：`recovery.go` 是跨 S LLM 错误恢复 helpers（IsContextLengthError + IsOverloadOr5xx + compressMessagesForRecovery 等），却放在 `turn/` 包内
3. **横切包目录不存在**：`orchestration/hardening/` 在 v6.0.0 文档中提及但代码侧从未创建
4. **bootstrap 14 wire 收口受阻**：follow-up #6 (`devrix-d7-6s-bootstrap-slim`) 需要 hardening/ 就位，否则 wire_coordinator.go 仍需引用 turn/ + escape/ + sessionorchestrator/ 3 处散落实现

## 3. Proposed Solution

**创建 `orchestration/hardening/` 新包，把 2 类真正横切的关注点收口：**

```
当前：                              目标：
orchestration/                     orchestration/
├── sessionorchestrator/           ├── hardening/  (NEW, Discipline Keeper)
│   ├── metrics.go           ───►  │   ├── doc.go        (NEW, 说明横切职责)
│   ├── metrics_test.go      ───►  │   ├── metrics.go
│   ├── interrupt.go (引用)        │   ├── metrics_test.go
│   └── interrupt_test.go (引用)   │   ├── recovery.go
├── turn/                          │   └── recovery_test.go
│   ├── recovery.go         ───►   ├── sessionorchestrator/
│   ├── recovery_test.go    ───►   │   ├── interrupt.go (引用 hardening.InterruptMetrics)
│   └── orchestrator.go (引用)     │   └── interrupt_test.go (引用)
├── escape/                        ├── turn/
│   └── circuit_breaker.go (不动)   │   └── orchestrator.go (引用 hardening helpers)
└── ...                            ├── escape/
                                   │   └── circuit_breaker.go (V5 EscapeEngine 不动)
                                   └── ...
```

**关键决策（详见 design.md §3）：**

1. **circuit_breaker.go 不迁 hardening/**：原计划 3 文件全迁，但探查发现 `circuit_breaker.go` 420 行 5-Layer CB 是 EscapeEngine 核心机制（engine.go + loop_depth_tracker.go 深度依赖，CB 决策喂给 EscapeEngine），不是横切观测。如果硬迁会违反依赖方向（EscapeEngine → hardening）。保留在 escape/。

2. **receiver methods 同步迁移**：recovery.go 的 `compressMessagesForRecovery` 和 `invokeStreamWithRecovery` 是 `*DefaultOrchestrator` 方法，与 turn/ orchestrator 紧耦合。**整文件 git mv 到 hardening/ 后改名 `package hardening`**，方法 receiver 同步改为 `*hardening.RecoveryOrchestrator` 之类的 wrapper，或保留方法但 orchestrator 内部组合使用（详见 design.md Decision 2）。

3. **metrics_test.go 同步迁**：原 sessionorchestrator/metrics_test.go 同包内可访问 unexported 字段，迁到 hardening/ 后需 import hardening 才能调用 `&InterruptMetrics{}` — 同 package 测试更直接，所以 git mv 同步迁移。

## 4. Success Metrics

| 指标 | 当前 | 目标 |
|------|------|------|
| **`orchestration/hardening/` 目录** | 0 文件 | 5 文件 (doc.go + metrics.go + metrics_test.go + recovery.go + recovery_test.go) |
| **metrics.go 散落位置** | sessionorchestrator/ | hardening/ |
| **recovery.go 散落位置** | turn/ | hardening/ |
| **circuit_breaker.go 位置** | escape/ | escape/ (保持) |
| **`grep "sessionorchestrator/metrics"`** | 1 命中 | 0 |
| **`grep "turn/recovery"`** | 2 命中 (recovery.go + recovery_test.go) | 0 |
| **D7 orchestration 子包数** | 22 (含 mups) | 23 (+hardening) |
| **go test -race 通过包数** | 22/22 | 23/23 |
| **D7-S7-A01 + D7-S7-A02 新 T 点** | 0 | 4 PLANNED → 4 IMPLEMENTED |
| **circuit_breaker.go diff** | — | 0 (git diff 无变更) |
| **LP-1 / LP-2 / LP-5 路径变化** | — | 0 (行为不变) |

## 5. Implementation Plan

### Step 1: 物理目录创建 + git mv (0.1 天)

```bash
# 创建 hardening/ 父目录
mkdir -p internal/layers/orchestration/hardening

# git mv 4 个 .go 文件（保留 git history）
git mv internal/layers/orchestration/sessionorchestrator/metrics.go \
       internal/layers/orchestration/hardening/metrics.go
git mv internal/layers/orchestration/sessionorchestrator/metrics_test.go \
       internal/layers/orchestration/hardening/metrics_test.go
git mv internal/layers/orchestration/turn/recovery.go \
       internal/layers/orchestration/hardening/recovery.go
git mv internal/layers/orchestration/turn/recovery_test.go \
       internal/layers/orchestration/hardening/recovery_test.go

# 新建 doc.go
echo '// Package hardening provides cross-cutting discipline keeping...'
# (手写 hardening/doc.go)

# 物理删除旧文件
git rm internal/layers/orchestration/sessionorchestrator/metrics.go
git rm internal/layers/orchestration/sessionorchestrator/metrics_test.go
git rm internal/layers/orchestration/turn/recovery.go
git rm internal/layers/orchestration/turn/recovery_test.go

# 验证
ls internal/layers/orchestration/hardening/  # 应有 5 .go files
ls internal/layers/orchestration/sessionorchestrator/metrics* 2>&1  # No such file
ls internal/layers/orchestration/turn/recovery* 2>&1                # No such file
head -1 internal/layers/orchestration/hardening/metrics.go   # package hardening
head -1 internal/layers/orchestration/hardening/recovery.go  # package hardening
```

**预期 commit 影响**: 0 编译错误（package rename + import path 还未替换），仅文件移动。

### Step 2: Import Path + Package Rename 全仓替换 (0.2 天)

```bash
# 列出所有引用 metrics.go + recovery.go 的文件
grep -rln "sessionorchestrator\.InterruptMetrics\|turn\.IsContextLengthError\|turn\.IsOverloadOr5xx\|turn\.NeedsMaxOutputTokenRecovery\|turn\.emitStreamRecoveryTombstones\|turn\.partialStreamEmit" internal/ cmd/

# 全仓 sed 替换 import path
# (具体 sed 命令见 design.md §4 Key Interfaces)

# 验证 0 残留
grep -rln "sessionorchestrator\.InterruptMetrics" internal/ cmd/   # 0 命中
grep -rln "turn\.IsContextLengthError" internal/ cmd/               # 0 命中
```

**改动的文件清单（预估 6 文件）：**
- `internal/layers/orchestration/sessionorchestrator/interrupt.go` — Metrics 字段类型 `*InterruptMetrics` → `*hardening.InterruptMetrics` + 加 import
- `internal/layers/orchestration/sessionorchestrator/interrupt_test.go` — 加 import
- `internal/layers/orchestration/turn/orchestrator.go` — 调用 hardening helpers
- `internal/layers/orchestration/hardening/metrics.go` — `package sessionorchestrator` → `package hardening`
- `internal/layers/orchestration/hardening/recovery.go` — `package turn` → `package hardening` + receiver 调整

### Step 3: 编译 + 测试回归 (0.1 天)

```bash
go build ./...          # 0 错误
go vet ./...            # 0 警告
go test -race -count=1 ./internal/layers/orchestration/...  # 23/23 PASS (含 hardening 1 新包)
```

### Step 4: 文档同步 (0.1 天)

- `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 §① Cross-cutting Hardening 章节包路径描述更新
- `openspec/specs/d7-orchestration/design.md` v4.0.0 §① 横切 Discipline Keeper 包路径描述更新
- `openspec/specs/d7-orchestration/t-registry.md` v4.2.0 → v4.3.0 (新增 4 P0 T)
- `openspec/t-registry.md` (root) v5.2.0 → v5.3.0 (新增 DM-20260626-003 增量条目 + D7 行更新)

## 6. Risks & Mitigations

| 风险 | 等级 | 缓解 |
|------|------|------|
| interrupt.go Metrics 字段类型跨包引用 | 中 | 中断点：metrics 字段类型 `*hardening.InterruptMetrics` + interrupt.go 加 import hardening |
| recovery.go 的 receiver method (compressMessagesForRecovery + invokeStreamWithRecovery) 跨包调用 | 中 | 详见 design.md §3 Decision 2: receiver 改为 `*Orchestrator` 内部组合使用 hardening helpers；或保留在 turn/ 而 hardening/ 只暴露纯函数 |
| hardening 包新建无 doc.go | 低 | Step 1 同时创建 hardening/doc.go 说明横切职责（Discipline Keeper）|
| 中间态编译失败 | 低 | Step 1 (git mv) + Step 2 (sed) + Step 3 (test) 分离；PR 未合入不影响 master |
| 23 包 -race 测试 CI flaky | 低 | hardening 包是从已有文件迁移，逻辑 0 变化，flaky 风险同 baseline |

## 7. Out of Scope

- ❌ 不动 EscapeEngine 5-Layer CB（`escape/circuit_breaker.go` 整文件保留）
- ❌ 不改 InterruptMetrics / recovery helpers 函数签名（仅物理迁移）
- ❌ 不改 bootstrap wire 14 → 6（follow-up #6 单独处理）
- ❌ 不动 5 个新 P0/P1 Span emit 路径
- ❌ 不动 LP-1 / LP-2 / LP-5 路径
- ❌ 不动其他 follow-up PR (6s-package-merge + 6s-verify-promotion + 6s-observe-merge + 6s-bootstrap-slim)
- ❌ 不新增 hardening/ 内的"CB observability wrapper"（如果未来需要单独 PR 处理）

## 8. Change Manifest

**新建文件 (1)：**
- `internal/layers/orchestration/hardening/doc.go`

**迁移文件 (4)：**
- `internal/layers/orchestration/{sessionorchestrator → hardening}/metrics.go`
- `internal/layers/orchestration/{sessionorchestrator → hardening}/metrics_test.go`
- `internal/layers/orchestration/{turn → hardening}/recovery.go`
- `internal/layers/orchestration/{turn → hardening}/recovery_test.go`

**删除文件 (4)：** （git mv 后原位置自动消失，无需手动 rm）

**改动文件 (3)：**
- `internal/layers/orchestration/sessionorchestrator/interrupt.go` — 加 import + Metrics 字段类型
- `internal/layers/orchestration/sessionorchestrator/interrupt_test.go` — 加 import
- `internal/layers/orchestration/turn/orchestrator.go` — 加 import hardening + 调用 helpers

**不改文件 (1)：**
- `internal/layers/orchestration/escape/circuit_breaker.go` — V5 EscapeEngine 核心机制

**文档同步 (4)：**
- `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 → v2.1.0
- `openspec/specs/d7-orchestration/design.md` v4.0.0 → v4.1.0
- `openspec/specs/d7-orchestration/t-registry.md` v4.2.0 → v4.3.0
- `openspec/t-registry.md` (root) v5.2.0 → v5.3.0
