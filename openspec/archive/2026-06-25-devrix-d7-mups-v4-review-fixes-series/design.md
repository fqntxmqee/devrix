# Design: D7 复审 review fixes 系列

**Change ID:** devrix-d7-mups-v4-review-fixes-series
**Demand IDs:** DM-20260625-005 + DM-20260625-006
**Status:** S4-Implemented

---

## 1. 修复总览

本 series 包含 **14 个独立 fix**（9 silent failure + 1 dead code + 3 god function + 1 regression guard test），按 2 个 DM 拆 2 个 PR：

| DM | 路径 | PR | Fix 数 | 文件 | 行数 |
|----|------|----|--------|------|------|
| DM-20260625-005 | hotfix | #205 | 10 | 10 | +85/-26 |
| DM-20260625-006 | 标准 S1-S6 | #206 | 3 (god function) | 1 (纯 H2 forward-port) | +123/-79 |

**PR #206 实际 forward-port 范围**：原 4 文件改动 (H1, H2, C4) 因 master 已 H1 拆分（DM-20260625-012 D3）+ master 已无 shadowClassifier 字段 → 只 forward-port H2 (`escape/arbitrator.go`)。

---

## 2. 关键修复的设计决策

### 2.1 DM-20260625-005 — 错误吞咽统一模式

**统一模式**：
```go
// 之前（silent failure）
_, _ = x.Method(...)

// 之后（结构化 warn）
if err := x.Method(...); err != nil {
    slog.Warn("d7: x.Method failed; ...", 
        "session_id", sessionID, 
        "task_id", taskID, 
        "worker_id", workerID,
        "error", err)
}
```

**结构化字段选择**：
- `session_id` — 跨域追踪 ID
- `task_id` / `worker_id` — 5 节点管道标识
- `error` — 错误详情（slog 自动处理 wrapping）
- `command_kind` / `plan_mode` / `escape_kind` — 场景特定字段

**特殊处理（Problem 7 planMode.Enter）**：
- 不是 slog.Warn，而是把错误**返回给用户**（用户感知）
- 失败时 `return fmt.Errorf("failed to enter plan mode: %w", err)`
- 配套测试断言改为 "Failed to enter plan mode"

**特殊处理（Problem 6 os.Remove）**：
- 加 `os.IsNotExist(err)` 过滤（不存在的文件不警告）
- 避免 "os.Remove: file does not exist" 噪声

**特殊处理（Problem 8 os.MkdirAll）**：
- 失败时 `outputDir = ""` 置空
- 避免后续 `os.WriteFile(outputDir/...)` 连锁失败

### 2.2 DM-20260625-006 — god function 拆分模式

**统一拆分原则**：
1. **phase helper 命名**：动词短语（`tryResumeShortCircuit` / `classifyIntent` / `runOneIteration`）
2. **carrier struct 引入**：per-loop / per-iteration mutable state 集中管理
3. **错误传递**：phase helper 返回 error，主函数聚合 + 决策
4. **Span 透传**：sessionSpan 在主函数创建，phase helper 通过 ctx 访问

#### 2.2.1 H2 LLMArbitrator.Arbitrate 拆分细节

**原结构（132 行）**：
```
Arbitrate(ctx) {
    spawn goroutine (invokeLLM)
    3-way select (ctx.Done / llmResult / timer)
    错误转义 (ctx_cancelled / llm_timeout_5s)
    parse result with retry (max 2)
    parse fail → format-hint 重试
    parse success → invokeGenerateBounded
    validate action (Continue / Exit / invalid)
    build EscapeDecision
}
```

**新结构（21 行）**：
```
Arbitrate(ctx) (*EscapeDecision, error) {
    raw, err := invokeLLMWithTimeout(ctx)
    if err != nil { return nil, err }
    parsed, err := parseWithRetry(ctx, raw)
    if err != nil { return nil, err }
    result, err := invokeGenerateBounded(ctx, parsed)
    if err != nil { return nil, err }
    return validateActionAndBuild(result)
}
```

