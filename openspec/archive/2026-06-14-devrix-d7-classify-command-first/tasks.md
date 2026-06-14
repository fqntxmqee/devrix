# D7-S5-T03 + T06 实施任务

## S4 任务清单

### T1: 端到端测试 — orchestrator_test.go

- [ ] 在 `internal/layers/d7/orchestrator_test.go` 末尾追加
  `TestSessionOrchestrator_CommandFirst_ShadowNotCalled`
- [ ] 引入 `sync/atomic`、`time` import（如未在）

### T2: 配置回归测试 — classifier_test.go

- [ ] 在 `internal/layers/d7/classifier_test.go` 末尾追加
  `TestRuleClassifier_Classify_CommandFirst_Disabled`

### T3: gofmt / vet / test

- [ ] `gofmt -w internal/layers/d7/`
- [ ] `go vet ./internal/layers/d7/...`
- [ ] `go test -race -count=10 ./internal/layers/d7/...`

### T4: t-registry 同步

- [ ] `openspec/specs/d7-orchestration/t-registry.md`
  - D7-S5-T03 PLANNED → IMPLEMENTED + Test 位置
  - D7-S5-T06 PLANNED → IMPLEMENTED + Test 位置
  - Statistics: 37 → 39 / PLANNED 7 → 5
  - By Scenario: D7-S5 IMPLEMENTED 3 → 5 / PLANNED 4 → 2
- [ ] `openspec/t-registry.md`
  - D7 行: IMPLEMENTED 37 → 39 / PLANNED 7 → 5
  - 总计: IMPLEMENTED 256 → 258 / PLANNED 13 → 11

### T5: S5 acceptance-report

- [ ] 列出 AC1~AC7 的通过证据
- [ ] go test 输出片段
- [ ] coverage 截图（textual）

### T6: S6 归档

- [ ] move `openspec/changes/devrix-d7-classify-command-first/` →
  `openspec/archive/2026-06-14-devrix-d7-classify-command-first/`
- [ ] 更新 `openspec/demand-archive-index.md` DM-20260614-005 行
- [ ] 更新 memory `devrix-d7-orchestration-archived.md`
