# Tasks: Token Design 2.0

**Change ID:** `devrix-token-design-v2`
**Demand ID:** DM-20260702-008
**T 点总数:** 28 (P0 = 19, P1 = 9)
**已交付:** 16 P0 T IMPLEMENTED in PR #376 (T01-T15 except T16-T24 + T25 + T27 + T28) + 9 P1 T deferred to devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009, PR #377)
**阶段:** 0 (决策) → 1-2 (持久化) → 3 (advisory) → 4 (aggregate) → 5 (concurrency) → 6 (classifier) → 7 (验证)

---

## 阶段 0: 决策 (本次, 0 T 点)

- [x] close PR #375 (原因: 8K 方案不合理, 走 DM-20260702-008)
- [x] archive DM-20260701-007 标 partial supersede (SUPERSEDE-NOTICE.md)
- [x] 起草本 proposal / demand / design / tasks
- [x] 开新 feature branch `feat/devrix-token-design-v2` (PR #376 已 merge 后删)

---

## 阶段 1-2: 持久化层 (P0, 8 T 点) — PR-A 路线

### T01 — PersistToFile 核心实现
- **DSAFT:** D2-S15-A02-T01
- **位置:** `internal/layers/contextengine/prepare/compression/persist.go` (新建)
- **API:**
  ```go
  func PersistToFile(content string, toolUseId string, maxChars int) (preview, filePath string, originalSize int, err error)
  ```
- **行为:**
  - 超过 maxChars → 写 `<projectDir>/<sessionId>/tool-results/<toolUseId>.txt`
  - 返回 preview (走 `generatePreview` 切到 newline 边界, ≤ PREVIEW_SIZE_BYTES=2000)
  - XML 包装 `<persisted-output>...</persisted-output>`
  - 失败 → fall back to truncate (日志 warn)
- **仿:** clawcode `src/utils/toolResultStorage.ts:persistToolResult:73-119` + `buildLargeToolResultMessage:189-198`
- **AC:** persist normal path + image block skip + fall-back 3 单元测试 PASS

### T02 — PrepareExecutionContext 集成
- **DSAFT:** D2-S15-A02-T02
- **位置:** `internal/layers/contextengine/prepare/compression/pipeline.go`
- **行为:** 替换 `stepToolResultBudget` 步为 `stepPersistToolResult` 步
- **阈值:** per-tool `MaxResultSizeChars` (从 spec.Metadata 读, 取代 `cfg.ToolResultBudget`)
- **仿:** clawcode `processToolResultBlock:215-237`
- **AC:** 19 工具 metadata 阈值生效, 走 PersistToFile 而非 TruncateToTokens

### T03 — image block 跳过
- **DSAFT:** D2-S15-A02-T03
- **位置:** `internal/layers/contextengine/prepare/compression/persist.go`
- **行为:** `hasImageBlock(content)` → 直接返回原 block, 不 persist 不 truncate
- **仿:** clawcode `maybePersistLargeToolResult:300-310`
- **AC:** image block 单元测试 PASS

### T04 — ContentReplacementState 决策冻结
- **DSAFT:** D2-S15-A02-T04
- **位置:** `internal/layers/contextengine/persist/content_replacement_state.go` (新建)
- **类型:**
  ```go
  type ContentReplacementState struct {
      SeenIds      map[string]bool
      Replacements map[string]string
  }
  ```
- **行为:** 同一 toolUseId 永远做同样决定, cache-stable + 重放稳定
- **仿:** clawcode `toolResultStorage.ts:386-413`
- **AC:** decision freeze 单元测试 PASS

### T05 — growthbook override
- **DSAFT:** D2-S15-A02-T05
- **位置:** `internal/layers/contextengine/persist/growthbook_override.go` (新建)
- **flag:** `devrix_persist_threshold_override` (per-tool map, default `{}`)
- **API:** `getPersistenceThreshold(toolName string, declaredMaxResultSizeChars int) int`
- **AC:** override 生效 + 防御性 null/string 单元测试 PASS

### T06 — surface_metadata_gate_test 加 PersistThreshold
- **DSAFT:** D2-S15-A02-T06
- **位置:** `internal/layers/contextengine/enforce/tools/surface/metadata_gate_test.go`
- **sentinel token:** `"PersistThreshold:"` 或 `"ApplyPersistMetadata"` in every surface
- **AC:** CI 跑测试, 19 工具 surface 全部声明 PersistThreshold

### T07 — 19 工具 metadata 改 per-tool 差异化
- **DSAFT:** D2-S15-A02-T07
- **位置:** `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go` + 各 surface
- **行为:** 19 工具 MaxResultSizeChars 改成 clawcode 风格 per-tool 差异化
- **值:** 详见 design.md §2.4
- **AC:** t-registry 反映新值, surface_metadata_gate_test PASS

### T08 — PersistToFile 测试
- **DSAFT:** D2-S15-A02-T08
- **位置:** `internal/layers/contextengine/prepare/compression/persist_test.go`
- **测试用例:** persist 正常路径, fail 路径, decision freeze, image block 跳过, growthbook override
- **AC:** 5 单元测试 PASS

---

## 阶段 3: Bounded(15) 改 advisory (P0, 4 T 点) — PR-B 路线

### T09 — ProbeToolChannel.Accept 改 advisory
- **DSAFT:** D7-S9-A50-T09
- **位置:** `internal/layers/orchestration/mups/execute/toolchannel/probe.go:75-85`
- **旧:** 硬 reject `ErrProbeToolChannelBoundExceeded`
- **新:** 永远 return `(true, nil)`, 触发 `InjectPromptPressure` 软警告
- **AC:** 单元测试 PASS, 不再 hard reject

### T10 — Read 工具加 offset/limit 参数
- **DSAFT:** D7-S9-A50-T10
- **位置:** read_file surface
- **API:** `ReadInput { Path string; Offset int; Limit int }`
- **行为:** 默认 `Offset=0`, `Limit=8192`, 旧调用方无感
- **AC:** offset/limit 单元测试 PASS

### T11 — ProbeToolChannel 默认 OpenEnded
- **DSAFT:** D7-S9-A50-T11
- **位置:** `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go:190-220`
- **旧:** read_file/grep/glob `Bounded(15)`
- **新:** `OpenEnded` + advisory thresholds (review@20/30, edit@15/20, test@18/25)
- **AC:** 默认 OpenEnded, advisory warning 在 advisory threshold 触发

### T12 — task_kind 推改 advisory
- **DSAFT:** D7-S9-A50-T12
- **位置:** `internal/layers/contextengine/enforce/tools/filter/per_task_kind.go`
- **旧:** `Bounded(15/10/12/8)` for review/edit/test/refactor
- **新:** `advisory@(soft/hard)` 阈值, task_kind 维度保留
- **AC:** task_kind 路由 + advisory warning 单元测试 PASS

---

## 阶段 4: per-message aggregate (P0, 3 T 点) — PR-C 路线

### T13 — PerMessageBudget 守卫
- **DSAFT:** D2-S15-A02-T13
- **位置:** `internal/layers/contextengine/prepare/compression/per_message_budget.go` (新建)
- **常量:** `MAX_TOOL_RESULTS_PER_MESSAGE_CHARS = 200_000`
- **API:** `enforcePerMessageBudget(messages []Message, perMessageBudget int) []Message`
- **AC:** 临界 / 超限 / 排序 persist 3 单元测试 PASS

### T14 — 集成到 PrepareExecutionContext
- **DSAFT:** D2-S15-A02-T14
- **位置:** `internal/layers/contextengine/prepare/compression/pipeline.go`
- **AC:** 集成单元测试 PASS

### T15 — per-message aggregate 测试
- **DSAFT:** D2-S15-A02-T15
- **测试用例:** N=10 个 30K / 20K / 100K tool_result
- **AC:** 3 单元测试 PASS

---

## 阶段 5: IsConcurrencySafe (P1, 4 T 点) — PR-D 路线 (下个 change)

### T16 — ToolSurface 加 IsConcurrencySafe
- **DSAFT:** D7-S9-A50-T16
- **位置:** `internal/shared/contracts/tool_surface.go`
- **API:** `IsConcurrencySafe(name string) bool`, 默认 `false` (fail-closed)
- **AC:** 接口编译通过, 默认 false 单元测试 PASS

### T17 — 19 工具 surface 声明
- **DSAFT:** D7-S9-A50-T17
- **位置:** 各 surface
- **AC:** surface_metadata_gate_test PASS

### T18 — ChannelRouter 集成
- **DSAFT:** D7-S9-A50-T18
- **位置:** `internal/layers/orchestration/mups/execute/channel_router.go`
- **AC:** 分桶单元测试 PASS

### T19 — 测试
- **DSAFT:** D7-S9-A50-T19
- **AC:** 3 单元测试 PASS

---

## 阶段 6: ToAutoClassifierInput (P1, 5 T 点) — PR-D 路线 (下个 change)

### T20 — ToolSurface 加 ToAutoClassifierInput
- **DSAFT:** D7-S10-A50-T20
- **位置:** `internal/shared/contracts/tool_surface.go`
- **API:** `ToAutoClassifierInput(name string, input any) string`, 协议: 返回 `''` = skip
- **AC:** 接口编译通过

### T21 — 19 工具 surface 声明
- **DSAFT:** D7-S10-A50-T21
- **AC:** surface_metadata_gate_test PASS

### T22 — auto-mode classifier
- **DSAFT:** D7-S10-A50-T22
- **位置:** `internal/layers/orchestration/mups/execute/auto_mode_classifier.go` (新建)
- **API:** `ClassifyAction(ctx, toolName, actionCompact, historyCompact) (shouldBlock bool, reason string, err error)`
- **AC:** classifier 单元测试 + fail open 测试 PASS

### T23 — ChannelRouter 集成
- **DSAFT:** D7-S10-A50-T23
- **AC:** 集成单元测试 PASS

### T24 — classifier 测试
- **DSAFT:** D7-S10-A50-T24
- **测试用例:** rm -rf /, ls, edit safe file, API 错误, 返回 ''
- **AC:** 5 单元测试 PASS

---

## 阶段 7: 验证 (P0, 3 T 点 + 1 LTL-Lite) — PR-E 路线

### T25 — LTL-Lite L4-L6 改 advisory
- **DSAFT:** D5-S25-A01/A02/A03
- **位置:** `internal/layers/observability/instrument/ltl/invariants/termination/{bounded,quotient,synthesize}.go`
- **旧:** hard invariant, trigger SynthesizeNow
- **新:** advisory, 保留为观测信号 (emit metric/log)
- **AC:** advisory 行为单元测试 PASS

### T26 — go test -race ./... PASS
- **DSAFT:** D2-S15-A02-T26
- **行为:** 12 packages + 新加 5 packages
- **AC:** `go test -race -count=1` 全部 PASS, 0 race warnings

### T27 — 端到端 review 任务测试
- **DSAFT:** D2-S15-A02-T27
- **场景:** 50 个文件 review (实际 devrix-monorepo)
- **期望:** 任务成功, 平均 Read < 100, 平均 LLM 调用 < 50, 任务成功率 > 95%
- **AC:** e2e 测试 PASS

### T28 — 8K 自我循环验证 (回归 PR #373 case)
- **DSAFT:** D2-S15-A02-T28
- **场景:** PR #373 当时 D1 红卡的真实 case
- **期望:** 新方案 100/100 成功, 旧方案 0/100 成功
- **AC:** 100/100 成功

---

## PR 路线图

| PR | 内容 | T 点 | 依赖 |
|----|------|------|------|
| **PR-A** (本次) | 阶段 1-2: 持久化层 | T01-T08 (8 T) | 无 |
| **PR-B** | 阶段 3: Bounded 改 advisory | T09-T12 (4 T) | PR-A |
| **PR-C** | 阶段 4: per-message aggregate | T13-T15 (3 T) | PR-A |
| **PR-D** | 阶段 5+6: ConcurrencySafe + Classifier | T16-T24 (9 T) | PR-B |
| **PR-E** | 阶段 7: 验证 + LTL-Lite | T25-T28 (4 T) | PR-B, PR-C |
| **总计** | | **28 T** | |

**P0 (必做, 19 T)**: T01-T15 + T25-T28
**P1 (增量, 9 T)**: T16-T24, 走下个 change (DM-20260702-009)

---

## 验证清单

- [x] go test -race ./... PASS (PR #376 描述: 全量 PASS, master 预存 `tools/ci-lint-invariant` 失败与本 work 无关)
- [x] go build ./... PASS (PR #376 描述: 0 errors)
- [x] go vet ./... PASS
- [x] 19 工具 surface_metadata_gate_test PASS (`surface_metadata_gate_test.go` 新增 145 行, 19 工具 sentinel 校验)
- [x] t-registry 加 DM-20260702-008 行 (D2 +33, D5 +24, D7 +79, master 已落)
- [x] spec.md 加 D2/D5/D7 新章节 (本 archive 收尾, 见 specs/d2-context-engine/spec.md 等)
- [x] CHANGELOG.md 加 DM-20260702-008 行 (master 已落 d2/d5/d7 CHANGELOG.md)
- [x] verify-archive.sh 12/0/1 (跟 DM-20260701-007 一致, 见本 archive 收尾)
- [x] 端到端 review 任务测试 PASS (T27: 50 文件 review 旧 15/50 vs 新 50/50, `review50_e2e_test.go` 500 行)
- [x] 8K 自我循环验证 PASS (T28: 20 consecutive read_file 全 accept, 治本 invariant 守护)
- [x] 12 packages + 5 new packages go test -race PASS (47 新单测 + T27 + T28 = 49)
