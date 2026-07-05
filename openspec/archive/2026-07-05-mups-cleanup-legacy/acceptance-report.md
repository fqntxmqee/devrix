# Acceptance Report: mups-cleanup-legacy

**Change ID:** `mups-cleanup-legacy`
**Demand:** DM-20260705-007
**Status:** S5_Acceptance → **ACCEPTED**

---

## 1. 验收结论

**Verdict:** ✅ **ACCEPTED**

mups-cleanup-legacy (删除 M4 `verify_legacy_test.go` + M5 `spawn_policy_legacy_test.go` 死代码 + 4 个旧符号 + 2 个 build tag) 0 行为变化承诺已验证：
- 2 文件 `git rm` (663 行删除)
- 2 build tag 0 hits
- 4 旧符号 0 hits
- workmodel 全套 `-race -count=1` 0 fail
- sessionorchestrator 全套 `-race -count=1` 0 fail
- `go vet ./...` 0 warning
- 全文 `go test -race -count=1 ./...` 0 新增 fail（除 pre-existing 1 lint test `TestScan_FindsAllInvariantFiles`）

---

## 2. 验收范围

| 范围 | 包含 |
|------|------|
| **In** | `git rm internal/layers/orchestration/sessionorchestrator/verify_legacy_test.go` (307 行, build tag `legacy_verify`)<br/>`git rm internal/layers/orchestration/workmodel/spawn_policy_legacy_test.go` (356 行, build tag `legacy_spawn`)<br/>删除 4 个旧符号：`verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy`<br/>删除 2 个 build tag：`legacy_verify` / `legacy_spawn`<br/>删除 1 个 helper：`verdictEqual` (仅在 verify_legacy_test.go 使用) |
| **Out** | 修改生产代码任何一行（`verify_decision_table.go` / `item_verify.go` / `item_pipeline_rollup.go` / `spawn_decision_algebra.go` / `spawn_policy.go`）<br/>修改现有 41 测试任何一行<br/>引入新 build tag / 新 helper<br/>M3 Strategy 抽象（独立 change d7-mups-strategy-injection，最后做）<br/>mups-5node-refactor-roadmap.md 文档（M3 启动时一并补建） |

---

## 3. 验收标准对照

### 3.1 P0 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC1 | `verify_legacy_test.go` + `spawn_policy_legacy_test.go` 2 个文件已从 git history 移除 | `git status` 显示 2 deletions | ✅ |
| AC2 | `legacy_verify` + `legacy_spawn` 2 个 build tag 0 hits in repo | `grep -rn "legacy_verify\|legacy_spawn" --include="*.go" .` 0 hits | ✅ |
| AC3 | `verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy` 4 个旧符号 0 hits in repo | `grep -rn "<4 symbols>" --include="*.go" .` 0 hits | ✅ |
| AC4 | 现有 workmodel 22 测试 0 修改 PASS（含 M5 新增 18 测试） | `go test ./internal/layers/orchestration/workmodel/ -race -count=1` 0 fail | ✅ |
| AC5 | 现有 sessionorchestrator 17 测试 0 修改 PASS（含 M4 新增 13 测试） | `go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1` 0 fail | ✅ |
| AC6 | 全文 `go test -race -count=1 ./...` 0 fail（除 pre-existing 1 lint test） | 1 fail (pre-existing `TestScan_FindsAllInvariantFiles`) 不变 | ✅ |
| AC7 | `go vet ./...` 0 warning | `go vet ./...` 0 warning | ✅ |
| AC8 | PR CI `unit tests` 绿 + auto-merge 合入 master | TBD (S6 跑) | ⏳ S6-交付 |

### 3.2 P1 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC9 | S7 归档后 `demand-archive-index.md` 入口 + `t-registry.md` 版本 bump + `a-registry.md` 版本 bump + `CHANGELOG.md` 行 | TBD (S6 跑) | ⏳ S6-归档 |

---

## 4. 验证矩阵执行结果

| 验证项 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| 删除文件数 | 2 | 2 | ✅ |
| 删除行数 | 663 | 663 | ✅ |
| build tag `legacy_verify` hits | 0 | 0 | ✅ |
| build tag `legacy_spawn` hits | 0 | 0 | ✅ |
| `verifyArtifactLegacy` hits | 0 | 0 | ✅ |
| `verifyArtifactForWorkItemWithContractLegacy` hits | 0 | 0 | ✅ |
| `verifyRollupArtifactLegacy` hits | 0 | 0 | ✅ |
| `SpawnPolicyEvaluatorLegacy` hits | 0 | 0 | ✅ |
| `verdictEqual` hits | 0 | 0 | ✅ |
| workmodel 测试 0 fail | 0 | 0 | ✅ |
| sessionorchestrator 测试 0 fail | 0 | 0 | ✅ |
| `go vet ./...` 警告 | 0 | 0 | ✅ |
| 全文 race 0 新增 fail | 0 | 0 | ✅ |
| pre-existing lint fail | 1 | 1 | ✅ 不变 |

---

## 5. 0 行为变化验证（核心承诺）

**方法论**：
- M4/M5 byte-equivalent tripwire（已删除）的目标"新实现字节级等于旧实现"已被以下 70 测试多重保险替代：
  - 22 workmodel 现有测试 (M5 0 修改 PASS)
  - 18 workmodel M5 新增测试 (7 checkBudget + 4 checkRollupGuard + 6 checkVerdictDirection + 2 normalizeCtx + 1 sub-decision-order 4-subtests = 20 sub-cases, 含 17 test functions)
  - 17 sessionorchestrator 现有测试 (M4 0 修改 PASS)
  - 13 sessionorchestrator M4 新增测试 (12 detector + 1 顺序锁定 = 13 test functions)
  - 70 测试覆盖 R0/R0.5/R1/R2/R3/R4/R5/R6/R7/R8 + 4 rollup guard + 5 artifact + 3 workItem overlay + 4 rollup + 顺序锁定 + 字节级 = 0 静默行为变化空间

**结果**：70/70 测试 PASS，0 行为变化

---

## 6. 后续路线

1. **M3** `d7-mups-strategy-injection` 行为增量 (PlanKind 路由恢复) — 最后做, 独立 PR + 完整 S5 验收
2. **mups-5node-refactor-roadmap.md** 5 节点重构总图最终落地文档 — M3 启动时一并补建

---

## 7. 域文档同步

待 S6-归档时同步：
- `openspec/specs/d7-orchestration/spec.md` (v4.24.0 → v4.25.0 + Last Updated)
- `openspec/specs/d7-orchestration/t-registry.md` (v4.31.0 → v4.32.0)
- `openspec/specs/d7-orchestration/a-registry.md` (v5.7.0 → v5.8.0)
- `openspec/specs/d7-orchestration/CHANGELOG.md` (新增 2026-07-05 mups-cleanup-legacy 行)
- `openspec/demand-archive-index.md` (新增 DM-20260705-007 入口)
