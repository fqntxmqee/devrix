# Devrix Project Context

Multi-agent development assistant. 6-layer architecture, OpenSpec S1-S6 workflow.

## Architecture

| Domain | Domain | Path |
|----|-------|------|
| D1 | Communication | `internal/layers/communication/` |
| D2 | Context Engine | `internal/layers/contextengine/` |
| D3 | LLM Gateway | `internal/layers/llmgateway/` |
| D4 | Multi-Agent | `internal/layers/multiagent/` |
| D5 | Observability | `internal/layers/observability/` |
| D6 | Evolution | — |

## Development Workflow (OpenSpec S1-S6)

```
S1 Demand → S2 Proposal → S3 Design → S3-Gate(Review) → S4 Implementation → S4-Gate(Review) → S5 Acceptance → S6 Archive
```

## Stage → Spec Routing

Before starting any task, determine the stage and load the corresponding spec from `openspec/specs/project/`:

| Stage | Specs to Load | Gate |
|-------|---------------|------|
| S1 Demand | `requirements.md` | DM ID valid |
| S2 Proposal | `requirements.md` + `architecture-design.md` | Files complete |
| S3 Design | `architecture-design.md` | — |
| S3-Gate | `review-design.md` | Design review passed |
| S4 Implementation | `coding.md` + `testing.md` | go vet + test-unit |
| S4-Gate | `review-code.md` | Code review passed |
| S5 Acceptance | `testing.md` | P0 L5 100% + coverage ≥ 80% |
| S6 Archive | `archiving.md` | Archive checklist |

Authority: `openspec/specs/project/master.md`

## Key Rules

- **Errors**: `internal/shared/errors/` SentinelError pattern. Do NOT use `panic` for business errors.
- **Config**: `devrix.yaml` (default) → `config.yaml` (local) → env vars → CLI flags
- **Immutability**: Create new objects. Do NOT mutate in place.
- **File size**: Functions < 50 lines, files < 800 lines
- **Git**: GitHub Flow, `feat/<change-id>` branches, squash merge
- **L5 tests**: Format `L5-{D}-{S}-{NN}`, registered in `openspec/l5-registry.md`
- **Changes**: `openspec/changes/<change-id>/`, archived to `openspec/archive/<YYYY-MM-DD>-<change-id>/`
