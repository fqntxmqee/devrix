# Design: D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**Status:** S3_Design (待 S3-Gate review + 博弈论 Round 3 收敛)
**Parent Proposal:** `proposal.md`
**Template:** `docs/methodology/detail-design-framework.md` (六段式)
**Created:** 2026-07-02
**Updated:** 2026-07-02 (博弈论 Round 3 收敛, 含 Design 阶段 D5-D8)
**Domains:** D2 (contextengine), D3 (llmgateway), D5 (observability), D7 (orchestration)

> **本文档反映 2026-07-02 三方博弈论 (Claude + Codex + Cursor) Round 3 收敛**:
>
> **S2 阶段 D1-D4**:
> - **D1**: per-input = **分层混合** (interface 19 函数 + 4 工具 override + 15 default)
> - **D2**: auto-mode classifier = **P2 interface only** (T22'-T23' stub, 0 行实现, metric 触发升 P1)
> - **D3**: GrowthBook = **P0 部分保留 1 flag** (bash 30K→50K, 其他推迟)
> - **D4**: **5 PR (D+E 合并)**
>
> **Design 阶段 D5-D8** (新浮现, 已 Round 3 收敛):
> - **D5**: `IsConcurrencySafe(input)` 参数类型 = **`json.RawMessage`** (跟 CheckPermission 对齐, 类型内聚 > 扩展性)
> - **D6**: partition batch 边界 = **连续 safe 合并** (clawcode 实战验证, 三方一致)
> - **D7**: AutoModeClassifier 返回类型 = **`ClassifierResult`** (devrix Naming Policy, 三方一致)
> - **D8**: GrowthBook 注入方式 = **M1 复用 PERSIST 模式** + **M2/M3 未来独立 struct** (Cursor+Codex 一致指出 M1 是 persist concern, Claude 让步)。**M2/M3 定义**: M2 = per-tool concurrency threshold GB override (本 change 不实现, 后续 D8 v1.2 起); M3 = AutoModeClassifier canary GB override (本 change 不实现, OOS-NEW-2 ensemble 启用时)
>
> 完整辩论: `gaming-debate-{round1,round2-codex,round2-cursor,round3-convergence}-*.md` (S2) + `gaming-debate-design-{round1,round2-codex,round2-cursor,round3-convergence}-*.md` (Design)

---

## ① 架构目标

### 业务目标 (解决痛点 → 对应 AC)

| 痛点 | 描述 | 解决 AC |
|------|------|---------|
| **RC-1**: 静态 ConcurrencySafe 治标 | `tool_surface.go:39-43` 静态 bool 不知 input, `bash` 永远串行, `read_file` 永远并发 (无差别) | AC1, AC2, AC3, AC10 |
| **RC-2**: 无 auto-mode classifier | 静态规则 (`CheckPermission`) + VerifyContract 4 元组已够用, 但 LLM 智能判断的 0 部署 | AC4, AC7 (P2 stub) |
| **RC-3**: 50 文件 review 慢 | T19 fixture 50 read_file 串行执行, 用户体验差 | AC3, AC10 |
| **RC-4**: Bash 并行批次无 cancel 链路 | 同 batch Bash 一个失败, 兄弟继续跑浪费资源 | AC12 |
| **RC-5**: Fallback model 切换时 in-flight 工具不收口 | QueryLoop 切 fallback, 已 emit 的 tool_result 重复处理 | AC13 |
| **RC-6**: ContentReplacementState cache 误判等价 | 不同字段顺序的等价 JSON 视为不同 cache entry | AC14 |

### 技术目标 (量化指标)

| 指标 | 目标 | 来源 |
|------|------|------|
| **P99 partitionToolCalls 决策** | < 1ms (无 I/O, 纯内存 partition) | T18 |
| **P99 per-tool IsConcurrencySafe** | < 5ms (BashASTPolicy 解析是 hot path) | tool_surface.go:158 注释 |
| **50 文件 e2e 总耗时** | < 串行 / 3 (e.g. 串行 5s → 并发 ≤ 1.7s) | AC10 |
| **AutoModeClassifier 5s timeout** | 硬上限 (ctx.WithTimeout), 0 panic | T22' P2 stub (代码 placeholder) |
| **Production-Safety (GB 默认全关)** | 0 行为变化 (单测 `TestGrowthBook_AllFlagsOff_NoBehaviorChange`) | T25' |
| **Coverage (T 层)** | ≥ 80% (跟 devrix 通用规则一致) | testing.md |
| **SLA partitionToolCalls** | 不破坏 turn_adapter.ExecuteRound 既有 P99 | turn_adapter.go:295 注释 |

### 约束条件

- **SemVer**: d2-domain v9.0.0 → v10.0.0 (新增 2 个 ToolSurface method = 扩展契约)
- **灰度**: PR-A → PR-B → PR-C → PR-D+E → PR-F 顺序合入 (W1 D1-D5 + W2 D1-D2 + W3 D1-D2)
- **Production-Safety**: GrowthBook 默认全关, flag 未开启时 0 行为变化 (硬约束)
- **Pure types / 错误码闭合**: ClassifierResult / Decision / ToolSpec 等不可变值对象
- **devrix 文化**: hotfix 模式 (PR 合并 + 用户飞书验收) 而非外部贡献者 PR review 模式

