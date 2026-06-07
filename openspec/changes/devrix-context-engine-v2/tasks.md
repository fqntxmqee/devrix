# Tasks: devrix-context-engine-v2

**Change ID:** devrix-context-engine-v2
**Demand:** DM-20260607-003
**Status:** S3 Ready (pending sign-off)

---

## 前置条件

- [ ] `devrix-llm-gateway` V1 M1（TokenCounter）+ M3（ChatStream/Adapter）可用
- [ ] `shared/contracts.ITokenCounter` 与 llm-gateway 接口对齐（跨 change 联调）
- [ ] V1 `devrix-context-engine` 已归档，master 基线绿

---

## Milestone 1: 共享契约 + Token 计数统一

- [ ] **T1**: 新增 `internal/shared/contracts/tokencounter.go`（L2/L3 共享 `ITokenCounter`）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-16
  - Estimate: 2h
  - Note: 同步更新 `devrix-llm-gateway` T1.x 实现同一接口

- [ ] **T2**: `HeuristicCounter` 实现 `contracts.ITokenCounter`（`token/counter.go`）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-16
  - Estimate: 2h

- [ ] **T3**: `Pipeline` / `engine` 注入 `contracts.ITokenCounter`，生产默认 gateway
  - L4: L4-CTX-COMPRESS, L4-CTX-STATE
  - L5: L5-CTX-03, L5-CTX-16
  - Estimate: 3h

- [ ] **T4**: 单元测试 `token/counter_test.go` + 基准样例集（cl100k_base 误差 <5%）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-16
  - Estimate: 3h

- [ ] **T5**: 扩展 `shared/config/contextengine.go` — `token_counter.source`
  - L4: L4-CTX-STATE
  - L5: —
  - Estimate: 1h

---

## Milestone 2: Autocompact（步骤 6）

- [ ] **T6**: 实现 `compression/autocompact.go`（`autocompact.model` 直传 `LLMRequest.Model`）
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-12, L5-CTX-13
  - Estimate: 6h

- [ ] **T7**: 修改 `compression/pipeline.go` — 执行序 1-4 → 6 → 5 → 7
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-08, L5-CTX-12
  - Estimate: 3h
  - Note: 更新 `pipeline_test.go` 覆盖 `enabled=true/false`

- [ ] **T8**: 扩展配置 `compression.autocompact.*` + 错误码 `CTX_AUTOCOMPACT_4010`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-13
  - Estimate: 2h

- [ ] **T9**: 单元测试 `compression/autocompact_test.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-12, L5-CTX-13
  - Estimate: 4h

- [ ] **T10**: 验收测试 `tests/acceptance/p0/ctx_autocompact_test.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-12
  - Estimate: 3h

---

## Milestone 3: PEV Verify Commands

- [ ] **T11**: 实现 `IVerifyCommandRunner` + `verify_runner.go`（exec.CommandContext，无 shell）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-14, L5-CTX-15
  - Estimate: 4h

- [ ] **T12**: 实现 `verify_commands.go`，扩展 `verifyPEV`（与 `pev_engine.go` 同包）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-14, L5-CTX-15
  - Estimate: 4h

- [ ] **T13**: 配置 `pev.verify_commands`（executable+args）+ 校验 + 错误码 4011/4012
  - L4: L4-CTX-PEV
  - L5: L5-CTX-15
  - Estimate: 2h

- [ ] **T14**: 集成测试 `tests/integration/context_verify_commands_test.go`
  - L4: L4-CTX-PEV
  - L5: L5-CTX-14, L5-CTX-15
  - Estimate: 4h

---

## Milestone 4: Observability + 主路径接线

- [ ] **T15**: 新增 `ICompressionObserver` + NoOp 默认实现
  - L4: L4-CTX-OBS
  - L5: L5-CTX-17
  - Estimate: 3h

- [ ] **T16**: 集成测试 `tests/integration/context_compression_obs_test.go`
  - L4: L4-CTX-OBS
  - L5: L5-CTX-17
  - Estimate: 3h

- [ ] **T17**: `cmd/devrix/main.go` + `cmd/devrix-feishu/main.go` 接入真实 LLMGateway
  - L4: L4-CTX-STATE
  - L5: L5-CTX-06, L5-CTX-16, L5-CTX-18
  - Estimate: 3h

- [ ] **T18**: 集成测试 `tests/integration/context_llm_gateway_test.go`（recorded fixture + `-tags=live`）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-18
  - Estimate: 3h

- [ ] **T19**: 更新 `devrix.yaml` 示例配置
  - L4: L4-CTX-STATE
  - L5: —
  - Estimate: 1h

---

## Milestone 5: 文档与验收

- [ ] **T20**: 更新 `openspec/l5-registry.md` — L5-CTX-12~18 状态 IMPLEMENTED
  - L5: L5-CTX-12 ~ L5-CTX-18
  - Estimate: 1h

- [ ] **T21**: 更新 `docs/context-engine-design.md` 附录 B/C（V2 决议）
  - L5: —
  - Estimate: 1h

- [ ] **T22**: S5 `./scripts/gen-acceptance-report.sh --change devrix-context-engine-v2`
  - L5: 全部 P0
  - Estimate: 1h

---

## 任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 共享契约+Token | 5 | 11h |
| M2 Autocompact | 5 | 18h |
| M3 Verify Commands | 4 | 14h |
| M4 Obs + 接线 | 5 | 13h |
| M5 文档验收 | 3 | 3h |
| **合计** | **22** | **~59h** |

---

## V3 Backlog（本变更不实施）

- [ ] PEV Plan + Milestone DAG
- [ ] LongTerm Memory SQLite
- [ ] 快照 AES 加密
- [ ] 异步 Autocompact
