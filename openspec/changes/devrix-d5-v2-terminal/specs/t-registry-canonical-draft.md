# T-Registry Canonical 列校正草案（v3.2.0）

**Change:** devrix-d5-v2-terminal  
**Base:** `openspec/specs/d5-observability/t-registry.md` v3.1.0

> S7 归档时合并入主 `t-registry.md`。**T ID 字符串不变。**

---

## 新增列（表头）

在现有 `canonical_s` 列旁增 **`canonical_a`** 列（可选，与 `canonical_s` 同表）。

---

## 校正行

| T ID | canonical_s（新） | canonical_a（新） | 说明 |
|------|-------------------|-------------------|------|
| D5-S23-A06-T01 | **S0** | **A03** | SessionBridge → Facade |
| D5-S23-A08-T01 | **S21** | **A14** | DebugFilter → Instrument |
| D5-S23-A08-T02 | **S21** | **A14** | 同上 |
| D5-S23-A03-T01 | S23 | **A10** | Doctor；T ID 保留 A03 |
| D5-S23-A03-T02 | S23 | **A10** | 同上 |
| D5-S23-A07-T01 | S23 | A07 | Tracker |
| D5-S23-A07-T02 | S23 | A07 | Tracker linter |
| D5-S23-A09-T01 | S23 | A09 | FaultInject |
| D5-S23-A09-T02 | S23 | A09 | FaultInject prod stub |

其余行：`canonical_s` 按 Legacy→Canonical 映射表（S1→S21, S4→S22, S5/S8→S23, S7/S9→S24）已存在于 v3.1.0，复核即可。

---

## PLANNED → IMPLEMENTED（S4 验收）

| T ID | 闭合条件 |
|------|----------|
| D5-S21-A05-T01 | 补 span 传播单测或引用 `obs_trace_propagation_test` |
| D5-S21-A05-T02 | 补 Counter 单测或 `meter_test` 覆盖 |
| D5-S23-A06-T02 | `HealthCheck()` 已含 coverage 摘要 → IMPLEMENTED |

---

## Statistics（终态目标）

| 总数 | IMPLEMENTED | PLANNED |
|------|-------------|---------|
| 41 | 41 | 0 |
