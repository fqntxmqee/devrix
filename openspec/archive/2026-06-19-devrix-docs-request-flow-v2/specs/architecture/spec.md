# Spec: 链路图文档对齐 D7 v2.0

**Change ID:** devrix-docs-request-flow-v2
**Demand ID:** DM-20260619-001
**Status:** S7_Archived (2026-06-19; PR #89 merged)
**Capability:** architecture-layering
**Version:** 1.2.0 (code-atlas); 1.1.0 (dsaft-overview)
**Affects:** docs/architecture/*, openspec/specs/architecture/*

---

## 1. 改动范围（docs-only）

本 change **不新增业务能力**，仅对齐 3 个架构文档到 D7 编排层 v3.8.0（2026-06-18）SoT。

| 文档 | 旧版本 | 新版本 | 关键变更 |
|------|--------|--------|----------|
| `docs/architecture/request-flow.md` | 2026-06-13 (QueryLoop era) | 2026-06-19 (D7 v2.0 era) | 4 IntentKind 真实链 + turn.RunTurn + D7 直调 D3 + D2→D3 import ban |
| `openspec/specs/architecture/code-atlas.md` | v1.1.0 | **v1.2.0** | D-S Index 替换为 D7 v2.0 unified（19 模块）；3 个 DEPRECATED；加 `wire_coordinator.go` |
| `docs/architecture/dsaft-overview.md` | v1.0.0 | **v1.1.0** | 6 域+ORCH → **7 域架构**（D7 升格核心域）；D7-S 5 子层 IMPLEMENTED |

## 2. 不变更（边界声明）

- **不新增 D-S 编号**：D1-D7 + S/A/F/T 编号体系不变
- **不修改 D7 spec v3.8.0**：SoT 单向对齐，docs 单向同步到 spec
- **不改 .go 源码**：docs-only 改动
- **不新增 T 点**：纯文档对齐

## 3. 与现有 spec 关系

| 现有 spec | 关系 |
|-----------|------|
| `openspec/specs/d7-orchestration/spec.md` v3.8.0 | **SoT**（不动）|
| `openspec/specs/architecture/code-atlas.md` v1.2.0 | 本 change 直接修订 |
| `openspec/specs/architecture/layering.md` v3.1.0 | 引用，未改 |
| `openspec/specs/project/master.md` | 规范权威，本 change 4 件套依据 |

## 4. 验收

- 7/7 AC PASS（详见 `acceptance-report.md`）
- `go vet ./...` 0 错
- 7 个代码锚点 `git grep` 100% 命中
- 关键词命中 25 次（要求 ≥10）
- CI 7/7 SUCCESS（layer-lint strict/warn + unit + integration + e2e + acceptance + coverage）

## 5. 归档位置

`openspec/archive/2026-06-19-devrix-docs-request-flow-v2/`
PR: https://github.com/fqntxmqee/devrix/pull/89
