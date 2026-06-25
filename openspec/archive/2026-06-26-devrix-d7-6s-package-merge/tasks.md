# Tasks: D7 turn/ 包合并到 sessionorchestrator/

**Change ID:** `devrix-d7-6s-package-merge`
**Demand ID:** DM-20260626-004
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**Sprint:** d7-v6 follow-up #3
**PR Count:** 1
**前置:** devrix-d7-six-s-simplification (DM-20260626-001) + devrix-d7-mups-package-migration (DM-20260626-002) + devrix-d7-hardening-cross-cutting (DM-20260626-003) 全部 S7_Archived

---

## 任务总览

| Phase | Task | 描述 | 工作量 | 状态 |
| ----- | ---- | ---- | ------ | ---- |
| **Step 1** | T1.1 | 创建 `feat/devrix-d7-6s-package-merge` 分支 (从 master) | 0.05 天 | ⬜ |
| **Step 1** | T1.2 | 23 个直接 git mv (orchestrator.go + orchestrator_test.go + ... 排除 doc.go + tracing.go) | 0.1 天 | ⬜ |
| **Step 1** | T1.3 | 2 个同名文件重命名 git mv (`turn/doc.go` → `sessionorchestrator/turn_doc.go` + `turn/tracing.go` → `sessionorchestrator/tracing_turn.go`) | 0.05 天 | ⬜ |
| **Step 1** | T1.4 | 验证 `internal/layers/orchestration/turn/` 目录物理消失 | 0.05 天 | ⬜ |
| **Step 1** | T1.5 | 验证 sessionorchestrator/ 包扩展至 ~60 文件 | 0.05 天 | ⬜ |
| **Step 1** | T1.6 | commit 1: "refactor(d7): turn package migration Step 1 — git mv" | 0.05 天 | ⬜ |
| **Step 2** | T2.1 | 25 个迁入文件 package 声明: `package turn` → `package sessionorchestrator` (1 行 sed per file) | 0.05 天 | ⬜ |
| **Step 2** | T2.2 | 全仓 `grep -rln "orchestration/turn\""` 列出 14 个 importer | 0.05 天 | ⬜ |
| **Step 2** | T2.3 | 14 importer import path 替换 (`orchestration/turn"` → `orchestration/sessionorchestrator"`) | 0.1 天 | ⬜ |
| **Step 2** | T2.4 | sessionorchestrator/turn_tools.go 内部 turn.X 引用 → sessionorchestrator.X 引用 | 0.05 天 | ⬜ |
| **Step 2** | T2.5 | sessionorchestrator/turn_tools_test.go 内部 turn.X 引用 → sessionorchestrator.X 引用 | 0.05 天 | ⬜ |
| **Step 2** | T2.6 | 验证 `grep -rln "orchestration/turn\"" internal/ cmd/` 返回 0 命中 | 0.05 天 | ⬜ |
| **Step 2** | T2.7 | 验证 `grep -rln "package turn$" internal/layers/orchestration/` 返回 0 命中 | 0.05 天 | ⬜ |
| **Step 2** | T2.8 | 验证 `grep -rln "turn\.NewOrchestrator\|turn\.DefaultOrchestrator\|..."` 返回 0 命中 | 0.05 天 | ⬜ |
| **Step 2** | T2.9 | commit 2: "refactor(d7): turn package migration Step 2 — package rename + import path" | 0.05 天 | ⬜ |
| **Step 3** | T3.1 | `go build ./...` 全仓编译 0 错误 | 0.1 天 | ⬜ |
| **Step 3** | T3.2 | `go vet ./...` 全仓静态检查 0 警告 | 0.1 天 | ⬜ |
| **Step 3** | T3.3 | `go test ./internal/layers/orchestration/... -race -count=1` 23/23 PASS | 0.2 天 | ⬜ |
| **Step 3** | T3.4 | LP-1/LP-2/LP-5 集成测试验证 (TestAutoClose_FullLP1Loop + TestIntegration_5NodePipeline_End2End) | 0.1 天 | ⬜ |
| **Step 3** | T3.5 | 验证 `internal/layers/orchestration/hardening/` 0 变化 (Decision 5 hardening 不变) | 0.05 天 | ⬜ |
| **Step 3** | T3.6 | 验证 `internal/layers/orchestration/escape/circuit_breaker.go` 0 变化 (Decision 1) | 0.05 天 | ⬜ |
| **Step 3** | T3.7 | 验证 `internal/layers/orchestration/sessionorchestrator/autoclose.go` 0 变化 (Decision 4) | 0.05 天 | ⬜ |
| **Step 3** | T3.8 | commit 3: "refactor(d7): turn package migration Step 3 — build+vet+test green" | 0.05 天 | ⬜ |
| **Step 4** | T4.1 | 更新 `openspec/specs/d7-orchestration/d7-domain.md` v2.1.0 → v2.2.0 §① S2 SessionOrchestrator 章节包路径描述 | 0.1 天 | ⬜ |
| **Step 4** | T4.2 | 更新 `openspec/specs/d7-orchestration/design.md` v4.1.0 → v4.2.0 §① S2 SessionOrchestrator 包路径描述 | 0.1 天 | ⬜ |
| **Step 4** | T4.3 | 更新 `openspec/specs/d7-orchestration/t-registry.md` v4.3.0 → v4.4.0 (新增 D7-S2-A50-T01..T04) | 0.1 天 | ⬜ |
| **Step 4** | T4.4 | 更新 `openspec/t-registry.md` (root) v5.3.0 → v5.4.0 (新增 DM-20260626-004 增量条目) | 0.1 天 | ⬜ |
| **Step 4** | T4.5 | commit 4: "docs(openspec): turn package migration Step 4 — doc sync" | 0.05 天 | ⬜ |
| **S5** | T5.1 | 编写 `acceptance-report.md` §1-§10 全部 13 AC 验收 | 0.2 天 | ⬜ |
| **S5** | T5.2 | 4 新 P0 T (D7-S2-A50-T01..T04) 状态 PLANNED → IMPLEMENTED | 0.05 天 | ⬜ |
| **S5** | T5.3 | commit 5: "docs(openspec): turn package migration S5 acceptance" | 0.05 天 | ⬜ |
| **S6** | T6.1 | `gh pr ready` 触发 S4-Gate review | 0.05 天 | ⬜ |
| **S6** | T6.2 | CI `unit tests` 通过 | 0.1 天 | ⬜ |
| **S6** | T6.3 | `gh pr merge --auto --squash` 自动合入 master | 0.05 天 | ⬜ |
| **S6** | T6.4 | 本地 `git pull origin master` 同步最新 master | 0.05 天 | ⬜ |
| **S6 归档** | T7.1 | 移动 `openspec/changes/devrix-d7-6s-package-merge/` → `openspec/archive/2026-06-26-devrix-d7-6s-package-merge/` | 0.05 天 | ⬜ |
| **S6 归档** | T7.2 | 更新 `openspec/demand-archive-index.md` 新增 DM-20260626-004 行 | 0.05 天 | ⬜ |
| **S6 归档** | T7.3 | 运行 `./scripts/verify-archive.sh devrix-d7-6s-package-merge` 12/12 PASS | 0.05 天 | ⬜ |
| **S6 归档** | T7.4 | commit 6: "chore(openspec): S6 archive devrix-d7-6s-package-merge" | 0.05 天 | ⬜ |