**phase helper 细节**：

- `invokeLLMWithTimeout(ctx)`:
  - goroutine + 3-way select (ctx.Done / llmResult / timer)
  - 错误转义通过 `*EscapeDecision` 指针返回
  - `ctx_cancelled` / `llm_timeout_5s` 区分

- `parseWithRetry(ctx, raw)`:
  - `for i := 0; i < 2; i++` retry
  - 失败时带 format hint 重试
  - 成功时返回 parsed struct

- `invokeGenerateBounded(ctx, parsed)`:
  - 单次 `Generate` bounded by ctx
  - 返回 `*GenerationResult`

- `validateActionAndBuild(result)`:
  - `Continue` → build Continue decision
  - `Exit` → build ForceExit decision（通过 `buildForceExit` 工厂）
  - `invalid` → build invalid decision

**buildForceExit 工厂**：
```go
func buildForceExit(reason string) *EscapeDecision {
    return &EscapeDecision{
        Kind:    EscapeForceExit,
        Reason:  reason,
        Audit:   1,
    }
}
```

保留原 reason 区分：`ctx_cancelled` / `llm_timeout_5s` / `llm_stuck_force_exit`。

---

## 3. 行为不变性保证

### 3.1 DM-20260625-005
- 9 处 slog.Warn 只是增加日志输出，**不改变**控制流
- Problem 7 修复是修复**bug**（让用户感知失败），不算行为变化
- Problem 6 / 8 修复是**健壮性增强**（过滤 / 置空），不算行为变化

### 3.2 DM-20260625-006 (H2)
- 所有 phase helper 内 emit 的 error 事件与原代码一致
- 所有 Span 属性 key/value 不变
- 所有 decision routing (Continue / EscalateToRule / ForceExit) 不变
- 所有 audit level (0/1/2) 不变
- 所有 reason 区分不变

### 3.3 测试覆盖
- 22/22 orchestration packages `go test -race -count=1` PASS
- `TestCommandHandler_Handle_PlanCommand` 改为断言 "Failed to enter plan mode"
- 包含 race detection 测试（escape/loop_depth_tracker atomic.Pointer fix 已通过）

---

## 4. Re-base 实战记录

### 4.1 PR #205 (DM-20260625-005)
- **初始问题**：分支基于旧 master 缺 `atomic.Pointer[func() time.Time]` race fix
- **Rebase 命令**：`git rebase origin/master`
- **冲突点**：`internal/layers/orchestration/executionflow/hub/hub.go`
  - 原 commit 写 `h.tasks.SetOwner(...)`，编译错误（`*workmodel.TaskManager` 无 `SetOwner` 方法）
  - 修复：先 `tree := h.tasks.Tree()` 再 `tree.SetOwner(...)`
- **CI 绕过 LP-1 flake**：空 commit `debf4e3` re-trigger CI，unit tests pass

### 4.2 PR #206 (DM-20260625-006)
- **初始问题**：4 文件改动 (H1, H2, C4) 中 H1 已被 master 处理 (DM-20260625-012) + C4 缺 shadowClassifier 字段
- **策略**：只 forward-port H2 (`escape/arbitrator.go`)，其他 skip via conflict resolution
- **Rebase 结果**：commit `60002b9` 仅 `escape/arbitrator.go 1 file +123/-79`
- **CI 自动 pass**：rebase 后本地 22/22 PASS，GitHub CI 直接 SUCCESS

---

## 5. 后续 Follow-up（Out of Scope）

- **M1-M20 Medium 修复**：留给 cleanup change（按 `devrix-d7-mups-v4-review-fixes` (DM-20260625-002) Out of Scope 段一致）
- **L1-L14 Low 修复**：暂不立项
- **C4/H1 完整 forward-port**：C4 需 master 恢复 `shadowClassifier` 字段，H1 已由 master 完成（无需 forward-port）
