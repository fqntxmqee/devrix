# Acceptance Report: devrix-d2-structure-closure

**Change ID:** devrix-d2-structure-closure
**Demand ID:** DM-20260619-007
**Result:** ACCEPTED
**Date:** 2026-06-19
**Reviewer:** devrix-team

## Summary

D2 v2.2 Structure 终态（DM-20260619-007）闭环。Scenario 编排（`prepare.Orchestrator` + `persist.Orchestrator`）成为生产唯一 SoT；`enforce/tools/` + `enforce/sandbox/` 物理归位；memory 读写分离（Recall→S15 / Store→S17）；`facade/` → `legacy/` 退役；规格双锚点闭合（`d2-domain.md` v8.2.0 + `code-layout.md` v1.12.0 + `layering.md` v4.7.0）。9 提交合入 `feat/d2-structure-p1e-persist-orchestrator` 分支（P1-a 已 S3-Gate Approved，P1-b/c/d/e/f + P2/P3/P4/P5 + P6 全部 IMPLEMENTED）。

## S5 Gate Criteria

### Acceptance Criteria

| AC | 描述 | 状态 |
|----|------|------|
| **AC-P0-1** | 生产 Prepare 走 `prepare.Orchestrator.Prepare`（不再 inline facade） | ✅ P1-d |
| **AC-P0-2** | 生产 Persist 走 `persist.Orchestrator` + `commitWindow`（不再 inline facade） | ✅ P1-e |
| **AC-P0-3** | `legacy.ContextEngine.Process()` 加 `// Deprecated:` + `slog.Warn` 运行时告警 | ✅ P5 |
| **AC-P0-4** | layout 守卫全 7 项 IMPLEMENTED（D2-STRUCT-T01..T07） | ✅ P2/P3/P4/P5 |
| **AC-P0-5** | `d2-domain.md` v8.1.0 → v8.2.0 终态物理路径 + v8.2 修订记录 | ✅ P6 |
| **AC-P0-6** | `code-layout.md` v1.11.1 → v1.12.0 §4.3 D2 表 + 深度规则 + 守卫 T 列表 | ✅ P6 |
| **AC-P1-1** | `enforce/tools/` 重命名（49 production + 21 test）| ✅ P3-T2 |
| **AC-P1-2** | `enforce/sandbox/` 物理迁入 | ✅ P3-T1 |
| **AC-P1-3** | `enforce/orchestrator.go`（92 行 stub）删除 | ✅ P3-T4 |
| **AC-P1-4** | `prepare/memory/longterm.go` 拆分 `recall.go`（S15）+ `persist/memory/store.go`（S17） | ✅ P4 |
| **AC-P1-5** | `MemoryEntry` / `LongTermRecaller` / `LongTermStore` 提升至 `shared/types` + `shared/contracts` | ✅ P4 |
| **AC-P1-6** | `facade/` → `legacy/` 物理迁移 + 包名重命名 | ✅ P5 |
| **AC-P1-7** | T07 layout guard：禁止新增 `legacy.Process()` 生产引用（allowlist 8 个 caller） | ✅ P5 |
| **AC-P1-8** | `layering.md` v4.6.0 → v4.7.0 D2 终态树 | ✅ P6 |
| **AC-P1-9** | `layer-delta.md` v8.0.0 → v8.2.0 章节 + 3 个新 requirement 块 | ✅ P6 |
| **AC-P1-10** | `span-registry.md` v2.2.0 → v2.3.0 路径同步 | ✅ P6 |
| **AC-P1-11** | `code-atlas.md` `worker_dir_sandbox` 路径修正 | ✅ P6 |
| **AC-P1-12** | `a-registry.md` S15-A02 + S17-A03 Code Location 与物理路径一致 | ✅ P4 + P6 |
| **AC-P1-13** | `t-registry.md` D2-STRUCT-T01..T07 全部 IMPLEMENTED，DM-20260619-007 关联 | ✅ P3/P4/P5/P6 |
| **AC-P1-14** | `integration test golden` AC-P1-6：单测 `TestPrepareOrchestrator_FinalizeTurn` 覆盖 P1-d + P1-e 路径 | ✅ P1-d |

