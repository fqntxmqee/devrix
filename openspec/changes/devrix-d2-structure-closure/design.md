# Design: D2 v2.2 Structure 终态

**Change ID:** devrix-d2-structure-closure  
**Demand ID:** DM-20260619-007  
**Status:** S3_Draft  
**Methodology:** DSAFT Refactoring Playbook §6 双锚点对齐

---

## 1. 架构目标

在 **不破坏 T ID、不改变 D7/D2 跨域契约签名** 的前提下：

1. **运行锚点** — 生产 Prepare/Persist/ToolRound 经 scenario orchestrator（或 persist F 层），消除 facade 双轨
2. **物理锚点** — 终态目录与 `demand.md` §4 一致
3. **规格锚点** — S5 后 `openspec/specs/d2-context-engine/` 与仓库 grep 一致

---

## 2. Decision 记录

### Decision: 编排收敛策略（P1）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A: facade 一次性删，全切 orchestrator | 一步到位 | PrepareOrchestrator 缺 span/emit/fork/attachment；风险极高 |
| **B: 分 slice 下沉 F 层，再 wired orchestrator** | 每 PR <400 行；T 连续绿 | 多 PR |
| C: 只改文档 | 零风险 | 双轨依旧 |

**选择:** B  
**理由:** 当前 `prepare.Orchestrator` 是简化版（无 RepairToolChain、无 CompressPerTurn、无 worker fork）；直接替换会丢行为  
**影响:** P1 顺序：S17-A04 commit → S15 ports → S17 persist wired → facade thin → S18 对齐

### Decision: Prepare 端口适配器位置

**选择:** `prepare/adapters/` 包（或 `prepare/memory_loader.go` 等同包文件），**不**放在 facade  
**理由:** 端口实现属于 S15；facade 仅组装 `PrepareDeps` 并调用 `Orchestrator.Prepare`  
**影响:** `SessionLoader` 包装 `memory.Manager` + snapshot 恢复 emit 可选 hook

### Decision: facade 终态命名

**选择:** P5 重命名为 `legacy/`；P1–P4 保持 `facade/` 减少 churn  
**理由:** import alias 已稳定；rename 与 P3 git mv 合并  
**影响:** `demand.md` §4 终态仍写 `legacy/`

### Decision: toolrunner 重命名

**选择:** P3 `enforce/toolrunner/` → `enforce/tools/`  
**理由:** scenario slug，非技术分层名  
**影响:** 全仓库 import 机械替换；layer-lint 更新

### Decision: T 层

**选择:** T ID 不变；新增 layout 守卫 T（待登记 `D2-STRUCT-T01`）  
**理由:** Playbook 原则 3

---

## 3. P1 分 slice 实施顺序

```text
P1-a  persist/commit.go          ← S17-A04（本 PR）
P1-b  prepare/adapters/*.go       ← SessionLoader / Compressor 等
P1-c  prepare/orchestrator 增强   ← RepairToolChain / CompressPerTurn / span hooks
P1-d  facade → PrepareOrchestrator
P1-e  persist/orchestrator + finalizeTurn 收敛
P1-f  turn_adapter 去 duplicate
```

---

## 4. persist/commit 设计（P1-a）

### 4.1 职责

`AppendAndTrimMessages` = **S17-A04 CommitWindow**：将 D7 turn 消息 batch 写入 SessionContext 并 Trim。

### 4.2 端口

```go
// persist/contracts.go
type MessageStore interface {
    Get(sessionID string) (*types.SessionContext, bool)
    AppendFullMessage(sc *types.SessionContext, msg types.Message)
    TrimMessages(sc *types.SessionContext)
}

type SessionBootstrap func(sessionID string) (*types.SessionContext, error)

type CommitDeps struct {
    Store     MessageStore
    Bootstrap SessionBootstrap // optional lazy init (D7 first-write)
}
```

### 4.3 调用关系

```text
turn_adapter.PersistTurn
  → ContextEngine.AppendAndTrimMessages (alias, 保留)
    → facade.ContextEngine.AppendAndTrimMessages
      → persist.AppendAndTrimMessages(deps, ...)
```

---

## 5. 双锚点验收 grep

S5 前必须满足：

```bash
# 根生产文件仅 2–3 个
ls internal/layers/contextengine/*.go | grep -v _test | wc -l  # → 2 或 3（过渡）

# 无 engine_persist.go 在根或 facade 外
! test -f internal/layers/contextengine/engine_persist.go

# a-registry S17-A04 指向 persist/commit.go
grep 'persist/commit.go' openspec/specs/d2-context-engine/a-registry.md
```

---

## 6. 风险与缓解

| 风险 | 缓解 |
|------|------|
| PrepareOrchestrator 行为缺口 | P1-b/c 补全后再 wired；单测对比 facade golden |
| import churn (tools/) | 独立 PR；go test ./... |
| bootstrap 边界 | adapter 仅 wiring；逻辑留 D2 |

---

## 7. S3-Gate 检查清单

- [x] 终态目录树（demand.md §4）
- [x] P1 slice 顺序与 Decision 记录
- [x] T 策略
- [ ] Owner 确认 P1-b 前是否扩 PrepareOrchestrator span/emit（建议：P1-c 做）

**Gate 结论:** Approved for S4 P1-a（persist/commit）；P1-d 待 P1-c 完成。
