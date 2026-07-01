# Implementation Tasks: D7 S 层归一化

**Change ID:** `devrix-d7-s-layer-normalization`  
**Demand ID:** DM-20260701-002

---

## Phase P0 — OpenSpec Demand

- [x] 1.1 创建 `.openspec.yaml`
- [x] 1.2 创建 `demand.md`
- [x] 1.3 创建 `proposal.md`
- [x] 1.4 创建 `design.md`
- [x] 1.5 创建 `tasks.md`
- [x] 1.6 创建 D7 delta spec

**T:** D7-SN-T01

---

## Phase P1 — Spec / Registry Normalization

- [x] 2.1 `spec.md` current canonical S 收敛为 S1-S6
- [x] 2.2 `a-registry.md` 增加 current canonical mapping，S7-S14/S20/S21 转 historical/contract
- [x] 2.3 `f-registry.md` 增加 current path correction note
- [x] 2.4 `t-registry.md` 登记 D7-SN-T01~T06
- [x] 2.5 `layering.md` / `code-layout.md` 同步 D7 S6 governance overlay

**T:** D7-SN-T02, D7-SN-T03

---

## Phase P2 — Compat Shim / Guard

- [x] 3.1 retired ingress 注释澄清
- [x] 3.2 compat shim 注释澄清
- [x] 3.3 architecture guard 防止 retired ingress 文件回归
- [x] 3.4 architecture guard 防止 current registry 重引入 S7+ canonical

**T:** D7-SN-T04

---

## Phase P3 — Feedback Mechanism Closure

- [x] 4.1 `StrategicPlanReject` 写入 round / next prompt feedback
- [x] 4.2 增加 StrategicPlanReject 单元测试
- [x] 4.3 parent uncertainty reevaluate 使用 child-stats-driven round signal
- [x] 4.4 增加 child-stats uncertainty 单元测试

**T:** D7-SN-T05, D7-SN-T06

---

## Phase P4 — Acceptance / Archive

- [x] 5.1 运行相关 Go 单测
- [x] 5.2 ReadLints 检查编辑文件
- [x] 5.3 创建 acceptance-report.md
- [x] 5.4 更新 `openspec/demand-archive-index.md`
- [x] 5.5 归档 change 到 `openspec/archive/2026-07-01-devrix-d7-s-layer-normalization/`

---

## Completion Checklist

- [x] Demand AC 全部可勾选
- [x] D7 canonical S 只有 S1-S6
- [x] S7-S14/S20/S21 只作为 historical/contract mapping
- [x] 相关 tests PASS
- [x] OpenSpec 归档完成
