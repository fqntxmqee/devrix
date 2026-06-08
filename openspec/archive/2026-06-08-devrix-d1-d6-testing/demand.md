# Demand: D1 & D6 Testing Coverage

**Demand ID:** DM-20260608-011
**Change ID:** devrix-d1-d6-testing
**Date:** 2026-06-08
**Type:** Testing Enhancement
**Priority:** P1

## Context

After the test coverage review (see `openspec/changes/devrix-layering-standard/coverage-review.md`), two domains were identified with **0% L5 test coverage**:

| Domain | Current Coverage | Risk |
|--------|-----------------|------|
| D1 Communication | 0/5 PLANNED | **HIGH** - User-facing entry point |
| D6 Evolution | 0/2 PLANNED | LOW - Optional enhancement |

D1 is the user interaction entry point; lack of tests could lead to critical UX bugs.

---

## Problems

### D1 Communication (5 PLANNED, 0 IMPLEMENTED)

| Problem | Impact |
|---------|--------|
| No command parsing tests (/new, /help, /stop) | Commands may silently fail or behave unexpectedly |
| No session creation/rejection tests | Gateway rejection scenarios unverified |
| No Feishu message parsing tests | Inbound message format errors uncaught |
| No ShortId uniqueness tests | Duplicate IDs may cause session conflicts |
| No Gateway flow tests | Integration between components unverified |

### D6 Evolution (2 PLANNED, 0 IMPLEMENTED)

| Problem | Impact |
|---------|--------|
| No version detection tests | Version reporting may be incorrect |
| No config hot-reload tests | Runtime config updates may fail silently |

---

## Goals

### Primary (Must Fix)

1. **D1-S3 Commands**: Implement L5-1-3-01, L5-1-3-02, L5-1-3-03 test cases (3 tests)
2. **D1-S1 Gateway**: Implement L5-1-1-01 test case (session rejection) (1 test)
3. **D1-S2 Adapters**: Implement L5-1-2-01 test case (Feishu parsing) (1 test)
4. **D1-S8 Renderers**: Implement L5-1-8-01 test case (ShortId uniqueness) (1 test)

### Secondary (Should Fix)

5. **D6-S1 Version**: Implement L5-6-1-01 test case (1 test)
6. **D6-S2 Config**: Implement L5-6-2-01 test case (1 test) **[DEPENDS ON: config.LoadAndWatch implementation]**

---

## Success Criteria

### D1 Coverage Target: 6/6 IMPLEMENTED (6 tests total)

| L5 ID | Test Case | Acceptance Criteria |
|-------|-----------|---------------------|
| L5-1-3-01 | /new command parsing | Command extracted, current session terminated, new session created |
| L5-1-3-02 | /help command parsing | Help text returned, no session interaction |
| L5-1-3-03 | /stop command parsing | LLM call cancelled, partial response preserved |
| L5-1-1-01 | Session rejection | CLI session rejected with appropriate error code |
| L5-1-2-01 | Feishu message parsing | Message struct correctly extracted from payload |
| L5-1-8-01 | ShortId uniqueness | All generated IDs unique, no confusing characters |

### D6 Coverage Target: 2/2 IMPLEMENTED

| L5 ID | Test Case | Acceptance Criteria |
|-------|-----------|---------------------|
| L5-6-1-01 | Version detection | Version string correctly extracted and formatted |
| L5-6-2-01 | Config hot-reload | Config changes reflected without restart |

---

## Scope

### In Scope
- Unit tests for command parsing logic
- Unit tests for ShortId generation
- Unit tests for Feishu message parsing
- Integration tests for Gateway session flow
- Unit tests for version detection
- Unit tests for config hot-reload (if implemented)

### Out of Scope
- E2E tests (deferred to separate change)
- Performance tests (covered by D5)
- Security penetration tests (covered by existing shell_injection tests)

---

## Dependencies

| Dependency | Reason |
|------------|--------|
| `internal/layers/communication/commands/` | Command parsing implementation must exist |
| `internal/layers/communication/adapters/feishu.go` | Feishu adapter must exist |
| `internal/layers/communication/gateway/` | Gateway session management must exist |
| `internal/shared/types/shortid.go` | ShortId generation must exist |

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Implementation not started | Low | Tests can't be written | Check existing code first |
| APIs unstable | Medium | Tests may break | Use interface-based testing |
| Fixtures needed | Medium | Setup complexity | Use table-driven tests |

---

## Estimated Effort

| Domain | Test Cases | Estimated LOC | Priority |
|--------|------------|--------------|----------|
| D1 Commands | 3 | ~150 | P0 |
| D1 Gateway | 1 | ~80 | P0 |
| D1 Adapters | 1 | ~100 | P1 |
| D1 Renderers | 1 | ~60 | P1 |
| D6 Version | 1 | ~40 | P2 |
| D6 Config | 1 | ~60 | P2 |
| **Total** | **8** | **~490** | — |

---

## Open Questions

1. Should D1 tests use mock adapters or real Feishu WebSocket connection?
2. Is ShortId generation deterministic or random? (Affects test design)
3. Is config hot-reload implemented or still planned?
