# 三方博弈综合 — DM-20260702-009

**日期:** 2026-07-02
**作者:** Claude (主 agent, 综合者)
**参与方:** Claude + Codex (MiniMax-M2.7) + Cursor (plan mode)
**输入材料:**
- `demand.md` (S1 需求, 14 AC + 13 T)
- `openspec/tech-debt/streaming-tool-executor-v2.md` (TD-STE-01~06)
- `internal/shared/contracts/tool_surface.go` (现 9 字段 + v3 6 control plane)
- `clawcode Tool interface 35 字段`

---

## 0. 一句话总结

三方一致接受 4 项治本（per-input 函数 / auto-mode classifier 链路 / 4 项 tech-debt 收口 / Bash sibling abort + Discard）; **争议在 3 个 scope 裁剪项**: auto-mode 触发强度 / GrowthBook 必要性 / PR 粒度。Codex 跑出**关键新发现**: devrix 已经有 `ReadOnly` (`tool_surface.go:43`) + `InterruptMode` (`tool_surface.go:66`), clawcode 对应字段不需借鉴。

---

## 1. 三方立场对照

| # | 博弈点 | Claude | Codex (M2.7) | Cursor (plan) | 共识度 |
|---|--------|--------|--------------|---------------|--------|
| 1 | per-input 函数 vs 字段化 | 函数化 (clawcode 路线) | **分层混合** (静态默认 + Bash override) | 函数化 | ⚠️ **三方接近, Codex 折中** |
| 2 | auto-mode classifier 必要性 | P0 实施, 默认关 | **只加 interface, 降 P2** | 结构上必要, P0 | ❌ **三方分歧** |
| 3 | 4 tech-debt 同 change 收 | 同 change | 同 change | 同 change | ✅ **三方一致** |
| 4 | PR 数量 (5 vs 6) | 5 PR (合并 D+E) | **6 PR (维持)** | 5 PR | ⚠️ **Claude+Cursor 一致, Codex 反对** |
| 5a | GrowthBook (AC11) | 降 P2 (默认关是死代码) | **全删** | **保留 P0** (横向复用) | ❌ **三方分歧** |
| 5b | inputsEquivalent (AC14) | 降 P3 或删 | **保持 P2** | 降 P3 | ⚠️ **Claude+Cursor 一致, Codex 偏强** |

---

## 2. 关键新发现 (Codex 贡献)

Codex 跑 agent loop 时 exec 读了 `tool_surface.go` 全文, 发现**已有字段** (clawcode 对应字段不需借鉴):

```go
// internal/shared/contracts/tool_surface.go
type ToolSpec struct {
    // ...
    ReadOnly       bool   // line 43 — clawcode isReadOnly 已有
    Destructive    bool   // clawcode isDestructive 已有
    OpenWorld      bool   // clawcode openWorld 已有
    DeferLoading   bool   // clawcode shouldDefer 已有
    // ...
    MaxResultSizeChars int // v3 field — clawcode maxResultSizeChars 已有
}

type InterruptMode string  // line 66 — clawcode interruptBehavior 1:1 映射
const (
    InterruptCancel InterruptMode = "cancel"  // clawcode 1:1
    InterruptBlock  InterruptMode = "block"
)
```

**Codex 借鉴评分修正** (与需求文档对比):
- 需求暗示"借鉴关系 10 项" → 实际**只有 4 项真正需要从 clawcode 学**
- isConcurrencySafe (per-input 函数) / toAutoClassifierInput / siblingAbortController / discard()
- 其他 5 项是 devrix **已有** (ReadOnly / InterruptMode / MaxResultSize / DeferLoading / Destructive)

**Claude + Cursor 都未独立验证这点** — 这是 Codex 独立视角的实质贡献。

---

## 3. 博弈点深析

### 博弈点 1: per-input 函数 vs 字段化 vs 分层混合

| 立场 | 方案 | 优劣 |
|------|------|------|
| Claude (P1) | 全函数化 | 表达力最强; 但 19 工具都写函数, 工作量 + 回归风险 |
| **Codex (P1)** | **分层混合: 静态默认值 + 关键工具 override** | **治本 (Bash isReadOnly + read_file size) 不需要每个工具都函数化; 99% 工具走默认, 1% override** |
| Cursor (P1) | 全函数化 | 同 Claude |
| 字段化 (0 票) | ToolSpec 加 enum / metadata | 三方一致拒绝: 表达力不够 |

