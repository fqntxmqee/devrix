# Proposal: Context Budget & Isolation

**Change ID:** `2026-06-20-devrix-context-budget-and-isolation`  
**Demand ID:** DM-20260620-001  
**Created:** 2026-06-20  
**Status:** S2_Clarified

---

## 澄清记录（S2）

### Q1: minimax LLM provider 有无 prompt cache 等价机制？

**A**: devrix 当前默认 provider 为 minimax（`MiniMax-M3`，环境变量 `MINIMAX_API_KEY`）。AC11 涉及 Anthropic `cache_control` 锚点，**minimax 的等价语义需在 S3 design.md 阶段详细调研**：

- 若 minimax 支持 cache_control / 类似锚点 → AC11 按原方案落地
- 若 minimax 无 cache 机制 → AC11 降级为「保留 fork 占位逻辑（为减少 prompt 体积）」，不再标注 cache 锚点语义

**当前决策**：S3 调研；若不支持则 AC11 拆为 AC11a（占位逻辑，必做）和 AC11b（cache 锚点，可选）。

### Q2: AC12 回归验证方式？

**A**: **原 prompt 复跑** —— 将 D5 spans 原 prompt + 用户输入序列保存为 `tests/fixtures/d5-spans-replay.jsonl`，Phase A/B 全部合入后脚本化重跑，对比 `prompt_tokens` P95（应 ≤ 40K）与 feishu ERROR 计数（应为 0）。同时记录 22 步的 token 增长曲线作为 benchmark artifact。

### Q3: AC5 feishu 降级范围？

**A**: **只修 feishu，留可扩展 hook** —— 在 `internal/layers/communication/sender/card_precheck.go` 抽象 `CardContentPrecheck` interface；feishu adapter 实现 `FeishuTableCountPrecheck`；CLI / TUI 不受影响；其他 IM 渠道（lark、微信等）后续按需复用同一接口。

### Q4: SubTurnRunner.Mode 默认值与向后兼容？

**A**: **默认 mode=brief，但提供 legacy_mode=full 一次性切换** ——

```yaml
# devrix.yaml
context:
  subagent:
    default_mode: brief        # 新默认（推荐选项）
    legacy_mode: full          # 旧调用方显式切换用；不设则走 default_mode
    max_depth: 3
```

Phase B.1 PR 合入后默认 brief；如有现网调用方需要旧行为，加 `legacy_mode: full` 即可。后续 minor release 移除 `legacy_mode` 配置项。

### Q5: AC5 feishu card table 阈值？

**A**: **默认 5**（实测 D5 spans 任务在飞书侧首次 ERROR 时已超 5 table）。`MaxTablesPerCard` 在 `devrix.yaml` 可配。

---

## Problem Statement

Devrix turn loop 在多工具调用 + 子 agent 递归场景下，messages 体积以**非线性**方式膨胀：

- 单条 `read_file` 17KB 直接进 messages，无截断 → 单次 +5K tokens
- 14K 字符 assistant 长输出整段回灌 → 后续每轮 +5K tokens  
- 子 agent `PreloadedMessages` 全量继承父 history → 递归 ≥ 2 时爆炸
- 飞书 card table 限额冲突 → 用户视角"中断"，session 静默死亡

根因是 devrix turn loop 在以下三处缺乏防护：

1. **工具结果入口** —— 无 size cap（clawcode 有 50K 单条 + 200K 聚合）
2. **turn 边界** —— 仅 reactive CompressHint（clawcode 有 5 层 proactive pipeline）
3. **子 agent spawn 路径** —— 默认全量继承（clawcode 默认 brief）

工程债定位：

- `prepare/token/counter.go:50-65` 已有 `TruncateToTokens` 启发式，但 turn loop **从不调用**
- D2 `Prepare()` 在 turn loop 开头只跑一次（`orchestrator.go:245`），不是每轮
- `SubTurnRunner.PreloadedMessages` 设计即"全量继承"，无 mode 概念

## Proposed Solution

按 Phase A（基础设施）→ Phase B（架构重构）两阶段推进，**Phase A 先合入 + 验证，Phase B 在 A 基础上做隔离模式重构**。

### Phase A — 同 turn 体积治理（先做）