---

## ② 架构原则

### 2.1 设计原则 (10 条以内, 每条对应 AC + 落地)

| # | 原则 | 落地 | AC |
|---|------|------|-----|
| P1 | **Accept interfaces, return structs** | ToolSurface interface + ToolSpec/ToolResult struct | AC1 |
| P2 | **fail-safe 优于 fail-fast** | parse failure → metric + raw input; nil override → declared default; panic recover | AC4, AC7 |
| P3 | **devrix 文化 = hotfix 模式** | PR-D+E 合并 (vs 拆 6 PR), 实现+测试同 PR | D4 收敛 |
| P4 | **Production-Safety 默认关** | GrowthBook registry 默认全关, single test `AllFlagsOff_NoBehaviorChange` | AC11 |
| P5 | **不可变值对象 + With\*** | ToolSpec / ClassifierResult / ThresholdOverride 不可变, 通过 WithOptions 配置 | ②⑥ |
| P6 | **聚合根 ≤ 4 个** | ToolSpec + ThresholdOverride + ClassifierResult + SurfaceLookup | ④ |
| P7 | **异常不过模块边界** | BashASTPolicy 错误 → IPermissionGate Decision 而非 error return; AutoMode 不达 → 默认 allow | AC4, AC7 |
| P8 | **SentinelError 模式** | 新增 `ErrAutoModeClassifierPanic` / `ErrPartitionEmpty` 等 | ⑥ |
| P9 | **跨域 D2↔D7 boundary 守恒** | D7→D2 ToolSurface 调用走 contract package, 不引入 D7→D2 直引 | ④ |
| P10 | **显式优于隐式** | BashASTPolicy 显式判定, 不靠 LLM 推断; GB 显式 flag 名称 | AC11 |

### 2.2 命名规范

| 类型 | 模板 | 示例 |
|------|------|------|
| **DSAFT ID** | `D{X}-S{X}-A{XX}-T{XX}` (活动层) / `D{X}-S{X}-A{XX}-F{XX}-T{XX}` (功能层) | D7-S9-A50-T16 |
| **Type (interface)** | `<Name>Surface` / `<Name>Classifier` (契约) | ToolSurface, AutoModeClassifier |
| **Type (struct)** | `<Name>Result` / `<Name>Override` (值对象) | ClassifierResult, ThresholdOverride |
| **Method** | `Is<Property>` / `To<Projection>` (布尔返 / 投影) | IsConcurrencySafe, ToAutoClassifierInput |
| **Error Code** | `Err<Action><Subject>` (SentinelError 模式) | ErrPartitionEmpty |
| **Span Op** | `d2.<action>` / `d7.<node>.<action>` | d2.partition_tool_calls, d7.execute.classify |
| **Metric** | `<domain>.<subject>.<verb>` | auto_mode.malformed_tool_input, auto_mode.deny |
| **GB Flag** | `devrix_<scope>_<noun>` (output-truncation 类, 跟 concurrency 解耦) | devrix_bash_max_result_size_chars |

### 2.3 代码风格

- 函数 < 50 行, 文件 < 800 行
- 异常不过模块边界 (SentinelError)
- 不可变值对象 (`With*` 返回新副本)
- 工具类 (`partitionToolCalls` / `toCompactBlock`) 不过业务逻辑
- 测试 table-driven, `-race` 必须
- panic 仅用于 invariant violation (例如 P2 stub AutoModeClassifier.ClassifyToolUse)

---

## ③ 业务流程

### 3.1 核心用例时序图 — `partitionToolCalls + IsConcurrencySafe per-input`

```
LLM emits tool calls: [read_file(A), read_file(B), bash(ls), read_file(C), edit_file(X), read_file(D)]
         │
         ▼
ChannelRouter.ExecuteRound (turn_adapter.go:295)
         │
         │  Phase 1: CheckPermission pre-dispatch (DM-20260618-002 F07)
         │  ┌─────────────────────────────────────────────────────────┐
         │  │ for each tc → surface.CheckPermission(spec, input)      │
         │  │   bash(ls) → BashASTPolicy.Parse("ls") → Allow          │
         │  │   edit_file(X) → Allow (非破坏)                            │
         │  └─────────────────────────────────────────────────────────┘
         ▼
partitionToolCalls(calls, surfaces)  ◄── NEW (T18)
   │
   │  for each call:
   │    input := parseInput(call.Input)
   │    safe := surface.IsConcurrencySafe(input)  ◄── NEW per-input
   │      bash(ls)  → isReadOnlyBashCommand("ls") → true
   │      read_file(A) → true (read-only, 跟 v2 一致, 无 size-based 决策)
   │      edit_file(X) → 同 target 路径 false
   │
   │  Partition logic (clawcode toolOrchestration.ts:84-118):
   │    - safe && last.IsConcurrencySafe → 合并同 batch
   │    - unsafe or first call → 新 batch
   │
   ▼
Batches: [
   {IsConcurrencySafe: true,  Calls: [read_file(A), read_file(B)]},
   {IsConcurrencySafe: false, Calls: [bash(ls)]},                       ◄── unsafe 独占 batch
   {IsConcurrencySafe: false, Calls: [edit_file(X)]},                   ◄── unsafe 独占 batch
   {IsConcurrencySafe: true,  Calls: [read_file(C), read_file(D)]},
]
         │
         ▼
ExecuteBatches (batches 串行, batch 内 errgroup 并发)
   │
   │  ┌──────────────────────────────────────────────────────────────────┐
   │  │ Phase 2: parallel dispatch (DM-20260618-001 F06, 升级 per-input) │
   │  │   batch.IsConcurrencySafe=true → errgroup.Go (concurrent)        │
   │  │   batch.IsConcurrencySafe=false → 串行                            │
   │  │                                                                  │
   │  │   Bash 兄弟 abort (T26 NEW):                                      │
   │  │     Bash 启动 → siblingAbortController.Spawn(toolUseID)           │
   │  │     Bash 失败 → controller.AbortSiblings(exceptID, reason)        │
   │  │     兄弟 Bash ctx.Done() → synthetic tool_result cancel            │
   │  └──────────────────────────────────────────────────────────────────┘
         ▼
Results: [read_file(A)✓, read_file(B)✓, bash(ls)✓, edit_file(X)✓, read_file(C)✓, read_file(D)✓]
```

