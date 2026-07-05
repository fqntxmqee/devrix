# Delta: shared/prompttags — Go-struct binding kernel

**Change ID:** `mups-go-struct-driven`  
**Demand:** DM-20260705-003
**Affects:** `internal/shared/prompttags/structbind.go` (NEW), `internal/shared/prompttags/semantics.go` (MOD)

## ADDED Requirements

### Requirement: `MustRegisterFrame[T]` 反射注册

`prompttags` 包提供 `MustRegisterFrame[T any](frameName FrameName) *RegisteredFrame[T]`，通过反射解析 `T` 的 `pt:"<tag>,<plane>,<flags>"` struct tag，写入 `LineFrameRegistry` 并校验以下 4 项一致性：

1. **pt tag 完整**：T 的每个 exported field 都有 `pt:"..."` tag（或显式 `pt:"-"` 标记为非 user frame 字段）
2. **tag_name 合法**：tag_name 必须在 `TagName` 常量集中
3. **plane 合法**：plane 必须是 `data` 或 `control`（与 `PromptPlane` 枚举一致）
4. **i18n 翻译存在**：每个 tag 在 i18n `FrameFieldGuide` 表中都有 zh/en 双语条目

任一校验失败 → **panic**（`fmt.Errorf` 包装的 `ErrStructBindPanic` sentinel），进程启动失败。

#### Scenario: 注册成功
- **GIVEN** `ObserveSignalInput` 9 字段均带合法 `pt:"..."` tag
- **WHEN** `init()` 调 `MustRegisterFrame[ObserveSignalInput](FrameObserveUser)`
- **THEN** `LineFrameRegistry[FrameObserveUser].Fields` 长度 == 9；进程启动成功

#### Scenario: pt tag 缺失 → panic
- **GIVEN** struct 某字段无 `pt:"..."` tag
- **WHEN** `MustRegisterFrame[T]` 反射该字段
- **THEN** panic with `"prompttags: ObserveSignalInput.SessionID: missing pt struct tag"`；进程退出码非 0

#### Scenario: plane 错误 → panic
- **GIVEN** struct 字段 `pt:"directive,bogus"`
- **WHEN** `MustRegisterFrame[T]` 反射该字段
- **THEN** panic with `"prompttags: directive: invalid plane 'bogus', want data|control"`

#### Scenario: i18n 翻译缺失 → panic
- **GIVEN** struct 字段 `pt:"new_tag,data"` 但 `prompttags_semantics_zh.go` 无该 tag 条目
- **WHEN** `MustRegisterFrame[T]` 调 `HasFrameFieldGuide`
- **THEN** panic with `"prompttags: FrameFieldGuide missing for new_tag in zh"`；提示需补 i18n 翻译

### Requirement: `BuildLineFrameFromStruct` 反射序列化

`prompttags` 包提供 `BuildLineFrameFromStruct(frameName FrameName, s any) string`，反射读取 `s` 的每个字段，调用内部 `BuildAnnotatedLineFrame(frameName, spec, fields)`（复用现有 kernel）。

**输出字节级等价**于手工 map 调 `BuildAnnotatedLineFrame`。

#### Scenario: 完整字段输入
- **GIVEN** `ObserveSignalInput{WorkItemID:"wi-1", Directive:"...", PriorMean:0.7, ScopeGoal:"...", ScopeOpenQuestions:["q1"], InboundSignalLines:["s1","s2"], PriorObservationIDs:["obs-1"], IncrementalOnly:true, PriorParseReject:""}`
- **WHEN** `BuildLineFrameFromStruct(FrameObserveUser, &in)`
- **THEN** 输出含 9 行，每行形如 `[control] work_item_id: wi-1` / `[data] directive: ...`；与 `buildLLMObservationUserPrompt` 旧实现字节等价（modulo whitespace）

#### Scenario: omit_empty 字段
- **GIVEN** `ObserveSignalInput{ScopeGoal:"", ScopeOpenQuestions:nil, InboundSignalLines:nil, PriorObservationIDs:nil, IncrementalOnly:false, PriorParseReject:""}`
- **WHEN** `BuildLineFrameFromStruct(FrameObserveUser, &in)`
- **THEN** 输出仅含 `work_item_id` / `directive` 2 行；其余 7 个 omit_empty 字段跳过

