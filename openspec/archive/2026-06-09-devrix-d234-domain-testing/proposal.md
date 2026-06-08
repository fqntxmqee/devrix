# Proposal: D2/D3/D4 Domain Segmentation Testing

**Change ID:** devrix-d234-domain-testing
**Demand ID:** DM-20260609-001
**Status:** S5 Accepted

## Solution

采用**方案 B**：在层级 build tag 上叠加域 tag，全量脚本显式 `-tags="integration,d1,d2,d3,d4,d5,cross"`，域脚本传子集。

## Deliverables

| 产物 | 路径 |
|------|------|
| 域分段 SoT | `openspec/specs/testing-framework/domain-segmentation.md` |
| 框架规范更新 | `openspec/specs/testing-framework/spec.md` |
| 项目入口更新 | `openspec/specs/project/testing.md` |
| 域脚本 | `scripts/test-domain.sh` |
| 测试 tag 标注 | `tests/integration/*`, `tests/acceptance/p0/*`, `tests/performance/*`, `tests/e2e/*` |

## Non-Goals

- 不新增 L5 测试点（测试基础设施变更）
- 不移动 `internal/` 单元测试位置
