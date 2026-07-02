# Proposal: D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**Status:** S7_Archived (S5 验收 ACCEPTED, S6 归档完成)
**Created:** 2026-07-02
**Updated:** 2026-07-02 (博弈论 Round 3 收敛 + S6 归档)
**Parent Demand:** `demand.md`

> **本文档反映 2026-07-02 三方博弈论 (Claude + Codex + Cursor) 收敛结果**。完整辩论过程见:
> - `gaming-debate-round1-claude.md` (强论证稿 + 12 反问)
> - `gaming-debate-round2-codex.md` (Codex 答辩, 5552 行)
> - `gaming-debate-round2-cursor.md` (Cursor 答辩, 297 行)
> - `gaming-debate-round3-convergence.md` (最终收敛)
> - `gaming-analysis-synthesis.md` (Round 0 原始三方分析)
>
> **关键调整** (vs 需求原状):
> - **D1**: per-input 函数 = **分层混合** (interface 19 函数 + 4 工具 override + 15 default)
> - **D2**: auto-mode classifier = **P2 interface only** (不实施 SideQuery, metric 触发升 P1)
> - **D3**: GrowthBook = **P0 部分保留 1 flag** (bash 30K→50K, 其他推迟)
> - **D4**: **5 PR (D+E 合并)** (vs 6 PR 原状)

---

## 0. Synthesis Lineage

本文档基于 2026-07-02 对 DM-20260702-008 P1 延期 + 复盘清单 6 项审计 + 借鉴关系 10 项的复盘, 主要输入:

**clawcode 真实源码** (10 处):

- `/Users/fukai/workspace/clawcode/src/Tool.ts:402` (`isConcurrencySafe` interface)
- `/Users/fukai/workspace/clawcode/src/Tool.ts:556` (`toAutoClassifierInput` interface)
- `/Users/fukai/workspace/clawcode/src/Tool.ts:759,767` (TOOL_DEFAULTS — fail-closed)
- `/Users/fukai/workspace/clawcode/src/Tool.ts:402,556,712-714` (`inputsEquivalent` 35 字段之一)
- `/Users/fukai/workspace/clawcode/src/services/tools/toolOrchestration.ts:84-118` (`partitionToolCalls`)
- `/Users/fukai/workspace/clawcode/src/services/tools/toolOrchestration.ts:26-32` (batch consume)
- `/Users/fukai/workspace/clawcode/src/utils/permissions/yoloClassifier.ts:378-410` (`toCompactBlock`)
- `/Users/fukai/workspace/clawcode/src/utils/permissions/yoloClassifier.ts:1485-1493` (sideQuery)
- `/Users/fukai/workspace/clawcode/src/tools/BashTool/BashTool.tsx:434-442` (per-input IsConcurrencySafe + ToAutoClassifierInput 实例)
- `/Users/fukai/workspace/clawcode/src/services/tools/StreamingToolExecutor.ts` (siblingAbortController + discard)

**devrix 现状** (6 处):

- `internal/shared/contracts/tool_surface.go:39-43` (devrix 静态 `ConcurrencySafe bool` 现状)
- `internal/shared/contracts/tool_surface.go:43` (devrix 已有 `ReadOnly` 字段, clawcode isReadOnly 不需借鉴)
- `internal/shared/contracts/tool_surface.go:66` (devrix 已有 `InterruptMode`, 跟 clawcode interruptBehavior 1:1)
- `internal/bootstrap/turn_adapter.go:277` (devrix `ExecuteRound` 现状)
- `internal/layers/contextengine/persist/growthbook_override.go:1-9, 24-28, 57-89` (devrix 已有 GB 预埋模式, 跟本 change D3 横向复用)
- `internal/layers/contextengine/persist/content_replacement_state.go:14-23, 81-118` (devrix ContentReplacementState, inputsEquivalent 不需借鉴)
- `internal/layers/contextengine/enforce/tools/bash/bash_runner.go` (BashTool runner — 集成 sibling abort)

**复盘文档** (4 处):

- `openspec/tech-debt/streaming-tool-executor-v2.md` (TD-STE-01~06, 4 项被本 change 关闭)
- `openspec/tech-debt/queryloop-error-recovery.md` (TD-QL-03 已 CLOSED, TD-QL-07 联动)
- `/Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/53-clawcode-tools-design.md` (35 字段参考)
- `openspec/changes/devrix-token-design-v2/{demand,proposal,design}.md` (借鉴关系 10 项)

---

## 1. 提案动机 (RC-1 + RC-2)

