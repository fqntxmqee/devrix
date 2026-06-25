# Design: D7 turn/ 包合并到 sessionorchestrator/

**Change ID:** `devrix-d7-6s-package-merge`
**Demand ID:** DM-20260626-004
**Status:** S3_Design → S3-Gate(Review) → S4_Implemented
**设计原则:** DSAFT + 6 S + 1 横切 (v6.0.0 域升级)
**前置:** devrix-d7-six-s-simplification (DM-20260626-001) + devrix-d7-mups-package-migration (DM-20260626-002) + devrix-d7-hardening-cross-cutting (DM-20260626-003) 全部 S7_Archived

---

## 1. 背景与目标

`devrix-d7-six-s-simplification` (DM-20260626-001) v6.0.0 域升级把 D7 编排层 14 S 精简为 6 S + 1 横切。S2 SessionOrchestrator 角色定义为 **Mediator + Turn Leader + Error Recovery**，是单一博弈角色。

但代码侧 `orchestration/turn/` 子包（25 .go, 6467 行）仍作为独立物理包存在，包含：
- `DefaultOrchestrator` + `RunTurn` 主循环（1462 行 orchestrator.go）
- LLM gateway (`GatewayInvoker`, `LLMInvokerDeps`)
- Compression (`CompressionSummarizer`)
- Sub-turn (`SubTurnRunner`, `SubTurnConfig`)
- Recovery (receiver methods 子集，hardening 落地后)
- 14 ExitReason 临时留 turn/（等 #4 promote）
- Tool stream + tracing + focus hint + resolve_await

`sessionorchestrator/` 包内已有 `turn_tools.go`（#1 落地时已并入），但 turn/ 主循环 + LLM 链路还在 turn/ 子包，违反 S2 单包封装。

**目标**：把 turn/ 25 .go 整包 git mv 到 sessionorchestrator/，`package turn` → `package sessionorchestrator`，让 S2 SessionOrchestrator 成为单包物理封装，为 follow-up #6 (devrix-d7-6s-bootstrap-slim) 14 wire → 6 wire 解锁。

---

## 2. 当前结构 vs 目标结构

### 2.1 当前结构（v6.0.0 follow-up #2 后）

```
orchestration/
├── sessionorchestrator/  (35 .go ~7500 行, package sessionorchestrator)
│   ├── orchestrator.go (SessionOrchestrator 顶层, OrchestratorOption func options)
│   ├── autoclose.go (processAutoClose)
│   ├── turn_tools.go (已 import "turn", 内部 turn.X 引用)
│   ├── dispatch.go + command_handler.go + interrupt.go + escape_wiring.go + ...
│   └── (32 其他文件)
├── turn/  (25 .go 6467 行, package turn)
│   ├── orchestrator.go (DefaultOrchestrator, 1462 行)
│   ├── recovery.go (receiver methods 子集, hardening 落地后)
│   ├── llm.go + subturn.go + compression_summarizer.go + ...
│   ├── exit_reason.go (72 行, 临时留)
│   └── verdict_to_exit_reason.go (49 行, 临时留)
├── hardening/  (5 文件, package hardening, DM-20260626-003 落地)
├── escape/  (V5 EscapeEngine, package escape)
├── mups/  (execute + learn 子树, DM-20260626-002 落地)
└── (其他 9 个包)
```

### 2.2 目标结构（v6.0.0 follow-up #3 后）

