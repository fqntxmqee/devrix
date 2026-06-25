# Tasks: D7 子包清理热身 Sprint

**Change ID:** `d7-package-cleanup-sprint`
**Demand ID:** DM-20260625-018
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**Sprint:** d7-v6 收尾
**PR Count:** 3 (PR-1, PR-2, PR-3)

---

## 任务总览

| Phase | PR | 描述 | 工作量 | 状态 |
| ----- | -- | ---- | ------ | ---- |
| **S1** | - | OpenSpec 五件套（demand/proposal/tasks/design/specs） | 0.2 天 | ⬜ |
| **S2-S3** | - | Proposal + Design 写完 | 0.3 天 | ⬜ |
| **S4-PR-1** | PR-1 | `runregistry` → `workmodel`（热身） | 0.5 天 | ⬜ |
| **S4-PR-2** | PR-2 | `toolpolicy` → `decisionplanning`（跨域隔离） | 1 天 | ⬜ |
| **S4-PR-3** | PR-3 | `d7spans` → `hardening` + `sessionqueue` → `executionflow` | 1.5 天 | ⬜ |
| **S5** | - | acceptance-report.md 12/12 AC 验收 | 0.2 天 | ⬜ |
| **S6** | - | 3 PR auto-merge + S7 归档 | 0.5 天 | ⬜ |

**总计**: ~4.2 天

---

## PR-1: `runregistry` → `workmodel` (热身)

### Step 1.1: OpenSpec + 分支

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T1.1.1 | 创建 `feat/d7-package-cleanup-sprint-pr1-runregistry` 分支（从 master） | ⬜ |
| T1.1.2 | 创建 `openspec/changes/d7-package-cleanup-sprint/` 五件套 | ⬜ |

### Step 1.2: git mv

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T1.2.1 | `git mv runregistry/await.go workmodel/await.go` | ⬜ |
| T1.2.2 | `git mv runregistry/registry.go workmodel/registry.go` | ⬜ |
| T1.2.3 | `git mv runregistry/registry_test.go workmodel/registry_test.go` | ⬜ |
| T1.2.4 | 验证 `ls internal/layers/orchestration/runregistry/` 返回 0 文件 | ⬜ |

### Step 1.3: package 改名 + import 路径

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T1.3.1 | 3 个移动文件 `package runregistry` → `package workmodel` | ⬜ |
| T1.3.2 | 6 个 D7 内部 importer 路径替换 `runregistry"` → `workmodel"` | ⬜ |
| T1.3.3 | 4 个 workmodel 内部 importer 删 import 行（变 in-package 访问） | ⬜ |
| T1.3.4 | 1 个跨域 importer（bootstrap/wire_coordinator.go）路径替换 | ⬜ |
| T1.3.5 | 验证 `rg "orchestration/runregistry" internal/ cmd/ tests/` 0 命中 | ⬜ |

### Step 1.4: CI 资源清理

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T1.4.1 | `.github/CODEOWNERS` 删除 `runregistry/` 行 | ⬜ |
| T1.4.2 | `scripts/audit-property-rights.sh:26` 删除 `*orchestration/runregistry/*` 兜底分支 | ⬜ |
| T1.4.3 | `bash scripts/audit-property-rights.sh` 0 violations 验证 | ⬜ |

### Step 1.5: Spec 同步

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T1.5.1 | `openspec/specs/d7-orchestration/t-registry.md:85` `runregistry/spawn_test.go` → `workmodel/registry_test.go` | ⬜ |

### Step 1.6: 验证

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T1.6.1 | `go build ./...` 0 错误 | ⬜ |
| T1.6.2 | `go vet ./...` 0 警告 | ⬜ |
| T1.6.3 | `go test -race ./internal/layers/orchestration/workmodel/...` 全 PASS | ⬜ |
| T1.6.4 | `go test -race ./internal/layers/orchestration/...` 全 PASS | ⬜ |
| T1.6.5 | `bash scripts/verify-archive.sh` 12/12 PASS | ⬜ |
| T1.6.6 | commit + push + 开 PR + `gh pr merge --auto --squash --delete-branch` | ⬜ |

---

## PR-2: `toolpolicy` → `decisionplanning` (跨域隔离)

