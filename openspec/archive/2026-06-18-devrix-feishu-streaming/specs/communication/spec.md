# Communication Layer Delta — Feishu 2.0 Streaming

**Change:** devrix-feishu-streaming (DM-20260611-006)  
**Base:** 无独立 `openspec/specs/communication/spec.md`（本 delta 归档时创建主规格 v1.0.0）

## ADDED Requirements

### Requirement: Feishu Cardkit Card Entity

When `im.feishu.streaming.enabled=true`, the Feishu adapter MUST create a cardkit card entity via `POST /open-apis/cardkit/v1/cards` before sending the first reply card of a user turn, and MUST send the IM message referencing `{"type":"card","data":{"card_id":"<id>"}}`.

**Priority:** P0  
**L3:** L3-BE-COM-01  
**L4:** L4-BE-COM-cardkit

#### Scenario: First reply creates card entity

- GIVEN streaming enabled and no `replyCardID` for the session
- WHEN the first `event_type=text` chunk arrives
- THEN adapter calls cardkit Create with JSON 2.0 card (`streaming_mode: true`, `element_id: reply_text`)
- AND replies with card_id reference message
- AND stores `cardID` on session stream

#### Scenario: Cardkit create fails

- GIVEN streaming enabled and cardkit Create returns error
- WHEN the first text chunk arrives
- THEN adapter logs WARN and falls back to inline card JSON + `Im.Message.Patch` path
- AND sets `cardkitEnabled=false` for the remainder of the turn

---

### Requirement: Feishu Element-Level Streaming Update

When `cardkitEnabled=true`, reply text updates MUST use `PUT /open-apis/cardkit/v1/cards/{card_id}/elements/reply_text/content` with monotonically increasing `sequence`, passing the **accumulated full text** (not delta).

**Priority:** P0  
**L4:** L4-BE-COM-stream-reply

#### Scenario: Streaming chunks update element

- GIVEN an active `cardID` and three text chunks "A", "B", "C"
- WHEN chunks are processed (after throttle)
- THEN element PUT is called with content "A", then "AB", then "ABC"
- AND sequence values strictly increase (e.g. 1, 2, 3)

#### Scenario: Rate limit 230020

- GIVEN element PUT returns code 230020
- WHEN a throttled flush occurs
- THEN adapter skips the frame without disabling cardkit
- AND a later flush retries with higher sequence

---

### Requirement: Feishu Streaming Throttle

The adapter MUST throttle element PUT calls using configurable `interval_ms` (default 400) and `min_delta_chars` (default 24). A final flush on `event_type=complete` MUST NOT be throttled away.

**Priority:** P1  
**L4:** L4-BE-COM-stream-reply

#### Scenario: High-frequency chunks coalesced

- GIVEN `interval_ms=400` and chunks arriving every 50ms
- WHEN fewer than 400ms elapsed since last PUT
- THEN no element PUT is sent until interval elapses or complete fires

---

### Requirement: Feishu Streaming Completion

On `event_type=complete`, when `cardkitEnabled=true`, the adapter MUST perform a final element PUT with full text, then `PUT /open-apis/cardkit/v1/cards/{card_id}` with `streaming_mode: false` and optional footer content.

**Priority:** P0  
**L4:** L4-BE-COM-stream-reply

#### Scenario: Complete closes streaming

- GIVEN cardkit streaming active with partial reply text
- WHEN complete event arrives with summary footer
- THEN final element PUT contains full reply text
- AND card entity update sets `streaming_mode: false`
- AND footer (elapsed/tokens) is visible in the final card

---

### Requirement: Feishu Streaming Config Kill Switch

When `im.feishu.streaming.enabled=false`, behavior MUST be identical to pre-change Patch-only streaming (no cardkit calls).

**Priority:** P0  
**L4:** L4-BE-COM-stream-reply

#### Scenario: Streaming disabled

- GIVEN `streaming.enabled=false`
- WHEN text chunks arrive
- THEN only `Im.Message.Reply` + `Patch` are used
- AND no cardkit API is invoked

---

## MODIFIED Requirements

### Requirement: Feishu Reply Card Markdown

Reply card markdown content MUST be passed through without legacy header conversion (`###` → broken `**`). JSON 2.0 native markdown syntax (headings, tables, code blocks, bold) MUST be preserved.

**Priority:** P1  
**Note:** 已在代码中修复（PreprocessMarkdown 透传）；本变更不回归。

#### Scenario: Markdown passthrough

- GIVEN reply content `### Title\n\n**bold**`
- WHEN card JSON is built
- THEN markdown element content equals input unchanged
