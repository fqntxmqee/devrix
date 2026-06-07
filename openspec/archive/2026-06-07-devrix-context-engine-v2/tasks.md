# Tasks: devrix-context-engine-v2

**Change ID:** devrix-context-engine-v2
**Demand:** DM-20260607-003
**Status:** S7 Archived (2026-06-07)
**Grill Session:** 2026-06-07

---

## 前置条件

- [x] `devrix-llm-gateway` V1 M1（TokenCounter）+ M3–M6（ChatStream/Adapter/Gateway）已归档可用
- [x] `shared/contracts.ITokenCounter` 已存在于 `internal/shared/contracts/tokencounter.go`（L2/L3 均已实现）
- [x] V1 `devrix-context-engine` 已归档，master 基线绿

---

## Milestone 1: 共享契约 + Token 计数统一

- [x] **T1**: ~~新增 `internal/shared/contracts/tokencounter.go`~~ **已完成**（接口已存在，L2/L3 均已 `var _ contracts.ITokenCounter = (*Counter)(nil)` 编译期验证）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-16
  - Estimate: 0h（已完成）
  - Note: 无需同步更新，双方已对齐

- [x] **T2**: ~~`HeuristicCounter` 实现 `contracts.ITokenCounter`~~ **已完成**（`token/counter.go:85` 已有编译期检查）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-16
  - Estimate: 0h（已完成）

- [x] **T3**: `Pipeline` / `engine` 注入 `contracts.ITokenCounter`，生产默认 gateway；`NewPipeline` 改为 **functional options 模式**
  - L4: L4-CTX-COMPRESS, L4-CTX-STATE
  - L5: L5-CTX-03, L5-CTX-16
  - Estimate: 4h（含 functional options 重构，影响 6 个调用点）

- [x] **T4**: 单元测试 `token/counter_test.go` + 基准样例集（cl100k_base 误差 <5%）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-16
  - Estimate: 3h

- [x] **T5**: 扩展 `shared/config/contextengine.go` — `token_counter.source`
  - L4: L4-CTX-STATE
  - L5: —
  - Estimate: 1h

---

## Milestone 2: Autocompact（步骤 6）

- [x] **T6**: 实现 `compression/autocompact.go`（`autocompact.model` 直传 `LLMRequest.Model`）
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-12, L5-CTX-13
  - Estimate: 6h

- [x] **T7**: 修改 `compression/pipeline.go` — 执行序 1-4 → 6 → 5 → 7（Autocompact 在 Assembly 之前，不含 system prompt）
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-08, L5-CTX-12
  - Estimate: 3h
  - Note: 更新 `pipeline_test.go` 覆盖 `enabled=true/false`；`len(turns) <= head+tail` 时不触发

- [x] **T8**: 扩展配置 `compression.autocompact.*` + 错误码 `CTX_AUTOCOMPACT_4010`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-13
  - Estimate: 2h

- [x] **T9**: 单元测试 `compression/autocompact_test.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-12, L5-CTX-13
  - Estimate: 4h

- [x] **T10**: 验收测试 `tests/acceptance/p0/ctx_autocompact_test.go`
  - L4: L4-CTX-COMPRESS
  - L5: L5-CTX-12
  - Estimate: 3h

---

## Milestone 3: PEV Verify Commands

- [x] **T11**: 实现 `IVerifyCommandRunner` + `verify_runner.go`（exec.CommandContext，无 shell，WorkDir 精确匹配校验）
  - L4: L4-CTX-PEV
  - L5: L5-CTX-14, L5-CTX-15
  - Estimate: 4h

- [x] **T12**: 实现 `verify_commands.go`；`verifyPEV` 从纯函数改为 **PEVEngine 方法**，注入 `IVerifyCommandRunner` 到 PEVEngine
  - L4: L4-CTX-PEV
  - L5: L5-CTX-14, L5-CTX-15
  - Estimate: 4h

- [x] **T13**: 配置 `pev.verify_commands`（executable+args）+ 校验 + 错误码 4011/4012
  - L4: L4-CTX-PEV
  - L5: L5-CTX-15
  - Estimate: 2h

- [x] **T14**: 集成测试 `tests/integration/context_verify_commands_test.go`
  - L4: L4-CTX-PEV
  - L5: L5-CTX-14, L5-CTX-15
  - Estimate: 4h

---

## Milestone 4: Observability + 主路径接线

- [x] **T15**: 新增 `ICompressionObserver` + `IPEVObserver`（两个独立接口）+ 各自 NoOp 默认实现
  - L4: L4-CTX-OBS
  - L5: L5-CTX-17
  - Estimate: 3h

- [x] **T16**: 集成测试 `tests/integration/context_compression_obs_test.go`
  - L4: L4-CTX-OBS
  - L5: L5-CTX-17
  - Estimate: 3h

- [x] **T17**: `cmd/devrix/main.go` + `cmd/devrix-feishu/main.go` 接入真实 LLMGateway + `PEVObserver`
  - L4: L4-CTX-STATE
  - L5: L5-CTX-06, L5-CTX-16, L5-CTX-18
  - Estimate: 3h

- [x] **T18**: 集成测试 `tests/integration/context_llm_gateway_test.go`（recorded fixture + `-tags=live`）
  - L4: L4-CTX-STATE
  - L5: L5-CTX-18
  - Estimate: 3h

- [x] **T19**: 更新 `devrix.yaml` 示例配置
  - L4: L4-CTX-STATE
  - L5: —
  - Estimate: 1h

---

## Milestone 5: 文档与验收

- [x] **T20**: 更新 `openspec/l5-registry.md` — L5-CTX-12~18 状态 IMPLEMENTED
  - L5: L5-CTX-12 ~ L5-CTX-18
  - Estimate: 1h

- [x] **T21**: 更新 `docs/context-engine-design.md` 附录 B/C（V2 决议）
  - L5: —
  - Estimate: 1h

- [x] **T22**: S5 `./scripts/gen-acceptance-report.sh --change devrix-context-engine-v2`
  - L5: 全部 P0
  - Estimate: 1h

---

## 任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 共享契约+Token | 5（2 已完成） | 9h（-2h T1/T2 已完成，+1h M1 buffer） |
| M2 Autocompact | 5 | 18h |
| M3 Verify Commands | 4 | 14h |
| M4 Obs + 接线 | 5 | 13h |
| M5 文档验收 | 3 | 3h |
| **合计** | **22（2 已完成）** | **~57h**（+2h M1 buffer 已含） |

---

## V3 Backlog（本变更不实施）

- [ ] PEV Plan + Milestone DAG
- [ ] LongTerm Memory SQLite
- [ ] 快照 AES 加密
- [ ] 异步 Autocompact