**Codex 的折中价值**: 9 P0 工具中, 真正需要 per-input 判定的只有 3-4 个 (Bash / read_file / Edit / Write), 其他 15 个工具按 v2 静态值即可。**全函数化是过度工程**。

**建议裁决**: 采用 **Codex 折中** (分层混合), 节省 15 × ~30 行 + 15 × ~3 单测 = ~600 行代码 + 45 单测。

### 博弈点 2: auto-mode classifier 必要性

| 立场 | 触发模式 | 实施优先级 |
|------|---------|----------|
| Claude | P0 实施 + 默认关 + 5s timeout | P0 实施, 默认 OFF (Production-Safety) |
| Cursor | 结构上必要 (填补执行前防线空洞) | P0 实施, 默认 OFF |
| Codex | 只加 interface + 实证后再开 | **P2** (只加 `ClassifyToolUse` 方法, 不接线) |

**Codex 反对理由**: "5s timeout + 默认关闭" = 实施后**长期处于名义存在、实际无人依赖状态** — 死代码, 浪费工时。

**Cursor 反驳 Codex**: 防线空洞是结构问题, 必须在 P0 修, 否则下个 change 也会被同样的论据推迟。

**关键事实**: VerifyContract 4 元组已是 ground truth, auto-mode 是**中间层防御**而非替代品。三方都接受这点, 争议是"什么时候建"。

**建议裁决**: 倾向 **Codex P2** (只加 interface, 不接线, 不写 SideQuery 实际调用) — 节省 AC4-AC7 + AC11 大半工时; P1 再开 SideQuery 实际调用。

### 博弈点 3: 4 tech-debt 同 change 收口

✅ **三方一致**:
- TD-STE-01 (混合批次) → PR-B (partitionToolCalls)
- TD-STE-02 (Bash sibling abort) → PR-F
- TD-STE-03 (discard on fallback) → PR-F
- TD-STE-06 (ConcurrencySafe 注册表) → PR-A (per-input 函数)

无争议, 推进。

### 博弈点 4: PR 数量

| 立场 | 方案 | 理由 |
|------|------|------|
| Claude | 5 PR (合并 D+E) | "D/E 本质同一 PR, 拆开拉长回归期" |
| Cursor | 5 PR (合并 D+E) | "D/E 拆开会出现半成品时间窗" |
| Codex | 6 PR (维持) | "6 PR review 面更窄, 风险更可控" |

**争议本质**: 风险分散 vs 实现原子性。

**Cursor 加分理由**: devrix 现状 (Hotfix 模式 + 用户验收) 5 PR 足够, 不需要每个 PR 都能独立 review。
**Codex 加分理由**: classifier 这种高风险变更, 测试和实现应同 PR 原子出现 — 但这正是 Cursor 说的"实现与回归同 PR 原子", 跟 Cursor 立场实际一致。

**Codex 可能是误读需求**: 需求"PR-D (classifier 集成) + PR-E (测试 + telemetry + e2e)" — 这两个都是 classifier 相关的, **Codex 也同意 D+E 应同 PR 出现**, 但仍坚持 6 PR, 显得矛盾。

**建议裁决**: **5 PR** (合并 D+E), 接受 Claude+Cursor 一致立场。

### 博弈点 5a: GrowthBook 必要性

| 立场 | 方案 | 理由 |
|------|------|------|
| Claude | 降 P2 | "默认全关 = 死代码" |
| Cursor | **保留 P0** | "devrix 已有同类先例 (`internal/layers/contextengine/persist/growthbook_override.go:1-9, 57-89`), 是 baseline + runtime override 治理模式的横向复用" |
| Codex | **全删** | "GrowthBook 引入但不维护 = 技术债 +1" |

**Cursor 引用证据最硬**: 
```go
// internal/layers/contextengine/persist/growthbook_override.go:1-9
// Per-tool persistence threshold override.
// Use case: roll out the 100K per-tool thresholds progressively by
// changing the override map for the 5% canary first, then 25%, 100%.
// The hardcoded per-tool values in orthogonal_flags.go stay as the
// "consensus" baseline; GB can shift individual tools up or down.
```

**Codex 反驳 Cursor**: 即使有先例, "本 change 内 GrowthBook 是 P0 强制" + "默认全关" = 当下不工作, 跟 persist/ 的"已实战" 不同。

**Claude 折中**: 默认全关 + Production-Safety 实际上是 P0 死代码, 应**降 P2** (等有实战需要时再加)。

