# Tasks: devrix-testing-quality

**Change ID:** devrix-testing-quality
**Status:** S4 Development
**Based on:** design.md, `specs/testing-quality/spec.md`

---

## Milestone 1: Boundary Tests（P0）

### Definition of Done
- [x] Verify 命令超时/退出码/非零退出码测试
- [x] Shell injection 16 种危险模式全部覆盖
- [x] PEV 并发隔离 + context 取消 + MaxIterations 耗尽测试
- [x] Autocompact 超时降级 + 空消息 + 超预算测试

### Tasks

- [x] **T1**: 扩展 `tests/integration/context_verify_commands_test.go`
  - 超时返回 CodeVerifyTimeout（命令被 kill）
  - 不存在的命令 exit code 127
  - 非零退出码捕获（exit 42）
  - L5: L5-CTX-26
  - Estimate: 2h
  - Dependencies: None

- [x] **T2**: 新增 `tests/security/shell_injection_test.go`
  - 16 种危险命令全覆盖
  - 对齐 devrix-tool-security 的 deny patterns
  - PEV 并发 session 隔离 (10+ sessions)
  - PEV context 取消 goroutine 清理
  - MaxIterations 耗尽退出
  - L5: L5-CTX-27, L5-CTX-28, L5-CTX-29
  - Estimate: 3h
  - Dependencies: None

---

## Milestone 2: VCR + Real API（P1）

### Definition of Done
- [x] VCR 录制/回放核心实现
- [x] DeepSeek + MiniMax fixtures
- [x] 429 限流/SSE 错误/网络超时场景覆盖

### Tasks

- [x] **T3**: 新增 `tests/fixtures/llm_responses/vcr.go` + fixtures
  - VCRTransport 实现 http.RoundTripper
  - 录制模式：调用真实 API 并保存到 fixture 文件
  - 回放模式：从 fixture 文件读取录制响应
  - DeepSeek fixtures: 正常/sse错误/超时
  - MiniMax fixtures: 正常/429/500
  - 新增 `tests/integration/llm_real_api_test.go`
  - L5: L5-LLM-17, L5-LLM-18
  - Estimate: 4h
  - Dependencies: None

---

## Milestone 3: Strict Assertions（P1）

### Definition of Done
- [x] 关键 L5 测试点增强为结构化断言
- [x] Token counter 中文/混合 CJK/嵌套 JSON 准确性验证

### Tasks

- [x] **T4**: 增强现有测试断言 + Token 准确性
  - SessionContext 反序列化字段级验证
  - Compression Report 结构完整性验证
  - 错误类型断言（非仅 err != nil）
  - Token counter: 中文误差 < 5%、混合 CJK、嵌套 JSON
  - L5: L5-LLM-19
  - Estimate: 3h
  - Dependencies: None

---

## Milestone 4: Performance Baseline（P2）

### Definition of Done
- [x] 压缩 P99 延迟基准
- [x] 并发 session 内存增长基准

### Tasks

- [x] **T5**: 新增性能测试
  - `tests/performance/compression_test.go`: 50 消息 100 次迭代 P99 < 500ms
  - `tests/performance/memory_test.go`: 10 并发 session 内存增长 < 50MB
  - `-tags=performance` 独立标签
  - L5: L5-OBS-19, L5-OBS-20
  - Estimate: 4h
  - Dependencies: None

---

## 任务统计

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 Boundary Tests | 2 | 5h |
| M2 VCR + Real API | 1 | 4h |
| M3 Strict Assertions | 1 | 3h |
| M4 Performance Baseline | 1 | 4h |
| **合计** | **5** | **~16h** |

---

## 依赖关系图

```
T1 ──┐
T2 ──┤
T3 ──┼── (全部独立，可并行)
T4 ──┤
T5 ──┘
```

## 执行顺序建议

1. **并行**: T1, T2, T3, T4, T5（无任务间依赖）
2. 所有任务仅添加/修改测试文件，不影响生产代码