### 1.1 RC-1: `ConcurrencySafe` 静态 bool 是治标

devrix 现状 (`internal/shared/contracts/tool_surface.go:39-43`):

```go
// ConcurrencySafe: multiple invocations of the same tool may run in parallel
// without mutual interference (e.g. read_file on different paths).
// turn_adapter.ExecuteRound uses this to decide parallel vs sequential dispatch.
ConcurrencySafe bool
```

**问题**: 静态 bool, **per-tool**, 不知道具体 input

| 工具 | 现状 (v2 bool) | 应该的 per-input 决策 |
|------|----------------|---------------------|
| `bash` | `false` (永远串行) | `git status` → true, `rm -rf` → false |
| `read_file` | `true` (永远并发) | 大文件 (>1MB) → 串行, 小文件 → true |
| `write_file` | `false` | 永远 false (写并发会乱序) |
| `edit_file` | `false` | 永远 false (同 path 互斥) |
| `grep` | `true` | true (read-only) |
| `glob` | `true` | true (read-only) |
| `lsp_*` | `true` | true (read-only) |
| `web_fetch` | `true` | 永远 false (per-host rate-limit) |
| `verify_*` | `true` | 永远 false (重资源) |
| `free_fork` | `false` | 永远 false (spawn 副作用) |
| `mcp_*` | `true` | 跟具体 mcp server 协议有关, 默认 false (保守) |

**vs clawcode** (`src/Tool.ts:402`):

```typescript
isConcurrencySafe(input: z.infer<Input>): boolean
```

**per-input 函数**, 工具 surface 自己决定。`BashTool` 实际实现 (`src/tools/BashTool/BashTool.tsx:434-437`):

```typescript
isConcurrencySafe(input) {
  return this.isReadOnly?.(input) ?? false;
}
```

**consequence**: 9 个 `git status` 在 devrix 当前全串行 (9×1s = 9s), 在 clawcode 1 batch 并发 (1×1s = 1s). **9× speedup** for typical read-only batches.

### 1.2 RC-2: 无 auto-mode 安全分类器, 缺中间层 (P2 推迟)

devrix 当前安全栈 (3 道):

1. **事前静态规则** — `surface.CheckPermission` (VerifyContract 4 元组 Burden × Class × Discipline × Outcome, DM-20260701-007)
2. **执行** — tool runner
3. **事后验证** — `executionflow/verify/` Verify 节点

**缺中间层** (执行后, Verify 节点前). 静态规则漏掉的攻击直接执行, 后果不可逆.

**vs clawcode** (`src/utils/permissions/yoloClassifier.ts`):

- **事前投影**: `Tool.toAutoClassifierInput(input)` → 紧凑 string (e.g. `ls -la` for Bash, `/tmp/x: new content` for Edit)
- **transcript 序列化**: `toCompactBlock` → `{"Bash":"ls"}` JSONL 喂独立 LLM (SideQuery)
- **LLM 判 allow/deny**: 5s timeout, **fail-closed** (修正 demand §6 原"fail-open", 采纳 Cursor Q6)
- **失败 telemetry**: `tengu_auto_mode_malformed_tool_input` 事件 + `tengu_auto_mode_classifier_unavailable` 事件
- **复用 ToolUseContext**: sideQuery 复用 main loop 的 LLM gateway, 不另起 connection

**对比**:

| 层 | devrix | clawcode |
|----|--------|----------|
| L0 静态规则 | ✅ VerifyContract 4 元组 | ✅ checkPermissions |
| L1 SideQuery 中间层 | ⚠️ **P2 interface only (本 change)** | ✅ yoloClassifier |
| L2 运行时沙箱 | ✅ Bash AST analyzer (W4 AC10) | ✅ bashClassifier |
| L3 事后 Verify | ✅ executionflow/verify | ✅ TaskVerify (post) |

**P2 推迟理由** (Round 3 收敛):
- Cursor (D2 P0 派) 承认 **devrix 无生产安全事故** (Round 2 Q4)
- 缺口**未被实战证明**, 是**预防型架构**而非修复型
- devrix 资源有限, P0 应优先治本 (per-input 函数)
- **升级触发 metric** (Cursor + Claude 综合): 90 天内 `permission.allow+manual_review_tagged.semantic_risk >= 3/week` 即升 P1

---

## 2. 核心机制 (M1-M5, P2 降级标注)

### 2.1 M1 — Per-Input `IsConcurrencySafe` (P0, 三方一致)

