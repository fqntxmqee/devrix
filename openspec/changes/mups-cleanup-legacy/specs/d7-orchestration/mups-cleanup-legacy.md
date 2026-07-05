# Spec: mups-cleanup-legacy

**Change ID:** `mups-cleanup-legacy`
**Demand:** DM-20260705-007
**Status:** S3_Design
**Created:** 2026-07-05

---

## 1. 目标

清理 MUPS 5 节点重构（M1+M2+M4+M5）期间的 byte-equivalent tripwire 死代码：
- 删除 2 个 `_legacy_test.go` 文件（`sessionorchestrator/verify_legacy_test.go` 307 行 + `workmodel/spawn_policy_legacy_test.go` 356 行 = 663 行）
- 删除 4 个旧符号（`verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy`）
- 删除 1 个 helper（`verdictEqual`，仅在 `verify_legacy_test.go` 使用）
- 删除 2 个 build tag（`legacy_verify` / `legacy_spawn`）

**0 行为变化承诺**：仅 test 编译路径删除，生产代码（`verify_decision_table.go` / `item_verify.go` / `item_pipeline_rollup.go` / `spawn_decision_algebra.go` / `spawn_policy.go`）0 修改。

---

## 2. 删除清单

| 文件 / 符号 | 类型 | 路径 | 行数 / 大小 |
|------------|------|------|-------------|
| `verify_legacy_test.go` | 文件 (test) | `sessionorchestrator/` | 307 行 |
| `spawn_policy_legacy_test.go` | 文件 (test) | `workmodel/` | 356 行 |
| `verifyArtifactLegacy` | 符号 (func) | `sessionorchestrator/verify_legacy_test.go:23` | 44 行 |
| `verifyArtifactForWorkItemWithContractLegacy` | 符号 (func) | `sessionorchestrator/verify_legacy_test.go:79` | 44 行 |
| `verifyRollupArtifactLegacy` | 符号 (func) | `sessionorchestrator/verify_legacy_test.go:137` | 39 行 |
| `SpawnPolicyEvaluatorLegacy` | 符号 (func) | `workmodel/spawn_policy_legacy_test.go:24` | 50+ 行 |
| `verdictEqual` | 符号 (helper) | `sessionorchestrator/verify_legacy_test.go:301` | 7 行 |
| `legacy_verify` | build tag | 2 文件顶部 | (注释 + 编译指令) |
| `legacy_spawn` | build tag | 1 文件顶部 | (注释 + 编译指令) |

**总计**：2 文件 + 4 旧符号 + 1 helper + 2 build tag = 663 行删除

---

## 3. 0 行为变化验证矩阵

| 验证维度 | 工具 | 期望 | 实际 |
|----------|------|------|------|
| workmodel 全套 -race -count=1 PASS | `go test ./internal/layers/orchestration/workmodel/ -race -count=1` | 22 现有 + 18 新增 = 40/40 PASS | ✅ |
| sessionorchestrator 全套 -race -count=1 PASS | `go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1` | 17 现有 + 13 新增 = 30/30 PASS | ✅ |
| `go vet ./...` 0 warning | `go vet ./...` | 0 warning | ✅ |
| 全文 race 0 新增 fail | `go test ./... -race -count=1` | 1 fail (pre-existing lint) 不变 | ✅ |
| build tag 0 hits | `grep -rn "legacy_verify\|legacy_spawn" --include="*.go" .` | 0 hits | ✅ |
| 旧符号 0 hits | `grep -rn "verifyArtifactLegacy\|..." --include="*.go" .` | 0 hits | ✅ |

---

## 4. 关联

- **Parent Demands**: DM-20260705-003 (M1) / DM-20260705-004 (M2) / DM-20260705-005 (M4) / DM-20260705-006 (M5)
- **Predecessor**: M4 §4.Q6 + M5 §4.Q7 明确记录本 change
- **Successor**: M3 d7-mups-strategy-injection (行为增量，最后做)
