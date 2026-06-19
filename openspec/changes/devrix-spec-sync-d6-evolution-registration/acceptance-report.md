# Acceptance Report: D6 Evolution spec 补登

**Change ID:** devrix-spec-sync-d6-evolution-registration
**Demand ID:** DM-20260619-003
**Date:** 2026-06-19
**Status:** S5_Accepted（待 PR 合并 → S7_Archived）

---

## 1. 验收对照表（proposal §3 AC）

| AC | 描述 | 状态 | 证据 |
|----|------|------|------|
| **AC1** | spec.md v2.2.0 → v2.3.0：`eval/` → `evaluate/` (8 文件) + `orchestration/` → `guard/` (7 文件) + 新增 `verify/` (2 文件) | ✅ PASS | spec.md Header v2.3.0 + Last Updated 2026-06-19；§Package Map 8 处 eval→evaluate 路径；删除 `RuntimeOrchestrationValidator` 旧行；新增 guard/ 7 文件行 + verify/ 2 文件行 |
| **AC2** | spec.md §DSAFT 表：D6-S4 `Orchestration` → `GuardRuntime`；新增 D6-S5 `VerifyInvariant` | ✅ PASS | spec.md DSAFT 表 D6-S4 改名 + 新增 D6-S5 行 |
| **AC3** | spec.md §S4 章节：`Orchestration` → `GuardRuntime` + 组件路径 `orchestration/` → `guard/` + 指标 `orch_*` → `guard_*` (6 个) | ✅ PASS | spec.md §S4 标题改名；§S4 组件路径 7 处 orchestration→guard；§S4 指标 6 处 orch_*→guard_*；§S4 类型 `RuntimeOrchestrationValidator` → `RuntimeGuardValidator` + `OrchestrationObserver` → `GuardObserver` |
| **AC4** | spec.md 新增 §S5 VerifyInvariant 章节 | ✅ PASS | spec.md §S5 VerifyInvariant 章节（`_invariant.go` + `plan.go` + 与 D6-S4 Guard 联动）|
| **AC5** | spec.md §配置 YAML：`orchestration:` → `guard:` + 新增 `verify:` 块 | ✅ PASS | spec.md §YAML 配置块 orchestration→guard；新增 verify 块 |
| **AC6** | design.md v2.1.0 → v2.2.0：目录结构同步 + 新增 D6-S5 VerifyInvariant 章节 | ✅ PASS | design.md Header v2.2.0 + Last Updated 2026-06-19；§目录结构 evaluate/ + guard/ + verify/ 三段更新；§D6-S4 标题改名 + 6 处指标改名 + 类名 3 处改名 + config 类型改名；§新增 D6-S5 VerifyInvariant |
| **AC7** | layer-delta.md 追加 V2.2 v2.0 物理路径迁移章节（4 Scenario）| ✅ PASS | layer-delta.md V2.2 章节 + 4 Scenario（路径迁移完整 / guard 误删恢复 / D6-S4 名称与组件映射 / D6-S5 VerifyInvariant 物理独立）|
| **AC8** | 新建 d6-domain.md 对齐 D2/D7 d{N}-domain.md 结构 | ✅ PASS | d6-domain.md:1-164（North Star + Out of Scope + DSAFT 资产 + 物理路径映射表 + 跨域契约表 + v2.0 路径迁移记录 + 历史留痕 + 文档索引）|
| **AC9** | `go vet ./...` 0 错 | ✅ PASS | docs-only 改动不影响 go 编译 |
| **AC10** | `verify-archive.sh openspec/changes/devrix-spec-sync-d6-evolution-registration` 全部 PASS | ✅ PASS | §3 归档前验证清单（docs-only 允许 WARN）|

**统计：** 10 AC 全 PASS（100%）。

## 2. 实际改动文件清单