**接口** (`internal/shared/contracts/tool_surface_v4.go` 新建):

```go
// IsConcurrencySafe reports whether THIS SPECIFIC INVOCATION (with this
// specific input) may run concurrently with other concurrency-safe tool
// calls in the same batch. The decision may depend on input — e.g. bash
// with a read-only command is concurrency-safe, but bash with `rm -rf`
// is not.
//
// **Default implementation**: return ToolSpec.ConcurrencySafe (v2 static
// bool) for back-compat. Tools that need per-input logic MUST override.
//
// **Fail-safe**: implementations MUST NOT panic; on parse failure, return
// false (treat as not concurrency-safe). Emits telemetry
// `tool.is_concurrency_safe.failed` on parse failure for observability.
//
// Mirrors clawcode Tool.ts:402 + Tool.ts:759 (`(_input?: unknown) => false`
// default — fail-closed).
type ToolSurface interface {
    // ... existing 9 + 6 v3 methods ...
    
    // IsConcurrencySafe(input) is the v4 per-input decision.
    IsConcurrencySafe(input []byte) bool
}
```

**4 工具 override 详细** (Round 3 收敛采纳 Codex 折中 + Cursor 实现层接受):

| Tool | IsConcurrencySafe(input) 判定 | 借鉴 clawcode |
|------|------------------------------|---------------|
| **Bash** | `isReadOnly(input) → true; else false` | `BashTool.tsx:434-437` |
| **read_file** | 解析 input 找 `path` + `limit`, 大文件 (>1MB) → false | clawcode `read_file.ts` |
| **edit_file** | 解析 input 找 `file_path`, 同一 path 在同 batch → false (mutual exclusion) | clawcode `edit_file.ts` |
| **write_file** | 同 edit_file (写并发会乱序, 永远 false) | clawcode `write_file.ts` |

**15 工具 default 路由** (走 `s.ConcurrencySafe` 字段):

```go
// BuiltinSurface 6 工具中, 4 个有 override, 2 个走 default (grep, glob)
func (s *BuiltinSurface) IsConcurrencySafe(input []byte) bool {
    // bash/read/edit/write 有 override, 走 OverrideLookup
    // grep/glob 走 s.ConcurrencySafe
    if override, ok := s.overrideLookup(s.toolName); ok {
        return override(input)
    }
    return s.ConcurrencySafe // v2 static bool
}
```

LSPToolSurface 5 + FreeFork/Tracker/Verify/AskUser/BackgroundTask/ToolSearch 8 = 13 工具走 default, 全部返回 `s.ConcurrencySafe` 字段.

### 2.2 M2 — `partitionToolCalls` Batch 改造 (P0, 三方一致)

**位置**: `internal/bootstrap/turn_adapter.go:277` 改造

**改造后** (per-input 函数 + partition):

```go
// partitionToolCalls mirrors clawcode toolOrchestration.ts:84-118.
// Consecutive concurrency-safe tool calls go into the same batch;
// the next non-safe call starts a new batch. Each batch runs
// concurrently (errgroup); batches run sequentially to preserve
// LLM-issued ordering within non-safe regions.
func (a *contextEngineAdapter) partitionToolCalls(
    calls []ToolCall,
    surfaces map[string]ToolSurface,
) []Batch {
    batches := []Batch{}
    for _, call := range calls {
        s := surfaces[call.Name]
        input := parseInput(call.Input)
        safe := s.IsConcurrencySafe(input)
        if safe && len(batches) > 0 && batches[len(batches)-1].isConcurrencySafe {
            batches[len(batches)-1].calls = append(batches[len(batches)-1].calls, call)
        } else {
            batches = append(batches, Batch{isConcurrencySafe: safe, calls: []ToolCall{call}})
        }
    }
    return batches
}
```

**预期提速**: 9 个 `git status` (read-only) → 1 batch 并发, 1s 而非 9s. 50 文件 review (9 并发 read_file) → ~10 batch × 1s = ~10s 而非 ~50s 串行.

### 2.3 M3 — Per-Input `ToAutoClassifierInput` (P0 interface, P2 实施)

**接口**:

