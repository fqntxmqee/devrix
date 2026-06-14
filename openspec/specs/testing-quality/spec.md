# Testing Quality Enhancement Specification

**Capability:** testing-quality
**Change ID:** devrix-testing-quality (archived 2026-06-08)
**Demand:** DM-20260608-004
**Status:** Canonical — source of truth
**Version:** 1.0.0
**Last Updated:** 2026-06-08
**Parent Capability:** testing-framework
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Archive:** `openspec/archive/2026-06-08-devrix-testing-quality/`

---

## Overview

本文档基于 `t-registry.md` 测试审计结果，定义 Devrix 测试质量增强需求。覆盖三大类问题：

1. **边界条件测试缺失** - 错误处理、超时、并发
2. **Mock 滥用导致真实场景缺失** - 需要引入 VCR/真实 API 测试
3. **断言不严格** - 需要增强验证深度

---

## Problem Analysis

### 1. Boundary Condition Test Gaps

| D-S | Module | Missing Tests |
|--------|--------|---------------|
| D2-S8 | Sandbox | shell injection (migrated from retired D2-S1) |
| D2-S2 | Compression | autocompact timeout fallback |
| D2-S3 | Memory | longterm failure handling |
| D3-S2 | StreamChat | SSE parse errors |
| D3-S1+D3-S2+D3-S3 | RouteModel/StreamChat/ProtectCall | 429 rate limit, network timeout |

### 2. Mock Overuse Issues

当前测试过度依赖 Mock，无法验证：
- 真实 LLM API 错误处理
- SSE 流解析边界情况
- 网络超时降级

### 3. Weak Assertion Patterns

现有测试仅验证"不报错"，缺少：
- 字段级别验证
- 返回值边界检查
- 状态转换完整性

---

## ADDED Requirements

### Requirement: D2-S8 Sandbox Boundary Testing (shell injection)

> D2-S1 PEV Verify 已退役；shell injection 测试迁移至 D2-S8。

**Priority**: P0

#### Scenario: Shell injection prevention

- GIVEN malicious bash command patterns
- WHEN `DefaultCommandPolicy().Validate` is invoked
- THEN dangerous commands MUST be rejected

<!-- RETIRED: D2-S1 PEV Verify timeout / WorkDir / concurrent isolation requirements removed with PEV engine -->

---

### Requirement: D2-S2 Compression Fallback

**Priority**: P0

#### Scenario: Autocompact LLM timeout fallback

- GIVEN Autocompact is triggered
- WHEN the summarization LLM call exceeds timeout (10s)
- THEN pipeline MUST skip autocompact step
- AND continue with truncation fallback
- AND record metric `ctx.compression.autocompact.timeout`

---

### Requirement: D3-S2 StreamChat Real API Testing

> **V3.0.0 重命名**：原"D3-S1 Adapter Real API Testing"（S 切法 7 段时）→ 现"D3-S2 StreamChat Real API Testing"（5+1 S 价值流化后；devrix-d3-sa-refine / DM-20260614-013）。

**Priority**: P1

#### Scenario: SSE parse error handling

- GIVEN LLM returns malformed SSE stream
- WHEN gateway parses the stream
- THEN it MUST handle gracefully
- AND return partial content if possible

#### Scenario: Network timeout

- GIVEN LLM API does not respond within timeout
- WHEN request is pending
- THEN gateway MUST timeout and return `CodeLLMTimeout`

---

### Requirement: D3-S3 ProtectCall Rate Limit

> **V3.0.0 重命名**：原"D3-S2 Gateway Rate Limit"（S 切法 7 段时）→ 现"D3-S3 ProtectCall Rate Limit"（Breaker+Retry 合并后，429 属 ProtectCall 韧性保护职责；devrix-d3-sa-refine / DM-20260614-013）。

**Priority**: P1

#### Scenario: 429 rate limit handling

- GIVEN LLM provider returns 429 Too Many Requests
- WHEN gateway processes the response
- THEN it MUST propagate `CodeLLMRateLimit` error
- AND circuit breaker MUST record failure
- AND retry logic SHOULD backoff appropriately

---

### Requirement: Strict Assertion Standards

**Priority**: P1

#### Scenario: SessionContext field validation

