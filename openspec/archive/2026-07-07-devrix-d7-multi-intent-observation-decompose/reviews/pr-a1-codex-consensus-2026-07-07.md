# PR-A1 Consensus Review — codex (MiniMax-M3), 2026-07-07

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Reviewer:** codex exec (MiniMax-M3, --model MiniMax-M3, sandbox read-only)
**Session ID:** 019f3b2c-963c-7011-8114-5748e44d0188
**Tokens:** 22,750
**Status:** Single-reviewer consensus (cursor-agent 撞用量上限,7/20 重置;仅 codex 一路对齐)

---

## Q1 — Plan immutability 边界

**Codex 答复**:同意,`Plan.Validate()` **不**递归进入 `IntentSegmentSet`。DAG validator 管 DAG 语义,`Plan.Validate()` 管自己 8 字段。**加 1 行 boundary 注释** 防止后续 reader 把它们"贴心地"连起来。

可选(零成本):加 `Plan.IsSegmented() bool` getter 给下游分支用。否则跳过。

**采纳**:`Plan.Validate()` 不读 IntentSegmentSet;`plan_struct.go` 加边界注释。

## Q2 — 文件拆分(plan_dag.go + dag_validator.go vs 单合并)

**Codex 答复**:拆 2 文件。合并 ~330 行(< 800 行上限)但 `dag_validator_test.go` ~260 行要独立;co-location 伤 review focus。

**采纳**:拆 2 文件(已定)。

## Q3 — Error code 起点 ORC_7010~7024

**Codex 答复**:Buffer 太短。建议:
- ORC_7001-7009 留给 PR-A2(AC contract) + PR-A3(LLM IO)
- ORC_7030+ 留给 PR-B/PR-C/PR-E

否则 PR-A2 会撞 code。

**采纳**:
- IntentSegment 段:`ORC_7010-7019` (10 个:7010 invalid kind, 7011 invalid priority, 7012 invalid confidence, 7013 invalid text, 7014 set empty, ...)
- DAG 段:`ORC_7020-7029` (10 个:7020 cycle, 7021 too-many-nodes, 7022 duplicate-id, 7023 dangling-edge, **7025 empty-dag (NEW)**, ...)

## Q4 — MaxFanOut 校验层 8 vs 运行时 4

**Codex 答复**:双层 enforcement 正确 — validator 抓**编写错误**,WaveScheduler 是**资源防护**。`dag_validator.go` header 注释区分二者,否则读者会把 gap 当 bug。

**采纳**:加 `dag_validator.go` 顶部 header 注释。

## Q5 — DataEdge.DependsOnOutputs 留还是删

**Codex 答复**:留 + future-scope 注释。正确绕过 naming policy(不允许"废弃"注释,改成"future scope")。

**采纳**:`// v2: enable DataDep analysis (PR-A1 reserves field, Parse/Validate ignore)` — 这是 future-scope 注释。

## §8 Consensus

1. **β 完整性**:2 字段够。不要预建 Append/Reset(那是 PR-B RunPlanDAG 的事)。

2. **TDD 顺序**(采纳):
   ```
   T01/T02/T10/T11/T12  happy-path JSON round-trip → 锁 wire format
   T04/T14              Validate error-path red→green → 锁校验语义
   T03                  Plan 加字段 + 22-test 回归
   T13/DAG 最后         validateDAG happy + 5 sub-errors(含 ErrPlanDAGEmpty + 自环)
   ```
   **依据**:Validate 错误会掩盖类型 shape bug(类型 bug 也会触发 Validate 失败);先 happy-path 锁格式,再 error-path 锁语义。

3. **PlanNode.priority_hint**:不要。优先级走 `IntentSegment.Priority`,PlanNode 通过 `SegmentID` 解析。重复字段 = 状态发散风险。

4. **ErrPlanDAGEmpty 缺口**:确认。Empty DAG ≠ cycle/duplicate/dangling — 调用者无法区分 "no plan" vs "broken plan"。**新增 `ErrPlanDAGEmpty` (ORC_7025)**。

5. **Validate 签名**:`Validate() error`,一致。

6. **回归**:
   - `golangci-lint run`
   - `go test -cover` ≥ 80%
   - `plan/` 加 fuzz/property tests(短 DAG 构造 + Validate)
   - 如 `changes/devrix-d7-*/tests/` 有 integration test,加 `-tags=integration`

## 额外风险(codex 提出)

1. **`IntentSegmentSet.SourceDirective` 空值 policy**:spec 没规定。**拍板(2026-07-07 用户确认继续)**:
   `IntentSegmentSet.Validate()` 不直接返回 error,但 `slog.Warn("intent_segment_set_empty_source_directive", segment_count=len(segments))` 记录 audit log。
   比"拒绝空"更宽松,比"完全沉默"更可观测。
   **实现位置**:`orchtypes/intent_segment.go:Validate()` 内,在 Validate 主路径外发 slog,Validate 主路径不变。
   **后续 PR-B hook**:Phase 3 接 Execute 时如有需要再升级为 error 返回。

2. **自环 edge**:DFS 显式拒绝 `edge.From == edge.To`(算 cycle)。加 test `TestValidateDAG_SelfLoop`。

3. **`Priorities map[string]int` 结构耦合**:`ErrPlanDAGInvalidPriority` 已抓孤儿,OK。

## 待办(采纳后)

- [x] Plan.Validate() boundary 注释
- [x] 2 文件拆分
- [x] ORC_7010-7019 / ORC_7020-7029 分段
- [x] ORC_7025 = ErrPlanDAGEmpty
- [x] MaxFanOut 8 / runtime 4 双层注释
- [x] DataEdge future-scope 注释
- [x] PlanNode 不加 priority_hint
- [x] TDD 顺序 = happy → error → Plan → DAG
- [x] SourceDirective policy 拍板:`slog.Warn` audit log,不返回 error
- [ ] 自环 edge `TestValidateDAG_SelfLoop` test

---

## Cursor 状况

`cursor-agent -p` 撞用量上限(2026-07-20 重置)。**15 天内 cursor 无法拉新 review**。

**应对**:
- 7 项核心 codex 反馈已采纳(上表)
- SourceDirective policy 不阻塞 PR-A1(可在 Validate 中加 Note,后置 PR 调整)
- 等 cursor 恢复后补一份独立 review;若 codex 已经覆盖关键点,cursor 主要是健壮性 check

不影响 PR-A1 编码启动。