```go
// ToAutoClassifierInput returns a compact, security-relevant
// representation of this tool invocation for the auto-mode security
// classifier. Examples:
//
//   Bash("ls -la")               → `"ls -la"`
//   Edit("/tmp/x", "new content") → `"/tmp/x: new content"`
//   Read("/etc/passwd")          → `""`  (no security relevance, skip)
//
// Return '' to skip this tool in the classifier transcript
// (e.g. tools with no security relevance like TodoWrite). Mirrors
// clawcode Tool.ts:556 + Tool.ts:767 (`(_input?: unknown) => ''`
// default).
//
// Fail-safe: on parse failure or panic, return the raw input string
// and emit `auto_mode.malformed_tool_input` metric. NEVER panic.
type ToolSurface interface {
    ToAutoClassifierInput(input []byte) string
}
```

**19 工具默认实现**:

```go
func (s *BuiltinSurface) ToAutoClassifierInput(input []byte) string {
    var p struct {
        Command string `json:"command"`
        Path    string `json:"path"`
    }
    if err := json.Unmarshal(input, &p); err != nil {
        metric.AutoModeMalformedToolInput(s.toolName).Inc()
        return string(input) // fail-open: 落 raw input
    }
    switch s.toolName {
    case "bash":
        return p.Command
    case "read_file", "grep", "glob":
        return "" // 无安全相关性, 跳过
    case "edit_file", "write_file":
        return fmt.Sprintf("%s: %s", p.Path, truncate(p.Content, 200))
    }
    return ""
}
```

### 2.4 M4 — `toCompactBlock` JSONL Transcript (P2 interface only, 不实施)

**位置**: `internal/layers/orchestration/decisionplanning/to_compact_block.go` (本 change 内**只声明 interface, 不实施**)

**Round 3 收敛决定**: 
- 加 `ToCompactBlock` 方法签名 (P0 阶段, 跟 M3 配套)
- **不**实施 `toCompactBlock.go` 实际序列化逻辑
- **不**集成到 `ExecuteRound`
- P1 升级触发时 (跟 M5 一起) 才实施

**接口** (保留供 P1 实施):

```go
// toCompactBlock mirrors clawcode yoloClassifier.ts:378-410.
// Serializes one transcript block as a JSON dict line: `{"Bash":"ls"}`
// for tool calls, `{"user":"text"}` for user text. The tool value is
// the per-tool ToAutoClassifierInput projection. JSON escaping means
// hostile content can't break out of its string context to forge a
// `{"user":...}` line — newlines become `\n` inside the value.
//
// Returns '' for tool_use blocks whose tool encodes to '' (e.g. read_file
// has no security relevance).
//
// P2 status: interface declared, implementation deferred to P1 upgrade
// (see Round 3 convergence gaming-debate-round3-convergence.md).
func toCompactBlock(
    block TranscriptBlock,
    role string,
    surfaceLookup map[string]ToolSurface,
) string {
    // P2: not implemented in this change
    panic("toCompactBlock: P2 interface, not implemented; see gaming-debate-round3-convergence.md")
}
```

### 2.5 M5 — Auto-Mode Classifier (SideQuery LLM) (P2 interface only, 不实施)

**Round 3 收敛决定**:
- 加 `IntentClassifier.ClassifyToolUse(transcript, sideQuery) YoloResult` 方法签名
- **不**实施 `auto_classifier.go` 实际分类器
- **不**集成到 ChannelRouter
- P1 升级触发时 (metric 命中) 才实施

**接口** (保留供 P1 实施):

```go
// AutoModeClassifier runs the LLM-driven security classifier on a
// compact transcript. Returns Allow | Deny + reason.
//
// 5s timeout hard cap (mirrors clawcode yoloClassifier.ts:5s policy).
// **Fail-closed on LLM unavailable or timeout** (adopted from Cursor
// Q6; corrects demand §6 "fail-open" which is unsafe in security context).
// Returns Deny + emits `auto_mode.classifier_timeout_deny` metric.
// Fail-open is only available via explicit ops GrowthBook flag
// `devrix_classifier_fail_open=true`.
//
// The classifier NEVER replaces VerifyContract 4 元组 (第一道安全);
// it is a complementary middle layer.
//
// P2 status: interface declared, implementation deferred to P1 upgrade
// (trigger metrics in §10).
type AutoModeClassifier interface {
    ClassifyToolUse(ctx context.Context, transcript []TranscriptBlock) (YoloResult, error)
}

type YoloResult struct {
    Decision YoloDecision  // Allow | Deny
    Reason   string        // LLM 解释
    Source   string        // "anthropic" | "external" | "rule-fallback"
}

type YoloDecision int

const (
    YoloAllow YoloDecision = iota
    YoloDeny
)
```

---

## 3. 架构图 (M1-M5 关系, P2 降级标注)

