# PR-A1 Consensus Packet — Grammar Only (7 T, ~1.0 人天)

**Change ID:** `devrix-d7-multi-intent-observation-decompose` (DM-20260707-001)
**Branch:** `feat/devrix-d7-multi-intent-observation-decompose`
**S3-Gate:** Closed at commit `d9df289c` (S3-Gate 后事实校准 commit `0b1468e3`)
**PR-A1 Goal:** Grammar only — 不引入行为,只引入类型 + 校验器。后续 PR-A2 起叠加 AC contract + LLM IO + DAG executor。

---

## 1. PROBLEM (the precise thing PR-A1 must solve)

D7 MUPS v4 pipeline 当前把每个 directive 当作**单意图原子单元**。本 Change 解决多意图混合,但 7 PR 拆分下 PR-A1 只解决"语法底座",即:

1. **新类型可被 Go 编译**
2. **新类型可被 JSON 序列化**(`/dev/null` 互通即可,无下游消费者)
3. **新类型的运行时校验**(`Validate()`)覆盖 5 种错误路径
4. **既有代码不受影响**(0 行现有代码改动 = "Plan/SpawnPolicy/ExecuteRound 完全不动")

PR-A1 不解决:
- ❌ 不接 Runner(PR-B 才接 RunPlanDAG)
- ❌ 不接 Planner prompt(PR-A2/PR-F 才接)
- ❌ 不接 Execute(PR-B 才接)
- ❌ 不发任何 emit 事件(PR-C 才发)
- ❌ 不动 Learn(PR-E 才动)
- ❌ 不接 SpawnPolicy(已锁不动 — 方案 β)

## 2. CONSTRAINTS (硬约束)

| 约束 | 来源 | 校验方式 |
|------|------|---------|
| `SpawnPolicy` 完全不动 | `workmodel/pipeline_round.go:27-34`,3 值字符串枚举 | `git diff` 检查 workmodel/ 0 行变更 |
| `Plan` 当前 8 字段必须保留 | Phase 2 PR-B1 PP-1/2/3 契约 | 现存 `plan_test.go` 100% 通过 |
| PlanAcceptanceContractBuilder.Build 失败语义不能变 | 方案 β 兼容路径 | 不引入新 sentinel error code,复用 `PLAN_*` |
| 必须 100% 通过 22/22 orchestration 包 `go test -race` | Phase 5 等级测试覆盖 | CI 自动验证 |
| 必须 `go vet ./...` 无警告 | devrix CI 标准 | CI 自动验证 |
| 函数 < 50 行,文件 < 800 行 | devrix coding-style.md | self-review |
| 测试覆盖 ≥ 80% | devrix testing.md | `go test -cover` 验证 |
| 不可引入 panic 业务错误 | devrix coding-style.md | review |
| 不可 hardcode 字符串 magic | devrix naming policy | constants only |
| 必须用 SentinelError 模式,Code ORC_7xxx | 已锁定(`shared/errors/orchestration.go` NEW,CodeD7Xxx) | self-review |

## 3. SCOPE: 7 T 点的精确边界

### T01 — `IntentSegment` 类型(orchtypes/intent_segment.go)

```go
package orchtypes

type IntentKind string
const (
    IntentKindDeterministic IntentKind = "deterministic"
    IntentKindExplore        IntentKind = "explore"
    IntentKindCommit         IntentKind = "commit"
    IntentKindAnalyze        IntentKind = "analyze"
)

type IntentSegment struct {
    ID         string     `json:"id"`
    Text       string     `json:"text"`
    IntentKind IntentKind `json:"intent_kind"`
    Priority   int        `json:"priority"`        // [0, 100],default 50
    Confidence float64    `json:"confidence"`      // [0, 1],default 0.5
}
```

**测试**:JSON marshal/unmarshal 5 round-trip + 字段缺失返回 ErrIntentSegmentInvalid。

### T02 — `IntentSegmentSet` 容器(同文件)

```go
type IntentSegmentSet struct {
    Segments        []IntentSegment `json:"segments"`
    SourceDirective string          `json:"source_directive"`
    DetectedAt      time.Time       `json:"detected_at"`
}
```

