# Proposal: D7 turn/ 包合并到 sessionorchestrator/

**Change ID:** `devrix-d7-6s-package-merge`
**Demand ID:** DM-20260626-004
**Priority:** P1
**Sprint:** d7-v6 follow-up #3
**PR Count:** 1
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**SoT:** `devrix-d7-six-s-simplification` (DM-20260626-001) acceptance-report.md §7 后续工作 + `devrix-d7-hardening-cross-cutting` (DM-20260626-003) acceptance-report.md §7 后续工作

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

6 S 文档已在 PR #215 (commit 0ce5e52) 中完整重写并 S7_Archived。物理代码包路径迁移作为 6 个 follow-up 推进：

```
v6.0.0 follow-up 序列（DM-20260626-001/002/003/004/005/006/007）：
#1  devrix-d7-six-s-simplification         ✅ S7_Archived (PR #215) — 14 S → 6 S + 1 横切文档
#1.5 devrix-d7-mups-package-migration      ✅ S7_Archived (PR #216) — execute/ + learn/ → mups/
#2  devrix-d7-hardening-cross-cutting      ✅ S7_Archived (PR #218+#219) — hardening/ 横切包物理落地
#3  devrix-d7-6s-package-merge             📋 本 change — turn/ + autoclose → sessionorchestrator/
#4  devrix-d7-6s-verify-promotion          📋 PLANNED — exit_reason + observe/verify → executionflow/verify/
#5  devrix-d7-6s-observe-merge             📋 PLANNED — observe/orchtypes → decisionplanning/
#6  devrix-d7-6s-bootstrap-slim             📋 PLANNED — wire 14 → 6, **依赖 #3/#4/#5 完成**
```

## 2. Problem Statement

虽然 v6.0.0 spec/code 语义层已对齐 6 S + 1 横切（#1 spec + #1.5 mups 子树 + #2 hardening 横切包都已 S7_Archived），但 **S2 SessionOrchestrator 角色仍被两个物理包拆开**：

```
当前 (v6.0.0 follow-up #2 后)：         目标 (v6.0.0 follow-up #3 后)：
orchestration/                           orchestration/
├── sessionorchestrator/                 ├── sessionorchestrator/  (扩展至 ~60 文件)
│   ├── autoclose.go                     │   ├── autoclose.go (已就位)
│   ├── orchestrator.go (35 文件)        │   ├── orchestrator.go (原)
│   ├── dispatch.go                      │   ├── dispatch.go (原)
│   ├── ...                              │   ├── ... (原 35 文件)
│   └── turn_tools.go (已 import turn)   │   ├── turn_tools.go (改引用 sessionorchestrator.X)
├── turn/ (25 .go, 6467 行)             │   ├── orchestrator.go (NEW, 来自 turn/, 1462 行)
│   ├── orchestrator.go (Default)        │   ├── orchestrator_test.go (NEW, 来自 turn/, 2100 行)
│   ├── recovery.go (receiver methods)   │   ├── recovery.go (NEW, receiver methods)
│   ├── llm.go + subturn.go              │   ├── llm.go + subturn.go (NEW)
│   ├── exit_reason.go (72 行)           │   ├── exit_reason.go (临时留, 等 #4 promote)
│   └── ...                              │   └── ... (其余 22 .go)
├── hardening/ (5 文件)                  ├── hardening/ (不变)
└── ...                                  └── ...
```

**问题：**

1. **S2 博弈角色被两个物理包拆开**：`sessionorchestrator/` 是 Mediator+Turn Leader 顶层入口，`turn/` 是 RunTurn 主循环实现。v6.0.0 spec 说"S2 = Mediator+Turn Leader+Error Recovery"是单一博弈角色，但代码层两个独立 Go 包
2. **`turn_tools.go` 已在 `sessionorchestrator/`**（#1 落地时已并入），但 `turn/orchestrator.go` (1462 行 DefaultOrchestrator) 还在 `turn/`，导致 sessionorchestrator/ 包内部职责不完整
3. **`turn/` 11 个核心 type + 6 个核心函数被 12 个 importer 跨包调用**，打破 S2 单包封装（12 个 importer 包括 10 个 bootstrap 文件 + 2 个 decisionplanning 文件 + 2 个 sessionorchestrator/turn_tools）
4. **bootstrap 14 wire 收口受阻**：wire_coordinator.go 中需分别 wire `turn.NewOrchestrator` + `sessionorchestrator.NewSessionOrchestrator` 两个独立包，未来 #6 (devrix-d7-6s-bootstrap-slim) 14 wire → 6 wire 还需要再做一次 wire 收敛
5. **`turn/recovery.go` + `turn/recovery_test.go` 在 hardening/ 落地后已是子集**（receiver methods 留 turn/，4 纯函数 + 1 const 已迁 hardening/），receiver methods 现在随 turn/ 迁入 sessionorchestrator/，调用 hardening 0 变化