- GIVEN `MemoryManager.LoadOrInit` returns successfully
- THEN ALL fields MUST be validated:
  - `SessionID`: non-empty
  - `SystemPrompt`: matches input
  - `Messages`: empty slice (not nil)
  - `TokenBudget`: non-nil, valid limits
  - `CreatedAt`: non-zero timestamp

#### Scenario: Compression report validation

- GIVEN `Pipeline.Run` completes successfully
- THEN ALL report fields MUST be validated:
  - `StepsApplied`: non-empty array
  - `TokensBefore > TokensAfter`
  - `CompressionRatio`: within 0-1

---

##T 层测试点注册表

### D2 Context Engine Extensions

| T 层 ID | 描述 | L2 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D2-S8-A01-T02 | Shell injection attack prevention | Sandbox | `tests/security/shell_injection_test.go` | IMPLEMENTED |
| D2-S2-T08 | Autocompact timeout fallback | Compression | `internal/layers/contextengine/prepare/compression/autocompact_test.go` | PLANNED |

### D3 LLM Gateway Extensions

| T 层 ID | 描述 | L2 映射 | Test 位置 | Status | Legacy Alias |
|-------|------|---------|-----------|--------|--------------|
| **D3-S2-A01-T03** | SSE parse error handling | StreamChat F02 ParseSSE | `tests/integration/llm_real_api_test.go` | PLANNED | 旧 D3-S1-A01-T03 |
| **D3-S3-A01-T07** | LLM 429 rate limit handling | ProtectCall F05 StreamWithFallback | `tests/integration/llm_real_api_test.go` | PLANNED | 旧 D3-S2-A01-T06 |
| **D3-S4-A01-T03** | Token counter 中文准确性 | BudgetTokens F01 CountText (CJK) | `internal/layers/llmgateway/token/counter_test.go` | PLANNED | 旧 D3-S5-A01-T03 |

> **V3.0.0 重映射**（devrix-d3-sa-refine / DM-20260614-013）：T ID 按 5+1 S 价值流重排；旧 ID 100% alias 追溯见 `openspec/specs/d3-llm-gateway/t-registry.md §Legacy Archive`。L2 映射列由"技术角色词"（Adapter / Gateway / Token）改为"价值流 + F"（StreamChat F02 / ProtectCall F05 / BudgetTokens F01）。

### D5 Observability Extensions

| T 层 ID | 描述 | L2 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D5-S2-T06 | Compression P99 latency < 500ms | Metrics | `tests/performance/compression_test.go` | PLANNED |
| D5-S2-T07 | Concurrent session memory bounded | Metrics | `tests/performance/memory_test.go` | PLANNED |

---

## Directory Structure Additions

```
tests/
├── security/
│   └── shell_injection_test.go     # D2-S8-A01-T02
├── performance/
│   ├── compression_test.go         # D5-S2-T06
│   └── memory_test.go              # D5-S2-T07
└── fixtures/
    └── llm_responses/              # VCR recorded responses
```

---

## New Build Tags

| Tag | 用途 | 执行条件 |
|-----|------|----------|
| `live` | 真实 API 调用 | `go test -tags="integration live"` |
| `performance` | 性能测试 | `go test -tags=performance -bench=.` |
| `security` | 安全测试 | `go test -tags=security` |

---

## Acceptance Criteria

| ID | 描述 | 验证方法 |
|----|------|----------|
| AC-01 | D2-S8 shell injection + D2-S10 QueryLoop 核心 T 点通过 | `./scripts/test-unit.sh` |
| AC-02 | Shell injection 测试覆盖 15+ 危险模式 | `tests/security/shell_injection_test.go` |
| AC-03 | QueryLoop 多 session 隔离（integration） | `tests/integration/query_loop_*` |
| AC-04 | 压缩超时测试验证降级到 truncation | `TestPipeline_autocompact_timeout_skips_and_fallback` |
| AC-05 | Token counter 中文准确性误差 < 5% | `TestCounter_chinese_text_accuracy` |
| AC-06 | 性能测试 P99 < 500ms | `go test -tags=performance -bench=BenchmarkCompressionPipeline` |

---

## Dependencies

| 依赖 | 版本 | 用途 |
|------|------|------|
| testing/quick | builtin | Property-based testing |
| golang.org/x/sync/errgroup | latest | Concurrent test utilities |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Initial proposal with D-S numbering |