### Step 2.1: 分支

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T2.1.1 | 从 master 拉新分支 `feat/d7-package-cleanup-sprint-pr2-toolpolicy` | ⬜ |
| T2.1.2 | 确认 PR-1 已合入 master | ⬜ |

### Step 2.2: git mv

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T2.2.1 | `git mv toolpolicy/filter.go decisionplanning/filter.go` | ⬜ |
| T2.2.2 | `git mv toolpolicy/filter_adapter.go decisionplanning/filter_adapter.go` | ⬜ |
| T2.2.3 | `git mv toolpolicy/plan_mode.go decisionplanning/plan_mode.go` | ⬜ |
| T2.2.4 | 3 个 test 文件 `git mv` 到 decisionplanning/ | ⬜ |
| T2.2.5 | 验证 `ls internal/layers/orchestration/toolpolicy/` 返回 0 文件 | ⬜ |

### Step 2.3: package 改名 + import 路径

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T2.3.1 | 6 个移动文件 `package toolpolicy` → `package decisionplanning` | ⬜ |
| T2.3.2 | 4 个跨域 importer 路径替换（surfaces.go, context_engine.go, context_engine_builder.go, cli/tool/list.go） | ⬜ |
| T2.3.3 | 1 个 integration test importer 路径替换 | ⬜ |
| T2.3.4 | D2→D7 注释路径 `enforce/contracts.go:14` 替换 | ⬜ |
| T2.3.5 | 验证 `rg "orchestration/toolpolicy" internal/ cmd/ tests/` 0 命中 | ⬜ |

### Step 2.4: Spec 同步

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T2.4.1 | `openspec/specs/architecture/code-layout.md:101` 表格行替换 | ⬜ |
| T2.4.2 | `openspec/specs/d2-context-engine/spec.md:1197` 路径替换 | ⬜ |
| T2.4.3 | `openspec/specs/d2-context-engine/a-registry.md:73` 路径替换 | ⬜ |
| T2.4.4 | `openspec/specs/d2-context-engine/t-registry.md` 多处（244/260/307/308/318/333）路径替换 | ⬜ |

### Step 2.5: 验证

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T2.5.1 | `go build ./...` 0 错误 | ⬜ |
| T2.5.2 | `go vet ./...` 0 警告 | ⬜ |
| T2.5.3 | `go test -race ./internal/layers/orchestration/decisionplanning/...` 全 PASS | ⬜ |
| T2.5.4 | `go test -race ./internal/layers/contextengine/enforce/...` 全 PASS（D2 import 验证） | ⬜ |
| T2.5.5 | `go test -race ./internal/bootstrap/... ./internal/cli/... ./tests/integration/...` 全 PASS | ⬜ |
| T2.5.6 | `bash scripts/verify-archive.sh` 12/12 PASS | ⬜ |
| T2.5.7 | commit + push + 开 PR + auto-merge | ⬜ |

---

## PR-3: `d7spans` → `hardening` + `sessionqueue` → `executionflow` (结构扁平)

### Step 3.1: 分支

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T3.1.1 | 从 master 拉新分支 `feat/d7-package-cleanup-sprint-pr3-d7spans-sessionqueue` | ⬜ |
| T3.1.2 | 确认 PR-2 已合入 master | ⬜ |

### Step 3.2: PR-3a `d7spans` → `hardening` (先做)

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T3.2.1 | `git mv d7spans/emitter.go hardening/emitter.go` | ⬜ |
| T3.2.2 | `git mv d7spans/emitter_test.go hardening/emitter_test.go` | ⬜ |
| T3.2.3 | 2 个文件 `package d7spans` → `package hardening` | ⬜ |
| T3.2.4 | 5 个 D7 内部 importer 路径替换 | ⬜ |
| T3.2.5 | `d7spans.XXX` → `hardening.XXX` 引用替换 | ⬜ |
| T3.2.6 | 3 个 spec 同步（design.md:759, d7-domain.md:112, t-registry.md:435） | ⬜ |
| T3.2.7 | `go test -race ./internal/layers/orchestration/hardening/...` 全 PASS | ⬜ |

