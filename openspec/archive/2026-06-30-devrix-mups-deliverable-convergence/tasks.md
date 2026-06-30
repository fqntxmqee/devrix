# Implementation Tasks: MUPS 交付收敛

**Change ID:** `devrix-mups-deliverable-convergence`  
**Demand ID:** DM-20260630-012

---

## Phase P0 — 交付门控（用户不见过渡句）

### T-P0-1 DeliverableVerifier

- [x] 1.1 新增 `deliverable_verify.go` — `DeliverableSchema` + `VerifyDeliverable`
- [x] 1.2 `p0_p1_file_line`：file:line 正则 + P0/P1 结构检测
- [x] 1.3 `max_iters` 无 citation → Incomplete
- [x] 1.4 单测 `deliverable_verify_test.go` — should_* when_* 命名

**T:** D7-S9-A32-T01

### T-P0-2 Verify + Status 集成

- [x] 2.1 `verifyArtifactForWorkItem` 调用 DeliverableVerifier
- [x] 2.2 `WorkItemPipelineRound.DeliverableStatus` 字段
- [x] 2.3 `StatusAfterSpawnNone` 条件化（incomplete → InProgress）
- [x] 2.4 单测：Partial 无 deliverable 不 Completed

**T:** D7-S9-A32-T02

### T-P0-3 Session complete gate

- [x] 3.1 `session_turn_loop.go` — ExtractSessionDeliverable 优先
- [x] 3.2 接入 `EmitLastTextQualityGate` + meta summary_quality/final_quality
- [x] 3.3 扩展过渡句 marker（EN/ZH）
- [x] 3.4 单测 + 集成 `TestSessionTurnLoop_CompletePrefersRollupDeliverable`

**T:** D7-S2-A73-T03, D7-S2-A73-T04

**P0 Quality Gate:**

- [x] `./scripts/test-unit.sh` PASS
- [x] `go test ./internal/layers/orchestration/sessionorchestrator/... -count=1`

---

## Phase P1 — LLM 战略提案 + 向上载荷

### T-P1-1 StrategicPlanProposer

- [x] 4.1 新增 `strategic_plan_proposer.go` — D2 Prepare → appendix → D3 JSON
- [x] 4.2 `ValidateStrategicPlan` 门控（scope/depth/budget）
- [x] 4.3 `item_pipeline.go` — Plan 前调用 proposer，失败 fallback DefaultPlanner
- [x] 4.4 单测：LLM 提案 single 时不走固定 2 子任务 decompose

**T:** D7-S5-A22-T01, D7-S5-A22-T02, D7-S16-A76-T01

### T-P1-2 StructuredDeliverable + Bubble

- [x] 5.1 `DeliverablePayload` on `WorkItemPipelineRound`
- [x] 5.2 Execute appendix — 最后一轮 JSON schema 提示（WorkItemExecutor + exec ctx）
- [x] 5.3 Verify 解析 JSON → payload
- [x] 5.4 `StructuredBubbleStatement` 含 findings digest
- [x] 5.5 Rollup directive 合并子 findings（Materialize child_finding signals）

**T:** D7-S15-A41-T02

### T-P1-3 Bootstrap + Registry

- [x] 6.1 `wire_item_pipeline.go` wire `NewLLMStrategicPlanProposer`
- [x] 6.2 `wire_item_pipeline_test.go` 断言 wired
- [x] 6.3 更新 `openspec/specs/d7-orchestration/t-registry.md`
- [x] 6.4 change delta spec 合并指引（S6 合入 main）

**T:** D7-S16-A76-T01

### T-P1-4 集成测试

- [x] 7.1 `tests/integration/d7/d7_deliverable_convergence_test.go`
- [ ] 7.2 场景：review kernel — CI stub LLM 环境手工/集成跑通（deferred: staging Jaeger）

**P1 Quality Gate:**

- [x] 关联 T 测试点全部 IMPLEMENTED
- [ ] Jaeger 手工验收树（design §4）一次通过（deferred: next staging deploy）

---

## Phase P2 — Backlog（本 change 不编码）

- [ ] LLM SpawnPolicy proposer（替代 R0-R8 部分规则）
- [ ] Deliverable LLM verifier（schema 之上语义校验）
- [ ] 更多 deliverable_schema 注册（security_audit, api_diff, …）

---

## Completion Checklist

- [x] demand.md AC 全部勾选（实现侧）
- [x] acceptance-report.md 填写
- [x] `openspec/specs/d7-orchestration/` delta 合入 SoT（S6 门禁）
- [x] demand-archive-index 条目（S7）
