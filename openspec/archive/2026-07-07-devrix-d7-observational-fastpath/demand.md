# Demand: D7 Observational-Answer Fast-Return

**Demand ID**: DM-20260706-011
**Change ID**: devrix-d7-observational-fastpath
**Title**: D7 Observational-Answer Fast-Return — skip Plan+Execute+Verify on high-strength CatBusiness ObsFact
**Priority**: P0
**Submitter**: devrix-orchestration team
**Date**: 2026-07-07

## Problem

Trivial deterministic Q&A ("1+1=几?", "法国首都是哪?") runs through the full
Observe → Plan → Execute → Verify → Learn pipeline, costing ~3-5s and 3 LLM
calls per round. The Observe node already knows the answer at strength ≥ 0.85
but the user waits for Plan + Execute + Verify to materialise it.

## Required Outcome

When the observer is confident (CatBusiness ObsFact, strength ≥ 0.85) AND the
report carries no open user-facing questions, emit ObsFact.Statement directly
as the user-visible finalText. Learn still runs so reputation scoring keeps
observing the observer's accuracy. Target latency: ≤ 1.5s for trivial Q&A
(down from 3-5s).

## Out of Scope

- Multi-intent decomposition (separate change DM-20260707-001)
- Cross-round answer caching
- LLM-bypass verification (reputation BayesianUpdate is the substitute)
- Configurable per-tenant threshold knob

## Success Criteria

- 9 unit tests PASS (gate conditions + Learn + persistence + source filter)
- 27/27 orchestration packages `go test -race` PASS
- `go vet ./...` 0 warning
- Manual trivial Q&A round-trip latency ≤ 1.5s in dev
- Manual complex directive (multi-intent) still falls through to Plan path

## Stakeholders

- devrix D7 orchestration team (D7 owner)
- D2 context engine (i18n prompt touch)
- D5 observability (hardening span wiring)
- D7 Learn (reputation BayesianUpdate)