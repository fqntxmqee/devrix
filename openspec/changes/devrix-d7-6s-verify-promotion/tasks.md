# Tasks: D7 verify-promotion

**Change ID:** devrix-d7-6s-verify-promotion
**Status:** S3_Design
**Priority:** P1
**Created:** 2026-06-26
**DM:** DM-20260626-005

---

## Phase 1: Spec 同步 (前置, 0.5h)

### Task 1.1: d7-domain.md v2.2.0 → v2.3.0

- [ ] 更新 `**Version:** 2.2.0` → `**Version:** 2.3.0`
- [ ] 更新 `**Last Updated:** 2026-06-26` → `**Last Updated:** 2026-06-26 (verify-promotion)`
- [ ] 在 `§DSAFT 资产` 表 S4 ExecutionFlow + Verify 行追加 `executionflow/verify/exit_reason.go + verdict_to_exit_reason.go` 描述
- [ ] 新增 `v2.3.0` changelog row:
  ```
  | **2.3.0** | **2026-06-26** | **verify-promotion**（DM-20260626-005）：(1) `sessionorchestrator/{exit_reason,verdict_to_exit_reason,verdict_to_exit_reason_test}.go` 3 文件 git mv → `executionflow/verify/`（218 行）；(2) `sessionorchestrator/turn_orchestrator.go` 11 处 `ExitReason*` 引用改为 `verify.ExitReason*`；(3) S4 Certifier 角色的可验证承诺 (14 ExitReason + VerdictToExitReason 4 态映射) 在 spec/code 归属对齐；(4) D7-S4-A50 4 新 P0 T IMPLEMENTED（D7-S4-A50-T01..T04）→ 域 t-registry v4.5.0 / 根 v5.5.0；(5) 0 函数签名变化 (pure physical migration) + 22/22 orchestration packages go test -race PASS
  ```

### Task 1.2: design.md v4.2.0 → v4.3.0

- [ ] 更新 `**Version:** 4.2.0` → `**Version:** 4.3.0`
- [ ] 更新 `**Last Updated:** 2026-06-26` → `**Last Updated:** 2026-06-26 (verify-promotion)`
- [ ] 在 §1.2 D7 6 S + 1 横切 物理包归属 S4 ExecutionFlow + Verify 行追加 `executionflow/verify/` 包含 `exit_reason.go + verdict_to_exit_reason.go + verdict_to_exit_reason_test.go` (218 行)
- [ ] 在 §"Verify" 章节更新 `VerdictToExitReason` 实现位置: `sessionorchestrator/verdict_to_exit_reason.go::VerdictToExitReason` → `executionflow/verify/verdict_to_exit_reason.go::VerdictToExitReason`
- [ ] 新增 v4.3.0 changelog row

### Task 1.3: t-registry.md (D7 域) v4.4.0 → v4.5.0

- [ ] 更新 `**Version:** 4.4.0` → `**Version:** 4.5.0`
- [ ] 更新 `**Last Updated:** 2026-06-26` → `**Last Updated:** 2026-06-26 (verify-promotion)`
- [ ] 在 D7-S4-A50 章节新增 4 P0 T (T01-T04), 全部 IMPLEMENTED:
  - D7-S4-A50-T01: `sessionorchestrator/{exit_reason,verdict_to_exit_reason,verdict_to_exit_reason_test}.go` 3 文件 `git mv` 到 `executionflow/verify/`
  - D7-S4-A50-T02: 3 文件 `package sessionorchestrator` → `package verify` + 11 处 `ExitReason*` 跨包引用替换为 `verify.ExitReason*`
  - D7-S4-A50-T03: executionflow/verify/ 包 0 sessionorchestrator 反向依赖 + 跨包 import cycle 0 风险
  - D7-S4-A50-T04: `go build/vet/test -race` 22/22 orchestration packages 全绿 + LP-1/2/5 集成测试 100% 兼容 + `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` git diff 0 变化
- [ ] 更新 §Statistics: 域 t-registry 218 IMPLEMENTED → 222 IMPLEMENTED, P0 181 → 185

### Task 1.4: t-registry.md (root) v5.4.0 → v5.5.0

