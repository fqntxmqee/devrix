---
review-id: review-code-devrix-d7-classify-command-first
phase: S4-Gate
demand-id: DM-20260614-005
status: APPROVED
reviewer: in-flight reviewer (self-review)
created: 2026-06-14
---

# S4-Gate Code Review — devrix-d7-classify-command-first

## 1. 变更摘要

| 文件 | 类型 | 行数变化 |
|------|------|---------|
| `internal/layers/d7/orchestrator_test.go` | 修改 | +33 (新测试) +1 (import atomic) |
| `internal/layers/d7/classifier_test.go` | 修改 | +17 (新测试) |
| `openspec/specs/d7-orchestration/t-registry.md` | 修改 | T03/T06 状态 + Statistics + Scenario + Revision |
| `openspec/t-registry.md` | 修改 | Version + D7 行 + 总计 |
| `openspec/changes/devrix-d7-classify-command-first/*` | 新增 | demand/proposal/design/tasks/review-design |

**生产代码：零修改**。

## 2. 代码审查检查表

### 2.1 测试质量

| 检查 | 结果 |
|------|------|
| 测试名称遵循 `Test{Subject}_{Scenario}_{Expected}` | ✅ |
| T 注释明确关联 D7-S5-T06 / T03 | ✅ |
| 复用既有 fixture（fakeD2 / stubLLM / newShadowTestMeter） | ✅ |
| 无 t.Parallel + 共享状态污染 | ✅ |
| 错误断言信息丰富（含实际值） | ✅ |
| race detector -count=10 全绿 | ✅ |
| 无 `time.Sleep` 替代信号（30ms 是「未发生」窗口，无替代手段） | ✅ 已设计认可 |

### 2.2 testing.md 规范遵循

| 规则 | 满足 |
|------|------|
| 每个 P0 T 必须有可执行证据 | ✅ T03/T06 均有 |
| 表驱动 / 单元独立运行 | ✅ |
| 复用 helper（waitForCalls 等） | N/A（本次仅命令路径，无异步等待）|
| t-registry Test 位置准确 | ✅ |

### 2.3 coding.md / golang/coding-style 遵循

| 规则 | 满足 |
|------|------|
| gofmt | ✅ |
| go vet 零警告 | ✅ |
| 函数 < 50 行 | ✅ 33 / 17 |
| 文件 < 800 行 | ✅ orchestrator_test.go ~320 / classifier_test.go ~130 |
| 错误显式处理 | ✅ |
| 无硬编码 secret | ✅ |
| 无 panic 用于业务错误 | ✅ |
| import 分组 | ✅ |

### 2.4 范围一致性

| 检查 | 结果 |
|------|------|
| 与 demand.md AC 一一对应 | ✅ AC1→新增 orch test / AC2→新增 classifier test / AC3-AC4→t-registry |
| 不修改 RuleClassifier / ShadowClassifier / Orchestrator 实现 | ✅ |
| 不引入新文件 | ✅ 仅追加现有测试文件 |
| 不引入新依赖 | ✅ |

## 3. 边界与回归风险

| 项 | 评估 |
|----|------|
| 30ms sleep 是否在慢 CI 上失败 | 低 — 现有 shadow_classifier_test.go 同模式已稳定；命令路径同步 return，goroutine 不会启动 |
| AC2 与 short-default 规则耦合 | 已设计为「!= IntentCommand」，与具体回退解耦 |
| t-registry 统计数值 | 经表格机械执行，无歧义；CI 一致性检查可独立 cross-check |
| d7 包覆盖率退化 | 86.7% > 80%，反而略微上升（增量测试覆盖既有路径） |

## 4. 质疑回应

| Q | A |
|---|---|
| 为什么不在 d7-orchestration/spec.md 加新 Gherkin scenario？ | spec 表述未变（命令路径 + tail-only），本次仅同步 T 状态。spec.md 变更门槛是「合约/行为变化」 |
| AC1 是否应进一步断言 ShadowMetrics.Disabled 未增？ | 不必要：tail-only 命令路径不进入 shadowAsync，metrics 字段从未被触碰；测试已检测 LLM 0 调用，覆盖更强 |
| 是否补 metric assertion？ | 命令路径不产生 D6 metric（已通过 callD6Validator nil-check 验证），无需新断言 |

## 5. 通过判定

| Gate Check | 状态 |
|-----------|------|
| 全部测试通过 (`go test -race -count=10`) | ✅ |
| go vet 零警告 | ✅ |
| gofmt 一致 | ✅ |
| 覆盖率 ≥ 80% (86.7%) | ✅ |
| t-registry 数值同步且自洽 | ✅ |
| 范围与 demand 一致 | ✅ |

## 6. 决议

**APPROVED**。允许进入 S5 验收。
