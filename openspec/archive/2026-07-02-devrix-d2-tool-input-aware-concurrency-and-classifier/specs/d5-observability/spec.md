# Delta: D5 Observability — GrowthBook Override 1 Flag

**Change ID:** `devrix-d2-tool-input-aware-concurrency-and-classifier`
**Demand ID:** DM-20260702-009
**Affects:** D5-S25 (Termination — GrowthBook override cross-check 配套)

---

## ADDED

### Requirement: D5-S25-A04 GrowthBook Override 1 Flag (bash 30K→50K)

`Override` SHALL provide 1 P0 flag:

- `BashReadOnlyThresholdBytes(defaultBytes int) int` — bash readonly threshold override

Production-Safety Constraints:
- 默认全关: 启动时 `seedFeatureFlags` 走 secure default (空 map = 全关)
- flag 未开启时, override 返回 defaultVal, **0 行为变化**
- flag 运行时变更通过 GrowthBook SDK 推送, 不需要重启 devrix

#### Scenario: Default Behavior (flag disabled)

- GIVEN seedFeatureFlags = {} (空 map)
- WHEN 调用 BashReadOnlyThresholdBytes(30000)
- THEN 返回 30000 (defaultVal, 0 行为变化)

#### Scenario: Override Enabled

- GIVEN seedFeatureFlags = {"bash_readonly_threshold_bytes": true}
- WHEN 调用 BashReadOnlyThresholdBytes(30000)
- THEN 返回 50000 (override value)

#### Scenario: Production-Safety Runtime Toggle

- GIVEN devrix 启动时 flag disabled
- WHEN ops 通过 GrowthBook SDK push flag enabled
- THEN 后续 BashReadOnlyThresholdBytes 调用返 override 值
- AND 不需要重启 devrix

## DEFERRED

### Future Flags (P2)

- `bash_readonly_canary_percent`: 等 bash 30K→50K 实际调优后再立 flag
- `auto_classifier_canary_percent`: 等 T22'-T23' 升 P1 实施时一并立

## Cross-Reference

- d2-spec-delta: ToolSurface v4 + 19 工具 default + partition + toCompactBlock + inputsEquivalent
- d7-spec-delta: D7 Execute 节点 partition + Bash sibling abort + Discard