```
orchestration/
├── sessionorchestrator/  (~60 .go ~15000 行, package sessionorchestrator)
│   ├── orchestrator.go (原 SessionOrchestrator 顶层, OrchestratorOption 独占)
│   ├── orchestrator.go (NEW, 原 turn/orchestrator.go, DefaultOrchestrator, 1462 行)
│   ├── orchestrator_test.go (NEW, 原 turn/, 2100 行)
│   ├── orchestrator_toolcap_test.go (NEW)
│   ├── autoclose.go (原, 不变)
│   ├── turn_tools.go (MODIFY: import turn → sessionorchestrator, 内部 turn.X → sessionorchestrator.X)
│   ├── recovery.go (NEW, 原 turn/, receiver methods)
│   ├── recovery_test.go (NEW)
│   ├── llm.go + llm_test.go (NEW, 原 turn/)
│   ├── subturn.go + subturn_test.go + subturn_fork_test.go (NEW)
│   ├── compression_summarizer.go + _test.go (NEW)
│   ├── contracts.go (NEW, TurnOrchestrator 接口)
│   ├── turn_doc.go (NEW, 原 turn/doc.go 改名, 避免与 sessionorchestrator/doc.go 冲突)
│   ├── exit_reason.go (NEW, 临时留, 等 #4 promote)
│   ├── verdict_to_exit_reason.go + _test.go (NEW, 临时留)
│   ├── fake_gateway_test.go + focus_hint.go + resolve_await.go (NEW)
│   ├── runturn_main_path_test.go + tool_stream.go + _test.go (NEW)
│   ├── tracing_turn.go (NEW, 原 turn/tracing.go 改名, 避免与 sessionorchestrator/tracing.go 冲突)
│   ├── tracing.go (原 sessionorchestrator/, 保留 SessionOrchestrator.startSpan)
│   └── (其他 33 原 sessionorchestrator/ 文件)
├── hardening/  (5 文件, 不变)
├── escape/  (V5 EscapeEngine, 不变)
├── mups/  (execute + learn 子树, 不变)
└── (其他 9 个包)
```

---

## 3. 关键决策

### Decision 1: 整包迁移 + 改 package 声明 (与 #2 策略不同)

**方案**：25 个 .go 文件 git mv + `package turn` → `package sessionorchestrator` (1 行 sed per file)

**理由**：
- #2 (mups migration) 策略：保留 `package execute` / `package learn`，只改目录（叶子包，不需合并）
- #3 策略：必须改 `package turn` → `package sessionorchestrator`（目标是包合并，非目录迁移）
- sessionorchestrator/ 内 13 个 `With*(...) OrchestratorOption` (lines 91-184 of sessionorchestrator/orchestrator.go) 已是 functional options 模式，turn/orchestrator.go 的 `NewOrchestrator(deps OrchestratorDeps)` 是 struct deps 模式，**两套不同的构造模式，但无 OrchestratorOption 同名冲突**（详见 Decision 2）

**拒绝方案 A**: 把 turn/ 改名 turn.orchestrator → sessionorchestrator.turnorchestrator 子包（违反 6 S 单包目标，复杂化 wire）

**拒绝方案 B**: 保留 turn/ 独立子包，仅在 wire 层组装（重复 #2 的策略，不解决 S2 单包封装问题）

### Decision 2: OrchestratorOption 无冲突 + 2 个同名文件需重命名

**pre-S3 实测结果**（2026-06-26 grep 验证）：

| turn/ 导出 | sessionorchestrator/ 导出 | 冲突？ |
| ---------- | ------------------------- | ------ |
| `DefaultOrchestrator` (struct) | `SessionOrchestrator` (struct) | ❌ 不同 type |
| `NewOrchestrator(deps OrchestratorDeps)` | `NewSessionOrchestrator(cfg, executor, ...OrchestratorOption)` | ❌ 不同函数 |
| `OrchestratorDeps` (struct) | 无 | ❌ 不冲突 |
| `OrchestratorOption` (turn 中 0 个定义) | `OrchestratorOption` (func type, sessionorchestrator/orchestrator.go:88) | ❌ 不冲突（turn 不使用 OrchestratorOption） |
| `SubTurnRunner` / `GatewayInvoker` / `CompressionSummarizer` | 无 | ❌ 不冲突 |
| `TurnOrchestrator` (接口) | 无 | ❌ 不冲突 |
| `PreparedTurnAdapter` | 无 | ❌ 不冲突 |
| `FormatToolResultContentForLLM` | 无 | ❌ 不冲突 |
| `focus_hint.go` (unexported types) | 无 | ❌ 不冲突 |

**两个同名文件需重命名（避免 `git mv` 同名覆盖）：**

| 原文件名 | 改名后 | 理由 |
| ---------- | ------ | ---- |
| `turn/doc.go` | `sessionorchestrator/turn_doc.go` | sessionorchestrator/doc.go 已存在（"Package sessionorchestrator implements D7-S2 Session Orchestrator."），turn/doc.go 内容更长描述 Turn Leader 角色 |
| `turn/tracing.go` | `sessionorchestrator/tracing_turn.go` | sessionorchestrator/tracing.go 已存在（包含 `startObsSpan` + `(o *SessionOrchestrator).startSpan`），turn/tracing.go 包含 `(o *DefaultOrchestrator).startSpan` 是不同 receiver method。两个 tracing.go 内容不同，必须分文件 |