**总计**: ~3 天工作量（参考值，实际以实施为准）

---

## 实施步骤（commit-by-commit）

### Commit 1: Step 1 物理目录迁移 + 2 同名文件重命名 (T1.1 - T1.6)

```bash
# 切分支（已完成）
git checkout -b feat/devrix-d7-6s-package-merge

# 23 个直接 git mv (无同名冲突)
git mv internal/layers/orchestration/turn/orchestrator.go \
       internal/layers/orchestration/sessionorchestrator/orchestrator.go
git mv internal/layers/orchestration/turn/orchestrator_test.go \
       internal/layers/orchestration/sessionorchestrator/orchestrator_test.go
git mv internal/layers/orchestration/turn/orchestrator_toolcap_test.go \
       internal/layers/orchestration/sessionorchestrator/orchestrator_toolcap_test.go
git mv internal/layers/orchestration/turn/compression_summarizer.go \
       internal/layers/orchestration/sessionorchestrator/compression_summarizer.go
git mv internal/layers/orchestration/turn/compression_summarizer_test.go \
       internal/layers/orchestration/sessionorchestrator/compression_summarizer_test.go
git mv internal/layers/orchestration/turn/contracts.go \
       internal/layers/orchestration/sessionorchestrator/contracts.go
git mv internal/layers/orchestration/turn/exit_reason.go \
       internal/layers/orchestration/sessionorchestrator/exit_reason.go  # 临时留, 等 #4
git mv internal/layers/orchestration/turn/fake_gateway_test.go \
       internal/layers/orchestration/sessionorchestrator/fake_gateway_test.go
git mv internal/layers/orchestration/turn/focus_hint.go \
       internal/layers/orchestration/sessionorchestrator/focus_hint.go
git mv internal/layers/orchestration/turn/llm.go \
       internal/layers/orchestration/sessionorchestrator/llm.go
git mv internal/layers/orchestration/turn/llm_test.go \
       internal/layers/orchestration/sessionorchestrator/llm_test.go
git mv internal/layers/orchestration/turn/recovery.go \
       internal/layers/orchestration/sessionorchestrator/recovery.go
git mv internal/layers/orchestration/turn/recovery_test.go \
       internal/layers/orchestration/sessionorchestrator/recovery_test.go
git mv internal/layers/orchestration/turn/resolve_await.go \
       internal/layers/orchestration/sessionorchestrator/resolve_await.go
git mv internal/layers/orchestration/turn/runturn_main_path_test.go \
       internal/layers/orchestration/sessionorchestrator/runturn_main_path_test.go
git mv internal/layers/orchestration/turn/subturn.go \
       internal/layers/orchestration/sessionorchestrator/subturn.go
git mv internal/layers/orchestration/turn/subturn_test.go \
       internal/layers/orchestration/sessionorchestrator/subturn_test.go
git mv internal/layers/orchestration/turn/subturn_fork_test.go \
       internal/layers/orchestration/sessionorchestrator/subturn_fork_test.go
git mv internal/layers/orchestration/turn/tool_stream.go \
       internal/layers/orchestration/sessionorchestrator/tool_stream.go
git mv internal/layers/orchestration/turn/tool_stream_test.go \
       internal/layers/orchestration/sessionorchestrator/tool_stream_test.go
git mv internal/layers/orchestration/turn/verdict_to_exit_reason.go \
       internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go  # 临时留, 等 #4
git mv internal/layers/orchestration/turn/verdict_to_exit_reason_test.go \
       internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go

# 2 个同名文件重命名 (避免 git mv 覆盖)
git mv internal/layers/orchestration/turn/doc.go \
       internal/layers/orchestration/sessionorchestrator/turn_doc.go
git mv internal/layers/orchestration/turn/tracing.go \
       internal/layers/orchestration/sessionorchestrator/tracing_turn.go

# 验证 turn/ 物理删除
ls internal/layers/orchestration/turn/  # 应 "No such file or directory"
ls internal/layers/orchestration/sessionorchestrator/ | wc -l  # 应 60 文件 (35 原 + 25 迁入)

# commit
git add -A
git commit -m "refactor(d7): turn package migration Step 1 — git mv (25 .go files)

- mkdir sessionorchestrator/ turn/ 整包 25 .go git mv (preserve git history)
- 2 同名文件重命名: turn/doc.go → sessionorchestrator/turn_doc.go (避免与 sessionorchestrator/doc.go 冲突)
- 2 同名文件重命名: turn/tracing.go → sessionorchestrator/tracing_turn.go (避免与 sessionorchestrator/tracing.go 冲突)
- turn/ 目录物理删除 (git mv 已自动处理)
- sessionorchestrator/ 包扩展至 60 文件 ~15000 行
- escape/circuit_breaker.go + hardening/ + sessionorchestrator/autoclose.go 0 变化 (Decision 1 + 5 + 4)
- exit_reason.go + verdict_to_exit_reason.go 临时留 sessionorchestrator/ (等 #4 promote, Decision 4)
- git history 保留 (--follow 可追溯)"
```

