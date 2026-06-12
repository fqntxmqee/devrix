# Devrix 测试点注册表（T 层）

**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-12
**Layering Spec:** `openspec/specs/project/dsaft-methodology.md`

> 本注册表即 DSAFT **T 层**资产登记。所有测试点使用 DSAFT T 层标准编号。
>
> **编号格式**: `D{X}-S{X}-A{XX}-T{XX}`（T 归属 A）或 `D{X}-S{X}-A{XX}-F{XX}-T{XX}`（T 归属 F）
>
> **迁移状态**: ✅ 已完成 (2026-06-12)。所有 130+ 条目已从过渡格式 `D{X}-S{X}-T{NN}` 升级为标准格式 `D{X}-S{X}-A{XX}-T{NN}`。遗留 ID 映射见文末 [Legacy ID Mapping](#legacy-id-mapping)。

---

## 注册规则

| 字段 | 说明 |
|------|------|
| ID | DSAFT T 编号：`D{X}-S{X}-A{XX}-T{XX}` 或 `D{X}-S{X}-A{XX}-F{XX}-T{XX}` |
| Priority | P0（阻断交付）/ P1（需执行，失败记例外）/ P2（尽力） |
| S 映射 | 关联的 Scenario 模块 ID |
| Test 位置 | 测试文件路径 |
| Status | PLANNED / IMPLEMENTED / DEPRECATED |

---

## D1: Communication Domain (COMM)

> **Spec Reference:** `openspec/archive/2026-06-08-devrix-d1-d6-testing/demand.md`

### D1-S1: Gateway Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S1-A01-T01 | 新会话创建被拒绝 | Gateway | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| D1-S1-A01-T02 | IM 入口实例 Register/Unregister | Gateway | `internal/layers/communication/instance/registry_test.go` | IMPLEMENTED | P2 |
| D1-S1-A01-T03 | `buildCompletionSummary` 注入 ctx_pct 段（含/省略/clamp/异常） | Gateway | `internal/layers/communication/gateway/summary_test.go` | IMPLEMENTED | P1 |
| D1-S1-A01-T04 | `ComputeCtxPct` 边界：0 prompt / 0 max / 负数 / 超限 clamp | Gateway | `internal/layers/communication/gateway/summary_test.go` | IMPLEMENTED | P1 |

### D1-S5: Milestone Module

> **Change:** `devrix-v3-integration` (DM-20260608-010)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S5-A01-T01 | Milestone 环检测拒绝循环依赖 | Milestone | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T02 | TaskFlow 多里程碑链顺序执行至完成 | Milestone | `internal/layers/communication/milestone/taskflow_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T03 | 无 V1 TaskFlow stub 误导日志 | Milestone | — (file removed) | IMPLEMENTED | P2 |

### D1-S3: Commands Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S3-A01-T01 | /new 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| D1-S3-A01-T02 | /help 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |
| D1-S3-A01-T03 | /stop 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |

### D1-S8: Renderers Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S8-A01-T01 | ShortId 唯一且排除异议字符 | Renderers | `internal/shared/types/shortid_test.go` | IMPLEMENTED | P1 |
| D1-S8-A01-T02 | ProgressBar / StatusBadge 渲染输出合法 | Renderers | `internal/layers/communication/renderers/components_test.go` | IMPLEMENTED | P2 |

### D1-S2: Adapters Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S2-A01-T01 | 飞书消息解析正确 | Adapters | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| D1-S2-A01-T02 | 钉钉 Webhook 入站路由 + Session 出站 | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T03 | 钉钉 milestone 出站走 CardRenderer | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T04 | Cardkit 双步发卡成功 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T05 | 元素级流式 PUT sequence 递增 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T06 | cardkit 失败降级 Patch | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T07 | complete 关闭 streaming_mode | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T08 | 流式节流配置生效 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P1 |

### D1-S9: EventBus Module

> **Change:** `devrix-event-channel` (DM-20260611-003)
> 注意：本节 T ID 由原 `D2-S3-A01-T01~07` 重编号而来。原 ID 与 D2-S3 Memory 冲突，2026-06-12 重命名以解耦。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S9-A01-T01 | BackpressureEventBus 正常事件流不丢 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T02 | 背压触发 Drain 排空非关键事件 | EventBus | `internal/layers/communication/eventbus/drain_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T03 | Compact 同类事件合并 | EventBus | `internal/layers/communication/eventbus/compact_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T04 | Reconnect 重建通道后继续处理 | EventBus | `internal/layers/communication/eventbus/reconnect_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T05 | Critical 事件（complete）必达不被丢弃 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T06 | Error 事件必达不被丢弃 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T07 | Publish 在高水位阻塞回压到上游 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |

### D0: Cross-Domain Architecture Compliance (LAYER)

> **Change:** `devrix-layer-isolation` (DM-20260611-002)
> 跨域架构合规：分层依赖 lint + 跨层契约注册表 + D6 探针

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| CROSS-A01-T01 | D2→D1 反向 import 数为 0 | Layer | `internal/lint/layer/regression_test.go` | IMPLEMENTED | P0 |
| CROSS-A01-T02 | 分层 lint 规则检测到违规时阻断 | Layer | `cmd/devrix-layer-lint/main_test.go` | IMPLEMENTED | P0 |
| CROSS-A02-T03 | 契约注册表覆盖所有跨层接口 | Layer | `internal/shared/contracts/registry_test.go` | IMPLEMENTED | P1 |
| CROSS-A01-T04 | D6 LayerViolationProbe 运行时探针 | Layer | `internal/layers/evolution/eval/layer_violation_probe_test.go` | IMPLEMENTED | P1 |

---

## D2: Context Engine Domain (CTX)

> **Domain tests:** `./scripts/test-domain.sh d2` · **Spec:** `openspec/specs/testing-framework/domain-segmentation.md`

> V1 已归档：`openspec/archive/2026-06-07-devrix-context-engine/`
> V2 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v2/`
> V3 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v3/`
> V4 已归档：`openspec/archive/2026-06-08-devrix-context-engine-v4/`

### D2-S3: Memory Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S3-A01-T01 | 新会话历史正确追加 | Memory | `internal/layers/contextengine/memory/manager_test.go` | IMPLEMENTED |
| D2-S3-A01-T02 | ContextSnapshot 备份 | Memory | `internal/layers/contextengine/snapshot/store_test.go` | IMPLEMENTED |
| D2-S3-A01-T03 | LongTerm Recall 注入上下文 | Memory | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| D2-S3-A01-T04 | LongTerm Store 持久化写入 | Memory | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |
| D2-S3-A01-T05 | L3 长期记忆返回 NotImplemented | Memory | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |
| D2-S3-A01-T06 | 快照使用 snappy 压缩体积显著缩减 | Memory | `internal/layers/contextengine/snapshot/store_test.go` | IMPLEMENTED |

### D2-S2: Compression Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S2-A01-T01 | 超 Token 阈值触发七步压缩 | Compression | `tests/acceptance/p0/ctx_compression_test.go` | IMPLEMENTED |
| D2-S2-A01-T02 | TokenBlock 超限返回 ContextExceeded | Compression | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| D2-S2-A01-T03 | Autocompact 触发并降低 token | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| D2-S2-A01-T04 | Autocompact LLM 失败降级跳过 | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| D2-S2-A01-T05 | Autocompact 禁用时跳过步骤 6 | Compression | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| D2-S2-A01-T06 | 异步压缩占位不阻塞主路径 | Compression | `internal/layers/contextengine/compression/async_compact_test.go` | IMPLEMENTED |
| D2-S2-A01-T07 | 异步压缩失败降级不丢失数据 | Compression | `internal/layers/contextengine/compression/async_compact_test.go` | IMPLEMENTED |
| D2-S2-A01-T08 | Autocompact timeout fallback | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |

### D2-S1: PEV Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S1-A01-T01 | PEV Execute 调用 LLM 并流输出 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| D2-S1-A02-T02 | 工具执行 Verify basic 模式 | PEV | `internal/layers/contextengine/verify_runner_test.go` | IMPLEMENTED |
| D2-S1-A01-T03 | EngineEvent 与通信层四握约定一致 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| D2-S1-A01-T04 | 批准/拒绝 PEV 行为正确 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| D2-S1-A02-T05 | Verify commands 全部通过 | PEV | `tests/integration/context_verify_commands_test.go` | IMPLEMENTED |
| D2-S1-A02-T06 | Verify 命令失败触发重试 | PEV | `internal/layers/contextengine/verify_runner_test.go` | IMPLEMENTED |
| D2-S1-A03-T07 | Milestone 按序执行 | PEV | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |
| D2-S1-A03-T08 | milestone_progress 事件正确投射 | PEV | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| D2-S1-A02-T09 | Verify timeout kills command (DeadlineExceeded) | PEV | `tests/integration/context_verify_commands_test.go` | IMPLEMENTED |
| D2-S1-A02-T10 | Shell injection attack prevention | PEV | `tests/security/shell_injection_test.go` | IMPLEMENTED |
| D2-S1-A01-T11 | PEV concurrent session isolation | PEV | `internal/layers/contextengine/pev_engine_test.go` | IMPLEMENTED |
| D2-S1-A01-T12 | PEV context cancellation cleanup | PEV | `internal/layers/contextengine/pev_engine_test.go` | IMPLEMENTED |
| D2-S1-A01-T13 | PEV emit complete 透传 ctx_pct + llm_called（主路径/milestone-only 区分） | PEV | `internal/layers/contextengine/pev_engine.go` | IMPLEMENTED | P1 |
| D2-S1-A01-T14 | query loop runSpan 含 pev.prompt_tokens / pev.completion_tokens / pev.ctx_pct | PEV | `internal/layers/contextengine/query_loop_run.go` | IMPLEMENTED | P1 |

### D2-S4: Token Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S4-A01-T01 | Token 计数共享约定与 Gateway 对齐 | Token | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |

### D2-S8: Sandbox Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S8-A01-T01 | bash 困居工作目录 + 命令白名单 | Sandbox | `internal/layers/contextengine/sandbox_test.go` | IMPLEMENTED |

### D2-S9: Harness Module

> **Change:** `devrix-harness-bootstrap` (DM-20260609-004)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S9-A01-T01 | harness.enabled 首次 Process 触发 Bootstrap | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T02 | WorkspaceContext 扫描 Go 文件与 AGENTS.md | Harness | `internal/layers/contextengine/harness/workspace_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T03 | Bootstrap 幂等（同 Session 不重复） | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T04 | trusted=false 时 deferred_init 标志全 false | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A03-T05 | ToolPool simple_mode / MCP / deny 过滤 | Harness | `internal/layers/contextengine/harness/toolpool_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T06 | PromptRouter advisory 关键词计分 | Harness | `internal/layers/contextengine/harness/router_test.go` | IMPLEMENTED | P2 |
| D2-S9-A01-T07 | Transcript 内存分离与 compact | Harness | `internal/layers/contextengine/harness/transcript_test.go` | IMPLEMENTED | P1 |
| D2-S9-A01-T08 | harness.enabled=false V4 回归 + 无 bootstrap info | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T09 | Preflight warn-only 规则评分与 tool filter | Harness | `internal/layers/contextengine/harness/preflight_test.go` | IMPLEMENTED | P1 |
| D2-S9-A02-T10 | System Prompt Assembly §十 XML 块 | Harness | `internal/layers/contextengine/harness/system_prompt_assembler_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T11 | Jaeger span 树（enabled/disabled） | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |
| D2-S9-A02-T12 | disabled 与 BuildLegacy 字节级一致 | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A02-T13 | CompressedView system = Build 输出 | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A03-T14 | PEV 可见工具 ⊆ VisibleTools | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T15 | bootstrap.stage parent = bootstrap.run | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |

### D2-S9.BG: Background SubQuery Task Tools

> **Change:** `devrix-background-task-tools` (DM-20260611-009)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S9-A01-T16 | stop running task → cancelled (idempotent) | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T17 | output block=false 返回 running 状态 + partial result | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T18 | output block=true 阻塞至 terminal 或 timeout（max 600s） | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| D2-S9-A01-T19 | cancel 后 SessionQueue 不发 completed notification（tombstone 协议） | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` | IMPLEMENTED | P1 |

### D2-S10: QueryLoop Module

> **Change:** `devrix-queryloop-context` (DM-20260610-012)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S10-A01-T34 | 多轮 tool_use 直至无 tool | QueryLoop | `internal/layers/contextengine/query/loop_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T35 | UserContext prepend 不在 snapshot | QueryLoop | `internal/layers/contextengine/usercontext/` | IMPLEMENTED | P0 |
| D2-S10-A01-T36 | plan_mode attachment full/sparse throttle | QueryLoop | `internal/layers/contextengine/attachments/registry.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T37 | plan mode 拒绝 Write 非 plan 文件 | QueryLoop | `internal/layers/contextengine/permission/mode_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T38 | task_create 磁盘持久 + list 一致 | QueryLoop | `internal/layers/contextengine/tasks/disk_store_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T39 | query_loop.enabled=false V4 回归 | QueryLoop | `tests/integration/query_loop_integration_test.go` | IMPLEMENTED | P0 |
| D2-S10-A01-T40 | SubQuery Explore omitClaudeMd + read-only | QueryLoop | `internal/layers/contextengine/query/subquery_test.go` | IMPLEMENTED | P1 |
| D2-S10-A01-T41 | Fork subagent placeholder tool_results 一致 | QueryLoop | `internal/layers/contextengine/query/fork_test.go` | IMPLEMENTED | P1 |
| D2-S10-A01-T42 | sidechain transcript resume 重建 messages | QueryLoop | `internal/layers/contextengine/transcript/sidechain_test.go` | IMPLEMENTED | P1 |

### D2-S11: Harness Unification (Legacy 退役)

> **Change:** `devrix-harness-unification` (DM-20260611-004)
> 注意：2026-06-12 S4-Gate 修正 — 原 D2-S9-A01-T01~04 / TD01 / TD03 / D6PR 与 §D2-S9 / §D2-S9.BG 编号冲突，重编号为 D2-S11-*。原 ID 在 test 注释中保留为历史交叉引用。

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S11-A01-T01 | `query_loop.enabled` 默认 true | HarnessUnification | `internal/shared/config/queryloop_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T02 | harnessEnabled 分支不再被生产路径触发 (D5 runtime.path_resolved_total{path=query_loop}++) | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T03 | 旧路径调用计数基线=0 (100 次 Process 循环后 LegacyHarness=0) | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| D2-S11-A01-T04 | 压缩入口统一：QueryLoop 走 messages-only 七步管道 (`WithSkipAssembly=true`) | HarnessUnification | `internal/layers/contextengine/compression_unified_test.go` | IMPLEMENTED | P1 |
| D2-S11-A01-TD01 | TD-QL-01: 413 → 一轮 messages-only 压缩 → 重试 LLM.Call | HarnessUnification | `internal/layers/contextengine/query/loop_recovery_test.go` | IMPLEMENTED | P1 |
| D2-S11-A01-TD03 | TD-QL-03: overload/5xx → 切换 fallback model (**生产未接线，见 S4-Gate 报告**) | HarnessUnification | `internal/layers/contextengine/query/loop_fallback_test.go` | PARTIAL | P1 |
| D2-S11-A01-D6PR | D6 PathRegressionProbe: legacy_harness > 0 ⇒ score 0 (CI 阻断) | HarnessUnification | `internal/layers/evolution/eval/path_regression_probe_test.go` | IMPLEMENTED | P0 |

### D2-S12: Worktree Module

> **Change:** `devrix-queryloop-context` v2.0 (DM-20260610-012)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D2-S12-A01-T01 | Worktree enter 后 write 不污染主 WorkDir | Worktree | `internal/layers/contextengine/worktree/manager_test.go` | IMPLEMENTED | P0 |

### D2: Cross-Scenario Tests

| T ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| D2-S0-A01-T01 | 压缩/Verify 步骤可观事务 | `tests/integration/context_compression_obs_test.go` | IMPLEMENTED |
| D2-S0-A01-T02 | 主路径接入真实 LLM Gateway | `tests/integration/context_llm_gateway_test.go` | IMPLEMENTED |
| D2-S0-A01-T03 | plan.enabled=false 时回退 V2 路径 | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |

---

## ORCH: Orchestration Layer (Cross-Domain)

> **Change:** `devrix-queryloop-context` v2.0 (DM-20260610-012) · Package: `internal/layers/orchestration/`

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| ORCH-S2-T01 | WorkPlan.Snapshot 含 Task + ExecutionFlow | WorkPlan | `internal/layers/orchestration/workplan/service_test.go` | IMPLEMENTED | P0 |

### ORCH-S2: Wave Scheduler Module

> **Change:** `devrix-wave-scheduler` (DM-20260611-007)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| ORCH-S2-T10 | DAG 6 ready subagent + 1 cursor 持续调度峰值并发=5 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T11 | upstream policy 收到 A artifact，无 Leader 全量 | ContextPolicy | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T12 | fresh policy SubAgent 启动 Messages 仅含 directive | ContextPolicy | `internal/layers/orchestration/wave/context_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T13 | 同 conflict_group Task 不并行 | ConflictGuard | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T14 | 每 Task 独立双区块 IM 卡流式 | WorkerCard | `internal/layers/communication/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T15 | 槽位释放后 ready Task 立即派发 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T16 | cursor + claude-code 并行 file_scope 不交 | ConflictGuard | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P1 |
| ORCH-S2-T17 | Plan 产出 DAG 仅 ready 节点被派发 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T18 | wave 全完成 Leader 收到 wave_completed 汇总 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P1 |
| ORCH-S2-T19 | CancelWorker 槽位释放 status=cancelled | WorkerLifecycle | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T20 | CancelAll 5 running 全部 terminal pool 全释放 | WorkerLifecycle | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T21 | CLI Worker cancel 进程终止 IM 卡 cancelled | WorkerLifecycle | `internal/layers/orchestration/wave/runners/agent_tool_l5_21_test.go` | PARTIAL | P1 |

---

## D3: LLM Gateway Domain (LLM)

> **Domain tests:** `./scripts/test-domain.sh d3` · **Live:** `--live` · **Spec:** `domain-segmentation.md`

### D3-S1: Adapter Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S1-A01-T01 | DeepSeek 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/deepseek_test.go` | IMPLEMENTED |
| D3-S1-A01-T02 | MiniMax 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/minimax_test.go` | IMPLEMENTED |
| D3-S1-A01-T03 | SSE parse error handling | Adapter | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

### D3-S3: Breaker Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S3-A01-T01 | Circuit breaker 正常关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T02 | Circuit breaker 触发放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T03 | Circuit breaker 半开→关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T04 | Circuit breaker 半开→放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T05 | 熔断器状态长久化 | Breaker | - | PLANNED |

### D3-S5: Token Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S5-A01-T01 | Token 计数准确性 (cl100k_base) | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| D3-S5-A01-T02 | Token 预算检查 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| D3-S5-A01-T03 | Token counter 中文准确性 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |

### D3-S6: Config Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S6-A01-T01 | Provider 配置加载 | Config | `internal/layers/llmgateway/config/loader_test.go` | IMPLEMENTED |

### D3-S4: Retry Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S4-A01-T01 | 重试策略执行 | Retry | `internal/layers/llmgateway/retry/retry_test.go` | IMPLEMENTED |
| D3-S4-A01-T02 | DeepSeek Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| D3-S4-A01-T03 | MiniMax Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |

### D3-S2: Gateway Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S2-A01-T01 | LLM 调用可观测事件 | Gateway | `tests/integration/llm_observer_test.go` | IMPLEMENTED |
| D3-S2-A01-T02 | 未知 Provider/Model 报错 | Gateway | `internal/layers/llmgateway/gateway/router_test.go` | IMPLEMENTED |
| D3-S2-A01-T03 | 多 Provider 并发调用 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T04 | Retry 与 CB 联动，context 取消不触发 CB | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T05 | Half-Open 并发探测上游 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T06 | LLM 429 rate limit handling | Gateway | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

---

## D4: Multi-Agent Domain (AGENT)

> **Domain tests:** `./scripts/test-domain.sh d4` · **Spec:** `domain-segmentation.md`

> 已归档：`openspec/archive/2026-06-08-devrix-multi-agent/`（Demand: DM-20260608-005）

### D4-S1: Factory Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S1-A01-T01 | AgentFactory 创建 Agent 实例 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |

### D4-S2: Agent Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S2-A01-T01 | Agent 生命周期状态转换 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S2-A02-T02 | AgentPermissionGate 批准/拒绝/超时 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |
| D4-S2-A02-T03 | CRITICAL 工具权限异步流程 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |

### D4-S3: ForkJoin Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S3-A01-T01 | Fork/Join 消息隔离模型 | ForkJoin | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S3-A01-T02 | Fork 双层限额 MaxChildren+MaxTotalAgents | ForkJoin | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |
| D4-S3-A01-T03 | Agent 超时自动终止 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S3-A01-T04 | Context 取消传播到子 Agent | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S3-A02-T05 | Join 排序 + tool_call ID 去重 + SessionView COW | SessionView | `internal/layers/multiagent/sessionview/sessionview_test.go` | IMPLEMENTED |

### D4-S4: Collaboration Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S4-A01-T01 | CoT prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |
| D4-S4-A01-T02 | Iterative-Refinement prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |

### D4-S5: Observer Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D4-S5-A01-T01 | ObserverAdapter 桥接 AgentEvent → IObserver | Observer | `internal/layers/multiagent/observer/adapter.go` | IMPLEMENTED |

### D4-S6: Agent Tool Module

> **Change:** `devrix-agent-tools` (DM-20260608-012)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S6-A01-T01 | Agent Tool Registry 注册/查找/按能力查询 | AgentTool | `internal/layers/multiagent/tool/registry_test.go` | IMPLEMENTED | P0 |
| D4-S6-A02-T02 | CLI 适配器正常启动子进程并解析 stream-json | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |
| D4-S6-A02-T03 | CLI 适配器超时正确终止子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T04 | Session 首次创建子进程，后续调用复用的同一进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |
| D4-S6-A02-T05 | Session 空闲超时自动回收子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T06 | D1 Session 销毁清理关联的 Agent Tool 子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| D4-S6-A02-T07 | 不同 D1 Session 的 Agent Tool 隔离运行互不干扰 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |

### D4-S10: Delegate Module

> **Change:** `devrix-queryloop-context` v2.0 (DM-20260610-012)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D4-S10-A01-T01 | Leader delegate_explore 创建 Worker / MaxWorkers | Delegate | `internal/layers/multiagent/delegate/service_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T02 | Worker Run 设置 AgentID，sidechain 隔离 | Delegate | `internal/layers/contextengine/worker_tools_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T03 | Worker 不能 delegate_* 或 Fork | Delegate | `internal/layers/multiagent/agent/worker_engine_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T04 | delegate-progress 仅 Leader Drain | Delegate | `internal/layers/contextengine/queue/delegate_progress_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T05 | worker_progress 到达 Gateway/IM | Delegate | `internal/layers/orchestration/imsink/gateway_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T06 | SubQuery 与 D4 Worker 共用 FlowEvent schema | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D4-S10-A02-T07 | FlowStarted 自动 task owner + in_progress | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T08 | D4 未启用 delegate 降级 SubQuery | Delegate | `internal/layers/contextengine/delegate_fallback_flow_test.go` | IMPLEMENTED | P0 |
| D4-S10-A01-T09 | 用户单会话：无第二对话入口 | Delegate | `internal/bootstrap/cli_events_test.go` | IMPLEMENTED | P0 |

### D4: Cross-Scenario Tests

| T ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| D4-S0-A01-T01 | Agent 并发安全 (-race) | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S0-A01-T02 | Fork 消息隔离并发安全 | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| D4-S0-A01-T03 | Gateway → ResolvePermission 集成全流程 | `tests/integration/agent_integration_test.go` | IMPLEMENTED |
| D4-S0-A01-T04 | E2E Fork 端到端 | `tests/e2e/agent_fork_e2e_test.go` | IMPLEMENTED |

---

## D5: Observability Domain (OBS)

### D5-S2: Metrics Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S2-A01-T01 | Tracing Span 创建与传播 | Metrics | - | PLANNED |
| D5-S2-A01-T02 | Metrics Counter 计数 | Metrics | - | PLANNED |
| D5-S2-A01-T03 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | Metrics | `internal/layers/observability/metrics/gauge_test.go` | IMPLEMENTED |
| D5-S2-A01-T04 | Histogram Prometheus 输出与 golden 一致 | Metrics | `internal/layers/observability/metrics/histogram_test.go` | IMPLEMENTED |
| D5-S2-A01-T05 | Int64UpDownCounter 返回 Gauge | Metrics | `internal/layers/observability/metrics/meter_test.go` | IMPLEMENTED |
| D5-S2-A01-T06 | Compression P99 latency < 500ms | Metrics | `tests/performance/compression_test.go` | IMPLEMENTED |
| D5-S2-A01-T07 | Concurrent session memory bounded | Metrics | `tests/performance/memory_test.go` | IMPLEMENTED |

### D5-S3: Logger Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S3-A01-T01 | 日志级别过滤 | Logger | - | PLANNED |
| D5-S3-A01-T02 | Shutdown 覆盖 Tracer + Logger | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| D5-S3-A01-T03 | Error 日志包含 stacktrace 字段 | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| D5-S3-A01-T04 | 日志采样 max_entries_per_span 生效 | Logger | `internal/layers/observability/logger/sampling_test.go` | IMPLEMENTED |

### D5-S1: Tracer Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S1-A01-T01 | Shutdown 刷写所有 pending spans | Tracer | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED |
| D5-S1-A01-T02 | ConsoleExporter 可直接作为 SpanExporter | Tracer | `internal/layers/observability/exporter/console_test.go` | IMPLEMENTED |

### D5-S4: Exporter Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S4-A01-T01 | LongTerm recall/store 产生 canonical Operation span | Exporter | `internal/layers/contextengine/engine.go` | IMPLEMENTED |
| D5-S4-A01-T02 | Plan 产生 Milestone Run 产生 canonical Operation span | Exporter | `internal/layers/contextengine/pev_engine.go` | IMPLEMENTED |
| D5-S4-A01-T03 | Feishu 入站产生 adapter.message.receive span | Exporter | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED |

### D5-S5: Coverage Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S5-A01-T01 | Operation Registry 与 names.go 常驻全集一致 | Coverage | `internal/layers/observability/coverage/registry_test.go` | IMPLEMENTED |
| D5-S5-A01-T02 | Coverage 报告正确列出 zero_hit operations | Coverage | `tests/integration/obs_coverage_test.go`, `tests/integration/context_harness_obs_test.go` | IMPLEMENTED |

### D5: Cross-Scenario Tests

| T ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| D5-S0-A01-T01 | Gateway 会话 | - | PLANNED |

---

## D6: Evolution Domain (EVO)

> **Spec Reference:** `openspec/specs/eval/spec.md` (D6-S3); D6-S1/S2 见 `openspec/archive/2026-06-08-devrix-d1-d6-testing/demand.md`

### D6-S1: Version Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D6-S1-A01-T01 | 版本检测与记录（PlannedVersion: v2.1.0） | Version | `internal/layers/evolution/version/version_test.go` | PLANNED | P2 |

### D6-S2: Config Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D6-S2-A01-T01 | 配置热更新（PlannedVersion: v2.2.0） | Config | `internal/layers/evolution/config/hotreload_test.go` | PLANNED | P2 |

### D6-S3: Eval Module (Pilot)

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D6-S3-A01-T01 | EvalRun 编排 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED | P0 |
| D6-S3-A02-T02 | LLM-as-Judge 校准与分歧 | Eval | `internal/layers/evolution/eval/judge_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T03 | Compression Recall Probe F1 | Eval | `internal/layers/evolution/eval/compression_recall_probe_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T04 | Delta 报告对比 | Eval | `internal/layers/evolution/eval/delta_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T07 | eval.enabled=false 零行为 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED | P0 |
| D6-S3-A01-T06 | PEV Tool 选择准确率探针 | Eval | `internal/layers/evolution/eval/pev_tool_accuracy_probe_test.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T11 | devrix eval run 子命令 | Eval | `internal/cli/eval/run_test.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T12 | 调优建议生成 | Eval | `internal/layers/evolution/eval/tune_test.go` | IMPLEMENTED | P2 |
| D6-S3-A01-T13 | eval run 真实 Judge 接入 | Eval | `internal/cli/eval/judge.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T09 | Provider 质量对比探针 | Eval | `internal/layers/evolution/eval/provider_quality_probe_test.go` | IMPLEMENTED | P1 |
| D6-S3-A01-T10 | Agent Fork/Join 质量探针 | Eval | `internal/layers/evolution/eval/agent_forkjoin_probe_test.go` | IMPLEMENTED | P2 |
| D6-S3-A01-T14 | Eval CI delta gate | Eval | `internal/layers/evolution/eval/gate_test.go` | IMPLEMENTED | P2 |
| D6-S3-A01-T15 | run-eval.sh CI 脚本 | Eval | `scripts/eval/run-eval.sh` | IMPLEMENTED | P2 |

---

## D0: Code Integrity Domain (INTEGRITY)

> 跨域代码健康规范。属架构治理层，非业务域。
> **Spec Reference:** `openspec/archive/2026-06-08-devrix-code-integrity/demand.md`

### D0-S1: Specification Module

| T ID | 描述 | Test 位置 | Status | Priority |
|-------|------|-----------|--------|----------|
| D0-S1-A01-T01 | `coding.md §9` 包含不可变性分层规范 | `openspec/specs/project/coding.md` §9 | IMPLEMENTED | P0 |
| D0-S1-A01-T02 | CLAUDE.md 引用新规范 | `CLAUDE.md` | IMPLEMENTED | P0 |
| D0-S1-A01-T03 | emitEvent 处理 EventConnectionLostData 不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| D0-S1-A01-T04 | emitEvent 处理 EventConnectionRestoredData 不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| D0-S1-A01-T05 | emitEvent 处理未知类型不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| D0-S1-A01-T06 | 新会话创建被拒绝 | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| D0-S1-A01-T07 | /new /help /stop 命令解析正确 | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| D0-S1-A01-T08 | 飞书消息解析正确 | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| D0-S1-A01-T09 | D6 L5 注册表条目包含目标版本 | `openspec/t-registry.md` | IMPLEMENTED | P1 |

---

## 状态汇总

| Domain | Domain Name | Total | IMPLEMENTED | PLANNED | P0 |
|--------|------------|-------|-------------|---------|-----|
| D0 | Code Integrity | 9 | 9 | 0 | 3 |
| D1 | Communication | 13 | 13 | 0 | 2 |
| D2 | Context Engine | 41 | 41 | 0 | 7 |
| D3 | LLM Gateway | 21 | 20 | 1 | 0 |
| D4 | Multi-Agent | 24 | 24 | 0 | 9 |
| D5 | Observability | 19 | 15 | 4 | 0 |
| D6 | Evolution | 2 | 0 | 2 | 0 |
| ORCH | Orchestration | 1 | 1 | 0 | 1 |
| **Total** | | **130** | **123** | **7** | **22** |

---

## Legacy ID Mapping

本表记录过渡格式 `D{X}-S{X}-T{NN}`（缺少 A 段）→ 标准格式 `D{X}-S{X}-A{XX}-T{NN}` 的映射，供 CI 脚本、文档交叉引用等外部消费方查阅。

### 格式转换规则

| 过渡格式 | 标准格式 | 规则 |
|----------|----------|------|
| `D{X}-S{X}-T{NN}` | `D{X}-S{X}-A{XX}-T{NN}` | 插入 A 段（默认 A01，特定测试根据语义分配） |
| `CROSS-T{NN}` | `CROSS-A{XX}-T{NN}` | 跨域测试点归属 CROSS 活动 |
| `D2-S11-TD{NN}` | `D2-S11-A01-TD{NN}` | 特殊后缀保留，插入 A 段 |
| `D2-S11-D6PR` | `D2-S11-A01-D6PR` | 探针测试点 |
| `D4-S12-T01` | `D2-S12-A01-T01` | 修正错误的域编号（实际代码在 contextengine/worktree/） |

### 关键映射速查

| 旧 ID | 新 ID | 说明 |
|-------|-------|------|
| D1-S2-T03~T08 | D1-S2-A02-T03~T08 | SendOutbound 活动 |
| D1-S9-T02~T04 | D1-S9-A02-T02~T04 | ManageBusLifecycle 活动 |
| D2-S1-T02,T05,T06,T09,T10 | D2-S1-A02-T* | VerifyExecution 活动 |
| D2-S1-T07,T08 | D2-S1-A03-T07,T08 | PlanExecution 活动 |
| D2-S9-T05,T14 | D2-S9-A03-T05,T14 | FilterToolPool 活动 |
| D2-S9-T10,T12,T13 | D2-S9-A02-T10,T12,T13 | AssembleSystemPrompt 活动 |
| D4-S2-T02,T03 | D4-S2-A02-T02,T03 | ResolvePermission 活动 |
| D4-S3-T05 | D4-S3-A02-T05 | JoinAgents 活动 |
| D4-S6-T02~T07 | D4-S6-A02-T02~T07 | ExecuteAgentTool 活动 |
| D4-S10-T04~T07 | D4-S10-A02-T04~T07 | TrackProgress 活动 |
| D6-S3-T02 | D6-S3-A02-T02 | JudgeResult 活动 |
| CROSS-T03 | CROSS-A02-T03 | CheckContracts 活动 |
| D4-S12-T01 | D2-S12-A01-T01 | 修正域编号（D4→D2） |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Complete rewrite with D-S numbering system |
| 1.1.0 | 2026-06-08 | Section headers migrated L1-L2 → D-S (DM-20260608-007) |
| 1.2.0 | 2026-06-08 | D1/D6 testing specs added; Priority column added (devrix-d1-d6-testing) |
| 1.3.0 | 2026-06-08 | D1 L5 IMPLEMENTED; registry summary reconciled (DM-20260608-011) |
| 1.4.0 | 2026-06-09 | D2/D3/D4 domain build tags + test-domain.sh (DM-20260609-001) |
| 1.5.0 | 2026-06-10 | QueryLoop v1/v2 + Delegate + WorkPlan L5 (DM-20260610-012) |
| 2.0.0 | 2026-06-12 | DSAFT 迁移：编号体系切换为 T 层标准 `D{X}-S{X}-A{XX}-T{XX}`，完成 L5→T 层迁移；删除重复 D5 章节 |
| 3.0.0 | 2026-06-12 | T 层升级：全部 130+ 条目添加 A 段，过渡格式归零；新增 Legacy ID Mapping 表 |

---

> **已删除的重复 D5 章节**（2026-06-12）：原有第二处 D5（D5-S1~S6，含 28 个 {T}-OBS-* 非标准条目）与第一处 D5 模块编号冲突（D5-S1 同为 Tracer/Tracing 但内容不同，D5-S3 分别为 Logger/Coverage）。旧条目需按 DSAFT T 标准重新注册到第一处 D5 对应模块下。原始内容见 git 历史。

