# Delta: D2 Context Engine — Token Design 2.0 (PersistToFile + ContentReplacementState + per-message aggregate)

**Change ID:** `devrix-token-design-v2`
**Demand ID:** DM-20260702-008
**Affects:** D2-S15 (Prepare — PersistToFile 替代 TruncateToTokens + 决策冻结 + aggregate 守卫)

---

## ADDED

### Requirement: D2-S15-A02 PersistToFile — 治本不丢失

`PersistToFile(content, toolUseId, maxChars)` SHALL 把超阈值 result 写到 `<projectDir>/<sessionId>/tool-results/<toolUseId>.txt` 并返回 preview (≤ 2KB) + XML 包装 `<persisted-output>...</persisted-output>`:

- 超过 per-tool `MaxResultSizeChars` → 写盘 + 2KB preview
- 不超过 → 原样返回 (0 行为变化)
- 写盘失败 → fall back to truncate (日志 warn，不丢任务)
- 仿 clawcode `toolResultStorage.ts:73-119` + `buildLargeToolResultMessage:189-198`

#### Scenario: AC 持久化正常路径

- GIVEN 50KB bash 输出, per-tool `MaxResultSizeChars(30K)`
- WHEN PersistToFile
- THEN 写 `<sessionId>/tool-results/<toolUseId>.txt` 50KB 全量
- AND 返回 preview ≤ 2KB (newline 边界切)
- AND XML 包装 `<persisted-output><toolUseId>...</toolUseId><preview>...</preview></persisted-output>`

#### Scenario: AC image block 跳过

- GIVEN content 含 image block (base64 data URL)
- WHEN PersistToFile
- THEN 直接返回原 block, 不 persist 不 truncate
- AND 0 行为变化

#### Scenario: AC fall-back (写盘失败)

- GIVEN 磁盘满 / permission denied
- WHEN PersistToFile
- THEN fall back to truncate + slog warn
- AND 任务不丢失, 走 truncate 路径

### Requirement: D2-S15-A02 ContentReplacementState — 决策冻结

`ContentReplacementState{SeenIds, Replacements}` SHALL 保证同一 toolUseId 永远做同样决定 (cache-stable + 重放稳定):

- 决策层 cache: 同一 ID 命中直接返旧 preview
- 跨 turn 一致性: 重放历史 transcript 不产生不同内容
- 仿 clawcode `toolResultStorage.ts:386-413`

#### Scenario: AC decision freeze

- GIVEN ContentReplacementState{toolUseId="x": preview="..."}
- WHEN 第二次 PersistToFile("x", ...) 同输入
- THEN 返回相同 preview (不重新生成)
- AND decision 跨 turn 稳定

### Requirement: D2-S15-A02 growthbook override — 运行时调参

`getPersistenceThreshold(toolName, declaredMaxResultSizeChars)` SHALL 提供 per-tool 阈值 override:

- flag: `devrix_persist_threshold_override` (per-tool map, default `{}`)
- 防御性 null/string (runtime SDK 推空值不 panic)
- flag off → 返回 declaredMaxResultSizeChars (0 行为变化)
- flag on → 返回 override 值, 不需重启 devrix

#### Scenario: AC override 生效

- GIVEN GrowthBook flag = `{bash: 50K}`
- WHEN getPersistenceThreshold("bash", 30K)
- THEN 返回 50K (override 优先)

### Requirement: D2-S15-A02 surface_metadata_gate — 19 工具 per-tool 阈值 CI 守护

`surface_metadata_gate_test.go` SHALL 强制 19 工具 surface 全部声明 `MaxResultSizeChars` + 至少 1 个 `ConvergenceContract`:

- silent default → CI 跑测试 FAIL
- 缺字段 → 编译期 + 测试期双层拦截
- 仿 clawcode 19 工具 per-tool 差异化阈值表

#### Scenario: 19 工具 per-tool 阈值表 (clawcode-style)

- GIVEN 19 工具 surface
- WHEN apply surface_metadata_gate_test
- THEN read_file=8K, grep/glob=20K, bash=30K, edit/write=100K (clawcode toolLimits.ts 真实值)
- AND 所有 19 工具 sentinel 校验 PASS

### Requirement: D2-S15-A02 PerMessageBudget — aggregate 守卫

`enforcePerMessageBudget(messages, perMessageBudget)` SHALL 累加 per-message tool_result 总长度, 超过 200K cap 触发临界/超限/排序 persist:

- 常量: `MAX_TOOL_RESULTS_PER_MESSAGE_CHARS = 200_000`
- 临界 (< 200K) → 0 行为变化
- 超限 (≥ 200K) → 排序按字符数 desc, 触发 PersistToFile
- 集成到 PrepareExecutionContext pipeline

#### Scenario: AC 临界 / 超限 / 排序

- GIVEN N=10 个 tool_result 30K/20K/100K 混合
- WHEN enforcePerMessageBudget(cap=200K)
- THEN 临界路径 (< 200K) → 0 行为变化
- AND 超限路径 (≥ 200K) → 排序后 persist, 任务不丢失
- AND 排序后剩余 aggregate ≤ 200K

## REMOVED

(none — `TruncateToTokens` + `truncate_marker.go` 保留作为 fall-back 路径)

## MODIFIED

### D2-S15 PrepareExecutionContext pipeline

- `internal/layers/contextengine/prepare/compression/pipeline.go` 改造
- 替换 `stepToolResultBudget` → `stepPersistToolResult`
- 阈值改 per-tool `MaxResultSizeChars` (从 spec.Metadata 读)
- 集成 `PerMessageBudget` 步骤

## Cross-Reference

- d5-spec-delta: LTL-Lite L4-L6 改 advisory 配套
- d7-spec-delta: ProbeToolChannel.Accept 永真 + read_file offset/limit 配套
- clawcode 真源头: `toolResultStorage.ts:73-119, 189-198, 340-360, 386-413`
