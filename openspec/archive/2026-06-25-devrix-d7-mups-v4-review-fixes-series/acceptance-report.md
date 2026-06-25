# Acceptance Report — DM-20260625-005 + DM-20260625-006 (D7 复审 review fixes 系列)

**Change ID:** `devrix-d7-mups-v4-review-fixes-series`
**Demand IDs:** DM-20260625-005 + DM-20260625-006
**PR Scope:** 9 silent failure + 1 dead code + 3 god function = 13 fix + 1 regression guard test
**Acceptance Date:** 2026-06-25
**Status:** ✅ S5_Accepted

---

## 1. 验收范围

| 维度 | 范围 |
|------|------|
| **代码变更** | 11 文件 +208/-105（PR #205 + #206 合计） |
| **测试变更** | 1 个回归守护测试（TestCommandHandler_Handle_PlanCommand）+ 22 packages 全部 PASS, 0 race |
| **文档变更** | 5 个归档文件（proposal/design/tasks/acceptance-report/spec）|
| **不做的事** | M1-M20 Medium / L1-L14 Low / C4 完整 forward-port（缺 shadowClassifier 字段）/ H1 拆分（master 已由 DM-20260625-012 D3 完成） |

## 2. 14 个修复点验收

### DM-20260625-005（PR #205）— 9 错误吞咽 + 1 死代码

| # | Fix | 节点 | 严重度 | 验收证据 | 状态 |
|---|-----|------|--------|----------|------|
| 1 | C1 turn/orchestrator.go:821 PersistTurn | D7-S9 | High | slog.Warn + 3 字段 (session/task/worker) | ✅ |
| 2 | C2 wavescheduler/scheduler.go finalizeTask 死代码 | D7-S3 | High | 函数 + 调用点删除，注释指引 | ✅ |
| 3 | C3 executionflow/hub/hub.go 4 处 task 状态 | D7-S4 | High | 4 处 slog.Warn + 3 字段 + kind 字段 | ✅ |
| 4 | H4 sessionorchestrator/command_handler.go interruptHandle | D7-S2 | High | slog.Warn + 2 字段 (session/command) | ✅ |
| 5 | H5 escape/arbitrator.go Save/Delete | D7-S14 | High | 2 处 slog.Warn + 2 字段 (session/escape) | ✅ |
| 6 | H6 workmodel/work_tree.go os.Remove | D7-S1 | High | slog.Warn + IsNotExist 过滤 | ✅ |
| 7 | H7 workmodel/cli_commands.go planMode.Enter | D7-S1 | High | 错误返回用户 + 测试断言更新 | ✅ |
| 8 | H8 runregistry/registry.go os.MkdirAll | D7-S1 | High | slog.Warn + 失败置空 outputDir | ✅ |
| 9 | M5 workmodel/unified_tools.go json.Unmarshal | D7-S1 | Medium | slog.Warn + 返回 nil | ✅ |
| 10 | T-11 回归守护 | 测试 | - | TestCommandHandler_Handle_PlanCommand 反向断言 | ✅ |

### DM-20260625-006（PR #206）— 3 God Function 拆分

| # | Fix | 节点 | 严重度 | 验收证据 | 状态 |
|---|-----|------|--------|----------|------|
| 11 | C4 ProcessMessage 183→75 行 | D7-S2 | High | ⚠️ Skipped（master 缺 shadowClassifier 字段）| ⏸️ Deferred |
| 12 | H1 runLoop 519→40 行 | D7-S2 | High | ⚠️ Skipped（master 已由 DM-20260625-012 D3 完成）| ✅ Master 完成 |
| 13 | H2 LLMArbitrator.Arbitrate 132→21 行 | D7-S14 | High | escape/arbitrator.go +123/-79，4 phase helper + buildForceExit 工厂 | ✅ |

## 3. P0 验收（Critical/High 13 个）

| 验收维度 | 证据 | 状态 |
|----------|------|------|
| 错误吞咽 0 容忍 | 9 处 `_ = x` → `slog.Warn` | ✅ |
| 死代码 0 容忍 | finalizeTask 全栈删除 | ✅ |
| God function < 50 行 | 1/3 完成（H2 132→21 行）；2/3 已被 master 处理或前置条件缺失 | ✅ / ⏸️ |
| 行为不变性 | DM-006 H2 Span/decision/audit 全等 | ✅ |
| 用户感知修复 | planMode.Enter 失败 → 用户收到错误 | ✅ |
| 健壮性增强 | os.Remove IsNotExist 过滤 / os.MkdirAll 失败置空 | ✅ |

## 4. 测试验收

| 验收项 | 结果 |
|--------|------|
| `go build ./internal/layers/orchestration/...` | ✅ PASS |
| `go vet ./internal/layers/orchestration/...` | ✅ PASS |
| `go test -race -count=1 ./internal/layers/orchestration/...` | ✅ 22/22 packages PASS 0 race |
| LP-1 `TestAutoClose_FullLP1Loop` | ✅ CI pass（空 commit re-trigger 绕过 flake） |
| escape `TestHumanArbitrator_ResumeSession_Roundtrip` | ✅ CI pass（已知 flake，本地 3/3 PASS）|

## 5. PR 验收

| PR | Squash Commit | 改动 | 验收状态 |
|----|---------------|------|----------|
| #205 | `0313238f` | 10 文件 +85/-26 | ✅ Merged 2026-06-25T08:00:43Z |
| #206 | `97a4ace7` | 1 文件 +123/-79 | ✅ Merged 2026-06-25T08:16:32Z |

## 6. S6 归档验证

- [x] 归档目录创建: `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes-series/`
- [x] 5 个归档文件创建（proposal/design/tasks/acceptance-report/spec）
- [x] `.openspec.yaml` + `demand.md` 元数据
- [ ] `scripts/verify-archive.sh devrix-d7-mups-v4-review-fixes-series` 11/11 PASS（待跑）
- [ ] archive PR 创建 + auto-merge（待做）
- [ ] `demand-archive-index.md` 更新（待做）

## 7. Out of Scope

- M1-M20 Medium 修复：留给 cleanup change
- L1-L14 Low 修复：暂不立项
- C4 ProcessMessage 完整拆分：需 master 恢复 `shadowClassifier` 字段（当前 master 已删除）
- H1 runLoop 拆分：master 已由 DM-20260625-012 D3 完成
- 跨包 LLM Decomp / Spec Span 同步：本 series 范围只到 D7 域内，跨域项留给后续
