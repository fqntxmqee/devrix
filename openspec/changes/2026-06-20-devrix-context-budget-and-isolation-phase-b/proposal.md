# Proposal: Context Budget & Isolation — Phase B (Sub-Agent Mode + Cache Anchor)

**Change ID:** 2026-06-20-devrix-context-budget-and-isolation-phase-b
**Demand ID:** DM-20260620-001-B
**Created:** 2026-06-20
**Status:** S2_Proposal
**Parent:** DM-20260620-001 Phase A (S7_Archived 2026-06-20, PR #128 + #129)
**Base:** clawcode 3-mode sub-agent pattern (reference `/Users/fukai/workspace/clawcode/src/tools/AgentTool/AgentTool.tsx:495-602`)

---

## 澄清记录（S2）

### Q1: minimax provider 的 prompt cache 机制？

**A**: 调研结论（2026-06-20，commit `5fc85b9` 已合）：
- `internal/layers/llmgateway/stream/adapter/minimax.go:42` 显示
  minimax 走 **OpenAI-compatible 协议**,无显式 `cache_control` 字段
- OpenAI / OpenAI-compat 的 prompt cache 是**自动按 prefix 命中**,
  无 anchor 概念
- Anthropic SDK 才有 `cache_control: { type: "ephemeral" }` 锚点
- devrix 当前**未实现** Anthropic 专用 adapter, 锚点代码无意义

**当前决策**:
- **AC11a (必做)**: fork 模式 prefix 字节级稳定 — 逻辑层 invariant,
  对将来切 Anthropic / 加 cache 都直接收益
- **AC11b (Defer / 单独 OpenSpec)**: Anthropic 专用 cache 锚点,
  等 devrix 支持 Anthropic provider 时单独推进 (DM-2026XXXX-XXX)

### Q2: AC12 回归验证方式？

**A** (沿用 Phase A): D5 spans 原 prompt 22 步复跑, prompt_tokens P95
≤ 40K (vs Phase A 51K), feishu 0 ERROR。fixture:
`tests/fixtures/d5-spans-replay.jsonl`。

### Q3-Q5: (沿用 Phase A,无新澄清)

### Q6: AC3 per-iter Prepare 是否纳入 Phase B?

**A**: **继续 Defer**。理由:
- AC4 audit + proactive fold 已覆盖高价值场景 (Phase A 实测 P95 ≤ 40K)
- Prepare→LLM→Tool pipeline 重复代价高
- 单独 OpenSpec 重新评估

### Q7: Sub-agent recursion 触发 fork vs brief 的策略?

**A**:
- 同一 turn 内 sibling sub-agent **share prefix** → 用 fork (favor cache)
- 跨 turn sub-agent **默认 brief** (历史不重放,避免体积累积)
- `mode=fork` 适用: D5 spans 调研类 (多步调研需看父 tool_use 链)
- `mode=brief` 适用: 单文件查询 / 单次 grep
- `mode=full` 适用: 强依赖父 message 流 (D5 评估场景,需显式声明)

---

## Problem Statement (Phase B 增量)

Phase A 修了"单 turn 内部体积"（tool result / assistant 输出 / per-iter
audit），但**子 agent 递归**这条路未治本：

- 1 层 sub-agent: 父 history (1x)
- 2 层 sub-agent: 父 history + 子-1 history (2x)
- 3 层 sub-agent: 父 history + 子-1 history + 子-2 history (3x)
- D5 spans 实测 22 步后 prompt_tokens 51K（vs 目标 40K）

根因是 `SubTurnRunner.RunSubTurn`（`internal/layers/orchestration/turn/subturn.go:54,96`）默认全量继承父 history:

```go
PreloadedMessages: messagesWithoutLastUser(req.Messages),  // 总是全量
```

无 mode 概念,无 depth 限制。`SubTurnRequest` 也无 Mode 字段
（`internal/shared/contracts/subturn.go:20-33`），调用方无法表达
"我不需要父 history"。

clawcode 通过 3 mode (brief/fork/full) + `MaxSubagentDepth` 解决。
devrix 需要对齐。

## Proposed Solution

按 **B.1 → B.5** 五阶段推进，每阶段独立 PR：

| AC | 描述 | 度量 | PR |
|----|------|------|----|
| **AC6** | `SubTurnRunner` 新增 `Mode` 字段（`brief`/`fork`/`full`），默认 `brief`；`mode=brief` 时 `PreloadedMessages=nil`，`UserMessage=req.UserMessage` | 单测覆盖 3 mode；LLM 日志 sub-agent first call prompt_tokens ≤ 3K | B.1 |
| **AC9** | `MaxSubagentDepth`（默认 3）递归深度限制；`SubTurnRunner` 增 `depth` 参数透传；`depth >= MaxSubagentDepth` 拒绝 spawn 并返回 `ErrSubagentDepthExceeded`；error message 引导改 `mode=brief` | 单测覆盖 depth=0/1/2/3/4；integration test 验证 4 层递归被拒 | B.1 |
| **AC10** | LLM 工具 schema 暴露 mode 字段：`delegate` / `free_fork` 工具 input schema 增加 `mode?: "brief" \| "fork" \| "full"`；缺省时按 brief 处理 | tool schema json dump 验证 | B.2 |
| **AC8** | `mode=full` 保留当前行为（`PreloadedMessages = messagesWithoutLastUser`），需调用方显式声明；用于 D5 这种"必须看完整上下文"的特例 | 单测覆盖；向后兼容旧调用方 | B.3 |
| **AC11a** | `mode=fork` 时构造 `promptMessages = buildForkedMessages(parentMessages, userMessage)`：保留 parent 完整 assistant message（含所有 tool_use），将所有 tool_result 替换为占位 `"Fork started — processing in background"`；保证 prefix 字节级稳定（prompt cache 锚点） | 单测验证 prefix 稳定性；integration test 验证 fork sibling sub-agent 共享 cache prefix | B.3 |
| **AC12** | 回归 D5 spans 设计任务原 prompt 走一遍：22 步后 prompt_tokens P95 ≤ 40K（vs 当前 51K），feishu 0 ERROR | D5 spans replay script + 22 步 token 增长曲线 benchmark | B.5 |

### Sub-PR 拆分

| PR | 范围 | 风险 | 依赖 |
|----|------|------|------|
| **B.1** | AC6 + AC9 — Mode + Depth + legacy_mode=full 切换 | Low (新增字段,默认 brief,旧调用方无感) | — |
| **B.2** | AC10 — tool schema mode 字段 (delegate + free_fork) | Low (schema 扩展,缺省 brief) | B.1 (mode 字段已存在) |
| **B.3** | AC8 + AC11a — full 模式 + fork 模式 prefix 稳定 | Med (buildForkedMessages 复杂, prefix 稳定性需严格测试) | B.1 |
| **B.4** | docs + legacy mode 测试 + 切换链路 verify | Low (docs + tests) | B.1-B.3 |
| **B.5** | AC12 — D5 spans 22 步复跑回归 | Med (依赖所有 sub-agent mode 落地) | B.1-B.3 |

### 跨 AC

| AC | 描述 |
|----|------|
| **AC12** | 所有 sub-PR 合入后, 回归 D5 spans 设计任务原 prompt 走一遍: 22 步后 prompt_tokens P95 ≤ 40K (vs 当前 51K), feishu 0 ERROR |
| **AC11b (DEFERRED)** | Anthropic 专用 cache 锚点 (cache_control: ephemeral) — 单独 OpenSpec, 等 Anthropic provider 接入时推进 |

## Backward Compatibility

`devrix.yaml` 新配置段:

```yaml
context:
  subagent:
    default_mode: brief        # Phase B.1 新默认 (推荐)
    legacy_mode: full          # 旧调用方显式切换用; 不设则走 default_mode
    max_depth: 3               # Phase B.1 默认 3
```

- Phase B.1 合入后默认 `brief`; 如有现网调用方需要旧行为, 加
  `legacy_mode: full` 即可
- 后续 minor release 移除 `legacy_mode` 配置项 (发出 deprecation warning)
- 行为变更在 acceptance-report.md 显式标注, 触发 semver MINOR bump

## Scope

### In Scope

- `internal/shared/contracts/subturn.go` — `SubTurnRequest.Mode`/`Depth` 字段
- `internal/layers/orchestration/turn/subturn.go` — `SubTurnRunner.Mode`/`Depth` 字段 + 3 mode 逻辑
- `internal/layers/orchestration/turn/subturn_fork.go` (新) — `buildForkedMessages` 实现
- `internal/layers/orchestration/turn/subturn_test.go` — 3 mode × depth 边界测试
- `internal/shared/errors/subturn.go` (新) — `ErrSubagentDepthExceeded` sentinel
- `internal/layers/orchestration/delegatetools/freefork.go` — `delegate` / `free_fork` 工具 schema 加 mode 字段
- `internal/layers/contextengine/enforce/subquery.go` — 透传 `Depth` 字段
- `devrix.yaml` schema — `context.subagent.*` 配置段
- `openspec/specs/d7-orchestration/t-registry.md` — 新增 ~6 个 T 点 (B.1-B.3)
- `openspec/specs/d4-multi-agent/t-registry.md` — 新增 ~2 个 T 点 (B.2 schema)
- `openspec/specs/d2-context-engine/t-registry.md` — 新增 ~3 个 T 点 (B.3 fork prefix)

### Out of Scope

- AC11b (Anthropic cache 锚点) — 单独 OpenSpec
- AC3 (per-iter Prepare) — Defer
- RunRegistry 自身的 disk output 治理 (DM-011)
- agent_tools / multi_agent 配置发现
- LLM provider 选型 / 模型切换
- 跨 session 上下文共享
- 长上下文模型 (Claude Sonnet 4.6 1M context) 适配
- Anthropic provider 接入

## Impact Analysis

| Component | Change Required | Phase | Details |
|-----------|-----------------|-------|---------|
| `internal/shared/contracts/subturn.go` | Yes | B.1 | `SubTurnRequest.Mode`/`Depth` 字段新增 |
| `internal/layers/orchestration/turn/subturn.go` | Yes | B.1 | `SubTurnRunner.Mode`/`Depth` 字段 + 3 mode dispatch + depth check |
| `internal/layers/orchestration/turn/subturn_fork.go` | Yes (新) | B.3 | `buildForkedMessages` 工具结果占位逻辑 |
| `internal/layers/orchestration/delegatetools/freefork.go` | Yes | B.2 | delegate/free_fork 工具 schema mode 字段 |
| `internal/layers/contextengine/enforce/subquery.go` | Yes | B.1 | 透传 Depth 字段到 SubTurnRequest |
| `internal/layers/contextengine/enforce/freefork/subquery.go` | Yes | B.1 | 透传 Depth 字段到 SubTurnRequest |
| `internal/bootstrap/wire_coordinator.go` | Yes | B.1 | `NewSubTurnRunner(orch, SubTurnConfig{DefaultMode, MaxDepth})` 配置注入 |
| `internal/shared/config/orchestration.go` | Yes | B.1 | 新增 `Context.Subagent.DefaultMode` / `MaxSubagentDepth` 字段 |
| `devrix.yaml` | Yes | B.1 | `context.subagent.*` 配置段 |
| `LayerViolationProbe` | No | - | 接口契约扩展 (新增字段) 非破坏性 |
| `openspec t-registry` | Yes | B.1-B.5 | 新增 ~11 个 T 点 |

## Architecture Considerations

### 1. clawcode 对齐策略

参考 clawcode `src/tools/AgentTool/AgentTool.tsx:495-602` 的 3 mode:

| clawcode mode | devrix 适配 | SubTurnRequest.PreloadedMessages |
|---------------|------------|----------------------------------|
| `general-purpose` (= brief) | `brief` | `nil` (子 agent 全新 history) |
| `statusline-setup` 等特化 | `fork` | `buildForkedMessages(parentMsgs, userMsg)` |
| `Explore` (= full) | `full` | `messagesWithoutLastUser` (旧行为) |

devrix 3 mode 名称 (`brief`/`fork`/`full`) 与 clawcode 略有差异,
但语义对齐。

### 2. fork 模式 prefix 稳定算法

clawcode fork 模式核心 (TypeScript, `src/tools/AgentTool/AgentTool.tsx:495-602`):

```typescript
const fullAssistantMessage = clone(parentAssistantMessage);
const toolResultMessage = {
  role: 'user',
  content: toolResults.map(r => ({
    type: 'tool_result',
    tool_use_id: r.id,
    content: 'Fork started — processing in background'
  }))
};
```

**devrix 适配 (B.3)**:

```go
// internal/layers/orchestration/turn/subturn_fork.go
func buildForkedMessages(parentMsgs []types.Message, userMsg types.Message) []types.Message {
    forked := make([]types.Message, 0, len(parentMsgs)+1)

    // 1. 保留所有 parent assistant message (含 tool_use)
    for _, m := range parentMsgs {
        if m.Role == types.MessageRoleAssistant {
            forked = append(forked, m)
            continue
        }
        if m.Role == types.MessageRoleUser && len(m.ToolResults) > 0 {
            // tool_result → 占位 "Fork started — processing in background"
            forked = append(forked, types.Message{
                Role:    types.MessageRoleUser,
                Content: "Fork started — processing in background",
                ToolResults: m.ToolResults,  // 保留 tool_call_id 引用
            })
            continue
        }
        // system / 纯 user 消息 → 保留
        forked = append(forked, m)
    }

    // 2. 追加子 agent 自己的 user message
    forked = append(forked, userMsg)
    return forked
}
```

**Prefix 稳定性保证**:
- 所有 fork sibling sub-agent 调用 `buildForkedMessages` 时, 父 messages
  完全相同 → fork 后的 prefix 字节级一致 → 后续切 Anthropic 时自动获 cache 收益
- 占位字符串 `"Fork started — processing in background"` 字节级固定
- tool_use_id / tool_result 引用结构保留 (LLM schema 兼容)

### 3. 递归深度限制

`MaxSubagentDepth=3` 拒绝 4 层递归。error message 引导调用方改 `mode=brief`:

```go
// internal/layers/orchestration/turn/subturn.go
if req.Depth >= r.cfg.MaxSubagentDepth {
    return nil, fmt.Errorf("subturn: depth %d >= max %d (hint: use mode=brief to reduce context size): %w",
        req.Depth, r.cfg.MaxSubagentDepth, errors.ErrSubagentDepthExceeded)
}
```

**深度计数**: `Depth=0` 为 root turn, 1/2/3 为 sub-agent, ≥3 拒绝。
D5 spans 实测 ≤ 2 层,3 层 (含 4 次 spawn) 极少。

### 4. 工具 schema 暴露 mode 字段 (B.2)

`delegate` / `free_fork` 工具 input schema 增加 `mode`:

```json
{
  "type": "object",
  "properties": {
    "agent": {"type": "string"},
    "task": {"type": "string"},
    "mode": {"type": "string", "enum": ["brief", "fork", "full"], "default": "brief"}
  },
  "required": ["agent", "task"]
}
```

LLM 看到 mode 选项后, D5 spans 类调研场景可显式选 `fork`,
单文件查询场景选 `brief` (默认)。

## Success Criteria

| Milestone | AC 范围 | 核心交付 | 验证 |
|-----------|---------|---------|------|
| **B.1** | AC6 + AC9 | Mode + Depth + legacy_mode=full | unit + integration -race |
| **B.2** | AC10 | delegate/free_fork schema mode 字段 | tool schema json dump |
| **B.3** | AC8 + AC11a | full 模式 + fork 模式 prefix 稳定 | unit + integration (prefix 字节级断言) |
| **B.4** | docs + legacy mode tests | docs + 切换链路 | docs sync |
| **B.5** | AC12 | D5 spans 22 步复跑 | prompt_tokens P95 ≤ 40K, feishu 0 ERROR |

- [ ] 全量 `go test ./...` 每版绿
- [ ] 全量 `go vet ./...` 通过
- [ ] `tools/layer-lint` 通过
- [ ] integration test 覆盖 AC6-AC11a
- [ ] B.5 D5 spans 22 步 prompt_tokens P95 ≤ 40K (从 51K 降)
- [ ] feishu 0 ERROR 命中"D5 spans 任务"

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| brief mode 丢失父上下文导致子 agent 任务失败 | Med | High | error message 明确引导调用方改 `mode=fork` 或 `mode=full`; unit + integration test 覆盖 fallback 路径 |
| fork 模式 prefix 字节级不稳定 → 后续切 Anthropic 时 cache miss | Low | High | unit test 用 `bytes.Equal` 断言 sibling fork prefix 字节级一致; 严格覆盖占位字符串字面量 |
| depth 限制误拒合法场景 | Low | Med | `MaxSubagentDepth=3` 默认, D5 调研场景实测 ≤ 2; 超 3 时 error 引导 brief |
| tool schema 加 mode 后 LLM 调用 schema 不匹配 | Low | Med | 缺省时按 brief 处理, 向后兼容; integration test 覆盖 default |
| legacy_mode=full 切换链路漏一个 caller | Med | Med | B.4 docs 显式列出所有 caller; integration test 覆盖完整链路 |
| B.5 D5 spans 复跑不达 40K | Med | Med | Phase A 已 P95 ≤ 40K 单 turn, Phase B 治理递归; 失败则回 B.1 调 depth / brief |
| recursion 误拒导致 worker 任务失败 | Low | Med | 4 层递归 D5 实测无; error message 引导 brief |

## Implementation Order

1. **B.1**: AC6 + AC9 — Mode + Depth + default brief, 1 PR, 1 squash auto-merge
2. **B.2**: AC10 — delegate/free_fork schema mode 字段, 1 PR, 1 squash auto-merge
3. **B.3**: AC8 + AC11a — full 模式测试 + fork 模式 + prefix 稳定, 1 PR, 1 squash auto-merge
4. **B.4**: docs + legacy mode tests + 切换链路 verify, 1 PR (docs+tests), 1 squash auto-merge
5. **B.5**: AC12 — D5 spans 22 步复跑回归, 1 verification PR (B.5 commit may be empty if regression script standalone)

## Reference

- 调研对比 (Phase A): `/Users/fukai/.devrix/logs/llm/unknown.jsonl` 行 3336-3338 (46046 → 51076 跳变)
- D5 spans 任务: 22 步实测 prompt_tokens 51K (vs 目标 40K)
- clawcode 参考:
  - `src/tools/AgentTool/AgentTool.tsx:495-602` (3 mode)
  - `src/query.ts:365-468` (5 层 pipeline)
  - `src/utils/toolResultStorage.ts:189-412` (persist-to-disk, Phase A 已对齐)
- 现有 dead code (Phase A 已修): `internal/layers/contextengine/prepare/token/counter.go:50-65` `TruncateToTokens` → 必调
- Phase A 归档: `openspec/archive/2026-06-20-devrix-context-budget-and-isolation/`