**建议裁决**: 倾向 **Codex 删 / Claude 降 P2**, 反对 Cursor P0。
- 节省 AC11 (GrowthBook override_test + 19 工具 default + Production-Safety) ≈ 1 周工时
- T25 (GrowthBook 集成) 同步降级

### 博弈点 5b: inputsEquivalent 价值

| 立场 | 方案 |
|------|------|
| Claude | 降 P3 或删 |
| Cursor | 降 P3 |
| Codex | 保持 P2 |

三方接近 (Claude+Cursor 降 P3, Codex P2, 实际一致) — 都认为价值不足, 只是 P2 vs P3 之争。

**关键事实**: devrix `ContentReplacementState` (DM-20260702-008 已落地, `internal/layers/contextengine/persist/content_replacement_state.go:14-23, 81-118`) 用 `toolUseID` 冻结单位, **不依赖输入等价判定**。`inputsEquivalent` 是它的弱化版。

**建议裁决**: **降 P3** (Claude + Cursor 一致), AC14 走 backlog。

---

## 4. 综合 scope 建议

**裁剪后 (vs 需求原 14 AC)**:

| AC | 原 | 建议 | 理由 |
|----|-----|------|------|
| AC1-AC3 | 保留 | ✅ P0 | per-input 函数 + partitionToolCalls 治本 |
| AC4-AC7 (auto-mode classifier) | P0 实施 | ⚠️ **只加 interface, P2 实施** | 节省 ~1 周; Codex 论据有力 |
| AC5 (telemetry) | 保留 P0 | ⚠️ **降到 P2 (跟 classifier 一起)** | classifier 不实施, telemetry 无意义 |
| AC6 (fail-safe) | 保留 P0 | ✅ P0 | 治本前提, 必须有 |
| AC7 (Bash isReadOnly) | 保留 P0 | ✅ P0 | 治本核心, 三方一致 |
| AC8 (no silent default) | 保留 P0 | ✅ P0 | 跟 per-input 函数配套 |
| AC9-AC10 (13 T 全实施 + e2e) | 保留 | ✅ P0 | 基础达标 |
| **AC11 (GrowthBook)** | P0 实施 | ❌ **降 P2** | 死代码争议, Cursor 论据不充分 |
| AC12 (Bash sibling abort) | P1 | ✅ P1 | TD-STE-02 收口 |
| AC13 (Discard on fallback) | P1 | ✅ P1 | TD-STE-03 收口 |
| **AC14 (inputsEquivalent)** | P2 | ❌ **降 P3** | ContentReplacementState 已覆盖, 价值低 |

**最终 scope**: **11 P0 + 2 P1 + 1 P2 (interface only)**, 14 AC → 12 AC, 13 T → 12 T (T25 GrowthBook 降 P2, T28 inputsEquivalent 降 P3)。

**PR 拆分**: 5 PR (Claude+Cursor 一致, Codex 反对)
- **PR-A**: ToolSurface v4 + 19 工具 per-input 函数 (含分层混合) — 3 T
- **PR-B**: partitionToolCalls 改造 + 50 文件 e2e — 2 T
- **PR-C**: ToAutoClassifierInput + 19 工具默认 — 2 T
- **PR-D+E (合并)**: ClassifyToolUse interface (no SideQuery 实施) + telemetry + e2e — 3 T
- **PR-F**: GrowthBook (P2) + Bash sibling abort + Discard on fallback + inputsEquivalent (P3) — 4 T

---

## 5. 待用户裁决的 4 个差异点

| # | 项 | 三方立场 | 默认建议 | 替代方案 |
|---|----|---------|---------|---------|
| **D1** | per-input 函数实现 | Claude/Cursor=全函数, **Codex=分层混合** | **Codex (分层混合)** | 全函数 (15 工具多写 ~600 行) |
| **D2** | auto-mode classifier 优先级 | Claude=Cursor=P0, **Codex=P2 (interface only)** | **Codex (P2 interface only)** | P0 实施 + 默认关 (需求现状) |
| **D3** | GrowthBook 必要性 | Claude=降P2, Cursor=P0, **Codex=全删** | **Claude 降 P2 (等实战再加)** | 删 / 保留 P0 |
| **D4** | PR 数量 | Claude=Cursor=5, **Codex=6** | **5 PR (合并 D+E)** | 6 PR |

**用户请就 D1-D4 给出明确选择, 之后我 (Claude) 推进到 S2 提案 + S3 设计阶段。**

---

