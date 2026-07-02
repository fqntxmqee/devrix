# Delta: D5 Observability — LTL-Lite L4-L6 Termination Invariants 改 Advisory

**Change ID:** `devrix-token-design-v2`
**Demand ID:** DM-20260702-008
**Affects:** D5-S25 (Termination — LTL-Lite L4-L6 invariants)

---

## ADDED

### Requirement: D5-S25-A01/A02/A03 LTL-Lite Bounded Advisory Mode (T25)

`BoundedInvariant` (L4) + `QuotientInvariant` (L5) + `SynthesizeInvariant` (L6) SHALL 改为 advisory 模式:

- 旧: hard invariant, 触发 `SynthesizeNow` 强制收敛
- 新: advisory, 保留为观测信号 (emit metric + log)
- 触发 `InjectPromptPressure` 软警告 (D7 接管)
- 仿 clawcode 无 iteration bound 哲学 (LLM 自治, 软警告)

#### Scenario: AC BoundedInvariant advisory 行为

- GIVEN `BoundedInvariant` 检测到 `IterationsUsed > MaxN(15)`
- WHEN check
- THEN 不触发 `SynthesizeNow` (旧: 触发)
- AND emit metric `bounded_advisory.triggered` + slog warn
- AND `InjectPromptPressure` 通知 D7 channel

#### Scenario: AC QuotientInvariant + SynthesizeInvariant 同上

- GIVEN L5/L6 检测到违反
- WHEN check
- THEN 同 advisory 行为, 0 hard reject

#### Scenario: AC 观测信号保留

- GIVEN emit metric `bounded_advisory.triggered` (counter)
- WHEN ops dashboard 查
- THEN 能看到触发次数 + 频率
- AND 长期 high → 调 `MaxN` 或优化任务

## MODIFIED

### D5-S25 终止不变量语义

- hard invariant → advisory 不变量
- 触发 `SynthesizeNow` 路径删除
- 触发 `InjectPromptPressure` 路径新增
- 配套 metric: `bounded_advisory.triggered` + `quotient_advisory.triggered` + `synthesize_advisory.triggered`

## Cross-Reference

- d2-spec-delta: PersistToFile + ContentReplacementState 治本 (信息不丢失)
- d7-spec-delta: ProbeToolChannel.Accept 永真 (channel 永不硬拒)
- 8K 自我循环治本 = d2 (信息不丢) + d5 (advisory) + d7 (channel 永真) 三件套
