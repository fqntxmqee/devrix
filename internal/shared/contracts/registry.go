// Package contracts — cross-layer contract registry.
//
// The registry is the single source of truth for every contract exposed to
// multiple D{N} layers. Adding a new interface to shared/contracts should
// always be paired with a Register() call from package init() (or the
// explicit DefaultCatalog seed below). SelfCheck() then guarantees that no
// consumer references a contract that the registry does not know about.
//
// T: CROSS-A02-T03  (contract registry resolves every cross-layer interface)
package contracts

import "sort"

// Contract is a description of one interface/type that crosses layer
// boundaries. Owner is the package that defines it; Source is the file inside
// the package (best-effort, for human reference only).
type Contract struct {
	Name   string
	Owner  string
	Source string
}

// Consumer is a layer that depends on a Contract. The registry uses this
// table at SelfCheck() time to flag dangling references.
type Consumer struct {
	Contract string
	Layer    string
}

// Registry is a thread-unsafe lookup of contracts. The intended pattern is to
// build it once at package init() and only read after.
type Registry struct {
	contracts map[string]Contract
	consumers []Consumer
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{contracts: map[string]Contract{}}
}

// Register adds a contract to the registry. Re-registration overwrites.
func (r *Registry) Register(c Contract) {
	r.contracts[c.Name] = c
}

// RegisterAll is a convenience for seeding the registry from a slice.
func (r *Registry) RegisterAll(cs []Contract) {
	for _, c := range cs {
		r.contracts[c.Name] = c
	}
}

// RegisterConsumer records that a layer depends on the named contract.
func (r *Registry) RegisterConsumer(c Consumer) {
	r.consumers = append(r.consumers, c)
}

// Lookup resolves a contract by name. The second return is false on miss.
func (r *Registry) Lookup(name string) (Contract, bool) {
	c, ok := r.contracts[name]
	return c, ok
}

// Contracts returns a sorted snapshot of every registered contract.
func (r *Registry) Contracts() []Contract {
	out := make([]Contract, 0, len(r.contracts))
	for _, c := range r.contracts {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// SelfCheck returns a slice of human-readable problems. An empty slice means
// the registry is internally consistent (every consumer references a known
// contract and every contract is registered).
func (r *Registry) SelfCheck() []string {
	var problems []string
	for _, c := range r.consumers {
		if _, ok := r.contracts[c.Contract]; !ok {
			problems = append(problems, "consumer "+c.Layer+" references unregistered contract "+c.Contract)
		}
	}
	sort.Strings(problems)
	return problems
}

// DefaultCatalog returns the seed list of every cross-layer contract that
// lives in shared/contracts. The list is consulted by SelfCheck() tests and
// by the devrix-layer-lint --strict CI gate (via the contract-coverage
// probe in D6 eval).
func DefaultCatalog() []Contract {
	return []Contract{
		{Name: "IEngine", Owner: "shared/contracts", Source: "engine.go"},
		{Name: "EngineEvent", Owner: "shared/contracts", Source: "engine.go"},
		{Name: "ITokenCounter", Owner: "shared/contracts", Source: "tokencounter.go"},
		{Name: "ExecutionFlowHub", Owner: "shared/contracts", Source: "execution_flow.go"},
		{Name: "FlowEvent", Owner: "shared/contracts", Source: "execution_flow.go"},
		{Name: "ComputeCtxPct", Owner: "shared/contracts", Source: "ctxutil.go"},
		{Name: "IPermissionGate", Owner: "shared/contracts", Source: "permission.go"},
		{Name: "ILLMGateway", Owner: "llmgateway", Source: "contracts.go"},
		{Name: "ITierResolver", Owner: "llmgateway", Source: "contracts.go"},
		{Name: "IToolRunner", Owner: "contextengine/toolrunner", Source: "contracts.go"},
		{Name: "IToolRegistry", Owner: "contextengine/toolrunner", Source: "contracts.go"},
	}
}

// global registry seeded at process start so the layer-lint CI gate and
// D6 LayerViolationProbe can resolve contracts without each caller having
// to wire its own instance.
var global = NewRegistry()

func init() {
	global.RegisterAll(DefaultCatalog())
}

// Global returns the process-wide registry. It is preloaded with the
// DefaultCatalog seed list; layers can register additional contracts at
// package init() time.
func Global() *Registry { return global }
