# 博弈论辩论 Design Round 3 — 收敛

**日期:** 2026-07-02
**作者:** Claude (综合者, 不是辩论者)
**任务:** 基于 Design Round 1 (Claude 强论证) + Round 2 (Codex + Cursor 答辩) 收敛, 更新 design.md
**输入:** S2 阶段 D1-D4 Round 3 收敛基线 + Design Round 1-2 4 决策点

> **范围**: S2 阶段 D1-D4 已 S2_R3 收敛, 本 design_R3 仅针对 design.md 浮现的 D5-D8.

---

## 1. 让步矩阵 (D5-D8 最终立场)

| # | 决策项 | S2 状态 | Design Round 1 (Claude) | Codex (D-R2) | Cursor (D-R2) | **Design Round 3 收敛** | 关键反方让步 |
|---|--------|--------|----------------------|-------------|--------------|------------------------|-------------|
| **D5** | `IsConcurrencySafe` 参数类型 | (新) | `[]byte` (扩展性) | `json.RawMessage` (跟 CheckPermission 对齐) | `[]byte` (Q3 决定性) | **`json.RawMessage`** | Claude+Cursor 让步: 类型内聚 > 扩展性 (YAGNI 适用 mcp_*) |
| **D6** | partition batch 边界 | (新) | 连续 safe 合并 (clawcode) | 连续 safe 合并 (接受 Claude) | 连续 safe 合并 (接受 Claude) | **连续 safe 合并** | 三方一致, 无争议 |
| **D7** | AutoModeClassifier 命名 | (新) | `ClassifierResult` (devrix Naming Policy) | `ClassifierResult` (坚守) | `ClassifierResult` (承认草稿 YoloResult 疏忽) | **`ClassifierResult`** | 三方一致, 修 design.md 草稿 YoloResult |
| **D8** | GrowthBook 注入方式 | (新) | 新增 CONCURRENCY (语义化) | M1 复用 PERSIST + M2/M3 独立 (类型系统) | **M1 复用 PERSIST** + M2/M3 未来独立 (devrix 文化: 复用 proven pattern) | **M1 复用 PERSIST + M2/M3 未来独立** | Claude 让步: M1 是 persist concern (bash 30K→50K = MaxResultSizeChars), 不是 concurrency |

---

## 2. D5 收敛详情: `json.RawMessage` (跟 CheckPermission 对齐)

### 2.1 三方论据

**Claude + Cursor (`[]byte` 派)**:
- 扩展性: 未来 mcp_* 接收 protobuf / messagepack, `[]byte` 不锁死 v4 签名
- bash input 是 wire layer JSON object bytes, `[]byte` 完全兼容 JSON
- devrix 文化: "interface 先宽后窄" (见 `growthbook_override.go:24-28`)

**Codex (`json.RawMessage` 派)**:
- 类型系统内聚: `IsConcurrencySafe` + `CheckPermission` 都是 pre-dispatch hook, 应同输入类型
- 既 `Execute(input string)` 跟 `CheckPermission(input json.RawMessage)` 已不一致, 不要再加 `[]byte` 第三种类型
- Bash parse failure → return false 是合理 fail-safe (T16 必须加测试点)

### 2.2 收敛选择: `json.RawMessage`

**理由**:
1. **YAGNI 适用 mcp_* 扩展**: 当前 19 工具都是 JSON-encoded, mcp_* 真实需求未触发, 不为推测性需求锁死 v4
2. **类型内聚 > 扩展性**: 同 interface 内 pre-dispatch hook 类型一致 = 减少 reviewer 认知负担
3. **godoc + IDE 心智模型**: `json.RawMessage` 在 godoc 提示 "tool_call input field bytes, expected JSON object", 跟 CheckPermission 注释同模板
4. **可演化性**: 若未来 mcp_* 真实需要非 JSON, 通过 adapter pattern 处理 (`mcp.IsConcurrencySafe(input []byte)` 私有方法 + 公有 IsConcurrencySafe(json.RawMessage) 包装)

### 2.3 T16 必须新增测试点

