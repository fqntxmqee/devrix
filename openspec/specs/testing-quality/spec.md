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

本文档基于 `l5-registry.md` 测试审计结果，定义 Devrix 测试质量增强需求。覆盖三大类问题：

1. **边界条件测试缺失** - 错误处理、超时、并发
2. **Mock 滥用导致真实场景缺失** - 需要引入 VCR/真实 API 测试
3. **断言不严格** - 需要增强验证深度

---

## Problem Analysis

### 1. Boundary Condition Test Gaps

| D-S | Module | Missing Tests |
|--------|--------|---------------|
| D2-S1 | PEV | timeout, shell injection, concurrent isolation |
| D2-S2 | Compression | autocompact timeout fallback |
| D2-S3 | Memory | longterm failure handling |
| D3-S1 | Adapter | SSE parse errors |
| D3-S2 | Gateway | 429 rate limit, network timeout |

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

### Requirement: D2-S1 PEV Boundary Testing

**Priority**: P0

#### Scenario: Verify timeout handling

- GIVEN a command takes longer than configured timeout
- WHEN `VerifyCommands` is invoked
- THEN it MUST return error with code `CodeVerifyTimeout`
- AND the error message MUST include elapsed time

#### Scenario: Shell injection prevention

- GIVEN malicious input with shell metacharacters
- WHEN `VerifyCommands` processes the input
- THEN it MUST reject: `;`, `$()`, `` ``, `|`, `&&`
- AND MUST NOT execute injected command

#### Scenario: WorkDir validation

- GIVEN a non-existent or inaccessible WorkDir
- WHEN `VerifyCommands` is invoked
- THEN it MUST return error `CodeVerifyWorkDirInvalid`

---

### Requirement: D2-S1 PEV Concurrent Safety

**Priority**: P0

#### Scenario: Concurrent session isolation

- GIVEN 10+ sessions processed concurrently
- WHEN `PEVEngine.Process` is called from multiple goroutines
- THEN each session MUST produce correct isolated results
- AND sessions MUST NOT interfere with each other's message history

#### Scenario: Context cancellation cleanup

- GIVEN a request is in progress
- WHEN the context is cancelled
- THEN `PEVEngine.Process` MUST return within 100ms
- AND MUST NOT leave goroutines hanging

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

### Requirement: D3-S1 Adapter Real API Testing

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

### Requirement: D3-S2 Gateway Rate Limit

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

## L5 Test Points Registry

### D2 Context Engine Extensions

| L5 ID | 描述 | L2 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-1-09 | Verify timeout returns CodeVerifyTimeout | PEV | `tests/integration/context_verify_commands_test.go` | PLANNED |
| L5-2-1-10 | Shell injection attack prevention | PEV | `tests/security/shell_injection_test.go` | PLANNED |
| L5-2-1-11 | PEV concurrent session isolation | PEV | `internal/layers/contextengine/pev_engine_test.go` | PLANNED |
| L5-2-1-12 | PEV context cancellation cleanup | PEV | `internal/layers/contextengine/pev_engine_test.go` | PLANNED |
| L5-2-2-08 | Autocompact timeout fallback | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | PLANNED |

### D3 LLM Gateway Extensions

| L5 ID | 描述 | L2 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-1-03 | SSE parse error handling | Adapter | `tests/integration/llm_real_api_test.go` | PLANNED |
| L5-3-2-06 | LLM 429 rate limit handling | Gateway | `tests/integration/llm_real_api_test.go` | PLANNED |
| L5-3-5-03 | Token counter 中文准确性 | Token | `internal/layers/llmgateway/token/counter_test.go` | PLANNED |

### D5 Observability Extensions

| L5 ID | 描述 | L2 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-2-06 | Compression P99 latency < 500ms | Metrics | `tests/performance/compression_test.go` | PLANNED |
| L5-5-2-07 | Concurrent session memory bounded | Metrics | `tests/performance/memory_test.go` | PLANNED |

---

## Directory Structure Additions

```
tests/
├── security/
│   └── shell_injection_test.go     # L5-2-1-10
├── performance/
│   ├── compression_test.go         # L5-5-2-06
│   └── memory_test.go              # L5-5-2-07
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
| AC-01 | L5-2-1-09 到 L5-2-1-12 实现并通过 | `./scripts/test-acceptance.sh` |
| AC-02 | Shell injection 测试覆盖 5+ 危险模式 | `grep -c dangerous tests/security/` |
| AC-03 | PEV 并发测试验证 10+ 并发 session | `TestPEVEngine_concurrent_sessions` |
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
