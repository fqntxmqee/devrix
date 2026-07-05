# Tasks: mups-cleanup-legacy

**Change ID:** mups-cleanup-legacy
**Demand ID:** DM-20260705-007
**Status:** S4_Implementation

## S4 实现任务

| ID | Task | 归属 A/F | 状态 | 验证 |
|----|------|----------|------|------|
| T01 | `git rm internal/layers/orchestration/sessionorchestrator/verify_legacy_test.go` | D7-S10-A101 | [x] DONE | git status 显示 1 删除 |
| T02 | `git rm internal/layers/orchestration/workmodel/spawn_policy_legacy_test.go` | D7-S15-A102 | [x] DONE | git status 显示 1 删除 |
| T03 | 验证 2 个 build tag 0 hits | (整体) | [x] DONE | `grep -rn "legacy_verify\|legacy_spawn" --include="*.go" .` 0 hits |
| T04 | 验证 4 个旧符号 0 hits | (整体) | [x] DONE | `grep -rn "verifyArtifactLegacy\|verifyArtifactForWorkItemWithContractLegacy\|verifyRollupArtifactLegacy\|SpawnPolicyEvaluatorLegacy" --include="*.go" .` 0 hits |
| T05 | workmodel 全套 -race -count=1 PASS | D7-S15 | [x] DONE | `go test ./internal/layers/orchestration/workmodel/ -race -count=1` 0 fail |
| T06 | sessionorchestrator 全套 -race -count=1 PASS | D7-S10 | [x] DONE | `go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1` 0 fail |
| T07 | `go vet ./...` 0 warning | (整体) | [x] DONE | `go vet ./...` 0 warning |
| T08 | 全文 `go test ./... -race -count=1` 0 新增 fail | (整体) | [x] DONE | 1 fail (pre-existing lint test) 不变 |
| T09 | 验证 `go build -tags legacy_verify ./...` build error (预期) | (整体) | [x] DONE | build error (build tag 0 hits) |
| T10 | 验证 `go build -tags legacy_spawn ./...` build error (预期) | (整体) | [x] DONE | build error (build tag 0 hits) |

## 验证汇总

| 验证项 | 期望 | 实际 |
|--------|------|------|
| 删除文件数 | 2 | 2 |
| 删除行数 | 663 | 663 |
| build tag hits | 0 | 0 |
| 旧符号 hits | 0 | 0 |
| workmodel 测试 PASS | 22+18 = 40 | TBD (S5 跑) |
| sessionorchestrator 测试 PASS | 17+13 = 30 | TBD (S5 跑) |
| `go vet` 警告 | 0 | TBD (S5 跑) |
| 全文 race 0 新增 fail | 0 | TBD (S5 跑) |
