# Devrix L5 测试点注册表

**Status:** Active
**Last Updated:** 2026-06-10
**Layering Spec:** `openspec/specs/architecture/layering.md`

> L5 测试点是 OpenSpec S5 验收的确定性锚点。新增 L4/L3 能力时 MUST 先在此记录或复用现有 L5。
>
> **编号格式**: `L5-{D}-{S}-{NN}`
> - D = 域编号 Domain (1-6)
> - S = 场景编号 Scenario (1-8)
> - NN = 序号 (01-99)

---

## 注册规则

| 字段 | 说明 |
|------|------|
| ID | `L5-{D}-{S}-{NN}` |
| Priority | P0（阻断交付）/ P1（需执行，失败记例外）/ P2（尽力） |
| S 映射 | 关联的 Scenario 模块 ID |
| Test 位置 | 测试文件路径 |
| Status | PLANNED / IMPLEMENTED / DEPRECATED |

---

## D1: Communication Domain (COMM)

> **Spec Reference:** `openspec/archive/2026-06-08-devrix-d1-d6-testing/demand.md`

### D1-S1: Gateway Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-1-01 | 新会话创建被拒绝 | Gateway | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| L5-1-1-02 | IM 入口实例 Register/Unregister | Gateway | `internal/layers/communication/instance/registry_test.go` | IMPLEMENTED | P2 |
| L5-1-1-03 | `buildCompletionSummary` 注入 ctx_pct 段（含/省略/clamp/异常） | Gateway | `internal/layers/communication/gateway/summary_test.go` | IMPLEMENTED | P1 |
| L5-1-1-04 | `ComputeCtxPct` 边界：0 prompt / 0 max / 负数 / 超限 clamp | Gateway | `internal/layers/communication/gateway/summary_test.go` | IMPLEMENTED | P1 |

### D1-S5: Milestone Module

> **Change:** `devrix-v3-integration` (DM-20260608-010)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-5-01 | Milestone 环检测拒绝循环依赖 | Milestone | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED | P1 |
| L5-1-5-02 | TaskFlow 多里程碑链顺序执行至完成 | Milestone | `internal/layers/communication/milestone/taskflow_test.go` | IMPLEMENTED | P1 |
| L5-1-5-03 | 无 V1 TaskFlow stub 误导日志 | Milestone | — (file removed) | IMPLEMENTED | P2 |

### D1-S3: Commands Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-3-01 | /new 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| L5-1-3-02 | /help 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |
| L5-1-3-03 | /stop 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |

### D1-S8: Renderers Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-8-01 | ShortId 唯一且排除异议字符 | Renderers | `internal/shared/types/shortid_test.go` | IMPLEMENTED | P1 |
| L5-1-8-02 | ProgressBar / StatusBadge 渲染输出合法 | Renderers | `internal/layers/communication/renderers/components_test.go` | IMPLEMENTED | P2 |

### D1-S2: Adapters Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-2-01 | 飞书消息解析正确 | Adapters | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| L5-1-2-02 | 钉钉 Webhook 入站路由 + Session 出站 | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| L5-1-2-03 | 钉钉 milestone 出站走 CardRenderer | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| L5-1-2-04 | Cardkit 双步发卡成功 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| L5-1-2-05 | 元素级流式 PUT sequence 递增 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| L5-1-2-06 | cardkit 失败降级 Patch | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| L5-1-2-07 | complete 关闭 streaming_mode | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| L5-1-2-08 | 流式节流配置生效 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P1 |

---

## D2: Context Engine Domain (CTX)

> **Domain tests:** `./scripts/test-domain.sh d2` · **Spec:** `openspec/specs/testing-framework/domain-segmentation.md`

> V1 已归档：`openspec/archive/2026-06-07-devrix-context-engine/`
> V2 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v2/`
> V3 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v3/`
> V4 已归档：`openspec/archive/2026-06-08-devrix-context-engine-v4/`

### D2-S3: Memory Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-3-01 | 新会话历史正确追加 | Memory | `internal/layers/contextengine/memory/manager_test.go` | IMPLEMENTED |
| L5-2-3-02 | ContextSnapshot 备份 | Memory | `internal/layers/contextengine/snapshot/store_test.go` | IMPLEMENTED |
| L5-2-3-03 | LongTerm Recall 注入上下文 | Memory | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| L5-2-3-04 | LongTerm Store 持久化写入 | Memory | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |
| L5-2-3-05 | L3 长期记忆返回 NotImplemented | Memory | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |
| L5-2-3-06 | 快照使用 snappy 压缩体积显著缩减 | Memory | `internal/layers/contextengine/snapshot/store_test.go` | IMPLEMENTED |

