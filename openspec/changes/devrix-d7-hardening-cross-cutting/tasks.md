# Tasks: D7 hardening/ 横切包迁移

**Change ID:** `devrix-d7-hardening-cross-cutting`
**Demand ID:** DM-20260626-003
**Status:** S3_Design
**Sprint:** d7-v6 follow-up
**PR Count:** 1
**前置:** devrix-d7-six-s-simplification (DM-20260626-001) S7_Archived + devrix-d7-mups-package-migration (DM-20260626-002) S7_Archived

---

## 任务总览

| Phase | Task | 描述 | 工作量 | 状态 |
| ----- | ---- | ---- | ------ | ---- |
| **Step 1** | T1.1 | 创建 `internal/layers/orchestration/hardening/` 父目录 | 0.05 天 | ⬜ |
| **Step 1** | T1.2 | `git mv sessionorchestrator/metrics.go hardening/metrics.go` (61 行) | 0.05 天 | ⬜ |
| **Step 1** | T1.3 | `git mv sessionorchestrator/metrics_test.go hardening/metrics_test.go` | 0.05 天 | ⬜ |
| **Step 1** | T1.4 | `package sessionorchestrator` → `package hardening` (metrics.go + metrics_test.go 各 1 行) | 0.05 天 | ⬜ |
| **Step 1** | T1.5 | `git mv turn/recovery.go hardening/recovery.go` (133 行) | 0.05 天 | ⬜ |
| **Step 1** | T1.6 | 拆分 hardening/recovery.go: 保留 4 纯函数 + 1 const, 删除 receiver methods + partialStreamEmit + emitStreamRecoveryTombstones | 0.05 天 | ⬜ |
| **Step 1** | T1.7 | `package turn` → `package hardening` (recovery.go 1 行) | 0.05 天 | ⬜ |
| **Step 1** | T1.8 | `git mv turn/recovery_test.go hardening/recovery_test.go` + 拆分（3 纯函数测试迁 hardening/） | 0.05 天 | ⬜ |
| **Step 1** | T1.9 | turn/recovery.go 重新写入（保留 receiver methods + partialStreamEmit + emitStreamRecoveryTombstones + maxOutputTokenRecoveryAttempts + 加 import hardening） | 0.05 天 | ⬜ |
| **Step 1** | T1.10 | turn/recovery_test.go 重新写入（保留 3 orchestrator-coupled 测试 + recoveryStubLLM stub） | 0.05 天 | ⬜ |
| **Step 1** | T1.11 | 创建 `internal/layers/orchestration/hardening/doc.go` (~30 行说明 Discipline Keeper 职责) | 0.05 天 | ⬜ |
| **Step 1** | T1.12 | 物理删除 `sessionorchestrator/metrics.go` + `metrics_test.go` + `turn/recovery.go` + `recovery_test.go` (git rm 自动) | 0.05 天 | ⬜ |
| **Step 1** | T1.13 | commit 1: "refactor(d7): hardening package migration Step 1 — directory move" | 0.05 天 | ⬜ |
| **Step 2** | T2.1 | 全仓 `grep -rln "sessionorchestrator\.InterruptMetrics" internal/ cmd/` 列出引用文件 | 0.05 天 | ⬜ |
| **Step 2** | T2.2 | `sessionorchestrator/interrupt.go` Metrics 字段类型 `*InterruptMetrics` → `*hardening.InterruptMetrics` + 加 import | 0.05 天 | ⬜ |
| **Step 2** | T2.3 | `sessionorchestrator/interrupt_test.go` 加 import hardening + 引用更新 | 0.05 天 | ⬜ |
| **Step 2** | T2.4 | `turn/orchestrator.go` 加 import hardening + `NeedsMaxOutputTokenRecovery` → `hardening.NeedsMaxOutputTokenRecovery` + `MaxOutputTokensRecoveryMessage` → `hardening.MaxOutputTokensRecoveryMessage` (2 处) | 0.05 天 | ⬜ |
| **Step 2** | T2.5 | `turn/recovery.go` 加 import hardening + `IsContextLengthError` → `hardening.IsContextLengthError` (1 处) | 0.05 天 | ⬜ |
| **Step 2** | T2.6 | 验证 `grep -rln "sessionorchestrator\.InterruptMetrics" internal/ cmd/` 返回 0 命中 | 0.05 天 | ⬜ |
| **Step 2** | T2.7 | 验证 `grep -rln "turn\.IsContextLengthError" internal/ cmd/` 返回 0 命中 | 0.05 天 | ⬜ |
| **Step 2** | T2.8 | commit 2: "refactor(d7): hardening package migration Step 2 — import path replacement" | 0.05 天 | ⬜ |
| **Step 3** | T3.1 | `go build ./...` 全仓编译 0 错误 | 0.1 天 | ⬜ |
| **Step 3** | T3.2 | `go vet ./...` 全仓静态检查 0 警告 | 0.1 天 | ⬜ |
| **Step 3** | T3.3 | `go test ./internal/layers/orchestration/... -race -count=1` 23/23 PASS | 0.2 天 | ⬜ |
| **Step 3** | T3.4 | LP-1/LP-2/LP-5 集成测试验证（Phase 6 + Phase 7 集成测试通过） | 0.1 天 | ⬜ |
| **Step 3** | T3.5 | 验证 `escape/circuit_breaker.go` git diff 0 变化（Decision 1 不动） | 0.05 天 | ⬜ |
| **Step 3** | T3.6 | commit 3: "refactor(d7): hardening package migration Step 3 — build+vet+test green" | 0.05 天 | ⬜ |
| **Step 4** | T4.1 | 更新 `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 → v2.1.0 §① Cross-cutting Hardening 章节包路径描述 | 0.1 天 | ⬜ |
| **Step 4** | T4.2 | 更新 `openspec/specs/d7-orchestration/design.md` v4.0.0 → v4.1.0 §① Discipline Keeper 包路径描述 | 0.1 天 | ⬜ |
| **Step 4** | T4.3 | 更新 `openspec/specs/d7-orchestration/t-registry.md` v4.2.0 → v4.3.0（新增 D7-S7-A01-T01..T04） | 0.1 天 | ⬜ |
| **Step 4** | T4.4 | 更新 `openspec/t-registry.md` (root) v5.2.0 → v5.3.0（新增 DM-20260626-003 增量条目） | 0.1 天 | ⬜ |
| **Step 4** | T4.5 | commit 4: "docs(openspec): hardening package migration Step 4 — doc sync" | 0.05 天 | ⬜ |
| **S5** | T5.1 | 编写 `acceptance-report.md` §1-§9 全部 10 AC 验收 | 0.2 天 | ⬜ |
| **S5** | T5.2 | 4 新 P0 T (D7-S7-A01-T01..T04) 状态 PLANNED → IMPLEMENTED | 0.05 天 | ⬜ |
| **S5** | T5.3 | commit 5: "docs(openspec): hardening package migration S5 acceptance" | 0.05 天 | ⬜ |
| **S6** | T6.1 | `gh pr ready` 触发 S4-Gate review | 0.05 天 | ⬜ |
| **S6** | T6.2 | CI `unit tests` 通过 | 0.1 天 | ⬜ |
| **S6** | T6.3 | `gh pr merge --auto --squash` 自动合入 master | 0.05 天 | ⬜ |
| **S6** | T6.4 | 本地 `git pull origin master` 同步最新 master | 0.05 天 | ⬜ |
| **S6 归档** | T7.1 | 移动 `openspec/changes/devrix-d7-hardening-cross-cutting/` → `openspec/archive/2026-06-26-devrix-d7-hardening-cross-cutting/` | 0.05 天 | ⬜ |
| **S6 归档** | T7.2 | 更新 `openspec/demand-archive-index.md` 新增 DM-20260626-003 行 | 0.05 天 | ⬜ |
| **S6 归档** | T7.3 | 运行 `./scripts/verify-archive.sh devrix-d7-hardening-cross-cutting` 12/12 PASS | 0.05 天 | ⬜ |
| **S6 归档** | T7.4 | commit 6: "chore(openspec): S6 archive devrix-d7-hardening-cross-cutting" | 0.05 天 | ⬜ |

**总计**: ~2.5 天工作量（参考值，实际以实施为准）

---

## 实施步骤（commit-by-commit）

### Commit 1: Step 1 物理目录创建 + git mv (T1.1 - T1.13)

```bash
# 创建 hardening/ 父目录
mkdir -p internal/layers/orchestration/hardening

