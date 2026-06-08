# Proposal: 测试质量增强

**Change ID:** devrix-testing-quality
**Demand ID:** DM-20260608-004
**Status:** S2 Design
**Author:** Architecture
**Date:** 2026-06-08

---

## 1. Background

Gemini 对 Devrix 项目进行了代码质量深度审查，审计范围覆盖 `internal/` 下所有测试文件。审查发现三个系统性问题：

1. **边界条件测试缺失** — 大量测试仅覆盖 golden path，缺少错误路径、超时、并发场景
2. **Mock 滥用** — LLM Gateway 测试仅使用 mock adapter，无法验证真实 I/O 行为（429 限流、SSE 解析错误、网络超时）
3. **断言不严格** — 常见模式是仅检查 `err == nil`，不验证错误类型、字段值、状态转换

## 2. Problem Statement

### 2.1 边界测试缺失

| 模块 | 缺失场景 | 风险 |
|------|---------|------|
| Verify Commands | 命令超时、exit code 127、非零退出码 | 命令执行异常时行为未定义 |
| PEV Engine | session 隔离、context 取消清理、MaxIterations 耗尽 | 并发场景可能出现 goroutine 泄漏或状态污染 |
| Compression | Autocompact 超时降级、空消息、压缩后仍超预算 | 边缘情况下压缩行为不确定 |

### 2.2 Mock 与真实 I/O 差距

当前 LLM Gateway 的集成测试全部使用 `MockAdapter`：

```go
// 典型 mock 模式 — 无法捕获真实 API 行为
adapter := NewMockAdapter()
adapter.On("Stream", mock.Anything, mock.Anything).Return(mockCh, nil)
```

Mock 无法验证：429 响应解析、SSE 数据帧错误、连接超时/重置、响应体截断。

### 2.3 断言不严格

常见模式：
```go
// 当前 — 过于宽松
result, err := doSomething()
assert.NoError(t, err)

// 应有 — 结构化验证
assert.NoError(t, err)
assert.Equal(t, expectedCode, result.Code)
assert.NotEmpty(t, result.Timestamp)
```

## 3. Proposed Solution

### 3.1 三层测试策略

| 层级 | 测试类型 | 工具 | 说明 |
|------|---------|------|------|
| **基础** | 单元测试 | `testing` + `quick` | 边界条件、错误路径、并发安全 |
| **集成** | VCR 录制回放 | `tests/fixtures/llm_responses/` | 真实 API 响应录制，离线回放 |
| **性能** | 基准测试 | `-bench` + `-tags=performance` | P99 延迟、内存增长 |

### 3.2 VCR 模式

```
首次运行（联网）:
  go test -tags=integration,live → 录制真实 API 响应到 fixtures/

后续运行（离线）:
  go test -tags=integration → 回放 fixtures/ 中的录制响应
```

### 3.3 Shell Injection 测试对齐

Shell injection 测试直接对齐 `devrix-tool-security` 的 `defaultDenyPatterns`，使用相同的 16 条危险正则模式验证拦截效果。

### 3.4 不改动的部分

- 不修改生产代码
- 不修改现有测试框架
- 不引入新的外部依赖（VCR 使用标准库 `httptest` 实现）
- 不改变 CI 流水线结构

---

## 4. Success Metrics

| Metric | Target |
|--------|--------|
| 新增 L5 测试点 | 10 个（P0: 5, P1: 3, P2: 2） |
| Shell injection 覆盖 | 16 种危险模式 |
| PEV 并发测试 | 10+ 并发 session 无泄漏 |
| Token counter 中文误差 | < 5% |
| 压缩 P99 延迟 | < 500ms |
| 新增覆盖率提升 | > 5%（边界路径） |

---

## 5. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Live API 测试不稳定 | 中 — 依赖外部 API 可用性 | VCR 录制回放模式，CI 默认使用 fixtures |
| 性能测试 CI 资源竞争 | 低 | 使用 `-tags=performance` 独立执行，不在普通 CI 中运行 |
| Shell injection 模式与 tool-security 不一致 | 中 | 共享 deny patterns 常量，spec 中明确引用 |

---

## 6. 任务估算

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 Boundary Tests | 2 | 5h |
| M2 VCR + Real API | 1 | 4h |
| M3 Strict Assertions | 1 | 3h |
| M4 Performance Baseline | 1 | 4h |
| **合计** | **5** | **~16h** |
