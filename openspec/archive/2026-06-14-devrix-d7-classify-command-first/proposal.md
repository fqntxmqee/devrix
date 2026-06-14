---
proposal-id: devrix-d7-classify-command-first
title: D7-S5-T03 + T06 ClassifyIntent 规则置信度 + Command-first 端到端闭环 — 提案
demand-id: DM-20260614-005
status: S2_Proposal
created: 2026-06-14
last-updated: 2026-06-14
---

# D7-S5-T03 + T06 — 提案

## 1. 方案概览

| 方案 | 端到端断言 | CommandFirst=false 回归 | t-registry 同步 | 影响实现代码 | 决策 |
|------|----------|----------|-----------|----------|------|
| A. 仅 t-registry 标 IMPLEMENTED | ❌ | ❌ | ✅ | ❌ | ❌ |
| **B. 端到端测试 + 配置回归 + t-registry 同步** | ✅ | ✅ | ✅ | ❌ | ✅ |
| C. 拆 ShadowClassifier 端到端到独立 file | ✅ | ✅ | ✅ | ❌ | ❌（过设计） |

**决议**：选 **B**。理由：

- A 不满足 testing.md «P0 T 测试点必须有可执行证据» 规则
- B 在现有 `orchestrator_test.go` / `classifier_test.go` 增量 2 个测试，复用现有 fixture（fakeD2 / stubLLM / waitForCalls），最小变更
- C 拆文件破坏现有「shadow 测在 shadow_classifier_test.go、orchestrator 测在 orchestrator_test.go」的分层，不必要

## 2. 方案 B 详细

### 2.1 端到端测试 1：CommandFirst + ShadowClassifier 启用 → LLM 未调用

定位在 `internal/layers/d7/orchestrator_test.go`，验证 D7-S5-T06 端到端断言：

```go
// T: D7-S5-T06 — Command-first 路径在 ShadowClassifier 启用时不触发 LLM
func TestSessionOrchestrator_CommandFirst_ShadowNotCalled(t *testing.T) {
    exec := &fakeD2{}
    rule := NewRuleClassifier(DefaultConfig())
    llm := &stubLLM{result: IntentClassification{Kind: IntentOrchestrate, Confidence: 80}}
    mtr := newShadowTestMeter(t)
    m := NewShadowMetrics(mtr)
    shadow := NewShadowClassifier(rule, llm, m, 500)
    orch := NewSessionOrchestrator(DefaultConfig(), exec, WithShadowClassifier(shadow))
    ch, err := orch.ProcessMessage(context.Background(), ProcessRequest{
        SessionID: "sess-cmd-shadow",
        Message:   "/plan add auth",
    })
    if err != nil {
        t.Fatalf("ProcessMessage err: %v", err)
    }
    for range ch {
    }
    // Allow async path a window — shadow MUST NOT have been called.
    time.Sleep(30 * time.Millisecond)
    if atomic.LoadInt32(&llm.calls) != 0 {
        t.Fatalf("LLM called on command path: calls=%d", llm.calls)
    }
    if exec.calls != 1 {
        t.Fatalf("D2 must be called once for command path, got %d", exec.calls)
    }
}
```

### 2.2 配置回归测试 2：CommandFirst=false 时 /plan 不再短路

定位在 `internal/layers/d7/classifier_test.go`，验证规则在配置关闭时的退化行为：

```go
// T: D7-S5-T06 (negative) — CommandFirst=false 时 /plan 不再匹配 IntentCommand
func TestRuleClassifier_Classify_CommandFirst_Disabled(t *testing.T) {
    cfg := DefaultConfig()
    cfg.CommandFirst = false
    c := NewRuleClassifier(cfg)
    got, err := c.Classify(context.Background(), "/plan add auth")
    if err != nil {
        t.Fatalf("Classify err: %v", err)
    }
    if got.Kind == IntentCommand {
        t.Fatalf("CommandFirst=false should not match IntentCommand, got kind=%q", got.Kind)
    }
}
```

### 2.3 t-registry 同步

```diff
- | D7-S5-T03 | ClassifyIntent 规则高置信 → simple | D7-S5-A01 | — | PLANNED (v1.0) | P0 |
+ | D7-S5-T03 | ClassifyIntent 规则高置信 → simple | D7-S5-A01 | `internal/layers/d7/classifier_test.go` | IMPLEMENTED | P0 |
- | D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | D7-S5-A01 | — | PLANNED (v1.0) | P0 |
+ | D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | D7-S5-A01 | `internal/layers/d7/{classifier_test.go,shadow_classifier_test.go,orchestrator_test.go}` | IMPLEMENTED | P0 |
```

域统计：`Total 46 / IMPLEMENTED 37 / PLANNED 7 / P0 26` → `46 / 39 / 5 / 26`
根索引 D7 行同步；总计 `IMPLEMENTED 256 / PLANNED 13` → `258 / 11`

## 3. 备选方案

### 3.1 方案 A：仅 t-registry 标 IMPLEMENTED

- 实施：直接改 t-registry 状态
- 缺点：缺端到端「shadow + command」证据；缺 CommandFirst=false 回归；违反 testing.md «每个 T 必须有可执行证据»

### 3.2 方案 C：拆 ShadowClassifier 端到端到独立文件

- 实施：建 `orchestrator_shadow_integration_test.go`
- 缺点：过设计，引入新文件成本 > 收益；测试仅 2 个，归到现有文件即可

## 4. 关键决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 是否动 classifier/orchestrator 实现 | **否** | 既有行为已正确，仅 T-registry 同步 + 测试补强 |
| 异步等待方式 | **time.Sleep(30ms) 兜底** | 与 shadow_classifier_test.go 既有模式一致；LLM 未被调用是「未发生」断言，sleep 提供时间窗 |
| CommandFirst=false 回退到何种 Kind | **不限定，仅断 != IntentCommand** | 短消息 `/plan add auth` 长度 14 → 进入 short-default fast；但断言不依赖具体回退，更稳健 |
| 是否补 D6 metric | **否** | D6-D7-T01~T06 已闭环；本 Change 仅 T03/T06 |

## 5. 实施计划

| 阶段 | 估算 | 备注 |
|------|------|------|
| S3 设计 | 15 分钟 | design.md（精简） |
| S3-Gate | 5 分钟 | 内部 review |
| S4 实现 | 30 分钟 | 2 个测试 + t-registry 双更新 |
| S4-Gate | 10 分钟 | review-code.md |
| S5 验收 | 15 分钟 | go test -race + acceptance-report |
| S6 归档 | 10 分钟 | move + 索引 |

总计约 1.5 小时。
