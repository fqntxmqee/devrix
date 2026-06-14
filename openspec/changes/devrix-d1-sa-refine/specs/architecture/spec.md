# Architecture Layering — Delta（切法 A 双轨）

**Change ID:** devrix-d1-sa-refine
**Demand ID:** DM-20260614-006
**Affects:** `openspec/specs/architecture/layering.md` — D1 域

---

## ADDED

### Requirement: D1 价值流 Scenario 表（canonical — 切法 A）

D1 根本目标驱动的 Scenario 注册。**SoT 为本表（S13–S18）**。

<!-- 验证: AC1 -->

#### Scenario: 每个价值流 S 有用户可理解目标

- GIVEN layering.md D1 价值流表
- WHEN 非技术人员阅读「用户目标」列
- THEN 可理解且不引用 Go 包名

| Module ID | Scenario | 用户目标 | Status |
|-----------|----------|----------|--------|
| D1-S13 | CaptureUserIntent | 我的指令一定进系统、查得到、能接着聊 | IMPLEMENTED |
| D1-S14 | PresentThinking | 我能看到它在想什么 | IMPLEMENTED |
| D1-S15 | PresentTaskProgress | 我能看到它在做什么任务 | IMPLEMENTED |
| D1-S16 | DeliverConclusion | 我能拿到针对我指令的总结 | IMPLEMENTED |
| D1-S17 | ConnectChannel | 换 IM 平台，三类信息结构一致 | IMPLEMENTED |
| D1-S18 | GuaranteeDelivery | 弱网也不丢结论和错误 | IMPLEMENTED |

Revision: **3.4.0** — DM-20260614-006 切法 A

---

### Requirement: Legacy Module Index（FROZEN — D1-S1–S12）

旧 Scenario 编号保留为 **module 追溯索引**，**不得**赋予新用户目标语义。

#### Scenario: 旧 T 注释仍指向 Legacy S

- GIVEN 测试注释 `// T: D1-S9-A01-T05`
- WHEN 查阅 t-registry Canonical 列
- THEN 映射到 S18 GuaranteeDelivery
- AND Legacy S9 仍可在 Legacy 表查到

---

## MODIFIED

### Requirement: Directory Structure Mapping 注释

包目录映射为实现注释，**不等同** canonical Scenario：

```
communication/
  gateway/      → 主要服务 S13, S18
  adapters/     → S13, S14–S17 Encode F
  eventbus/     → S18
  renderers/    → S14–S16 Encode F
  milestone/    → S15 F（Legacy S5 DEPRECATED）
  core/         → Domain Kernel（非 S）
```

---

## REMOVED

(None)

---

## DEPRECATED

### Requirement: 将 D1-S1–S12 作为 Scenario SoT

自 v3.4.0 起，价值流 SoT 迁移至 D1-S13–S18；S1–S12 仅 Legacy Module Index。