- [ ] 更新 `**Version:** 5.4.0` → `**Version:** 5.5.0`
- [ ] 更新 `**Last Updated:** 2026-06-26` → `**Last Updated:** 2026-06-26 (verify-promotion)`
- [ ] 在 §Overview 域级注册表 D7 Orchestration 行更新 Total 214 → 218, IMPLEMENTED 214 → 218, P0 181 → 185
- [ ] 更新 §Overview "**总计**": 536 → 540, IMPLEMENTED 531 → 535, P0 350 → 354
- [ ] 新增 DM-20260626-005 增量条目:
  ```
  > 2026-06-26 增量（verify-promotion 包归属迁移 IMPLEMENTED 收口）：DM-20260626-005 (devrix-d7-6s-verify-promotion) — v6.0.0 域升级 Step 5 follow-up 落地：sessionorchestrator/ 包内临时留存的 exit_reason.go (72 行) + verdict_to_exit_reason.go (49 行) + verdict_to_exit_reason_test.go (97 行) 3 文件 git mv → executionflow/verify/（218 行）+ package sessionorchestrator → package verify + sessionorchestrator/turn_orchestrator.go 11 处 ExitReason* → verify.ExitReason* + turn_orchestrator_test.go 2 处 ExitReasonNatural → verify.ExitReasonNatural。D7 加 4 个 P0 T 全部 IMPLEMENTED：D7-S4-A50-T01 3 文件 git mv + git log --follow 100% rename detection / D7-S4-A50-T02 3 文件 package 改名 + 13 处 ExitReason* 跨包引用全替换 + grep 0 残留 / D7-S4-A50-T03 executionflow/verify/ 包 0 sessionorchestrator 反向依赖 + 跨包 cycle 0 风险 / D7-S4-A50-T04 go build/vet/test -race 22/22 orchestration pkgs + LP-1/LP-2/LP-5 集成测试 100% 兼容 + hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化。Total 540, P0 354 (IMPLEMENTED 531→535, P0 350→354)。D7 t-registry v4.4.0 → v4.5.0。22 包 baseline 持平（DM-20260626-004 PR #220+#221 验证），0 函数签名变化 (pure physical migration)。
  ```

## Phase 2: 物理迁移 (1h)

### Task 2.1: git mv 3 文件

- [ ] `git mv internal/layers/orchestration/sessionorchestrator/exit_reason.go internal/layers/orchestration/executionflow/verify/exit_reason.go`
- [ ] `git mv internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go`
- [ ] `git mv internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go`

### Task 2.2: sed package 改名 (3 文件)

- [ ] `sed -i '' '1s|package sessionorchestrator|package verify|' internal/layers/orchestration/executionflow/verify/exit_reason.go`
- [ ] `sed -i '' '1s|package sessionorchestrator|package verify|' internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go`
- [ ] `sed -i '' '1s|package sessionorchestrator|package verify|' internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go`

### Task 2.3: 验证迁移成功

- [ ] `head -1 internal/layers/orchestration/executionflow/verify/exit_reason.go` 输出 `package verify`
- [ ] `head -1 internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go` 输出 `package verify`
- [ ] `head -1 internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go` 输出 `package verify`
- [ ] `ls internal/layers/orchestration/sessionorchestrator/ | grep -E "exit_reason|verdict_to_exit"` 输出空 (3 文件已移出 sessionorchestrator/)

## Phase 3: 跨包引用更新 (1h)

### Task 3.1: sessionorchestrator/turn_orchestrator.go 加 import

- [ ] 在 import block 中 `executionflow/bridge` 后追加 `"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"`
- [ ] 验证 `goimports` 后 import block 顺序符合 goimports 标准 (group: stdlib + 3rd party + internal)

### Task 3.2: 11 处 ExitReason* 替换

- [ ] sed 替换 (含单词边界, 避免误替换 exitReason 变量名):
  ```bash
  sed -i '' -E 's|\bExitReason\b|verify.ExitReason|g; s|\bExitReason([A-Z][a-zA-Z]*)\b|verify.ExitReason\1|g' \
    internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go
  ```