# git mv metrics.go + metrics_test.go (整文件迁)
git mv internal/layers/orchestration/sessionorchestrator/metrics.go \
       internal/layers/orchestration/hardening/metrics.go
git mv internal/layers/orchestration/sessionorchestrator/metrics_test.go \
       internal/layers/orchestration/hardening/metrics_test.go

# 修改 package 声明
sed -i '' 's|^package sessionorchestrator|package hardening|' \
  internal/layers/orchestration/hardening/metrics.go \
  internal/layers/orchestration/hardening/metrics_test.go

# 创建 hardening/doc.go
cat > internal/layers/orchestration/hardening/doc.go <<'EOF'
// Package hardening provides cross-cutting discipline keeping concerns
// for D7 orchestration: observability counters (metrics) and LLM error
// recovery helpers (recovery). It implements the "Discipline Keeper"
// 横切 (cross-cutting) role in the v6.0.0 6 S + 1 横切 layout.
//
// Architecture (v6.0.0):
//
//	6 S + 1 横切:
//	  S1 WorkModel         (State Authority)
//	  S2 SessionOrchestrator (Mediator + Turn Leader + Error Recovery)
//	  S3 WaveScheduler     (Mechanism Designer)
//	  S4 ExecutionFlow+Verify (Costly Signaler + Certifier)
//	  S5 DecisionPlanning+Observe (Info Producer + Quantizer)
//	  S6 MUPS Pipeline     (Pipeline Coordinator + Memory Curator)
//	  横切 Hardening         (Discipline Keeper) → THIS PACKAGE
//
// hardening/ contents:
//   - metrics.go: InterruptMetrics cross-canceller failure counters
//   - recovery.go: LLM error recovery helpers (context length, 5xx, output truncation)
//
// What hardening/ does NOT contain (intentional):
//   - escape.CircuitBreaker (5-Layer CB is core to EscapeEngine, see Decision 1)
//   - turn.Orchestrator (Mediator+Turn Leader is S2, see Decision 2)
package hardening
EOF

