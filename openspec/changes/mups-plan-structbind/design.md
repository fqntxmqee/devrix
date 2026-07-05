# Design: MUPS Go-struct-driven I/O contract (M2 Plan)

**Change ID:** `mups-plan-structbind`  
**Demand:** DM-20260705-004
**Status:** S3_Design
**Template:** ../../docs/methodology/detail-design-framework.md (六段式 lite-mode)

## 1. 架构目标

### 1 业务目标

- 消除 Plan 节点 LLM I/O schema 三处描述漂移 (struct / FrameSpec / 手工 map)。
- 复用 M1 kernel 零代码增量, 验证"kernel 一次编写, 5 节点复用"的可扩展性。
- 0 行为变化 (M2 阶段): golden snapshot + E2E 全 PASS。

### 2 技术目标 (量化)

| 指标 | 目标值 |
|------|--------|
| buildStrategicPlanUserPrompt 函数体行数 | <= 5 (含签名) |
| StrategicPlanFrame 字段数 == FrameSpec 字段数 == i18n 翻译条目数 | 16 == 16 == 16 |
| kernel (structbind.go / linefield.go / semantics.go) 改动行数 | 0 |
| 反射开销 (每轮 user prompt 构造) | < 50 us (与 M1 对齐) |
| 测试覆盖 | 5 L5 + golden snapshot + 0 行为变化 E2E |

### 3 约束