```
LLM turn
   │
   ▼
ExecuteRound (turn_adapter.go:277)
   │
   ├─ partitionToolCalls (per-input IsConcurrencySafe)        P0 (M1+M2)
   │     │
   │     └─→ Batch[0] (9 read_file)         Batch[1] (1 write_file)    Batch[2] (1 bash)
   │            │                                   │                           │
   │            │ (auto-mode classifier: P2)        │ (P2)                      │ (P2)
   │            │ ClassifyToolUse                  │                           │
   │            │ (interface only, 不实施)          ▼                           ▼
   │            ▼                              skip (write to plan)         run (or deny)
   │       errgroup (concurrent)                  P0 阶段无中间层防御
   │
   └─ per-batch: result merge
```

**3 道安全栈** (本 change 强化 L0 + 加 L1 interface 推迟 P2):

| 层 | 机制 | 落地状态 |
|----|------|----------|
| L0 事前静态 | VerifyContract 4 元组 (DM-20260701-007) | ✅ P0 实施 (本 change 强化) |
| **L1 中间 SideQuery** | **AutoModeClassifier + toCompactBlock** | ⚠️ **P2 interface only** (Round 3 收敛) |
| L2 运行时沙箱 | Bash AST analyzer (W4 AC10) | ✅ 已有 |
| L3 事后 Verify | executionflow/verify | ✅ 已有 |

**L1 升级触发** (Cursor + Claude 综合):
- `permission.allow+manual_review_tagged.semantic_risk >= 3/week` (90 天窗口) → 升 P1
- `verify_contract.fail_after_destructive_exec > 0` (任一即触发, 即时) → 升 P1
- `subquery.p99_latency < 3s AND 可用率 > 99%` (实施前提) → 升 P1

---

## 4. 博弈论共识 (H13-H17)

| ID | 设计承诺 | 落地 T 点 | 博弈轮次 |
|----|----------|----------|----------|
| **H13** | **per-input 并发决策 (4 override + 15 default), 不过度保守** | **T16-T19 (IsConcurrencySafe + partitionToolCalls)** | R3 三方一致 |
| **H14** | **3 道安全栈 (L0/L2/L3) 互补, L1 中间层 P2 interface only** | **T20-T21 (ToAutoClassifierInput interface) + T22-T24 推迟** | R3 Cursor/Codex/Claude 综合 |
| **H15** | **Fail-safe 默认 (抛错 → 不并发 / 落 raw input)** | **T16/T21 (interface 默认实现)** | R0 三方一致 |
| **H16** | **Transcript 投影 + JSONL 序列化, 不暴露整个 transcript** | **T20-T21 (interface 保留, 实施推迟 P1)** | R3 综合 |
| **H17** | **Telemetry 完整 (malformed_input + classifier_unavailable + timeout_deny)** | **P2 推迟, P1 实施时一并完成** | R3 综合 |
| **H18** | **GrowthBook P0 部分保留 1 flag (bash 30K→50K, 跟 persist/GB 横向复用)** | **T25 (PR-F, 1 flag 实施)** | R3 采纳 Cursor 论据 |
| **H19** | **Bash sibling abort (TD-STE-02) + Discard on fallback (TD-STE-03) 收口** | **T26 + T27 (PR-F)** | R0 三方一致 |

---

## 5. T 点划分 (12 T, Round 3 收敛)

