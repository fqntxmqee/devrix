# Design: D2 spec 退役标记完整性

**Change ID:** devrix-spec-sync-d2-layer-delta-soften
**Demand ID:** DM-20260619-004

> docs-only change，D2 spec 两份文档的逐文件变更映射。SoT 不动：D2 域代码（`internal/layers/contextengine/**`）+ D2 spec.md §18 LEGACY 标记（已存在，保持）。

---

## 1. 设计原则

1. **三层一致性**：D2 spec.md §18 LEGACY + layer-delta.md Requirement + d7-boundary.md 契约 三层标记需对齐
2. **保留回滚兼容**：D2 Scenarios 不删除（per spec.md §18 声明）
3. **措辞软化而非删除**：layer-delta.md Requirement 加 DEPRECATED 注脚而非删除
4. **代码锚点 grep 验证**：所有引用的 `xxx.go:Function` 必须能 `git grep` 命中

## 2. 文件级变更映射

### 2.1 W1: `openspec/specs/d2-context-engine/layer-delta.md`

**原 line 12-14**：

```markdown
### Requirement: QueryLoop Primary Runtime

When `context_engine.query_loop.enabled=true` (default since DM-20260611-004),
`ContextEngine.Process` MUST route all LLM↔Tool rounds through `query.Loop.Run`
instead of the retired PEV engine.
```

**新内容**：

```markdown
### Requirement: QueryLoop Default Runtime ⚠️ DEPRECATED in `loopFirst=false` path

> **DEPRECATED (2026-06-17, DM-20260617-001)**: canonical 主路径已迁至 D7-S2-A06
> `turn.RunTurnLoop`（`internal/layers/orchestration/turn/orchestrator.go`）。
> `loopFirst=true` 是默认（Default），`loopFirst=false` 路径下本 Requirement **DEPRECATED**。
> 本 Requirement 与 D2-S10 所有 Scenario **保留** 用于紧急回滚兜底，新能力**不得**依赖本路径。

When `context_engine.query_loop.enabled=true` (default since DM-20260611-004)
AND `loopFirst=true` (default since DM-20260614-020), `ContextEngine.Process`
routes LLM↔Tool rounds through `query.Loop.Run`. **Default 路径**——但 canonical
主路径是 D7-S2-A06 `turn.RunTurnLoop`。
```

### 2.2 W2: `openspec/specs/d2-context-engine/d7-boundary.md`

**原 §79 表格**：

```markdown
| `LoopHooks` | `query/loop.go` | D7 注入 | D2 Loop |
```

**新 §79 表格**（加 DEPRECATED 状态列）：

```markdown
| `LoopHooks` | `query/loop.go` | D7 注入 | D2 Loop | **DEPRECATED** (`loopFirst=false`; canonical=D7-S2-A06 RunTurnLoop) |
```

**§4 契约表** 加 Loop.Run 契约的 DEPRECATED 标注：

```markdown
| 契约 | D2 实现 | D7 期望 | 状态 |
|------|---------|---------|------|
| `Loop.Run` | `query/loop.go` | (fallback only) | **DEPRECATED** (2026-06-17 DM-001; `loopFirst=false` 路径；canonical=D7-S2-A06 RunTurnLoop) |
| `LoopHooks` | `query/loop.go` | D7 注入 | **DEPRECATED** (同上) |
| `IContextEngine.Process` | `engine.go` | D7 turn.RunTurn 调用 | ACTIVE |
| ... | ... | ... | ... |
```

## 3. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 措辞软化不彻底 | W1/W2 两文档 Last Updated 统一刷至 2026-06-19 + grep 验证无 "MUST route all" 字样 |
| 误删 Scenarios | D2 Scenarios 不删（per spec.md §18 回滚兼容声明） |
| 与 spec.md §18 矛盾 | W1 措辞与 §18 LEGACY 标记一致（都引用 DM-20260617-001） |

## 4. 不变更（边界声明）

- `internal/layers/contextengine/**` 全部代码
- `openspec/specs/d2-context-engine/spec.md` §18 LEGACY 标记（已存在，保持）
- D2 Scenarios（保留回滚兼容）
- D-S 编号体系（D2-S/A/F/T）
- t-registry.md
