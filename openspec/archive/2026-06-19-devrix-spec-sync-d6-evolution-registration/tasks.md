# Tasks: D6 Evolution spec 补登

**Change ID:** devrix-spec-sync-d6-evolution-registration
**Demand ID:** DM-20260619-003

---

## W1: 更新 `openspec/specs/d6-evolution/spec.md` v2.2.0 → v2.3.0

**目标**：v2.0 物理路径迁移完整同步

- [ ] Version: v2.2.0 → **v2.3.0**
- [ ] Last Updated: 2026-06-14 → **2026-06-19**
- [ ] Demand 标注：DM-20260619-003
- [ ] §Package Map：`eval/` → `evaluate/`（8 个文件路径）
- [ ] §Package Map：删除 `RuntimeOrchestrationValidator | orchestration/validator.go`（已迁至 guard/）
- [ ] §Package Map：新增 `guard/` 子包（7 个 .go 文件）+ `verify/` 子包（2 个 .go 文件）
- [ ] §DSAFT 表：D6-S4 `Orchestration` → `GuardRuntime`；新增 D6-S5 `VerifyInvariant`
- [ ] §S3 探针路径：`eval/` → `evaluate/`（10 个探针）
- [ ] §S4 章节：组件 `orchestration/` → `guard/`（7 个 .go 文件）
- [ ] §S4 指标：`orch_*` → `guard_*`（6 个指标）
- [ ] §S4 类型：`RuntimeOrchestrationValidator` → `RuntimeGuardValidator`；`OrchestrationObserver` → `GuardObserver`
- [ ] §新增 D6-S5 VerifyInvariant（`_invariant.go` + `plan.go` + 与 D6-S4 Guard 联动）
- [ ] §配置 YAML：`orchestration:` → `guard:`；新增 `verify:` 块
- [ ] §Revision History 追加 v2.3.0 行
- [ ] §Domain SoT 指针：指向新建的 `d6-domain.md`

## W2: 更新 `openspec/specs/d6-evolution/design.md` v2.1.0 → v2.2.0

**目标**：v2.0 路径迁移 + 新增 D6-S5 VerifyInvariant 章节

- [ ] Version: v2.1.0 → **v2.2.0**
- [ ] Last Updated: 2026-06-14 → **2026-06-19**
- [ ] Header v2.2.0 状态注释（DM-20260619-003）
- [ ] §目录结构：`eval/` → `evaluate/`（14 个文件清单）；`orchestration/` → `guard/`（8 个文件清单）；新增 `verify/`（2 个文件清单）
- [ ] §D6-S4 标题：`Orchestration` → `GuardRuntime`
- [ ] §D6-S4 校验管道：`RuntimeOrchestrationValidator` → `RuntimeGuardValidator`；指标 `orch_*` → `guard_*`（6 处）
- [ ] §D6-S4 Observer 标题：`OrchestrationObserver` → `GuardObserver`
- [ ] §D6-S4 配置：`OrchestrationConfig` → `GuardConfig`
- [ ] §D6-S4 依赖关系：`OrchestrationObserver` → `GuardObserver`
- [ ] §新增 D6-S5 VerifyInvariant 章节（VerifyPlan 管道 + Invariant 接口 + 与 D6-S4 Guard 联动）
- [ ] §修订历史：v2.2.0 行

## W3: 追加 `openspec/specs/d6-evolution/layer-delta.md` V2.2 章节

**目标**：v2.0 物理路径迁移事实记录

- [ ] §Header：Affects 加上 `guard validator, verify invariant`
- [ ] §Header：Last Updated 2026-06-19
- [ ] §V2.1 Orchestration 表格：注释 v2.0 路径迁移后映射（orchestration→guard + 类名变更）
- [ ] §V2.2 新增章节：v2.0 物理路径迁移
  - [ ] 3 包重命名 + 1 包新增表
  - [ ] Scenario: 路径迁移完整
  - [ ] Scenario: guard 误删恢复（42bf1d7）
  - [ ] Scenario: D6-S4 名称与组件映射
  - [ ] Scenario: D6-S5 VerifyInvariant 物理独立

## W4: 新建 `openspec/specs/d6-evolution/d6-domain.md`

**目标**：对齐 D2/D7 d{N}-domain.md 结构（域描述 + 价值流 + 跨域契约）

- [ ] 头部元数据（Domain ID D6 / Slug evolution / Type Supporting / Version 1.0.0 / Last Updated 2026-06-19）
- [ ] §North Star：Self-Eval + Guard + Verify 三大支撑能力
- [ ] §Out of Scope：D1/D2/D3/D4/D5/D7 能力归属表
- [ ] §DSAFT 资产：Canonical S3-S5（Evaluate / GuardRuntime / VerifyInvariant）+ Legacy S1-S2
- [ ] §物理路径映射表（Canonical S → 代码目录）
- [ ] §跨域契约表（D2/D3/D4/D5/D7 五向）
- [ ] §v2.0 路径迁移记录
- [ ] §历史留痕（5 个事件，含 42bf1d7 guard 误删恢复）
- [ ] §规格文档索引（spec.md / design.md / d6-domain.md / layer-delta.md / a-f-t-registry.md）

## W5: 验证 + PR + 归档

- [ ] `bash scripts/verify-archive.sh openspec/changes/devrix-spec-sync-d6-evolution-registration` 全部 PASS（docs-only 允许 WARN）
- [ ] `go vet ./...` 0 错
- [ ] `git grep` 验证新路径（`internal/layers/evolution/evaluate/` / `internal/layers/evolution/guard/` / `internal/layers/evolution/verify/`）全部命中
- [ ] `git grep` 验证旧路径 `internal/layers/evolution/eval/` / `internal/layers/evolution/orchestration/` 在 spec 文档中**仅作为历史引用存在**（确认未误用）
- [ ] commit: `docs(d6-spec): register v2.0 physical paths + d6-domain.md (DM-20260619-003)`
- [ ] push: `git push -u origin feat/devrix-spec-sync-d6-evolution-registration`
- [ ] `gh pr create --title "..." --body "..." --base master`
- [ ] `gh pr merge --auto --squash --delete-branch`（Devrix PR Auto-Merge 偏好）
- [ ] 合并后：`mv openspec/changes/devrix-spec-sync-d6-evolution-registration openspec/archive/2026-06-19-devrix-spec-sync-d6-evolution-registration/`
- [ ] 合并后：`.openspec.yaml` `status: s7_archived`
- [ ] 合并后：`openspec/demand-archive-index.md` 追加新行（DM-20260619-003 → archive）
- [ ] 合并后：`git pull` 验证工作区干净

## 依赖关系

```
W1 ─┐
W2 ─┼─→ W5
W3 ─┤
W4 ─┘
```

W1/W2/W3/W4 互相独立可并行；W5 依赖前四者全部完成。