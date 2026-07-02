# Delta: D5 Observability — Termination LTL-Lite L4–L6

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Affects:** D5-S25 (new) — Termination

---

## ADDED

### Requirement: D5-S25 Termination LTL-Lite — 4 invariant

`internal/layers/observability/instrument/ltl/invariants/termination/` 包 SHALL 提供 3 termination invariant + 1 L7 umbrella:

- **L4 BoundedInvariant**: `Check(state) → (ok, reason)`, iter ≥ MaxN 时返 false + "Bounded exceeded iter N/MaxN, inject synthesize-now"
- **L5 QuotientInvariant**: `Check(state) → (ok, reason)`, metric(state) < Threshold 时返 false + "Quotient below threshold X/Y, inject synthesize-now"
- **L6 SynthesizeInvariant**: `Check(state) → (ok, reason)`, len(text) < MinChars 时返 false + "Synthesize too short X < MinChars, inject synthesize-now"
- **L7 umbrella**: `L7-FACT-SAME-Q-5x` (Fact 工具同 query 5x 升级 Probe) / `L7-ACTION-POSTSNAPSHOT` (PostSnapshot≠PreSnapshot 才 Verifiable) / `L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE` (deadline < ConcludedAt 校验)

#### Scenario: BoundedInvariant 触发
- GIVEN state.Iter = 16, MaxN = 15
- WHEN Check
- THEN 返 (false, "Bounded exceeded iter 16/15, inject synthesize-now")
- AND 不 override L0–L3 readonly/permission guards (cross-check)

#### Scenario: QuotientInvariant 触发
- GIVEN metric(state) = 0.6, Threshold = 0.8
- WHEN Check
- THEN 返 (false, "Quotient below threshold 0.6/0.8, inject synthesize-now")

#### Scenario: SynthesizeInvariant 触发
- GIVEN len(text) = 5, MinChars = 20
- WHEN Check
- THEN 返 (false, "Synthesize too short 5 < 20, inject synthesize-now")
