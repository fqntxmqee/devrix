# Design: D7 Historical S Cleanup

## Principles

- **Current vs historical separation**: canonical SoT = S1–S6 in spec + registries; historical = read-only appendix.
- **No T renumbering**: former S prefixes remain in T IDs and historical doc only.
- **S3 stays canonical**: WaveScheduler remains D7-S3 as an independent explicit-invocation scenario.

## S3 Boundary

| Path | Trigger | S layer |
|------|---------|---------|
| User IM message | ProcessMessage → RunSessionTurnLoop → ItemPipelineRunner | S2 + S5 + S6 (+ S1 WorkTree) |
| Delegate / Plan / background wave | WaveScheduler.Start | S3 (parallel to main ingress) |

S3 writes WorkItem/TaskGraph state and publishes via S4, but is **not** entered by default on every user message.

## Document Layout

```
openspec/specs/d7-orchestration/
  spec.md                 # canonical S1-S6 + summary mapping table
  a-registry.md           # canonical A for S1-S6 only
  f-registry.md           # canonical F for S1-S6 only
  historical-s-mapping.md # former S7-S21 detail + F index
  t-registry.md           # unchanged historical T IDs + new D7-HC T section
```

## Guard Rules

- `a-registry.md` must not contain `### Historical: D7-S7` or `## D7-S20`
- `f-registry.md` must not contain `## D7-S8-A15` or `fastpath.go`
- `spec.md` must reference `historical-s-mapping.md` and S3 explicit-wave wording

## Tests

Extend `TestD7MainPath_CanonicalSLayerNormalized` in `main_path_arch_test.go`.
