package config

import "time"

// ExecutionFlowConfig controls WorkPlan / ExecutionFlowHub (v2 Hub-Spoke).
type ExecutionFlowConfig struct {
	Enabled             bool `yaml:"enabled"`
	LinkTasks           bool `yaml:"link_tasks"`
	IMProgress          bool `yaml:"im_progress"`
	ToolSummaryThrottleMs int  `yaml:"tool_summary_throttle_ms"`
	EventBufferSize     int  `yaml:"event_buffer_size"`
}

// DefaultExecutionFlowConfig returns v2 defaults (disabled until explicitly enabled).
func DefaultExecutionFlowConfig() ExecutionFlowConfig {
	return ExecutionFlowConfig{
		Enabled:               false,
		LinkTasks:             true,
		IMProgress:            true,
		ToolSummaryThrottleMs: 500,
		EventBufferSize:       32,
	}
}

// NormalizeExecutionFlowConfig applies defaults to zero values.
func NormalizeExecutionFlowConfig(cfg ExecutionFlowConfig) ExecutionFlowConfig {
	def := DefaultExecutionFlowConfig()
	if cfg.EventBufferSize <= 0 {
		cfg.EventBufferSize = def.EventBufferSize
	}
	if cfg.ToolSummaryThrottleMs <= 0 {
		cfg.ToolSummaryThrottleMs = def.ToolSummaryThrottleMs
	}
	return cfg
}

// ToolSummaryThrottle returns the throttle duration.
func (c ExecutionFlowConfig) ToolSummaryThrottle() time.Duration {
	ms := c.ToolSummaryThrottleMs
	if ms <= 0 {
		ms = 500
	}
	return time.Duration(ms) * time.Millisecond
}
