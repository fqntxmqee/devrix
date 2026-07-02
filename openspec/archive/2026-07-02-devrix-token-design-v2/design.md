# Design: Token Design 2.0

**Change ID:** `devrix-token-design-v2`
**Demand ID:** DM-20260702-008
**Status:** S2_Clarified (待 S3-Gate review)

---

## 0. 一句话总结

借鉴 clawcode 的 3 个核心设计 (持久化 + 引用 + LLM 自治 offset/limit) + per-message aggregate + isConcurrencySafe + toAutoClassifierInput, 重做 devrix 8K token 处理, 保留 devrix 6 大创新 (EmissionClass / task_kind 推 / VerifyContract / MUPS / Learn / LTL-Lite advisory)。

---

## 1. 架构对比

### 1.1 现状 (DM-20260701-007)

```
D2 compression pipeline
  ↓
truncate (m.Role == tool && CountText > maxPerResult)
  ↓
TruncateToTokens + "\n...[truncated]"
  ↓
物理消失
  ↓
D7 ChannelRouter → ToolChannel (Probe/Action/Fact/Experiment)
  ↓
ProbeToolChannel.Accept
  ↓
state.IterationsUsed >= MaxN(15) → ErrProbeToolChannelBoundExceeded
  ↓
D7 Verify
  ↓
探索式 finalText → task_incomplete
  ↓
D1 红卡 → 用户重发
```

### 1.2 目标 (DM-20260702-008)

```
D2 compression pipeline
  ↓
persist (per-tool MaxResultSizeChars 阈值)
  ↓
大 result → 写磁盘 + 2KB preview + XML 引用
  ↓
小 result → 原样返回
  ↓
per-message aggregate (200K)
  ↓
N parallel tool_result 累加 > 200K → 自动 persist 最大块
  ↓
D7 ChannelRouter
  ↓
(1) auto-mode classifier 过一道 (返回 '' 跳过, P1)
  ↓
(2) IsConcurrencySafe 分桶 (P1)
  ↓
(3) ProbeToolChannel.Accept (advisory, 不 reject)
  ↓
InjectPromptPressure 软警告 (3 阶段 soft → hard → forced)
  ↓
SynthesizeNow (连续 N 次同 query 触发)
  ↓
D7 Verify (保留)
  ↓
VerifyContract 4 元组 (Burden × Class × Discipline × Outcome)
  ↓
deliverable mandatory → 通过
探索式 finalText → 失败
```

---

## 2. 关键设计

### 2.1 持久化层 (核心, 治本)

#### 2.1.1 API

```go
// PersistToFile persists large content to disk and returns preview + path.
// When content size > maxChars, the function writes to:
//   <projectDir>/<sessionId>/tool-results/<toolUseId>.{txt|json}
// and returns a preview (up to PREVIEW_SIZE_BYTES=2000) wrapped in
// <persisted-output>...</persisted-output> XML tag.
//
// Returns:
//   - (preview, filePath, originalSize, nil) on success
//   - (truncated, "", originalSize, error) on persist failure (fall back to truncate)
//
// DSAFT: D2-S15-A02-T01.
func PersistToFile(content string, toolUseId string, maxChars int) (preview, filePath string, originalSize int, err error)
```

#### 2.1.2 输出格式 (跟 clawcode 对齐)

```xml
<persisted-output>
Output too large (48.3KB). Full output saved to: /Users/fukai/.devrix/projects/devrix/sess-abc/tool-results/toolu_xxx.txt

Preview (first 2.0KB):
...前 2KB 实际内容 (走 generatePreview 切到 newline 边界)...
...
</persisted-output>
```

#### 2.1.3 ContentReplacementState (决策冻结)

```go
// ContentReplacementState tracks replacement decisions per toolUseId
// to preserve cache stability and conversation replay determinism.
//
// State must be stable to preserve prompt cache:
//   - seenIds: results that have passed through the budget check (replaced
//     or not). Once seen, a result's fate is frozen for the conversation.
//   - replacements: subset of seenIds that were persisted to disk and
//     replaced with previews, mapped to the exact preview string shown
//     to the model. Re-application is a Map lookup — no file I/O,
//     guaranteed byte-identical, cannot fail.
type ContentReplacementState struct {
    SeenIds      map[string]bool
    Replacements map[string]string
}

func (s *ContentReplacementState) Seen(id string) bool
func (s *ContentReplacementState) Replace(id, repl string)
func (s *ContentReplacementState) Get(id string) (string, bool)
func (s *ContentReplacementState) Clone() *ContentReplacementState
```

