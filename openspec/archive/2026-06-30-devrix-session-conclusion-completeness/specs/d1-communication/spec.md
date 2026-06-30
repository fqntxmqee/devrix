# Spec Delta — D1-Communication — Session Conclusion Completeness

**Change ID:** `devrix-session-conclusion-completeness`
**Target Spec:** `openspec/specs/d1-communication/spec.md`
**Target Version:** see `openspec/specs/d1-communication/CHANGELOG.md` (incremented on merge)
**Demand ID:** DM-20260630-011
**Created:** 2026-06-30
**Status:** S4_Implementation

---

## Why this delta

Sess `sess_1782814140202_7000` 飞书回复卡片出现重复内容（LLM streaming 重放前缀 + 末尾 recap）+ 无正常 review 结果。Bug A（dedup threshold）+ B/C/D/E 经 S1-S4 评估为结构性数据缺口，不是阈值调优问题。本次 Change 治本 4 个跨域 Bug：

1. **D1 conclusion.EmitComplete 不感知 summary 质量** — 当 D7 LastTextQualityGate 分类为 `too_short` / `inconclusive` 时，D1 仍然把截断/模板化的 summary 渲染到飞书卡片上。
2. **D2 Materialize span 缺少真实计数回填** — `materialize.message_count` / `token_est` 在 span start 时为 0/0，运营侧无法在 Jaeger 上看到 WorkItem 是否实际有内容。
3. **D7 编排层硬编码 `learn.classifier_source="rule"`** — 一旦未来接入 LLM classifier，observability 会失真。
4. **D7 缺少 LLM 末次文本的结构化质检** — Devrix v6.0.x 一直未在 IM 派发前评估 summary 是否被 planning/template 污染。

本次 delta 聚焦 D1 域：EmitComplete 显式化 fallback chain + 新增 `D1_EmitComplete_Fallback` 可观测 span。

---

## ADDED Requirements

### D1-S16-A72: Summary Quality Fallback Chain

When the terminal `complete` event from D7 carries `summary_quality ∈ {too_short, inconclusive}` (LastTextQualityGate classification), `EmitComplete` MUST replace the rendered Content with `event.Content` rather than the truncated/recap-leaked summary. The original summary is preserved on `meta["summary"]` for observability / CLI / transcript; the fallback is recorded via the new `D1_EmitComplete_Fallback` span for dashboard alerting on abnormal fallback rates.

#### Scenario: too_short summary falls back to event.Content

- **Given** terminal `complete` event with `meta["summary"]="ok"` (2 chars), `meta["summary_quality"]="too_short"`, `event.Content="[full multi-turn transcript]"`
- **When** `EmitComplete` is called
- **Then** rendered `OutboundMessage.Content` SHALL equal `event.Content` (not the truncated summary)
- **And** `meta["summary"]` SHALL preserve `"ok"` for CLI / transcript observability
- **And** a `D1_EmitComplete_Fallback` span SHALL be emitted with `fallback.source="event.Content"`, `summary_quality="too_short"`

#### Scenario: inconclusive summary (planning marker) falls back to event.Content

- **Given** `meta["summary"]="<scope_contract> scope ..."`, `meta["summary_quality"]="inconclusive"`
- **When** `EmitComplete` is called
- **Then** rendered Content SHALL equal `event.Content`
- **And** a `D1_EmitComplete_Fallback` span SHALL be emitted with `summary_quality="inconclusive"`

#### Scenario: valid summary preserves summary on Content (no fallback)

- **Given** `meta["summary"]="代码审查完成 ..."`, `meta["summary_quality"]="valid"`
- **When** `EmitComplete` is called
- **Then** rendered Content SHALL equal `meta["summary"]`
- **And** `D1_EmitComplete_Fallback` span SHALL NOT be emitted

#### Scenario: empty summary + empty Content falls back to stats

- **Given** `meta["summary"]=""`, `meta["summary_quality"]="too_short"`, `event.Content="   "`
- **When** `EmitComplete` is called
- **Then** rendered Content SHALL equal the `BuildCompletionSummary` stats line
- **And** a `D1_EmitComplete_Fallback` span SHALL be emitted with `fallback.source="stats"`

---

## Test Point Mapping

| T Point             | Description                                    | File                                                                  |
| ------------------- | ---------------------------------------------- | --------------------------------------------------------------------- |
| D1-S16-T01 (AC2)    | 4 fallback scenarios (too_short/inconclusive/empty/stats) | conclusion_test.go:TestEmitComplete_* |

---

## Out of Scope (per DM-20260630-011 §7)

- **飞书卡片 dedup (Bug A)** — threshold 调优是 anti-pattern；通过 `D7_LastText_Quality_Gate` 在 D7 源头分类为 `too_short` 让 D1 走 fallback，从根本上避免 dedup 触发。
- **streaming replay (Bug B)** — 已在 PR #139 通过 `detectDuplicateReplay` 处理。
- **feishu card streaming closed (Bug C)** — 已在 PR #138 通过 `UpdateCard` fallback 处理。

---

## Files Modified

- `internal/layers/communication/conclusion/conclusion.go` — EmitComplete 增加 summary_quality 感知 + EmitEmitCompleteFallback span
- `internal/layers/observability/instrument/telemetry/names.go` — 新增 `OpD1_S16_EmitComplete_Fallback = "D1_EmitComplete_Fallback"`
