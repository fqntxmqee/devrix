# D1 Communication — Spec Delta (2026-06-20-devrix-context-budget-and-isolation)

**Change ID:** 2026-06-20-devrix-context-budget-and-isolation
**Demand ID:** DM-20260620-001
**Base:** `openspec/specs/d1-communication/spec.md` v3.x
**Scope:** AC5 — Feishu card table-count / size precheck + plain-text fallback

---

## MODIFIED: Card Send Path (D1-S5-A07 SendOutboundCard)

The Feishu card send path (`FeishuAdapter.SendCard` and
`sendCardToSession`) MUST consult a `CardContentPrecheck` before invoking
the Feishu cardkit API. On precheck failure the adapter MUST fall back
to a plain-text representation rather than retrying into a silent
rejection loop (Feishu ErrCode 11310, 30 KB hard limit).

#### Scenario: AC5 Table-count precheck

- GIVEN a card JSON containing more than `MaxTablesPerCard` (default 5)
  `<table>` elements
- WHEN `FeishuAdapter.SendCard` is called
- THEN the precheck fires before the API call
- AND the adapter falls back to `SendMessage` with a flattened plain-text
  representation of the card
- AND the API is never invoked for the over-limit payload

<!-- T: D1-S5-A07-T05 -->

#### Scenario: AC5 Size precheck

- GIVEN a card JSON whose length exceeds `MaxCharsPerCard` (default 28 000)
- WHEN `FeishuAdapter.SendCard` is called
- THEN the precheck fires before the API call
- AND the adapter falls back to `SendMessage` with the same content
  truncated and a "[card auto-flattened]" marker

<!-- T: D1-S5-A07-T06 -->

#### Scenario: AC5 Default precheck wired in WireD7

- GIVEN the devrix binary is started with no custom precheck configured
- WHEN `NewFeishuAdapter` runs
- THEN `cardPrecheck` is initialised to
  `NewFeishuTableCountPrecheck(DefaultCardPrecheckConfig())`
- AND the default config is `MaxTablesPerCard=5, MaxCharsPerCard=28000`

<!-- T: D1-S5-A07-T07 -->

#### Scenario: AC5 Plain-text fallback preserves header + markdown

- GIVEN a `kernel.Card` with `Header.Title` and one `CardMarkdown` element
- AND a precheck failure (e.g. `ErrTooManyTables`)
- WHEN `cardFallbackText` is called
- THEN the returned string contains the header title, the markdown
  content, and a "[card auto-flattened]" trailer with the precheck
  error message

<!-- T: D1-S5-A07-T08 -->
