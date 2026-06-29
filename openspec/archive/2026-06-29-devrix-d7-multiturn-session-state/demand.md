---
demand-id: DM-20260628-004
title: D7 多轮 session 串行化与 complete 时机修正 — WaitForTurnCompletion + turn 上下文自动注入
priority: P0
status: S6_Archived (2026-06-29, PARTIAL — RC-3 hotfix done via PR #271, RC-1/2/4 deferred to v1.1)
dsaft_domain: orchestration
created: 2026-06-28
reporter: 2026-06-28 17:31:04 sess_1782638991113_5000 panic hotfix（PR #271）现场跟踪 — multi-turn session 体验缺陷
related:
  - PR-271-hotfix（DM-20260628-002）已修复 send-on-closed-channel panic，但暴露更深层 session 生命周期问题
  - 2026-06-20 devrix-final-text-accumulation-pr137（PR #137）首次揭示"finalText 跨 turn 累积"需求
  - 2026-06-27 devrix-prior-output-summary-strip-pr149（PR #149 iter3）在 D1 边界剥离 <prior-output-summary> 标签，说明 turn 上下文已在 prompt 中被注入
---

# D7 多轮 session 串行化与 complete 时机修正

## 1. 背景

2026-06-28 17:31 sess_1782638991113_5000 用户发送 `review d2 领域 kernel` 后追问 "你并没有给出 review 结论？哪里有问题吗"，devrix 在第二轮 panic 退出（已通过 PR #271 修复）。但 panic 修复后的飞书实测暴露两个**更深层的 session 生命周期缺陷**：

### 1.1 complete 事件触发太早（用户感知："devrix 还没说完就 Done 了"）

- 18:34:28 turn 1 `complete` 事件已 emit，飞书卡片显示 ✅ Done
- 18:34:28 → 18:35:42 期间 turn 1 仍在跑后续子 WorkItem（review 展开的 spawn decompose 子任务）
- 18:35:42 才真正 finalText 落地

根因：`internal/layers/orchestration/sessionorchestrator/session_turn_loop.go:186-189` 当 `for iter` 循环 break 后**立即** emit `complete`，但：
- 循环 break 仅代表当前 ProcessMessage 调用内的"主线"已闭环
- 子 WorkItem（如 SpawnDecompose 拆出的 review 子任务）并未等到 terminal
- 飞书卡片拿到 `complete` 即渲染 Done emoji，后到的 finalText 不再追加

### 1.2 turn N+1 缺乏 turn N 上下文（用户感知："第二条没第一条记忆"）

- turn 1 finalText 已在 18:35:42 完成，但 prompt 落盘时**未注入**到 WorkItem.Directive
- turn 2 (18:35:17 进站）开启新 EnsureGoal 时，只看到上一轮的 directive 原话（"review d2 领域 kernel"），看不到 turn 1 finalText
- turn 2 启动新一轮 LLM ReAct loop 时直接跑 `git status && git diff --stat`，把 turn 1 当作没发生过

根因：
1. `RunSessionTurnLoop` 每次调用都是**独立的 goroutine + 独立的 channel**（`session_turn_loop.go:32-33`），turn N 与 turn N+1 之间没有串行化约束 —— turn N 还没收完 finalText 时 turn N+1 就进站了
2. `WorkItem.Directive` 在 `EnsureGoal(sessionID, req.Message)` 时就是用户原话，**没有任何机制把前一轮 finalText 拼接到下一轮 directive**（D2 的 `FoldAssistantOutput` 已经在做 disk-persist 但 session_turn_loop 没有 read 路径）
3. D7 没有 "WaitForTurnCompletion" 语义 —— gateway 收到 turn N+1 消息后立即进 ProcessMessage，不等 turn N 的 goroutine 真正 drain `out` channel

### 1.3 与 panic hotfix 的关系

DM-20260628-002（PR #271）已修复"send on closed channel" panic。但 panic 的根因（stale emit hook）暴露了更底层的 session 生命周期问题：

- panic 之所以能发生，就是因为 multi-turn session 设计上就允许"前后 turn 的 goroutine 并发执行"
- panic 修了，但 "并发允许" 这个根因还在 —— 用户能继续看到 complete 提前 + turn 上下文丢失

所以本次需求不是 panic hotfix 的补丁，而是**正确的 session 生命周期重构**。

## 2. 问题陈述

### 2.1 P0：complete 事件触发时机错误

**当前行为**：`session_turn_loop.go:186-189` emit `complete` 在 `for iter` break 之后立即发生。

```go
for iter := 0; iter < defaultSessionTurnLoopMax; iter++ {
    // ... 处理一个 focus item ...
    if !o.taskManager.Tree().HasOpenWork(sessionID) {
        if _, triggered := workmodel.MaybeRootRollupFallback(...); triggered {
            continue
        }
        break  // ← 跳出循环
    }
}

deliverable := workmodel.ExtractSessionDeliverable(...)
emit(ctx, o.sink, out, &contracts.EngineEvent{
    Type: "complete", Content: deliverable, SessionID: sessionID,  // ← 立即触发
})
```

**期望行为**：`complete` 应在 session **真正 terminal**（所有子 WorkItem 都进入 `TaskStatusCompleted` / `TaskStatusFailed` / `TaskStatusCancelled`，且 `processAutoClose` 已经把 finalText 写入持久化层）之后才 emit。

### 2.2 P0：turn 上下文未注入下一轮 directive

**当前行为**：`ProcessMessage` 在 line 329-331 调用 `EnsureGoal(req.SessionID, req.Message)`，用**用户原话**做 root directive，前一轮 finalText 完全丢失。

```go
if o.taskManager != nil && req.SessionID != "" && strings.TrimSpace(req.Message) != "" && intent.Kind != orchtypes.IntentSkip {
    _, _ = o.taskManager.Tree().EnsureGoal(req.SessionID, req.Message)  // ← 只有用户原话
}
```

**期望行为**：当 session 已有 turn N 的 finalText 落盘时，`EnsureGoal`（或紧随其后的 directive 准备逻辑）应自动拼上 `<prior-output-summary>` 块，让 turn N+1 的 LLM 看到前一轮完整内容。

### 2.3 P0：turn N+1 不等 turn N 收尾就进站

**当前行为**：D1 gateway 收到消息立即 `ProcessMessage` → `RunSessionTurnLoop`，turn N 的 goroutine 还在 drain `out` channel 时 turn N+1 的 goroutine 已经开始了。

**期望行为**：D7 应暴露 WaitForTurnCompletion 语义，让 D1 gateway 在投递 turn N+1 之前等 turn N 的 finalText 真正落地（或保留"turn N 仍在跑"的错误返回，由 gateway 决定是否丢弃 / 排队）。

### 2.4 P1：transcript jsonl 索引缺失

PR #137 已经把 finalText 写入 `transcript jsonl`（per session 落盘），但 session_turn_loop 在 turn 边界读取 transcript 的逻辑还没接上 —— 需要补全"读取最近 N 轮 transcript 作为 prior context"的 helper。

## 3. 验收标准

| ID | 标准 | 优先级 | 可验证方式 |
|----|------|--------|-----------|
| AC1 | `session_turn_loop.go` emit `complete` 前必须等到 `processAutoClose` 完成（`ch` 真正 close，且 finalText 已写入 transcript jsonl） | P0 | 注入 mock LLM 模拟 2 轮子 WorkItem 串行的 session；断言 complete 是最后一个事件，且前序 text/pipeline_round 事件都已 emit |
| AC2 | multi-turn session 中 turn N+1 进站时，若 turn N 仍在跑，D7 必须返回明确的 "turn_in_progress" 错误或把 turn N+1 入队（由 gateway 决定）；**不允许两个 turn 的 goroutine 并发持有同一个 session_id 的 emitFn** | P0 | 集成测试：mock 慢 LLM（10s 出 text）+ 立即发第二条消息；断言第二条消息获得明确错误或排队结果 |
| AC3 | `EnsureGoal` / directive 准备阶段必须读取 transcript jsonl，把最近 N 轮（默认 N=3，可配置）finalText 拼成 `<prior-output-summary>` 注入 directive；**N=0 时行为完全等价于现状**（向后兼容） | P0 | 单测覆盖 N=0/1/2/3 四档；集成测试覆盖跨 turn 实际注入 |
| AC4 | D1 IM 适配器（feishu / cli）在收到 `turn_in_progress` 错误时显示 "上一条消息还在处理中，请稍候"；不返回 error 事件，避免误标 Done 后被覆盖 | P1 | feishu adapter 单测覆盖 turn_in_progress code |
| AC5 | 新增 `internal/layers/orchestration/sessionorchestrator/turn_state.go`：`TurnState` struct 持有 `sessionID → currentTurnGoroutineID` map + mutex；提供 `BeginTurn / EndTurn / WaitTurn` 三个 API | P0 | 单测覆盖并发 Begin/End、Wait 超时 |
| AC6 | `internal/layers/orchestration/sessionorchestrator/turn_state_test.go` 单测覆盖率 ≥ 80%（与现有 sessionorchestrator 包一致） | P0 | `go test -race -cover ./internal/layers/orchestration/sessionorchestrator/` |
| AC7 | LP-5 多轮对话 e2e 集成测试：`sess_e2e_multiturn_v1` —— 发送 turn 1 "review foo" 等到 complete；发送 turn 2 "再 review bar" 验证 turn 2 directive 包含 turn 1 finalText；断言两条 turn 都 emit 了 complete，第二次 complete 的 Content 包含"bar" 关键词 | P0 | `tests/integration/multiturn_session_e2e_test.go` |
| AC8 | 端到端飞书实测：sess_live_multiturn_v1 在 production devrix 上跑"review D2 领域 kernel" + "补充结论"，验证两条消息都有最终结论输出，第二次看到第一次的内容 | P0 | 用户实测验收 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | PR #271（DM-20260628-002）已 merged，提供 emit 不会 panic 的稳定基底 |
| 依赖 | PR #137（devrix-final-text-accumulation-pr137）已 merged，提供 transcript jsonl 持久化 |
| 依赖 | D2 `FoldAssistantOutput` 已能产出 `<prior-output-summary>` 标签 |
| 依赖 | D2 `prior-output-summary` PR #149 iter3 已在 D1 边界剥离该标签 |
| 约束 | 改动必须保持 `feat/devrix-d7-multiturn-session-state` 分支 + squash merge + auto-merge 流程 |
| 约束 | 0 函数签名变化前提下扩展（pure additive）：新增 API 通过新文件 / 新字段，老路径完全不动 |
| 约束 | 22/22 orchestration packages `go test -race` 必须 PASS |
| 约束 | `verify-archive.sh` 12/12 必须 PASS |
| 约束 | LP-1 / LP-2 / LP-5 兼容性不能破坏（baseline 测试结果不能退化） |
| 约束 | **不引入** session 级全局 mutex（会回退到单 goroutine 模型，破坏 OrchestratePath 并行收益） |

## 5. 变更范围

### 5.1 新增
- `internal/layers/orchestration/sessionorchestrator/turn_state.go`：`TurnState` struct + BeginTurn/EndTurn/WaitTurn API
- `internal/layers/orchestration/sessionorchestrator/turn_state_test.go`：单测
- `internal/layers/orchestration/sessionorchestrator/transcript_reader.go`：读 transcript jsonl helper
- `internal/layers/orchestration/sessionorchestrator/transcript_reader_test.go`：单测
- `tests/integration/multiturn_session_e2e_test.go`：LP-5 e2e 测试

### 5.2 修改
- `session_turn_loop.go`：emit `complete` 前必须等 `processAutoClose` 完成（实际上是 `ch` close + transcript 写入，channel close 已隐含 — 这里要做的是确保 finalText 已写入）
- `orchestrator.go` `ProcessMessage`：在 `EnsureGoal` 之前插入 turn_in_progress 校验（通过 `TurnState.WaitTurn`），不合规则返回 `TurnInProgressError`
- `orchestrator.go` `ProcessMessage`：在 `EnsureGoal` 之后插入 transcript 读取 + prior-output-summary 注入 directive
- `internal/layers/communication/feishu/feishu.go`：识别 `turn_in_progress` 错误，走"上一条还在处理中" 文案

### 5.3 不变更
- `ItemPipelineRunner.Run` 内部逻辑（不破坏 D7-S5-A50 包迁移成果）
- `EnsureGoal` 现有语义（pure additive：prior context 注入作为 EnsureGoal 后的 directive 修饰，不改 EnsureGoal 本身）
- `processAutoClose` / `synthesizeVerdict` / Learn 闭环（保持 Phase 7 行为不变）
- 命令行（CLI）适配器：保持现有错误处理（AC4 仅针对 feishu）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 多轮 session 串行化会降低吞吐（单 session 单 goroutine） | 单 session 不能并行处理多个 turn | 这是**正确**的语义（用户体感对齐 chat 模型），不是吞吐问题。AC3 的 `MaxConcurrentTurnsPerSession=1` 是设计意图而非限制 |
| prior-output-summary 注入会增加 prompt size | turn N+1 token 消耗增加 | 默认 N=3，单轮 finalText 上限由 D2 FoldAssistantOutput 的 maxChars 控制（当前 800），3 轮总计 ≤ 2.4K token，影响可忽略 |
| turn_in_progress 错误会被 CLI 用户视为异常 | CLI 用法（脚本调用）退化为必须按序 | CLI 路径保持 ignore（按 §5.3），仅 IM（feishu）显示友好文案 |
| TurnState 全局 map 在大量 session 时内存增长 | devrix 长期运行下 memory leak | map key 是 session_id，session 关闭（`sessiondone` 事件）时显式 delete；helper `cleanupClosedSessions` 定期扫描 |
| transcript jsonl 文件持续增长，无 rotate | 磁盘占满 | 不在本需求 scope，单独提 follow-up：`devrix-d2-transcript-rotate` |
| 改动 ProcessMessage 涉及核心热路径 | 引入回归风险 | 严格走 OpenSpec S3-Gate review + 22/22 orchestration packages `go test -race` 兜底 + 用户飞书实测 |

## 7. P0 T 点预案（待 S3-Gate 后定稿）

| T ID | 描述 |
|------|------|
| D7-S15-A55-T01 | `TurnState` struct 定义 + BeginTurn/EndTurn/WaitTurn API + 并发安全 |
| D7-S15-A55-T02 | `ProcessMessage` 在 `EnsureGoal` 之前插入 `WaitTurn` 校验 + `TurnInProgressError` 定义 |
| D7-S15-A56-T03 | `transcript_reader.go` 读最近 N 轮 finalText helper |
| D7-S15-A56-T04 | `EnsureGoal` 后 directive 注入 `<prior-output-summary>`（可配置 N） |
| D7-S15-A57-T05 | `session_turn_loop.go` complete 事件 emit 时机修正（确保 processAutoClose 已完成） |
| D7-S15-A58-T06 | feishu adapter 识别 `turn_in_progress` 错误 + 友好文案 |
| D7-S15-A59-T07 | LP-5 e2e 集成测试：sess_e2e_multiturn_v1 |

最终 T 层编号以 S3 design 阶段定稿为准。

## 8. 备注：与已有变更的关系

- **DM-20260628-002 (PR #271)**：panic hotfix，不动 session 生命周期 — 本需求是它的**治本**版本
- **devrix-final-text-accumulation-pr137 (PR #137)**：transcript jsonl 写入已就绪，本需求补**读取**路径
- **devrix-prior-output-summary-strip-pr149 iter3 (PR #149)**：D1 边界剥离 `<prior-output-summary>` 标签已就绪，本需求在 D7 边界**写入**该标签（标签在 prompt 中，LLM 输出会被 D1 剥掉防止泄漏到 IM 卡片）
- **devrix-d7-itempipeline-emit-hook (PR #257)**：emit 链路补齐，本需求扩展 emit 时机约束
- **devrix-d7-six-s-simplification (DM-20260626-001)**：v6.0.0 域升级 14S → 6S，本需求新建 D7-S15 在其上