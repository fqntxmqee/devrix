# Tasks: D7 MUPS v4 Review 修复

**Change ID:** devrix-d7-mups-v4-review-fixes
**Demand ID:** DM-20260625-002

---

## Phase 1: Critical 修复 (S4 实现)

- [x] **T-01**: `workmodel/aggregate_verdicts.go::clamp01OrFallback` 加 `math.IsNaN(v)` 检查
  - 文件: `internal/layers/orchestration/workmodel/aggregate_verdicts.go`
  - 添加 `math` import
  - 入口先 `if math.IsNaN(v) { return fallback }`
- [x] **T-02**: `aggregateMeta` 重写为 dedup + IndeterminateReason 最长 + SystemAnomaly OR
  - 文件: `internal/layers/orchestration/workmodel/aggregate_verdicts.go`
  - `SourceID` → `strings.Join(dedup(...), ",")`
  - `IndeterminateReason` → 选最长的（信息量最大）
  - `SystemAnomaly` → OR 聚合
- [x] **T-03**: `execute/channel_protocol.go::rollback` 用独立 `context.Background() + timeout`
  - 文件: `internal/layers/orchestration/execute/channel_protocol.go`
  - `rollbackCtx, cancel := context.WithTimeout(context.Background(), c.cfg.Timeout)`
  - first non-nil error wins

## Phase 2: High 修复 (S4 实现)

- [x] **T-04**: `plan/plan_struct.go::Plan.Validate` 删除 `!= ""` 短路，改用 `Valid()` 检查
  - 文件: `internal/layers/orchestration/plan/plan_struct.go`
  - 新增 `PLAN_PERSIST_8012` sentinel
- [x] **T-05**: `plan/planner.go::NewPlanID` 改用 UUID + SHA-256
  - 文件: `internal/layers/orchestration/plan/planner.go`
  - `plan_<uuid[:8]>_<sha256[:8]>`
  - obsIDs 去重 + 排序后再 hash
- [x] **T-06**: `execute/errors.go` 新增 `ErrChannelStepInvalid` + `ErrChannelToolCallTimedOut`
  - 文件: `internal/layers/orchestration/execute/errors.go`
  - 配套 `NewChannelStepInvalidError` + `NewChannelToolCallTimedOutError`
- [x] **T-07**: `execute/channel_commit.go` 改用新错误（不滥用 StepCountMismatch）
  - 文件: `internal/layers/orchestration/execute/channel_commit.go`
- [x] **T-08**: `execute/channel_exploration.go` 改用 sync.WaitGroup 模式消除死锁
  - 文件: `internal/layers/orchestration/execute/channel_exploration.go`
  - spawn-all + buffered out + close after wg.Wait
- [x] **T-09**: `execute/channel_exploration.go` 新增 `mostInformativeError` helper
  - 文件: `internal/layers/orchestration/execute/channel_exploration.go`
  - 选最长 error（ties 选 first）
- [x] **T-10**: `learn/learner.go::DefaultLearner.Learn` 调换顺序（LP-3：先 Reputation 后 Memory）
  - 文件: `internal/layers/orchestration/learn/learner.go`
  - Reputation 写入幂等
- [x] **T-11**: `learn/memory.go::ScheduledMemory.ListDue` 返回深拷贝
  - 文件: `internal/layers/orchestration/learn/memory.go`
  - 新建 `ScheduledRetry` struct 拷贝内部 entry
- [x] **T-12**: `sessionorchestrator/autoclose.go::processAutoClose` Learn 改异步
  - 文件: `internal/layers/orchestration/sessionorchestrator/autoclose.go`
  - goroutine + context.Background() + 10s timeout
  - 加 `endSpanWithOnce(span, sync.Once)` 防 race
- [x] **T-13**: `sessionorchestrator/orchestrator_autoclose_test.go` Round 2 加 500ms wait
  - 文件: `internal/layers/orchestration/sessionorchestrator/orchestrator_autoclose_test.go`
- [x] **T-14**: 新增 `pipeline-architecture.md` (5 节点管道端到端总图)
  - 文件: `openspec/specs/d7-orchestration/pipeline-architecture.md` (NEW, 589 行)
  - `spec.md` 在 `## Architecture` 顶部加引用链接

## Phase 3: 适配测试 (S4 实现配套)

- [x] **T-15**: `plan/plan_test.go` 4 个 test 加 `BlastRadius: BlastRadius{PersistScope: ...}`
  - 文件: `internal/layers/orchestration/plan/plan_test.go`
  - 适配新 PersistScope fail-fast 行为

## Phase 4: S4-Gate 验证

- [x] **T-16**: `go vet ./...` exit 0
- [x] **T-17**: `go test -race -count=1 ./internal/layers/orchestration/...` all PASS
- [x] **T-18**: `go test -cover ./internal/layers/orchestration/...` coverage ≥ 88%

## Phase 5: S5 验收

- [ ] **T-19**: 提交 PR（feat/devrix-d7-mups-v4-review-fixes → master）
- [ ] **T-20**: CI checks 通过
- [ ] **T-21**: auto-merge squash
- [ ] **T-22**: 与 7 个 Phase archive 关系核对（无重叠）

## Phase 6: S6 归档

- [ ] **T-23**: `cp -r openspec/changes/devrix-d7-mups-v4-review-fixes openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes`
- [ ] **T-24**: 写 `acceptance-report.md` (实测数据 + 14 个 commit 列表)
- [ ] **T-25**: 跑 `scripts/verify-archive.sh` 通过
- [ ] **T-26**: 更新 `_meta/00.4-project-application-index.md`（如有需要）

---

## Verification 总体验证

| 项 | 命令 | 期望 |
|----|------|------|
| 编译 | `go vet ./...` | exit 0 |
| 测试 | `go test -race -count=1 ./internal/layers/orchestration/...` | all PASS |
| 覆盖率 | `go test -cover ./internal/layers/orchestration/...` | ≥ 88% |
| 归档 | `scripts/verify-archive.sh` | 7+1 Phase 全部 IMPLEMENTED |
| Git log | `git log --oneline -14` | 14 个 fix commit |
| PR | `gh pr list --state merged` | review-fixes PR 在列表中 |
