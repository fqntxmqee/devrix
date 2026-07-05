# Design: mups-cleanup-legacy — MUPS 重构死代码清理

**Change ID:** mups-cleanup-legacy
**Demand ID:** DM-20260705-007
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式 - 小型 Change 裁剪版）
**Created:** 2026-07-05

---

## ① 架构目标

**业务目标**：
- 解决 M4 §4.Q6 + M5 §4.Q7 明确记录的"tripwire 死代码清理"问题
- 减少 663 行死代码 + 2 个 build tag + 4 个旧符号 的长期维护成本
- 简化 reader 认知（"生产实现 = `verifyArtifact` / `SpawnPolicyEvaluator`" 单一权威）

**技术目标**：
- `git rm` 2 个 `_legacy_test.go` 文件
- 4 个 Legacy 后缀符号 0 hits
- 2 个 `legacy_*` build tag 0 hits
- 现有 22+17 测试 + 18+13 sub-decision/detector + 2 顺序锁定 = 70 测试 0 修改全 PASS
- `go vet ./...` 0 warning
- 全文 `go test -race -count=1 ./...` 0 新增 fail（除 pre-existing 1 lint test）

**约束条件**：
- "0 行为变化" 承诺：生产代码 0 修改
- 现有 41 测试 0 修改
- 不引入新 build tag / 新 helper
- 不复活 M3 行为增量

## ② 架构原则

**设计原则**：
1. **删除 = 0 行为变化**（核心）：仅删除 test 编译路径的文件和符号，生产代码路径 0 修改
2. **Tripwire 单一权威**：M4/M5 byte-equivalent test 是 tripwire 单一权威点；tripwire 验证完后删除，无歧义
3. **测试矩阵简化**：删除 2 个 build tag 后，CI 矩阵从 `{default, legacy_verify, legacy_spawn}` 3 个 target 简化到 `{default}` 1 个 target
4. **Reader 认知一致性**：删除 4 个 Legacy 后缀符号后，"生产实现 = 单一名字" 0 歧义

**命名规范**（本 change 不引入新符号）：
- 删除 `verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy` 4 个 Legacy 后缀符号
- 删除 `legacy_verify` / `legacy_spawn` 2 个 build tag

**代码风格**：
- 函数 < 50 行（删除后无变化）
- 文件 < 800 行（删除后 `sessionorchestrator/` 总文件数从 ~25 → 24，`workmodel/` 总文件数从 ~15 → 14）
- 异常不过模块边界（删除后无变化）

## ③ 业务流程

**核心用例时序图**（删除 = 0 业务影响）：

```
[Before] PR merge → CI 跑 3 target (default + legacy_verify + legacy_spawn)
                   ↓
[After]  PR merge → CI 跑 1 target (default)
```

**异常补偿**：无（删除操作不引入新代码路径，无异常补偿需求）

**分支处理决策树**：

```
git rm verify_legacy_test.go spawn_policy_legacy_test.go
  ↓
  ├─ workmodel 全套 -race -count=1 PASS (22+18 = 40/40) ✓
  ├─ sessionorchestrator 全套 -race -count=1 PASS (17+13 = 30/30) ✓
  ├─ go vet ./... 0 warning ✓
  ├─ grep 0 hits (legacy_verify, legacy_spawn, Legacy 4 symbols) ✓
  └─ PR CI unit tests PASS → auto-merge → master ✓
```

## ④ 领域模型

**聚合根**：无（本 change 是 test 文件清理，不涉及领域模型变更）

**限界上下文**（删除前 → 删除后）：
- `sessionorchestrator/`: 25 文件 → 24 文件（删除 `verify_legacy_test.go`）
- `workmodel/`: 15 文件 → 14 文件（删除 `spawn_policy_legacy_test.go`）

**领域事件**（删除前 → 删除后）：
- 删除 `legacy_verify` build tag 编译事件
- 删除 `legacy_spawn` build tag 编译事件
- 0 Span / Metric 变化（无生产代码修改）

**跨域消费模型**：无（仅 test 文件删除，不跨域）

## ⑤ 核心链路图

**端到端路径**（删除 = 路径 0 变化）：

