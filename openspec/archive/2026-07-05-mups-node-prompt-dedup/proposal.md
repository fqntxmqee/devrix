# Proposal: MUPS 三节点 Prompt 去冗余

**Change ID:** `mups-node-prompt-dedup`  
**Demand:** DM-20260705-004
**Status:** S7_Archived (2026-07-05)

## Capabilities

| Capability | Change |
|------------|--------|
| Observe user frame | LLM-visible fields only; no plane line prefixes |
| Plan user frame | No plane line prefixes; guide lists present fields only |
| Execute system | Task body before output hints; Execute labels i18n |
| Semantic appendix | Observe/Plan skip duplicate node_role; Execute skip execute.node_role |

## Non-goals

- PrepareBase phase-specific trimming (separate demand)
