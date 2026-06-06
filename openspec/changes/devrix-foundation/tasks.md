# Implementation Tasks: Devrix Foundation

**Change ID:** `devrix-foundation`
**Status:** Draft

---

## Phase 1: Project Setup & Shared Modules

### 1.1 Initialize Project
- [ ] 1.1.1 Initialize npm project with package.json
- [ ] 1.1.2 Install dependencies (typescript, vitest, commander, pino, etc.)
- [ ] 1.1.3 Configure TypeScript (tsconfig.json)
- [ ] 1.1.4 Configure Vitest (vitest.config.ts)
- [ ] 1.1.5 Setup build scripts (tsup or tsx)

### 1.2 Shared Types
- [ ] 1.2.1 Define Message, Session, Agent types
- [ ] 1.2.2 Define Tool, ToolCall, ToolResult types
- [ ] 1.2.3 Define Permission, PermissionRequest types
- [ ] 1.2.4 Define TraceEvent, Metric types
- [ ] 1.2.5 Define Error types (DevrixError, ContextExceededError, etc.)

### 1.3 Shared Config
- [ ] 1.3.1 Define LayerConfig interface
- [ ] 1.3.2 Implement ConfigLoader (JSON file + env override)
- [ ] 1.3.3 Define LLM model configurations
- [ ] 1.3.4 Define compression budget settings

### 1.4 Shared Utils
- [ ] 1.4.1 Implement token counter utility
- [ ] 1.4.2 Implement ID generator (requestId)
- [ ] 1.4.3 Implement logger factory (pino-based)

### 1.5 Shared Errors
- [ ] 1.5.1 Define error hierarchy (DevrixError base class)
- [ ] 1.5.2 Define ContextExceededError
- [ ] 1.5.3 Define PermissionDeniedError
- [ ] 1.5.4 Define PermissionTimeoutError
- [ ] 1.5.5 Define UnsupportedModelError
- [ ] 1.5.6 Define TokenBudgetExceededError

**Quality Gate:**
- [ ] Code analysis passes (no critical issues)
- [ ] Shared module tests pass

---

## Phase 2: Communication Layer (Layer 1)

### 2.1 CLI Adapter
- [ ] 2.1.1 Implement CliAdapter class
- [ ] 2.1.2 Implement ANSI renderer for streaming
- [ ] 2.1.3 Implement command parser (/new, /stop, /help)
- [ ] 2.1.4 Test CLI adapter

### 2.2 Session Store
- [ ] 2.2.1 Define Session interface
- [ ] 2.2.2 Implement FileSessionStore
- [ ] 2.2.3 Implement createSession, restoreSession, updateSession
- [ ] 2.2.4 Implement session idle timeout (30min)
- [ ] 2.2.5 Test session store

### 2.3 Communication Gateway
- [ ] 2.3.1 Implement CommunicationGateway class
- [ ] 2.3.2 Implement routeInbound, routeOutbound
- [ ] 2.3.3 Implement routePermission
- [ ] 2.3.4 Test gateway integration

**Quality Gate:**
- [ ] Code analysis passes
- [ ] Unit tests pass for all 2.x tasks

---

## Phase 3: Observability Layer (Layer 5)

> Note: Observability is implemented before other layers because other layers depend on it.

### 3.1 Trace Events
- [ ] 3.1.1 Define TraceEvent interface
- [ ] 3.1.2 Implement TraceEmitter class
- [ ] 3.1.3 Implement traceId generation and propagation
- [ ] 3.1.4 Implement emit methods for all event types
- [ ] 3.1.5 Test trace emitter

### 3.2 Metrics Collection
- [ ] 3.2.1 Define Metric types (Counter, Histogram, Gauge)
- [ ] 3.2.2 Implement MetricsCollector class
- [ ] 3.2.3 Implement llm_tokens_total counter
- [ ] 3.2.4 Implement llm_latency_seconds histogram
- [ ] 3.2.5 Implement tool_calls_total counter
- [ ] 3.2.6 Implement session_active gauge
- [ ] 3.2.7 Implement permission_timeouts counter
- [ ] 3.2.8 Test metrics collector

