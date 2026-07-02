# Partial Supersede Notice (2026-07-02)

**Superseded by:** `devrix-token-design-v2` (DM-20260702-008, Token Design 2.0 — clawcode-style persistence)
**Supersede scope:** narrow (Token 处理 4 个核心 T 点), 不影响本 change 33/33 T 全部保留
**Supersede reason:** 8K token 截断 + Bounded(15) hard reject 是治标不治本 — 信息物理丢失 + 强 reject 等同 PR #373 红卡挪到 channel 层

## 撤销 (重做, 进入 DM-20260702-008)

| T 点 | 旧实现 | 新实现 |
|------|--------|--------|
| D2-S15-A02-T13 | `TruncateWithMarker` 强制 `complete=false` in-content | `PersistToFile` 写磁盘 + `<persisted-output>` XML 引用 + 2KB preview |
| D7-S9-A50-T03 (ProbeToolChannel.Accept) | `Bounded(15) hard reject` 返 `ErrProbeToolChannelBoundExceeded` | `OpenEnded + advisory warning` (3 阶段 soft → hard → forced) |
| D2-S15-A02-T06..T12 (19 工具 MaxResultSizeChars) | 全部 8K chars uniform | per-tool 差异化 (Read=8K persist, Grep=20K, Bash=30K, Edit/Write=100K, Web*=100K) |
| D5-S25-A01/A02/A03 (LTL-Lite L4-L6) | hard invariant, trigger SynthesizeNow | advisory, 保留为观测信号 (emits metric/log, 不 trigger reject) |
| D2 compression_steps.go:14-19 | `TruncateToTokens + "\n...[truncated]"` 物理消失 | 走 `PersistToFile`, 实在失败才 fall back to truncate (日志 warn) |

## 保留 (仍有效, 沿用)

| T 点 | 保留实现 | 理由 |
|------|---------|------|
| D7-S10-A50 VerifyContract 4 元组 | (Burden × Class × Discipline × Outcome) 探索式 finalText 必败 | 事后治本, 跟 token 无关, 创新 |
| D7-S9-A50 EmissionClass 4 类 (Fact/Action/Probe/Experiment) | tool 自我分类 + 4 ToolChannel 路由 | 架构性创新, clawcode 缺 |
| D7-S9-A26-T06 Filter v2 task_kind 推 | review/edit/test/observe task_kind 路由 | 创新, clawcode 缺 |
| D7-S2-A50-T07/T08 session_complete meta + Learn FeedbackMemory | 跨 session reputation (H7) | 创新, clawcode SessionMemory 较简单 |
| D7-S9-A50-T06 PlanChannel rename | Channel → PlanChannel, type alias 保留 1 release | 命名清晰, 跟 token 无关 |
| D5-S25 LTL-Lite 观测信号 | 改 advisory 后保留 | 治本 (L4 hard reject) 改观测 (emit metric) |
| PR-A ToolSpec v3 + 6 control plane fields | EmissionClass/ConvergenceContract/IterationBound/SourceUncertainty/MaxResultSizeChars/TruncateMarkerText | 元数据 schema 不变, 值/语义在 DM-20260702-008 调整 |

## 历史决策保留 (设计探索记录)

本 change 的 6 轮对话合成 + 博弈论双 review (H7-H12) 全部保留, 是 DM-20260702-008 的重要输入:
- H7 SPE + reputation: 保留到 Learn FeedbackMemory
- H8 L4-L6 cross-check ≥3 条: LTL-Lite 改 advisory 但保留 ≥3 条检查作为观测
- H9 PlanKind × EmissionClass 交叉一致: 保留
- H10 cheap talk → Learn FeedbackMemory: 保留
- H11 Phase B shadow mode: 保留 (PromptPressure 仍走 shadow → enforce 两阶段)
- H12 禁止 silent default: 保留 (surface_metadata_gate_test 仍强制每 surface 声明 EmissionClass + 新增 PersistThreshold)

## 流程合规

- 流程 SoT `openspec/specs/project/master.md` + `openspec/specs/project/archiving.md`
- 本 change 状态保持 `s7_archived`, supersede 在 archive 内部标注, 不修改 S7 状态
- DM-20260702-008 起草后会建对应 `supersedes: devrix-mups-tool-classification-and-channel-autonomy (partial: 5 T 点)` 字段做溯源