**验证不变量**:`len(Segments) >= 1`(空 set 是错误状态,不是初始状态)。

### T03 — `Plan` 加 2 可选字段(plan/plan_struct.go)

```go
type Plan struct {
    // ... 现有 8 字段 ...
    IntentSegmentSet *orchtypes.IntentSegmentSet `json:"intent_segment_set,omitempty"`  // NEW
    DAG              *PlanDAG                    `json:"dag,omitempty"`                   // NEW
}

// 新 builder
func (p Plan) WithIntentSegmentSet(s *orchtypes.IntentSegmentSet) Plan  // immutable copy
func (p Plan) WithDAG(d *PlanDAG) Plan                                    // immutable copy
```

**关键不变性**:`Plan` 是 immutable value-object,加 2 字段是纯加,不动现有 builder。

### T04 — `Validate()` + 全错误类型(orchtypes/intent_segment.go)

```go
var (
    ErrIntentSegmentInvalidKind      = errors.WithCode("ORC_7010", ...)
    ErrIntentSegmentInvalidPriority  = errors.WithCode("ORC_7011", ...)
    ErrIntentSegmentInvalidConfidence = errors.WithCode("ORC_7012", ...)
    ErrIntentSegmentInvalidText      = errors.WithCode("ORC_7013", ...)
    ErrIntentSegmentSetEmpty         = errors.WithCode("ORC_7014", ...)
)

func (s *IntentSegment) Validate() error
func (s *IntentSegmentSet) Validate() error
```

### T10 — `PlanNode` 类型(plan/plan_dag.go)

```go
type WorkerHint string  // e.g. "explorer"/"commit"/"scenario"/"explore_parallel"
type PlanNode struct {
    ID                   string                       `json:"id"`
    SegmentID            string                       `json:"segment_id"`
    WorkerHint           WorkerHint                   `json:"worker_hint"`
    ExpectedArtifactTags []string                     `json:"expected_artifact_tags,omitempty"`
}
```

### T11 — `DataEdge` 类型(plan/plan_dag.go)

```go
type DataEdge struct {
    From              string   `json:"from"`
    To                string   `json:"to"`
    DependsOnOutputs  []string `json:"depends_on_outputs,omitempty"`  // v1 留空,字段保留
}
```

### T12 — `PlanDAG` 类型(plan/plan_dag.go)

```go
type PlanDAG struct {
    Nodes           []PlanNode       `json:"nodes"`
    Edges           []DataEdge       `json:"edges,omitempty"`
    Priorities      map[string]int   `json:"priorities,omitempty"`        // key = node.ID
    MaxParallelism  int              `json:"max_parallelism,omitempty"`   // v1 ignored,硬上限 4
}
```

### T13 — `validateDAG()` 校验函数(plan/dag_validator.go)

```go
func validateDAG(dag PlanDAG, opts ValidateOpts) error
// ValidateOpts 含 MaxFanOut (default 8) + maxNodes (10)
```

4 个子检查:
1. **环检测**:DFS white/gray/black 节点染色
2. **节点数 ≤ MaxFanOut**:超则返回 ErrPlanDAGTooManyNodes
3. **Node ID 唯一性**:O(n) set 检测
4. **Edge 端点存在**:edge.From/To 必须在 Nodes 内

### T14 — `ValidateError` 4 子类型(plan/dag_validator.go)

```go
var (
    ErrPlanDAGContainsCycle     = errors.WithCode("ORC_7020", ...)
    ErrPlanDAGTooManyNodes      = errors.WithCode("ORC_7021", ...)
    ErrPlanDAGDuplicateNodeID   = errors.WithCode("ORC_7022", ...)
    ErrPlanDAGDanglingEdge      = errors.WithCode("ORC_7023", ...)
    ErrPlanDAGInvalidPriority   = errors.WithCode("ORC_7024", ...)  // priorities key 不在 nodes
)
```

## 4. OPEN QUESTIONS(请 review 时给建议)

### Q1 — `Plan` 加字段是否破坏 immutability?
**风险**:`Plan` 当前 8 字段都用 `With*` builder 返回新值。T03 加 2 字段后,新 builder 是否影响现有测试?

