# Delta: D7 Orchestration — Migration Retired

**Change ID:** `devrix-d1-d7-only-ingress`  
**Affects:** Migration Coexistence Contract

---

## MODIFIED

### Requirement: D7-D1 Primary Entry

`orchestration.d7_enabled` MUST default to `true`. When `false`, process startup MUST fail (no silent fallback to D1→D2).

#### Scenario: Startup requires D7

- GIVEN `d7.enabled=false` in config
- WHEN `devrix` main initializes bootstrap
- THEN startup fails with explicit error
- AND D1 legacy path is NOT available

---

## REMOVED

### Requirement: Migration Coexistence — legacy D1→D2 row

- REMOVED `d7_enabled=false | * | D1→D2.Process` matrix row
- REMOVED bit-identical rollback promise for D1→D2 (superseded by DM-20260614-007)