### D2-S2: Compression Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-2-01 | 超 Token 阈值触发七步压缩 | Compression | `tests/acceptance/p0/ctx_compression_test.go` | IMPLEMENTED |
| L5-2-2-02 | TokenBlock 超限返回 ContextExceeded | Compression | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| L5-2-2-03 | Autocompact 触发并降低 token | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| L5-2-2-04 | Autocompact LLM 失败降级跳过 | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| L5-2-2-05 | Autocompact 禁用时跳过步骤 6 | Compression | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| L5-2-2-06 | 异步压缩占位不阻塞主路径 | Compression | `internal/layers/contextengine/compression/async_compact_test.go` | IMPLEMENTED |
| L5-2-2-07 | 异步压缩失败降级不丢失数据 | Compression | `internal/layers/contextengine/compression/async_compact_test.go` | IMPLEMENTED |
| L5-2-2-08 | Autocompact timeout fallback | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |

### D2-S1: PEV Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-1-01 | PEV Execute 调用 LLM 并流输出 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-2-1-02 | 工具执行 Verify basic 模式 | PEV | `internal/layers/contextengine/verify_runner_test.go` | IMPLEMENTED |
| L5-2-1-03 | EngineEvent 与通信层四握约定一致 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-2-1-04 | 批准/拒绝 PEV 行为正确 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-2-1-05 | Verify commands 全部通过 | PEV | `tests/integration/context_verify_commands_test.go` | IMPLEMENTED |
| L5-2-1-06 | Verify 命令失败触发重试 | PEV | `internal/layers/contextengine/verify_runner_test.go` | IMPLEMENTED |
| L5-2-1-07 | Milestone 按序执行 | PEV | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |
| L5-2-1-08 | milestone_progress 事件正确投射 | PEV | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| L5-2-1-09 | Verify timeout kills command (DeadlineExceeded) | PEV | `tests/integration/context_verify_commands_test.go` | IMPLEMENTED |
| L5-2-1-10 | Shell injection attack prevention | PEV | `tests/security/shell_injection_test.go` | IMPLEMENTED |
| L5-2-1-11 | PEV concurrent session isolation | PEV | `internal/layers/contextengine/pev_engine_test.go` | IMPLEMENTED |
| L5-2-1-12 | PEV context cancellation cleanup | PEV | `internal/layers/contextengine/pev_engine_test.go` | IMPLEMENTED |
| L5-2-1-13 | PEV emit complete 透传 ctx_pct + llm_called（主路径/milestone-only 区分） | PEV | `internal/layers/contextengine/pev_engine.go` | IMPLEMENTED | P1 |
| L5-2-1-14 | query loop runSpan 含 pev.prompt_tokens / pev.completion_tokens / pev.ctx_pct | PEV | `internal/layers/contextengine/query_loop_run.go` | IMPLEMENTED | P1 |

### D2-S4: Token Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-4-01 | Token 计数共享约定与 Gateway 对齐 | Token | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |

### D2-S8: Sandbox Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-8-01 | bash 困居工作目录 + 命令白名单 | Sandbox | `internal/layers/contextengine/sandbox_test.go` | IMPLEMENTED |

### D2-S9: Harness Module