**预期 commit 影响**: 0 编译错误（package rename + import path 还未替换），仅文件移动。

### Commit 2: Step 2 package 改名 + 14 importer import path 替换 (T2.1 - T2.9)

```bash
# 25 个迁入文件 package 声明: package turn → package sessionorchestrator
sed -i '' 's|^package turn$|package sessionorchestrator|' \
  internal/layers/orchestration/sessionorchestrator/orchestrator.go \
  internal/layers/orchestration/sessionorchestrator/orchestrator_test.go \
  internal/layers/orchestration/sessionorchestrator/orchestrator_toolcap_test.go \
  internal/layers/orchestration/sessionorchestrator/compression_summarizer.go \
  internal/layers/orchestration/sessionorchestrator/compression_summarizer_test.go \
  internal/layers/orchestration/sessionorchestrator/contracts.go \
  internal/layers/orchestration/sessionorchestrator/turn_doc.go \
  internal/layers/orchestration/sessionorchestrator/exit_reason.go \
  internal/layers/orchestration/sessionorchestrator/fake_gateway_test.go \
  internal/layers/orchestration/sessionorchestrator/focus_hint.go \
  internal/layers/orchestration/sessionorchestrator/llm.go \
  internal/layers/orchestration/sessionorchestrator/llm_test.go \
  internal/layers/orchestration/sessionorchestrator/recovery.go \
  internal/layers/orchestration/sessionorchestrator/recovery_test.go \
  internal/layers/orchestration/sessionorchestrator/resolve_await.go \
  internal/layers/orchestration/sessionorchestrator/runturn_main_path_test.go \
  internal/layers/orchestration/sessionorchestrator/subturn.go \
  internal/layers/orchestration/sessionorchestrator/subturn_test.go \
  internal/layers/orchestration/sessionorchestrator/subturn_fork_test.go \
  internal/layers/orchestration/sessionorchestrator/tool_stream.go \
  internal/layers/orchestration/sessionorchestrator/tool_stream_test.go \
  internal/layers/orchestration/sessionorchestrator/tracing_turn.go \
  internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go \
  internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason_test.go

# 14 importer import path 替换
for f in \
  internal/bootstrap/wire_coordinator.go \
  internal/bootstrap/turn_wiring.go \
  internal/bootstrap/turn_adapter.go \
  internal/bootstrap/turn_adapter_test.go \
  internal/bootstrap/turn_adapter_persist_test.go \
  internal/bootstrap/turn_adapter_permission_test.go \
  internal/bootstrap/turn_adapter_surface_test.go \
  internal/bootstrap/context_engine.go \
  internal/bootstrap/context_engine_builder.go \
  internal/bootstrap/plan_llm_completer.go \
  internal/layers/orchestration/decisionplanning/llm_decomposer.go \
  internal/layers/orchestration/decisionplanning/llm_decomposer_test.go \
  internal/layers/orchestration/sessionorchestrator/turn_tools.go \
  internal/layers/orchestration/sessionorchestrator/turn_tools_test.go ; do
  sed -i '' 's|internal/layers/orchestration/turn"|internal/layers/orchestration/sessionorchestrator"|g' "$f"
done

# sessionorchestrator/turn_tools.go 内部 turn.X 引用 → sessionorchestrator.X (同包内 bare name 应自动, 但手工验证)
# 如 turn_tools.go 内有 turn.NewOrchestrator 这种 import "turn" 后调用, 需改为 sessionorchestrator.NewOrchestrator

# 验证 0 残留
grep -rln "orchestration/turn\"" internal/ cmd/  # 必须 0 命中
grep -rln "package turn$" internal/layers/orchestration/  # 必须 0 命中
grep -rln "turn\.NewOrchestrator\|turn\.DefaultOrchestrator\|turn\.SubTurnRunner\|turn\.GatewayInvoker\|turn\.CompressionSummarizer\|turn\.OrchestratorDeps\|turn\.TurnOrchestrator\|turn\.PreparedTurnAdapter" internal/ cmd/  # 必须 0 命中

# commit
git add -A
git commit -m "refactor(d7): turn package migration Step 2 — package rename + import path

- 25 迁入文件 package turn → package sessionorchestrator (1 行 sed per file)
- 14 importer import path orchestration/turn → orchestration/sessionorchestrator (sed -i '')
  - 10 bootstrap files (wire_coordinator + turn_wiring + turn_adapter + 4 test + context_engine + context_engine_builder + plan_llm_completer)
  - 2 decisionplanning files (llm_decomposer + _test)
  - 2 sessionorchestrator/turn_tools files (内部 turn.X → sessionorchestrator.X)
- grep 0 残留验证通过"
```

