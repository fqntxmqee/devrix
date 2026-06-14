---
design-id: devrix-d7-classify-command-first
title: D7-S5-T03 + T06 端到端闭环 — 技术设计
demand-id: DM-20260614-005
status: S3_Design
created: 2026-06-14
last-updated: 2026-06-14
---

# 技术设计

## 1. 设计目标

闭环 D7 v1.0 最后 2 个 P0 PLANNED：D7-S5-T03（规则置信度）+ D7-S5-T06（Command-first 不触发 LLM）。无生产代码变更，仅补端到端测试 + t-registry 同步。

## 2. 现状分析

### 2.1 RuleClassifier 当前行为（classifier.go）

```
Classify(ctx, message):
  - empty?  → IntentSkip, conf=100
  - CommandFirst && /cmd? → IntentCommand, conf=100
  - fastRule match? → IntentFast, conf=95
  - len ≤ 32 && no \n\;? → IntentFast, conf=70
  - else → IntentOrchestrate, conf=60
```

### 2.2 ShadowClassifier tail-only 行为（shadow_classifier.go:Classify）

```
Classify(ctx, message):
  result = rule.Classify(ctx, message)
  if result.Kind != IntentOrchestrate → return result (LLM 不调用)
  go shadowAsync(WithoutCancel(ctx), message, result)
  return result
```

→ 命令路径 `IntentCommand` ≠ `IntentOrchestrate`，自然 short-circuit，不触发 LLM。

### 2.3 SessionOrchestrator dispatch（orchestrator.go:ProcessMessage）

```
ProcessMessage(ctx, req):
  intent = (shadow ?? classifier).Classify(ctx, req.Message)
  switch intent.Kind:
    IntentSkip → empty channel
    IntentCommand → handleCommand → fastPath.Run with "[command:/plan]" hint
    IntentFast → fastPath.Run with ""
    IntentOrchestrate → orchestrate → fastPath.Run with "[orchestrate: ...]" hint
```

## 3. 测试设计

### 3.1 AC1: 端到端 — ShadowClassifier + /plan

**测试位置**：`internal/layers/d7/orchestrator_test.go`

**测试名**：`TestSessionOrchestrator_CommandFirst_ShadowNotCalled`

**Setup**：
- `exec = &fakeD2{}`（既有 fixture）
- `rule = NewRuleClassifier(DefaultConfig())`
- `llm = &stubLLM{result: IntentOrchestrate, conf 80}`（来自 shadow_classifier_test.go）
- `mtr = newShadowTestMeter(t)`
- `m = NewShadowMetrics(mtr)`
- `shadow = NewShadowClassifier(rule, llm, m, 500)`
- `orch = NewSessionOrchestrator(DefaultConfig(), exec, WithShadowClassifier(shadow))`

**Act**：
```go
ch, _ := orch.ProcessMessage(ctx, ProcessRequest{
    SessionID: "sess-cmd-shadow",
    Message:   "/plan add auth",
})
for range ch {}
time.Sleep(30 * time.Millisecond)
```

**Assert**：
- `atomic.LoadInt32(&llm.calls) == 0`
- `exec.calls == 1`

**为什么 sleep 30ms 安全**：
- ShadowClassifier.Classify 命中 IntentCommand 时**同步 return**，不启动 goroutine
- 30ms 仅作为「未发生」窗口；与 shadow_classifier_test.go 既有惯例一致

### 3.2 AC2: 配置回归 — CommandFirst=false

**测试位置**：`internal/layers/d7/classifier_test.go`

**测试名**：`TestRuleClassifier_Classify_CommandFirst_Disabled`

**Setup**：
```go
cfg := DefaultConfig()
cfg.CommandFirst = false
c := NewRuleClassifier(cfg)
```

**Act**：`got := c.Classify(ctx, "/plan add auth")`

**Assert**：`got.Kind != IntentCommand`

**关键**：不限定回退到具体 Kind。`/plan add auth` 长度 14，理论上落入 short-default → IntentFast，但测试不绑定具体回退，避免规则微调时回归 brittle。

### 3.3 复用现有 fixture

| Fixture | 来源 | 用途 |
|---------|------|------|
| `fakeD2` | orchestrator_test.go:17 | 计数 D2 调用 |
| `stubLLM` | shadow_classifier_test.go:25 | 计数 LLM 调用 |
| `newShadowTestMeter` | shadow_classifier_test.go:16 | 构造测试 Meter |
| `NewShadowMetrics` | shadow_classifier.go | 构造 ShadowMetrics |

跨包 fixture 共享：`stubLLM` 和 `newShadowTestMeter` 都在 `package d7`，可直接使用。

## 4. t-registry 更新

### 4.1 域级 (`openspec/specs/d7-orchestration/t-registry.md`)

```diff
- | D7-S5-T03 | ClassifyIntent 规则高置信 → simple | D7-S5-A01 | — | PLANNED (v1.0) | P0 |
+ | D7-S5-T03 | ClassifyIntent 规则高置信 → simple | D7-S5-A01 | `internal/layers/d7/classifier_test.go` | IMPLEMENTED | P0 |
- | D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | D7-S5-A01 | — | PLANNED (v1.0) | P0 |
+ | D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | D7-S5-A01 | `internal/layers/d7/{classifier_test.go,shadow_classifier_test.go,orchestrator_test.go}` | IMPLEMENTED | P0 |

  Statistics:
- | Total | IMPLEMENTED | PARTIAL | PLANNED | P0 |
- | 46 | 37 | 2 | 7 | 26 |
+ | 46 | 39 | 2 | 5 | 26 |

  By Scenario:
- | D7-S5 | 7 | 3 | 4 |
+ | D7-S5 | 7 | 5 | 2 |
```

### 4.2 根索引 (`openspec/t-registry.md`)

```diff
- | D7 Orchestration | ... | 46 | 37 | 7 | 26 |
+ | D7 Orchestration | ... | 46 | 39 | 5 | 26 |

- 总计: 272 · IMPLEMENTED 256 · PLANNED 13 · PARTIAL 2 · P0 118
+ 总计: 272 · IMPLEMENTED 258 · PLANNED 11 · PARTIAL 2 · P0 118
```

## 5. 风险与缓解

| 风险 | 缓解 |
|------|------|
| stubLLM 跨文件可见性 | 同 package `d7`，符号自动可见 |
| time.Sleep 在 CI 上不稳定 | 30ms 远超 shadow goroutine 启动开销（<1ms）；既有 shadow_classifier_test.go 验证过 |
| Short-default 规则微调误伤 AC2 | 断言改为 `!= IntentCommand`，与具体回退解耦 |
| t-registry 数值漂移 | 设计文档明确给出新旧数值；S4 实现按表填入 |

## 6. 实施 checklist

- [ ] orchestrator_test.go 增 `TestSessionOrchestrator_CommandFirst_ShadowNotCalled`
- [ ] classifier_test.go 增 `TestRuleClassifier_Classify_CommandFirst_Disabled`
- [ ] gofmt + go vet ./internal/layers/d7/...
- [ ] go test -race -count=10 ./internal/layers/d7/...
- [ ] d7 t-registry 改 2 行 + Statistics + By Scenario
- [ ] 根 t-registry 改 D7 行 + 总计行
- [ ] tasks.md 标实施完成
