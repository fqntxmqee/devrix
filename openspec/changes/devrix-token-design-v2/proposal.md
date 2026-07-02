# Proposal: Token Design 2.0 — clawcode-style persistence + advisory iteration bound

**Change ID:** `devrix-token-design-v2`
**Demand ID:** DM-20260702-008
**Status:** S2_Clarified (复盘 DM-20260701-007 后提案, 待 S3-Gate review)
**Created:** 2026-07-02
**Parent Demand:** `demand.md`
**Supersedes:** devrix-mups-tool-classification-and-channel-autonomy (partial: 5 T 点, 详见 archive/2026-07-02-.../SUPERSEDE-NOTICE.md)

---

## 0. Synthesis Lineage

本文档基于 2026-07-02 对 DM-20260701-007 的复盘, 主要输入:

- `/Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/53-clawcode-tools-design.md` (doc 53, 35 字段)
- `/Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/51-clawcode-context-engine.md` (doc 51, context engine)
- `/Users/fukai/workspace/clawcode/src/Tool.ts` (35 字段真实源码, 792 行)
- `/Users/fukai/workspace/clawcode/src/utils/toolResultStorage.ts` (持久化真实源码)
- `/Users/fukai/workspace/clawcode/src/constants/toolLimits.ts` (50K/100K/200K 常量)
- `/Users/fukai/workspace/clawcode/src/services/tools/toolOrchestration.ts` (isConcurrencySafe 实际用法)
- `/Users/fukai/workspace/clawcode/src/utils/permissions/yoloClassifier.ts` (toAutoClassifierInput 实际用法)

---

## 1. Background (复盘 + 根因)