### 3.2 异常补偿 (Fallback 路径表)

| 阶段 | 失败模式 | Fallback 路径 | 触发条件 | 幂等保障 |
|------|---------|--------------|---------|---------|
| **Phase 1 CheckPermission** | BashASTPolicy 解析失败 | Deny + metric `perm.parse_fail` | bash input 非字符串 / 异常 unicode | 单 tool 失败不影响同 batch |
| **partitionToolCalls** | unknown surface | 默认 `s.ConcurrencySafe=false` (v2 static) | surfaceLookup 缺 key | 保留 v2 行为 |
| **IsConcurrencySafe per-input** | parse failure | return false (NEVER panic) | input 非 JSON | fail-safe |
| **ToAutoClassifierInput** | parse failure | return raw input + emit metric | input 非 JSON | fail-safe |
| **errgroup batch** | 1 Bash 失败 | siblingAbortController.AbortSiblings (T26) | Bash 返回 error | 兄弟 ctx cancel |
| **StreamingToolExecutor.Discard** | model fallback | `Discard("model_fallback")` 收口所有 in-flight | QueryLoop 切 fallback model | synthetic tool_result 唯一 |
| **AutoModeClassifier (P2 stub)** | 当前调用 | `panic("P2 interface, not implemented; see gaming-debate-round3-convergence.md")` | 占位 (T22' panic) | N/A (未启用) |
| **GrowthBook override** | flag 未开启 | return defaultVal (declared value) | registry 启动时 secure default | 0 行为变化 |

### 3.3 分支处理决策树

```
LLM emits tool call(s)
   │
   ├─ CheckPermission
   │    ├─ Allow ──→ 继续
   │    ├─ Deny ──→ results[i] = PermissionDeniedError, skip Execute
   │    └─ Ask ──→ IPermissionGate.CheckPermission (plan_mode/OpenWorld 走这)
   │
   ├─ partitionToolCalls
   │    ├─ safe=true && last.safe=true → append last batch
   │    ├─ safe=true && last.safe=false → new batch (safe)
   │    ├─ safe=false → new batch (unsafe, 独占)
   │    └─ unknown surface → s.ConcurrencySafe (v2 static, 治标)
   │
   ├─ ExecuteBatches
   │    ├─ batch.safe=true → errgroup.Go (concurrent)
   │    └─ batch.safe=false → 串行
   │
   └─ T26: Bash sibling abort
        ├─ Bash 成功 → controller 完成, 不 abort
        ├─ Bash 失败 → AbortSiblings(exceptID, "parallel tool call errored")
        └─ 兄弟检测 ctx.Done() → synthetic cancel result
```

---

## ④ 领域模型

### 4.1 聚合根 (≤ 4 个)

| 聚合根 | 职责 | 不可变性 |
|--------|------|----------|
| **ToolSpec** (D2-owned, 现有) | 工具元数据 + 9 v2 + 6 v3 + 2 v4 字段 (IsConcurrencySafe/ToAutoClassifierInput 默认实现) | 不可变值对象, `With*` 返回新副本 |
| **ThresholdOverride** (D2-owned, 现有 GB 预埋) | per-tool persistence threshold override, map copy 防御 | 不可变, 通过 WithOverrides 配置 |
| **ClassifierResult** (D7-owned, P2 stub) | auto-mode classifier decision + reason + source | 不可变值对象, 0 行实现 (panic placeholder) |
| **SurfaceLookup** (D7-owned, ExecuteRound 既有) | toolName → ToolSurface 映射 | 运行时 registry, 非值对象 |

### 4.2 限界上下文 (包边界图)

```
┌─────────────────────────────────────────────────────────────────────┐
│ D7 orchestration/sessionorchestrator                                │
│   └─ bootstrap/turn_adapter.go (ExecuteRound)                       │
│        ├─→ D2 contract.ToolSurface (interface)                      │
│        │     ▲                                                       │
│        │     │ implements                                            │
│        │     │                                                       │
│        │   D2 enforce/tools/surface/* (19 surface)                  │
│        │     ├─ builtin (bash/read/write/edit/grep/glob)            │
│        │     ├─ lsp (5) / verify / tracker / ask_user / etc.        │
│        │     └─ surface/orthogonal_flags_v2.go (T17 default table) │
│        │                                                             │
│        ├─→ D2 persist/growthbook_override.go (T25' M1 复用)         │
│        │     └─ bash 30K→50K MaxResultSizeChars 走                  │
│        │        PersistThresholdOverrideFlag (跟 clawcode 1:1)     │
│        │                                                             │
│        ├─→ D7 orchestration/decisionplanning/                       │
│        │     ├─ to_compact_block.go (T20)                            │
│        │     ├─ auto_classifier.go (T22' P2 stub)                   │
│        │     └─ partition_tool_calls.go (T18 helper)                │
│        │                                                             │
│        └─→ D2 persist/content_replacement_state.go (T28 联动)        │
│              └─ inputsEquivalent (T28, 19 工具默认)                 │
└─────────────────────────────────────────────────────────────────────┘

依赖方向 (单向, 无 cycle):
  D7 sessionorchestrator → D2 contract → D2 surface → (library)
  D7 decisionplanning    → D2 contract.ToolSurface
  D5 growthbook          → (no D2 import, pure observability)
  D2 persist             → D2 contract (intra-domain)
```

### 4.3 领域事件 (Span / Metric 列表)

| Span Op | 类型 | 触发位置 | 属性 |
|---------|------|---------|------|
| `d2.partition_tool_calls` | span | turn_adapter.ExecuteRound | `batch_count`, `safe_count`, `unsafe_count` |
| `d7.execute.classify` (P2 stub) | span | (placeholder, 不触发) | (T22' 升 P1 后激活) |
| `d2.tool.is_concurrency_safe` | span | partitionToolCalls 内嵌 | `tool`, `input_bytes`, `result` |
| `d7.execute.batch` | span | ExecuteBatches | `batch_id`, `safe`, `count` |
| `d7.execute.bash_sibling_abort` | span (event) | Bash 失败时 | `tool_use_id`, `siblings_aborted` |

| Metric | 类型 | 触发 |
|--------|------|------|
| `auto_mode.malformed_tool_input` | counter | ToAutoClassifierInput parse 失败 |
| `auto_mode.classifier_unavailable` | counter | (P2 stub 升 P1 后激活) |
| `auto_mode.denied` | counter | (P2 stub 升 P1 后激活) |
| `partition.empty_input` | counter | partition 收到 0 calls |
| `perm.parse_fail` | counter | BashASTPolicy 解析失败 (已有, 引用) |
| `bash.sibling_abort` | counter | Bash 兄弟 abort 触发 |
| `growthbook.flag_disabled` | counter | GB flag 未开启返回 default |

### 4.4 跨域消费模型 (D2↔D7 boundary contract)

| 边界 | 方向 | 契约 | 守恒约束 |
|------|------|------|----------|
| **D7→D2 ToolSurface** | D7 consume D2 contract | `ToolSurface` interface (9 v2 + 6 v3 + 2 v4 methods) | D7 不引入 D2 直引, 走 contract 包 |
| **D2→D7 executor** | D7 turn_adapter 暴露 ExecuteRound | `ToolRoundRequest` / `ToolRoundResult` | D2 surface 不 import D7 |
| **D5→D2 growthbook** | D5 提供 override getter | `OverrideGetter func() map[string]int` | D2 不 import D5 (D2 持引用, D5 注入) |
| **D2→D2 inputsEquivalent** | persist → enforce/tools/surface | `inputsEquivalent(a, b []byte) bool` | intra-domain, 无 boundary |
| **D7→D3 llmgateway** | D7 SideQuery (P2 stub) 调 LLM | (T22' 升 P1 后激活) | D3 不 import D7 |

---

## ⑤ 核心链路图

### 5.1 端到端路径 — `partitionToolCalls 端到端`

```
┌──────────┐
│ LLM      │ (claude-sonnet-4.6)
│ emits    │ 6 tool calls: 4 read_file + 1 bash(ls) + 1 edit_file(X)
└────┬─────┘
     │ JSON wire
     ▼
┌──────────────────────────────────────────────────────────────────┐
│ D7 sessionorchestrator / turn_adapter.go:295                     │
│   ExecuteRound (Phase 1: CheckPermission)                        │
│   ┌──────────────────────────────────────────────────────────┐   │
│   │ P99 < 5ms (BashASTPolicy hot path)                        │   │
│   │ 6 surface.CheckPermission 调用 → 6 Allow                 │   │
│   └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│   partitionToolCalls (NEW T18)                                   │
│   ┌──────────────────────────────────────────────────────────┐   │
│   │ P99 < 1ms (pure memory, no I/O)                           │   │
│   │ 4 read_file → IsConcurrencySafe(input) = true (read-only)  │   │
│   │ bash(ls) → isReadOnlyBashCommand("ls") = true             │   │
│   │ edit_file(X) → false (写并发会乱序)                        │   │
│   │ → 3 batches:                                              │   │
│   │   [safe: read(A), read(B)]                                │   │
│   │   [unsafe: bash(ls)]                                      │   │
│   │   [unsafe: edit(X)]                                       │   │
│   │   [safe: read(C), read(D)]                                │   │
│   └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│   ExecuteBatches (3 batch 串行, batch 内并发)                      │
│   ┌──────────────────────────────────────────────────────────┐   │
│   │ Batch 1 (safe): errgroup 并发, P99 < 50ms (2 read_file I/O 并发, 远低于 100ms 串行) │
│   │ Batch 2 (unsafe): 串行, P99 < 50ms (ls 命令本身快速)       │   │
│   │ Batch 3 (unsafe): 串行, P99 < 50ms (单文件写)              │   │
│   │ Batch 4 (safe): errgroup, P99 < 50ms                      │   │
│   │ ────────────────────────────────────────                   │   │
│   │ 总 P99 < 200ms (vs 串行 6×50ms = 300ms, 并发节省 ~100ms)  │   │
│   └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
     │
     ▼
┌──────────────────────────────────────────────────────────────────┐
│ D2 enforce/tools/surface/builtin/*  (19 surface)                 │
│   BuiltinSurface.bash → BashRunner (T26 sibling abort wiring)    │
│   BuiltinSurface.read_file → file.ReadFile (MaxResultSizeChars   │
│     截断, 跟并发决策无关 — 见 orthogonal_flags.go:363)            │
│   BuiltinSurface.edit_file → file.EditFile                       │
└──────────────────────────────────────────────────────────────────┘
     │
     ▼
┌──────────────────────────────────────────────────────────────────┐
│ D5 observability (Span + Metric emission)                        │
│   span: d2.partition_tool_calls, d7.execute.batch                │
│   metric: partition.batch_count, bash.sibling_abort (if abort)   │
└──────────────────────────────────────────────────────────────────┘
     │
     ▼
Results → sessionorchestrator.ToolRoundResult → LLM next turn
```

### 5.2 时序标注 (识别瓶颈节点)

| 节点 | P99 | 占比 | 瓶颈? | 缓解 |
|------|-----|------|-------|------|
| Phase 1 CheckPermission | < 5ms | 1% | ✗ | (tool_surface.go:158 已优化) |
| partitionToolCalls | < 1ms | <1% | ✗ | pure memory |
| **Batch 1 (errgroup 2 read)** | < 50ms | **25%** | ✓ | errgroup 内 read_file 受限于 I/O 而非 partition 决策 |
| Batch 2 (bash 串行) | < 50ms | 25% | ✗ | ls 命令天然快 (与并发无关) |
| Batch 3 (edit 串行) | < 50ms | 25% | ✗ | 单文件写天然快 |
| Batch 4 (errgroup 2 read) | < 50ms | **25%** | ✓ | 同 Batch 1 |
| **Phase 2 total** | **< 200ms** | **100%** | | vs 串行 6×50ms = 300ms |

**瓶颈识别**: 50 read_file 场景 (AC10 验收基准) 下, errgroup 内 read_file 受限于 I/O 而非 partition 决策。partition 本身不是瓶颈。AC10 场景目标为 e2e < 串行 / 3 (50 文件串行 5s → 并发 ≤ 1.7s)。本 §5.1 示例仅 4 个 read_file, 不能直接套 AC10, 仅作 partition 行为演示。

### 5.3 单点风险与缓解

| 单点 | 风险 | 缓解 (对应 AC) |
|------|------|---------------|
| **BashASTPolicy 解析失败** | bash(rm -rf /) 漏过 | CheckPermission Phase 1 已拦截 (DM-002 F07); AC4 fail-safe |
| **partitionToolCalls 阻塞** | 19 工具 surface 未注册 → panic | unknown surface fallback to v2 ConcurrencySafe (治标) + metric; AC2 |
| **errgroup panic 蔓延** | 1 tool panic → 整个 batch 失败 | Go recover() + SentinelError 包装; 测试覆盖 |
| **siblingAbortController 锁泄漏** | sync.Mutex 死锁 | T26 集成测试覆盖边界; AC12 |
| **GrowthBook flag 误启用** | devrix ops 误操作开启 flag → 行为变化 | Production-Safety 单测 + 默认全关 + registry 启动 secure default; AC11 |
| **AutoModeClassifier (P2 stub) 误激活** | 有人实例化 stub → panic | T22' panic 信息明确, 编译时即警告 (interface only); 升 P1 触发 metric `verify_contract.deny_rate > 5%` |
| **inputsEquivalent cycle** | a == b && b == c 但 a != c | JSON unmarshal + reflect.DeepEqual 保证传递性; AC14 57 单测 |

---

## ⑥ 接口 / API 设计

### 6.1 风格

- **Pure types**: `ToolSpec` / `ClassifierResult` / `ThresholdOverride` / `Batch` 不可变值对象
- **Builder**: `WithOverrides(getter)` 配置 GB override
- **With\***: `ToolSpec.WithConcurrencySafe(bool)` 等返回新副本
- **SentinelError**: 业务错误用 SentinelError 模式 (`ErrPartitionEmpty` / `ErrAutoModeClassifierPanic`)
- **接口定义在 consumer 包**: `ToolSurface` 在 D2 contract, `AutoModeClassifier` 在 D7 decisionplanning

### 6.2 契约 (错误码三元组 + TraceID)

| Error | Code | Message | Remediation | TraceID |
|-------|------|---------|-------------|---------|
| `ErrPartitionEmpty` | E_D2_PARTITION_001 | "partitionToolCalls: empty input" | 无操作 (正常返回 0 batches) | d2.partition_tool_calls span_id |
| `ErrUnknownSurface` | E_D2_PARTITION_002 | "partition: surface not found for tool {name}" | fallback to v2 ConcurrencySafe | d2.partition_tool_calls span_id |
| `ErrIsConcurrencySafeParse` | E_D2_PARTITION_003 | "IsConcurrencySafe: parse failed for tool {name}" | return false (NEVER panic) | d2.tool.is_concurrency_safe span_id |
| `ErrToAutoClassifierParse` | E_D7_AUTO_001 | "ToAutoClassifierInput: parse failed for tool {name}" | return raw input + emit metric | d7.execute.classify span_id |
| `ErrAutoModeClassifierPanic` | E_D7_AUTO_002 | "P2 interface, not implemented; see gaming-debate-round3-convergence.md" | 升 P1 触发 verify_contract.deny_rate > 5% | d7.execute.classify span_id |
| `ErrBashSiblingAbort` | E_D2_BASH_001 | "Bash sibling aborted by parallel tool call error" | 无操作 (synthetic tool_result) | d7.execute.bash_sibling_abort span_id |

### 6.3 幂等保障表

| 操作 | 幂等机制 | 测试 |
|------|---------|------|
| `partitionToolCalls` | pure function (no side effect) | `TestPartition_PureFunction_SameInputSameOutput` |
| `IsConcurrencySafe` | pure (无 I/O) | `TestIsConcurrencySafe_Idempotent` |
| `toCompactBlock` | pure (除 metric emit) | `TestToCompactBlock_Idempotent_MetricCountStable` |
| `BashSiblingAbort.AbortSiblings` | map dedup, sync.Mutex 保单次 | `TestSiblingAbort_Idempotent_DoubleCallNoOp` |
| `StreamingToolExecutor.Discard` | inflight map 清空后 idempotent | `TestDiscard_Idempotent` |
| `ThresholdOverride.GetPersistenceThreshold` | pure (除 GB client 状态) | `TestThresholdOverride_Idempotent` |

### 6.4 版本演进路径

| 版本 | 范围 | 触发 |
|------|------|------|
| **v1.0** (本 change) | interface 19 函数 + 4 工具 override + 15 default + auto-mode P2 stub + GB 1 flag + sibling abort + discard + inputsEquivalent | PR-A ~ PR-F 顺序合入 |
| **v1.1** (后续 change, P1) | AutoModeClassifier 实施 (升 P1) | verify_contract.deny_rate 7d > 5% OR 真实 incident 1 次 |
| **v1.2** (后续 change, P2) | GB flag 扩容: bash readonly canary + classifier canary | v1.0 GB bash threshold 实际调优有 ops 需要 |
| **v2.0** (后续 change, breaking) | auto-mode classifier ensemble (OOS-NEW-2) + 跨 session reputation (OOS-NEW-3) | 远期, 不在本 change 范围 |

---

## 附录 A: File Manifest

### 新增文件

| 路径 | T | PR | 用途 |
|------|---|-----|------|
| `internal/shared/contracts/tool_surface_v4.go` | T16 | PR-A | ToolSurface interface v4 扩展 |
| `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` | T17 | PR-A | 19 工具 IsConcurrencySafe + ToAutoClassifierInput default table |
| `internal/layers/orchestration/decisionplanning/partition_tool_calls.go` | T18 | PR-B | partitionToolCalls helper |
| `internal/layers/contextengine/prepare/compression/review50_e2e_concurrent_test.go` | T19 | PR-B | 50 文件 e2e 并发版 |
| `internal/layers/orchestration/decisionplanning/to_compact_block.go` | T20 | PR-C | toCompactBlock JSONL 序列化 |
| `internal/layers/orchestration/decisionplanning/auto_classifier.go` | T22' | PR-D+E | AutoModeClassifier interface stub (P2) |
| `internal/layers/orchestration/decisionplanning/auto_classifier_test.go` | T24' | PR-D+E | Classifier P2 stub 单测 |
| `internal/layers/observability/instrument/growthbook/registry.go` | T25' | PR-D+E | GrowthBook registry (默认全关) |
| `internal/layers/contextengine/persist/growthbook_override_bash.go` (M1 复用) | T25' | PR-D+E | M1 = `devrix_persist_threshold_override` (bash 30K→50K MaxResultSizeChars), 复用 `persist.ThresholdOverride` + `GetPersistenceThreshold` 模式 |
| `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` | T26 | PR-F | BashSiblingAbortController |
| `internal/layers/contextengine/enforce/tools/bash/sibling_abort_test.go` | T26 | PR-F | |
| `internal/bootstrap/streaming_executor.go` | T27 | PR-F | StreamingToolExecutor.Discard() |
| `internal/bootstrap/discard_on_fallback.go` | T27 | PR-F | QueryLoop fallback wiring |
| `internal/layers/contextengine/enforce/tools/surface/inputs_equivalent.go` | T28 | PR-F | inputsEquivalent 19 工具默认 |

### 修改文件

| 路径 | T | PR | 修改点 |
|------|---|-----|--------|
| `internal/layers/contextengine/enforce/tools/surface/builtin.go` | T17, T26 | PR-A, PR-F | BuiltinSurface 4 工具 override + BashRunner 集成 sibling abort |
| `internal/bootstrap/turn_adapter.go:295` | T18, T23', T26 | PR-B, PR-D+E, PR-F | ExecuteRound 集成 partitionToolCalls + 占位 classifier + Bash sibling abort 调用点 |
| `internal/layers/orchestration/decisionplanning/to_compact_block.go` | T20 | PR-C | panic recovery + metric emit |
| `internal/layers/contextengine/persist/content_replacement_state.go` | T28 | PR-F | 走 inputsEquivalent 做 cache invalidation |
| `internal/layers/contextengine/prepare/compression/pipeline.go` | T25' | PR-D+E | ContentReplacementState 集成 GB override |

### 不修改 (回归基线)

- `internal/layers/contextengine/persist/growthbook_override.go` (现有 GB 预埋, 仅 T25' 引用模式)
- `internal/shared/contracts/tool_surface.go:39-43` (ConcurrencySafe bool 字段保留, 作 v4 fallback)
- `internal/layers/orchestration/decisionplanning/verify_contract.go` (4 元组 VerifyContract 保留, P2 stub 不破坏)

---

## 附录 B: Rollback Plan

### B.1 多层回滚机制

| 层 | 触发条件 | 机制 | RTO |
|----|---------|------|-----|
| **L1: PR revert** | PR-A/B/C/D+E/F 任意一个引入 critical bug | `git revert <commit>` + squash merge | < 1h |
| **L2: Feature flag disable** | GB bash threshold 误启用 | GB SDK 推空 map → override 失效 → default 生效 | < 5min |
| **L3: interface 兼容** | 工具 surface override panic | partitionToolCalls 捕获 panic → fallback to v2 ConcurrencySafe (治标) | 即时 |
| **L4: stub 隔离** | AutoModeClassifier P2 stub panic | ChannelRouter 占位代码 TODO 注释 + 0 实例化 | 即时 (未启用) |

### B.2 触发条件 (具体)

- **L1 触发**: PR-A 引入 critical bug (e.g. 19 工具 compile error) → git revert + 复盘
- **L2 触发**: GB bash threshold 30K→50K 在 devrix ops 验证中导致 bash 输出截断频繁 → GB SDK 关 flag
- **L3 触发**: 某个工具 surface 实现的 IsConcurrencySafe panic → partitionToolCalls recover (T18 fail-safe wrapper) + metric `partition.tool_panic`
- **L4 触发**: (无触发, P2 stub 永不实例化)

---

## 附录 C: 回归风险评估

### C.1 baseline 对比

| 测试 | baseline (现状) | 本 change 后 | 风险等级 |
|------|---------------|--------------|---------|
| 50 read_file 串行 e2e | PASS (T27 fixture) | PASS (T19 fixture 复用 + partition 升级) | Low |
| 19 工具 surface 注册 | PASS | PASS (T17 default table 新增) | Low |
| turn_adapter.ExecuteRound | PASS (Phase 1+2) | PASS (Phase 1 保留 + Phase 2 升级 per-input) | Medium |
| GrowthBook override | PASS (T04 ContentReplacementState 已 CLOSED) | PASS (T25' bash threshold 1 flag 追加) | Low |
| AutoModeClassifier | N/A (无) | PASS (T22' P2 stub panic 信息合规) | Low |
| Bash sibling abort | N/A (无) | PASS (T26 集成测试) | Medium |
| StreamingToolExecutor.Discard | N/A (无) | PASS (T27 QueryLoop fallback wiring) | Medium |
| inputsEquivalent | PASS (raw string compare) | PASS (T28 JSON unmarshal + reflect.DeepEqual) | Low |

### C.2 高风险改动点

1. **turn_adapter.ExecuteRound 升级 per-input** (T18) — 影响所有 19 工具 dispatch
   - 缓解: Phase 1 CheckPermission 保留, Phase 2 partition 走 fail-safe wrapper
2. **Bash sibling abort** (T26) — 影响 BashRunner 启动逻辑
   - 缓解: 集成测试覆盖 cancel 边界 + 父 turn 不 cancel
3. **StreamingToolExecutor.Discard** (T27) — 影响 QueryLoop fallback 路径
   - 缓解: 依赖 TD-QL-03 已 CLOSED, fallback wiring 走 QueryLoop 已 wired path

### C.3 测试策略

| 测试类型 | 范围 | 工具 |
|---------|------|------|
| 单元测试 | 19 工具 IsConcurrencySafe + ToAutoClassifierInput default (T17) | go test -race ./internal/layers/contextengine/enforce/tools/surface/... |
| 单元测试 | partitionToolCalls pure function (T18) | go test ./internal/layers/orchestration/decisionplanning/ |
| 集成测试 | 50 read_file e2e 并发版 (T19) | go test ./internal/layers/contextengine/prepare/compression/ |
| 单元测试 | AutoModeClassifier P2 stub panic 信息 (T22') | go test ./internal/layers/orchestration/decisionplanning/ |
| 单元测试 | GrowthBook bash threshold flag (T25') | go test ./internal/layers/observability/instrument/growthbook/ |
| 集成测试 | Bash sibling abort 边界 (T26) | go test ./internal/layers/contextengine/enforce/tools/bash/ |
| 单元测试 | StreamingToolExecutor.Discard idempotent (T27) | go test ./internal/bootstrap/ |
| 单元测试 | inputsEquivalent 19 工具 × 3 case = 57 单测 (T28) | go test ./internal/layers/contextengine/enforce/tools/surface/ |
| 端到端 | ChannelRouter 集成 (T23' 占位) | go test ./internal/bootstrap/ |

---

## 附录 D: S3 检查清单自检 + 博弈论 Round 3 收敛要点

### D.1 S3 检查清单 (per architecture-design.md §8)

- [x] 六段式完整性: ①架构目标 / ②架构原则 / ③业务流程 / ④领域模型 / ⑤核心链路图 / ⑥接口/API 设计
- [x] 六段式非空: 每段 5-20 行 + 时序图 + 表格 (中型 Change 标准)
- [x] `dsaft_activities` 已标注: T16-T28 + T22'-T23' + T25' 完整
- [x] design.md 明确每个 A 的 F 编排关系: 见 ④.1 聚合根 + 附录 A File Manifest
- [x] `specs/*/spec.md` 包含所有 Gherkin Scenario: S4 阶段产出 (本设计不含)
- [x] 每个 Requirement 有对应的 T 层注释: T16-T28 + T22'-T23' + T25' 在 tasks.md
- [x] 重大决策已记录: 见附录 D.2 博弈论决策表
- [x] **S3-Gate Review 结论**: 待 review (Approved / Changes Requested)
- [x] Draft PR 已创建: 待 review 通过后创建

### D.2 博弈论决策表 (Round 3 收敛审计)

| 决策点 | 倾向 | 关键证据 | 反方让步理由 |
|--------|------|---------|--------------|
| **D1** per-input 实现 | **分层混合** (4 override + 15 default) | clawcode `Tool.ts:402,556` interface + `BashTool.tsx:434-442` 实例 | Claude+Cursor 让步: 15 工具 default table 不写 boilerplate |
| **D2** auto-mode classifier | **P2 interface only** | devrix 无相关 incident; VerifyContract 4 元组已够用 | Cursor 让步: 无 prod incident 证据, 接口先就位 |
| **D3** GrowthBook | **P0 部分保留 1 flag** (bash 30K→50K) | Cursor 引用 `persist/growthbook_override.go:1-9` devrix 内部 ops 调优先例 | Claude 让步: bash threshold 是真实 ops 需要, 不是推测; Codex 让步: 全删过于激进 |
| **D4** PR 数量 | **5 PR (D+E 合并)** | devrix hotfix 模式 + DM-20260702-008 延期教训 | Codex 让步: 6 PR 拆分违反即时反馈文化 |

### D.3 design 阶段新增差异点 (Round 3 已收敛)

> **D5-D8 是 design 阶段才浮现的细分决策**, 三方已 Round 3 收敛 (见 `gaming-debate-design-round3-convergence.md`):

| # | 差异点 | 最终立场 | 关键反方让步 |
|---|--------|---------|--------------|
| **D5** | `IsConcurrencySafe` 参数类型 | **`json.RawMessage`** (跟 CheckPermission 对齐) | Claude+Cursor 让步: YAGNI 适用 mcp_* 扩展, 类型内聚 > 推测性扩展性 |
| **D6** | partition batch 边界规则 | **连续 safe 合并** (clawcode 实战验证) | 三方一致, 无让步 |
| **D7** | AutoModeClassifier 接口命名 | **`ClassifierResult`** (devrix Naming Policy) | 三方一致, 修 design.md 草稿 YoloResult 疏忽 |
| **D8** | GrowthBook override 注入方式 | **M1 复用 PERSIST 模式** + **M2/M3 未来独立 struct** | Claude 让步: M1 是 persist concern (`MaxResultSizeChars`), 不是 concurrency; Cursor+Codex 指出 `growthbook_override_test.go:33-38` 已预演 `bash: 50*1024`。**M2/M3 定义**: M2 = per-tool concurrency threshold GB override (后续 change, 不在本 change 范围); M3 = AutoModeClassifier canary GB override (OOS-NEW-2 ensemble 启用时, 不在本 change 范围) |

---

## 附录 E: 下一步

1. ~~三方博弈论 Round 1 强论证稿~~ (已完成: `gaming-debate-design-round1-claude.md`)
2. ~~codex + cursor 读 Round 1 写 Round 2 回应~~ (已完成: codex + cursor 双方答辩)
3. ~~基于 Round 2 写 Round 3 收敛, 更新 design.md (本文件)~~ (已完成: `gaming-debate-design-round3-convergence.md` + design.md 修正)
4. **S3-Gate review** (Approved / Changes Requested)
5. **进入 S4 实现** (PR-A 路线优先, 5 PR 合并 D+E)