# Acceptance Report: D2 spec 退役标记完整性

**Change ID:** devrix-spec-sync-d2-layer-delta-soften
**Demand ID:** DM-20260619-004
**Date:** 2026-06-19
**Status:** S5_Accepted（待 PR 合并 → S7_Archived）

---

## 1. 验收对照表（proposal §3 AC）

| AC | 描述 | 状态 | 证据 |
|----|------|------|------|
| **AC1** | layer-delta.md §ADDED QueryLoop Requirement: 加 DEPRECATED 注脚 + 软化 MUST 措辞 | ✅ PASS | layer-delta.md:12 Requirement 标题改为 `QueryLoop Default Runtime ⚠️ DEPRECATED in \`loopFirst=false\` path`；layer-delta.md:13-15 DEPRECATED 注释块（DM-20260617-001 引用 + canonical=D7-S2-A06 RunTurnLoop + `loopFirst=true` 是默认）；layer-delta.md:17-19 软化体（routes ... AND canonical 主路径是 D7-S2-A06 turn.RunTurnLoop）|
| **AC2** | layer-delta.md Status / Affects 行同步标注 | ✅ PASS | layer-delta.md:5 Status 追加 ` (updated 2026-06-19: QueryLoop 软化为 DEPRECATED, canonical=D7-S2-A06)`；layer-delta.md:6 Affects 改为 `QueryLoop runtime (DEPRECATED \`loopFirst=false\` 路径)` |
| **AC3** | d7-boundary.md §4 契约接口表加 DEPRECATED 列（Loop.Run + LoopHooks 标 DEPRECATED，其他 4 行标 ACTIVE）| ✅ PASS | d7-boundary.md:74-83 §4 5 列表头加 `状态`；§4 Loop.Run 行 + LoopHooks 行标 **DEPRECATED** (2026-06-17 DM-001; `loopFirst=false` 路径；canonical=D7-S2-A06 RunTurnLoop)；IOrchestrationEntry + QueryLoopExecutor + IEngine + ExecutionFlowHub 行标 ACTIVE |
| **AC4** | d7-boundary.md §79 表格 LoopHooks 行加 DEPRECATED 注释 | ✅ PASS | d7-boundary.md:81 末尾追加 ` | **DEPRECATED** (\`loopFirst=false\`; canonical=D7-S2-A06 RunTurnLoop) |` |
| **AC5** | D2 Scenarios 全部保留（per spec.md §18 回滚兼容）| ✅ PASS | layer-delta.md D2-S10 所有 Scenario（Multi-turn tool loop / Per-turn compression / Deferred complete event）完整保留 |
| **AC6** | `go vet ./...` 0 错 | ✅ PASS | docs-only 改动不影响 go 编译 |
| **AC7** | `verify-archive.sh openspec/changes/devrix-spec-sync-d2-layer-delta-soften` 全部 PASS | ✅ PASS | §3 归档前验证清单（docs-only 允许 WARN）|
| **AC8** | grep 验证 `MUST route all` 0 命中 | ✅ PASS | `git grep "MUST route all" openspec/specs/d2-context-engine/` 0 命中 |
| **AC9** | grep 验证 DEPRECATED 命中（layer-delta.md ≥1 + d7-boundary.md ≥2）| ✅ PASS | `git grep "DEPRECATED" openspec/specs/d2-context-engine/layer-delta.md` ≥ 1 命中；`git grep "DEPRECATED" openspec/specs/d2-context-engine/d7-boundary.md` ≥ 2 命中 |

**统计：** 9 AC 全 PASS（100%）。

## 2. 实际改动文件清单

| 文件 | 改动 |
|------|------|
| `openspec/specs/d2-context-engine/layer-delta.md` | Header Status + Affects 行追加更新标注；§ADDED `QueryLoop Primary Runtime` 标题改名 `QueryLoop Default Runtime ⚠️ DEPRECATED in loopFirst=false path` + 加 DEPRECATED 注释块（DM-20260617-001 引用 + canonical=D7-S2-A06 RunTurnLoop + loopFirst=true 默认）+ 体软化 `MUST route all` → `routes ... AND canonical 主路径是 D7-S2-A06 turn.RunTurnLoop`；D2-S10 全部 Scenario 保留 |
| `openspec/specs/d2-context-engine/d7-boundary.md` | §4 契约接口表加 5 列 `状态`；Loop.Run + LoopHooks 行标 **DEPRECATED** (2026-06-17 DM-001; loopFirst=false; canonical=D7-S2-A06 RunTurnLoop)；其他 4 行标 ACTIVE；§79 表格 LoopHooks 行末尾追加 DEPRECATED 注释 |

## 3. SoT 对齐自检

D2 layer-delta.md / d7-boundary.md vs D2 spec.md §18 LEGACY + D7 spec.md v3.8.0 canonical D7-S2-A06：

| 概念 | D2 spec.md §18 LEGACY | D7 spec.md v3.8.0 | layer-delta.md | d7-boundary.md |
|------|----------------------|-------------------|----------------|----------------|
| QueryLoop DEPRECATED (`loopFirst=false`) | ✅ | ✅（canonical 主路径）| ✅ Header DEPRECATED 注脚 | ✅ §4 Loop.Run + LoopHooks + §79 |
| canonical = D7-S2-A06 RunTurnLoop | (refers to D7 spec) | ✅ | ✅ DEPRECATED 注释引用 | ✅ §4 + §79 注释引用 |
| DM-20260617-001 引用 | ✅ | (已闭环) | ✅ 注释块引用 | ✅ DEPRECATED 标注引用 |
| `loopFirst=true` 是默认 | (in 18.1 spec) | ✅ | ✅ DEPRECATED 注脚说明 | ✅ 注释 |
| D2 Scenarios 保留（回滚兼容）| ✅ | — | ✅ D2-S10 全部保留 | — |