> **Change:** `devrix-harness-bootstrap` (DM-20260609-004)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-2-9-01 | harness.enabled 首次 Process 触发 Bootstrap | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| L5-2-9-02 | WorkspaceContext 扫描 Go 文件与 AGENTS.md | Harness | `internal/layers/contextengine/harness/workspace_test.go` | IMPLEMENTED | P1 |
| L5-2-9-03 | Bootstrap 幂等（同 Session 不重复） | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P1 |
| L5-2-9-04 | trusted=false 时 deferred_init 标志全 false | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| L5-2-9-05 | ToolPool simple_mode / MCP / deny 过滤 | Harness | `internal/layers/contextengine/harness/toolpool_test.go` | IMPLEMENTED | P0 |
| L5-2-9-06 | PromptRouter advisory 关键词计分 | Harness | `internal/layers/contextengine/harness/router_test.go` | IMPLEMENTED | P2 |
| L5-2-9-07 | Transcript 内存分离与 compact | Harness | `internal/layers/contextengine/harness/transcript_test.go` | IMPLEMENTED | P1 |
| L5-2-9-08 | harness.enabled=false V4 回归 + 无 bootstrap info | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| L5-2-9-09 | Preflight warn-only 规则评分与 tool filter | Harness | `internal/layers/contextengine/harness/preflight_test.go` | IMPLEMENTED | P1 |
| L5-2-9-10 | System Prompt Assembly §十 XML 块 | Harness | `internal/layers/contextengine/harness/system_prompt_assembler_test.go` | IMPLEMENTED | P0 |
| L5-2-9-11 | Jaeger span 树（enabled/disabled） | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |
| L5-2-9-12 | disabled 与 BuildLegacy 字节级一致 | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| L5-2-9-13 | CompressedView system = Build 输出 | Harness | `tests/integration/context_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| L5-2-9-14 | PEV 可见工具 ⊆ VisibleTools | Harness | `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | IMPLEMENTED | P0 |
| L5-2-9-15 | bootstrap.stage parent = bootstrap.run | Harness | `tests/integration/context_harness_obs_test.go` | IMPLEMENTED | P0 |

### D2-S9.BG: Background SubQuery Task Tools

