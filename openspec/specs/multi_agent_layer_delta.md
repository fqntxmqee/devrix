# Delta: Multi-Agent Layer (Layer 4)

**Change ID:** devrix-foundation
**Affects:** multi-agent, agent lifecycle, collaboration modes, tool registry, permission pipeline

---

## ADDED

### Requirement: Agent Lifecycle

Agent state machine for lifecycle management.

#### Scenario: Create agent
- GIVEN no active agent for session
- WHEN AgentFactory.create is called
- THEN agent transitions to CREATED state
- AND initial context is set
- AND tools are registered

#### Scenario: Run agent
- GIVEN agent is in CREATED state
- WHEN agent.run is called
- THEN state transitions to RUNNING
- AND LLM is invoked with agent context
- AND state transitions to ITERATING

#### Scenario: Agent iteration loop
- GIVEN agent is in ITERATING state
- WHEN LLM response contains tool calls
- THEN tools are executed via ToolRegistry
- AND results are fed back to LLM
- AND loop continues until no more tools

#### Scenario: Fork agent (V1 - simple)
- GIVEN agent needs parallel subtask
- WHEN agent.fork is called
- THEN new Agent is created with shared context
- AND parent agent waits for child result
- AND results are merged after child completes

#### Scenario: Terminate agent
- GIVEN agent has completed all tasks
- WHEN agent.terminate is called
- THEN state transitions to TERMINATED
- AND final response is sent via Communication Layer

---

### Requirement: Built-in Tool Registry

V1 built-in tools with risk levels.

#### Scenario: Register read-only tools
- GIVEN tool definitions for read operations
- WHEN ToolRegistry.registerBuiltins is called
- THEN tools registered: read, glob, ls, grep
- AND risk level: LOW

#### Scenario: Register medium-risk tool
- GIVEN tool definition for fetch
- WHEN ToolRegistry.registerBuiltins is called
- THEN fetch tool is registered
- AND risk level: MEDIUM

#### Scenario: Register high-risk tools
- GIVEN tool definitions for write operations
- WHEN ToolRegistry.registerBuiltins is called
- THEN tools registered: write, edit
- AND risk level: HIGH

#### Scenario: Register critical tools
- GIVEN tool definitions for shell operations
- WHEN ToolRegistry.registerBuiltins is called
- THEN tools registered: bash, git
- AND risk level: CRITICAL
- AND require permission even in V1

---

### Requirement: Permission Pipeline

User confirmation for sensitive operations.

#### Scenario: Request permission for critical tool
- GIVEN tool bash with command "rm -rf /" is called
- WHEN PermissionPipeline.request is called
- THEN permission request is sent to user
- AND execution pauses (async wait)
- AND timeout timer starts (60s default)

#### Scenario: User grants permission
- GIVEN permission request is pending
- WHEN user responds with "yes"
- THEN permission is granted
- AND tool execution continues
- AND audit log is written

#### Scenario: User denies permission
- GIVEN permission request is pending
- WHEN user responds with "no"
- THEN permission is denied
- AND PermissionDeniedError is thrown
- AND audit log is written

#### Scenario: Permission timeout
- GIVEN permission request is pending
- WHEN 60 seconds pass without response
- THEN permission is auto-denied
- AND PermissionTimeoutError is thrown
- AND user is notified

---

### Requirement: Collaboration Modes

V1 implements simplified chain-of-thought and iterative-refinement.

#### Scenario: Chain-of-thought mode
- GIVEN complex reasoning task
- WHEN agent is configured with chain-of-thought mode
- THEN LLM is instructed to reason step-by-step
- AND each step's output is added to context
- AND final answer is extracted from last message

#### Scenario: Iterative-refinement mode
- GIVEN task requires improvement
- WHEN agent is configured with iterative-refinement mode
- THEN initial response is generated
- AND critique is requested
- AND improvements are applied
- AND loop continues until满意 or max iterations

---

## MODIFIED

(None - initial layer specification)

---

## REMOVED

| Item | Reason |
|------|--------|
| Supervisor-Worker mode (task decomposition) | V3 feature |
| Peer-Review mode (multi-agent review) | V3 feature |
| Vote-Consensus mode (multi-agent voting) | V3 feature |
| Full Fork/Merge with Milestone DAG | V3 feature |
| ShortId (5-char permission code) | V2 feature |