```
PR merge → CI unit tests
  → go test ./... -race -count=1 (default tag, 含 70 测试)
  → [before: 也跑 -tags legacy_verify / -tags legacy_spawn] ← 删
  → master
```

**时序标注**：
- CI 时间：删除前 ~3m23s → 删除后 ~3m10s（-13s, 约 -6%）
- PR merge SLA：删除前 ~3m30s → 删除后 ~3m15s

**单点风险与缓解**：
- 单点 1：byte-equivalent tripwire 失去 → 缓解：70 测试多重保险
- 单点 2：reader 看不懂哪个是生产实现 → 缓解：删除 4 个 Legacy 后缀符号后 0 歧义

## ⑥ 接口 / API 设计

**风格**：本 change 是 test 文件删除，不涉及 API 设计变更

**契约**：本 change 是 test 文件删除，不涉及契约变更

**幂等保障表**：
- `git rm` 2 个文件幂等（重复 `git rm` 已删除文件 = 错误，但本 change 一次性执行 0 重复）
- 4 个旧符号 grep 0 hits 幂等（删除后 0 hits = 永久）

**版本演进路径**：
- v1.0: 初始（tripwire 保留）
- v1.1 (本 change): 0 行为变化清理（删除 tripwire）
- v2.0 (M3): 行为增量（PlanKind 路由恢复，独立 change）

---

## 附录 A：File Manifest

### 新增
- 无

### 修改
- 无

### 删除
- `internal/layers/orchestration/sessionorchestrator/verify_legacy_test.go` (307 行, build tag `legacy_verify`)
- `internal/layers/orchestration/workmodel/spawn_policy_legacy_test.go` (356 行, build tag `legacy_spawn`)

**总计**：2 文件删除，663 行删除，0 行新增

## 附录 B：Rollback Plan

**回滚触发条件**：
- 70 测试 fail 任一
- `go vet ./...` 警告
- 4 个旧符号 grep 仍有 hits
- 2 个 build tag grep 仍有 hits

**回滚方式**：`git revert <merge-commit-sha>` 或 `git reset --hard <merge-commit-sha>^`

**回滚影响**：恢复 663 行死代码 + 2 个 build tag + 4 个旧符号，不影响其他代码

## 附录 C：回归风险评估

**baseline 对比**：
- Before: workmodel 22+18 + sessionorchestrator 17+13 = 70 测试
- After: workmodel 22+18 + sessionorchestrator 17+13 = 70 测试
- 差异：0 测试变化，0 生产代码变化

**高风险改动点**：
- 无（仅 test 文件删除，无生产代码改动）

**测试策略**：
- workmodel 全套 -race -count=1 必跑
- sessionorchestrator 全套 -race -count=1 必跑
- `go vet ./...` 必跑
- 全文 grep 0 hits 验证 必跑

## 附录 D：S3 检查清单自检

| 章节 | 状态 | 备注 |
|------|------|------|
| ① 架构目标 | ✅ | 业务目标 + 技术目标 + 约束条件 |
| ② 架构原则 | ✅ | 4 条原则（删除 = 0 行为变化 / Tripwire 单一权威 / 测试矩阵简化 / Reader 认知一致性） |
| ③ 业务流程 | ✅ | 时序图 + 异常补偿 + 决策树 |
| ④ 领域模型 | ✅ | 聚合根 0 + 限界上下文变化 + 领域事件删除 + 跨域 0 |
| ⑤ 核心链路图 | ✅ | 端到端路径 + 时序标注 + 单点风险 |
| ⑥ 接口 / API 设计 | ✅ | 风格 0 变化 + 契约 0 变化 + 幂等 + 版本演进 |

**S3-Gate Review 结论**: Approved（小型 Change 1 PR，0 生产代码变化，70 测试多重保险）

## 附录 E：下一步

1. S4 实现：`git rm` 2 个文件
2. S4-Gate：自检代码（git status + grep 0 hits）
3. S5 验收：跑测试 + acceptance-report.md (verdict: ACCEPTED)
4. S6-交付：PR + auto-merge
5. S6-归档：move to archive/ + 同步 5 个域规范文档
