package evaluate

import "context"

// Probe 是单个评测维度的评分器。
type Probe interface {
	ID() string
	Run(ctx context.Context, item EvalItem, judge Judge) (*DomainScore, error)
}

// probeRegistry 全局探针注册表。
var probeRegistry = map[string]Probe{}

// RegisterProbe 注册探针。
func RegisterProbe(p Probe) {
	probeRegistry[p.ID()] = p
}

// GetProbe 按 ID 查找探针。
func GetProbe(id string) Probe {
	return probeRegistry[id]
}
