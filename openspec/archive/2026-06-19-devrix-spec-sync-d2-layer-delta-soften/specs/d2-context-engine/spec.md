# Spec: D2 spec 退役标记完整性

**Change ID:** devrix-spec-sync-d2-layer-delta-soften
**Demand ID:** DM-20260619-004
**Status:** S7_Archived (2026-06-19)
**Capability:** d2-context-engine
**Version:** layer-delta.md softening + d7-boundary.md 状态列追加
**Affects:** openspec/specs/d2-context-engine/layer-delta.md, openspec/specs/d2-context-engine/d7-boundary.md

---

## 1. 改动范围（docs-only）

本 change **不新增业务能力**，仅软化 D2 spec 中 QueryLoop "Primary Runtime" 措辞 + 补全 d7-boundary.md 契约表状态列，对齐 spec.md §18 LEGACY 标记（DM-20260617-001）。

| 文档 | 旧内容 | 新内容 | 关键变更 |
|------|--------|--------|----------|
| `openspec/specs/d2-context-engine/layer-delta.md` | `QueryLoop Primary Runtime` + `MUST route all` | `QueryLoop Default Runtime ⚠️ DEPRECATED in loopFirst=false path` | 加 DEPRECATED 注脚（DM-20260617-001，canonical=D7-S2-A06）；软化 MUST 措辞；保留所有 D2-S10 Scenario |
| `openspec/specs/d2-context-engine/d7-boundary.md` §4 契约接口表 | 4 列（接口/定义/实现/消费）| 5 列（接口/定义/实现/消费/状态）| `Loop.Run` + `LoopHooks` 行标 **DEPRECATED**（2026-06-17 DM-001; loopFirst=false; canonical=D7-S2-A06 RunTurnLoop）；其他 4 行标 ACTIVE |
| `openspec/specs/d2-context-engine/d7-boundary.md` §79 表格 | `LoopHooks | query/loop.go | D7 注入 | D2 Loop` | 末尾追加 `| **DEPRECATED** (loopFirst=false; canonical=D7-S2-A06 RunTurnLoop) |` | |

## 2. 不变更（边界声明）

- `internal/layers/contextengine/**` 全部代码
- `openspec/specs/d2-context-engine/spec.md` §18 LEGACY 标记（已存在，保持）
- D2 Scenarios（保留回滚兼容）
- D-S 编号体系（D2-S/A/F/T）
- `t-registry.md`

## 3. 与现有 spec 关系

| 现有 spec | 关系 |
|-----------|------|
| `openspec/specs/d2-context-engine/spec.md` §18 LEGACY 标记 | **SoT**（不动，本 change 与之对齐） |
| `openspec/specs/d2-context-engine/layer-delta.md` | 本 change 直接修订 |
| `openspec/specs/d2-context-engine/d7-boundary.md` | 本 change 直接修订 |
| `openspec/specs/d7-orchestration/spec.md` v3.8.0 | canonical D7-S2-A06 RunTurnLoop 来源 |
| `openspec/specs/project/master.md` | 规范权威 |

## 4. 验收

- AC 详见 `acceptance-report.md`
- `verify-archive.sh` 全部 PASS
- `go vet ./...` 0 错
- `git grep "MUST route all" openspec/specs/d2-context-engine/` 0 命中
- `git grep "DEPRECATED" openspec/specs/d2-context-engine/layer-delta.md` ≥ 1 命中
- `git grep "DEPRECATED" openspec/specs/d2-context-engine/d7-boundary.md` ≥ 2 命中

## 5. 归档位置

`openspec/archive/2026-06-19-devrix-spec-sync-d2-layer-delta-soften/`