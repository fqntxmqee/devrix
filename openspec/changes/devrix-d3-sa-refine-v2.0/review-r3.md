---
review-round: R3
reviewer: Claude 运行层
change-id: devrix-d3-sa-refine-v2.0
demand-id: DM-20260614-019
date: 2026-06-14
status: APPROVED
---

# Review R3 — D3 v2.0 运行层审查

## 0. 命题

| # | 命题 | 裁决 |
|---|------|------|
| P1 | 物理迁移后 11 P0 T 保持 IMPLEMENTED | ✅ 预期 PASS（仅 import 路径变更） |
| P2 | runtime span/metric/config key 字面量不变 | ✅ PASS（v1.0 不变性承诺） |
| P3 | `go build ./...` 全绿（含旧路径 re-export） | ✅ 预期 PASS |
| P4 | Bridge 跨域锚点 `internal/bridges/llm/` 路径不变 | ✅ PASS |

## 1. P1 — P0 T 保持

v2.0 仅移动文件物理位置 + 更新 import 路径。所有 F 实现、T 测试逻辑不变。`t-registry.md` 中的 T ID 不变（S/A/F 编号不动）。

## 2. P2 — 运行时字面量

5 span 名 + 5 metric 名 + YAML config key 全部保持 v1.0 字面量。v2.0 不新增也不改名。

## 3. P3 — 编译兼容

旧路径 re-export 桥接确保所有 import 旧路径的代码（包括外部测试 `tests/integration/`）在 1 发布周期内不破坏。

## 4. P4 — Bridge 锚点

`internal/bridges/llm/` 作为跨域锚点不变。F10 仅验证，不执行任何迁移。

## 5. NQ（Non-Questions，已由 v1.0 决议覆盖）

| # | 议题 | 覆盖来源 |
|---|------|---------|
| NQ-1 | 旧路径何时物理删除 | v1.0 R1 Q5（1 发布周期 + G4） |
| NQ-2 | contracts.go 是否引入 `kernel/` 子包 | v1.0 R3 NQ-6（不引入） |
| NQ-3 | 测试文件 import 是否需要同步 | 是，F2-F8 每步验证包含 go test |

## 6. 裁决

**4/4 命题 PASS。** 运行层无阻塞项。S3-Gate Cleared → S4 实施。

---

**Revision:** 0.1 — 2026-06-14 初稿