### Decision 3: type 名不改 (DefaultOrchestrator / SessionOrchestrator 维持双 type)

**方案**：保留 `DefaultOrchestrator` (turn 主循环) + `SessionOrchestrator` (顶层 mediator) 两个 type

**理由**：
- 两个 type 职责不同：
  - `DefaultOrchestrator` 是 RunTurn 主循环（1462 行），紧耦合 LLM gateway + compression + sub-turn
  - `SessionOrchestrator` 是顶层 mediator（35 文件），接收 process request → 调度 fastpath/orchestrate/turn
- type 名改 (`DefaultOrchestrator` → `SessionOrchestratorTurn`) 会导致：
  - 调用方大改（bootstrap 10 文件 + decisionplanning 2 文件 + turn_tools 2 文件）
  - 调用链 sessionorchestrator.SessionOrchestrator.processRequest → sessionorchestrator.SessionOrchestratorTurn.runTurn 名字混乱
- 当前命名清晰：`*SessionOrchestrator` 是入口，`*DefaultOrchestrator` 是 RunTurn 实现层
- 接受双 type 换取调用链清晰度

**拒绝方案 A**: 合并为单一 SessionOrchestrator（破坏 SRP，SessionOrchestrator 类膨胀至 5000+ 行）

**拒绝方案 B**: 把 DefaultOrchestrator 改名为 TurnLoop（命名变化大，跨包调用链全改）

### Decision 4: exit_reason.go + verdict_to_exit_reason.go 临时留 sessionorchestrator/

**方案**：turn/exit_reason.go (72 行) + verdict_to_exit_reason.go (49 行) + verdict_to_exit_reason_test.go (97 行) 随 turn/ 整体迁移到 sessionorchestrator/，由后续 follow-up #4 (`devrix-d7-6s-verify-promotion` / DM-20260626-005) 从 sessionorchestrator/ promote 到 executionflow/verify/

**理由**：
- exit_reason.go + verdict_to_exit_reason.go 是 14 ExitReason 定义 + Verdict→ExitReason 映射，属于 S4 ExecutionFlow+Verify 角色（Certifier），不是 S2 SessionOrchestrator 角色
- #1 spec 重写时把这部分逻辑规划到 S4，但 PR #215 物理迁移阶段仅做了 spec 未做代码迁移
- #3 物理合并阶段把 turn/ 整体迁 sessionorchestrator/，但 #4 是单独 PR 处理 exit_reason 从 sessionorchestrator/ 上提到 executionflow/verify/
- #3 范围内 exit_reason.go + verdict_to_exit_reason.go 临时留 sessionorchestrator/，不在 #3 promote（避免 #3 scope 蔓延）

**拒绝方案 A**: #3 直接 promote exit_reason 到 executionflow/verify/（scope 蔓延，应该独立 PR）

**拒绝方案 B**: #3 不迁 exit_reason，保留 turn/exit_reason.go（破坏整包迁移一致性）

### Decision 5: receiver methods 类型不变 (硬ening/ 调用 0 变化)

**方案**：`compressMessagesForRecovery` + `invokeStreamWithRecovery` receiver methods 迁 sessionorchestrator/ 后仍为 `func (o *DefaultOrchestrator)` 方法，调用 `hardening.IsContextLengthError` + `hardening.IsOverloadOr5xx` + `hardening.NeedsMaxOutputTokenRecovery` + `hardening.MaxOutputTokensRecoveryMessage` 不变

