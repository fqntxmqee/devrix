# Tasks: D7 MUPS 包路径迁移

**Change ID:** `devrix-d7-mups-package-migration`
**Demand ID:** DM-20260626-002
**Status:** S4_Implemented
**Sprint:** d7-v6 follow-up
**PR Count:** 1
**前置:** devrix-d7-six-s-simplification (DM-20260626-001) S7_Archived (PR #215)

---

## 任务总览

| Phase | Task | 描述 | 工作量 | 状态 |
| ----- | ---- | ---- | ------ | ---- |
| **Step 1** | T1.1 | 创建 `internal/layers/orchestration/mups/` 父目录 | 0.05 天 | ⬜ |
| **Step 1** | T1.2 | 创建 `internal/layers/orchestration/mups/execute/` 子目录 | 0.05 天 | ⬜ |
| **Step 1** | T1.3 | `git mv orchestration/execute/*.go orchestration/mups/execute/` (7 .go) | 0.1 天 | ⬜ |
| **Step 1** | T1.4 | 验证 `mups/execute/` 包含 7 .go + `package execute` 声明不变 | 0.05 天 | ⬜ |
| **Step 1** | T1.5 | 创建 `internal/layers/orchestration/mups/learn/` 子目录 | 0.05 天 | ⬜ |
| **Step 1** | T1.6 | `git mv orchestration/learn/*.go orchestration/mups/learn/` (17 .go) | 0.1 天 | ⬜ |
| **Step 1** | T1.7 | 验证 `mups/learn/` 包含 17 .go + `package learn` 声明不变 | 0.05 天 | ⬜ |
| **Step 1** | T1.8 | 物理删除 `orchestration/execute/` + `orchestration/learn/` 目录（git rm） | 0.05 天 | ⬜ |
| **Step 1** | T1.9 | commit 1: "refactor(d7): mups package migration Step 1 — directory move" | 0.05 天 | ⬜ |
| **Step 2** | T2.1 | 全仓 `grep -rl "orchestration/learn\"" internal/ cmd/` 列出 17 个文件 | 0.05 天 | ⬜ |
| **Step 2** | T2.2 | 全仓 `sed -i 's\|orchestration/learn"\|orchestration/mups/learn"\|g'` 17 个文件 | 0.1 天 | ⬜ |
| **Step 2** | T2.3 | 验证 `grep -rln "orchestration/learn\"" internal/ cmd/` 返回 0 命中 | 0.05 天 | ⬜ |
| **Step 2** | T2.4 | 验证 `grep -rln "orchestration/execute\"" internal/ cmd/` 返回 0 命中 | 0.05 天 | ⬜ |
| **Step 2** | T2.5 | commit 2: "refactor(d7): mups package migration Step 2 — import path replacement" | 0.05 天 | ⬜ |
| **Step 3** | T3.1 | `go build ./...` 全仓编译 0 错误 | 0.1 天 | ⬜ |
| **Step 3** | T3.2 | `go vet ./...` 全仓静态检查 0 警告 | 0.1 天 | ⬜ |
| **Step 3** | T3.3 | `go test ./internal/layers/orchestration/... -race -count=1` 22/22 PASS | 0.2 天 | ⬜ |
| **Step 3** | T3.4 | LP-1/LP-2/LP-5 集成测试验证（Phase 6 + Phase 7 集成测试通过） | 0.1 天 | ⬜ |
| **Step 3** | T3.5 | commit 3: "refactor(d7): mups package migration Step 3 — build+vet+test green" | 0.05 天 | ⬜ |
| **Step 4** | T4.1 | 更新 `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 §MUPS 5 节点管道挂载章节 | 0.1 天 | ⬜ |
| **Step 4** | T4.2 | 更新 `openspec/specs/d7-orchestration/design.md` v4.0.0 §⑦ MUPS 5-node 6 S 归类 | 0.1 天 | ⬜ |
| **Step 4** | T4.3 | 更新 `openspec/specs/d7-orchestration/t-registry.md` v4.1.0 → v4.2.0（PLANNED → IMPLEMENTED） | 0.1 天 | ⬜ |
| **Step 4** | T4.4 | 更新 `openspec/t-registry.md` (root) v5.1.0 → v5.2.0（PLANNED → IMPLEMENTED） | 0.1 天 | ⬜ |
| **Step 4** | T4.5 | commit 4: "docs(openspec): mups package migration Step 4 — doc sync" | 0.05 天 | ⬜ |
| **S5** | T5.1 | 编写 `acceptance-report.md` §1-§5 全部 10 AC 验收 | 0.2 天 | ⬜ |
| **S5** | T5.2 | 4 新 P0 T (D7-S6-A51-T01..T04) 状态 PLANNED → IMPLEMENTED | 0.05 天 | ⬜ |
| **S5** | T5.3 | commit 5: "docs(openspec): mups package migration S5 acceptance" | 0.05 天 | ⬜ |
| **S6** | T6.1 | `gh pr ready` 触发 S4-Gate review | 0.05 天 | ⬜ |
| **S6** | T6.2 | CI `unit tests` 通过 | 0.1 天 | ⬜ |
| **S6** | T6.3 | `gh pr merge --auto --squash` 自动合入 master | 0.05 天 | ⬜ |
| **S6** | T6.4 | 本地 `git pull origin master` 同步最新 master | 0.05 天 | ⬜ |
| **S6 归档** | T7.1 | 移动 `openspec/changes/devrix-d7-mups-package-migration/` → `openspec/archive/2026-06-26-devrix-d7-mups-package-migration/` | 0.05 天 | ⬜ |
| **S6 归档** | T7.2 | 更新 `openspec/demand-archive-index.md` 新增 DM-20260626-002 行 | 0.05 天 | ⬜ |
| **S6 归档** | T7.3 | 编写 `openspec/archive/2026-06-26-devrix-d7-mups-package-migration/proposal.md` + `design.md` + `tasks.md` + `acceptance-report.md` (copy from changes/) | 0.1 天 | ⬜ |
| **S6 归档** | T7.4 | 编写 `openspec/archive/2026-06-26-devrix-d7-mups-package-migration/specs/d7-orchestration/spec.md` (copy from changes/) | 0.05 天 | ⬜ |
| **S6 归档** | T7.5 | 运行 `./scripts/verify-archive.sh devrix-d7-mups-package-migration` 11/11 PASS | 0.05 天 | ⬜ |
| **S6 归档** | T7.6 | commit 6: "chore(openspec): S6 archive devrix-d7-mups-package-migration" | 0.05 天 | ⬜ |

**总计**: ~2.5 天工作量（参考值，实际以实施为准）

---

## 实施步骤（commit-by-commit）

### Commit 1: Step 1 物理目录迁移 (T1.1 - T1.9)

```bash
# 创建 mups/ 父目录 + 两个子目录
mkdir -p internal/layers/orchestration/mups/execute
mkdir -p internal/layers/orchestration/mups/learn

# git mv execute/ 7 .go → mups/execute/
git mv internal/layers/orchestration/execute/*.go internal/layers/orchestration/mups/execute/

# git mv learn/ 17 .go → mups/learn/
git mv internal/layers/orchestration/learn/*.go internal/layers/orchestration/mups/learn/

# 物理删除旧目录
rmdir internal/layers/orchestration/execute internal/layers/orchestration/learn

# 验证
ls internal/layers/orchestration/mups/execute/  # 应有 7 .go files
ls internal/layers/orchestration/mups/learn/    # 应有 17 .go files
ls internal/layers/orchestration/execute 2>&1   # No such file or directory
ls internal/layers/orchestration/learn 2>&1    # No such file or directory
head -1 internal/layers/orchestration/mups/execute/channel.go  # package execute
head -1 internal/layers/orchestration/mups/learn/adaptive_prior.go  # package learn

# commit
git add -A
git commit -m "refactor(d7): execute/ + learn/ → mups/ 子树物理目录迁移 (DM-20260626-002 Step 1)

- git mv orchestration/execute/*.go (7 .go) → mups/execute/
- git mv orchestration/learn/*.go (17 .go) → mups/learn/
- package execute / package learn 声明 0 变化
- git history 保留 (--follow 可追溯)"
```

**预期 commit 影响**: 0 编译错误（import path 还未替换），仅文件移动。

### Commit 2: Step 2 import path 全仓替换 (T2.1 - T2.5)

```bash
# 列出会被修改的 17 个文件
grep -rln "orchestration/learn\"" internal/ cmd/

# 全仓 sed 替换
grep -rl "orchestration/learn\"" internal/ cmd/ | xargs sed -i \
  's|orchestration/learn"|orchestration/mups/learn"|g'

# 验证 0 残留
grep -rln "orchestration/learn\"" internal/ cmd/  # 必须 0 命中
grep -rln "orchestration/execute\"" internal/ cmd/  # 必须 0 命中 (execute 包本来 0 外部 import)

# commit
git add -A
git commit -m "refactor(d7): import path 全仓替换 orchestration/learn → mups/learn (DM-20260626-002 Step 2)

- 17 处 import path 替换: decisionplanning 2 + orchtypes 6 + sessionorchestrator 9
- execute 包 0 外部 import 跳过替换
- grep 0 残留验证通过"
```

**预期 commit 影响**: 编译错误（中间态）。Step 3 立即验证修复。

### Commit 3: Step 3 编译 + 测试回归 (T3.1 - T3.5)

```bash
# 编译验证
go build ./...  # 必须 0 错误

# 静态检查
go vet ./...  # 必须 0 警告

# 22 包 race 测试
go test ./internal/layers/orchestration/... -race -count=1  # 必须 22/22 PASS

# LP-1/LP-2/LP-5 集成测试（Phase 6 + Phase 7 覆盖）
go test ./internal/layers/orchestration/sessionorchestrator/... -race -run "TestAutoClose_FullLP1Loop"
go test ./internal/layers/orchestration/... -race -run "TestIntegration_5NodePipeline_End2End"

# commit
git add -A
git commit -m "refactor(d7): build/vet/test -race 全绿 22/22 PASS (DM-20260626-002 Step 3)

- go build ./... 0 错误
- go vet ./... 0 警告
- go test ./internal/layers/orchestration/... -race -count=1 22/22 PASS
- LP-1 (Bayesian reputation) / LP-2 (Memory 3 通道) / LP-5 (Cross-session traceability) 路径 0 变化"
```

### Commit 4: Step 4 文档同步 (T4.1 - T4.5)

```bash
# 更新 d7-domain.md §MUPS 5 节点管道挂载章节包路径描述
# (手动编辑 openspec/specs/d7-orchestration/d7-domain.md)

# 更新 design.md §⑦ MUPS 5-node 6 S 归类包路径描述
# (手动编辑 openspec/specs/d7-orchestration/design.md)

# 更新 t-registry.md (域): D7-S6-A51 T01..T04 状态 PLANNED → IMPLEMENTED
# + v4.1.0 → v4.2.0 Revision History 条目追加

# 更新 t-registry.md (root): v5.1.0 → v5.2.0 + 新增条目

# commit
git add -A
git commit -m "docs(openspec): mups package migration 文档同步 (DM-20260626-002 Step 4)

- d7-domain.md v2.0.0 §MUPS 5 节点管道挂载章节包路径描述更新
- design.md v4.0.0 §⑦ MUPS 5-node 6 S 归类包路径描述更新
- 域 t-registry.md v4.1.0 → v4.2.0 + D7-S6-A51 T01..T04 IMPLEMENTED
- root t-registry.md v5.1.0 → v5.2.0 + 新增量条目"
```

### Commit 5: S5 验收报告 (T5.1 - T5.3)

```bash
# 编写 acceptance-report.md
# (手动编辑 openspec/changes/devrix-d7-mups-package-migration/acceptance-report.md)

# commit
git add -A
git commit -m "docs(openspec): mups package migration S5 acceptance report (DM-20260626-002)

- acceptance-report.md §1-§5 全部 10 AC 验收
- 4 新 P0 T (D7-S6-A51-T01..T04) PLANNED → IMPLEMENTED"
```

### Commit 6: S6 归档 (T7.1 - T7.6)

```bash
# 移动到 archive
mv openspec/changes/devrix-d7-mups-package-migration openspec/archive/2026-06-26-devrix-d7-mups-package-migration

# 更新 demand-archive-index.md
# (手动追加 DM-20260626-002 行)

# verify-archive.sh
./scripts/verify-archive.sh devrix-d7-mups-package-migration  # 11/11 PASS

# commit
git add -A
git commit -m "chore(openspec): S6 archive devrix-d7-mups-package-migration (DM-20260626-002)

- 移动 changes/ → archive/2026-06-26-devrix-d7-mups-package-migration/
- demand-archive-index.md 新增 DM-20260626-002 行
- verify-archive.sh 11/11 PASS"
```

---

## 验证清单 (S5 验收)

- [ ] AC1: `mups/execute/` 目录创建，7 .go 迁移完成
- [ ] AC2: `mups/learn/` 目录创建，17 .go 迁移完成
- [ ] AC3: `grep "orchestration/execute\""` 0 命中
- [ ] AC4: `grep "orchestration/learn\""` 0 命中
- [ ] AC5: `go build ./...` 0 错误
- [ ] AC6: `go vet ./...` 0 警告
- [ ] AC7: `go test ./internal/layers/orchestration/... -race -count=1` 22/22 PASS
- [ ] AC8: `bootstrap/wire_coordinator.go` 若涉及 execute/learn 引用同步更新（0 引用，跳过）
- [ ] AC9: 4 新 P0 T (D7-S6-A51-T01..T04) 全部 IMPLEMENTED
- [ ] AC10: follow-up PR 列表 README 同步（follow-up #1 = 本次）

## 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| 17 个文件 import 替换遗漏 | 中 | grep 0 命中 + go build 0 错误是硬门禁 |
| 中间态编译失败 | 低 | Step 1 commit 1 (git mv) + Step 2 commit 2 (sed) + Step 3 commit 3 (test) 分离；中间态编译失败不影响 master（PR 未合入） |
| LP-1/LP-2/LP-5 行为漂移 | 极低 | 包名不变 + 函数签名不变；Phase 6 + Phase 7 集成测试覆盖 |
| CI 镜像缓存 | 低 | 删除旧目录后强制 re-build；CI unit tests 100% PASS 硬门禁 |