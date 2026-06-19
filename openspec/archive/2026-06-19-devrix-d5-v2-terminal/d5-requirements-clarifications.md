# D5 Observability — 需求澄清与 Review 记录

**Change ID:** devrix-d5-v2-terminal  
**Demand ID:** DM-20260619-006  
**Status:** S3 Draft  
**用途:** Review R1/R2 澄清归档；博弈论对焦前置问题清单

---

## 1. 领域定位澄清

### Q1: D5 是「技术模块集合」还是「价值流域」？

**决议（DM-20260615-001 + 本 change）：** 价值流域。Canonical S21–S24 表达可验证承诺；Legacy S1–S9 冻结追溯。

### Q2: D5 是否拥有 Turn 主循环 span？

**决议：** **否。** D7 创建 `orchestration.turn.*`；D5 提供 `Op*` 常量、Bridge、Registry。见 `d5-boundary.md` §4。

### Q3: D5 诊断失败是否阻塞业务？

**决议（倾向）：** **否。** Doctor/Coverage 为审计面；`HealthCheck` degraded 不阻断 Process（与 Graceful Degradation 一致）。**待博弈论对焦确认 OQ-2。**

---

## 2. S 层切法澄清

### Q4: 为何 Instrument 合并 Tracer+Metrics+Logger+Telemetry？

**理由（博弈论）：** 消费者目标函数是「为我的操作产生可观测信号」，不是「选哪个子模块」。拆成 9 S 鼓励「只测 tracer 不测 logger」的局部最优。

### Q5: 为何 S23 不拆 S25？

**理由：** Tracker/Doctor/FaultInject 均属审计子博弈；增 S 会震荡 T ID。子承诺 C3a–C3e 在 A 层编排。**待 OQ-1 对焦。**

### Q6: DebugFilter 为何归 S21 而非 S23？

**理由：** 物理在 `instrument/logger/debugfilter/`；语义是日志管道滤波，不是事后诊断报告。

---

## 3. 物理路径澄清

### Q7: bridge 为何 v2.1 删除而非再保留 1 release？

**理由：** grep 显示仅 D5 内部 5 处 legacy import；D6 已删 bridge；双路径维护激励开发者继续用 Deprecated 包。

### Q8: 根目录 `genai_tokens.go` / `llm_log.go` 归哪？

**决议：** `instrument/metrics/genai_tokens.go`（S21）；`diagnose/incident/llm_log.go`（S23 C3b）。

---

## 4. T 层澄清

### Q9: Doctor T 为 D5-S23-A03-T* 但 A03 是 GenerateDailyReport？

**决议：** **T ID 冻结**；canonical Activity = **A10 RunDoctorChecks**；a-registry 与 t-registry 用 `canonical_a` 列校正。

### Q10: PLANNED T 如何处理？

| T ID | 计划 |
|------|------|
| D5-S21-A05-T01/T02 | 补测或映射既有 integration → IMPLEMENTED |
| D5-S23-A06-T02 | 验证 Health coverage 字段 → IMPLEMENTED |

---

## 5. 跨域澄清

### Q11: D2 TrackerSurface 与 D5 tracker 关系？

**决议：** D5 `diagnose/tracker` 唯一写 SoT；D2 只读 `Recent()`。禁止 D2 新建 tracker 写模型。

### Q12: D6 metrics 写哪里？

**决议：** D6 guard 经 OpenTelemetry 写入 D5 meter（见 `d6-domain.md`）；D5 不实现 Guard 逻辑。

---

## 6. Grill Review 预留问题（给 Claude）

1. **Referee 边界：** D5 审计失败时，系统应 warn 还是 fail？对 on-call 激励有何影响？
2. **Coverage as gate：** zero_hit 是否应成为发布硬门禁？与 D6 Delta gate 如何分工？
3. **S23 子承诺上限：** C3a–C3e 是否已触及「万能 S」边界？何时应拆 S25？
4. **选择性失明：** RecordHit 独立采样是否足够？是否需要「未 Register 的 op 硬拒绝 Start」？
5. **FaultInject 伦理：** testbuild only 是否足够防生产误开？
6. **Legacy harness metric：** `legacy_harness` path label 保留多久？对路径分流观测有何扭曲？

---

## 7. L1–L5 需求映射（DSAFT 追溯）

| 层级 | 本 change 产出 |
|------|----------------|
| L1 Domain | D5 不变；`d5-domain.md` North Star |
| L2 Scenario | S21–S24 + S0 终态登记；S23 子承诺 |
| L3 Activity | a-registry v4.0 +4 A |
| L4 Function | f-registry v3.0 +诊断 F |
| L5 Test | t-registry canonical 列校正；2 PLANNED 闭合 |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-19 | 初稿 |
