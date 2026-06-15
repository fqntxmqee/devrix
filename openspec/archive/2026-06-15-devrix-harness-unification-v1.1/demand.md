---
demand-id: DM-20260612-013
title: Harness Unification v1.1 — TD-QL-03 兜底接线 + design/tasks.md 补全
source: devrix-harness-unification v1.0 S4-Gate CONDITIONAL（2026-06-12）
priority: P1
status: S1_Proposal
dsaft_domain: context-engine
parent_change: devrix-harness-unification
created: 2026-06-12
---

# Harness Unification v1.1

## 1. 背景

`devrix-harness-unification` v1.0 S4-Gate 给出 **⚠️ CONDITIONAL PASS** 裁决（2026-06-12）：
- **`query_loop.enabled` 默认 true、统一压缩入口、`PathRegressionProbe` 注册 D6、TD-QL-01/02 恢复** — 全部 ✅
- **TD-QL-03（`Loop.FallbackLLM` / `FallbackOnErr` 兜底）字段已就位**，但**生产路径 `runViaQueryLoop` 尚未消费该字段**（仅 schema 暴露，未 wiring）
- **`design.md` / `tasks.md` 缺失**（v1.0 仅交付 demand.md / proposal.md / acceptance-report.md）

本 v1.1 跟进项目标：完成 TD-QL-03 兜底接线 + 补全 OpenSpec 文档。

## 2. 改动范围

### 2.1 TD-QL-03 兜底接线

`internal/layers/contextengine/query/loop.go::runViaQueryLoop` 当前行为：
```
LLM 调用失败 → 直接返回 error（无 FallbackLLM 兜底）
```

v1.1 期望行为：
```
LLM 调用失败 → 检查 Loop.FallbackLLM 是否配置
              → 若配置：重试一次 FallbackLLM，event 标记 fallback=true
              → 若未配置：返回原 error
              → 检查 Loop.FallbackOnErr 错误白名单
              → 仅匹配白名单时才走兜底
```

### 2.2 集成测试

- `TestL5_2_11_TD03_FallbackLLMWired`：主 LLM 返回 500 → FallbackLLM 成功 → event.Metadata["fallback"] = "true"
- `TestL5_2_11_TD03_FallbackOnErrFilter`：主 LLM 返回 400（不在白名单）→ 直接返回 error，不走 Fallback
- `TestL5_2_11_TD03_NoFallbackConfigured`：Loop.FallbackLLM 空 → 主 LLM 失败时返回 error（保持当前行为）

### 2.3 文档

- 补 `devrix-harness-unification/design.md`：技术设计（TD-QL-03 兜底决策树、error 白名单配置格式）
- 补 `devrix-harness-unification/tasks.md`：实施任务拆解

## 3. 验收标准

### P0（阻止合并）

- [ ] TD-QL-03 兜底接线完成（`runViaQueryLoop` 消费 `Loop.FallbackLLM` / `Loop.FallbackOnErr`）
- [ ] 3 个集成测试 PASS（见 §2.2）
- [ ] 现有 D2-S11-T01~04 / TD01 / TD02 测试全部 PASS（无回归）
- [ ] `-race -count=1` PASS

### P1（必须完成）

- [ ] 补全 `devrix-harness-unification/design.md` + `tasks.md`
- [ ] `PathRegressionProbe` 增加 `td_ql_03_wired` 维度（生产路径消费 FallbackLLM 时计数 = 1）

### P2（建议完成）

- [ ] FallbackLLM 调用记录到 D5 metrics（`runtime.fallback_llm_total`）

## 4. 依赖与顺序

```
v1.0（已合）→ v1.1（本需求）→ 后续 v1.2 移除 harnessEnabled 配置项
```

## 5. 回归风险

- TD-QL-03 接线可能引入新的 LLM 调用路径，需在 `runViaQueryLoop` 内做并发安全
- FallbackLLM 配置缺失时必须保持当前行为（直接返回 error），向后兼容
- error 白名单正则可能误匹配（设计时需明确语法）