## 3. Proposed Solution

**把 `orchestration/turn/` 25 个 .go 文件（6467 行）整包 git mv 到 `orchestration/sessionorchestrator/`，`package turn` 改为 `package sessionorchestrator`：**

```
当前：                              目标：
orchestration/                     orchestration/
├── sessionorchestrator/           ├── sessionorchestrator/  (~60 文件 ~15000 行)
│   ├── autoclose.go (已就位)       │   ├── autoclose.go (原)
│   ├── orchestrator.go (顶层)      │   ├── orchestrator.go (原 SessionOrchestrator 顶层)
│   ├── ... (33 文件)               │   ├── dispatch.go + interrupt.go + ... (原 33 文件)
│   └── turn_tools.go (import turn) │   ├── turn_tools.go (改 sessionorchestrator.X 引用)
├── turn/ (25 .go, 6467 行)        │   ├── orchestrator.go (NEW, 来自 turn/, DefaultOrchestrator)
│   ├── orchestrator.go (1462 行)   │   ├── orchestrator_test.go (NEW, 来自 turn/, 2100 行)
│   ├── recovery.go (receiver)      │   ├── recovery.go (NEW, receiver methods 留 sessionorchestrator/)
│   ├── llm.go + subturn.go         │   ├── llm.go + subturn.go (NEW)
│   ├── exit_reason.go (72 行)      │   ├── exit_reason.go (临时留, 等 #4 promote)
│   └── ... (其余 22 文件)          │   └── ... (其余 22 文件)
├── hardening/ (5 文件)            ├── hardening/ (不变)
└── ... (其他 14 个包)              └── ... (其他 14 个包, 不变)
```

**关键决策（详见 design.md §3）：**

