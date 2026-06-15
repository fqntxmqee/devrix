# acceptance-report（v1.0）— D6 Evolution S/A 重切

| 属性 | 值 |
|------|-----|
| Change ID | devrix-d6-sa-refine |
| DM ID | DM-20260615-002 |
| 验收日期 | 2026-06-15 |
| 验收人 | Claude Code (AI) |
| Decision | **ACCEPTED** |
| 归档 | openspec/archive/2026-06-15-devrix-d6-sa-refine/ |

## AC 裁决

| # | AC | 裁决 | 证据 |
|---|----|------|------|
| AC-01 | a-registry.md Canonical S11–S14 + Legacy 双轨 | ✅ PASS | a-registry.md v3.0: S11(5A)/S12(1A)/S13(1A PLANNED)/S14(1A PLANNED)，每行 Legacy 列 |
| AC-02 | t-registry.md canonical_s + Legacy T ID 列 | ✅ PASS | t-registry.md v3.0: 24 T (22 IMPL/2 PLANNED), 6 P0，每行 canonical_s + Legacy T ID |
| AC-03 | layering.md §D6 Canonical/Legacy 表 | ✅ PASS | 新增 Canonical S11–S14 + Legacy S1–S4 → Canonical 映射 |
| AC-04 | code-layout.md §4.7 D6 scenario-slug 注册表 | ✅ PASS | evaluate/guard/version/reload 四 slug 已登记 |
| AC-05 | design.md A/F 编排 + Decision 表 | ✅ PASS | 3 Decision + 完整 Legacy→Canonical T 映射表 |
| AC-06 | 统计一致性 | ✅ PASS | 24 T (22 IMPL/2 PLANNED), 6 P0，S3-Gate 验证 |
| AC-07 | P0 T 层数不变（6 P0） | ✅ PASS | 零 P0 变更 |
| AC-08 | S4→S12 GuardRuntime 命名冲突解决 | ✅ PASS | D6 S4 "Orchestration" 曾与 D7 Orchestration Domain 重名，现已改为 S12 GuardRuntime |
| AC-09 | S1/S2 PLANNED → S13/S14 | ✅ PASS | 原 D6-S1 TrackVersion / D6-S2 ReloadConfig 占位符重编号为 S13/S14 |
| AC-10 | 零代码变更 | ✅ PASS | v1.0 仅注册表/文档，不改任何 Go 文件 |
| AC-11 | Legacy Module Index 完整 | ✅ PASS | a-registry + t-registry 均含 Legacy S1–S4→Canonical 索引 |

## 变更摘要

### v1.0 范围

纯文档重构，无代码变更。修复 D6 Evolution 域 3 个结构性问题：

| 问题 | 修复 |
|------|------|
| S1/S2 占位符无实际内容 | 重编号为 S13 TrackVersion / S14 ReloadConfig，状态 PLANNED |
| S4 "Orchestration" 与 D7 命名冲突 | 重命名为 S12 GuardRuntime |
| S3 Eval 位置不自然 | 重编号为 S11 RunEvaluation |

### Canonical 重排

| 旧 S | Canonical S | Scenario |
|------|-------------|----------|
| D6-S3 Eval | S11 | RunEvaluation |
| D6-S4 Orchestration | S12 | GuardRuntime |
| D6-S1 Version | S13 | TrackVersion (PLANNED) |
| D6-S2 Config | S14 | ReloadConfig (PLANNED) |

### 修改文件

| 文件 | 变更 |
|------|------|
| `openspec/specs/d6-evolution/a-registry.md` | v2→v3.0: Canonical S11–S14 + Legacy 列 |
| `openspec/specs/d6-evolution/t-registry.md` | v2→v3.0: canonical_s 列 + Legacy T ID 列 |
| `openspec/specs/architecture/layering.md` | §D6 Canonical/Legacy 双表 |
| `openspec/specs/architecture/code-layout.md` | §4.7 D6 scenario-slug 注册表 |
| `openspec/specs/architecture/cross-domain-boundaries.md` | v1.4.0 revision entry |
| `openspec/changes/devrix-d6-sa-refine/` | proposal + demand + design + tasks |

## Decision

**ACCEPTED** — 11 AC 全部 PASS。v1.0 注册表重排质量合格，GuardRuntime 命名冲突已解决，Legacy 双轨完整，零代码变更，可归档。