**理由**：
- hardening/ 落地 (DM-20260626-003 / PR #218+#219) 时 receiver methods 已确认紧耦合 `*DefaultOrchestrator`（紧耦合 `o.llm` + `o.runCompress` 字段）
- hardening/ 仅收口 4 纯函数 + 1 const，receiver methods 留 turn/（即现在的 sessionorchestrator/）
- #3 turn/ → sessionorchestrator/ 后 receiver methods 仍在 sessionorchestrator/ 内，但 receiver type 不变（仍为 `*DefaultOrchestrator`），调用 hardening/ 不变
- hardening/ 落地时已记录："receiver methods 留 turn/，Decision 2"，与本次 Decision 5 一致

---

## 4. 实施步骤

### Step 1: 物理目录迁移 + 2 个同名文件重命名 (0.3 天)

```bash
# 23 个直接 git mv (无同名冲突)
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
git mv internal/layers/orchestration/turn/exit_reason.go \
       internal/layers/orchestration/sessionorchestrator/exit_reason.go  # 临时留, 等 #4
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
git mv internal/layers/orchestration/turn/verdict_to_exit_reason.go \
       internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go  # 临时留, 等 #4
git mv internal/layers/orchestration/turn/verdict_to_exit_reason_test.go \
       internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go

# 2 个同名文件重命名 (避免 git mv 覆盖)
git mv internal/layers/orchestration/turn/doc.go \
       internal/layers/orchestration/sessionorchestrator/turn_doc.go
git mv internal/layers/orchestration/turn/tracing.go \
       internal/layers/orchestration/sessionorchestrator/tracing_turn.go

# 验证 turn/ 物理删除
ls internal/layers/orchestration/turn/  # 应 "No such file or directory"
ls internal/layers/orchestration/sessionorchestrator/ | wc -l  # 应 ~60 文件
```

**风险**：turn/ 物理删除必须发生在 git mv 后立即验证，避免遗留空目录。

### Step 2: Package 改名 + import path 全仓替换 (0.3 天)

```bash
# 25 个迁入文件 package 声明: package turn → package sessionorchestrator
sed -i '' 's|^package turn$|package sessionorchestrator|' \
  internal/layers/orchestration/sessionorchestrator/orchestrator.go \
  internal/layers/orchestration/sessionorchestrator/orchestrator_test.go \
  internal/layers/orchestration/sessionorchestrator/orchestrator_toolcap_test.go \
  internal/layers/orchestration/sessionorchestrator/compression_summarizer.go \
  internal/layers/orchestration/sessionorchestrator/compression_summarizer_test.go \
  internal/layers/orchestration/sessionorchestrator/contracts.go \
  internal/layers/orchestration/sessionorchestrator/turn_doc.go \
  internal/layers/orchestration/sessionorchestrator/exit_reason.go \
  internal/layers/orchestration/sessionorchestrator/fake_gateway_test.go \
  internal/layers/orchestration/sessionorchestrator/focus_hint.go \
  internal/layers/orchestration/sessionorchestrator/llm.go \
  internal/layers/orchestration/sessionorchestrator/llm_test.go \
  internal/layers/orchestration/sessionorchestrator/recovery.go \
  internal/layers/orchestration/sessionorchestrator/recovery_test.go \
  internal/layers/orchestration/sessionorchestrator/resolve_await.go \
  internal/layers/orchestration/sessionorchestrator/runturn_main_path_test.go \
  internal/layers/orchestration/sessionorchestrator/subturn.go \
  internal/layers/orchestration/sessionorchestrator/subturn_test.go \
  internal/layers/orchestration/sessionorchestrator/subturn_fork_test.go \
  internal/layers/orchestration/sessionorchestrator/tool_stream.go \
  internal/layers/orchestration/sessionorchestrator/tool_stream_test.go \
  internal/layers/orchestration/sessionorchestrator/tracing_turn.go \
  internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go \
  internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go

# 14 个 importer import path: orchestration/turn → orchestration/sessionorchestrator
for f in \
  internal/bootstrap/wire_coordinator.go \
  internal/bootstrap/turn_wiring.go \
  internal/bootstrap/turn_adapter.go \
  internal/bootstrap/turn_adapter_test.go \
  internal/bootstrap/turn_adapter_persist_test.go \
  internal/bootstrap/turn_adapter_permission_test.go \
  internal/bootstrap/turn_adapter_surface_test.go \
  internal/bootstrap/context_engine.go \
  internal/bootstrap/context_engine_builder.go \
  internal/bootstrap/plan_llm_completer.go \
  internal/layers/orchestration/decisionplanning/llm_decomposer.go \
  internal/layers/orchestration/decisionplanning/llm_decomposer_test.go \
  internal/layers/orchestration/sessionorchestrator/turn_tools.go \
  internal/layers/orchestration/sessionorchestrator/turn_tools_test.go ; do
  sed -i '' 's|internal/layers/orchestration/turn"|internal/layers/orchestration/sessionorchestrator"|g' "$f"
done

# 验证 0 残留
grep -rln "orchestration/turn\"" internal/ cmd/  # 必须 0 命中
grep -rln "package turn$" internal/layers/orchestration/  # 必须 0 命中 (turn/ 已删, sessionorchestrator/ 已改)
```

**额外**: sessionorchestrator/turn_tools.go 内部 `turn.X` 引用 → `sessionorchestrator.X`（同包内 bare name 引用应自动 0 改动，但需手工 grep 验证 turn_tools.go 是否曾 import "turn" 然后调用 turn.X — 大概率不需要改，因为同包后 bare name）

### Step 3: 编译 + 测试回归 (0.2 天)

```bash
go build ./...          # 0 错误
go vet ./...            # 0 警告
go test -race -count=1 ./internal/layers/orchestration/...  # 23/23 PASS

# LP-1 / LP-2 / LP-5 集成测试
go test ./internal/layers/orchestration/sessionorchestrator/... -race -run "TestAutoClose_FullLP1Loop"
go test ./internal/layers/orchestration/... -race -run "TestIntegration_5NodePipeline_End2End"

# 验证 escape/circuit_breaker.go + hardening/ + sessionorchestrator/autoclose.go 0 变化
git diff HEAD -- internal/layers/orchestration/escape/circuit_breaker.go  # 必须空
git diff HEAD -- internal/layers/orchestration/hardening/  # 必须空
git diff HEAD -- internal/layers/orchestration/sessionorchestrator/autoclose.go  # 必须空
```

### Step 4: 文档同步 (0.1 天)

- `openspec/specs/d7-orchestration/d7-domain.md` v2.1.0 → v2.2.0 §① S2 SessionOrchestrator 章节包路径描述
- `openspec/specs/d7-orchestration/design.md` v4.1.0 → v4.2.0 §① S2 SessionOrchestrator 包路径描述
- `openspec/specs/d7-orchestration/t-registry.md` v4.3.0 → v4.4.0 (新增 D7-S2-A50-T01..T04)
- `openspec/t-registry.md` (root) v5.3.0 → v5.4.0 (新增 DM-20260626-004 增量条目)

---

## 5. 关键接口 (跨包调用一览)

### 5.1 turn/ 11 个核心导出 type (迁 sessionorchestrator/ 后)

| Type | 签名 | 迁入文件 |
| ---- | ---- | -------- |
| `DefaultOrchestrator` | struct (含 llm, runCompress 等字段) | sessionorchestrator/orchestrator.go |
| `OrchestratorDeps` | struct (NewOrchestrator 参数) | sessionorchestrator/orchestrator.go |
| `TurnOrchestrator` | interface (RunTurn 契约) | sessionorchestrator/contracts.go |
| `OrchestratorOption` | (turn 不使用, sessionorchestrator/orchestrator.go 独占) | - |
| `SubTurnRunner` | struct (NewSubTurnRunner 返回) | sessionorchestrator/subturn.go |
| `SubTurnConfig` | struct (NewSubTurnRunner 参数) | sessionorchestrator/subturn.go |
| `GatewayInvoker` | struct (NewGatewayInvoker 返回) | sessionorchestrator/llm.go |
| `LLMInvokerDeps` | struct (NewGatewayInvoker 参数) | sessionorchestrator/llm.go |
| `CompressionSummarizer` | struct (NewCompressionSummarizer 返回) | sessionorchestrator/compression_summarizer.go |
| `CompressionSummarizerDeps` | struct (NewCompressionSummarizer 参数) | sessionorchestrator/compression_summarizer.go |
| `PreparedTurnAdapter` | struct (NewPreparedTurnAdapter 返回) | sessionorchestrator/orchestrator.go (推测, 待 Step 1 grep 确认) |

### 5.2 turn/ 6 个核心导出函数

| 函数 | 签名 | 迁入文件 |
| ---- | ---- | -------- |
| `NewOrchestrator` | `func NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator` | sessionorchestrator/orchestrator.go |
| `NewSubTurnRunner` | `func NewSubTurnRunner(orch TurnOrchestrator, cfg SubTurnConfig) *SubTurnRunner` | sessionorchestrator/subturn.go |
| `NewGatewayInvoker` | `func NewGatewayInvoker(deps LLMInvokerDeps) *GatewayInvoker` | sessionorchestrator/llm.go |
| `NewCompressionSummarizer` | `func NewCompressionSummarizer(deps CompressionSummarizerDeps) *CompressionSummarizer` | sessionorchestrator/compression_summarizer.go |
| `NewPreparedTurnAdapter` | `func NewPreparedTurnAdapter(orch TurnOrchestrator) *PreparedTurnAdapter` | sessionorchestrator/orchestrator.go |
| `FormatToolResultContentForLLM` | `func FormatToolResultContentForLLM(toolName, output, errMsg string) string` | sessionorchestrator/orchestrator.go (推测) |

### 5.3 跨包调用全仓替换 (14 importer)

| 文件 | 调用方式 | 替换 |
| ---- | -------- | ---- |
| `internal/bootstrap/wire_coordinator.go` | `turn.NewOrchestrator(deps)` | `sessionorchestrator.NewOrchestrator(deps)` |
| `internal/bootstrap/turn_wiring.go` | `turn.NewOrchestrator` | `sessionorchestrator.NewOrchestrator` |
| `internal/bootstrap/turn_adapter.go` | `turn.NewPreparedTurnAdapter` | `sessionorchestrator.NewPreparedTurnAdapter` |
| `internal/bootstrap/turn_adapter_test.go` | `turn.NewPreparedTurnAdapter` | `sessionorchestrator.NewPreparedTurnAdapter` |
| `internal/bootstrap/turn_adapter_persist_test.go` | 同上 | 同上 |
| `internal/bootstrap/turn_adapter_permission_test.go` | 同上 | 同上 |
| `internal/bootstrap/turn_adapter_surface_test.go` | 同上 | 同上 |
| `internal/bootstrap/context_engine.go` | `turn.X` | `sessionorchestrator.X` |
| `internal/bootstrap/context_engine_builder.go` | 同上 | 同上 |
| `internal/bootstrap/plan_llm_completer.go` | `turn.X` | `sessionorchestrator.X` |
| `internal/layers/orchestration/decisionplanning/llm_decomposer.go` | `turn.X` | `sessionorchestrator.X` |
| `internal/layers/orchestration/decisionplanning/llm_decomposer_test.go` | 同上 | 同上 |
| `internal/layers/orchestration/sessionorchestrator/turn_tools.go` | `turn.X` (跨包 import) | `sessionorchestrator.X` (改 import + bare name) |
| `internal/layers/orchestration/sessionorchestrator/turn_tools_test.go` | 同上 | 同上 |

---

## 6. 测试与验证

### 6.1 单元测试

- 25 个迁入文件包含 11 个测试文件（orchestrator_test.go + orchestrator_toolcap_test.go + compression_summarizer_test.go + recovery_test.go + subturn_test.go + subturn_fork_test.go + tool_stream_test.go + verdict_to_exit_reason_test.go + llm_test.go + fake_gateway_test.go + runturn_main_path_test.go），全部作为 sessionorchestrator/ 同包测试，go test 应自动适配
- sessionorchestrator/ 原 35 文件的测试 (entry_test.go + command_handler_test.go + dispatch_test.go + interrupt_test.go + orchestrate_path_test.go + orchestrator_autoclose_test.go + orchestrator_escape_test.go + orchestrator_learner_test.go + orchestrator_priorspan_test.go + orchestrator_resume_test.go + orchestrator_test.go + orchestrator_trackmode_test.go + tracing_test.go + d7_s6_a14_t06_test.go + turn_tools_test.go + validation_metrics_test.go) 也作为同包测试，go test 自动适配

### 6.2 集成测试

- LP-1 (Bayesian reputation): `TestAutoClose_FullLP1Loop` + `TestProcessAutoClose_*` 集成测试
- LP-2 (Memory 3 通道): `TestOrchestrator_AdvisoryValidator_*` 集成测试
- LP-5 (Cross-session traceability): `TestIntegration_5NodePipeline_End2End` 集成测试

### 6.3 编译验证

```bash
go build ./...                      # 0 错误
go vet ./...                        # 0 警告
go test ./internal/layers/orchestration/... -race -count=1  # 23/23 PASS
```

### 6.4 LP-1/LP-2/LP-5 兼容性

- 物理迁移不改变任何函数逻辑
- 0 函数签名变化
- 0 接口变化
- LP-1/LP-2/LP-5 路径应 0 变化（验证：git log --follow turn/orchestrator.go → sessionorchestrator/orchestrator.go 历史保留）

---

## 7. 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| `sessionorchestrator/` 包扩大至 ~60 文件 ~15000 行 | 中 | v6.0.0 设计目标 — S2 复合角色；Go 增量编译无性能影响；文件分类清晰（顶层 vs RunTurn 子循环） |
| `doc.go` + `tracing.go` 同名文件覆盖 | 中 | Decision 2 — 重命名为 `turn_doc.go` + `tracing_turn.go`，Step 1 git mv 时直接改名 |
| `OrchestratorOption` 误判同名冲突 | 低 | pre-S3 实测确认 turn/ 0 个 OrchestratorOption 定义，sessionorchestrator/orchestrator.go 独占 |
| 14 importer import path 替换遗漏 | 中 | Step 2 全仓 grep `orchestration/turn"` 0 命中验证；14 个 importer 列表在 design.md §5.3 |
| `sessionorchestrator/turn_tools.go` 内部 turn.X 引用遗漏 | 中 | Step 2 同时改 import path + 内部 turn.X 引用 → sessionorchestrator.X |
| `DefaultOrchestrator` + `SessionOrchestrator` 双 type 命名混乱 | 低 | Decision 3 — 接受双 type 换取职责清晰；调用链 SessionOrchestrator.processRequest → DefaultOrchestrator.runTurn 简洁 |
| `exit_reason.go` + `verdict_to_exit_reason.go` 临时留 sessionorchestrator/ | 低 | Decision 4 — 显式标注"等 #4 promote"，避免 scope 蔓延；Step 4 文档同步明确说明 |
| receiver methods 跨包类型变化 | 低 | Decision 5 — hardening/ 落地时已确认 receiver 紧耦合 `*DefaultOrchestrator`；迁 sessionorchestrator/ 后类型不变 |
| LP-1 / LP-2 / LP-5 行为漂移 | 极低 | 0 函数逻辑变化，仅物理迁移 |
| 23 包 -race 测试 CI flaky | 低 | 物理迁移，逻辑 0 变化，flaky 风险同 hardening baseline |
| CI 镜像缓存导致旧路径仍编译过 | 低 | turn/ 物理删除后强制 re-build；CI 单测 100% PASS 是硬门禁 |
| IDE/Goland 索引需要重新同步 | 极低 | 文档同步说明 + README 更新 |

---

## 8. Out of Scope

- ❌ 不动 `escape/circuit_breaker.go` (V5 EscapeEngine 核心机制)
- ❌ 不动 `hardening/` 横切包 (receiver methods 调用 hardening 不变)
- ❌ 不动 `sessionorchestrator/autoclose.go` (已在 sessionorchestrator/)
- ❌ 不改任何函数签名、行为、对外接口 (纯物理迁移 + import path 替换)
- ❌ 不改 type 名字 (DefaultOrchestrator / SessionOrchestrator / OrchestratorDeps 等保留)
- ❌ 不 promote exit_reason.go + verdict_to_exit_reason.go 到 executionflow/verify/ (follow-up #4 处理)
- ❌ 不合并 observe/orchtypes → decisionplanning/ (follow-up #5 处理)
- ❌ 不收敛 wire 14 → 6 (follow-up #6 处理)
- ❌ 不动 D7 5 个新 P0/P1 Span emit 路径
- ❌ 不动 LP-1 / LP-2 / LP-5 路径
- ❌ 不动 multiagent/ 域

---

## 9. Change Manifest

详见 proposal.md §8 Change Manifest (此处不重复)。

**关键设计决策总结：**
1. Decision 1 — 整包迁移 + 改 package 声明
2. Decision 2 — OrchestratorOption 无冲突 + 2 同名文件重命名 (turn_doc.go + tracing_turn.go)
3. Decision 3 — type 名不改 (双 type: DefaultOrchestrator + SessionOrchestrator)
4. Decision 4 — exit_reason.go 临时留 sessionorchestrator/ (等 #4 promote)
5. Decision 5 — receiver methods 类型不变 (硬ening/ 调用 0 变化)