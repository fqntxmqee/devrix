# Proposal: D7 子包清理热身 Sprint

**Change ID:** `d7-package-cleanup-sprint`
**Demand ID:** DM-20260625-018
**Priority:** P1
**Sprint:** d7-v6 收尾
**PR Count:** 3
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**SoT:** v6.0.0 域升级 6 S + 1 横切（DM-20260626-001/002/003/004/005/007 已 S7_Archived） + 6 个 follow-up PR 全部 S7_Archived 后剩余的"小尾巴"

---

## 1. Background

v6.0.0 域升级把 D7 编排层 14 S 博弈角色精简为 6 S + 1 横切（hardening）。6 个 follow-up PR（#1-#6, 详见 `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/acceptance-report.md`）已全部 S7_Archived：

- DM-20260626-001 spec 落地（14 S → 6 S + 1 横切文档）
- DM-20260626-002 mups 子树物理迁移
- DM-20260626-003 hardening 横切包物理落地
- DM-20260626-004 turn/ → sessionorchestrator/ 整包合并
- DM-20260626-005 exit_reason/ → executionflow/verify/ promote
- DM-20260626-007 6 S bootstrap wire 14 → 6 收口

**遗留 4 个小子包**是 v6.0.0 之前的"逻辑分离"产物，每个只有 2-3 个文件、零独立测试价值：

| 子包 | 父包 | 文件数 |
|------|------|-------|
| `runregistry/` | `workmodel/` | 3 (1 prod, 1 helper, 1 test) |
| `toolpolicy/` | `decisionplanning/` | 6 (3 prod + 3 test) |
| `sessionqueue/` | `executionflow/` | 3 (1 prod + 2 test) |
| `d7spans/` | `hardening/` | 2 (1 prod + 1 test) |

## 2. Problem Statement

虽然 v6.0.0 spec/code 语义层已完成 6 S + 1 横切的对齐，但 4 个小子包仍在以下方面构成问题：

1. **目录浏览噪音**：读 D7 目录需要下钻一层才知道逻辑
2. **import 路径碎片化**：`workmodel/run_spawn.go` 已 import `workmodel` 父包却还要 import `runregistry` 子包（同级不同包，跨包访问）
3. **新成员认知负担**：每个子包都要解释一次"为什么独立"
4. **review 噪音**：4 个子包导致 4 个独立的 import graph 节点需要 review

## 3. Proposed Solution

**4 个小子包通过 3 个 PR 完成物理合并到父包，纯 git mv + import 路径替换，0 函数签名变化：**

### 3.1 PR 分组（按"影响半径 + 隔离风险"）

| PR | 范围 | 工作量 | 风险 |
|----|------|-------|------|
| **PR-1** | `runregistry` → `workmodel`（热身） | 0.5 天 | 最低：仅 1 个跨域 importer + 1 spec |
| **PR-2** | `toolpolicy` → `decisionplanning`（跨域隔离） | 1 天 | 中：唯一 D2→D7 反向引用，隔离避免影响其他 PR |
| **PR-3** | `d7spans` → `hardening` + `sessionqueue` → `executionflow` | 1.5 天 | 较高：两个最耦合的子包合并 |

### 3.2 合并原则

- **纯物理迁移**：0 函数签名变化、0 业务逻辑改动、0 test 断言改动
- **in-package import 删行**：workmodel 内部 importer 合并后变 in-package，直接删 import 行
- **跨域 import 改路径**：4 个跨域 importer 全仓统一替换路径
- **executionflow 父级扁平**：sessionqueue 内容下沉到 executionflow/ 父级（executionflow/ 当前无 Go 文件），新增 doc.go 解释

### 3.3 spec/code 同步

| 类型 | 文件 | 数量 |
|------|------|------|
| D7 spec | `openspec/specs/d7-orchestration/{t-registry,design,d7-domain,observability-guide,d7-requirements-clarifications}.md` | 5 |
| 全局 spec | `openspec/specs/architecture/code-layout.md` | 1 |
| D2 spec | `openspec/specs/d2-context-engine/{spec,a-registry,t-registry}.md` | 3 |
| CI 资源 | `.github/CODEOWNERS`、`scripts/audit-property-rights.sh` | 2 |

## 4. Trade-offs

### 4.1 方案 A：扁平到 executionflow/ 父级（推荐）✅

- **优点**：与其他 3 个合并一致（runregistry→workmodel 父级、toolpolicy→decisionplanning 父级、d7spans→hardening 父级）
- **缺点**：executionflow/ 父级从 0 Go 文件 → 3 Go 文件
- **缓解**：新增 `executionflow/doc.go` 1 行注释说明（参考 hardening/doc.go 模式）

### 4.2 方案 B：合并到 executionflow/hub/ 兄弟包（备选）

- **优点**：executionflow/ 父级保持空目录
- **缺点**：hub 包从 2 文件 → 6 文件，与 bridge 逻辑混在一起
- **否决**：违反 6 S + 1 横切对齐精神

## 5. Risks & Mitigations

| 风险 | 可能性 | 影响 | 缓解 |
|------|-------|------|------|
| in-package import 残留 | 高 | 编译失败 | 每 PR `go build ./...` 验证 |
| `enforce/contracts.go` 注释路径遗漏 | 低 | 无（注释） | `rg "toolpolicy"` 二次扫描 |
| `audit-property-rights.sh` 兜底分支悬挂 | 中 | CI 失败 | PR-1 必删 |
| `CODEOWNERS` 孤儿行 | 中 | PR 被拦 | PR-1 必删 |

## 6. Timeline

| 日期 | 工作 |
|------|------|
| 周二 | PR-1 runregistry → workmodel（热身，半日） |
| 周三 | PR-2 toolpolicy → decisionplanning（跨域隔离，1 日） |
| 周四-周五 | PR-3 d7spans + sessionqueue（结构扁平，1.5 日） |
| 周末 | spec 同步 follow-up PR（如需要） |
| 下周 | S6 归档 PR |

**总计**：1 周内 3 PR 合入，2 周内 S6 归档
