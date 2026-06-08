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
		{Name: "gateway.message.receive", Layer: "communication", Component: "gateway", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "gateway.session.lifecycle", Layer: "communication", Component: "gateway", SinceVersion: "1.3.0", Instrumented: true},

		{Name: "adapter.message.receive", Layer: "communication", Component: "adapter", SinceVersion: "1.3.0", Instrumented: true},

		{Name: "context.process", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.snapshot.load", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.compression.run", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.plan.generate", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "context.milestone.run", Layer: "context", Component: "pev_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "context.longterm.recall", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "context.longterm.store", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},

		{Name: "context.pev.run", Layer: "context", Component: "pev_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.pev.llm_call", Layer: "context", Component: "pev_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.pev.tool_execute", Layer: "context", Component: "pev_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.pev.permission_check", Layer: "context", Component: "pev_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "context.pev.verify", Layer: "context", Component: "pev_engine", SinceVersion: "1.2.0", Instrumented: true},

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
