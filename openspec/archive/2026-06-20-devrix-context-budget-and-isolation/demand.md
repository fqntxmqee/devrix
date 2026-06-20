---
demand-id: DM-20260620-001
title: Context Budget & Isolation — messages 体积治理 + 子 agent 上下文隔离
source: D5 spans 设计任务实测（prompt_tokens 46046→51076 单跳 +5000 / 22 步 123 messages / 51K tokens）
priority: P0
status: S1_Proposal
dsaft_domain: orchestration,context-engine,multiagent
created: 2026-06-20
---

# Context Budget & Isolation

## 1. 背景

2026-06-20 用户在飞书发起 D5 observability spans 设计任务（sess_1781916669178_3000），实测 devrix turn loop 在 22 步工具调用后 LLM request 已膨胀到 **51,076 prompt tokens / 123 messages**。同一回复因飞书 card table 超限被拒，devrix 重试无果，session 实际上"已完成但用户不可见"。

事后审计 `~/.devrix/logs/llm/unknown.jsonl` 发现 3 类非预期的体积来源：

| 来源 | 单次增量 | 累计影响 |
|------|----------|----------|
| `read_file` / `bash(grep)` 全量 dump | 单条 tool result 最高 17,009 字符（stream.go） | +5000~8000 tokens / 次 |
| assistant 长输出回灌 | 14,281 字符的 D5 设计文档完整保留 | +5000 tokens / turn 边界 |
| 子 agent `PreloadedMessages` 全量继承 | 51K tokens prior + brief | 递归 ≥ 2 时爆炸性增长 |

与 clawcode 对照分析（详见 `proposal.md` § Architecture Considerations）：

| 维度 | devrix 现状 | clawcode | 影响 |
|------|-------------|----------|------|
| 同 turn 内多工具调用 | 全量累积，无 per-step 截断 | 全量发送，但过 5 层压缩管道 | devrix 单 turn 可不可逆膨胀 |
| 工具结果超限 | 无 cap，in-band 存储 | 50K 单条 + 200K 聚合 → 落盘 + preview | devrix 单次 read 即可爆 prompt |
| Turn 边界 | 仅 D2 reactive `CompressHint` 触发 | 5 层 pipeline 必跑（boundary/budget/snip/microcompact/collapse/autocompact） | devrix 缺乏 proactive 防护 |
| 子 agent 默认继承 | **全量 parent history** (`PreloadedMessages`) | **只传任务 brief** | devrix 递归成本结构失控 |
| 子 agent fork | 同 session COW snapshot | 全量 history + 占位 tool_result（**为 prompt cache 刻意设计**） | devrix 无 prompt cache 锚点概念 |
| Prompt cache 稳定性 | 未启用 | `ContentReplacementState` 用 seenIds 做确定性替换 | devrix 完全没有 cache 优化 |

## 2. 问题陈述

| 场景 | 现状 | 风险 |
|------|------|------|
| LLM 调 `read_file` 读 17KB 文件 | 完整内容塞进 messages，下一轮全量重发 | 单次工具调用即可 +5K tokens |
| LLM 写出 14K 设计文档 | 下一轮整段 14K assistant content 回灌 | 已完成回合的"过期长文"持续占 token |
| 22 步工具调用后第 23 次 LLM call | prompt_tokens 51K、messages 123 条 | 已触及 MiniMax 8K 输出上限的 6 倍预算 |
| Turn 中段 spawn sub-agent 查细节 | sub-agent 上来先吞 51K tokens 父 history | 递归深度 ≥ 2 时单次 LLM call > 100K tokens |
| 子 agent 再 spawn 孙 agent | 孙 agent 吞 51K + 中间 N 步 ≈ 70K+ tokens | 递归失控 |
| 用户用飞书发起 D5 这种跨域调研 | LLM 自然倾向于多 read_file + 长设计输出 | 与飞书 50K char/card table 限额冲突 |
| 飞书拒收响应 | devrix 重试无果 | 用户视角"中断"，实际 session 静默死亡 |

## 3. 终态目标

### Phase A — 同 turn 体积治理（先做）

- **A1**: `read_file` / `bash(grep)` / `bash(find)` 三类工具结果超 `MaxToolResultChars`（默认 12K 字符）自动 truncate + 标 `<persisted-output>` + 落 `~/.devrix/tool-results/{uuid}.txt`
- **A2**: turn loop 每个 iteration 结束 → per-step 调用 `TruncateToTokens`（已有算法，`prepare/token/counter.go:50-65`），单条 > 24K 字符的 assistant 输出折叠为 `<prior-output-summary>` + 落盘
- **A3**: D2 `Prepare()` 从"循环开头跑一次"改为"每轮开头跑"，输出 `CompressHint` 主动告警；turn loop 据 hint 触发 reactive 压缩
- **A4**: turn 边界增加 per-iteration token audit，> 60% 上下文预算（默认 80K）时**主动**触发 assistant 长文折叠（不等 hint）

### Phase B — 子 agent 上下文隔离（深度重构）

- **B1**: SubTurnRunner 拆出 3 种 mode：
  - `mode=brief`：只传任务 brief（默认，**对齐 clawcode**）
  - `mode=fork`：传父 history + 占位 tool_result（clawcode fork 模式）
  - `mode=full`：保留当前 `PreloadedMessages` 全量（向后兼容，需显式声明）
- **B2**: prompt cache 锚点 —— devrix system prompt 顶部加 `cache_control: { type: "ephemeral" }` 锚点；fork mode 子 agent 输出 placeholder `"Fork started — processing in background"`，保证 prefix 字节级稳定
- **B3**: 递归深度限制 —— SubTurnRunner 增加 `depth` 参数透传，超过 `MaxSubagentDepth`（默认 3）拒绝 spawn 并报错
- **B4**: LLM 工具 schema 暴露 mode 字段 —— `delegate` / `free_fork` 工具调用时 LLM 可显式选择 mode；`mode` 缺省时按 brief 处理

## 4. 不在范围

- RunRegistry 自身的 disk output 治理（DM-011 已覆盖）
- D2 QueryLoop 旁路（已 S7_Archived，DM-20260617-008）
- 跨 session 上下文共享
- agent_tools / multi_agent 配置发现

## 5. 验收维度（顶层）

| 维度 | 目标 | 度量 |
|------|------|------|
| 单次 LLM call prompt_tokens | 22 步任务下不超过 40K | LLM 日志 `prompt_tokens` P95 |
| 单条 tool result | 永远 ≤ 12K 字符 | tool result audit 日志 |
| 子 agent 启动 token | 默认 mode=brief 下不超过 3K | sub-agent first LLM call |
| 递归 2 层总 token | 不超过 50K | 父 + 子 + 孙累计 |
| 飞书 card 拒收率 | D5 这种 22 步任务 0 拒收 | feishu ERROR 日志计数 |

详细 AC 见 `proposal.md` § Success Criteria。