1. **整包迁移 + 改 package 声明**：与 #2 (mups migration) 策略不同，#2 保留 `package execute` / `package learn` 不变（execute 是叶子包，learn 是叶子包），#3 必须改 `package turn` → `package sessionorchestrator`（因为目标是包合并，非目录迁移）
2. **type 名不改**：DefaultOrchestrator / OrchestratorDeps / SubTurnRunner / GatewayInvoker / CompressionSummarizer 等 11 个核心 type 全部保留原名（避免调用方大改）
3. **exit_reason.go + verdict_to_exit_reason.go 临时留 sessionorchestrator/**：这 2 文件（121 行）+ verdict_to_exit_reason_test.go（97 行）随 turn/ 迁入 sessionorchestrator/，由后续 follow-up #4 (`devrix-d7-6s-verify-promotion` / DM-20260626-005) 从 sessionorchestrator/ promote 到 executionflow/verify/
4. **OrchestratorOption 同名问题**：turn/orchestrator.go 已定义 `OrchestratorOption`（func type），sessionorchestrator/orchestrator.go 也定义了同名 `OrchestratorOption`（不同 func type），design.md Decision 2 详查是否需要 alias
5. **receiver methods 留在 sessionorchestrator/**：hardening/ 落地时（DM-20260626-003）receiver methods `compressMessagesForRecovery` + `invokeStreamWithRecovery` 紧耦合 `*DefaultOrchestrator`，hardening/ 已收口 4 纯函数 + 1 const；本次 turn/ → sessionorchestrator/ 后 receiver methods 类型不变（仍为 `*DefaultOrchestrator`），调用 hardening.IsContextLengthError 不变

**Import path 替换（12 处 importer）：**
- `internal/bootstrap/wire_coordinator.go` (1)
- `internal/bootstrap/turn_wiring.go` (1)
- `internal/bootstrap/turn_adapter.go` (1)
- `internal/bootstrap/turn_adapter_test.go` (1)
- `internal/bootstrap/turn_adapter_persist_test.go` (1)
- `internal/bootstrap/turn_adapter_permission_test.go` (1)
- `internal/bootstrap/turn_adapter_surface_test.go` (1)
- `internal/bootstrap/context_engine.go` (1)
- `internal/bootstrap/context_engine_builder.go` (1)
- `internal/bootstrap/plan_llm_completer.go` (1)
- `internal/layers/orchestration/decisionplanning/llm_decomposer.go` (1)
- `internal/layers/orchestration/decisionplanning/llm_decomposer_test.go` (1)
- `internal/layers/orchestration/sessionorchestrator/turn_tools.go` (1) — 改 import path
- `internal/layers/orchestration/sessionorchestrator/turn_tools_test.go` (1) — 改 import path

总计 14 处 import path 替换（10 bootstrap + 2 decisionplanning + 2 sessionorchestrator/turn_tools）

## 4. Success Metrics

| 指标 | 当前 | 目标 |
|------|------|------|
| **`orchestration/turn/` 目录** | 25 文件 6467 行 | 0（物理删除） |
| **`orchestration/sessionorchestrator/` 文件数** | 35 文件 ~7500 行 | ~60 文件 ~15000 行 |
| **`package sessionorchestrator` 一致性** | 35 文件 | ~60 文件 |
| **`grep "orchestration/turn\""` 命中数** | 14 (12 importer + 2 自身) | 0 |
| **`grep "turn\.NewOrchestrator\|turn\.DefaultOrchestrator"` 等跨包调用** | ~40 处 | 0 |
| **`sessionorchestrator/` 包 export 数** | ~35 个 type/func | ~46 个 type/func (35 + 11 turn 核心) |
| **D7 orchestration 子包数** | 15（含 hardening + mups + sessionorchestrator + turn + ...） | 14 (turn 合并到 sessionorchestrator) |
| **`go test -race` 通过包数** | 23/23 (含 hardening) | 23/23 (持平 baseline, sessionorchestrator/ 仍是 1 包) |
| **D7-S2-A50 新 T 点** | 0 | 4 PLANNED → 4 IMPLEMENTED |
| **`escape/circuit_breaker.go` 变化** | — | 0 (Decision: 不动) |
| **`hardening/` 变化** | — | 0 (Decision: hardening 落地后 receiver methods 调用 hardening 不变) |
| **`sessionorchestrator/autoclose.go` 变化** | — | 0 (Decision: 已在 sessionorchestrator/) |
| **LP-1 / LP-2 / LP-5 路径变化** | — | 0 (行为不变) |
| **verify-archive.sh** | 12/12 PASS (hardening) | 12/12 PASS (本次) |

## 5. Implementation Plan

### Step 1: 物理目录迁移 + package 改名 (0.3 天)

```bash
# 创建目标目录（如果 sessionorchestrator/ 不存在则需 mkdir，但已存在）
# mkdir -p internal/layers/orchestration/sessionorchestrator/

# 25 个 .go 文件 git mv (保留 git history)
git mv internal/layers/orchestration/turn/orchestrator.go \
       internal/layers/orchestration/sessionorchestrator/orchestrator.go
git mv internal/layers/orchestration/turn/orchestrator_test.go \
       internal/layers/orchestration/sessionorchestrator/orchestrator_test.go
git mv internal/layers/orchestration/turn/orchestrator_toolcap_test.go \
       internal/layers/orchestration/sessionorchestrator/orchestrator_toolcap_test.go
git mv internal/layers/orchestration/turn/compression_summarizer.go \
       internal/layers/orchestration/sessionorchestrator/compression_summarizer.go
git mv internal/layers/orchestration/turn/compression_summarizer_test.go \
       internal/layers/orchestration/sessionorchestrator/compression_summarizer_test.go
git mv internal/layers/orchestration/turn/contracts.go \
       internal/layers/orchestration/sessionorchestrator/contracts.go
git mv internal/layers/orchestration/turn/doc.go \
       internal/layers/orchestration/sessionorchestrator/turn_doc.go  # 避免与 sessionorchestrator/doc.go 冲突
git mv internal/layers/orchestration/turn/exit_reason.go \
       internal/layers/orchestration/sessionorchestrator/exit_reason.go  # 临时留, 等 #4 promote
git mv internal/layers/orchestration/turn/fake_gateway_test.go \
       internal/layers/orchestration/sessionorchestrator/fake_gateway_test.go
git mv internal/layers/orchestration/turn/focus_hint.go \
       internal/layers/orchestration/sessionorchestrator/focus_hint.go
git mv internal/layers/orchestration/turn/llm.go \
       internal/layers/orchestration/sessionorchestrator/llm.go
git mv internal/layers/orchestration/turn/llm_test.go \
       internal/layers/orchestration/sessionorchestrator/llm_test.go
git mv internal/layers/orchestration/turn/recovery.go \
       internal/layers/orchestration/sessionorchestrator/recovery.go
git mv internal/layers/orchestration/turn/recovery_test.go \
       internal/layers/orchestration/sessionorchestrator/recovery_test.go
git mv internal/layers/orchestration/turn/resolve_await.go \
       internal/layers/orchestration/sessionorchestrator/resolve_await.go
git mv internal/layers/orchestration/turn/runturn_main_path_test.go \
       internal/layers/orchestration/sessionorchestrator/runturn_main_path_test.go
git mv internal/layers/orchestration/turn/subturn.go \
       internal/layers/orchestration/sessionorchestrator/subturn.go
git mv internal/layers/orchestration/turn/subturn_test.go \
       internal/layers/orchestration/sessionorchestrator/subturn_test.go
git mv internal/layers/orchestration/turn/subturn_fork_test.go \
       internal/layers/orchestration/sessionorchestrator/subturn_fork_test.go
git mv internal/layers/orchestration/turn/tool_stream.go \
       internal/layers/orchestration/sessionorchestrator/tool_stream.go
git mv internal/layers/orchestration/turn/tool_stream_test.go \
       internal/layers/orchestration/sessionorchestrator/tool_stream_test.go
git mv internal/layers/orchestration/turn/tracing.go \
       internal/layers/orchestration/sessionorchestrator/tracing.go  # 与 sessionorchestrator/tracing.go 同名, 需合并
git mv internal/layers/orchestration/turn/verdict_to_exit_reason.go \
       internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go  # 临时留, 等 #4
git mv internal/layers/orchestration/turn/verdict_to_exit_reason_test.go \
       internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go

# 物理删除 turn/ 目录（git mv 已自动处理）
ls internal/layers/orchestration/turn/  # 应 "No such file or directory"
```

**冲突检查（design.md Decision 2 详查）：**
- `doc.go` — turn/doc.go 与 sessionorchestrator/doc.go 同名，git mv 后 turn/doc.go 改名为 `turn_doc.go`
- `tracing.go` — turn/tracing.go 与 sessionorchestrator/tracing.go 同名，git mv 后需检查内容是否冲突（可能合并或重命名）

### Step 2: Package 改名 + import path 全仓替换 (0.3 天)

```bash
# 25 个迁入文件 package 声明: package turn → package sessionorchestrator
sed -i '' 's|^package turn$|package sessionorchestrator|' \
  internal/layers/orchestration/sessionorchestrator/{orchestrator,orchestrator_test,orchestrator_toolcap_test}.go \
  internal/layers/orchestration/sessionorchestrator/{compression_summarizer,compression_summarizer_test,contracts,turn_doc,exit_reason,fake_gateway_test,focus_hint,llm,llm_test,recovery,recovery_test,resolve_await,runturn_main_path_test,subturn,subturn_test,subturn_fork_test,tool_stream,tool_stream_test,tracing,verdict_to_exit_reason,verdict_to_exit_reason_test}.go

# 12 个 importer import path: orchestration/turn → orchestration/sessionorchestrator
grep -rln "orchestration/turn\"" internal/ cmd/ | xargs sed -i '' \
  's|internal/layers/orchestration/turn"|internal/layers/orchestration/sessionorchestrator"|g'

# 验证 0 残留
grep -rln "orchestration/turn\"" internal/ cmd/  # 必须 0 命中
grep -rln "package turn$" internal/layers/orchestration/sessionorchestrator/  # 必须 0 命中
```

### Step 3: 编译 + 测试回归 (0.2 天)

```bash
go build ./...          # 0 错误
go vet ./...            # 0 警告
go test -race -count=1 ./internal/layers/orchestration/...  # 23/23 PASS

# LP-1 / LP-2 / LP-5 集成测试
go test ./internal/layers/orchestration/sessionorchestrator/... -race -run "TestAutoClose_FullLP1Loop"
go test ./internal/layers/orchestration/... -race -run "TestIntegration_5NodePipeline_End2End"
```

### Step 4: 文档同步 (0.1 天)

```bash
# d7-domain.md v2.1.0 → v2.2.0 §① S2 SessionOrchestrator 章节包路径描述更新
# design.md v4.1.0 → v4.2.0 §① S2 SessionOrchestrator 包路径描述更新
# t-registry.md (域): D7-S2-A50-T01..T04 状态 PLANNED → IMPLEMENTED + v4.3.0 → v4.4.0
# t-registry.md (root): v5.3.0 → v5.4.0 + 新增条目
```

## 6. Risks & Mitigations

| 风险 | 等级 | 缓解 |
|------|------|------|
| `sessionorchestrator/` 包扩大至 ~60 文件 ~15000 行 | 中 | v6.0.0 设计目标 — S2 Mediator+Turn Leader 复合角色；接受包大小换取角色内聚；Go 增量编译无性能影响 |
| `doc.go` / `tracing.go` 与 sessionorchestrator/ 同名冲突 | 中 | doc.go 改名为 `turn_doc.go`；tracing.go 内容比对决定合并或重命名（详见 design.md Decision 2） |
| `OrchestratorOption` 同名冲突（turn + sessionorchestrator） | 中 | design.md Decision 2 详查；可能需要 rename turn/orchestrator.go 中 `OrchestratorOption` 为 `TurnOrchestratorOption` |
| 12 个 importer import path 替换 | 中 | 全仓 `grep -rln "orchestration/turn\""` 列出后 `sed -i ''` 一次性替换；同包内 bare name 引用 0 影响 |
| `sessionorchestrator/turn_tools.go` 内部 turn.X 引用 | 中 | 该文件已 import turn，本次需同时更新 import + 内部 turn.X 引用 → sessionorchestrator.X |
| receiver methods (compressMessagesForRecovery + invokeStreamWithRecovery) 跨包类型 | 低 | hardening/ 落地时已确认 receiver 紧耦合 `*DefaultOrchestrator`；迁 sessionorchestrator/ 后类型不变 |
| CI 镜像缓存导致旧路径仍编译过 | 低 | 删除 turn/ 目录后强制 re-build；CI 单测 100% PASS 是硬门禁 |
| LP-1 / LP-2 / LP-5 行为漂移 | 极低 | 0 函数逻辑变化，仅物理迁移 |
| 23 包 -race 测试 CI flaky | 低 | 物理迁移，逻辑 0 变化，flaky 风险同 hardening baseline |

## 7. Out of Scope

- ❌ 不动 `escape/circuit_breaker.go`（V5 EscapeEngine 核心机制）
- ❌ 不动 `hardening/` 横切包（receiver methods 调用 hardening 不变）
- ❌ 不动 `sessionorchestrator/autoclose.go`（已在 sessionorchestrator/）
- ❌ 不改任何函数签名、行为、对外接口（仅物理迁移）
- ❌ 不改 type 名字（DefaultOrchestrator / OrchestratorDeps 等保留原样）
- ❌ 不 promote exit_reason.go + verdict_to_exit_reason.go 到 executionflow/verify/（follow-up #4 处理）
- ❌ 不合并 observe/orchtypes → decisionplanning/（follow-up #5 处理）
- ❌ 不收敛 wire 14 → 6（follow-up #6 处理，依赖 #3+#4+#5）
- ❌ 不动 D7 5 个新 P0/P1 Span emit 路径
- ❌ 不动 LP-1 / LP-2 / LP-5 路径
- ❌ 不动 multiagent/ 域

## 8. Change Manifest

**迁移文件（25）：**
- `internal/layers/orchestration/turn/orchestrator.go` → `sessionorchestrator/orchestrator.go` (1462 行)
- `internal/layers/orchestration/turn/orchestrator_test.go` → `sessionorchestrator/orchestrator_test.go` (2100 行)
- `internal/layers/orchestration/turn/orchestrator_toolcap_test.go` → `sessionorchestrator/orchestrator_toolcap_test.go` (263 行)
- `internal/layers/orchestration/turn/compression_summarizer.go` → `sessionorchestrator/compression_summarizer.go` (98 行)
- `internal/layers/orchestration/turn/compression_summarizer_test.go` → `sessionorchestrator/compression_summarizer_test.go` (144 行)
- `internal/layers/orchestration/turn/contracts.go` → `sessionorchestrator/contracts.go` (142 行)
- `internal/layers/orchestration/turn/doc.go` → `sessionorchestrator/turn_doc.go` (17 行, 改名避免冲突)
- `internal/layers/orchestration/turn/exit_reason.go` → `sessionorchestrator/exit_reason.go` (72 行, 临时留)
- `internal/layers/orchestration/turn/fake_gateway_test.go` → `sessionorchestrator/fake_gateway_test.go` (40 行)
- `internal/layers/orchestration/turn/focus_hint.go` → `sessionorchestrator/focus_hint.go` (8 行)
- `internal/layers/orchestration/turn/llm.go` → `sessionorchestrator/llm.go` (102 行)
- `internal/layers/orchestration/turn/llm_test.go` → `sessionorchestrator/llm_test.go` (495 行)
- `internal/layers/orchestration/turn/recovery.go` → `sessionorchestrator/recovery.go` (84 行, hardening 落地后子集)
- `internal/layers/orchestration/turn/recovery_test.go` → `sessionorchestrator/recovery_test.go` (130 行, hardening 落地后子集)
- `internal/layers/orchestration/turn/resolve_await.go` → `sessionorchestrator/resolve_await.go` (8 行)
- `internal/layers/orchestration/turn/runturn_main_path_test.go` → `sessionorchestrator/runturn_main_path_test.go` (38 行)
- `internal/layers/orchestration/turn/subturn.go` → `sessionorchestrator/subturn.go` (380 行)
- `internal/layers/orchestration/turn/subturn_test.go` → `sessionorchestrator/subturn_test.go` (466 行)
- `internal/layers/orchestration/turn/subturn_fork_test.go` → `sessionorchestrator/subturn_fork_test.go` (135 行)
- `internal/layers/orchestration/turn/tool_stream.go` → `sessionorchestrator/tool_stream.go` (30 行)
- `internal/layers/orchestration/turn/tool_stream_test.go` → `sessionorchestrator/tool_stream_test.go` (63 行)
- `internal/layers/orchestration/turn/tracing.go` → `sessionorchestrator/tracing.go` (44 行, 同名文件需合并)
- `internal/layers/orchestration/turn/verdict_to_exit_reason.go` → `sessionorchestrator/verdict_to_exit_reason.go` (49 行, 临时留)
- `internal/layers/orchestration/turn/verdict_to_exit_reason_test.go` → `sessionorchestrator/verdict_to_exit_reason_test.go` (97 行, 临时留)

**删除文件（25）：** （git mv 后原位置自动消失）

**修改文件（14）：**
- `internal/layers/orchestration/sessionorchestrator/turn_tools.go` — import path + 内部 turn.X 引用 → sessionorchestrator.X
- `internal/layers/orchestration/sessionorchestrator/turn_tools_test.go` — 同上
- `internal/bootstrap/wire_coordinator.go` — import path
- `internal/bootstrap/turn_wiring.go` — import path
- `internal/bootstrap/turn_adapter.go` — import path
- `internal/bootstrap/turn_adapter_test.go` — import path
- `internal/bootstrap/turn_adapter_persist_test.go` — import path
- `internal/bootstrap/turn_adapter_permission_test.go` — import path
- `internal/bootstrap/turn_adapter_surface_test.go` — import path
- `internal/bootstrap/context_engine.go` — import path
- `internal/bootstrap/context_engine_builder.go` — import path
- `internal/bootstrap/plan_llm_completer.go` — import path
- `internal/layers/orchestration/decisionplanning/llm_decomposer.go` — import path
- `internal/layers/orchestration/decisionplanning/llm_decomposer_test.go` — import path

**不改文件（4）：**
- `internal/layers/orchestration/escape/circuit_breaker.go` — V5 EscapeEngine 核心机制
- `internal/layers/orchestration/hardening/` — receiver methods 调用 hardening 不变
- `internal/layers/orchestration/sessionorchestrator/autoclose.go` — 已在 sessionorchestrator/
- `internal/layers/orchestration/sessionorchestrator/doc.go` — 保持原 sessionorchestrator 包说明

**文档同步（4）：**
- `openspec/specs/d7-orchestration/d7-domain.md` v2.1.0 → v2.2.0
- `openspec/specs/d7-orchestration/design.md` v4.1.0 → v4.2.0
- `openspec/specs/d7-orchestration/t-registry.md` v4.3.0 → v4.4.0
- `openspec/t-registry.md` (root) v5.3.0 → v5.4.0