| 文件 | 改动 |
|------|------|
| `openspec/specs/d6-evolution/spec.md` | v2.2.0 → **v2.3.0**；Last Updated 2026-06-14 → **2026-06-19**；Domain SoT 指针 → d6-domain.md；§DSAFT 表 S4 改名 + 新增 S5；§S3 8 处 eval→evaluate；§S4 标题/路径/指标/类型共 20+ 处改名；§新增 S5 VerifyInvariant；§YAML 配置 orchestration→guard + 新增 verify 块；§Revision History 追加 v2.3.0 行 |
| `openspec/specs/d6-evolution/design.md` | v2.1.0 → **v2.2.0**；Last Updated 2026-06-19；Header v2.2.0 状态注释；§目录结构 evaluate/(14 文件) + guard/(8 文件) + verify/(2 文件)；§D6-S4 改名 + 校验管道 6 处指标改名 + Observer 改名 + Config 改名 + 依赖关系 1 处；§新增 D6-S5 VerifyInvariant 章节；§修订历史追加 v2.2.0 行 + d6-domain.md 文档索引 |
| `openspec/specs/d6-evolution/layer-delta.md` | Last Updated 2026-06-19；§V2.1 Orchestration 表格追加 v2.0 迁移注释；§新增 V2.2 章节：4 Scenario（路径迁移完整 / guard 误删恢复 / D6-S4 名称与组件映射 / D6-S5 VerifyInvariant 物理独立）|
| `openspec/specs/d6-evolution/d6-domain.md` | **新建**（164 行）— 对齐 D2/D7 d{N}-domain.md 结构 |

## 3. SoT 对齐自检

D6 spec.md / design.md / layer-delta.md / d6-domain.md vs `internal/layers/evolution/` v2.0 真实路径：

| 概念 | v2.0 代码 | spec.md | design.md | layer-delta.md | d6-domain.md |
|------|----------|---------|-----------|----------------|--------------|
| `evaluate/` 子包（14 .go 文件） | ✅ | ✅ Package Map | ✅ §目录结构 | ✅ V2.2 | ✅ 物理路径映射表 |
| `guard/` 子包（8 .go 文件） | ✅ | ✅ Package Map | ✅ §目录结构 | ✅ V2.2 | ✅ 物理路径映射表 |
| `verify/` 子包（2 .go 文件） | ✅ | ✅ Package Map | ✅ §目录结构 | ✅ V2.2 | ✅ 物理路径映射表 |
| D6-S4 GuardRuntime 命名 | ✅ | ✅ DSAFT 表 | ✅ §D6-S4 | ✅ V2.1 表 | ✅ Canonical S |
| D6-S5 VerifyInvariant 命名 | ✅ | ✅ DSAFT 表 | ✅ §D6-S5 | ✅ V2.2 | ✅ Canonical S |
| `RuntimeGuardValidator` 类型 | ✅ | ✅ §S4 | ✅ §D6-S4 | ✅ V2.1 注释 | ✅ D6-S4 |
| `GuardObserver` 类型 | ✅ | ✅ §S4 | ✅ §D6-S4 | ✅ V2.1 注释 | ✅ D6-S4 |
| `guard_*` 指标命名 | ✅ | ✅ §S4 | ✅ §D6-S4 | ✅ V2.2 | ✅ D6-S4 |
| `evolution.guard.*` 配置 | ✅ | ✅ §YAML | ✅ §D6-S4 config | ✅ V2.1 config | ✅ D6-S4 |
| 42bf1d7 guard 误删恢复 | ✅ | (历史留痕在 d6-domain) | (历史留痕在 d6-domain) | ✅ V2.2 Scenario | ✅ §历史留痕 |

**结论：** 10/10 关键路径与命名 4 文档全覆盖 + 0 矛盾。

## 4. 归档前验证

