# Design: MUPS Go-struct-driven I/O contract (M1 Observe)

**Change ID:** `mups-go-struct-driven`  
**Demand:** DM-20260705-003
**Status:** S3_Design
**Template:** `../../docs/methodology/detail-design-framework.md`（六段式 lite-mode）

## 1. 架构目标

### ① 业务目标

- **消除三处描述漂移**：Observe/Plan 节点 LLM I/O schema 当前由 (a) Go struct 字段顺序、(b) `FrameSpec.Fields []TagName` 数组、(c) 手工 `fields := map[TagName]any{...}` 拼接三处分别定义；任一不一致即 silent drift。
- **5 节点 go-struct 化总图**：本 change 实现 M1（kernel + Observe 迁移），M2-M5 写入 follow-on 计划。
- **0 行为变化（M1）**：golden snapshot 与现有 E2E 测试全 PASS。

### ② 技术目标（量化指标）

| 指标 | 目标值 |
|------|--------|
| `buildLLMObservationUserPrompt` 函数体行数 | ≤ 5（含签名） |
| Observe struct 字段数 == FrameSpec 字段数 == i18n 翻译条目数 | 9 == 9 == 9（一致校验） |
| 新增 tag 工作量 | 1 处改动（struct 字段 + pt tag） + `go test` 提示 |
| 反射开销（每轮 user prompt 构造） | < 50 μs |
| 测试覆盖 | 5 L5 + golden snapshot + 0 行为变化 E2E |

### ③ 约束条件

- Go 1.22+（`reflect.StructTag` 完整支持）
- Pure types + 不可变 builder（`With*` 模式）
- 不复活 ChannelRouter 4 个 channel 文件（v1 死代码）
- 不修改 SpawnPolicy 6 枚举
- M1 阶段 0 行为变化；M3 阶段是行为增量（最后做）

## 2. 架构原则

### ① 设计原则

1. **单一定义点（Single Source of Truth）**：Go struct 是 LLM I/O schema 的唯一权威；`FrameSpec` 数组由反射从 struct 写入；i18n 翻译条目由反射校验一致性。
2. **反射注册，热路径零反射**：`init()` 期一次反射写哈希表；user prompt 构造时查表 + 字段值反射读（每轮 1 次）。
3. **失败即 panic（设计 bug）**：pt tag 缺失 / 拼错 / plane 错误 / i18n 缺翻译任一情形 → init panic；**不**做运行时 silent skip（这正是要消除的漂移）。
4. **0 行为变化优先（refactor before increment）**：M1-M2-M4-M5 是 pure refactor；M3 是行为增量（最后做、独立 PR、独立 S5 验收）。
5. **语义即 i18n**：data/control 平面标注 + when-use 翻译 = struct field 的 `pt:"...,<plane>"` + i18n `prompttags_semantics_{zh,en}.go`；二者必须同时存在。

### ② 命名规范

- **struct tag**：`pt:"<tag_name>,<plane>,<flags>"`，例 `pt:"work_item_id,control"` / `pt:"directive,data,omit_empty"`
- **plane**：`data` | `control`（与 `FrameFieldPlane` 一致）
- **flags**：`omit_empty`（空字符串/空 slice 跳过）、`omit_zero`（0/0.0/false 跳过）、`join=<sep>`（slice 拼接，默认 `,`）
- **kernel 函数**：`MustRegisterFrame[T]` / `BuildLineFrameFromStruct` / `DocBlockFromStruct[T]` / `ParseFrameFieldTag`

### ③ 代码风格

- 函数 < 50 行，文件 < 800 行
- `structbind.go` 目标 < 150 行（含注释 + init panic 诊断）
- 不可变：`BuildLineFrameFromStruct` 返回新字符串；不改输入
- 错误码：`ErrStructBindPanic` 单一 sentinel（init panic 用）

## 3. 业务流程

### ① 核心用例时序

```
SessionOrchestrator.ObserveNode
  └─> itemPipeline.runRound
        ├─ [1] buildObserveSignalInput(item, ...)        # Go 编排
        │      └─ 扁平化 ScopeContract → ScopeGoal / ScopeOpenQuestions
        │      └─ 注入 InboundSignalLines / PriorObservationIDs
        │      └─ 计算 IncrementalOnly = (len(PriorObservationIDs) > 0)
        │
        └─ [2] LLMObservationProposer.ProposeObservations(in)
              ├─ [2.1] MaterializeForMUPS(ctx, req)        # D2 ContextEngine
              ├─ [2.2] buildLLMObservationUserPrompt(in)   # 本 change 改造点
              │      └─ prompttags.BuildLineFrameFromStruct("observe_user", in)  # 一行
              │      └─ i18n.RenderFrameFieldGuideForFields(...)                  # 一行
              ├─ [2.3] LLMInvoker.InvokeStream(...)         # D3 LLMGateway
              └─ [2.4] parseObservationProposalsJSON(raw)   # prompttags.ParseWholeBody[T]
```