DM-20260701-007 (PR #374 + PR #375, S7_archived) 通过 4-PR 联动实现 "8K token 自我循环治本":

- PR-A (commit 74fba9c5): ToolSpec v3 + 19 工具默认 metadata
- PR-B-pre: PlanChannel rename
- PR-B: 4 ToolChannel (Fact/Action/Probe/Experiment) + LTL-Lite L4-L6
- PR-C: VerifyContract 4 元组 + Reason 透传 + Learn FeedbackMemory + TruncateWithMarker
- PR-D: Filter v2 三维 + cross-consistency

**复盘发现**: PR-B/C/D 的 **8K token 处理**部分是治标不治本, 核心问题 3 个:

### 1.1 信息物理丢失 (致命)

devrix 真实代码 (`internal/layers/contextengine/prepare/compression/compression_steps.go:14-19`):
```go
if m.Role == types.MessageRoleTool && counter.CountText(m.Content) > maxPerResult {
    out[i].Content = counter.TruncateToTokens(m.Content, maxPerResult) + "\n...[truncated]"
}
```

**问题**: 截断 = 物理消失, LLM 看到 marker 后**只能 REREAD**, 但 REREAD 也截断 → 死循环 → Bounded(15) hard reject → 任务失败。

**实际后果**: 任务失败 → 走 D7 Verify → 标 task_incomplete → D1 红卡 → 用户重发。**这跟 PR #373 的失败模式本质一样, 只是失败点从 D1 表现层挪到 D7 channel 层**。

### 1.2 阈值偏低 + 缺差异化

devrix 19 工具 MaxResultSizeChars 全部 8K chars uniform。实际:
- bash 输出经常 30K-50K (编译日志, 测试输出) — 截 8K 不够
- edit/write 输出 100K+ (生成代码) — 截 8K 不够
- webfetch 整页 markdown 100K+ — 截 4K-8K 不够

clawcode 真实做法:
- Read=Infinity (不持久化, 自带 offset/limit)
- Bash=30K chars
- Grep=20K chars
- Edit/Write/NotebookEdit=100K chars
- Web*/LSP=100K chars
- 全局上限 50K chars, per-message 200K chars, 全局 token 上限 100K

### 1.3 强 reject = 治标

`probe.go:78-82` 的 `ErrProbeToolChannelBoundExceeded` 是把 PR #373 的 task_incomplete 挪到 channel 层。LLM 失败 = 任务失败, 系统强 reject, 治标。

clawcode 真实做法: 无 iteration bound, LLM 自由探索, Read 自带 `offset` + `limit` + `pages` 自治。复杂 review 任务 30+ reads 也能完成。

---

## 2. 借鉴 clawcode 的设计哲学

### 2.1 持久化 (治本)

clawcode 真实做法 (`/Users/fukai/workspace/clawcode/src/utils/toolResultStorage.ts`):
```ts
// 写磁盘: <projectDir>/<sessionId>/tool-results/<toolUseId>.{txt|json}
async function persistToolResult(content, toolUseId): PersistedToolResult {
  const filepath = join(getToolResultsDir(), `${toolUseId}.${ext}`)
  await writeFile(filepath, content, 'utf-8')
  return { filepath, originalSize, isJson, preview, hasMore }
}

// 生成引用消息
function buildLargeToolResultMessage(result: PersistedToolResult): string {
  return `<persisted-output>
Output too large (${formatFileSize(result.originalSize)}). Full output saved to: ${result.filepath}

Preview (first ${formatFileSize(PREVIEW_SIZE_BYTES)}):
${result.preview}
${result.hasMore ? '...' : ''}
</persisted-output>`
}
```

**关键不变量** (ContentReplacementState):
```ts
// State must be stable to preserve prompt cache:
// - seenIds: results that have passed through the budget check (replaced or not).
//   Once seen, a result's fate is frozen for the conversation.
// - replacements: subset of seenIds that were persisted to disk and replaced
//   with previews, mapped to the exact preview string shown to the model.
//   Re-application is a Map lookup — no file I/O, guaranteed byte-identical.
```

devrix 改造: 用同一模式, D2 截断时改成 PersistToFile。

### 2.2 LLM 自治 (offset/limit)

clawcode FileReadTool 真实代码:
```ts
{ file_path, offset = 1, limit = undefined, pages }
```

devrix 改造: read_file 加 `offset` + `limit` 参数 (T10), LLM 自主分段读。

### 2.3 per-message aggregate

clawcode (`toolLimits.ts:50-65`):
```ts
export const MAX_TOOL_RESULTS_PER_MESSAGE_CHARS = 200_000
// Default maximum aggregate size in characters for tool_result blocks within
// a SINGLE user message (one turn's batch of parallel tool results). When a
// message's blocks together exceed this, the largest blocks in that message
// are persisted to disk and replaced with previews until under budget.
```

devrix 改造: 加 per-message 200K 守卫 (T13-T15)。

### 2.4 isConcurrencySafe (并发/串行分桶)

clawcode (`toolOrchestration.ts:84-112`):
```ts
type Batch = { isConcurrencySafe: boolean; blocks: ToolUseBlock[] }

function partitionToolCalls(toolUseMessages, toolUseContext): Batch[] {
  // - 1 个 non-concurrency-safe tool → 单独批 (串行)
  // - 多个 consecutive concurrency-safe tools → 合并批 (并发)
  // fail-closed: safeParse 失败 → false, 抛错 → false
}
```

devrix 改造: ToolSurface 加 `IsConcurrencySafe(name) bool` 字段 (T16, P1 走下个 change)。

### 2.5 toAutoClassifierInput (第二道安全防线)

clawcode (`yoloClassifier.ts:400-420`):
```ts
let encoded: unknown
try {
  encoded = tool.toAutoClassifierInput(input) ?? input
} catch (e) {
  encoded = input
}

// 协议: 返回 '' = "no security relevance" → 跳过 classifier
if (actionCompact === '') {
  return { shouldBlock: false, reason: 'Tool declares no classifier-relevant input' }
}
```

devrix 改造: ToolSurface 加 `ToAutoClassifierInput(name, input) string` (T20, P1 走下个 change)。

---

## 3. 保留 devrix 创新 (clawcode 缺)

| 创新 | 保留实现 | 跟 clawcode 对比 |
|------|---------|----------------|
| **EmissionClass 4 类 (Fact/Action/Probe/Experiment)** | tool 自我分类 + 4 ToolChannel 路由 | clawcode 走 35 字段但无 4 类粒度 |
| **task_kind 推 Filter v2** | review/edit/test/observe task_kind 路由 | clawcode 缺任务类型维度 |
| **VerifyContract 4 元组 (事后治本)** | (Burden × Class × Discipline × Outcome) | clawcode 14 cascade 是事前 |
| **MUPS 5 节点 × 4 类正交分解** | 架构性创新 | clawcode 没这层抽象 |
| **Learn FeedbackMemory (H7 reputation)** | 跨 session 信用累积 | clawcode SessionMemory 较简单 |
| **LTL-Lite 4 节点 (改 advisory)** | L4-Bounded/L5-Quotient/L6-Synthesize 保留为观测信号 | clawcode 缺 |
| **InterruptBehavior (Cancel/Block)** | 工具级 abort 行为 | 跟 clawcode interruptBehavior 同, **已实现** |
| **RiskLevel** | 风险等级 | 跟 clawcode isDestructive 同, **已实现** |
| **ShouldDeferByDefault** | 工具懒加载 | 跟 clawcode shouldDefer 同, **已实现** |

---

## 4. T 点拆解 (28 T 点, 7 阶段)

### 阶段 0: 决策 (本次, 0 T 点)
- [x] close PR #375
- [x] archive DM-20260701-007 标 partial supersede (SUPERSEDE-NOTICE.md)
- [x] 起草本 proposal (本文档)
- [ ] 开新 feature branch `feat/devrix-token-design-v2`

### 阶段 1-2: 持久化层 (P0, 8 T 点)

T01 — PersistToFile 核心实现 (`compression/persist.go`)
T02 — PrepareExecutionContext 集成
T03 — image block 跳过
T04 — ContentReplacementState 决策冻结
T05 — growthbook override
T06 — surface_metadata_gate_test 加 PersistThreshold
T07 — 19 工具 metadata 改 per-tool 差异化
T08 — PersistToFile 测试

### 阶段 3: Bounded(15) 改 advisory (P0, 4 T 点)

T09 — ProbeToolChannel.Accept 改 advisory (probe.go:75-85)
T10 — Read 工具加 offset/limit 参数
T11 — ProbeToolChannel 默认 OpenEnded
T12 — task_kind 推改 advisory

### 阶段 4: per-message aggregate (P0, 3 T 点)

T13 — PerMessageBudget 守卫 (200K)
T14 — 集成到 PrepareExecutionContext
T15 — per-message aggregate 测试

### 阶段 5: IsConcurrencySafe (P1, 4 T 点) — 下个 change DM-20260702-009

T16 — ToolSurface 加 IsConcurrencySafe
T17 — 19 工具 surface 声明
T18 — ChannelRouter 集成
T19 — 测试

### 阶段 6: ToAutoClassifierInput (P1, 5 T 点) — 下个 change DM-20260702-009

T20 — ToolSurface 加 ToAutoClassifierInput
T21 — 19 工具 surface 声明
T22 — auto-mode classifier
T23 — ChannelRouter 集成
T24 — classifier 测试

### 阶段 7: 验证 (P0, 3 T 点 + 1 LTL-Lite)

T25 — LTL-Lite L4-L6 改 advisory
T26 — go test -race ./... PASS
T27 — 端到端 review 任务测试 (50 文件 review)
T28 — 8K 自我循环验证 (回归 PR #373 case, 100/100 成功)

---

## 5. PR 路线图

| PR | 内容 | T 点 | 依赖 |
|----|------|------|------|
| **PR-A** | 阶段 1-2: 持久化层 | T01-T08 (8 T) | 无 |
| **PR-B** | 阶段 3: Bounded 改 advisory | T09-T12 (4 T) | PR-A |
| **PR-C** | 阶段 4: per-message aggregate | T13-T15 (3 T) | PR-A |
| **PR-D** | 阶段 5+6: ConcurrencySafe + Classifier | T16-T24 (9 T) | PR-B |
| **PR-E** | 阶段 7: 验证 + LTL-Lite | T25-T28 (4 T) | PR-B, PR-C |
| **总计** | | **28 T** | |

**P0 (必做, 19 T)**: T01-T15 + T25-T28
**P1 (增量, 9 T)**: T16-T24, 走下个 change (DM-20260702-009)

---

## 6. 风险评估

| 风险 | 缓解 |
|------|------|
| Read offset/limit 改 API 破坏现有调用方 | 默认 offset=0, limit=8K 兼容, 旧调用方无感 |
| persist 写磁盘 IO 开销 | growthbook override, 生产可关; image block 不 persist |
| LLM 不会用 offset/limit | prompt 注入说明 (跟 clawcode FileReadTool prompt 一致) |
| advisory Bounded 治本失效 (LLM 仍然 30+ reads) | InjectPromptPressure 仍触发, soft → hard → forced 3 阶段 |
| persist 失败 (磁盘满) | fall back to truncate + 日志 warn, 不丢失任务 |
| auto-mode classifier 误判 | fail open (API 错误), 返回 '' 跳过, 人类 review |
| per-message 200K 阈值跟 per-tool 30K 不一致 | 200K 是 aggregate 上限, 跟 per-tool 是两层, 不冲突 |
| decision freeze 跟 LLM 探索冲突 | freeze 是 cache-stable 保证, 跟 LLM 探索无关 |