**我的判断**:`plan_test.go` 不测字段数,只测 Validate/builder。给 `Plan{...}` 字面量构造时缺省 2 字段默认为 nil。

**待你确认**:你是否同意 `Plan.IntegrateIntentSegmentSet` 不进入 `Plan.Validate()`?(只校验非空 DAG 才校验 IntentSegmentSet)

### Q2 — `plan_dag.go` 文件命名
计划:`internal/layers/orchestration/plan/plan_dag.go` + `dag_validator.go`(~300 行总)。文件名用 snake_case,符合 devrix naming policy。

**待你确认**:是否拆 2 文件 OK?还是合并 1 个 plan_dag.go (~500 行)?

### Q3 — error code 起点
现有 `shared/errors/orchestration.go` 还没建(§技术债锁定过要建)。PR-A1 同时建文件 + 注册 ~10 个新 sentinel。

**待你确认**:ORC_7010 ~ ORC_7024 这段起点 OK?还是从 ORC_7000 起步,留 buffer?

### Q4 — `MaxFanOut` 默认值
目前 spec_delta §3 说 default 8,但 WaveScheduler 硬上限 4。`validateDAG` 用哪个?

**我的建议**:`validateDAG` 用 8(校验 fan-out 上限),WaveScheduler 运行时硬上限 4(资源约束)。两者不一致是设计而非 bug。

**待你确认**:是否同意双层 enforcement(校验层 8 / 运行时 4)?

### Q5 — `DataEdge.DependsOnOutputs` 字段保留 or 删
方案 α 决定"留 v1 不解析",但既然留字段就要解释文档。

**我的建议**:留,加 `// v1: ignored; v2: enable DataDep analysis`(devrix naming policy 不准"废弃注释",改成"future scope"标注)。

**待你确认**:保留 + 加 future-scope 注释 OK 吗?

## 5. DELIVERABLES

| 文件 | 新/改 | 行数估计 | T 覆盖 |
|------|------|---------|--------|
| `orchtypes/intent_segment.go` | NEW | ~120 | T01/T02/T04 |
| `orchtypes/intent_segment_test.go` | NEW | ~200 | T01/T02/T04 |
| `plan/plan_dag.go` | NEW | ~150 | T10/T11/T12 |
| `plan/plan_dag_test.go` | NEW | ~180 | T10/T11/T12 |
| `plan/dag_validator.go` | NEW | ~180 | T13/T14 |
| `plan/dag_validator_test.go` | NEW | ~260 | T13/T14 |
| `plan/plan_struct.go` | MODIFIED | +30 | T03 |
| `plan/plan_struct_test.go` | MODIFIED | +60 | T03 |
| `shared/errors/orchestration.go` | NEW | ~50 | T04/T14 |
| `shared/errors/orchestration_test.go` | NEW | ~40 | T04/T14 |
| **总计** | | **~1270 行** | **T01-T04 + T10-T14** |

## 6. TEST MATRIX (TDD red-green-refactor)

### 单元测试 T01

```go
func TestIntentSegment_JSONRoundTrip(t *testing.T) {
    // marshal + unmarshal 后字段一致
}
func TestIntentSegment_ValidateErrors(t *testing.T) {
    cases := []struct{ name string; mutator func(*IntentSegment); wantErr error }{
        {"empty_text",     func(s *IntentSegment){s.Text=""}, ErrIntentSegmentInvalidText},
        {"empty_kind",     func(s *IntentSegment){s.IntentKind=""}, ErrIntentSegmentInvalidKind},
        {"kind_not_in_enum", func(s *IntentSegment){s.IntentKind="garbage"}, ErrIntentSegmentInvalidKind},
        {"priority_too_high", func(s *IntentSegment){s.Priority=101}, ErrIntentSegmentInvalidPriority},
        {"priority_too_low",  func(s *IntentSegment){s.Priority=-1}, ErrIntentSegmentInvalidPriority},
        {"confidence_too_high", func(s *IntentSegment){s.Confidence=1.01}, ErrIntentSegmentInvalidConfidence},
        {"confidence_too_low",  func(s *IntentSegment){s.Confidence=-0.01}, ErrIntentSegmentInvalidConfidence},
    }
}
```

### 单元测试 T03

