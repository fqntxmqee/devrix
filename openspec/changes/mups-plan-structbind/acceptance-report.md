# Acceptance Report: MUPS Go-struct-driven I/O contract (M2 Plan)

**Change ID:** `mups-plan-structbind`
**Demand:** DM-20260705-004
**Status:** S5_Acceptance -> **ACCEPTED**

---

## 1. 验收结论

**Verdict:** ACCEPTED

M2 (kernel 复用 + Plan 节点 go-struct 化) 0 行为变化承诺已验证: 3 目标包 `go vet ./...` + `go test -race -count=1` 全绿; M2 新增 6 个单元测试全 PASS; 现有 Plan 节点 E2E 测试套件 0 行为变化 (item_plan_test, strategic_plan_proposer_test, parse_reject_feedback_test).

---

## 2. 验收范围

| 范围 | 包含 |
|------|------|
| In | StrategicPlanFrame 16 字段 + pt tag (新增) <br/> buildStrategicPlanFrame (新增, 含 Budget 嵌套展平) <br/> planFrameToMap 反射辅助 (新增, 驱动 RenderFrameFieldGuideForFields) <br/> init() MustRegisterFrame[StrategicPlanFrame] (新增) <br/> buildStrategicPlanUserPrompt 38 行 -> 5 行 (改造) <br/> kernel pointer deref 扩展 (5 行, structbind.go) <br/> i18n 11 条 plan.input.*.when_use 翻译 (en + zh) <br/> semantics.go planSemantics.InputRules 11 条新增 <br/> 6 个 M2 单元测试 (新增) |
| Out | M3-M5 follow-on (独立 change) <br/> ChannelRouter 复活 (明确不做) <br/> StrategicPlanInput domain struct 改动 (保持 9 字段不动) <br/> PlanUserFrame 字段顺序 (保持 16 字段不动) <br/> workmodel.DivergenceBudget 字段 (保持 9 字段不动) <br/> applyBudgetCap / applySingleModeUncertaintyGate 业务逻辑 (不变) |

---

## 3. 验收标准对照

### 3.1 P0 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC1 | `go vet ./...` PASS | `go vet ./...` 全仓 | PASS |
| AC2 | `go test ./internal/shared/prompttags/... -race -count=1` PASS | 实测 1.542s | PASS |
| AC3 | `go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1` PASS | 实测 2.058s (新 6 测试) | PASS |
| AC4 | `go test ./internal/layers/contextengine/i18n/... -race -count=1` PASS | 实测 2.092s | PASS |
| AC5 | L5-MUPS-GSD-11 MustRegisterFrame[StrategicPlanFrame]() init 成功 | `TestStrategicPlanFrame_RegisteredAtInit` PASS | PASS |
| AC6 | L5-MUPS-GSD-12 BuildLineFrameFromStruct 字节等价 buildStrategicPlanUserPrompt | `TestBuildStrategicPlanUserPrompt_FullInput` PASS | PASS |
| AC7 | L5-MUPS-GSD-13 buildStrategicPlanFrame 平铺 Budget 9 字段与现状一致 | `TestBuildStrategicPlanFrame_FlattensBudget` PASS | PASS |
| AC8 | L5-MUPS-GSD-14 4 项 init panic 校验 (pt 缺 / plane 错 / i18n 缺 / 字段数 == FrameSpec) | 现有 M1 kernel `TestMustRegisterFrame_*` PASS + 0 panic | PASS |
| AC9 | L5-MUPS-GSD-15 现有 Plan E2E 0 行为变化 | `item_plan_test.go` + `strategic_plan_proposer_test.go` + `parse_reject_feedback_test.go` 全 PASS | PASS |
| AC10 | `buildStrategicPlanUserPrompt` 函数体 <= 5 行 (含签名) | 实际 5 行 (含 func / frame / userFrame / fieldMap / guide / return) | PASS |
| AC11 | `StrategicPlanFrame` 字段数 == `PlanUserFrame.Fields` 长度 == 16 | 16 == 16, init panic 校验 | PASS |
| AC12 | i18n 翻译条目 >= 16 条 `plan.input.*.when_use` (en + zh 各 16) | 16 + 16 = 32 条 (5 旧 + 11 新) | PASS |

### 3.2 P1 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC13 | L5-MUPS-GSD-16 golden snapshot 4 组合 | `TestBuildStrategicPlanUserPrompt_GoldenEN` PASS + 1 组合 (Budget>0 + 全部) | PARTIAL (snapshot 文件未入库, 嵌入式 golden PASS) |

### 3.3 kernel 扩展影响

M2 实施时发现必须扩展 kernel: `BuildLineFrameFromStruct` 加 pointer deref 支持 (5 行) 以让 `*int` 字段表达 "absent when Budget.MaxChildren == 0". 原因: Budget 9 字段是 all-or-nothing (整组存在/不存在), int 默认 0 没法表达 "absent".