**预期 commit 影响**: 编译通过。

### Commit 3: Step 3 编译 + 测试回归 (T3.1 - T3.8)

```bash
# 编译验证
go build ./...  # 必须 0 错误

# 静态检查
go vet ./...  # 必须 0 警告

# 23 包 race 测试
go test ./internal/layers/orchestration/... -race -count=1  # 必须 23/23 PASS

# LP-1/LP-2/LP-5 集成测试
go test ./internal/layers/orchestration/sessionorchestrator/... -race -run "TestAutoClose_FullLP1Loop"
go test ./internal/layers/orchestration/... -race -run "TestIntegration_5NodePipeline_End2End"

# 验证 hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go 0 变化
git diff HEAD~3 HEAD -- internal/layers/orchestration/hardening/  # 必须空
git diff HEAD~3 HEAD -- internal/layers/orchestration/escape/circuit_breaker.go  # 必须空
git diff HEAD~3 HEAD -- internal/layers/orchestration/sessionorchestrator/autoclose.go  # 必须空

# commit
git add -A
git commit -m "refactor(d7): turn package migration Step 3 — build+vet+test green

- go build ./... 0 错误
- go vet ./... 0 警告
- go test ./internal/layers/orchestration/... -race -count=1 23/23 PASS (含 sessionorchestrator 扩展后 60 文件)
- hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go 0 变化 (Decision 5 + 1 + 4)
- LP-1 (Bayesian reputation) / LP-2 (Memory 3 通道) / LP-5 (Cross-session traceability) 路径 0 变化"
```

