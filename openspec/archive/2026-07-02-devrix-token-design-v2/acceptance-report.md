# Acceptance Report: Token Design 2.0

**Change ID:** `devrix-token-design-v2`
**Demand ID:** DM-20260702-008
**Verdict:** **ACCEPTED**
**Merged PR:** [#376](https://github.com/fqntxmqee/devrix/pull/376) (squash merge 2026-07-02T08:50:17Z)
**S6 Archived:** 2026-07-02

---

## 1. 交付概况 (T 点 IMPLEMENTED 状态)

| 阶段 | T 点范围 | IMPLEMENTED | DEFERRED | 说明 |
|------|---------|-------------|----------|------|
| 阶段 0: 决策 | (无 T) | — | — | close PR #375 + archive DM-20260701-007 partial supersede + 起草 proposal/demand/design/tasks |
| 阶段 1-2: 持久化层 | T01-T08 (8 T) | 7 (T01-T02, T04-T08) | T03 image block (合并到 T08 测试) | PersistToFile + pipeline 集成 + ContentReplacementState + growthbook override + 19 工具 per-tool 阈值 + surface gate + tests |
| 阶段 3: Bounded 改 advisory | T09-T12 (4 T) | 4 (T09-T12) | — | ProbeToolChannel.Accept 永真 + L4-BOUNDED advisory + read_file offset/limit + orthogonal flags |
| 阶段 4: per-message aggregate | T13-T15 (3 T) | 3 (T13-T15) | — | PerMessageBudget 200K cap + pipeline 集成 + tests |
| 阶段 5-6: Concurrency + Classifier (P1) | T16-T24 (9 T) | 0 | **9 (T16-T24 全部)** | **deferred to devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009, PR #377, S7_Archived)** |
| 阶段 7: 验证 + LTL-Lite | T25-T28 (4 T) | 3 (T25, T27, T28) | T26 (是 verification 自身) | LTL-Lite L4-BOUNDED advisory + 50 文件 e2e + 8K 回归 |
| **合计 (P0)** | **19 T** | **16 IMPLEMENTED** + 1 verification (T26) | **9 P1 走 DM-20260702-009** | **PR #376 标题: 16/16 P0 T** |

**注：** 阶段 1-2 列 7 IMPLEMENTED (T01-T02 + T04-T08)，T03 image block 跳过被 T08 测试覆盖；阶段 7 列 3 IMPLEMENTED (T25 + T27 + T28)，T26 是 verification 自身而非 T 点。

## 2. PR 实施 (6 commits)

| commit | 内容 | T 点 | + / - |
|--------|------|------|-------|
| `86700768` | T01-T02 持久化层核心 (`PersistToFile` + pipeline 集成) + T08 单元测试 | 3 (T01 + T02 + T08) | persist.go 246 + persist_test.go 258 |
| `abb8bfc4` | T04-T07 `ContentReplacementState` 决策冻结 + `growthbook_override` + 19 工具 per-tool 阈值 | 4 (T04 + T05 + T06 + T07) | content_replacement_state.go 174 + growthbook_override.go 105 + orthogonal_flags.go +209 + surface_metadata_gate_test.go 145 |
| `8ee2d3ce` | T09-T12 + T25 `ProbeToolChannel.Accept` 永真 + L4-BOUNDED advisory + `read_file` offset/limit + orthogonal flags | 5 (T09 + T10 + T11 + T12 + T25) | probe.go 232 + tool_runner.go +53 + bounded.go 454 + orthogonal_flags_test.go 81 |
| `189d16a7` | T28 8K 自循环回归测试 (20 consecutive read_file 全部 accept) | 1 (T28) | probe_regression_test.go 102 |
| `78f4192a` | T13-T15 `PerMessageBudget` per-message aggregate 200K cap + pipeline 集成 | 3 (T13 + T14 + T15) | per_message_budget.go 124 + per_message_budget_test.go 170 |
| `f641a436` | T27 50 文件 review 端到端 fixture (旧 15/50 vs 新 50/50) | 1 (T27) | review50_e2e_test.go 500 |
| **合计** | **16 P0 T IMPLEMENTED** | **17** (含 T08 测试) | **+8872 / -68 / 89 files** |

## 3. 治本 vs 治标 (4 件套)

| 治标 (DM-20260701-007 撤回) | 治本 (本 PR) | 验证 |
|------------------------------|--------------|------|
| `TruncateToTokens` → 信息物理丢失 | `PersistToFile` → 全量写盘 + preview (T01) | `persist_test.go` 11 单测 |
| 无 offset/limit, re-read 拿同一截断内容 | `read_file(path, offset, limit)` (T10) | `tool_runner_offset_test.go` 90 行 |
| `Bounded(15)` 硬拒, 第 16 次调用挂死 | `Bounded(15)` advisory, 永真 + 记录违规 (T09/T25) | `bounded.go` 454 + `bounded_test.go` 238 + `bounded_advisory_test.go` 117 |
| 无 per-message 守卫, 30 个并行 tool result 累计爆炸 | `PerMessageBudget` 200K cap + 决策冻结 (T13/T04) | `per_message_budget_test.go` 8 单测 + `content_replacement_state_test.go` 9 单测 |

## 4. 治本 invariant 量化 (T27 端到端)

**50 个 line-numbered 文件 review 任务** (20 小 1-2K + 20 中 10-20K + 10 大 50-100K), 唯一 EOF marker 验证 agent 真读到 EOF:

| 设计 | completed | tool_calls | sawPersistedRefs | stoppedByBound |
|------|-----------|------------|------------------|----------------|
| 旧 (8K truncate + Bounded(15) hard reject) | **15/50** | 15 | 0 | true @ iter 15 |
| 新 (PersistToFile + offset/limit + advisory) | **50/50** | 160 | 0 (read_file 自身不触发) | false |

**增量 +35 是治本 vs 治标的量化证据**, 直接打到 PR #373 那个挂死场景 (channel 永不拒, 信息永远可达)。

## 5. 5 T 点撤回 (PR #375 partial supersede)

详见 `openspec/archive/2026-07-02-devrix-mups-tool-classification-and-channel-autonomy/SUPERSEDE-NOTICE.md`:

- 8K TruncateToTokens → T01 PersistToFile
- Bounded(15) hard reject → T09 advisory
- 单一 uniform 阈值 → T07 per-tool 差异化 (read_file 8K, grep/glob 20K, bash 30K, edit/write 100K)
- 缺 offset/limit → T10 Read offset/limit
- 无 per-message aggregate cap → T13 PerMessageBudget 200K

## 6. 保留 devrix 6 大创新

- `EmissionClass` 4 类正交分解 (Fact/Action/Probe/Experiment) — `per_emission_class.go` 86 行 + 176 行测试
- `task_kind` 推 (per_task_kind.go 134 行 + advisory_test 68 行, 16 task_kind 阈值/绑定差异化)
- `VerifyContract` 4 元组 (Burden × Class × Discipline × Outcome) — `verify_contract.go` 289 行 + 227 行测试
- MUPS 5 节点 (Multi-Unit Parallel Synthesis) — 4 ToolChannel (fact.go 71 / action.go 73 / probe.go 232 / experiment.go 55)
- Learn FeedbackMemory (SPE → reputation) — `reason_log.go` 148 + 119 行测试
- LTL-Lite L4-L6 (改 advisory 模式, T25)

## 7. 借鉴来源 (clawcode 真实源码)

- `/Users/fukai/workspace/clawcode/src/utils/toolResultStorage.ts:73-119` (persistToolResult)
- `/Users/fukai/workspace/clawcode/src/utils/toolResultStorage.ts:189-198` (buildLargeToolResultMessage)
- `/Users/fukai/workspace/clawcode/src/utils/toolResultStorage.ts:386-413` (ContentReplacementState)
- `/Users/fukai/workspace/clawcode/src/utils/toolResultStorage.ts:340-360` (generatePreview)
- `/Users/fukai/workspace/clawcode/src/tools/FileReadTool/FileReadTool.ts:497` (offset/limit)
- `/Users/fukai/workspace/clawcode/src/constants/toolLimits.ts` (50K/100K/200K)

## 8. 域 t-registry 同步 (master 已落)

| 域 | version bump | T 新增 | 来源 commit (PR #376) |
|---|---|---|---|
| D2 Context Engine | 2.13.x → 2.14.0 | +33 (T01-T08, T13-T15) | d2-context-engine/t-registry.md +33/-2 |
| D5 Observability | 3.3.x → 3.4.0 | +24 (T25 + LTL-Lite advisory) | d5-observability/t-registry.md +24/-4 |
| D7 Orchestration | 4.25.x → 4.26.0 | +79 (T09-T12 + VerifyContract + ToolChannel + ReasonLog) | d7-orchestration/t-registry.md +79/-7 |
| 根 t-registry | v4.7.x → v4.8.0 | +11 (聚合) | openspec/t-registry.md +11/-7 |

**注：** 2.14.0/3.4.0/4.26.0 在 DM-20260701-007 S4+S5 验收中已记；DM-20260702-008 在 PR #376 落地后被 2.15.0/3.5.0/4.27.0 (DM-20260702-009) 覆盖。

## 9. 流程合规

- ✅ S2_Clarified (proposal.md / demand.md / design.md / tasks.md)
- ✅ S3-Gate codex 复审 PASS
- ✅ S4 实施 (6 commits, 16 P0 T IMPLEMENTED)
- ✅ SUPERSEDE-NOTICE.md 已发 (DM-20260701-007 partial)
- ✅ 5 T 点撤回 (PR #375 已 close)
- ✅ 治本 invariant 端到端验证 (T27: 15/50 → 50/50)
- ✅ S5 验收 ACCEPTED (16/16 P0 T + 50/50 e2e + 100/100 8K 回归)
- ✅ S6 归档 (本 archive, verify-archive.sh 12/0/1)
- ✅ PR #376 squash merge 2026-07-02T08:50:17Z

## 10. P1 延期 (走 DM-20260702-009, 已 S7_Archived)

- T16-T19 IsConcurrencySafe (仿 clawcode `toolOrchestration.ts:131`) — DM-20260702-009 PR-B 落地
- T20-T24 ToAutoClassifierInput (仿 clawcode `yoloClassifier.ts:45`) — DM-20260702-009 PR-C 落地
- DM-20260702-009 S7_Archived 2026-07-02 (PR #377, 13/13 T + 21/21 AC + 4 tech-debt 关闭)

## 11. 12 OOS 项目 (走 P2/P3 后续 change)

1. 完整 LLM 上下文 transcript (10+ 工具) — P2
2. 多 LLM ensemble (ensemble classifier) — P3
3. 跨 session reputation → classifier input — P2
4. `bash_readonly_canary_percent` flag — P2
5. `auto_classifier_canary_percent` flag — P2
6. AutoModeClassifier P1 升格 — P2 (metric 触发)
7. ContentReplacementState 跨 session 持久化 — P2
8. read_file line-based offset (vs byte-based) — P3
9. persist 压缩 (gzip 大 result) — P3
10. image block 持久化 (当前 image block 跳过) — P3
11. per-tool MaxResultSizeChars LLM 提示注入 — P2
12. 8K 截断历史 metrics dashboard — P3

## 12. Lessons Learned

1. **治本 vs 治标判断**：8K truncate + Bounded(15) hard reject 看着像治本 (把"超载"挡住)，实际跟 PR #373 失败模式同源 (信息物理丢失 + 强 reject)，只是失败点从 D1 挪到 D7。**治本的 2 个核心约束：信息永不物理丢失 + channel 永不硬拒**。
2. **借鉴要追到源码行号**：clawcode 的 `toolResultStorage.ts:73-119` + `FileReadTool.ts:497` + `toolLimits.ts` 是真源头，文档 53 / 51 是二手转述。设计时直接读源码比读 doc 准。
3. **per-tool 差异化阈值**：bash 30K / grep 20K / edit-write 100K / read_file 8K(配合 offset-limit) 比 uniform 8K 实际可承受 5-10× 信息量，且不损失精度。
4. **decision freeze (ContentReplacementState) 是 cache-stable + 重放稳定的根基**，跟 LLM 探索无关但跟 VerifyContract 强相关（同一 toolUseId 必须做同样决定才能验证一致性）。
5. **P1 9 T 走独立 change (DM-20260702-009)** 而不是塞进同一 PR：保持 PR 可审 + 风险隔离 + 后期并发/分类器独立迭代。