#### 2.1.4 growthbook override

```go
// devrix_persist_threshold_override flag:
//   {tool_name: threshold_chars}
// e.g. {"Bash": 50000, "Read": 0} (0 = Infinity, hard opt-out)
//
// Defensive: GrowthBook cache returns cached !== undefined ? cached : default,
// so a flag served as null leaks through. Guard with optional chaining and
// typeof check so any non-object flag value (null, string, number) falls
// through to the hardcoded default.
func getPersistenceThreshold(toolName string, declaredMaxResultSizeChars int) int
```

#### 2.1.5 image block 跳过

```go
// hasImageBlock returns true if content contains image blocks.
// Image blocks MUST be sent as-is to Claude (not persisted, not truncated).
func hasImageBlock(content any) bool
```

### 2.2 Bounded(15) 改 advisory (治本失效 → 治本生效)

#### 2.2.1 ProbeToolChannel.Accept 改造

```go
// 旧 (DM-20260701-007):
func (p *ProbeToolChannel) Accept(ctx context.Context, call *ToolCall, state *termination.State) (bool, error) {
    maxN := call.Spec.IterationBound.MaxN
    if maxN <= 0 { maxN = 15 }
    state.BoundMax = maxN
    ok, reason := p.inv.Check(state)
    if !ok {
        return false, fmt.Errorf("%w: %s (call=%s, task_kind=%s)",
            ErrProbeToolChannelBoundExceeded, reason, call.ToolName, call.TaskKind)
    }
    return true, nil
}

// 新 (DM-20260702-008):
func (p *ProbeToolChannel) Accept(ctx context.Context, call *ToolCall, state *termination.State) (bool, error) {
    // 不再 hard reject
    // 触发 InjectPromptPressure 软警告 (3 阶段 soft → hard → forced)
    // LLM 自主决定: 用 offset/limit 自治, 或继续 explore
    return true, nil
}
```

#### 2.2.2 Read 工具加 offset/limit

```go
// 旧:
type ReadInput struct {
    Path string `json:"path"`
}

// 新:
type ReadInput struct {
    Path   string `json:"path"`
    Offset int    `json:"offset,omitempty"` // 0-based, default 0
    Limit  int    `json:"limit,omitempty"`  // bytes/chars, default 8192
}
```

#### 2.2.3 ProbeToolChannel 默认 OpenEnded + advisory thresholds

```go
// 旧 (DM-20260701-007):
// read_file/grep/glob → Bounded(15) hard reject

// 新 (DM-20260702-008):
// read_file/grep/glob → OpenEnded (无 iteration bound)
// advisory thresholds: review@20/30, edit@15/20, test@18/25
// 实际 LLM 看到 soft warning 后用 offset/limit 自治
```

#### 2.2.4 task_kind 推改 advisory

```go
// 旧 (per_task_kind.go:10-14):
// review   → Bounded(15)
// edit     → Bounded(10)
// test     → Bounded(12)
// refactor → Bounded(8)
// observe  → OpenEnded (no injection)

// 新 (DM-20260702-008):
// review   → advisory@20/30
// edit     → advisory@15/20
// test     → advisory@18/25
// refactor → advisory@12/15
// observe  → OpenEnded (no injection)
```

### 2.3 per-message aggregate (200K)

```go
// enforcePerMessageBudget evaluates the aggregate size of tool_result
// blocks within a single user message. When blocks together exceed
// perMessageBudget, the largest blocks are persisted to disk and replaced
// with previews until under budget.
//
// Messages are evaluated independently — a 150K result in one turn and
// a 150K result in the next are both untouched.
func enforcePerMessageBudget(messages []Message, perMessageBudget int) []Message
```

### 2.4 19 工具 per-tool 阈值 (跟 clawcode 对齐)

| 工具 | devrix 现状 | devrix v2 (DM-20260702-008) | clawcode 对照 |
|------|------------|---------------------------|--------------|
| read_file | 8K | 8K (保留, devrix 没 offset/limit 时) | Infinity |
| grep | 8K | 20K | 20K |
| glob | 8K | 20K | 100K |
| bash | (无) | 30K | 30K |
| edit | (无) | 100K | 100K |
| write | (无) | 100K | 100K |
| notebook_edit | (无) | 100K | 100K |
| webfetch | 4K | 100K | 100K |
| websearch | 2K | 100K | 100K |
| lsp | (无) | 100K | 100K |
| agent | (无) | 100K | 100K |
| task_* | (无) | 100K | 100K |
| enter_plan_mode | (无) | 100K | 100K |
| mcp_auth | (无) | 10K | 10K |