### 3.3 Structured Logging
- [ ] 3.3.1 Implement Logger with trace context
- [ ] 3.3.2 Configure pino with JSON format
- [ ] 3.3.3 Implement component identification
- [ ] 3.3.4 Implement log level filtering
- [ ] 3.3.5 Test logger

**Quality Gate:**
- [ ] Code analysis passes
- [ ] Observability layer tests pass

---

## Phase 4: Context Engine Layer (Layer 2)

### 4.1 PEV Engine
- [ ] 4.1.1 Define PEVState interface
- [ ] 4.1.2 Implement PEVEngine class
- [ ] 4.1.3 Implement plan phase (parse task)
- [ ] 4.1.4 Implement execute phase (call LLM + tools)
- [ ] 4.1.5 Implement verify phase (run verification commands)
- [ ] 4.1.6 Test PEV engine (simplified V1 version)

### 4.2 Compression Pipeline
- [ ] 4.2.1 Define CompressionContext interface
- [ ] 4.2.2 Implement ToolResultBudget step
- [ ] 4.2.3 Implement Snip step (old-to-new truncation)
- [ ] 4.2.4 Implement Microcompact step (same-role merge)
- [ ] 4.2.5 Implement ContextCollapse step
- [ ] 4.2.6 Implement SystemPromptAssembly step
- [ ] 4.2.7 Skip Autocompact (V2 feature - add placeholder)
- [ ] 4.2.8 Implement TokenBlock step
- [ ] 4.2.9 Test compression pipeline

### 4.3 Layered Memory
- [ ] 4.3.1 Define Memory interfaces (Working, ShortTerm, LongTerm)
- [ ] 4.3.2 Implement WorkingMemory (in-memory Map)
- [ ] 4.3.3 Implement ShortTermMemory (session-scoped)
- [ ] 4.3.4 Stub LongTermMemory (throw FeatureNotImplemented)
- [ ] 4.3.5 Test layered memory

### 4.4 Context State Management
- [ ] 4.4.1 Implement initializeContext
- [ ] 4.4.2 Implement updateContext
- [ ] 4.4.3 Test context management

**Quality Gate:**
- [ ] Code analysis passes
- [ ] Context engine tests pass

---

## Phase 5: LLM Gateway Layer (Layer 3)

### 5.1 Model Adapter Interface
- [ ] 5.1.1 Define ILLMAdapter interface
- [ ] 5.1.2 Define LLMRequest, LLMResponse types
- [ ] 5.1.3 Implement AnthropicAdapter
- [ ] 5.1.4 Implement DeepSeekAdapter
- [ ] 5.1.5 Implement OpenAIAdapter (for Qwen, MiniMax, etc.)
- [ ] 5.1.6 Test model adapters

### 5.2 Circuit Breaker
- [ ] 5.2.1 Define CircuitState enum (closed, open, half-open)
- [ ] 5.2.2 Implement CircuitBreaker class
- [ ] 5.2.3 Implement failure threshold logic
- [ ] 5.2.4 Implement 30-second half-open timer
- [ ] 5.2.5 Test circuit breaker

### 5.3 Token Counter
- [ ] 5.3.1 Implement TokenCounter class
- [ ] 5.3.2 Implement count method (cl100k_base encoding)
- [ ] 5.3.3 Implement checkBudget method
- [ ] 5.3.4 Implement estimateRemaining method
- [ ] 5.3.5 Test token counter

### 5.4 LLM Gateway
- [ ] 5.4.1 Implement LLMGateway class
- [ ] 5.4.2 Implement chat method with streaming
- [ ] 5.4.3 Implement fallback model logic
- [ ] 5.4.4 Test LLM gateway

**Quality Gate:**
- [ ] Code analysis passes
- [ ] LLM gateway tests pass

---

## Phase 6: Multi-Agent Layer (Layer 4)