| T | DSAFT | 优先级 | 内容 | 关闭/引用 |
|---|-------|--------|------|------|
| T16 | D7-S9-A50-T16 | P0 | ToolSurface interface v4 加 `IsConcurrencySafe(input) bool` | TD-STE-06 partial |
| T17 | D7-S9-A50-T17 | P0 | 19 工具 surface 默认实现 (4 工具 override + 15 default router) | **TD-STE-06 closed-by** |
| T18 | D7-S9-A50-T18 | P0 | `turn_adapter.ExecuteRound` 改造为 `partitionToolCalls` batch | **TD-STE-01 closed-by** |
| T19 | D7-S9-A50-T19 | P0 | 50 文件 e2e 并发版 + 9 并发 read_file batch test | — |
| T20 | D7-S10-A50-T20 | P0 | `ToAutoClassifierInput` interface 加到 ToolSurface (P2 实施推迟) | — |
| T21 | D7-S10-A50-T21 | P0 | 19 工具 `ToAutoClassifierInput` 默认实现 (Bash=command, Edit="path: content", Read/grep/glob="" skip) | — |
| ~~T22~~ | ~~D7-S10-A50-T22~~ | ~~P0~~ | ~~AutoModeClassifier 实现 (5s timeout SideQuery + fail-open)~~ | **R3 推迟 P2** |
| ~~T23~~ | ~~D7-S10-A50-T23~~ | ~~P0~~ | ~~ChannelRouter 集成 (ExecuteRound 每个 batch 前调 ClassifyToolUse)~~ | **R3 推迟 P2** |
| T22' | D7-S10-A50-T22' | P0 | `ClassifyToolUse(transcript, sideQuery) YoloResult` interface (P2 占位, 不实施) | **R3 替换原 T22** |
| T23' | D7-S10-A50-T23' | P0 | `toCompactBlock` interface 声明 (P2 占位, 不实施) | **R3 替换原 T23** |
| T24 | D7-S10-A50-T24 | P0 | Interface 测试 + AC5 telemetry 接口 (P2 stub) + 端到端 e2e | **R3 调整范围** |
| T25 | D5-S25-A04-T01 (new) | P0 | **GrowthBook runtime override (1 flag)** — 仅 `devrix_persist_threshold_override` (bash 30K→50K canary), 跟 persist/T05 横向复用, 19 工具其他 flag 推迟 | DM-20260702-008 借鉴 #8 (R3 部分采纳) |
| T26 | D7-S9-A50-T25 (new) | P1 | **Bash sibling abort** — BashTool 集成 `siblingAbortController`, 并行 Bash 中一个失败 abort 兄弟, 返 synthetic `Cancelled: parallel tool call errored` | **TD-STE-02 closed-by** |
| T27 | D7-S9-A50-T26 (new) | P1 | **Discard on fallback** — `StreamingToolExecutor.Discard()` + QueryLoop fallback 路径 wiring, 在途/queued 工具注入 `streaming_fallback` synthetic result | **TD-STE-03 closed-by** (TD-QL-03 CLOSED) |
| T28 | D2-S15-A02-T29 (new) | P3 | **inputsEquivalent(a, b)** — 19 工具 surface 加 `inputsEquivalent(a, b []byte) bool` 默认实现, 配合 ContentReplacementState (T04) 实现 cache invalidation 收口 | clawcode Tool.ts:712-714 (R3 降 P3) |

**总 T 数**: 13 T → 12 T (砍原 T22-T23 实施, 替换为 T22'-T23' P2 占位)

---

## 6. 兼容性 (0 业务代码 out-of-scope diff)

- **ToolSpec v3 struct 0 字段变更** — 0 break (15 字段 → 15 字段)
- **ToolSurface interface additive** — v4 加 3 方法 (IsConcurrencySafe + ToAutoClassifierInput + ClassifyToolUse/toCompactBlock stub), 已有 surface 通过 `v3 → v4` 升级, 19 surface 全部声明, 4 override + 15 default
- **ExecuteRound 行为升级** — 旧 9 read_file 串行 (假) → 新 9 read_file 1 batch 并发 (真), 实际提速
- **无 surface 改语义** — 19 surface 默认 `IsConcurrencySafe` 行为跟 v2 `ConcurrencySafe bool` 一致 (per-input 函数 fallback 到 bool, AC1)
- **Auto-mode classifier 默认不实施** (P2 推迟) — ChannelRouter 不集成, 跟现状一致
- **GrowthBook 仅 1 flag** (bash 30K→50K) — T25 范围缩小, 19 工具其他 flag 推迟
- **Bash sibling abort 边界** — T26 只 abort 同 batch 并行 Bash 兄弟, 不 abort 父 QueryLoop turn, 不影响非 Bash 工具, 单测覆盖边界
- **Discard 只在 fallback 触发** — T27 只在 QueryLoop 切换 fallback model 前调 Discard(), 正常路径不触发, 单测覆盖"无 fallback 时无 discard 行为"
- **inputsEquivalent 默认按字段比较** — T28 (P3) 19 工具 surface 默认按 JSON unmarshal 后逐字段比较, 跟 clawcode `inputsEquivalent` 默认行为一致, 不引入新机制

---

## 7. 测试策略 (T19 + T24 端到端)

### 7.1 单元测试 (T16-T21, T22'-T23')

- **per-input decision**: 19 工具 × 2 方法 = 38 单测 (passes-fail matrix)
- **partitionToolCalls**: 6 case (all_safe, all_unsafe, mixed, empty, single, large_N)
- **ToAutoClassifierInput**: 19 工具 × 3 case (allow/deny/empty) = 57 单测
- **ClassifyToolUse interface (P2 stub)**: 3 case (placeholder, panic_with_doc, no_op)
- **toCompactBlock interface (P2 stub)**: 2 case (placeholder, no_op)