```go
// T16 必加 T 层测试点 (Codex 条件)
func TestIsConcurrencySafe_BashParseFailure_ReturnsFalse(t *testing.T) {
    surface := &BashSurface{...}
    cases := []struct{
        name  string
        input []byte
    }{
        {"empty", []byte("")},
        {"non-json", []byte("rm -rf /")},  // wire layer 必须是 JSON
        {"json-missing-command", []byte(`{"path":"/foo"}`)},
        {"json-malformed", []byte(`{"command":`)},  // truncated
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            if surface.IsConcurrencySafe(tc.input) {
                t.Errorf("expected false on parse failure, got true")
            }
        })
    }
}
```

---

## 3. D6 收敛详情: 连续 safe 合并 (三方一致)

### 3.1 三方一致理由

- clawcode `toolOrchestration.ts:84-118` 实战验证
- 保留 LLM 输出顺序 (因果链)
- 50 read_file 场景: 1 batch 全并发, < 串行/3
- 同 tool 合并会破坏 LLM 顺序 + 5× 串行化

### 3.2 design.md 已正确 (无修改)

design.md §3.1 / §3.3 / §5.1 已写明连续 safe 合并规则, 维持.

---

## 4. D7 收敛详情: `ClassifierResult` (三方一致)

### 4.1 design.md 草稿需修正

**原草稿 (§6.2 错误码三元组 + §④ 聚合根表) 出现 `YoloResult`** — 三方一致认为这是起草疏忽, 应改为 `ClassifierResult`.

### 4.2 修正点

- design.md §④.1 聚合根表: `YoloResult` → `ClassifierResult`
- design.md §6.2 错误码三元组表: 涉及 `YoloResult` 的 2 处都改
- design.md §6.3 幂等保障表: 1 处
- design.md §6.4 版本演进: 1 处
- T22' API 代码: `type YoloResult struct{...}` → `type ClassifierResult struct{...}`

---

## 5. D8 收敛详情: M1 复用 PERSIST + M2/M3 未来独立 struct

### 5.1 关键发现 (Cursor + Codex 一致指出, 我接受)

**Cursor 引用 `growthbook_override_test.go:33-38` 已预演 M1 = `bash: 50*1024` 即 50K**
- 该 flag 在 `growthbook_override.go:18` 已命名: `devrix_persist_threshold_override`
- 这是 **persist concern** (MaxResultSizeChars 持久化阈值)
- **不是 concurrency concern** (跟 IsConcurrencySafe 决策无关)

**我之前 (Claude S2 + D-R1) 误判 M1 = concurrency**:
- "新增 CONCURRENCY struct 服务 M1" 是 **domain 混淆** (Cursor 反驳)
- M1 应走 PERSIST 模式, M2/M3 未来各独立 struct

### 5.2 M1 / M2 / M3 concern 分类

| Flag | Concern | 值类型 | 本 change 状态 |
|------|---------|--------|---------------|
| **M1** `devrix_persist_threshold_override` (bash 30K→50K) | **persist** (MaxResultSizeChars) | `map[string]int` | **P0 保留** (T25' 复用 PERSIST) |
| **M2** `devrix_concurrency_bash_readonly` | **concurrency** (read-only bash canary) | `bool` / session% | **P2 推迟** (等 RC-1 per-input 验证) |
| **M3** `devrix_classifier_enable` | **classifier** (auto-mode 比例) | `float` session% | **P2 推迟** (等 D2 升 P1) |

### 5.3 T25' 收敛: 复用 PERSIST 模式

**之前 design.md 附录 A 误写**:
```
| `internal/layers/observability/instrument/growthbook/concurrency_override.go` | T25' | PR-D+E | bash 30K→50K threshold override (1 flag) |
```

**修正为** (复用 PERSIST, 不新建 struct):
```
| `internal/layers/contextengine/persist/growthbook_override_bash.go` (M1 30K→50K 实际是 persist concern, 复用 growthbook_override.go 模式) |
```

**理由**:
1. M1 是 persist concern, 复用 `persist/growthbook_override.go:18` `PersistThresholdOverrideFlag` + `GetPersistenceThreshold` 模式
2. devrix 文化: 复用 proven pattern, 不发明平行类型宇宙
3. T25' 实现 = 给 bash tool 加 `MaxResultSizeChars` GB override entry, 走现有 `ThresholdOverride` struct

### 5.4 M2 / M3 未来独立 struct (P2)

**Cursor 接受 Codex 的 "未来 M2/M3 各独立 struct" 立场**, 但:
- M2 是 `bool` / session% canary, **不是 `map[string]int`**, 跟 PERSIST struct 值类型不同
- M3 是 `float` session% classifier enable, 也跟 PERSIST 不同
- 因此 M2/M3 落地时新建 struct 是 **类型系统** 自然结果, **不是 DRY 违反**

**M2 / M3 未来 concern struct 命名** (devrix Naming Policy 语义化):
- M2 → `ConcurrencyCanaryOverride{values: map[string]bool}` 或 `map[string]float64`
- M3 → `ClassifierEnableOverride{ratio: float64}`

---

## 6. design.md 修正清单

| § | 原 | 修正 |
|---|----|-----|
| §④.1 聚合根表 (3rd row) | `YoloResult` | `ClassifierResult` |
| §④.3 Span/Metric 表 | (无 YoloResult) | (无修改) |
| §5.1 时序图 | (无 YoloResult) | (无修改) |
| §6.2 错误码三元组 (3 处 YoloResult) | `YoloResult` | `ClassifierResult` |
| §6.3 幂等保障 | `YoloResult` | `ClassifierResult` |
| §6.4 版本演进 | `YoloResult` | `ClassifierResult` |
| §T16 API 代码 | `IsConcurrencySafe(input []byte) bool` | `IsConcurrencySafe(input json.RawMessage) bool` |
| §T16 新增 T 测试点 | (无) | `TestIsConcurrencySafe_BashParseFailure_ReturnsFalse` |
| §T22' API 代码 | `YoloResult` | `ClassifierResult` |
| §T25' 附录 A | `concurrency_override.go` | **删除该 file, 改 `growthbook_override_bash.go` (复用 PERSIST 模式)** |
| §T25' 命名规范表 | `ConcurrencyOverride` | `PersistThresholdOverride` (M1) / `ConcurrencyCanaryOverride` (M2, P2) / `ClassifierEnableOverride` (M3, P2) |
| §附录 D.2 博弈论决策表 | (新增 D5-D8 4 行) | 见下表 |
| §附录 D.3 (待 Round 2-3 决策) | D5-D8 4 行 | 全部更新为最终立场 |

### 附录 D.2 博弈论决策表 (design 阶段新增)

| 决策点 | 倾向 | 关键证据 | 反方让步理由 |
|--------|------|---------|--------------|
| **D5** IsConcurrencySafe 参数类型 | **`json.RawMessage`** (跟 CheckPermission 对齐) | `tool_surface.go:160` CheckPermission 既有 `json.RawMessage` 签名 + devrix 类型内聚 | Claude+Cursor 让步: YAGNI 适用 mcp_* 扩展, 类型内聚 > 推测性扩展性 |
| **D6** partition batch 边界 | **连续 safe 合并** | clawcode `toolOrchestration.ts:84-118` 实战验证 + LLM 顺序保留 | 三方一致, 无让步 |
| **D7** AutoModeClassifier 命名 | **`ClassifierResult`** | devrix Naming Policy (PR #33 落地) + spec.md/t-registry "classifier" 术语 | 三方一致, 修 design.md 草稿 YoloResult 疏忽 |
| **D8** GrowthBook 注入方式 | **M1 复用 PERSIST + M2/M3 未来独立 struct** | `growthbook_override_test.go:33-38` 已预演 M1 = `bash: 50*1024` = persist concern + devrix 文化: 复用 proven pattern | Claude 让步: M1 是 persist concern, 不是 concurrency; 之前的 "新增 CONCURRENCY" 是 domain 混淆 |

---

## 7. 完整决策链 (S2 + Design 三方共识)

| 决策点 | 立场 | 关键事实 |
|--------|------|----------|
| **D1** per-input 实现 | 分层混合 (4 override + 15 default) | clawcode Tool.ts:402,556 + BashTool.tsx:434-442 |
| **D2** auto-mode classifier | P2 interface only | devrix 无相关 incident, VerifyContract 4 元组已够用 |
| **D3** GrowthBook (1 flag P0) | 复用 PERSIST 模式 (D8 收敛后) | M1 bash 30K→50K = persist concern |
| **D4** PR 数量 | 5 PR (D+E 合并) | devrix hotfix 模式 |
| **D5** IsConcurrencySafe 类型 | **`json.RawMessage`** | 跟 CheckPermission 对齐 |
| **D6** partition batch 边界 | 连续 safe 合并 | clawcode 实战验证 |
| **D7** AutoModeClassifier 命名 | `ClassifierResult` | devrix Naming Policy |
| **D8** GB 注入方式 | M1 复用 PERSIST + M2/M3 未来独立 | Cursor 引用 growthbook_override_test.go:33-38 |

**总工时影响**:
- D8 收敛后 T25' 复用现有 `persist.ThresholdOverride` struct, 节省 ~50 行 + 12 单测 (跟 Codex 估算一致)
- D5 收敛后 T16 必须加 `TestIsConcurrencySafe_BashParseFailure_ReturnsFalse`, +1 单测 (~20 行)
- D7 收敛后 design.md 修 4 处 YoloResult → ClassifierResult, 0 实质影响
- D6 已写, 0 修改

---

## 8. 总结: 三方 design 阶段共识度

| 决策点 | Claude | Codex | Cursor | 收敛 |
|--------|--------|-------|--------|------|
| D5 | []byte | json.RawMessage | []byte | **json.RawMessage** (2:1 让步) |
| D6 | 连续safe | 连续safe ✓ | 连续safe ✓ | **连续safe** (一致) |
| D7 | Classifier | Classifier ✓ | Classifier ✓ | **ClassifierResult** (一致) |
| D8 | CONCURRENCY | M1 PERSIST | M1 PERSIST ✓ | **M1 PERSIST + M2/M3 未来独立** (Claude 让步) |

**共识度**: D6/D7 完全一致, D5/D8 通过让步收敛. design.md 进入 S3-Gate ready 状态.
