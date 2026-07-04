---
demand-id: DM-20260704-003
title: "Runtime 反馈链路闭环 — i18n 中文硬规则、tracing parent-span 连续性、tool 调用可超时"
priority: P0
status: S1_Proposal
dsaft_domain: context-engine, observability, orchestration
created: 2026-07-04
related:
  - openspec/archive/2026-07-04-mups-d2-context-tools-ownership/  # DM-20260704-002 同 D2 域收口
  - openspec/specs/d2-context-engine/i18n/prompt_sections_zh.go   # 修复目标
  - internal/layers/contextengine/prepare/compression/tracing_step_observer.go  # ctx 透传可疑点
  - internal/bootstrap/turn_adapter.go::executeOne                # tool timeout 缺位
---

# Runtime 反馈链路闭环

## 1. 背景

DM-20260704-002（mups-d2-context-tools-ownership）刚完成 S6 归档，D2 上下文化路径已经统一到 `MaterializeForMUPS`。但 D2 链路在 **生产运行时**暴露三个独立但同源的反馈回路缺陷：

1. **i18n 反馈断点** — D2 contextengine 切换到 `zh-CN` 时，LLM 收到的 system prompt 不含"请用中文回复"硬规则；只要 tool 错误消息、文件路径、或英文 `<system-reminder>` 混入，模型会自然切到英文。
2. **Tracing 父链断点** — D5 链路里 `tracingStepObserver.OnStep` 把 `startSpan` 返回的新 ctx 用 `_` 丢弃，compression pipeline 上的子 span 失去 `parent_span_id`；Jaeger 上累计 200+ orphan spans。
3. **Tool 执行无 timeout** — `bootstrap/turn_adapter.go::executeOne`（line 446）只过 `perm.Request` 权限 gate，缺 tool-level timeout；一个慢 tool（例如 shell 子进程挂起）会阻塞整个 Turn 主循环，直到 feishu channel 用户侧发来 `EscapeAbort`。

三个问题都跨 D2 / D5 / D7 三个域。共同主题是"运行时反馈链路未闭环"：上游信号（locale、ctx、tool budget）到下游 LLM / observability / orchestrator 之间的 transmission gap。本 change 一次性把 3 处断点封堵，不引入新架构。

## 2. 问题陈述

### P1 — i18n 中英混杂
- 现象：用户系统默认 `zh-CN`，但 devrix 回复中英混杂。
- 根因（已定位）：`internal/layers/contextengine/i18n/prompt_sections_zh.go` 的 `intro`/`system`/`tone_and_style` 段均无"请始终用中文回复用户"硬规则。`prompt_sections_en.go` 也没有对称的反向指令（英文 locale 不会被中文污染，目前安全）。
- 触发条件：LLM 收到中文 system prompt，但 tool 错误、文件路径、`<system-reminder>` 标签等英文 inline 内容一旦混入，模型自然切到英文。
- 影响：用户体验差，与产品"中文优先"承诺不符。

### P2 — Jaeger 200 missing parent
- 现象：Jaeger trace `3579c3fdc47efd428353b0339e76fadb` 报"200 spans have missing parent spans"。
- 根因（高概率定位）：`internal/layers/contextengine/prepare/compression/tracing_step_observer.go:28` 在 `OnStep` 调 `o.startSpan(ctx, ...)`，**返回的新 ctx 没用**（`_, span :=`）。`tracingStepObserver` 内部 startSpan 的 fallback 路径（`tracer.go:128`）在 ctx 缺 sc 时会生成新 TraceID，导致 compression 子 span 与 prepare 主 span 失去父子链。
- 次要可疑点：
  - `context.go:71-76` `Detach(ctx)` 在父 sc 不存在时返回 `context.Background()`，可能影响 Worker fork 边界。
  - 跨 goroutine 时若某处用 `context.Background()` 替代传入 ctx，父子就断链。
- 影响：AI 排障与 trace 关联失效；Jaeger 200+ orphan spans 干扰关键 trace 搜索。

### P3 — Tool 调用无 timeout（无运行时复现）
- 现象：用户报告"工具 #54 调用后没动作"（前 session 反馈），feishu 端无响应。
- 现状：仅静态分析 — `internal/bootstrap/turn_adapter.go::executeOne`（line 446）无 `context.WithTimeout` 包裹；工具执行走 `partitionToolCalls` + `errgroup` 并发，但每个 tool call 自身无 deadline。
- 可能根因（待 runtime 复现）：
  - 子进程挂起（无 timeout 永远等）
  - `EscapePendingHuman` 触发后工具在等用户输入（死锁）
  - `MaxMUPSRoundsExceeded` 路径与 d7-convergence-contract 交互