```go
func TestPlan_WithIntentSegmentSet_ImmutableCopy(t *testing.T) {
    p := NewPlan(...)
    iss := &orchtypes.IntentSegmentSet{...}
    p2 := p.WithIntentSegmentSet(iss)
    // p.IntentSegmentSet == nil
    // p2.IntentSegmentSet == iss
    // 各自修改互不影响
}
func TestPlan_Validate_StillPassesAfterNewFields(t *testing.T) {
    // 回归测试:不传新字段时,Plan.Validate() 通过
}
```

### 单元测试 T13

```go
func TestValidateDAG_Cycle(t *testing.T) {
    // 3-node 环 (A→B→C→A) → ErrPlanDAGContainsCycle
}
func TestValidateDAG_TooManyNodes(t *testing.T) {
    // 11 个独立节点 → ErrPlanDAGTooManyNodes
}
func TestValidateDAG_DuplicateNodeID(t *testing.T) {
    // 2 个节点 ID 相同 → ErrPlanDAGDuplicateNodeID
}
func TestValidateDAG_DanglingEdge(t *testing.T) {
    // edge.From 不在 nodes → ErrPlanDAGDanglingEdge
}
func TestValidateDAG_InvalidPriorityKey(t *testing.T) {
    // priorities["ghost"] 节点 ID 不存在 → ErrPlanDAGInvalidPriority
}
func TestValidateDAG_HappyPath(t *testing.T) {
    // 3-node 无环无错误 → nil
}
```

### 回归测试

- 22/22 orchestration 包 `go test -race`
- `go vet ./...`
- `go build ./...`
- 现有 `plan_test.go` 100% 通过(回归)

## 7. RISK REGISTER

| 风险 | 严重度 | 缓解 |
|------|--------|------|
| `Plan` 加字段导致 plan_test.go 失败 | Low | 加测试前先跑回归,确认 0 行 plan_test.go 改动 |
| `Plan.WithIntentSegmentSet` builder 影响现有 Plan.Validate() | Low | Validate() 不读新字段(只校验旧字段);Plan.Validate() 不变 |
| error code 起点冲突 ORC_7xxx 已占用 | Low | 现搜确认 ORC_7000-7024 无冲突;不为 0,起点 ORC_7010 |
| dag_validator 的 DFS 性能 | Low | N ≤ 8,DFS trivial;不优化 |
| `DataEdge.DependsOnOutputs` 字段让 v1 误用 | Low | 字段加 future-scope 注释 + Validate() 不读 |
| 与 upstream Phase 7 验收契约冲突 | Low | Phase 7 已 S7_Archived;只动 plan 包不破坏 |

## 8. CONSENSUS QUESTIONS (请你回答)

请从下面的角度给我反馈:

1. **架构层**:方案 β 是否完整?(Plan 加 2 字段足够吗?是否需要 Append/Reset 类的 helper?)
2. **测试层**:TDD red-green-refactor 顺序对吗?先 happy-path 还是先 error-path?
3. **类型层**:PlanNode 字段够吗?(是否需 priority_hint 字段放优先级 hint?)
4. **错误层**:10 个 error code (ORC_7010-7024) 起点 OK?有没有该加但没加的?
5. **签名层**:Validate() 签名是 `Validate()` 或 `Validate() error`?
6. **回归层**:除了 22/22 orchestration 包测试,还该跑哪些集成测试?

---

## 9. REVIEW DELIVERABLES(给 codex / cursor)

如果你跑不动这里的 Go 工程,告诉我哪些部分信息不全,我补具体代码片段。

如果你看了:
1. 工程根:`/Users/fukai/workspace/devrix`
2. 5 件套:`openspec/changes/devrix-d7-multi-intent-observation-decompose/{proposal,tasks,design,decision-tree}.md` + `specs/d7-orchestration/spec_delta.md`
3. 关键类型:
   - `internal/layers/orchestration/workmodel/pipeline_round.go:27-34`(SpawnPolicy 现状)
   - `internal/layers/orchestration/plan/plan_struct.go:19-39`(Plan 现状)
   - `internal/layers/orchestration/plan/plan.go:23-55`(PlanKind 现状)

请直接给反馈。