### Step 3.3: PR-3b `sessionqueue` → `executionflow` 父级 (扁平)

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T3.3.1 | `git mv sessionqueue/session_queue.go executionflow/session_queue.go` | ⬜ |
| T3.3.2 | `git mv sessionqueue/session_queue_test.go executionflow/session_queue_test.go` | ⬜ |
| T3.3.3 | `git mv sessionqueue/delegate_progress_test.go executionflow/delegate_progress_test.go` | ⬜ |
| T3.3.4 | 3 个文件 `package sessionqueue` → `package executionflow` | ⬜ |
| T3.3.5 | 新增 `executionflow/doc.go` 1 行注释说明 | ⬜ |
| T3.3.6 | 2 个 hub/ 内部 importer 删 import 行（变 in-package） | ⬜ |
| T3.3.7 | 4 个 bootstrap importer 路径替换 | ⬜ |
| T3.3.8 | 1 个跨域 testutil importer 路径替换 | ⬜ |
| T3.3.9 | 4 个 spec 同步（observability-guide.md, d7-requirements-clarifications.md, code-layout.md 多处） | ⬜ |

### Step 3.4: 验证 + 提交

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T3.4.1 | `go build ./...` 0 错误 | ⬜ |
| T3.4.2 | `go vet ./...` 0 警告 | ⬜ |
| T3.4.3 | `go test -race ./internal/layers/orchestration/...` 22/22 PASS | ⬜ |
| T3.4.4 | `go test -race ./internal/bootstrap/... ./tests/testutil/...` 全 PASS | ⬜ |
| T3.4.5 | `bash scripts/verify-archive.sh` 12/12 PASS | ⬜ |
| T3.4.6 | 验证 `ls internal/layers/orchestration/` 不含 4 个子包 | ⬜ |
| T3.4.7 | commit + push + 开 PR + auto-merge | ⬜ |

---

## S5 验收

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T5.1 | 编写 `acceptance-report.md` §1-§10 12 AC 验证 | ⬜ |
| T5.2 | 4 P0 T (D7-S6-A50 T01-T04) 状态 PLANNED → IMPLEMENTED | ⬜ |
| T5.3 | 更新 `openspec/specs/d7-orchestration/d7-domain.md` v2.2.0 → v2.3.0 目录结构图 | ⬜ |
| T5.4 | 更新 `openspec/t-registry.md` (root) v5.4.0 → v5.5.0 | ⬜ |

## S6 归档

| T# | 描述 | 状态 |
| -- | ---- | ---- |
| T6.1 | `gh pr ready` 触发 S4-Gate review | ⬜ |
| T6.2 | CI `unit tests` 通过 | ⬜ |
| T6.3 | `gh pr merge --auto --squash` 自动合入 master | ⬜ |
| T6.4 | 本地 `git pull origin master` 同步 | ⬜ |
| T6.5 | 移动 `openspec/changes/d7-package-cleanup-sprint/` → `openspec/archive/2026-06-25-d7-package-cleanup-sprint/` | ⬜ |
| T6.6 | 更新 `openspec/demand-archive-index.md` 新增 DM-20260625-018 行 | ⬜ |
| T6.7 | 运行 `./scripts/verify-archive.sh d7-package-cleanup-sprint` 12/12 PASS | ⬜ |

---

## 风险矩阵

| 风险 | 可能性 | 影响 | 缓解 |
|------|-------|------|------|
| in-package import 残留 | 高 | 编译失败 | 每 PR `go build ./...` 验证 |
| `enforce/contracts.go:14` 注释遗漏 | 低 | 无 | `rg "toolpolicy"` 二次扫描 |
| `audit-property-rights.sh` 兜底分支悬挂 | 中 | CI 失败 | PR-1 必删 |
| `CODEOWNERS` 孤儿行 | 中 | PR 被拦 | PR-1 必删 |
| `verify-archive.sh` 索引找不到子包 | 中 | CI 失败 | 更新索引表 |
| `executionflow` 父级从 0 Go 文件变 3 个 | 低 | 可读性 | 加 `doc.go` 说明 |

## 复用现有模式

- **devrix-d7-6s-package-merge** (DM-20260626-004, PR #220/#221) — turn/ → sessionorchestrator/ 物理合并模板
- **devrix-d7-6s-verify-promotion** (DM-20260626-005, PR #222/#223) — executionflow/verify/ promote 模板
- **devrix-d7-mups-package-migration** (DM-20260626-002, PR #216/#217) — 跨子树物理迁移模板
