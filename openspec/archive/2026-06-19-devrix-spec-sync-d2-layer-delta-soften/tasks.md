# Tasks: D2 spec 退役标记完整性

**Change ID:** devrix-spec-sync-d2-layer-delta-soften
**Demand ID:** DM-20260619-004

---

## W1: 软化 `openspec/specs/d2-context-engine/layer-delta.md` QueryLoop 措辞

**目标**：QueryLoop "Primary Runtime" 加 DEPRECATED 注脚 + 软化 MUST 措辞

- [ ] §Status 行：追加 ` (updated 2026-06-19: QueryLoop 软化为 DEPRECATED, canonical=D7-S2-A06)`
- [ ] §Affects 行：`QueryLoop runtime` → `QueryLoop runtime (DEPRECATED \`loopFirst=false\` 路径)`
- [ ] §ADDED Requirement: `QueryLoop Primary Runtime` → `QueryLoop Default Runtime ⚠️ DEPRECATED in \`loopFirst=false\` path`
- [ ] §Requirement 体：MUST route all → routes ... AND canonical 主路径是 D7-S2-A06 turn.RunTurnLoop
- [ ] 保留所有 D2-S10 Scenario（per spec.md §18 回滚兼容声明）

## W2: `openspec/specs/d2-context-engine/d7-boundary.md` 加 DEPRECATED 列

**目标**：§4 契约表 + §79 LoopHooks 加 DEPRECATED 状态标注

- [ ] §4 契约接口表：加 `状态` 列
- [ ] §4 契约表：`Loop.Run` 行（fallback only）标 **DEPRECATED** (2026-06-17 DM-001; `loopFirst=false` 路径；canonical=D7-S2-A06 RunTurnLoop)
- [ ] §4 契约表：`LoopHooks` 行标 **DEPRECATED** (同上)
- [ ] §4 契约表：其他 4 行（IOrchestrationEntry / QueryLoopExecutor / IEngine / ExecutionFlowHub）标 ACTIVE
- [ ] §79 表格：`LoopHooks | query/loop.go | D7 注入 | D2 Loop` 末尾追加 ` | **DEPRECATED** (\`loopFirst=false\`; canonical=D7-S2-A06 RunTurnLoop) |`

## W3: 验证 + PR + 归档

- [ ] `bash scripts/verify-archive.sh openspec/changes/devrix-spec-sync-d2-layer-delta-soften` 全部 PASS（docs-only 允许 WARN）
- [ ] `go vet ./...` 0 错
- [ ] `git grep "MUST route all" openspec/specs/d2-context-engine/` 0 命中（措辞已软化）
- [ ] `git grep "DEPRECATED" openspec/specs/d2-context-engine/layer-delta.md` ≥ 1 命中
- [ ] `git grep "DEPRECATED" openspec/specs/d2-context-engine/d7-boundary.md` ≥ 2 命中
- [ ] commit: `docs(d2-spec): soften QueryLoop to DEPRECATED + add status column to d7-boundary (DM-20260619-004)`
- [ ] push: `git push -u origin feat/devrix-spec-sync-d2-layer-delta-soften`
- [ ] `gh pr create --title "..." --body "..." --base master`
- [ ] `gh pr merge --auto --squash --delete-branch`（Devrix PR Auto-Merge 偏好）
- [ ] 合并后：`mv openspec/changes/devrix-spec-sync-d2-layer-delta-soften openspec/archive/2026-06-19-devrix-spec-sync-d2-layer-delta-soften/`
- [ ] 合并后：`.openspec.yaml` `status: s7_archived`
- [ ] 合并后：`openspec/demand-archive-index.md` 追加新行（DM-20260619-004 → archive）
- [ ] 合并后：`git pull` 验证工作区干净

## 依赖关系

```
W1 ─┐
W2 ─┴─→ W3
```

W1/W2 互相独立可并行；W3 依赖前两者全部完成。

## 不变更（边界声明）

- `internal/layers/contextengine/**` 全部代码
- `openspec/specs/d2-context-engine/spec.md` §18 LEGACY 标记（已存在，保持）
- D2 Scenarios（保留回滚兼容）
- D-S 编号体系（D2-S/A/F/T）
- t-registry.md