> **Change:** `devrix-background-task-tools` (DM-20260611-009)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-2-9-16 | stop running task → cancelled (idempotent) | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| L5-2-9-17 | output block=false 返回 running 状态 + partial result | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| L5-2-9-18 | output block=true 阻塞至 terminal 或 timeout（max 600s） | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` + `background_task_tools_test.go` | IMPLEMENTED | P0 |
| L5-2-9-19 | cancel 后 SessionQueue 不发 completed notification（tombstone 协议） | BGTask | `internal/layers/contextengine/query/background_cancel_test.go` | IMPLEMENTED | P1 |

### D2-S10: QueryLoop Module

> **Change:** `devrix-queryloop-context` (DM-20260610-012)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-CTX-34 | 多轮 tool_use 直至无 tool | QueryLoop | `internal/layers/contextengine/query/loop_test.go` | IMPLEMENTED | P0 |
| L5-CTX-35 | UserContext prepend 不在 snapshot | QueryLoop | `internal/layers/contextengine/usercontext/` | IMPLEMENTED | P0 |
| L5-CTX-36 | plan_mode attachment full/sparse throttle | QueryLoop | `internal/layers/contextengine/attachments/registry.go` | IMPLEMENTED | P0 |
| L5-CTX-37 | plan mode 拒绝 Write 非 plan 文件 | QueryLoop | `internal/layers/contextengine/permission/mode_test.go` | IMPLEMENTED | P0 |
| L5-CTX-38 | task_create 磁盘持久 + list 一致 | QueryLoop | `internal/layers/contextengine/tasks/disk_store_test.go` | IMPLEMENTED | P0 |
| L5-CTX-39 | query_loop.enabled=false V4 回归 | QueryLoop | `tests/integration/query_loop_integration_test.go` | IMPLEMENTED | P0 |
| L5-CTX-40 | SubQuery Explore omitClaudeMd + read-only | QueryLoop | `internal/layers/contextengine/query/subquery_test.go` | IMPLEMENTED | P1 |
| L5-CTX-41 | Fork subagent placeholder tool_results 一致 | QueryLoop | `internal/layers/contextengine/query/fork_test.go` | IMPLEMENTED | P1 |
| L5-CTX-42 | sidechain transcript resume 重建 messages | QueryLoop | `internal/layers/contextengine/transcript/sidechain_test.go` | IMPLEMENTED | P1 |

### D2-S11: Harness Unification (Legacy 退役)

> **Change:** `devrix-harness-unification` (DM-20260611-004)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-2-9-01 | `query_loop.enabled` 默认 true (L5 命名沿用 §D2-S9 编号区间以避免重复) | HarnessUnification | `internal/shared/config/queryloop_test.go` | IMPLEMENTED | P0 |
| L5-2-9-02 | harnessEnabled 分支不再被生产路径触发 (D5 runtime.path_resolved_total{path=query_loop}++) | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| L5-2-9-03 | 旧路径调用计数基线=0 (100 次 Process 循环后 LegacyHarness=0) | HarnessUnification | `internal/layers/contextengine/path_regression_integration_test.go` | IMPLEMENTED | P0 |
| L5-2-9-04 | 压缩入口统一：QueryLoop 走 messages-only 七步管道 (`WithSkipAssembly=true`) | HarnessUnification | `internal/layers/contextengine/compression_unified_test.go` | IMPLEMENTED | P1 |
| L5-2-9-TD01 | TD-QL-01: 413 → 一轮 messages-only 压缩 → 重试 LLM.Call | HarnessUnification | `internal/layers/contextengine/query/loop_recovery_test.go` | IMPLEMENTED | P1 |
| L5-2-9-TD03 | TD-QL-03: overload/5xx → 切换 fallback model | HarnessUnification | `internal/layers/contextengine/query/loop_fallback_test.go` | IMPLEMENTED | P1 |
| L5-2-9-D6PR | D6 PathRegressionProbe: legacy_harness > 0 ⇒ score 0 (CI 阻断) | HarnessUnification | `internal/layers/evolution/eval/path_regression_probe_test.go` | IMPLEMENTED | P0 |

### D2-S12: Worktree Module

> **Change:** `devrix-queryloop-context` v2.0 (DM-20260610-012)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-4-12-01 | Worktree enter 后 write 不污染主 WorkDir | Worktree | `internal/layers/contextengine/worktree/manager_test.go` | IMPLEMENTED | P0 |

### D2: Cross-Scenario Tests

| L5 ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| L5-2-0-01 | 压缩/Verify 步骤可观事务 | `tests/integration/context_compression_obs_test.go` | IMPLEMENTED |
| L5-2-0-02 | 主路径接入真实 LLM Gateway | `tests/integration/context_llm_gateway_test.go` | IMPLEMENTED |
| L5-2-0-03 | plan.enabled=false 时回退 V2 路径 | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |

---

## ORCH: Orchestration Layer (Cross-Domain)

> **Change:** `devrix-queryloop-context` v2.0 (DM-20260610-012) · Package: `internal/layers/orchestration/`

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-ORCH-01 | WorkPlan.Snapshot 含 Task + ExecutionFlow | WorkPlan | `internal/layers/orchestration/workplan/service_test.go` | IMPLEMENTED | P0 |

### ORCH-S2: Wave Scheduler Module

> **Change:** `devrix-wave-scheduler` (DM-20260611-007)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-ORCH-10 | DAG 6 ready subagent + 1 cursor 持续调度峰值并发=5 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-11 | upstream policy 收到 A artifact，无 Leader 全量 | ContextPolicy | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-12 | fresh policy SubAgent 启动 Messages 仅含 directive | ContextPolicy | `internal/layers/orchestration/wave/context_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-13 | 同 conflict_group Task 不并行 | ConflictGuard | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-14 | 每 Task 独立双区块 IM 卡流式 | WorkerCard | `internal/layers/communication/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-15 | 槽位释放后 ready Task 立即派发 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-16 | cursor + claude-code 并行 file_scope 不交 | ConflictGuard | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P1 |
| L5-ORCH-17 | Plan 产出 DAG 仅 ready 节点被派发 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-18 | wave 全完成 Leader 收到 wave_completed 汇总 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_l5_test.go` | IMPLEMENTED | P1 |
| L5-ORCH-19 | CancelWorker 槽位释放 status=cancelled | WorkerLifecycle | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-20 | CancelAll 5 running 全部 terminal pool 全释放 | WorkerLifecycle | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| L5-ORCH-21 | CLI Worker cancel 进程终止 IM 卡 cancelled | WorkerLifecycle | `internal/layers/orchestration/wave/runners/agent_tool_l5_21_test.go` | PARTIAL | P1 |

---

## D3: LLM Gateway Domain (LLM)

> **Domain tests:** `./scripts/test-domain.sh d3` · **Live:** `--live` · **Spec:** `domain-segmentation.md`