| AC | 描述 | 度量 |
|----|------|------|
| **AC1** | `read_file` / `bash(grep)` / `bash(find)` / `bash(ls)` / `bash(cat)` 工具结果超 `MaxToolResultChars`（默认 12000）自动 truncate + 标 `<persisted-output>Output too large (N.NKB). Full output saved to: ~/.devrix/tool-results/{uuid}.txt\n\nPreview (first 2KB):\n{preview}\n...</persisted-output>` + 落盘 | tool audit 日志 100% 命中；`MaxToolResultChars` 在 `devrix.yaml` 可配 |
| **AC2** | turn loop 每个 iteration 结束 → per-step 调用 `TruncateToTokens`（已有算法迁移）；单条 assistant 输出 > `MaxAssistantChars`（默认 24000）折叠为 `<prior-output-summary>{前 800 字 + 后 200 字 + 落盘路径}</prior-output-summary>`，原内容写 `~/.devrix/turn-outputs/{sessionID}/{turnN}.md` | per-iteration token count log；落盘文件可读 |
| **AC3** | D2 `Prepare()` 从「循环开头一次」改为「每轮开头」；turn loop `for { ... }` 内每次 LLM call 前调 `prepared := o.Prepare(ctx, req)`；`CompressHint` 触发即压缩 | 单测覆盖；integration test 验证 5+ 工具调用后每轮 Prepare 都被调到 |
| **AC4** | turn 边界 token audit：每 iteration 结束计算 `currentTokens := estimateTokens(messages)`；`currentTokens > ContextBudget * 0.6`（默认 80000 token）时**主动**触发 assistant 长文折叠（不等 CompressHint） | 单元测试覆盖边界条件；LLM 日志 P95 ≤ 40K |
| **AC5** | `internal/layers/communication/sender/card_precheck.go` 抽象 `CardContentPrecheck` interface；feishu adapter 实现 `FeishuTableCountPrecheck`（超 `MaxTablesPerCard` 默认 5 自动改走纯文本路径）；CLI / TUI 不受影响 | feishu ERROR 日志 0 命中"D5 spans 任务"；其他 IM 渠道可复用同一接口 |

### Phase B — 子 agent 上下文隔离（深度重构）

| AC | 描述 | 度量 |
|----|------|------|
| **AC6** | `SubTurnRunner` 新增 `Mode` 字段（`brief` / `fork` / `full`），默认 `brief`；`mode=brief` 时 `PreloadedMessages=nil`，`UserMessage=req.UserMessage` | 单测覆盖 3 种 mode；LLM 日志 sub-agent first call prompt_tokens ≤ 3K |
| **AC7** | `mode=fork` 时构造 `promptMessages = buildForkedMessages(parentMessages, userMessage)`：保留 parent 完整 assistant message（含所有 tool_use），将所有 tool_result 替换为占位 `"Fork started — processing in background"`；保证 prefix 字节级稳定（prompt cache 锚点） | 单测验证 prefix 稳定性；integration test 验证 fork 子 agent 与兄弟 fork 子 agent 共享 cache prefix |
| **AC8** | `mode=full` 保留当前行为（`PreloadedMessages = messagesWithoutLastUser`），需调用方显式声明；用于 D5 这种"必须看完整上下文"的特例 | 单测覆盖；向后兼容旧调用方 |
| **AC9** | `MaxSubagentDepth`（默认 3）递归深度限制；SubTurnRunner 增加 `depth` 参数透传；`depth >= MaxSubagentDepth` 拒绝 spawn 并返回 `ErrSubagentDepthExceeded`；error message 引导调用方改用 `mode=brief` | 单测覆盖 depth=0/1/2/3/4；integration test 验证 4 层递归被拒 |
| **AC10** | LLM 工具 schema 暴露 mode 字段：`delegate` / `free_fork` 工具 input schema 增加 `mode?: "brief" | "fork" | "full"`；缺省时按 brief 处理 | tool schema json dump 验证 |
| **AC11** | prompt cache 锚点：devrix system prompt 顶部（D2 assembler_adapter.go）加 `cache_control: { type: "ephemeral" }` 锚点；所有 LLM call 第一条 message 引用同一 system prompt block。**S3 design.md 阶段需先验证 minimax provider 是否支持 cache_control 等价机制；若不支持则拆为 AC11a（占位逻辑必做）+ AC11b（cache 锚点可选）** | LLM 日志 `cache_read_input_tokens > 0` 命中（若 provider 支持） |

### 跨 AC

