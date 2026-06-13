package coverage

import "sort"

// OperationMeta describes a canonical operation for registry and reconciliation.
type OperationMeta struct {
	Name          string
	Layer         string
	Component     string
	SinceVersion  string
	Instrumented  bool
}

// AllOperations returns the canonical operation registry sorted by name.
func AllOperations() []OperationMeta {
	ops := []OperationMeta{
		{Name: "adapter.cli.send", Layer: "communication", Component: "adapter", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "adapter.feishu.outbound", Layer: "communication", Component: "adapter", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "adapter.message.receive", Layer: "communication", Component: "adapter", SinceVersion: "1.3.0", Instrumented: true},

		{Name: "agent.fork", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "agent.join", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "agent.run", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "agent.state.transition", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "agent.terminate", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "agent.tool.call", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},

		{Name: "context.compression.run", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.harness.bootstrap.run", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "context.harness.bootstrap.stage", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "context.harness.preflight", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "context.harness.route", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "context.harness.tool_pool", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "context.system_prompt.build", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "context.longterm.recall", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "context.longterm.store", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
			{Name: "context.memory.snapshot.save", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "context.process", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.snapshot.load", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
			{Name: "context.system_prompt.load", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},
			{Name: "context.tools.register", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},

		{Name: "gateway.agent.create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.engine_event.handle", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.message.receive", Layer: "communication", Component: "gateway", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "gateway.permission.check", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.session.create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.session.expire", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.session.get", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.session.lifecycle", Layer: "communication", Component: "gateway", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "gateway.store.create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.store.delete", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.store.get", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "gateway.store.update", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},

		{Name: "llm.adapter.stream", Layer: "llm", Component: "llm_adapter", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "llm.circuit_breaker", Layer: "llm", Component: "llm_gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "llm.provider.route", Layer: "llm", Component: "llm_gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "llm.retry", Layer: "llm", Component: "llm_gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "llm.stream", Layer: "llm", Component: "llm_gateway", SinceVersion: "1.2.0", Instrumented: true},
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
	return ops
}

// KnownOperations returns a set of registered operation names.
func KnownOperations() map[string]struct{} {
	ops := AllOperations()
	set := make(map[string]struct{}, len(ops))
	for _, op := range ops {
		set[op.Name] = struct{}{}
	}
	return set
}

// IsKnown reports whether operation is in the canonical registry.
func IsKnown(operation string) bool {
	_, ok := KnownOperations()[operation]
	return ok
}