- [ ] 验证替换覆盖 (预期 11 处):
  - line 222: `exitReason verify.ExitReason` (state 字段类型)
  - line 270: `st.exitReason = verify.ExitReasonMaxTurns`
  - line 513: `exitReason: verify.ExitReasonNatural,`
  - line 743: `st.exitReason = verify.ExitReasonNatural`
  - line 753: `st.exitReason = verify.ExitReasonRepeatedTool`
  - line 786: `st.exitReason = verify.ExitReasonToolFailure`
  - line 803: `st.exitReason = verify.ExitReasonTokenDiminishing`
  - line 864: `func resolveFinalText(... exitReason verify.ExitReason, maxTurns int)`
  - line 870: `if exitReason == verify.ExitReasonMaxTurns && maxTurns > 0`
  - line 910: `exitReason verify.ExitReason,` (makeCompletionMessage 参数类型)
  - line 921: `metadataKeyExitReason: string(exitReason),` (不变, string conversion 无类型引用)
- [ ] 残留验证: `grep -n "ExitReason" internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go | grep -v "verify\."` 输出空

### Task 3.3: sessionorchestrator/turn_orchestrator_test.go 2 处替换

- [ ] sed 替换:
  ```bash
  sed -i '' 's|ExitReasonNatural|verify.ExitReasonNatural|g' \
    internal/layers/orchestration/sessionorchestrator/turn_orchestrator_test.go
  ```
- [ ] 验证替换覆盖 (预期 2 处, line 1386 + 1393)

### Task 3.4: 残留验证

- [ ] `grep -rn "ExitReason[^a-zA-Z]" internal/layers/orchestration/sessionorchestrator/*.go | grep -v "// " | grep -v "verify\."` 输出空
- [ ] `grep -rn "sessionorchestrator\." internal/layers/orchestration/executionflow/verify/exit_reason.go internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason.go internal/layers/orchestration/executionflow/verify/verdict_to_exit_reason_test.go` 输出空 (无反向依赖)

## Phase 4: 验证 (1.5h)

### Task 4.1: 编译 + Lint + 单元测试