### D3-S1: Adapter Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-1-01 | DeepSeek 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/deepseek_test.go` | IMPLEMENTED |
| L5-3-1-02 | MiniMax 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/minimax_test.go` | IMPLEMENTED |
| L5-3-1-03 | SSE parse error handling | Adapter | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

### D3-S3: Breaker Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-3-01 | Circuit breaker 正常关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-02 | Circuit breaker 触发放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-03 | Circuit breaker 半开→关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-04 | Circuit breaker 半开→放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-05 | 熔断器状态长久化 | Breaker | - | PLANNED |

### D3-S5: Token Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-5-01 | Token 计数准确性 (cl100k_base) | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| L5-3-5-02 | Token 预算检查 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| L5-3-5-03 | Token counter 中文准确性 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |

### D3-S6: Config Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-6-01 | Provider 配置加载 | Config | `internal/layers/llmgateway/config/loader_test.go` | IMPLEMENTED |

### D3-S4: Retry Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-4-01 | 重试策略执行 | Retry | `internal/layers/llmgateway/retry/retry_test.go` | IMPLEMENTED |
| L5-3-4-02 | DeepSeek Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| L5-3-4-03 | MiniMax Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |

### D3-S2: Gateway Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-2-01 | LLM 调用可观测事件 | Gateway | `tests/integration/llm_observer_test.go` | IMPLEMENTED |
| L5-3-2-02 | 未知 Provider/Model 报错 | Gateway | `internal/layers/llmgateway/gateway/router_test.go` | IMPLEMENTED |
| L5-3-2-03 | 多 Provider 并发调用 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| L5-3-2-04 | Retry 与 CB 联动，context 取消不触发 CB | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| L5-3-2-05 | Half-Open 并发探测上游 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| L5-3-2-06 | LLM 429 rate limit handling | Gateway | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

---

## D4: Multi-Agent Domain (AGENT)

> **Domain tests:** `./scripts/test-domain.sh d4` · **Spec:** `domain-segmentation.md`

> 已归档：`openspec/archive/2026-06-08-devrix-multi-agent/`（Demand: DM-20260608-005）

### D4-S1: Factory Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-1-01 | AgentFactory 创建 Agent 实例 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |

### D4-S2: Agent Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-2-01 | Agent 生命周期状态转换 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-2-02 | AgentPermissionGate 批准/拒绝/超时 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |
| L5-4-2-03 | CRITICAL 工具权限异步流程 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |

### D4-S3: ForkJoin Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-3-01 | Fork/Join 消息隔离模型 | ForkJoin | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-3-02 | Fork 双层限额 MaxChildren+MaxTotalAgents | ForkJoin | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |
| L5-4-3-03 | Agent 超时自动终止 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-3-04 | Context 取消传播到子 Agent | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |

### D4-S4: Collaboration Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-4-01 | CoT prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |
| L5-4-4-02 | Iterative-Refinement prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |

### D4-S5: Observer Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-5-01 | ObserverAdapter 桥接 AgentEvent → IObserver | Observer | `internal/layers/multiagent/observer/adapter.go` | IMPLEMENTED |

### D4-S6: Agent Tool Module

> **Change:** `devrix-agent-tools` (DM-20260608-012)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-4-6-01 | Agent Tool Registry 注册/查找/按能力查询 | AgentTool | `internal/layers/multiagent/tool/registry_test.go` | IMPLEMENTED | P0 |
| L5-4-6-02 | CLI 适配器正常启动子进程并解析 stream-json | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |
| L5-4-6-03 | CLI 适配器超时正确终止子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| L5-4-6-04 | Session 首次创建子进程，后续调用复用的同一进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |
| L5-4-6-05 | Session 空闲超时自动回收子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| L5-4-6-06 | D1 Session 销毁清理关联的 Agent Tool 子进程 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P1 |
| L5-4-6-07 | 不同 D1 Session 的 Agent Tool 隔离运行互不干扰 | AgentTool | `internal/layers/multiagent/tool/cli_adapter_test.go` | IMPLEMENTED | P0 |

### D4-S10: Delegate Module

> **Change:** `devrix-queryloop-context` v2.0 (DM-20260610-012)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-4-10-01 | Leader delegate_explore 创建 Worker / MaxWorkers | Delegate | `internal/layers/multiagent/delegate/service_test.go` | IMPLEMENTED | P0 |
| L5-4-10-02 | Worker Run 设置 AgentID，sidechain 隔离 | Delegate | `internal/layers/contextengine/worker_tools_test.go` | IMPLEMENTED | P0 |
| L5-4-10-03 | Worker 不能 delegate_* 或 Fork | Delegate | `internal/layers/multiagent/agent/worker_engine_test.go` | IMPLEMENTED | P0 |
| L5-4-10-04 | delegate-progress 仅 Leader Drain | Delegate | `internal/layers/contextengine/queue/delegate_progress_test.go` | IMPLEMENTED | P0 |
| L5-4-10-05 | worker_progress 到达 Gateway/IM | Delegate | `internal/layers/orchestration/imsink/gateway_test.go` | IMPLEMENTED | P0 |
| L5-4-10-06 | SubQuery 与 D4 Worker 共用 FlowEvent schema | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| L5-4-10-07 | FlowStarted 自动 task owner + in_progress | Delegate | `internal/layers/orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| L5-4-10-08 | D4 未启用 delegate 降级 SubQuery | Delegate | `internal/layers/contextengine/delegate_fallback_flow_test.go` | IMPLEMENTED | P0 |
| L5-4-10-09 | 用户单会话：无第二对话入口 | Delegate | `internal/bootstrap/cli_events_test.go` | IMPLEMENTED | P0 |

