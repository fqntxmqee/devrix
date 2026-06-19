# Spec: D3 LLM Gateway spec 路径与 v2.0 状态同步

**Change ID:** devrix-spec-sync-d3-package-map
**Demand ID:** DM-20260619-002
**Status:** S7_Archived (2026-06-19)
**Capability:** d3-llm-gateway
**Version:** 3.3.0 (design.md); model-resolution-trace.md sync
**Affects:** openspec/specs/d3-llm-gateway/design.md, openspec/specs/d3-llm-gateway/model-resolution-trace.md

---

## 1. 改动范围（docs-only）

本 change **不新增业务能力**，仅对齐 2 个 D3 spec 文档到 v2.0 物理路径迁移（DM-20260614-019, 2026-06-14 落地）状态。

| 文档 | 旧版本 | 新版本 | 关键变更 |
|------|--------|--------|----------|
| `openspec/specs/d3-llm-gateway/design.md` | v3.2.0 | **v3.3.0** | §5.2 路径 `shared/config/llmgateway.go` → `layers/llmgateway/configure/shared_config.go`；§10.2 状态 `Phase F 实施中` → `✅ 已完成 (DM-20260614-019)` |
| `openspec/specs/d3-llm-gateway/model-resolution-trace.md` | 2026-06-14 | **2026-06-19** | 头部加 v2.0 路径迁移状态注释（4 处 import 路径变更说明） |

## 2. 不变更（边界声明）

- `internal/layers/llmgateway/**` 全部代码（v2.0 物理路径迁移已在 DM-20260614-019 落地）
- D3 Scenarios 行为（`Router.Resolve()` + `Router.ResolveTier()` 仅 import 路径变更，逻辑不变）
- D-S 编号体系（D3-S/A/F/T）
- `t-registry.md`

## 3. 与现有 spec 关系

| 现有 spec | 关系 |
|-----------|------|
| `openspec/specs/d3-llm-gateway/spec.md` | 不动（SoT 不变） |
| `openspec/specs/d3-llm-gateway/design.md` | 本 change 直接修订 v3.2.0 → v3.3.0 |
| `openspec/specs/d3-llm-gateway/model-resolution-trace.md` | 本 change 直接修订 |
| `openspec/specs/project/master.md` | 规范权威，本 change 4 件套依据 |

## 4. 验收

- AC 详见 `acceptance-report.md`
- `verify-archive.sh` 全部 PASS
- `go vet ./...` 0 错
- `git grep` 验证新路径命中

## 5. 归档位置

`openspec/archive/2026-06-19-devrix-spec-sync-d3-package-map/`