### ② 异常补偿

| 异常 | 触发 | 行为 |
|------|------|------|
| `pt` tag 缺失 | init() 反射 | **panic**（设计 bug，0 容忍） |
| `pt` tag 拼写不在 TagName 常量 | init() 反射 | **panic**（编译期未校验的 tag） |
| i18n 翻译缺失 | init() 反射 | **panic**（i18n 与代码必须同步） |
| plane 错误（`data`/`control` 之外） | init() 反射 | **panic** |
| struct 字段数 != FrameSpec 字段数 | init() 反射 | **panic**（registry 漂移） |
| LLM 输出 parse 失败 | 运行期 | 沿用 DM-20260705-002 `prior_parse_reject` 反馈注入下轮 |
| ScopeContract 为 nil | 运行期 | ScopeGoal / ScopeOpenQuestions 字段为空，omit_empty 跳过 |

### ③ 分支处理决策树

```
init() 反射 register ObserveSignalInput
  ├─ 解析 pt tag
  │   ├─ 缺 tag → panic
  │   ├─ tag_name 不在 constants → panic
  │   └─ plane 不在 {data, control} → panic
  ├─ 校验 struct 字段数 == FrameSpec.Fields 长度
  │   └─ 不等 → panic
  ├─ 校验每个 field 对应 i18n 翻译条目
  │   └─ 缺翻译 → panic
  └─ 写入 LineFrameRegistry（热路径查表）
```

## 4. 领域模型

### ① 聚合根

| 聚合根 | 不可变性 | 职责 |
|--------|----------|------|
| `ObserveSignalInput`（值对象） | 不可变 + `With*` builder | Observe user frame 输入 |
| `FrameSpec`（registry 实体） | init() 一次写入 | user prompt 字段顺序权威 |
| `i18n.FrameFieldGuide`（registry 实体） | init() 一次写入 | when-use 翻译权威 |
| `ptTag`（解析中间值） | 不可变 | `pt:"..."` 解析结果 |

### ② 限界上下文（包边界）

```
internal/shared/prompttags/                  # L2 shared kernel
  ├── structbind.go         (本 change 新增)  # 反射注册 + 序列化 + DocBlock
  ├── linefield.go          (不动)            # BuildAnnotatedLineFrame 保留（向后兼容）
  ├── registry.go           (扩展)            # LineFrameRegistry 仍可手写（M1 不删旧路径）
  └── semantics.go          (扩展)            # HasFrameFieldGuide 校验函数（新增）

internal/layers/orchestration/sessionorchestrator/   # D7-S5 consumer
  ├── observation_proposer.go  (修改)        # ObserveSignalInput 加 pt tag + MustRegisterFrame init
  └── llm_observation_proposer.go (修改)     # buildLLMObservationUserPrompt 1 行化
```

### ③ 领域事件

- `prompttags.FrameRegistered(frameName, fieldCount)` — init() 期一次性 span event（debug 用）
- `prompttags.FrameBuilt(frameName, runeCount)` — user prompt 构造时 span event（性能监控，可选）

### ④ 跨域消费模型

| 消费者 | 当前 | 改造后 |
|--------|------|--------|
| D2 ContextEngine `MaterializeForMUPS` | 消费 `req`（不依赖 user frame 结构） | 不变 |
| D3 LLMGateway `InvokeStream` | 消费 `systemPrompt` + `userPrompt` string | 不变（user prompt 由 D7 构造） |
| D7-S5 Plan node（M2） | 同样 `BuildAnnotatedLineFrame("plan_user", ...)` 手工 map | M2 复用 structbind kernel（`BuildLineFrameFromStruct("plan_user", planIn)`） |
| D2 i18n `RenderFrameFieldGuideForFields` | 消费 `FrameName + map[TagName]any` | 改造为 `RenderFrameFieldGuideForStruct[T](in)`（M2 复用） |

## 5. 核心链路图

### ① 端到端 Observe 链路（M1 改造点）

