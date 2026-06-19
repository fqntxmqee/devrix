# D2 Context Engine — Spec Delta (devrix-d2-structure-closure)

**Change ID:** devrix-d2-structure-closure  
**Demand ID:** DM-20260619-007  
**Base:** `openspec/specs/d2-context-engine/` v8.1.0 → v8.2.0

---

## MODIFIED: Scenario Orchestrator as Production SoT

D2 S15/S17/S18 orchestrators MUST be the production entry points for Prepare/Persist/ToolRound operations. The legacy `facade/` package (renamed to `legacy/` in v8.2) is deprecated and MUST NOT be used by new code.

#### Scenario: Prepare flows through `prepare.Orchestrator.Prepare`

- GIVEN `devrix-d2-structure-closure` P1-d is merged
- WHEN a Turn's Prepare is invoked
- THEN the path is `bootstrap.turn_adapter.Prepare` → `prepare.Orchestrator.Prepare` (NOT `facade.ContextEngine.Process`)
- AND `legacy.ContextEngine.Process()` is not called in production

<!-- T: D2-S15-A01-T01 -->

#### Scenario: Persist flows through `persist.Orchestrator` with CommitWindow

- GIVEN P1-e is merged
- WHEN a Turn's Persist is invoked
- THEN the path is `bootstrap.turn_adapter.PersistTurn` → `persist.Orchestrator.Persist` → `persist.commitWindow` (messages-only seven-step pipeline)

<!-- T: D2-S17-A04-T01 -->

#### Scenario: ToolRound flows through `enforce.tools.Registry`

- GIVEN P3-T2 is merged
- WHEN ExecuteToolRound is invoked
- THEN the path is `enforce` subpackages (permission, registry, tools, sandbox) — NOT `enforce.toolrunner` (which no longer exists)

<!-- T: D2-S18-A03-T01 -->

---

## MODIFIED: Physical Path Registration (v2.2 Final)

D2 sub-directory structure MUST align with `architecture/code-layout.md` §4.3 v2.2 final paths and `architecture/layering.md` v4.7.0 D2 tree.

#### Scenario: tools package at enforce/tools slug

- GIVEN P3-T2 is merged
- WHEN a developer locates `ToolRegistry` or surface implementations
- THEN the package lives under `internal/layers/contextengine/enforce/tools/`
- AND package name is `package tools` (not `package toolrunner`)

<!-- T: D2-STRUCT-T03 -->

#### Scenario: WorkerDirSandbox under enforce/sandbox

- GIVEN P3-T1 is merged
- WHEN a developer locates `WorkerDirSandbox` or `Manager.Enter/Exit`
- THEN the path is `internal/layers/contextengine/enforce/sandbox/` (NOT `contextengine/sandbox/`)

#### Scenario: Memory split — Recall in prepare, Store in persist

- GIVEN P4 is merged
- WHEN a developer locates LongTerm memory operations
- THEN `Recall` lives in `contextengine/prepare/memory/recall.go` (S15)
- AND `Store` lives in `contextengine/persist/memory/store.go` (S17)
- AND `MemoryEntry` lives in `internal/shared/types/memory.go` (no domain owns)
- AND `LongTermRecaller` / `LongTermStore` ports live in `internal/shared/contracts/memory.go`

<!-- T: D2-STRUCT-T04 -->

---

## MODIFIED: Layout Guards (D2-STRUCT-T01..T07)

Seven new layout guards MUST be IMPLEMENTED in `internal/lint/layer/d2_layout_test.go`:

| T ID | Description |
|------|-------------|
| **D2-STRUCT-T01** | 根目录生产文件仅 `contracts.go` + `aliases.go` |
| **D2-STRUCT-T02** | 无 `engine_persist.go` 在根或 `facade/` 外（功能归属 `persist/commit.go`） |
| **D2-STRUCT-T03** | `enforce/tools/` 包名为 `package tools`（禁止 `package toolrunner`） |
| **D2-STRUCT-T04** | `prepare/memory/` 与 `persist/memory/` 无循环依赖 |
| **D2-STRUCT-T05** | `enforce/orchestrator.go` 已删除（dispatch 由 `turn_adapter` 接管） |
| **D2-STRUCT-T06** | scenario 下目录深度 ≤2 层（`enforce/tools/surface/` ✅；更深需 F-registry 登记） |
| **D2-STRUCT-T07** | P5 legacy 退役：禁止新增 `legacy.ContextEngine.Process()` 生产引用（CI 硬阻断）；allowlist 包含 8 个已知 caller（cmd/llm-smoke + multiagent/run + tests/* + communication mocks） |

#### Scenario: Root directory has only contracts.go + aliases.go

- GIVEN P2 is merged
- WHEN the layout guard runs
- THEN any production file in `contextengine/` root matching the allowlist is permitted
- AND violations cause CI to fail

<!-- T: D2-STRUCT-T01 -->

#### Scenario: New code cannot call legacy.Process

- GIVEN P5 is merged
- WHEN any non-allowlisted production file under `cmd/` or `internal/` calls `legacy.ContextEngine.Process(ctx`
- THEN `D2-STRUCT-T07` fails
- AND the developer is prompted to migrate to `D7 SessionOrchestrator` or `turn_adapter.ExecuteRound`

<!-- T: D2-STRUCT-T07 -->

---

## MODIFIED: Legacy Retirement (facade → legacy)

The `facade/` directory is renamed to `legacy/` to reflect its retired status. `ContextEngine` / `EngineDeps` / `NewContextEngine` remain accessible as `// Deprecated:` type aliases to `legacy.*`.

#### Scenario: Process() emits slog.Warn at runtime

- GIVEN P5 is merged
- WHEN `legacy.ContextEngine.Process()` is called (e.g., from `cmd/llm-smoke` or test)
- THEN a `slog.Warn` is logged with `sessionID` and migration hint
- AND execution continues (backward compatible)

#### Scenario: legacy/ directory deletion gate

- The `legacy/` directory is physically deleted only after AC-P5-4 conditions are met:
  - All Process callers have migrated to D7 SessionOrchestrator or turn_adapter
  - Integration tests have been green for ≥7 consecutive days

---

## ADDED: Memory Shared Types/Contracts

`MemoryEntry` and the `LongTermRecaller` / `LongTermStore` port interfaces are extracted to shared types/contracts to break the prepare/memory ↔ persist/memory cyclic import risk.

#### Scenario: MemoryEntry lives in shared types

- GIVEN P4 is merged
- WHEN a developer imports `MemoryEntry` from anywhere in the codebase
- THEN the canonical import is `internal/shared/types/memory.go`
- AND no domain-specific package re-exports the type via alias (only convenience aliases in `contextengine/prepare/memory/recall.go`)

#### Scenario: Memory port interfaces are domain-agnostic

- GIVEN P4 is merged
- WHEN a consumer injects a LongTerm memory backend
- THEN the consumer depends on `internal/shared/contracts.LongTermRecaller` / `LongTermStore`
- AND the implementation (`contextengine/persist/memory/store.go`) satisfies these interfaces

---

## Cross-References

- **layer-delta.md** v8.0.0 → v8.2.0 section: P3/P4/P5 requirement blocks
- **d2-domain.md** v8.2.0: 终态物理路径映射表 + 修订记录
- **code-layout.md** v1.12.0: §4.3 D2 表 + 深度规则 + D2-STRUCT-T01..T07 守卫列表
- **layering.md** v4.7.0: contextengine/ 终态树
- **span-registry.md** v2.3.0: 路径同步
- **t-registry.md**: D2-STRUCT-T01..T07 + DM-20260619-007 关联