- 影响：单 tool 卡死 → 整个 Turn 卡死 → feishu 卡片 never complete。**沙箱内无法复现**（无 devrix 实例、ps 命令被禁）。
- 处理：本次只加 timeout 防御（fail-closed 兜底）；**完整 root cause 确认需 runtime log + 进程 trace，列入 OUT-OF-SCOPE**。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `prompt_sections_zh.go` 在 zh-CN locale 下含"请始终用中文回复用户"硬规则；`prompt_sections_en.go` 不含（避免英文污染） | P0 |
| AC2 | i18n golden test：zh/en prompt bytes 稳定（无 LLM 抖动） | P0 |
| AC3 | `tracingStepObserver.OnStep` 透传 ctx，新 ctx 用于子 span；`go test -race` 0 race | P0 |
| AC4 | Worker fork 边界 parent_span_id 100% 命中（mock trace 测试，3 case 覆盖 trace fork / scheduler dispatch / child_downlink） | P0 |
| AC5 | `turn_adapter.executeOne` 工具调用默认 60s timeout（env: `DEVRIX_TOOL_TIMEOUT_SECONDS` 可调），超时后 cancel ctx + 走 fail-closed 路径（EmitChannelRoute warn span） | P0 |
| AC6 | orphan span 标记：tracer.Start 失败 fallback 路径加 `slog.Warn("orphan span")` + span attribute `span.orphan=true`，Jaeger 可筛 | P1 |
| AC7 | 全量 `go test -race -count=1 ./internal/...` PASS，0 race detector warnings | P0 |
| AC8 | 22/22 orchestration + 22/22 contextengine + d5 packages `go vet` 0 issue | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | DM-20260704-002 已归档的 `MaterializeForMUPS` 路径（D2 7 步 filter pipeline） |
| 依赖 | DM-20260630-013 `d2-d7-review-hardening`（D2/D7 hardening P0/P1 收口） |
| 依赖 | DM-20260703-001 `d7-convergence-contract`（CC-1～CC-5 收敛契约） |
| 约束 | **不破坏** 现有 174 个归档的 acceptance criteria（zero regression） |
| 约束 | **不引入** 新依赖（仅用 stdlib + 现有 project 库） |
| 约束 | **不修改** D2↔D3 import lint（DM-020 单独 change 管辖） |
| 约束 | **沙箱限制**：`ps` 命令被禁，无法在沙箱内复现 tool #54 卡住；runtime 验证交由用户在 production 跑 |

## 5. 变更范围

### 新增
- `internal/layers/contextengine/i18n/prompt_sections_zh.go` — `intro` 段追加中文硬规则
- `internal/layers/contextengine/i18n/prompt_sections_en_test.go` — golden test
- `internal/layers/contextengine/i18n/prompt_sections_zh_test.go` — golden test
- `internal/layers/observability/instrument/tracer/orphan_marker.go` — orphan span 标记
- `internal/layers/observability/instrument/tracer/orphan_marker_test.go`
- `internal/layers/orchestration/sessionorchestrator/turn_loop/orphan_span_test.go` — 端到端验证
- `internal/bootstrap/turn_adapter_timeout.go` — tool-level timeout 封装
- `internal/bootstrap/turn_adapter_timeout_test.go`
- `openspec/specs/d2-context-engine/runtime-feedback-closure.md` — spec 增量
- `openspec/specs/d5-observability/parent-span-continuity.md` — spec 增量
- `openspec/specs/d7-orchestration/tool-call-timeout.md` — spec 增量

### 修改
- `internal/layers/contextengine/i18n/prompt_sections_zh.go` — `intro` / `system` / `tone_and_style` 段中文硬规则
- `internal/layers/contextengine/prepare/compression/tracing_step_observer.go` — 透传 ctx
- `internal/bootstrap/turn_adapter.go` — `executeOne` 加 timeout 包裹
- `internal/layers/observability/instrument/tracer/tracer.go` — Start 失败 fallback 加 orphan marker
- `internal/shared/config/user.go` — 新增 `ToolTimeoutSeconds` 字段（默认 60）
- `openspec/t-registry.md` — 新增 8 个 P0 T 点
- `openspec/specs/d2-context-engine/t-registry.md` — D2-S15-A82-T01..T03
- `openspec/specs/d5-observability/t-registry.md` — D5-S2-A01-T01..T02
- `openspec/specs/d7-orchestration/t-registry.md` — D7-S2-A50-T09..T10
- `devrix.yaml` — `tool_timeout_seconds: 60` 默认值

### 不变更
- D2↔D3 import lint（DM-020 不动）
- D2↔D7 边界（d7-boundary.md v2.1 不动）
- 现有 174 个归档的 acceptance criteria
- `MUPSPreparedContext` / `MaterializeForMUPS` API（DM-20260704-002 已定）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 中文硬规则在英文 locale 误启用 | 英文用户被中文污染 | 严格 locale gating（仅 `zh-CN` / `zh-Hans` / `zh` 触发）；EN locale prompt 字节测试 |
| tracing ctx 透传引入 race | 子 span 数据竞争 | `go test -race` 全量；tracingStepObserver 改为 value receiver（已是） |
| 60s timeout 太短打断正常 tool（如 long build） | 用户体验差 | env `DEVRIX_TOOL_TIMEOUT_SECONDS` 可调；per-tool override 留作 v1.1 |
| orphan marker 增加 span 大小 | OTLP 出口负载 | 仅在 fallback 路径标记；正常路径 0 开销 |
| sandbox 无法复现 tool #54 | 修复未验证 root cause | runtime 验证交用户；fail-closed 防御性兜底 |

## 7. OUT-OF-SCOPE

1. **完整 tool #54 卡住 root cause 复现** — 需要 production devrix 实例 + 进程 trace + feishu 端 log。沙箱（`ps` 禁、不能跑 devrix）无法复现。修复仅加 timeout 防御（AC5），不验证 root cause。
2. **per-tool timeout override**（如 `Bash` 300s / `Read` 10s）— v1.1 follow-up。
3. **D2↔D3 import lint 增强**（DM-020 单独 change 管辖）。
4. **D7 verify-promotion 物理迁移 PLANNED 收口**（D7-S4-A50 T01-T03）— 独立 follow-up change。
5. **D7 S15 PARTIAL rollup E2E / trace replay stub**（D7-S15-IT01/IT02）— 独立 follow-up change。
6. **完整 streaming fallback 自动切换**（P0-2，DM-20260628-001 已 defer）— 独立 follow-up。
7. **LLM 输出 token-level i18n 检测**（运行时 LLM 切语言时自动告警）— 探索性，留作 tech-debt。