### D4: Cross-Scenario Tests

| L5 ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| L5-4-0-01 | Agent 并发安全 (-race) | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-0-02 | Fork 消息隔离并发安全 | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-0-03 | Gateway → ResolvePermission 集成全流程 | `tests/integration/agent_integration_test.go` | IMPLEMENTED |
| L5-4-0-04 | E2E Fork 端到端 | `tests/e2e/agent_fork_e2e_test.go` | IMPLEMENTED |

---

## D5: Observability Domain (OBS)

### D5-S2: Metrics Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-2-01 | Tracing Span 创建与传播 | Metrics | - | PLANNED |
| L5-5-2-02 | Metrics Counter 计数 | Metrics | - | PLANNED |
| L5-5-2-03 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | Metrics | `internal/layers/observability/metrics/gauge_test.go` | IMPLEMENTED |
| L5-5-2-04 | Histogram Prometheus 输出与 golden 一致 | Metrics | `internal/layers/observability/metrics/histogram_test.go` | IMPLEMENTED |
| L5-5-2-05 | Int64UpDownCounter 返回 Gauge | Metrics | `internal/layers/observability/metrics/meter_test.go` | IMPLEMENTED |
| L5-5-2-06 | Compression P99 latency < 500ms | Metrics | `tests/performance/compression_test.go` | IMPLEMENTED |
| L5-5-2-07 | Concurrent session memory bounded | Metrics | `tests/performance/memory_test.go` | IMPLEMENTED |

### D5-S3: Logger Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-3-01 | 日志级别过滤 | Logger | - | PLANNED |
| L5-5-3-02 | Shutdown 覆盖 Tracer + Logger | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| L5-5-3-03 | Error 日志包含 stacktrace 字段 | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| L5-5-3-04 | 日志采样 max_entries_per_span 生效 | Logger | `internal/layers/observability/logger/sampling_test.go` | IMPLEMENTED |

### D5-S1: Tracer Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-1-01 | Shutdown 刷写所有 pending spans | Tracer | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED |
| L5-5-1-02 | ConsoleExporter 可直接作为 SpanExporter | Tracer | `internal/layers/observability/exporter/console_test.go` | IMPLEMENTED |

### D5-S4: Exporter Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-4-01 | LongTerm recall/store 产生 canonical Operation span | Exporter | `internal/layers/contextengine/engine.go` | IMPLEMENTED |
| L5-5-4-02 | Plan 产生 Milestone Run 产生 canonical Operation span | Exporter | `internal/layers/contextengine/pev_engine.go` | IMPLEMENTED |
| L5-5-4-03 | Feishu 入站产生 adapter.message.receive span | Exporter | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED |

### D5-S5: Coverage Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-5-01 | Operation Registry 与 names.go 常驻全集一致 | Coverage | `internal/layers/observability/coverage/registry_test.go` | IMPLEMENTED |
| L5-5-5-02 | Coverage 报告正确列出 zero_hit operations | Coverage | `tests/integration/obs_coverage_test.go`, `tests/integration/context_harness_obs_test.go` | IMPLEMENTED |

### D5: Cross-Scenario Tests

| L5 ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| L5-5-0-01 | Gateway 会话 | - | PLANNED |

---

## D6: Evolution Domain (EVO)

> **Spec Reference:** `openspec/specs/eval/spec.md` (D6-S3); D6-S1/S2 见 `openspec/archive/2026-06-08-devrix-d1-d6-testing/demand.md`

### D6-S1: Version Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-6-1-01 | 版本检测与记录（PlannedVersion: v2.1.0） | Version | `internal/layers/evolution/version/version_test.go` | PLANNED | P2 |

### D6-S2: Config Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-6-2-01 | 配置热更新（PlannedVersion: v2.2.0） | Config | `internal/layers/evolution/config/hotreload_test.go` | PLANNED | P2 |