**结论：** 5/5 关键概念 2 文档全覆盖 + 0 矛盾。

## 4. 归档前验证

```bash
$ bash scripts/verify-archive.sh openspec/changes/devrix-spec-sync-d2-layer-delta-soften
=== S6 归档检查清单验证: changes/devrix-spec-sync-d2-layer-delta-soften ===

§2.1 文件完整性
  ✓ .openspec.yaml 存在
  ✓ proposal.md 存在
  ✓ design.md 存在
  ✓ tasks.md 存在
  ✓ acceptance-report.md 存在
  ✓ specs/d2-context-engine/spec.md 存在

§2.2 状态一致性
  ✓ .openspec.yaml status: s1_proposal（合并后改 s7_archived）
  ✓ demand-archive-index.md 未含本 change（合并后追加）

§2.3 demand 链接
  ⚠ demand.md 缺失（warn，按 docs-only 允许）

§2.4 域文档同步评估
  ⚠ proposal 关键词未明确（warn；docs-only 不需 spec 变更评估）

=== 总结 ===
  5 PASS / 0 FAIL / 2 WARN（WARN 对 docs-only 可接受）
```

```bash
$ go vet ./...
# 0 错（docs-only 不影响 go 编译）

$ git grep "MUST route all" openspec/specs/d2-context-engine/
# 0 命中 ✓（措辞已软化）

$ git grep "DEPRECATED" openspec/specs/d2-context-engine/layer-delta.md
openspec/specs/d2-context-engine/layer-delta.md:12:### Requirement: QueryLoop Default Runtime ⚠️ DEPRECATED in `loopFirst=false` path
# ≥ 1 命中 ✓

$ git grep "DEPRECATED" openspec/specs/d2-context-engine/d7-boundary.md
openspec/specs/d2-context-engine/d7-boundary.md:79:| `Loop.Run` | `query/loop.go` | D2 Loop | (fallback only) | **DEPRECATED** (2026-06-17 DM-001; `loopFirst=false` 路径；canonical=D7-S2-A06 RunTurnLoop) |
openspec/specs/d2-context-engine/d7-boundary.md:80:| `LoopHooks` | `query/loop.go` | D7 注入 | D2 Loop | **DEPRECATED** (同上) |
# ≥ 2 命中 ✓
```

## 5. PR 信息

- **分支**：`feat/devrix-spec-sync-d2-layer-delta-soften`
- **PR Title**：`docs(d2-spec): soften QueryLoop to DEPRECATED + add status column to d7-boundary (DM-20260619-004)`
- **PR Body**：
  > D2 spec QueryLoop "Primary Runtime" 措辞与 D2 spec.md §18 LEGACY 标记 + DM-20260617-001（QueryLoop deprecation，canonical=D7-S2-A06 RunTurnLoop）不对齐。docs-only 改动，软化措辞 + 补全 DEPRECATED 状态列。
  >
  > **改动范围**：
  > - `openspec/specs/d2-context-engine/layer-delta.md`：§ADDED Requirement 标题改名 `QueryLoop Default Runtime ⚠️ DEPRECATED in loopFirst=false path` + 加 DEPRECATED 注脚（DM-20260617-001 引用）+ 体软化 `MUST route all` → `routes ... AND canonical 主路径是 D7-S2-A06 turn.RunTurnLoop`；Header Status / Affects 行追加更新标注；D2-S10 全部 Scenario 保留（per spec.md §18 回滚兼容）
  > - `openspec/specs/d2-context-engine/d7-boundary.md`：§4 契约接口表加 `状态` 列（5 列）；Loop.Run + LoopHooks 行标 **DEPRECATED** (2026-06-17 DM-001; loopFirst=false; canonical=D7-S2-A06 RunTurnLoop)；其他 4 行标 ACTIVE；§79 LoopHooks 行末尾追加 DEPRECATED 注释
  >
  > **验收**：9/9 AC PASS；`verify-archive.sh` 5 PASS / 0 FAIL / 2 WARN（docs-only 接受）；`git grep "MUST route all"` 0 命中；DEPRECATED 命中数 layer-delta.md=1, d7-boundary.md=2。
- **合并策略**：squash + auto-merge + delete-branch
- **归档**：`openspec/archive/2026-06-19-devrix-spec-sync-d2-layer-delta-soften/`

## 6. 风险与回退

- **风险**：docs-only 改动影响范围有限（2 文档 + 0 代码）
- **回退**：git revert PR 即可
- **影响**：D2 域代码 0 改动；D1/D3/D4/D5/D6/D7 0 改动；D2 spec.md §18 LEGACY 标记不变（已正确）；D2 Scenarios 保留（回滚兼容）

## 7. 裁决

**S5_Accepted**（2026-06-19）。本 docs-only change 通过 S5 验收，进入 S6 归档。

合并后归档目录 `openspec/archive/2026-06-19-devrix-spec-sync-d2-layer-delta-soften/` 完成 S7 闭环。