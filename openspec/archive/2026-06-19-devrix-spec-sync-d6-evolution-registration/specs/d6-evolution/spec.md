# Spec: D6 Evolution spec 补登

**Change ID:** devrix-spec-sync-d6-evolution-registration
**Demand ID:** DM-20260619-003
**Status:** S7_Archived (2026-06-19)
**Capability:** d6-evolution
**Version:** 2.3.0 (spec.md); 2.2.0 (design.md); V2.2 layer-delta; 1.0.0 d6-domain.md (new)
**Affects:** openspec/specs/d6-evolution/spec.md, design.md, layer-delta.md; 新建 d6-domain.md

---

## 1. 改动范围（docs-only）

本 change **不新增业务能力**，仅对齐 3 份 D6 spec 文档 + 新建 1 份 d6-domain.md 到 v2.0 物理路径迁移（DM-20260615-003, 2026-06-15 落地）状态。

| 文档 | 旧版本 | 新版本 | 关键变更 |
|------|--------|--------|----------|
| `openspec/specs/d6-evolution/spec.md` | v2.2.0 | **v2.3.0** | `eval/` → `evaluate/`（8 文件）；`orchestration/` → `guard/`（7 文件 + 6 指标 + 2 类型）；新增 `verify/`（2 文件）+ D6-S5 章节 |
| `openspec/specs/d6-evolution/design.md` | v2.1.0 | **v2.2.0** | 目录结构同步；新增 D6-S5 VerifyInvariant 章节 |
| `openspec/specs/d6-evolution/layer-delta.md` | V2.1 | **V2.1 + V2.2** | 追加 v2.0 物理路径迁移章节（4 Scenario）|
| `openspec/specs/d6-evolution/d6-domain.md` | (无) | **1.0.0 (新建)** | 对齐 D2/D7 d{N}-domain.md 结构（North Star + Out of Scope + DSAFT + 物理路径 + 跨域契约 + 历史留痕）|

## 2. 不变更（边界声明）

- `internal/layers/evolution/**` 全部代码（v2.0 物理路径迁移已在 DM-20260615-003 落地）
- D6 Scenarios 行为
- D-S 编号体系（D6-S/A/F/T）
- `t-registry.md`

## 3. 与现有 spec 关系

| 现有 spec | 关系 |
|-----------|------|
| `openspec/specs/d6-evolution/spec.md` | 本 change 直接修订 v2.2.0 → v2.3.0 |
| `openspec/specs/d6-evolution/design.md` | 本 change 直接修订 v2.1.0 → v2.2.0 |
| `openspec/specs/d6-evolution/layer-delta.md` | 本 change 直接追加 V2.2 章节 |
| `openspec/specs/d6-evolution/d6-domain.md` | 本 change 新建 |
| `openspec/specs/project/master.md` | 规范权威，本 change 4 件套依据 |

## 4. 验收

- AC 详见 `acceptance-report.md`
- `verify-archive.sh` 全部 PASS
- `go vet ./...` 0 错
- `git grep` 验证新路径（`evaluate/` / `guard/` / `verify/`）全部命中
- `git grep` 验证旧路径（`eval/` / `orchestration/`）在 spec 文档中仅作历史引用

## 5. 归档位置

`openspec/archive/2026-06-19-devrix-spec-sync-d6-evolution-registration/`