### D6-S3: Eval Module (Pilot)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-6-3-01 | EvalRun 编排 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED | P0 |
| L5-6-3-02 | LLM-as-Judge 校准与分歧 | Eval | `internal/layers/evolution/eval/judge_test.go` | IMPLEMENTED | P0 |
| L5-6-3-03 | Compression Recall Probe F1 | Eval | `internal/layers/evolution/eval/compression_recall_probe_test.go` | IMPLEMENTED | P0 |
| L5-6-3-04 | Delta 报告对比 | Eval | `internal/layers/evolution/eval/delta_test.go` | IMPLEMENTED | P0 |
| L5-6-3-07 | eval.enabled=false 零行为 | Eval | `internal/layers/evolution/eval/engine_test.go` | IMPLEMENTED | P0 |
| L5-6-3-06 | PEV Tool 选择准确率探针 | Eval | `internal/layers/evolution/eval/pev_tool_accuracy_probe_test.go` | IMPLEMENTED | P1 |
| L5-6-3-11 | devrix eval run 子命令 | Eval | `internal/cli/eval/run_test.go` | IMPLEMENTED | P1 |
| L5-6-3-12 | 调优建议生成 | Eval | `internal/layers/evolution/eval/tune_test.go` | IMPLEMENTED | P2 |
| L5-6-3-13 | eval run 真实 Judge 接入 | Eval | `internal/cli/eval/judge.go` | IMPLEMENTED | P1 |
| L5-6-3-09 | Provider 质量对比探针 | Eval | `internal/layers/evolution/eval/provider_quality_probe_test.go` | IMPLEMENTED | P1 |
| L5-6-3-10 | Agent Fork/Join 质量探针 | Eval | `internal/layers/evolution/eval/agent_forkjoin_probe_test.go` | IMPLEMENTED | P2 |
| L5-6-3-14 | Eval CI delta gate | Eval | `internal/layers/evolution/eval/gate_test.go` | IMPLEMENTED | P2 |
| L5-6-3-15 | run-eval.sh CI 脚本 | Eval | `scripts/eval/run-eval.sh` | IMPLEMENTED | P2 |

---

## D0: Code Integrity Domain (INTEGRITY)

> 跨域代码健康规范。属架构治理层，非业务域。
> **Spec Reference:** `openspec/archive/2026-06-08-devrix-code-integrity/demand.md`

### D0-S1: Specification Module

| L5 ID | 描述 | Test 位置 | Status | Priority |
|-------|------|-----------|--------|----------|
| L5-0-1-01 | `coding.md §9` 包含不可变性分层规范 | `openspec/specs/project/coding.md` §9 | IMPLEMENTED | P0 |
| L5-0-1-02 | CLAUDE.md 引用新规范 | `CLAUDE.md` | IMPLEMENTED | P0 |
| L5-0-1-03 | emitEvent 处理 EventConnectionLostData 不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| L5-0-1-04 | emitEvent 处理 EventConnectionRestoredData 不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| L5-0-1-05 | emitEvent 处理未知类型不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| L5-0-1-06 | 新会话创建被拒绝 | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| L5-0-1-07 | /new /help /stop 命令解析正确 | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| L5-0-1-08 | 飞书消息解析正确 | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| L5-0-1-09 | D6 L5 注册表条目包含目标版本 | `openspec/l5-registry.md` | IMPLEMENTED | P1 |

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

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Complete rewrite with D-S numbering system |
| 1.1.0 | 2026-06-08 | Section headers migrated L1-L2 → D-S (DM-20260608-007) |
| 1.2.0 | 2026-06-08 | D1/D6 testing specs added; Priority column added (devrix-d1-d6-testing) |
| 1.3.0 | 2026-06-08 | D1 L5 IMPLEMENTED; registry summary reconciled (DM-20260608-011) |
| 1.4.0 | 2026-06-09 | D2/D3/D4 domain build tags + test-domain.sh (DM-20260609-001) |
| 1.5.0 | 2026-06-10 | QueryLoop v1/v2 + Delegate + WorkPlan L5 (DM-20260610-012) |

---

## D5: Observability Domain (OBS)

> **Spec Reference:** `docs/observability-design.md`

### D5-S1: Tracing Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-1-01 | LLM 请求日志完整记录 | OBS-TRACE | `internal/layers/observability/llm_logger_test.go` | PLANNED |
| L5-5-1-02 | LLM 响应日志完整记录 | OBS-TRACE | `internal/layers/observability/llm_logger_test.go` | PLANNED |
| L5-5-1-03 | PEV 迭代独立 Span | OBS-TRACE | `internal/layers/contextengine/pev_engine_test.go` | PLANNED |
| L5-5-1-04 | Baggage 传递业务上下文 | OBS-TRACE | `internal/layers/observability/tracer/baggage_test.go` | IMPLEMENTED |