- [ ] `go build ./...` 0 错误
- [ ] `go vet ./...` 0 警告
- [ ] `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS 0 race
- [ ] `go test -race -count=1 ./internal/layers/orchestration/executionflow/verify/...` 6/6 PASS (5 测试函数 + anomaly_test.go)

### Task 4.2: 集成测试 (LP-1/2/5 兼容)

- [ ] `go test -race -count=1 ./tests/integration/d7/...` baseline 持平
- [ ] LP-1 (Bayesian reputation TestAutoClose_FullLP1Loop) PASS
- [ ] LP-2 (5 节点 TestIntegration_5NodePipeline_End2End) PASS
- [ ] LP-5 (Cross-session traceability) PASS

### Task 4.3: Baseline stability 验证

- [ ] `git diff -- internal/layers/orchestration/sessionorchestrator/autoclose.go` 输出空
- [ ] `git diff -- internal/layers/orchestration/hardening/` 输出空
- [ ] `git diff -- internal/layers/orchestration/escape/circuit_breaker.go` 输出空

### Task 4.4: Cross-package import DAG 验证

- [ ] `go list -deps ./internal/layers/orchestration/executionflow/verify | grep sessionorchestrator` 输出空 (单向 DAG: sessionorchestrator → verify, 无反向)
- [ ] `go list -deps ./internal/layers/orchestration/sessionorchestrator | grep executionflow/verify` 命中 (确认 sessionorchestrator 引用 verify)

### Task 4.5: sessionorchestrator/ 包内 ExitReason 残留 0

- [ ] `grep -rn "^\s*ExitReason\b\|^\s*ExitReason[A-Z]" internal/layers/orchestration/sessionorchestrator/*.go` 输出空 (无顶层 ExitReason 类型引用)

## Phase 5: S4-Gate PR + Auto-Merge (0.5h)

### Task 5.1: 分支 + commit + push

- [ ] `git checkout -b feat/devrix-d7-6s-verify-promotion`
- [ ] `git add -A` (5 modified .go + 4 modified .md + 3 renamed .go + 1 created .openspec.yaml)
- [ ] `git commit -m "refactor(d7): exit_reason + verdict_to_exit_reason promote sessionorchestrator/ → executionflow/verify/ (DM-20260626-005)\n\n[详细 body 见 PR description]"`
- [ ] `git push -u origin feat/devrix-d7-6s-verify-promotion`

### Task 5.2: PR 创建 + auto-merge

- [ ] `gh pr create --title "refactor(d7): exit_reason promote to executionflow/verify/ (DM-20260626-005)" --body "..."`
- [ ] `gh pr merge <PR#> --auto --squash --delete-branch`
- [ ] 等 CI unit tests (3min) + layer-lint (10s) 全 PASS
- [ ] squash auto-merge 完成, 分支删除

## Phase 6: S5 验收报告 (1h)

### Task 6.1: acceptance-report.md 创建

- [ ] 创建 `openspec/changes/devrix-d7-6s-verify-promotion/acceptance-report.md`, 13 AC × 9 sections:
  1. §1 摘要 (3 文件 promote, 0 函数签名变化)
  2. §2 T 层验证 (D7-S4-A50-T01..T04 全 IMPLEMENTED)
  3. §3 22 包 baseline 回归 (22/22 PASS)
  4. §4 LP-1/2/5 集成测试兼容
  5. §5 hardening/escape/autoclose 0 变化 baseline 保持
  6. §6 cross-package import DAG 验证 (单向)
  7. §7 sessionorchestrator/ 包内 ExitReason 残留 0 验证
  8. §8 spec 同步 (4 文档 version bump)
  9. §9 PR + auto-merge 落地证据

## Phase 7: S6 归档 (1h)

### Task 7.1: archive 目录迁移

- [ ] `mkdir -p openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/specs/d7-orchestration/`
- [ ] `git mv openspec/changes/devrix-d7-6s-verify-promotion/{demand,proposal,design,tasks,acceptance-report}.md openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/`
- [ ] `git mv openspec/changes/devrix-d7-6s-verify-promotion/specs/d7-orchestration/spec.md openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/specs/d7-orchestration/`
- [ ] `git mv openspec/changes/devrix-d7-6s-verify-promotion/.openspec.yaml openspec/archive/2026-06-26-devrix-d7-6s-verify-promotion/`

### Task 7.2: archive 状态更新

- [ ] `.openspec.yaml`: `status=s2_proposal` → `s7_archived`, 4 P0 T 全 PLANNED → IMPLEMENTED
- [ ] `proposal.md`: `**Status:** S2_Proposal` → `S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived`
- [ ] `design.md`: `**Status:** S3_Design` → `S3_Design → S3-Gate Approved → S4_Implemented → S5_Accepted → S7_Archived`
- [ ] `tasks.md`: `**Status:** S3_Design` → `S3_Design → S5_Accepted → S7_Archived`
- [ ] `specs/d7-orchestration/spec.md`: `**Status:** S3_Design` → `S3_Design → S5_Accepted → S7_Archived`

### Task 7.3: demand-archive-index.md 新增行

- [ ] 在 openspec/demand-archive-index.md DM-20260626-004 行后追加 DM-20260626-005 行:
  ```
  | DM-20260626-005 | devrix-d7-6s-verify-promotion | S7_Archived | 2026-06-26 | sessionorchestrator/ exit_reason.go + verdict_to_exit_reason.go + verdict_to_exit_reason_test.go (218 行) git mv → executionflow/verify/, package 改名 + 11 处 ExitReason* → verify.ExitReason*, 0 函数签名变化 (pure physical migration), 13/13 AC + 4/4 P0 T (D7-S4-A50 T01-T04) IMPLEMENTED, 22/22 orchestration packages go test -race PASS, LP-1/LP-2/LP-5 100% 兼容, hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化 |
  ```

### Task 7.4: changes/ 目录清理

- [ ] `rm -rf openspec/changes/devrix-d7-6s-verify-promotion/`

### Task 7.5: verify-archive.sh 验证

- [ ] `bash scripts/verify-archive.sh devrix-d7-6s-verify-promotion` 输出 "结果: 12 通过, 0 失败, 1 警告" (警告 = spec.md/design.md/a-registry.md/f-registry.md 人工确认, 已在 Task 1.1-1.4 完成)

### Task 7.6: PR + auto-merge

- [ ] `git checkout -b chore/s6-archive-devrix-d7-6s-verify-promotion`
- [ ] `git add -A` (7 files rename + 1 modify)
- [ ] `git commit -m "chore(openspec): S6 archive devrix-d7-6s-verify-promotion (DM-20260626-005)"`
- [ ] `git push -u origin chore/s6-archive-devrix-d7-6s-verify-promotion`
- [ ] `gh pr create --title "chore(openspec): S6 archive devrix-d7-6s-verify-promotion (DM-20260626-005)" --body "..."`
- [ ] `gh pr merge <PR#> --auto --squash --delete-branch`
- [ ] 等 CI PASS + squash auto-merge 完成