# 创建 hardening/recovery.go (subset of turn/recovery.go)
# (手写, 4 纯函数 + 1 const)
# 内容: IsContextLengthError + IsOverloadOr5xx + NeedsMaxOutputTokenRecovery + MaxOutputTokensRecoveryMessage

# 创建 hardening/recovery_test.go (subset of turn/recovery_test.go)
# (手写, 3 纯函数测试)

# 修改 turn/recovery.go (保留 receiver methods + struct + func)
# (手写, 4 内容: partialStreamEmit + emitStreamRecoveryTombstones + compressMessagesForRecovery + invokeStreamWithRecovery)

# 修改 turn/recovery_test.go (保留 orchestrator-coupled 测试 + recoveryStubLLM)
# (手写, 3 测试 + stub)

# 物理删除旧位置文件 (git mv 已自动 rename, 无需 rm)

# commit
git add -A
git commit -m "refactor(d7): hardening package migration Step 1 — directory move

- mkdir hardening/ + git mv sessionorchestrator/{metrics,metrics_test}.go + turn/{recovery,recovery_test}.go
- recovery.go subset split: 4 纯函数 + 1 const → hardening/ (receiver methods 留 turn/ Decision 2)
- package rename: sessionorchestrator/turn → hardening (2 文件 + recovery.go)
- hardening/doc.go NEW (Discipline Keeper 横切角色说明)
- escape/circuit_breaker.go 不动 (Decision 1 V5 EscapeEngine 核心机制)
- git history 保留 (--follow 可追溯)"
```

**预期 commit 影响**: 0 编译错误（import path 还未替换），仅文件移动。

### Commit 2: Step 2 import path + package prefix 全仓替换 (T2.1 - T2.8)

```bash
# 列出所有引用 metrics + recovery 的文件
grep -rln "sessionorchestrator\.InterruptMetrics" internal/ cmd/
grep -rln "turn\.IsContextLengthError\|turn\.IsOverloadOr5xx\|turn\.NeedsMaxOutputTokenRecovery\|turn\.MaxOutputTokensRecoveryMessage" internal/ cmd/

# 修改 sessionorchestrator/interrupt.go
# (手写: 加 import hardening + Metrics 字段类型改为 *hardening.InterruptMetrics)

# 修改 sessionorchestrator/interrupt_test.go
# (手写: 加 import hardening + 引用更新)

# 修改 turn/orchestrator.go
# (手写: 加 import hardening + 2 处 hardening. 前缀)

# 修改 turn/recovery.go
# (手写: 加 import hardening + 1 处 hardening.IsContextLengthError)

# 验证 0 残留
grep -rln "sessionorchestrator\.InterruptMetrics" internal/ cmd/   # 必须 0 命中
grep -rln "turn\.IsContextLengthError" internal/ cmd/               # 必须 0 命中
grep -rln "sessionorchestrator/metrics" internal/ cmd/             # 必须 0 命中
grep -rln "turn/recovery" internal/ cmd/                            # 必须 0 命中

# commit
git add -A
git commit -m "refactor(d7): hardening package migration Step 2 — import path + package prefix

- sessionorchestrator/interrupt.go: Metrics 字段类型 *InterruptMetrics → *hardening.InterruptMetrics (1 处) + 加 import
- sessionorchestrator/interrupt_test.go: 加 import hardening + 引用更新 (3 处)
- turn/orchestrator.go: 加 import hardening + NeedsMaxOutputTokenRecovery + MaxOutputTokensRecoveryMessage 加 hardening. 前缀 (2 处)
- turn/recovery.go: 加 import hardening + IsContextLengthError 加 hardening. 前缀 (1 处)
- grep 0 残留验证通过"
```

**预期 commit 影响**: 编译通过。

### Commit 3: Step 3 编译 + 测试回归 (T3.1 - T3.6)

```bash
# 编译验证
go build ./...  # 必须 0 错误

