# D7 MUPS Pipeline — mups Package Migration Spec

**Module:** D7 Orchestration / S6 MUPS Pipeline
**Change:** `devrix-d7-mups-package-migration` (DM-20260626-002)
**Status:** S3_Design
**Spec Version:** v1.0
**依赖:** devrix-d7-six-s-simplification (DM-20260626-001) S7_Archived

---

## ADDED

### Requirement: mups/execute Package Directory Exists

`internal/layers/orchestration/mups/execute/` 目录必须存在，包含原 `orchestration/execute/` 全部 7 个 .go 文件，`package execute` 声明保持不变。

<!-- T: D7-S6-A51-T01 -->

#### Scenario: mups/execute directory contains all 7 original .go files

- GIVEN 原 `internal/layers/orchestration/execute/` 目录包含 7 个 .go 文件（channel.go + channel_commit.go + channel_exploration.go + channel_protocol.go + channel_scenario.go + errors.go + execute_test.go）
- AND `package execute` 声明在所有 7 个文件中
- WHEN 执行 `git mv internal/layers/orchestration/execute/*.go internal/layers/orchestration/mups/execute/`
- AND 物理删除 `internal/layers/orchestration/execute/` 目录
- THEN `internal/layers/orchestration/mups/execute/` 目录存在
- AND 包含 7 个 .go 文件（与原文件一一对应，文件名不变）
- AND 每个文件 `package execute` 声明不变
- AND `git log --follow` 能追溯到原文件的 commit 历史（保留 git history）

---

### Requirement: mups/learn Package Directory Exists

`internal/layers/orchestration/mups/learn/` 目录必须存在，包含原 `orchestration/learn/` 全部 17 个 .go 文件（含 8 个 _test.go），`package learn` 声明保持不变。

<!-- T: D7-S6-A51-T02 -->

#### Scenario: mups/learn directory contains all 17 original .go files

- GIVEN 原 `internal/layers/orchestration/learn/` 目录包含 17 个 .go 文件（含 8 个 _test.go: adaptive_prior.go + asset_builder.go + asset_content.go + learner.go + learning_asset.go + memory.go + reputation_evidence.go + reputation_store.go + 9 _test.go）
- AND `package learn` 声明在所有 17 个文件中
- WHEN 执行 `git mv internal/layers/orchestration/learn/*.go internal/layers/orchestration/mups/learn/`
- AND 物理删除 `internal/layers/orchestration/learn/` 目录
- THEN `internal/layers/orchestration/mups/learn/` 目录存在
- AND 包含 17 个 .go 文件（与原文件一一对应，文件名不变）
- AND 每个文件 `package learn` 声明不变
- AND `git log --follow` 能追溯到原文件的 commit 历史（保留 git history）

---

### Requirement: Zero Residual Old-Name Imports

全仓 `grep -rl "orchestration/execute\""` 和 `grep -rl "orchestration/learn\""` 必须 0 命中；15 处外部 import 全部更新为 `orchestration/mups/learn"`。

<!-- T: D7-S6-A51-T03 -->

#### Scenario: All external imports of orchestration/learn/ are migrated

- GIVEN 15 处外部 import 引用 `internal/layers/orchestration/learn"`：
  - `decisionplanning/classifier.go` (1 处)
  - `orchtypes/` 4 文件 + 4 _test.go (8 处)
  - `sessionorchestrator/` 3 文件 + 7 _test.go (10 处)
- WHEN 执行 `grep -rl "internal/layers/orchestration/learn\"" internal/ cmd/ | xargs sed -i 's|internal/layers/orchestration/learn"|internal/layers/orchestration/mups/learn"|g'`
- THEN `grep -rln "internal/layers/orchestration/learn\"" internal/ cmd/` 返回 0 命中
- AND `grep -rln "internal/layers/orchestration/mups/learn\"" internal/ cmd/` 返回 15 命中
- AND execute 包 0 外部 import，跳过替换步骤

#### Scenario: execute package has zero external imports

- GIVEN 原 `internal/layers/orchestration/execute/` 包
- WHEN 执行 `grep -rln "internal/layers/orchestration/execute\"" internal/ cmd/`
- THEN 返回 0 命中（execute 包仅自身 `execute_test.go` 使用）

#### Scenario: orchtypes test fixtures use new import path

- GIVEN `internal/layers/orchestration/orchtypes/` 包含 4 个 _test.go 文件（anomaly_detector_test.go + intent_quantizer_test.go + observe_request_test.go + process_test.go）
- AND 这些测试文件原本 import `internal/layers/orchestration/learn"`
- WHEN 完成全仓 import path 替换
- THEN 4 个 _test.go 文件全部使用 `internal/layers/orchestration/mups/learn"`
- AND 测试 fixture 通过 `go test ./internal/layers/orchestration/orchtypes/...` 验证

#### Scenario: sessionorchestrator test fixtures use new import path

- GIVEN `internal/layers/orchestration/sessionorchestrator/` 包含 7 个 _test.go 文件
- AND 这些测试文件原本 import `internal/layers/orchestration/learn"`
- WHEN 完成全仓 import path 替换
- THEN 7 个 _test.go 文件全部使用 `internal/layers/orchestration/mups/learn"`
- AND 测试 fixture 通过 `go test ./internal/layers/orchestration/sessionorchestrator/... -race` 验证

---

### Requirement: Build, Vet, Test All Green

`go build ./...` 0 错误 + `go vet ./...` 0 警告 + `go test ./internal/layers/orchestration/... -race -count=1` 22/22 PASS。

<!-- T: D7-S6-A51-T04 -->

#### Scenario: go build returns 0 errors

- GIVEN 完成物理目录迁移 + 全仓 import path 替换
- WHEN 执行 `go build ./...`
- THEN 返回 exit code 0
- AND stdout/stderr 无编译错误

#### Scenario: go vet returns 0 warnings

- GIVEN 完成物理目录迁移 + 全仓 import path 替换
- WHEN 执行 `go vet ./...`
- THEN 返回 exit code 0
- AND stdout 无 vet 警告

#### Scenario: go test -race passes 22/22 orchestration packages

- GIVEN 完成物理目录迁移 + 全仓 import path 替换
- WHEN 执行 `go test ./internal/layers/orchestration/... -race -count=1`
- THEN 返回 22/22 包 PASS
- AND 0 race condition detected
- AND 与 baseline 持平（PR #215 验证的 22 包 baseline）

#### Scenario: LP-1/LP-2/LP-5 paths are unchanged

- GIVEN 完成物理目录迁移
- WHEN 检查 LP-1（Bayesian reputation）→ LP-2（Memory 3 通道）→ LP-5（Cross-session traceability）三条核心数据流
- THEN 三条路径全部 0 变化
- AND Phase 6 + Phase 7 集成测试（TestAutoClose_FullLP1Loop + TestIntegration_5NodePipeline_End2End）全部通过

## MODIFIED

(None)

## REMOVED

(None)