### 7.2 端到端 e2e (T19, AC10)

**复用 `internal/layers/contextengine/prepare/compression/review50_e2e_test.go`** 加并发版本:

- 50 文件 review, **9 并发 read_file batch** (per partitionToolCalls)
- 期望: 50/50 完成, 总时间 < 串行 / 3
- 老 e2e 保留做回归基线

### 7.3 集成测试 (T19 + T24)

- `turn_adapter_partition_test.go`: 100 个并发 read_file, 全部允许 + 实际并发
- `interface_stub_test.go`: P2 stub interface panic 行为 + metric 发射

---

## 8. 范围外 (OOS, 走 P2/P3 后续 change)

> 本 change 收纳了原 OOS-1 (GrowthBook 走 T25 部分) + TD-STE-01/02/03/06 (4 项 tech-debt 关闭) + inputsEquivalent (走 T28 P3)

- OOS-NEW-1: Transcript 完整 LLM 上下文 (10+ 工具全 transcript) — P2
- OOS-NEW-2: 多 LLM ensemble (ensemble classifier) — P3
- OOS-NEW-3: 跨 session reputation → classifier input — P2
- OOS-NEW-4: Classifier-driven microcompact (T13 PerMessageBudget 联动) — P2
- OOS-NEW-5: LLM SideQuery 模型选择 (Haiku vs Sonnet) — P2
- OOS-NEW-6: YoloClassifier telemetry 跟 Learn FeedbackMemory 联动 — P2
- OOS-NEW-7: 工具 progress 流 (TD-STE-04) — P2
- OOS-NEW-8: synthetic error 统一 (TD-STE-05) — P2
- OOS-NEW-9: Bash 22 zsh rules 改造 (DM-20260701-007 OOS-7 弱相关) — 域自治
- OOS-NEW-10: Filter v2 workspace 维 (DM-20260701-007 OOS-10) — 走 P1 独立 change

**OOS-NEW-11 (新增, 来自 Round 3)**: AutoModeClassifier P1 实施升级 — 触发 metric 命中后, 走新 change 实施 SideQuery + toCompactBlock 实际逻辑

**OOS-NEW-12 (新增, 来自 Round 3)**: GrowthBook 19 工具其他 flag 接线 — 等 RC-1 治本验证 + D2 升 P1 后, 走新 change

---

## 9. 验收 + 归档 (S5 + S6)

- **S5 验收**: 12 T 全 IMPLEMENTED + AC1-AC3 + AC6-AC11 + **AC15-AC21 (并发不变量)** 全 PASS + 50 文件 e2e 并发版 < 串行 / 3 + `partition_invariants_test` PASS + verify-archive.sh 12 PASS (P2 stub AC4/AC5/AC14 走契约守护测试)
- **S6 归档**: `openspec/archive/2026-07-02-devrix-d2-tool-input-aware-concurrency-and-classifier/` + 域文档同步 (D2 t-registry +12 T, D7 t-registry +9 T, root v5.15.0)

**AC 调整** (Round 3 收敛):
- AC1-AC3 P0 保留 (per-input 函数 + partitionToolCalls)
- AC4 (classifier 实施) **降 P2** (Cursor 承认无生产事故, 保留契约守护测试)
- AC5 (telemetry) **降 P2** (跟 classifier 一起)
- AC6 (fail-safe) P0 保留
- AC7 (Bash isReadOnly) P0 保留
- AC8 (no silent default) P0 保留
- AC9 (13 T 全实施) → **12 T** (T22-T23 砍, T22'-T23' 替换)
- AC10 (e2e) P0 保留
- **AC11 (GrowthBook)** → **P0 部分保留 1 flag** (bash 30K→50K)
- AC12 (Bash sibling abort) P1 保留
- AC13 (Discard on fallback) P1 保留
- **AC14 (inputsEquivalent)** → **P2** (ContentReplacementState 已覆盖)