### D5-S2: Metrics Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-2-01 | 工具延迟 Histogram 注册 | OBS-METRICS | `internal/layers/observability/metrics/tool_latency_test.go` | PLANNED |
| L5-5-2-02 | 压缩率 Histogram 注册 | OBS-METRICS | `internal/layers/observability/metrics/compression_test.go` | PLANNED |
| L5-5-2-03 | 活跃会话 Gauge 注册 | OBS-METRICS | `internal/layers/observability/metrics/session_test.go` | PLANNED |

### D5-S3: Coverage Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-3-01 | Span 创建触发 RecordHit | OBS-COVERAGE | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED |
| L5-5-3-02 | 未知 Operation 记录 Unknown | OBS-COVERAGE | `internal/layers/observability/coverage/coverage_test.go` | IMPLEMENTED |
| L5-5-3-03 | 每日报表持久化 | OBS-COVERAGE | `internal/layers/observability/coverage/persistence_test.go` | IMPLEMENTED |
| L5-5-3-04 | Coverage 覆盖率 ≥80% | OBS-COVERAGE | `internal/layers/observability/coverage/coverage_test.go` | PLANNED |

### D5-S4: Integration Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-4-01 | Context → LLM → Tool 链路可追踪 | OBS-TRACE | `tests/integration/full_chain_trace_test.go` | PLANNED |
| L5-5-4-02 | Metrics 与 Trace 关联 | OBS-METRICS | `tests/integration/observability_test.go` | PLANNED |
| L5-5-4-03 | HealthCheck 包含 Coverage 统计 | OBS-COVERAGE | `internal/layers/observability/observability_test.go` | IMPLEMENTED |

### D5-S6: AI Debug Readiness (DM-20260610-001 P0)

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-OBS-TRACE-04 | Span 父子层级契约 (R1-R2) | OBS-TRACE | `tests/integration/obs_pev_span_hierarchy_test.go` | IMPLEMENTED |
| L5-OBS-TRACE-05 | Log-Trace-LLM 关联 | OBS-TRACE | `logger/slog_bridge_test.go`, `llm_log_test.go` | IMPLEMENTED |
| L5-OBS-GENAI-ATTR | gen_ai.* 语义属性双写 | OBS-TRACE | `obs_pev_span_hierarchy_test.go`, `telemetry/names_test.go` | IMPLEMENTED |
| L5-OBS-DECISION-01 | verify.failure_reason | OBS-TRACE | `pev_engine` integration (partial) | IMPLEMENTED |
| L5-OBS-METRICS-01 | tool_latency histogram | OBS-METRICS | `bridge_tool_latency_test.go`, `pev_engine.go` | IMPLEMENTED |
| L5-OBS-METRICS-02 | compression_ratio histogram | OBS-METRICS | `engine.go`, `types/context_test.go` | IMPLEMENTED |
| L5-OBS-DECISION-02 | compression.trigger_reason + ratio | OBS-TRACE | `engine.go` compression span | IMPLEMENTED |
| L5-OBS-EXPORT-01 | Session incident export | OBS-EXPORT | `incident/export_test.go`, `cmd/debug-export` | IMPLEMENTED |
| L5-OBS-TRACE-06 | SpanKind 契约 (SERVER/CLIENT/INTERNAL) | OBS-TRACE | `obs_pev_span_hierarchy_test.go` | IMPLEMENTED |
| L5-OBS-DECISION-03 | gen_ai.prompt.version + template_hash | OBS-TRACE | `system_prompt_assembler_test.go` | IMPLEMENTED |
| L5-OBS-METRICS-03 | gen_ai.client.token.usage Counter | OBS-METRICS | `genai_tokens_test.go` | IMPLEMENTED |
| L5-OBS-EXPORT-02 | devrix debug export 子命令 | OBS-EXPORT | `internal/cli/debug/export_test.go` | IMPLEMENTED |
| L5-OBS-TRACE-03 | W3C Baggage 传播 (session.id/user.id) | OBS-TRACE | `tracer/baggage_test.go`, `propagation_test.go` | IMPLEMENTED |
| L5-OBS-METRICS-04 | cache_read/reasoning token breakdown | OBS-METRICS | `genai_tokens_test.go`, `sse_parser_test.go` | IMPLEMENTED |