# 静态检查
go vet ./...  # 必须 0 警告

# 23 包 race 测试
go test ./internal/layers/orchestration/... -race -count=1  # 必须 23/23 PASS

# 验证 escape/circuit_breaker.go 未动
git diff HEAD~3 HEAD -- internal/layers/orchestration/escape/circuit_breaker.go  # 必须空

# LP-1/LP-2/LP-5 集成测试
go test ./internal/layers/orchestration/sessionorchestrator/... -race -run "TestAutoClose_FullLP1Loop"
go test ./internal/layers/orchestration/... -race -run "TestIntegration_5NodePipeline_End2End"

# commit
git add -A
git commit -m "refactor(d7): hardening package migration Step 3 — build+vet+test green

- go build ./... 0 错误
- go vet ./... 0 警告
- go test ./internal/layers/orchestration/... -race -count=1 23/23 PASS (含 hardening 1 新包)
- escape/circuit_breaker.go 0 变化 (git diff 空, Decision 1 不动)
- LP-1 (Bayesian reputation) / LP-2 (Memory 3 通道) / LP-5 (Cross-session traceability) 路径 0 变化"
```

### Commit 4: Step 4 文档同步 (T4.1 - T4.5)

```bash
# 更新 d7-domain.md §① Cross-cutting Hardening 章节包路径描述
# 更新 design.md §① Discipline Keeper 包路径描述
# 更新 t-registry.md (域): D7-S7-A01-T01..T04 状态 PLANNED → IMPLEMENTED + v4.2.0 → v4.3.0
# 更新 t-registry.md (root): v5.2.0 → v5.3.0 + 新增条目

git add -A
git commit -m "docs(openspec): hardening package migration Step 4 — doc sync

- d7-domain.md v2.0.0 → v2.1.0 §① Cross-cutting Hardening 章节包路径描述更新
- design.md v4.0.0 → v4.1.0 §① Discipline Keeper 包路径描述更新
- 域 t-registry.md v4.2.0 → v4.3.0 + D7-S7-A01 T01..T04 IMPLEMENTED
- root t-registry.md v5.2.0 → v5.3.0 + 新增量条目"
```

### Commit 5: S5 验收报告 (T5.1 - T5.3)

```bash
# 编写 acceptance-report.md
git add -A
git commit -m "docs(openspec): hardening package migration S5 acceptance report (DM-20260626-003)

- acceptance-report.md §1-§9 全部 10 AC 验收
- 4 新 P0 T (D7-S7-A01-T01..T04) PLANNED → IMPLEMENTED"
```

### Commit 6: S6 归档 (T7.1 - T7.4)

```bash
# 移动到 archive
mv openspec/changes/devrix-d7-hardening-cross-cutting openspec/archive/2026-06-26-devrix-d7-hardening-cross-cutting

# 更新 demand-archive-index.md
# verify-archive.sh
./scripts/verify-archive.sh devrix-d7-hardening-cross-cutting  # 12/12 PASS

# commit
git add -A
git commit -m "chore(openspec): S6 archive devrix-d7-hardening-cross-cutting (DM-20260626-003)"
```

---

## 验证清单 (S5 验收)

- [ ] AC1: hardening/ 目录创建，metrics.go + recovery.go 4 .go git mv 100%
- [ ] AC2: package hardening 声明在 4 文件保持一致
- [ ] AC3: 全仓 grep "sessionorchestrator/metrics" 0 命中
- [ ] AC4: 全仓 grep "turn/recovery" 0 命中（仅 hardening/recovery 命中）
- [ ] AC5: go build ./... 0 错误
- [ ] AC6: go vet ./... 0 警告
- [ ] AC7: go test ./internal/layers/orchestration/... -race -count=1 23/23 PASS
- [ ] AC8: 全仓 unit tests 通过
- [ ] AC9: LP-1/LP-2/LP-5 集成测试 100% 兼容
- [ ] AC10: escape/circuit_breaker.go 0 变化 (git diff 空)

## 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| interrupt.go Metrics 字段类型跨包引用 | 中 | Step 2 加 import + 字段类型更新 |
| recovery.go receiver method 跨包调用 hardening.IsContextLengthError | 中 | Step 2 turn/recovery.go 加 import hardening + 1 处前缀更新 |
| recovery_test.go 拆分 2 文件可能漏 import | 低 | Step 1 同步拆 + Step 3 测试验证 |
| hardening 包新创建无 doc.go | 低 | Step 1 同时创建 hardening/doc.go (Discipline Keeper 说明) |
| 中间态编译失败 | 低 | Step 1 (git mv) + Step 2 (sed) + Step 3 (test) 分离；PR 未合入不影响 master |
| LP-1/LP-2/LP-5 行为漂移 | 极低 | 0 函数逻辑变化，仅物理迁移 |