- Go 1.22+
- Pure types + 不可变 builder
- 不修改 M1 kernel (PR #403)
- 不修改 workmodel.DivergenceBudget 字段
- 不复活 ChannelRouter 4 文件
- M2 阶段 0 行为变化; M3 阶段是行为增量 (最后做)

## 2. 架构原则

### 1 设计原则

1. **单一定义点 (Single Source of Truth)**: `StrategicPlanFrame` 是 Plan user frame 唯一权威; `FrameSpec` 数组由反射从 struct 写入; i18n 翻译条目由反射校验一致性。
2. **kernel 复用, 热路径零反射**: M1 kernel 已落地; M2 仅消费。
3. **失败即 panic (设计 bug)**: pt tag 缺失 / 拼错 / plane 错误 / i18n 缺翻译任一 → init panic; 0 容忍 silent skip。
4. **0 行为变化优先 (refactor before increment)**: M1-M2-M4-M5 pure refactor; M3 行为增量。
5. **嵌套平铺契约**: 嵌套 struct (Budget) 的展平是 D7 编排域职责, 不污染 shared kernel。

### 2 命名规范

- **struct tag**: `pt:"<tag_name>,<plane>,<flags>"`, 例 `pt:"work_item_id,control"` / `pt:"depth,control,omit_zero"`
- **plane**: `data` | `control` (与 `FrameFieldPlane` 一致)
- **flags**: `omit_empty` (空字符串/空 slice 跳过), `omit_zero` (0/0.0/false 跳过)
- **类型**: `StrategicPlanFrame` (LLM 视图, 16 字段) vs `StrategicPlanInput` (domain 概念, 9 字段含 Budget 嵌套)

### 3 代码风格

- 函数 < 50 行, 文件 < 800 行
- 不可变: `BuildLineFrameFromStruct` 返回新字符串
- 错误码: `ErrStructBindPanic` 单一 sentinel (init panic 用, 复用 M1)

## 3. 业务流程

### 1 核心用例时序

```
SessionOrchestrator.PlanNode
  └─> itemPipeline.runRound
        ├─ [1] buildStrategicPlanFrame(in StrategicPlanInput) StrategicPlanFrame
        │      ├─ Budget.MaxChildren > 0 守卫保留
        │      ├─ Budget 9 字段平铺 (Depth, MaxDepth, ..., MaxIters)
        │      ├─ 条件字段: ObservationIDs/Summary/ParentScopeIn/UncertaintyMean/PriorParseReject
        │      └─ omitempty / omit_zero 标志驱动跳过
        │
        └─ [2] LLMStrategicPlanProposer.ProposeStrategicPlan(frame)
              ├─ [2.1] MaterializeForMUPS(ctx, req)        # D2 ContextEngine
              ├─ [2.2] buildStrategicPlanUserPrompt(frame) # 本 change 改造点
              │      └─ prompttags.BuildLineFrameFromStruct("plan_user", frame)  # 一行
              │      └─ i18n.RenderFrameFieldGuideForFields(...)                  # 一行
              ├─ [2.3] LLMInvoker.InvokeStream(...)         # D3 LLMGateway
              └─ [2.4] parseStrategicPlanJSON(raw)           # prompttags.ParseWholeBody[T]
              └─ [2.5] applyBudgetCap(prop, in.Budget)      # 业务逻辑, 不变
              └─ [2.6] applySingleModeUncertaintyGate(prop, in) # 业务逻辑, 不变
```

### 2 异常补偿

| 异常 | 触发 | 行为 |
|------|------|------|
| pt tag 缺失 | init() 反射 | panic (设计 bug) |
| pt tag 拼写不在 TagName 常量 | init() 反射 | panic |
| i18n 翻译缺失 | init() 反射 | panic |
| plane 错误 (data/control 之外) | init() 反射 | panic |
| struct 字段数 != FrameSpec 字段数 | init() 反射 | panic |
| LLM 输出 parse 失败 | 运行期 | 沿用 DM-20260705-002 prior_parse_reject 反馈注入下轮 |
| Budget.MaxChildren == 0 | 运行期 | Budget 9 字段全 omit, 仅输出 WorkItemID/Directive/PriorParseReject 等非 Budget 字段 |

### 3 分支处理决策树

```
init() 反射 register StrategicPlanFrame
  ├─ 解析 pt tag × 16
  │   ├─ 缺 tag → panic
  │   ├─ tag_name 不在 constants → panic
  │   └─ plane 不在 {data, control} → panic
  ├─ 校验 struct 字段数 == FrameSpec.Fields 长度 (16 == 16)
  │   └─ 不等 → panic
  ├─ 校验每个 field 对应 i18n 翻译条目
  │   └─ 缺翻译 → panic
  └─ 写入 LineFrameRegistry (热路径查表)
```

## 4. 领域模型

### 1 聚合根

| 聚合根 | 不可变性 | 职责 |
|--------|----------|------|
| StrategicPlanInput (值对象, domain) | 不可变 | Plan 阶段 domain 输入 (9 字段含 Budget 嵌套) |
| StrategicPlanFrame (值对象, LLM 视图) | 不可变 | Plan user frame 输入 (16 字段平铺) |
| workmodel.DivergenceBudget (值对象) | 不可变 | 发散预算 (9 字段, 嵌套于 StrategicPlanInput) |
| FrameSpec (registry 实体) | init() 一次写入 | user prompt 字段顺序权威 |
| i18n.FrameFieldGuide (registry 实体) | init() 一次写入 | when-use 翻译权威 |
| buildStrategicPlanFrame (转换函数) | 不可变 | domain → LLM 视图唯一转换点 |

### 2 限界上下文 (包边界)

```
internal/shared/prompttags/                  # L2 shared kernel (M1 已落地, 本 change 零代码)
  ├── structbind.go         (不动)            # 反射注册 + 序列化 + DocBlock (M1)
  ├── linefield.go          (不动)            # BuildAnnotatedLineFrame (向后兼容)
  ├── registry.go           (不动)            # LineFrameRegistry
  └── semantics.go          (不动)            # FrameFieldPlane / HasFrameFieldGuide

internal/layers/orchestration/sessionorchestrator/   # D7-S5 consumer (本 change 改造点)
  └── strategic_plan_proposer.go  (修改)
        ├─ StrategicPlanInput struct        (保持 9 字段, 不动)
        ├─ StrategicPlanFrame struct (NEW)   16 字段 + pt tag
        ├─ buildStrategicPlanFrame() (NEW)   Budget 嵌套展平
        ├─ buildStrategicPlanUserPrompt()    35+ 行 → 1 行 BuildLineFrameFromStruct
        └─ init() (NEW)                      MustRegisterFrame[StrategicPlanFrame]()

internal/layers/contextengine/i18n/          # i18n 翻译 (本 change 补 11 条 × 2)
  ├── prompttags_semantics_en.go  (+11 条 plan.input.*.when_use)
  └── prompttags_semantics_zh.go  (+11 条 plan.input.*.when_use)
```

### 3 领域事件

- prompttags.FrameRegistered("plan_user", 16) — init() 期一次性 span event
- prompttags.FrameBuilt("plan_user", runeCount) — user prompt 构造时 span event (性能监控, 可选)

### 4 跨域消费模型

| 消费者 | 当前 | 改造后 |
|--------|------|--------|
| D2 ContextEngine MaterializeForMUPS | 消费 req (不依赖 user frame 结构) | 不变 |
| D3 LLMGateway InvokeStream | 消费 systemPrompt + userPrompt string | 不变 (user prompt 由 D7 构造) |
| D7-S5 Observe node (M1) | BuildAnnotatedLineFrame 手工 map → 反射 | 已 M1 改造 |
| D7-S5 Plan node (本 change) | BuildAnnotatedLineFrame 手工 map → 反射 | 本 change 改造 |
| D7-S5 Execute/Verify/Learn | 散落手工 / magic number | M3-M5 follow-on |
| D2 i18n RenderFrameFieldGuideForFields | 消费 FrameName + map[TagName]any | 保持, 接受 map 等价 |

## 5. 核心链路图

### 1 端到端 Plan 链路 (M2 改造点)

```
itemPipeline.runRound
  │
  ├─ [buildStrategicPlanFrame]  strategic_plan_proposer.go (NEW)
  │   输入: StrategicPlanInput{9 fields, Budget 嵌套}
  │   输出: StrategicPlanFrame{16 fields, 平铺}
  │         │                       │
  │         │                       │ 反射 init 校验
  │         │                       ▼
  │         │             MustRegisterFrame[StrategicPlanFrame]("plan_user")
  │         │                  ├─ 反射 16 fields
  │         │                  ├─ 校验 pt tag × 16
  │         │                  ├─ 校验 FrameSpec.Fields 长度 == 16
  │         │                  └─ 校验 i18n 翻译条目 × 16
  │         │
  │         ▼
  ├─ [LLMStrategicPlanProposer.ProposeStrategicPlan]  strategic_plan_proposer.go
  │   │
  │   ├─ [buildStrategicPlanUserPrompt]  ◄──── 本 change 主改造点
  │   │   旧: 35+ 行 fields := map[TagName]any{...} + BuildAnnotatedLineFrame
  │   │   新: 1 行 prompttags.BuildLineFrameFromStruct("plan_user", frame)
  │   │        + 1 行 i18n.RenderFrameFieldGuideForFields(...)
  │   │
  │   ├─ [MaterializeForMUPS]  D2 ContextEngine
  │   ├─ [LLMInvoker.InvokeStream]  D3 LLMGateway
  │   ├─ [parseStrategicPlanJSON]  prompttags.ParseWholeBody[T]
  │   ├─ [applyBudgetCap]  (业务, 不变)
  │   └─ [applySingleModeUncertaintyGate]  (业务, 不变)
  │
  ▼
StrategicPlanProposal → 持久化
```

### 2 时序标注 (SLA / P99)

| 节点 | P99 目标 | 实测 (M2 验收) |
|------|----------|-----------------|
| buildStrategicPlanFrame | < 5 us (纯 struct copy) | TBD |
| BuildLineFrameFromStruct (反射) | < 50 us (与 M1 对齐) | TBD |
| RenderFrameFieldGuideForFields (i18n) | < 200 us | TBD |
| parseStrategicPlanJSON | < 100 us | TBD |
| LLM 调用整体 | 秒级 | 不变 |

### 3 单点风险与缓解

| 风险点 | 影响 | 缓解 |
|--------|------|------|
| init() 反射 panic 阻断进程启动 | High | S4 多组 fixture 覆盖; S5 跑 go test -race 全包 |
| Budget 字段平铺顺序漂移 | Med | buildStrategicPlanFrame 是唯一平铺点; golden snapshot 4 组合 |
| 反射性能瓶颈 | Low | 热路径零反射; user prompt 构造 1 次/轮 vs LLM 秒级 |
| golden snapshot 漂移 | Med | M2 0 行为变化承诺; snapshot 入库; diff 阻断 PR |
| 与 PR #403 merge 顺序 | Low | M2 PR 标注 depends on #403; merge 后 rebase |

## 6. 接口/API 设计

### 1 转换函数 (D7 域内, 非 kernel 公开 API)

```go
// buildStrategicPlanFrame converts StrategicPlanInput (domain) to
// StrategicPlanFrame (LLM view), flattening nested Budget struct.
// Budget.MaxChildren > 0 guard retained for 0 行为变化.
func buildStrategicPlanFrame(in StrategicPlanInput) StrategicPlanFrame {
    frame := StrategicPlanFrame{
        WorkItemID:       in.WorkItemID,
        Directive:        in.Directive,
        PriorParseReject: in.PriorParseReject,
    }
    if len(in.ObservationIDs) > 0 {
        frame.ObservationIDs = in.ObservationIDs
    }
    if s := strings.TrimSpace(in.ReportSummary); s != "" {
        frame.ObservationSummary = s
    }
    if in.Budget.MaxChildren > 0 {
        b := in.Budget
        frame.Depth = b.Depth
        frame.MaxDepth = b.MaxDepth
        frame.ExistingChildren = b.ExistingChildren
        frame.RemainingChildren = b.RemainingChildren()
        frame.MaxChildren = b.MaxChildren
        frame.DecomposeUsedToday = b.DecomposeUsedToday
        frame.RemainingDaily = b.RemainingDaily()
        frame.MaxDaily = b.MaxDaily
        frame.MaxIters = b.MaxIters
    }
    if len(in.ParentScopeIn) > 0 {
        frame.ParentScopeIn = in.ParentScopeIn
    }
    if in.UncertaintyMean > 0 {
        frame.UncertaintyMean = in.UncertaintyMean
    }
    return frame
}
```

### 2 StrategicPlanFrame struct (16 字段, 与 PlanUserFrame 1:1)

```go
type StrategicPlanFrame struct {
    // 7 字段直接映射 (WorkItemID/Directive/PriorParseReject/ObservationIDs/ObservationSummary/ParentScopeIn/UncertaintyMean)
    WorkItemID         string   `pt:"work_item_id,control"`
    Directive          string   `pt:"directive,data"`
    PriorParseReject   string   `pt:"prior_parse_reject,control,omit_empty"`
    ObservationIDs     []string `pt:"observation_ids,control,omit_empty"`
    ObservationSummary string   `pt:"observation_summary,data,omit_empty"`
    ParentScopeIn      []string `pt:"parent_scope_in,control,omit_empty"`
    UncertaintyMean    float64  `pt:"uncertainty_mean,control,omit_zero"`

    // 9 字段平铺自 Budget (Budget.MaxChildren > 0 守卫在转换函数)
    Depth              int      `pt:"depth,control,omit_zero"`
    MaxDepth           int      `pt:"max_depth,control,omit_zero"`
    ExistingChildren   int      `pt:"existing_children,control,omit_zero"`
    RemainingChildren  int      `pt:"remaining_children,control,omit_zero"`
    MaxChildren        int      `pt:"max_children,control,omit_zero"`
    DecomposeUsedToday int      `pt:"decompose_used_today,control,omit_zero"`
    RemainingDaily     int      `pt:"remaining_daily,control,omit_zero"`
    MaxDaily           int      `pt:"max_daily,control,omit_zero"`
    MaxIters           int      `pt:"max_iters,control,omit_zero"`
}

func init() {
    prompttags.MustRegisterFrame[StrategicPlanFrame](prompttags.FramePlanUser)
}
```

### 3 业务代码 1 行化

```go
// 旧: 35+ 行 fields := map[TagName]any{...} + BuildAnnotatedLineFrame
// 新: 1 行 BuildLineFrameFromStruct + 1 行 RenderFrameFieldGuideForFields
func buildStrategicPlanUserPrompt(in StrategicPlanInput, loc i18n.Locale) string {
    frame := buildStrategicPlanFrame(in)
    userFrame := prompttags.BuildLineFrameFromStruct(prompttags.FramePlanUser, frame)
    guide := i18n.RenderFrameFieldGuideForFields(prompttags.FramePlanUser, loc, frame)
    if guide == "" {
        return userFrame
    }
    return guide + "\n\n" + userFrame
}
```

### 4 i18n 翻译补 11 条 (en + zh)

```yaml
# English (prompttags_semantics_en.go)
plan.input.work_item_id.when_use: "Work item identifier (control)"
plan.input.observation_ids.when_use: "Prior observation IDs (control)"
plan.input.depth.when_use: "Current work item depth (control)"
plan.input.max_depth.when_use: "Maximum depth allowed (control)"
plan.input.existing_children.when_use: "Existing child count (control)"
plan.input.max_children.when_use: "Maximum children allowed (control)"
plan.input.decompose_used_today.when_use: "Decompose operations used today (control)"
plan.input.remaining_daily.when_use: "Remaining decompose budget today (control)"
plan.input.max_daily.when_use: "Daily decompose limit (control)"
plan.input.max_iters.when_use: "Max react iterations (control)"
plan.input.parent_scope_in.when_use: "Parent scope in-paths (control)"

# Chinese (prompttags_semantics_zh.go)
plan.input.work_item_id.when_use: "工作项 ID (control)"
plan.input.observation_ids.when_use: "上一轮 Obs IDs (control)"
plan.input.depth.when_use: "当前工作项深度 (control)"
plan.input.max_depth.when_use: "允许的最大深度 (control)"
plan.input.existing_children.when_use: "已存在子节点数 (control)"
plan.input.max_children.when_use: "允许的最大子节点数 (control)"
plan.input.decompose_used_today.when_use: "今日已用 decompose 次数 (control)"
plan.input.remaining_daily.when_use: "今日剩余 decompose 预算 (control)"
plan.input.max_daily.when_use: "每日 decompose 上限 (control)"
plan.input.max_iters.when_use: "最大 react 迭代次数 (control)"
plan.input.parent_scope_in.when_use: "父级 scope in 路径 (control)"
```

## 7. Follow-on 计划 (M3-M5)

| 阶段 | 范围 | 行为变化 | 工作量 | change-id |
|------|------|----------|--------|-----------|
| M3 | Strategy 抽象注入 WorkItemExecContext | 行为增量 | ~300 行 | d7-mups-strategy-injection |
| M4 | Verify 决策表化 (4 VerdictKind × N trigger) | 0 | ~150 行 | mups-verify-table-driven |
| M5 | SpawnDecision 3 子决策代数化 (R0-R8 → checkBudget/checkDirection/checkEscalation) | 0 | ~200 行 | d7-spawn-decision-algebra |

M2 完成后, 启动 M4 / M5 (可并行), M3 最后做。