### Commit 4: Step 4 文档同步 (T4.1 - T4.5)

```bash
# 更新 d7-domain.md §① S2 SessionOrchestrator 章节包路径描述 (v2.1.0 → v2.2.0)
# 更新 design.md §① Discipline Keeper 包路径描述 (v4.1.0 → v4.2.0)
# 更新 t-registry.md (域): D7-S2-A50-T01..T04 状态 PLANNED → IMPLEMENTED + v4.3.0 → v4.4.0
# 更新 t-registry.md (root): v5.3.0 → v5.4.0 + 新增条目

git add -A
git commit -m "docs(openspec): turn package migration Step 4 — doc sync

- d7-domain.md v2.1.0 → v2.2.0 §① S2 SessionOrchestrator 章节包路径描述更新
- design.md v4.1.0 → v4.2.0 §① S2 SessionOrchestrator 包路径描述更新
- 域 t-registry.md v4.3.0 → v4.4.0 + D7-S2-A50 T01..T04 IMPLEMENTED
- root t-registry.md v5.3.0 → v5.4.0 + 新增量条目"
```

### Commit 5: S5 验收报告 (T5.1 - T5.3)

```bash
# 编写 acceptance-report.md
git add -A
git commit -m "docs(openspec): turn package migration S5 acceptance report (DM-20260626-004)

- acceptance-report.md §1-§10 全部 13 AC 验收
- 4 新 P0 T (D7-S2-A50-T01..T04) PLANNED → IMPLEMENTED"
```