### 2.5 IsConcurrencySafe (P1, 走 DM-20260702-009)

```go
// IsConcurrencySafe returns true when the tool is safe to run concurrently
// with other concurrency-safe tools in the same turn.
//
// Default: false (fail-closed). Override per-tool or per-input.
//
// DSAFT: D7-S9-A50-T16.
type ToolSurface interface {
    // ... 现有方法
    IsConcurrencySafe(name string) bool
}
```

#### 19 工具声明 (DM-20260702-009 实施)

| 工具 | IsConcurrencySafe | 理由 |
|------|------------------|------|
| read_file | true | 只读 |
| grep | true | 只读 |
| glob | true | 只读 |
| bash | per-input | 委托给 IsReadOnly (跟 clawcode BashTool.tsx:437-439 一致) |
| edit | false | 写入 |
| write | false | 写入 |
| notebook_edit | false | 写入 |
| webfetch | true | 只读 |
| websearch | true | 只读 |
| lsp | true | 只读 |
| agent | per-input | 委托给 IsReadOnly |
| task_* | per-input | 委托给 IsReadOnly |
| mcp | per-input | 委托给 IsReadOnly |

### 2.6 ToAutoClassifierInput (P1, 走 DM-20260702-009)

```go
// ToAutoClassifierInput returns a compact representation of this tool
// use for the auto-mode security classifier. Returns '' to skip this
// tool in the classifier transcript (tools with no security relevance).
//
// Examples:
//   Bash: return input.command
//   Edit: return file_path + old_string + new_string summary
//   Read/Grep/Glob: return '' (no security relevance)
//
// DSAFT: D7-S10-A50-T20.
type ToolSurface interface {
    // ... 现有方法
    ToAutoClassifierInput(name string, input any) string
}
```

#### Classifier 实现 (DM-20260702-009 实施)

```go
// ClassifyAction evaluates a tool call against a compact transcript of
// the conversation history using a small LLM (Haiku). Returns
// shouldBlock=true if the action is high-risk.
//
// On API errors, returns shouldBlock: false (fail open). Transient
// errors (429, 500) are retried by sideQuery internally.
func ClassifyAction(ctx context.Context, toolName, actionCompact, historyCompact string) (shouldBlock bool, reason string, err error)
```

---

## 3. 测试策略

### 3.1 单元测试

- `persist_test.go`: 正常路径, fail 路径 (磁盘满), image block 跳过, growthbook override
- `content_replacement_state_test.go`: seen, replace, get, clone, decision freeze
- `per_message_budget_test.go`: 临界, 超限, 排序 persist
- `toolchannel/probe_test.go`: advisory 行为, soft/hard/forced threshold, 不再 hard reject
- `toolsurface/builtin_test.go`: IsConcurrencySafe 默认 false, ToAutoClassifierInput 返回 ''

### 3.2 集成测试

- 19 工具 metadata 走 per-tool 阈值, 验证 MaxResultSizeChars 生效
- D2 compression pipeline 集成 PersistToFile, 验证 truncate → persist 切换
- D7 ChannelRouter 集成 IsConcurrencySafe, 验证分桶

### 3.3 端到端测试 (T27, T28)

- **T27**: 50 个文件 review (实际 devrix-monorepo)
  - 旧方案 (8K truncate + Bounded(15)): 任务失败
  - 新方案 (persist + offset/limit + advisory): 任务成功
  - 验证: 平均 Read 次数, 平均 LLM 调用次数, 任务成功率, token 消耗
