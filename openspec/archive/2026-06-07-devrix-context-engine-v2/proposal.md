# Proposal: Context Engine V2

**Change ID:** devrix-context-engine-v2
**Layer:** 2 - Context Engine
**Type:** Enhancement
**Status:** S7 Archived
**Based on:** `devrix-context-engine` (V1 archived), `devrix-llm-gateway` (V1 archived 2026-06-07)
**Demand:** DM-20260607-003
**Grill Session:** 2026-06-07, 14 decisions resolved

---

## Problem Statement

V1 上下文引擎已替换 Stub，但三类能力仍为「占位或启发式」：

1. **压缩步骤 6 被跳过** — 步骤 1–5 后仍超预算时只能 Snip 删消息，语义损失大
2. **Verify 仅 basic** — 无法运行 `go test` 等命令验证工具副作用
3. **Token 计数不准** — char/4 与模型实际 tokenizer 偏差，压缩触发时机不可靠

此外 `main.go` 仍注入 Mock LLM，生产路径未接通 Layer 3。

## Proposed Solution

| 能力 | V2 方案 |
|------|---------|
| Autocompact | 步骤 1–5 后仍超限 → LLM 结构化摘要替换中间段 → 步骤 7 |
| Verify commands | 配置化白名单命令 + `IVerifyCommandRunner` + 超时沙箱 |
| Token 统一 | 注入 `llmgateway.TokenCounter`，移除本地启发式为默认 |
| 可观测 | 每压缩步骤 + Autocompact + Verify 命令独立 span/metric |

## Goals

| Goal | V1 | V2 |
|------|----|----|
| 压缩步骤 6 Autocompact | ❌ skip | ✅ LLM 摘要 |
| PEV verify_mode `commands` | ❌ | ✅ |
| Token cl100k_base | ❌ 启发式 | ✅ Gateway 统一 |
| 真实 LLM Gateway 接线 | ❌ Mock | ✅ 主路径 |
| Observability 逐步骤 | 部分 | ✅ 完整 |

## Capabilities

| Capability | L4 映射 | 说明 |
|------------|---------|------|
| autocompact | L4-CTX-COMPRESS | 压缩管道步骤 6 |
| verify-commands | L4-CTX-PEV | PEV Verify 扩展 |
| token-counter-bridge | L4-CTX-STATE | Gateway TokenCounter 适配 |
| compression-observability | L4-CTX-OBS | span/metrics 增强 |

## Alternatives Considered

| 方案 | 结论 |
|------|------|
| 不做 V2，直接 V3 Plan+Milestone | 拒绝 — 压缩与验证缺口阻塞真实开发场景 |
| Autocompact 异步后台 | 拒绝 V2 — 一致性复杂；V2 保持同步 |
| Verify 用 LLM 判断（非命令） | 拒绝 — 不可复现；commands 更确定性 |
| Token 计数留在 L2 | 拒绝 — 与 V1 决议冲突，应与 Gateway 统一 |

## Impact

| 组件 | 变更 |
|------|------|
| `compression/pipeline.go` | 步骤 6 条件执行 |
| `compression/autocompact.go` | **新增** |
| `pev/verify_commands.go` | **新增** |
| `pev/verify_runner.go` | **新增** |
| `token/counter.go` | 改为 Gateway 适配器 |
| `internal/shared/contracts/tokencounter.go` | **新增** 共享 `ITokenCounter` |
| `contracts.go` | +`ICompressionObserver`, +`IPEVObserver`, +`IVerifyCommandRunner` |
| `shared/config/contextengine.go` | +autocompact, +verify_commands |
| `cmd/devrix/main.go` | Mock → 真实 LLMGateway |
| `openspec/l5-registry.md` | +L5-CTX-12~17 |
| `openspec/specs/context-engine/spec.md` | S7 合并 delta |

## Scope

**In Scope:** 见 `demand.md` §3.2

**Out of Scope:** Plan/Milestone、LongTerm、快照加密、异步 Autocompact

## Dependencies

```
devrix-llm-gateway (V1: TokenCounter + ChatStream)
        │
        ▼
devrix-context-engine-v2
        │
        ├── devrix-observability (span/metrics，可并行)
        └── communication layer (无接口变更)
```

**阻塞关系:** M1（Token 统一）与 M2（Autocompact）依赖 LLM Gateway TokenCounter 与 ChatStream 可用。

## Success Criteria (S3 准出)

- [x] proposal / design / specs / tasks 四件套完整
- [x] L5-CTX-12 ~ L5-CTX-18 已登记 `l5-registry.md`
- [x] 每个新增 L4 至少 1 个 L5 测试点（Given-When-Then）
- [x] V1 canonical spec 的 MODIFIED 章节含完整 Requirement 全文
- [x] 开放问题 Q3~Q10 已决议（见 demand.md）
- [x] 压缩管道步骤顺序已文档化（1-4 → 6 → 5 → 7）
- [x] `ITokenCounter` 共享契约与 llm-gateway 对齐
- [x] `docs/context-engine-design.md` 附录 B/C 已同步 V2 决议

## Risks

| 风险 | 缓解 |
|------|------|
| Autocompact 增加 LLM 成本与延迟 | 轻量模型（`autocompact.model`）+ summary_max_tokens 上限 + 10s 超时 |
| Autocompact 用户等待过长 | P99 < 10s 超时；超时降级等同 V1 |
| Verify 命令安全风险 | executable+args 无 shell + 白名单 + WorkDir 精确匹配 + 10s/120s 超时 |
| Verify 命令标签基数爆炸 | name regex `[a-z0-9_-]+` + ≤10 上限 + 不加 exit_code 标签 |
| LLM Gateway 未就绪 | M1 可并行；`contracts.ITokenCounter` 已就绪（L2/L3 均已实现） |
| 摘要幻觉 | 结构化 JSON + Prompt 模板 + metadata 标记 + 仅总结已有消息 |

## Timeline (估算)

| 阶段 | 工期 |
|------|------|
| S3 规划（本 PR） | 1d |
| S4 实现 | 7–9d |
| S5 验收 | 2d |

---

## Archive Information

**Archived:** 2026-06-07
**Duration:** 1 day (2026-06-07)
**Outcome:** Successfully implemented (V2)

### Files Modified
- `internal/layers/contextengine/compression/` — autocompact, pipeline step 6, observers
- `internal/layers/contextengine/` — token counter, verify runner, PEV verify commands
- `internal/bridges/llm/` — WireContextLLM + TokenCounter injection
- `internal/shared/config/contextengine_v2.go`, `shared/errors/context.go`
- `cmd/devrix/main.go`, `cmd/devrix-feishu/main.go` — real LLM Gateway wiring
- `tests/integration/context_*_test.go`, `tests/acceptance/p0/ctx_autocompact_test.go`

### Specs Updated
- `openspec/specs/context-engine/spec.md` — canonical Layer 2 specification (v2.0.0)
- `openspec/specs/context_engine_layer_delta.md` — merged reference
- `openspec/l5-registry.md` — L5-CTX-12~18 IMPLEMENTED

### Acceptance
- Verdict: **ACCEPTED** (see `acceptance-report.md`)
- Demand: DM-20260607-003
