# LSP Tool Surface Specification

**Surface ID:** D2-S4-A01
**Phase:** 1 (P0)
**Status:** S3_Designed

<!-- T: D2-S4-A01-T01, D2-S4-A01-T02, D2-S4-A01-T03, D2-S4-A01-T04, D2-S4-A01-T05, D2-S4-A01-T06 -->

## ADDED

### Requirement: LSP Go-to-Definition

#### Scenario: Successful Go-to-Definition
- GIVEN a workspace with Go files and a running gopls server
- WHEN LLM calls `lsp_goToDefinition` with `{uri: "main.go", line: 42, character: 10}` referring to a function `CalculateTotal`
- THEN the surface returns positions where `CalculateTotal` is defined
- AND the result includes `{uri, range: {start, end}}` for each definition
- AND the result is returned within 2 seconds

#### Scenario: LSP Server Unavailable Fallback
- GIVEN gopls server is not running or crashes during query
- WHEN LLM calls `lsp_goToDefinition`
- THEN the surface returns `{fallback: "grep", result: grep_matches}` (greps for the symbol)
- AND a span `d2.lsp.fallback` is emitted with `fallback_reason`

#### Scenario: LSP Timeout
- GIVEN gopls server takes > 2 seconds to respond
- WHEN LLM calls `lsp_goToDefinition`
- THEN the surface returns a partial result if available
- AND emits span `d2.lsp.timeout` with `elapsed_ms`

#### Scenario: Symbol Not Found
- GIVEN the symbol at the given position is not defined elsewhere
- WHEN LLM calls `lsp_goToDefinition`
- THEN the surface returns an empty array `[]`

### Requirement: LSP Find References

#### Scenario: Find All References
- GIVEN a Go workspace with a function `CalculateTotal` used in 5 places
- WHEN LLM calls `lsp_findReferences` with `{uri, line, character}` pointing to `CalculateTotal`
- THEN the surface returns 5 reference positions
- AND includes the definition itself in the results

<!-- T: D2-S4-A01-T02 -->

#### Scenario: Find References with Include Declaration Flag
- GIVEN a function `CalculateTotal`
- WHEN LLM calls `lsp_findReferences` with `{includeDeclaration: false}`
- THEN the surface returns 4 reference positions (excluding the declaration)

### Requirement: LSP Incoming Calls (Call Hierarchy)

#### Scenario: Find Incoming Calls
- GIVEN a function `CalculateTotal` called by `OrderService.Submit`
- WHEN LLM calls `lsp_incomingCalls` with `{uri, line, character}` pointing to `CalculateTotal`
- THEN the surface returns the call hierarchy with `OrderService.Submit` as a caller

<!-- T: D2-S4-A01-T03 -->

#### Scenario: Recursive Call Chain
- GIVEN A calls B, B calls A (recursive)
- WHEN LLM calls `lsp_incomingCalls` on A
- THEN the surface returns the full call chain (with cycle detection)

### Requirement: LSP Hover

#### Scenario: Hover for Function
- GIVEN a Go function with doc comments
- WHEN LLM calls `lsp_hover` with `{uri, line, character}` pointing to a function name
- THEN the surface returns `{contents: {kind: "markdown", value: "Func doc"}}`

<!-- T: D2-S4-A01-T04 -->

#### Scenario: Hover for Variable Type
- GIVEN a Go variable with inferred type
- WHEN LLM calls `lsp_hover` on the variable
- THEN the surface returns the type signature

### Requirement: LSP Workspace Symbol

#### Scenario: Search Symbols by Pattern
- GIVEN a Go workspace
- WHEN LLM calls `lsp_workspaceSymbol` with `{query: "Calcul*"}`
- THEN the surface returns all symbols starting with "Calcul" (functions, types, variables)

<!-- T: D2-S4-A01-T05 -->

### Requirement: LSP Server Process Pool

#### Scenario: LRU Pool Eviction
- GIVEN 5 workspaces with gopls servers (LRU capacity 4)
- WHEN a 5th workspace is opened
- THEN the least recently used gopls server is evicted
- AND total LSP server count stays at 4

<!-- T: D2-S4-A01-T06 -->

#### Scenario: Server Health Check
- GIVEN a gopls server is unresponsive (heartbeat miss)
- WHEN next LSP call is made
- THEN the server is restarted
- AND a span `d2.lsp.server_restart` is emitted

### Requirement: LTL-Lite Invariants

<!-- T: D2-S4-A01-F*-T07 -->

The LSP surface MUST satisfy the following invariants (encoded in `lsp/_invariant.go`):

- `read_only => !modifies_files` (LSP operations never modify files)
- `lsp_servers <= 4` (LRU pool upper bound)
- `timeout => fallback(grep)` (timeout always has grep fallback)
- `hover_result.format = markdown` (hover always returns markdown)
- `!modifies_lsp_server_state` (concurrent calls don't mutate server state)

## MODIFIED

(None)

## REMOVED

(None)
