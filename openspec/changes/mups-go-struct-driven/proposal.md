# Proposal: MUPS Go-struct-driven I/O contract (M1 Observe)

**Change ID:** `mups-go-struct-driven`  
**Demand:** DM-20260705-003
**Status:** S2_Design

## Why

MUPS Observe/Plan 节点向 LLM 输入 user frame 走 `BuildAnnotatedLineFrame(frame, spec, map)` 三段链路：struct 字段顺序 + `FrameSpec.Fields []TagName` + 35 行手工 `fields := map[TagName]any{...}`。三处分别描述同一份 schema，新增/删除/重命名 tag 必须三处同步；任何一处遗漏即 silent drift。**go-struct-driven 模式** 用 `pt:"<tag>,<plane>,<flags>"` struct tag 把三处折叠为单一定义点；反射注册 + 反射序列化让所有漂移在 `init()` 期 panic，编译期不可见的不一致被消灭。

## What

| Capability | Description |
|------------|-------------|
| **shared-A99** | `prompttags/structbind.go` (~120 行) — `MustRegisterFrame[T]` + `BuildLineFrameFromStruct` + `DocBlockFromStruct[T]` + `ptTag` 解析 |
| **D7-S5-A99** | `ObserveSignalInput` 加 `pt:"..."` tag + 8 字段精简到 9 字段（与 `ObserveUserFrame` 1:1 对齐） |
| **D7-S5-A99** | `buildLLMObservationUserPrompt` 35 行手工 map → 1 行 `prompttags.BuildLineFrameFromStruct("observe_user", in)` |
| **D7-S5-A99** | `MustRegisterFrame[ObserveSignalInput]()` 在 `observation_proposer.go` init 调一次 |

## Scope

- **M1（本次落地）**：`prompttags/structbind.go` kernel + Observe 节点 go-struct 化 + 0 行为变化验证
- **M2-M5（follow-on）**：仅写入总图附录，不在本 change 实现
  - M2 `mups-plan-structbind`：PlanUserFrame + StrategicPlanInput 同模式
  - M3 `d7-mups-strategy-injection`：Strategy 接口注入 WorkItemExecContext（行为增量）
  - M4 `mups-verify-table-driven`：4 VerdictKind × N trigger 表驱动
  - M5 `d7-spawn-decision-algebra`：R0-R8 嵌套 if 拆为 checkBudget/checkDirection/checkEscalation 3 个命名子决策

## Out of scope

- 复活 ChannelRouter 4 个 channel 文件（v1 死代码，DM-20260626-009 已 decommissioned）
- 修改 SpawnPolicy 6 枚举本身
- Execute / Verify / Learn 节点的 whole-body 输出 schema 变化
- 跨域 LLM 节点（D3 LLMGateway、D4 Delegate）改造
- LLM 调用本身的提示工程（schema/格式外的内容）

## Architecture decision

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: go-struct-driven + 反射（采纳）** | 单一定义点；新增 tag 1 处改动；编译期不可见的不一致被反射注册 panic 拦截 | 反射开销（init 一次 + user prompt 构造 1 次/轮，< 50μs/次）；需 `pt` struct tag 解析 |
| B: 代码生成（`go generate` 写 FrameSpec 数组） | 零运行时反射；FrameSpec 仍是手写 | 增加 `go:generate` 工具链；Go struct → FrameSpec 单向；新 tag 仍需手写 2 处（struct + generator input） |
| C: 维持现状（手写 map） | 零新依赖 | 漂移风险持续累积；DM-20260705-002 已暴露手工字段错位 bug（靠 review 抓） |

**选 A**。理由：用户最在意"重复链路 / 二义性"；反射开销可忽略（user prompt 构造 1 次/轮 vs LLM 调用的秒级延迟）；方案 C 的漂移风险已经被 DM-20260705-002 暴露过（手工 map 漏写 `prior_parse_reject` 字段、靠测试才抓到）。方案 B 解决 50% 问题但保留手写 2 处的痛点。

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 反射性能瓶颈 | Low | Low | 压测：每轮 user prompt 构造 1 次 < 50μs；LLM 调用秒级 |
| `pt` tag 漏写 / 拼错 | Med | Med | `MustRegisterFrame[T]()` init 校验所有 9 字段都有合法 `pt` tag；缺则 panic |
| i18n 翻译条目缺失 | Med | Med | `MustRegisterFrame` 调 `i18n.HasFrameFieldGuide(frame, tag)` 校验；缺则 panic |
| golden snapshot 漂移 | Low | Low | M1 0 行为变化承诺；snapshot 与 baseline diff = 0；任何 diff 阻断 PR |
| M3 行为增量回归 | Med | High | M3 最后做；M1-M2-M4-M5 全 0 行为变化；M3 单独 PR + 完整 S5 验收 |

## Success Metrics

- 0 行为变化（golden diff + E2E 测试）
- `buildLLMObservationUserPrompt` 函数体 ≤ 5 行
- 9 字段 struct 与 9 字段 FrameSpec 与 9 个 i18n 条目三方一致
- 新增 tag 工作量从 6 处 → 1 处（struct 字段 + `go test` 提示缺翻译）

## Follow-on changes (M2-M5)

参见 `design.md` §6 与 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md`（S5 同步生成）。M1 通过后，依次启动 M2/M4/M5（并行可行），M3 最后做。