#### Scenario: join="," 字段
- **GIVEN** `ObserveSignalInput{PriorObservationIDs:["obs-1","obs-2","obs-3"]}`
- **WHEN** `BuildLineFrameFromStruct(FrameObserveUser, &in)`
- **THEN** 输出 `[control] prior_observation_ids: obs-1,obs-2,obs-3`（逗号拼接，与旧实现一致）

### Requirement: `DocBlockFromStruct[T]` 反射 schema 文档

`prompttags` 包提供泛型函数 `DocBlockFromStruct[T any]() string`，反射 struct 字段生成 JSON schema 文档，字节等价于手写 `DocBlockObserveSchema()` / `DocBlockPlanSchema(...)`。

#### Scenario: Observe schema 生成
- **GIVEN** `T = ObserveSignalInput`
- **WHEN** `DocBlockFromStruct[ObserveSignalInput]()`
- **THEN** 输出含 9 字段定义（kind / strength / statement / question / evidence + 5 个 user frame 字段）；与 `DocBlockObserveSchema()` 字段一致

### Requirement: `HasFrameFieldGuide` i18n 校验

`prompttags` 包提供 `HasFrameFieldGuide(frame FrameName, tag TagName) bool`，查 `i18n.FrameFieldGuideRegistry`（zh + en 任一缺失返回 false）。

#### Scenario: 翻译存在
- **GIVEN** i18n registry 有 `FrameObserveUser → TagWorkItemID` 条目（zh/en）
- **WHEN** `HasFrameFieldGuide(FrameObserveUser, TagWorkItemID)`
- **THEN** 返回 true

#### Scenario: 翻译缺失
- **GIVEN** i18n registry 无 `FrameObserveUser → TagNewField` 条目
- **WHEN** `HasFrameFieldGuide(FrameObserveUser, TagNewField)`
- **THEN** 返回 false

## MODIFIED

| 文件 | 变更 |
|------|------|
| `internal/shared/prompttags/registry.go` | 新增 `RegisteredFrame[T]` 类型（持有反射 metadata）+ `frameRegistryByName map[FrameName]*RegisteredFrameMeta`；`LineFrameRegistry` 保留手写入口（M1 不删） |
| `internal/shared/prompttags/semantics.go` | 新增 `HasFrameFieldGuide` 函数；现有 `FrameFieldPlane` 不变 |

## REMOVED

无（M1 阶段保留 `BuildAnnotatedLineFrame` 旧 API；M2 移除）

## Invariants

1. **热路径零反射**：`init()` 期一次反射写哈希表；`BuildLineFrameFromStruct` 仅做字段值反射读（每轮 1 次）
2. **struct = SoT**：struct 字段顺序由 `MustRegisterFrame` 反射写 `FrameSpec.Fields`；手写 `FrameSpec.Fields` 与 struct 字段不一致 → init panic
3. **i18n 同步**：struct 字段必须与 `prompttags_semantics_{zh,en}.go` 翻译条目一一对应；缺一即 init panic
4. **向后兼容**：`BuildAnnotatedLineFrame` / `BuildLineFrame` / `FrameSpec` / `LineFrameRegistry` 旧 API 全部保留；M2 移除 `BuildAnnotatedLineFrame`（kernel 已统一为反射版）

## Test Points

| T ID | 描述 | L5 |
|------|------|-----|
| shared-A99-T01 | `MustRegisterFrame[T]` 反射注册成功（happy path） | L5-MUPS-GSD-01 |
| shared-A99-T02 | `BuildLineFrameFromStruct` 字节等价旧 map 拼接 | L5-MUPS-GSD-02 |
| shared-A99-T03 | `DocBlockFromStruct[T]` 字段一致手写 DocBlock | L5-MUPS-GSD-03 |
| shared-A99-T04 | 4 项 init panic 校验（pt 缺 / plane 错 / i18n 缺 / 字段数漂移） | L5-MUPS-GSD-04 |
| shared-A99-T05 | `HasFrameFieldGuide` zh/en 双语查找 | — |
