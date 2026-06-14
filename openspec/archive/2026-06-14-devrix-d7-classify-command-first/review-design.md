---
review-id: review-design-devrix-d7-classify-command-first
phase: S3-Gate
demand-id: DM-20260614-005
status: APPROVED
reviewer: in-flight architect (self-review)
created: 2026-06-14
---

# S3-Gate Design Review — devrix-d7-classify-command-first

## 1. 设计目标一致性

| 检查 | 结论 |
|------|------|
| demand.md AC1~AC7 全部在 design 中映射 | ✅ AC1→§3.1, AC2→§3.2, AC3/AC4→§4, AC5→§6 checklist, AC6→cov gate, AC7→acceptance |
| 不修改 RuleClassifier / ShadowClassifier / Orchestrator 实现 | ✅ §1 / §6 明确 |
| Tail-only 行为复用既有 ShadowClassifier 短路 | ✅ §2.2 引证 `result.Kind != IntentOrchestrate → return` |
| CommandFirst=false 回归独立于具体回退 Kind | ✅ §3.2 「断言 `!= IntentCommand`」 |

## 2. 范围 / 风险审视

| 项 | 评价 |
|----|------|
| 测试文件数 | 2（既有文件追加，无新文件） |
| 跨包符号引用 | 均在 `package d7`，零依赖外溢 |
| time.Sleep 风险 | 30ms 仅作「未发生」窗口；现有 shadow_classifier_test.go 同一惯例已稳定 |
| t-registry 数值漂移 | 表格精确给出，S4 按表填，可机械执行 |

## 3. 质疑与回应

| Q | A |
|---|---|
| 为什么不强制 ShadowClassifier 类型断言 LLM 路径未执行？ | stubLLM.calls 计数器是更精确的「未调用」证据；类型断言无法覆盖 LLMIntentClassifier 接口未触发 |
| 为什么不用 channel 而 sleep？ | shadow 是「未启动 goroutine」的负向断言，无 channel 可阻塞；sleep 给「假设其会启动则会调用」的反证窗口 |
| AC2 是否应限定回退 Kind？ | 不限定。Short-default 规则后续可能调整（如阈值改成 64 字符），强绑定 Kind 会让规则演进时回归脆弱 |
| 为何不在 spec.md 加新 scenario？ | T03 / T06 行为已在 d7-orchestration/spec.md 表达（S5 / Command-first 矩阵），本次仅同步状态 |

## 4. 通过判定

| Gate Check | 状态 |
|-----------|------|
| demand.md 完整 + AC 全部 P0 | ✅ |
| proposal.md 方案对比 + 决议明确 | ✅ |
| design.md 测试可执行可落地 | ✅ |
| 实施 checklist 可机械执行 | ✅ |
| 不动既有实现代码 | ✅ |
| 风险全部缓解 | ✅ |

## 5. 决议

**APPROVED**。允许进入 S4 实现。