### 门禁验证

| 检查项 | 命令 | 结果 |
|--------|------|------|
| go vet | `go vet ./...` | ✅ PASS（0 错） |
| go build | `go build ./...` | ✅ PASS |
| D2 单测 | `go test -race -count=1 ./internal/layers/contextengine/...` | ✅ PASS |
| Bootstrap 单测 | `go test -race -count=1 ./internal/bootstrap/...` | ✅ PASS |
| Layer-lint | `go test ./internal/lint/layer/...` | ✅ PASS（T01-T07 全绿） |
| D2→D3 import ban | `TestD2_D3Ban` | ✅ PASS（CI 硬阻断未触发） |
| cyclic import | `D2-STRUCT-T04` | ✅ PASS（prepare/memory ↔ persist/memory 解耦） |
| no package toolrunner | `D2-STRUCT-T03` | ✅ PASS（grep 0 命中） |
| no legacy.Process new caller | `D2-STRUCT-T07` | ✅ PASS（allowlist 8 项，0 新增） |
| D2 集成（turn） | `go test -tags='integration && cross' ./tests/integration/ -run 'Turn' -count=1` | ✅ PASS |

### 文档同步

| 文档 | 版本 | 状态 |
|------|------|------|
| `d2-domain.md` | 8.2.0 | ✅ P6（含 v8.2 修订记录 + 物理路径映射 + 实现状态） |
| `code-layout.md` | 1.12.0 | ✅ P6（§4.3 D2 表 + 深度规则 + D2-STRUCT-T01..T07 守卫列表） |
| `layering.md` | 4.7.0 | ✅ P6（contextengine/ 终态树） |
| `layer-delta.md` | (v8.0.0 → v8.2.0 章节) | ✅ P6（P3/P4/P5 requirement + scenario） |
| `code-atlas.md` | 1.2.0 | ✅ P6（sandbox 路径修正） |
| `span-registry.md` | 2.3.0 | ✅ P6（v2.2 路径同步） |
| `a-registry.md` | 3.x | ✅ P4 + P6（Recall / Store Code Location 同步） |
| `t-registry.md` | (D2-STRUCT-T01..T07) | ✅ P3/P4/P5/P6 |

## Implementation Statistics

- **9 提交** 合入 `feat/d2-structure-p1e-persist-orchestrator`（含 P1-c PR #112 + 8 commit + 1 docs commit）
- **70+ 文件** 重命名（`toolrunner/` → `tools/`，49 production + 21 test）
- **13 文件** `facade/` → `legacy/` 迁移
- **2 新建** 共享类型/契约（`internal/shared/types/memory.go` + `internal/shared/contracts/memory.go`）
- **1 新建** `internal/layers/contextengine/persist/memory/store.go`（S17-A03 物理落点）
- **1 删除** `enforce/orchestrator.go`（92 行 stub）
- **7 新增** layout guards（D2-STRUCT-T01..T07）
- **6 文档** 同步（d2-domain + code-layout + layering + layer-delta + span-registry + code-atlas）

## Tech Debt

无新增 TD。所有 v2.2 终态目标已闭合；legacy/ 目录最终删除条件（AC-P5-4）为「所有 Process caller 已迁 D7 路径 + 集成测试全绿持续 ≥7 天」，按 S7 归档后转入下个 change 跟踪。

## Conclusion

**ACCEPTED — PASS.** D2 v2.2 Structure 终态完成（19/19 AC 全 PASS）。规格双锚点（`openspec/specs/d2-context-engine/` ↔ `internal/layers/contextengine/`）100% 一致；scenario 编排成为生产唯一 SoT；layout 守卫 7/7 IMPLEMENTED；go vet + go test -race + layer-lint strict + D2 集成全绿。可归档 S7。

---

**归档元数据：**
- Archive path: `openspec/archive/2026-06-19-devrix-d2-structure-closure/`
- Parent changes: `devrix-d2-sa-refine` (DM-20260614-009), `devrix-d2-queryloop-dismantle` (DM-20260618-010)
- PR: 合并后补登 (待 gh CLI 可用时)