- **T28**: 8K 自我循环验证 (回归 PR #373 case)
  - 跑 PR #373 当时的 input data 100 次
  - 期望: 100/100 成功
  - 旧方案: 0/100 成功 (验证根因是治标)

### 3.4 性能测试

- persist 写磁盘 IO 开销 (期望 < 10ms for 100K content)
- ContentReplacementState Map lookup 开销 (期望 < 1µs)
- per-message aggregate 200K 评估开销 (期望 < 5ms for 10 tool_results)

---

## 4. 部署 + 监控

### 4.1 Feature flag

- `devrix_persist_threshold_override` (per-tool map, default {})
- `devrix_per_message_budget_chars` (int, default 200_000)
- `devrix_auto_mode_enabled` (bool, default false) — DM-20260702-009 引入
- `devrix_probe_advisory_warn_at` (int, default 20/15) — advisory threshold

### 4.2 监控指标

- `devrix_persist_total` (counter, labels: tool, result)
- `devrix_persist_fail_total` (counter, labels: tool, error)
- `devrix_persist_size_bytes` (histogram, labels: tool)
- `devrix_persist_latency_ms` (histogram, labels: tool)
- `devrix_probe_advisory_warn_total` (counter, labels: task_kind, level)
- `devrix_probe_hard_reject_total` (counter, labels: task_kind) — 期望为 0
- `devrix_per_message_aggregate_persist_total` (counter)
- `devrix_read_offset_limit_used_total` (counter, labels: tool) — 验证 LLM 用了 offset/limit

### 4.3 日志

- 持久化: `info` 级别, 含 toolName, toolUseId, filePath, originalSize
- 持久化失败: `warn` 级别, 含 toolName, error, fall back to truncate
- advisory warning: `debug` 级别, 含 toolName, level, remaining
- decision freeze: `trace` 级别, 含 toolUseId, replaced

---

## 5. 回滚策略

- feature flag 控制: `devrix_persist_enabled` (bool, default true)
- 回滚路径: 关闭 flag → 走旧 truncate 路径
- 数据回滚: persisted files 在 `<projectDir>/<sessionId>/tool-results/`, 不影响其他目录
- 兼容性: Read 工具 offset/limit 参数是 optional, 旧调用方无感

---

## 6. 跟 clawcode 真实设计对齐

| 设计点 | clawcode 真实做法 | devrix v2 |
|--------|------------------|----------|
| 持久化 | `toolResultStorage.ts:persistToolResult` | `compression/persist.go:PersistToFile` |
| 引用 XML | `<persisted-output>...</persisted-output>` | 同 |
| 路径 | `<projectDir>/<sessionId>/tool-results/<toolUseId>.{txt\|json}` | 同 |
| Preview size | 2000 bytes | 同 |
| 决策冻结 | ContentReplacementState | 同 |
| Clone for fork | cloneContentReplacementState | 同 |
| growthbook | tengu_satin_quoll | devrix_persist_threshold_override |
| per-message | MAX_TOOL_RESULTS_PER_MESSAGE_CHARS=200_000 | 同 |
| per-message growthbook | tengu_hawthorn_window | devrix_per_message_budget_chars |
| Read offset/limit | FileReadTool:497 | read_file 工具加 offset/limit |
| per-tool 阈值 | Tool.ts:466 + 决策矩阵 doc 53 §32 | 19 工具 metadata per-tool |
| isConcurrencySafe | Tool.ts:402 + partitionToolCalls | (下个 change) |
| toAutoClassifierInput | Tool.ts:444 + yoloClassifier | (下个 change) |
| isReadOnly | Tool.ts:404 | 走 EmissionClass.Fact (4 类粒度, 包含 isReadOnly) |
| isDestructive | Tool.ts:406 | RiskLevel (已实现) |
| interruptBehavior | Tool.ts:416 | InterruptBehavior (已实现) |
| shouldDefer | Tool.ts:455 | ShouldDeferByDefault (已实现) |
| alwaysLoad | Tool.ts:461 | (下下个 change) |
| searchHint | Tool.ts:368 | (下下个 change) |
| isOpenWorld | Tool.ts:434 | (下下个 change, 仅 UI 提示) |
| isSearchOrReadCommand | Tool.ts:429 | (下下个 change, 仅 UI 折叠) |

---

## 7. 跟 devrix 现有创新对齐

| devrix 创新 | 跟本设计关系 |
|------------|------------|
| EmissionClass 4 类 | 保留, IsConcurrencySafe 可委托给 EmissionClass 决策 |
| task_kind 推 Filter v2 | 保留, Bounded 改 advisory 但 task_kind 维度仍生效 |
| VerifyContract 4 元组 | 保留, 不在本设计范围 |
| MUPS 5 节点 × 4 类 | 保留, 不在本设计范围 |
| Learn FeedbackMemory | 保留, 不在本设计范围 |
| LTL-Lite L4-L6 | 改 advisory, 跟 Bounded 同步 |
| InterruptBehavior | 保留, 不在本设计范围 |
| RiskLevel | 保留, 不在本设计范围 |
| ShouldDeferByDefault | 保留, 不在本设计范围 |