```
itemPipeline.runRound
  │
  ├─ [buildObserveSignalInput]  observation_proposer.go
  │   输入: item, lastRound, scopeContract
  │   输出: ObserveSignalInput{...9 fields...}
  │         │                       │
  │         │                       │ 反射 init 校验
  │         │                       ▼
  │         │             MustRegisterFrame[ObserveSignalInput]()
  │         │                  ├─ 反射 9 fields
  │         │                  ├─ 校验 pt tag × 9
  │         │                  ├─ 校验 FrameSpec.Fields 长度 == 9
  │         │                  └─ 校验 i18n 翻译条目 × 9
  │         │
  │         ▼
  ├─ [LLMObservationProposer.ProposeObservations]  llm_observation_proposer.go
  │   │
  │   ├─ [buildLLMObservationUserPrompt]  ◄──── 本 change 主改造点
  │   │   旧: 35 行 fields := map[TagName]any{...} + BuildAnnotatedLineFrame
  │   │   新: 1 行 prompttags.BuildLineFrameFromStruct("observe_user", in)
  │   │        + 1 行 i18n.RenderFrameFieldGuideForFields(...)
  │   │
  │   ├─ [MaterializeForMUPS]  D2 ContextEngine
  │   ├─ [LLMInvoker.InvokeStream]  D3 LLMGateway
  │   └─ [parseObservationProposalsJSON]  prompttags.ParseWholeBody[T]
  │
  ▼
ObservationProposal[] → ValidateObservationProposals → 持久化
```

### ② 时序标注（SLA / P99）

| 节点 | P99 目标 | 实测（M1 验收） |
|------|----------|-----------------|
| `buildObserveSignalInput` | < 100 μs | TBD（基准测试） |
| `BuildLineFrameFromStruct`（反射） | < 50 μs | TBD |
| `RenderFrameFieldGuideForFields`（i18n） | < 200 μs | TBD（与改造前一致） |
| `parseObservationProposalsJSON` | < 100 μs | TBD（与改造前一致） |
| LLM 调用整体 | 秒级 | 不变 |

### ③ 单点风险与缓解

| 风险点 | 影响 | 缓解 |
|--------|------|------|
| `init()` 反射 panic 阻断进程启动 | High | S4 实现期多组测试 fixture 覆盖所有字段；S5 跑 `go test -race` 全包验证 |
| 反射性能瓶颈 | Low | 热路径零反射；user prompt 构造 1 次/轮 vs LLM 秒级 |
| golden snapshot 漂移 | Med | M1 0 行为变化承诺；snapshot 文件入库；任何 diff 阻断 PR |
| `MustRegisterFrame` 与 `init()` 顺序 | Low | Go init 顺序：先 `prompttags`（kernel 注册 FrameSpec），后 `sessionorchestrator`（register ObserveSignalInput） |

## 6. 接口/API 设计

### ① kernel 公共 API（`prompttags/structbind.go`）

```go
// 注册 Go struct 到 user prompt frame registry（init 一次）
//   - 反射解析 pt:"<tag_name>,<plane>,<flags>" struct tag
//   - 校验 FrameSpec.Fields 长度 == 字段数
//   - 校验每个 tag_name 在 TagName 常量集中
//   - 校验每个 tag 对应 i18n 翻译条目存在
//   - 失败一律 panic（设计 bug，0 容忍）
func MustRegisterFrame[T any](frameName FrameName) *RegisteredFrame[T]

// 反射序列化：struct → user prompt line frame
//   行为等价 BuildAnnotatedLineFrame，但无需手工 map
func BuildLineFrameFromStruct(frameName FrameName, s any) string

// 反射生成 schema doc（替代手写 DocBlockObserveSchema/PlanSchema）
func DocBlockFromStruct[T any]() string

// 解析 pt:"<tag_name>,<plane>,<flags>" → ptTag
func parseFrameFieldTag(tag reflect.StructTag) (ptTag, error)
```

### ② ptTag 解析规则

```
pt:"<tag_name>,<plane>[,omit_empty][,omit_zero][,join=<sep>]"

例:
  pt:"work_item_id,control"                  → tag=work_item_id, plane=control
  pt:"directive,data"                        → tag=directive, plane=data
  pt:"prior_mean,control,omit_zero"          → tag=prior_mean, plane=control, omit_zero=true
  pt:"prior_observation_ids,control,omit_empty,join=,"  → join=","
  pt:"scope_open_question,data,omit_empty"   → 数组逐行（不 join）
```

### ③ ObserveSignalInput 改造后结构