## 6. 借鉴关系最终评分 (三方共识)

| clawcode 字段 | devrix 现状 | 借鉴评分 | 实施路径 |
|--------------|-----------|---------|---------|
| `isConcurrencySafe(input)` | 静态 bool 字段 | ★★★★★ | PR-A (per-input 函数, 分层混合) |
| `toAutoClassifierInput` | 无 | ★★★★★ | PR-C (per-tool 紧凑投影) |
| `siblingAbortController` | 无 (TD-STE-02) | ★★★★★ | PR-F (T26) |
| `discard()` | 无 (TD-STE-03) | ★★★★ | PR-F (T27) |
| `interruptBehavior` | **已有 `InterruptMode` (1:1)** | — | 不借鉴 |
| `isReadOnly` | **已有 `ReadOnly` 字段** | — | 不借鉴 |
| `maxResultSizeChars` | **已有 v3 字段** | — | 不借鉴 |
| `shouldDefer` | **已有 `DeferLoading`** | — | 不借鉴 |
| `isDestructive` | **已有 `Destructive`** | — | 不借鉴 |
| `inputsEquivalent` | 无 | ★★ (P3) | PR-F (T28, 降 P3) |
| `yoloClassifier` | 无 | ★★★ (P2) | PR-D (interface only) |
| `toCompactBlock` | 无 | ★★★ (P2) | PR-D (interface only) |
| `extractSearchText` / `requiresUserInteraction` / `isTransparentWrapper` 等 | 无 | <★★ | 不借鉴 |

**借鉴效率**: 需求暗示 10 项, 实际**只有 4 项真正需要从 clawcode 学** (isConcurrencySafe / toAutoClassifierInput / siblingAbortController / discard), 其他 5 项是 devrix 已有, 2 项 (yoloClassifier / toCompactBlock) 走 P2。

---

## 7. 关键风险 (三方共识排序)

| 序 | 风险 | 缓解 |
|----|------|------|
| 1 | `BashTool.isReadOnly` 误判 (compound `ls; rm -rf`) | parse 整个 command tree (仿 clawcode `bashSecurity.ts`); 不可靠时保守 false; 加 `isReadOnlyPanics` metric |
| 2 | `IsConcurrencySafe` 抛错 → turn 崩溃 | AC6 fail-safe (catch + return false), 强制 |
| 3 | `partitionToolCalls` 改造破坏现有并发行为 | AC1 强制 19 工具默认保持 v2 静态行为; `surface_metadata_gate_test` AC8 case 回归 |
| 4 | SideQuery LLM 不可用 (网络/CK) → fail-open | 5s timeout + metric `auto_mode.classifier_unavailable` (P2 阶段再开) |
| 5 | `ToAutoClassifierInput` 抛错 → turn 崩溃 | fail-safe: catch + emit metric + fall back to raw input |
| 6 | transcript 序列化 leak PII | toCompactBlock 只投影 tool_use 块, 不投影 tool_result 内容 (跟 clawcode 一致) |
| 7 | 6 PR 延期 (DM-20260702-008 9T 延期先例) | 5 PR 缩到 1W + 1D; DoR 门槛 + 跨 PR 依赖合并后才开 |

---

## 8. 共识诉求 (三方一致)

1. **per-input 函数治本** (静态 bool 必弃) — 三方一致, 区别在"全函数" vs "分层混合"
2. **VerifyContract 4 元组是 ground truth, 不可被 auto-mode 替代** — 三方一致
3. **4 项 tech-debt 同 change 收口** — 三方一致
4. **fail-safe 是硬性要求** (抛错 → false, 不上抛) — 三方一致
5. **借鉴 clawcode 35 字段中实际只有 4 项需学** (Codex 发现, Claude + Cursor 接受) — 三方一致
6. **0 业务代码 out-of-scope diff** — 三方一致 (跟 DM-20260702-008 同样的纪律)

---

## 9. 差异点摘要 (需用户裁决)

- **D1**: per-input 实现 (全函数 vs 分层混合)
- **D2**: auto-mode classifier 优先级 (P0 vs P2 interface only)
- **D3**: GrowthBook (保留 P0 vs 降 P2 vs 全删)
- **D4**: PR 数量 (5 vs 6)

**默认建议**: D1=Codex (分层混合), D2=Codex (P2 interface only), D3=Claude (降 P2), D4=5 PR
**用户请就 4 个差异点给出明确选择**, 我将基于裁决推进到 S2 提案 + S3 设计阶段。