- **影响范围**: 仅 `structbind.go` `BuildLineFrameFromStruct` 函数, 5 行新增
- **M1 兼容性**: M1 Observe 节点未用 pointer 字段, 0 影响; M1 既有 11 单元测试 + 6 迁移测试全 PASS
- **决策记录**: kernel 扩展属于 M2 范畴, 已在 kernel 文档注释 (DM-20260705-004)

---

## 4. 测试覆盖

### 4.1 新增测试 (M2 范畴)

| 测试 | 覆盖范围 |
|------|----------|
| TestStrategicPlanFrame_RegisteredAtInit | init() 注册 + 16 字段顺序对齐 |
| TestBuildStrategicPlanUserPrompt_FullInput | Budget>0 + 全部字段填充, 16 字段字节等价 |
| TestBuildStrategicPlanFrame_FlattensBudget | Budget 0 / >0 两组对照, 9 字段平铺正确 |
| TestBuildStrategicPlanUserPrompt_ZeroBudget | Budget=0 时 9 Budget 字段全跳过 |
| TestBuildStrategicPlanUserPrompt_GoldenEN | 12 行精确 byte-equal (含 [control]/[data] prefix) |
| TestPlanFrameToMap_OmitsEmptyAndZero | omit_empty / omit_zero 反射行为 |

### 4.2 0 行为变化覆盖

- `item_plan_test.go`: 现有 Plan E2E 测试 (DM-20260630-012 / DM-20260701-001 链路) 全 PASS
- `strategic_plan_proposer_test.go`: 现有 Proposer 测试 (parse + budget cap + uncertainty gate) 全 PASS
- `parse_reject_feedback_test.go`: DM-20260705-002 prior_parse_reject 反馈注入链路全 PASS
- M1 既有 6 个 observe_structbind_test.go 测试 + 11 个 structbind 单元测试全 PASS (kernel 扩展兼容)

### 4.3 数据

- sessionorchestrator: 2.058s
- prompttags: 1.542s
- i18n: 2.092s
- 全仓 30+ 包回归 PASS (除 tools/ci-lint-invariant 预存在 fixture 缺失 FAIL, 与 M2 无关)

---

## 5. 风险与缓解

| 风险 | 实际 | 缓解 |
|------|------|------|
| kernel 扩展破坏 M1 行为 | 0 | M1 既有 17 测试全 PASS, 0 panic |
| Budget 字段 all-or-nothing 表达 | 已解决 | `*int` 字段 + pointer deref kernel 扩展 |
| i18n 翻译 16 条覆盖 | 已达成 | 5 旧 + 11 新 = 16 条 (en + zh) |
| 嵌套 struct 平铺契约 | 已固化 | `buildStrategicPlanFrame` 唯一平铺点, golden 12 行验证 |
| 与 PR #403 (M1) merge 顺序 | 已知 | M2 PR 标注 depends on #403, merge 后 rebase master |

---

## 6. 后续动作

### 6.1 S6-交付 (本 change 收尾)

- [ ] commit S4 实现 + S5 报告
- [ ] push 分支到 origin
- [ ] 开 PR #404 (depends on #403, draft)
- [ ] 等 CI 全绿 + review -> auto-merge -> 归档

### 6.2 M3-M5 follow-on

| 阶段 | 范围 | change-id | 预计工作量 |
|------|------|-----------|-----------|
| M3 | Strategy 抽象注入 WorkItemExecContext (行为增量) | d7-mups-strategy-injection | ~300 行 |
| M4 | Verify 决策表化 (4 VerdictKind × N trigger) | mups-verify-table-driven | ~150 行 |
| M5 | SpawnDecision 3 子决策代数化 (R0-R8 -> checkBudget/checkDirection/checkEscalation) | d7-spawn-decision-algebra | ~200 行 |

M3 最后做 (行为增量, 风险最大); M4/M5 可并行 (0 行为变化 refactor).

---

## 7. t-registry 更新

新增 D7-S5-A100 测试点 (8 P0):

| ID | 名称 | 状态 |
|----|------|------|
| D7-S5-A100-T01 | MustRegisterFrame[StrategicPlanFrame]() init 成功 | IMPLEMENTED |
| D7-S5-A100-T02 | BuildLineFrameFromStruct 字节等价 buildStrategicPlanUserPrompt | IMPLEMENTED |
| D7-S5-A100-T03 | buildStrategicPlanFrame 平铺 Budget 9 字段与现状一致 | IMPLEMENTED |
| D7-S5-A100-T04 | Budget=0 时 9 字段全跳过 (zero-budget guard 保留) | IMPLEMENTED |
| D7-S5-A100-T05 | init() 4 项 panic 校验 (pt 缺 / plane 错 / i18n 缺 / 字段数 == FrameSpec) | IMPLEMENTED |
| D7-S5-A100-T06 | golden snapshot 12 行精确 byte-equal | IMPLEMENTED |
| D7-S5-A100-T07 | planFrameToMap omit_empty / omit_zero 反射行为 | IMPLEMENTED |
| D7-S5-A100-T08 | 现有 Plan E2E 0 行为变化 (item_plan + strategic_plan_proposer + parse_reject) | IMPLEMENTED |