| AC | 描述 |
|----|------|
| **AC12** | 所有 AC 完成后，回归 D5 spans 设计任务原 prompt 走一遍：22 步后 prompt_tokens P95 ≤ 40K（vs 当前 51K），feishu 0 ERROR |
| **AC13** | `internal/layers/contextengine/prepare/token/counter.go` `TruncateToTokens` 算法从「未引用」升级为「turn loop 必调」；移除原 `_ = TruncateToTokens` dead-code 标记 |

## Scope

### In Scope

- `internal/layers/orchestration/turn/orchestrator.go` —— per-iteration Prepare + token audit
- `internal/layers/orchestration/turn/subturn.go` —— 3 mode + depth
- `internal/layers/contextengine/prepare/adapters/` —— truncate 落盘 + cache anchor
- `internal/layers/contextengine/prepare/token/counter.go` —— 升级为必调
- `internal/layers/llmgateway/` —— message construction 加 cache_control
- `internal/layers/communication/feishu/` —— card table 数预检
- `devrix.yaml` schema —— `context.budget.*` 配置项
- `openspec/specs/d2-context-engine/t-registry.md` —— 新增 T 点
- `openspec/specs/d7-orchestration/t-registry.md` —— 新增 T 点
- `openspec/specs/d1-communication/t-registry.md` —— feishu adapter 新增 T 点

### Out of Scope

- RunRegistry 自身的 disk output 治理（DM-011）
- D2 QueryLoop 旁路（已 S7_Archived）
- 跨 session 上下文共享
- agent_tools / multi_agent 配置发现
- LLM provider 选型 / 模型切换
- 长上下文模型（如 Claude Sonnet 4.6 1M context）适配

## Impact Analysis

| Component | Change Required | Phase | Details |
|-----------|-----------------|-------|---------|
| orchestrator.go (D7) | Yes | A | per-iteration Prepare + token audit |
| subturn.go (D7) | Yes | B | Mode + depth 字段 |
| prepare/token/counter.go (D2) | Yes | A | 从 dead-code 升级为必调 |
| prepare/adapters/session_loader.go (D2) | Yes | A | truncate 落盘 |
| prepare/adapters/assembler_adapter.go (D2) | Yes | B | cache_control 锚点 |
| llmgateway/message.go (D3) | Yes | B | message construction 加 cache_control block |
| feishu/send.go (D1) | Yes | A | card table 数预检 |
| tool_runner.go (D2) | Yes | A | 工具结果 size cap + persisted-output marker |
| devrix.yaml | Yes | A+B | context.budget.* 配置 |
| openspec t-registry (D2/D7/D1) | Yes | A+B | 新增 ~15 个 T 点 |
| LayerViolationProbe | No | - | 接口契约未变，仅 behavior 变化 |
| Tool surface | Yes (B only) | B | delegate/free_fork schema 加 mode 字段 |

## Architecture Considerations

### 1. clawcode 对齐策略

参考 `clawcode`（`/Users/fukai/workspace/clawcode`）的 5 层 turn boundary pipeline + 3 mode sub-agent：

- **5 层 pipeline**：boundary → budget → snip → microcompact → collapse → autocompact
- **devrix 适配**：A3/A4 实现"boundary + budget + reactive collapse"，snip/microcompact/autocompact 留待后续 change

### 2. prompt cache 锚点设计

clawcode fork 模式核心：