### 6.1 Tool Registry
- [ ] 6.1.1 Define ToolDefinition interface
- [ ] 6.1.2 Implement ToolRegistry class
- [ ] 6.1.3 Register read-only tools (read, glob, ls, grep)
- [ ] 6.1.4 Register medium-risk tool (fetch)
- [ ] 6.1.5 Register high-risk tools (write, edit)
- [ ] 6.1.6 Register critical tools (bash, git)
- [ ] 6.1.7 Test tool registry

### 6.2 Permission Pipeline
- [ ] 6.2.1 Define PermissionRequest interface
- [ ] 6.2.2 Implement PermissionPipeline class
- [ ] 6.2.3 Implement request method (async wait)
- [ ] 6.2.4 Implement resolve method (user response)
- [ ] 6.2.5 Implement timeout handler (60s)
- [ ] 6.2.6 Test permission pipeline

### 6.3 Agent Lifecycle
- [ ] 6.3.1 Define AgentState enum
- [ ] 6.3.2 Define AgentContext interface
- [ ] 6.3.3 Implement AgentFactory
- [ ] 6.3.4 Implement Agent.run (iteration loop)
- [ ] 6.3.5 Implement Agent.fork (simplified V1)
- [ ] 6.3.6 Implement Agent.terminate
- [ ] 6.3.7 Test agent lifecycle

### 6.4 Collaboration Modes
- [ ] 6.4.1 Implement ChainOfThought mode
- [ ] 6.4.2 Implement IterativeRefinement mode
- [ ] 6.4.3 Test collaboration modes

**Quality Gate:**
- [ ] Code analysis passes
- [ ] Multi-agent tests pass

---

## Phase 7: Evolution Layer (Layer 6)

### 7.1 Placeholder Implementation
- [ ] 7.1.1 Implement EvolutionLayer placeholder class
- [ ] 7.1.2 Add log indicating V2 feature
- [ ] 7.1.3 Test placeholder

**Quality Gate:**
- [ ] Code analysis passes

---

## Phase 8: CLI Entry Point & Integration

### 8.1 CLI Commands
- [ ] 8.1.1 Setup Commander.js entry point
- [ ] 8.1.2 Implement --version, --help commands
- [ ] 8.1.3 Implement /new, /stop, /help commands
- [ ] 8.1.4 Test CLI commands

### 8.2 Layer Integration
- [ ] 8.2.1 Wire Communication Layer → Context Engine Layer
- [ ] 8.2.2 Wire Context Engine Layer → LLM Gateway Layer
- [ ] 8.2.3 Wire Context Engine Layer → Multi-Agent Layer
- [ ] 8.2.4 Wire all layers → Observability Layer
- [ ] 8.2.5 Test layer integration

### 8.3 End-to-End Test
- [ ] 8.3.1 Write E2E test for CLI → LLM response flow
- [ ] 8.3.2 Write E2E test for tool execution flow
- [ ] 8.3.3 Write E2E test for permission flow
- [ ] 8.3.4 Test error scenarios

**Quality Gate:**
- [ ] All E2E tests pass
- [ ] Code analysis clean

---

## Phase 9: Documentation & Polish

### 9.1 Documentation
- [ ] 9.1.1 Write README.md with setup instructions
- [ ] 9.1.2 Write USAGE.md with command reference
- [ ] 9.1.3 Update delta specs if implementation differs

### 9.2 Final Verification
- [ ] 9.2.1 Run full test suite
- [ ] 9.2.2 Verify coverage ≥ 80%
- [ ] 9.2.3 Build production bundle
- [ ] 9.2.4 Smoke test the bundle

**Quality Gate:**
- [ ] All tests pass
- [ ] Coverage ≥ 80%
- [ ] Bundle builds successfully

---

## Completion Checklist

- [ ] All phases complete
- [ ] All quality gates passed
- [ ] Delta specs match implementation
- [ ] Documentation synced
- [ ] Ready for `/openspec-archive`
