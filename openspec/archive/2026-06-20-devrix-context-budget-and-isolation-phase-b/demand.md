# Demand: Context Budget & Isolation — Phase B (Sub-Agent Mode + Cache Anchor)

**Demand ID:** DM-20260620-001-B
**Change ID:** 2026-06-20-devrix-context-budget-and-isolation-phase-b
**Created:** 2026-06-20
**Status:** S1_Demand
**Parent:** DM-20260620-001 (Phase A — 2026-06-20 S7_Archived, PR #128 + #129)
**Owner:** devrix core

---

## Background

Phase A (`openspec/archive/2026-06-20-devrix-context-budget-and-isolation/`,
PR #128) 已落地**同 turn 体积治理**:

- AC1 tool result size cap (12K 字符,白名单)
- AC2 assistant output head/tail fold (24K 字符,800+200)
- AC4 per-iter token audit + proactive fold at 60% budget
- AC5 feishu card precheck (ErrCode 11310 防护)
- AC13 TruncateToTokens dead-code 升级

但 **D5 spans 复跑 (AC12)** 显示 22 步后 prompt_tokens 仍达 51K (vs
目标 40K),根因不在单 turn 体积,而在**子 agent 递归**:`SubTurnRunner`
默认全量继承父 history, 3 层 sub-agent 后 messages 爆炸。

## Problem Statement

当前 `SubTurnRunner.RunSubTurn` 行为 (`internal/layers/orchestration/turn/subturn.go:54,96`):

```go
PreloadedMessages: messagesWithoutLastUser(req.Messages),
```

即 `PreloadedMessages` 总是传入父 session 全量历史。**递归 ≥ 2 层时
messages 体积非线性膨胀**:

- 1 层: 父 history (1x)
- 2 层: 父 history + 子-1 history (2x)
- 3 层: 父 history + 子-1 history + 子-2 history (3x)
- D5 spans 实测: 3 层 sub-agent 后单 turn prompt_tokens 跳到 51K

clawcode 已通过 3 mode (`brief` / `fork` / `full`) 解决此问题,
devrix 当前架构无 mode 概念。**对齐 clawcode 是 Phase B 的核心目标**。

## Phase A → Phase B 衔接

| 维度 | Phase A 成果 | Phase B 缺失 |
|------|------------|--------------|
| Tool result 大小 | AC1 cap + 落盘 ✓ | — |
| Assistant 输出 | AC2 fold ✓ | — |
| Turn 边界 | AC4 audit + proactive fold ✓ | — |
| **子 agent 上下文** | **全量继承 (v1 行为)** | **3 mode + depth 限制** |
| **递归深度** | **无限制** | **MaxSubagentDepth=3** |
| Prompt cache 锚点 | — | AC11a (fork 模式 prefix 稳定) + AC11b (Anthropic 锚点, 单独 OpenSpec) |

## S1 Clarifications (从 Phase A 提案 + 调研)

### Q1: minimax provider 的 prompt cache 机制？

**调研结论 (2026-06-20)**:
- `internal/layers/llmgateway/stream/adapter/minimax.go:42` 显示
  minimax 走 **OpenAI-compatible 协议**,无显式 `cache_control` 字段
- OpenAI / OpenAI-compat 的 prompt cache 是**自动按 prefix 命中**,
  无 anchor 概念
- Anthropic SDK 才有 `cache_control: { type: "ephemeral" }` 锚点
  (`internal/layers/llmgateway/stream/adapter/` 内未实现)

**Q1 答案**: devrix 当前默认 provider (minimax) **无显式锚点机制**。
拆为:
- **AC11a (必做)**: fork 模式 prefix 字节级稳定 — 这是**逻辑层
  invariant** (不论 provider 是否利用),对将来切 Anthropic / 加 cache
  都直接收益
- **AC11b (可选 / 单独 OpenSpec)**: Anthropic 专用 cache 锚点注入,
  后续 devrix 支持 Anthropic provider 时单独推进

### Q2: AC12 回归验证方式？

**Q2 答案 (沿用 Phase A)**: D5 spans 原 prompt 22 步复跑,
prompt_tokens P95 ≤ 40K, feishu 0 ERROR。fixture:
`tests/fixtures/d5-spans-replay.jsonl`。

### Q3: AC5 feishu 降级范围？

**Q3 答案 (沿用 Phase A)**: Phase A 已闭环 (PR #128 合并),不重复。

### Q4: SubTurnRunner.Mode 默认值与向后兼容？

**Q4 答案**:
- **新默认 `brief`** (推荐, 节省 token)
- 提供 `legacy_mode=full` 一次性切换 (`devrix.yaml`)
  ```yaml
  context:
    subagent:
      default_mode: brief
      legacy_mode: full        # 旧调用方显式切换
      max_depth: 3
  ```
- Phase B.1 PR 合入后默认 `brief`;如有现网调用方需要旧行为,加
  `legacy_mode: full` 即可
- 后续 minor release 移除 `legacy_mode` 配置项

### Q5: AC5 feishu card table 阈值？

**Q5 答案 (沿用 Phase A)**: 默认 5,Phase A 已闭环。

### Q6 (Phase B 新): AC3 per-iter Prepare 是否纳入 Phase B?

**Q6 答案**: **继续 Defer**。理由:
- AC4 audit + proactive fold 已覆盖高价值场景 (Phase A 实测 P95 ≤ 40K)
- Prepare→LLM→Tool pipeline 重复代价高, systemPrompt + Tools set
  在 turn 内稳定
- 单独 OpenSpec 重新评估 (例如 phase B+)

### Q7 (Phase B 新): Sub-agent recursion 触发 fork vs brief 的策略?

**Q7 答案**:
- 同一 turn 内 sibling sub-agent **必须 share prefix** (favor fork over brief)
- 跨 turn sub-agent **默认 brief** (历史不重放,避免体积累积)
- `mode=fork` 适用场景: D5 spans 调研类 (多步调研需看父 tool_use 链)
- `mode=brief` 适用场景: 单文件查询 / 单次 grep (无需父 history)
- `mode=full` 适用场景: 强依赖父 message 流 (D5 评估场景)

## Goal

**Sub-agent 上下文体积治理**:
- brief mode: 3 层 sub-agent 累积 prompt_tokens ≤ 5K
- fork mode: sibling sub-agent 共享 cache prefix (字节级稳定)
- full mode: 向后兼容, 默认 brief
- depth 限制: 4 层递归被拒

**AC12 目标**: D5 spans 22 步复跑 prompt_tokens P95 ≤ 40K (vs Phase A 51K)。

## Non-Goals

- AC11b (Anthropic cache 锚点) — 单独 OpenSpec,等 Anthropic provider
  接入时推进
- AC3 per-iter Prepare — Defer
- RunRegistry 自身的 disk output 治理 (DM-011)
- agent_tools / multi_agent 配置发现
- LLM provider 选型 / 模型切换
- 跨 session 上下文共享
- 长上下文模型 (Claude Sonnet 4.6 1M context) 适配

## Success Criteria (Phase B)

- [ ] **B.1** AC6 + AC9 落地 (mode + depth),backward-compat (default brief)
- [ ] **B.2** AC10 落地 (tool schema mode 字段)
- [ ] **B.3** AC8 + AC11a 落地 (full 模式 + fork 模式 prefix 稳定)
- [ ] **B.4** docs + legacy mode tests + 切换链路 verify
- [ ] **B.5** AC12 D5 spans 22 步复跑 PASS
- [ ] 全量 `go test ./...` 绿
- [ ] 全量 `go vet ./...` 通过
- [ ] `tools/layer-lint` 通过
- [ ] integration test 覆盖 AC6-AC11a