**S3 设计阶段 AC 复核增补** (Claude+Codex 两方共识, cursor 后端宕机待补审):
- **AC15** partition 结果完整性 (N:N + 保序 + tool_use_id 1:1) — P0
- **AC16** 交错 safe/unsafe 保序拆分 — P0
- **AC17** read-only batch 部分失败不 abort 兄弟 — P0
- **AC18** read_file IsConcurrencySafe 忽略 size 恒 true (8K 回归锁) — P0
- **AC19** panic 隔离 (单 tool goroutine panic 不污染 batch) — P0
- **AC20** 并发上限 enforcement (errgroup.SetLimit) — P1
- **AC21** ctx 取消 goroutine 清理无泄漏 (goleak) — P1
- 均折进 T18 (`partition_invariants_test.go`) + T17 (AC18), 不新增 T 编号; B2/B3 → OOS-NEW-11/12

**最终 AC**: 14 → **21** (7 新增并发不变量: 5 P0 + 2 P1), 13 T → 12 T, 6 PR → 5 PR

---

## 10. 时间表 (5 PR, Round 3 收敛)

| 周 | 活动 | 产出 | 估时 |
|----|------|------|------|
| W1 D1-D2 | **PR-A**: ToolSurface v4 + 19 工具 `IsConcurrencySafe` 默认 (4 override + 15 default) | T16-T17 | 关闭 TD-STE-06 | 2 天 |
| W1 D3-D5 | **PR-B**: partitionToolCalls 改造 + 50 文件并发 e2e | T18-T19 | 关闭 TD-STE-01 | 3 天 |
| W2 D1-D2 | **PR-C**: `ToAutoClassifierInput` interface + 19 工具默认 + `ClassifyToolUse`/`toCompactBlock` P2 stub | T20-T21 + T22'-T23' | — | 2 天 |
| W2 D3-D5 | **PR-D+E (合并)**: P2 stub interface 测试 + AC8 no-silent-default + 端到端 e2e (无 SideQuery 实施) | T24 | — | 3 天 |
| W3 D1-D2 | **PR-F**: 1 个 GB flag (bash 30K→50K) + Bash sibling abort (T26) + Discard on fallback (T27) + inputsEquivalent (T28 P3) | T25-T28 | 关闭 TD-STE-02 + TD-STE-03 | 2 天 |
| W3 D3 | S3-Gate + S4-Gate | 12 T 全 IMPLEMENTED | 1 天 |
| W3 D4-D5 | S5 验收 + S6 归档 + PR squash auto-merge | ACCEPTED + 4 tech-debt closed | 2 天 |

**总估时**: 1W+3D (跟原状 1W+2D 相近, 略增 1 天因 5 PR 更稳健, D2 实施推迟节省 ~1 周抵消)

---

## 11. 博弈论决策记录 (审计追溯)

完整辩论过程见 `gaming-debate-round{1,2,3}*.md` + `gaming-analysis-*.md`, 关键决策:

| 决策点 | Round 0 三方立场 | Round 1 Claude 强论证 | Round 2 让步后 | Round 3 最终 | 关键依据 |
|--------|----------------|---------------------|--------------|-------------|---------|
| D1 per-input | Claude/Cursor=全函数, Codex=分层混合 | 倾向 Codex | Cursor 接受 4 override | **分层混合** | 三方实质一致 |
| D2 classifier | Claude/Cursor=P0, Codex=P2 | 倾向 Codex | Cursor 承认无事故 | **P2 interface + metric 触发** | Cursor 暴露 3 类证据 (RH-D2 incidents) + 承认无事故 |
| D3 GrowthBook | Claude=降P2, Cursor=P0 3flag, Codex=全删 | 倾向 Codex (降P2) | Cursor 引用 GB 预埋文化 + 具体 ops | **P0 部分保留 1 flag** | Cursor 引用 `persist/growthbook_override.go:24-28` + `growthbook_override_test.go:38` 50*1024 预演 |
| D4 PR 数量 | Claude/Cursor=5, Codex=6 | 倾向 5 (Cl+Cu 一致) | Codex 接受 5 (耦合性论据) | **5 PR (D+E 合并)** | 三方一致 |

**收敛机制**: 独立分析 (R0) → 强论证+12 反问 (R1) → 让步矩阵答辩 (R2) → 综合者重评+裁决 (R3)

**关键证据**:
- `internal/layers/contextengine/persist/growthbook_override.go:1-9, 24-28, 57-89` (devrix 已有 GB 模式)
- `internal/shared/contracts/tool_surface.go:43, 66` (devrix 已有 ReadOnly + InterruptMode)
- `internal/layers/contextengine/persist/content_replacement_state.go:14-23, 81-118` (devrix ContentReplacementState)
- `growthbook_override_test.go:38` (bash 30K→50K 预演测试用例)
- RH-D2-01/05/07 (CheckPermission 漏洞, 不是 auto-mode 直接威胁)