### Commit 6: S6 归档 (T7.1 - T7.4)

```bash
# 移动到 archive
mv openspec/changes/devrix-d7-6s-package-merge openspec/archive/2026-06-26-devrix-d7-6s-package-merge

# 更新 demand-archive-index.md
# verify-archive.sh
./scripts/verify-archive.sh devrix-d7-6s-package-merge  # 12/12 PASS

# commit
git add -A
git commit -m "chore(openspec): S6 archive devrix-d7-6s-package-merge (DM-20260626-004)"
```

---

## 验证清单 (S5 验收)

- [ ] AC1: turn/ 物理目录消失，25 .go 全部 git mv 到 sessionorchestrator/
- [ ] AC2: package sessionorchestrator 声明在 60 文件保持一致（含 35 原 + 25 迁入）
- [ ] AC3: 全仓 grep "orchestration/turn\"" 0 命中（14 importer 全部替换）
- [ ] AC4: 全仓 grep "turn\.NewOrchestrator\|turn\.DefaultOrchestrator\|..." 0 命中（跨包调用全部更新）
- [ ] AC5: go build ./... 0 错误
- [ ] AC6: go vet ./... 0 警告
- [ ] AC7: go test ./internal/layers/orchestration/... -race -count=1 23/23 PASS
- [ ] AC8: 10 bootstrap files import path 同步更新
- [ ] AC9: decisionplanning/llm_decomposer.go + _test.go import path 同步更新
- [ ] AC10: sessionorchestrator/turn_tools.go + _test.go 内部 turn.X → sessionorchestrator.X 引用更新
- [ ] AC11: 4 P0 T (D7-S2-A50-T01..T04) 全部 IMPLEMENTED
- [ ] AC12: LP-1/LP-2/LP-5 集成测试 100% 兼容
- [ ] AC13: verify-archive.sh devrix-d7-6s-package-merge 12/12 PASS 0 FAIL

---

## 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| sessionorchestrator/ 包扩大至 ~60 文件 ~15000 行 | 中 | v6.0.0 设计目标 — S2 复合角色；Go 增量编译无影响；接受包大小换取角色内聚 |
| doc.go + tracing.go 同名文件覆盖 | 中 | Decision 2 — 重命名为 turn_doc.go + tracing_turn.go，Step 1 git mv 时直接改名 |
| OrchestratorOption 误判同名冲突 | 低 | pre-S3 实测确认 turn/ 0 个 OrchestratorOption 定义 |
| 14 importer import path 替换遗漏 | 中 | Step 2 全仓 grep orchestration/turn" 0 命中验证；14 个 importer 列表在 design.md §5.3 |
| turn_tools.go 内部 turn.X 引用遗漏 | 中 | Step 2 同时改 import path + 内部 turn.X 引用 → sessionorchestrator.X |
| DefaultOrchestrator + SessionOrchestrator 双 type 命名混乱 | 低 | Decision 3 — 接受双 type 换取职责清晰 |
| exit_reason.go 临时留 sessionorchestrator/ | 低 | Decision 4 — 显式标注"等 #4 promote" |
| hardening/ receiver methods 跨包类型变化 | 低 | Decision 5 — hardening/ 落地时已确认 receiver 类型不变 |
| LP-1/LP-2/LP-5 行为漂移 | 极低 | 0 函数逻辑变化，仅物理迁移 |
| CI 镜像缓存 | 低 | turn/ 物理删除后强制 re-build；CI 单测 100% PASS 是硬门禁 |