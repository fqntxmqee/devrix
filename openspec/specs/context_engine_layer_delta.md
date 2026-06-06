# Delta: Context Engine Layer (Layer 2)

**Change ID:** devrix-foundation
**Affects:** context engine, PEV engine, compression pipeline, layered memory

---

## ADDED

### Requirement: PEV Engine

Plan → Execute → Verify loop for task handling.

#### Scenario: PEV Execute phase
- GIVEN user message requires action
- WHEN PEVEngine.execute is called
- THEN LLM is invoked with current context
- AND tool calls are extracted from response
- AND tools are registered for execution

#### Scenario: PEV Verify phase
- GIVEN tool execution completed
- WHEN PEVEngine.verify is called
- THEN verification commands are constructed
- AND results are analyzed for deviation
- AND if deviation > threshold, re-execute from Plan

#### Scenario: PEV simplified (V1)
- GIVEN V1 does not have Milestone DAG
- WHEN PEV loop runs
- THEN only single-level Execute → Verify
- AND no recursive sub-task verification

---

### Requirement: Seven-Step Compression Pipeline

V1 implements steps 1-5 and 7 (step 6 is V2).

#### Scenario: Step 1 - Tool Result Budget
- GIVEN messages contain tool results
- WHEN compressMessages is called
- THEN tool results are truncated to budget (e.g., 800 tokens)
- AND truncated indicator is added if cut

#### Scenario: Step 2 - Snip (Old-to-New Truncation)
- GIVEN total tokens exceed budget
- WHEN Snip step runs
- THEN oldest messages are removed until within budget
- AND "truncated" marker is added

#### Scenario: Step 3 - Microcompact (Same-Role Merge)
- GIVEN consecutive messages from same role
- WHEN Microcompact step runs
- THEN messages are merged into single message
- AND content is concatenated with separators

#### Scenario: Step 4 - Context Collapse
- GIVEN conversation has repetitive patterns
- WHEN Collapse step runs
- THEN trivial exchanges are folded
- AND key information is preserved

#### Scenario: Step 5 - System Prompt Assembly
- GIVEN system prompt and compressed messages
- WHEN Assembly step runs
- THEN system prompt is placed first
- AND messages follow in chronological order
- AND total token budget is respected

#### Scenario: Step 6 - Autocompact (V2, skipped in V1)
- GIVEN V1 implementation
- WHEN step 6 would be called
- THEN it is skipped (V2 feature)
- AND a placeholder log is emitted

#### Scenario: Step 7 - Token Block
- GIVEN compressed result still exceeds budget
- WHEN TokenBlock step runs
- THEN error is thrown: ContextExceededError
- AND no further processing occurs

---

### Requirement: Layered Memory

Three-tier memory system.

#### Scenario: Working Memory (in-memory)
- GIVEN current task processing
- WHEN messages, currentTask, activeTools are needed
- THEN working memory provides them from memory Map
- AND data is never persisted

#### Scenario: Short-Term Memory (session-scoped)
- GIVEN session exists with sessionId
- WHEN session state is accessed
- THEN short-term memory provides sessionId, milestones, budget
- AND data is kept in memory + optionally persisted to file

#### Scenario: Long-Term Memory (V3, not implemented)
- GIVEN V1/V2 implementation
- WHEN long-term memory is accessed
- THEN FeatureNotImplementedError is thrown

---

### Requirement: Context State Management

#### Scenario: Initialize context for new session
- GIVEN new session is created
- WHEN initializeContext is called
- THEN working memory is populated
- AND short-term memory structures are created
- AND system prompt is loaded

#### Scenario: Update context after message
- GIVEN user sends a message
- WHEN updateContext is called
- THEN message is added to history
- AND compression pipeline is triggered if needed
- AND working memory is updated

---

## MODIFIED

(None - initial layer specification)

---

## REMOVED

| Item | Reason |
|------|--------|
| Autocompact (LLM-generated summary) | V2 feature |
| Long-term memory (cross-session) | V3 feature |
| Milestone DAG | V3 feature |