```typescript
// 所有 fork 子 agent 的 prefix 字节级一致 → Anthropic cache 命中
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

devrix 适配（B7）：Anthropic SDK 不直接暴露 cache_control，需在 D3 `llmgateway` 的 message construction 里手动注入。

### 3. 递归深度限制 vs 自由递归

clawcode 通过 `<FORK_BOILERPLATE_TAG>` 检查做 recursion guard（`forkSubagent.ts:78-89`）：

```typescript
const hasBoilerplateTag = messages.some(m => 
  m.content?.includes(FORK_BOILERPLATE_TAG)
);
if (hasBoilerplateTag) return { allowed: false, reason: '...' };
```

devrix 适配（B9）：用显式 `depth` 参数（更可控，更易观测）而非 content tag 检测。

### 4. 子 agent mode 选型

| 场景 | 推荐 mode | 理由 |
|------|----------|------|
| 查单个文件 / 单次 grep | `brief` | 子 agent 不需要上下文 |
| 多步调研（如 D5 spans 跨域设计） | `fork` | 保留父 tool_use 历史，但 tool_result 占位节省 token |
| D5 这种 LLM 必须看完整对话流 | `full` | 显式声明，承担 token 代价 |
| 测试 / fixture | `brief` | 测试场景天然隔离 |

### 5. feishu card table 预检

实测 ErrCode 11310（card table number over limit）触发条件：单 card 中 table tag 数量超飞书内部阈值。预检方案：

```go
func (s *Sender) precheckCardContent(content string) error {
    tableCount := strings.Count(content, "<table")
    if tableCount > s.maxTablesPerCard {
        return ErrTooManyTables{tableCount, s.maxTablesPerCard}
    }
    return nil
}
```

降级路径：超限 → 改走 plain text path（已实现，分支选择即可）。

## Success Criteria

| Milestone | AC 范围 | 核心交付 |
|-----------|---------|---------|
| **Phase A** | AC1, AC2, AC3, AC4, AC5, AC13 | tool result cap + assistant truncate + per-iter Prepare + feishu 预检 |
| **Phase B** | AC6, AC7, AC8, AC9, AC10, AC11 | 3 mode + cache 锚点 + depth 限制 |
| **回归验证** | AC12 | D5 spans 原 prompt 复跑，prompt_tokens P95 ≤ 40K，feishu 0 ERROR |

- [ ] 全量 `go test ./...` 每版绿
- [ ] 全量 `go vet ./...` 通过
- [ ] `tools/layer-lint` 通过
- [ ] integration test 覆盖 AC1-AC11

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| truncate 后 LLM 看不到完整内容导致任务失败 | Med | High | preview 保留前 2K + 落盘路径可读；LLM 可显式 `read_file --offset` 续读 |
| per-iteration Prepare 增加延迟 | Low | Med | Prepare 是 O(n) cache lookup + 常数级 token estimate；22 步任务实测 P95 增加 < 50ms |
| SubTurnRunner mode 拆解破坏向后兼容 | Med | High | `mode` 字段新增，缺省 `full`（旧行为）；下个 minor release 改 `brief` 默认；提供 `devrix.yaml` 一次性切换 |
| prompt cache 锚点不命中 | Med | Low | Anthropic cache 命中率与 message prefix 稳定性强相关；B11 仅在 system prompt 顶部加锚点，user/assistant 消息不变；fallback：无 cache 时性能与当前持平 |
| recursion depth 误拒合法场景 | Low | Med | `MaxSubagentDepth` 默认 3，D5 调研场景实测 ≤ 2；超出时 error message 引导 LLM 改 brief mode |
| feishu 预检 <table> tag 与 markdown 不一致 | Low | Low | 走 `unstructured` 库统一解析；fallback 字符串匹配 |
| TruncateToTokens 算法边界 case | Low | Med | 单测覆盖 unicode、binary、控制字符；A2 落盘原文，L1 fallback 可恢复 |

## Implementation Order

1. **Phase A.1**: AC1（tool result truncate + 落盘）—— 独立 PR，验证落盘稳定性
2. **Phase A.2**: AC2（assistant output fold）—— 独立 PR，依赖 A.1 落盘基础设施
3. **Phase A.3**: AC3 + AC4（per-iter Prepare + token audit）—— 一个 PR，依赖 A.1/A.2
4. **Phase A.4**: AC5（feishu 预检）—— 独立 PR，紧急修复
5. **Phase B.1**: AC6 + AC9（mode + depth）—— 一个 PR，加 backward-compat 默认 full
6. **Phase B.2**: AC10（tool schema 暴露 mode）—— 独立 PR
7. **Phase B.3**: AC7 + AC11（fork mode + cache 锚点）—— 一个 PR
8. **Phase B.4**: AC8（mode=full 显式声明）—— 文档 + 单测，无新代码
9. **回归**: AC12（D5 spans 复跑）—— 验证 PR，依赖 Phase A+B 全部合入

## Reference

- 调研对比：`/Users/fukai/.devrix/logs/llm/unknown.jsonl` 行 3336-3338（46046 → 51076 跳变）
- 飞书错误：sess_1781916669178_3000，10:29:51 起 `feishu API error: code=230099` 重复
- clawcode 参考：`src/query.ts:365-468`（5 层 pipeline），`src/tools/AgentTool/AgentTool.tsx:495-602`（3 mode），`src/utils/toolResultStorage.ts:189-412`（persist-to-disk）
- 现有 dead code：`internal/layers/contextengine/prepare/token/counter.go:50-65` `TruncateToTokens`