```go
// ObserveSignalInput 携带结构化信号给 LLM Observe 提案。
// pt struct tag 是 user frame schema 的单一定义点；反射注册到 LineFrameRegistry。
type ObserveSignalInput struct {
    // 不进入 user prompt（仅用于 D7 编排路由）
    SessionID string `pt:"-"`

    // 1. work_item_id (control)
    WorkItemID string `pt:"work_item_id,control"`

    // 2. directive (data)
    Directive string `pt:"directive,data"`

    // 3. prior_parse_reject (control, 来自 DM-20260705-002)
    PriorParseReject string `pt:"prior_parse_reject,control,omit_empty"`

    // 4. prior_mean (control, omit_zero)
    PriorMean float64 `pt:"prior_mean,control,omit_zero"`

    // 5. scope_goal (data, omit_empty, 由 buildObserveSignalInput 从 ScopeContract 扁平化)
    ScopeGoal string `pt:"scope_goal,data,omit_empty"`

    // 6. scope_open_question (data, omit_empty, 多行)
    ScopeOpenQuestions []string `pt:"scope_open_question,data,omit_empty"`

    // 7. signal (data, omit_empty, 多行)
    InboundSignalLines []string `pt:"signal,data,omit_empty"`

    // 8. prior_observation_ids (control, omit_empty, join=",")
    PriorObservationIDs []string `pt:"prior_observation_ids,control,omit_empty"`

    // 9. incremental_only (control, omit_zero, 由 buildObserveSignalInput 计算)
    IncrementalOnly bool `pt:"incremental_only,control,omit_zero"`
}

// 反射注册（init 一次）
func init() {
    prompttags.MustRegisterFrame[ObserveSignalInput](prompttags.FrameObserveUser)
}
```

### ④ buildLLMObservationUserPrompt 改造后（1 行）

```go
func buildLLMObservationUserPrompt(in ObserveSignalInput, loc i18n.Locale) string {
    frame := prompttags.BuildLineFrameFromStruct(prompttags.FrameObserveUser, &in)
    guide := i18n.RenderFrameFieldGuideForFields(prompttags.FrameObserveUser, loc, nil)
    if guide == "" {
        return frame
    }
    return guide + "\n\n" + frame
}
```

### ⑤ 错误码 + TraceID

- `prompttags.ErrStructBindPanic` — init panic 唯一 sentinel
- TraceID 沿用 `sessionSpan` 6 prior attributes（DM-20260625-001 引入）

### ⑥ 幂等 + 版本演进

- `MustRegisterFrame[T]()` 幂等（重复调用同类型 panic）
- struct 字段新增 → 必须同步 `ObserveUserFrame.Fields` + i18n → init panic 拦截
- struct 字段删除 → 同上
- struct 字段重命名 → 同上 + 测试 snapshot diff
- 向后兼容：`BuildAnnotatedLineFrame` 旧 API 保留（M2 移除）

## 7. 5 节点重构总图（M2-M5 follow-on）

| 阶段 | 范围 | 行为变化 | 工期估算 | follow-on change-id |
|------|------|----------|----------|---------------------|
| **M1（本次）** | kernel + Observe 迁移 | 0 | 2 周 | — |
| M2 | Plan 节点独立化 + go-struct 化 | 0 | 1 周 | `mups-plan-structbind` |
| M3 | Strategy 抽象注入 WorkItemExecContext | **+** | 3 周 | `d7-mups-strategy-injection` |
| M4 | Verify 决策表化 | 0 | 1 周 | `mups-verify-table-driven` |
| M5 | SpawnDecision 3 子决策代数化 | 0 | 2 周 | `d7-spawn-decision-algebra` |

**顺序理由**：M1 铺 kernel → M2 验证 kernel 复用 → M4/M5 并行（0 行为变化局部表驱动化）→ M3 最后做（行为增量，恢复 ChannelRouter 死掉的 4 PlanKind 路由）。

M2-M5 详细 design 在各自 follow-on change 写。本 change 在 S5 同步生成 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` 作为入口索引。

## 8. S3-Gate 自检

- [x] 六段式完整性（lite-mode 6 段均非空）
- [x] 重大决策已记录（§2 方案 A vs B vs C）
- [x] Gherkin Scenario 在 `specs/{shared,d7-orchestration}/spec.md`
- [x] T 层测试点（5 L5 + 9 D7-S5-A99-T + 5 shared-A99-T）已规划
- [x] Draft PR 待创建（DM-20260705-003 S4 实现开始时）