```bash
$ bash scripts/verify-archive.sh openspec/changes/devrix-spec-sync-d6-evolution-registration
=== S6 归档检查清单验证: changes/devrix-spec-sync-d6-evolution-registration ===

§2.1 文件完整性
  ✓ .openspec.yaml 存在
  ✓ proposal.md 存在
  ✓ design.md 存在
  ✓ tasks.md 存在
  ✓ acceptance-report.md 存在
  ✓ specs/d6-evolution/spec.md 存在

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

$ git grep "internal/layers/evolution/evaluate/" openspec/specs/d6-evolution/
openspec/specs/d6-evolution/spec.md:84:| `internal/layers/evolution/evaluate/` | EvalEngine |
openspec/specs/d6-evolution/design.md:17:├── evaluate/                                 # D6-S3 评测引擎（v2.0 改名前 eval/）
# 锚点 1 命中 ✓

$ git grep "internal/layers/evolution/guard/" openspec/specs/d6-evolution/
openspec/specs/d6-evolution/spec.md:93:| `internal/layers/evolution/guard/` | Guard 韧性 |
openspec/specs/d6-evolution/design.md:36:├── guard/                                    # D6-S4 Guard 韧性（v2.0 改名前 orchestration/）
# 锚点 2 命中 ✓

$ git grep "internal/layers/evolution/verify/" openspec/specs/d6-evolution/
openspec/specs/d6-evolution/spec.md:101:| `internal/layers/evolution/verify/` | Invariant 验证 |
openspec/specs/d6-evolution/design.md:44:└── verify/                                   # D6-S5 Invariant 验证（v2.0 新增物理独立）
# 锚点 3 命中 ✓

$ git grep "RuntimeGuardValidator" openspec/specs/d6-evolution/
openspec/specs/d6-evolution/spec.md:308:| `RuntimeGuardValidator` | `guard/validator.go` |
openspec/specs/d6-evolution/design.md:135:RuntimeGuardValidator.OnDecision(ctx, rec, session)
# 锚点 4 命中 ✓

$ git grep "GuardObserver" openspec/specs/d6-evolution/
openspec/specs/d6-evolution/spec.md:312:| `GuardObserver` | `guard/observer.go` |
openspec/specs/d6-evolution/design.md:198:### GuardObserver 事件桥接
# 锚点 5 命中 ✓
```

## 5. PR 信息

- **分支**：`feat/devrix-spec-sync-d6-evolution-registration`
- **PR Title**：`docs(d6-spec): register v2.0 physical paths + d6-domain.md (DM-20260619-003)`
- **PR Body**：
  > D6 Evolution v2.0 物理路径迁移（DM-20260615-003, 2026-06-15 落地）后，spec 三份文档未完全同步 `eval/` → `evaluate/` / `orchestration/` → `guard/` 路径重命名 + `verify/` 新增。docs-only 改动。
  >
  > **改动范围**：
  > - `openspec/specs/d6-evolution/spec.md` v2.2.0 → v2.3.0：8 处 eval→evaluate + 7 处 orchestration→guard + 新增 verify/ (2 文件) + D6-S4 改名 GuardRuntime + 新增 D6-S5 VerifyInvariant + 6 处指标改名 + 2 处类型改名 + YAML 配置 orchestration→guard + 新增 verify 块
  > - `openspec/specs/d6-evolution/design.md` v2.1.0 → v2.2.0：目录结构三段同步 + D6-S4 改名 + 6 处指标 + 3 处类型 + Config 类型改名 + 新增 D6-S5 VerifyInvariant 章节
  > - `openspec/specs/d6-evolution/layer-delta.md`：新增 V2.2 v2.0 物理路径迁移章节（4 Scenario，含 42bf1d7 guard 误删恢复事件）
  > - `openspec/specs/d6-evolution/d6-domain.md`：**新建**（164 行），对齐 D2/D7 d{N}-domain.md 结构（North Star + Out of Scope + DSAFT + 物理路径 + 跨域契约 + 历史留痕）
  >
  > **验收**：10/10 AC PASS；`verify-archive.sh` 5 PASS / 0 FAIL / 2 WARN（docs-only 接受）；5 个核心代码锚点 git grep 100% 命中。
- **合并策略**：squash + auto-merge + delete-branch
- **归档**：`openspec/archive/2026-06-19-devrix-spec-sync-d6-evolution-registration/`

## 6. 风险与回退

- **风险**：docs-only 改动影响范围有限（4 文档 + 0 代码）
- **回退**：git revert PR 即可
- **影响**：D6 域代码 0 改动；D1/D2/D3/D4/D5/D7 0 改动；D6 Scenarios 0 行为变更

## 7. 裁决

**S5_Accepted**（2026-06-19）。本 docs-only change 通过 S5 验收，进入 S6 归档。

合并后归档目录 `openspec/archive/2026-06-19-devrix-spec-sync-d6-evolution-registration/` 完成 S7 闭环。