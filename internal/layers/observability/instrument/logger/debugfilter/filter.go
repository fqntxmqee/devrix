// Package debugfilter — A2 Debug 日志分类过滤,对标 clawcode --debug=category CLI 行为。
//
// 包装一个 logger.Handler,只放行 Component ∈ enabled 集合(或非 debug 级别)的 entry。
//
// 启用方式:CLI `--debug=api,hooks,telemetry` → New(inner, []string{"api","hooks","telemetry"})。
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-parity/design.md §2.10
package debugfilter

import (
	"github.com/devrix/devrix/internal/layers/observability/instrument/logger"
)

// Filter logger.Handler 包装器:按 Component 字段过滤 debug 级别。
type Filter struct {
	inner   logger.Handler
	enabled map[string]bool
	// passthroughNonDebug: 任何非 Debug 级别(Info/Warn/Error)直接放行,不受 enabled 控制。
	passthroughNonDebug bool
}

// New 构造 Filter。categories 空时,Filter 等同 passthrough(全部放行)。
func New(inner logger.Handler, categories []string) *Filter {
	enabled := make(map[string]bool, len(categories))
	for _, c := range categories {
		if c != "" {
			enabled[c] = true
		}
	}
	return &Filter{
		inner:               inner,
		enabled:             enabled,
		passthroughNonDebug: true,
	}
}

// WithPassthroughNonDebug 切换非 debug 级别是否放行。默认 true。
func (f *Filter) WithPassthroughNonDebug(b bool) *Filter {
	f.passthroughNonDebug = b
	return f
}

// Enabled 报告 categories 是否非空。
func (f *Filter) Enabled() bool {
	return f != nil && len(f.enabled) > 0
}

// Handle 过滤 + 委托。
func (f *Filter) Handle(entry *logger.LogEntry) error {
	if entry == nil {
		return nil
	}
	if f == nil || f.inner == nil {
		return nil
	}
	if !f.Enabled() {
		// 没启用 categories → 全部放行(等同 raw passthrough)
		return f.inner.Handle(entry)
	}
	if f.passthroughNonDebug && entry.Level != "DEBUG" {
		return f.inner.Handle(entry)
	}
	if entry.Component == "" {
		// 无 Component 标记的 debug entry 默认不通过(强制走 whitelist)
		return nil
	}
	if f.enabled[entry.Component] {
		return f.inner.Handle(entry)
	}
	return nil
}

// Categories 导出当前启用的 category 集合(只读副本)。
func (f *Filter) Categories() []string {
	if f == nil {
		return nil
	}
	out := make([]string, 0, len(f.enabled))
	for c := range f.enabled {
		out = append(out, c)
	}
	return out
}
